# iOS Setup

Set up Tern to build and upload iOS apps to TestFlight.

## Prerequisites

- **macOS** (required for iOS builds)
- **Xcode** installed and signed in
- **Apple Developer Account** ($99/year)
- **Flutter** configured for iOS

## Quick Setup (5 minutes)

### 1. Export Certificate

1. Open **Keychain Access** on your Mac
2. Find your **iOS Distribution** certificate
3. Right-click → **Export** → `.p12` format
4. Set a password (remember it!)

Save as `secrets/certificate.p12`

### 2. Export Provisioning Profile

1. Go to [Apple Developer Portal](https://developer.apple.com/account/resources/profiles/list)
2. Download your **iOS Distribution** provisioning profile
3. Save as `secrets/profile.mobileprovision`

### 3. Configure Tern

Add to `.env`:

```bash
# Certificate
IOS_CERTIFICATE=secrets/certificate.p12
IOS_CERTIFICATE_PASSWORD=your-p12-password

# Provisioning Profile
IOS_PROVISIONING_PROFILE=secrets/profile.mobileprovision

# Apple Team ID (found in Apple Developer Portal → Membership)
IOS_TEAM_ID=ABC123DEF4
```

### 4. Test

```bash
tern doctor  # Should show iOS signing validated
```

## How It Works

When you run `tern run release_ios`:

1. **Create keychain** — Temporary keychain for CI (not your login keychain)
2. **Import certificate** — `.p12` imported to keychain
3. **Validate profile** — Check expiry and validity
4. **Build** — `flutter build ipa --release`
5. **Upload** — Send to TestFlight via App Store Connect API

## Keychain Management

Tern automatically manages a temporary keychain:

```
~/Library/Keychains/tern-signing.keychain-db
```

- Created on each release
- Deleted after build completes
- Never touches your login keychain

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `IOS_CERTIFICATE` | ✅ | Path to `.p12` certificate |
| `IOS_CERTIFICATE_PASSWORD` | ✅ | Password for `.p12` file |
| `IOS_PROVISIONING_PROFILE` | ✅ | Path to `.mobileprovision` |
| `IOS_TEAM_ID` | ✅ | Apple Developer Team ID |
| `IOS_KEYCHAIN_PATH` | ❌ | Custom keychain path (default: `~/Library/Keychains/tern-signing.keychain-db`) |
| `IOS_KEYCHAIN_PASSWORD` | ❌ | Keychain password (default: auto-generated) |

## CI Setup (GitHub Actions)

Add secrets to your repository:

```
Settings → Secrets and variables → Actions

IOS_CERTIFICATE_BASE64=<base64 of .p12>
IOS_CERTIFICATE_PASSWORD=your-p12-password
IOS_PROVISIONING_PROFILE_BASE64=<base64 of .mobileprovision>
IOS_TEAM_ID=ABC123DEF4
```

Encode files:
```bash
base64 -i certificate.p12 | pbcopy
base64 -i profile.mobileprovision | pbcopy
```

## Common Issues

| Problem | Fix |
|---------|-----|
| "security: SecKeychainOpen: The specified keychain could not be found" | Delete `~/Library/Keychains/tern-signing.keychain-db` and retry |
| "No signing certificate" | Ensure certificate is in Keychain Access |
| "Provisioning profile not valid" | Re-download from Apple Developer Portal |
| "flutter: command not found" | Ensure Flutter is in PATH |

## Verification

```bash
# Check signing material
tern doctor

# Dry-run to see what would happen
tern run release_ios --dry-run

# Real release
tern run release_ios
```

## Production Release

```bash
# Upload to TestFlight
tern run release_ios

# Then promote to App Store via App Store Connect (browser)
```
