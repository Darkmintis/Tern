# Contributing to Tern

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` new user-facing capability
- `fix:` bug fix
- `refactor:` internal change without behavior change
- `test:` tests only
- `docs:` documentation
- `chore:` tooling / CI / deps

Examples:

```
feat: add flutter appbundle dry-run paths
fix: reject unknown Ternfile step kinds
```

## Code standards

- `gofmt` and `golangci-lint` must be clean
- Prefer stdlib; keep the dependency tree small (signing-key trust surface)
- New frameworks = new adapter under `internal/adapter/<name>/` — never fork the engine
- Wrap errors with `%w` and use `internal/errors` classes
- Table-driven tests for parsers and validators; mock `exec` at the Runner boundary

See [`docs/adr/`](docs/adr/) for architecture decisions and [`docs/getting-started.md`](docs/getting-started.md) for user docs.

## Pull requests

Run the same gates as GitHub Actions locally:

```bash
make install-lint   # once
make ci             # gofmt + golangci-lint v2.12.2 + go test
```

Or step by step:

1. `make fmt`
2. `make test`
3. `make lint`
4. Describe the *why* in the PR body
