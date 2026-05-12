---
phase: 07-admin-allowlist-eviction-enforcement
plan: 03
subsystem: apps-script
tags: [apps-script, eviction, onopen, menu, admin-guard, lazy-bootstrap, ship-gate, shipped, uat-verified]
status: shipped
dependency_graph:
  requires:
    - apps-script/src/lib/admin.ts (Plan 01 — 9 public exports, esp. isAdmin/requireAdminOrThrow/normalizeEmail/bootstrapGuildAdmins)
    - apps-script/src/triggers/showAdminMgmtSidebar.ts (Plan 02 — global lifted via Code.ts)
    - apps-script/src/lib/log.ts (log helper — first-time import in onOpen.ts)
  provides:
    - "showEvictionSidebar admin-gated at opener + 3 callbacks (ADMIN-02 closed at code layer)"
    - "onOpen lazy bootstrapGuildAdmins() call (ADMIN-01 bootstrap closed at code layer)"
    - "2 new menu items wired (Manage Admins…, Initialize Admin Allowlist (manual))"
  affects:
    - apps-script/src/__tests__/showEvictionSidebar.test.ts (re-seeded for guard; Test 12 reframed)
tech_stack:
  added: []
  patterns:
    - "Server-side admin gate (normalizeEmail(Session.getEffectiveUser().getEmail()) → isAdmin → modal alert; defense-in-depth requireAdminOrThrow on every callback)"
    - "Lazy onOpen bootstrap with try/catch (D-01 — onOpen MUST NOT throw; bootstrapGuildAdmins is internally idempotent + lock-wrapped + returns silently on lock_busy)"
    - "Unicode horizontal-ellipsis (U+2026) menu label convention (Manage Admins…)"
key_files:
  created:
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SMOKE.md
  modified:
    - apps-script/src/triggers/showEvictionSidebar.ts
    - apps-script/src/triggers/onOpen.ts
    - apps-script/src/__tests__/showEvictionSidebar.test.ts
    - apps-script/appsscript.json
decisions:
  - "Test 12 reframed (Rule 1): pre-Phase-7 it asserted initiated_by='unknown' soft-fallback; post-Phase-7 the same Session call drives the admin guard FIRST and fail-closes empty. The audit-log soft-fallback is residual defensive code (the only path to it would be Session returning a non-empty admin email at the guard and an empty value at the later log call — non-physical)."
  - "seedMetaWithAdmins helper consolidates schema_version + guild_admins + workbook_owner_floor in one seedMeta call (seedMeta REPLACES the _meta sheet; cannot append)."
  - "Both Session.getEffectiveUser callsites in commitEviction (new admin guard + existing audit-log lookup) coexist per D-06; the new one fail-closes empty for authorization, the existing one soft-falls-back to 'unknown' for audit logging."
  - "Comment in onOpen.ts header references 'Manage Admins…' for traceability; this means the literal string matches 2 lines (header comment + menu .addItem). The acceptance criterion's spirit (one .addItem with the Unicode ellipsis label) is honored."
  - "(Smoke deviation §1) Latent v1.0 OAuth-scope bug fixed inline as Rule 3 mid-smoke: appsscript.json was missing https://www.googleapis.com/auth/userinfo.email; under consumer @gmail.com accounts Session.getEffectiveUser().getEmail() silently returned '' so every Phase 7 admin guard fail-closed for every guildie. Fix shipped as commit 544bef8; non-sensitive scope; no Production-consent or Google verification audit triggered. Side effect: retires the silent v1.0 'unknown' initiated_by audit-log fallback bug."
metrics:
  duration_seconds_total: ~3600
  duration_human_total: "~60 minutes (Tasks 1-3 ~10 min; Task 4 clasp push + 5-hook smoke + OAuth fix + redeploy ~50 min)"
  completed_date: "2026-05-12 (SHIPPED + UAT-verified)"
  tasks_completed: 4
  tests_added: 0
  tests_modified: 1 (Test 12 reframed)
  full_suite_total: 324
  smoke_hooks_passed: 5 / 5
---

# Phase 7 Plan 03: Eviction Guard + onOpen + Smoke Summary (SHIPPED 2026-05-12)

