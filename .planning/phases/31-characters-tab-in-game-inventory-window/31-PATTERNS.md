# Phase 31: Characters Tab + In-Game Inventory Window - Pattern Map

**Mapped:** 2026-06-18
**Files analyzed:** 14 (8 backend new/modified, 1 migration, 5 web new/modified)
**Analogs found:** 14 / 14 (every file maps onto a shipped, verified analog — this is a pure pattern-replication phase)

This is a brownfield phase across THREE layers (Go backend read-API + enrichment, an
extend-only goose migration, SvelteKit web). The data model (`compute.StructuredInventory`,
the 23-slot taxonomy, container nesting, name-keyed price) and the 5-tab shell SHIPPED in
Phases 29-30. Every new file copies an existing idiom verbatim; the genuinely-new code is
two read handlers, one parser line + one column, and the Svelte rendering of an existing
typed payload.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/readapi/inventory.go` | controller (read handler) | request-response | `readapi/views.go` | exact |
| `internal/backendsrv/readapi/characters.go` | controller (read handler) | request-response | `readapi/meta.go` + `webadmin/assignment.go` (`ListMyAssignmentsHandler`) | exact (compose two analogs) |
| `internal/backendsrv/store/readviews.go` (EDIT) | model (store read) | CRUD (read) | existing `InventoryForChar` + `CharFreshness` in same file; new `RosterFor` | role-match |
| `internal/backendsrv/store/enrich.go` (EDIT) | model (store write) | CRUD (upsert) | `ItemMaster` struct + `UpsertItemMasterTx` (same file) | exact |
| `internal/backendsrv/enrich/wikiitem.go` (EDIT) | utility (pure parser) | transform | `ParseItempage` / `getParam` (same file) | exact |
| `internal/backendsrv/enrich/jobs/wiki.go` (EDIT) | service (weekly job) | batch/transform | `upsertItemAndQuests` (same file) | exact |
| `internal/backendsrv/compute/types.go` (EDIT) | model (struct/contract) | n/a | `InventorySlot` / `CharacterInventory` (same file) | exact (append-only) |
| `internal/backendsrv/compute/inventory.go` (EDIT) | utility (pure transform) | transform | `slotFromRow` (same file) | exact |
| `internal/backendsrv/migrations/00012_item_icon.sql` | migration | n/a | `00003_enrich_columns.sql` (`ALTER TABLE … ADD COLUMN`) | exact |
| `cmd/squirebot-server/main.go` (EDIT) | route (registration) | n/a | the `RequireSession`-wrapped read-route block (same file, lines 286-291, 351) | exact |
| `web/src/lib/api.ts` (EDIT) | utility (typed fetch) | request-response | `getJSON` + `fetchMeta`/`fetchMyCharacters` + row interfaces (same file) | exact |
| `web/src/routes/characters/+page.svelte` (REPLACE) | component (page) | request-response | `routes/guild-views/+page.svelte` (onMount load + StateBlock states) | role-match |
| `web/src/lib/components/InventoryWindow.svelte` (+ `PaperdollSlot.svelte`, `ExaminePanel.svelte`) | component | request-response | `ItemTooltip.svelte` + `composeNotes.ts` + `StateBlock.svelte` | role-match |
| `web/src/lib/myview.ts`-style pure helper (e.g. `roster.ts`) | utility (pure logic) | transform | `myview.ts` (viewer-first filter, node-testable) | exact |

---

## Pattern Assignments

### `internal/backendsrv/readapi/inventory.go` (controller, request-response) — NEW

**Analog:** `internal/backendsrv/readapi/views.go`

The new handler is the `views.go` shape with two deltas: (1) it reads the `{char}` path param via `r.PathValue("char")`, and (2) it dispatches to `compute.StructuredInventory(ctx, h.store, char)` instead of the view switch. Copy the struct + `NewX` constructor + `ServeHTTP` skeleton from `views.go` (lines 44-139). The handler holds `*store.Store`, guards defensively with a 405, logs op + count only (V7), and JSON-encodes the result.

**Handler skeleton to copy** (`views.go:44-68`):
```go
type ViewsHandler struct {
	store *store.Store
	view  string
}
func NewViews(s *store.Store, view string) *ViewsHandler {
	return &ViewsHandler{store: s, view: view}
}
func (h *ViewsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	// ... dispatch + encode ...
```

**The `{char}` param read + the empty-not-404 contract** (RESEARCH Pattern 2 / V4): the new handler reads `char := r.PathValue("char")` and passes it straight to `compute.StructuredInventory`. An unknown char returns an empty `CharacterInventory` (`InventoryForChar`'s `WHERE c.name = ?` yields zero rows → `buildStructuredInventory` returns empty slices), NOT a 404 — so D-11's "no inventory synced yet" renders client-side. The `char` value is ONLY a parameterized `?` bind (it flows into `InventoryForChar`'s `query, char` at `readviews.go:298`); never string-concat it into SQL or a content log.

**Encode + V7 logging tail** (`views.go:130-138`):
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
if err := json.NewEncoder(w).Encode(result); err != nil {
	slog.Error("inventory encode failed", "err", err)   // op + err only, NEVER char/item content
	return
}
slog.Info("inventory ok", "rows", count, "status", http.StatusOK)  // count only, never the char name
```

**nil→[] coercion:** `CharacterInventory.Equipment/General/Bank` are slices; coerce each nil to `[]InventorySlot{}` before encode (mirrors `views.go:87-89`) so the JSON is always arrays.

---

### `internal/backendsrv/readapi/characters.go` (controller, request-response) — NEW

**Analogs:** `internal/backendsrv/readapi/meta.go` (the local-typed snake_case response + GET-only skeleton) AND `internal/backendsrv/webadmin/assignment.go` `ListMyAssignmentsHandler` (the viewer-identity-from-context pattern).

This is the viewer-aware roster endpoint (D-10). Unlike `views.go`/`meta.go` it needs the viewer's `discord_user_id` to flag "yours" and sort viewer-first. RESEARCH Open Q2 recommends a single `*store.Store` method `RosterFor(ctx, viewerDiscordID)` returning band-tagged, viewer-first rows so the handler stays thin.

**Viewer identity inside a RequireSession-gated handler** (RESEARCH Pattern 3; the `webadmin/assignment.go:100-101` idiom — note `caller(ctx)` is webadmin's wrapper over `webauth.UserFromContext`):
```go
// Source: webauth/session.go:112 + webadmin/assignment.go:100
ctx := r.Context()
uid, ok := webauth.UserFromContext(ctx)   // the viewer's discord_user_id (the v2.3 "my characters" key)
// out, err := h.store.RosterFor(ctx, uid)
```

**Local typed snake_case response** (copy the shape from `meta.go:25-34`):
```go
// Source: readapi/meta.go
type metaChar struct {
	Name     string `json:"name"`
	LastSeen string `json:"last_seen"`
}
type MetaResponse struct {
	Characters []metaChar `json:"characters"`
}
```
The roster row analog adds `level/race/class/is_bank_toon/is_guild_bot/is_mine` to this; pre-size the slice so empty → `[]` (`meta.go:69`).

**Handler signature note (RESEARCH Open Q2):** `readapi` handlers hold `*store.Store`, but assignment reads (`ListMyAssignments`) are `*sql.DB`-shaped. The clean path is `RosterFor` as a `*Store` method that does the viewer-assignment join in-SQL; the handler then holds only `*store.Store` like `views.go`. If a `*sql.DB` is also needed, the registration may pass both (RESEARCH §"Registering the new routes": `readapi.NewCharacters(st, db)`).

---

### `internal/backendsrv/store/readviews.go` (model, CRUD read) — EDIT (two changes)

**Analog (existing reads in the same file):** `InventoryForChar` (lines 279-346) and `CharFreshness` (lines 614-641).

**Change 1 — `InventoryRow` gains `IconID`** so the icon flows to the window via the existing id-join. `InventoryForChar` already `LEFT JOIN item_master im ON im.item_id = ii.item_id` (`readviews.go:292`) — add `im.icon_id` to the SELECT list and scan it via a `sql.NullInt64` (the icon is nullable until enrichment runs). The SELECT to extend (`readviews.go:286-289`):
```go
SELECT c.name, ii.location, ii.name, ii.item_id, ii.count, ii.slots,
       im.wiki_url, im.wiki_summary, im.is_quest_item,    -- ADD im.icon_id here
       pp.direction, pp.a30, pp.t30, pp.last_seen,
       c.last_seen, ii.row_ordinal
```
Scan it into a new `iconID sql.NullInt64` and set `r.IconID = iconID.Int64` (mirroring the `slots sql.NullInt64` / `r.Slots = slots.Int64` handling at lines 318/330). **Pitfall 3:** this is an ID join in ONE namespace (`im.item_id = ii.item_id`, EQ-namespace) — correct. Do NOT key the icon by normalized name (that rule is ONLY for the cross-namespace PigParse *price* join via the `pp_rep` CTE).

**Change 2 — NEW `RosterFor` store read** (Pitfall 4: no existing read returns the full roster shape — `CharsWithMeta` lacks `is_guild_bot`+`last_seen`, `CharFreshness` lacks meta). Author it as a `*Store` method modeled on `CharsWithMeta` (lines 503-536), selecting the missing columns and LEFT-joining the viewer's assignment for the "yours" flag. `?`-placeholders only.

**`CharsWithMeta` to mirror** (`readviews.go:503-530`):
```go
func (s *Store) CharsWithMeta(ctx context.Context) ([]CharMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, class, level, race, is_bank_toon
		 FROM character
		 WHERE is_removed = 0
		 ORDER BY name`)
	// ... scan class/level/race as sql.Null*, is_bank_toon as int ...
}
```
The new `RosterFor(ctx, viewerDiscordID string)` extends this to (per RESEARCH Pitfall 4):
```sql
SELECT c.id, c.name, c.class, c.level, c.race,
       c.is_bank_toon, c.is_guild_bot, c.last_seen,
       (a.discord_user_id IS NOT NULL) AS is_mine
  FROM character c
  LEFT JOIN character_assignment a
    ON a.character_id = c.id AND a.discord_user_id = ?
 WHERE c.is_removed = 0
 ORDER BY c.name COLLATE NOCASE
