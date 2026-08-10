# ADR 0003 — Signing & Secrets Threat Model

**Status:** Accepted  
**Date:** 2026-08-10

## Threats

1. Accidental commit of keystores / P12 / API keys in Ternfile or git
2. Logging secret values in CI logs
3. Weak / default passwords (`changeme`, empty, `secret`)
4. Expired iOS provisioning profiles / certs causing late failures
5. Supply-chain trust: users install a binary that handles signing keys

## Controls (v0 / v1)

- Secrets referenced only as `env:NAME` or local file paths from env — never inline in Ternfile
- `tern doctor` checks: missing env, weak patterns, file readability, profile expiry
- `log/slog` redacts values that look like secrets / env-resolved secret contents
- No hosted secrets vault in v1 (permanent non-goal until a separate product decision)
- Cert sync (Phase 1.5) uses age-style encryption over a git/S3 backend — design hook in `internal/signing/sync.go`; no custom crypto invention
- Release binaries published with checksums from GitHub Actions

## Non-goals

- Tern never stores secrets server-side
- Tern never phones home with project or key material
