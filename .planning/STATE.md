---
gsd_state_version: 1.0
milestone: v1.0.1
milestone_name: Installer + Permissions Hardening
status: phase7_complete
last_updated: "2026-05-12T18:00:00.000Z"
last_activity: 2026-05-12
resume_file: .planning/phases/08-test-infra-persistence-docs-backfill/
progress:
  total_phases: 3
  completed_phases: 2
  total_plans: 8
  completed_plans: 8
  percent: 100.0
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

Phase: 7 — Admin Allowlist + Eviction Enforcement (SHIPPED + UAT-VERIFIED 2026-05-12). v1.0.1 milestone now at 2/3 phases shipped.
Plan: 3 of 3 complete (07-01 ✓ 2026-05-12, 07-02 ✓ 2026-05-12, 07-03 ✓ 2026-05-12 — all 4 tasks including clasp-push + 5-hook dev-workbook smoke 5/5 PASS). Plan 03 commits: `1937fd0` (test seed staging) + `7f7ffb0` (eviction guards) + `c3c0033` (onOpen wiring) + `2d03581` (interim docs) + `544bef8` (mid-smoke OAuth scope fix — Rule 3, retired latent v1.0 manifest gap). Bundle live on dev workbook script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`. ADMIN-01, ADMIN-02, ADMIN-03 all closed. Schema impact zero (`_meta.schema_version=3`, `WatcherMaxSchemaVersion=3` both untouched).
Status: Phase 7 closed. Next: Phase 8 (Test Infra + Persistence + Docs Backfill — TEST-01, TEST-02, SEARCH-05, DOC-04). Phase 8 planning is unblocked; invoke `/gsd-plan-phase 8` (or `/gsd-discuss-phase 8` first if a research/discussion pass is wanted).
Last activity: 2026-05-12 — Phase 7 closed. Plan 07-03 Task 4 dev-workbook smoke executed by workbook owner (boejowen@gmail.com) against dev workbook (script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`). All 5 verification hooks from 07-CONTEXT.md §verification_hooks PASS: (1) `_meta.guild_admins` + `workbook_owner_floor` + `admin_log` written on first open; second open did not append duplicate bootstrap entry. (2) Non-admin saw "Not authorized" modal + zero `_char_owner` flips + zero `eviction_log` appends + zero `admin_log` appends (allowlist temporarily set to `["someone-else@example.com"]` then restored). (3) Owner added `joseph.bowen2@gmail.com` via Manage Admins sidebar + shared workbook as Editor; joseph.bowen2 in second browser session opened Evict Guildie successfully (after Google OAuth re-consent dialog including new userinfo.email scope). (4) joseph.bowen2 (non-floor admin) opened Manage Admins → boejowen@gmail.com row had `(owner)` annotation and NO Remove button (client-side suppression confirmed); only joseph.bowen2's own row had Remove. (5) `migrations.ts` writeMetaRow schema_version='3' unchanged (1 line); `client.go` WatcherMaxSchemaVersion=3 unchanged (1 line). Significant Rule 3 deviation surfaced and fixed inline: original `apps-script/appsscript.json` was missing `https://www.googleapis.com/auth/userinfo.email` scope — under consumer @gmail.com `Session.getEffectiveUser().getEmail()` silently returned empty, so every Phase 7 admin guard fail-closed. Fixed in commit `544bef8`; non-sensitive scope (no Production-consent change required). Side effect: retires latent v1.0 bug where every eviction's `initiated_by` audit-log field silently wrote `'unknown'` across the entire v1.0 release. Final SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md`; smoke evidence at `07-03-SMOKE.md`. Modified `apps-script/src/triggers/showEvictionSidebar.ts` (+1 import line for `isAdmin`/`requireAdminOrThrow`/`normalizeEmail` from `'../lib/admin'`; opener gets normalizeEmail(Session.getEffectiveUser) → isAdmin → getUi().alert non-admin path; getEvictionEmails/previewEviction/commitEviction each get requireAdminOrThrow as FIRST statement). Modified `apps-script/src/triggers/onOpen.ts` (+2 imports for `bootstrapGuildAdmins` + `log` — first imports ever in this file; +6 lines try/catch lazy bootstrap at top of onOpen; +1 line `Manage Admins…` menu item between Evict Guildie and Set Theme; +2 lines separator + `Initialize Admin Allowlist (manual)` menu item after Run Migration v=2 legacy). Modified `apps-script/src/__tests__/showEvictionSidebar.test.ts` (added `seedMetaWithAdmins(state, adminEmails, extraRows?, floor?)` helper; all 4 beforeEach blocks switched from `seedMeta(state, [['schema_version','3']])` to `seedMetaWithAdmins(state, ['officer@example.com'])`; Test 11 uses extraRows arg; Test 12 reframed per Rule 1 — pre-Phase-7 it asserted initiated_by='unknown' soft-fallback, post-Phase-7 the same Session call drives the admin guard FIRST and fail-closes empty, so the test now asserts the new auth boundary). Two minor deviations documented (Rule 1 Test 12 reframe; Rule 3 `Manage Admins…` matches 2 lines in onOpen.ts because of file-header doc comment — spirit honored). INTERIM SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md`.

