# TODO

hitl-metrics の開発タスクを管理する。完了したタスクは削除する。変更履歴は git log と GitHub Release を参照し、設計の経緯は `docs/history.md` に集約する。

## 実装タスク

- `todo-cleanup` hook を CHANGELOG 移送から TODO.md 削除のみに変更し、セクション名変更（`未着手` → `実装タスク`、`進行中` 廃止）に対応する
  - [ ] `internal/hook/todocleanup.go` が `TODO.md` 内の完了済みタスクを削除するだけになり、`CHANGELOG.md` を読み書きしない
  - [ ] `CHANGELOG.md` が存在しない環境（廃止後）でも hook が成功する
  - [ ] 対象セクションのヘッダ判定が `## 実装タスク` を見るように変更されている（旧 `## 未着手` は探さない）
  - [ ] `## 進行中` セクション参照に依存していたテストケースは削除または書き換えられている
  - [ ] 完了タスク削除のテストが残り、CHANGELOG 出力に依存するテストは削除または書き換えられている
  - [ ] 出力メッセージから `CHANGELOG.md` への言及が消えている
- `pr_urls` の追加順を保持して sync-db の採用 PR を安定化
  - [ ] `sessionindex.Update` / `UpdateByBranch` が URL を辞書順ソートせず既存順 + 追加順を保持する
  - [ ] `sync-db` の「最後の PR URL を採用」仕様と実装が一致する
  - [ ] 複数 PR URL を持つ session-index のテストを追加する
- `session-index.jsonl` の書き戻しを atomic 化
  - [ ] `WriteAll` が一時ファイルへ書き込んでから rename する
  - [ ] 書き込み失敗時に既存の `session-index.jsonl` が truncate されない
  - [ ] `backfill` / `update` 系の既存テストが通る
- Stop hook の `hitl-metrics` PATH 依存をなくす
  - [ ] `hitl-metrics hook stop` が hook 実行環境の PATH 差異で失敗しない
  - [ ] `backfill` と `sync-db` を同一プロセスで直接呼ぶ、または setup 時に絶対パスを登録する方針を決める
  - [ ] 失敗時のログがユーザーに原因を追える内容になっている
- ローカル検証環境と CI の再現性を改善
  - [ ] `go test ./...` の SQLite 関連テストが macOS arm64 ローカル環境で安定して実行できる
  - [ ] `go test -race ./...` がローカルで実行不能な場合の代替手順を docs に明記する
  - [ ] Go バージョン・toolchain・modernc.org/sqlite の制約を整理する

## 検討中

- Codex CLI 対応 — agent アダプタ層の導入
  - [ ] `internal/agent/` に `Agent` インタフェースと `claude` / `codex` 実装を追加
  - [ ] `--agent <claude|codex>` フラグを `hook` / `setup` / `backfill` / `sync-db` / `doctor` に追加。既定値は `claude`
  - [ ] `HITL_METRICS_AGENT` 環境変数を読む
  - [ ] agent 自動検出（`~/.claude/session-index.jsonl` / `~/.codex/session-index.jsonl` の存在）が CLI から動く
  - [ ] 既存の Claude 専用テストが回帰しない
- Codex CLI 対応 — SQLite スキーマと sync-db
  - [ ] `sessions.coding_agent` カラム追加・PRIMARY KEY を `(session_id, coding_agent)` に変更
  - [ ] `sessions.agent_version` カラム追加（取得不能なら空文字列）
  - [ ] `transcript_stats.coding_agent` カラム追加・PRIMARY KEY 変更
  - [ ] `transcript_stats.reasoning_tokens` カラム追加（Claude は 0、Codex は実値）
  - [ ] `pr_metrics` VIEW を `(pr_url, coding_agent)` で集約・`reasoning_tokens` を `total_tokens` に含める（`agent_version` は集約しない）
  - [ ] `session_concurrency_*` VIEW を `coding_agent` ごとに分割
  - [ ] Claude のみの環境で sync-db が回帰しない（Codex データ無しでも動く）
- Codex CLI 対応 — hook サブコマンド実装
  - [ ] `hitl-metrics hook session-start --agent codex` が `~/.codex/session-index.jsonl` に追記する
  - [ ] `hitl-metrics hook stop --agent codex` が `ended_at` を上書きしつつ `backfill` → `sync-db` を実行する
  - [ ] `hitl-metrics hook post-tool-use --agent codex` が `tool_response` から PR URL を抽出して `pr_urls` に追記する
  - [ ] hook 入力 JSON スキーマの違い（Codex の `model` / `turn_id` / `source` フィールド）を吸収する
  - [ ] SessionStart hook で `agent_version` を取得して `session-index.jsonl` に記録する（Claude / Codex とも、hook input → 環境変数の順、外部コマンドは呼ばない）
  - [ ] Codex backfill が rollout JSONL の最初のメタイベントから `agent_version` を補完する（hook input で取れなかった場合）
- Codex CLI 対応 — transcript パーサ
  - [ ] `internal/transcript/codex.go` が rollout JSONL から `tool_use_total` / `mid_session_msgs` / token 系を集計する
  - [ ] `event_msg.payload.type == "token_count"` の最終累積値が input/output/cache_read/cache_write/reasoning に反映される
  - [ ] `.jsonl.zst` を `klauspost/compress/zstd` で透過デコードできる
  - [ ] backfill が rollout JSONL の最終 event タイムスタンプで `ended_at` を補正できる
- Codex CLI 対応 — setup / uninstall-hooks / doctor / docs
  - [ ] `hitl-metrics install` を `hitl-metrics setup` にリネーム。`setup [--agent <claude|codex>]` が agent 別の登録例を表示する
  - [ ] `internal/install/` パッケージを `internal/setup/` にリネームする
  - [ ] `hitl-metrics install --uninstall-hooks` を `hitl-metrics uninstall-hooks` 独立サブコマンドに分離する
  - [ ] 旧 `hitl-metrics install` を deprecation warning 付き alias として残す（`setup` を呼び出して同等の案内を表示）
  - [ ] `hitl-metrics doctor` が両 agent を自動検出して hook 登録状況を warning で表示する
  - [ ] `docs/setup.md` に Codex セットアップ手順と `setup` コマンドの新名称を反映する
  - [ ] `docs/usage.md` に agent 切替・自動検出の挙動を追記する
  - [ ] README とリポジトリ内の既存 `install` 言及を `setup` に更新する（`docs/archive/adr/` は除外）
  - [ ] `Makefile` の `install` ターゲット（バイナリのインストール）と CLI の `setup` が混同されない説明を追加する
- Codex CLI 対応 — Grafana ダッシュボード
  - [ ] `coding_agent` テンプレート変数を追加（All / claude / codex）
  - [ ] PR 別スコアカードに `coding_agent` 列を追加
  - [ ] 週別 token 消費・PR / 1M tokens・concurrent sessions を agent 別シリーズで表示
  - [ ] Agent 別比較 stat パネルを追加（avg tokens / PR と PR / 1M tokens）
  - [ ] `make grafana-screenshot` を実行して `docs/images/dashboard-*.png` を更新する

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示
