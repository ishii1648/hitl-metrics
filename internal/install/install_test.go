package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestHooksDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hooks")
	os.MkdirAll(dir, 0755)
	for _, name := range []string{"session-index.sh", "permission-log.sh", "stop.sh"} {
		os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/bash\n"), 0755)
	}
	return dir
}

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
	hooksDir := setupTestHooksDir(t)
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	// Override settingsPath for test
	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	if err := Run(hooksDir); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	for _, event := range []string{"SessionStart", "PermissionRequest", "Stop"} {
		if _, ok := hooks[event]; !ok {
			t.Fatalf("missing hook event: %s", event)
		}
	}
}

func TestRun_Idempotent(t *testing.T) {
	hooksDir := setupTestHooksDir(t)
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	// Run twice
	if err := Run(hooksDir); err != nil {
		t.Fatal(err)
	}
	if err := Run(hooksDir); err != nil {
		t.Fatal(err)
	}

	// Each event should have exactly 1 entry
	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	for _, event := range []string{"SessionStart", "PermissionRequest", "Stop"} {
		var entries []hookEntry
		json.Unmarshal(hooks[event], &entries)
		if len(entries) != 1 {
			t.Fatalf("%s: expected 1 entry, got %d", event, len(entries))
		}
	}
}

func TestRun_PreservesExistingSettings(t *testing.T) {
	hooksDir := setupTestHooksDir(t)
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	// Write existing settings with a custom key
	os.MkdirAll(filepath.Dir(settingsFile), 0755)
	os.WriteFile(settingsFile, []byte(`{"model":"sonnet","hooks":{}}`), 0644)

	if err := Run(hooksDir); err != nil {
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
	hooksDir := setupTestHooksDir(t)
	settingsDir := t.TempDir()
	settingsFile := filepath.Join(settingsDir, ".claude", "settings.json")

	origFn := settingsPathFn
	settingsPathFn = func() string { return settingsFile }
	defer func() { settingsPathFn = origFn }()

	// Write existing settings with a custom hook
	os.MkdirAll(filepath.Dir(settingsFile), 0755)
	os.WriteFile(settingsFile, []byte(`{
		"hooks": {
			"SessionStart": [
				{"matcher": "", "hooks": [{"type": "command", "command": "/other/hook.sh"}]}
			]
		}
	}`), 0644)

	if err := Run(hooksDir); err != nil {
		t.Fatal(err)
	}

	m := readJSON(t, settingsFile)
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	var entries []hookEntry
	json.Unmarshal(hooks["SessionStart"], &entries)
	// Should have both: existing + newly added
	if len(entries) != 2 {
		t.Fatalf("SessionStart: expected 2 entries, got %d", len(entries))
	}
}
