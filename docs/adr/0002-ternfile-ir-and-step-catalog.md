# ADR 0002 — Ternfile IR and v0 Step Catalog

**Status:** Accepted  
**Date:** 2026-08-10

## Decision

Both the Ternfile DSL and `ternfile.yaml` compile to one internal representation (`internal/config.Config`). There is a single execution engine.

### IR (canonical)

- `Config` → map of named `Lane`s
- `Lane` → ordered list of `Step`s
- `Step` → `Kind` + typed fields (`Platform`, `Mode`, `EnvRef`, `Track`, `Target`, args)

### v0 step catalog (closed set)

| Kind | DSL shape | Notes |
|---|---|---|
| `build` | `build <android\|ios> <debug\|release> [aab\|apk] [flavor:<name>] [scheme:<name>]` | Adapter build; android release defaults to aab; flavor/scheme → Flutter `--flavor` |
| `sign` | `sign <android\|ios> with <keystore\|cert> env:<NAME>` | Shared signing |
| `upload` | `upload <android\|ios> to <play_store\|testflight\|app_store> [track:<name>] [rollout:<pct>] [release_name:…] [notes:…]` | Shared upload (validates first; production needs `--yes`) |
| `ship` | `ship <android\|ios> from <last\|path> to <target> [track:<name>] [rollout:<pct>] [release_name:…] [notes:…]` | Upload saved artifact without rebuild |
| `bump` | `bump version [major\|minor\|patch\|build]` | Phase 1.5 |
| `tag` | `tag git [prefix:v]` | Phase 1.5 |
| `sync_certs` | `sync_certs <pull\|push> [repo:env:CERT_REPO]` | Phase 1.5; encrypted cert sync |
| `notify` | `notify <slack\|discord> env:<WEBHOOK>` | Reserved; implement post-adoption |

Unknown step kinds are hard errors at parse time.

### Grammar notes (DSL)

- Comments: `#` to end of line
- Lanes: `lane <name>:` then indented steps (2 spaces)
- `env:NAME` references environment variables only — never literal secrets
- YAML escape hatch: `ternfile.yaml` with equivalent structure

## Compatibility

Ternfile grammar is `v0` until 1.0. Additive steps may appear in minor releases; removing or renaming a step kind requires a major version after 1.0.
