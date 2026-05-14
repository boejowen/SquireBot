---
name: Phase 2 code-complete + deferred items + LICENSE gap
description: Phase 2 (watcher robustness + schema lock) shipped 2026-05-02 — 49 commits + 5 commit fixture work, all 20 REQ-IDs done. Four user-deferred items + one prereq gap (no LICENSE) blocking SignPath OSS submission.
type: project
originSessionId: dfdf0595-b2de-450e-a3e8-15ecb9220949
---
Phase 2 (watcher-robustness-schema-lock) is **CODE-COMPLETE** as of 2026-05-02. 10 plans across 8 waves, 49 atomic commits since the `phase1-complete` tag. All 20 REQ-IDs covered (INST-04, INST-05, AUTH-05, WATCH-02..09, SCHEMA-01..08, OPS-04, OPS-05). Strict TDD throughout — every `feat(02-XX):` is preceded by a `test(02-XX):` RED commit.

**Why:** The phase delivered: spellbook parser + multi-folder watcher + WATCH-09 catch-up, WATCH-07 retry envelope + `*Client` mutex (Pitfall D closure), AUTH-05 refresh-token death UX (tray red + Reauthorize + auth-suspension flag), 24h heartbeat with W6 D/E preservation, `minio/selfupdate` startup-swap auto-updater (Pitfall #14 closure), HKCU autostart hardening + NSIS uninstaller wipe-or-preserve UX, goreleaser local + revised release.yml publishing bare binary + latest.json, SmartScreen walkthrough doc + SignPath OSS application package, 7-day soak runbook + injection scripts.

**Locked decisions implemented:**
- Spellbook column renamed `Slot → Level` everywhere (CLAUDE.md, ARCHITECTURE.md, SUMMARY.md)
- Schema scaffolded all v1 tabs at `schema_version=1` (extend-only forever after)
- Three-state `ValidateWorkbook` (MatchesCanonical / Empty / WrongCanonical); `bootstrapMeta` deleted
- Tray menu now 7 items: Open Workbook / Open log folder / Check for updates / Change Workbook / Continue setup / Reauthorize / Quit
- Heartbeat 24h `time.AfterFunc` self-reschedule (never `time.Ticker`); fires on startup if last_seen > 23h
- `latest.json` schema: version, binary_url, installer_url, binary_sha256, installer_sha256, released
- Autostart at `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
- Uninstaller checkbox to wipe config + wincred (default = preserve via `MB_DEFBUTTON2`)
- `--uninstall-wipe-credentials <email>` runs as the FIRST action in main.go, BEFORE `update.Apply`

**How to apply:** Phase 3 planning may begin — Phase 2 → Phase 3 share no code paths (Go vs TypeScript/clasp), so the soak observation does NOT block Phase 3.

**Four user-deferred items remain:**
1. **Plan 02-08 Task 4 — rc1 release**: push a real `v0.2.0-rc1` tag, run AUTH-03 negative test (verify the workflow refuses to release if `consent_screen_status != "PRODUCTION"`), then a real `v0.2.0` tag for Phase 2 release. Full checklist in `.planning/phases/02-watcher-robustness-schema-lock/02-08-SUMMARY.md`.
2. **Plan 02-07 Task 5 — logon-cycle smoke**: 6-step manual runbook in `.planning/phases/02-watcher-robustness-schema-lock/02-07-SUMMARY.md`. Build → install → autostart-verify → logout/login → uninstall test A (preserve) → uninstall test B (full wipe). Requires real Windows logout/login.
3. **Plan 02-09 Task 2 — SignPath OSS application**: copy-paste-ready package at `docs/signpath-application.md`. **BLOCKED by LICENSE-file gap** (see below) — SignPath requires an OSI-approved license at repo root. Suggested fix is MIT but it's a user decision (long-term implications for any contributors).
4. **Plan 02-10 Task 5 — 7-day soak observation**: full runbook + injection scripts at `docs/soak-runbook.md` + `scripts/soak/`. Day 0/1/3/5/7 checkpoints with concrete PowerShell/bash injection commands. Calendar-bound — run on user's schedule.

**Prereq gap blocking item #3:** No `LICENSE` file at repo root. SignPath OSS application can't ship without it. Adding a license is a user decision — suggested MIT (per executor flag), but the user owns this choice.

**Outstanding tag/commit work:** Phase 2 should be tagged `phase2-code-complete` when the user is ready (Phase 1 was tagged `phase1-complete`). The actual `phase2-complete` tag is gated on the soak observation.

**Test coverage end of Phase 2:** 14 packages, ~110+ tests, all green. Race detector verification (`go test -race`) deferred to CI consistently across 02-03/04/05/06 because local Windows install lacks CGO/GCC.
