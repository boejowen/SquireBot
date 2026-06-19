# Phase 29: Data Foundation — Inventory Parse + Price/Value Aggregation - Pattern Map

**Mapped:** 2026-06-17
**Files analyzed:** 11 (4 CREATE + 7 MODIFY)
**Analogs found:** 11 / 11 (every new/modified file has an in-repo analog — Phase 29 is recombination of shipped seams, not greenfield)

> Backend-only Go phase (`internal/backendsrv`). Pure **compute-on-read** over SQLite — NO schema migration, NO web/HTTP, watcher untouched. The seam to mirror everywhere is `compute/view.go` (pure transform) ↔ `store/readviews.go` (parameterized SELECT/JOIN). `compute` authors ZERO SQL; `store` authors all SQL with `?` placeholders only.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/compute/inventory.go` (CREATE) | service (pure transform) | transform / CRUD-read | `internal/backendsrv/compute/view.go` | exact (same package, same pure-transform shape) |
| `internal/backendsrv/compute/slotconst.go` (CREATE, optional) | config (lookup tables) | transform | `internal/backendsrv/compute/eqconst.go` + `enrich/eqconst.go` | exact |
| `internal/backendsrv/compute/inventory_test.go` (CREATE) | test | transform | `internal/backendsrv/compute/bank_test.go` | exact |
| `internal/backendsrv/compute/testdata/<Real>-Inventory.txt` (CREATE) | fixture | file-I/O | `internal/parse/testdata/sample-inventory.txt` | role-match (flat synthetic → needs nesting) |
| `internal/backendsrv/store/readviews.go` (MODIFY) | model / store-read | request-response (SELECT/JOIN) | self — `InventoryJoin` / `InventoryByChar` in same file | exact |
| `internal/backendsrv/store/readviews_test.go` (MODIFY) | test | request-response | self — `TestReadViews_InventoryJoinAndGrouping` | exact |
| `internal/backendsrv/compute/types.go` (MODIFY, append) | model (JSON contract) | transform | self — `ViewRow` / `BankView` / `CoinTotals` | exact |
| `internal/backendsrv/compute/view.go` (MODIFY, comment fix only) | service | transform | self — lines 84-95 | exact |
| `internal/backendsrv/compute/types.go` (MODIFY, comment fix) | model | transform | self — lines 61-63 | exact |
| `internal/backendsrv/compute/fixtures_test.go` (MODIFY) | test helper | transform | self — `seedInv` / `seedPigparse` | exact |
| `internal/backendsrv/store/coin.go` (READ-ONLY, reuse `ListBankToons`) | store-read | request-response | self — `ListBankToons` (D-04 platinum source) | exact (reuse verbatim, no edit) |

---

## Pattern Assignments

### `internal/backendsrv/compute/inventory.go` (service, pure transform — CREATE)

**Analog:** `internal/backendsrv/compute/view.go` (the pure-transform half of the compute-on-read seam)

The new file holds the four pure functions RESEARCH specifies: `StructuredInventory`, `BankValuation`, `TotalPlatinum`, `classifySlot`. All four mirror `view.go`'s purity contract: **no `store` access inside the transform** (the public entry fetches via `store`, the pure helper consumes typed rows). Reuse `pickPrice` VERBATIM — do not re-implement price selection.

**Package-doc + dependency-direction pattern** (`view.go:1-23`, `types.go:7-10`):
```go
// compute imports internal/backendsrv/store (read methods) + internal/backendsrv/enrich
// (constants). store never imports compute. compute authors NO SQL — it consumes the
// typed store-local structs the read methods return.
import (
	"context"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)
```

**Public-entry → pure-helper split to copy** (`view.go:47-82` — `View` fetches, `buildViewRows` is pure & directly unit-testable):
```go
// View computes ... over the store ... Rows returned in the store's order (no re-sort).
func View(ctx context.Context, s *store.Store) ([]ViewRow, error) {
	joinRows, err := s.InventoryJoin(ctx, false)
	if err != nil { return nil, err }
	...
	return buildViewRows(joinRows, links), nil
}

