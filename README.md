# claudedog

Claude Code の**人の介入率**を追跡・可視化する計測ツール。

Claude Code を使った開発で「どれだけ自律的に作業できているか」を PR 単位で計測し、改善ポイントを特定します。

![ヘッドライン KPI](docs/images/dashboard-headline.png)

## 何がわかるか

**ヘッドライン** — merged PR 数、平均 perm rate（許可を求めた頻度）、mid-session メッセージ数、レビューコメント数を一目で把握。perm rate が低いほど Claude が自律的に作業できています。

**トレンド** — 週ごとの perm rate 推移で改善・悪化を追跡。

![週別トレンド](docs/images/dashboard-weekly-trend.png)

**PR 別スコアカード** — 各 PR の介入指標を一覧表示。perm rate のセル色分け（緑 < 5% < 黄 < 15% < 赤）で問題 PR が一目でわかります。

![PR スコアカード](docs/images/dashboard-pr-scorecard.png)

**アクション** — どのツール（Bash, Edit, Write...）が最も許可を求めているかを可視化。上位ツールを allowlist に追加すれば perm rate を直接改善できます。

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
Claude Code hooks → ~/.claude/*.jsonl|log → claudedog sync-db → SQLite → Grafana
```

1. **データ収集層** (`hooks/`) — Claude Code hook で session/permission/tool-use イベントを記録
2. **データ変換層** (`cmd/claudedog/`, `internal/`) — Go CLI で JSONL/log → SQLite 変換・PR URL 補完
3. **可視化層** (`grafana/`) — Grafana ダッシュボードで介入率・ツール分布を表示

## CLI コマンド

```
claudedog update <session_id> <url>...          # PR URL を追加
claudedog update --mark-checked <session_id>... # backfill_checked をセット
claudedog update --by-branch <repo> <branch> <url>  # ブランチ全セッションに URL 追加
claudedog backfill [--recheck]                  # PR URL・merged 判定・レビューコメント数の一括補完
claudedog sync-db                               # JSONL/log → SQLite 変換
```

## セットアップ

```fish
# ビルド＆配置
go build -o ~/.local/bin/claudedog ./cmd/claudedog/

# DB 生成 & ダッシュボード確認
claudedog sync-db
# Grafana で ~/.claude/claudedog.db を SQLite datasource として設定
```

## E2E テスト（Grafana スクリーンショット検証）

```fish
make grafana-up          # Docker で Grafana + Image Renderer 起動 → http://localhost:13000
make grafana-screenshot  # 全パネルの PNG を .outputs/grafana-screenshots/ に取得
make grafana-down        # コンテナ停止
```
