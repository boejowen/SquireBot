---
phase: 31-characters-tab-in-game-inventory-window
plan: 01
subsystem: database
tags: [go, sqlite, goose, enrich, wiki, compute-on-read, json-contract, icon-id]

# Dependency graph
requires:
  - phase: 29-data-foundation-inventory-parse-price-value-aggregation
    provides: compute.StructuredInventory + InventorySlot/CharacterInventory JSON contract + InventoryForChar store read + the EQ-namespace im.item_id=ii.item_id join
provides:
  - "item_master.icon_id column (extend-only migration 00012) populated by the weekly wiki job from lucy_img_ID"
  - "InventorySlot.IconID (json:\"icon_id\") flowing through StructuredInventory via the id join"
  - "CharacterInventory.LastSeen (json:\"last_seen\") — the per-character examine 'Last synced' carrier"
affects: [31-02, 31-03, 31-04, 33-banks-tab, 34-wishlist-rework]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Extend-only icon enrichment: one nullable ADD COLUMN + one pure parser line + the existing upsert path (zero new fetch/tx)"
    - "Append-only JSON-contract growth: new snake_case tags on InventorySlot/CharacterInventory, no existing tag renamed"

key-files:
  created:
    - internal/backendsrv/migrations/00012_item_icon.sql
  modified:
    - internal/backendsrv/enrich/wikiitem.go
    - internal/backendsrv/enrich/wikiitem_test.go
    - internal/backendsrv/enrich/jobs/wiki.go
    - internal/backendsrv/store/enrich.go
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/compute/types.go
    - internal/backendsrv/compute/inventory.go
    - internal/backendsrv/compute/inventory_test.go
    - internal/backendsrv/migrations/migrate_test.go

key-decisions:
  - "icon_id stored as a nullable column on item_master (not a mapping table) — rides the existing UpsertItemMasterTx with zero new join"
  - "parseIconID returns 0 (the no-icon sentinel) for absent/blank/non-numeric/negative input — a NULL/0 surfaces as the colored-tile fallback (D-02) and guarantees the later Item_<int>.png URL is always integer-driven (T-31-01)"
  - "LastSeen carried on CharacterInventory (Open Q1 recommendation), sourced from rows[0].LastSeen — kept the window self-contained over one payload, distinct from per-slot LastListed (Pitfall 2)"
  - "Migration 00012 needs NO WatcherMaxSchemaVersion change — the watcher is off the read path; goose version() is the version of record (no _meta.schema_version cell in this backend)"

patterns-established:
  - "Icon joins by ID in the item_master EQ namespace (im.item_id = ii.item_id), NEVER by normalized name — the name-key rule is ONLY the cross-namespace PigParse price join (Pitfall 3)"

requirements-completed: [INV-04]

# Metrics
duration: 11min
completed: 2026-06-18
---

# Phase 31 Plan 01: Item-Icon Enrichment + StructuredInventory Carry-Through Summary

**`item_master.icon_id` populated from the wiki `lucy_img_ID` (migration 00012) and plumbed — alongside the per-character "Last synced" value — through `InventoryForChar` → `compute.StructuredInventory` into the `InventorySlot.icon_id` / `CharacterInventory.last_seen` JSON contract the web inventory window will render.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-06-18T07:25:29Z
- **Completed:** 2026-06-18T07:36:01Z
- **Tasks:** 2
- **Files modified:** 9 (1 created, 8 modified)

