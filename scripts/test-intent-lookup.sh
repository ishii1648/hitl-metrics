#!/usr/bin/env bash
# scripts/intent-lookup の振る舞いを fixture git repo + 期待出力の差分で検証する。
#
# - tmp dir に fresh な git repo を作って fixture issue + commit を仕込む
# - intent-lookup を呼んで JSON 出力を期待値と比較
# - markdown 出力は構造的なアサーションのみ（本文の整形はあえて固定しない）
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
script="$repo_root/scripts/intent-lookup"

if [ ! -x "$script" ]; then
    echo "FAIL: $script not executable" >&2
    exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# 親リポジトリの状態が混ざらないよう、fixture 専用の deterministic な git env を作る
export GIT_AUTHOR_NAME='Test'
export GIT_AUTHOR_EMAIL='test@example.com'
export GIT_COMMITTER_NAME='Test'
export GIT_COMMITTER_EMAIL='test@example.com'
export GIT_AUTHOR_DATE='2026-01-01T00:00:00Z'
export GIT_COMMITTER_DATE='2026-01-01T00:00:00Z'

cd "$work"
git init -q -b main
mkdir -p issues issues/closed issues/pending internal/hook internal/syncdb cmd/agent-telemetry
echo '0003' > issues/SEQUENCE

# fixture issue 0001 (closed) — internal/hook/ 配下
cat > issues/closed/0001-bug-fixture-stop.md <<'EOF'
---
decision_type: design
affected_paths:
  - internal/hook/stop.go
  - internal/syncdb/
tags: [hooks, fixture]
closed_at: 2026-01-02
---

# Fixture: Stop hook の何かを直す

Created: 2026-01-01

## 概要

fixture issue.
EOF

# fixture issue 0002 (open) — internal/hook/ ディレクトリ全体
cat > issues/0002-feat-fixture-hook-area.md <<'EOF'
---
decision_type: implementation
affected_paths:
  - internal/hook/
tags: [hooks, fixture, broad]
---

# Fixture: hook ディレクトリ全体の話

Created: 2026-01-01
EOF

# fixture issue 0003 (pending) — 関係ない path
cat > issues/pending/0003-design-fixture-unrelated.md <<'EOF'
---
decision_type: design
affected_paths:
  - cmd/agent-telemetry/setup.go
tags: [unrelated, fixture]
---

# Fixture: 関係ない issue

Created: 2026-01-01
EOF

# frontmatter なし issue (skip されるべき)
cat > issues/no-frontmatter.md <<'EOF'
# 古い形式の issue

frontmatter を持たない。intent-lookup は skip するはず。
EOF

# 対象ファイルを作って commit を 2 つ積む（action 行ありとなし）
echo 'package hook' > internal/hook/stop.go
git add -A
git commit -q -m "feat(hook): add stop.go

intent: capture stop signals
decision: handle SIGTERM in main loop
"

echo 'package hook // updated' > internal/hook/stop.go
git -c commit.gpgsign=false commit -q -am "fix(hook): tweak stop.go

constraint: must remain idempotent
learned: subagent termination races with parent
"

# action 行のないコミット (extracted should be 0)
echo '// noop' >> internal/hook/stop.go
git -c commit.gpgsign=false commit -q -am "chore(hook): noop comment"

# ---------- assertions ----------
fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok: $*"; }

# 1) JSON 出力の構造
out_json="$("$script" --format=json internal/hook/stop.go)"
echo "$out_json" | jq -e '.path == "internal/hook/stop.go"' >/dev/null \
    || fail "json: path mismatch"
pass "json: path normalized"

issue_ids="$(echo "$out_json" | jq -r '.issues[].id' | sort | tr '\n' ' ')"
[ "$issue_ids" = "0001 0002 " ] \
    || fail "json: expected issues '0001 0002 ', got '$issue_ids'"
pass "json: matched issues 0001 (file path) + 0002 (parent dir)"

# unrelated issue 0003 must not appear
echo "$out_json" | jq -e '[.issues[].id] | index("0003") == null' >/dev/null \
    || fail "json: unrelated issue 0003 leaked into result"
pass "json: unrelated 0003 excluded"

