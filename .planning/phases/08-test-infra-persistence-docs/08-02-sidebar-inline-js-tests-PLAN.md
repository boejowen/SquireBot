---
phase: 08-test-infra-persistence-docs
plan: 02
type: execute
wave: 2
depends_on: [08-01]
files_modified:
  - apps-script/src/triggers/showSearchSidebar.ts
  - apps-script/src/triggers/showEvictionSidebar.ts
  - apps-script/src/triggers/showBankCoinSidebar.ts
  - apps-script/src/triggers/showCharInfoSidebar.ts
  - apps-script/src/__tests__/searchSidebar.inline.test.ts
  - apps-script/src/__tests__/evictionSidebar.inline.test.ts
  - apps-script/src/__tests__/bankCoinSidebar.inline.test.ts
  - apps-script/src/__tests__/charInfoSidebar.inline.test.ts
  - .planning/REQUIREMENTS.md
autonomous: true
requirements: [TEST-02]
tags: [apps-script, sidebar-tests, jsdom, inline-js, google-script-run]
must_haves:
  truths:
    - "4 net-new sidebar inline-JS test files exist in `apps-script/src/__tests__/`: `searchSidebar.inline.test.ts`, `evictionSidebar.inline.test.ts`, `bankCoinSidebar.inline.test.ts`, `charInfoSidebar.inline.test.ts`."
    - "Each net-new test file has at least 2 `it(...)` cases: one happy path + one error path per D-03."
    - "Each net-new test file uses `mountSidebar` from `./test-helpers` (Plan 08-01) to mount the sidebar HTML; tests assert on DOM mutations after `dispatchRunCall` / `failRunCall` resolve enqueued google.script.run handlers."
    - "`buildSidebarHtml` is exported (not just defined) in ALL 4 sidebars under TEST-02: `apps-script/src/triggers/showSearchSidebar.ts`, `showEvictionSidebar.ts`, `showBankCoinSidebar.ts`, `showCharInfoSidebar.ts`. Plan-check B1 caught that RESEARCH §A3 incorrectly claimed Search and Eviction already exported it; baseline grep confirmed zero of the 5 sidebars currently export the function."
    - "All 5 shipping sidebars are now under at least one form of vitest coverage: 4 net-new inline-JS tests + 1 existing trigger-call test for admin-mgmt (`adminMgmtSidebar.test.ts`, 7 cases from Phase 7) = 5/5."
    - "`.planning/REQUIREMENTS.md` TEST-02 wording is corrected — `Theme Picker` removed from the sidebar list and replaced with `Admin-Mgmt`; the Admin-Mgmt inline-JS deferral is documented inline. Verification: `grep -c 'Theme Picker' .planning/REQUIREMENTS.md` returns 0."
    - "Theme Picker is documented in this plan's SUMMARY as a `showModalDialog` surface (NOT a sidebar; lives in `onOpen.ts:52-77` as `showThemePickerModal`). The original REQUIREMENTS.md wording was a historical inaccuracy now corrected by Task 1."
    - "Full apps-script suite ends green with 8+ new test cases (4 files × at least 2 cases each)."
  artifacts:
    - path: "apps-script/src/__tests__/searchSidebar.inline.test.ts"
      provides: "Search sidebar inline-JS happy + error path coverage"
      min_lines: 60
      contains: "mountSidebar"
    - path: "apps-script/src/__tests__/evictionSidebar.inline.test.ts"
      provides: "Eviction sidebar inline-JS happy + error path; window.confirm stubbed"
      min_lines: 60
      contains: "mountSidebar"
    - path: "apps-script/src/__tests__/bankCoinSidebar.inline.test.ts"
      provides: "Bank-Coin sidebar inline-JS happy + error path"
      min_lines: 60
      contains: "mountSidebar"
    - path: "apps-script/src/__tests__/charInfoSidebar.inline.test.ts"
      provides: "Char-Info sidebar inline-JS happy + error path"
      min_lines: 60
      contains: "mountSidebar"
  key_links:
    - from: "apps-script/src/__tests__/*Sidebar.inline.test.ts (4 new files)"
      to: "apps-script/src/__tests__/test-helpers.ts:mountSidebar"
      via: "named import from './test-helpers'"
      pattern: "import\\s+\\{[^}]*mountSidebar"
    - from: "apps-script/src/__tests__/searchSidebar.inline.test.ts"
      to: "apps-script/src/triggers/showSearchSidebar.ts:buildSidebarHtml"
      via: "named import of newly-exported buildSidebarHtml"
      pattern: "^export function buildSidebarHtml"
    - from: "apps-script/src/__tests__/evictionSidebar.inline.test.ts"
      to: "apps-script/src/triggers/showEvictionSidebar.ts:buildSidebarHtml"
      via: "named import of newly-exported buildSidebarHtml"
      pattern: "^export function buildSidebarHtml"
    - from: "apps-script/src/__tests__/bankCoinSidebar.inline.test.ts"
      to: "apps-script/src/triggers/showBankCoinSidebar.ts:buildSidebarHtml"
      via: "named import of newly-exported buildSidebarHtml"
      pattern: "^export function buildSidebarHtml"
    - from: "apps-script/src/__tests__/charInfoSidebar.inline.test.ts"
      to: "apps-script/src/triggers/showCharInfoSidebar.ts:buildSidebarHtml"
      via: "named import of newly-exported buildSidebarHtml"
      pattern: "^export function buildSidebarHtml"
