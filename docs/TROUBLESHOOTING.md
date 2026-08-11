# Troubleshooting

Tern prints a short `error:` plus a `hint:` when something fails. Prefer fixing the hint before digging through full Gradle logs.

```bash
tern release              # short error + hint (command stderr captured, not dumped)
tern release --verbose    # stream full logs live; print full captured log on failure
TERN_VERBOSE=1 tern release
```

## `tern doctor` fails

Read each failing check and its `hint:`. Common fixes:

| Check | Fix |
|---|---|
| `flutter not found` | Install Flutter; ensure `flutter` is on `PATH` |
| `android_sdk` | Export `ANDROID_HOME` (or `ANDROID_SDK_ROOT`) to your SDK path |
| `jdk` | Install JDK 17+ and set `JAVA_HOME` |
| `android_licenses` | Run `flutter doctor --android-licenses` and accept all |
| `android_cmdline_tools` | Install Android SDK Command-line Tools |
| `android_signing_gradle` | Wire `key.properties` into `android/app/build.gradle` (Flutter default template) |
| `env:ANDROID_KEYSTORE` | Export path to a real keystore file |
| `GOOGLE_APPLICATION_CREDENTIALS` | Download Play service-account JSON and export the path |
| `sync_certs` | Remove that step from Ternfile (not ready in v0) |
| `adapter` | Use a Flutter project (`pubspec.yaml` + Flutter SDK) |

## Build / release failures Tern rewrites

| You see | Typical cause | What to do |
|---|---|---|
| Android SDK licenses not accepted | Licenses never accepted | `flutter doctor --android-licenses` |
| Android SDK / ANDROID_HOME not configured | Missing SDK env | Set `ANDROID_HOME`, install platform + build-tools |
| JDK version incompatible… | JDK too old for AGP | Use JDK 17+ |
| Android keystore password or alias is wrong | Bad signing env | Fix `ANDROID_KEYSTORE_PASSWORD` / alias / key password |
| Release build is not signed | Skipped `sign` step | Run `sign android` before release build |
| Flutter plugin or dependency failed | Pub/plugin compile | `flutter pub get`; pin the failing plugin |
| CocoaPods install failed | iOS pods | `cd ios && pod install --repo-update` |
| iOS code signing or provisioning failed | Xcode team/profile | Fix signing in Xcode; `flutter build ipa` once |
| Play Console denied access | SA lacks app access | Play Console → Users and permissions |
| App package not found in Play Console | App missing / wrong id | Create app or set `ANDROID_PACKAGE_NAME` |
| Play versionCode already used | Duplicate build number | `bump version build`, rebuild, upload |
| Network error talking to store APIs | Offline / proxy | Fix network/VPN; retry |
| App Store Connect API authentication failed | Bad ASC API key | Check key id / issuer / `.p8` path |

## Build fails after sign

1. Confirm `android/key.properties` exists after `sign android`  
2. Confirm Gradle `signingConfigs.release` uses those properties  
3. Run `flutter build appbundle --release` manually and fix Flutter/Gradle errors first  

## Play upload fails

| Symptom | Fix |
|---|---|
| missing credentials | Set `GOOGLE_APPLICATION_CREDENTIALS` |
| permission denied / 403 | Grant the service account access to the app in Play Console |
| wrong package | Set `ANDROID_PACKAGE_NAME` or fix `applicationId` |
| artifact missing | Ensure `build android release` succeeded and produced an `.aab` |
| versionCode clash | Bump build number and rebuild |

## TestFlight upload fails

| Symptom | Fix |
|---|---|
| `xcrun` missing | Use macOS for iOS uploads |
| API key errors | Set `APP_STORE_CONNECT_API_KEY_ID`, `…_ISSUER_ID`, `…_KEY_PATH` |
| no `.ipa` | Fix Xcode signing; `flutter build ipa` must succeed alone |

## Useful flags

```bash
tern doctor                   # cheap preflight (SDK, JDK, licenses, secrets)
tern release --dry-run        # no network / no mutating upload
tern release --json           # machine-readable step events (includes hint)
tern release --verbose        # full flutter/gradle/altool logs
tern run release_prod --yes   # required for Play production / App Store in CI
tern run release_ios --dry-run
```

Production uploads (`track:production` / `app_store`) refuse without `--yes` in CI; interactive terminals prompt instead.

## Exit codes

| Code | Class |
|---|---|
| 2 | ConfigError |
| 3 | DoctorError |
| 4 | BuildError |
| 5 | SignError |
| 6 | UploadError |
| 7 | ExecError |
