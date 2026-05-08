package userid

import (
	"os"
	"path/filepath"
	"testing"
)

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
