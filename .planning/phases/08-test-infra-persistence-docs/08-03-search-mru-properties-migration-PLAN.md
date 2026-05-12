---
phase: 08-test-infra-persistence-docs
plan: 03
type: execute
wave: 1
depends_on: []
files_modified:
  - apps-script/src/lib/searchIndex.ts
  - apps-script/src/__tests__/searchIndex.test.ts
autonomous: true
requirements: [SEARCH-05]
tags: [apps-script, search-index, properties-service, mru, persistence-migration, search-05]
must_haves:
  truths:
    - "`getRecentSearches()` in `apps-script/src/lib/searchIndex.ts` reads from `PropertiesService.getUserProperties()` instead of `CacheService.getDocumentCache()`."
    - "`pushRecentSearch(q)` in the same file writes to `PropertiesService.getUserProperties().setProperty(KEY_RECENT, ...)` and does NOT call `CacheService.put(...)` with the TTL arg (D-06 clear-and-replace: no dual-write)."
    - "`CacheService.getDocumentCache()` is still called from `searchIndex.ts` at exactly 2 of the previously-3 call sites: `prewarmSearchCache` (~line 176) and `runSearch` (~lines 310-311) -- these are the per-`inv:Char` enrichment cache, NOT MRU."
    - "`CACHE_TTL_SECONDS = 60` constant remains defined and used by the surviving per-char cache call sites; not deleted (per Pitfalls #5 in 08-RESEARCH -- the same constant is referenced by `buildInvCache`)."
    - "Existing `searchIndex.test.ts` tests for `getRecentSearches` / `pushRecentSearch` (currently tests 16-17 around lines 290-310) still pass without body changes thanks to the `getUserProperties()` mock alias landed in Plan 08-01."
    - "A new persistence-across-TTL-elapse test asserts that pushRecentSearch + getRecentSearches survives a simulated 25-minute time advance via `vi.useFakeTimers()` + `vi.setSystemTime()`."
    - "Schema gates untouched: `_meta.schema_version=3`, `WatcherMaxSchemaVersion=3`."
  artifacts:
    - path: "apps-script/src/lib/searchIndex.ts"
      provides: "Per-user persistent recent-search MRU via PropertiesService"
      contains: "PropertiesService.getUserProperties()"
    - path: "apps-script/src/__tests__/searchIndex.test.ts"
      provides: "Updated mock seed sites + new persistence-across-TTL test"
      contains: "useFakeTimers"
  key_links:
    - from: "apps-script/src/lib/searchIndex.ts:getRecentSearches"
      to: "PropertiesService.getUserProperties().getProperty(KEY_RECENT)"
      via: "named-API call to Apps Script PropertiesService"
      pattern: "PropertiesService\\.getUserProperties\\(\\)"
    - from: "apps-script/src/lib/searchIndex.ts:pushRecentSearch"
      to: "PropertiesService.getUserProperties().setProperty(KEY_RECENT, value)"
      via: "named-API call without TTL arg"
      pattern: "props\\.setProperty\\(KEY_RECENT"
    - from: "apps-script/src/__tests__/searchIndex.test.ts:recent-searches block"
      to: "apps-script/src/__tests__/test-helpers.ts:getUserProperties() mock"
      via: "globalThis.PropertiesService.getUserProperties() Map-backed mock (landed in Plan 08-01)"
      pattern: "getUserProperties"
---

<objective>
Migrate the recent-3 search MRU in `apps-script/src/lib/searchIndex.ts` from `CacheService.getDocumentCache()` (document-scoped, ephemeral, TTL-evictable) to `PropertiesService.getUserProperties()` (per-user, durable across sessions and CacheService eviction). Per D-05/D-06 this is a clear-and-replace migration with no dual-write and no cache backfill — the worst-case UX impact is one empty `recent[]` array on a guildie's first search after v1.0.1 ships, which is a negligible single-session regression.

Purpose: REQUIREMENTS.md SEARCH-05 — recent-search history that survives the 25-minute CacheService default TTL. This closes a real user pain (closing the sidebar for >25 min then reopening shows an empty recent list, even though the guildie expects it to persist).

Output: a surgical 2-function swap in `searchIndex.ts` (~10 lines net) + a verification update + one new test asserting persistence across a simulated TTL boundary.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md
@.planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md
@.planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md
@apps-script/src/lib/searchIndex.ts
@apps-script/src/__tests__/searchIndex.test.ts

