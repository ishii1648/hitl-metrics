---
name: git-ship
description: >-
  Use when implementation is complete and code needs to be shipped.
  Triggered automatically per CLAUDE.md when there are uncommitted changes
  on feature/fix/docs/chore branches. Also use when the user says
  "git-ship", "ship", "シップ", "ship して".
  Runs: contextual-commit → CHANGELOG check → push → Draft PR.
version: 0.1.0
---

# git-ship

実装完了時に commit → CHANGELOG チェック → push → Draft PR 作成を一貫して行う。

## 前提条件

実行前に以下を確認する。条件を満たさなければ実行しない：

1. `$PWD` がプロジェクトルート配下であること
2. 現在のブランチが `feat/`, `fix/`, `docs/`, `chore/` のいずれかで始まること（`main` 上では実行しない）
3. 未コミット変更またはステージ済み変更があること

## フロー

### Step 1: コミット

`contextual-commit` skill を呼び出してコミットを作成する。

### Step 2: CHANGELOG チェック（feat/fix ブランチのみ）

現在のブランチが `feat/` または `fix/` で始まる場合のみ実行する。`docs/` や `chore/` ブランチではスキップ。

1. `git diff main -- CHANGELOG.md` を実行
2. 差分がなければ（CHANGELOG 未更新）:
   a. ブランチの全コミットを確認: `git log main..HEAD --oneline`
   b. CHANGELOG.md に今日の日付セクションを追加または更新し、変更内容を記載
   c. 既存エントリのスタイル・粒度に合わせる
   d. 追加コミットを作成: `docs: CHANGELOG.md を更新`
3. 差分があれば（すでに更新済み）スキップ

#### CHANGELOG の書式

```markdown
## YYYY-MM-DD

- 変更内容の説明
```

既存の同日セクションがあればそこに追記。なければファイルヘッダー直後に新規セクションを挿入。

### Step 3: Push

```
git push -u origin <current-branch>
```

### Step 4: PR 作成（未作成の場合のみ）

`gh pr list --head <branch> --state open` で既存 PR を確認する。

- **PR が存在しない場合**: `gh pr create --draft` で Draft PR を作成。タイトルはブランチの変更内容から生成。
- **PR が存在する場合**: push のみで完了（PR はすでにある）。

完了後、PR URL を出力する。
