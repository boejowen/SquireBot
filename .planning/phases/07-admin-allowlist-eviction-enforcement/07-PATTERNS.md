# Phase 7: Admin Allowlist + Eviction Enforcement — Pattern Map

**Mapped:** 2026-05-11
**Files analyzed:** 7 (4 CREATE, 3 MODIFY)
**Analogs found:** 7 / 7 (every file has a strong in-repo analog)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `apps-script/src/lib/admin.ts` (CREATE) | lib / policy module | request-response (read+write `_meta` via lib) | `apps-script/src/lib/migrations.ts` (lock envelope) + `apps-script/src/triggers/showEvictionSidebar.ts:113-187` (audit-log envelope) | role-match (no existing `lib/` policy module — closest is migrations.ts for the lock pattern; eviction sidebar for the audit-log shape) |
| `apps-script/src/triggers/showAdminMgmtSidebar.ts` (CREATE) | trigger / sidebar | request-response (HtmlService + google.script.run callbacks) | `apps-script/src/triggers/showEvictionSidebar.ts` | exact (per CONTEXT.md §canonical_refs — this is THE contract anchor) |
| `apps-script/src/__tests__/admin.test.ts` (CREATE) | test (unit) | request-response | `apps-script/src/__tests__/migrations.test.ts` + `apps-script/src/__tests__/showEvictionSidebar.test.ts` | exact (vitest + mocked SpreadsheetApp + seedMeta) |
| `apps-script/src/__tests__/adminMgmtSidebar.test.ts` (CREATE) | test (unit) | request-response | `apps-script/src/__tests__/showEvictionSidebar.test.ts` | exact |
| `apps-script/src/triggers/showEvictionSidebar.ts` (MODIFY) | trigger / sidebar | request-response | self (additive guard — opener + 3 callbacks) | self |
| `apps-script/src/triggers/onOpen.ts` (MODIFY) | trigger / menu | event-driven (simple trigger) | self | self |
| `apps-script/src/Code.ts` (MODIFY) | config / re-export footer | n/a (module re-exports) | self | self |

---

## Pattern Assignments

### `apps-script/src/lib/admin.ts` (lib, policy module)

**Primary analog:** `apps-script/src/triggers/showEvictionSidebar.ts` (envelope shape, lock-wrapped write, malformed-tolerant parse)
**Secondary analog:** `apps-script/src/lib/migrations.ts` (lock envelope idiom)

**Imports pattern** (from `showEvictionSidebar.ts:37-39` — verbatim shape):
```typescript
import { log } from '../lib/log';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from '../lib/sheet-helpers';
```
The admin module is itself a `lib/` file, so imports become `from './log'` and `from './sheet-helpers'` (sibling, not parent). NO direct `getRange` calls — go through `readMetaRows` / `writeMetaRow` per CONTEXT.md §canonical_refs.

**LockService envelope** (clone from `migrations.ts:89-93` AND `showEvictionSidebar.ts:122-124,184-186`):
```typescript
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) throw new Error('addAdmin: lock_busy');
try {
  // ... read _meta, mutate JSON array, writeMetaRow, append admin_log ...
} finally {
  lock.releaseLock();
}
```
Error message prefix uses the function name (`addAdmin:` / `removeAdmin:`) matching `commitEviction: lock_busy` at `showEvictionSidebar.ts:123`. Mandatory per CONTEXT.md §canonical_refs (Pitfall P6).

**Malformed-JSON-tolerant parse** (clone from `showEvictionSidebar.ts:166-176` — verbatim):
```typescript
const meta = readMetaRows('_meta');
const row = meta.find((r) => r.key === 'guild_admins');
let list: string[] = [];
if (row && row.value) {
  try {
    const parsed = JSON.parse(row.value);
    if (Array.isArray(parsed)) list = parsed.map((s) => String(s).toLowerCase().trim()).filter(Boolean);
  } catch (_e) {
    log('warn', 'getAdminList', { malformedExistingList: true });
  }
}
```
Note the `parsed.map(...).filter(Boolean)` normalization step is new — guild_admins values are emails and need lowercase+trim defensive read per CONTEXT.md §specifics "apply normalization at THREE points: ... (1) on read".

