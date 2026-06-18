---
sketch: 004
name: wishlist-per-slot
question: "How does the per-character, per-equipment-slot upgrade wishlist (equipped item + 3 blanks + Velious wiki suggestions + Discord ping toggle + EC badge) feel?"
winner: "C"
tags: [wishlist, suggestions, pings, characters]
decisions:
  - "Wishlist = Variant C (master-detail, suggestions-forward) chosen 2026-06-17 — slot list + detail panel showing the full Velious upgrade list."
  - "Every item (equipped, wishlisted, suggestion) shows PigParse price + wiki link + last-listed-for-sale date."
  - "NO hard cap on upgrades per slot (was 3) — open-ended."
  - "Suggestions = the COMPLETE Velious Pre-raid/Grouping + Raiding lists per (class, slot) from the existing _wiki_gear_tier scrape (enrich/wikigear.go parses tiers 'Velious Pre-Raid/Group' + 'Velious Raiding' from two wiki pages via the MediaWiki API). Gear-tier rows have NO item_id, so PigParse price + last-listed join by NORMALIZED NAME."
  - "Tier labels dropped: NO Group/Raid binary. Only no-drop + raid-only items get a 'Raid' tag (can't be bought); everything else is unlabeled (groupable or buyable in EC) and shows price + last-listed when SquireBot has seen it for sale."
---

# Sketch 004: Per-slot Wishlist

## Design Question
Your spec: in the Wishlist tab, clicking a character shows a grid that — unlike the Characters tab — does **not** look like the in-game window. It lists only the **equipped** items; beside each, **three blanks** where you enter upgrade targets (your wishlist for that slot). SquireBot **suggests** items for each blank from the wiki's *Velious Pre-raid/Grouping* and *Velious Raiding* sections. Each wishlisted item has a **Discord ping toggle**; if it pings you (e.g. it appeared in the EC tunnel), a **badge** shows next to it.

Example char: **Bashley** (L58 Half Elf Ranger), with 3 upgrades already wishlisted (Arms → Crystal Chitin Armplates has a 🔔-on + **EC ✦** badge).

## How to View
open .planning/sketches/004-wishlist-per-slot/index.html

Click a **Suggested (Velious)** chip to add it, or type in the **+ add upgrade…** box (datalist suggestions; Enter to add). Toggle **🔔** to flip the Discord ping; **✕** removes. Note the **EC ✦** badge on the Arms upgrade (it appeared in the tunnel). Switch variants up top.

## Variants
- **C: ★ Chosen — Master-detail (suggestions-forward)** — slot list left (with a per-slot count of upgrades added); right panel shows the equipped item + the full Velious suggestion list, each row with a **+ add**, a **Raid** tag if no-drop/raid-only, PigParse **price** (or "raid drop — not sold"), **wiki ↗**, and **last-listed** date. Open-ended (no cap). The default tab.
- **A: Slot table** — every slot in one dense scan; same suggestion rows inline per slot.
- **B: Slot cards** — roomier card per slot.

## What to Look For (chosen design)
- The suggestion rows: **price + last-listed + wiki** per item, **Raid** tag only on no-drop/raid-only items (no Group/Raid binary). Reads clearly?
- Open-ended adds (no 3-cap) — the **+ add any item by name** box plus one-click **+** from suggestions.
- 🔔 ping toggle + **EC ✦** badge on wishlisted items (see Arms → Crystal Chitin Armplates).
- The detail panel shows ~3-4 suggestions per slot here; production shows the **complete** wiki list per (class, slot).

## Notes (for planning — the heaviest new feature)
- New data model: wishlist keyed by (character, equipment-slot) rather than the current (user, item). Reworks the v2.2/v2.3 wantlist; the Discord ping + EC-hit badge reuse the existing EC monitor + notification spine.
- Suggestions come from the existing Velious gear-tier wiki scrape (today's `gear_check` source) filtered to the slot + class.
- "SquireBot notices the item on the character → drops it off the wishlist" needs the inventory parse from 002.
