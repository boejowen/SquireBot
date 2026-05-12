---
phase: 08-test-infra-persistence-docs
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - apps-script/vitest.config.ts
  - apps-script/package.json
  - apps-script/package-lock.json
  - apps-script/src/__tests__/test-helpers.ts
autonomous: true
requirements: [TEST-01]
tags: [apps-script, vitest, jsdom, test-infra, mount-sidebar, properties-service-mock]
must_haves:
  truths:
    - "`cd apps-script && npm test` runs every existing test file under a JSDOM environment with zero per-test environment annotations and zero existing tests broken."
    - "`apps-script/vitest.config.ts` exists, declares `environment: 'jsdom'` at the top level of `defineConfig({ test: { ... } })`, and lists `src/__tests__/**/*.test.ts` as the explicit include glob."
    - "`jsdom` appears as a `devDependencies` entry in `apps-script/package.json` and is installed under `apps-script/node_modules/jsdom/`."
    - "`test-helpers.ts` exports a `mountSidebar(html: string): MountedSidebar` helper that parses the HTML, installs a fluent `window.google.script.run` Proxy mock BEFORE executing inline `<script>` content, and returns `{ document, window, dispatchRunCall, failRunCall, runCalls }`."
    - "`test-helpers.ts` PropertiesService global mock exposes `getDocumentProperties()` AND `getUserProperties()` (and `getScriptProperties()` for completeness), with `getUserProperties()` backed by a separate `state.userProperties: Map<string, string>` so user-scope and document-scope writes do not bleed (D-04)."
    - "`MockState` interface has a new `userProperties: Map<string, string>` field and `newMockState()` initializes it as `new Map()`."
  artifacts:
    - path: "apps-script/vitest.config.ts"
      provides: "vitest 1.6 config with JSDOM environment default per D-01"
      min_lines: 8
      contains: "environment: 'jsdom'"
    - path: "apps-script/package.json"
      provides: "jsdom 24.x devDependency"
      contains: "\"jsdom\""
    - path: "apps-script/src/__tests__/test-helpers.ts"
      provides: "mountSidebar JSDOM helper + getUserProperties scope alias"
      contains: "export function mountSidebar"
  key_links:
    - from: "apps-script/vitest.config.ts"
      to: "apps-script/node_modules/jsdom"
      via: "vitest test.environment = 'jsdom' resolves jsdom via vitest optionalDependencies"
      pattern: "environment:\\s*'jsdom'"
    - from: "apps-script/src/__tests__/test-helpers.ts"
      to: "globalThis.PropertiesService.getUserProperties()"
      via: "Map-backed mock returning getProperty/setProperty/deleteProperty"
      pattern: "getUserProperties:\\s*\\(\\)\\s*=>"
    - from: "apps-script/src/__tests__/test-helpers.ts"
      to: "mountSidebar() consumers in wave 2 (Plan 08-02)"
      via: "named export consumed by 4 new sidebar inline-JS test files"
      pattern: "^export function mountSidebar"
---

<objective>
Wire JSDOM into vitest for the `apps-script/` package so every test file runs against a DOM by default (TEST-01), and extend `test-helpers.ts` with two test-infrastructure surfaces that Plan 08-02 (sidebar inline-JS tests) and Plan 08-03 (SEARCH-05 PropertiesService migration) consume in the same wave: a `mountSidebar(html)` JSDOM helper and a `getUserProperties()` scope alias on the existing PropertiesService mock.

Purpose: TEST-02 sidebar inline-JS tests need a DOM; SEARCH-05 needs a per-user PropertiesService mock. Both surfaces are tested infrastructure, not feature code — landing them in Wave 1 (parallel with 08-03 and 08-04) lets Wave 2 (08-02) consume them without further setup. Per D-04, both helpers live in `test-helpers.ts` alongside the existing `resetMocks` / `makeSheet` / `seedMeta` / `installSessionMock` exports.

