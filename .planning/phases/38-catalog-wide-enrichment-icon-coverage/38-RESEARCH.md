# Phase 38: Catalog-wide enrichment + icon coverage - Research

**Researched:** 2026-06-25
**Domain:** Go backend — widen the weekly wiki enrichment loop from held-items to the full PigParse Blue catalog; backfill icons; emit a coverage diagnostic
**Confidence:** HIGH (everything verified against the codebase; no external/network claims)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Seed once, then the weekly pass re-validates the WHOLE catalog via ETag. First wiki run after deploy does the full paced crawl over all ~4,341 catalog items (~70+ min, 1 req/s — one Sunday). Every weekly pass thereafter sends `If-None-Match` for ALL catalog items: unchanged pages 304 cheaply (still 1s-spaced), changed pages re-parse. The weekly job has NO execution cap (`wiki.go` header comment). Simplest, always-self-healing, courteous at 1 req/s. Rejected: "weekly re-checks held-only + unheld on a slow rotation" and "throttle initial crawl across days."
- **D-01a:** The crawl runs automatically as part of the existing weekly wiki job (no new manual admin trigger). The existing job-level 1s `wikiSleepFn` + ETag 304 short-circuit + log-and-skip-one-bad-page resilience all apply unchanged to the widened set.
- **D-02:** "Full catalog" = the PigParse auction catalog only (`pigparse_price`, ~4,341 rows). Do NOT union never-auctioned `wiki_gear_tier` items.
- **D-03:** A per-run structured `slog` summary line is the maintainer-visible coverage surface (total / enriched / icon-covered / icon-less + residue names or a bounded sample). No new UI surface. Rejected for this phase: an officer Admin "Item coverage" panel and a queryable/downloadable report.
- **D-04 — FLAGGED FOR RESEARCH (the load-bearing technical risk):** how to STORE enrichment for catalog-only (unheld) items + how SEARCH-06 reconciles held vs. unheld without name-duplication. See `## D-04 RESOLUTION` below.

### Claude's Discretion
- Migration mechanics (next number after 00016 → **00017** if a new table/column is needed; may be none if a name-keyed read suffices); whether `WatcherMaxSchemaVersion` needs a bump (backend-only, watcher off the read path — almost certainly not; `internal/sheet` no longer exists post-v2.0 so that constant is moot).
- The icon backfill reuses the Phase 37 freshness short-circuit pattern (`GetItemMasterFreshnessTx` already compares `icon_id`; 00012 established "parse one field, nullable column, add to freshness, backfill on next pass").

### Deferred Ideas (OUT OF SCOPE)
- Maintainer-triggered re-crawl button (manual kickoff from the Admin panel).
- Officer Admin "Item coverage" panel / queryable coverage report.
- Search facets beyond Clicky/Haste and the SEARCH-06 UI itself — Phase 39, not here.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ENRICH-14 | Item enrichment covers the full PigParse Blue catalog, not only held items — politefetch-paced. | Widen the loop in `runWikiItems` to iterate a held∪catalog union deduped by normalized name (`## D-04 RESOLUTION`, recommended Option A). Runtime/courtesy math in `## Crawl Runtime + Resilience Math`. The existing 1s `wikiSleepFn` + ETag 304 + log-and-skip resilience carry over unchanged. |
| ENRICH-15 | Icon coverage backfilled for every item whose wiki page provides one + a maintainer can see which items are still icon-less. | The icon is ALREADY in `GetItemMasterFreshnessTx`'s comparison — widening the item set newly populates icons for unheld items with NO new freshness logic. `parseIconID` already extracts `lucy_img_ID`. The residue diagnostic is the D-03 slog summary (`## D-03 slog Summary Line`). "Icon-less" classification in `## Icon-Backfill Mechanics`. |
</phase_requirements>

## Summary

Phase 38 is a **loop-widening + storage-identity** phase, not a parser phase. The wiki item parser
(`enrich/wikiitem.go`), the per-item upsert (`store/enrich.go::UpsertItemMasterTx`), the freshness
short-circuit (`GetItemMasterFreshnessTx` — already compares `icon_id`), the 1s courtesy sleep, the
ETag 304 short-circuit, and the log-and-skip-one-bad-page resilience all already exist and already
do everything per-item. The ONLY behavioral change ENRICH-14 needs is to change the SET of refs the
`runWikiItems` loop iterates: today `store.DistinctInventoryItemIDs` returns only items some
character HOLDS (~hundreds); the phase widens that to a **held ∪ catalog union, deduped by
normalized name** (~4,341). Because the icon already participates in the freshness comparison,
ENRICH-15 (icon backfill) falls out of ENRICH-14 for free — widening the item set is what newly
populates icons for unheld items.

The single load-bearing decision is **D-04**: where to store enrichment for catalog-only (unheld)
items, given that `item_master` is keyed by `item_id INTEGER PRIMARY KEY` in the **EQ-inventory**
namespace while the PigParse catalog `pigparse_price.item_id` is a **DIFFERENT namespace** (catalog↔inventory
join is by normalized name, NEVER raw item_id — `pigparse-vs-ingame-item-id-namespaces` memory,
Phase 29 DATA-01). The recommended resolution (Option A) admits catalog-only rows into `item_master`
keyed by the **PigParse `item_id`** — but ONLY for normalized names that have no held EQ-id row, so
no PK collision is possible and held readers (Phases 31/32/37) are byte-for-byte unaffected. SEARCH-06
(Phase 39) then reads held + catalog through the existing normalized-name dedup that the rollup and
the `pp_rep` CTE already use. **No new migration is needed** — `item_master` already has every column
the parse writes (00012/00013/00016). The phase is backend-only, the watcher is untouched, and there
is no `v*` tag and no `WatcherMaxSchemaVersion` concern.