---

<objective>
Land 4 net-new inline-JS sidebar test files (Search, Eviction, Bank-Coin, Char-Info) using the `mountSidebar` JSDOM helper from Plan 08-01. Each file ships a happy path + error path per D-03; admin-mgmt's existing `adminMgmtSidebar.test.ts` covers the 5th sidebar at trigger-call depth and is not modified.

Purpose: TEST-02 currently has zero inline-JS-layer coverage for the 4 user-facing sidebars (`google.script.run` callback wiring, DOM event handlers, error rendering). After this plan ships, every shipping sidebar has a vitest companion exercising the SAME contract the live HtmlService iframe runs.

Output: 4 new `.inline.test.ts` files (~60-120 lines each), 1-line `export` rename in **4 sidebars** (Search, Eviction, Bank-Coin, Char-Info — plan-check B1 corrected the original scope which only covered 2), a REQUIREMENTS.md TEST-02 wording fix (Theme Picker → Admin-Mgmt with documented Admin-Mgmt-inline-JS deferral), and a SUMMARY that documents the Theme Picker = modal (not sidebar) correction.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md
@.planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md
@.planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md
@apps-script/src/__tests__/test-helpers.ts
@apps-script/src/__tests__/adminMgmtSidebar.test.ts
@apps-script/src/triggers/showSearchSidebar.ts
@apps-script/src/triggers/showEvictionSidebar.ts
@apps-script/src/triggers/showBankCoinSidebar.ts
@apps-script/src/triggers/showCharInfoSidebar.ts
@apps-script/src/triggers/onOpen.ts

<interfaces>
<!-- Plan 08-01 lands these. Consume verbatim. -->

mountSidebar return shape (from apps-script/src/__tests__/test-helpers.ts post-Plan-08-01):
```typescript
export interface MountedSidebar {
  document: Document;
  window: Window & typeof globalThis;
  runCalls: Array<{ method: string; args: unknown[] }>;
  dispatchRunCall: (method: string, payload: unknown) => void;
  failRunCall: (method: string, error: { message: string }) => void;
  getPendingCalls: () => Array<{ method: string; args: unknown[] }>;
}
export function mountSidebar(html: string): MountedSidebar;
```

Existing test-helpers exports (Phase 7 baseline, still in force):
- `resetMocks(): MockState`
- `seedMeta(state: MockState, rows: Array<[string, string]>): void`
- `makeSheet(name: string, rows?: unknown[][]): MockSheet`
- `seedMetaWithAdmins(state, adminEmails[], extraRows?, floor?)` (defined in showEvictionSidebar.test.ts -- copyable)

installSessionMock pattern (from adminMgmtSidebar.test.ts:29-34, copy verbatim per Pattern B):
```typescript
function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}
```

