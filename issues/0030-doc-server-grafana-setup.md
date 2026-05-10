---
decision_type: process
affected_paths:
  - docs/setup.md
  - docs/usage.md
  - README.md
tags: [server, docs, grafana, k8s, operations]
---

# サーバ運用ドキュメント — k8s manifest と Grafana 設定資産の共通化

Created: 2026-05-10

## 概要

サーバ送信機能を利用者が立ち上げるためのドキュメントを `docs/setup.md` / `docs/usage.md` に追加する。本番想定は **k8s pod**（[0029](0029-feat-server-ingest.md) で `deploy/k8s/` に Kustomize manifest を提供）。Grafana の **設定資産**（dashboard JSON、datasource provisioning yaml）はローカル `docker-compose.yaml` と k8s ConfigMap の **両方から同じファイルを参照** する形で共通化し、二重メンテナンスを避ける。配布手段（compose vs k8s）自体は揃えず、それぞれの環境ネイティブな形を取る。

## 根拠

サーバの本番形態は k8s pod のため、`docker-compose.server.yml` 等で配布するメリットがない。一方で Grafana のダッシュボード JSON / datasource provisioning は形式が単純（YAML / JSON）なので、ローカル `docker-compose.yaml` の volume mount と k8s ConfigMap mount の両方から同じファイルを参照できる。これにより「ダッシュボード変更が両環境に同時反映される」共通化を成立させつつ、配布手段は環境ネイティブなものを使う。

## 対応方針

- `docs/setup.md` に「サーバ送信を有効化する」節を追加
  - **k8s デプロイ手順**: `kubectl apply -k deploy/k8s/overlays/production/` の例
  - **ローカル動作確認**: `kind create cluster` + `kubectl apply -k deploy/k8s/overlays/local/`
  - **Go binary 起動**: VPS / bare metal で systemd unit を使うパターン
  - `AGENT_TELEMETRY_SERVER_TOKEN` の生成と Secret 経由の供給
  - `~/.claude/agent-telemetry.toml` の `[server]` セクション設定例
  - `agent-telemetry push --since-last` を cron / launchd plist / systemd timer で定期起動するサンプル
  - 新メトリクス追加時の遡及反映手順: サーバ image 更新 → Deployment rolling update → 全クライアント binary 更新 → `agent-telemetry sync-db --recheck && push --full`（`schema_mismatch` エラーのトラブルシュート例も含める）
- `docs/usage.md` に「サーバ DB を Grafana で見る」を追加
  - k8s 経由: Grafana Service にアクセスする手順（NodePort / Port-forward / Ingress）
  - ローカル開発時: `AGENT_TELEMETRY_DB=<server_data>/agent-telemetry.db make grafana-up` で同じダッシュボードがそのまま動くことを明記（個人利用 + ローカル DB 確認の用途）
  - `agent-telemetry push --dry-run` での運用確認方法
- `README.md` の機能一覧と図にサーバ送信経路を追加（送るのは集計値のみ、transcript はクライアント手元に残る、Grafana 設定資産はローカル/k8s で共通参照）

## 受け入れ条件

- [ ] `docs/setup.md` の k8s 手順通りに `kubectl apply -k deploy/k8s/overlays/local/` を実行すると、kind 上で server + Grafana + PVC + Secret が立ち上がり、クライアントから push が通る
- [ ] `docs/setup.md` の Go binary 手順通りに systemd unit 起動 + クライアント push まで通る
- [ ] Grafana の ConfigMap mount 経由で `grafana/dashboards/agent-telemetry.json` がそのまま描画されることを確認できる（ローカル `make grafana-up` と同一の見た目）
- [ ] `docs/usage.md` のローカル開発時手順で、サーバ DB ファイルを `AGENT_TELEMETRY_DB` で指して `make grafana-up` するとダッシュボードがそのまま動く
- [ ] `make grafana-screenshot` の E2E 経路は変更せず動き続ける（fixture 使用なのでサーバ/ローカル区別不要）
- [ ] `README.md` の図と機能一覧が更新されている

依存: [0028](closed/0028-feat-server-push-client.md) と [0029](0029-feat-server-ingest.md)（実装と manifest が無いと文書化が成立しない）、[0031](0031-feat-server-image-ghcr-publish.md)（image が ghcr に上がっていないと k8s manifest が pull できない）