**Phase 7 plan-phase completed 2026-05-12** (prior history): 3 PLAN.md files: 07-01-admin-policy-module-PLAN.md (wave 1, lib/admin.ts + tests, 20 unit-test scenarios T1–T20), 07-02-admin-mgmt-sidebar-PLAN.md (wave 2, showAdminMgmtSidebar.ts + tests + Code.ts re-exports), 07-03-eviction-guard-onopen-smoke-PLAN.md (wave 2, eviction sidebar admin-guard + onOpen lazy bootstrap + 2 new menu items + clasp-push interactive smoke against dev workbook).

### v1.0.1 Phase Plan (2026-05-11)

| Phase | Name | Requirements | Stack | Ship gate | Status |
|-------|------|--------------|-------|-----------|--------|
| 6 | Installer Overwrite-Running Shim | INST-06 | Go + NSIS | watcher v1.0.1 binary release | ✅ SHIPPED 2026-05-11 (tag `v1.0.1`) |
| 7 | Admin Allowlist + Eviction Enforcement | ADMIN-01, ADMIN-02, ADMIN-03 | Apps Script | `clasp push` of apps-script bundle | ✅ SHIPPED 2026-05-12 (5/5 hooks PASS) |
| 8 | Test Infra + Persistence + Docs Backfill | TEST-01, TEST-02, SEARCH-05, DOC-04 | Apps Script + docs | `clasp push` + green CI + milestone retrospective | Not started — `/gsd-plan-phase 8` next |

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
| Phases complete (v1.0.1) | 2 / 3 (Phase 6 shipped 2026-05-11 as `v1.0.1`; Phase 7 SHIPPED + UAT-verified 2026-05-12) |
| Plans complete (v1.0.1) | 8 / 8 currently planned (Phase 6: 5/5; Phase 7: 3/3 fully shipped — Phase 8 plans TBD) |
| Requirements mapped (v1.0.1) | 8 / 8 |
| Requirements complete (v1.0.1) | 4 / 8 (INST-06 ✓ UAT-verified 2026-05-11; ADMIN-01 + ADMIN-02 + ADMIN-03 ✓ UAT-verified 2026-05-12 via dev-workbook 5-hook smoke) |
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
7. **(Plan 07-01) admin.ts cloned the eviction-sidebar lock+audit-log envelope verbatim.** Per D-05, addAdmin/removeAdmin use the same shape as commitEviction. Identical idiom prevents drift; future readers reach for the eviction-sidebar example and find the admin module already follows it. (Plan 07-01 SUMMARY, 2026-05-12.)
8. **(Plan 07-01) bootstrapGuildAdmins is the documented exception that returns `{reason:'lock_busy'}` silently.** Per D-01, onOpen MUST NOT throw or the menu breaks for everyone. The lock-busy branch logs a warn and returns the enum instead of throwing. The next workbook open retries idempotently. (Plan 07-01 SUMMARY, 2026-05-12.)
9. **(Plan 07-01) Per-test getOwner override lives in admin.test.ts, NOT in test-helpers.ts.** Only 4 tests need `SpreadsheetApp.getActiveSpreadsheet().getOwner()` (T15/T17/T18/T19); a 7-line helper at the top of admin.test.ts wraps the base mock per-test. test-helpers.ts surface unchanged. Plan 02's adminMgmtSidebar.test.ts may need to extend test-helpers for `getUi().alert(...)` capture — that decision belongs to Plan 02. (Plan 07-01 SUMMARY, 2026-05-12.)
10. **(Plan 07-02) test-helpers extended minimally for alert capture.** Per Plan 02's deferred Plan 01 decision: 3 additive changes to `test-helpers.ts` (alertCalls field, alert mock body, ButtonSet/Button enums on getUi()). Backward-compatible — full suite stayed 317→324 GREEN with no existing test broken. Lib-alias imports in `showAdminMgmtSidebar.ts` (libGetAdminList/libAddAdmin/libRemoveAdmin) resolve the wrappers-export-the-same-names contract from CONTEXT.md §canonical_refs. (Plan 07-02 SUMMARY, 2026-05-12.)
11. **(Plan 07-02) build.mjs TRIGGER_GLOBALS list update is a load-bearing scope addition for every plan that adds Apps Script globals.** The Phase 3 d0a2645-style sync assertion fires at build time and blocks the build if Code.ts and TRIGGER_GLOBALS diverge. Plan 02's plan text didn't enumerate `build.mjs` in `<files>`, but the assertion is non-negotiable. Future plans adding new google.script.run callbacks or menu-item-targeted globals must update both Code.ts AND TRIGGER_GLOBALS. (Plan 07-02 SUMMARY, 2026-05-12.)
12. **(Plan 07-03 / Test 12 reframe — Rule 1) Pre-Phase-7 Test 12 asserted the audit-log soft-fallback (`initiated_by='unknown'`) when `Session.getEffectiveUser` returns empty.** Post-Phase-7, the SAME Session call drives the new admin guard FIRST and fail-closes empty per `requireAdminOrThrow`. Test 12 was reframed to assert the new auth boundary (empty Session → `not_authorized` throw + no writes) — preserves the test's intent (exercising the empty-Session branch) while honoring the sequence-load-bearing guard. The audit-log `'unknown'` soft-fallback code path remains in `commitEviction` as residual defensive code (only path to reach it would be Session returning a non-empty admin email at the guard and an empty value at the later log call — non-physical). (Plan 07-03 SUMMARY, 2026-05-12.)
13. **(Plan 07-03 mid-smoke / OAuth scope — Rule 3 fix, latent v1.0 bug retired) The original `apps-script/appsscript.json` did not declare `https://www.googleapis.com/auth/userinfo.email`.** Under consumer @gmail.com accounts, `Session.getEffectiveUser().getEmail()` silently returns `""` without that scope declared. Phase 7's correct fail-closed admin guards exposed the latent v1.0 manifest gap immediately during Hook 3 (peer joseph.bowen2 saw "Not authorized" despite being correctly added to `_meta.guild_admins`). Fixed inline as commit `544bef8` (added the scope; non-sensitive per Google's OAuth classification — no Production-consent change required, no Google verification audit triggered). **Side effect:** retires a pre-existing latent v1.0 bug — every eviction's `initiated_by` audit-log field has been silently writing `'unknown'` (the D-06 soft fallback) across the entire v1.0 release because of this same scope omission. **Future-state rule:** any future Apps Script scope addition stays non-sensitive (or the manifest stays at the current 5-scope set) so the consent screen does not require Production-consent re-review. (Plan 07-03 SMOKE, 2026-05-12.)

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

