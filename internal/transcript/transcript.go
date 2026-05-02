// Package transcript parses Claude Code / Codex CLI transcripts and
// produces a uniform Stats record for sync-db.
package transcript

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Stats holds computed metrics from one transcript file. The same record
// shape is used for both Claude Code and Codex CLI; per-agent fields that
// have no equivalent are 0 (e.g. AskUserQuestion is always 0 for Codex,
// ReasoningTokens is always 0 for Claude).
type Stats struct {
	ToolUseTotal     int
	MidSessionMsgs   int
	AskUserQuestion  int
	InputTokens      int64
	OutputTokens     int64
	CacheWriteTokens int64
	CacheReadTokens  int64
	ReasoningTokens  int64
	Model            string
	IsGhost          bool // true if no user message exists
	LastTimestamp    time.Time
}

// Parse dispatches to the agent-specific parser. agentName must be
// "claude" or "codex"; an unknown name is treated as Claude for
// backward compatibility with pre-Codex session-index entries.
func Parse(transcriptPath, agentName string) Stats {
	switch agentName {
	case "codex":
		return ParseCodex(transcriptPath)
	default:
		return ParseClaude(transcriptPath)
	}
}

// openTranscript opens a transcript file, transparently decompressing
// .jsonl.zst rollouts. The caller must close the returned ReadCloser.
//
// Codex archives older sessions as zstd; Claude never compresses, so this
// helper only kicks in when needed.
func openTranscript(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, ".zst") {
		return f, nil
	}
	dec, err := zstd.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &zstdCloser{dec: dec, file: f}, nil
}

// zstdCloser couples the zstd.Decoder lifecycle to the underlying file so
// callers only need a single Close().
type zstdCloser struct {
	dec  *zstd.Decoder
	file *os.File
}

func (z *zstdCloser) Read(p []byte) (int, error) { return z.dec.Read(p) }
func (z *zstdCloser) Close() error {
	z.dec.Close()
	return z.file.Close()
}

// ParseTimestamp parses a transcript timestamp string in the Claude format.
// Codex uses the same RFC3339-with-Z form so this works for both.
func ParseTimestamp(s string) (time.Time, error) {
	s = strings.Replace(s, "Z", "+00:00", 1)
	return time.Parse("2006-01-02T15:04:05+00:00", s)
}
