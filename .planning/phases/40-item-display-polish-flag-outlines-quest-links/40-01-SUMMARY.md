---
phase: 40-item-display-polish-flag-outlines-quest-links
plan: 01
subsystem: api
tags: [go, sqlite, compute, json-contract, item-flags, quest-links, itemui]

# Dependency graph
requires:
  - phase: 37-item-enrichment-backbone
    provides: "item_master.is_no_drop/is_lore/is_magic flag columns (migration 00016)"
  - phase: 14-read-api
    provides: "store.QuestLinksByItem + compute.QuestLink + the InventorySlot/ItemRollup JSON contract"
  - phase: 39-faceted-item-search
    provides: "the is_clicky/has_haste flag-plumbing template (item_master → IconStats → ItemRollup)"
provides:
  - "InventorySlot JSON carries is_no_drop/is_lore/is_magic (held item_master source) + quest_links[] (each with source_url)"
  - "ItemRollup JSON carries the same three flags (id-correct IconStats source) + quest_links[] copied from the representative ViewRow"
  - "QuestLink.SourceURL plumbed end-to-end: quest_items.source_url → QuestLinkRow.SourceURL → compute.QuestLink → JSON source_url"
affects: [40-02-web, 41-character-paperdoll, 42-wishlist-polish]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Additive JSON-contract evolution: new fields appended at the right edge of QuestLink/InventorySlot/ItemRollup; no tag renamed"
    - "NullInt64 != 0 → bool flag idiom reused verbatim from the Phase-39 is_clicky/has_haste path"
    - "Pure-transform preserved: buildStructuredInventory takes the QuestLinksByItem map as a param (store fetch stays in the public StructuredInventory)"

key-files:
  created: []
  modified:
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/store/readviews_test.go
    - internal/backendsrv/compute/types.go
    - internal/backendsrv/compute/view.go
    - internal/backendsrv/compute/inventory.go
    - internal/backendsrv/compute/inventory_test.go
    - internal/backendsrv/compute/itemrollup.go
    - internal/backendsrv/compute/itemrollup_test.go

key-decisions:
  - "ItemRollup quest_links copied from the representative ViewRow (View already fetched the links) — honors itemrollup.go's 'copy from representative, never re-select' iron law; no new store call"
  - "InventorySlot quest links attached via a post-build walk over Equipment/General/Bank + their Children, keeping buildStructuredInventory a pure transform with the links map passed in"
  - "questLinksFor returns ALL links (notes_link + in_game_flag); the notes_link-only filter (D-06) is a web-layer concern (40-02), left out of the backend per CONTEXT"

patterns-established:
  - "Held-item flags (ITEMUI-01 tile outline) read from the id-joined item_master in InventoryForChar; rollup flags read from ItemMasterIconStats — both EQ-namespace correct, NOT catalog_enrichment"

requirements-completed: [ITEMUI-01, ITEMUI-02]

# Metrics
duration: 12min
completed: 2026-06-26
---

# Phase 40 Plan 01: Item display polish — backend flag + quest-link plumbing Summary

**Backend (Go) plumbing that carries the three already-stored item_master flags (is_no_drop/is_lore/is_magic) and the named quest links (quest_items.source_url) through compute → the JSON contract onto the modern InventorySlot and ItemRollup payloads — pure additive, no migration, watcher untouched.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-06-26T03:43:47Z
- **Completed:** 2026-06-26T03:56:00Z
- **Tasks:** 3 (2 code + 1 verification gate)
- **Files modified:** 8

## Accomplishments
- Store layer reads `quest_items.source_url` into `QuestLinkRow.SourceURL` and the three `item_master` flag columns into both the `InventoryForChar` join (held source) and `ItemMasterIconStats` (rollup source), copying the Phase-39 NullInt64→bool idioms verbatim.
- Compute layer: `QuestLink` gains `SourceURL` (`json:"source_url"`); `InventorySlot` + `ItemRollup` each gain `is_no_drop`/`is_lore`/`is_magic` + `quest_links[]`. `slotFromRow` copies the flags; `StructuredInventory` fetches the quest-link map and a post-build walk attaches links to every filled slot incl. bag children; `buildItemRollups` copies the flags from `IconStats` and `quest_links` from the representative `ViewRow`.
- Go table tests assert the new fields end-to-end (store + compute), full `go test ./...` for the backend module is green, `go vet` clean, no migration added, watcher untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Store layer — source_url + 3 item_master flags on the two reads** - `bf55fac` (feat)
2. **Task 2: Compute layer — QuestLink.SourceURL + 3 flags + quest_links onto InventorySlot/ItemRollup** - `0810552` (feat)
3. **Task 3: Full-module regression gate** - verification-only (no code, no commit): `go vet ./...` + `go test ./...` green, no new migration, watcher untouched.

