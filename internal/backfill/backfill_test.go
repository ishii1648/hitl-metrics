package backfill

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestIndex(t *testing.T, dir string, lines []string) string {
	t.Helper()
	p := filepath.Join(dir, "session-index.jsonl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	f.Close()
	return p
}

func TestRunWithState_CursorAdvances(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":true}`,
		`{"timestamp":"2026-03-01 11:00:00","session_id":"s2","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/2"],"transcript":"","parent_session_id":"","backfill_checked":true}`,
	})
	statePath := filepath.Join(dir, "state.json")

	// First run: cursor should advance to 2
	err := RunWithState(indexPath, statePath, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.LastBackfillOffset != 2 {
		t.Fatalf("expected offset 2, got %d", state.LastBackfillOffset)
	}
}

func TestRunWithState_CursorSkipsProcessed(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":true}`,
	})
	statePath := filepath.Join(dir, "state.json")

	// Pre-set cursor to 1 (already processed)
	SaveState(statePath, State{
		LastBackfillOffset: 1,
		LastMetaCheck:      time.Now(),
	})

	err := RunWithState(indexPath, statePath, false)
	if err != nil {
		t.Fatal(err)
	}

	state, _ := LoadState(statePath)
	if state.LastBackfillOffset != 1 {
		t.Fatalf("expected offset 1, got %d", state.LastBackfillOffset)
	}
}

func TestRunWithState_RecheckResetsCursor(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":true}`,
	})
	statePath := filepath.Join(dir, "state.json")

	// Pre-set cursor
	SaveState(statePath, State{LastBackfillOffset: 1, LastMetaCheck: time.Now()})

	// --recheck should reset cursor to 0 and process all
	err := RunWithState(indexPath, statePath, true)
	if err != nil {
		t.Fatal(err)
	}

	state, _ := LoadState(statePath)
	if state.LastBackfillOffset != 1 {
		t.Fatalf("expected offset 1 after recheck, got %d", state.LastBackfillOffset)
	}
}

func TestRunWithState_Phase2SkippedIfRecent(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":true,"is_merged":false}`,
	})
	statePath := filepath.Join(dir, "state.json")

	recentTime := time.Now().Add(-10 * time.Minute)
	SaveState(statePath, State{
		LastBackfillOffset: 0,
		LastMetaCheck:      recentTime,
	})

	err := RunWithState(indexPath, statePath, false)
	if err != nil {
		t.Fatal(err)
	}

	state, _ := LoadState(statePath)
	// LastMetaCheck should NOT be updated (Phase 2 was skipped)
	if state.LastMetaCheck.After(recentTime.Add(time.Second)) {
		t.Fatalf("Phase 2 should have been skipped, but LastMetaCheck was updated to %v", state.LastMetaCheck)
	}
}

func TestRunWithState_Phase2RunsAfterInterval(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":true,"is_merged":false}`,
	})
	statePath := filepath.Join(dir, "state.json")

	// Set last check to 2 hours ago
	oldTime := time.Now().Add(-2 * time.Hour)
	SaveState(statePath, State{
		LastBackfillOffset: 0,
		LastMetaCheck:      oldTime,
	})

	before := time.Now()
	err := RunWithState(indexPath, statePath, false)
	if err != nil {
		t.Fatal(err)
	}

	state, _ := LoadState(statePath)
	// LastMetaCheck should be updated (Phase 2 ran)
	if state.LastMetaCheck.Before(before) {
		t.Fatalf("Phase 2 should have run, but LastMetaCheck was not updated")
	}
}

func TestRunWithState_IndexNotExist(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "nonexistent.jsonl")
	statePath := filepath.Join(dir, "state.json")

	err := RunWithState(indexPath, statePath, false)
	if err != nil {
		t.Fatalf("expected nil error for missing index, got: %v", err)
	}
}

func TestRunWithState_CursorBeyondLength(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":true}`,
	})
	statePath := filepath.Join(dir, "state.json")

	// Cursor beyond current length (e.g., entries were removed)
	SaveState(statePath, State{LastBackfillOffset: 999, LastMetaCheck: time.Now()})

	err := RunWithState(indexPath, statePath, false)
	if err != nil {
		t.Fatal(err)
	}

	state, _ := LoadState(statePath)
	// Should reset to current length
	if state.LastBackfillOffset != 1 {
		t.Fatalf("expected offset 1 after reset, got %d", state.LastBackfillOffset)
	}
}
