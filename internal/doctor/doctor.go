// Package doctor inspects the local environment and reports whether
// agent-telemetry is set up correctly. It NEVER mutates settings — diagnosis only.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ishii1648/agent-telemetry/internal/agent"
	"github.com/ishii1648/agent-telemetry/internal/legacy"
	"github.com/ishii1648/agent-telemetry/internal/setup"
)

// Run executes all checks against the real filesystem and writes a report
// to stdout. Returns nil even when warnings/failures are found — the exit
// code is decided by the caller based on Result.HasFailure.
func Run() (Result, error) {
	return RunWith(os.Stdout, defaultEnv())
}

// RunWith is the test-friendly entry point.
func RunWith(w io.Writer, env Env) (Result, error) {
	r := Result{}

	r.Binary = checkBinary(env)
	writeBinary(w, r.Binary)

	for _, a := range env.Agents {
		ar := AgentReport{Agent: a}
		ar.DataDir = checkDataDir(a)
		writeDataDir(w, a, ar.DataDir)

		ar.Hooks = checkHooks(env, a)
		writeHooks(w, a, env, ar.Hooks)

		r.AgentReports = append(r.AgentReports, ar)
	}

	r.Legacy = checkLegacy(env)
	writeLegacy(w, r.Legacy)

	return r, nil
}

// Env bundles the lookup functions doctor depends on. Tests substitute
// fakes; production code uses defaultEnv().
type Env struct {
	LookPath   func(string) (string, error)
	BinaryName string

	// Agents is the list of agents to inspect. defaultEnv() populates this
	// from agent.Detect() (or both agents when none are detected).
	Agents []*agent.Agent

	// SettingsLoader returns the registered command strings (per event)
	// for the given agent. Allows tests to inject fake settings without
	// writing to disk. When nil, defaults to the on-disk loader.
	SettingsLoader func(*agent.Agent) map[string][]string

	// LegacyPaths returns paths from the hitl-metrics era that still exist
	// on disk. When nil, defaults to legacy.PresentLegacyPaths.
	LegacyPaths func() []string
}

func defaultEnv() Env {
	agents := agent.Detect()
	if len(agents) == 0 {
		agents = agent.All()
	}
	return Env{
		LookPath:   exec.LookPath,
		BinaryName: "agent-telemetry",
		Agents:     agents,
	}
}

// Result aggregates the outcome of every check.
type Result struct {
	Binary       CheckResult
	AgentReports []AgentReport
	Legacy       LegacyReport
}

// HasFailure is true when at least one check did not pass. Legacy
// artifacts are reported as warnings (not failures) — they don't block
// the user, they just nudge them to clean up.
func (r Result) HasFailure() bool {
	if !r.Binary.OK {
		return true
	}
	for _, ar := range r.AgentReports {
		if !ar.DataDir.OK {
			return true
		}
		for _, h := range ar.Hooks {
			if !h.OK && !h.Spec.Optional {
				return true
			}
		}
	}
	return false
}

// AgentReport bundles the per-agent check results.
type AgentReport struct {
	Agent   *agent.Agent
	DataDir CheckResult
	Hooks   []HookCheck
}

// CheckResult is the result of a single binary/dir check.
type CheckResult struct {
	OK     bool
	Detail string
}

// HookCheck is the result of looking up one expected hook.
type HookCheck struct {
	Spec setup.HookSpec
	OK   bool
}

// LegacyReport lists hitl-metrics era files that still exist on disk
// and legacy hook command strings still registered in settings.
type LegacyReport struct {
	Paths []string
	Hooks []LegacyHook
}

// LegacyHook is a hook command string that still references the
// hitl-metrics binary name.
type LegacyHook struct {
	Agent   string
	Event   string
	Command string
}

func checkBinary(env Env) CheckResult {
	path, err := env.LookPath(env.BinaryName)
	if err != nil {
		return CheckResult{OK: false, Detail: fmt.Sprintf("not found in PATH (%v)", err)}
	}
	return CheckResult{OK: true, Detail: path}
}

func checkDataDir(a *agent.Agent) CheckResult {
	info, err := os.Stat(a.DataDir)
	if err != nil {
		return CheckResult{OK: false, Detail: fmt.Sprintf("missing: %s", a.DataDir)}
	}
	if !info.IsDir() {
		return CheckResult{OK: false, Detail: fmt.Sprintf("not a directory: %s", a.DataDir)}
	}
	return CheckResult{OK: true, Detail: a.DataDir}
}

func checkHooks(env Env, a *agent.Agent) []HookCheck {
	loader := env.SettingsLoader
	if loader == nil {
		loader = loadRegisteredCommandsForAgent
	}
	registered := loader(a)
	specs := setup.HookSpecsFor(a.Name)
	out := make([]HookCheck, 0, len(specs))
	for _, spec := range specs {
		out = append(out, HookCheck{
			Spec: spec,
			OK:   isRegistered(registered[spec.Event], spec.Subcommand),
		})
	}
	return out
}

