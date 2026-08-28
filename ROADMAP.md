# Tern Roadmap

## Current (v0.x) — Release Automation
- ✅ Build, sign, upload, tag
- ✅ Safety (Ctrl+C restore, pubspec restore)
- ✅ Release history, rollback, status
- ✅ iOS keychain + cert management
- ✅ Test running
- ✅ Telegram notifications
- ✅ Platform auto-detection
- ✅ CI/CD auto-detection

## Near Future (v0.5) — Polish & Reliability
- [ ] `tern doctor` — full toolchain validation
- [ ] `tern validate` — pre-release checks
- [ ] Store API rollback (Play/App Store)
- [ ] Webhook notifications (Slack, Discord, custom)
- [ ] Multi-track promotion (internal → alpha → beta → production)
- [ ] Artifact cleanup (auto-remove old builds)
- [ ] Release notes from git log

## Medium Future (v1.0) — Enterprise Features
- [ ] `tern audit` — immutable release log with SHA256 proof
- [ ] `tern rotate` — auto-rotate keystore/certs before expiry
- [ ] Role-based access control (RBAC)
- [ ] Multi-app releases (monorepo support)
- [ ] Approval workflows (require sign-off before production)
- [ ] Secret rotation with zero-downtime
- [ ] Compliance reports (SOC2, GDPR)

## Long Future (v2.0) — Platform Expansion
- [ ] Web deploy support
- [ ] Desktop app support
- [ ] Plugin system (community extensions)
- [ ] Tern Cloud (hosted CI/CD)
- [ ] AI-powered release notes
- [ ] Predictive rollback (auto-rollback on crash spike)

## Design Principles (Never Change)
1. **Simple > Feature-rich** — One command does one thing well
2. **Safety > Speed** — Always restore, always verify
3. **Flutter first** — Other frameworks later
4. **No server required** — CLI tool, not a service
5. **Observable** — Logs, history, audit trail
