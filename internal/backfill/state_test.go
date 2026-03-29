package backfill

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadState_NotExist(t *testing.T) {
	s, err := LoadState("/nonexistent/path/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.LastBackfillOffset != 0 {
		t.Fatalf("expected offset 0, got %d", s.LastBackfillOffset)
	}
	if !s.LastMetaCheck.IsZero() {
		t.Fatalf("expected zero time, got %v", s.LastMetaCheck)
	}
}

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	now := time.Now().Truncate(time.Second)
	original := State{
		LastBackfillOffset: 42,
		LastMetaCheck:      now,
	}

	if err := SaveState(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.LastBackfillOffset != 42 {
		t.Fatalf("expected offset 42, got %d", loaded.LastBackfillOffset)
	}
	if !loaded.LastMetaCheck.Equal(now) {
		t.Fatalf("expected %v, got %v", now, loaded.LastMetaCheck)
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte("not json"), 0644)

	s, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	// Invalid JSON returns zero state (not an error)
	if s.LastBackfillOffset != 0 {
		t.Fatalf("expected offset 0, got %d", s.LastBackfillOffset)
	}
}

func TestSaveState_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := State{LastBackfillOffset: 10}
	if err := SaveState(path, s); err != nil {
		t.Fatal(err)
	}

	// File should exist and contain valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty file")
	}

	// No temp files should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "state.json" {
			t.Fatalf("unexpected file: %s", e.Name())
		}
	}
}