## Accomplishments
- Extend-only migration `00012_item_icon.sql` adds a nullable `item_master.icon_id` column (no DEFAULT/UNIQUE), applied by goose on boot.
- The weekly wiki job now captures each item's `lucy_img_ID` (`enrich.ParseItempage` → `ParsedWikiItem.IconID` → `store.ItemMaster.IconID` → the existing `item_master` upsert), so icon coverage grows automatically with enrichment.
- `icon_id` flows to the inventory window via the existing EQ-namespace `im.item_id = ii.item_id` join — `InventoryRow.IconID` → `slotFromRow` → `InventorySlot.IconID` (`json:"icon_id"`).
- The examine's "Last synced" footer value (`character.last_seen`) is surfaced once per character on `CharacterInventory.LastSeen` (`json:"last_seen"`), kept distinct from the per-slot `LastListed` (the price last-listed date).
- Full Go coverage: a pure `TestParseIconID` table test (present/absent/blank/non-numeric/whitespace/negative), `TestParseItempage_IconID` (Cloak of Flames = 658, absent → 0), `TestMigrate_00012_AddsItemIcon`, `TestStructuredInventory_IconID` (658 hit / NULL → 0 / no-item_master → 0), and `TestStructuredInventory_LastSeen` (== `character.last_seen`, ≠ `LastListed`).

## Task Commits

Each task was committed atomically (TDD: RED in the test file, then GREEN in the same commit):

1. **Task 1: Migration 00012 + lucy_img_ID parse + item_master.icon_id upsert** - `ab8be50` (feat)
2. **Task 2: Carry icon_id + per-char last_seen through store → compute → JSON contract** - `5a88a36` (feat)

_Note: both tasks are `tdd="true"`; the failing test was authored first and the implementation landed in the same task commit (the repo convention here is a single feat commit per task with its tests, mirroring prior 31-adjacent plans)._

## Files Created/Modified
- `internal/backendsrv/migrations/00012_item_icon.sql` - NEW: extend-only `ALTER TABLE item_master ADD COLUMN icon_id INTEGER` (+ `Down` DROP).
- `internal/backendsrv/enrich/wikiitem.go` - `ParsedWikiItem.IconID` + pure `parseIconID` (trim → atoi; 0 on absent/blank/non-numeric/negative) + the `lucy_img_ID` extraction in the `ParseItempage` literal; added `strconv` import.
- `internal/backendsrv/enrich/wikiitem_test.go` - `TestParseIconID` + `TestParseItempage_IconID`.
- `internal/backendsrv/enrich/jobs/wiki.go` - passes `item.IconID` into the `store.ItemMaster` the weekly job upserts.
- `internal/backendsrv/store/enrich.go` - `ItemMaster.IconID` + `icon_id` in `itemMasterUpsert` (column/VALUES/ON CONFLICT) + the `ExecContext` bind.
- `internal/backendsrv/store/readviews.go` - `InventoryRow.IconID` + `im.icon_id` in `InventoryForChar`'s SELECT/scan (sql.NullInt64 → 0).
- `internal/backendsrv/compute/types.go` - append-only `InventorySlot.IconID` (`json:"icon_id"`) + `CharacterInventory.LastSeen` (`json:"last_seen"`).
- `internal/backendsrv/compute/inventory.go` - `slotFromRow` copies `row.IconID`; `buildStructuredInventory` sources `LastSeen` from `rows[0]`.
- `internal/backendsrv/compute/inventory_test.go` - `TestStructuredInventory_IconID` + `TestStructuredInventory_LastSeen` + the `seedItemMasterIcon` helper.
- `internal/backendsrv/migrations/migrate_test.go` - `TestMigrate_00012_AddsItemIcon`.

## Decisions Made
- **icon_id is a column on `item_master`, not a mapping table** — it is a 1:1 attribute the weekly job already upserts, so a column rides the existing write path with zero new join (RESEARCH discretion default).
- **`parseIconID` rejects negatives and non-numerics to 0** — the stored `icon_id` is always a non-negative integer, so the later client URL `Item_${int}.png` can never carry an arbitrary wiki string (the T-31-01 type-safety mitigation).
- **`LastSeen` lives on `CharacterInventory` (Open Q1 rec.)**, sourced from the first row (it is the same `character.last_seen` on every row) — self-contained over one payload, and explicitly NOT aliased to the per-slot `LastListed`.
- **No watcher coordination** — a read-only additive column; goose `version()` is the version of record (stated in the migration header per the plan).

