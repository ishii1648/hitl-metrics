# agent-telemetry 計測フレームワーク

この文書は agent-telemetry が **何を観察するために** どのメトリクスを収集しているかを整理する。
メトリクス名・型・ラベルの一覧は本文書末尾「メトリクスカタログ」を、SQLite テーブル/VIEW のカラム定義は `docs/spec.md ## SQLite データモデル` を、実装は `docs/design.md` を参照する。

---

## 計測の前提

agent-telemetry は **HITL 型コーディングエージェント（Claude Code / Codex CLI）の効率を PR 単位で観察する** ためのツールである。
"効率" は単一指標では測れないが、本ツールは観察軸を **トークン効率** と **開発生産性** の 2 軸に絞り込む。横断軸として **エージェント間比較** と **モデル / バージョン跨ぎ比較** を併用する。`pr_metrics` VIEW のフィルタ（merged のみ・subagent / ghost / dotfiles 除外）はすべての軸の前提として効く。

| 軸 | 答えたい疑問 |
|---|---|
| 1. トークン効率 | 1 PR を完了するのに何 token かかっているか |
| 2. 開発生産性 | エージェントが詰まらず PR をマージまで到達させられているか |
| 横断 A. エージェント間比較 | claude vs codex の差はどこに出るか |
| 横断 B. モデル / バージョン跨ぎ | バージョンアップで効率がどう変わったか |

---

## 1. トークン効率

**疑問**: 1 PR を完了するのに何 token かかっているか。token あたり何 PR を出せているか。

**主要指標**:
- `agent_pr_total_tokens` — PR 内の全セッション合計トークン（input / output / cache_write / cache_read / reasoning の和）
- `agent_pr_fresh_tokens` — `cache_read` を除いた合計（input / output / cache_write / reasoning）。長時間セッションで cache_read が支配的になると total_tokens が「重さ」の体感と乖離するため、代替の効率指標として併用する
- `agent_pr_per_million_tokens` — 100 万 token あたりに完了した PR 数。効率の逆数指標として最も読みやすい
- `agent_pr_tokens_per_session` — PR 内の平均セッションサイズ。セッションを分割して進めるかどうかの傾向

**補助指標**:
- `agent_pr_tokens_per_tool_use` — ツール 1 回あたりのトークン消費。**単独で良し悪しを評価せず**、token 効率の傾向理解と異常検出に使う。例: 高 `reasoning_tokens` × 低 `tool_use_total` のセッションは「思考の空回り」の兆候
- `agent_pr_tool_use_total` — PR 内の全ツール呼び出し数。`tokens_per_tool_use` の分母

**内訳指標**:
- `agent_pr_input_tokens_total` / `agent_pr_output_tokens_total` / `agent_pr_cache_write_tokens_total` / `agent_pr_cache_read_tokens_total` / `agent_pr_reasoning_tokens_total`
- 単体では軸として扱わないが、total / fresh の異常時に内訳に降りて原因分解する用途

**解釈の注意**:
- 集計対象は `is_merged = 1` の PR のみ。未マージ PR は ROI 不明のため除外している（`pr_metrics` VIEW のフィルタ）
- リファクタ系 PR（差分は小さいが議論が長い）と feature 系 PR は素直に比較できないことがある。`task_type` ラベルで絞り込むか、PR 別スコアカードで個別に見る
- `model` 跨ぎだとキャッシュ動作・出力傾向が違うため、モデル混在の単純平均は誤読しやすい
- total / fresh どちらで見るかは目的次第。「課金や物理 token 量」を見たいなら total、「実質的な作業量」を見たいなら fresh
- `cache_read_tokens` が大きい = 効率が良いとは限らない。長大なコンテキストで自然と増える側面があるため、`fresh_tokens` を主軸にする運用が安全
- `cache_write_tokens` が異常に大きい = キャッシュヒットしておらず毎回書き直している兆候。プロンプト構造の安定性を疑う材料として使う

---

## 2. 開発生産性

