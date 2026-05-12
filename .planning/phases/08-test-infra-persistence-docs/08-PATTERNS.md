# Phase 8: Test Infra + Persistence + Docs Backfill — Pattern Map

**Mapped:** 2026-05-12
**Files analyzed:** 17 (1 create config + 1 modify package.json + 1 modify helpers + 5 create sidebar inline tests + 1 modify searchIndex.ts + 1 modify searchIndex.test.ts + 8 create SUMMARY.md)
**Analogs found:** 16 / 17 (vitest.config.ts has no direct in-repo analog; documented from scratch)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `apps-script/vitest.config.ts` (CREATE) | config | build-tool | `apps-script/tsconfig.json` (only sibling config) + `apps-script/build.mjs` (only other build script) | no-analog (NEW pattern) |
| `apps-script/package.json` (MODIFY) | config | build-tool | self (devDependencies block at lines 13-19) | exact (self-extend) |
| `apps-script/src/__tests__/test-helpers.ts` (MODIFY: add `mountSidebar`, `makePropertiesServiceMock`) | utility / test-helper | request-response (mock factory) | self (lines 292-298 PropertiesService stub; lines 430-438 HtmlService stub) | role-match + NEW JSDOM mechanic |
| `apps-script/src/__tests__/showSearchSidebar.inline.test.ts` (CREATE) | test | request-response (DOM events + google.script.run mocks) | `apps-script/src/__tests__/adminMgmtSidebar.test.ts` (shape) + `apps-script/src/__tests__/showSearchSidebar.test.ts` (existing trigger-level tests for same sidebar) | role-match (trigger-level analog) + NEW inline-JS mechanic |
| `apps-script/src/__tests__/showEvictionSidebar.inline.test.ts` (CREATE) | test | request-response | `adminMgmtSidebar.test.ts` + existing `showEvictionSidebar.test.ts` | role-match + NEW mechanic |
| `apps-script/src/__tests__/showBankCoinSidebar.inline.test.ts` (CREATE) | test | request-response | `adminMgmtSidebar.test.ts` + existing `bankCoinSidebar.test.ts` | role-match + NEW mechanic |
| `apps-script/src/__tests__/showCharInfoSidebar.inline.test.ts` (CREATE) | test | request-response | `adminMgmtSidebar.test.ts` + existing `charInfoSidebar.test.ts` | role-match + NEW mechanic |
| `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` (CREATE) | test | request-response | `adminMgmtSidebar.test.ts` (same SUT, different layer) | role-match + NEW mechanic |
| `apps-script/src/lib/searchIndex.ts` (MODIFY lines 355-371) | service | persistence (cache → properties swap) | self (lines 355-371 current cache impl; lines 292-298 of test-helpers.ts PropertiesService stub shape) | exact (in-file surgical swap) |
| `apps-script/src/__tests__/searchIndex.test.ts` (MODIFY: tests 16-17 at lines 292-309) | test | persistence | self (existing tests 16-17 at lines 296-309) | exact (self-rewire) |
| `.planning/phases/03-apps-script-enrichment-foundation/03-0{1..4}-SUMMARY.md` (CREATE x4) | doc | none (pure markdown) | `.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md` (canonical template) | exact (template clone) |
| `.planning/phases/04-differentiator-features/04-0{1..4}-SUMMARY.md` (CREATE x4) | doc | none (pure markdown) | `.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md` (canonical template) | exact (template clone) |

---

## Pattern Assignments

### `apps-script/vitest.config.ts` (config, build-tool) — NEW PATTERN

**Analog:** None in-repo. The closest sibling files are `apps-script/tsconfig.json` (compiler config) and `apps-script/build.mjs` (esbuild driver). vitest is currently invoked with zero config (`"test": "vitest run"` in `package.json:9`) and relies on default discovery.

**Reference path-alias and `include` shape to mirror from `apps-script/tsconfig.json` lines 13–22:**

