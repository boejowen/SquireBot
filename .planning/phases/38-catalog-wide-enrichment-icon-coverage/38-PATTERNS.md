# Phase 38: Catalog-wide enrichment + icon coverage - Pattern Map

**Mapped:** 2026-06-25 (RE-PLAN — D-04 reversed id-keyed → NAME-KEYED; OVERWRITES the stale Option-A map)
**Files analyzed:** 6 (1 NEW migration, 1 NEW store file, 2 MODIFIED store/job files, 3 test files: 1 MODIFIED + 2 NEW + 1 MODIFIED)
**Analogs found:** 6 / 6 (every new/modified file has an exact in-repo template — this phase is reuse, not invention)

> **Why this overwrites the prior map.** The previous 38-PATTERNS.md mapped Option A (catalog rows id-keyed into `item_master` behind a collision guard). The prod probe found 43 dropped catalog items, the user held the deploy, and D-04 was reversed to **name-keyed Option B**: a separate `catalog_enrichment(norm_name TEXT PRIMARY KEY, …)` table (migration 00017), `item_master` stays held-only, the write path branches by held-ness. This map re-points every file to the name-keyed templates. The Option-A code (`DistinctEnrichmentRefs` collision guard, the unbranched `runWikiItems` write, the single-store `ItemMasterIconCoverage`, `TestRunWiki_EnrichesUnheldCatalogItem` asserting an `item_master` row) will be **REPLACED**.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00017_catalog_enrichment.sql` | migration | DDL / batch | `00001_init.sql` (CREATE TABLE shape) + `00016_item_flags_effects.sql` (additive header) + `00012_item_icon.sql` (nullable-col + "no WatcherMaxSchemaVersion" header) | exact (additive CREATE TABLE) |
| `internal/backendsrv/store/catalogenrich.go` (NEW; or appended to `enrich.go`) | store / model | CRUD upsert + freshness read | `store/enrich.go` `ItemMaster` struct + `itemMasterUpsert` + `UpsertItemMasterTx` + `GetItemMasterFreshnessTx` | exact (re-key item_id → norm_name) |
| `internal/backendsrv/store/itemids.go` (MODIFIED) | store | request-response read (union + coverage) | itself (`DistinctEnrichmentRefs`, `ItemMasterIconCoverage`) — drop guard, add `Held`, both-stores coverage | self-modify |
| `internal/backendsrv/enrich/jobs/wiki.go` (MODIFIED) | service / job | event-driven (weekly scheduler) | itself (`runWikiItems` / `upsertItemAndQuests` / `logItemsCoverage`) — branch the write | self-modify |
| `internal/backendsrv/store/itemids_test.go` (MODIFIED) | test | — | itself (`TestDistinctEnrichmentRefs` Case C assertion FLIPS; `TestItemMasterIconCoverage`→both-stores) | self-modify |
| `internal/backendsrv/store/catalogenrich_test.go` (NEW) | test | — | `store/enrich_test.go` `TestUpsertItemMaster_AndSHA1Getter` + `TestMarshalFlags` | exact (mirror by norm_name) |
| `internal/backendsrv/enrich/jobs/wiki_test.go` (MODIFIED) | test | — | itself (`TestRunWiki_EnrichesUnheldCatalogItem` — assertion moves item_master → catalog_enrichment) | self-modify |
| `internal/backendsrv/migrations/migrate_test.go` (MODIFIED) | test | — | `TestMigrate_00016_AddsItemFlagsEffects` (the table/column + idempotency template) | exact (new `TestMigrate_00017_AddsCatalogEnrichment`) |

> Note: the planner may fold the catalog upsert/freshness methods into the existing `store/enrich.go` instead of a new `catalogenrich.go` (same package, RESEARCH leaves it to the planner). Either way the templates below are identical.

## Pattern Assignments

### `internal/backendsrv/migrations/00017_catalog_enrichment.sql` (migration, additive CREATE TABLE)

**Analogs:** `00001_init.sql` (CREATE TABLE shape, `migrations/00001_init.sql:59-63`) · `00016_item_flags_effects.sql` (additive forward-only header + flags_json comment) · `00012_item_icon.sql` (the canonical "no WatcherMaxSchemaVersion gate / goose version() is the version of record" header block).

**Goose Up/Down envelope + the additive header to copy** (`00012_item_icon.sql:1-14`; same shape in `00016`):
```sql
-- +goose Up
-- Phase 31 (INV-04, D-01/D-02/D-03). Forward-only; 00001-00011 are shipped and NOT
-- edited. SQLite permits only ONE column per ALTER TABLE ADD COLUMN; the added
-- column is nullable (no DEFAULT needed) and carries no UNIQUE/PK constraint
-- (the 00003 pattern). ... Read-only additive column: the watcher is OFF the read
-- path (untouched this phase), so NO WatcherMaxSchemaVersion gate is touched and
-- goose version() is the version of record (no _meta.schema_version cell exists in
-- this backend).
ALTER TABLE item_master ADD COLUMN icon_id INTEGER;

