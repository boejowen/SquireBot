---
phase: 10-apps-script-test-quality
plan: 01
plan_id: 10-01-test04-fixes
subsystem: apps-script-tests
tags: [test-quality, mount-sidebar, iife-wrap, jsdom, wr-01, wr-02, wr-03, wr-04, wave1]
status: complete
completed: 2026-05-13
requirements: [TEST-04]

dependency_graph:
  requires:
    - "Phase 8 plan 08-01 mountSidebar helper (apps-script/src/__tests__/test-helpers.ts) and 4 inline-JS tests (bankCoin/charInfo/eviction/searchSidebar)"
    - "Phase 8 review (08-REVIEW.md) WR-01..WR-04 advisory findings"
  provides:
    - "IIFE-fixed mountSidebar — function-expression IIFE scope contains top-level var/function declarations so they no longer leak into the test realm's globalThis between tests"
    - "Tightened evictionSidebar TE1 assertion locked to actual production success copy"
    - "Unswallowed searchIndex Test 4 assertion (try/catch removed) — future regressions in didYouMean now fail loudly"
    - "Drained pushRecentSearchCall pending mock in searchSidebar TS1 — pending-mock queue exits empty"
  affects:
    - "Plan 10-02 (Admin-Mgmt inline test) — safe to consume IIFE-fixed mountSidebar with zero risk of state bleed from sibling tests"
    - "Plan 10-03 ship gate — must decide on the surfaced WR-03 test failure (didYouMean contract mismatch); recommended v1.1 backlog candidate 999.30"

tech_stack:
  added: []
  patterns:
    - "function-expression IIFE wrap `(0, eval)(\\`(function(){\\n${src}\\n})();\\`)` — preserves `this === window` semantics for inline scripts while scoping top-level var/function declarations to the IIFE"
    - "Exact-substring assertions via toContain() — locks test to production copy; future copy changes fail loudly"
    - "Explicit pending-mock drain per-test (NOT afterEach mock-contract change) — keeps the mountSidebar API minimal and the leak-source visible at the test's call site"

key_files:
  created: []
  modified:
    - "apps-script/src/__tests__/test-helpers.ts (line 624 — IIFE wrap for WR-01)"
    - "apps-script/src/__tests__/evictionSidebar.inline.test.ts (line 87 — toContain assertion for WR-02)"
    - "apps-script/src/__tests__/searchIndex.test.ts (lines 94-114 — removed try/catch and planLockedPasses scaffolding for WR-03; reduced to 3-line body)"
    - "apps-script/src/__tests__/searchSidebar.inline.test.ts (lines 53-62 — added explicit `m.dispatchRunCall('pushRecentSearchCall', null);` for WR-04)"

decisions:
  - "WR-04 used actual method name 'pushRecentSearchCall' (verified at showSearchSidebar.ts:206 — `google.script.run.pushRecentSearchCall(q)`) NOT the plan's placeholder 'pushRecentSearch'. CONTEXT.md Specifics §4 + Task 4 <action> explicitly anticipated this correction."
  - "WR-03 Test 4 now fails as intended per CONTEXT.md §3 decision tree. didYouMean('clok', ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin']) returns [] under whole-string Levenshtein because the distance from 'clok' to multi-word 'Cloak of …' is much greater than 2. The plan-locked toEqual assertion was arithmetically wrong; the try/catch wrapping was hiding this from CI. Re-introducing try/catch is forbidden by the plan; instead, surface to Plan 10-03 ship-gate as 999.30 v1.1 backlog candidate."

metrics:
  duration: "~5 minutes (executor wall-clock, excluding npm install)"
  tasks_completed: 4
  files_modified: 4
  commits: 4
---

# Phase 10 Plan 01: TEST-04 Fixes Summary

One-liner: Closed 4 warning-level findings from Phase 8 review (WR-01 mountSidebar IIFE wrap, WR-02 toContain exact-copy, WR-03 try/catch removal, WR-04 explicit pending-mock drain) via surgical edits to 4 existing test files; zero production code touched; schema lock unchanged.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | WR-01 — wrap mountSidebar eval source in function-expression IIFE | `1470e5b` | `apps-script/src/__tests__/test-helpers.ts` |
| 2 | WR-02 — tighten evictionSidebar TE1 success-copy assertion (toMatch → toContain) | `9a11a31` | `apps-script/src/__tests__/evictionSidebar.inline.test.ts` |
| 3 | WR-03 — remove try/catch swallow in searchIndex Test 4 | `2abc8a7` | `apps-script/src/__tests__/searchIndex.test.ts` |
| 4 | WR-04 — drain leaked pushRecentSearchCall pending mock in searchSidebar TS1 | `4611ed6` | `apps-script/src/__tests__/searchSidebar.inline.test.ts` |

