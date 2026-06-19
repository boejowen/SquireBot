# Phase 29: Data Foundation — Inventory Parse + Price/Value Aggregation - Context

**Gathered:** 2026-06-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Backend-only. Turn the watcher's raw `Location | Name | ID | Count | Slots` inventory
rows into a clean, query-ready model that every v2.4 web tab (Phases 31–34) reads
**compute-on-read** instead of re-parsing strings:

1. **Slot taxonomy + container nesting (INV-05)** — classify each stored
   `inventory_item` row into `equipment` / `general` / `bank`, give equipment rows a
   canonical EQ paperdoll slot key, and represent bag contents (`<ParentSlot>-Slot<N>`)
   as a parent→children nesting tree. The watcher is **untouched** — it already
   uploads `Location`/`Slots`; this is server-side parsing/surfacing only.
2. **Name-keyed price + last-listed join (DATA-01)** — join PigParse price +
   last-listed-for-sale date to wiki/gear-tier items by **normalized name** (gear-tier
   rows carry no `item_id`), surfaced on examine, suggestions, and item lists.
3. **Bank valuation + total-platinum aggregation (DATA-02)** — summed PigParse value
   of bank-held items (per bank + guild-wide) and total platinum from the manual
   bank-coin entries, queryable as guild-wide totals that power the Banks tab.

**Out of phase scope:** any web/UI surface (Phases 30–34 render it), item-icon→iconId
mapping (Phase 31 / INV-04), and the guild-wide item *list* + holder drill-down UI
(Phase 32 — Phase 29 only produces the underlying rollup + holder-with-slot data).
No schema-breaking change to existing tables (extend-only).

</domain>

<decisions>
## Implementation Decisions

### Slot-model shape (INV-05) — locked to recommended default (not separately discussed)
- **D-01:** The backend emits a **structured slot model**, not a raw `Location`
  pass-through. Each inventory row is classified into `equipment` / `general` / `bank`;
  equipment rows carry a **canonical EQ paperdoll slot key** (Charm, Head, Face, Neck,
  Shoulders, Arms, Back, Wrist1, Wrist2, Range, Hands, Primary, Secondary, Finger1,
  Finger2, Chest, Legs, Feet, Waist, Power, Ammo); container nesting is a parent→children
  tree. Rationale: the phase goal is explicitly "a clean computed model rather than
  re-parsing strings," and Phases 31/33 render the paperdoll/bank window from this model.

### Container contents counting (INV-05 → rollups & valuation)
- **D-02:** Bag contents are **first-class held items, counted everywhere.** Parse the
  EQ `<ParentSlot>-Slot<N>` nesting format so contents (a) nest under their parent
  container for drill-down (INV-03), (b) count toward guild-wide item quantity
  (Inventory tab / ITEM-01), and (c) count toward bank valuation. The **container (bag)
  itself is also a priced item** that counts. (A gem sitting in a bag in the bank is
  "in the guild" and adds to bank value.)

### Bank valuation basis (DATA-02)
- **D-03:** Item value = the existing **`pickPrice`** (WTS 30-day average `a30`, else
  WTB `a30` fallback — `compute/view.go`) **× stack `count`**, summed across all
  bank-held items (including bag contents per D-02). Items with no PigParse price
  contribute **0**, but every valuation total carries an **"N items unpriced"**
  annotation so the figure is never silently understated.

### Total platinum (DATA-02)
- **D-04:** Guild bank total platinum = the **SUM of the `plat` column** across live
  bank toons (`is_bank_toon=1 AND is_removed=0`, via `store.ListBankToons`). **Literal
  platinum only** — gold/silver/copper are NOT rolled into the plat figure (they stay
  available separately). Matches the DATA-02 wording and the existing `character` coin
  columns.

### Claude's Discretion (implementation — planner/researcher owns these)
- **Compute-on-read vs materialized:** follow the established `compute/` compute-on-read
  pattern (guild-scale data is tiny — <100 MB, ~50–150 writes/day). Materialize only if
  research surfaces a concrete reason; default is compute-on-read.
- **Schema extension:** extend-only via a new `goose` migration if any new column/table
  is needed (e.g., a parsed slot-category or container-parent column on `inventory_item`),
  OR keep it purely computed on read. Either is acceptable; no breaking change to
  existing tables.
- **Name normalization:** use the existing `lower(trim(name))` convention
  (`ReplaceSpellbookTx` / `wiki_spells.normalized_name`) for the DATA-01 name join key.
- **last-listed-for-sale source:** `pigparse_price.last_seen` (already stored by the
  daily getall job); surface alongside price, blank when absent.

### ⚠ Research must reconcile — PigParse item_id namespace
- The existing inventory price join is **by `item_id`** (`pigparse_price.item_id` PK),
  but project memory (`pigparse-vs-ingame-item-id-namespaces`) warns the PigParse
  **catalog item_ids may differ from in-game EQ item_ids** — join by normalized name,
  not raw item_id. DATA-01 already mandates name-keyed join for the no-`item_id`
  gear-tier rows. **Research must verify** whether the existing `item_id` inventory
  price join is correct or should also fall back to / move to the normalized-name join,
  and NOT silently trust raw `item_id` across namespaces. Do not break the live `view`
  price display while resolving this.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — "Phase 29: Data Foundation" section (goal, 4 success
  criteria, the strict 29 → 30 → 31 → 32 → 33 → 34 dependency chain; Phase 29 unblocks
  31/32/33/34).
