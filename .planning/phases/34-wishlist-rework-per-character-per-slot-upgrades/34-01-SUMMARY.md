---
phase: 34-wishlist-rework-per-character-per-slot-upgrades
plan: 01
subsystem: database
tags: [sqlite, goose, migration, compute-on-read, wishlist, pigparse, gear-tier, owner-scoped, idor]

# Dependency graph
requires:
  - phase: 29-data-foundation-inventory-parse-price-value-aggregation
    provides: GearTierPrices (name-keyed gear-tier price read, built for WISH-04) + the pp_rep name-bridge convention
  - phase: 31-characters-tab-in-game-inventory-window
    provides: compute.StructuredInventory (equipped item per slot + the held set for auto-removal) + the public-fn/pure-helper split
  - phase: 19-wantlist (v2.2)
    provides: store/wantlist.go + 00006/00007 migration lineage (the owner-scoped CRUD + 2067-dup idiom + alert_log rebuild pattern cloned here)
provides:
  - "Migration 00014_wishlist (schema v14): wishlist_item table + alert_log FK rebuilt to wishlist_item(id) + the retired wantlist_item DROPPED (D-01 clean break)"
  - "store/wishlist.go: owner-scoped AddWishlistTx / ListOwnWishlist / RemoveOwnWishlistTx / SetPingedTx + AlertedWishlistIDs (EC-hit badge set)"
  - "store.PriceByName: whole-catalog name-keyed price map (the examine name-bridge for any wishlist target's price)"
  - "compute.WishlistFor + pure buildWishlistView: equipped + auto-removal-filtered targets (name-keyed price) + class+slot gear-tier suggestions via the slot bridge"
  - "compute/types.go: append-only WishlistView/WishlistSlot/WishlistTarget/WishlistSuggestion contract"
affects: [34-02 (matcher repoint + read/write API consume wishlist_item + WishlistView), 34-03 (web tab consumes WishlistView), wantlist retirement]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Clean-break migration: rebuild a FK-referencing table (alert_log) BEFORE dropping the referenced table (wantlist_item) — correct under either PRAGMA foreign_keys setting"
    - "Keep the FK COLUMN name (wantlist_item_id) while repointing its FK TARGET (→ wishlist_item) — zero churn to store/alertlog.go (Pitfall 6 option B)"
    - "Canonical-worn-slot → wiki-prose-slot bridge by INVERTING enrich.WIKI_SLOT_TO_INV_SLOTS (Finger1 & Finger2 → Fingers); empty for Ammo/Charm/Power"
    - "Auto-removal compute-on-read (D-02): hide-not-delete a target whose normalized name the char holds ANYWHERE"
    - "Name-keyed target price over the FULL pigparse_price catalog (PriceByName), not just the gear-tier slice (WARNING-3)"

key-files:
  created:
    - internal/backendsrv/migrations/00014_wishlist.sql
    - internal/backendsrv/store/wishlist.go
    - internal/backendsrv/store/wishlist_test.go
    - internal/backendsrv/compute/wishlist.go
    - internal/backendsrv/compute/wishlist_test.go
  modified:
    - internal/backendsrv/migrations/migrate_test.go
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/store/readviews_test.go
    - internal/backendsrv/compute/types.go

key-decisions:
  - "00014 KEEPS the alert_log column name wantlist_item_id (only the FK target changes to wishlist_item(id)) so store/alertlog.go needs no edit (Pitfall 6 option B)"
  - "alert_log rebuilt BEFORE wantlist_item is dropped (rebuild-before-drop) — correct under either PRAGMA foreign_keys setting (T-34-01)"
  - "pinged defaults ON (DEFAULT 1) — the inverse of the wantlist's muted; the matcher gate becomes pinged=1 (Pitfall 8)"
  - "Target price resolves over the WHOLE pigparse_price catalog by normalized name (store.PriceByName), not just the gear-tier slice — a catalog-only priced item still shows a price (WARNING-3 fix)"
  - "Historical migration tests (00006/00007/00010/00011) PINNED to their version via a new openAtVersion(UpTo) helper, since HEAD now drops wantlist_item"

patterns-established:
  - "openAtVersion(t, N): a migrate-test helper that opens a raw store handle pinned at goose version N — for historical tests of a table a later migration drops"
  - "The slot-vocabulary inverse map (invSlotToWiki) built once at package init from enrich.WIKI_SLOT_TO_INV_SLOTS"

