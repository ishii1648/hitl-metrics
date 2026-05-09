---
decision_type: process
affected_paths:
  - issues/
  - AGENTS.md
  - CLAUDE.md
  - scripts/intent-lookup
  - scripts/test-intent-lookup.sh
  - .claude/skills/intent-lookup/
  - Makefile
tags: [intent, decision-record, frontmatter, tooling]
closed_at: 2026-05-10
---

# 構造化された意図蓄積 — issues/ への frontmatter 導入と path 逆引き

Created: 2026-05-09

## 概要

`issues/<NNNN>-*.md` の意味を「タスク」から「**タスク兼意思決定記録**」に拡張する。issue に frontmatter（`decision_type` / `affected_paths` / `supersedes` / `tags` / `closed_at`）を追加し、意思決定の構造化された primary store とする。コードを編集している場所から関連する過去の意図を逆引きできる CLI を提供する。

## 根拠

agentic engineering で最も参照される情報は「なぜその仕様・設計・実装にしたのか」。これはコード・spec・design いずれにも登場せず、将来の実装変更やバグ修正の際に決定的に重要となる。

現状の意図は 4 箇所に分散している（`docs/history.md` / `docs/archive/adr/` / Contextual Commits の action 行 / `issues/closed/*.md` の `## 解決方法`）が、以下の構造的な弱点を抱える：

1. **`history.md` は事後編集**で「これは history に書くべき」という判断漏れで永久に欠落する。capture が optional なので decay する
2. **粒度が大方針に偏る**。history.md は 11 件の pivot しか拾えず、API 命名・retry 戦略・フィールド追加の根拠・フラグ既定値の根拠などの micro decision は commit body にしか残らない
3. **コードからの逆引きが存在しない**。`internal/agent/codex/parser.go` を編集する時に、その場所に effect を持つ過去の意思決定を引くインターフェースがない（`git blame` → commit body → 隣接 issue を辿る職人芸でしか辿れない）

issues/ への移行で得られた基盤（**永続化された連番** / **ライフサイクル** / **closed の `## 解決方法`**）を活用すれば、上記 3 点を構造的に解決できる。

## 問題

- frontmatter 規約が未定義（書き手によってバラバラになる）
- 既存の open / closed / pending issues に frontmatter が入っていない（retrofit が必要）
- code path から intent を逆引きするインターフェースがない
- `AGENTS.md` の「issues について」が「タスクの記録」前提のまま（拡張された意味論が未反映）
- `CLAUDE.md` の「意思決定の記録方針」が history.md / commit / spec / design の 4 分類のままで、issue が意思決定 store として位置づけられていない

## 対応方針

