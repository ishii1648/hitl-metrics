# アーキテクチャ

hitl-metrics は Claude Code の人の介入率を追跡・可視化する計測ツールです。3層構成で動作します。

## 全体構成

```
┌─────────────────────────────────────────────────────┐
│  Claude Code                                         │
│  ┌───────────────────────────────────────────────┐  │
│  │  hooks (settings.json で登録)                  │  │
│  │  hitl-metrics hook session-start              │  │
│  │  hitl-metrics hook permission-request         │  │
│  │  hitl-metrics hook pre-tool-use               │  │
│  │  hitl-metrics hook stop                       │  │
│  │  hitl-metrics hook todo-cleanup               │  │
│  └───────────────────────────────────────────────┘  │
└─────────────┬───────────────────────────────────────┘
              │ JSONL / log 出力
              ▼
┌─────────────────────────────────────────────────────┐
│  データファイル (~/.claude/)                          │
│  session-index.jsonl    セッション情報               │
│  logs/permission.log    permission UI ログ           │
│  logs/last-tool-*       ツール名一時ファイル          │
└─────────────┬───────────────────────────────────────┘
              │ hitl-metrics backfill + sync-db
              ▼
┌─────────────────────────────────────────────────────┐
│  SQLite (hitl-metrics.db)                            │
│  sessions / permission_events / transcript_stats     │
│  pr_metrics (VIEW)                                   │
└─────────────┬───────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────┐
│  Grafana ダッシュボード                               │
│  介入率・ツール分布・PR 単位メトリクス               │
└─────────────────────────────────────────────────────┘
```

## パッケージ構成

```
cmd/hitl-metrics/        CLI エントリポイント
  main.go                サブコマンドルーティング

internal/
  hook/                  Claude Code hook ハンドラ（Go 実装）
    input.go             HookInput 型、stdin からの JSON 読み取り
    annotate.go          ツール名アノテーション（internal/external 分類）
    sessionstart.go      SessionStart: セッションインデックス記録
    permissionrequest.go PermissionRequest: permission UI ログ記録
    pretooluse.go        PreToolUse: ツール名一時ファイル記録
    stop.go              Stop: backfill + sync-db 実行
    todocleanup.go       SessionStart: 完了タスク CHANGELOG 移動

  backfill/              PR URL 補完・マージ判定（gh CLI 連携）
  install/               settings.json への hook 登録
  sessionindex/          session-index.jsonl の読み書き・更新
  syncdb/                JSONL/log → SQLite 変換
  permlog/               permission.log パーサー
  transcript/            トランスクリプト JSONL パーサー
```

## hook の実行方式

全 hook は `hitl-metrics hook <event-name>` サブコマンドとして実装されています。Claude Code が JSON を stdin に渡し、各ハンドラが処理します。

| サブコマンド | イベント | 処理内容 |
|---|---|---|
| `hook session-start` | SessionStart | セッション情報を session-index.jsonl に記録 |
| `hook permission-request` | PermissionRequest | ツール名をアノテーションし permission.log に記録 |
| `hook pre-tool-use` | PreToolUse | ツール名を一時ファイルに記録 |
| `hook stop` | Stop | backfill + sync-db を実行 |
| `hook todo-cleanup` | SessionStart | main ブランチで完了タスクを CHANGELOG に移動 |

### ツールアノテーション

`permission-request` と `pre-tool-use` は共通の `AnnotateTool` 関数でツール名にコンテキスト情報を付記します:

- `Bash` → `Bash(cp(internal))` / `Bash(git(external))`
- `Read` / `Write` / `Edit` / `Grep` → `Read(internal)` / `Read(external)`
- その他 → そのまま

internal/external の判定は、対象ファイルパスが作業ディレクトリ配下かどうかで行います。

## データモデル（SQLite）

- **sessions** — セッション単位の基本情報（session_id, repo, branch, pr_url, is_subagent 等）
- **permission_events** — permission UI 発生イベント（timestamp, session_id, tool）
- **transcript_stats** — トランスクリプトから抽出した統計（tool_use_total, mid_session_msgs 等）
- **pr_metrics**（VIEW） — PR 単位で session_count, perm_count, perm_rate を集約
