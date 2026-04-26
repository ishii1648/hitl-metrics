package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// Stats holds computed metrics from a transcript JSONL file.
type Stats struct {
	ToolUseTotal     int
	MidSessionMsgs   int
	AskUserQuestion  int
	InputTokens      int64
	OutputTokens     int64
	CacheWriteTokens int64
	CacheReadTokens  int64
	Model            string
	IsGhost          bool // true if no user message exists
}

// Parse reads a transcript JSONL file and returns computed stats.
// This is a direct port of server.py's load_transcript_stats() + has_user_message().
func Parse(transcriptPath string) Stats {
	if transcriptPath == "" {
		return Stats{IsGhost: true}
	}

	f, err := os.Open(transcriptPath)
	if err != nil {
		return Stats{IsGhost: true}
	}
	defer f.Close()

	var (
		toolUseTotal    int
		midSessionMsgs  int
		askUserQuestion int
		inputTokens     int64
		outputTokens    int64
		cacheWrite      int64
		cacheRead       int64
		model           string
		firstUserSeen   bool
		hasUserMsg      bool
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "user":
			hasUserMsg = true
			if !firstUserSeen {
				firstUserSeen = true
			} else {
				if isHumanTextMessage(&entry) {
					midSessionMsgs++
				}
			}

		case "assistant":
			inputTokens += entry.Message.Usage.InputTokens
			outputTokens += entry.Message.Usage.OutputTokens
			cacheWrite += entry.Message.Usage.CacheCreationInputTokens
			cacheRead += entry.Message.Usage.CacheReadInputTokens
			if entry.Message.Model != "" {
				model = entry.Message.Model
			}

			content := entry.Message.Content
			for _, item := range content {
				if item.Type == "tool_use" {
					toolUseTotal++
					if item.Name == "ask-user-question" {
						askUserQuestion++
					}
				}
			}
		}
	}

	return Stats{
		ToolUseTotal:     toolUseTotal,
		MidSessionMsgs:   midSessionMsgs,
		AskUserQuestion:  askUserQuestion,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheWriteTokens: cacheWrite,
		CacheReadTokens:  cacheRead,
		Model:            model,
		IsGhost:          !hasUserMsg,
	}
}

// isHumanTextMessage checks if a user entry is a human-typed message
// (excludes command outputs and tool_result-only content).
// Direct port of server.py's is_human_text_message().
func isHumanTextMessage(entry *transcriptEntry) bool {
	if entry.Type != "user" {
		return false
	}

	// Check string content for local-command prefix
	if entry.Message.ContentStr != "" {
		return !strings.Contains(entry.Message.ContentStr, "<local-command-")
	}

	// Check array content
	if len(entry.Message.Content) == 0 {
		return true
	}

	// If all items are tool_result, it's not a human message
	allToolResult := true
	for _, item := range entry.Message.Content {
		if item.Type != "tool_result" {
			allToolResult = false
			break
		}
	}
	if allToolResult {
		return false
	}

	// Check for local-command in any text content
	for _, item := range entry.Message.Content {
		if item.Type == "text" && strings.Contains(item.Text, "<local-command-") {
			return false
		}
	}

	return true
}

// transcriptEntry represents a line in the transcript JSONL.
type transcriptEntry struct {
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Message   transcriptMessage `json:"message"`
}

// transcriptMessage handles the message field which can have string or array content.
type transcriptMessage struct {
	Content    []contentItem `json:"-"`
	ContentStr string        `json:"-"`
	Usage      usageStats    `json:"-"`
	Model      string        `json:"-"`
}

func (m *transcriptMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		Content json.RawMessage `json:"content"`
		Usage   usageStats      `json:"usage"`
		Model   string          `json:"model"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Usage = raw.Usage
	m.Model = raw.Model
	if len(raw.Content) == 0 {
		return nil
	}

	// Try as string first
	var s string
	if err := json.Unmarshal(raw.Content, &s); err == nil {
		m.ContentStr = s
		return nil
	}

	// Try as array
	var items []contentItem
	if err := json.Unmarshal(raw.Content, &items); err == nil {
		m.Content = items
		return nil
	}

	return nil
}

type contentItem struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

type usageStats struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// ParseTimestamp parses a transcript timestamp string.
func ParseTimestamp(s string) (time.Time, error) {
	s = strings.Replace(s, "Z", "+00:00", 1)
	return time.Parse("2006-01-02T15:04:05+00:00", s)
}