意思決定の primary store を **issues/** に集約する。段階 1 が前提、段階 3 まで終われば「コードを触る Claude が当該箇所の過去の意思決定を取れる」状態になる。

### 段階 1: frontmatter 規約

`AGENTS.md` の「issue ファイルの構造」セクションに以下の frontmatter を追記する：

```yaml
---
decision_type: spec | design | implementation | process
affected_paths:
  - internal/agent/codex/
  - cmd/agent-telemetry/setup.go
supersedes: [0023]
tags: [hooks, multi-agent, packaging]
closed_at: YYYY-MM-DD
---
```

セマンティクス:

- `decision_type` — 意思決定の層（`spec`=外部契約 / `design`=内部設計 / `implementation`=実装 detail / `process`=開発プロセス）
- `affected_paths` — path snapshot。rename は `git log --follow` で逆引き側が吸収するため path 自体は更新しない
- `supersedes` — 過去 issue 番号の配列。supersededBy への双方向参照は索引側で生成
- `tags` — free-form。当面は明示的なタグ語彙統制を置かず、出現頻度から事後に整理する
- `closed_at` — close 時に確定。open / pending では省略可

合意済みの線引き: **「複数コミット or 後続が参照しそうな決定」**を issue 化する。1 コミットで完結する判断は Contextual Commits の action 行で十分（既存規約のまま）。

### 段階 2: 既存 issue の retrofit

- `issues/closed/*.md` (0001, 0002, 0010) に frontmatter を埋める
- `issues/<id>-*.md` (0003, 0004, 0009) に open 用の最小 frontmatter を入れる（`closed_at` 省略）
- `issues/pending/*.md` (0005-0008) にも同様
- `docs/history.md` の 11 件は本 issue では触らない。retro 化（過去判断を遡って issue として起こすか、history.md に残置するか）は別 issue で扱う

### 段階 3: path 逆引き script

**production binary (`agent-telemetry`) に subcommand を生やさない** — telemetry 計測 / hook という本体の責務範囲を超え、distribution（goreleaser で end user に配布される binary）に dev/docs 用ツールを混ぜることになるため。

実装は **bash script で十分**（frontmatter 走査 + `git log` 抽出 + 出力整形のみ、~100 行規模）。Go binary は overkill：型安全性が load-bearing になるほどスキーマは複雑にならず、将来の Hugo build-time 連携も script を外部コマンドとして呼ぶ形で十分。

```
scripts/intent-lookup            # bash script（既存 scripts/migrate-db-name.sh と同じ慣習: #!/usr/bin/env bash + set -euo pipefail）
.claude/skills/intent-lookup/    # Claude Code から呼ぶ skill（script を wrap）
Makefile                         # `make intent PATH=...` target を追加（script の thin wrapper）
```

- `cmd/agent-telemetry/` には触れない。`.goreleaser.yaml` も変更しない
- 人間は `make intent PATH=internal/agent/codex/` で invoke
- Claude Code は `.claude/skills/intent-lookup/` 経由で自然に呼ぶ
- 依存: `yq`（YAML frontmatter parse）/ `jq`（JSON 出力）。両者とも開発環境に既に入っている前提（必要なら setup.md に明記）

機能要件:

- 入力 path に対し `affected_paths` が前方一致する open / closed / pending issues を一覧
- 併せて `git log --follow -- <path>` で commit body から `intent:` / `decision:` / `rejected:` / `constraint:` / `learned:` 行を抽出してマージ表示
- 既定出力は markdown（人も Claude も読める）
- `--format=json` で機械可読出力（Claude が context として読み込みやすくする / 将来 0011 段階 4 の Hugo build へ食わせる中間データとしても流用可）

テストは fixture issue + 期待出力の差分比較で十分（bats を入れるか、`scripts/test-intent-lookup.sh` で expected/actual diff を取るかは実装時に判断）。

### 段階 4 (本 issue の対象外): 人間用俯瞰 view

人間用俯瞰 view（timeline / supersedes グラフ / tag filter / 全文検索）の提供は別 issue として切り出す。**0012 (Hugo docs site) を前提**とし、`site/content/intent/` 配下に issues/ の frontmatter から生成する形で実装する（hand-rolled HTML は採用しない — 機能追加ごとに JS ライブラリを足す負債化を避けるため）。

Claude は引き続き `issues/<id>-*.md` の frontmatter / 本文を直接読むため、段階 3 までで agentic 編集サイクルの要求は満たされる。段階 4 は人間用 view 限定の付加価値。

### `docs/history.md` の位置付け変更

段階 3 完了後の history.md は「**書く場所**」ではなく「**人間が大方針を筋立てたナラティブ要約**」に役割を絞る。新規エントリは原則 issue 側に書き、history.md には issue へのリンクと一文要約のみ追記する形が望ましい。これは段階 2 完了後に CLAUDE.md の「意思決定の記録方針」を更新する形で明文化する。

## 受け入れ条件

- [ ] `AGENTS.md` の「issue ファイルの構造」に frontmatter 仕様を追記し、各フィールドの語彙とセマンティクスを文書化
- [ ] `AGENTS.md` の「issues について」冒頭で issue が「タスク兼意思決定記録」であることを明記
- [ ] `CLAUDE.md` の「意思決定の記録方針」を更新し、micro decision の線引き（「複数コミット or 後続が参照しそうな決定」）と issue が primary store である旨を明記
- [ ] 既存 closed issues 3 件 (0001, 0002, 0010) に frontmatter を retrofit
- [ ] 既存 open issues 3 件 (0003, 0004, 0009) と pending issues 4 件 (0005-0008) にも最小 frontmatter を追加
- [ ] `scripts/intent-lookup` を bash で実装（frontmatter 走査 + `git log --follow` の Contextual Commits 行抽出をマージ）
- [ ] **`cmd/agent-telemetry/` / `.goreleaser.yaml` には一切変更を加えない**
- [ ] `--format=json` / 既定 markdown 出力の両方をサポート
- [ ] `Makefile` に `make intent PATH=<p>` target を追加（script の thin wrapper）
- [ ] `.claude/skills/intent-lookup/` を新設（Claude Code 経由で自然に呼べるようにする）
- [ ] script のテスト（fixture issue + git log で逆引きが期待通り返ることを検証）
- [ ] 段階 4（HTML view 生成）は本 issue に含めず、別 issue として切り出す

## 進行方針・PR 分割

- 段階 1（規約）+ 段階 2（retrofit）は同 PR に含めて良い（規約と適用例がセットで意味を持つため）
- 段階 3（CLI）は別 PR に分ける
- close 時に frontmatter 必須化する hook / lint は本 issue では入れない（運用後に必要性が出たら別 issue）

## 影響を受ける既存仕様

- `AGENTS.md` の「issues について」セクション
- `CLAUDE.md` の「意思決定の記録方針」セクション
- `issues/<id>-*.md` のファイル形式（frontmatter 追加）
- `Makefile`（`make intent` target 追加）
- `scripts/intent-lookup`（新規 bash script）
- `.claude/skills/intent-lookup/`（新規）

**変更しない**:
- `cmd/agent-telemetry/`（production binary は telemetry / hook 計測の責務に閉じる）
- `.goreleaser.yaml`（end user に配布する binary に dev tool を混ぜない）

Completed: 2026-05-10

## 解決方法

### 段階 1: frontmatter 規約

- `AGENTS.md` の「issues について」冒頭で、`issues/` が「タスク兼意思決定記録」の primary store であることを明記。`make intent PATH=<p>` で逆引きできる旨と「複数コミット or 後続が参照しそうな決定」を issue 化する線引きを追記
- `AGENTS.md` の「issue ファイルの構造」セクションに frontmatter テンプレートと各フィールド（`decision_type` / `affected_paths` / `supersedes` / `tags` / `closed_at`）の表を追加
- `CLAUDE.md` の「意思決定の記録方針」を 4 分類から 5 分類に更新し、issue が primary store / `docs/history.md` は事後ナラティブ要約に役割を絞る旨を明記

### 段階 2: 既存 issue の retrofit

- closed 3 件 (0001, 0002, 0010)・open 3 件 (0003, 0004, 0009)・pending 4 件 (0005-0008) すべてに frontmatter を追加
- 本 issue (0011) と新規 0012 (Hugo docs site) にも揃って frontmatter を入れた（一貫性維持）

### 段階 3: path 逆引き script

- `scripts/intent-lookup` を bash で実装。frontmatter 走査 + `git log --follow -- <path>` の Contextual Commits 行抽出を結合
- prefix 一致は **bidirectional**（`entry ⊆ query` か `query ⊆ entry` のどちらでも overlap 扱い）。これでファイル指定 / ディレクトリ指定どちらでも自然に拾える
- `--format=markdown`（既定）と `--format=json`（Claude が context として読み込みやすい / 将来 0012 の Hugo build で中間データとして流用可）の両方に対応
- 依存: `yq` (mikefarah) / `jq` / `git`。dev 環境前提なので distribution 経路には載せない
- `.claude/skills/intent-lookup/SKILL.md` を新設（Claude Code がコード編集前に自然に呼ぶための trigger 文書）

### 段階 4 を別 issue に切り出し

人間用俯瞰 view は 0012 (Hugo docs site) を前提に切り出す方針が issue 本文で確定済み。本 issue の対象外。

### Makefile target の変数名

issue 本文の例では `make intent PATH=<p>` だが、`PATH` は Make の実行時 PATH と衝突して recipe が壊れるため、実装では `make intent P=<p>` に変更した（FORMAT=json も同様に追加）。

### 採用しなかった代替

- bats 等のテストフレームワーク導入: `scripts/test-intent-lookup.sh` 1 本で fixture 込み self-contained に書ける規模だったため不要
- frontmatter 必須化の lint hook: 本 issue のスコープ外（運用後に欠落事故が出たら別 issue で対処）
- `cmd/agent-telemetry/` への subcommand 追加 / `.goreleaser.yaml` 変更: production binary の責務（telemetry / hook 計測）を逸脱するため明示的に除外（受け入れ条件にも記載）

### 検証

- `make test-intent` → 12 アサーション全 pass（fixture git repo で JSON 構造 / bidirectional prefix overlap / 関係ない issue の除外 / action 行のないコミットの除外 / markdown 構造 / no-arg / frontmatter 欠落 issue の skip を検証）
- `./scripts/intent-lookup internal/hook/stop.go` 等の手動実行で実 repo に対する出力も期待通りであることを確認
