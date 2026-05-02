package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ishii1648/hitl-metrics/internal/setup"
)

// withSettingsPath swaps the underlying setup package's path lookup.
func withSettingsPath(t *testing.T, path string) {
	t.Helper()
	setup.SetSettingsPathForTest(t, path)
}

// install.Run is a deprecated alias that just delegates to setup.Run and
// MUST NOT modify settings.json.
func TestRunAliasIsNoOp(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	if err := Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settingsFile); !os.IsNotExist(err) {
		t.Fatalf("install.Run should not create settings.json, got err=%v", err)
	}
}

// install.Uninstall must still remove a legacy single-hook entry.
func TestUninstallAliasRemovesHook(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	withSettingsPath(t, settingsFile)

	body := `{
		"hooks": {
			"Stop": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook stop"}]}
			]
		}
	}`
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsFile, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == body {
		t.Fatalf("Uninstall did not modify settings.json:\n%s", got)
	}
}
