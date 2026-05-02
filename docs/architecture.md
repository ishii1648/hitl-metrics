# hitl-metrics アーキテクチャ

現在の設計スナップショット。「なぜそう決めたか」は各 ADR を参照。

> 更新ルール: ADR のステータスを「採用済み」に変更した設計セッションで、このファイルも合わせて更新する。

---

## 全体構成

3層構成:

```
hooks（Go）
    ↓ ~/.claude/session-index.jsonl
    ↓ ~/.claude/hitl-metrics-state.json
hitl-metrics CLI（Go）
    ↓ hitl-metrics sync-db / backfill
SQLite（~/.claude/hitl-metrics.db）
    ↓
Grafana ダッシュボード
```

---

## データ収集層（hooks）

実装: `hitl-metrics hook <event>` サブコマンド（`internal/hook/` パッケージ）
設定場所: `~/.claude/settings.json`（dotfiles または手動で登録。検証は `hitl-metrics doctor`）

| hook イベント | サブコマンド | 役割 | 出力先 |
|---|---|---|---|
| SessionStart | `hitl-metrics hook session-start` | セッション開始時のメタデータを記録 + TODO 完了タスク移動 | `~/.claude/session-index.jsonl` |
| SessionEnd | `hitl-metrics hook session-end` | セッション終了時刻と終了理由を記録し、DB を同期 | `~/.claude/session-index.jsonl`, `~/.claude/hitl-metrics.db` |
| Stop | `hitl-metrics hook stop` | 応答完了時に backfill → sync-db を実行（※） | — |

> ※ Stop hook は**ブロッキング実行**（backfill && sync-db が完了するまでセッション終了を待機）。ただしセッション終了時に実行されるためユーザー操作への影響は限定的。処理時間は cursor ベースの増分処理・Phase 2 の時間条件スキップ（1時間未満なら省略）・goroutine 8並列 + 8秒タイムアウトで抑制している。

> [ADR-001](adr/001-claude-session-index.md), [ADR-019](adr/019-backfill-stop-hook-migration.md), [ADR-021](adr/021-migrate-shell-hooks-to-go-subcommands.md), [ADR-023](adr/023-pr-token-efficiency-metrics.md)

### 中間ファイル

| ファイル | 形式 | 内容 |
|---|---|---|
| `~/.claude/session-index.jsonl` | JSON Lines | セッション単位のメタデータ。SessionStart で新規追記、SessionEnd/backfill で更新 |
| `~/.claude/hitl-metrics-state.json` | JSON | backfill の cursor（last_backfill_offset, last_meta_check） |

> **なぜ中間ファイルを挟むか:** hook は Claude Code セッション中に同期実行されるため高速に完了する必要がある。追記のみの軽量フォーマット（JSONL）に書き出し、構造化 DB への変換は `sync-db` に委譲することで「書き込みは軽く・読み込みは構造化」を実現している。また `sync-db` は毎回 DROP & CREATE でフル再構築するため、中間ファイルがソースオブレコードとして機能し、DB 破損時も再生成できる。

---

## データ変換層（Go CLI）

### コマンド一覧

```
hitl-metrics update <session_id> <url>...           # PR URL を session-index.jsonl に追加（重複排除）
hitl-metrics update --mark-checked <session_id>...  # backfill_checked フラグをセット
hitl-metrics update --by-branch <repo> <branch> <url>  # 同一 repo+branch の全セッションに URL を追加
hitl-metrics backfill [--recheck]                   # pr_urls が空のセッションを GitHub API で補完
hitl-metrics sync-db                                # JSONL/transcript → SQLite 変換（DROP & CREATE）
hitl-metrics install                                # セットアップ案内（hook 登録は dotfiles 側で行う）
hitl-metrics install --uninstall-hooks              # 旧 install で書き込んだ hook を ~/.claude/settings.json から削除
hitl-metrics doctor                                 # binary / data dir / hook 登録の検証（自動修復はしない）
```

### backfill の処理フロー

> [ADR-006](adr/006-session-index-pr-url-backfill-cron-batch.md), [ADR-019](adr/019-backfill-stop-hook-migration.md)

```
~/.claude/hitl-metrics-state.json（cursor）を読み込み
    ↓
Phase 1: cursor 以降の新規エントリから pr_urls 空かつ backfill_checked=0 を抽出
    → (repo, branch) でグループ化して gh pr view を1回実行
    → session-index.jsonl に pr_url を追記・backfill_checked=1 に更新
    ↓
Phase 2: last_meta_check から一定間隔経過時のみ実行
    → 既存 PR の is_merged・review_comments を再チェック
    ↓
cursor（last_backfill_offset, last_meta_check）を更新
```

`--recheck` を指定すると cursor を無視してフルスキャン。

### sync-db の処理フロー

中間ファイル → SQLite への ETL（Extract-Transform-Load）。毎回 DROP & CREATE でフル再構築し、1トランザクションで一括 COMMIT する。

```
1. session-index.jsonl を全件読み込み、session_id で重複排除（last wins）
   → sessions テーブルに INSERT
     - pr_urls 配列の最後の1件を pr_url カラムに変換
     - parent_session_id の有無から is_subagent フラグを導出
     - ended_at / end_reason からセッション終了情報を保持
     - ブランチプレフィックス（feat/, fix/ 等）から task_type を抽出

2. 各セッションの transcript ファイルをパース
   → transcript_stats テーブルに INSERT
     - tool_use_total, mid_session_msgs, ask_user_question を集計
     - input/output/cache token と model を集計
     - type:"user" エントリなしなら is_ghost = 1

3. pr_metrics / session_concurrency_daily / session_concurrency_weekly VIEW がスキーマ定義済みのため自動利用可能
```