**2026-05-12 (Phase 7 SHIPPED + UAT-VERIFIED — Plan 03 closure + dev-workbook smoke):** Plan 07-03 finalized. Workbook owner (boejowen@gmail.com) executed the dev-workbook smoke against script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`. All 5 verification hooks from 07-CONTEXT.md §verification_hooks PASS: (1) `_meta.guild_admins=["boejowen@gmail.com"]` + `workbook_owner_floor=boejowen@gmail.com` + `admin_log` bootstrap entry on first open; second open did not duplicate (idempotent). (2) Non-admin path: with allowlist temporarily set to `["someone-else@example.com"]`, `Evict Guildie…` showed "Not authorized" modal + zero `_char_owner` writes + zero `eviction_log` appends + zero `admin_log` appends; allowlist restored. (3) Owner added `joseph.bowen2@gmail.com` via Manage Admins sidebar + shared workbook as Editor (separate Google permission); joseph.bowen2 in second browser session passed Google OAuth re-consent (incl. new userinfo.email scope) and opened Evict Guildie successfully. (4) joseph.bowen2 (non-floor admin) opened Manage Admins → boejowen@gmail.com row had `(owner)` annotation and NO Remove button (client-side suppression confirmed); joseph.bowen2's own row had Remove. Server-side `'owner_floor_protected'` throw covered by admin.test.ts T18 + adminMgmtSidebar.test.ts TS6. (5) Schema gates PASS (migrations.ts schema_version='3' unchanged 1 line; client.go WatcherMaxSchemaVersion=3 unchanged 1 line). **Significant Rule 3 deviation surfaced + fixed mid-smoke:** original `apps-script/appsscript.json` was missing `https://www.googleapis.com/auth/userinfo.email` — under consumer @gmail.com `Session.getEffectiveUser().getEmail()` silently returned empty so every Phase 7 admin guard fail-closed for every guildie. Fix shipped as commit `544bef8`; non-sensitive scope (no Production-consent change required). Side effect: retires latent v1.0 bug where every eviction's `initiated_by` audit-log field silently wrote `'unknown'` across the entire v1.0 release. Two `clasp push --force` invocations during the session (first shipped Phase 7 bundle commits 1937fd0+7f7ffb0+c3c0033; second shipped the manifest fix); both deployed `dist/Code.js` + `dist/appsscript.json` cleanly. Final SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md`; smoke evidence at `07-03-SMOKE.md`. **ADMIN-01, ADMIN-02, ADMIN-03 all closed.** Phase 7 = SHIPPED + UAT-VERIFIED. v1.0.1 milestone now at 2/3 phases shipped; Phase 8 (test infra + persistence + docs backfill) unblocked.

