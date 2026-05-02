// Package setup prints agent-specific hook registration instructions and
// (only as `uninstall-hooks`) removes legacy auto-registered entries from
// ~/.claude/settings.json. It does NOT register hooks — dotfiles or manual
// editing remain the source of truth.
package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ishii1648/hitl-metrics/internal/agent"
)

// HookSpec describes one hitl-metrics hook entry.
//
// CodingAgent identifies which agent the entry belongs to so doctor /
// uninstall can scope their action correctly.
type HookSpec struct {
	CodingAgent string // "claude" or "codex"
	Event       string // hook event name as written in settings (e.g. "SessionStart")
	Subcommand  string // hitl-metrics hook subcommand
	Optional    bool   // true → doctor flags as info, not failure
}

// ClaudeHookSpecs lists the canonical hooks for Claude Code.
var ClaudeHookSpecs = []HookSpec{
	{CodingAgent: agent.NameClaude, Event: "SessionStart", Subcommand: "session-start"},
	{CodingAgent: agent.NameClaude, Event: "SessionStart", Subcommand: "todo-cleanup"},
	{CodingAgent: agent.NameClaude, Event: "SessionEnd", Subcommand: "session-end"},
	{CodingAgent: agent.NameClaude, Event: "Stop", Subcommand: "stop"},
}

// CodexHookSpecs lists the canonical hooks for Codex CLI. PostToolUse is
// optional — Codex is happy without PR URL auto-detection, since `update`
// and Stop-time backfill cover the same data.
var CodexHookSpecs = []HookSpec{
	{CodingAgent: agent.NameCodex, Event: "SessionStart", Subcommand: "session-start"},
	{CodingAgent: agent.NameCodex, Event: "Stop", Subcommand: "stop"},
	{CodingAgent: agent.NameCodex, Event: "PostToolUse", Subcommand: "post-tool-use", Optional: true},
}

// HookSpecsFor returns the hook list for a given agent.
func HookSpecsFor(name string) []HookSpec {
	switch name {
	case agent.NameCodex:
		return CodexHookSpecs
	default:
		return ClaudeHookSpecs
	}
}

// settingsPathFn returns ~/.claude/settings.json. Replaceable in tests.
var settingsPathFn = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// SettingsPath returns the resolved path of ~/.claude/settings.json.
func SettingsPath() string {
	return settingsPathFn()
}

// CodexHooksPath returns the conventional path for ~/.codex/hooks.json.
//
// Codex supports two equivalent registration mechanisms (TOML in
// config.toml or JSON in hooks.json); doctor inspects both.
func CodexHooksPath() string {
	a := agent.Codex()
	return filepath.Join(a.DataDir, "hooks.json")
}

// CodexConfigPath returns ~/.codex/config.toml.
func CodexConfigPath() string {
	a := agent.Codex()
	return filepath.Join(a.DataDir, "config.toml")
}

// Run prints registration instructions for the requested agent (or all
// agents when a is nil). It writes nothing to disk.
func Run(a *agent.Agent) error {
	return RunWith(os.Stdout, a)
}

// RunWith is the test-friendly variant of Run.
func RunWith(w io.Writer, a *agent.Agent) error {
	if a == nil {
		fmt.Fprintln(w, "setup: agent 別のセットアップ手順を表示します（hook 登録は dotfiles または手動）")
		fmt.Fprintln(w)
		printClaude(w)
		fmt.Fprintln(w)
		printCodex(w)
		return nil
	}
	switch a.Name {
	case agent.NameCodex:
		printCodex(w)
	default:
		printClaude(w)
	}
	return nil
}

func printClaude(w io.Writer) {
	fmt.Fprintln(w, "# Claude Code (~/.claude/settings.json)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  hook 登録は dotfiles または手動で行ってください。")
	fmt.Fprintln(w, "  検証: hitl-metrics doctor")
	fmt.Fprintln(w, "  過去の自動登録を削除: hitl-metrics uninstall-hooks")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  例:")
	fmt.Fprintln(w, "  {")
	fmt.Fprintln(w, "    \"hooks\": {")
	fmt.Fprintln(w, "      \"SessionStart\": [")
	fmt.Fprintln(w, "        {\"matcher\": \"\", \"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook session-start --agent claude\"}]},")
	fmt.Fprintln(w, "        {\"matcher\": \"\", \"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook todo-cleanup\"}]}")
	fmt.Fprintln(w, "      ],")
	fmt.Fprintln(w, "      \"SessionEnd\": [")
	fmt.Fprintln(w, "        {\"matcher\": \"\", \"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook session-end --agent claude\", \"timeout\": 10}]}")
	fmt.Fprintln(w, "      ],")
	fmt.Fprintln(w, "      \"Stop\": [")
	fmt.Fprintln(w, "        {\"matcher\": \"\", \"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook stop --agent claude\"}]}")
	fmt.Fprintln(w, "      ]")
	fmt.Fprintln(w, "    }")
	fmt.Fprintln(w, "  }")
}

func printCodex(w io.Writer) {
	fmt.Fprintln(w, "# Codex CLI (~/.codex/config.toml or ~/.codex/hooks.json)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Codex には SessionEnd イベントが無いため、Stop hook が SessionEnd を兼ねます。")
	fmt.Fprintln(w, "  hooks.json での例:")
	fmt.Fprintln(w, "  {")
	fmt.Fprintln(w, "    \"hooks\": {")
	fmt.Fprintln(w, "      \"SessionStart\": [")
	fmt.Fprintln(w, "        {\"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook session-start --agent codex\"}]}")
	fmt.Fprintln(w, "      ],")
	fmt.Fprintln(w, "      \"Stop\": [")
	fmt.Fprintln(w, "        {\"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook stop --agent codex\"}]}")
	fmt.Fprintln(w, "      ],")
	fmt.Fprintln(w, "      \"PostToolUse\": [")
	fmt.Fprintln(w, "        {\"hooks\": [{\"type\": \"command\", \"command\": \"hitl-metrics hook post-tool-use --agent codex\"}]}")
	fmt.Fprintln(w, "      ]")
	fmt.Fprintln(w, "    }")
	fmt.Fprintln(w, "  }")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  config.toml 形式で書く場合は [features] codex_hooks = true を有効にしてから")
	fmt.Fprintln(w, "  [[hooks.SessionStart]] / [[hooks.Stop]] を追加してください。")
}
