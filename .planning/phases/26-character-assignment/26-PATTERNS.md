# Phase 26: Character Assignment - Pattern Map

**Mapped:** 2026-06-08
**Files analyzed:** 17 (8 new, 9 modified)
**Analogs found:** 17 / 17 (every file has a strong in-repo analog — zero new infrastructure)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00009_character_assignment.sql` | migration | batch (DDL+seed) | `migrations/00006_wantlist.sql` (+ `00005`) | exact (partial-unique + CHECK + soft-state + auto-seed) |
| `internal/backendsrv/store/assignment.go` | store | CRUD + state-machine | `store/admins.go` (`AddOfficerTx`/`IsOfficerTx`) + `store/charmeta.go` (`SetCharMetaTx`) | exact (authorize-under-tx + typed errors + UPDATE-WHERE-scoped) |
| `internal/backendsrv/store/assignment_test.go` | test | — | `store/charmeta_test.go` (demote test) + `admins` tests | role-match |
| `internal/backendsrv/webadmin/assignment.go` (member) | controller | request-response | `webadmin/wantlist.go` (`AddWantHandler`/`RemoveOwnWantHandler`) | exact |
| `internal/backendsrv/webadmin/assignment_admin.go` (officer) | controller | request-response | `webadmin/officers.go` (`OfficerAddHandler`/`OfficerRemoveHandler`) | exact |
| `internal/backendsrv/webadmin/assignment_test.go` | test | — | `webadmin` handler tests (wantlist/officers) | role-match |
| `internal/backendsrv/store/charmeta.go` (MODIFY) | store | CRUD | itself — drop demote + `isBankToon` param | self-edit |
| `internal/backendsrv/compute/bank.go` (MODIFY) | compute | transform | itself — stale doc comment only (no query change) | self-edit |
| `internal/backendsrv/webadmin/charmeta.go` (MODIFY) | controller | request-response | itself — remove `IsBankToon` from req + payload | self-edit |
| `cmd/squirebot-server/main.go` (MODIFY) | route | request-response | lines 308–372 (RequireOfficer/RequireSession blocks) | exact |
| `web/src/lib/components/MyCharactersPanel.svelte` (NEW) | component | request-response | `WatcherCodesPanel.svelte` | exact (load→list→confirm-commit) |
| `web/src/lib/components/AssignmentAdminPanel.svelte` (NEW) | component | request-response | `MonitorAdminPanel.svelte` (officer panel) + `AdminMgmtForm.svelte` | role-match |
| `web/src/lib/api.ts` (MODIFY) | utility | request-response | `getJSON`/`postJSON` + the wantlist/charmeta typed fns | self-edit (additive) |
| `web/src/lib/components/CharMetaForm.svelte` (MODIFY) | component | request-response | itself — remove bank-toon checkbox (lines 199–200) | self-edit |
| `web/src/lib/charmeta.ts` (MODIFY) | utility | transform | itself — drop `isBankToon` field (lines 68/130/136/151/167) | self-edit |
| `web/src/routes/my-characters/+page.svelte` (NEW) | route | request-response | `web/src/routes/account/+page.svelte` | role-match |
| `web/src/routes/admin/+page.svelte` (MODIFY) | route | request-response | itself — add `<section class="form-card">` for AssignmentAdminPanel | self-edit |
| `web/src/lib/components/SettingsMenu.svelte` (MODIFY) | component | — | itself — add `<a href="/my-characters">` beside line 187–188 | self-edit |

> Schema-versioning note for the planner: there is NO `_meta.schema_version` write in `00009` (that is Apps-Script-era). "Schema v9" == goose migration `00009` applied. Do NOT add a version-cell write.

---

## Pattern Assignments

### `internal/backendsrv/migrations/00009_character_assignment.sql` (migration, DDL+seed)

**Analog:** `migrations/00006_wantlist.sql` (partial-unique + CHECK + soft-state) and `00005_self_service_linking.sql` (`ALTER ADD COLUMN` + FK + partial unique index).

**`ALTER ADD COLUMN` + partial-unique idiom** (`00005:20-23` — SQLite can't `ALTER ADD UNIQUE`):
```sql
ALTER TABLE owner ADD COLUMN discord_user_id TEXT REFERENCES web_user(discord_user_id);
CREATE UNIQUE INDEX owner_discord_user_id_uidx
  ON owner(discord_user_id) WHERE discord_user_id IS NOT NULL;  -- one owner per Discord id; many NULLs ok
