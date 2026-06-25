# Phase 39: Faceted item search (Clicky / Haste + scope toggle) - Research

**Researched:** 2026-06-25
**Domain:** Go read-API faceting + SQLite name-keyed UNION join (`item_master ∪ catalog_enrichment`) + SvelteKit 5 client-side facet filter + scope state
**Confidence:** HIGH

## Summary

Phase 39 turns the existing flat name/id item search into a discovery tool by adding two AND-combined facets (Clicky, Haste) and a Holdings↔Catalog scope toggle. Every piece of the data plumbing already exists in the codebase — this phase **adds facet parameters/payload fields, NOT new tables or migrations.** Phase 37 (migration 00016) put `is_clicky`/`has_haste`/`haste_pct`/`clicky_effect` on `item_master` (held items, keyed by EQ `item_id`). Phase 38 (migration 00017) put the SAME columns on the new `catalog_enrichment` table (catalog-only items, keyed by `norm_name = lower(trim(name))`). Both are already populated by the weekly wiki job [VERIFIED: `enrich/jobs/wiki.go:280-411` `upsertItemAndQuests`].

The natural implementation split (already anticipated by CONTEXT D-disc + UI-SPEC §1) is: **holdings facet = CLIENT-side** over the already-loaded `ItemRollup[]` (add two booleans to the rollup payload from the existing `ItemMasterIconStats` lookup — SC-4-safe, no catalog dependency); **catalog facet = SERVER-side** filter params on `store.SearchCatalog` + the `/api/v1/items/search` handler, joining each `pigparse_price` row to its flags via the **name-keyed `item_master ∪ catalog_enrichment` union** that Phase 38 already ships as `store.CatalogIconCoverage` [VERIFIED: `store/itemids.go:177-235`]. That UNION-ALL-over-two-tables-keyed-by-normalized-name is the load-bearing precedent — the catalog facet join is the same shape with the flag columns instead of `icon_id`.

**Primary recommendation:** Add `is_clicky`/`has_haste` to `compute.ItemRollup` (from a widened `ItemMasterIconStats`) for the client-side holdings facet; add optional `clicky bool`/`haste bool` params to `SearchCatalog` whose predicate filters `pigparse_price` rows via a `JOIN` to a `lower(trim(name))`-keyed `item_master ∪ catalog_enrichment` flag union; surface the catalog scope on the Inventory tab as the SAME `searchCatalog` read enriched with a per-name holders lookup from `compute.Items`. **No new migration. Watcher untouched. No `v*` tag.**

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Holdings facet (Clicky/Haste over held items) | Browser/Client (Svelte) | API (adds 2 booleans to payload) | The full holdings rollup is already loaded client-side; filtering it needs no new query and CANNOT block on the catalog crawl (SC-4). Facet flags ride on the rollup from `item_master`. |
| Catalog facet (Clicky/Haste over full catalog) | API/Backend (Go store + handler) | DB (name-keyed flag union) | The catalog is ~4,343 rows + LIMIT-capped; the flag predicate is a SQL join. Pushing it server-side keeps the wire small + the LIKE/ESCAPE/V7 discipline in one tested SQL path. |
| Scope toggle (Holdings↔Catalog) | Browser/Client (Svelte state) | API (the two reads it switches between) | The toggle is a client lens that swaps the data source between `fetchItems` (holdings) and `searchCatalog` (catalog) while persisting query + facets. |
| Catalog-scope holder list (held → holders; unheld → "not held") | API/Backend (reuse `compute.Items` by name) | Browser/Client (join) | A catalog row's holders come from the existing name-keyed holdings rollup; unheld → empty. No new holders query. |
| Wishlist add-form facets (catalog-only) | API/Backend (same `searchCatalog`) | Browser/Client (facet chips) | Second surface for the SAME catalog facet params; no scope toggle there (D-01). |

## Standard Stack

This phase introduces **NO new libraries.** It composes the existing backend (Go 1.24 stdlib `database/sql` + `net/http` ServeMux) and web (SvelteKit + Svelte 5 runes + Tailwind v4 + `@lucide/svelte`) exactly as the surrounding code does.

### Core (already in the tree — versions confirmed against the repo, not the registry)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `database/sql` + `modernc.org/sqlite` | (in `go.mod`) | The single tested SQL path for the catalog facet join | The whole `store/` package is plain `(*Store)` read methods over `s.db.QueryContext` [VERIFIED: `store/itemsearch.go:61`] |
| Go 1.22+ `net/http.ServeMux` | Go 1.24 | `GET /api/v1/items/search` + `GET /api/v1/items` route patterns | `mux.Handle("GET /api/v1/items/search", …)` [VERIFIED: `cmd/squirebot-server/main.go:367`] |
| Svelte 5 runes (`$state`/`$derived`/`$effect`) | (in `web/package.json`) | The scope/facet/query reactive state on the Inventory tab | The existing tab is all Svelte-5 runes [VERIFIED: `web/src/routes/inventory/+page.svelte:45-49`] |
| `@lucide/svelte` (`Search` glyph) | (vendored) | The search field icon — already imported | UI-SPEC Design System row |

