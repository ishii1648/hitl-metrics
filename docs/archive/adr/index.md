# ADR 一覧

hitl-metrics の設計判断を記録する ADR（Architecture Decision Record）の一覧。

## サマリ

| # | ステータス | 領域 | タイトル | ADR |
|:---:|:---:|:---|:---|:---|
| 001 | 採用済み | hooks | Claude セッションを PR ベースで追跡する | [ADR-001](001-claude-session-index.md) |
| 002 | 採用済み | hooks | Claude Code 起動時の session-index.sh ネットワーク呼び出し最適化 | [ADR-002](002-claude-session-index-startup-optimization.md) |
| 003 | 採用済み | hooks | Notification hook による permission UI 表示回数の計測 | [ADR-003](003-claude-permission-ui-count-via-hook.md) |
| 004 | Superseded | metrics | 作業量で正規化した Claude 自律度指標の導入 | [ADR-004](004-claude-autonomy-rate-per-work-unit.md) |
| 005 | 採用済み | hooks | Stop フックで既存 PR URL を補完する | [ADR-005](005-session-index-pr-url-backfill-on-stop.md) |
| 006 | 採用済み | batch | session-index pr_urls バックフィルを cron バッチ方式に移行する | [ADR-006](006-session-index-pr-url-backfill-cron-batch.md) |
| 007 | 採用済み | metrics / dashboard | claude-stats の人の介入指標を拡張する | [ADR-007](007-claude-human-intervention-metrics-expansion.md) |
| 008 | 採用済み | dashboard | perm UI 発生率の時系列トレンドグラフを追加する | [ADR-008](008-perm-rate-time-series-trend.md) |
| 009 | 採用済み | hooks / dashboard | permission UI 内訳の監視 | [ADR-009](009-permission-ui-breakdown-monitoring.md) |
| 010 | 採用済み | batch | session-index-backfill-batch.py の並列実行化 | [ADR-010](010-session-index-backfill-parallel-execution.md) |
| 011 | 採用済み | dashboard | セッション数計測からサブエージェントセッションを除外する | [ADR-011](011-session-count-excludes-subagent-sessions.md) |
| 012 | 採用済み | dashboard | ダッシュボードにツール別 permission UI テーブルを追加 | [ADR-012](012-tool-breakdown-table-in-dashboard.md) |
| 013 | 部分廃止 | 複合 | hitl-metrics をトップレベルディレクトリに隔離し開発プロセスを分離する | [ADR-013](013-claude-stats-directory-isolation.md) |
| 014 | 採用済み | hooks | permission-log を PermissionRequest フックに移行する | [ADR-014](014-permission-log-use-permission-request-hook.md) |
| 015 | 採用済み | dashboard | ダッシュボード可視化基盤の選定 | [ADR-015](015-dashboard-visualization-backend-selection.md) |
| 016 | 採用済み | e2e | Grafana E2E スクリーンショット検証基盤 | [ADR-016](016-grafana-e2e-screenshot-testing.md) |
| 018 | 部分廃止 | metrics / dashboard | メトリクス体系の再設計 — merged PR スコープとタスク種別分類 | [ADR-018](018-metrics-redesign-merged-pr-scope.md) |
| 021 | 採用済み | hooks / CLI | hooks の Shell スクリプトを Go サブコマンドに統一する | [ADR-021](021-migrate-shell-hooks-to-go-subcommands.md) |
| 023 | 採用済み | metrics / dashboard | PR 単位のトークン消費効率メトリクスを導入する | [ADR-023](023-pr-token-efficiency-metrics.md) |
| 024 | 採用済み | metrics / dashboard | hitl-metrics の責務範囲 — 定量指標と定性評価の分離 | [ADR-024](024-quantitative-scope-and-task-type-removal.md) |
