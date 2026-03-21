---
name: create-adr
description: This skill should be used when the user asks to "create-adr", "ADR作って",
  "設計判断を記録して", "ADR書いて", or describes a design decision they want to record.
  Creates a Draft ADR in docs/adr/ and optionally adds a TODO entry.
version: 0.1.0
---

# create-adr

設計判断を受け取り、`docs/adr/` に Draft ADR を作成する。

## 目的

- 設計判断を伴う変更を ADR として記録し、将来の参照価値を確保する
- TODO.md のタスク管理フローと両立させる

## ステップ

### Step 0: ADR 評価（evaluate-adr スキルをコール）

`evaluate-adr` スキルを実行してユーザーのプロンプトを評価する。

- 判定が **ADR 推奨** の場合: Step 1 へ進む
- 判定が **ADR 不要** の場合: 理由と代替案を提示して処理を中断する
- 判定が **要確認** の場合: ユーザーの回答を待ち、再評価してから次を決める

### Step 1: 入力解析と矛盾チェック

ユーザーの入力（箇条書き or 自由文）から以下を抽出する：

- **課題の本質**: 何が問題か・何を決める必要があるか
- **領域**: 関係する領域を特定する（hooks / batch / dashboard / metrics / CLI / 複合）

**矛盾チェック**（`$PWD/.claude/skills/adr-reference/skill.md` の矛盾チェック指針を Read して参照）:
- 同領域の既存 ADR を `$PWD/docs/adr/` から Grep で検索し、矛盾する決定がないか確認する
- 依存する ADR がある場合、その前提と矛盾しないか確認する

### Step 2: ADR 番号の決定

`$PWD/docs/adr/` 内のファイルを Glob で確認し、既存の最大番号 + 1 を次番号とする。番号は3桁ゼロ埋め（001, 002, ... 054, 055）。

### Step 3: ADR 作成

`$PWD/docs/adr/NNN-英語タイトル.md` を Write ツールで作成する。

- ファイル名はハイフン区切りの英語（例: `055-metrics-collection-openmetrics-migration.md`）
- 日本語タイトルは英訳する

`$PWD/.claude/skills/adr-reference/skill.md` を Read ツールで読み込み、そのテンプレートに従って ADR を作成する。

- ユーザーが「Spike として作成する」「Spike ADR を作る」など Spike を明示した場合: ステータスを `Spike中` に設定する
- それ以外の場合: ステータスは `Draft` 固定
- ADR に受け入れ条件は記載しない（受け入れ条件は TODO.md が SSOT）

### Step 4: TODO.md に追記（任意）

実装タスクが伴う場合、`$PWD/TODO.md` の「未着手」セクションに追記する：

```
- ADR-NNN: タイトル（日本語）
  - 関連 ADR: [ADR-NNN](docs/adr/NNN-title.md)
  - [ ] 受け入れ条件1（「〜される」「〜できる」形式）
  - [ ] 受け入れ条件2
```

- 受け入れ条件は `- [ ]` チェックリストとして TODO.md に直接記述する
- 「〜される」「〜できる」形式で具体的に記述する
- 移動・統合操作は削除確認と移動先確認を別条件として書く
- 設計記録のみで実装が不要な場合（却下・Spike 完了など）はスキップする

### Step 5: 完了報告

作成した ADR パスと、TODO.md に追記した場合はその旨を報告する。

## 制約

- 操作対象は常に `$PWD` 配下
- ADR は `$PWD/docs/adr/` に直接作成する
- TODO.md は末尾追記のみ。既存行は変更しない