**Version verification:** Not applicable — this phase adds NO new dependency. All code reuses already-vendored versions. (Running `npm view`/`go list` for a new package is unnecessary; confirm `go.mod`/`web/package.json` are untouched in the plan.)

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Client-side holdings facet | A new server-side `clicky`/`haste` param on `GET /api/v1/items` | Breaks SC-4 (would couple holdings faceting to a flag join) + re-fetches on every chip toggle; the rollup is already fully loaded, so client-side is strictly better. REJECT. |
| Reuse `SearchCatalog` for the catalog scope on the Inventory tab | A brand-new full-catalog read method | `SearchCatalog` already does name/id LIKE + ESCAPE + LIMIT + V7 logging; the facet params + the holders enrichment are the only deltas. Reuse. |
| Joining catalog flags by `item_id` | (the namespace trap) | `pigparse_price.item_id` is the PigParse namespace; `item_master.item_id` is the EQ namespace — they collide numerically. MUST join by `lower(trim(name))` [VERIFIED: memory `pigparse-vs-ingame-item-id-namespaces`; `store/itemids.go:84-87`]. |

**Installation:** none.

## Architecture Patterns

### System Architecture Diagram

```
                    HOLDINGS SCOPE (client-side facet, SC-4)            CATALOG SCOPE (server-side facet)
                    ───────────────────────────────────────            ─────────────────────────────────
  Inventory tab  ──▶ GET /api/v1/items                                 GET /api/v1/items/search?q=&clicky=&haste=
  +page.svelte       │                                                  │
   (scope state)     ▼                                                  ▼
                  compute.Items(ctx, store, viewerID)                readapi.ItemSearch.ServeHTTP
                     │  itemrollup.go:38                                │  (2-rune guard, V7 log, LIMIT 25)
                     ├─ compute.View ........... name-bridged price     ▼
                     ├─ store.RosterFor ........ is_mine / bank flags  store.SearchCatalog(ctx, q, clicky, haste, limit)
                     └─ store.ItemMasterIconStats                       │  itemsearch.go
                        (icon + stats + ★is_clicky + ★has_haste★)       │  SELECT … FROM pigparse_price pp
                     │                                                  │  ★JOIN (name-keyed flag union) f
                     ▼                                                  │     ON f.norm = lower(trim(pp.name))★
                  ItemRollup[]  ──(client filter over rows)──▶          │  WHERE name/id LIKE …
                     │   items.ts filterItems + ★facetItems★            │    ★AND (f.is_clicky) AND (f.has_haste)★
                     ▼                                                  ▼
                  rendered list (no new fetch on toggle)             CatalogItem[]  ──▶ client renders rows;
                                                                       held name → holders (from compute.Items by name)
                                                                       unheld    → "not held in the guild"

  The flag union (the load-bearing join, server-side ONLY):
     SELECT lower(trim(name)) AS norm, is_clicky, has_haste FROM item_master           ← held, EQ-id-keyed
     UNION ALL
     SELECT norm_name           AS norm, is_clicky, has_haste FROM catalog_enrichment   ← catalog-only, norm_name-keyed
  (a held name is NEVER in catalog_enrichment — the write-path dedup, itemids.go:177-191 — so no precedence logic needed)
```

### Recommended changes (file-level — what each task touches)
```
internal/backendsrv/
├── store/itemsearch.go        # SearchCatalog gains clicky/haste params + the name-keyed flag JOIN
├── store/itemsearch_test.go   # facet cases: clicky-only, haste-only, both, held-flag vs catalog-flag
├── store/readviews.go         # ItemMasterIconStats widened: add IsClicky/HasHaste to IconStats (or a sibling read)
├── readapi/itemsearch.go      # parse ?clicky=&haste= bool params; pass to SearchCatalog; V7 log unchanged
├── readapi/itemsearch_test.go # handler facet param cases
└── compute/types.go           # ItemRollup gains is_clicky / has_haste JSON fields (append-only)
   compute/itemrollup.go       # copy the two booleans from the iconStats map into the rollup
   compute/itemrollup_test.go  # assert the two booleans propagate

web/src/lib/
├── api.ts                     # ItemRollup += is_clicky/has_haste; CatalogItem += is_clicky/has_haste;
│                              #   searchCatalog(q, {clicky,haste}); (fetchItems unchanged signature)
├── items.ts                   # facetItems(rows, {clicky,haste}) pure helper (node-tested)
└── components/FacetBar.svelte  # (optional) the two facet chips, prop-driven (UI-SPEC Reuse Note)

web/src/routes/
├── inventory/+page.svelte     # scope state (holdings|catalog) + facet state + the catalog read + render
└── wishlist/+page.svelte      # add the facet chips to the add-form catalog search (no scope toggle)
```

