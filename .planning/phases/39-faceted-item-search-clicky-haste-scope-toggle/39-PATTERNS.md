# Phase 39: Faceted item search (Clicky / Haste + scope toggle) - Pattern Map

**Mapped:** 2026-06-25
**Files analyzed:** 13 (8 source + 4 test + 0 migration)
**Analogs found:** 13 / 13 (every new/modified file is an EXTENSION of an existing file — exact in-file analogs)

> **Read first:** This is an EXTEND-IN-PLACE phase — every "new" file is actually a modification of an existing file (the analog IS the file being edited). There is no greenfield file and **no new migration** (Pattern 0). The planner should treat each row's "Analog" as "the lines to copy the surrounding idiom from when adding the delta."

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/store/itemsearch.go` | store (DB read) | request-response / CRUD-read | **itself** `SearchCatalog` + `CatalogIconCoverage` (`store/itemids.go:193-205`) | exact (same file + union precedent) |
| `internal/backendsrv/store/readviews.go` | store (DB read) | request-response | **itself** `IconStats` + `ItemMasterIconStats` (`:759-793`) | exact (widen in place) |
| `internal/backendsrv/compute/types.go` | model (DTO) | transform | **itself** `ItemRollup` (`:225-238`) | exact (append fields) |
| `internal/backendsrv/compute/itemrollup.go` | service (pure transform) | transform | **itself** `buildItemRollups` `ic := iconStats[vr.ID]` (`:72-82`) | exact (copy 2 booleans) |
| `internal/backendsrv/readapi/itemsearch.go` | controller (HTTP handler) | request-response | **itself** `ItemSearch.ServeHTTP` (`:46-75`) | exact (add 2 param parses) |
| `internal/backendsrv/readapi/items.go` | controller (HTTP handler) | request-response | **itself** (`:54-83`) — payload widens via `ItemRollup`; handler body UNCHANGED | exact (no body change) |
| `web/src/lib/api.ts` | utility (API client) | request-response | **itself** `searchCatalog` + `ItemRollup`/`CatalogItem` (`:237-251`, `:751-761`) | exact (append fields + 2nd arg) |
| `web/src/lib/items.ts` | utility (pure filter) | transform | **itself** `filterItems`/`viewerFirstItems` (`:21-36`) | exact (sibling helper) |
| `web/src/routes/inventory/+page.svelte` | component (route) | request-response / event-driven | **itself** state block (`:45-49`) + `asSlot` seam (`:104-125`); `.seg` block from `guild-views/+page.svelte:275-289,459-494` | exact + role-match for `.seg` |
| `web/src/routes/wishlist/+page.svelte` | component (route) | request-response / event-driven | **itself** add-form debounce (`:397-455`) | exact (add facet chips) |
| `internal/backendsrv/store/itemsearch_test.go` | test | — | **itself** `seedCatalogItem` (`:18-33`) + `TestSearchCatalog_*` | exact |
| `internal/backendsrv/readapi/itemsearch_test.go` | test | — | **itself** `seedCatalog`/`decodeItems` (`:26-47`) | exact |
| `internal/backendsrv/compute/itemrollup_test.go` | test | — | **itself** `setItemIconStats` (`:44-56`) + `TestItems_GroupsByNameWithHoldersAndFlags` (`:62`) | exact |
| `web/src/lib/items.test.ts` (or `__tests__/items.test.ts`) | test | — | the existing `items.ts` node-test (imports `filterItems`/`viewerFirstItems`) | exact |

---

## Pattern 0: NO new migration (confirm this first)

The two columns this phase reads ALREADY exist and are populated by the weekly wiki job — **do not add a `migrations/` file.**

- `item_master.is_clicky` / `has_haste` — **migration `00016_item_flags_effects.sql:23,25`** (Phase 37, held items, EQ-id-keyed):
  ```sql
  ALTER TABLE item_master ADD COLUMN is_clicky INTEGER;   -- :23 (nullable, no DEFAULT)
  ALTER TABLE item_master ADD COLUMN has_haste INTEGER;   -- :25
  ```
- `catalog_enrichment.is_clicky` / `has_haste` — **migration `00017_catalog_enrichment.sql:27,29`** (Phase 38, catalog-only, `norm_name` PK):
  ```sql
  CREATE TABLE catalog_enrichment (
    norm_name TEXT PRIMARY KEY,   -- :13 lower(trim(name)) — the cross-namespace key
    ...
    is_clicky INTEGER,            -- :27
    has_haste INTEGER,            -- :29
    ...
  );
  ```

Both columns are `INTEGER` and **nullable** → scan via `sql.NullInt64` and treat NULL/missing as `false` (a pre-00016 row, or a catalog row not yet covered by the Sunday crawl). `00017`'s own header comment (`:9-11`) names Phase 39 as the consumer: *"so Phase 39 can COALESCE held(item_master by id) UNION unheld(catalog_enrichment by name) one row per item."* **Watcher untouched → no `v*` tag** (a `v*` tag fires watcher CI; the watcher does not change this phase).

---

## Pattern Assignments

### `internal/backendsrv/store/readviews.go` (store, holdings-facet widen point)

**Analog:** itself — `IconStats` + `ItemMasterIconStats` (`:759-793`). This is the ONLY holdings-side data change; it keeps the holdings facet a pure client filter (SC-4: zero `catalog_enrichment` dependency).

**Struct to widen** (`:759-762`):
```go
type IconStats struct {
	IconID     int64
	Statsblock string
	// ★ Phase 39 — append two booleans (00016 columns, EQ-namespace correct here):
	IsClicky   bool
	HasHaste   bool
}
```

**The SELECT + nullable scan to widen** (`:770-792`) — copy the existing `sql.NullInt64`/`sql.NullString` idiom; the new columns are nullable so NULL → `false`:
```go
// existing:  SELECT item_id, icon_id, statsblock FROM item_master
//   widen →  SELECT item_id, icon_id, statsblock, is_clicky, has_haste FROM item_master
rows, err := s.db.QueryContext(ctx,
	`SELECT item_id, icon_id, statsblock, is_clicky, has_haste FROM item_master`)
