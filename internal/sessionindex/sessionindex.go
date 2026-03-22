package sessionindex

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Session represents a single entry in session-index.jsonl.
type Session struct {
	Timestamp       string   `json:"timestamp"`
	SessionID       string   `json:"session_id"`
	CWD             string   `json:"cwd"`
	Repo            string   `json:"repo"`
	Branch          string   `json:"branch"`
	PRURLs          []string `json:"pr_urls"`
	Transcript      string   `json:"transcript"`
	ParentSessionID string   `json:"parent_session_id"`
	BackfillChecked bool     `json:"backfill_checked"`
	IsMerged        bool     `json:"is_merged"`
	ReviewComments  int      `json:"review_comments"`
}

// IndexFile returns the default path to session-index.jsonl.
func IndexFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "session-index.jsonl")
}

// ReadAll reads all sessions from the JSONL file.
// Invalid lines are preserved as raw JSON for round-trip safety.
func ReadAll(path string) ([]json.RawMessage, []Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var raws []json.RawMessage
	var sessions []Session
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var s Session
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			// preserve invalid lines as-is
			raws = append(raws, json.RawMessage(line))
			sessions = append(sessions, Session{})
			continue
		}
		raws = append(raws, json.RawMessage(line))
		sessions = append(sessions, s)
	}
	return raws, sessions, scanner.Err()
}

// WriteAll writes sessions back to the JSONL file.
// Each raw message is written as a single line.
func WriteAll(path string, raws []json.RawMessage) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i, raw := range raws {
		if i > 0 {
			w.WriteByte('\n')
		}
		w.Write(raw)
	}
	w.WriteByte('\n')
	return w.Flush()
}

// NormalizeRepo removes trailing ".git" suffix.
func NormalizeRepo(repo string) string {
	return strings.TrimSuffix(repo, ".git")
}

// MarshalSession marshals a Session to JSON without escaping non-ASCII.
func MarshalSession(s Session) (json.RawMessage, error) {
	buf, err := json.Marshal(s)
	return json.RawMessage(buf), err
}

// UnmarshalSession decodes raw JSON into a Session, preserving extra fields.
func UnmarshalSession(raw json.RawMessage) (Session, error) {
	var s Session
	err := json.Unmarshal(raw, &s)
	return s, err
}
