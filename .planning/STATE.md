---
gsd_state_version: 1.0
milestone: v1.0.1
milestone_name: Installer + Permissions Hardening
status: phase7_context_gathered
last_updated: "2026-05-12T00:30:00.000Z"
last_activity: 2026-05-12
resume_file: .planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md
progress:
  total_phases: 3
  completed_phases: 1
  total_plans: 5
  completed_plans: 5
  percent: 100
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-11 (v1.0.1 roadmap landed — 3 phases, 8 requirements, ship gate = watcher v1.0.1 binary release)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-11 after v1.0 milestone close + v1.0.1 milestone open)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.
- **Current focus:** Milestone v1.0.1 Installer + Permissions Hardening — retire v1.0 carry-over debt (installer upgrade path, officer-only eviction by code, sidebar test + docs debt). 3 phases planned (Phase 6 = installer shim, Phase 7 = admin allowlist, Phase 8 = test infra + persistence + docs backfill).
- **Mode:** yolo
- **Granularity:** coarse
- **Total v1.0.1 phases:** 3 (none started)
- **Total milestone v1.0 phases:** 5 (all shipped — archived in `.planning/milestones/v1.0-ROADMAP.md`)

## Current Position

Phase: 7 — Admin Allowlist + Eviction Enforcement (CONTEXT gathered 2026-05-12; ready for /gsd-plan-phase)
Plan: None yet. Phase 6 fully shipped + UAT-verified 2026-05-11. Phase 7 CONTEXT.md captures 6 locked decisions across the 4 gray areas presented; user delegated all four to Claude with the same "simplest end-user experience" directive used in Phase 6.
Status: Phase 7 context captured. CONTEXT.md + DISCUSSION-LOG.md committed as `docs(07): capture phase context`. Awaiting `/gsd-plan-phase 7`.
Last activity: 2026-05-12 — Phase 7 discuss-phase completed. Six locked decisions: D-01 lazy onOpen bootstrap + manual-fallback menu item; D-02 JSON array (`_meta.guild_admins`) + separate single-email row (`_meta.workbook_owner_floor`) + JSON-array audit log (`_meta.admin_log`); D-03 Apps Script modal alert for non-admins; D-04 dedicated `showAdminMgmtSidebar` + new "Manage Admins…" menu item; D-05 central `apps-script/src/lib/admin.ts` policy module; D-06 fail-closed auth + soft-fallback audit-log identity. No schema bump, no watcher rebuild. Estimated 3-4 plans.

### v1.0.1 Phase Plan (2026-05-11)

| Phase | Name | Requirements | Stack | Ship gate |
|-------|------|--------------|-------|-----------|
| 6 | Installer Overwrite-Running Shim | INST-06 | Go + NSIS | watcher v1.0.1 binary release |
| 7 | Admin Allowlist + Eviction Enforcement | ADMIN-01, ADMIN-02, ADMIN-03 | Apps Script | `clasp push` of apps-script bundle |
| 8 | Test Infra + Persistence + Docs Backfill | TEST-01, TEST-02, SEARCH-05, DOC-04 | Apps Script + docs | `clasp push` + green CI + milestone retrospective |

**Sequencing rationale:**

1. Phase 6 first because it is the only Go-side change and produces the binary release that tags the milestone (`v1.0.1`). Independent of the apps-script work.
2. Phase 7 second because ADMIN-02 (eviction enforcement) is the highest user-value bit — it closes the code-gap behind the v1.0 "social convention only" eviction model. ADMIN-01 is the prerequisite extend-only `_meta` row; ADMIN-03 is the management UX.
3. Phase 8 last because TEST-02 covers the new eviction + admin sidebars landing in Phase 7 — testing them after they ship is the natural order. SEARCH-05 and DOC-04 are independent but ride along in the same apps-script deploy.

**Schema impact:** NONE expected across all 3 phases. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3 (the watcher binary rebuild in Phase 6 is for the NSIS install path, not for schema reasons). `_meta.guild_admins` is an extend-only row addition.

### v1.0 SHIPPED (2026-05-11)

Milestone v1.0 complete. 5/5 phases shipped. 69/69 effective requirements covered. Tag `v1.0.0` pushed. Pages site live at https://boejowen.github.io/SquireBot/. Archive lives in `.planning/milestones/v1.0-*.md`. See archive for the full execution log.

