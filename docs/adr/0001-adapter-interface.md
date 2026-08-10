# ADR 0001 — Adapter Interface Boundary

**Status:** Accepted  
**Date:** 2026-08-10

## Decision

Framework support is added only via implementations of `internal/adapter.Adapter`. The core engine, signing, secrets, and upload packages never import a concrete framework adapter.

```go
type Adapter interface {
    Name() string
    Detect(projectRoot string) bool
    Build(ctx context.Context, opts BuildOptions) (BuildArtifact, error)
}
```

Adapters report artifact paths only. Signing and store upload remain in shared core packages.

## Consequences

- New frameworks = new package under `internal/adapter/<name>/` + registry registration.
- If adding a framework requires forking `internal/engine/` or `internal/signing/`, stop and redesign.
- Adapters are injected through a registry; unit tests use mocks / fake `os/exec` runners.
