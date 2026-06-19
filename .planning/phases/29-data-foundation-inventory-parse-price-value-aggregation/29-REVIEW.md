---
phase: 29-data-foundation-inventory-parse-price-value-aggregation
reviewed: 2026-06-17T23:10:00Z
depth: deep
files_reviewed: 8
files_reviewed_list:
  - internal/backendsrv/compute/inventory.go
  - internal/backendsrv/compute/slotconst.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/compute/view.go
  - internal/backendsrv/compute/inventory_test.go
  - internal/backendsrv/compute/slotconst_test.go
  - internal/backendsrv/compute/fixtures_test.go
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/readviews_test.go
findings:
  blocker: 1
  high: 1
  medium: 2
  low: 2
  nit: 2
  total: 8
status: clean
fix_status:
  fixed: [CR-01, WR-01, MR-01, MR-02, NIT-02]
  deferred: [LR-01, LR-02, NIT-01]
fix_report: 29-REVIEW-FIX.md
fixed_at: 2026-06-17T23:30:00Z
---

# Phase 29: Code Review Report

**Reviewed:** 2026-06-17T23:10:00Z
**Depth:** deep (cross-file: compute ↔ store seam, SQL discipline, pointer-aliasing trace)
**Files Reviewed:** 8 source + 1 fixture
**Status:** issues_found

## Summary

Phase 29 is well-built recombination of the existing `compute` ↔ `store` seam: the name-keyed PigParse price join is honored everywhere (no `item_id` price join reintroduced), all dynamic SQL values are `?`-bound (no name-concatenation, no injection surface), the two stale "item_id is the PK" comments were correctly fixed, and the DATA-01/DATA-02 contracts (last-listed vs char-freshness separation, flat-sum valuation counting bag + contents, literal-plat-only nil-safe total) are faithfully implemented and tested. The store layer is clean.

The defects are concentrated in the one genuinely-new piece of logic — `buildStructuredInventory`'s nesting tree (`compute/inventory.go`):

1. **A BLOCKER pointer-aliasing bug that silently drops a real container's nested children** whenever an orphan/grandchild row precedes that container's children in `row_ordinal` order. The orphan-handling `append` reallocates the same group slice that `parentRef` holds pointers into, dangling those pointers. Reproduced empirically (see CR-01).
2. **A HIGH case-sensitivity inconsistency** that defeats the phase's own stated A5 case-robustness goal: the sub-slot regex is case-sensitive while the container regexes are case-insensitive, so uppercase live data mis-nests children AND produces canonical-slot collisions. Reproduced empirically (see WR-01). The live `inventory_item.location` case was flagged UNVERIFIED in RESEARCH (Open Question 1) and was never resolved before coding.

Neither defect is exercised by the current test fixture (which is Title-case and contains no orphan-before-container ordering), and neither function has a production caller yet — so the existing 165-ish-line green suite passes while both bugs sit latent, exactly the "green ≠ correct" trap the project has hit before (memory `web-tests-node-only-blind-to-dom`). They will corrupt the INV-05 model the moment Phases 31–33 wire `StructuredInventory` to an endpoint.

`status: issues_found` (one BLOCKER + one HIGH).

## Blocker Issues

### CR-01: `buildStructuredInventory` drops a container's children when an orphan row precedes them (dangling pointer after slice realloc)

**File:** `internal/backendsrv/compute/inventory.go:148-181`

**Issue:** `parentRef` is a `map[string]*InventorySlot` whose values are pointers **into the backing arrays** of `inv.Equipment` / `inv.General` / `inv.Bank` (captured at lines 148-153, correctly after Pass A so Pass A's own appends don't dangle them). The comment at 144-147 reasons that Pass B only appends to `parent.Children` (a per-slot slice) and so the group backing arrays stay stable. **But the orphan/grandchild branch at lines 172-177 violates exactly that invariant** — it does `inv.General = append(inv.General, it.slot)` / `inv.Bank = append(inv.Bank, it.slot)`, appending to the very group slice `parentRef` points into. When that `append` grows past capacity it reallocates the backing array, so:

