# agent-telemetry

Claude Code および Codex CLI を使った開発で、**PR 単位のトークン消費効率**を追跡・可視化する計測ツール。

merged PR ごとに各 agent が消費した input / output / cache / reasoning token を集計し、どの PR が重く、どの agent / タスク種別で効率が落ちているかを特定します。

![Claude token 効率ダッシュボード](docs/images/dashboard-full.png)

ダッシュボードは3つのセクションで構成されています。上から順に読むだけで「どれだけ使ったか → 改善しているか → どの PR が重いか」がわかります。

---

### 1. ヘッドライン — 今の消費効率を一目で

merged PR 数、total tokens、平均 tokens / PR、PR / 1M tokens、changes requested 数を表示します。平均 tokens / PR が下がり、PR / 1M tokens が上がっていれば、同じ計算資源でより多くの PR を完了できています。

### 2. トレンド — 改善しているか

週ごとの token 消費、merged PR 数、PR / 1M tokens を表示します。横に並ぶタスク種別バーで feat/fix/docs/chore ごとの token 消費傾向も確認できます。

### 3. PR 詳細 — どこが重いか

![PR スコアカード](docs/images/dashboard-pr-scorecard.png)

各 PR の token 指標を total_tokens の高い順に表示します。tokens_per_tool_use が高ければ文脈肥大、session_count や mid_session_msgs が多ければタスク分割や要件伝達に改善余地があります。

---

## 計測する指標

| 指標 | 意味 |
|------|------|
| **total_tokens** | PR に紐づく input / output / cache write / cache read token の合計 |
| **tokens_per_session** | 1 セッションあたりの token 消費量 |
| **tokens_per_tool_use** | 1 tool_use あたりの token 消費量。文脈肥大の代理指標 |
| **pr_per_million_tokens** | 100万 token あたりに完了できた PR 数。高いほど効率的 |
| **mid_session_msgs** | ユーザーが途中で方向転換した回数。要件の曖昧さの代理指標 |
| **ask_user_question** | Claude がユーザーに質問した回数。仕様不明瞭さの指標 |
| **session_count** | PR に紐づくセッション数。作業の完了困難さの指標 |
| **peak_concurrent_sessions** | 期間内のトップレベル Claude Code セッション最大同時実行数 |
| **avg_concurrent_sessions** | 期間内のトップレベル Claude Code セッション平均同時実行数 |
| **review_comments** | PR レビューコメント数。成果物品質の外部フィードバック |
| **changes_requested** | CHANGES_REQUESTED レビュー回数 |
| **task_type** | ブランチプレフィックス (feat/fix/docs/chore) から自動抽出 |

## アーキテクチャ

```
Claude Code hooks → ~/.claude/session-index.jsonl + transcript JSONL ┐
                                                                     ├→ agent-telemetry backfill / sync-db
Codex CLI hooks   → ~/.codex/session-index.jsonl  + rollout JSONL    ┘
                                                                     → ~/.claude/agent-telemetry.db (SQLite)
                                                                     → Grafana
```

1. **データ収集層** (`internal/hook/`) — 各 agent の hook で session イベントを記録（`internal/agent/` で agent 差分を吸収）
2. **データ変換層** (`cmd/agent-telemetry/`, `internal/syncdb/`, `internal/transcript/`) — Go CLI で JSONL/transcript → SQLite 変換・PR URL 補完
3. **可視化層** (`grafana/`) — Grafana ダッシュボードで PR 単位の token 効率を agent 別に表示

## ドキュメント

| ファイル | 内容 |
|---|---|
| [docs/spec.md](docs/spec.md) | 外部契約（CLI・hook 仕様・データモデル） |
| [docs/metrics.md](docs/metrics.md) | 計測フレームワーク（観察軸・解釈・OpenMetrics カタログ） |
| [docs/design.md](docs/design.md) | 実装方針と設計判断 |
| [docs/history.md](docs/history.md) | 過去の経緯と廃止された設計 |
| [docs/setup.md](docs/setup.md) | セットアップ手順 |
| [docs/usage.md](docs/usage.md) | 日常運用とトラブルシューティング |
| [docs/archive/adr/](docs/archive/adr/) | 過去の意思決定記録（旧 ADR 形式・参照のみ） |
