#!/usr/bin/env bash
set -euo pipefail

OUTDIR="${1:-.outputs/grafana-screenshots}"
mkdir -p "$OUTDIR"

BASE="http://localhost:13000"
# 2026-03-01T00:00:00Z → 1772323200000 ms
# 2026-03-17T00:00:00Z → 1773705600000 ms
FROM=1772323200000
TO=1773705600000
WIDTH=1000
HEIGHT=500
TZ="Asia/Tokyo"

declare -a PANELS=(
  "1:summary"
  "2:pr-table"
  "3:perm-rate-by-pr"
  "4:perm-count-by-pr"
  "5:session-count-by-pr"
  "6:mid-session-msgs-by-pr"
  "7:ask-user-question-by-pr"
  "8:perm-rate-daily-trend"
  "9:perm-rate-weekly-trend"
  "10:tool-breakdown-table"
  "11:tool-breakdown-bar"
)

for entry in "${PANELS[@]}"; do
  ID="${entry%%:*}"
  NAME="${entry##*:}"
  OUTFILE="${OUTDIR}/panel-${ID}-${NAME}.png"
  URL="${BASE}/render/d-solo/claudedog/claudedog?panelId=${ID}&from=${FROM}&to=${TO}&width=${WIDTH}&height=${HEIGHT}&tz=${TZ}"
  echo "Capturing panel ${ID} (${NAME})..."
  curl -sf -o "$OUTFILE" "$URL"
  echo "  → ${OUTFILE}"
done

echo "Done: ${#PANELS[@]} panels captured in ${OUTDIR}"
