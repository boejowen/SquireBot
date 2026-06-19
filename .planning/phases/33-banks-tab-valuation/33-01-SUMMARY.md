---
phase: 33-banks-tab-valuation
plan: 01
subsystem: api
tags: [go, sqlite, bank-valuation, compute-on-read, read-api, option-b]

# Dependency graph
requires:
  - phase: 29-data-foundation-inventory-parse-price-value-aggregation
    provides: BankValuationFor / buildBankValuation / TotalPlatinum, the name-bridged pp_rep price CTE, the bank/bot designation
  - phase: 31-characters-tab-in-game-inventory-window
    provides: RosterFor band-2 banks/bots, the session-gated read-API pattern, the {char} inventory route reused per bank in 33-02
  - phase: 32-inventory-tab-item-centric
    provides: the items.go read-route twin shape, the ItemRollup/ItemHolder is_bank holder flag the 33-02 bank item-search reuses
provides:
  - "GET /api/v1/banks — session-gated guild-wide bank+bot valuation read"
  - "compute.Banks(ctx, store) + buildBanks — widened bank+bot valuation transform (bot goods included)"
  - "BanksView / BankRowSummary snake_case append-only contract (per-bank item count/value/nullable plat + guild summary)"
  - "store.InventoryJoinBanksAndBots — dedicated bank+bot flat-row read (Option B)"
  - "store.ListBankAndBotToons — widened toon list A-Z (bots carry Plat nil)"
