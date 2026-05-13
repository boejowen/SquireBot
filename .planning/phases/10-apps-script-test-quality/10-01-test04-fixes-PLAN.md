---
phase: 10-apps-script-test-quality
plan: 01
plan_id: 10-01-test04-fixes
type: execute
wave: 1
depends_on: []
files_modified:
  - apps-script/src/__tests__/test-helpers.ts
  - apps-script/src/__tests__/evictionSidebar.inline.test.ts
  - apps-script/src/__tests__/searchIndex.test.ts
  - apps-script/src/__tests__/searchSidebar.inline.test.ts
autonomous: true
requirements: [TEST-04]
tags: [test-quality, mount-sidebar, iife-wrap, jsdom, wave1]

must_haves:
  truths:
    - "mountSidebar's eval site wraps the inline-script source in a function-expression IIFE so top-level `var` / `function` declarations no longer leak to the test realm's globalThis (WR-01 closed at source)."
    - "evictionSidebar.inline.test.ts TE1 asserts the exact production success copy ('Marked 2 character(s) as removed') via toContain, NOT a permissive `/Marked 2|removed/i` regex that could silently match stale copy (WR-02 closed)."
    - "searchIndex.test.ts Test 4 no longer wraps its assertion in try/catch — the plan-locked toEqual is unguarded; a future regression fails loudly instead of being swallowed (WR-03 closed)."
    - "searchSidebar.inline.test.ts TS1 explicitly resolves the inline success-handler's enqueued `pushRecentSearch` call via `m.dispatchRunCall('pushRecentSearch', null);` immediately after the existing `runSearch` dispatch — pending mock queue exits empty at end of test (WR-04 closed)."
    - "All 5 existing inline-JS tests (8 happy/error cases across bankCoin/charInfo/eviction/searchSidebar) remain green after the mountSidebar IIFE wrap — the IIFE form is the function-expression `(function(){ ... })();` (NOT arrow) so `this === window` semantics survive."
    - "No production code is touched: `apps-script/src/triggers/` and `apps-script/src/lib/` directories have zero edits (per CONTEXT.md D-01)."
    - "Schema lock unchanged: `_meta.schema_version` = 3 in `apps-script/src/lib/migrations.ts`; `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go`."
  artifacts:
    - path: apps-script/src/__tests__/test-helpers.ts
      provides: "mountSidebar eval site (lines ~620-625) wrapped in function-expression IIFE — `(0, eval)(\\`(function(){\\n${src}\\n})();\\`)`"
      contains: "(function(){"
    - path: apps-script/src/__tests__/evictionSidebar.inline.test.ts
      provides: "TE1 success-copy assertion uses toContain('Marked 2 character(s) as removed')"
      contains: "Marked 2 character(s) as removed"
    - path: apps-script/src/__tests__/searchIndex.test.ts
      provides: "Test 4 body has no try/catch around the toEqual assertion; only a single unguarded `expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);`"
      contains: "didYouMean('clok', seed4)"
    - path: apps-script/src/__tests__/searchSidebar.inline.test.ts
      provides: "TS1 has explicit `m.dispatchRunCall('pushRecentSearch', null);` immediately after the runSearch dispatch"
      contains: "pushRecentSearch"
  key_links:
    - from: "apps-script/src/__tests__/test-helpers.ts:mountSidebar eval"
      to: "all 5 inline-JS test files (bankCoin, charInfo, eviction, search, future Admin-Mgmt)"
      via: "function-expression IIFE wrap — `(0, eval)(\\`(function(){\\\\n${src}\\\\n})();\\`)`"
      pattern: "\\(function\\(\\)\\{"
    - from: "WR-01..WR-04 advisory findings in 08-REVIEW.md"
      to: "this plan's 4 task closures"
      via: "1:1 fix per warning; no info-level items (per CONTEXT.md D-01 — IN-01..IN-06 deferred to v1.1 as 999.24..999.29)"
      pattern: "WR-0[1-4]"
---

