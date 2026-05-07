package hook

import (
	"errors"
	"os"
	"testing"

	"github.com/ishii1648/agent-telemetry/internal/agent"
	"github.com/ishii1648/agent-telemetry/internal/sessionindex"
)

// withPRLookup swaps the package-level prLookup with a fake for the
// duration of a test. Restores the original on cleanup so tests stay
// isolated.
func withPRLookup(t *testing.T, fake prLookupFn) {
	t.Helper()
	orig := prLookup
	prLookup = fake
	t.Cleanup(func() { prLookup = orig })
}

func TestPinPRForSession_BindsResolvedPR(t *testing.T) {
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}
	idx := a.SessionIndexPath()

	cwd := t.TempDir() // exists, satisfies isExistingDir

	if err := os.WriteFile(idx, []byte(
		`{"coding_agent":"claude","session_id":"s1","cwd":"`+cwd+`","repo":"u/r","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":""}`+"\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	withPRLookup(t, func(gotCwd, gotBranch string) (*prViewJSON, error) {
		if gotCwd != cwd {
			t.Errorf("cwd passed to lookup = %q, want %q", gotCwd, cwd)
		}
		if gotBranch != "feat" {
			t.Errorf("branch passed to lookup = %q, want feat", gotBranch)
		}
		return &prViewJSON{
			URL:      "https://github.com/u/r/pull/42",
			Title:    "feat: pin PR",
			State:    "OPEN",
			Comments: []any{struct{}{}, struct{}{}},
		}, nil
	})

	if err := pinPRForSession(a, &HookInput{SessionID: "s1"}); err != nil {
		t.Fatalf("pinPRForSession: %v", err)
	}

	_, sessions, err := sessionindex.ReadAll(idx)
	if err != nil {
		t.Fatal(err)
	}
	if !sessions[0].PRPinned {
		t.Fatal("pr_pinned should be true after pin")
	}
	if len(sessions[0].PRURLs) != 1 || sessions[0].PRURLs[0] != "https://github.com/u/r/pull/42" {
		t.Fatalf("pr_urls = %v, want [pull/42]", sessions[0].PRURLs)
	}
	if sessions[0].PRTitle != "feat: pin PR" {
		t.Errorf("pr_title = %q, want %q", sessions[0].PRTitle, "feat: pin PR")
	}
	if sessions[0].ReviewComments != 2 {
		t.Errorf("review_comments = %d, want 2", sessions[0].ReviewComments)
	}
}

func TestPinPRForSession_NoPRFound(t *testing.T) {
	// PR not yet created — pinPRForSession must leave the session alone
	// (pr_pinned stays false, pr_urls untouched) so backfill can pick it
	// up later.
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}
	idx := a.SessionIndexPath()

	cwd := t.TempDir()
	if err := os.WriteFile(idx, []byte(
		`{"coding_agent":"claude","session_id":"s1","cwd":"`+cwd+`","repo":"u/r","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":""}`+"\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	withPRLookup(t, func(_, _ string) (*prViewJSON, error) { return nil, nil })

	if err := pinPRForSession(a, &HookInput{SessionID: "s1"}); err != nil {
		t.Fatalf("pinPRForSession: %v", err)
	}

	_, sessions, err := sessionindex.ReadAll(idx)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].PRPinned {
		t.Fatal("pr_pinned must remain false when no PR is found")
	}
	if len(sessions[0].PRURLs) != 0 {
		t.Fatalf("pr_urls altered: %v", sessions[0].PRURLs)
	}
}

func TestPinPRForSession_LookupErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}
	idx := a.SessionIndexPath()

	cwd := t.TempDir()
	if err := os.WriteFile(idx, []byte(
		`{"coding_agent":"claude","session_id":"s1","cwd":"`+cwd+`","repo":"u/r","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":""}`+"\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	withPRLookup(t, func(_, _ string) (*prViewJSON, error) {
		return nil, errors.New("gh: not authenticated")
	})

	err := pinPRForSession(a, &HookInput{SessionID: "s1"})
	if err == nil {
		t.Fatal("expected error from lookup to propagate")
	}
}

func TestPinPRForSession_MissingCWDIsNoOp(t *testing.T) {
	// cwd has been deleted (worktree removed) — we cannot safely run gh
	// against the wrong directory, so skip without erroring.
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}
	idx := a.SessionIndexPath()

	if err := os.WriteFile(idx, []byte(
		`{"coding_agent":"claude","session_id":"s1","cwd":"/this/path/should/not/exist/abc123","repo":"u/r","branch":"feat","pr_urls":[],"transcript":"","parent_session_id":""}`+"\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	withPRLookup(t, func(_, _ string) (*prViewJSON, error) {
		called = true
		return nil, nil
	})

	if err := pinPRForSession(a, &HookInput{SessionID: "s1"}); err != nil {
		t.Fatalf("pinPRForSession: %v", err)
	}
	if called {
		t.Fatal("prLookup must not be called when cwd is missing")
	}

	_, sessions, _ := sessionindex.ReadAll(idx)
	if sessions[0].PRPinned {
		t.Fatal("pr_pinned must remain false")
	}
}

func TestPinPRForSession_AlreadyPinnedIsNoOp(t *testing.T) {
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}
	idx := a.SessionIndexPath()

	cwd := t.TempDir()
	if err := os.WriteFile(idx, []byte(
		`{"coding_agent":"claude","session_id":"s1","cwd":"`+cwd+`","repo":"u/r","branch":"feat","pr_urls":["https://github.com/u/r/pull/42"],"pr_pinned":true,"transcript":"","parent_session_id":""}`+"\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	withPRLookup(t, func(_, _ string) (*prViewJSON, error) {
		called = true
		return nil, nil
	})
	if err := pinPRForSession(a, &HookInput{SessionID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("prLookup must not run for already-pinned sessions")
	}
}

func TestPinPRForSession_EmptyBranchIsNoOp(t *testing.T) {
	// Sessions started outside a git repo have empty branch — pinning
	// would be meaningless and `gh pr list --head ''` is undefined.
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}
	idx := a.SessionIndexPath()

	cwd := t.TempDir()
	if err := os.WriteFile(idx, []byte(
		`{"coding_agent":"claude","session_id":"s1","cwd":"`+cwd+`","repo":"","branch":"","pr_urls":[],"transcript":"","parent_session_id":""}`+"\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	withPRLookup(t, func(_, _ string) (*prViewJSON, error) {
		called = true
		return nil, nil
	})
	if err := pinPRForSession(a, &HookInput{SessionID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("prLookup must not run when branch is empty")
	}
}

func TestPinPRForSession_NoIndexFileIsNoOp(t *testing.T) {
	// Fresh install with no session-index yet must not error.
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}

	if err := pinPRForSession(a, &HookInput{SessionID: "s1"}); err != nil {
		t.Fatalf("expected no error for missing index, got: %v", err)
	}
}
