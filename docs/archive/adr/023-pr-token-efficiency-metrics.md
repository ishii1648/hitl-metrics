# ADR-023: PR 単位のトークン消費効率メトリクスを導入する

## ステータス

採用済み

## 関連 ADR

- 依存: ADR-018（merged PR スコープと `pr_metrics` VIEW の構造）
- 関連: ADR-020（`transcript_stats` へのトークン使用量カラム追加案）
- 関連: ADR-022（ダッシュボードのアクショナビリティ改善）

## コンテキスト

hitl-metrics はこれまで Claude Code の「人の介入率」を中心に、PR 単位の作業を振り返るための指標を整備してきた。
現在の主指標は `perm_rate = perm_count / tool_use_total` であり、permission UI の発生率を下げることを改善対象としている。

しかし Claude Code の auto mode が進化すると、permission UI の発生頻度は早晩、継続的に計測・改善するほどの課題ではなくなる可能性が高い。
allowlist 改善のために permission 発生率を追うよりも、PR を完了するためにどれだけの計算資源を使ったかを追うほうが、今後の改善対象として安定している。

一方で、Claude Code の利用が増えるほど、次の問いが重要になる。

- 1 PR を完了するためにどれだけのトークンを消費しているか
- トークン消費が大きい PR は、タスクの大きさ・手戻り・文脈肥大のどれが原因か
- 同じ merged PR 数に対して、週ごとのトークン効率は改善しているか
- cache read/write が支配的な場合、単純な input/output token だけで評価していないか

ADR-020 では `transcript_stats` に `input_tokens` / `output_tokens` / cache token を保存する案を検討したが、当時は API コスト試算が主目的であり、HITL 計測への直接的貢献が弱いため凍結した。

今回の目的はコスト試算ではなく、既存の merged PR スコープに合わせて「PR 単位のトークン消費効率」を測定することにある。
そのため ADR-020 の収集方針を復活させ、`pr_metrics` の集約指標として扱う。

また、permission wait の計測は導入しない。
待機時間はユーザーの離席・端末状態・通知確認タイミングに強く影響され、PR の成果や Claude の作業効率と結びつけにくい。
代わりに、transcript に記録される usage を一次データとして、再現性の高いトークン消費指標を採用する。

同じ理由で、`perm_rate` も今後のメトリクス体系から削除する。
permission UI は過渡的な運用課題として扱い、PR 単位の評価軸には含めない。

## 決定

### A. `transcript_stats` にトークン使用量を保存する

`sync-db` 時に transcript JSONL の `assistant` メッセージから `usage` を読み取り、セッション単位で合計する。

追加カラム:

| カラム名 | 型 | 意味 |
|---|---|---|
| `input_tokens` | INTEGER | `usage.input_tokens` の合計 |
| `output_tokens` | INTEGER | `usage.output_tokens` の合計 |
| `cache_write_tokens` | INTEGER | `usage.cache_creation_input_tokens` の合計 |
| `cache_read_tokens` | INTEGER | `usage.cache_read_input_tokens` の合計 |
| `model` | TEXT | セッション内で最後に観測した model |

`model` は同一セッション内で複数モデルが混在しうるが、まずは最後に観測した値を保存する。
モデル別の厳密な集計が必要になった場合は、将来 `model_usage_stats` のような別テーブルで分離する。

### B. `pr_metrics` に PR 単位のトークン集約を追加する

既存の `pr_metrics` VIEW は merged PR、非 subagent、非 ghost、対象 repo のみを扱う。
トークン指標も同じフィルタ条件に従う。

追加集約カラム:

| カラム名 | 意味 |
|---|---|
| `input_tokens` | PR に紐づくセッションの input token 合計 |
| `output_tokens` | PR に紐づくセッションの output token 合計 |
| `cache_write_tokens` | PR に紐づくセッションの cache creation token 合計 |
| `cache_read_tokens` | PR に紐づくセッションの cache read token 合計 |
| `total_tokens` | 上記 4 種の合計 |
| `tokens_per_session` | `total_tokens / session_count` |
| `tokens_per_tool_use` | `total_tokens / tool_use_total` |
| `pr_per_million_tokens` | `1000000 / total_tokens` |

`total_tokens` は PR 単位の消費量を表す主指標とする。
`tokens_per_tool_use` は文脈肥大や一回あたりの重さを検出する補助指標とする。
`pr_per_million_tokens` は週次やタスク種別の効率比較に使う。

### C. `perm_rate` をメトリクス体系から削除する

`perm_rate`、`perm_count`、permission tool breakdown は、PR 単位の評価指標から削除する。
Claude Code の auto mode により permission UI の発生は構造的に減る見込みであり、今後の改善対象として長期的な価値が低い。

`permission_events` テーブルは新スキーマから削除し、既存 DB に残っている場合も `sync-db` の再構築時に drop する。
また、`hitl-metrics install` は PermissionRequest / PreToolUse hook を新規登録しない。

