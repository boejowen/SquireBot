# Phase 38: Catalog-wide enrichment + icon coverage - Research

**Researched:** 2026-06-25 (RE-PLAN — D-04 reversed id-keyed → NAME-KEYED)
**Domain:** Go backend — widen the weekly wiki enrichment loop from held-items to the full PigParse Blue catalog; store catalog-only enrichment in a NEW name-keyed table; backfill icons; emit a coverage diagnostic across both stores
**Confidence:** HIGH (everything verified against the codebase; no external/network claims)

> **RE-PLAN NOTICE.** Phase 38 first shipped (code-complete, verifier 5/5, code-review 0-blocker — **NEVER deployed**) with **id-keyed Option A**: catalog-only rows admitted into `item_master` keyed by their PigParse `item_id` behind an `item_id NOT IN (SELECT item_id FROM item_master)` collision guard. The pre-deploy prod probe disproved the research's `[ASSUMED]` "≈0 collisions" (A2): against the live DB (`item_master`=953, `pigparse_price`=4,343) there are **60 raw PigParse↔EQ id collisions / 43 genuinely-unheld catalog items the guard silently DROPS** (e.g. *Cured Silk Gi*, *Ancient Tarnished Breastplate*, *Etched Velium Brawl Stick* — ~1% of the catalog, a permanent coverage hole the icon-less residue can never close). The user **HELD the deploy and RATIFIED reversing D-04 to NAME-KEYED** (committed 722edbc / CONTEXT ⚠ block). This research **REPLACES the prior recommendation A→B**: every verified codebase fact below is reused, but the storage model is now a separate name-keyed `catalog_enrichment` table covering all 4,343 with NO drop. The existing Option-A code (`store.DistinctEnrichmentRefs` with its collision guard, the unbranched `runWikiItems` write, `ItemMasterIconCoverage`) will be **REPLACED by the re-plan executor**.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Seed once, then the weekly pass re-validates the WHOLE catalog via ETag. First wiki run after deploy does the full paced crawl over all ~4,341 catalog items (~70+ min, 1 req/s — one Sunday). Every weekly pass thereafter sends `If-None-Match` for ALL catalog items: unchanged pages 304 cheaply (still 1s-spaced), changed pages re-parse. The weekly job has NO execution cap (`wiki.go` header comment). Simplest, always-self-healing, courteous at 1 req/s. Rejected: "weekly re-checks held-only + unheld on a slow rotation" and "throttle initial crawl across days."
- **D-01a:** The crawl runs automatically as part of the existing weekly wiki job (no new manual admin trigger). The existing job-level 1s `wikiSleepFn` + ETag 304 short-circuit + log-and-skip-one-bad-page resilience all apply unchanged to the widened set.
- **D-02:** "Full catalog" = the PigParse auction catalog only (`pigparse_price`, ~4,341 rows). Do NOT union never-auctioned `wiki_gear_tier` items.
- **D-03:** A per-run structured `slog` summary line is the maintainer-visible coverage surface (total / enriched / icon-covered / icon-less + residue names or a bounded sample). No new UI surface. Rejected for this phase: an officer Admin "Item coverage" panel and a queryable/downloadable report.
- **D-04 — REVERSED → NAME-KEYED (ratified 2026-06-25, binding).** The original id-keyed Option A is REJECTED. Catalog enrichment MUST be stored keyed by normalized name (`lower(trim(name))`) so ALL ~4,343 catalog items — including the 43 the Option-A guard dropped — are covered with NO collision drop. Adopt **Option B**: a separate name-keyed `catalog_enrichment` table; `item_master` stays held-only, keyed by EQ `item_id`, unchanged. Re-verify the held-reader blast radius is ZERO. Define the Phase-39 read contract: "what exists" = held(`item_master` by EQ id) ∪ unheld(`catalog_enrichment` by name), COALESCE'd/deduped by normalized name. The crawl still iterates the held∪catalog-by-name set, but the WRITE path BRANCHES by held-ness; DROP the Option-A `item_id NOT IN (…)` collision guard. Migration footprint → **00017** (new additive table; extend-only; goose-on-boot; no `v*` tag). See `## D-04 RESOLUTION` below.

### Claude's Discretion
- Migration mechanics (next number after 00016 → **00017** — a new table IS now needed under name-keyed Option B); whether `WatcherMaxSchemaVersion` needs a bump (backend-only, watcher off the read path — NO; `internal/sheet` no longer exists post-v2.0 so that constant is moot).
- The icon backfill reuses the Phase 37 freshness short-circuit pattern (`GetItemMasterFreshnessTx` already compares `icon_id`; the new `catalog_enrichment` parallel freshness getter mirrors it; 00012 established "parse one field, nullable column, add to freshness, backfill on next pass").

### Deferred Ideas (OUT OF SCOPE)
- Maintainer-triggered re-crawl button (manual kickoff from the Admin panel).
- Officer Admin "Item coverage" panel / queryable coverage report.
- Search facets beyond Clicky/Haste and the SEARCH-06 UI itself — Phase 39, not here.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ENRICH-14 | Item enrichment covers the full PigParse Blue catalog, not only held items — politefetch-paced. | Widen the loop in `runWikiItems` to iterate the held∪catalog union (`DistinctEnrichmentRefs`, with the collision guard DROPPED and a `held` flag ADDED). Catalog-only refs now write to a NEW name-keyed `catalog_enrichment` table — so all 4,343 items (incl. the 43 formerly dropped) are covered (`## D-04 RESOLUTION`). Runtime/courtesy math unchanged (`## Crawl Runtime + Resilience Math`). The existing 1s `wikiSleepFn` + ETag 304 + log-and-skip resilience carry over unchanged. |
| ENRICH-15 | Icon coverage backfilled for every item whose wiki page provides one + a maintainer can see which items are still icon-less. | The icon is ALREADY in `GetItemMasterFreshnessTx`'s comparison (held) and is mirrored in the new `GetCatalogEnrichmentFreshnessTx` (catalog) — widening the set + branching the write newly populates icons for unheld items with NO new parse logic. `parseIconID` already extracts `lucy_img_ID`. The residue diagnostic now reads BOTH stores (`## D-03 Coverage Diagnostic — across both stores`). "Icon-less" classification in `## Icon-Backfill Mechanics`. |
</phase_requirements>

## Summary

Phase 38 is a **loop-widening + storage-identity** phase, not a parser phase. The wiki item parser
(`enrich/wikiitem.go`), the per-item upsert (`store/enrich.go::UpsertItemMasterTx`), the freshness
short-circuit (`GetItemMasterFreshnessTx` — already compares `icon_id`), the 1s courtesy sleep, the
ETag 304 short-circuit, and the log-and-skip-one-bad-page resilience all already exist and already do
everything per HELD item. ENRICH-14 widens the SET of refs `runWikiItems` iterates from held-only
(`DistinctInventoryItemIDs`, ~hundreds) to the **held ∪ full PigParse catalog** (~4,343). Because the
icon already participates in the freshness comparison, ENRICH-15 (icon backfill) falls out of the
crawl for free — fetching an item's page is what populates its icon.

The single load-bearing decision is **D-04**, and it has been **REVERSED to NAME-KEYED**. The prod
probe proved the id-keyed model (Option A) silently drops 43 real catalog items whose PigParse
`item_id` numerically collides with a held EQ `item_id` (a genuine namespace overlap, not the "≈0"
the prior research assumed). The reversed model stores catalog-only (unheld) enrichment in a NEW
**`catalog_enrichment` table keyed by `norm_name TEXT PRIMARY KEY`** (migration **00017**), carrying
the same enrichment shape `item_master` holds (icon/statsblock/flags/effects/summary/url/slot/quest +
a representative `name` and PigParse `item_id`). `item_master` stays **held-only, keyed by EQ
`item_id`, byte-for-byte unchanged** — so every held reader (Phases 31/32/37) is provably
unaffected (verified zero blast radius). The crawl iterates the same held∪catalog-by-name union, but
the WRITE path **branches**: a held ref → today's `UpsertItemMasterTx` by EQ id; a catalog-only ref →
a NEW `UpsertCatalogEnrichmentTx(norm_name, …)` with a parallel `GetCatalogEnrichmentFreshnessTx` for
the weekly ETag re-validation. The Option-A `item_id NOT IN (SELECT item_id FROM item_master)`
collision guard is **DROPPED** — name-keying in a separate table removes the collision entirely, so
all 4,343 (incl. the 43) are covered with no drop. SEARCH-06 (Phase 39) reads
held(`item_master` by EQ id) ∪ unheld(`catalog_enrichment` by name), COALESCE'd by `lower(trim(name))`
so each item appears once and a held item keeps its holders — making Phase 39's catalog↔enrichment
facet join name-keyed end-to-end and dissolving the namespace-bridge hazard 39-CONTEXT flagged.

