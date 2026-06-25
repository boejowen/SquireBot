---
phase: 37-item-enrichment-backbone-flags-effects
plan: 02
subsystem: database
tags: [go, sqlite, goose-migration, item-flags, clicky, haste, backfill, enrichment]

# Dependency graph
requires:
  - phase: 37-01 (parser half)
    provides: "ParsedWikiItem.{IsLore,IsNoDrop,IsMagic,IsTemporary,Flags,IsClicky,ClickyEffect,HasHaste,HastePct} + the clicky-classification rule — the nine field names persisted here"
  - phase: 31-item-enrichment (00012 icon / 00013 statsblock)
    provides: "item_master.statsblock TEXT (the stored block the backfill re-parses) + the ADD-COLUMN + freshness-comparison precedent mirrored for 00016"
provides:
  - "migration 00016: nine discrete nullable item_master columns (is_lore/is_no_drop/is_magic/is_temporary, is_clicky/clicky_effect, has_haste/haste_pct, flags_json)"
  - "store.ItemMaster extended + itemMasterUpsert (19 cols) + GetItemMasterFreshnessTx returns flags_json"
  - "store.MarshalFlags — the ONE canonical flags-array encoder (empty -> \"[]\", never null)"
  - "enrich.DeriveFlagsAndEffects — the single pure flag/effect derivation shared by the live parser + the backfill"
  - "store.BackfillItemFlags — the no-network, idempotent (flags_json IS NULL) boot re-parse of the stored statsblock (D-05)"
  - "weekly wiki job self-heals pre-00016 rows on the next pass (D-06)"
affects: [38-enrichment-coverage, 39-search-facets, 40-item-ui-outlines]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "ONE canonical encoder (store.MarshalFlags) at every flags_json write/compare site so byte-equality freshness is deterministic (D-06 idempotency)"
    - "ONE pure derivation (enrich.DeriveFlagsAndEffects) called from BOTH the live parser and the boot backfill — no drift"
    - "store importing enrich (the pure parser) for the backfill — acyclic because enrich never imports store"
    - "separator-union splitter (<br> OR \\n) so the same parseStatsblock serves the raw <br> form AND the stored cleaned \\n form"

key-files:
  created:
    - internal/backendsrv/migrations/00016_item_flags_effects.sql
    - internal/backendsrv/store/backfill.go
    - internal/backendsrv/store/backfill_test.go
  modified:
    - internal/backendsrv/store/enrich.go
    - internal/backendsrv/store/enrich_test.go
    - internal/backendsrv/enrich/wikiitem.go
    - internal/backendsrv/enrich/wikiitem_test.go
    - internal/backendsrv/enrich/jobs/wiki.go
    - internal/backendsrv/enrich/jobs/wiki_test.go
    - internal/backendsrv/migrations/migrate_test.go
    - cmd/squirebot-server/main.go

key-decisions:
  - "Column names: is_lore, is_no_drop, is_magic, is_temporary, is_clicky, clicky_effect, has_haste, haste_pct, flags_json (all nullable INTEGER except clicky_effect/flags_json TEXT)"
  - "flags_json holds the FULL detected flag SET as a JSON array (D-03/D-04) — a future flag needs no new migration"
  - "MarshalFlags contract: input assumed already-sorted; nil/empty -> literal \"[]\" (never null/\"\"); non-empty -> json.Marshal array"
  - "Backfill idempotency key = flags_json IS NULL (a row drops out once populated; a flagless row stores \"[]\" so it too drops out)"
  - "Freshness self-heal signal = stored flags_json vs MarshalFlags(parsed flags); a pre-00016 NULL reads \"\" != \"[]\" so it re-writes ONCE"
  - "WatcherMaxSchemaVersion is NOT touched — that Go constant does not exist in the off-Google backend (watcher is off the item_master read path)"
  - "ParseItempage now routes its flag/effect derivation through DeriveFlagsAndEffects (one extra in-memory parseStatsblock, negligible at 1 req/s) to make the single-entrypoint contract literal"

patterns-established:
  - "Pattern 1: single-canonical-encoder — every flags_json string is produced by store.MarshalFlags, so the upsert, backfill, and freshness compare byte-equal each other (D-06)"
  - "Pattern 2: parse-once-derive-everywhere — enrich.DeriveFlagsAndEffects is the only flag/clicky/haste derivation, reached from the live parser AND the backfill"

requirements-completed: [ENRICH-12, ENRICH-13]

# Metrics
duration: 16min
completed: 2026-06-25
---

# Phase 37 Plan 02: Item enrichment backbone — flags + effects (persist half) Summary

**Migration 00016 persists the nine parsed flag/effect fields as discrete, individually-queryable item_master columns; an immediate no-network boot backfill re-parses every already-enriched row's stored statsblock (D-05); and the weekly wiki job self-heals any pre-00016 row — all flags_json values produced by one canonical encoder so a flagless item is written exactly once.**

## Performance