```jsonc
{
  "compilerOptions": {
    // ...
    "moduleResolution": "bundler",
    "isolatedModules": true,
    // ...
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "src/__fixtures__"]
}
```

**Required vitest.config.ts shape (NEW — author from scratch per D-01):**

```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',                                 // D-01: global default
    include: ['src/__tests__/**/*.test.ts'],              // mirror tsconfig "include" minus __fixtures__
    exclude: ['node_modules', 'dist', 'src/__fixtures__'],
    globals: false,                                       // vitest 1.6 default; keep explicit-import discipline
  },
});
```

**Why each line:**

- `environment: 'jsdom'` → D-01 locks JSDOM as the default. Sidebar inline tests outnumber non-DOM tests ~5:1 once TEST-02 ships; per-test override (`// @vitest-environment node`) remains available.
- `include` → mirrors the existing default discovery pattern (`src/__tests__/**/*.test.ts`); see the 31-file `__tests__` directory listing for confirmation.
- `exclude` → matches `tsconfig.json:21` (`__fixtures__` is non-runnable JSON/text data).
- `globals: false` → keeps the current explicit-import discipline; every existing test file imports `{ describe, it, expect, ... }` from `'vitest'` (verified: `searchIndex.test.ts:14`, `adminMgmtSidebar.test.ts:15`).

**Dependency add** (`apps-script/package.json:13-19` current state):

```json
"devDependencies": {
  "@google/clasp": "^2.4.2",
  "@types/google-apps-script": "^1.0.91",
  "esbuild": "^0.20.0",
  "typescript": "^5.4.0",
  "vitest": "^1.6.0"
}
```

**Add line:** `"jsdom": "^24.0.0",` (vitest 1.6 peer-compatible; D-01 confirms ~1.5MB ship is acceptable).

---

### `apps-script/src/__tests__/test-helpers.ts` (utility, test-helper) — MODIFY (additive)

**Analog (in-file):** existing PropertiesService stub at lines 292-298 + existing HtmlService stub at lines 430-438.

**Existing PropertiesService stub** (lines 292-298 — note: currently ONLY exposes `getDocumentProperties()`; Phase 8 must extend with `getUserProperties()`):

```typescript
(globalThis as Record<string, unknown>).PropertiesService = {
  getDocumentProperties: () => ({
    getProperty: (k: string) => state.properties.get(k) ?? null,
    setProperty: (k: string, v: string) => { state.properties.set(k, v); },
    deleteProperty: (k: string) => { state.properties.delete(k); },
  }),
};
```

**Pattern to extend** (D-05 contract: `getUserProperties()` returns the same shape). Recommended addition:

```typescript
(globalThis as Record<string, unknown>).PropertiesService = {
  getDocumentProperties: () => ({
    getProperty: (k: string) => state.properties.get(k) ?? null,
    setProperty: (k: string, v: string) => { state.properties.set(k, v); },
    deleteProperty: (k: string) => { state.properties.delete(k); },
  }),
  // Phase 8 plan 08-01: per-user scope mock for SEARCH-05 MRU.
  // Backed by a SEPARATE Map so tests can distinguish user vs document
  // scope; defaults to a shared user-scope Map per test-run.
  getUserProperties: () => ({
    getProperty: (k: string) => state.userProperties.get(k) ?? null,
    setProperty: (k: string, v: string) => { state.userProperties.set(k, v); },
    deleteProperty: (k: string) => { state.userProperties.delete(k); },
  }),
};
```

**MockState extension** needed at lines 23-53:

```typescript
// Append to MockState interface (sibling to existing `properties: Map<string, string>`):
userProperties: Map<string, string>;  // Phase 8 plan 08-03: SEARCH-05 per-user MRU
```

**And in `newMockState()` at lines 444-461:**

```typescript
return {
  // ... existing fields
  properties: new Map(),
  userProperties: new Map(),  // Phase 8 plan 08-01
  // ...
};
```

---

**NEW: `mountSidebar(html: string)` helper** — D-04 contract. NO in-repo analog; document the contract from scratch.

