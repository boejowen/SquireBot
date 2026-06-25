# Phase 38: Catalog-wide enrichment + icon coverage - Context

**Gathered:** 2026-06-25
**Status:** RE-PLANNING — D-04 reversed to name-keyed 2026-06-25 (Option A shipped but the
pre-deploy prod probe found 43 dropped catalog items vs. research's ≈0; deploy held). See
the ⚠ D-04 REVERSED block under Implementation Decisions.

<domain>
## Phase Boundary

Widen item enrichment from "only items some character currently HOLDS" (today's
`store.DistinctInventoryItemIDs` gate, ~hundreds of pages) to the **full PigParse
Blue catalog** (`pigparse_price`, ~4,341 rows), and backfill the wiki icon for
every catalog item whose wiki page provides one — with a maintainer-visible
diagnostic for the items that remain icon-less. This is the "not all items have
images" bug AND the full-catalog data the Phase 39 search-scope toggle (SEARCH-06,
"what exists") reads.

**In scope:** ENRICH-14 (enrichment covers the full PigParse Blue catalog, not only
held items — politefetch-paced), ENRICH-15 (icon coverage backfilled for every item
whose wiki page provides one + a maintainer can see which items are still icon-less).

**Out of scope (later phases / other work):** the search UI/facets that READ this
data (SEARCH-04/05/06 = Phase 39); flag outlines (Phase 40); the colored-tile
fallback's visual treatment (already shipped in Phase 31, stays as-is for genuinely
icon-less items); any watcher change (ingest already captures everything). The
flag/effect parse itself already landed in Phase 37 — this phase only widens the
SET of items that parse runs over + the icon backfill.
</domain>

<decisions>
## Implementation Decisions

