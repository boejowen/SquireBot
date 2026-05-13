---
phase: 10-apps-script-test-quality
plan: 02
plan_id: 10-02-admin-mgmt-inline-test
type: execute
wave: 2
depends_on: [10-01-test04-fixes]
files_modified:
  - apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts
autonomous: true
requirements: [TEST-03]
tags: [test-coverage, admin-mgmt, inline-js, sidebar, wave2]

must_haves:
  truths:
    - "A new file exists at `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` containing exactly 2 `it(...)` test blocks: one happy-path (TM1) + one error-path (TM2), mirroring the Phase 8 D-03 2-test pattern locked for the other 4 inline-JS sidebars."
    - "TM1 (happy path) mounts the Admin-Mgmt sidebar's SIDEBAR_BODY via the IIFE-fixed `mountSidebar` helper (from Plan 10-01), fills the email input, clicks the Add button, resolves `addAdmin` via `m.dispatchRunCall(...)`, resolves the follow-up `getAdminList` call, and asserts the success message renders in `#msg` AND the input is cleared."
    - "TM2 (error path) mounts the sidebar, fills the input, clicks Add, then resolves `addAdmin` with `m.failRunCall(...)` carrying a 'not_authorized' or 'invalid_email'-class error, and asserts the error message renders in `#msg` AND the input is NOT cleared (preserves user's typed value)."
    - "Both tests use the same imports + beforeEach + describe-block shape as `bankCoinSidebar.inline.test.ts` (the canonical reference template): `import { describe, it, expect, beforeEach } from 'vitest'; import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers'; import { buildSidebarHtml } from '../triggers/showAdminMgmtSidebar';`"
    - "Total apps-script vitest count after this plan lands is ≥ 338 (336 baseline + 2 new TM1/TM2 — accounting for Test 4 in searchIndex.test.ts continuing to pass or being clearly flagged per Plan 10-01 Task 3 deviation)."
    - "No production code modified EXCEPT a single export-keyword affordance on `buildSidebarHtml` in `apps-script/src/triggers/showAdminMgmtSidebar.ts` (line 135) — this is the same test-affordance edit landed in v1.0.1 Plan 08-02 for the other 4 sidebars (verify: `grep -n 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` returns 4 lines pre-plan, 5 lines post-plan). This single keyword change is the ONLY production-code edit in Phase 10 and is explicitly justified by the symmetry requirement in CONTEXT.md D-03."
    - "Schema lock unchanged: `_meta.schema_version` = 3; `WatcherMaxSchemaVersion = 3`."
  artifacts:
    - path: apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts
      provides: "New inline-JS test file for Admin-Mgmt sidebar — TM1 happy + TM2 error, mirroring the 4 existing inline-JS files. ~80-100 LOC."
      contains: "showAdminMgmtSidebar — inline JS"
      min_lines: 70
    - path: apps-script/src/triggers/showAdminMgmtSidebar.ts
      provides: "`buildSidebarHtml` function gains the `export` keyword on line 135 (was `function buildSidebarHtml`; becomes `export function buildSidebarHtml`). Body unchanged."
      contains: "export function buildSidebarHtml"
  key_links:
    - from: "apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts"
      to: "apps-script/src/triggers/showAdminMgmtSidebar.ts buildSidebarHtml"
      via: "ESM import: `import { buildSidebarHtml } from '../triggers/showAdminMgmtSidebar';`"
      pattern: "import \\{ buildSidebarHtml \\} from '\\.\\./triggers/showAdminMgmtSidebar'"
    - from: "apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts"
      to: "apps-script/src/__tests__/test-helpers.ts (IIFE-fixed mountSidebar from Plan 10-01)"
      via: "ESM import: `import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';`"
      pattern: "mountSidebar"
    - from: "Plan 10-02"
      to: "Plan 10-01"
      via: "CROSS-PLAN DEPENDENCY: 10-01 MUST land before 10-02 — the new tests consume the IIFE-wrapped mountSidebar; without the IIFE wrap, top-level `var state` declarations from the Admin-Mgmt inline script leak to globalThis and bleed into subsequent tests"
      pattern: "depends_on: \\[10-01"
---

<objective>
Author the 5th and final inline-JS sidebar test file — `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` — closing TEST-03 and completing 5/5 shipping-sidebar inline-JS coverage. The file mirrors the 4 existing inline test files exactly (`bankCoinSidebar.inline.test.ts`, `charInfoSidebar.inline.test.ts`, `evictionSidebar.inline.test.ts`, `searchSidebar.inline.test.ts`) so future maintainers learn one pattern across all 5 sidebars (CONTEXT.md D-03 + D-06 maintenance-simplicity rule).