**Primary recommendation (Option B — name-keyed):** Add migration **00017** creating
`catalog_enrichment(norm_name TEXT PRIMARY KEY, …)`. Change `DistinctEnrichmentRefs` to (a) DROP the
`item_id NOT IN (item_master)` guard and (b) carry a `Held bool` per ref. Branch the write in
`runWikiItems`: held → `UpsertItemMasterTx`+`GetItemMasterFreshnessTx` (unchanged), catalog-only →
new `UpsertCatalogEnrichmentTx`+`GetCatalogEnrichmentFreshnessTx`. Replace `ItemMasterIconCoverage`
with a coverage read that UNIONs both stores. NO watcher change, no `v*` tag.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Widen the enrichment ref set (held ∪ catalog by name, with a `Held` flag) | API/Backend — `store` read method (`DistinctEnrichmentRefs`) | Enrichment job (`enrich/jobs/wiki.go`) | The dedup-by-name union is a SELECT over `inventory_item` + `pigparse_price`; lives in `store/` per the single-tested-SQL-path rule. |
| Fetch + parse each item's wiki page | Enrichment job (`enrich/jobs/wiki.go`) | Pure parser (`enrich/wikiitem.go`) | Already the established pattern; unchanged per item. |
| Store enrichment for HELD items | Database (`item_master`, EQ-id PK) | — | Unchanged. `UpsertItemMasterTx` / `GetItemMasterFreshnessTx`. |
| Store enrichment for CATALOG-ONLY items | Database (NEW `catalog_enrichment`, `norm_name` PK) | `store` (new upsert + freshness getter) | D-04 name-keyed: no EQ id exists for an unheld item; the name is the only stable key. Migration 00017. |
| Branch the write by held-ness | Enrichment job (`runWikiItems`) | `store` (two write paths) | Held → item_master by EQ id; catalog-only → catalog_enrichment by name. |
| Icon backfill | Enrichment job (freshness short-circuit, both stores) | Database (`*.icon_id`) | Icon already in `GetItemMasterFreshnessTx`; mirrored in the new catalog freshness getter; the crawl is the mechanism. |
| Coverage diagnostic | Enrichment job (`slog`) + `store` (two-store coverage read) | — | D-03: structured log line over BOTH stores' true coverage; no UI/read-API. |
| SEARCH-06 held-vs-catalog reconciliation (downstream) | API/Backend read (Phase 39) | compute (normalized-name COALESCE) | Phase 39 reads held(item_master by id) ∪ unheld(catalog_enrichment by name); the dedup contract is set HERE. |

## D-04 RESOLUTION (the load-bearing section) — NAME-KEYED (Option B)

### The tension, restated against the code

- `item_master` is `CREATE TABLE item_master (item_id INTEGER PRIMARY KEY, …)` — keyed in the **EQ-inventory**
  namespace (the `/outputfile` `ID` column the watcher uploads). `[VERIFIED: migrations/00001_init.sql:59]`
- `pigparse_price` is `CREATE TABLE pigparse_price (item_id INTEGER PRIMARY KEY, …)` — keyed in the
  **PigParse catalog** namespace. `[VERIFIED: migrations/00001_init.sql:60]`
- These two `item_id` spaces are DIFFERENT and **numerically overlap**. The price/enrichment bridge
  is `lower(trim(name))`, NEVER raw `item_id` — every held reader's price join uses the `pp_rep` CTE
  by normalized name for exactly this reason. `[VERIFIED: store/readviews.go:36-47,287-302; itemrollup.go:16-21; memory pigparse-vs-ingame-item-id-namespaces]`
- Today's shipped (Option-A) `runWikiItems` iterates `DistinctEnrichmentRefs` and upserts EVERY ref into
  `item_master` keyed by `ref.ItemID` — for a catalog-only ref that id is the PigParse id.
  `[VERIFIED: store/itemids.go:100-142; jobs/wiki.go:164,288-289]`

### Why id-keyed (Option A) is REJECTED — the prod probe

A catalog-only item has **no EQ-inventory item_id**. Option A keyed its `item_master` row by the
PigParse id and guarded against overwriting a held row with
`item_id NOT IN (SELECT item_id FROM item_master)` `[VERIFIED: store/itemids.go:90-92,118]`. That guard
is *correct* — it protects the held row — but it **drops** the catalog item entirely when the PigParse
id collides. The prior research `[ASSUMED]` (A2) such collisions were "≈0". The pre-deploy prod probe
disproved it: live `item_master`=953, `pigparse_price`=4,343 → **60 raw id collisions, 43 genuinely-unheld
catalog items dropped** (e.g. *Cured Silk Gi*, *Ancient Tarnished Breastplate*, *Etched Velium Brawl
Stick*). ~1% of the catalog, no correctness risk, but a **permanent coverage hole** — the icon-less
residue can never close it because those names never reach a write. Any synthetic/offset-id scheme is
also rejected (it leaks a fake id into `item_master.item_id`, which held readers treat as a real EQ id).
`[CITED: 38-CONTEXT.md ⚠ D-04 REVERSED; STATE.md "Phase 38 RE-OPENED"]`

### The reversed model: a separate name-keyed `catalog_enrichment` table

Name-keying in a SEPARATE table removes the collision class entirely: the PK is `norm_name`, not an id,
so two items with numerically-equal ids in different namespaces simply have different names and live in
different rows. All 4,343 catalog items are covered with no drop. `item_master` is untouched, so its
held readers keep their exact EQ-id keying.

**New table (migration 00017) — column set confirmed against `itemMasterUpsert`'s 19 columns
`[VERIFIED: store/enrich.go:204-216]` and what Phase 39 reads `[VERIFIED: store/itemsearch.go:61-98]`:**

```sql
-- +goose Up
-- Phase 38 (ENRICH-14/15, D-04 name-keyed). Forward-only; 00001-00016 are shipped
-- and NOT edited. catalog_enrichment holds wiki enrichment for CATALOG-ONLY (unheld)
-- items, keyed by normalized name lower(trim(name)) — the cross-namespace bridge
-- (PigParse catalog ids != EQ inventory ids; join by name, never raw item_id). A held
-- item's enrichment stays in item_master keyed by its EQ item_id; this table holds ONLY
-- names with no held item_master row. The weekly wiki job's catalog write path upserts
-- here by norm_name; the held write path is unchanged. Read-only additive table: the
-- watcher is OFF the read path (untouched this phase), so NO WatcherMaxSchemaVersion gate
-- is touched (that gate does not exist in the off-Google backend) and goose version() is
-- the version of record. Mirrors item_master's enrichment shape so Phase 39 can COALESCE
-- held(item_master by id) UNION unheld(catalog_enrichment by name) one row per item.
CREATE TABLE catalog_enrichment (
  norm_name      TEXT PRIMARY KEY,             -- lower(trim(name)) — the cross-namespace key
  name           TEXT,                         -- representative display name (first-seen casing)
  item_id        INTEGER,                      -- representative PigParse id (for examine / icon URL); NOT a key
  wiki_summary   TEXT,
  wiki_url       TEXT,
  slot           TEXT,
  is_quest_item  INTEGER NOT NULL DEFAULT 0,
  wikitext_sha1  TEXT,                          -- freshness: re-parse only on change
  icon_id        INTEGER,                       -- lucy_img_ID; NULL/0 = colored-tile fallback (00012 contract)
  statsblock     TEXT,                          -- the cleaned in-game stat block; "" when absent
  is_lore        INTEGER,
  is_no_drop     INTEGER,
  is_magic       INTEGER,
  is_temporary   INTEGER,
  is_clicky      INTEGER,
  clicky_effect  TEXT,
  has_haste      INTEGER,
  haste_pct      INTEGER,
  flags_json     TEXT,                          -- the full detected flag SET as a JSON array (no future migration per flag)
  last_refreshed TEXT
);

-- +goose Down
DROP TABLE catalog_enrichment;
```

