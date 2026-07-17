# Phase 42: Wishlist polish — compaction + sub-Velious tiers - Context

**Gathered:** 2026-07-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Compact the Wishlist tab to match the other tabs' density **and** extend the per-character per-slot upgrade suggestions to sub-Velious (Kunark / classic) gear tiers.

- **WISHUI-01** — compact `web/src/routes/wishlist/+page.svelte` for density/visual consistency with the Characters/Inventory tabs (it already uses the same two-pane master-detail grid; this is density polish, not a rebuild).
- **WISHUI-02** — extend the gear-tier suggestions beyond Velious to Kunark + classic tiers. `wiki_gear_tier` already exists (generic TEXT `tier` column) and the wiki parser is page-structure-agnostic → **NO migration, NO new parser** (given the pages follow the same MediaWiki format); add source-page entries + tier constants + expose the tier to the UI.

**Scope:** backend (`internal/backendsrv/enrich` — new wiki source pages + tier constants; `compute`/`store` — tier ordering + the exposed tier field) + web (`web/` — compaction + the tier badge). **Watcher UNTOUCHED → no `v*` tag.** Out of scope: any migration, a wishlist architecture swap, an epic/quest tier, non-gear suggestion sources.

</domain>

<decisions>
## Implementation Decisions

### WISHUI-01 — Wishlist compaction
- **D-01 (compaction target):** **Density polish** — tighten the per-slot accordion gap (24→16), slot padding (16→12), target/suggestion row heights (44→40, still a safe touch target), and the add-control gap (16→12), matching the Characters/Inventory density idiom; **keep the two-pane master-detail structure**. Web-only, low risk (mirrors the CHARUI-01 "tighten toward the sibling-tab feel" approach). NOT a fuller restructure, NOT minimal.

