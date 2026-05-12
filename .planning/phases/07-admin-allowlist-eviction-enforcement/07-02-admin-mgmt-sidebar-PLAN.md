---
phase: 07-admin-allowlist-eviction-enforcement
plan: 02
type: execute
wave: 2
depends_on: [07-01]
files_modified:
  - apps-script/src/triggers/showAdminMgmtSidebar.ts
  - apps-script/src/__tests__/adminMgmtSidebar.test.ts
  - apps-script/src/Code.ts
autonomous: true
requirements: [ADMIN-03]
tags: [apps-script, sidebar, htmlservice, ui, audit-log]

must_haves:
  truths:
    - "A new `apps-script/src/triggers/showAdminMgmtSidebar.ts` file exists with 4 exports: `showAdminMgmtSidebar` (opener), `getAdminList` (read callback), `addAdmin` (write callback), `removeAdmin` (write callback)."
    - "Opener calls `isAdmin(callerEmail)` BEFORE building HTML; non-admin → `getUi().alert('Not authorized', 'Only guild officers can manage admins. Contact a workbook admin if you think this is wrong.', ButtonSet.OK)` and returns without opening any sidebar (D-03)."
    - "Sidebar mounts at 300px width, title `'SquireBot — Manage admins'`, theme-aware via the same `themeStyleBlock` + `buildSidebarHtml` triplet cloned verbatim from `showEvictionSidebar.ts:191-217`."
    - "The 3 google.script.run-callable functions (`getAdminList`, `addAdmin`, `removeAdmin`) each call `requireAdminOrThrow(callerEmail)` as their FIRST statement. callerEmail comes from `Session.getEffectiveUser().getEmail()` server-side — never from a client parameter."
    - "`getAdminList()` returns `{ admins: string[]; floor: string; callerEmail: string }` (admins sorted+lowercased; floor is the lowercased single-string `_meta.workbook_owner_floor` value; callerEmail is the normalized effective user) so the client can client-side-suppress the Remove button on the floor row when caller != floor."
    - "`addAdmin(email)` and `removeAdmin(email)` are THIN wrappers — they normalize/validate the email at the sidebar layer if needed, then delegate to `lib/admin.ts` primitives. Owner-floor enforcement happens server-side inside `lib/admin.ts removeAdmin`; the sidebar wrapper does not re-implement it."
    - "Every interpolation into the inline `<script>` of `SIDEBAR_BODY` flows through the inline `escapeHtml(s)` helper cloned verbatim from `showEvictionSidebar.ts:257` (XSS hardening — extends T-05-04-01/02 to admin-mgmt)."
    - "Apps Script `Code.ts` re-exports 5 new global names so Apps Script's menu-by-name resolver finds them: `showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`."
    - "`apps-script/src/__tests__/adminMgmtSidebar.test.ts` exercises: (a) sidebar-opens for admin with correct title+width+body strings, (b) non-admin caller fires `alert(...)` and NO sidebar opens, (c) `getAdminList()` shape includes admins+floor+callerEmail, (d) `addAdmin` wrapper rejects non-admin caller with `not_authorized`, (e) `removeAdmin` wrapper rejects floor-by-non-floor caller with `owner_floor_protected`. Minimum 5 named tests."
    - "All `npm test` suites remain green: the new admin.test.ts suite from Plan 01, the new adminMgmtSidebar.test.ts suite, and the existing 297+ tests."
  artifacts:
    - path: apps-script/src/triggers/showAdminMgmtSidebar.ts
      provides: "HtmlService 300px admin-management sidebar + 3 google.script.run callbacks (getAdminList, addAdmin, removeAdmin), opener with admin guard + non-admin alert modal"
      min_lines: 240
      contains: "showAdminMgmtSidebar"
    - path: apps-script/src/__tests__/adminMgmtSidebar.test.ts
      provides: "Vitest suite covering opener guard, sidebar shape, callback wrappers, server-side enforcement of D-04 owner-floor lockout"
      min_lines: 180
      contains: "describe('showAdminMgmtSidebar'"
    - path: apps-script/src/Code.ts
      provides: "Top-level globals lifted by build.mjs footer so Apps Script menu resolves function names — adds 5 new exports for Phase 7"
      contains: "showAdminMgmtSidebar"
  key_links:
    - from: apps-script/src/triggers/showAdminMgmtSidebar.ts
      to: apps-script/src/lib/admin.ts
      via: "named imports `isAdmin`, `requireAdminOrThrow`, `getAdminList` (rename to avoid name clash with the exported wrapper), `addAdmin` (rename), `removeAdmin` (rename)"
      pattern: "from '\\.\\./lib/admin'"
    - from: apps-script/src/Code.ts
      to: apps-script/src/triggers/showAdminMgmtSidebar.ts
      via: "named import block + re-export block (Apps Script global-name resolver)"
      pattern: "showAdminMgmtSidebar"
    - from: apps-script/src/Code.ts
      to: apps-script/src/lib/admin.ts
      via: "named import for `bootstrapGuildAdminsManual` (the only lib/admin function that needs to be a global because Plan 03's onOpen menu item references it by name string)"
      pattern: "bootstrapGuildAdminsManual"
---

<objective>
Create the new admin-management sidebar (D-04) plus its vitest suite, and wire all five new global names into `Code.ts`. This is one of two Wave-2 plans (parallel with Plan 03); both consume `lib/admin.ts` from Plan 01.

Purpose: give admins a discoverable UI for adding and removing other admins, with owner-floor lockout protection enforced both client-side (Remove button suppressed on floor row when caller != floor) AND server-side (`lib/admin.removeAdmin` throws `'owner_floor_protected'` as defense-in-depth). 300px sidebar matches every other v1.0 sidebar; theme-aware; auditable via `_meta.admin_log` (handled by `lib/admin.ts`); five-minute UX for admins.

Output: 1 new trigger file (~240 lines including the inline `SIDEBAR_BODY` HTML/CSS/JS template), 1 new test file (~180 lines), 1 modified `Code.ts` (5 new globals lifted). After this plan ships AND Plan 03 ships, `clasp push` deploys the admin-mgmt UX to the dev workbook.

ADMIN-03 is fully closed by this plan (the management UX + owner-floor protection). ADMIN-01 sees its `bootstrapGuildAdminsManual` global lift here (the menu wiring lands in Plan 03).
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
@apps-script/src/triggers/showEvictionSidebar.ts
@apps-script/src/Code.ts
@apps-script/src/lib/themes.ts

<interfaces>
<!-- Sidebar callback contracts. The client-side JS in SIDEBAR_BODY is written against these shapes. -->

