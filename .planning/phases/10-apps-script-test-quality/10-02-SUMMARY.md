---
phase: 10-apps-script-test-quality
plan: 02
plan_id: 10-02-admin-mgmt-inline-test
subsystem: apps-script-tests
tags: [test-coverage, admin-mgmt, inline-js, sidebar, wave2]
status: complete
completed: 2026-05-13
requirements: [TEST-03]

dependency_graph:
  requires:
    - "Plan 10-01 IIFE-fixed mountSidebar (apps-script/src/__tests__/test-helpers.ts:624) — top-level var/function declarations from the Admin-Mgmt inline script must stay scoped to the IIFE rather than leaking to globalThis"
    - "Phase 8 plan 08-02 inline-JS test pattern (bankCoin/charInfo/eviction/searchSidebar inline tests) — canonical 2-test template"
  provides:
    - "5/5 shipping sidebars now have inline-JS test coverage (Search, Eviction, Bank-Coin, Char-Info from v1.0.1 + Admin-Mgmt added this plan)"
    - "Symmetric `export function buildSidebarHtml` across all 5 sidebar trigger files (4 → 5)"
    - "Retired the v1.0.1 TEST-02 'admin-mgmt inline-JS tests deferred to v1.1' clause — Plan 10-03 can retire the wording in REQUIREMENTS.md"
  affects:
    - "Plan 10-03 ship gate — TEST-03 closed; ready for clasp push"

tech_stack:
  added: []
  patterns:
    - "Phase 8 D-03 2-test inline-JS pattern (happy + error) applied symmetrically across all 5 shipping sidebars"
    - "Test-affordance export keyword on buildSidebarHtml — already in use on 4/4 other sidebars (v1.0.1 Plan 08-02); now symmetric on all 5"

key_files:
  created:
    - "apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts (103 lines; TM1 happy + TM2 error)"
  modified:
    - "apps-script/src/triggers/showAdminMgmtSidebar.ts (line 135 — single `export` keyword affordance; 1 insertion + 1 deletion)"

decisions:
  - "Used the actual DOM IDs `#addInput` / `#addBtn` / `#msg` from `showAdminMgmtSidebar.ts` SIDEBAR_BODY (lines 187-189), NOT the placeholder IDs `#email` / `#add` / `#status` from CONTEXT.md §5 (the plan's <interfaces> block explicitly anticipated this correction)."
  - "Used `invalid_email` as the TM2 error injection (not `already-exists`) because the inline-JS `onAdd` success handler renders the `alreadyExists` case as a SUCCESS message (line 248: `setMsg('Already in list: ' + value + '.', 'success')`), so it's not a meaningful error path for the inline layer."
  - "Drained both `getAdminList` follow-up calls in TM1 (initial + post-addAdmin refresh) using the same pattern landed by Plan 10-01 WR-04 fix in searchSidebar TS1 — keeps pending-mock queue empty at test end."

metrics:
  duration: "~6 minutes (executor wall-clock, excluding npm install)"
  tasks_completed: 2
  files_created: 1
  files_modified: 1
  commits: 2
---

# Phase 10 Plan 02: Admin-Mgmt Inline-JS Test Summary

One-liner: Added 5th and final inline-JS sidebar test file (`showAdminMgmtSidebar.inline.test.ts`) with TM1 happy + TM2 error tests mirroring the 4 existing inline-test files; closed the asymmetric-export gap via a single `export` keyword on `buildSidebarHtml`; TEST-03 closed; full apps-script suite is now 337 passed + 1 failed (the searchIndex Test 4 backlog 999.30 carried over from Plan 10-01).

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add `export` keyword to `buildSidebarHtml` in showAdminMgmtSidebar.ts | `b171e00` | `apps-script/src/triggers/showAdminMgmtSidebar.ts` |
| 2 | Create `showAdminMgmtSidebar.inline.test.ts` with TM1 + TM2 | `2e3d843` | `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` |

## Surgical Edits (by file)

### Task 1 — `apps-script/src/triggers/showAdminMgmtSidebar.ts:135`

```diff
-function buildSidebarHtml(theme: Theme | null): string {
+export function buildSidebarHtml(theme: Theme | null): string {
```

Pure name-visibility affordance. No behavior change. The function body, the caller at line 69, the SIDEBAR_BODY template literal, and every other declaration in the file are unchanged. `git diff --stat` confirms exactly `1 file changed, 1 insertion(+), 1 deletion(-)`.

### Task 2 — `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` (new file, 103 lines)

Exactly 2 `it(...)` blocks inside `describe('showAdminMgmtSidebar — inline JS', () => { ... })`.

