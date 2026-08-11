# ADR 0006 — Release Engine Principles

**Status:** Accepted  
**Date:** 2026-08-11

## Decision

Tern is an optimized mobile **release engine**, not a generic CI orchestrator.

### Seven advantages (product contract)

1. Smart incremental builds — skip rebuilds when inputs unchanged; never `flutter clean` unless required or `--clean`
2. Intelligent caching — configure CI caches for pub/Gradle/SDK/CocoaPods
3. Parallel release execution — independent Android + iOS builds via `errgroup`
4. Dependency resolution optimization — fingerprint lockfiles; skip redundant resolves
5. Selective platform/artifact builds — only run steps for platforms/kinds the lane needs
6. Artifact-first architecture — build once, `tern ship` retries upload without rebuild
7. Pre-release validation — validate before upload; refuse to ship invalid artifacts

### Benchmark rule (non-negotiable)

Public docs may describe Tern as an “optimized release engine” and list these features.  
Public docs **must not** claim Tern is faster than Fastlane (or any competitor) until:

1. A reproducible harness exists under `benchmarks/`
2. Results are checked in (or linked) and reproduced at least twice on comparable runners

Competitive strategy notes stay in `local/`.

### Phase gate

Release Engine (Phase R) lands on Flutter before Native/KMP/RN expansion. R1 (artifacts + ship) and R2 (validate + parallel) precede Native work.
