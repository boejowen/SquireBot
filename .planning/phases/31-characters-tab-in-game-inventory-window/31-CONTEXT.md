# Phase 31: Characters Tab + In-Game Inventory Window - Context

**Gathered:** 2026-06-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Web surface (`web/`, SvelteKit) over the Phase 29 data foundation, plugged into the
Phase 30 app shell — plus the one new backend piece this requires (a read-API endpoint
to expose the structured inventory, and item→icon-id enrichment). Two parts:

1. **Characters tab (CHAR-01..03)** — a guild character list (name, level, race, class)
   ordered the viewer's own characters first (A-Z), then other guild characters, then guild
   banks/bots; a per-character name search that prioritizes the viewer's characters;
   selecting a character (list row or search result) opens that character's inventory window.

2. **In-game-style inventory window (INV-01..04)** — for the selected character:
   - **Equipment** rendered as a faithful EQ paperdoll (the 21 canonical slots).
   - **General inventory** slots with **openable bags** (bag drill-down).
   - The character's **own bank** listed below, as a faithful EQ bank-window grid.
   - **Real P1999 wiki item icons**, with the locked colored-tile fallback on load error.
   - A right-click-style **examine** (hover preview + click/tap-to-pin detail panel).

**Out of phase scope (owned elsewhere — do not build here):**
- The data parse/model itself (Phase 29 SHIPPED `StructuredInventory` + slot taxonomy +
  container nesting + name-keyed price/last-listed; the watcher is untouched).
- The 5-tab shell, routing, and the Characters route slot (Phase 30 SHIPPED; this phase
  fills the Characters-tab placeholder body + its scoped search per the D-08 pattern).
- The **item-centric** Inventory tab / holder drill-down (Phase 32, ITEM-01..03).
- The **Banks tab** list + guild-wide valuation/platinum totals (Phase 33, BANK-01..03).
  NOTE: Phase 31 renders one character's OWN personal bank slots in their window; the
  guild-wide bank valuation surface is Phase 33. Phase 33 reuses THIS window per bank toon.
- The per-character/per-slot **Wishlist** rework (Phase 34, WISH-01..07) — which depends on
  this phase's equipped-slot rendering.
- Exact pixel layout, mobile reflow of the paperdoll, slot-grid/tooltip styling →
  `/gsd-ui-phase 31` (UI hint = yes) produces the UI-SPEC.

</domain>

<decisions>
## Implementation Decisions

The four core gray areas were each locked to the recommended option in one pass; the user
then confirmed the smaller discretion defaults below for capture. (Continues the user's
delegate-and-lock pattern — see [[feedback_delegate_gray_areas]].)

