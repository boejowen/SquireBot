# Phase 42: Wishlist polish — compaction + sub-Velious tiers - Research

**Researched:** 2026-07-16
**Domain:** P1999 MediaWiki gear-tier crawl extension + Go compute/store tier-ordering + Svelte density polish
**Confidence:** HIGH (load-bearing question verified against the LIVE P1999 wiki API)

---

## VERDICT — the load-bearing WISHUI-02 question

**The same-format sub-Velious gear pages EXIST and parse UNCHANGED. No new parser branch. No migration. The milestone's "just add source pages + tier constants" assumption is CONFIRMED against the live wiki.**

### Exact page titles (verified via `action=parse&prop=wikitext`, 2026-07-16)

| Page title | Wikitext len | `== [[Class]] ==` hdrs | `'''Slot'''` labels | `{{:Item}}` transcl | Parses? |
|------------|-------------:|------------------------:|---------------------:|---------------------:|:-------:|
| `Players:Kunark Gear`      | 22,964 | 14 | 241 | 594 | ✅ [VERIFIED] |
| `Players:Planar Gear`      | 18,528 | 14 | 239 | 429 | ✅ [VERIFIED] |
| `Players:Pre Planar Gear`  | 24,834 | 14 | 224 | 690 | ✅ [VERIFIED] |
| `Players:Velious Pre-Raid Gear` (existing) | 24,286 | 14 | — | — | ✅ (shipped) |
| `Players:Velious Raiding Gear` (existing)  | 25,507 | 14 | — | — | ✅ (shipped) |

Every page returns 14 `== [[ClassName]] ==` headers matching `CLASS_DISPLAY_TO_ABBREV` (Bard…Wizard), `'''Ears''' / '''Fingers''' / '''Neck'''…` bold slot labels matching `WIKI_SLOT_TO_INV_SLOTS`, and `{{:ItemName}}` transclusions — the exact three structural markers `wikigear.go` depends on. `[VERIFIED: live wiki.project1999.com/api.php action=parse]`

Representative `<li>` from `Players:Kunark Gear`, Cleric section (byte-identical to the Velious format):
```
'''Ears'''      - {{:Forest Loop}}, {{:Ivandyr's Hoop}}, {{:Golden Black Sapphire Earring}}, {{:Truewind Earring}}, {{:Yunnb's Earring}}
'''Fingers'''   - {{:Moonstone Ring}}, {{:Band of Eternal Flame}}, {{:Golem Tear Ring}}
```

### The wiki's own page family (from the index page `Players:Gear` + search)

The P1999 progression ladder is a documented, self-referential set. Snippets from the search:
- `Players:Pre Planar Gear` → "references [[Players:Planar Gear]] (46+) and [[Players:Kunark Gear]] (51+)" `[CITED: wiki search snippet]`
- `Players:Planar Gear` → "Old World raiding gear… for higher end gear, check [[Players:Kunark Gear]] (51+)" `[CITED]`
- `Players:Kunark Gear` → "Gear suggestions for each class for Kunark raiding (51+)" `[CITED]`

So the natural low→high ladder the wiki itself describes is:
**Pre-Planar (classic, leveling/group) → Planar (Old World raiding, 46+) → Kunark (Kunark raiding, 51+) → Velious Pre-Raid/Group → Velious Raiding.**

### Recommended tier constants + ladder rank (D-03)

Add three constants to `wikigear.go` alongside the existing three. Use human-legible labels (they render verbatim in the badge unless the UI maps them):

| New constant | String value | Source page | Base tier for `wikiVeliousGearSources` |
|--------------|--------------|-------------|-----------------------------------------|
| `TierClassic` | `"Classic/Pre-Planar"` | `Players:Pre Planar Gear` | `TierClassic` |
| `TierPlanar`  | `"Planar/Old-World Raid"` | `Players:Planar Gear` | `TierPlanar` |
| `TierKunark`  | `"Kunark"` | `Players:Kunark Gear` | `TierKunark` |

**Ladder RANK (low→high) — the explicit order D-03 requires, replacing the alphabetic `ORDER BY wgt.tier`:**

| Rank | Tier string | Notes |
|-----:|-------------|-------|
| 1 | `Classic/Pre-Planar` | leveling / group, cheapest |
| 2 | `Iksar` | racial minor tier (already emitted only from the Velious Pre-Raid page — see Iksar note) |
| 3 | `Planar/Old-World Raid` | Old World raid |
| 4 | `Kunark` | Kunark raid (51+) |
| 5 | `Velious Pre-Raid/Group` | existing `TierVeliousPreRaid` |
| 6 | `Velious Raiding` | existing `TierVeliousRaiding` |

