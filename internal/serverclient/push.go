package serverclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ishii1648/agent-telemetry/internal/agent"
	"github.com/ishii1648/agent-telemetry/internal/backfill"
	"github.com/ishii1648/agent-telemetry/internal/syncdb"
)

// MaxBatchBytes is the per-request hard cap from docs/spec.md. Kept as a var
// so tests can shrink it without producing massive fixtures.
var MaxBatchBytes = 50 * 1024 * 1024

// gzipThreshold is the payload size above which we compress. Below this,
// gzip overhead (~30 bytes minimum) is comparable to the savings on small
// JSON, so we send raw.
const gzipThreshold = 4 * 1024

// requestTimeout caps each HTTP attempt. Server work is dumb upsert into
// SQLite, so anything beyond this points at network trouble.
const requestTimeout = 60 * time.Second

// Options configure a Run invocation. ClientVersion is stamped onto every
// payload so the server can correlate ingest patterns with releases.
type Options struct {
	ClientVersion string
	SinceLast     bool // when true, send only diffs vs state.PushedSessionVersions
	Full          bool // when true, ignore the version map and resend everything
	DryRun        bool // when true, compute counts/sizes only — no network call
	AgentName     string
	DBPath        string
	ConfigPath    string
	HTTPClient    *http.Client
}

// Result reports per-agent push outcomes. PerAgent indexes are stable across
// runs (agent name as map key) so callers can join with their own logs.
type Result struct {
	PerAgent map[string]*AgentResult
}

// AgentResult captures what was sent (or would be sent in dry-run) for one
// agent. PayloadBytes is the wire size after gzip when applicable.
type AgentResult struct {
	Eligible          int
	Sent              int
	Skipped           int
	Batches           int
	PayloadBytes      int64
	ReceivedSessions  int
	ServerSkipped     int
	SchemaMismatch    bool
	NoConfig          bool
	StateUpdated      bool
	DryRun            bool
}

// ErrSchemaMismatch is returned when the server reports a schema_hash
// disagreement. The CLI surfaces this with a non-zero exit so users notice
// they need to upgrade either side.
var ErrSchemaMismatch = errors.New("schema_mismatch: client and server schemas disagree — upgrade binaries")

