# ADR 0005 — Error Taxonomy

**Status:** Accepted  
**Date:** 2026-08-10  
**Updated:** 2026-08-10

Stable error classes in `internal/errors` (import as `ternerrors`):

| Class | Exit | When |
|---|---|---|
| `ConfigError` | 2 | Parse / unknown step / invalid Ternfile |
| `DoctorError` | 3 | Failed doctor checks |
| `BuildError` | 4 | Adapter / toolchain build failure |
| `SignError` | 5 | Keystore / codesign / profile failure |
| `UploadError` | 6 | Play / App Store Connect failure |
| `ExecError` | 7 | Subprocess failure |

Rules:

- Wrap with `%w` via `Wrap` / `WrapHint`
- Prefer `NewHint` / `WrapHint` for actionable remediation text
- CLI prints `error:` then `hint:` when present
- JSON events may include `error_class`
