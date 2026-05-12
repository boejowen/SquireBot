---
phase: 07-admin-allowlist-eviction-enforcement
plan: 03
type: execute
wave: 2
depends_on: [07-01]
files_modified:
  - apps-script/src/triggers/showEvictionSidebar.ts
  - apps-script/src/triggers/onOpen.ts
  - apps-script/src/__tests__/showEvictionSidebar.test.ts
autonomous: false
requirements: [ADMIN-01, ADMIN-02]
tags: [apps-script, eviction, onopen, menu, clasp-push, ship-gate]

must_haves:
  truths:
    - "`showEvictionSidebar` opener checks `isAdmin(callerEmail)` BEFORE building HTML; non-admin → `getUi().alert('Not authorized', 'Only guild officers can evict members. Contact a workbook admin if you think this is wrong.', ButtonSet.OK)` + return (no sidebar opens)."
    - "Each of `getEvictionEmails`, `previewEviction`, `commitEviction` calls `requireAdminOrThrow(callerEmail)` as its FIRST statement, BEFORE any existing validation. callerEmail comes from `Session.getEffectiveUser().getEmail()` server-side."
    - "`apps-script/src/triggers/onOpen.ts` lazily calls `bootstrapGuildAdmins()` from the TOP of `onOpen` wrapped in `try { ... } catch (err) { log('warn', 'onOpen.bootstrap_failed', { error: String(err) }) }` — onOpen never throws."
    - "`onOpen.ts` menu chain adds `.addItem('Manage Admins…', 'showAdminMgmtSidebar')` BETWEEN `Evict Guildie…` (existing line 22) and `Set Theme…` (existing line 23). Uses Unicode `…` (U+2026), not three ASCII dots."
    - "`onOpen.ts` menu chain adds `.addSeparator().addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')` AFTER the `Run Migration (v=2 legacy)` line."
    - "Existing `showEvictionSidebar.test.ts` tests continue to pass — they are extended so every test that calls eviction callbacks first seeds `_meta.guild_admins=[testCallerEmail]` so `requireAdminOrThrow` succeeds."
    - "All `npm test` suites are green: the 297+-test baseline, admin.test.ts (20 tests, Plan 01), adminMgmtSidebar.test.ts (5+ tests, Plan 02), and updated showEvictionSidebar.test.ts."
    - "`clasp push` is executed against the dev workbook and `apps-script/dist/Code.js` lands successfully (no script errors visible in Apps Script editor)."
    - "Dev-workbook smoke verifies all 5 verification hooks from 07-CONTEXT.md §verification_hooks: (1) `_meta.guild_admins` exists after first open + idempotent re-open; (2) non-admin sees alert + no eviction writes; (3) admin adds peer + peer's eviction sidebar opens; (4) owner-floor cannot be removed by non-floor admin; (5) `_meta.schema_version` is still `3` and `WatcherMaxSchemaVersion` is still `3` (grep-level)."
  artifacts:
    - path: apps-script/src/triggers/showEvictionSidebar.ts
      provides: "Eviction sidebar with admin guard inserted at opener + each callback (additive — existing 30-day-grace + lock-wrapped commit logic unchanged)"
      contains: "isAdmin(callerEmail)"
    - path: apps-script/src/triggers/onOpen.ts
      provides: "onOpen with lazy `bootstrapGuildAdmins()` call + 2 new menu items (Manage Admins…, Initialize Admin Allowlist (manual))"
      contains: "bootstrapGuildAdmins"
    - path: apps-script/src/__tests__/showEvictionSidebar.test.ts
      provides: "Updated eviction-sidebar tests with seeded admin list per test so requireAdminOrThrow passes for the admin caller mocks"
      contains: "guild_admins"
    - path: .planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SMOKE.md
      provides: "Smoke evidence: clasp push output, dev-workbook verification matrix mapping each of the 5 hooks to a PASS/notes line, grep verification of schema_version + WatcherMaxSchemaVersion"
  key_links:
    - from: apps-script/src/triggers/showEvictionSidebar.ts
      to: apps-script/src/lib/admin.ts
      via: "new named imports `isAdmin`, `requireAdminOrThrow` (additive to existing import block)"
      pattern: "from '\\.\\./lib/admin'"
    - from: apps-script/src/triggers/onOpen.ts
      to: apps-script/src/lib/admin.ts
      via: "new named import `bootstrapGuildAdmins`"
      pattern: "from '\\.\\./lib/admin'"
    - from: apps-script/src/triggers/onOpen.ts
      to: apps-script/src/lib/log.ts
      via: "new named import `log` (first imports introduced to this file)"
      pattern: "from '\\.\\./lib/log'"
---

<objective>
Wire `lib/admin.ts` (Plan 01) into the existing eviction sidebar and `onOpen`, then ship via `clasp push` + dev-workbook smoke. This is the second Wave-2 plan (parallel with Plan 02; both depend on Plan 01).

Scope:
1. **Eviction sidebar guard** (D-03) — opener admin-gates; each of the 3 callbacks calls `requireAdminOrThrow` first. The existing 30-day-grace, lock-wrapped commit, JSON-array audit log behavior is UNTOUCHED. This is purely additive.
2. **onOpen integration** (D-01 + D-04 menu) — lazy `bootstrapGuildAdmins()` call wrapped in try/catch at the top of `onOpen` (must never throw — would break the menu for everyone); two new menu items wired in.
3. **Test suite update** — every existing eviction-sidebar test that exercises a callback now needs `_meta.guild_admins` seeded with the test's caller email so `requireAdminOrThrow` succeeds. Sequence-load-bearing: tests get updated BEFORE the guard is added (otherwise the test suite goes red in the middle of the plan).
4. **Ship gate** — `clasp push` of the apps-script bundle to the dev workbook + a dev-workbook smoke that exercises all 5 verification hooks from 07-CONTEXT.md. Smoke result lands as `07-03-SMOKE.md`.