- every `parentRef` pointer for that group now points into the **old, orphaned** array;
- a later `parent.Children = append(parent.Children, it.slot)` (line 180) for a parent in that group writes `Children` into the **stale** array;
- the returned `inv.General` / `inv.Bank` is the **new** array, where that parent has `Children == nil`.

Net effect: **any legitimate container whose children appear after an orphan/grandchild row (in `row_ordinal` order) silently loses all its nested children** — data-loss in the INV-05 model with no error. Orphan rows are not hypothetical: the code path is the documented A2 "grandchild / bags-in-bags / orphaned `-Slot`" defensive case, and any future EQ dump quirk, augment-on-a-missing-parent, or partial upload can produce one.

Reproduced empirically (orphan `General9-Slot1` at ordinal 2, real container `General1` + child `General1-Slot1` at ordinals 1 and 3):
```
General1 children = 0, want 1 (Diamond)   ← child dropped; written to the stale backing array
```

**Fix:** Don't mutate the group slices in Pass B. Resolve nesting against a non-aliasing structure. Simplest: build `parentRef` over an index map instead of element pointers, and append children to a side map keyed by parent Location, stitching them in at the end — or collect orphans into a separate slice and append them to the groups only AFTER all `Children` writes are done. Minimal patch that preserves the current shape:

