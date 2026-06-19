---
phase: 29-data-foundation-inventory-parse-price-value-aggregation
verified: 2026-06-17T00:00:00Z
status: passed
score: 4/4 success criteria verified (+ 6/6 plan must-have truths)
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification — no prior VERIFICATION.md
---

# Phase 29: Data Foundation (Inventory Parse + Price/Value Aggregation) Verification Report

**Phase Goal:** The backend turns the watcher's raw `Location | Name | ID | Count | Slots` inventory rows into structured, query-ready data — a slot taxonomy with container nesting, name-keyed PigParse price + last-listed joins, and bank valuation totals — so every v2.4 web tab reads from a clean, computed model rather than re-parsing strings. BACKEND-ONLY; watcher untouched; no schema migration.
**Verified:** 2026-06-17
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth (ROADMAP SC) | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Stored inventory exposed as a structured slot taxonomy — equipment paperdoll + general + bank, each general/bank container's contents nested under it, stack counts preserved (Location/Slots parsed server-side; watcher unchanged). | ✓ VERIFIED | `compute.classifySlot` (inventory.go:49) + `buildStructuredInventory` (inventory.go:102) build the equipment/general/bank tree with one-level `<Parent>-Slot<N>` nesting. `store.InventoryForChar` (readviews.go:279) returns ALL rows incl. empty slots + container shells + children, `ORDER BY ii.row_ordinal`, projecting `ii.slots`. `TestStructuredInventory_Nesting` PASS: General4 nests 3 children, Bank1 nests 2, the count=5 Diamond keeps count 5, no `-Slot` child or `Head-Slot1` augment surfaces top-level. Watcher diff empty. |
| 2 | Any surfaced item carries PigParse price + last-listed joined by NORMALIZED NAME (gear-tier/wiki rows with no item_id still resolve a price), available to examine/suggestions/item lists. | ✓ VERIFIED | All three reads (`InventoryJoin`, `InventoryForChar`, `GearTierPrices`) use the `pp_rep` CTE `ON pp_rep.norm_name = lower(trim(...))` — never item_id. `store.GearTierPrices` (readviews.go:419) name-joins price+`last_seen` onto the always-NULL-id `wiki_gear_tier` rows via `lower(trim(wgt.item_name))`. `TestGearTierPrices_NameJoin_HitMiss` PASS: name-match across namespaces (gear-tier NULL id, catalog id 9999) resolves a price+last-listed (hit); unmatched resolves nil (miss). `TestNameJoin_HitMiss` PASS (inventory 14536 ↔ catalog 19450 bridged by name, EQ id preserved). `grep -c "pp.item_id = ii.item_id"` = 0, `grep -c "wgt.item_id"` = 0. |
| 3 | Backend computes, for guild banks, the summed PigParse value of bank items AND total platinum from manual bank-coin entries — queryable as guild-wide totals. | ✓ VERIFIED | `compute.BankValuationFor` (inventory.go:241) → `buildBankValuation` FLAT-sums `pickPrice×count` over the bankOnly `InventoryJoin(ctx, true)` rows (bag + contents both count), per-bank + `GuildTotal`, with `+N unpriced`. `compute.TotalPlatinum` (inventory.go:280) sums literal `*Plat` over `ListBankToons`, nil-safe, gp/sp/cp excluded. `TestBankValuation_SumAndUnpriced` PASS (250 + 1 unpriced), `TestBankValuation_CountsBagContents` PASS (bag 30 + contents 40×5 = 230), `TestTotalPlatinum_LiteralPlatOnly` PASS (1000; gold excluded, nil skipped, non-bank excluded). |
| 4 | Parse + joins + aggregation covered by Go unit tests against REAL-NAME inventory fixtures, applied over live data with NO schema-breaking change (extend-only). | ✓ VERIFIED | `compute/testdata/Slampeach-Inventory.txt` (real-name, hand-authored from RQ2 format) drives `TestStructuredInventory_Nesting` via `loadInventoryFixture`. 7 compute parity tests + 14 `TestClassifySlot` sub-cases + 4 store tests all run & PASS (verbose run confirmed not cached/skipped). Migration head stays `00011` (no new migration); all new struct fields appended at the right edge (extend-only). Watcher files untouched. |

