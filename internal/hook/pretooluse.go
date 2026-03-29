package hook

import (
	"os"
	"path/filepath"
)

// RunPreToolUse handles the PreToolUse hook event.
// Writes annotated tool name to a per-session temp file for correlation.
func RunPreToolUse(input *HookInput) error {
	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = "unknown"
	}

	toolName := input.ToolName
	if toolName == "" {
		toolName = "unknown"
	}

	cwd, _ := os.Getwd()
	annotated := AnnotateTool(toolName, input.ToolInput, cwd)

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".claude", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(
		filepath.Join(logDir, "last-tool-"+sessionID),
		[]byte(annotated+"\n"),
		0644,
	)
}