### Pattern 1: Widen the id-correct `item_master` lookup with the facet booleans (holdings facet)
**What:** `ItemMasterIconStats` already returns a `map[int64]IconStats` keyed by EQ `item_id`. Add `IsClicky`/`HasHaste` to `IconStats` and select the two extra columns. `buildItemRollups` already does `ic := iconStats[vr.ID]` — copy the two booleans onto the rollup.
**When to use:** This is the ONLY holdings-side data change. It keeps the holdings facet a pure client filter (SC-4: zero catalog dependency).
**Example:**
```go
// Source: internal/backendsrv/store/readviews.go:759-793 (extend IconStats + the SELECT)
type IconStats struct {
	IconID     int64
	Statsblock string
	IsClicky   bool // ★ Phase 39 — 00016 column, EQ-namespace correct here
	HasHaste   bool // ★ Phase 39
}
// SELECT item_id, icon_id, statsblock, is_clicky, has_haste FROM item_master
//   scan is_clicky/has_haste via sql.NullInt64 → != 0 (a pre-00016/NULL row is false)
```
```go
// Source: internal/backendsrv/compute/itemrollup.go:73-82 (already does ic := iconStats[vr.ID])
roll = &ItemRollup{
	// …existing fields…
	IconID:     ic.IconID,
	Statsblock: ic.Statsblock,
	IsClicky:   ic.IsClicky, // ★
	HasHaste:   ic.HasHaste, // ★
}
```
```ts
// Source: web/src/lib/items.ts (sibling of filterItems — pure, node-tested)
export function facetItems(
	rows: ItemRollup[],
	f: { clicky: boolean; haste: boolean }
): ItemRollup[] {
	return rows.filter(
		(r) => (!f.clicky || r.is_clicky) && (!f.haste || r.has_haste) // AND-combined; neither set = pass-through (D-02)
	);
}
// inventory/+page.svelte: let shown = $derived(facetItems(filterItems(items, query), { clicky, haste }));
```

### Pattern 2: Name-keyed flag union join for the catalog facet (server-side)
**What:** `SearchCatalog` gains `clicky bool, haste bool` params. When either is true, it joins each `pigparse_price` row to its flags via the `item_master ∪ catalog_enrichment` union keyed by `lower(trim(name))`, then filters on the requested booleans (AND-combined). The existing name/id LIKE + ESCAPE + ORDER BY + LIMIT discipline is preserved verbatim.
**When to use:** Both catalog surfaces — the Inventory-tab catalog scope AND the wishlist add-form.
**Example:**
```go
// Source: internal/backendsrv/store/itemsearch.go:61-98 (extend SearchCatalog; the union mirrors
//         CatalogIconCoverage at store/itemids.go:193-205, swapping icon_id for is_clicky/has_haste)
func (s *Store) SearchCatalog(ctx context.Context, q string, clicky, haste bool, limit int) ([]CatalogItem, error) {
	like := "%" + escapeLike(q) + "%"
	prefix := escapeLike(q) + "%"

	// Build the optional facet predicate. The flags come from the name-keyed union of the
	// TWO enrichment stores (held item_master by lower(trim(name)) ∪ catalog_enrichment by
	// norm_name). A held name is NEVER in catalog_enrichment (the write-path dedup,
	// itemids.go:177-191), so UNION ALL counts each name once — no precedence logic.
	// item_id is NEVER the join key (PigParse vs EQ namespace).
	const flagUnion = `
	  LEFT JOIN (
	    SELECT lower(trim(name)) AS norm, is_clicky, has_haste FROM item_master
	    UNION ALL
	    SELECT norm_name          AS norm, is_clicky, has_haste FROM catalog_enrichment
	  ) f ON f.norm = lower(trim(pigparse_price.name))`

	var facet strings.Builder
	if clicky { facet.WriteString(" AND COALESCE(f.is_clicky,0) = 1") }
	if haste  { facet.WriteString(" AND COALESCE(f.has_haste,0) = 1") }

	query := "SELECT pigparse_price.item_id, pigparse_price.name, pigparse_price.current_avg " +
		"FROM pigparse_price"
	if clicky || haste { query += flagUnion } // join ONLY when a facet is active (keep the no-facet path identical)
	query += " WHERE (name LIKE ? ESCAPE '\\' OR CAST(pigparse_price.item_id AS TEXT) = ?)" +
		facet.String() +
		" ORDER BY (name LIKE ? ESCAPE '\\') DESC, length(name), name COLLATE NOCASE LIMIT ?"

	rows, err := s.db.QueryContext(ctx, query, like, q, prefix, limit)
	// …existing scan loop, sql.NullString name, sql.NullFloat64 avg, len(q)-only error wrap (V7)…
}
```
**Anti-pattern guard:** Do NOT `INNER JOIN` — a catalog row with no enrichment row yet (coverage still filling) must still appear when NO facet is active. The join is added ONLY when a facet is on, and a missing flag row `COALESCE`s to 0 (correctly excluded by an active facet).

### Pattern 3: The scope toggle as a client lens (persisting query + facets)
**What:** Add `scope: 'holdings' | 'catalog'` state. Holdings scope renders `facetItems(filterItems(items, query), …)` over the already-loaded `ItemRollup[]`. Catalog scope debounce-calls `searchCatalog(query, {clicky, haste})` and renders catalog rows. Toggling scope NEVER clears `query` or the facet booleans (D-03) — it just re-runs against the other source.
**When to use:** Inventory tab only (the wishlist add-form is catalog-only — facets, no scope).
**Example:**
```ts
// Source: web/src/routes/inventory/+page.svelte (extend the existing $state block at :45-49)
let scope = $state<'holdings' | 'catalog'>('holdings'); // D-03 default Holdings
let clicky = $state(false);
let haste = $state(false);
let catalogRows = $state<CatalogItem[]>([]);
// holdings: pure client filter (no new fetch). catalog: debounced searchCatalog (the wishlist
// add-form debounce idiom at wishlist/+page.svelte:397-440 is the clone source).
// setScope(next) { scope = next; }  ← does NOT touch query/clicky/haste (the "lens, not reset" contract)
```
**Catalog-scope holders (D-04):** a catalog row that IS held shows its holders; reuse `compute.Items` by name client-side — build a `Map<normName, ItemRollup>` from the already-fetched `items` and look the catalog row up by `name.toLowerCase().trim()`. Held → render its `holders[]`; absent → "not held in the guild". No new holders endpoint.