**Input contract** (the SIDEBAR_BODY shape, e.g., `apps-script/src/triggers/showSearchSidebar.ts:133-282`):

The HTML string is the concatenation of:
1. A `<style>` block with `:root` CSS custom properties (theme tokens).
2. A `<div class="sidebar">…</div>` body.
3. An inline `<script>…</script>` block that calls `google.script.run.withSuccessHandler(...).withFailureHandler(...).METHOD_NAME(...)` and mutates DOM elements by ID.

**Critical JSDOM pitfall** (per CONTEXT specifics line 167): JSDOM does NOT auto-execute `<script>` tags injected via `innerHTML`. Standard pattern:

```typescript
export interface MountedSidebar {
  document: Document;
  window: Window;
  // google.script.run mock — fluent builder so .withSuccessHandler().withFailureHandler().METHOD()
  // chain works identically to the live HtmlService surface. Each METHOD call enqueues a pending
  // dispatch; tests resolve via dispatchRunCall(method, response).
  google: {
    script: {
      run: {
        withSuccessHandler: (fn: (v: unknown) => void) => /* fluent */ unknown;
        withFailureHandler: (fn: (e: Error) => void) => /* fluent */ unknown;
        // Plus a method-trap (Proxy) catching any METHOD_NAME(args) call.
      };
    };
  };
  // Per-test dispatcher: invokes the most-recently-registered success/failure
  // handler for `method`. Mirrors the `state.lockTryLockReturn` pattern in
  // test-helpers.ts:285-290 (controllable boolean → callable dispatcher).
  dispatchRunCall(method: string, response: { ok?: unknown; err?: Error }): void;
  // Inspection helpers
  getPendingCalls(): Array<{ method: string; args: unknown[] }>;
}

export function mountSidebar(html: string): MountedSidebar {
  // 1. Set document.body.innerHTML to the HTML string (strips <script> execution).
  document.body.innerHTML = html;

  // 2. Walk the document, extract every <script> textContent into an array,
  //    remove each from the DOM, then re-create as fresh script elements and
  //    append to document.head — JSDOM executes those.

  // 3. BEFORE re-injecting scripts, stub `window.google.script.run` with the
  //    fluent mock + queue-backed dispatcher.

  // 4. Re-inject the scripts (init() at bottom of each sidebar fires immediately;
  //    it calls google.script.run.getXxxData() which lands in the queue).

  // 5. Return { document, window, google, dispatchRunCall, getPendingCalls }.
}
```

**Sentinel verification:** every sidebar's inline `<script>` ends with `init();` or equivalent (e.g., `showSearchSidebar.ts:281`); `mountSidebar` must call this AFTER the `google.script.run` stub is installed, otherwise the init call's `.withSuccessHandler(...)` registration is lost.

**Why this shape** (D-04 rationale): the existing `state.alertCalls` (line 43) + `state.lockTryLockReturn` (line 26) pattern is "capture call, expose deterministic resolution control"; `dispatchRunCall` is the same pattern for async google.script.run callbacks.

---

### Sidebar inline-JS tests (5 new files) — NEW PATTERN with `adminMgmtSidebar.test.ts` as the OUTER shape

**Outer-shape analog** (file-structure, imports, beforeEach, describe nesting): `apps-script/src/__tests__/adminMgmtSidebar.test.ts:1-46` and `:204-219`.