// loadRegisteredCommandsForAgent dispatches to the agent's settings
// loader. Claude reads ~/.claude/settings.json (JSON), Codex reads
// ~/.codex/hooks.json (JSON) — config.toml is ignored for now because
// adding a TOML dependency is heavyweight; users that prefer TOML can
// also write hooks.json side-by-side.
func loadRegisteredCommandsForAgent(a *agent.Agent) map[string][]string {
	switch a.Name {
	case agent.NameCodex:
		return loadJSONHooks(setup.CodexHooksPath())
	default:
		return loadJSONHooks(setup.SettingsPath())
	}
}

// loadJSONHooks returns the per-event command strings recorded under
// the top-level `hooks` map in a settings/hooks JSON file.
func loadJSONHooks(path string) map[string][]string {
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

// isRegistered reports whether any command string under the event
// references the given agent-telemetry subcommand.
//
// Matches both the new "agent-telemetry" and the legacy "hitl-metrics"
// binary names so doctor still detects unmigrated environments after
// the rename. checkLegacy collects the legacy matches separately so a
// dedicated warning section can list them.
func isRegistered(commands []string, subcommand string) bool {
	needle := "hook " + subcommand
	for _, cmd := range commands {
		if !strings.Contains(cmd, needle) {
			continue
		}
		if strings.Contains(cmd, "agent-telemetry") || strings.Contains(cmd, "hitl-metrics") {
			return true
		}
	}
	return false
}

// checkLegacy collects every hitl-metrics-era artifact still present:
// renamed-but-not-migrated files on disk, plus settings entries that
// still call the old binary name.
func checkLegacy(env Env) LegacyReport {
	paths := env.LegacyPaths
	if paths == nil {
		paths = legacy.PresentLegacyPaths
	}
	r := LegacyReport{Paths: paths()}

	loader := env.SettingsLoader
	if loader == nil {
		loader = loadRegisteredCommandsForAgent
	}
	for _, a := range env.Agents {
		registered := loader(a)
		for event, cmds := range registered {
			for _, cmd := range cmds {
				if strings.Contains(cmd, "hitl-metrics") && !strings.Contains(cmd, "agent-telemetry") {
					r.Hooks = append(r.Hooks, LegacyHook{Agent: a.Name, Event: event, Command: cmd})
				}
			}
		}
	}
	return r
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

func writeDataDir(w io.Writer, a *agent.Agent, c CheckResult) {
	if c.OK {
		fmt.Fprintf(w, "%s [%s] data dir at %s\n", markPass, a.Name, c.Detail)
		return
	}
	fmt.Fprintf(w, "%s [%s] data dir: %s\n", markFail, a.Name, c.Detail)
}

func writeHooks(w io.Writer, a *agent.Agent, env Env, checks []HookCheck) {
	allOK := true
	for _, c := range checks {
		if !c.OK && !c.Spec.Optional {
			allOK = false
			break
		}
	}

	settingsPath := agentSettingsPath(a)
	if allOK {
		fmt.Fprintf(w, "%s [%s] hook registration:\n", markPass, a.Name)
	} else {
		fmt.Fprintf(w, "%s [%s] hook registration:\n", markWarn, a.Name)
	}
	for _, c := range checks {
		switch {
		case c.OK:
			fmt.Fprintf(w, "  - %s: %s %s\n", c.Spec.Event, c.Spec.Subcommand, markPass)
		case c.Spec.Optional:
			fmt.Fprintf(w, "  - %s: %s %s (optional, not registered in %s)\n",
				c.Spec.Event, c.Spec.Subcommand, markWarn, settingsPath)
		default:
			fmt.Fprintf(w, "  - %s: %s %s (not registered in %s)\n",
				c.Spec.Event, c.Spec.Subcommand, markFail, settingsPath)
		}
	}
	if !allOK {
		fmt.Fprintln(w, "  → register manually or via dotfiles (see docs/setup.md)")
	}
}

func agentSettingsPath(a *agent.Agent) string {
	switch a.Name {
	case agent.NameCodex:
		return setup.CodexHooksPath()
	default:
		return setup.SettingsPath()
	}
}

func writeLegacy(w io.Writer, r LegacyReport) {
	if len(r.Paths) == 0 && len(r.Hooks) == 0 {
		return
	}
	fmt.Fprintf(w, "%s legacy hitl-metrics artifacts detected:\n", markWarn)
	for _, p := range r.Paths {
		fmt.Fprintf(w, "  - file: %s (will be auto-renamed by next backfill / sync-db run)\n", p)
	}
	for _, h := range r.Hooks {
		fmt.Fprintf(w, "  - hook [%s/%s]: %s\n", h.Agent, h.Event, h.Command)
	}
	if len(r.Hooks) > 0 {
		fmt.Fprintln(w, "  → update settings.json / hooks.json to call `agent-telemetry hook ...` (see docs/setup.md)")
	}
}
