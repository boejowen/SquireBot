---
phase: 29-data-foundation-inventory-parse-price-value-aggregation
fixed_at: 2026-06-17T23:30:00Z
review_path: .planning/phases/29-data-foundation-inventory-parse-price-value-aggregation/29-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 5
deferred: 3
status: all_blocking_fixed
gate:
  go_build: pass
  go_vet: pass
  go_test_backend: pass
  go_test_all: pass
commits:
  - bfb76c5  # CR-01 + WR-01 + MR-01 + MR-02 (inventory.go + inventory_test.go)
  - 5dcd154  # NIT-02 (readviews.go)
---

# Phase 29: Code Review Fix Report

**Fixed at:** 2026-06-17T23:30:00Z
**Source review:** `29-REVIEW.md`
**Iteration:** 1
**Scope:** backend only (`internal/backendsrv/{compute,store}`) — no watcher edits, no goose
migration (head stays `00011`), `compute` stays SQL-free, name-keyed `pp_rep` join preserved,
`pickPrice` / `ListBankToons` reused verbatim.

**Summary:**
- Findings in scope: 8 (1 BLOCKER, 1 HIGH, 2 MEDIUM, 2 LOW, 2 NIT)
- Fixed: 5 (CR-01, WR-01, MR-01, MR-02, NIT-02)
- Deferred (non-blocking, documented in `29-REVIEW.md`): 3 (LR-01, LR-02, NIT-01)
- All BLOCKER + HIGH cleared → `29-REVIEW.md status: clean`

**Gate (all pass):** `go build ./...`, `go vet ./internal/backendsrv/...`,
`go test ./internal/backendsrv/...`, `go test ./...`.

---

## Fixed Issues

### CR-01 (BLOCKER): dangling parent pointers drop nested children

**File:** `internal/backendsrv/compute/inventory.go` (`buildStructuredInventory`)
**Commit:** `bfb76c5`

**Root cause:** `parentRef` held `*InventorySlot` pointers INTO the `inv.General` / `inv.Bank`
backing arrays. Pass B's orphan/grandchild branch did `inv.General = append(...)` /
`inv.Bank = append(...)` on those same slices. When that append grew past capacity it
reallocated the backing array, so every `parentRef` pointer for the group went stale; a
later `parent.Children = append(...)` wrote into the OLD (orphaned) array, and the returned
group (the NEW array) had `Children == nil`. Net effect: any real container whose children
appeared after an orphan row (in `row_ordinal` order) silently lost all nested children.

**Fix:** Stop retaining pointers into a slice that is later appended to. Nesting is now
resolved with a side map `childrenByParent map[string][]InventorySlot` keyed by parent
Location (a stable string, immune to slice reallocation). Orphans/grandchildren are
collected into separate `orphanGeneral` / `orphanBank` slices and appended to the groups
ONLY at the very end — after every `Children` attachment is done — so no append can dangle
a parent still being mutated. Children are attached by indexing the (now-final) group slices
by Location and writing `Children` straight onto the element.

**Regression tests (failing-first, in `inventory_test.go`):**
- `TestStructuredInventory_OrphanBeforeContainer` — orphan `General9-Slot1` at ordinal 1,
  exactly 4 real top-level General containers (so Pass A leaves `inv.General` at
  `len==cap==4`, guaranteeing the orphan append reallocates), then `General1-Slot1` at
  ordinal 6. Asserted `General1.Children == 1 (Diamond)`. Reproduced the reviewer's
  "children = 0" before the fix; passes after.
- `TestStructuredInventory_OrphanBeforeContainer_Bank` — same hazard on the `inv.Bank` group.

> The `len==cap` seeding is deliberate: with a slack-capacity seed the single orphan append
> did not realloc and the bug stayed latent (confirming the existing green suite never
> exercised it). Pinning the container count to a power-of-two-aligned value (4) forces the
> realloc deterministically.

---

### WR-01 (HIGH): sub-slot regex was case-sensitive

**File:** `internal/backendsrv/compute/inventory.go:30` (`subSlotRe`)
**Commit:** `bfb76c5`

**Root cause:** `subSlotRe = ^Slot\d+$` was case-sensitive while `generalRe`/`bankRe` carried
`(?i)` and `equipmentSlotsLC` compared lower-cased. On uppercase live data,
`classifySlot("GENERAL4-SLOT1")` matched the parent case-insensitively (→ canonical
`General4`) but `splitChild("GENERAL4-SLOT1")` returned `("", false)` because `"SLOT1"`
failed `^Slot\d+$`. The child was treated as top-level AND classified to `CanonicalSlot
"General4"`, surfacing as a phantom second top-level slot colliding with the real
container — defeating the phase's own A5 case-robustness goal.

