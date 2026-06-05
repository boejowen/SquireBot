# Phase 19: Wantlist CRUD - Pattern Map

**Mapped:** 2026-06-03
**Files analyzed:** 13 (10 new, 3 modified/extended)
**Analogs found:** 13 / 13 (every file has a verified in-repo twin)

> This phase is a **composition phase** — every new file clones a named, read-this-session analog.
> The only genuinely net-new logic is the D-10 server-side catalog search, and even that clones the
> `readapi/views.go` GET-handler + `store/readviews.go` SELECT idioms. All line numbers below were
> verified against the actual files this session.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00006_wantlist.sql` | migration | DDL/CRUD | `migrations/00005_self_service_linking.sql` | role-match (forward-only goose) |
| `internal/backendsrv/store/wantlist.go` | store | CRUD (owner-scoped `*Tx`) | `store/linking.go` | exact |
| `internal/backendsrv/store/wantlist_test.go` | test | CRUD | `store/linking_test.go` / `readviews_test.go` seed idiom | role-match |
| `internal/backendsrv/store/itemsearch.go` | store | request-response (read) | `store/readviews.go` (`InventoryJoin`) | role-match |
| `internal/backendsrv/webadmin/wantlist.go` | controller | CRUD (request-response) | `webadmin/account.go` | exact |
| `internal/backendsrv/webadmin/wantlist_test.go` | test | request-response | `webadmin/account_test.go` | exact |
| `internal/backendsrv/readapi/itemsearch.go` | controller | request-response (read) | `readapi/views.go` (`ViewsHandler`) | exact |
| `internal/backendsrv/migrate_test.go` (EXTEND) | test | DDL | existing `TestMigrate_00005_*` cases | exact |
| `cmd/squirebot-server/main.go` (MODIFIED) | route | request-response | the `/account/codes` route block (lines 307-314) | exact |
| `web/src/routes/wantlist/+page.svelte` | route/page | request-response | `web/src/routes/account/+page.svelte` | exact |
| `web/src/lib/components/WantlistPanel.svelte` (+ `WantAddForm.svelte`) | component | request-response | `WatcherCodesPanel.svelte` | exact |
| `web/src/lib/api.ts` (EXTEND) | utility | request-response | the `/account/codes` wrapper block (lines 534-565) | exact |
| `web/src/lib/columns.ts` (EXTEND) | config | transform | `gearCheckColumns` + `tierSort` (lines 54-127) | exact |
| `web/src/lib/components/StateBlock.svelte` (EXTEND) | component | — | existing `no-codes` StateKind (lines 6-25, 93) | exact |
| `web/src/lib/components/SiteShell.svelte` (MODIFIED, nav only) | component | — | existing `session?.authenticated` nav block | role-match |

---

## Pattern Assignments

### `internal/backendsrv/store/wantlist.go` (store, owner-scoped CRUD)

**Analog:** `internal/backendsrv/store/linking.go`

**Owner-scoped IDOR-safe remove** — the load-bearing twin is `RevokeOwnCodeTx` (linking.go:198-210). Copy
its `WHERE id=? AND <owner>=?` + `RowsAffected → (bool, nil)` silent-no-op shape. For Phase 19 the remove
is a **soft-delete** (`SET active=0`, Pitfall 4), keyed on `discord_user_id` (NOT `owner_id` — Pitfall 3):
```go
// linking.go:198-210 — the exact shape to mirror for RemoveOwnWantTx
func RevokeOwnCodeTx(ctx context.Context, tx *sql.Tx, codeID, ownerID int64) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE guild_code SET disabled_at = datetime('now') WHERE id = ? AND owner_id = ? AND disabled_at IS NULL`,
		codeID, ownerID)
	if err != nil { return false, fmt.Errorf("revoke own code (code=%d owner=%d): %w", codeID, ownerID, err) }
	n, err := res.RowsAffected()
	if err != nil { return false, fmt.Errorf(...) }
	return n > 0, nil
}
```
Wantlist twin: `RemoveOwnWantTx(ctx, tx, wantID int64, discordID string)` → `UPDATE wantlist_item SET active = 0 WHERE id = ? AND discord_user_id = ? AND active = 1`.

