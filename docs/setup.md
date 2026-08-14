# Setup guide (Flutter → Play)

Simple path for developers. Tern runs from your **Flutter app root** — not inside `android/` and not in a `fastlane/`-style folder.

## Project layout

```text
your_flutter_app/                    ← run tern here
├── Ternfile                         ← lanes (commit)
├── AGENTS.md                        ← Tern section for AI assistants (commit)
├── .env.example                     ← from tern init (commit)
├── .env                             ← secrets (do NOT commit)
├── secrets/                         ← keystore + Play JSON (do NOT commit)
│   ├── upload.jks
│   └── play.json
├── .tern/                           ← release state (gitignore)
│   └── artifacts/
├── .github/
│   └── workflows/
│       └── tern-release.yml         ← from tern init (commit) — CI only
├── android/
├── ios/
└── pubspec.yaml
```

The GitHub Actions file is always under **`.github/workflows/`** (standard GitHub path). It is not under `android/`.

## First time — what you do by hand vs what Tern does

| One-time (you, in Play Console) | Every release (Tern) |
|---|---|
| Create the app (same `applicationId` as Gradle) | `sign` → `build` AAB → upload → tag |
| Create keystore + service account JSON | Uses `.env` / CI secrets |
| Grant the service account access to the app | Uploads to **internal** (default) |
| Finish any Play “complete your setup” forms if Console blocks uploads | Retry with `tern ship` if needed |

**Do you need to upload an AAB manually in the Console first?**  
Usually **no**. After the app exists and API access is set, Tern can be your **first** upload to the **internal** track.

You **do** need to create the app in Play Console once (browser). Tern cannot create the Play listing for you.

Then forever: `tern doctor` → `tern release` (or CI workflow).

## 5-step workflow

### 1. Install Tern

```bash
curl -sSL https://raw.githubusercontent.com/darkmintis/Tern/main/install/install.sh | sh
# or: go install github.com/darkmintis/Tern/cmd/tern@latest
tern version
```

### 2. Init in your app

```bash
cd /path/to/your_flutter_app
tern init
```

Creates:

- `Ternfile` — release lanes (default upload → Play **internal**)
- `.env.example` — which variables you need
- `secrets/` + `.gitignore` entries for `.env`, `.tern/`, `secrets/`
- `AGENTS.md` — Tern section for AI assistants (appends if file already exists)
- `.github/workflows/tern-release.yml` — optional CI (under `.github/workflows/`)

### 3. Add credentials (one-time)

1. Copy the template: `cp .env.example .env`
2. Follow **[Play credentials setup](play-setup.md)** (keystore + service account JSON)
3. Put files under `secrets/` and point paths in `.env`
4. Create the app in Play Console if it does not exist yet (same package id)

Tern also auto-loads `.env` from the project root when you run commands.

Tern **cannot** download Play JSON or invent your keystore — Google requires you to create those in the Console once.

### 4. Check

```bash
tern doctor
```

Fix any failing check using its `hint:` (or [TROUBLESHOOTING](TROUBLESHOOTING.md)).

### 5. Release

```bash
tern release --dry-run    # plan only — no upload
tern release              # bump → sign → build AAB → Play internal → tag
```

If upload fails after a good build:

```bash
tern ship last --to play_store --track internal
```

Production later:

```bash
tern run release_prod --yes    # CI needs --yes
```

## CI (GitHub Actions)

After `tern init`, open **Actions** on GitHub and run the **Release** workflow  
(file: `.github/workflows/tern-release.yml`). Add the same secrets as repo secrets (see the workflow comments).

Local and CI use the same `Ternfile` — only how secrets are injected differs.

## What Tern does *not* do

- Create the Play Console app for you (do that once in the browser)
- Install the app on your phone (use Play **Internal testing** after upload)
- Auto-fetch secrets from Google

## Next

- Credentials: [play-setup.md](play-setup.md)  
- Env reference: [ENV.md](ENV.md)  
- Full feature notes: [getting-started.md](getting-started.md)  
- Troubleshooting: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)  
