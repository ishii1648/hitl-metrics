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
- `site/content/explain/` — 仕組み解説 docs（visualize 主体、gh-pages へ配信）

`docs/` は Claude が input として load する **reference** の正本。`site/content/explain/` は Mermaid 図・dashboard 解説など visualize 主体の **解説 docs** で、配信先は GitHub Pages (`https://ishii1648.github.io/agent-telemetry/`)。site から docs への参照は GitHub の markdown URL を直接張る（Hugo 内に取り込まない）。

新規の設計判断は ADR を作成しない。`docs/design.md` を更新し、Contextual Commits のアクション行で「なぜ」を記録する。大きな方針転換は `docs/history.md` にも追記する。

## セッションモード

### 設計セッション（main ブランチ）

- 変更対象: `docs/`, `issues/`, `CLAUDE.md` のみ
- コード変更禁止（Spike を除く）
- 仕様変更は `docs/spec.md` を更新する
- 実装方針の変更は `docs/design.md` を更新する。「なぜ」が複数コミットにわたる大きな転換の場合は `docs/history.md` にも追記する
- 着手可能なタスクは `issues/<NNNN>-<cat>-<slug>.md`（受け入れ条件 `- [ ]` あり）として open し、設計判断・外部依存待ちは `issues/pending/` に置く。詳細は `AGENTS.md` の「issues について」を参照

### 実装セッション（feature ブランチ / worktree）

- 対象 issue を 1 つ実装する（`issues/<NNNN>-<cat>-<slug>.md` の受け入れ条件に従う）
- worktree 作成: `gw_add feat/<task-name>`
- 受け入れ条件を満たすまで実装 → 検証 → 修正
- 完了したら最終コミットで `git mv issues/<id>-... issues/closed/<id>-...` し、末尾に `Completed:` と `## 解決方法` を追記する
- main の `docs/` は変更しない（仕様/設計の更新は merge 後に main で実施）。`site/` は実装ブランチで触ってよい（解説 docs の更新を実装と同 PR に乗せられる）

## 開発規約

### 意思決定の記録方針

意思決定の primary store は `issues/`。frontmatter の `decision_type` / `affected_paths` で構造化される。`make intent P=<p>` はコードから関連 issue / コミットへの **逆引き索引** として使う（意図そのものは issue 本文・docs・commit body 側にあり、`--full` で本文を取得できる）。

- **複数コミット or 後続が参照しそうな決定** → `issues/<NNNN>-...` に書く（frontmatter で `decision_type` と `affected_paths` を埋める）
- 仕様の変更 → `docs/spec.md` を更新（issue にも `decision_type: spec` で記録）
- 実装方針の変更 → `docs/design.md` を更新（issue にも `decision_type: design` で記録）
- 1 コミット内で完結する判断 → Contextual Commits のアクション行で記録（issue 化不要）
- chore / リファクタなど意思決定を伴わない変更 → アクション行不要

`docs/history.md` は「人間が大方針を筋立てたナラティブ要約」に役割を絞る。新規エントリは原則 issue 側に書き、history.md には issue へのリンクと一文要約のみ追記する。

### コミット

Contextual Commits を使用。Conventional Commits プレフィックス + 構造化されたアクション行でコミットの意図を記録する。

### ブランチ命名

`feat/`, `fix/`, `docs/`, `chore/` + kebab-case（例: `feat/add-sync-db`）

### バグ・課題管理

`issues/` 配下で Markdown ライフサイクル管理する。命名規則・SEQUENCE 運用・ディレクトリ構成・close/reopen/pending の手順は `AGENTS.md` の「issues について」セクションを正とする。CLAUDE.md と AGENTS.md の二重管理を避けるため、ルールの本体は AGENTS.md 側のみに置く。

### テスト

```fish
go test ./...                          # 全テスト
make grafana-screenshot                # E2E: Grafana スクリーンショット検証
```

### ダッシュボード変更時の必須作業

- `grafana/dashboards/agent-telemetry.json` の表示を変更した場合は、必ず `make grafana-screenshot` を実行して README 用スクリーンショット（`docs/images/dashboard-*.png`）も同じ変更に合わせて更新する（`grafana-screenshot` は `grafana-up-e2e` 経由で fixture データを使うので、画像が決定的に再現される）。
- スクリーンショット生成でポート競合が起きる場合は `GRAFANA_PORT=<unused-port> make grafana-screenshot` を使う。
- 実データで動作確認したい場合は `make grafana-up`（`~/.claude/agent-telemetry.db` を mount）。E2E と同じコンテナを使うので、切替時は片方が再作成される。
- panel の構成・読み方を変更した場合は `site/content/explain/dashboard/index.md` の解説も同期更新が必要なケースがある。

### docs site（`site/`）

- ローカル確認: `make docs-serve`（既定 port 1313、`HUGO_PORT=<n>` で上書き）
- ビルド検証: `make docs-build`
- theme は Hugo modules で導入（`site/go.mod` + `[module.imports]` in `hugo.toml`）。初回・theme 更新時は `make docs-mod-update`
- main push 時に `.github/workflows/docs-deploy.yml` が `gh-pages` ブランチへ自動 deploy する
