package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionIndexPathPerAgent(t *testing.T) {
	c := &Agent{Name: NameClaude, DataDir: "/home/x/.claude"}
	if got := c.SessionIndexPath(); got != "/home/x/.claude/session-index.jsonl" {
		t.Errorf("claude SessionIndexPath = %q", got)
	}
	x := &Agent{Name: NameCodex, DataDir: "/home/x/.codex"}
	if got := x.SessionIndexPath(); got != "/home/x/.codex/session-index.jsonl" {
		t.Errorf("codex SessionIndexPath = %q", got)
	}
}

func TestByName(t *testing.T) {
	if a, err := ByName(NameClaude); err != nil || a.Name != NameClaude {
		t.Errorf("ByName(claude) = %+v, %v", a, err)
	}
	if a, err := ByName(NameCodex); err != nil || a.Name != NameCodex {
		t.Errorf("ByName(codex) = %+v, %v", a, err)
	}
	if _, err := ByName("opus"); err == nil {
		t.Error("ByName(opus) should error")
	}
	if _, err := ByName(""); err == nil {
		t.Error("ByName(\"\") should error")
	}
}

func TestResolveFallsBackToClaude(t *testing.T) {
	t.Setenv(EnvVar, "")
	a, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != NameClaude {
		t.Errorf("default = %q, want claude", a.Name)
	}
}

func TestResolveFlagOverridesEnv(t *testing.T) {
	t.Setenv(EnvVar, NameClaude)
	a, err := Resolve(NameCodex)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != NameCodex {
		t.Errorf("flag should win: got %q", a.Name)
	}
}

func TestResolveEnvWhenFlagEmpty(t *testing.T) {
	t.Setenv(EnvVar, NameCodex)
	a, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != NameCodex {
		t.Errorf("env should win when flag empty: got %q", a.Name)
	}
}

func TestPresentChecksSessionIndexOrDir(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{Name: NameClaude, DataDir: filepath.Join(dir, "missing")}
	if Present(a) {
		t.Error("missing dir should not be present")
	}

	have := &Agent{Name: NameCodex, DataDir: dir}
	if !Present(have) {
		t.Error("existing dir should be present")
	}

	// session-index alone is also enough
	dataDir := filepath.Join(dir, "alt")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	idx := &Agent{Name: NameClaude, DataDir: dataDir}
	if !Present(idx) {
		t.Error("dir alone should be present")
	}
}

func TestCodexHomeRespectsEnv(t *testing.T) {
	t.Setenv("CODEX_HOME", "/tmp/codex-custom")
	if got := codexHome(); got != "/tmp/codex-custom" {
		t.Errorf("CODEX_HOME ignored: got %q", got)
	}
}
