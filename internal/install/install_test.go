package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func withSettingsPath(t *testing.T, path string) {
	t.Helper()
	orig := settingsPathFn
	settingsPathFn = func() string { return path }
	t.Cleanup(func() { settingsPathFn = orig })
}

func readSettings(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Run() は settings.json を一切変更してはならない。ファイルが存在しない場合も
// 作成されないこと（dotfiles 一元管理の前提を壊さないため）。
func TestRun_DoesNotModifySettings_WhenMissing(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	if err := Run(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(settingsFile); !os.IsNotExist(err) {
		t.Fatalf("Run() should not create settings.json, got err=%v", err)
	}
}

func TestRun_DoesNotModifySettings_WhenPresent(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	original := []byte(`{"model":"sonnet","hooks":{"SessionStart":[{"matcher":"","hooks":[{"type":"command","command":"/other/hook.sh"}]}]}}`)
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsFile, original, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Run(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("Run() modified settings.json:\n  before: %s\n  after:  %s", original, got)
	}
}

func TestUninstall_RemovesHitlHooks(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	initial := `{
		"model": "sonnet",
		"hooks": {
			"SessionStart": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-start"}]},
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook todo-cleanup"}]},
				{"matcher": "", "hooks": [{"type": "command", "command": "/other/script.sh"}]}
			],
			"SessionEnd": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-end", "timeout": 10}]}
			],
			"Stop": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook stop"}]}
			]
		}
	}`
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsFile, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}

	m := readSettings(t, settingsFile)

	var model string
	if err := json.Unmarshal(m["model"], &model); err != nil {
		t.Fatal(err)
	}
	if model != "sonnet" {
		t.Fatalf("model lost: got %q", model)
	}

	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(m["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}

	// SessionStart には ユーザー定義の /other/script.sh のみ残るはず
	var ssEntries []hookEntry
	if err := json.Unmarshal(hooks["SessionStart"], &ssEntries); err != nil {
		t.Fatal(err)
	}
	if len(ssEntries) != 1 {
		t.Fatalf("SessionStart: expected 1 entry, got %d", len(ssEntries))
	}
	if ssEntries[0].Hooks[0].Command != "/other/script.sh" {
		t.Fatalf("SessionStart: unexpected remaining command %q", ssEntries[0].Hooks[0].Command)
	}

	// SessionEnd / Stop は全削除 → キーごと消える
	if _, ok := hooks["SessionEnd"]; ok {
		t.Fatalf("SessionEnd should be removed entirely, got %s", hooks["SessionEnd"])
	}
	if _, ok := hooks["Stop"]; ok {
		t.Fatalf("Stop should be removed entirely, got %s", hooks["Stop"])
	}
}

// 複数 hook をまとめたエントリ（matcher 付き or 複数 hook）は人間が編集した
// 可能性が高いため触らない。
func TestUninstall_PreservesComposedEntries(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	initial := `{
		"hooks": {
			"SessionStart": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "hitl-metrics hook session-start"},
						{"type": "command", "command": "/other/script.sh"}
					]
				}
			]
		}
	}`
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsFile, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}

	m := readSettings(t, settingsFile)
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(m["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	var entries []hookEntry
	if err := json.Unmarshal(hooks["SessionStart"], &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].Hooks) != 2 {
		t.Fatalf("composed entry should be preserved, got %+v", entries)
	}
}

func TestUninstall_NoSettingsFile(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall on missing file should be no-op, got err=%v", err)
	}
}
