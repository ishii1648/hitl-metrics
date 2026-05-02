// Package install is a deprecated alias for internal/setup. It exists so
// the old `hitl-metrics install` subcommand continues to work while users
// migrate to `hitl-metrics setup` / `hitl-metrics uninstall-hooks`.
//
// New code MUST use internal/setup directly.
package install

import (
	"fmt"
	"os"

	"github.com/ishii1648/hitl-metrics/internal/setup"
)

// HookSpec is re-exported for backward compatibility (doctor used to import
// it from here). Prefer setup.HookSpec going forward.
type HookSpec = setup.HookSpec

// HookSpecs preserves the pre-Codex Claude-only list. Doctor uses
// setup.HookSpecsFor instead.
//
// Deprecated: use setup.ClaudeHookSpecs.
var HookSpecs = setup.ClaudeHookSpecs

// SettingsPath returns ~/.claude/settings.json.
//
// Deprecated: use setup.SettingsPath.
func SettingsPath() string { return setup.SettingsPath() }

// Run is the legacy entry point. It now prints a deprecation warning and
// delegates to setup.Run with no agent (so both Claude + Codex examples
// are shown).
func Run() error {
	fmt.Fprintln(os.Stderr, "warning: `hitl-metrics install` は廃止予定です。`hitl-metrics setup` を使ってください。")
	return setup.Run(nil)
}

// Uninstall delegates to setup.Uninstall.
//
// Deprecated: use the standalone `hitl-metrics uninstall-hooks` subcommand.
func Uninstall() error {
	fmt.Fprintln(os.Stderr, "warning: `hitl-metrics install --uninstall-hooks` は廃止予定です。`hitl-metrics uninstall-hooks` を使ってください。")
	return setup.Uninstall()
}
