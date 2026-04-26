package hook

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
)

// RunSessionEnd handles the SessionEnd hook event.
// Records session end metadata for parallel-session metrics, then refreshes SQLite.
func RunSessionEnd(input *HookInput) error {
	endedAt := time.Now().Format("2006-01-02 15:04:05")
	updated, err := sessionindex.UpdateEnd(sessionindex.IndexFile(), input.SessionID, endedAt, input.Reason)
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