window.confirm gotcha (08-RESEARCH §Pitfalls #3): showEvictionSidebar.ts:344 calls window.confirm(body) before commitEviction. JSDOM's default returns false; eviction happy-path MUST stub with `vi.spyOn(window, 'confirm').mockReturnValue(true)` before clicking the commit button.

Theme Picker identity (08-RESEARCH §Theme Picker Sidebar Identity): NOT a sidebar -- `showThemePickerModal` in apps-script/src/triggers/onOpen.ts:52-77 is a `SpreadsheetApp.getUi().showModalDialog(...)` call. Plan 08-02 ships 4 sidebar test files, NOT 5. REQUIREMENTS.md TEST-02 wording is corrected in this plan's SUMMARY.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Export buildSidebarHtml in all 4 sidebars under test (Search, Eviction, Bank-Coin, Char-Info) + correct REQUIREMENTS.md TEST-02 wording</name>
  <files>apps-script/src/triggers/showSearchSidebar.ts, apps-script/src/triggers/showEvictionSidebar.ts, apps-script/src/triggers/showBankCoinSidebar.ts, apps-script/src/triggers/showCharInfoSidebar.ts, .planning/REQUIREMENTS.md</files>
  <read_first>
    - apps-script/src/triggers/showSearchSidebar.ts (entire file -- locate `function buildSidebarHtml` near line 114; currently NOT exported despite RESEARCH §A3 claim)
    - apps-script/src/triggers/showEvictionSidebar.ts (entire file -- locate `function buildSidebarHtml` near line 252; currently NOT exported)
    - apps-script/src/triggers/showBankCoinSidebar.ts (entire file -- locate `function buildSidebarHtml` near line 77; currently NOT exported)
    - apps-script/src/triggers/showCharInfoSidebar.ts (entire file -- locate `function buildSidebarHtml` near line 129; currently NOT exported)
    - apps-script/src/triggers/showAdminMgmtSidebar.ts (Phase 7 reference; `function buildSidebarHtml` at line 135 — verify whether it is exported in current state, NOT modified by this plan)
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §Pitfalls #4 -- export-rename rationale (note: A3 line 652 incorrectly claims Search/Eviction/Admin-Mgmt already export; verify via direct grep before assuming)
    - .planning/REQUIREMENTS.md (locate the TEST-02 acceptance criterion; current wording lists "Theme Picker" as a sidebar)
  </read_first>
  <action>
1. **Verify current state first** — plan-check found that NONE of the 5 sidebars currently export `buildSidebarHtml` (research and pattern map both got this wrong). Confirm with grep before patching:

```bash
cd apps-script
grep -nE "^(export )?function buildSidebarHtml" src/triggers/show*Sidebar.ts
```

Expected baseline (5 hits, all without `export`):
```
src/triggers/showAdminMgmtSidebar.ts:135:function buildSidebarHtml(theme: Theme | null): string {
src/triggers/showBankCoinSidebar.ts:77:function buildSidebarHtml(): string {
src/triggers/showCharInfoSidebar.ts:129:function buildSidebarHtml(): string {
src/triggers/showEvictionSidebar.ts:252:function buildSidebarHtml(theme: Theme | null): string {
src/triggers/showSearchSidebar.ts:114:function buildSidebarHtml(theme: Theme | null): string {
```

If the baseline differs (e.g., Admin-Mgmt is already exported because Phase 7 shipped it that way), adjust the patch scope accordingly. **Do not modify `showAdminMgmtSidebar.ts` in this task — Admin-Mgmt's inline-JS tests are out of scope for this plan (it already has trigger-level coverage from Phase 7).**

2. In each of the 4 sidebars under test (Search, Eviction, Bank-Coin, Char-Info), prepend `export ` to the `function buildSidebarHtml` declaration. The rest of the function signature, body, and any internal callers are unchanged — `export` is additive and the existing trigger flow (`show*Sidebar()` calls `buildSidebarHtml()` directly in the same module) is unaffected.

3. Update `.planning/REQUIREMENTS.md` TEST-02 to correct the Theme-Picker misnomer. Find the TEST-02 acceptance line that lists 5 sidebars "(Search, Eviction, Bank-Coin, Char-Info, Theme Picker)". Replace `Theme Picker` with `Admin-Mgmt`. Optionally append a parenthetical clarification on the same line: ` — Theme Picker is a showModalDialog (onOpen.ts:52-77), not a sidebar; Admin-Mgmt inline-JS deferred to v1.1 since adminMgmtSidebar.test.ts already provides trigger-level coverage`. The exact wording is flexible; the load-bearing change is: (a) `Theme Picker` removed, (b) `Admin-Mgmt` added, (c) Admin-Mgmt inline-JS deferral noted.

4. Verify production build still bundles cleanly:

```bash
cd apps-script
grep -n "showSearchSidebar\|showEvictionSidebar\|showBankCoinSidebar\|showCharInfoSidebar" src/Code.ts
npm run build 2>&1 | tail -5
```

If `npm run build` fails because esbuild complains about a duplicate export, the `Code.ts` re-export footer may have a name collision — resolve by importing under an alias if needed. (Unlikely; the `Code.ts` re-export footer re-exports each *opener* function `show*Sidebar`, not `buildSidebarHtml`.)

5. Verify the existing 324+ test suite still passes:

```bash
cd apps-script && npm test 2>&1 | tail -10
```

6. Commit (two commits — one for the export rename, one for the REQUIREMENTS.md correction; keeps the audit trail clean):
```bash
git add apps-script/src/triggers/showSearchSidebar.ts apps-script/src/triggers/showEvictionSidebar.ts apps-script/src/triggers/showBankCoinSidebar.ts apps-script/src/triggers/showCharInfoSidebar.ts
git commit -m "refactor(08-02): export buildSidebarHtml in 4 sidebars under TEST-02 (search, eviction, bank-coin, char-info)"

git add -f .planning/REQUIREMENTS.md
git commit -m "docs(08-02): correct TEST-02 wording — Theme Picker is a modal, not a sidebar; Admin-Mgmt inline-JS deferred"
```
  </action>
  <verify>
    <automated>
cd apps-script
grep -c "^export function buildSidebarHtml" src/triggers/showSearchSidebar.ts | grep -E "^1$" || exit 1
grep -c "^export function buildSidebarHtml" src/triggers/showEvictionSidebar.ts | grep -E "^1$" || exit 1
grep -c "^export function buildSidebarHtml" src/triggers/showBankCoinSidebar.ts | grep -E "^1$" || exit 1
grep -c "^export function buildSidebarHtml" src/triggers/showCharInfoSidebar.ts | grep -E "^1$" || exit 1
cd ..
grep -c "Theme Picker" .planning/REQUIREMENTS.md  # should be 0 after the wording fix
cd apps-script
npm run build 2>&1 | tail -3
npx tsc --noEmit 2>&1 | tail -3
npm test 2>&1 | tail -3
    </automated>
  </verify>
  <acceptance_criteria>
    - `grep -c "^export function buildSidebarHtml" apps-script/src/triggers/showSearchSidebar.ts` returns exactly 1
    - `grep -c "^export function buildSidebarHtml" apps-script/src/triggers/showEvictionSidebar.ts` returns exactly 1
    - `grep -c "^export function buildSidebarHtml" apps-script/src/triggers/showBankCoinSidebar.ts` returns exactly 1
    - `grep -c "^export function buildSidebarHtml" apps-script/src/triggers/showCharInfoSidebar.ts` returns exactly 1
    - `grep -c "Theme Picker" .planning/REQUIREMENTS.md` returns 0 (the misnomer is gone)
    - `grep -c "Admin-Mgmt\|adminMgmtSidebar" .planning/REQUIREMENTS.md` is >= 1 (TEST-02 now references Admin-Mgmt instead)
    - `cd apps-script && npm run build` exits 0 (esbuild produces `dist/Code.js`)
    - `cd apps-script && npx tsc --noEmit` exits 0
    - `cd apps-script && npm test` exits 0 (existing 324+ suite still green)
  </acceptance_criteria>
  <done>All 4 sidebars under TEST-02 expose `buildSidebarHtml` as a named export. REQUIREMENTS.md TEST-02 wording corrected to drop the Theme-Picker misnomer and document the Admin-Mgmt inline-JS deferral. Production build and full test suite stay green.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Create searchSidebar.inline.test.ts + evictionSidebar.inline.test.ts</name>
  <files>apps-script/src/__tests__/searchSidebar.inline.test.ts, apps-script/src/__tests__/evictionSidebar.inline.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/adminMgmtSidebar.test.ts (entire file -- canonical outer-shape analog from Phase 7)
    - apps-script/src/triggers/showSearchSidebar.ts (locate buildSidebarHtml signature + read SIDEBAR_BODY init() at the bottom -- assert the getSearchInitialData callback target + runSearch payload shape)
    - apps-script/src/triggers/showEvictionSidebar.ts (locate buildSidebarHtml + read SIDEBAR_BODY init() -- getEvictionEmails / previewEviction / commitEviction callbacks; line 344 window.confirm gotcha)
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §mountSidebar architecture -- canonical happy + error assertions per sidebar table (D-03 rows for Search + Eviction)
    - .planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md §Sidebar inline-JS tests -- full template skeleton
  </read_first>
  <behavior>
    Search sidebar (`searchSidebar.inline.test.ts`):
    - it('TS1 -- happy path: initial-data → search → results render'): mountSidebar(buildSidebarHtml(null)); dispatchRunCall('getSearchInitialData', { chars: ['Findom'], slots: ['Bag'], recent: [] }); set #q.value = 'bone'; click #searchBtn; dispatchRunCall('runSearch', { groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }], suggestions: [], coldFill: false, durationMs: 12 }); assert #results.innerHTML contains 'Bone Helm'.
    - it('TS2 -- error path: runSearch failure renders #results error region'): same setup as TS1 but failRunCall('runSearch', { message: 'CacheService unavailable' }); assert #results.innerHTML contains 'Search failed' AND 'CacheService unavailable'.

    Eviction sidebar (`evictionSidebar.inline.test.ts`):
    - it('TE1 -- happy path: load emails → preview → confirm → commit'): seed admin via seedMetaWithAdmins; installSessionMock('officer@example.com'); vi.spyOn(window, 'confirm').mockReturnValue(true); mountSidebar; dispatchRunCall('getEvictionEmails', ['a@x.com', 'b@x.com']); set #emailSel; click preview button; dispatchRunCall('previewEviction', { chars: [...], guardrails: 'ok' }); click commit; dispatchRunCall('commitEviction', { ok: true, evicted: 2 }); assert #msg.textContent / #status contains a success marker.
    - it('TE2 -- error path: getEvictionEmails failure renders #msg.error'): mountSidebar; failRunCall('getEvictionEmails', { message: 'unauth' }); assert #msg/.error region textContent contains 'Eviction failed: unauth'.
  </behavior>
  <action>
1. Create `apps-script/src/__tests__/searchSidebar.inline.test.ts` (NEW FILE). Skeleton (adapt selectors and dispatch payloads to the actual DOM IDs in showSearchSidebar.ts's SIDEBAR_BODY):

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showSearchSidebar';

describe('showSearchSidebar -- inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
  });

  // TS1 -- D-03 happy path
  it('TS1 -- initial-data loads, user searches, results render', () => {
    const html = buildSidebarHtml(/* theme */ null as unknown as undefined);
    const m = mountSidebar(html);

    // init() fires getSearchInitialData on mount.
    m.dispatchRunCall('getSearchInitialData', {
      chars: ['Findom'],
      slots: ['Bag'],
      recent: [],
    });

    // User types + clicks the Search button.
    const q = m.document.getElementById('q') as HTMLInputElement;
    q.value = 'bone';
    (m.document.getElementById('searchBtn') as HTMLButtonElement).click();

    // Resolve the runSearch call.
    m.dispatchRunCall('runSearch', {
      groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }],
      suggestions: [],
      coldFill: false,
      durationMs: 12,
    });

    expect(m.document.getElementById('results')!.innerHTML).toContain('Bone Helm');
  });

  // TS2 -- D-03 error path
  it('TS2 -- runSearch failure renders error region in #results', () => {
    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);
    m.dispatchRunCall('getSearchInitialData', { chars: [], slots: [], recent: [] });

    const q = m.document.getElementById('q') as HTMLInputElement;
    q.value = 'bone';
    (m.document.getElementById('searchBtn') as HTMLButtonElement).click();

    m.failRunCall('runSearch', { message: 'CacheService unavailable' });

    const html2 = m.document.getElementById('results')!.innerHTML;
    expect(html2).toContain('Search failed');
    expect(html2).toContain('CacheService unavailable');
  });
});
```

Notes for executor:
- The exact DOM IDs (`#q`, `#searchBtn`, `#results`) and the runSearch result-rendering selector come from `showSearchSidebar.ts`'s SIDEBAR_BODY. If the actual ID is different, update both the test and the assertion accordingly -- the test is canonical via behavior, not via literal ID strings.
- If `buildSidebarHtml` takes a `theme` argument with a richer type, import `THEMES` from `../lib/themes` and pass `THEMES['minimalist']` or `THEMES['vanilla']`. The test only cares about HTML being buildable; theme content is irrelevant.
- If the runSearch error renders into a different DOM node than `#results` (some sidebars use a sibling `.error` div), update the assertion to match — read the source of truth from `showSearchSidebar.ts`.

