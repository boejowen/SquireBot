# Phase 38: Catalog-wide enrichment + icon coverage - Pattern Map

**Mapped:** 2026-06-25
**Files analyzed:** 3 modified + 2 test files (the whole phase is reuse + one input swap + one log line)
**Analogs found:** 5 / 5 (every change has an exact in-package analog)

> **Read this first (the one load-bearing invariant for the whole phase):**
> The widened crawl MUST fetch each wiki page **exactly once**, deduped by
> `lower(trim(name))` — **NOT by item_id**. Held and catalog rows for the same item
> have *different* ids (EQ-inventory namespace vs PigParse namespace) but the **same
> name**; only the name is the wiki-page identity. The catalog arm of the union MUST
> exclude any `pigparse_price.item_id` already present in `item_master` (so a PigParse
> id numerically equal to a held EQ id never reaches the `ON CONFLICT(item_id)` upsert
> and overwrites the wrong row) AND any normalized name already produced by the held
> arm (held wins). This is the D-04 collision guard — Pitfall 2/4 in RESEARCH.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/store/itemids.go` (ADD `DistinctEnrichmentRefs`) | store / read-method | batch / read-only SELECT | `DistinctInventoryItemIDs` in the SAME file + the `pp_rep` CTE in `readviews.go` | exact (same file, same `ItemRef` return shape) |
| `internal/backendsrv/store/itemids_test.go` (ADD union test) | test | — | `TestDistinctInventoryItemIDs` in the SAME file | exact |
| `internal/backendsrv/enrich/jobs/wiki.go` (`runWikiItems` input swap + D-03 slog) | enrichment job / loop | event-driven (weekly) / batch crawl | `runWikiItems` ITSELF (only the ref source + a post-loop log change) | exact (in-place) |
| `internal/backendsrv/enrich/jobs/wiki_test.go` (widen seeding + coverage assertion) | test | — | `TestRunWiki_PopulatesAllTables` / `TestRunWiki_BackfillsStaleIcon` in the SAME file | exact |
| `internal/backendsrv/store/enrich.go` | store / write-method | — | **UNCHANGED** — `UpsertItemMasterTx` / `GetItemMasterFreshnessTx` already namespace-agnostic on `item_id` | n/a (reuse verbatim) |

**No migration in this phase.** Option A (RESEARCH §D-04) reuses `item_master` keyed by
`item_id` for any namespace; `item_master` already carries every column the parse writes
(00012 icon, 00013 statsblock, 00016 flags). 00016 stays the last migration — **there is
NO 00017 here.** (`item_master` schema: `migrations/00001_init.sql:59`; columns confirmed
at `store/enrich.go:204-216`.)

---

## Pattern Assignments

### `store.DistinctEnrichmentRefs` — NEW read method in `internal/backendsrv/store/itemids.go` (store, batch read)

**Analog:** `DistinctInventoryItemIDs` in the SAME file (`store/itemids.go:40-64`) +
the normalized-name dedup CTE `pp_rep` in `store/readviews.go:287-302`.

**The `ItemRef` return shape is UNCHANGED** (`store/itemids.go:25-28`) — for a held name
`ItemID` is the EQ id (`MIN` representative, exactly as today); for a catalog-only name
`ItemID` is the PigParse id:
```go
type ItemRef struct {
	ItemID int64
	Name   string
}
```

**Existing held-arm pattern to copy** (`store/itemids.go:40-49`) — note `GROUP BY item_id`
+ `MIN(name)` is the "fetch each page once, deterministic representative name" politeness
rule the union MUST preserve (the header comment at `:30-39` is the load-bearing contract):
```go
func (s *Store) DistinctInventoryItemIDs(ctx context.Context) ([]ItemRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, MIN(name) AS name
		 FROM inventory_item
		 WHERE item_id IS NOT NULL AND item_id > 0
		 GROUP BY item_id
		 ORDER BY item_id`)
	// ... scan loop into []ItemRef, wrapped errors ...
}
```

**The normalized-name dedup idiom to reuse** (`store/readviews.go:287-302`) — this is the
EXISTING `pp_rep` CTE; the catalog arm of the union mirrors it (`lower(trim(name))`,
`name IS NOT NULL AND trim(name) <> ''`, `GROUP BY lower(trim(name))`):
```sql
WITH pp_rep AS (
       SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
       FROM pigparse_price
       WHERE name IS NOT NULL AND trim(name) <> ''
       GROUP BY lower(trim(name))
)
```

**The union to author** (RESEARCH §D-04 sketch — held arm = EQ ids unchanged; catalog arm
= PigParse names NOT held AND NOT already an EQ-id row; held wins on a name collision).
Keep the **EQ-id-collision exclusion** (`AND item_id NOT IN (SELECT item_id FROM item_master)`)
and the **held-name exclusion** (`AND lower(trim(name)) NOT IN (SELECT … held names)`):
```sql
WITH held AS (
  SELECT item_id, MIN(name) AS name
  FROM inventory_item
  WHERE item_id IS NOT NULL AND item_id > 0
  GROUP BY item_id
),
held_names AS (SELECT DISTINCT lower(trim(name)) AS norm FROM inventory_item WHERE item_id > 0),
catalog AS (
  SELECT MIN(item_id) AS item_id, MIN(name) AS name
  FROM pigparse_price
  WHERE name IS NOT NULL AND trim(name) <> ''
    AND lower(trim(name)) NOT IN (SELECT norm FROM held_names)
    AND item_id NOT IN (SELECT item_id FROM item_master)   -- EQ-id-collision exclusion (Pitfall 2)
  GROUP BY lower(trim(name))
)
SELECT item_id, name FROM held
UNION ALL
SELECT item_id, name FROM catalog
ORDER BY item_id;
```

**Conventions to copy (from the same package, mandatory):**
- The method is a plain `(*Store)` read method (no `Tx` variant; read side) — `store/itemids.go:9-14`.
- Wrap every error with `fmt.Errorf("...: %w", err)`; `defer rows.Close()`; check `rows.Err()` — `store/itemids.go:47-62`.
- The job authors ZERO inline SQL — this SELECT lives HERE, the job calls it (the 11-05 single-tested-SQL-path rule, stated in the file header `store/itemids.go:9-14` and `store/enrich.go:3-23`).
- slog is silent on the happy path for a read method (`store/itemids.go:14`).

**Load-bearing invariant:** dedup the catalog arm by `lower(trim(name))`, NEVER by id
(an id-keyed or `(id,name)`-keyed dedup refetches the same page under multiple casings =
a politeness regression — Anti-Pattern in RESEARCH; `store/itemids.go:30-39`).

---

### `runWikiItems` input swap + D-03 coverage slog — `internal/backendsrv/enrich/jobs/wiki.go` (enrichment job, batch crawl)

**Analog:** `runWikiItems` ITSELF (`jobs/wiki.go:160-214`) — the loop body is reused
byte-for-byte; ONLY the ref source and a post-loop log line change.

**The ONE behavioral change** (`jobs/wiki.go:161`):
```go
// today:
refs, rerr := s.DistinctInventoryItemIDs(ctx)   // held EQ ids only
// Phase 38:
refs, rerr := s.DistinctEnrichmentRefs(ctx)      // held ∪ catalog, deduped by name (Option A)
// ... the entire loop body (jobs/wiki.go:167-213) is UNCHANGED.
```

**The per-item body that carries the icon for FREE — DO NOT touch** (`jobs/wiki.go:227-245`):
the freshness short-circuit already compares `icon_id` alongside sha/statsblock/flags, so
widening the ref set is the entire icon-backfill mechanism (ENRICH-15). No new icon code:
```go
existingSHA, existingIcon, existingStats, existingFlagsJSON, err :=
	store.GetItemMasterFreshnessTx(ctx, tx, ref.ItemID)
parsedFlagsJSON := store.MarshalFlags(item.Flags)
if existingSHA == item.WikitextSHA1 && existingIcon == int64(item.IconID) &&
	existingStats == item.Statsblock && existingFlagsJSON == parsedFlagsJSON {
	return false, nil // unchanged — icon already in the comparison
}
```

**The upsert is namespace-agnostic on the id — reuse verbatim** (`jobs/wiki.go:247-271` →
`store.UpsertItemMasterTx`, `store/enrich.go:234-248`, `ON CONFLICT(item_id) DO UPDATE`
at `store/enrich.go:204-216`). A catalog-only ref's `ref.ItemID` (a PigParse id) flows
through `ItemID: int(ref.ItemID)` exactly like a held EQ id — no branch needed.

**Resilience to inherit unchanged (all already in `runWikiItems`):**
- 1s ctx-aware courtesy sleep before every fetch — `wikiSleepFn(ctx, interRequestSleep)` (`jobs/wiki.go:49-51, 168-172`). NEVER a bare `time.Sleep` (`jobs/wiki.go:59`).
- ETag 304 short-circuit — `fetchUnchanged → unchanged++; continue` (`jobs/wiki.go:176-179, 484-486`); ETag persisted after EVERY successful fetch+parse, write or not (`jobs/wiki.go:199-206`).
- log-and-skip-one-bad-page — `fetchSkip`/`!ok`/per-item write error all `failed++; continue`, the run NEVER aborts (`jobs/wiki.go:180-198`); junk catalog names (spell scrolls, redirects) fall here and land in the residue (Pitfall 3).
- ctx-cancel mid-crawl returns partial counts cleanly, not an error (`jobs/wiki.go:169-172`).

**D-03 coverage slog line — append before `runWikiItems`' `return`** (`jobs/wiki.go:213`).
Analog = the existing `detail`-string summary the job already logs (`jobs/wiki.go:149-153`).
Increment coverage counters inside the existing loop (or do ONE post-loop store read).
Log counts + a **bounded** residue sample (RESEARCH recommends ≤50 names) so a hundreds-long
residue never floods the log:
```go
slog.Info(wikiJobName+": items coverage",
	"total", total,            // union size iterated
	"enriched", enriched,      // rows with an item_master row after the pass
	"icon_covered", iconCovered, // icon_id > 0
	"icon_less", iconLess,       // icon_id 0/NULL = the colored-tile residue
	"residue_sample", boundedSample(residueNames, 50))
```
Follow the structured-logging convention: counts / ids / **public page NAMES** only —
NEVER raw statsblock/wikitext bodies (V7; `store/enrich.go:22-23`; CLAUDE.md Conventions).
Item names are public wiki titles, acceptable to log (RESEARCH Security V7 / A4).

**Anti-patterns to avoid (RESEARCH):** do NOT add a `BackfillItemIcon` boot pass (an unheld
item has no local statsblock to re-parse — the crawl is the only icon source; contrast the
Phase 37 `BackfillItemFlags` at `store/backfill.go`); do NOT touch the gear-tier pass
(`jobs/wiki.go:364-432`) — it is a deliberate unconditional full-replace (H-01 staleness trap).

---

### `store/itemids_test.go` — ADD a union test (mirror `TestDistinctInventoryItemIDs`)

**Analog:** `TestDistinctInventoryItemIDs` in the SAME file (`store/itemids_test.go:32-83`)
+ the `store`-package seed helpers.

**Scaffolding to reuse (all in the `store` package — confirmed):**
- `NewTestDB(t)` — `store/testhelper.go:23`
- `seedOwnerChar(t, db, label, name)` — `store/replace_test.go:12`
- `seedRaw(t, db, charID, location, name, itemID *int64, ordinal)` + `i64ptr(v)` — `store/itemids_test.go:12-27` (controls `item_id = 0 / NULL / concrete`)
- `seedItemMaster(t, db, itemID, name, summary, url, isQuest)` — `store/readviews_test.go:30-43`
- `seedPigparse(t, db, itemID, name, direction, a30, t30)` — `store/readviews_test.go:48-57` (its doc already notes the name-bridge / id-namespace rule)

**The held-arm assertion shape to copy** (`store/itemids_test.go:42-75`): seed duplicate
ids across characters, `item_id=0`, and a NULL-id row; assert ONE `ItemRef` per distinct
held id with the `MIN(name)` representative, ordered.

**New cases the union test MUST add (the D-04 guards):**
1. A `pigparse_price` row whose name is NOT held → appears in the union keyed by its PigParse id.
2. A `pigparse_price` row whose name IS held (same `lower(trim(name))`) → appears ONCE, keyed by the held EQ id (held wins; no duplicate row).
3. **The collision case:** a `pigparse_price.item_id` numerically equal to a held EQ id (`item_master` already has that id for a different name) → the catalog row is EXCLUDED (the `NOT IN (SELECT item_id FROM item_master)` guard), so the held row is never overwritten.
4. Assert exactly one ref per normalized name after the full union (Pitfall 1 regression).

---

### `enrich/jobs/wiki_test.go` — widen seeding + assert catalog coverage

**Analog:** `seedAllItemRefs` / `TestRunWiki_PopulatesAllTables` (`jobs/wiki_test.go:108-114,
118-126`) and the icon-backfill regression `TestRunWiki_BackfillsStaleIcon`
(`jobs/wiki_test.go:196-238`).

**Scaffolding to reuse:**
- `newWikiFixtureServer(t, wikiServerOpts{})` — serves `../testdata/<fixture>.json` per `?page=`; unknown page → MediaWiki error envelope (`jobs/wiki_test.go:48-78`).
- `serverFetcher(srv)` — rewrites the wiki URL to the httptest server (`jobs/wiki_test.go:81-92`).
- `setWikiSleepNoop()` — instant no-op sleep so the run is fast (`jobs/wiki.go:73-77`; used at `jobs/wiki_test.go:119-120, 197-198`).
- `seedInvFor` / `seedOwnerChar` for held refs (`jobs/wiki_test.go:96-114`).

**New for Phase 38:** also seed `pigparse_price` rows (a catalog-only item with a wiki
fixture + a junk/no-page catalog name) so the test proves the widened crawl enriches an
UNHELD catalog item and that the junk name lands in the icon-less residue. Assert the catalog
item now has an `item_master` row with a non-zero `icon_id` (the ENRICH-15 win condition).
The icon-self-heal mechanism is already proven by `TestRunWiki_BackfillsStaleIcon`
(`jobs/wiki_test.go:196-238`) — mirror its read-back pattern
(`SELECT icon_id FROM item_master WHERE item_id=?`).

---

## Shared Patterns

### Single tested SQL path (11-05) — applies to the new union read
**Source:** `store/itemids.go:9-14`, `store/enrich.go:3-23`
**Apply to:** the `DistinctEnrichmentRefs` SELECT (lives in `store/`) — the job authors
ZERO inline SQL; it calls the store method and composes the existing `*Tx` writers over one
tx. The new union is the only SQL added this phase.

### Namespace-agnostic upsert + freshness short-circuit — reuse verbatim
**Source:** `store/enrich.go:234-294` (`UpsertItemMasterTx` `ON CONFLICT(item_id)`,
`GetItemMasterFreshnessTx` returning `sha, iconID, statsblock, flagsJSON`)
**Apply to:** every ref in the widened loop — works for an EQ id or a PigParse id identically;
the icon already participates in the comparison (`store/enrich.go:282-294`). Do NOT modify.
**Guard:** `ON CONFLICT(item_id) DO UPDATE` (`store/enrich.go:208`) is why the union's
EQ-id-collision exclusion is load-bearing — without it a colliding PigParse id silently
overwrites a held row's enrichment.

### Structured logging (V7) — applies to the D-03 coverage line
**Source:** `store/enrich.go:22-23`; CLAUDE.md "Conventions" (`slog` with op + key/value)
**Apply to:** the coverage summary — counts, ids, and public wiki page NAMES only; bound the
residue sample slice; NEVER log statsblock/wikitext bodies.

### Extend-only / goose-on-boot, no watcher gate
**Source:** `migrations/00012_item_icon.sql:6-10` (the "no `WatcherMaxSchemaVersion`,
goose `version()` is the version of record, `internal/sheet` gone post-v2.0" note)
**Apply to:** confirms NO 00017, NO `WatcherMaxSchemaVersion` bump, NO `v*` tag this phase
(Option A needs no schema change). The CLAUDE.md `WatcherMaxSchemaVersion` reference is STALE.

### ctx-aware courtesy pacing — already in the loop
**Source:** `jobs/wiki.go:53-77, 168-172` (`wikiSleepFn` / `realWikiSleep`)
**Apply to:** the widened set inherits the 1 req/s pace unchanged (~72-min seed run is fine
per D-01); never substitute a bare `time.Sleep`.

---

## No Analog Found

None. Every change has an exact in-package analog (the union read mirrors
`DistinctInventoryItemIDs` + `pp_rep`; the loop swap is in-place; the upsert/freshness/icon
paths are reused verbatim; the tests mirror sibling tests). Zero new dependency, zero new
package, zero new table.

---

## Planner Action Item (carried from RESEARCH, NOT a pattern)

Run the D-04 prod probe during planning to size the EQ-id-collision exclusion (RESEARCH A2 /
Open Question 2) — how many catalog ids collide with a held EQ id for a *different* name:
```sql
SELECT count(*) FROM pigparse_price p
JOIN item_master m ON p.item_id = m.item_id
WHERE lower(trim(p.name)) <> lower(trim(m.name));
```
Expected ~0 → the exclusion is harmless (those few names simply stay icon-less, surfaced in
the D-03 residue). If material → escalate to a name-keyed fallback for just that set (still no
new table). This is a read-only probe against the prod SQLite (Hetzner VPS), not a code change.

---

## Metadata

**Analog search scope:** `internal/backendsrv/store`, `internal/backendsrv/enrich/jobs`,
`internal/backendsrv/enrich`, `internal/backendsrv/compute`, `internal/backendsrv/migrations`
**Files read for excerpts:** `store/itemids.go`, `store/itemids_test.go`, `store/enrich.go`,
`store/readviews.go`, `store/readviews_test.go`, `enrich/jobs/wiki.go`, `enrich/jobs/wiki_test.go`,
`enrich/wikiitem.go` (parseIconID), `compute/itemrollup.go`, `migrations/00001_init.sql`,
`migrations/00012_item_icon.sql`
**Pattern extraction date:** 2026-06-25
