# Sketch Manifest — v2.4 Web UI Revamp (5-Tab Restructure)

## Design Direction

Reorganize squirebot.quest around the five top-level tabs the user specified in `Future Features.txt` (desktop, 2026-06-17): **Characters · Inventory · Banks · Wishlist · Settings**, each answering one question. The aesthetic is fixed — reuse the live EQ themes (Velious default; tokens `--accent`/`--panel`/`--font-display`, fonts Cinzel Decorative + IM Fell English) untouched. This is a **structure & interaction** exploration: information architecture, navigation, the in-game-style inventory window, the item-centric inventory list, bank valuation, and the per-character/per-slot wishlist. The consolidated-views lock was **relaxed 2026-06-17** — per-character master-detail drill-down is allowed.

Mockups are throwaway HTML with fake-but-realistic guild data (real char names: Slampeach, Biteaucul, Bashley, Findom; real Velious gear-tier item names; real EQ slot taxonomy).

## Locked design decisions (running)

- **App shell = top tabs (sketch 001, Variant A)**, in order: Characters · Inventory · Banks · Wishlist · Settings.
- **Notifications belong to the Wishlist tab, not Settings.** Every SquireBot alert is a wishlist-item ping (EC tunnel / WTS / raid), so the unread badge + alert inbox + ping prefs live with the Wishlist. Settings keeps Theme / Watcher Codes / Set Class & Level / My Characters / Admin.
- **Consolidated-views lock relaxed (2026-06-17)** — per-character master-detail drill-down allowed.
- **Character inventory window (sketch 002) = Variant D** — faithful EQ paperdoll + click-to-pin detail; bank listed below general (no toggle).
- **Inventory list (sketch 003) = Variant B (master-detail)**, same click-to-pin model as 002. Banks tab as-is (item-value + platinum totals).
- **Wishlist (sketch 004) = Variant C (master-detail, suggestions-forward).** Per-character, per-equipment-slot upgrade list; **open-ended** (no 3-cap). Every item shows PigParse price + wiki link + **last-listed-for-sale** date. Suggestions = the **complete** Velious Pre-raid/Grouping + Raiding lists per (class, slot) from the existing `_wiki_gear_tier` scrape (`enrich/wikigear.go` parses tiers `Velious Pre-Raid/Group` + `Velious Raiding`); gear-tier rows have **no item_id**, so price/last-listed join by **normalized name**. **No Group/Raid binary** — only no-drop + raid-only items get a `Raid` tag (not buyable); everything else is unlabeled (groupable / EC-buyable). Discord ping toggle + EC-hit badge reuse the existing notification + EC-monitor spine.
- **Real item icons from the P1999 wiki** — pattern `https://wiki.project1999.com/images/Item_<iconId>.png` (verified: Cloak of Flames 658, Wurmslayer 736, Ring of the Ancients 563, Blue Diamond 966, Rubicite BP 624, Cloudy Potion 585). Each EQ item carries an icon id; production maps item→iconId and caches on the Hetzner box (spec: "wiki information stored on the Hetzner server"). Tooltip = right-click examine (flags → slot/skill → DMG/DLY → AC → stats → wt/size → class/race → PigParse price → wiki link → last-synced).

## Reference Points

- The **in-game EQ inventory window** (paperdoll equipment slots + general inventory + bank window) — the Characters tab should "look and behave as much like the in-game inventory menu as possible."
- The live SquireBot site (squirebot.quest) — current DataGrid, tooltips, EQ theming to carry forward.
- `Future Features.txt` — the authoritative target-UX spec.

## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|----------------|--------|------|
| 001 | app-shell-5tab-nav | What's the right top-level frame for the 5 tabs + per-tab search + character list? | **A · top tabs** | nav, ia, shell |
| 002 | character-inventory-window | How faithfully should the per-character inventory render the in-game window (slots, stacks, backpack drill-down, bank)? | **D · paperdoll + click-to-pin + real wiki icons** | inventory, drill-down, tooltip |
| 003 | inventory-and-banks-lists | How do the item-centric Inventory list + Banks valuation present (expandable holders, totals)? | **B · master-detail** (Banks as-is) | list, expand, totals |
| 004 | wishlist-per-slot | How does the per-character per-equipment-slot upgrade wishlist (open-ended blanks + full wiki suggestions + price/last-listed + ping/badge) feel? | **C · master-detail, suggestions-forward** | wishlist, suggestions, pings |
