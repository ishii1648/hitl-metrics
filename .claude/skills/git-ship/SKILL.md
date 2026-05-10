---
name: git-ship
description: >-
  Use when implementation is complete and code needs to be shipped.
  Triggered automatically per CLAUDE.md when there are uncommitted changes
  on feature/fix/docs/chore branches. Also use when the user says
  "git-ship", "ship", "シップ", "ship して".
  Runs: contextual-commit → push → Draft PR → auto-fix-ci.
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

PR URL を出力したら Step 4 に進む。

### Step 4: CI 自動監視

`auto-fix-ci` skill を起動する。Monitor tool で CI を継続 watch し、失敗ジョブのログを取得して原因を診断 → 修正 → 再 push のループを自動実行する。

すべての check が緑になるか、自動修正不能な失敗（secrets 不足・外部障害・scope 越え）に達した時点で報告して終了する。
