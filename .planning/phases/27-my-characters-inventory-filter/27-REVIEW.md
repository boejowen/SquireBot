---
phase: 27-my-characters-inventory-filter
reviewed: 2026-06-08T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - web/src/lib/myview.ts
  - web/src/lib/__tests__/myview.test.ts
  - web/src/routes/+page.svelte
findings:
  critical: 0
  warning: 0
  info: 4
  total: 4
status: issues_found
---

# Phase 27: Code Review Report

**Reviewed:** 2026-06-08
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found (no blockers/highs — 4 low/nit items)

## Summary

Phase 27 adds a purely client-side "My characters" quick-filter + single-character
drill-down over the four consolidated views via one EQ-themed `<select>` and a pure,
node-tested predicate (`web/src/lib/myview.ts`). The implementation is correct on its
load-bearing axes:

- **Filter predicate** is sound. Drill-down dominates; passthrough returns the SAME
  reference (verified by `expect(out).toBe(rows)` in the test); mine-only uses the set;
  empty-mine correctly yields `[]` (empty set → `.has()` always false). The
  case-insensitive lowercased join matches between `myCharNameSet` and `applyMyFilter`.
- **Svelte 5 runes** are correct. `mineNames`/`filtered*Rows`/`filterActive`/`hasMine`/
  `filterValue` are all `$derived` over `$state` sources (`myCharacters`, `mineOnly`,
  `selectedChar`, the row arrays), so they recompute when any input changes. The
  `Promise.all` 7th element (`mc`) is destructured into the 7th slot and assigned to
  `myCharacters` — ordering is correct. No stale closures.
- **Negative security property holds.** No `?mine=1` / server scoping added; the four
  grids are fed pre-filtered arrays while the API still returns all-members rows; names
  render via plain `{}` (no `{@html}`); drill-down options come ONLY from
  `fetchMyCharacters()` (session-scoped). Verified `grep` of the file.
- **Consolidated-views LOCK preserved.** `git status` confirms `DataGrid.svelte` and
  `columns.ts` are unmodified; the SearchBox still reads the full `viewRows`
  (`rows={viewRows}`), not the filtered arrays.

No blockers or warnings. Four low/nit items below concern stale-state UX, a minor
view-consistency gap, and two test-coverage gaps. The DOM-rendering browser-smoke gap is
already flagged in the plan/SUMMARY and is intentionally NOT re-raised here.

## Info

### IN-01: `selectedChar` can become a stale, unrecoverable drill-down value after a refetch

**File:** `web/src/routes/+page.svelte:105,176-179,191`
**Severity:** LOW
**Issue:** `selectedChar` is independent `$state` that is never reconciled against
`myCharacters` after a reload. If a member drills into character `X`, then `refetch()`
runs (the Retry handler, or any future re-load) and `X` is no longer in `myCharacters`
(unassigned in the interim, or removed by an officer), `selectedChar` still holds `'X'`.
Because `applyMyFilter` drill-down dominates, all four grids filter to a character that
no longer has any "mine" backing, and every view shows the `filter-empty` copy ("None of
your characters have rows in this view"). Meanwhile `filterValue = selectedChar ?? ...`
resolves to `'X'`, which is no longer one of the `<select>`'s options, so the browser
shows an out-of-list/blank selection — the member cannot obviously select "All members"
to recover (the control no longer reflects a real state).
**Why it matters:** A silently stuck filter reads as "my data vanished." Low likelihood
today (refetch only fires on explicit Retry), but it is a latent correctness/UX trap the
moment any auto-refresh is added.
**Fix:** After assigning `myCharacters = mc;` in `load()`, reconcile the drill-down:
```ts
myCharacters = mc;
// drop a drill-down that no longer maps to a claimed character
if (selectedChar && !mc.some((c) => c.name.toLowerCase() === selectedChar!.toLowerCase())) {
    selectedChar = null;
}
```
(Or guard `filterValue` to fall back to `'all'`/`'mine'` when `selectedChar` is not in
`myCharacters`.)

### IN-02: Bank coin summary is NOT narrowed by the drill-down, unlike the grid below it

**File:** `web/src/routes/+page.svelte:164,308-333`
**Severity:** LOW
**Issue:** When the filter is active (mine-only or a single-char drill-down), the bank
`DataGrid` is fed `filteredBankRows`, but the coin summary directly above it
(`coinToons = $derived(bankToons.filter(hasRecordedCoin))`) still lists EVERY toon's
recorded coin. Drilling into one character shows that character's bank rows but every
member's coin totals — an inconsistent mixed scope within one view.
**Why it matters:** Not a security issue (coin is already all-members-visible and the
filter is explicitly not access control), but it's a UX inconsistency a reviewer/user
will notice: "I filtered to my char, why do I still see everyone's plat?" The plan only
specced narrowing the four grid arrays, so this is arguably out of the executor's
mandate — flagging for a product decision rather than as a defect.
**Fix:** Either (a) intentionally leave coin guild-wide and add a one-line comment near
`coinToons` stating coin is deliberately not narrowed (bank coin is a guild-bank
aggregate), or (b) narrow it when `filterActive`:
```ts
let visibleCoinToons = $derived(
    filterActive ? coinToons.filter((t) => mineNames.has(t.name.toLowerCase()) ||
        (selectedChar !== null && t.name.toLowerCase() === selectedChar.toLowerCase()))
        : coinToons
);
```
Option (a) is likely correct for a shared bank; pick one and document it.

### IN-03: `myCharNameSet` case-collapse and empty-string `selectedChar` are untested

**File:** `web/src/lib/__tests__/myview.test.ts:28-40,57-64`
**Severity:** NIT
**Issue:** Two predicate edges are unasserted:
1. `myCharNameSet` lowercases names, so `['Slampeach','SLAMPEACH']` collapses to a
   size-1 set — the case-insensitive contract is exercised on the `applyMyFilter` row
   side (line 76) but not on the set-construction side.
2. `applyMyFilter`'s `if (selectedChar)` treats `''` (empty string) as falsy and falls
   through to the mine-only branch. That is the intended behavior (empty string is not a
   real drill-down), but it is undocumented and untested, so a future refactor to
   `selectedChar !== null` would silently change it.
**Why it matters:** These are the exact "weak/missing cases" the review focus asks about.
Both are correctness-adjacent invariants worth pinning.
**Fix:** Add two assertions:
```ts
it('myCharNameSet collapses case', () => {
    expect(myCharNameSet([mine({ name: 'Slampeach' }), mine({ name: 'SLAMPEACH' })]).size).toBe(1);
});
it('empty-string selectedChar is not a drill-down (falls through to mineOnly)', () => {
    expect(applyMyFilter(rows, names, true, '').map((r) => r.char)).toEqual(['Slampeach', 'Findom']);
});
```

### IN-04: Duplicate per-character `<option>` values are not deduplicated

**File:** `web/src/routes/+page.svelte:247-249`
**Severity:** NIT
**Issue:** `{#each myCharacters as c (c.character_id)}` keys by `character_id` (good — no
Svelte key collision), but the emitted `<option value={c.name}>` uses the NAME. If a
caller ever holds two characters with the same name, two options share a value and the
drill-down narrows to both (since `applyMyFilter` matches by name). In practice a single
caller having two identically-named characters is implausible (names are per-character
proper nouns), so this is theoretical — but the join key (name) and the iteration key
(id) diverging is worth a one-line acknowledgement.
**Why it matters:** Defensive note only; no current data path produces this.
**Fix:** None required. If desired, dedupe option names, or add a comment that the
filter joins by name and assumes a caller's character names are unique.

---

_Reviewed: 2026-06-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