## Performance Metrics

| Metric | Value |
|--------|-------|
| Phases planned (v1.0.1) | 3 / 3 |
| Phases complete (v1.0.1) | 1 / 3 (Phase 6 shipped 2026-05-11 as `v1.0.1`) |
| Plans complete (v1.0.1) | 5 / 5 (Phase 6) |
| Requirements mapped (v1.0.1) | 8 / 8 |
| Requirements complete (v1.0.1) | 1 / 8 (INST-06 ✓ UAT-verified 2026-05-11 — binary shipped + live upgrade validated on Azure VM) |
| Active blockers | 0 |
| Milestone v1.0 final | 69/69 effective requirements; 5/5 phases; 31/31 plans (archived) |

## Accumulated Context

### Decisions Log (v1.0.1)

Decisions locked at milestone open and roadmap creation:

1. **v1.0.1 is a patch milestone — small scope, 3 phases.** Eight requirements; granularity=coarse. Resist over-fragmenting. (Roadmap, 2026-05-11.)
2. **No schema bump.** `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3. `_meta.guild_admins` is an extend-only row addition. No watcher migration logic; no `SCRIPT_MIN_SCHEMA_VERSION` change. (Roadmap, 2026-05-11.)
3. **Phase 6 is the only Go-side change.** Everything else is apps-script + docs. The Phase 6 ship produces the binary release that tags the milestone (`v1.0.1`). (Roadmap, 2026-05-11.)
4. **Ship-gate model = single ship gate per phase.** Phase 6 ships a binary release; Phase 7 + Phase 8 each ship via `clasp push` of the apps-script bundle to the dev workbook. (Roadmap, 2026-05-11.)
5. **Owner-floor lockout protection on ADMIN-03.** Workbook owner email cannot be removed from `_meta.guild_admins` by anyone other than the owner themselves. (REQUIREMENTS.md ADMIN-03.)
6. **SEARCH-05 = PropertiesService per-user scope, NOT Document scope.** Recent-3 MRU is per-guildie state, not workbook-shared state. Migrating from CacheService to PropertiesService keeps the same scope semantics. (REQUIREMENTS.md SEARCH-05.)

### Decisions Log (v1.0, mirrored for continuity)

The 10 v1.0-era decisions are preserved in `PROJECT.md` Key Decisions table and in the archived `.planning/milestones/v1.0-ROADMAP.md` — they remain in force for v1.0.1.

### Open TODOs

- **(Day-10 token-survival check, fires automatically)** Routine `trig_01Uog2muQ22CBsjZfqPiSH9r` fires 2026-05-13T15:00:00Z to validate AUTH-03 / Pitfall #1 (refresh token survives the 7-day Testing-mode expiry boundary). If it succeeds, AUTH-03 is structurally validated permanently. If it fails, re-open consent-screen publishing investigation.
- **(Roadmap audit, v1.0 carry)** Reconcile the "56 v1 requirements" figure mentioned historically with the actual literal count of 69 REQ-IDs from v1.0. Already noted in v1.0 milestone audit; not blocking v1.0.1.
- **(SignPath OSS, separate track)** SignPath Foundation OSS approval is in flight; lands as a hotfix (NOT a v1.0.1 phase) when the Foundation review completes. Would retire INST-05 partial → full.

### Active Blockers

None. v1.0 shipped clean; v1.0.1 is planning-stage. Phase 6 is unblocked and ready for `/gsd-plan-phase 6`.

## Session Continuity

### Last Session Summary

**2026-05-12 (Phase 7 discuss-phase completed):** Phase 7 (Admin Allowlist + Eviction Enforcement) context gathered via `/gsd-discuss-phase 7`. Four phase-specific gray areas were surfaced (bootstrap mechanism, storage shape + owner-floor tracking, non-admin failure UX, admin-management UX shape); the user delegated all four to Claude with "make whichever choice(s) that will make the end-user experience as simple as possible" — same play as Phase 6. Six locked decisions emerged (D-01 through D-06 in CONTEXT.md). New code surfaces planned: `apps-script/src/lib/admin.ts` (policy primitives), `apps-script/src/triggers/showAdminMgmtSidebar.ts` (300px sidebar matching v1.0 sidebar patterns), plus admin-guards inserted into the existing `showEvictionSidebar.ts` opener + all three callbacks, lazy `bootstrapGuildAdmins()` call from `onOpen`, and two new menu items in `onOpen.ts`. No schema bump (`_meta.guild_admins`, `_meta.workbook_owner_floor`, `_meta.admin_log` are extend-only row additions). No watcher rebuild. Estimated 3-4 plans. CONTEXT.md + DISCUSSION-LOG.md at `.planning/phases/07-admin-allowlist-eviction-enforcement/` committed (`e168be5`).

**2026-05-11 (Phase 6 Plan 05 executed — Phase 6 SHIPPED as v1.0.1):** Pushed tag `v1.0.1` (annotated, message "Phase 6 (INST-06): installer overwrite-running shim. Watcher binary v1.0.1 release.") to origin at master HEAD commit `265bbd9`. Release workflow run [25686757380](https://github.com/boejowen/SquireBot/actions/runs/25686757380) completed in 1m55s with all 17 steps green — AUTH-03 PRODUCTION consent_screen gate held (release.yml:136). GitHub Release [v1.0.1](https://github.com/boejowen/SquireBot/releases/tag/v1.0.1) published with 4 assets: `SquireBot-Setup-1.0.1.exe` (sha256 `38a447bf1d79813861276d2480bd53451fdda66eaf4be24a8f88e2d9ac43fbc4`), `squirebot.exe` bare binary (sha256 `4707c92db98b3b748efb574927a5b8c8e7773ceec0e2a7c826ea6683d0cf4389`), `latest.json` (version=1.0.1, fresh sha256s, working URLs verified via curl HEAD), and the versionless permalink alias `SquireBot-Setup.exe`. D-06 (latest.json schema unchanged) and D-07 (tag = `v1.0.1`) honored. Phase 6 source-complete + shipped. **Task 4 — end-user v1.0.0 → v1.0.1 upgrade UAT on clean Win11 VM — is DEFERRED**; until that runs, INST-06 is "shipped-pending-UAT" rather than fully verified. Single planning commit captures this state. SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md`.

