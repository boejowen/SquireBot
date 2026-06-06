# Phase 14: Web Frontend - Pattern Map

**Mapped:** 2026-05-30
**Files analyzed:** 38 new/modified files (12 backend Go, 26 frontend SvelteKit)
**Analogs found:** 12 / 38 with an in-repo analog (all 12 backend); 6 / 38 with a port-source (search/tooltip/theme + their tests); 20 / 38 are pure greenfield SvelteKit scaffolding with NO analog.

> **Read this first (the half/half framing):**
> - **Backend half (Go read API + 4 builder reimpls)** → STRONG in-repo analogs. Copy the handler/store/test patterns precisely (excerpts below).
> - **Frontend logic ports (`searchIndex`/`composeNotes`/`themes`)** → no Svelte analog, but a tested v1 TS *port-source* exists. Mapped source→destination + the behavior changes the port must make.
> - **Frontend scaffolding (SvelteKit/Tailwind/TanStack adapter/components)** → GREENFIELD. No `.svelte`/`svelte.config`/`tailwind.config` exists anywhere in the repo (verified). Do NOT force a match; use RESEARCH §"Architecture Patterns" Pattern 3 + the cited external idioms instead.

---

## File Classification

### Backend (Go) — has analogs

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/readapi/views.go` (NEW) | handler (route) | request-response (read) | `internal/backendsrv/ingest/whoami.go` | exact (read-only handler; DROP the bearer guard per D-04) |
| `internal/backendsrv/readapi/meta.go` (NEW) | handler (route) | request-response (read) | `internal/backendsrv/ingest/whoami.go` | exact |
| `internal/backendsrv/readapi/cors.go` (NEW) | middleware | request-response | **none in repo** (flagged) | NO ANALOG — stdlib `func(http.Handler) http.Handler`; see RESEARCH Pattern 2 |
| `internal/backendsrv/store/readviews.go` (NEW) | store (read query) | CRUD (read/JOIN) | `internal/backendsrv/store/itemids.go` (read-side `(*Store)` method) + `enrich.go` (struct shapes) | role-match (itemids.go is the only existing pure-read method) |
| `internal/backendsrv/compute/view.go` (NEW) | service (transform) | transform | `apps-script/src/tabs/buildView.ts` (behavioral oracle) | port-source (TS→Go reimpl) |
| `internal/backendsrv/compute/bank.go` (NEW) | service (transform) | transform | `apps-script/src/tabs/buildBank.ts` | port-source |
| `internal/backendsrv/compute/gearcheck.go` (NEW) | service (transform) | transform | `apps-script/src/tabs/buildGearCheck.ts` | port-source |
| `internal/backendsrv/compute/spellcheck.go` (NEW) | service (transform) | transform | `apps-script/src/tabs/buildSpellCheck.ts` | port-source |
| `internal/backendsrv/compute/{view,bank,gearcheck,spellcheck}_test.go` (NEW) | test | — | the matching `apps-script/src/__tests__/build*.test.ts` (parity oracle) + `store/testhelper.go` (temp-DB helper) | port-source + role-match |
| `cmd/squirebot-server/main.go` (MODIFY) | config (route wiring) | — | itself — extend the existing `mux.Handle(...)` block (lines 258-265) + wrap in CORS | exact (in-place extension) |
| `internal/backendsrv/compute/eqconst.go` (NEW, *only if needed*) | utility (constants) | — | `internal/backendsrv/enrich/eqconst.go` (ALREADY has `WIKI_SLOT_TO_INV_SLOTS`) | exact — **reuse, do not duplicate** (see Shared Patterns) |

### Frontend (SvelteKit) — port-source only, no Svelte analog

| New/Modified File | Role | Data Flow | Port Source | Match Quality |
|-------------------|------|-----------|-------------|---------------|
| `web/src/lib/search/searchIndex.ts` (NEW) | utility (pure logic) | transform | `apps-script/src/lib/searchIndex.ts` | port-source (strip Apps-Script I/O; fix 999.28 + 999.30) |
| `web/src/lib/tooltip/composeNotes.ts` (NEW) | utility (pure logic) | transform | `apps-script/src/tabs/composeNotes.ts` | port-source (plain-text → escaped HTML) |
| `web/src/lib/theme/themes.ts` (NEW) | utility (registry) | — | `apps-script/src/lib/themes.ts` (keys only) + UI-SPEC Theme Catalog (values) | port-source (registry → CSS custom props) |
| `web/src/lib/__tests__/searchIndex.test.ts` (NEW) | test | — | `apps-script/src/__tests__/searchIndex.test.ts` (UN-skip Test 4 = 999.30) | port-source |
| `web/src/lib/__tests__/composeNotes.test.ts` (NEW) | test | — | `apps-script/src/__tests__/composeNotes.test.ts` | port-source |
| `web/src/lib/__tests__/themes.test.ts` (NEW) | test | — | `apps-script/src/__tests__/themes.test.ts` | port-source |

### Frontend (SvelteKit) — GREENFIELD, no analog (see "No Analog Found")

`web/svelte.config.js`, `web/vite.config.ts`, `web/tsconfig.json`, `web/package.json`, `web/vitest.config.ts`, `web/src/app.html`, `web/src/app.css`, `web/src/routes/+layout.ts`, `web/src/routes/+layout.svelte`, `web/src/routes/+page.svelte`, `web/src/lib/api.ts`, `web/src/lib/table/createSvelteTable.ts`, `web/src/lib/table/FlexRender.svelte`, `web/src/lib/components/{SiteShell,ThemePicker,DataGrid,StatusCell,ItemTooltip,SearchBox,SearchResults,StateBlock}.svelte`, `web/static/robots.txt`.

---

## Pattern Assignments — BACKEND (copy these)

### `internal/backendsrv/readapi/views.go` + `meta.go` (handler, request-response read)

**Analog:** `internal/backendsrv/ingest/whoami.go` — the only existing *read-only* handler. Copy its skeleton EXACTLY, with one deletion: **D-04 says P14 is public, so DROP the `h.guard.ResolveToken(...)` block.** Everything else (method guard, `slog` on error only, JSON encode, V7 "never log row content") carries over verbatim.

**Handler struct + constructor pattern** (`whoami.go:40-51`):
```go
type WhoamiHandler struct {
	guard *auth.Auth
	db    *sql.DB
}
func NewWhoami(guard *auth.Auth, db *sql.DB) *WhoamiHandler {
	return &WhoamiHandler{guard: guard, db: db}
}
```
→ For P14: drop `guard`; hold a `*store.Store` instead of a raw `*sql.DB` (the read methods are `(*Store)` methods). e.g. `type ViewsHandler struct { store *store.Store; view string }`.

**Method-guard + read + JSON-encode body** (`whoami.go:57-93`) — the load-bearing skeleton to copy, minus the guard:
```go
func (h *WhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {                       // KEEP — defensive 405
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// >>> P14: DELETE the whole ResolveToken/401 block here (D-04 public read). <<<
	var label string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT label FROM owner WHERE id = ?`, ownerID).Scan(&label); err != nil {
		label = ""
		slog.Warn("whoami label lookup failed", "owner_id", ownerID, "err", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{ "owner_id": ownerID, "owner_label": label })
	slog.Info("whoami ok", "owner_id", ownerID, "status", http.StatusOK)
}
```

**Error-logging discipline to preserve verbatim (V7):** `slog.Error("... read failed", "view", h.view, "err", err)` then `http.Error(w, "internal error", http.StatusInternalServerError)` — **never echo row content or query params.** (Same posture as `handler.go:151` and `whoami.go:82`.)

---

### `cmd/squirebot-server/main.go` (route wiring, MODIFY in place)

**Analog:** itself. Extend the existing `runServe` mux block (`main.go:258-260`). Current verbatim:
```go
mux := http.NewServeMux()
mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))
mux.Handle("GET /api/v1/whoami", ingest.NewWhoami(auth.New(db), db))
srv := &http.Server{ Addr: *addr, Handler: mux }
```
**Extend to** (RESEARCH §"Code Examples" + Pattern 1):
```go
st := store.NewStore(db)                                  // store.NewStore exists (replace.go:45)
mux.Handle("GET /api/v1/meta",              readapi.NewMeta(st))
mux.Handle("GET /api/v1/views/view",        readapi.NewViews(st, "view"))
mux.Handle("GET /api/v1/views/gear_check",  readapi.NewViews(st, "gear_check"))
mux.Handle("GET /api/v1/views/spell_check", readapi.NewViews(st, "spell_check"))
mux.Handle("GET /api/v1/views/bank",        readapi.NewViews(st, "bank"))
srv := &http.Server{ Addr: *addr, Handler: cors(staticOrigin, mux) }  // CORS wraps everything
```
Note: Go 1.22+ method+pattern routing is already in use here — no router dep (D-10). `staticOrigin` should be a new `serve` flag or const (RESEARCH Open-Q2: pick the deploy origin before this task).

---

### `internal/backendsrv/readapi/cors.go` (middleware) — NO IN-REPO ANALOG

Grep across `internal/` for `Access-Control-Allow-Origin`/`OPTIONS`/`cors` returns **zero hits** — this is genuinely new. Build the ~10-line stdlib middleware from RESEARCH Pattern 2 (exact-origin, not `*`; `Vary: Origin`; 204 on `OPTIONS` preflight). Pitfall 5: set CORS once (in Go), and verify Caddy isn't also adding the header (duplicate → browser rejects).

---

### `internal/backendsrv/store/readviews.go` (store read query, CRUD-read)

**Analog:** `internal/backendsrv/store/itemids.go` — the repo's ONLY existing read-side `(*Store)` method (everything else in `store/` is write-side `*Tx`). Copy its exact shape: a plain `(*Store)` method (no tx-composing variant needed for reads), parameterless/parameterized `QueryContext` → `rows.Next()`/`Scan` loop → `rows.Err()` check, with a typed result struct.

**Read-method skeleton to copy** (`itemids.go:40-64`):
```go
func (s *Store) DistinctInventoryItemIDs(ctx context.Context) ([]ItemRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, MIN(name) AS name FROM inventory_item
		 WHERE item_id IS NOT NULL AND item_id > 0 GROUP BY item_id ORDER BY item_id`)
	if err != nil { return nil, fmt.Errorf("query distinct inventory item ids: %w", err) }
	defer rows.Close()
	var refs []ItemRef
	for rows.Next() {
		var ref ItemRef
		if err := rows.Scan(&ref.ItemID, &ref.Name); err != nil {
			return nil, fmt.Errorf("scan inventory item ref: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("iterate inventory item refs: %w", err) }
	return refs, nil
}
```

**Result-struct convention to mirror** (`enrich.go:38-103` defines store-local input structs; do the same for read outputs — store-local, NOT imported from `compute/`, so the dependency runs `compute → store`, never the reverse, matching `enrich.go:17-19`'s "store never imports its caller" rule).

**The `view`/`bank` JOIN (RESEARCH §"Per-View SQL Read Queries"):**
```sql
SELECT c.name AS char, ii.location AS slot, ii.name AS item, ii.item_id, ii.count,
       im.wiki_url, im.wiki_summary, im.is_quest_item,
       pp.direction, pp.a30, pp.t30,
       c.last_seen, ii.row_ordinal
FROM inventory_item ii
JOIN character c            ON c.id = ii.character_id
LEFT JOIN item_master im     ON im.item_id = ii.item_id
LEFT JOIN pigparse_price pp  ON pp.item_id = ii.item_id
WHERE c.is_removed = 0                      -- bank: AND c.is_bank_toon = 1
ORDER BY c.name, ii.name, ii.location;
```
- **Schema source of truth:** `migrations/00001_init.sql` (tables) + `00003_enrich_columns.sql` (the `a30/t30/...` price-history columns). Verified column names are in RESEARCH §"Per-View SQL Read Queries" table.
- `quest_items` is 0..N per item_id → fetch separately and group in Go (avoids row fan-out), mirroring the v1 `Map<number, QuestLinkRow[]>` approach (`buildView.ts:232-254`).
- **`pigparse_price.item_id` is the PRIMARY KEY** (`00001_init.sql:60`) → one price row per item; no both-directions fan-out (RESEARCH A3/Pitfall 6).
- **`/api/v1/meta`:** `SELECT name, last_seen FROM character WHERE is_removed=0`.

**gear_check / spell_check read methods:** these aren't a single JOIN — the store provides RAW inputs (a `SELECT` of `wiki_gear_tier` rows; a `SELECT` of `wiki_spells` rows; per-char `inventory_item`/`spellbook_entry`), and `compute/` produces the status rows (D-02). Keep the read methods dumb; the logic lives in `compute/`.

---

### `internal/backendsrv/compute/{view,bank,gearcheck,spellcheck}.go` (service transform, Go reimpl of v1)

**Analog (behavioral oracle, NOT code to copy line-for-line):** the matching `apps-script/src/tabs/build*.ts`. The Go output MUST match v1 semantics exactly (D-02 / WEB-02). The algorithms are transcribed in RESEARCH §"Porting the View Builders to Go"; the load-bearing details from the verified v1 source:

**`view` — `pickPrice` (the price-pick to reimplement)** (`buildView.ts:259-265`):
```ts
export function pickPrice(rows: PigparseRow[]): number | string {
  const wts = rows.find((r) => r.direction === 0);     // WTS first
  if (wts && wts.a30 > 0) return wts.a30;
  const wtb = rows.find((r) => r.direction === 1);      // then WTB
  if (wtb && wtb.a30 > 0) return wtb.a30;
  return '';
}
```
> **Type-drift warning (Pitfall 6 / RESEARCH):** v1 `direction` is NUMERIC (`0`=WTS, `1`=WTB). The SQLite `pigparse_price.direction` is **TEXT** (`enrich.go:46` `Direction string`; the P12 job stores `strconv.Itoa(t)`). The Go pick must compare the **stringified** direction. Spike `SELECT DISTINCT direction FROM pigparse_price` on the box before pinning the comparison (Open-Q1).
- **Sort:** Char asc → item asc → location asc (`buildView.ts:95-99`).
- **DROP the Sheet artifacts:** `=HYPERLINK(...)` formula strings (`buildView.ts:107-109`) — emit a plain `wiki_url`; the web renders a real `<a>`. Drop the `parseToDate`/conditional-format machinery (`buildView.ts:267-304`) — freshness coloring is client-side (UI-SPEC).
- **Enrichment inline (D-03):** the row struct must also carry `wiki_summary`, the WTS/WTB `a30`+`t30` detail, `is_quest_item`, and the quest-link list so the client tooltip needs no second fetch.

**`bank` — same join as `view`** but `WHERE c.is_bank_toon = 1`. **Bank-toon identity changed:** v1 read `_meta.bank_toon_name` (`buildBank.ts:54-55`); the DB equivalent is `character.is_bank_toon = 1` (`00001_init.sql:15`) — no `_meta` row exists in SQLite. **Coin is null/0 in P14** (ADMIN-05 fills it in P15) — return coin as null/absent; do NOT port the `composeCoinRow`/`writeCoinRow` machinery (`buildBank.ts:135-163`). Client renders "Coin: not yet recorded" (UI-SPEC copy).

**`gear_check` — the slot-pair match (the subtle parity case)** (`buildGearCheck.ts:101-125`):
```ts
const invSlots = WIKI_SLOT_TO_INV_SLOTS[slot] ?? [];
const charItemsInSlots = (inventoriesByChar.get(c.char_name) ?? [])
  .filter((it) => invSlots.includes(it.location));
for (const rec of recommendations) {
  const matched = charItemsInSlots.find((it) =>
    it.itemName.toLowerCase() === rec.item_name.toLowerCase());   // case-insensitive name match
  let status: 'OK' | 'MISSING' | 'OTHER'; let have = '';
  if (matched)                          { status = 'OK';      have = matched.itemName; }
  else if (charItemsInSlots.length > 0) { status = 'OTHER';   have = charItemsInSlots[0].itemName; }
  else                                  { status = 'MISSING'; have = ''; }
  dataRows.push([c.char_name, c.class, tier, slot, have, rec.item_name, status]);
}
```
- **Tiers shown:** `'Velious Pre-Raid/Group'` + `'Velious Raiding'` always; add `'Iksar'` iff `race === 'IKS'` (`buildGearCheck.ts:87-89`).
- **Tier sort rank** (`buildGearCheck.ts:29-33`): Pre-Raid=1, Raiding=2, Iksar=3. Sort: Char asc → tier rank → slot asc → recommended asc (`buildGearCheck.ts:131-141`).
- **`WIKI_SLOT_TO_INV_SLOTS` ALREADY EXISTS in Go** — `internal/backendsrv/enrich/eqconst.go:56-74` (ported verbatim from `eq-constants.ts`). **Reuse it**; do not redefine. The only missing constant is the tier-rank map (3 entries) — add that.

**`spell_check`** (`buildSpellCheck.ts:76-86`):
```ts
for (const c of chars) {
  if (!c.class || !Number.isFinite(c.level) || c.level < 1) continue;
  const wikiRows = wikiByClass.get(c.class) ?? [];
  const known = knownByChar.get(c.char_name) ?? new Set<string>();
  for (const w of wikiRows) {
    if (w.level > c.level) continue;                                  // level <= char.level
    const status = known.has(w.normalized_name) ? 'KNOWN' : 'MISSING';
    dataRows.push([c.char_name, c.class, w.level, w.spell_name, status]);
  }
}
```
- **Join key `normalized_name` is ALREADY MATERIALIZED in both DB tables** with the identical expression: `spellbook_entry.normalized_name` (`replace.go:169` `strings.ToLower(strings.TrimSpace(name))`) and `wiki_spells.normalized_name` (`enrich.go:248`, same expression). So the Go join is a direct equality on `normalized_name` — **no recompute** (cleaner than v1, which normalized at read time). Indexes exist: `spellbook_norm_idx` (`00001_init.sql:47`).
- **Sort:** Char asc → level asc → spell asc (`buildSpellCheck.ts:89-95`).

---

### `internal/backendsrv/compute/*_test.go` (parity tests)

**Analog:** `store/testhelper.go` (the temp-DB fixture) + the v1 `apps-script/src/__tests__/build*.test.ts` (the parity oracle).

**Test-DB helper to use** (`store/testhelper.go:23` — `NewTestDB(t)` opens a migrated temp SQLite DB and registers cleanup):
```go
func NewTestDB(t *testing.T) *sql.DB { /* Open(temp) + RunMigrations + t.Cleanup(close) */ }
```
This is `package store` and lives in a non-`_test.go` file specifically so other packages' test code can import it (`testhelper.go:18-22`) — `compute` tests can call `store.NewTestDB(t)`.

**Fixture-porting strategy (RESEARCH approach 1 — RECOMMENDED):** the v1 fixtures are vitest seed-arrays *coupled to the Apps Script mock* (`buildGearCheck.test.ts` seeds via `makeSheet`/`seedMeta` from `__tests__/test-helpers.ts`, asserting row tuples) — they **cannot** be imported into Go directly. Instead, translate each seed-array + expected-tuple set into a Go table-test that INSERTs the same rows into a `NewTestDB` and asserts the same `(status, have)` / `(level, spell, status)` tuples. The v1 expected-output tuples are the parity bar.
- The v1 test header column layouts you need to mirror when seeding (verified):
  - `_char_owner` cols (gear/spell tests): `char_name, owner_email, display_name, discord_handle, class, level, is_bank_toon, is_hidden, is_removed, first_seen, last_seen, server, watcher_version, race` → map to `character` columns (`class`, `level`, `race`, `is_bank_toon`, `is_removed`).
  - `wiki_gear_tier`: `tier, class, slot, item_id, item_name, rank, last_refreshed` (matches `00001_init.sql:62`).
  - `wiki_spells`: `class, level, spell_name, normalized_name, last_refreshed` (matches `00001_init.sql:61`).
  - `pigparse` (view/bank tests): the 15-col header `item_id, name, current_avg, last_seen, blue_volume, last_refreshed, direction, t30, a30, t60, a60, t6m, a6m, ty, ay` — **note direction is at index 6** and was numeric in the Sheet; in SQLite it is TEXT (re-confirm the stored value when seeding the parity test).

---

## Pattern Assignments — FRONTEND LOGIC PORTS (source → destination + the change)

### `web/src/lib/search/searchIndex.ts` ← `apps-script/src/lib/searchIndex.ts`

**Port verbatim:** `levenshtein` (`searchIndex.ts:71-87` — Wagner-Fischer, correct as-is), `groupAndSort` + `COLLAPSE_THRESHOLD=5` (`searchIndex.ts:277-301`, `:29`), and the `SearchResultRow`/`SearchResultGroup` types (`:40-66`).

**DROP the Apps Script I/O** (does not exist in the browser): `CacheService`, `PropertiesService`, `getActiveSpreadsheet`, `prewarmSearchCache` (`:175-192`), `getOrFillInvCache`/`readInvSheetCompact` (`:136-171`), `enrichResults` reading Sheet tabs (`:198-273`), `getCandidateChars` (`:103-134`), `getRecentSearches`/`pushRecentSearch` (`:364-380`). The web runs over the already-fetched `view` rows in memory (D-03).

**FIX 999.28** (empty-query contract). Current `didYouMean` (`searchIndex.ts:89-97`) has no empty guard; `didYouMean('')` returns every 1-2-char name. The intended fix (RESEARCH §"Two Carried Bugs", verbatim):
```ts
export function didYouMean(query: string, itemNames: string[]): string[] {
  if (!query || !query.trim()) return [];          // 999.28 FIX — first line
  const q = query.toLowerCase();
  return itemNames
    .map((n) => ({ n, d: levenshtein(q, n.toLowerCase()) }))
    .filter((x) => x.d <= 2 && x.d > 0)
    .sort((a, b) => a.d - b.d).slice(0, 3).map((x) => x.n);
}
```

**FIX 999.30** (the skipped test). RESEARCH recommends **path (a)**: keep whole-string Levenshtein, correct the wrong assertion. In the ported test (below) the formerly-`it.skip` Test 4 must become a real `it(...)` asserting `toEqual([])` (or single-word candidates where the math holds). **SC for 999.30: NO `.skip` remains on any `didYouMean` case.** (Path-(a) vs (b) is the one user-facing call — RESEARCH A5/Open-Q3 flags it; default to (a).)

**Trust-boundary note to carry forward verbatim** (`searchIndex.ts:15-18`): this lib returns RAW strings; escaping is the presentation layer's job. Keep that contract; the *consumer* (Svelte `{query}` interpolation, and `composeNotes`) escapes.

### `web/src/lib/tooltip/composeNotes.ts` ← `apps-script/src/tabs/composeNotes.ts`

**Port the content order + the `MAX_QUEST_LINKS_IN_NOTE=5` rule** (`composeNotes.ts:26`, `:33-62`): summary → blank → `Recent ask: <n>pp (30d avg, <t> transactions)` (WTS, direction 0) / `Buy posts: ...` (WTB, direction 1) / else `No recent transactions on PigParse.` → `Quest item: yes (in-game flag)` → `Used in quests: <names>` (max 5). `formatPp` (`:67-70`) ports as-is.

**THE CHANGE (WEB-04 / D-08): plain-text → escaped rich HTML.** v1 joins parts with `\n` (`composeNotes.ts:64`). The web port emits HTML (`<a>` for the wiki link, `<p>` for summary, etc.) AND **must add `escapeHtml()` applied to every interpolated string** (item/char/location/quest names, summary) before injection — only structural HTML is literal. This is the one HIGH-severity security item (RESEARCH §"Security Domain" / Pitfall: no `{@html}` on un-escaped content). The port test MUST assert a malicious name like `<img src=x onerror=...>` renders escaped.
> `direction` type: v1 uses numeric `PigparseDirection` (0/1). The web receives whatever the read API sends in JSON — coordinate the JSON field type with `compute/view.go` (the API can normalize to a stable shape, e.g. `"WTS"`/`"WTB"` or numeric; pick one and keep `composeNotes` and the Go encoder consistent).

### `web/src/lib/theme/themes.ts` ← `apps-script/src/lib/themes.ts` (keys) + UI-SPEC Theme Catalog (values)

**Port the 5 theme KEYS** (`themes.ts:13-23`) but **DROP `sheets-default`** (meaningless off-Sheets, D-06) and **change `DEFAULT_THEME` from `minimalist` to `velious`** (`themes.ts:69` → `velious`, D-06).

**The CHANGE:** the v1 `Theme` interface is Sheet-API-shaped (`headerBg`/`rowFg`/`fontFamily` + `applyTheme` calling `setBackground`/`setFontColor` — `themes.ts:25-125`). The web emits a CSS block per `[data-theme="<key>"]` with custom properties (`--bg`, `--panel`, `--text`, `--accent`, `--status-ok/missing/other`, `--font-display`, `--font-body`, `--weight-display`). **Use the UI-SPEC Theme Catalog values, NOT the v1 `THEMES` hexes** — RESEARCH §"State of the Art" flags the v1 palette as superseded by the richer mockup values (CLAUDE.md's "derive from `eq-aesthetic-theme.md`" rule is satisfied transitively because the UI-SPEC catalog derives from that doc). Per-theme overrides: `minimalist` uses `--weight-display: 400`; `heavy` inverts (light parchment rows on dark frame, MedievalSharp). Persistence: `localStorage.setItem('theme', key)` + set `[data-theme]` on the shell root; no stored pref → `velious`.

### `web/src/lib/__tests__/{searchIndex,composeNotes,themes}.test.ts`

Port the vitest files (the apps-script side uses vitest; the new `web/` package uses current vitest 4.x). `composeNotes.test.ts` (107 lines) and the `didYouMean`/`levenshtein` describe-blocks of `searchIndex.test.ts` port cleanly (they're pure-logic, no Apps Script mock). **`searchIndex.test.ts` Test 4 (lines 107-110) MUST be un-skipped + corrected (999.30).** The `runSearch` integration tests (`searchIndex.test.ts:157+`) and the `themes.test.ts` Sheet-coupled cases do NOT port (they depend on Apps Script mocks / `applyTheme`); rewrite `themes.test.ts` to assert the CSS-var registry shape instead.

---

## Shared Patterns

### Read-only handler skeleton (all 5 endpoints)
**Source:** `internal/backendsrv/ingest/whoami.go:57-93`
**Apply to:** `readapi/views.go`, `readapi/meta.go`
Copy the method-guard → read → `Content-Type: application/json` → `json.NewEncoder(w).Encode(...)` → `slog.Info` flow. **DELETE the `ResolveToken`/401 block** (D-04 public). Read-only contract: GET handlers author ZERO writes (V4) — same posture `whoami.go` documents in its header.

### Single-tested-SQL-path discipline (store ↔ caller dependency direction)
**Source:** `enrich.go:17-19` + `itemids.go:9-14` (the "store authors all SQL; callers compose; store never imports its caller" rule)
**Apply to:** `store/readviews.go` ↔ `compute/*`
Read SQL lives in `store/readviews.go` (parameterized `?` only — V5); `compute/` consumes typed structs. Result structs are store-local (mirror `enrich.go`'s store-local input structs), so the dep runs `compute → store`, never the reverse. No inline SQL in `compute/` or the handlers.

### Parameterized SQL only (V5 / Tampering)
**Source:** every `store/*.go` (`binding.go:57-58`, `enrich.go:109-118`, `itemids.go:42-46`) — `?` placeholders exclusively, never string-concat.
**Apply to:** `store/readviews.go`. P14 reads take no user input server-side (no filters), but keep the parameterized habit for any future `WHERE`.

### `WIKI_SLOT_TO_INV_SLOTS` (gear_check slot-pair map) — REUSE, don't duplicate
**Source:** `internal/backendsrv/enrich/eqconst.go:56-74` (already ported verbatim from `eq-constants.ts`)
**Apply to:** `compute/gearcheck.go`. The map already exists in Go in a sibling backend package. Import/reference it (or, if a cross-package import is awkward, the planner decides whether `compute` imports `enrich` or the const moves to a shared spot — but do NOT hand-retype the 17-entry map). The tier-rank map (3 entries, `buildGearCheck.ts:29-33`) is the only new constant.

### Structured logging, no row content (V7)
**Source:** `handler.go:29-31` + `whoami.go:20-22` headers; every `slog.*` call logs op + status + ids/counts + err, NEVER raw content or tokens.
**Apply to:** all `readapi/*` handlers + `store/readviews.go`. P14 has no token, but keep the habit; never echo item/char names or query params into logs.

### Trust boundary: escape user/wiki strings at presentation
**Source:** `searchIndex.ts:15-18` (v1's explicit "this lib returns raw; the sidebar escapes" note)
**Apply to:** `composeNotes.ts` (add `escapeHtml`), and Svelte components (rely on default `{}` escaping; never `{@html}` on un-escaped values). HIGH-severity gate (RESEARCH §"Security block gate").

---

## No Analog Found

These have **no in-repo analog** — the repo has zero `.svelte`/`svelte.config`/`tailwind.config`/`vite.config` files (verified by codebase scout 2026-05-30 and re-confirmed this session). The planner should use **RESEARCH §"Architecture Patterns"** (Pattern 3 local TanStack adapter), **§"Standard Stack"** (verified versions + the `@tanstack/table-core` NOT `@tanstack/svelte-table` correction — Pitfall 1), and **§"Code Examples"** (svelte.config / Tailwind v4 / `+layout.ts` SPA config) instead of an in-repo pattern.

| File | Role | Data Flow | Reason — no analog |
|------|------|-----------|--------------------|
| `web/svelte.config.js` | config | — | First SvelteKit app in repo. Use RESEARCH §"Code Examples" `adapter-static` + `fallback:'200.html'`. |
| `web/vite.config.ts` | config | — | Tailwind v4 Vite plugin (`@tailwindcss/vite`, NO PostCSS — Pitfall 3). RESEARCH §"Code Examples". |
| `web/tsconfig.json`, `web/package.json`, `web/vitest.config.ts` | config | — | New `web/` package; `npx sv create web` scaffolds these. |
| `web/src/app.html` | config (shell) | — | `<meta name="robots" content="noindex">` (D-05). Greenfield. |
| `web/src/app.css` | config (styles) | — | `@import "tailwindcss"` + `@fontsource/*` + the `[data-theme]` CSS-var blocks (from `theme/themes.ts`). Greenfield. |
| `web/src/routes/+layout.ts` | config (SPA) | — | `export const ssr=false; prerender=false;` (RESEARCH anti-pattern: don't prerender data routes). |
| `web/src/routes/+layout.svelte` | component (SiteShell) | — | Header/theme-picker/footer-attribution shell. Greenfield (UI-SPEC component inventory). |
| `web/src/routes/+page.svelte` | component (page) | request-response | The 4 tabbed views + SearchBox. Greenfield. |
| `web/src/lib/api.ts` | utility (fetch) | request-response | `fetch` wrappers for `/api/v1/views/*` + `/meta` (`PUBLIC_API_BASE` env). Greenfield. |
| `web/src/lib/table/createSvelteTable.ts` | utility (adapter) | — | Local Svelte-5 wrapper over `@tanstack/table-core` (RESEARCH Pattern 3 — the shadcn-svelte idiom; ~40 lines). No analog. |
| `web/src/lib/table/FlexRender.svelte` | component (adapter) | — | Header/cell renderer for the adapter. RESEARCH Pattern 3. |
| `web/src/lib/components/DataGrid.svelte` | component (grid) | — | The one reusable filterable/sortable grid (instantiated 4×). UI-SPEC Grid Contract + RESEARCH Pattern 3 (sticky col/header = pure CSS; faceted filter via `getFacetedRowModel`). |
| `web/src/lib/components/StatusCell.svelte` | component | — | OK/MISSING/OTHER/KNOWN text badge. UI-SPEC Color §status. |
| `web/src/lib/components/ItemTooltip.svelte` | component (popover) | event-driven | Hover/tap popover wrapping `composeNotes.ts` output. UI-SPEC Tooltip Contract. |
| `web/src/lib/components/SearchBox.svelte` + `SearchResults.svelte` | component | event-driven | Wrap `searchIndex.ts`; inline "did you mean?". UI-SPEC Search Contract. |
| `web/src/lib/components/ThemePicker.svelte` | component | event-driven | `<select>`/swatch → `localStorage` + `[data-theme]`. UI-SPEC. |
| `web/src/lib/components/SiteShell.svelte` | component | — | (if split from `+layout.svelte`) shell chrome. Greenfield. |
| `web/src/lib/components/StateBlock.svelte` | component | — | Shared empty/error/loading presentation. UI-SPEC Copywriting Contract. |
| `web/static/robots.txt` | config (static asset) | — | `Disallow: /` (D-05). Greenfield. |

---

## Metadata

**Analog search scope:** `internal/backendsrv/{ingest,store,enrich,migrations}/`, `cmd/squirebot-server/`, `apps-script/src/{tabs,lib,__tests__,__fixtures__}/`, repo-wide for `.svelte`/`svelte.config`/CORS.
**Files scanned (read in full):** `whoami.go`, `version.go`, `handler.go`, `main.go`, `db.go`, `enrich.go`, `replace.go`, `binding.go`, `itemids.go`, `testhelper.go`, `enrich/eqconst.go`, `00001_init.sql`, `00003_enrich_columns.sql`, `buildView.ts`, `buildGearCheck.ts`, `buildSpellCheck.ts`, `buildBank.ts`, `composeNotes.ts`, `searchIndex.ts`, `themes.ts`, `eq-constants.ts`, `searchIndex.test.ts` (Test-4 region); test-file headers for the 6 ports.
**Key verifications this session:** (1) CORS = zero in-repo hits → genuinely new; (2) `WIKI_SLOT_TO_INV_SLOTS` already in Go at `enrich/eqconst.go` → reuse; (3) `normalized_name` materialized identically in `spellbook_entry` + `wiki_spells` → spell_check join needs no recompute; (4) `pigparse_price.direction` is TEXT in SQLite vs numeric in v1 → port-time type-drift; (5) no `web/` dir / no `.svelte` files → frontend scaffolding is pure greenfield.
**Pattern extraction date:** 2026-05-30
