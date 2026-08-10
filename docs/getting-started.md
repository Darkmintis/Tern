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
- run: tern doctor
- run: tern release
```

## Next

- Env reference: [ENV.md](ENV.md)  
- Troubleshooting: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)  
- Example configs: [../examples/](../examples/)  
- Future adapters: [adapters.md](adapters.md)  