// buildViewRows is the pure transform ... Kept pure (no store access) so it is
// directly unit-testable and so Bank reuses it.
func buildViewRows(joinRows []store.InventoryJoinRow, links map[int64][]store.QuestLinkRow) []ViewRow { ... }
```
Apply: `StructuredInventory(rows []store.InventoryRow) CharacterInventory`, `BankValuation(rows []store.InventoryJoinRow) Valuation`, `TotalPlatinum(banks []store.BankToon) int64` are ALL pure (take the store rows, return the model — no `ctx`/`*store.Store` inside). A thin `StructuredInventory(ctx, s, char)`-style public wrapper that calls `s.InventoryForChar` then the pure helper is the `View`/`buildViewRows` split.

**`pickPrice` — reuse VERBATIM for D-03 valuation** (`view.go:117-138`, do not rewrite):
```go
func pickPrice(prices []PriceDetail) *float64 {
	if p := findDirection(prices, directionWTS); p != nil && p.A30 > 0 { v := p.A30; return &v }
	if p := findDirection(prices, directionWTB); p != nil && p.A30 > 0 { v := p.A30; return &v }
	return nil
}
```
`BankValuation` = for each bank row, build `[]PriceDetail` (the existing `pricesFromJoin`, `view.go:86-95`), `pickPrice(prices)`; if nil → `UnpricedCount++` (D-03 "+N items unpriced", value contributes 0); else `total += *price * float64(row.Count)`. The flat row list IS the valuation scope — do NOT walk the nesting tree to sum (Pitfall 3: children are already their own `inventory_item` rows; summing the flat list counts bag + contents per D-02).

**Direction consts — already TEXT-correct** (`view.go:38-41`): reuse `directionWTS="0"` / `directionWTB="1"`. Never write a fresh `direction == 0` int compare (Pitfall 6 — the column is TEXT).

**`TotalPlatinum` nil-safe sum** — D-04 is literal plat only (gp/sp/cp excluded). `store.BankToon.Plat` is `*int64` (`coin.go:33-40`); skip nil:
```go
func TotalPlatinum(banks []store.BankToon) int64 {
	var sum int64
	for _, b := range banks { if b.Plat != nil { sum += *b.Plat } }
	return sum
}
```

**`classifySlot` (INV-05, the only genuinely-new logic)** — build a Location-native classifier; do NOT retrofit `enrich.WIKI_SLOT_TO_INV_SLOTS` (uppercase, wiki-vocab-keyed, case-mismatched — Pitfall 5). Compare case-insensitively; split the `Location` on the FIRST `-` into `(parent, subSlot)`; a row whose suffix matches `^Slot\d+$` AND whose parent equals another row's top-level Location is a child (nest one level deep only — Pitfall: bags-in-bags don't exist in classic EQ; defensive-flatten a grandchild). `^General\d+` → general, `^Bank\d+` → bank, known equip token → equipment with canonical key = the token. See `slotconst.go` for the canonical set.

---

### `internal/backendsrv/compute/slotconst.go` (config, lookup tables — CREATE, optional / may fold into inventory.go)

**Analog:** `internal/backendsrv/compute/eqconst.go` (3-entry sort map, package-private, doc explains why it isn't re-typed elsewhere) + `internal/backendsrv/enrich/eqconst.go` (the uppercase `WIKI_SLOT_TO_INV_SLOTS` it must NOT reuse).

**Pattern** (`compute/eqconst.go:1-19` — a tiny dependency-free var-map file with a doc-comment that cites its source and explains the seam):
```go
package compute
// eqconst.go holds ONLY the gear-tier sort-rank map ... The slot-pair map
// (WIKI_SLOT_TO_INV_SLOTS) is NOT re-typed here: it already exists ... in
// internal/backendsrv/enrich/eqconst.go ...
var tierRank = map[string]int{ "Velious Pre-Raid/Group": 1, "Velious Raiding": 2, "Iksar": 3 }
```

**The canonical equipment slot set to define** (verified verbatim from `internal/parse/testdata/sample-inventory.txt:20-40` — Title-case Location tokens): `Charm, Head, Face, Neck, Shoulders, Arms, Back, Wrist1, Wrist2, Range, Hands, Primary, Secondary, Finger1, Finger2, Chest, Legs, Feet, Waist, Power, Ammo`. Add `Ear1`/`Ear2` even though the synthetic fixture omits them (A4 — real dumps have 2 ear slots). General = `General1..10`, Bank = `Bank1..8`.

**Landmine to bridge** (`enrich/eqconst.go:65-83`): `WIKI_SLOT_TO_INV_SLOTS` uses UPPERCASE pair forms (`{"Ears":{"EAR1","EAR2"}}`, `{"Fingers":{"FINGER1","FINGER2"}}`) that do NOT match the Title-case inventory `Location` tokens. If a wiki-slot→canonical bridge map is defined here (for Phase 34/WISH-04), use **inventory-Location case** (`Ears→[Ear1,Ear2]`), distinct from the legacy uppercase map. RESEARCH recommends defining the canonical equip set + classifier here and deferring the full wiki→slot bridge to whoever needs it first.

---

### `internal/backendsrv/store/readviews.go` (model / store-read — MODIFY)

**Analog:** self — `InventoryJoin` (`readviews.go:135-213`) and `InventoryByChar` (`readviews.go:350-381`) in the same file. The store-method shape is `func (s *Store) X(ctx) ([]Row, error)`: `s.db.QueryContext` → `rows.Next()`/`Scan` into `sql.Null*` locals → resolve to zero-values on the result struct → `rows.Err()` → `%w`-wrapped errors.

**Change 1 — ADD `InventoryForChar(ctx, char string) ([]InventoryRow, error)`** (all rows for one char, `row_ordinal` order, NO `item_id>0` filter so empty slots + container shells survive — INV-05 needs them). Reuse the EXACT `pp_rep` CTE name-join from `InventoryJoin` and ADD `pp.last_seen` to the projection.

**Reuse the name-join CTE VERBATIM** (`readviews.go:144-159` — this is the 0a169f3 fix; never re-introduce an `item_id` price join — Pitfall 1):
```go
const base = `WITH pp_rep AS (
       SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
       FROM pigparse_price
       WHERE name IS NOT NULL AND trim(name) <> ''
       GROUP BY lower(trim(name))
)
SELECT c.name, ii.location, ii.name, ii.item_id, ii.count,
       im.wiki_url, im.wiki_summary, im.is_quest_item,
       pp.direction, pp.a30, pp.t30,
       pp.last_seen,                  -- DATA-01 last-listed (NEW in projection)
       c.last_seen, ii.row_ordinal
FROM inventory_item ii
JOIN character c            ON c.id = ii.character_id
LEFT JOIN item_master im     ON im.item_id = ii.item_id       -- id-keyed (correct: EQ namespace)
LEFT JOIN pp_rep             ON pp_rep.norm_name = lower(trim(ii.name))
LEFT JOIN pigparse_price pp  ON pp.item_id = pp_rep.rep_item_id -- price via NAME, not id
WHERE c.is_removed = 0 AND c.name = ?`                          // per-char; NO item_id>0 filter
const orderBy = ` ORDER BY ii.row_ordinal`
```
Note the `?` placeholder for `c.name` (the one user-controlled value — bind it, never concat — `[HARD]` SQL rule). `InventoryJoin` has zero `?` params today; this method introduces one — bind via `QueryContext(ctx, query, char)`.

**Nullable-scan pattern to copy verbatim** (`readviews.go:176-208`):
```go
var (
	r InventoryJoinRow
	wikiURL, wikiSummary, direction, lastSeen sql.NullString
	isQuest, t30, itemID, count sql.NullInt64
	a30 sql.NullFloat64
)
if err := rows.Scan(&r.Char, &r.Location, &r.ItemName, &itemID, &count, ...); err != nil {
	return nil, fmt.Errorf("scan inventory join row: %w", err)
}
r.ItemID = itemID.Int64
r.HasPrice = direction.Valid
... // resolve every nullable to its zero-value
```

**Change 2 — ADD `pp.last_seen` to the existing `InventoryJoin` projection** (RESEARCH recommends extending the single join, not forking it) + a `LastListed string` field on `InventoryJoinRow` (`readviews.go:46-61`, append at the right edge — zero-value-safe, existing `View`/`Bank` consumers ignore it). **CRITICAL (Pitfall 2):** `pp.last_seen` (last-listed-for-sale) is DISTINCT from `c.last_seen` (upload freshness, already surfaced as `LastSeen`/`ViewRow.LastSynced`). Scan into a SEPARATE `sql.NullString` → distinct `LastListed` field; never alias them.

**Change 3 — ADD an `InventoryRow` struct** (location/name/item_id/count/slots/row_ordinal + the joined enrichment + price + `LastListed`). Model it on `InventoryJoinRow` (`readviews.go:46-61`) but carry `Slots int64` (the container-capacity column `InventoryJoinRow` omits) since INV-05 nesting needs it.

---

### `internal/backendsrv/store/readviews_test.go` (test — MODIFY)

**Analog:** self — `TestReadViews_InventoryJoinAndGrouping` (`readviews_test.go:105-238`) and the name-bridge regressions `TestReadViews_PriceBridgesByNameAcrossNamespaces` (`:244-291`) + `TestReadViews_PriceNoFanOutOnSharedName` (`:298-334`).

**Seed-helper set to reuse** (package-private, defined in `replace_test.go`/`itemids_test.go`/this file): `NewTestDB(t)`, `seedOwnerChar`, `setCharMeta` (`:16-28`), `seedRaw`, `i64ptr`, `seedItemMaster` (`:30-43`), `seedPigparse` (`:48-57`).

**New coverage to add** (mirroring the existing subtests' structure):
- `InventoryForChar` keeps empty-slot (`item_id=0`) + container-shell + `*-Slot*` child rows (the inverse of the `:130-166` "excludes empty slot" assertion — here they must be PRESENT and `row_ordinal`-ordered).
- `pp.last_seen` projects into `LastListed`, distinct from `c.last_seen`/`LastSeen`. Seed `seedPigparse` writes `last_seen="2026-05-09"` (`fixtures_test.go:92`); assert `LastListed=="2026-05-09"` AND `LastSeen` (char freshness) is the DIFFERENT `"2026-05-09T00:00:00Z"` value from `setCharMeta`.

**Name-bridge assertion to copy** (`readviews_test.go:277-285` — proves catalog-id ≠ inventory-id, joins by name; this is the DATA-01 contract for the new method too):
```go
seedRaw(t, db, charID, "GENERAL1", "10 Dose Ant's Potion", i64ptr(14536), 1) // EQ id
seedPigparse(t, db, 19450, "10 Dose Ant's Potion", "0", 320, 12)              // catalog id differs
// ... assert HasPrice && A30==320 (name-bridged); ItemID stays 14536 (EQ id preserved)
```

---

### `internal/backendsrv/compute/types.go` (model / JSON contract — MODIFY, append-only)

**Analog:** self — `ViewRow` (`types.go:83-96`), `BankView`+`CoinTotals` (`:107-119`), `PriceDetail` (`:64-68`). The package doc (`:13-57`) declares the existing snake_case tags a FIXED cross-plan contract — **APPEND new structs, NEVER rename an existing tag.**

**Snake-case-tag struct pattern to mirror** (`types.go:83-96`):
```go
type ViewRow struct {
	Char        string        `json:"char"`
	Slot        string        `json:"slot"`
	Item        string        `json:"item"`
	ID          int64         `json:"id"`
	Count       int64         `json:"count"`
	Price       *float64      `json:"price"`        // *float64; null when no a30
	LastSynced  string        `json:"last_synced"`
	Prices      []PriceDetail `json:"prices"`
}
```
Add (RESEARCH-recommended): `InventorySlot` (category + canonical-slot + item + count + price + `last_listed` + nested children), `CharacterInventory` (the equipment/general/bank-grouped tree), `BankValuation` / `Valuation` (per-bank + guild totals + `unpriced_count`). Nullable-money fields stay `*float64`/`*int64` so "unpriced/never-entered" ≠ "0" (the `CoinTotals` nil-vs-0 discipline, `types.go:102-106`, `coin.go:33-40`).

**Comment FIX (Pitfall 1 — stale, must correct)** `PriceDetail` doc at `types.go:61-63`:
```go
// CURRENT (STALE — WRONG as of commit 0a169f3):
// Because pigparse_price.item_id is the PRIMARY KEY, the join yields at most ONE
// price row per item, so a ViewRow's Prices slice holds 0 or 1 PriceDetail.
// FIX TO: the one-row guarantee now comes from the pp_rep CTE's GROUP BY norm_name
// + MIN(item_id) fan-out guard (readviews.go), NOT from the item_id PK — the price
// join is by NORMALIZED NAME, not item_id.
```

---

### `internal/backendsrv/compute/view.go` (service — MODIFY, comment fix only)

**Analog:** self — `pricesFromJoin` doc at `view.go:84-85`.

**Comment FIX (Pitfall 1)** — currently claims the wrong rationale (the result is still correct; the explanation rotted post-0a169f3):
```go
// CURRENT (STALE): "The join is one row per item (pigparse_price.item_id is the PK),
//                   so this is 0-or-1 PriceDetail."
// FIX TO: the join yields at most one price row per item because the pp_rep CTE
//         collapses pigparse_price to one representative per normalized name BEFORE
//         the name-keyed LEFT JOIN — NOT because item_id is the PK.
```
No functional change to `view.go` — comment only. (CLAUDE.md's "item_id is the stable join key" is also wrong for the price join — true only for `item_master`, the watcher's EQ namespace.)

---

### `internal/backendsrv/compute/fixtures_test.go` (test helper — MODIFY)

**Analog:** self — `seedInv` (`fixtures_test.go:55-64`, hardcodes `count=1, slots=0`) and `seedPigparse` (`:87-96`, the note that name must match for the price to attach).

**Pattern to extend** (`fixtures_test.go:55-64`):
```go
func seedInv(t *testing.T, db *sql.DB, charID int64, location, name string, itemID, ordinal int64) {
	... INSERT INTO inventory_item (..., count, slots, row_ordinal, ...) VALUES (?,?,?,?, 1, 0, ?, ...)
}
```
Add a `seedInvFull(t, db, charID, location, name, itemID, count, slots, ordinal)` variant that sets `count` + `slots` + a `*-SlotN` location, so nesting (INV-05) + valuation (DATA-02) are testable:
```go
seedInvFull(t, db, charID, "General4", "Large Bag", 1038, 1, 10, ord)        // container shell, slots=10
seedInvFull(t, db, charID, "General4-Slot1", "Diamond", 1071, 5, 0, ord+1)   // nested, count=5
```
**Name-join constraint** (`fixtures_test.go:84-86`): `seedPigparse(name)` MUST equal the inventory item name (case/whitespace tolerant — the join is `lower(trim(name))`) or the price won't attach. Add an unpriced item (no `seedPigparse` row) to exercise the D-03 "+N unpriced" annotation.

---

### `internal/backendsrv/compute/inventory_test.go` (test — CREATE)

**Analog:** `internal/backendsrv/compute/bank_test.go` (the `compute_test` external-package parity-test pattern: `newTestDB(t)` → `store.NewStore(db)` → seed via the shared helpers → call the compute func → assert the shaped result).

**Test-scaffold pattern to copy** (`bank_test.go:15-54`):
```go
package compute_test
func TestStructuredInventory_Classify(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()
	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	seedInvFull(t, db, char, "Head", "Crown of Narandi", 1234, 1, 0, 1)
	seedPigparse(t, db, 9999, "Crown of Narandi", "0", 4500, 75)
	// ... call compute.StructuredInventory(ctx, s, "Slampeach"); assert category/slot/nesting
}
```
**Tests required by RESEARCH's test map** (one per success-criterion behavior): `TestStructuredInventory_Classify`, `_Nesting` (children nest, count preserved), `TestNameJoin_HitMiss` (price + gear-tier NULL-id name-join hit AND miss), `TestLastListed_NotCharFreshness` (Pitfall 2), `TestBankValuation_SumAndUnpriced`, `TestBankValuation_CountsBagContents` (bag + contents both count — Pitfall 3), `TestTotalPlatinum_LiteralPlatOnly` (gp/sp/cp excluded, nil-safe). Prefer loading the real-name fixture (below) for the nesting/sum tests.

---

### `internal/backendsrv/compute/testdata/<Real>-Inventory.txt` (fixture — CREATE, BLOCKING)

**Analog:** `internal/parse/testdata/sample-inventory.txt` (the existing 250-row flat synthetic fixture — verified flat: Title-case Locations, `Slots` is a per-row 0/8/10 capacity number, NO `<Parent>-Slot<N>` child rows). The new fixture is the FORMAT analog but must ADD what the synthetic one lacks: nesting.

**File format to follow** (`sample-inventory.txt:1-20`, tab-separated, header row, `Location\tName\tID\tCount\tSlots`):
```
Location	Name	ID	Count	Slots
General4	Large Bag	1038	1	10        <- container (Slots=10), itself a priced item (D-02)
General4-Slot1	Diamond	1071	5	0        <- NESTED child (the synthetic fixture has NO rows like this)
Bank1	Bag of Holding	1039	1	8
Bank1-Slot1	Words of the Spoken	7001	1	0   <- bag-in-bank nesting (A1/A2 to confirm)
Head	Crown of Narandi	2050	1	0
```

**CLAUDE.md fixture convention** `[HARD]`: real-name file (e.g. `Slampeach-Inventory.txt`) when sourced from a real character; generic (`sample-*`) only when synthetic. This MUST be real-name. It is the **blocking input** for INV-05 nesting tests (RESEARCH Environment Availability) — capture via `/outputfile inventory` on a P99 char with bagged items in both inventory AND bank, or hand-author from the confirmed `<Parent>-Slot<N>` format. Must include: nested bag contents (general + bank), a container shell that is itself priced, ≥1 stacked item (`count>1`), ≥1 unpriced item (no PigParse match), and (for A3) augment rows (`Head-Slot1`) the parser must not crash on.

---

## Shared Patterns

### Name-keyed price join (DATA-01 — the load-bearing cross-cutting pattern)
**Source:** `internal/backendsrv/store/readviews.go:144-159` (the `pp_rep` CTE, commit 0a169f3)
**Apply to:** `InventoryForChar`, the extended `InventoryJoin`, and any gear-tier price lookup (Phase 34 consumer).
Join price by `lower(trim(name))` via the `pp_rep` representative-row CTE — NEVER by `item_id` (PigParse catalog ids ≠ EQ inventory ids; the id-join silently left ~91% of held rows unpriced). For the gear-tier extension: `wiki_gear_tier.item_id` is ALWAYS NULL (`enrich/wikigear.go:30-40`, `store/readviews.go:70-71`), so gear-tier rows can ONLY be priced by `lower(trim(item_name))` → `pp_rep.norm_name` (Pitfall 4 — any reference to `wiki_gear_tier.item_id` in a join is a bug).
```sql
WITH pp_rep AS (
  SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
  FROM pigparse_price WHERE name IS NOT NULL AND trim(name) <> ''
  GROUP BY lower(trim(name))
)
LEFT JOIN pp_rep ON pp_rep.norm_name = lower(trim(ii.name))
LEFT JOIN pigparse_price pp ON pp.item_id = pp_rep.rep_item_id
```

### Price selection (D-03 valuation basis)
**Source:** `internal/backendsrv/compute/view.go:117-138` (`pickPrice`) + `:38-41` (TEXT direction consts)
**Apply to:** `BankValuation`. Reuse VERBATIM — WTS `a30` → WTB `a30` fallback, nil when neither >0. Direction compare is on the STRINGIFIED `"0"`/`"1"` (Pitfall 6 — the SQLite column is TEXT). Per-row value = `*pickPrice × count`; nil → unpriced (count toward "+N items unpriced", contributes 0).

### Store-read seam discipline `[HARD]`
**Source:** `internal/backendsrv/store/readviews.go:12-24`, `itemids.go:10-14`
**Apply to:** every new/extended `store` method. `?` placeholders ONLY (bind `char` — never concat untrusted names); pure SELECT (zero DELETE/INSERT/UPDATE); `slog` silent on happy path, error logs op+err only (never row content — V7); `%w`-wrapped errors; nullable columns → `sql.Null*` → zero-values. `compute` authors ZERO SQL.

### Bank-toon + platinum read (D-04)
**Source:** `internal/backendsrv/store/coin.go:42-69` (`ListBankToons`) + `:33-40` (`BankToon.Plat *int64`)
**Apply to:** `TotalPlatinum`. Reuse `ListBankToons` verbatim — it already scopes `is_bank_toon=1 AND is_removed=0` and returns `*int64` plat (nil-distinguishable from 0). Sum `Plat` only (literal platinum; gp/sp/cp stay separate). No new store read needed for D-04.

### Pure-transform + external test-package parity
**Source:** `compute/view.go:59-82` (pure helper) + `compute/bank_test.go` (`package compute_test`)
**Apply to:** `inventory.go` transforms + `inventory_test.go`. Keep the transform a pure function (store rows in → model out, no `ctx`/store inside) so it is directly table-testable; tests live in `package compute_test` and seed via the shared `fixtures_test.go` helpers over `store.NewTestDB`.

### Append-only JSON contract
**Source:** `compute/types.go:13-57` (the FIXED-contract package doc)
**Apply to:** every `types.go` edit. Append new snake_case-tagged structs; NEVER rename an existing tag (`composeNotes.ts` + read handlers + the Svelte client consume them). Nullable money/price → pointer types (nil ≠ 0).

---

## No Analog Found

None. Every Phase 29 file maps to an in-repo analog — the phase is recombination of shipped seams (`compute/view.go` ↔ `store/readviews.go`, `pickPrice`, the `pp_rep` CTE, `ListBankToons`). The single genuinely-new logic is the `Location`-string `classifySlot` + nesting parser (INV-05), which has no exact analog but mirrors the pure-transform shape of `buildViewRows` and the constant-table shape of `eqconst.go`.

---

## Landmines (carry into every plan)

| # | Landmine | Source | Avoid by |
|---|----------|--------|----------|
| 1 | Stale "item_id is the PK / one row per item" comments | `view.go:84-85`, `types.go:61-63`, CLAUDE.md | Join price by name (`pp_rep`); FIX the two comments; treat the running SQL as truth |
| 2 | `pigparse_price.last_seen` (last-listed) vs `character.last_seen` (upload freshness) | `readviews.go:153`, `:206`; both named `last_seen` | Scan `pp.last_seen` into a SEPARATE `LastListed`; never alias to `LastSynced` |
| 3 | Double/under-counting bag contents | D-02 | Valuation = flat sum over ALL bank rows (children are own rows); NEVER walk the nesting tree to sum |
| 4 | `wiki_gear_tier.item_id` always NULL | `wikigear.go:30-40`, `readviews.go:70-71` | Gear-tier price by `lower(trim(item_name))` only; never by id |
| 5 | Slot-token case mismatch (`HEAD` vs `Head`) | `enrich/eqconst.go:65-83` (UPPERCASE) vs fixture Title-case | Case-insensitive classifier; verify live `SELECT DISTINCT location` (A5) |
| 6 | `direction` is TEXT `"0"`/`"1"`, not int | `view.go:38-41`, `jobs/pigparse.go` | Reuse `pickPrice` / `directionWTS` consts; never `direction == 0` |

**Unverified flags for the planner to resolve on-box (RESEARCH A5/A7):**
- `SELECT DISTINCT location FROM inventory_item` — confirm live-data case (Title vs upper) before finalizing canonical keys.
- `SELECT name, last_seen FROM pigparse_price LIMIT 5` — confirm `last_seen` is last-listed-for-sale (post-WTS-filter).

---

## Metadata

**Analog search scope:** `internal/backendsrv/compute/`, `internal/backendsrv/store/`, `internal/backendsrv/enrich/`, `internal/parse/`
**Files read this session:** `compute/view.go`, `compute/types.go`, `compute/eqconst.go`, `compute/fixtures_test.go`, `compute/bank_test.go`, `store/readviews.go`, `store/readviews_test.go`, `store/coin.go`, `store/itemids.go`, `enrich/eqconst.go`, `enrich/wikigear.go`, `parse/inventory.go`, `parse/inventory_test.go`, `parse/testdata/sample-inventory.txt`
**Pattern extraction date:** 2026-06-17
**Schema footprint:** ZERO migrations (compute-on-read; everything needed already stored). NO new `readapi` handler (deferred to Phases 31-34).