```go
// Collect orphans separately; do NOT append to inv.General/inv.Bank during Pass B.
var orphanGeneral, orphanBank []InventorySlot
// ... in the orphan branch:
switch it.category {
case SlotBank:
    orphanBank = append(orphanBank, it.slot)
default:
    orphanGeneral = append(orphanGeneral, it.slot)
}
// ... after Pass B completes (all parent.Children writes done):
inv.General = append(inv.General, orphanGeneral...)
inv.Bank = append(inv.Bank, orphanBank...)
```
(Note: even this final append re-aliases `parentRef`, but it happens *after* the last `Children` write, so it is safe. Add a regression test seeding an orphan row before a real container's child — the suite has no such case today.)

## High Issues

### WR-01: Sub-slot regex is case-sensitive while container regexes are case-insensitive — uppercase live data mis-nests children and collides canonical slots (defeats the A5 goal)

**File:** `internal/backendsrv/compute/inventory.go:30, 34-37` (consumed at `splitChild`, line 221-231; vs `classifySlot`, 49-69)

**Issue:** `generalRe`/`bankRe` carry `(?i)` and `equipmentSlotsLC` compares lower-cased, explicitly so the classifier is robust to whatever case live `inventory_item.location` uses (the A5 / Pitfall-5 landmine the phase set out to defuse). But `subSlotRe = ^Slot\d+$` (line 30) is **case-sensitive**, and `splitChild` (the nesting predicate) uses it directly. So for uppercase live data:

- `classifySlot("GENERAL4-SLOT1")` → `(general, "General4")` (parent matched case-insensitively), but
- `splitChild("GENERAL4-SLOT1")` → `("", false)` because `"SLOT1"` fails `^Slot\d+$`.

The child is therefore treated as a **top-level** slot, AND it classifies to `CanonicalSlot "General4"` — i.e. it surfaces as a **second top-level General slot colliding with the real container's canonical key**, instead of nesting. Reproduced empirically:
```
General group has 2 top-level entries (GENERAL4 + GENERAL4-SLOT1, both CanonicalSlot "General4"), want 1 with a nested child
```

This is the same case-mismatch class as the legacy `WIKI_SLOT_TO_INV_SLOTS` bug `slotconst.go` was written to avoid — half-fixed. RESEARCH Open Question 1 / A5 flagged the live-data case as UNVERIFIED and recommended verifying `SELECT DISTINCT location FROM inventory_item` before finalizing; the resume notes show that on-box check was never run. If live data is uppercase (or mixed), every bag's contents render as loose top-level slots.

**Fix:** Make the sub-slot match case-insensitive to match the container regexes:
```go
var subSlotRe = regexp.MustCompile(`(?i)^slot\d+$`)
```
And/or actually run the on-box `SELECT DISTINCT location` check (RESEARCH Open Question 1) and pin the live case in the test fixture, so the A5 robustness is proven rather than assumed.

## Medium Issues

### MR-01: Display/valuation asymmetry for equipment augments and orphans is undocumented and divergent

**File:** `internal/backendsrv/compute/inventory.go:163-164` (display) vs `241-251`/`258-275` (valuation)

**Issue:** `buildStructuredInventory` deliberately drops equipment augments (`Head-Slot1`, line 163-164) and re-homes orphans, i.e. the structured model is a *display* projection. `BankValuationFor` values the **flat** `InventoryJoin(bankOnly)` row list, which includes those same augment rows (they are real `inventory_item` rows with `item_id>0`). That is arguably correct per each function's spec (D-02 says every held item counts toward valuation; the paperdoll hides augments), but the two functions now disagree about what "the inventory" is, with no comment tying them together. A future maintainer wiring a per-character bank-window value off `StructuredInventory` (instead of the flat list) would silently exclude augment value. Add a one-line note on each function that the structured model is display-only and valuation must use the flat join list (Pitfall 3 already warns the inverse; make the augment case explicit too).

**Fix:** Comment both functions to state the augment/orphan display-vs-valuation split, or surface augments in the structured model under their equipment parent so the two views agree.

### MR-02: `BankValuationFor` valuation scope can diverge from the platinum scope (bank toon with zero priced rows vs zero rows)

**File:** `internal/backendsrv/compute/inventory.go:258-275`

**Issue:** `PerBank` is keyed only by `Char` names that appear in the **inventory** join rows. A live bank toon with coin entered but no `inventory_item` rows (freshly flagged, not yet uploaded, or removed inventory) contributes to `TotalPlatinum` (via `ListBankToons`) but gets **no `PerBank` entry at all** — so a consumer iterating `PerBank` to render per-bank rows will omit that toon's platinum line entirely, even though `TotalPlatinum` includes it. The guild total stays correct; the per-bank breakdown silently drops the toon. Whether that matters depends on the Phase-33 render, but the mismatch is a latent surprise. Consider seeding `PerBank` from `toons` first (zero `Valuation` for each live bank toon) so every bank toon has a row.

**Fix:**
```go
bv := BankValuation{PerBank: make(map[string]Valuation, len(toons))}
for _, t := range toons {
    bv.PerBank[t.Name] = Valuation{} // ensure every live bank toon has an entry
}
```

## Low Issues

### LR-01: `Valuation.TotalValue` accumulates `float64` prices — order-dependent rounding across a large bank

**File:** `internal/backendsrv/compute/inventory.go:268-269`

**Issue:** Valuation is a running `float64` sum of `*price * float64(count)`. PigParse `a30` averages are non-integer, so a guild-bank sum over hundreds of rows is subject to float accumulation error and is order-dependent (the store `ORDER BY` fixes order, so it is at least deterministic). For a "total guild bank worth ~X platinum" headline this is almost certainly fine (display rounds anyway), and it matches the existing `pickPrice` `float64` contract, so it is not a correctness bug — but it is worth a one-line acknowledgement that the figure is an approximation, so nobody later treats it as exact currency. No code change required; documentation only.

### LR-02: `canonicalNumbered` relies on a load-bearing invariant with only a comment guarding it

**File:** `internal/backendsrv/compute/inventory.go:74-77`

**Issue:** `canonicalNumbered` slices `parent[len(prefix):]` and the comment asserts "len(parent) >= len(prefix); the regex guaranteed prefix+digits." That is true *today* because the only callers are the two `generalRe`/`bankRe` match arms. But the guarantee lives entirely in caller discipline + a comment — if a future arm calls `canonicalNumbered` with a non-matching token, this panics with a slice-out-of-range. Cheap hardening: `if len(parent) < len(prefix) { return parent }` before the slice, or fold the digit extraction into the same regex via a capture group so the function can't be misused.

## Nit Issues

### NIT-01: `findSlot` test helper iterates value copies of the group slices

**File:** `internal/backendsrv/compute/inventory_test.go:20-29`

**Issue:** `findSlot` ranges over `[][]compute.InventorySlot{inv.Equipment, inv.General, inv.Bank}` and returns `&group[i]`. Because the outer literal copies the three slice headers (not the elements), `&group[i]` points into the original backing arrays — so it works correctly for the read-only assertions here. It is fine as written, but the returned pointer's lifetime is subtle enough to be worth a comment, and a reader could mistake it for returning a pointer into a local copy. Harmless; flagging for clarity only.

### NIT-02: Two near-identical `pp_rep` CTE + nullable-scan blocks across three read methods

**File:** `internal/backendsrv/store/readviews.go:192-207` (`InventoryJoin`), `280-296` (`InventoryForChar`), `420-431` (`GearTierPrices`)

**Issue:** The `WITH pp_rep AS (...) ... LEFT JOIN pp_rep ... LEFT JOIN pigparse_price` bridge is copy-pasted three times, as is the price-detail nullable-scan boilerplate (`direction/a30/t30/lastListed` → struct). The duplication is *deliberate* per the package doc (single tested SQL path, each method self-contained, store authors all SQL) and each copy is independently tested, so this is not a defect — but if a fourth name-join read lands, factor the CTE into a `const ppRepCTE` string to prevent drift (one copy already differs in alias naming: the comment at line 184 calls it `pp_by_name` while the SQL aliases it `pp_rep`). Fix that stale alias name in the comment at minimum.

---

## Fix Status (2026-06-17T23:30:00Z)

`status: clean` — no BLOCKER or HIGH remains. See `29-REVIEW-FIX.md` for per-finding
root cause → fix → regression test → commit detail.

**Fixed (5):**
- **CR-01** (BLOCKER) — `buildStructuredInventory` no longer retains pointers into the
  group slices; nesting resolves via a side map keyed by parent Location and orphans are
  appended last. Regression tests `TestStructuredInventory_OrphanBeforeContainer{,_Bank}`
  reproduce the dropped-child (count 0) bug and now pass. Commit `bfb76c5`.
- **WR-01** (HIGH) — `subSlotRe` is now `(?i)^slot\d+$`. Regression test
  `TestStructuredInventory_UppercaseNesting` (UPPERCASE locations nest + emit Title-case).
  Commit `bfb76c5`.
- **MR-01** (MEDIUM) — documented the display-vs-valuation scope split on both
  `buildStructuredInventory` and `buildBankValuation` (comment-only; behavior unchanged —
  valuation already correctly uses the flat join list). Commit `bfb76c5`.
- **MR-02** (MEDIUM) — `PerBank` is seeded from the bank-toon list first, so coin-only
  bank toons get a zero-Valuation row. Regression test `TestBankValuation_CoinOnlyBankToon`.
  Commit `bfb76c5`.
- **NIT-02** — corrected the stale `pp_by_name` comment to `pp_rep` in `readviews.go`.
  Commit `5dcd154`.

### Deferred (non-blocking) — user to triage

These were reviewed and intentionally NOT fixed; none is a correctness defect:

- **LR-01** (LOW) — `Valuation.TotalValue` is a running `float64` sum. Documentation-only
  per the review (the order is deterministic via the store `ORDER BY`; display rounds; it
  matches the existing `pickPrice` float64 contract). No code change warranted now; revisit
  only if an exact-currency requirement appears.
- **LR-02** (LOW) — `canonicalNumbered` slices `parent[len(prefix):]` guarded by caller
  discipline + a comment. Cheap hardening was suggested but the only two callers are the
  `generalRe`/`bankRe` match arms (both guarantee the prefix), so it cannot be misused
  today. Deferred to avoid speculative change; harden if a third caller is added.
- **NIT-01** — `findSlot` test helper iterates value copies of the group slice headers
  (`&group[i]` still points into the original backing arrays). The reviewer flagged it as
  harmless/clarity-only. Left as-is.

_Fixed by: Claude (gsd-code-fixer) — backend only; no watcher change, no goose migration
(head stays 00011), `compute` stays SQL-free, name-keyed `pp_rep` join preserved._

---

_Reviewed: 2026-06-17T23:10:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
