#!/usr/bin/env bash
# Times a Tern lane; writes JSON to benchmarks/results/.
# Usage: ./run_tern.sh S2 /path/to/flutter_app release
set -euo pipefail
SCENARIO="${1:?scenario id e.g. S2}"
APP_ROOT="${2:?flutter app root}"
LANE="${3:-release}"
OUT_DIR="$(cd "$(dirname "$0")/../results" && pwd)"
mkdir -p "$OUT_DIR"
START=$(date +%s)
( cd "$APP_ROOT" && tern run "$LANE" --dry-run )
END=$(date +%s)
WALL=$((END - START))
VER=$(tern version 2>/dev/null || tern --version 2>/dev/null || echo unknown)
FILE="$OUT_DIR/tern_${SCENARIO}_$(date -u +%Y%m%dT%H%M%SZ).json"
cat > "$FILE" <<JSON
{
  "scenario": "$SCENARIO",
  "tool": "tern",
  "wall_clock_sec": $WALL,
  "runner": "$(uname -s)-$(uname -m)",
  "flutter": "$(flutter --version 2>/dev/null | head -1 | tr -d '\n' || true)",
  "tool_version": "$VER",
  "reproduced": 1,
  "notes": "dry-run lane=$LANE"
}
JSON
echo "wrote $FILE (${WALL}s)"
