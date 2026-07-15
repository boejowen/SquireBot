# SquireBot — Requirements: v2.6 "Item Detail & Polish"

**Milestone goal:** Turn the item data SquireBot already half-captures into usable intelligence — faceted item search (Clicky / Haste), flag-coded outlines, named quest links on the modern tabs, broader icon/tier coverage — and tighten the Characters (paper-doll), Wishlist, and item-display surfaces.

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher is untouched** → **no `v*` tag** (consistent with v2.3/v2.4/v2.5). Phases continue from v2.5 (last = 36) → v2.6 starts at **Phase 37**.

**Source spec:** `Future Features.txt` (user-authored, 2026-06-24) — 1 bug, 4 tweaks, 1 feature. Codebase mapped 2026-06-24 (data-availability per item confirmed). Research skipped (extends well-understood subsystems).

**Status:** Roadmap created 2026-06-24 — Traceability filled; Phase 37 ready to plan.

---

## Requirements

### Item Enrichment Backbone (ENRICH)
> The shared data layer for the flag outlines (ITEMUI-01), the search facets (SEARCH-04/05), and the missing-images bug (ENRICH-15). The wiki stat-block parser already *computes* flags + effects but the Go port deliberately discards them (the `wikiitem.go` D-8 scope guard); this milestone re-enables that into discrete, queryable fields.

- [ ] **ENRICH-12**: Item attribute flags (LORE, NO DROP, MAGIC, TEMPORARY) are parsed from the wiki stat block into discrete, queryable fields. *(New migration + the enrich freshness short-circuit updated so existing rows backfill.)*
- [ ] **ENRICH-13**: Item click-effect (Clicky) and Haste are parsed from the wiki stat block into discrete, queryable fields. *(Same parser pass as ENRICH-12.)*
- [ ] **ENRICH-14**: Item enrichment covers the full PigParse Blue catalog, not only currently-held items. *(Widen the inventory-driven `DistinctInventoryItemIDs` gate; powers the full-catalog search scope and fixes most missing icons; politefetch-paced so the weekly crawl stays courteous.)*
- [ ] **ENRICH-15**: Icon coverage is backfilled for every item whose wiki page provides an icon, and a maintainer can see which items are still icon-less. *(The "not all items have images" bug; the colored-tile fallback remains for genuinely icon-less items — this closes the *fixable* gap and makes the residue visible.)*

### Faceted Item Search (SEARCH)
- [x] **SEARCH-04**: A user can filter item search to only Clicky items (items with a click effect).
- [x] **SEARCH-05**: A user can filter item search to only Haste items.
- [x] **SEARCH-06**: A user can toggle item-search scope between guild holdings ("who has one") and the full P99 catalog ("what exists").

### Item Display Polish (ITEMUI)
- [x] **ITEMUI-01**: Items in the inventory, bank, and paper-doll views show a color-coded outline by flag — No-Drop = red, Lore = gold, Magic = blue (default/no special flag stays the existing neutral border).
- [x] **ITEMUI-02**: The modern Characters / Inventory / Wishlist tabs show the named quests an item is used in ("used in quest X"), not just the yes/no QUEST-ITEM badge. *(Plumb the already-harvested `quest_links` into `InventorySlot`/`ItemRollup` + the examine panel.)*

### Character Paper-Doll (CHARUI)
- [ ] **CHARUI-01**: The character paper-doll view is compacted to reclaim the empty portrait space, tightening toward the in-game inventory-window layout.
- [ ] **CHARUI-02**: A user can set an optional portrait photo for each of their characters, shown in the paper-doll's portrait area. *(New per-character image reference + migration + upload path + read-API plumbing.)*

### Wishlist Polish (WISHUI)
- [ ] **WISHUI-01**: The Wishlist tab is compacted and made visually consistent with the other tabs' density and layout idiom.
- [ ] **WISHUI-02**: Wishlist suggestions include sub-Velious (Kunark / classic) gear tiers, not only Velious tiers. *(New wiki source pages + tier values; `wiki_gear_tier` already exists — no migration.)*

---

## Future Requirements (deferred — not this milestone)
- Search facets beyond Clicky/Haste — by worn slot, by stat threshold (+STR, +AC), by weapon type (1H/2H/piercing). The parsed stat backbone (ENRICH-12/13) makes each an incremental add later.
- Epic-quest-specific tracking (which epic an item feeds, per class, with steps/turn-ins) — needs a new quest-step data model. v2.6 ships generic named quest links (ITEMUI-02) instead.

## Out of Scope (with reasoning)
- **Inventory history / per-item time-series** — still parked; not in the Core Value, adds real complexity.
- **Daybreak EQ UI chrome bitmaps** for the portrait/aesthetic — P99-community license caution; recreate the vibe with original styling + P99-wiki-licensed assets.
- **Watcher changes** — v2.6 is backend + web only; ingest already captures everything needed. No `v*` tag.

## Traceability
*(REQ-ID → Phase — filled by the roadmapper 2026-06-24.)*

| REQ-ID | Phase |
|--------|-------|
| ENRICH-12 | Phase 37 |
| ENRICH-13 | Phase 37 |
| ENRICH-14 | Phase 38 |
| ENRICH-15 | Phase 38 |
| SEARCH-04 | Phase 39 |
| SEARCH-05 | Phase 39 |
| SEARCH-06 | Phase 39 |
| ITEMUI-01 | Phase 40 |
| ITEMUI-02 | Phase 40 |
| CHARUI-01 | Phase 41 |
| CHARUI-02 | Phase 41 |
| WISHUI-01 | Phase 42 |
| WISHUI-02 | Phase 42 |
