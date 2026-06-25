# Phase 37: Item enrichment backbone — flags + effects - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Re-enable the wiki item-page stat-block parse that the Go port currently DISCARDS
(the `wikiitem.go` "D-8 scope guard") and persist its output as discrete, queryable
fields on the item record: item flags (Lore / No-Drop / Magic / Temporary + whatever
else the parser detects) and the Clicky / Haste effects. This is the shared DATA
LAYER only — the item-display outlines (Phase 40 / ITEMUI-01) and the search facets
(Phase 39 / SEARCH-04/05) READ these fields but are out of scope here. Includes a
goose migration for the new fields and an immediate backfill so already-enriched
items populate without waiting for the weekly wiki crawl.

**In scope:** ENRICH-12 (flags → discrete fields), ENRICH-13 (Clicky/Haste → discrete
fields), the migration, the backfill, and the enrich freshness short-circuit update.

**Out of scope (later phases):** any UI/rendering of these fields (outlines = P40,
facets = P39); widening enrichment coverage to the full catalog (P38 / ENRICH-14);
icon work (P38 / ENRICH-15).
</domain>

<decisions>
## Implementation Decisions

### Clicky / Haste effect parsing (ENRICH-13)
- **D-01:** "Clicky" = an item with an **activatable click effect** — inventory-clickable
  OR must-equip-then-click — but NOT worn passives and NOT combat procs. This is the
  common P99 meaning; the `is_clicky` boolean and the downstream SEARCH-04 facet are
  scoped to exactly this set. The parser must distinguish Click (activatable) from
  Worn/Combat effect lines.
- **D-02:** Store BOTH the boolean AND the value for each effect (the "booleans + values"
  choice): `is_clicky` + the clicky's spell/effect NAME; `has_haste` + the haste **%**
  value (integer). The values let the examine panel later show "Haste 21%" / the clicky
  name and enable value-sorting without a re-parse.

### Data shape — flags + effects (ENRICH-12)
- **D-03:** Capture **every** all-caps flag line the parser detects as a flag SET
  (the "everything detected" choice) — so a future flag never needs a new migration.
  Materialize discrete BOOLEANS for the ones we query/outline now: `is_lore`,
  `is_no_drop`, `is_magic`, `is_temporary`. Everything else (Attunable, No Rent,
  Artifact, Augmentation, …) lives in the captured set automatically.
