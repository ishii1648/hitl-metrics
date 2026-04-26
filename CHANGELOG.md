# Changelog

hitl-metrics の変更履歴。新しいものが上。

## 2026-04-27

- PR 単位のトークン消費効率メトリクスを導入（ADR-023）
  - transcript の `usage` から input / output / cache write / cache read token と model を集計
  - `pr_metrics` に `total_tokens`, `tokens_per_session`, `tokens_per_tool_use`, `pr_per_million_tokens` を追加
  - `perm_rate` / `perm_count` / permission breakdown を SQLite VIEW・Grafana・README から削除
  - `hitl-metrics install` が PermissionRequest / PreToolUse hook を新規登録しないよう変更
  - Grafana ダッシュボードを token 効率中心に再構成

## 2026-03-29

- ダッシュボードのアクショナビリティを改善（ADR-022）
  - Permission ログの enrichment を allowlist 判断向けに変更（`Bash(cmd)` / `Tool(dir/subdir)`）
  - サマリーの mid_session_msgs を PR 平均に変更し、ask_user_question 週別トレンドを追加
  - session_count 分布パネルを追加
  - CHANGES_REQUESTED レビュー回数を backfill・SQLite・pr_metrics・ダッシュボードに追加
- hooks の Shell スクリプトを Go サブコマンドに統一（ADR-021）
  - 5 つの hook を `hitl-metrics hook <event>` サブコマンドとして Go 実装
  - ツールアノテーション（internal/external 分類）を AnnotateTool 共通関数に統合
  - todo-cleanup の TODO パース処理に Go テストを追加
  - `hitl-metrics install` が Go サブコマンド形式で settings.json に登録
  - `go:embed` + Shell スクリプトファイルを削除、単一バイナリで完結
  - `docs/architecture.md` を新規作成
- GitHub Release でバイナリを自動ビルド・配布
  - goreleaser + GitHub Actions でタグ push 時にマルチプラットフォームバイナリを生成（darwin/linux × amd64/arm64）
  - hook スクリプトを go:embed でバイナリに内包、`hitl-metrics install` で `~/.local/share/hitl-metrics/hooks/` に展開
  - `docs/setup.md` を Go ビルド不要のバイナリダウンロード手順に変更
- backfill を launchd 定期バッチから Stop hook に移行（ADR-019）
  - セッション終了時に自動で backfill + sync-db を実行
  - cursor（hitl-metrics-state.json）による増分処理で実行コストを最小化
  - launchd plist テンプレートを削除
  - `hitl-metrics install` コマンドで hooks を自動登録（冪等）
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
