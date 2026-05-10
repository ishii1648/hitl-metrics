---
decision_type: process
affected_paths:
  - docs/setup.md
  - docs/usage.md
  - Makefile
  - README.md
tags: [server, docs, grafana, operations]
---

# サーバ運用ドキュメント — ローカル Grafana 構成を流用

Created: 2026-05-10

## 概要

サーバ送信機能を利用者が立ち上げるためのドキュメントを `docs/setup.md` / `docs/usage.md` に追加する。Grafana 側はローカル運用と **完全に同じ構成**（既存 `docker-compose.yaml` + `make grafana-up`）を流用し、`AGENT_TELEMETRY_DB` env で DB パスを差し替えるだけでサーバ DB を可視化できるようにする。

## 根拠

既存 `docker-compose.yaml` は既に `AGENT_TELEMETRY_DB` env で DB パスを差し替え可能な作りで、`grafana-up` / `grafana-up-e2e` の違いは mount する DB ファイルだけ。サーバ運用でも同じ仕組みで `AGENT_TELEMETRY_DB=<server_data>/agent-telemetry.db make grafana-up` すれば、ダッシュボード JSON / datasource provisioning / Image Renderer 設定がそのまま動く。サーバ専用の Grafana スタックを別ファイルで持つ必然性はない。違うのは「サーバ binary `agent-telemetry-server` を同居させるかどうか」だけなので、その差分のみを overlay 用 compose ファイルで足す。

## 対応方針

- `docs/setup.md` に「サーバ送信を有効化する」節を追加
  - `agent-telemetry-server` の起動手順（systemd 経由 / docker-compose 経由）
  - `AGENT_TELEMETRY_SERVER_TOKEN` の生成と配布
  - `~/.claude/agent-telemetry.toml` の `[server]` セクション設定例
  - `agent-telemetry push --since-last` を cron / launchd plist / systemd timer で定期起動するサンプル
  - 新メトリクス追加時の遡及反映手順: サーバを先にデプロイ → 全クライアント binary 更新 → `agent-telemetry sync-db --recheck && push --full`（`schema_mismatch` エラーのトラブルシュート例も含める）
- `docs/usage.md` に「サーバ DB を Grafana で見る」を追加
  - **ローカル grafana-up と完全に同じ構成を使う**：`AGENT_TELEMETRY_DB=/var/lib/agent-telemetry/agent-telemetry.db make grafana-up` で OK
  - サーバ binary を同居起動する場合は `docker compose -f docker-compose.yaml -f docker-compose.server.yml up` で agent-telemetry-server も起動
  - `agent-telemetry push --dry-run` での運用確認方法
- `Makefile` に `server-up` thin wrapper を追加（任意）: `docker compose -f docker-compose.yaml -f docker-compose.server.yml up -d`
- `README.md` の機能一覧と図にサーバ送信経路を追加（送るのは集計値のみ、transcript はクライアント手元に残る、Grafana 構成はローカル/サーバ共通）

## 受け入れ条件

- [ ] `docs/setup.md` のサーバ手順通りに進めると、ローカル `agent-telemetry-server` 起動 + クライアント push まで通る
- [ ] `docs/usage.md` の手順で `AGENT_TELEMETRY_DB=<server_data>/agent-telemetry.db make grafana-up` を実行すると、サーバ DB を参照した Grafana ダッシュボードが描画される
- [ ] `docker compose -f docker-compose.yaml -f docker-compose.server.yml up` で server binary + Grafana が同居起動する
- [ ] **新規の grafana 専用ターゲットを増やさない**（`grafana-up-server` 等は作らず、既存 `grafana-up` の env オーバーライドで対応）
- [ ] `make grafana-screenshot` の E2E 経路は変更せず動き続ける（fixture 使用なのでサーバ/ローカル区別不要）
- [ ] `README.md` の図と機能一覧が更新されている

依存: [0028](0028-feat-server-push-client.md) と [0029](0029-feat-server-ingest.md)（実装が無いと文書化が成立しない）