- **Duration:** 16 min
- **Started:** 2026-06-25T03:35:20Z
- **Completed:** 2026-06-25T03:51:38Z
- **Tasks:** 3
- **Files modified:** 11 (8 modified, 3 created)

## Accomplishments
- Added goose migration `00016_item_flags_effects.sql`: nine extend-only, nullable `ADD COLUMN`s (no DEFAULT/UNIQUE) — `is_lore`, `is_no_drop`, `is_magic`, `is_temporary`, `is_clicky`, `clicky_effect`, `has_haste`, `haste_pct`, `flags_json` — with `TestMigrate_00016_AddsItemFlagsEffects` asserting all nine columns + idempotency.
- Extended `store.ItemMaster` + `itemMasterUpsert` (19 columns/placeholders, all `ON CONFLICT DO UPDATE`) + `GetItemMasterFreshnessTx` (now returns `flags_json` as the self-heal signal).
- Added the **single canonical** `store.MarshalFlags` (empty/nil → `"[]"`, never `null`/`""`) used at all three flags_json sites (upsert literal, backfill, freshness compare) — the D-06 idempotency keystone — with `TestMarshalFlags`.
- Factored the 37-01 derivation into the **single pure** `enrich.DeriveFlagsAndEffects` (returns `DerivedFlagsEffects`), called from BOTH `ParseItempage` and the backfill; generalized the statsblock splitter to `<br>` OR newline so the stored cleaned form re-parses identically.
- Added `store.BackfillItemFlags`: one-tx, no-network, idempotent (`flags_json IS NULL`) re-parse of each stored statsblock — including correctly classifying + naming a clicky from the no-bracket cleaned `Effect:` line — wired into the server boot right after `RunMigrations` (non-fatal, log-and-continue).
- Self-healing freshness: the weekly job re-writes a pre-00016 row (correct SHA-1 but NULL flags_json) to backfill the flag/effect columns; `TestRunWiki_BackfillsStaleFlags` regression-guards it.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 00016 + migrate-test + store struct/upsert/freshness/MarshalFlags** - `e590f72` (feat)
2. **Task 2: No-network backfill from the stored statsblock (D-05) + boot invocation** - `dd42fc3` (feat)
3. **Task 3: Self-healing flags backfill on the weekly wiki pass (regression test)** - `2dcc0b3` (test)

_Note: the wiki.go production wiring (extended freshness compare via the canonical `store.MarshalFlags` + the new fields in the store literal) shipped in the **Task 1** commit, because the `GetItemMasterFreshnessTx` signature change rippled into its one caller and had to land together to keep that commit buildable. Task 3 is therefore the regression test alone — the production change it covers is in Task 1._

## Files Created/Modified
- `internal/backendsrv/migrations/00016_item_flags_effects.sql` - the nine extend-only ADD COLUMNs (created).
- `internal/backendsrv/migrations/migrate_test.go` - `TestMigrate_00016_AddsItemFlagsEffects` (nine-column assertion + idempotency).
- `internal/backendsrv/store/enrich.go` - extended `ItemMaster`, `itemMasterUpsert` (19 cols), `UpsertItemMasterTx`, `GetItemMasterFreshnessTx`; added the canonical `MarshalFlags`.
- `internal/backendsrv/store/enrich_test.go` - `TestMarshalFlags` (nil/empty→"[]", round-trip).
- `internal/backendsrv/store/backfill.go` - `BackfillItemFlags` (no-network, idempotent re-parse) (created).
- `internal/backendsrv/store/backfill_test.go` - clicky-from-no-bracket-Effect + idempotent, haste/no-effect, flagless→"[]" (created).
- `internal/backendsrv/enrich/wikiitem.go` - extracted `DeriveFlagsAndEffects` + `deriveFromMaps` + `DerivedFlagsEffects`; generalized the splitter to `<br>` OR `\n`.
- `internal/backendsrv/enrich/wikiitem_test.go` - `TestDeriveFlagsAndEffects_NewlineForm` (no-bracket clicky, haste, empty, br-vs-nl parity).
- `internal/backendsrv/enrich/jobs/wiki.go` - freshness compare + store literal carry the new fields via the canonical `parsedFlagsJSON`.
- `internal/backendsrv/enrich/jobs/wiki_test.go` - `TestRunWiki_BackfillsStaleFlags` regression.
- `cmd/squirebot-server/main.go` - one-time boot backfill call after `RunMigrations` (non-fatal).