This plan closes ADMIN-01 (the bootstrap call-site lands here; the policy was created in Plan 01) and ADMIN-02 (the eviction sidebar refuses non-admin invokers). It DOES NOT touch `apps-script/src/Code.ts` — Plan 02 owns all Code.ts edits. Both plans must ship together via the same `clasp push` for the new menu items to find their function-name targets in the deployed bundle.

Output: 2 modified source files, 1 modified test file, 1 new smoke evidence file. The autonomous flag is `false` because the smoke task requires the user (workbook owner) to execute `clasp push` from their machine — clasp uses OAuth credentials in `~/.clasprc.json` that Claude cannot access (per the `apps-script-deploy.md` runbook).
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
@.planning/phases/07-admin-allowlist-eviction-enforcement/07-UI-SPEC.md
@.planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md
@.planning/phases/07-admin-allowlist-eviction-enforcement/07-01-admin-policy-module-PLAN.md
@.planning/phases/07-admin-allowlist-eviction-enforcement/07-02-admin-mgmt-sidebar-PLAN.md
@apps-script/src/triggers/showEvictionSidebar.ts
@apps-script/src/triggers/onOpen.ts
@apps-script/src/__tests__/showEvictionSidebar.test.ts
@docs/apps-script-deploy.md

<interfaces>
<!-- Imports added to the modified files. -->

```typescript
// apps-script/src/triggers/showEvictionSidebar.ts (additive)
import { isAdmin, requireAdminOrThrow, normalizeEmail } from '../lib/admin';
```

```typescript
// apps-script/src/triggers/onOpen.ts (NEW imports — file currently has none)
import { bootstrapGuildAdmins } from '../lib/admin';
import { log } from '../lib/log';
```
</interfaces>

<menu_diff>
<!-- Exact diff against onOpen.ts lines 7-27. Reproduced from 07-UI-SPEC.md §Menu Integration Spec for executor convenience. -->

Current shape (lines 7-27):
```typescript
export function onOpen(): void {
  SpreadsheetApp.getUi()
    .createMenu('SquireBot')
    .addItem('Install Triggers', 'installTriggers')
    .addSeparator()
    .addItem('Rebuild Views Now', 'buildView')
    .addItem('Refresh PigParse Now', 'refreshPigparse')
    .addItem('Refresh Wiki Items Now', 'refreshWikiItems')
    .addItem('Refresh Wiki Spells Now', 'refreshWikiSpells')
    .addItem('Refresh Wiki Gear Tier Now', 'refreshWikiGearTier')
    .addItem('Run Cell-Count Check Now', 'monitorCellCount')
    .addSeparator()
    .addItem('Set Character Info…', 'showCharInfoSidebar')
    .addItem('Set Bank Coin…', 'showBankCoinSidebar')
    .addItem('Search…', 'showSearchSidebar')
    .addItem('Evict Guildie…', 'showEvictionSidebar')
    .addItem('Set Theme…', 'showThemePickerModal')
    .addSeparator()
    .addItem('Run Migration (v=3)', 'migrateToV3')
    .addItem('Run Migration (v=2 legacy)', 'migrateToV2')
    .addToUi();
}
```

Target shape (5 line-additions, no deletions, no edits to existing items):
```typescript
export function onOpen(): void {
  try {
    bootstrapGuildAdmins();
  } catch (err) {
    log('warn', 'onOpen.bootstrap_failed', { error: String(err) });
  }
  SpreadsheetApp.getUi()
    .createMenu('SquireBot')
    .addItem('Install Triggers', 'installTriggers')
    .addSeparator()
    .addItem('Rebuild Views Now', 'buildView')
    .addItem('Refresh PigParse Now', 'refreshPigparse')
    .addItem('Refresh Wiki Items Now', 'refreshWikiItems')
    .addItem('Refresh Wiki Spells Now', 'refreshWikiSpells')
    .addItem('Refresh Wiki Gear Tier Now', 'refreshWikiGearTier')
    .addItem('Run Cell-Count Check Now', 'monitorCellCount')
    .addSeparator()
    .addItem('Set Character Info…', 'showCharInfoSidebar')
    .addItem('Set Bank Coin…', 'showBankCoinSidebar')
    .addItem('Search…', 'showSearchSidebar')
    .addItem('Evict Guildie…', 'showEvictionSidebar')
    .addItem('Manage Admins…', 'showAdminMgmtSidebar')   // NEW (Phase 7)
    .addItem('Set Theme…', 'showThemePickerModal')
    .addSeparator()
    .addItem('Run Migration (v=3)', 'migrateToV3')
    .addItem('Run Migration (v=2 legacy)', 'migrateToV2')
    .addSeparator()                                                          // NEW (Phase 7)
    .addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')  // NEW (Phase 7)
    .addToUi();
}
```