Phase 7 admin-allowlist + eviction-enforcement closure: admin policy module (Plan 01) wired into the existing eviction sidebar (admin guard at opener + 3 callbacks) and onOpen.ts (lazy bootstrap + 2 new menu items), then deployed to the dev workbook via `clasp push` and verified live against all 5 verification hooks from 07-CONTEXT.md §verification_hooks. **5/5 hooks PASS.** A latent v1.0 OAuth-scope bug surfaced and was retired inline (commit `544bef8`) — the original `appsscript.json` did not declare `userinfo.email`, so under consumer @gmail.com accounts `Session.getEffectiveUser().getEmail()` returned empty and Phase 7's correct fail-closed admin guards exposed the gap immediately.

Phase 7 = SHIPPED + UAT-VERIFIED. ADMIN-01, ADMIN-02, ADMIN-03 all closed. Schema impact zero (`_meta.schema_version=3`, `WatcherMaxSchemaVersion=3` both untouched). v1.0.1 milestone now at 2/3 phases shipped; Phase 8 unblocked.

## Outcome (Tasks 1–4, SHIPPED)

- **4 files modified, 1 created** (07-03-SMOKE.md).
- **5 commits** (3 task commits + 1 interim doc commit + 1 mid-smoke fix commit).
- Apps-script test suite stayed **324/324 GREEN** throughout (Task 1 staged the seed; Task 2 added the guard with one test reframe; Task 3 added the menu wiring).
- Typecheck (`npx tsc --noEmit`) clean; build (`npm run build`) clean.
- `dist/Code.js` contains: `showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`, `Manage Admins`, `Initialize Admin Allowlist (manual)`, `Only guild officers can evict members`, `Only guild officers can manage admins`, `bootstrapGuildAdmins(` (3 references — lazy onOpen + 2 admin.ts internals).
- `clasp push` shipped `dist/Code.js` + `dist/appsscript.json` to dev workbook script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`.
- 5/5 verification hooks PASS live against the dev workbook.

### Commits

| Hash | Message | Files |
|------|---------|-------|
| `1937fd0` | test(07-03): seed _meta.guild_admins in showEvictionSidebar tests | apps-script/src/__tests__/showEvictionSidebar.test.ts |
| `7f7ffb0` | feat(07-03): admin-guard showEvictionSidebar opener + 3 callbacks | apps-script/src/triggers/showEvictionSidebar.ts, apps-script/src/__tests__/showEvictionSidebar.test.ts |
| `c3c0033` | feat(07-03): wire onOpen lazy bootstrap + Manage Admins menu items | apps-script/src/triggers/onOpen.ts |
| `2d03581` | docs(07-03): interim SUMMARY + STATE + ROADMAP — code-complete pending smoke | `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md` + STATE.md + ROADMAP.md |
| `544bef8` | fix(07-03): add userinfo.email OAuth scope to Apps Script manifest | `apps-script/appsscript.json` (Rule 3 mid-smoke fix; see Deviations §3 below) |

## Files Modified (absolute paths)

- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\triggers\showEvictionSidebar.ts` — +1 import line (`isAdmin`, `requireAdminOrThrow`, `normalizeEmail`); +13 lines opener guard (try/catch Session lookup → isAdmin → getUi().alert + return); +7 lines × 3 callbacks (`getEvictionEmails`, `previewEviction`, `commitEviction`) for `requireAdminOrThrow` first-statement guard. Net +35 lines. Existing 30-day-grace, LockService.tryLock(30000), audit-log JSON-array malformed-tolerant parse, and `'unknown'` initiated_by soft-fallback are byte-for-byte unchanged.
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\triggers\onOpen.ts` — +2 import lines (`bootstrapGuildAdmins` from `'../lib/admin'`; `log` from `'../lib/log'`); +6 lines try/catch lazy bootstrap at top of `onOpen()`; +1 line `.addItem('Manage Admins…', 'showAdminMgmtSidebar')` between Evict Guildie and Set Theme; +2 lines `.addSeparator().addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')` after Run Migration (v=2 legacy). File-header comment updated to reference Phase 7. Net +20 lines (counting comment + import lines).
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\__tests__\showEvictionSidebar.test.ts` — Added `seedMetaWithAdmins(state, adminEmails, extraRows?, floor?)` helper (consolidates schema_version + guild_admins + workbook_owner_floor in one seedMeta call). All 4 `beforeEach` blocks switched from `seedMeta(state, [['schema_version', '3']])` to `seedMetaWithAdmins(state, ['officer@example.com'])`. Test 11 (existing eviction_log preserve) uses the `extraRows` arg. Test 12 reframed (Rule 1; details below).
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\appsscript.json` — added `https://www.googleapis.com/auth/userinfo.email` to the `oauthScopes` array (mid-smoke Rule 3 fix; commit `544bef8`). The original 4 scopes (`spreadsheets.currentonly`, `script.external_request`, `script.scriptapp`, `script.container.ui`) preserved; manifest now declares 5 scopes. Non-sensitive scope per Google's OAuth classification; no Production-consent change required.

