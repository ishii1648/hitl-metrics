---
name: intent-lookup
description: >-
  Use when about to edit a file or directory in this repo and you want to know
  the past structured intent (issues with overlapping affected_paths, plus
  Contextual Commits action lines from `git log --follow`) for that path.
  Trigger on phrases like "intent", "意図を調べて", "なぜこのコードはこうなった",
  "この path の意思決定履歴", or when starting non-trivial edits to internal/
  / cmd/ / docs/ / grafana/ files.
---

# intent-lookup

`issues/<id>-*.md` の frontmatter（`affected_paths`）と git log の Contextual
Commits 行を統合して、コードの特定箇所に effect を持つ過去の意思決定を取得する。

## いつ使うか

- 既存のファイルを編集する直前。なぜ現状の実装になっているか・過去にどんな代替案が検討・却下されたかを context として読み込む
- バグ修正で「同じ path で過去に近い修正があったか」を確かめたい
- 大きめのリファクタ前に、既存の制約（`constraint:` 行や `rejected:` 行）を見落とさないように

逆に：1 行の typo 修正や、自分が直近で触ったばかりの path には不要。

## 呼び方

```bash
scripts/intent-lookup <path> [--format=markdown|json]
```

または Makefile 経由：

```bash
make intent P=<path>              # markdown 出力（既定）
make intent P=<path> FORMAT=json  # JSON 出力
```

`<path>` はファイルでもディレクトリでも可（リポジトリルートからの相対 path）。
末尾 `/` の有無は問わない。

## 動作

1. `issues/`, `issues/closed/`, `issues/pending/` の全 issue から frontmatter を抽出
2. `affected_paths` のいずれかが `<path>` と前方一致で overlap する issue を集める
   （`entry ⊆ query` か `query ⊆ entry` のどちらでも該当）
3. `git log --follow -- <path>` から各 commit を取得し、body の `intent:` /
   `decision:` / `rejected:` / `constraint:` / `learned:` 行を抽出
4. issue 一覧と commit 一覧をまとめて出力

## 出力の読み方

- **Issues** セクション — 構造化された大きめの決定。`decision_type`、関連 path、tags、close 日付を含む
- **Commits** セクション — 1 コミットで完結した micro decision。Contextual Commits の action 行のみ抜粋

両方で 0 件なら、その path には記録された意図がない。新しい決定を残すなら
`issues/<NNNN>-...` に書き起こすか、commit body に action 行を入れる。

## 使用例

```bash
# 特定ファイルへの過去の意図
make intent P=internal/hook/stop.go

# ディレクトリ全体の意思決定（broader）
make intent P=internal/syncdb/

# JSON で取って Claude が context として読み込む
make intent P=internal/sessionindex/ FORMAT=json
```

## 依存

- `yq` (mikefarah/yq) — frontmatter parse
- `jq` — JSON 整形
- `git` — commit body 抽出

開発環境に既に入っている前提。なければ `brew install yq jq` 等。
