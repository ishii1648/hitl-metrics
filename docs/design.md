# hitl-metrics 設計

この文書は `docs/spec.md` の振る舞いをどう実現するかを記述する。
ユーザ視点の外部契約（CLI・データモデル・hook 仕様）は `docs/spec.md` を正とする。
過去の実装の経緯と廃止された設計は `docs/history.md` に分離する。

---

## 全体構成

3 層構成。

```
[データ収集層]   Go subcommand hooks
                ~/.claude/session-index.jsonl
                ~/.claude/hitl-metrics-state.json
       │
       ▼
[データ変換層]   hitl-metrics CLI
                backfill / sync-db
       │
       ▼
[可視化層]       SQLite + Grafana
```

| 層 | パッケージ | 主な責務 |
|---|---|---|
| データ収集 | `internal/hook/` | SessionStart / SessionEnd / Stop hook の処理。中間ファイル書き込み |
| データ変換 | `internal/sessionindex/`, `internal/backfill/`, `internal/transcript/`, `internal/syncdb/` | session-index と transcript を読み、SQLite を再構築 |
| 配布補助 | `internal/install/`, `internal/doctor/` | セットアップ案内と検証 |
| エントリポイント | `cmd/hitl-metrics/` | CLI dispatch |

---

## データ収集層

### Go サブコマンド統一

hook はすべて `hitl-metrics hook <event>` の Go サブコマンドで実装する。Shell スクリプトは持たず、`settings.json` には `{"type":"command","command":"hitl-metrics hook session-start"}` のような形で登録する。

理由:

- Shell 5 本に分散していた tool annotation・JSON パース・git 操作のロジックを `internal/hook/` の共通関数に集約できる
- バイナリへの埋め込み (`embed`) と展開 (`ExtractHooks`) が不要になり、配布が「PATH 上にバイナリがあること」だけで完結する
- awk による複雑なパース（旧 `todo-cleanup-check.sh` の 80 行）を Go テストでカバーできる
- Go バイナリの起動コスト（〜10 ms）は hook の発火間隔に対して無視できる

### Stop hook のブロッキング実行

`Stop` hook は `hitl-metrics backfill && hitl-metrics sync-db` を同期実行する。fire-and-forget や非同期化はしない。

ブロッキングを許容する根拠:

- 発火タイミングがセッション応答完了直後であり、ユーザーの次操作までの体感影響が小さい
- backfill は cursor 方式で増分処理する（後述）
- マージ判定の Phase 2 は `last_meta_check` から一定時間（1 時間）経過した場合のみ走る
- 各グループの `gh pr` 呼び出しは goroutine で 8 並列、1 件あたり 8 秒タイムアウト

過去には fire-and-forget や launchd cron も試みたが、launchd は Claude Code 外の唯一の手作業になり UX が悪化していた。詳細は `docs/history.md` を参照。

### `session-index.jsonl` の追記モデル

`session-index.jsonl` は append-only に近い扱いで、SessionStart で新規 1 行を追加し、SessionEnd / backfill / `update` ではマッチする `session_id` の行を読み直して書き戻す。

書き戻しは現状 in-place で行っている。書き込み中断時の truncate 耐性を上げる atomic 化（一時ファイル + rename）は `TODO.md` に積んでいる。

---

## データ変換層

### `sync-db` の DROP & CREATE 戦略

`sync-db` は実行ごとに DB をすべて DROP & CREATE で再構築し、1 トランザクションで一括 COMMIT する。マイグレーションは存在しない。

理由:

- ソース・オブ・レコードは `session-index.jsonl` と transcript JSONL であり、SQLite はあくまで集計キャッシュ
- スキーマ変更時の互換コードを抱えなくて済む
- DB 破損時はファイルを消して再実行すれば回復する

中間ファイルが SoR である構造は、hook の書き込みを軽量に保つためでもある。hook はセッション中に同期実行されるため、JSONL への追記だけに留める。構造化変換は `sync-db` に委譲する。

### `backfill` の cursor + 2 フェーズ設計

`hitl-metrics-state.json` に `last_backfill_offset`（JSONL の処理済み行数）と `last_meta_check`（Phase 2 の最終実行時刻）を保存する。

| フェーズ | 対象 | 実行条件 |
|---|---|---|
| Phase 1: URL 補完 | cursor 以降の新規エントリで `pr_urls` 空かつ `backfill_checked=0` のもの | 毎回 |
| Phase 2: マージ判定 | 既存 PR の `is_merged` と `review_comments` の再チェック | `last_meta_check` から一定時間経過時のみ |

`--recheck` 指定時は cursor を無視してフルスキャンする。

cursor が古くても結果に影響はない。`backfill_checked` フラグが API 呼び出しの永続スキップを担うため、cursor は単なる効率化のヒントとして扱う。

### `(repo, branch)` グルーピングと `backfill_checked`

backfill は `pr_urls` が空のセッションを `(repo, branch)` でグループ化し、`gh pr list` を 1 回だけ実行する。同一ブランチで複数セッションがあっても API 呼び出しは 1 回。

PR が存在しないブランチ（`main` / `master` 等）は初回チェック後に `backfill_checked: true` をセットして永続スキップする。これがないと dotfiles の `master` のような大量エントリが毎回 8 秒の空振りを起こす。

### 並列化

`(repo, branch)` グループの `gh pr list` 呼び出しは goroutine で 8 並列実行する。