```typescript
// Header comment + imports (clone from adminMgmtSidebar.test.ts:1-22)
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { resetMocks, makeSheet, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml /* or showSearchSidebar */ } from '../triggers/showSearchSidebar';
// + the lib whose callbacks the inline JS calls (searchIndex, archive, etc.)

// installSessionMock pattern from adminMgmtSidebar.test.ts:29-34 (lift verbatim
// when the sidebar reads Session.getEffectiveUser).
function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

describe('showSearchSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
    // + sidebar-specific seed (admin allowlist for eviction; bank_toon for bank-coin; etc.)
  });
  afterEach(() => {
    delete (globalThis as Record<string, unknown>).Session;
  });

  // --- TS1 happy path (D-03 mandatory) ----------------------------------
  it('TS1 — user types query, clicks Search, results render', () => {
    // 1. Build the sidebar HTML
    const html = buildSidebarHtml(/* theme */ null);

    // 2. Mount it into JSDOM via the new helper from Plan 08-01
    const m = mountSidebar(html);

    // 3. Resolve the init call (getSearchInitialData) — sidebar's init() calls
    //    google.script.run.withSuccessHandler(...).getSearchInitialData() at
    //    showSearchSidebar.ts:181-192.
    m.dispatchRunCall('getSearchInitialData', { ok: { chars: ['Findom'], slots: [...], recent: [] } });

    // 4. User types + clicks
    const q = m.document.getElementById('q') as HTMLInputElement;
    q.value = 'bone';
    (m.document.getElementById('searchBtn') as HTMLButtonElement).click();

    // 5. Resolve the runSearch call
    m.dispatchRunCall('runSearch', { ok: {
      groups: [{ itemName: 'Bone Helm', itemId: 1234, collapsed: false, rows: [...], ... }],
      suggestions: [], coldFill: false, durationMs: 12,
    }});

    // 6. Assert post-condition: #results contains the rendered group
    expect(m.document.getElementById('results')!.innerHTML).toContain('Bone Helm');
    expect(m.document.getElementById('results')!.innerHTML).not.toContain('No matches');
  });

  // --- TS2 error path (D-03 mandatory) ----------------------------------
  it('TS2 — runSearch failure renders .error region with message', () => {
    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);
    m.dispatchRunCall('getSearchInitialData', { ok: { chars: [], slots: [], recent: [] } });
    (m.document.getElementById('searchBtn') as HTMLButtonElement).click();
    // submit() short-circuits if q is empty — set a value first
    // ...
    m.dispatchRunCall('runSearch', { err: new Error('CacheService unavailable') });
    expect(m.document.getElementById('results')!.innerHTML).toContain('Search failed');
    expect(m.document.getElementById('results')!.innerHTML).toContain('CacheService unavailable');
  });
});
```

**Per-sidebar specifics** (each new `.inline.test.ts` file needs ≥2 `it(...)` per D-03):

| File | Build-fn export to import | Seeds required (beforeEach) | Happy-path callback to dispatch | Error-path callback |
|------|---------------------------|------------------------------|------------------------------------|---------------------|
| `showSearchSidebar.inline.test.ts` | `buildSidebarHtml(theme)` from `showSearchSidebar.ts:114` (currently private — Plan 08-02 may need to export it OR import the trigger and intercept `HtmlService.createHtmlOutput`) | `_meta` theme; `_char_owner`; `inv:Char` | `getSearchInitialData` then `runSearch` | `runSearch` failure → `.error` region |
| `showEvictionSidebar.inline.test.ts` | parallel build-fn in `showEvictionSidebar.ts` | `_meta.guild_admins` allowlist (Phase 7 admin gate); seed eviction-eligible char | `getEvictionEmails` then `previewEviction` | `commitEviction` failure |
| `showBankCoinSidebar.inline.test.ts` | parallel in `showBankCoinSidebar.ts` | `_meta.bank_toon_name`; bank toon inv tab | `getBankCoinInitialData` then `saveBankCoin` | `saveBankCoin` validation reject |
| `showCharInfoSidebar.inline.test.ts` | parallel in `showCharInfoSidebar.ts` | `_char_owner` row for selected char | `getCharInfo` then `saveCharInfo` | `saveCharInfo` failure |
| `showAdminMgmtSidebar.inline.test.ts` | parallel in `showAdminMgmtSidebar.ts` | `_meta.guild_admins` + `workbook_owner_floor` (clone seed from `adminMgmtSidebar.test.ts:103-111`) | `getAdminList` then `addAdmin` | `addAdmin` `/not_authorized/` error path |

