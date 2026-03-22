# ADR-018: メトリクス体系の再設計 — merged PR スコープとタスク種別分類

## ステータス

採用済み

## コンテキスト

ADR-007 で定義したメトリクス体系には以下の課題がある。

### 1. 未マージ PR のノイズ

現行の `pr_metrics` VIEW は `pr_url != ''` でフィルタしているだけなので、
作業中（open）や放棄（closed but not merged）の PR も含まれる。
これらは最終成果物ではないため、振り返り指標としてノイズになる。

### 2. LEFT JOIN 膨張バグ

`pr_metrics` VIEW が `sessions × permission_events` を LEFT JOIN しているため、
1セッションに N 件の permission_events があると `tool_use_total` 等が N 倍に膨張する。
perm_rate の算出結果が不正確になる根本バグ。

### 3. タスク種別ごとの傾向が見えない

feat / fix / docs / chore でタスクの性質が異なるのに、
すべて同列に並べて比較している。
ブランチプレフィックスからタスク種別を自動抽出して集計すれば、
種別ごとの介入傾向（例: fix は feat より perm_rate が低い等）が見える。

### 4. コードレビューフィードバックの可視化

PR レビューコメント数は「成果物の品質に対する外部フィードバック」として有用だが、
現在のデータモデルにはこのフィールドがない。

## 決定

### A. merged PR フィルタ

`pr_metrics` VIEW に `s.is_merged = 1` 条件を追加。
`sessions` テーブルに `is_merged INTEGER NOT NULL DEFAULT 0` カラムを追加。
`backfill` コマンドで `gh pr list/view` から state を取得し、
`MERGED` なら `is_merged=true` をセッションに記録する。

### B. LEFT JOIN 膨張バグの修正

`permission_events` の JOIN をサブクエリに変更:

```sql
LEFT JOIN (
    SELECT session_id, COUNT(*) AS perm_count
    FROM permission_events
    GROUP BY session_id
) pe_agg ON s.session_id = pe_agg.session_id
```

これにより sessions × transcript_stats は 1:1 のまま、
perm_count は事前集計済みの値を結合する。

### C. task_type カラム

`sessions` テーブルに `task_type TEXT NOT NULL DEFAULT ''` を追加。
`sync-db` 時に `branch` カラムから `feat/fix/docs/chore` プレフィックスを抽出。
`pr_metrics` VIEW に `MAX(s.task_type) AS task_type` を追加。

### D. review_comments カラム

`sessions` テーブルに `review_comments INTEGER NOT NULL DEFAULT 0` を追加。
`backfill` コマンドで `gh pr list/view --json comments` から取得。
`pr_metrics` VIEW に `MAX(s.review_comments) AS review_comments` を追加。

### E. Grafana ダッシュボード再構成

- PR 別テーブルに `task_type`, `review_comments` カラム追加
- タスク種別 perm rate バーチャート追加
- タスク種別テーブル追加（種別ごとの PR 数, perm_rate, avg review_comments）
- review comments by PR バーチャート追加

## 受け入れ条件

- [x] sessions テーブルに is_merged, task_type, review_comments カラムが追加される
- [x] pr_metrics ビューの LEFT JOIN 膨張バグが修正される
- [x] pr_metrics ビューが merged PR のみをフィルタする
- [x] task_type がブランチプレフィックスから自動抽出される
- [x] backfill コマンドで merged 判定・レビューコメント数が取得される
- [x] Grafana ダッシュボードが再構成される（merged PR フィルタ、タスク種別グループ化、review_comments パネル）

## 影響

- `sync-db` で DB が再構築されるため、既存データへの影響なし（毎回 DROP + CREATE）
- `backfill` を再実行すれば既存セッションの `is_merged` / `review_comments` も補完される
- マージ前の PR はダッシュボードに表示されなくなるが、これは意図した動作