Three new menu chain inserts (Manage Admins…, separator, Initialize Admin Allowlist (manual)) + one new try/catch block at the top. All other existing items, separators, and order are preserved verbatim.
</menu_diff>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Update `showEvictionSidebar.test.ts` to seed `guild_admins` for every callback test</name>
  <files>apps-script/src/__tests__/showEvictionSidebar.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/showEvictionSidebar.test.ts (the full file — every test that calls `getEvictionEmails`, `previewEviction`, or `commitEviction`. Note: showEvictionSidebar (opener) tests will ALSO need this seed once the opener guard lands)
    - apps-script/src/__tests__/test-helpers.ts (the `seedMeta(state, rows)` signature)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md §`apps-script/src/triggers/showEvictionSidebar.ts` ("Tests to update: ... currently uses installSessionMock('officer@example.com'). After this modification, those tests will need _meta.guild_admins seeded with ['officer@example.com'] to keep passing. Add this to every beforeEach that touches the eviction callbacks.")
    - apps-script/src/lib/admin.ts (just landed in Plan 01 — confirm `guild_admins` storage shape: JSON.stringify(string[]) lowercased+sorted)
  </read_first>
  <behavior>
    - Every test in `showEvictionSidebar.test.ts` that exercises a callback or the opener must have `_meta.guild_admins` seeded with the test's caller email (`installSessionMock('officer@example.com')` → seed `['officer@example.com']`).
    - The simplest, lowest-touch implementation: in the shared `beforeEach` (or a wrapping helper if there isn't one), add a `seedMeta(state, [['guild_admins', JSON.stringify(['officer@example.com'])], ['workbook_owner_floor', 'officer@example.com']])` call after the existing `_meta` setup. If the existing `beforeEach` doesn't seed `_meta` rows by default, add the new seeding to each test individually that needs it.
    - Tests that specifically WANT to exercise the non-admin path (none exist today; they don't need to be added in this plan — Plan 02's adminMgmtSidebar.test.ts TS2 covers the alert flow at the sidebar layer) should NOT have the admin seed.
    - This task runs BEFORE Task 2 — that way the suite stays green throughout the plan. Sequence:
      1. Update tests to seed `guild_admins` (tests pass against the EXISTING unguarded eviction sidebar because the seed is benign — `_meta.guild_admins` is currently unread).
      2. Then in Task 2 add the guard. Tests STILL pass because the seed is now load-bearing.
  </behavior>
  <action>
Read `apps-script/src/__tests__/showEvictionSidebar.test.ts` in full. For each test that calls `showEvictionSidebar()`, `getEvictionEmails()`, `previewEviction(...)`, or `commitEviction(...)`:

1. Identify the caller email used in `installSessionMock(...)` (currently `'officer@example.com'` for most tests per 07-PATTERNS.md).
2. Ensure that BEFORE the SUT call, `_meta.guild_admins` is seeded with a JSON-stringified array containing that exact email (lowercased+normalized form). Add `_meta.workbook_owner_floor` set to the same email for completeness (matches what bootstrapGuildAdmins would produce on first open).

The cleanest implementation: factor a helper if the seeding pattern repeats across more than 5 tests:

```typescript
function seedAdmins(state: MockState, emails: string[], floor?: string): void {
  const normalized = emails.map((e) => e.toLowerCase().trim()).sort();
  seedMeta(state, [
    ['guild_admins', JSON.stringify(normalized)],
    ['workbook_owner_floor', (floor ?? normalized[0] ?? '').toLowerCase().trim()],
  ]);
}
```

Place it near the top of the test file (after the `installSessionMock` helper). Then add `seedAdmins(state, ['officer@example.com']);` to each test's setup. If most tests share an admin-caller fixture, put the call in a shared `beforeEach`.

Run the suite to confirm GREEN:
```bash
cd apps-script && npm test -- showEvictionSidebar.test
```
All existing tests must pass. The seed is currently benign (no code reads `_meta.guild_admins` yet) but stages the suite for Task 2's guard.
  </action>
  <verify>
    <automated>
      cd apps-script && npm test -- showEvictionSidebar.test 2>&1 | tee /tmp/eviction-test-pre.log; grep -E "FAIL|✗" /tmp/eviction-test-pre.log | wc -l
    </automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path apps-script/src/__tests__/showEvictionSidebar.test.ts -Pattern "guild_admins" -SimpleMatch` matches >= 1 line (the new seed in beforeEach or per-test).
    - `Select-String -Path apps-script/src/__tests__/showEvictionSidebar.test.ts -Pattern "workbook_owner_floor" -SimpleMatch` matches >= 1 line.
    - `(Get-Content apps-script/src/__tests__/showEvictionSidebar.test.ts | Where-Object { $_ -match "  it\\(" }).Count` is >= existing-count (no tests deleted; only seeding added).
    - `cd apps-script; npm test -- showEvictionSidebar.test 2>&1` exits 0; all pre-existing tests still pass (no regression from adding the seed).
    - `cd apps-script; npm test 2>&1` exits 0 (full suite green including admin.test.ts and adminMgmtSidebar.test.ts if Plan 02 has also landed).
  </acceptance_criteria>
  <done>
    Every callback test in `showEvictionSidebar.test.ts` seeds `_meta.guild_admins` with its caller email. The suite is green BEFORE the guard lands in Task 2. This staging guarantees we never observe a red commit in the plan.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add admin guard to showEvictionSidebar opener + 3 callbacks</name>
  <files>apps-script/src/triggers/showEvictionSidebar.ts</files>
  <read_first>
    - apps-script/src/triggers/showEvictionSidebar.ts (the current file — opener at lines 48-57, getEvictionEmails at 59-75, previewEviction starting around line 82, commitEviction at 113-187. Confirm the existing import block at lines 37-39 — your additive import goes here)
    - apps-script/src/lib/admin.ts (Plan 01 — the exact import names: `isAdmin`, `requireAdminOrThrow`, `normalizeEmail`)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md §code_context Integration Points (the exact opener-guard insertion code; same alert copy as the admin-mgmt sidebar but for the eviction-flow variant)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md §`apps-script/src/triggers/showEvictionSidebar.ts (MODIFY)` (the diff scope: opener + 3 callbacks, additive only, no other changes)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-02-admin-mgmt-sidebar-PLAN.md (the analogous opener+callback guard pattern in showAdminMgmtSidebar.ts — clone the shape so the two sidebars are consistent)
  </read_first>
  <behavior>
    - Add 1 new import line to the existing import block (lines 37-39): `import { isAdmin, requireAdminOrThrow, normalizeEmail } from '../lib/admin';`
    - In `showEvictionSidebar`, insert the admin guard immediately AFTER the `themeKey` const at line 49 and BEFORE the `theme` const at line 50:
      ```typescript
      let callerEmail = '';
      try {
        callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
      } catch (_e) { /* sandbox quirk — empty fail-closes */ }
      if (!isAdmin(callerEmail)) {
        SpreadsheetApp.getUi().alert(
          'Not authorized',
          'Only guild officers can evict members. Contact a workbook admin if you think this is wrong.',
          SpreadsheetApp.getUi().ButtonSet.OK,
        );
        log('warn', 'showEvictionSidebar', { notAuthorized: true, callerEmail });
        return;
      }
      ```
    - In `getEvictionEmails` (currently line 59), insert as the FIRST statement of the function body:
      ```typescript
      let callerEmail = '';
      try {
        callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
      } catch (_e) { /* fail-closed below */ }
      requireAdminOrThrow(callerEmail);
      ```
    - Same pattern in `previewEviction` — insert as the FIRST statement of the function body, BEFORE the existing email-validation check (`if (!email || …)` around line 83-85). The admin check fails closed BEFORE we even validate the email argument; that's correct.
    - Same pattern in `commitEviction` — insert as the FIRST statement of the function body, BEFORE the existing email-validation check at line 114.
    - The existing `initiated_by` `Session.getEffectiveUser` fallback in `commitEviction` at lines 146-153 is UNCHANGED. The two `Session.getEffectiveUser` callsites in `commitEviction` (one new for the admin check, one existing for audit-log `initiated_by`) coexist; the audit-log path keeps its `'unknown'` soft-fallback per D-06.
    - NO other changes to the file. The 30-day-grace logic, the LockService envelope at lines 122-186, the audit-log JSON-append at lines 166-178 — all UNCHANGED.

    After saving, run the full suite to confirm everything stays green:
    ```bash
    cd apps-script && npm test
    ```
    All three suites pass: admin.test.ts (Plan 01), adminMgmtSidebar.test.ts (Plan 02), and the now-extended showEvictionSidebar.test.ts.
  </behavior>
  <action>
Edit `apps-script/src/triggers/showEvictionSidebar.ts` with the 4 insertions described in `<behavior>`. The diff is purely additive — no existing lines are deleted or reordered. Patterns:

**Patch 1 — import (after line 39):**
```typescript
import { isAdmin, requireAdminOrThrow, normalizeEmail } from '../lib/admin';
```

**Patch 2 — opener guard (inside `showEvictionSidebar`, after the `themeKey`/`theme` consts at lines 49-50, BEFORE the `html` const):**
```typescript
let callerEmail = '';
try {
  callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
} catch (_e) { /* sandbox quirk — empty fail-closes */ }
if (!isAdmin(callerEmail)) {
  SpreadsheetApp.getUi().alert(
    'Not authorized',
    'Only guild officers can evict members. Contact a workbook admin if you think this is wrong.',
    SpreadsheetApp.getUi().ButtonSet.OK,
  );
  log('warn', 'showEvictionSidebar', { notAuthorized: true, callerEmail });
  return;
}
```

**Patch 3 — callback guard at top of `getEvictionEmails`, `previewEviction`, `commitEviction` (3 places, identical shape — adapt the log op name to match the function):**
```typescript
let callerEmail = '';
try {
  callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
} catch (_e) { /* fail-closed below */ }
requireAdminOrThrow(callerEmail);
```

After editing, run typecheck + tests:
```bash
cd apps-script && npx tsc --noEmit
cd apps-script && npm test
```
Both must exit 0.
  </action>
  <verify>
    <automated>
      cd apps-script && npm test 2>&1 | tee /tmp/eviction-guard-test.log; grep -c "FAIL\|✗" /tmp/eviction-guard-test.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "from '\\.\\./lib/admin'"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "isAdmin\(callerEmail\)"` matches >= 1 line (opener guard).
    - `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "requireAdminOrThrow\(callerEmail\)"` matches exactly 3 lines (one per callback).
    - `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "Only guild officers can evict members" -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "30 \* 24 \* 60 \* 60 \* 1000" -SimpleMatch` matches exactly 1 line (existing GRACE_MS constant unchanged — sanity check that the existing 30-day grace logic was not touched).
    - `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "LockService\.getDocumentLock\(\)\.tryLock\(30000\)"` matches exactly 1 line (existing commit envelope unchanged).
    - `cd apps-script; npx tsc --noEmit 2>&1` exits 0.
    - `cd apps-script; npm test 2>&1` exits 0 with all three suites green (admin.test.ts ≥ 20 PASS, adminMgmtSidebar.test.ts ≥ 5 PASS, showEvictionSidebar.test.ts ≥ existing-count PASS).
    - `cd apps-script; npm run build 2>&1` exits 0; `dist/Code.js` still produces.
  </acceptance_criteria>
  <done>
    The eviction sidebar opener and all 3 callbacks now gate on `isAdmin` / `requireAdminOrThrow`. Existing tests pass because they seeded `guild_admins` in Task 1. The 30-day grace logic, the lock envelope, and the audit log shape are byte-for-byte unchanged. ADMIN-02 is closed at the code layer (integration smoke comes in Task 4).
  </done>
</task>

<task type="auto">
  <name>Task 3: Wire onOpen — lazy bootstrap call + 2 new menu items</name>
  <files>apps-script/src/triggers/onOpen.ts</files>
  <read_first>
    - apps-script/src/triggers/onOpen.ts (the current 57-line file — note that line 1-6 has NO imports today; this is the first file to add imports here)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md §code_context Integration Points (the lazy-bootstrap insertion code; "Wrapped in try/catch + `log('warn', 'bootstrap_failed', { error })` — never throws out of onOpen")
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-UI-SPEC.md §Menu Integration Spec (the exact menu insertion order — lines 304-317)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-02-admin-mgmt-sidebar-PLAN.md (Plan 02 must have landed Code.ts re-exports for `showAdminMgmtSidebar` and `bootstrapGuildAdminsManual` — the global names referenced by the new `.addItem(...)` strings in this task)
  </read_first>
  <behavior>
    - Add 2 new imports to the top of `onOpen.ts`:
      ```typescript
      import { bootstrapGuildAdmins } from '../lib/admin';
      import { log } from '../lib/log';
      ```
    - At the TOP of the `onOpen()` function body (before `SpreadsheetApp.getUi()`), insert the lazy bootstrap:
      ```typescript
      try {
        bootstrapGuildAdmins();
      } catch (err) {
        log('warn', 'onOpen.bootstrap_failed', { error: String(err) });
      }
      ```
    - In the menu chain:
      - INSERT `.addItem('Manage Admins…', 'showAdminMgmtSidebar')` between the existing `.addItem('Evict Guildie…', 'showEvictionSidebar')` (line 22) and `.addItem('Set Theme…', 'showThemePickerModal')` (line 23).
      - AFTER the existing `.addItem('Run Migration (v=2 legacy)', 'migrateToV2')` (line 26), INSERT two new chain calls: `.addSeparator()` then `.addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')`.
      - Both ellipsis characters MUST be Unicode `…` (U+2026), not three ASCII dots. Copy-paste from 07-UI-SPEC.md.
      - The `.addToUi()` call at the end stays UNCHANGED.
    - DO NOT modify the existing `showThemePickerModal` function (lines 32-57). That belongs to Phase 3.
  </behavior>
  <action>
Edit `apps-script/src/triggers/onOpen.ts`:

1. Replace the existing 6-line file-header block (lines 1-6) with the existing comment + 2 new imports:
   ```typescript
   // onOpen — Phase 3 plan 03-04 task 5; Phase 7 plan 07-03 added lazy
   // admin-bootstrap + 2 new menu items (Manage Admins…, Initialize
   // Admin Allowlist (manual)).
   //
   // Adds the SquireBot custom menu when the workbook opens. Phase 3
   // shipped a minimal theme picker modal; Phase 5 replaced it with the
   // polished picker. Phase 7 adds the admin-allowlist surface.

   import { bootstrapGuildAdmins } from '../lib/admin';
   import { log } from '../lib/log';
   ```
2. Replace the existing `onOpen()` body (lines 7-28) with the target shape from `<menu_diff>` above (the try/catch block + the modified menu chain).

   The full replacement function:
   ```typescript
   export function onOpen(): void {
     // Phase 7: lazy admin bootstrap. Errors NEVER throw out of onOpen
     // (would break the menu for everyone). bootstrapGuildAdmins is
     // idempotent + lock-wrapped internally + returns silently on
     // lock_busy (D-01).
     try {
       bootstrapGuildAdmins();
     } catch (err) {
       log('warn', 'onOpen.bootstrap_failed', { error: String(err) });
     }

     SpreadsheetApp.getUi()
       .createMenu('SquireBot')
       .addItem('Install Triggers', 'installTriggers')
       .addSeparator()
       .addItem('Rebuild Views Now', 'buildView')
       .addItem('Refresh PigParse Now', 'refreshPigparse')
       .addItem('Refresh Wiki Items Now', 'refreshWikiItems')
       .addItem('Refresh Wiki Spells Now', 'refreshWikiSpells')
       .addItem('Refresh Wiki Gear Tier Now', 'refreshWikiGearTier')
       .addItem('Run Cell-Count Check Now', 'monitorCellCount')
       .addSeparator()
       .addItem('Set Character Info…', 'showCharInfoSidebar')
       .addItem('Set Bank Coin…', 'showBankCoinSidebar')
       .addItem('Search…', 'showSearchSidebar')
       .addItem('Evict Guildie…', 'showEvictionSidebar')
       .addItem('Manage Admins…', 'showAdminMgmtSidebar')
       .addItem('Set Theme…', 'showThemePickerModal')
       .addSeparator()
       .addItem('Run Migration (v=3)', 'migrateToV3')
       .addItem('Run Migration (v=2 legacy)', 'migrateToV2')
       .addSeparator()
       .addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')
       .addToUi();
   }
   ```
3. Leave `showThemePickerModal` (lines 32-57 of the original file) untouched.

Run typecheck + build:
```bash
cd apps-script && npx tsc --noEmit
cd apps-script && npm run build
```
Both must exit 0. `dist/Code.js` should contain the new menu chain (verify via `grep -E "Manage Admins|Initialize Admin Allowlist" apps-script/dist/Code.js`).
  </action>
  <verify>
    <automated>
      cd apps-script && npm run build 2>&1 | tail -20 && grep -c "Manage Admins\|Initialize Admin Allowlist" apps-script/dist/Code.js
    </automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "from '\\.\\./lib/admin'"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "from '\\.\\./lib/log'"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "bootstrapGuildAdmins\(\)"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "onOpen\.bootstrap_failed" -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "Manage Admins…" -SimpleMatch -Encoding utf8` matches exactly 1 line (Unicode ellipsis).
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "Initialize Admin Allowlist \(manual\)" -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "showAdminMgmtSidebar" -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "bootstrapGuildAdminsManual" -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "Manage Admins\.\.\." -SimpleMatch` matches ZERO lines (must NOT use three ASCII dots).
    - `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "\.addToUi\(\)"` matches exactly 1 line (unchanged).
    - `cd apps-script; npx tsc --noEmit 2>&1` exits 0.
    - `cd apps-script; npm run build 2>&1` exits 0; `Select-String -Path apps-script/dist/Code.js -Pattern "Manage Admins" -SimpleMatch` matches >= 1 line in the built bundle.
    - `cd apps-script; npm test 2>&1` exits 0 (existing test suite still green).
  </acceptance_criteria>
  <done>
    `onOpen.ts` now lazily bootstraps admin allowlist on every workbook open (idempotent) and surfaces 2 new menu items. The bundle builds cleanly with the new menu chain visible in `dist/Code.js`. ADMIN-01 is closed at the code layer; integration smoke comes in Task 4.
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 4: clasp push + dev-workbook smoke (5-hook verification)</name>
  <what-built>
    Plan 01 created `lib/admin.ts` policy module + tests; Plan 02 created `showAdminMgmtSidebar.ts` + tests + Code.ts re-exports; Plan 03 Tasks 1-3 added the eviction guard + onOpen bootstrap + 2 new menu items. The full apps-script bundle is now Phase 7-complete in source. This task ships it to the dev workbook via `clasp push` and verifies all 5 ROADMAP Phase 7 success criteria interactively.

    Pre-checkpoint state (Claude completes automatically before pausing):
    - `cd apps-script && npm test` exits 0 (all suites green: admin.test.ts ≥ 20, adminMgmtSidebar.test.ts ≥ 5, showEvictionSidebar.test.ts ≥ existing-count, no regression in the 297+ baseline).
    - `cd apps-script && npm run build` exits 0; `apps-script/dist/Code.js` exists and contains: `showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`, `Manage Admins`, `Initialize Admin Allowlist (manual)`, `Local\SquireBot` (sanity) — verified by grep.
    - `Select-String -Path apps-script/src/lib/migrations.ts -Pattern "writeMetaRow\('_meta', 'schema_version', '3'\)"` matches exactly 1 line (verification hook 5a: schema_version stays at 3).
    - `Select-String -Path internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion\s*=\s*3"` matches exactly 1 line (verification hook 5b: WatcherMaxSchemaVersion stays at 3).
  </what-built>
  <how-to-verify>
    Detailed runbook (the user — workbook owner — executes these steps; per CLAUDE.md, `clasp` lives on the user's machine with their OAuth credentials in `~/.clasprc.json`).

    **Step 1 — Build + push.**

    Run from the repo root:
    ```bash
    cd apps-script
    npm test            # confirm green one more time
    npm run build       # confirm dist/Code.js fresh
    npx clasp push      # ship to the dev workbook
    ```

    Expected: `clasp push` reports "Pushed N files." with no errors. If clasp reports auth errors, re-run `npx clasp login` per `docs/apps-script-deploy.md`.

    **Step 2 — Open the dev workbook in a browser.**

    Open the dev workbook URL. Wait for the menu to load. Expected: SquireBot menu now has the 2 new items in the locked positions: `Manage Admins…` between `Evict Guildie…` and `Set Theme…`; `Initialize Admin Allowlist (manual)` below the two `Run Migration` items separated by a divider.

    **Step 3 — Verification hook 1 (`_meta.guild_admins` exists + idempotent re-bootstrap).**

    Unhide system tabs (existing `Hide All System Tabs` is a previous v1.0 menu item; reverse it via `Extensions → Apps Script → Files → Code.js → Run → unhideAllSystemTabs` if needed, or use the existing menu's unhide button). Inspect `_meta`. Expected:
    - A `guild_admins` row exists with value `["<owner-email-lowercased>"]` (JSON-stringified array of 1 email).
    - A `workbook_owner_floor` row exists with value `<owner-email-lowercased>` (plain string).
    - An `admin_log` row exists with a JSON-array containing one `{ action: 'bootstrap', email: '<owner>', initiated_by: 'onOpen', at: '<ISO timestamp>' }` entry.

    Close + reopen the workbook (refresh the browser tab). Re-inspect `_meta`. Expected: NO new admin_log entry was appended (idempotent re-bootstrap returned `already_initialized`); the three rows are byte-for-byte unchanged.

    **VERIFY PASS:** verification hook 1 ✓ if both halves above hold. Capture the values in `07-03-SMOKE.md`.

    **Step 4 — Verification hook 2 (non-admin sees alert, eviction is safe no-op).**

    Temporarily edit `_meta.guild_admins` cell value to `["someone-else@example.com"]` (a value that does NOT include your account). Save. Refresh the browser tab.

    Click `SquireBot → Evict Guildie…`. Expected: An Apps Script modal alert appears with title "Not authorized" and body containing "Only guild officers can evict members." Click OK. The modal dismisses; NO sidebar opens.

    Inspect `_char_owner` — no row's `is_removed` column flipped. Inspect `_meta.eviction_log` — no new entry was appended. Inspect `_meta.admin_log` — no new entry was appended (the failed click did not write).

    Restore `_meta.guild_admins` to its bootstrap value (`["<your-email-lowercased>"]`).

    **VERIFY PASS:** verification hook 2 ✓.

    **Step 5 — Verification hook 3 (admin adds peer; peer can evict).**

    Click `SquireBot → Manage Admins…`. Expected: 300px sidebar opens, themed, with title "SquireBot — Manage admins", heading "Manage admins", a `Current admins (1):` heading, a single `<li>` showing your email with `(owner)` annotation and a `[Remove]` button (you ARE the floor so the button shows for you), an input with placeholder `email@example.com`, an `[Add admin]` button.

    Type a second guildie's email (a real Gmail account you can sign in to in another browser session) into the input. Click `Add admin`. Expected: status region shows `Admin added: <email>.` in green; the list updates to show 2 admins.

    Open the dev workbook URL in a second browser session signed in as the new admin. Click `SquireBot → Evict Guildie…`. Expected: the eviction sidebar opens normally (no "Not authorized" alert) and the email selector populates with `_char_owner` rows.

    Close the second-browser session. Back in the first session, click `Remove` on the second admin's row. Confirm the browser-confirm dialog. Expected: status region shows `Admin removed: <email>.` and the list re-renders to 1 entry.

    **VERIFY PASS:** verification hook 3 ✓.

    **Step 6 — Verification hook 4 (owner-floor lockout).**

    With the admin-mgmt sidebar still open as the workbook owner (floor), verify: the floor row (your email) has both the `(owner)` annotation AND a `[Remove]` button visible (because you ARE the floor, the client-side suppression doesn't apply to you).

    Add a second admin (`testpeer@example.com` — can be a synthetic email; this test is about server-side enforcement and does not need a real peer login). Add. Verify list updates to 2 admins.

    Open another tab signed in as the second admin (or simulate via the dev workbook owner re-running the admin-mgmt sidebar after temporarily hand-editing `_meta.guild_admins` to put a different caller email first — but the cleanest test is a real second account). Open Manage Admins as the second admin.

    Expected: the second admin sees the floor row (`<owner-email> (owner)`) WITHOUT a Remove button (client-side suppression because `callerEmail !== floor`).

    Defense-in-depth test (only if convenient): open browser devtools in the second admin's sidebar, manually invoke `google.script.run.withSuccessHandler(console.log).withFailureHandler(console.error).removeAdmin('<owner-email>')`. Expected: the server-side throws `'owner_floor_protected'`, the failure handler logs the error, and `_meta.guild_admins` is unchanged.

    Verify by inspecting `_meta.guild_admins` — owner email is still present; the second admin is still present.

    **VERIFY PASS:** verification hook 4 ✓.

    **Step 7 — Verification hook 5 (grep gates).**

    From the repo root (NOT in apps-script):
    ```bash
    grep -n "schema_version" apps-script/src/lib/migrations.ts | grep -E "writeMetaRow|'3'"
    grep -n "WatcherMaxSchemaVersion" internal/sheet/client.go
    ```
    Expected: `migrations.ts` shows the existing `writeMetaRow('_meta', 'schema_version', '3')` line and nothing new (no `'4'` bump). `client.go` shows the existing `WatcherMaxSchemaVersion = 3` constant.

    **VERIFY PASS:** verification hook 5 ✓.

    **Step 8 — Record results.**

    Write `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SMOKE.md` with:
    - Timestamp of clasp push (and the `clasp push` output paste).
    - For each of the 5 hooks above: PASS/FAIL/N-A, with one-line evidence (e.g., `Hook 1 PASS: _meta.guild_admins=["jbowen@mncivic.com"]; second-open admin_log unchanged.`).
    - Any deviations observed (e.g., `getOwner() returned null on dev workbook; manual-fallback path exercised instead via Initialize Admin Allowlist (manual) menu item — bootstrapped successfully.`).

    Cleanup: any test admins added during smoke that you don't want to keep should be removed via the sidebar before signing off.
  </how-to-verify>
  <resume-signal>
    Type "approved" to mark Phase 7 shipped (writes SUMMARY + advances STATE.md to phase_7_complete). If a verification hook FAILED, paste the failure details — Claude will analyze, propose a fix, and either create a follow-up plan (`/gsd-plan-phase 7 --gaps`) or patch in place if it's a trivial fix-and-republish.
  </resume-signal>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Eviction sidebar entry point (`showEvictionSidebar` opener) | The destructive workflow's entry guard. Non-admin must see the alert and be unable to construct the sidebar. |
| Eviction sidebar callbacks (`getEvictionEmails`, `previewEviction`, `commitEviction`) | Defense-in-depth: even if a stale sidebar instance survives a demotion, every callback re-checks. |
| `onOpen` simple-trigger context | Runs without user authorization grant on first open; any throw here breaks the menu for the user. The lazy-bootstrap try/catch is the seatbelt. |
| `clasp push` deploy operation | Deploys ALL changes atomically. Plan 02 + Plan 03 must ship together — partial deploys leave menu items pointing at non-existent function names. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-07-03-01 | Spoofing | A non-admin invokes `commitEviction(email)` via `google.script.run` from devtools / a stale sidebar / a separate script | mitigate | The new FIRST statement of `commitEviction` is `requireAdminOrThrow(callerEmail)` where `callerEmail = Session.getEffectiveUser().getEmail()` — server-side identity, never client-supplied. Tested at the policy layer by admin.test.ts T2, integration-tested at the sidebar layer by Step 4 of the smoke runbook. |
| T-07-03-02 | Spoofing | Lazy `onOpen` bootstrap claims `Session.getActiveSpreadsheet().getOwner()` even though the active sheet may not be owned by the runner | accept | `bootstrapGuildAdmins` uses `getOwner()` only for the seed; the seed is then visible in `_meta.workbook_owner_floor` and admins can remove themselves voluntarily. If `getOwner()` returns the wrong user under a sharing-corner-case, the floor can be hand-edited or re-bootstrapped via the manual-fallback menu item. Manual-fallback (D-01) handles the consumer-account-null case. |
| T-07-03-03 | Tampering | Lazy `bootstrapGuildAdmins` throws unexpectedly (e.g., transient API error), breaking the SquireBot menu for the user | mitigate | The lazy call is `try { bootstrapGuildAdmins(); } catch (err) { log('warn', ...) }`. Even if the underlying primitive somehow throws (currently designed to return `{ reason: 'lock_busy' }` etc., not throw), the menu chain still builds. Next workbook open retries the lazy bootstrap. |
| T-07-03-04 | Information Disclosure | Apps Script Stackdriver logs reveal every admin invocation of eviction, including `email` of the target | accept | Same logging convention as v1.0 (eviction sidebar already logged this). Stackdriver access scoped to script owner. |
| T-07-03-05 | Denial of Service | Repeated rapid `onOpen` calls (e.g., user mass-switching tabs) cause lock contention on the bootstrap | mitigate | `bootstrapGuildAdmins` returns `{ reason: 'lock_busy' }` silently on contention (D-01 — does not throw out of onOpen). Once one open wins the lock, the rest see the idempotent already-initialized check on the next open. Tested by admin.test.ts T19. |
| T-07-03-06 | Tampering | The 2 new menu items reference function-name strings (`showAdminMgmtSidebar`, `bootstrapGuildAdminsManual`) that must resolve to globals in `dist/Code.js`; a partial-deploy where Plan 02 wasn't pushed yet leaves the menu items as broken "Script function not found" errors | mitigate | Plan 02 owns the Code.ts re-exports for those 5 globals; Plan 03 cannot reach Task 4 (clasp push) until Plan 02 has shipped (both are wave-2 plans; the clasp push task is a single shared deploy). The smoke runbook explicitly verifies the menu items resolve. If Plan 02 has NOT shipped at clasp-push time, the user is instructed (in the resume signal) to deploy Plan 02 first. |
| T-07-03-07 | Elevation of Privilege | A user invokes `bootstrapGuildAdminsManual` from the menu when `_meta.guild_admins` is empty and seeds themselves as owner-floor | accept | This is the intended fallback when `getOwner()` returns null. The `getUi().alert(... Continue?)` confirmation modal makes the action visible to the user. Only relevant when the allowlist is empty (idempotent check) — once initialized the function is a no-op. Captured in Plan 02 threat T-07-02-10. |

ASVS L1: zero high-severity threats. The two load-bearing mitigations are T-07-03-01 (server-side requireAdminOrThrow on every callback — tested in 3 separate test files: admin.test.ts policy layer, adminMgmtSidebar.test.ts sidebar layer, showEvictionSidebar.test.ts seeded-tests) and T-07-03-06 (deploy-time consistency — enforced by the wave-2 dependency in this phase's plan structure).
</threat_model>

<verification>
- `cd apps-script; npm test 2>&1` exits 0 (all 3+ test suites green: admin.test.ts ≥ 20, adminMgmtSidebar.test.ts ≥ 5, showEvictionSidebar.test.ts ≥ existing-count, plus the 297+ baseline).
- `cd apps-script; npm run build 2>&1` exits 0; `apps-script/dist/Code.js` contains all 5 new global names AND the 2 new menu-item label strings.
- `Select-String -Path apps-script/src/triggers/showEvictionSidebar.ts -Pattern "requireAdminOrThrow" -SimpleMatch` matches exactly 3 lines (callbacks) plus 1 line in the import block = 4 total.
- `Select-String -Path apps-script/src/triggers/onOpen.ts -Pattern "bootstrapGuildAdmins" -SimpleMatch` matches >= 2 lines (import + call).
- `Select-String -Path apps-script/src/lib/migrations.ts -Pattern "writeMetaRow\('_meta', 'schema_version', '3'\)"` shows the existing line exactly once — verification hook 5a.
- `Select-String -Path internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion\s*=\s*3"` matches exactly 1 line — verification hook 5b.
- `clasp push` succeeded (paste in `07-03-SMOKE.md`).
- All 5 verification hooks marked PASS in `07-03-SMOKE.md` with one-line evidence each.
</verification>

<success_criteria>
- `showEvictionSidebar.ts` opener admin-gates BEFORE building HTML; all 3 callbacks call `requireAdminOrThrow` first. Existing 30-day-grace + LockService + audit log shape are byte-for-byte unchanged.
- `onOpen.ts` lazily bootstraps the admin allowlist on every workbook open (idempotent), never throws, and surfaces 2 new menu items in the locked positions per 07-UI-SPEC.
- Existing eviction-sidebar tests pass (seeded with `_meta.guild_admins` in Task 1 to keep the suite green throughout the plan).
- Full apps-script test suite passes (admin.test.ts ≥ 20, adminMgmtSidebar.test.ts ≥ 5, showEvictionSidebar.test.ts ≥ existing-count, all baseline tests).
- `clasp push` deploys the bundle to the dev workbook; smoke evidence in `07-03-SMOKE.md` confirms all 5 verification hooks PASS.
- ADMIN-01 closed: `_meta.guild_admins` exists after first open + idempotent re-open (hook 1 PASS).
- ADMIN-02 closed: non-admin sees alert + eviction is safe no-op (hook 2 PASS).
- ADMIN-03 already closed at the code layer by Plans 01+02; this plan adds the integration evidence (hook 3+4 PASS via the smoke).
- Schema impact: zero. Watcher impact: zero. (Hook 5 PASS via grep gates.)
- Ship gate per ROADMAP.md Phase 7: `clasp push` of the apps-script bundle + admin-management UX smoke ✓.
</success_criteria>

<output>
After completion, create `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md` capturing:
- Files modified (3 absolute paths: showEvictionSidebar.ts, onOpen.ts, showEvictionSidebar.test.ts).
- Files created (1: 07-03-SMOKE.md).
- Test results (full suite green; hook-PASS table from the smoke).
- The `clasp push` output (file count + any warnings).
- Confirmation that `schema_version` and `WatcherMaxSchemaVersion` are untouched.
- Phase 7 ship verdict: SHIPPED + UAT-verified, or SHIPPED-pending-UAT if any hook deferred.
- Update STATE.md `last_activity` and `progress.completed_plans` to reflect Phase 7 closure.
</output>