**Primary recommendation:** Widen `runWikiItems` to iterate a new `store.DistinctEnrichmentRefs`
(held ∪ catalog, deduped by `lower(trim(name))`, held EQ-id wins so its existing `item_master` row
keeps its EQ id) → reuse the existing per-item fetch/parse/freshness/upsert path verbatim → emit one
`slog` coverage summary at the end of `runWikiItems`. NO new table, NO new migration (00016 is the
last), NO watcher change.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Widen the enrichment ref set (held ∪ catalog by name) | API/Backend — `store` read method | Enrichment job (`enrich/jobs/wiki.go`) | The dedup-by-name set is a SELECT over `inventory_item` + `pigparse_price`; lives in `store/` per the single-tested-SQL-path rule. |
| Fetch + parse + upsert each item's wiki page | Enrichment job (`enrich/jobs/wiki.go`) | Pure parser (`enrich/wikiitem.go`) | Already the established pattern; unchanged per item. |
| Store enrichment for catalog-only items | Database (`item_master`) | — | Recommended Option A reuses `item_master` keyed by PigParse id for unheld names. No new table. |
| Icon backfill | Enrichment job (freshness short-circuit) | Database (`item_master.icon_id`) | Icon already in `GetItemMasterFreshnessTx`; widening the set is the whole mechanism. |
| Coverage diagnostic | Enrichment job (`slog`) | — | D-03: structured log line, no UI/read-API. |
| SEARCH-06 held-vs-catalog reconciliation (downstream) | API/Backend read | compute (normalized-name dedup) | Phase 39 reads; the dedup contract is set HERE. |

## D-04 RESOLUTION (the load-bearing section)

### The tension, restated against the code

- `item_master` is `CREATE TABLE item_master (item_id INTEGER PRIMARY KEY, …)` — keyed in the **EQ-inventory**
  namespace (the `/outputfile` `ID` column the watcher uploads). `[VERIFIED: migrations/00001_init.sql:59]`
- `pigparse_price` is `CREATE TABLE pigparse_price (item_id INTEGER PRIMARY KEY, …)` — keyed in the
  **PigParse catalog** namespace. `[VERIFIED: migrations/00001_init.sql:60]`
- These two `item_id` spaces are DIFFERENT. Only ~58/713 inventory ids exist in the catalog *by id*,
  vs ~559 *by name* — so the price/enrichment bridge is `lower(trim(name))`, NEVER raw `item_id`.
  `[VERIFIED: store/readviews.go:36-47; itemrollup.go:16-21; memory pigparse-vs-ingame-item-id-namespaces]`
- Today `runWikiItems` iterates `DistinctInventoryItemIDs` (held EQ ids only) and the upsert keys
  `item_master` by `ref.ItemID` (an EQ id). `[VERIFIED: store/itemids.go:40-64; jobs/wiki.go:160-214,247]`

So a catalog-only (unheld) item has **no EQ-inventory item_id** to key an `item_master` row by, and
naively reusing the PigParse id as the `item_master` PK *could* collide with a real EQ id for a
different item.

### The collision is structurally avoidable

The dedup-by-name union means: for any normalized name that IS held, we already have an EQ-id
`item_master` row (or will get one this pass). We only ever mint a catalog-keyed row for a name that
is **NOT held**. A held name and its catalog row are the SAME item — they must merge to ONE row, not
appear twice. So the only rows ever inserted under a PigParse id are for names with no EQ-id row at
all. A PigParse id `P` collides with an EQ id `E` only if `P == E` AND they are different items AND
both reach the upsert — but the held one always wins the name in the union, so its EQ-id row is the
one written, and the catalog row for that same name is never separately keyed. The residual risk
(a PigParse id numerically equal to an EQ id for a *different, unheld* name) is real but inert: each
is a distinct normalized name, so each is one independent `item_master` row; `item_master` is keyed
by id, and two different unheld names with colliding numeric ids WOULD clash. **This is the one real
hazard and the planner must guard it** — see Option A's collision guard below.

### Options

| # | Storage model | Collision-safety vs EQ id PK | Extend-only | Blast radius on held readers (P31/32/37) | How the loop iterates | How SEARCH-06 reads held+unheld | Migration cost |
|---|---------------|------------------------------|-------------|------------------------------------------|----------------------|--------------------------------|----------------|
| **A (RECOMMENDED)** | Admit catalog-only rows into **`item_master` keyed by PigParse `item_id`**, ONLY for names with no held EQ-id row. Held names keep their EQ-id row. | Safe IF the union dedups by name (held wins) AND a guard skips a catalog id already present as a different name's EQ row. The numeric-collision-of-two-unheld-names case is the residue to guard (rare; see below). | YES — zero schema change; `item_master` already has every column (00012/00013/00016). | **Zero.** Held readers join `item_master` by EQ id exactly as today; their rows are untouched. New rows exist only for names no held reader looks up. | One union read `DistinctEnrichmentRefs` returns `(item_id, name)` where item_id is the EQ id for held names and the PigParse id for catalog-only names; the loop is otherwise byte-identical. | The existing `pp_rep`-style normalized-name dedup already collapses held + catalog to one row per name (rollup/itemsearch). SEARCH-06 reads `item_master` joined by name, deduped — one row per item regardless of which id keyed it. | **None** (00016 is last). |
| B | New side table `catalog_enrichment(norm_name TEXT PRIMARY KEY, icon_id, statsblock, flags_json, …)` joined by normalized name; `item_master` stays held-only. | Fully collision-proof (different table, name PK). | Adds table → migration 00017. | Low-but-nonzero: every held reader that wants enrichment for catalog scope must now LEFT JOIN a second table by name and COALESCE EQ-id `item_master` vs name-keyed `catalog_enrichment`. Two sources of truth for the same fields. | Same union loop, but the upsert branches: held → `item_master` by id; catalog-only → `catalog_enrichment` by name. Two write paths, two freshness getters. | SEARCH-06 must UNION/COALESCE two enrichment sources by name. More join logic, more drift risk. | One migration + a parallel store layer (upsert/freshness/read) for the side table. |
| C (hybrid) | `item_master` for held (EQ id) + `catalog_enrichment` for unheld (name PK), but a nightly reconciler promotes a catalog row into `item_master` the moment an item becomes held. | Collision-proof. | Migration 00017 + a reconciler job. | Highest — adds a promotion/migration step and a window where the same item lives in both tables. | Union loop + branch + reconciler. | Same as B plus the promotion edge cases. | Migration + side table + reconciler job. |