**Owner-scoped list** — `ListOwnCodes` (linking.go:162-185) is the read twin: `QueryContext` with
`WHERE owner_id = ?`, `make([]T, 0)` non-nil slice (→ JSON `[]` not `null`), `rows.Next()/Scan` loop,
`rows.Err()` check. Wantlist twin: `ListOwnWants(ctx, db, discordID)` → `WHERE discord_user_id = ? AND active = 1 ORDER BY ...`.

**Add (insert) mutator** — model on the linking.go INSERT branch (linking.go:140-149): `tx.ExecContext` +
`res.LastInsertId()`, wrapped errors. Insert returns the new id so the handler can echo the created row.

**Error wrapping idiom:** every store func wraps with `fmt.Errorf("...(id=%d): %w", ...)` — keep it.

---

### `internal/backendsrv/store/itemsearch.go` (store, read — D-10 NEW)

**Analog:** `internal/backendsrv/store/readviews.go` (`InventoryJoin`, lines 127-150+)

This is a plain `(*Store)` read method (no tx). Copy `InventoryJoin`'s structure: build the query,
`s.db.QueryContext`, `defer rows.Close()`, scan loop, `rows.Err()`. The query searches `pigparse_price`
(NOT `item_master` — Pitfall A1) with **bound `?` placeholders only, never concatenated** (V5):
```go
// New SearchCatalog — readviews.go QueryContext idiom; pigparse_price is the D-10 corpus.
// COLLATE NOCASE is the repo case-insensitive idiom; ESCAPE '\' so a typed %/_ is literal.
func (s *Store) SearchCatalog(ctx context.Context, q string, limit int) ([]CatalogItem, error) {
	like   := "%" + escapeLike(q) + "%"
	prefix := escapeLike(q) + "%"
	rows, err := s.db.QueryContext(ctx,
		"SELECT item_id, name, current_avg FROM pigparse_price "+
		"WHERE name LIKE ? ESCAPE '\\' COLLATE NOCASE OR CAST(item_id AS TEXT) = ? "+
		"ORDER BY (name LIKE ? ESCAPE '\\' COLLATE NOCASE) DESC, length(name), name COLLATE NOCASE "+
		"LIMIT ?",
		like, q, prefix, limit)
	// ... rows.Next()/Scan -> rows.Err() (the readviews.go idiom) ...
}
```
**Proof of same id-space:** `InventoryJoin` already does `LEFT JOIN pigparse_price pp ON pp.item_id = ii.item_id`
(readviews.go:135) — so a want pinned from `pigparse_price.item_id` joins correctly against `ViewRow.id`
client-side. **Fixed-string note:** like `InventoryJoin`'s `bankOnly` branch, never interpolate user input
into the SQL string.

---

### `internal/backendsrv/webadmin/wantlist.go` (controller, CRUD)

**Analog:** `internal/backendsrv/webadmin/account.go` (the verbatim structural twin)

**Shared helpers (REUSE, do not re-declare):** `caller(ctx)`, `nowUnix()`, `writeJSON`, `writeJSONError`
live in `webadmin/officers.go` (lines 37-61). `withTx` + `AppendAuditTx` live in `webadmin/audit.go`
(lines 57-107). `wantlist.go` is in `package webadmin`, so it uses them directly — same as `account.go`.

**Remove handler** — clone `RevokeOwnCodeHandler` (account.go:137-187): method-check → decode `{id}` body
(reject `id <= 0` → `400 invalid_input`) → `callerID := caller(ctx)` → `withTx` { `store.RemoveOwnWantTx`
+ conditional `AppendAuditTx` only on a real removal } → `writeJSON({removed})`. **Owner from session, never
the body** (D-02). Use audit event `wantlist_remove`, detail `{want_id}` only (never note text — V7).

**Add handler** — model on `MintOwnCodeHandler` (account.go:42-73): method-check → `caller(ctx)` → `now` →
`withTx` { `store.AddWantTx` + `AppendAuditTx("wantlist_add", callerID, {item_id}, now)` }. **But do NOT call
the owner-resolve algorithm** (`ResolveOrCreateOwnerByDiscordTx`) — Pitfall 3: the wantlist keys directly on
`caller(ctx)` (the `discord_user_id`), there is no `owner` entity involved.

**List handler** — clone `ListOwnCodesHandler` (account.go:89-123): method-check (GET) → `caller(ctx)` →
`store.ListOwnWants` → `writeJSON(out)` where `out := make([]T, 0)` (non-nil → `[]`).

