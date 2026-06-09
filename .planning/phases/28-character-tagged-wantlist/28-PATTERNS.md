# Phase 28: Character-Tagged Wantlist - Pattern Map

**Mapped:** 2026-06-08
**Files analyzed:** 14 (11 backend/web edits + 3 net-new)
**Analogs found:** 14 / 14 (every surface has an in-repo analog — confirms RESEARCH's "threading one nullable column" thesis)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00010_character_tagged_wantlist.sql` | migration | schema-DDL | `migrations/00006_wantlist.sql` (partial-unique idiom) + `00009_character_assignment.sql` (extend-only ADD COLUMN + header) | exact |
| `internal/backendsrv/migrations/migrate_test.go` (edit) | test | schema-assert | `migrate_test.go:569` `TestMigrate_00009_CharacterAssignment` | exact |
| `internal/backendsrv/store/wantlist.go` (edit) | store | CRUD | self — `AddWantTx:70` / `ListOwnWants:97` (thread `character_id`) | exact (self-extend) |
| `internal/backendsrv/store/wantlist.go` NEW `ListGuildWants` | store | request-response (read) | `assignment.go:550` `ListAllAssignments` (all-members JOIN char) + `wantlist.go:97` `ListOwnWants` (scan shape) | role-match (net-new) |
| `internal/backendsrv/store/assignment.go` NEW `IsCharAssignedToTx` | store | request-response (in-tx probe) | `assignment.go:92` `charSharedTx` (in-tx existence probe) | exact |
| `internal/backendsrv/wantmatch/match.go` (edit) | service | event-driven (fan-out) | self — `Hit:38` / `ForItem:55` / `scanHits:86` (add `CharacterName` LEFT JOIN) | exact (self-extend) |
| `internal/backendsrv/ec/embed.go` (edit) | utility | transform (presentation) | self — `buildEmbed:118` (append a "For" field) | exact (self-extend) |
| `internal/backendsrv/webadmin/wantlist.go` (edit) | controller | request-response | self — `AddWantHandler:115` (thread `character_id` + IDOR guard) | exact (self-extend) |
| `internal/backendsrv/webadmin/wantlist.go` NEW `ListGuildWantsHandler` | controller | request-response (read) | `wantlist.go:228` `ListOwnWantsHandler` | exact |
| `cmd/squirebot-server/main.go` (edit) | route | request-response | `main.go:342` wantlist route block | exact |
| `web/src/lib/api.ts` (edit) | utility | request-response (client) | self — `WantlistRow:580` / `addWant:616` / `fetchMyCharacters:906` | exact (self-extend) |
| `web/src/lib/wantlist/groupByChar.ts` NEW | utility | transform (pure helper) | `web/src/lib/myview.ts` (DOM-free filter helper) | exact |
| `web/src/lib/components/WantAddForm.svelte` (edit) | component | request-response | self — Reason/Priority `<select>` block (`:207-222`) + `fetchMyCharacters` | exact (self-extend) |
| `web/src/lib/components/WantlistPanel.svelte` (edit) | component | request-response | self + `myview.ts` `<select>` precedent (`+page.svelte` toggle) | exact (self-extend) |

---

## Pattern Assignments

### `internal/backendsrv/migrations/00010_character_tagged_wantlist.sql` (migration, schema-DDL)

**Analog A (header + extend-only ADD COLUMN):** `migrations/00009_character_assignment.sql`

The 00009 header is the copy-faithful template for the 00010 header (backend-only / no `WatcherMaxSchemaVersion` / "Schema vN == goose N applied" / forward-only Down). Lines 1-11:
```sql
-- +goose Up
-- Phase 26 plan 26-01 (ASSIGN-01..06). ...
--
-- Backend-only: the watcher never reads/writes these tables, so there is NO
-- _meta.schema_version bump and NO WatcherMaxSchemaVersion change (the watcher
-- targets the ingest API, not these backend tables). "Schema v9" == goose
-- migration 00009 applied (goose_db_version is the version record).
```
Extend-only ADD COLUMN idiom (line 16) — copy for the nullable `character_id`:
```sql
ALTER TABLE character ADD COLUMN is_guild_bot INTEGER NOT NULL DEFAULT 0;
```
For CWANT-28, instead: `ALTER TABLE wantlist_item ADD COLUMN character_id INTEGER REFERENCES character(id);` — NULLABLE, **no `NOT NULL`, no `DEFAULT`** (so existing rows auto-backfill NULL — CWANT-02), and **no `ON DELETE CASCADE`** (a deleted char falls back to account-level via LEFT-JOIN-NULL — RESEARCH anti-pattern).

Forward-only Down (lines 68-70) — copy verbatim:
```sql
-- +goose Down
-- Forward-only in practice (mirrors 00004-00008): explicit no-op.
SELECT 1;
```

**Analog B (the partial-unique dedup indexes to DROP+recreate):** `migrations/00006_wantlist.sql` lines 33-34:
```sql
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(discord_user_id, item_id, reason)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx  ON wantlist_item(discord_user_id, item_name, reason) WHERE item_id IS NULL     AND active = 1;
```
The 00006 header (lines 10-15) documents the NULL-DISTINCT trap that re-bites `character_id` — read it before writing the COALESCE rewrite:
```sql
-- D-05 dedupe (19-RESEARCH Pitfall 1): SQLite treats NULL as DISTINCT in a UNIQUE
-- index, so a single UNIQUE(discord_user_id, item_id, reason) would NOT dedupe
-- custom wants (item_id NULL). Two PARTIAL unique indexes split the cases ...
```
**00010 rewrite (per RESEARCH Pattern 1):** `DROP INDEX` both, then recreate with `COALESCE(character_id, -1)` appended to each key so NULL (account-level) collapses to one sentinel while distinct char ids stay distinct. The partial-`WHERE` clauses are unchanged.

---

### `internal/backendsrv/migrations/migrate_test.go` — `TestMigrate_00010_*` (test, schema-assert)

**Analog:** `migrate_test.go:569` `TestMigrate_00009_CharacterAssignment`

Column-exists assertion (lines 572-576) — copy for `character_id` on `wantlist_item`:
```go
charCols := columnSet(t, db, "character")
if !charCols["is_guild_bot"] {
    t.Errorf("expected character to have column %q after 00009 ...", "is_guild_bot", charCols)
}
```
Partial-unique collision assertion (lines 607-619) is the dedup-test template: insert one row, assert a duplicate collides, then assert a distinguishing change (here: a different `character_id`, or NULL-vs-NULL still colliding) behaves. The 00010 test must assert BOTH: (a) two account-level (`character_id` NULL) wants for the same (user,item,reason) STILL collide (COALESCE sentinel preserves 00006 dedup — Pitfall 2 warning sign), and (b) the same (user,item,reason) for two DIFFERENT `character_id`s does NOT collide. Helpers available in-file: `columnSet`, `tableExists`, `indexExists`, `mustInsertOwner`, `mustInsertChar`, `store.NewTestDB(t)` (line 570).

---

### `internal/backendsrv/store/wantlist.go` — `AddWantTx` + `ListOwnWants` (store, CRUD)

**Analog:** self.

`AddWantTx:70` gains a `characterID *int64` param and one column in the INSERT. Current signature + INSERT (lines 70-74):
```go
func AddWantTx(ctx context.Context, tx *sql.Tx, discordID string, itemID *int64, itemName, reason, priority string, note *string, now int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		discordID, itemID, itemName, reason, priority, note, now)
```
Keep the `ErrDuplicateWant` extended-result-code detection (lines 79-82) UNCHANGED — the COALESCE index still raises `sqliteConstraintUnique` (2067):
```go
var sqliteErr *sqlite.Error
if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintUnique {
	return 0, ErrDuplicateWant
}
```

`WantlistRow:52` gains `CharacterID *int64` + `CharacterName *string` (mirror the existing `*int64 ItemID` / `*string Note` pointer-for-nullable idiom, lines 52-61). `ListOwnWants:97` gains the LEFT JOIN + two nullable scans. Current query (lines 98-102) and the `sql.Null*` → pointer scan idiom (lines 110-126) are the exact pattern to extend:
```go
rows, err := db.QueryContext(ctx,
	`SELECT id, item_id, item_name, reason, priority, note, created_at, muted
	   FROM wantlist_item
	  WHERE discord_user_id = ? AND active = 1
	  ORDER BY created_at DESC`, discordID)
// ...
var itemID sql.NullInt64; var note sql.NullString
if itemID.Valid { v := itemID.Int64; r.ItemID = &v }
```
The LEFT JOIN to `character c ON c.id = w.character_id` + `c.name AS character_name` follows `assignment.go:448` (`SELECT c.name ... JOIN character c ON c.id = a.character_id`); scan `character_id`/`character_name` via `sql.NullInt64`/`sql.NullString` → pointers exactly as `item_id`/`note` already are.

---

### `internal/backendsrv/store/wantlist.go` — NEW `ListGuildWants` (store, read)

**Analog A (all-members read shape):** `assignment.go:550` `ListAllAssignments`:
```go
func ListAllAssignments(ctx context.Context, db *sql.DB) ([]Assignment, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT a.character_id, c.name, a.discord_user_id, a.assigned_at, a.assigned_by
		   FROM character_assignment a
		   JOIN character c ON c.id = a.character_id
		  WHERE c.is_removed = 0
		  ORDER BY c.name COLLATE NOCASE`)
