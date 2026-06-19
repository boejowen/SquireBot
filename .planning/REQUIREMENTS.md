# Requirements — Milestone v2.4 "Web UI Revamp (5-Tab Restructure)"

**Source spec:** `Future Features.txt` (user-authored, 2026-06-17) + the locked sketch decisions in `.planning/sketches/MANIFEST.md` (sketches 001–004).

**Goal:** Reorganize squirebot.quest around five top-level tabs — **Characters · Inventory · Banks · Wishlist · Settings** — each answering one user question, backed by the new data architecture this requires.

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher is untouched** — it already uploads the inventory `Location | Name | ID | Count | Slots` data; this milestone parses and surfaces it. Reworks the v2.2/v2.3 wantlist into the per-slot Wishlist. Reuses the shipped EC-monitor + notification spine for pings (v2.2 Track 2 WTS/raid monitors stay parked).

**Locked architecture decisions:** consolidated-views lock **relaxed** — per-character master-detail drill-down allowed (CLAUDE.md updated). Gear-tier/wiki items carry **no item_id**, so PigParse price + last-listed join by **normalized name**.

---

## Requirements

### Navigation & Shell (NAV)
- [x] **NAV-01** — The site presents five persistent top-level tabs (Characters, Inventory, Banks, Wishlist, Settings) with the active tab indicated. ✅ Plan 30-01 (5-tab strip in SiteShell, path-derived aria-current).
- [x] **NAV-02** — Each tab has its own in-context search bar scoped to that tab's content. ✅ Pattern-established Plan 30-02 (the WantlistPanel filter is the Wishlist tab's scoped search; the live section-filter is the Settings tab's search); the stub tabs get theirs with content in Phases 31/32/33. Done — deployed live + browser-smoke PASS 2026-06-18.
- [x] **NAV-03** — The Settings tab consolidates the existing surfaces: Theme, Notifications prefs, Watcher Codes, Set Class & Level, My Characters, and (officers only) Admin — with a settings search. ✅ Plan 30-02 (/settings composes the 6 panels as in-page id'd sections behind an officer-gated Admin section + a live settings search; notifications prefs moved to the Wishlist tab per D-07). Done — deployed live + browser-smoke PASS 2026-06-18.
- [x] **NAV-04** — Notifications belong to the Wishlist tab: the unread-alert badge sits on the Wishlist tab and the alert inbox + per-item ping prefs are reached there (every alert is a wishlist-item ping). ✅ Chrome Plan 30-01 (badge→Wishlist tab) + inbox/prefs/per-item ping (the WantlistPanel mute bell) Plan 30-02 (/wishlist composes WantlistPanel + NotificationPrefsPanel + NotificationInbox). Done — deployed live + browser-smoke PASS 2026-06-18.

### Characters tab (CHAR) — "what does character X have?"
- [x] **CHAR-01** — Lists all guild characters with name, level, race, class; default order = the viewer's characters first (A-Z), then other guild characters, then guild banks/bots.
- [x] **CHAR-02** — Per-character name search that prioritizes the viewer's characters.
- [x] **CHAR-03** — Selecting a character (from the list or a search result) opens that character's inventory window. ✅ Phase 31

### Character inventory window (INV)
- [x] **INV-01** — A character's inventory renders in an in-game-style window: equipment slots in the EQ paperdoll arrangement, general-inventory slots, and the character's bank items listed below; stacked slots show their count. ✅ Phase 31
- [x] **INV-02** — Hovering or tapping an item shows a right-click-style examine: item name + stats (from the stored wiki data), PigParse price, wiki link, and last-synced. (Sketch decision: click-to-pin detail panel.) ✅ Phase 31
- [x] **INV-03** — General-inventory containers (bags) can be opened to view their contents, which behave like the inventory grid. ✅ Phase 31
- [x] **INV-04** — Item icons render from the P1999 wiki item-icon images. ✅ Phase 31
- [x] **INV-05** — *(backend/data)* The watcher's `Location`/`Slots` inventory data is parsed server-side into a slot taxonomy + container nesting that powers INV-01..03. Watcher unchanged. ✅ Phase 29

### Inventory tab (ITEM) — "which characters have item X?"
- [ ] **ITEM-01** — Lists all guild items with name, guild-wide quantity, a wiki link, and a PigParse price that links to PigParse (when applicable).
- [ ] **ITEM-02** — Per-item name search that prioritizes items on the viewer's characters.
- [ ] **ITEM-03** — Selecting an item shows which characters hold it, the inventory slot on each, the quantity, and the last-synced day/time (master-detail, consistent with INV-02).

### Banks tab (BANK) — "what's in the guild banks?"
- [x] **BANK-01** — Lists only guild-bank characters (same ordering style as Characters); each opens its inventory window. ✅ Phase 33 (deployed live + browser-smoke PASS 2026-06-19)
- [x] **BANK-02** — Shows the total PigParse value of all items held by bank characters and the total platinum held across the guild banks. ✅ Phase 33
- [x] **BANK-03** — Per-item name search across the items held by the guild banks. ✅ Phase 33

### Wishlist tab (WISH) — "what can I get to improve my characters?" *(reworks v2.2/v2.3 wantlist)*
- [ ] **WISH-01** — Lists characters (the viewer's first A-Z, then others); excludes guild banks/bots; per-character.
- [ ] **WISH-02** — Selecting a character shows its equipped slots (not the in-game window) with the currently-equipped item per slot.
- [ ] **WISH-03** — Each equipped slot holds an **open-ended** set of user-entered upgrade targets (the slot's wishlist); items are typed or chosen from suggestions; an item leaves the wishlist when SquireBot sees it on that character or the user removes it.
- [ ] **WISH-04** — Per slot, SquireBot suggests upgrades from the **complete** Velious Pre-raid/Grouping + Raiding lists for that class+slot (from the existing `_wiki_gear_tier` data); each suggestion shows its PigParse price, wiki link, and last-listed-for-sale date; no-drop/raid-only items are tagged "Raid" and shown as not-for-sale (no Group/Raid binary).
- [ ] **WISH-05** — Each wishlisted item has a Discord ping toggle; when SquireBot pings the user (e.g. the item appeared in the EC tunnel), a badge appears beside that item in the wishlist. Reuses the shipped EC-monitor + notification spine.
- [ ] **WISH-06** — Hovering or tapping any item shows the right-click-style examine (stats, price, wiki, last-synced).
- [ ] **WISH-07** — Wishlist search covers all items on any wishlist plus the non-bank/bot characters.

### Cross-cutting data (DATA)
- [x] **DATA-01** — PigParse price + last-listed-for-sale data joins to wiki/gear-tier items by **normalized name** (gear-tier rows carry no item_id); surfaced on examine, suggestions, and item lists. ✅ Phase 29 (name-keyed `pp_rep` join extended to gear-tier rows via `store.GearTierPrices`)
- [x] **DATA-02** — Bank valuation aggregation (sum of PigParse item value per bank + guild-wide) and total platinum (from the manual bank-coin entries) power BANK-02. ✅ Phase 29 (`BankValuationFor` Σ pickPrice×count +N unpriced; `TotalPlatinum` literal plat)

---

## Out of Scope (this milestone)
| Item | Reason |
|------|--------|
| New color themes / visual re-skin | This is a structure + data revamp; the existing 5 EQ themes are reused unchanged. |
| v2.2 Track 2 (WTS / raid-target Discord monitors) | Still parked on the 3 Raid Alliance bot invites; independent of this milestone. WISH-05 reuses only the shipped EC monitor. |
| Watcher changes | The watcher already uploads `Location`/`Slots`; all new work is backend parsing + web. |
| Coin from inventory files | The file format has no coin; platinum stays manual bank-coin entry (BANK-02 / DATA-02). |

## Traceability
*Each v2.4 requirement maps to exactly one phase (filled by the roadmap 2026-06-17). Coverage: 27/27 — no orphans, no duplicates.*

| Requirement | Phase | Status |
|-------------|-------|--------|
| NAV-01 | Phase 30 | ✅ Done (Plan 30-01) |
| NAV-02 | Phase 30 | ✅ Done (Plan 30-02 — wantlist filter + settings search; deployed browser-smoke PASS 2026-06-18) |
| NAV-03 | Phase 30 | ✅ Done (Plan 30-02 — /settings 6 sections + officer gate + search; deployed browser-smoke PASS 2026-06-18) |
| NAV-04 | Phase 30 | ✅ Done (Plan 30-01 badge→Wishlist tab + Plan 30-02 inbox/prefs; deployed browser-smoke PASS 2026-06-18) |
| CHAR-01 | Phase 31 | ✅ Done (31-03: bespoke 3-band viewer-first list — YOUR CHARACTERS/GUILD/BANKS & BOTS, A-Z within band — over the 31-02 GET /api/v1/characters roster; deployed + browser-smoke PASS 2026-06-18) |
| CHAR-02 | Phase 31 | ✅ Done (31-03: scoped viewer-priority search via the pure node-tested filterRoster, keeps mine→guild→banks ranking among matches; deployed + browser-smoke PASS 2026-06-18) |
| CHAR-03 | Phase 31 | ✅ Done (31-03 selection + 31-04 window render: row/search click → fetchInventory → InventoryWindow opens; deployed + browser-smoke PASS 2026-06-18) |
| INV-01 | Phase 31 | ✅ Done (31-04: 21-of-23-slot paperdoll [Charm/Power Source omitted — post-Velious] + general/bank grids + stack counts over StructuredInventory; deployed + browser-smoke PASS 2026-06-18) |
| INV-02 | Phase 31 | ✅ Done (31-04: hover-preview + click-to-pin ExaminePanel — name/stats/price/wiki/last-synced; statsblock via migration 00013; deployed + browser-smoke PASS 2026-06-18) |
| INV-03 | Phase 31 | ✅ Done (31-04: inline bag expand — children-based detection — over the Children[] nesting; deployed + browser-smoke PASS 2026-06-18) |
| INV-04 | Phase 31 | ✅ Done (31-01 icon_id enrichment via migration 00012 + 31-04 PaperdollSlot Item_<iconId>.png with colored-tile onerror fallback; deployed + browser-smoke PASS 2026-06-18) |
| INV-05 | Phase 29 | ✅ Done |
| ITEM-01 | Phase 32 | ✅ Done — guild-wide item list (name + qty + holder count + inline PigParse price + Wiki ↗) live at squirebot.quest/inventory over GET /api/v1/items; deployed + 7-point browser-smoke PASS across 5 themes 2026-06-18 |
| ITEM-02 | Phase 32 | ✅ Done — per-item name search floats the viewer's own characters' items first (server is_mine + pure items.ts viewer-first/filter); deployed + browser-smoke PASS 2026-06-18 |
| ITEM-03 | Phase 32 | ✅ Done — selecting an item shows holders (char · slot · qty · last-synced) via the reused ExaminePanel + holders table deep-linking to /characters?c=; deployed + browser-smoke PASS 2026-06-18 |
| BANK-01 | Phase 33 | ✅ Done — banks/bots A-Z list, each opens the reused P31 inventory window; live at squirebot.quest/banks |
| BANK-02 | Phase 33 | ✅ Done — guild-wide summary (total PigParse value incl. bots + total platinum) via compute.Banks over the shipped BankValuation |
| BANK-03 | Phase 33 | ✅ Done — item-centric search across bank holders (bank-slice qty), holder-click opens the bank window in-tab |
| WISH-01 | Phase 34 | Pending |
| WISH-02 | Phase 34 | Pending |
| WISH-03 | Phase 34 | Pending |
| WISH-04 | Phase 34 | Pending |
| WISH-05 | Phase 34 | Pending |
| WISH-06 | Phase 34 | Pending |
| WISH-07 | Phase 34 | Pending |
| DATA-01 | Phase 29 | ✅ Done |
| DATA-02 | Phase 29 | ✅ Done |

**Coverage by phase:**

| Phase | Requirements | Count |
|-------|--------------|-------|
| Phase 29 — Data Foundation (Inventory Parse + Price/Value Aggregation) | INV-05, DATA-01, DATA-02 | 3 |
| Phase 30 — App Shell + 5-Tab Navigation | NAV-01, NAV-02, NAV-03, NAV-04 | 4 |
| Phase 31 — Characters Tab + In-Game Inventory Window | CHAR-01, CHAR-02, CHAR-03, INV-01, INV-02, INV-03, INV-04 | 7 |
| Phase 32 — Inventory Tab (Item-Centric) | ITEM-01, ITEM-02, ITEM-03 | 3 |
| Phase 33 — Banks Tab + Valuation | BANK-01, BANK-02, BANK-03 | 3 |
| Phase 34 — Wishlist Rework (Per-Character Per-Slot) | WISH-01, WISH-02, WISH-03, WISH-04, WISH-05, WISH-06, WISH-07 | 7 |
| **Total** | | **27** |

---
*Created 2026-06-17 for milestone v2.4. REQ-IDs start fresh per new category (no overlap with prior milestones' AUTH/WANT/ASSIGN/etc.). Traceability filled by the roadmap 2026-06-17 — 27/27 mapped across Phases 29–34.*
