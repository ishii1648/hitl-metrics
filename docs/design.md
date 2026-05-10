# agent-telemetry 設計

この文書は `docs/spec.md` の振る舞いをどう実現するかを記述する。
ユーザ視点の外部契約（CLI・データモデル・hook 仕様）は `docs/spec.md` を正とする。
過去の実装の経緯と廃止された設計は `issues/closed/` の retro issue に分離する。

---

## 全体構成

3 層構成。データ収集層は agent ごとにアダプタを分離する。

```
[データ収集層]   Go subcommand hooks (agent アダプタ)
                ├ ~/.claude/session-index.jsonl  (claude)
                └ ~/.codex/session-index.jsonl   (codex)
                ~/.{claude,codex}/agent-telemetry-state.json
       │
       ▼
[データ変換層]   agent-telemetry CLI
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
| エントリポイント | `cmd/agent-telemetry/` | CLI dispatch（`--agent` パース） |

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
    StatePath() string                  // <DataDir>/agent-telemetry-state.json
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

hook はすべて `agent-telemetry hook <event> [--agent <claude|codex>]` の Go サブコマンドで実装する。Shell スクリプトは持たず、`settings.json` / `config.toml` には `{"type":"command","command":"agent-telemetry hook session-start --agent claude"}` のような形で登録する。

`--agent` を省略した場合は `claude` を既定値として扱い、既存の `~/.claude/settings.json` 登録（agent 引数なし）の後方互換を保つ。

理由:

- Shell 5 本に分散していた tool annotation・JSON パース・git 操作のロジックを `internal/hook/` の共通関数に集約できる
- バイナリへの埋め込み (`embed`) と展開 (`ExtractHooks`) が不要になり、配布が「PATH 上にバイナリがあること」だけで完結する
- awk による複雑なパース（旧 `todo-cleanup-check.sh` の 80 行、現在は `todo-cleanup` 系統ごと廃止済み）を Go テストでカバーできる
- Go バイナリの起動コスト（〜10 ms）は hook の発火間隔に対して無視できる

### Stop hook のブロッキング実行

`Stop` hook は `agent-telemetry backfill && agent-telemetry sync-db` を同期実行する。fire-and-forget や非同期化はしない。

ブロッキングを許容する根拠:

- 発火タイミングがセッション応答完了直後であり、ユーザーの次操作までの体感影響が小さい
- backfill は cursor 方式で増分処理する（後述）
- マージ判定の Phase 2 は `last_meta_check` から一定時間（1 時間）経過した場合のみ走る
- 各グループの `gh pr` 呼び出しは goroutine で 8 並列、1 件あたり 8 秒タイムアウト

過去には fire-and-forget や launchd cron も試みたが、launchd は Claude Code 外の唯一の手作業になり UX が悪化していた。詳細は [issues/closed/0020-design-backfill-evolution-to-stop-hook.md](../issues/closed/0020-design-backfill-evolution-to-stop-hook.md) を参照。

### PR の確定は Stop hook で early binding

PR と session の紐づけは `Stop` hook 時点で `gh pr list --head <branch>` を 1 回叩いて確定する。1 件取れた場合は `pr_urls` を `[<url>]` で置き換え、`pr_pinned: true` を立てる。pinned 状態に入ったセッションは以降 PostToolUse / `update` / `backfill` の URL 追記をすべて拒否する。late binding（backfill 経由）は **PR 未作成のままセッションが終わったケースのフォールバック** として残す。

このルールが解決する誤接続:

- **PostToolUse 正規表現の汚染** — `gh pr view 999` や `gh pr list` の出力、ユーザが Bash で他人の PR URL を貼ったケースで `pr_urls` 末尾に無関係な PR が付き、`sync-db` が末尾を採用するため誤った PR に紐づく問題。pin 後はすべての append が no-op になるため塞がる。
- **ブランチ再利用** — 同一ブランチで別 PR を使い回す運用で、新 PR の URL が古いセッションに付与される問題。pin の時点で `(repo, branch)` 解決を済ませているため、後から作られた別 PR の影響を受けない。

実装の要点:

- pin 経路は `internal/sessionindex.PinPR` に集約。`pr_urls` の追加 (`Update` / `UpdateByBranch`) は内部で `PRPinned == true` をスキップする。
- Stop hook の lookup は `gh pr list --head <branch> --author @me --state all --limit 1` を `cwd` で実行する。これは backfill の URL 解決経路と同じ呼び出しなので、pin した URL と backfill が同 tick で解決した場合の URL は一致する。
- `cwd` 不在 / git リポジトリでない / branch 空 / `gh` がエラーの場合は best-effort で skip し、Stop の hot path を落とさない。fallback として backfill が次 tick で再試行する。
- `Phase 2` の meta 取得は pinned セッションも対象に含める（`is_merged` / `review_comments` の更新は継続したい）。pin で抑止するのは **URL の追記だけ**。

### `session-index.jsonl` の追記モデル

`session-index.jsonl` は append-only に近い扱いで、SessionStart で新規 1 行を追加し、SessionEnd / backfill / `update` ではマッチする `session_id` の行を読み直して書き戻す。

書き戻しは現状 in-place で行っている。書き込み中断時の truncate 耐性を上げる atomic 化（一時ファイル + rename）は今後の課題として残しており、「既知の制約」節に再掲する。

---

## データ変換層

### `sync-db` の incremental UPSERT 戦略

`sync-db` は通常実行で `sessions` / `transcript_stats` を `INSERT OR REPLACE` するだけで、テーブル/VIEW の DROP & CREATE は行わない。スキーマ DDL は `internal/syncdb/schema.sql` に集約し、SHA256 ハッシュを `go:generate` で `schema_hash.go` に埋め込む。起動時に埋め込みハッシュと DB の `schema_meta` テーブルに保存されたハッシュを比較する:

| 状態 | DDL 実行 | 行の書き込み |
|---|---|---|
| ハッシュ一致 | しない | `INSERT OR REPLACE` のみ |
| ハッシュ不一致 / `schema_meta` 不在 | `schema.sql` を全実行（既存 VIEW/TABLE を DROP & CREATE）| `INSERT OR REPLACE` 後に新ハッシュを `schema_meta` へ書き込む |

理由:

- ソース・オブ・レコードは `session-index.jsonl` と transcript JSONL であり、SQLite はあくまで集計キャッシュ
- スキーマ変更時の互換コードを抱えなくて済む（DDL 自体はハッシュ不一致時にのみフル再構築する）
- DB 破損時はファイルを消して再実行すれば回復する
- DDL を毎回実行しないため、`sync-db` 実行中も Grafana のクエリは VIEW を見失わない（race condition の解消）

中間ファイルが SoR である構造は、hook の書き込みを軽量に保つためでもある。hook はセッション中に同期実行されるため、JSONL への追記だけに留める。構造化変換は `sync-db` に委譲する。

`schema.sql` の編集忘れによるハッシュ未更新を防ぐため、CI（`.github/workflows/schema-hash.yml`）で `go generate ./... && git diff --exit-code` を実行する。

### `backfill` の cursor + 2 フェーズ設計

`agent-telemetry-state.json` に `last_backfill_offset`（JSONL の処理済み行数）と `last_meta_check`（Phase 2 の最終実行時刻）を保存する。

| フェーズ | 対象 | 実行条件 |
|---|---|---|
| Phase 1: URL 補完 | cursor 以降の新規エントリ + cursor 以下で `pr_urls` 空かつ `backfill_checked=0` のリトライ待ちエントリ | 毎回 |
| Phase 2: マージ判定 | 既存 PR の `is_merged` と `review_comments` の再チェック | `last_meta_check` から一定時間経過時のみ |

`--recheck` 指定時は cursor を無視してフルスキャンする。

cursor が古くても結果に影響はない。`backfill_checked` フラグが API 呼び出しの永続スキップを担うため、cursor は単なる効率化のヒントとして扱う。Stop hook 起動時にまだ PR が作られていなかったセッション（リトライ待ち）は cursor の進行とは独立して毎回再評価する — そうしないと PR が後から作られたときに永久に取りこぼす。

### PR タイトルの取得

backfill は `gh pr list` / `gh pr view` の `--json` 引数に `title` を含めて、`is_merged` / `review_comments` / `changes_requested` と同じ呼び出しで PR タイトルを取得する。追加の API 呼び出しは発生しない（同じレスポンスから別フィールドを抽出するだけ）。

取得した `title` は `sessionindex.UpdatePRMeta` を介して同一 `pr_url` を持つ全セッションの `pr_title` フィールドに転写する。`sync-db` は `sessions.pr_title` カラムへ単純コピーし、`pr_metrics` VIEW では `MAX(s.pr_title)` で集約する。

空文字列での上書きはしない。`gh` が title を返さなかった（タイトルが空 / API エラーで取得失敗）場合に既存の `pr_title` を消さないため、`UpdatePRMeta` は `prTitle == ""` のとき `pr_title` フィールドを書き換えずスキップする。

### `(repo, branch)` グルーピングと `backfill_checked`

backfill は `pr_urls` が空のセッションを `(repo, branch)` でグループ化し、`gh pr list` を 1 回だけ実行する。同一ブランチで複数セッションがあっても API 呼び出しは 1 回。

PR が存在しないブランチ（`main` / `master` 等）は初回チェック後に `backfill_checked: true` をセットして永続スキップする。これがないと dotfiles の `master` のような大量エントリが毎回 8 秒の空振りを起こす。

### 並列化

`(repo, branch)` グループの `gh pr list` 呼び出しは goroutine で 8 並列実行する。

- GitHub API の認証済みレート制限（5,000 req/h）に対し、毎時 1 回実行 × 並列 8 でも余裕がある
- 書き込み（`session-index.jsonl` への反映）は逐次にして競合を回避する

### `pr_urls` の採用ルール

`session-index.jsonl` の `pr_urls` は配列だが、`sync-db` がセッション → PR の単一 URL に変換する際は **配列の最後の 1 件** を採用する。

`update` / `backfill` が PR URL を追記する順序が結果に影響するため、辞書順ソートはしない。

通常の運用では Stop hook の pin により `pr_urls` は要素 1 件で確定する（前述「PR の確定は Stop hook で early binding」）。late binding（pin 失敗 → backfill 経由）でも要素 1 件になる。複数要素になるのは、pin 前に PostToolUse の正規表現で複数 URL がスクレイプされた極端なケースに限られ、pin 後は `[<確定 URL>]` で置き換えられるため一過性で残らない。

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

## ユーザ識別子

### 取得経路と優先順位

`session-index.jsonl` の `user_id` を埋める際に参照するソースの優先順位は次のとおり。**先に値が取れたものを採用** する。

| 優先 | ソース | 用途 |
|---|---|---|
| 1 | 環境変数 `AGENT_TELEMETRY_USER` | CI / コンテナでの決定的な上書き口 |
| 2 | `~/.claude/agent-telemetry.toml` の `user` キー | 永続的な人間識別子。dotfiles で複数マシンに同一値を配る前提 |
| 3 | `git config --global user.email` | フォールバック。`--global` のみ参照する |
| 4 | （取得失敗）`unknown` | hook を失敗させない |

`git config --local` を **意図的に見ない**。理由:

- リポジトリごとに別 email を設定している運用（OSS と業務でメールを分ける）で、同一人物が分裂するのを避ける
- hook の cwd が git リポジトリでないケース（`~/` で起動、temp dir で起動）で取得が揺れる
- マシン跨ぎで人物を束ねるという user attribution の本来目的と逆方向

`internal/userid/` で Resolver を実装。hook と sync-db から共通で呼び出す。`Resolve()` は (識別子, 取得元) を返し、doctor が「どのソースから来たか」を表示できるようにする。

### 形式とハッシュ化

`user_id` の形式は **任意の文字列**（メールアドレスでも pseudonym でも UUID でも可）。ハッシュ化は **しない**。理由:

- ハッシュ化は join 不可で、複数マシンからの集約に困る（束ねるためのキーとして使えない）
- 表示と保存を分離したいケースは、ユーザが TOML に pseudonym を書くだけで成立する
- 組織内利用での人間可読性を阻害したくない

PII 取り扱いをどうしても分離したい場合は、TOML の `user` キーに pseudonym を入れて運用する選択肢がある。サーバ側のアクセス制御は 0009 の AuthN/AuthZ 設計で扱う（0010 のスコープ外）。

### 欠損時の扱い

- ローカル運用では `unknown` で記録し、hook を失敗させない
- 異なる人物の `unknown` レコードが集約から区別できない問題は、サーバ送信時のゲート判定（0009）で対処する
- `pr_metrics` などの VIEW では `unknown` を集計から除外しない（ローカル単独運用で `user_id` が未設定でもダッシュボードが空にならないよう、`unknown` も 1 ユーザとして扱う）

### 既存 `session-index.jsonl` レコードへの埋め戻し

`sync-db` は読み込んだレコードに `user_id` フィールドが欠落していれば、`internal/userid.Resolve()` の現在値で埋め、JSONL に書き戻す。これで JSONL を SoR として一貫させられる。マイグレーションコマンドは追加しない（既存方針通りスキーマハッシュ不一致で再構築）。

注意: 過去にマシン A で記録されたセッションをマシン B の DB に取り込んだ場合、現在の `user_id` で埋まる。これは仕様割り切り（過去ログのマシン間移動はサポートしない）。

### `pr_metrics` VIEW の集約軸

GROUP BY に `user_id` を加えて (`pr_url`, `coding_agent`, `user_id`) で集約する。同一 PR を複数ユーザが触ったケース（pair coding / 引き継ぎ）で人物別に分離するため。単独利用時は 1 PR = 1 行のままで結果は変わらない。

`session_concurrency_*` VIEW は既存互換のため `user_id` を集約軸に追加しない。user 別の同時実行数が必要になったら別 VIEW として追加する。

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

ダッシュボード JSON の datasource は `uid: agent-telemetry` で固定し、Grafana provisioning が解決しない `${DS_*}` テンプレート変数は使わない。`__inputs` セクションも持たない。

### 週別 time series のプロット位置

週別パネルの SQL では `time = strftime('%s', week_start, '+3 days', '+12 hours')` を返し、データポイントを **週の中央（木曜 12:00 UTC = JST 木曜 21:00）** にプロットする。`week_start` 自身（月曜 00:00 UTC = JST 月曜 09:00）をそのまま `time` に使うと、Grafana の time range（例: Last 7 days）の `__from` に対して JST 月曜の朝が境界より前に位置し、X 軸範囲外で描画されない週が出てしまう。中央プロットなら通常の time range で確実に範囲内に入り、また「週の代表値」としても直感的。

合わせて WHERE 句は `week_start BETWEEN date('${__from:date:iso}', '-7 days') AND date('${__to:date:iso}')` と `__from` を 7 日緩める。データポイント時刻が `__from` ～ `__to` に入る週でも `week_start` は最大 6 日前になりうるため。

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

## サーバ側集約パイプライン

### 全体方針

ローカル `~/.claude/agent-telemetry.db` に閉じていたメトリクスを、複数マシン・複数ユーザのデータを統合できるサーバへ送る経路を追加する。クライアントは `sync-db` 完了後の **集計値のみ** を push し、サーバはそれをクライアントと同一の SQLite スキーマでそのまま upsert する。集計（transcript パース・PR 集計）はクライアント側で完結し、サーバは「dumb ingest layer」として責務を最小化する。ローカル Grafana とサーバ Grafana が同じダッシュボード JSON を共有できるよう、サーバの DB スキーマはクライアントと同一のものを使う。

### 送信するもの — `sessions` 行 + `transcript_stats` 行

クライアントは `~/.claude/agent-telemetry.db` の `sessions` と `transcript_stats` から差分行を抽出して送る。`session-index.jsonl` の生行・transcript JSONL（会話本体）・rollout JSONL は **送らない**。

理由:

- 送信サイズが圧倒的に小さい（1 セッションあたり 1〜2 KB、月数 MB）
- サーバ側の集計負荷がゼロ。`internal/syncdb/` を持たず、受信値の `INSERT OR REPLACE` だけ
- transcript（会話本体）がサーバに渡らないため、プライバシー観点の議論が不要になる
- transcript / rollout の保管はクライアント手元に残るので、新メトリクスを追加したくなった場合は「全クライアントを新 binary に更新 → 各クライアントで `sync-db --recheck && push --full`」で遡及反映できる

### 採用しなかった代替

- **raw JSONL 転送 + サーバ側 `internal/syncdb/` 再実行**: 当初の第一候補。サーバ側で過去 transcript から新メトリクスを再集計できる利点があったが、(1) 送信サイズが 1 セッション数 MB〜数十 MB に膨らむ、(2) サーバが transcript を保管することになりプライバシー観点とストレージ運用の議論が必須になる、(3) サーバ側で transcript パース処理のメンテナンスが発生する、の 3 点が大きい。指標追加の遡及反映は前述のクライアント binary 配布経路で代替できるため、軽量な集計値転送を優先した
- **OTLP / Prometheus remote write**: agent-telemetry のデータは PR 単位の構造化レコードで、`is_merged` / `pr_url` / `review_comments` が backfill で後追い更新される。append-only に近い OTLP 経路では遡及更新の表現が面倒。ローカル SQLite + Grafana スタックと OTel collector / Mimir スタックの二重メンテナンスも避けたい。`docs/metrics.md` の OpenMetrics カタログは観察軸の整理として残るが、収集経路の固定化は意味しない
- **`agent-telemetry-server` でも `internal/syncdb/` 全体を共通化**: 集計値送信に切り替えたためサーバ側で集計ロジックは不要。共通化対象は schema DDL（`schema.sql`）だけで十分

### プロトコル — 独自 HTTP JSON

```
POST /v1/metrics
Authorization: Bearer <api_key>
Content-Type: application/json
Content-Encoding: gzip   (optional)