**2026-05-11 (Phase 6 Plan 04 executed):** Retired the "Installer won't overwrite a running SquireBot" section from `docs/troubleshooting.md` (-479 bytes) and appended a `### Manual debug aids` subsection to `docs/build-and-install.md` (+995 bytes) documenting `squirebot.exe --quit` with cross-reference to Plan 06 / INST-06. Pure docs change, zero code impact. Single commit `4465836`. SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md`.

**2026-05-11 (Phase 6 Plan 03 executed):** Shipped INST-06 NSIS pre-install shim — `installer/squirebot.nsi` gained 99 lines (WordFunc.nsh include + inlined StrContains helper + pre-install shim block at top of `Section "Install"`). Reads `DisplayVersion` from HKCU, gates against `1.0.1` via `${VersionCompare}`; on `>= 1.0.1` runs `ExecWait '"$INSTDIR\squirebot.exe" --quit'` then polls `tasklist /FI` via bundled `nsExec::Exec` for up to 40 iterations × 250ms = 10s hard cap; always falls back to `taskkill /IM /F` (verbatim from uninstaller). Honors D-01..D-05: no new plugins (nsExec is bundled), `RequestExecutionLevel user` preserved, post-install `Exec` line untouched, autostart Run-key untouched. All 14 acceptance-criteria greps PASS. NSIS toolchain not installed locally — `makensis` build verification deferred to CI per the plan's fallback clause. Single commit `9a179bd`.

**2026-05-11 (Phase 6 Plan 02 executed):** Shipped INST-06 main.go wiring — `--quit` CLI flag handler block + named-event listener goroutine in `cmd/squirebot/main.go`. CLI handler placed before `update.Apply()`, uses stderr-only logging, exits 0 on every branch. Listener goroutine funnels through `cancel()` + `systray.Quit()` (no `os.Exit`). 2 commits (`5256382` feat, `a36e72f` feat) + docs commit `80b61f7`. CLI contract `squirebot.exe --quit` now safe for the NSIS shim to invoke.

**2026-05-11 (Phase 6 Plan 01 executed):** Shipped INST-06 Go-side primitive — new `internal/system` package with paired build-tag files (`shutdown_signal_windows.go` + `shutdown_signal_other.go` + `doc.go` + `shutdown_signal_windows_test.go`). `SignalShutdown()` opens-or-creates a `Local\SquireBot-Shutdown` named event and SetEvents it (OpenEvent then CreateEvent-on-ERROR_FILE_NOT_FOUND fallback for fire-and-forget semantics). `WaitForShutdown(ctx)` returns a channel that closes on signal OR ctx-cancel via a watchdog goroutine. 5 Windows-only tests pass (round-trip, no-listener no-op, ctx-cancel cleanup, idempotency, late-listener-loses-signal contract). No `go.mod` changes — `golang.org/x/sys v0.43.0` is already a direct dep. No `slog` calls (signal side runs before `logging.Setup`). Single commit `a705f4e`. Signatures locked for Plan 06-02 to consume verbatim.

**2026-05-11 (v1.0.1 milestone open + roadmap):** v1.0 shipped clean as tag `v1.0.0`. Milestone v1.0.1 Installer + Permissions Hardening opened the same day. v1.0.1 REQUIREMENTS.md defined 8 requirements across 5 carry-over features (INST-06, ADMIN-01..03, TEST-01/02, SEARCH-05, DOC-04). v1.0.1 ROADMAP.md mapped all 8 requirements onto 3 phases:

- Phase 6 (Installer Overwrite-Running Shim) — INST-06 only, Go + NSIS, ships watcher v1.0.1 binary
- Phase 7 (Admin Allowlist + Eviction Enforcement) — ADMIN-01..03, apps-script
- Phase 8 (Test Infra + Persistence + Docs Backfill) — TEST-01/02 + SEARCH-05 + DOC-04, apps-script + docs

Schema impact NONE across all 3 phases. 100% requirement coverage validated. REQUIREMENTS.md Traceability table updated.

**2026-05-01 → 2026-05-11 (v1.0 execution):** 5 phases shipped end-to-end over 11 days. Full execution log archived in `.planning/milestones/v1.0-ROADMAP.md`.

### Next Action

**Phase 7 CONTEXT GATHERED 2026-05-12.** Next actions, in priority order:

1. **Plan Phase 7** — `/gsd-plan-phase 7` (or `/gsd-plan-phase 7 --skip-research` — research optional per CONTEXT.md, all patterns are already in the codebase). CONTEXT.md at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md` has 6 locked decisions, full canonical_refs, code_context, and verification_hooks. Estimated 3-4 plans.
2. **After Phase 7 ships, plan Phase 8** (test infra + persistence + docs backfill).

