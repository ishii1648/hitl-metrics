package serverclient

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// SessionRow mirrors the sessions table columns documented in
// docs/spec.md ## SQLite データモデル. Field order MUST match the column order
// in schema.sql so json.Marshal produces a stable byte sequence — that
// determinism is what makes the SHA-256 hash a meaningful diff signal.
type SessionRow struct {
	SessionID        string `json:"session_id"`
	CodingAgent      string `json:"coding_agent"`
	AgentVersion     string `json:"agent_version"`
	UserID           string `json:"user_id"`
	Timestamp        string `json:"timestamp"`
	CWD              string `json:"cwd"`
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	PRURL            string `json:"pr_url"`
	PRTitle          string `json:"pr_title"`
	Transcript       string `json:"transcript"`
	ParentSessionID  string `json:"parent_session_id"`
	EndedAt          string `json:"ended_at"`
	EndReason        string `json:"end_reason"`
	IsSubagent       int    `json:"is_subagent"`
	BackfillChecked  int    `json:"backfill_checked"`
	IsMerged         int    `json:"is_merged"`
	TaskType         string `json:"task_type"`
	ReviewComments   int    `json:"review_comments"`
	ChangesRequested int    `json:"changes_requested"`
}

// StatsRow mirrors transcript_stats columns. Same determinism contract as
// SessionRow.
type StatsRow struct {
	SessionID        string `json:"session_id"`
	CodingAgent      string `json:"coding_agent"`
	ToolUseTotal     int    `json:"tool_use_total"`
	MidSessionMsgs   int    `json:"mid_session_msgs"`
	AskUserQuestion  int    `json:"ask_user_question"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	CacheWriteTokens int    `json:"cache_write_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens"`
	ReasoningTokens  int    `json:"reasoning_tokens"`
	Model            string `json:"model"`
	IsGhost          int    `json:"is_ghost"`
}

// SessionPair groups a session with its transcript_stats counterpart. The
// pair is the unit of diff detection — a change in either row invalidates
// the cached hash.
type SessionPair struct {
	Session SessionRow
	Stats   StatsRow
}

// Key returns "<coding_agent>:<session_id>", the composite key used for
// pushed_session_versions. The composite shape avoids cross-agent collisions
// described in issue 0028's 対応方針 section.
func (p SessionPair) Key() string {
	return p.Session.CodingAgent + ":" + p.Session.SessionID
}

// Hash returns the SHA-256 of the canonical JSON for (session, stats). The
// session and stats blobs are concatenated with a single 0x1F separator so
// shifting bytes between them can't produce a hash collision.
func (p SessionPair) Hash() (string, error) {
	sb, err := json.Marshal(p.Session)
	if err != nil {
		return "", fmt.Errorf("marshal session %s: %w", p.Key(), err)
	}
	tb, err := json.Marshal(p.Stats)
	if err != nil {
		return "", fmt.Errorf("marshal stats %s: %w", p.Key(), err)
	}
	h := sha256.New()
	h.Write(sb)
	h.Write([]byte{0x1f})
	h.Write(tb)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Payload is the request body for POST /v1/metrics. The shape is documented
// in docs/spec.md ## サーバ送信 ### プロトコル.
type Payload struct {
	ClientVersion   string       `json:"client_version"`
	SchemaHash      string       `json:"schema_hash"`
	Sessions        []SessionRow `json:"sessions"`
	TranscriptStats []StatsRow   `json:"transcript_stats"`
}

// Response is the server's reply.
type Response struct {
	ReceivedSessions int  `json:"received_sessions"`
	SkippedSessions  int  `json:"skipped_sessions"`
	SchemaMismatch   bool `json:"schema_mismatch"`
}

// LoadPairs reads every (session, stats) pair from the local DB for the given
// coding_agent, filtering out in-progress sessions (ended_at empty OR
// end_reason empty) per the spec. The LEFT JOIN produces zero-valued stats
// when transcript parsing has not yet populated the row — those still ship
// because the server's upsert handles them.
//
// Pass codingAgent == "" to fetch every agent in one query.
func LoadPairs(db *sql.DB, codingAgent string) ([]SessionPair, error) {
	const baseQuery = `
SELECT
    s.session_id, s.coding_agent, s.agent_version, s.user_id, s.timestamp, s.cwd, s.repo, s.branch,
    s.pr_url, s.pr_title, s.transcript, s.parent_session_id, s.ended_at, s.end_reason,
    s.is_subagent, s.backfill_checked, s.is_merged, s.task_type, s.review_comments, s.changes_requested,
    COALESCE(ts.tool_use_total, 0), COALESCE(ts.mid_session_msgs, 0), COALESCE(ts.ask_user_question, 0),
    COALESCE(ts.input_tokens, 0), COALESCE(ts.output_tokens, 0), COALESCE(ts.cache_write_tokens, 0),
    COALESCE(ts.cache_read_tokens, 0), COALESCE(ts.reasoning_tokens, 0), COALESCE(ts.model, ''),
    COALESCE(ts.is_ghost, 0)
FROM sessions s
LEFT JOIN transcript_stats ts
    ON s.session_id = ts.session_id AND s.coding_agent = ts.coding_agent
WHERE s.ended_at != '' AND s.end_reason != ''`

	query := baseQuery
	var args []any
	if codingAgent != "" {
		query += " AND s.coding_agent = ?"
		args = append(args, codingAgent)
	}
	query += " ORDER BY s.coding_agent, s.session_id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var out []SessionPair
	for rows.Next() {
		var p SessionPair
		if err := rows.Scan(
			&p.Session.SessionID, &p.Session.CodingAgent, &p.Session.AgentVersion, &p.Session.UserID,
			&p.Session.Timestamp, &p.Session.CWD, &p.Session.Repo, &p.Session.Branch,
			&p.Session.PRURL, &p.Session.PRTitle, &p.Session.Transcript, &p.Session.ParentSessionID,
			&p.Session.EndedAt, &p.Session.EndReason,
			&p.Session.IsSubagent, &p.Session.BackfillChecked, &p.Session.IsMerged, &p.Session.TaskType,
			&p.Session.ReviewComments, &p.Session.ChangesRequested,
			&p.Stats.ToolUseTotal, &p.Stats.MidSessionMsgs, &p.Stats.AskUserQuestion,
			&p.Stats.InputTokens, &p.Stats.OutputTokens, &p.Stats.CacheWriteTokens,
			&p.Stats.CacheReadTokens, &p.Stats.ReasoningTokens, &p.Stats.Model,
			&p.Stats.IsGhost,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		p.Stats.SessionID = p.Session.SessionID
		p.Stats.CodingAgent = p.Session.CodingAgent
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return out, nil
}

// SelectChanged returns the pairs whose hash differs from versions[key]. When
// full is true, every pair is returned regardless. Hashes for the chosen pairs
// are returned alongside so the caller can update state after a successful
// push without recomputing.
func SelectChanged(pairs []SessionPair, versions map[string]string, full bool) ([]SessionPair, map[string]string, error) {
	out := make([]SessionPair, 0, len(pairs))
	hashes := make(map[string]string, len(pairs))
	for _, p := range pairs {
		h, err := p.Hash()
		if err != nil {
			return nil, nil, err
		}
		key := p.Key()
		if !full {
			if existing, ok := versions[key]; ok && existing == h {
				continue
			}
		}
		out = append(out, p)
		hashes[key] = h
	}
	return out, hashes, nil
}

// SplitBatches partitions pairs so each batch's marshaled Payload stays under
// maxBytes. The split is approximate: we encode pairs incrementally and start
// a new batch when adding the next pair would exceed the limit. Per spec the
// server's hard cap is 50 MB; a single oversized pair is allowed through as
// its own batch (better to let the server reject one row than refuse to send).
func SplitBatches(pairs []SessionPair, clientVersion, schemaHash string, maxBytes int) ([]Payload, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	var batches []Payload
	current := Payload{ClientVersion: clientVersion, SchemaHash: schemaHash}
	currentSize := payloadOverhead(clientVersion, schemaHash)

	for _, p := range pairs {
		sb, err := json.Marshal(p.Session)
		if err != nil {
			return nil, fmt.Errorf("marshal session %s: %w", p.Key(), err)
		}
		tb, err := json.Marshal(p.Stats)
		if err != nil {
			return nil, fmt.Errorf("marshal stats %s: %w", p.Key(), err)
		}
		// +2 per row to account for comma + newline-ish delimiters that
		// json.Marshal of the slice adds on top of the element bytes.
		addSize := len(sb) + len(tb) + 4
		if len(current.Sessions) > 0 && currentSize+addSize > maxBytes {
			batches = append(batches, current)
			current = Payload{ClientVersion: clientVersion, SchemaHash: schemaHash}
			currentSize = payloadOverhead(clientVersion, schemaHash)
		}
		current.Sessions = append(current.Sessions, p.Session)
		current.TranscriptStats = append(current.TranscriptStats, p.Stats)
		currentSize += addSize
	}
	if len(current.Sessions) > 0 {
		batches = append(batches, current)
	}
	return batches, nil
}

// payloadOverhead estimates the bytes spent on the envelope (client_version,
// schema_hash, JSON braces, array brackets) so SplitBatches can reason about
// the marginal cost of adding rows without re-marshaling on every iteration.
func payloadOverhead(clientVersion, schemaHash string) int {
	return len(clientVersion) + len(schemaHash) + 80
}
