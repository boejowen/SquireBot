# Phase 32: Inventory Tab (Item-Centric) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-18
**Phase:** 32-inventory-tab-item-centric
**Areas discussed:** List scope & item identity, Default list ordering, Detail panel content, Row headline & price

---

## List scope & item identity (ITEM-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Everything, grouped by name | One row per normalized item name; every copy anywhere (equipped + general + bags + banks, all chars/banks/bots); nothing hidden | ✓ |
| Exclude equipped gear | List only loose/bagged/bank items (the "what's available" view) | |
| Hide zero-value junk | Everything, but suppress rows with no PigParse price AND no wiki entry | |

**User's choice:** Everything, grouped by name (D-01)
**Notes:** Identity = normalized name, not item_id (inventory ids ≠ PigParse/gear catalog ids; gear-tier rows have no id) — consistent with the Phase 29 DATA-01 name-keyed join.

---

## Default list ordering (ITEM-02)

| Option | Description | Selected |
|--------|-------------|----------|
| Viewer's items first, then A-Z | Your-character holdings float up, then alphabetical; matches P31 list + ITEM-02 search priority | ✓ |
| A-Z by name | Plain alphabetical, ownership-neutral at rest | |
| Most-held first | Guild-wide quantity descending (commodity-forward) | |

**User's choice:** Viewer's items first, then A-Z (D-02)
**Notes:** Makes the resting order consistent with the search-time viewer priority.

---

## Detail panel content (ITEM-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Examine + holders, with holder deep-links | Reuse P31 ExaminePanel + a holders table (char · slot · qty · last-synced); holder rows deep-link to /characters?c= | ✓ |
| Holders table only | Just char · slot · qty · last-synced + a wiki/PigParse link | |
| Examine + holders, no deep-link | Full examine + holders table, holder rows not clickable | |

**User's choice:** Examine + holders, with holder deep-links (D-03)
**Notes:** Single pinned panel, replace-on-click (D-03a), mirroring P31 D-06/D-07. The cross-tab jump into the P31 window is the high-value interaction.

---

## Row headline & price (ITEM-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Qty + holders + inline price & wiki | Summed stack count + holder count ("142 · 3 holders") + PigParse price + wiki link, inline on the row | ✓ |
| Qty + holders only; price/wiki in detail | Lean rows; price/wiki only in the detail panel | |
| Summed count only (no holder count) | Just the guild-wide total, drop the holder count | |

**User's choice:** Qty + holders + inline price & wiki (D-04)
**Notes:** Matches the sketch-003 headline; summed count = total individual items, holder count = distinct holding characters.

---

## Claude's Discretion

- The item-rollup backend shape (new compute func + new session-gated read route over `compute.View`/`StructuredInventory`; do NOT reuse the catalog `GET /api/v1/items/search`).
- Slot-label granularity + within-holder grouping in the holders table.
- Holders-table ordering (viewer-first expected).
- Whether the list reuses `DataGrid.svelte` or a purpose-built master-detail list; whether the detail reuses `ExaminePanel.svelte` directly.
- Quantity edge cases (coin/plat excluded; sum bag + loose copies of the same name).
- Layout/mobile reflow/row density/icon sizing → UI-SPEC.

## Deferred Ideas

- Banks tab + valuation/platinum (Phase 33); per-character/per-slot Wishlist (Phase 34).
- Sort/filter controls beyond viewer-first/A-Z + name search — revisit only if the default proves insufficient.
- Multi-pin compare board — rejected for the single replace-on-click panel.
