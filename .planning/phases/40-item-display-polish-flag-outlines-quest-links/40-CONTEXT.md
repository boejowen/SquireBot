# Phase 40: Item display polish — flag outlines + named quest links - Context

**Gathered:** 2026-06-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Make items readable at a glance and traceable to their quests, **on the modern web tabs only**:

- **ITEMUI-01** — every item *tile* carries a color-coded outline by its attribute flag: **No-Drop = red, Lore = gold, Magic = blue**; an item with none of those keeps the existing neutral border.
- **ITEMUI-02** — the modern Characters / Inventory / Wishlist tabs surface the **named quests** an item is used in ("used in quest X"), not just the yes/no QUEST-ITEM badge.

**No new data pipeline, no migration, watcher untouched (→ no `v*` tag).** Both halves plumb data that *already exists*: the P37 discrete flag columns in `item_master`, and the `quest_items` table + `store.QuestLinksByItem` (already wired into the legacy `view`/`bank` builders). Out of scope: epic-quest step modeling (deferred — generic named links only), any watcher change, any new enrichment crawl.

</domain>

<decisions>
## Implementation Decisions

### ITEMUI-01 — Flag outline (the tile border)
- **D-01 (multi-flag resolution):** **Priority outline** — a single border color, most-restrictive flag wins, in fixed order **No-Drop (red) > Lore (gold) > Magic (blue)**. An item carrying none of the three keeps the existing neutral border (`var(--border)`). Rationale: a 62px tile can't legibly carry three border colors, and No-Drop is the most action-relevant flag for the guild (it governs whether an item can move between guildies — the Core Value question). No information is lost overall because the examine panel still lists every flag in full (the stored wiki statsblock already shows MAGIC/LORE/NO-DROP).
- **D-02 (surfaces):** The outline applies to the **62px icon tiles** (`PaperdollSlot.svelte` — the Characters paper-doll equipment slots + the inventory-window general grid + the Banks tab grid) **AND** the **examine panel's item-name heading** (`ExaminePanel.svelte` / `examine.ts` `name` field), colored by the same D-01 priority flag. **Dense DataGrid list rows (Inventory tab / Wishlist) are NOT outlined for ITEMUI-01** — the user chose "Tiles + examine heading," not list-row tinting. (Reconciles with ROADMAP SC-2's "inventory/bank list rows" = the *rows of tiles* inside the inventory window, which ARE tiles — not the item-centric DataGrid.)
- **D-03 (hover/focus coexistence) — PRINCIPLE locked, mechanism is Claude's discretion:** the flag color MUST stay visible while a tile is hovered/focused. Today `.slot.filled:hover` / `:focus-visible` override `border-color` to `var(--accent)` plus a box-shadow ring, so the flag CANNOT simply be the resting `border-color` (it would vanish on hover). The flag must ride a dedicated, static treatment (e.g. an inset `box-shadow` ring or a `::before` outline keyed off a `--flag-color` CSS var) that coexists with the existing accent hover/focus affordance. Exact CSS mechanism → UI-SPEC / planner.

### ITEMUI-02 — Named quest links
- **D-04 (where they render): detail surfaces only.** Named quests appear in (a) the Characters **examine panel** (`examine.ts` / `ExaminePanel.svelte`) and (b) the Inventory/Wishlist **item tooltip** (`composeNotes.ts` / `ItemTooltip.svelte`). Dense list rows keep the compact yes/no QUEST-ITEM badge — no inline quest names in rows.
- **D-05 (interactivity): clickable wiki links.** Each named quest links to its P1999 wiki page via the **stored `quest_items.source_url`**; falls back to **plain text** when no URL was harvested. Every `href` MUST pass through the existing `safeHttpUrl` scheme allow-list before render (the load-bearing escaping control — same guard the examine wiki line already uses). Quest names are raw/user-controlled → Svelte auto-escaping for the text.
- **D-06 (named vs generic — the load-bearing data distinction):** a `quest_items` row is one of two kinds — a **real named quest** (`source = 'notes_link'`, has a real `quest_name` + a `source_url`) OR the **generic in-game QUEST flag** (`source = 'in_game_flag'`, `quest_name = '[in-game QUEST flag]'`, no URL). The "used in quest X" list renders ONLY the `notes_link` entries. When an item is a quest item but has *only* the `in_game_flag` entry (no harvested named quest), the existing generic **"QUEST ITEM"** badge stays as the fallback. Never render the literal `[in-game QUEST flag]` pseudo-name as a quest.

