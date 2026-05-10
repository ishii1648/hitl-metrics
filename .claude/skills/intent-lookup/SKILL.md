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

## 位置づけ

これは **意図記録への逆引き索引** であって、意図そのものではない。意図そのもの
は issue 本文・docs・commit body 側にある。本ツールは「変更しようとしている
path に紐づく決定の候補」を見落とさないための入口を提供する。

## いつ使うか

- 既存のファイルを編集する直前。なぜ現状の実装になっているか・過去にどんな代替案が検討・却下されたかを context として読み込む
- バグ修正で「同じ path で過去に近い修正があったか」を確かめたい
- 大きめのリファクタ前に、既存の制約（`constraint:` 行や `rejected:` 行）を見落とさないように

逆に：1 行の typo 修正や、自分が直近で触ったばかりの path には不要。

## 呼び方

```bash
scripts/intent-lookup <path> [--format=markdown|json] [--full]
scripts/intent-lookup --lint  [--format=markdown|json]
```

または Makefile 経由：

```bash
make intent P=<path>                       # markdown 出力（既定: 抜粋付き）
make intent P=<path> FORMAT=json           # JSON 出力
make intent P=<path> FULL=1                # 抜粋ではなく本文全文を含める
make intent-lint                           # frontmatter の健全性検査（warnings 非 gating, errors のみ gating）
make intent-lint STRICT=1                  # warnings も gating（CI 用）
make intent-lint FORMAT=json               # lint を JSON で
```

`<path>` はファイルでもディレクトリでも可（リポジトリルートからの相対 path）。
末尾 `/` の有無は問わない。

## 動作

1. `issues/`, `issues/closed/`, `issues/pending/` の全 issue から frontmatter を抽出
2. クエリ path を `git log --follow --name-only` で展開し、過去の rename を考慮した path 候補集合を作る（**rename-aware**: issue 側の `affected_paths` を rename 後に書き換えなくても、現在 path で逆引きできる）
3. issue の `affected_paths` のいずれかが path 候補集合と前方一致で overlap する issue を集める（`entry ⊆ query` か `query ⊆ entry` のどちらでも該当）
4. issue 本文から「概要 / 根拠 / 問題 / 対応方針 / 解決方法」セクションの先頭段落を抜粋（`--full` で全文）
5. `git log --follow -- <path>` から各 commit を取得し、body の `intent:` /
   `decision:` / `rejected:` / `constraint:` / `learned:` 行を抽出
6. issue 一覧と commit 一覧をまとめて出力

## 出力の読み方

- **Issues** セクション — 構造化された大きめの決定。`decision_type`、関連 path、tags、close 日付に加え、本文の **概要 / 対応方針 / 解決方法** の抜粋を含む。詳細を読む必要があるかどうかの判定材料として使う
- **Commits** セクション — 1 コミットで完結した micro decision。Contextual Commits の action 行のみ抜粋
- 出力先頭の `Rename-aware: also matched against N historical path(s)` は、現 path だけでなく旧 path も対象にしたことを示す

両方で 0 件なら、その path には記録された意図がない。新しい決定を残すなら
`issues/<NNNN>-...` に書き起こすか、commit body に action 行を入れる。

抜粋を見て「もっと読まないと判断できない」と感じたら、`--full` を付けて再実行する
か、`file:` に出ている issue ファイルを直接開く。

## 使用例

```bash
# 特定ファイルへの過去の意図（抜粋）
make intent P=internal/hook/stop.go

# 抜粋では足りない時は全文
make intent P=internal/hook/stop.go FULL=1

# ディレクトリ全体の意思決定（broader）
make intent P=internal/syncdb/

# JSON で取って Claude が context として読み込む
make intent P=internal/sessionindex/ FORMAT=json

# affected_paths の健全性検査（CI でも使える）
make intent-lint
```

## lint mode

`scripts/intent-lookup --lint` は `issues/` 全体を walk して以下を検出する:

- `frontmatter_missing` — frontmatter が無い issue（warning）
- `frontmatter_invalid` — frontmatter のパース失敗（error）
- `decision_type_invalid` — `spec` / `design` / `implementation` / `process` 以外（error）
- `affected_paths_empty` — 逆引きでヒットしない（warning）
- `affected_path_missing` — repo に存在しない path（warning）
- `affected_path_broad` — top-level dir 単独 (`internal/`, `docs/` 等) でノイズ massive（warning）

legitimate な broad / missing は `lint_ignore_broad` / `lint_ignore_missing` を frontmatter に書いて path 単位で抑制する（理由は YAML コメントで併記）。例:

```yaml
lint_ignore_broad: [issues/]      # meta issue: 規約変更そのものが issues/ 全体に効く
lint_ignore_missing: [site/]      # 0012 の Hugo build 後に生成される dir
```

抑制を放置せず、warning が出たら **抑制 / 修正 / path 具体化** のいずれかで対応すること。warning が常時出る状態が続くと lint の信号性が落ちる。

exit code:
- 既定 (`make intent-lint`): `0` clean or warnings only / `2` errors
- `--strict` (`make intent-lint STRICT=1`): `0` clean / `1` warnings / `2` errors

ローカル開発では既定で十分（warnings は表示されるが make が失敗しない）。CI で warnings まで gating したい場合は `--strict` / `STRICT=1` を使う。

## 依存

- Python 3.9+（標準ライブラリのみ。yq / jq / awk は不要）
- `git`

開発環境に標準で入っている前提。`brew install python` 以外の追加 install は不要。
