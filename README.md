# hitl-metrics

Claude Code の**人の介入率**を追跡・可視化する計測ツール。

Claude Code を使った開発で「どれだけ自律的に作業できているか」を PR 単位で計測し、改善ポイントを特定します。

![Claude 自律度ダッシュボード](docs/images/dashboard-full.png)

ダッシュボードは4つのセクションで構成されています。上から順に読むだけで「今どうなっているか → 改善しているか → どこが問題か → 何をすべきか」がわかります。

---

### 1. ヘッドライン — 今の状況を一目で

![ヘッドライン KPI](docs/images/dashboard-headline.png)

merged PR 数、平均 perm rate、mid-session メッセージ数、レビューコメント数の4指標。**perm rate が低いほど Claude が自律的に作業できています**。赤は 15% 以上（改善が必要）、緑は 5% 未満（良好）。

### 2. トレンド — 改善しているか

![週別トレンド](docs/images/dashboard-weekly-trend.png)

週ごとの perm rate 推移。右肩下がりなら自律度が改善しています。横に並ぶタスク種別バーで feat/fix/docs/chore ごとの傾向も確認できます。

### 3. PR 詳細 — どこが問題か

![PR スコアカード](docs/images/dashboard-pr-scorecard.png)

各 PR の介入指標を perm rate の高い順に表示。セル色分け（赤/黄/緑）で問題 PR が一目でわかります。mid_session_msgs が多ければ要件の伝え方、session_count が多ければタスク分割に改善余地があります。

### 4. アクション — 何をすべきか

ツール別の permission 内訳を表示。**上位ツールを allowlist に追加すれば perm rate を直接改善**できます。例えば Bash が 40% を占めていれば、`Bash(git:*)` を許可するだけで perm rate が 4 割改善します。

---

## 計測する指標

| 指標 | 意味 |
|------|------|
| **perm_rate** | Permission UI 発生率 (perm_count / tool_use_total)。低いほど自律的 |
| **mid_session_msgs** | ユーザーが途中で方向転換した回数。要件の曖昧さの代理指標 |
| **ask_user_question** | Claude がユーザーに質問した回数。仕様不明瞭さの指標 |
| **session_count** | PR に紐づくセッション数。作業の完了困難さの指標 |
| **review_comments** | PR レビューコメント数。成果物品質の外部フィードバック |
| **task_type** | ブランチプレフィックス (feat/fix/docs/chore) から自動抽出 |

## アーキテクチャ

```
Claude Code hooks → ~/.claude/*.jsonl|log → hitl-metrics sync-db → SQLite → Grafana
```

1. **データ収集層** (`hooks/`) — Claude Code hook で session/permission/tool-use イベントを記録
2. **データ変換層** (`cmd/hitl-metrics/`, `internal/`) — Go CLI で JSONL/log → SQLite 変換・PR URL 補完
3. **可視化層** (`grafana/`) — Grafana ダッシュボードで介入率・ツール分布を表示

## セットアップ

→ [docs/setup.md](docs/setup.md)
