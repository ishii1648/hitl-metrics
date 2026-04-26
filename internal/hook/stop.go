package hook

import (
	"fmt"
	"os/exec"
)

// RunStop handles the Stop hook event.
// Runs backfill (PR URL completion + merge status) and sync-db (JSONL/transcript → SQLite).
// Uses os/exec to call hitl-metrics subcommands, matching the original shell behavior
// and avoiding sqlite dependency in this package.
func RunStop() error {
	if out, err := exec.Command("hitl-metrics", "backfill").CombinedOutput(); err != nil {
		return fmt.Errorf("backfill: %w\n%s", err, out)
	}
	if out, err := exec.Command("hitl-metrics", "sync-db").CombinedOutput(); err != nil {
		return fmt.Errorf("sync-db: %w\n%s", err, out)
	}
	return nil
}
