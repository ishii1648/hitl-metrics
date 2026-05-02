# hitl-metrics 仕様

この文書は hitl-metrics の外部契約を記述する。
実装方法や設計判断は `docs/design.md`、過去の経緯は `docs/history.md` に分離する。
セットアップ手順と日常運用は `docs/setup.md` と `docs/usage.md` を参照する。

---

## 概要

hitl-metrics は Claude Code の **PR 単位のトークン消費効率** を追跡・可視化する計測ツールである。
Claude Code hook が記録したセッションイベントとトランスクリプトを SQLite に変換し、Grafana ダッシュボードで PR ごとの token 消費・効率を表示する。

データフロー:

```
Claude Code hooks
    → ~/.claude/session-index.jsonl + transcript JSONL
    → hitl-metrics backfill / sync-db
    → ~/.claude/hitl-metrics.db (SQLite)
    → Grafana
```

---

## hook の登録と役割

`~/.claude/settings.json` に登録する hook は `hitl-metrics hook <event>` のサブコマンド形式で呼び出す。`hitl-metrics` バイナリが PATH 上にある必要がある。登録は dotfiles または手動で行い、`hitl-metrics install` は自動登録しない。

| hook イベント | サブコマンド | 役割 |
|---|---|---|
| `SessionStart` | `hitl-metrics hook session-start` | セッション開始メタデータを `session-index.jsonl` に追記 |
| `SessionStart` | `hitl-metrics hook todo-cleanup` | main ブランチで `TODO.md` の完了タスクを `CHANGELOG.md` に移送 |
| `SessionEnd` | `hitl-metrics hook session-end` | 終了時刻と終了理由を `session-index.jsonl` に追記し、SQLite を同期 |
| `Stop` | `hitl-metrics hook stop` | 応答完了時に `backfill` → `sync-db` を実行（ブロッキング） |

`Stop` hook はセッション終了を待機するが、cursor 方式・時間条件スキップ・goroutine 並列・8 秒タイムアウトで処理時間を抑制する。

---

## CLI

```
hitl-metrics install                                   セットアップ案内を表示（hook 登録は dotfiles または手動）
hitl-metrics install --uninstall-hooks                 旧 install で書き込んだ hook を ~/.claude/settings.json から削除
hitl-metrics doctor                                    binary / data dir / hook 登録の検証（自動修復はしない）
hitl-metrics backfill [--recheck]                      pr_urls / is_merged / review_comments を補完
hitl-metrics sync-db                                   JSONL/transcript → SQLite 変換（毎回フル再構築）
hitl-metrics update <session_id> <url>...              session-index.jsonl に PR URL を追加（重複排除）
hitl-metrics update --mark-checked <session_id>...     backfill_checked フラグをセット
hitl-metrics update --by-branch <repo> <branch> <url>  同一 repo+branch の全セッションに URL を追加
hitl-metrics hook <event>                              hook サブコマンド（settings.json から呼ばれる）
hitl-metrics version                                   version を表示
```

`backfill --recheck` は cursor を無視してフルスキャンする。

---

## データファイル

すべて `~/.claude/` 配下に配置する。

| ファイル | 形式 | 役割 |
|---|---|---|
| `session-index.jsonl` | JSON Lines | セッション単位のメタデータ。SessionStart で追記、SessionEnd / backfill で更新 |
| `hitl-metrics-state.json` | JSON | backfill の cursor（`last_backfill_offset`, `last_meta_check`） |
| `hitl-metrics.db` | SQLite | sync-db が生成する集計 DB。毎回 DROP & CREATE で再構築 |
| `logs/session-index-debug.log` | テキスト | hook のデバッグログ |

### `session-index.jsonl` のレコード

```json
{
  "timestamp": "2026-02-27 12:34:56",
  "session_id": "xxx-yyy-zzz",
  "cwd": "/path/to/project",
  "repo": "org/repo",
  "branch": "feature-xxx",
  "pr_urls": ["https://github.com/org/repo/pull/123"],
  "transcript": "/path/to/transcript.jsonl",
  "parent_session_id": "",
  "ended_at": "2026-02-27 13:00:00",
  "end_reason": "exit",
  "backfill_checked": true,
  "is_merged": true,
  "review_comments": 0,
  "changes_requested": 0
}
```

- `pr_urls` は PostToolUse / Stop / `update` / `backfill` から重複排除しつつ追記される。`sync-db` は配列の最後の 1 件を採用する。
- `backfill_checked: true` のレコードは backfill で再 API 呼び出しされない。PR が存在しないブランチで永続スキップされる。
- 後方互換: 古いレコードに新フィールドが欠けていても扱える（欠落値は 0 / false / 空文字列）。

---

## SQLite データモデル

`sync-db` は毎回 DROP & CREATE でフル再構築する。マイグレーションは存在しない。

### `sessions` テーブル

