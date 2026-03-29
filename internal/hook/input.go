package hook

import (
	"encoding/json"
	"io"
	"os"
)

// HookInput represents the JSON input from Claude Code hooks.
type HookInput struct {
	SessionID       string          `json:"session_id"`
	ToolName        string          `json:"tool_name"`
	ToolInput       json.RawMessage `json:"tool_input"`
	CWD             string          `json:"cwd"`
	TranscriptPath  string          `json:"transcript_path"`
	ParentSessionID string          `json:"parent_session_id"`
}

// ReadInput reads and parses JSON hook input from stdin.
func ReadInput() (*HookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	return &input, nil
}