### Recommendation: Option A

**Rationale:**
- **Smallest blast radius — verified zero on held readers.** Phases 31/32/37 read `item_master` by EQ
  `item_id` (`ItemMasterIconStats` `[VERIFIED: store/readviews.go:770-793]`; the rollup's id-correct
  icon/stats lookup `[VERIFIED: itemrollup.go:73-81]`; the inventory join's `im.item_id = ii.item_id`
  `[VERIFIED: readviews.go:299]`). Those reads only ever look up ids that ARE held, so the new
  catalog-only rows are invisible to them. No existing query changes.
- **No new migration, no new store layer.** `item_master` already carries `icon_id` (00012),
  `statsblock` (00013), and all nine flag/effect columns + `flags_json` (00016)
  `[VERIFIED: store/enrich.go:204-216]`. The upsert (`UpsertItemMasterTx`) and freshness getter
  (`GetItemMasterFreshnessTx`) work for any `item_id` regardless of namespace. The whole phase is a
  read-method addition + a loop input swap.
- **One enrichment table = one source of truth.** SEARCH-06's held-vs-catalog scope is just "filter
  the existing per-name rollup to held, or show all" — the normalized-name dedup that already powers
  the item rollup (`buildItemRollups`, group by `lower(trim(name))` `[VERIFIED: itemrollup.go:69]`)
  and the `pp_rep` CTE (`[VERIFIED: readviews.go:287-302]`) already guarantees one row per item. No
  COALESCE-across-two-tables logic to write or test.

**The required collision guard (the one real hazard).** The union must enforce: a normalized name
that is held keeps its EQ-id row; a catalog-only name is keyed by its PigParse id, BUT if two distinct
catalog-only names would map to numerically-equal ids, or a catalog id numerically equals an existing
EQ-id row that belongs to a *different* name, the upsert's `ON CONFLICT(item_id) DO UPDATE` would
silently overwrite the wrong row. Mitigations the planner should choose between (recommend the first):
1. **Held-wins union with an EQ-id exclusion** (recommended): the catalog arm of the union excludes
   any `pigparse_price.item_id` that already exists in `item_master` (so a catalog id equal to a held
   EQ id never reaches the upsert), and excludes any normalized name already produced by the held arm
   (so held names win). Two catalog-only names sharing a numeric id is not observed in the data (PigParse
   ids are unique per catalog row, and the dedup is by name not id), but the planner should add a test
   asserting one `item_master` row per normalized name after a full crawl. `[ASSUMED]` that
   PigParse-id-equals-EQ-id-for-a-different-item is rare enough to exclude rather than re-key — verify
   against prod counts (a quick `SELECT count(*) FROM pigparse_price p JOIN item_master m ON p.item_id=m.item_id WHERE lower(trim(p.name)) <> lower(trim(m.name))`).
2. Synthetic/namespaced id (e.g. offset PigParse ids by a large constant) — rejected: it leaks a
   fake id into `item_master.item_id`, which several readers treat as a real EQ id, and it complicates
   the icon URL / examine lookups. Do NOT do this.

**The union read shape.** Add a `store.DistinctEnrichmentRefs(ctx) ([]ItemRef, error)` next to
`DistinctInventoryItemIDs` (`store/itemids.go`), returning `(item_id, name)` where for a held name
`item_id` is the EQ id (`MIN`/representative, exactly as today) and for a catalog-only name `item_id`
is the PigParse id. Sketch (planner refines, keep ZERO inline SQL in the job per the
single-tested-SQL-path rule):

```sql
-- held arm: the existing DistinctInventoryItemIDs (EQ ids), one rep per item_id
WITH held AS (
  SELECT item_id, MIN(name) AS name, lower(trim(MIN(name))) AS norm
  FROM inventory_item
  WHERE item_id IS NOT NULL AND item_id > 0
  GROUP BY item_id
),
held_names AS (SELECT DISTINCT lower(trim(name)) AS norm FROM inventory_item WHERE item_id > 0),
-- catalog arm: pigparse_price names NOT held, NOT already an EQ-id row, one rep per name
catalog AS (
  SELECT MIN(item_id) AS item_id, MIN(name) AS name
  FROM pigparse_price
  WHERE name IS NOT NULL AND trim(name) <> ''
    AND lower(trim(name)) NOT IN (SELECT norm FROM held_names)
    AND item_id NOT IN (SELECT item_id FROM item_master)   -- EQ-id-collision exclusion
  GROUP BY lower(trim(name))
)
SELECT item_id, name FROM held
UNION ALL
SELECT item_id, name FROM catalog
ORDER BY item_id;
```

Note the held arm's representative-name pick must stay `MIN(name)` (the politeness rule:
fetch each item's page exactly once — see `## Pitfalls`). The `ItemRef` struct is unchanged
(`{ItemID int64; Name string}` `[VERIFIED: store/itemids.go:25-28]`).

### Is a new migration needed? NO.

`item_master` already has `item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1,
last_refreshed, icon_id, statsblock, is_lore, is_no_drop, is_magic, is_temporary, is_clicky,
clicky_effect, has_haste, haste_pct, flags_json` `[VERIFIED: store/enrich.go:204-216 itemMasterUpsert]`.
The catalog-only rows write the same columns through the same `UpsertItemMasterTx`. **00016 stays the
last migration; there is NO 00017 in this phase.** (If the planner picks Option B/C instead, then a
00017 is required — but the recommendation is A, which needs none.)

