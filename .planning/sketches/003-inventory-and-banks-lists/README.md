---
sketch: 003
name: inventory-and-banks-lists
question: "How should the item-centric Inventory list (expandable holders, wiki/PigParse links, slot + last-synced) and the Banks valuation tab present?"
winner: "B"
tags: [inventory, banks, list, expand, totals]
decisions:
  - "Inventory list = Variant B (master-detail) chosen 2026-06-17 — same click-to-pin model as the character inventory window (002), so Characters + Inventory feel consistent."
  - "Banks tab approved as-is: two headline totals (bank item value + platinum on hand), per-bank value/plat rows that open the bank's inventory window, and a bank-item search."
---

# Sketch 003: Inventory & Banks tabs

## Design Question
Two tabs from the spec:
- **Inventory** ("which characters have item X?") — a guild-wide item list: each item shows its name, guild-wide count, and an expandable list of the characters holding it (with **slot** and **last-synced**), plus a wiki link and a PigParse price that links to PigParse.
- **Banks** ("what's in the guild banks?") — a banks-only list + **total item value** (PigParse) + **total platinum**, with a search across bank items.

## How to View
open .planning/sketches/003-inventory-and-banks-lists/index.html

Switch variants up top. In A/C, click an item to expand its holders. In B, click an item to load the detail panel. Type in any search box. The **▸ Banks tab** button shows that surface (try searching it — e.g. "velium").

## Variants (Inventory list)
- **A: Accordion (spec-literal)** — each item is a row that expands inline to its holder list (character · slot · qty · last-synced). Closest to "an expandable list of the guild characters who have that item."
- **B: Master-detail** — item list on the left; clicking shows a right-hand detail panel with a full holders table. **Consistent with the character inventory window (002's click-to-pin)** — same mental model across tabs.
- **C: Dense table** — sortable columns (Item · Guild-wide · PigParse · Wiki) with expandable holder sub-rows. Most data-dense; closest to today's DataGrid.

## Banks tab (one design)
Two big stat cards (**Bank item value**, **Platinum on hand**), a bank-item search, and the banks list (Findom / Mulebot) — each bank opens its inventory window (002's layout). Searching filters to matching bank items with their bank holders.

## What to Look For
- A's inline-expand vs B's side-detail vs C's table — which fits "which characters have this item?" best, and should it match 002's pinned-detail (→ B) for consistency?
- Is guild-wide count + holder count (e.g. "142 guild-wide · 3 holders") the right headline per item?
- Banks: are the two totals (item value + platinum) the right top-line numbers? Should each bank row show its own value/plat (it does) — and link into 002's window?
- Search-prioritizes-your-items: visible enough? (yours sort to top + a "you" tag.)

## Notes (for planning)
- Item value/platinum totals are new backend aggregation (sum PigParse value across bank holdings; platinum from the manual bank-coin entries — the inventory file has no coin).
- "Holder · slot · last-synced" needs the same `Location`→slot parse as 002.
