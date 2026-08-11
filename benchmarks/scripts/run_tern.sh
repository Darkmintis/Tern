#!/usr/bin/env bash
# Time a Tern release lane. Writes JSON to stdout or -o FILE.
# Usage: ./run_tern.sh [--dir PROJECT] [--lane release] [-o out.json]
set -euo pipefail
DIR="."
LANE="release"
OUT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir) DIR="$2"; shift 2 ;;
    --lane) LANE="$2"; shift 2 ;;
    -o) OUT="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done
START=$(date +%s)
tern --dir "$DIR" run "$LANE"
END=$(date +%s)
WALL=$((END - START))
VER=$(tern version 2>/dev/null || echo "unknown")
FLUTTER=$(flutter --version 2>/dev/null | head -1 || echo "unknown")
JSON=$(cat <<EOF
{"scenario":"manual","tool":"tern","wall_clock_sec":${WALL},"runner":"$(uname -s)","flutter":$(printf '%s' "$FLUTTER" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))'),"tool_version":$(printf '%s' "$VER" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))'),"reproduced":1,"notes":"fill scenario id after run"}
EOF
)
if [[ -n "$OUT" ]]; then
  printf '%s\n' "$JSON" >"$OUT"
  echo "wrote $OUT (${WALL}s)"
else
  printf '%s\n' "$JSON"
fi
