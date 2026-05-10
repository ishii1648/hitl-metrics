package configpath

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeHome points HOME (and clears XDG_CONFIG_HOME) at a temp dir so
// Preferred()/Legacy() resolve under it. Returns the path roots so the
// test can write fixture files.
func withFakeHome(t *testing.T) (home, preferred, legacy string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	preferred = filepath.Join(home, ".config", "agent-telemetry", "config.toml")
	legacy = filepath.Join(home, ".claude", "agent-telemetry.toml")
	return home, preferred, legacy
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve_PreferredOnly(t *testing.T) {
	_, preferred, _ := withFakeHome(t)
	writeFile(t, preferred, "user = \"a@b\"")

	var buf bytes.Buffer
	restore := SetWarnWriterForTest(&buf)
	defer restore()

	if got := Resolve(); got != preferred {
		t.Errorf("Resolve = %q, want %q", got, preferred)
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected warning: %q", buf.String())
	}
}

func TestResolve_LegacyOnly_EmitsWarningOnce(t *testing.T) {
	_, preferred, legacy := withFakeHome(t)
	writeFile(t, legacy, "user = \"a@b\"")

	var buf bytes.Buffer
	restore := SetWarnWriterForTest(&buf)
	defer restore()

	for i := 0; i < 3; i++ {
		if got := Resolve(); got != legacy {
			t.Errorf("Resolve #%d = %q, want %q", i, got, legacy)
		}
	}
	out := buf.String()
	if strings.Count(out, "agent-telemetry: reading config") != 1 {
		t.Errorf("warning should fire exactly once, got: %q", out)
	}
	if !strings.Contains(out, legacy) || !strings.Contains(out, preferred) {
		t.Errorf("warning should mention both paths, got: %q", out)
	}
}

func TestResolve_BothExist_PreferredWins(t *testing.T) {
	_, preferred, legacy := withFakeHome(t)
	writeFile(t, preferred, "user = \"new\"")
	writeFile(t, legacy, "user = \"old\"")

	var buf bytes.Buffer
	restore := SetWarnWriterForTest(&buf)
	defer restore()

	if got := Resolve(); got != preferred {
		t.Errorf("Resolve = %q, want %q", got, preferred)
	}
	if buf.Len() != 0 {
		t.Errorf("warning should not fire when preferred exists, got: %q", buf.String())
	}
}

func TestResolve_NeitherExists_NoWarning(t *testing.T) {
	_, preferred, _ := withFakeHome(t)

	var buf bytes.Buffer
	restore := SetWarnWriterForTest(&buf)
	defer restore()

	if got := Resolve(); got != preferred {
		t.Errorf("Resolve = %q, want %q (preferred path even when absent)", got, preferred)
	}
	if buf.Len() != 0 {
		t.Errorf("warning should not fire when neither path exists, got: %q", buf.String())
	}
}

func TestPreferred_RespectsXDGConfigHome(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir()) // ensure HOME fallback isn't accidentally returned

	want := filepath.Join(xdg, "agent-telemetry", "config.toml")
	if got := Preferred(); got != want {
		t.Errorf("Preferred = %q, want %q", got, want)
	}
}

func TestStatus_ReportsExistenceWithoutWarning(t *testing.T) {
	_, preferred, legacy := withFakeHome(t)
	writeFile(t, legacy, "")

	var buf bytes.Buffer
	restore := SetWarnWriterForTest(&buf)
	defer restore()

	st := Status()
	if st.Preferred != preferred || st.Legacy != legacy {
		t.Errorf("Status paths: got (%q, %q)", st.Preferred, st.Legacy)
	}
	if st.PreferredExists {
		t.Error("PreferredExists should be false")
	}
	if !st.LegacyExists {
		t.Error("LegacyExists should be true")
	}
	if buf.Len() != 0 {
		t.Errorf("Status must not emit warning, got: %q", buf.String())
	}
}