```
(Bind `viewerDiscordID` as the `?`.) The three D-10 bands are derived in Go from the row flags: yours = `is_mine`; banks/bots = `is_bank_toon OR is_guild_bot`; guild = everyone else. The viewer-first A-Z ordering can be applied in Go (a pure sort, node-/Go-testable) or in SQL. Scan nullable meta via `sql.NullString`/`sql.NullInt64` exactly as `CharsWithMeta` does (lines 516-529).

**The `is_bank_toon`/`is_guild_bot` precedent** (these two columns already exist on `character`; read together at `store/assignment.go:97-108` `charSharedTx`):
```go
err := tx.QueryRowContext(ctx,
	`SELECT is_bank_toon, is_guild_bot FROM character WHERE id = ?`, characterID,
).Scan(&isBank, &isBot)
```

---

### `internal/backendsrv/store/enrich.go` (model, upsert) — EDIT

**Analog:** the `ItemMaster` struct (lines 58-67) + `itemMasterUpsert` SQL (lines 157-163) + `UpsertItemMasterTx` (lines 181-194), all in the same file.

Add `IconID int` to the `ItemMaster` struct, add `icon_id` to the upsert column list + the `ON CONFLICT DO UPDATE SET`, and pass `item.IconID` into the `ExecContext`. The icon rides the EXISTING write path — zero new tx, zero new join.

**Struct + upsert to extend** (`enrich.go:58-67`, `157-163`, `181-194`):
```go
type ItemMaster struct {
	ItemID        int
	Name          string
	WikiSummary   string
	WikiURL       string
	Slot          string
	IsQuestItem   bool
	WikitextSHA1  string
	LastRefreshed string
	// IconID int   ← ADD (append at the end; the wiki icon id, 0 = none yet)
}

