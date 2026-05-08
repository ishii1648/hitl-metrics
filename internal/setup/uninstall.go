package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Uninstall removes agent-telemetry hook entries from ~/.claude/settings.json.
//
// Targets: single-hook entries created by the legacy `install` subcommand
// (matcher empty, exactly one command referencing an agent-telemetry or
// hitl-metrics subcommand). Composed entries (matcher set, or multiple
// hooks bundled) are left alone — those were almost certainly written by
// a human. The legacy "hitl-metrics" name is still matched so that
// `agent-telemetry uninstall-hooks` can clean up entries written before
// the rename.
//
// Codex side (`~/.codex/config.toml`) is intentionally not touched: the
// TOML is human-edited and we cannot safely round-trip arbitrary TOML in
// Go without a heavyweight dependency.
func Uninstall() error {
	path := settingsPathFn()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("uninstall: %s が存在しません — 何もすることがありません\n", path)
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	settings := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	hooksRaw, ok := settings["hooks"]
	if !ok {
		fmt.Println("uninstall: hooks セクションが存在しません — 何もすることがありません")
		return nil
	}
	hooks := make(map[string]json.RawMessage)
	if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
		return fmt.Errorf("parse hooks: %w", err)
	}

	removed := 0
	cleanEvent := func(event, subcommand string) error {
		raw, ok := hooks[event]
		if !ok {
			return nil
		}
		var entries []hookEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return fmt.Errorf("parse hooks.%s: %w", event, err)
		}

		filtered := entries[:0]
		for _, e := range entries {
			if isHitlOnlyEntry(e, subcommand) {
				fmt.Printf("uninstall: %s (%s) — 削除\n", event, subcommand)
				removed++
				continue
			}
			filtered = append(filtered, e)
		}

		if len(filtered) == 0 {
			delete(hooks, event)
			return nil
		}
		out, err := json.Marshal(filtered)
		if err != nil {
			return err
		}
		hooks[event] = json.RawMessage(out)
		return nil
	}

	for _, spec := range ClaudeHookSpecs {
		if err := cleanEvent(spec.Event, spec.Subcommand); err != nil {
			return err
		}
	}
	// 廃止済みサブコマンド（過去の install で書き込まれた可能性のあるもの）
	// は SessionStart に残ることが多い。全イベントを舐めて取り除く。
	for _, sub := range LegacyClaudeSubcommands {
		events := make([]string, 0, len(hooks))
		for event := range hooks {
			events = append(events, event)
		}
		for _, event := range events {
			if err := cleanEvent(event, sub); err != nil {
				return err
			}
		}
	}

	if removed == 0 {
		fmt.Println("uninstall: agent-telemetry の hook エントリは見つかりませんでした")
		return nil
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		out, err := json.Marshal(hooks)
		if err != nil {
			return err
		}
		settings["hooks"] = json.RawMessage(out)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	fmt.Printf("uninstall: 完了 — 削除 %d（%s）\n", removed, path)
	return nil
}

type hookEntry struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookCommand `json:"hooks"`
}

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// isHitlOnlyEntry reports whether the entry is a single-hook entry created
// by the legacy `install` subcommand (matcher empty, exactly one command
// that references the given agent-telemetry subcommand).
//
// Matches both the new "agent-telemetry" and the legacy "hitl-metrics"
// binary names so unmigrated environments can still be cleaned up.
func isHitlOnlyEntry(e hookEntry, subcommand string) bool {
	if e.Matcher != "" {
		return false
	}
	if len(e.Hooks) != 1 {
		return false
	}
	cmd := e.Hooks[0].Command
	if !strings.Contains(cmd, "hook "+subcommand) {
		return false
	}
	return strings.Contains(cmd, "agent-telemetry") || strings.Contains(cmd, "hitl-metrics")
}
