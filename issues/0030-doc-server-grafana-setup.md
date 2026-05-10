---
decision_type: process
affected_paths:
  - docs/setup.md
  - docs/usage.md
  - README.md
tags: [server, docs, grafana, k8s, operations]
---

# サーバ運用ドキュメント — image 配布 + 参考デプロイ手順 + Grafana 設定資産の共通化

Created: 2026-05-10

## 概要

サーバ送信機能を利用者が立ち上げるためのドキュメントを `docs/setup.md` / `docs/usage.md` に追加する。agent-telemetry が公式に提供するのは **image** (`ghcr.io/ishii1648/agent-telemetry-server`) と **Go binary** (goreleaser) のみ。本番デプロイ手段（Helm / Argo CD / Flux / 素の kubectl）と cluster topology（StorageClass / IngressClass / cert-manager の有無）は運用者の責務とし、docs では **参考用 YAML スニペット** を annotated markdown 形式で示す（ファイル化はしない）。Grafana の **設定資産**（dashboard JSON、datasource provisioning yaml）はローカル `docker-compose.yaml` と k8s ConfigMap の **両方から同じファイルを参照可能** な形を保ち、二重メンテナンスを避ける。

## 根拠

当初は `deploy/k8s/` に Kustomize manifest（base + overlays/local + overlays/production）を canonical として提供する案だったが、0029 完了後の議論で取り下げた。理由:

- デプロイ方式と cluster topology は運用者ごとに異なり、ひとつの manifest を canonical として置くと fork or copy が必要になる
- `letsencrypt-prod` cert-manager / `nginx` ingressClass / `standard` StorageClass などの defaults は陳腐化コストが本リポのスコープに見合わない
- `REPLACE_ME` token / サンプル Secret といった「うっかり deploy されると事故るもの」をリポに置くこと自体がアンチパターン
- agent-telemetry のスコープは hook + CLI + dashboard JSON + ingest server。インフラ配布物は別レイヤーの責務

代わりに「image を提供 + docs で参考 YAML を示す」という最小スコープにする。

## 対応方針

- `docs/setup.md` に「サーバ送信を有効化する」節を追加
  - **image の入手**: `docker pull ghcr.io/ishii1648/agent-telemetry-server:latest` の案内（tag pin の例も）
  - **k8s 参考デプロイ**: docs 内の annotated YAML スニペットとして以下を示す（ファイル化はしない、運用者が cluster に合わせて改変する前提）
    - 最小構成: `Deployment` (replicas=1, RollingUpdate→Recreate) + `Service` (ClusterIP) + `PersistentVolumeClaim` (operator が StorageClass を埋める前提) + `Secret` (token)
    - Grafana 同居版: 上記 + Grafana `Deployment` + ConfigMap mount で `grafana/provisioning/datasources/agent-telemetry-docker.yaml` と `grafana/dashboards/agent-telemetry.json` を参照
    - Ingress / cert-manager / StorageClass などのクラスタ前提に依存する箇所は `# REPLACE_ME` コメントで明示
  - `AGENT_TELEMETRY_SERVER_TOKEN` の生成と Secret 経由の供給
  - `~/.claude/agent-telemetry.toml` の `[server]` セクション設定例
  - `agent-telemetry push --since-last` を cron / launchd plist / systemd timer で定期起動するサンプル
  - 新メトリクス追加時の遡及反映手順: サーバ image 更新 → Deployment rolling update → 全クライアント binary 更新 → `agent-telemetry sync-db --recheck && push --full`（`schema_mismatch` エラーのトラブルシュート例も含める）
- `docs/usage.md` に「サーバ DB を Grafana で見る」を追加
  - k8s 経由: Grafana Service にアクセスする手順（Port-forward を最小例として、NodePort / Ingress は応用例）
  - ローカル開発時: `AGENT_TELEMETRY_DB=<server_data>/agent-telemetry.db make grafana-up` で同じダッシュボードがそのまま動くことを明記（個人利用 + ローカル DB 確認の用途）
  - `agent-telemetry push --dry-run` での運用確認方法
- `README.md` の機能一覧と図にサーバ送信経路を追加（送るのは集計値のみ、transcript はクライアント手元に残る、Grafana 設定資産はローカル/k8s で共通参照）

## 受け入れ条件

- [ ] `docs/setup.md` の k8s 参考スニペット通りに自分で `kubectl apply -f -` できる粒度で書かれている（最小構成 + Grafana 同居版の 2 パターン）
- [ ] スニペット内で operator が埋める箇所（StorageClass / IngressClass / token / cert-manager 設定）が `# REPLACE_ME` で明示されている
- [ ] Grafana を docs 通りに mount すれば `grafana/dashboards/agent-telemetry.json` がそのまま描画される（ローカル `make grafana-up` と同一の見た目）
- [ ] `docs/usage.md` のローカル開発時手順で、サーバ DB ファイルを `AGENT_TELEMETRY_DB` で指して `make grafana-up` するとダッシュボードがそのまま動く
- [ ] `make grafana-screenshot` の E2E 経路は変更せず動き続ける（fixture 使用なのでサーバ/ローカル区別不要）
- [ ] `README.md` の図と機能一覧が更新されている

依存: [0028](closed/0028-feat-server-push-client.md) と [0029](closed/0029-feat-server-ingest.md)（実装が無いと文書化が成立しない）、[0031](closed/0031-feat-server-image-ghcr-publish.md)（image が ghcr に上がっていないとデプロイ手順が成立しない）