Output: a new `vitest.config.ts`, a `jsdom` devDependency entry + install, and ~50 additive lines in `test-helpers.ts` (mountSidebar + PropertiesService scope aliases + MockState extension).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md
@.planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md
@.planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md
@apps-script/package.json
@apps-script/tsconfig.json
@apps-script/src/__tests__/test-helpers.ts

<interfaces>
<!-- Existing test-helpers.ts surfaces this plan extends. Lift verbatim -- do NOT re-derive. -->

PropertiesService global mock (existing, test-helpers.ts:292-298):
```typescript
(globalThis as Record<string, unknown>).PropertiesService = {
  getDocumentProperties: () => ({
    getProperty: (k: string) => state.properties.get(k) ?? null,
    setProperty: (k: string, v: string) => { state.properties.set(k, v); },
    deleteProperty: (k: string) => { state.properties.delete(k); },
  }),
};
```

MockState fields used by mock helpers (existing, test-helpers.ts:23-53):
- `properties: Map<string, string>` -- backs getDocumentProperties()
- `lockTryLockReturn: boolean` -- controllable mock for LockService.tryLock
- `alertCalls: Array<...>` -- captures getUi().alert(...) calls (Phase 7)
- `triggers: Array<...>` -- captures ScriptApp.newTrigger().create() calls

newMockState() initializer (existing, test-helpers.ts:444-461):
```typescript
function newMockState(): MockState {
  return {
    sheets: new Map(),
    triggers: [],
    properties: new Map(),
    cache: new Map(),
    // ...
  };
}
```

vitest 1.6 import path for defineConfig (per RESEARCH §vitest+JSDOM config):
```typescript
import { defineConfig } from 'vitest/config';
```

