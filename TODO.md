# TODO

hitl-metrics の開発タスクを管理する。完了したら CHANGELOG.md に記録して削除する。

## 実装待ち

- ADR-022-BD: ダッシュボード KPI 改善 + session_count 分布
  - 関連 ADR: [ADR-022](docs/adr/022-dashboard-actionability-improvements.md)
  - [ ] サマリーの mid_session_msgs が PR 平均（AVG）で表示される
  - [ ] ask_user_question の週別トレンドチャートが追加される
  - [ ] session_count の分布パネルが追加される
- ADR-022-A: permission ディレクトリパターン集約
  - 関連 ADR: [ADR-022](docs/adr/022-dashboard-actionability-improvements.md)
  - [ ] Bash の enrichment が `Bash(CMD)` 形式（コマンド名のみ）に変更される
  - [ ] Read/Edit/Write/Grep の enrichment が `TOOL(dir1/dir2)` 形式（相対パス先頭2階層）に変更される
  - [ ] テストデータ（permission.log）が enriched 形式に更新される
  - [ ] ダッシュボードのツール別パネルが enriched tool name で集約表示される
- ADR-022-C: CHANGES_REQUESTED 区分
  - 関連 ADR: [ADR-022](docs/adr/022-dashboard-actionability-improvements.md)
  - [ ] backfill が `gh pr view --json reviews` から CHANGES_REQUESTED 件数を取得する
  - [ ] sessions テーブルに `changes_requested` カラムが追加される
  - [ ] pr_metrics VIEW に `changes_requested` が含まれる
  - [ ] ダッシュボードに changes_requested が表示される
  - [ ] テストデータ（session-index.jsonl）に `changes_requested` フィールドが追加される

## 未着手

## 検討中

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示

## 進行中