```

**CHECK-constraint + partial-unique-WHERE-soft-state idiom** (`00006:25-34` — the status enum + the `WHERE active=1` scoped partial unique to mirror for `WHERE status='pending'`):
```sql
reason          TEXT NOT NULL CHECK (reason IN ('buy','quest')),   -- CHECK = DB-level defense-in-depth
...
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(discord_user_id, item_id, reason)   WHERE item_id IS NOT NULL AND active = 1;
```

**Down is an explicit no-op** (`00005:25-29`, `00006:59-61`):
```sql
-- +goose Down
-- Forward-only in practice (mirrors 00004/00005): explicit no-op.
SELECT 1;
```

The full recommended `00009` body (the two tables + `is_guild_bot` column + idempotent `INSERT OR IGNORE … SELECT` auto-seed) is spelled out verbatim in `26-RESEARCH.md` § "Recommended `00009` Schema" — copy it. Key mechanics:
- `character_assignment.character_id` is the **PRIMARY KEY** → mechanically guarantees D-01 single-assignee; reassign is `INSERT … ON CONFLICT(character_id) DO UPDATE`.
- `assignment_request_pending_uidx … WHERE status='pending'` → at-most-one-pending per (char,requester); the `00006` partial-unique precedent (SQLite NULL-distinct rule).
- Auto-seed (D-04) excludes `is_removed=1 OR is_bank_toon=1 OR is_guild_bot=1`; `assigned_by='migration'`.

---

### `internal/backendsrv/store/assignment.go` (store, CRUD + state-machine)

**Analog:** `store/admins.go` (`AddOfficerTx` authorize-under-tx + `IsOfficerTx`) and `store/charmeta.go` (`SetCharMetaTx` UPDATE-WHERE-scoped + typed `ErrCharNotFound`).

**Authorize-under-transaction officer mutator** (`admins.go:269-289` — copy this shape for `OfficerAssignTx`/`RemoveAssignTx`/`ApproveRequestTx`/`DenyRequestTx`/`DesignateCharTx`):
```go
func AddOfficerTx(ctx context.Context, tx *sql.Tx, targetID, callerID string, now int64) (added bool, err error) {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotAuthorized
	}
	res, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO guild_admins (discord_user_id, added_at, added_by) VALUES (?, ?, ?)`,
		targetID, now, callerID,
	)
	...
	n, err := res.RowsAffected()
	return n > 0, nil // n==0 ⇒ already existed (idempotent, no-op)
}
```

**The exported in-tx re-check the member-composed handlers call** (`admins.go:110-112`) — for officer paths whose store mutator does NOT self-authorize (eviction precedent), the *handler* calls this first:
```go
func IsOfficerTx(ctx context.Context, tx *sql.Tx, discordUserID string) (bool, error) {
	return isOfficerTx(ctx, tx, discordUserID)
}
```

**Typed sentinel error + RowsAffected==0 → typed error** (`charmeta.go:29` + `:79-82`) — define `ErrCharAlreadyAssigned`, `ErrCharShared`, `ErrNotAssignee` the same way:
```go
var ErrCharNotFound = errors.New("char_not_found")
...
n, _ := res.RowsAffected()
if n == 0 {
	return ErrCharNotFound
}
```

**Owner-scoped silent-no-op release** (the wantlist `RemoveOwnWantTx` pattern — see `wantlist.go:275-286` caller) — `ReleaseCharTx` is `DELETE … WHERE character_id=? AND discord_user_id=caller`, returns `(removed bool)`; a cross-actor row affects 0 and returns `false` (never leaks existence).

**The single-bank demote to DELETE** (`charmeta.go:64-71`) — this block is what the reconciliation REMOVES from `SetCharMetaTx`; the new `DesignateCharTx` does NOT demote (multiple guild banks allowed), and instead in the same tx DELETEs any `character_assignment` for the designated char + marks pending `assignment_request`s denied (D-02 exemption, Pitfall 6).

