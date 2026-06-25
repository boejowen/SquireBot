# Phase 39: Faceted item search (Clicky / Haste + scope toggle) - Context

**Gathered:** 2026-06-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn the flat name/id item search into a discovery tool: add **Clicky** and **Haste**
facets (reading Phase 37's parsed `item_master.is_clicky` / `has_haste` fields) and a
**holdings ↔ full-catalog** scope toggle ("who has one" vs "what exists"). The facets
also apply to the wishlist add-item catalog search. Backend extends the holdings item
list + the catalog search with facet/scope filters; web adds the facet controls + the
scope toggle and renders catalog-scope rows.

**In scope:** SEARCH-04 (filter to Clicky-only), SEARCH-05 (filter to Haste-only),
SEARCH-06 (holdings↔catalog scope toggle on the Inventory tab) — plus the facets on
the wishlist add-search (D-01 expansion).

**Out of scope (later / other phases):** any facet beyond Clicky + Haste — slot, stat
thresholds (+STR/+AC), weapon type (1H/2H/piercing) are explicitly deferred (REQUIREMENTS
"Future Requirements"); the flag outlines + named quest links (Phase 40); a new
wishlist-pin action from catalog search (D-04 — NOT this phase). The watcher is UNTOUCHED.
</domain>

<decisions>
## Implementation Decisions

### Search surface / home (SEARCH-06, D-01)
- **D-01:** The facets + the holdings↔catalog scope toggle live on the **Inventory tab**
  (`/inventory` — the item-centric "who has one" guild-wide list); the toggle flips that
  tab's data source between guild holdings and the full catalog. **AND** the Clicky/Haste
  facets are ALSO added to the **wishlist add-item catalog search** (the existing
  `SearchCatalog`-backed add form), where they narrow the catalog suggestions. (The
  wishlist add-search is catalog-only — it has NO holdings scope toggle; only the facets
  apply there.) This is a small, deliberate scope expansion beyond a single surface,
  ratified by the user 2026-06-25.

### Facet controls + combination (SEARCH-04/05, D-02)
- **D-02:** The Clicky and Haste facets are **two independent toggles, AND-combined.**
  Neither on → all items (current behavior). One on → that type only. BOTH on → items that
  are BOTH clicky AND haste (those exist). No facet selected is NOT a filter. This is the
  most expressive form and maps directly onto "show me clickies" / "show me haste items."

### Default scope + toggle behavior (SEARCH-06, D-03)
- **D-03:** The Inventory tab **defaults to Holdings** ("who has one" — today's behavior +
  the guild's primary "where is it" question). Flipping the scope toggle to "what exists"
  (full catalog) **PERSISTS the typed query + the active facets** — the toggle reads as a
  lens over the same search, not a reset. (Same for facet state when toggling back.)

### Catalog-scope row content + actions (SEARCH-06, D-04)
- **D-04:** A catalog-scope ("what exists") row reuses the existing **examine panel** shape
  (name + flags/effects + PigParse price + wiki link + last-listed). If the item IS also
  held, the row still shows its **holders** (catalog = a SUPERSET of holdings); an item
  nobody holds reads "not held in the guild." **NO new "pin to wishlist" action this phase**
  — that discovery→wishlist shortcut is deferred (the wishlist add flow already exists on
  the Wishlist tab).

### Carried forward (locked — not re-discussed)
- **Facets = Clicky + Haste ONLY** (roadmap + P37 lock). "Clicky" = an activatable click
  effect (P37 D-01 — `is_clicky`); "Haste" = `has_haste` + the `haste_pct` value (P37 D-02).
- **Held-items faceting works INDEPENDENTLY of the catalog crawl** (roadmap SC-4) — the
  holdings facet must not block on full-catalog coverage filling in.
- Existing 5 EQ themes reused unchanged; per-character master-detail allowed; the guild-wide
  consolidated Inventory grid remains the holdings surface; watcher UNTOUCHED; no `v*` tag.

### Claude's Discretion (implementation — planner/executor decide)
- **Server-side vs client-side facet split (likely):** the **holdings** facet can be
  CLIENT-side over the already-loaded Inventory rollup IF the rollup payload carries
  `is_clicky` / `has_haste` (add those two booleans to the `ItemRollup`/items payload from
  `item_master`) — this also satisfies SC-4 (no catalog dependency). The **catalog** facet
  is SERVER-side: `SearchCatalog` (and/or a faceted full-catalog read) gains optional
  `clicky`/`haste` filter params (the corpus is large + LIMIT-capped). Planner confirms the
  exact split.
- **The catalog ↔ flags join for the facet fields (the load-bearing technical detail):**
  `pigparse_price.item_id` is the PigParse namespace. **⚠ Phase 38 shipped NAME-KEYED
  (deployed 2026-06-25) — NOT the originally-sketched "Option A id-keyed `item_master`":**
  `item_master` stayed **held-only, keyed by EQ id** (P37 migration 00016 added
  `is_clicky`/`has_haste` there); catalog-only flags live in a SEPARATE **`catalog_enrichment`**
  table (migration 00017), keyed by **`norm_name` = `lower(trim(name))`**
  (`internal/backendsrv/store/catalogenrich.go`). So a catalog row's `is_clicky`/`has_haste`
  is read from **`item_master` (held) ∪ `catalog_enrichment` (catalog-only)**, joined to
  `pigparse_price` by **normalized name** — the shipped `CatalogIconCoverage` UNION-ALL in
  `internal/backendsrv/store/itemids.go` is the exact precedent (swap `icon_id` for the two
  booleans). NEVER raw item_id. Name-keying makes this join clean end-to-end; the
  namespace-bridge hazard is gone.
- Exact query shapes, the facet param plumbing through `readapi`/`compute`/`api.ts`, the
  toggle/persistence state model in the Svelte Inventory tab, and the catalog-row render
  (reuse `ExaminePanel` + the holders display) — planner/UI-spec decide. **UI phase →
  expect a `/gsd-ui-phase 39` UI-spec gate before/within planning.**
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements / roadmap
- `.planning/REQUIREMENTS.md` — SEARCH-04 (Clicky filter), SEARCH-05 (Haste filter),
  SEARCH-06 (holdings↔catalog scope toggle); the "Future Requirements" note (facets beyond
  Clicky/Haste are explicitly deferred — do NOT add slot/stat/weapon-type).
- `.planning/ROADMAP.md` — Phase 39 detail + success criteria (lines ~437+); the SC-4
  "held-items faceting ships even if the catalog crawl runs long" constraint.
- `.planning/phases/37-item-enrichment-backbone-flags-effects/37-CONTEXT.md` — D-01 (Clicky
  meaning), D-02 (`is_clicky`+`clicky_effect`, `has_haste`+`haste_pct`); the discrete fields
  this phase filters on.
- `.planning/phases/38-catalog-wide-enrichment-icon-coverage/38-CONTEXT.md` +
  `38-RESEARCH.md` — ⚠ D-04 was REVERSED to NAME-KEYED (shipped 2026-06-25): catalog-only
  enrichment lives in `catalog_enrichment` keyed by `norm_name` (migration 00017); `item_master`
  stayed held-only by EQ id. The basis for the catalog↔flags facet join (read `item_master` ∪
  `catalog_enrichment` by normalized name — see the join note under Claude's Discretion).

### The search being extended
- `internal/backendsrv/store/itemsearch.go` — `SearchCatalog` over `pigparse_price` (name/id
  LIKE, ESCAPE, LIMIT 25, V7 no-PII-logging); the catalog-scope + wishlist-add-search read to
  extend with facet params.
- `internal/backendsrv/readapi/itemsearch.go` — `GET /api/v1/items/search` handler (session-
  gated, 2-rune guard, DoS LIMIT); add facet query params here.
- `internal/backendsrv/compute/itemrollup.go` + `internal/backendsrv/readapi/items.go` +
  `GET /api/v1/items` — the **holdings** item list (`compute.Items`, one row per held item by
  normalized name + per-holder detail + the `ItemMasterIconStats` lookup). The Inventory tab's
  data source; add `is_clicky`/`has_haste` to its payload for the client-side holdings facet.
- `internal/backendsrv/store/enrich.go` — `item_master` read helpers (`ItemMasterIconStats` /
  `GetItemMasterFreshnessTx`) — where the `is_clicky`/`has_haste` columns (migration 00016)
  are surfaced.
- The Phase 29 normalized-name join helper in `internal/backendsrv/compute` — the catalog↔
  `item_master` bridge for the catalog-scope facet (search `compute` for the name-norm fn).

### Web (the UI surface)
- `web/src/routes/inventory/+page.svelte` + `web/src/lib/items.ts` — the Inventory tab
  (master-detail, viewer-first, the existing search box); add the facet toggles + scope toggle.
- `web/src/lib/api.ts` — `fetchItems` / `searchCatalog` wrappers + the `ItemRollup`/`CatalogItem`
  interfaces (mirror the Go contract; add the facet params + the new booleans).
- `web/src/lib/components/ExaminePanel.svelte` (P31) — reuse for the catalog-scope row examine.
- `web/src/routes/wishlist/+page.svelte` / the wishlist add form — the second facet surface
  (catalog-only; facets narrow `searchCatalog`).

### Conventions
- `CLAUDE.md` — structured `slog`, the single-tested-SQL-path rule, no `{@html}` sinks /
  escape note for the web examine render, extend-only.

### Memory (cross-session facts)
- `pigparse-vs-ingame-item-id-namespaces` — catalog ids ≠ EQ inventory ids; join by normalized
  name. **Load-bearing for the catalog-scope facet join.**
- `consolidated-views-relaxed-v2.4` (`project_consolidated_views`) — per-character master-detail
  allowed; the guild-wide Inventory grid stays consolidated.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`compute.Items` (holdings rollup)** — already the Inventory tab's data source with a
  per-item `item_master` lookup; adding `is_clicky`/`has_haste` to that payload makes the
  holdings facet a pure client-side filter (no new query, SC-4-safe).
- **`SearchCatalog` + `GET /api/v1/items/search`** — the catalog search (used by the wishlist
  add form today); the catalog-scope + wishlist-add facets extend this one read with optional
  `clicky`/`haste` params (server-side, LIMIT-capped).
- **`ExaminePanel` (P31)** — the catalog-scope row examine reuses it (flags/effects/price/wiki/
  last-listed); no new examine UI.
- **The P29 normalized-name join** — the catalog↔`item_master` bridge for catalog facet flags.

### Established Patterns
- **Holdings facet = client-side, catalog facet = server-side** — the natural split given the
  holdings rollup is already fully loaded client-side while the catalog is large + LIMIT-capped.
- **V7 no-PII search logging + 2-rune guard + ESCAPE'd LIKE** — preserve when adding facet params.

### Integration Points
- The facets READ Phase 37's `item_master.is_clicky`/`has_haste` (00016 columns). The catalog
  scope's "what exists" coverage depends on Phase 38's catalog enrichment — see the deploy note.
</code_context>

<specifics>
## Specific Ideas

- The toggle is a "lens over the same search" — typing "cloak", toggling Clicky, then flipping
  Holdings→Catalog should show "clicky cloaks that exist" without re-typing (D-03 persistence).
- Catalog "what exists" is a SUPERSET of "who has one": a held item shown in catalog scope keeps
  its holders; the difference is catalog scope ALSO lists items nobody holds (D-04).
</specifics>

<deferred>
## Deferred Ideas

- **"Pin to wishlist" from catalog search** (the discovery→wishlist shortcut, D-04) — natural
  fast-follow; not this phase.
- **Facets beyond Clicky/Haste** (slot, stat thresholds, weapon type) — REQUIREMENTS "Future
  Requirements"; the parsed-stat backbone makes each an incremental later add.

</deferred>

---

*Phase: 39-faceted-item-search-clicky-haste-scope-toggle*
*Context gathered: 2026-06-25*
