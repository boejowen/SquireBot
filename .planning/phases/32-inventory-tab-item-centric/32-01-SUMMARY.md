---
phase: 32-inventory-tab-item-centric
plan: 01
subsystem: api
tags: [go, compute-on-read, sqlite, item-rollup, readapi, requireSession]

# Dependency graph
requires:
  - phase: 29-data-foundation-inventory-parse-price-value-aggregation
    provides: compute.View (guild-wide ViewRows w/ name-bridged pp_rep price), classifySlot/splitChild slot taxonomy
  - phase: 31-characters-tab-in-game-inventory-window
    provides: store.RosterFor/RosterRow (is_mine + bank/bot flags), item_master.icon_id+statsblock (schema v13, migrations 00012/00013)
provides:
  - "compute.Items(ctx, store, viewerID) → []ItemRollup grouping all guild holdings by normalized name"
  - "compute.ItemRollup / ItemHolder JSON contract (snake_case, append-only)"
  - "store.ItemMasterIconStats — id-keyed item_master icon_id/statsblock map read"
  - "readapi.ItemsHandler + NewItems(st) serving GET /api/v1/items under RequireSession"
affects: [32-02 (web inventory tab consumes GET /api/v1/items + the ItemRollup/ItemHolder contract), 33 (Banks tab — same rollup/holder shape)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Item rollup: group compute.View rows by lower(trim(name)) — NEVER item_id — into one-row-per-item with per-holder detail"
    - "Public-fn → pure-helper compute split (Items → buildItemRollups) for table-testability"
    - "Representative-id icon/stats lookup (item_master EQ namespace) joined separately, leaving the shared InventoryJoin/View query untouched"

key-files:
  created:
    - internal/backendsrv/compute/itemrollup.go
    - internal/backendsrv/compute/itemrollup_test.go
    - internal/backendsrv/compute/itemrollup_internal_test.go
    - internal/backendsrv/readapi/items.go
    - internal/backendsrv/readapi/items_test.go
  modified:
    - internal/backendsrv/compute/types.go
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/store/readviews_test.go
    - cmd/squirebot-server/main.go

key-decisions:
  - "Group key = normalized name (lower(trim)); representative ViewRow.ID used ONLY for the id-correct item_master icon/stats lookup, never for price"
  - "Rollup composes compute.View (reuses the name-bridged representative price) rather than re-reading InventoryJoin or re-selecting prices"
  - "Icon/stats sourced via a tiny new full-table store read (Pattern-1 option b), not by widening the shared InventoryJoin/View query"
  - "Bagged-copy holder labels 'Bag' (the parent bag display name is not on the ViewRow — A2)"

patterns-established:
  - "Pattern: compute.Items groups guild holdings by normalized name with summed qty + distinct holder count + viewer is_mine + per-holder slot/qty/last-synced"
  - "Pattern: session-gated guild-wide read route reading the viewer id from webauth.UserFromContext, [] not null, V7 count+status logging"

requirements-completed: [ITEM-01, ITEM-02, ITEM-03]

# Metrics
duration: 12min
completed: 2026-06-18
---

# Phase 32 Plan 01: Inventory Tab (Item-Centric) — Backend Item Rollup Summary

**A new compute.Items(ctx, store, viewerID) that groups every guild holding by normalized name into one-row-per-item rollups (summed qty, distinct holder count, viewer is_mine, name-keyed price/wiki, id-correct icon/stats, per-holder slot/qty/last-synced), served at a new session-gated GET /api/v1/items.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-06-18T17:51:24Z
- **Completed:** 2026-06-18T18:03:19Z
- **Tasks:** 3
- **Files modified:** 9 (5 created, 4 modified)

## Accomplishments
- `compute.Items` + pure `buildItemRollups` group all `View` rows by `lower(trim(name))` (never item_id) into one `ItemRollup` per name: Σ stack count, distinct holder count, `is_mine` propagated from any viewer-assigned holder, representative name-keyed price/wiki/quest, id-correct `item_master` icon/statsblock, and a `holders[]` carrying `{char, slot_label, qty, last_synced, is_mine, is_bank}`.
- `ItemRollup` / `ItemHolder` snake_case JSON contract appended to `compute/types.go` (append-only; reuses the existing `PriceDetail`).
- `store.ItemMasterIconStats` — a `?`-free full-table read returning `map[int64]IconStats` (NULL icon_id → 0, NULL statsblock → ""), id-correct in the watcher's own EQ namespace.
- `GET /api/v1/items` registered under `webauth.RequireSession` (login-only, NOT public, NOT officer; distinct from the P19 `/items/search` catalog route) — viewer id is server-truth from the session, `[]` not null on empty, V7 slog (count+status only), GET-only 405, proven to 401 fail-closed without a cookie.
- No new goose migration (schema stays v13). Watcher untouched.

## Task Commits

1. **Task 1: ItemRollup/ItemHolder structs + ItemMasterIconStats store read** — `39b6660` (feat)
2. **Task 2: compute.Items + buildItemRollups + table tests** — `8bc2cbd` (feat)
3. **Task 3: readapi/items.go (GET /api/v1/items, RequireSession) + main.go registration + tests** — `446fa2d` (feat)

_Note: the two TDD tasks were committed as single feat commits (the failing-test + implementation landed together per task), with the structural/build acceptance criteria as the gate._

## Files Created/Modified
- `internal/backendsrv/compute/types.go` — appended `ItemRollup` + `ItemHolder` (snake_case, append-only; reuses `PriceDetail`).
- `internal/backendsrv/store/readviews.go` — added `IconStats` + `ItemMasterIconStats(ctx)` (id-keyed full-table icon/statsblock read).
- `internal/backendsrv/store/readviews_test.go` — `TestItemMasterIconStats` (populated + NULL-zero-value cases).
- `internal/backendsrv/compute/itemrollup.go` — `Items` + pure `buildItemRollups` + `slotLabel` (group by normalized name; reuse classifySlot/splitChild; price copied from View).
- `internal/backendsrv/compute/itemrollup_test.go` — end-to-end `TestItems_GroupsByNameWithHoldersAndFlags` over a seeded DB.
- `internal/backendsrv/compute/itemrollup_internal_test.go` — white-box `TestBuildItemRollups` + `TestSlotLabel`.
- `internal/backendsrv/readapi/items.go` — `ItemsHandler` + `NewItems(st)` serving `compute.Items`.
- `internal/backendsrv/readapi/items_test.go` — RequireSession 401, []-not-null, grouped-rollup 200, non-GET 405.
- `cmd/squirebot-server/main.go` — one route registration line under `webauth.RequireSession`.

## Decisions Made
- **Group by normalized name, never item_id** — the EQ-inventory vs PigParse/gear-tier id-namespace split makes name the only consistent key; the representative `ViewRow.ID` is used ONLY for the id-correct `item_master` icon/stats lookup, never for price (price is name-bridged in the store's `pp_rep` CTE, copied from `View`).
- **Compose `compute.View` + a tiny `item_master` icon/stats map** (Pattern-1 option b) rather than widening the shared `InventoryJoin`/`View`/`Bank` query — keeps the tested SQL untouched and the grouping table-testable.
- **Bagged-copy holder slot label = "Bag"** — the parent bag's display name is not joined onto the `ViewRow` (A2); the simplest correct label drops it.
- **White-box internal test for the pure `buildItemRollups`/`slotLabel`** — matches the established `pickprice_internal_test.go` convention (unexported helpers tested in `package compute`), with the end-to-end `Items` test in the external `compute_test` package.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected a wrong test expectation (Bone Chips summed_qty)**
- **Found during:** Task 2 (compute.Items table test)
- **Issue:** The end-to-end test asserted Bone Chips `summed_qty == 4`, but the shared `seedInv` helper hardcodes `count=1` per row, so two copies sum to 2, not 4. The expectation (not the code) was wrong.
- **Fix:** Corrected the assertion to `summed_qty == 2` (still proving bag-vs-loose copies of one name collapse into one rollup) and clarified the comment.
- **Files modified:** internal/backendsrv/compute/itemrollup_test.go
- **Verification:** `go test ./internal/backendsrv/compute/...` green.
- **Committed in:** `8bc2cbd` (Task 2 commit)

**2. [Rule 3 - Blocking] Reworded comments off the literal `pickPrice` / `NewItemSearch`/`SearchCatalog` tokens**
- **Found during:** Task 3 (grep-gate verification)
- **Issue:** The plan's acceptance grep gates require `itemrollup.go` to NOT contain `pickPrice` and `items.go` to NOT contain `NewItemSearch`/`SearchCatalog`. My explanatory comments mentioned those tokens literally (the code never calls them), tripping the gates — the same fix-forward the P31 work applied for the `@html` grep.
- **Fix:** Reworded the comments to describe the behavior without the literal tokens (e.g. "representative price — already selected + name-bridged by View"; "does NOT reuse the P19 catalog-search constructor or store read").
- **Files modified:** internal/backendsrv/compute/itemrollup.go, internal/backendsrv/readapi/items.go
- **Verification:** grep gates now return 0 for the forbidden tokens; `go build ./...` + `go test ./internal/backendsrv/{compute,readapi}/...` green.
- **Committed in:** `446fa2d` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both were corrections to test/comment text, not behavior changes. No scope creep; the three tasks landed exactly as designed.

## Issues Encountered
- The `compute_test` external package cannot call the unexported `buildItemRollups`. Resolved by following the established `pickprice_internal_test.go` convention: a white-box `package compute` test for the pure helper, plus the end-to-end `Items` test in `compute_test`.
- The `seedItemMaster` helpers (both compute + store) do not write `icon_id`/`statsblock`; resolved by stamping those columns with a follow-up `UPDATE` in the tests.

## Verification
- `go test ./internal/backendsrv/compute/...` and `./readapi/...` — all pass (table tests + the RequireSession 401 + [] not null + grouped-rollup 200).
- `go build ./...` exits 0; `go vet ./internal/backendsrv/{compute,store,readapi}/...` clean.
- Full module regression `go test ./...` — all 31 packages ok (incl. `cmd/squirebot-server` compiling the new route).
- Grep gates: `GET /api/v1/items` present in main.go under RequireSession; `compute.Items` present in items.go; `strings.ToLower(strings.TrimSpace(` present in itemrollup.go; `classifySlot(`+`splitChild(` present; `pickPrice` / `NewItemSearch` / `SearchCatalog` absent where required.
- No new goose migration (icon_id/statsblock already at schema v13).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The `GET /api/v1/items` route + the `ItemRollup`/`ItemHolder` snake_case contract are ready for Plan 32-02 (the SvelteKit master-detail Inventory tab): `fetchItems()` wrapper, pure `items.ts` (viewer-first/filter/sort-holders), bespoke selectable list, `ExaminePanel` reuse, and the `/characters?c=` holder deep-link.
- Deploy note: NO migration this phase, but the backend binary MUST restart to register the new route (a binary swap + web atomic-swap, per `docs/backend-deploy.md`; take the R2 backup anyway). The rendered web tab (32-02) MUST be browser-smoked on a DEPLOYED build (node vitest is DOM-blind).

## Self-Check: PASSED

All 5 created code/test files + the SUMMARY exist on disk; all 3 task commits (`39b6660`, `8bc2cbd`, `446fa2d`) are in the git log.

---
*Phase: 32-inventory-tab-item-centric*
*Completed: 2026-06-18*
