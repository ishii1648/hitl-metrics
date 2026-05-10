package serverclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ServerSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-telemetry.toml")
	body := `user = "alice@example.com"

[server]
endpoint = "https://telemetry.example.com"
token = "secret-token"
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Endpoint != "https://telemetry.example.com" {
		t.Errorf("endpoint: got %q", cfg.Endpoint)
	}
	if cfg.Token != "secret-token" {
		t.Errorf("token: got %q", cfg.Token)
	}
	if !cfg.Configured() {
		t.Error("Configured() = false")
	}
}

func TestLoadConfig_NoServerSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-telemetry.toml")
	if err := os.WriteFile(path, []byte(`user = "alice@example.com"`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Configured() {
		t.Errorf("Configured()=true on missing section: %+v", cfg)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Configured() {
		t.Error("missing file produced configured cfg")
	}
}

func TestLoadConfig_UnknownSectionIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-telemetry.toml")
	body := `[other]
endpoint = "https://wrong"
[server]
endpoint = "https://right"
token = "tok"
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != "https://right" {
		t.Errorf("endpoint: got %q", cfg.Endpoint)
	}
}

func TestLoadConfig_PartialSectionNotConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-telemetry.toml")
	body := `[server]
endpoint = "https://only-endpoint"
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Configured() {
		t.Error("partial config should not be Configured()")
	}
}