**TM1 (happy path)** — successful `addAdmin` renders success copy in `#msg` and clears `#addInput`:
1. `mountSidebar(buildSidebarHtml(null))` — mount with sheets-default theme.
2. `m.dispatchRunCall('getAdminList', { admins: ['existing@example.com'], floor: 'owner@example.com', callerEmail: 'owner@example.com' })` — drain the `init()`-enqueued initial fetch.
3. Set `#addInput.value = 'newadmin@example.com'`; click `#addBtn`.
4. `m.dispatchRunCall('addAdmin', { added: true })` — drain the write.
5. `m.dispatchRunCall('getAdminList', { admins: [...], floor: ..., callerEmail: ... })` — drain the success-handler's follow-up refresh (`showAdminMgmtSidebar.ts:250`).
6. Assert `#msg.textContent` contains `'Admin added: newadmin@example.com'`.
7. Assert `addInput.value === ''` (cleared by inline-JS line 247).

**TM2 (error path)** — `addAdmin` failure with `invalid_email` renders error copy in `#msg` and preserves `#addInput`:
1. `mountSidebar(buildSidebarHtml(null))`.
2. Drain initial `getAdminList` (same as TM1 step 2).
3. Set `#addInput.value = 'bademail'`; click `#addBtn`.
4. `m.failRunCall('addAdmin', { message: 'invalid_email' })` — triggers `withFailureHandler` → `routeError` → matches `/invalid_email/` branch.
5. Assert `#msg.textContent` contains `'Invalid email'` AND `'invalid_email'` (the inline-JS template at line 209 produces `'Invalid email: invalid_email. No changes were written.'`).
6. Assert `addInput.value === 'bademail'` (NOT cleared on failure — the failure handler does not call `input.value = ''`).
7. Assert `addBtn.disabled === false` (re-enabled by inline-JS line 252).

## Verification (plan `<verification>` hooks 1-10)

| Hook | Command | Result |
|------|---------|--------|
| 1. Typecheck | `cd apps-script && npx tsc --noEmit` | PASS (exit 0; clean) |
| 2. Full suite | `cd apps-script && npm test -- --run` | 337 PASS / 1 FAIL (failure = searchIndex Test 4 from Plan 10-01, backlog 999.30 — pre-existing, expected) |
| 3. `it(...)` count in new file | `grep -cE '^\s*it\(' showAdminMgmtSidebar.inline.test.ts` | 2 (exact match — no scope expansion) |
| 4. Export count across triggers | `grep -c 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` | 5 (was 4 pre-plan; now 5 sidebars symmetric) |
| 5. Single-keyword diff | `git diff --stat showAdminMgmtSidebar.ts` | 1 insertion + 1 deletion (single keyword swap on line 135) |
| 6. Only one trigger touched | `git diff --stat apps-script/src/triggers/` (post-commit, vs pre-plan) | Only `showAdminMgmtSidebar.ts` changed |
| 7. Lib untouched | `git diff --stat apps-script/src/lib/` (vs pre-plan) | Empty (zero files changed) |
| 8. Schema gate (Go) | `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` | 1 (schema lock holds) |
| 9. Schema gate (apps-script) | `grep -nE "writeMetaRow.*_meta.*schema_version.*'3'" apps-script/src/lib/migrations.ts` | 1 match at line 97 (`writeMetaRow('_meta', 'schema_version', '3');`) |
| 10. Existing inline tests still green | New file run + full suite | All 4 prior inline tests (TB1+TB2, TC1+TC2, TE1+TE2, TS1+TS2) still pass; no regression from the new file's coexistence |

## Pre/Post Apps-Script Suite Test Counts

| State | Test files | Tests | PASS | FAIL |
|---|---|---|---|---|
| Pre-plan (Plan 10-01 HEAD) | 34 | 336 | 335 | 1 (searchIndex Test 4 = 999.30) |
| Post-plan | 35 | 338 | 337 | 1 (same — unchanged) |
| Delta | +1 file | +2 tests | +2 pass | 0 |

Net: exactly +2 tests, both passing (TM1 + TM2). The 1 failure is the same pre-existing searchIndex Test 4 from Plan 10-01 (backlog 999.30 — out of scope per CONTEXT.md D-01 + Plan 10-01 SUMMARY deviation 2).

## Pre/Post Inline-JS Test Coverage

