package main

import (
	"fmt"
	"os"

	"github.com/ishii1648/hitl-metrics/internal/backfill"
	"github.com/ishii1648/hitl-metrics/internal/hook"
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
		runInstall()
	case "hook":
		runHook(os.Args[2:])
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
  sync-db                                JSONL/transcript → SQLite 変換
  install                                hooks を ~/.claude/settings.json に登録
  hook <event>                           Claude Code hook を実行
    session-start                        セッションインデックスを記録
    session-end                          セッション終了時刻を記録
    stop                                 backfill + sync-db を実行
    todo-cleanup                         完了タスクを CHANGELOG に移動`)
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

func runInstall() {
	if err := install.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "install error: %v\n", err)
		os.Exit(1)
	}
}

func runHook(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: hitl-metrics hook <event-name>")
		os.Exit(1)
	}

	var err error
	switch args[0] {
	case "session-start":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		err = hook.RunSessionStart(input)
	case "session-end":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		err = hook.RunSessionEnd(input)
	case "permission-request":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		err = hook.RunPermissionRequest(input)
	case "pre-tool-use":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		err = hook.RunPreToolUse(input)
	case "stop":
		err = hook.RunStop()
	case "todo-cleanup":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		err = hook.RunTodoCleanup(input)
	default:
		fmt.Fprintf(os.Stderr, "unknown hook event: %s\n", args[0])
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "hook error: %v\n", err)
		os.Exit(1)
	}
}
