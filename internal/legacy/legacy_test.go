package legacy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateAll_renamesPresentLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "hitl-metrics.db")
	dst := filepath.Join(dir, "agent-telemetry.db")
	mustWrite(t, src, "data")

	moved, errs := migrateAll(
		[]Migration{{From: src, To: dst}},
		os.Rename,
		statExists,
	)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(moved) != 1 || moved[0].To != dst {
		t.Fatalf("expected one move to %s, got %+v", dst, moved)
	}
	if statExists(src) {
		t.Errorf("source %s should be gone", src)
	}
	if !statExists(dst) {
		t.Errorf("destination %s should exist", dst)
	}
}

func TestMigrateAll_skipsAbsentSource(t *testing.T) {
	dir := t.TempDir()
	moved, errs := migrateAll(
		[]Migration{{
			From: filepath.Join(dir, "missing"),
			To:   filepath.Join(dir, "new"),
		}},
		os.Rename,
		statExists,
	)
	if len(moved) != 0 || len(errs) != 0 {
		t.Fatalf("expected no-op, got moved=%v errs=%v", moved, errs)
	}
}

func TestMigrateAll_reportsConflictWhenBothExist(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old")
	dst := filepath.Join(dir, "new")
	mustWrite(t, src, "old")
	mustWrite(t, dst, "new")

	moved, errs := migrateAll(
		[]Migration{{From: src, To: dst}},
		os.Rename,
		statExists,
	)
	if len(moved) != 0 {
		t.Fatalf("expected no move on conflict, got %+v", moved)
	}
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %v", errs)
	}
}

func TestMigrateAll_renameErrorIsCollected(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	mustWrite(t, src, "x")

	boom := errors.New("rename failed")
	moved, errs := migrateAll(
		[]Migration{{From: src, To: dst}},
		func(string, string) error { return boom },
		statExists,
	)
	if len(moved) != 0 {
		t.Fatalf("expected no move on rename error")
	}
	if len(errs) != 1 || !errors.Is(errs[0], boom) {
		t.Fatalf("expected wrapped boom error, got %v", errs)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
