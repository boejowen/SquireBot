---
phase: 14-web-frontend
plan: 01
subsystem: api
tags: [go, sqlite, modernc, read-api, compute, parity, view, bank, gear_check, spell_check, pigparse, wiki]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api
    provides: "SQLite schema (character/inventory_item/spellbook_entry/item_master/pigparse_price/wiki_spells/wiki_gear_tier/quest_items), store.Store + store.NewTestDB, goose migrations"
  - phase: 12-enrichment-job-migration
    provides: "the dimension tables this read layer joins (item_master, pigparse_price, wiki_spells, wiki_gear_tier, quest_items) + enrich.WIKI_SLOT_TO_INV_SLOTS + the TEXT direction encoding (strconv.Itoa(t))"
provides:
  - "store/readviews.go — 8 parameterized read methods feeding the 4 view builders (InventoryJoin, QuestLinksByItem, WikiGearTiers, WikiSpells, CharsWithMeta, InventoryByChar, SpellbookNormalizedByChar, CharFreshness)"
  - "compute package — Go reimpl of buildView/buildBank/buildGearCheck/buildSpellCheck over the live store, with v1 parity proven by Go table-tests"
  - "the FIXED snake_case JSON row contract (ViewRow/BankRow/PriceDetail/QuestLink/BankView/CoinTotals/GearCheckRow/SpellCheckRow) that Plan 03 encodes and Plans 02/04 consume"
affects: [14-02-search-tooltip-theme-ports, 14-03-read-handlers-cors, 14-04-svelte-client, 15-admin-web-forms]

# Tech tracking
tech-stack:
  added: []  # no new deps — pure Go over the existing modernc.org/sqlite store + stdlib
  patterns:
    - "Read-side (*Store) methods mirror itemids.go (QueryContext -> rows.Next/Scan -> rows.Err, %w-wrapped); store-local result structs keep the dependency compute -> store (store never imports compute)"
    - "compute authors ZERO SQL — it consumes typed store structs; the four builders are pure transforms (buildViewRows/buildGearCheckRows/buildSpellCheckRows) split out so they unit-test without a DB"
    - "v1 parity via translated table-tests: each v1 vitest seed-array + expected tuple becomes a Go table-test over store.NewTestDB (RESEARCH approach 1)"

key-files:
  created:
    - "internal/backendsrv/store/readviews.go"
    - "internal/backendsrv/store/readviews_test.go"
    - "internal/backendsrv/compute/types.go"
    - "internal/backendsrv/compute/view.go"
    - "internal/backendsrv/compute/bank.go"
    - "internal/backendsrv/compute/eqconst.go"
    - "internal/backendsrv/compute/gearcheck.go"
    - "internal/backendsrv/compute/spellcheck.go"
    - "internal/backendsrv/compute/view_test.go"
    - "internal/backendsrv/compute/bank_test.go"
    - "internal/backendsrv/compute/gearcheck_test.go"
    - "internal/backendsrv/compute/spellcheck_test.go"
    - "internal/backendsrv/compute/pickprice_internal_test.go"
    - "internal/backendsrv/compute/fixtures_test.go"
  modified: []

key-decisions:
  - "JSON tags are snake_case (FIXED cross-plan contract): char/slot/item/id/count/wiki_url/price/last_synced/wiki_summary/is_quest_item/prices/quest_links; PriceDetail direction/a30/t30; QuestLink quest_name/source; BankView rows/coin; CoinTotals pp/gp/sp/cp; GearCheckRow + SpellCheckRow likewise"
  - "Price is *float64 (nil when neither WTS nor WTB a30>0) so JSON encodes null and the client renders the Price column blank — replaces the v1 '' sentinel"
  - "pigparse_price.direction is TEXT; stored values are '0'=WTS / '1'=WTB / '2'=BOTH (confirmed from the P12 job source enrich/pigparse.go:42 = strconv.Itoa(t)); pickPrice compares the stringified direction (directionWTS/directionWTB consts)"
  - "Bank Coin is *CoinTotals, ALWAYS nil in P14 (no fabricated 0pp) — ADMIN-05 fills it in P15; CoinTotals defined so the JSON shape is stable"
  - "Spellbook<->wiki join is a direct set membership on the materialized normalized_name (lower(trim)) — no recompute, since both tables already store it identically"
  - "InventoryJoin (view/bank) excludes empty slots (item_id > 0) matching v1; InventoryByChar (gear_check) keeps all named items (matches on slot token + name, not id)"