**Existing trigger-level tests STAY** (`showEvictionSidebar.test.ts`, `showSearchSidebar.test.ts`, `bankCoinSidebar.test.ts`, `charInfoSidebar.test.ts`, `adminMgmtSidebar.test.ts`). The new `.inline.test.ts` files are a NEW layer that mounts the rendered HTML; they do not replace the server-side trigger-callback tests.

**File-naming convention warning:** the existing `bankCoinSidebar.test.ts` and `charInfoSidebar.test.ts` files (lines without the `show` prefix) deviate from the `show*Sidebar.test.ts` shape of `showEvictionSidebar.test.ts` / `showSearchSidebar.test.ts` / `adminMgmtSidebar.test.ts`. CONTEXT D-08 names the new files `show*Sidebar.inline.test.ts`; planner should pick the `show…inline.test.ts` shape for all 5 to keep the inline-JS layer trivially greppable as a cohort.

---

### `apps-script/src/lib/searchIndex.ts` (service, persistence) — MODIFY lines 355-371

**Analog (in-file):** the function pair to swap.

**Current state** (lines 353-371 — the surgical-change target):

```typescript
// --- Recent searches ----------------------------------------------------

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

**Target state per D-05/D-06** (clear-and-replace; no dual-write):

```typescript
// --- Recent searches ----------------------------------------------------
// SEARCH-05 (Phase 8 plan 08-03): per-user persistent MRU via
// PropertiesService.getUserProperties(). KEY_RECENT and the JSON-encoded
// string-array shape are unchanged; only the storage backend swaps from
// CacheService.getDocumentCache() (25-min TTL, document-scoped) to
// PropertiesService.getUserProperties() (durable, per-user). D-06 dictates
// no dual-write / no cache backfill -- the 25-min cache window is a
// negligible one-session regression.

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
  props.setProperty(KEY_RECENT, JSON.stringify(next));  // no TTL arg
}
```

**Surgical scope** (do NOT touch):

- **Lines 26-27, 31-34** — `CACHE_TTL_SECONDS = 60`, `MAX_CACHE_VALUE_BYTES`, `KEY_INV`, `KEY_ITEMS_MASTER`, `KEY_PIGPARSE`. These are for per-`inv:Char` cache + enrichment cache — a SEPARATE concern from MRU. `CACHE_TTL_SECONDS` becomes unused by MRU but is still consumed at lines 165, 234, 263, 405. Planner may delete or leave (CONTEXT line 138 Deferred / Claude's Discretion).
- **Line 176** — `prewarmSearchCache`'s `CacheService.getDocumentCache()` call (per-char inv cache; NOT MRU). DO NOT CHANGE.
- **Lines 310-311** — `runSearch`'s `CacheService.getDocumentCache()` call (per-char inv cache + enrichment cache; NOT MRU). DO NOT CHANGE.
- **Lines 355-371** — the ONLY two functions to modify. Verification grep: `grep -n "CacheService.getDocumentCache" apps-script/src/lib/searchIndex.ts` returns **3 hits** before and **2 hits** after Plan 08-03 (176, 310, 311 → 176, 310, 311).

---

### `apps-script/src/__tests__/searchIndex.test.ts` (test, persistence) — MODIFY tests 16-17

**Analog (in-file):** existing tests 16-17 at lines 292-309.

**Current state** (lines 290-310):

```typescript
// ---------------------------------------------------------------------------
// Recent searches
// ---------------------------------------------------------------------------

