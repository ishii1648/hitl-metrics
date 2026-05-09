# 仕組み解説 docs の Hugo 化と gh-pages 公開

Created: 2026-05-09

## 概要

仕組み（hook 構成 / データフロー / collection から Grafana 表示までの全体像 / dashboard の読み方など）を解説する docs を執筆・配信するための Hugo site を立ち上げる。配信先は GitHub Pages (public)。既存の `docs/` 配下は markdown の source-of-truth として **維持**し、解説 docs は新設する `site/content/` に書く。

## 根拠

- 仕組み解説 docs は visualize（アーキ図 / シーケンス図 / データフロー / interactive 表）の重要度が高く、純 markdown では表現が苦しい
- Mermaid in markdown だけでも一定までは行けるが、ナビゲーション・タグ filter・全文検索・複数ページの cross-reference を含む「site としての体験」を提供したい
- hand-rolled HTML は機能追加ごとに JS ライブラリを足す負債化が避けられない（→ 0011 でも採用却下済み）
- 将来 0011 段階 4（intent visualize）を同 site の `content/intent/` として乗せられるため、Hugo 投入の ROI が単発で終わらない

## 問題

- visualize 主体の解説 docs を執筆できる場所が repo 内にない
- 既存 `docs/` は Claude が input として頻繁に load する「仕様 reference 系」と「解説系」が混在しはじめており、性質に応じた分離ができていない
- gh-pages 配信の CI / build pipeline / theme 選定がいずれも未整備

## 対応方針

### 配置と分離

```
docs/                      # 既存。markdown source-of-truth として維持
  spec.md                  # Claude が input として load する reference
  design.md
  metrics.md
  setup.md
  usage.md
  history.md
  archive/adr/
  images/                  # README から参照する screenshot 等

site/                      # 新設。Hugo project root
  hugo.toml                # 設定（baseURL / theme / params）
  content/
    _index.md              # ランディング
    explain/               # 仕組み解説 docs（初版のスコープ）
      _index.md
      architecture/        # アーキテクチャ overview（page bundle）
        index.md
      data-flow/           # collection → SQLite → Grafana のデータフロー
        index.md
      hooks/               # hook 構成（Claude / Codex の対応関係）
        index.md
      dashboard/           # dashboard の読み方
        index.md
    intent/                # 将来 0011 段階 4 で追加（本 issue では空でも作らない）
  layouts/                 # theme override が必要になった時に置く
  static/                  # 共通 static assets
  themes/                  # theme（git submodule か Hugo modules で導入）

.github/workflows/
  docs-deploy.yml          # main push で hugo build → gh-pages へ deploy
```

**docs/ → site/ への参照ポリシー**:
- site から docs への link は GitHub 上の markdown URL を直接張る（Hugo 内に取り込まない）
- 逆方向（docs から site への link）は最小化。必要なら gh-pages の URL を README から張る

### 主要な実装論点

| 論点 | 既定方針 | 実装時に評価 |
|---|---|---|
| theme | docs 系で広く使われる **Hugo Book** または **Geekdoc** から選定 | 実装時に 1 日掛けて両方触って決める。最低条件: 全文検索 / sidebar nav / Mermaid 標準 |
| 検索 | theme built-in（lunr 系）で開始 | 規模が増えたら Pagefind / Algolia DocSearch を検討 |
| 図 | Mermaid 標準 | より複雑な図が必要なら D2 / PlantUML を後追い |
| asset 配置 | 解説 content は **page bundle**（`content/explain/architecture/index.md` + 同フォルダに画像） | 共通 asset のみ `site/static/` |
| theme 導入方式 | **Hugo modules**（`go.mod` で管理） | git submodule よりも Go プロジェクトとの親和性が高い |
| local 開発 | `make docs-serve` で `hugo serve --buildDrafts` | port 衝突時は `HUGO_PORT=<n> make docs-serve` |
| build / deploy | GitHub Actions: `peaceiris/actions-hugo` + `peaceiris/actions-gh-pages` | main push をトリガに `gh-pages` ブランチへ |
| baseURL | gh-pages の URL（`https://ishii1648.github.io/agent-telemetry/`） | カスタムドメインを後で当てる場合は `hugo.toml` の `baseURL` 変更で完結 |
| link checker | CI に markdown link checker を追加 | docs/ ↔ site/ 間 link rot 防止 |

### 初版コンテンツのスコープ

執筆対象は最小 4 ページ（page bundle）:
1. **architecture** — agent-telemetry 全体図（hook → JSONL → SQLite → Grafana）
2. **data-flow** — Claude / Codex 各々の transcript パース → 集約までのフロー
3. **hooks** — どの hook がどのデータを何に使うかの対応表
4. **dashboard** — Grafana dashboard の panel ごとの読み方

これ以上の content は別 issue で追加する。**本 issue は site の立ち上げと最低限の解説 4 ページを scope とする**。

### CLAUDE.md / AGENTS.md への影響

- 「ドキュメント構成」セクションに `site/content/explain/` の役割を追記
- 「ダッシュボード変更時の必須作業」に「`site/content/explain/dashboard/` 側も同期更新が必要なケースがある」旨を追記（screenshot 更新と同じ列に並べる）
- 「実装セッション（feature ブランチ / worktree）」の「main の `docs/` は変更しない」ルールは **`docs/` のままで `site/` を含めない**（`site/` は実装ブランチで触ってよい）

これらの改訂は本 issue の PR 内で行う。

## 受け入れ条件

- [ ] `site/` ディレクトリ構成と `hugo.toml` を作成
- [ ] theme を選定（Hugo Book / Geekdoc 等）し、Hugo modules で導入
- [ ] 初版 4 ページ（architecture / data-flow / hooks / dashboard）を page bundle として執筆
- [ ] Mermaid で各ページに最低 1 つは図を入れる
- [ ] `make docs-serve` で local 確認できる（Makefile に target 追加）
- [ ] `.github/workflows/docs-deploy.yml` を作成、main push で gh-pages へ自動 deploy
- [ ] 初回 deploy 後、gh-pages の URL を README に追加
- [ ] markdown link checker を CI に追加（または既存 lint job に統合）
- [ ] CLAUDE.md / AGENTS.md を上記方針に沿って更新
- [ ] `docs/` 配下は本 issue では **触らない**（移管しない）

## 進行方針・PR 分割

- **PR 1**: site scaffolding + theme + Makefile target + 1 ページ executable な状態（local 確認可能）
- **PR 2**: 残り 3 ページ + GitHub Actions workflow + README 更新
- 1 PR にまとめても良いが、theme 選定で時間が読めない場合は PR 1 で scaffolding を確定して PR 2 で content に集中する

## 関連 issue

- **0011** — issues/ への structured intent store。本 issue (0012) と独立に進行可能
- **将来 0013（仮）** — 0011 段階 4。0011 段階 1+2 と 0012 の両方完了後に着手し、`site/content/intent/` を frontmatter から生成する