## Crawl Runtime + Resilience Math (ENRICH-14)

- **Courtesy sleep:** `interRequestSleep = 1 * time.Second`, applied via `wikiSleepFn(ctx, …)` BEFORE
  every page fetch in the items loop. `[VERIFIED: jobs/wiki.go:49-51,168-172]`
- **Catalog size:** ~4,341 PigParse Blue rows `[CITED: 38-CONTEXT.md D-01; memory: 4341-row catalog]`.
  Held set is the existing ~hundreds; the union is held ∪ catalog-only ≈ catalog size + held-names-not-in-catalog.
- **First (seed) run:** ~4,341 × ~1s sleep + fetch/parse ≈ **~72+ minutes** of wall time on the first
  Sunday. This is explicitly fine — the job has NO execution cap (`wiki.go` header
  `[VERIFIED: jobs/wiki.go:3-31]`), runs in a background goroutine, and the single-writer DB + per-job
  mutex serialize writes. The spec says "a 70-minute Sunday background run is explicitly fine; do not
  add complexity to make it shorter" `[CITED: 38-CONTEXT.md ## Specific Ideas]`.
- **Steady-state weekly run:** every page sends `If-None-Match`; unchanged pages 304 (still 1s-spaced,
  so wall time stays ~72 min but with near-zero parse/DB cost). The ETag 304 short-circuit
  (`fetchUnchanged` → `continue`) skips parse+write `[VERIFIED: jobs/wiki.go:176-183,476-488]`. The
  ETag is now persisted after EVERY successful fetch+parse, write or not — the prior bug where no-write
  items had no cached ETag (and thus re-fetched every run) was already fixed
  `[VERIFIED: jobs/wiki.go:199-206]`. So the widened set 304s cheaply once seeded.
- **Resilience scales unchanged:** a single bad catalog page (fetch error / MediaWiki error envelope /
  empty wikitext / parse failure) is logged-and-SKIPPED via `fetchSkip` and `!ok` reason — the run
  NEVER aborts on one page `[VERIFIED: jobs/wiki.go:176-198,518-547]`. ctx cancellation (SIGTERM
  mid-crawl) unwinds cleanly and returns the partial counts, not an error `[VERIFIED: jobs/wiki.go:168-172]`.
- **No day-boundary bookkeeping** — the capless job is one uninterrupted pass; the scheduler runs it
  once per Sunday UTC (`dueWiki`: Sunday AND last < start-of-this-Sunday `[VERIFIED: scheduler/scheduler.go:98-106]`).

## Icon-Backfill Mechanics (ENRICH-15)

- **The icon is ALREADY parsed and ALREADY in the freshness comparison.** `parseIconID` extracts
  `lucy_img_ID` (returns 0 for absent/blank/non-numeric/negative — the "no icon" sentinel)
  `[VERIFIED: enrich/wikiitem.go:584-601]`. `ParseItempage` sets `item.IconID`
  `[VERIFIED: enrich/wikiitem.go:126]`. The upsert writes `icon_id` `[VERIFIED: store/enrich.go:240]`.
  The freshness getter returns `iconID` and the job re-writes whenever `existingIcon != int64(item.IconID)`
  `[VERIFIED: store/enrich.go:282-294; jobs/wiki.go:227-245]`.
- **So ENRICH-15 needs NO new icon logic.** Widening the item set (ENRICH-14) is the entire mechanism:
  unheld catalog items now get a wiki fetch, their `lucy_img_ID` is parsed, and `icon_id` is populated.
  Already-enriched held items with a stale `icon_id` already self-heal via the 00012 freshness
  precedent (there is even a regression test `TestRunWiki_BackfillsStaleIcon`
  `[VERIFIED: jobs/wiki_test.go:190-236]`).
- **What makes an item "icon-less" (two distinct cases — the diagnostic must distinguish):**
  1. **Genuinely icon-less:** the item's wiki page has no `lucy_img_ID` param → `parseIconID` returns
     0 → `icon_id = 0` → the client renders the colored-tile fallback (the intended permanent
     behavior). `[VERIFIED: migrations/00012_item_icon.sql:7-9; enrich/wikiitem.go:585-589]`
  2. **Not-yet-enriched:** the item has no `item_master` row at all yet (never fetched), OR a row whose
     `icon_id` is NULL/0 because it predates a successful enrichment. After a full crawl these should
     be empty; any residue is either case 1 or a page that 304'd/failed.
  The residue the maintainer cares about (ENRICH-15 "still icon-less") is the count of catalog items
  whose `item_master.icon_id` is 0/NULL after the crawl — that IS the colored-tile set.
- **No boot backfill needed for icons.** Phase 37 added a one-time `BackfillItemFlags` boot pass for
  flags because flags were a NEW field with no network source on existing rows (re-parse the stored
  statsblock) `[VERIFIED: store/backfill.go:1-106; main.go:228-239]`. Icons are DIFFERENT: an unheld
  catalog item has NO stored statsblock to re-parse and NO `item_master` row at all — the only source
  of its icon is a fresh wiki fetch, which the widened weekly crawl performs. So the icon "backfill"
  IS the crawl; there is no analog boot pass. (Held items' stale icons already self-heal via the
  freshness path.) Do NOT add a `BackfillItemIcon` boot function — it would have nothing local to read.

## D-03 slog Summary Line (ENRICH-15 maintainer diagnostic)

- **Where:** at the END of `runWikiItems` (`jobs/wiki.go`), right before `return written, unchanged,
  failed, nil` — it already returns `(written, unchanged, failed)` counts `[VERIFIED: jobs/wiki.go:160-213]`.
  Compute the icon/coverage counts during the same loop (increment counters as each item is
  fetched/parsed/upserted) or with one post-loop store read.
