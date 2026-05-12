---
phase: 07-admin-allowlist-eviction-enforcement
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - apps-script/src/lib/admin.ts
  - apps-script/src/__tests__/admin.test.ts
autonomous: true
requirements: [ADMIN-01, ADMIN-03]
tags: [apps-script, policy, lib, lockservice, unit-tests]

must_haves:
  truths:
    - "A new `apps-script/src/lib/admin.ts` module exists exporting the full Phase 7 policy surface: `normalizeEmail`, `getAdminList`, `isAdmin`, `requireAdminOrThrow`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`, `bootstrapGuildAdminsManual`, `appendAdminLogEntry`."
    - "`requireAdminOrThrow('')` throws `Error('not_authorized')` (fail-closed empty-email policy per D-06)."
    - "`requireAdminOrThrow(email)` throws `Error('not_authorized')` when `email` is not in `_meta.guild_admins` (case-insensitive compare via normalizeEmail)."
    - "`addAdmin(newEmail, callerEmail)` (where callerEmail is an admin) returns `{ added: true }`, writes the sorted+lowercased list back to `_meta.guild_admins`, and appends a `{ action: 'add', email, initiated_by }` entry to `_meta.admin_log`."
    - "`addAdmin(existingEmail, callerEmail)` is idempotent: returns `{ added: false, alreadyExists: true }`, NO _meta write, NO log entry."
    - "`removeAdmin(target, caller)` throws `Error('owner_floor_protected')` when `target === _meta.workbook_owner_floor && caller !== floor`; succeeds for self-removal (`target === caller === floor`); succeeds for any non-floor removal by an admin caller."
    - "`bootstrapGuildAdmins()` (no opts) seeds `_meta.guild_admins` and `_meta.workbook_owner_floor` from `SpreadsheetApp.getActiveSpreadsheet().getOwner().getEmail()` on first run; second call is a no-op via the idempotent check (returns `{ bootstrapped: false, reason: 'already_initialized' }`). On lock-busy returns `{ bootstrapped: false, reason: 'lock_busy' }` WITHOUT throwing (D-01 onOpen mustn't throw)."
    - "`bootstrapGuildAdmins()` when `getOwner()` returns null writes a `{ action: 'bootstrap_failed', email: '', initiated_by: 'onOpen', reason: 'owner_null' }` entry to `_meta.admin_log` and returns `{ bootstrapped: false, reason: 'owner_null' }` (no throw)."
    - "Every multi-step `_meta` write inside admin.ts is wrapped in `LockService.getDocumentLock().tryLock(30000)` — Pitfall P6 mitigation. `addAdmin`/`removeAdmin` throw `'addAdmin: lock_busy'` / `'removeAdmin: lock_busy'` on contention; `bootstrapGuildAdmins` is the documented exception that returns the lock_busy enum instead."
    - "`apps-script/src/__tests__/admin.test.ts` exercises the 20 scenarios T1–T20 mapped in 07-PATTERNS.md and ALL pass under `npm test`."
    - "`_meta.schema_version` cell value remains `3` after all admin-module operations (grep gate against test fixture sets and against `apps-script/src/lib/migrations.ts`)."
  artifacts:
    - path: apps-script/src/lib/admin.ts
      provides: "Central admin-policy module: normalizeEmail, getAdminList, isAdmin, requireAdminOrThrow, addAdmin, removeAdmin, bootstrapGuildAdmins, bootstrapGuildAdminsManual, appendAdminLogEntry"
      min_lines: 220
      contains: "requireAdminOrThrow"
    - path: apps-script/src/__tests__/admin.test.ts
      provides: "Vitest unit suite covering T1–T20 of 07-PATTERNS.md (auth fail-closed, idempotent add, owner-floor lockout, self-removal, malformed-JSON tolerance, lock-busy semantics, bootstrap idempotent + null-owner fallback)"
      min_lines: 320
      contains: "describe('admin.ts'"
  key_links:
    - from: apps-script/src/lib/admin.ts
      to: apps-script/src/lib/sheet-helpers.ts
      via: "named imports `readMetaRows`, `writeMetaRow`, `getActiveSpreadsheet`"
      pattern: "from '\\./sheet-helpers'"
    - from: apps-script/src/lib/admin.ts
      to: apps-script/src/lib/log.ts
      via: "named import `log` for structured logging at every public function"
      pattern: "from '\\./log'"
    - from: apps-script/src/__tests__/admin.test.ts
      to: apps-script/src/lib/admin.ts
      via: "direct unit test under same package; imports every public export"
      pattern: "from '\\.\\./lib/admin'"
---

<objective>
Create the central admin-policy module `apps-script/src/lib/admin.ts` (per CONTEXT.md D-05) along with its vitest unit suite. This is the dependency root of Phase 7: Plan 02 (admin-mgmt sidebar) and Plan 03 (eviction guards + onOpen bootstrap) BOTH import from this module. Nothing else in Phase 7 compiles until this plan lands.

Purpose: every admin-policy primitive lives in one file. Single source of truth for "who can do destructive things" — `isAdmin`, `requireAdminOrThrow`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`, `appendAdminLogEntry`. Tested in isolation against mocked SpreadsheetApp+Session globals so the policy is provably correct BEFORE it gets wired into the eviction sidebar or the new admin-mgmt sidebar. (a) trivial test surface (one file, one suite), (b) prevents the eviction sidebar from re-implementing the admin check with a subtle bug, (c) future-proofs 999.1 bank-coin permission lock and 999.5 self-service eviction (per CONTEXT.md §D-05 rationale).

Output: 1 new lib file (~220 lines), 1 new test file (~320 lines covering T1–T20 from 07-PATTERNS.md), zero modifications to existing files. No Code.ts re-export needed yet — admin.ts exports are internal-to-apps-script-bundle imports (Plan 02 consumes `addAdmin`/`removeAdmin`/`getAdminList` via sidebar wrappers; Plan 03 consumes `isAdmin`/`requireAdminOrThrow`/`bootstrapGuildAdmins`/`bootstrapGuildAdminsManual` via direct module imports). The ONLY global-lift required is `bootstrapGuildAdminsManual` for the new menu item, and that re-export is owned by Plan 02 (which already touches Code.ts for the sidebar exports).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md
@.planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md
@apps-script/src/triggers/showEvictionSidebar.ts
@apps-script/src/lib/sheet-helpers.ts
@apps-script/src/lib/migrations.ts
@apps-script/src/lib/log.ts
@apps-script/src/__tests__/showEvictionSidebar.test.ts
@apps-script/src/__tests__/test-helpers.ts

<interfaces>
<!-- Exact contract Plans 02 and 03 import against. Keep these signatures STABLE — Plan 02's callbacks and Plan 03's guards are written against them verbatim. -->

```typescript
// apps-script/src/lib/admin.ts — public exports

