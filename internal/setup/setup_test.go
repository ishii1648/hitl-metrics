package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ishii1648/hitl-metrics/internal/agent"
)

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

// Run() never modifies settings.json — same contract as the old install.
func TestRun_DoesNotModifySettings(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, "settings.json")
	SetSettingsPathForTest(t, settingsFile)

	original := []byte(`{"model":"sonnet"}`)
	if err := os.WriteFile(settingsFile, original, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Run(nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(settingsFile)
	if string(got) != string(original) {
		t.Fatalf("Run modified settings.json:\nbefore: %s\nafter:  %s", original, got)
	}
}

func TestRunWith_ClaudeOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := RunWith(&buf, agent.Claude()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "session-start --agent claude") {
		t.Errorf("missing claude session-start example:\n%s", out)
	}
	if strings.Contains(out, "PostToolUse") {
		t.Error("Claude output should not include PostToolUse")
	}
}

func TestRunWith_CodexOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := RunWith(&buf, agent.Codex()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"session-start --agent codex",
		"stop --agent codex",
		"post-tool-use --agent codex",
		"SessionEnd イベントが無いため",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("codex output missing %q\n%s", want, out)
		}
	}
}

func TestRunWith_NilShowsBoth(t *testing.T) {
	var buf bytes.Buffer
	if err := RunWith(&buf, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Claude Code") || !strings.Contains(out, "Codex CLI") {
		t.Errorf("nil agent should show both:\n%s", out)
	}
}

func TestUninstall_RemovesHitlHooks(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	SetSettingsPathForTest(t, settingsFile)

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
	var hooks map[string]json.RawMessage
	json.Unmarshal(m["hooks"], &hooks)

	var ssEntries []hookEntry
	json.Unmarshal(hooks["SessionStart"], &ssEntries)
	if len(ssEntries) != 1 || ssEntries[0].Hooks[0].Command != "/other/script.sh" {
		t.Fatalf("unexpected SessionStart leftover: %+v", ssEntries)
	}
	if _, ok := hooks["Stop"]; ok {
		t.Errorf("Stop should be empty after uninstall")
	}
}

func TestUninstall_NoSettingsFile(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	SetSettingsPathForTest(t, settingsFile)

	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall on missing file should be no-op, got err=%v", err)
	}
}

func TestHookSpecsFor(t *testing.T) {
	if got := HookSpecsFor(agent.NameClaude); len(got) != len(ClaudeHookSpecs) {
		t.Errorf("claude specs count = %d", len(got))
	}
	if got := HookSpecsFor(agent.NameCodex); len(got) != len(CodexHookSpecs) {
		t.Errorf("codex specs count = %d", len(got))
	}
	if got := HookSpecsFor(""); len(got) != len(ClaudeHookSpecs) {
		t.Errorf("default → claude specs, got %d", len(got))
	}
}
