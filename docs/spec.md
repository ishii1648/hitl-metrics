# agent-telemetry 仕様

この文書は agent-telemetry の外部契約を記述する。
実装方法や設計判断は `docs/design.md`、過去の意思決定の経緯は `issues/closed/` の retro issue に分離する。
セットアップ手順と日常運用は `docs/setup.md` と `docs/usage.md` を参照する。

---

## 概要

agent-telemetry は **Claude Code および Codex CLI** の **PR 単位のトークン消費効率** を計測するデータ収集ツールである。
各エージェントの hook が記録したセッションイベントとトランスクリプトを SQLite に変換し、収集したメトリクスを SQL から参照可能にする。可視化はユーザの任意とし、リポジトリ同梱の Grafana ダッシュボードはあくまで参考実装である。

データフロー:

```
Claude Code hooks → ~/.claude/session-index.jsonl + transcript JSONL ┐
                                                                     ├→ agent-telemetry backfill / sync-db
Codex CLI hooks   → ~/.codex/session-index.jsonl  + rollout JSONL    ┘
                                                                     → ~/.claude/agent-telemetry.db (SQLite)
```

DB は単一の `~/.claude/agent-telemetry.db` に集約する。後方互換のためファイル位置は変更せず、`sessions.coding_agent` カラムで `claude` / `codex` を区別する。

---

## hook の登録と役割

hook は `agent-telemetry hook <event> --agent <claude|codex>` のサブコマンド形式で呼び出す。`agent-telemetry` バイナリが PATH 上にある必要がある。登録は dotfiles または手動で行い、`agent-telemetry setup` は登録例を表示するだけで自動登録はしない。

`--agent` を省略した場合は `claude` を既定値とする（既存登録の後方互換）。

### Claude Code

`~/.claude/settings.json` に登録する。

| hook イベント | サブコマンド | 役割 |
|---|---|---|
| `SessionStart` | `agent-telemetry hook session-start --agent claude` | セッション開始メタデータを `~/.claude/session-index.jsonl` に追記 |
| `SessionEnd` | `agent-telemetry hook session-end --agent claude` | 終了時刻と終了理由を `~/.claude/session-index.jsonl` に追記し、SQLite を同期 |
| `Stop` | `agent-telemetry hook stop --agent claude` | 応答完了時に branch から PR を解決して `pr_pinned` で確定 → `backfill` → `sync-db` を実行（ブロッキング） |

### Codex CLI

`~/.codex/config.toml` に `[features] codex_hooks = true` を設定したうえで `[[hooks.<Event>]]` を追加するか、`~/.codex/hooks.json` を配置する。Codex には `SessionEnd` イベントが存在しないため、終了時刻は `Stop` hook で逐次更新する（最後の `Stop` 発火が SessionEnd 相当となる）。

| hook イベント | サブコマンド | 役割 |
|---|---|---|
| `SessionStart` (`startup\|resume`) | `agent-telemetry hook session-start --agent codex` | セッション開始メタデータを `~/.codex/session-index.jsonl` に追記 |
| `Stop` | `agent-telemetry hook stop --agent codex` | 応答完了時に branch から PR を解決して `pr_pinned` で確定し、`ended_at` を更新、`backfill` → `sync-db` を実行（ブロッキング） |
| `PostToolUse` | `agent-telemetry hook post-tool-use --agent codex` | `tool_response` から PR URL を抽出し `pr_urls` に追記（`pr_pinned: true` のセッションでは無視される） |

`Stop` hook はセッション終了を待機するが、cursor 方式・時間条件スキップ・goroutine 並列・8 秒タイムアウトで処理時間を抑制する。

---

## CLI

```
agent-telemetry setup [--agent <claude|codex>]            セットアップ案内を表示（hook 登録は dotfiles または手動）
agent-telemetry uninstall-hooks                           旧 install が書き込んだ hook を ~/.claude/settings.json から削除
agent-telemetry doctor                                    検出された agent ごとに binary / data dir / hook 登録を検証（自動修復はしない）
agent-telemetry backfill [--recheck]                      検出された agent すべての pr_urls / is_merged / review_comments を補完
agent-telemetry sync-db                                   検出された agent すべての JSONL/transcript → SQLite 変換（毎回フル再構築）
agent-telemetry update <session_id> <url>...              session-index.jsonl に PR URL を追加（重複排除）
agent-telemetry update --mark-checked <session_id>...     backfill_checked フラグをセット
agent-telemetry update --by-branch <repo> <branch> <url>  同一 repo+branch の全セッションに URL を追加
agent-telemetry hook <event> [--agent <claude|codex>]     hook サブコマンド（settings.json / config.toml から呼ばれる、既定 claude）
agent-telemetry version                                   version を表示
agent-telemetry install                                   廃止予定 alias。`setup` への誘導 warning を出して同等の案内を表示
```

