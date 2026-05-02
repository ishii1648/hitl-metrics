---
name: git-ship
description: >-
  Use when implementation is complete and code needs to be shipped.
  Triggered automatically per AGENTS.md when there are uncommitted changes
  on feature/fix/docs/chore branches. Also use when the user says
  "git-ship", "ship", "シップ", "ship して".
  Runs: contextual-commit → push → Draft PR.
version: 0.1.0
---

# git-ship

実装完了時に commit → push → Draft PR 作成を一貫して行う。

## 前提条件

実行前に以下を確認する。条件を満たさなければ実行しない：

1. `$PWD` がプロジェクトルート配下であること
2. 現在のブランチが `feat/`, `fix/`, `docs/`, `chore/` のいずれかで始まること（`main` 上では実行しない）
3. 未コミット変更またはステージ済み変更があること

## フロー

### Step 1: コミット

`contextual-commit` skill を呼び出してコミットを作成する。

### Step 2: Push

```
git push -u origin <current-branch>
```

### Step 3: PR 作成（未作成の場合のみ）

`gh pr list --head <branch> --state open` で既存 PR を確認する。

- **PR が存在しない場合**: `gh pr create --draft` で Draft PR を作成。タイトルはブランチの変更内容から生成。
- **PR が存在する場合**: push のみで完了（PR はすでにある）。

完了後、PR URL を出力する。