### Item icons (INV-04) — source & storage
- **D-01:** Item→icon-id mapping is obtained by **extending the existing weekly P1999 wiki
  enrichment job** to also capture each item's wiki icon id, **cached server-side on the
  Hetzner box** (matches the spec's "wiki information stored on the Hetzner server").
  Coverage grows automatically as enrichment runs — **no manual curation**, no
  fetch-on-demand round-trip. Rejected: a hand-maintained static lookup (manual upkeep) and
  fetch-on-demand+cache (first-view latency).
- **D-02:** Icons render from `https://wiki.project1999.com/images/Item_<iconId>.png`; the
  **colored-tile fallback on image load error is locked** (sketch 002). Because every gap
  falls back gracefully, the icon coverage can ship **incrementally** — the window is never
  blocked on 100% icon coverage.
- **D-03 (storage = extend-only):** The icon id is stored alongside the existing wiki
  enrichment via an **extend-only** schema change (new column/table; never a breaking change
  to existing tables — CLAUDE.md schema-evolution rule). The exact wiki field/parse that
  exposes the icon id is a **research item** (the wiki Item-page infobox image filename is
  the likely source — research must confirm).

### Bag drill-down (INV-03)
- **D-04:** Opening a general-inventory bag **expands its contents inline** (beneath/within
  the grid), behaving like the inventory grid — stays in context, mobile-friendly, and
  matches INV-03's literal "behave like the inventory grid." Rejected: a modal pop-out
  (higher game-fidelity but clunkier on web/mobile — the sketch used it but flagged it) and
  routing contents into the pinned examine panel. The Phase 29 `InventorySlot.Children`
  nesting feeds this directly (`Slots > 0` marks an openable container).

### Bank section fidelity (INV-01)
- **D-05:** The character's own bank renders as a **faithful EQ bank-window-style grid**
  (Bank1..Bank8 positions + openable bank bags) directly **below the paperdoll**, **reusing
  the same grid + inline-drill-down component** as general inventory. This honors the
  "in-game window" goal AND is DRY (one grid renderer + one bag-open interaction, not two).
  Rejected: a plain scannable list (less literal + a second rendering style to maintain).
  No Inventory/Bank toggle — bank is one continuous section below general (sketch 002 lock).

### Examine interaction + missing data (INV-02)
- **D-06:** **Hover preview + click-to-pin.** Desktop: hovering an item shows a lightweight
  examine preview; clicking **pins** the full examine into a side detail panel (compare while
  browsing). Touch: tap = pin (no hover). Matches the spec's "hover **or** tap" and the
  in-game right-click feel. Rejected: click-to-pin only (simpler but loses the desktop hover
  affordance the spec calls for).
- **D-07 (single pinned panel):** The pinned detail panel is **single — replace-on-click**
  (clicking a new item updates the one panel); the hover preview is transient. Not a
  multi-pin compare board.
- **D-08 (examine content + order — LOCKED upstream):** flags → slot/skill → DMG/DLY → AC →
  stats → wt/size → class/race → **PigParse price** → **wiki link** → **last-synced**
  (`.planning/sketches/MANIFEST.md`). Stats come from the stored wiki data
  (`WikiSummary`/`WikiURL`), price/last-listed from the Phase 29 name-keyed join.
- **D-09 (graceful missing data):** When an item has no stored wiki stats or no PigParse
  price, the missing fields are **simply omitted** — never shown blank/broken/"null". An
  examine always renders at least the name + whatever is known.

### Character list (CHAR-01/02) — ordering & data (spec-fixed, captured for clarity)
- **D-10:** Default order = the **viewer's own characters first (A-Z)**, then other guild
  characters, then guild banks/bots; the per-character name **search prioritizes the
  viewer's characters**. Viewer identity = the Discord session; "the viewer's characters" =
  their `character_assignment` rows (v2.3). List rows show name, level, race, class.

### Empty / sparse states
- **D-11:** Empty equipment slots render as **empty paperdoll positions** (Phase 29 already
  KEEPS empty slots, `item_id 0`). A character with **no synced inventory yet** shows a
  friendly "no inventory synced yet" empty state in the window (not a crash/blank). A
  character **missing level/race/class** metadata shows what's available (— / blank for the
  missing fields), not an error.

### Claude's Discretion (planner/researcher owns these)
- The exact wiki field/parse that yields the icon id, and whether icon-id storage is a new
  column on the item/wiki table vs. a small mapping table (extend-only either way — D-03).
- The new read-API surface: `StructuredInventory` is NOT yet wired to a route — pick the
  endpoint shape (e.g. `GET /api/v1/inventory/{char}`) + the character-list endpoint
  (roster + meta + assignment + bank/bot flags), following the existing `readapi` pattern.
- Whether the hover preview reuses the existing `ItemTooltip.svelte` while the pinned panel
  is a fuller examine, or both share one component.
- Paperdoll grid arrangement, slot labels, mobile reflow, icon sizing → deferred to UI-SPEC.
- Whether the character list reuses `DataGrid.svelte` or a purpose-built list.

</decisions>

<specifics>
## Specific Ideas

- The Characters tab should "look and behave as much like the in-game EQ inventory menu as
  possible" (`Future Features.txt`) — paperdoll equipment slots flanking the figure, general
  inventory below, bank below that, openable bags, right-click examine.
