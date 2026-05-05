# agent-telemetry

Claude Code および Codex CLI の PR 単位のトークン消費効率を追跡・可視化する計測ツール（hook・CLI・ダッシュボード）。

> 旧称 `hitl-metrics`。リネームの意思決定は `docs/history.md` の「8. リポジトリ名変更 — hitl-metrics → agent-telemetry（2026-05-04）」を参照。`doctor` / `uninstall-hooks` は旧名の hook 登録も検出する（互換性のため）。

## ドキュメント構成

- `docs/spec.md` — 外部契約（CLI コマンド・hook 仕様・データモデル）
- `docs/metrics.md` — 計測フレームワーク（観察軸・解釈・OpenMetrics カタログ）
- `docs/design.md` — 実装方針と設計判断
- `docs/history.md` — 過去の経緯と廃止された設計
- `docs/setup.md` / `docs/usage.md` — セットアップと運用
- `docs/archive/adr/` — 過去の意思決定記録（旧 ADR 形式、参照のみ）

新規の設計判断は ADR を作成しない。`docs/design.md` を更新し、Contextual Commits のアクション行で「なぜ」を記録する。大きな方針転換は `docs/history.md` にも追記する。

## セッションモード

### 設計セッション（main ブランチ）

- 変更対象: `docs/`, `TODO.md`, `CLAUDE.md` のみ
- コード変更禁止（Spike を除く）
- 仕様変更は `docs/spec.md` を更新する
- 実装方針の変更は `docs/design.md` を更新する。「なぜ」が複数コミットにわたる大きな転換の場合は `docs/history.md` にも追記する
- `TODO.md` のセクションは `実装タスク`（受け入れ条件あり、すぐ着手可能）と `検討中`（仕様未確定）の 2 つだけ。仕様が固まったタスクは受け入れ条件（`- [ ]`）を整えて `検討中` → `実装タスク` に移動する

### 実装セッション（feature ブランチ / worktree）

- 対象タスクを 1 つ実装する（`TODO.md` の受け入れ条件に従う）
- worktree 作成: `gw_add feat/<task-name>`
- 受け入れ条件を満たすまで実装 → 検証 → 修正
- main の `docs/` は変更しない（仕様/設計の更新は merge 後に main で実施）

## 開発規約

### 意思決定の記録方針

- 仕様の変更 → `docs/spec.md` を更新
- 実装方針の変更 → `docs/design.md` を更新
- 過去の意思決定の経緯として残す価値がある転換 → `docs/history.md` に追記
- 1 コミット内で完結する判断 → Contextual Commits のアクション行で記録
- chore / リファクタなど意思決定を伴わない変更 → アクション行不要

### コミット

Contextual Commits を使用。Conventional Commits プレフィックス + 構造化されたアクション行でコミットの意図を記録する。

### ブランチ命名

`feat/`, `fix/`, `docs/`, `chore/` + kebab-case（例: `feat/add-sync-db`）

### テスト

```fish
go test ./...                          # 全テスト
make grafana-screenshot                # E2E: Grafana スクリーンショット検証
```

### ダッシュボード変更時の必須作業

- `grafana/dashboards/agent-telemetry.json` の表示を変更した場合は、必ず `make grafana-screenshot` を実行して README 用スクリーンショット（`docs/images/dashboard-*.png`）も同じ変更に合わせて更新する（`grafana-screenshot` は `grafana-up-e2e` 経由で fixture データを使うので、画像が決定的に再現される）。
- スクリーンショット生成でポート競合が起きる場合は `GRAFANA_PORT=<unused-port> make grafana-screenshot` を使う。
- 実データで動作確認したい場合は `make grafana-up`（`~/.claude/agent-telemetry.db` を mount）。E2E と同じコンテナを使うので、切替時は片方が再作成される。
