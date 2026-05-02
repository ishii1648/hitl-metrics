// Package agent abstracts coding-agent specific paths and detection.
//
// Claude Code と Codex CLI でデータディレクトリ・session-index 形式が異なるため、
// 上位レイヤ（hook / backfill / sync-db / doctor）はこの Agent 構造体を経由して
// agent 間の差分を吸収する。
package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Agent identifies a coding agent and resolves its on-disk locations.
type Agent struct {
	// Name is the canonical short name ("claude" or "codex"). Stored as-is
	// in session-index.coding_agent / sessions.coding_agent so the value
	// MUST be stable across versions.
	Name string

	// DataDir is the agent's home directory (~/.claude or $CODEX_HOME).
	DataDir string
}

const (
	// NameClaude is the canonical name for Claude Code.
	NameClaude = "claude"
	// NameCodex is the canonical name for Codex CLI.
	NameCodex = "codex"
)

// EnvVar is the environment variable that overrides --agent when the flag
// is omitted but autodetection is not desired.
const EnvVar = "HITL_METRICS_AGENT"

// SessionIndexPath returns <DataDir>/session-index.jsonl.
func (a *Agent) SessionIndexPath() string {
	return filepath.Join(a.DataDir, "session-index.jsonl")
}

// StatePath returns <DataDir>/hitl-metrics-state.json.
func (a *Agent) StatePath() string {
	return filepath.Join(a.DataDir, "hitl-metrics-state.json")
}

// LogDir returns <DataDir>/logs. Both agents write debug logs there for
// uniformity (the Codex log location is a hitl-metrics convention,
// not a Codex CLI requirement).
func (a *Agent) LogDir() string {
	return filepath.Join(a.DataDir, "logs")
}

// Claude returns the Claude Code agent rooted at ~/.claude.
func Claude() *Agent {
	return &Agent{Name: NameClaude, DataDir: claudeHome()}
}

// Codex returns the Codex CLI agent rooted at $CODEX_HOME (or ~/.codex).
func Codex() *Agent {
	return &Agent{Name: NameCodex, DataDir: codexHome()}
}

// ByName returns the agent with the given canonical name.
func ByName(name string) (*Agent, error) {
	switch name {
	case NameClaude:
		return Claude(), nil
	case NameCodex:
		return Codex(), nil
	case "":
		return nil, errors.New("agent name is empty")
	default:
		return nil, fmt.Errorf("unknown agent: %q (expected %q or %q)", name, NameClaude, NameCodex)
	}
}

// All returns both agents in canonical order (claude first, codex second).
// Order is stable so CLI output is reproducible.
func All() []*Agent {
	return []*Agent{Claude(), Codex()}
}

// Resolve picks one agent based on the precedence: flag → env → claude.
//
// Use this when a single agent must be chosen (typically hook subcommands).
// For commands that should iterate every present agent (backfill / sync-db /
// doctor) prefer ResolveOrDetect.
func Resolve(flag string) (*Agent, error) {
	if flag != "" {
		return ByName(flag)
	}
	if env := os.Getenv(EnvVar); env != "" {
		return ByName(env)
	}
	return Claude(), nil
}

// ResolveOrDetect returns the requested agent when flag/env names one,
// otherwise returns the auto-detected list (data-dir presence). When
// nothing is detected, it falls back to Claude so commands still produce
// a useful "no data" message instead of erroring out.
func ResolveOrDetect(flag string) ([]*Agent, error) {
	if flag != "" {
		a, err := ByName(flag)
		if err != nil {
			return nil, err
		}
		return []*Agent{a}, nil
	}
	if env := os.Getenv(EnvVar); env != "" {
		a, err := ByName(env)
		if err != nil {
			return nil, err
		}
		return []*Agent{a}, nil
	}
	detected := Detect()
	if len(detected) == 0 {
		return []*Agent{Claude()}, nil
	}
	return detected, nil
}

// Detect returns the agents whose data dir or session-index.jsonl exists.
// Order is canonical (claude before codex).
func Detect() []*Agent {
	var out []*Agent
	for _, a := range All() {
		if Present(a) {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Present reports whether an agent's data directory or session-index exists.
// Either signal is enough — a brand-new agent installation may have created
// the dir but no session yet, and a long-running setup may have only the
// session-index after a manual move.
func Present(a *Agent) bool {
	if _, err := os.Stat(a.SessionIndexPath()); err == nil {
		return true
	}
	if info, err := os.Stat(a.DataDir); err == nil && info.IsDir() {
		return true
	}
	return false
}

func claudeHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func codexHome() string {
	if env := os.Getenv("CODEX_HOME"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}