`setup` は何も書き込まず案内表示のみを行う。書き込みを伴うのは `uninstall-hooks` だけで、対象は `~/.claude/settings.json` に過去 `install` が登録した単一エントリに限定する。Codex 側 (`~/.codex/config.toml`) は人間編集が前提のため自動削除を提供しない。

`backfill --recheck` は cursor を無視してフルスキャンする。

agent の検出は次の優先順位で行う:

1. `--agent` フラグ（hook 経路では必須に近い）
2. 環境変数 `AGENT_TELEMETRY_AGENT`（`claude` / `codex`）
3. データディレクトリの存在（`~/.claude/session-index.jsonl` および `~/.codex/session-index.jsonl` の有無）

`backfill` / `sync-db` / `doctor` は検出された agent **すべて** を対象とする。明示的に絞り込むには `--agent` を指定する。

---

## データファイル

agent ごとに収集元を分離し、SQLite DB は単一に集約する。

| ファイル | 形式 | 役割 |
|---|---|---|
| `~/.claude/session-index.jsonl` | JSON Lines | Claude Code セッション単位のメタデータ |
| `~/.claude/agent-telemetry-state.json` | JSON | Claude Code 用 backfill の cursor |
| `~/.claude/agent-telemetry.toml` | TOML | user 識別子などのユーザ設定。両 agent で共通（後述「ユーザ設定ファイル」） |
| `~/.claude/projects/**/<session-id>.jsonl` | JSON Lines | Claude Code transcript |
| `~/.codex/session-index.jsonl` | JSON Lines | Codex CLI セッション単位のメタデータ |
| `~/.codex/agent-telemetry-state.json` | JSON | Codex CLI 用 backfill の cursor |
| `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl[.zst]` | JSON Lines (任意で zstd 圧縮) | Codex CLI rollout transcript |
| `~/.claude/agent-telemetry.db` | SQLite | sync-db が生成・更新する集計 DB（両 agent を集約）。実行ごとに最新の JSONL/transcript を `sessions` / `transcript_stats` に upsert する |
| `~/.claude/logs/session-index-debug.log` | テキスト | hook のデバッグログ（agent を問わず共通） |

`session-index.jsonl` の形式は agent 共通。`agent-telemetry-state.json` の cursor も agent ごとに独立して管理する。

### `session-index.jsonl` のレコード

