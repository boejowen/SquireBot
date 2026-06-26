---
phase: 39-faceted-item-search-clicky-haste-scope-toggle
reviewed: 2026-06-25T19:10:00Z
depth: deep
files_reviewed: 15
files_reviewed_list:
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/itemsearch.go
  - internal/backendsrv/store/itemsearch_test.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/compute/itemrollup.go
  - internal/backendsrv/compute/itemrollup_test.go
  - internal/backendsrv/readapi/itemsearch.go
  - internal/backendsrv/readapi/itemsearch_test.go
  - web/src/lib/components/FacetBar.svelte
  - web/src/lib/items.ts
  - web/src/lib/api.ts
  - web/src/lib/__tests__/items.test.ts
  - web/src/lib/__tests__/banks.test.ts
  - web/src/routes/inventory/+page.svelte
  - web/src/routes/wishlist/+page.svelte
findings:
  blocker: 1
  high: 0
  medium: 2
  low: 3
  info: 2
  total: 8
status: issues_found
---

# Phase 39: Code Review Report

**Reviewed:** 2026-06-25T19:10:00Z
**Depth:** deep (cross-file: store SQL → readapi handler → compute rollup → web api → Svelte routes)
**Files Reviewed:** 15
**Status:** issues_found

## Summary

Phase 39 adds Clicky/Haste facets and a Holdings↔Catalog scope toggle to item search. The
SQL-injection posture is sound (the `q` stays `?`-bound + `escapeLike` + `ESCAPE`, the facet
bools select a FIXED predicate fragment, the 2-rune guard + `LIMIT 25` are preserved), V7
no-PII logging holds (`q` is never logged; only the facet bools + qlen are added), and no new
`{@html}`/`innerHTML`/`eval` XSS sinks are introduced. SC-4 holds: the holdings facet reads
`is_clicky`/`has_haste` id-correctly from `item_master` through the rollup, independent of
`catalog_enrichment` coverage. The extend-only convention is respected (no new migration; the
flag columns were added in 00016/00017).

There is **one real BLOCKER-class correctness defect**: the catalog facet `LEFT JOIN` fans out
(duplicates) a catalog row whenever its name matches more than one `item_master` row sharing a
normalized name — a common P99 case (spell "Words of…" scrolls, quest turn-ins, multi-id items).
The pattern was copied from `CatalogIconCoverage`, where the union sits inside a `SUM`/`count(*)`
aggregate (duplicates are harmless), but here it is a row-multiplying `LEFT JOIN` with **no
`DISTINCT`/`GROUP BY`**. This both returns duplicate items and feeds Svelte `{#each}` blocks keyed
by `item.item_id` / `it.name`, triggering the `each_key_duplicate` runtime crash the code
elsewhere takes pains to avoid. A reproduction test (two same-name held rows → 2 returned rows for
1 catalog item) is included below. The existing facet tests passed because none seed a same-name
collision.

Per the review brief this phase is already deployed + browser-smoke-approved, so all findings are
**fixed-forward / backlog (advisory)**, not deploy-blocking.

## Blocker Issues

### BL-01: Catalog facet `LEFT JOIN` fans out duplicate rows on same-name held items

**File:** `internal/backendsrv/store/itemsearch.go:66-71, 97-105`

**Issue:**
`flagUnion` is a `LEFT JOIN` to `item_master ∪ catalog_enrichment` keyed on `lower(trim(name))`.
`item_master` is `(item_id INTEGER PRIMARY KEY, name TEXT, …)` — **`name` is NOT unique**. The
`flagUnion` docstring itself acknowledges (mirroring the 38-REVIEW IN-01 note) that "two distinct
held EQ ids that share a normalized name still count once **each**" — i.e. the union emits ≥2 rows
for one normalized name. In `CatalogIconCoverage` (store/itemids.go:193-205) that union lives
inside `count(*)`/`SUM(...)`, so duplicate rows are harmless. Here it is a row-multiplying
`LEFT JOIN` in `SearchCatalog`, and the query has **no `DISTINCT` and no `GROUP BY`**. So when a
facet is active and the matched catalog name corresponds to N held EQ ids with the same name, the
single `pigparse_price` row is emitted N times.

This is a common P99 reality, not an edge case: spell-scroll names ("Words of the Suffering" et
al.), quest turn-ins, and other items exist as many distinct EQ item_ids sharing one display name.

