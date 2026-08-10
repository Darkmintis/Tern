# Tern

Open-source **mobile release automation CLI** for Android & iOS.

**v0 supports Flutter end-to-end** (bump → sign → build → Play / TestFlight).  
Native, KMP, and React Native packages are scaffolds for later — see [`docs/adapters.md`](docs/adapters.md).

```bash
tern init
tern doctor
tern release --dry-run
tern release
```

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

```yaml
- uses: darkmintis/Tern/action/setup-tern@main
- run: tern doctor
- run: tern release
```

## License

[MIT](LICENSE) · Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
