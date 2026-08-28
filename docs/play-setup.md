# Play Store Setup

Set up Tern to upload Android builds to Google Play Console.

## Prerequisites

- **App created** in [Play Console](https://play.google.com/console)
- **Google Cloud project** linked to Play Console
- **Service account** with Play API access

## Quick Setup (10 minutes)

### 1. Create Upload Keystore

If you don't have a keystore:

```bash
keytool -genkey -v \
  -keystore secrets/upload.jks \
  -keyalg RSA -keysize 2048 \
  -validity 10000 \
  -alias upload
```

Save as `secrets/upload.jks`

### 2. Create Service Account

1. Go to [Play Console](https://play.google.com/console) → **Setup → API access**
2. Click **Create new project** or **Link existing project**
3. Click **Create service account**
4. Service account name: `tern-release`
5. Role: **Service Account User**
6. Copy the JSON key file to `secrets/play.json`

### 3. Grant Permissions

1. Back in Play Console → **API access**
2. Find your service account → **Grant access**
3. Add these permissions:
   - ✅ **Release apps to testing tracks** (internal, alpha, beta)
   - ✅ **Release to production** (optional, for production uploads)

### 4. Configure Tern

Add to `.env`:

```bash
# Keystore
ANDROID_KEYSTORE=secrets/upload.jks
ANDROID_KEYSTORE_PASSWORD=your-keystore-password
ANDROID_KEY_ALIAS=upload
ANDROID_KEY_PASSWORD=your-key-password

# Play Console
GOOGLE_APPLICATION_CREDENTIALS=secrets/play.json
```

### 5. Test

```bash
tern doctor  # Should show all green
tern run release_internal --dry-run  # Preview what would happen
```

## How It Works

When you run `tern release_internal`:

1. **Bump version** — Increment patch version in `pubspec.yaml`
2. **Sign** — Write `android/key.properties` from env vars
3. **Build** — `flutter build appbundle --release`
4. **Validate** — Check version code is ahead of Play
5. **Upload** — Send AAB to Play Console via API
6. **Tag** — Create git tag `v1.2.3`

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANDROID_KEYSTORE` | ✅ | Path to `.jks` keystore file |
| `ANDROID_KEYSTORE_PASSWORD` | ✅ | Keystore password |
| `ANDROID_KEY_ALIAS` | ✅ | Key alias (usually `upload`) |
| `ANDROID_KEY_PASSWORD` | ✅ | Key password |
| `GOOGLE_APPLICATION_CREDENTIALS` | ✅ | Path to service account JSON |
| `ANDROID_PACKAGE_NAME` | ❌ | App package ID (auto-detected) |

## CI Setup (GitHub Actions)

### Option A: Base64 encoded keystore

Add secrets:
```
ANDROID_KEYSTORE_BASE64=<base64 of upload.jks>
ANDROID_KEYSTORE_PASSWORD=your-password
ANDROID_KEY_ALIAS=upload
ANDROID_KEY_PASSWORD=your-password
GOOGLE_APPLICATION_CREDENTIALS_PATH=secrets/play.json
```

Encode keystore:
```bash
base64 -i secrets/upload.jks | pbcopy
```

### Option B: Use gh secret set

```bash
gh secret set ANDROID_KEYSTORE < secrets/upload.jks
gh secret set ANDROID_KEYSTORE_PASSWORD
gh secret set ANDROID_KEY_ALIAS
gh secret set ANDROID_KEY_PASSWORD
gh secret set GOOGLE_APPLICATION_CREDENTIALS < secrets/play.json
```

## Track Configuration

| Track | Command | Description |
|-------|---------|-------------|
| Internal | `track:internal` | Default, for testing |
| Alpha | `track:alpha` | Limited testing |
| Beta | `track:beta` | Wider testing |
| Production | `track:production` | Public release |

### Staged Rollout

```bash
# 10% rollout
tern run release_prod --yes
# or
tern upload android to play_store track:production rollout:10
```

## Version Code Rules

- Play Console requires each upload to have a **higher** version code
- Tern checks this before upload and warns if behind
- Use `bump version build` to increment only the version code

## Common Issues

| Problem | Fix |
|---------|-----|
| "403 Permission denied" | Grant service account access in Play Console |
| "Version code too low" | Bump version code higher than current Play version |
| "Package not found" | App not created yet, or wrong `applicationId` |
| "Invalid keystore" | Check keystore path and password |
| "Service account not found" | Re-download JSON from Google Cloud IAM |

## Verification

```bash
# Full check
tern doctor

# Dry-run
tern run release_internal --dry-run

# Real release
tern run release_internal
```

## Production Release

```bash
# First: release to internal
tern run release_internal

# Then: promote to production
tern promote internal production
# or
tern run release_prod --yes
```