Store funcs to create (per `26-RESEARCH.md` § file structure): `ClaimCharTx`, `ReleaseCharTx`, `RequestTx`, `CancelRequestTx`, `OfficerAssignTx`, `RemoveAssignTx`, `ApproveRequestTx`, `DenyRequestTx`, `DesignateCharTx`, `ListMyAssignments`, `ListAllAssignments`, `ListPendingRequests` + typed errors. `ApproveRequestTx` must, in the same tx, deny all OTHER pending requests for that `character_id` (Pitfall 3, double-approval).

---

### `internal/backendsrv/webadmin/assignment.go` (controller, member, request-response)

**Analog:** `webadmin/wantlist.go` (`AddWantHandler` / `RemoveOwnWantHandler` / `MuteWantHandler`). The research already wrote the target `ClaimCharHandler` (§ Pattern 1) from this template.

**Member-CRUD handler spine** (`wantlist.go:115-168`) — method-check → decode body (NO identity field) → `caller(ctx)` from SESSION → `withTx(store mutator + AppendAuditTx)` → typed-error map → `writeJSON`:
```go
callerID := caller(r.Context()) // D-02: owner from session, body carries NO owner
now := nowUnix()
var newID int64
err := withTx(ctx, db, func(tx *sql.Tx) error {
	id, e := store.AddWantTx(ctx, tx, callerID, req.ItemID, itemName, req.Reason, priority, notePtr, now)
	if e != nil {
		return e // ErrDuplicateWant → mapWantErr → 409 duplicate
	}
	newID = id
	// V7: detail carries item_id ONLY — never the note text.
	return AppendAuditTx(ctx, tx, "wantlist_add", callerID, map[string]any{"item_id": req.ItemID}, now)
})
if err != nil {
	mapWantErr(w, err)
	return
}
```

**Owner-scoped silent-no-op + audit-only-on-real-change** (`wantlist.go:259-292` — copy for `ReleaseCharHandler` / `CancelRequestHandler`):
```go
var removed bool
err := withTx(ctx, db, func(tx *sql.Tx) error {
	var e error
	removed, e = store.RemoveOwnWantTx(ctx, tx, req.ID, callerID)
	if e != nil { return e }
	if removed { // audit ONLY a real change (an idempotent no-op need not spam the log)
		return AppendAuditTx(ctx, tx, "wantlist_remove", callerID, map[string]any{"want_id": req.ID}, now)
	}
	return nil
})
...
writeJSON(w, map[string]any{"removed": removed})
```

**Typed-error → HTTP map with `errors.Is` on the store sentinel** (`wantlist.go:92-100`) — build `mapAssignErr` the same way (`ErrCharAlreadyAssigned`/`ErrCharShared` → 409; `ErrNotAuthorized` → 403; default 500):
```go
func mapWantErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrDuplicateWant):
		writeJSONError(w, http.StatusConflict, "duplicate")
	default:
		slog.Error("wantlist write failed", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}
```

Member handlers to create: `ListMyAssignmentsHandler` (GET), `ClaimableHandler` (GET), `ClaimCharHandler`, `ReleaseCharHandler`, `RequestCharHandler`, `CancelRequestHandler`. Pitfall 1: the body carries ONLY `character_id` — never a `discord_user_id`.

---

### `internal/backendsrv/webadmin/assignment_admin.go` (controller, officer, request-response)

**Analog:** `webadmin/officers.go` (`OfficerAddHandler` / `OfficerRemoveHandler`) — RequireOfficer at the route + the in-tx re-check inside the store mutator.

**Officer handler spine** (`officers.go:108-149`) — decode → `caller(ctx)` → `withTx(store *Tx that re-checks IsOfficer first + audit)` → `mapOfficerErr`:
```go
var added bool
err := withTx(ctx, db, func(tx *sql.Tx) error {
	var e error
	added, e = store.AddOfficerTx(ctx, tx, req.DiscordUserID, callerID, now)
	if e != nil { return e }
	if added { // audit only a real promotion
		return AppendAuditTx(ctx, tx, "officer_add", callerID, map[string]any{"target": req.DiscordUserID}, now)
	}
	return nil
})
if err != nil { mapOfficerErr(w, err, "officer_add"); return }
```

