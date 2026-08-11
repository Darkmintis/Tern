# Examples

Copy these into a real Flutter app. They are **config samples**, not full apps.

| Example | What it shows |
|---|---|
| [`flutter-android-play/`](flutter-android-play/) | Bump → sign → AAB → Play; `ship_retry` without rebuild |
| [`flutter-ios-testflight/`](flutter-ios-testflight/) | Sign → IPA → TestFlight (macOS) |
| [`flutter-both/`](flutter-both/) | Android + iOS parallel builds + ship lane |

## How to use

```bash
cd /path/to/your_flutter_app
cp /path/to/Tern/examples/flutter-android-play/Ternfile .
# fill env from .env.example (never commit real secrets)
tern doctor
tern release --dry-run
tern release
```

Or generate a starter with `tern init`, then compare against these files.

See also: [getting-started](../docs/getting-started.md) · [ENV](../docs/ENV.md)