**2026-05-12 (Phase 7 Plan 02 executed — admin-mgmt sidebar shipped):** Created `apps-script/src/triggers/showAdminMgmtSidebar.ts` (263 lines, 4 exports: opener admin-gated with getUi().alert non-admin path + 3 google.script.run callbacks getAdminList/addAdmin/removeAdmin) + `apps-script/src/__tests__/adminMgmtSidebar.test.ts` (219 lines, 7 named tests TS1–TS7 covering required scenarios + bonus floor self-removal + empty-Session fail-closed). Modified `apps-script/src/Code.ts` (+12 lines: 2 import blocks for the 4 sidebar exports + bootstrapGuildAdminsManual from lib/admin, plus 5 new entries in the export footer), `apps-script/build.mjs` (+6 lines: TRIGGER_GLOBALS extended with the 5 new names under a Phase 7 plan 07-02 section comment), `apps-script/src/__tests__/test-helpers.ts` (+20 lines: alertCalls capture field, getUi().alert mock body that pushes args + returns 'OK' sentinel, ButtonSet/Button enums on getUi()). 3 commits, all autonomous: `2ea0cd2` (trigger), `a5d3cb4` (test + helpers), `d84d684` (Code.ts + build wiring). Full apps-script suite went 317→324 GREEN; typecheck clean; build clean (`dist/Code.js` contains all 5 new top-level globals as verified via grep). Verification hook 5 grep gates still PASS (admin-mgmt sidebar has 0 schema_version references; migrations.ts schema_version writes unchanged at 1 line; WatcherMaxSchemaVersion=3 unchanged in client.go). Two minor deviations documented (build.mjs scope addition per Rule 3 — the d0a2645-style sync assertion is non-negotiable; "exactly 2 lines" Code.ts grep matches 3 because the import-path string also matches the regex — spirit of 1 import + 1 export honored). ZERO breaking changes — Plan 03 can now wire the menu items + onOpen lazy bootstrap + clasp-push smoke. SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-02-SUMMARY.md`.

**2026-05-12 (Phase 7 Plan 01 executed — admin policy module shipped):** Created `apps-script/src/lib/admin.ts` (357 lines, 9 public exports: normalizeEmail, getAdminList, isAdmin, requireAdminOrThrow, addAdmin, removeAdmin, bootstrapGuildAdmins, bootstrapGuildAdminsManual, appendAdminLogEntry) + `apps-script/src/__tests__/admin.test.ts` (462 lines, 20 named tests T1–T20). 2 commits, TDD discipline followed (RED `dfc3533` → GREEN `8780222`). All 20 tests PASS on first GREEN run; full apps-script suite went 297→317 GREEN; typecheck clean. Verification hook 5 grep gates all PASS: `admin.ts` has 0 schema_version references, `migrations.ts` schema_version writes unchanged at 9 lines, `WatcherMaxSchemaVersion=3` unchanged in `internal/sheet/client.go`. ZERO deviations from the `<interfaces>` contract — Plans 02 + 03 can import the 9 exports verbatim. Owner-floor lockout (D-04), fail-closed empty-email (D-06), bootstrap-must-not-throw (D-01) all enforced and tested. Test-helpers.ts UNCHANGED — per-test getOwner override lives in admin.test.ts only. SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-01-SUMMARY.md`.

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

