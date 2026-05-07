package sessionindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestUpdate_PreservesAppendOrder(t *testing.T) {
	// Existing URLs are intentionally in non-alphabetical order to verify
	// that the existing sequence is preserved (not re-sorted).
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"main","pr_urls":["https://github.com/user/repo/pull/9","https://github.com/user/repo/pull/2"],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	// Add two more URLs. Existing order + append order must be preserved.
	updated, err := Update(p, "s1", []string{
		"https://github.com/user/repo/pull/2", // duplicate, ignored
		"https://github.com/user/repo/pull/5",
		"https://github.com/user/repo/pull/1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	want := []string{
		"https://github.com/user/repo/pull/9",
		"https://github.com/user/repo/pull/2",
		"https://github.com/user/repo/pull/5",
		"https://github.com/user/repo/pull/1",
	}
	if len(sessions[0].PRURLs) != len(want) {
		t.Fatalf("pr_urls length = %d, want %d (%v)", len(sessions[0].PRURLs), len(want), sessions[0].PRURLs)
	}
	for i, u := range want {
		if sessions[0].PRURLs[i] != u {
			t.Errorf("pr_urls[%d] = %q, want %q", i, sessions[0].PRURLs[i], u)
		}
	}

	// sync-db adopts the last URL — verify it matches the latest appended URL.
	last := sessions[0].PRURLs[len(sessions[0].PRURLs)-1]
	if last != "https://github.com/user/repo/pull/1" {
		t.Errorf("last pr_url = %q, want %q (sync-db adopts last)", last, "https://github.com/user/repo/pull/1")
	}
}

func TestUpdateByBranch_PreservesAppendOrder(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/9"],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := UpdateByBranch(p, "user/repo", "feat", "https://github.com/user/repo/pull/3")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	want := []string{
		"https://github.com/user/repo/pull/9",
		"https://github.com/user/repo/pull/3",
	}
	if len(sessions[0].PRURLs) != len(want) {
		t.Fatalf("pr_urls = %v, want %v", sessions[0].PRURLs, want)
	}
	for i, u := range want {
		if sessions[0].PRURLs[i] != u {
			t.Errorf("pr_urls[%d] = %q, want %q", i, sessions[0].PRURLs[i], u)
		}
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

	updated, err := UpdatePRMeta(p, "https://github.com/user/repo/pull/1", true, 4, 2, "fix: address review")
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
	if sessions[0].PRTitle != "fix: address review" {
		t.Fatalf("pr_title = %q, want %q", sessions[0].PRTitle, "fix: address review")
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

func TestPinPR_BindsAndReplacesPRURLs(t *testing.T) {
	// Existing pr_urls is a polluted set (PostToolUse regex grabbed two
	// unrelated PR URLs). Pin must REPLACE — not append — so the polluted
	// entries are dropped, and pr_pinned=true must be persisted.
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/9","https://github.com/user/other/pull/1"],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := PinPR(p, "s1", "https://github.com/user/repo/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	sessions := readSessions(t, p)
	if len(sessions[0].PRURLs) != 1 || sessions[0].PRURLs[0] != "https://github.com/user/repo/pull/42" {
		t.Fatalf("pr_urls = %v, want [pull/42]", sessions[0].PRURLs)
	}
	if !sessions[0].PRPinned {
		t.Fatal("pr_pinned should be true")
	}
}

func TestPinPR_IsIdempotent(t *testing.T) {
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/42"],"pr_pinned":true,"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := PinPR(p, "s1", "https://github.com/user/repo/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false (already pinned to same URL)")
	}
}

func TestPinPR_OverwritesDifferentURL(t *testing.T) {
	// If somehow pinned to a stale URL (e.g. PR was closed and a new one
	// reopened), re-pinning to the current URL must succeed.
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/1"],"pr_pinned":true,"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := PinPR(p, "s1", "https://github.com/user/repo/pull/2")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true (URL changed)")
	}
	sessions := readSessions(t, p)
	if sessions[0].PRURLs[0] != "https://github.com/user/repo/pull/2" {
		t.Fatalf("pr_urls[0] = %q, want pull/2", sessions[0].PRURLs[0])
	}
}

func TestUpdate_SkipsPinnedSession(t *testing.T) {
	// Once a session is pinned, regex-scraped URLs from PostToolUse must
	// NOT pollute pr_urls. This is the core bug we're fixing.
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/42"],"pr_pinned":true,"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := Update(p, "s1", []string{"https://github.com/user/other/pull/999"})
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false on pinned session")
	}
	sessions := readSessions(t, p)
	if len(sessions[0].PRURLs) != 1 || sessions[0].PRURLs[0] != "https://github.com/user/repo/pull/42" {
		t.Fatalf("pr_urls leaked update on pinned session: %v", sessions[0].PRURLs)
	}
}

func TestUpdateByBranch_SkipsPinnedSession(t *testing.T) {
	// Branch reuse scenario: a new PR was created on the same branch,
	// but the pinned session stays bound to its original PR.
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":["https://github.com/user/repo/pull/42"],"pr_pinned":true,"transcript":"","parent_session_id":"","backfill_checked":false}`,
		`{"timestamp":"2026-03-01 11:00:00","session_id":"s2","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})

	updated, err := UpdateByBranch(p, "user/repo", "feat", "https://github.com/user/repo/pull/100")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true (s2 should accept the URL)")
	}
	sessions := readSessions(t, p)
	if sessions[0].PRURLs[0] != "https://github.com/user/repo/pull/42" {
		t.Fatalf("s1 (pinned) pr_urls changed: %v", sessions[0].PRURLs)
	}
	if len(sessions[1].PRURLs) != 1 || sessions[1].PRURLs[0] != "https://github.com/user/repo/pull/100" {
		t.Fatalf("s2 (unpinned) pr_urls = %v, want [pull/100]", sessions[1].PRURLs)
	}
}

func TestPinPR_PreservesFieldOrder(t *testing.T) {
	// pr_pinned must be serialized between pr_urls and pr_title to match
	// remarshalWithUpdate's order map. This guards against accidentally
	// putting pr_pinned at the end of the JSON object on round-trip.
	p := writeTempJSONL(t, []string{
		`{"timestamp":"2026-03-01 10:00:00","session_id":"s1","cwd":"/tmp","repo":"user/repo","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":"","backfill_checked":false}`,
	})
	if _, err := PinPR(p, "s1", "https://github.com/user/repo/pull/42"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	urlsIdx := strings.Index(line, `"pr_urls"`)
	pinnedIdx := strings.Index(line, `"pr_pinned"`)
	transcriptIdx := strings.Index(line, `"transcript"`)
	if urlsIdx < 0 || pinnedIdx < 0 || transcriptIdx < 0 {
		t.Fatalf("missing field in serialized line: %q", line)
	}
	if !(urlsIdx < pinnedIdx && pinnedIdx < transcriptIdx) {
		t.Errorf("field order broken: pr_urls=%d pr_pinned=%d transcript=%d in %q",
			urlsIdx, pinnedIdx, transcriptIdx, line)
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
