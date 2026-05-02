package transcript

import (
	"bufio"
	"encoding/json"
	"strings"
	"time"
)

// ParseCodex reads a Codex CLI rollout JSONL (or .jsonl.zst) and returns
// uniform Stats. The Codex format differs from Claude's transcript:
//
//   - Top-level entries carry a `type` ("session_meta", "turn_context",
//     "event_msg", "response_item", ...) and a `timestamp`.
//   - User input lives in `event_msg.payload.type == "user_message"`.
//   - Cumulative token totals live in `event_msg.payload.type ==
//     "token_count"` (we keep the *last* total, not the sum).
//   - Tool calls live in `response_item.payload.type` and span several
//     concrete shapes (function_call, custom_tool_call, mcp_tool_call_*),
//     so we count any payload type containing "tool_call" or
//     "function_call".
//   - Model lives in `turn_context.payload.model` (most recent wins).
//
// Codex has no AskUserQuestion analogue; AskUserQuestion stays at 0.
func ParseCodex(transcriptPath string) Stats {
	if transcriptPath == "" {
		return Stats{IsGhost: true}
	}

	rc, err := openTranscript(transcriptPath)
	if err != nil {
		return Stats{IsGhost: true}
	}
	defer rc.Close()

	var (
		toolUseTotal   int
		midSessionMsgs int
		userMsgCount   int
		model          string
		lastTokens     codexTokenInfo
		seenTokens     bool
		lastTimestamp  time.Time
	)

	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry codexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if ts := parseCodexTimestamp(entry.Timestamp); !ts.IsZero() {
			lastTimestamp = ts
		}

		switch entry.Type {
		case "turn_context":
			var p struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(entry.Payload, &p); err == nil && p.Model != "" {
				model = p.Model
			}
		case "event_msg":
			payloadType := readPayloadType(entry.Payload)
			switch payloadType {
			case "user_message":
				userMsgCount++
				if userMsgCount > 1 {
					midSessionMsgs++
				}
			case "token_count":
				var p struct {
					Info *struct {
						Total codexTokenInfo `json:"total_token_usage"`
					} `json:"info"`
				}
				if err := json.Unmarshal(entry.Payload, &p); err == nil && p.Info != nil {
					lastTokens = p.Info.Total
					seenTokens = true
				}
			}
		case "response_item":
			payloadType := readPayloadType(entry.Payload)
			if isCodexToolCall(payloadType) {
				toolUseTotal++
			}
		}
	}

	stats := Stats{
		ToolUseTotal:    toolUseTotal,
		MidSessionMsgs:  midSessionMsgs,
		AskUserQuestion: 0,
		Model:           model,
		IsGhost:         userMsgCount == 0,
		LastTimestamp:   lastTimestamp,
	}
	if seenTokens {
		stats.InputTokens = lastTokens.InputTokens
		stats.OutputTokens = lastTokens.OutputTokens
		stats.CacheReadTokens = lastTokens.CachedInputTokens
		// Codex reports a single "cached_input_tokens"; there is no separate
		// cache write counter on the OpenAI API, so cache_write stays 0.
		stats.CacheWriteTokens = 0
		stats.ReasoningTokens = lastTokens.ReasoningOutputTokens
	}
	return stats
}

// ReadCodexMeta extracts the session_meta payload (without scanning the
// whole file). Used by backfill / hooks to pull cli_version when the
// hook input did not carry one.
func ReadCodexMeta(transcriptPath string) (cliVersion, model string, lastTimestamp time.Time, ok bool) {
	if transcriptPath == "" {
		return "", "", time.Time{}, false
	}
	rc, err := openTranscript(transcriptPath)
	if err != nil {
		return "", "", time.Time{}, false
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry codexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if ts := parseCodexTimestamp(entry.Timestamp); !ts.IsZero() {
			lastTimestamp = ts
		}
		if entry.Type == "session_meta" && cliVersion == "" {
			var p struct {
				CliVersion string `json:"cli_version"`
			}
			if err := json.Unmarshal(entry.Payload, &p); err == nil {
				cliVersion = p.CliVersion
			}
		}
		if entry.Type == "turn_context" {
			var p struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(entry.Payload, &p); err == nil && p.Model != "" {
				model = p.Model
			}
		}
	}
	return cliVersion, model, lastTimestamp, cliVersion != "" || !lastTimestamp.IsZero()
}

func readPayloadType(payload json.RawMessage) string {
	var p struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(payload, &p)
	return p.Type
}

func isCodexToolCall(payloadType string) bool {
	switch payloadType {
	case "function_call", "custom_tool_call", "mcp_tool_call":
		return true
	}
	// Future-proof: anything ending in "_tool_call" counts. Excludes
	// "_tool_call_end" / "_output" which represent results, not calls.
	if strings.HasSuffix(payloadType, "tool_call") {
		return true
	}
	return false
}

func parseCodexTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Codex emits RFC3339Nano with Z suffix.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

type codexEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexTokenInfo struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}
