package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ishii1648/agent-telemetry/internal/agent"
	"github.com/ishii1648/agent-telemetry/internal/backfill"
	"github.com/ishii1648/agent-telemetry/internal/doctor"
	"github.com/ishii1648/agent-telemetry/internal/hook"
	"github.com/ishii1648/agent-telemetry/internal/legacy"
	"github.com/ishii1648/agent-telemetry/internal/sessionindex"
	"github.com/ishii1648/agent-telemetry/internal/setup"
	"github.com/ishii1648/agent-telemetry/internal/syncdb"
	"github.com/ishii1648/agent-telemetry/internal/upgrade"
)

// version is overwritten at build time via -ldflags "-X main.version=<tag>".
// goreleaser sets this from the git tag; `go build` without ldflags leaves "dev".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
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
	case "upgrade":
		runUpgrade(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("agent-telemetry %s\n", version)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(1)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: agent-telemetry <command> [args...]

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
  upgrade [--check]                      GitHub Releases から最新版を取得して自身を置き換える（--check は確認のみ）
  version                                version を表示
  help                                   このヘルプを表示

Agent precedence: --agent → $AGENT_TELEMETRY_AGENT → autodetect (~/.claude / ~/.codex)`)
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

// runUpdate dispatches `update` to every detected agent's session-index.
// session_id is a UUID and the user shouldn't have to remember which agent
// owns it — we apply the operation to each present index and let the
// no-match branches return cleanly.
func runUpdate(args []string) {
	agentName, args := extractAgentFlag(args)
	agents, err := agent.ResolveOrDetect(agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		return
	}

	if args[0] == "--mark-checked" {
		sessionIDs := args[1:]
		if len(sessionIDs) == 0 {
			return
		}
		for _, a := range agents {
			if _, err := sessionindex.MarkChecked(a.SessionIndexPath(), sessionIDs); err != nil {
				fmt.Fprintf(os.Stderr, "mark-checked[%s] error: %v\n", a.Name, err)
				os.Exit(1)
			}
		}
		return
	}

	if args[0] == "--by-branch" {
		if len(args) < 4 {
			return
		}
		repo, branch, url := args[1], args[2], args[3]
		for _, a := range agents {
			if _, err := sessionindex.UpdateByBranch(a.SessionIndexPath(), repo, branch, url); err != nil {
				fmt.Fprintf(os.Stderr, "by-branch[%s] error: %v\n", a.Name, err)
				os.Exit(1)
			}
		}
		return
	}

	if len(args) < 2 {
		return
	}
	sessionID := args[0]
	urls := args[1:]
	for _, a := range agents {
		if _, err := sessionindex.Update(a.SessionIndexPath(), sessionID, urls); err != nil {
			fmt.Fprintf(os.Stderr, "update[%s] error: %v\n", a.Name, err)
			os.Exit(1)
		}
	}
}

func runBackfill(args []string) {
	migrateLegacy()
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
	migrateLegacy()
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

// migrateLegacy renames any hitl-metrics-era files left over in
// ~/.claude / ~/.codex to their agent-telemetry counterparts. Runs
// before commands that read or write these paths so users on the old
// layout don't have to perform a manual migration.
func migrateLegacy() {
	moved, errs := legacy.Migrate()
	legacy.Report(os.Stderr, moved, errs)
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

// runInstallAlias preserves the legacy `agent-telemetry install [--uninstall-hooks]`
// surface for users still on the old invocation. New flows MUST use the
// dedicated `setup` / `uninstall-hooks` subcommands.
//
// We deliberately call setup.* directly (rather than the install package's
// thin alias) so this file does not trip the staticcheck SA1019 rule.
// The deprecation warning is emitted inline and stays close to the
// dispatch logic.
func runInstallAlias(args []string) {
	for _, a := range args {
		if a == "--uninstall-hooks" {
			fmt.Fprintln(os.Stderr, "warning: `agent-telemetry install --uninstall-hooks` は廃止予定です。`agent-telemetry uninstall-hooks` を使ってください。")
			if err := setup.Uninstall(); err != nil {
				fmt.Fprintf(os.Stderr, "install --uninstall-hooks error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
	fmt.Fprintln(os.Stderr, "warning: `agent-telemetry install` は廃止予定です。`agent-telemetry setup` を使ってください。")
	if err := setup.Run(nil); err != nil {
		fmt.Fprintf(os.Stderr, "install error: %v\n", err)
		os.Exit(1)
	}
}

func runUpgrade(args []string) {
	checkOnly := false
	for _, a := range args {
		switch a {
		case "--check":
			checkOnly = true
		default:
			fmt.Fprintf(os.Stderr, "upgrade: unknown flag %q\n", a)
			os.Exit(1)
		}
	}
	if err := upgrade.Run(upgrade.Options{
		CurrentVersion: version,
		CheckOnly:      checkOnly,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "upgrade error: %v\n", err)
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
		fmt.Fprintln(os.Stderr, "usage: agent-telemetry hook <event-name> [--agent <claude|codex>]")
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
