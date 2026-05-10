---
decision_type: process
affected_paths:
  - docs/setup.md
  - docs/usage.md
  - Makefile
  - README.md
tags: [server, docs, grafana, operations]
---

# サーバ運用ドキュメント・Grafana 連携・E2E 検証

Created: 2026-05-10

## 概要

サーバ送信機能を利用者が立ち上げるためのドキュメントを `docs/setup.md` / `docs/usage.md` に追加し、`Makefile` にサーバ Grafana 起動ターゲットを増設する。`make grafana-screenshot` の E2E がサーバ構成でも検証できるように拡張する。

## 根拠

[0028](0028-feat-server-push-client.md) と [0029](0029-feat-server-ingest.md) でクライアント・サーバ実装が完了するが、立ち上げ手順と Grafana datasource 設定が文書化されていないと利用できない。また、ローカル Grafana の E2E（`make grafana-screenshot`）と同等の検証経路をサーバ構成にも用意して、ダッシュボード JSON が両構成で動くことを継続検証する仕組みを整える。

## 対応方針

- `docs/setup.md` に「サーバ送信を有効化する」節を追加
  - `agent-telemetry-server` の起動手順（systemd 経由 / docker-compose 経由 両方）
  - `AGENT_TELEMETRY_SERVER_TOKEN` の生成と配布
  - `~/.claude/agent-telemetry.toml` の `[server]` セクション設定例
  - `agent-telemetry push --since-last` を cron / launchd plist / systemd timer で定期起動するサンプル
- `docs/usage.md` に「サーバ Grafana の見方」を追加
  - datasource 設定例（`uid: agent-telemetry` を踏襲）
  - 既存ダッシュボード JSON が server SQLite でも動くことを明記
  - `agent-telemetry push --dry-run` での運用確認方法
- `Makefile` に `grafana-up-server` ターゲットを追加（`docker-compose.server.yml` 経由で server + Grafana + Image Renderer を立ち上げ）
- `make grafana-screenshot` をサーバモードでも実行可能にする（環境変数で切り替え、もしくは `grafana-screenshot-server` 別ターゲット）
- `README.md` の機能一覧と図にサーバ送信経路を追加

## 受け入れ条件

- [ ] `docs/setup.md` のサーバ手順通りに進めると、ローカル `agent-telemetry-server` 起動 + クライアント push まで通る
- [ ] `docs/usage.md` のサーバ Grafana 見方の手順で、ダッシュボードがサーバ SQLite を参照して描画される
- [ ] `make grafana-up-server` でサーバ + Grafana + Image Renderer が立ち上がる
- [ ] `make grafana-screenshot`（または相当ターゲット）でサーバ構成のスクリーンショットが取得できる
- [ ] `README.md` の図と機能一覧が更新されている

依存: [0028](0028-feat-server-push-client.md) と [0029](0029-feat-server-ingest.md)（実装が無いと文書化と E2E が成立しない）
