package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ishii1648/hitl-metrics/internal/backfill"
	"github.com/ishii1648/hitl-metrics/internal/install"
	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
	"github.com/ishii1648/hitl-metrics/internal/syncdb"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "update":
		runUpdate(os.Args[2:])
	case "backfill":
		runBackfill(os.Args[2:])
	case "sync-db":
		runSyncDB()
	case "install":
		runInstall(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: hitl-metrics <command> [args...]

Commands:
  update <session_id> <url>...           PR URL を追加
  update --mark-checked <session_id>...  backfill_checked をセット
  update --by-branch <repo> <branch> <url>  ブランチ全セッションに URL 追加
  backfill [--recheck]                   PR URL の一括補完
  sync-db                                JSONL/log → SQLite 変換
  install [--hooks-dir <path>]           hooks を ~/.claude/settings.json に登録`)
}

func runUpdate(args []string) {
	indexPath := sessionindex.IndexFile()

	if len(args) == 0 {
		return
	}

	// --mark-checked mode
	if args[0] == "--mark-checked" {
		sessionIDs := args[1:]
		if len(sessionIDs) == 0 {
			return
		}
		if _, err := sessionindex.MarkChecked(indexPath, sessionIDs); err != nil {
			fmt.Fprintf(os.Stderr, "mark-checked error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// --by-branch mode
	if args[0] == "--by-branch" {
		if len(args) < 4 {
			return
		}
		repo, branch, url := args[1], args[2], args[3]
		if _, err := sessionindex.UpdateByBranch(indexPath, repo, branch, url); err != nil {
			fmt.Fprintf(os.Stderr, "by-branch error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// default mode: <session_id> <url>...
	if len(args) < 2 {
		return
	}
	sessionID := args[0]
	urls := args[1:]
	if _, err := sessionindex.Update(indexPath, sessionID, urls); err != nil {
		fmt.Fprintf(os.Stderr, "update error: %v\n", err)
		os.Exit(1)
	}
}

func runBackfill(args []string) {
	recheck := false
	for _, a := range args {
		if a == "--recheck" {
			recheck = true
		}
	}
	if err := backfill.Run(sessionindex.IndexFile(), recheck); err != nil {
		fmt.Fprintf(os.Stderr, "backfill error: %v\n", err)
		os.Exit(1)
	}
}

func runSyncDB() {
	if err := syncdb.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sync-db error: %v\n", err)
		os.Exit(1)
	}
}

func runInstall(args []string) {
	hooksDir := ""
	for i, a := range args {
		if a == "--hooks-dir" && i+1 < len(args) {
			hooksDir = args[i+1]
		}
	}

	// Default: ./hooks/ relative to CWD
	if hooksDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "install error: %v\n", err)
			os.Exit(1)
		}
		hooksDir = filepath.Join(cwd, "hooks")
	}

	absDir, err := filepath.Abs(hooksDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install error: %v\n", err)
		os.Exit(1)
	}

	if err := install.Run(absDir); err != nil {
		fmt.Fprintf(os.Stderr, "install error: %v\n", err)
		os.Exit(1)
	}
}