## Decisions Made
- **Column names + types:** the four flag booleans + `is_clicky`/`has_haste` are `INTEGER` 0/1 (the same `b2i` convention `is_quest_item` uses); `clicky_effect` + `flags_json` are `TEXT`. `flags_json` is the full detected flag set as a JSON array (D-03/D-04), so a future flag (Attunable, No Rent, …) is captured with no new migration.
- **`MarshalFlags` is the only flags_json producer** (empty → `"[]"`). Without one canonical form, an empty set encoded as `null` at one site and `"[]"` at another would re-write the row on every weekly pass forever — this is the load-bearing D-06 idempotency decision.
- **Backfill idempotency key = `flags_json IS NULL`.** A first boot scans+updates every enriched row; a second boot scans the same rows but updates 0 (flags_json is now populated, even for flagless items which store `"[]"`).
- **Freshness self-heal signal = `flags_json` byte-compare** (stored vs `MarshalFlags(parsed flags)`). A pre-00016 NULL reads `""` here, which differs from `"[]"`/the array, so the row re-writes exactly ONCE to backfill — the same argument 00012 used for `icon_id`, one more field.
- **`ParseItempage` routes its derivation through `DeriveFlagsAndEffects`** (one extra in-memory `parseStatsblock` per item) so the single-entrypoint contract is literal and there is provably no second derivation. At the weekly crawl's 1-req/s rate this is negligible.
- **WatcherMaxSchemaVersion intentionally untouched.** CLAUDE.md references the constant in `internal/sheet/client.go`, but that file/constant does NOT exist in the post-v2.0 off-Google backend (the watcher uploads over HTTPS and never reads `item_master`). These are read-only additive backend columns, so there is nothing to bump — consistent with every prior backend migration test comment (00010/00012/00014/00015).

## Deviations from Plan

None - plan executed exactly as written. (The only structural note: the wiki.go production change the plan grouped under Task 3 landed in the Task 1 commit for build-coherence — the `GetItemMasterFreshnessTx` signature change forced its one caller to update in the same commit. No behavior differs from the plan; Task 3 remains the regression test it specified. This is a commit-grouping detail, not a content deviation.)

## Issues Encountered
- **`b2i` already existed** in `store/notifyprefs.go` — my first draft redeclared it and the build flagged the duplicate. Removed the duplicate and reused the existing package-level helper. (Caught immediately by `go build`; no rework beyond deleting the dupe.)
- **Stray `database/sql` import** in `backfill.go` (unused after the final shape) — removed; build green.

## Threat Model Compliance
- **T-37-04 (SQL injection):** mitigated — `itemMasterUpsert` and the backfill `UPDATE` bind every value through `?` placeholders; the migration is static DDL. No untrusted parsed text is concatenated into SQL.
- **T-37-05 (DoS/panic on bad statsblock):** mitigated — the backfill re-uses the bounded `DeriveFlagsAndEffects` (split-on-separator, no nested-quantifier regex); empty/NULL statsblock rows are filtered by the SELECT (`statsblock != ''`); `DeriveFlagsAndEffects("")` returns the zero value without panicking (proven by a test case).
- **T-37-06 (info disclosure in logs):** mitigated — `slog` in the backfill logs `scanned`/`updated` counts + `item_id`/`err` only, never raw statsblock/flag content (V7).
- **T-37-07 (render-time escaping of clicky_effect/flags_json):** accepted/deferred — these columns are stored, not yet rendered; escaping is a Phase 40 (ITEMUI) concern. No rendering surface exists this phase.

## User Setup Required
None - no external service configuration required. Deploy = drop the new server binary + restart (goose runs 00016 on boot, then the one-time backfill re-parses the live `item_master.statsblock` rows with no network — the first prod boot of 00016 will light up every already-enriched item's flags/effects).

## Next Phase Readiness
- **P39 (search facets, SEARCH-04/05):** the discrete columns are ready to filter/sort on — `is_clicky` (+ `clicky_effect` name), `is_lore`/`is_no_drop`/`is_magic`/`is_temporary`, `has_haste`/`haste_pct`, and `flags_json` for "has flag X" facets. The full flag set lives in `flags_json` (JSON array) for any flag P39 wants beyond the four named booleans.
- **P40 (item-detail outlines, ITEMUI-01):** the examine panel can read `clicky_effect` ("Haste 36%" / the clicky name) directly. Note T-37-07: these stored strings MUST be escaped at render time.
- **Idempotency / freshness contract for downstream:** flags_json is the freshness signal AND the backfill key; it is ALWAYS produced by `store.MarshalFlags` (empty → `"[]"`). Any future writer of these columns MUST go through `MarshalFlags` to preserve the byte-equality the weekly self-heal depends on.

## Self-Check: PASSED

- FOUND: internal/backendsrv/migrations/00016_item_flags_effects.sql (created)
- FOUND: internal/backendsrv/store/backfill.go (created)
- FOUND: internal/backendsrv/store/backfill_test.go (created)
- FOUND: .planning/phases/37-item-enrichment-backbone-flags-effects/37-02-SUMMARY.md (created)
- FOUND commit e590f72 (Task 1 feat)
- FOUND commit dd42fc3 (Task 2 feat)
- FOUND commit 2dcc0b3 (Task 3 test)
- `go build ./...` exit 0, `go vet ./internal/backendsrv/...` clean, `go test ./...` exit 0 (whole-module green)
- WatcherMaxSchemaVersion: no Go constant exists in the off-Google backend — watcher untouched (assertion holds)

---
*Phase: 37-item-enrichment-backbone-flags-effects*
*Completed: 2026-06-25*
