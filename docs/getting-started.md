# Getting started (Flutter)

Tern v0 automates **Flutter** Android & iOS release lanes: bump → sign → build → upload.

## 1. Install

```bash
go install github.com/darkmintis/Tern/cmd/tern@latest
# or
curl -sSL https://raw.githubusercontent.com/darkmintis/Tern/main/install/install.sh | sh

tern version
```

Requires **Go 1.25.12+** to build from source.

## 2. Init in your Flutter app

```bash
cd /path/to/your_flutter_app
tern init
tern doctor
```

This creates a `Ternfile` and optionally `.github/workflows/tern-release.yml`.

## 3. Android → Play Store

### Env vars

See [ENV.md](ENV.md).

### One-time project setup

Your `android/app/build.gradle` must load `key.properties` (Flutter’s usual template). Tern’s `sign` step writes `android/key.properties` from env.

### Release

```bash
tern doctor
tern release --dry-run
tern release
```

Default `release` lane:

1. bump `pubspec.yaml` patch  
2. write `android/key.properties`  
3. `flutter build appbundle --release`  
4. upload AAB to Play `internal` track  
5. git tag  

## 4. iOS → TestFlight

Requires **macOS + Xcode** with signing already working for `flutter build ipa`.

```bash
tern run release_ios --dry-run
tern run release_ios
```

## 5. CI

```yaml
- uses: actions/checkout@v4
- uses: subosito/flutter-action@v2
- uses: darkmintis/Tern/action/setup-tern@main
# or: tern cache --github-actions
- run: tern doctor
- run: tern release
```

## 6. Ship without rebuild

After a successful build, Tern stores metadata under `.tern/artifacts/`. If Play upload fails:

```bash
tern validate --to play_store --artifact last
tern ship last --to play_store --track internal
```

## 7. Release name & notes

By default Tern sets the Play release name from your app version and notes to `Bug fixes and improvements.`  
Override per upload — see [release-name-and-notes.md](release-name-and-notes.md).

## Next

- Env reference: [ENV.md](ENV.md)  
- Troubleshooting: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)  
- Example configs: [../examples/](../examples/)  
- Release Engine: [adr/0006-release-engine.md](adr/0006-release-engine.md)  
- Future adapters: [adapters.md](adapters.md)  
