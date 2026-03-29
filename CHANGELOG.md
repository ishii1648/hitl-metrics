# Changelog

hitl-metrics の変更履歴。新しいものが上。

## 2026-03-29

- Go 静的テスト CI を導入（golangci-lint + go test -race）

## 2026-03-23

- メトリクス体系を再設計 — merged PR スコープ・タスク種別分類・review_comments 追加（ADR-018）
- ダッシュボードをナラティブ構造に再構成（14 パネル → 6 パネル + 4 セクション）
- ダッシュボードの時間フィルタ・ラベル表示を修正
- プロジェクト名を claudedog → hitl-metrics にリネーム
- セットアップガイドを追加（docs/setup.md）
- macOS launchd による定期同期用 plist テンプレートを追加（configs/launchd/）
- `git-ship` skill を追加（CHANGELOG 更新チェック付き ship フロー）

## 2026-03-21

- ADR-017: 設計/実装セッション分離の自動ディスパッチ
- Python バッチ2本（session-index-update.py, session-index-backfill-batch.py）を Go に移植
- dashboard/server.py を削除し、SQLite + Grafana に可視化を移行（ADR-015）
- `hitl-metrics sync-db` サブコマンドを新規追加（JSONL/log → SQLite 変換）
- `hitl-metrics update` / `hitl-metrics backfill` サブコマンドで Python 版を置換
- Grafana ダッシュボード定義を追加（grafana/）
- bash ラッパー `hitl-metrics` を削除（Go バイナリが代替）
- Python 依存をゼロに

## 2026-03-06

- `configs/claude/scripts/` からトップレベル `hitl-metrics/` に移動し、ディレクトリ隔離を実施（ADR-052）
- 開発プロセスを ADR 駆動から TODO.md + CHANGELOG.md ベースに移行