既存ユーザーの `settings.json` から過去に登録された hook を自動削除する移行は行わない。
ただし、少なくとも以下からは除外する。

- `pr_metrics` VIEW
- Grafana のヘッドライン
- Grafana のトレンド
- PR 別スコアカード
- README の主要指標説明
- architecture の主要メトリクス定義
- `hitl-metrics install` の新規 hook 登録

### D. Grafana ダッシュボードをトークン効率中心に再構成する

ヘッドライン:

- merged PR 数
- total tokens
- avg tokens / PR
- PR / 1M tokens
- cache read/write tokens
- changes requested

トレンド:

- 週別 total tokens
- 週別 merged PR 数
- 週別 PR / 1M tokens
- 週別 avg tokens / PR

PR 詳細:

- `total_tokens DESC` の PR 別テーブル
- `tokens_per_tool_use` の高い PR
- `cache_read_tokens` / `cache_write_tokens` の内訳
- 既存の `mid_session_msgs`, `ask_user_question`, `session_count`, `changes_requested` を併記し、消費増加の原因を推測できるようにする

## 実装方針

### transcript parser

`internal/transcript.Parse()` の戻り値 `Stats` に token usage フィールドを追加する。

想定する transcript の assistant entry:

```json
{
  "type": "assistant",
  "message": {
    "model": "claude-sonnet-4-5-20250929",
    "usage": {
      "input_tokens": 1000,
      "output_tokens": 200,
      "cache_creation_input_tokens": 500,
      "cache_read_input_tokens": 3000
    }
  }
}
```

`usage` が存在しない古い transcript では 0 として扱う。
これにより既存データや fixture を壊さず段階導入できる。

### SQLite schema

`transcript_stats` の `INSERT` と `CREATE TABLE` を拡張する。
DB は `sync-db` で毎回再構築されるため、マイグレーションは不要。

`pr_metrics` では、既存の LEFT JOIN 膨張バグ対策と同じく、`transcript_stats` は session と 1:1 のまま集約する。
`permission_events` との JOIN は不要になる。

### ダッシュボード

既存パネル名のうち `perm rate` を主語にしたものは、トークン効率のパネルへ置き換える。
permission 関連のテーブル・バーは削除する。

## 変更が必要なファイル（affected-scope）

| ファイル / パッケージ | 変更内容 |
|---|---|
| `internal/transcript/transcript.go` | `usage` と `model` のパース、`Stats` への token フィールド追加 |
| `internal/transcript/transcript_test.go` | token usage 集計、usage 欠落時の後方互換テスト |
| `internal/syncdb/schema.go` | `transcript_stats` と `pr_metrics` に token 関連カラム追加 |
| `internal/syncdb/syncdb.go` | `transcript_stats` INSERT の拡張、permission log 取り込み削除 |
| `internal/syncdb/syncdb_test.go` | PR 単位の token 集約テスト追加 |
| `internal/install/install.go` | PermissionRequest / PreToolUse hook の新規登録を削除 |
| `grafana/dashboards/hitl-metrics.json` | `perm_rate` / permission breakdown パネルを削除し、ヘッドライン・トレンド・PR 詳細を token 効率中心に再構成 |
| `e2e/testdata/transcripts/*.jsonl` | 代表 fixture に `usage` を追加 |
| `e2e/testdata/hitl-metrics.db` | fixture DB を再生成 |
| `README.md` | 主説明を「人の介入率」から「PR 単位のトークン消費効率」へ更新し、`perm_rate` を主要指標から削除 |
| `docs/architecture.md` | `transcript_stats` / `pr_metrics` / Grafana パネル構成を更新 |

## 受け入れ条件

- [x] `sync-db` 後、`transcript_stats` に token usage が保存される
- [x] `usage` が存在しない transcript でも `sync-db` が失敗しない
- [x] `pr_metrics` に `total_tokens`, `tokens_per_session`, `tokens_per_tool_use`, `pr_per_million_tokens` が表示される
- [x] merged PR・非 subagent・非 ghost・対象 repo のフィルタ条件が token 指標にも適用される
- [x] Grafana のヘッドラインが token 効率中心になる
- [x] Grafana から `perm_rate` と permission breakdown パネルが削除される
- [x] `pr_metrics` から `perm_rate` と `perm_count` が削除される
- [x] `permission_events` テーブルが新スキーマから削除される
- [x] `hitl-metrics install` が PermissionRequest / PreToolUse hook を新規登録しない
- [x] README と architecture が新しい主指標に更新される

## 影響

- `sync-db` は DB を再構築するため、既存 DB へのマイグレーションは不要
- 古い transcript は token usage が 0 になり、PR 効率指標には反映されない
- token usage が記録されている期間と記録されていない期間を同じグラフで比較すると誤解を生むため、Grafana では `total_tokens > 0` の条件を使う
- `perm_rate` に依存していた過去のダッシュボード比較はできなくなる
- 既存ユーザーの `~/.claude/settings.json` に登録済みの PermissionRequest / PreToolUse hook は自動削除されない