<objective>
Close the 4 warning-level findings in `.planning/phases/08-test-infra-persistence-docs/08-REVIEW.md` (TEST-04) by making the minimum surgical edits across 4 existing test files. No production code is touched. The schema lock is preserved verbatim from Phases 7/8/9.

Per CONTEXT.md D-02, the mountSidebar realm-leak fix is the **function-expression IIFE wrap** (Option a) — 3 LOC in `test-helpers.ts:620-625`. This was locked under the "simple, seamless" criterion (D-06): per-test JSDOM realm (Option b) adds ~500ms/run with no benefit; globalThis-tracking cleanup (Option c) is fragile and bug-prone to maintain.

Per CONTEXT.md D-01, the 6 info-level findings (IN-01..IN-06) are explicitly **OUT OF SCOPE** for this plan — they are deferred to v1.1 backlog as 999.24..999.29. Two of them touch production code (IN-01 `COL_RACE`/`COL_COUNT` collision, IN-02 orphaned CacheService key); the other four are nits with no current-state user impact.

**Scope discipline — non-negotiable (per CONTEXT.md D-01 + D-05):**
- If during execution you find yourself opening any file under `apps-script/src/triggers/` or `apps-script/src/lib/`, STOP — that is a signal scope has crept. Plans in this phase touch `apps-script/src/__tests__/` only.
- `_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. No migrations.ts edits. No `internal/sheet/client.go` edits.
- No new dependencies; no `package.json` edits; no `vitest.config.ts` edits.
- Do NOT introduce a per-test JSDOM realm (Option b for WR-01 — rejected by D-02).
- Do NOT introduce `afterEach` global-state cleanup hooks (Option c for WR-01 — rejected by D-02).
- Do NOT rewrite any of the 4 affected tests beyond the surgical fix described — keep their existing setup/structure intact.

This plan is **load-bearing for Plan 10-02**: Plan 10-02's new Admin-Mgmt inline test depends on the IIFE-fixed mountSidebar so its tests cannot leak state into the rest of the suite. CONTEXT.md D-04 explicitly orders 10-01 in Wave 1 and 10-02 in Wave 2 for this reason.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/10-apps-script-test-quality/10-CONTEXT.md
@.planning/phases/08-test-infra-persistence-docs/08-REVIEW.md
@.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md
@CLAUDE.md
@apps-script/src/__tests__/test-helpers.ts
@apps-script/src/__tests__/evictionSidebar.inline.test.ts
@apps-script/src/__tests__/searchIndex.test.ts
@apps-script/src/__tests__/searchSidebar.inline.test.ts

<interfaces>
<!-- Key contracts the executor will edit. Extracted from the test files. -->
<!-- Do NOT re-read these files to discover these; they are the contract. -->

From apps-script/src/__tests__/test-helpers.ts (lines 620-625, the mountSidebar eval site):

```typescript
scripts.forEach((orig) => {
  const src = orig.textContent || '';
  if (!src.trim()) return;
  // eslint-disable-next-line no-eval
  (0, eval)(src);                       // ← WR-01 leak site: replace with IIFE wrap
});
```

Required replacement (CONTEXT.md §1 verbatim):

```typescript
scripts.forEach((orig) => {
  const src = orig.textContent || '';
  if (!src.trim()) return;
  // eslint-disable-next-line no-eval
  (0, eval)(`(function(){\n${src}\n})();`);
});
```

The IIFE MUST be a function-expression `(function(){ ... })()` — NOT an arrow `(() => { ... })()`. Arrow IIFEs would break `this === window` semantics for any inline script that references `this` at top level (sidebar scripts use `var state = {...}` and `this` patterns).

From apps-script/src/__tests__/evictionSidebar.inline.test.ts (line 87, WR-02 fix site):

```typescript
const msg = m.document.getElementById('msg')!;
expect(msg.textContent || msg.innerHTML).toMatch(/Marked 2|removed/i);  // ← WR-02
```

Required replacement (CONTEXT.md §2 verbatim):

```typescript
const msg = m.document.getElementById('msg')!;
expect(msg.textContent || msg.innerHTML).toContain('Marked 2 character(s) as removed');
```

From apps-script/src/__tests__/searchIndex.test.ts (lines 94-114, WR-03 fix site — Test 4 body):

```typescript
it('returns the items within edit-distance ≤2 of the query (exact pair)', () => {
  const seed4 = ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin'];
  let planLockedPasses = false;
  try {
    expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
    planLockedPasses = true;
  } catch {
    expect(didYouMean('clok', seed4)).toEqual([]);
  }
  void planLockedPasses;
});
```

Required replacement: remove the try/catch entirely, drop the `planLockedPasses` flag, leave only the plan-locked toEqual. Per CONTEXT.md §3: "if the plan-locked behavior is actually correct, the assertion passes and the test is clean; if it's wrong, the test fails — which is the entire point of a test."

```typescript
it('returns the items within edit-distance ≤2 of the query (exact pair)', () => {
  const seed4 = ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin'];
  expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
});
```

Note: keep the leading comment block (lines 84-93) intact — it documents the plan-locked-vs-semantic history; only the test body's `try { ... } catch { ... }` scaffolding is removed.

From apps-script/src/__tests__/searchSidebar.inline.test.ts (lines 52-58, WR-04 fix site — inside TS1):

```typescript
// Resolve runSearch — one matched group with the item name.
m.dispatchRunCall('runSearch', {
  groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }],
  suggestions: [],
  coldFill: false,
  durationMs: 12,
});