**Audit-log append pattern** (clone from `showEvictionSidebar.ts:155-178`):
```typescript
const entry = {
  at: new Date().toISOString(),
  action: 'add' as const,        // 'add' | 'remove' | 'bootstrap' | 'bootstrap_failed'
  email,
  initiated_by: initiatedBy,
};
const meta = readMetaRows('_meta');
const row = meta.find((r) => r.key === 'admin_log');
let logList: unknown[] = [];
if (row && row.value) {
  try {
    const parsed = JSON.parse(row.value);
    if (Array.isArray(parsed)) logList = parsed;
  } catch (_e) {
    log('warn', 'addAdmin', { malformedExistingLog: true });
  }
}
logList.push(entry);
writeMetaRow('_meta', 'admin_log', JSON.stringify(logList));
```
The envelope mirrors `_meta.eviction_log` shape exactly per CONTEXT.md §specifics (note `reason` field absent — admin_log doesn't need it).

**Caller-identity / `initiated_by` fallback** (clone from `showEvictionSidebar.ts:146-153`):
```typescript
let initiatedBy = 'unknown';
try {
  const effective = Session.getEffectiveUser().getEmail();
  if (effective) initiatedBy = effective;
} catch (_e) { /* sandbox quirk — fall through to 'unknown' */ }
```
Per D-06: this `'unknown'` fallback is **only for audit-log recording**. For `requireAdminOrThrow` authorization, empty string is fail-closed (NOT 'unknown').

**Structured logging** (every public function emits at least one log call — pattern from `migrations.ts:68-70`):
```typescript
log('info', 'addAdmin', { email, callerEmail, alreadyExists: false });
log('warn', 'bootstrapGuildAdmins', { skipped: 'lock_busy' });
log('warn', 'requireAdminOrThrow', { notAuthorized: true, callerEmail });
```
op = function name; fields include `email` and `callerEmail` for traceability per CONTEXT.md §code_context.

**MigrationOutcome-style return enum (for bootstrap)** (analog: `migrations.ts:36-39`):
```typescript
// bootstrapGuildAdmins returns a discriminated shape rather than throwing:
return { bootstrapped: false, reason: 'already_initialized' };
return { bootstrapped: true, seedEmail };
return { bootstrapped: false, reason: 'owner_null' };  // getOwner() returned null
```

---

### `apps-script/src/triggers/showAdminMgmtSidebar.ts` (trigger, sidebar)

**Primary analog:** `apps-script/src/triggers/showEvictionSidebar.ts` (verbatim contract anchor per CONTEXT.md §canonical_refs)

**Opener pattern with admin guard** (lines 48-56 of analog, plus the admin-guard insertion from CONTEXT.md §code_context):
```typescript
export function showAdminMgmtSidebar(): void {
  const callerEmail = (Session.getEffectiveUser().getEmail() ?? '').toLowerCase().trim();
  if (!isAdmin(callerEmail)) {
    SpreadsheetApp.getUi().alert(
      'Not authorized',
      'Only guild officers can manage admins. Contact a workbook admin if you think this is wrong.',
      SpreadsheetApp.getUi().ButtonSet.OK,
    );
    return;
  }
  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];
  const html = HtmlService
    .createHtmlOutput(buildSidebarHtml(theme))
    .setTitle('SquireBot — Manage admins')
    .setWidth(300);
  SpreadsheetApp.getUi().showSidebar(html);
  log('info', 'showAdminMgmtSidebar', { theme: themeKey, callerEmail });
}
```

**themeStyleBlock + buildSidebarHtml** (CLONE VERBATIM from `showEvictionSidebar.ts:191-217`):
```typescript
function themeStyleBlock(theme: Theme | null): string {
  if (!theme) return '';
  return `
    <style>
      :root {
        --bg: ${theme.headerBg};
        --bg-row: ${theme.rowAltBg};
        --fg: ${theme.rowFg};
        --fg-header: ${theme.headerFg};
        --accent-bg: ${theme.accentBg};
        --accent-fg: ${theme.accentFg};
        --font-header: ${theme.fontFamily};
        --font-body: ${theme.fontFamily};
        --space-xs: 4px; --space-sm: 8px; --space-md: 12px; --space-lg: 16px; --space-xl: 24px;
      }
    </style>`;
}
function buildSidebarHtml(theme: Theme | null): string {
  const tokens = themeStyleBlock(theme);
  const fallbackVars = !theme
    ? '<style>:root { --space-xs:4px; --space-sm:8px; --space-md:12px; --space-lg:16px; --space-xl:24px; --bg:#f8f9fa; --bg-row:#fff; --fg:#222; --fg-header:#222; --accent-bg:#1a73e8; --accent-fg:#fff; --font-header:Arial,sans-serif; --font-body:Arial,sans-serif; }</style>'
    : '';
  return tokens + fallbackVars + SIDEBAR_BODY;
}
```
Two functions, copied byte-for-byte. The whole point of these helpers being in-file (not lib) is to keep each sidebar self-contained per the 999.7 deferral.

**`SIDEBAR_BODY` String.raw template** (clone shape from `showEvictionSidebar.ts:228-323`):
- Keep the `<style>` block verbatim (lines 229-246) — same spacing tokens, same `#msg.error`/`#msg.success` rules.
- ADD a new `.remove-btn` rule per UI-SPEC §Color (transparent bg, `var(--fg)` at opacity 0.7, 11px font, 24px min-height, 3px radius, `border: 1px solid var(--bg)`).
- Body swaps from `<select>`+`<div class="preview">`+`<button class="primary">` to:
  - `<ul id="adminList"></ul>` (rendered client-side from `getAdminList()`)
  - `<label for="addInput">Add admin</label>`
  - `<input id="addInput" type="text" placeholder="email@example.com" aria-label="New admin email address">`
  - `<button id="addBtn" class="primary">Add admin</button>`
  - `<div id="msg" aria-live="polite"></div>` (unchanged from line 254)

**`escapeHtml` helper** (CLONE VERBATIM from `showEvictionSidebar.ts:257`):
```javascript
function escapeHtml(s) { const d = document.createElement('div'); d.textContent = String(s == null ? '' : s); return d.innerHTML; }
```
Per CONTEXT.md §canonical_refs: "do not re-implement." Every interpolation into the inline `<script>` MUST go through this.

**Client-side `init()` pattern** (clone shape from `showEvictionSidebar.ts:260-269`):
```javascript
function init() {
  google.script.run.withSuccessHandler(renderList).withFailureHandler(showErr).getAdminList();
  document.getElementById('addBtn').addEventListener('click', onAdd);
  document.getElementById('addInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') onAdd();
  });
}
```
Note keyboard-Enter listener is new (UI-SPEC §A11y); no existing analog. `document.getElementById('addInput').focus()` at the end of `renderList`'s success handler per UI-SPEC §A11y.

**Client-side confirm-before-action** (clone from `showEvictionSidebar.ts:300-301`):
```javascript
if (!window.confirm('Remove ' + email + ' from the admin allowlist? ...')) return;
```
Copy text per UI-SPEC §Copywriting > Per-row Remove confirmation.

**Server-side callback skeleton** (each callback first calls `requireAdminOrThrow`, then locks, then mutates):
```typescript
export function addAdmin(email: string): { added: boolean; alreadyExists?: boolean } {
  const callerEmail = (Session.getEffectiveUser().getEmail() ?? '').toLowerCase().trim();
  requireAdminOrThrow(callerEmail);
  // ... validation, lock-wrapped mutate via lib/admin.ts internals ...
}
```
Per CONTEXT.md §canonical_refs: thin wrappers delegate to `lib/admin.ts` primitives. Sidebar callbacks own the `Session` call; lib functions take `callerEmail` as a parameter (testability).

---

### `apps-script/src/__tests__/admin.test.ts` (test, unit)

**Primary analog:** `apps-script/src/__tests__/showEvictionSidebar.test.ts` (Session mock, seedMeta, lock failure, log envelope assertions)

**Test-helpers imports** (verbatim from analog lines 25-26):
```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { resetMocks, makeSheet, seedMeta, type MockState } from './test-helpers';
```

**Session mock** (clone from `showEvictionSidebar.test.ts:42-47` — verbatim):
```typescript
function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}
```
Plus the `afterEach(() => { delete (globalThis as Record<string, unknown>).Session; })` cleanup at file bottom (line 281-284 of analog).

**Test scenario coverage map** (per CONTEXT.md §verification_hooks unit-testable column):

| Test ID | Function under test | Scenario |
|---------|---------------------|----------|
| T1 | `requireAdminOrThrow` | empty caller email → throws `not_authorized` (fail-closed per D-06) |
| T2 | `requireAdminOrThrow` | caller not in list → throws `not_authorized` |
| T3 | `requireAdminOrThrow` | caller in list (case-mismatched in cell) → succeeds (normalized compare) |
| T4 | `isAdmin` | empty list → false |
| T5 | `isAdmin` | case-insensitive match against stored lowercase value |
| T6 | `getAdminList` | malformed JSON in `_meta.guild_admins` → returns `{ admins: [], floor: '' }` (fail-closed read per D-05) |
| T7 | `addAdmin` | happy path: appends sorted+lowercased; returns `{ added: true }`; admin_log entry written |
| T8 | `addAdmin` | idempotent: existing email → `{ added: false, alreadyExists: true }`; NO log entry |
| T9 | `addAdmin` | rejects empty email and missing-`@` email |
| T10 | `addAdmin` | lock-busy → throws `addAdmin: lock_busy`; no `_meta` write |
| T11 | `removeAdmin` | non-floor target by non-floor caller → `{ removed: true }`; log entry |
| T12 | `removeAdmin` | floor target by non-floor caller → throws `owner_floor_protected`; NO writes |
| T13 | `removeAdmin` | floor target by FLOOR caller (self-removal) → `{ removed: true }`; floor row NOT updated (orphan per D-04) |
| T14 | `removeAdmin` | not-in-list → `{ removed: false, notFound: true }`; NO log entry (idempotent) |
| T15 | `bootstrapGuildAdmins` | empty `_meta.guild_admins` → writes seed; writes `workbook_owner_floor`; appends bootstrap log entry |
| T16 | `bootstrapGuildAdmins` | non-empty `_meta.guild_admins` → no-op; returns `{ bootstrapped: false }` |
| T17 | `bootstrapGuildAdmins` | `getOwner()` returns null → log warn; writes `bootstrap_failed` admin_log entry; returns `{ bootstrapped: false, reason: 'owner_null' }` |
| T18 | `bootstrapGuildAdmins` | manual fallback with seedEmail opt → uses opts.seedEmail (not getOwner) |
| T19 | `bootstrapGuildAdmins` | lock-busy → silent no-op (DOES NOT throw — per D-01 onOpen must not throw) |
| T20 | `appendAdminLogEntry` | malformed existing log → starts fresh; warn logged |

**Lock-failure assertion pattern** (clone from `showEvictionSidebar.test.ts:212-223`):
```typescript
state.lockTryLockReturn = false;
expect(() => addAdmin('new@x.com', 'admin@x.com')).toThrowError(/addAdmin: lock_busy/);
const meta = state.sheets.get('_meta')!;
expect(meta.values.find((r) => r[0] === 'guild_admins' && r[1] !== '[]')).toBeUndefined();
```

**Audit-log JSON-decode assertion** (clone shape from `showEvictionSidebar.test.ts:168-188`):
```typescript
const meta = state.sheets.get('_meta')!;
const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
expect(list.length).toBe(1);
expect(list[0].action).toBe('add');
expect(list[0].email).toBe('new@x.com');
expect(list[0].initiated_by).toBe('admin@x.com');
expect(String(list[0].at)).toMatch(/^\d{4}-\d{2}-\d{2}T/);
```

**`appendedRowsLog` / `setValuesLog` write-count assertion** (clone from `showEvictionSidebar.test.ts:207-209`):
```typescript
// Verify exactly 1 write to guild_admins on idempotent re-add (none, because alreadyExists)
const writes = state.setValuesLog.filter((w) => w.sheet === '_meta');
expect(writes.length).toBe(0);
```

---

### `apps-script/src/__tests__/adminMgmtSidebar.test.ts` (test, unit)

**Primary analog:** `apps-script/src/__tests__/showEvictionSidebar.test.ts` lines 49-69 (sidebar opens + width + title + body asserts)

**Sidebar-opens test pattern** (clone from analog lines 57-68):
```typescript
it('Test 1 — opens 300px-wide sidebar with locked title + body strings', () => {
  installSessionMock('admin@example.com');
  seedMeta(state, [
    ['schema_version', '3'],
    ['guild_admins', JSON.stringify(['admin@example.com'])],
    ['workbook_owner_floor', 'admin@example.com'],
  ]);
  showAdminMgmtSidebar();
  const captured = (state as MockState & { lastSidebar?: { _html: string; _title: string; _width: number } }).lastSidebar;
  expect(captured).toBeDefined();
  expect(captured!._width).toBe(300);
  expect(captured!._title).toBe('SquireBot — Manage admins');
  expect(captured!._html).toContain('Manage admins');
  expect(captured!._html).toContain('Add admin');
  expect(captured!._html).toContain('email@example.com'); // placeholder
});
```

**Non-admin opener test** (new — no exact analog; closest is the admin-guard logic from showEvictionSidebar after Phase 7 modifies it; for now this test asserts the alert was called and NO sidebar captured):
```typescript
it('non-admin caller → alert fired, no sidebar opens', () => {
  installSessionMock('intruder@example.com');
  seedMeta(state, [['guild_admins', JSON.stringify(['admin@example.com'])]]);
  // ... capture SpreadsheetApp.getUi().alert via mock and assert title + body ...
  expect(captured.alertCalled).toBe(true);
  expect(captured.lastSidebar).toBeUndefined();
});
```
NOTE: the test-helpers SpreadsheetApp.getUi() mock may need a small extension to capture `alert(title, body, buttonSet)` calls. The eviction test file does not exercise alert(), so this is the one new mock surface Phase 7 tests need. Planner should verify against `test-helpers.ts` lines 121-300 (not yet read here) and extend if missing.

**Callback shape tests:**
- `getAdminList()` returns `{ admins: string[]; floor: string; callerEmail: string }` (sorted, lowercased)
- `addAdmin(email)` server-side validates + delegates to lib (most coverage in admin.test.ts, lighter here)
- `removeAdmin(email)` owner-floor server-side enforcement (defense-in-depth; main coverage in admin.test.ts T12)

Inline-JS DOM tests are DEFERRED to Phase 8 TEST-02 per CONTEXT.md §out-of-scope.

---

### `apps-script/src/triggers/showEvictionSidebar.ts` (MODIFY)

**Diff scope:** add admin guard to opener + 3 callbacks. NO other changes.

**Opener guard insertion** — insert immediately after the `themeKey` const at current line 49 (per CONTEXT.md §code_context):
```typescript
export function showEvictionSidebar(): void {
  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];
  // NEW (Phase 7):
  const callerEmail = (Session.getEffectiveUser().getEmail() ?? '').toLowerCase().trim();
  if (!isAdmin(callerEmail)) {
    SpreadsheetApp.getUi().alert(
      'Not authorized',
      'Only guild officers can evict members. Contact a workbook admin if you think this is wrong.',
      SpreadsheetApp.getUi().ButtonSet.OK,
    );
    return;
  }
  // ... rest unchanged ...
}
```

**Callback guard insertion** — first statement of each of `getEvictionEmails`, `previewEviction`, `commitEviction`:
```typescript
export function getEvictionEmails(): string[] {
  const callerEmail = (Session.getEffectiveUser().getEmail() ?? '').toLowerCase().trim();
  requireAdminOrThrow(callerEmail);
  // ... rest unchanged from current line 60+ ...
}
```
Same pattern in `previewEviction` (after the email-validation check at line 83-85) and `commitEviction` (after the email-validation check at line 114).

**Imports addition** — add to the import block at lines 37-39:
```typescript
import { isAdmin, requireAdminOrThrow } from '../lib/admin';
```

**Tests to update:** `showEvictionSidebar.test.ts` currently uses `installSessionMock('officer@example.com')`. After this modification, those tests will need `_meta.guild_admins` seeded with `['officer@example.com']` to keep passing. Add this to every `beforeEach` that touches the eviction callbacks. (The existing 12 tests should remain green; planner should sequence: bump tests FIRST, then add the guard.)

---

### `apps-script/src/triggers/onOpen.ts` (MODIFY)

**Diff scope:** lazy bootstrap call at top + 2 new menu items.

**Lazy bootstrap insertion** — at the top of `onOpen()`, BEFORE the `createMenu` chain (per CONTEXT.md §code_context Integration Points):
```typescript
export function onOpen(): void {
  // NEW (Phase 7) — lazy admin bootstrap. Errors NEVER throw out of onOpen
  // (would break the menu for everyone). Idempotent; lock-wrapped internally;
  // silent no-op on lock contention.
  try {
    bootstrapGuildAdmins();
  } catch (err) {
    log('warn', 'onOpen.bootstrap_failed', { error: String(err) });
  }

  SpreadsheetApp.getUi()
    .createMenu('SquireBot')
    // ... existing items unchanged ...
}
```

**Menu insertion** — exactly per UI-SPEC §Menu Integration Spec line 304-317:
- Insert `.addItem('Manage Admins…', 'showAdminMgmtSidebar')` between current line 22 (`Evict Guildie…`) and line 23 (`Set Theme…`).
- Insert `.addSeparator().addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')` after current line 26 (`Run Migration (v=2 legacy)`).

**Imports addition** — add new top-of-file imports:
```typescript
import { bootstrapGuildAdmins } from '../lib/admin';
import { log } from '../lib/log';
```
(Currently `onOpen.ts` has no imports — see line 1-6. This is the first time imports get added to this file.)

**Ellipsis character:** use Unicode `…` (U+2026), NOT three ASCII dots. Per UI-SPEC §Menu label rules.

---

### `apps-script/src/Code.ts` (MODIFY)

**Diff scope:** re-export footer additions. The Apps Script trigger system finds functions by global name; missing re-export = silent menu failure (CONTEXT.md §canonical_refs).

**Pattern to follow** — every existing import-block + export-block pair (e.g., lines 44-49 for showEvictionSidebar):
```typescript
// IMPORT BLOCK (after line 49):
import {
  showAdminMgmtSidebar,
  getAdminList,
  addAdmin,
  removeAdmin,
} from './triggers/showAdminMgmtSidebar';
import { bootstrapGuildAdminsManual } from './lib/admin';

// EXPORT BLOCK (in the existing `export { ... }` at lines 56-71, append):
showAdminMgmtSidebar, getAdminList, addAdmin, removeAdmin,
bootstrapGuildAdminsManual,
```

**Note:** `bootstrapGuildAdmins` (the lazy/auto version) does NOT need to be re-exported as a global — it's called only from within `onOpen.ts`, which already imports it as a module. Only the manual-fallback `bootstrapGuildAdminsManual` needs the global lift (it's referenced by name from the menu `.addItem(..., 'bootstrapGuildAdminsManual')` string).

