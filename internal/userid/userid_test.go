package userid

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ishii1648/agent-telemetry/internal/configpath"
)

// configpathSilenceWarning silences the migration warning that
// configpath.Resolve emits when only the legacy path exists. Returned
// closure restores the prior writer.
func configpathSilenceWarning() func() {
	return configpath.SetWarnWriterForTest(io.Discard)
}

func TestResolveEnvBeatsConfig(t *testing.T) {
	t.Setenv(EnvVar, "env-user@example.com")

	dir := t.TempDir()
	path := filepath.Join(dir, "agent-telemetry.toml")
	if err := os.WriteFile(path, []byte(`user = "config-user@example.com"`), 0644); err != nil {
		t.Fatal(err)
	}

	got := readConfigUser(path)
	if got != "config-user@example.com" {
		t.Fatalf("readConfigUser: got %q, want config-user@example.com", got)
	}

	id, src := Resolve()
	if id != "env-user@example.com" {
		t.Errorf("Resolve id: got %q, want env-user@example.com", id)
	}
	if src != SourceEnv {
		t.Errorf("Resolve src: got %q, want %q", src, SourceEnv)
	}
}

func TestResolveEnvWhitespaceTrimmed(t *testing.T) {
	t.Setenv(EnvVar, "  spaced@example.com  ")
	id, src := Resolve()
	if id != "spaced@example.com" {
		t.Errorf("got %q, want spaced@example.com", id)
	}
	if src != SourceEnv {
		t.Errorf("src %q", src)
	}
}

func TestResolveEnvEmptyFallsThrough(t *testing.T) {
	t.Setenv(EnvVar, "")
	// We can't easily neutralize git/config in a unit test without
	// hermetic isolation, so just check that an empty env doesn't
	// short-circuit to "" or claim SourceEnv.
	id, src := Resolve()
	if src == SourceEnv {
		t.Errorf("empty env shouldn't be reported as SourceEnv (id=%q src=%q)", id, src)
	}
}

func TestReadConfigUser(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"quoted", `user = "alice@example.com"`, "alice@example.com"},
		{"unquoted", `user = bob@example.com`, "bob@example.com"},
		{"with comment", `user = "carol@example.com" # primary`, "carol@example.com"},
		{"comment line then key", "# comment\nuser = \"dan@example.com\"", "dan@example.com"},
		{"empty file", ``, ""},
		{"missing key", `other = "x"`, ""},
		{"section ignored", "[server]\nuser = \"in-section@example.com\"", ""},
		{"key before section honored", "user = \"top@example.com\"\n[server]\nuser = \"x\"", "top@example.com"},
		{"malformed line", `notakey`, ""},
		{"empty value", `user = `, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "agent-telemetry.toml")
			if err := os.WriteFile(path, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			got := readConfigUser(path)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadConfigUserMissingFile(t *testing.T) {
	got := readConfigUser(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

// TestResolve_ReadsFromXDGPath confirms the precedence chain delegates to
// configpath.Resolve() so the new XDG path is preferred. configpath has
// its own coverage for the fallback semantics; this test only verifies
// the wiring.
func TestResolve_ReadsFromXDGPath(t *testing.T) {
	t.Setenv(EnvVar, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := filepath.Join(home, ".config", "agent-telemetry")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"),
		[]byte(`user = "xdg-user@example.com"`), 0644); err != nil {
		t.Fatal(err)
	}

	id, src := Resolve()
	if id != "xdg-user@example.com" {
		t.Errorf("Resolve id: got %q, want xdg-user@example.com", id)
	}
	if src != SourceConfig {
		t.Errorf("Resolve src: got %q, want %q", src, SourceConfig)
	}
}

// TestResolve_ReadsFromLegacyPath confirms that a config under the old
// ~/.claude location is still picked up (with the migration warning fired
// from configpath, suppressed here so test output stays clean).
func TestResolve_ReadsFromLegacyPath(t *testing.T) {
	t.Setenv(EnvVar, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent-telemetry.toml"),
		[]byte(`user = "legacy-user@example.com"`), 0644); err != nil {
		t.Fatal(err)
	}

	restore := configpathSilenceWarning()
	defer restore()

	id, src := Resolve()
	if id != "legacy-user@example.com" {
		t.Errorf("Resolve id: got %q, want legacy-user@example.com", id)
	}
	if src != SourceConfig {
		t.Errorf("Resolve src: got %q, want %q", src, SourceConfig)
	}
}

func TestSplitKV(t *testing.T) {
	cases := []struct {
		in     string
		k, v   string
		wantOK bool
	}{
		{`user = "x"`, "user", `"x"`, true},
		{`  user="x"  `, "user", `"x"`, true},
		{`= "x"`, "", "", false},
		{`user`, "", "", false},
		{`user = x # comment`, "user", "x", true},
		{`user = "x # not a comment"`, "user", `"x # not a comment"`, true},
	}
	for _, tc := range cases {
		k, v, ok := splitKV(tc.in)
		if ok != tc.wantOK || k != tc.k || v != tc.v {
			t.Errorf("splitKV(%q) = (%q, %q, %v); want (%q, %q, %v)", tc.in, k, v, ok, tc.k, tc.v, tc.wantOK)
		}
	}
}