const results = m.document.getElementById('results')!;
expect(results.innerHTML).toContain('Bone Helm');
```

Required addition (CONTEXT.md §4 — locked choice is "explicit resolve in the test, NOT an `afterEach` mock-contract change"): add one line immediately after the existing `runSearch` dispatch and before the assertion:

```typescript
// Resolve runSearch — one matched group with the item name.
m.dispatchRunCall('runSearch', {
  groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }],
  suggestions: [],
  coldFill: false,
  durationMs: 12,
});
// WR-04: drain the pushRecentSearch call that runSearch's success handler enqueued.
m.dispatchRunCall('pushRecentSearch', null);

const results = m.document.getElementById('results')!;
expect(results.innerHTML).toContain('Bone Helm');
```

Existing mountSidebar API (read-only — DO NOT modify the helper's contract beyond the IIFE wrap):

- `mountSidebar(html: string): MockState` — installs the SIDEBAR_BODY into a fresh JSDOM-scoped iframe and indirect-evals every inline `<script>` block.
- `m.dispatchRunCall(method: string, payload: unknown): void` — resolves the next pending success handler for `method` FIFO.
- `m.failRunCall(method: string, error: { message: string }): void` — resolves the next pending failure handler for `method` FIFO.
- `m.document` — JSDOM Document inside the mount realm; sidebar inline-JS attaches handlers + state here.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: WR-01 — wrap mountSidebar eval source in function-expression IIFE</name>
  <files>apps-script/src/__tests__/test-helpers.ts</files>
  <read_first>
    - apps-script/src/__tests__/test-helpers.ts (lines 580-660 — the mountSidebar helper body; you only need this slice, not the full 700+ LOC file)
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §1 (exact IIFE form + arrow-vs-function-expression constraint)
    - .planning/phases/08-test-infra-persistence-docs/08-REVIEW.md §WR-01 (the leak symptom + repro)
  </read_first>
  <behavior>
    - After the change, the `(0, eval)(src)` call inside `scripts.forEach((orig) => { ... })` at `test-helpers.ts:620-625` is replaced with `(0, eval)(\`(function(){\n${src}\n})();\`)`.
    - The IIFE is a function-expression `(function(){ ... })()` — NOT an arrow `(() => { ... })()`. Arrow-form would break inline scripts that reference `this` at top level (sidebar code does).
    - The comment block at lines 605-619 (JSDOM gotcha #2 + security note) stays intact — it remains accurate after the IIFE wrap (indirect eval still evaluates in the test realm; the IIFE only changes what hoists to globalThis).
    - All 5 existing inline-JS tests (bankCoin TB1+TB2, charInfo TC1+TC2, eviction TE1+TE2, search TS1+TS2) MUST remain green after the change. If any fails, the IIFE wrap is breaking a script that relies on global var/function bindings — surface as a deviation note rather than silently rewriting any test.
  </behavior>
  <action>
    Open `apps-script/src/__tests__/test-helpers.ts` and locate the `scripts.forEach((orig) => { ... })` block around lines 620-625. Replace ONLY the `(0, eval)(src);` line with the IIFE-wrapped form. The exact diff is:

    ```diff
       scripts.forEach((orig) => {
         const src = orig.textContent || '';
         if (!src.trim()) return;
         // eslint-disable-next-line no-eval
    -    (0, eval)(src);
    +    (0, eval)(`(function(){\n${src}\n})();`);
       });
    ```

    Do NOT change anything else in this file. Do NOT change the comment block above the forEach (lines 605-619). Do NOT change the function signature, the dispatchRunCall/failRunCall closures, or the MockState export. Single-line surgical edit.

    Schema-impact assertion (per CONTEXT.md D-05): this task touches `apps-script/src/__tests__/test-helpers.ts` only. `WatcherMaxSchemaVersion = 3` lives in `internal/sheet/client.go` (NOT in this task's scope). `_meta.schema_version = 3` lives in `apps-script/src/lib/migrations.ts` (NOT in this task's scope). No schema change.
  </action>
  <verify>
    <automated>cd apps-script && npm test -- --run __tests__/bankCoinSidebar.inline.test.ts __tests__/charInfoSidebar.inline.test.ts __tests__/evictionSidebar.inline.test.ts __tests__/searchSidebar.inline.test.ts</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE '\(0, eval\)\(.*\(function\(\)\{' apps-script/src/__tests__/test-helpers.ts` matches exactly 1 line (the new IIFE wrap).
    - `grep -nE '\(0, eval\)\(src\);' apps-script/src/__tests__/test-helpers.ts` returns 0 (old form removed).
    - `grep -nE '\(\(\) =>' apps-script/src/__tests__/test-helpers.ts | grep -i eval` returns 0 (NO arrow IIFE — per D-02 constraint).
    - All 8 existing inline-JS test cases pass (bankCoin TB1+TB2, charInfo TC1+TC2, eviction TE1+TE2, search TS1+TS2). The vitest command above exits 0.
    - `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1 (schema unchanged).
    - `grep -cE "writeMetaRow.*'_meta', 'schema_version', '3'|schema_version.*=\s*3" apps-script/src/lib/migrations.ts` returns ≥1 (schema unchanged).
  </acceptance_criteria>
  <done>The mountSidebar eval site wraps inline-script source in a function-expression IIFE; all 8 existing inline-JS test cases remain green; schema constants unchanged; no other file modified.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: WR-02 — tighten evictionSidebar.inline.test.ts TE1 success-copy assertion</name>
  <files>apps-script/src/__tests__/evictionSidebar.inline.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/evictionSidebar.inline.test.ts (full file — 105 LOC; read entirely for context)
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §2 (exact replacement form)
    - .planning/phases/08-test-infra-persistence-docs/08-REVIEW.md §WR-02 (overly permissive regex reasoning)
  </read_first>
  <behavior>
    - Line 87 (inside TE1, the happy-path test) replaces the permissive `toMatch(/Marked 2|removed/i)` with the exact substring `toContain('Marked 2 character(s) as removed')`.
    - The TE2 error-path assertion at line 101-102 (`expect(text).toContain('Eviction failed')` + `expect(text).toContain('unauth')`) is already exact-substring style — DO NOT touch it.
    - The IN-03 finding (this test's admin-gate bypass) is **OUT OF SCOPE per CONTEXT.md D-01**; do NOT add admin-gate setup to this test.
  </behavior>
  <action>
    Open `apps-script/src/__tests__/evictionSidebar.inline.test.ts` and locate line 87 inside TE1. Replace ONLY that single line:

    ```diff
       const msg = m.document.getElementById('msg')!;
    -  expect(msg.textContent || msg.innerHTML).toMatch(/Marked 2|removed/i);
    +  expect(msg.textContent || msg.innerHTML).toContain('Marked 2 character(s) as removed');
     });
    ```

    The production success copy in `showEvictionSidebar.ts` writes exactly `'Marked 2 character(s) as removed'` (verify by reading the trigger's `commitEviction` success handler — but DO NOT EDIT the trigger). If the actual production copy differs from this exact string (e.g., uses Unicode em-dash vs hyphen, different pluralization), use the actual production copy verbatim — surface the mismatch as a deviation note in the SUMMARY.

    Do NOT modify TE2. Do NOT modify the describe block, beforeEach, or imports. Single-line surgical edit.
  </action>
  <verify>
    <automated>cd apps-script && npm test -- --run __tests__/evictionSidebar.inline.test.ts</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE "toContain\('Marked 2 character\(s\) as removed'\)" apps-script/src/__tests__/evictionSidebar.inline.test.ts` matches exactly 1 line.
    - `grep -nE "toMatch\(\/Marked 2\|removed\/i\)" apps-script/src/__tests__/evictionSidebar.inline.test.ts` returns 0 (old permissive form removed).
    - `cd apps-script && npm test -- --run __tests__/evictionSidebar.inline.test.ts` reports both TE1 and TE2 PASS.
    - Schema gates from Task 1 still hold (no schema files touched).
  </acceptance_criteria>
  <done>TE1 asserts the exact production success copy via toContain; TE2 unchanged; both tests green; no other file modified.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: WR-03 — remove try/catch swallow in searchIndex.test.ts Test 4</name>
  <files>apps-script/src/__tests__/searchIndex.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/searchIndex.test.ts (lines 80-130 — the Test 4 block + comment header + Test 4b)
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §3 (try/catch removal reasoning)
    - .planning/phases/08-test-infra-persistence-docs/08-REVIEW.md §WR-03 (assertion-swallow risk)
  </read_first>
  <behavior>
    - The `it('returns the items within edit-distance ≤2 of the query (exact pair)', ...)` block at lines 94-114 is reduced to a single unguarded `expect(...).toEqual(...)` body.
    - The leading comment block (lines 84-93) describing the plan-locked vs whole-string-Levenshtein history STAYS INTACT — it documents legitimate context for future maintainers.
    - The `let planLockedPasses = false; ... void planLockedPasses;` scaffolding (lines 101, 113) is deleted along with the try/catch.
    - Test 4b (lines 116+, "returns close matches when whole-string distance permits") is UNCHANGED — it remains the semantic verification of fuzzy match.
  </behavior>
  <action>
    Open `apps-script/src/__tests__/searchIndex.test.ts` and replace lines 94-114 (the Test 4 `it(...)` block body) wholesale. The exact result must read:

    ```typescript
    it('returns the items within edit-distance ≤2 of the query (exact pair)', () => {
      const seed4 = ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin'];
      expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
    });
    ```

    Specifically:
    - DELETE the `let planLockedPasses = false;` line.
    - DELETE the `try {` line.
    - KEEP the `expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);` line UNCHANGED.
    - DELETE the `planLockedPasses = true;` line.
    - DELETE the `} catch {` line + the entire catch body (`// Fall through ...` comment + `expect(didYouMean('clok', seed4)).toEqual([]);` line + `}` close brace).
    - DELETE the `void planLockedPasses;` line.

    KEEP the comment block at lines 84-93 (the "Rule 1" plan-locked-vs-whole-string-Levenshtein explanation) UNCHANGED. It remains useful historical context.

    KEEP Test 4b (the next `it(...)` block at line 118+) UNCHANGED.

    If after the edit Test 4 fails (i.e., the plan-locked toEqual is actually wrong under whole-string Levenshtein), that is the **correct, intended outcome of WR-03** — surface as a Phase 10 deviation note in the SUMMARY rather than re-adding the try/catch. The decision tree per CONTEXT.md §3:
    - Test 4 passes → ship as-is; WR-03 closed.
    - Test 4 fails → the assertion is the bug, not the regression; document as a finding in the SUMMARY + open a v1.1 backlog item (999.30 candidate) for an explicit fix to `didYouMean` or the assertion. Do NOT re-introduce try/catch under any circumstance.
  </action>
  <verify>
    <automated>cd apps-script && npm test -- --run __tests__/searchIndex.test.ts</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE 'try \{' apps-script/src/__tests__/searchIndex.test.ts | wc -l` returns 0 (try/catch entirely removed from this file — note: also assert there were no other try/catch blocks in the file pre-edit; if there were unrelated ones, this grep needs to be scoped to the Test 4 line range. Verify with `grep -nE '} catch' apps-script/src/__tests__/searchIndex.test.ts` returning 0 as well).
    - `grep -nE 'planLockedPasses' apps-script/src/__tests__/searchIndex.test.ts` returns 0 (scaffolding flag deleted).
    - `grep -nE "didYouMean\('clok', seed4\)\)\.toEqual\(\['Cloak of Confusion', 'Cloak of Flames'\]\)" apps-script/src/__tests__/searchIndex.test.ts` matches exactly 1 line (the surviving plan-locked assertion).
    - `cd apps-script && npm test -- --run __tests__/searchIndex.test.ts` either passes (preferred) OR fails the Test 4 case with a clear "expected [..., ...] received [...]" diff (acceptable per CONTEXT.md §3 — surface as deviation note + ship-gate-blocking finding for Plan 10-03 to decide on).
    - Schema gates from Task 1 still hold.
  </acceptance_criteria>
  <done>Test 4 body is one unguarded toEqual assertion; planLockedPasses flag deleted; try/catch removed; Test 4b unchanged; comment header preserved.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 4: WR-04 — drain the leaked pushRecentSearch pending mock call in searchSidebar.inline.test.ts TS1</name>
  <files>apps-script/src/__tests__/searchSidebar.inline.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/searchSidebar.inline.test.ts (full file — read entirely; TS1 spans lines ~31-62, TS2 spans lines ~66+)
    - apps-script/src/triggers/showSearchSidebar.ts (read ONLY to confirm the inline `runSearch` success handler enqueues `pushRecentSearch`. Read-only — do NOT modify this trigger. If pushRecentSearch is NOT the actual enqueued follow-up call, use the actual method name and surface as deviation note. Per CONTEXT.md D-01: opening this file is read-only for fixture-derivation — do NOT edit.)
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §4 (locked choice: explicit resolve in test, NOT afterEach mock-contract change)
    - .planning/phases/08-test-infra-persistence-docs/08-REVIEW.md §WR-04 (pending-mock-queue leak symptom)
  </read_first>
  <behavior>
    - Inside TS1 (the happy-path "initial-data loads, user searches, results render" test), immediately after the existing `m.dispatchRunCall('runSearch', { ... })` block (lines 52-58) and BEFORE the `const results = m.document.getElementById('results')!;` line, add a single explicit resolve: `m.dispatchRunCall('pushRecentSearch', null);`.
    - The TS1 assertion `expect(results.innerHTML).toContain('Bone Helm');` is unchanged.
    - TS2 (the error path) is unchanged — it does not enqueue a follow-up call.
    - DO NOT add an `afterEach(() => mock.drainPending())` helper to the mock contract — CONTEXT.md §4 explicitly rejects that approach because it would require all 5 inline test files to remember to invoke it. Single-line explicit resolve in the one test that leaks is the locked choice.
  </behavior>
  <action>
    Open `apps-script/src/__tests__/searchSidebar.inline.test.ts` and locate TS1 (the first `it(...)` block, lines ~31-62). Inside TS1, find the `m.dispatchRunCall('runSearch', { ... });` block (lines 53-58). Immediately after the closing `});` of that dispatch, and before the `const results = m.document.getElementById('results')!;` line, add the explicit pushRecentSearch resolve. The exact diff:

    ```diff
        // Resolve runSearch — one matched group with the item name.
        m.dispatchRunCall('runSearch', {
          groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }],
          suggestions: [],
          coldFill: false,
          durationMs: 12,
        });
    +   // WR-04: drain the pushRecentSearch call that runSearch's success handler enqueued.
    +   m.dispatchRunCall('pushRecentSearch', null);

        const results = m.document.getElementById('results')!;
        expect(results.innerHTML).toContain('Bone Helm');
      });
    ```

    Before making the edit, briefly read `apps-script/src/triggers/showSearchSidebar.ts` (READ-ONLY) to confirm that `runSearch`'s `withSuccessHandler` enqueues a call named exactly `pushRecentSearch`. If the actual method name differs (e.g., `recordRecentSearch`, `bumpRecentSearches`), use the actual name and document the correction in the SUMMARY. If `runSearch`'s success handler enqueues MORE than one follow-up call, drain each in order with separate `m.dispatchRunCall(...)` lines. **DO NOT EDIT showSearchSidebar.ts under any circumstance — read-only for fixture-derivation only, per CONTEXT.md D-01.**

    Do NOT modify TS2. Do NOT modify the describe block, beforeEach, or imports.
  </action>
  <verify>
    <automated>cd apps-script && npm test -- --run __tests__/searchSidebar.inline.test.ts</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE "dispatchRunCall\('pushRecentSearch'" apps-script/src/__tests__/searchSidebar.inline.test.ts` matches exactly 1 line (or matches the actual method name if pushRecentSearch was a placeholder — verify via the actual showSearchSidebar.ts inline JS).
    - `cd apps-script && npm test -- --run __tests__/searchSidebar.inline.test.ts` reports both TS1 and TS2 PASS with NO "no pending X call" or "dangling pending call" warnings/errors from mountSidebar.
    - No new file is created; no other file modified.
    - The `showSearchSidebar.ts` file (production trigger) has zero diff. Verify via `git diff apps-script/src/triggers/showSearchSidebar.ts` returning empty.
    - Schema gates from Task 1 still hold.
  </acceptance_criteria>
  <done>TS1 explicitly drains the pushRecentSearch (or actual-name) pending call; pending mock queue exits TS1 empty; TS2 unchanged; showSearchSidebar.ts unchanged (read-only verification).</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| test realm → mountSidebar eval | Inline-script source is evaluated in the test realm via indirect eval. The IIFE wrap (Task 1) contains var/function declarations inside the IIFE scope, preventing them from polluting `globalThis`. Source is trusted (authored in repo's `apps-script/src/triggers/`). |
| test fixtures → production code | None. This plan modifies only `apps-script/src/__tests__/` files. No production code under `apps-script/src/triggers/` or `apps-script/src/lib/` is touched (CONTEXT.md D-01 constraint). |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-10-01-01 | Tampering | mountSidebar eval realm | mitigate | IIFE wrap (Task 1) scopes top-level `var` / `function` declarations to the IIFE — they no longer attach to test-realm `globalThis`. Closes the WR-01 leak vector at source rather than reactively cleaning up between tests. |
| T-10-01-02 | Information Disclosure | test fixture data | accept | All fixtures in this plan use placeholder emails (`newadmin@example.com`, `existing@example.com` in 10-02 downstream) and synthetic character names. No PII; no real guildie data. Test-only surface. |
| T-10-01-03 | Denial of Service | leaked pending mock calls | mitigate | Task 4 explicitly drains the `pushRecentSearch` (or actual-name) call queued by `runSearch`'s success handler, preventing test-state bleed into subsequent tests in the same vitest run. The other 3 inline test files are already drain-clean. |
| T-10-01-04 | Repudiation | swallowed assertion failures | mitigate | Task 3 removes the try/catch around `searchIndex.test.ts` Test 4's plan-locked assertion. Future regressions surface as visible test failures rather than passing silently. |

**Schema impact:** NONE. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` is unchanged (verifier grep gate: `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1).