**Officer error map** (`officers.go:203-213` — `not_authorized` → 403):
```go
func mapOfficerErr(w http.ResponseWriter, err error, op string) {
	switch {
	case errors.Is(err, store.ErrOwnerFloorProtected):
		writeJSONError(w, http.StatusForbidden, "owner_floor_protected")
	case errors.Is(err, store.ErrNotAuthorized):
		writeJSONError(w, http.StatusForbidden, "not_authorized")
	default:
		slog.Error("officer write failed", "op", op, "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}
```

> The officer path legitimately reads a *target* from the body (the assignee for `OfficerAssignHandler`; the `character_id` + bank/bot mode for `DesignateCharHandler`). Validate the target assignee against `web_user` (the `usernameOf` lookup at `officers.go:218-227` is the precedent). Identity of the ACTOR is still `caller(ctx)`, never the body.

Officer handlers to create: `ListAllAssignmentsHandler` (GET, + pending requests), `OfficerAssignHandler`, `OfficerRemoveAssignHandler`, `ApproveRequestHandler`, `DenyRequestHandler`, `DesignateCharHandler`.

---

### `cmd/squirebot-server/main.go` (route, MODIFY — additive)

**Analog:** the existing wantlist/notify/char-meta blocks at lines 325–372. Mirror the member (`RequireSession`) vs officer (`RequireOfficer`) split exactly. The research § "Officer route registration" gives the literal `mux.Handle` lines to add:
```go
// Member (RequireSession) — mirrors the wantlist block at main.go:340-342
mux.Handle("GET  /api/v1/assignments/mine",    webauth.RequireSession(db, webadmin.ListMyAssignmentsHandler(db)))
mux.Handle("POST /api/v1/assignments/claim",   webauth.RequireSession(db, webadmin.ClaimCharHandler(db)))
mux.Handle("POST /api/v1/assignments/release", webauth.RequireSession(db, webadmin.ReleaseCharHandler(db)))
// ...request/request-cancel/claimable
// Officer (RequireOfficer) — mirrors the officers block at main.go:308-310
mux.Handle("POST /api/v1/admin/assignments/assign",  webauth.RequireOfficer(db, webadmin.OfficerAssignHandler(db)))
mux.Handle("POST /api/v1/admin/characters/designate", webauth.RequireOfficer(db, webadmin.DesignateCharHandler(db)))
// ...remove/approve/deny + GET /api/v1/admin/assignments
```
Char-meta re-route (reconciliation): the bank-toon write moves OFF the member `POST /api/v1/char/meta` (line 326) and ONTO the new officer designate route. Leave `class`/`level`/`race` on the member path.

---

### `internal/backendsrv/store/charmeta.go` (store, MODIFY — reconciliation)

**Self-edit.** Drop the demote block (`charmeta.go:64-71`, shown above) and the `isBankToon` param from `SetCharMetaTx` (line 51 signature + line 73 SQL). The bank-toon write relocates to the new `DesignateCharTx` in `assignment.go`, which does NOT demote. Update the MD-01 doc comment (`charmeta.go:39-50`).

---

### `internal/backendsrv/compute/bank.go` (compute, MODIFY — doc comment only)

**Self-edit, NO query change.** The `WHERE c.is_bank_toon = 1` query in `store.InventoryJoin(bankOnly)` already returns ALL guild banks. Only the stale single-bank doc comment changes (`bank.go:22-31`):
```go
// CURRENT (stale): "...scoped to the single is_bank_toon character; Char is constant within it."
// + the whole "single is_bank_toon character assumption is upheld by the write side... demotes any
//   other live bank toon" paragraph (lines 27-31) — DELETE it.
// REPLACE with: all guild-bank characters; rows carry their Char (consolidated, multiple banks supported).
```
The consolidated `Char`-column grid (`buildViewRows`) already groups N characters — this is the LOCKED consolidated-views architecture making N bank toons safe (CLAUDE.md). Add a 2-bank `compute/bank_test.go` case to prove it (Pitfall 5).

---

### `internal/backendsrv/webadmin/charmeta.go` (controller, MODIFY — reconciliation)

**Self-edit.** Remove `IsBankToon` from `charMetaReq` (`charmeta.go:46`), the `store.SetCharMetaTx(... req.IsBankToon)` arg (`:106`), and `is_bank_toon` from the echo payload (`:141`). Everything else (the `validCharMeta` value-set re-check at `:155-166`, the `withTx`+audit) stays.

---

### `web/src/lib/components/MyCharactersPanel.svelte` (component, NEW)