**Fix:** `var subSlotRe = regexp.MustCompile(`(?i)^slot\d+$`)` — case-insensitive, consistent
with the container regexes and `equipmentSlotsLC`.

**Regression test:** `TestStructuredInventory_UppercaseNesting` feeds `GENERAL4`,
`GENERAL4-SLOT1`, `BANK1`, `BANK1-SLOT1` and asserts exactly one top-level slot per group,
correct nesting, and Title-case canonical output (`General4` / `Bank1`). Failed before
("General group = 2 top-level slots"), passes after.

---

### MR-01 (MEDIUM): display/valuation scope asymmetry undocumented

**File:** `internal/backendsrv/compute/inventory.go` (`buildStructuredInventory`,
`buildBankValuation`)
**Commit:** `bfb76c5`

**Assessment:** Not a behavioral defect — `BankValuationFor` already values the FLAT
`InventoryJoin(bankOnly)` row list (every real `inventory_item` row incl. augments), which is
correct per D-02; the structured model is a display projection that drops augments / re-homes
orphans. The risk was a future maintainer wiring a per-character bank value off the display
tree and silently excluding augment value.

**Fix:** Documentation-only — added a note on both functions stating the structured INV-05
model is display-only and that valuation must use the flat join list (making the augment case
of Pitfall 3 explicit). No behavior change, so no separate failing-first test; the existing
`TestBankValuation_CountsBagContents` already pins the flat-sum (bag + contents) behavior.

---

### MR-02 (MEDIUM): `PerBank` omitted coin-only bank toons

**File:** `internal/backendsrv/compute/inventory.go` (`buildBankValuation`)
**Commit:** `bfb76c5`

**Root cause:** `PerBank` was keyed only by `Char` names appearing in the inventory join
rows. A bank toon with platinum entered but no `inventory_item` rows (freshly flagged,
mid-upload, or emptied) contributed to `TotalPlatinum` (via `ListBankToons`) but got no
`PerBank` entry — so a consumer iterating `PerBank` to render per-bank lines dropped that
toon's platinum line even though the guild total counted it.

**Fix:** Seed `PerBank` from the bank-toon list FIRST
(`make(map[string]Valuation, len(toons))` then a zero `Valuation{}` per `t.Name`), so every
live bank toon has a row. `ListBankToons` is reused verbatim; the platinum scope and the
per-bank scope now agree on the bank-toon set.

**Regression test:** `TestBankValuation_CoinOnlyBankToon` — one bank toon with a priced item
plus a coin-only bank toon (`plat=777`, no items). Asserts the coin-only toon has a
zero-`Valuation` `PerBank` entry, its plat is in `TotalPlatinum`, and the item toon is
unaffected. Failed before ("PerBank missing the coin-only bank toon"), passes after.

---

### NIT-02: stale `pp_by_name` comment

**File:** `internal/backendsrv/store/readviews.go:184`
**Commit:** `5dcd154`

**Root cause:** The `InventoryJoin` CTE comment named the bridge `pp_by_name` while the SQL
aliases it `pp_rep` (matching `InventoryForChar` / `GearTierPrices`).

**Fix:** Comment corrected to `pp_rep`. No SQL or behavior change; the name-keyed `pp_rep`
join is preserved verbatim (no item_id price join reintroduced). No test (comment-only).

---

## Deferred (non-blocking) — recorded in `29-REVIEW.md` for user triage

- **LR-01** (LOW) — `float64` running sum in `Valuation.TotalValue`. Review itself said "no
  code change required; documentation only" — order is deterministic via the store
  `ORDER BY`, display rounds, and it matches the existing `pickPrice` float64 contract. Not
  a correctness bug; left for the user to decide if an exact-currency representation is ever
  required.
- **LR-02** (LOW) — `canonicalNumbered` slice guarded by caller discipline + comment. The
  only two callers (the `generalRe`/`bankRe` match arms) both guarantee `prefix+digits`, so
  it cannot panic today. Speculative hardening deferred; add the `if len(parent) <
  len(prefix)` guard (or a capture-group rewrite) only when a third caller appears.
- **NIT-01** — `findSlot` test helper iterates value copies of the group slice headers;
  flagged harmless/clarity-only by the reviewer. Left as-is.

---

_Fixed: 2026-06-17T23:30:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