## Deviations from Plan

The plan was executed as written. Two small additive items beyond the literal task file, both completeness (not scope creep):

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added `TestMigrate_00012_AddsItemIcon` migration test**
- **Found during:** Task 1 (migration)
- **Issue:** `migrate_test.go` has a per-migration regression test for every prior migration (`TestMigrate_00003..00011`); shipping `00012` without one would be the only un-tested migration in the file.
- **Fix:** Added `TestMigrate_00012_AddsItemIcon` (column exists; row without `icon_id` reads NULL; with `icon_id` round-trips; second `Up` is an idempotent no-op) mirroring the established convention.
- **Files modified:** `internal/backendsrv/migrations/migrate_test.go`
- **Verification:** `go test ./internal/backendsrv/migrations/...` green.
- **Committed in:** `ab8be50` (Task 1 commit)

**2. [Rule 2 - Missing Critical] Added `seedItemMasterIcon` test helper + a third icon assertion (no-item_master miss)**
- **Found during:** Task 2 (compute test)
- **Issue:** The existing `seedItemMaster` helper has 2 callers in sibling test files and does not set `icon_id`; changing its signature would touch unrelated tests. The plan's icon test also only named the hit + NULL cases.
- **Fix:** Added a dedicated `seedItemMasterIcon` helper (leaves `seedItemMaster` untouched) and a third assertion — an item with NO `item_master` row at all (the LEFT JOIN miss) also surfaces `IconID == 0` — strengthening the sentinel proof.
- **Files modified:** `internal/backendsrv/compute/inventory_test.go`
- **Verification:** `go test ./internal/backendsrv/compute/...` green.
- **Committed in:** `5a88a36` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 2 — test completeness).
**Impact on plan:** Both stay within the plan's `files_modified` list (`migrate_test.go` was implied by the migration task; `inventory_test.go` is a named file). No production-code scope creep; no new public surface.

## Issues Encountered
None. All referenced files, line numbers, and analog patterns matched the codebase exactly. Confirmed migration `00011` was the latest before adding `00012`. The wiki fixtures already carried `lucy_img_ID` values (Cloak of Flames `658`, matching RESEARCH), so the parse test used a real fixture with no new test data.

## Known Stubs
None. Both changes are real data-flow wiring (column → parse → upsert → read → compute → JSON), end-to-end test-covered.

## User Setup Required
None - no external service configuration required. (Migration `00012` applies on the next backend boot; the live deploy happens in Plan 31-04's deploy step alongside the binary, per the plan's `<verification>`.)

## Next Phase Readiness
- The backend data half of INV-04 is complete: `icon_id` + per-character `last_seen` are in the `compute.StructuredInventory` JSON contract.
- **Ready for Plan 31-02** (the new `GET /api/v1/inventory/{char}` + `GET /api/v1/characters` read-API routes) and **31-04** (the SvelteKit window that renders `Item_<iconId>.png` with the colored-tile fallback + the examine "Last synced" footer).
- No route, no web, and no watcher change in this plan — exactly as scoped. The icon coverage is incremental by design (D-02); it backfills as the weekly wiki job re-runs (an item's `icon_id` writes on its next wikitext-SHA-1-changed enrichment, or immediately for any item whose page changes).

## Self-Check: PASSED

- All created/modified files verified present on disk (migration 00012, wikiitem.go, enrich.go, readviews.go, types.go, inventory.go, SUMMARY.md).
- Both task commits verified in git log: `ab8be50` (Task 1), `5a88a36` (Task 2).
- Gates green: `go test ./internal/backendsrv/...` (all packages ok), `go vet ./internal/backendsrv/...` clean, `go build ./...` rc=0.

---
*Phase: 31-characters-tab-in-game-inventory-window*
*Completed: 2026-06-18*