-- +goose Down
ALTER TABLE item_master DROP COLUMN icon_id;
```

**CREATE TABLE / DROP TABLE shape to copy** (`00001_init.sql:59` shows the one-line `CREATE TABLE item_master (...)` enrichment shape; `00001_init.sql:66-68` shows the `-- +goose Down` `DROP TABLE` form). 00017 is the SAME enrichment column set as `item_master` re-PK'd on `norm_name`. The exact column set is dictated by RESEARCH §"D-04 RESOLUTION" (lines 139-160) and MUST match `itemMasterUpsert` (`store/enrich.go:204-216`) so the new upsert maps 1:1:
```sql
-- +goose Up
-- Phase 38 (ENRICH-14/15, D-04 name-keyed). Forward-only; 00001-00016 are shipped and NOT
-- edited. catalog_enrichment holds wiki enrichment for CATALOG-ONLY (unheld) items, keyed
-- by normalized name lower(trim(name)) — the cross-namespace bridge (PigParse catalog ids
-- != EQ inventory ids; join by name, never raw item_id). A held item's enrichment stays in
-- item_master keyed by its EQ item_id; this table holds ONLY names with no held item_master
-- row. Read-only additive table: the watcher is OFF the read path, so NO WatcherMaxSchemaVersion
-- gate is touched and goose version() is the version of record.
CREATE TABLE catalog_enrichment (
  norm_name      TEXT PRIMARY KEY,             -- lower(trim(name)) — the cross-namespace key
  name           TEXT,
  item_id        INTEGER,                      -- representative PigParse id (examine/icon URL); NOT a key
  wiki_summary   TEXT,
  wiki_url       TEXT,
  slot           TEXT,
  is_quest_item  INTEGER NOT NULL DEFAULT 0,
  wikitext_sha1  TEXT,
  icon_id        INTEGER,                       -- lucy_img_ID; NULL/0 = colored-tile fallback (00012 contract)
  statsblock     TEXT,
  is_lore        INTEGER,
  is_no_drop     INTEGER,
  is_magic       INTEGER,
  is_temporary   INTEGER,
  is_clicky      INTEGER,
  clicky_effect  TEXT,
  has_haste      INTEGER,
  haste_pct      INTEGER,
  flags_json     TEXT,                          -- full flag SET as a JSON array (no future per-flag migration — 00016 precedent)
  last_refreshed TEXT
);

