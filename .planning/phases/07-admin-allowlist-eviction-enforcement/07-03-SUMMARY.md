---
phase: 07-admin-allowlist-eviction-enforcement
plan: 03
subsystem: apps-script
tags: [apps-script, eviction, onopen, menu, admin-guard, lazy-bootstrap, ship-gate, awaiting-smoke]
status: code-complete-pending-smoke
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
  created: []
  modified:
    - apps-script/src/triggers/showEvictionSidebar.ts
    - apps-script/src/triggers/onOpen.ts
    - apps-script/src/__tests__/showEvictionSidebar.test.ts
decisions:
  - "Test 12 reframed (Rule 1): pre-Phase-7 it asserted initiated_by='unknown' soft-fallback; post-Phase-7 the same Session call drives the admin guard FIRST and fail-closes empty. The audit-log soft-fallback is residual defensive code (the only path to it would be Session returning a non-empty admin email at the guard and an empty value at the later log call — non-physical)."
  - "seedMetaWithAdmins helper consolidates schema_version + guild_admins + workbook_owner_floor in one seedMeta call (seedMeta REPLACES the _meta sheet; cannot append)."
  - "Both Session.getEffectiveUser callsites in commitEviction (new admin guard + existing audit-log lookup) coexist per D-06; the new one fail-closes empty for authorization, the existing one soft-falls-back to 'unknown' for audit logging."
  - "Comment in onOpen.ts header references 'Manage Admins…' for traceability; this means the literal string matches 2 lines (header comment + menu .addItem). The acceptance criterion's spirit (one .addItem with the Unicode ellipsis label) is honored."
metrics:
  duration_seconds_so_far: ~600
  duration_human_so_far: "~10 minutes (Tasks 1-3; Task 4 awaits user clasp push + smoke)"
  completed_date_partial: "2026-05-12 (code-complete; smoke pending)"
  tasks_completed_so_far: 3
  tests_added: 0
  tests_modified: 1 (Test 12 reframed)
  full_suite_total: 324
---

# Phase 7 Plan 03: Eviction Guard + onOpen + Smoke Summary (INTERIM — code-complete, awaiting clasp-push smoke)

ADMIN-01 + ADMIN-02 source-code closure shipped via 3 atomic commits. Admin policy module (Plan 01) wired into the existing eviction sidebar (admin guard) and onOpen.ts (lazy bootstrap + 2 new menu items). Apps-script test suite stayed 324/324 GREEN throughout the plan; typecheck + build clean; dist/Code.js contains every Phase 7 surface (5 new globals + 2 new menu labels + the eviction-sidebar guard alert copy).

The remaining Task 4 (clasp push to dev workbook + interactive 5-hook smoke) is a `checkpoint:human-verify` and pauses execution. The user (workbook owner) holds the clasp OAuth credentials in `~/.clasprc.json` — Claude cannot run `clasp push`. Once the user completes the smoke and returns evidence, this SUMMARY will be re-finalized with the smoke results and Phase 7 ship verdict.

## Outcome (Tasks 1–3, code-complete)

- **3 files modified**, 0 created.
- **3 commits**, 1 per task, all autonomous (no checkpoint, no auth gate). One Rule 1 deviation (Test 12 reframed).
- Apps-script test suite stayed **324/324 GREEN** throughout (Task 1 staged the seed; Task 2 added the guard with one test reframe; Task 3 added the menu wiring).
- Typecheck (`npx tsc --noEmit`) clean; build (`npm run build`) clean.
- `dist/Code.js` contains: `showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`, `Manage Admins`, `Initialize Admin Allowlist (manual)`, `Only guild officers can evict members`, `Only guild officers can manage admins`, `bootstrapGuildAdmins(` (3 references — lazy onOpen + 2 admin.ts internals).

### Commits

| Hash | Message | Files |
|------|---------|-------|
| `1937fd0` | test(07-03): seed _meta.guild_admins in showEvictionSidebar tests | apps-script/src/__tests__/showEvictionSidebar.test.ts |
| `7f7ffb0` | feat(07-03): admin-guard showEvictionSidebar opener + 3 callbacks | apps-script/src/triggers/showEvictionSidebar.ts, apps-script/src/__tests__/showEvictionSidebar.test.ts |
| `c3c0033` | feat(07-03): wire onOpen lazy bootstrap + Manage Admins menu items | apps-script/src/triggers/onOpen.ts |

## Files Modified (absolute paths)

- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\triggers\showEvictionSidebar.ts` — +1 import line (`isAdmin`, `requireAdminOrThrow`, `normalizeEmail`); +13 lines opener guard (try/catch Session lookup → isAdmin → getUi().alert + return); +7 lines × 3 callbacks (`getEvictionEmails`, `previewEviction`, `commitEviction`) for `requireAdminOrThrow` first-statement guard. Net +35 lines. Existing 30-day-grace, LockService.tryLock(30000), audit-log JSON-array malformed-tolerant parse, and `'unknown'` initiated_by soft-fallback are byte-for-byte unchanged.
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\triggers\onOpen.ts` — +2 import lines (`bootstrapGuildAdmins` from `'../lib/admin'`; `log` from `'../lib/log'`); +6 lines try/catch lazy bootstrap at top of `onOpen()`; +1 line `.addItem('Manage Admins…', 'showAdminMgmtSidebar')` between Evict Guildie and Set Theme; +2 lines `.addSeparator().addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')` after Run Migration (v=2 legacy). File-header comment updated to reference Phase 7. Net +20 lines (counting comment + import lines).
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\__tests__\showEvictionSidebar.test.ts` — Added `seedMetaWithAdmins(state, adminEmails, extraRows?, floor?)` helper (consolidates schema_version + guild_admins + workbook_owner_floor in one seedMeta call). All 4 `beforeEach` blocks switched from `seedMeta(state, [['schema_version', '3']])` to `seedMetaWithAdmins(state, ['officer@example.com'])`. Test 11 (existing eviction_log preserve) uses the `extraRows` arg. Test 12 reframed (Rule 1; details below).

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

**Two minor deviations, both surfaced and explained:**

1. **(Rule 1, Task 2) Test 12 assertion reframed.** The plan's `<behavior>` says: "Existing `showEvictionSidebar.test.ts` tests continue to pass — they are extended so every test that calls eviction callbacks first seeds `_meta.guild_admins=[testCallerEmail]`." Test 12 specifically tests the audit-log soft-fallback when `Session.getEffectiveUser` returns empty. After the Task 2 guard lands, that exact Session call (now the FIRST statement of `commitEviction`) fail-closes via `requireAdminOrThrow` — the test's original assertion (`initiated_by='unknown'`) becomes unreachable. Reframing the test to assert the new auth boundary (empty Session → `not_authorized` throw + no writes) preserves the test's intent (exercising the empty-Session branch) while honoring the new sequence-load-bearing guard. Same it() count (12); same line-of-coverage; same `installSessionMock('')` setup. The audit-log `'unknown'` soft-fallback code path is NOT removed — it remains in `commitEviction` as residual defensive code (the threat model's T-07-03-04 covers Stackdriver visibility regardless).

2. **(Rule 3, Task 3) `Manage Admins…` literal matches 2 lines, not 1.** The acceptance criterion says exactly 1. The second match is in the file-header comment (`// admin-bootstrap + 2 new menu items (Manage Admins…, Initialize…`). The comment is documentation traceability and does not affect the menu's runtime behavior. Spirit of the criterion (one `.addItem(...)` with the Unicode ellipsis label) is honored. No production code change required.

These are documented per Rules 1/3 of the deviation protocol; both have zero impact on the plan's `<success_criteria>`.

## Authentication Gates

**None encountered for Tasks 1–3.** Pure Vitest unit-test environment; no Apps Script bindings invoked. Task 4 is a `checkpoint:human-verify` requiring the user to run `clasp push` from their machine (clasp OAuth credentials in `~/.clasprc.json` are not accessible to Claude per `docs/apps-script-deploy.md`).

## Task 4 Status: AWAITING USER SMOKE

The clasp-push + 5-hook dev-workbook smoke is the ship gate. See "Smoke Verification Matrix (PENDING)" below — the user fills in the PASS/FAIL/notes column after running the runbook from the plan.

## Smoke Verification Matrix (PENDING — user fills after smoke)

| Hook | What | Status | Evidence |
|------|------|--------|----------|
| 1 | `_meta.guild_admins` exists after first open + idempotent re-open | PENDING | (user pastes here) |
| 2 | Non-admin sees alert + no eviction writes | PENDING | (user pastes here) |
| 3 | Admin adds peer + peer's eviction sidebar opens | PENDING | (user pastes here) |
| 4 | Owner-floor cannot be removed by non-floor admin | PENDING | (user pastes here) |
| 5 | `_meta.schema_version=3` + `WatcherMaxSchemaVersion=3` | PASS (grep gates ✓) | Verified locally; user re-greps post-push for confirmation |

## Phase 7 Ship Verdict (PENDING)

- **Code layer:** ADMIN-01 closed (Plans 01 + 03), ADMIN-02 closed (Plan 03), ADMIN-03 closed (Plan 02). All 9 admin policy primitives, the admin-mgmt sidebar, the eviction guards, and the onOpen lazy bootstrap + 2 menu items are in `dist/Code.js`.
- **Integration layer:** PENDING the user's `clasp push` + smoke. Once smoke 5/5 PASS, Phase 7 = SHIPPED + UAT-verified.
- **If a smoke hook FAILS:** classify as either trivial-fix (patch + republish; same Plan 03 scope) or follow-up plan (gap-plan via `/gsd-plan-phase 7 --gaps`). Do not advance STATE.md to `phase_7_complete` until 5/5 PASS.

## Self-Check: PASSED (for Tasks 1–3)

- ✓ `apps-script/src/triggers/showEvictionSidebar.ts` modified on disk (admin guards inserted).
- ✓ `apps-script/src/triggers/onOpen.ts` modified on disk (lazy bootstrap + 2 menu items).
- ✓ `apps-script/src/__tests__/showEvictionSidebar.test.ts` modified on disk (seedMetaWithAdmins helper + Test 12 reframe).
- ✓ Commit `1937fd0` exists in git log (Task 1 — test seed staging).
- ✓ Commit `7f7ffb0` exists in git log (Task 2 — eviction guards).
- ✓ Commit `c3c0033` exists in git log (Task 3 — onOpen wiring).
- ✓ Full apps-script suite: 324/324 PASS verified post-Task-3.
- ✓ Typecheck: clean.
- ✓ Build: clean; `dist/Code.js` contains all Phase 7 surface (verified via grep).
- ✓ Verification hook 5 grep gates: all 4 PASS.

Self-check for Task 4 (clasp push + smoke evidence) is the user's responsibility per the checkpoint protocol.