const itemMasterUpsert = `INSERT INTO item_master
	(item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed)  -- ADD icon_id
 VALUES (?,?,?,?,?,?,?,?)                                                                          -- ADD one ?
 ON CONFLICT(item_id) DO UPDATE SET
   name=excluded.name, ... last_refreshed=excluded.last_refreshed`                                 -- ADD icon_id=excluded.icon_id

func UpsertItemMasterTx(ctx context.Context, tx *sql.Tx, item ItemMaster) error {
	quest := 0
	if item.IsQuestItem { quest = 1 }
	if _, err := tx.ExecContext(ctx, itemMasterUpsert,
		item.ItemID, item.Name, item.WikiSummary, item.WikiURL, item.Slot,
		quest, item.WikitextSHA1, item.LastRefreshed,   // ADD item.IconID
	); err != nil { ... }
}
```

---

### `internal/backendsrv/enrich/wikiitem.go` (utility, transform) — EDIT (the one-line gap)

**Analog:** `ParseItempage` (lines 65-94) + `getParam` (lines 428-433), same file. `parseTemplateParams(blockBody)` already returns the `{{Itempage}}` params map (line 75); RESEARCH verified the icon id is the `lucy_img_ID` param.

Add `IconID int` to `ParsedWikiItem` (lines 41-48), then capture it in the `item := ParsedWikiItem{...}` literal (lines 83-90):
```go
// Source: enrich/wikiitem.go — after `params := parseTemplateParams(blockBody)` (line 75)
item := ParsedWikiItem{
	ItemName:     itemname,
	Summary:      summary,
	WikiURL:      wikiURLFor(pageTitle),
	Slot:         kv["Slot"],
	IsQuestItem:  flags["QUEST ITEM"],
	WikitextSHA1: sha1Hex(wikitext),
	IconID:       parseIconID(getParam(params, "lucy_img_ID", "")),   // NEW
}
```
Add a small `parseIconID(string) int` helper: trim, atoi, return `0` when absent/blank/non-numeric (0 is the "no icon yet" sentinel — the client falls back to the colored tile, D-02). The existing `getParam` returns the fallback `""` when the key is absent (line 428-433), so a page with no `lucy_img_ID` yields `0`. Pure function, no API call — testable in `wikiitem_test.go` (present/absent/blank/non-numeric cases per RESEARCH Wave-0 gaps).

---

### `internal/backendsrv/enrich/jobs/wiki.go` (service, batch) — EDIT

**Analog:** `upsertItemAndQuests` (lines 217-264), same file — it already constructs the `store.ItemMaster` from the parsed `enrich.ParsedWikiItem`.

Pass `item.IconID` into the existing `store.UpsertItemMasterTx(ctx, tx, store.ItemMaster{...})` literal (lines 233-242):
```go
// Source: enrich/jobs/wiki.go:233
if err := store.UpsertItemMasterTx(ctx, tx, store.ItemMaster{
	ItemID:        int(ref.ItemID),
	Name:          item.ItemName,
	WikiSummary:   item.Summary,
	WikiURL:       item.WikiURL,
	Slot:          item.Slot,
	IsQuestItem:   item.IsQuestItem,
	WikitextSHA1:  item.WikitextSHA1,
	LastRefreshed: nowStr,
	// IconID: item.IconID,   ← ADD (one line)
}); err != nil { ... }
```
The SHA-1 short-circuit (`existing == item.WikitextSHA1` → skip, line 228) is unaffected — an icon-only change without a wikitext change won't re-write, which is fine (icon ids are stable for an item; the page they came from changing also changes the wikitext SHA-1, re-triggering the upsert).

---

### `internal/backendsrv/compute/types.go` (model, contract) — EDIT (append-only)

**Analog:** `InventorySlot` (lines 158-173) + `CharacterInventory` (lines 178-183), same file. The package doc warns these tags are the FIXED cross-plan JSON contract consumed by the web client — extend append-only, never rename.

Add `IconID int64 \`json:"icon_id"\`` to `InventorySlot` (the window reads it per slot). Per RESEARCH Open Q1 / Pitfall 2, add `LastSeen string \`json:"last_seen"\`` to `CharacterInventory` for the examine "Last synced" footer (a per-CHARACTER value — DO NOT confuse with the per-slot `LastListed`, which is the price last-listed date).

