---
phase: 39-faceted-item-search-clicky-haste-scope-toggle
plan: 02
subsystem: web
tags: [sveltekit, svelte5, faceting, scope-toggle, examine-panel, wishlist, vitest, eq-themes]

# Dependency graph
requires:
  - phase: 39-faceted-item-search-clicky-haste-scope-toggle
    plan: 01
    provides: "ItemRollup payload with is_clicky/has_haste (client holdings facet data); GET /api/v1/items/search ?clicky=1/?haste=1 params; the prefix-first order + 2-rune guard contract"
provides:
  - "facetItems(rows, {clicky, haste}) — pure, node-tested, AND-combined client-side holdings facet (web/src/lib/items.ts)"
  - "api.ts facet contract: ItemRollup += is_clicky/has_haste; CatalogItem += optional booleans; searchCatalog(q, facets={}, fetch?) encodes ?clicky=1/?haste=1 (defaulted arg keeps the wishlist caller compiling)"
  - "FacetBar.svelte — two aria-pressed filter chips (NOT the Toggle switch), token-only, 5-theme parity"
  - "Inventory tab: Holdings|Catalog .seg scope toggle (D-03 lens-not-reset) + catalog-row ExaminePanel reuse + holders-by-name (D-04); wishlist add-form facet chips (D-01, catalog-only)"
affects: [faceted-search, web-inventory-tab, web-wishlist-add-form]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Client-side holdings facet: facetItems(filterItems(items, query), {clicky, haste}) — pure predicate (!clicky||r.is_clicky) && (!haste||r.has_haste), zero catalog dependency (SC-4)"
    - "Scope toggle = the .seg/.seg-btn segmented control duplicated from guild-views (component-scoped Svelte styles); setScope is a lens, never touches query/clicky/haste (D-03)"
    - "Unified RowVM so Holdings + Catalog scopes share the list/detail render; catalog rows reuse ExaminePanel unchanged; holders resolved by normalized name from the loaded rollup (D-04)"
    - "Debounced catalog fetch (250ms + sequence guard), 2-rune guard mirrors the server so an empty/short q never fires"

key-files:
  created:
    - web/src/lib/components/FacetBar.svelte
  modified:
    - web/src/lib/items.ts
    - web/src/lib/api.ts
    - web/src/lib/__tests__/items.test.ts
    - web/src/lib/__tests__/banks.test.ts
    - web/src/routes/inventory/+page.svelte
    - web/src/routes/wishlist/+page.svelte

key-decisions:
  - "Facets = two independent aria-pressed filter chips (FacetBar), NOT the Toggle.svelte ON/OFF switch (reserved for async server-write switches) — per UI-SPEC"
  - "Scope toggle is Inventory-tab ONLY; the wishlist add-form gets facet chips but NO scope control (D-01)"
  - "searchCatalog's new facets arg DEFAULTS to {} so the lone existing wishlist caller stays compiling (the TS half of the call-site ripple)"
  - "No new {@html}: all dynamic strings render via {} auto-escape; the only escaped-raw-HTML sink stays inside the reused ExaminePanel (composeItemNote)"

patterns-established:
  - "FacetBar presentational component shared by both facet surfaces (Inventory + wishlist add-form); token-only styling → 5 EQ themes for free"
  - "Active facet state = aria-pressed + fill/label inversion + leading filled mark (three signals, never color-alone)"

requirements-completed: [SEARCH-04, SEARCH-05, SEARCH-06]

# Metrics
duration: ~50min (3 impl tasks; Task 4 = human browser-smoke checkpoint)
completed: 2026-06-25
---

# Phase 39 Plan 02: Faceted item search — web facet/scope UI Summary

**Added the Clicky/Haste facet chips, the Holdings↔Catalog scope toggle, and catalog-row examine to the Inventory tab (plus facet chips to the wishlist add-form), mirroring Plan 01's Go contract — built from existing idioms (FacetBar chips, the .seg segmented control, ExaminePanel reuse), token-only across all 5 EQ themes, no new {@html}, watcher untouched.**

## Performance

- **Duration:** ~50 min (Tasks 1–3 implementation; Task 4 is the blocking human browser-smoke checkpoint)
- **Completed:** 2026-06-25
- **Tasks:** 4 (3 implementation + 1 human-verify checkpoint)
- **Files:** 1 created, 6 modified

## Accomplishments
- **`facetItems()` pure helper (SC-4):** AND-combined predicate `(!clicky||r.is_clicky) && (!haste||r.has_haste)`, node-tested (6 new cases: neither / clicky / haste / both / empty-AND / no-mutation). The holdings facet is a pure client-side filter over the already-loaded rollup — zero catalog-crawl dependency.
- **`api.ts` facet contract:** `ItemRollup` += `is_clicky`/`has_haste`; `CatalogItem` += optional booleans; `searchCatalog(q, facets={}, fetch?)` encodes `?clicky=1`/`?haste=1` (mirrors Plan 01; the defaulted `facets` arg keeps the existing `wishlist:431` `searchCatalog(q)` caller compiling — the TS half of the call-site ripple).
- **`FacetBar.svelte` (NEW):** two `aria-pressed` filter chips (NOT `Toggle.svelte`), leading filled-state mark (never color-alone), token-only (0 literal hex → 5-theme parity).
- **Inventory tab:** Holdings|Catalog `.seg` scope control (duplicated from guild-views) + FacetBar in one control bar. Holdings render = `facetItems(filterItems(items, query), {clicky, haste})` (client-side, SC-4); Catalog render = debounced `searchCatalog(query, {clicky, haste})` (250ms + sequence guard, 2-rune guard mirrors the server). A unified RowVM lets both scopes share the list/detail; catalog rows reuse `ExaminePanel` + holders-by-name; held → holders, unheld → "not held in the guild" (D-04, no pin); `setScope` is lens-not-reset (D-03, never touches query/clicky/haste); sparse-catalog "still filling in" copy (SC-4 graceful degradation).
- **Wishlist add-form:** the same FacetBar chips, catalog-only (D-01), NO scope control; toggling a chip re-runs the debounced add-search.