- **What to log (minimum per D-03):** total catalog/union size, count enriched (got an `item_master`
  row this pass or already fresh), count icon-covered (`icon_id > 0`), count icon-less (`icon_id` 0/NULL),
  plus a BOUNDED sample of residue item names (cap the slice — e.g. first 50 — so a ~hundreds-long
  residue never floods the log). Follow the package convention: structured key/value `slog`, never raw
  page content (V7). The existing job already logs a `detail` summary string with counts
  `[VERIFIED: jobs/wiki.go:149-153]` — extend that pattern.
- **Shape (planner refines field names):**
  ```go
  slog.Info(wikiJobName+": items coverage",
      "total", total,            // union size iterated
      "enriched", enriched,      // rows with an item_master row after the pass
      "icon_covered", iconCovered,
      "icon_less", iconLess,
      "residue_sample", sampleNames, // bounded slice (e.g. <=50)
  )
  ```
- **Why slog, not a UI/read-API:** the maintainer already greps `slog` JSON on the VPS (the
  established ops pattern; CLAUDE.md structured-logging convention) `[CITED: 38-CONTEXT.md D-03; CLAUDE.md Conventions]`.
  Zero new surface, lowest cost. An Admin panel / queryable report is explicitly deferred.

## Standard Stack

This phase adds **no new dependency**. It edits existing Go backend packages only.

| Package (existing) | Purpose in this phase |
|--------------------|----------------------|
| `internal/backendsrv/store` | New `DistinctEnrichmentRefs` union read (held ∪ catalog by name) + coverage-count reads. Single tested SQL path. |
| `internal/backendsrv/enrich/jobs` | `runWikiItems` swaps its ref source to the union; emits the D-03 coverage `slog` line. |
| `internal/backendsrv/enrich` | Parser UNCHANGED — `ParseItempage`/`parseIconID` already produce everything. |
| `internal/backendsrv/store` (`enrich.go`) | `UpsertItemMasterTx` / `GetItemMasterFreshnessTx` UNCHANGED — already namespace-agnostic on `item_id`. |

**Go version:** 1.24 per CLAUDE.md (watcher) / the backend module. No version-sensitive APIs are
introduced. `[ASSUMED]` — backend builds under the same toolchain the repo CI uses; no new module.

## Architecture Patterns

### System data flow (the widened items pass)

```
                 ┌─────────────────────────────────────────────┐
                 │ scheduler.dueWiki (Sunday UTC, once/Sunday)  │
                 └───────────────────┬─────────────────────────┘
                                     │ RunWiki(ctx, db, politefetch.Fetch)
                                     ▼
        ┌────────────────────────── runWikiItems ───────────────────────────┐
        │  refs := store.DistinctEnrichmentRefs(ctx)   ← WIDENED (held∪catalog│
        │                                                  deduped by name)   │
        │  for each ref:                                                      │
        │     wikiSleepFn(ctx, 1s)          ← courtesy pace, ctx-aware        │
        │     fetchWikiPage(ETag)  ──304──► unchanged++; continue (Pitfall 6) │
        │            │ 200                                                    │
        │            ▼                                                        │
        │     ParseItempage(wikitext) ─!ok─► failed++; continue (log+skip)    │
        │            │ ok (IconID, flags, statsblock, …)                      │
        │            ▼                                                        │
        │     GetItemMasterFreshnessTx(ref.ItemID)  ← sha|icon|stats|flags    │
        │            │ all 4 unchanged ─► unchanged++; continue               │
        │            │ any differs                                            │
        │            ▼                                                        │
        │     UpsertItemMasterTx(... item_id=ref.ItemID ...)  ← namespace-    │
        │            │                                          agnostic id   │
        │            ▼   SetETag(url)                                         │
        │     written++                                                      │
        │  ── after loop: slog "items coverage" (total/enriched/icon/residue) │← D-03
        └────────────────────────────────────────────────────────────────────┘
                                     │
   downstream (Phase 39 SEARCH-06):  ▼  reads item_master + pigparse_price,
              one row per item via the existing normalized-name dedup (pp_rep / rollup)
```

### Pattern 1: held ∪ catalog union deduped by normalized name (held wins)
**What:** a single store read returns `(item_id, name)` refs covering every distinct item by name;
held names carry their EQ id, catalog-only names carry their PigParse id.
**When to use:** the input to the widened `runWikiItems` loop.
**Why:** preserves the existing "fetch each item's page exactly once" politeness (the dedup is by name,
matching the wiki-page identity) and keeps held `item_master` rows on their EQ id so all held readers
are unaffected. `[VERIFIED: store/itemids.go:30-39 politeness rule; readviews.go:287-302 pp_rep dedup]`

### Pattern 2: freshness short-circuit carries the icon for free
**What:** `GetItemMasterFreshnessTx` compares `wikitext_sha1 AND icon_id AND statsblock AND flags_json`;
the job re-writes whenever ANY differs.
**When to use:** unchanged — it already exists.
**Why:** widening the item set is sufficient to populate icons; no separate icon-write path.
`[VERIFIED: store/enrich.go:267-294; jobs/wiki.go:227-245]`

### Anti-Patterns to Avoid
- **Per-(id,name) DISTINCT instead of per-name dedup** — would refetch the same wiki page under
  multiple casings/ids → a politeness regression (re-fetches the same page twice). The existing
  `DistinctInventoryItemIDs` deliberately uses `GROUP BY item_id` + `MIN(name)`; the new union must
  dedup by `lower(trim(name))`. `[VERIFIED: store/itemids.go:30-39]`
- **Synthetic/offset PigParse ids in `item_master.item_id`** — leaks a fake id that held readers treat
  as a real EQ id. Rejected (D-04 Option A note).
