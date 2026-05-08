---
name: analyze-pr
description: agent-telemetry の PR スコアカードで外れ値となった PR の transcript を読み、token 消費の外れ値要因と改善仮説を Markdown で stdout に出力する。「/analyze-pr <pr_url>」または「/analyze-pr --worst-by <column> --limit <n>」で起動する。
argument-hint: "<pr_url> | --worst-by <column> --limit <n>"
version: 0.1.0
---

# analyze-pr

agent-telemetry が記録した PR ごとの token 消費を分析する skill のサンプル実装。

agent-telemetry 本体の責務は外れ値 PR を「示唆」するところまで。本 skill はその先の「なぜ token を食ったのか / 次回どう減らせるか」の仮説生成を担う。出力先（GitHub Issue / PR コメント / Slack 投稿等）は呼び出し側の責務とし、本 skill は **stdout に Markdown を吐くだけ** に責務を絞る。

## 引数フォーマット

```
/analyze-pr <pr_url>
/analyze-pr --worst-by <pr_metrics_column> --limit <n>
```

- 単発モード: 指定 PR の全セッションを対象に分析する
- 一括モード: `pr_metrics` VIEW を `<column>` でソートして上位 `<n>` 件を分析する

`<column>` に許容する値は `pr_metrics` VIEW のカラム名のうち以下:

| カラム | 意味 | ソート方向 |
|---|---|---|
| `fresh_tokens` | cache_read を除いた合計（推奨。長時間セッションで cache_read 支配を避ける） | 降順 |
| `total_tokens` | 全 token 合計 | 降順 |
| `tokens_per_session` | 平均セッションサイズ | 降順 |
| `tokens_per_tool_use` | tool 1 回あたりの token 消費 | 降順 |
| `pr_per_million_tokens` | 100 万 token あたりに完了した PR 数 | **昇順** |

それ以外のカラムを渡された場合は使用方法を表示して終了する。

## ステップ

### Step 1: 引数の解析

1. 第 1 引数が `--worst-by` で始まる場合は一括モード。`<column>` を取得し、続く `--limit <n>` を読む（`--limit` 未指定時は既定 5）
2. 第 1 引数が `https://` で始まる場合は単発モード
3. どちらにも該当しない場合は使用方法を表示して終了

### Step 2: DB の特定

DB のパスは `${AGENT_TELEMETRY_DB:-$HOME/.claude/agent-telemetry.db}` を使う。ファイルが存在しない場合は「`agent-telemetry sync-db` を先に実行してください」と表示して終了する。

### Step 3: 対象 PR の特定（一括モードのみ）

**Bash ツール**で `sqlite3` を実行して対象 PR を取得する:

```sh
sqlite3 -separator $'\t' "$DB" <<SQL
SELECT pr_url, pr_title, coding_agent, ${COLUMN} AS metric
  FROM pr_metrics
 ORDER BY metric ${DIRECTION}
 LIMIT ${LIMIT};
SQL
```

`${DIRECTION}` は上表に従う（`pr_per_million_tokens` のみ `ASC`、それ以外は `DESC`）。

### Step 4: 各 PR のセッション一覧を取得

PR 単位で次のクエリを実行する:

```sh
sqlite3 -separator $'\t' "$DB" <<SQL
SELECT s.session_id, s.coding_agent, s.timestamp, s.transcript, s.pr_title,
       ts.input_tokens, ts.output_tokens, ts.cache_write_tokens, ts.cache_read_tokens, ts.reasoning_tokens,
       ts.tool_use_total, ts.mid_session_msgs, ts.model
  FROM sessions s
  JOIN transcript_stats ts USING (session_id, coding_agent)
 WHERE s.pr_url = '${PR_URL}'
   AND s.is_subagent = 0
   AND ts.is_ghost = 0
 ORDER BY s.timestamp;
SQL
```

### Step 5: transcript を読む

セッション 1 件ごとに `transcript` パスを開く。

- Claude transcript (`~/.claude/projects/**/<session_id>.jsonl`): assistant message の `usage.*` と `message.content[*].type == "tool_use"` を走査する
- Codex transcript (`~/.codex/sessions/.../rollout-*.jsonl[.zst]`): 拡張子 `.zst` なら `zstd -dc` で透過解凍する。`event_msg.payload.type == "token_count"` の累積遷移と `tool_use` イベントを走査する

ファイルが存在しない / 形式が壊れているケースは「transcript 不在」としてスキップ理由を出力に明記する（強制終了しない）。

### Step 6: 外れ値要因の特定と改善仮説の生成

セッション単位で次のような兆候を抽出する。**断定はしない**。「〜の傾向がある」「〜を試す価値がある」レベルにとどめる。

| 兆候 | 観察ロジック | 仮説 |
|---|---|---|
| 長文コンテキスト × 思考停滞 | `cache_read_tokens` が `fresh_tokens` を大きく上回るのに `tool_use_total` が小さい | スコープが広すぎ。対象範囲を絞ってから着手する余地 |
| 思考の空回り | `reasoning_tokens` が `output_tokens` の数倍（Codex のみ） | reasoning 効果が頭打ち。中間でユーザ確認を挟む価値 |
| 探索が広く浅い | 同一ツール（Glob / Grep / Read）の連続呼び出しが多い | 仮説立てが弱い。Plan mode / 設計セッションを先行させる余地 |
| ユーザ介入過多 | `mid_session_msgs` が多い | 計画段階で要件が固まっていない |
| 文脈ロスト | transcript 後半で同じファイルを何度も Read / Edit | コンテキストが流れた。チャンク分割や設計メモの先行作成 |

### Step 7: Markdown を stdout に出力

```markdown
# analyze-pr report

## <PR タイトル> — <pr_url>

| 指標 | 値 |
|---|---|
| coding_agent | claude |
| sessions | 3 |
| total_tokens | 1,234,567 |
| fresh_tokens | 456,789 |
| tokens_per_tool_use | 13,872 |
| tool_use_total | 89 |

### 観察

- ...

### 改善仮説

- ...

### セッション内訳

| session_id (短縮) | tokens | tool_use | 主な兆候 |
|---|---|---|---|
| abc12345 | 512,345 | 34 | 長文コンテキスト × 思考停滞 |
```

一括モードでは PR 単位のセクションを `<limit>` 個並べる。各 PR の見出しに対象指標とソート順位（例: `#1 by fresh_tokens`）を併記する。

## 注意

- 本 skill の出力は **仮説**。自動修正ではない。呼び出し側で人間が判断するか、別 skill にハンドオフする前提
- transcript には機密情報（コード断片・URL・コメント等）が含まれる可能性がある。stdout を外部送信（Issue / Slack / PR コメント等）する前に **必ずスコープを確認** すること
- 本 skill は `examples/` 配下の **best-effort なリファレンス実装**。CI で検証されておらず、実 DB スキーマの変更に追随しないことがある