describe('recent searches', () => {
  beforeEach(() => { resetMocks(); });

  // Test 16 — push + rolling 3
  it('rolls forward in MRU order capped at 3', () => {
    pushRecentSearch('q1');
    pushRecentSearch('q2');
    pushRecentSearch('q3');
    pushRecentSearch('q4');
    expect(getRecentSearches()).toEqual(['q4', 'q3', 'q2']);
  });

  // Test 17 — duplicate suppression
  it('dedupes consecutive duplicate pushes', () => {
    pushRecentSearch('q1');
    pushRecentSearch('q1');
    expect(getRecentSearches()).toEqual(['q1']);
  });
});
```

**Required changes (Plan 08-03):**

1. Tests 16-17 keep their exact public assertions (the API surface is unchanged). The mock wire-up under `resetMocks()` flips automatically once `test-helpers.ts` gains the `getUserProperties()` shape from Plan 08-01.
2. Add a **new persistence-across-resetMocks test** (NOT in CONTEXT but D-05 implies it — the whole point of the swap is durability):

```typescript
// Test 17b — persists across the cache 25-min TTL boundary (D-05/D-06).
// The cache mock at line 394-423 would expire at TTL; PropertiesService has no TTL.
it('persists across simulated 25-min cache-TTL elapse', () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-05-11T00:00:00Z'));
  pushRecentSearch('persistent-query');
  vi.setSystemTime(new Date('2026-05-11T00:30:00Z'));  // +30 min, past old 25-min TTL
  expect(getRecentSearches()).toEqual(['persistent-query']);
  vi.useRealTimers();
});
```

3. **Note (planner): if the existing `state.cache` Map-backed mock at `test-helpers.ts:394-423` is the ONLY consumer of `CacheService.getDocumentCache()` mock, IT MUST STAY — `runSearch` (lines 310-311) and `prewarmSearchCache` (line 176) still consume it. Do NOT delete the cache mock.

---

### `.planning/phases/03-…/03-0{1..4}-SUMMARY.md` and `04-…/04-0{1..4}-SUMMARY.md` (doc, none) — 8 retroactive files

**Canonical analog (template):** `.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md` — VERBATIM frontmatter shape per D-07.

**Frontmatter fields (verbatim, in order, from `05-01-SUMMARY.md:1-57`):**

```yaml
---
phase: <phase-slug>                         # e.g., 03-apps-script-enrichment-foundation
plan: <NN>                                  # e.g., 01
subsystem: <kebab-case-subsystem-name>      # e.g., apps-script-housekeeping-and-healthcheck
tags: [<tag1>, <tag2>, ...]                 # 4-6 short tags; include REQ-IDs like ops-06, view-05
requires:
  - <plan-id> (<short rationale>)           # cross-plan dependency
  - <plan-id> (<short rationale>)
provides:
  - "<REQ-ID>: <one-line summary>"          # double-quoted; one bullet per shipped capability
  - "<helper-name>: <what-it-does>"
  - ...
affects:
  - "<future-plan>: <how-it-extends>"       # forward refs
tech-stack:
  added: []                                 # array of npm packages added in this plan; usually []
  patterns:
    - "<pattern-name>: <explanation> Locked by RESEARCH §<section> / PATTERNS §<section>."
    - ...
key-files:
  created:
    - <path> (<NN lines>)                   # parenthetical line count is conventional
  modified:
    - <path> (<+NN/-NN lines>; <what-changed-summary>)
decisions:
  - "<terse imperative statement of one decision>"
  - ...
metrics:
  duration: <~Nmin or ~Nh>
  completed: <ISO-8601 timestamp>
  tasks_completed: <X of Y>
  commits: <N> (<short-hash short-msg>; <short-hash short-msg>; ...)
  files_changed: <N> (<X created + Y modified>, ~<N>00 lines added)
  tests_added: <N> (<distribution by test-file>)
  trigger_count_after: <N> (was <N-1>; <forward note>)
  schema_version_after: <N> (<unchanged-or-bump note>)
  watcher_rebuild_required: <true|false> (<why>)
---

# Phase <N> Plan <NN>: <Title>

**One-liner:** <single dense sentence summarizing the entire plan's outcome>.

## What shipped

### Task 1 -- <task-name> (commit `<short-hash>`)
<2-4 paragraphs of past-tense narrative; reference exact line numbers + function names where load-bearing>

### Task 2 -- ...

## Threat-register coverage
<bullet per STRIDE item from the plan>