{
  "client_version": "x.y.z",
  "schema_hash": "<sync-db スキーマ SHA-256>",
  "sessions": [
    { "session_id": "...", "coding_agent": "...", ... }
  ],
  "transcript_stats": [
    { "session_id": "...", "coding_agent": "...", ... }
  ]
}
```

- 各行のスキーマは `docs/spec.md ## SQLite データモデル` の `sessions` / `transcript_stats` テーブルと完全一致
- `schema_hash` はクライアント `internal/syncdb/schema_hash.go` の埋め込み値。サーバ DB の `schema_meta` ハッシュと一致しない場合、サーバは `schema_mismatch: true` を返して受信拒否
- HTTP gzip は **optional**（集計値だけなので無圧縮でも数 KB〜数百 KB で収まる）。クライアントは payload size を見て gzip 適用を判断
- 1 リクエスト 50 MB 上限（保険）。集計値だけなので通常は超えない

### 差分検知 — `state.json` の `pushed_session_versions`

`agent-telemetry-state.json` に新フィールドを追加する:

```json
{
  "last_backfill_offset": 123,
  "last_meta_check": "...",
  "pushed_session_versions": {
    "<session_id>": "<sha256 of sessions row + transcript_stats row>"
  }
}
```

- 各 push 時に対象セッションの `sessions` 行 + `transcript_stats` 行を JSON canonicalize → SHA-256 → `pushed_session_versions` と比較
- hash 一致 → 既送信、スキップ
- hash 不一致 → 後追い更新あり、再送信
- backfill が `is_merged` / `pr_url` / `review_comments` / `pr_title` を更新すると `sessions` 行の hash が変わり、再送信される
- 進行中セッション（`ended_at` または `end_reason` が空）は送信対象外
- `agent-telemetry push --full` は `pushed_session_versions` を無視してフルスキャンする（新メトリクス追加後の遡及送信などに使用）

