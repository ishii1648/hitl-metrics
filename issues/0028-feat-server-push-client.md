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

# クライアント側 push 実装 — agent-telemetry push でサーバへ集計値を送る

Created: 2026-05-10

## 概要

`agent-telemetry push --since-last` を新設し、`sync-db` 完了後の **集計値**（`sessions` 行 + `transcript_stats` 行）を独自 HTTP JSON でサーバへ送信する経路を実装する。`session-index.jsonl` の生行や transcript JSONL（会話本体）は送らない。仕様の外部契約は `docs/spec.md ## サーバ送信`、設計判断は `docs/design.md ## サーバ側集約パイプライン` を参照する。

## 根拠

[0009](closed/0009-feat-server-side-metrics-pipeline.md) でサーバ送信の方針が確定した。クライアント側で必要な要素は (1) `[server]` 設定読み込み、(2) ローカル DB から差分行を抽出、(3) `pushed_session_versions` による差分検知、(4) HTTP 送信、(5) `state.json` の更新 の 5 つに整理できる。サーバ側の ingest 実装（[0029](0029-feat-server-ingest.md)）と並列で進められるよう、独立 issue として分離する。

## 対応方針

- `internal/serverclient/` 新規パッケージ
  - `agent-telemetry.toml` の `[server]` セクション読み込み（既存 TOML パーサを拡張）
  - `pushed_session_versions` の読み書き（`agent-telemetry-state.json` 拡張、欠落時は空マップ）。キーは `<coding_agent>:<session_id>` 形式の文字列（複合 PK `(session_id, coding_agent)` を反映、Claude / Codex 間の UUID 衝突で hash が上書きされないようにする）
  - クライアント DB から `sessions` / `transcript_stats` の差分行を抽出（進行中セッションは除外: `ended_at` または `end_reason` が空のものは対象外）
  - 差分検知: 各セッションの `sessions` 行 + 対応する `transcript_stats` 行を JSON canonicalize → SHA-256 → `pushed_session_versions[<coding_agent>:<session_id>]` と比較
  - HTTP POST `/v1/metrics`（gzip optional、Bearer 認証、`schema_hash` 添付、50 MB を超えるバッチは自動分割）
  - レスポンスの `schema_mismatch: true` を検出した場合はユーザに binary 更新を促す
- `cmd/agent-telemetry/main.go` に `push` サブコマンドを追加
  - フラグ: `--since-last`（既定）/ `--full` / `--dry-run` / `--agent <claude|codex>`
- `user_id` は payload の `sessions` 行に既に含まれているため、別途 Resolver を呼ぶ必要はない
- `[server]` 設定欠落時は stderr に warning を出して exit code 0 で終了する（cron で叩いて壊れないこと）

## 受け入れ条件

- [ ] `agent-telemetry push --dry-run` が対象セッション件数と payload サイズを表示する
- [ ] `agent-telemetry push --since-last` でローカルテストサーバ（[0029](0029-feat-server-ingest.md)）に到達し、レスポンスが `{received_sessions, skipped_sessions, schema_mismatch}` の形で返る
- [ ] 2 回連続で `push` を実行すると、2 回目は差分検知で送信ゼロになる
- [ ] backfill が `is_merged` を更新した後の `push` で、該当セッションだけ再送される（`sessions` 行 hash の不一致を検出）
- [ ] 進行中セッション（`ended_at` 空）は送信対象から除外される
- [ ] `[server]` 設定欠落時は warning を出して exit code 0 で終了する
- [ ] サーバが古いスキーマで `schema_mismatch: true` を返した場合、クライアントは exit code 非ゼロで終了し原因をログに記録する
- [ ] `go test ./...` が通る

依存: [0029](0029-feat-server-ingest.md)（サーバ側 ingest 実装、E2E 検証に必要）
