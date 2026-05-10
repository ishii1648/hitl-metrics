package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ishii1648/agent-telemetry/internal/agent"
)

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