**The struct to extend** (`types.go:158-183`):
```go
type InventorySlot struct {
	Location      string          `json:"location"`
	Category      SlotCategory    `json:"category"`
	CanonicalSlot string          `json:"canonical_slot"`
	Item          string          `json:"item"`
	ID            int64           `json:"id"`
	Count         int64           `json:"count"`
	Slots         int64           `json:"slots"`
	Price         *float64        `json:"price"`
	LastListed    string          `json:"last_listed"`   // the PRICE last-listed date — NOT "last synced"
	WikiURL       string          `json:"wiki_url"`
	WikiSummary   string          `json:"wiki_summary"`
	IsQuestItem   bool            `json:"is_quest_item"`
	Prices        []PriceDetail   `json:"prices"`
	Children      []InventorySlot `json:"children"`
	// IconID int64 `json:"icon_id"`   ← ADD (append-only)
}

type CharacterInventory struct {
	Char      string          `json:"char"`
	Equipment []InventorySlot `json:"equipment"`
	General   []InventorySlot `json:"general"`
	Bank      []InventorySlot `json:"bank"`
	// LastSeen string `json:"last_seen"`   ← ADD (per-char examine "Last synced", Open Q1)
}
```

---

### `internal/backendsrv/compute/inventory.go` (utility, transform) — EDIT