| カラム | 型 | 説明 |
|---|---|---|
| `session_id` | TEXT PK | Claude Code セッション ID |
| `timestamp` | TEXT | セッション開始時刻（ISO8601） |
| `cwd` | TEXT | 作業ディレクトリ |
| `repo` | TEXT | リポジトリ（`org/repo` 形式） |
| `branch` | TEXT | ブランチ名 |
| `pr_url` | TEXT | PR URL（`pr_urls` 配列の最後の 1 件） |
| `transcript` | TEXT | transcript ファイルパス |
| `parent_session_id` | TEXT | 親セッション ID。サブエージェント判定用 |
| `ended_at` | TEXT | セッション終了時刻 |
| `end_reason` | TEXT | SessionEnd hook の終了理由 |
| `is_subagent` | INTEGER | `parent_session_id` 非空なら 1 |
| `backfill_checked` | INTEGER | backfill 処理済みなら 1 |
| `is_merged` | INTEGER | PR がマージ済みなら 1 |
| `task_type` | TEXT | ブランチプレフィックス（feat/fix/docs/chore） |
| `review_comments` | INTEGER | PR レビューコメント数 |
| `changes_requested` | INTEGER | CHANGES_REQUESTED レビュー回数 |

### `transcript_stats` テーブル

| カラム | 型 | 説明 |
|---|---|---|
| `session_id` | TEXT PK | セッション ID |
| `tool_use_total` | INTEGER | ツール呼び出し総数 |
| `mid_session_msgs` | INTEGER | mid-session ユーザーメッセージ数（tool_result 除外） |
| `ask_user_question` | INTEGER | AskUserQuestion 呼び出し回数 |
| `input_tokens` | INTEGER | `usage.input_tokens` の合計 |
| `output_tokens` | INTEGER | `usage.output_tokens` の合計 |
| `cache_write_tokens` | INTEGER | `usage.cache_creation_input_tokens` の合計 |
| `cache_read_tokens` | INTEGER | `usage.cache_read_input_tokens` の合計 |
| `model` | TEXT | セッション内で最後に観測した model |
| `is_ghost` | INTEGER | `type:"user"` エントリが 0 件なら 1 |

`usage` 欠落の transcript はトークンを 0 として扱う。

### `pr_metrics` VIEW

PR 単位の集約ビュー。次のフィルタ条件を適用する。

| 条件 | 理由 |
|---|---|
| `pr_url != ''` | PR 未作成セッションを除外 |
| `is_subagent = 0` | サブエージェントセッションを除外 |
| `is_merged = 1` | 未マージ・放棄 PR を除外（最終成果物のみ） |
| `is_ghost = 0` | ゴーストセッションを除外 |
| `repo NOT IN ('ishii1648/dotfiles')` | dotfiles リポジトリを除外 |

集約カラム: `pr_url`, `task_type`, `model`, `session_count`, `tool_use_total`, `mid_session_msgs`, `ask_user_question`, `input_tokens`, `output_tokens`, `cache_write_tokens`, `cache_read_tokens`, `review_comments`, `changes_requested`, `total_tokens`, `tokens_per_session`, `tokens_per_tool_use`, `pr_per_million_tokens`

`total_tokens` は input / output / cache write / cache read token の合計。`pr_per_million_tokens` は 100 万 token あたりに完了した PR 数。

### `session_concurrency_daily` / `session_concurrency_weekly` VIEW

トップレベル Claude Code セッションの同時実行数を時間軸で集約する。`sessions.timestamp` と `sessions.ended_at` の区間重なりから算出し、subagent / ghost / dotfiles を除外する。

集約カラム: `day` または `week_start`, `avg_concurrent_sessions`, `peak_concurrent_sessions`

---

## ダッシュボード

データソース: SQLite（`hitl-metrics.db`）+ [frser-sqlite-datasource](https://github.com/fr-ser/grafana-sqlite-datasource)
ダッシュボード JSON: `grafana/dashboards/hitl-metrics.json`

### パネル構成

| パネル | 種別 | 内容 |
|---|---|---|
| ヘッドライン | Stat | merged PR 数、total tokens、avg tokens / PR、PR / 1M tokens、changes requested |
| 週別 token 消費 | Time series | total_tokens と merged PR 数の推移 |
| 週別 PR / 1M tokens | Time series | token 効率の推移 |
| 週別 concurrent sessions | Time series | トップレベルセッションの平均・最大同時実行数 |
| タスク種別 token | Bar chart | task_type 別 avg tokens / PR |
| PR 別スコアカード | Table | pr_url, task_type, model, total_tokens, tokens_per_session, tokens_per_tool_use, pr_per_million_tokens, token 内訳, session_count, tool_use_total, mid_session_msgs, ask_user_question, review_comments, changes_requested |
| PR 別 session_count 分布 | Bar chart | 多セッション PR の外れ値検出 |
| PR 別 tokens / tool_use | Bar chart | 1 tool_use あたりの token 消費が大きい PR |

---

## 環境変数

| 変数 | 説明 |
|---|---|
| `GRAFANA_PORT` | E2E スクリーンショット時の Grafana ポート。未指定なら `13000` |

---

## 非目標

- 個別の API 課金額の算出（モデルごとの単価変動が大きいため、token 量のみを記録する）
- permission UI 表示回数や `perm_rate` の計測（Claude Code の auto mode 進化で改善対象としての価値が低いと判断したため廃止）
- 未マージ PR や PR なしセッションの効率指標（`pr_metrics` のスコープ外）
- マイグレーション機能（`sync-db` が毎回フル再構築するため）
