# Phase 32: Inventory Tab (Item-Centric) - Context

**Gathered:** 2026-06-18
**Status:** Ready for planning

<domain>
## Phase Boundary

The **item-centric** Inventory tab — the guild-wide answer to *"which characters have item X?"* —
a web surface (`web/`, SvelteKit) in the Phase 30 app shell over Phase 29 data, plus the one
new backend piece it requires (an item-rollup read endpoint). Three requirements:

1. **Guild-wide item list (ITEM-01)** — every item the guild holds, each row showing name,
   guild-wide quantity, a wiki link, and a PigParse price that links to PigParse when applicable.
2. **Viewer-prioritized item search (ITEM-02)** — a per-item name search that floats items held
   on the viewer's own characters to the top.
3. **Master-detail drill-down (ITEM-03)** — selecting an item reveals which characters hold it,
   the inventory slot on each, the quantity, and the last-synced day/time — consistent with the
   P31 character-window examine (master-detail, click-to-pin).

**Out of phase scope (owned elsewhere — do not build here):**
- The inventory parse/model + name-keyed price/last-listed join (Phase 29 SHIPPED — `compute.View`
  produces the guild-wide instance rows; `StructuredInventory` produces per-char slot structure;
  the watcher is untouched). Phase 32 ADDS the *item-centric aggregation* over this.
- The 5-tab shell, routing, the Inventory route slot + its scoped-search placement (Phase 30
  SHIPPED — this phase fills the Inventory-tab placeholder body).
- The in-game **character** inventory window (Phase 31 SHIPPED) — Phase 32 REUSES its `ExaminePanel`
  and deep-links INTO its `/characters?c=<name>` window, but does not rebuild it.
- The **Banks** tab + guild-wide valuation/platinum totals (Phase 33, BANK-01..03).
- The per-character/per-slot **Wishlist** (Phase 34, WISH-01..07).
- Exact pixel layout, mobile reflow of the master-detail split, list/row styling →
  `/gsd-ui-phase 32` (UI hint = yes) produces the UI-SPEC.

</domain>

<decisions>
## Implementation Decisions

All four discussed gray areas were locked to the recommended option in one pass (the user's
established delegate-and-lock pattern — see [[feedback_delegate_gray_areas]]).

### List scope & item identity (ITEM-01)
- **D-01:** **One row per normalized item NAME, counting every copy held anywhere.** The list
  aggregates ALL holdings — equipped + general inventory + bag contents + bank — across **every**
  character, bank toon, and guild bot. Nothing is hidden (no exclude-equipped, no junk filter).
  Identity is the **normalized name**, NOT `item_id`: inventory item-ids belong to the EQ namespace
  while PigParse/gear-tier catalog ids differ (and gear-tier rows have no id at all), so name is the
  only consistent join/group key — this matches the Phase 29 DATA-01 name-keyed price join. Rejected:
  exclude-equipped (loses "who has item X" completeness) and hide-zero-value (would suppress legitimate
  no-price items the user may still be hunting).