- GitHub API の認証済みレート制限（5,000 req/h）に対し、毎時 1 回実行 × 並列 8 でも余裕がある
- 書き込み（`session-index.jsonl` への反映）は逐次にして競合を回避する

### `pr_urls` の採用ルール

`session-index.jsonl` の `pr_urls` は配列だが、`sync-db` がセッション → PR の単一 URL に変換する際は **配列の最後の 1 件** を採用する。

`update` / `backfill` が PR URL を追記する順序が結果に影響するため、辞書順ソートはしない。順序保持と「最後の 1 件」採用の整合性は `TODO.md` の検証タスクで継続検証する。

### transcript パース

`internal/transcript/Parse()` が transcript JSONL を 1 セッション分読み、以下を集計する。

- `tool_use_total`: assistant の tool_use エントリ数
- `mid_session_msgs`: 2 件目以降の `type:"user"` で `tool_result` のみで構成されないもの
- `ask_user_question`: `tool_use.name == "ask-user-question"` の件数
- `usage` 系トークン: assistant message の `usage.input_tokens` / `output_tokens` / `cache_creation_input_tokens` / `cache_read_input_tokens` の合計
- `model`: 最後に観測した model（同一セッション内で混在しうる場合は最後の値）
- `is_ghost`: `type:"user"` エントリが 0 件の場合に true

`usage` 欠落時は 0 として扱う。古い transcript と新しい transcript を混ぜても sync-db が落ちない。

---

## データモデル設計

### `is_subagent` 判定

SessionStart hook は Task サブエージェントでは発火しない（Spike で確認済み）ため、`parent_session_id` フィールドはほとんど空になる。代わりに transcript ファイル名のパス構造（`{session_id}/subagents/agent-{agent_id}.jsonl`）からも判定可能だが、現状は `parent_session_id` の有無を主指標としている。

### `is_ghost` 判定

Claude Code はファイル編集履歴のスナップショットとして UUID 名の JSONL を作る場合があり、これらは `type:"user"` を含まない。SessionStart hook がこれらを「セッション」として記録してしまう問題を吸収するため、transcript 内に `type:"user"` が 1 件もない場合は `is_ghost = 1` にして PR 集計から除外する。

### LEFT JOIN 膨張バグの回避

`pr_metrics` VIEW では `transcript_stats` を session と 1:1 で JOIN する。permission_events のような 1:N の補助テーブルを LEFT JOIN すると `tool_use_total` が N 倍に膨張する事例があったため、N:1 の集約が必要な補助テーブルは事前集計サブクエリで結合する。

現在のスキーマは permission_events を持たないため該当箇所はないが、将来 1:N の補助テーブルを追加する際はこの方針を維持する。

### `task_type` の自動抽出

`branch` カラムを `^(feat|fix|docs|chore)/` でマッチして抽出する。マッチしないブランチは空文字列とし、ダッシュボードでは `(unknown)` として扱う。

### `session_concurrency_*` VIEW

`sessions.timestamp` と `sessions.ended_at` の区間重なりから同時実行数を算出する。`ended_at` が空のセッションは現在時刻で打ち切る。subagent / ghost / dotfiles を除外する。

---

## 可視化層

### SQLite + Grafana の選定

Prometheus + Grafana ではなく SQLite + Grafana を採用している。理由は「任意の日付範囲で PR 別に集計する」という用途が SQL の典型ユースケースであり、Prometheus の「現在状態のスクレイプ」モデルとは合わないため。

ClickHouse / Loki も候補だが、個人利用規模では SQLite で十分。

### datasource uid の固定化

ダッシュボード JSON の datasource は `uid: hitl-metrics` で固定し、Grafana provisioning が解決しない `${DS_*}` テンプレート変数は使わない。`__inputs` セクションも持たない。

### E2E スクリーンショット

`make grafana-screenshot` が Docker Compose で Grafana + Image Renderer を起動し、Render API でパネルごとに PNG を取得する。Playwright 等のブラウザ自動操作は採用しない（Go プロジェクトに異質な依存を持ち込まないため、また Image Renderer で十分なため）。

ダッシュボードを変更したら必ず `make grafana-screenshot` を実行し、`docs/images/dashboard-*.png` も合わせて更新する（CLAUDE.md の必須作業）。

---

## 配布補助

### `install` コマンド

hook の自動登録はしない。dotfiles または手動で `~/.claude/settings.json` を管理する前提に整合させるため。`install` は登録例を表示するだけで、`--uninstall-hooks` のみ過去バージョンが書き込んだ単一エントリを削除する（matcher 付き・複数 hook を束ねたエントリは人間編集の可能性が高いので触らない）。

### `doctor` コマンド

binary の PATH 配置・データディレクトリの存在・hook 登録状況をチェックする。未登録の hook は warning として表示するが**自動修復はしない**。dotfiles 一元管理の前提を壊さないため。

---

## 既知の制約

- `Stop` hook はブロッキング実行のため、極端に遅い `gh pr list` を含む場合は応答完了の直後に最大数秒の待機が発生する
- `pr_urls` の順序保持と「最後の 1 件」採用の整合性は現在検証中（`TODO.md` 参照）
- `session-index.jsonl` の書き戻しは atomic 化未対応。書き込み中の停電やプロセスキルで truncate される可能性が残る（`TODO.md` 参照）
- `sync-db` は SQLite の WAL モードを前提にせず単純な DROP & CREATE で再構築するため、Grafana がクエリ中に実行すると一時的にエラーが出る場合がある
- transcript のパス取得失敗時は当該セッションの `transcript_stats` が空になるが、`sessions` 行は残る