- Canonical EQ paperdoll slots — the AUTHORITATIVE set is `compute/slotconst.go` (Phase 29),
  which defines **23** equipment slots incl. **Ear1, Ear2** (so: Charm, Ear1, Ear2, Head, Face,
  Neck, Shoulders, Arms, Back, Wrist1, Wrist2, Range, Hands, Primary, Secondary, Finger1,
  Finger2, Chest, Legs, Feet, Waist, Power, Ammo). (An earlier list in 29-CONTEXT omitted the
  two ear slots — defer to `slotconst.go`, not the prose.) General = General1..General10; Bank = Bank1..Bank8;
  nested bag/bank-bag contents = `<ParentSlot>-Slot<N>`; `Slots > 0` ⇒ openable container.
- Verified real icon ids (sketch 002): Cloak of Flames 658, Wurmslayer 736, Ring of the
  Ancients 563, Blue Diamond 966, Rubicite BP 624, Cloudy Potion 585 →
  `https://wiki.project1999.com/images/Item_<iconId>.png`.
- Reuse the live EQ themes unchanged (Velious default; `--accent`/`--panel`/`--font-display`;
  Cinzel Decorative + IM Fell English) — this is a structure/data revamp, not a re-skin.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — "Phase 31: Characters Tab + In-Game Inventory Window" section
  (goal, 4 success criteria, dependency on Phases 29 + 30, the strict 29→30→31→32→33→34 chain;
  Phase 31's equipped-slot rendering also unblocks Phase 34).
- `.planning/REQUIREMENTS.md` — CHAR-01, CHAR-02, CHAR-03, INV-01, INV-02, INV-03, INV-04
  (and INV-05/DATA-01 already done in Phase 29 — the data this window reads).

### Locked design direction
- `.planning/sketches/MANIFEST.md` — locked decisions: inventory window **Variant D**
  (paperdoll + click-to-pin + bank below general, no toggle), examine content/order, real
  wiki icons + colored-tile fallback, name-keyed price/last-listed.
- `.planning/sketches/002-character-inventory-window/README.md` + `index.html` — the chosen
  Variant D and the three "for planning" open questions THIS discussion resolved (bag
  drill-down → inline D-04; bank fidelity → faithful grid D-05; examine model → hover+pin D-06).
- `.planning/sketches/001-app-shell-5tab-nav/` (README + index.html) — the Characters list
  ordering (viewer-first) + per-tab scoped-search pattern the Characters tab follows.
- `Future Features.txt` (user's desktop, 2026-06-17 — NOT in repo) — authoritative target-UX
  spec: "an organized grid of each of the items equipped on that character" that "look[s] and
  behave[s] as much like the in-game inventory menu as possible."
- `CLAUDE.md` — Architecture: consolidated-views lock **relaxed** (per-character master-detail
  allowed — this window is the canonical drill-down); **extend-only** schema evolution (D-03);
  watcher untouched; EQ-theme single `[data-theme]` writer.

### Prior phase context (continuity)
- `.planning/phases/29-data-foundation-inventory-parse-price-value-aggregation/29-CONTEXT.md`
  — the data model this window renders: slot taxonomy, container nesting, name-keyed price,
  empty-slot retention, the PigParse item-id-namespace caveat.
- `.planning/phases/30-app-shell-5-tab-navigation/30-CONTEXT.md` — the shell/routing the
  Characters tab plugs into (D-03b preserved-classic-view, D-08 per-tab-search pattern, the
  theme-context bridge); the Characters route is a Phase-30 placeholder this phase fills.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (backend)
- `internal/backendsrv/compute/inventory.go` — **`StructuredInventory(ctx, store, char)` →
  `CharacterInventory{Char, Equipment[], General[], Bank[]}`** (Phase 29). Each
  `InventorySlot{Location, Category, CanonicalSlot, Item, ID, Count, Slots, Price,
  LastListed, WikiURL, WikiSummary, IsQuestItem, Prices, Children[]}`. The window renders
  this directly; `Children` is the nested bag/bank-bag contents (D-04/D-05); empty slots are
  kept (D-11). This is compute-on-read — no new parse needed.
- `internal/backendsrv/compute/types.go` + `slotconst.go` + `eqconst.go` — the slot-model
  types + canonical EQ slot constants / paperdoll ordering.
- `internal/backendsrv/store/readviews.go` — the store read seam `StructuredInventory` rides
  (compute→store; `*Tx` single-tested-SQL-path; `?` placeholders only; extend-only).
- `internal/backendsrv/store/assignment.go` — character_assignment reads → "the viewer's
  characters" for D-10 ordering/search priority.
- `internal/backendsrv/readapi/{views.go,meta.go,itemsearch.go,cors.go}` — the versioned
  read-API pattern + char-meta (level/race/class) source. **`StructuredInventory` has NO
  route yet** — Phase 31 ADDS the inventory-window endpoint + a character-list endpoint here.
- `internal/backendsrv/enrich/` (weekly wiki job) — extend here for the D-01/D-03 item→iconId
  capture (the new backend/data work this phase introduces).

### Reusable Assets (web)
- `web/src/routes/characters/+page.svelte` — the Phase-30 **placeholder** to replace with the
  real Characters tab (list + scoped search + window).
- `web/src/lib/components/ItemTooltip.svelte` — existing rich-HTML tooltip → candidate for the
  D-06 hover preview (pinned panel may reuse or extend it).
- `web/src/lib/components/DataGrid.svelte` — existing filterable/sortable grid (candidate for
  the character list; the window's paperdoll/bag grid is purpose-built).
- `web/src/lib/components/SiteShell.svelte` — the 5-tab shell (Phase 30); the Characters tab
  body mounts inside it; per-tab search follows the D-08 placement pattern.

### Established Patterns
- Compute-on-read (`compute/` imports `store`+`enrich`; authors zero SQL); extend-only schema
  via `goose`; never string-concat untrusted names into SQL.
- EQ-theme tokens only (`--accent`/`--panel`/`--font-display`); 44px touch targets;
  focus-visible 2px accent outline.
- **Web tests are node-only / DOM-blind** ([[web-tests-node-only-blind-to-dom]]) — green
  vitest ≠ works in the browser. This window (paperdoll, hover/pin, bag expand, icons) MUST be
  **browser-smoked on a deployed build** (localhost can't auth against prod —
  [[web-local-dev-cant-auth-against-prod]]).

### Integration Points
- NEW read-API: inventory window (`StructuredInventory` for one char) + character list
  (roster + char-meta + assignment + bank/bot flags), session-gated (`RequireSession`).
- NEW enrichment field: item→iconId captured by the weekly wiki job, stored extend-only.
- The Characters tab consumes the Phase 30 shell + scoped-search pattern; this window is
  later REUSED by Phase 33 (per bank toon) and its equipped-slot data feeds Phase 34.

</code_context>

<deferred>
## Deferred Ideas

- **Item-centric Inventory tab** (which characters hold item X, holder+slot drill-down) —
  Phase 32 (ITEM-01..03).
- **Banks tab** (banks-only list + guild-wide PigParse value + total platinum) — Phase 33
  (BANK-01..03); reuses THIS inventory window per bank toon + the Phase 29
  `BankValuationFor`/`TotalPlatinum`.
- **Per-character/per-slot Wishlist rework** — Phase 34 (WISH-01..07); depends on this
  phase's equipped-slot rendering (currently-equipped item per slot).
- **Modal/draggable bag pop-out** (max in-game fidelity) — rejected for inline expand (D-04);
  revisit only if inline proves insufficient.
- **Multi-pin examine compare board** — rejected for single replace-on-click panel (D-07).

None of the above is dropped — each is owned by its mapped downstream phase.

</deferred>

---

*Phase: 31-characters-tab-in-game-inventory-window*
*Context gathered: 2026-06-18*