---

## Shared Patterns

### Pattern 1: LockService envelope for `_meta` writes

**Source:** `apps-script/src/lib/migrations.ts:55-74` AND `apps-script/src/triggers/showEvictionSidebar.ts:122-186`

**Apply to:** `addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`, `appendAdminLogEntry` in `lib/admin.ts`.

```typescript
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) throw new Error('<funcName>: lock_busy');
try {
  // ... all reads + writes to _meta happen here ...
} finally {
  lock.releaseLock();
}
```

**Exception for `bootstrapGuildAdmins`:** per D-01, lock-busy is a silent no-op (NOT a throw), because the function is called from `onOpen` simple-trigger context where a throw would break the menu. Pattern:
```typescript
if (!lock.tryLock(30000)) {
  log('warn', 'bootstrapGuildAdmins', { skipped: 'lock_busy' });
  return { bootstrapped: false, reason: 'lock_busy' };
}
```
This is the ONLY admin-module function that swallows lock-busy instead of throwing.

### Pattern 2: Malformed-JSON-tolerant read

**Source:** `apps-script/src/triggers/showEvictionSidebar.ts:166-176`

**Apply to:** every read of `_meta.guild_admins` and `_meta.admin_log` in `lib/admin.ts`.

```typescript
const meta = readMetaRows('_meta');
const row = meta.find((r) => r.key === '<key>');
let list: T[] = [];
if (row && row.value) {
  try {
    const parsed = JSON.parse(row.value);
    if (Array.isArray(parsed)) list = parsed;
  } catch (_e) {
    log('warn', '<funcName>', { malformedExisting<X>: true });
  }
}
```