This is the SAME enrichment shape `item_master` carries (`item_id, name, wiki_summary, wiki_url, slot,
is_quest_item, wikitext_sha1, last_refreshed, icon_id, statsblock, is_lore, is_no_drop, is_magic,
is_temporary, is_clicky, clicky_effect, has_haste, haste_pct, flags_json` `[VERIFIED: store/enrich.go:204-216]`),
re-PK'd on `norm_name` and adding `name` + `item_id` as non-key representative columns. `flags_json`
carries the full flag set so a future flag needs no further migration (the 00016 precedent
`[VERIFIED: migrations/00016_item_flags_effects.sql:27]`).

### The two new store methods (ZERO inline SQL in the job — 11-05 rule `[VERIFIED: store/enrich.go:3-23]`)

**1. `UpsertCatalogEnrichmentTx(ctx, tx, CatalogEnrichment)`** — mirrors `UpsertItemMasterTx`
`[VERIFIED: store/enrich.go:234-248]` but ON CONFLICT(norm_name):

```go
// in store/enrich.go (or a new store/catalogenrich.go — planner's choice; same package).
// Mirrors itemMasterUpsert exactly, re-keyed on norm_name; every UNTRUSTED wiki value
// (name, summary, slot, clicky_effect, flags_json, statsblock) binds through a ? placeholder
// (V5 / Tampering) — NEVER string-concatenated. b2i for the booleans (same as item_master).
const catalogEnrichmentUpsert = `INSERT INTO catalog_enrichment
    (norm_name, name, item_id, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1,
     icon_id, statsblock, is_lore, is_no_drop, is_magic, is_temporary, is_clicky,
     clicky_effect, has_haste, haste_pct, flags_json, last_refreshed)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(norm_name) DO UPDATE SET
   name=excluded.name, item_id=excluded.item_id, wiki_summary=excluded.wiki_summary,
   wiki_url=excluded.wiki_url, slot=excluded.slot, is_quest_item=excluded.is_quest_item,
   wikitext_sha1=excluded.wikitext_sha1, icon_id=excluded.icon_id, statsblock=excluded.statsblock,
   is_lore=excluded.is_lore, is_no_drop=excluded.is_no_drop, is_magic=excluded.is_magic,
   is_temporary=excluded.is_temporary, is_clicky=excluded.is_clicky,
   clicky_effect=excluded.clicky_effect, has_haste=excluded.has_haste,
   haste_pct=excluded.haste_pct, flags_json=excluded.flags_json, last_refreshed=excluded.last_refreshed`
```

The caller computes `norm_name = lower(trim(item.ItemName))` in Go (matching the
`strings.ToLower(strings.TrimSpace(...))` idiom already used at `store/enrich.go:331` and
`itemrollup.go:69`). `flags_json` is `store.MarshalFlags(item.Flags)` — the ONE canonical encoder, so
a flagless item stores `"[]"` and byte-matches the freshness compare (D-06 idempotency
`[VERIFIED: store/enrich.go:34-60]`).

**2. `GetCatalogEnrichmentFreshnessTx(ctx, tx, normName) (sha string, iconID int64, statsblock,
flagsJSON string, err error)`** — the EXACT parallel of `GetItemMasterFreshnessTx`
`[VERIFIED: store/enrich.go:282-294]`, re-keyed on `norm_name`:

```go
// Returns the stored wikitext_sha1, icon_id, statsblock, flags_json for normName (zero values
// when the row is absent or a column is NULL). The job's catalog write path compares ALL FOUR
// (sha OR icon OR statsblock OR flags_json differs ⇒ re-write) so a row written before icon/stats/
// flags backfills on the next pass — the identical self-heal item_master gets. SHA-1 alone is NOT
// sufficient (same 00012-icon argument, by name).
SELECT wikitext_sha1, icon_id, statsblock, flags_json FROM catalog_enrichment WHERE norm_name = ?
```

The freshness-compare semantics are byte-identical to the held path: the job re-writes whenever
`existingSHA != item.WikitextSHA1 || existingIcon != int64(item.IconID) || existingStats !=
item.Statsblock || existingFlagsJSON != parsedFlagsJSON`. So the weekly ETag re-validation self-heals
catalog rows by name exactly as item_master self-heals held rows by id.

### The branched write path in `runWikiItems`

The loop body stays almost identical `[VERIFIED: jobs/wiki.go:160-234]`; only `upsertItemAndQuests`
gains a branch on the ref's `Held` flag. Held → today's tx (item_master + quest_items). Catalog-only →
a NEW tx that calls `GetCatalogEnrichmentFreshnessTx` then `UpsertCatalogEnrichmentTx`.

**Quest links for catalog-only items:** quest_items is keyed by `item_id` (the EQ namespace) and is
read id-joined `[VERIFIED: store/readviews.go:463-465 QuestLinksByItem; migrations/00001_init.sql:63]`.
A catalog-only item has no EQ id, so writing its quest links under the PigParse id would land in the
EQ-namespace quest_items table where no held reader will ever find them (and could numerically collide).
**Recommendation:** the catalog-only branch SKIPS the quest_items write (parse the page, write
`catalog_enrichment` only). Quest-link surfacing for unheld items is out of scope (ENRICH-14/15 are
icon + flags coverage; quest links are the existing held-item Characters-tab surface). This keeps the
EQ-namespace quest_items table held-only, matching item_master. `[ASSUMED]` that no current requirement
needs quest links for unheld catalog items — confirm against Phase 39 scope (39-CONTEXT facets are
Clicky/Haste, not quests).

### item_master STAYS held-only — held-reader blast radius is ZERO (re-verified)

Every reader of `item_master` joins/looks up by EQ `item_id`, and `item_master` continues to hold ONLY
held EQ-id rows under Option B. None of them read `catalog_enrichment`. Enumerated:

| Reader | Where | How it reads item_master | Affected by Option B? |
|--------|-------|--------------------------|----------------------|
| `ItemMasterIconStats` (P31/32 rollup icon/stats) | `store/readviews.go:770-793` | `SELECT item_id, icon_id, statsblock FROM item_master` → `map[int64]IconStats` by EQ id | **No** — only held EQ-id rows exist; the rollup's `iconStats[vr.ID]` lookup `[VERIFIED: itemrollup.go:72-81]` is id-correct. |
| Inventory view/bank join | `store/readviews.go:299` | `LEFT JOIN item_master im ON im.item_id = ii.item_id` | **No** — joins held EQ ids only; an unheld catalog item is never in `inventory_item`, so never joined. |
| Per-char inventory join | `store/readviews.go:400` | `LEFT JOIN item_master im ON im.item_id = ii.item_id` | **No** — same id-join, held-only. |
| `GetItemMasterFreshnessTx` / `UpsertItemMasterTx` (P37 weekly + boot backfill) | `store/enrich.go:234-294; backfill.go:55-106` | by EQ `item_id` | **No** — the held write path is unchanged; the catalog path uses the new methods. |
| `BackfillItemFlags` boot pass (P37) | `store/backfill.go:32-34` | `SELECT … FROM item_master WHERE flags_json IS NULL` | **No** — scans item_master only; catalog_enrichment is born populated by the crawl (no NULL-flags backfill needed — born with flags from the parse). |

`catalog_enrichment` holds ONLY names with no held `item_master` row (the union's held-name dedup
guarantees it — see below). **Verdict: zero blast radius, verified.**

### The crawl FETCH set + the `Held` flag (revise `DistinctEnrichmentRefs`)

The held∪catalog-by-name union is REUSABLE as the fetch set, with two changes:

1. **DROP** the Option-A catalog-arm guard `item_id NOT IN (SELECT item_id FROM item_master)`
   `[VERIFIED: store/itemids.go:118]` — name-keying removes the collision, so a PigParse id equal to a
   held EQ id no longer matters (different tables, different keys). Removing it is what recovers the 43
   dropped items.
