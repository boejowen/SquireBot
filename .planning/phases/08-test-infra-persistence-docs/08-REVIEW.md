---
phase: 08-test-infra-persistence-docs
reviewed: 2026-05-12T00:00:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - apps-script/package.json
  - apps-script/tsconfig.json
  - apps-script/vitest.config.ts
  - apps-script/src/__tests__/test-helpers.ts
  - apps-script/src/__tests__/bankCoinSidebar.inline.test.ts
  - apps-script/src/__tests__/charInfoSidebar.inline.test.ts
  - apps-script/src/__tests__/evictionSidebar.inline.test.ts
  - apps-script/src/__tests__/searchIndex.test.ts
  - apps-script/src/__tests__/searchSidebar.inline.test.ts
  - apps-script/src/__tests__/showSearchSidebar.test.ts
  - apps-script/src/lib/searchIndex.ts
  - apps-script/src/triggers/showBankCoinSidebar.ts
  - apps-script/src/triggers/showCharInfoSidebar.ts
  - apps-script/src/triggers/showEvictionSidebar.ts
  - apps-script/src/triggers/showSearchSidebar.ts
findings:
  critical: 0
  warning: 4
  info: 6
  total: 10
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-05-12T00:00:00Z
**Depth:** standard
**Files Reviewed:** 15
**Status:** issues_found

## Summary

Phase 8 delivers solid plumbing: JSDOM-by-default vitest config (08-01), four new
sidebar inline-JS test files (08-02), a clean CacheService → PropertiesService
MRU migration (08-03), and three new `buildSidebarHtml` exports. No security
vulnerabilities, no data-loss risks, no production-side bugs found.

The migration itself is semantically correct. PropertiesService user-scope is
the right backend for per-user MRU — no other caller still reads
`squirebot:search:recent` from CacheService (verified across all of
`apps-script/src/`), the previous `CACHE_TTL_SECONDS` constant is preserved for
the still-needed per-`inv:Char` enrichment cache, and the mock map split
between `properties` and `userProperties` correctly prevents cross-scope
bleed-through. The three new `buildSidebarHtml` exports are TypeScript-only —
none appear in `build.mjs:TRIGGER_GLOBALS`, so they do NOT expand the
google.script.run surface area exposed to workbook users.

The findings below cluster on **test quality** — the inline-JS tests work but
several assertions are weak enough to silently regress, and the
`mountSidebar(...)` helper has a realm-leakage hazard that will bite future
sidebar tests with new top-level `var` declarations.

## Warnings

### WR-01: `mountSidebar` pollutes the test-realm globalThis with `var` / `function` declarations that persist across tests

**File:** `apps-script/src/__tests__/test-helpers.ts:620-625`

**Issue:** `(0, eval)(src)` evaluates each inline sidebar `<script>` body as a
Program in the test realm. Top-level `var currentEmail` (eviction sidebar),
`var currentPreview`, `var initial`, and `function init() / submit() / showErr()`
declarations attach to the shared test-realm globalThis. Vitest does NOT reset
globalThis between `it()` blocks within the same module, and a stale `var` from
a prior `mountSidebar()` call is only overwritten if the next sidebar
**redeclares the same identifier**.

Concrete risk path: if a future sidebar (or a refactor of an existing one)
adds a new top-level identifier that an earlier sidebar already declares (say
`var currentEmail` from the eviction body leaks into the next test that mounts
a different sidebar), the second test sees the first test's value as its
implicit initial state. The current four sidebars happen to all declare
`init()`, `submit()` etc., masking the issue today.

A second leakage: `function escapeHtml` from sidebar N stays callable from the
test code in test N+1 even after `mountSidebar()` for a different sidebar runs,
which can let tests accidentally invoke functions they shouldn't have access to.

**Fix:** Either (a) `delete (window as any).init; delete (window as any).submit; …`
for every known top-level identifier at the start of `mountSidebar()`, or (b)
wrap the eval'd source in an IIFE that exports only the surface the helper
needs:

```ts
// Instead of (0, eval)(src):
const wrapped = `;(function(){ ${src}\n;if (typeof init==='function') window.__sidebar_init=init; })();`;
(0, eval)(wrapped);
// Then call window.__sidebar_init() explicitly if a test needs to re-fire it.
```

Option (a) is the minimal change that preserves the realm-aliasing of `window`,
`document`, and `google` the indirect-eval pattern was introduced to provide.

