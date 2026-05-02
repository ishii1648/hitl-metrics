package main

import (
	"fmt"
	"os"

	"github.com/ishii1648/hitl-metrics/internal/agent"
	"github.com/ishii1648/hitl-metrics/internal/backfill"
	"github.com/ishii1648/hitl-metrics/internal/doctor"
	"github.com/ishii1648/hitl-metrics/internal/hook"
	"github.com/ishii1648/hitl-metrics/internal/install"
	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
	"github.com/ishii1648/hitl-metrics/internal/setup"
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
		runSyncDB(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "uninstall-hooks":
		runUninstallHooks()
	case "install":
		runInstallAlias(os.Args[2:])
	case "doctor":
		runDoctor()
	case "hook":
		runHook(os.Args[2:])
	case "version":
		fmt.Println("hitl-metrics version unknown")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: hitl-metrics <command> [args...]

Commands:
  setup [--agent <claude|codex>]         セットアップ案内を表示（hook 登録は dotfiles または手動）
  uninstall-hooks                        旧 install で登録された hook を ~/.claude/settings.json から削除
  doctor                                 検出された agent ごとに binary / data dir / hook 登録を検証（自動修復はしない）
  backfill [--recheck] [--agent <a>]     検出された agent すべての pr_urls / is_merged / review_comments を補完
  sync-db [--agent <a>]                  検出された agent すべての JSONL/transcript → SQLite 変換（毎回フル再構築）
  update <session_id> <url>...           session-index.jsonl に PR URL を追加（重複排除）
  update --mark-checked <session_id>...  backfill_checked フラグをセット
  update --by-branch <repo> <branch> <url>  同一 repo+branch の全セッションに URL を追加
  hook <event> [--agent <a>]             hook サブコマンド（settings.json / config.toml / hooks.json から呼ばれる）
    session-start                        セッションメタデータを記録
    session-end                          セッション終了時刻を記録（Claude のみ）
    stop                                 backfill + sync-db を実行
    post-tool-use                        tool_response から PR URL を抽出（Codex 用）
    pre-tool-use                         per-session tool 注釈を記録（Claude のみ）
    permission-request                   permission ログを追記（Claude のみ）
    todo-cleanup                         main ブランチで TODO.md の完了タスクを削除
  install                                廃止予定 alias。setup を呼び出して同等の案内を表示
  version                                version を表示

Agent precedence: --agent → $HITL_METRICS_AGENT → autodetect (~/.claude / ~/.codex)`)
}

// extractAgentFlag pulls "--agent <name>" out of args, returning the name
// and the remaining args. Order is preserved for non-flag arguments.
func extractAgentFlag(args []string) (string, []string) {
	out := args[:0:0]
	name := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--agent" && i+1 < len(args):
			name = args[i+1]
			i++
		case len(args[i]) > len("--agent=") && args[i][:len("--agent=")] == "--agent=":
			name = args[i][len("--agent="):]
		default:
			out = append(out, args[i])
		}
	}
	return name, out
}

func runUpdate(args []string) {
	indexPath := sessionindex.IndexFile()

	if len(args) == 0 {
		return
	}

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
	agentName, args := extractAgentFlag(args)
	recheck := false
	for _, a := range args {
		if a == "--recheck" {
			recheck = true
		}
	}
	agents, err := agent.ResolveOrDetect(agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill: %v\n", err)
		os.Exit(1)
	}
	if err := backfill.RunForAgents(agents, recheck); err != nil {
		fmt.Fprintf(os.Stderr, "backfill error: %v\n", err)
		os.Exit(1)
	}
}

func runSyncDB(args []string) {
	agentName, _ := extractAgentFlag(args)
	agents, err := agent.ResolveOrDetect(agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-db: %v\n", err)
		os.Exit(1)
	}
	if err := syncdb.RunForAgents(agents, syncdb.DBPath()); err != nil {
		fmt.Fprintf(os.Stderr, "sync-db error: %v\n", err)
		os.Exit(1)
	}
}

func runSetup(args []string) {
	agentName, _ := extractAgentFlag(args)
	var a *agent.Agent
	if agentName != "" {
		var err error
		a, err = agent.ByName(agentName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setup: %v\n", err)
			os.Exit(1)
		}
	}
	if err := setup.Run(a); err != nil {
		fmt.Fprintf(os.Stderr, "setup error: %v\n", err)
		os.Exit(1)
	}
}

func runUninstallHooks() {
	if err := setup.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "uninstall-hooks error: %v\n", err)
		os.Exit(1)
	}
}

// runInstallAlias preserves the legacy `hitl-metrics install [--uninstall-hooks]`
// surface for users still on the old invocation. New flows MUST use the
// dedicated subcommands.
func runInstallAlias(args []string) {
	for _, a := range args {
		if a == "--uninstall-hooks" {
			if err := install.Uninstall(); err != nil {
				fmt.Fprintf(os.Stderr, "install --uninstall-hooks error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
	if err := install.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "install error: %v\n", err)
		os.Exit(1)
	}
}

func runDoctor() {
	r, err := doctor.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor error: %v\n", err)
		os.Exit(1)
	}
	if r.HasFailure() {
		os.Exit(1)
	}
}

func runHook(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: hitl-metrics hook <event-name> [--agent <claude|codex>]")
		os.Exit(1)
	}

	eventName := args[0]
	agentName, _ := extractAgentFlag(args[1:])
	a, err := agent.Resolve(agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hook: %v\n", err)
		os.Exit(1)
	}

	var hookErr error
	switch eventName {
	case "session-start":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		hookErr = hook.RunSessionStart(input, a)
	case "session-end":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		hookErr = hook.RunSessionEnd(input, a)
	case "permission-request":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		hookErr = hook.RunPermissionRequest(input)
	case "pre-tool-use":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		hookErr = hook.RunPreToolUse(input)
	case "post-tool-use":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		hookErr = hook.RunPostToolUse(input, a)
	case "stop":
		// Stop hook reads input only when needed (Codex updates ended_at)
		// but Claude historically called stop with no stdin. Be tolerant
		// of either: try to read but don't fail on EOF / empty stdin.
		input, _ := hook.ReadInput()
		hookErr = hook.RunStop(input, a)
	case "todo-cleanup":
		input, e := hook.ReadInput()
		if e != nil {
			fmt.Fprintf(os.Stderr, "hook input error: %v\n", e)
			os.Exit(1)
		}
		hookErr = hook.RunTodoCleanup(input)
	default:
		fmt.Fprintf(os.Stderr, "unknown hook event: %s\n", eventName)
		os.Exit(1)
	}

	if hookErr != nil {
		fmt.Fprintf(os.Stderr, "hook error: %v\n", hookErr)
		os.Exit(1)
	}
}
