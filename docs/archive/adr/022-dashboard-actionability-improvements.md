# ADR-022: ダッシュボードのアクショナビリティ改善

## ステータス

採用済み

## 関連 ADR

- 依存: ADR-018（メトリクス体系・pr_metrics VIEW の構造を前提）
- 関連: ADR-009（permission UI 内訳モニタリング）
- 関連: ADR-012（ツール別テーブル）
- 関連: ADR-021（hooks の Go 移行 — A の hook 変更は Shell で先行実装し、Go 移行時に移植）

## コンテキスト

ADR-018 で merged PR スコープ・タスク種別分類を導入し、計測基盤は整った。
しかし現在のダッシュボードは数値を表示するだけで「次に何をすればよいか」のアクションに繋がらない。

### 1. ツール別 permission 分布の粒度不足

現在の表示は `Bash: 43%`, `Edit: 32%` のレベル。
Bash の 43% が `go test` なのか `rm` なのかで対処が全く異なるが判別できない。
Read/Edit も `internal` としか分からず、どのディレクトリの permission を allowlist すべきか決められない。

### 2. KPI の文脈欠如

- `mid_session_msgs` が全 PR 合計で表示されており、PR 平均が不明（12 PR で 24 → 平均 2 なのか不明瞭）
- `ask_user_question`（仕様不確実性の代理指標）が PR テーブルにのみ存在し、トレンドがない

### 3. review_comments の質的区分がない

`review_comments` は全コメント（LGTM 含む）をカウントしており、品質指標として機能しない。
「修正を求められた回数」（CHANGES_REQUESTED）を区分すれば、成果物品質の代理指標になる。

### 4. session_count の分布が見えない

PR ごとの session_count はテーブルにあるが、分布がない。
多セッション PR（外れ値）の特定ができず、タスク分割やプロンプト改善のトリガーにならない。

## 設計案

### A. Permission ログのディレクトリパターン集約

hook の enrichment を改善し、allowlist 設定に直結する粒度にする。

**Bash**: `Bash(CMD(LOC))` → `Bash(CMD)` に変更。コマンド名のみ記録する。internal/external は Bash では対処に影響しない。
- 例: `Bash(git)`, `Bash(make)`, `Bash(go)`

**Read/Edit/Write/Grep**: `TOOL(internal|external)` → internal の場合、CWD からの相対パスの先頭 2 階層を記録する。
- 例: `Edit(internal/syncdb)`, `Read(docs/adr)`, `Write(grafana/dashboards)`
- ルートファイルは `TOOL(.)` とする
- external は引き続き `TOOL(external)` のまま

ダッシュボードはこの enriched tool name で GROUP BY し、カーディナリティを抑えつつアクションに繋がる粒度を実現する。

### B. KPI の改善

- サマリーの `mid_session_msgs`: `SUM` → `ROUND(AVG(...), 1)` に変更（PR 平均）
- `ask_user_question`: 週別トレンドチャートを追加

### C. CHANGES_REQUESTED の区分

- `gh pr view --json` のフィールドに `reviews` を追加
- reviews 配列から `state == "CHANGES_REQUESTED"` の件数を抽出
- `sessions` テーブルに `changes_requested INTEGER NOT NULL DEFAULT 0` カラムを追加
- `pr_metrics` VIEW に `MAX(s.changes_requested) AS changes_requested` を追加
- ダッシュボード: `review_comments` とは別に `changes_requested` を表示

### D. session_count 分布パネル

PR ごとの `session_count` のバーチャートを追加。多セッション PR を特定可能にする。

### 変更が必要なファイル（affected-scope）

| ファイル / パッケージ | 変更内容 |
|---|---|
| `hooks/permission-log.sh` | enrichment ロジック変更（ディレクトリパターン） |
| `internal/install/hooks/permission-log.sh` | 同上（embed コピー） |
| `internal/backfill/backfill.go` | `--json` に `reviews` 追加、CHANGES_REQUESTED 解析 |
| `internal/sessionindex/sessionindex.go` | `ChangesRequested` フィールド追加 |
| `internal/syncdb/schema.go` | `changes_requested` カラム追加、VIEW 更新 |
| `internal/syncdb/syncdb.go` | 新フィールドのマッピング |
| `grafana/dashboards/hitl-metrics.json` | 全パネル更新（A, B, C, D） |
| `e2e/testdata/permission.log` | enriched 形式のテストデータ |
| `e2e/testdata/session-index.jsonl` | `changes_requested` フィールド追加 |
