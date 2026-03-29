# hitl-metrics

Claude Code の人の介入率を追跡・可視化する計測ツール（hook・CLI・ダッシュボード）。

## アーキテクチャ

3層構成:

1. **データ収集層** (`hooks/`) — Claude Code hook で session/permission/tool-use イベントを JSONL/log に記録
2. **データ変換層** (`cmd/hitl-metrics/`, `internal/`) — Go CLI で JSONL/log → SQLite 変換・PR URL 補完
3. **可視化層** (`grafana/`) — Grafana ダッシュボードで介入率・ツール分布を表示

データフロー: `hooks → ~/.claude/*.jsonl|log → hitl-metrics sync-db → SQLite → Grafana`

## データモデル（SQLite）

- **sessions** — セッション単位の基本情報（session_id, repo, branch, pr_url, is_subagent 等）
- **permission_events** — permission UI 発生イベント（timestamp, session_id, tool）
- **transcript_stats** — トランスクリプトから抽出した統計（tool_use_total, mid_session_msgs, ask_user_question, is_ghost）
- **pr_metrics**（VIEW） — PR 単位で session_count, perm_count, perm_rate を集約

## CLI コマンド

```
hitl-metrics update <session_id> <url>...          # PR URL を追加
hitl-metrics update --mark-checked <session_id>... # backfill_checked をセット
hitl-metrics update --by-branch <repo> <branch> <url>  # ブランチ全セッションに URL 追加
hitl-metrics backfill [--recheck]                  # PR URL の一括補完
hitl-metrics sync-db                               # JSONL/log → SQLite 変換
hitl-metrics install [--hooks-dir <path>]          # hooks を ~/.claude/settings.json に登録
```

## セッションモード

### 設計セッション（main ブランチ）

- 変更対象: `docs/adr/`, `TODO.md`, `CLAUDE.md` のみ
- コード変更禁止（Spike を除く）
- ADR には必ず「変更が必要なファイル」を記載し、並列実装時の衝突リスクを事前把握する
- 複数 ADR の対象ファイルが重複する場合、パッケージ分割・実装順序・許容判断を設計セッション内で行う
- 仕様が固まったタスクは受け入れ条件（`- [ ]`）を定義し「検討中」→「実装待ち」に移動する
- `/dispatch` は「実装待ち」セクションのタスクのみを対象に実装セッションを一括起動する

### 実装セッション（feature ブランチ / worktree）

- 対象タスクを 1 つ実装する（TODO.md の受け入れ条件に従う）
- worktree 作成: `/dispatch` で自動、手動なら `gw_add feat/adr-017`
- TODO.md の受け入れ条件を満たすまで実装 → 検証 → 修正
- main の `docs/adr/` は変更しない（ステータス更新は merge 後に main で実施）

## 開発規約

### 意思決定の記録方針

- 複数コミットにまたがる設計判断 → `docs/adr/` に ADR を作成
- 1コミット内で完結する判断 → Contextual Commits のアクション行で記録
- chore/リファクタなど意思決定を伴わない変更 → アクション行不要

### コミット

Contextual Commits を使用。Conventional Commits プレフィックス + 構造化されたアクション行でコミットの意図を記録する。

### ブランチ命名

`feat/`, `fix/`, `docs/`, `chore/` + kebab-case（例: `feat/add-sync-db`）

### テスト

```fish
go test ./...                          # 全テスト
make grafana-screenshot                # E2E: Grafana スクリーンショット検証
```
