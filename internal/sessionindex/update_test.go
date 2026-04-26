package sessionindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempJSONL(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
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

func readSessions(t *testing.T, path string) []Session {
	t.Helper()
	_, sessions, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	return sessions
}

func TestUpdate_AddURL(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := Update(p, "s1", []string{"https://github.com/user/repo/pull/1"})
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].PRURLs) != 1 || sessions[0].PRURLs[0] != "https://github.com/user/repo/pull/1" {
		t.Fatalf("unexpected pr_urls: %v", sessions[0].PRURLs)
	}
}

func TestUpdate_Deduplicate(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := Update(p, "s1", []string{"https://github.com/user/repo/pull/1"})
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false (duplicate URL)")
	}
}

func TestUpdate_NoMatch(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := Update(p, "s999", []string{"https://github.com/user/repo/pull/1"})
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false (no matching session)")
	}
}

func TestMarkChecked(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
		`{"timestamp":"2026-03-01 11:00:00","session_id":"s2","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := MarkChecked(p, []string{"s1"})
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	if !sessions[0].BackfillChecked {
		t.Fatal("s1 should be backfill_checked=true")
	}
	if sessions[1].BackfillChecked {
		t.Fatal("s2 should remain backfill_checked=false")
	}
}

func TestMarkChecked_AlreadyChecked(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":true}`,
	})

	updated, err := MarkChecked(p, []string{"s1"})
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false (already checked)")
	}
}

func TestUpdateByBranch(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
		`{"timestamp":"2026-03-01 11:00:00","session_id":"s2","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
		`{"timestamp":"2026-03-01 12:00:00","session_id":"s3","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := UpdateByBranch(p, "user/repo", "feat", "https://github.com/user/repo/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	// s1 and s2 should have the URL, s3 should not
	if len(sessions[0].PRURLs) != 1 {
		t.Fatalf("s1 pr_urls: %v", sessions[0].PRURLs)
	}
	if len(sessions[1].PRURLs) != 1 {
		t.Fatalf("s2 pr_urls: %v", sessions[1].PRURLs)
	}
	if len(sessions[2].PRURLs) != 0 {
		t.Fatalf("s3 should have no pr_urls: %v", sessions[2].PRURLs)
	}
}

func TestUpdateByBranch_NormalizesRepo(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo.git","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := UpdateByBranch(p, "user/repo", "feat", "https://github.com/user/repo/pull/1")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true (repo normalization)")
	}
}

func TestUpdatePRMeta_ChangesRequested(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := UpdatePRMeta(p, "https://github.com/user/repo/pull/1", true, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	if !sessions[0].IsMerged {
		t.Fatal("is_merged should be true")
	}
	if sessions[0].ReviewComments != 4 {
		t.Fatalf("review_comments = %d, want 4", sessions[0].ReviewComments)
	}
	if sessions[0].ChangesRequested != 2 {
		t.Fatalf("changes_requested = %d, want 2", sessions[0].ChangesRequested)
	}
}

func TestUpdateEnd(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := UpdateEnd(p, "s1", "2026-03-01 10:30:00", "prompt_input_exit")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	if sessions[0].EndedAt != "2026-03-01 10:30:00" {
		t.Fatalf("ended_at = %q", sessions[0].EndedAt)
	}
	if sessions[0].EndReason != "prompt_input_exit" {
		t.Fatalf("end_reason = %q", sessions[0].EndReason)
	}
}

func TestReadAll_PreservesExtraFields(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false,"extra_field":"keep_me"}`,
	})

	raws, _, err := ReadAll(p)
	if err != nil {
		t.Fatal(err)
	}

	// Verify extra field is preserved in raw
	var m map[string]json.RawMessage
	json.Unmarshal(raws[0], &m)
	if _, ok := m["extra_field"]; !ok {
		t.Fatal("extra_field should be preserved in raw JSON")
	}
}