## Files Created (absolute paths)

- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\.planning\phases\07-admin-allowlist-eviction-enforcement\07-03-SMOKE.md` — dev-workbook smoke evidence per Plan 03 Task 4 output spec. Captures clasp-push output, the 5-hook verification matrix with PASS results + one-line evidence each, the latent OAuth-scope bug discovery + fix narrative, and the Phase 7 ship verdict.

## Test Coverage (12/12 PASS — re-seeded; Test 12 reframed)

All 12 pre-existing eviction-sidebar tests still PASS post-guard. Test count unchanged (12). One test (Test 12) had its assertion repurposed.

| Test | Status | Scenario |
|------|--------|----------|
| Test 1 | PASS | sidebar opens at 300px (admin caller) |
| Test 2 | PASS | getEvictionEmails distinct sorted active emails |
| Test 3 | PASS | getEvictionEmails partial-removal NOT excluded |
| Test 4 | PASS | previewEviction happy path: chars + graceUntil ISO+30d |
| Test 5 | PASS | previewEviction no chars (does NOT throw) |
| Test 6 | PASS | commitEviction happy path (flips is_removed, appends entry) |
| Test 7 | PASS | commitEviction idempotent for already-removed rows |
| Test 8 | PASS | commitEviction lock failure throws, no writes |
| Test 9 | PASS | commitEviction missing _char_owner throws |
| Test 10 | PASS | commitEviction invalid email throws |
| Test 11 | PASS | commitEviction appends to existing eviction_log |
| Test 12 | PASS (REFRAMED) | Phase 7 admin guard: empty Session email fail-closes (D-06 auth path) |

**Test 12 reframe (Rule 1 deviation):** Pre-Phase-7 the test asserted the audit-log soft-fallback (`initiated_by='unknown'`) when `Session.getEffectiveUser().getEmail()` returns `''`. Post-Phase-7, the SAME Session call drives the new admin guard FIRST (line 47 of `showEvictionSidebar.ts`), which fail-closes empty per `requireAdminOrThrow`. The audit-log soft-fallback is now residual defensive code — the only path to reach it would be Session returning a non-empty admin email at the guard and an empty value at the later log call, which is non-physical (single Session lookup; both calls return identical strings). The test now asserts the new auth boundary: empty Session → `not_authorized` throw + no `_char_owner` writes + no `eviction_log` write.

## Verification Hook 5 — Schema Untouched (PASS)

| Gate | Expected | Actual |
|------|----------|--------|
| `Select-String apps-script/src/triggers/showEvictionSidebar.ts -Pattern "schema_version"` | 0 matches | 0 matches ✓ |
| `Select-String apps-script/src/triggers/onOpen.ts -Pattern "schema_version"` | 0 matches | 0 matches ✓ |
| `apps-script/src/lib/migrations.ts` `writeMetaRow('_meta', 'schema_version', '3')` | 1 match (unchanged from baseline) | 1 match ✓ |
| `internal/sheet/client.go` `WatcherMaxSchemaVersion = 3` | 1 match | 1 match ✓ |

`_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. No watcher rebuild required for Phase 7 (consistent with Phase 7 ROADMAP success criterion 5).

## Acceptance Criteria — All PASS (Tasks 1–3)

### Task 1 acceptance criteria

- ✓ `Select-String apps-script/src/__tests__/showEvictionSidebar.test.ts -Pattern "guild_admins"` matches 2 lines (≥1 required).
- ✓ `Select-String ... -Pattern "workbook_owner_floor"` matches 2 lines (≥1 required).
- ✓ `it(` count: 12 (≥ existing-count required; no tests deleted).
- ✓ `npx vitest run showEvictionSidebar.test` exits 0; 12/12 PASS.
- ✓ `npm test` exits 0 (full apps-script suite stays 324/324 GREEN — staged seed is benign because no code reads `_meta.guild_admins` until Task 2).

