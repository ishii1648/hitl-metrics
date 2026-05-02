# ADR-015: ダッシュボード可視化基盤の選定

## ステータス
採用済み

## 関連 ADR
- 関連: ADR-003（permission UI ログ収集・可視化の基盤）
- 関連: ADR-004（自律率メトリクスの定義）
- 関連: ADR-008（perm rate 時系列トレンド）
- 関連: ADR-012（ツール内訳テーブル）

## コンテキスト

`dashboard/server.py` が集計ロジックと表示ロジックを混在して持っており、計測・集計と可視化の責務が分離されていない。可視化を Prometheus + Grafana 等の既存 monitoring ツールに委ねることで、ダッシュボード実装（純 Python HTTP サーバ + SVG グラフ + HTML テーブル）の保守コストを削減できないか検討した。

調査ログ: [docs/log/2026-03-14-openmetrics-feasibility.md](../log/2026-03-14-openmetrics-feasibility.md)

## 設計案

### 案A: OpenMetrics / Prometheus + Grafana（却下）

`exporter.py` で `/metrics` エンドポイントを提供し、Prometheus でスクレイプ → Grafana で表示する。

**却下理由 — データモデルのミスマッチ:**

- ダッシュボードの核心機能は「任意の日付範囲で PR 別に集計」すること
- Prometheus は「現在の状態のカウンタ/ゲージを scrape する」用途に最適化されており、「過去のイベントログを任意の軸で集計・可視化する」用途とはミスマッチ
- PR 完了後も時系列として残り続け、「この PR の作業期間中の数字」を後から取り出しにくい

ラベルカーディナリティ（年 500〜1000 PR 程度なら許容範囲）や Push vs Scrape（常駐プロセスが必要）は軽微な問題。

### 案B: SQLite + Grafana（採用）

JSONL → SQLite 変換だけで Grafana から直接クエリ可能。最もシンプル。

- JSONL のイベントログ形式と SQL の任意集計が相性良好
- 「任意の日付範囲で PR 別集計」がそのまま SQL で表現できる
- 既存の JSONL 資産をそのまま活かせる

### 案C: その他の代替案（保留）

| ツール | 特徴 |
|---|---|
| ClickHouse + Grafana | イベントログの任意集計が得意。規模が大きくなった場合の選択肢 |
| Loki + Grafana | permission.log のようなログデータに向いている |

個人利用規模では SQLite で十分であり、これらは将来の拡張候補として保留する。

### 変更が必要なファイル

| ファイル | 変更内容 |
|---|---|
| `batch/` 配下（新規） | JSONL → SQLite 変換バッチ |
| `dashboard/server.py` | 集計ロジックを SQLite クエリに置換、または廃止 |
| Grafana ダッシュボード定義（新規） | SQLite データソースを使った可視化定義 |

## 受け入れ条件

- [x] JSONL から SQLite への変換バッチが存在し、全メトリクス（permission rate, session count, tool breakdown）を格納できる
- [x] Grafana から SQLite データソース経由で PR 別集計クエリが実行できる
- [x] 任意の日付範囲フィルタで PR 別メトリクスが表示される
- [x] 既存の `dashboard/server.py` の集計・表示機能が Grafana ダッシュボードで代替されている