**Server-side validation** — clone the `validCharMeta` precedent (charmeta.go:93, 155): NEVER trust the form
`<select>`. Re-validate `reason ∈ {buy,quest}`, `priority ∈ {low,med,high}`, and **`utf8.RuneCountInString(note) > 280`**
(Pitfall 2 — runes, NOT `len()` bytes). Reject custom wants with a blank label.

**Error mapping** — clone `mapAccountErr` (account.go:211-219) if a typed dedupe-conflict error is introduced
(map the partial-unique-index violation → `409 duplicate`); otherwise default → `500 internal`.

---

### `internal/backendsrv/readapi/itemsearch.go` (controller, read — D-10 NEW)

**Analog:** `internal/backendsrv/readapi/views.go` (`ViewsHandler`, lines 44-139)

**Handler struct + constructor** — clone `ViewsHandler` / `NewViews` (views.go:44-54): hold a `*store.Store`,
construct one at startup. New: `NewItemSearch(st)`.

**ServeHTTP shape** — clone views.go:64-139: GET-only guard (405 otherwise), read+trim `q := r.URL.Query().Get("q")`,
**short-circuit to `[]` for `len(q) < 2`** (the empty-query guard + DoS mitigation, Pitfall A4), call
`store.SearchCatalog`, **nil→`[]` coercion** (views.go:87-89 — `if vr == nil { vr = []T{} }`), `json.NewEncoder(w).Encode`.