**疑問**: エージェントが正しい道筋を一発で見つけられているか。レビューでどれくらい差し戻されているか。同時に何セッション捌けているか。

**主要指標**:
- `agent_session_mid_session_msgs_total` — セッション中のユーザ追加メッセージ数（道筋修正の頻度）
- `agent_session_ask_user_question_total` — 仕様確認ツールの発火回数（Claude のみ、Codex は 0 固定）
- `agent_pr_review_comments` — PR レビューコメント数
- `agent_pr_changes_requested` — CHANGES_REQUESTED レビュー回数
- `agent_concurrent_sessions_avg` / `agent_concurrent_sessions_peak` — 日次・週次の同時実行数。ユーザのマルチタスク上限を測る

**解釈の注意**:
- `mid_session_msgs` が高い = **エージェントが正しい道筋を見つけられない**、もしくはユーザが auto モードを信用しきれていない
- `ask_user_question` は agent 非対称（Codex に相当ツールがない）。agent 跨ぎ比較に使ってはいけない
- `changes_requested` は人間レビュアの厳しさ・PR 規模に依存する。同一レビュア・同一規模帯での時系列比較が安全
- 同時実行数は **手元のマシン能力 × 自分のマルチタスク能力** の上限を測る。peak が高い時期にトークン効率が落ちていれば「並列やりすぎ」のサイン
- `ended_at` が空のセッションは現在時刻で打ち切る扱いのため、進行中セッションの含まれる時間帯は同時実行数が膨らむ
- ghost / subagent は `pr_metrics` から除外されるが、生のセッション数で見る場合はフィルタを意識する必要がある（`agent_session_is_ghost` / `agent_session_is_subagent` で分離）

---

## 横断 A. エージェント間比較

`coding_agent` ラベルで claude / codex を区別。

**比較できる指標**:
- token 系（input / output / cache_read / cache_write）— ただしキャッシュ意味論差に注意
- `tool_use_total` / `mid_session_msgs` / `review_comments` / `changes_requested`
- PR 効率系すべて（`total_tokens` / `fresh_tokens` / `per_million_tokens`）

**比較してはいけない指標**:
- `ask_user_question` — Codex に相当ツール無しで 0 固定（agent 非対称）
- `reasoning_tokens` — Claude は extended thinking を分離計上できず 0 固定（API 構造的制約）。Codex 側だけ reasoning が乗るため絶対値比較は誤読
- `total_tokens` の絶対値も上記の影響を受ける。agent 跨ぎでは `fresh_tokens` から `reasoning_tokens` を引いた値、もしくは内訳ごとの並列比較が安全

agent 跨ぎでは `model` / `agent_version` の混在で平均化が壊れやすい。ラベル絞り込みを必ず併用する。

---

## 横断 B. モデル / バージョン跨ぎ比較

`agent_version` は session レベルでのみ集計可能。`pr_metrics` には集約しない（1 PR 内でバージョンが混在しうるため平均が無意味になる）。

想定ユースケース: 「version A の全 session の token 効率」vs「version B の全 session」を session ベースのクエリで比較する。詳細は `docs/design.md ## agent_version の取得` を参照。

`model` ラベルは PR 単位でも保持されるが、PR 内でモデル切替が起きた場合は最後に観測した model が記録される点に注意。

---

## 観察しないこと

| 非目標 | 理由 |
|---|---|
| 個別 API 課金額 | モデル単価変動が大きく、token 量の方が安定指標 |
| permission UI 表示回数 | Claude Code の auto mode 進化で改善ターゲットとしての価値が低下 |
| 未マージ PR / PR 無しセッションの効率 | ROI 不明のため `pr_metrics` から除外 |
| `task_type` 軸の集計 | カテゴリ間で性質が違いすぎ、平均値が誤読されやすいため集計軸から廃止（`sessions.task_type` カラム自体は任意フィルタ用に残す） |
| キャッシュヒット率を単独軸として | 長文コンテキストでヒット率が機械的に上がる傾向があり、運用の良し悪しとの相関が弱い。`fresh_tokens` の構成要素としてのみ扱う |
| reasoning トークンの「思考量」を単独軸として | Codex のみ計上で agent 跨ぎ比較不能。Claude は API 構造的に分離計上できない。さらにユーザが介入できる項目ではなく運用判断に使えない |
| ツール利用パターンを単独軸として | `tokens_per_tool_use` 自体に良し悪しは無い。token 効率の補助指標として残し、独立軸では立てない |
| ghost / subagent セッションの効率 | 実体のないセッション。生のメトリクスは残すが PR 単位集計から除外 |