...
var (
	id    int64
	icon  sql.NullInt64
	stats sql.NullString
	clk   sql.NullInt64 // ★ NULL/0 → false
	hst   sql.NullInt64 // ★
)
if err := rows.Scan(&id, &icon, &stats, &clk, &hst); err != nil { ... }
out[id] = IconStats{
	IconID:   icon.Int64,
	Statsblock: stats.String,
	IsClicky: clk.Int64 != 0, // ★ the NullInt64-→bool idiom
	HasHaste: hst.Int64 != 0, // ★
}
```
**Note:** the SELECT is `?`-free (no untrusted input) — keep it that way; only the projection list grows.

---

### `internal/backendsrv/compute/types.go` (model, append-only DTO fields)

**Analog:** itself — `ItemRollup` (`:225-238`). Append-only (the schema-evolution rule + the `api.ts` mirror contract — never rename an existing JSON tag).

**Append after `Statsblock` (`:236`), before `Holders`:**
```go
type ItemRollup struct {
	// ...existing fields through Statsblock (:236)...
	Statsblock  string        `json:"statsblock"`
	IsClicky    bool          `json:"is_clicky"` // ★ Phase 39 — from item_master (00016), client holdings facet
	HasHaste    bool          `json:"has_haste"` // ★ Phase 39
	Holders     []ItemHolder  `json:"holders"`
}
```
The snake_case tags MUST match the `web/src/lib/api.ts` `ItemRollup` interface field-for-field (the established Go↔TS mirror — see that file's doc comment at `:218-224` of the wishlist block and the items block).

---

### `internal/backendsrv/compute/itemrollup.go` (service, copy the two booleans onto the rollup)

**Analog:** itself — `buildItemRollups`'s existing `ic := iconStats[vr.ID]` propagation (`:72-82`). The map is ALREADY widened by the `readviews.go` change above; just copy two more fields into the rollup literal. **The IRON LAW (`:11`): this file authors ZERO SQL** — it only composes the widened `IconStats` map.

**Extend the rollup literal (`:73-82`):**
```go
ic := iconStats[vr.ID] // representative id-correct icon/stats (item_master EQ namespace)
roll = &ItemRollup{
	Name:        vr.Item,
	Price:       vr.Price,
	Prices:      vr.Prices,
	WikiURL:     vr.WikiURL,
	WikiSummary: vr.WikiSummary,
	IsQuestItem: vr.IsQuestItem,
	IconID:      ic.IconID,
	Statsblock:  ic.Statsblock,
	IsClicky:    ic.IsClicky, // ★ Phase 39
	HasHaste:    ic.HasHaste, // ★ Phase 39
}
```
No other change — `Items(...)` (`:38-52`) already calls `s.ItemMasterIconStats(ctx)`; the widened struct flows through for free.

---

### `internal/backendsrv/store/itemsearch.go` (store, the load-bearing catalog facet join — SERVER-side)

**Analog:** itself — `SearchCatalog` (`:61-98`); **the join shape is copied from `CatalogIconCoverage` (`store/itemids.go:193-205`)** — the shipped `item_master ∪ catalog_enrichment` UNION-ALL keyed by `lower(trim(name))`, swapping `icon_id` for `is_clicky`/`has_haste`. **NEVER join by `item_id`** (PigParse vs EQ namespace trap — Pitfall 1).

**Signature change (`:61`)** — add two bool params (this is a BREAKING signature change; see "Call-Site Ripple" below):
```go
func (s *Store) SearchCatalog(ctx context.Context, q string, clicky, haste bool, limit int) ([]CatalogItem, error) {
```

**The union JOIN (copy `CatalogIconCoverage`'s UNION-ALL `:199-202`, swap columns)** — add the `LEFT JOIN` ONLY when a facet is active so the no-facet path stays byte-identical to today's single-table query (Pitfall 3 — never `INNER JOIN`):
```go
like := "%" + escapeLike(q) + "%"   // PRESERVE escapeLike (:48-53) verbatim
prefix := escapeLike(q) + "%"

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
if clicky || haste { query += flagUnion } // join ONLY when a facet is on
query += " WHERE (pigparse_price.name LIKE ? ESCAPE '\\' OR CAST(pigparse_price.item_id AS TEXT) = ?)" +
	facet.String() +
	" ORDER BY (pigparse_price.name LIKE ? ESCAPE '\\') DESC, length(pigparse_price.name), pigparse_price.name COLLATE NOCASE LIMIT ?"

rows, err := s.db.QueryContext(ctx, query, like, q, prefix, limit)
// ... PRESERVE the existing scan loop (:75-97): sql.NullString name, sql.NullFloat64 avg,
//     fmt.Errorf("...(len=%d)...", len(q), err) — V7 len-only wrap, NEVER the q value
```

**PRESERVE verbatim (the V5/V7 discipline this file already enforces — `:14-25`):**
- `escapeLike` + `ESCAPE '\\'` + `?`-bound `q` (never concatenated) — the facet bools select a FIXED predicate fragment, no user string reaches SQL.
- `sql.NullString` name scan + `sql.NullFloat64` avg.
- The `fmt.Errorf("... (len=%d): %w", len(q), err)` wraps — len only, never `q` (V7).
- The `ORDER BY (name LIKE ...) DESC, length(name), name COLLATE NOCASE` prefix-first ranking (keep for catalog scope — Research Open Q1 recommendation).

> When a facet is active, qualify every bare `name` with `pigparse_price.name` (the union subquery also has a `name`-derived `norm` — qualify to avoid ambiguity; the column references above are already qualified).

---

### `internal/backendsrv/readapi/itemsearch.go` (controller, parse the facet params)

**Analog:** itself — `ItemSearch.ServeHTTP` (`:46-75`). Add exactly two bool-param parses + pass them to `SearchCatalog`; **preserve everything else verbatim** (Pitfall 4).

**The delta (`:51-69`):**
```go
q := strings.TrimSpace(r.URL.Query().Get("q"))
if utf8.RuneCountInString(q) < 2 {                 // PRESERVE the 2-rune guard BEFORE any DB hit
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]store.CatalogItem{})
	return
}
clicky := r.URL.Query().Get("clicky") == "1"        // ★ additive bool params (encoding "1" — Assumption A1; test it)
haste := r.URL.Query().Get("haste") == "1"          // ★
items, err := h.st.SearchCatalog(r.Context(), q, clicky, haste, searchLimit) // ★ pass facets
...
// V7 slog: add the two booleans (NOT PII), NEVER the q string:
slog.Info("item search ok", "rows", len(items), "qlen", utf8.RuneCountInString(q),
	"clicky", clicky, "haste", haste, "status", http.StatusOK) // ★