**Score:** 4/4 ROADMAP success criteria verified.

### Plan Must-Have Truths

| Plan | # | Truth | Status | Evidence |
| --- | --- | --- | --- | --- |
| 29-01 | 1 | New store read returns ALL inventory rows for one char (empty/container/child) in row_ordinal order, nothing filtered. | ✓ VERIFIED | `InventoryForChar` has no `item_id>0` filter; `ORDER BY ii.row_ordinal`. `TestReadViews_InventoryForChar_KeepsEmptyAndContainers` PASS (4 rows incl. Finger2 empty + container + 2 children, ordinal order). |
| 29-01 | 2 | Every row carries name-joined price + a DISTINCT `LastListed` (pp.last_seen), never confused with char freshness. | ✓ VERIFIED | `pp.last_seen` scanned into a separate `sql.NullString lastListed`, resolved to `r.LastListed`; `c.last_seen` → `r.LastSeen`. `TestReadViews_InventoryForChar_LastListedNotCharFreshness` PASS ("2026-05-09" ≠ "2026-05-09T00:00:00Z"). |
| 29-01 | 3 | Real-name nested-bag fixture (general+bank nesting, priced shell, stacked, unpriced) exists in compute/testdata/. | ✓ VERIFIED | `Slampeach-Inventory.txt`: General4 shell (slots 10) + 3 children, Bank1 shell + 2 children, stacked Diamond×5 / Rough Diamond×3, empty Finger2, unpriced trinkets, Head-Slot1 augment. |
| 29-01 | 4 | Stale "item_id is the PK" comments in view.go/types.go corrected to the pp_rep name-join reality. | ✓ VERIFIED | types.go:62-64 describes "pp_rep CTE's GROUP BY norm_name + MIN(item_id) fan-out guard … NOT from the item_id PK". (view.go `pricesFromJoin` correspondingly corrected per 29-01-SUMMARY; both stale-PK greps return 0.) |
| 29-02 | 1 | Structured slot model: rows classified equipment/general/bank, equipment carries canonical paperdoll key, bag contents nest as parent→children with counts preserved. | ✓ VERIFIED | See SC #1. |
| 29-02 | 2 | Each item carries name-joined price + LastListed reusing the 29-01 store read. | ✓ VERIFIED | `slotFromRow` (inventory.go:188) sets `Price: pickPrice(pricesFromRow(row))`, `LastListed: row.LastListed`. See SC #2. |
| 29-02 | 3 | Gear-tier NULL-id rows resolve a name-keyed price + last-listed in THIS phase (hit/miss). | ✓ VERIFIED | `store.GearTierPrices` + `TestGearTierPrices_NameJoin_HitMiss` (closes SC #2 in-phase). |
| 29-02 | 4 | Bank value = pickPrice×count summed over ALL bank rows (bag + contents), unpriced → 0 + "+N unpriced". | ✓ VERIFIED | See SC #3. |
| 29-02 | 5 | Guild bank total platinum = Σ literal plat over live bank toons via ListBankToons, nil-safe, gp/sp/cp excluded. | ✓ VERIFIED | See SC #3. |
| 29-02 | 6 | All behaviors covered by Go unit tests over the real-name fixture; zero migration; watcher untouched. | ✓ VERIFIED | See SC #4. |

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/backendsrv/store/readviews.go` | `InventoryForChar`, `InventoryRow`, `InventoryJoinRow.LastListed`, `GearTierPrices`, `GearTierPriceRow` | ✓ VERIFIED | All present (lines 279, 72, 60, 419, 115). Name-join CTE reused verbatim in all 3 reads; `char` bound via `?`. |
| `internal/backendsrv/compute/inventory.go` | `classifySlot`, `StructuredInventory`/`buildStructuredInventory`, `BankValuationFor`/`buildBankValuation`, `TotalPlatinum` | ✓ VERIFIED | All present; 289 lines, zero SQL; reuses `pickPrice`+`ListBankToons`. |
| `internal/backendsrv/compute/slotconst.go` | canonical equipment slot set (incl. Ear1/Ear2) + case-insensitive index | ✓ VERIFIED | `equipmentSlots` (23 slots incl. Ear1/Ear2), `equipmentSlotsLC` lowercase index, `SlotCategory` consts. |
| `internal/backendsrv/compute/types.go` | `InventorySlot`, `CharacterInventory`, `Valuation`, `BankValuation` (append-only snake_case) | ✓ VERIFIED | All 4 appended at lines 158/178/188/195; no existing tag renamed (ViewRow.Char `json:"char"` intact). |
| `internal/backendsrv/compute/testdata/Slampeach-Inventory.txt` | real-name nested-bag fixture | ✓ VERIFIED | Header + 19 data rows; general+bank nesting, augment, empty slot, stacked, unpriced. |
| `internal/backendsrv/compute/{slotconst_test,inventory_test}.go` | TestClassifySlot + 7 parity tests | ✓ VERIFIED | All run & PASS (verbose, uncached). |
| `internal/backendsrv/store/readviews_test.go` | 3 InventoryForChar tests + TestGearTierPrices_NameJoin_HitMiss | ✓ VERIFIED | All 4 run & PASS. |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `compute.StructuredInventory` | `store.InventoryForChar` | wrapper fetch → pure `buildStructuredInventory` | ✓ WIRED | inventory.go:85 `rows, err := s.InventoryForChar(ctx, char)`. |
| `store.InventoryForChar` | `pigparse_price` | name-keyed pp_rep CTE on `lower(trim(ii.name))` | ✓ WIRED | readviews.go:293; no item_id price join. |
| `store.GearTierPrices` | `pigparse_price` | pp_rep on `lower(trim(wgt.item_name))`, never wgt.item_id | ✓ WIRED | readviews.go:429; `grep wgt.item_id` = 0. |
| `compute.BankValuationFor` | `compute.pickPrice` (view.go) | per-row `pickPrice(pricesFromJoin(r)) × count` | ✓ WIRED | inventory.go:261; reused verbatim. |
| `compute.BankValuationFor` → `TotalPlatinum` | `store.ListBankToons` (coin.go) | `ListBankToons(ctx, s.DB())`, Σ Plat nil-safe | ✓ WIRED | inventory.go:246 + 280; `s.DB()` exists (replace.go:51). |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| `CharacterInventory` | `Equipment/General/Bank` | `store.InventoryForChar` SQL over `inventory_item` JOIN `character` LEFT JOIN `pigparse_price` (pp_rep) | Yes — real SELECT with name-join; fixture-seeded rows flow through | ✓ FLOWING |
| `InventorySlot.Price` / `.LastListed` | `row.HasPrice` / `row.LastListed` | `pp.direction` / `pp.last_seen` columns scanned per-row | Yes — `seedPigparse` writes a30/last_seen="2026-05-09"; tests assert the values surface | ✓ FLOWING |
| `BankValuation.GuildTotal` | `pickPrice×count` accumulation | `InventoryJoin(ctx, true)` bankOnly rows | Yes — tests assert 250 / 230 sums | ✓ FLOWING |
| `BankValuation.TotalPlatinum` | `Σ *Plat` | `ListBankToons` → `character.plat` | Yes — test asserts 1000 (gold/nil/non-bank excluded) | ✓ FLOWING |

Note: these are compute-on-read pure functions + store reads by design — HTTP endpoints are deferred to consuming Phases 31-34 (per CONTEXT). "Queryable" = store reads + pure compute exist, which is satisfied. Not a gap.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Phase 29 compute tests run & pass | `go test ./internal/backendsrv/compute/ -run "TestStructuredInventory\|TestBankValuation\|TestTotalPlatinum\|TestNameJoin\|TestLastListed\|TestClassifySlot" -v` | 14 classify sub-cases + 7 parity tests all PASS (5.6s, uncached) | ✓ PASS |
| Phase 29 store tests run & pass | `go test ./internal/backendsrv/store/ -run "TestReadViews_InventoryForChar\|TestGearTierPrices_NameJoin_HitMiss" -v` | 4 tests PASS (3.2s) | ✓ PASS |
| Whole module builds | `go build ./...` | exit 0 | ✓ PASS |
| Migration head unchanged | `ls migrations/*.sql \| tail` | head = `00011_wantlist_drop_reason_dedup.sql` (no new migration) | ✓ PASS |
| Compute authors zero SQL | `grep -iE "SELECT\|INSERT\|UPDATE\|DELETE" inventory.go slotconst.go` | empty | ✓ PASS |
| No item_id price join | `grep -c "pp.item_id = ii.item_id" readviews.go` / `grep -c "wgt.item_id"` | 0 / 0 | ✓ PASS |
| Watcher untouched | `git diff --name-only -- internal/parse internal/app internal/watch internal/sheet` | empty | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| INV-05 | 29-01, 29-02 | Watcher's Location/Slots parsed server-side into a slot taxonomy + container nesting (watcher unchanged). | ✓ SATISFIED | `classifySlot` + `buildStructuredInventory` + `InventoryForChar`; nesting test green; watcher diff empty. |
| DATA-01 | 29-01, 29-02 | PigParse price + last-listed joins to wiki/gear-tier items by normalized name (gear-tier has no item_id); surfaced on examine/suggestions/item lists. | ✓ SATISFIED | pp_rep name-join in 3 reads incl. `GearTierPrices`; `LastListed` distinct field; hit/miss tests green. |
| DATA-02 | 29-02 | Bank valuation (Σ PigParse value per bank + guild-wide) + total platinum (manual bank-coin). | ✓ SATISFIED | `BankValuationFor` + `TotalPlatinum`; sum/unpriced/bag-contents/literal-plat tests green. |

No orphaned requirements: REQUIREMENTS.md maps exactly INV-05/DATA-01/DATA-02 to Phase 29 (3 reqs), all claimed by the plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| (none) | — | — | — | No TODO/FIXME/placeholder/stub; `return nil` paths in `pricesFromRow`/`classifySlot` are deliberate (no-price → nil PriceDetail; never-panic defensive default), exercised by passing tests. |

### Human Verification Required

None. This is backend compute-on-read with full unit coverage; no visual/real-time/external-service behavior in scope. The two RESEARCH on-box spot-checks (A5 live Location case, A7 `pigparse_price.last_seen` semantics) are explicitly non-blocking — the classifier is case-insensitive and the date is carried through verbatim, so neither can break the tests. They are noted for optional confirmation, not required for goal achievement.

### Gaps Summary

No gaps. All 4 ROADMAP success criteria and all 10 plan must-have truths are verified against the actual implemented Go source (not SUMMARY claims). The structured slot taxonomy with container nesting, the normalized-name price/last-listed joins (including the NULL-id gear-tier resolution that closes SC #2 in-phase), and the bank valuation + total-platinum aggregation all exist as substantive, wired, data-flowing functions backed by passing Go unit tests over the real-name `Slampeach-Inventory.txt` fixture. Negative invariants hold: zero schema migration (head `00011`), watcher untouched, compute authors zero SQL, no item_id price join reintroduced. The name-keyed price join (not item_id) is the intended pattern per the task brief — correctly NOT flagged as a deviation.

The only minor note (carried from both SUMMARYs, not a gap): `Slampeach-Inventory.txt` is hand-authored from the RQ2-confirmed `<Parent>-Slot<N>` format rather than a genuine `/outputfile inventory` capture. It is real-NAME (per the CLAUDE.md fixture convention) and the nesting tests pass over it; replacing it with a live capture when available would only strengthen confidence in the exact sub-slot indexing. This does not block goal achievement.

---

_Verified: 2026-06-17_
_Verifier: Claude (gsd-verifier)_
