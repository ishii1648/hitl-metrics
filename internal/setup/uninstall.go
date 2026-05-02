package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Uninstall removes hitl-metrics hook entries from ~/.claude/settings.json.
//
// Targets: single-hook entries created by the legacy `hitl-metrics install`
// (matcher empty, exactly one command referencing a hitl-metrics
// subcommand). Composed entries (matcher set, or multiple hooks bundled)
// are left alone — those were almost certainly written by a human.
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
	for _, spec := range ClaudeHookSpecs {
		raw, ok := hooks[spec.Event]
		if !ok {
			continue
		}
		var entries []hookEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return fmt.Errorf("parse hooks.%s: %w", spec.Event, err)
		}

		filtered := entries[:0]
		for _, e := range entries {
			if isHitlOnlyEntry(e, spec.Subcommand) {
				fmt.Printf("uninstall: %s (%s) — 削除\n", spec.Event, spec.Subcommand)
				removed++
				continue
			}
			filtered = append(filtered, e)
		}

		if len(filtered) == 0 {
			delete(hooks, spec.Event)
			continue
		}
		out, err := json.Marshal(filtered)
		if err != nil {
			return err
		}
		hooks[spec.Event] = json.RawMessage(out)
	}

	if removed == 0 {
		fmt.Println("uninstall: hitl-metrics の hook エントリは見つかりませんでした")
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
// by the legacy `hitl-metrics install` (matcher empty, exactly one command
// that references the given hitl-metrics subcommand).
func isHitlOnlyEntry(e hookEntry, subcommand string) bool {
	if e.Matcher != "" {
		return false
	}
	if len(e.Hooks) != 1 {
		return false
	}
	cmd := e.Hooks[0].Command
	return strings.Contains(cmd, "hitl-metrics") && strings.Contains(cmd, "hook "+subcommand)
}
