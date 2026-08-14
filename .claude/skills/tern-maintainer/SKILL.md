---
name: tern-maintainer
description: >-
  Maintainer rules for the Tern CLI repo (github.com/darkmintis/Tern). Use when
  editing Tern itself — adapters, engine, Ternfile, signing, uploads, CI, or
  docs/adr. Enforces core+adapter architecture, phase gating, and Go standards.
---

# Tern maintainer skill

You are working **in the Tern repository** (building the release CLI), not in an end-user mobile app.

## Architecture rules (protect above all else)

- **Core engine + thin adapters.** New framework support = new adapter in `internal/adapter/`. Never fork or duplicate core engine logic (`internal/engine/`, `internal/signing/`) to support a new framework.
- **The Adapter interface is the one boundary that must never leak.** If adding a framework requires touching the core engine beyond a genuinely new capability, stop and flag it — that's a design smell.
- **Signing and secrets logic is shared across all adapters**, lives once in `internal/signing/` and `internal/secrets/` — never duplicated per-framework.
- Core packages must not import concrete adapters; register adapters from `cmd/tern` only.

## Scope discipline (refuse to drift on your own)

- **In scope, forever:** Android + iOS release automation for Flutter → Native → KMP → React Native, in that phase order.
- **Out of scope, forever:** web deploy, generic backend CI/CD, hosted build infrastructure, a hosted Tern cloud/SaaS.
- If a request doesn't fit **"build/sign/publish for Android/iOS,"** it doesn't go in this repo — say so instead of building it.
- **Phase gating:** don't add Native/KMP/React Native production code while Flutter (current phase) isn't fully shipped and dogfooded. Check which phase is active (`docs/adr/0004`, adapter `Phase` constants) before adding scope. Supported path today: **Flutter**.

## Go coding standards

- `cmd/` stays thin — all logic in `internal/`.
- Errors wrapped with `%w`, never swallowed. No bare `panic()` outside `cmd/`.
- Concurrency via goroutines + `errgroup`, `context.Context` threaded through every long-running operation.
- Structured logging via `log/slog`, not `fmt.Println`.
- No global mutable state — config/secrets/engine state passed explicitly.
- Table-driven tests; adapters tested against mocked `os/exec`, never shelling out to real `xcodebuild`/`gradlew` in unit tests.
- `gofmt` + `golangci-lint` clean, no exceptions.
- CLI routing via Cobra; Ternfile parsing is hand-rolled internal code, not a generic config library.
- Conventional Commits (`feat:`, `fix:`, `refactor:`…), semver starting at `v0.x`.
- Prefer stdlib over third-party deps where reasonable — this tool touches signing keys, dependency bloat is a trust/security concern here, not just style.

## Explicit don'ts

- Don't add a check/feature "while you're in there" that isn't the task at hand.
- Don't invent new top-level commands without checking CLI design conventions (verb-based, reads like plain instructions, lane name = subcommand).
- Don't couple Tern to any specific external tool's internals — shell out via a clean subprocess boundary, never imported as a library dependency baked into core logic.
- Don't claim speed vs Fastlane without `benchmarks/` results.
- Don't commit keystores, `.env` secrets, or service-account JSON.

## Where to look

- Root `AGENTS.md` — same rules in longer form
- `docs/adr/` — frozen decisions
- `make ci` — gofmt + lint + test before claiming done