**Analog:** `WatcherCodesPanel.svelte` — the member self-service rhythm.

**`<script module>` pure DOM-free helpers for node-vitest** (`WatcherCodesPanel.svelte:1-45`) — put any date/status formatting here (Pitfall 7: web tests are node-only, no jsdom):
```svelte
<script lang="ts" module>
	// Pure, DOM-free helpers split into the module block so they're unit-testable
	// under the node vitest project (the repo runs vitest with NO jsdom).
	export function formatLastSeen(lastSeen: string | null, now: number = Date.now()): string { ... }
</script>
```

**Load→phase→AuthGuard-route lifecycle** (`WatcherCodesPanel.svelte:83-114`) — `getContext<AuthGuard>`, `$state<Phase>('loading')`, `onMount→load()`, a 401 routes to LoginScreen via the guard:
```svelte
const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);
type Phase = 'loading' | 'error' | 'ready';
let phase = $state<Phase>('loading');
let codes = $state<OwnCode[]>([]);
async function load() {
	phase = 'loading';
	try { codes = await fetchOwnCodes(); phase = 'ready'; }
	catch (err) { if (route(err)) return; phase = 'error'; }
}
```

**Confirm-before-commit destructive action** — reuse `ConfirmDialog.svelte` for Release (the `revokeTarget`/`revokeDialogOpen`/`revoking` state at `WatcherCodesPanel.svelte:98-103`). Claim = instant for unassigned; Request = a button that files; Release = ConfirmDialog.

---

### `web/src/lib/components/AssignmentAdminPanel.svelte` (component, NEW)

**Analog:** `MonitorAdminPanel.svelte` (officer panel, already a `<section class="form-card">` child of `/admin`) + `AdminMgmtForm.svelte` (the pick-a-user-and-act rhythm). Same `getContext<AuthGuard>` 403→officers-only collapse. Renders: the all-assignments table (assign/reassign/remove), the pending-requests queue (approve/deny), the designate guild-bank/bot radio.

---

### `web/src/lib/api.ts` (utility, MODIFY — additive)

**Self-edit.** Add typed interfaces + `getJSON`/`postJSON` wrappers mirroring the wantlist/charmeta fns. The wrappers already exist and carry the `credentials:'include'` + `Unauthenticated`/`Forbidden` typed-error contract (`api.ts:154-269`) — just add public fns:
```ts
export function fetchMyCharacters(fetchFn = fetch) { return getJSON<MyCharacter[]>('/api/v1/assignments/mine', fetchFn); }
export function claimChar(character_id: number, fetchFn = fetch) { return postJSON<ClaimResult>('/api/v1/assignments/claim', { character_id }, fetchFn); }
// ...release / request / cancel / officer assign / designate
```
Field names are snake_case to match the Go JSON contract (the existing `BankToon`/`CharMetaItem` interfaces at `api.ts:274-281` are the precedent).

---

### `web/src/routes/admin/+page.svelte` (route, MODIFY)

**Self-edit.** Add one `<section class="form-card">` wrapping `<AssignmentAdminPanel />`, exactly like the Monitors section already added (`admin/+page.svelte:49-52`):
```svelte
<section class="form-card">
	<h2 class="form-title">Monitors</h2>
	<MonitorAdminPanel />
</section>
```
The Layer-1 `{#if !isOfficer}` officers-only refusal (`:36-38`) already gates the whole page; the server is the real gate.

---

### `web/src/routes/my-characters/+page.svelte` (route, NEW) + `SettingsMenu.svelte` (MODIFY)

**Analog:** `web/src/routes/account/+page.svelte` (a thin page that renders `WatcherCodesPanel`). The new page renders `<MyCharactersPanel />`. Add a nav link in `SettingsMenu.svelte` beside the existing member links (`SettingsMenu.svelte:187-188`):
```svelte
<a href="/account" role="menuitem" class="menu-link">Watcher codes</a>
<a href="/char-meta" role="menuitem" class="menu-link">Character details</a>
<!-- + <a href="/my-characters" role="menuitem" class="menu-link">My characters</a> -->
```

---

## Shared Patterns

