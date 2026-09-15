# Release Notes

## v0.1.0 (2026-09-15)

First public release of Tern — mobile release automation for Flutter.

### Features
- `tern doctor` — verify toolchain, signing keys, and secrets
- `tern build` — build Android/iOS artifacts
- `tern release` — full release lane (bump, sign, build, upload)
- `tern promote` — promote between Play Store tracks
- `tern rollback` — rollback to previous version
- `tern status` — check current release status
- `tern commit` / `tern tag` — local git commit and tag
- Human-friendly colored output with ANSI icons
- Verbose error details with `--verbose` flag
- Telegram notifications on release success/failure
- Ternfile DSL for defining custom release lanes
- Flutter adapter with Android + iOS support

### Supported Platforms
- Android (Play Store)
- iOS (App Store) — coming soon

### Install
```bash
# macOS / Linux
curl -sSL https://raw.githubusercontent.com/darkmintis/Tern/main/install/install.sh | sh

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/darkmintis/Tern/releases/latest/download/tern-windows-amd64.exe" -OutFile "tern.exe"
```

### Quick Start
```bash
tern doctor          # verify setup
tern init            # create Ternfile in your project
tern release         # run release lane
```