**V7 logging (load-bearing):** like views.go:124-138, log op + **result-count + `len(q)` ONLY — NEVER the
query string `q`** (it's user input). Mirror the `slog.Error("... read failed", "err", err)` / `slog.Info("... ok", "rows", n)` lines.

---

### `internal/backendsrv/migrations/00006_wantlist.sql` (migration, DDL)

**Analog:** `internal/backendsrv/migrations/00005_self_service_linking.sql`

**Forward-only convention** (00005:25-29): the `+goose Down` block is an explicit no-op `SELECT 1;` — copy verbatim.

**Partial-unique-index precedent** (00005:21-22): the D-05 dedupe uses the SAME tool 00005 used for
`owner_discord_user_id_uidx`. Two partial indexes (Pitfall 1), BOTH scoped `WHERE ... AND active = 1`:
```sql
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(discord_user_id, item_id, reason)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx  ON wantlist_item(discord_user_id, item_name, reason) WHERE item_id IS NULL     AND active = 1;
```
**Avoid the 00005 landmine** (00005:5-8): SQLite cannot `ALTER ... ADD UNIQUE column` — but 00006 uses fresh
`CREATE TABLE`s, so this is sidestepped (uniqueness via the partial indexes above). Full DDL skeleton in
RESEARCH.md lines 418-457 (the `wantlist_item` + `alert_log` stub + the optional `pigparse_name_idx`).

**Migration test** (EXTEND `migrate_test.go`): clone the `TestMigrate_00005_*` case using the established
`columnSet`/`tableExists`/`indexExists` + `NewTestDB` helpers — assert `wantlist_item` + `alert_log` exist,
the partial indexes exist, and `alert_log` is empty.

---

### `cmd/squirebot-server/main.go` (route registration, MODIFIED)

**Analog:** the `/account/codes` block (main.go:307-314)

Add the routes immediately after that block, copying the `RequireSession` (NOT `RequireOfficer` — login-only,
every member manages their OWN wantlist) wrapping exactly:
```go
mux.Handle("GET  /api/v1/wantlist",        webauth.RequireSession(db, webadmin.ListOwnWantsHandler(db)))
mux.Handle("POST /api/v1/wantlist",        webauth.RequireSession(db, webadmin.AddWantHandler(db)))
mux.Handle("POST /api/v1/wantlist/remove", webauth.RequireSession(db, webadmin.RemoveOwnWantHandler(db)))
// D-10 catalog search — session-gated like the view endpoints (readapi, takes the *store.Store `st`):
mux.Handle("GET  /api/v1/items/search",    webauth.RequireSession(db, readapi.NewItemSearch(st)))
```
Note `st` (the `*store.Store`) is already constructed at main.go:264 for the readapi view routes — reuse it.
Extend `main_test.go` with the anon→401 / member→admitted cases (twin of the existing route gate tests).

---

### `web/src/routes/wantlist/+page.svelte` + `WantlistPanel.svelte` (page + component)

**Analogs:** `web/src/routes/account/+page.svelte` (the `.form-card` page shell) +
`web/src/lib/components/WatcherCodesPanel.svelte` (the load→confirm→server-truth-reload lifecycle)

**Lifecycle (WatcherCodesPanel)** — verified line anchors:
- `onMount(() => { ... })` (WatcherCodesPanel:224) kicks the initial `load()`.
- `async function load()` (WatcherCodesPanel:105) → `await fetchOwnCodes()` (line 108), with `StateBlock kind="loading"` (line 233) / `kind="error" onRetry={load}` (line 235) phases.
- **Server-truth reload after a mutation** (WatcherCodesPanel:178-179): `doRevoke()` re-fetches
  (`codes = await fetchOwnCodes()`) rather than optimistically mutating the local array. Clone this for
  add AND remove (NEVER optimistic-mutate — the grid stays authoritative).
- `<StateBlock kind="no-codes" />` (line 295) → wantlist's `kind="no-wants"`.
- **Node-testable pure logic** (WatcherCodesPanel:24 `formatLastSeen`): keep search-result mapping,
  holder grouping, dedupe-check, and note-counter math in DOM-free exported functions (Pitfall 5 — node
  vitest is blind to the DOM; plan a manual browser-smoke).

**In-bank holder join (client-side, UNCHANGED by D-10)** — group the already-fetched `fetchView()`
`ViewRow[]` by `id`; reuse the `SearchResults` `↳ <Char>: <count>` treatment + `COLLAPSE_THRESHOLD=5`.
Code in RESEARCH.md lines 502-514 (`holdersFor()`).

**XSS boundary (load-bearing):** item names, custom labels, notes render via plain `{}` (Svelte
auto-escape) — NEVER `{@html}`. The only sanctioned `{@html}` sink stays `ItemTooltip`→`composeItemNote`.

---

### `web/src/lib/api.ts` (EXTEND)

**Analog:** the `/account/codes` wrapper block (api.ts:534-565)

Clone the `OwnCode` interface + `fetchOwnCodes`/`mintOwnCode`/`revokeOwnCode` wrappers (which use the shared
`getJSON`/`postJSON` cores). Add `WantlistRow` + `CatalogItem` interfaces and
`fetchOwnWants`/`addWant`/`removeWant`/`searchCatalog`. The add/remove bodies carry NO owner (session-derived,
D-02) — exactly like `mintOwnCode`'s `{}` body and `revokeOwnCode`'s `{id}`. Full typed signatures in
RESEARCH.md lines 475-499.

---

### `web/src/lib/columns.ts` (EXTEND)

**Analog:** `gearCheckColumns` + the `tierSort` custom `sortingFn` (columns.ts:54-127)

`tierSort` (columns.ts:55-57) maps a string to a rank for non-alpha sort — the EXACT twin for the priority
sort (`high=3/med=2/low=1`):
```ts
// columns.ts:55-57 — clone for prioritySort
function tierSort(a: Row<GearCheckRow>, b: Row<GearCheckRow>): number {
	return tierRank(a.original.tier) - tierRank(b.original.tier);
}
// gearCheckColumns wires it: { ..., sortingFn: tierSort }  (columns.ts:127)
```
Add `wantlistColumns: ColumnDef<WantlistRow, unknown>[]` per the 19-UI-SPEC column order
(Priority · Item · Reason · In bank? · Note · Remove), wiring `prioritySort` on the Priority column.
The DataGrid `defaultSorting` seed is `[{ id: 'priority', desc: true }, { id: 'in_bank', desc: false }]`.
**Global-filter caveat** (columns.ts:63-72): set `enableGlobalFilter: false` on accessor columns whose raw
value diverges from the rendered cell (e.g. the computed in-bank column, item_id).

---

### `web/src/lib/components/StateBlock.svelte` (EXTEND)

**Analog:** the existing `no-codes` StateKind (StateBlock.svelte:6-25 union, line 93 render branch)

Add `'no-wants'` to the `StateKind` union (line 25 region) and a matching `{:else if kind === 'no-wants'}`
render branch (clone the line-93 `no-codes` block) — heading "Your wantlist is empty" + the empty-state body
copy from 19-UI-SPEC. Reuse `loading` / `error` verbatim.

---

### `web/src/lib/components/SiteShell.svelte` (MODIFIED, nav only)

**Analog:** the existing `session?.authenticated` nav block (the `.char-meta-nav` entries)

Add a `Wantlist` link inside the authenticated-only block, styled exactly like the existing entries
(13px/600 display, uppercase, 0.08em tracking, 44px target). Nav-only change — no logic.

---

## Shared Patterns

### Session-derived owner (the IDOR boundary)
**Source:** `webadmin/officers.go:58` (`caller(ctx)` = `webauth.UserFromContext`)
**Apply to:** every wantlist handler (add/list/remove)
```go
func caller(ctx context.Context) string { uid, _ := webauth.UserFromContext(ctx); return uid }
```
The owner is `caller(ctx)` — the request body NEVER carries an owner/discord id (D-02). For the wantlist this
resolves directly to `discord_user_id` (Pitfall 3 — NOT an `owner` entity; do not call the owner-resolve algo).

### Atomic write + audit (panic-safe tx)
**Source:** `webadmin/audit.go:88-107` (`withTx`) + `audit.go:57-69` (`AppendAuditTx`)
**Apply to:** every mutating wantlist handler (add, remove)
```go
err := withTx(ctx, db, func(tx *sql.Tx) error {
	// store.*Tx mutator + AppendAuditTx in the SAME tx → atomic; deferred-rollback panic-safe
})
```
`AppendAuditTx` detail carries ids ONLY (`want_id`/`item_id`) — never note/label text (V7).

### Owner-scoped SQL no-op (TOCTOU-free IDOR)
**Source:** `store/linking.go:198-210` (`RevokeOwnCodeTx`)
**Apply to:** `RemoveOwnWantTx` (and the list query)
`UPDATE ... WHERE id=? AND discord_user_id=?` → `RowsAffected → (bool,nil)`; cross-owner = silent `false`,
never leaks the row's existence.

### JSON I/O contract
**Source:** `webadmin/officers.go:37-48` (`writeJSON` / `writeJSONError`)
**Apply to:** every wantlist handler — `{...}` success / `{"error":"code"}` errors; the frontend `api.ts`
routes off these. List/search ALWAYS emit a non-nil `[]` (the `make([]T, 0)` / nil→`[]` coercion).

### Server-side enum/length re-validation (V5)
**Source:** `webadmin/charmeta.go:155` (`validCharMeta`)
**Apply to:** the add-want handler — re-check `reason`/`priority` enums + `utf8.RuneCountInString(note) ≤ 280`
server-side; NEVER trust the form `<select>`/`<textarea>`.

### Parameterized SQL only (V5/Tampering)
**Source:** `store/readviews.go:127-150` (fixed-string branch, `?` placeholders)
**Apply to:** ALL wantlist + itemsearch queries — including the D-10 search `q` (bound `LIKE` term with
`ESCAPE '\'`, never concatenated).

### Plain-`{}` XSS escaping
**Source:** the 4 existing view pages (item names rendered via `{}`); the lone `{@html}` sink is `ItemTooltip`→`composeItemNote`
**Apply to:** every wantlist cell rendering item names, custom labels, notes, AND catalog-search result names.

---

## No Analog Found

None. Every Phase 19 file has a verified in-repo twin (the phase is a composition of P14/P16/P17 seams).

The ONE place with reduced reuse (flagged, not "no analog"): the client-side `didYouMean` typo-suggester in
`searchIndex.ts` cannot run over the full ~7k `pigparse_price` corpus (it isn't shipped client-side). Per
RESEARCH Addendum Q3/A5: drop it for the catalog field and rely on the server's substring+prefix ranking, OR
run `didYouMean` over the returned top-N result names only (optional polish, not required by WANT-01).

## Metadata

**Analog search scope:** `internal/backendsrv/{webadmin,store,readapi,migrations}/`, `cmd/squirebot-server/`,
`web/src/{routes,lib,lib/components}/`
**Files scanned (read or grepped this session):** account.go, audit.go, officers.go, charmeta.go,
linking.go, readviews.go, views.go, 00005_self_service_linking.sql, main.go, api.ts, columns.ts,
StateBlock.svelte, WatcherCodesPanel.svelte (+ RESEARCH.md / CONTEXT.md / UI-SPEC.md)
**Pattern extraction date:** 2026-06-03