<interfaces>
<!-- The exact lines to change. Do NOT touch anything else in searchIndex.ts. -->

Current state in apps-script/src/lib/searchIndex.ts (lines 355-371):
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

Sites that MUST NOT change (per char inv cache; same file):
- Line ~176: `prewarmSearchCache` calls `CacheService.getDocumentCache()` for the per-char inv cache.
- Lines ~310-311: `runSearch` calls `CacheService.getDocumentCache()` for the per-char inv cache.
- Lines ~26-27: `CACHE_TTL_SECONDS = 60`, `MAX_CACHE_VALUE_BYTES`, `KEY_INV`, etc. -- all still consumed by `buildInvCache` / `prewarmSearchCache` / `runSearch`.

Wave-1 boundary note (per 08-RESEARCH Plan Structure Validation): Plan 08-01 lands the `getUserProperties()` mock alias in `test-helpers.ts` BEFORE this plan runs `npm test`. The mock is backed by `state.userProperties` (a separate Map from the existing `state.properties`). Since all three plans in Wave 1 (08-01, 08-03, 08-04) are merged via the same wave-boundary `npm test`, this plan can rely on the mock being present.

Existing test bodies (apps-script/src/__tests__/searchIndex.test.ts lines 290-310) -- public surface is unchanged so test bodies do NOT need to change:
```typescript
describe('recent searches', () => {
  beforeEach(() => { resetMocks(); });

  it('rolls forward in MRU order capped at 3', () => {
    pushRecentSearch('q1');
    pushRecentSearch('q2');
    pushRecentSearch('q3');
    pushRecentSearch('q4');
    expect(getRecentSearches()).toEqual(['q4', 'q3', 'q2']);
  });

  it('dedupes consecutive duplicate pushes', () => {
    pushRecentSearch('q1');
    pushRecentSearch('q1');
    expect(getRecentSearches()).toEqual(['q1']);
  });
});
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Swap getRecentSearches + pushRecentSearch from CacheService to PropertiesService.getUserProperties</name>
  <files>apps-script/src/lib/searchIndex.ts</files>
  <read_first>
    - apps-script/src/lib/searchIndex.ts (entire file, with attention to lines 26 [CACHE_TTL_SECONDS], 176 [prewarmSearchCache cache call], 310-311 [runSearch cache call], 355-371 [the surgical-change MRU pair])
    - .planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md §PropertiesService migration mechanics -- exact target code block
    - .planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md §apps-script/src/lib/searchIndex.ts -- surgical scope (DO NOT TOUCH sites at lines 26, 176, 310-311)
  </read_first>
  <behavior>
    - getRecentSearches() returns [] when PropertiesService.getUserProperties() returns null (defensive)
    - getRecentSearches() returns [] when the KEY_RECENT property doesn't exist
    - getRecentSearches() returns the parsed JSON array when KEY_RECENT exists with valid JSON
    - getRecentSearches() returns [] when JSON.parse throws on malformed data (catch block preserves resilience)
    - pushRecentSearch('') is a no-op (whitespace-trimmed empty string short-circuit unchanged)
    - pushRecentSearch('q1') after pushRecentSearch('q2') after pushRecentSearch('q3') leaves getRecentSearches() returning ['q3','q2','q1']
    - pushRecentSearch('q1') after pushRecentSearch('q1') deduplicates -- final state is ['q1'] (single entry)
    - pushRecentSearch never calls cache.put() and never passes a TTL arg
    - prewarmSearchCache and runSearch (the other two CacheService call sites in the same file) are UNCHANGED -- post-Plan grep `CacheService.getDocumentCache` in searchIndex.ts returns exactly 2 hits (was 3)
  </behavior>
  <action>
1. Open `apps-script/src/lib/searchIndex.ts` and locate the `// --- Recent searches ---` section (around line 353).

2. Replace ONLY the two functions `getRecentSearches` and `pushRecentSearch` with the PropertiesService-backed versions. Exact target text:

```typescript
// --- Recent searches ----------------------------------------------------
// SEARCH-05 (Phase 8 plan 08-03): per-user persistent MRU via
// PropertiesService.getUserProperties(). KEY_RECENT and the JSON-encoded
// string-array storage shape are unchanged; only the storage backend swaps
// from CacheService.getDocumentCache() (document-scoped, 25-min default
// eviction) to PropertiesService.getUserProperties() (per-user, durable).
// D-06: clear-and-replace -- no dual-write, no cache backfill. Worst-case
// UX is one empty recent[] on a guildie's first search after v1.0.1 ships.
// CACHE_TTL_SECONDS is NOT deleted -- prewarmSearchCache and runSearch
// still consume it for the per-`inv:Char` enrichment cache.

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

3. CONFIRM nothing else changed. Run these greps inside `apps-script/`:

```bash
# Exactly 2 surviving CacheService.getDocumentCache call sites in searchIndex.ts (was 3)
grep -c "CacheService.getDocumentCache" src/lib/searchIndex.ts
# Expect: 2

# CACHE_TTL_SECONDS constant still present
grep -c "CACHE_TTL_SECONDS" src/lib/searchIndex.ts
# Expect: >=2 (definition + at least 1 consumer)

# New PropertiesService call sites land
grep -c "PropertiesService.getUserProperties" src/lib/searchIndex.ts
# Expect: 2 (one in each of the two swapped functions)
```

4. Typecheck + run the searchIndex test file. The existing tests 16-17 (rolling-3, dedupe) should still pass without body changes because their assertions are public-surface assertions, and the Plan 08-01 `getUserProperties()` mock shares the same getProperty/setProperty/deleteProperty shape:

```bash
cd apps-script
npx tsc --noEmit 2>&1 | tail -5
npm test searchIndex 2>&1 | tail -15
```

If tests fail because `state.userProperties` wasn't reset in `resetMocks()`, fall back to verifying Plan 08-01's `newMockState()` initializer landed correctly (it should have `userProperties: new Map()` -- see 08-01 Task 2 acceptance criteria).

5. Run the full apps-script suite to confirm no regression in OTHER test files (some sidebar tests may seed `state.cache.has('squirebot:search:recent')` and now need to seed `state.userProperties.get('squirebot:search:recent')` instead — but per 08-RESEARCH §PropertiesService migration mechanics, no current test does this seeding directly; the public `pushRecentSearch` API is the only writer):

```bash
cd apps-script && npm test 2>&1 | tail -15
```

6. Commit:
```bash
git add apps-script/src/lib/searchIndex.ts
git commit -m "feat(08-03): migrate recent-search MRU from CacheService to PropertiesService.getUserProperties (SEARCH-05)"
```
  </action>
  <verify>
    <automated>
cd apps-script
# Two new PropertiesService call sites land
n=$(grep -c "PropertiesService.getUserProperties" src/lib/searchIndex.ts); [ "$n" -eq 2 ] || exit 1
# Cache call sites drop from 3 to 2 (prewarmSearchCache + runSearch survive)
n=$(grep -c "CacheService.getDocumentCache" src/lib/searchIndex.ts); [ "$n" -eq 2 ] || exit 1
# CACHE_TTL_SECONDS still present
grep -q "CACHE_TTL_SECONDS" src/lib/searchIndex.ts || exit 1
# pushRecentSearch no longer passes TTL arg (cache.put(KEY_RECENT, ..., TTL) gone)
grep -A 10 "^export function pushRecentSearch" src/lib/searchIndex.ts | grep -q "cache.put" && exit 1
grep -A 10 "^export function pushRecentSearch" src/lib/searchIndex.ts | grep -q "props.setProperty" || exit 1
# Typecheck + tests
npx tsc --noEmit 2>&1 | tail -3
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `grep -c "PropertiesService.getUserProperties" apps-script/src/lib/searchIndex.ts` returns exactly 2
    - `grep -c "CacheService.getDocumentCache" apps-script/src/lib/searchIndex.ts` returns exactly 2 (was 3 pre-Plan; prewarmSearchCache + runSearch survive; recent-MRU pair swapped)
    - `grep -c "CACHE_TTL_SECONDS" apps-script/src/lib/searchIndex.ts` returns at least 2 (constant definition + at least 1 surviving consumer)
    - `pushRecentSearch` body contains `props.setProperty(KEY_RECENT, ...)` and does NOT contain `cache.put(KEY_RECENT, ...)`
    - `getRecentSearches` body contains `props.getProperty(KEY_RECENT)` and does NOT contain `cache.get(KEY_RECENT)`
    - `cd apps-script && npx tsc --noEmit` exits 0
    - `cd apps-script && npm test` exits 0 -- all existing tests still pass (the public API surface is unchanged; the mock alias from Plan 08-01 transparently routes the writes)
  </acceptance_criteria>
  <done>Recent-MRU persists via PropertiesService.getUserProperties; per-char inv cache call sites in the same file are untouched; existing tests 16-17 pass without body changes.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add persistence-across-TTL test to searchIndex.test.ts</name>
  <files>apps-script/src/__tests__/searchIndex.test.ts</files>
  <read_first>
    - apps-script/src/__tests__/searchIndex.test.ts (entire file -- existing recent-searches describe block at lines 290-310)
    - apps-script/src/lib/searchIndex.ts (the post-Task-1 state, to confirm the API surface the new test exercises)
    - .planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md §searchIndex.test.ts MOD note -- the new test signature using vi.useFakeTimers()
  </read_first>
  <behavior>
    - New test inside the existing `describe('recent searches', ...)` block.
    - Test pushes one query, advances simulated time by 30 minutes (past the old CacheService 25-min default eviction), and asserts the query is still present.
    - With PropertiesService backing, the simulated time advance is irrelevant (no TTL) — the test passes trivially because the property persists.
    - With the OLD CacheService-backed implementation, this test would fail if the mock had TTL semantics (it doesn't currently — the test-helpers cache mock is a plain Map without time-based eviction — so the test is documentation-as-regression-guard rather than a failure trip).
    - The test name explicitly mentions SEARCH-05 / D-06 so future readers know its intent.
  </behavior>
  <action>
1. Open `apps-script/src/__tests__/searchIndex.test.ts`. Locate the existing `describe('recent searches', ...)` block (around lines 290-310). 

2. Inside the block, AFTER the existing `it('dedupes consecutive duplicate pushes', ...)` test, append:

```typescript
  // Phase 8 plan 08-03 (SEARCH-05 / D-06): persists across the legacy
  // CacheService 25-min default-eviction boundary. PropertiesService has no
  // TTL so this is structurally guaranteed; the test documents the
  // user-facing contract -- "my recent searches survive me closing the
  // sidebar for half an hour" -- so future readers don't accidentally
  // revert to a TTL-bounded backend.
  it('persists across simulated 25-min CacheService-TTL elapse (SEARCH-05)', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-12T00:00:00Z'));
    pushRecentSearch('persistent-query');
    vi.setSystemTime(new Date('2026-05-12T00:30:00Z'));  // +30min, past old 25-min TTL
    expect(getRecentSearches()).toEqual(['persistent-query']);
    vi.useRealTimers();
  });