**Analog:** `slotFromRow` (lines 210-227), same file — the pure row→slot mapper.

Copy `IconID` from the row onto the slot in `slotFromRow` (mirroring how it copies `WikiURL`/`Slots`):
```go
// Source: compute/inventory.go:210
return InventorySlot{
	Location:      row.Location,
	// ... existing fields ...
	WikiSummary:   row.WikiSummary,
	IsQuestItem:   row.IsQuestItem,
	Prices:        prices,
	// IconID: row.IconID,   ← ADD
}
```
For `CharacterInventory.LastSeen` (Open Q1 recommendation): in `buildStructuredInventory` (lines 114-206), source it from the first non-empty `row.LastSeen` (it's the same value on every row — `character.last_seen`). `StructuredInventory`'s shape is otherwise unchanged.

---

### `internal/backendsrv/migrations/00012_item_icon.sql` (migration) — NEW

**Analog:** `internal/backendsrv/migrations/00003_enrich_columns.sql` (the `ALTER TABLE … ADD COLUMN` pattern). Latest migration is `00011`; the next is `00012`. `item_master` is `(item_id INTEGER PRIMARY KEY, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed)` (`00001_init.sql:59`) — add the column at the right edge (extend-only).

**The verified pattern** (from `00003`'s header comment — SQLite permits only ONE column per ALTER; added columns are nullable, no DEFAULT, no UNIQUE/PK):
```sql
-- +goose Up
ALTER TABLE item_master ADD COLUMN icon_id INTEGER;   -- nullable, no DEFAULT, no UNIQUE (extend-only)

-- +goose Down
ALTER TABLE item_master DROP COLUMN icon_id;
```
The `_meta.schema_version` discipline from CLAUDE.md applies if a version bump is warranted; but note this repo's migrations are goose files applied on boot (no `WATCHER_MAX_SCHEMA_VERSION` gate is touched — the watcher is off the read path, untouched this phase).

---

### `cmd/squirebot-server/main.go` (route registration) — EDIT

**Analog:** the `RequireSession`-wrapped read-route block (lines 286-291) + the item-search route (line 351), same file. EVERY read route is `webauth.RequireSession(db, …)`-wrapped (login-gated since P15 — NOT public).

**The exact registration form to copy** (`main.go:286-291`, `351`):
```go
mux.Handle("GET /api/v1/meta", webauth.RequireSession(db, readapi.NewMeta(st)))
mux.Handle("GET /api/v1/views/view", webauth.RequireSession(db, readapi.NewViews(st, "view")))
// ...
mux.Handle("GET /api/v1/items/search", webauth.RequireSession(db, readapi.NewItemSearch(st)))
```
Add the two new routes (RESEARCH §"Registering the new routes"):
```go
mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
mux.Handle("GET /api/v1/characters",       webauth.RequireSession(db, readapi.NewCharacters(st)))   // or NewCharacters(st, db) if the roster handler needs *sql.DB
```
`{char}` is a Go 1.22+ ServeMux wildcard read via `r.PathValue("char")` (the repo is Go 1.25.7). Both MUST be `RequireSession`-wrapped, NEVER public, NEVER `RequireOfficer` (the roster + inventory are guild-wide reads — the gate is membership, not ownership; V4).

---

### `web/src/lib/api.ts` (utility, typed fetch) — EDIT

**Analog:** `getJSON` core (lines 154-196) + `fetchMeta`/`fetchMyCharacters` wrappers + the row interfaces (`MetaResponse`, `MyCharacter`), same file.

Add two credentialed wrappers + their TS row interfaces (mirroring the snake_case Go contract from `compute/types.go`). `getJSON` already carries `credentials:'include'`, typed `Unauthenticated`/`Forbidden`, and the malformed-JSON guard — reuse it, do NOT hand-roll a `fetch`.

**Wrapper idiom to copy** (`api.ts:220-222`, plus the `encodeURIComponent` path-param idiom at `api.ts:476`/`648`):
```ts
/** GET /api/v1/meta → { characters: [{ name, last_seen }] }. */
export function fetchMeta(fetchFn: typeof fetch = fetch): Promise<MetaResponse> {
	return getJSON<MetaResponse>('/api/v1/meta', fetchFn);
}
```
The new wrappers (the `char` path segment is `encodeURIComponent`'d — character names are guildie-controlled):
```ts
export function fetchInventory(char: string, f: typeof fetch = fetch): Promise<CharacterInventory> {
	return getJSON<CharacterInventory>(`/api/v1/inventory/${encodeURIComponent(char)}`, f);
}
export function fetchCharacters(f: typeof fetch = fetch): Promise<RosterCharacter[]> {
	return getJSON<RosterCharacter[]>('/api/v1/characters', f);
}
```
Define `CharacterInventory`/`InventorySlot`/`RosterCharacter` interfaces mirroring the Go snake_case tags (the `ViewRow` precedent at `api.ts:92-108` shows the exact mapping discipline; `InventorySlot` adds `icon_id`, `children`, `canonical_slot`, `last_listed`).

---

### `web/src/routes/characters/+page.svelte` (component, page) — REPLACE the placeholder

**Analog:** `web/src/routes/guild-views/+page.svelte` (the onMount data-load + loading/error/empty state machine, lines 113-181) + `StateBlock.svelte` (the verbatim copy strings).

Replace the Phase-30 "coming soon" placeholder with the real tab: a scoped search bar at the top (D-08), the bespoke roster list (3 bands), and the selected-character `InventoryWindow`. The data-load + state machine is the `guild-views` pattern.

**onMount one-shot load + AuthGate routing** (`guild-views/+page.svelte:113-176`):
```ts
const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);
let status = $state<'loading' | 'error' | 'ready'>('loading');

async function load() {
	status = 'loading';
	try {
		const roster = await fetchCharacters();   // + fetchMyCharacters() if needed for viewer-first
		// ... assign $state ...
		status = 'ready';
	} catch (err) {
		// 401/403 → hand to the AuthGate guard (server-truth re-route); else generic error
		if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
			authGuard(err);
		} else {
			status = 'error';
		}
	}
}
function refetch() { void load(); }
onMount(() => { void load(); });   // one-shot — NOT a bare $effect (re-fire risk, review WR-03)
```
Selecting a character fires a second `fetchInventory(name)` (own loading/error state inside the window column, UI-SPEC §J). URL-reflect the selection via `?c=<name>` (RESEARCH Open Q3 — query param, no per-character route file; reads `new URLSearchParams(window.location.search)` exactly as `guild-views` reads `?view=` at line 171).

**State blocks (verbatim copy, do NOT invent strings):** `StateBlock kind="loading"` / `"error"` (+ `onRetry={refetch}`) / `"empty"` (roster empty — the existing `empty` kind's copy is literally "No characters yet" / "No one's uploaded inventory yet…", `StateBlock.svelte:149-156`). The "Pick a character" and "No inventory synced yet" prompts (UI-SPEC §K) are NEW inline blocks in the StateBlock tone.

---

### `web/src/lib/components/InventoryWindow.svelte` + `PaperdollSlot.svelte` + `ExaminePanel.svelte` (components) — NEW

**Analogs:** `ItemTooltip.svelte` (the hover+tap+Esc+outside-dismiss popover mechanics, lines 49-111) and `composeNotes.ts` (the ONE safe `{@html}` sink, `composeItemNote`/`escapeHtml`/`safeHttpUrl`).

The window is a generic, prop-driven component over `CharacterInventory` (reused by Phase 33, feeds Phase 34 — no Characters-tab-only assumptions). The examine hover preview reuses `ItemTooltip`'s mechanics; the pinned panel renders the SAME composed body in a sticky card (UI-SPEC §G "share one `examineBody`").

**The hover/tap/dismiss mechanics to reuse** (`ItemTooltip.svelte:49-75`):
```svelte
let open = $state(false);
function show() { open = true; }
function hide() { open = false; }
function toggle() { open = !open; }
$effect(() => {
	if (!open) return;
	function onPointerDown(e: PointerEvent) {
		if (triggerEl && !triggerEl.contains(e.target as Node)) hide();
	}
	function onKey(e: KeyboardEvent) { if (e.key === 'Escape') hide(); }
	window.addEventListener('pointerdown', onPointerDown, true);
	window.addEventListener('keydown', onKey);
	return () => { /* remove both */ };
});
```

**The ONLY sanctioned `{@html}` sink** (`ItemTooltip.svelte:45-47`, `104-109`) — any examine HTML body MUST go through `composeItemNote`, which `escapeHtml`'s every interpolated value + `safeHttpUrl`-allow-lists the wiki href (`composeNotes.ts:53-72`, `99-164`). Item/character names are guildie-controlled (attacker-influenceable); render them via plain `{}` (auto-escaped) or the escaped composer — NEVER raw `{@html}` anything else (the live T-14.04-01 gate):
```svelte
let bodyHtml = $derived(
	composeItemNote(itemName, wikiUrl || wikiUrlFor(itemName), summary, prices, questLinks)
);
{#if open}
	<div class="tooltip-popover" role="dialog" aria-label="Item details">
		{@html bodyHtml}
	</div>
{/if}
```
Note D-08 reorders examine content (flags → slot → DMG/DLY → AC → stats → wt/size → class/race → price → wiki → last-synced) and adds the "Last synced" line, so the examine body likely needs a small extension of `composeItemNote` (or a sibling composer that reuses `escapeHtml`/`safeHttpUrl`) — keep the single escaped sink, never a second un-audited one.

**Colored-tile fallback (D-02)** — the locked icon-load-error pattern (RESEARCH §"Code Examples"):
```svelte
<img
  src={`https://wiki.project1999.com/images/Item_${slot.icon_id}.png`}
  alt=""
  style="image-rendering: pixelated; object-fit: contain;"
  onerror={(e) => (e.currentTarget.style.display = 'none')}  /* reveal the colored-tile under-layer */
/>
<!-- icon_id == 0 (or null): skip the <img>; render the deterministic hsl() gradient tile keyed on item name/id (D-02, UI-SPEC §Color). -->
```
No backend image proxy — the `<img>` loads directly from the wiki CDN (RESEARCH anti-pattern).

**Theme tokens only** — every tile/panel/badge reads `--accent`/`--panel`/`--border`/`--text`/`--font-display` (the `ItemTooltip.svelte:113-182` + `StateBlock.svelte:178-262` precedent); no literal hex, no new global vars (the sketch's `--slot`/`--slot-edge`/`--bg-2` map to real registry tokens — UI-SPEC §Color mapping table). Focus-visible: `outline: 2px solid var(--accent)` (`ItemTooltip.svelte:124-128`).

---

## Shared Patterns

### Session gating + viewer identity
**Source:** `internal/backendsrv/webauth/session.go` (`RequireSession` lines 194-204, `UserFromContext` lines 112-115)
**Apply to:** BOTH new read routes (register under `RequireSession`); the roster handler reads the viewer via `UserFromContext`.
```go
func RequireSession(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := resolveSessionUser(r, db)
		if !ok { writeJSONError(w, http.StatusUnauthorized, "unauthorized"); return }
		ctx := WithUser(r.Context(), uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func UserFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userCtxKey).(string)
	return v, ok
}
```

### Parameterized `?` SQL — the `{char}` injection guard (V5)
**Source:** `internal/backendsrv/store/readviews.go:280-298` (`InventoryForChar`) + `store/assignment.go` (every mutator binds `?`)
**Apply to:** the inventory endpoint (`char` is a value bind, never SQL text) and the new `RosterFor` read.
```go
rows, err := s.db.QueryContext(ctx, query, char)   // char is a ? bind in the WHERE c.name = ? — never string-concat
```

### V7 logging discipline
**Source:** `readapi/views.go:124-138`, `itemsearch.go:61-74`
**Apply to:** both new handlers. slog records carry op + count + status + err ONLY — never the `char`/item/character name or the row content.

### The one `{@html}` sink (XSS gate)
**Source:** `web/src/lib/tooltip/composeNotes.ts` (`composeItemNote`/`escapeHtml`/`safeHttpUrl`) + `web/src/lib/components/ItemTooltip.svelte` (the sole `{@html}` consumer)
**Apply to:** the examine hover preview + pinned panel. NEVER add a second un-audited `{@html}`; render names via plain `{}` (auto-escaped) elsewhere.

### Credentialed fetch + typed auth errors
**Source:** `web/src/lib/api.ts` `getJSON` (lines 154-196) + `Unauthenticated`/`Forbidden` (lines 46-59)
**Apply to:** `fetchInventory`/`fetchCharacters`; the page catch routes 401/403 to the AuthGate guard (the `guild-views/+page.svelte:146-150` precedent).

### Loading / empty / error copy
**Source:** `web/src/lib/components/StateBlock.svelte` (`kind="loading"/"empty"/"error"/"no-results"`)
**Apply to:** every roster + window fetch state (UI-SPEC §J reuses the kinds verbatim).

### Extend-only schema
**Source:** `internal/backendsrv/migrations/00003_enrich_columns.sql`
**Apply to:** `00012_item_icon.sql` (one nullable `ADD COLUMN`, no DEFAULT/UNIQUE).

### Viewer-first pure helper (node-testable)
**Source:** `web/src/lib/myview.ts` (the pure, DOM-free filter extracted so node vitest can cover it)
**Apply to:** the D-10 viewer-first roster sort/filter + the icon-url builder + the examine field-omission logic — extract into pure `web/src/lib/` modules so `npm test` covers the logic (the DOM stays a browser-smoke gap — Pitfall 1).

---

## No Analog Found

None. Every file in this phase maps cleanly onto a shipped, verified analog in the codebase. The two genuinely-new HTTP handlers copy `views.go`/`meta.go`; the new store read copies `CharsWithMeta`; the migration copies `00003`; the parser/struct/upsert edits are one-line extensions of their existing siblings; the web components reuse `ItemTooltip`/`composeNotes`/`StateBlock`/`getJSON`/`myview`.

---

## Metadata

**Analog search scope:** `internal/backendsrv/{readapi,store,compute,enrich,enrich/jobs,webauth,webadmin,migrations}`, `cmd/squirebot-server/`, `web/src/lib/{api.ts,tooltip,components}`, `web/src/routes/{characters,guild-views}`.
**Files scanned (read in full or targeted):** ~20 (views.go, meta.go, itemsearch.go, inventory.go-compute, types.go, slotconst.go, assignment.go, charmeta.go, readviews.go, enrich.go, wikiitem.go, jobs/wiki.go, session.go, webadmin/assignment.go, main.go route block, 00001/00003 migrations, api.ts, ItemTooltip.svelte, composeNotes.ts, StateBlock.svelte, guild-views/+page.svelte, characters/+page.svelte, myview.ts).
**Pattern extraction date:** 2026-06-18
