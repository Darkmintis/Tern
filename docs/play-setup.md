# Play credentials setup

One-time setup so Tern can sign Android builds and upload to Google Play.  
You do this in a browser; Tern only **reads** the files via env vars.

## Checklist

- [ ] App exists in [Play Console](https://play.google.com/console)
- [ ] Upload keystore (`.jks` / `.keystore`)
- [ ] Play API service-account JSON
- [ ] Service account invited to the app in Play Console
- [ ] Paths set in `.env` (from `.env.example`)

---

## A. Upload keystore

If you already ship with a keystore, reuse it. Otherwise create one:

```bash
keytool -genkey -v -keystore secrets/upload.jks -keyalg RSA -keysize 2048 -validity 10000 -alias upload
```

Put the file in `secrets/` (gitignored). Set in `.env`:

```bash
ANDROID_KEYSTORE=secrets/upload.jks          # or an absolute path
ANDROID_KEYSTORE_PASSWORD=…
ANDROID_KEY_ALIAS=upload
ANDROID_KEY_PASSWORD=…
```

Your Flutter `android/app/build.gradle` must load `key.properties` (Flutter’s usual release template). Tern’s `sign` step writes that file from these env vars.

---

## B. Play service account (API access)

1. Open **Play Console** → your developer account → **Setup → API access**  
   (wording may vary slightly; look for “API access” / service accounts).
2. Link a Google Cloud project if prompted.
3. Create or choose a **service account**.
4. In Google Cloud → IAM → that service account → **Keys → Add key → JSON**.  
   Save as `secrets/play.json`.
5. Back in Play Console, **grant the service account access** to your app  
   (at least permission to manage releases / testing tracks).
6. Set:

```bash
GOOGLE_APPLICATION_CREDENTIALS=secrets/play.json
```

Optional if package detection fails:

```bash
ANDROID_PACKAGE_NAME=com.your.package
```

---

## C. Create the app + internal track (one-time)

1. In Play Console: **Create app** (or open an existing one).
2. Package name / `applicationId` must match your Flutter Android app.
3. Complete any required Console setup forms if Google asks (store listing drafts, declarations, etc.). You only need enough for **testing** uploads to work — not a full public launch.
4. Tern uploads to `track:internal` by default. You do **not** need to manually drag-and-drop an AAB first in most cases — Tern’s first `tern release` can create the internal release via the API.

After a successful upload: **Testing → Internal testing** to see the build and install via Play (optional).

Tern does **not** need a special “enable automation” switch beyond API access + app permissions.

---

## D. Verify

```bash
set -a && source .env && set +a
tern doctor
tern release --dry-run
```

When doctor is green, run `tern release` for a real **internal** upload.

---

## Common failures

| Problem | Fix |
|---|---|
| missing `GOOGLE_APPLICATION_CREDENTIALS` | Path to the JSON file |
| 403 / permission denied | Invite the service account on the app in Play Console |
| package not found | App not created yet, or wrong `applicationId` / `ANDROID_PACKAGE_NAME` |
| unsigned release | Run `sign android` before build; fix keystore env |

More: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