### Plumbing (both halves — no migration, extend-only)
- **D-07 (flag plumbing, ITEMUI-01):** `item_master` already has `is_lore` / `is_no_drop` / `is_magic` / `is_temporary` (Phase 37, migration 00016). Carry `is_no_drop` / `is_lore` / `is_magic` through `compute` (the `InventorySlot` builder + the `ItemRollup` builder read them off `item_master`) → the Go JSON types → `web/src/lib/api.ts` (`InventorySlot` + `ItemRollup` gain the three booleans — exactly the Phase-39 `is_clicky`/`has_haste` precedent at api.ts:250-251). The outline is a HELD-item concern (every tile shows a held item keyed by EQ `item_id`), so the **`item_master` (held) read is the source** — `catalog_enrichment` (P38, unheld/name-keyed) is NOT involved here.
- **D-08 (quest plumbing, ITEMUI-02):** extend `store.QuestLinksByItem`'s SELECT to also read `source_url` (currently selects only `item_id, quest_name, source`), add `SourceURL` to `store.QuestLinkRow` + `compute.QuestLink` (currently `QuestName` + `Source` only), then attach the existing per-item map into the **modern** `compute.InventorySlot` + `compute.ItemRollup` builders — the same call (`s.QuestLinksByItem(ctx)`) already used in `compute/view.go` + `compute/bank.go`. `ViewRow` already carries `quest_links` (api.ts:107) — leave it untouched. Add `quest_links: QuestLink[]` to `InventorySlot` + `ItemRollup` in `api.ts`.

### Claude's Discretion
- The exact CSS mechanism for the flag outline (box-shadow ring vs `::before` outline vs layered border) and the `--flag-color` token wiring — within D-03's "survives hover/focus" principle and the brand token system.
- Examine-heading flag tinting treatment (full-color heading vs a small flag chip beside the name) — within D-02.
- Whether the examine/tooltip lists multiple named quests comma-joined on one line vs one per line, and any cap when an item feeds many quests (default: show all `notes_link` entries; the set is small in practice).
- Whether `is_temporary` is also plumbed for future use (it exists in `item_master`) — but it is NOT part of the D-01 outline (locked to the three named flags).

</decisions>

<specifics>
## Specific Ideas

- Flag colors are fixed by the requirement and the user: **No-Drop = red, Lore = gold, Magic = blue**. These map to the EQ-aesthetic palette — the planner should source the actual hex/token from `docs/design/eq-aesthetic-theme.md` / the `THEMES` registry, not invent literals (e.g. a "danger/red", "gold/accent-warm", "info/blue" token), keeping the one-sanctioned-non-token-color rule (the per-item tile gradient) the only exception.
- "Used in quest X" mirrors the in-game / wiki phrasing; clickable to the P1999 wiki walkthrough is the high-value behavior (jump straight to how to complete the quest).
- The priority outline deliberately trades multi-flag completeness on the tile for legibility, because the examine panel is the full-detail surface.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope + requirements
- `.planning/ROADMAP.md` § "Phase 40" (lines ~456-468) — goal, success criteria 1-4, codebase facts, dependency on Phase 37.
- `.planning/REQUIREMENTS.md` — ITEMUI-01 (line 29), ITEMUI-02 (line 30); the deferred epic-quest note (line 44).

### ITEMUI-01 — flags (data already shipped in P37)
- `internal/backendsrv/migrations/00016_item_flags_effects.sql` — the `item_master` flag columns (`is_lore`/`is_no_drop`/`is_magic`/`is_temporary`) this phase reads.
- `web/src/lib/components/PaperdollSlot.svelte` — the 62px tile; the `.slot` border + hover/focus rules the outline must coexist with (D-03).
- `web/src/lib/components/InventoryWindow.svelte`, `web/src/lib/components/ExaminePanel.svelte` — the Characters-tab surfaces (tile grids + examine heading, D-02).
- `web/src/lib/api.ts` §`InventorySlot` (177-197) + §`ItemRollup` (237-253) — the `is_clicky`/`has_haste` Phase-39 precedent the three new flag booleans follow.
- `docs/design/eq-aesthetic-theme.md` + `apps-script/src/lib/themes.ts` `THEMES` — source of truth for the red/gold/blue flag tokens (no literal hex).

