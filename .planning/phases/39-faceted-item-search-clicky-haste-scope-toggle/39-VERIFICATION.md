---
phase: 39-faceted-item-search-clicky-haste-scope-toggle
verified: 2026-06-25T19:05:00Z
status: passed
score: 10/10 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification_satisfied:
  - test: "Holdings facet filters instantly (no network); Clicky/Haste AND-combine; scope toggle persists query+facets (D-03); catalog rows examine via ExaminePanel with holders / 'not held in the guild' and no pin (D-04); wishlist add-form facets with no scope control (D-01); theme spot-check legible."
    result: "APPROVED by user 2026-06-25 on live deploy (https://squirebot.quest — apex 200, JS text/javascript, API 401, schema v17, 0 restarts)"
    recorded_in: "39-02-SUMMARY.md Verification section, Task 4 blocking checkpoint"
---

# Phase 39: Faceted item search (Clicky / Haste + scope toggle) Verification Report

**Phase Goal:** A guildie can narrow item search to just Clicky or Haste items, and flip the search scope between guild holdings ("who has one") and the full P99 catalog ("what exists") — turning the flat name/id search into a discovery tool.
**Verified:** 2026-06-25T19:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + PLAN must_haves)

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| SC-1 | Clicky filter shows only Clicky items; result set excludes non-clicky | ✓ VERIFIED | Holdings: `facetItems` predicate `(!f.clicky \|\| r.is_clicky)` (web/src/lib/items.ts:49). Catalog: `AND COALESCE(f.is_clicky,0) = 1` (store/itemsearch.go:91). Tests: `TestSearchCatalog_ClickyOnly` (store), `TestItemSearch_ClickyParam` (handler), 6 facetItems node cases — all green. |
| SC-2 | Haste filter shows only Haste items; result set excludes non-haste | ✓ VERIFIED | Holdings: `(!f.haste \|\| r.has_haste)` (items.ts:49). Catalog: `AND COALESCE(f.has_haste,0) = 1` (itemsearch.go:94). Tests: `TestSearchCatalog_HasteOnly`, `TestSearchCatalog_BothFacets` (intersection) — green. |
| SC-3 | Holdings↔Catalog scope toggle; holdings answers from held items, catalog from full enriched catalog | ✓ VERIFIED | `.seg` scope control + `setScope` (inventory/+page.svelte:261,356,363); Holdings = `facetItems(filterItems(items, query),…)` client filter (:106); Catalog = `searchCatalog(query,{clicky,haste})` server fetch (:207). |
| SC-4 | Holdings faceting works even if full-catalog scope is still filling in (holdings does not block on catalog crawl) | ✓ VERIFIED | Holdings booleans sourced from `ItemMasterIconStats` → `item_master` ONLY (itemrollup.go:47, readviews.go:774); zero `catalog_enrichment` reference in the holdings path. No-SQL-in-compute IRON LAW holds (grep `QueryContext\|s.db.Query` in itemrollup.go = 0). |
| P39-T1 | Holdings payload (GET /api/v1/items) carries is_clicky/has_haste from item_master, no catalog dependency | ✓ VERIFIED | `IconStats.IsClicky/HasHaste` (readviews.go:762-763) ← `SELECT … is_clicky, has_haste FROM item_master` (:774); flows to `ItemRollup` JSON tags (types.go:237-238) via itemrollup.go:82-83. |
| P39-T2 | SearchCatalog accepts clicky/haste; name-keyed union join; AND-combined; no-facet path byte-identical | ✓ VERIFIED | 5-arg sig (itemsearch.go:85); `flagUnion` LEFT JOIN added ONLY when facet active (:99-101); `TestSearchCatalog_NoFacetRegression` green. |
| P39-T3 | Handler parses ?clicky=1/?haste=1; preserves 2-rune guard, escapeLike+ESCAPE, LIMIT 25, V7 no-q logging | ✓ VERIFIED | Parse (itemsearch.go:60-61); guard at :52 fires before call at :62; `searchLimit=25` (:27); slog logs clicky/haste not q (grep `"q", q` = 0). |
| P39-T4 | Catalog scope keeps prefix-first ordering (Open Q1) | ✓ VERIFIED | `ORDER BY (… LIKE ?) DESC, length(name), name COLLATE NOCASE` (itemsearch.go:104). |
| P39-T5 | 2-rune guard preserved — short/empty q returns [] even with facet (Open Q2) | ✓ VERIFIED | `utf8.RuneCountInString(q) < 2` (itemsearch.go:52) short-circuits before SearchCatalog; web mirrors it (inventory debounce comment :195). `TestItemSearch_GuardWithFacet` green. |
| P39-W | facetItems pure/node-tested; FacetBar chips; D-01 wishlist facets; D-04 catalog-row reuse | ✓ VERIFIED | `facetItems` pure new-array filter (items.ts:45-50, 18 node tests pass); FacetBar 5 aria-pressed / 0 hex; wishlist `searchCatalog(q,{clicky:addClicky,haste:addHaste})` (:441) no scope control; catalog rows reuse ExaminePanel + holders-by-name (:111,521,435). |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/store/itemsearch.go` | 5-arg SearchCatalog + name-keyed flag union (facet-only) | ✓ VERIFIED | Sig at :85; `flagUnion` name-keyed (`f.norm = lower(trim(pigparse_price.name))`); INNER JOIN count 0; item_id facet-key count 0. |
| `internal/backendsrv/store/readviews.go` | IconStats widened + widened SELECT | ✓ VERIFIED | `IsClicky/HasHaste` :762-763; SELECT :774 with NullInt64→bool scan :795-796. |
| `internal/backendsrv/compute/types.go` | ItemRollup is_clicky/has_haste JSON fields | ✓ VERIFIED | :237-238 snake_case tags, append-only after statsblock. |
| `internal/backendsrv/compute/itemrollup.go` | Two booleans on rollup literal, no SQL | ✓ VERIFIED | `ic.IsClicky/ic.HasHaste` :82-83; no QueryContext/s.db.Query. |
| `internal/backendsrv/readapi/itemsearch.go` | ?clicky=/?haste= parse + facet-aware V7 slog | ✓ VERIFIED | :60-62 parse+call; :74 slog booleans, no q. |
| `web/src/lib/items.ts` | pure facetItems(rows,{clicky,haste}) AND-combined new array | ✓ VERIFIED | :45-50; node-tested. |
| `web/src/lib/api.ts` | ItemRollup += booleans; CatalogItem += optional; searchCatalog faceted | ✓ VERIFIED | :250-251 required; :758-759 optional; :766-773 `?clicky=1/?haste=1` encoding. |
| `web/src/lib/components/FacetBar.svelte` | prop-driven aria-pressed chips, token-only | ✓ VERIFIED | 5 aria-pressed, 0 literal hex (5-theme parity). |
| `web/src/routes/inventory/+page.svelte` | scope control + facet chips + catalog read + examine/holders | ✓ VERIFIED | setScope :261; facetItems :106; searchCatalog :207; ExaminePanel :521; "not held in the guild" :435. |
| `web/src/routes/wishlist/+page.svelte` | add-form facets, no scope toggle | ✓ VERIFIED | FacetBar :736; searchCatalog faceted :441; class="seg"/setScope count 0. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| itemrollup.go | store.IconStats | `ic.IsClicky = …` | ✓ WIRED | itemrollup.go:82-83 reads widened map from ItemMasterIconStats. |
| itemsearch.go | item_master ∪ catalog_enrichment | name-keyed UNION-ALL LEFT JOIN | ✓ WIRED | itemsearch.go:66-71 `lower(trim(name))` / norm_name; never item_id. |
| readapi/itemsearch.go | store.SearchCatalog | 5-arg call w/ parsed bools | ✓ WIRED | :62 `SearchCatalog(r.Context(), q, clicky, haste, searchLimit)`. |
| inventory/+page.svelte | items.ts facetItems | `$derived(facetItems(filterItems(items,query),{clicky,haste}))` | ✓ WIRED | :106. |
| inventory/+page.svelte | api.ts searchCatalog | debounced catalog fetch w/ facets | ✓ WIRED | :207 inside `runCatalogSearch` (250ms debounce + seq-guard). |
| wishlist/+page.svelte | api.ts searchCatalog | `searchCatalog(q,{clicky:addClicky,haste:addHaste})` | ✓ WIRED | :441. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| inventory holdings render | `shown` / `items` | GET /api/v1/items → compute.Items → ItemMasterIconStats (item_master, live-backfilled 00016) | Yes | ✓ FLOWING |
| inventory catalog render | `catalogRows` | searchCatalog → /api/v1/items/search → SearchCatalog over live pigparse_price ∪ item_master ∪ catalog_enrichment | Yes (held + held-item catalog facets immediate; unheld-catalog facet coverage backfills on next Sunday wiki crawl — documented graceful degradation, not a stub) | ✓ FLOWING |
| catalog-row holders | `holdingsByName` | already-loaded `items` rollup keyed by normalized name (D-04) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Backend facet suite | `go test ./store ./compute ./readapi -run 'SearchCatalog\|GroupsByNameWithHoldersAndFlags\|ItemSearch\|IconStats'` | ok (3 packages, all green) | ✓ PASS |
| Store facet cases exist | grep TestSearchCatalog_* | ClickyOnly/HasteOnly/BothFacets/CatalogOnlyFlag/NoFacetRegression present | ✓ PASS |
| Web facetItems node tests | `npx vitest run src/lib/__tests__/items.test.ts` | 18 passed | ✓ PASS |
| No INNER JOIN / no item_id facet key | grep itemsearch.go | INNER JOIN=0, item_id facet key=0, name-keyed join=1 | ✓ PASS |
| No new migration | ls migrations/ | ends at 00017_catalog_enrichment.sql | ✓ PASS |
| Watcher untouched | `git status --porcelain internal/watcher/ cmd/squirebot-watcher/` | empty | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| SEARCH-04 | 39-01, 39-02 | Filter item search to only Clicky items | ✓ SATISFIED | SC-1 truth (holdings facetItems + catalog COALESCE predicate); tests green; REQUIREMENTS.md:24 / map line 60 → Phase 39. |
| SEARCH-05 | 39-01, 39-02 | Filter item search to only Haste items | ✓ SATISFIED | SC-2 truth (has_haste predicate both scopes); tests green; REQUIREMENTS.md:25 / map line 61 → Phase 39. |
| SEARCH-06 | 39-01, 39-02 | Toggle scope between guild holdings and full P99 catalog | ✓ SATISFIED | SC-3 truth (.seg toggle, holdings client-filter, catalog searchCatalog); D-03 lens-not-reset honored; REQUIREMENTS.md:26 / map line 62 → Phase 39. |

No orphaned requirements: REQUIREMENTS.md maps only SEARCH-04/05/06 to Phase 39 (lines 60-62); all three are claimed in both plans' `requirements:` frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | No TODO/FIXME/PLACEHOLDER/stub in any phase-39 changed file. FacetBar 0 literal hex. No new `{@html}` sink (inventory 0; wishlist baseline 1, unchanged). |

### Human Verification Required

None outstanding. The single blocking human-verify checkpoint (Plan 39-02 Task 4 — DOM render, debounced catalog fetch, D-03 scope persistence, D-04 catalog-row content, D-01 wishlist facets, theme legibility) was **APPROVED by the user on 2026-06-25** on the live deploy (https://squirebot.quest — apex 200, JS `text/javascript`, API 401, schema v17, 0 restarts). Recorded in 39-02-SUMMARY.md. node vitest is DOM-blind, so this browser smoke is the canonical evidence for the render/interaction behaviors — it is satisfied, not pending.

### Gaps Summary

No gaps. All four ROADMAP success criteria are made TRUE by the committed code:
- **SC-1/SC-2 (Clicky/Haste filtering)** — verified at every layer: the pure `facetItems` predicate (holdings, client-side), the `COALESCE(f.is_clicky/has_haste,0)=1` SQL fragments (catalog, server-side), and the `?clicky=1/?haste=1` handler parse, all with passing store/handler/node tests covering exclusion and the AND-combination intersection.
- **SC-3 (scope toggle)** — the `.seg` Holdings↔Catalog control with `setScope`; Holdings answers from the already-loaded rollup, Catalog from `searchCatalog` over the full catalog; query + facets persist across flips (D-03 lens-not-reset confirmed: setScope assigns nothing but `scope`).
- **SC-4 (graceful degradation)** — the load-bearing finding: the holdings facet booleans come from `ItemMasterIconStats → item_master` with zero `catalog_enrichment` dependency (the no-SQL-in-compute law guarantees the holdings path can't reach the catalog tables), so holdings faceting works on day one regardless of catalog-crawl coverage.

Decisions honored: D-01 (wishlist facets, catalog-only, no scope toggle), D-02 (two independent AND-combined facets), D-03 (lens-not-reset), D-04 (catalog row reuses ExaminePanel + holders-by-name, no pin action). The catalog↔flags join is name-keyed (`lower(trim(name))`) end-to-end — never raw item_id — exactly as the namespace-bridge hazard requires. No new migration (dir ends at 00017), watcher untouched (clean git status, no `v*` tag). Backend `go test` and web vitest green; deployed live and human browser-smoke approved.

---

_Verified: 2026-06-25T19:05:00Z_
_Verifier: Claude (gsd-verifier)_
