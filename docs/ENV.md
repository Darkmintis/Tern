# Environment variables

Secrets are **never** written into `Ternfile`. Reference them with `env:NAME` only.

**Setup guides:** [setup.md](setup.md) · [play-setup.md](play-setup.md)

Local tip: keep values in `.env` at the Flutter app root (`tern init` writes `.env.example`). Tern loads `.env` automatically; existing shell/CI variables are not overwritten.

## Android / Play

| Variable | Required for | Description |
|---|---|---|
| `ANDROID_KEYSTORE` | `sign android` | Absolute or relative path to `.jks` / `.keystore` |
| `ANDROID_KEYSTORE_PASSWORD` | `sign android` | Keystore password |
| `ANDROID_KEY_ALIAS` | `sign android` | Key alias |
| `ANDROID_KEY_PASSWORD` | `sign android` | Key password |
| `GOOGLE_APPLICATION_CREDENTIALS` | `upload … play_store` | Path to Play Console service-account JSON |
| `ANDROID_PACKAGE_NAME` | optional | Override package id if Gradle detection fails |

## iOS / TestFlight

| Variable | Required for | Description |
|---|---|---|
| `IOS_CERT` | `sign ios` | Path to cert/profile material you want doctor to validate |
| `APP_STORE_CONNECT_API_KEY_ID` | `upload … testflight` | ASC API key id |
| `APP_STORE_CONNECT_API_ISSUER_ID` | `upload … testflight` | ASC issuer id |
| `APP_STORE_CONNECT_API_KEY_PATH` | `upload … testflight` | Path to `AuthKey_XXX.p8` |

Xcode must already be able to codesign the app for `flutter build ipa`.

## Example (shell)

```bash
export ANDROID_KEYSTORE="$PWD/secrets/upload.jks"
export ANDROID_KEYSTORE_PASSWORD='…'
export ANDROID_KEY_ALIAS=upload
export ANDROID_KEY_PASSWORD='…'
export GOOGLE_APPLICATION_CREDENTIALS="$PWD/secrets/play.json"

tern doctor
tern release
```