// Run executes the push pipeline for the agents resolved from opts.AgentName.
// Missing [server] config is reported via Result and logged but is NOT an
// error — cron should not page on an opt-out.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.DBPath == "" {
		opts.DBPath = syncdb.DBPath()
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = ConfigPath()
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	// --since-last is the documented default. main.go normally sets one of
	// the two flags; this guard keeps direct library callers honest.
	if !opts.Full && !opts.SinceLast {
		opts.SinceLast = true
	}

	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	agents, err := agent.ResolveOrDetect(opts.AgentName)
	if err != nil {
		return nil, fmt.Errorf("resolve agent: %w", err)
	}

	db, err := sql.Open("sqlite", opts.DBPath+"?_pragma=busy_timeout(30000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	res := &Result{PerAgent: make(map[string]*AgentResult, len(agents))}
	var firstErr error
	for _, a := range agents {
		ar, err := runForAgent(ctx, db, a, cfg, opts)
		res.PerAgent[a.Name] = ar
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return res, firstErr
}

func runForAgent(ctx context.Context, db *sql.DB, a *agent.Agent, cfg ServerConfig, opts Options) (*AgentResult, error) {
	ar := &AgentResult{DryRun: opts.DryRun}

	pairs, err := LoadPairs(db, a.Name)
	if err != nil {
		return ar, fmt.Errorf("load pairs[%s]: %w", a.Name, err)
	}
	ar.Eligible = len(pairs)

	state, err := backfill.LoadState(a.StatePath())
	if err != nil {
		return ar, fmt.Errorf("load state[%s]: %w", a.Name, err)
	}
	if state.PushedSessionVersions == nil {
		state.PushedSessionVersions = map[string]string{}
	}

	changed, hashes, err := SelectChanged(pairs, state.PushedSessionVersions, opts.Full)
	if err != nil {
		return ar, err
	}
	ar.Sent = len(changed)
	ar.Skipped = ar.Eligible - ar.Sent

	if !cfg.Configured() {
		ar.NoConfig = true
		// Still report what would have been sent so dry-run / cron logs are
		// useful, but skip the network call entirely.
		batches, _ := SplitBatches(changed, opts.ClientVersion, schemaHash(), MaxBatchBytes)
		ar.Batches = len(batches)
		for _, b := range batches {
			body, _ := json.Marshal(b)
			ar.PayloadBytes += int64(len(body))
		}
		return ar, nil
	}

	batches, err := SplitBatches(changed, opts.ClientVersion, schemaHash(), MaxBatchBytes)
	if err != nil {
		return ar, err
	}
	ar.Batches = len(batches)

	if opts.DryRun {
		for _, b := range batches {
			body, _ := json.Marshal(b)
			ar.PayloadBytes += int64(len(body))
		}
		return ar, nil
	}

	endpoint, err := metricsURL(cfg.Endpoint)
	if err != nil {
		return ar, err
	}

	for _, b := range batches {
		resp, sentBytes, err := postBatch(ctx, opts.HTTPClient, endpoint, cfg.Token, b)
		ar.PayloadBytes += int64(sentBytes)
		if err != nil {
			return ar, err
		}
		ar.ReceivedSessions += resp.ReceivedSessions
		ar.ServerSkipped += resp.SkippedSessions
		if resp.SchemaMismatch {
			ar.SchemaMismatch = true
			return ar, ErrSchemaMismatch
		}
	}

	for _, p := range changed {
		state.PushedSessionVersions[p.Key()] = hashes[p.Key()]
	}
	if err := backfill.SaveState(a.StatePath(), state); err != nil {
		return ar, fmt.Errorf("save state[%s]: %w", a.Name, err)
	}
	ar.StateUpdated = true
	return ar, nil
}

// metricsURL appends /v1/metrics to the configured base, normalizing trailing
// slashes. Doing this here (rather than asking the user to type the path)
// lets us evolve the path without a config migration.
func metricsURL(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/metrics"
	return u.String(), nil
}

func postBatch(ctx context.Context, client *http.Client, endpoint, token string, p Payload) (Response, int, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return Response{}, 0, fmt.Errorf("marshal payload: %w", err)
	}

	var reqBody io.Reader = bytes.NewReader(body)
	contentEncoding := ""
	wireSize := len(body)
	if len(body) >= gzipThreshold {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(body); err != nil {
			return Response{}, 0, fmt.Errorf("gzip write: %w", err)
		}
		if err := gz.Close(); err != nil {
			return Response{}, 0, fmt.Errorf("gzip close: %w", err)
		}
		reqBody = &buf
		contentEncoding = "gzip"
		wireSize = buf.Len()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reqBody)
	if err != nil {
		return Response{}, 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, wireSize, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Response{}, wireSize, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return Response{}, wireSize, fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var r Response
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &r); err != nil {
			return Response{}, wireSize, fmt.Errorf("decode response: %w", err)
		}
	}
	return r, wireSize, nil
}

// schemaHash exposes internal/syncdb's embedded schema hash. Wrapped in a
// function so tests can stub if needed (currently they don't).
func schemaHash() string {
	return syncdb.SchemaHash()
}

// Summarize renders a result for stderr / dry-run output. The shape is meant
// to be human-readable in cron mail rather than machine-parsed.
func (r *Result) Summarize(w io.Writer) {
	names := make([]string, 0, len(r.PerAgent))
	for name := range r.PerAgent {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ar := r.PerAgent[name]
		switch {
		case ar.NoConfig:
			fmt.Fprintf(w, "push[%s]: [server] 設定なし — eligible=%d sent=0 (skipped network)\n", name, ar.Eligible)
		case ar.DryRun:
			fmt.Fprintf(w, "push[%s] dry-run: eligible=%d sent=%d skipped=%d batches=%d payload=%d bytes\n",
				name, ar.Eligible, ar.Sent, ar.Skipped, ar.Batches, ar.PayloadBytes)
		case ar.SchemaMismatch:
			fmt.Fprintf(w, "push[%s]: schema_mismatch — server rejected payload, upgrade binaries\n", name)
		default:
			fmt.Fprintf(w, "push[%s]: eligible=%d sent=%d skipped=%d batches=%d payload=%d bytes received=%d\n",
				name, ar.Eligible, ar.Sent, ar.Skipped, ar.Batches, ar.PayloadBytes, ar.ReceivedSessions)
		}
	}
}
