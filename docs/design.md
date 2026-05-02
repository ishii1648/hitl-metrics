# hitl-metrics 設計

この文書は `docs/spec.md` の振る舞いをどう実現するかを記述する。
ユーザ視点の外部契約（CLI・データモデル・hook 仕様）は `docs/spec.md` を正とする。
過去の実装の経緯と廃止された設計は `docs/history.md` に分離する。

---

## 全体構成

3 層構成。データ収集層は agent ごとにアダプタを分離する。

```
[データ収集層]   Go subcommand hooks (agent アダプタ)
                ├ ~/.claude/session-index.jsonl  (claude)
                └ ~/.codex/session-index.jsonl   (codex)
                ~/.{claude,codex}/hitl-metrics-state.json
       │
       ▼
[データ変換層]   hitl-metrics CLI
                backfill / sync-db (agent ごとに走査して 1 つの DB に集約)
       │
       ▼
[可視化層]       SQLite + Grafana (coding_agent カラムで分類)
```

| 層 | パッケージ | 主な責務 |
|---|---|---|
| Agent アダプタ | `internal/agent/`, `internal/agent/claude/`, `internal/agent/codex/` | データディレクトリ・hook 入力スキーマ・transcript 形式の差を吸収 |
| データ収集 | `internal/hook/` | SessionStart / SessionEnd / Stop / PostToolUse hook の処理。中間ファイル書き込み |
| データ変換 | `internal/sessionindex/`, `internal/backfill/`, `internal/transcript/`, `internal/syncdb/` | session-index と transcript を読み、SQLite を再構築 |
| 配布補助 | `internal/setup/`, `internal/doctor/` | セットアップ案内と検証（agent 別）。旧 `internal/install/` をリネーム |
| エントリポイント | `cmd/hitl-metrics/` | CLI dispatch（`--agent` パース） |

---

## Agent アダプタ層

### 抽象化のスコープ

agent 間で異なるのは次の 4 点のみ。それぞれ `internal/agent/` のインタフェースで吸収する。