2. Create `apps-script/src/__tests__/evictionSidebar.inline.test.ts` (NEW FILE). Skeleton:

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showEvictionSidebar';

function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

describe('showEvictionSidebar -- inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '3'],
      ['theme', 'minimalist'],
      ['guild_admins', JSON.stringify(['officer@example.com'])],
      ['workbook_owner_floor', 'officer@example.com'],
    ]);
    installSessionMock('officer@example.com');
  });
  afterEach(() => {
    delete (globalThis as Record<string, unknown>).Session;
    vi.restoreAllMocks();
  });

  // TE1 -- D-03 happy path (window.confirm must be stubbed -- 08-RESEARCH Pitfalls #3)
  it('TE1 -- emails load, preview renders, confirm + commit fires', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);

    m.dispatchRunCall('getEvictionEmails', ['guildie-a@example.com', 'guildie-b@example.com']);

    // Select an email then trigger preview. Element IDs come from showEvictionSidebar.ts SIDEBAR_BODY.
    const sel = m.document.getElementById('emailSel') as HTMLSelectElement;
    sel.value = 'guildie-a@example.com';
    (m.document.getElementById('previewBtn') as HTMLButtonElement).click();
    m.dispatchRunCall('previewEviction', { chars: ['CharA'], guardrails: 'ok' });

    // Confirm + commit
    (m.document.getElementById('commitBtn') as HTMLButtonElement).click();
    m.dispatchRunCall('commitEviction', { ok: true, evicted: 1 });

    const status = m.document.getElementById('msg') ?? m.document.getElementById('status');
    expect(status!.textContent || status!.innerHTML).toMatch(/evict|success|complete/i);
  });

  // TE2 -- D-03 error path
  it('TE2 -- getEvictionEmails failure renders error in #msg', () => {
    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);
    m.failRunCall('getEvictionEmails', { message: 'unauth' });

    const msg = m.document.getElementById('msg');
    expect(msg).not.toBeNull();
    expect(msg!.textContent || msg!.innerHTML).toContain('unauth');
  });
});
```

Notes for executor:
- The exact button + select IDs (`#emailSel`, `#previewBtn`, `#commitBtn`, `#msg`) must match the IDs in `showEvictionSidebar.ts`'s SIDEBAR_BODY. Adjust if names differ.
- If `buildSidebarHtml` requires a theme argument with a non-null type, import the appropriate type or pass a fixture from `THEMES`.