---

## データモデル（SQLite）

DB パス: `~/.claude/hitl-metrics.db`
再生成: `sync-db` 実行時に DROP & CREATE（毎回フル再構築）

### sessions テーブル

| カラム | 型 | 説明 |
|---|---|---|
| session_id | TEXT PK | Claude Code セッション ID |
| timestamp | TEXT | セッション開始時刻（ISO8601） |
| cwd | TEXT | 作業ディレクトリ |
| repo | TEXT | リポジトリ（`org/repo` 形式） |
| branch | TEXT | ブランチ名 |
| pr_url | TEXT | PR URL（単一値） |
| transcript | TEXT | transcript ファイルパス |
| parent_session_id | TEXT | 親セッション ID（サブエージェント判定用） |
| ended_at | TEXT | セッション終了時刻 |
| end_reason | TEXT | SessionEnd hook の終了理由 |
| is_subagent | INTEGER | サブエージェントなら 1 |
| backfill_checked | INTEGER | backfill 処理済みなら 1 |
| is_merged | INTEGER | PR がマージ済みなら 1 → [ADR-018](adr/018-metrics-redesign-merged-pr-scope.md) |
| task_type | TEXT | feat/fix/docs/chore（ブランチプレフィックスから自動抽出）→ [ADR-018](adr/018-metrics-redesign-merged-pr-scope.md) |
| review_comments | INTEGER | PR レビューコメント数 → [ADR-018](adr/018-metrics-redesign-merged-pr-scope.md) |
| changes_requested | INTEGER | CHANGES_REQUESTED レビュー回数 → [ADR-022](adr/022-dashboard-actionability-improvements.md) |

### transcript_stats テーブル

| カラム | 型 | 説明 |
|---|---|---|
| session_id | TEXT PK | セッション ID |
| tool_use_total | INTEGER | ツール呼び出し総数 |
| mid_session_msgs | INTEGER | mid-session ユーザーメッセージ数（tool_result 除外） |
| ask_user_question | INTEGER | AskUserQuestion 呼び出し回数 |
| input_tokens | INTEGER | `usage.input_tokens` の合計 |
| output_tokens | INTEGER | `usage.output_tokens` の合計 |
| cache_write_tokens | INTEGER | `usage.cache_creation_input_tokens` の合計 |
| cache_read_tokens | INTEGER | `usage.cache_read_input_tokens` の合計 |
| model | TEXT | セッション内で最後に観測した model |
| is_ghost | INTEGER | ゴーストセッションなら 1（`type:"user"` エントリなし）→ [ADR-011](adr/011-session-count-excludes-subagent-sessions.md) |

### pr_metrics VIEW

PR 単位の集約ビュー。以下の条件でフィルタ:

| フィルタ条件 | 理由 |
|---|---|
| `pr_url != ''` | PR 未作成セッションを除外 |
| `is_subagent = 0` | サブエージェントセッションを除外 → [ADR-011](adr/011-session-count-excludes-subagent-sessions.md) |
| `is_merged = 1` | 未マージ・放棄 PR を除外（最終成果物のみ）→ [ADR-018](adr/018-metrics-redesign-merged-pr-scope.md) |
| `is_ghost = 0` | ゴーストセッションを除外 → [ADR-011](adr/011-session-count-excludes-subagent-sessions.md) |
| `repo NOT IN ('ishii1648/dotfiles')` | dotfiles リポジトリを除外 |

集約カラム: `pr_url`, `task_type`, `model`, `session_count`, `tool_use_total`, `mid_session_msgs`, `ask_user_question`, `input_tokens`, `output_tokens`, `cache_write_tokens`, `cache_read_tokens`, `review_comments`, `changes_requested`, `total_tokens`, `tokens_per_session`, `tokens_per_tool_use`, `pr_per_million_tokens`

`total_tokens` は input / output / cache write / cache read token の合計。`pr_per_million_tokens` は 100万 token あたりに完了できた PR 数。> [ADR-023](adr/023-pr-token-efficiency-metrics.md)

### session_concurrency_daily / session_concurrency_weekly VIEW

トップレベル Claude Code セッションの同時実行数を時間軸で集約するビュー。`sessions.timestamp` と `sessions.ended_at` の区間重なりから算出し、subagent・ghost・dotfiles は除外する。

集約カラム: `day` / `week_start`, `avg_concurrent_sessions`, `peak_concurrent_sessions`

---

## 可視化層（Grafana）

> [ADR-015](adr/015-dashboard-visualization-backend-selection.md), [ADR-016](adr/016-grafana-e2e-screenshot-testing.md)

データソース: SQLite（`hitl-metrics.db`）
E2E テスト: `make grafana-screenshot`（Grafana Image Renderer でスクリーンショット検証）

### ダッシュボード パネル構成

| パネル | 種別 | 内容 |
|---|---|---|
| サマリカード | Stat | merged PR 数、total tokens、avg tokens / PR、PR / 1M tokens、changes requested |
| 週別 token 消費 | Time series | total_tokens と merged PR 数の推移 |
| 週別 PR / 1M tokens | Time series | token 効率の推移 |
| タスク種別 token バーチャート | Bar chart | task_type 別 avg tokens / PR |
| PR 別 token スコアカード | Table | pr_url, task_type, model, total_tokens, tokens_per_session, tokens_per_tool_use, pr_per_million_tokens, token 内訳, session_count, tool_use_total, mid_session_msgs, ask_user_question, review_comments, changes_requested |
| PR 別 session_count 分布 | Bar chart | 多セッション PR の外れ値検出 |
| PR 別 tokens / tool_use | Bar chart | 1 tool_use あたりの token 消費が大きい PR |
