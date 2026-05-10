package serverclient

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ishii1648/agent-telemetry/internal/backfill"
	"github.com/ishii1648/agent-telemetry/internal/syncdb/schema"
)

// pushTestEnv wires up a temp ~/.claude (and ~/.codex) layout, a fresh DB
// with the production schema, a fake server, and a minimal toml file.
type pushTestEnv struct {
	t            *testing.T
	home         string
	dbPath       string
	configPath   string
	server       *httptest.Server
	requests     atomic.Int32
	lastPayloads []Payload
	mismatch     bool
}

func newPushTestEnv(t *testing.T) *pushTestEnv {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, ".config", "agent-telemetry")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	env := &pushTestEnv{
		t:          t,
		home:       home,
		dbPath:     filepath.Join(home, ".claude", "agent-telemetry.db"),
		configPath: filepath.Join(configDir, "config.toml"),
	}
	env.server = httptest.NewServer(http.HandlerFunc(env.handle))
	t.Cleanup(env.server.Close)
	return env
}

func (e *pushTestEnv) handle(w http.ResponseWriter, r *http.Request) {
	e.requests.Add(1)
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		http.Error(w, "auth", http.StatusUnauthorized)
		return
	}
	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer gz.Close()
		body = gz
	}
	var p Payload
	if err := json.NewDecoder(body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	e.lastPayloads = append(e.lastPayloads, p)
	resp := Response{
		ReceivedSessions: len(p.Sessions),
		SchemaMismatch:   e.mismatch,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (e *pushTestEnv) writeConfig(extra string) {
	e.t.Helper()
	body := "[server]\nendpoint = \"" + e.server.URL + "\"\ntoken = \"test-token\"\n" + extra
	if err := os.WriteFile(e.configPath, []byte(body), 0644); err != nil {
		e.t.Fatal(err)
	}
}

// seedSession inserts one (session, stats) pair into the DB. Tests call this
// directly rather than going through sync-db so they can construct edge cases
// (in-progress, post-backfill, etc.) cheaply.
func (e *pushTestEnv) seedSession(sessionID, codingAgent string, mut func(*SessionRow, *StatsRow)) {
	e.t.Helper()
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(schemaSQLForTest()); err != nil {
		e.t.Fatal(err)
	}
	if _, err := db.Exec("INSERT OR REPLACE INTO schema_meta (key, value) VALUES ('schema_hash', ?)", schema.Hash); err != nil {
		e.t.Fatal(err)
	}

	s := SessionRow{
		SessionID:    sessionID,
		CodingAgent:  codingAgent,
		AgentVersion: "1.0.0",
		UserID:       "alice@example.com",
		Timestamp:    "2026-05-10 10:00:00",
		Repo:         "ishii1648/agent-telemetry",
		Branch:       "feat/x",
		PRURL:        "https://github.com/ishii1648/agent-telemetry/pull/42",
		PRTitle:      "feat: x",
		Transcript:   "/tmp/" + sessionID + ".jsonl",
		EndedAt:      "2026-05-10 11:00:00",
		EndReason:    "exit",
		TaskType:     "feat",
	}
	st := StatsRow{
		SessionID:    sessionID,
		CodingAgent:  codingAgent,
		ToolUseTotal: 5,
		InputTokens:  100,
		OutputTokens: 50,
		Model:        "claude-sonnet",
	}
	if mut != nil {
		mut(&s, &st)
	}
	if _, err := db.Exec(`INSERT OR REPLACE INTO sessions (session_id, coding_agent, agent_version, user_id, timestamp, cwd, repo, branch, pr_url, pr_title, transcript, parent_session_id, ended_at, end_reason, is_subagent, backfill_checked, is_merged, task_type, review_comments, changes_requested) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.CodingAgent, s.AgentVersion, s.UserID, s.Timestamp, s.CWD, s.Repo, s.Branch,
		s.PRURL, s.PRTitle, s.Transcript, s.ParentSessionID, s.EndedAt, s.EndReason,
		s.IsSubagent, s.BackfillChecked, s.IsMerged, s.TaskType, s.ReviewComments, s.ChangesRequested,
	); err != nil {
		e.t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT OR REPLACE INTO transcript_stats (session_id, coding_agent, tool_use_total, mid_session_msgs, ask_user_question, input_tokens, output_tokens, cache_write_tokens, cache_read_tokens, reasoning_tokens, model, is_ghost) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.SessionID, st.CodingAgent, st.ToolUseTotal, st.MidSessionMsgs, st.AskUserQuestion,
		st.InputTokens, st.OutputTokens, st.CacheWriteTokens, st.CacheReadTokens,
		st.ReasoningTokens, st.Model, st.IsGhost,
	); err != nil {
		e.t.Fatal(err)
	}
}

func (e *pushTestEnv) run(opts Options) (*Result, error) {
	if opts.DBPath == "" {
		opts.DBPath = e.dbPath
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = e.configPath
	}
	if opts.AgentName == "" {
		opts.AgentName = "claude"
	}
	return Run(context.Background(), opts)
}

func TestRun_DryRunReportsCountsAndSize(t *testing.T) {
	env := newPushTestEnv(t)
	env.writeConfig("")
	env.seedSession("s1", "claude", nil)

	res, err := env.run(Options{ClientVersion: "test", DryRun: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	ar := res.PerAgent["claude"]
	if ar.Eligible != 1 || ar.Sent != 1 {
		t.Errorf("dry-run counts: eligible=%d sent=%d", ar.Eligible, ar.Sent)
	}
	if ar.PayloadBytes == 0 {
		t.Error("dry-run should report payload size")
	}
	if env.requests.Load() != 0 {
		t.Errorf("dry-run should not call server, got %d requests", env.requests.Load())
	}
}

func TestRun_SinceLastSendsThenSkipsOnSecondCall(t *testing.T) {
	env := newPushTestEnv(t)
	env.writeConfig("")
	env.seedSession("s1", "claude", nil)

	first, err := env.run(Options{ClientVersion: "test", SinceLast: true})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.PerAgent["claude"].Sent != 1 {
		t.Fatalf("first sent: %d", first.PerAgent["claude"].Sent)
	}
	if env.requests.Load() != 1 {
		t.Errorf("first server requests: %d", env.requests.Load())
	}

	second, err := env.run(Options{ClientVersion: "test", SinceLast: true})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.PerAgent["claude"].Sent != 0 {
		t.Errorf("second sent: %d (expected 0 — diff should detect unchanged)", second.PerAgent["claude"].Sent)
	}
	if env.requests.Load() != 1 {
		t.Errorf("second should make no server call, requests=%d", env.requests.Load())
	}
}

func TestRun_BackfillUpdateTriggersResend(t *testing.T) {
	env := newPushTestEnv(t)
	env.writeConfig("")
	env.seedSession("s1", "claude", nil)
	env.seedSession("s2", "claude", nil)

	if _, err := env.run(Options{ClientVersion: "test", SinceLast: true}); err != nil {
		t.Fatal(err)
	}
	requestsAfterFirst := env.requests.Load()

	// Simulate backfill marking s1's PR as merged.
	env.seedSession("s1", "claude", func(s *SessionRow, _ *StatsRow) {
		s.IsMerged = 1
	})

	res, err := env.run(Options{ClientVersion: "test", SinceLast: true})
	if err != nil {
		t.Fatal(err)
	}
	ar := res.PerAgent["claude"]
	if ar.Sent != 1 {
		t.Errorf("expected only s1 to be resent, got %d", ar.Sent)
	}
	if env.requests.Load() != requestsAfterFirst+1 {
		t.Errorf("server should have been called once more, got delta %d", env.requests.Load()-requestsAfterFirst)
	}
	if got := env.lastPayloads[len(env.lastPayloads)-1].Sessions[0].SessionID; got != "s1" {
		t.Errorf("resent wrong session: %s", got)
	}
}

func TestRun_InProgressSessionExcluded(t *testing.T) {
	env := newPushTestEnv(t)
	env.writeConfig("")
	env.seedSession("ongoing", "claude", func(s *SessionRow, _ *StatsRow) {
		s.EndedAt = ""
		s.EndReason = ""
	})
	env.seedSession("done", "claude", nil)

	res, err := env.run(Options{ClientVersion: "test", SinceLast: true})
	if err != nil {
		t.Fatal(err)
	}
	ar := res.PerAgent["claude"]
	if ar.Eligible != 1 {
		t.Errorf("eligible should exclude in-progress, got %d", ar.Eligible)
	}
	if got := env.lastPayloads[0].Sessions[0].SessionID; got != "done" {
		t.Errorf("wrong session pushed: %s", got)
	}
}

func TestRun_MissingConfigExitsZeroWithWarning(t *testing.T) {
	env := newPushTestEnv(t)
	// no writeConfig
	env.seedSession("s1", "claude", nil)

	res, err := env.run(Options{ClientVersion: "test", SinceLast: true})
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	ar := res.PerAgent["claude"]
	if !ar.NoConfig {
		t.Errorf("expected NoConfig=true, got %+v", ar)
	}
	if env.requests.Load() != 0 {
		t.Errorf("missing config should make no calls, requests=%d", env.requests.Load())
	}
}

func TestRun_SchemaMismatchReturnsError(t *testing.T) {
	env := newPushTestEnv(t)
	env.writeConfig("")
	env.mismatch = true
	env.seedSession("s1", "claude", nil)

	res, err := env.run(Options{ClientVersion: "test", SinceLast: true})
	if err == nil {
		t.Fatal("expected error on schema_mismatch")
	}
	if !strings.Contains(err.Error(), "schema_mismatch") {
		t.Errorf("error should mention schema_mismatch: %v", err)
	}
	if !res.PerAgent["claude"].SchemaMismatch {
		t.Error("result should record schema mismatch")
	}
	// State should NOT be updated when server rejects.
	st, _ := backfill.LoadState(filepath.Join(env.home, ".claude", "agent-telemetry-state.json"))
	if len(st.PushedSessionVersions) != 0 {
		t.Errorf("state should not have updated on mismatch: %v", st.PushedSessionVersions)
	}
}

func TestRun_FullResendsSkippedSessions(t *testing.T) {
	env := newPushTestEnv(t)
	env.writeConfig("")
	env.seedSession("s1", "claude", nil)

	if _, err := env.run(Options{ClientVersion: "test", SinceLast: true}); err != nil {
		t.Fatal(err)
	}
	requestsAfterFirst := env.requests.Load()

	res, err := env.run(Options{ClientVersion: "test", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.PerAgent["claude"].Sent != 1 {
		t.Errorf("--full should resend, got Sent=%d", res.PerAgent["claude"].Sent)
	}
	if env.requests.Load() != requestsAfterFirst+1 {
		t.Errorf("--full should call server again, delta=%d", env.requests.Load()-requestsAfterFirst)
	}
}

// schemaSQLForTest exposes the embedded schema to tests so they can spin up
// production-shaped DBs without round-tripping through sync-db's full pipeline.
func schemaSQLForTest() string {
	return schema.SQL
}