```
**Analog B (scan + non-nil slice + JSON-pointer nullables):** `wantlist.go:108-133` `ListOwnWants` body (`out := make([]..., 0)` → JSON `[]`; `sql.Null*` → pointer; `rows.Err()` wrap).

**New `GuildWantRow` struct** mirrors `WantlistRow:52` but adds owner attribution (`DiscordUserID string` + `Owner string` from `wu.username`) and `CharacterName *string`, and per the Security recommendation **EXCLUDES `note`**. The query (per RESEARCH Pattern 2): `JOIN web_user wu ON wu.discord_user_id = w.discord_user_id` (owner display name) + `LEFT JOIN character c ON c.id = w.character_id` (nullable char name), `WHERE w.active = 1 ORDER BY w.created_at DESC`. Plain `*sql.DB`, parameterized, non-nil slice. Do NOT fork `ListOwnWants` and strip the owner scope inline (RESEARCH anti-pattern) — this is a distinct func.

---

### `internal/backendsrv/store/assignment.go` — NEW `IsCharAssignedToTx` (store, in-tx probe)

**Analog:** `assignment.go:92` `charSharedTx` — the in-tx existence-probe to mirror exactly:
```go
func charSharedTx(ctx context.Context, tx *sql.Tx, characterID int64) (bool, error) {
	var isBank, isBot int
	err := tx.QueryRowContext(ctx,
		`SELECT is_bank_toon, is_guild_bot FROM character WHERE id = ?`, characterID,
	).Scan(&isBank, &isBot)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("check char shared (character_id=%d): %w", characterID, err)
	}
	return isBank == 1 || isBot == 1, nil
}
```
`IsCharAssignedToTx(ctx, tx, characterID int64, callerID string) (bool, error)` is the same shape: `SELECT 1 FROM character_assignment WHERE character_id = ? AND discord_user_id = ?`, `sql.ErrNoRows` → `(false, nil)`, other err → `%w`-wrapped, a row → `(true, nil)`. The "AUTHORIZE-UNDER-TRANSACTION" discipline (assignment.go header lines 27-31) is why this is a `*Tx` probe, not a `*sql.DB` read — it runs inside `AddWantHandler`'s `withTx`. Add a typed sentinel beside lines 52-65, e.g. `ErrCharNotAssigned = errors.New("char_not_assigned")` (mirror `ErrCharAlreadyAssigned`).

---

### `internal/backendsrv/wantmatch/match.go` — `Hit.CharacterName` (service, event-driven)

**Analog:** self.

`Hit:38` gains `CharacterName *string` (mirror the existing `Note *string` nullable pointer, lines 44-47):
```go
type Hit struct {
	WantID        int64
	DiscordUserID string  // the DM target — DO NOT change (owner-targeting, CWANT-05)
	ItemID        *int64
	ItemName      string
	Reason        string
	Note          *string
}
```
**CRITICAL (Pitfall 3):** `DiscordUserID` (line 40) stays `wantlist_item.discord_user_id` — never derive the DM target from `character_id`. `ForItem:55` + `ForName:72` queries gain `LEFT JOIN character c ON c.id = w.character_id` + `c.name`. Current `ForItem` SELECT (lines 56-59):
```go
rows, err := db.QueryContext(ctx,
	`SELECT id, discord_user_id, item_id, item_name, reason, note
	   FROM wantlist_item
	  WHERE item_id = ? AND active = 1 AND muted = 0`, itemID)