For `guild_admins` specifically, add a normalization step inside the `if (Array.isArray(parsed))` branch: `list = parsed.map((s) => String(s).toLowerCase().trim()).filter(Boolean);` per CONTEXT.md §specifics.

### Pattern 3: Caller identity (dual-policy)

**Source:** `apps-script/src/triggers/showEvictionSidebar.ts:146-153` (audit-log path)
**Per CONTEXT.md §D-06.**

**For audit-log `initiated_by`** (soft fallback to `'unknown'`):
```typescript
let initiatedBy = 'unknown';
try {
  const effective = Session.getEffectiveUser().getEmail();
  if (effective) initiatedBy = effective;
} catch (_e) { /* sandbox quirk */ }
```

**For authorization (`requireAdminOrThrow`)** (fail-closed on empty):
```typescript
const callerEmail = (Session.getEffectiveUser().getEmail() ?? '').toLowerCase().trim();
if (!callerEmail) throw new Error('not_authorized');  // empty == not admin
if (!isAdmin(callerEmail)) throw new Error('not_authorized');
```

### Pattern 4: Structured logging

**Source:** `apps-script/src/lib/log.ts` + every existing trigger/lib file (`migrations.ts:68-70`, `showEvictionSidebar.ts:180-182`).

**Apply to:** every public function in `lib/admin.ts` AND every callback in `showAdminMgmtSidebar.ts`.