## Deviations from Plan
<None. or itemize.>

## Schema impact
<Path A confirmed. or Path B narrative.>

## Verification log
\`\`\`
$ npm test
Test Files  XX passed (XX)
Tests       XXX passed (XXX)

$ grep -n "<acceptance-grep>" <path>
<expected line:hit>
\`\`\`

## Self-Check: PASSED
**Files exist (all N changed):**
- FOUND: <path> (<sentinel>)
...
**Commits exist:**
- FOUND: <short-hash> -- <commit-msg>
...

## Next plan
<single sentence pointing at what /gsd-execute-phase will spawn next>
```

**Variance across `05-0{2..5}-SUMMARY.md`:**

All five Phase 5 SUMMARY files (`05-01..05-05`) share this exact frontmatter shape verbatim. Variance is content-only (different `provides`, `key-files`, `metrics` values); the field NAMES + ORDER are identical across all five. The planner can clone any of the five as the literal byte-template; `05-01` is the canonical reference.

**Source-of-truth for each backfill's content** (per CONTEXT D-07):

- **Plan files:** `.planning/phases/03-apps-script-enrichment-foundation/03-0{1..4}-PLAN.md` and `.planning/phases/04-differentiator-features/04-0{1..4}-PLAN.md` — provide the `<files_created>` / `<files_modified>` arrays (already at the top of each PLAN.md, verified for `03-01-PLAN.md:7-25`).
- **Chronology:** `.planning/milestones/v1.0-ROADMAP.md` — chronological execution log of v1.0 (the only milestone-level chronicle in repo).
- **Commit shas:** `git log --oneline -- apps-script/` filtered to 2026-04-30 through 2026-05-04 window.

**Verification depth** (per CONTEXT D-07 line 95): existence grep gate on all 8 SUMMARY.md paths + spot-check of `key-files.created` and `decisions` arrays. The 8 docs don't need deep auditability — they need to exist so the milestone-audit's "Phase 3/4 documentation debt" line item retires.

---

## Shared Patterns

### Pattern A — `resetMocks() + seedMeta()` test bootstrap

**Source:** `apps-script/src/__tests__/test-helpers.ts:464-468` (resetMocks) + `:471-474` (seedMeta).

**Apply to:** Every new sidebar inline-JS test file + the existing `searchIndex.test.ts` update.

```typescript
let state: MockState;
beforeEach(() => {
  state = resetMocks();
  seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
});
```

### Pattern B — `installSessionMock` for auth-gated sidebars

**Source:** `apps-script/src/__tests__/adminMgmtSidebar.test.ts:29-34` (verbatim).

**Apply to:** `showAdminMgmtSidebar.inline.test.ts`, `showEvictionSidebar.inline.test.ts` (Phase 7 admin gate); optional for the other three.

```typescript
function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}
// + afterEach(() => { delete (globalThis as Record<string, unknown>).Session; });
```

### Pattern C — Apps Script global stub installation via `installAppsScriptMocks`

**Source:** `apps-script/src/__tests__/test-helpers.ts:61-442` (full mock installer; called by resetMocks).

**Apply to:** Nothing new — Plan 08-01's `mountSidebar` rides on top of this; the `getUserProperties()` extension adds to the existing `PropertiesService` block at lines 292-298.

### Pattern D — Logging contract `log(level, op, fields)`

**Source:** `apps-script/src/lib/searchIndex.ts:191` and `:345-349` (call sites).

**Apply to:** searchIndex.ts MRU change is in-scope; the swapped functions do NOT currently log (lines 355-371 have no `log` call). Planner may add a `log('info', 'pushRecentSearch', { q })` for observability symmetry; CONTEXT defers this to Claude's discretion.

### Pattern E — Verbatim string assertions for sidebar copy

**Source:** `apps-script/src/__tests__/showSearchSidebar.test.ts:60-67` (trigger-level analog) + `apps-script/src/__tests__/adminMgmtSidebar.test.ts:65-72`.