**Plan metadata:** committed separately (docs: complete plan).

_Note: this plan's two code tasks were `tdd="true"` but were committed as single feat commits per task (source + tests together) since the TDD red/green happened inline within each task; the gate task adds no code._

## Files Created/Modified
- `internal/backendsrv/store/readviews.go` - `QuestLinkRow.SourceURL` + `source_url` SELECT in `QuestLinksByItem`; `is_no_drop/is_lore/is_magic` added to `InventoryRow` + the `InventoryForChar` join; same three flags on `IconStats` + the `ItemMasterIconStats` SELECT.
- `internal/backendsrv/store/readviews_test.go` - `.SourceURL` assertion in the `QuestLinksByItem` subtest; flag assertions in `TestItemMasterIconStats` + `TestReadViews_InventoryForChar_NameJoinHitAndMiss` (flagged row true, LEFT-JOIN/NULL row false).
- `internal/backendsrv/compute/types.go` - `QuestLink.SourceURL`; `InventorySlot` + `ItemRollup` gain the three flags + `quest_links`; package-doc contract block updated.
- `internal/backendsrv/compute/view.go` - `questLinksFor` copies `SourceURL` (feeds both legacy views and the new InventorySlot path).
- `internal/backendsrv/compute/inventory.go` - `slotFromRow` copies the three flags; `StructuredInventory` fetches `QuestLinksByItem`; `buildStructuredInventory` takes the links map + a post-build walk attaches `questLinksFor` per slot (incl. children).
- `internal/backendsrv/compute/inventory_test.go` - `TestStructuredInventory_FlagsAndQuestLinks` (flags surface, named-quest source_url plumbed, bag-child link attach, unflagged item all-false/nil).
- `internal/backendsrv/compute/itemrollup.go` - first-seen branch copies the three flags from `IconStats` + `quest_links` from the representative `ViewRow` (no re-fetch).
- `internal/backendsrv/compute/itemrollup_test.go` - `setItemDisplayFlags` helper + extended `TestItems_GroupsByNameWithHoldersAndFlags` to assert the three flags + quest_links (with source_url) on the rollup, and all-false/nil on the un-flagged trinket.

## Decisions Made
- **ItemRollup quest_links copied from the representative ViewRow** (View already fetched the links), not a new store call — honors itemrollup.go's "copy from the representative, never re-select" iron law.
- **InventorySlot quest links attached via a post-build walk** over Equipment/General/Bank + their Children (the plan's permitted alternative), keeping `buildStructuredInventory` a pure transform with the `links` map passed in (store fetch stays in public `StructuredInventory`).
- **questLinksFor returns all links** (notes_link + in_game_flag); the notes_link-only filter (D-06) is deferred to the web layer (40-02) per CONTEXT — the backend passes data through verbatim, introducing no new sink.

## Deviations from Plan

None - plan executed exactly as written. (The plan permitted either the param-threading or the post-build-walk approach for attaching InventorySlot quest links; the post-build walk was chosen, which is explicitly allowed.)

## Issues Encountered
None. One observation: the plan's Task-2 acceptance grep expected `json:"quest_links"` count == 2 in types.go on the assumption `ViewRow` lives in view.go — it actually lives in types.go, so the count is 3 (ViewRow + the two new ones). This is a pre-existing fact, not a deviation; the two new fields are correctly added and all tests pass.

## User Setup Required
None - no external service configuration required. This is pure backend plumbing; the new JSON fields are additive and consumed by the web plan (40-02).

## Next Phase Readiness
- The backend JSON contract is ready for **40-02** (web): `InventorySlot` + `ItemRollup` now carry `is_no_drop`/`is_lore`/`is_magic` + `quest_links[]` (each link with `source_url`). 40-02 mirrors these in `web/src/lib/api.ts` and renders the tile outline (ITEMUI-01) + clickable named quest links (ITEMUI-02).
- NO migration added (prod schema stays at the existing version); watcher untouched → NO `v*` tag.
- Security note for 40-02: `source_url`/`quest_name` flow out as data; the web layer applies the `safeHttpUrl` scheme allow-list + Svelte auto-escaping (T-40-02). The backend introduces no new HTML sink.

## Self-Check: PASSED

- All 8 modified source files exist on disk.
- `40-01-SUMMARY.md` created.
- Both task commits exist: `bf55fac` (Task 1), `0810552` (Task 2).

---
*Phase: 40-item-display-polish-flag-outlines-quest-links*
*Completed: 2026-06-26*