```
**PRESERVE:** the GET-only 405 (`:47-50`), the `searchLimit = 25` DoS cap (`:27`), the nil→`[]` coercion (`:65-67`), the no-`q`-in-logs error path (`:60-61`). The route registration in `cmd/squirebot-server/main.go:367` stays the same shape (`RequireSession`-gated — NEVER `RequireOfficer`, V4).

---

### `internal/backendsrv/readapi/items.go` (controller, NO handler-body change)

**Analog:** itself (`:54-83`). The two new booleans reach the wire purely via the widened `compute.ItemRollup` — **the handler body does not change.** `compute.Items(ctx, h.store, uid)` already returns the widened rollup; the JSON encoder serializes the appended `is_clicky`/`has_haste` automatically. This row exists only to record "no edit needed here" so the planner doesn't hunt for one. (Do NOT add a `?clicky=`/`?haste=` server param here — that would break SC-4; the holdings facet is client-side — Anti-pattern, Research §Anti-Patterns.)

---

### `web/src/lib/api.ts` (utility, append-only interface fields + the param wrapper)

**Analog:** itself — `ItemRollup` (`:237-251`), `CatalogItem` + `searchCatalog` (`:751-761`). Append-only (mirror the Go structs).

**`ItemRollup` — append after `statsblock` (`:249`):**
```ts
export interface ItemRollup {
	// ...existing through statsblock (:249)...
	statsblock: string;
	is_clicky: boolean; // ★ Phase 39 (mirrors compute.ItemRollup)
	has_haste: boolean; // ★ Phase 39
	holders: ItemHolder[];
}
```

**`CatalogItem` — append optional booleans (`:751-756`):**
```ts
export interface CatalogItem {
	item_id: number;
	name: string;
	current_avg?: number;
	is_clicky?: boolean; // ★ optional — server only sets when relevant; UI tolerates undefined
	has_haste?: boolean; // ★
}
```

**`searchCatalog` — add a 2nd facets arg (`:759-761`)** — the existing call passes only `q`, so the new arg MUST default to `{}` to keep that call compiling:
```ts
export function searchCatalog(
	q: string,
	facets: { clicky?: boolean; haste?: boolean } = {},
	f: typeof fetch = fetch
): Promise<CatalogItem[]> {
	const p = new URLSearchParams({ q });
	if (facets.clicky) p.set('clicky', '1'); // mirror the handler's "1" encoding (A1)
	if (facets.haste) p.set('haste', '1');
	return getJSON<CatalogItem[]>('/api/v1/items/search?' + p.toString(), f);
}
```
> **Signature note:** the third positional arg moves from `f` (today, position 2) to position 3. The ONLY caller (`wishlist/+page.svelte:431`) passes `searchCatalog(q)` with no 2nd/3rd arg, so it stays compiling — but if any caller passed a custom `fetch` positionally it would now land in `facets`. Confirmed: NO such caller exists (only `wishlist:431` + the tests).

---

### `web/src/lib/items.ts` (utility, the new pure node-tested facet filter)

**Analog:** itself — `filterItems`/`viewerFirstItems` (`:21-36`). Add a SIBLING pure helper (DOM-free, node-testable — the whole reason `items.ts` is a plain `.ts`, see its header `:1-9`). This keeps the facet logic OUT of `.svelte` where node vitest is DOM-blind (Pitfall 5).

**Add (sibling of `filterItems`):**
```ts
/** AND-combined Clicky/Haste facet (D-02): neither set → pass-through (full set);
 *  one set → that flag only; both set → the intersection. Pure; new array. */