### Crawl cadence & control (ENRICH-14)
- **D-01:** **Seed once, then the weekly pass re-validates the WHOLE catalog via
  ETag.** The first wiki run after deploy does the full paced crawl over all ~4,341
  catalog items (~70+ min, 1 req/s — one Sunday). Every weekly pass thereafter sends
  `If-None-Match` for ALL catalog items: unchanged pages 304 cheaply (still 1s-spaced
  per the existing courtesy sleep), changed pages re-parse. Rationale: the weekly job
  has **no execution cap** (`wiki.go` header comment — "a backend job has NO execution
  cap, so the weekly scrape is one uninterrupted pass"), so a long background run is
  harmless; this is the simplest, always-self-healing model and stays courteous at 1
  req/s. Rejected: "weekly re-checks held-only + unheld on a slow rotation" (more
  moving parts, unheld wiki edits land slowly) and "throttle initial crawl across
  days" (day-boundary bookkeeping the capless job doesn't need).
- **D-01a:** The crawl runs **automatically as part of the existing weekly wiki job**
  (no new manual admin trigger this phase). The existing job-level 1s `wikiSleepFn`
  courtesy sleep + ETag 304 short-circuit + log-and-skip-one-bad-page resilience all
  apply unchanged to the widened set. A maintainer-triggered re-crawl button is a
  possible later add, not in scope.

### Catalog scope (ENRICH-14)
- **D-02:** **"Full catalog" = the PigParse auction catalog only** (`pigparse_price`,
  ~4,341 items that have actually been auctioned on Blue). Matches ENRICH-14's literal
  wording, the rows carry prices, and it is the natural "what can I get" set for
  SEARCH-06. **Do NOT** union never-auctioned no-drop/raid items from `wiki_gear_tier`
  into the catalog crawl — those carry no price, bloat the crawl with un-buyable items,
  and already surface via the Wishlist gear-tier suggestions (Phase 34 / Phase 42).

### Coverage diagnostic (ENRICH-15)
- **D-03:** **A per-run structured `slog` summary line** is the maintainer-visible
  surface for "which items are still icon-less / un-enriched." The widened wiki job
  logs (at minimum) total catalog size, count enriched, count icon-covered, and the
  icon-less / un-enriched count — and SHOULD include the residue item names (or a
  bounded sample) so the maintainer can act. Rationale: the maintainer already greps
  `slog` JSON on the VPS (the established ops pattern, CLAUDE.md structured-logging
  convention); zero new UI surface, lowest cost. Rejected for this phase: an officer
  Admin "Item coverage" panel and a queryable/downloadable report (both add a new
  read-API + surface — possible later, not needed to satisfy ENRICH-15's "a maintainer
  can see").

### Claude's Discretion (implementation — planner/executor decide)
- **D-04 — FLAGGED FOR RESEARCH (the load-bearing technical risk):** how to STORE
  enrichment for catalog-only (unheld) items and how SEARCH-06 reconciles held vs.
  unheld without name-duplication. The tension: `item_master` is keyed by
  `item_id INTEGER PRIMARY KEY` in the **EQ-inventory** namespace; the PigParse
  catalog's `pigparse_price.item_id` is a **DIFFERENT namespace** (per the
  `pigparse-vs-ingame-item-id-namespaces` memory — catalog↔inventory join is by
  **normalized name**, NEVER raw item_id). So a catalog-only item has no EQ-inventory
  item_id to key an `item_master` row by, and reusing the PigParse id as the
  `item_master` PK would COLLIDE with real EQ inventory ids. The user DELEGATED this to
  research + planning. The researcher/planner MUST resolve, against the codebase:
  1. The storage/identity model for unheld-item enrichment — candidate options to
     evaluate: (a) a name-keyed enrichment store / separate `catalog_enrichment`-style
     table joined by normalized name; (b) extend `item_master` to admit catalog rows
     under a synthetic/namespaced id; (c) some hybrid. Must preserve the extend-only
     rule + the existing held-item `item_master` keying that Phases 31/32/37 rely on.
  2. The **name-dedup rule** so SEARCH-06's full-catalog scope shows each item ONCE
     (a held item and its catalog row are the SAME item by normalized name — they must
     merge, not appear twice).
  3. Whether the wiki fetch loop iterates a UNION of held `DistinctInventoryItemIDs`
     + the catalog, deduped by normalized name (so an item is fetched once whether held
     or catalog-only), and how that interacts with the per-item `item_id` the upsert
     needs.

> **⚠ D-04 REVERSED → NAME-KEYED (ratified 2026-06-25). The original delegated
> research picked id-keyed Option A; it is now REJECTED. Re-plan MUST be name-keyed.**
>
> **What happened:** Phase 38 first shipped (code-complete, verifier 5/5, code-review
> 0-blocker — NOT deployed) with **Option A: admit catalog-only rows into `item_master`
> keyed by their PigParse `item_id`, with an `item_id NOT IN (SELECT item_id FROM
> item_master)` collision guard** so a PigParse id numerically equal to a held EQ id
> could never overwrite the held row. The research `[ASSUMED]` (A2) that such collisions
> were "rare … ≈0" and that excluding them was harmless.
>
> **The pre-deploy prod probe disproved that ≈0.** Against the live DB (`item_master`
> = 953 rows, `pigparse_price` = 4,343): **60 raw PigParse↔EQ id collisions**, of which
> **43 are genuinely-unheld catalog items** the Option-A guard silently DROPS from
> enrichment (the held item they numerically collide with is correctly protected — the
> guard works; but those 43 real catalog items, e.g. *Cured Silk Gi*, *Ancient Tarnished
> Breastplate*, *Etched Velium Brawl Stick*, get NO icon/flags). ~1% of the catalog, no
> correctness risk, but a permanent coverage hole that the icon-less residue can never
> close. The plan's own STOP-at-~20 gate fired; the finding was surfaced and the user
> chose to **hold the deploy and re-plan name-keyed** rather than ship the gap.
>
> **NEW CONSTRAINT for the re-plan (binding — researcher + planner):**
> - Catalog enrichment MUST be stored **keyed by normalized name** (`lower(trim(name))`)
>   so **ALL ~4,343 catalog items — including the 43 — are covered with NO collision
>   drop**. The id-keyed-`item_master` storage of catalog-only rows (Option A) and any
>   synthetic/offset-id scheme are REJECTED.
> - Adopt the **Option B family** from `38-RESEARCH.md`: a separate name-keyed
>   enrichment store (e.g. `catalog_enrichment(norm_name TEXT PRIMARY KEY, icon_id,
>   statsblock, flags_json, is_clicky, has_haste, …)`) joined by normalized name;
>   `item_master` stays **held-only, keyed by EQ `item_id`** exactly as today. Migration
>   footprint → **00017** (new additive table; extend-only; goose-on-boot; no `v*` tag).
> - **Re-verify the held-reader blast radius is ZERO.** Phases 31/32/37 read
>   `item_master` by EQ `item_id`; held items stay in `item_master`, so those readers
>   MUST be byte-for-byte unaffected. The name-keyed `catalog_enrichment` holds ONLY
>   unheld items — no existing held reader needs it.
> - **Define the Phase-39 catalog-scope read contract:** "what exists" = held
>   (`item_master` by EQ id) ∪ unheld (`catalog_enrichment` by name), COALESCE'd /
>   deduped by normalized name so each item appears ONCE and a held item keeps its
>   holders (catalog = superset). This is the join Phase 39's facets read — get it right
>   here (it also makes Phase 39's catalog↔enrichment facet join name-keyed throughout,
>   removing the namespace-bridge hazard).
> - The crawl still iterates the held∪catalog-by-name set (the existing
>   `store.DistinctEnrichmentRefs` union is reusable for the FETCH set), but the
>   **WRITE path branches by held-ness**: held name → `item_master` by EQ id (today's
>   `UpsertItemMasterTx`); catalog-only name → `catalog_enrichment` by norm_name (new
>   upsert + a per-name freshness getter for the weekly ETag re-validation). Drop the
>   Option-A `item_id NOT IN (SELECT item_id FROM item_master)` collision guard — it is
>   no longer needed once catalog rows are name-keyed in a separate table.
> - Keep everything else from D-01/D-01a/D-02/D-03 (cadence, catalog scope, the `slog`
>   coverage diagnostic). The `ItemMasterIconCoverage` diagnostic must be re-pointed to
>   read the name-keyed store's true coverage (it must reflect the catalog_enrichment
>   rows, not just `item_master`).
- Migration mechanics (next number after 00016 → **00017** if a new table/column is
  needed; may be none if a name-keyed read suffices); whether `WatcherMaxSchemaVersion`
  needs a bump (backend-only, watcher off the read path — almost certainly not; confirm
  against CLAUDE.md's schema-evolution rule, noting `internal/sheet` no longer exists
  post-v2.0 so that specific constant may be moot).
