// Package userid resolves the user identifier that gets stamped onto
// session-index.jsonl records and copied into sessions.user_id.
//
// Source precedence (first hit wins):
//
//  1. Environment variable AGENT_TELEMETRY_USER
//  2. agent-telemetry's TOML config `user` key (XDG path; falls back to
//     ~/.claude/agent-telemetry.toml on legacy installs — see internal/configpath)
//  3. git config --global user.email
//  4. fallback: "unknown"
//
// `git config --local` is intentionally NOT consulted: cwd-dependent
// values would split the same person across repos that override email
// per-checkout, which defeats the purpose of cross-machine attribution.
package userid

import (
	"bufio"
	"os"
	"os/exec"
	"strings"

	"github.com/ishii1648/agent-telemetry/internal/configpath"
)

// Unknown is the sentinel returned when no source produced a value.
const Unknown = "unknown"

// EnvVar is the highest-priority override.
const EnvVar = "AGENT_TELEMETRY_USER"

// Source identifies which precedence tier produced the resolved user_id.
// Stable string values — doctor displays these to the user.
type Source string

const (
	SourceEnv     Source = "env"
	SourceConfig  Source = "config"
	SourceGit     Source = "git"
	SourceUnknown Source = "unknown"
)

// Resolve returns the user_id along with the source that produced it.
// Errors are swallowed at every tier — falling back is always safe.
func Resolve() (string, Source) {
	if v := strings.TrimSpace(os.Getenv(EnvVar)); v != "" {
		return v, SourceEnv
	}
	if v := readConfigUser(ConfigPath()); v != "" {
		return v, SourceConfig
	}
	if v := readGitGlobalEmail(); v != "" {
		return v, SourceGit
	}
	return Unknown, SourceUnknown
}

// ConfigPath returns the resolved path of agent-telemetry's TOML config
// (XDG path with ~/.claude fallback for legacy installs). Delegates to
// configpath.Resolve so both userid and serverclient share the same
// migration warning gate.
func ConfigPath() string {
	return configpath.Resolve()
}

// readConfigUser reads the `user` key from a minimal TOML subset
// (top-level key=value lines, # comments, optional double quotes around
// the value). Anything we can't parse is silently ignored — a malformed
// config must never break the hook.
func readConfigUser(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			// Sections aren't supported; ignore everything after the
			// first one so we only honor top-level keys.
			break
		}
		key, value, ok := splitKV(line)
		if !ok {
			continue
		}
		if key != "user" {
			continue
		}
		return unquote(value)
	}
	return ""
}

// splitKV splits "key = value" (with optional inline `# comment`).
// `#` inside a double-quoted value is preserved; outside it begins a comment.
func splitKV(line string) (key, value string, ok bool) {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	value = strings.TrimSpace(line[eq+1:])
	if key == "" {
		return "", "", false
	}
	value = stripInlineComment(value)
	return key, value, true
}

// stripInlineComment removes a trailing `# ...` from value, respecting a
// single double-quoted region at the start (no escape handling — good enough
// for the `user` key).
func stripInlineComment(value string) string {
	if strings.HasPrefix(value, `"`) {
		if end := strings.IndexByte(value[1:], '"'); end >= 0 {
			head := value[:end+2]
			tail := value[end+2:]
			if hash := strings.IndexByte(tail, '#'); hash >= 0 {
				tail = tail[:hash]
			}
			return strings.TrimSpace(head + tail)
		}
		return value
	}
	if hash := strings.IndexByte(value, '#'); hash >= 0 {
		return strings.TrimSpace(value[:hash])
	}
	return value
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// readGitGlobalEmail returns `git config --global user.email`. We
// intentionally pass --global so cwd-local overrides don't split the
// same person across repositories.
func readGitGlobalEmail() string {
	cmd := exec.Command("git", "config", "--global", "user.email")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