### ITEMUI-02 — named quest links (data already harvested)
- `internal/backendsrv/migrations/00001_init.sql` line 63 — `quest_items(item_id, quest_name, source_url, source, …)` schema.
- `internal/backendsrv/store/readviews.go` §`QuestLinksByItem` (460-) + `QuestLinkRow` (96-99) — the existing grouped-by-item_id read to extend with `source_url`.
- `internal/backendsrv/compute/view.go` (52) + `compute/bank.go` (39) — the existing `s.QuestLinksByItem(ctx)` attach pattern to mirror into the InventorySlot/ItemRollup builders.
- `internal/backendsrv/compute/types.go` §`QuestLink` (71-76) — gains a `SourceURL` field.
- `internal/backendsrv/enrich/wikiitem.go` §`WikiQuestItemLink` (82-85) — confirms the `notes_link` vs `in_game_flag` source taxonomy + the `[in-game QUEST flag]` pseudo-name (D-06).
- `web/src/lib/examine.ts` (the `flags`/`QUEST ITEM` field, 71-75) + `web/src/lib/tooltip/composeNotes.ts` + `web/src/lib/components/ItemTooltip.svelte` — the detail surfaces that render the named links (D-04); `examine.ts` `wikiHref` + the `safeHttpUrl` guard are the URL-escaping pattern for D-05.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `store.QuestLinksByItem(ctx) → map[int64][]QuestLinkRow` — already returns quest links grouped by item_id; `compute/view.go` + `compute/bank.go` already consume it for `ViewRow.quest_links`. ITEMUI-02 reuses this verbatim (plus a `source_url` SELECT add) for the modern builders.
- `web/src/lib/api.ts` `ViewRow.quest_links` (line 107) + `compute.QuestLink` — the JSON contract already exists for the legacy views; the modern models copy it.
- `examine.ts` `wikiHref()` + the `ExaminePanel` `safeHttpUrl` scheme allow-list — the proven clickable-wiki-link + escaping pattern (D-05) to reuse for quest links.
- The Phase-39 `is_clicky`/`has_haste` plumbing (compute → types → api.ts) is the exact template for the three flag booleans (D-07).

### Established Patterns
- Extend-only schema/contract evolution: add columns at the right edge, add JSON fields, never break. No migration here (P37 columns + the quest_items table both pre-exist).
- One sanctioned non-token color (the per-item tile gradient in `PaperdollSlot`); flag colors MUST be theme tokens sourced from the EQ-aesthetic doc.
- `examine.ts` is a pure, DOM-free, node-testable helper (the order/omission contract is unit-tested) — the named-quest fields belong there (testable) with `ExaminePanel.svelte` doing the render.

### Integration Points
- **Backend:** `compute` InventorySlot + ItemRollup builders (attach flags + quest_links) → Go JSON types (`compute/types.go`) → no API route signature change (additive fields on existing payloads).
- **Web:** `api.ts` type additions → `PaperdollSlot.svelte` (flag outline) + `ExaminePanel.svelte`/`examine.ts` (heading tint + named quests) + `ItemTooltip.svelte`/`composeNotes.ts` (Inventory/Wishlist named quests).
- **Tests:** Go `compute`/`store` table tests for the new fields; web vitest is DOM-blind (no `@testing-library/svelte`) → the examine/tooltip RENDER stays a **browser-smoke checkpoint** (deploy-then-smoke-on-prod, per the established web-smoke discipline). Expect a `/gsd-ui-phase 40` gate before planning (UI hint: yes).

</code_context>

<deferred>
## Deferred Ideas

- Epic-quest-specific tracking (which epic an item feeds, per class, with steps/turn-ins) — needs a new quest-step data model; explicitly deferred in REQUIREMENTS (line 44). v2.6 ships generic named links only.
- Tinting the dense Inventory/Wishlist DataGrid list rows by flag — user chose "Tiles + examine heading"; not this phase.
- Inline named-quest names directly in list rows — user chose "Detail surfaces"; not this phase.
- `is_temporary` as a fourth outline color — out of the locked three-flag set.

</deferred>

---

*Phase: 40-item-display-polish-flag-outlines-quest-links*
*Context gathered: 2026-06-25*
