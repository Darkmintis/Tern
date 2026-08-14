# AGENTS.md — Tern maintainer guide

Instructions for people and AI agents **building Tern itself** (`github.com/darkmintis/Tern`).  
For end-user projects, `tern init` writes a separate Tern section into that project's `AGENTS.md`.

## Architecture rules (protect above all else)

- **Core engine + thin adapters.** New framework support = new package under `internal/adapter/<name>/`. Never fork or duplicate core engine logic (`internal/engine/`, `internal/signing/`) to support a new framework.
- **The `Adapter` interface is the one boundary that must never leak.** If adding a framework requires touching the core engine beyond a genuinely new capability, stop and flag it — that's a design smell. See `docs/adr/0001-adapter-interface.md`.
- **Signing and secrets are shared.** Logic lives once in `internal/signing/` and `internal/secrets/` — never duplicated per-framework.
- Engine, upload, and signing packages **must not import** concrete adapters (Flutter/Native/…). Adapters are registered from `cmd/tern` only.

## Scope discipline (refuse to drift)

**In scope, forever:** Android + iOS release automation for Flutter → Native → KMP → React Native, in that phase order.

**Out of scope, forever:** web deploy, generic backend CI/CD, hosted build infrastructure, a hosted Tern cloud/SaaS.

If a request doesn't fit **"build / sign / publish for Android / iOS,"** it doesn't go in this repo — say so instead of building it.

**Phase gating:** don't add Native/KMP/React Native *production* support while Flutter isn't fully shipped and dogfooded. Scaffold packages may exist with `Detect` off; check `docs/adr/0004-phase-gating-and-non-goals.md` and adapter `Phase` constants before expanding scope. Active supported path: **Flutter**.

## Go coding standards

- `cmd/` stays thin — all logic in `internal/`.
- Errors wrapped with `%w` (and `internal/errors` classes/hints). Never swallowed. No bare `panic()` outside `cmd/`.
- Concurrency via goroutines + `errgroup`; `context.Context` threaded through every long-running operation.
- Structured logging via `log/slog`, not `fmt.Println` for operational logs.
- No global mutable state — config / secrets / engine state passed explicitly.
- Table-driven tests; adapters tested against mocked `os/exec` (`exec.Runner`), never shelling out to real `xcodebuild` / `gradlew` in unit tests.
- `gofmt` + `golangci-lint` clean, no exceptions. Prefer `make ci`.
- CLI routing via Cobra; Ternfile parsing is hand-rolled internal code (`internal/config`), not a generic config framework.
- Conventional Commits (`feat:`, `fix:`, `refactor:`, …), semver starting at `v0.x`.
- Prefer stdlib over third-party deps where reasonable — this tool touches signing keys; dependency bloat is a trust/security concern, not just style.

## Explicit don'ts

- Don't add a check/feature "while you're in there" that isn't the task at hand.
- Don't invent new top-level commands without matching CLI design: verb-based, reads like plain instructions, **lane name = subcommand** (`tern release` → `tern run release` when a `release` lane exists).
- Don't couple Tern to any specific external tool's internals — shell out via a clean subprocess boundary (`exec.Runner`); never import Play/Xcode internals as library dependencies baked into core logic.
- Don't claim "faster than Fastlane" without benchmark results under `benchmarks/`.
- Don't commit secrets, keystores, or service-account JSON.

## Layout map (where to change what)

| Concern | Package |
|---|---|
| CLI | `cmd/tern/` |
| Ternfile IR / parse | `internal/config/` |
| Lane execution | `internal/engine/` |
| Framework builds | `internal/adapter/*` |
| Signing / cert sync hooks | `internal/signing/` |
| Store upload | `internal/upload/` |
| Release name / notes | `internal/releasemeta/` |
| Production gates | `internal/safety/` |
| Init scaffolds | `internal/initcmd/` |
| ADRs | `docs/adr/` |

## Skill copy

A Cursor/Claude skill mirroring this file lives at `.claude/skills/tern-maintainer/SKILL.md`. Keep them consistent when updating architecture rules.