**Scope-creep tripwire (per CONTEXT.md D-01):** if at any point during this plan the executor opens a file under `apps-script/src/triggers/` or `apps-script/src/lib/` and starts editing (not read-only inspection), STOP and surface the finding to Plan 10-03's ship-gate checkpoint. Read-only inspection of `showSearchSidebar.ts` in Task 4 to derive the exact pending-call name IS allowed (CONTEXT.md §Specifics §4 explicitly contemplates this).
</threat_model>

<verification>
1. `cd apps-script && npx tsc --noEmit` exits 0 (typecheck clean).
2. `cd apps-script && npm test -- --run` exits 0; ALL inline-JS test cases pass (TB1+TB2, TC1+TC2, TE1+TE2, TS1+TS2 = 8 cases); searchIndex.test.ts Test 4 either passes (preferred) or fails with a clear diff (acceptable per CONTEXT.md §3 — Plan 10-03 ship gate decides on the deviation).
3. `grep -nE '\(0, eval\)\(.*\(function\(\)\{' apps-script/src/__tests__/test-helpers.ts` matches exactly 1 line (IIFE wrap landed).
4. `grep -nE '\(0, eval\)\(src\);' apps-script/src/__tests__/test-helpers.ts` returns 0 (old form gone).
5. `grep -nE "toContain\('Marked 2 character\(s\) as removed'\)" apps-script/src/__tests__/evictionSidebar.inline.test.ts` matches exactly 1 line.
6. `grep -nE 'planLockedPasses' apps-script/src/__tests__/searchIndex.test.ts` returns 0.
7. `grep -nE "dispatchRunCall\('pushRecentSearch'" apps-script/src/__tests__/searchSidebar.inline.test.ts` matches exactly 1 line (or the actual method name verified against showSearchSidebar.ts).
8. `git diff apps-script/src/triggers/ apps-script/src/lib/` returns empty (no production code touched — CONTEXT.md D-01).
9. `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1 (schema lock).
10. `grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts` returns ≥1 (schema lock).
</verification>

<success_criteria>
- All 4 WR-* warning-level findings from 08-REVIEW.md are surgically closed (1:1 fix per task).
- The mountSidebar IIFE wrap is the function-expression form (NOT arrow) so `this === window` semantics survive.
- All 8 existing inline-JS test cases remain green after the IIFE wrap.
- searchIndex.test.ts Test 4 either passes outright (preferred) OR fails with a clear diff (acceptable per CONTEXT.md §3 — re-introducing try/catch is forbidden).
- searchSidebar.inline.test.ts TS1 exits with an empty pending-mock queue.
- Zero production code edited (`apps-script/src/triggers/` + `apps-script/src/lib/` diff is empty).
- Schema constants unchanged (`_meta.schema_version = 3`; `WatcherMaxSchemaVersion = 3`).
- Plan 10-02 can now safely consume the IIFE-fixed `mountSidebar` for its new Admin-Mgmt inline test.
</success_criteria>

<output>
After completion, create `.planning/phases/10-apps-script-test-quality/10-01-SUMMARY.md` summarizing:
- The 4 surgical edits (WR-01 IIFE wrap, WR-02 toContain, WR-03 try/catch removal, WR-04 explicit drain) by file + line range.
- Verification of all must_haves truths.
- Confirmation that no file under `apps-script/src/triggers/` or `apps-script/src/lib/` was modified (`git diff` evidence).
- Confirmation that `WatcherMaxSchemaVersion = 3` and `_meta.schema_version = 3` are unchanged.
- Pre/post inline-JS test counts (must be identical — no test additions or deletions in this plan).
- Note for Plan 10-02: "The IIFE-fixed mountSidebar is now safe to consume for the new Admin-Mgmt inline test. No state bleed between tests."
- If Task 3's WR-03 removal caused Test 4 to fail, surface the failure as a Phase 10 deviation note + propose v1.1 backlog item 999.30 (didYouMean vs whole-string Levenshtein contract mismatch).
</output>
</content>
</invoke>