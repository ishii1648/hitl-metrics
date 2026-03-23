# TODO

hitl-metrics の開発タスクを管理する。完了したら CHANGELOG.md に記録して削除する。

## 実装待ち

- Go 静的テスト CI 導入
  - [ ] `.github/workflows/go.yml` が作成され、PR トリガーで Go ファイル変更時に実行される
  - [ ] `golangci-lint run` が CI 上で成功する
  - [ ] `go test -race ./...` が CI 上で成功する

- ADR-018: メトリクス体系の再設計
  - 関連 ADR: [ADR-018](docs/adr/018-metrics-redesign-merged-pr-scope.md)
  - [ ] sessions テーブルに is_merged, task_type, review_comments カラムが追加される
  - [ ] pr_metrics ビューの LEFT JOIN 膨張バグが修正される
  - [ ] pr_metrics ビューが merged PR のみをフィルタする
  - [ ] task_type がブランチプレフィックスから自動抽出される
  - [ ] backfill コマンドで merged 判定・レビューコメント数が取得される
  - [ ] Grafana ダッシュボードが再構成される（merged PR フィルタ、タスク種別グループ化、review_comments パネル）

## 検討中

- [x] Go 静的テスト CI 導入
- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示

## 進行中

（なし）