```typescript
log('info', '<opName>', { /* fields */ });   // happy-path completions
log('warn', '<opName>', { skipped: '...' }); // expected skips
log('error', '<opName>', { /* error context */ }); // unexpected
```

`opName` = function name (e.g., `'addAdmin'`, `'bootstrapGuildAdmins'`). Fields include `email`, `callerEmail`, action booleans.

### Pattern 5: Theme injection (sidebar mount)

**Source:** `apps-script/src/triggers/showEvictionSidebar.ts:48-57`

**Apply to:** `showAdminMgmtSidebar` opener.

Verbatim copy with title swap to `'SquireBot — Manage admins'`. Width unchanged at 300px.

### Pattern 6: `escapeHtml` defensive interpolation

**Source:** `apps-script/src/triggers/showEvictionSidebar.ts:257`

**Apply to:** every variable interpolation inside `SIDEBAR_BODY`'s inline `<script>` of `showAdminMgmtSidebar.ts`. Threat-register T-05-04-01/02 extended to admin-mgmt per UI-SPEC §A11y.

```javascript
function escapeHtml(s) { const d = document.createElement('div'); d.textContent = String(s == null ? '' : s); return d.innerHTML; }
```

Verbatim. Don't re-implement.

### Pattern 7: Test mocking — Session + seedMeta + lock state

