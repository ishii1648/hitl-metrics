package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookSpec describes an expected hitl-metrics hook entry.
// It is shared by install (uninstall path), doctor (verification),
// and tests so the canonical list lives in one place.
type HookSpec struct {
	Event      string // Claude Code hook event name
	Subcommand string // hitl-metrics hook subcommand (without "hitl-metrics hook " prefix)
}

// HookSpecs is the canonical list of hooks hitl-metrics expects to be
// registered (typically via dotfiles).
var HookSpecs = []HookSpec{
	{Event: "SessionStart", Subcommand: "session-start"},
	{Event: "SessionStart", Subcommand: "todo-cleanup"},
	{Event: "SessionEnd", Subcommand: "session-end"},
	{Event: "Stop", Subcommand: "stop"},
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

// Run is the entry point for `hitl-metrics install`.
//
// hook の登録は dotfiles 側に一元化したため、本コマンドは settings.json を
// 変更しない。検証は `hitl-metrics doctor` で行い、移行のため過去の自動登録
// を取り除きたい場合は `hitl-metrics install --uninstall-hooks` を使う。
func Run() error {
	fmt.Println("install: hook の自動登録は廃止されました。")
	fmt.Println("  - 登録方法: dotfiles に取り込むか、手動で ~/.claude/settings.json を編集してください")
	fmt.Println("  - 登録の検証: hitl-metrics doctor")
	fmt.Println("  - 過去に自動登録された hook を削除: hitl-metrics install --uninstall-hooks")
	fmt.Println("  - 設定例: docs/setup.md")
	return nil
}

// Uninstall removes hitl-metrics hook entries from ~/.claude/settings.json.
// 各イベント配下から、コマンドが "hitl-metrics hook <subcommand>" を含む
// 単一フックのエントリを削除する。複数フックを束ねたエントリや matcher 付き
// エントリは（人間が組み立てた可能性が高いため）触らない。
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
	for _, spec := range HookSpecs {
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

// isHitlOnlyEntry reports whether the entry is a single-hook entry created by
// the legacy `hitl-metrics install` (matcher empty, exactly one command that
// references the given hitl-metrics subcommand).
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
