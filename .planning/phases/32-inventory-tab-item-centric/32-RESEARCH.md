# Phase 32: Inventory Tab (Item-Centric) - Research

**Researched:** 2026-06-18
**Domain:** Go backend item-rollup (compute-on-read) + session-gated read route + SvelteKit master-detail web tab over Phase 29 data, reusing the Phase 31 examine + `?c=` seam.
**Confidence:** HIGH (the entire surface is built over already-shipped, code-read internals — no external/unverified dependencies; the one genuinely-new piece is a pure compute grouping over two existing reads).

## Summary

Phase 32 is almost entirely a *composition* phase: every data input already exists and was read in this
research. The guild-wide instance rows come from `compute.View(ctx, store) → []ViewRow` (Phase 29); the
per-holder slot label comes from the SAME `classifySlot` taxonomy Phase 31 uses; the viewer-first /
bank/bot holder banding comes from `store.RosterFor(ctx, viewerDiscordID) → []RosterRow` (Phase 31); the
examine block is the unmodified `ExaminePanel.svelte` (Phase 31); the holder deep-link target is the live
`/characters?c=<name>` selection seam (Phase 31). The list/detail shape, every token, copy string, and
state are pinned in the APPROVED `32-UI-SPEC.md`.

The ONE new backend piece is a pure compute function that groups `View`'s rows by **normalized name**
(`lower(trim(name))` — never `item_id`; the EQ-inventory vs PigParse-catalog id-namespace split makes
`item_id` an invalid join key — `[VERIFIED: store/readviews.go:39-47 + memory pigparse-vs-ingame-item-id-namespaces]`),
sums stack `Count`, counts distinct holders, derives a per-holder slot label, and stamps each item's
representative price/wiki onto the row. It needs the per-char flags (`is_mine`/`is_bank_toon`/`is_guild_bot`)
that `View`/`InventoryJoin` do NOT carry — so the function ALSO reads `RosterFor` and joins by char name to
build a char→flags map. It is exposed as a NEW session-gated route (`GET /api/v1/items`), registered beside
the Phase 31 routes in `main.go`. **No new migration** — it is pure compute over existing tables
(`icon_id`/`statsblock` already shipped in P31's 00012/00013).

**Primary recommendation:** Add `compute.Items(ctx, *store.Store, viewerDiscordID string) ([]ItemRollup, error)`
that fans `View` rows + `RosterFor` flags into one-row-per-normalized-name rollups (summed qty, distinct
holder count, `is_mine`, name-keyed price/wiki/icon/statsblock, and a `holders[]` with `{char, slot_label,
qty, last_synced, is_mine, is_bank}`); expose it at `GET /api/v1/items` under `RequireSession`; build a
bespoke selectable item list (mirroring `/characters`) whose detail is `ExaminePanel` (charLastSeen="") +
a holders table whose rows deep-link to `/characters?c=<name>`; extract a pure node-tested `web/src/lib/items.ts`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** ONE row per normalized item NAME, counting every copy held anywhere (equipped + general +
  bag contents + bank) across every character, bank toon, and guild bot. Nothing hidden (no
  exclude-equipped, no junk filter). Identity = **normalized name**, NOT `item_id`.
- **D-02:** Default order = **viewer's items first, then A-Z**. Viewer = the Discord session; "the
  viewer's items" = items held on their `character_assignment` characters (v2.3).
- **D-03:** Detail = the reused P31 `ExaminePanel` (wiki stats / PigParse price / wiki link / last-synced,
  D-08 order, single escaped `composeItemNote` `{@html}` sink) **PLUS** a holders table (one row per
  holding: character · slot · quantity · last-synced). **Clicking a holder deep-links to
  `/characters?c=<name>`** (the P31 selection seam).
- **D-03a:** Single pinned panel, replace-on-click (same as P31 D-06/D-07). Not a multi-pin board.
- **D-04:** Each list row shows **summed stack count + holder count** (e.g. "142 · 3 holders") + inline
  PigParse price (the existing `pickPrice` buy/sell selection) + inline wiki link — scannable without
  opening the detail.
- Coin/platinum rows are NOT items (excluded). Sum stack `Count`s across bag-vs-loose copies of same name.
- Compute-on-read; extend-only schema; `?`-bound SQL; never string-concat names into SQL; session-gated
  route (`RequireSession`, login-gated NOT officer-only); slog carries counts/status, never raw item/char
  values.

### Claude's Discretion (planner/researcher owns these — RESOLVED below)
- **The item-rollup backend shape** — resolved: a NEW `compute.Items(...)` function + a NEW `GET /api/v1/items`
  route. (Do NOT reuse `GET /api/v1/items/search` — that is the P19 wantlist search over the PigParse CATALOG.)
- **Slot-label granularity in the holders table** — resolved (see Backend Item-Rollup §slot-label).
- **Holders-table ordering** — resolved: viewer's own characters first (consistent with D-02).
- **DataGrid vs purpose-built list** — resolved: **bespoke list** (mirror `/characters`); ExaminePanel reused directly.
- **Quantity edge cases** — coin/platinum excluded; sum all bag-vs-loose copies of same name.
- Exact list/detail split layout, mobile reflow, row density, icon sizing → already pinned in `32-UI-SPEC.md`.

### Deferred Ideas (OUT OF SCOPE)
- **Banks tab** (banks-only list + guild-wide PigParse value + total platinum) — Phase 33 (BANK-01..03);
  fed by Phase 29 `BankValuationFor`/`TotalPlatinum`.
- **Per-character/per-slot Wishlist rework** — Phase 34 (WISH-01..07).
- **Sort/filter controls** beyond the locked viewer-first/A-Z + name search — revisit only if needed.
- **Multi-pin item compare board** — rejected for the single replace-on-click panel (D-03a).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ITEM-01 | List all guild items with name, guild-wide quantity, a wiki link, and a PigParse price that links to PigParse (when applicable). | `compute.Items` groups `View` rows by normalized name → `summed_qty` + name-keyed `price`/`wiki_url`/`prices`; `GET /api/v1/items` serves them; bespoke list row renders icon + name + `{qty} · {N} holders` + inline price (`pickPrice`) + inline wiki link (UI-SPEC §B/§Color). Coin/platinum excluded by construction (View already excludes `item_id <= 0`; coin is not an inventory_item row — file format carries no coin). |
| ITEM-02 | Per-item name search that prioritizes items on the viewer's characters. | `is_mine` is computed in the rollup by joining each item's holders against `RosterFor`'s `is_mine` flag (the `character_assignment` LEFT JOIN on the viewer's discord id). A pure `items.ts` `viewerFirstItems()` + `filterItems()` (mirroring `roster.ts`) sorts mine-first-then-A-Z and preserves that order among filtered matches. |
| ITEM-03 | Selecting an item shows which characters hold it, the slot on each, the quantity, and the last-synced day/time (master-detail, consistent with INV-02). | Each rollup carries `holders[] = {char, slot_label, qty, last_synced, is_mine, is_bank}`. `slot_label` derives from `classifySlot` (P29). `last_synced` = `View`'s per-row `LastSynced` (= `character.last_seen`). Detail = `ExaminePanel` (charLastSeen="") + a holders table; holder rows `goto('/characters?c=' + encodeURIComponent(char))` (the live P31 seam — zero P31 change). |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Group all holdings by normalized name → guild-wide qty + holder count + per-holder rows | API / Backend (`compute.Items`) | Database (`View`'s `InventoryJoin` + `RosterFor` reads) | Aggregation over the full guild dataset is a server concern; the client receives a bounded, already-rolled-up payload. Compute-on-read (no materialized table). |
| Name-keyed price / wiki / icon resolution | Database/Storage (the `pp_rep` CTE + `item_master` id-join, already in `InventoryJoin`) | API (`pickPrice` selection in compute) | The cross-namespace name join is already solved in the store CTE (commit 0a169f3); compute reuses it, never re-implements. |
| Viewer identity (`is_mine`) | API / Backend (session → `webauth.UserFromContext` → `RosterFor`'s `?` bind) | — | Identity is server-truth from the Discord session cookie; never trusted from the client. |
| Viewer-first ordering + name search ranking | Frontend (pure `items.ts`) | API (the rollup carries `is_mine` so the client need not recompute assignment) | Presentation/ranking is client-side and bounded; mirrors the shipped `roster.ts` precedent. NOT access control (every member sees every item). |
| Master-detail selection + holder deep-link | Browser / Client (SvelteKit `goto` + `history.replaceState`) | — | Selection/pin is a client render (relaxed consolidated-views rule — one reusable detail, not N routes). |
| Examine render (escaped wiki `{@html}`) | Browser / Client (`ExaminePanel` reused unchanged) | — | The single sanctioned escaped-HTML sink already exists and is reused verbatim. |

## Standard Stack

This phase adds **no new dependency** (backend or web). It composes already-shipped internals.

### Core (all already in the tree — verified by code read)
| Library / Module | Version | Purpose | Why Standard |
|------------------|---------|---------|--------------|
| Go std `net/http` `ServeMux` | Go 1.24 | New `GET /api/v1/items` route + `RequireSession` wrap | The shipped route idiom (`main.go:286-363`) — every read route uses it. |
| `internal/backendsrv/compute` | in-repo | New `Items(...)` pure-grouping function (the `View`/`buildViewRows` split pattern) | Compute-on-read layer; authors ZERO SQL; consumes typed store structs. |
| `internal/backendsrv/store` (`View` inputs + `RosterFor`) | in-repo | The two existing reads the rollup composes | `InventoryJoin` (via `View`) + `RosterFor` are the tested SQL paths; the rollup is pure Go over them. |
| `internal/backendsrv/readapi` | in-repo | New `Items` handler (the `characters.go` twin) | Versioned, session-gated read-API pattern; `characters.go` is the closest analog (it also reads the viewer id from context). |
| SvelteKit 5 (runes) + Tailwind v4 + `@lucide/svelte` | as shipped | The web tab | The established web stack across 30+ phases; `@lucide/svelte` `Search`/`ExternalLink` already in use. |
| `web/src/lib/components/ExaminePanel.svelte` | in-repo (P31) | The item detail's examine block (reused UNCHANGED, charLastSeen="") | The single escaped `composeItemNote` `{@html}` sink; D-08 order; node-tested via `examine.ts`. |
| `web/src/lib/components/StateBlock.svelte` | in-repo | loading / error / empty / no-results states | The shared state presentation; reuse verbatim (UI-SPEC §H). |

### Supporting (new files this phase creates — all mirroring an existing analog)
| File | Purpose | Mirrors |
|------|---------|---------|
| `web/src/lib/items.ts` | Pure `viewerFirstItems()` + `filterItems()` + the holders-sort | `web/src/lib/roster.ts` (exact pattern) |
| `web/src/lib/__tests__/items.test.ts` | Node tests for the above | `web/src/lib/__tests__/roster.test.ts` (14 cases) |
| `fetchItems()` in `web/src/lib/api.ts` | Credentialed `GET /api/v1/items` wrapper | `fetchCharacters()` (api.ts:289-293) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| A NEW `compute.Items` reading both `View` + `RosterFor` | Extend `InventoryJoin` SQL to carry `is_mine`/`is_bank_toon`/`is_guild_bot` per row | Touching the shared `InventoryJoin` query risks regressing `View`/`Bank`/`gear_check`. The compose-in-Go approach keeps the tested SQL untouched and the banding logic node/table-testable. **Recommended: compose in Go.** |
| A bespoke selectable item list | Reuse `DataGrid.svelte` | DataGrid has no native "selected row → render detail" affordance and carries the guild-views grid chrome (sticky Char, facets, Heavy parchment). The bespoke list reads as the master pane and matches `/characters` 1:1. **Recommended: bespoke** (UI-SPEC §B confirms). |
| `ExaminePanel` reused directly with `charLastSeen=""` | A thin item-examine wrapper | The item examine's "last synced" is a PER-HOLDER fact (it lives in the holders table), so passing `charLastSeen=""` correctly omits the footer (D-09 omits empty `charLastSeen`) — zero `ExaminePanel` change needed. **Recommended: reuse directly.** |

**Installation:** none — no `npm install`, no `go get`. `[VERIFIED: .planning/config.json + code read — every needed module is in-repo]`

## Architecture Patterns

### System Architecture Diagram

```
                          GET /api/v1/items   (session cookie)
  Browser /inventory  ───────────────────────────────────────►  readapi.ItemsHandler
  (SvelteKit)                                                    (RequireSession wrap)
      │  fetchItems()                                                   │
      │  (credentials:'include')                                        │ viewer id =
      │                                                                 │ webauth.UserFromContext(ctx)
      │                                                                 ▼
      │                                              compute.Items(ctx, store, viewerDiscordID)
      │                                                     │
      │                                    ┌────────────────┴───────────────────┐
      │                                    ▼                                     ▼
      │                          compute.View(ctx, store)              store.RosterFor(ctx, viewerID)
      │                          → []ViewRow                           → []RosterRow
      │                          (per-instance holdings:               (per-char flags:
      │                           Char, Slot/Location, Item,            is_mine, is_bank_toon,
      │                           ID, Count, Price, Prices,             is_guild_bot — joined by
      │                           WikiURL, LastSynced, ...)             char NAME into a flags map)
      │                                    │                                     │
      │                                    └──────────────┬──────────────────────┘
      │                                                   ▼
      │                           group ViewRows by lower(trim(Item)) →
      │                             per normalized name:
      │                               summed_qty = Σ Count
      │                               holder_count = distinct chars
      │                               is_mine = any holder flagged is_mine
      │                               price/wiki/icon/statsblock = representative
      │                               holders[] = {char, slot_label(=classifySlot label),
      │                                            qty, last_synced(=LastSynced), is_mine, is_bank}
      │                                                   │
      │  []ItemRollup (JSON, snake_case) ◄────────────────┘
      ▼
  items.ts: viewerFirstItems() + filterItems(query)   ──►  bespoke selectable list (left pane)
                                                              │ row click → select(name)
                                                              ▼
                                                    detail (right pane):
                                                      ExaminePanel(slot=asSlot, charLastSeen="")
                                                      + holders table
                                                          │ holder row click
                                                          ▼
                                                    goto('/characters?c='+enc(char))
                                                    (live P31 selection seam — opens that
                                                     character's inventory window)
```

### Recommended Project Structure (files touched/created)
```
internal/backendsrv/
├── compute/
│   ├── itemrollup.go        # NEW: Items(...) + buildItemRollups(...) pure transform + ItemRollup/ItemHolder structs
│   └── itemrollup_test.go   # NEW: table tests over seeded View rows + roster flags (the view_test.go fixture pattern)
├── readapi/
│   └── items.go             # NEW: ItemsHandler (the characters.go twin — reads viewer id from ctx, RequireSession)
└── (cmd/squirebot-server/main.go)  # MODIFY: register GET /api/v1/items beside /api/v1/characters (line ~363)

web/src/
├── lib/
│   ├── api.ts               # MODIFY: add ItemRollup + ItemHolder interfaces + fetchItems()
│   ├── items.ts             # NEW: pure viewerFirstItems()/filterItems()/sortHolders() (roster.ts twin)
│   └── __tests__/items.test.ts  # NEW: node tests (roster.test.ts twin)
└── routes/inventory/+page.svelte # MODIFY: replace the P30 placeholder with the master-detail tab
```

### Pattern 1: The public-fn → pure-helper compute split (MIRROR `view.go`)
**What:** A public `Items(ctx, *store.Store, viewerID)` fetches via the store, then delegates to a pure
`buildItemRollups(viewRows []ViewRow, roster []RosterRow) []ItemRollup` that takes typed slices and returns
the model with no ctx/store inside — directly table-testable.
**When to use:** Always in `compute/` — it is the package's iron law (`compute` authors zero SQL).
```go
// Source: internal/backendsrv/compute/view.go:47-82 (View / buildViewRows) — the exact pattern to mirror.
func Items(ctx context.Context, s *store.Store, viewerDiscordID string) ([]ItemRollup, error) {
    viewRows, err := s.... // via compute.View(ctx, s) OR s.InventoryJoin(ctx, false) directly
    if err != nil { return nil, err }
    roster, err := s.RosterFor(ctx, viewerDiscordID)
    if err != nil { return nil, err }
    return buildItemRollups(viewRows, roster), nil
}
```
> **Decision for the planner:** prefer composing `compute.View(ctx, s)` (which already calls `pickPrice`
> + carries `WikiURL`/`Prices`/`IsQuestItem`/`LastSynced`) over re-reading `InventoryJoin` directly — it
> reuses the price selection and the inline enrichment for free. BUT note `View`'s `ViewRow` does **not**
> carry `icon_id` or `statsblock` (those are only on the per-char `InventoryRow`/`InventorySlot` path). If
> the examine reuse needs `statsblock` + `icon_id` per item (it does — see §Examine reuse seam), the rollup
> must ALSO read them. Two clean options: (a) add `icon_id`/`statsblock` to the `InventoryJoin` SELECT +
> `ViewRow` (extend-only, append at the right edge — but it touches the shared `View`/`Bank` query), or
> (b) read `item_master` icon_id/statsblock separately (a small id-keyed map) and stamp it onto the rollup
> by the representative item's `ID`. **Recommendation: option (b)** — a tiny new `store` read
> (`SELECT item_id, icon_id, statsblock FROM item_master`) keyed into a map, joined in compute by the
> representative `ViewRow.ID`, leaving `InventoryJoin`/`View` untouched. Confirm in planning.

### Pattern 2: Per-holder slot label from `classifySlot` (REUSE P29)
**What:** Each holder's slot label is derived from the holding's raw `Location` (`ViewRow.Slot`) via the
SAME `compute.classifySlot(location) (SlotCategory, canonicalSlot)` Phase 31 uses. Render as the UI-SPEC §F
form: `Worn · {Slot}` (equipment), `General · Slot {N}` (general), `Bank` / `Bank · Slot {N}` (bank),
`Bag · {…}` for a bagged copy.
**When to use:** Building the `holders[]` rows.
```go
// Source: internal/backendsrv/compute/inventory.go:60-88 (classifySlot) — already exported within the package.
cat, canonical := classifySlot(vr.Slot) // vr.Slot is the raw Location, e.g. "General4-Slot1", "Chest", "Bank2"
// map (cat, canonical, isChild) → a display label; isChild detectable via splitChild(vr.Slot).
```
> **Slot-label granularity (Claude's-discretion, resolved per UI-SPEC §F):** one row per holding; the slot
> label carries the equipped/bagged/bank distinction (no grouping required). A `*-Slot<N>` child Location
> (a bagged copy) labels as `Bag` / `Bag · Slot {N}` (the parent bag's display name is not on the ViewRow —
> `classifySlot` only yields the category; "Bag · {bag name}" from the sketch requires the parent row's
> name, which is NOT joined here — so the simplest correct label is `Bag · Slot {N}` or just `Bag`).
> Confirm the exact label strings in planning; the data sufficient for the simple form is present.

### Pattern 3: Viewer-first ordering — server stamps `is_mine`, client sorts (MIRROR `roster.ts`)
**What:** The rollup stamps `is_mine = any holder is on a viewer-assigned char`. The pure `items.ts`
sorts mine-first-then-A-Z and preserves that among search matches — identical to `roster.ts`
`viewerFirst`/`filterRoster`.
```ts
// Source: web/src/lib/roster.ts:36-51 — the exact two-function shape to mirror in items.ts.
export function viewerFirstItems(rows: ItemRollup[]): ItemRollup[] { /* is_mine ? 0 : 1, then localeCompare */ }
export function filterItems(rows: ItemRollup[], query: string): ItemRollup[] { /* filter by name, then viewerFirstItems */ }
```

### Pattern 4: Examine reuse seam — an `InventorySlot`-shaped object per item (UI-SPEC §C, load-bearing)
**What:** `ExaminePanel` takes `slot: InventorySlot | null` + `charLastSeen`. The rollup must expose a
representative `InventorySlot`-shaped object per item so the detail renders `<ExaminePanel
slot={asSlot} charLastSeen="" />` UNCHANGED. The frontend builds this from the rollup fields (name → `item`,
plus `icon_id`, `statsblock`, `wiki_summary`, `is_quest_item`, `price`, `prices`, `wiki_url`); the
list-context-irrelevant fields (`count`/`slots`/`children`/`canonical_slot`/`location`) may be zero/empty.
`charLastSeen=""` correctly OMITS the examine footer (last-synced is per-holder, in the table).

### Anti-Patterns to Avoid
- **Joining catalog↔inventory by raw `item_id`:** the canonical landmine of this domain (see Pitfalls).
  Group/join ONLY by normalized name.
- **Reusing `GET /api/v1/items/search`:** that is the P19 wantlist catalog search — NOT guild holdings.
- **Reusing `DataGrid` for the list:** it has no master-detail selection affordance and pulls the wrong chrome.
- **Adding a second `{@html}` sink:** the ONLY raw-HTML is `ExaminePanel`'s escaped `composeItemNote`. Item +
  character names render via plain `{}` (Svelte auto-escapes) everywhere else.
- **A new theme writer / re-skin:** the single `[data-theme]` writer stays on the shell root; reuse the 5 themes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Name-keyed price resolution across the EQ-inventory ↔ PigParse-catalog id split | A custom item_id→price lookup | The `pp_rep` CTE name-join already in `InventoryJoin` (reached via `compute.View`) + `pickPrice` | The id-join silently leaves ~91% of held rows unpriced (`[VERIFIED: store/readviews.go:39-47]`). The name bridge is solved. |
| Per-holder slot label parsing | A new `Location`→label parser | `compute.classifySlot` + `splitChild` (P29) | The taxonomy (equipment/general/bank + `-Slot<N>` nesting + paired-slot numbering + case-insensitive) is already battle-tested with smoke fixes. |
| Viewer "is_mine" determination | A client-side assignment lookup | `store.RosterFor`'s `character_assignment` LEFT JOIN (server-truth, `?`-bound) | Identity must be server-truth; the SQL is tested and IDOR-safe. |
| Examine rendering + escaped wiki link | A new examine component | `ExaminePanel.svelte` (charLastSeen="") | The single sanctioned `{@html}` sink, D-08 order, D-09 omission — all node-tested via `examine.ts`. |
| Loading/error/empty/no-results UI | New state markup/copy | `StateBlock.svelte` | Shared copy + a11y; reuse verbatim. |
| The colored-tile icon fallback | A new `<img onerror>` tile | The `PaperdollSlot` `.ico` mechanic (extract a shared tile or thin variant) | Keeps the list/detail icons and the paperdoll from drifting (UI-SPEC §Color recommends extraction). |

**Key insight:** Phase 32 introduces exactly ONE new algorithm — grouping instance rows into name-keyed
rollups with per-holder detail. Everything else is wiring already-shipped, code-read parts together. The
risk is not "can we build it" but "do we group by name (not id), gate the route, and avoid a route-name
collision" — all three are flagged in Pitfalls.

## Common Pitfalls

### Pitfall 1: Grouping/joining by `item_id` instead of normalized name
**What goes wrong:** Items split into duplicate rows or merge wrong; prices vanish for ~91% of holdings.
**Why it happens:** CLAUDE.md historically called `item_id` "the stable join key", but that is FALSE across
namespaces — the EQ `/outputfile` inventory ids and the PigParse catalog ids are different namespaces, and
gear-tier rows have NO id at all. `[VERIFIED: memory pigparse-vs-ingame-item-id-namespaces + store/readviews.go:39-47]`
**How to avoid:** Group `ViewRow`s by `lower(trim(Item))` in `buildItemRollups`. The price/wiki are already
name-bridged in the store (`pp_rep` CTE). The representative `ViewRow.ID` is used ONLY to look up
`icon_id`/`statsblock` from `item_master` (the watcher's own EQ namespace — id-correct there).
**Warning signs:** the same item appearing twice in the list; a known commodity (Blue Diamond) showing no price.

### Pitfall 2: Route-name collision with `GET /api/v1/items/search`
**What goes wrong:** Registering `GET /api/v1/items` is fine, but a planner might reflexively reuse
`itemsearch.go` or name the route `/items/search`-adjacent and collide.
**Why it happens:** `itemsearch.go` already owns `GET /api/v1/items/search` (P19 catalog search). `[VERIFIED:
main.go:351 + readapi/itemsearch.go]`
**How to avoid:** Use `GET /api/v1/items` (no `/search` suffix). Go 1.22+ `ServeMux` treats `/api/v1/items`
and `/api/v1/items/search` as distinct patterns — no shadowing — but DO NOT reuse `NewItemSearch`/`SearchCatalog`
(those scan the PigParse catalog, not guild holdings). Build a NEW `readapi.ItemsHandler` + `compute.Items`.
**Warning signs:** the list showing catalog items the guild doesn't hold; the list having no holder data.

### Pitfall 3: `ViewRow` lacks `icon_id`/`statsblock` (and `is_mine`/bank flags)
**What goes wrong:** Building the examine `InventorySlot` from `View` rows yields no icon and an empty stat
block; building viewer-first from `View` alone yields no `is_mine`.
**Why it happens:** `icon_id`/`statsblock` live on the per-char `InventoryRow`/`InventorySlot` path
(`InventoryForChar`), NOT on `InventoryJoin`/`ViewRow`. Per-char flags (`is_mine`/`is_bank_toon`/`is_guild_bot`)
live ONLY on `RosterFor`'s `RosterRow`, NOT on `ViewRow`. `[VERIFIED: store/readviews.go:48-64 vs 73-92 vs 656-666]`
**How to avoid:** The rollup reads `RosterFor` (for flags, joined by char name) AND a small `item_master`
icon_id/statsblock map (keyed by the representative `ViewRow.ID`). Do NOT widen the shared `InventoryJoin`
query unless the planner deliberately chooses option (a) in Pattern 1 (and accepts the `View`/`Bank` blast radius).
**Warning signs:** colored-tile fallback on EVERY item; examine showing only the name; nothing ranked viewer-first.

### Pitfall 4: DOM-blind node tests give false confidence
**What goes wrong:** `npm test` green, but the rendered tab is broken (the P15/P31 trap — number coercion,
epoch-sec dates, crashing components — all passed node tests).
**Why it happens:** No `@testing-library/svelte` (toolchain-install rule); node vitest is `environment:node`
and EXCLUDES `*.svelte.{test,spec}.ts`. `[VERIFIED: web/vite.config.ts + memory web-tests-node-only-blind-to-dom]`
**How to avoid:** Node-test ONLY the pure `items.ts`; the rendered tab MUST be browser-smoked on a DEPLOYED
build (see Verification Approach). `npm run dev` cannot auth against prod (cookie Domain=squirebot.quest +
apex-only CORS — `[VERIFIED: memory web-local-dev-cant-auth-against-prod]`).
**Warning signs:** "all tests pass" used as the verification verdict for a rendered surface.

### Pitfall 5: Coin/platinum leaking into the item list
**What goes wrong:** Plat/coin rows appearing as "items".
**Why it happens:** A naive aggregation over all inventory rows could include coin if coin were ever stored
as inventory_item rows.
**How to avoid:** `View` already filters `ii.item_id IS NOT NULL AND ii.item_id > 0` (empty slots excluded)
AND the P99 inventory file format carries NO coin at all (plat is a separate manual bank-coin entry —
`[VERIFIED: memory project_inventory_file_format + CLAUDE.md]`). So coin cannot be in `View` rows by
construction. No extra filter needed — but state this explicitly in the plan so a future reader doesn't add one.
**Warning signs:** a "Platinum" or coin-name row in the list.

### Pitfall 6: Hardcoding colors/fonts instead of theme tokens
**What goes wrong:** A row reads correctly under Velious but breaks under Heavy (parchment) or Minimalist.
**Why it happens:** Copying the sketch HTML (which uses helper vars `--slot`/`--bg-2`/`--text-dim` and a
Google Fonts link) verbatim.
**How to avoid:** Use ONLY the registry tokens (UI-SPEC §Color mapping table); the sketch helper vars MUST
map to real tokens; NO Google Fonts link (fonts are self-hosted via `@fontsource`). Spot-check Heavy
(`#f0d088` on `#1a0e05`) + Minimalist (`#b8915c`) at build.
**Warning signs:** literal hex in a `.svelte`; a `--radius`/`--text-dim` global reintroduced.

## Code Examples

### The new compute function signature + structs (the contract the planner pins)
```go
// Source pattern: internal/backendsrv/compute/view.go + types.go (append-only, snake_case).
// ItemRollup — one guild-wide item (grouped by normalized name). snake_case JSON tags.
type ItemRollup struct {
    Name        string         `json:"name"`          // the representative display name (first-seen casing)
    SummedQty   int64          `json:"summed_qty"`    // Σ Count across all holdings (D-01/D-04)
    HolderCount int64          `json:"holder_count"`  // distinct holding characters (D-04)
    IsMine      bool           `json:"is_mine"`        // any holder on a viewer-assigned char (D-02/ITEM-02)
    Price       *float64       `json:"price"`          // pickPrice; null when unpriced (D-04/D-09)
    Prices      []PriceDetail  `json:"prices"`         // raw WTS/WTB detail (examine)
    WikiURL     string         `json:"wiki_url"`
    WikiSummary string         `json:"wiki_summary"`
    IsQuestItem bool           `json:"is_quest_item"`
    IconID      int64          `json:"icon_id"`        // 0 → colored-tile fallback (D-02)
    Statsblock  string         `json:"statsblock"`     // "" → examine omits the stats line (D-09)
    Holders     []ItemHolder   `json:"holders"`        // one per holding (ITEM-03)
}
// ItemHolder — one holding of an item (ITEM-03 holders-table row).
type ItemHolder struct {
    Char       string `json:"char"`
    SlotLabel  string `json:"slot_label"`  // from classifySlot (P29)
    Qty        int64  `json:"qty"`
    LastSynced string `json:"last_synced"` // ViewRow.LastSynced (= character.last_seen)
    IsMine     bool   `json:"is_mine"`
    IsBank     bool   `json:"is_bank"`      // is_bank_toon || is_guild_bot (for the bank/bot tag + holder band)
}
func Items(ctx context.Context, s *store.Store, viewerDiscordID string) ([]ItemRollup, error)
func buildItemRollups(viewRows []ViewRow, roster []store.RosterRow, /* icon/stats map */) []ItemRollup
```

### The new route handler (the `characters.go` twin)
```go
// Source: internal/backendsrv/readapi/characters.go:64-103 — reads viewer id from the
// RequireSession-populated context; encodes [] not null on empty.
func (h *ItemsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    ctx := r.Context()
    uid, _ := webauth.UserFromContext(ctx) // "" → nothing flagged is_mine; still a valid list
    rows, err := compute.Items(ctx, h.store, uid)
    if err != nil { slog.Error("items read failed", "err", err); http.Error(w, "internal error", 500); return }
    out := rows; if out == nil { out = []compute.ItemRollup{} } // [] not null
    w.Header().Set("Content-Type", "application/json"); w.WriteHeader(200)
    _ = json.NewEncoder(w).Encode(out)
    slog.Info("items ok", "rows", len(out), "status", 200) // V7: count+status only, never item/char text
}
```

### The route registration (the exact site to edit)
```go
// Source: cmd/squirebot-server/main.go:362-363 — add the new line beside the Phase 31 routes.
mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
mux.Handle("GET /api/v1/characters", webauth.RequireSession(db, readapi.NewCharacters(st)))
mux.Handle("GET /api/v1/items", webauth.RequireSession(db, readapi.NewItems(st)))   // NEW (Phase 32 / ITEM-01..03)
```

### The holder deep-link (the live P31 seam — zero P31 change)
```svelte
<!-- Source: web/src/routes/characters/+page.svelte:88-92 — /characters pre-selects from ?c= on mount. -->
<!-- A holder row navigates to that char's window (an <a> or a goto()). -->
<a href={`/characters?c=${encodeURIComponent(h.char)}`}>{h.char}</a>
<!-- or: import { goto } from '$app/navigation'; onclick={() => goto('/characters?c=' + encodeURIComponent(h.char))} -->
```

### The new fetch wrapper (the `fetchCharacters` twin)
```ts
// Source: web/src/lib/api.ts:289-293 — credentialed getJSON; [] on empty; typed 401/403 the AuthGate re-routes.
export function fetchItems(f: typeof fetch = fetch): Promise<ItemRollup[]> {
    return getJSON<ItemRollup[]>('/api/v1/items', f);
}
```

## File-by-File Plan Inputs

### Backend (create)
| File | Action | Mirrors / Notes |
|------|--------|-----------------|
| `internal/backendsrv/compute/itemrollup.go` | CREATE | `view.go` (public-fn → pure-helper split). Holds `Items`, `buildItemRollups`, `ItemRollup`, `ItemHolder`. Groups by `lower(trim(Item))`; reuses `pickPrice`, `classifySlot`, `splitChild`. Reads `compute.View` (or `InventoryJoin`) + `store.RosterFor` + a small `item_master` icon/stats map. |
| `internal/backendsrv/compute/itemrollup_test.go` | CREATE | `view_test.go`/`fixtures_test.go` (seeded temp DB: a couple chars, one viewer-assigned, an item held by 2+ chars in different slots, an unpriced item, an icon/stats item). Asserts: group-by-name (not id), summed qty across bag+loose, distinct holder count, is_mine propagation, slot labels, coin/empty exclusion. |
| `internal/backendsrv/readapi/items.go` | CREATE | `characters.go` (reads viewer id from ctx; `[]` not null). New `ItemsHandler` + `NewItems(st)`. V5 (`?`-bound downstream) / V7 (count+status only) discipline. |
| `internal/backendsrv/store/*` | MAYBE CREATE | A tiny `ItemMasterIconStats(ctx) map[int64]{IconID,Statsblock}` read IF the planner chooses Pattern-1 option (b) (recommended). Pure SELECT, `?`-free (full-table), extend-only. |

### Backend (modify)
| File | Action | Notes |
|------|--------|-------|
| `cmd/squirebot-server/main.go` | MODIFY (~line 363) | Register `GET /api/v1/items` under `webauth.RequireSession` beside the P31 routes. ONE line. |

### Web (create)
| File | Action | Mirrors |
|------|--------|---------|
| `web/src/lib/items.ts` | CREATE | `roster.ts` — `viewerFirstItems()`, `filterItems()`, `sortHolders()` (viewer-chars-first band order). Pure, immutable, DOM-free. |
| `web/src/lib/__tests__/items.test.ts` | CREATE | `roster.test.ts` — node cases for the sort/filter/holder-sort. |

### Web (modify)
| File | Action | Notes |
|------|--------|-------|
| `web/src/lib/api.ts` | MODIFY | Add `ItemRollup` + `ItemHolder` interfaces (snake_case, mirroring the Go contract) + `fetchItems()`. Reuse the existing `PriceDetail` interface. |
| `web/src/routes/inventory/+page.svelte` | MODIFY (replace placeholder) | The master-detail tab: `onMount` one-shot `fetchItems()` (401/403 → AuthGate guard, else error StateBlock+Retry); scoped search (`Search items…` + "Items on your characters match first." hint) → `filterItems`; bespoke selectable list (mirror `/characters` `.row`, `?i=` URL reflect via `history.replaceState`); detail = `<ExaminePanel slot={asSlot} charLastSeen="" />` + holders table; holder rows deep-link `/characters?c=`. Theme tokens only; NO new `{@html}`. |
| `web/src/lib/components/PaperdollSlot.svelte` | OPTIONAL | UI-SPEC recommends extracting the `.ico` colored-tile mechanic into a shared tile so list/detail icons don't drift from the paperdoll. Planner's discretion. |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `item_id` as "the stable join key" (CLAUDE.md Architecture text) | Join/group catalog↔inventory by NORMALIZED NAME | 2026-06-06 (commit 0a169f3; the `pp_rep` CTE) | Item-centric rollup MUST group by name. Verified in `store/readviews.go`. |
| Per-character view tabs forbidden (Google-Sheets 200-tab limit) | Per-item master-detail drill-down ALLOWED (one reusable detail rendered on selection) | 2026-06-17 (consolidated-views RELAXED, CLAUDE.md updated) | The Inventory tab's single pinned detail panel is sanctioned. |
| Inventory read had no item-icon/statsblock | `item_master.icon_id` (00012) + `statsblock` (00013) shipped | Phase 31 (2026-06-18, schema v13) | The rollup can carry icon + examine stats with NO new migration. |

**Deprecated/outdated:**
- `GET /api/v1/items/search` (P19) for "guild holdings" — it is the PigParse CATALOG search; do NOT reuse.
- Trusting `ViewRow.ID` for price (it's the EQ-namespace inventory id) — use it ONLY for the id-correct
  `item_master` icon/stats lookup, never for price (price is name-bridged in the store).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The new route path is `GET /api/v1/items` (CONTEXT says "name TBD"; UI-SPEC §C agrees). | Backend route | Low — a different path is a one-line change; the only hard constraint is "not `/items/search`". Confirm in planning. |
| A2 | The `Bag · {bag name}` holder label cannot be fully formed from `ViewRow` alone (the parent bag's display name isn't joined); the simplest correct label is `Bag` / `Bag · Slot {N}`. | Slot-label (Pattern 2) | Low/cosmetic — if the user wants the bag's NAME, the rollup must additionally resolve the parent `Location`'s item name (a within-char lookup); flag in planning if the literal sketch label is required. |
| A3 | Composing `compute.View` (not re-reading `InventoryJoin`) is the cleaner rollup input, plus a small `item_master` icon/stats map (Pattern-1 option b). | Pattern 1 | Low — both options work; option (b) avoids touching the shared `View`/`Bank` query. The planner may pick (a). |
| A4 | Coin/platinum cannot appear in `View` rows (P99 file has no coin; `View` filters `item_id > 0`). | Pitfall 5 | Low — verified against the file-format memory + the `View` WHERE clause; no extra filter needed. |
| A5 | The list is "guild-scale, bounded" so client-side filtering (no server search param) is fine. | UI-SPEC §D | Low — ~12 guildies × ~10 chars; the rollup is small. If it ever grows, a server `?q=` is additive. |

## Open Questions

1. **Exact holder-row slot-label strings (and whether `Bag · {name}` is required).**
   - What we know: `classifySlot` yields (category, canonical) for every `Location`; `splitChild` detects `-Slot<N>` children.
   - What's unclear: whether the user wants the literal sketch labels (`Worn · Back`, `Bag · Large Lambent Bag`) — the bag's *name* needs the parent row's item name, not on the `ViewRow`.
   - Recommendation: ship `Worn · {Slot}` / `General · Slot {N}` / `Bank` / `Bag · Slot {N}`; add the bag-name resolution only if the user asks (it's a within-char parent lookup, additive).

2. **Rollup input: `compute.View` vs `InventoryJoin` directly + how icon/stats are sourced.**
   - What we know: `View` reuses `pickPrice` + inline enrichment but drops `icon_id`/`statsblock`; `InventoryJoin` is the underlying read.
   - Recommendation: Pattern-1 option (b) — `compute.View` + a small `item_master` icon/stats map keyed by the representative id. Confirm in planning (Plan-checker will sanity-check the chosen approach).

3. **Whether to extract the colored-tile icon into a shared component now (vs inline in the list).**
   - Recommendation: extract (UI-SPEC §Color recommendation) so the list/detail/paperdoll icons don't drift; low effort, planner's discretion.

## Environment Availability

> The phase is code/config-only over an already-running stack. The only "external" surface is the LIVE
> deployed API+web (for the browser-smoke), which is the established deploy path.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (`go build`/`go test`) | backend compute + route | ✓ (shipped P29–31) | 1.24 | — |
| Node + the web `npm` scripts (`check`/`test`/`build`) | web tab + pure helper | ✓ (shipped) | adapter-static | — |
| Live prod API+web (squirebot.quest / api.squirebot.quest) | browser-smoke of the rendered tab | ✓ (live since P11; P31 deployed 2026-06-18) | — | deploy-then-smoke OR a full local stack (local backend + `SQUIREBOT_COOKIE_INSECURE` + `PUBLIC_API_BASE` + seeded `sb_session`) per memory |
| SSH to the Hetzner box (for deploy) | deploying the new route + web bundle | ✓ (ssh-agent + id_ed25519) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** prod-auth for the smoke — use deploy-then-smoke (no migration this
phase, so it's a backend-binary + web atomic-swap, NOT a goose run; confirm in the plan whether the backend
binary even needs a restart — it does, to register the new route).

## Security Domain

> `security_enforcement: true`, ASVS level 1, block-on `high`. The Inventory tab is a READ-ONLY browse/
> select surface — no destructive actions, no writes.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | The route is `webauth.RequireSession` (login-only since P15); the viewer id is server-truth from the Discord session cookie (`webauth.UserFromContext`), never the request body. |
| V3 Session Management | yes | Session cookie Domain=squirebot.quest, cross-subdomain credentialed fetch; the shipped `getJSON` `credentials:'include'` + typed 401/403 → `AuthGate` re-route. No new session surface. |
| V4 Access Control | yes | Membership gate, NOT ownership/officer: every signed-in member sees every item + every holder (the consolidated-views model). The rollup is ORDERED viewer-first but NEVER SCOPED to the viewer (the `roster.ts` T-27-01 negative property — presentation is not access control). NEVER `RequireOfficer`. |
| V5 Input Validation | yes | The route takes NO user input server-side (no query params; the viewer id comes from the session, bound only as `RosterFor`'s single `?` placeholder). The client name search is a client-side filter — never hits SQL. If a future `?q=` is added, it MUST bind as `?` (the `itemsearch.go` len-guard + LIMIT precedent). |
| V6 Cryptography | no | No crypto in this phase (no secrets, no hashing). |

### Known Threat Patterns for {Go read API + SvelteKit render over guildie-controlled names}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via item/char names | Tampering | Names are NEVER concatenated into SQL — the rollup is pure Go over `View`/`RosterFor` rows; the only bind is the viewer id (`?` in `RosterFor`). Any future server search binds as `?`. |
| Reflected/stored XSS via guildie-controlled item or character names | Tampering / Elevation | Item + character names render via plain `{}` interpolation (Svelte auto-escapes) in the list rows, detail header, and holders table. The ONLY raw-HTML is `ExaminePanel`'s `composeItemNote` (escapeHtml + safeHttpUrl scheme allow-list) — reused unchanged. This phase adds NO new `{@html}` sink. |
| Info disclosure via logs | Info Disclosure | V7: slog carries op + row count + status + err ONLY — never an item name, char name, or holder content (the `characters.go`/`inventory.go` discipline). |
| IDOR / over-broad data exposure | Elevation of Privilege | The read is guild-wide BY DESIGN (membership gate). `is_mine` is computed from the session's own `character_assignment` rows — a viewer cannot forge another's "mine" set (no client-supplied viewer id). |
| DoS via the icon `<img>` source | Tampering | `icon_id` is a trusted INTEGER from the weekly wiki job (`Item_${int}.png`) — no guildie string in the path (the P31 T-31-15 control); a bad/empty id falls back to the colored tile. |

## Verification Approach

> `nyquist_validation: false` → no formal Validation Architecture section. Verification is the established
> three-tier path: Go tests + web node tests (pure helpers) + deploy-then-browser-smoke (rendered tab).

**ITEM-01 (guild-wide item list w/ qty, wiki link, PigParse price):**
- *Automated:* `go test ./internal/backendsrv/compute/...` — a table test seeding two chars holding the
  same item in different slots asserts ONE rollup row, `summed_qty` = Σ counts, `holder_count` = distinct
  chars, name-keyed price present for a priced item / null for an unpriced one, coin/empty-slot rows absent.
  `web vitest items.test.ts` asserts the list ordering/shape over fixture rollups.
- *Browser-smoke (deployed):* the list renders one row per item with `{qty} · {N} holders`, an inline
  PigParse price (and OMITS cleanly when no price), and a working "Wiki ↗" link; a known commodity shows a price.

**ITEM-02 (viewer-prioritized name search):**
- *Automated:* `items.test.ts` — `viewerFirstItems` floats `is_mine` rows first then A-Z; `filterItems`
  preserves that among matches; an empty query returns the full viewer-first set; a no-match returns [].
  `itemrollup_test.go` — `is_mine` is true on a rollup whose holder is a viewer-assigned char, false otherwise.
- *Browser-smoke (deployed):* at rest the viewer's items are on top; typing a name keeps the viewer's
  matches first; no-results shows the `StateBlock kind="no-results"` with the escaped query.

**ITEM-03 (master-detail: holders + slot + qty + last-synced; consistent with INV-02):**
- *Automated:* `itemrollup_test.go` — `holders[]` has one row per holding with the correct `slot_label`
  (from `classifySlot`), `qty`, `last_synced` (= the char's `last_seen`), and `is_mine`/`is_bank` flags;
  viewer's chars sort first (the `sortHolders` helper, node-tested in `items.test.ts`).
- *Browser-smoke (deployed):* selecting an item pins the detail; selecting another REPLACES it (D-03a); the
  examine block shows the D-08 order and omits missing fields (no "null"); the holders table lists
  character/slot/qty/last-synced with the viewer's chars first; **clicking a holder navigates to
  `/characters?c=<name>` and that character's window opens** (the load-bearing cross-tab interaction); a
  known iconId loads the wiki PNG and a bogus/empty iconId falls back to the colored tile; the whole tab
  renders under ALL 5 themes (Heavy + Minimalist contrast spot-check).

**Regression (every phase):** `go test ./...` all packages ok; `web npm run check` 0/0 + `npm test`
green (incl. the new `items.test.ts`) + `npm run build` ok (adapter-static).

**Deploy note:** NO migration this phase (icon_id/statsblock already shipped). The deploy is a backend
binary swap (to register the new `/api/v1/items` route — the server MUST restart) + a web atomic-swap.
Take the R2 backup per the established `docs/backend-deploy.md` path even without a migration.

## Sources

### Primary (HIGH confidence — code read this session)
- `internal/backendsrv/compute/view.go` — `View`/`buildViewRows`/`pickPrice` (rollup input + price selection).
- `internal/backendsrv/compute/inventory.go` — `classifySlot`/`splitChild`/`canonicalNumbered` (slot labels).
- `internal/backendsrv/compute/types.go` + `slotconst.go` — `ViewRow`/`InventorySlot`/`CharacterInventory`/`SlotCategory` + the canonical slot set.
- `internal/backendsrv/store/readviews.go` — `InventoryJoin` (`pp_rep` name-join CTE), `InventoryForChar`, `RosterFor`/`RosterRow` (is_mine + bank/bot flags), the name-vs-id discipline.
- `internal/backendsrv/store/assignment.go` — `ListMyAssignments`/`IsCharAssignedToTx` (viewer-character provenance).
- `internal/backendsrv/readapi/characters.go` / `inventory.go` / `itemsearch.go` — the route twins; the explicit `itemsearch.go ≠ guild holdings` boundary.
- `cmd/squirebot-server/main.go:286-363` — the `RequireSession` route registration site.
- `web/src/lib/api.ts` — `getJSON`, `fetchCharacters`/`fetchInventory`, the `RosterCharacter`/`InventorySlot`/`CharacterInventory` interfaces.
- `web/src/lib/roster.ts` — the pure viewer-first/filter helper to mirror.
- `web/src/lib/examine.ts` + `web/src/lib/components/{ExaminePanel,StateBlock,PaperdollSlot}.svelte` — the reuse seams.
- `web/src/routes/characters/+page.svelte` — the master-detail + `?c=` selection seam; `web/src/routes/inventory/+page.svelte` — the placeholder to replace.
- `web/vite.config.ts` — the node-only test project config (DOM-blind).
- `.planning/phases/32-inventory-tab-item-centric/{32-CONTEXT.md,32-UI-SPEC.md}` — locked decisions + the approved UI contract.
- `.planning/phases/{29,31}-*/[..]-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/config.json` — scope, requirements, workflow flags.

### Secondary (MEDIUM confidence — project memory, cross-checked against code)
- `pigparse-vs-ingame-item-id-namespaces` — name-join, never raw item_id (confirmed in `readviews.go`).
- `web-tests-node-only-blind-to-dom` + `web-local-dev-cant-auth-against-prod` — verification path (confirmed by `vite.config.ts`).
- `project_inventory_file_format` — no coin in the file (confirmed by `View`'s `item_id > 0` filter).
- `project_consolidated_views` — per-item master-detail RELAXED (confirmed in CLAUDE.md).

### Tertiary (LOW confidence)
- None — every claim is code-verified or cited; no unverified web/training claims were needed.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every module is in-repo and code-read.
- Architecture: HIGH — the rollup is a pure grouping over two verified reads; the route + web shape are exact twins of shipped code.
- Pitfalls: HIGH — the three load-bearing traps (name-not-id, route collision, DOM-blind tests) are all verified against code + memory.
- Slot-label exactness: MEDIUM — the data is present; the literal `Bag · {name}` label needs a small additional lookup if the user requires it (Open Q1 / A2).

**Research date:** 2026-06-18
**Valid until:** 2026-07-18 (stable — internal-only surface; revisit if the backend route idiom or the EQ-theme token set changes).

## RESEARCH COMPLETE