/** Lowercase + trim. Single normalization point used everywhere policy decisions are made (read, write, compare). Per CONTEXT.md §specifics "apply at THREE points". */
export function normalizeEmail(s: string | null | undefined): string;

/** Read both _meta rows. Tolerates malformed JSON in guild_admins (returns empty list — fail-closed). admins[] is sorted+lowercased on the way out. */
export function getAdminList(): { admins: string[]; floor: string };

/** Convenience: normalizes input, returns whether it's in the list. Empty string → false (fail-closed). */
export function isAdmin(email: string | null | undefined): boolean;

/** Throws `Error('not_authorized')` if !isAdmin(email). Used by every protected callback (FIRST statement). Empty/null email also throws (fail-closed per D-06). */
export function requireAdminOrThrow(email: string | null | undefined): void;

/** Lock-wrapped. Caller MUST be admin (requireAdminOrThrow). Validates email (non-empty, contains '@'). Idempotent. */
export function addAdmin(email: string, callerEmail: string): { added: boolean; alreadyExists?: boolean };

/** Lock-wrapped. Caller MUST be admin (requireAdminOrThrow). Enforces owner-floor: throws Error('owner_floor_protected') if target===floor && caller!==floor. Self-removal of floor allowed (floor row NOT updated — documented orphan per D-04). Idempotent. */
export function removeAdmin(email: string, callerEmail: string): { removed: boolean; notFound?: boolean };

/** Lazy onOpen bootstrap (D-01). Idempotent. Lock-wrapped but SWALLOWS lock_busy (returns reason, no throw — onOpen must not throw). Uses opts.seedEmail if provided (manual-fallback path), else SpreadsheetApp.getActiveSpreadsheet().getOwner().getEmail(). Writes guild_admins + workbook_owner_floor + admin_log bootstrap entry. */
export function bootstrapGuildAdmins(opts?: { seedEmail?: string; initiatedBy?: string }): { bootstrapped: boolean; seedEmail?: string; reason?: 'already_initialized' | 'owner_null' | 'lock_busy' | 'utf16_failed' };

/** Manual-fallback wrapper for the "Initialize Admin Allowlist (manual)" menu item (D-01). Reads Session.getEffectiveUser().getEmail() as seed; shows getUi().alert OK_CANCEL confirmation BEFORE writing; on success toasts via SpreadsheetApp.getActiveSpreadsheet().toast. Calls bootstrapGuildAdmins({ seedEmail, initiatedBy: 'manual_fallback' }) under the hood. */
export function bootstrapGuildAdminsManual(): void;

