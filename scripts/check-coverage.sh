#!/usr/bin/env bash
# Fail CI if aggregate coverage falls below the gate or any package is untested
# below the per-package floor. Generates a coverage profile for artifacts too.
set -euo pipefail

TOTAL_MIN="${TOTAL_MIN:-60}"
PKG_MIN="${PKG_MIN:-35}"

profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT

# Regenerate profile; per-package numbers come from the same run.
go test -coverprofile="$profile" ./internal/... ./cmd/... 2>/dev/null
cov_out="$(go test -cover ./internal/... ./cmd/... 2>/dev/null || true)"

total="$(go tool cover -func="$profile" | awk '/total:/ {print $3}' | tr -d '%')"
if [ -z "$total" ]; then
    echo "error: could not compute total coverage"
    exit 1
fi

echo "aggregate coverage: ${total}% (gate >= ${TOTAL_MIN}%)"
if awk "BEGIN { exit !($total >= $TOTAL_MIN) }"; then
    echo "aggregate coverage gate passed"
else
    echo "error: aggregate coverage ${total}% is below gate ${TOTAL_MIN}%"
    exit 1
fi

lowest=100
below=""
while IFS= read -r line; do
    pct="$(printf '%s\n' "$line" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p')"
    [ -z "$pct" ] && continue
    pkg="$(printf '%s\n' "$line" | awk '{print $2}')"
    if awk "BEGIN { exit !($pct < $lowest) }"; then
        lowest="$pct"
    fi
    if awk "BEGIN { exit !($pct < $PKG_MIN) }"; then
        below="${below}\n  ${pkg}: ${pct}%"
    fi
done <<<"$cov_out"

echo "lowest package coverage: ${lowest}% (floor ${PKG_MIN}%)"
if [ -n "$below" ]; then
    echo -e "error: these packages are below the ${PKG_MIN}% coverage floor:$below"
    cp "$profile" coverage.out
    exit 1
fi
cp "$profile" coverage.out
echo "coverage check passed (profile saved to coverage.out)"