Downstream blast radius:
- **Wishlist add-form** (`web/src/routes/wishlist/+page.svelte:751`): `{#each addResults as item (item.item_id)}` keys by `item_id`. The fan-out duplicates the SAME catalog row (same `item_id`) → **duplicate keys → Svelte `each_key_duplicate` runtime crash** — the exact failure class the holders-table comment (inventory/+page.svelte:542) deliberately disambiguates against.
- **Inventory catalog scope** (`web/src/routes/inventory/+page.svelte`, `rows` keyed `(it.name)`): same-name duplicates → **duplicate name keys → `each_key_duplicate` crash**.
- **`LIMIT 25` undercounts:** the cap counts fanned-out rows, so a facet search can return fewer than 25 DISTINCT items (e.g. 13 unique shown as 25 rows).

Reproduction (fails on current `HEAD`; returns 2 rows for 1 catalog item):
```go
func TestSearchCatalog_DuplicateNameFanout(t *testing.T) {
	db := NewTestDB(t)
	st := NewStore(db)
	seedCatalogItem(t, db, 9000, "Words of the Suffering", true, 1.0) // ONE catalog row
	seedHeldFlag(t, db, 11, "Words of the Suffering", true, false)    // two distinct held
	seedHeldFlag(t, db, 12, "Words of the Suffering", true, false)    // EQ ids, SAME name
	got, _ := st.SearchCatalog(context.Background(), "words", true, false, 25)
	if len(got) != 1 {
		t.Errorf("FAN-OUT: got %d rows for one catalog item, want 1", len(got)) // got 2
	}
}
```

**Fix:** collapse the union to one row per normalized name BEFORE joining (any of the three
matching half-flags is enough — `MAX` over the boolean), or guarantee distinct output. Option A
(pre-aggregate the union, recommended — preserves the LEFT-JOIN-only-when-facet shape):
```go
const flagUnion = `
  LEFT JOIN (
    SELECT norm, MAX(is_clicky) AS is_clicky, MAX(has_haste) AS has_haste
    FROM (
      SELECT lower(trim(name)) AS norm, is_clicky, has_haste FROM item_master
      UNION ALL
      SELECT norm_name          AS norm, is_clicky, has_haste FROM catalog_enrichment
    )
    GROUP BY norm
  ) f ON f.norm = lower(trim(pigparse_price.name))`
```
Option B (defensive belt-and-suspenders, independent of join shape): add
`GROUP BY pigparse_price.item_id` (or `SELECT DISTINCT`) to the outer query so the catalog row
can never appear twice regardless of how the flag source is keyed. Add a `TestSearchCatalog_DuplicateNameFanout`
regression test (the no-test gap is why this shipped).

## Medium Issues

### MD-01: Pending catalog debounce timer survives a scope flip to Holdings

**File:** `web/src/routes/inventory/+page.svelte:225-241`

**Issue:** The catalog-search `$effect` short-circuits with `if (scope !== 'catalog') return;`
BEFORE the `if (catalogTimer) clearTimeout(catalogTimer)` line. If the user types in Catalog
scope and flips to Holdings within the 250 ms debounce window, the already-scheduled
`setTimeout` still fires `runCatalogSearch`, which mutates `catalogRows`/`catalogStatus` while
the user is in Holdings scope. It is benign for correctness today (Holdings ignores
`catalogRows`), but it is a wasted authenticated request and leaves stale results that flash on
the next flip back to Catalog before the new debounce settles.

**Fix:** clear the pending timer and bump the seq when leaving catalog scope, e.g.:
```ts
$effect(() => {
	if (scope !== 'catalog') {
		if (catalogTimer) clearTimeout(catalogTimer);
		++catalogSeq; // cancel any in-flight resolution
		return;
	}
	// …existing body…
});
```

### MD-02: Unheld catalog detail shows a `price` with an empty `prices` array

**File:** `web/src/routes/inventory/+page.svelte:151-176`

**Issue:** For an UNHELD catalog item, `asSlot` is built with `price: selectedRow.price`
(`= c.current_avg`) but `prices: selectedRow.held?.prices ?? []`. The `RowVM.price` for unheld
falls back to `c.current_avg`, so an unheld item can carry a scalar `price` while `prices` is
`[]`. Depending on how `ExaminePanel`/`composeItemNote` reconcile the scalar `price` vs the
`prices` list (one may take precedence, or both may render), this risks an inconsistent
single-price display or a "price present but no price breakdown" note for an item the guild does
not hold. Confirm `ExaminePanel` renders cleanly when `price != null && prices.length === 0`;
if it assumes `prices` is the source of truth, the scalar `price` is silently dropped (or vice
versa).