```
`scanHits:86` gains a `sql.NullString characterName` scan → `*string` (clone the `note` nullable scan, lines 101-104):
```go
if note.Valid { v := note.String; h.Note = &v }
```

---

### `internal/backendsrv/ec/embed.go` — `buildEmbed` "For" field (utility, transform)

**Analog:** self — `buildEmbed:118`. After the "Why you wanted it" field (lines 146-150), append a conditional "For" field exactly like the existing OMIT-when-empty fields (Price `:122`, Seen `:130`, Seller `:138`):
```go
fields = append(fields, &discordgo.MessageEmbedField{
	Name:   "Why you wanted it",
	Value:  whyWanted(hit),
	Inline: false,
})
```
New (per RESEARCH Pattern 3): `if hit.CharacterName != nil && strings.TrimSpace(*hit.CharacterName) != "" { fields = append(fields, &discordgo.MessageEmbedField{Name: "For", Value: *hit.CharacterName, Inline: true}) }`. `strings` is already imported (line 22). Display-only — touches NO send path. The owner-targeting proof lives in `ec/ec.go` (`DiscordUserID: hit.DiscordUserID`) and `notify/dm.go` (`UserChannelCreate(a.DiscordUserID)`), both UNCHANGED.

---

### `internal/backendsrv/webadmin/wantlist.go` — `AddWantHandler` + NEW `ListGuildWantsHandler` (controller, request-response)

**Analog:** self.

`addWantReq:45` gains `CharacterID *int64 `json:"character_id"`` (mirror the existing `ItemID *int64` pointer-for-optional, lines 45-51). The IDOR guard threads into the existing `withTx` block at `AddWantHandler:144-152` (per RESEARCH Code Example):
```go
err := withTx(ctx, db, func(tx *sql.Tx) error {
	id, e := store.AddWantTx(ctx, tx, callerID, req.ItemID, itemName, req.Reason, priority, notePtr, now)
	if e != nil { return e }
	newID = id
	return AppendAuditTx(ctx, tx, "wantlist_add", callerID, map[string]any{"item_id": req.ItemID}, now)
})
```
Insert the `IsCharAssignedToTx` check **before** `AddWantTx` inside the tx; `req.CharacterID` is added to the `AddWantTx` call and (per V7) to the audit `map[string]any` as `"character_id"` (ids only — never note/name). `mapWantErr:92` gains a case mapping `store.ErrCharNotAssigned` → 403 (mirror the `errors.Is(err, store.ErrDuplicateWant)` → 409 case, lines 94-95). `validWant:67` optionally validates `CharacterID` is a positive int when non-nil (V5), but authorization is the in-tx guard (V4).

**NEW `ListGuildWantsHandler`** — clone `ListOwnWantsHandler:228` verbatim, swapping `store.ListOwnWants(ctx, db, callerID)` for `store.ListGuildWants(ctx, db)` (no caller scope — guild-wide):
```go
func ListOwnWantsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
		ctx := r.Context()
		callerID := caller(r.Context())
		out, err := store.ListOwnWants(ctx, db, callerID)
		if err != nil { slog.Error(...); writeJSONError(w, http.StatusInternalServerError, "internal"); return }
		writeJSON(w, out)
	}
}
```

---

### `cmd/squirebot-server/main.go` — register `GET /api/v1/wantlist/guild` (route)

**Analog:** the wantlist route block, `main.go:342-344`:
```go
mux.Handle("GET /api/v1/wantlist", webauth.RequireSession(db, webadmin.ListOwnWantsHandler(db)))
mux.Handle("POST /api/v1/wantlist", webauth.RequireSession(db, webadmin.AddWantHandler(db)))
mux.Handle("POST /api/v1/wantlist/remove", webauth.RequireSession(db, webadmin.RemoveOwnWantHandler(db)))
```
Add: `mux.Handle("GET /api/v1/wantlist/guild", webauth.RequireSession(db, webadmin.ListGuildWantsHandler(db)))` — `RequireSession` NOT `RequireOfficer` (login-gated like the whole read API since P15).

---

### `web/src/lib/api.ts` — `WantlistRow` + `addWant` + NEW `fetchGuildWants` (utility, client)

**Analog:** self.

`WantlistRow:580` gains `character_id: number | null` + `character_name: string | null` (mirror the existing `item_id: number | null` / `note: string | null`, lines 580-595). `addWant:616` body gains `character_id?: number | null`:
```ts
export function addWant(
	body: { item_id: number | null; item_name: string; reason: 'buy' | 'quest'; priority: 'low' | 'med' | 'high'; note?: string; },
	f: typeof fetch = fetch
): Promise<WantlistRow> {
	return postJSON<WantlistRow>('/api/v1/wantlist', body, f);
}
```
**NEW `GuildWantRow` interface + `fetchGuildWants`** — clone `fetchOwnWants:606` for the new route:
```ts
export function fetchOwnWants(f: typeof fetch = fetch): Promise<WantlistRow[]> {
	return getJSON<WantlistRow[]>('/api/v1/wantlist', f);
}
```
→ `fetchGuildWants(f)` calls `getJSON<GuildWantRow[]>('/api/v1/wantlist/guild', f)`. `GuildWantRow` mirrors the Go `GuildWantRow` (item + reason + priority + `owner` + `character_name`, NO `note`). The tag-select source `fetchMyCharacters:906` + `MyCharacter:832` already exist — reuse, do NOT add a new my-characters fetch.

---

### `web/src/lib/wantlist/groupByChar.ts` — NEW (utility, pure helper)

**Analog:** `web/src/lib/myview.ts` — the DOM-free helper to clone. The full file is the template (pure functions, `import type` only, node-testable under the `server`/`environment:node` vitest project). Header doctrine (lines 1-18) explains WHY it's a plain `.ts` not `.svelte` and the LOAD-BEARING note that this is presentation, NOT a security boundary. The filter signature to mirror (lines 36-48):
```ts
export function applyMyFilter<T extends { char: string }>(
	rows: T[], mineNames: Set<string>, mineOnly: boolean, selectedChar: string | null
): T[] {
	if (selectedChar) { const sel = selectedChar.toLowerCase(); return rows.filter((r) => r.char.toLowerCase() === sel); }
	if (!mineOnly) return rows;
	return rows.filter((r) => mineNames.has(r.char.toLowerCase()));
}
```
`groupByChar(wants: WantlistRow[], charId: number | null)` is the identical shape over `WantlistRow[]` keyed on `character_id` (or `character_name`). Co-locate the node test as `web/src/lib/wantlist/groupByChar.test.ts` (the `holders.test.ts` / `priority.test.ts` precedent already in `web/src/lib/wantlist/`).

---

### `web/src/lib/components/WantAddForm.svelte` — character `<select>` (component, request-response)

**Analog:** self — the Reason/Priority `<select>` block (lines 207-222) is the exact markup to clone for the new optional character tag:
```svelte
<FormField label="Reason" id="want-reason">
	<select id="want-reason" class="field" bind:value={reason}>
		<option value="" disabled>Choose…</option>
		<option value="buy">Buy</option>
		<option value="quest">Quest</option>
	</select>