- **A second `catalog_enrichment` table (Options B/C)** — two sources of truth for the same fields,
  forces every catalog-scope reader to COALESCE across tables. Avoid unless A's collision guard proves
  infeasible.
- **Adding a `BackfillItemIcon` boot pass** — there is no local data to re-parse for an unheld item;
  the crawl is the only icon source. Don't mirror `BackfillItemFlags` here.
- **Sending ETags for the gear-tier pages** — unrelated to this phase, but note the gear pass is
  deliberately unconditional (full-replace) and must stay so (H-01 staleness trap). This phase touches
  only the items pass. `[VERIFIED: jobs/wiki.go:364-432,490-511]`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Dedup held vs catalog by name | A bespoke in-Go map merge in the job | The `pp_rep`/`lower(trim(name))` SQL dedup already used by the rollup + inventory join | One tested SQL path; the job authors ZERO inline SQL (11-05 rule). `[VERIFIED: itemrollup.go:16-21; readviews.go:287-302]` |
| Per-item wiki fetch + 304 + parse + upsert | A new loop | The existing `runWikiItems` body | It already does everything per item; only the input set changes. `[VERIFIED: jobs/wiki.go:160-291]` |
| Icon extraction | A new icon parser | `parseIconID` | Already parses `lucy_img_ID`, type-safe, sentinel-0. `[VERIFIED: enrich/wikiitem.go:584-601]` |
| Freshness/backfill of icon | A new compare | `GetItemMasterFreshnessTx` (icon already in it) | The 00012 precedent already self-heals icons. `[VERIFIED: store/enrich.go:267-294]` |
| ctx-aware courtesy pacing | A bare `time.Sleep` | `wikiSleepFn` | Unwinds promptly on SIGTERM; test-overridable. `[VERIFIED: jobs/wiki.go:53-77]` |

**Key insight:** Phase 38 is almost entirely *reuse + one input swap + one log line*. The new code is
a single store read method (the union) and a coverage `slog` line. Resist building new write paths,
new tables, or new backfill passes.

## Common Pitfalls

### Pitfall 1: Name-collision between two different items sharing a normalized name
**What goes wrong:** two genuinely different items normalize to the same `lower(trim(name))`; the dedup
merges them into one enrichment row / one search result.
**Why it happens:** the catalog↔held bridge is name-based by necessity (different id namespaces).
**How to avoid:** this is an ACCEPTED tradeoff already baked into the product — the rollup, the price
join, and the wishlist all key on normalized name and already tolerate it (a held item + its catalog
row are deliberately merged). The phase must NOT regress to id-keying to "fix" it. Add a test asserting
one `item_master` row per normalized name after a full crawl. `[VERIFIED: itemrollup.go:16-21; readviews_test.go:412-426]`

### Pitfall 2: PigParse id numerically equal to an EQ id for a different item
**What goes wrong:** the catalog arm mints an `item_master` row under a PigParse id that equals a held
EQ id for a DIFFERENT item; `ON CONFLICT(item_id) DO UPDATE` overwrites the held row's enrichment.
**Why it happens:** the two id namespaces overlap numerically.
**How to avoid:** the catalog arm of the union MUST exclude `item_id`s already present in `item_master`
(the EQ-id exclusion in the union sketch). Verify with the prod probe in D-04. `[VERIFIED: store/enrich.go:204-216 ON CONFLICT(item_id)]`

### Pitfall 3: pigparse_price rows with junk/spell names that have no wiki item page
**What goes wrong:** the catalog contains rows whose `name` is not a real wiki *item* page (spell
scrolls, mislabeled rows, EC-tunnel noise); fetching them 404s / returns a redirect stub / a
non-Itempage page.
**Why it happens:** PigParse is an auction feed, not a curated item list.
**How to avoid:** the existing resilience already handles it — `ParseItempage` returns `ok=false`
("wikitext_too_short" for a redirect stub, "no_itempage" when no `{{Itempage}}` template), the loop
logs `item parse skipped` and `continue`s (failed++), and these names simply stay in the icon-less
residue (the D-03 diagnostic). NO abort. `[VERIFIED: enrich/wikiitem.go:93-101; jobs/wiki.go:185-190]`
A bounded residue sample in the D-03 line surfaces these for the maintainer.

### Pitfall 4: Politeness regression — refetching the same page twice
**What goes wrong:** if the union dedups by id (or by (id,name)) instead of by name, the same wiki page
is fetched under multiple ids/casings → > 1 req/s effective for that page, breaking SC-4 courtesy.
**Why it happens:** held and catalog rows for the same item have different ids; only the NAME is shared.
**How to avoid:** dedup by `lower(trim(name))`; one ref per normalized name. `[VERIFIED: store/itemids.go:30-39]`

### Pitfall 5: Seed run interrupted (server restart / SIGTERM mid-crawl) leaves partial coverage
**What goes wrong:** the ~72-min first run is cut short; only some catalog items are enriched.
**Why it happens:** deploys / reboots happen.
**How to avoid:** this is SELF-HEALING by design — `dueWiki` re-runs next Sunday and the ETag
short-circuit means already-enriched pages 304 cheaply while the un-reached ones get their first fetch.
A mid-crawl ctx cancel returns partial counts cleanly (not an error). No resume bookkeeping needed
(the capless job + ETag is the resume mechanism). `[VERIFIED: jobs/wiki.go:168-172; scheduler/scheduler.go:98-106]`
The job_run marker advances to 'ok' only on full completion `[VERIFIED: jobs/wiki.go:153-154]`.

## Code Examples

### The loop input swap (the ONE behavioral change)
```go
// jobs/wiki.go runWikiItems — today:
refs, rerr := s.DistinctInventoryItemIDs(ctx)   // held EQ ids only
// Phase 38 — widen to the held∪catalog union (deduped by normalized name):
refs, rerr := s.DistinctEnrichmentRefs(ctx)      // NEW store method (Option A)
// ... the rest of the loop body is UNCHANGED.
// [VERIFIED: jobs/wiki.go:160-214 is the unchanged body]
```