## Task Commits

1. **Task 1: facetItems() pure helper + widen api.ts facet contract** — `1f3227e` (feat)
2. **Task 2: Inventory scope toggle + facet chips + catalog rows** — `6202409` (feat)
3. **Task 3: Wishlist add-form Clicky/Haste facet chips (catalog-only)** — `a83c2a4` (feat)
4. **Task 4: Browser-smoke checkpoint (human-verify, blocking)** — APPROVED by the user 2026-06-25 on the live deploy (see Verification).

## Files Created/Modified
- `web/src/lib/components/FacetBar.svelte` (NEW) — two prop-driven `aria-pressed` facet chips; token-only; 0 literal hex.
- `web/src/lib/items.ts` — pure `facetItems(rows, {clicky, haste})` AND-combined filter.
- `web/src/lib/api.ts` — `ItemRollup` += `is_clicky`/`has_haste`; `CatalogItem` += optional booleans; `searchCatalog(q, facets={}, fetch?)` facet-param encoding.
- `web/src/lib/__tests__/items.test.ts` — 6 `facetItems` cases.
- `web/src/lib/__tests__/banks.test.ts` — updated for the widened rollup shape.
- `web/src/routes/inventory/+page.svelte` — scope toggle + FacetBar + unified RowVM + catalog-row ExaminePanel/holders/empty-state + D-03 persistence.
- `web/src/routes/wishlist/+page.svelte` — FacetBar chips on the add-form (catalog-only, no scope toggle).

## Decisions Made
None beyond the plan + UI-SPEC. Facet control = chips (not the Toggle switch); scope = `.seg` segmented control, Inventory-only; catalog rows reuse ExaminePanel with no pin action — all per the approved UI-SPEC and D-01..D-04.

## Deviations from Plan

None behavioral. One in-task fix:

1. **[Rule 1 - Bug] Repaired 2 pre-existing NUL bytes** in the inventory holders `{#each}` key template literal (present at HEAD, predating Phase 39 — latent corruption that was blocking the edit). Folded into commit `6202409`. No behavior change.

---
**Total deviations:** 1 (pre-existing-corruption fix). No scope creep.
**Impact on plan:** none — the UI shipped per the UI-SPEC + D-01..D-04.

## Issues Encountered
None of consequence. vitest is node-only / DOM-blind (no `@testing-library/svelte` per the toolchain-install rule), so the DOM render, the debounced catalog fetch, and the D-03 scope-persistence behavior were verified by the human browser-smoke (Task 4), not by vitest — exactly what the blocking checkpoint is for.

## Threat Surface
No new security surface beyond the plan's threat model. T-39-06..10 mitigated: no new `{@html}` sink (inventory 0 / wishlist baseline 1 — Svelte `{}` auto-escape; the only escaped-raw-HTML sink stays inside the reused ExaminePanel `composeItemNote`); the facet params ride the existing session-gated reads (authz unchanged); the client 2-rune guard mirrors the server so no empty-q corpus fetch fires; theme-token-only styling (no injected style). No new endpoint, no new trust boundary.

## Known Stubs
None. The holdings facet filters live rollup data; the catalog scope hits the live `/api/v1/items/search`. Catalog-scope facets over UNHELD items are sparse until the next Sunday UTC wiki crawl populates `catalog_enrichment` — a deploy-coverage note (graceful degradation surfaced as the "catalog still filling in" copy), not a stub; the holdings facet and held-item catalog facets work immediately.

## Verification
- `npm run check` — 0 errors
- `npx vitest run` — 375/375 pass
- `npm run build` — succeeds (FacetBar.*.css present in the bundle)
- No new `{@html}` sinks (inventory 0, wishlist baseline 1); FacetBar 0 literal hex
- Watcher untouched → no `v*` tag
- **Human browser-smoke (Task 4) — APPROVED 2026-06-25 on the live deploy** (https://squirebot.quest, backend 39-01 + web 39-02 deployed together; apex 200, JS `text/javascript`, API 401, schema v17, 0 restarts): Holdings facet filters instantly (no network); scope toggle flips Holdings↔Catalog persisting query+facets (D-03); catalog rows examine via ExaminePanel with holders / "not held in the guild" and no pin action (D-04); wishlist add-form facets narrow suggestions with no scope control (D-01); theme spot-check legible.

## Next Phase Readiness
- Phase 39 delivers SEARCH-04/05/06 end-to-end (backend + web), deployed + smoke-approved. No blockers for Phase 40 (flag outlines + named quest links).
- Watcher untouched; no `v*` tag. NO migration (schema stays v17). The unheld-catalog facet coverage backfills on the next Sunday UTC wiki crawl (holdings + held-item facets already live).

---
*Phase: 39-faceted-item-search-clicky-haste-scope-toggle*
*Completed: 2026-06-25*

## Self-Check: PASSED
- `web/src/lib/components/FacetBar.svelte` exists (new); 6 modified files exist on disk.
- All 3 task commit hashes (`1f3227e`, `6202409`, `a83c2a4`) exist in git history.
- `npm run check` 0 errors, `npx vitest run` 375/375, `npm run build` succeeds.
- No new `{@html}`; theme-token-only; watcher untouched.
- Task 4 browser-smoke checkpoint APPROVED by the user on the live deploy.