- `.planning/REQUIREMENTS.md` — INV-05, DATA-01, DATA-02 (and the downstream consumers
  INV-01..04, ITEM-01..03, BANK-02, WISH-04 that read this data).

### Locked design direction
- `.planning/sketches/MANIFEST.md` — locked sketch decisions: inventory window Variant D
  (paperdoll + bank below general), name-keyed price/last-listed join, gear-tier rows
  carry no item_id, "no Group/Raid binary," real-icon pattern (Phase 31).
- `Future Features.txt` (user's desktop, 2026-06-17) — authoritative target-UX spec.
- `CLAUDE.md` — Architecture section: consolidated-views lock **relaxed** (per-character
  master-detail allowed); **extend-only** schema evolution; watcher write contract
  (atomic replace; watcher untouched this milestone).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/backendsrv/compute/view.go` — `pickPrice` (WTS `a30` → WTB `a30` fallback)
  and `buildViewRows` (the pure compute-on-read transform). Reuse `pickPrice` verbatim
  for D-03 valuation; extend the transform layer for the structured slot model + totals.
- `internal/backendsrv/compute/types.go` — the FIXED snake_case JSON contract
  (`ViewRow`, `BankView`, `CoinTotals`). Extend (don't rename) for the slot model +
  valuation/platinum totals.
- `internal/backendsrv/store/coin.go` — `ListBankToons` returns plat/gold/silver/copper
  on live bank toons → the D-04 platinum source (sum `plat`).
- `internal/backendsrv/store/enrich.go` — `pigparse_price` schema (item_id PK, `name`,
  `last_seen`, `a30`/`t30`, `direction`); `wiki_gear_tier` (item_id always NULL → the
  DATA-01 name-join target); the single-tested-SQL-path `*Tx` rule (jobs/compute author
  zero inline SQL).
- `internal/backendsrv/store/itemids.go` — distinct inventory `(item_id, name)` refs;
  reference for name handling.
- `internal/backendsrv/enrich/wikigear.go` — gear-tier parse (tiers `Velious Pre-Raid/Group`
  + `Velious Raiding`); produces the no-`item_id` rows DATA-01 must name-join.
- `internal/backendsrv/compute/eqconst.go` + `internal/backendsrv/enrich/eqconst.go` —
  existing EQ slot constants (e.g. `WIKI_SLOT_TO_INV_SLOTS`) to seed the canonical
  equipment-slot taxonomy (D-01).

### Established Patterns
- **Compute-on-read:** `compute/` imports `store` (read methods) + `enrich` (constants);
  `store` never imports `compute`; `compute` authors zero SQL.
- **Store seam:** `*Tx` single-tested-SQL-path; `?` placeholders only; extend-only
  schema via `goose`; never string-concat untrusted item names into SQL.
- **Watcher untouched:** `internal/parse/inventory.go` passes the raw 5 columns through
  (`[Location, Name, ID, Count, Slots]`) with NO Location/Slots interpretation — every
  bit of slot/nesting parsing is new **server-side** work.

### Integration Points
- New parse/classify layer: `inventory_item` rows (Location/Slots) → structured slot
  model (equipment/general/bank + canonical equip-slot + nesting tree).
- New name-keyed price + last-listed lookup (normalized name) alongside the existing
  item_id join (see the ⚠ namespace reconciliation flag).
- New bank valuation (D-03) + total-platinum (D-04) aggregation over the store.
- **Test fixtures:** the existing `internal/parse/testdata/sample-inventory.txt` is
  synthetic and **flat** (no nested bag rows). Add a **real-name** inventory fixture
  WITH nested bag contents (`<ParentSlot>-Slot<N>`) per the CLAUDE.md fixture convention
  to exercise INV-05 nesting, name-join hits/misses, and value/platinum sums.

</code_context>

<specifics>
## Specific Ideas

- **Real slot taxonomy (from `/outputfile inventory`):** equipment = `Charm, Head, Face,
  Neck, Shoulders, Arms, Back, Wrist1, Wrist2, Range, Hands, Primary, Secondary, Finger1,
  Finger2, Chest, Legs, Feet, Waist, Power, Ammo`; general = `General1`..`General10`;
  bank = `Bank1`..`Bank8`; nested bag contents = `<ParentSlot>-Slot<N>`. The `Slots`
  column = a container's capacity (0 = not a container).
- The four success criteria in ROADMAP Phase 29 include a unit-test requirement: parse +
  joins + aggregation covered by Go unit tests against **real-name** inventory fixtures
  (slot positions, nested-bag contents, name-join hits/misses, value/platinum sums),
  applied over live data with no schema-breaking change.

</specifics>

<deferred>
## Deferred Ideas

- **Item-icon → iconId mapping** (`https://wiki.project1999.com/images/Item_<iconId>.png`)
  — Phase 31 (INV-04), not Phase 29.
- **Guild-wide item *list* + holder drill-down UI** — Phase 32 (ITEM-01..03). Phase 29
  produces only the underlying guild-wide rollup + holder-with-slot data.
- **Per-slot wishlist suggestion engine** (Velious Pre-raid/Group + Raiding per class+slot)
  — Phase 34 (WISH-04); Phase 29 provides the name-keyed price/last-listed join those
  suggestions consume.

None of the above is dropped — each is owned by its mapped downstream phase.

</deferred>

---

*Phase: 29-data-foundation-inventory-parse-price-value-aggregation*
*Context gathered: 2026-06-17*