Sidebar SIDEBAR_BODY shape (from apps-script/src/triggers/showSearchSidebar.ts and 4 others):
- A `<style>` block with `:root` CSS custom properties (theme tokens)
- A `<div class="sidebar">…</div>` body with elements like `<input id="q">`, `<button id="searchBtn">`, `<div id="results">`
- An inline `<script>…</script>` block whose last statement is `init();` -- it calls `google.script.run.withSuccessHandler(...).withFailureHandler(...).METHOD_NAME(args)` at top-level / on click handlers
- Chain shape is uniform across all 5 sidebars: `google.script.run.withSuccessHandler(fn).withFailureHandler(fn).METHOD(args)` (verified by grep in RESEARCH §mountSidebar architecture)
- One fire-and-forget call exists (Search's `pushRecentSearchCall(q)` at showSearchSidebar.ts:206) -- no handler chain -- mock must tolerate this.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Install jsdom + create vitest.config.ts</name>
  <files>apps-script/package.json, apps-script/package-lock.json, apps-script/vitest.config.ts</files>
  <read_first>
    - apps-script/package.json (lines 1-30) -- current devDependencies block; verify vitest is at ^1.6.0 and no jsdom entry exists yet
    - apps-script/tsconfig.json -- include/exclude patterns to mirror in vitest config
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §vitest+JSDOM config -- locked install command and config shape
    - .planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md §apps-script/vitest.config.ts -- NEW PATTERN section with exact config body
  </read_first>
  <action>
1. From the `apps-script/` directory, install jsdom as a devDependency pinned to 24.x (D-01 quota note: ~50MB transitive; acceptable for devDep):

```bash
cd apps-script && npm install -D jsdom@^24.0.0
```

If `npm install` fails with a Windows-toolchain prompt (canvas, etc.), accept the prompt or rerun with `--ignore-scripts` -- jsdom's optional native deps are not required by vitest's environment.

2. Verify the install:

```bash
test -d apps-script/node_modules/jsdom
grep -q '"jsdom"' apps-script/package.json
```

3. Create `apps-script/vitest.config.ts` (NEW FILE) with this exact body:

```typescript
import { defineConfig } from 'vitest/config';

// Phase 8 plan 08-01 (TEST-01): JSDOM is the default test environment so
// sidebar inline-JS tests (Plan 08-02) and any future DOM-touching test get a
// DOM by default. Existing tests that mock Apps Script globals
// (SpreadsheetApp, CacheService, etc.) treat JSDOM as a no-op. Per CONTEXT
// D-01, per-test `// @vitest-environment node` overrides remain available but
// no current test needs one.
export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['src/__tests__/**/*.test.ts'],
    exclude: ['node_modules', 'dist', 'src/__fixtures__'],
    globals: false,
  },
});
```

4. Run the full existing apps-script suite to confirm no regression from the environment switch (jsdom should be a no-op for the existing 324+ tests that mock Apps Script globals -- none currently touch `document` or `window`):

```bash
cd apps-script && npm test 2>&1 | tail -20
```

Expected: same green count as the post-Phase-7 baseline (324+/324+).

5. Commit:
```bash
git add apps-script/package.json apps-script/package-lock.json apps-script/vitest.config.ts
git commit -m "test(08-01): add jsdom + vitest.config.ts for TEST-01"
```
  </action>
  <verify>
    <automated>
cd apps-script
test -f vitest.config.ts || exit 1
grep -q "environment: 'jsdom'" vitest.config.ts || exit 1
grep -q "include: \['src/__tests__" vitest.config.ts || exit 1
grep -q '"jsdom"' package.json || exit 1
test -d node_modules/jsdom || exit 1
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `test -f apps-script/vitest.config.ts` exits 0
    - `grep -c "environment: 'jsdom'" apps-script/vitest.config.ts` returns 1
    - `grep -c "include: \['src/__tests__" apps-script/vitest.config.ts` returns 1
    - `grep -c '"jsdom"' apps-script/package.json` returns at least 1 (the devDep entry)
    - `test -d apps-script/node_modules/jsdom` exits 0
    - `cd apps-script && npm test` exits 0 (no test regressions)
  </acceptance_criteria>
  <done>vitest.config.ts ships with JSDOM env, jsdom devDep is installed and recorded, full existing suite stays green.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Extend PropertiesService mock with getUserProperties scope alias + add userProperties to MockState</name>
  <files>apps-script/src/__tests__/test-helpers.ts</files>
  <read_first>
    - apps-script/src/__tests__/test-helpers.ts (lines 1-60 MockState interface, lines 280-310 PropertiesService block, lines 440-470 newMockState initializer) -- the existing mock is the verbatim analog
    - .planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md §test-helpers.ts -- exact extension shape (PropertiesService block + MockState + newMockState)
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §PropertiesService migration mechanics -- D-04/D-05 contract: separate state.userProperties Map so user vs document writes don't bleed
  </read_first>
  <behavior>
    - PropertiesService.getDocumentProperties() still reads/writes state.properties (unchanged)
    - PropertiesService.getUserProperties() reads/writes state.userProperties (new, isolated)
    - PropertiesService.getScriptProperties() returns the SAME shape over state.userProperties (acceptable per RESEARCH §Pitfall resolution -- no test currently exercises script scope; the alias guards against future "where did it go" reaches for the wrong scope)
    - state.userProperties.get('k') after a getUserProperties().setProperty('k', 'v') in the same test returns 'v'
    - state.properties.get('k') after a getUserProperties().setProperty('k', 'v') returns undefined (no cross-scope bleed)
    - resetMocks() clears state.userProperties (each test starts empty)
  </behavior>
  <action>
1. Open `apps-script/src/__tests__/test-helpers.ts`.

2. Add `userProperties: Map<string, string>` to the `MockState` interface (existing block, lines 23-53). Place it immediately after the existing `properties: Map<string, string>` field. Add a `// Phase 8 plan 08-01: SEARCH-05 per-user MRU scope` comment.

3. In `newMockState()` (existing block at lines 444-461), add `userProperties: new Map(),` immediately after the existing `properties: new Map(),` line.

