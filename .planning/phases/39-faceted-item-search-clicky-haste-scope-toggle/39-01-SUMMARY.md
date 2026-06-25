---
phase: 39-faceted-item-search-clicky-haste-scope-toggle
plan: 01
subsystem: api
tags: [go, sqlite, search, faceting, item-master, catalog-enrichment, name-keyed-join, slog]

# Dependency graph
requires:
  - phase: 37-item-enrichment-backbone-flags-effects
    provides: "item_master.is_clicky / has_haste columns (migration 00016), backfilled live by the weekly wiki job"
  - phase: 38-catalog-wide-enrichment-icon-coverage
    provides: "catalog_enrichment table (norm_name PK, migration 00017) carrying is_clicky/has_haste for catalog-only items; the CatalogIconCoverage name-keyed UNION-ALL precedent (store/itemids.go)"
provides:
  - "ItemRollup payload (GET /api/v1/items) now carries is_clicky / has_haste booleans sourced from item_master — the client-side holdings facet data with NO catalog dependency (SC-4)"
  - "store.SearchCatalog 5-arg signature (ctx, q, clicky, haste, limit) with the name-keyed item_master ∪ catalog_enrichment flag union (LEFT JOIN added ONLY when a facet is active)"
  - "GET /api/v1/items/search parses ?clicky=1 / ?haste=1 into bools, preserving the 2-rune guard, escapeLike+ESCAPE, LIMIT 25, and V7 no-q logging"
  - "The Go↔TS contract Plan 02 mirrors: two snake_case JSON tags (is_clicky/has_haste) + the ?clicky=1/?haste=1 query-param encoding"
affects: [39-02, web-inventory-tab, web-wishlist-add-form, faceted-search]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Name-keyed flag union: read facet flags from item_master (held, EQ-id) ∪ catalog_enrichment (catalog-only, norm_name), joined to pigparse_price by lower(trim(name)) — NEVER item_id (PigParse vs EQ namespace)"
    - "Conditional LEFT JOIN: the facet join is appended to the query ONLY when a facet is active, so the no-facet path stays byte-identical to the original single-table search (Pitfall 3 — no unconditional/INNER join that drops unenriched rows)"
    - "Bool facet params select a FIXED predicate fragment — no user string reaches SQL (q stays ?-bound + escapeLike + ESCAPE)"

key-files:
  created: []
  modified:
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/store/itemsearch.go
    - internal/backendsrv/store/itemsearch_test.go
    - internal/backendsrv/compute/types.go
    - internal/backendsrv/compute/itemrollup.go
    - internal/backendsrv/compute/itemrollup_test.go
    - internal/backendsrv/readapi/itemsearch.go
    - internal/backendsrv/readapi/itemsearch_test.go

key-decisions:
  - "Holdings facet = client-side (booleans on ItemRollup from item_master); catalog facet = server-side (SearchCatalog params) — the SC-4-safe split"
  - "Open Q1 RESOLVED: catalog scope keeps SearchCatalog's prefix-first ORDER BY (NOT re-sorted viewer-first)"
  - "Open Q2 RESOLVED: the 2-rune guard is preserved — empty/short q returns [] even with a facet active (no 'browse all clickies' corpus dump)"
  - "?clicky=1 / ?haste=1 '1'-encoding (Assumption A1) is the contract Plan 02's api.ts mirrors"

patterns-established:
  - "Name-keyed facet flag union copied from CatalogIconCoverage (swap icon_id → is_clicky/has_haste); UNION ALL is safe (a held name is never in catalog_enrichment — write-path dedup), no precedence logic"
  - "NullInt64 → bool idiom for the nullable 00016/00017 flag columns (NULL/0 → false)"

requirements-completed: [SEARCH-04, SEARCH-05, SEARCH-06]

# Metrics
duration: ~30min
completed: 2026-06-25
---

# Phase 39 Plan 01: Faceted item search — backend facet plumbing Summary

