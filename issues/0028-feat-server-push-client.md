---
decision_type: implementation
affected_paths:
  - internal/serverclient/
  - cmd/agent-telemetry/
  - internal/sessionindex/
# 新規パッケージのため close 時点で未存在
lint_ignore_missing:
  - internal/serverclient/
tags: [server, push, client]
---

# クライアント側 push 実装 — agent-telemetry push でサーバへ raw JSONL を送る

Created: 2026-05-10

## 概要

`agent-telemetry push --since-last` を新設し、`session-index.jsonl` の差分行と完了済みセッションの transcript JSONL を独自 HTTP JSON でサーバへ送信する経路を実装する。仕様の外部契約は `docs/spec.md ## サーバ送信`、設計判断は `docs/design.md ## サーバ側集約パイプライン` を参照する。

## 根拠

[0009](closed/0009-feat-server-side-metrics-pipeline.md) でサーバ送信の方針が確定した。クライアント側で必要な要素は (1) `[server]` 設定読み込み、(2) `pushed_session_versions` による差分検知、(3) HTTP 送信、(4) `state.json` の更新の 4 つに整理できる。サーバ側の ingest 実装（[0029](0029-feat-server-ingest.md)）と並列で進められるよう、独立 issue として分離する。

## 対応方針

- `internal/serverclient/` 新規パッケージ
  - `agent-telemetry.toml` の `[server]` セクション読み込み（既存 TOML パーサを拡張）
  - `pushed_session_versions` の読み書き（`agent-telemetry-state.json` 拡張、欠落時は空マップ）
  - 差分検知（session-index.jsonl の対応行 + transcript の SHA-256 で hash を計算し、`pushed_session_versions` と比較）
  - HTTP POST `/v1/ingest`（gzip 圧縮、Bearer 認証、50 MB を超えるバッチは自動分割）
  - 進行中セッション（`ended_at` または `end_reason` が空）は送信対象外
- `cmd/agent-telemetry/main.go` に `push` サブコマンドを追加
  - フラグ: `--since-last`（既定）/ `--full` / `--dry-run` / `--agent <claude|codex>`
- `internal/userid/` の Resolver を流用し、`user_id` は payload に含める
- Codex の `.jsonl.zst` は再展開せず `encoding: "zstd"` で送る
- `[server]` 設定欠落時は stderr に warning を出して exit code 0 で終了する（cron で叩いて壊れないこと）

## 受け入れ条件

- [ ] `agent-telemetry push --dry-run` が対象セッション件数と圧縮前後のサイズを表示する
- [ ] `agent-telemetry push --since-last` でローカルテストサーバ（[0029](0029-feat-server-ingest.md)）に到達し、レスポンスが `{received_sessions, skipped_sessions, collisions}` の形で返る
- [ ] 2 回連続で `push` を実行すると、2 回目は差分検知で送信ゼロになる
- [ ] backfill が `is_merged` を更新した後の `push` で、該当セッションだけ再送される（hash 不一致を検出する）
- [ ] 進行中セッション（`ended_at` 空）は送信対象から除外される
- [ ] `[server]` 設定欠落時は warning を出して exit code 0 で終了する
- [ ] 50 MB を超えるバッチが複数リクエストに自動分割される
- [ ] `go test ./...` が通る

依存: [0029](0029-feat-server-ingest.md)（サーバ側 ingest 実装、E2E 検証に必要）
