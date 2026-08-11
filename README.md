# Tern

**Mobile release automation CLI** for Android & iOS — an optimized **release engine** to build, sign, and ship apps to Google Play and the App Store / TestFlight from one simple config (Ternfile).

**v0 supports Flutter end-to-end** (bump → sign → build → validate → Play / TestFlight).  
Native, KMP, and React Native packages are scaffolds for later — see [`docs/adapters.md`](docs/adapters.md).

```bash
tern init
tern doctor
tern release --dry-run
tern release
# retry upload without rebuild:
tern ship last --to play_store
```

## Release Engine

Tern focuses on the path from commit → store:

- **Selective builds** — only platforms your lane needs; AAB by default (APK on request)
- **Artifact-first** — successful builds land in `.tern/artifacts/`; `tern ship` retries upload without `flutter build`
- **Release name & notes** — Play release title from app version; notes default to “Bug fixes and improvements.” (custom/file supported)
- **Pre-release validation** — `tern validate` / upload gate checks version, artifact, credentials
- **Parallel builds** — independent Android + iOS builds run concurrently
- **CI caching** — `tern cache --github-actions` (also baked into `tern init` workflows)
- **Dependency skip** — unchanged lockfiles → `--no-pub` on Flutter builds
- **Incremental builds** — unchanged inputs reuse the last artifact (never `flutter clean` unless `--clean`)

See [ADR 0006](docs/adr/0006-release-engine.md). Benchmark protocol: [`benchmarks/`](benchmarks/) (no public speed comparisons until results exist).

## Install

```bash
go install github.com/darkmintis/Tern/cmd/tern@latest
# or
curl -sSL https://raw.githubusercontent.com/darkmintis/Tern/main/install/install.sh | sh
```

Build-from-source requires **Go 1.25.12+**.

## Quick start

Full walkthrough: [`docs/getting-started.md`](docs/getting-started.md)  
Env vars: [`docs/ENV.md`](docs/ENV.md)  
Troubleshooting: [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md)  
**Copy-paste configs:** [`examples/`](examples/)

```
lane release:
  bump version patch
  sign android with keystore env:ANDROID_KEYSTORE
  build android release
  upload android to play_store track:internal
  tag git prefix:v
```

## GitHub Actions

CI-ready release automation for mobile apps:

```yaml
- uses: darkmintis/Tern/action/setup-tern@main
- run: tern doctor
- run: tern release
```

## License

[MIT](LICENSE) · Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
