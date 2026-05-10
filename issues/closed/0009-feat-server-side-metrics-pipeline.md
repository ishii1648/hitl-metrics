---
decision_type: design
affected_paths:
  - docs/spec.md
  - docs/design.md
tags: [server, transport, multi-user]
closed_at: 2026-05-10
---

# 収集した metrics をサーバ側に転送・加工・表示できるようにする

Created: 2026-05-08

## 概要

現状、Claude Code / Codex CLI の hook で収集した metrics は各クライアントの `~/.claude/agent-telemetry.db`（SQLite）に閉じており、可視化も手元の Grafana コンテナ（`make grafana-up`）でしか行えない。複数マシン・複数ユーザーのデータをまとめて分析できるよう、共通サーバへ転送し、サーバ側で加工・表示できる経路を用意したい。

## 根拠

- ローカル SQLite + ローカル Grafana という構成は個人用途には足りるが、組織横断の分析・継続的なダッシュボードホスティングができない
- 別マシン（自宅 / 業務 / CI）で同一ユーザーが Claude を使った場合、現在は DB ファイルを物理的に集約しないと統合できない
- `docs/metrics.md` の観察軸（PR 単位のトークン効率など）はチーム全体での比較が本来価値を持つ。手元 DB に閉じている限り、その比較が実現できない
- `sync-db` が「セッション JSON → SQLite」までは集約してくれているので、その先の「SQLite → サーバ」の経路を増やす余地は仕様上残っている

## 問題

転送・加工・表示を成立させるには、少なくとも以下の論点を仕様として確定する必要がある。現時点ではいずれも未確定。

- **転送方式**: push 型（クライアントがサーバへ送る） / pull 型（サーバが各クライアントから取りに行く）のどちらを基本にするか
- **転送タイミング**: Stop hook 直後に逐次送るのか、`sync-db` 完了後にバッチで送るのか、ユーザー明示の `agent-telemetry push` で送るのか
- **プロトコル / データモデル**: 既存の OTLP（OpenTelemetry）/ Prometheus remote write / 独自 HTTP JSON のいずれに乗せるか。`docs/metrics.md` の OpenMetrics カタログとの対応関係をどう保つか
- **サーバ側の責務**: 単なる蓄積層（オブジェクトストレージ + クエリエンジン）にするのか、加工パイプライン（集計・PR 単位のロールアップ）まで含めるのか
- **既存ローカル Grafana との関係**: ローカル可視化を残しつつサーバ可視化を足すのか、サーバを正としてローカルは廃止するのか
- **認証・権限**: 誰がどのデータを書ける／読めるか。0004（複数人のデータ識別）と密結合
- **オフライン耐性**: ネットワーク断絶時のローカル蓄積とリトライ戦略

## 対応方針

設計判断が複数論点に渡るため、まず方向性を `docs/design.md` に書き起こし、合意後に実装タスクへ分解する。たたき台:

- push 型 + バッチ送信を第一候補にする。Stop hook の latency に転送 I/O を載せたくないこと、CI からの送信を考えるとクライアントから能動的に送る方が単純なため
- プロトコルは OTLP/HTTP を第一候補にする。`docs/metrics.md` の OpenMetrics カタログを将来 Prometheus 互換で配るとしても、収集経路は OTLP の方がメタデータ（resource attributes）を載せやすい
- サーバ側は最初は薄く保つ（ingestion + 蓄積のみ）。加工は Grafana / クエリエンジン側に寄せる
- 認証は 0010 の方針確定を待ってから決める（user 識別子の表現が決まらないと AuthN/AuthZ の設計が割れる）

検討の結果、たたき台を採らない判断もあり得る。`docs/spec.md` の hook / CLI 仕様は転送経路の有無で接点が増える可能性が高いので、決まった時点で同期する。

依存: 0010（複数人のデータ識別）— サーバへ集約する前提で必須。両方が揃わないと意味のある可視化にならない。

Completed: 2026-05-10

## 解決方法

設計セッションで方針を確定し、`docs/spec.md` / `docs/design.md` に書き起こした。実装は子 issue 0028 / 0029 / 0030 に分解した。

### 確定した方針

- **送信ペイロード**: `session-index.jsonl` 差分行 + transcript JSONL を **そのまま** raw 転送（集計済み値や OTLP は不採用）
- **プロトコル**: 独自 HTTP JSON `POST /v1/ingest`、Bearer 認証、HTTP gzip 必須、1 リクエスト 50 MB 上限
- **送信タイミング**: 独立コマンド `agent-telemetry push --since-last`。Stop hook hot path には載せない（ユーザは cron / launchd / systemd timer で起動）
- **差分検知**: `state.json` の `pushed_session_versions: {session_id: sha256}`。backfill による後追い更新（`is_merged` 等）も hash 不一致で検知される
- **進行中セッションは除外**: `ended_at` または `end_reason` が空のセッションは送信対象外
- **認証**: 単一 API key（`AGENT_TELEMETRY_SERVER_TOKEN`）。`user_id` は payload 内、認証境界とは責務分離
- **サーバ側**: `cmd/agent-telemetry-server/` で別 binary を提供。クライアントと同一 SQLite スキーマ + 共通 `internal/syncdb/` で集計し、Grafana ダッシュボード JSON を再利用する
- **配布形態**: Go binary + Docker image 両方（systemd unit と `docker-compose.server.yml` を同梱）

### プライバシー観点の整理

「transcript には secret が含まれる」懸念は精査の結果 **本質的に消えた**。transcript の中身は既に Anthropic API に context として送信済みであり、自前 telemetry サーバへ送ることに追加のプライバシー懸念はない。残るのは保存期間・閲覧範囲・組織ポリシーで、いずれもサーバ運用ポリシーで吸収可能。MVP では `send_transcripts` フラグもサニタイズフックも不要（YAGNI）。

### 採用しなかった代替

- OTLP / Prometheus remote write（後追い更新の表現が面倒、二重スタックの維持コスト）
- 集計済み `transcript_stats` のみ送る（SoR が JSONL という方針と整合せず、サーバ側で指標追加の道が塞がれる）
- Stop hook 同期 push（latency 侵食、failure mode の混入）
- fire-and-forget での子プロセス push（失敗が静かに死ぬためデバッグ困難）
- `send_transcripts = false` フラグ（プライバシー懸念が「Anthropic に既送信」で消えるため YAGNI）

### 主な変更点

- `docs/spec.md` に「サーバ送信」節、`agent-telemetry push` コマンド、`AGENT_TELEMETRY_SERVER_TOKEN` 環境変数を追加
- `docs/design.md` に「サーバ側集約パイプライン」節を追加、「既知の制約」にサーバ送信由来の制約を追加
- 子 issue 3 件を新規発番:
  - `0028-feat-server-push-client.md`（クライアント push 実装）
  - `0029-feat-server-ingest.md`（サーバ ingest 実装）
  - `0030-doc-server-grafana-setup.md`（運用ドキュメント + Grafana 連携）

### 残課題

- 0028 / 0029 / 0030 で実装する。0028 と 0029 は並列着手可能（両方が無いと E2E 検証ができないため、両方完了後に統合検証）
- 配布形態は VPS / Docker 両方提供。ユーザは環境次第で選択

依存: 0010（user 識別子）— 完了済み。
