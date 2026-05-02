package hook

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/ishii1648/hitl-metrics/internal/agent"
	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
)

// RunStop handles the Stop hook event for the given agent.
//
// Behavior:
//   - Codex: overwrite ended_at on every fire (Codex has no SessionEnd, so
//     the last Stop fire is the de-facto SessionEnd). end_reason is fixed
//     to "stop".
//   - Both agents: shell out to `hitl-metrics backfill` then
//     `hitl-metrics sync-db` so the SQLite DB is fresh by the time the
//     user looks at the dashboard. We exec the binary (instead of calling
//     the packages directly) to keep the sqlite dependency out of the
//     hook hot path and to match the original shell behaviour.
//
// `--agent <name>` is forwarded so the subprocesses scope their cursor /
// session-index correctly.
func RunStop(input *HookInput, a *agent.Agent) error {
	if a == nil {
		a = agent.Claude()
	}

	if a.Name == agent.NameCodex && input != nil && input.SessionID != "" {
		endedAt := time.Now().Format("2006-01-02 15:04:05")
		if _, err := sessionindex.UpdateEnd(a.SessionIndexPath(), input.SessionID, endedAt, "stop"); err != nil {
			return fmt.Errorf("update ended_at: %w", err)
		}
	}

	if out, err := exec.Command("hitl-metrics", "backfill", "--agent", a.Name).CombinedOutput(); err != nil {
		return fmt.Errorf("backfill: %w\n%s", err, out)
	}
	if out, err := exec.Command("hitl-metrics", "sync-db").CombinedOutput(); err != nil {
		return fmt.Errorf("sync-db: %w\n%s", err, out)
	}
	return nil
}
