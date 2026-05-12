---
phase: 07-admin-allowlist-eviction-enforcement
plan: 01
subsystem: apps-script
tags: [apps-script, policy, lib, lockservice, unit-tests, admin, tdd]
dependency_graph:
  requires:
    - apps-script/src/lib/log.ts (log helper)
    - apps-script/src/lib/sheet-helpers.ts (readMetaRows, writeMetaRow, getActiveSpreadsheet)
  provides:
    - apps-script/src/lib/admin.ts (9 public exports — policy primitives + bootstrap + audit-log helper)
  affects: []
tech_stack:
  added: []
  patterns: [LockService.tryLock(30000) envelope, malformed-JSON-tolerant read, dual-policy caller identity, structured logging]
key_files:
  created:
    - apps-script/src/lib/admin.ts
    - apps-script/src/__tests__/admin.test.ts
  modified: []
decisions:
  - "Cloned the eviction-sidebar lock+audit-log envelope verbatim for addAdmin/removeAdmin (D-05 single source of truth)"
  - "bootstrapGuildAdmins is the documented exception that returns {reason:'lock_busy'} silently instead of throwing — D-01 onOpen mustn't throw"
  - "Owner-floor protection check happens BEFORE any write in removeAdmin (T12)"
  - "normalizeEmail is the single normalization point applied at read, write, and compare (CONTEXT.md §specifics)"
  - "Per-test getOwner() override (rather than extending test-helpers.ts) keeps the SpreadsheetApp mock minimal — only T15/T17/T18/T19 need it"
metrics:
  duration_seconds: 328
  duration_human: "~5.5 minutes"
  completed_date: "2026-05-12"
  tasks_completed: 2
  tests_added: 20
  full_suite_total: 317
---

# Phase 7 Plan 01: Admin Policy Module Summary

Central admin-policy module `apps-script/src/lib/admin.ts` shipped with 9 public exports + 20-scenario vitest unit suite (T1–T20 from 07-PATTERNS.md), all GREEN. Plans 02 and 03 are unblocked.

## Outcome

- 2 files created (admin.ts: 357 lines; admin.test.ts: 462 lines).
- 0 files modified — Task 1 + Task 2 are pure additions; no existing source touched.
- 2 commits, 1 per task, no deviations.
- Apps-script test suite went from 297/297 → 317/317 GREEN (+20 admin.ts tests).
- Typecheck (`npm run typecheck`) clean.
- Full suite (`npm test`) clean: 29 test files, 317 tests pass, 0 fail, ~19s duration.

### Commits

| Hash | Message | Files |
|------|---------|-------|
| `dfc3533` | test(07-01): add failing vitest suite for admin.ts (T1-T20) | apps-script/src/__tests__/admin.test.ts |
| `8780222` | feat(07-01): implement lib/admin.ts policy module (GREEN: 20/20 tests pass) | apps-script/src/lib/admin.ts |

## Files Created (absolute paths)

- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\lib\admin.ts`
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\__tests__\admin.test.ts`

## Public-Export Signatures (verbatim — Plans 02 and 03 import against these)

```typescript
// apps-script/src/lib/admin.ts

export interface AdminLogEntry {
  at: string;          // ISO8601
  action: 'add' | 'remove' | 'bootstrap' | 'bootstrap_failed';
  email: string;
  initiated_by: string;
  reason?: string;     // optional — only set for 'bootstrap_failed' entries
}

export type BootstrapReason =
  | 'already_initialized'
  | 'owner_null'
  | 'lock_busy'
  | 'utf16_failed';

export function normalizeEmail(s: string | null | undefined): string;

export function getAdminList(): { admins: string[]; floor: string };

export function isAdmin(email: string | null | undefined): boolean;

export function requireAdminOrThrow(email: string | null | undefined): void;

export function appendAdminLogEntry(entry: AdminLogEntry): void;

export function addAdmin(
  email: string,
  callerEmail: string,
): { added: boolean; alreadyExists?: boolean };

export function removeAdmin(
  email: string,
  callerEmail: string,
): { removed: boolean; notFound?: boolean };

export function bootstrapGuildAdmins(
  opts?: { seedEmail?: string; initiatedBy?: string },
): { bootstrapped: boolean; seedEmail?: string; reason?: BootstrapReason };

export function bootstrapGuildAdminsManual(): void;
```

These match the `<interfaces>` contract in 07-01-PLAN.md byte-for-byte. ZERO deviations from the planned signatures. Plans 02 and 03 should:

- **Plan 02 (admin-mgmt sidebar):** import `getAdminList`, `addAdmin`, `removeAdmin` (server-side callbacks); export `bootstrapGuildAdminsManual` as a top-level global via Code.ts re-export footer.
- **Plan 03 (eviction guards + onOpen + smoke):** import `isAdmin`, `requireAdminOrThrow` for the eviction-sidebar opener + 3 callback guards; import `bootstrapGuildAdmins` for the onOpen lazy bootstrap call.

## Test Coverage (T1–T20)

All 20 named tests pass on first GREEN run. Coverage map:

| Test | Function | Scenario | Verification Hook |
|------|----------|----------|-------------------|
| T1 | requireAdminOrThrow | empty/null email → throws not_authorized (D-06 fail-closed) | hook 2 |
| T2 | requireAdminOrThrow | email not in list → throws not_authorized | hook 2 |
| T3 | requireAdminOrThrow | case-mismatched input → succeeds (normalized compare) | hook 2 |
| T4 | isAdmin | empty list → false | hook 2 |
| T5 | isAdmin | case-insensitive match | hook 3 |
| T6 | getAdminList | malformed JSON → {admins:[], floor:''} + warn log (D-05) | hook 2 |
| T7 | addAdmin | happy path: sorted+lowercased write + admin_log entry | hook 3 |
| T8 | addAdmin | idempotent: existing email → {added:false} no writes | hook 3 |
| T9 | addAdmin | invalid email (empty / no '@') → throws invalid_email | — |
| T10 | addAdmin | lock-busy → throws addAdmin: lock_busy + no writes | — |
| T11 | removeAdmin | non-floor target by non-floor caller → succeeds | hook 4 |
| T12 | removeAdmin | floor target by non-floor caller → throws owner_floor_protected (D-04) | hook 4 |
| T13 | removeAdmin | floor self-removal → succeeds; floor row UNCHANGED (orphan per D-04) | hook 4 |
| T14 | removeAdmin | not-in-list → idempotent {removed:false, notFound:true} | — |
| T15 | bootstrapGuildAdmins | empty _meta → writes seed + floor + bootstrap log | hook 1 |
| T16 | bootstrapGuildAdmins | already seeded → no-op {bootstrapped:false, reason:'already_initialized'} | hook 1 |
| T17 | bootstrapGuildAdmins | getOwner null → bootstrap_failed log entry, no guild_admins write | hook 1 |
| T18 | bootstrapGuildAdmins | manual opts.seedEmail overrides getOwner | hook 1 |
| T19 | bootstrapGuildAdmins | lock-busy → silent no-op (does NOT throw — D-01) | — |
| T20 | appendAdminLogEntry | malformed existing log → starts fresh + warn log | — |

Hooks 1–4 covered at the **policy** layer; integration verification rides in Plans 02 (hooks 3 + 4) and 03 (hooks 1 + 2). Hook 5 is the schema_version grep gate exercised below.

## Verification Hook 5 — Schema Untouched (PASS)

| Gate | Expected | Actual |
|------|----------|--------|
| `Select-String apps-script/src/lib/admin.ts -Pattern "schema_version"` | 0 matches | 0 matches ✓ |
| `apps-script/src/lib/migrations.ts` schema_version line count | 9 (unchanged from baseline) | 9 ✓ |
| `internal/sheet/client.go` `WatcherMaxSchemaVersion = 3` | 1 match | 1 match ✓ |

