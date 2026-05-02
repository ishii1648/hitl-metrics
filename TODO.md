# TODO

hitl-metrics の開発タスクを管理する。完了したタスクは削除する。変更履歴は git log と GitHub Release を参照し、設計の経緯は `docs/history.md` に集約する。

## 実装タスク

- `todo-cleanup` hook を CHANGELOG 移送から TODO.md 削除のみに変更し、セクション名変更（`未着手` → `実装タスク`、`進行中` 廃止）に対応する
  - [x] `internal/hook/todocleanup.go` が `TODO.md` 内の完了済みタスクを削除するだけになり、`CHANGELOG.md` を読み書きしない
  - [x] `CHANGELOG.md` が存在しない環境（廃止後）でも hook が成功する
  - [x] 対象セクションのヘッダ判定が `## 実装タスク` を見るように変更されている（旧 `## 未着手` は探さない）
  - [x] `## 進行中` セクション参照に依存していたテストケースは削除または書き換えられている
  - [x] 完了タスク削除のテストが残り、CHANGELOG 出力に依存するテストは削除または書き換えられている
  - [x] 出力メッセージから `CHANGELOG.md` への言及が消えている
- `pr_urls` の追加順を保持して sync-db の採用 PR を安定化
  - [x] `sessionindex.Update` / `UpdateByBranch` が URL を辞書順ソートせず既存順 + 追加順を保持する
  - [x] `sync-db` の「最後の PR URL を採用」仕様と実装が一致する
  - [x] 複数 PR URL を持つ session-index のテストを追加する
- `session-index.jsonl` の書き戻しを atomic 化
  - [x] `WriteAll` が一時ファイルへ書き込んでから rename する
  - [x] 書き込み失敗時に既存の `session-index.jsonl` が truncate されない
  - [x] `backfill` / `update` 系の既存テストが通る
- Codex CLI 対応 — agent アダプタ層の導入
  - [x] `internal/agent/` に `Agent` インタフェースと `claude` / `codex` 実装を追加
  - [x] `--agent <claude|codex>` フラグを `hook` / `setup` / `backfill` / `sync-db` / `doctor` に追加。既定値は `claude`
  - [x] `HITL_METRICS_AGENT` 環境変数を読む
  - [x] agent 自動検出（`~/.claude/session-index.jsonl` / `~/.codex/session-index.jsonl` の存在）が CLI から動く
  - [x] 既存の Claude 専用テストが回帰しない
- Codex CLI 対応 — SQLite スキーマと sync-db
  - [x] `sessions.coding_agent` カラム追加・PRIMARY KEY を `(session_id, coding_agent)` に変更
  - [x] `sessions.agent_version` カラム追加（取得不能なら空文字列）
  - [x] `transcript_stats.coding_agent` カラム追加・PRIMARY KEY 変更
  - [x] `transcript_stats.reasoning_tokens` カラム追加（Claude は 0、Codex は実値）
  - [x] `pr_metrics` VIEW を `(pr_url, coding_agent)` で集約・`reasoning_tokens` を `total_tokens` に含める（`agent_version` は集約しない）
  - [x] `session_concurrency_*` VIEW を `coding_agent` ごとに分割
  - [x] Claude のみの環境で sync-db が回帰しない（Codex データ無しでも動く）
- Codex CLI 対応 — hook サブコマンド実装
  - [x] `hitl-metrics hook session-start --agent codex` が `~/.codex/session-index.jsonl` に追記する
  - [x] `hitl-metrics hook stop --agent codex` が `ended_at` を上書きしつつ `backfill` → `sync-db` を実行する
  - [x] `hitl-metrics hook post-tool-use --agent codex` が `tool_response` から PR URL を抽出して `pr_urls` に追記する
  - [x] hook 入力 JSON スキーマの違い（Codex の `model` / `turn_id` / `source` フィールド）を吸収する
  - [x] SessionStart hook で `agent_version` を取得して `session-index.jsonl` に記録する（Claude / Codex とも、hook input → 環境変数の順、外部コマンドは呼ばない）
  - [x] Codex backfill が rollout JSONL の最初のメタイベントから `agent_version` を補完する（hook input で取れなかった場合）
- Codex CLI 対応 — transcript パーサ
  - [x] `internal/transcript/codex.go` が rollout JSONL から `tool_use_total` / `mid_session_msgs` / token 系を集計する
  - [x] `event_msg.payload.type == "token_count"` の最終累積値が input/output/cache_read/cache_write/reasoning に反映される
  - [x] `.jsonl.zst` を `klauspost/compress/zstd` で透過デコードできる
  - [x] backfill が rollout JSONL の最終 event タイムスタンプで `ended_at` を補正できる