```typescript
expect(captured!._html).toContain('Search');           // h3 title
expect(captured!._html).toContain('Item name…');       // placeholder
expect(captured!._html).toContain('Results may be up to 60 seconds stale.');  // SEARCH-03 affordance
```

**Apply to:** Inline-JS tests CAN reuse these literals as smoke (already covered by trigger-level test for happy-path; inline test focuses on DOM mutation rather than initial HTML content).

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `apps-script/vitest.config.ts` | config | build-tool | No existing vitest config; planner authors from scratch using D-01 + the documented shape above |
| `mountSidebar` helper in `test-helpers.ts` | utility | request-response (DOM + async callbacks) | No JSDOM usage anywhere in the repo currently; planner authors using the SIDEBAR_BODY shape (e.g., `showSearchSidebar.ts:133-282`) as the input contract and the documented helper signature above as the output contract |

---

## Metadata

**Analog search scope:**
- `apps-script/src/__tests__/` (31 test files)
- `apps-script/src/triggers/` (16 files)
- `apps-script/src/lib/` (searchIndex.ts; test-helpers.ts)
- `apps-script/package.json`, `apps-script/tsconfig.json`, `apps-script/build.mjs`
- `.planning/phases/05-search-onboarding-privacy-polish/05-0{1..5}-SUMMARY.md` (5 files; verbatim template for DOC-04)
- `.planning/phases/03-apps-script-enrichment-foundation/03-0{1..4}-PLAN.md` headers (frontmatter `files_created` arrays)
- `.planning/phases/04-differentiator-features/04-0{1..4}-PLAN.md` headers

**Files scanned:** ~50 (most weighed by Grep; full reads on test-helpers.ts, adminMgmtSidebar.test.ts, searchIndex.ts/.test.ts, 05-01-SUMMARY.md, showSearchSidebar.ts trigger).

**Pattern extraction date:** 2026-05-12

## PATTERN MAPPING COMPLETE

**Phase:** 08 - test-infra-persistence-docs
**Files classified:** 17
**Analogs found:** 15 / 17 (2 NEW patterns: vitest.config.ts, mountSidebar)

### Coverage
- Files with exact analog: 11 (8 SUMMARY.md template clones + 2 in-file modify + 1 package.json self-extend)
- Files with role-match analog (NEW mechanic on top): 5 (sidebar inline-JS tests; existing trigger-level tests are the outer-shape analog, JSDOM mounting is the new inner mechanic) + 1 (test-helpers.ts extension)
- Files with no analog: 2 (vitest.config.ts authored from scratch per D-01; mountSidebar authored from scratch per D-04)

### Key Patterns Identified
- All apps-script tests follow `resetMocks() + seedMeta() + describe(...) + it(...)` shape (verified across `adminMgmtSidebar.test.ts`, `searchIndex.test.ts`, `showSearchSidebar.test.ts`)
- `Session.getEffectiveUser` mock is per-test-file boilerplate (NOT exported from test-helpers); admin-gated sidebars clone the helper verbatim from `adminMgmtSidebar.test.ts:29-34`
- SIDEBAR_BODY template literals are `String.raw` constants inside each `show*Sidebar.ts` trigger file (e.g., `showSearchSidebar.ts:133-282`); the build-fn shape (`buildSidebarHtml(theme): string`) is the public surface tests should call
- `PropertiesService` mock at `test-helpers.ts:292-298` currently exposes ONLY `getDocumentProperties()` — Phase 8 plan 08-01 must extend with `getUserProperties()` returning the same shape over a separate `state.userProperties` Map
- `CacheService.getDocumentCache()` is called from 3 sites in `searchIndex.ts` (lines 176, 310, 311, 355-371); only the 355-371 pair is the MRU swap target — the other two are per-char inv cache and must NOT change
- Phase 5 SUMMARY.md frontmatter shape is byte-stable across all 5 plans (`05-01..05-05`); DOC-04 backfills clone the field NAMES + ORDER verbatim and vary only content