---

## メトリクス → 観察軸 逆引き表

| メトリクス | 主用途軸 |
|---|---|
| `agent_pr_total_tokens` / `agent_pr_fresh_tokens` / `agent_pr_per_million_tokens` / `agent_pr_tokens_per_session` | 1. トークン効率 |
| `agent_pr_tool_use_total` / `agent_pr_tokens_per_tool_use` | 1. トークン効率（補助） |
| `agent_pr_input_tokens_total` / `agent_pr_output_tokens_total` / `agent_pr_cache_write_tokens_total` / `agent_pr_cache_read_tokens_total` / `agent_pr_reasoning_tokens_total` | 1. トークン効率（内訳） |
| `agent_session_input_tokens_total` / `agent_session_output_tokens_total` / `agent_session_cache_write_tokens_total` / `agent_session_cache_read_tokens_total` / `agent_session_reasoning_tokens_total` | 1. トークン効率（session 粒度の内訳） |
| `agent_session_mid_session_msgs_total` / `agent_session_ask_user_question_total` | 2. 開発生産性 |
| `agent_pr_review_comments` / `agent_pr_changes_requested` | 2. 開発生産性 |
| `agent_concurrent_sessions_avg` / `agent_concurrent_sessions_peak` | 2. 開発生産性（並列上限） |
| `agent_session_started_timestamp_seconds` / `agent_session_ended_timestamp_seconds` | 並列上限の区間計算 |
| `agent_session_is_subagent` / `agent_session_is_ghost` | フィルタ条件 |
| `agent_session_pr_merged` / `agent_session_pr_review_comments` / `agent_session_pr_changes_requested` | フィルタ / 2. 開発生産性の session 粒度 |

---

## メトリクスカタログ

メトリクス名・型・ラベルの一覧。各メトリクスは SQLite のカラム/VIEW と 1:1 で対応する（データモデルは `docs/spec.md ## SQLite データモデル` を参照）。型は時系列指標としての性格を表す（`counter` = 単調増加、`gauge` = 瞬時値）。Grafana が SQLite を直接 SQL で参照する想定で、外部配信プロトコルは備えない。

### セッション単位（`sessions` + `transcript_stats`）

すべてのセッション単位メトリクスは次の共通ラベル集合を持つ:

| ラベル | 値の例 |
|---|---|
| `coding_agent` | `claude` / `codex` |
| `session_id` | エージェント発行の UUID |
| `model` | セッション内で最後に観測した model（例 `claude-sonnet-4-6`） |
| `agent_version` | agent 自身が報告するバージョン（取得不能なら空） |
| `repo` | `org/repo` |
| `branch` | ブランチ名 |
| `pr_url` | PR URL（未作成なら空） |
| `task_type` | `feat` / `fix` / `docs` / `chore` / 空 |
| `parent_session_id` | 親セッション ID（top-level なら空） |
| `end_reason` | SessionEnd hook の終了理由（Codex は `stop` 固定） |