Per CONTEXT.md D-03, the file contains **exactly 2 tests**: one happy path (TM1) + one error path (TM2). Reasoning (locked under "simple, seamless" criterion D-06):
- Symmetry with the 4 existing inline-JS test files. One pattern, applied 5 places.
- The admin-gate (`requireAdminOrThrow` + `SpreadsheetApp.getUi().alert(...)`) is already covered at the trigger layer by `adminMgmtSidebar.test.ts` (existing) + `admin.test.ts` (existing). The inline-JS layer's job is DOM event handlers + `google.script.run` wiring — same surface as the other 4.
- Adding 3rd/4th cases (e.g., owner-floor-protected, list-rendering edge cases) would test admin-mgmt logic already covered server-side, doubling review surface with no new coverage.

Per CONTEXT.md D-04, this plan runs in **wave 2 sequentially after Plan 10-01** because the new tests consume the IIFE-wrapped `mountSidebar` from 10-01. If 10-02 ran in parallel with 10-01, the new Admin-Mgmt test would either (a) be written against the leaky mountSidebar and need rewriting after 10-01 merges, or (b) silently corrupt other tests via top-level `var state = {...}` leakage from the Admin-Mgmt inline script.

**Special note on the single production-code edit (per CONTEXT.md D-01 read carefully):**

The Admin-Mgmt sidebar trigger at `apps-script/src/triggers/showAdminMgmtSidebar.ts:135` currently declares its HTML builder as `function buildSidebarHtml(theme: Theme | null): string {` (NO `export`). The other 4 sidebars all export this function (`showSearchSidebar.ts:114`, `showEvictionSidebar.ts:252`, `showCharInfoSidebar.ts:129`, `showBankCoinSidebar.ts:77` — verify with `grep -n 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` returning 4 lines pre-plan). The export was uniformly added to the other 4 in v1.0.1 Plan 08-02 as the test-affordance enabler for the inline-JS test pattern.

Plan 10-02 therefore adds the SAME single-keyword `export` to `showAdminMgmtSidebar.ts:135`, closing the asymmetry. This is:
- Not a behavior change (pure name-visibility affordance).
- The same test-affordance edit that landed across the other 4 sidebars in Phase 8.
- Strictly required to mount the Admin-Mgmt SIDEBAR_BODY into the test realm (the test file cannot otherwise reach the inline HTML).

Per CONTEXT.md D-01's tripwire ("if any plan touches `apps-script/src/triggers/` STOP"): this is a known, narrow exception that the planner has elevated to the surface here for the executor to acknowledge. If the executor finds itself doing ANYTHING ELSE in `showAdminMgmtSidebar.ts` beyond adding the single `export` keyword on line 135, STOP — that is scope creep. No other production code edit in Phase 10 is authorized.

**Scope discipline — non-negotiable:**
- The ONLY production-code edit allowed in this plan is adding `export` to `function buildSidebarHtml` on `showAdminMgmtSidebar.ts:135`. Body unchanged. No other `apps-script/src/triggers/` edits. No `apps-script/src/lib/` edits.
- No schema changes. `_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3.
- Exactly 2 tests (TM1 + TM2) — do NOT expand to 3 or 4. If during execution an obvious additional gap surfaces (e.g., owner-floor-Remove flow), flag it as a Phase 10 deviation note in the SUMMARY rather than silently expanding the test count (per CONTEXT.md D-03 constraint).
- DO NOT extract `SIDEBAR_BODY` to a standalone `.html` file (backlog 999.7 still deferred — CONTEXT.md `<code_context>` anti-pattern list).
- DO NOT introduce new helper utilities in `test-helpers.ts` — reuse existing `mountSidebar`/`dispatchRunCall`/`failRunCall`/`resetMocks`/`seedMeta`.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/10-apps-script-test-quality/10-CONTEXT.md
@.planning/phases/10-apps-script-test-quality/10-01-test04-fixes-PLAN.md
@.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md
@CLAUDE.md
@apps-script/src/triggers/showAdminMgmtSidebar.ts
@apps-script/src/__tests__/adminMgmtSidebar.test.ts
@apps-script/src/__tests__/bankCoinSidebar.inline.test.ts
@apps-script/src/__tests__/charInfoSidebar.inline.test.ts

<interfaces>
<!-- Key contracts the executor needs. Extracted from the trigger source + reference templates. -->
<!-- Do NOT re-explore the codebase; use these directly. -->

From apps-script/src/triggers/showAdminMgmtSidebar.ts (DOM IDs derived from SIDEBAR_BODY at lines 179-190):

```html
<div class="sidebar">
  <h3>Manage admins</h3>
  <p class="desc">...</p>
  <div id="listRegion">
    <label id="listHeading">Current admins:</label>
    <ul id="adminList"></ul>
  </div>
  <label for="addInput">Add admin</label>
  <input id="addInput" type="text" placeholder="email@example.com" ... />
  <button id="addBtn" class="primary">Add admin</button>
  <div id="msg" aria-live="polite"></div>