/** Internal helper. Reads _meta.admin_log, defensively parses JSON (malformed → fresh array + warn log), pushes the entry, writes back. NOT lock-wrapped on its own — caller is responsible for holding the lock when called inside add/remove. */
export interface AdminLogEntry {
  at: string;          // ISO8601
  action: 'add' | 'remove' | 'bootstrap' | 'bootstrap_failed';
  email: string;
  initiated_by: string;
  reason?: string;     // optional — only set for 'bootstrap_failed' entries
}
export function appendAdminLogEntry(entry: AdminLogEntry): void;
```
</interfaces>

<storage_shape>
<!-- _meta rows owned by admin.ts. All three are extend-only additions; no schema_version bump per STATE.md D-02. -->

| _meta key | Value cell shape | Example |
|-----------|------------------|---------|
| `guild_admins` | JSON.stringify(string[]) — lowercased+trimmed+sorted | `["alice@example.com","bob@example.com","jbowen@mncivic.com"]` |
| `workbook_owner_floor` | Single email string — lowercased+trimmed | `jbowen@mncivic.com` |
| `admin_log` | JSON.stringify(AdminLogEntry[]) — append-only | `[{"at":"2026-05-11T18:00:00.000Z","action":"bootstrap","email":"jbowen@mncivic.com","initiated_by":"onOpen"}]` |

The `_meta.workbook_owner_floor` row is the ONLY admin row that holds a single-string value (not JSON-array). All reads of this row use `String(row.value).toLowerCase().trim()` directly; no JSON.parse.
</storage_shape>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Write failing vitest suite for admin.ts (T1–T20 from 07-PATTERNS.md)</name>
  <files>apps-script/src/__tests__/admin.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/showEvictionSidebar.test.ts (THE template; copy the Session mock pattern at lines 42-47, the seedMeta usage, the audit-log JSON-decode assertion shape at lines 168-188, and the lock-failure pattern at lines 212-223)
    - apps-script/src/__tests__/test-helpers.ts (the `resetMocks`, `makeSheet`, `seedMeta`, `MockState`, `lockTryLockReturn` surface — confirm what's available; this test file uses ONLY what's already exported)
    - apps-script/src/triggers/showEvictionSidebar.ts:122-186 (the lock+envelope shape this module mirrors — your `addAdmin`/`removeAdmin` tests assert the same kind of writes)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md §`apps-script/src/__tests__/admin.test.ts` (the T1–T20 table is the test scenario manifest — every test must exist by name)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md §verification_hooks (the planner-MUST-cover-all-5 list — every hook has at least one test here)
  </read_first>
  <behavior>
    Tests written BEFORE admin.ts exists. Every test will RED on first run (`Cannot find module '../lib/admin'`) — that is correct TDD state.

    Test scenarios (20 total, names must match exactly for traceability to 07-PATTERNS.md):

    - **T1** `requireAdminOrThrow_emptyEmail_throwsNotAuthorized` — `requireAdminOrThrow('')` and `requireAdminOrThrow(null as unknown as string)` both throw `/not_authorized/`. D-06 fail-closed.
    - **T2** `requireAdminOrThrow_notInList_throwsNotAuthorized` — seed `guild_admins=["alice@x.com"]`; `requireAdminOrThrow('intruder@x.com')` throws.
    - **T3** `requireAdminOrThrow_caseMismatched_succeeds` — seed `guild_admins=["alice@x.com"]`; `requireAdminOrThrow('ALICE@X.COM')` does NOT throw (normalized compare).
    - **T4** `isAdmin_emptyList_returnsFalse` — no `guild_admins` row at all; `isAdmin('anyone@x.com')` returns false.
    - **T5** `isAdmin_caseInsensitiveMatch` — seed `["alice@x.com"]`; `isAdmin('Alice@X.com')` returns true.
    - **T6** `getAdminList_malformedJson_failsClosedToEmpty` — seed `guild_admins='not valid json {{'`; `getAdminList()` returns `{ admins: [], floor: '' }` and a `warn` log was emitted. D-05 fail-closed read.
    - **T7** `addAdmin_happyPath_appendsSortedLowercasedAndLogs` — seed `guild_admins=["bob@x.com"]`, floor=`bob@x.com`; call `addAdmin('Alice@X.com', 'bob@x.com')`. Assert: returns `{ added: true }`; `guild_admins` cell now contains `["alice@x.com","bob@x.com"]` (sorted, lowercased); `admin_log` has exactly 1 new entry with `action:'add'`, `email:'alice@x.com'`, `initiated_by:'bob@x.com'`, `at` ISO-8601 matched by `/^\d{4}-\d{2}-\d{2}T/`.
    - **T8** `addAdmin_alreadyExists_idempotentNoWriteNoLog` — seed `guild_admins=["alice@x.com","bob@x.com"]`; call `addAdmin('alice@x.com', 'bob@x.com')`. Assert: returns `{ added: false, alreadyExists: true }`; NO `guild_admins` write happened (state.setValuesLog filtered to `_meta` rows for `guild_admins` is length 0); NO `admin_log` entry appended.
    - **T9** `addAdmin_rejectsEmptyOrMissingAt` — call `addAdmin('', 'bob@x.com')` and `addAdmin('notanemail', 'bob@x.com')`. Both throw `/invalid_email/` (or similar). NO writes.
    - **T10** `addAdmin_lockBusy_throwsAndDoesNotWrite` — set `state.lockTryLockReturn = false`; call `addAdmin('new@x.com', 'bob@x.com')`. Assert: throws `/addAdmin: lock_busy/`; NO `_meta` writes.
    - **T11** `removeAdmin_nonFloorByNonFloorCaller_succeeds` — seed `guild_admins=["alice@x.com","bob@x.com","jbowen@x.com"]`, floor=`jbowen@x.com`; call `removeAdmin('alice@x.com', 'bob@x.com')`. Returns `{ removed: true }`; `guild_admins` is now `["bob@x.com","jbowen@x.com"]`; `admin_log` has a new `action:'remove'` entry.
    - **T12** `removeAdmin_floorByNonFloor_throwsOwnerFloorProtected` — same seed; `removeAdmin('jbowen@x.com', 'bob@x.com')` throws `/owner_floor_protected/`; NO writes happen.
    - **T13** `removeAdmin_floorByFloor_selfRemovalSucceeds` — same seed; `removeAdmin('jbowen@x.com', 'jbowen@x.com')` returns `{ removed: true }`; `guild_admins` is `["alice@x.com","bob@x.com"]`; `_meta.workbook_owner_floor` cell is UNCHANGED (still `jbowen@x.com` — orphan pointer per D-04).
    - **T14** `removeAdmin_notInList_idempotentNoWriteNoLog` — seed `guild_admins=["alice@x.com"]`; `removeAdmin('ghost@x.com', 'alice@x.com')` returns `{ removed: false, notFound: true }`; NO writes; NO log entry.
    - **T15** `bootstrapGuildAdmins_emptyMeta_writesSeedAndFloorAndLog` — `_meta` empty; mock `getOwner().getEmail()` returns `'OWNER@X.COM'`; call `bootstrapGuildAdmins()`. Assert: returns `{ bootstrapped: true, seedEmail: 'owner@x.com' }`; `guild_admins='["owner@x.com"]'`; `workbook_owner_floor='owner@x.com'`; `admin_log` has 1 entry `{action:'bootstrap', email:'owner@x.com', initiated_by:'onOpen'}`.
    - **T16** `bootstrapGuildAdmins_alreadySeeded_noOp` — seed `guild_admins=["existing@x.com"]`; call `bootstrapGuildAdmins()`. Returns `{ bootstrapped: false, reason: 'already_initialized' }`; NO writes; NO log entry.
    - **T17** `bootstrapGuildAdmins_ownerNull_writesFailedLogAndReturns` — mock `getOwner()` to return null; call `bootstrapGuildAdmins()`. Returns `{ bootstrapped: false, reason: 'owner_null' }`; NO `guild_admins` write; `admin_log` has 1 `action:'bootstrap_failed'` entry with `reason:'owner_null'`; a `warn` log was emitted.
    - **T18** `bootstrapGuildAdmins_manualOpts_usesSeedEmail` — `_meta` empty; getOwner returns null; call `bootstrapGuildAdmins({ seedEmail: 'manual@x.com', initiatedBy: 'manual_fallback' })`. Returns `{ bootstrapped: true, seedEmail: 'manual@x.com' }`; `guild_admins='["manual@x.com"]'`; `workbook_owner_floor='manual@x.com'`; `admin_log` entry has `initiated_by:'manual_fallback'`.
    - **T19** `bootstrapGuildAdmins_lockBusy_silentNoOpDoesNotThrow` — set `state.lockTryLockReturn = false`; call `bootstrapGuildAdmins()`. Returns `{ bootstrapped: false, reason: 'lock_busy' }`; DOES NOT THROW (D-01 onOpen mustn't throw); NO writes; a `warn` log was emitted with `skipped:'lock_busy'`.
    - **T20** `appendAdminLogEntry_malformedExisting_startsFreshAndWarns` — seed `admin_log='{ broken json'`; call `appendAdminLogEntry({ at:'2026-05-11T00:00:00.000Z', action:'add', email:'x@x.com', initiated_by:'y@y.com' })`. Assert: `admin_log` cell now contains exactly the new entry (JSON.parse returns array of length 1); `warn` log emitted with `malformedExistingLog: true`.

    Helpers (clone from `showEvictionSidebar.test.ts`):
    - `installSessionMock(email)` — clone verbatim from lines 42-47 with the `afterEach delete (globalThis).Session` cleanup.
    - For tests that exercise `getOwner()` (T15, T17, T18), extend the SpreadsheetApp mock so `getActiveSpreadsheet().getOwner()` returns either `{ getEmail: () => 'OWNER@X.COM' }` or `null`. If the existing mock in `test-helpers.ts` already exposes a hook for this, use it; if not, install a per-test override directly on the mock and restore in cleanup.
    - For tests that capture `log()` warnings (T6, T17, T19, T20), either use vitest's `vi.spyOn` against the `log` module's exported function, or use the existing test-helpers `state.logCalls`-style capture if one exists. Match the eviction-sidebar test's existing pattern.

    Test file conventions:
    - `import { describe, it, expect, beforeEach, afterEach } from 'vitest';`
    - `import { resetMocks, makeSheet, seedMeta, type MockState } from './test-helpers';`
    - `import * as admin from '../lib/admin';` (single namespace import — the test file is the sole consumer; this also helps with `vi.spyOn` on internals if needed).
    - Top-level `describe('admin.ts', () => { ... })` block; nested `describe` per public function (requireAdminOrThrow, isAdmin, getAdminList, addAdmin, removeAdmin, bootstrapGuildAdmins, appendAdminLogEntry) with `it(...)` per test scenario.
    - `let state: MockState;` and `beforeEach(() => { state = resetMocks(); makeSheet(state, '_meta'); });` like the eviction-sidebar test file.
    - `afterEach(() => { delete (globalThis as Record<string, unknown>).Session; })`.

    Running expectation:
    - `npm test` from `apps-script/` will RED on every test in this file (module not found). This is the expected state after Task 1.
    - Task 2 implements admin.ts to GREEN the suite.
  </behavior>
  <action>
Write `apps-script/src/__tests__/admin.test.ts` containing all 20 tests T1–T20 above. The file is purely test code; do NOT touch `apps-script/src/lib/admin.ts` in this task (that's Task 2 — the GREEN step).

Structure the file with a single top-level `describe('admin.ts', () => { ... })`, then one nested `describe` per public function. Use the exact `it(...)` names from the T1–T20 list above for traceability.

For the **SpreadsheetApp `getOwner()` mock**, the existing `test-helpers.ts` may not expose it (eviction sidebar tests don't exercise getOwner). Pattern to use inside each relevant test:

```typescript
// Install per-test override BEFORE calling the SUT. Restore in afterEach via the resetMocks fresh-state contract.
(globalThis as Record<string, unknown>).SpreadsheetApp = {
  ...(globalThis as Record<string, unknown>).SpreadsheetApp as object,
  getActiveSpreadsheet: () => ({
    ...((globalThis as Record<string, unknown>).SpreadsheetApp as { getActiveSpreadsheet: () => unknown }).getActiveSpreadsheet(),
    getOwner: () => ({ getEmail: () => 'OWNER@X.COM' }),  // or null for T17
  }),
};
```

If extending `test-helpers.ts` is cleaner (single one-line addition to the mock factory), DO that instead and document the diff in the SUMMARY. PREFERRED: minimal local override per test, no test-helpers.ts edit.

For the **log capture**, use `vi.spyOn(logModule, 'log')` style:

```typescript
import * as logModule from '../lib/log';
// ...
const logSpy = vi.spyOn(logModule, 'log').mockImplementation(() => {});
// ... act ...
expect(logSpy).toHaveBeenCalledWith('warn', expect.stringMatching(/getAdminList|bootstrapGuildAdmins|appendAdminLogEntry/), expect.objectContaining({ malformedExistingList: true }));
logSpy.mockRestore();
```

(Import `vi` from `'vitest'` alongside `describe/it/expect/beforeEach/afterEach`.)

For **lock-busy tests (T10, T19)**, use the existing `state.lockTryLockReturn = false` switch from the eviction-sidebar test file lines 212-213. Confirm via grep that `test-helpers.ts` exports this surface; if not, look for the actual variable name and use it.

For each test, follow the assertion shape from `showEvictionSidebar.test.ts:168-188`:
```typescript
const meta = state.sheets.get('_meta')!;
const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
expect(adminsRow[1]).toBe('["alice@x.com","bob@x.com"]');
const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
expect(list).toHaveLength(1);
expect(list[0]).toMatchObject({ action: 'add', email: 'alice@x.com', initiated_by: 'bob@x.com' });
expect(String(list[0].at)).toMatch(/^\d{4}-\d{2}-\d{2}T/);
```

After the file is complete and saved, RUN the suite once to confirm RED:
```bash
cd apps-script && npm test -- admin.test
```
Every test should fail with "Cannot find module '../lib/admin'" — that is the correct TDD RED state. Do NOT proceed to Task 2 until you've observed RED.
  </action>
  <verify>
    <automated>
      cd apps-script && npm test -- admin.test 2>&1 | tee /tmp/admin-test-red.log; grep -c "Cannot find module.*lib/admin" /tmp/admin-test-red.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Test-Path apps-script/src/__tests__/admin.test.ts` returns True.
    - `(Get-Content apps-script/src/__tests__/admin.test.ts | Where-Object { $_ -match "it\\(" }).Count` is >= 20 (T1–T20 plus permitted helper scenarios).
    - `Select-String -Path apps-script/src/__tests__/admin.test.ts -Pattern "requireAdminOrThrow_emptyEmail_throwsNotAuthorized" -SimpleMatch` matches >= 1 line.
    - `Select-String -Path apps-script/src/__tests__/admin.test.ts -Pattern "owner_floor_protected" -SimpleMatch` matches >= 1 line (T12 owner-floor lockout test).
    - `Select-String -Path apps-script/src/__tests__/admin.test.ts -Pattern "bootstrapGuildAdmins_ownerNull_writesFailedLogAndReturns" -SimpleMatch` matches >= 1 line.
    - `Select-String -Path apps-script/src/__tests__/admin.test.ts -Pattern "from '\\.\\./lib/admin'" -SimpleMatch` matches >= 1 line.
    - `cd apps-script; npm test -- admin.test 2>&1` exits non-zero with module-not-found errors (RED state confirmed). At least 1 occurrence of "Cannot find module" referencing `lib/admin` in the output.
    - The eviction sidebar tests still pass under `npm test` — adding a new test file does not break the existing suite.
  </acceptance_criteria>
  <done>
    Test file exists with 20 named tests covering verification_hooks 1, 2, 3, 4 at the policy level (hook 5 = grep gate, exercised in Task 2 acceptance). The suite is RED (all tests fail with module-not-found). Task 2 will GREEN it without touching this file.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Implement apps-script/src/lib/admin.ts to GREEN the suite</name>
  <files>apps-script/src/lib/admin.ts</files>
  <read_first>
    - apps-script/src/triggers/showEvictionSidebar.ts:113-187 (the canonical lock + envelope + audit-log-append shape; D-05 says clone this verbatim into add/removeAdmin)
    - apps-script/src/triggers/showEvictionSidebar.ts:146-153 (the `initiated_by` `Session.getEffectiveUser` fallback pattern; D-06 says soft-fallback to 'unknown' for audit-log only)
    - apps-script/src/lib/migrations.ts:80-107 (the LockService.tryLock(30000) envelope idiom; same pattern, different policy — your lock errors throw `addAdmin: lock_busy` / `removeAdmin: lock_busy` per 07-PATTERNS.md Pattern 1)
    - apps-script/src/lib/sheet-helpers.ts (readMetaRows + writeMetaRow signatures; admin.ts uses ONLY these — no raw getRange)
    - apps-script/src/lib/log.ts (log signature: `log(level, op, fields)` — op = function name)
    - apps-script/src/__tests__/admin.test.ts (the suite you just wrote — that's your spec)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md §Shared Patterns 1-7 (the LockService envelope, malformed-JSON-tolerant read, dual-policy caller identity, structured logging — all assigned to this module)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md §decisions D-01, D-02, D-05, D-06 (the four locked decisions this file implements)
  </read_first>
  <behavior>
    - Every test T1–T20 from Task 1 passes (GREEN). Run `npm test -- admin.test` after writing the file.
    - The 9 exported functions match the `<interfaces>` contract in the plan frontmatter byte-for-byte (Plans 02 and 03 are written against these signatures).
    - Every multi-step `_meta` write is inside a `LockService.getDocumentLock().tryLock(30000)` envelope. EXCEPTION: `bootstrapGuildAdmins` returns `{ reason: 'lock_busy' }` instead of throwing (D-01 onOpen-must-not-throw).
    - Every public function emits at least one structured `log(level, op, fields)` call with `op` = function name. Examples: `log('info', 'addAdmin', { email, callerEmail, alreadyExists: false })`, `log('warn', 'bootstrapGuildAdmins', { skipped: 'lock_busy' })`, `log('warn', 'requireAdminOrThrow', { notAuthorized: true, callerEmail })`.
    - `normalizeEmail` is the SINGLE NORMALIZATION POINT: every comparison, every read post-parse, every write goes through it. Lower(trim(s ?? '')).
    - `getAdminList` defensively parses `guild_admins`: malformed JSON → `{ admins: [], floor: '' }` and a `warn` log. Floor is read from a separate `_meta.workbook_owner_floor` row as a plain lowercased string (no JSON parse).
    - `addAdmin` validates `email` (non-empty after normalize + contains `@`); rejects with `Error('invalid_email')` (or similar — match T9's regex). Sorts the list ASCII-asc on write (deterministic for git-diff audits via clasp pull). Calls `appendAdminLogEntry` inside the lock.
    - `removeAdmin` reads floor inside the lock, splices the target out, calls `appendAdminLogEntry`. Owner-floor check is `target === floor && callerEmail !== floor` → throw `'owner_floor_protected'` BEFORE any write.
    - `bootstrapGuildAdmins`:
      - If lock busy → `log('warn', 'bootstrapGuildAdmins', { skipped: 'lock_busy' })` + return `{ bootstrapped: false, reason: 'lock_busy' }`. NO throw. (D-01.)
      - Inside lock: read `guild_admins`; if it parses to a non-empty array, return `{ bootstrapped: false, reason: 'already_initialized' }`. NO writes.
      - Otherwise: determine seed. If `opts?.seedEmail` provided, use it; else `SpreadsheetApp.getActiveSpreadsheet().getOwner()?.getEmail()`. If neither yields a non-empty email, log warn + write a `bootstrap_failed` admin_log entry + return `{ bootstrapped: false, reason: 'owner_null' }`.
      - Happy path: write `guild_admins=JSON.stringify([seed])` + `workbook_owner_floor=seed` + admin_log bootstrap entry; return `{ bootstrapped: true, seedEmail: seed }`.
    - `bootstrapGuildAdminsManual`: reads `Session.getEffectiveUser().getEmail()`. If empty → `getUi().alert('Initialize Admin Allowlist', 'Could not determine your email; please ensure you are signed in.', ButtonSet.OK)` + return. If non-empty → show OK_CANCEL confirmation modal with the verbatim copy from 07-UI-SPEC.md §Bootstrap confirmation modal. On OK → call `bootstrapGuildAdmins({ seedEmail, initiatedBy: 'manual_fallback' })`. On success → `SpreadsheetApp.getActiveSpreadsheet().toast('Admin allowlist initialized with ' + seed + '.')`. If result is `already_initialized` → `toast('Admin allowlist already initialized.')`.
    - `appendAdminLogEntry`: defensive parse of `_meta.admin_log` (malformed → `[]` + warn log), push entry, `writeMetaRow('_meta', 'admin_log', JSON.stringify(list))`.
    - schema_version is NEVER read or written by admin.ts. (Grep gate in acceptance.)
  </behavior>
  <action>
Implement `apps-script/src/lib/admin.ts` from scratch. Target ~220 lines. Structure:

```typescript
// admin.ts — Phase 7 plan 07-01.
//
// Central admin-policy module: every primitive for "who can do destructive
// things" lives here. Single source of truth so the eviction sidebar
// (Plan 03) and the admin-mgmt sidebar (Plan 02) cannot drift into
// re-implementing the admin check with a subtle bug.
//
// Storage shape (all three _meta rows are extend-only additions; no
// schema_version bump per STATE.md D-02):
//   _meta.guild_admins         — JSON.stringify(string[]) lowercased+sorted
//   _meta.workbook_owner_floor — single email string, lowercased
//   _meta.admin_log            — JSON.stringify(AdminLogEntry[]), append-only
//
// Dual-policy caller identity (D-06):
//   - Authorization (requireAdminOrThrow): empty → fail-closed throw
//   - Audit-log initiated_by: empty → 'unknown' soft fallback

import { log } from './log';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from './sheet-helpers';

// --- Types --------------------------------------------------------------

export interface AdminLogEntry {
  at: string;
  action: 'add' | 'remove' | 'bootstrap' | 'bootstrap_failed';
  email: string;
  initiated_by: string;
  reason?: string;
}

// --- Normalization ------------------------------------------------------

export function normalizeEmail(s: string | null | undefined): string {
  return String(s ?? '').toLowerCase().trim();
}

// --- Read primitives ----------------------------------------------------

export function getAdminList(): { admins: string[]; floor: string } {
  const meta = readMetaRows('_meta');
  let admins: string[] = [];
  const adminsRow = meta.find((r) => r.key === 'guild_admins');
  if (adminsRow && adminsRow.value) {
    try {
      const parsed = JSON.parse(adminsRow.value);
      if (Array.isArray(parsed)) {
        admins = parsed.map((s) => normalizeEmail(String(s))).filter(Boolean).sort();
      }
    } catch (_e) {
      log('warn', 'getAdminList', { malformedExistingList: true });
    }
  }
  const floorRow = meta.find((r) => r.key === 'workbook_owner_floor');
  const floor = floorRow ? normalizeEmail(String(floorRow.value)) : '';
  return { admins, floor };
}

export function isAdmin(email: string | null | undefined): boolean {
  const normalized = normalizeEmail(email);
  if (!normalized) return false;
  const { admins } = getAdminList();
  return admins.indexOf(normalized) !== -1;
}

export function requireAdminOrThrow(email: string | null | undefined): void {
  const normalized = normalizeEmail(email);
  if (!normalized || !isAdmin(normalized)) {
    log('warn', 'requireAdminOrThrow', { notAuthorized: true, callerEmail: normalized });
    throw new Error('not_authorized');
  }
}

// --- Internal helpers ---------------------------------------------------

function resolveInitiatedBy(): string {
  try {
    const effective = Session.getEffectiveUser().getEmail();
    if (effective) return normalizeEmail(effective);
  } catch (_e) {
    // sandbox quirk — fall through
  }
  return 'unknown';
}

export function appendAdminLogEntry(entry: AdminLogEntry): void {
  const meta = readMetaRows('_meta');
  const row = meta.find((r) => r.key === 'admin_log');
  let list: AdminLogEntry[] = [];
  if (row && row.value) {
    try {
      const parsed = JSON.parse(row.value);
      if (Array.isArray(parsed)) list = parsed as AdminLogEntry[];
    } catch (_e) {
      log('warn', 'appendAdminLogEntry', { malformedExistingLog: true });
    }
  }
  list.push(entry);
  writeMetaRow('_meta', 'admin_log', JSON.stringify(list));
}

// --- Mutating operations (lock-wrapped) --------------------------------

export function addAdmin(
  email: string,
  callerEmail: string,
): { added: boolean; alreadyExists?: boolean } {
  requireAdminOrThrow(callerEmail);
  const target = normalizeEmail(email);
  if (!target || target.indexOf('@') === -1) {
    throw new Error('invalid_email');
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    throw new Error('addAdmin: lock_busy');
  }
  try {
    const { admins } = getAdminList();
    if (admins.indexOf(target) !== -1) {
      log('info', 'addAdmin', { email: target, callerEmail, alreadyExists: true });
      return { added: false, alreadyExists: true };
    }
    const next = admins.concat([target]).sort();
    writeMetaRow('_meta', 'guild_admins', JSON.stringify(next));
    appendAdminLogEntry({
      at: new Date().toISOString(),
      action: 'add',
      email: target,
      initiated_by: callerEmail || resolveInitiatedBy(),
    });
    log('info', 'addAdmin', { email: target, callerEmail, added: true });
    return { added: true };
  } finally {
    lock.releaseLock();
  }
}

export function removeAdmin(
  email: string,
  callerEmail: string,
): { removed: boolean; notFound?: boolean } {
  requireAdminOrThrow(callerEmail);
  const target = normalizeEmail(email);
  if (!target) {
    throw new Error('invalid_email');
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    throw new Error('removeAdmin: lock_busy');
  }
  try {
    const { admins, floor } = getAdminList();
    const normalizedCaller = normalizeEmail(callerEmail);
    if (target === floor && normalizedCaller !== floor) {
      log('warn', 'removeAdmin', { email: target, callerEmail: normalizedCaller, blockedBy: 'owner_floor_protected' });
      throw new Error('owner_floor_protected');
    }
    const idx = admins.indexOf(target);
    if (idx === -1) {
      log('info', 'removeAdmin', { email: target, callerEmail: normalizedCaller, notFound: true });
      return { removed: false, notFound: true };
    }
    const next = admins.slice(0, idx).concat(admins.slice(idx + 1));
    writeMetaRow('_meta', 'guild_admins', JSON.stringify(next));
    // NOTE: workbook_owner_floor row is INTENTIONALLY not updated on
    // self-removal of floor (D-04). The floor row is the "who is protected
    // from non-self removal" pointer, not the "who is currently an admin"
    // pointer. Orphan-pointer state is documented as intentional.
    appendAdminLogEntry({
      at: new Date().toISOString(),
      action: 'remove',
      email: target,
      initiated_by: normalizedCaller || resolveInitiatedBy(),
    });
    log('info', 'removeAdmin', { email: target, callerEmail: normalizedCaller, removed: true });
    return { removed: true };
  } finally {
    lock.releaseLock();
  }
}

// --- Bootstrap ----------------------------------------------------------

export function bootstrapGuildAdmins(
  opts?: { seedEmail?: string; initiatedBy?: string },
): { bootstrapped: boolean; seedEmail?: string; reason?: 'already_initialized' | 'owner_null' | 'lock_busy' | 'utf16_failed' } {
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    log('warn', 'bootstrapGuildAdmins', { skipped: 'lock_busy' });
    return { bootstrapped: false, reason: 'lock_busy' };
  }
  try {
    const { admins } = getAdminList();
    if (admins.length > 0) {
      return { bootstrapped: false, reason: 'already_initialized' };
    }

    let seed = normalizeEmail(opts?.seedEmail ?? '');
    const initiatedBy = opts?.initiatedBy ?? 'onOpen';

    if (!seed) {
      try {
        const owner = getActiveSpreadsheet().getOwner();
        const ownerEmail = owner ? owner.getEmail() : '';
        seed = normalizeEmail(ownerEmail);
      } catch (_e) { /* getOwner threw — fall through */ }
    }

    if (!seed) {
      log('warn', 'bootstrapGuildAdmins', { reason: 'owner_null' });
      appendAdminLogEntry({
        at: new Date().toISOString(),
        action: 'bootstrap_failed',
        email: '',
        initiated_by: initiatedBy,
        reason: 'owner_null',
      });
      return { bootstrapped: false, reason: 'owner_null' };
    }

    writeMetaRow('_meta', 'guild_admins', JSON.stringify([seed]));
    writeMetaRow('_meta', 'workbook_owner_floor', seed);
    appendAdminLogEntry({
      at: new Date().toISOString(),
      action: 'bootstrap',
      email: seed,
      initiated_by: initiatedBy,
    });
    log('info', 'bootstrapGuildAdmins', { seedEmail: seed, initiatedBy, bootstrapped: true });
    return { bootstrapped: true, seedEmail: seed };
  } finally {
    lock.releaseLock();
  }
}

export function bootstrapGuildAdminsManual(): void {
  const ui = SpreadsheetApp.getUi();
  let seed = '';
  try {
    seed = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* sandbox quirk */ }
  if (!seed) {
    ui.alert(
      'Initialize Admin Allowlist',
      'Could not determine your email. Please ensure you are signed in and try again.',
      ui.ButtonSet.OK,
    );
    return;
  }
  const response = ui.alert(
    'Initialize Admin Allowlist',
    'About to add ' + seed + ' as the first admin and owner-floor. This is the seed identity that bootstraps the allowlist; the owner-floor protection means no one else will be able to remove this email. Continue?',
    ui.ButtonSet.OK_CANCEL,
  );
  if (response !== ui.Button.OK) {
    log('info', 'bootstrapGuildAdminsManual', { cancelled: true });
    return;
  }
  const result = bootstrapGuildAdmins({ seedEmail: seed, initiatedBy: 'manual_fallback' });
  if (result.bootstrapped) {
    getActiveSpreadsheet().toast('Admin allowlist initialized with ' + seed + '.');
  } else if (result.reason === 'already_initialized') {
    getActiveSpreadsheet().toast('Admin allowlist already initialized.');
  } else {
    getActiveSpreadsheet().toast('Admin allowlist bootstrap failed: ' + (result.reason ?? 'unknown') + '.');
  }
}
```

After writing the file, RUN the test suite to GREEN:
```bash
cd apps-script && npm test -- admin.test
```
All 20 tests must pass. If any fail, debug the assertion vs. the implementation; do NOT relax the test. If a test reveals a behavioral gap, fix admin.ts.

If tests pass, also run the full apps-script suite to verify no regression in the existing 297+ test count:
```bash
cd apps-script && npm test
```
The existing eviction-sidebar tests will still pass because admin.ts has not been wired into the eviction sidebar yet (that's Plan 03).
  </action>
  <verify>
    <automated>
      cd apps-script && npm test -- admin.test 2>&1 | tee /tmp/admin-test-green.log; grep -c "✓\|PASS" /tmp/admin-test-green.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Test-Path apps-script/src/lib/admin.ts` returns True.
    - `(Get-Content apps-script/src/lib/admin.ts).Count` is >= 200 (the module is non-trivial).
    - `Select-String -Path apps-script/src/lib/admin.ts -Pattern "^export function (normalizeEmail|getAdminList|isAdmin|requireAdminOrThrow|addAdmin|removeAdmin|bootstrapGuildAdmins|bootstrapGuildAdminsManual|appendAdminLogEntry)\b"` matches exactly 9 lines (one per public export).
    - `Select-String -Path apps-script/src/lib/admin.ts -Pattern "LockService\.getDocumentLock\(\)\.tryLock\(30000\)"` matches >= 3 lines (addAdmin, removeAdmin, bootstrapGuildAdmins).
    - `Select-String -Path apps-script/src/lib/admin.ts -Pattern "throw new Error\('not_authorized'\)"` matches >= 1 line (requireAdminOrThrow fail-closed).
    - `Select-String -Path apps-script/src/lib/admin.ts -Pattern "throw new Error\('owner_floor_protected'\)"` matches >= 1 line (D-04 floor enforcement).
    - `Select-String -Path apps-script/src/lib/admin.ts -Pattern "schema_version"` matches ZERO lines (the policy module must not touch schema_version — verification hook 5 grep gate).
    - `Select-String -Path apps-script/src/lib/admin.ts -Pattern "from '\\./sheet-helpers'"` matches exactly 1 line (no raw getRange — readMetaRows/writeMetaRow only).
    - `cd apps-script; npm test -- admin.test 2>&1` exits 0; the GREEN log contains >= 20 passing-test markers (vitest emits `✓` per test).
    - `cd apps-script; npm test 2>&1` exits 0 (full suite still green — no regression).
    - `cd apps-script; npm run typecheck 2>&1` exits 0 (or whatever the typecheck script is; if absent, `cd apps-script; npx tsc --noEmit` exits 0).
    - `Select-String -Path apps-script/src/lib/migrations.ts -Pattern "schema_version'" -SimpleMatch` still matches its existing line count (verification hook 5: schema_version stays at 3; admin.ts didn't bump it).
    - `Select-String -Path internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion\s*=\s*3"` matches exactly 1 line (verification hook 5: watcher max schema version stays at 3).
  </acceptance_criteria>
  <done>
    `lib/admin.ts` exists with 9 public exports matching the `<interfaces>` contract; all 20 unit tests in `admin.test.ts` PASS (GREEN); the existing apps-script test suite still passes (no regression); typecheck passes; schema_version is untouched anywhere in admin.ts or migrations.ts. Plans 02 and 03 are unblocked: they can import `isAdmin`, `requireAdminOrThrow`, `addAdmin`, `removeAdmin`, `getAdminList`, `bootstrapGuildAdmins`, `bootstrapGuildAdminsManual` directly from `'../lib/admin'`.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `Session.getEffectiveUser().getEmail()` → authorization decision | The boundary between "untrusted/maybe-sandboxed string" and "policy decision (is this caller an admin?)". Empty string here MUST mean "not admin," never "skip the check." |
| `_meta.guild_admins` cell value → policy state | Human-editable cell. Malformed JSON, mixed casing, or hand-typed garbage MUST fail-closed (empty admin list) rather than crash policy reads. |
| Concurrent admin actions (two admins both adding/removing simultaneously) | LockService.getDocumentLock() is the only thing preventing lost-write races on `_meta.guild_admins` and `_meta.admin_log`. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-07-01-01 | Spoofing | `Session.getEffectiveUser().getEmail()` returns empty under sandbox; an attacker-controlled context could exploit "empty == skip check" if implemented permissively | mitigate | `requireAdminOrThrow` normalizes to empty string and FAIL-CLOSES (`if (!normalized) throw 'not_authorized'`). Tested by T1. Audit-log soft-fallback to `'unknown'` is via a SEPARATE code path (`resolveInitiatedBy`); the two policies never share a code path. (D-06.) |
| T-07-01-02 | Tampering | `_meta.guild_admins` cell hand-edited to malformed JSON by a guildie poking around | mitigate | `getAdminList` defensive `try { JSON.parse } catch { warn-log }` → returns `{ admins: [], floor: '' }`. Empty admin list = nobody is admin = fail-closed. Tested by T6. |
| T-07-01-03 | Tampering | Admin allowlist hand-edited to insert an unauthorized email | accept | `_meta` tab is hidden by default (per project convention); editing requires unhiding system tabs. Any admin can already use the admin-mgmt sidebar to do the same write through the audited path. The audit log records the sidebar-mediated writes but cannot detect raw-cell edits — capture as backlog (admin-log gap detection) if it becomes a concern. Threat is bounded by trust in the ~12-person guildmate group per PROJECT.md core scope. |
| T-07-01-04 | Repudiation | Admin removes another admin without leaving a trace | mitigate | Every `addAdmin` / `removeAdmin` writes a `_meta.admin_log` entry inside the lock with `{ at, action, email, initiated_by }`. Tested by T7, T11. |
| T-07-01-05 | Information Disclosure | `_meta.admin_log` reveals admin actions to anyone who unhides system tabs | accept | Admin actions are intentionally auditable; the log IS the disclosure surface. No PII beyond email addresses (which are already shared in `_char_owner.owner_email`). |
| T-07-01-06 | Denial of Service | Lock-busy (two admins acting simultaneously, or a hung migration holding the doc lock) blocks admin actions | mitigate | `addAdmin` / `removeAdmin` throw `Error('<func>: lock_busy')` after 30s; sidebar surfaces a status message and the user retries. `bootstrapGuildAdmins` returns `{ reason: 'lock_busy' }` silently (D-01 onOpen must not throw — silent retry on next workbook open). Tested by T10, T19. |
| T-07-01-07 | Denial of Service | Owner-floor lockout: a malicious admin removes everyone including themselves | partial mitigate | Owner-floor protection prevents the WORKBOOK owner from being removed by anyone else (D-04). However an admin can still remove every NON-floor admin including themselves, leaving only the floor. The floor user always retains an unimpeachable seat by design. Self-removal-of-self is permitted (admin steps down voluntarily). |
| T-07-01-08 | Elevation of Privilege | A non-admin invokes `addAdmin` directly via `google.script.run` from a stale sidebar / devtools | mitigate | `addAdmin` and `removeAdmin` call `requireAdminOrThrow(callerEmail)` as their FIRST statement. The check uses server-side `Session.getEffectiveUser` — not a client-supplied parameter — so the sidebar wrapper in Plan 02 cannot spoof the caller. Tested by T1, T2, T9 indirectly (callerEmail must be admin). |
| T-07-01-09 | Tampering | `LockService` is bypassed by some future refactor that forgets the envelope | mitigate | The lock envelope is in this single file (admin.ts). Tests T10 + T19 fail if the envelope is removed (state.lockTryLockReturn = false → no write expected). Acceptance criteria grep gate enforces >= 3 lock-tryLock matches. |
| T-07-01-10 | Information Disclosure | Logging `callerEmail` in structured logs leaks PII to anyone with Apps Script log access | accept | Per CLAUDE.md "Structured logging both Go side and Apps Script side ... keeps logs greppable" — `callerEmail` is a required field for incident-response traceability. Apps Script logs are scoped to script-owner viewers (Stackdriver / Logger.log) — same trust boundary as the workbook itself. |

ASVS L1: zero `high` severity threats — all 10 are mitigated in-code or accepted with documented rationale. The empty-email fail-closed boundary (T-07-01-01) and the lock envelope (T-07-01-09) are the two load-bearing mitigations; both are exercised by the test suite.
</threat_model>

<verification>
- `cd apps-script; npm test -- admin.test 2>&1` exits 0 with all 20 named tests passing.
- `cd apps-script; npm test 2>&1` exits 0 (full suite green; the existing eviction-sidebar and migration tests are unaffected).
- `Select-String -Path apps-script/src/lib/admin.ts -Pattern "schema_version" -SimpleMatch` matches ZERO lines.
- `Select-String -Path apps-script/src/lib/migrations.ts -Pattern "writeMetaRow\('_meta', 'schema_version', '3'\)" -SimpleMatch` still matches its existing 1 line (no migration bump introduced by this plan).
- `Select-String -Path internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion" -SimpleMatch` shows the constant is still `3` (no watcher rebuild needed — verification hook 5).
- The 9 public exports from admin.ts are importable by sibling modules (Plans 02 and 03 will import via `from '../lib/admin'`). Confirmed by typecheck.
</verification>

<success_criteria>
- New `apps-script/src/lib/admin.ts` exists with 9 public exports matching the `<interfaces>` contract byte-for-byte.
- New `apps-script/src/__tests__/admin.test.ts` exists with the 20 named tests T1–T20 from 07-PATTERNS.md.
- `npm test` is GREEN for the new suite AND the existing suite (no regression).
- Phase 7 verification hook 1 partial coverage: `bootstrapGuildAdmins` empty-list write + idempotent re-call covered at the policy level (T15, T16). Integration verification rides in Plan 03 via `onOpen` smoke.
- Phase 7 verification hook 2 partial coverage: `requireAdminOrThrow('nonadmin@example.com')` throws `'not_authorized'` (T2). Integration verification rides in Plan 03 via the eviction-sidebar opener guard.
- Phase 7 verification hook 3 partial coverage: `addAdmin('newadmin@example.com', 'existingadmin@example.com')` returns `{ added: true }` then `isAdmin('newadmin@example.com')` returns true (T7, T5). Integration verification rides in Plan 02 via the admin-mgmt sidebar smoke.
- Phase 7 verification hook 4 partial coverage: non-floor → non-floor remove succeeds (T11); floor → non-floor caller throws `owner_floor_protected` (T12); floor → self succeeds (T13). Integration verification rides in Plan 02.
- Phase 7 verification hook 5 grep gate: `admin.ts` does not reference `schema_version`; `migrations.ts` schema_version write is unchanged; `WatcherMaxSchemaVersion` is unchanged at 3 in the Go watcher.
- ADMIN-01 partial (policy module exists; bootstrap call-site lands in Plan 03).
- ADMIN-03 partial (add/remove/floor-protection policy exists; the management UX surface lands in Plan 02).
- ADMIN-02 not touched here (the eviction guard wiring lands in Plan 03).
</success_criteria>

<output>
After completion, create `.planning/phases/07-admin-allowlist-eviction-enforcement/07-01-SUMMARY.md` capturing:
- Files created (2 absolute paths: admin.ts, admin.test.ts).
- Test results (20/20 named tests pass; full suite still green at N+20 tests).
- The exact 9 public-export signatures from admin.ts (so Plans 02 and 03 can read these without re-deriving from source).
- Any test-helpers.ts extensions made (if any) for `getOwner()` or log-spy support.
- Confirmation that `schema_version` and `WatcherMaxSchemaVersion` are untouched (verification hook 5).
- Decision log: any deviations from the `<interfaces>` contract (NONE expected; if any, surface them so Plans 02/03 can react).
</output>