| メトリクス名 | 型 | 値 | 説明 |
|---|---|---|---|
| `agent_session_tool_use_total` | counter | int | セッション内のツール呼び出し回数 |
| `agent_session_mid_session_msgs_total` | counter | int | セッション中の人間追加メッセージ数（`tool_result` のみで構成されるエントリは除外） |
| `agent_session_ask_user_question_total` | counter | int | AskUserQuestion ツール発火回数（Codex は常に 0） |
| `agent_session_input_tokens_total` | counter | int | 入力トークン |
| `agent_session_output_tokens_total` | counter | int | 出力トークン |
| `agent_session_cache_write_tokens_total` | counter | int | プロンプトキャッシュへの書き込みトークン |
| `agent_session_cache_read_tokens_total` | counter | int | プロンプトキャッシュからの読み込みトークン |
| `agent_session_reasoning_tokens_total` | counter | int | reasoning トークン（Claude は常に 0。Anthropic API が thinking を `output_tokens` から分離しないため） |
| `agent_session_started_timestamp_seconds` | gauge | int | セッション開始時刻（Unix epoch） |
| `agent_session_ended_timestamp_seconds` | gauge | int | セッション終了時刻（Unix epoch、未設定なら 0） |
| `agent_session_is_subagent` | gauge | 0\|1 | `parent_session_id` が非空なら 1 |
| `agent_session_is_ghost` | gauge | 0\|1 | transcript に user 相当エントリが 0 件なら 1 |
| `agent_session_pr_merged` | gauge | 0\|1 | セッションの PR が merged なら 1 |
| `agent_session_pr_review_comments` | gauge | int | セッションの PR のレビューコメント数 |
| `agent_session_pr_changes_requested` | gauge | int | セッションの PR の CHANGES_REQUESTED レビュー数 |

### PR 単位の集約（`pr_metrics` VIEW）

`pr_url != ''` AND `is_subagent = 0` AND `is_merged = 1` AND `is_ghost = 0` AND `repo NOT IN ('ishii1648/dotfiles')` でフィルタした PR スコープの集約値。

ラベル: `pr_url`, `coding_agent`, `model`

| メトリクス名 | 型 | 値 | 説明 |
|---|---|---|---|
| `agent_pr_session_count` | gauge | int | PR に寄与したセッション数 |
| `agent_pr_tool_use_total` | counter | int | PR 内全セッションのツール呼び出し合計 |
| `agent_pr_mid_session_msgs_total` | counter | int | PR 内全セッションの mid_session_msgs 合計 |
| `agent_pr_ask_user_question_total` | counter | int | PR 内全セッションの AskUserQuestion 合計 |
| `agent_pr_input_tokens_total` | counter | int | PR 全セッションの input トークン合計 |
| `agent_pr_output_tokens_total` | counter | int | PR 全セッションの output トークン合計 |
| `agent_pr_cache_write_tokens_total` | counter | int | PR 全セッションの cache write トークン合計 |
| `agent_pr_cache_read_tokens_total` | counter | int | PR 全セッションの cache read トークン合計 |
| `agent_pr_reasoning_tokens_total` | counter | int | PR 全セッションの reasoning トークン合計 |
| `agent_pr_total_tokens` | counter | int | input + output + cache_write + cache_read + reasoning の合計 |
| `agent_pr_fresh_tokens` | counter | int | input + output + cache_write + reasoning（`cache_read` を除外、実質的な作業量に近い） |
| `agent_pr_review_comments` | gauge | int | PR レビューコメント数 |
| `agent_pr_changes_requested` | gauge | int | CHANGES_REQUESTED レビュー数 |
| `agent_pr_tokens_per_session` | gauge | float | PR 内の平均セッショントークン（派生） |
| `agent_pr_tokens_per_tool_use` | gauge | float | PR 内のツール 1 回あたりトークン（派生） |
| `agent_pr_per_million_tokens` | gauge | float | 100 万 token あたりに完了した PR 数（効率の逆数指標、派生） |

### 同時実行数（`session_concurrency_daily` / `session_concurrency_weekly` VIEW）

トップレベルセッション（subagent / ghost / dotfiles を除外）の同時実行数を時間軸で集約。

ラベル: `coding_agent`, `bucket`（`day` または `week`）, `bucket_start`（ISO8601 日付）

| メトリクス名 | 型 | 値 | 説明 |
|---|---|---|---|
| `agent_concurrent_sessions_avg` | gauge | float | bucket 内の平均同時実行数 |
| `agent_concurrent_sessions_peak` | gauge | int | bucket 内のピーク同時実行数 |