export function facetItems(
	rows: ItemRollup[],
	f: { clicky: boolean; haste: boolean }
): ItemRollup[] {
	return rows.filter((r) => (!f.clicky || r.is_clicky) && (!f.haste || r.has_haste));
}
```
The Inventory tab composes it AFTER the name filter: `facetItems(filterItems(items, query), { clicky, haste })` (order-independent, but filter-then-facet matches the UI-SPEC "facets refine within the search").

---

### `web/src/routes/inventory/+page.svelte` (component, scope + facet state, the catalog read, the catalog-row render)

**Analogs:**
- The existing `$state` block (`:45-49`) — add `scope`/`clicky`/`haste`/`catalogRows` state.
- The `asSlot` ExaminePanel reuse seam (`:104-125`) — reuse UNCHANGED for catalog-row examine (D-04); build the same representative `InventorySlot` from a `CatalogItem` (catalog rows lack `holders`/`is_mine` — fill those from the holdings map).
- The `.seg`/`.seg-btn` segmented control — copy from **`web/src/routes/guild-views/+page.svelte`**: the markup (`:275-289`) + `setScope` (`:224-233`) + the DUPLICATED `<style>` block (`:459-494`). Svelte styles are component-scoped, so the `.seg`/`.seg-btn` CSS MUST be duplicated locally (the sanctioned precedent — `guild-views` itself duplicated it from `WantlistPanel`).
- The debounced catalog search — clone from **`wishlist/+page.svelte:397-440`** (the `DEBOUNCE_MS`/`addSeq` seq-guard idiom).

**State (extend the `:45-49` block):**
```ts
let scope = $state<'holdings' | 'catalog'>('holdings'); // D-03 default Holdings
let clicky = $state(false);
let haste = $state(false);
let catalogRows = $state<CatalogItem[]>([]);
```

**`.seg` scope markup (copy `guild-views/+page.svelte:275-289`, relabel):**
```svelte
<div class="seg" role="group" aria-label="Search scope">
  <button type="button" class="seg-btn" class:active={scope === 'holdings'}
          aria-pressed={scope === 'holdings'} onclick={() => setScope('holdings')}>Holdings</button>
  <button type="button" class="seg-btn" class:active={scope === 'catalog'}
          aria-pressed={scope === 'catalog'} onclick={() => setScope('catalog')}>Catalog</button>