### Anti-Patterns to Avoid
- **Joining catalog flags by `item_id`:** the #1 trap this phase exists to avoid. `pigparse_price.item_id` ≠ `item_master.item_id` (different namespaces; numerically collide). ALWAYS `lower(trim(name))` [VERIFIED: `store/itemids.go:84-87`, memory `pigparse-vs-ingame-item-id-namespaces`].
- **A server-side `clicky`/`haste` param on `GET /api/v1/items` (holdings):** breaks SC-4 + re-fetches on toggle. Keep holdings faceting client-side.
- **Adding a precedence/COALESCE between the two flag tables:** unnecessary — a held name is never in `catalog_enrichment` (write-path dedup), so `UNION ALL` already counts each name once [VERIFIED: `store/itemids.go:177-191`].
- **An `INNER JOIN` to the flag union:** would drop catalog rows lacking an enrichment row when no facet is active. Conditionally add the `LEFT JOIN` only when a facet is on.
- **A new `{@html}` sink for catalog rows:** the catalog-row examine REUSES `ExaminePanel` (the one sanctioned escaped sink). Plain `{}` interpolation everywhere else [VERIFIED: UI-SPEC Copywriting; `inventory/+page.svelte:18-21`].

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Catalog row → flags lookup | A new id-keyed join or a synthetic id map | The name-keyed `item_master ∪ catalog_enrichment` UNION (Pattern 2) | Phase 38 already ships this exact shape as `CatalogIconCoverage` (`store/itemids.go:193`); copy it, swap `icon_id`→`is_clicky,has_haste` |
| LIKE injection / wildcard escaping | A custom sanitizer | The existing `escapeLike` + `ESCAPE '\'` + `?` bind | Already battle-tested in `SearchCatalog` (`store/itemsearch.go:44-53`); preserve verbatim |
| Catalog-row examine UI | A new detail component | The reused `ExaminePanel` + the `asSlot` seam | `inventory/+page.svelte:104-125` already builds a representative `InventorySlot` for the holdings examine; the catalog row uses the identical seam |
| Catalog-row holders | A new holders endpoint/query | `compute.Items` (already loaded) keyed by normalized name | The holdings rollup is already name-keyed (`itemrollup.go:69`); a `Map<norm,rollup>` lookup gives held→holders for free |
| Facet chips on two surfaces | Two copies hand-styled | A small prop-driven `FacetBar.svelte` (or inline `.facet` block) | UI-SPEC Reuse Note: extract once, prop `clicky`/`haste`/`onToggle` |
| Debounced catalog search | A new debounce | The wishlist add-form debounce (`wishlist/+page.svelte:397-440`) | Identical staging+seq-guard idiom; clone it for the catalog-scope search |

**Key insight:** Phase 38's name-keyed re-plan was specifically engineered so this phase's catalog↔flags join is a clean name-to-name join end-to-end (the "namespace-bridge hazard" the original CONTEXT flagged is dissolved). Reuse `CatalogIconCoverage`'s UNION shape rather than inventing a new join.

## Runtime State Inventory

> Not a rename/refactor/migration phase (additive read params + payload fields only). This section is included for completeness because the phase touches the search read path.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `item_master.is_clicky/has_haste` (00016) + `catalog_enrichment.is_clicky/has_haste` (00017) already EXIST + are populated by the weekly wiki job | None — Phase 39 READS them. NO new migration. |
| Live service config | None | None — verified: no n8n/Datadog/external config references item facets |
| OS-registered state | None | None — backend + web only |
| Secrets/env vars | None | None — no new endpoint secret; `/items/search` is already session-gated |
| Build artifacts | None | None — no package rename; `go test ./...` + `npm run build` only |

**Deploy-coverage note (NOT a blocker):** `catalog_enrichment` was seeded EMPTY on the 37+38 deploy; it populates on the next **Sunday UTC** wiki crawl [VERIFIED: STATE.md:41]. So **catalog-scope facets over UNHELD items return sparse/empty until that crawl runs** — the holdings facet works immediately (it reads `item_master`, backfilled live: 16/953 rows flagged). The UI-SPEC already specs the "catalog still filling in" empty-state copy. This is graceful degradation, not a bug.

## Common Pitfalls

### Pitfall 1: Joining the catalog facet by item_id (the namespace trap)
**What goes wrong:** A clicky filter returns the wrong items or none, because `pigparse_price.item_id` (PigParse namespace) was joined to `item_master.item_id` (EQ namespace) — they collide numerically.
**Why it happens:** It's the "obvious" join and CLAUDE.md historically (wrongly) called `item_id` the stable join key; the memory `pigparse-vs-ingame-item-id-namespaces` corrects that.
**How to avoid:** Join the flag union by `lower(trim(name))` ONLY (Pattern 2). The `escapeLike`/`pp_rep` precedent and `CatalogIconCoverage` both do this.
**Warning signs:** A facet test where a catalog item's flag doesn't match its held twin; a clicky filter that returns items with a numerically-coincident EQ id.

