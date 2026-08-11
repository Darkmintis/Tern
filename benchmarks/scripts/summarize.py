#!/usr/bin/env python3
"""Merge benchmarks/results/*.json into a markdown summary table. No invented numbers."""
from __future__ import annotations

import json
import sys
from pathlib import Path


def main() -> int:
    root = Path(__file__).resolve().parents[1] / "results"
    rows = []
    for p in sorted(root.glob("*.json")):
        rows.append(json.loads(p.read_text()))
    if not rows:
        print("No JSON results yet. Run the protocol in benchmarks/README.md first.")
        return 0
    print("| scenario | tool | wall_clock_sec | runner | reproduced |")
    print("|---|---|---:|---|---:|")
    for r in rows:
        print(
            f"| {r.get('scenario','')} | {r.get('tool','')} | {r.get('wall_clock_sec','')} | "
            f"{r.get('runner','')} | {r.get('reproduced','')} |"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