```typescript
// apps-script/src/triggers/showAdminMgmtSidebar.ts — public exports

/** Opener. Admin-check fail → getUi().alert + return (no sidebar opens). Admin → 300px HtmlService sidebar with title 'SquireBot — Manage admins'. */
export function showAdminMgmtSidebar(): void;

/** google.script.run read callback. Server-side: requireAdminOrThrow(caller) → return list shape. Client uses callerEmail+floor to suppress the Remove button on the floor row. */
export function getAdminList(): { admins: string[]; floor: string; callerEmail: string };

/** google.script.run write callback. Server-side: requireAdminOrThrow(caller) → delegate to lib/admin.addAdmin. Returns the lib result shape. */
export function addAdmin(email: string): { added: boolean; alreadyExists?: boolean };

/** google.script.run write callback. Server-side: requireAdminOrThrow(caller) → delegate to lib/admin.removeAdmin (which enforces owner-floor server-side). Returns the lib result shape. */
export function removeAdmin(email: string): { removed: boolean; notFound?: boolean };
```

NOTE on name collision: `lib/admin.ts` exports `getAdminList`, `addAdmin`, `removeAdmin`. The sidebar trigger file ALSO exports symbols by those exact names (because Apps Script global names = `google.script.run` callback names = top-level export names; CONTEXT.md §canonical_refs spells out the five global names). To avoid TS import collision, the trigger file imports them under aliases:

```typescript
import {
  isAdmin,
  requireAdminOrThrow,
  getAdminList as libGetAdminList,
  addAdmin as libAddAdmin,
  removeAdmin as libRemoveAdmin,
} from '../lib/admin';
```

Then the exported wrappers wrap the aliased lib functions. Tests assert the wrapper shape; the lib's own suite (Plan 01) covers the underlying primitives.
</interfaces>

<copywriting_contract>
<!-- All copy is verbatim from 07-UI-SPEC.md §Copywriting — DO NOT improvise. -->

| Element | Copy |
|---------|------|
| Sidebar title | `SquireBot — Manage admins` |
| Sidebar heading (h3) | `Manage admins` |
| Description | `Manage who can evict guildies. The owner-floor email cannot be removed by anyone else.` |
| List section heading | `Current admins ({N}):` |
| Owner-floor annotation | ` (owner)` (trailing, lowercase, single space before parenthesis) |
| Owner-floor row tooltip | `This is the workbook owner. The owner-floor lockout protection prevents anyone else from removing this email.` |
| Empty-list state | `No admins yet. Click "Initialize Admin Allowlist (manual)" under the SquireBot menu to bootstrap.` |
| Per-row Remove button label | `Remove` |
| Per-row Remove `aria-label` | `Remove admin {email}` (interpolated) |
| Add admin section label | `Add admin` |
| Add admin input placeholder | `email@example.com` |
| Add admin input `aria-label` | `New admin email address` |
| Add admin button label | `Add admin` |
| Non-admin alert title | `Not authorized` |
| Non-admin alert body | `Only guild officers can manage admins. Contact a workbook admin if you think this is wrong.` |
| Per-row Remove confirm | `Remove {email} from the admin allowlist? They will no longer be able to evict members or manage admins. This is reversible by adding them back via this same sidebar.` |

Status region copy (aria-live polite):

| Trigger | Class | Copy |
|---------|-------|------|
| addAdmin success | `success` | `Admin added: {email}.` |
| addAdmin alreadyExists | `success` (informational) | `Already in list: {email}.` |
| addAdmin validation reject | `error` | `Invalid email: {detail}. No changes were written.` |
| removeAdmin success | `success` | `Admin removed: {email}.` |
| removeAdmin notFound | `success` (informational) | `Not found in list: {email}.` |
| removeAdmin owner_floor_protected | `error` | `Owner-floor protected — only the workbook owner can remove themselves. No changes were written.` |
| not_authorized (stale-sidebar replay) | `error` | `Not authorized — you are no longer an admin. Please close this sidebar.` |
| Generic failure | `error` | `Action failed: {detail}. No changes were written.` |
</copywriting_contract>

<remove_button_styling>
<!-- 07-UI-SPEC.md §Color > "Remove button styling" — intentional contract diff from eviction sidebar. -->

```css
.remove-btn {
  background: transparent;
  color: var(--fg);
  opacity: 0.7;
  border: 1px solid var(--bg);
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 3px;
  min-height: 24px;
  cursor: pointer;
  font-family: inherit;
}
.remove-btn:hover {
  opacity: 1;
  border-color: var(--accent-bg);
}
```