3. Run the apps-script suite. Both new files should pass; existing tests unaffected.

```bash
cd apps-script && npm test 2>&1 | tail -15
```

4. Commit:
```bash
git add apps-script/src/__tests__/searchSidebar.inline.test.ts apps-script/src/__tests__/evictionSidebar.inline.test.ts
git commit -m "test(08-02): add search + eviction sidebar inline-JS tests (TEST-02)"
```
  </action>
  <verify>
    <automated>
cd apps-script
test -f src/__tests__/searchSidebar.inline.test.ts || exit 1
test -f src/__tests__/evictionSidebar.inline.test.ts || exit 1
grep -c "^\s*it(" src/__tests__/searchSidebar.inline.test.ts | awk '{exit ($1 >= 2 ? 0 : 1)}'
grep -c "^\s*it(" src/__tests__/evictionSidebar.inline.test.ts | awk '{exit ($1 >= 2 ? 0 : 1)}'
grep -q "mountSidebar" src/__tests__/searchSidebar.inline.test.ts || exit 1
grep -q "mountSidebar" src/__tests__/evictionSidebar.inline.test.ts || exit 1
grep -q "window.confirm" src/__tests__/evictionSidebar.inline.test.ts || exit 1
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `test -f apps-script/src/__tests__/searchSidebar.inline.test.ts` exits 0
    - `test -f apps-script/src/__tests__/evictionSidebar.inline.test.ts` exits 0
    - `grep -c "^\s*it(" apps-script/src/__tests__/searchSidebar.inline.test.ts` returns at least 2
    - `grep -c "^\s*it(" apps-script/src/__tests__/evictionSidebar.inline.test.ts` returns at least 2
    - `grep -c "mountSidebar" apps-script/src/__tests__/searchSidebar.inline.test.ts` returns at least 1
    - `grep -c "window.confirm" apps-script/src/__tests__/evictionSidebar.inline.test.ts` returns at least 1 (08-RESEARCH Pitfalls #3 mitigation)
    - `cd apps-script && npm test` exits 0 (both new test files pass; existing tests still pass)
  </acceptance_criteria>
  <done>Search + Eviction inline-JS tests pass; the eviction window.confirm gotcha is stubbed; full suite stays green.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Create bankCoinSidebar.inline.test.ts + charInfoSidebar.inline.test.ts</name>
  <files>apps-script/src/__tests__/bankCoinSidebar.inline.test.ts, apps-script/src/__tests__/charInfoSidebar.inline.test.ts</files>
  <read_first>
    - apps-script/src/triggers/showBankCoinSidebar.ts (entire file -- read after Task 1's export rename; SIDEBAR_BODY init() callback names: getBankCoinForForm, saveBankCoin per RESEARCH)
    - apps-script/src/triggers/showCharInfoSidebar.ts (entire file -- after Task 1 export rename; SIDEBAR_BODY init() callback names: getCharsForForm, saveCharInfo per RESEARCH)
    - apps-script/src/__tests__/searchSidebar.inline.test.ts (just landed in Task 2 -- use as analog)
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §mountSidebar architecture -- canonical assertions table rows for Bank-Coin and Char-Info
  </read_first>
  <behavior>
    Bank-Coin sidebar (`bankCoinSidebar.inline.test.ts`):
    - it('TB1 -- happy path: initial-data populates inputs'): mountSidebar(buildSidebarHtml(null)); dispatchRunCall('getBankCoinForForm', { pp: 100, gp: 50, sp: 25, cp: 0 }); assert (#pp as HTMLInputElement).value === '100' and #saveBtn is enabled.
    - it('TB2 -- error path: getBankCoinForForm failure renders #msg with #c00 color'): failRunCall('getBankCoinForForm', { message: 'denied' }); assert #msg.style.color (or msg textContent) reflects "Failed to load: denied".

    Char-Info sidebar (`charInfoSidebar.inline.test.ts`):
    - it('TC1 -- happy path: chars list populates table'): dispatchRunCall('getCharsForForm', [{ char_name: 'X', class: 'SHD', level: 60, race: 'IKS' }]); assert #charBody (or whatever the tbody ID is) contains a row with '<td>X</td>' or textContent 'X'.
    - it('TC2 -- error path: getCharsForForm failure renders #msg'): failRunCall('getCharsForForm', { message: 'fail' }); assert #msg textContent contains 'Failed to load: fail'.
  </behavior>
  <action>
1. Create `apps-script/src/__tests__/bankCoinSidebar.inline.test.ts` (NEW FILE) with the Search/Eviction-test pattern. Adapt selectors to bank-coin's SIDEBAR_BODY (read source for canonical IDs):

```typescript
import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showBankCoinSidebar';

describe('showBankCoinSidebar -- inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist'], ['bank_toon_name', 'BankToon']]);
  });

  // TB1 -- D-03 happy path
  it('TB1 -- initial-data populates pp/gp/sp/cp inputs', () => {
    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);

    m.dispatchRunCall('getBankCoinForForm', { pp: 100, gp: 50, sp: 25, cp: 0 });

    const pp = m.document.getElementById('pp') as HTMLInputElement;
    expect(pp.value).toBe('100');
    const saveBtn = m.document.getElementById('saveBtn') as HTMLButtonElement | null;
    if (saveBtn) expect(saveBtn.disabled).toBe(false);
  });

  // TB2 -- D-03 error path
  it('TB2 -- getBankCoinForForm failure renders #msg error', () => {
    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);

    m.failRunCall('getBankCoinForForm', { message: 'denied' });

    const msg = m.document.getElementById('msg');
    expect(msg).not.toBeNull();
    expect(msg!.textContent || msg!.innerHTML).toMatch(/denied|Failed/i);
  });
});
```

2. Create `apps-script/src/__tests__/charInfoSidebar.inline.test.ts` (NEW FILE):

```typescript
import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showCharInfoSidebar';

describe('showCharInfoSidebar -- inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
  });

  // TC1 -- D-03 happy path
  it('TC1 -- chars list populates table', () => {
    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);

    m.dispatchRunCall('getCharsForForm', [
      { char_name: 'X', class: 'SHD', level: 60, race: 'IKS' },
    ]);

    // The exact tbody / row container ID comes from showCharInfoSidebar.ts SIDEBAR_BODY.
    // Common candidates: #charBody, #charsTable tbody, #rows. Adjust to match source.
    const body = m.document.querySelector('tbody') ?? m.document.getElementById('charBody');
    expect(body).not.toBeNull();
    expect((body as Element).innerHTML).toContain('X');
  });

  // TC2 -- D-03 error path
  it('TC2 -- getCharsForForm failure renders #msg', () => {
    const html = buildSidebarHtml(null as unknown as undefined);
    const m = mountSidebar(html);
    m.failRunCall('getCharsForForm', { message: 'fail' });
    const msg = m.document.getElementById('msg');
    expect(msg).not.toBeNull();
    expect(msg!.textContent || msg!.innerHTML).toMatch(/fail|Failed/i);
  });
});
```

3. Run the suite:

```bash
cd apps-script && npm test 2>&1 | tail -15
```

If any test fails because the selector ID in the test doesn't match the actual SIDEBAR_BODY ID, FIX the test to read the source-of-truth ID from `showBankCoinSidebar.ts` / `showCharInfoSidebar.ts`. Do NOT change the SIDEBAR_BODY HTML to match the test.

4. Commit:
```bash
git add apps-script/src/__tests__/bankCoinSidebar.inline.test.ts apps-script/src/__tests__/charInfoSidebar.inline.test.ts
git commit -m "test(08-02): add bank-coin + char-info sidebar inline-JS tests (TEST-02)"
```
  </action>
  <verify>
    <automated>
cd apps-script
test -f src/__tests__/bankCoinSidebar.inline.test.ts || exit 1
test -f src/__tests__/charInfoSidebar.inline.test.ts || exit 1
grep -c "^\s*it(" src/__tests__/bankCoinSidebar.inline.test.ts | awk '{exit ($1 >= 2 ? 0 : 1)}'
grep -c "^\s*it(" src/__tests__/charInfoSidebar.inline.test.ts | awk '{exit ($1 >= 2 ? 0 : 1)}'
grep -q "mountSidebar" src/__tests__/bankCoinSidebar.inline.test.ts || exit 1
grep -q "mountSidebar" src/__tests__/charInfoSidebar.inline.test.ts || exit 1
# 5/5 sidebar coverage: 4 new inline-JS + 1 existing admin-mgmt trigger-call
ls src/__tests__/ | grep -E "(Sidebar|sidebar)\.(inline\.)?test\.ts" | wc -l | awk '{exit ($1 >= 5 ? 0 : 1)}'
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `test -f apps-script/src/__tests__/bankCoinSidebar.inline.test.ts` exits 0
    - `test -f apps-script/src/__tests__/charInfoSidebar.inline.test.ts` exits 0
    - `grep -c "^\s*it(" apps-script/src/__tests__/bankCoinSidebar.inline.test.ts` returns at least 2
    - `grep -c "^\s*it(" apps-script/src/__tests__/charInfoSidebar.inline.test.ts` returns at least 2
    - Sidebar test coverage check: counting files matching `*Sidebar.test.ts` OR `*Sidebar.inline.test.ts` in `apps-script/src/__tests__/` returns at least 5 (the 4 new inline-JS files + the existing `adminMgmtSidebar.test.ts`)
    - `cd apps-script && npm test` exits 0 with at least 8 net-new test cases passing (4 files × at least 2 cases each)
  </acceptance_criteria>
  <done>4 net-new sidebar inline-JS test files exist; each ships happy + error paths; full apps-script suite stays green with 8+ new tests added.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| test code ↔ production HtmlService bundle | New `.inline.test.ts` files run only under vitest; never imported by `src/Code.ts` (esbuild entry) — structurally excluded from `dist/Code.js`. |
| `buildSidebarHtml` export | Now globally accessible from any consumer of the sidebar module; was previously module-private. Since the function is pure (string-build, no side effects), exposing it does not expand the runtime attack surface. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-08-02-01 | Tampering | Tests assert wrong DOM contract -- failing to catch real regressions | mitigate | Tests assert on real DOM IDs read from each sidebar's SIDEBAR_BODY source-of-truth; if the trigger-side SIDEBAR_BODY changes its IDs, the test will fail loudly. Cross-reference between test and source held by literal ID strings. |
| T-08-02-02 | Information disclosure | mountSidebar Proxy mock exposes a wider chain surface than live google.script.run | accept | Documented in 08-01 threat model; reverse failure mode is the standard test-vs-production tradeoff. |
| T-08-02-03 | Repudiation | Theme Picker explicitly excluded from this plan; future TEST-02 audit might ask "where's the Theme Picker test?" | mitigate | Plan SUMMARY explicitly documents Theme Picker is a `showModalDialog` (NOT a sidebar) in `onOpen.ts:52-77`, and that REQUIREMENTS.md TEST-02 wording was historically inaccurate. Admin-Mgmt's `adminMgmtSidebar.test.ts` covers the actual 5th sidebar at trigger-call depth. |
| T-08-02-04 | Elevation of privilege | Bank-Coin / Char-Info `buildSidebarHtml` now exported | accept | Pure function; no runtime privilege change. Function is already invoked by the sidebar opener in the same module; export merely adds external visibility for tests. |
</threat_model>

<verification>
After all 3 tasks complete:

```bash
cd apps-script
# Bank-Coin + Char-Info exports
grep -c "^export function buildSidebarHtml" src/triggers/showBankCoinSidebar.ts
grep -c "^export function buildSidebarHtml" src/triggers/showCharInfoSidebar.ts

# 4 net-new sidebar test files exist
for f in searchSidebar.inline.test.ts evictionSidebar.inline.test.ts bankCoinSidebar.inline.test.ts charInfoSidebar.inline.test.ts; do
  test -f "src/__tests__/$f" || { echo "MISSING: $f"; exit 1; }
done

# Each has >= 2 it() cases per D-03
for f in src/__tests__/{search,eviction,bankCoin,charInfo}Sidebar.inline.test.ts; do
  n=$(grep -c "^\s*it(" "$f")
  [ "$n" -ge 2 ] || { echo "FAIL: $f has only $n it() cases"; exit 1; }
done

# Full suite green
npm test
```

Verification-hook 5 (schema-gate, untouched):
```bash
grep -c "writeMetaRow.*schema_version.*'3'" apps-script/src/lib/migrations.ts  # baseline unchanged
grep "WatcherMaxSchemaVersion" internal/sheet/client.go  # = 3, unchanged
```
</verification>

<success_criteria>
- 4 new sidebar inline-JS test files exist; each has >= 2 it() cases (happy + error per D-03).
- `buildSidebarHtml` is exported from `showBankCoinSidebar.ts` AND `showCharInfoSidebar.ts`.
- All 5 shipping sidebars have at least one test file: searchSidebar.inline.test.ts, evictionSidebar.inline.test.ts, bankCoinSidebar.inline.test.ts, charInfoSidebar.inline.test.ts, adminMgmtSidebar.test.ts (existing).
- SUMMARY explicitly notes Theme Picker is a modal (not a sidebar) and that REQUIREMENTS.md TEST-02 wording is corrected.
- Full apps-script suite ends green (~332+/332+ tests; baseline 324+ plus at least 8 new from this plan).
- Schema gates unchanged: migrations.ts schema_version='3' write count and client.go WatcherMaxSchemaVersion=3 both untouched.
</success_criteria>

<output>
After completion, create `.planning/phases/08-test-infra-persistence-docs/08-02-SUMMARY.md` per the Phase 5 template. Include in `decisions:` the Theme-Picker-is-a-modal correction and the buildSidebarHtml export rationale.
</output>
