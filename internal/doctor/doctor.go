// Package doctor inspects the local environment and reports whether
// hitl-metrics is set up correctly. It NEVER mutates settings — diagnosis only.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ishii1648/hitl-metrics/internal/install"
)

// Run executes all checks against the real filesystem and writes a report
// to stdout. Returns nil even when warnings/failures are found — the exit
// code is decided by the caller based on Result.HasFailure.
func Run() (Result, error) {
	return RunWith(os.Stdout, defaultEnv())
}

// RunWith is the test-friendly entry point. The caller supplies an io.Writer
// for the report and an Env describing the paths/lookup functions to use.
func RunWith(w io.Writer, env Env) (Result, error) {
	r := Result{}

	r.Binary = checkBinary(env)
	writeBinary(w, r.Binary)

	r.DataDir = checkDataDir(env)
	writeDataDir(w, r.DataDir)

	r.Hooks = checkHooks(env)
	writeHooks(w, env, r.Hooks)

	return r, nil
}

// Env bundles the paths and lookup functions doctor depends on. Tests
// substitute fakes; production code uses defaultEnv().
type Env struct {
	SettingsPath string                       // ~/.claude/settings.json
	DataDir      string                       // ~/.claude
	LookPath     func(string) (string, error) // exec.LookPath replacement
	BinaryName   string                       // "hitl-metrics"
}

func defaultEnv() Env {
	home, _ := os.UserHomeDir()
	return Env{
		SettingsPath: install.SettingsPath(),
		DataDir:      filepath.Join(home, ".claude"),
		LookPath:     exec.LookPath,
		BinaryName:   "hitl-metrics",
	}
}

// Result aggregates the outcome of every check. HasFailure reports whether
// any check failed (used to set the process exit code).
type Result struct {
	Binary  CheckResult
	DataDir CheckResult
	Hooks   []HookCheck
}

// HasFailure is true when at least one check did not pass.
func (r Result) HasFailure() bool {
	if !r.Binary.OK || !r.DataDir.OK {
		return true
	}
	for _, h := range r.Hooks {
		if !h.OK {
			return true
		}
	}
	return false
}

// CheckResult is the result of a single binary/dir check.
type CheckResult struct {
	OK     bool
	Detail string // path on success, error description on failure
}

// HookCheck is the result of looking up one expected hook.
type HookCheck struct {
	Spec install.HookSpec
	OK   bool
}

func checkBinary(env Env) CheckResult {
	path, err := env.LookPath(env.BinaryName)
	if err != nil {
		return CheckResult{OK: false, Detail: fmt.Sprintf("not found in PATH (%v)", err)}
	}
	return CheckResult{OK: true, Detail: path}
}

func checkDataDir(env Env) CheckResult {
	info, err := os.Stat(env.DataDir)
	if err != nil {
		return CheckResult{OK: false, Detail: fmt.Sprintf("missing: %s", env.DataDir)}
	}
	if !info.IsDir() {
		return CheckResult{OK: false, Detail: fmt.Sprintf("not a directory: %s", env.DataDir)}
	}
	return CheckResult{OK: true, Detail: env.DataDir}
}

// checkHooks returns the per-spec registration status, in the canonical
// HookSpecs order so the report is stable.
func checkHooks(env Env) []HookCheck {
	registered := loadRegisteredCommands(env.SettingsPath)
	out := make([]HookCheck, 0, len(install.HookSpecs))
	for _, spec := range install.HookSpecs {
		out = append(out, HookCheck{
			Spec: spec,
			OK:   isRegistered(registered[spec.Event], spec.Subcommand),
		})
	}
	return out
}

// loadRegisteredCommands extracts, per event name, the command strings of
// every registered hook entry. Missing/invalid files yield an empty map —
// the caller treats that as "nothing registered" and emits warnings.
func loadRegisteredCommands(path string) map[string][]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}
	out := make(map[string][]string, len(settings.Hooks))
	for event, entries := range settings.Hooks {
		for _, e := range entries {
			for _, h := range e.Hooks {
				out[event] = append(out[event], h.Command)
			}
		}
	}
	return out
}

// isRegistered reports whether any command string under the event references
// the given hitl-metrics subcommand. Matcher / timeout / co-existence with
// other hooks are deliberately ignored — only command substring matters.
func isRegistered(commands []string, subcommand string) bool {
	needle := "hook " + subcommand
	for _, cmd := range commands {
		if strings.Contains(cmd, "hitl-metrics") && strings.Contains(cmd, needle) {
			return true
		}
	}
	return false
}

const (
	markPass = "✓"
	markFail = "✗"
	markWarn = "⚠"
)

func writeBinary(w io.Writer, c CheckResult) {
	if c.OK {
		fmt.Fprintf(w, "%s binary at %s\n", markPass, c.Detail)
		return
	}
	fmt.Fprintf(w, "%s binary: %s\n", markFail, c.Detail)
}

func writeDataDir(w io.Writer, c CheckResult) {
	if c.OK {
		fmt.Fprintf(w, "%s data dir at %s\n", markPass, c.Detail)
		return
	}
	fmt.Fprintf(w, "%s data dir: %s\n", markFail, c.Detail)
}

func writeHooks(w io.Writer, env Env, checks []HookCheck) {
	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}

	if allOK {
		fmt.Fprintf(w, "%s hook registration:\n", markPass)
	} else {
		fmt.Fprintf(w, "%s hook registration:\n", markWarn)
	}
	for _, c := range checks {
		if c.OK {
			fmt.Fprintf(w, "  - %s: %s %s\n", c.Spec.Event, c.Spec.Subcommand, markPass)
		} else {
			fmt.Fprintf(w, "  - %s: %s %s (not registered in %s)\n",
				c.Spec.Event, c.Spec.Subcommand, markFail, env.SettingsPath)
		}
	}
	if !allOK {
		fmt.Fprintln(w, "  → register manually or via dotfiles (see docs/setup.md)")
	}
}