4. Replace the existing PropertiesService block (lines 292-298, currently exposing only `getDocumentProperties`) with:

```typescript
  (globalThis as Record<string, unknown>).PropertiesService = {
    getDocumentProperties: () => ({
      getProperty: (k: string) => state.properties.get(k) ?? null,
      setProperty: (k: string, v: string) => { state.properties.set(k, v); },
      deleteProperty: (k: string) => { state.properties.delete(k); },
    }),
    // Phase 8 plan 08-01 (D-04 / D-05): per-user scope, backed by a SEPARATE
    // Map so SEARCH-05's getUserProperties() writes don't bleed into the
    // document-scope tests that already passed in Phases 1-7. getScriptProperties
    // aliases the same per-user Map for the rare current consumer; no test
    // currently distinguishes the two and the production code never reads
    // script-scope, so the alias is safe.
    getUserProperties: () => ({
      getProperty: (k: string) => state.userProperties.get(k) ?? null,
      setProperty: (k: string, v: string) => { state.userProperties.set(k, v); },
      deleteProperty: (k: string) => { state.userProperties.delete(k); },
    }),
    getScriptProperties: () => ({
      getProperty: (k: string) => state.userProperties.get(k) ?? null,
      setProperty: (k: string, v: string) => { state.userProperties.set(k, v); },
      deleteProperty: (k: string) => { state.userProperties.delete(k); },
    }),
  };
```

5. Add a test-helpers regression check (run the full suite -- nothing should break since getDocumentProperties() behavior is preserved byte-for-byte):

```bash
cd apps-script && npm test 2>&1 | tail -10
```

6. Commit:
```bash
git add apps-script/src/__tests__/test-helpers.ts
git commit -m "test(08-01): extend PropertiesService mock with getUserProperties scope alias"
```
  </action>
  <verify>
    <automated>