### Pitfall 2: Holdings facet coupled to the catalog crawl (SC-4 violation)
**What goes wrong:** Clicky/Haste over held items returns nothing until the Sunday catalog crawl completes.
**Why it happens:** Putting the holdings facet server-side and reading flags from `catalog_enrichment` instead of from the already-populated `item_master` rollup.
**How to avoid:** Holdings facet is CLIENT-side over `ItemRollup.is_clicky/has_haste`, which come from `item_master` (held, backfilled live). Never touches `catalog_enrichment`.
**Warning signs:** A test that needs `catalog_enrichment` seeded to assert a holdings facet; the holdings facet failing right after deploy.

### Pitfall 3: INNER-joining the flag union (dropping unenriched catalog rows)
**What goes wrong:** With NO facet active, catalog rows that have no enrichment row yet vanish from the result.
**Why it happens:** Always-on `INNER JOIN` to the flag union.
**How to avoid:** Add the `LEFT JOIN` ONLY when a facet is on; `COALESCE(f.is_clicky,0)=1` excludes unenriched rows correctly when the facet is active, and the no-facet path keeps the exact original single-table query.
**Warning signs:** The catalog row count drops when no facet is selected; a regression in the existing `SearchCatalog` tests.

### Pitfall 4: Breaking the V7 no-PII search logging / 2-rune guard / DoS LIMIT
**What goes wrong:** The query string or facet values leak into logs, or the short-query short-circuit / LIMIT 25 is lost.
**Why it happens:** Refactoring the handler while adding `?clicky=&haste=`.
**How to avoid:** Preserve `readapi/itemsearch.go:46-75` verbatim except the two bool-param parses + the `SearchCatalog` call; the slog line stays `rows + qlen + status` (add `clicky`/`haste` booleans — they are NOT PII — but NEVER the `q` string). Keep the `utf8.RuneCountInString(q) < 2` short-circuit BEFORE the store hit and `searchLimit = 25`.
**Warning signs:** A log line containing the query text; a 1-rune query hitting the DB.

### Pitfall 5: web vitest is node-only — green tests ≠ working facet UI
**What goes wrong:** The facet/scope render or the debounced catalog fetch is broken in the browser despite green `npm test`.
**Why it happens:** No `@testing-library/svelte` (toolchain-install rule); node vitest can't see the DOM [VERIFIED: memory `web-tests-node-only-blind-to-dom`].
**How to avoid:** Keep the facet/filter logic in pure node-tested helpers (`items.ts facetItems`), and gate the phase on a deploy-then-browser-smoke for the scope toggle + catalog render (the P32/P34 precedent).
**Warning signs:** A "verified" frontend with no browser-smoke; logic inlined in `.svelte` instead of `items.ts`.

## Code Examples

### Handler: parse the facet params, preserve V7 + the guard
```go
// Source: internal/backendsrv/readapi/itemsearch.go:46-75 (extend; everything else unchanged)
q := strings.TrimSpace(r.URL.Query().Get("q"))
if utf8.RuneCountInString(q) < 2 {           // unchanged 2-rune guard BEFORE any DB hit
	_ = json.NewEncoder(w).Encode([]store.CatalogItem{})
	return
}
clicky := r.URL.Query().Get("clicky") == "1" // additive bool params (or "true"; pick one + test it)
haste := r.URL.Query().Get("haste") == "1"
items, err := h.st.SearchCatalog(r.Context(), q, clicky, haste, searchLimit)
// …unchanged: nil→[] coercion, V7 slog (add clicky/haste booleans; NEVER the q string), encode…
slog.Info("item search ok", "rows", len(items), "qlen", utf8.RuneCountInString(q),
	"clicky", clicky, "haste", haste, "status", http.StatusOK)
```

### api.ts: append-only interface fields + the param wrapper
```ts
// Source: web/src/lib/api.ts:237-251 (ItemRollup) + :751-761 (CatalogItem/searchCatalog)
export interface ItemRollup { /* …existing… */ is_clicky: boolean; has_haste: boolean; } // ★ append-only
export interface CatalogItem { item_id: number; name: string; current_avg?: number;
	is_clicky?: boolean; has_haste?: boolean; }                                            // ★ append-only
export function searchCatalog(
	q: string, facets: { clicky?: boolean; haste?: boolean } = {}, f: typeof fetch = fetch
): Promise<CatalogItem[]> {
	const p = new URLSearchParams({ q });
	if (facets.clicky) p.set('clicky', '1');
	if (facets.haste) p.set('haste', '1');
	return getJSON<CatalogItem[]>('/api/v1/items/search?' + p.toString(), f);
}
// NOTE: searchCatalog is CALLED by the wishlist add-form (wishlist/+page.svelte:431) — update that
// call site (it passes only q today; the new 2nd arg defaults to {}, so it stays compiling) AND
// add the facet chips there (D-01: facets narrow the add suggestions; NO scope toggle).
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Catalog-only enrichment id-keyed into `item_master` (Option A) | A separate `catalog_enrichment` table keyed by `norm_name` (Option B, name-keyed) | Phase 38, 2026-06-25 | The catalog facet join is name-to-name end-to-end; CONTEXT's stale "Option A id-keyed" reference is OVERRIDDEN (see Assumptions). |
| `SearchCatalog` = name/id LIKE only | + optional `clicky`/`haste` facet params via the name-keyed flag union | This phase | The first faceted catalog read |
| Inventory tab = holdings only | + a Holdings↔Catalog scope toggle (client lens) | This phase | "what exists" vs "who has one" |

**Deprecated/outdated:**
- **CONTEXT.md `### Claude's Discretion` + the `38-…` canonical ref describing "Option A — catalog-only `item_master` rows keyed by PigParse id":** STALE. Phase 38 shipped name-keyed (`catalog_enrichment`, migration 00017). Use the name-keyed union (Pattern 2). [VERIFIED: STATE.md:41-43; `store/catalogenrich.go`; `migrations/00017_catalog_enrichment.sql`]