| Sidebar | Pre | Post | Delta |
|---|---|---|---|
| Bank-Coin (`bankCoinSidebar.inline.test.ts`) | 2 (TB1, TB2) | 2 (TB1, TB2) | 0 |
| Char-Info (`charInfoSidebar.inline.test.ts`) | 2 (TC1, TC2) | 2 (TC1, TC2) | 0 |
| Eviction (`evictionSidebar.inline.test.ts`) | 2 (TE1, TE2) | 2 (TE1, TE2) | 0 |
| Search (`searchSidebar.inline.test.ts`) | 2 (TS1, TS2) | 2 (TS1, TS2) | 0 |
| **Admin-Mgmt (`showAdminMgmtSidebar.inline.test.ts`)** | **0 (deferred)** | **2 (TM1, TM2)** | **+2** |
| **Total** | **8 (4/5 sidebars)** | **10 (5/5 sidebars)** | **+2 / +1 sidebar** |

## Must-Haves Verification

All 7 truths from the plan frontmatter `must_haves.truths` are satisfied:

1. ✅ New file exists at `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` with exactly 2 `it(...)` blocks (TM1 happy + TM2 error). Verified: `grep -cE '^\s*it\(' ... = 2`.
2. ✅ TM1 mounts via IIFE-fixed `mountSidebar`, fills `#addInput`, clicks `#addBtn`, resolves `addAdmin` + follow-up `getAdminList`, asserts `'Admin added: newadmin@example.com'` in `#msg` AND `#addInput.value === ''`. Test passes.
3. ✅ TM2 mounts, fills `#addInput`, clicks, fails `addAdmin` with `{ message: 'invalid_email' }`, asserts `'Invalid email'` in `#msg` AND `#addInput.value === 'bademail'` (preserved). Test passes.
4. ✅ Imports match the bankCoin template verbatim: `import { describe, it, expect, beforeEach } from 'vitest'; import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers'; import { buildSidebarHtml } from '../triggers/showAdminMgmtSidebar';`. Verified via `grep -nE "import" ...` (3 import lines, exact shape).
5. ✅ Total apps-script vitest count is 338 (336 baseline + 2 new TM1/TM2). Plan 10-01's searchIndex Test 4 deviation continues — 337 passed + 1 failed (999.30 carried over) as expected.
6. ✅ Single production-code edit is exactly the `export` keyword on `showAdminMgmtSidebar.ts:135`. `grep -c 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` returns 5 (was 4 pre-plan). `git diff --stat apps-script/src/triggers/showAdminMgmtSidebar.ts` shows `1 file changed, 1 insertion(+), 1 deletion(-)`.
7. ✅ Schema lock unchanged: `WatcherMaxSchemaVersion = 3` (1 grep match in `internal/sheet/client.go`); `writeMetaRow('_meta', 'schema_version', '3')` at `apps-script/src/lib/migrations.ts:97`.

## Confirmation of Single Production-Code Edit

`git diff --stat` evidence (Task 1 commit `b171e00`):

```
 apps-script/src/triggers/showAdminMgmtSidebar.ts | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)
```

`git diff --stat apps-script/src/triggers/` from base `710a7d3` to HEAD shows only `showAdminMgmtSidebar.ts` changed (1 insertion / 1 deletion). No other trigger file modified.

`git diff --stat apps-script/src/lib/` from base `710a7d3` to HEAD: empty (no lib changes).

## Deviations from Plan

### 1. [Plan-anticipated — Task 2 <action> guidance] CONTEXT.md §5 placeholder DOM IDs corrected via `<interfaces>` block