patterns-established:
  - "compute -> store -> SQLite read pipeline (D-01); compute is the query/compute layer, the HTTP handlers (Plan 03) compose it"
  - "Pure-transform split: each builder has a public ctx+store entry + an inner pure func over store structs for DB-free unit testing"
  - "Sort owned by the store ORDER BY for view/bank (no re-sort); owned by the builder's sort.SliceStable for gear/spell (Char->tierRank->slot->recommended; Char->level->spell)"

requirements-completed: [BACKEND-05, WEB-02]

# Metrics
duration: 73min
completed: 2026-05-30
---

# Phase 14 Plan 01: Read API Compute + Data Layer Summary

**The four v1 Apps Script view builders (view / bank / gear_check / spell_check) reimplemented in Go over the live SQLite store, with v1 OK/OTHER/MISSING + KNOWN/MISSING parity proven by Go table-tests translated from the v1 vitest fixtures, plus the 8 parameterized store read methods they consume — the compute/data half of BACKEND-05 (the HTTP handlers are Plan 03).**

## Performance

- **Duration:** ~73 min
- **Started:** 2026-05-30T14:40:00Z (approx)
- **Completed:** 2026-05-30T15:53:00Z
- **Tasks:** 3 (all TDD)
- **Files created:** 14 (2 store, 12 compute incl. tests)

## Accomplishments

- **`store/readviews.go` — 8 parameterized read methods** (the relational/data layer, D-01): `InventoryJoin(bankOnly)` (the view/bank join, fixed-string `bankOnly` WHERE switch, excludes empty slots, one-price-row-per-item), `QuestLinksByItem` (grouped `map[int64][]QuestLinkRow`), `WikiGearTiers`, `WikiSpells`, `CharsWithMeta`, `InventoryByChar` (gear-check per-char slot inputs), `SpellbookNormalizedByChar` (KNOWN/MISSING set), `CharFreshness` (Plan 03 `/api/v1/meta`). All mirror `itemids.go` exactly; store-local result structs; `?`-placeholders only; `slog` error-only.
- **`compute` package — Go reimpl of all four builders** (D-02 / WEB-02): `View` + `Bank` (enrichment inline per D-03, plain wiki URLs, `pickPrice` with the Pitfall-6 TEXT-direction fix, bank Coin nil), `GearCheck` (slot-pair match reusing `enrich.WIKI_SLOT_TO_INV_SLOTS`, load-bearing OK/OTHER/MISSING branch order, Iksar-iff-IKS, tierRank sort), `SpellCheck` (normalized_name KNOWN/MISSING, level gate, classless/level<1 skip).
- **WEB-02 parity is test-provable:** the v1 `buildGearCheck.test.ts` + `buildSpellCheck.test.ts` seed-arrays + expected `(Char,Tier,Slot,Have,Recommended,Status)` / `(Char,Level,Spell,Status)` tuples were translated into Go table-tests over `store.NewTestDB` and assert the same output for the same input.
- **The FIXED JSON contract is locked + documented** in `compute/types.go`'s header (snake_case tags, `Price *float64`, `PriceDetail.Direction` string `"0"`/`"1"`/`"2"`, `BankView.Coin` nil), so Plan 03 encodes it and Plans 02/04 consume it without re-deriving names.

## Task Commits

Each task was committed atomically (TDD; tests + impl landed together per task since the Go test cannot compile without the method/type signatures):