</div>
```
```ts
function setScope(next: 'holdings' | 'catalog') {
	scope = next; // ★ D-03 "lens, not reset": NEVER touch query / clicky / haste here
}
```

**`.seg`/`.seg-btn` `<style>` block — DUPLICATE verbatim from `guild-views/+page.svelte:459-494`** (token-only — no literal hex; covers all 5 themes for free). Drop the `:disabled` rule (facets/scope are never disabled here — UI-SPEC §1 states).

**Facet chips (UI-SPEC §1 markup contract — `aria-pressed` filter-button idiom, NOT `Toggle.svelte`):**
```svelte
<button type="button" class="facet" class:active={clicky}
        aria-pressed={clicky} onclick={() => clicky = !clicky}>Clicky</button>
<button type="button" class="facet" class:active={haste}
        aria-pressed={haste} onclick={() => haste = !haste}>Haste</button>
```
The `.facet` style mirrors `.seg-btn` (UI-SPEC Typography: chips read as one control family) — inactive `--panel`/`--text` + 1px `--border`; active `--accent` fill + `--bg` label + a leading mark (never color-alone).

**Holdings render (no new fetch — pure client filter):**
```ts
let shown = $derived(facetItems(filterItems(items, query), { clicky, haste }));
```

**Catalog render (debounced server fetch — clone `wishlist/+page.svelte:417-440`):** on `query`/`clicky`/`haste` change while `scope === 'catalog'`, debounce-call `searchCatalog(query, { clicky, haste })` with the `addSeq`-style seq-guard, write `catalogRows`.

**Catalog-scope holders (D-04 — reuse the already-loaded rollup by name):**
```ts
// Build once from the already-fetched holdings (no new endpoint):
let holdingsByName = $derived(
	new Map(items.map((r) => [r.name.toLowerCase().trim(), r]))
);
// For a catalog row: const held = holdingsByName.get(row.name.toLowerCase().trim());
//   held → render held.holders[] (sortHolders); absent → "not held in the guild" (UI-SPEC Copywriting)
```

**PRESERVE:** plain `{}` interpolation for every name (Svelte auto-escape — NO new `{@html}`; the ONLY escaped sink stays the reused `ExaminePanel`/`composeItemNote`, `:18-21`). The stale-selection guard idiom (`:57-60`) — clear a selection that's absent in the new scope's result set on toggle.

---

### `web/src/routes/wishlist/+page.svelte` (component, the SECOND facet surface — catalog-only, NO scope toggle)

**Analog:** itself — the add-form catalog search (`:397-455`), specifically the `runAddSearch` call at `:431`. Add the SAME facet chips (D-01); narrow `searchCatalog`; **NO scope segmented control here** (the wishlist add is catalog-only).

**The delta (`:428-440`):** thread the facet state into the existing call:
```ts
let addClicky = $state(false); // ★ per-add-form facet state
let addHaste = $state(false);  // ★
async function runAddSearch(q: string) {
	const seq = ++addSeq;
	try {
		const items = await searchCatalog(q, { clicky: addClicky, haste: addHaste }); // ★ was searchCatalog(q)
		if (seq !== addSeq) return;
		addResults = items;
	} catch { ... }
}
```
Re-run `onAddInput`/`runAddSearch` when a chip toggles (the existing debounce path). Render the SAME `.facet` chips as the Inventory tab — **recommended: extract a small prop-driven `FacetBar.svelte`** (UI-SPEC Reuse Note) so the chip styling lives once; inlining `.facet` on both surfaces is also sanctioned (Svelte component-scoped styles). NO scope `.seg` block here.

---

## Test Pattern Assignments (the single-tested-SQL-path convention)

### `internal/backendsrv/store/itemsearch_test.go`

**Analog:** itself — `seedCatalogItem` (`:18-33`) + the `TestSearchCatalog_*` family. The existing `SearchCatalog(ctx, "rusty", 25)` calls (`:60,80,99,...`) MUST be updated to the new 5-arg signature `SearchCatalog(ctx, q, false, false, 25)` (the no-facet path == today's behavior — that's the regression proof). Add a `seedHeldFlag`/`seedCatalogEnrichmentFlag` helper (mirror `seedCatalogItem`) and new cases:
- clicky-only filters to `is_clicky=1` rows (held flag via `item_master`).
- haste-only filters to `has_haste=1` rows.
- both → the intersection.
- a CATALOG-only flag (seed `catalog_enrichment` by `norm_name`, no `item_master` row) is honored via the union.
- a HELD flag (seed `item_master`) is honored via the union.
- no-facet path returns the SAME set as before (the `INNER JOIN` regression guard — Pitfall 3).

### `internal/backendsrv/readapi/itemsearch_test.go`

**Analog:** itself — `seedCatalog`/`decodeItems` (`:26-47`) + `TestItemSearch_Matches` (`:49`). Add cases for `?clicky=1`/`?haste=1` reaching `SearchCatalog`; assert the 2-rune guard + nil→`[]` still hold with facets present. (The handler test seeds raw `pigparse_price` + the flag tables directly.)

### `internal/backendsrv/compute/itemrollup_test.go`

**Analog:** itself — `setItemIconStats` (`:44-56`) + `TestItems_GroupsByNameWithHoldersAndFlags` (`:62`), which ALREADY seeds `item_master` + calls `setItemIconStats(t, db, 1001, 560, "MAGIC ITEM\nDMG: 14")` (`:78`). Widen `setItemIconStats` to also stamp `is_clicky`/`has_haste` (or add a sibling `setItemFlags`), then assert the two booleans propagate onto the rollup — this is the boolean-propagation home (the test already has the seed scaffolding).

### `web/src/lib/items.test.ts` (or `__tests__/items.test.ts`)

**Analog:** the existing `items.ts` node test that imports `filterItems`/`viewerFirstItems`. Add `facetItems` cases: neither flag → pass-through; clicky-only; haste-only; both → intersection; AND-combination over a mixed set. Pure node vitest (DOM-free) — the facet LOGIC is covered here; the DOM render is browser-smoke (Pitfall 5).

---

## Shared Patterns

### V5 — Parameterized search (no user string in SQL)
**Source:** `store/itemsearch.go:44-53,64-69` (`escapeLike` + `ESCAPE '\\'` + `?`-bound `q`).
**Apply to:** the extended `SearchCatalog`. The facet bools select a FIXED predicate fragment (`AND COALESCE(f.is_clicky,0)=1`); NO user string is concatenated. Preserve `escapeLike` verbatim.

### V7 — No-PII search logging
**Source:** `readapi/itemsearch.go:60-61,68-69` + `readapi/items.go:82` (`rows`/`qlen`/`status` — never `q`).
**Apply to:** the extended handler slog line — ADD `clicky`/`haste` booleans (not PII), NEVER the `q` value. The error wrap stays `len(q)`-only (`itemsearch.go:71,83,95`).

### V12/V13 — DoS bound on the full-scan LIKE
**Source:** `readapi/itemsearch.go:27,52` (`searchLimit = 25` + the `utf8.RuneCountInString(q) < 2` guard BEFORE any DB hit).
**Apply to:** preserve both with the facet params present. Keep the 2-rune guard (Research Open Q2: do NOT add an empty-`q` "browse all clickies" path this phase — it's a separately-bounded read).

### V4 — Membership gate, never ownership scope
**Source:** `readapi/items.go:10-13` + `cmd/squirebot-server/main.go:367,386` (`RequireSession`, NEVER `RequireOfficer`).
**Apply to:** both reads stay `RequireSession`-gated (guild-wide; the catalog is public P99 data). No new endpoint, no new gate.

### The name-keyed cross-namespace join (the load-bearing rule)
**Source:** `store/itemids.go:193-205` (`CatalogIconCoverage` UNION-ALL by `lower(trim(name))`) + `compute/itemrollup.go:16-21,69` (group by normalized name, NEVER `item_id`) + memory `pigparse-vs-ingame-item-id-namespaces`.
**Apply to:** the catalog facet join in `SearchCatalog`. `pigparse_price.item_id` ≠ `item_master.item_id` — join the flag union by `lower(trim(name))` ONLY. A held name is NEVER in `catalog_enrichment` (write-path dedup, `itemids.go:177-191`) → plain `UNION ALL`, no precedence/COALESCE-between-tables needed.

### No new `{@html}` sink (web escape discipline)
**Source:** `inventory/+page.svelte:18-21` (plain `{}` interpolation; the ONE escaped sink is `ExaminePanel`/`composeItemNote`).
**Apply to:** every new catalog-row/facet-chip render — plain `{}`. The catalog-row examine REUSES `ExaminePanel` (the sanctioned escaped, scheme-allow-listed sink) — no new directive.

### Theme tokens only (5-theme parity for free)
**Source:** `guild-views/+page.svelte:459-494` (`.seg`/`.seg-btn` — `var(--panel)`/`var(--text)`/`var(--accent)`/`var(--bg)`/`var(--font-display)`, zero literal hex).
**Apply to:** the `.facet` chips + the duplicated `.seg`/`.seg-btn` + the catalog rows. No literal color, no new font, no new spacing scale (UI-SPEC Theme contract). `min-height: 44px` touch target + `:focus-visible` `outline: 2px solid var(--accent)` on every new control.

---

## Call-Site Ripple (breaking-signature checklist for the planner)

| Symbol | Old signature | New signature | Call sites to update |
|--------|---------------|---------------|----------------------|
| `store.SearchCatalog` (Go) | `(ctx, q, limit)` | `(ctx, q, clicky, haste, limit)` | `readapi/itemsearch.go:58` (prod) + `store/itemsearch_test.go:60,80,99,115,139,159,173,196` (8 test calls → pass `false, false`) |
| `searchCatalog` (TS) | `(q, f?)` | `(q, facets?, f?)` | `wishlist/+page.svelte:431` (passes only `q` → unchanged, compiles) + new `inventory/+page.svelte` caller. **No positional-`fetch` caller exists**, so the arg-shift is safe. |

`readapi/items.go` (`/api/v1/items`) and `cmd/squirebot-server/main.go` route registrations are UNCHANGED (the rollup widens via the struct; the handler body and routes don't move).

---

## No Analog Found

None. Every new/modified file is an in-place extension of an existing file with a verified analog (the file itself + a cited cross-file idiom for the new `.seg` control). There is no greenfield surface and no new migration in this phase.

---

## Metadata

**Analog search scope:** `internal/backendsrv/{store,compute,readapi,migrations}/`, `web/src/lib/`, `web/src/routes/{inventory,wishlist,guild-views}/`.
**Files scanned:** 17 (8 source analogs + 4 test analogs + 2 migrations + 3 web-route idiom sources).
**Pattern extraction date:** 2026-06-25
**Constraints confirmed:** NO new migration (00016/00017 columns read-only); single tested SQL path (catalog facet SQL lives ONLY in `store/itemsearch.go`); `RequireSession` (never `RequireOfficer`); V7 no-`q`-in-logs; no new `{@html}`; watcher untouched → no `v*` tag.

## PATTERN MAPPING COMPLETE

**Phase:** 39 - Faceted item search (Clicky / Haste + scope toggle)
**Files classified:** 13 (8 source + 4 test + 0 migration)
**Analogs found:** 13 / 13

### Coverage
- Files with exact analog: 13 (every file is an in-place extension; each cites the exact lines + a cross-file idiom source where one applies)
- Files with role-match analog: 0 (no greenfield file)
- Files with no analog: 0

### Key Patterns Identified
- **Catalog facet = name-keyed `item_master ∪ catalog_enrichment` UNION-ALL** (copied from the shipped `CatalogIconCoverage`, `store/itemids.go:193-205`, swapping `icon_id`→`is_clicky`/`has_haste`); join by `lower(trim(name))` ONLY, `LEFT JOIN` added ONLY when a facet is active (no-facet path stays byte-identical), never `item_id`.
- **Holdings facet = client-side over the widened `ItemRollup`** — two booleans flow `readviews.go IconStats` → `itemrollup.go` → `compute/types.go ItemRollup` → `api.ts` → pure `facetItems()` in `items.ts` (node-tested, SC-4-safe, zero catalog dependency).
- **Scope toggle = the `.seg`/`.seg-btn` segmented control duplicated from `guild-views/+page.svelte:275-289,459-494`**; `setScope` is a "lens, not reset" (never clears query/facets — D-03). Catalog-row examine reuses `ExaminePanel` unchanged (D-04); holders come from the already-loaded rollup by normalized name.
- **NO new migration** — 00016 + 00017 columns are read-only here; watcher untouched → no `v*` tag.
- **Preserved discipline:** `escapeLike`+ESCAPE (V5), `qlen`-not-`q` logging (V7), 2-rune guard + LIMIT 25 (DoS), `RequireSession` (V4), plain `{}` / no new `{@html}` (web escape), theme-tokens-only (5-theme parity).

### File Created
`.planning/phases/39-faceted-item-search-clicky-haste-scope-toggle/39-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. The planner can reference each analog's file:line excerpts directly in the PLAN action sections, and the Call-Site Ripple table flags the two breaking-signature edits (`SearchCatalog` Go + `searchCatalog` TS).