`_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. No watcher rebuild required for Phase 7 (consistent with STATE.md D-02 and Phase 7 ROADMAP success criterion 5).

## Acceptance Criteria — All PASS

- ✓ `apps-script/src/lib/admin.ts` exists; 357 lines (≥200 required).
- ✓ 9 `^export function` matches for the 9 expected names (exactly the contract surface).
- ✓ 3 `LockService.getDocumentLock()` envelopes (≥3 required: addAdmin, removeAdmin, bootstrapGuildAdmins).
- ✓ 1 `throw new Error('not_authorized')` (requireAdminOrThrow fail-closed).
- ✓ 1 `throw new Error('owner_floor_protected')` (D-04 floor enforcement).
- ✓ 0 `schema_version` references (verification hook 5 grep gate).
- ✓ 1 `from './sheet-helpers'` import (no raw getRange — readMetaRows/writeMetaRow only).
- ✓ `apps-script/src/__tests__/admin.test.ts` exists; 462 lines (≥320 required).
- ✓ 20 `it(` calls (≥20 required, exact T1–T20 names matched).
- ✓ `npm test -- admin.test`: 20/20 PASS (GREEN). Logged stdout shows the structured warn lines for T1, T2, T12, T17, T19 — log envelope verified.
- ✓ `npm test`: full suite 29 files / 317 tests PASS (was 297; +20 from admin.test.ts; existing eviction-sidebar tests unaffected because admin.ts is not yet wired in).
- ✓ `npm run typecheck`: clean (tsc --noEmit returns 0).

## Test-Helpers Extensions Made

**NONE.** The base `apps-script/src/__tests__/test-helpers.ts` was not edited. Per the plan's PREFERRED guidance, the per-test `installOwnerOverride(ownerEmail)` lives at the top of `admin.test.ts` and re-wraps the SpreadsheetApp mock for tests T15/T17/T18/T19 only. `resetMocks()` in `beforeEach` reinstalls the base mock so each test starts clean.

This keeps the test-helpers surface unchanged, isolating Phase 7's testing concerns to the Phase 7 test file. Plan 02's `adminMgmtSidebar.test.ts` may need to extend test-helpers to capture `SpreadsheetApp.getUi().alert(...)` calls — that decision is deferred to Plan 02 since the eviction-sidebar tests don't exercise alert() either.

## Decisions Made

1. **Cloned the eviction-sidebar lock+audit-log envelope verbatim.** Per D-05 (central policy module), addAdmin/removeAdmin use the same shape as commitEviction (`showEvictionSidebar.ts:122-186`): tryLock(30000) → try / readMetaRows / mutate / writeMetaRow / appendLog / finally releaseLock. Identical idiom prevents drift; future readers reach for the eviction-sidebar example and find the admin module already follows it.
2. **bootstrapGuildAdmins is the documented exception that returns `{reason:'lock_busy'}` silently.** D-01: onOpen must NOT throw or the menu breaks for everyone. Internal pattern is the same lock envelope, but the lock-busy branch logs a warn and returns the enum instead of throwing. Tested by T19.
3. **Owner-floor protection check fires BEFORE any write in removeAdmin.** Throws `owner_floor_protected` immediately on detect (T12). Self-removal of floor by floor user is permitted but does NOT update the workbook_owner_floor row — the orphan-pointer state is intentional (D-04 documented). T13 asserts the floor row is unchanged after self-removal.
4. **Per-test getOwner() override instead of extending test-helpers.** Only 4 tests (T15, T17, T18, T19) need to exercise `SpreadsheetApp.getActiveSpreadsheet().getOwner()`. A 7-line `installOwnerOverride(email|null)` helper at the top of admin.test.ts wraps the base mock per-test; `resetMocks()` in beforeEach reinstalls the base mock. Keeps test-helpers surface unchanged.
5. **`normalizeEmail` single normalization point.** Per CONTEXT.md §specifics: applied at read (inside getAdminList map+filter+sort), write (inside addAdmin sort, inside bootstrapGuildAdmins seed write), and compare (every isAdmin / requireAdminOrThrow / addAdmin / removeAdmin input). One helper, three call sites.

## Deviations from Plan

**None.** Plan executed exactly as written. The 9 public-export signatures match the `<interfaces>` contract byte-for-byte. The TDD discipline (RED commit → GREEN commit) was followed cleanly. No auth gates, no Rule 1/2/3 auto-fixes were triggered.

The plan's `<read_first>` referenced `makeSheet(state, name)` at one point in the test-helpers description, but the actual signature is `makeSheet(name, headers, dataRows)`; the test file uses `seedMeta(state, rows)` (which already wraps `state.sheets.set('_meta', makeSheet(...))`) and `state.sheets.set(name, makeSheet(...))` for non-_meta sheets, matching the existing eviction-sidebar test file conventions. This is not a deviation — just disambiguating the helper signatures from the plan prose.

## Authentication Gates

None. Plan executed in pure Vitest unit-test environment; no Apps Script bindings involved at this stage. The Apps Script integration (`clasp push` to dev workbook + interactive smoke) is owned by Plan 03.

## Threat Flags

None. The 10 threat-register entries from 07-01-PLAN.md are all mitigated in-code or accepted with documented rationale; no new security-relevant surface was introduced beyond what the plan and threat model already covered.

## Self-Check: PASSED

- ✓ `apps-script/src/lib/admin.ts` exists on disk
- ✓ `apps-script/src/__tests__/admin.test.ts` exists on disk
- ✓ Commit `dfc3533` exists in git log (test commit)
- ✓ Commit `8780222` exists in git log (impl commit)
- ✓ Full apps-script suite: 317/317 PASS verified post-impl
- ✓ Typecheck: clean
- ✓ Verification hook 5 grep gates: all 3 PASS