**Independent of v1.0.1:** Day-10 token-survival check fires automatically 2026-05-13T15:00:00Z (routine `trig_01Uog2muQ22CBsjZfqPiSH9r`).

### Files of Record

- `.planning/PROJECT.md` — core value, requirements summary, constraints, key decisions (updated 2026-05-11 with v1.0.1 milestone scope).
- `.planning/REQUIREMENTS.md` — v1.0.1 scope (8 requirements + Traceability table filled).
- `.planning/ROADMAP.md` — v1.0.1 phase structure (Phases 6–8) with success criteria.
- `.planning/STATE.md` — this file.
- `.planning/MILESTONES.md` — historical record (v1.0 archived; v1.0.1 next entry on ship).
- `.planning/milestones/v1.0-ROADMAP.md` — v1.0 archive (5 phases, 31 plans, 69/69 reqs).
- `.planning/milestones/v1.0-REQUIREMENTS.md` — v1.0 archive (full 69 REQ-ID reconciliation).
- `.planning/milestones/v1.0-MILESTONE-AUDIT.md` — v1.0 retrospective + tech debt log.
- `.planning/research/SUMMARY.md` — research synthesis (v1.0-era; still in force for v1.0.1).
- `.planning/research/STACK.md` — locked stack decisions.
- `.planning/research/PITFALLS.md` — 27 pitfalls catalogue.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.
- `docs/troubleshooting.md` — installer manual-stop workaround that Phase 6 retires.
- `docs/apps-script-deploy.md` — clasp-push deploy runbook used by Phase 7 + Phase 8 ship steps.

---

*State initialized: 2026-04-30 after roadmap creation. v1.0 milestone COMPLETE: 2026-05-11 (5/5 phases shipped). v1.0.1 roadmap landed: 2026-05-11 (3 phases planned, 0 started).*