### 送信タイミング — 独立コマンド `agent-telemetry push --since-last`

Stop hook 経路には載せない:

- Stop hook は既に `backfill` + `sync-db` を同期実行しており、ネット I/O を追加すると latency 影響が拡大する
- 送信失敗が Stop hook の挙動に直接影響すると、デバッグ困難な fail mode を生む
- 過去に launchd を撤廃した経緯（[issues/closed/0020-design-backfill-evolution-to-stop-hook.md](../issues/closed/0020-design-backfill-evolution-to-stop-hook.md)）は「Claude Code 外の唯一の手作業を残すと UX が悪化する」が、サーバ送信は **オプトイン**。サーバを使わないユーザに新たな手作業を要求しない

ユーザは以下のいずれかで起動する:

- macOS launchd / Linux systemd timer / cron で定期実行（5〜30 分間隔）
- 手動実行（必要なときだけ）

`docs/setup.md` に launchd plist と systemd timer のサンプルを置く。

### 認証 — 単一 API key

サーバ起動時に `AGENT_TELEMETRY_SERVER_TOKEN` 環境変数で API key を渡す。クライアントは `~/.claude/agent-telemetry.toml` の `[server]` セクションで同値を持つ。

```toml
[server]
endpoint = "https://telemetry.example.com"
token = "xxx"
```

