# Benchmarks — Tern vs Fastlane (protocol)

**Status:** harness scaffold only. **No numbers yet.**

Public README may call Tern an “optimized release engine” and list Release Engine features.  
It **must not** claim Tern is faster than Fastlane until this harness has been run **twice** on comparable runners and results are checked in (or linked from here).

See [ADR 0006](../docs/adr/0006-release-engine.md).

## Sample app

Use an identical Flutter sample (same commit) for both tools:

1. Create once: `flutter create --org com.example tern_bench_app`
2. Pin the sample under `benchmarks/sample/` **or** document a git SHA of an external fixture
3. Both Tern and Fastlane lanes must produce the same artifact kind (default: Android release AAB)

## Scenarios

| ID | Scenario | What to measure |
|---|---|---|
| S1 | Cold CI | Clean runner, no caches restored |
| S2 | Warm CI | Second run with pub/Gradle caches warm |
| S3 | Android-only release | Build + (optional dry) upload path |
| S4 | Upload retry without rebuild | Tern: `tern ship last`; Fastlane: re-run upload-only lane |
| S5 | Setup time | `tern init` → first `tern release --dry-run` vs Fastlane Fastfile bootstrap |

## Runner class

Record for every result file:

- OS / image (e.g. `ubuntu-24.04`, `macos-14`)
- CPU / memory (or GHA label)
- Flutter SDK version
- Tern version / Fastlane version
- Date (UTC)

## Scripts (to implement)

```text
benchmarks/
  README.md          # this file
  scripts/
    run_tern.sh      # times Tern lane; writes JSON
    run_fastlane.sh  # times Fastlane lane; writes JSON
    summarize.py     # merges JSON → markdown table
  results/           # checked-in JSON after real runs (empty until then)
```

### JSON result shape

```json
{
  "scenario": "S2",
  "tool": "tern",
  "wall_clock_sec": 0,
  "runner": "ubuntu-24.04",
  "flutter": "3.x.x",
  "tool_version": "0.x.x",
  "reproduced": 1,
  "notes": ""
}
```

## Rule

Do not invent wall-clock numbers. Leave `results/` empty or with `README` placeholders until measured.