**Source:** `apps-script/src/__tests__/showEvictionSidebar.test.ts:42-47` (Session mock) + `apps-script/src/__tests__/test-helpers.ts:421-431` (resetMocks, seedMeta).

**Apply to:** both new test files. The `installSessionMock` helper should be cloned at the top of each new test file (it's not exported from test-helpers; eviction-sidebar tests defined it locally — admin tests should do the same).

`afterEach(() => { delete (globalThis as Record<string, unknown>).Session; })` at the bottom of each file to prevent leakage.

---

## No Analog Found

Every file in scope has a strong analog. The only "new surface" items in Phase 7 are:

| New surface | Why no analog | Recommendation for executor |
|-------------|---------------|------------------------------|
| `SpreadsheetApp.getUi().alert(title, body, ButtonSet)` invocation | No existing trigger calls `alert()` (only `getUi().showSidebar`/`showModalDialog`). | Use the platform built-in directly per UI-SPEC §Copywriting > Non-admin failure modal. Test-helpers mock may need `alert` capture extension — verify against unread portion of `test-helpers.ts` and extend minimally. |
| `Session.getEffectiveUser().getEmail()` for AUTHORIZATION (vs. audit-log) | All existing uses are audit-log only (eviction `initiated_by`). | Apply Pattern 3 dual-policy. Fail-closed for auth; soft-fallback only for log. |
| Keyboard-Enter listener on text input | No existing sidebar has a text input + Enter-submits pattern. | Implement per UI-SPEC §A11y inline: `addEventListener('keydown', e => { if (e.key === 'Enter') onAdd(); })`. |
| `bootstrapGuildAdmins` called from simple-trigger `onOpen` (must not throw) | All existing `_meta`-touching functions throw on lock-busy or error. | Wrap in try/catch + `log('warn', ...)`. Internal lock-busy returns no-op result (Pattern 1 exception). |
| `_meta.workbook_owner_floor` single-string value (not JSON-array) | All existing structured `_meta` values are JSON-array (`eviction_log`, future `guild_admins`, `admin_log`). Single-string values exist (`theme`, `contact_email`, `schema_version`) but are config primitives, not policy state. | Use `writeMetaRow('_meta', 'workbook_owner_floor', email.toLowerCase().trim())` — single string. No JSON serialize/parse needed for this row. |

---

## Metadata

**Analog search scope:**
- `apps-script/src/triggers/*.ts` (sidebar triggers, simple triggers, time-driven triggers)
- `apps-script/src/lib/*.ts` (sheet-helpers, migrations, themes, log, archive, searchIndex)
- `apps-script/src/__tests__/*.test.ts` (28 test files — focused on showEvictionSidebar.test.ts and charInfoSidebar.test.ts)
- `apps-script/src/Code.ts` (re-export footer convention)

**Files scanned:** ~35 (Read targeted; Glob enumerated `__tests__`).

**Strongest single anchor:** `apps-script/src/triggers/showEvictionSidebar.ts` — the new admin-mgmt sidebar is a near-direct clone of this file with the body content swapped and admin-guard preflight added. Confirmed by CONTEXT.md §canonical_refs ("the eviction sidebar is the reference implementation for the admin-mgmt sidebar").

**Secondary anchors:**
- `apps-script/src/lib/migrations.ts` (LockService envelope idiom + MigrationOutcome-style return shape)
- `apps-script/src/__tests__/showEvictionSidebar.test.ts` (test mocking + Session stub + audit-log JSON assertion patterns)

**Pattern extraction date:** 2026-05-11
