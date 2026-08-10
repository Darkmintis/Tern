# Troubleshooting

## `tern doctor` fails

Read each failing check and its `hint:`. Common fixes:

| Check | Fix |
|---|---|
| `flutter not found` | Install Flutter; ensure `flutter` is on `PATH` |
| `android_signing_gradle` | Wire `key.properties` into `android/app/build.gradle` (Flutter default template) |
| `env:ANDROID_KEYSTORE` | Export path to a real keystore file |
| `GOOGLE_APPLICATION_CREDENTIALS` | Download Play service-account JSON and export the path |
| `sync_certs` | Remove that step from Ternfile (not ready in v0) |
| `adapter` | Use a Flutter project (`pubspec.yaml` + Flutter SDK) |

## Build fails after sign

1. Confirm `android/key.properties` exists after `sign android`  
2. Confirm Gradle `signingConfigs.release` uses those properties  
3. Run `flutter build appbundle --release` manually and fix Flutter/Gradle errors first  

## Play upload fails

| Symptom | Fix |
|---|---|
| missing credentials | Set `GOOGLE_APPLICATION_CREDENTIALS` |
| permission denied | Grant the service account access to the app in Play Console |
| wrong package | Set `ANDROID_PACKAGE_NAME` or fix `applicationId` |
| artifact missing | Ensure `build android release` succeeded and produced an `.aab` |

## TestFlight upload fails

| Symptom | Fix |
|---|---|
| `xcrun` missing | Use macOS for iOS uploads |
| API key errors | Set `APP_STORE_CONNECT_API_KEY_ID`, `…_ISSUER_ID`, `…_KEY_PATH` |
| no `.ipa` | Fix Xcode signing; `flutter build ipa` must succeed alone |

## Useful flags

```bash
tern release --dry-run          # no network / no mutating upload
tern release --json             # machine-readable step events
tern run release_ios --dry-run
```

## Exit codes

| Code | Class |
|---|---|
| 2 | ConfigError |
| 3 | DoctorError |
| 4 | BuildError |
| 5 | SignError |
| 6 | UploadError |
| 7 | ExecError |