</div>
```

**CRITICAL — DOM element IDs to use in the test (verbatim from the trigger SIDEBAR_BODY):**
- Email input: `#addInput` (NOT `#email` — that was a placeholder in CONTEXT.md §5).
- Add button: `#addBtn` (NOT `#add` — that was a placeholder in CONTEXT.md §5).
- Status/message region: `#msg` (matches CONTEXT.md §5).
- (Read-only context: `#adminList` is the rendered list ul; `#listHeading` is the count label.)

From apps-script/src/triggers/showAdminMgmtSidebar.ts inline-JS `onAdd()` function (lines 238-254):

```javascript
function onAdd() {
  var input = document.getElementById('addInput');
  var value = String(input.value || '').trim();
  if (!value) { setMsg('Invalid email: empty. No changes were written.', 'error'); return; }
  var btn = document.getElementById('addBtn');
  btn.disabled = true;
  google.script.run
    .withSuccessHandler(function(result) {
      btn.disabled = false;
      input.value = '';                                       // ← happy path clears input
      if (result && result.alreadyExists) setMsg('Already in list: ' + value + '.', 'success');
      else setMsg('Admin added: ' + value + '.', 'success');  // ← happy-path success copy
      google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();  // ← follow-up call
    })
    .withFailureHandler(function(err) { btn.disabled = false; routeError(err); })
    .addAdmin(value);                                         // ← google.script.run.addAdmin(...)
}
```

**Happy-path expected flow (TM1):**
1. Mount sidebar — `init()` fires on DOMContentLoaded (or immediately) and enqueues `getAdminList` for the initial render.
2. Drain that initial `getAdminList` call: `m.dispatchRunCall('getAdminList', { admins: ['existing@example.com'], floor: 'owner@example.com', callerEmail: 'owner@example.com' });`
3. Fill `#addInput` with `'newadmin@example.com'`.
4. Click `#addBtn`.
5. Drain `addAdmin`: `m.dispatchRunCall('addAdmin', { added: true });`
6. The success handler enqueues a second `getAdminList` call (line 250) — drain it: `m.dispatchRunCall('getAdminList', { admins: ['existing@example.com', 'newadmin@example.com'], floor: 'owner@example.com', callerEmail: 'owner@example.com' });`
7. Assert: `#msg` text contains `'Admin added: newadmin@example.com'`; `#addInput.value === ''` (cleared).

**Error-path expected flow (TM2)** — from inline-JS `withFailureHandler` (line 252) → `routeError(err)` (lines 205-212):

```javascript
function routeError(err) {
  var m = (err && err.message) ? String(err.message) : String(err || '');
  if (/owner_floor_protected/.test(m)) setMsg('Owner-floor protected — only the workbook owner can remove themselves. No changes were written.', 'error');
  else if (/not_authorized/.test(m))   setMsg('Not authorized — you are no longer an admin. Please close this sidebar.', 'error');
  else if (/invalid_email/.test(m))    setMsg('Invalid email: ' + m + '. No changes were written.', 'error');
  else if (/lock_busy/.test(m))        setMsg('Action failed: another admin action is in flight. Please retry. No changes were written.', 'error');
  else                                  setMsg('Action failed: ' + m + '. No changes were written.', 'error');
}
```

**Error-path locked choice: use `invalid_email` (already-exists-class is rendered as success per inline-JS line 248, so it is NOT an error in this UI). Plan invokes:**
1. Drain initial `getAdminList` (same as TM1 step 2).
2. Fill `#addInput` with `'bademail'` (a value the server-side validator will reject; the inline `onAdd` does NOT pre-validate non-empty strings — it sends them to `addAdmin`).
3. Click `#addBtn`.
4. `m.failRunCall('addAdmin', { message: 'invalid_email' });` — triggers the inline-JS `withFailureHandler` path → `routeError` → matches `/invalid_email/` branch → renders `'Invalid email: invalid_email. No changes were written.'` in `#msg`.
5. Assert: `#msg` text contains `'Invalid email'`; `#addInput.value === 'bademail'` (NOT cleared — the failure handler does not call `input.value = ''`); the button is re-enabled (`btn.disabled = false`).

From apps-script/src/__tests__/bankCoinSidebar.inline.test.ts (the canonical 2-test template — mirror this shape exactly):