- **Found during:** Task 2 — read-only inspection of `apps-script/src/triggers/showAdminMgmtSidebar.ts` SIDEBAR_BODY (allowed per CONTEXT.md D-01 + Plan's `<interfaces>` block explicit note).
- **Issue:** CONTEXT.md §Specifics §5 used placeholder DOM IDs `#email` / `#add` / `#status`. Actual IDs in the trigger are `#addInput` / `#addBtn` / `#msg` (verified at lines 187-189 of SIDEBAR_BODY).
- **Resolution:** The Plan's `<interfaces>` block already mapped placeholders → actual IDs and instructed the executor to use the real ones. Used the actual IDs in the test file. No silent reinterpretation; the planner front-loaded this correction.
- **Files modified:** `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` (created with correct IDs from the start).

### 2. [Carried over from Plan 10-01 — out of scope] searchIndex Test 4 still failing (backlog 999.30)

- **Found during:** Full-suite verification (`npm test -- --run`).
- **Issue:** `searchIndex.test.ts` Test 4 (`returns the items within edit-distance ≤2 of the query (exact pair)`) still fails post-plan with `Expected ['Cloak of Confusion', 'Cloak of Flames']; Received []`. This is the same failure surfaced by Plan 10-01's WR-03 try/catch removal (Plan 10-01 SUMMARY deviation 2; backlog 999.30 candidate).
- **Why this is the intended outcome, not a bug:**
  - Plan 10-01 SUMMARY explicitly surfaced this for Plan 10-03 ship-gate decision (autonomous=false). Plan 10-02's scope discipline explicitly states "This is OUT OF SCOPE for 10-02. Do NOT investigate or fix it. The full test suite will be 1 failed / 337+ passed after your work; that's expected."
  - Plan 10-02 success criterion expected: "EXACTLY: 1 failed (searchIndex Test 4 from Plan 10-01's 999.30) + 337+ passed (335 from 10-01 + 2 new Admin-Mgmt tests)" — exact match achieved (337 passed + 1 failed).
- **Fix:** None applied (explicitly forbidden by Plan 10-02 scope discipline). Surfaced to Plan 10-03 ship gate (999.30 v1.1 backlog candidate).

### Note for Plan 10-03 Ship Gate

- **TEST-03 closed.** 5/5 inline-JS sidebars now have D-03 happy+error coverage; symmetric `export function buildSidebarHtml` across all 5 sidebar triggers.
- **Apps-script vitest suite:** 337 passed + 1 failed (999.30 carried over from 10-01). Plan 10-03 must decide on 999.30 before `clasp push` — either (a) fix the `didYouMean` assertion to match whole-string Levenshtein semantics (`toEqual([])` and rename test), (b) fix `didYouMean` production code for first-word-aware matching, or (c) defer to v1.1 and proceed with push (the failing test does not affect runtime behavior; it asserts a planner-locked grep gate that turns out to be arithmetically wrong under whole-string distance).
- **Ready for clasp push from owner's machine** once 999.30 is decided.

## Authentication Gates

None. Plan executed end-to-end autonomously without any auth gate.

## Known Stubs

None. Plan creates one test file + a single-keyword export affordance; no stubs, no placeholder values, no TODO/FIXME markers introduced.

## Threat Flags

None. Per the plan's `<threat_model>`:
- T-10-02-01 (Tampering): test file under `apps-script/src/__tests__/` — not bundled into `dist/Code.js`; no production-shipping surface.
- T-10-02-02 (Info Disclosure): all fixture emails use `@example.com` placeholders; zero PII.
- T-10-02-03 (Spoofing): inline tests exercise CLIENT-side DOM + `google.script.run` mocks only; do NOT bypass server-side `requireAdminOrThrow` gate (executes on Apps Script server, not in JSDOM).
- T-10-02-04 (EoP): `export` keyword on `buildSidebarHtml` does NOT change Apps Script runtime semantics — Apps Script trigger system finds triggers by global function name (set by esbuild footer), and `buildSidebarHtml` is a build helper (not a trigger callable). Pure ESM-import visibility.

No new security-relevant surface introduced.

## Self-Check: PASSED

- ✅ `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` — exists on disk (4254 bytes).
- ✅ `apps-script/src/triggers/showAdminMgmtSidebar.ts` — line 135 now reads `export function buildSidebarHtml(theme: Theme | null): string {`.
- ✅ Commit `b171e00` exists (Task 1 — export affordance).
- ✅ Commit `2e3d843` exists (Task 2 — new test file).
- ✅ `git diff --stat apps-script/src/triggers/showAdminMgmtSidebar.ts` (vs pre-plan): 1 insertion + 1 deletion.
- ✅ `git diff --stat apps-script/src/lib/` (vs pre-plan): empty.
- ✅ `grep -c 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` = 5 (4 pre-plan + 1 new = symmetric).
- ✅ `grep -cE '^\s*it\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` = 2 (TM1 + TM2; no scope expansion).
- ✅ `grep -cE 'mountSidebar\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` = 2 (one per test).
- ✅ `grep -cE 'dispatchRunCall\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` = 4 (TM1: initial getAdminList + addAdmin + follow-up getAdminList = 3; TM2: initial getAdminList = 1; total 4).
- ✅ `grep -cE 'failRunCall\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` = 1 (TM2 only).
- ✅ Full apps-script suite: 337 passed + 1 failed (searchIndex Test 4 = 999.30 carried over from 10-01).
- ✅ TM1 + TM2 standalone run: 2/2 PASS in 133ms.
- ✅ `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` — 1 grep match (schema lock holds).
- ✅ `writeMetaRow('_meta', 'schema_version', '3')` at `apps-script/src/lib/migrations.ts:97` — 1 match (schema lock holds).
