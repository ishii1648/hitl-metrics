package transcript

import (
	"bufio"
	"encoding/json"
	"strings"
)

// ParseClaude reads a Claude Code transcript JSONL file and returns
// computed stats. ReasoningTokens is always 0 — Claude transcripts do
// not break out reasoning tokens separately.
func ParseClaude(transcriptPath string) Stats {
	if transcriptPath == "" {
		return Stats{IsGhost: true}
	}

	rc, err := openTranscript(transcriptPath)
	if err != nil {
		return Stats{IsGhost: true}
	}
	defer rc.Close()

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

	scanner := bufio.NewScanner(rc)
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
		ReasoningTokens:  0,
		Model:            model,
		IsGhost:          !hasUserMsg,
	}
}

// isHumanTextMessage checks if a user entry is a human-typed message
// (excludes command outputs and tool_result-only content).
func isHumanTextMessage(entry *transcriptEntry) bool {
	if entry.Type != "user" {
		return false
	}

	if entry.Message.ContentStr != "" {
		return !strings.Contains(entry.Message.ContentStr, "<local-command-")
	}

	if len(entry.Message.Content) == 0 {
		return true
	}

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

	for _, item := range entry.Message.Content {
		if item.Type == "text" && strings.Contains(item.Text, "<local-command-") {
			return false
		}
	}

	return true
}

type transcriptEntry struct {
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Message   transcriptMessage `json:"message"`
}

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

	var s string
	if err := json.Unmarshal(raw.Content, &s); err == nil {
		m.ContentStr = s
		return nil
	}

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
