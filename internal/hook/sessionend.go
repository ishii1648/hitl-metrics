package hook

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/ishii1648/hitl-metrics/internal/agent"
	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
)

// RunSessionEnd handles the SessionEnd hook (Claude only — Codex has no
// SessionEnd; the Stop hook covers that case).
//
// Records session end metadata for parallel-session metrics, then refreshes
// SQLite. Failure to find the session is silent (e.g. transcript-only ghost
// sessions never had a SessionStart entry).
func RunSessionEnd(input *HookInput, a *agent.Agent) error {
	if a == nil {
		a = agent.Claude()
	}
	endedAt := time.Now().Format("2006-01-02 15:04:05")
	updated, err := sessionindex.UpdateEnd(a.SessionIndexPath(), input.SessionID, endedAt, input.Reason)
	if err != nil {
		return err
	}
	if !updated {
		return nil
	}
	if out, err := exec.Command("hitl-metrics", "sync-db").CombinedOutput(); err != nil {
		return fmt.Errorf("sync-db: %w\n%s", err, out)
	}
	return nil
}