Why not destructive-red: removal is reversible (re-add in 5s). Red would over-cue danger and dilute the eviction sidebar's actual destructive palette.
</remove_button_styling>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create `showAdminMgmtSidebar.ts` (opener + 3 callbacks + inline SIDEBAR_BODY)</name>
  <files>apps-script/src/triggers/showAdminMgmtSidebar.ts</files>
  <read_first>
    - apps-script/src/triggers/showEvictionSidebar.ts (THE template — clone the opener at lines 48-57, the themeStyleBlock+buildSidebarHtml triplet at 191-217, the SIDEBAR_BODY String.raw template at 228-323, and the escapeHtml helper at 257)
    - apps-script/src/lib/admin.ts (just landed in Plan 01 — confirm the 9 exports are importable; in particular `getAdminList`, `addAdmin`, `removeAdmin`, `isAdmin`, `requireAdminOrThrow`)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-UI-SPEC.md §Spacing, §Typography, §Color, §Copywriting, §Interaction Contract, §Accessibility Baseline (this is the design contract for the new sidebar — strict)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md §`apps-script/src/triggers/showAdminMgmtSidebar.ts` (the implementation pattern map — opener guard insertion, themeStyleBlock+buildSidebarHtml verbatim clones, SIDEBAR_BODY shape diff, server-side callback skeleton, alias-imports)
    - apps-script/src/lib/themes.ts (Theme interface + THEMES registry — the theme injection is verbatim from eviction sidebar)
  </read_first>
  <behavior>
    - Opener calls `isAdmin` before building HTML. Non-admin → `getUi().alert(...)` + return. Admin → 300px sidebar opens with verbatim title and `themeStyleBlock`+`fallbackVars`+`SIDEBAR_BODY` shape.
    - The `escapeHtml` JS helper at the top of `<script>` is verbatim from `showEvictionSidebar.ts:257`.
    - Inline `<style>` block clones the eviction sidebar's spacing tokens, button.primary styling, #msg.error/#msg.success colors, and ADDS the `.remove-btn` rule per 07-UI-SPEC §Color (transparent bg, opacity 0.7, 11px, 24px min-height, var(--bg) border, hover state goes to opacity 1 + accent border).
    - Client-side `init()`:
      1. `google.script.run.withSuccessHandler(renderList).withFailureHandler(showErr).getAdminList()` on load.
      2. Wires the `addBtn` click handler and the `addInput` keydown-Enter handler (07-UI-SPEC §A11y).
      3. After successful list render, focuses `#addInput` (07-UI-SPEC §A11y).
    - `renderList(payload)`:
      - Reads `payload.admins`, `payload.floor`, `payload.callerEmail`.
      - Renders `Current admins (${admins.length}):` heading.
      - For each admin, renders an `<li>`. If `admin === floor`, append ` (owner)` annotation + set `title=` tooltip on the `<li>`. The `[Remove]` button is rendered on EVERY row EXCEPT the floor row when `payload.callerEmail !== payload.floor`.
      - If `admins.length === 0`, renders the empty-list copy from the contract.
      - Each Remove button gets `aria-label="Remove admin {email}"` (escaped).
    - `onAdd()`:
      - Disables `addBtn`.
      - Calls `google.script.run.addAdmin(value)`.
      - Success: clears the input, re-fetches list, writes status message per the contract.
      - Failure: writes error status per the contract; the error message routing maps server-side error.message to the right copy line (e.g., `/owner_floor_protected/` → "Owner-floor protected — ...", `/not_authorized/` → "Not authorized — you are no longer an admin.", `/invalid_email/` → "Invalid email: ..."; default → "Action failed: {detail}.").
      - Re-enables `addBtn`.
    - `onRemove(email)`:
      - `window.confirm(...)` with the verbatim copy from the contract. Cancel = no-op.
      - OK → `google.script.run.removeAdmin(email)` → success/error routing as above.
    - Server-side callback wrappers each:
      1. Compute `callerEmail` from `Session.getEffectiveUser().getEmail()` and normalize (lowercase+trim).
      2. Call `requireAdminOrThrow(callerEmail)` — first statement, before any validation or work.
      3. Delegate to `lib/admin.ts` primitive (`libGetAdminList`, `libAddAdmin`, `libRemoveAdmin`).
      4. For `getAdminList`: spread the lib result + `callerEmail` into the returned object.
      5. For `addAdmin`/`removeAdmin`: pass `callerEmail` as the second arg to the lib function so it can enforce caller-aware policy (owner-floor).
    - One `log('info', '<funcName>', { ... })` call per callback (structured logging convention).
  </behavior>
  <action>
Create `apps-script/src/triggers/showAdminMgmtSidebar.ts`. The file structure should mirror `showEvictionSidebar.ts` exactly:

```typescript
// showAdminMgmtSidebar — Phase 7 plan 07-02.
//
// HtmlService sidebar (ADMIN-03 implementation) for admin-allowlist
// management. 300px wide per UI-SPEC §Spacing; theme-aware via the same
// CSS-custom-property injection pattern landed in 05-03's
// showSearchSidebar; inline SIDEBAR_BODY is the SINGLE SOURCE OF TRUTH
// for the HTML/CSS/JS body (Option A — no companion .html file).
//
// Server-side public surface (1 opener + 3 google.script.run callbacks):
//   - showAdminMgmtSidebar()      — opens the panel (admin-gated; non-admin → alert + return)
//   - getAdminList()              — { admins[], floor, callerEmail }
//   - addAdmin(email)             — { added, alreadyExists? } delegates to lib/admin
//   - removeAdmin(email)          — { removed, notFound? } delegates to lib/admin
//                                   (lib enforces owner-floor lockout server-side)
//
// Security: every interpolation into the SIDEBAR_BODY inline <script>
// flows through the inline escapeHtml() helper cloned verbatim from
// showEvictionSidebar.ts:257. T-07-02-* in the threat register below.
//
// Owner-floor lockout is enforced at TWO layers:
//   1. Client-side: Remove button suppressed on floor row when caller != floor
//   2. Server-side: lib/admin.removeAdmin throws 'owner_floor_protected'
// The server-side check is the security boundary; the client-side check
// is the UX (prevents confusing error toasts for the common case).
//
// Audit trail: every addAdmin/removeAdmin appends an entry to
// _meta.admin_log via lib/admin.appendAdminLogEntry (called inside the
// lib primitive's lock envelope).

import { log } from '../lib/log';
import { getActiveTheme, THEMES, type Theme } from '../lib/themes';
import {
  isAdmin,
  requireAdminOrThrow,
  normalizeEmail,
  getAdminList as libGetAdminList,
  addAdmin as libAddAdmin,
  removeAdmin as libRemoveAdmin,
} from '../lib/admin';

// --- Opener -------------------------------------------------------------

export function showAdminMgmtSidebar(): void {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* sandbox quirk — empty string fail-closes below */ }

  if (!isAdmin(callerEmail)) {
    SpreadsheetApp.getUi().alert(
      'Not authorized',
      'Only guild officers can manage admins. Contact a workbook admin if you think this is wrong.',
      SpreadsheetApp.getUi().ButtonSet.OK,
    );
    log('warn', 'showAdminMgmtSidebar', { notAuthorized: true, callerEmail });
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

// --- Read callback ------------------------------------------------------

export function getAdminList(): { admins: string[]; floor: string; callerEmail: string } {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* fail-closed below */ }
  requireAdminOrThrow(callerEmail);
  const { admins, floor } = libGetAdminList();
  log('info', 'getAdminList', { callerEmail, count: admins.length });
  return { admins, floor, callerEmail };
}

// --- Write callbacks ----------------------------------------------------

export function addAdmin(email: string): { added: boolean; alreadyExists?: boolean } {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* fail-closed below */ }
  requireAdminOrThrow(callerEmail);
  return libAddAdmin(String(email ?? ''), callerEmail);
}

export function removeAdmin(email: string): { removed: boolean; notFound?: boolean } {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* fail-closed below */ }
  requireAdminOrThrow(callerEmail);
  return libRemoveAdmin(String(email ?? ''), callerEmail);
}

// --- HTML rendering -----------------------------------------------------

function themeStyleBlock(theme: Theme | null): string {
  // CLONE VERBATIM from showEvictionSidebar.ts:191-204
  // (… exact same body — copy the implementation byte-for-byte from the eviction sidebar)
}

function buildSidebarHtml(theme: Theme | null): string {
  // CLONE VERBATIM from showEvictionSidebar.ts:206-217
  const tokens = themeStyleBlock(theme);
  const fallbackVars = !theme
    ? '<style>:root { --space-xs:4px; --space-sm:8px; --space-md:12px; --space-lg:16px; --space-xl:24px; --bg:#f8f9fa; --bg-row:#fff; --fg:#222; --fg-header:#222; --accent-bg:#1a73e8; --accent-fg:#fff; --font-header:Arial,sans-serif; --font-body:Arial,sans-serif; }</style>'
    : '';
  return tokens + fallbackVars + SIDEBAR_BODY;
}

const SIDEBAR_BODY = String.raw`
<style>
  body { margin: 0; padding: 0; background: var(--bg-row); color: var(--fg); font-family: var(--font-body); font-size: 13px; line-height: 1.5; }
  .sidebar { padding: 12px var(--space-lg) var(--space-lg); }
  h3 { margin: 0 0 var(--space-xs) 0; font-family: var(--font-header); font-size: 16px; font-weight: 600; color: var(--fg-header); line-height: 1.3; }
  .desc { margin: 0 0 var(--space-md) 0; font-size: 11px; line-height: 1.4; opacity: 0.8; }
  label { display: block; margin-top: var(--space-sm); font-size: 11px; line-height: 1.4; }
  input[type="text"] { width: 100%; box-sizing: border-box; padding: var(--space-sm); margin-top: var(--space-xs); font-size: 13px; font-family: inherit; background: var(--bg-row); color: var(--fg); border: 1px solid var(--bg); border-radius: 3px; }
  input[type="text"]:focus { outline: 2px solid var(--accent-bg); outline-offset: -1px; }
  ul#adminList { list-style: none; padding-left: var(--space-lg); margin: var(--space-sm) 0 var(--space-xl) 0; }
  ul#adminList li { display: flex; justify-content: space-between; align-items: center; gap: var(--space-sm); padding: var(--space-xs) 0; }
  .remove-btn { background: transparent; color: var(--fg); opacity: 0.7; border: 1px solid var(--bg); font-size: 11px; padding: 4px 8px; border-radius: 3px; min-height: 24px; cursor: pointer; font-family: inherit; }
  .remove-btn:hover { opacity: 1; border-color: var(--accent-bg); }
  button.primary { display: block; width: 100%; margin-top: var(--space-lg); min-height: 32px; padding: var(--space-sm); background: var(--accent-bg); color: var(--accent-fg); border: none; border-radius: 3px; font-size: 13px; font-family: inherit; cursor: pointer; }
  button.primary:disabled { opacity: 0.5; cursor: wait; }
  #msg { margin-top: var(--space-md); font-size: 13px; min-height: 1.4em; }
  #msg.error { color: #c00; }
  #msg.success { color: #060; }