```json
{
  "coding_agent": "claude",
  "agent_version": "1.2.3",
  "user_id": "ishii1492@gmail.com",
  "timestamp": "2026-02-27 12:34:56",
  "session_id": "xxx-yyy-zzz",
  "cwd": "/path/to/project",
  "repo": "org/repo",
  "branch": "feature-xxx",
  "pr_urls": ["https://github.com/org/repo/pull/123"],
  "pr_pinned": true,
  "pr_title": "feat: add metrics dashboard",
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

- `coding_agent` は `claude` または `codex`。欠落時は `claude` として扱う（後方互換）。
- `agent_version` は agent 自身が報告するバージョン文字列（取得不能なら空文字列）。バージョン跨ぎでの効率比較に使う。
- `user_id` はセッションを記録したユーザの識別子。SessionStart hook が後述の優先順位で解決して埋める。欠落時は `unknown` として扱う（後方互換）。`sync-db` は欠落レコードに対して現在の解決値で埋め戻し、JSONL にも書き戻す。
- `pr_urls` は PostToolUse / Stop / `update` / `backfill` から重複排除しつつ追記される。`sync-db` は配列の最後の 1 件を採用する。
- `pr_pinned: true` は Stop hook が `gh pr list --head <branch>` で確定した PR にセッションが束縛されたことを示す。pinned レコードに対しては PostToolUse / `update` / `backfill` の URL 追記は **すべて no-op** になる（誤接続防止）。欠落時は `false` として扱う（後方互換）。
- `pr_title` は backfill が `gh pr view --json title` で取得する PR タイトル。欠落時 / 取得失敗時は空文字列として扱う（後方互換）。
- `backfill_checked: true` のレコードは backfill で再 API 呼び出しされない。PR が存在しないブランチで永続スキップされる。
- Codex の場合: `end_reason` は Stop hook の最終発火を記録するため `stop` 固定。`transcript` は `~/.codex/sessions/.../rollout-*.jsonl[.zst]` のフルパス。
- 後方互換: 古いレコードに新フィールドが欠けていても扱える（欠落値は 0 / false / 空文字列、`user_id` のみ `unknown`）。

### `agent-telemetry.toml`（ユーザ設定ファイル）

`~/.claude/agent-telemetry.toml` に以下のキーを置ける。両 agent から共通参照される。

```toml
user = "ishii1492@gmail.com"
```

| キー | 型 | 説明 |
|---|---|---|
| `user` | string | `session-index.jsonl` の `user_id` フィールドに焼き付ける識別子。形式は任意（メールアドレス / pseudonym / UUID 等）。複数マシンで同一人物として束ねたい場合はマシン間で同じ値を揃える運用 |

ファイルが存在しない・キーが欠落・パース不能の場合は無視して次の優先順位にフォールバックする（hook を失敗させない）。

---

## SQLite データモデル

`sync-db` は実行ごとに `sessions` / `transcript_stats` を `INSERT OR REPLACE` で upsert する。スキーマ DDL は埋め込みハッシュと DB 内 `schema_meta` テーブルのハッシュを比較し、不一致時のみフル再構築する（実装詳細は `docs/design.md`）。明示的なマイグレーションコマンドは持たない。

### `sessions` テーブル

| カラム | 型 | 説明 |
|---|---|---|
| `session_id` | TEXT | エージェント発行のセッション ID |
| `coding_agent` | TEXT | `claude` または `codex` |
| `agent_version` | TEXT | agent 自身が報告するバージョン文字列（取得不能なら空） |
| `user_id` | TEXT | セッションを記録したユーザの識別子。`session-index.jsonl` の `user_id` から sync 時に転写。欠落時は `unknown` |
| `timestamp` | TEXT | セッション開始時刻（ISO8601） |
| `cwd` | TEXT | 作業ディレクトリ |
| `repo` | TEXT | リポジトリ（`org/repo` 形式） |
| `branch` | TEXT | ブランチ名 |
| `pr_url` | TEXT | PR URL（`pr_urls` 配列の最後の 1 件） |
| `pr_title` | TEXT | PR タイトル。backfill が `gh pr view --json title` で取得（取得不能なら空） |
| `transcript` | TEXT | transcript ファイルパス |
| `parent_session_id` | TEXT | 親セッション ID。サブエージェント判定用 |
| `ended_at` | TEXT | セッション終了時刻 |
| `end_reason` | TEXT | SessionEnd hook の終了理由（Codex は `stop` 固定） |
| `is_subagent` | INTEGER | `parent_session_id` 非空なら 1 |
| `backfill_checked` | INTEGER | backfill 処理済みなら 1 |
| `is_merged` | INTEGER | PR がマージ済みなら 1 |
| `task_type` | TEXT | ブランチプレフィックス（feat/fix/docs/chore） |
| `review_comments` | INTEGER | PR レビューコメント数 |
| `changes_requested` | INTEGER | CHANGES_REQUESTED レビュー回数 |

PRIMARY KEY は (`session_id`, `coding_agent`) の複合キー。両 agent の UUID が万一衝突しても安全に区別できる。

### `transcript_stats` テーブル

| カラム | 型 | 説明 |
|---|---|---|
| `session_id` | TEXT | セッション ID |
| `coding_agent` | TEXT | `claude` または `codex` |
| `tool_use_total` | INTEGER | ツール呼び出し総数 |
| `mid_session_msgs` | INTEGER | mid-session ユーザーメッセージ数（tool_result 除外） |
| `ask_user_question` | INTEGER | AskUserQuestion 呼び出し回数（Codex では常に 0） |
| `input_tokens` | INTEGER | 入力トークン合計 |
| `output_tokens` | INTEGER | 出力トークン合計 |
| `cache_write_tokens` | INTEGER | cache write トークン合計 |
| `cache_read_tokens` | INTEGER | cache read トークン合計 |
| `reasoning_tokens` | INTEGER | reasoning トークン合計（Claude では常に 0、Codex のみ非ゼロ） |
| `model` | TEXT | セッション内で最後に観測した model |
| `is_ghost` | INTEGER | ユーザー発話相当のエントリが 0 件なら 1 |

PRIMARY KEY は (`session_id`, `coding_agent`)。

トークンの収集元:

- Claude: assistant message の `usage.input_tokens` / `output_tokens` / `cache_creation_input_tokens` / `cache_read_input_tokens`
- Codex: rollout JSONL の `event_msg.payload.type == "token_count"` の最終累積値（input / output / cache_read / cache_write / reasoning）

いずれも該当フィールド欠落時は 0 として扱う。

### `pr_metrics` VIEW

PR 単位の集約ビュー。次のフィルタ条件を適用する。

| 条件 | 理由 |
|---|---|
| `pr_url != ''` | PR 未作成セッションを除外 |
| `is_subagent = 0` | サブエージェントセッションを除外 |
| `is_merged = 1` | 未マージ・放棄 PR を除外（最終成果物のみ） |
| `is_ghost = 0` | ゴーストセッションを除外 |
| `repo NOT IN ('ishii1648/dotfiles')` | dotfiles リポジトリを除外 |

集約カラム: `pr_url`, `pr_title`, `coding_agent`, `user_id`, `model`, `session_count`, `tool_use_total`, `mid_session_msgs`, `ask_user_question`, `input_tokens`, `output_tokens`, `cache_write_tokens`, `cache_read_tokens`, `reasoning_tokens`, `review_comments`, `changes_requested`, `total_tokens`, `fresh_tokens`, `tokens_per_session`, `tokens_per_tool_use`, `pr_per_million_tokens`

`pr_title` は同一 PR に紐づく全セッションで等しい想定だが、安全のため `MAX(s.pr_title)` で集約する（未取得セッションが空文字列を返しても、取得済みセッションのタイトルが採用される）。

`task_type` は集約軸から外れている（ADR-024 で「定量指標は task_type を集計軸に使わない」方針が採用されたため）。`sessions.task_type` カラム自体は後方互換と任意フィルタの余地として残す。

GROUP BY は (`pr_url`, `coding_agent`, `user_id`)。同一 PR が複数 agent / 複数ユーザから触られた場合はそれぞれ別行になる（実運用上ほぼ発生しないが意味的に分離する。pair coding で人物が分かれた場合の集計を正しく扱うため）。

`total_tokens` は input / output / cache write / cache read / reasoning token の合計。`fresh_tokens` は `cache_read_tokens` を除いた合計（input / output / cache write / reasoning）で、長時間セッションで `cache_read_tokens` が支配的になり「重さ」の体感と乖離する問題に対する代替指標。`pr_per_million_tokens` は 100 万 token あたりに完了した PR 数。

### `session_concurrency_daily` / `session_concurrency_weekly` VIEW

トップレベルセッションの同時実行数を時間軸で集約する。`sessions.timestamp` と `sessions.ended_at` の区間重なりから算出し、subagent / ghost / dotfiles を除外する。`coding_agent` ごとに別行で集約する。

集約カラム: `day` または `week_start`, `coding_agent`, `avg_concurrent_sessions`, `peak_concurrent_sessions`

---

これらのカラム/VIEW のメトリクス名・型・ラベル一覧、および何を観察したいか・どう解釈すべきかは `docs/metrics.md` を参照する。

---

## 環境変数

| 変数 | 説明 |
|---|---|
| `AGENT_TELEMETRY_AGENT` | hook / CLI のデフォルト agent（`claude` / `codex`）。`--agent` が省略され、かつ自動検出を行わない経路で参照する |
| `AGENT_TELEMETRY_USER` | `session-index.jsonl` の `user_id` を上書きする。CI / コンテナで決定的に設定したい場合に使う。最優先のソース（`agent-telemetry.toml` や git config より優先される） |
| `CODEX_HOME` | Codex CLI のホームディレクトリ。未指定なら `~/.codex`。Codex 標準と同じ |

---

## 非目標

- 個別の API 課金額の算出（モデルごとの単価変動が大きいため、token 量のみを記録する）
- permission UI 表示回数や `perm_rate` の計測（Claude Code の auto mode 進化で改善対象としての価値が低いと判断したため廃止）
- 未マージ PR や PR なしセッションの効率指標（`pr_metrics` のスコープ外）
- 明示的なマイグレーションコマンド（スキーマ変更は `sync-db` がハッシュ比較で透過的に再構築する）
