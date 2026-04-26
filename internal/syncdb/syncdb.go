package syncdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
	"github.com/ishii1648/hitl-metrics/internal/transcript"
)

const dummyPRURL = "https://github.com/org/repo/pull/123"

// DBPath returns the default path to hitl-metrics.db.
func DBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "hitl-metrics.db")
}

// Run performs a full rebuild of the SQLite database from JSONL/transcript sources.
func Run() error {
	return RunWithPaths(sessionindex.IndexFile(), DBPath())
}

// RunWithPaths performs a full rebuild using specified paths (for testing).
func RunWithPaths(indexPath, dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	// Enable WAL mode for better performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("pragma: %w", err)
	}

	// Create schema (DROP + CREATE)
	if _, err := db.Exec(createTablesSQL); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// Load sessions
	_, sessions, err := sessionindex.ReadAll(indexPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read sessions: %w", err)
	}

	// Deduplicate by session_id (last wins, matching Python behavior)
	sessionMap := make(map[string]sessionindex.Session)
	for _, s := range sessions {
		if s.SessionID == "" {
			continue
		}
		sessionMap[s.SessionID] = s
	}

	// Insert sessions
	fmt.Printf("sync-db: %d セッションを処理中...\n", len(sessionMap))

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	sessionStmt, err := tx.Prepare(`INSERT OR REPLACE INTO sessions
		(session_id, timestamp, cwd, repo, branch, pr_url, transcript, parent_session_id, is_subagent, backfill_checked, is_merged, task_type, review_comments, changes_requested)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer sessionStmt.Close()

	statsStmt, err := tx.Prepare(`INSERT OR REPLACE INTO transcript_stats
		(session_id, tool_use_total, mid_session_msgs, ask_user_question, input_tokens, output_tokens, cache_write_tokens, cache_read_tokens, model, is_ghost)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer statsStmt.Close()

	transcriptCount := 0
	for _, s := range sessionMap {
		prURL := ""
		if len(s.PRURLs) > 0 {
			prURL = s.PRURLs[len(s.PRURLs)-1]
		}
		// Skip dummy PR URL
		if prURL == dummyPRURL {
			prURL = ""
		}

		isSubagent := 0
		if s.ParentSessionID != "" {
			isSubagent = 1
		}
		backfillChecked := 0
		if s.BackfillChecked {
			backfillChecked = 1
		}
		isMerged := 0
		if s.IsMerged {
			isMerged = 1
		}
		taskType := ExtractTaskType(s.Branch)

		if _, err := sessionStmt.Exec(
			s.SessionID, s.Timestamp, s.CWD, s.Repo, s.Branch,
			prURL, s.Transcript, s.ParentSessionID,
			isSubagent, backfillChecked, isMerged, taskType, s.ReviewComments, s.ChangesRequested,
		); err != nil {
			return fmt.Errorf("insert session %s: %w", s.SessionID, err)
		}

		// Parse transcript and insert stats
		ts := transcript.Parse(s.Transcript)
		isGhost := 0
		if ts.IsGhost {
			isGhost = 1
		}
		if _, err := statsStmt.Exec(
			s.SessionID, ts.ToolUseTotal, ts.MidSessionMsgs, ts.AskUserQuestion,
			ts.InputTokens, ts.OutputTokens, ts.CacheWriteTokens, ts.CacheReadTokens,
			ts.Model, isGhost,
		); err != nil {
			return fmt.Errorf("insert stats %s: %w", s.SessionID, err)
		}
		transcriptCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("sync-db: 完了 — sessions: %d, transcript_stats: %d\n",
		len(sessionMap), transcriptCount)

	// Verify with a quick query
	var prMetricsCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM pr_metrics").Scan(&prMetricsCount); err != nil {
		// VIEW might fail if data is empty, that's ok
		prMetricsCount = 0
	}
	fmt.Printf("sync-db: pr_metrics VIEW: %d PR\n", prMetricsCount)

	return nil
}

// ExtractTaskType extracts the task type from a branch name prefix.
// e.g. "feat/add-metrics" → "feat", "fix/bug-42" → "fix", "main" → ""
func ExtractTaskType(branch string) string {
	parts := strings.SplitN(branch, "/", 2)
	if len(parts) == 2 {
		switch parts[0] {
		case "feat", "fix", "docs", "chore":
			return parts[0]
		}
	}
	return ""
}

// ShortenPRURL extracts "owner/repo#number" from a full GitHub PR URL.
func ShortenPRURL(url string) string {
	// https://github.com/owner/repo/pull/123 → owner/repo#123
	parts := strings.Split(url, "/")
	if len(parts) >= 7 && parts[2] == "github.com" && parts[5] == "pull" {
		return parts[3] + "/" + parts[4] + "#" + parts[6]
	}
	return url
}