`user_id`（人物識別）は payload の `sessions` 行に含まれる。**API key の認証**（信頼境界）と **`user_id` 経路**（集計軸）は責務を分ける。チーム内信頼を前提とし、人物単位の write 制限や OIDC は需要が出てから追加する。

### サーバ側 — dumb ingest API

新設するパッケージ:

```
cmd/
  agent-telemetry-server/main.go     # HTTP server エントリポイント
internal/
  serverpipe/                        # ingest ハンドラ（受信 → schema_hash 検証 → INSERT OR REPLACE）
```

サーバ側のデータ配置:

```
<server_data_dir>/
  agent-telemetry.db                 # 全 user 集約 SQLite
  collisions.log                     # session_id 衝突ログ
```

ingest ハンドラの責務:

1. Bearer token を検証
2. `schema_hash` をサーバ DB の `schema_meta` と比較。不一致なら `schema_mismatch: true` を返して受信拒否
3. 受信した `sessions` / `transcript_stats` 行を `(session_id, coding_agent)` PK で `INSERT OR REPLACE`
4. レスポンスとして受信件数 / スキップ件数 / `schema_mismatch` を返す

`internal/syncdb/schema.sql` をサーバ binary にも埋め込み、起動時に `schema_meta` ハッシュ比較で DDL 再構築する仕組みはクライアントと同じ。**集計ロジック（transcript パース等）はサーバ側に存在しない**。