**Fix:** make the two consistent for the unheld branch — either synthesize a one-element
`prices` from `c.current_avg`, or null the scalar `price` when `prices` is empty, whichever
matches `composeItemNote`'s contract. Add a browser-smoke note for "select an unheld catalog
item with a known catalog price."

## Low Issues

### LO-01: `CatalogItem.is_clicky?` / `has_haste?` are dead interface fields

**File:** `web/src/lib/api.ts:758-759`

**Issue:** The server `store.CatalogItem` struct only marshals `item_id`, `name`, `current_avg`
— it never returns flag fields. No client code reads `CatalogItem.is_clicky`/`has_haste`
(catalog filtering is server-side; `facetItems` runs only over `ItemRollup`). These two optional
fields are never populated on the wire nor consumed. The comment "the UI tolerates undefined"
implies an intended read path that does not exist.

**Fix:** remove the two fields (and the misleading comment), or, if the intent is to let the
client also facet-filter catalog rows client-side later, wire it up — otherwise it is dead
surface that invites a future reader to assume the flags arrive.

### LO-02: Catalog `noResults` empty-state hidden during the search-pending window

**File:** `web/src/routes/inventory/+page.svelte:243-249`

**Issue:** `noResults` for catalog scope keys off `catalogStatus === 'ready'`. While a search is
`'searching'` (debounce + in-flight), `noResults` is false and `rows` is empty, so the list
region renders neither the results nor the no-results block nor any "searching…" affordance —
the area is blank for up to ~250 ms + network latency. Minor UX gap (no spinner/skeleton for the
catalog search), not a correctness bug.

**Fix:** render a lightweight "Searching…" state when `scope === 'catalog' && catalogStatus === 'searching'`,
mirroring the wishlist add-form's `addSearching` affordance.

### LO-03: `runCatalogSearch` shadows the `clicky`/`haste` reactive `$state` with params

**File:** `web/src/routes/inventory/+page.svelte:200-219`

**Issue:** `runCatalogSearch(query, clicky, haste)` names its params identically to the
module-level `$state` `query`/`clicky`/`haste`. It is intentional (the comment documents the
captured-at-fire-time values + the seq-guard), and correct, but the deliberate shadowing of
reactive state inside an async function is a foot-gun for the next maintainer (a stray reference
to the outer reactive value vs. the captured param would not be flagged by the compiler). The
wishlist `runAddSearch(q)` uses the distinct name `q` for exactly this reason.

**Fix:** rename the params (`qParam`/`clickyParam`/`hasteParam` or `q`/`c`/`h` to match the
effect's local captures) so the captured-snapshot vs. live-reactive distinction is lexically
obvious.

## Info

### IN-01: Facet predicate uses `COALESCE(...,0)=1` where the join NULL is impossible to false-positive but the redundancy is worth a note

**File:** `internal/backendsrv/store/itemsearch.go:90-95`

**Issue:** `AND COALESCE(f.is_clicky,0) = 1` correctly treats a missing/NULL flag as not-set.
This is right and defensive (a LEFT JOIN miss → NULL → 0 → excluded). Purely informational: once
BL-01's pre-aggregation is in place, the `COALESCE` still belongs (the `GROUP BY`'d `MAX` over an
all-NULL group is still NULL). No action beyond keeping it through the BL-01 fix.

### IN-02: Facet tests never exercise the same-name collision that BL-01 exposes

**File:** `internal/backendsrv/store/itemsearch_test.go:239-357`

**Issue:** `TestSearchCatalog_ClickyOnly/HasteOnly/BothFacets/CatalogOnlyFlag/NoFacetRegression`
each seed at most one flag row per name, so the union never emits >1 row per norm and the
fan-out stays invisible. The `NoFacetRegression` guard correctly proves the no-facet path is
unchanged but does not cover the facet-active multiplicity. Add the
`TestSearchCatalog_DuplicateNameFanout` case (BL-01) as the regression lock.

**Fix:** ship the reproduction test from BL-01 alongside the fan-out fix.

---

_Reviewed: 2026-06-25T19:10:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
