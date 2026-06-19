# Phase 29: Data Foundation — Inventory Parse + Price/Value Aggregation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-17
**Phase:** 29-data-foundation-inventory-parse-price-value-aggregation
**Areas discussed:** Container contents counting, Bank valuation basis, Total platinum definition
**Areas locked to default (not separately discussed):** Slot-model shape

---

## Gray-area selection

Presented 4 candidate areas; user selected 3 to discuss. The 4th (Slot-model shape)
was locked to the recommended default and noted in CONTEXT.md (D-01).

| Area | Description | Discussed |
|------|-------------|-----------|
| Slot-model shape | Structured classified+nested model vs raw `Location` pass-through | locked to default |
| Container contents counting | Whether bag contents count in rollups + valuation | ✓ |
| Bank valuation basis | Which price = value, stacks, unpriced handling | ✓ |
| Total platinum definition | Literal `plat` vs plat-equivalent of all coin | ✓ |

---

## Container contents counting

| Option | Description | Selected |
|--------|-------------|----------|
| Count everywhere | Bag contents nest for drill-down AND count toward guild-wide quantity AND bank valuation; the bag itself is a priced item too | ✓ |
| Drill-down only, exclude from rollups | Contents show in drill-down but don't count toward quantity/valuation; only top-level slots tallied | |

**User's choice:** Count everywhere (Recommended)
**Notes:** Parse the EQ `<ParentSlot>-Slot<N>` nesting format. A gem in a bag in the bank is "in the guild" and adds to bank value. → CONTEXT D-02.

---

## Bank valuation basis

| Option | Description | Selected |
|--------|-------------|----------|
| pickPrice × count, flag unpriced | Value = existing pickPrice (WTS a30 → WTB a30) × stack count, summed; unpriced items = 0 but total annotated "+N items unpriced" | ✓ |
| pickPrice × count, no flag | Same math, no unpriced annotation (quietly omits unpriced items) | |
| WTS ask only | Only items with a live WTS ask count (ignore WTB bids) | |

**User's choice:** pickPrice × count, flag unpriced (Recommended)
**Notes:** Reuse `compute/view.go` `pickPrice` verbatim; never silently understate the total. → CONTEXT D-03.

---

## Total platinum definition

| Option | Description | Selected |
|--------|-------------|----------|
| Plat column only | Sum just the `plat` field across bank toons (literal platinum); gp/sp/cp shown separately | ✓ |
| Plat-equivalent of all coin | Convert gold/silver/copper into plat and add to plat — total coin wealth in plat | |

**User's choice:** Plat column only (Recommended)
**Notes:** Matches the DATA-02 wording "total platinum" and the existing `character` coin columns. Sum over live bank toons (`is_bank_toon=1 AND is_removed=0`). → CONTEXT D-04.

---

## Claude's Discretion

- Compute-on-read vs materialized (default: compute-on-read, per the existing `compute/` pattern).
- Schema extension shape (extend-only via `goose` if needed, or pure compute-on-read).
- Name normalization key (`lower(trim(name))`, per the existing spellbook/wiki convention).
- last-listed-for-sale source (`pigparse_price.last_seen`, already stored).

## Research-flagged (not a user decision)

- Reconcile the PigParse catalog item_id vs in-game EQ item_id namespace question
  (project memory `pigparse-vs-ingame-item-id-namespaces`) against the existing
  `item_id` inventory price join — verify correctness, prefer normalized-name join,
  don't break the live `view` price display.

## Deferred Ideas

- Item-icon → iconId mapping → Phase 31 (INV-04).
- Guild-wide item list + holder drill-down UI → Phase 32 (ITEM-01..03).
- Per-slot wishlist suggestion engine → Phase 34 (WISH-04).