- The icon backfill itself reuses the Phase 37 freshness short-circuit pattern
  (`GetItemMasterFreshnessTx` already compares `icon_id`; 00012 established
  "parse one field, nullable column, add to freshness, backfill on next pass").
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements / roadmap
- `.planning/REQUIREMENTS.md` — ENRICH-14 (catalog-wide coverage), ENRICH-15 (icon
  backfill + residue visibility); the "Item Enrichment Backbone (ENRICH)" intro.
- `.planning/ROADMAP.md` — Phase 38 line in the v2.6 checklist (the "missing item
  images" bug + the full-catalog data SEARCH-06 reads) + the milestone sequencing
  insight ("P37 backbone → P38 catalog coverage → P39 faceted search").
- `.planning/phases/37-item-enrichment-backbone-flags-effects/37-CONTEXT.md` — the
  immediately-prior phase; D-05/D-06 (immediate backfill + freshness self-heal) are the
  pattern this phase extends to the icon + the widened item set.

### The crawl loop being widened (the core change)
- `internal/backendsrv/enrich/jobs/wiki.go` — `RunWiki` / `runWikiItems` (the
  weekly items pass; `:160-214`); the 1s `wikiSleepFn` courtesy sleep + ETag 304
  short-circuit + log-and-skip-one-page resilience; `upsertItemAndQuests` (the
  per-item upsert + `GetItemMasterFreshnessTx` SHA-1/icon/statsblock/flags
  short-circuit, `:216-291`). **The "no execution cap" header comment (`:3-31`) is the
  justification for D-01.**
- `internal/backendsrv/store/itemids.go` — `DistinctInventoryItemIDs` + the `ItemRef`
  (item_id, name) shape (`:30-64`); the GROUP-BY-item_id "fetch each page once,
  MIN(name) representative, never refetch" politeness rule the widened crawl MUST
  preserve. **This is the gate Phase 38 widens** (held-union → held∪catalog by name).

### Storage + namespaces (the D-04 research surface)
- `internal/backendsrv/migrations/00001_init.sql` — the `item_master`
  (`item_id INTEGER PRIMARY KEY`, EQ-inventory namespace) + `pigparse_price`
  (`item_id INTEGER PRIMARY KEY`, PigParse namespace) table definitions: the two
  DIFFERENT id namespaces at the heart of D-04.
- `internal/backendsrv/migrations/00012_item_icon.sql` — the `icon_id` nullable
  ADD COLUMN + the "NULL surfaces as 0 → colored-tile fallback" contract + the
  freshness-comparison precedent the icon backfill (ENRICH-15) repeats.
- `internal/backendsrv/store/enrich.go` — `UpsertItemMasterTx` +
  `GetItemMasterFreshnessTx` (the freshness short-circuit, extended in Phase 37 with
  flags_json; the icon already participates).
- The normalized-name join precedent (Phase 29 DATA-01): how `pigparse_price` joins to
  inventory/items by normalized name today — the planner reuses this for the
  catalog↔held dedup (D-04.2). (Search `internal/backendsrv/compute` for the
  name-normalization helper used by the price join.)

### Conventions
- `CLAUDE.md` — schema-evolution rule (extend-only, version-stamped, idempotent;
  note the `internal/sheet`/`WatcherMaxSchemaVersion` reference is stale post-v2.0) +
  the structured-logging convention (`slog` Go side) the D-03 diagnostic follows.

### Memory (cross-session facts)
- `pigparse-vs-ingame-item-id-namespaces` — catalog (`pigparse_price`) ids ≠ EQ
  inventory ids; join by normalized name, never raw item_id. **Load-bearing for D-04.**
- `pigparse-server-numbering-blue-is-1` — Blue = server 1 for getall (the catalog is
  the Blue catalog).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **The whole `runWikiItems` loop** (`wiki.go`) — fetch-by-name → ETag 304 →
  `ParseItempage` → `upsertItemAndQuests` already does everything per item; Phase 38
  changes only the SET of refs it iterates (and lets the icon land for all of them).
- **`GetItemMasterFreshnessTx` + the 00012 icon pattern** — "nullable column, add to
  freshness, backfill on next pass" is the exact shape ENRICH-15's icon backfill
  repeats; the icon is ALREADY in the freshness comparison, so widening the item set is
  what newly populates icons for unheld items.
- **The Phase 29 normalized-name price join** — the established held↔catalog name join;
  reuse its normalization for the D-04 dedup so SEARCH-06 shows one row per item.

### Established Patterns
- **politefetch + 1s job-level courtesy sleep + log-and-skip-one-bad-page** — already
  in `wiki.go`; the widened crawl inherits all of it unchanged (D-01a). A single bad
  catalog page must never abort the run.
- **ETag 304 short-circuit** — the mechanism that makes D-01's "weekly re-validate the
  whole catalog" cheap on unchanged pages.
- **Extend-only / goose-on-boot** — any new table/column is the next number after 00016
  (→ 00017), additive, no watcher change, no `v*` tag.

### Integration Points
- Phase 39 (SEARCH-04/05/06) READS this phase's output: the parsed flags/effects
  (Phase 37) over the WIDENED item set (Phase 38) + the held-vs-catalog scope toggle.
  The D-04 dedup rule is the contract Phase 39's full-catalog scope depends on — get it
  right here.
- The icon backfill feeds the existing client colored-tile-vs-wiki-icon render
  (Phase 31) — no client change needed; more items simply resolve a real icon.
</code_context>

<specifics>
## Specific Ideas

- The bug is literally "not all items have images" — the win condition is that an
  item you can SEE in search/inventory shows its real wiki icon wherever the wiki
  actually provides one; the colored tile stays only for items with genuinely no wiki
  icon, and the maintainer can now SEE that residue (the D-03 log summary).
- Keep the crawl courteous (1 req/s) over correctness-of-runtime — a 70-minute Sunday
  background run is explicitly fine; do not add complexity to make it shorter.
</specifics>

<deferred>
## Deferred Ideas

- **Maintainer-triggered re-crawl button** (manual kickoff of the full catalog crawl
  from the Admin panel) — not needed for ENRICH-14; the automatic weekly job suffices.
- **Officer Admin "Item coverage" panel / queryable coverage report** — richer
  surfaces for the icon-less residue than the D-03 log line; revisit only if the log
  summary proves insufficient.
- Search facets beyond Clicky/Haste and the SEARCH-06 UI itself — Phase 39, not here.

</deferred>

---

*Phase: 38-catalog-wide-enrichment-icon-coverage*
*Context gathered: 2026-06-25*
