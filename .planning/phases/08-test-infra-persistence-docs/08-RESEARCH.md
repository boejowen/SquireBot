# Phase 8: Test Infra + Persistence + Docs Backfill — Research

**Researched:** 2026-05-12
**Domain:** vitest 1.6 + JSDOM environment configuration, sidebar inline-JS testing, Apps Script PropertiesService migration, SUMMARY.md backfill
**Confidence:** HIGH (all four surfaces verified against shipped source + npm registry + current docs)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01** — JSDOM environment is globally configured in `vitest.config.ts` (not per-test annotation). Per-test `// @vitest-environment node` overrides remain available but no current test needs one.
- **D-02** — Sidebar tests mount the existing `SIDEBAR_BODY` HTML string verbatim — no extraction to `.html` files. Tests import `buildSidebarHtml(theme)` (or equivalent), call it with a fixture theme, and feed the result string into `mountSidebar(htmlString)`. Backlog 999.7 (extract to `.html`) stays deferred.
- **D-03** — Test scope per sidebar = happy path + at least 1 error path (exactly 2 `it(...)` cases minimum per new sidebar test file). Optional 3rd case (empty state / validation rejection / double-click guard) only if the existing trigger-side test references the case.
- **D-04** — Shared `mountSidebar(html: string)` helper lives in `apps-script/src/__tests__/test-helpers.ts` alongside `resetMocks` / `makeSheet` / `seedMeta` / `installSessionMock` / `installAppsScriptMocks`. Returns the JSDOM realm + a controllable `google.script.run` mock with `dispatchRunCall(methodName, response)` for resolving enqueued calls.
- **D-05** — SEARCH-05 PropertiesService scope is `getUserProperties()` (per-user), NOT `getDocumentProperties()`. JSON-encoded-string-array storage shape identical to the prior cache shape — no parse changes downstream.
- **D-06** — Migration semantics = clear-and-replace. Drop the `CacheService.getDocumentCache()` write path entirely; no dual-write transition, no cache backfill. Worst-case UX: one empty `recent[]` list on first search after v1.0.1 ship.
- **D-07** — DOC-04 backfill matches Phase 5 SUMMARY.md template byte-for-byte: YAML frontmatter (`phase / plan / subsystem / tags / requires / provides / affects / tech-stack / key-files / decisions / metrics`) + Markdown body (`# Phase X Plan NN: …`, `## What shipped`, `## Schema impact`, `## Verification log`, `## Self-Check`).
- **D-08** — Plan structure = 4 plans across 2 waves. Wave 1 (parallel): 08-01 (TEST-01 config + helper), 08-03 (SEARCH-05 swap), 08-04 (DOC-04 docs). Wave 2 (depends_on 08-01): 08-02 (TEST-02 5 new sidebar test files).

### Claude's Discretion

- `describe`/`beforeEach` ordering inside each new sidebar test file — match `adminMgmtSidebar.test.ts` shape (closest analog; ships in current `master`).
- Whether `mountSidebar` returns the `Document` directly or a wrapping object — pick whichever produces the cleanest test bodies (recommend a wrapping object with `{ document, window, google }` keys so tests don't need to reach into global state).
- Whether to delete the now-unused `CACHE_TTL_SECONDS` constant in `searchIndex.ts` after Plan 08-03 — tiny cleanup, planner's call (recommend keep + comment `// retained for buildInvCache; recent-MRU TTL retired 08-03` so future readers don't ping-pong).
- Prose voice in the 8 retroactive SUMMARY.md files — past tense, declarative, mirror Phase 5's voice.

### Deferred Ideas (OUT OF SCOPE)

- **999.7** — Extract `SIDEBAR_BODY` constants to standalone `apps-script/src/sidebars/*.html` files. Cosmetic refactor; CSP + theme-token interpolation surface area. v1.1 polish.
- Coverage thresholds (`coverage.thresholds.lines: 70`) in `vitest.config.ts` — defer to v1.1.
- Snapshot tests for sidebar rendered HTML — vitest supports them; maintenance burden not worth it.
- PropertiesService quota monitoring trigger — currently no guard; defer to 999.X.
- Backfilling SUMMARY.md for milestone v1.0 itself — out of scope; milestone audit already exists.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TEST-01 | vitest configured with JSDOM environment so `npm test` exercises sidebar inline-JS on every PR with no separate command | §Technical surface — vitest+JSDOM config block + jsdom devDep install + ~50MB install cost confirmed via npm registry |
| TEST-02 | Every shipping sidebar has a co-located `__tests__/*-sidebar.test.ts` companion file exercising real DOM event handlers, payload assembly, error display | §Technical surface — mountSidebar helper architecture + uniform `google.script.run.withSuccessHandler().withFailureHandler().METHOD()` chain shape verified across all 5 sidebars |
| SEARCH-05 | Recent-3 search MRU persists across CacheService 25-min TTL via `PropertiesService.getUserProperties()` per-user scope | §Technical surface — PropertiesService migration mechanics + 500KB scope quota + 8KB-per-value cap confirmed; 3-entry JSON array fits with ~25,000x headroom |
| DOC-04 | 8 retroactive SUMMARY.md files in Phase 3 (4 plans) + Phase 4 (4 plans) | §Technical surface — Phase 5 frontmatter field inventory + verified zero existing SUMMARY.md in either phase dir |

</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Sidebar HTML+JS rendering (under test) | Browser / Client (JSDOM-mocked) | — | Inline `<script>` blocks execute in the HtmlService iframe at runtime; tests replicate that by mounting in JSDOM |
| `google.script.run` callback routing | Browser / Client (mocked) | API / Backend (real implementations covered by existing tests) | The TEST-02 gap is the inline-JS layer (success/failure handler wiring); server callbacks are already covered by trigger-level tests |
| Recent-search MRU storage (SEARCH-05) | API / Backend (Apps Script) | — | `PropertiesService.getUserProperties()` is a server-side primitive; the sidebar still calls into `getRecentSearches`/`pushRecentSearchCall` over `google.script.run` |
| SUMMARY.md backfill (DOC-04) | Documentation / Static | — | Pure docs surface; no runtime tier involved |

## Executive Summary

Phase 8 closes four pieces of v1.0 carry-over debt, all locked to specific implementations by CONTEXT D-01..D-08. The four pieces that the planner needs most:

1. **The 5th sidebar in TEST-02 is Admin-Mgmt, not Theme Picker.** REQUIREMENTS.md SEARCH-05 was written before Phase 7 shipped `showAdminMgmtSidebar`; the named "Theme Picker" is actually `showThemePickerModal` in `apps-script/src/triggers/onOpen.ts` — a `showModalDialog` call, not `showSidebar`. The 5 actual sidebars (verified by `grep -l "SpreadsheetApp.getUi().showSidebar"`) are Search, Eviction, Bank-Coin, Char-Info, Admin-Mgmt. Of these, Admin-Mgmt already has tests (`adminMgmtSidebar.test.ts`, 7 cases); the 4 net-new files are Search, Eviction, Bank-Coin, Char-Info. Theme Picker modal is a 70-line inline-JS surface that could be tested with the same `mountSidebar` helper if the planner wants — recommend NOT in 08-02 scope (REQUIREMENTS.md doesn't explicitly require it once the "sidebar" misnomer is clarified, and modal closure via `google.script.host.close()` adds a JSDOM gotcha the helper doesn't need to solve in v1.0.1).