### WISHUI-02 — Sub-Velious gear tiers
- **D-02 (scope):** **Kunark + classic both**, sourced from the P1999 wiki gear-tier pages that follow the same section-header/bold-slot/transclusion MediaWiki format the existing parser expects (`enrich/wikigear.go`). Add tier constants (`TierKunark`, `TierClassic`, and any raiding split the researcher finds) + append source entries to `wikiVeliousGearSources` (`enrich/jobs/wiki.go:83-89`). **No migration** (`wiki_gear_tier.tier` is generic TEXT); **no new parser** if the pages match the format. The exact page titles are a RESEARCH item (see below) — a page that does NOT parse must be skipped gracefully (never break the Velious crawl).
- **D-03 (tier labels + ordering — the load-bearing WISHUI-02 UX):** **Expose the tier as a badge + order the per-slot list as an upgrade LADDER low→high.** Today `WishlistSuggestion` carries only an `is_raid` bool and NO tier label, and the suggestions sort by *alphabetic tier string* — so dropping in "Classic"/"Kunark" unlabeled would sort them first and read as noise. Fix: (1) add `tier` (string) to `WishlistSuggestion` (`compute/types.go` + `web/src/lib/api.ts`) and render a small tier badge on each suggestion row (`[Classic]` / `[Kunark]` / `[Pre-Raid]` / `[Raid]`); (2) order the per-slot suggestions by an **explicit tier RANK** low→high — **Classic → Kunark → (Iksar at the Kunark level) → Velious Pre-Raid → Velious Raiding** — NOT the current alphabetic `ORDER BY wgt.tier`. Since there is **no migration**, implement the rank via a Go-side tier-order map or a SQL `CASE` (planner's discretion), NOT a new column.
- **D-04 (raid tagging for sub-Velious):** sub-Velious **raid / no-drop** items get the same **Raid / not-for-sale** treatment as Velious Raiding. Today `IsRaid = (tier == TierVeliousRaiding)` (`compute/wishlist.go:200`); extend it to the sub-Velious raiding tiers the researcher identifies (mechanism is Claude's discretion — a "raiding tier" name-set, or drive it off the P37 `item_master.is_no_drop` flag now that flags exist). Keep the existing Velious behavior byte-for-byte.
- **D-05 (population + deploy):** the new tiers only appear after a wiki gear-tier crawl. After the deploy, **trigger `run-job wiki` on the box to populate the new tiers immediately** (else they land on the next Sunday UTC weekly job). The gear-tier job full-replaces the table on success, so a partial/failed new-page fetch must not wipe the working Velious rows (verify the per-page failure isolation).

### Claude's Discretion
- The Iksar tier's exact ladder position (it sits at ~the Kunark level; an existing minor tier); the tier-rank mechanism (Go sort vs SQL `CASE`); the exact compaction CSS values (within D-01); the tier-badge visual (small pill vs an eyebrow tag, sourced from theme tokens — no literal hex); whether `is_raid` stays tier-name-based or moves to the `is_no_drop` flag.

### Research needed (flag for the gsd-phase-researcher — do NOT skip research for this phase)
- **The exact P1999 wiki page title(s) for Kunark + classic gear tiers**, and whether each follows the same MediaWiki **section-header (`== [[Class]] ==`) + bold-slot (`'''Slot'''`) + template-transclusion (`{{:Item}}`)** structure the existing `wikigear.go` parser needs. This is the load-bearing WISHUI-02 uncertainty: the milestone assumes "just add source pages," but the pages must EXIST in the parseable format. If a tier's page uses a different structure, the researcher flags it (a new parser branch, or drop that tier). Confirm via the P1999 wiki API (`action=parse&prop=wikitext`), the same source the Velious pages use.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope + requirements
- `.planning/ROADMAP.md` § "Phase 42" (the checklist line + the v2.6 milestone locked-decisions block — "sub-Velious tiers INCLUDED; no migration").
- `.planning/REQUIREMENTS.md` — WISHUI-01 (compaction) + WISHUI-02 (sub-Velious tiers: "new wiki source pages + tier values; `wiki_gear_tier` already exists — no migration").
- `.planning/phases/34-wishlist-rework-per-character-per-slot-upgrades/` (the v2.4 Phase 34 that built the Velious wishlist — the pattern this extends).

### WISHUI-01 — wishlist layout (the compaction target)
- `web/src/routes/wishlist/+page.svelte` — the two-pane grid (`:885+`), the per-slot accordion (`.accordion` gap `:1131`, `.slot` padding `:1137`), the suggestion rows (`:784-826`, `:1242`), the add control (`:1348`). The sibling density idiom: `web/src/routes/characters/+page.svelte:313` + `web/src/routes/inventory/+page.svelte` (same `minmax(280px,360px) 1fr` grid, 44px rows).

### WISHUI-02 — the gear-tier pipeline (add Kunark/classic)
- `internal/backendsrv/migrations/00001_init.sql:62` — the `wiki_gear_tier` schema (generic TEXT `tier`; NO migration this phase).
- `internal/backendsrv/enrich/wikigear.go:22-28` — the tier constants (`TierVeliousPreRaid`/`TierVeliousRaiding`/`TierIksar`) to extend; `:58-160` the page-structure-agnostic parser.
- `internal/backendsrv/enrich/jobs/wiki.go:83-89` — `wikiVeliousGearSources` (THE source-page list to append Kunark/classic entries to); `:485-566` the fetch + full-table-replace (verify per-page failure isolation, D-05).
- `internal/backendsrv/scheduler/scheduler.go:145-150` — the weekly `wiki_weekly` job (Sunday UTC) that runs the gear-tier crawl; the `run-job wiki` CLI trigger (D-05).
- `internal/backendsrv/compute/wishlist.go:81-116,193-207` — `buildWishlistView` filters tiers by class+slot → `WishlistSuggestion` (`IsRaid` at `:200`); the suggestion emit + the tier-order source.
- `internal/backendsrv/store/readviews.go:536-595` — `GearTierPrices` (`ORDER BY wgt.tier,class,slot,rank` at `:557` — the alphabetic order D-03 replaces with an explicit tier rank).
- `internal/backendsrv/compute/types.go:339-347` + `web/src/lib/api.ts:913-919` — the `WishlistSuggestion` type (gains a `tier` field, D-03).
- External: `https://wiki.project1999.com/api.php?action=parse&prop=wikitext` — the wiki source the researcher probes for the Kunark/classic pages.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- The **Velious gear-tier crawl** (`wikigear.go` parser + `wiki.go` `wikiVeliousGearSources` + the scheduler) is reused verbatim — WISHUI-02 appends source entries + tier constants, no new pipeline.
- The **wishlist suggestion path** (`compute/wishlist.go` → `WishlistSuggestion` → `api.ts` → the suggestion rows) is reused — WISHUI-02 adds a `tier` field + the badge; the tiers flow through the existing filter.
- The **two-pane master-detail grid** is shared across Characters/Inventory/Wishlist — WISHUI-01 tightens the wishlist's spacing to the sibling density, no structural change.
- **P37's `item_master.is_no_drop`** now exists — a candidate signal for the D-04 Raid/not-for-sale tag (vs the current tier-name match).

### Established Patterns
- Extend-only: new tier strings + source pages + an additive `tier` JSON field; no migration (`wiki_gear_tier` + the suggestion contract already exist).
- The weekly full-table-replace gear-tier crawl (per-page fetch → parse → replace) — new pages ride it; a failed page must not wipe the Velious rows.
- Theme-token styling only (the tier badge uses `var(--…)` tokens, no literal hex — the one sanctioned non-token color is the per-item tile gradient, not relevant here).

### Integration Points
- **Backend:** `enrich/wikigear.go` (tier constants) + `enrich/jobs/wiki.go` (source list) → `store/readviews.go` (tier-rank order) → `compute/wishlist.go` + `compute/types.go` (the `tier` field + extended `IsRaid`).
- **Web:** `api.ts` (`WishlistSuggestion.tier`) → `wishlist/+page.svelte` (the tier badge on suggestion rows + the density compaction).
- **Tests:** Go parser/compute tests for the new tiers + the rank order; web vitest for the badge/order logic (pure) → the density + badge RENDER is a **browser-smoke checkpoint** (deploy-then-smoke, the established discipline). Expect a `/gsd-ui-phase 42` gate before planning (UI: yes). **Deploy needs a `run-job wiki` to populate the new tiers (D-05).**

</code_context>

<specifics>
## Specific Ideas

- The tier badge makes the ladder legible: a per-slot suggestion list reads **Classic → Kunark → Velious Pre-Raid → Velious Raiding**, each tagged, so a user sees the upgrade path bottom-to-top.
- WISHUI-02 is "just add source pages + tier values" ONLY if the Kunark/classic wiki pages match the parser's format — that assumption is the phase's research gate, not a given.
- Reuse P37's `is_no_drop` for the Raid/not-for-sale tag is an option worth weighing (more accurate than a tier-name match), but keep the Velious behavior identical.

</specifics>

<deferred>
## Deferred Ideas

- A per-tier collapse/filter (hide Classic once you're in Velious) — a nice UX follow-up, not this phase.
- Non-gear suggestion sources (spells, tradeskill items) — out of the gear-tier scope.
- An epic/quest-specific tier — explicitly out of scope (v2.6 ships generic tiers).
- A wishlist architecture rework — WISHUI-01 is density polish only; the two-pane pattern stays.

</deferred>

---

*Phase: 42-wishlist-polish-compaction-sub-velious-tiers*
*Context gathered: 2026-07-16*