### Atomic write+audit (`withTx`)
**Source:** `internal/backendsrv/webadmin/audit.go:88-107`
**Apply to:** EVERY mutating handler (member + officer). BEGIN IMMEDIATE (the store DSN sets `_txlock=immediate`) + panic-safe deferred rollback. Compose the store mutator + `AppendAuditTx` in ONE `fn`.
```go
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE
	if err != nil { return fmt.Errorf("begin webadmin tx: %w", err) }
	committed := false
	defer func() { if !committed { _ = tx.Rollback() } }()
	if ferr := fn(tx); ferr != nil { return ferr }
	if cerr := tx.Commit(); cerr != nil { return fmt.Errorf("commit webadmin tx: %w", cerr) }
	committed = true
	return nil
}
```

### Audit append (ASSIGN-06)
**Source:** `internal/backendsrv/webadmin/audit.go:57-69`
**Apply to:** every assignment mutation. `event` is the action name (`assignment_claim`/`assignment_release`/`assignment_request`/`request_cancel`/`officer_assign`/`assignment_remove`/`request_approve`/`request_deny`/`char_designate`); `detail` carries `character_id` ONLY (V7 — no PII beyond the keyed discord_user_id, D-10):
```go
return AppendAuditTx(ctx, tx, "assignment_claim", callerID,
	map[string]any{"character_id": req.CharacterID}, now)
```

### Identity from session (D-02 / Pitfall 1)
**Source:** `internal/backendsrv/webadmin/officers.go:52-61`
**Apply to:** every handler. `caller(ctx)` reads the gate-placed discord_user_id; fail-closed (`""` makes the in-tx officer re-check reject). The request body NEVER carries the actor's identity.
```go
var nowUnix = func() int64 { return time.Now().Unix() }
func caller(ctx context.Context) string {
	uid, _ := webauth.UserFromContext(ctx)
	return uid
}
```

### Route gates (server is truth — Pitfall 2)
**Source:** `cmd/squirebot-server/main.go:308-342` + `webauth.RequireSession`/`RequireOfficer`
**Apply to:** member routes = `RequireSession`; officer routes = `RequireOfficer` AND the store mutator re-checks `IsOfficerTx` in-tx (WR-04 TOCTOU). The frontend nav suppression is UX, never the boundary.

### `writeJSON` / `writeJSONError`
**Source:** `internal/backendsrv/webadmin/officers.go:37-48`
**Apply to:** all handler responses. `{"error":"code"}` body shape the frontend `readErrorCode` routes on.

### Migrate test (verify `00009`)
**Source:** `internal/backendsrv/migrations/migrate_test.go:328-392` (`TestMigrate_00006_AddsWantlist`) + `:511` (`TestMigrate_00008_AddsECCursor`)
**Apply to:** a new `TestMigrate_00009_*` case. Mirror: `tableExists` for both new tables; `columnSet` for `is_guild_bot`; `indexExists` for `assignment_request_pending_uidx`; a CHECK probe (`status='bogus'` rejected); an auto-seed probe (a linked-owner char assigned, a NULL-owner char + a bank toon skipped); and a second `RunMigrations` is a clean no-op (idempotency).
```go
if !indexExists(t, db, "assignment_request", "assignment_request_pending_uidx") { ... }
if _, err := db.Exec(`INSERT INTO assignment_request (... status ...) VALUES (..., 'bogus', ...)`); err == nil {
	t.Errorf("expected status='bogus' to fail the CHECK")
}
if err := migrations.RunMigrations(db); err != nil { t.Fatalf("second RunMigrations should be a no-op: %v", err) }
```

---

## No Analog Found

None. Every file has a strong in-repo analog. The single genuinely-new artifact is the `is_guild_bot` column — but it copies the existing boolean-flag idiom (`is_bank_toon`/`is_hidden`/`is_removed` on `character`, `00001_init.sql`) exactly; it is "new data, old pattern," not "new pattern."

---

## Metadata

**Analog search scope:** `internal/backendsrv/{migrations,store,webadmin,compute,webauth}/`, `cmd/squirebot-server/`, `web/src/{lib,lib/components,routes}/`
**Files scanned:** ~22 read in full or targeted (migrations 00005/00006, store admins/charmeta, compute/bank, webadmin wantlist/officers/audit/charmeta, migrate_test, api.ts, WatcherCodesPanel, admin+page, SettingsMenu, CharMetaForm, charmeta.ts, main.go routes)
**Pattern extraction date:** 2026-06-08