-- +goose Down
DROP TABLE catalog_enrichment;
```

**Convention notes:** additive only (new table, NO ALTER of `item_master`/`pigparse_price`); goose-on-boot (`cmd/squirebot-server/main.go:223 RunMigrations`); no `WatcherMaxSchemaVersion` bump (stale post-v2.0); no `v*` tag.

---

### `internal/backendsrv/store/catalogenrich.go` (store/model, CRUD upsert + freshness read) — NEW

**Analog:** `internal/backendsrv/store/enrich.go` — the `ItemMaster` struct (`:87-114`), `itemMasterUpsert` const (`:204-216`), `UpsertItemMasterTx` (`:234-248`), `GetItemMasterFreshnessTx` (`:267-294`), `MarshalFlags` (`:34-60`). The new methods are these re-keyed on `norm_name` instead of `item_id`. `b2i` already exists package-local at `store/notifyprefs.go:78`.

**File-header discipline to copy** (`store/enrich.go:3-23` — the single-tested-SQL-path + V5 placeholder + V7 logging contract):
```go
// ... the enrichment jobs author ZERO inline DELETE/INSERT/UPDATE SQL — they call
// the exported *Tx methods here and compose them over one *sql.Tx ...
// Every value is bound through a ? placeholder (V5 / Tampering): parsed item
// names, wikitext, and quest names are UNTRUSTED and are NEVER string-concatenated
// into a SQL literal. slog logs counts / ids / err only — never raw content (V7).
```

**Input struct to copy/adapt** — a `CatalogEnrichment` mirroring `ItemMaster` (`store/enrich.go:87-114`) with a `NormName string` PK + the representative `Name`/`ItemID` (RESEARCH §"D-04 RESOLUTION", lines 175-202). Same fields otherwise: `WikiSummary, WikiURL, Slot, IsQuestItem, WikitextSHA1, LastRefreshed, IconID, Statsblock, IsLore, IsNoDrop, IsMagic, IsTemporary, IsClicky, ClickyEffect, HasHaste, HastePct, FlagsJSON`.

**Upsert const to copy** (`store/enrich.go:204-216` — re-key `ON CONFLICT(item_id)` → `ON CONFLICT(norm_name)`, lead with `norm_name`):
```go
const itemMasterUpsert = `INSERT INTO item_master
	(item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed, icon_id, statsblock,
	 is_lore, is_no_drop, is_magic, is_temporary, is_clicky, clicky_effect, has_haste, haste_pct, flags_json)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(item_id) DO UPDATE SET
   name=excluded.name, wiki_summary=excluded.wiki_summary, wiki_url=excluded.wiki_url,
   slot=excluded.slot, is_quest_item=excluded.is_quest_item,
   wikitext_sha1=excluded.wikitext_sha1, last_refreshed=excluded.last_refreshed,
   icon_id=excluded.icon_id, statsblock=excluded.statsblock,
   is_lore=excluded.is_lore, is_no_drop=excluded.is_no_drop, is_magic=excluded.is_magic,
   is_temporary=excluded.is_temporary, is_clicky=excluded.is_clicky,
   clicky_effect=excluded.clicky_effect, has_haste=excluded.has_haste,
   haste_pct=excluded.haste_pct, flags_json=excluded.flags_json`