1. **Task 1: Store read methods (readviews.go) + parity test** — `0dc3b35` (feat)
2. **Task 2: compute view + bank (pickPrice + enrichment inline) + parity tests** — `bbe681e` (feat)
3. **Task 3: compute gear_check + spell_check (WEB-02 parity heart) + table-tests** — `f0efa8d` (feat)

**Plan metadata:** _(this SUMMARY + STATE/ROADMAP)_ committed separately.

## Files Created/Modified

- `internal/backendsrv/store/readviews.go` — 8 read methods + store-local result structs (InventoryJoinRow, QuestLinkRow, WikiGearTierRow, WikiSpellRow, CharMeta, InvSlotItem, CharFreshness).
- `internal/backendsrv/store/readviews_test.go` — seeded-temp-DB parity test (rows + grouping for all 8 methods; empty-slot exclusion; bankOnly scoping; quest-link grouping).
- `internal/backendsrv/compute/types.go` — the 8 exported row/view structs + the documented snake_case JSON contract + direction-encoding rationale.
- `internal/backendsrv/compute/view.go` — `View()` + `pickPrice` (TEXT-direction fix; `directionWTS="0"`/`directionWTB="1"`) + the pure `buildViewRows` transform.
- `internal/backendsrv/compute/bank.go` — `Bank()` (bankOnly join; Coin nil).
- `internal/backendsrv/compute/eqconst.go` — only the 3-entry `tierRank` map (slot-pair map reused from enrich).
- `internal/backendsrv/compute/gearcheck.go` — `GearCheck()` + pure `buildGearCheckRows` (slot-pair match, branch order, sort).
- `internal/backendsrv/compute/spellcheck.go` — `SpellCheck()` + pure `buildSpellCheckRows` (normalized_name join, level gate, sort).
- `internal/backendsrv/compute/{view,bank,gearcheck,spellcheck}_test.go` — parity tests translated from the v1 vitest oracles.
- `internal/backendsrv/compute/pickprice_internal_test.go` — white-box pickPrice table-test (WTS/WTB/neither).
- `internal/backendsrv/compute/fixtures_test.go` — shared raw-insert seed helpers (compute_test package, over store.NewTestDB).

## Decisions Made