</style>
<div class="sidebar">
  <h3>Manage admins</h3>
  <p class="desc">Manage who can evict guildies. The owner-floor email cannot be removed by anyone else.</p>
  <div id="listRegion">
    <label id="listHeading">Current admins:</label>
    <ul id="adminList"></ul>
  </div>
  <label for="addInput">Add admin</label>
  <input id="addInput" type="text" placeholder="email@example.com" aria-label="New admin email address" autocomplete="off" />
  <button id="addBtn" class="primary">Add admin</button>
  <div id="msg" aria-live="polite"></div>
</div>
<script>
  function escapeHtml(s) { const d = document.createElement('div'); d.textContent = String(s == null ? '' : s); return d.innerHTML; }
  let state = { admins: [], floor: '', callerEmail: '' };
  function setMsg(text, cls) { const m = document.getElementById('msg'); m.textContent = text; m.className = cls || ''; }
  function renderList(payload) {
    state = payload || { admins: [], floor: '', callerEmail: '' };
    const heading = document.getElementById('listHeading');
    heading.textContent = 'Current admins (' + state.admins.length + '):';
    const ul = document.getElementById('adminList');
    if (state.admins.length === 0) {
      ul.innerHTML = '<li style="font-size:11px;opacity:0.7;justify-content:flex-start">No admins yet. Click "Initialize Admin Allowlist (manual)" under the SquireBot menu to bootstrap.</li>';
    } else {
      ul.innerHTML = state.admins.map(function(email) {
        const isFloor = (email === state.floor);
        const showRemove = !isFloor || (state.callerEmail === state.floor);
        const annotation = isFloor ? ' (owner)' : '';
        const tooltip = isFloor ? ' title="This is the workbook owner. The owner-floor lockout protection prevents anyone else from removing this email."' : '';
        const btn = showRemove
          ? '<button class="remove-btn" aria-label="Remove admin ' + escapeHtml(email) + '" data-email="' + escapeHtml(email) + '">Remove</button>'
          : '';
        return '<li' + tooltip + '><span>' + escapeHtml(email) + escapeHtml(annotation) + '</span>' + btn + '</li>';
      }).join('');
      Array.prototype.forEach.call(ul.querySelectorAll('.remove-btn'), function(btn) {
        btn.addEventListener('click', function() { onRemove(btn.getAttribute('data-email')); });
      });
    }
    document.getElementById('addInput').focus();
  }
  function routeError(err) {
    const m = (err && err.message) ? String(err.message) : String(err || '');
    if (/owner_floor_protected/.test(m)) setMsg('Owner-floor protected — only the workbook owner can remove themselves. No changes were written.', 'error');
    else if (/not_authorized/.test(m)) setMsg('Not authorized — you are no longer an admin. Please close this sidebar.', 'error');
    else if (/invalid_email/.test(m)) setMsg('Invalid email: ' + m + '. No changes were written.', 'error');
    else if (/lock_busy/.test(m)) setMsg('Action failed: another admin action is in flight. Please retry. No changes were written.', 'error');
    else setMsg('Action failed: ' + m + '. No changes were written.', 'error');
  }
  function onAdd() {
    const input = document.getElementById('addInput');
    const value = String(input.value || '').trim();
    if (!value) { setMsg('Invalid email: empty. No changes were written.', 'error'); return; }
    const btn = document.getElementById('addBtn');
    btn.disabled = true;
    google.script.run
      .withSuccessHandler(function(result) {
        btn.disabled = false;
        input.value = '';
        if (result && result.alreadyExists) setMsg('Already in list: ' + value + '.', 'success');
        else setMsg('Admin added: ' + value + '.', 'success');
        google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();
      })
      .withFailureHandler(function(err) { btn.disabled = false; routeError(err); })
      .addAdmin(value);
  }
  function onRemove(email) {
    if (!window.confirm('Remove ' + email + ' from the admin allowlist? They will no longer be able to evict members or manage admins. This is reversible by adding them back via this same sidebar.')) return;
    google.script.run
      .withSuccessHandler(function(result) {
        if (result && result.notFound) setMsg('Not found in list: ' + email + '.', 'success');
        else setMsg('Admin removed: ' + email + '.', 'success');
        google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();
      })
      .withFailureHandler(routeError)
      .removeAdmin(email);
  }
  function init() {
    document.getElementById('addBtn').addEventListener('click', onAdd);
    document.getElementById('addInput').addEventListener('keydown', function(e) { if (e.key === 'Enter') onAdd(); });
    google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
</script>
`;
```

CRITICAL execution notes:
- The two helpers `themeStyleBlock` and `buildSidebarHtml` are CLONED VERBATIM from `showEvictionSidebar.ts:191-217`. The pseudo-comment `// CLONE VERBATIM from …` in the sketch above is shorthand: in the actual file you write, both function bodies must contain the exact code from the eviction sidebar (same `:root` token map, same six theme tokens, same fallback inline style for sheets-default). Read those lines and paste them in.
- `normalizeEmail` is imported from `lib/admin` — do not re-implement.
- The `SIDEBAR_BODY` String.raw template above is complete and self-contained; no further work needed except verifying that the `escapeHtml` helper inside the inline `<script>` is the exact same line as `showEvictionSidebar.ts:257` (it should be).
- DO NOT use the dollar-curly placeholder syntax inside `String.raw` template for runtime values — all dynamic values are rendered client-side by JS reading from `state`. The String.raw is used only to safely include `${...}` literal text in CSS/JS if any (here there are none; String.raw is still used for forward consistency with the eviction sidebar's exact pattern).

After writing, run typecheck:
```bash
cd apps-script && npx tsc --noEmit
```
Must exit 0.
  </action>
  <verify>
    <automated>
      cd apps-script && npx tsc --noEmit 2>&1 | tee /tmp/admin-mgmt-typecheck.log; if [ -s /tmp/admin-mgmt-typecheck.log ] && grep -q "error TS" /tmp/admin-mgmt-typecheck.log; then echo TYPECHECK FAIL; exit 1; else echo TYPECHECK OK; fi
    </automated>
  </verify>
  <acceptance_criteria>
    - `Test-Path apps-script/src/triggers/showAdminMgmtSidebar.ts` returns True.
    - `(Get-Content apps-script/src/triggers/showAdminMgmtSidebar.ts).Count` is >= 220.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "^export function showAdminMgmtSidebar\(\): void"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "^export function getAdminList\(\)"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "^export function addAdmin\("` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "^export function removeAdmin\("` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "SquireBot — Manage admins" -SimpleMatch` matches exactly 1 line (title).
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "Only guild officers can manage admins" -SimpleMatch` matches exactly 1 line (non-admin alert copy verbatim).
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "\.setWidth\(300\)"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "requireAdminOrThrow\(callerEmail\)"` matches >= 3 lines (one per callback: getAdminList, addAdmin, removeAdmin).
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "escapeHtml" -SimpleMatch` matches >= 2 lines (the helper definition AND uses inside the script template; verified via String.raw template body).
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "remove-btn" -SimpleMatch` matches >= 3 lines (CSS rule + the data-email rendering + the querySelectorAll wire-up).
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "from '\\.\\./lib/admin'"` matches exactly 1 line.
    - `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "schema_version" -SimpleMatch` matches ZERO lines (verification hook 5).
    - `cd apps-script; npx tsc --noEmit 2>&1` exits 0.
    - `cd apps-script; npm run build 2>&1` exits 0 (esbuild bundle still produces `dist/Code.js`).
  </acceptance_criteria>
  <done>
    Trigger file exists with opener + 3 callbacks. Opener admin-gates BEFORE building HTML. Callbacks delegate to lib/admin via alias imports. Inline SIDEBAR_BODY contains the locked copy verbatim and the .remove-btn styling per 07-UI-SPEC §Color. Typecheck and build pass. Tests come in Task 2.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add Code.ts re-exports for the 5 new globals + write adminMgmtSidebar.test.ts</name>
  <files>apps-script/src/Code.ts, apps-script/src/__tests__/adminMgmtSidebar.test.ts</files>
  <read_first>
    - apps-script/src/Code.ts (the current 71-line file — patterns at lines 44-49 for showEvictionSidebar are the closest analog for the import block; lines 56-71 for the export block)
    - apps-script/src/__tests__/showEvictionSidebar.test.ts (the template for sidebar-opens tests — particularly the captured-sidebar assertion at lines 57-68 and the Session mock at 42-47)
    - apps-script/src/__tests__/test-helpers.ts (the `lastSidebar` capture surface — confirm via grep that the mock captures `_html`, `_title`, `_width` from `HtmlOutput`)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md §`apps-script/src/__tests__/adminMgmtSidebar.test.ts` (the test surface map)
    - .planning/phases/07-admin-allowlist-eviction-enforcement/07-01-admin-policy-module-PLAN.md (the lib/admin.ts contract this sidebar consumes; in particular `requireAdminOrThrow` throws `not_authorized`)
  </read_first>
  <behavior>
    **Code.ts edits:** add an import block for the 4 sidebar exports + `bootstrapGuildAdminsManual`, and append all 5 to the existing `export { … }` block. Preserve the existing block formatting (multi-line, comma-grouped per source file). 5 new global names: `showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`. NO other Code.ts changes (existing imports and exports are untouched).

    NAME-COLLISION RISK: Code.ts already imports `getEvictionEmails` etc. from `showEvictionSidebar`. The new sidebar exports `getAdminList`, `addAdmin`, `removeAdmin` — none of these names clash with existing Code.ts globals (verified: no existing `getAdminList` / `addAdmin` / `removeAdmin` export). So Code.ts can import them under their original names — NO aliasing needed at the Code.ts level.

    **Test file:** minimum 5 named tests:

    - **TS1** `showAdminMgmtSidebar_admin_opensSidebarWithLockedShape` — seed `_meta.guild_admins=["admin@example.com"]`, floor=`admin@example.com`; install Session mock as `admin@example.com`. Call `showAdminMgmtSidebar()`. Assert: `state.lastSidebar` defined, `_width === 300`, `_title === 'SquireBot — Manage admins'`, `_html` contains `'Manage admins'`, `'Add admin'`, `'email@example.com'` (placeholder).
    - **TS2** `showAdminMgmtSidebar_nonAdmin_firesAlertAndDoesNotOpenSidebar` — seed admin list with `admin@example.com`; install Session mock as `intruder@example.com`. Call `showAdminMgmtSidebar()`. Assert: a `getUi().alert(...)` was invoked with title `'Not authorized'` AND body containing `'Only guild officers can manage admins'`; `state.lastSidebar` is undefined (no sidebar opened).
    - **TS3** `getAdminList_returnsAdminsFloorAndCallerEmail` — seed admin list `["alice@x.com","bob@x.com","jbowen@x.com"]`, floor=`jbowen@x.com`; install Session as `bob@x.com`. Call `getAdminList()`. Assert: returns `{ admins: ['alice@x.com','bob@x.com','jbowen@x.com'], floor: 'jbowen@x.com', callerEmail: 'bob@x.com' }`. Sorted lowercased.
    - **TS4** `addAdmin_nonAdminCaller_throwsNotAuthorized` — seed admin list `["admin@x.com"]`; install Session as `intruder@x.com`. Call `addAdmin('newadmin@x.com')`. Assert: throws `/not_authorized/`. Assert: NO `_meta.guild_admins` write happened (still `["admin@x.com"]`); NO `_meta.admin_log` entry.
    - **TS5** `removeAdmin_floorByNonFloorAdmin_throwsOwnerFloorProtected` — seed admin list `["bob@x.com","jbowen@x.com"]`, floor=`jbowen@x.com`; install Session as `bob@x.com`. Call `removeAdmin('jbowen@x.com')`. Assert: throws `/owner_floor_protected/`. Assert: NO writes (guild_admins still has both; admin_log empty or unchanged).

    Optional 6th test (RECOMMENDED if time permits):
    - **TS6** `removeAdmin_floorByFloor_selfRemovalSucceedsAndPreservesFloorRow` — seed admin list `["bob@x.com","jbowen@x.com"]`, floor=`jbowen@x.com`; install Session as `jbowen@x.com`. Call `removeAdmin('jbowen@x.com')`. Assert: returns `{ removed: true }`; `guild_admins` now `["bob@x.com"]`; `workbook_owner_floor` STILL `jbowen@x.com` (orphan pointer per D-04 documented behavior).

    For TS2 the test needs to capture `SpreadsheetApp.getUi().alert(...)` calls. Check `test-helpers.ts` for an existing alert-capture surface. If NONE exists (the existing eviction-sidebar tests don't exercise alert), extend `test-helpers.ts` MINIMALLY:
    - Add a `state.alertCalls: Array<{ title: string; body: string; buttonSet: unknown }>` array.
    - In the SpreadsheetApp.getUi() mock factory, route `alert(title, body, buttonSet)` calls into this array.
    - This is a one-time, additive extension; it does not break any existing test.
    - Surface the diff in the Plan 02 SUMMARY.

    The test file mirrors `showEvictionSidebar.test.ts` in structure (top-level describe; resetMocks/seedMeta beforeEach; Session mock helper at top; afterEach cleanup).
  </behavior>
  <action>
**Step A — modify `apps-script/src/Code.ts`:**

Insert a new import block after the existing `showEvictionSidebar` import block at lines 44-49 (around line 50, before the `prewarmSearchCache` import):

```typescript
import {
  showAdminMgmtSidebar,
  getAdminList,
  addAdmin,
  removeAdmin,
} from './triggers/showAdminMgmtSidebar';
import { bootstrapGuildAdminsManual } from './lib/admin';
```

Append to the existing `export { ... }` block (after the `showEvictionSidebar, getEvictionEmails, previewEviction, commitEviction,` line at line 67):

```typescript
  showAdminMgmtSidebar, getAdminList, addAdmin, removeAdmin,
  bootstrapGuildAdminsManual,
```

Verify after editing:
```bash
cd apps-script && npx tsc --noEmit
cd apps-script && npm run build
```
Both must exit 0. `dist/Code.js` should now have the 5 new names lifted to top-level globals (this is verifiable by `grep -E "globalThis\\.(showAdminMgmtSidebar|getAdminList|addAdmin|removeAdmin|bootstrapGuildAdminsManual) =" apps-script/dist/Code.js` after build).

**Step B — write `apps-script/src/__tests__/adminMgmtSidebar.test.ts`:**

Follow the structure of `apps-script/src/__tests__/showEvictionSidebar.test.ts`. Use the same `installSessionMock` helper pattern at the top of the file (copy lines 42-47 of the eviction-sidebar test file verbatim with the same afterEach cleanup at the bottom).

The 5 tests above (TS1–TS5) plus optional TS6. Each test follows this shape:

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  resetMocks, makeSheet, seedMeta, type MockState,
} from './test-helpers';
import {
  showAdminMgmtSidebar,
  getAdminList,
  addAdmin,
  removeAdmin,
} from '../triggers/showAdminMgmtSidebar';

function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

describe('showAdminMgmtSidebar', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    makeSheet(state, '_meta');
  });
  afterEach(() => {
    delete (globalThis as Record<string, unknown>).Session;
  });

  it('showAdminMgmtSidebar_admin_opensSidebarWithLockedShape', () => {
    installSessionMock('admin@example.com');
    seedMeta(state, [
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
    expect(captured!._html).toContain('email@example.com');
  });

  it('showAdminMgmtSidebar_nonAdmin_firesAlertAndDoesNotOpenSidebar', () => {
    installSessionMock('intruder@example.com');
    seedMeta(state, [
      ['guild_admins', JSON.stringify(['admin@example.com'])],
      ['workbook_owner_floor', 'admin@example.com'],
    ]);
    showAdminMgmtSidebar();
    const stateWithAlerts = state as MockState & { alertCalls?: Array<{ title: string; body: string }>; lastSidebar?: unknown };
    const alerts = stateWithAlerts.alertCalls ?? [];
    expect(alerts.length).toBeGreaterThanOrEqual(1);
    expect(alerts[0].title).toBe('Not authorized');
    expect(alerts[0].body).toContain('Only guild officers can manage admins');
    expect(stateWithAlerts.lastSidebar).toBeUndefined();
  });

  it('getAdminList_returnsAdminsFloorAndCallerEmail', () => {
    installSessionMock('bob@x.com');
    seedMeta(state, [
      ['guild_admins', JSON.stringify(['alice@x.com', 'bob@x.com', 'jbowen@x.com'])],
      ['workbook_owner_floor', 'jbowen@x.com'],
    ]);
    const result = getAdminList();
    expect(result).toEqual({
      admins: ['alice@x.com', 'bob@x.com', 'jbowen@x.com'],
      floor: 'jbowen@x.com',
      callerEmail: 'bob@x.com',
    });
  });

  it('addAdmin_nonAdminCaller_throwsNotAuthorized', () => {
    installSessionMock('intruder@x.com');
    seedMeta(state, [
      ['guild_admins', JSON.stringify(['admin@x.com'])],
      ['workbook_owner_floor', 'admin@x.com'],
    ]);
    expect(() => addAdmin('newadmin@x.com')).toThrowError(/not_authorized/);
    const meta = state.sheets.get('_meta')!;
    const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
    expect(adminsRow[1]).toBe(JSON.stringify(['admin@x.com']));
    const logRow = meta.values.find((r) => r[0] === 'admin_log');
    expect(logRow === undefined || logRow[1] === '' || logRow[1] === '[]').toBe(true);
  });

  it('removeAdmin_floorByNonFloorAdmin_throwsOwnerFloorProtected', () => {
    installSessionMock('bob@x.com');
    seedMeta(state, [
      ['guild_admins', JSON.stringify(['bob@x.com', 'jbowen@x.com'])],
      ['workbook_owner_floor', 'jbowen@x.com'],
    ]);
    expect(() => removeAdmin('jbowen@x.com')).toThrowError(/owner_floor_protected/);
    const meta = state.sheets.get('_meta')!;
    const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
    expect(JSON.parse(String(adminsRow[1]))).toEqual(['bob@x.com', 'jbowen@x.com']);
  });

  it('removeAdmin_floorByFloor_selfRemovalSucceedsAndPreservesFloorRow', () => {
    installSessionMock('jbowen@x.com');
    seedMeta(state, [
      ['guild_admins', JSON.stringify(['bob@x.com', 'jbowen@x.com'])],
      ['workbook_owner_floor', 'jbowen@x.com'],
    ]);
    const result = removeAdmin('jbowen@x.com');
    expect(result).toEqual({ removed: true });
    const meta = state.sheets.get('_meta')!;
    const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
    expect(JSON.parse(String(adminsRow[1]))).toEqual(['bob@x.com']);
    const floorRow = meta.values.find((r) => r[0] === 'workbook_owner_floor')!;
    expect(floorRow[1]).toBe('jbowen@x.com'); // orphan pointer per D-04
  });
});
```

**Step C — extend `test-helpers.ts` minimally for alert capture (if needed):**

Check first whether `state.alertCalls` or an equivalent already exists. If yes, use it. If no, add the minimal extension:

1. In the `MockState` type, add `alertCalls: Array<{ title: string; body: string; buttonSet: unknown }>;`.
2. In `resetMocks`, initialize `alertCalls: []`.
3. In the `SpreadsheetApp.getUi()` mock factory, ensure `alert(title, body, buttonSet)` pushes onto `state.alertCalls` and returns a sentinel (e.g., `'OK'`).
4. Run the full test suite to confirm no existing test broke (`npm test`).

This extension is minimal and additive. Document the diff in the SUMMARY.

**Step D — run full suite:**
```bash
cd apps-script && npm test
```
All three suites must pass: existing 297+ tests, the admin.test.ts suite (20 tests from Plan 01), and the new adminMgmtSidebar.test.ts suite (5–6 tests).
  </action>
  <verify>
    <automated>
      cd apps-script && npm test 2>&1 | tee /tmp/admin-mgmt-test.log; if grep -qE "(adminMgmtSidebar.*FAIL|FAIL.*adminMgmtSidebar)" /tmp/admin-mgmt-test.log; then echo TEST FAIL; exit 1; else grep -c "PASS\|✓" /tmp/admin-mgmt-test.log; fi
    </automated>
  </verify>
  <acceptance_criteria>
    - `Test-Path apps-script/src/__tests__/adminMgmtSidebar.test.ts` returns True.
    - `(Get-Content apps-script/src/__tests__/adminMgmtSidebar.test.ts | Where-Object { $_ -match "  it\\(" }).Count` is >= 5.
    - `Select-String -Path apps-script/src/__tests__/adminMgmtSidebar.test.ts -Pattern "showAdminMgmtSidebar_nonAdmin_firesAlert" -SimpleMatch` matches 1 line (TS2 — the alert assertion test).
    - `Select-String -Path apps-script/src/__tests__/adminMgmtSidebar.test.ts -Pattern "owner_floor_protected" -SimpleMatch` matches >= 1 line.
    - `Select-String -Path apps-script/src/Code.ts -Pattern "showAdminMgmtSidebar"` matches exactly 2 lines (1 in the import block, 1 in the export block).
    - `Select-String -Path apps-script/src/Code.ts -Pattern "bootstrapGuildAdminsManual"` matches exactly 2 lines (import + export).
    - `Select-String -Path apps-script/src/Code.ts -Pattern "from './triggers/showAdminMgmtSidebar'"` matches exactly 1 line.
    - `cd apps-script; npx tsc --noEmit 2>&1` exits 0.
    - `cd apps-script; npm run build 2>&1` exits 0; `dist/Code.js` contains `showAdminMgmtSidebar` as a top-level global symbol.
    - `cd apps-script; npm test 2>&1` exits 0 with at least 5 NEW PASS markers for adminMgmtSidebar tests (in addition to admin.test.ts's 20).
    - `Select-String -Path apps-script/src/lib/migrations.ts -Pattern "writeMetaRow\('_meta', 'schema_version', '3'\)"` still matches its existing 1 line (verification hook 5: no schema_version bump introduced by this plan).
    - `Select-String -Path internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion\s*=\s*3"` matches exactly 1 line (verification hook 5).
  </acceptance_criteria>
  <done>
    Code.ts re-exports all 5 new globals. The new test file passes 5+ tests covering opener admin gate, non-admin alert flow, getAdminList shape, addAdmin requireAdminOrThrow boundary, and removeAdmin owner-floor server-side enforcement. Full apps-script test suite is green. Verification hooks 3 (admin add) and 4 (owner-floor server-side) gain integration-level coverage at the sidebar layer (their policy-level coverage is in admin.test.ts T7/T11/T12).
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `google.script.run` callback ↔ server-side function | Client sidebar can invoke `addAdmin('attacker@x.com')` from devtools. Server MUST re-validate caller identity from `Session.getEffectiveUser()` — never trust a client-supplied caller argument. |
| HtmlService inline `<script>` interpolation | Every admin email rendered into the DOM has to flow through `escapeHtml` — same XSS hardening as eviction sidebar. |
| Owner-floor client-side suppression vs. server-side enforcement | Client-side suppression is UX; server-side enforcement is the security boundary. A stale sidebar where the caller was just demoted could submit a removeAdmin(floor) request — server-side `lib/admin.removeAdmin` MUST reject it. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-07-02-01 | Spoofing | `google.script.run.addAdmin(email)` invoked from devtools by a non-admin user | mitigate | Every callback's FIRST statement is `requireAdminOrThrow(callerEmail)` where `callerEmail = Session.getEffectiveUser().getEmail()` — server-side identity, never client-supplied. Tested by TS4 (addAdmin throws not_authorized for non-admin caller). |
| T-07-02-02 | Tampering | Stale-sidebar attack: admin opens sidebar, gets demoted, then submits a callback before reload | mitigate | Server-side `requireAdminOrThrow` re-reads `_meta.guild_admins` on every call (no caching). The demoted caller fails the check; client-side `routeError` displays "Not authorized — you are no longer an admin." |
| T-07-02-03 | Tampering | XSS via attacker-controlled email injected into the rendered admin list | mitigate | All email interpolations into the inline `<script>` flow through `escapeHtml` (cloned verbatim from showEvictionSidebar.ts:257). T-05-04-01/02 mitigation extended. Grep gate asserts >= 2 escapeHtml references in the trigger file. |
| T-07-02-04 | Elevation of Privilege | Owner-floor client-side suppression bypassed via devtools (attacker reveals hidden Remove button and clicks) | mitigate | Server-side `lib/admin.removeAdmin` throws `'owner_floor_protected'` when `target === floor && caller !== floor`. Tested by TS5 + admin.test.ts T12 (defense in depth across two test files). |
| T-07-02-05 | Information Disclosure | Sidebar exposes the full admin list (including emails) to anyone authorized to open it | accept | The full admin list is information admins need to do their job. Non-admins cannot open the sidebar (D-03 opener guard, alert-only). Emails are not PII beyond what's already in `_char_owner.owner_email`. |
| T-07-02-06 | Denial of Service | Repeated `addAdmin` / `removeAdmin` clicks from the sidebar exhaust the document lock | mitigate | Add button disables (`btn.disabled = true`) during in-flight calls; routeError re-enables it. Lock-busy returns a clear error message. The sidebar is single-user (one admin at a time) so contention is rare. |
| T-07-02-07 | Repudiation | Admin claims they didn't perform an action recorded in `_meta.admin_log` | mitigate | The log entry's `initiated_by` is server-side `Session.getEffectiveUser().getEmail()` — same identity as the authorization check. Same Google-account-level identity used everywhere in the project. |
| T-07-02-08 | Tampering | A non-admin manually invokes the global `showAdminMgmtSidebar` from a menu URL or Apps Script editor | mitigate | The opener admin-gates before constructing HTML. Even if invoked, the non-admin sees the alert modal and nothing else. Tested by TS2. |
| T-07-02-09 | Information Disclosure | Apps Script `Logger.log` / Stackdriver logs emit `callerEmail` for every addAdmin/removeAdmin invocation | accept | Standard structured-logging convention per CLAUDE.md. Log access is scoped to script owner (same trust boundary as the workbook itself). |
| T-07-02-10 | Elevation of Privilege | New global `bootstrapGuildAdminsManual` invocable by anyone via Apps Script editor | partial mitigate | The function reads `Session.getEffectiveUser().getEmail()` and uses that as the seed — so a non-admin invoking it would seed themselves as owner-floor ONLY IF `guild_admins` is currently empty (idempotent check at the lib level). Once seeded, the global cannot be used to escalate further. Acceptable for the bootstrap surface; once initialized the function is a no-op. The `getUi().alert(... Continue?)` confirmation modal adds one user-visible friction step. |

ASVS L1: zero high-severity threats remaining. T-07-02-04 (owner-floor bypass) is mitigated by defense-in-depth across two test layers; T-07-02-01/02/08 are all mitigated by server-side `requireAdminOrThrow` which is the load-bearing primitive (covered exhaustively by admin.test.ts).
</threat_model>

<verification>
- `cd apps-script; npm test 2>&1` exits 0 (all three suites green: existing 297+, admin.test.ts 20, adminMgmtSidebar.test.ts 5–6).
- `cd apps-script; npx tsc --noEmit 2>&1` exits 0.
- `cd apps-script; npm run build 2>&1` exits 0; `dist/Code.js` contains the 5 new globals.
- `Select-String -Path apps-script/src/Code.ts -Pattern "(showAdminMgmtSidebar|getAdminList|addAdmin|removeAdmin|bootstrapGuildAdminsManual)"` matches >= 10 lines (5 imports + 5 exports).
- `Select-String -Path apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "Session\.getEffectiveUser" -SimpleMatch` matches >= 4 lines (one per callback + the opener).
- `Select-String -Path apps-script/src/lib/migrations.ts -Pattern "writeMetaRow\('_meta', 'schema_version', '3'\)"` shows schema_version write is unchanged.
- `Select-String -Path internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion\s*=\s*3"` shows the watcher constant is still 3 (verification hook 5).
</verification>

<success_criteria>
- New `apps-script/src/triggers/showAdminMgmtSidebar.ts` with 4 exports (opener + 3 callbacks) matching the `<interfaces>` contract.
- New `apps-script/src/__tests__/adminMgmtSidebar.test.ts` with 5+ named tests.
- `apps-script/src/Code.ts` exports the 5 new global names (`showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`).
- ADMIN-03 fully closed: admin can open the management UI, add another guildie, and remove non-floor admins; owner-floor is protected by both client-side button suppression and server-side throw.
- Phase 7 verification hook 3 fully covered (admin add + new admin in list — policy from admin.test.ts T7 + sidebar from adminMgmtSidebar.test.ts TS3 — and the integration smoke in Plan 03).
- Phase 7 verification hook 4 fully covered at the sidebar layer (TS5 owner-floor server-side throw; optional TS6 self-removal-of-floor).
- Phase 7 verification hook 5 grep gate: no `schema_version` references in the new sidebar file; `migrations.ts` unchanged; `WatcherMaxSchemaVersion` still 3.
- `clasp push` from the workbook owner's machine will deploy this sidebar to the dev workbook once Plan 03 also lands (the menu wiring is in Plan 03).
</success_criteria>

<output>
After completion, create `.planning/phases/07-admin-allowlist-eviction-enforcement/07-02-SUMMARY.md` capturing:
- Files created (2 absolute paths: showAdminMgmtSidebar.ts, adminMgmtSidebar.test.ts).
- Files modified (1: Code.ts — 5 new globals lifted).
- Test results (5+ adminMgmtSidebar tests pass; full suite still green at N+25 from the v1.0 baseline).
- Any `test-helpers.ts` extensions made for alert capture (diff documented).
- Confirmation that `schema_version` and `WatcherMaxSchemaVersion` are untouched (verification hook 5).
- Confirmation that the 5 new global names appear in `dist/Code.js` (built bundle) so Apps Script's menu resolver can find them after the eventual clasp push.
</output>