**Widened the holdings item payload with is_clicky/has_haste from item_master and gave store.SearchCatalog Clicky/Haste facet params via a name-keyed item_master ∪ catalog_enrichment flag union — the Go↔TS contract Plan 02's UI consumes (NO new migration, watcher untouched).**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-06-25T22:45Z (approx)
- **Completed:** 2026-06-25T23:16Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- **Holdings facet data (SC-4-safe):** `IconStats` + `ItemRollup` now carry `is_clicky`/`has_haste` sourced from `item_master` (00016) via the widened `ItemMasterIconStats` SELECT — so the web can facet holdings client-side with zero catalog-enrichment dependency. The two booleans flow `readviews.go → itemrollup.go → types.go` to the JSON payload, and the IRON LAW (no SQL in `compute`) is preserved.
- **Catalog facet (server-side):** `SearchCatalog` is now 5-arg `(ctx, q, clicky, haste, limit)`. When a facet is active it joins each `pigparse_price` row to its flags via the name-keyed `item_master ∪ catalog_enrichment` UNION-ALL (`lower(trim(name))`, NEVER `item_id`) and AND-filters; with NO facet the query is byte-identical to today's single-table search (the regression guard, `TestSearchCatalog_NoFacetRegression`).
- **Handler params:** `GET /api/v1/items/search` parses `?clicky=1`/`?haste=1` into bools, passes them to `SearchCatalog`, preserving the 2-rune guard, `escapeLike`+ESCAPE, `LIMIT 25`, and V7 logging (now logging `clicky`/`haste` booleans but still NEVER the `q` string).
- **Test coverage:** clicky-only / haste-only / both / held-flag / catalog-only-flag / no-facet-regression at the store level; clicky-param + guard-with-facet at the handler level; held-flag propagation + NULL-row-false at the rollup level. Full backend suite green (0 FAIL, 17 packages).

## Task Commits

Each task was committed atomically (normal commits, with hooks):

1. **Task 1: Widen IconStats + ItemRollup with is_clicky/has_haste (holdings facet)** - `a3b815d` (feat)
2. **Task 2: Add clicky/haste facets to SearchCatalog via the name-keyed flag union** - `2d36059` (feat)
3. **Task 3: Parse ?clicky=/?haste= on /api/v1/items/search, preserve guard + V7** - `ecd7253` (feat)

_Note: each TDD task here combined test + impl in one commit because the new tests reference new struct fields / a new signature and would not compile against the pre-change code — the standard pattern for an extend-in-place TDD task where a pure RED would be a non-compiling test._

## Files Created/Modified
- `internal/backendsrv/store/readviews.go` - `IconStats` gains `IsClicky`/`HasHaste`; `ItemMasterIconStats` SELECT widened to `is_clicky, has_haste` with the NullInt64→bool scan.
- `internal/backendsrv/store/itemsearch.go` - `SearchCatalog` 5-arg signature; the `flagUnion` name-keyed LEFT JOIN (facet-only); the optional COALESCE-fragment predicate; columns qualified `pigparse_price.*`.
- `internal/backendsrv/store/itemsearch_test.go` - 8 call sites → 5-arg `false,false`; `seedHeldFlag`/`seedCatalogEnrichmentFlag` helpers; 5 new facet cases.
- `internal/backendsrv/compute/types.go` - `ItemRollup` gains `is_clicky`/`has_haste` JSON fields (append-only, after `statsblock`).
- `internal/backendsrv/compute/itemrollup.go` - copies the two booleans onto the rollup literal (no SQL).
- `internal/backendsrv/compute/itemrollup_test.go` - `setItemFlags` helper; asserts the booleans propagate (Jade Reaver clicky=1/haste=0) and a NULL-flag row → false/false.
- `internal/backendsrv/readapi/itemsearch.go` - parses `?clicky=1`/`?haste=1`; passes to `SearchCatalog`; V7 slog adds the booleans, never `q`.
- `internal/backendsrv/readapi/itemsearch_test.go` - `seedHeldFlag` helper + `containsID`; `ClickyParam` + `GuardWithFacet` cases.

## The Go↔TS contract for Plan 02

Plan 02's `web/src/lib/api.ts` MUST mirror these exactly:

- **`ItemRollup`** (holdings payload, GET `/api/v1/items`): append `is_clicky: boolean` + `has_haste: boolean` (snake_case tags, always present).
- **`CatalogItem`** is unchanged on the wire by this plan (the catalog facet is a server-side filter, not new response fields); Plan 02 may add optional `is_clicky?`/`has_haste?` if it surfaces them.
- **`searchCatalog`**: encode `?clicky=1` / `?haste=1` (the literal `"1"` string — Assumption A1) when the facet is on; omit the param when off. Absent/any-other value is treated as `false` server-side.
- **Resolved defaults:** catalog scope is prefix-first ordered (Open Q1); a query under 2 runes returns `[]` even with a facet active (Open Q2 — no empty-q corpus dump).