```typescript
import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showBankCoinSidebar';

describe('showBankCoinSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '3'],
      ['theme', 'minimalist'],
      ['bank_toon_name', 'BankToon'],
    ]);
  });

  it('TB1 — initial-data populates pp/gp/sp/cp inputs and enables saveBtn', () => { /* ... */ });
  it('TB2 — getBankCoinForForm failure renders #msg with the error copy', () => { /* ... */ });
});
```

From apps-script/src/__tests__/charInfoSidebar.inline.test.ts (for any nuance differences — read for second-template comparison; same shape).

From apps-script/src/__tests__/test-helpers.ts API surface (read-only — DO NOT modify):

```typescript
export function resetMocks(): MockState;
export function seedMeta(state: MockState, rows: Array<[string, string]>): void;
export function mountSidebar(html: string): {
  document: Document;
  dispatchRunCall: (method: string, payload: unknown) => void;
  failRunCall: (method: string, error: { message: string }) => void;
  // ... and other internals not needed by this test
};
export type MockState = { /* opaque */ };
```

`buildSidebarHtml` signature for Admin-Mgmt (after the `export` affordance lands):

```typescript
// apps-script/src/triggers/showAdminMgmtSidebar.ts:135
export function buildSidebarHtml(theme: Theme | null): string;
// Pass `null` for sheets-default theme in the test (matches the 4 existing templates).
```
</interfaces>
</context>

<tasks>

<task type="checkpoint:human-action" gate="advisory">
  <name>Task 0 (preflight): Verify Plan 10-01 has landed</name>
  <what-built>Plan 10-01's IIFE wrap in mountSidebar — this plan's tests depend on it.</what-built>
  <how-to-verify>
    Run:
    ```bash
    cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot"
    grep -nE '\(0, eval\)\(.*\(function\(\)\{' apps-script/src/__tests__/test-helpers.ts
    ```
    Expected: exactly 1 line of output showing the IIFE wrap. If the grep returns 0 lines, Plan 10-01 has not landed — STOP and surface to the user before proceeding. Plan 10-02 cannot proceed without 10-01 because the new Admin-Mgmt test's top-level `var state` and `function escapeHtml` declarations would leak to globalThis and bleed into the rest of the suite (which is exactly the WR-01 bug 10-01 closes).
  </how-to-verify>
  <resume-signal>Automatic — proceed if grep returns 1 line; halt if it returns 0.</resume-signal>
</task>