- **D-04 (Claude's discretion — shape):** discrete columns for the queried booleans +
  values, plus the full flag set as a JSON/TEXT column (or a normalized side table) —
  consistent with the existing `item_master.statsblock TEXT` precedent (00013). Planner
  picks the exact shape; the REQUIREMENT is that the four named flags + `is_clicky` +
  `has_haste` are individually queryable and the rest are retained.

### Backfill timing (the load-bearing "lights up now" decision)
- **D-05:** **Immediate backfill from the stored stat block.** Re-parse the new fields
  from the `item_master.statsblock` TEXT that is ALREADY persisted (migration 00013) —
  **no wiki refetch / no network**. Already-enriched items populate instantly. New/changed
  items continue to refresh via the weekly wiki crawl as today.
- **D-06:** The enrich freshness short-circuit (`GetItemMasterFreshnessTx`, `store/enrich.go`)
  MUST be updated so a row missing the new fields re-parses on the next touch (mirrors how
  00012 added `icon_id` to the freshness comparison). The one-time backfill + the freshness
  update together guarantee no stale "parsed=null" rows linger.

### Carried forward (locked — not re-discussed)
- **Extend-only schema** (new columns at the right edge / additive side structure); goose
  migration applied on boot (next number after 00015 → **00016**); **watcher UNTOUCHED**;
  **no `v*` tag**.
- The parse logic **already exists** — the TS parser is the behavioral oracle and the Go
  `parseStatsblock` already classifies flags into a map (`wikiitem.go:262-290`) but reads
  only `flags["QUEST ITEM"]`. **Re-enable / extend, do not reinvent.**

### Claude's Discretion (implementation — planner/executor decide)
- Exact column-vs-JSON shape (per D-04); the goose migration mechanics; whether the
  backfill runs inside the migration, as a boot-time re-parse pass, or by forcing the
  freshness short-circuit — the only hard requirement is **immediate coverage from the
  stored stat-block text**.
- Parser internals for distinguishing Click vs Worn vs Combat effect lines (use the TS
  oracle + the existing `parseStatsblock` flag-map logic).
- Whether `WatcherMaxSchemaVersion` needs a bump — backend-only schema, no watcher write
  path touches these columns, so likely not; planner confirms against the schema-evolution
  rule in CLAUDE.md.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements / roadmap
- `.planning/REQUIREMENTS.md` — ENRICH-12 (flags → discrete fields) + ENRICH-13 (Clicky/Haste → discrete fields).
- `.planning/ROADMAP.md` — Phase 37 detail + success criteria (the "v2.6 — Item Detail & Polish" section; Phase Details lines ~409+).

### The parser (the thing being re-enabled)
- `internal/backendsrv/enrich/wikiitem.go` — the D-8 scope guard comment (`:11-17`, lists exactly which flags the port discards); `parseStatsblock` (`:262-290`, already builds a `flags` map, only `QUEST ITEM` read at `:91`); `cleanStatsblock` (`:404-428`); `parseIconID` (`:479-489`) as the "parse one field off the page" precedent.

### Storage + freshness (where the new fields land + the backfill source)
- `internal/backendsrv/migrations/00013_item_statsblock.sql` — adds `item_master.statsblock TEXT` (the ALREADY-STORED blob the immediate backfill re-parses).
- `internal/backendsrv/migrations/00012_item_icon.sql` — the `ADD COLUMN` (nullable, no default) + freshness-comparison precedent to mirror for the new fields.
- `internal/backendsrv/store/enrich.go` — `UpsertItemMasterTx` (`:159-197`, the upsert to extend); `GetItemMasterFreshnessTx` (`:224-236`, the SHA-1 + icon_id + statsblock short-circuit to extend per D-06).

### Conventions
- `CLAUDE.md` — schema-evolution rule (extend-only, version-stamped, `WatcherMaxSchemaVersion` discipline) + the structured-logging convention.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`parseStatsblock` flag map** (`wikiitem.go:262-290`) — already classifies all-caps
  lines as flags; only `QUEST ITEM` is consumed today. The "everything detected" flag
  set (D-03) is essentially surfacing this existing map.
- **The stored `statsblock` TEXT** (00013) — the backfill source; re-parsing it is pure
  CPU, no network (enables the "immediate" backfill, D-05).
- **`parseIconID` + 00012** — the canonical "parse one field, add a nullable column, add
  it to the freshness check, backfill on next pass" pattern this phase repeats for many fields.

### Established Patterns
- **Inventory-driven enrichment** stays as-is for Phase 37 (we only re-parse already-stored
  text); widening coverage is Phase 38, deliberately separate.
- **Freshness short-circuit** (`GetItemMasterFreshnessTx`) is the gate that decides re-parse;
  extending it (D-06) is how the backfill becomes self-healing.
- **Extend-only / goose-on-boot** — the migration is `00016`, additive, no watcher change.

### Integration Points
- New fields hang off the `item_master` row (same row the icon/statsblock/slot already use),
  so the Phase 39 search + Phase 40 examine/outlines read them via the existing
  `item_master`↔inventory id-join (EQ namespace) — no new join introduced here.
</code_context>

<specifics>
## Specific Ideas

- "Clicky" must mean what a P99 player means by it: a clicky you can actually activate
  (cloak of flames, geni-bottle-style clickies, must-equip clickies), NOT worn haste/stat
  items and NOT combat-proc weapons.
- Surface the haste **%** (e.g., "Haste 21%") and the clicky's spell name as values now so
  the examine panel and any later value-sort get them for free.
</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (The flag set captures extra flags like
Attunable / No Rent / Artifact, but surfacing those as outlines or facets is later/other
work; capturing them now is the in-scope, no-extra-migration choice per D-03.)
</deferred>

---

*Phase: 37-item-enrichment-backbone-flags-effects*
*Context gathered: 2026-06-24*
