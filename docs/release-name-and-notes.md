# Release name & notes

Tern sets **Play Console release name** and **release notes** automatically on upload/ship.
Defaults match what most teams want: marketing version + generic notes.

## Defaults

| Field | Default |
|---|---|
| Release name | `version` → `1.2.3` from pubspec (before `+`) |
| Release notes | `Bug fixes and improvements.` |
| Notes locale | `en-US` |

## Release name strategies (`release_name:…`)

| Strategy | Example (pubspec `1.2.3+9`) | When to use |
|---|---|---|
| `version` (default) | `1.2.3` | Standard Play / App Store marketing version |
| `version_build` | `1.2.3 (9)` | Play Console style with build number |
| `version_code` | `9` | Build-number-only naming |
| `semver_plus` | `1.2.3+9` | Keep exact pubspec string |
| `name_version` | `Cool app 1.2.3` | App display name + version (`TERN_APP_NAME` / pubspec `name`) |
| `date` | `2026-08-11` | Date-stamped internal drops |
| `version_date` | `1.2.3 · 2026-08-11` | Version plus UTC date |
| `git_tag` | `v1.2.3` | Tag / `git describe` |
| `git_sha` | `a1b2c3d` | Short commit |
| `none` | _(omit)_ | Do not set a store release name |
| `"Custom title"` | `Custom title` | Quoted literal / free text |

## Release notes (`notes:…`)

| Form | Behavior |
|---|---|
| _(omit)_ / `notes:default` | `Bug fixes and improvements.` |
| `notes:none` | Omit notes |
| `notes:"Fixed login crash."` | Inline custom text for this upload |
| `notes:file:RELEASE_NOTES.md` | Read file (path relative to project root) |
| `notes_locale:en-US` | Play `LocalizedText` language (default `en-US`) |

## Ternfile examples

```text
# Defaults — version name + generic notes
upload android to play_store track:internal

# Play-style name + custom notes
upload android to play_store track:beta release_name:version_build notes:"Bug fixes and performance improvements."

# Per-release notes file + custom title
ship android from last to play_store track:production release_name:"Hotfix March" notes:file:notes/en-US.txt

# Omit both
upload android to play_store track:internal release_name:none notes:none
```

## CLI

```bash
tern ship last --to play_store --release-name version_build --notes "Bug fixes and improvements."
tern ship last --to play_store --notes-file RELEASE_NOTES.md
```

## iOS / TestFlight note

IPA upload via `altool` does not set App Store Connect “What’s New” yet. Tern still resolves name/notes and writes `.tern/artifacts/ios-release-meta.txt` so you (or a later ASC API step) can apply them. Play Store name + notes are applied live on upload.