requirements-completed: [WISH-02, WISH-03, WISH-04]

# Metrics
duration: 21min
completed: 2026-06-19
---

# Phase 34 Plan 01: Wishlist Backend Data Foundation Summary

**Migration 00014 (schema v14, the D-01 clean break) + owner-scoped store/wishlist.go CRUD + store.PriceByName + compute.WishlistFor — the per-character/per-slot wishlist data + transform layer (schema, store reads, the WishlistFor compute) every later Phase-34 plan reads. NO route/web/watcher change.**

## Performance

- **Duration:** 21 min
- **Started:** 2026-06-19T03:16:45Z
- **Completed:** 2026-06-19T03:38:15Z
- **Tasks:** 3
- **Files modified:** 9 (5 created, 4 modified)

## Accomplishments
- **Migration 00014_wishlist (schema → v14, the D-01 clean break):** new `wishlist_item` table (per char + canonical worn-slot + target, `pinged` default-ON, `active` soft-delete, two partial-unique dedup indexes); `alert_log` REBUILT (DROP+CREATE) with its FK repointed to `wishlist_item(id)` **before** `wantlist_item` is dropped (correct under either `PRAGMA foreign_keys` setting); the retired `wantlist_item` + its indexes dropped. The column name `wantlist_item_id` is KEPT so `store/alertlog.go` needs no edit. Forward-only; 00001–00013 unedited.
- **store/wishlist.go (owner-scoped, IDOR-safe):** `AddWishlistTx` (typed `ErrDuplicateWishlist` via the 2067 extended result code), `ListOwnWishlist` (owner+char-scoped, char binds as `?`), `RemoveOwnWishlistTx` / `SetPingedTx` (silent `RowsAffected=0 → (false,nil)` IDOR no-op), `AlertedWishlistIDs` (the EC-hit badge set over `alert_log.wantlist_item_id`).
- **store.PriceByName:** the whole-catalog name-keyed price map (the `pp_rep` MIN(item_id) collapse) — the same name-bridge the examine uses — so a wishlist target's price resolves against the FULL `pigparse_price` catalog, not just the gear-tier slice (the WARNING-3 fix).
- **compute.WishlistFor + pure buildWishlistView:** per the 21 worn slots — the equipped item + auto-removal-filtered targets (a held name is HIDDEN, D-02) with name-keyed price + the class+slot gear-tier suggestions via the canonical→wiki-prose slot bridge (`Finger1`&`Finger2`→`Fingers`; "Raid" tag = `tier == "Velious Raiding"`).
- **compute/types.go:** the append-only `WishlistView`/`WishlistSlot`/`WishlistTarget`/`WishlistSuggestion` contract.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 00014_wishlist.sql + migrate test** — `ea0223a` (feat)
2. **Task 2: store/wishlist.go owner-scoped CRUD + ping toggle + badge read** — `6f1eea3` (feat)
3. **Task 3: store.PriceByName + compute.WishlistFor + types append** — `00574e9` (feat)

_TDD: each task wrote its test alongside its implementation; the migration's RED was verified (`wishlist_item` absent / `wantlist_item` present) before the GREEN migration shipped._

## Files Created/Modified
- `internal/backendsrv/migrations/00014_wishlist.sql` — wishlist_item table + alert_log FK rebuild + drop wantlist_item (D-01)
- `internal/backendsrv/migrations/migrate_test.go` — `TestMigrate_00014_AddsWishlist` + pinned the 4 historical wantlist-table tests to their version (new `openAtVersion` helper)
- `internal/backendsrv/store/wishlist.go` — owner-scoped CRUD + ping toggle + ListOwnWishlist + AlertedWishlistIDs
- `internal/backendsrv/store/wishlist_test.go` — add/dup (catalog+custom), cross-owner remove/ping no-op, owner+char scoping, badge set
- `internal/backendsrv/store/readviews.go` — `PriceByName` + `PriceByNameRow` (the catalog-wide name-keyed price read)
- `internal/backendsrv/store/readviews_test.go` — `TestPriceByName_RepresentativePerName` (fan-out collapse + normalized key)
- `internal/backendsrv/compute/wishlist.go` — `WishlistFor` / pure `buildWishlistView` / `wikiSlotFor` slot bridge / `norm`
- `internal/backendsrv/compute/wishlist_test.go` — slot bridge, suggestion class/slot/Raid filter, auto-removal name join, catalog-price-by-name, 21-slot D-04
- `internal/backendsrv/compute/types.go` — append-only WishlistView contract

