---
sketch: 002
name: character-inventory-window
question: "How faithfully should the per-character view render the in-game inventory window (slot layout, stacks, backpack drill-down, bank, right-click examine)?"
winner: "D"
tags: [inventory, drill-down, tooltip, characters]
decisions:
  - "Chosen 2026-06-17 = Variant D synthesis: A's faithful EQ paperdoll + C's click-to-pin detail panel (touch-friendly, honours the spec's 'hover OR tap')."
  - "Inventory/Bank toggle REMOVED — bank items are listed in their own section directly below the general inventory slots (one continuous view)."
  - "Item icons are REAL P1999 wiki images via https://wiki.project1999.com/images/Item_NNN.png (icon number per item); colored-tile fallback on load error. Production should map each item to its EQ icon id (cache on the Hetzner box per spec)."
---

# Sketch 002: Character Inventory Window

## Design Question
Your spec: clicking a character shows "an organized grid of each of the items equipped on that character" that "look[s] and behave[s] as much like the in-game inventory menu as possible." This sketch tries three fidelities, all with: real EQ slot taxonomy, stack counts, a ⊞ backpack you can open, a Bank toggle, and a right-click-style **examine** popover (wiki stats + PigParse price + wiki link + last-synced — sourced from the Hetzner-stored wiki data).

Example character: **Bashley** (L58 Half Elf Ranger), reached from the Characters list (sketch 001).

## How to View
open .planning/sketches/002-character-inventory-window/index.html

Hover items to examine them. Click the **⊞ Large Lambent Bag** (first general slot) to open it. Toggle **Inventory / Bank**. Switch variants A/B/C up top.

## Variants
- **D: ★ Chosen** — A's faithful paperdoll + C's click-to-pin detail panel; bank items listed below general inventory (no Inventory/Bank toggle); real P1999 wiki item icons. The default tab.
- **A: Faithful paperdoll** — equipment slots flank a center figure exactly like the EQ inventory window; weapons + general inventory below. Examine appears as a hover popover (the in-game right-click feel). Maximum game fidelity.
- **B: Grouped panels** — a web-native abstraction: slots collected into labeled cards (Armor / Jewelry / Weapons / General). Less literal, more scannable, reflows better on narrow screens.
- **C: Paperdoll + pinned detail** — the faithful paperdoll, but clicking an item **pins** its examine into a side panel instead of hovering. Touch-friendly (no hover needed) and lets you compare while scrolling.

## What to Look For
- Is the literal paperdoll (A) worth it, or does the grouped view (B) read better on the web?
- Hover popover (A/B) vs click-to-pin (C) for the "right-click examine" — which suits desktop **and** the spec's "hover **or tap**"?
- Backpack drill-down: does opening a bag in a modal feel right, or should it expand inline?
- The Bank toggle here is a simplification (swaps the general area to bank slots) — the real thing should render a proper bank-window grid. Flag how literal the bank view needs to be.
- Examine content/order: is the EQ-style block (flags → slot/skill → DMG/DLY → AC → stats → wt/size → class/race → PigParse/wiki/synced) the right set and order?

## Notes / open data questions (for planning, not this sketch)
- The in-game layout requires the backend to parse the inventory file's `Location` column into a slot taxonomy + container nesting (today it's an opaque string). This is the "new data architecture" the spec anticipates.
- Item stats in the examine come from the existing wiki enrichment (`_item_master` / gear-tier scrape) already on the Hetzner box — surfacing, not new scraping.