### Task 2 acceptance criteria

- ✓ `Select-String apps-script/src/triggers/showEvictionSidebar.ts -Pattern "from '../lib/admin'"` matches 1 line.
- ✓ `... -Pattern "isAdmin(callerEmail)"` matches 1 line (opener guard).
- ✓ `... -Pattern "requireAdminOrThrow(callerEmail)"` matches 3 lines (one per callback).
- ✓ `... -Pattern "Only guild officers can evict members" -SimpleMatch` matches 1 line.
- ✓ `... -Pattern "30 * 24 * 60 * 60 * 1000" -SimpleMatch` matches 1 line (existing GRACE_MS unchanged).
- ✓ `... -Pattern "LockService.getDocumentLock().tryLock(30000)"` matches 1 line (existing commit envelope unchanged).
- ✓ `npx tsc --noEmit` exits 0.
- ✓ `npm test` exits 0; 324/324 PASS (Test 12 reframed but still PASS).
- ✓ `npm run build` exits 0; `dist/Code.js` produces.

### Task 3 acceptance criteria

- ✓ `Select-String apps-script/src/triggers/onOpen.ts -Pattern "from '../lib/admin'"` matches 1 line.
- ✓ `... -Pattern "from '../lib/log'"` matches 1 line.
- ✓ `... -Pattern "bootstrapGuildAdmins()"` matches 1 line.
- ✓ `... -Pattern "onOpen.bootstrap_failed" -SimpleMatch` matches 1 line.
- ✓ `... -Pattern "Manage Admins…" -SimpleMatch` matches 2 lines (one in header comment, one in menu chain — see deviation below).
- ✓ `... -Pattern "Initialize Admin Allowlist (manual)" -SimpleMatch` matches 1 line.
- ✓ `... -Pattern "showAdminMgmtSidebar" -SimpleMatch` matches 1 line.
- ✓ `... -Pattern "bootstrapGuildAdminsManual" -SimpleMatch` matches 1 line.
- ✓ `... -Pattern "Manage Admins..." -SimpleMatch` matches 0 lines (no ASCII three-dots).
- ✓ `... -Pattern ".addToUi()"` matches 1 line (unchanged).
- ✓ `npx tsc --noEmit` exits 0.
- ✓ `npm run build` exits 0; `dist/Code.js` contains `Manage Admins` (1 match), `Initialize Admin Allowlist (manual)` (2 matches — one in source map ref + one in compiled body), `bootstrapGuildAdminsManual` (6 references — global registration + menu string + admin.ts source).
- ✓ `npm test` exits 0 (324/324).

## Deviations from Plan

**Three deviations total — two minor (Tasks 2 + 3) and one significant (mid-smoke Task 4).**

1. **(Rule 1, Task 2) Test 12 assertion reframed.** The plan's `<behavior>` says: "Existing `showEvictionSidebar.test.ts` tests continue to pass — they are extended so every test that calls eviction callbacks first seeds `_meta.guild_admins=[testCallerEmail]`." Test 12 specifically tests the audit-log soft-fallback when `Session.getEffectiveUser` returns empty. After the Task 2 guard lands, that exact Session call (now the FIRST statement of `commitEviction`) fail-closes via `requireAdminOrThrow` — the test's original assertion (`initiated_by='unknown'`) becomes unreachable. Reframing the test to assert the new auth boundary (empty Session → `not_authorized` throw + no writes) preserves the test's intent (exercising the empty-Session branch) while honoring the new sequence-load-bearing guard. Same it() count (12); same line-of-coverage; same `installSessionMock('')` setup. The audit-log `'unknown'` soft-fallback code path is NOT removed — it remains in `commitEviction` as residual defensive code (the threat model's T-07-03-04 covers Stackdriver visibility regardless).

2. **(Rule 3, Task 3) `Manage Admins…` literal matches 2 lines, not 1.** The acceptance criterion says exactly 1. The second match is in the file-header comment (`// admin-bootstrap + 2 new menu items (Manage Admins…, Initialize…`). The comment is documentation traceability and does not affect the menu's runtime behavior. Spirit of the criterion (one `.addItem(...)` with the Unicode ellipsis label) is honored. No production code change required.