# frontmatter 各フィールドが取れている
echo "$out_json" | jq -e '.issues[] | select(.id == "0001") | .frontmatter.decision_type == "design"' >/dev/null \
    || fail "json: 0001 decision_type not preserved"
echo "$out_json" | jq -e '.issues[] | select(.id == "0001") | .frontmatter.closed_at == "2026-01-02"' >/dev/null \
    || fail "json: 0001 closed_at not preserved"
echo "$out_json" | jq -e '.issues[] | select(.id == "0002") | .status == "open"' >/dev/null \
    || fail "json: 0002 status mismatch"
pass "json: frontmatter fields preserved (decision_type / closed_at / status)"

# 2) commit action 行 — action 行のあるコミットだけ拾われる（noop は除外）
commit_count="$(echo "$out_json" | jq '.commits | length')"
[ "$commit_count" = "2" ] || fail "json: expected 2 commits with actions, got $commit_count"
pass "json: 2 commits with action lines extracted, noop excluded"

# action 行の中身
all_actions="$(echo "$out_json" | jq -r '.commits[].actions[]' | tr '\n' '|')"
case "$all_actions" in
    *"intent: capture stop signals"*) ;;
    *) fail "json: 'intent: capture stop signals' missing — got: $all_actions" ;;
esac
case "$all_actions" in
    *"learned: subagent termination races with parent"*) ;;
    *) fail "json: 'learned: ...' missing — got: $all_actions" ;;
esac
pass "json: action lines (intent / decision / constraint / learned) captured"

# 3) broader query (ディレクトリ) — narrower path に effect する issue がヒット
out_broad="$("$script" --format=json internal/hook/)"
broad_ids="$(echo "$out_broad" | jq -r '.issues[].id' | sort | tr '\n' ' ')"
[ "$broad_ids" = "0001 0002 " ] \
    || fail "broad query: expected '0001 0002 ', got '$broad_ids'"
pass "broad query: dir 'internal/hook/' matches both 0001 (specific file) and 0002 (same dir)"

# 4) narrower query — broader な affected_paths を持つ issue がヒット
out_narrow="$("$script" --format=json internal/syncdb/foo.go)"
narrow_ids="$(echo "$out_narrow" | jq -r '.issues[].id' | sort | tr '\n' ' ')"
[ "$narrow_ids" = "0001 " ] \
    || fail "narrow query: expected '0001 ', got '$narrow_ids'"
pass "narrow query: 'internal/syncdb/foo.go' matches 0001 (whose affected_paths has 'internal/syncdb/')"

# 5) no-match
out_none="$("$script" --format=json totally/unrelated/path)"
[ "$(echo "$out_none" | jq '.issues | length')" = "0" ] || fail "no-match: issues should be 0"
[ "$(echo "$out_none" | jq '.commits | length')" = "0" ] || fail "no-match: commits should be 0"
pass "no-match path: empty issues + commits"

# 6) markdown 出力 — 構造的なアサーション
out_md="$("$script" internal/hook/stop.go)"
case "$out_md" in
    *"# Intent for"*) ;;
    *) fail "markdown: missing top heading" ;;
esac
case "$out_md" in
    *"## Issues"*) ;;
    *) fail "markdown: missing Issues section" ;;
esac
case "$out_md" in
    *"## Commits"*) ;;
    *) fail "markdown: missing Commits section" ;;
esac
case "$out_md" in
    *"#0001"*"#0002"*) ;;
    *"#0002"*"#0001"*) ;;
    *) fail "markdown: issue ids 0001/0002 not both rendered" ;;
esac
pass "markdown: top-level structure (heading / Issues / Commits / both ids)"

# 7) help / no-arg
"$script" -h >/dev/null 2>&1 || fail "--help should exit 0"
"$script" >/dev/null 2>&1 && fail "no-arg should fail" || true
pass "cli: --help / no-arg behavior"

# 8) frontmatter なし issue は skip された (0002 が拾われ no-frontmatter は混ざらない)
echo "$out_json" | jq -e '[.issues[].path] | any(test("no-frontmatter")) | not' >/dev/null \
    || fail "frontmatter-less issue leaked"
pass "skip: issue without frontmatter is ignored"

echo
echo "All tests passed."