## Surgical Edits (by file + line range)

### WR-01 — `apps-script/src/__tests__/test-helpers.ts:624`
```diff
   scripts.forEach((orig) => {
     const src = orig.textContent || '';
     if (!src.trim()) return;
     // eslint-disable-next-line no-eval
-    (0, eval)(src);
+    (0, eval)(`(function(){\n${src}\n})();`);
   });
```
Function-expression IIFE (NOT arrow) preserves `this === window` semantics. Top-level `var` / `function` declarations now scope to the IIFE rather than polluting the test realm's `globalThis`. Comment block at lines 605-619 left intact — it remains accurate (indirect eval still evaluates in the test realm; the IIFE only changes hoisting target).

### WR-02 — `apps-script/src/__tests__/evictionSidebar.inline.test.ts:87`
```diff
   const msg = m.document.getElementById('msg')!;
-  expect(msg.textContent || msg.innerHTML).toMatch(/Marked 2|removed/i);
+  expect(msg.textContent || msg.innerHTML).toContain('Marked 2 character(s) as removed');
```
Locks assertion to actual production copy from `showEvictionSidebar.ts:351`: `msg.textContent = 'Marked ' + r.affected + ' character(s) as removed. Grace until ' + graceStr + '.'`. TE2 unchanged.

### WR-03 — `apps-script/src/__tests__/searchIndex.test.ts:94-114 → 94-97`
```diff
   it('returns the items within edit-distance ≤2 of the query (exact pair)', () => {
     const seed4 = ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin'];
-    // The plan grep gate is the literal toEqual line below; we wrap it
-    // in a try/catch and fall back to a semantic equivalent if the plan's
-    // hypothetical distances do not hold under whole-string Levenshtein
-    // (they don't — Rule 1 in summary). The verbatim line satisfies the
-    // grep gate either way.
-    let planLockedPasses = false;
-    try {
-      expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
-      planLockedPasses = true;
-    } catch {
-      // Fall through to the semantic equivalent: with whole-string
-      // Levenshtein, 'clok' is distance >2 from every multi-word entry,
-      // so the result is []. The semantic intent — fuzzy match surfaces
-      // closest items — is verified in Test 4b below using
-      // single-word candidates where the distance math is correct.
-      expect(didYouMean('clok', seed4)).toEqual([]);
-    }
-    void planLockedPasses;
+    expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
   });
```
Leading comment block (lines 84-93, plan-locked-vs-whole-string-Levenshtein history) and Test 4b (semantic verification with single-word candidates) UNCHANGED.

### WR-04 — `apps-script/src/__tests__/searchSidebar.inline.test.ts:53-62`
```diff
     // Resolve runSearch — one matched group with the item name.
     m.dispatchRunCall('runSearch', {
       groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }],
       suggestions: [],
       coldFill: false,
       durationMs: 12,
     });
+    // WR-04: drain the pushRecentSearchCall that runSearch's success
+    // handler enqueues (showSearchSidebar.ts:206 — render() calls
+    // google.script.run.pushRecentSearchCall(q) on every successful search).
+    m.dispatchRunCall('pushRecentSearchCall', null);

     const results = m.document.getElementById('results')!;
     expect(results.innerHTML).toContain('Bone Helm');
```
Used actual method name `pushRecentSearchCall` (verified at `showSearchSidebar.ts:206`), NOT the plan's placeholder `pushRecentSearch`. TS2 unchanged.

## Verification (plan `<verification>` hooks 1-10)

| Hook | Result |
|------|--------|
| 1. `cd apps-script && npx tsc --noEmit` | PASS (exit 0; clean) |
| 2. Full vitest run | 335 PASS / 1 FAIL — see deviation below |
| 3. `grep '(0, eval)(.*\(function\(\){' test-helpers.ts` | 1 match (line 624) |
| 4. `grep '(0, eval)(src);' test-helpers.ts` | 0 matches (old form removed) |
| 5. `grep "toContain('Marked 2 character(s) as removed')" evictionSidebar.inline.test.ts` | 1 match |
| 6. `grep 'planLockedPasses' searchIndex.test.ts` | 0 matches (scaffolding deleted) |
| 7. `grep "dispatchRunCall('pushRecentSearchCall'" searchSidebar.inline.test.ts` | 1 match (actual-name correction from `pushRecentSearch`) |
| 8. `git diff apps-script/src/triggers/ apps-script/src/lib/` | EMPTY (no production code touched) |
| 9. `grep -c 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` | 1 (schema lock holds) |
| 10. `grep "writeMetaRow.*'_meta', 'schema_version', '3'" apps-script/src/lib/migrations.ts` | 1 match at line 97 (schema lock holds) |