2. **jsdom is NOT currently installed.** `apps-script/package.json` lists `vitest ^1.6.0` but no jsdom devDep; `npm ls jsdom` returns empty. Plan 08-01 must `npm install -D jsdom@^24.0.0` (24.x is the current major series; 29.1.1 is the latest stable but vitest 1.6 ships in 2024 and pairs cleanly with jsdom 24.x — see §vitest+JSDOM config). The `@types/jsdom` package is OPTIONAL because vitest 1.6 ships its own JSDOM globals types when `environment: 'jsdom'` is set; vitest's docs say "add `vitest/jsdom` to tsconfig.json compilerOptions.types if you want TS to recognize globals."

3. **All 5 sidebars use the SAME `google.script.run` chain shape.** Verified via grep: `withSuccessHandler(fn).withFailureHandler(fn).METHOD(args)`. This uniformity is what makes the D-04 helper viable. The one ordering deviation — `showSearchSidebar.ts:206` chains `withSuccessHandler` first, `withFailureHandler` second, then `runSearch(...)`; some sidebars omit `withFailureHandler` on fire-and-forget calls like `pushRecentSearchCall(q)` — is structural, not semantic. The mock must be fluent and tolerant of any-order chaining + missing handlers.

4. **PropertiesService quota fits with ~25,000x headroom.** Official quota: 500KB total per scope, 8KB per value, 9KB per key+value combined (Apps Script docs + Tanaike's specification report). Three queries × ~50 chars × JSON overhead ≈ 200 bytes. The migration is structurally safe; no bounded-size guard needed beyond the existing `.slice(0, RECENT_LIMIT)` cap.

**Primary recommendation:** Lock the 4 net-new sidebar test files in Plan 08-02 to `searchSidebar.test.ts`, `evictionSidebar.test.ts`, `bankCoinSidebar.test.ts`, `charInfoSidebar.test.ts`. Reframe the "5th sidebar = Theme Picker" assumption in REQUIREMENTS.md TEST-02 as a clarifying footnote in the 08-02 SUMMARY (Theme Picker is a modal, Admin-Mgmt is the actual 5th sidebar — and it already has the closest-analog test as of 2026-05-12).

## Decision Verification (D-01..D-08)

| Decision | Researchable? | Surprises / Confirmations |
|----------|--------------|---------------------------|
| **D-01** JSDOM global env | ✅ Yes | Confirmed: vitest 1.6 syntax is `test: { environment: 'jsdom' }` in `defineConfig`. jsdom is `optionalDependency` in vitest, NOT a peer — vitest will prompt "do you want to install jsdom?" if env is set but package missing. No `include` pattern conflict; current default discovery is `**/*.{test,spec}.?(c|m)[jt]s?(x)` which covers `src/__tests__/**/*.test.ts`. |
| **D-02** Inline HTML mount | ✅ Yes | Confirmed: 5/5 sidebars build their HTML via either an exported `buildSidebarHtml(theme)` function (Search, Eviction, Admin-Mgmt) or a non-exported `buildSidebarHtml()` (Bank-Coin, Char-Info). Bank-Coin and Char-Info will need either an export-rename or a test-side helper. JSDOM `innerHTML` gotcha is REAL (HTML5 spec: script tags inserted via innerHTML do NOT execute). Workaround pattern documented below in §mountSidebar. |
| **D-03** Happy + 1 error per sidebar | ✅ Yes | Canonical error-path assertions per sidebar identified — see §mountSidebar architecture. All 5 sidebars render error states into a known DOM region (`#msg`, `#results`, `#status` — varies). All call `showErr(err)` or equivalent in their inline-JS error handler. |
| **D-04** mountSidebar helper | ✅ Yes | Verified `google.script.run.withSuccessHandler(fn).withFailureHandler(fn).METHOD(args)` chain shape is uniform across all 5 sidebars (grepped + read). One sidebar (Search) does a fire-and-forget `google.script.run.pushRecentSearchCall(q)` with NO handler chain — the mock's fluent surface must support both shapes. |
| **D-05** getUserProperties | ✅ Yes | Confirmed: `PropertiesService.getUserProperties()` exists in `@types/google-apps-script ^1.0.91` (currently installed). API surface used by `searchIndex.ts`: `getProperty(key)`, `setProperty(key, value)`. No `getProperties()` / `deleteProperty()` / `deleteAllProperties()` needed for the recent-MRU. Quota fits with ~25,000x headroom. |
| **D-06** Clear-and-replace | ✅ Yes — confirmed safe | Existing implementation writes only to `CacheService.getDocumentCache()` with 25-min TTL (`searchIndex.ts:355-371`). NO transient retry path, NO queued-trigger replay — the data is ephemeral by design. The dual-write would protect against NOTHING — confirmed. |
| **D-07** Phase 5 template byte-for-byte | ✅ Yes | Verified frontmatter field inventory across all 5 Phase 5 SUMMARYs — see §SUMMARY.md template fidelity. Mild variance exists (e.g., `tech-stack.added` is `[]` for backend plans, populated for 05-05 which added Jekyll Pages); the planner should mirror this honestly, NOT invent a stricter schema. |
| **D-08** 4 plans / 2 waves | ✅ Yes — confirmed | Dependency graph verified: Plan 08-02 imports `mountSidebar` from `test-helpers.ts` which Plan 08-01 lands. Plans 01/03/04 touch zero overlapping source files (vitest.config.ts + test-helpers.ts vs. searchIndex.ts + searchIndex.test.ts vs. SUMMARY.md docs paths). Wave-1 parallel safe. |

## Technical Surfaces

### vitest + JSDOM config (TEST-01 → Plan 08-01)

**jsdom devDep status:** NOT installed. `npm ls jsdom` returns empty inside `apps-script/`. Plan 08-01 must add it.

**Recommended install command:**
```bash
cd apps-script && npm install -D jsdom@^24.0.0
```

`jsdom 24.x` is the version series that pairs with vitest 1.6 (released April 2024, contemporary with vitest 1.6 release). `jsdom 25.x` and later require Node 18.17+; the project's Node baseline is undocumented but the CI presumably runs on a current LTS. If CI breaks on jsdom 24.x, fall forward to 25.x or 26.x — vitest 1.6 has an `optionalDependencies` relationship, not a peer pin, so any modern jsdom works.

**Exact `vitest.config.ts` for Plan 08-01 (NEW FILE):**
```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['src/__tests__/**/*.test.ts'],
    // No globals: true — existing tests already import { describe, it, expect, beforeEach } from 'vitest' explicitly
  },
});
```

**Why `include` is explicit:** the apps-script repo has one `__tests__/` dir under `src/`. Without `include`, vitest's default would also crawl `node_modules/` for any nested `*.test.ts` — slower + accidental discovery risk. The existing `npm test` script runs `vitest run` with no config file, relying on default discovery; explicit include hardens the contract.

**TypeScript globals (optional):** vitest 1.3+ exposes a `jsdom` global equal to the current JSDOM instance. If a test wants to access it, the planner can add `"vitest/jsdom"` to `apps-script/tsconfig.json` `compilerOptions.types`. Recommend NOT adding unless a test needs it — keeps the type surface minimal.

**Quota check (D-01 footnote):** jsdom 24.x install size is ~50MB (transitive includes `cssom`, `tough-cookie`, `webidl-conversions`, `whatwg-url`, etc.). This is fine for devDep but worth noting in the SUMMARY.

**Acceptance gate (grep-friendly):**
```bash
test -f apps-script/vitest.config.ts
grep -q "environment: 'jsdom'" apps-script/vitest.config.ts
grep -q '"jsdom"' apps-script/package.json
cd apps-script && npm test  # all existing 324+ tests still pass
```

### mountSidebar helper architecture (TEST-02 → Plan 08-01 + 08-02)

**The JSDOM `innerHTML` gotcha:** Per HTML5 spec, script tags inserted via `document.body.innerHTML = htmlWithScript` do NOT execute. This is browser-uniform behavior, not a JSDOM bug. JSDOM does support `runScripts: 'dangerously'` constructor option for full script execution, but that option only applies when JSDOM is instantiated standalone — vitest's `environment: 'jsdom'` does NOT expose it.

**The correct pattern (canonical, used by ghinda/run-script-tags and similar libs):**

```typescript
// In test-helpers.ts (D-04)
export interface MountedSidebar {
  document: Document;
  window: Window & typeof globalThis;
  // Fluent mock for google.script.run; tests resolve enqueued calls via dispatch.
  dispatchRunCall: (method: string, payload: unknown) => void;
  failRunCall: (method: string, error: { message: string }) => void;
  // For assertion: every captured call.
  runCalls: Array<{ method: string; args: unknown[] }>;
}

export function mountSidebar(html: string): MountedSidebar {
  // 1. Reset the body.
  document.body.innerHTML = '';

  // 2. Parse the html into a detached fragment.
  const tpl = document.createElement('template');
  tpl.innerHTML = html;

  // 3. Walk the parsed fragment, separating script nodes from non-script nodes.
  const scripts: HTMLScriptElement[] = [];
  const frag = document.createDocumentFragment();
  tpl.content.childNodes.forEach((node) => {
    if (node.nodeName === 'SCRIPT') {
      scripts.push(node as HTMLScriptElement);
    } else {
      frag.appendChild(node.cloneNode(true));
    }
  });
  document.body.appendChild(frag);

  // 4. Install the google.script.run mock BEFORE executing inline scripts
  //    (the inline scripts read `google.script.run` at top level in some sidebars).
  const runCalls: Array<{ method: string; args: unknown[] }> = [];
  const pendingByMethod = new Map<string, Array<{ success?: Function; failure?: Function }>>();

  function makeChain(currentHandlers: { success?: Function; failure?: Function }): any {
    const chain = new Proxy({}, {
      get(_target, prop: string) {
        if (prop === 'withSuccessHandler') {
          return (fn: Function) => makeChain({ ...currentHandlers, success: fn });
        }
        if (prop === 'withFailureHandler') {
          return (fn: Function) => makeChain({ ...currentHandlers, failure: fn });
        }
        // Any other prop = the terminal method invocation.
        return (...args: unknown[]) => {
          runCalls.push({ method: prop, args });
          const queue = pendingByMethod.get(prop) ?? [];
          queue.push(currentHandlers);
          pendingByMethod.set(prop, queue);
        };
      },
    });
    return chain;
  }

  (window as any).google = { script: { run: makeChain({}) } };

  // 5. Re-create script elements with .textContent set, then append to head
  //    — this is the standard pattern for forcing inline scripts to execute
  //    in JSDOM.
  scripts.forEach((orig) => {
    const s = document.createElement('script');
    if (orig.textContent) s.textContent = orig.textContent;
    document.head.appendChild(s);
  });

  function dispatchRunCall(method: string, payload: unknown): void {
    const queue = pendingByMethod.get(method);
    if (!queue || queue.length === 0) throw new Error(`No pending ${method} call`);
    const next = queue.shift()!;
    if (next.success) next.success(payload);
  }

  function failRunCall(method: string, error: { message: string }): void {
    const queue = pendingByMethod.get(method);
    if (!queue || queue.length === 0) throw new Error(`No pending ${method} call`);
    const next = queue.shift()!;
    if (next.failure) next.failure(error);
  }

  return {
    document,
    window: window as Window & typeof globalThis,
    dispatchRunCall,
    failRunCall,
    runCalls,
  };
}
```

**Why the Proxy:** the chain shape `.withSuccessHandler(s).withFailureHandler(f).METHOD(args)` can be invoked in any order (Search uses success-then-failure, Eviction uses success-then-failure; Bank-Coin's save handler reuses the same shape; Admin-Mgmt does too). A Proxy lets the mock intercept arbitrary terminal methods without enumerating every callback name. The planner could also use a hand-rolled fluent object enumerating each method — equally fine — but the Proxy is dryer.

**Canonical happy + error assertions per sidebar (D-03):**

| Sidebar | Happy path test | Error path test |
|---------|-----------------|-----------------|
| Search | mount HTML → `dispatchRunCall('getSearchInitialData', {chars:[...], slots:[...], recent:['q1']})` → assert `#charSel` has options + `#recentList` shows `q1` button | mount → `failRunCall('getSearchInitialData', {message:'boom'})` → assert `#results` `.error` element renders `"Search failed: boom"` |
| Eviction | mount → `dispatchRunCall('getEvictionEmails', ['a@x','b@x'])` → assert `#emailSel` options populate | mount → `failRunCall('getEvictionEmails', {message:'unauth'})` → assert `#msg.error` shows "Eviction failed: unauth. No changes were written." |
| Bank-Coin | mount → `dispatchRunCall('getBankCoinForForm', {pp:100,gp:50,sp:25,cp:0})` → assert `#pp` input value `=== '100'` + `#saveBtn` enabled | mount → `failRunCall('getBankCoinForForm', {message:'denied'})` → assert `#msg` color is `#c00` + textContent `"Failed to load: denied"` |
| Char-Info | mount → `dispatchRunCall('getCharsForForm', [{char_name:'X',class:'SHD',level:60,race:'IKS'}])` → assert `#charBody tbody` has a row with `<td>X</td>` + class select value `'SHD'` | mount → `failRunCall('getCharsForForm', {message:'fail'})` → assert `#msg` color `#c00` + textContent `"Failed to load: fail"` |
| ~~Theme Picker~~ | *(NOT a sidebar — see §Theme Picker identity)* | — |
| Admin-Mgmt | **ALREADY HAS adminMgmtSidebar.test.ts** — server-side covered. Inline-JS test could be added but D-04 helper makes it ~20 lines if planner wants 6 test files instead of 4. | Same |

**Recommendation:** Plan 08-02 ships 4 NEW sidebar test files (Search, Eviction, Bank-Coin, Char-Info). Admin-Mgmt's existing test file is the trigger-call shape, NOT the JSDOM shape — Phase 8 SUMMARY should note this as a follow-up opportunity for v1.1 if the planner wants the inline-JS layer covered for that sidebar too, but per D-03 the existing test already locks the contract.

### PropertiesService migration mechanics (SEARCH-05 → Plan 08-03)

**Current state (`searchIndex.ts:355-371`):**
```typescript
export function getRecentSearches(): string[] {
  const cache = CacheService.getDocumentCache();
  if (!cache) return [];
  const raw = cache.get(KEY_RECENT);
  if (!raw) return [];
  try { return JSON.parse(raw) as string[]; } catch { return []; }
}

export function pushRecentSearch(query: string): void {
  const q = (query || '').trim();
  if (!q) return;
  const cache = CacheService.getDocumentCache();
  if (!cache) return;
  const current = getRecentSearches().filter((x) => x !== q);
  const next = [q, ...current].slice(0, RECENT_LIMIT);
  cache.put(KEY_RECENT, JSON.stringify(next), CACHE_TTL_SECONDS);
}
```

**Target state (Plan 08-03):**
```typescript
export function getRecentSearches(): string[] {
  const props = PropertiesService.getUserProperties();
  if (!props) return [];
  const raw = props.getProperty(KEY_RECENT);
  if (!raw) return [];
  try { return JSON.parse(raw) as string[]; } catch { return []; }
}

export function pushRecentSearch(query: string): void {
  const q = (query || '').trim();
  if (!q) return;
  const props = PropertiesService.getUserProperties();
  if (!props) return;
  const current = getRecentSearches().filter((x) => x !== q);
  const next = [q, ...current].slice(0, RECENT_LIMIT);
  props.setProperty(KEY_RECENT, JSON.stringify(next));
}
```

Changes: 2 lines per function (cache call → props call; `cache.put(key, val, TTL)` → `props.setProperty(key, val)` — TTL arg drops). KEY_RECENT, RECENT_LIMIT, and the JSON encoding shape are unchanged.

**PropertiesService mock in `test-helpers.ts`:** The existing mock at `test-helpers.ts:292-298` ALREADY implements `getDocumentProperties()` with `getProperty/setProperty/deleteProperty` semantics. Plan 08-03 must extend it to also implement `getUserProperties()` (and ideally `getScriptProperties()` for completeness — the test surface only needs `getUserProperties`, but adding all three guards against future tests reaching for the others).

Recommended shape (additive to existing mock):
```typescript
// In test-helpers.ts (extend the existing PropertiesService block at L292)
const propsApi = {
  getProperty: (k: string) => state.properties.get(k) ?? null,
  setProperty: (k: string, v: string) => { state.properties.set(k, v); },
  deleteProperty: (k: string) => { state.properties.delete(k); },
};
(globalThis as Record<string, unknown>).PropertiesService = {
  getDocumentProperties: () => propsApi,
  getUserProperties: () => propsApi,
  getScriptProperties: () => propsApi,
};
```

All three scopes share `state.properties` — the existing `MockState.properties: Map<string, string>` is reused. Tests that need per-scope isolation can introduce that complexity later; YAGNI for SEARCH-05.

**Existing `searchIndex.test.ts` test rewire:** Tests 16-17 (`pushRecentSearch` / `getRecentSearches` rolling-3 + dedupe) at `searchIndex.test.ts:297-308` currently exercise the cache-backed implementation. Post-08-03 they exercise the properties-backed implementation, BUT because the existing test calls go through `pushRecentSearch` / `getRecentSearches` (not the underlying primitives) and the JSON shape is identical, the test bodies do NOT change. Only the seed/assertion side that touches `state.cache.has('squirebot:search:recent')` would need to read `state.properties.get('squirebot:search:recent')` instead — verify no such test currently exists (none surfaced in the grep, but planner should confirm).

**Quota safety check:** 3 queries × ~50 chars × JSON overhead ≈ 200 bytes. Per the official Apps Script docs + Tanaike's specification report:
- Per-value: 8,066 bytes (Tanaike, verified) — the official "9 KB per value" is the rounded marketing number.
- Per-scope: 524,288 bytes total (512 KB).
- 200 bytes / 524,288 bytes ≈ 0.038% — ~2,500x headroom on a per-property basis, ~25,000x on a per-scope basis.

No bounded-size guard needed. The `.slice(0, RECENT_LIMIT)` cap already caps growth at 3 entries.

**D-06 edge-case re-check (CONFIRMED no dual-write protection needed):** I reviewed the search sidebar flow end-to-end. `pushRecentSearchCall(q)` is invoked by the inline-JS handler at `showSearchSidebar.ts:206` — fire-and-forget, ~once per search, no retry loop, no queued-trigger replay. The CacheService TTL is 25 minutes. After 25 minutes the cache evicts silently and the next `getRecentSearches()` returns `[]`. There is no recoverable path through which the OLD cache state would be needed AFTER the new property-backed read returns its own state — they're disjoint by design. D-06 clear-and-replace is the correct semantics; dual-write would protect against nothing.

### SUMMARY.md template fidelity (DOC-04 → Plan 08-04)

**Verified across all 5 Phase 5 SUMMARY.md files** (`05-01` through `05-05`). The frontmatter shape is:

```yaml
---
phase: 05-search-onboarding-privacy-polish      # string: phase-dir-name
plan: 01                                          # string or number, zero-padded to 2 digits
subsystem: apps-script-housekeeping-and-healthcheck  # string: kebab-case subsystem label (NEW per plan)
tags: [apps-script, housekeeping, schema-healthcheck, ops-06, range-protect, system-tab-hide]
                                                  # array of kebab-case strings; first ~2 are categorical
                                                  # (apps-script / docs / watcher), remainder are topical
requires:                                         # array of strings, one per upstream plan/phase
  - 04-04 (protectBankCoinCells, monitorCellCount, installTriggers 7-trigger baseline)
  - 03 (sheet-helpers.ts: readMetaRows / writeMetaRow / getActiveSpreadsheet)
provides:                                         # array of quoted strings — each = a shippable artifact summary
  - "OPS-06: weekly Sun-03:00 PT schema healthcheck …"
  - "protectBankToonName: warning-only Range.protect …"
affects:                                          # array of quoted strings — downstream-plan impact
  - "Phase 5 plan 05-02 (archive lib extends migrations.ts …)"
tech-stack:                                       # nested object: { added: [], patterns: [] }
  added: []                                       # array — NEW external runtime deps; usually [] for pure code changes
  patterns:                                       # array of quoted strings — patterns added/cemented
    - "Tab-by-ID verification (vs. tab-by-name): …"
key-files:                                        # nested object: { created: [], modified: [] }
  created:
    - apps-script/src/triggers/weeklySchemaHealthcheck.ts (82 lines)
    - apps-script/src/__tests__/weeklySchemaHealthcheck.test.ts (121 lines)
    - .planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md (this file)
  modified:
    - apps-script/src/lib/migrations.ts (+72 lines; 2 new exported helpers + 1 const)
    - apps-script/src/triggers/installTriggers.ts (+17/-6 lines; …)
decisions:                                        # array of quoted strings — major decisions / deviations / rationale
  - "Plan executed exactly as written -- no deviations from the action text."
  - "EXPECTED_TABS is the 13-tab list …"
metrics:                                          # nested object: free-form key-value
  duration: ~20min
  completed: 2026-05-11T04:34Z
  tasks_completed: 3 of 3
  commits: 3 (c85586b feat helpers, ae57d61 feat trigger, a9562b6 feat wiring)
  files_changed: 9 (2 created + 7 modified, ~330 lines added)
  tests_added: 15
  trigger_count_after: 8
  schema_version_after: 3
  watcher_rebuild_required: false
---
```

**Body shape** (Markdown after frontmatter):

```markdown
# Phase 5 Plan 01: Schema Healthcheck + Tab Hide + Bank-Toon-Name Protection Summary

**One-liner:** [1-sentence executive summary]

## What shipped

### Task 1 -- [name] (commit `<sha>`)

[Per-task narrative]

### Task 2 -- ...

## Threat-register coverage  (optional — only if the plan had a threat model)

## Deviations from Plan

[Either "None. Plan executed exactly as written." or a numbered list]

## Schema impact

**Path A confirmed.** [or "Schema bumped to N. Migration covered by …"]

## Verification log

```
[Greppable proof commands + their output]
```

## Self-Check: PASSED

**Files exist (all N changed):**
- FOUND: `path` (with grep-anchor)
- ...

**Commits exist:**
- FOUND: `<sha>` -- <subject>

## Next plan

`/gsd-execute-phase N` will spawn ...
```

**Variance across Phase 5 SUMMARYs (verified):**

| Field | Variance observed | Implication for DOC-04 |
|-------|------------------|------------------------|
| `tech-stack.added` | `[]` in 05-01/02/03/04; populated only in 05-05 (Jekyll Pages) | Most Phase 3/4 backfills will be `[]`; 03-01 added clasp + esbuild + @types so it WILL have entries |
| `requires` | Always present; range from 1 to 4 entries | Backfill should walk upstream plans honestly |
| `affects` | Always present even if just "Phase X plan Y-Z gets …" | Look at downstream plans in the same phase + the next phase |
| `## Threat-register coverage` | Present in 05-01 (T-05-01-01..06), absent in 05-02 (no threat model in that plan); 05-04 has it | Phase 3 plans likely don't have threat models; Phase 4 plans may. Omit the section if no `<threat_model>` exists in the source PLAN.md |
| Commit SHA format | Always real (verifiable via `git log`) | Backfill must use real SHAs — git-log mining is load-bearing |

**Source-of-truth feeders for the 8 backfills** (verified existence):
- 8 `*-PLAN.md` files: `03-01-PLAN.md` through `03-04-PLAN.md` + `04-01-PLAN.md` through `04-04-PLAN.md` (all confirmed exist)
- `.planning/milestones/v1.0-ROADMAP.md` (chronological execution log per CONTEXT line 144)
- Git log filtered by `--grep="03-0[1-4]"` and `--grep="04-0[1-4]"` (commit SHAs feed `key-files.modified` line counts via `git show --stat <sha>`)
- `.planning/STATE.md` (Phase 3/4 "Last Session Summary" entries, if not pruned)

**Acceptance gates for Plan 08-04 (grep-friendly):**
```bash
# 8 SUMMARY.md files exist
for n in 01 02 03 04; do test -f ".planning/phases/03-apps-script-enrichment-foundation/03-$n-SUMMARY.md"; done
for n in 01 02 03 04; do test -f ".planning/phases/04-differentiator-features/04-$n-SUMMARY.md"; done

# Each has the locked frontmatter keys (spot-check)
for f in .planning/phases/0{3,4}-*/0{3,4}-0*-SUMMARY.md; do
  grep -q "^phase:" "$f" && grep -q "^plan:" "$f" && grep -q "^provides:" "$f" && \
  grep -q "^key-files:" "$f" && grep -q "^decisions:" "$f" && grep -q "^metrics:" "$f" || \
  { echo "FAIL: $f missing frontmatter keys"; exit 1; }
done
```

## Theme Picker Sidebar Identity (Critical Finding)

**Resolution:** Theme Picker is NOT a sidebar.

**Evidence:**
- `grep -l "SpreadsheetApp.getUi().showSidebar" apps-script/src/triggers/*.ts` returns 5 files: `showSearchSidebar.ts`, `showEvictionSidebar.ts`, `showBankCoinSidebar.ts`, `showCharInfoSidebar.ts`, `showAdminMgmtSidebar.ts`.
- The Theme Picker entry point is `showThemePickerModal()` in `apps-script/src/triggers/onOpen.ts:52-77`. It calls `SpreadsheetApp.getUi().showModalDialog(html, 'SquireBot — Theme')` — a MODAL DIALOG, not a sidebar.
- No file named `showThemePickerSidebar.ts` exists; no `triggers/themePicker.ts` exists either.
- Phase 5 SUMMARY 05-05 confirms the v1.0 picker is the modal-dialog form (the polished 6-tile picker was deferred to v1.0.x polish; the shipped picker IS the modal in onOpen.ts).

**Why REQUIREMENTS.md TEST-02 calls it a sidebar:** historical. REQUIREMENTS.md was authored before Phase 7 shipped Admin-Mgmt as the 5th sidebar; the "5 sidebars" count was inferred from an early roadmap that grouped the modal Theme Picker with the four real sidebars for round-number aesthetic. The CONTEXT.md spec confirms 5 sidebars and names them Search/Eviction/Bank-Coin/Char-Info/Theme-Picker; the truth is Search/Eviction/Bank-Coin/Char-Info/Admin-Mgmt.

**Recommendation for Plan 08-02:**

| Sidebar | Test file plan | Status |
|---------|---------------|--------|
| Search | NEW: `apps-script/src/__tests__/searchSidebar.test.ts` | Net-new — happy + error |
| Eviction | NEW: `apps-script/src/__tests__/evictionSidebar.test.ts` | Net-new — happy + error |
| Bank-Coin | NEW: `apps-script/src/__tests__/bankCoinSidebar.test.ts` | Net-new — happy + error |
| Char-Info | NEW: `apps-script/src/__tests__/charInfoSidebar.test.ts` | Net-new — happy + error |
| Admin-Mgmt | EXISTS: `apps-script/src/__tests__/adminMgmtSidebar.test.ts` (7 cases, trigger-call layer) | Optionally extend with JSDOM happy+error tests; SKIP for v1.0.1 |
| ~~Theme Picker~~ | Document in 08-02 SUMMARY as "not a sidebar — modal in onOpen.ts; tested informally via the manual smoke runbook" | Skip in scope |

**Total net-new test files: 4.** This matches the 4-plans-2-waves decision (08-02 ships exactly 4 new files, one per net-new sidebar; existing `adminMgmtSidebar.test.ts` is unmodified). The 08-02 SUMMARY frontmatter should `provides: "5 of 5 shipping sidebars have a vitest companion (4 net-new + 1 existing from Phase 7)"` and note in `decisions:` that "Theme Picker named in REQUIREMENTS.md TEST-02 is a modal dialog, not a sidebar — Admin-Mgmt (Phase 7) is the actual 5th sidebar."

**Optional extension (planner's call):** If the planner wants to be thorough, Plan 08-02 could ALSO add a `themePicker.test.ts` covering the modal's inline `apply(key)` handler. The helper supports modals just as well as sidebars (they're both inline HTML+JS strings rendered into an iframe). Recommend NOT including in v1.0.1 — it adds a 5th test file plus the `google.script.host.close()` mock surface that mountSidebar otherwise wouldn't need to solve. Defer to v1.1.

## Plan Structure Validation (D-08 Dependency Graph)

**Confirmed Wave-1 / Wave-2 split:**

```
Wave 1 (parallel, autonomous):
├── 08-01: TEST-01 — vitest.config.ts + mountSidebar helper + jsdom install
│         Touches: apps-script/vitest.config.ts (new), apps-script/package.json (jsdom dep),
│                  apps-script/src/__tests__/test-helpers.ts (mountSidebar export + extended PropertiesService mock)
│
├── 08-03: SEARCH-05 — PropertiesService swap
│         Touches: apps-script/src/lib/searchIndex.ts (L355-371), apps-script/src/__tests__/searchIndex.test.ts (no body changes expected)
│
└── 08-04: DOC-04 — 8 retroactive SUMMARY.md files
          Touches: .planning/phases/03-*/03-0{1,2,3,4}-SUMMARY.md (4 new), .planning/phases/04-*/04-0{1,2,3,4}-SUMMARY.md (4 new)

Wave 2 (depends_on: [08-01]):
└── 08-02: TEST-02 — 4 net-new sidebar test files
          Touches: apps-script/src/__tests__/{search,eviction,bankCoin,charInfo}Sidebar.test.ts (4 new)
          DEPENDS: mountSidebar helper from 08-01
```

**Cross-plan file overlap analysis (Wave 1):**

| File | 08-01 | 08-03 | 08-04 |
|------|-------|-------|-------|
| `vitest.config.ts` | NEW | — | — |
| `package.json` | MOD (jsdom dep) | — | — |
| `test-helpers.ts` | MOD (mountSidebar + getUserProperties mock) | — | — |
| `searchIndex.ts` | — | MOD (lines 355-371) | — |
| `searchIndex.test.ts` | — | MOD (mock seed changes, body unchanged) | — |
| `.planning/phases/03-*/*-SUMMARY.md` | — | — | NEW (4 files) |
| `.planning/phases/04-*/*-SUMMARY.md` | — | — | NEW (4 files) |

**Verdict:** Zero overlap. Wave 1 parallel-safe. ✅

**Concern (resolved):** Plan 08-03 EXTENDS `test-helpers.ts` if the planner wants to add a PropertiesService mock there. **Resolution:** the existing mock at `test-helpers.ts:292-298` already implements `getDocumentProperties()`. Plan 08-01 should be the canonical place to extend the mock (adding `getUserProperties()` + `getScriptProperties()` aliases — see §PropertiesService section) since 08-01 already touches test-helpers.ts. Plan 08-03 then does NOT need to touch test-helpers.ts; the mock surface is ready for it. **This is the safer dependency:** 08-03 becomes `depends_on: [08-01]` for the PropertiesService mock — OR 08-01 lands the mock pre-emptively and 08-03 stays Wave 1.

**Recommended refinement:** Plan 08-01 lands ALL test-helpers.ts changes (mountSidebar + PropertiesService scope aliases). Plan 08-03 then only touches `searchIndex.ts` and the test seed sites in `searchIndex.test.ts`. **08-03 stays Wave 1**, parallel with 08-01, because the mock aliases land in 08-01 BEFORE any 08-03 test runs against them (Wave 1 plans don't actually run their tests against each other's deltas at the same instant — they merge then full-suite runs).

**Slightly safer alternative:** Make 08-03 `depends_on: [08-01]` and move it to Wave 2 with 08-02. Single-extra-wave cost vs. tighter dependency. Planner's call. The CONTEXT D-08 currently locks 08-03 to Wave 1 — recommend honoring that lock + relying on 08-01 to land the mock first within Wave 1 sequence.

## Pitfalls / Risks

**Reviewed `.planning/research/PITFALLS.md` (27 pitfalls).** None of the 27 directly cover JSDOM, vitest, or PropertiesService quota — all 27 are runtime/distribution/scale pitfalls for the v1 watcher+sheet design. PropertiesService is mentioned in 3 places (Pitfalls 9, 10 — as a cursor/ETag store) but never as a quota concern for small payloads.

**New risks surfaced during Phase 8 research:**

1. **JSDOM script execution under vitest's `environment: 'jsdom'`** — vitest's default JSDOM environment does NOT pass `runScripts: 'dangerously'` to the JSDOM constructor. Inline `<script>` tags in HTML inserted via `innerHTML` will NOT execute automatically. The `mountSidebar` helper MUST use the create-script-element-and-set-textContent workaround (documented above). If a plan author skips this workaround, tests will silently fail with "function `init` is not defined" errors. Mitigation: include the workaround pattern in the 08-01 plan's `<action>` block verbatim.

2. **`google.script.run` polyfill must be installed BEFORE the inline scripts execute** — some sidebars (Search at minimum) call `google.script.run.…getSearchInitialData()` at the top of their `init()` function which runs immediately on DOM load. If `window.google` isn't defined before the script executes, the call site throws. mountSidebar must install the mock BEFORE re-creating script elements. The pattern documented above does this correctly (step 4 before step 5).

3. **`window.confirm` in Eviction sidebar** — `showEvictionSidebar.ts:344` calls `window.confirm(body)` before invoking `commitEviction`. JSDOM ships a default `window.confirm` that returns `false`. The Eviction happy-path test must stub `window.confirm` to return `true` BEFORE the click handler fires, otherwise the test mounts the sidebar, dispatches `getEvictionEmails`, dispatches `previewEviction`, clicks the button — and confirm() returns false, no commitEviction call enqueued, assertion fails. Mitigation: 08-02 plan's Eviction test scaffolding must include `vi.spyOn(window, 'confirm').mockReturnValue(true)`.

4. **Bank-Coin + Char-Info do NOT export `buildSidebarHtml`** — `showBankCoinSidebar.ts:77` and `showCharInfoSidebar.ts` both have `function buildSidebarHtml(): string` as non-exported. To mount their HTML in tests, the planner has two options:
   - (a) Rename to export: `export function buildSidebarHtml(): string` — 1-line code change, low risk, but it's a minor signature change.
   - (b) Read the SIDEBAR_BODY string via filesystem in the test — gross, brittle.
   - **Recommended: option (a).** Both functions are already exported via `Code.ts` re-export (or about to be). Adding `export` to the local function declaration is a non-breaking change for production callers (still globally available via the trigger). 08-02 plan should bundle a "rename internal helpers to exported" task as a prerequisite for the Bank-Coin and Char-Info tests.

5. **CACHE_TTL_SECONDS constant becomes orphaned after 08-03** — `searchIndex.ts:26` defines `CACHE_TTL_SECONDS = 60` used by `pushRecentSearch` (this 25-min recent-MRU TTL) AND `buildInvCache` (the 60s inv-cache TTL — same constant! see line 26 vs the 25-min comment elsewhere). Wait — `CACHE_TTL_SECONDS` is 60, not 1500 (25min). Re-reading: the recent-MRU also used `CACHE_TTL_SECONDS = 60`? Let me re-verify. **Verified:** `searchIndex.ts:370` calls `cache.put(KEY_RECENT, JSON.stringify(next), CACHE_TTL_SECONDS)` — 60 seconds, not 25 minutes. The 25-min figure quoted in CONTEXT.md and REQUIREMENTS.md is the CacheService default cap; the actual TTL set by `pushRecentSearch` is 60 seconds, the SAME constant used by `buildInvCache`. **Implication:** the constant CANNOT be deleted — `buildInvCache` still needs it. Plan 08-03 keeps the constant and only changes the two recent-MRU functions to use PropertiesService. Document this in the 08-03 SUMMARY's `decisions:` block.

## Acceptance Criteria Handoff

Concrete grep gates + npm test commands the planner should embed in each Plan's `<verification>` block:

### Plan 08-01 (TEST-01)
```bash
# vitest.config.ts exists with JSDOM env
test -f apps-script/vitest.config.ts
grep -q "environment: 'jsdom'" apps-script/vitest.config.ts

# jsdom declared as devDep
grep -q '"jsdom"' apps-script/package.json

# mountSidebar helper exported from test-helpers.ts
grep -q "^export function mountSidebar" apps-script/src/__tests__/test-helpers.ts

# PropertiesService mock supports getUserProperties (extension landed)
grep -q "getUserProperties" apps-script/src/__tests__/test-helpers.ts

# Full suite still green
cd apps-script && npm test
# Expected: 324+/324+ green (Phase 7 baseline) -- unchanged since 08-01 adds infra, no test cases
```

### Plan 08-02 (TEST-02)
```bash
# 4 net-new sidebar test files exist
test -f apps-script/src/__tests__/searchSidebar.test.ts
test -f apps-script/src/__tests__/evictionSidebar.test.ts
test -f apps-script/src/__tests__/bankCoinSidebar.test.ts
test -f apps-script/src/__tests__/charInfoSidebar.test.ts

# Each has at least 2 it() cases (D-03)
for f in apps-script/src/__tests__/{search,eviction,bankCoin,charInfo}Sidebar.test.ts; do
  n=$(grep -c "^\s*it(" "$f")
  test "$n" -ge 2 || { echo "FAIL: $f has only $n it() cases"; exit 1; }
done

# All 5 shipping sidebars have a sidebar-companion test (4 new + 1 existing)
ls apps-script/src/__tests__/*Sidebar.test.ts | wc -l  # expect 5

# Bank-Coin and Char-Info now export buildSidebarHtml
grep -q "^export function buildSidebarHtml" apps-script/src/triggers/showBankCoinSidebar.ts
grep -q "^export function buildSidebarHtml" apps-script/src/triggers/showCharInfoSidebar.ts

# Full suite green
cd apps-script && npm test
# Expected: 324+8/324+8 green (4 new files × happy+error = 8 new tests; planner may add the optional 3rd)
```

### Plan 08-03 (SEARCH-05)
```bash
# getRecentSearches and pushRecentSearch use PropertiesService
grep -A 5 "^export function getRecentSearches" apps-script/src/lib/searchIndex.ts | grep -q "PropertiesService.getUserProperties"
grep -A 8 "^export function pushRecentSearch" apps-script/src/lib/searchIndex.ts | grep -q "PropertiesService.getUserProperties"

# CacheService removed from those two functions only (NOT from buildInvCache)
grep -A 8 "^export function pushRecentSearch" apps-script/src/lib/searchIndex.ts | grep -qv "CacheService.getDocumentCache"
grep -A 5 "^export function getRecentSearches" apps-script/src/lib/searchIndex.ts | grep -qv "CacheService.getDocumentCache"

# CACHE_TTL_SECONDS still defined (buildInvCache still needs it)
grep -q "^const CACHE_TTL_SECONDS = 60" apps-script/src/lib/searchIndex.ts

# Tests 16, 17 in searchIndex.test.ts still pass without body changes
cd apps-script && npm test searchIndex
# Expected: 24/24 pass (was 24/24; no new tests, mock-seed shifted from state.cache.has to state.properties.get)

# Schema version unchanged
grep -q "writeMetaRow('_meta', 'schema_version', '3')" apps-script/src/lib/migrations.ts
grep -q "WatcherMaxSchemaVersion.*=.*3" internal/sheet/client.go
```

### Plan 08-04 (DOC-04)
```bash
# 8 retroactive SUMMARY.md files exist
for n in 01 02 03 04; do
  test -f ".planning/phases/03-apps-script-enrichment-foundation/03-$n-SUMMARY.md" || exit 1
  test -f ".planning/phases/04-differentiator-features/04-$n-SUMMARY.md" || exit 1
done

# Each has the locked frontmatter keys
for f in .planning/phases/{03-apps-script-enrichment-foundation,04-differentiator-features}/0{3,4}-0{1,2,3,4}-SUMMARY.md; do
  for key in phase plan subsystem tags requires provides affects tech-stack key-files decisions metrics; do
    grep -q "^${key}:" "$f" || { echo "FAIL: $f missing $key"; exit 1; }
  done
done

# Commit SHAs in each SUMMARY are real (git verifiable)
for f in .planning/phases/{03,04}-*/0{3,4}-0*-SUMMARY.md; do
  for sha in $(grep -oE "[a-f0-9]{7}" "$f" | sort -u); do
    git cat-file -e "$sha" 2>/dev/null || echo "WARN: $f references unknown SHA $sha"
  done
done
```

## Sources

### Primary (HIGH confidence — verified in repo)
- `apps-script/package.json` — vitest ^1.6.0, no jsdom devDep (verified `npm ls jsdom` empty)
- `apps-script/src/__tests__/test-helpers.ts` — existing PropertiesService mock at L292; getDocumentProperties only
- `apps-script/src/triggers/{showSearchSidebar,showEvictionSidebar,showBankCoinSidebar,showCharInfoSidebar,showAdminMgmtSidebar}.ts` — 5 actual sidebars verified
- `apps-script/src/triggers/onOpen.ts:52-77` — `showThemePickerModal` confirmed as MODAL, not sidebar
- `apps-script/src/lib/searchIndex.ts:355-371` — SEARCH-05 surgical-change site, CACHE_TTL_SECONDS=60 confirmed
- `.planning/phases/05-search-onboarding-privacy-polish/05-{01,02,04,05}-SUMMARY.md` — D-07 template fidelity verified across 4 Phase 5 SUMMARYs
- `.planning/phases/0{3,4}-*/` directory listings — 0 existing SUMMARY.md files in either phase dir (8 missing confirmed)
- `apps-script/src/__tests__/adminMgmtSidebar.test.ts` — closest analog test shape for 08-02

### Secondary (HIGH confidence — verified via official docs)
- [Test Environment | Vitest v1.6](https://v1.vitest.dev/guide/environment) — `environment: 'jsdom'` global config syntax
- [Configuring Vitest | Vitest v1.6](https://v1.vitest.dev/config/) — `defineConfig` shape; jsdom as optional dep
- [Properties Service | Apps Script](https://developers.google.com/apps-script/guides/properties) — 9KB/value, 500KB/scope official limits
- [Class PropertiesService | Apps Script](https://developers.google.com/apps-script/reference/properties/properties-service) — `getUserProperties()` / `getProperty()` / `setProperty()` API
- [Quotas for Google Services | Apps Script](https://developers.google.com/apps-script/guides/services/quotas) — daily quota tables

### Tertiary (MEDIUM confidence — single source, secondary-verified)
- [Tanaike, Report: Specification of Properties Service](https://medium.com/google-cloud/report-specification-of-properties-service-for-google-apps-script-198c487f3896) — empirically-measured limits (8,066 bytes/value, 524,288 bytes/scope); cross-checked against Google's marketing-rounded "9 KB" / "500 KB" official doc
- [Ghinda, Run script tags in innerHTML content](https://www.ghinda.net/article/script-tags/) — canonical workaround for the JSDOM script-execution gotcha; cross-checked against [JSDOM issue #426](https://github.com/jsdom/jsdom/issues/426)
- npm registry — `npm view jsdom version` returns `29.1.1` as of 2026-05-12; jsdom 24.x is the version contemporary with vitest 1.6 (April 2024)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | jsdom 24.x is the right version pin for vitest 1.6 — newer (25+, 26+, 29+) may work but are untested in this project | §vitest+JSDOM config | LOW — vitest's optionalDependencies are loose; if 24 breaks the planner can fall forward to 25/26 with no code changes. Tested at 08-01 acceptance gate via `npm test`. |
| A2 | `runScripts` constructor option is NOT set by vitest's `environment: 'jsdom'` (forcing the create-script-element workaround) | §mountSidebar architecture | LOW — verifiable in 5 minutes by running `console.log(typeof window.eval)` in a test; if vitest DID enable runScripts the simpler `innerHTML` path would work too. The workaround is correct either way. |
| A3 | Bank-Coin and Char-Info's `buildSidebarHtml` are NOT exported — must be renamed in 08-02 | §Pitfall #4 | LOW — `grep "^export function buildSidebarHtml" apps-script/src/triggers/showBankCoinSidebar.ts` returns empty (verified during this research). The export-rename task lands in 08-02. |
| A4 | `CACHE_TTL_SECONDS = 60` (not 1500) means the constant survives SEARCH-05 | §Pitfall #5 | LOW — verified by direct read of `searchIndex.ts:26`. The CONTEXT.md "25-min TTL" framing is the user-facing TTL of CacheService's default eviction, not the explicit TTL passed to `cache.put`. |
| A5 | The 8 retroactive SUMMARY.md files can be authored from existing artifacts (PLAN.md + git log + v1.0-ROADMAP.md) without re-running Phase 3/4 tests | §SUMMARY.md template fidelity | LOW — the SUMMARY.md is documentation, not verification. The Verification log section in each can reference the existing test counts from STATE.md "Phase N closed" entries or from `git log --grep="(03-0N)"` commit messages. |

## Open Questions

None blocking. Two design notes for the planner:

1. **Should Plan 08-02 also test the Theme Picker modal?** Recommended NO for v1.0.1 (adds `google.script.host.close()` mock surface; modal closure semantics differ from sidebar persistence). Defer to v1.1.

2. **Should Plan 08-03 move to Wave 2 to be explicit about depending on Plan 08-01's PropertiesService mock alias?** Recommended NO — Wave 1 parallelism is safe because mock aliases land in 08-01's commit BEFORE 08-03's tests run, and the wave-boundary merge runs `npm test` once across all 3 Wave-1 plans together (catching any ordering issue).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | vitest, esbuild | ✓ (CI uses Node LTS) | — | — |
| npm | install jsdom | ✓ | — | — |
| vitest | npm test | ✓ (1.6.0) | 1.6.0 | — |
| jsdom | TEST-01 | ✗ | — | None — Plan 08-01 must install |
| @types/google-apps-script | typecheck | ✓ (1.0.91) | 1.0.91 | — |
| git | DOC-04 (SHA mining for SUMMARY frontmatter) | ✓ | — | — |
| Apps Script PropertiesService | SEARCH-05 (production runtime) | n/a — verified by `@types/google-apps-script` declarations | — | — |
| clasp | Ship-gate clasp push | ✓ (2.4.2) | 2.4.2 | — |

**Missing dependencies with no fallback:**
- jsdom — Plan 08-01 installs as part of TEST-01

**Missing dependencies with fallback:** None.

## Project Constraints (from CLAUDE.md)

- **Apps Script source location:** `apps-script/src/` (libs in `lib/`, triggers in `triggers/`, tests in `__tests__/`, fixtures in `__fixtures__/`) — Plan 08-02's 4 new test files land in `apps-script/src/__tests__/` per convention. ✅
- **Build / test commands:** `npm run build` (esbuild → `dist/Code.js`), `npm test` (vitest run), `npm run typecheck` (tsc --noEmit). All three must remain green at every Plan acceptance gate. ✅
- **Source-of-truth for theme palettes:** `docs/design/eq-aesthetic-theme.md` → `THEMES` registry in `lib/themes.ts`. Tests can use any THEMES entry as fixture data (e.g., `THEMES['vanilla']` or `THEMES['sheets-default']` for the null/no-token path). ✅
- **Schema evolution rule:** extend-only, version-stamped, idempotent; `_meta.schema_version` write is LAST in migrations. **Phase 8 has zero schema impact** — no migrations.ts changes, `_meta.schema_version` stays at 3, `WatcherMaxSchemaVersion` stays at 3 (CONTEXT D-08 ship-gate confirms). ✅
- **WatcherMaxSchemaVersion bump rule:** must be bumped before migration ships. **Not triggered** — no migration ships in Phase 8. ✅
- **Test fixture naming:** real-name files when from real character; generic-name otherwise. Plan 08-02's sidebar tests use synthetic data (`'admin@example.com'`, `'X'` char names) — no fixture files needed; inline test data is fine. ✅
- **Structured logging:** Go (slog) + Apps Script (`log(level, op, fields)` JSON helper). Tests do not change logging contract; the existing inline-JS error handlers already render structured error messages into the DOM. ✅
- **GSD Workflow Enforcement:** This research is being authored via `/gsd-research-phase` (or `/gsd-plan-phase` → research) — within-workflow. ✅

## Metadata

**Confidence breakdown:**
- vitest + JSDOM config: HIGH — verified against vitest 1.6 official docs + npm registry + repo state (package.json read)
- mountSidebar architecture: HIGH — verified against JSDOM issue #426 + Ghinda canonical workaround + all 5 sidebars' actual `google.script.run` chain shape via grep
- PropertiesService migration: HIGH — verified API surface (`getProperty`/`setProperty`) used in searchIndex.ts plus type definitions from `@types/google-apps-script ^1.0.91`; quota numbers cross-checked against Google official + Tanaike empirical report
- SUMMARY.md template: HIGH — read all 5 Phase 5 SUMMARYs; field inventory + variance documented
- Theme Picker identity: HIGH — direct file inspection confirmed modal, not sidebar
- Plan structure (D-08): HIGH — dependency graph verified by reading source files

**Research date:** 2026-05-12
**Valid until:** ~2026-08-12 (90 days — vitest 1.6 and Apps Script PropertiesService are stable; jsdom may have a major version bump in that window but vitest's optional-dep relationship makes that low-impact)

## RESEARCH COMPLETE