2. **ADD a `Held bool`** to the ref shape so the write path can branch. Two viable shapes (planner picks):
   - extend `ItemRef` with `Held bool` (`SELECT … , 1 AS held` / `0 AS held` per arm), OR
   - a new `EnrichmentRef{ItemID int64; Name string; Held bool}` struct (cleaner; leaves `ItemRef`
     untouched for `DistinctInventoryItemIDs`).
   Recommend the dedicated `EnrichmentRef` struct — `ItemRef` is shared with the held-only path
   `[VERIFIED: store/itemids.go:25-28,40-64]` and adding a field there ripples into unrelated tests.

Keep the held-name dedup (catalog arm `lower(trim(name)) NOT IN (SELECT norm FROM held_names)`) — a
name that is BOTH held and in the catalog is fetched ONCE and written to item_master (held wins), never
to catalog_enrichment, preserving the "one wiki page per normalized name" politeness
`[VERIFIED: store/itemids.go:87-89,108-117]`. Keep the catalog arm's `GROUP BY lower(trim(name))` rep
pick and the blank-name guard `[VERIFIED: store/itemids.go:113-119]`.

**Revised union sketch** (the held arm unchanged; catalog arm drops one exclusion, both arms emit `held`):

```sql
WITH held AS (
  SELECT item_id, MIN(name) AS name
  FROM inventory_item
  WHERE item_id IS NOT NULL AND item_id > 0
  GROUP BY item_id
),
held_names AS (
  SELECT DISTINCT lower(trim(name)) AS norm
  FROM inventory_item WHERE item_id IS NOT NULL AND item_id > 0
),
catalog AS (
  SELECT MIN(item_id) AS item_id, MIN(name) AS name
  FROM pigparse_price
  WHERE name IS NOT NULL AND trim(name) <> ''
    AND lower(trim(name)) NOT IN (SELECT norm FROM held_names)
    -- DROPPED (Option A): AND item_id NOT IN (SELECT item_id FROM item_master)
  GROUP BY lower(trim(name))
)
SELECT item_id, name, 1 AS held FROM held
UNION ALL
SELECT item_id, name, 0 AS held FROM catalog
ORDER BY item_id;
```

### The Phase-39 read contract (downstream — set HERE)

"What exists" for SEARCH-06 = held(`item_master` by EQ id) ∪ unheld(`catalog_enrichment` by name),
COALESCE'd/deduped by `lower(trim(name))` so each item appears ONCE and a held item keeps its holders
(catalog = superset). Phase 39's facet read (Clicky/Haste) resolves `is_clicky`/`has_haste` from
`item_master` by id for held names and from `catalog_enrichment` by name for unheld names. Shape Phase 39
will read (the join is **name-keyed end-to-end** — no namespace bridge):

```sql
-- conceptual Phase 39 catalog-scope read (NOT built this phase; the contract this phase guarantees):
WITH names AS (                          -- the universe of items by normalized name
  SELECT lower(trim(name)) AS norm, name, item_id, is_clicky, has_haste, icon_id, statsblock, ...
  FROM item_master                       -- held (EQ id), enrichment present
  UNION ALL
  SELECT norm_name AS norm, name, item_id, is_clicky, has_haste, icon_id, statsblock, ...
  FROM catalog_enrichment                -- unheld (PigParse id), enrichment present
)
-- COALESCE by norm: a held name (from item_master) and its catalog twin never co-occur, because
-- catalog_enrichment NEVER holds a held name (the crawl's held-name dedup). So a simple UNION ALL
-- already yields one row per normalized name; the held side carries holders (joined to the rollup),
-- the catalog side carries none.
```

Because `catalog_enrichment` holds ONLY unheld names (held-name dedup at write time), the UNION is
collision-free by construction — there is no name that appears in BOTH tables, so Phase 39's COALESCE
is trivial (no precedence logic needed; a held name is simply never in catalog_enrichment). This is the
property that makes the Phase-39 facet join name-keyed throughout and removes the namespace-bridge
hazard 39-CONTEXT flagged. `[VERIFIED: the held-name dedup at store/itemids.go:87-89,117]`

### Is a new migration needed? YES — 00017.