### Default list ordering (ITEM-02)
- **D-02:** **Viewer's items first, then A-Z.** With no search active, items held on the viewer's own
  characters float to the top, then alphabetical by name. This mirrors the P31 character-list ordering
  (D-10) and makes the resting order consistent with ITEM-02's search-time priority — your stuff is
  always on top. The viewer's identity = the Discord session; "the viewer's items" = items held on
  their `character_assignment` characters (v2.3). Rejected: plain A-Z (ignores ownership at rest) and
  most-held-first (commodity-forward, but buries the viewer's own gear).

### Detail panel content (ITEM-03)
- **D-03:** **Examine + holders table, with holder deep-links.** Selecting an item opens a detail
  panel that BOTH (a) reuses the P31 `ExaminePanel` (wiki stats / PigParse price / wiki link /
  last-synced, in the locked D-08 order, the single escaped `composeItemNote` `{@html}` sink) AND
  (b) shows a **holders table** — one row per holding: character · slot · quantity · last-synced.
  **Clicking a holder deep-links into that character's inventory window on the Characters tab**
  (`/characters?c=<name>`, the P31 selection seam). This is the most consistent (same examine as P31)
  and most actionable (jump straight to the holder's window). Rejected: holders-table-only (loses the
  examine block that the user already built) and examine+holders-without-deep-link (a dead-end —
  the cross-tab jump is the high-value interaction).
- **D-03a (single pinned panel, replace-on-click):** Same model as P31 D-06/D-07 — one detail panel,
  selecting a new item replaces it; not a multi-pin compare board.

### Row headline & price (ITEM-01)
- **D-04:** **Each list row shows quantity + holder count + inline PigParse price + wiki link.**
  Headline = **summed stack count** AND **holder count** (e.g. "142 · 3 holders") — both numbers, per
  sketch 003. Quantity is the sum of stack `Count`s across all holdings (total individual items), and
  the holder count is the number of distinct holding characters. PigParse price (the existing
  `pickPrice` buy/sell selection) and the wiki link render **inline on the row**, scannable without
  opening the detail. Rejected: counts-only-with-price-in-detail (less scannable) and summed-count-only
  (drops the useful "how spread out is it" holder count).

### Carried forward (locked upstream — NOT re-discussed, apply as-is)
- **Master-detail + click-to-pin** (sketch 003 Variant B) — same mental model as the P31 character
  window; the resting list is a guild-wide consolidated grid (allowed) and the per-item detail is the
  master-detail drill-down (allowed under the relaxed consolidated-views rule).
- **Real wiki icons + colored-tile fallback** on the list rows (P31 D-02 / `PaperdollSlot` pattern).
- **Examine content & order** = P31 D-08 (flags → slot/skill → DMG/DLY → AC → stats → wt/size →
  class/race → PigParse price → wiki link → last-synced); **graceful missing-data → omit** (P31 D-09 —
  never blank/"null"; a row/examine always renders at least the name).
- **EQ theme tokens only**, 44px touch targets, focus-visible 2px accent outline.
- **Node web tests are DOM-blind** ([[web-tests-node-only-blind-to-dom]]) → this tab MUST be
  **browser-smoked on a deployed build** ([[web-local-dev-cant-auth-against-prod]]).

### Claude's Discretion (planner/researcher owns these)
- **The item-rollup backend shape** — almost certainly a NEW `compute` function (group all holdings
  by normalized name → guild-wide summed qty + holder count + per-holder {char, slot label,
  qty, last-synced} + name-keyed price/wiki) over the existing `compute.View` / `StructuredInventory`
  data; exposed as a NEW session-gated read-API route (e.g. `GET /api/v1/items` — name TBD), following
  the P31 `readapi` pattern. Compute-on-read; extend-only; `?`-bound; never string-concat names into SQL.
  (Do NOT reuse `GET /api/v1/items/search` — that is the P19 wantlist search over the PigParse *catalog*,
  not guild holdings.)
- **Slot-label granularity in the holders table** — how to render the holder's slot (e.g. "Chest",
  "General3", "Bank2", "Bag: Large Backpack") from the `Location`/canonical-slot taxonomy; whether to
  group equipped vs bagged vs bank within a holder's rows.
- **Holders-table ordering** — viewer's own characters first within the holders list (consistent with
  D-02) is the expected default; confirm in planning.
- **Whether the list reuses `DataGrid.svelte`** or a purpose-built master-detail list; whether the
  detail's examine reuses `ExaminePanel.svelte` directly or a thin wrapper.
- **Quantity edge cases** — coin/platinum rows are NOT items (excluded); how stack-count summation
  treats bag-vs-loose copies of the same name (sum all).
- Exact list/detail split layout, mobile reflow, row density, icon sizing → deferred to UI-SPEC.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — "Phase 32: Inventory Tab (Item-Centric)" section (goal, 3 success criteria,
  dependency on Phases 29 + 30; strict 29→30→31→32→33→34 chain).
- `.planning/REQUIREMENTS.md` — ITEM-01, ITEM-02, ITEM-03 (and INV-05/DATA-01 from Phase 29 — the
  parsed/priced data this tab aggregates).

### Locked design direction
- `.planning/sketches/MANIFEST.md` — Inventory list = **sketch 003 Variant B (master-detail)**, same
  click-to-pin model as the character window (002); guild-wide count + holder count headline;
  search-prioritizes-your-items; real wiki icons; name-keyed price/last-listed.
- `.planning/sketches/003-inventory-and-banks-lists/README.md` + `index.html` — the chosen Variant B
  and the "for planning" notes (item-rollup is new backend aggregation; "holder · slot · last-synced"
  needs the same `Location`→slot parse as 002).
