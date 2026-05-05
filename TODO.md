# TODO

hitl-metrics の開発タスクを管理する。完了したタスクは削除する。変更履歴は git log と GitHub Release を参照し、設計の経緯は `docs/history.md` に集約する。

## 実装タスク

- Codex CLI 対応 — Grafana ダッシュボード
  - [ ] Agent 別比較 stat パネルを追加（avg tokens / PR と PR / 1M tokens）
  - [ ] `make grafana-screenshot` を実行して `docs/images/dashboard-*.png` を更新する

- PR 別 token スコアカードに PR タイトル列を追加
  - 動機: `pr_url` だけだと「どの作業だったか」を思い出せない。タイトルを 2 列目に置けば外れ値検出からドリルダウンまでの認知コストが下がる
  - [ ] `sessions` テーブルに `pr_title TEXT NOT NULL DEFAULT ''` カラムを追加。`session-index.jsonl` レコードにも `pr_title` フィールドを追加（後方互換: 欠落時は空文字）
  - [ ] `internal/backfill` で `gh pr view <url> --json title` を呼び出し、`is_merged` / `review_comments` と同じバッチで取得・保存する
  - [ ] `sync-db` で `session-index.jsonl` の `pr_title` を `sessions.pr_title` に転写する
  - [ ] `pr_metrics` VIEW に `MAX(s.pr_title) AS pr_title` を追加する
  - [ ] ダッシュボードのスコアカードに `pr_title` を 2 列目（`pr` の右隣）として追加する。長いタイトルは SQL で `substr(pr_title, 1, 50) || CASE WHEN length(pr_title) > 50 THEN '…' ELSE '' END` 相当に切り詰め、Grafana の `wrapText` でセル内改行表示する
  - [ ] `docs/spec.md` の `sessions` テーブル / `session-index.jsonl` レコード / `pr_metrics` 集約カラム一覧に `pr_title` を追記する
  - [ ] `docs/design.md` に backfill が `gh pr view --json title` を呼ぶ実装方針を追記する
  - [ ] 既存セッションへ `agent-telemetry backfill --recheck` を実行し、すべての merged PR で `pr_title` が埋まることを確認する
  - [ ] `make grafana-screenshot` を実行して `docs/images/dashboard-*.png` を更新する

- `examples/skills/analyze-pr/` に Claude Code skill のサンプルを追加
  - 動機: PR スコアカードで示唆された「外れ値 PR」を coding agent で分析する自動化のリファレンス実装を提供する。agent-telemetry の責務は外れ値 PR の示唆までだが、Claude Action / Claude Web Routine 等で自動化したいユースケースは成立するため、reference を `examples/` に同梱する
  - 設計判断:
    - 配置先は `examples/skills/`（別リポジトリ化はしない）。リポジトリスコープを薄めるリスクより同梱の利便性を優先
    - 引数名は `--worst-by <column> --limit <n>`（`--bottom 5 --by fresh_tokens` 案より下位の意味が明示的）
    - `dashboard.json` は `examples/` に移動しない。`grafana/` 配下の primary 可視化として現状維持（skill は sample、dashboard は default 同梱で性質が異なり、移動による連鎖修正コスト—Makefile / docker-compose / provisioning yaml / docs—が利得を上回る）
  - [ ] `examples/skills/analyze-pr/SKILL.md` を作成する
  - [ ] 引数を 2 系統サポート: 単発 `<pr_url>` と 一括 `--worst-by <pr_metrics_column> --limit <n>`
  - [ ] skill 内処理: `~/.claude/agent-telemetry.db` から PR と transcript path を SELECT し、transcript を読んで token 消費の外れ値要因と改善仮説を Markdown で stdout に出すのみ（出力先決定—Issue 作成 / PR コメント / Slack 投稿等—は呼び出し側責務）
  - [ ] `examples/README.md` を作成し、Claude Action / Claude Web Routine からの呼び出し例を 2〜3 件提示する
  - [ ] `docs/usage.md` に「自動化サンプル」セクションを追加し `examples/skills/` を参照する
  - [ ] `examples/` は best-effort 扱いと明記する（CI 必須にしない、`make grafana-screenshot` のような必須作業から外す）
  - [ ] transcript の privacy 注意喚起を README に明記する（外部送信前にスコープ確認）

- リポジトリリネーム — agent-telemetry（フェーズ 3〜5 完了 / 残: タグ push でのリリース動作確認）
  - 背景・決定事項（D1〜D4）と BREAKING CHANGE 一覧は `docs/history.md` 「8. リポジトリ名変更 — hitl-metrics → agent-telemetry（2026-05-04）」を参照
  - フェーズ 6 残タスク
    - [ ] tag push で GoReleaser 動作確認 + リリース（バイナリ名 `agent-telemetry_<os>_<arch>.tar.gz` が生成されることを確認）

## 検討中

- Stop hook の `agent-telemetry` PATH 依存をなくす — 解決方針を決める
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
