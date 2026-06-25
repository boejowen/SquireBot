---
phase: 38-catalog-wide-enrichment-icon-coverage
plan: 01
subsystem: api
tags: [go, sqlite, enrichment, mediawiki, pigparse, slog, item-master, icon]

# Dependency graph
requires:
  - phase: 37-item-enrichment-backbone-flags-effects
    provides: "item_master flag/effect columns (00016) + the GetItemMasterFreshnessTx short-circuit that already compares icon_id"
  - phase: 31-item-enrichment (icon)
    provides: "item_master.icon_id (00012) + the colored-tile-vs-wiki-icon client render"
provides:
  - "store.DistinctEnrichmentRefs — held EQ-id ∪ catalog-only PigParse-id union read, deduped by lower(trim(name)), held wins, with the two D-04 collision-guard exclusions"
  - "runWikiItems widened to the full PigParse Blue catalog (ENRICH-14) — the weekly wiki pass now enriches every catalog item, not only held ones"
  - "icon backfill for unheld catalog items (ENRICH-15) — falls out of the widened set + the existing freshness short-circuit; no new icon code"
  - "the D-03 'items coverage' slog summary (total/enriched/icon_covered/icon_less + bounded residue name sample)"
affects: [39-faceted-search, search-scope-toggle, SEARCH-04, SEARCH-05, SEARCH-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "held∪catalog union read deduped by normalized name (held EQ id wins; catalog arm keyed by PigParse id only for unheld names, with EQ-id + held-name exclusions)"
    - "loop-accumulated coverage counters → one bounded structured slog summary per pass (no extra store read)"

key-files:
  created: []
  modified:
    - "internal/backendsrv/store/itemids.go — added DistinctEnrichmentRefs"
    - "internal/backendsrv/store/itemids_test.go — added TestDistinctEnrichmentRefs"
    - "internal/backendsrv/enrich/jobs/wiki.go — loop input swap + D-03 coverage slog + bounded-residue helpers"
    - "internal/backendsrv/enrich/jobs/wiki_test.go — added TestRunWiki_EnrichesUnheldCatalogItem + seedCatalogItem helper"

key-decisions:
  - "D-04 Option A (admit catalog-only rows into item_master keyed by PigParse id, only for unheld names, with the EQ-id-collision exclusion) — no new table, no new migration (00016 stays last)"
  - "D-03 coverage counted by loop accumulation (no extra store read): IconID>0 = covered; IconID==0 / fetch-skip / parse-fail = icon-less residue; 304 = already-fresh, not counted as actionable residue"
  - "Residue sample bounded at 50 names; the full icon_less COUNT is still reported so no count is lost (T-38-04 self-DoS guard)"

patterns-established:
  - "Pattern: a single tested SQL union read in store/ feeds the job; the job authors zero inline SQL (11-05)"
  - "Pattern: widening a crawl's ref set is the entire icon-backfill mechanism — the freshness short-circuit already carries icon_id, so no BackfillItemIcon boot pass is added"

requirements-completed: [ENRICH-14, ENRICH-15]

# Metrics
duration: 35min
completed: 2026-06-25
---

# Phase 38 Plan 01: Catalog-wide Enrichment + Icon Coverage Summary

**The weekly wiki pass now enriches the full ~4,341-row PigParse Blue catalog (held∪catalog union deduped by name) instead of only held items, backfilling icons for unheld catalog items via the existing freshness short-circuit, with a bounded D-03 coverage slog summary — no new table, no migration, watcher untouched.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-06-25 (execution session)
- **Completed:** 2026-06-25
- **Tasks:** 2 (both TDD)
- **Files modified:** 4

## Accomplishments
- **ENRICH-14:** `runWikiItems` swapped its loop input from the held-only `DistinctInventoryItemIDs` gate to the new `store.DistinctEnrichmentRefs` (held EQ-id ∪ catalog-only PigParse-id, deduped by `lower(trim(name))`). The entire fetch/parse/freshness/upsert loop body is byte-for-byte unchanged — only the ref SET grew to the full PigParse Blue catalog.
- **ENRICH-15 (icon backfill):** widening the ref set is the whole mechanism — an unheld catalog item now gets a wiki fetch, its `lucy_img_ID` is parsed, and `icon_id` is populated through the existing `GetItemMasterFreshnessTx` comparison (which already compares `icon_id`). No new icon code, no `BackfillItemIcon` boot pass.
- **ENRICH-15 (D-03 diagnostic):** one structured `slog` "items coverage" line per pass — `total` / `enriched` / `icon_covered` / `icon_less` + a bounded (≤50) sample of icon-less **public wiki page names** only. A maintainer can grep the VPS logs to see the residue.
- **D-04 collision guard:** the catalog arm of the union excludes (1) any normalized name already held (held EQ-id row wins) and (2) any PigParse `item_id` already present in `item_master` (so `ON CONFLICT(item_id) DO UPDATE` can never overwrite the wrong row). Both exclusions are tested.
- Held readers (Phases 31/32/37) are byte-for-byte unaffected — verified by the full backend suite passing (17 packages green).

## Task Commits

Each task was committed atomically (both TDD, RED→GREEN done within one feat commit each since the test+impl land together):

1. **Task 1: Add store.DistinctEnrichmentRefs (held∪catalog union + test)** — `59e7180` (feat)
2. **Task 2: Swap runWikiItems input + D-03 coverage slog + widen job test** — `ac85211` (feat)

_TDD flow per task: wrote the test first and confirmed RED (Task 1: method-undefined compile fail; Task 2: `no rows in result set` for the unheld catalog item), then implemented to GREEN. The test + impl are committed together per task._

## Files Created/Modified
- `internal/backendsrv/store/itemids.go` — added `(*Store).DistinctEnrichmentRefs`: the held∪catalog union read with both D-04 exclusions and the normalized-name dedup; full doc comment on the two exclusions and the per-name "fetch each page once" politeness rule.
- `internal/backendsrv/store/itemids_test.go` — added `TestDistinctEnrichmentRefs` covering held-dedup, unheld-catalog (Case A), held-wins (Case B incl. a casing/whitespace variant), id-collision-exclusion (Case C), blank-name exclusion, and one-ref-per-normalized-name (Case D / Pitfall 1). Added the local `normEnrich` helper mirroring the SQL dedup key.
- `internal/backendsrv/enrich/jobs/wiki.go` — the one-line loop input swap to `DistinctEnrichmentRefs`; loop-accumulated `iconCovered`/`iconLess`/`residueNames`; the `logItemsCoverage` D-03 slog helper; the `appendBoundedResidue` helper + `residueSampleCap = 50` bound.
- `internal/backendsrv/enrich/jobs/wiki_test.go` — added `TestRunWiki_EnrichesUnheldCatalogItem` (proves an unheld "Cloak of Flames" catalog row gets an `item_master` row with `icon_id > 0`, the held item is preserved, and a junk catalog name with no wiki page does not abort the run) + the `seedCatalogItem` raw-insert helper.

## Decisions Made
- **D-04 → Option A** as planned: catalog-only rows go into `item_master` keyed by the PigParse `item_id`, but only for unheld names, with the EQ-id-collision exclusion. No new table, no migration (00016 stays the last on disk).
- **D-03 coverage by loop accumulation** (the plan's recommended path) rather than a post-loop store read: `IconID > 0` → covered; `IconID == 0` or fetch-skip or parse-fail → icon-less residue; a 304 (already-fresh) item is counted toward `enriched` but not toward icon-less (it is not actionable residue and we deliberately do not re-read its icon).
- **Left `DistinctInventoryItemIDs` in place** (store method + its test) — it is the analog and is still tested; the plan only required removing its reference from the job (now 0). It is an exported method, so no `unused` lint concern.

## Deviations from Plan

None — plan executed exactly as written. (Two cosmetic adjustments, neither a behavioral deviation: the D-03 helper is a named `logItemsCoverage`/`appendBoundedResidue` pair rather than inline, matching the plan's "write a tiny helper" suggestion; and the loop-input-swap comment was worded to avoid the literal `DistinctInventoryItemIDs` token so the job-grep acceptance criterion reads 0.)

## Issues Encountered
- **D-04 prod collision probe: DEFERRED to pre-deploy.** The read-only probe (`SELECT count(*) FROM pigparse_price p JOIN item_master m ON p.item_id = m.item_id WHERE lower(trim(p.name)) <> lower(trim(m.name))`) requires prod SSH to the Hetzner VPS (5.78.232.85), which needs the Windows ssh-agent SERVICE key — not available in this non-interactive execution environment (both Git Bash `ssh` and PowerShell `ssh.exe` returned `Permission denied (publickey,password)`). Per the plan's documented disposition, the EQ-id exclusion is the **safe default** and is correct independent of the count, so the code shipped with the exclusion applied. **Run the probe before the next backend deploy** — expected count ≈ 0 (a few colliding names, if any, simply stay icon-less and surface in the D-03 residue). If the count is material (>~20), escalate to a name-keyed fallback for the excluded set per the plan (still no new table) before relying on it.

## Verification (phase gates — from `internal/backendsrv/`)
- `go build ./...` → exit 0 (and repo-root `go build ./...` → exit 0).
- `go vet ./...` → exit 0.
- `go test ./...` → exit 0 — all 17 backend packages green (store, enrich/jobs, compute, readapi, webadmin, ingest, etc.).
- `go test ./store/ -run TestDistinctEnrichmentRefs` → PASS (held-dedup, unheld-catalog, held-wins, id-collision-exclusion, blank-name, one-ref-per-name).
- `go test ./enrich/jobs/ -run TestRunWiki` → PASS — the new `TestRunWiki_EnrichesUnheldCatalogItem` plus all 8 pre-existing `TestRunWiki_*` (PopulatesAllTables, SHA1ShortCircuit, BackfillsStaleIcon, BackfillsStaleFlags, 304SkipsResource, GearFullReplaceNoDuplicates, GearSinglePageChangeLands, OneBadPageDoesNotAbort).
- Migration check: `00016` is the last migration on disk (no `00017`); the goose-on-boot log in the test confirms "migrated database to version: 16".

## What did NOT change (per plan)
- **No new migration** — 00016 stays the last; `item_master` already had every column the parse writes.
- **No `BackfillItemIcon` boot pass** — an unheld item has no local statsblock to re-parse; the crawl is its only icon source (contrast Phase 37's `BackfillItemFlags`).
- **No `WatcherMaxSchemaVersion` change** — that gate no longer exists post-v2.0 (`internal/sheet` is gone).
- **Watcher untouched; no `v*` tag.** Backend-only.
- The gear-tier pass (deliberate unconditional full-replace) and `upsertItemAndQuests` / the freshness short-circuit were not touched.

## Next Phase Readiness
- **Deploy note:** this phase's enrichment rides the NEXT backend deploy, which also carries the not-yet-deployed Phase 37 migration 00016 / schema bump. The first prod weekly wiki run after deploy does the ~72-min seed crawl over the full catalog (1 req/s, capless job, ETag-cheap thereafter). **Run the deferred D-04 prod probe before that deploy.**
- **Downstream contract for Phase 39 (SEARCH-04/05/06):** held + catalog reconcile through the existing normalized-name dedup (the `pp_rep` CTE / the item rollup's `GROUP BY lower(trim(name))`), yielding one row per item regardless of which id namespace keyed its `item_master` row. The widened catalog is now the data the full-catalog search scope ("what exists") reads.

---
*Phase: 38-catalog-wide-enrichment-icon-coverage*
*Completed: 2026-06-25*