```

**`UpsertCatalogEnrichmentTx` body to copy** (`store/enrich.go:234-248` `UpsertItemMasterTx` — the `b2i(...)` boolean binding + the `?`-placeholder UNTRUSTED-text discipline; swap `item.ItemID` → `e.NormName` as the keyed value, add `e.Name`/`e.ItemID` as columns):
```go
func UpsertItemMasterTx(ctx context.Context, tx *sql.Tx, item ItemMaster) error {
	if _, err := tx.ExecContext(ctx, itemMasterUpsert,
		item.ItemID, item.Name, item.WikiSummary, item.WikiURL, item.Slot,
		b2i(item.IsQuestItem), item.WikitextSHA1, item.LastRefreshed, item.IconID, item.Statsblock,
		b2i(item.IsLore), b2i(item.IsNoDrop), b2i(item.IsMagic), b2i(item.IsTemporary),
		b2i(item.IsClicky), item.ClickyEffect, b2i(item.HasHaste), item.HastePct, item.FlagsJSON,
	); err != nil {
		slog.Error("item_master upsert: insert", "item_id", item.ItemID, "err", err)
		return fmt.Errorf("upsert item_master (item_id=%d): %w", item.ItemID, err)
	}
	return nil
}
```

**`GetCatalogEnrichmentFreshnessTx` body to copy** (`store/enrich.go:267-294` `GetItemMasterFreshnessTx` — returns `(sha string, iconID int64, statsblock, flagsJSON string, err error)`; the 4-field self-heal contract in the docstring; `sql.ErrNoRows`→zero values; swap `WHERE item_id = ?` → `WHERE norm_name = ?`):
```go
func GetItemMasterFreshnessTx(ctx context.Context, tx *sql.Tx, itemID int64) (sha string, iconID int64, statsblock, flagsJSON string, err error) {
	var s, sb, fj sql.NullString
	var icon sql.NullInt64
	qerr := tx.QueryRowContext(ctx,
		`SELECT wikitext_sha1, icon_id, statsblock, flags_json FROM item_master WHERE item_id = ?`, itemID).Scan(&s, &icon, &sb, &fj)
	switch {
	case qerr == sql.ErrNoRows:
		return "", 0, "", "", nil
	case qerr != nil:
		return "", 0, "", "", fmt.Errorf("read item_master freshness (item_id=%d): %w", itemID, qerr)
	}
	return s.String, icon.Int64, sb.String, fj.String, nil
}
```

**flags encoding:** the caller (job) MUST use `store.MarshalFlags(item.Flags)` (`store/enrich.go:34-60`) for `FlagsJSON`, NOT a local `json.Marshal` — so a flagless item stores `"[]"` and byte-matches the freshness compare (D-06 idempotency). The job computes `norm := strings.ToLower(strings.TrimSpace(item.ItemName))` (the existing idiom at `store/enrich.go:331` and `compute/itemrollup.go:69`).

**Do NOT add** a `ReplaceQuestItemsForIDTx`-style quest write for the catalog branch (RESEARCH §"Quest links for catalog-only items", lines 228-237): `quest_items` is the EQ namespace, id-joined; the catalog branch SKIPS it.

---

### `internal/backendsrv/store/itemids.go` (store, MODIFIED — drop guard, add `Held`, both-stores coverage)

**Analog:** itself. Two edits.

**Edit 1 — `DistinctEnrichmentRefs` union (`:100-142`): DROP the Option-A collision guard, ADD a `Held` flag.** The current shipped union (the modify target):
```go
catalog AS (
   SELECT MIN(item_id) AS item_id, MIN(name) AS name
   FROM pigparse_price
   WHERE name IS NOT NULL AND trim(name) <> ''
     AND lower(trim(name)) NOT IN (SELECT norm FROM held_names)
     AND item_id NOT IN (SELECT item_id FROM item_master)   -- <<< DROP THIS LINE (itemids.go:118)
   GROUP BY lower(trim(name))
 )
 SELECT item_id, name FROM held
 UNION ALL
 SELECT item_id, name FROM catalog
 ORDER BY item_id
