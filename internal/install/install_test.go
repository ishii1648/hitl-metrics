package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]json.RawMessage {
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

func TestRun_NewSettings(t *testing.T) {
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	if err := Run(); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	for _, event := range []string{"SessionStart", "SessionEnd", "Stop"} {
		if _, ok := hooks[event]; !ok {
			t.Fatalf("missing hook event: %s", event)
		}
	}

	// SessionStart should have 2 entries (session-start + todo-cleanup)
	var entries []hookEntry
	json.Unmarshal(hooks["SessionStart"], &entries)
	if len(entries) != 2 {
		t.Fatalf("SessionStart: expected 2 entries, got %d", len(entries))
	}
}

func TestRun_Idempotent(t *testing.T) {
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	// Run twice
	if err := Run(); err != nil {
		t.Fatal(err)
	}
	if err := Run(); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	// SessionStart: 2 entries (session-start + todo-cleanup), not duplicated
	var sessionEntries []hookEntry
	json.Unmarshal(hooks["SessionStart"], &sessionEntries)
	if len(sessionEntries) != 2 {
		t.Fatalf("SessionStart: expected 2 entries, got %d", len(sessionEntries))
	}

	var stopEntries []hookEntry
	json.Unmarshal(hooks["Stop"], &stopEntries)
	if len(stopEntries) != 1 {
		t.Fatalf("Stop: expected 1 entry, got %d", len(stopEntries))
	}

	var endEntries []hookEntry
	json.Unmarshal(hooks["SessionEnd"], &endEntries)
	if len(endEntries) != 1 {
		t.Fatalf("SessionEnd: expected 1 entry, got %d", len(endEntries))
	}
}

func TestRun_PreservesExistingSettings(t *testing.T) {
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	os.MkdirAll(filepath.Dir(settingsFile), 0755)
	os.WriteFile(settingsFile, []byte(`{"model":"sonnet","hooks":{}}`), 0644)

	if err := Run(); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var model string
	json.Unmarshal(m["model"], &model)
	if model != "sonnet" {
		t.Fatalf("existing key lost: model=%q", model)
	}
}

func TestRun_PreservesExistingHooks(t *testing.T) {
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	os.MkdirAll(filepath.Dir(settingsFile), 0755)
	os.WriteFile(settingsFile, []byte(`{
		"hooks": {
			"SessionStart": [
				{"matcher": "", "hooks": [{"type": "command", "command": "/other/hook.sh"}]}
			]
		}
	}`), 0644)

	if err := Run(); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	var entries []hookEntry
	json.Unmarshal(hooks["SessionStart"], &entries)
	// Should have: existing + session-start + todo-cleanup = 3
	if len(entries) != 3 {
		t.Fatalf("SessionStart: expected 3 entries, got %d", len(entries))
	}
}

func TestRun_CommandFormat(t *testing.T) {
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	if err := Run(); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	// Verify commands use "hitl-metrics hook <event>" format
	var stopEntries []hookEntry
	json.Unmarshal(hooks["Stop"], &stopEntries)
	if len(stopEntries) != 1 {
		t.Fatal("expected 1 Stop entry")
	}
	cmd := stopEntries[0].Hooks[0].Command
	if cmd != "hitl-metrics hook stop" {
		t.Fatalf("Stop command = %q, want %q", cmd, "hitl-metrics hook stop")
	}

	var endEntries []hookEntry
	json.Unmarshal(hooks["SessionEnd"], &endEntries)
	if len(endEntries) != 1 {
		t.Fatal("expected 1 SessionEnd entry")
	}
	cmd = endEntries[0].Hooks[0].Command
	if cmd != "hitl-metrics hook session-end" {
		t.Fatalf("SessionEnd command = %q, want %q", cmd, "hitl-metrics hook session-end")
	}
}