<task type="auto" tdd="false">
  <name>Task 1: Add `export` keyword to buildSidebarHtml in showAdminMgmtSidebar.ts (single-keyword test affordance)</name>
  <files>apps-script/src/triggers/showAdminMgmtSidebar.ts</files>
  <read_first>
    - apps-script/src/triggers/showAdminMgmtSidebar.ts (line 135 — the `function buildSidebarHtml` declaration; read lines 130-145 for context)
    - apps-script/src/triggers/showBankCoinSidebar.ts (line 77 — reference for the export-keyword convention)
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §1-§5 (justification for this single production-code affordance edit)
  </read_first>
  <behavior>
    - Line 135 of `showAdminMgmtSidebar.ts` changes from `function buildSidebarHtml(theme: Theme | null): string {` to `export function buildSidebarHtml(theme: Theme | null): string {`.
    - The function body, all callers within the file (line 69 inside `showAdminMgmtSidebar` opener), and all other exports/imports remain unchanged.
    - No behavior change. Pure name-visibility affordance to enable ESM import from the test file.
  </behavior>
  <action>
    Open `apps-script/src/triggers/showAdminMgmtSidebar.ts`. Locate line 135. Add `export ` before `function`. The exact diff:

    ```diff
    -function buildSidebarHtml(theme: Theme | null): string {
    +export function buildSidebarHtml(theme: Theme | null): string {
    ```

    DO NOT modify ANY other line in this file. DO NOT touch the SIDEBAR_BODY template literal, the opener function, the `getAdminList`/`addAdmin`/`removeAdmin` callbacks, or the `themeStyleBlock` helper.

    Verify the change is a 1-line diff via `git diff apps-script/src/triggers/showAdminMgmtSidebar.ts` — exactly one `-` line and one `+` line. If the diff shows ANY other change, revert and retry with the surgical 1-keyword edit.

    Schema-impact assertion (per CONTEXT.md D-05): this task touches only the export keyword on one line. `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` and `_meta.schema_version = 3` in `apps-script/src/lib/migrations.ts` are NOT in this task's scope. No schema change.
  </action>
  <verify>
    <automated>cd apps-script && npx tsc --noEmit && grep -c 'export function buildSidebarHtml' apps-script/src/triggers/showAdminMgmtSidebar.ts</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE 'export function buildSidebarHtml' apps-script/src/triggers/showAdminMgmtSidebar.ts` matches exactly 1 line (the new export).
    - `git diff --stat apps-script/src/triggers/showAdminMgmtSidebar.ts` shows exactly `1 file changed, 1 insertion(+), 1 deletion(-)` (one `-`/`+` pair — the keyword swap on line 135).
    - `cd apps-script && npx tsc --noEmit` exits 0 (typecheck clean — confirms no broken callers, no duplicate-export errors).
    - `grep -c 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` returns 5 (was 4 pre-plan; now all 5 sidebars export `buildSidebarHtml` symmetrically).
    - Schema gates still hold: `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1.
  </acceptance_criteria>
  <done>Single `export` keyword added to line 135; 1-line diff verified; typecheck clean; 5/5 sidebars now export `buildSidebarHtml` symmetrically.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Create showAdminMgmtSidebar.inline.test.ts with TM1 happy + TM2 error</name>
  <files>apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts</files>
  <read_first>
    - apps-script/src/triggers/showAdminMgmtSidebar.ts (lines 191-273 — the full inline `<script>` block; you need the exact `init()` / `onAdd()` / `routeError()` flow to derive the expected `google.script.run` call sequence)
    - apps-script/src/__tests__/bankCoinSidebar.inline.test.ts (full file — the canonical 2-test template; mirror its shape, imports, beforeEach, and describe block exactly)
    - apps-script/src/__tests__/charInfoSidebar.inline.test.ts (full file — second template; useful if Admin-Mgmt's `init()` enqueues different calls than `getBankCoinForForm`)
    - apps-script/src/__tests__/evictionSidebar.inline.test.ts (post-Plan-10-01 — note the TE1 `toContain('Marked 2 character(s) as removed')` exact-substring style; mirror this assertion strictness in TM1/TM2 messages)
    - apps-script/src/__tests__/adminMgmtSidebar.test.ts (full file — read for happy-path fixture-data shape: existing admins, floor, callerEmail. Do NOT duplicate its admin-gate coverage in the inline test; that's already trigger-layer coverage.)
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §5 (TM1/TM2 expected flow — note the planner has already mapped placeholder IDs `#email`/`#add` to the actual IDs `#addInput`/`#addBtn` in the `<interfaces>` block above)
  </read_first>
  <behavior>
    - The new file is ~80-110 LOC, mirroring `bankCoinSidebar.inline.test.ts` structure.
    - Exactly 2 `it(...)` blocks inside a single `describe('showAdminMgmtSidebar — inline JS', () => { ... })`.
    - **TM1 (happy path):**
      - Mount sidebar via `mountSidebar(buildSidebarHtml(null))` (sheets-default theme — matches the 4 existing templates that pass `null`).
      - Drain the initial `getAdminList` call enqueued by `init()` with a seed list.
      - Set `#addInput.value = 'newadmin@example.com'`.
      - Click `#addBtn`.
      - Drain `addAdmin` with `{ added: true }`.
      - Drain the follow-up `getAdminList` enqueued by `addAdmin`'s success handler (line 250 of trigger).
      - Assert: `#msg.textContent` contains `'Admin added: newadmin@example.com'`; `#addInput.value === ''` (cleared by line 247 of trigger).
    - **TM2 (error path):**
      - Mount sidebar.
      - Drain the initial `getAdminList` call (same as TM1).
      - Set `#addInput.value = 'bademail'`.
      - Click `#addBtn`.
      - Fail `addAdmin` with `{ message: 'invalid_email' }` via `m.failRunCall('addAdmin', { message: 'invalid_email' })`.
      - Assert: `#msg.textContent` contains `'Invalid email'`; `#addInput.value === 'bademail'` (NOT cleared); `(addBtn as HTMLButtonElement).disabled === false` (re-enabled by inline `withFailureHandler` line 252).
    - `beforeEach` calls `resetMocks()` + `seedMeta(state, [['schema_version', '3'], ['theme', 'sheets-default']])` (matches the bankCoin template; no admin-list seed needed because the test resolves `getAdminList` directly via `dispatchRunCall`).
    - NO `installSessionMock(...)` call is needed — the inline-JS test exercises the CLIENT side (DOM + `google.script.run`); `Session.getEffectiveUser()` runs server-side and is only relevant to the trigger-layer `adminMgmtSidebar.test.ts`.
    - NO admin-gate assertions — that's trigger-layer coverage (covered by existing `adminMgmtSidebar.test.ts` TS2 + `admin.test.ts`). Per CONTEXT.md D-03 constraint.
    - NO Remove-button / owner-floor tests — that's 3rd/4th test territory; rejected by D-03 (2-test limit).
  </behavior>
  <action>
    Create a new file at `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` with the following structure. Use the exact import lines, describe block name, and beforeEach shape (matching `bankCoinSidebar.inline.test.ts`). The test bodies follow the TM1/TM2 expected flows documented in the `<interfaces>` block above.

    ```typescript
    // Phase 10 plan 10-02 — Admin-Mgmt sidebar inline-JS tests (TEST-03).
    //
    // Closes the v1.0.1 TEST-02 deferral note: 5/5 shipping sidebars now have
    // inline-JS coverage (Search, Eviction, Bank-Coin, Char-Info from v1.0.1;
    // Admin-Mgmt added in v1.0.2).
    //
    // Coverage per Phase 10 CONTEXT.md D-03 (2-test pattern locked, mirroring
    // the 4 existing inline-JS sidebars):
    //   TM1 — happy path: user fills #addInput, clicks #addBtn, addAdmin
    //         succeeds → success message renders in #msg + input is cleared.
    //   TM2 — error path: addAdmin fails with 'invalid_email' → routeError
    //         renders 'Invalid email: ...' in #msg + input is NOT cleared.
    //
    // Admin-gate coverage (server-side requireAdminOrThrow path) is in
    // adminMgmtSidebar.test.ts TS2 + admin.test.ts — NOT duplicated here.
    // Remove-button + owner-floor-lockout flows are deferred to v1.1 per
    // CONTEXT.md D-03 (2-test limit).
    //
    // Consumes the IIFE-fixed mountSidebar from Plan 10-01 (CONTEXT.md D-02)
    // so top-level `var state` and `function escapeHtml` declarations in the
    // sidebar inline script stay scoped to the IIFE.

    import { describe, it, expect, beforeEach } from 'vitest';
    import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
    import { buildSidebarHtml } from '../triggers/showAdminMgmtSidebar';

    describe('showAdminMgmtSidebar — inline JS', () => {
      let state: MockState;
      beforeEach(() => {
        state = resetMocks();
        seedMeta(state, [
          ['schema_version', '3'],
          ['theme', 'sheets-default'],
        ]);
      });

      // --- TM1 — D-03 happy path ------------------------------------------

      it('TM1 — successful addAdmin renders success copy in #msg and clears #addInput', () => {
        const html = buildSidebarHtml(null);
        const m = mountSidebar(html);

        // init() fires getAdminList immediately on mount — drain it.
        m.dispatchRunCall('getAdminList', {
          admins: ['existing@example.com'],
          floor: 'owner@example.com',
          callerEmail: 'owner@example.com',
        });

        // User fills the email input and clicks Add.
        const addInput = m.document.getElementById('addInput') as HTMLInputElement;
        addInput.value = 'newadmin@example.com';
        (m.document.getElementById('addBtn') as HTMLButtonElement).click();

        // Drain addAdmin success.
        m.dispatchRunCall('addAdmin', { added: true });

        // The success handler enqueues a second getAdminList for the
        // refresh render — drain it (mirrors the pattern in
        // searchSidebar.inline.test.ts TS1 post Plan 10-01 WR-04 fix).
        m.dispatchRunCall('getAdminList', {
          admins: ['existing@example.com', 'newadmin@example.com'],
          floor: 'owner@example.com',
          callerEmail: 'owner@example.com',
        });

        const msg = m.document.getElementById('msg')!;
        expect(msg.textContent || msg.innerHTML).toContain('Admin added: newadmin@example.com');
        // Input cleared by inline-JS line 247 (input.value = '' on success).
        expect(addInput.value).toBe('');
      });

      // --- TM2 — D-03 error path ------------------------------------------

      it('TM2 — addAdmin failure (invalid_email) renders error copy in #msg and preserves #addInput', () => {
        const html = buildSidebarHtml(null);
        const m = mountSidebar(html);

        // Drain initial getAdminList from init().
        m.dispatchRunCall('getAdminList', {
          admins: ['existing@example.com'],
          floor: 'owner@example.com',
          callerEmail: 'owner@example.com',
        });

        const addInput = m.document.getElementById('addInput') as HTMLInputElement;
        addInput.value = 'bademail';
        const addBtn = m.document.getElementById('addBtn') as HTMLButtonElement;
        addBtn.click();

        // Fail addAdmin — routeError matches /invalid_email/ branch.
        m.failRunCall('addAdmin', { message: 'invalid_email' });

        const msg = m.document.getElementById('msg')!;
        const text = msg.textContent || msg.innerHTML;
        expect(text).toContain('Invalid email');
        expect(text).toContain('invalid_email');
        // Input NOT cleared on failure (preserves user's typed value).
        expect(addInput.value).toBe('bademail');
        // Button re-enabled by inline-JS line 252 (btn.disabled = false).
        expect(addBtn.disabled).toBe(false);
      });
    });
    ```

    Deviation-note guidance:
    - If `init()` enqueues a DIFFERENT method name than `getAdminList` (e.g., the trigger refactored to `loadInitialAdmins` between CONTEXT.md authoring and now), use the actual method name. Verify via re-reading the trigger's inline `<script>` block.
    - If `addAdmin`'s success handler enqueues MORE than one follow-up call (currently just `getAdminList` per line 250), drain each in order with separate `dispatchRunCall` invocations.
    - If the production success copy on line 249 changes from `'Admin added: ' + value + '.'` to a different format, use the actual copy in the `toContain` assertion. DO NOT loosen to `toMatch(/added/i)` — that's the WR-02 anti-pattern.

    DO NOT create any other files. DO NOT modify `test-helpers.ts` (already patched by 10-01). DO NOT modify any other inline test file. DO NOT extract SIDEBAR_BODY to a standalone .html file (999.7 deferred per CONTEXT.md).
  </action>
  <verify>
    <automated>cd apps-script && npm test -- --run __tests__/showAdminMgmtSidebar.inline.test.ts</automated>
  </verify>
  <acceptance_criteria>
    - File exists: `ls apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` succeeds.
    - File contains exactly 2 `it(...)` blocks: `grep -cE "^\s*it\(" apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` returns 2.
    - File imports the right symbols: `grep -nE "import \{ resetMocks, seedMeta, mountSidebar, type MockState \} from './test-helpers'" apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` matches 1 line.
    - File imports buildSidebarHtml from showAdminMgmtSidebar: `grep -nE "import \{ buildSidebarHtml \} from '\.\./triggers/showAdminMgmtSidebar'" apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` matches 1 line.
    - File uses mountSidebar at least twice (once per test): `grep -cE 'mountSidebar\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` returns ≥ 2.
    - File uses dispatchRunCall at least 4 times (initial getAdminList + addAdmin success + follow-up getAdminList in TM1, plus initial getAdminList in TM2): `grep -cE 'dispatchRunCall\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` returns ≥ 3 (4 is exact; ≥ 3 allows for the deviation cases above).
    - File uses failRunCall at least once (in TM2): `grep -cE 'failRunCall\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` returns ≥ 1.
    - `cd apps-script && npm test -- --run __tests__/showAdminMgmtSidebar.inline.test.ts` reports TM1 + TM2 PASS (2/2).
    - Full apps-script suite count is ≥ 338 (336 baseline + 2 new). Verify with `cd apps-script && npm test -- --run | tail -5` showing the total.
    - Schema gates: `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1; `grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts` returns ≥1.
  </acceptance_criteria>
  <done>New file `showAdminMgmtSidebar.inline.test.ts` exists with exactly 2 `it(...)` blocks (TM1 + TM2); both pass; full suite count ≥ 338; symmetry with the 4 existing inline-JS test files maintained.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| test realm → mountSidebar eval (consumed from 10-01) | Inline-script source for the Admin-Mgmt SIDEBAR_BODY is evaluated in the test realm via the IIFE-wrapped indirect eval from Plan 10-01. The IIFE scopes `var state`, `function escapeHtml`, `function onAdd`, etc. to the IIFE; nothing leaks to globalThis. |
| test fixtures → production code | Read-only inspection of `showAdminMgmtSidebar.ts` allowed (Task 2). The single permitted write is adding the `export` keyword to line 135 in Task 1 — pure name-visibility, no behavior change. |
| inline-JS test → server-side trigger | The new tests exercise CLIENT-side DOM event handlers + `google.script.run` mock wiring. They do NOT cross the server-side boundary (`requireAdminOrThrow`, `Session.getEffectiveUser`, `lib/admin.addAdmin`). Server-side coverage is already provided by `adminMgmtSidebar.test.ts` + `admin.test.ts`. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-10-02-01 | Tampering | new inline-JS test file | accept | Test file under `apps-script/src/__tests__/`; not bundled into `dist/Code.js` (esbuild entry is `src/Code.ts`, which does not import test files). No production-shipping surface. |
| T-10-02-02 | Information Disclosure | fixture data (admin emails, owner email) | accept | All fixtures use `@example.com` placeholders. Zero PII; zero real guildie data. |
| T-10-02-03 | Spoofing | bypass of admin-gate via inline test | mitigate | The inline test exercises CLIENT-side flows ONLY — it does NOT bypass the server-side `requireAdminOrThrow` gate (which executes on the Apps Script server, not in JSDOM). Inline tests cannot ship malicious admin-add calls to a real workbook because they run only in vitest. Admin-gate coverage remains at trigger-layer in `adminMgmtSidebar.test.ts` TS2. |
| T-10-02-04 | Elevation of Privilege | `export` keyword affordance (Task 1) | accept | Adding `export` to `buildSidebarHtml` does NOT change Apps Script runtime semantics. Apps Script's trigger system finds triggers by global function name (set by the esbuild footer). `buildSidebarHtml` is NOT a trigger callable (it's a build helper called only by `showAdminMgmtSidebar`); making it `export` only exposes it to ESM imports inside the apps-script TypeScript project. No new attack surface. |

**Schema impact:** NONE. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` is unchanged.

**Scope-creep tripwire (per CONTEXT.md D-01):** the ONLY production-code edit allowed in this plan is the single `export` keyword on `showAdminMgmtSidebar.ts:135`. If at any point the executor finds themselves editing the function body, the SIDEBAR_BODY template literal, the opener function, or ANY other file under `apps-script/src/triggers/` or `apps-script/src/lib/`, STOP and surface to Plan 10-03's ship-gate checkpoint. Read-only inspection of the trigger source to derive fixture DOM IDs and method-name expectations is the intended workflow.
</threat_model>

<verification>
1. `cd apps-script && npx tsc --noEmit` exits 0 (typecheck clean — confirms the `export` affordance does not create duplicate-export or unused-export errors).
2. `cd apps-script && npm test -- --run` exits 0; TM1 + TM2 both PASS; total suite count ≥ 338.
3. `grep -cE '^\s*it\(' apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` returns exactly 2 (no scope expansion beyond 2-test pattern).
4. `grep -c 'export function buildSidebarHtml' apps-script/src/triggers/*.ts` returns 5 (4 baseline + the new Admin-Mgmt export — symmetric across all 5 sidebars).
5. `git diff --stat apps-script/src/triggers/showAdminMgmtSidebar.ts` shows 1 insertion + 1 deletion (single keyword swap on line 135).
6. `git diff --stat apps-script/src/triggers/` shows ONLY `showAdminMgmtSidebar.ts` changed; no other trigger file modified.
7. `git diff --stat apps-script/src/lib/` shows zero files changed.
8. `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1 (schema lock).
9. `grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts` returns ≥1 (schema lock).
10. All 4 existing inline-JS test files still pass post-plan (no regression from the IIFE wrap + the new file's coexistence).
</verification>

<success_criteria>
- TEST-03 closed: `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` exists with exactly 2 tests (TM1 happy + TM2 error) that both pass.
- 5/5 shipping sidebars now have inline-JS test coverage (Search, Eviction, Bank-Coin, Char-Info from v1.0.1; Admin-Mgmt added this plan).
- The v1.0.1 TEST-02 "admin-mgmt inline-JS tests deferred to v1.1" wording in REQUIREMENTS.md can be retired (Plan 10-03 SUMMARY notes this).
- Production-code edits are limited to a single `export` keyword on `showAdminMgmtSidebar.ts:135` (test-affordance only; no behavior change; symmetry with the 4 other sidebars).
- Schema constants unchanged.
- Apps-script vitest suite count is ≥ 338 (336 baseline + 2 new; Test 4 of searchIndex.test.ts continues passing per Plan 10-01 outcome — if it failed under WR-03 removal, that's the load-bearing deviation for Plan 10-03 ship-gate to address).
- Plan 10-03's ship gate can now run with green CI and clasp push.
</success_criteria>

<output>
After completion, create `.planning/phases/10-apps-script-test-quality/10-02-SUMMARY.md` summarizing:
- The new test file's TM1 + TM2 shape, exact method-call sequence per test, and assertion list.
- Verification of all must_haves truths.
- Confirmation that the single production-code edit is exactly the `export` keyword on `showAdminMgmtSidebar.ts:135` (with `git diff --stat` evidence).
- Confirmation that `git diff --stat apps-script/src/triggers/` shows only `showAdminMgmtSidebar.ts` changed.
- Pre/post apps-script suite test counts (must show +2 from 336 → 338, modulo Plan 10-01 Task 3 WR-03 outcome).
- Note for Plan 10-03 ship gate: "TEST-03 closed; 5/5 inline-JS sidebars green; ready for clasp push."
- Any deviations encountered (e.g., if `init()` enqueued a different method name; if `addAdmin`'s success handler enqueued additional follow-up calls; if the production success copy on line 249 differs from `'Admin added: ' + value + '.'`).
</output>
</content>
</invoke>