- **Direction stored values confirmed from source** (not yet queried on the live box): the P12 daily job stores `direction = strconv.Itoa(t)` where `t` is `0=WTS / 1=WTB / 2=BOTH` (`internal/backendsrv/enrich/pigparse.go:42-44`), so the TEXT column holds `"0"`/`"1"`/`"2"`. `pickPrice` compares the stringified value via the `directionWTS`/`directionWTB` consts. A runtime `SELECT DISTINCT direction FROM pigparse_price` on the VPS is documented in `view.go` as a belt-and-suspenders confirmation, not a gate (the encoding is fixed by the job's code). **Plan 03/04 should treat the JSON `direction` field as a string `"0"`/`"1"`/`"2"`.**
- **`pickPrice` returns `*float64`** (nil sentinel) rather than the v1 `number | string` `''` sentinel — cleaner JSON (`null` vs an empty-string-in-a-number-field). The client renders blank on null.
- **Spellbook<->wiki join needs no recompute:** both `spellbook_entry.normalized_name` (replace.go:169) and `wiki_spells.normalized_name` (enrich.go:248) are materialized as `lower(trim(name))`, so `SpellCheck` is a direct set membership. Tests seed the normalized value via `lower(trim())` (NOT the v1 vitest fixtures' alphanumeric-strip variant, which the DB does not use); the v1 "capitalization + whitespace" case still passes under `lower(trim)`.
- **`InventoryByChar` (gear_check) deliberately does NOT filter `item_id`** (unlike `InventoryJoin`): gear_check matches on the inventory slot token + item name, not the id, so an item with a 0/NULL id in an equipment slot is still considered; only truly empty-named rows are skipped (matches v1 `readInventoriesByChar`).

## Deviations from Plan

None affecting behavior or scope. The plan's `<interfaces>` matched the codebase exactly (store struct, `NewTestDB`, `enrich.WIKI_SLOT_TO_INV_SLOTS`, schema column names), and the v1 parity oracle translated cleanly.

**One cosmetic adjustment (not a behavioral deviation):** two `view.go` code comments originally contained the literal token `HYPERLINK` while documenting that the Sheet's `=HYPERLINK(...)` formula machinery is dropped. The plan's acceptance criterion is `grep -c "HYPERLINK" view.go bank.go` == 0. The comments were reworded ("Sheet hyperlink-formula cell string" / "not a Sheet formula") to satisfy the literal grep while preserving the documented intent. No code behavior changed.

**Fixture-translation note (parity faithfulness):** the v1 vitest `seedWikiSpells` helper computes `normalized = name.toLowerCase().replace(/[^a-z0-9]/g,'')` (alphanumeric strip), but the production DB normalizes both sides as `lower(trim(name))`. The Go parity tests seed the normalized value with `lower(trim)` to match the **DB** contract (the join key the code actually uses), not the v1 test helper's stripped variant. The v1 expected status tuples are unchanged by this — every v1 spell case still produces the same KNOWN/MISSING result.

## Issues Encountered

- **External-package test seam for the unexported `pickPrice`:** the view/bank parity tests live in the external `compute_test` package (to use `store.NewTestDB`), which can't call the unexported `pickPrice`. Resolved by putting the `pickPrice` table-test in a white-box `package compute` file (`pickprice_internal_test.go`) — no exported test-only seam was added, so the production API stays clean and the acceptance grep `func pickPrice ... *float64` holds.

## Threat Surface

No new security-relevant surface beyond the plan's `<threat_model>`. This plan adds NO HTTP endpoints, NO auth paths, NO migrations, and NO schema changes (read-only SELECTs over existing tables). Per T-14.01-03 (disposition: accept, owned downstream), the compute row structs deliberately carry RAW wiki/user-controlled strings (item/char/quest names, wiki summary); HTML escaping is the client's obligation in Plans 02/04 (`composeNotes.ts` + Svelte `{}`), and the read API is the data layer, not the escaping layer. Documented in `compute/types.go`.

## Known Stubs

- **`BankView.Coin` is always nil in P14** — this is an INTENTIONAL, documented deferral, not an oversight. `/outputfile inventory` carries no coin data; the admin web form that records coin (ADMIN-05) ships in **P15**. `CoinTotals` is defined so the JSON shape is already stable for P15. The client renders "Coin: not yet recorded" on null; it must never fabricate `0pp` as real data. This does not block P14's goal (the bank inventory grid is fully functional).

## Next Phase / Next Plan Readiness

- **Plan 14-03 (read handlers + CORS)** can now compose `compute.View` / `compute.Bank` / `compute.GearCheck` / `compute.SpellCheck` + the `store.CharFreshness` `/meta` feed behind `GET /api/v1/views/*` handlers (mirror `whoami.go`, drop the bearer guard per D-04) and JSON-encode the structs directly. The snake_case contract is locked.
- **Plans 14-02 / 14-04 (search/tooltip/theme ports + Svelte client)** can rely on the documented JSON field names — notably `direction` is a string `"0"`/`"1"`/`"2"`, `price` is nullable, `prices[]` carries the per-direction `a30`/`t30` for the tooltip, and `quest_links[]` carries `quest_name`/`source`.
- **No blockers.** `go test ./internal/backendsrv/...` green (store + compute, incl. all parity tests); `go vet ./internal/backendsrv/...` and `go build ./...` exit 0; whole-repo `go test ./...` green (no v1 regression).

## Self-Check: PASSED

- All 14 created Go files verified present on disk (2 store, 12 compute incl. tests).
- All 3 task commits verified in git log: `0dc3b35`, `bbe681e`, `f0efa8d`.
- `go test ./internal/backendsrv/...` green; `go vet ./internal/backendsrv/...` + `go build ./...` exit 0; whole-repo `go test ./...` green (no regression).

---
*Phase: 14-web-frontend*
*Completed: 2026-05-30*