- Codex CLI 対応 — setup / uninstall-hooks / doctor / docs
  - [x] `hitl-metrics install` を `hitl-metrics setup` にリネーム。`setup [--agent <claude|codex>]` が agent 別の登録例を表示する
  - [x] `internal/install/` パッケージを `internal/setup/` にリネームする
  - [x] `hitl-metrics install --uninstall-hooks` を `hitl-metrics uninstall-hooks` 独立サブコマンドに分離する
  - [x] 旧 `hitl-metrics install` を deprecation warning 付き alias として残す（`setup` を呼び出して同等の案内を表示）
  - [x] `hitl-metrics doctor` が両 agent を自動検出して hook 登録状況を warning で表示する
  - [x] `docs/setup.md` に Codex セットアップ手順と `setup` コマンドの新名称を反映する
  - [x] `docs/usage.md` に agent 切替・自動検出の挙動を追記する
  - [x] README とリポジトリ内の既存 `install` 言及を `setup` に更新する（`docs/archive/adr/` は除外）
  - [x] `Makefile` の `install` ターゲット（バイナリのインストール）と CLI の `setup` が混同されない説明を追加する
- Codex CLI 対応 — Grafana ダッシュボード
  - [x] `coding_agent` テンプレート変数を追加（All / claude / codex）
  - [x] PR 別スコアカードに `coding_agent` 列を追加
  - [x] 週別 token 消費・PR / 1M tokens・concurrent sessions を agent 別シリーズで表示
  - [ ] Agent 別比較 stat パネルを追加（avg tokens / PR と PR / 1M tokens）
  - [ ] `make grafana-screenshot` を実行して `docs/images/dashboard-*.png` を更新する

## 検討中

- Stop hook の `hitl-metrics` PATH 依存をなくす — 解決方針を決める
  - 候補 A: `backfill` / `sync-db` を `internal/` 関数として直接呼ぶ（同一プロセス、PATH 非依存）
  - 候補 B: `setup` 時に hook コマンドの絶対パスを案内する（`settings.json` / `config.toml` 側で絶対パスを書く）
  - 候補 C: hook 内で binary を `os.Executable()` で解決し PATH にフォールバックしない
  - 失敗時ログの設計（PATH 不在 / 内部エラーの切り分け）も方針に含める
  - 方針確定後に受け入れ条件を整えて実装タスクへ昇格させる

- ローカル検証環境と CI の再現性 — 完了条件を具体化する
  - SQLite テストの不安定要因（macOS arm64 で `modernc.org/sqlite` 使用時に発生する事象）を特定し、安定化の具体条件を決める
  - `go test -race` がローカルで実行不能な事例を整理し、代替手順（CI に委ねる / Docker で回す等）の方針を決める
  - 制約整理の記録先（`docs/setup.md` か別 docs か）を決める

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` hook で Bash コマンドの stdout サイズを記録する想定
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定する
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
  - 受け入れ条件（記録先・閾値・集計方法）が未確定

- retro-pr との連携
  - PR の下位・上位 10% ずつは自動で retro-pr 実行する想定
  - 結果を PR と関連付けて表示する想定
  - 受け入れ条件（連携方式・表示先・自動化対象）が未確定

- sync-db の DROP & CREATE と Grafana クエリの race condition を解消する
  - 症状: ダッシュボード閲覧中に Stop hook が走ると `database is locked (261)` や「No data」「異常な軸スケール」が散発する。`internal/syncdb/schema.go` が全テーブル/ビューを毎回 DROP & CREATE しているため、Grafana の並列クエリが DROP 直後の空テーブルや再 CREATE 中のロックに当たる
  - 候補 A: `sessions` / `transcript_stats` を `INSERT OR REPLACE` の incremental UPSERT に切替。VIEW は DROP/CREATE のままで OK（行を持たないので race window 極小）
  - 候補 B: 別ファイル (`hitl-metrics.db.new`) に書き出して atomic rename。container 側の file mount で inode 切替が発生するため Grafana の再接続挙動を確認する必要あり
  - 候補 C: snapshot pattern — 5 分間隔で `cp` した read-only snapshot を Grafana に mount。リアルタイム性とのトレードオフ
  - 候補 D: `PRAGMA busy_timeout` — frser-sqlite-datasource 側に設定する手段がないため不採用見込み
  - 受け入れ条件（採用案・schema 変更範囲・既存テスト互換性）が未確定