```

3. Ensure `vi` is imported at the top of `searchIndex.test.ts`. If it isn't (current imports may be `{ describe, it, expect, beforeEach }`), extend the import:

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest';
```

4. Typecheck + run the targeted suite:

```bash
cd apps-script
npx tsc --noEmit 2>&1 | tail -5
npm test searchIndex 2>&1 | tail -15
```

Expected: existing 24 (or current baseline) tests + 1 new = N+1 green; the new test passes trivially because the property persists across `vi.setSystemTime`.

5. Run the full suite for regression sanity:

```bash
cd apps-script && npm test 2>&1 | tail -5
```

6. Commit:
```bash
git add apps-script/src/__tests__/searchIndex.test.ts
git commit -m "test(08-03): add persistence-across-TTL test for SEARCH-05 MRU migration"
```
  </action>
  <verify>
    <automated>
cd apps-script
grep -q "persists across simulated 25-min" src/__tests__/searchIndex.test.ts || exit 1
grep -q "vi.useFakeTimers" src/__tests__/searchIndex.test.ts || exit 1
grep -q "from 'vitest'" src/__tests__/searchIndex.test.ts || exit 1
# vi must be in the import list
grep -E "import.*\\{[^}]*vi[^}]*\\}.*from 'vitest'" src/__tests__/searchIndex.test.ts || exit 1
npx tsc --noEmit 2>&1 | tail -3
npm test 2>&1 | tail -5
    </automated>
  </verify>
  <acceptance_criteria>
    - `grep -c "persists across simulated 25-min" apps-script/src/__tests__/searchIndex.test.ts` returns at least 1
    - `grep -c "vi.useFakeTimers" apps-script/src/__tests__/searchIndex.test.ts` returns at least 1
    - The vitest import line in searchIndex.test.ts includes `vi` (regex `import.*\{[^}]*vi[^}]*\}.*from 'vitest'` matches)
    - `cd apps-script && npx tsc --noEmit` exits 0
    - `cd apps-script && npm test` exits 0 with the new test included (suite count grows by exactly 1 from the post-Task-1 state)
    - Schema gate: `grep -c "writeMetaRow.*schema_version.*'3'" apps-script/src/lib/migrations.ts` unchanged from Phase 7 baseline
    - Schema gate: `grep -c "WatcherMaxSchemaVersion.*=.*3" internal/sheet/client.go` returns at least 1 (unchanged)
  </acceptance_criteria>
  <done>New test documents and verifies SEARCH-05 persistence contract; suite count = post-Task-1 baseline + 1; typecheck clean.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Guildie's Google account ↔ PropertiesService.getUserProperties() | Recent-search history is now persistent per-user state. Previously document-scoped (ephemeral via CacheService TTL); now user-scoped (durable). |
