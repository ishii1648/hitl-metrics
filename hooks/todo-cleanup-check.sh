#!/bin/bash
# SessionStart hook: main ブランチで TODO.md に完了済みタスクがあれば通知する

branch=$(git branch --show-current 2>/dev/null)
if [ "$branch" != "main" ]; then
  exit 0
fi

todo_file="$PWD/TODO.md"
if [ ! -f "$todo_file" ]; then
  exit 0
fi

# 未着手セクションに [x] (完了済み) がある場合、クリーンアップ対象
if grep -q '\- \[x\]' "$todo_file"; then
  echo "TODO.md に完了済みの受け入れ条件があります。全条件が完了したタスクを CHANGELOG.md に移動して TODO.md から削除してください。"
fi