サーバの SQLite は Grafana datasource として読み込まれる。datasource の `uid: agent-telemetry` を踏襲することで、ローカル Grafana のダッシュボード JSON をそのまま再利用できる。

### スキーマバージョン整合性と新メトリクス追加

クライアントとサーバで `internal/syncdb/schema_hash.go` の埋め込み値が一致している必要がある。新メトリクスを追加した場合の遡及反映手順:

1. サーバ binary を新スキーマでデプロイ（起動時に DDL 自動再構築）
2. 全クライアント binary を新バージョンに更新
3. 各クライアントで `agent-telemetry sync-db --recheck && agent-telemetry push --full` を実行（過去全セッションを新スキーマで再集計し再送信）

クライアントが古いまま push すると、サーバが `schema_mismatch: true` を返して受信拒否する。**サーバはクライアントよりも先に新スキーマに上げる必要がある**（クライアント先行で push されると古いスキーマで永続化される懸念があるため）。

### 衝突セッションの扱い

複数マシンの同一ユーザで session_id が衝突する確率は UUID として実用上ゼロ。物理コピーされた DB を別マシンから再 push したケースだけが実際の衝突源になる。サーバは `(session_id, coding_agent)` PK で `INSERT OR REPLACE` する（最後に届いたものが勝つ）。衝突検出時は `<server_data>/collisions.log` に記録する。