| Workbook readers ↔ recent-search MRU | NO leak: getUserProperties is strictly per-user; the workbook owner cannot read another guildie's recent-search list from this storage path. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-08-03-01 | Information disclosure | Recent-search queries persist across sessions instead of expiring at 25 min | accept | This is the user-requested behavior (SEARCH-05). v1.0 privacy disposition already accepts that recent searches persist within the active session; SEARCH-05 extends durability to across-sessions. PropertiesService scope is per-user — no cross-guildie disclosure. |
| T-08-03-02 | Information disclosure | Wrong PropertiesService scope (getDocumentProperties vs getUserProperties) leaks one guildie's recent list to another | mitigate | Locked by D-05: `getUserProperties()` (per-user). Grep gate: `grep -c "PropertiesService.getUserProperties" searchIndex.ts` returns exactly 2, AND no `getDocumentProperties` calls in the same surgical-change site. Tests assert on the per-user mock backed by an isolated `state.userProperties` Map (Plan 08-01). |
| T-08-03-03 | Denial of service | PropertiesService quota exhaustion if recent-MRU grows unbounded | accept | Existing `.slice(0, RECENT_LIMIT)` cap holds growth at 3 entries × ~50 chars × JSON overhead ≈ 200 bytes. PropertiesService scope quota is 524,288 bytes (~25,000x headroom per 08-RESEARCH). No additional guard needed. |
| T-08-03-04 | Repudiation | The `CACHE_TTL_SECONDS` constant becomes orphaned and a future reader deletes it, breaking per-char cache | mitigate | The constant is still consumed by `prewarmSearchCache` (line 176) and `runSearch` (lines 310-311). Code-comment-in-place at the swap site documents this so the next person to touch the file knows not to delete the constant. |
</threat_model>

<verification>
After both tasks complete:

```bash
cd apps-script
# Surgical-scope correctness
grep -c "PropertiesService.getUserProperties" src/lib/searchIndex.ts  # = 2
grep -c "CacheService.getDocumentCache" src/lib/searchIndex.ts  # = 2 (was 3)
grep -c "CACHE_TTL_SECONDS" src/lib/searchIndex.ts  # >= 2

# Test landed
grep -q "persists across simulated 25-min" src/__tests__/searchIndex.test.ts

# Suite green
npx tsc --noEmit
npm test
```

Verification-hook 5 (schema-gate):
```bash
grep -c "writeMetaRow.*schema_version.*'3'" apps-script/src/lib/migrations.ts  # baseline, unchanged
grep "WatcherMaxSchemaVersion" internal/sheet/client.go  # = 3, unchanged
```
</verification>

<success_criteria>
- `getRecentSearches` and `pushRecentSearch` both use `PropertiesService.getUserProperties()`.
- Neither function calls `CacheService.getDocumentCache()` or passes a TTL arg.
- `CacheService.getDocumentCache()` call count in `searchIndex.ts` drops from 3 to exactly 2 (`prewarmSearchCache` + `runSearch` survive).
- `CACHE_TTL_SECONDS` constant remains defined and consumed by per-char cache call sites.
- Existing tests 16-17 (rolling-3, dedupe) pass without body changes.
- New persistence-across-TTL test passes.
- Full apps-script suite green.
- Schema gates unchanged: migrations.ts schema_version='3' write count and client.go WatcherMaxSchemaVersion=3 both untouched.
</success_criteria>

<output>
After completion, create `.planning/phases/08-test-infra-persistence-docs/08-03-SUMMARY.md` per the Phase 5 template. Include in `decisions:` the CACHE_TTL_SECONDS-stays rationale, the D-06 clear-and-replace acknowledgment, and the no-quota-guard rationale (~25,000x headroom).
</output>
