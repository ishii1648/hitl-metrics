package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunPermissionRequest handles the PermissionRequest hook event.
// Logs tool usage to ~/.claude/logs/permission.log.
func RunPermissionRequest(input *HookInput) error {
	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = os.Getenv("CLAUDE_SESSION_ID")
		if sessionID == "" {
			sessionID = "unknown"
		}
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

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	line := fmt.Sprintf("%s session=%s tool=%s\n", timestamp, sessionID, annotated)

	return appendFile(filepath.Join(logDir, "permission.log"), line)
}
