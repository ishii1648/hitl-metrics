// Package legacy migrates files left over from the hitl-metrics era to
// their agent-telemetry counterparts. The repo was renamed in 2026-05;
// this package is the one-shot rename helper invoked early in commands
// that read or write these paths so users on the old layout don't have
// to run a separate migration step.
//
// All operations are best-effort and idempotent: a missing source is a
// no-op, an existing destination short-circuits without overwriting,
// and any error is returned to the caller for logging (the caller
// should not fail the command on a migration error — fall back to the
// new path and let the user re-run after fixing the conflict).
package legacy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ishii1648/agent-telemetry/internal/agent"
)

// Migration describes a single legacy → new path rename.
type Migration struct {
	From string
	To   string
}

// MigratedPath records a rename that actually happened, for reporting.
type MigratedPath struct {
	From string
	To   string
}

// Migrate moves every known legacy file to its agent-telemetry-era
// location. Returns the list of paths that were renamed and any errors
// encountered. Callers should print errors as warnings and continue.
func Migrate() ([]MigratedPath, []error) {
	return migrateAll(defaultMigrations(), os.Rename, statExists)
}

// defaultMigrations enumerates every legacy file the rename touches.
// SQLite WAL/SHM siblings are rolled in alongside the main DB so the
// move stays consistent.
func defaultMigrations() []Migration {
	home, _ := os.UserHomeDir()
	claude := filepath.Join(home, ".claude")
	codex := codexHome(home)

	out := []Migration{
		{From: filepath.Join(claude, "hitl-metrics.db"), To: filepath.Join(claude, "agent-telemetry.db")},
		{From: filepath.Join(claude, "hitl-metrics.db-wal"), To: filepath.Join(claude, "agent-telemetry.db-wal")},
		{From: filepath.Join(claude, "hitl-metrics.db-shm"), To: filepath.Join(claude, "agent-telemetry.db-shm")},
		{From: filepath.Join(claude, "hitl-metrics-state.json"), To: filepath.Join(claude, "agent-telemetry-state.json")},
		{From: filepath.Join(codex, "hitl-metrics-state.json"), To: filepath.Join(codex, "agent-telemetry-state.json")},
	}
	return out
}

func codexHome(home string) string {
	if env := os.Getenv("CODEX_HOME"); env != "" {
		return env
	}
	return filepath.Join(home, ".codex")
}

// statExists reports whether path exists on disk.
func statExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func migrateAll(plans []Migration, rename func(string, string) error, exists func(string) bool) ([]MigratedPath, []error) {
	var moved []MigratedPath
	var errs []error
	for _, p := range plans {
		if !exists(p.From) {
			continue
		}
		if exists(p.To) {
			errs = append(errs, fmt.Errorf("legacy migration: %s and %s both exist; remove one to proceed", p.From, p.To))
			continue
		}
		if err := rename(p.From, p.To); err != nil {
			errs = append(errs, fmt.Errorf("legacy migration: rename %s → %s: %w", p.From, p.To, err))
			continue
		}
		moved = append(moved, MigratedPath(p))
	}
	return moved, errs
}

// Report writes a one-line summary for each rename and a stderr-style
// note for each error. Safe to pass an io.Discard writer for tests.
func Report(w io.Writer, moved []MigratedPath, errs []error) {
	for _, m := range moved {
		fmt.Fprintf(w, "legacy migration: moved %s → %s\n", m.From, m.To)
	}
	for _, e := range errs {
		fmt.Fprintf(w, "legacy migration warning: %v\n", e)
	}
}

// LegacyArtifacts returns the full inventory of paths the migration
// inspects. Used by doctor to flag environments that still hold legacy
// files (independent of whether migration has run).
func LegacyArtifacts() []string {
	plans := defaultMigrations()
	out := make([]string, 0, len(plans))
	for _, p := range plans {
		out = append(out, p.From)
	}
	return out
}

// PresentLegacyPaths returns the subset of LegacyArtifacts that exist
// on disk right now. Doctor uses this to surface a "stop using" hint.
func PresentLegacyPaths() []string {
	var present []string
	for _, p := range LegacyArtifacts() {
		if statExists(p) {
			present = append(present, p)
		}
	}
	return present
}

// ErrConflict is returned when both the legacy and the new path exist.
// Callers can use errors.Is to detect this and prompt the user.
var ErrConflict = errors.New("both legacy and new paths exist")

// MigrateForAgent runs the migrations that affect a specific agent.
// Useful for hook subcommands that only touch a single data dir.
func MigrateForAgent(a *agent.Agent) ([]MigratedPath, []error) {
	plans := []Migration{
		{From: filepath.Join(a.DataDir, "hitl-metrics-state.json"), To: filepath.Join(a.DataDir, "agent-telemetry-state.json")},
	}
	if a.Name == agent.NameClaude {
		plans = append(plans,
			Migration{From: filepath.Join(a.DataDir, "hitl-metrics.db"), To: filepath.Join(a.DataDir, "agent-telemetry.db")},
			Migration{From: filepath.Join(a.DataDir, "hitl-metrics.db-wal"), To: filepath.Join(a.DataDir, "agent-telemetry.db-wal")},
			Migration{From: filepath.Join(a.DataDir, "hitl-metrics.db-shm"), To: filepath.Join(a.DataDir, "agent-telemetry.db-shm")},
		)
	}
	return migrateAll(plans, os.Rename, statExists)
}