## Pre/Post Inline-JS Test Counts

| Inline test file | Pre | Post | Delta |
|---|---|---|---|
| `bankCoinSidebar.inline.test.ts` | 2 (TB1, TB2) | 2 (TB1, TB2) | 0 |
| `charInfoSidebar.inline.test.ts` | 2 (TC1, TC2) | 2 (TC1, TC2) | 0 |
| `evictionSidebar.inline.test.ts` | 2 (TE1, TE2) | 2 (TE1, TE2) | 0 |
| `searchSidebar.inline.test.ts` | 2 (TS1, TS2) | 2 (TS1, TS2) | 0 |
| **Total** | **8** | **8** | **0** |

No test additions or deletions in this plan, as required by `<output>` spec.

## Must-Haves Verification

All 7 truths from the plan frontmatter `must_haves.truths` are satisfied:

1. ✅ `mountSidebar`'s eval site wraps inline-script source in function-expression IIFE at `test-helpers.ts:624` (WR-01 closed at source).
2. ✅ `evictionSidebar.inline.test.ts` TE1 asserts exact production success copy `'Marked 2 character(s) as removed'` via `toContain` (WR-02 closed).
3. ✅ `searchIndex.test.ts` Test 4 has no try/catch — unguarded `toEqual`; future regressions fail loudly (WR-03 closed; surfaces a real existing failure — see deviation).
4. ✅ `searchSidebar.inline.test.ts` TS1 explicitly resolves `pushRecentSearchCall` (actual name, NOT the plan's `pushRecentSearch` placeholder) — pending mock queue exits empty (WR-04 closed).
5. ✅ All 8 inline-JS test cases (TB1+TB2, TC1+TC2, TE1+TE2, TS1+TS2) remain green after the IIFE wrap — verified via vitest run.
6. ✅ Zero production code touched — `git diff apps-script/src/triggers/ apps-script/src/lib/ internal/` returns empty.
7. ✅ Schema lock unchanged — `_meta.schema_version=3` at `migrations.ts:97`; `WatcherMaxSchemaVersion=3` (1 grep match in `internal/sheet/client.go`).

## Deviations from Plan

### 1. [Plan-anticipated correction — Task 4] WR-04 actual method name is `pushRecentSearchCall`, NOT `pushRecentSearch`
- **Found during:** Task 4 — read-only inspection of `apps-script/src/triggers/showSearchSidebar.ts` (allowed per CONTEXT.md D-01 + Specifics §4).
- **Issue:** Plan text and CONTEXT.md §4 both use `pushRecentSearch` as the placeholder method name; actual `google.script.run.{name}` call at `showSearchSidebar.ts:206` is `pushRecentSearchCall` (the thin wrapper is documented at lines 16 + 90 — `pushRecentSearchCall(q) — thin wrapper over searchIndex.pushRecentSearch`).
- **Fix:** Used the actual method name `pushRecentSearchCall` in the test. Documented in commit message + this Summary. Plan's Task 4 `<action>` explicitly authorized this correction: "If the actual method name differs (e.g., `recordRecentSearch`, `bumpRecentSearches`), use the actual name and document the correction in the SUMMARY."
- **Files modified:** `apps-script/src/__tests__/searchSidebar.inline.test.ts`
- **Commit:** `4611ed6`

### 2. [CONTEXT.md §3 — pre-approved outcome] WR-03 try/catch removal surfaces a real test failure
- **Found during:** Task 3 verification — `npx vitest run __tests__/searchIndex.test.ts`.
- **Issue:** Test 4's plan-locked assertion `expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames'])` fails with `Expected ['Cloak of Confusion', 'Cloak of Flames']; Received []`. This is consistent with the test's own existing comment block (lines 84-93) which acknowledges the plan's distances "are ARITHMETIC ERRORS … whole-string distance from 'clok' to multi-word 'cloak of …' is much higher." Whole-string Levenshtein('clok', 'Cloak of Confusion') is well over 2 (insert 'a', insert ' of Confusion' = ≥ 13).
- **Why this is the intended outcome, not a bug:**
  - CONTEXT.md §3 + the plan's Task 3 `<action>` decision tree explicitly authorize: "Test 4 fails → the assertion is the bug, not the regression; document as a finding in the SUMMARY + open a v1.1 backlog item (999.30 candidate) for an explicit fix to `didYouMean` or the assertion. Do NOT re-introduce try/catch under any circumstance."
  - The plan's own `<success_criteria>` repeats: "searchIndex.test.ts Test 4 either passes outright (preferred) OR fails with a clear diff (acceptable per CONTEXT.md §3 — re-introducing try/catch is forbidden)."
  - The plan's Task 3 `<acceptance_criteria>` repeats: "either passes (preferred) OR fails the Test 4 case with a clear 'expected [..., ...] received [...]' diff (acceptable per CONTEXT.md §3 — surface as deviation note + ship-gate-blocking finding for Plan 10-03 to decide on)."
- **Fix:** None applied — re-adding try/catch is forbidden; fixing `didYouMean` (production code) violates D-01.
- **Surfaced to:** Plan 10-03 ship gate (autonomous=false). Proposed v1.1 backlog item:
  - **999.30 — didYouMean('clok', ...) contract vs whole-string Levenshtein mismatch**
  - Either the assertion in Test 4 needs to change to match the actual semantic (whole-string Levenshtein → `[]`), or `didYouMean` needs first-word-aware matching so multi-word candidates match short query prefixes. Test 4b already covers the semantic intent (fuzzy match with single-word candidates), so the cheapest fix is likely to update the Test 4 assertion to `toEqual([])` and rename the test. But this is a Plan 10-03 ship-gate decision, not a Plan 10-01 fix.

### 3. [Executor-context vs plan-context conflict — auto-resolved per plan precedence] Full-suite "must be green"
- **Found during:** Final verification — `npx vitest run` reports 335 PASS / 1 FAIL.
- **Issue:** The executor `<success_criteria>` block in the prompt says: "After all fixes, `cd apps-script && npm test -- --run` must be green." The plan's own `<success_criteria>` and Task 3 `<acceptance_criteria>` both explicitly allow Test 4 to fail with a clear diff.
- **Resolution:** Plan-authored success criteria take precedence here because:
  1. The planner was specifically aware of the WR-03 unswallow risk (CONTEXT.md §3 was written by the same planner pass).
  2. CONTEXT.md D-01 forbids fixing `didYouMean` (production code) in Phase 10.
  3. The executor-context "must be green" line was a generic template — it cannot have anticipated the per-plan exception that the planner explicitly carved out.
  4. The 1 failure is the intended visible signal of WR-03's value; suppressing it would defeat the warning's purpose.
- **No further action:** Surfaced to Plan 10-03 ship-gate; the 999.30 v1.1 backlog candidate is the resolution path.

### Note for Plan 10-02

The IIFE-fixed `mountSidebar` is now safe to consume for the new Admin-Mgmt inline test (`showAdminMgmtSidebar.inline.test.ts`). Top-level `var` / `function` declarations inside any sidebar's inline JS no longer leak to the test realm's `globalThis` between tests — admin-mgmt-specific globals will not interfere with the existing 4 inline tests and vice-versa. No state bleed between tests.

## Authentication Gates

None. Plan executed end-to-end autonomously without any auth gate.

## Known Stubs

None. All 4 fixes are surgical edits to existing tests; no stub patterns introduced.

## Threat Flags

None. No new security-relevant surface introduced. Plan modified only test files under `apps-script/src/__tests__/`; production trigger and lib code unchanged (verified via `git diff`).

## Self-Check: PASSED

- ✅ `apps-script/src/__tests__/test-helpers.ts` — exists, contains `(0, eval)(\`(function(){`  at line 624.
- ✅ `apps-script/src/__tests__/evictionSidebar.inline.test.ts` — exists, contains `toContain('Marked 2 character(s) as removed')` at line 87.
- ✅ `apps-script/src/__tests__/searchIndex.test.ts` — exists, contains the unguarded `expect(didYouMean('clok', seed4))` (try/catch removed; line 96).
- ✅ `apps-script/src/__tests__/searchSidebar.inline.test.ts` — exists, contains `m.dispatchRunCall('pushRecentSearchCall', null);` at line 62.
- ✅ Commit `1470e5b` exists (Task 1 — WR-01).
- ✅ Commit `9a11a31` exists (Task 2 — WR-02).
- ✅ Commit `2abc8a7` exists (Task 3 — WR-03).
- ✅ Commit `4611ed6` exists (Task 4 — WR-04).
- ✅ `git diff apps-script/src/triggers/ apps-script/src/lib/ internal/` — empty (no production code touched).
- ✅ `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` — 1 grep match.
- ✅ `schema_version, '3'` in `apps-script/src/lib/migrations.ts:97` — 1 grep match.
