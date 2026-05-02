# ADR-016: Grafana E2E スクリーンショット検証基盤

## ステータス
採用済み

## 関連 ADR
- 依存: ADR-015（SQLite + Grafana を可視化基盤として採用）

## コンテキスト

ADR-015 で SQLite + Grafana への移行が完了したが、ダッシュボード JSON の表示確認手段がなかった。JSON を編集しても実際の描画結果を確認するにはブラウザで手動確認するしかなく、以下の2つの要件があった：

1. **Claude の visual ループ**: スクリーンショット取得 → 画像分析 → JSON 修正 → 再取得を自律的に回せる仕組み
2. **人の目視確認**: ブラウザで Grafana を開いてダッシュボードを確認できる環境

## 設計案

### 案A: Grafana Image Renderer API（採用）

Docker Compose で Grafana + Image Renderer コンテナを起動し、Render API（`/render/d-solo/...`）経由で各パネルの PNG を取得する。

- Grafana の公式レンダリング機構をそのまま利用でき、ブラウザ自動操作が不要
- パネル単位でスクリーンショットが取れるため、問題箇所の特定が容易
- Docker Compose 一発で環境が立ち上がり、`localhost:13000` で人の目視確認もできる
- `updateIntervalSeconds: 5` により JSON 編集が自動反映され、修正→再確認のループが高速

### 案B: Playwright によるブラウザ自動操作（却下）

Playwright でヘッドレス Chromium を起動し、Grafana の画面をスクリーンショットする。

**却下理由:**
- Node.js / Python の追加依存が必要（Go プロジェクトに異質）
- ログイン処理・ページ遷移・パネル読み込み待機など、壊れやすいブラウザ操作コードが増える
- Grafana 自身が Image Renderer で同等の機能を提供しており、車輪の再発明になる

### 設計上の補足

**datasource uid の固定化**: ダッシュボード JSON が `${DS_CLAUDEDOG}` テンプレート変数を使っていたが、Grafana の provisioning はこの変数を解決しない。全11パネルの datasource uid を固定値 `"hitl-metrics"` に置換し、`__inputs` セクションを削除した。ホスト環境の datasource yaml にも `uid: hitl-metrics` を明示的に追加して一致させた。

**テストフィクスチャ**: 8セッション・3PR・15パーミッションイベント・7 transcript で全11パネルにデータが表示される最小構成を設計。subagent セッション、dotfiles リポジトリ、ghost セッション（transcript なし）を含め、pr_metrics VIEW のフィルタ条件も検証できるようにした。

### 変更が必要なファイル

| ファイル | 変更内容 |
|---|---|
| `grafana/dashboards/hitl-metrics.json` | `${DS_CLAUDEDOG}` → `"hitl-metrics"` 置換、`__inputs` 削除 |
| `grafana/provisioning/datasources/hitl-metrics.yaml` | `uid: hitl-metrics` 追加 |
| `grafana/provisioning/datasources/hitl-metrics-docker.yaml` | Docker 用 datasource（新規） |
| `grafana/provisioning/dashboards/hitl-metrics-docker.yaml` | Docker 用 dashboard provisioning（新規） |
| `docker-compose.yaml` | Grafana + Image Renderer 構成（新規） |
| `Makefile` | `grafana-fixtures`, `grafana-up`, `grafana-down`, `grafana-screenshot`（新規） |
| `e2e/testdata/` | テストフィクスチャ一式（新規） |
| `e2e/gen_testdb_test.go` | テスト用 SQLite DB 生成（新規） |
| `e2e/screenshot.sh` | 全11パネル PNG 取得スクリプト（新規） |
| `.claude/skills/grafana-screenshot/SKILL.md` | スクリーンショット分析スキル（新規） |

## 受け入れ条件

- [x] `make grafana-up` で Grafana が `http://localhost:13000` で起動する
- [x] `make grafana-screenshot` で `.outputs/grafana-screenshots/` に 11枚の PNG が生成される
- [x] 各パネルにテストデータが表示される（空グラフでない）
- [x] ブラウザで `http://localhost:13000` を開きダッシュボードを目視確認できる
- [x] `/grafana-screenshot` スキルで Claude が分析→修正ループを回せる