### The freshness short-circuit already carries the icon (no change)
```go
// store/enrich.go GetItemMasterFreshnessTx returns sha, iconID, statsblock, flagsJSON
existingSHA, existingIcon, existingStats, existingFlagsJSON, err := store.GetItemMasterFreshnessTx(ctx, tx, ref.ItemID)
if existingSHA == item.WikitextSHA1 && existingIcon == int64(item.IconID) &&
   existingStats == item.Statsblock && existingFlagsJSON == parsedFlagsJSON {
    return false, nil // unchanged — icon already compared
}
// [VERIFIED: jobs/wiki.go:227-245]
```

### The D-03 coverage line (sketch — append to runWikiItems before its return)
```go
slog.Info(wikiJobName+": items coverage",
    "total", total, "enriched", enriched,
    "icon_covered", iconCovered, "icon_less", iconLess,
    "residue_sample", boundedSample(residueNames, 50))
// [pattern from jobs/wiki.go:149-153 detail line]
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Enrichment = held items only (`DistinctInventoryItemIDs`) | Held ∪ full PigParse catalog by name | This phase (38) | Unheld items get icons + flags; powers SEARCH-06 full-catalog scope. |
| Per-field migration for each new derived field | Field already exists in `item_master` (00012/00013/00016) | Phases 31/37 | No 00017 needed — Option A reuses the table. |
| `internal/sheet` + `WatcherMaxSchemaVersion` gate | Gone post-v2.0 ("off Google") | v2.0 | The CLAUDE.md `WatcherMaxSchemaVersion` reference is STALE; no such gate in the backend. `[VERIFIED: migrations/00012,00016 headers state the gate "does not exist in the off-Google backend"]` |

## Project Constraints (from CLAUDE.md)

- **Extend-only schema:** add columns at the right edge / additive side structure; goose on boot;
  migrations version-stamped + idempotent; the `_meta.schema_version` write is the LAST step. → Option A
  needs NO migration; if a migration were added it would be 00017. `[VERIFIED: CLAUDE.md Architecture/Conventions]`
- **`WatcherMaxSchemaVersion` reference is STALE post-v2.0** — `internal/sheet` no longer exists; the
  backend uses goose `version()` as the version of record, no watcher schema gate. No bump, no `v*` tag.
  `[VERIFIED: migrations/00012_item_icon.sql:6-10; 00016 header:15-18]`
- **Structured logging both sides** — Go `slog` with op + key/value fields; logs counts/ids/err only,
  NEVER raw page/flag content (V7). The D-03 diagnostic follows this. `[VERIFIED: CLAUDE.md Conventions; store/enrich.go:22-23]`
- **Single tested SQL path (11-05):** the enrichment job authors ZERO inline SQL — the new union read
  lives in `store/`, the job composes it. `[VERIFIED: store/itemids.go:5-14; enrich.go:1-23]`
- **Watcher untouched, no `v*` tag** — backend + (later) web only this milestone. `[VERIFIED: ROADMAP.md v2.6 scope]`
- **PigParse Blue = server 1 for getall** — the catalog is the Blue catalog (already populated by the
  daily job). `[VERIFIED: jobs/pigparse.go; memory pigparse-server-numbering-blue-is-1]`

## Runtime State Inventory

> Not a rename/refactor/migration phase. The closest analog is "new data lands in `item_master`."

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `item_master` gains rows for unheld catalog items (keyed by PigParse id under Option A). `pigparse_price` is the source (read-only here). | Code: the new union read + the loop swap. No data migration (rows are written by the crawl, idempotently). |
| Live service config | None — the crawl runs inside the existing weekly job; no new scheduler entry, no new external service. | None — verified: `scheduler.Start` already registers `wiki_weekly` `[VERIFIED: main.go:263-268; scheduler/scheduler.go:10]`. |
| OS-registered state | None — no OS task/cron; the in-process scheduler owns cadence. | None. |
| Secrets/env vars | None — no new secret; the wiki API needs no auth. | None. |
| Build artifacts | None — no new package, no new module, no new binary name. | None — verified: edits are within existing `internal/backendsrv` packages. |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| P1999 MediaWiki API (`wiki.project1999.com/api.php`) | The crawl (already used weekly) | ✓ (already in production) | — | log-and-skip per page (existing resilience) |
| `pigparse_price` table populated (~4,341 rows) | The catalog arm of the union | ✓ (daily PigParse job already populates it) | — | empty catalog → union degrades to held-only (no crash) |
| Go toolchain (backend module) | Build | ✓ `[ASSUMED — same toolchain as repo CI]` | 1.24 | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** If `pigparse_price` is somehow empty at crawl time, the union
collapses to held-only (today's behavior) — no error.

## Security Domain

> `security_enforcement: true`, ASVS level 1, block-on `high`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface added (a background job). |
| V3 Session Management | no | — |
| V4 Access Control | no | Read-only enrichment; the D-03 diagnostic is logs, not a user surface. |
| V5 Input Validation | yes | Untrusted wiki text (names, statsblock, flags) is bound through `?` placeholders ONLY in `UpsertItemMasterTx` — NEVER string-concatenated into SQL. `parseIconID`/`parseHastePct` coerce wiki values to non-negative ints (no untrusted string reaches the `Item_<int>.png` URL). `[VERIFIED: store/enrich.go:234-247; enrich/wikiitem.go:584-601,624-638]` |
| V6 Cryptography | no | `crypto/sha1` is a content fingerprint (change detection), NOT a security hash — already documented. `[VERIFIED: enrich/wikiitem.go:25-31]` |
| V7 Error/Log handling | yes | `slog` logs counts/ids/err only, never raw page/flag content. The D-03 residue sample logs item NAMES (already-public wiki page titles) — acceptable; do NOT log statsblock/wikitext bodies. `[VERIFIED: store/enrich.go:22-23; backfill.go:16-17]` |

### Known Threat Patterns for {Go backend + MediaWiki/PigParse ingest}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via parsed item name / flags | Tampering | `?` placeholders only (already enforced). `[VERIFIED: store/enrich.go:234-247]` |
| Untrusted icon id reaching an image URL | Tampering | `parseIconID` → non-negative int sentinel-0; type-safe `Item_<int>.png`. `[VERIFIED: enrich/wikiitem.go:584-601]` |
| Log injection / PII in logs | Info disclosure | Counts/ids/err + bounded public page names only; no body content. `[VERIFIED: store/enrich.go:22-23]` |
| Crawl DoS on the wiki (rude crawler) | DoS (against a third party) | 1s ctx-aware courtesy sleep per fetch + ETag 304s; capless but never faster than 1 req/s. `[VERIFIED: jobs/wiki.go:49-51,168-172]` |
| Resource exhaustion from a giant residue log | DoS (self) | Bound the residue sample slice in the D-03 line (e.g. ≤50 names). (Planner control.) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The catalog is ~4,341 rows and PigParse ids are distinct per catalog row. | Runtime math / D-04 | Low — runtime estimate scales linearly; the union still dedups by name. Verify with `SELECT count(*) FROM pigparse_price`. |
| A2 | A PigParse id numerically equal to a held EQ id for a *different* item is rare (handled by the EQ-id exclusion). | D-04 / Pitfall 2 | Medium — if common, the exclusion drops those catalog names from enrichment (they stay icon-less, surfaced in the residue). Verify with the prod probe in D-04; if material, the planner adds a name-keyed fallback for the excluded set. |
| A3 | The backend builds under the repo's existing Go 1.24 toolchain with no new module. | Standard Stack / Env | Low — no new dependency is introduced. |
| A4 | Logging public wiki page NAMES in the residue sample is acceptable (they are not PII). | Security V7 | Low — page titles are public wiki content; statsblock/wikitext bodies are NOT logged. |

## Open Questions

1. **Exact residue sample bound for D-03.**
   - What we know: D-03 wants "residue item names (or a bounded sample)".
   - What's unclear: the cap (50? 100?) and whether to log a count + sample or full list.
   - Recommendation: log full counts + a capped sample (≤50) to keep the line greppable; the
     maintainer can re-run/query if they need the full list (deferred Admin panel covers richer needs).

2. **Whether to drop EQ-id-colliding catalog names entirely or name-key them.**
   - What we know: Option A's exclusion drops a catalog name whose PigParse id collides with a held EQ id.
   - What's unclear: how many such rows exist in prod (A2).
   - Recommendation: run the D-04 probe during planning; if the count is ~0 (expected), the exclusion
     is harmless. If material, escalate to a name-keyed fallback for just that set (still no new table).

## Sources

### Primary (HIGH confidence — codebase, verified this session)
- `internal/backendsrv/enrich/jobs/wiki.go` — `RunWiki`/`runWikiItems`, 1s `wikiSleepFn`, ETag 304, log-and-skip, no-cap header, upsert/freshness short-circuit.
- `internal/backendsrv/store/itemids.go` — `DistinctInventoryItemIDs`, `ItemRef`, the per-name politeness rule.
- `internal/backendsrv/store/enrich.go` — `UpsertItemMasterTx`, `GetItemMasterFreshnessTx`, `itemMasterUpsert` (all 19 columns), `MarshalFlags`.
- `internal/backendsrv/store/readviews.go` — `pp_rep` CTE, `inventoryJoinBase`, `PriceByName`, `ItemMasterIconStats`.
- `internal/backendsrv/store/itemsearch.go` — `SearchCatalog` (the Phase 19 catalog search over `pigparse_price`).
- `internal/backendsrv/store/backfill.go` + `cmd/squirebot-server/main.go:228-239` — the Phase 37 boot backfill pattern (and why icons don't need one).
- `internal/backendsrv/compute/itemrollup.go` / `view.go` — the normalized-name dedup the rollup uses (the SEARCH-06 reuse target).
- `internal/backendsrv/enrich/wikiitem.go` — `ParseItempage`, `parseIconID`, `DeriveFlagsAndEffects`.
- `internal/backendsrv/migrations/00001_init.sql` / `00012_item_icon.sql` / `00016_item_flags_effects.sql` — the two id namespaces, the icon/freshness precedent, the latest migration (00016).
- `internal/backendsrv/scheduler/scheduler.go` — `dueWiki` (Sunday-once cadence).
- `.planning/config.json` — `nyquist_validation:false` (Validation Architecture omitted), `security_enforcement:true` ASVS 1.

### Secondary (MEDIUM)
- `.planning/phases/38-CONTEXT.md`, `37-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` — locked decisions + requirement wording.
- Memory: `pigparse-vs-ingame-item-id-namespaces`, `pigparse-server-numbering-blue-is-1`, `project_consolidated_views`.

### Tertiary (LOW)
- None — no unverified web claims were needed (pure internal phase).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; all packages read and verified.
- Architecture / D-04: HIGH — both id namespaces, the upsert keying, and the readers' id-usage verified directly; Option A's zero-blast-radius claim is verified against the three held readers.
- Pitfalls: HIGH — each maps to a verified resilience mechanism already in `wiki.go` or a verified namespace fact.
- The one residual risk (A2, EQ-id-vs-PigParse-id numeric collision) is flagged with a concrete prod probe for planning.

**Research date:** 2026-06-25
**Valid until:** 2026-07-25 (stable internal codebase; revalidate if `wiki.go`/`enrich.go`/`item_master` schema changes before planning).