## Decisions Made
- All locked decisions (D-01..D-04) and researcher resolutions (Pitfall 6 option B, rebuild-before-drop, pinged default-ON, name-keyed target price over the full catalog) were applied exactly as the plan specified.
- **Migration-test pinning (a planner-anticipated consequence, not a deviation):** because HEAD now drops `wantlist_item`, the historical `TestMigrate_00006/00007/00010/00011` (which probe `wantlist_item` at HEAD via `NewTestDB`) were repinned to their own schema version with a new `openAtVersion(t, N)` helper (built on the existing `migrations.UpTo`). The historical assertions are byte-for-byte unchanged; only the schema version they run against is pinned. This keeps the migration-package gate green without rewriting any migration.

## Deviations from Plan

None — plan executed exactly as written.

The migration-test repinning above is in-scope: it is the migration package's OWN tests adjusting to the table the plan's own migration drops (the migrations-package gate is a Task-1 acceptance criterion: "the migrate test passes"). It touches only the migration test file the plan already lists in `files_modified`, adds no new production behavior, and the historical assertions are unchanged.

## Issues Encountered

**The D-01 clean break drops `wantlist_item` → the retired-wantlist surface tests fail at HEAD (the EXPECTED 34-02 hand-off).** Five packages (`wantmatch`, `webadmin`, `store`, `ec`, `notify`) have tests that seed/query `wantlist_item`; after 00014 drops the table they fail SOLELY with `no such table: wantlist_item`. The plan's `<verification>` explicitly designates this the 34-02 hand-off ("if a pre-existing test breaks because this plan dropped wantlist_item, that is the EXPECTED hand-off to 34-02 — do NOT patch those files here"). 34-02 repoints the matcher + retires the wantlist write/read surface. **Production code still COMPILES** — `go build ./...` rc=0 (these are SQL strings, runtime-checked only); only the test runs fail. Logged in `deferred-items.md` with the per-package 34-02 action.

## Verification

- `go test ./internal/backendsrv/migrations/...` — ALL green (incl. `TestMigrate_00014_AddsWishlist`: wishlist_item present + 9 columns, wantlist_item gone, NULL-FK + real-FK alert_log inserts OK, idempotent re-run; the 4 repinned historical tests pass).
- `go test ./internal/backendsrv/store/... -run "Wishlist|AlertedWishlist|PriceByName"` — green (owner-scoped CRUD + 2067-dup catalog+custom + silent IDOR no-op + badge set + PriceByName fan-out collapse).
- `go test ./internal/backendsrv/compute/...` — FULL package green (slot bridge Finger1&Finger2→Fingers, Raid-tag-is-tier, auto-removal name join, catalog-price-by-name, 21-slot D-04).
- `go vet ./internal/backendsrv/{migrations,store,compute}/...` — clean.
- `go build ./...` — rc=0.
- Full module `go test ./...` — green EXCEPT the 5 retired-wantlist-surface packages above (all fail solely on the dropped `wantlist_item` table — the documented 34-02 hand-off).

## User Setup Required
None — no external service configuration required. (The live 00014 migration runs on the prod DB at the 34-04 deploy, behind an R2 backup; no action this plan.)

## Next Phase Readiness
- **34-02 unblocked:** consumes `wishlist_item` (matcher repoint of `wantmatch.ForItem`/`ForName`: `FROM wishlist_item`, `muted=0`→`pinged=1`, `LEFT JOIN`→`JOIN`, drop `note`), the `store/wishlist.go` write seam, and `compute.WishlistFor`/`WishlistView` (the `GET /api/v1/wishlist/{char}` read route). It MUST also unregister the 4 old `/api/v1/wantlist*` routes + retire `store/wantlist.go`/`webadmin/wantlist.go` + reseed the 5 retired-wantlist-surface test files on `wishlist_item` (see `deferred-items.md`).
- **34-03 unblocked:** the web tab consumes the `WishlistView` snake_case contract.
- No git tag (watcher UNTOUCHED — a `v*` tag would needlessly fire the watcher CI).

## Self-Check: PASSED

All 5 created files exist on disk; the SUMMARY exists; all 3 task commits (`ea0223a`, `6f1eea3`, `00574e9`) are present in git history.

---
*Phase: 34-wishlist-rework-per-character-per-slot-upgrades*
*Completed: 2026-06-19*