## Decisions Made
None beyond the plan — both Open Questions were pre-resolved in the plan's `must_haves` (Open Q1: keep prefix-first ordering; Open Q2: keep the 2-rune guard) and were implemented as specified.

## Deviations from Plan

None — plan executed essentially as written. Two minor in-task adjustments (not behavioral deviations, no scope change):

1. **[Rule 3 - Blocking] Removed a duplicate `boolToInt` helper.** While adding the `seedCatalogEnrichmentFlag` store-test helper I declared a local `boolToInt`, which collided with the package-existing `store/sqliteconstraint.go:22` `boolToInt`. Removed my duplicate and reused the existing identical-signature helper. Found during Task 2; fixed in the same `2d36059` commit. Verified by `go test ./store/...` compiling + passing.
2. **[Rule 1 - Bug, doc-only] Reworded a code comment to avoid the literal token `INNER JOIN`.** The acceptance criterion grep-gates that `INNER JOIN` appears 0 times in `itemsearch.go` (the Pitfall-3 / T-39-02 anti-pattern gate); my explanatory comment originally named the anti-pattern ("never INNER JOIN…"), which tripped the grep. Reworded to "the join is LEFT and conditional, never an unconditional/always-on join" — same meaning, gate now returns 0. Found during Task 2 acceptance check; fixed in the same `2d36059` commit.

---

**Total deviations:** 2 (1 blocking-fix, 1 doc-only) — both inside their task commits, no behavior change, no scope creep.
**Impact on plan:** none — the SQL/payload/handler contract shipped exactly as specified.

## Issues Encountered
None of consequence. The store-package `boolToInt` collision (above) was the only build hiccup; caught immediately by the test compile and resolved in the same task.

## Threat Surface
No new security surface beyond the plan's threat model. T-39-01..05 all mitigated as planned: the facet params are Go `bool` selecting a fixed predicate fragment (no string reaches SQL); the flag join is name-keyed only (grep-gated, T-39-02); V7 logging carries `qlen`/`clicky`/`haste`, never `q` (T-39-03); the 2-rune guard + LIMIT 25 cap the scan (T-39-04); both reads stay `RequireSession`-gated (T-39-05). No new endpoint, no schema change, no new trust boundary → no Threat Flags.

## Known Stubs
None. Every changed code path is wired to real data: holdings booleans read live `item_master` columns (backfilled in prod); the catalog facet reads the live `item_master ∪ catalog_enrichment` union. (Catalog-scope facets over UNHELD items are sparse until the next Sunday UTC wiki crawl populates `catalog_enrichment` — a deploy-coverage note from RESEARCH, not a stub; the holdings facet and held-item catalog facets work immediately.)

## Verification
- `go build ./...` — GREEN
- `go vet ./...` — GREEN
- `go test ./... -count=1` — GREEN (0 FAIL across 17 backend packages)
- No new migration file under `internal/backendsrv/migrations/` (dir still ends at `00017_catalog_enrichment.sql`)
- Watcher untouched: `git status --porcelain internal/watcher/ cmd/squirebot-watcher/` empty → no `v*` tag warranted

## Next Phase Readiness
- The Go↔TS contract (two `ItemRollup` booleans + the `?clicky=1`/`?haste=1` encoding) is stable and tested — **Plan 02 (web facet/scope UI) can target it directly.**
- No blockers. Watcher untouched; no `v*` tag. NO new migration → the backend binary can ship without a schema bump (00016/00017 already deployed; `catalog_enrichment` unheld coverage fills on the next Sunday crawl, holdings facet works immediately).

---
*Phase: 39-faceted-item-search-clicky-haste-scope-toggle*
*Completed: 2026-06-25*

## Self-Check: PASSED
- All 8 modified source files exist on disk.
- All 3 task commit hashes (`a3b815d`, `2d36059`, `ecd7253`) exist in git history.
- `go build ./...` + `go vet ./...` + `go test ./...` GREEN (0 FAIL, 17 packages).
- No new migration (dir ends at 00017); watcher untouched.