## Project Constraints (from CLAUDE.md)

The planner MUST verify the plan honors these (same authority as locked decisions):
- **Extend-only schema:** add columns at the right edge / add tables / add `_meta` rows. **This phase adds NO migration** — it reads 00016 + 00017 columns and adds read params + payload fields. CONFIRMED: no new persisted state.
- **Single tested SQL path (11-05 WARNING-3):** the catalog facet SQL lives in `store/itemsearch.go` ONLY; `readapi`/`compute` author ZERO inline SQL. The handler calls `SearchCatalog`; the holdings booleans come from `store.ItemMasterIconStats`.
- **Structured slog, no PII (V7):** the search log carries `rows + qlen + clicky + haste + status` — NEVER the `q` string.
- **No `{@html}` sinks:** all new render is plain `{}` interpolation; the catalog-row examine reuses the one sanctioned `ExaminePanel`/`composeItemNote` escaped sink.
- **Structured logging both sides:** Go `slog`; web has no new log surface.
- **Watcher untouched → no `v*` tag:** backend + web only. CONFIRMED (a `v*` tag fires the watcher CI; the watcher does not change).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `?clicky=1`/`?haste=1` encoding is a free planner choice (could be `=true`); pick ONE and test it | Code Examples / Pitfall 4 | LOW — purely internal contract between `api.ts` and the handler; a mismatch is caught by the handler test |
| A2 | The catalog-scope holders for a held item can be sourced client-side from the already-loaded `compute.Items` rollup (name-keyed) rather than a new endpoint | Pattern 3 / Don't Hand-Roll | LOW — the rollup is already fetched on the Inventory tab; if a future requirement needs catalog-scope holders WITHOUT loading holdings, a server join would be needed (not this phase) |
| A3 | `FacetBar.svelte` extraction is optional (UI-SPEC Reuse Note) — inlining the `.facet` block on both surfaces is acceptable | Recommended changes | NONE — UI-SPEC explicitly allows either |

**No `[ASSUMED]` factual claims about library/DB behavior:** every SQL/column/route/file claim in this research is `[VERIFIED]` against the repo (file:line anchored). The three assumptions above are implementation-choice latitude, not unverified facts.

## Open Questions (RESOLVED)

> Both questions were resolved at planning time (2026-06-25); the chosen defaults are bound into 39-01-PLAN.md `must_haves` truths #4/#5 + Task 2/3 actions + threat T-39-04/08. No unresolved decision reaches the executor.

1. **Catalog-scope sort/ranking parity with holdings viewer-first.**
   - What we know: holdings is viewer-first then A-Z (`items.ts viewerFirstItems`); `SearchCatalog` is prefix-first then length/name (`itemsearch.go:67`).
   - What's unclear: whether the catalog scope on the Inventory tab should re-sort viewer-first (it can't fully — catalog rows aren't is_mine-stamped) or keep `SearchCatalog`'s prefix-first ranking.
   - Recommendation: keep `SearchCatalog`'s existing prefix-first ranking for catalog scope (it's the established catalog ordering + the wishlist add-form already shows it that way); the UI-SPEC's worked example ("clicky cloaks that exist") doesn't require viewer-first. The planner/UI can confirm.
   - **RESOLVED: keep `SearchCatalog`'s prefix-first ordering — NOT re-sorted viewer-first** (39-01 must_have truth #4, Task 2 action).