affects: [33-02 (web Banks tab fetches /api/v1/banks + per-bank D-04 header), 33-03 (live deploy)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Option B store widen: a dedicated twin read (not widening a shared branch) when the shared read has a second caller"
    - "Value/platinum scope asymmetry: item value includes bots, platinum stays bank-toon-gated (bot Plat nil → 0)"

key-files:
  created:
    - internal/backendsrv/compute/banks.go
    - internal/backendsrv/compute/banks_test.go
    - internal/backendsrv/readapi/banks.go
    - internal/backendsrv/readapi/banks_test.go
  modified:
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/store/coin.go
    - internal/backendsrv/store/readviews_test.go
    - internal/backendsrv/compute/types.go
    - cmd/squirebot-server/main.go

key-decisions:
  - "Option B (dedicated InventoryJoinBanksAndBots twin) — NOT widening the shared InventoryJoin(ctx,true) bankOnly branch, which has a second caller (the legacy /views/bank grid)"
  - "Item value includes guild bots (BANK-02 scope widen); platinum stays bank-toon-gated (a bot's Plat is nil → contributes 0); SetCoinTx untouched (OQ1)"
  - "compute.Banks reuses buildBankValuation (zero new SQL, name-bridge only, MR-02 seed-from-toon-list); no new migration (schema stays v13)"

patterns-established:
  - "Shared-SQL twin: factor inventoryJoinBase/inventoryJoinOrderBy + scanInventoryJoinRows so the twin and the legacy read share one body, differing only in the fixed-string WHERE predicate"
  - "Guild-wide read route: items.go twin minus the viewer id (no UserFromContext), encodes the whole view object, [] not null on the nested rows"

requirements-completed: [BANK-01, BANK-02]

# Metrics
duration: 11min
completed: 2026-06-19
---

# Phase 33 Plan 01: Banks Tab Valuation (backend) Summary

**`GET /api/v1/banks` surfaces a WIDENED bank+bot valuation — `compute.Banks` over the new `InventoryJoinBanksAndBots`/`ListBankAndBotToons` reads (Option B) so a guild bot's goods finally count toward the guild item-value total, with per-bank item count + value + nullable platinum A-Z, session-gated, no new migration.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-06-19T01:30:49Z
- **Completed:** 2026-06-19T01:41:22Z
- **Tasks:** 3
- **Files modified:** 9 (4 created, 5 modified)

## Accomplishments
- Resolved the Phase-29 valuation scope bug (Pitfall 1): `compute.BankValuationFor` silently scoped to `is_bank_toon=1` only and dropped guild-bot holdings. This plan reconciles that scope via **Option B** — a dedicated `InventoryJoinBanksAndBots` read consumed ONLY by `compute.Banks`, leaving the shared `InventoryJoin(ctx,true)` bankOnly branch (and its second caller, the legacy `/views/bank` grid) untouched.
- `compute.Banks` + pure `buildBanks` reuse `buildBankValuation`'s `pickPrice`/`pricesFromJoin` over the name-bridged rows (zero new SQL, never raw item_id), then join each toon's nullable `Plat` + a per-bank flat item count into a `BankRowSummary`, A-Z, with the guild summary (`guild_value`/`guild_unpriced`/`total_platinum`).
- `GET /api/v1/banks` registered under `webauth.RequireSession` (login-only, NOT officer, NOT public), the `items.go` twin minus the viewer id; encodes the whole `BanksView` object with `banks: []` not null on empty, V7 slog count+status only, GET-only 405, fail-closes 401 without a cookie.
- The value/platinum **asymmetry** is documented as deliberate: item value includes bots; platinum stays bank-toon-gated by `SetCoinTx` (a bot's `Plat` is nil → contributes 0). `SetCoinTx` untouched (OQ1).

## Task Commits

Each task was committed atomically:

1. **Task 1: Widened bank+bot store reads (Option B)** - `e7ddfa7` (feat, TDD: test+impl in one package)
2. **Task 2: compute.Banks + BanksView/BankRowSummary contract** - `bfda028` (feat, TDD)
3. **Task 3: readapi/banks.go route + main.go registration** - `7e43687` (feat)
4. **Style: gofmt comment-column realignment** - `c2fc69d` (style — gofmt of structs adjacent to the appends)

_Note: Tasks 1 & 2 were `tdd="true"`; the RED gate (compile-fail on the undefined methods/types) was verified before the GREEN impl in each. Because the failing test and its implementation live in the same Go package, each landed as one `feat` commit after the RED→GREEN verification rather than two._

## Files Created/Modified
- `internal/backendsrv/store/readviews.go` - **+`InventoryJoinBanksAndBots`** (dedicated `is_bank_toon=1 OR is_guild_bot=1` flat-row read, fixed-string WHERE); factored the shared `inventoryJoinBase`/`inventoryJoinOrderBy` consts + `scanInventoryJoinRows` helper so the twin and the legacy `InventoryJoin` share one byte-identical body.
- `internal/backendsrv/store/coin.go` - **+`ListBankAndBotToons`** (widened toon list A-Z; reuses `scanBankToon`; documents the value/plat asymmetry).
- `internal/backendsrv/store/readviews_test.go` - +`TestInventoryJoinBanksAndBots` (bot included, plain char excluded, legacy `InventoryJoin(true)` unchanged) + `TestListBankAndBotToons` (bot Plat nil, A-Z, legacy `ListBankToons` unchanged).
- `internal/backendsrv/compute/banks.go` - **+`Banks`/`buildBanks`** (the widened valuation transform; THE IRON LAW header — zero SQL, name-bridge only).
- `internal/backendsrv/compute/types.go` - **+`BanksView`/`BankRowSummary`** (snake_case append-only; `Plat *int64` nullable).
- `internal/backendsrv/compute/banks_test.go` - the Pitfall-1 regression (bot's priced item in `GuildValue`), nil-plat carry, MR-02 coin-only bank, A-Z, empty.
- `internal/backendsrv/readapi/banks.go` - **+`BanksHandler`/`NewBanks`** (guild-wide, no viewer id; whole-object encode; `[]` not null; V7 logging).
- `internal/backendsrv/readapi/banks_test.go` - 401-without-cookie, `banks:[]` not null, OK encodes the bank+bot view + guild summary, 405.
- `cmd/squirebot-server/main.go` - registered `GET /api/v1/banks` under `RequireSession` beside `/api/v1/items`.

## Decisions Made
- **Option B (locked in the plan, confirmed by the pattern map):** the shared `InventoryJoin(ctx,true)` bankOnly branch has TWO callers (`BankValuationFor` + the legacy `compute.Bank` grid at `/api/v1/views/bank`), so widening it in place would corrupt the legacy grid. Added a dedicated twin read instead.
- **Plat carried nullable end-to-end** (`*int64`, nil ≠ 0) — a guild bot's `Plat` is nil, never coerced to 0; it contributes 0 to `TotalPlatinum`.
- **No new migration, schema stays v13** — bank/bot designation + bank-coin shipped in v2.3; icon/statsblock in P31's 00012/00013.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Reworded doc-comments off literal grep-gate tokens**
- **Found during:** Tasks 2 & 3
- **Issue:** The plan's acceptance criteria assert grep counts of 0 for `InventoryJoin(ctx, true)` in `compute/banks.go` and for `UserFromContext`/`RequireOfficer` in `readapi/banks.go` — but my explanatory doc-comments mentioned those exact literals in prose ("NOT InventoryJoin(ctx, true)…", "no UserFromContext read", "NEVER RequireOfficer"), tripping the grep gate.
- **Fix:** Reworded the prose to describe the same intent without the literal tokens (e.g. "the bank-toons-only InventoryJoin bankOnly branch", "never reads the session identity from the request context", "never an officer-scoped gate"). No behavior change. This is the same fix-forward applied in 31/32-01 (memory: planner/executor reword comments off grep-gate tokens).
- **Files modified:** `internal/backendsrv/compute/banks.go`, `internal/backendsrv/readapi/banks.go`
- **Verification:** grep counts now 0 for all three; tests + build + vet still green.
- **Committed in:** `bfda028` (Task 2) / `7e43687` (Task 3)

**2. [Rule 3 - Blocking] gofmt realignment of adjacent struct comment columns**
- **Found during:** Task 3 close-out (gofmt -l flagged 3 files)
- **Issue:** Appending `BanksView`/`BankRowSummary` to `types.go` and adding the `InventoryJoinBanksAndBots` method body to `readviews.go` caused gofmt to re-align the trailing comment columns of nearby pre-existing structs/assignments (gofmt aligns the whole block).
- **Fix:** Ran `gofmt -w` on the affected files; committed the whitespace-only realignment separately.
- **Files modified:** `internal/backendsrv/compute/types.go`, `internal/backendsrv/store/readviews.go`, `internal/backendsrv/readapi/banks_test.go`
- **Verification:** `gofmt -l` clean; full module `go test ./...` green.
- **Committed in:** `c2fc69d` (style)

---

**Total deviations:** 2 auto-fixed (both Rule 3 - blocking/grep-gate + tooling).
**Impact on plan:** Cosmetic only — no logic change, no scope creep. Both are the established repo convention for this codebase.

## Issues Encountered
None — the plan's inlined interfaces + the pattern map's line-numbered analogs were exact; every read/seed helper, the `s.DB()` accessor, the `is_guild_bot` column, and the `BankToon.Plat *int64` nullable were as documented.

## Threat Surface
No new threat surface beyond the plan's `<threat_model>` (T-33-01..06). The new reads use compile-time fixed-string WHERE predicates (no interpolation, no name concat); the route is `RequireSession`-gated; the value math reuses the existing name-bridge (never raw item_id); slog carries count+status only. No `## Threat Flags` needed.

## Known Stubs
None — backend compute/store/route work, fully wired (the web tab that renders this lands in 33-02). No hardcoded/placeholder data paths introduced.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend half of the Banks tab is shipped and tested. `GET /api/v1/banks` is live in the binary (registered), but NOT yet deployed to prod — the live deploy is **Plan 33-03**.
- **Plan 33-02** (web Banks tab) consumes this: `fetchBanks()` over `BanksView`, the D-02 guild summary header, the per-bank D-04 value/plat header above the reused P31 `InventoryWindow`, and the BANK-03 item-search reusing the P32 `/api/v1/items` rollup filtered to `is_bank` holders.
- The legacy `/views/bank` grid + its tests are confirmed unaffected (Option B); `go test ./...` all packages green.

## Self-Check: PASSED

- All 4 created files verified on disk (compute/banks.go + _test, readapi/banks.go + _test, SUMMARY).
- All 4 commits verified in git log (e7ddfa7, bfda028, 7e43687, c2fc69d).
- `go test ./...` all packages green; `go build ./...` rc=0; `go vet` clean on store/compute/readapi.

---
*Phase: 33-banks-tab-valuation*
*Completed: 2026-06-19*