| 観点 | Claude Code | Codex CLI |
|---|---|---|
| データディレクトリ | `~/.claude/` | `$CODEX_HOME` または `~/.codex/` |
| hook 入力スキーマ | `session_id` / `transcript_path` / `cwd` / `hook_event_name` | 同上 + `model` / `turn_id` / `source` 等。フィールド名は概ね共通 |
| SessionEnd 相当 | `SessionEnd` hook あり | `SessionEnd` 相当なし。Stop hook の最終発火で代替 |
| transcript 形式 | `~/.claude/projects/**/<session_id>.jsonl`。assistant message に `usage.*` トークン | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl[.zst]`。`event_msg.payload.type=="token_count"` で累積トークン |

session-index.jsonl のスキーマ・SQLite モデル・PR URL 検出ロジック・backfill の cursor 設計はすべて共通化する。agent 差分は **読み込み元と transcript パーサだけ** に閉じ込める。

### `internal/agent/` インタフェース

```go
type Agent interface {
    Name() string                       // "claude" or "codex"
    DataDir() string                    // ~/.claude or $CODEX_HOME
    SessionIndexPath() string           // <DataDir>/session-index.jsonl
    StatePath() string                  // <DataDir>/hitl-metrics-state.json
    ParseHookInput(io.Reader) (HookInput, error)
    ParseTranscript(path string) (TranscriptStats, error)
}
```

`Agent` の実装は `claude.New()` / `codex.New()` で取得し、CLI 側は `--agent` フラグまたは検出ロジックでインスタンスを選ぶ。

### Codex の SessionEnd 不在を Stop hook で代替

Codex には `SessionEnd` イベントが存在しない。代わりに `Stop` hook が応答完了ごとに発火するため、次の方針で `ended_at` を運用する:

1. Codex の `Stop` hook 発火ごとに `ended_at` を **常に上書き**（最後の Stop が事実上 SessionEnd 相当になる）
2. `end_reason` は `stop` で固定
3. Stop hook が呼ばれずプロセスが kill された場合は backfill フェーズで rollout JSONL の **最終 event タイムスタンプ** を読み、`ended_at` が空のセッションに反映する

(2) (3) によってプロセス強制終了でも終了時刻が失われない。session_concurrency_* VIEW の精度を保つために重要。

### `agent_version` の取得

`sessions.agent_version` は agent ごとに次の方法で取得する。取得失敗は空文字列として扱い、hook を失敗させない。

| agent | 一次ソース | フォールバック |
|---|---|---|
| Claude | SessionStart hook input の `version` フィールド（存在する場合） | 環境変数 `CLAUDE_CODE_VERSION` → 空文字列 |
| Codex | SessionStart hook input の `version` 系フィールド（要 Spike 確認） | rollout JSONL の最初のメタイベント内のバージョン情報 → 環境変数 → 空文字列 |

**hook 内で `--version` 等の外部コマンドは呼ばない**。hook の高速性を損なうため。hook input または環境変数で取れない場合は空のまま記録し、必要なら backfill フェーズで rollout / transcript の先頭メタから補完する。

`agent_version` は `pr_metrics` VIEW には集約しない。理由:

- 1 PR 内で複数セッション → 複数バージョンが混在しうる（バージョンアップを跨いで作業が続く場合）
- 平均化すると意味が壊れる
- バージョン跨ぎ比較は session レベルで集計するのが正しい（例: 「version A の全 session の token 効率」vs 「version B」）

ダッシュボードでは PR 別スコアカードの詳細展開で `sessions` を JOIN して表示し、テンプレート変数による絞り込みは将来必要になったタイミングで `sessions` ベースのパネルに対して個別追加する。

### `.jsonl.zst` 透過デコード

Codex は古い rollout JSONL を zstd 圧縮することがある。`internal/transcript/` に reader wrapper を追加し、拡張子で分岐する:

- `.jsonl` → `os.Open` のまま
- `.jsonl.zst` → `klauspost/compress/zstd` でストリーム解凍

依存追加: `github.com/klauspost/compress`。modernc.org/sqlite と同じく cgo フリーで Go プロジェクトの方針に整合する。

---

## データ収集層

### Go サブコマンド統一

hook はすべて `hitl-metrics hook <event> [--agent <claude|codex>]` の Go サブコマンドで実装する。Shell スクリプトは持たず、`settings.json` / `config.toml` には `{"type":"command","command":"hitl-metrics hook session-start --agent claude"}` のような形で登録する。

`--agent` を省略した場合は `claude` を既定値として扱い、既存の `~/.claude/settings.json` 登録（agent 引数なし）の後方互換を保つ。

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

`internal/transcript/Parse()` が agent ごとのアダプタを呼び分けて 1 セッション分の `TranscriptStats` を返す。出力スキーマは agent 共通。

#### Claude (`internal/transcript/claude.go`)

- `tool_use_total`: assistant の tool_use エントリ数
- `mid_session_msgs`: 2 件目以降の `type:"user"` で `tool_result` のみで構成されないもの
- `ask_user_question`: `tool_use.name == "ask-user-question"` の件数
- `usage` 系トークン: assistant message の `usage.input_tokens` / `output_tokens` / `cache_creation_input_tokens` / `cache_read_input_tokens` の合計
- `reasoning_tokens`: 0 固定
- `model`: 最後に観測した model
- `is_ghost`: `type:"user"` エントリが 0 件

#### Codex (`internal/transcript/codex.go`)

- `tool_use_total`: `event_msg.payload.type == "tool_call"` の件数（Bash / apply_patch / MCP tool 全て含む）
- `mid_session_msgs`: 2 件目以降の `event_msg.payload.type == "user_message"`（あるいは `UserPromptSubmit` 同等イベント）
- `ask_user_question`: 0 固定（Codex に AskUserQuestion 相当のツールが無いため。将来 PermissionRequest を流用するなら別途検討）
- token 系: `event_msg.payload.type == "token_count"` イベントの **最終累積値** を採用（input / output / cache_read / cache_write / reasoning）。途中 turn 単位の差分が必要な指標が無いため累積値で十分
- `reasoning_tokens`: `token_count.reasoning` の最終値
- `model`: rollout JSONL の最初のメタイベントから取得
- `is_ghost`: `event_msg.payload.type == "user_message"` が 0 件

`usage` / `token_count` いずれも欠落時は 0。Claude の古い transcript と Codex の新しい rollout を混ぜても sync-db が落ちない。

---

## データモデル設計

### session_id の名前空間

session_id は agent ごとに発行される UUID であり、衝突確率は実用上無視できるが、保証はない。`sessions` テーブルおよび `transcript_stats` テーブルの PRIMARY KEY を (`session_id`, `coding_agent`) の複合キーにすることで、衝突時にも区別できるようにする。

`session_id` を `claude:<uuid>` のように prefix 化する案も検討したが、外部出力（ログ、Grafana 表示）で生 UUID を扱える方が運用上扱いやすいため複合 PK 方式を採用する。

### `is_subagent` 判定

Claude Code の SessionStart hook は Task サブエージェントでは発火しない（Spike で確認済み）ため、`parent_session_id` フィールドはほとんど空になる。代わりに transcript ファイル名のパス構造（`{session_id}/subagents/agent-{agent_id}.jsonl`）からも判定可能だが、現状は `parent_session_id` の有無を主指標としている。

Codex はサブエージェント概念を持たないため `parent_session_id` は常に空・`is_subagent = 0` となる。

### `is_ghost` 判定

Claude Code はファイル編集履歴のスナップショットとして UUID 名の JSONL を作る場合があり、これらは `type:"user"` を含まない。SessionStart hook がこれらを「セッション」として記録してしまう問題を吸収するため、transcript 内に `type:"user"` が 1 件もない場合は `is_ghost = 1` にして PR 集計から除外する。

### LEFT JOIN 膨張バグの回避

`pr_metrics` VIEW では `transcript_stats` を session と 1:1 で JOIN する。permission_events のような 1:N の補助テーブルを LEFT JOIN すると `tool_use_total` が N 倍に膨張する事例があったため、N:1 の集約が必要な補助テーブルは事前集計サブクエリで結合する。

現在のスキーマは permission_events を持たないため該当箇所はないが、将来 1:N の補助テーブルを追加する際はこの方針を維持する。

### `task_type` の自動抽出（集計軸からは廃止）

`branch` カラムを `^(feat|fix|docs|chore)/` でマッチして `sessions.task_type` を埋める。マッチしないブランチは空文字列。

ADR-024 で task_type を集計軸から廃止したため、`pr_metrics` の集約・ダッシュボード panel ではこのカラムを使わない。schema にカラムは残すが、用途は SQL 実行時の任意フィルタと、過去の集計を再現する場合の後方互換に限定する。

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

### `setup` コマンド（旧 `install` のリネーム）

hook の自動登録はしない。dotfiles または手動で `~/.claude/settings.json` / `~/.codex/config.toml` を管理する前提に整合させるため。`setup [--agent <claude|codex>]` は agent 別の登録例を表示するだけで、書き込みは一切行わない。

旧 `install` という名前は次の理由でリネームした:

- 一般的な CLI 慣習（`pip install` 等）では「install」は実際にインストール／登録を行う。本コマンドは表示しかしないため、新規ユーザの期待を裏切っていた
- マルチエージェント対応で `--agent codex` が増え、「install というコマンドが Codex 設定に書き込みに来るのでは」という誤解を一層誘発する見通しがあった
- `setup` であれば「準備手順を出す」セマンティクスが自然で、`doctor` と並んで観察系コマンドであることが見た目で分かる

旧 `install` は廃止予定 alias として残し、deprecation warning を stderr に出して `setup` と同等の案内を表示する。次のメジャーバージョンで削除する。

### `uninstall-hooks` コマンド（旧 `install --uninstall-hooks` の独立化）

過去バージョンが `~/.claude/settings.json` に書き込んだ単一エントリを削除する。matcher 付き・複数 hook を束ねたエントリは人間編集の可能性が高いので触らない。

`install --uninstall-hooks` から独立サブコマンドに分離した理由:

- 旧 `install` は何も書き込まないのに `--uninstall-hooks` だけが破壊的、という非対称が認知的負荷だった
- リネーム後の `setup` に `--uninstall-hooks` を残すと「セットアップなのにアンインストール？」という語感破綻が発生する
- 独立サブコマンドにすることで、書き込みを伴う唯一の配布補助コマンドだと CLI 表面で明示できる

Codex 側 (`~/.codex/config.toml`) は提供しない。TOML で人間編集が前提のため自動削除のリスクが高い。

### `doctor` コマンド

検出された agent ごとに binary の PATH 配置・データディレクトリの存在・hook 登録状況をチェックする。Claude は `~/.claude/settings.json` の JSON、Codex は `~/.codex/config.toml`（および `~/.codex/hooks.json`）の TOML/JSON を読む。

未登録の hook は warning として表示するが**自動修復はしない**。dotfiles 一元管理の前提を壊さないため。

---

## 既知の制約

- `Stop` hook はブロッキング実行のため、極端に遅い `gh pr list` を含む場合は応答完了の直後に最大数秒の待機が発生する
- `pr_urls` の順序保持と「最後の 1 件」採用の整合性は現在検証中（`TODO.md` 参照）
- `session-index.jsonl` の書き戻しは atomic 化未対応。書き込み中の停電やプロセスキルで truncate される可能性が残る（`TODO.md` 参照）
- `sync-db` は SQLite の WAL モードを前提にせず単純な DROP & CREATE で再構築するため、Grafana がクエリ中に実行すると一時的にエラーが出る場合がある
- transcript のパス取得失敗時は当該セッションの `transcript_stats` が空になるが、`sessions` 行は残る
- Codex の SessionEnd 不在を Stop hook で代替するため、Stop hook を経由せずプロセスが kill された場合は最後の Stop 発火時刻が `ended_at` になる（rollout JSONL 最終 event での補正は backfill 経由）
- Codex の `ask_user_question` 相当指標が無いため、agent を跨いだ「仕様不明瞭さ」比較はできない