3. **(Rule 3, Task 4 — significant) Latent v1.0 OAuth-scope bug surfaced and fixed mid-smoke.** During Hook 3 (peer-add round-trip), joseph.bowen2 — correctly added to `_meta.guild_admins` and shared on the workbook as Editor — saw the "Not authorized" eviction modal alert. Root cause: the original `apps-script/appsscript.json` declared 4 scopes (`spreadsheets.currentonly`, `script.external_request`, `script.scriptapp`, `script.container.ui`), none of which grant access to user email. Under consumer @gmail.com accounts, `Session.getEffectiveUser().getEmail()` silently returns `""` without `https://www.googleapis.com/auth/userinfo.email` declared. Phase 7's correct fail-closed admin guard exposed the latent v1.0 manifest gap immediately. Fix shipped as commit `544bef8`: added `userinfo.email` to the manifest (non-sensitive scope per Google's OAuth classification — no Production-consent change, no verification-audit trigger), rebuilt, force-pushed via `npx clasp push --force`. User re-authorized at next sidebar invocation (Google's OAuth re-consent fired naturally because the scope set changed). Hook 3 then proceeded to PASS. **Side effect:** retires a pre-existing latent v1.0 bug — the eviction sidebar's `initiated_by` audit-log field has been silently writing `'unknown'` (the D-06 soft fallback) for every eviction action across the entire v1.0 release because of this same scope omission. This Phase 7 fix retires that bug too. **Threat-model alignment:** consistent with T-07-03-01 (server-side requireAdminOrThrow on every callback); the threat register correctly identified `Session.getEffectiveUser().getEmail()` as the load-bearing identity source but did not enumerate the manifest dependency. Fix closes the gap; no follow-up threat surface introduced.

These are documented per the deviation-rule protocol; all three have zero impact on the plan's `<success_criteria>` after the OAuth fix landed.

## Authentication Gates

**One gate during Task 4 smoke:** Google OAuth re-consent dialog fired for joseph.bowen2 on first eviction-sidebar invocation after the `userinfo.email` scope was added (Deviation §3). This is the standard Apps Script behavior when manifest scopes change — Google requires re-authorization on the next protected invocation. Resolved by joseph.bowen2 clicking Allow in the consent flow; sidebar then opened normally.

For Tasks 1–3 (vitest unit-test environment): none encountered.

## Task 4 — clasp push + 5-Hook Smoke (PASS)

**Push command:** `npx clasp push --force` (two invocations during the session — first deployed the original Phase 7 bundle, second deployed the manifest fix from `544bef8`). Output: `pushed 2 files` (`dist/Code.js` + `dist/appsscript.json`) on each invocation. Bundle live on dev workbook script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`.

**Operator:** workbook owner (boejowen@gmail.com). **Date:** 2026-05-12.

Full evidence in [`07-03-SMOKE.md`](07-03-SMOKE.md). Matrix:

| Hook | What | Status | Evidence (one-line) |
|------|------|--------|---------------------|
| 1 | `_meta.guild_admins` exists after first open + idempotent re-open | **PASS** | `guild_admins=["boejowen@gmail.com"]`, `workbook_owner_floor=boejowen@gmail.com`, `admin_log` contains 1 bootstrap entry; second open did NOT append a duplicate. |
| 2 | Non-admin sees alert + no eviction writes | **PASS** | With `guild_admins=["someone-else@example.com"]`, click triggered "Not authorized" modal; no `_char_owner` flips, no `eviction_log` append, no `admin_log` append. Restored allowlist. |
| 3 | Admin adds peer + peer's eviction sidebar opens | **PASS (after OAuth fix)** | Owner added `joseph.bowen2@gmail.com` via Manage Admins sidebar + shared workbook as Editor. joseph.bowen2 in second browser session opened Evict Guildie → OAuth consent fired (first-time auth incl. new userinfo.email scope) → Allow → eviction sidebar opened normally. |
| 4 | Owner-floor cannot be removed by non-floor admin | **PASS** | joseph.bowen2 (non-floor) opened Manage Admins → only joseph.bowen2 row had Remove button; boejowen@gmail.com row had `(owner)` annotation and NO Remove button. Client-side suppression working. (Server-side `'owner_floor_protected'` throw covered by admin.test.ts T18.) |
| 5 | `_meta.schema_version=3` + `WatcherMaxSchemaVersion=3` (grep gates) | **PASS** | `migrations.ts` `writeMetaRow('_meta', 'schema_version', '3')` matches 1 line (unchanged). `client.go` `WatcherMaxSchemaVersion = 3` matches 1 line (unchanged). Zero schema impact. |

## Phase 7 closes — all 3 plans + OAuth fix

**Phase 7 = SHIPPED + UAT-VERIFIED 2026-05-12.**

| Plan | Status | What shipped |
|------|--------|--------------|
| **07-01** | SHIPPED 2026-05-12 (commits `dfc3533` + `8780222`) | `apps-script/src/lib/admin.ts` (357 LOC, 9 exports: `normalizeEmail`, `getAdminList`, `isAdmin`, `requireAdminOrThrow`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`, `bootstrapGuildAdminsManual`, `appendAdminLogEntry`) + `apps-script/src/__tests__/admin.test.ts` (462 LOC, 20 tests T1–T20). Full suite 297→317 GREEN. |
| **07-02** | SHIPPED 2026-05-12 (commits `2ea0cd2` + `a5d3cb4` + `d84d684`) | `apps-script/src/triggers/showAdminMgmtSidebar.ts` (263 LOC, opener + 3 callbacks `getAdminList`/`addAdmin`/`removeAdmin` with owner-floor enforcement) + `apps-script/src/__tests__/adminMgmtSidebar.test.ts` (219 LOC, 7 tests TS1–TS7) + `Code.ts` re-exports + `build.mjs` TRIGGER_GLOBALS extension + test-helpers alert capture. Full suite 317→324 GREEN. |
| **07-03** | SHIPPED 2026-05-12 (commits `1937fd0` + `7f7ffb0` + `c3c0033` + this finalization) | `showEvictionSidebar.ts` admin-gated (opener + 3 callbacks); `onOpen.ts` lazy bootstrap + 2 new menu items; `showEvictionSidebar.test.ts` reseeded (Test 12 reframed); `clasp push` to dev workbook + 5/5 verification hooks PASS. |
| **OAuth fix** | SHIPPED mid-smoke 2026-05-12 (commit `544bef8`) | `apps-script/appsscript.json` gained `userinfo.email` scope (non-sensitive). Closed latent v1.0 bug where consumer @gmail.com `Session.getEffectiveUser().getEmail()` returned `""`, breaking every Phase 7 admin guard and silently writing `initiated_by='unknown'` for every v1.0 eviction. |