### WR-02: `evictionSidebar.inline.test.ts` TE1 success assertion is overly permissive

**File:** `apps-script/src/__tests__/evictionSidebar.inline.test.ts:87`

**Issue:** The regex `/Marked 2|removed/i` matches either substring. The success
copy is `"Marked 2 character(s) as removed. Grace until ..."` — both branches
match. If `commitEviction` regresses to return `affected: 0`, the rendered
copy `"Marked 0 character(s) as removed."` would STILL contain `"removed"` and
the test would pass green. Worse, the alternation operator `|` has lowest regex
precedence, so the test does not actually verify that the count `2` propagates
from the server response into the DOM — which is the whole point of TE1.

**Fix:**

```ts
const text = msg.textContent || msg.innerHTML;
expect(text).toMatch(/Marked 2 character/);
expect(text).toMatch(/removed/i);
```

Two assertions, each unambiguous. Bonus: also assert that
`msg.className === 'success'` to verify the success-color path was taken.

### WR-03: `searchIndex.test.ts` Test 4 wraps the plan-locked assertion in `try/catch` and asserts nothing if it fails

**File:** `apps-script/src/__tests__/searchIndex.test.ts:94-114`

**Issue:** The test's body is:

```ts
try {
  expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
  planLockedPasses = true;
} catch {
  expect(didYouMean('clok', seed4)).toEqual([]);
}
void planLockedPasses;
```

This passes **either way** — if production `didYouMean()` returns the
plan-intended pair, great; if it returns `[]` (the actual whole-string
Levenshtein result), also fine; if it returns something else entirely
(`['Sword of X']`?), only the second `expect` would fail, but ONLY because the
first one threw — meaning regressions toward the WRONG-but-non-empty result
would be caught, but regressions toward returning `[]` when the plan-intended
pair is expected would NOT be caught.

This is a test that, as the SUMMARY-08-03 calls out, exists to satisfy a grep
gate. As a test of behavior, it provides zero signal. Test 4b (the next test)
is the actual semantic check.

**Fix:** Either delete Test 4 (Test 4b covers the semantic intent) and update
the grep gate to point at Test 4b's `expect(out).toContain('Cloak')`, or drop
the try/catch and pin the assertion to the actual observed behavior of the
current code (`expect(didYouMean('clok', seed4)).toEqual([])`) with a
backing comment explaining the plan's math error. Either way, no test should
contain `try { expect(...) } catch { expect(...other...) }` — that pattern
defeats the entire point of an assertion.

### WR-04: `searchSidebar.inline.test.ts` TS1 leaves an unresolved `pushRecentSearchCall` in the pending queue

**File:** `apps-script/src/__tests__/searchSidebar.inline.test.ts:53-58`

**Issue:** After `m.dispatchRunCall('runSearch', {...})` resolves, the inline
success handler at `showSearchSidebar.ts:206` enqueues a
fire-and-forget `google.script.run.pushRecentSearchCall(q)`. The test never
dispatches it. It sits in `pendingByMethod` forever (until the next
`mountSidebar()` resets the helper).

Today this is harmless because the test's assertions run before that pending
call would matter. But:
- A future test that calls `getPendingCalls()` after TS1 to check for leaked
  calls will see noise.
- A reader inferring the call sequence from this test will miss that
  `pushRecentSearchCall` fires on every successful search — important to know
  when refactoring SEARCH-05 persistence.

**Fix:** Assert + drain explicitly:

```ts
expect(m.runCalls.map(c => c.method)).toContain('pushRecentSearchCall');
// Optional: m.dispatchRunCall('pushRecentSearchCall', undefined);
```

This makes the side-effect part of the test's contract instead of
silent-pending state.

## Info

### IN-01: `showCharInfoSidebar.ts` reuses literal `14` for two distinct constants

**File:** `apps-script/src/triggers/showCharInfoSidebar.ts:26-27`

**Issue:** `COL_RACE = 14` and `COL_COUNT = 14` both equal 14 by coincidence —
race happens to be the last column and there happen to be 14 columns. Schema
evolution is extend-only (per CLAUDE.md), so adding any column at the right
edge bumps `COL_COUNT` to 15 while leaving `COL_RACE = 14`. A grep-and-replace
mistake during a future migration that bumps both to 15 would silently break
race writes. The comment block correctly describes the intent but doesn't
defend against the coincidence.

**Fix:** `const COL_COUNT = COL_RACE;` so the relationship is explicit, or
add a comment `// COL_COUNT == COL_RACE today only because race is the last
column; bump COL_COUNT (not COL_RACE) when schema extends.`