> **On the CONTEXT wording "Kunark → (Iksar at the Kunark level)":** the wiki does NOT put Iksar racial items on the Kunark page as a distinct tier — the Iksar tier is a parser artifact that fires ONLY on the Velious Pre-Raid page (`baseTier == TierVeliousPreRaid && HasPrefix("Iksar ")`, `wikigear.go:88`). Iksar-prefixed items DO appear on the new pages (2 on Kunark, verified) but under their base tier, NOT re-tagged. That is the current, correct behavior — do NOT extend the Iksar re-tag to the new pages (it would fragment the ladder). Iksar sits low in the rank (it's early-game racial gear), hence rank 2 above; the planner can also drop it to rank 1.5 or leave it — its exact position is Claude's-discretion per CONTEXT.

**Naming choice is planner discretion within D-02.** CONTEXT asked for badges reading `[Classic] / [Kunark] / [Pre-Raid] / [Raid]`. If the planner wants those short badge labels, the cleanest path is: store the full tier string in `wiki_gear_tier.tier` (as above) and map tier-string → short-badge-label + rank in ONE Go table (see "Ordering mechanism" below) — a single source of truth for both the badge text and the sort key.

### Does the existing parser handle the new pages UNCHANGED? — YES

`ParseGearTierPage(wikitext, baseTier)` is fully page-structure-agnostic. It:
1. splits on `== [[Class]] ==` (all 3 new pages have exactly 14, all real classes),
2. pulls `<li>` blocks (all 3 have 224–242, one per slot-line),
3. reads `'''Slot'''` + `{{:Item}}` from each `<li>` (identical format).

The ONLY tier-specific logic in the parser is the Iksar re-tag gated on `TierVeliousPreRaid` — which correctly no-ops for the new base tiers. **Passing `TierClassic`/`TierPlanar`/`TierKunark` as `baseTier` produces correct rows with zero parser edits.** `[VERIFIED: parser source review + live wikitext structural match]`

### Fallback (if any page had failed to parse) — NOT NEEDED

All three parse. But the machinery already handles a bad page gracefully: `runWikiGearTier` fetches every `wikiVeliousGearSources` entry, and if ANY page errors/empties/parse-fails it sets `allFresh = false` and SKIPS the full-table replace (leaving working rows intact — `wiki.go:521-528,555-559`). So a future page-title change or wiki edit that breaks one page will NOT wipe the table; it just leaves the last-good rows. **Caveat (see Pitfall 1): adding pages to this all-or-nothing set means ANY one of the now-5 pages failing skips the ENTIRE replace — including the working Velious rows.** That is the single most important operational risk of this phase.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01 (compaction target):** Density polish — tighten the per-slot accordion gap (24→16), slot padding (16→12), target/suggestion row heights (44→40, still a safe touch target), and the add-control gap (16→12), matching the Characters/Inventory density idiom; **keep the two-pane master-detail structure**. Web-only, low risk. NOT a fuller restructure, NOT minimal.
- **D-02 (scope):** **Kunark + classic both**, sourced from the P1999 wiki gear-tier pages that follow the same section-header/bold-slot/transclusion MediaWiki format the existing parser expects. Add tier constants + append source entries to `wikiVeliousGearSources`. **No migration** (`wiki_gear_tier.tier` is generic TEXT); **no new parser** if the pages match the format. A page that does NOT parse must be skipped gracefully (never break the Velious crawl).
- **D-03 (tier labels + ordering):** Expose the tier as a badge + order the per-slot list as an upgrade LADDER low→high. (1) add `tier` (string) to `WishlistSuggestion` (`compute/types.go` + `web/src/lib/api.ts`) and render a small tier badge on each suggestion row; (2) order the per-slot suggestions by an **explicit tier RANK** low→high — Classic → Kunark → (Iksar at the Kunark level) → Velious Pre-Raid → Velious Raiding — NOT the current alphabetic `ORDER BY wgt.tier`. No migration → rank via Go-side tier-order map or SQL `CASE`, NOT a new column.
- **D-04 (raid tagging for sub-Velious):** sub-Velious **raid / no-drop** items get the same **Raid / not-for-sale** treatment as Velious Raiding. Extend `IsRaid = (tier == TierVeliousRaiding)` (`compute/wishlist.go:200`) to the sub-Velious raiding tiers (mechanism Claude's discretion — a "raiding tier" name-set, OR drive it off the P37 `item_master.is_no_drop` flag). Keep the existing Velious behavior byte-for-byte.
- **D-05 (population + deploy):** new tiers appear only after a wiki gear-tier crawl. After deploy, **trigger `run-job wiki` on the box** to populate immediately (else next Sunday UTC). The gear-tier job full-replaces on success → a partial/failed new-page fetch must not wipe the working Velious rows (verify per-page failure isolation).

### Claude's Discretion
- The Iksar tier's exact ladder position; the tier-rank mechanism (Go sort vs SQL `CASE`); the exact compaction CSS values (within D-01); the tier-badge visual (small pill vs eyebrow tag, theme tokens — no literal hex); whether `is_raid` stays tier-name-based or moves to `is_no_drop`.

### Deferred Ideas (OUT OF SCOPE)
- A per-tier collapse/filter (hide Classic once you're in Velious).
- Non-gear suggestion sources (spells, tradeskill items).
- An epic/quest-specific tier.
- A wishlist architecture rework.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WISHUI-01 | Wishlist tab compacted, visually consistent with sibling tabs' density | Low research need — CSS density polish within the shared two-pane grid (`web/src/routes/wishlist/+page.svelte`), mirroring the CHARUI-01 idiom already shipped on Characters/Inventory. UI-SPEC concern; no new architecture. |
| WISHUI-02 | Wishlist suggestions include sub-Velious (Kunark / classic) gear tiers | FULLY RESOLVED above: 3 real same-format pages (`Players:Kunark Gear`, `Players:Planar Gear`, `Players:Pre Planar Gear`) parse unchanged → new tier constants + `wikiVeliousGearSources` entries + `WishlistSuggestion.tier` field + explicit rank order + extended raid-tag. No migration, no new parser. |
</phase_requirements>

## Summary

Phase 42 is a low-risk extension of a fully-understood subsystem plus a CSS density pass. The load-bearing uncertainty — do parseable Kunark/classic wiki pages exist — is now **resolved YES**: three same-format pages (`Players:Kunark Gear`, `Players:Planar Gear`, `Players:Pre Planar Gear`) each carry 14 `== [[Class]] ==` sections with `'''Slot'''` + `{{:Item}}` transclusions, byte-compatible with the existing `wikigear.go` parser. WISHUI-02 collapses to: add 3 tier constants, append 3 rows to `wikiVeliousGearSources`, add a `tier` field to `WishlistSuggestion`, replace the alphabetic sort with an explicit low→high rank, extend the raid-tag, render a badge. No migration, no new parser, watcher untouched.

The two genuine engineering decisions are (1) WHERE the tier-rank lives (a Go tier-order map is strongly preferred over a SQL `CASE` — one source of truth for rank + badge label, table-testable, no SQL churn) and (2) HOW the raid tag generalizes (a small "raiding-tier" name-set is preferred over the `is_no_drop` route — the gear-tier row has no item_id to join `item_master` on, so `is_no_drop` is not directly reachable from `GearTierPriceRow` without new plumbing). The single real operational hazard is Pitfall 1: the gear crawl is all-or-nothing across ALL source pages, so adding 3 pages triples the surface where one bad page skips the entire replace (Velious included).

**Primary recommendation:** Add `TierClassic`/`TierPlanar`/`TierKunark` constants + 3 `wikiVeliousGearSources` entries; carry `tier` through `GearTierPriceRow` → `WishlistSuggestion`; implement rank + badge-label + raid-membership in ONE Go `map[enrich.Tier]tierMeta{rank, badge, isRaid}` table in `compute`; sort suggestions per-slot by that rank; keep `ORDER BY wgt.tier,class,slot,rank` in SQL only as a stable tiebreak (final order is Go-side). Deploy → `run-job wiki`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Sub-Velious tier source pages | API / Backend (`enrich/jobs/wiki.go`) | — | The wiki crawl is a backend job; source-page list + tier constants live in `enrich`. |
| Parse gear-tier wikitext | API / Backend (`enrich/wikigear.go`) | — | Pure parser, reused UNCHANGED. |
| Tier rank + badge-label + raid-membership | API / Backend (`compute`) | — | Business logic; belongs in the pure compute transform, table-testable. |
| Expose `tier` on the suggestion | API / Backend (`compute/types.go`) → Web (`api.ts` mirror) | — | Additive contract field, JSON-serialized. |
| Tier badge render + ladder order display | Web (`wishlist/+page.svelte`) | — | Client render of the already-ordered, already-labeled suggestion list. |
| Wishlist density compaction | Web (`wishlist/+page.svelte`) | — | Pure CSS; no data flow change. |

## Standard Stack

No new libraries. This phase extends existing code only.

| Component | Where | Change |
|-----------|-------|--------|
| `enrich.Tier` constants | `internal/backendsrv/enrich/wikigear.go:24-28` | +3 constants (`TierClassic`, `TierPlanar`, `TierKunark`) |
| `wikiVeliousGearSources` | `internal/backendsrv/enrich/jobs/wiki.go:83-89` | +3 `{pageTitle, tier}` entries |
| `WishlistSuggestion` | `internal/backendsrv/compute/types.go:339-347` | +`Tier string \`json:"tier"\`` field |
| `WishlistSuggestion` mirror | `web/src/lib/api.ts:913-919` (`tier` field) | +`tier: string` |
| tier-rank / badge / raid map | `internal/backendsrv/compute/wishlist.go` (new package var) | new `map[enrich.Tier]tierMeta` |
| suggestion sort | `compute/wishlist.go` buildWishlistView | sort per-slot suggestions by rank (Go-side) |
| badge render + density | `web/src/routes/wishlist/+page.svelte` | badge chip + CSS token values |

**Version verification:** N/A — no package installs. `[VERIFIED: no new deps required — codebase review]`

## Architecture Patterns

### Data flow (WISHUI-02)

```
[weekly wiki_weekly job / manual run-job wiki]
   └─> runWikiGearTier: for src in wikiVeliousGearSources (now 5 pages)
          ├─ fetchWikiPageUnconditional(pageTitle)   ← Kunark/Planar/Pre-Planar added here
          ├─ ParseGearTierPage(wikitext, baseTier)    ← UNCHANGED parser, new baseTier passed
          └─ accumulate rows (tier=baseTier string)
       if ALL pages fresh → replaceGearTier(all rows)  ← ALL-OR-NOTHING (Pitfall 1)
   ▼
wiki_gear_tier table  (tier TEXT — now 5 distinct tier strings)
   ▼
store.GearTierPrices  ── SELECT … ORDER BY wgt.tier,class,slot,rank (stable tiebreak only)
   ▼
compute.buildWishlistView
   ├─ filter rows by class+slot (unchanged)
   ├─ map each row.Tier → {rank, badge, isRaid} via tierMeta table   ← NEW
   ├─ IsRaid = tierMeta[row.Tier].isRaid   (D-04, generalizes the Velious-only check)
   ├─ sug.Tier = tierMeta[row.Tier].badge  (D-03)
   └─ sort ws.Suggestions by tierMeta[...].rank ASC (low→high ladder)   ← NEW
   ▼
WishlistSuggestion{ItemName, Tier, IsRaid, Price, LastListed} → api.ts → +page.svelte badge
```

### Pattern 1: Go-side tier metadata table (RECOMMENDED for the D-03 rank + D-04 raid tag)

**What:** One package-level `map[enrich.Tier]tierMeta` in `compute` holding `{rank int, badge string, isRaid bool}` per tier. This is the single source of truth for the ladder order, the badge label, AND the raid tag.
**When to use:** Whenever more than one derived property keys off the tier string — which is exactly this phase (rank + badge + isRaid).
**Why over a SQL `CASE`:** (a) the badge label and the isRaid flag are already Go-side (`compute`), so co-locating rank there keeps ONE table instead of splitting rank into SQL and badge/raid into Go; (b) it is directly table-testable in the pure `buildWishlistView` unit tests (the established D-7 discipline) with no DB; (c) it avoids touching the SQL `ORDER BY` at all (leave `readviews.go:557` as a stable class/slot/rank tiebreak — the FINAL order becomes Go-side).
**Example (illustrative — planner authors the exact form):**
```go
// compute/wishlist.go
type tierMeta struct {
    rank   int
    badge  string
    isRaid bool
}
var tierMetaByTier = map[enrich.Tier]tierMeta{
    enrich.TierClassic:         {rank: 1, badge: "Classic",  isRaid: false},
    enrich.TierIksar:           {rank: 2, badge: "Iksar",    isRaid: false},
    enrich.TierPlanar:          {rank: 3, badge: "Planar",   isRaid: true},
    enrich.TierKunark:          {rank: 4, badge: "Kunark",   isRaid: true},
    enrich.TierVeliousPreRaid:  {rank: 5, badge: "Pre-Raid", isRaid: false},
    enrich.TierVeliousRaiding:  {rank: 6, badge: "Raid",     isRaid: true},
}
```
> **Decision the planner must lock:** are Planar and Kunark "raid" tiers for D-04? The wiki labels both as *raiding* gear ("Kunark raiding (51+)", "Old World raiding"). Marking them `isRaid: true` matches the wiki's own framing and gives them the not-for-sale treatment CONTEXT asked for. Classic/Pre-Planar and Iksar are group/leveling → `isRaid: false`. This preserves the existing Velious behavior byte-for-byte (Velious Pre-Raid stays false, Velious Raiding stays true).

### Pattern 2: sort suggestions per-slot AFTER the filter loop

`buildWishlistView` appends suggestions in `tiers` iteration order (currently DB order). After the per-slot append loop, `sort.SliceStable(ws.Suggestions, ...)` by `tierMeta[...].rank`, then existing rank within tier. Keep it stable so same-tier items retain their wiki `rank`.

### Anti-Patterns to Avoid
- **Extending the Iksar re-tag to the new pages.** The `baseTier == TierVeliousPreRaid` gate in `wikigear.go:88` is intentional; leave it. Iksar-prefixed items on Kunark/Planar/Pre-Planar correctly stay under their base tier.
- **A SQL `CASE` for rank while badge+raid stay in Go.** Splits one concept across two layers. Put it all in the Go tierMeta table.
- **Driving `is_raid` off `item_master.is_no_drop` in THIS phase.** Tempting (P37 added the flag) but the gear-tier row carries NO item_id (`WikiGearTierRow.ItemID` is always nil, `GearTierPrices` joins pigparse by NAME only) — there is no existing join from a gear-tier suggestion to `item_master.is_no_drop`. Wiring that up is new plumbing beyond D-04's "keep Velious byte-for-byte" and risks changing Velious behavior. Use the tier-name-set (tierMeta.isRaid). `is_no_drop` remains a documented future refinement.
- **Per-character view tabs / new routes.** Not relevant here (density polish on the existing tab).
- **Literal hex in the badge.** Theme tokens only (`var(--…)`) per CLAUDE.md + CONTEXT.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fetch/parse the new gear pages | A new fetch+parse path | Append to `wikiVeliousGearSources`; reuse `ParseGearTierPage` | Verified byte-compatible; the loop already iterates the source list. |
| Per-page failure isolation | A new guard | The existing `allFresh` all-or-nothing replace | Already prevents partial wipes; just be aware it now spans 5 pages. |
| Name→price join for new-tier items | Anything | `GearTierPrices` pp_rep name-join (unchanged) | New rows flow through the same NULL-id name join. |

**Key insight:** WISHUI-02 is genuinely additive — the pipeline was built to take N source pages with N tier labels. The only NEW code is the tierMeta table + the sort + the contract field + the badge.

## Runtime State Inventory

> Rename/refactor phase? NO — this is additive feature work. But the wiki crawl mutates a live prod table, so the population step matters (D-05).

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `wiki_gear_tier` prod table currently holds ONLY Velious rows (2 tiers). After deploy it holds 5 tiers ONLY after a crawl runs. | Run `run-job wiki` on the box post-deploy (D-05) — else new tiers land next Sunday UTC. |
| Live service config | None — no external service config embeds these tier strings. | None — verified (tiers are internal TEXT values). |
| OS-registered state | The weekly `wiki_weekly` scheduled job (`scheduler.go:145-150`) already runs the crawl; no re-registration needed. | None — the new pages ride the existing schedule. |
| Secrets/env vars | None. | None. |
| Build artifacts | None — Go rebuild + web rebuild via the normal deploy; no egg-info/binary rename. | Standard backend binary-swap + web atomic swap. |

**Deploy hazard:** the crawl's all-or-nothing replace means the FIRST `run-job wiki` after deploy either populates all 5 tiers or (if any page fails) leaves the OLD 2-tier Velious set. That is safe (no data loss) but means "new tiers didn't appear" == "a page failed" — check the job log (`gear_replaced=false` + a per-page "gear page unavailable" warning).

## Common Pitfalls

### Pitfall 1: The gear crawl is ALL-OR-NOTHING across every source page
**What goes wrong:** `runWikiGearTier` sets `allFresh = false` if ANY page in `wikiVeliousGearSources` fails to fetch/parse, and then SKIPS the full-table replace entirely — including the working Velious rows. Adding 3 pages means the replace now depends on ALL 5 succeeding in a single run.
**Why it happens:** the full-table-replace strategy (the UNIQUE is NULL-poisoned, so it's DELETE-all + INSERT, `wiki.go:568-581`) needs the COMPLETE combined set or it would wipe rows it didn't re-fetch.
**How to avoid:** This is by design and CORRECT (no partial wipe). But (a) verify the 3 new page titles are EXACT (`Players:Kunark Gear`, `Players:Planar Gear`, `Players:Pre Planar Gear` — with the space, no trailing "Guide") — a typo makes an error envelope, `allFresh=false`, and NO tier ever updates; (b) the job log's `gear_replaced=%t gear_rows=%d` (`wiki.go:150-153`) is the health signal — `gear_replaced=true` with ~5× the old row count confirms success.
**Warning signs:** post-deploy `run-job wiki` shows `gear_replaced=false` or a "gear page unavailable; skipping full replace" warn line → a page title is wrong or the wiki changed a page.

### Pitfall 2: Unlabeled tiers sort as noise (the exact problem D-03 names)
**What goes wrong:** `GearTierPrices` currently `ORDER BY wgt.tier` alphabetically. Drop in "Classic/Pre-Planar" and "Kunark" and they sort BEFORE "Velious…" alphabetically anyway — but that's coincidental and fragile (rename a tier and the order breaks), and the badge is needed for legibility regardless.
**How to avoid:** Do the final ordering Go-side by explicit rank (Pattern 1); treat the SQL `ORDER BY` as a stable tiebreak only. Always render the badge so the ladder is legible even if two tiers tie.

### Pitfall 3: Slot-vocabulary mismatch would silently empty suggestions
**What goes wrong:** `buildWishlistView` filters `row.Slot != wiki` where `wiki = wikiSlotFor(canonical)`. If the new pages used slot labels NOT in `WIKI_SLOT_TO_INV_SLOTS`, those rows would parse but never match a paperdoll slot → invisible.
**Status:** VERIFIED SAFE — the new pages use the SAME labels (`Ears`, `Fingers`, `Neck`, `Head`, `Face`, `Chest`, `Arms`, `Back`, `Waist`, `Shoulders`, `Wrists`, `Legs`, …) already in the map. `[VERIFIED: live wikitext '''Slot''' extraction]`

### Pitfall 4: Iksar re-tag confusion
**What goes wrong:** Assuming CONTEXT's "(Iksar at the Kunark level)" means the Kunark page carries an Iksar tier. It does not — Iksar re-tag fires only on the Velious Pre-Raid page. Iksar-prefixed items DO exist on Kunark (2 verified) but stay under `TierKunark`.
**How to avoid:** Leave `wikigear.go:88` untouched. Place `TierIksar` in the rank table at rank ~2 (low, racial). Don't add Iksar handling to the new pages.

## Code Examples

### Appending the source pages (D-02)
```go
// internal/backendsrv/enrich/jobs/wiki.go — wikiVeliousGearSources
// (rename the var if desired — it's no longer Velious-only)
var wikiVeliousGearSources = []struct {
    pageTitle string
    tier      enrich.Tier
}{
    {"Players:Pre Planar Gear", enrich.TierClassic},     // NEW — classic/leveling
    {"Players:Planar Gear",     enrich.TierPlanar},      // NEW — Old World raid
    {"Players:Kunark Gear",     enrich.TierKunark},      // NEW — Kunark raid (51+)
    {"Players:Velious Pre-Raid Gear", enrich.TierVeliousPreRaid}, // existing
    {"Players:Velious Raiding Gear",  enrich.TierVeliousRaiding}, // existing
}
// Source: verified titles from live wiki.project1999.com/api.php action=parse (2026-07-16)
```

### The tier constants (D-02)
```go
// internal/backendsrv/enrich/wikigear.go
const (
    TierClassic        Tier = "Classic/Pre-Planar"     // NEW
    TierPlanar         Tier = "Planar/Old-World Raid"  // NEW
    TierKunark         Tier = "Kunark"                  // NEW
    TierVeliousPreRaid Tier = "Velious Pre-Raid/Group"  // existing
    TierVeliousRaiding Tier = "Velious Raiding"         // existing
    TierIksar          Tier = "Iksar"                   // existing (Velious Pre-Raid page only)
)
```

### Extended raid tag + tier field in buildWishlistView (D-03/D-04)
```go
// compute/wishlist.go — inside the per-slot suggestion loop, replacing lines ~198-206
meta := tierMetaByTier[enrich.Tier(row.Tier)] // zero-value = rank 0, "", false is acceptable for an unknown tier
sug := WishlistSuggestion{
    ItemName:   row.ItemName,
    Tier:       meta.badge,   // D-03: the badge label
    IsRaid:     meta.isRaid,  // D-04: generalizes the old (tier == TierVeliousRaiding) check
    LastListed: row.LastListed,
}
// … price block unchanged …
ws.Suggestions = append(ws.Suggestions, sug)
// after the loop, before append to out.Slots:
sort.SliceStable(ws.Suggestions, func(i, j int) bool {
    return tierMetaByTier[/* tier of i */].rank < tierMetaByTier[/* tier of j */].rank
})
```
> Note: `sug` currently drops `row.Tier` after computing `IsRaid`. To sort AFTER the loop the planner should either keep a parallel tier slice, sort BEFORE mapping to badge, or add an unexported `tier` field to a local struct. Cleanest: sort the `tiers` slice contribution per-slot before building `sug`, or carry rank on `WishlistSuggestion` transiently. Planner's call.

### WishlistSuggestion contract (D-03)
```go
// compute/types.go
type WishlistSuggestion struct {
    ItemName   string   `json:"item_name"`
    Tier       string   `json:"tier"`      // NEW — badge label ("Classic"/"Kunark"/"Pre-Raid"/"Raid"/…)
    IsRaid     bool     `json:"is_raid"`
    Price      *float64 `json:"price"`
    LastListed string   `json:"last_listed"`
    WikiURL    string   `json:"wiki_url"`
}
```
```typescript
// web/src/lib/api.ts — mirror
export interface WishlistSuggestion {
  item_name: string;
  tier: string;      // NEW
  is_raid: boolean;
  price: number | null;
  last_listed: string;
  wiki_url: string;
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Velious-only 2-page gear crawl | 5-page crawl (adds Classic/Planar/Kunark) | This phase | 3 new tiers, same pipeline |
| `IsRaid = tier == TierVeliousRaiding` | `IsRaid = tierMeta[tier].isRaid` | This phase (D-04) | generalizes; Velious behavior preserved |
| `ORDER BY wgt.tier` (alphabetic) drives suggestion order | Go-side explicit rank ladder | This phase (D-03) | legible low→high upgrade path |
| No tier on the suggestion | `tier` badge field | This phase (D-03) | ladder is visible in the UI |

**Deprecated/outdated:** nothing removed — purely additive. The var name `wikiVeliousGearSources` becomes a slight misnomer (now includes non-Velious pages); rename is optional cleanup.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Planar + Kunark should be tagged `isRaid: true` (not-for-sale) per the wiki's "raiding gear" framing | Pattern 1 / D-04 | Low — if the user wants them purchasable, flip two booleans in the tierMeta table. A pure UX call, trivially reversible. |
| A2 | Badge labels "Classic/Kunark/Pre-Raid/Raid" are the desired short strings | VERDICT / D-03 | Low — badge text is one column in the Go table; rename freely. CONTEXT suggested these exact labels. |
| A3 | Iksar sits low on the ladder (rank ~2) | VERDICT / Pitfall 4 | Low — explicitly Claude's discretion per CONTEXT; any rank is a one-line change. |

**All three page titles, their parseability, the slot vocabulary, the `<li>` structure, and the Iksar distribution are VERIFIED against the live wiki (not assumed).** The only assumptions are UX-labeling/ordering choices, all trivially reversible in the single Go table.

## Open Questions

1. **Rename `wikiVeliousGearSources`?**
   - What we know: it now holds non-Velious pages.
   - Recommendation: optional. Rename to `wikiGearSources` for clarity, or leave with a comment. Not load-bearing.
2. **Iksar rank exact position (1, 2, or between Classic and Planar)?**
   - Recommendation: rank 2 (racial early gear, above Classic-generic but below raid tiers). Claude's discretion per CONTEXT. Any choice is a one-line edit.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| P1999 wiki API (`wiki.project1999.com/api.php`) | the gear crawl (WISHUI-02) | ✓ | live | — (the crawl already depends on it weekly) |
| `run-job wiki` CLI on the prod box | D-05 population | ✓ (existing) | — | wait for Sunday UTC weekly job |
| Go toolchain / node+vite | build | assumed present (standard deploy) | — | user installs (toolchain-install rule) |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none blocking — if `run-job wiki` isn't triggered, the weekly Sunday job populates the tiers.

## Validation Architecture

> `.planning/config.json` not re-read in this session; the project has an established Go+vitest discipline (D-7 pure-parser tests, web vitest node-only). Treating validation as ENABLED.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (backend) + vitest (web, node-only — DOM-blind) |
| Config file | Go: none (std); web: `web/vitest.config.*` |
| Quick run command | `go test ./internal/backendsrv/...` |
| Full suite command | `go test ./...` + `cd web && npm test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WISHUI-02 | `ParseGearTierPage` parses a Kunark/Planar/Pre-Planar fixture into correct (tier,class,slot,item) rows | unit | `go test ./internal/backendsrv/enrich/ -run GearTier` | ✅ existing test file; add a new-page fixture |
| WISHUI-02 | `buildWishlistView` orders suggestions by tier rank low→high + sets Tier badge + IsRaid | unit (pure) | `go test ./internal/backendsrv/compute/ -run Wishlist` | ✅ existing; extend with multi-tier rows |
| WISHUI-02 | `wikiVeliousGearSources` includes the 3 new pages with correct tiers | unit | `go test ./internal/backendsrv/enrich/jobs/` | ⚠ verify a test asserts the source list, else add |
| WISHUI-01 | density compaction renders | manual/browser | deploy-then-smoke | ❌ vitest is DOM-blind — browser-smoke gate |
| WISHUI-02 | badge renders + ladder order in the UI | manual/browser | deploy-then-smoke | ❌ browser-smoke gate |

### Sampling Rate
- **Per task commit:** `go test ./internal/backendsrv/...`
- **Per wave merge:** `go test ./...` + `cd web && npm test`
- **Phase gate:** full suite green + browser-smoke (badge + order + density) on prod after `run-job wiki`.

### Wave 0 Gaps
- [ ] A gear-tier fixture from one new page (e.g. a trimmed `Players:Kunark Gear` wikitext) under `enrich/testdata/` — the existing 2 fixtures are Velious-only. Capture from the live JSON already saved this session, or synthesize a minimal `== [[Cleric]] ==` / `'''Ears''' - {{:X}}` block.
- [ ] A `buildWishlistView` test asserting cross-tier ordering (Classic before Kunark before Velious) + badge + IsRaid for Planar/Kunark.
- [ ] (optional) an assertion that `wikiVeliousGearSources` has 5 entries with the exact titles.

## Sources

### Primary (HIGH confidence)
- LIVE `https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=…` — fetched `Players:Kunark Gear`, `Players:Planar Gear`, `Players:Pre Planar Gear` (+ re-verified `Players:Velious Pre-Raid Gear`); counted class headers / slot labels / transclusions / `<li>` blocks; inspected Cleric-section `<li>` bodies; counted Iksar-prefixed transclusions. (2026-07-16)
- LIVE `…action=query&list=search&srsearch=Gear` — discovered the `Players:*Gear` page family + the wiki's own ladder cross-references.
- Codebase: `enrich/wikigear.go`, `enrich/jobs/wiki.go`, `enrich/eqconst.go`, `enrich/jobs/urls.go`, `compute/wishlist.go`, `compute/types.go`, `store/readviews.go:536-595`.

### Secondary (MEDIUM confidence)
- CONTEXT.md D-01..D-05 + REQUIREMENTS.md WISHUI-01/02 (locked scope).

### Tertiary (LOW confidence)
- None — the load-bearing claim is fully tool-verified.

## Metadata

**Confidence breakdown:**
- Sub-Velious pages exist + parse unchanged: **HIGH** — verified against live wiki with structural counts + `<li>` inspection.
- Tier constants + ranks + raid mapping: **HIGH** (mechanism) / MEDIUM (exact labels — a UX call, trivially reversible).
- Ordering mechanism (Go map > SQL CASE): **HIGH** — grounded in the existing pure-compute test discipline + the NULL-id gear-tier row's inability to reach `is_no_drop`.
- Compaction (WISHUI-01): **MEDIUM** — CSS density values are a UI-SPEC concern; low research need, mirrors shipped CHARUI-01.

**Research date:** 2026-07-16
**Valid until:** ~30 days for the page structure (P1999 wiki pages are stable; a page-title change or format rewrite would invalidate — the all-or-nothing crawl fails safe if that happens).
