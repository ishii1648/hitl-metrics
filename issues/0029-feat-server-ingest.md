---
decision_type: implementation
affected_paths:
  - cmd/agent-telemetry-server/
  - internal/serverpipe/
  - internal/syncdb/
  - Dockerfile.server
  - docker-compose.server.yml
  - contrib/systemd/
# 新規 binary / image / config のため close 時点で未存在
lint_ignore_missing:
  - cmd/agent-telemetry-server/
  - internal/serverpipe/
  - Dockerfile.server
  - docker-compose.server.yml
  - contrib/systemd/
tags: [server, ingest, http]
---

# サーバ側 ingest 実装 — agent-telemetry-server で集計値を受信し SQLite に upsert

Created: 2026-05-10

## 概要

`cmd/agent-telemetry-server/` を新設し、`POST /v1/metrics` で受信した `sessions` 行 + `transcript_stats` 行を SQLite に upsert する HTTP サーバを実装する。サーバは集計を行わず、受信値をそのまま `INSERT OR REPLACE` するだけの「dumb ingest layer」として動作する。仕様の外部契約は `docs/spec.md ## サーバ送信`、設計判断は `docs/design.md ## サーバ側集約パイプライン` を参照する。

## 根拠

[0009](closed/0009-feat-server-side-metrics-pipeline.md) で確定した方針として、サーバ側はクライアントと **DB スキーマだけ** を共通化し、集計（transcript パース等）はクライアント側で完結させる。これにより (1) 送信サイズが極小（月数 MB）、(2) サーバ実装が単純（`internal/syncdb/` 全体を持たず schema DDL のみ）、(3) transcript の保管不要（プライバシー観点とストレージ運用の議論がゼロ）という利点を取る。

## 対応方針

- `cmd/agent-telemetry-server/main.go` 新規
  - `--data-dir`（既定 `/var/lib/agent-telemetry`）、`--listen`（既定 `:8443`）
  - `AGENT_TELEMETRY_SERVER_TOKEN` 環境変数の必須チェック（未設定で起動時エラー終了）
  - 起動時に `<data_dir>/agent-telemetry.db` を作成、`internal/syncdb/schema.sql` を実行（`schema_meta` ハッシュ比較で DDL 再構築する仕組みはクライアントと同じ）
- `internal/serverpipe/` 新規パッケージ
  - `POST /v1/metrics` ハンドラ（Bearer 検証 → payload デコード → `schema_hash` 検証 → SQLite に `INSERT OR REPLACE`）
  - `schema_hash` 不一致なら `schema_mismatch: true` を返して受信拒否（DB は変更しない）
  - `(session_id, coding_agent)` PK での upsert 時に既存行があれば `<data_dir>/collisions.log` に記録（last-write-wins）
- `internal/syncdb/schema.sql` をサーバ binary に埋め込み、起動時 DDL 再構築をクライアントと共通化する。集計ロジック（`internal/transcript/Parse()` 等）はサーバ側に取り込まない
- `Dockerfile.server` 新規作成（`agent-telemetry-server` binary を含む image）
- `docker-compose.server.yml` を **overlay として新規作成**: 既存 `docker-compose.yaml`（Grafana + Image Renderer）を base に、`agent-telemetry-server` サービスのみ追加する。`docker compose -f docker-compose.yaml -f docker-compose.server.yml up` で server + Grafana を同居起動できる。Grafana / datasource / dashboard 設定は base から継承し、サーバ用に複製しない
- `contrib/systemd/agent-telemetry-server.service` を新規作成
- goreleaser 設定に `agent-telemetry-server` ビルドラインを追加

## 受け入れ条件

- [ ] `agent-telemetry-server` が `--listen :8443` で起動し、`AGENT_TELEMETRY_SERVER_TOKEN` 必須チェックが効く
- [ ] `POST /v1/metrics` に有効な payload を送ると、`<data_dir>/agent-telemetry.db` の `sessions` / `transcript_stats` テーブルに行が upsert される
- [ ] payload の `schema_hash` が DB の `schema_meta` と一致しない場合、`schema_mismatch: true` を返し DB を変更しない
- [ ] 受信後の DB を Grafana datasource uid `agent-telemetry` で参照すると、既存ダッシュボード JSON がそのまま動く
- [ ] 同一 `(session_id, coding_agent)` の再 push で `INSERT OR REPLACE` が効き、衝突は `collisions.log` に記録される
- [ ] `docker compose -f docker-compose.yaml -f docker-compose.server.yml up` で server + Grafana + Image Renderer が立ち上がる（Grafana 設定は base 側を継承し、overlay 側は server サービスのみ追加していること）
- [ ] 不正な Bearer token でのリクエストは 401 を返す
- [ ] `go test ./...` が通る

依存: [0028](0028-feat-server-push-client.md)（クライアント側送信、E2E 検証に必要）