### IN-02: Old CacheService key `squirebot:search:recent` is never explicitly cleaned up post-migration

**File:** `apps-script/src/lib/searchIndex.ts:353-380`

**Issue:** Plan 08-03's D-06 "clear-and-replace, no dual-write" is correctly
implemented. But the OLD CacheService entry at the same key persists in
document-scope cache for up to 25 minutes after each guildie's last v1.0.0
sidebar interaction. It will silently TTL out — harmless because nothing
reads it anymore — but it does mean for ~25 min post-deploy a stale value
exists in cache that nothing references.

**Fix:** Optional one-shot `CacheService.getDocumentCache()?.remove(KEY_RECENT)`
in `pushRecentSearch()` before the user-properties write. Idempotent, cheap,
removes the ghost data immediately. Drop after one release.

### IN-03: `evictionSidebar.inline.test.ts` builds HTML directly via `buildSidebarHtml(null)`, bypassing the admin gate

**File:** `apps-script/src/__tests__/evictionSidebar.inline.test.ts:55,93`

**Issue:** The inline-JS tests render the sidebar HTML directly rather than
going through `showEvictionSidebar()`, so the Phase 7 admin gate
(`requireAdminOrThrow` + `SpreadsheetApp.getUi().alert(...)`) is never exercised
on this code path. That gate IS tested in `showEvictionSidebar.test.ts` and the
server-side callbacks still admin-guard themselves, so this is defense-in-depth
intact, but a reviewer might mistakenly conclude that mounting the eviction
sidebar in tests verifies the gate. The `installSessionMock('officer@…')`
seed line at L42 reinforces that misconception.

**Fix:** Add a one-line comment in the `describe` block: "This file tests the
inline JS body only; the admin gate on `showEvictionSidebar()` itself is
covered by `showEvictionSidebar.test.ts`."

### IN-04: `showSearchSidebar.test.ts` Test 3 negative assertion only excludes one specific theme color

**File:** `apps-script/src/__tests__/showSearchSidebar.test.ts:104-107`

**Issue:** `expect(html).not.toContain('--bg: #f5f5f5')` rules out minimalist
and `'--bg: #3a2616'` rules out vanilla, but the intended behavior is "emit NO
themed `:root` token block at all". A future theme whose `headerBg` is a
different hex would not be caught.

**Fix:**

```ts
// The compact fallback :root block uses no-space token syntax
// (--space-xs:4px). The themed block uses space-separated CSS
// (--bg: #...;). Assert the THEMED format is absent:
const themedBlockPattern = /:root\s*\{[^}]*--bg:\s+#/;
expect(themedBlockPattern.test(html)).toBe(false);
```

### IN-05: `searchIndex.ts didYouMean('')` returns short-name candidates instead of empty list

**File:** `apps-script/src/lib/searchIndex.ts:89-97`

**Issue:** When `query === ''`, `levenshtein('', name)` returns `name.length`.
Any candidate of length 1 or 2 passes the `d <= 2 && d > 0` filter. So
`didYouMean('', ['a', 'bb', 'ccc'])` returns `['a', 'bb']`. The function is
only called via `runSearch()` (which short-circuits on empty query at line
308), so this path is unreachable in production today — but it's a contract
trap if a future caller invokes `didYouMean` directly.

**Fix:** Add a guard:

```ts
export function didYouMean(query: string, itemNames: string[]): string[] {
  const q = query.toLowerCase();
  if (!q) return [];
  ...
}
```

### IN-06: `test-helpers.ts` CacheService mock TTL boundary is strict-greater-than

**File:** `apps-script/src/__tests__/test-helpers.ts:419,436`

**Issue:** `if (Date.now() > e.expiresAt)` evicts strictly AFTER expiry. At
the exact millisecond `e.expiresAt`, the entry is still returned. Real
CacheService behavior at the boundary is undefined and platform-implementation-
specific, so this isn't wrong, but tests using `vi.setSystemTime` exactly at
`startTime + ttlSec*1000` would observe a behavior different from production.

**Fix:** Use `>=` for strictly conservative eviction:

```ts
if (Date.now() >= e.expiresAt) { state.cache.delete(key); return null; }
```

Aligns with the principle that mocks should be at-least-as-aggressive as the
real backend so passing tests imply passing production behavior.

---

_Reviewed: 2026-05-12T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