Under name-keyed Option B a new table is required. `[VERIFIED: 00016 is the last migration —
migrations/ glob shows 00001…00016, no 00017]`. The migration is additive (new table only), goose-on-boot,
no DROP/ALTER of existing tables, no `WatcherMaxSchemaVersion` bump (the watcher is off the read path;
`internal/sheet` no longer exists post-v2.0 — the migration headers state the gate "does not exist in the
off-Google backend" `[VERIFIED: migrations/00012_item_icon.sql:6-10; 00016 header:15-18]`), no `v*` tag.

## Crawl Runtime + Resilience Math (ENRICH-14) — unchanged from the prior research

- **Courtesy sleep:** `interRequestSleep = 1 * time.Second`, applied via `wikiSleepFn(ctx, …)` BEFORE
  every page fetch in the items loop. `[VERIFIED: jobs/wiki.go:49-51,170-179]`
- **Catalog size:** ~4,341 PigParse Blue rows; live probe shows 4,343 `[CITED: 38-CONTEXT.md ⚠ block; STATE.md prod probe]`.
  The union is held ∪ catalog-only ≈ catalog size + held-names-not-in-catalog. The reversal does NOT
  change the fetch-set size (dropping the collision guard ADDS 43 refs back — ~1% more, still ~4.3k).
- **First (seed) run:** ~4,343 × ~1s sleep + fetch/parse ≈ **~72+ minutes** of wall time on the first
  Sunday. Explicitly fine — the job has NO execution cap (`wiki.go` header `[VERIFIED: jobs/wiki.go:3-31]`),
  runs in a background goroutine, single-writer DB + per-job mutex serialize writes. "A 70-minute Sunday
  background run is explicitly fine; do not add complexity to make it shorter" `[CITED: 38-CONTEXT.md ## Specific Ideas]`.
- **Steady-state weekly run:** every page sends `If-None-Match`; unchanged pages 304 (still 1s-spaced,
  ~72 min wall but near-zero parse/DB cost). The 304 short-circuit (`fetchUnchanged` → `continue`) skips
  parse+write `[VERIFIED: jobs/wiki.go:184-186,524-527]`. The ETag is persisted after EVERY successful
  fetch+parse, write or not `[VERIFIED: jobs/wiki.go:207-214]`, so the widened set 304s cheaply once seeded.
  This is true for BOTH branches — the ETag is keyed by URL, independent of which store the write lands in.
- **Resilience scales unchanged:** a single bad catalog page (fetch error / MediaWiki error envelope /
  empty wikitext / parse failure) is logged-and-SKIPPED via `fetchSkip` / `!ok` — the run NEVER aborts
  `[VERIFIED: jobs/wiki.go:184-198,559-588]`. ctx cancellation (SIGTERM mid-crawl) unwinds cleanly and
  returns partial counts, not an error `[VERIFIED: jobs/wiki.go:172-179]`.
- **No day-boundary bookkeeping** — the capless job is one uninterrupted pass; `dueWiki` runs it once per
  Sunday UTC (Sunday AND last < start-of-this-Sunday `[VERIFIED: scheduler/scheduler.go:105-107]`).

## Icon-Backfill Mechanics (ENRICH-15)

- **The icon is ALREADY parsed and ALREADY in the held freshness comparison.** `parseIconID` extracts
  `lucy_img_ID` (0 for absent/blank/non-numeric/negative — the "no icon" sentinel)
  `[VERIFIED: enrich/wikiitem.go:591-601]`. `ParseItempage` sets `item.IconID`
  `[VERIFIED: enrich/wikiitem.go:126]`. The held upsert writes `icon_id` `[VERIFIED: store/enrich.go:240]`;
  `GetItemMasterFreshnessTx` returns it and re-writes on `existingIcon != int64(item.IconID)`
  `[VERIFIED: store/enrich.go:282-294; jobs/wiki.go:268-286]`. The NEW catalog path mirrors this via
  `catalog_enrichment.icon_id` + `GetCatalogEnrichmentFreshnessTx`.
- **So ENRICH-15 needs NO new icon PARSE logic.** Widening the set + branching the write is the entire
  mechanism: an unheld catalog item now gets a wiki fetch, its `lucy_img_ID` is parsed, and `icon_id` is
  written to `catalog_enrichment`. Held items' stale icons still self-heal via the item_master freshness
  path (regression test `TestRunWiki_BackfillsStaleIcon` `[VERIFIED: jobs/wiki_test.go exists for this]`).
- **What makes an item "icon-less" (two cases — the diagnostic must distinguish):**
  1. **Genuinely icon-less:** the wiki page has no `lucy_img_ID` → `parseIconID` returns 0 → `icon_id = 0`
     → colored-tile fallback (the intended permanent behavior, BOTH stores). `[VERIFIED: migrations/00012_item_icon.sql:7-9; enrich/wikiitem.go:591-595]`
  2. **Not-yet-enriched:** no row yet (never fetched), OR a row whose `icon_id` is NULL/0 (predates a
     successful enrichment). After a full crawl these should be empty; any residue is case 1 or a
     page that 304'd/failed.
  The residue ENRICH-15 cares about ("still icon-less") = catalog items with `icon_id` 0/NULL across
  BOTH stores after the crawl — that IS the colored-tile set.
- **No boot backfill needed for icons OR for `catalog_enrichment`.** Phase 37's `BackfillItemFlags` boot
  pass exists ONLY because flags were a NEW field over EXISTING held rows that already had a stored
  statsblock to re-parse (no network) `[VERIFIED: store/backfill.go:1-106; main.go:228-239]`.
  `catalog_enrichment` is DIFFERENT: it starts EMPTY and has no local data for unheld items to re-parse —
  the only source of an unheld item's icon/flags is a fresh wiki fetch, which the widened weekly crawl
  performs. So the catalog "backfill" IS the crawl; there is **no boot pass for catalog_enrichment**
  (do NOT add a `BackfillCatalogEnrichment` — it would have nothing local to read). **Recommendation: the
  weekly crawl populates `catalog_enrichment` from empty on first run; no one-time boot backfill.** This
  matches the icon precedent and the D-01 "seed once, weekly re-validates" decision. The first Sunday
  after deploy fills it; a maintainer who wants it sooner can manually trigger the scheduler (out of
  scope — deferred re-crawl button).

## D-03 Coverage Diagnostic — across BOTH stores (ENRICH-15 maintainer diagnostic)

The shipped `ItemMasterIconCoverage` reads ONLY `item_master` `[VERIFIED: store/itemids.go:163-197]`. Under
name-keyed Option B that under-reports — it would miss every catalog-only item (the bulk of the 4,343).
**Re-point it to read TRUE coverage across BOTH stores.** Two implementation options (planner picks):

1. **Extend `ItemMasterIconCoverage` → a `CatalogIconCoverage` that UNIONs both tables** (recommended):
   count/icon-cover/icon-less over `item_master ∪ catalog_enrichment`, deduped by normalized name (a held
   name is never in catalog_enrichment, so a `UNION ALL` already yields one row per item — same property
   as the Phase-39 read). The residue sample reads icon-less PUBLIC names from both stores, name-ordered,
   bounded by `sampleCap` (the self-DoS guard `[VERIFIED: store/itemids.go:160-162,236-238]`).
2. Keep two reads and sum in the job — simpler but two slog lines or a manual merge; less clean.

**Shape (recommended — one coverage struct over both stores):**

```sql
-- count + icon-covered over the UNION (one row per normalized name; held names never in catalog_enrichment)
SELECT count(*), COALESCE(SUM(CASE WHEN icon_id IS NOT NULL AND icon_id > 0 THEN 1 ELSE 0 END), 0)
FROM (
  SELECT lower(trim(name)) AS norm, icon_id FROM item_master
  UNION ALL
  SELECT norm_name AS norm, icon_id FROM catalog_enrichment
);
-- icon-less residue sample (bounded), PUBLIC names only (V7), name-ordered:
SELECT name FROM (
  SELECT name, icon_id FROM item_master WHERE name IS NOT NULL AND trim(name) <> ''
  UNION ALL
  SELECT name, icon_id FROM catalog_enrichment WHERE name IS NOT NULL AND trim(name) <> ''
)
WHERE icon_id IS NULL OR icon_id = 0
ORDER BY name LIMIT ?;
```

The slog line (extend `logItemsCoverage` `[VERIFIED: jobs/wiki.go:245-255]`) reports `union_size` /
`written` / `unchanged` (this pass) + `total` / `icon_covered` / `icon_less` + the bounded `residue_sample`
across BOTH stores. Structured key/value `slog`, PUBLIC names only — NEVER statsblock/wikitext bodies (V7)
`[VERIFIED: store/enrich.go:22-23; jobs/wiki.go:243-244]`. The maintainer greps `slog` JSON on the VPS (the
established ops pattern; CLAUDE.md structured-logging convention) — zero new UI surface, lowest cost.

## Standard Stack

This phase adds **no new dependency**. It edits existing Go backend packages + adds one migration file.

| Package (existing) | Purpose in this phase |
|--------------------|----------------------|
| `internal/backendsrv/migrations` | NEW `00017_catalog_enrichment.sql` (additive table; goose-on-boot). |
| `internal/backendsrv/store` | Revise `DistinctEnrichmentRefs` (drop guard, add `Held`); NEW `UpsertCatalogEnrichmentTx` + `GetCatalogEnrichmentFreshnessTx` + the `CatalogEnrichment` input struct; replace `ItemMasterIconCoverage` with a both-stores coverage read. Single tested SQL path. |
| `internal/backendsrv/enrich/jobs` | `runWikiItems` swaps to the held-flagged union + BRANCHES the write (held→item_master, catalog→catalog_enrichment); emits the both-stores D-03 coverage `slog` line. |
| `internal/backendsrv/enrich` | Parser UNCHANGED — `ParseItempage`/`parseIconID` already produce everything. |
| `internal/backendsrv/store` (`enrich.go`) | `UpsertItemMasterTx` / `GetItemMasterFreshnessTx` UNCHANGED (held path). |

**Go version:** 1.24 per CLAUDE.md / the backend module. No version-sensitive APIs introduced.
`[ASSUMED]` — backend builds under the repo's existing toolchain; no new module.

## Architecture Patterns

### System data flow (the widened + branched items pass)

```
                 ┌─────────────────────────────────────────────┐
                 │ scheduler.dueWiki (Sunday UTC, once/Sunday)  │
                 └───────────────────┬─────────────────────────┘
                                     │ RunWiki(ctx, db, politefetch.Fetch)
                                     ▼
        ┌────────────────────────── runWikiItems ───────────────────────────────┐
        │  refs := store.DistinctEnrichmentRefs(ctx)   ← held∪catalog, Held flag │
        │                                                 (collision guard DROPPED)│
        │  for each ref:                                                          │
        │     wikiSleepFn(ctx, 1s)            ← courtesy pace, ctx-aware          │
        │     fetchWikiPage(ETag)  ──304──► unchanged++; continue (Pitfall)       │
        │            │ 200                                                        │
        │            ▼                                                            │
        │     ParseItempage(wikitext) ─!ok─► failed++; continue (log+skip)        │
        │            │ ok (IconID, flags, statsblock, …)                          │
        │            ▼                                                            │
        │     ┌── ref.Held? ────────────────────────────────────────────────┐    │
        │     │ HELD  → GetItemMasterFreshnessTx(EQ id) → UpsertItemMasterTx │    │
        │     │          (+ ReplaceQuestItemsForIDTx)         [UNCHANGED]    │    │
        │     │ CATALOG→ GetCatalogEnrichmentFreshnessTx(norm) →            │    │
        │     │          UpsertCatalogEnrichmentTx(norm,…)   [NEW; no quests]│    │
        │     └─────────────────────────────────────────────────────────────┘    │
        │            │ all 4 fields unchanged ─► unchanged++; continue            │
        │            ▼ any differs → write → SetETag(url) → written++             │
        │  ── after loop: slog "items coverage" over BOTH stores ───────────────► │ D-03
        └────────────────────────────────────────────────────────────────────────┘
                                     │
   downstream (Phase 39 SEARCH-06):  ▼  reads item_master(held by EQ id)
              UNION catalog_enrichment(unheld by name), one row per normalized name
```

### Pattern 1: held∪catalog union with a `Held` branch flag (held wins the name)
**What:** a single store read returns `(item_id, name, held)` refs covering every distinct item by name;
held names carry their EQ id + `held=true`, catalog-only names carry their PigParse id + `held=false`.
**When to use:** the input to the widened `runWikiItems` loop.
**Why:** preserves "fetch each item's page exactly once" politeness (dedup by name = wiki-page identity)
AND tells the write path which store to target. `[VERIFIED: store/itemids.go:66-99 politeness rule; readviews.go:287-302 pp_rep dedup]`

### Pattern 2: parallel freshness short-circuit per store (icon carried for free, both)
**What:** held → `GetItemMasterFreshnessTx` (sha|icon|stats|flags); catalog → `GetCatalogEnrichmentFreshnessTx`
(same four, by name). The job re-writes whenever ANY differs.
**When to use:** the branched write.
**Why:** the icon participates in the compare in BOTH stores, so the crawl populates icons with no
separate icon-write path; an empty flag set's `"[]"` byte-matches (D-06 idempotency).
`[VERIFIED: store/enrich.go:267-294; jobs/wiki.go:268-286; MarshalFlags store/enrich.go:34-60]`

### Anti-Patterns to Avoid
- **Id-keying catalog rows into `item_master`** (Option A) — REJECTED; the prod probe proved it drops 43
  real catalog items on PigParse↔EQ id collisions. Use the name-keyed table. `[CITED: STATE.md prod probe]`
- **Synthetic/offset PigParse ids in `item_master.item_id`** — leaks a fake id that held readers treat as
  a real EQ id. Rejected.
- **Writing catalog-only quest links into quest_items under a PigParse id** — quest_items is the EQ
  namespace, id-joined `[VERIFIED: readviews.go:463-465; 00001:63]`; a PigParse id there is unreachable
  and can collide. The catalog branch SKIPS quest_items (see D-04).
- **A boot `BackfillCatalogEnrichment` pass** — there is no local data to re-parse for an unheld item; the
  crawl is the only source. Don't mirror `BackfillItemFlags` here.
- **Per-(id,name) DISTINCT instead of per-name dedup** — would refetch the same wiki page under multiple
  casings/ids → a politeness regression. Dedup by `lower(trim(name))`. `[VERIFIED: store/itemids.go:94-96]`
- **Leaving `ItemMasterIconCoverage` reading only item_master** — under name-keying it under-reports the
  whole catalog. Re-point it to BOTH stores. `[VERIFIED: store/itemids.go:163-197]`
- **Sending ETags for the gear-tier pages** — unrelated; the gear pass is deliberately unconditional
  (full-replace) and must stay so (H-01 staleness trap). This phase touches only the items pass.
  `[VERIFIED: jobs/wiki.go:426-486,531-552]`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Catalog enrichment upsert | A bespoke INSERT in the job | A new `UpsertCatalogEnrichmentTx` modeled on `UpsertItemMasterTx` | One tested SQL path; the job authors ZERO inline SQL (11-05). `[VERIFIED: store/enrich.go:3-23,234-248]` |
| Catalog freshness compare | A new compare shape | `GetCatalogEnrichmentFreshnessTx` mirroring `GetItemMasterFreshnessTx` | Same sha/icon/stats/flags self-heal, by name. `[VERIFIED: store/enrich.go:267-294]` |
| Flags JSON encoding | A local `json.Marshal` | `store.MarshalFlags` | The ONE canonical encoder — byte-matches upsert+freshness (D-06). `[VERIFIED: store/enrich.go:34-60]` |
| Dedup held vs catalog by name | A bespoke in-Go map merge | The `lower(trim(name))` SQL dedup in the union + the `pp_rep` precedent | One tested SQL path. `[VERIFIED: itemrollup.go:16-21; readviews.go:287-302]` |
| Per-item wiki fetch + 304 + parse | A new loop | The existing `runWikiItems` body (only the write branches) | It already does everything per item. `[VERIFIED: jobs/wiki.go:160-234]` |
| Icon extraction | A new icon parser | `parseIconID` | Already parses `lucy_img_ID`, type-safe, sentinel-0. `[VERIFIED: enrich/wikiitem.go:591-601]` |
| ctx-aware courtesy pacing | A bare `time.Sleep` | `wikiSleepFn` | Unwinds promptly on SIGTERM; test-overridable. `[VERIFIED: jobs/wiki.go:53-77]` |

**Key insight:** Phase 38 (name-keyed) is *reuse + one new parallel store layer (upsert/freshness, modeled
1:1 on the held one) + a write branch + a both-stores coverage read + one additive migration*. Every new
piece has an exact existing template — resist inventing new shapes.

## Common Pitfalls

### Pitfall 1: Forgetting to DROP the Option-A collision guard
**What goes wrong:** if `DistinctEnrichmentRefs` keeps `item_id NOT IN (SELECT item_id FROM item_master)`,
the 43 collision items stay dropped even though name-keying makes the guard pointless — the reversal's
whole purpose is defeated.
**Why it happens:** the guard is already in the shipped code `[VERIFIED: store/itemids.go:118]`.
**How to avoid:** explicitly remove that exclusion in the union; add a test asserting a catalog item whose
PigParse id equals a held EQ id (for a different name) NOW appears in the union with `held=false` (the
inverse of the shipped `TestDistinctEnrichmentRefs` Case C `[VERIFIED: store/itemids_test.go:128-132,194-197]`,
which currently asserts it is EXCLUDED — that assertion must FLIP).

### Pitfall 2: Catalog row written to the wrong store (branch on the wrong condition)
**What goes wrong:** a held ref accidentally routed to `catalog_enrichment` (duplicating its enrichment) or
a catalog ref routed to `item_master` (re-introducing the id collision).
**Why it happens:** the branch must key on the ref's `Held` flag, not on whether a row already exists.
**How to avoid:** branch strictly on `ref.Held`; the union guarantees a held name is never in the catalog
arm. Add a test: a held name produces an `item_master` row and NO `catalog_enrichment` row; a catalog-only
name produces a `catalog_enrichment` row and NO `item_master` row.

### Pitfall 3: pigparse_price rows with junk/spell names that have no wiki item page
**What goes wrong:** the catalog contains rows whose `name` is not a real wiki *item* page (spell scrolls,
mislabeled rows, EC-tunnel noise); fetching them 404s / returns a redirect stub / a non-Itempage page.
**Why it happens:** PigParse is an auction feed, not a curated item list.
**How to avoid:** the existing resilience already handles it — `ParseItempage` returns `ok=false`
("wikitext_too_short" for a redirect stub, "no_itempage" when no `{{Itempage}}` template), the loop logs
`item parse skipped` and `continue`s (failed++); these names stay in the icon-less residue (the D-03
diagnostic). NO abort, NO catalog_enrichment row for them. `[VERIFIED: enrich/wikiitem.go:93-101; jobs/wiki.go:193-198]`

### Pitfall 4: Politeness regression — refetching the same page twice
**What goes wrong:** if the union dedups by id (or by (id,name)) instead of by name, the same wiki page is
fetched under multiple ids/casings → > 1 req/s for that page.
**Why it happens:** held and catalog rows for the same item have different ids; only the NAME is shared.
**How to avoid:** the catalog arm dedups by `lower(trim(name))` and excludes held names; one ref per
normalized name across the boundary. `[VERIFIED: store/itemids.go:87-96,108-119]`

### Pitfall 5: Under-counting coverage (diagnostic still reads only item_master)
**What goes wrong:** the D-03 line reports only held-item coverage, so the maintainer sees ~953 total
instead of ~4,343 and the unheld icon-less residue is invisible — defeating ENRICH-15.
**Why it happens:** `ItemMasterIconCoverage` reads one table `[VERIFIED: store/itemids.go:166-181]`.
**How to avoid:** the coverage read must UNION both stores (see D-03 section). Add a test seeding both
tables and asserting total/covered/less span both.

### Pitfall 6: Seed run interrupted (server restart / SIGTERM mid-crawl) leaves partial coverage
**What goes wrong:** the ~72-min first run is cut short; only some catalog items are enriched.
**Why it happens:** deploys / reboots happen.
**How to avoid:** SELF-HEALING by design — `dueWiki` re-runs next Sunday; the ETag short-circuit means
already-enriched pages 304 cheaply while un-reached ones get their first fetch. A mid-crawl ctx cancel
returns partial counts cleanly. No resume bookkeeping. `[VERIFIED: jobs/wiki.go:172-179; scheduler/scheduler.go:105-107]`

## Code Examples

### The branched write in `upsertItemAndQuests` (the core behavioral change)
```go
// jobs/wiki.go — today the body always upserts item_master by ref.ItemID (Option A).
// Phase 38 (name-keyed) branches on ref.Held:
if ref.Held {
    // HELD — unchanged: item_master by EQ id + quest_items.
    existingSHA, existingIcon, existingStats, existingFlagsJSON, err := store.GetItemMasterFreshnessTx(ctx, tx, ref.ItemID)
    // ... compare 4 fields ... store.UpsertItemMasterTx(...) ... store.ReplaceQuestItemsForIDTx(...)
} else {
    // CATALOG-ONLY — NEW: catalog_enrichment by norm_name; NO quest_items write.
    norm := strings.ToLower(strings.TrimSpace(item.ItemName))
    existingSHA, existingIcon, existingStats, existingFlagsJSON, err := store.GetCatalogEnrichmentFreshnessTx(ctx, tx, norm)
    // ... same 4-field compare (parsedFlagsJSON via store.MarshalFlags) ...
    // store.UpsertCatalogEnrichmentTx(ctx, tx, store.CatalogEnrichment{NormName: norm, ItemID: int(ref.ItemID), Name: item.ItemName, IconID: item.IconID, ...})
}
// [VERIFIED templates: jobs/wiki.go:261-332; store/enrich.go:234-294]
```

### The freshness short-circuit already carries the icon (both stores, no parse change)
```go
// held: store.GetItemMasterFreshnessTx — catalog: store.GetCatalogEnrichmentFreshnessTx
// both return (sha, iconID, statsblock, flagsJSON); the job re-writes when ANY differs:
if existingSHA == item.WikitextSHA1 && existingIcon == int64(item.IconID) &&
   existingStats == item.Statsblock && existingFlagsJSON == parsedFlagsJSON {
    return false, nil // unchanged — icon already compared
}
// [VERIFIED: jobs/wiki.go:278-286 (held); the catalog getter mirrors store/enrich.go:282-294]
```

### The both-stores coverage line (D-03)
```go
slog.Info(wikiJobName+": items coverage",
    "union_size", unionSize, "written", written, "unchanged", unchanged,
    "total", cov.Total, "icon_covered", cov.IconCovered, "icon_less", cov.IconLess,
    "residue_sample", cov.ResidueSample) // both-stores read; bounded sample (<=50)
// [pattern: jobs/wiki.go:245-255 logItemsCoverage]
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Enrichment = held items only (`DistinctInventoryItemIDs`) | Held ∪ full PigParse catalog by name | This phase (38) | Unheld items get icons + flags; powers SEARCH-06 full-catalog scope. |
| Catalog-only enrichment id-keyed into `item_master` (Option A) | Catalog-only enrichment name-keyed in a SEPARATE `catalog_enrichment` table (Option B) | This re-plan (2026-06-25) | Recovers the 43 dropped collision items; covers all 4,343; held readers untouched; Phase 39 join name-keyed end-to-end. |
| No 00017 (Option A reused `item_master`) | 00017 adds `catalog_enrichment` | This re-plan | Additive migration; goose-on-boot; no watcher change. |
| `internal/sheet` + `WatcherMaxSchemaVersion` gate | Gone post-v2.0 ("off Google") | v2.0 | The CLAUDE.md `WatcherMaxSchemaVersion` reference is STALE; no such gate in the backend. `[VERIFIED: migrations/00012,00016 headers]` |

## Project Constraints (from CLAUDE.md)

- **Extend-only schema:** additive only; goose on boot; version-stamped + idempotent. → Option B adds
  migration **00017** (new table, no ALTER of existing tables). `[VERIFIED: CLAUDE.md Architecture/Conventions; 00016 is last]`
- **`WatcherMaxSchemaVersion` reference is STALE post-v2.0** — `internal/sheet` no longer exists; the
  backend uses goose `version()` as the version of record. No bump, no `v*` tag.
  `[VERIFIED: migrations/00012_item_icon.sql:6-10; 00016 header:15-18]`
- **Structured logging both sides** — Go `slog` with op + key/value fields; counts/ids/err only, NEVER raw
  page/flag content (V7). The D-03 diagnostic logs PUBLIC item names only. `[VERIFIED: CLAUDE.md Conventions; store/enrich.go:22-23]`
- **Single tested SQL path (11-05):** the enrichment job authors ZERO inline SQL — the new
  `catalog_enrichment` upsert/freshness/coverage reads live in `store/`; the job composes them.
  `[VERIFIED: store/itemids.go:5-14; enrich.go:1-23]`
- **Watcher untouched, no `v*` tag** — backend (+ later web) only this milestone. `[VERIFIED: ROADMAP.md v2.6 scope; CLAUDE.md]`
- **PigParse Blue = server 1 for getall** — the catalog is the Blue catalog (already populated daily).
  `[VERIFIED: memory pigparse-server-numbering-blue-is-1]`

## Runtime State Inventory

> Not a rename/refactor/migration phase. The closest analog is "new data lands in a NEW table."

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | NEW `catalog_enrichment` table gains a row per unheld catalog name (icon/flags/stats/etc.). `item_master` UNCHANGED (held-only). `pigparse_price`/`inventory_item` are read-only sources. | Migration 00017 (create table) + the new write path. No data migration — rows are written idempotently by the crawl from empty. |
| Live service config | None — the crawl runs inside the existing weekly job; no new scheduler entry, no new external service. | None — `scheduler.Start` already registers `wiki_weekly` `[VERIFIED: scheduler/scheduler.go:146-151; main.go:268]`. |
| OS-registered state | None — no OS task/cron; the in-process scheduler owns cadence. | None. |
| Secrets/env vars | None — no new secret; the wiki API needs no auth. | None. |
| Build artifacts | None — no new package, no new module, no new binary name (one new migration .sql file + new methods in existing packages). | None. |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| P1999 MediaWiki API (`wiki.project1999.com/api.php`) | The crawl (already used weekly) | ✓ (in production) | — | log-and-skip per page (existing resilience) |
| `pigparse_price` table populated (~4,343 rows) | The catalog arm of the union | ✓ (daily PigParse job populates it; live count 4,343) | — | empty catalog → union degrades to held-only (no crash) |
| goose (migrations on boot) | 00017 | ✓ (runs every boot) `[VERIFIED: main.go:223 RunMigrations]` | — | — |
| Go toolchain (backend module) | Build | ✓ `[ASSUMED — same toolchain as repo CI]` | 1.24 | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** If `pigparse_price` is empty at crawl time, the union collapses
to held-only (today's behavior) — no error.

## Security Domain

> `security_enforcement: true`, ASVS level 1, block-on `high` `[VERIFIED: .planning/config.json:40-42]`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface added (a background job). |
| V3 Session Management | no | — |
| V4 Access Control | no | Read/write enrichment in a background job; the D-03 diagnostic is logs, not a user surface. |
| V5 Input Validation | yes | Untrusted wiki text (names, statsblock, flags, slot, clicky_effect) binds through `?` placeholders ONLY in the NEW `UpsertCatalogEnrichmentTx` — NEVER string-concatenated (the `itemMasterUpsert` discipline, repeated). `parseIconID`/`parseHastePct` coerce wiki values to non-negative ints (no untrusted string reaches the `Item_<int>.png` URL). The `norm_name` PK is `lower(trim(name))` computed in Go, bound as a `?` value. `[VERIFIED: store/enrich.go:234-247; enrich/wikiitem.go:591-601,624-638]` |
| V6 Cryptography | no | `crypto/sha1` is a content fingerprint (change detection), NOT a security hash — already documented. `[VERIFIED: enrich/wikiitem.go:25-31]` |
| V7 Error/Log handling | yes | `slog` logs counts/ids/err only, never raw page/flag content. The D-03 residue sample logs item NAMES (public wiki page titles) from BOTH stores — acceptable; do NOT log statsblock/wikitext bodies. `[VERIFIED: store/enrich.go:22-23]` |

### Known Threat Patterns for {Go backend + MediaWiki/PigParse ingest}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via parsed item name / flags into the new table | Tampering | `?` placeholders only in `UpsertCatalogEnrichmentTx` (the item_master discipline). `[VERIFIED: store/enrich.go:234-247]` |
| Untrusted icon id reaching an image URL | Tampering | `parseIconID` → non-negative int sentinel-0; type-safe `Item_<int>.png`. `[VERIFIED: enrich/wikiitem.go:591-601]` |
| Log injection / PII in logs | Info disclosure | Counts/ids/err + bounded public page names only; no body content. `[VERIFIED: store/enrich.go:22-23]` |
| Crawl DoS on the wiki (rude crawler) | DoS (third party) | 1s ctx-aware courtesy sleep per fetch + ETag 304s; capless but never faster than 1 req/s. `[VERIFIED: jobs/wiki.go:49-51,170-179]` |
| Resource exhaustion from a giant residue log | DoS (self) | Bound the residue sample slice (≤50). `[VERIFIED: store/itemids.go:160-162,236-238]` |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The catalog is ~4,343 rows; the union (collision guard dropped) ≈ catalog size + held-names-not-in-catalog. | Runtime math | Low — runtime estimate scales linearly. Verify with `SELECT count(*) FROM pigparse_price` (probe already showed 4,343). |
| A2 | No current requirement needs quest links for UNHELD catalog items, so the catalog branch SKIPS the quest_items write. | D-04 (quest links) | Low-Medium — if Phase 39+ wants unheld quest links, a name-keyed catalog_quest_items would be a later additive table; quest_items stays held-only. Confirm against 39-CONTEXT (facets are Clicky/Haste, not quests). |
| A3 | The weekly crawl populates `catalog_enrichment` from empty on first run (no boot backfill), like icons. | Icon-Backfill Mechanics | Low — an unheld item has no local data to re-parse; a boot pass would have nothing to read. First Sunday fills it; deferred re-crawl button covers "sooner." |
| A4 | The backend builds under the repo's existing Go 1.24 toolchain with no new module. | Standard Stack / Env | Low — no new dependency. |
| A5 | Logging public wiki page NAMES (from both stores) in the residue sample is acceptable (not PII). | Security V7 | Low — page titles are public wiki content; statsblock/wikitext bodies are NOT logged. |

> **No `[ASSUMED]` claim in this research is a collision/coverage estimate** — the id-collision assumption
> that broke Option A (prior A2: "≈0 collisions") is now a `[VERIFIED]` prod fact (60 collisions / 43 drops),
> and name-keying makes it irrelevant. The remaining assumptions are scope/build, not coverage-correctness.

## Open Questions

1. **Quest links for unheld catalog items (A2).**
   - What we know: quest_items is EQ-namespace, id-joined; a catalog-only item has no EQ id.
   - What's unclear: whether any downstream surface wants quest links for unheld items.
   - Recommendation: SKIP the quest_items write in the catalog branch this phase (item_master quest links
     stay held-only). Revisit with a name-keyed `catalog_quest_items` only if a future requirement needs it.

2. **Coverage read: one both-stores method vs. two summed in the job.**
   - What we know: `ItemMasterIconCoverage` reads one table and must become both-stores.
   - What's unclear: whether to UNION in SQL (recommended) or sum two reads in Go.
   - Recommendation: one `CatalogIconCoverage` store method that UNIONs both tables (one tested SQL path,
     one slog line). Add a test seeding both stores.

3. **`DistinctEnrichmentRefs` ref shape: extend `ItemRef` vs. new `EnrichmentRef`.**
   - What we know: the held-only path shares `ItemRef`; adding `Held` ripples into its tests.
   - Recommendation: a dedicated `EnrichmentRef{ItemID; Name; Held}` struct, leaving `ItemRef` untouched.

## Sources

### Primary (HIGH confidence — codebase, verified this session)
- `internal/backendsrv/store/itemids.go` — `DistinctInventoryItemIDs`, the shipped Option-A `DistinctEnrichmentRefs` (+ its collision guard to DROP), `ItemRef`, `ItemMasterIconCoverage` (to re-point), the per-name politeness rule.
- `internal/backendsrv/store/enrich.go` — `UpsertItemMasterTx`, `GetItemMasterFreshnessTx`, `itemMasterUpsert` (all 19 columns), `MarshalFlags` — the templates for the new catalog methods.
- `internal/backendsrv/enrich/jobs/wiki.go` — `RunWiki`/`runWikiItems` (the loop to branch), `upsertItemAndQuests`, the 1s `wikiSleepFn`, ETag 304, log-and-skip, the no-cap header, `logItemsCoverage`.
- `internal/backendsrv/store/readviews.go` — the `pp_rep` CTE / normalized-name bridge, `ItemMasterIconStats` + the three id-keyed held readers (zero-blast-radius proof).
- `internal/backendsrv/compute/itemrollup.go` — `buildItemRollups` group-by `lower(trim(name))` (the name-dedup precedent + the id-correct `iconStats[vr.ID]` lookup).
- `internal/backendsrv/store/itemsearch.go` — `SearchCatalog` over `pigparse_price` (the Phase 39 read; the catalog_enrichment column set must serve it).
- `internal/backendsrv/store/backfill.go` + `cmd/squirebot-server/main.go:228-239` — the Phase 37 boot backfill pattern (and why catalog_enrichment needs none).
- `internal/backendsrv/enrich/wikiitem.go` — `ParseItempage`, `parseIconID`, `ParsedWikiItem` (the parse output the new upsert consumes).
- `internal/backendsrv/migrations/00001_init.sql` (the two id namespaces), `00012_item_icon.sql` / `00013_item_statsblock.sql` / `00016_item_flags_effects.sql` (the additive/nullable/freshness precedents; 00016 is last → 00017).
- `internal/backendsrv/scheduler/scheduler.go` — `dueWiki` (Sunday-once cadence), `wiki_weekly` registration.
- `internal/backendsrv/store/itemids_test.go` — the shipped `TestDistinctEnrichmentRefs` (Case C assertion must FLIP) + `TestItemMasterIconCoverage` (must become both-stores).
- `.planning/config.json` — `nyquist_validation:false` (Validation Architecture omitted), `security_enforcement:true` ASVS 1.

### Secondary (MEDIUM)
- `.planning/phases/38-…/38-CONTEXT.md` (⚠ D-04 REVERSED block — binding), `.planning/STATE.md` (the 60/43 prod probe), `.planning/REQUIREMENTS.md` (ENRICH-14/15), `.planning/ROADMAP.md`.
- Memory: `pigparse-vs-ingame-item-id-namespaces` (the load-bearing reason for name-keying), `pigparse-server-numbering-blue-is-1`.

### Tertiary (LOW)
- None — pure internal phase; no unverified web claims.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; all packages read and verified.
- Architecture / D-04 (name-keyed): HIGH — both id namespaces, the held readers' id-usage, the upsert/freshness templates, and the held-name dedup property are all verified directly; the zero-blast-radius claim is verified against the five enumerated held readers. The 60/43 collision count is a `[VERIFIED]` prod probe (replaces the broken Option-A `[ASSUMED]` ≈0).
- Pitfalls: HIGH — each maps to a verified resilience mechanism in `wiki.go` or a verified namespace fact, plus the two reversal-specific pitfalls (drop-the-guard, branch-correctly) keyed to the exact shipped lines to change.

**Research date:** 2026-06-25
**Valid until:** 2026-07-25 (stable internal codebase; revalidate if `wiki.go`/`enrich.go`/`item_master`/`itemids.go` change before planning).

## RESEARCH COMPLETE