```
Re-plan target (drop the `item_id NOT IN (item_master)` line; both arms emit `held`; RESEARCH §"Revised union sketch", lines 278-300):
```sql
SELECT item_id, name, 1 AS held FROM held
UNION ALL
SELECT item_id, name, 0 AS held FROM catalog
ORDER BY item_id;
```
**Ref-shape change (RESEARCH Open Question 3 / lines 263-268):** add a dedicated `EnrichmentRef{ItemID int64; Name string; Held bool}` struct rather than extending the shared `ItemRef` (`itemids.go:25-28`, also used by `DistinctInventoryItemIDs` — extending it ripples into its tests). KEEP the held-name dedup (`lower(trim(name)) NOT IN held_names`, `:108-117`), the catalog `GROUP BY lower(trim(name))` (`:119`), and the blank-name guard `trim(name) <> ''` (`:116`). Also update the docstring (`:86-99`) — exclusion #2 (the id-collision guard) is REMOVED and the keying note becomes "catalog-only refs go to catalog_enrichment by norm_name, not item_master."

**Edit 2 — `ItemMasterIconCoverage` → both-stores `CatalogIconCoverage` (`:163-197`).** The current shipped single-store coverage (the modify target):
```go
func (s *Store) ItemMasterIconCoverage(ctx context.Context, sampleCap int) (IconCoverage, error) {
	var cov IconCoverage
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*),
		        COALESCE(SUM(CASE WHEN icon_id IS NOT NULL AND icon_id > 0 THEN 1 ELSE 0 END), 0)
		 FROM item_master`,
	).Scan(&cov.Total, &cov.IconCovered); err != nil {
		return IconCoverage{}, fmt.Errorf("query item_master icon coverage: %w", err)
	}
	cov.IconLess = cov.Total - cov.IconCovered
	if sampleCap <= 0 || cov.IconLess == 0 {
		return cov, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM item_master
		 WHERE (icon_id IS NULL OR icon_id = 0) AND name IS NOT NULL AND trim(name) <> ''
		 ORDER BY name
		 LIMIT ?`, sampleCap)
	// ... scan loop into cov.ResidueSample ...
}
```
Re-plan target — wrap BOTH the count and the residue queries in a `UNION ALL` over `item_master` + `catalog_enrichment`, deduped by normalized name (RESEARCH §"D-03 Coverage Diagnostic", lines 411-426; held names are never in catalog_enrichment so a plain `UNION ALL` already yields one row per item). KEEP the `IconCoverage` struct (`:149-154`) and the `sampleCap` self-DoS bound (`:159-162`, the T-38-04 guard) unchanged:
```sql
SELECT count(*), COALESCE(SUM(CASE WHEN icon_id IS NOT NULL AND icon_id > 0 THEN 1 ELSE 0 END), 0)
FROM (
  SELECT lower(trim(name)) AS norm, icon_id FROM item_master
  UNION ALL
  SELECT norm_name AS norm, icon_id FROM catalog_enrichment
);
-- residue sample (bounded, PUBLIC names only, name-ordered):
SELECT name FROM (
  SELECT name, icon_id FROM item_master WHERE name IS NOT NULL AND trim(name) <> ''
  UNION ALL
  SELECT name, icon_id FROM catalog_enrichment WHERE name IS NOT NULL AND trim(name) <> ''
)
WHERE icon_id IS NULL OR icon_id = 0
ORDER BY name LIMIT ?;
```

---

### `internal/backendsrv/enrich/jobs/wiki.go` (service/job, MODIFIED — branch the write)

**Analog:** itself. The loop body (`runWikiItems`, `:160-234`) and the courtesy sleep / ETag 304 / log-and-skip resilience stay byte-for-byte; only the ref iteration (now `EnrichmentRef` with `Held`) and the per-item write (`upsertItemAndQuests`, `:261-332`) change.

**The freshness short-circuit + held write to copy/branch** (`wiki.go:268-309` — the 4-field compare is the template both branches use):
```go
existingSHA, existingIcon, existingStats, existingFlagsJSON, err := store.GetItemMasterFreshnessTx(ctx, tx, ref.ItemID)
if err != nil {
	return false, err
}
parsedFlagsJSON := store.MarshalFlags(item.Flags)   // the ONE canonical encoder (D-06)
if existingSHA == item.WikitextSHA1 && existingIcon == int64(item.IconID) &&
	existingStats == item.Statsblock && existingFlagsJSON == parsedFlagsJSON {
	return false, nil   // unchanged — icon already compared
}
if err := store.UpsertItemMasterTx(ctx, tx, store.ItemMaster{ /* ... */ IconID: item.IconID, FlagsJSON: parsedFlagsJSON /* ... */ }); err != nil {
	return false, err
}
```
Re-plan target (RESEARCH §"Code Examples", lines 587-602): branch `upsertItemAndQuests` strictly on `ref.Held`.
- `ref.Held == true` → today's tx UNCHANGED: `GetItemMasterFreshnessTx(ctx, tx, ref.ItemID)` → 4-field compare → `UpsertItemMasterTx` + `ReplaceQuestItemsForIDTx` (`:314-326`).
- `ref.Held == false` → NEW tx: `norm := strings.ToLower(strings.TrimSpace(item.ItemName))` → `GetCatalogEnrichmentFreshnessTx(ctx, tx, norm)` → the SAME 4-field compare → `UpsertCatalogEnrichmentTx(ctx, tx, store.CatalogEnrichment{NormName: norm, ItemID: int(ref.ItemID), Name: item.ItemName, IconID: item.IconID, FlagsJSON: parsedFlagsJSON, /* ... */})`. **NO `quest_items` write.**

**The coverage log to re-point** (`wiki.go:227-255`). The call site (`:227`) currently passes `s.ItemMasterIconCoverage`; re-point to the both-stores `CatalogIconCoverage`. `logItemsCoverage` (`:245-255`) keeps its structure — the keys can stay or be renamed `total`/`item_master_total` (RESEARCH §"Code Examples", lines 616-623). PUBLIC names only, never statsblock/wikitext bodies (V7, `:243-244`):
```go
func logItemsCoverage(unionSize, written, unchanged int, cov store.IconCoverage) {
	slog.Info(wikiJobName+": items coverage",
		"union_size", unionSize, "written", written, "unchanged", unchanged,
		"item_master_total", cov.Total, "icon_covered", cov.IconCovered, "icon_less", cov.IconLess,
		"residue_sample", cov.ResidueSample,
	)
}
```

**Anti-patterns (RESEARCH §"Anti-Patterns to Avoid", lines 501-517):** do NOT id-key catalog rows into `item_master`; do NOT add a synthetic/offset id; do NOT write catalog quest links to `quest_items`; do NOT add a boot `BackfillCatalogEnrichment` pass (the crawl is the only source — `catalog_enrichment` is born populated); do NOT leave `ItemMasterIconCoverage` reading one table.

---

### `internal/backendsrv/store/itemids_test.go` (test, MODIFIED — Case C assertion FLIPS; coverage→both-stores)

**Analog:** itself + the `seedItemMaster`/`seedPigparse`/`seedRaw` helpers already present (`itemids_test.go:17` `seedRaw`; package-shared `seedItemMaster`/`seedPigparse` at `readviews_test.go:30,48`).

**`TestDistinctEnrichmentRefs` Case C — the assertion that must FLIP** (currently `itemids_test.go:194-197`). Today it asserts the id-collision catalog row is EXCLUDED:
```go
// Case C: the id-collision catalog row "Colliding Catalog Name" is EXCLUDED.
if _, ok := idByName["Colliding Catalog Name"]; ok {
	t.Errorf("catalog row whose id (1001) collides with an item_master row leaked into the union — collision guard broken")
}
```
Re-plan target (RESEARCH §"Pitfall 1", lines 537-545): this FLIPS to assert the same row is now INCLUDED with `Held == false` (the guard is gone; name-keying makes the id-collision irrelevant). KEEP Cases A/B/D and `TestDistinctEnrichmentRefs_HeldVsHeldSameName` (`:221-245`). With the `EnrichmentRef{...Held bool}` shape, assertions also check `ref.Held` (held names true, catalog names false — Pitfall 2). NB: the `Case C` setup (`:128-132`) seeds `seedItemMaster(1001,...)` + a colliding `seedPigparse(1001,...)`; under name-keying that catalog row must surface (its name is unheld), so the seed stays and the assertion inverts.

**`TestItemMasterIconCoverage` → both-stores** (currently seeds only `item_master`, `:251-310`). Re-plan target: seed BOTH `item_master` and `catalog_enrichment` rows and assert `Total`/`IconCovered`/`IconLess`/`ResidueSample` span both (RESEARCH §"Pitfall 5", lines 571-576). The existing assertion structure (covered/less counts + name-ordered bounded residue + cap-honored, `:279-309`) is the template.

---

### `internal/backendsrv/store/catalogenrich_test.go` (test, NEW)

**Analogs:** `store/enrich_test.go` `TestUpsertItemMaster_AndSHA1Getter` (`:133-197`) for the upsert/round-trip/update-in-place shape + the `withTx` helper (`enrich_test.go:352-360`) for exercising `*Tx` getters; `TestMarshalFlags` (`enrich_test.go:120-131`) for the flags-encoding contract reuse.

**Upsert + round-trip test shape to copy** (`enrich_test.go:133-197`): seed a `CatalogEnrichment`, `UpsertCatalogEnrichmentTx` (or the `Store.X` wrapper), assert the row exists by `norm_name`, assert booleans store as 0/1, assert `GetCatalogEnrichmentFreshnessTx` returns the stored `(sha, icon, stats, flags)` and zero-values for an absent norm_name, then re-upsert the same norm_name and assert update-in-place (still one row). The `TestUpsertItemMaster_AndSHA1Getter` skeleton:
```go
func TestUpsertItemMaster_AndSHA1Getter(t *testing.T) {
	db := NewTestDB(t); s := NewStore(db); ctx := context.Background()
	item := ItemMaster{ItemID: 13128, Name: "Fungus Covered Scale Tunic", /* ... */ WikitextSHA1: "abc123def456", LastRefreshed: lastRefreshed}
	if err := s.UpsertItemMaster(ctx, item); err != nil { t.Fatalf("UpsertItemMaster: %v", err) }
	// ... assert is_quest_item stored as 1 ...
	if err := withTx(t, db, func(tx *sql.Tx) error { /* GetItemMasterSHA1Tx present-row + absent-row */ return nil }); err != nil { /* ... */ }
	// ... re-upsert same id with new sha → still one row, updated ...
}
```

**Freshness self-heal test** to mirror — a row written with `icon_id=0`/`flags_json="[]"` then re-upserted with a real icon/flags must re-write (the 4-field compare, RESEARCH lines 217-220). Use `withTx` (`enrich_test.go:352-360`) to call `GetCatalogEnrichmentFreshnessTx` directly. Reuse `MarshalFlags` for the expected `flags_json` (`enrich_test.go:120-131`).

---

### `internal/backendsrv/migrations/migrate_test.go` (test, MODIFIED — add `TestMigrate_00017_AddsCatalogEnrichment`)

**Analog:** `TestMigrate_00016_AddsItemFlagsEffects` (`migrate_test.go:1140-1204`) and `TestMigrate_00012_AddsItemIcon` (`:1061-1116`). The exact template: `store.NewTestDB(t)` migrates to HEAD; `tableExists`/`columnSet` (`:56-69,215-242`) assert `catalog_enrichment` exists with its column set; a row inserted WITH/WITHOUT a column round-trips; the idempotency tail (re-run `RunMigrations`, assert `goose_db_version` row count unchanged, `:1188-1203`) is copy-paste. The 00016 skeleton:
```go
func TestMigrate_00016_AddsItemFlagsEffects(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00016) + t.Cleanup
	cols := columnSet(t, db, "item_master")
	for _, c := range itemFlagsEffectsColumns {
		if !cols[c] { t.Errorf("expected item_master to have column %q after 00016 ...", c) }
	}
	// ... round-trip insert WITH/WITHOUT columns ...
	// idempotency: second RunMigrations is a no-op; goose_db_version row count unchanged ...
}
```
Re-plan target: a new `TestMigrate_00017_AddsCatalogEnrichment` asserting `tableExists(t, db, "catalog_enrichment")`, the full column set via `columnSet`, a name-keyed insert round-trip + an `ON CONFLICT(norm_name)` upsert keeping one row (mirroring the `ec_auction_cursor` PK-upsert proof at `:560-577`), and the idempotency tail. Also extend `dimensionTables`/`allTables` (`migrate_test.go:21-29`) if the planner wants `catalog_enrichment` in the "created empty" set (it IS created empty — populated by the crawl, so a `TestDimensionTables_Empty`-style 0-row assertion holds, `:109-124`).

---

### `internal/backendsrv/enrich/jobs/wiki_test.go` (test, MODIFIED — assertion moves item_master → catalog_enrichment)

**Analog:** itself. `TestRunWiki_EnrichesUnheldCatalogItem` (`:324-382`) and its `seedCatalogItem` helper (`:308-317`) already exist and already seed an unheld `pigparse_price` row + a junk no-page row. Today it asserts the unheld item lands in `item_master` keyed by its PigParse id:
```go
// Option A id-keying: the catalog-only row is keyed by its PigParse catalog id ...
if err := db.QueryRow(
	`SELECT item_id, name, icon_id FROM item_master WHERE lower(trim(name)) = lower(trim(?))`,
	"Cloak of Flames",
).Scan(&gotID, &name, &icon); err != nil {
	t.Fatalf("unheld catalog item Cloak of Flames has no item_master row after the widened crawl: %v", err)
}
if gotID != cofCatalogID { t.Errorf("... want PigParse catalog id %d (Option A id-keying)", cofCatalogID) }
```
Re-plan target: the unheld item must now appear in **`catalog_enrichment` by `norm_name`** (NOT `item_master`), with a non-zero `icon_id`; and ASSERT it is NOT in `item_master` (Pitfall 2 — a catalog ref must not route to the held store). The held Cloth Cap (1001) assertion stays in `item_master` (`:371-378`). The "junk name does not abort the run / status ok" assertion (`:380-381 assertJobStatus(... "ok")`) is unchanged. `TestRunWiki_BackfillsStaleIcon` (`:196-238`) and `TestRunWiki_BackfillsStaleFlags` (`:247-302`) stay UNCHANGED — they exercise the held path, which is byte-for-byte preserved.

---

## Shared Patterns

### Single tested SQL path (11-05 / WARNING-3)
**Source:** `store/enrich.go:3-23` (header) + `store/itemids.go:9-14`.
**Apply to:** `store/catalogenrich.go`, `store/itemids.go`, `enrich/jobs/wiki.go`.
The job authors ZERO inline DELETE/INSERT/UPDATE SQL — all new SQL (the upsert, the freshness getter, the union, the coverage) lives in `store/` with tests; the job composes the exported `*Tx`/`*` methods over one tx.

### V5 placeholder discipline (Tampering — untrusted wiki text)
**Source:** `store/enrich.go:21-23,234-247`.
**Apply to:** `UpsertCatalogEnrichmentTx`.
Every UNTRUSTED parsed value (`name`, `wiki_summary`, `slot`, `clicky_effect`, `flags_json`, `statsblock`, and the `norm_name` itself) binds through a `?` placeholder — NEVER string-concatenated. Booleans bind as `b2i(...)` 0/1 (`store/notifyprefs.go:78`).

### Canonical flags encoding (D-06 idempotency)
**Source:** `store/MarshalFlags`, `store/enrich.go:34-60` (+ its contract test `enrich_test.go:120-131`).
**Apply to:** the job's catalog branch (`FlagsJSON = store.MarshalFlags(item.Flags)`) and the freshness compare.
A flagless item must store `"[]"` (never `null`/`""`) so the weekly freshness compare byte-matches and does not re-write every pass.

### Freshness 4-field self-heal (icon carried for free)
**Source:** `store/enrich.go:267-294` (`GetItemMasterFreshnessTx`) + `enrich/jobs/wiki.go:268-286` (the compare).
**Apply to:** `GetCatalogEnrichmentFreshnessTx` + the catalog write branch.
Re-write whenever `sha OR icon OR statsblock OR flags_json` differs — so an `icon_id`/`flags_json` written before backfill self-heals on the next weekly pass (the 00012 icon argument, by name).

### Extend-only / goose-on-boot / no-watcher-gate migration header
**Source:** `migrations/00012_item_icon.sql:1-14`, `migrations/00016_item_flags_effects.sql:1-18`.
**Apply to:** `00017_catalog_enrichment.sql`.
Additive only; the "watcher is OFF the read path, NO WatcherMaxSchemaVersion gate, goose version() is the version of record" header is copy-paste; no `v*` tag.

### Migration test template (table/column + idempotency)
**Source:** `migrate_test.go` `TestMigrate_00016_AddsItemFlagsEffects` (`:1140-1204`), helpers `tableExists`/`columnSet`/`indexExists` (`:56-69,215-242,279-307`), idempotency tail (`:1188-1203`).
**Apply to:** `TestMigrate_00017_AddsCatalogEnrichment`.

### Structured slog, public names only (V7)
**Source:** `enrich/jobs/wiki.go:243-255`, `store/enrich.go:22-23`.
**Apply to:** `logItemsCoverage` (both-stores). Counts/ids/err + bounded PUBLIC item names only — NEVER statsblock/wikitext bodies; residue sample capped at `residueSampleCap = 50` (`wiki.go:236-238`).

## No Analog Found

None. Every file in this phase has an exact in-repo template — this is the explicit "resist inventing new shapes" insight from RESEARCH §"Don't Hand-Roll" (lines 519-533). The single genuinely-new artifact, the `catalog_enrichment` table, is the `item_master` enrichment shape re-keyed on `norm_name`, and even its CREATE/test/upsert/freshness all clone existing files.

## Metadata

**Analog search scope:** `internal/backendsrv/{migrations,store,enrich/jobs,enrich}` + the matching `*_test.go` files.
**Files scanned (read in full or targeted):** `store/enrich.go`, `store/itemids.go`, `store/itemsearch.go`, `store/enrich_test.go`, `store/itemids_test.go`, `store/notifyprefs.go` (b2i), `enrich/jobs/wiki.go`, `enrich/jobs/wiki_test.go`, `migrations/00001_init.sql`, `migrations/00012_item_icon.sql`, `migrations/00016_item_flags_effects.sql`, `migrations/migrate_test.go`.
**Pattern extraction date:** 2026-06-25

## PATTERN MAPPING COMPLETE
