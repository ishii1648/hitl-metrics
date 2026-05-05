---
name: grafana-screenshot
description: Grafana ダッシュボードのスクリーンショットを取得し、表示内容を分析・修正する
user_invocable: true
---

# Grafana スクリーンショット検証

Grafana ダッシュボードのスクリーンショットを取得し、パネルの表示内容を分析する。
問題があればダッシュボード JSON を修正し、再キャプチャで確認する。

## フロー

1. Docker 起動確認（未起動なら `make grafana-up-e2e`）
2. `bash e2e/screenshot.sh .outputs/grafana-screenshots` でスクリーンショット取得
3. `.outputs/grafana-screenshots/panel-*.png` を Read ツールで読み込み分析
4. 問題があれば `grafana/dashboards/agent-telemetry.json` を修正
5. 5秒待ち（`updateIntervalSeconds: 5` で Grafana が自動リロード）→ 再キャプチャ
6. 最大 3 回ループ

## 手順詳細

### 1. Docker 起動確認

```bash
curl -sf http://localhost:13000/api/health > /dev/null 2>&1 || make grafana-up-e2e
```

### 2. スクリーンショット取得

```bash
bash e2e/screenshot.sh .outputs/grafana-screenshots
```

### 3. 分析

各 PNG を Read ツールで読み込み、以下を確認:
- データが表示されているか（空グラフでないか）
- 軸ラベル・凡例が正しいか
- レイアウトが崩れていないか

### 4. 修正 → 再取得

`grafana/dashboards/agent-telemetry.json` を Edit ツールで修正後、5秒待ってから再キャプチャ。
Grafana の `updateIntervalSeconds: 5` により、ファイル変更が自動反映される。

## パネル一覧（全11枚）

| ID | 名前 | タイプ |
|---|---|---|
| 1 | サマリ | stat |
| 2 | PR 別テーブル | table |
| 3 | perm rate by PR | barchart |
| 4 | perm count by PR | barchart |
| 5 | session count by PR | barchart |
| 6 | mid-session msgs by PR | barchart |
| 7 | AskUserQuestion by PR | barchart |
| 8 | perm rate 日別トレンド | timeseries |
| 9 | perm rate 週別トレンド | timeseries |
| 10 | ツール別内訳テーブル | table |
| 11 | ツール別内訳バー | barchart |