- `Future Features.txt` (user's desktop, 2026-06-17 — NOT in repo) — authoritative target-UX spec:
  the Inventory tab answers "which characters have item X?" with an expandable holder list.
- `CLAUDE.md` — consolidated-views lock **relaxed** (guild-wide grid + per-item master-detail both
  allowed); **extend-only** schema; watcher untouched; EQ-theme single `[data-theme]` writer.

### Prior phase context (continuity)
- `.planning/phases/31-characters-tab-in-game-inventory-window/31-CONTEXT.md` — the examine model
  (D-06/07/08/09), the `?c=<name>` selection seam this tab deep-links into, the icon/fallback pattern.
- `.planning/phases/29-data-foundation-inventory-parse-price-value-aggregation/29-CONTEXT.md` — the
  data model this tab aggregates: slot taxonomy, container nesting, name-keyed price, the PigParse
  item-id-namespace caveat (group/join by normalized name, never raw item_id).
- `.planning/phases/30-app-shell-5-tab-navigation/30-CONTEXT.md` — the shell/routing + per-tab
  scoped-search pattern the Inventory tab plugs into (the Inventory route is a Phase-30 placeholder
  this phase fills).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (backend)
- `internal/backendsrv/compute/view.go` — **`View(ctx, store) → []ViewRow`** (Phase 29): the
  guild-wide consolidated inventory rows (one per item-instance per char: Char, Location, Name, ID,
  Count, Price, Prices, wiki/quest links). The item-rollup aggregates THIS (group by normalized name).
- `internal/backendsrv/compute/inventory.go` — `StructuredInventory` + `classifySlot`/`canonicalNumbered`
  — the `Location`→slot-label taxonomy reused to render each holder's slot in the holders table.
- `internal/backendsrv/store/readviews.go` — the store read seam (compute→store; `*Tx`; `?` placeholders
  only; extend-only) the rollup would ride.
- `internal/backendsrv/store/assignment.go` + `RosterFor` (Phase 31) — character_assignment reads →
  "the viewer's characters/items" for D-02 ordering + ITEM-02 search priority.
- `internal/backendsrv/readapi/{views.go,characters.go,inventory.go,itemsearch.go,cors.go}` — the
  versioned read-API pattern. P31's `characters.go`/`inventory.go` are the closest analogs for the NEW
  item-rollup route. **`itemsearch.go` (`GET /api/v1/items/search`) searches the PigParse CATALOG (P19
  wantlist), NOT guild holdings — do not reuse it for this list.**

### Reusable Assets (web)
- `web/src/routes/inventory/+page.svelte` — the Phase-30 **placeholder** to replace with the real
  item-centric tab (master-detail list + scoped search + detail panel).
- `web/src/lib/components/ExaminePanel.svelte` (Phase 31) — reused for the item detail's examine block
  (the single escaped `composeItemNote` `{@html}` sink; D-08 order).
- `web/src/lib/examine.ts` (Phase 31) — `examineFields` pure helper (node-tested) for the examine rows.
- `web/src/routes/characters/+page.svelte` (Phase 31) — the `?c=<name>` selection seam the holder
  deep-link targets.
- `web/src/lib/components/{DataGrid,StateBlock,PaperdollSlot}.svelte` + `web/src/lib/api.ts` — grid
  candidate, loading/error/empty states, the icon tile, and the credentialed fetch wrappers
  (`getJSON`, `fetchCharacters`/`fetchInventory` — add a `fetchItems()` twin).
- `web/src/lib/roster.ts` (Phase 31) — the pure viewer-first/search helper pattern to mirror for the
  item list (a pure `items.ts` with node tests).

### Established Patterns
- Compute-on-read (`compute/` imports `store`+`enrich`, authors zero SQL); extend-only schema via
  `goose`; never string-concat untrusted names into SQL.
- Pure DOM-free helpers extracted to `.ts` + node-tested (myview.ts/roster.ts/examine.ts precedent);
  the rendered DOM stays a browser-smoke gap.
- Session-gated read routes under `webauth.RequireSession` (login-gated since P15; NOT public, NOT
  officer-only); V7 slog carries counts/status, never raw item/char values.

### Integration Points
- NEW read-API: an item-rollup endpoint (guild-wide items + per-item holders w/ slot + last-synced +
  name-keyed price), session-gated. NEW pure web helper (`items.ts`) + `fetchItems()` wrapper.
- The Inventory tab consumes the Phase 30 shell + scoped-search pattern; its detail reuses the P31
  `ExaminePanel` and deep-links into the P31 `/characters?c=` window.
- Likely **no new migration** — the rollup is compute over existing tables (icon_id/statsblock already
  added by P31's 00012/00013). Confirm in research.

</code_context>

<specifics>
## Specific Ideas

- The headline per item is the sketch-003 form: "{summed qty} · {holder count} holders" (e.g.
  "142 guild-wide · 3 holders").
- The detail's holders table columns: character · slot · quantity · last-synced (ITEM-03 literal),
  with the viewer's own characters surfaced first.
- Clicking a holder navigates to `/characters?c=<charName>` — the live P31 selection seam (opens that
  character's inventory window directly).
- Reuse the live EQ themes unchanged (Velious default; `--accent`/`--panel`/`--font-display`; Cinzel
  Decorative + IM Fell English) — structure/data revamp, not a re-skin.

</specifics>

<deferred>
## Deferred Ideas

- **Banks tab** (banks-only list + guild-wide PigParse value + total platinum, reusing the P31 window
  per bank toon) — Phase 33 (BANK-01..03); the Phase 29 `BankValuation`/`TotalPlatinum` aggregation
  feeds it.
- **Per-character/per-slot Wishlist rework** — Phase 34 (WISH-01..07).
- **Sort/filter controls on the item list** (by value, by holder count, category facets) beyond the
  locked viewer-first/A-Z default + name search — revisit only if the default proves insufficient
  (a UI-SPEC / future-polish consideration, not net-new scope).
- **Multi-pin item compare board** — rejected for the single replace-on-click panel (D-03a).

None of the above is dropped — each is owned by its mapped downstream phase.

</deferred>

---

*Phase: 32-inventory-tab-item-centric*
*Context gathered: 2026-06-18*
