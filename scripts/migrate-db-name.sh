#!/usr/bin/env bash
# 使い捨てスクリプト: ~/.claude/hitl-metrics.db → ~/.claude/agent-telemetry.db にリネーム
# 実行後に削除してOK。`agent-telemetry sync-db` / `agent-telemetry backfill` 実行時にも自動で同等の処理が走るため、
# このスクリプトは pre-1.0 環境からの一括移行用途。
set -euo pipefail

CLAUDE_DIR="$HOME/.claude"
CODEX_DIR="${CODEX_HOME:-$HOME/.codex}"

migrate() {
  local src="$1"
  local dst="$2"
  if [ ! -e "$src" ]; then
    return 0
  fi
  if [ -e "$dst" ]; then
    echo "Skip: $dst already exists (remove either side to resolve)" >&2
    return 1
  fi
  mv "$src" "$dst"
  echo "Migrated: $src → $dst"
}

status=0
migrate "$CLAUDE_DIR/hitl-metrics.db" "$CLAUDE_DIR/agent-telemetry.db" || status=$?
migrate "$CLAUDE_DIR/hitl-metrics.db-wal" "$CLAUDE_DIR/agent-telemetry.db-wal" || status=$?
migrate "$CLAUDE_DIR/hitl-metrics.db-shm" "$CLAUDE_DIR/agent-telemetry.db-shm" || status=$?
migrate "$CLAUDE_DIR/hitl-metrics-state.json" "$CLAUDE_DIR/agent-telemetry-state.json" || status=$?
migrate "$CODEX_DIR/hitl-metrics-state.json" "$CODEX_DIR/agent-telemetry-state.json" || status=$?

exit "$status"