### 配布形態 — Go binary + Docker image

| 形態 | 提供物 | 想定 |
|---|---|---|
| Go binary | `cmd/agent-telemetry-server/`（goreleaser で配布） | Linux VPS で systemd unit 経由起動。`contrib/systemd/agent-telemetry-server.service` を同梱 |
| Docker image | `Dockerfile.server` + `docker-compose.server.yml` | docker-compose 1 本で server + Grafana + Image Renderer を立ち上げ。ローカル Grafana 構成と流儀を揃える |

両者は同一 Go binary をビルドするため、メンテナンスコストはほぼゼロ。

### 送信量とストレージ

| ケース | サイズ（集計値のみ、無圧縮） |
|---|---|
| 個人 1 日（10〜30 セッション × 1〜2 KB） | 10〜60 KB |
| 個人 1 ヶ月 | 300 KB〜1.8 MB |
| 5 人チーム 1 ヶ月 | 1.5〜9 MB |

集計値だけなのでネットワーク・ストレージともに極小。GC は実用上不要（数年分でも数百 MB 規模）。

---

## 既知の制約

- `Stop` hook はブロッキング実行のため、極端に遅い `gh pr list` を含む場合は応答完了の直後に最大数秒の待機が発生する
- `pr_urls` の順序保持と「最後の 1 件」採用の整合性は早期 pin（`pr_pinned`）で根本対処済みだが、未 pin セッションへの追記順序は検証ケースが限定的なため、引き続きデータの偏りがあれば再評価する
- `session-index.jsonl` の書き戻しは atomic 化未対応。書き込み中の停電やプロセスキルで truncate される可能性が残る
- transcript のパス取得失敗時は当該セッションの `transcript_stats` が空になるが、`sessions` 行は残る
- Codex の SessionEnd 不在を Stop hook で代替するため、Stop hook を経由せずプロセスが kill された場合は最後の Stop 発火時刻が `ended_at` になる（rollout JSONL 最終 event での補正は backfill 経由）
- Codex の `ask_user_question` 相当指標が無いため、agent を跨いだ「仕様不明瞭さ」比較はできない
- サーバ送信を有効化する場合、`agent-telemetry push --since-last` の定期起動を cron / launchd / systemd timer で自前運用する必要がある（Stop hook hot path に乗せないことの代償）
- 進行中セッション（`ended_at` 空）はサーバ送信対象外。最後の Stop 発火後にしか push されない
- クライアントとサーバで `internal/syncdb/` のスキーマハッシュが一致している必要がある。新メトリクス追加時はサーバを先にデプロイし、全クライアントを更新後に `push --full` で遡及反映する運用が必要
- サーバ認証は単一 API key。複数ユーザでの read/write 権限分離（user 別 RLS、OIDC 等）は将来課題