cd apps-script
grep -c "getUserProperties:" src/__tests__/test-helpers.ts | grep -E "^[1-9]" || exit 1
grep -q "userProperties: Map<string, string>" src/__tests__/test-helpers.ts || exit 1
grep -q "userProperties: new Map" src/__tests__/test-helpers.ts || exit 1
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `grep -c "getUserProperties:" apps-script/src/__tests__/test-helpers.ts` returns at least 1
    - `grep -c "getScriptProperties:" apps-script/src/__tests__/test-helpers.ts` returns at least 1
    - `grep -c "state.userProperties" apps-script/src/__tests__/test-helpers.ts` returns at least 3 (getProperty + setProperty + deleteProperty)
    - `grep -c "^\s*userProperties: Map<string, string>" apps-script/src/__tests__/test-helpers.ts` returns 1 (MockState field)
    - `grep -c "userProperties: new Map" apps-script/src/__tests__/test-helpers.ts` returns 1 (newMockState initializer)
    - `cd apps-script && npm test` exits 0 (no regressions on the existing suite -- getDocumentProperties behavior is byte-identical)
  </acceptance_criteria>
  <done>PropertiesService mock surfaces getUserProperties + getScriptProperties returning per-user-scope-isolated APIs; existing 324+ tests stay green.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Add mountSidebar(html) JSDOM helper to test-helpers.ts</name>
  <files>apps-script/src/__tests__/test-helpers.ts</files>
  <read_first>
    - apps-script/src/__tests__/test-helpers.ts (line 1 exports header, end-of-file for placement) -- new helper appends to the existing exports
    - apps-script/src/triggers/showSearchSidebar.ts (lines 133-282, SIDEBAR_BODY template + init() call site at the bottom of the inline script) -- canonical input shape
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §mountSidebar helper architecture -- locked Proxy-based fluent mock pattern + step-order (install google.script.run BEFORE re-executing scripts)
    - .planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md §mountSidebar -- contract type + JSDOM gotcha workaround
  </read_first>
  <behavior>
    - mountSidebar('<div id="x">hi</div><script>document.getElementById("x").textContent = "bye";</script>') executes the inline script (asserting on `.textContent === "bye"` confirms script execution worked)
    - mountSidebar of a sidebar HTML that calls `google.script.run.withSuccessHandler(s).withFailureHandler(f).METHOD(args)` at top of init() captures the call into runCalls and pendingByMethod (assert via getPendingCalls or runCalls array)
    - dispatchRunCall('METHOD', payload) invokes the success handler registered for the FIRST pending call to METHOD with payload as the argument (FIFO queue semantics)
    - failRunCall('METHOD', { message: 'boom' }) invokes the failure handler with the error object
    - dispatchRunCall on a method with no pending calls throws Error('No pending METHOD call')
    - Fire-and-forget calls (no withSuccessHandler / withFailureHandler chain, e.g., Search's pushRecentSearchCall(q)) succeed silently -- the runCalls array records them but no handlers are invoked
    - Re-calling mountSidebar in the same test resets document.body but the JSDOM realm persists across the test (resetMocks in beforeEach is sufficient cleanup)
  </behavior>
  <action>
1. Open `apps-script/src/__tests__/test-helpers.ts`. Append a NEW exported helper at the end of the file (after the existing exports — past `resetMocks`, `makeSheet`, `seedMeta`, `installSessionMock`, `installAppsScriptMocks`). Add this block verbatim:

```typescript
// ----------------------------------------------------------------------------
// Phase 8 plan 08-01 (D-04): mountSidebar(html) JSDOM helper for TEST-02
// sidebar inline-JS tests. Parses a SIDEBAR_BODY string, installs a
// controllable google.script.run Proxy mock BEFORE re-executing inline
// <script> blocks (init() runs immediately at the bottom of each sidebar's
// inline JS and reads window.google synchronously), then returns the realm
// plus dispatch helpers so tests can resolve enqueued call promises FIFO.
//
// JSDOM gotcha: per HTML5 spec, <script> tags inserted via innerHTML do NOT
// execute. We work around this by extracting <script> textContent, then
// recreating each as a fresh script element appended to document.head.
// See https://www.ghinda.net/article/script-tags/ and JSDOM issue #426.
// ----------------------------------------------------------------------------

export interface MountedSidebar {
  document: Document;
  window: Window & typeof globalThis;
  runCalls: Array<{ method: string; args: unknown[] }>;
  dispatchRunCall: (method: string, payload: unknown) => void;
  failRunCall: (method: string, error: { message: string }) => void;
  getPendingCalls: () => Array<{ method: string; args: unknown[] }>;
}

export function mountSidebar(html: string): MountedSidebar {
  // 1. Reset the body.
  document.body.innerHTML = '';

  // 2. Parse the HTML into a detached <template> so the browser parser
  //    splits <script> nodes from the rest without executing them.
  const tpl = document.createElement('template');
  tpl.innerHTML = html;

  // 3. Walk the parsed fragment, separating script nodes from the rest.
  const scripts: HTMLScriptElement[] = [];
  const frag = document.createDocumentFragment();
  Array.from(tpl.content.childNodes).forEach((node) => {
    if (node.nodeName === 'SCRIPT') {
      scripts.push(node as HTMLScriptElement);
    } else {
      frag.appendChild(node.cloneNode(true));
    }
  });
  document.body.appendChild(frag);

  // 4. Build the google.script.run fluent mock. The chain shape is
  //    `.withSuccessHandler(s).withFailureHandler(f).METHOD(args)` but any
  //    handler is optional (Search's pushRecentSearchCall is fire-and-forget).
  //    Each terminal METHOD invocation enqueues a record so dispatch can
  //    resolve handlers FIFO.
  const runCalls: Array<{ method: string; args: unknown[] }> = [];
  const pendingByMethod = new Map<string, Array<{ success?: Function; failure?: Function }>>();

  function makeChain(handlers: { success?: Function; failure?: Function }): unknown {
    return new Proxy({}, {
      get(_t, prop: string) {
        if (prop === 'withSuccessHandler') {
          return (fn: Function) => makeChain({ ...handlers, success: fn });
        }
        if (prop === 'withFailureHandler') {
          return (fn: Function) => makeChain({ ...handlers, failure: fn });
        }
        // Terminal method invocation.
        return (...args: unknown[]) => {
          runCalls.push({ method: prop, args });
          const queue = pendingByMethod.get(prop) ?? [];
          queue.push(handlers);
          pendingByMethod.set(prop, queue);
        };
      },
    });
  }

  (window as unknown as Record<string, unknown>).google = {
    script: { run: makeChain({}), host: { close: () => { /* no-op for sidebars */ } } },
  };

  // 5. Re-create script elements with .textContent set then append to head --
  //    this is the canonical workaround for HTML5's no-exec-via-innerHTML rule.
  scripts.forEach((orig) => {
    const s = document.createElement('script');
    if (orig.textContent) s.textContent = orig.textContent;
    document.head.appendChild(s);
  });

  function dispatchRunCall(method: string, payload: unknown): void {
    const queue = pendingByMethod.get(method);
    if (!queue || queue.length === 0) {
      throw new Error(`mountSidebar.dispatchRunCall: no pending ${method} call`);
    }
    const next = queue.shift()!;
    if (next.success) next.success(payload);
  }

  function failRunCall(method: string, error: { message: string }): void {
    const queue = pendingByMethod.get(method);
    if (!queue || queue.length === 0) {
      throw new Error(`mountSidebar.failRunCall: no pending ${method} call`);
    }
    const next = queue.shift()!;
    if (next.failure) next.failure(error);
  }

  function getPendingCalls(): Array<{ method: string; args: unknown[] }> {
    const out: Array<{ method: string; args: unknown[] }> = [];
    pendingByMethod.forEach((queue, method) => {
      queue.forEach(() => out.push({ method, args: [] }));
    });
    return out;
  }

  return {
    document,
    window: window as Window & typeof globalThis,
    runCalls,
    dispatchRunCall,
    failRunCall,
    getPendingCalls,
  };
}
```

2. Verify exports and typecheck:

```bash
cd apps-script && npx tsc --noEmit 2>&1 | tail -10
```

3. Verify the existing suite still passes (mountSidebar is opt-in -- no existing test imports it, so this should be a no-op):

```bash
cd apps-script && npm test 2>&1 | tail -10
```

4. Commit:
```bash
git add apps-script/src/__tests__/test-helpers.ts
git commit -m "test(08-01): add mountSidebar JSDOM helper for TEST-02"
```
  </action>
  <verify>
    <automated>
cd apps-script
grep -c "^export function mountSidebar" src/__tests__/test-helpers.ts | grep -E "^1$" || exit 1
grep -q "^export interface MountedSidebar" src/__tests__/test-helpers.ts || exit 1
grep -q "pendingByMethod" src/__tests__/test-helpers.ts || exit 1
grep -q "withSuccessHandler" src/__tests__/test-helpers.ts || exit 1
grep -q "withFailureHandler" src/__tests__/test-helpers.ts || exit 1
npx tsc --noEmit 2>&1 | tail -5
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `grep -c "^export function mountSidebar" apps-script/src/__tests__/test-helpers.ts` returns exactly 1
    - `grep -c "^export interface MountedSidebar" apps-script/src/__tests__/test-helpers.ts` returns exactly 1
    - `grep -c "withSuccessHandler" apps-script/src/__tests__/test-helpers.ts` returns at least 1
    - `grep -c "withFailureHandler" apps-script/src/__tests__/test-helpers.ts` returns at least 1
    - `grep -c "dispatchRunCall" apps-script/src/__tests__/test-helpers.ts` returns at least 2 (declaration + return-object entry)
    - `grep -c "document.createElement('script')" apps-script/src/__tests__/test-helpers.ts` returns at least 1 (the JSDOM-exec workaround)
    - `cd apps-script && npx tsc --noEmit` exits 0
    - `cd apps-script && npm test` exits 0 (full suite stays green; mountSidebar is unreferenced by existing tests)
  </acceptance_criteria>
  <done>mountSidebar is exported, typechecks clean, executes inline `<script>` blocks correctly under JSDOM, and the fluent google.script.run Proxy supports any-order chaining + fire-and-forget terminal invocations.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| dev-machine ↔ npm registry | jsdom 24.x install -- transitively pulls cssom, tough-cookie, webidl-conversions, whatwg-url. Standard supply-chain surface. |
| test code ↔ production bundle | test-helpers.ts MUST NOT be referenced by `dist/Code.js` -- esbuild entry is `src/Code.ts`, not `src/__tests__/`, so this is structurally enforced; grep gate verifies. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-08-01-01 | Tampering | jsdom transitive deps | accept | Standard devDep surface; package-lock.json captures resolved hashes. Same risk profile as existing vitest/esbuild/typescript deps already accepted. |
| T-08-01-02 | Information disclosure | test-helpers.ts leaks into production bundle | mitigate | Grep gate at task acceptance: `grep -c "mountSidebar\|userProperties" apps-script/dist/Code.js` returns 0 after `npm run build`. esbuild entry is `src/Code.ts` which never imports `__tests__/`. |
| T-08-01-03 | Repudiation | mountSidebar mock allows tests to claim behavior the live HtmlService iframe doesn't expose | accept | The Proxy mock supports a SUPERSET of the live `google.script.run` surface; any failure mode that passes under the mock would also pass live. Inverse is not guaranteed but is the existing tradeoff for all Apps Script tests. |
| T-08-01-04 | Denial of service | jsdom install bloats CI install time | accept | ~50MB transitive; CI install adds ~5-10s; well within budget. Documented in SUMMARY metrics. |
</threat_model>

<verification>
After all 3 tasks complete, run:

```bash
cd apps-script
test -f vitest.config.ts
grep -q "environment: 'jsdom'" vitest.config.ts
grep -q '"jsdom"' package.json
test -d node_modules/jsdom
grep -q "^export function mountSidebar" src/__tests__/test-helpers.ts
grep -q "getUserProperties:" src/__tests__/test-helpers.ts
grep -q "userProperties: Map<string, string>" src/__tests__/test-helpers.ts
npx tsc --noEmit
npm test
```

Expected: every assertion exits 0; `npm test` shows full suite (324+/324+) green.

Verification-hook 5 (schema-gate, untouched across Phase 8):
```bash
grep -c "writeMetaRow.*schema_version.*'3'" apps-script/src/lib/migrations.ts  # baseline
grep "WatcherMaxSchemaVersion" internal/sheet/client.go  # = 3, unchanged
```
</verification>

<success_criteria>
- `apps-script/vitest.config.ts` exists with `environment: 'jsdom'` declared.
- `jsdom` is a devDependency, installed under `apps-script/node_modules/jsdom/`.
- `test-helpers.ts` exports `mountSidebar` and `MountedSidebar` interface.
- `test-helpers.ts` PropertiesService mock supports `getUserProperties()` over an isolated `state.userProperties` Map.
- `MockState` interface has `userProperties: Map<string, string>` and `newMockState()` initializes it.
- Full apps-script test suite stays green (324+ baseline from Phase 7) with zero new tests.
- TypeScript compile (`npx tsc --noEmit`) clean.
- Schema gates unchanged: `migrations.ts` schema_version='3' write count and `client.go` `WatcherMaxSchemaVersion=3` both untouched.
</success_criteria>

<output>
After completion, create `.planning/phases/08-test-infra-persistence-docs/08-01-SUMMARY.md` per the Phase 5 template (`.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md`).
</output>