</FormField>
```
The new tag select is sourced from `fetchMyCharacters()` (load in `onMount`/`$effect`, store `$state<MyCharacter[]>`), with a default "(no character)" option for the untagged/account-level case (`value=""` → `character_id: null`). `submit:111-126` threads `character_id` into the `addWant({...})` call (lines 118-124) — add `character_id: pickedCharId` to that body. `$state`/`$derived`/`$props` runes pattern is established (lines 22-53). XSS: `character.name` renders via plain `{}` (the T-19-13 boundary, header lines 8-13) — never `{@html}`.

---

### `web/src/lib/components/WantlistPanel.svelte` — group-by-char + guildwide toggle (component, request-response)

**Analog:** self + the P27 `myview.ts` `<select>`-driven filter precedent.

The `onMount` load → server-truth-reload lifecycle (lines 92-117) is the shape: `load()` does `Promise.all([fetchOwnWants(), fetchView()])`; for the guildwide toggle add `fetchGuildWants()` to a parallel/lazy load. The `$derived` memo pattern (lines 69-86) is how a pure helper feeds the grid — `groupByChar` plugs in here, fed by a `<select>` of the caller's characters (from `fetchMyCharacters`). The My/Guild toggle (RESEARCH Open Q2 recommendation: option c) mirrors P27's My/All filter control on a view `+page.svelte` — a `$state` toggle selecting which row source (`fetchOwnWants` vs `fetchGuildWants`) + which columns feed the existing `DataGrid`. The guildwide grid shows owner + character attribution columns; reuse `wantlistColumns` shape (line 24, `$lib/columns`). NEVER optimistic-mutate (T-19-16) — always re-fetch (lines 106-117).

---

## Shared Patterns

### Session-derived identity (V4/D-02) — never the body
**Source:** `webadmin/wantlist.go:140` `callerID := caller(r.Context())`; `assignment.go` header lines 24-31.
**Apply to:** `AddWantHandler` (the IDOR guard's `callerID`), `ListGuildWantsHandler` (RequireSession gate). The `character_id` arrives in the body but is AUTHORIZED against the session caller via `IsCharAssignedToTx` — the client `<select>` is never trusted.

### Atomic mutation + audit (one withTx)
**Source:** `webadmin/wantlist.go:144-152` (`withTx` + `AppendAuditTx`).
**Apply to:** `AddWantHandler`. The IDOR check, `AddWantTx`, and the audit all run in the SAME tx. V7: audit detail carries `item_id`/`character_id` (ids ONLY) — never note text, never character name.

### Nullable column → JSON-null pointer
**Source:** `store/wantlist.go:52-61` (`*int64 ItemID` / `*string Note`) + the `sql.Null*` → pointer scan (`:110-126`); `wantmatch/match.go:101-104`.
**Apply to:** every new `character_id`/`character_name` field on `WantlistRow`, `GuildWantRow`, `Hit`, and the api.ts interfaces. NULL ⇒ JSON `null` ⇒ untagged/account-level.

### Non-nil slice → JSON `[]`
**Source:** `store/wantlist.go:108` (`out := make([]WantlistRow, 0)`); `wantmatch/match.go:87`.
**Apply to:** `ListGuildWants` (so the empty guild list is `[]` not `null`).

### Partial-unique dedup via extended result code
**Source:** `store/wantlist.go:46` `sqliteConstraintUnique = 2067` + `:79-82` detection.
**Apply to:** `AddWantTx` UNCHANGED — the COALESCE-rewritten index still raises 2067 → `ErrDuplicateWant` → 409.

### DOM-free pure helper + co-located node test
**Source:** `web/src/lib/myview.ts` + `web/src/lib/__tests__/myview.test.ts`; the `web/src/lib/wantlist/*.test.ts` precedent (`holders.test.ts`, `priority.test.ts`).
**Apply to:** `groupByChar.ts` + `groupByChar.test.ts`. Node vitest is DOM-blind — the `<select>` rendering/onchange in the Svelte components is a browser-smoke gap (Pitfall 4): smoke on a DEPLOYED build, never `npm run dev` (login bounces to prod).

### Plain `{}` auto-escape (XSS boundary T-19-13)
**Source:** `WantAddForm.svelte` header lines 8-13; `WantlistPanel.svelte` header lines 13-16.
**Apply to:** rendering `character_name` / `owner` in both the personal and guildwide UI — never `{@html}`. discordgo embed fields are plain-text (`ec/embed.go` — the "For" field value is not markdown-evaluated).

---

## No Analog Found

None. Every file in this phase has a concrete in-repo analog (most are self-extensions). The two "net-new" constructions (`ListGuildWants` + `IsCharAssignedToTx`) each have a near-exact role-analog (`ListAllAssignments` / `charSharedTx`) — they are new functions in existing files following an established shape, not new patterns.

## Metadata

**Analog search scope:** `internal/backendsrv/{migrations,store,wantmatch,ec,notify,webadmin}`, `cmd/squirebot-server`, `web/src/lib/{,components,wantlist,__tests__}`, `web/src/routes/wantlist`.
**Files scanned (read):** 13 (00006/00009 migrations, migrate_test.go, store/wantlist.go, store/assignment.go ×2 ranges, wantmatch/match.go, ec/embed.go, webadmin/wantlist.go, main.go, api.ts ×2 ranges, myview.ts, wantlist/+page.svelte, WantAddForm.svelte ×2 ranges, WantlistPanel.svelte).
**Pattern extraction date:** 2026-06-08