**Requirements closed:** ADMIN-01 ✓ (bootstrap primitive in 07-01 + lazy `onOpen` call-site in 07-03; Hook 1 PASS), ADMIN-02 ✓ (eviction sidebar admin-gated in 07-03; Hook 2 PASS), ADMIN-03 ✓ (admin-mgmt sidebar UX in 07-02 + owner-floor enforcement in 07-01 + 07-02; Hooks 3 + 4 PASS).

**Schema impact:** zero. Watcher rebuild: not required. Bundle deployed: `dist/Code.js` + `dist/appsscript.json` live on dev workbook.

## Self-Check: PASSED (for all of Plan 03)

- ✓ `apps-script/src/triggers/showEvictionSidebar.ts` modified on disk (admin guards inserted).
- ✓ `apps-script/src/triggers/onOpen.ts` modified on disk (lazy bootstrap + 2 menu items).
- ✓ `apps-script/src/__tests__/showEvictionSidebar.test.ts` modified on disk (seedMetaWithAdmins helper + Test 12 reframe).
- ✓ `apps-script/appsscript.json` modified on disk (userinfo.email scope added).
- ✓ `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SMOKE.md` created on disk.
- ✓ Commit `1937fd0` exists in git log (Task 1 — test seed staging).
- ✓ Commit `7f7ffb0` exists in git log (Task 2 — eviction guards).
- ✓ Commit `c3c0033` exists in git log (Task 3 — onOpen wiring).
- ✓ Commit `2d03581` exists in git log (interim doc commit before smoke).
- ✓ Commit `544bef8` exists in git log (mid-smoke OAuth scope fix).
- ✓ Full apps-script suite: 324/324 PASS verified post-Task-3.
- ✓ Typecheck: clean.
- ✓ Build: clean; `dist/Code.js` contains all Phase 7 surface (verified via grep).
- ✓ Verification hook 5 grep gates: all 4 PASS.
- ✓ All 5 dev-workbook smoke hooks PASS (live verification 2026-05-12).
- ✓ `clasp push` deployed `dist/Code.js` + `dist/appsscript.json` to dev workbook script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`.