**Phase 7 SHIPPED + UAT-VERIFIED 2026-05-12.** v1.0.1 milestone now at 2/3 phases shipped (Phase 6 ✓, Phase 7 ✓, Phase 8 unstarted). Next actions, in priority order:

1. **Plan Phase 8** — `/clear` then either `/gsd-discuss-phase 8` (for a context-gathering pass on TEST-01/02 + SEARCH-05 + DOC-04) or `/gsd-plan-phase 8` directly (the four requirements are well-scoped per REQUIREMENTS.md and ROADMAP.md Phase 8 success criteria). Phase 8 ships via `clasp push` of the apps-script bundle + green CI + milestone retrospective. Estimated 4–6 plans (one per requirement plus a possible JSDOM infra plan as a wave-1 prerequisite for TEST-02).
2. **After Phase 8 ships, close v1.0.1 milestone** — same pattern as v1.0 close: archive ROADMAP/REQUIREMENTS to `.planning/milestones/v1.0.1-*.md`, write `MILESTONE-AUDIT.md`, update PROJECT.md + MILESTONES.md, decide next milestone (or pause).

**Independent of v1.0.1:** Day-10 token-survival check fires automatically 2026-05-13T15:00:00Z (routine `trig_01Uog2muQ22CBsjZfqPiSH9r`).

**Open follow-up (low priority):** the v1.0 manifest scope omission (`userinfo.email`) was retired in commit `544bef8`. The latent v1.0 bug it caused — every eviction's `initiated_by` audit-log field silently writing `'unknown'` — is now structurally closed for all post-Phase-7 deploys; historical `_meta.eviction_log` entries from v1.0 are not retroactively patched (operator decision: leave history as-is; the audit trail still records `at`, `action`, `email` correctly).

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
