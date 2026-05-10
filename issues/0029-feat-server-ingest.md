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

# サーバ側 ingest 実装 — agent-telemetry-server で raw JSONL を受信し SQLite に集約

Created: 2026-05-10

## 概要

`cmd/agent-telemetry-server/` を新設し、`POST /v1/ingest` で受信した `session-index.jsonl` 差分行と transcript JSONL を user_id ごとに分配して保管したうえで、内部で `internal/syncdb/` を呼び出して SQLite を集約する HTTP サーバを実装する。仕様の外部契約は `docs/spec.md ## サーバ送信`、設計判断は `docs/design.md ## サーバ側集約パイプライン` を参照する。

## 根拠

[0009](closed/0009-feat-server-side-metrics-pipeline.md) で確定した方針として、サーバ側はクライアントと同一の SQLite スキーマと `internal/syncdb/` ロジックを使うことで、ローカル Grafana のダッシュボード JSON を再利用できる構造にする。受信 + 保管 + sync のパスを 1 binary に閉じ込め、ingest API の責務を最小化する。

## 対応方針

- `cmd/agent-telemetry-server/main.go` 新規
  - `--data-dir`（既定 `/var/lib/agent-telemetry`）、`--listen`（既定 `:8443`）
  - `AGENT_TELEMETRY_SERVER_TOKEN` 環境変数の必須チェック（未設定で起動時エラー終了）
- `internal/serverpipe/` 新規パッケージ
  - `POST /v1/ingest` ハンドラ（Bearer 検証 → payload デコード → user_id 分配 → ファイル書き込み → syncdb 呼び出し）
  - `<data_dir>/<user_id>/session-index.jsonl` への append
  - `<data_dir>/<user_id>/transcripts/<session_id>.jsonl.zst` への zstd 保管（受信時の `encoding` を見て、raw なら圧縮、zstd ならそのまま保存）
  - `(session_id, coding_agent)` PK の重複検出時に `<data_dir>/collisions.log` に記録（`INSERT OR REPLACE` は最後勝ち）
- `internal/syncdb/` のクライアント／サーバ両用化
  - 既存 API がクライアント前提（`~/.claude/` を直接見る）になっていればパス指定の引数化を行う
- `Dockerfile.server` と `docker-compose.server.yml` を新規作成（既存 `docker-compose.yml` とは別ライン、server + Grafana + Image Renderer 同梱）
- `contrib/systemd/agent-telemetry-server.service` を新規作成
- goreleaser 設定に `agent-telemetry-server` ビルドラインを追加

## 受け入れ条件

- [ ] `agent-telemetry-server` が `--listen :8443` で起動し、`AGENT_TELEMETRY_SERVER_TOKEN` 必須チェックが効く
- [ ] `POST /v1/ingest` に有効な payload を送ると、`<data_dir>/<user_id>/session-index.jsonl` に行が追記される
- [ ] transcript が `<data_dir>/<user_id>/transcripts/<session_id>.jsonl.zst` に zstd で保管される（raw / zstd のどちらで送られても）
- [ ] 受信後に `<data_dir>/agent-telemetry.db` が更新され、Grafana から既存ダッシュボード JSON を datasource uid `agent-telemetry` で参照できる
- [ ] 同一 `(session_id, coding_agent)` の再 push で `INSERT OR REPLACE` が効き、`collisions.log` に記録される
- [ ] `docker-compose.server.yml` で server + Grafana + Image Renderer が立ち上がる
- [ ] 不正な Bearer token でのリクエストは 401 を返す
- [ ] `go test ./...` が通る

依存: [0028](0028-feat-server-push-client.md)（クライアント側送信、E2E 検証に必要）
