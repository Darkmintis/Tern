# ADR 0004 — Phase Gating and Non-Goals

**Status:** Accepted  
**Date:** 2026-08-10

## Frozen decisions

1. Module: `github.com/darkmintis/Tern`, binary `tern`
2. CLI: Cobra + `log/slog`
3. Config: DSL primary, YAML secondary, one IR
4. Phase gate: Flutter must be proven on a real release before treating Native/KMP/RN as production-ready
5. Phase 1.5 (cert sync, bump/tag, GHA scaffold, `--dry-run`/`--json`) sits between Flutter proof and Native expansion
6. Cert sync: architecture hooks in the signing package early; full implementation in Phase 1.5

## Explicit non-goals (forever unless this ADR is superseded)

- Web app deployment
- Generic/backend CI/CD
- Hosting build infrastructure / Tern cloud runners
- Desktop app builds (macOS/Windows apps)
- Hosted SaaS dashboard / secrets vault in v1

## Dogfood contract

Phase 2 is not “done” until at least one real app ships a signed store upload via `tern release`. Adapter packages for Native/KMP/RN may exist behind the interface earlier for scaffolding, but Phase 1 Flutter remains the supported path.