2. **Empty `q` in catalog scope (browse-all-catalog vs require a query).**
   - What we know: `SearchCatalog`/the handler short-circuit to `[]` for `q < 2 runes`.
   - What's unclear: whether catalog scope with an empty query + an active facet should "browse all clickies" (no `q`) or keep the 2-rune guard.
   - Recommendation: keep the 2-rune guard (the DoS bound is real — the LIKE is a full scan; the LIMIT helps but an empty-q facet-only scan over 4,343 rows is still a scan). If "browse all clickies" is desired, that's a deliberate, separately-bounded read — flag to discuss, don't assume. The UI-SPEC's flows all start from a typed query.
   - **RESOLVED: keep the 2-rune guard — empty/short query returns `[]` even with a facet active, no full-corpus dump** (39-01 must_have truth #5, Task 3, threat T-39-04/08).

## Environment Availability

> The phase is backend (Go) + web (SvelteKit) changes over an existing toolchain. No NEW external dependency.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (`go test ./...`) | Backend facet store/handler + tests | ✓ (repo builds today) | 1.24 (go.mod) | — |
| Node/npm (`npm run build`, `npm test`) | Web facet/scope + node-tested helpers | ✓ (web/ builds today) | per web/package.json | — |
| `catalog_enrichment` populated | Catalog-scope facets over UNHELD items | ✗ (seeded empty; fills next Sunday UTC crawl) | — | Holdings facets + held-item catalog facets work immediately; UNHELD catalog facets sparse until the crawl (graceful, UI-SPEC empty-state) |

**Missing dependencies with no fallback:** none — the phase ships and is testable without the Sunday crawl (tests seed `catalog_enrichment` directly).
**Missing dependencies with fallback:** the live catalog-enrichment coverage — holdings path is fully functional day one (SC-4); the catalog scope's unheld coverage fills in post-crawl.

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high` (config.json). The facet read extends an EXISTING session-gated, no-PII, parameterized search.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No new auth; both routes already `RequireSession` (`main.go:367,386`) |
| V3 Session Management | no | Reuses the existing cookie session; no change |
| V4 Access Control | yes | The catalog + holdings reads are guild-wide membership-gated (NOT ownership-scoped) — preserve `RequireSession`, NEVER `RequireOfficer` (`items.go:11-13`) |
| V5 Input Validation | yes | `q` stays `?`-bound + `escapeLike` + `ESCAPE '\'` (never concatenated); facet params are coerced to `bool` (no string reaches SQL) — they bind nothing, they only toggle a fixed predicate fragment |
| V6 Cryptography | no | None |
| V7 Logging (no PII) | yes | The slog line carries `qlen`/`rows`/`clicky`/`haste`/`status` — NEVER the `q` value (`itemsearch.go:60-69`) |
| V12/V13 (DoS) | yes | The 2-rune guard + `LIMIT 25` cap the full-scan LIKE; the facet `LEFT JOIN` is over the small `item_master ∪ catalog_enrichment` union, bounded by the same LIMIT |

### Known Threat Patterns for the Go/SQLite + Svelte stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via `q` or facet | Tampering | `q` is `?`-bound + `escapeLike` + `ESCAPE '\'`; facets are Go `bool` that select a FIXED predicate fragment (`AND COALESCE(f.is_clicky,0)=1`) — no user string is interpolated into SQL [VERIFIED: `itemsearch.go:44-69`] |
| Search-query PII leak in logs | Information Disclosure | Log `qlen`, never `q` (V7) — preserve [VERIFIED: `itemsearch.go:9-13,60-69`] |
| DoS via unbounded LIKE scan | DoS | 2-rune guard + `LIMIT 25` — preserve [VERIFIED: `itemsearch.go:9-13,27`] |
| XSS via catalog item names in the new rows | Tampering/Info-Disclosure | Plain Svelte `{}` auto-escape; the only escaped sink is the reused `ExaminePanel` (scheme-allow-listed) — NO new `{@html}` [VERIFIED: UI-SPEC Copywriting; `inventory/+page.svelte:18-21`] |
| Privilege scope creep (catalog scope leaking non-member data) | Elevation/Info-Disclosure | Both reads are guild-wide membership reads; the catalog is public P99 catalog data — no per-owner scoping to breach. Keep `RequireSession`. |

## Sources

### Primary (HIGH confidence — all repo-internal, file:line anchored)
- `internal/backendsrv/store/itemsearch.go` — `SearchCatalog` (the catalog search to extend); `escapeLike`; V7 + LIMIT discipline
- `internal/backendsrv/store/itemids.go:177-235` — `CatalogIconCoverage` (the load-bearing `item_master ∪ catalog_enrichment` UNION-ALL precedent + the held-name dedup invariant); `DistinctEnrichmentRefs` (the namespace rationale)
- `internal/backendsrv/store/catalogenrich.go` — `catalog_enrichment` store (the name-keyed Phase-38 table carrying the facet columns)
- `internal/backendsrv/store/enrich.go:87-114,204-294` — `ItemMaster` shape + the 00016 facet columns + `GetItemMasterFreshnessTx`
- `internal/backendsrv/store/readviews.go:755-793` — `IconStats` + `ItemMasterIconStats` (the holdings-facet widen point)
- `internal/backendsrv/compute/itemrollup.go` + `compute/types.go:225-250` — `compute.Items`/`buildItemRollups`/`ItemRollup` (the holdings rollup to add the booleans to)
- `internal/backendsrv/readapi/itemsearch.go` + `readapi/items.go` — the two handlers (facet params on `/items/search`; `/items` unchanged)
- `internal/backendsrv/enrich/jobs/wiki.go:280-411` — `upsertItemAndQuests` (proves both stores' facet columns are populated, branched on `ref.Held`)
- `internal/backendsrv/migrations/00016_item_flags_effects.sql` + `00017_catalog_enrichment.sql` — the columns this phase reads (NO new migration)
- `web/src/lib/api.ts:237-251,751-761` — `ItemRollup`/`CatalogItem`/`searchCatalog` (the web contract to extend)
- `web/src/lib/items.ts` — `filterItems`/`viewerFirstItems` (the pure node-tested helper home for `facetItems`)
- `web/src/routes/inventory/+page.svelte` — the Inventory tab (scope/facet state + the `asSlot` ExaminePanel seam + holders render)
- `web/src/routes/wishlist/+page.svelte:397-455` — the add-form catalog search + debounce idiom (the second facet surface)
- `internal/backendsrv/store/itemsearch_test.go` + `readapi/itemsearch_test.go` + `compute/itemrollup_test.go` — the test seams to add facet/scope cases to
- `cmd/squirebot-server/main.go:367,386` — the route registrations (unchanged shape)
- `.planning/phases/39-…/39-CONTEXT.md`, `39-UI-SPEC.md`, `.planning/ROADMAP.md:440-453`, `.planning/REQUIREMENTS.md`, `.planning/STATE.md:41-45`

### Secondary (MEDIUM)
- Memory: `pigparse-vs-ingame-item-id-namespaces` (the name-join rule), `web-tests-node-only-blind-to-dom` (browser-smoke gate), `project_consolidated_views` (master-detail allowed)

### Tertiary (LOW)
- None — no external sources needed (this phase extends well-understood in-repo subsystems).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new libraries; every reused component is file:line-verified in the current tree
- Architecture (the name-keyed flag union + client/server split): HIGH — the union is a direct copy of the shipped `CatalogIconCoverage`; the split is anticipated by CONTEXT + UI-SPEC and matches the P32/P34 client-filter precedent
- Pitfalls: HIGH — the namespace trap, SC-4 coupling, V7 logging, and node-blind tests are all repo-documented (memory + code comments)
- Security: HIGH — extends an existing session-gated, parameterized, no-PII read; deltas are bool params + 2 payload booleans

**Validation Architecture:** SKIPPED — `workflow.nyquist_validation: false` in `.planning/config.json`.

**Research date:** 2026-06-25
**Valid until:** 2026-07-25 (stable — in-repo subsystems; the only time-sensitive fact is the Sunday-crawl catalog-enrichment population, which is a deploy-coverage note, not a code dependency)

## RESEARCH COMPLETE

**Phase:** 39 - Faceted item search (Clicky / Haste + scope toggle)
**Confidence:** HIGH

### Key Findings
- **No new migration.** Phase 39 READS Phase 37's `item_master` (00016) + Phase 38's `catalog_enrichment` (00017) facet columns; it adds read PARAMS + payload FIELDS only. Extend-only schema satisfied.
- **The load-bearing catalog↔flags join is name-keyed and already precedented:** `CatalogIconCoverage` (`store/itemids.go:193-205`) ships the exact `item_master ∪ catalog_enrichment` UNION-ALL keyed by `lower(trim(name))`; the catalog facet join copies it, swapping `icon_id` for `is_clicky`/`has_haste`. NEVER join by `item_id` (namespace trap).
- **Holdings facet = client-side (SC-4-safe):** add `is_clicky`/`has_haste` to `IconStats`/`ItemMasterIconStats` → `compute.ItemRollup` → `api.ts ItemRollup`; filter client-side via a pure `facetItems` helper in `items.ts` (node-tested). No new query, no catalog dependency.
- **Catalog facet = server-side:** optional `clicky`/`haste` bool params on `SearchCatalog` + `/api/v1/items/search`, preserving the 2-rune guard / `escapeLike`+ESCAPE / LIMIT 25 / V7 no-PII logging. The SAME read serves the Inventory-tab catalog scope AND the wishlist add-form (D-01).
- **Catalog-scope holders (D-04) reuse the already-loaded name-keyed `compute.Items` rollup** — held → its holders, unheld → "not held in the guild." No new holders endpoint.
- **STALE-REFERENCE CORRECTION carried into the research:** CONTEXT's "Option A id-keyed `item_master`" is overridden — Phase 38 shipped NAME-KEYED (`catalog_enrichment`, 00017).

### File Created
`.planning/phases/39-faceted-item-search-clicky-haste-scope-toggle/39-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | No new deps; every reused symbol file:line-verified in the current tree |
| Architecture | HIGH | Catalog join is a copy of the shipped `CatalogIconCoverage`; the client/server split matches P32/P34 precedent + CONTEXT/UI-SPEC |
| Pitfalls | HIGH | Namespace trap, SC-4, V7, node-blind tests all repo-documented |
| Security | HIGH | Extends an existing session-gated, parameterized, no-PII read |

### Open Questions (RESOLVED 2026-06-25 — bound into 39-01 plan)
- Catalog-scope ranking → **RESOLVED: keep `SearchCatalog`'s prefix-first order** (not re-sorted viewer-first; catalog rows aren't is_mine-stamped).
- Empty-`q` catalog scope → **RESOLVED: keep the 2-rune guard** (empty/short query returns `[]`, no "browse all clickies" corpus dump — the DoS bound).

### Ready for Planning
Research complete. The planner can map SEARCH-04/05/06 to: (1) backend holdings-rollup widen + client facet helper, (2) backend catalog-facet store/handler + the name-keyed union, (3) the Inventory-tab scope+facet state/render + the wishlist add-form facets — all extend-only, watcher untouched, no `v*` tag.
