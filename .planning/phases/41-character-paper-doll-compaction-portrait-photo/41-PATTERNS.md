# Phase 41: Character paper-doll — compaction + portrait photo - Pattern Map

**Mapped:** 2026-07-15
**Files analyzed:** 11 (6 backend + 3 web + 2 shared/type mirrors)
**Analogs found:** 10 / 11 (one file — the byte-streaming serve handler — has only a partial analog; called out under "No Analog Found")

This phase is backend (Go) + web (Svelte) + one new goose migration (**→ 00019**). Watcher is UNTOUCHED → **no `v*` tag**. Every value in the layout / control / copy contract is already pinned by `41-UI-SPEC.md`; this map answers "what existing code does each new/modified file copy its shape from."

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00019_character_portrait.sql` | migration | schema (additive side table) | `migrations/00009_character_assignment.sql` (side table + FK) + `00016_item_flags_effects.sql` (additive-columns header) | exact |
| `internal/backendsrv/store/portrait.go` | store | CRUD (blob upsert/get/delete) + IDOR gate | `store/assignment.go` (`IsCharAssignedToTx:119`, `charSharedTx:97`, `isOfficerTx` in-tx re-check) | role+flow match |
| `internal/backendsrv/webadmin/portrait.go` (upload POST + DELETE) | controller | request-response (JSON-in → tx → audit) | `webadmin/charmeta.go:87-150` (`CharMetaSetHandler`) | exact |
| `internal/backendsrv/readapi/portrait.go` (`GET …/{name}/portrait`, serve raw bytes) | controller | file-I/O / byte streaming | `readapi/inventory.go` (`{char}` wildcard + session gate + name-by-value) — **but serves raw bytes, not JSON** | partial (see No Analog) |
| `internal/backendsrv/readapi/characters.go` (`rosterChar:34` + additive `has_portrait`/`portrait_updated_at`) | controller | request-response (additive read field) | itself (extend in place) + `00016` additive-field precedent | exact |
| `cmd/squirebot-server/main.go` (register 3 routes with gates) | config/route | request-response | `main.go:340-341` (char-meta login-only) + `:361-364` (wishlist login-only, in-tx IDOR) | exact |
| `web/src/lib/components/InventoryWindow.svelte` (`.doll → portrait frame`, gap 24→16, portrait `<img>`) | component | request-response render | itself (`:159-162`, `:338-372`, `:479-483`) + `PaperdollSlot.svelte:91-124` (img + `onImgError`) | exact |
| Portrait upload/remove control (new component OR inline in InventoryWindow) | component | request-response (file → base64 → POST) | `web/src/lib/components/CharMetaForm.svelte` (whole file — states, `.primary`, `.result`, spinner, a11y) | role match (no file-input precedent) |
| `web/src/lib/api.ts` (`RosterCharacter:160`/`CharacterInventory:209` fields + portrait wrappers) | utility/types | request-response | `api.ts:616` (`saveCharMeta` → `postJSON`) + `getJSON:272` + `API_BASE:21` | exact |

---

## Pattern Assignments

### `internal/backendsrv/migrations/00019_character_portrait.sql` (migration, additive side table)

**Analog:** `migrations/00009_character_assignment.sql` (side table + FK + backend-only header) and `migrations/00016_item_flags_effects.sql` (the additive/forward-only header rationale).

**Header + forward-only discipline** — copy the `00016` boilerplate ("Forward-only; 00001-00018 are shipped and NOT edited… the watcher is OFF the read path, so NO WatcherMaxSchemaVersion gate is touched, goose version() is the version of record"). This is a **backend-only** migration (watcher never reads it) → no `_meta.schema_version`, no `WatcherMaxSchemaVersion` bump.

**Side-table shape** — mirror `00009`'s `character_assignment` table: `character_id INTEGER PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE` (00009_character_assignment.sql:22-23). D-02 columns:

```sql
-- +goose Up
CREATE TABLE character_portrait (
  character_id INTEGER PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE,  -- one portrait per char (side table, D-02)
  image_blob   BLOB NOT NULL,
  content_type TEXT NOT NULL,   -- sniffed server-side, NEVER the client claim (D-04)
  byte_size    INTEGER NOT NULL,
  updated_at   TEXT NOT NULL    -- cache-bust key for ?v= (D-07); text ISO to match character.last_seen / created_at style
);

-- +goose Down
-- Forward-only in practice (mirrors 00004-00015 no-op downs).
DROP TABLE character_portrait;
```

Note: `character(id)` PK is an `INTEGER PRIMARY KEY` (00001_init.sql:9). `updated_at TEXT` matches the `character.last_seen`/`created_at` TEXT convention (00001_init.sql:18,20); the assignment table used INTEGER epoch, but the roster payload's `last_seen`/`portrait_updated_at` are strings on the wire, so TEXT keeps the cache-bust value passthrough-clean. (Planner's call — either works; TEXT avoids a format conversion in the read path.)

---

### `internal/backendsrv/store/portrait.go` (store, blob CRUD + IDOR gate)

**Analog:** `store/assignment.go` — reuse `IsCharAssignedToTx` (assignment.go:119) and `charSharedTx` (assignment.go:97) verbatim as the D-05/D-06 gate; mirror the `isOfficerTx(ctx, tx, callerID)` in-tx re-check that every officer mutator opens with (assignment.go:265, WR-04 TOCTOU). The name→id lookup mirrors `bindCharacter` (binding.go:61-64).

**Character name→id lookup** (binding.go:63-64) — the serve + write paths receive `{name}` from the URL and resolve the id:

```go
err = tx.QueryRowContext(ctx,
    `SELECT owner_id, id FROM character WHERE name = ?`, charName).Scan(&ownerID, &charID)
// name column is UNIQUE COLLATE NOCASE → case-insensitive match, single indexed SELECT (V5).
```

**The assignee-OR-officer authorize-under-tx gate (D-05/D-06)** — compose the two existing in-tx checks (this is the exact `OfficerAssignTx` posture at assignment.go:264-278, but ORed rather than officer-only, and with the bank/bot flip from D-06):

```go
// D-05: assignee OR officer may write. D-06: a shared char (bank/bot) has no assignee → officer ONLY.
shared, err := charSharedTx(ctx, tx, charID)            // assignment.go:97
if err != nil { return err }
assigned := false
if !shared {
    assigned, err = IsCharAssignedToTx(ctx, tx, charID, callerID)  // assignment.go:119
    if err != nil { return err }
}
if !assigned {
    ok, err := isOfficerTx(ctx, tx, callerID)          // assignment.go:265 pattern (WR-04)
    if err != nil { return err }
    if !ok { return ErrNotAuthorized }                  // reuse the EXISTING store.ErrNotAuthorized (admins.go) — do NOT redefine
}
```

**Blob upsert / get / delete** — the `ON CONFLICT(character_id) DO UPDATE` upsert copies `OfficerAssignTx`'s upsert (assignment.go:279-286); the owner-scoped DELETE returning `(bool, err)` copies `ReleaseCharTx` (assignment.go:172-185):

```go
// Upsert (portrait is one row per char, PK = character_id):
_, err = tx.ExecContext(ctx,
    `INSERT INTO character_portrait (character_id, image_blob, content_type, byte_size, updated_at)
     VALUES (?, ?, ?, ?, ?)
     ON CONFLICT(character_id) DO UPDATE SET
       image_blob   = excluded.image_blob,
       content_type = excluded.content_type,
       byte_size    = excluded.byte_size,
       updated_at   = excluded.updated_at`,
    charID, blob, contentType, len(blob), now)

// Get (serve path — a []byte scan target + content_type; sql.ErrNoRows → not-found, not error-leak):
var blob []byte
var ct string
err := db.QueryRowContext(ctx,
    `SELECT image_blob, content_type FROM character_portrait WHERE character_id = ?`, charID,
).Scan(&blob, &ct)  // errors.Is(err, sql.ErrNoRows) → (nil, ErrPortraitNotFound), the charSharedTx:102-104 switch shape
```

**Typed sentinels** — declare `ErrPortraitNotFound` in the `var (...)` block the same way assignment.go:52-70 declares its sentinels; **reuse** `store.ErrCharNotFound` (charmeta.go, re-exported) and `store.ErrNotAuthorized` (admins.go) — do NOT redefine (assignment.go:48-51 explicitly notes this).

**Parameterized `?` only (V5), all timestamps from the passed `now`** (assignment.go:33-34 house rule).

---

### `internal/backendsrv/webadmin/portrait.go` (controller — base64 upload POST + DELETE)

**Analog:** `webadmin/charmeta.go:87-150` (`CharMetaSetHandler`) — the canonical decode → validate → `withTx` → audit JSON-POST shape D-03 mirrors. **No new multipart handler** (the codebase has zero; base64-in-JSON preserves the invariant).

**Request struct + decode** (charmeta.go:50-98):

```go
type portraitReq struct {
    ImageBase64 string `json:"image_base64"` // the browser-encoded PNG/JPEG/WebP bytes
    // NOTE: NO content_type field is trusted from the client (D-04) — the server sniffs it.
}
// ...
var req portraitReq
if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ImageBase64 == "" {
    writeJSONError(w, http.StatusBadRequest, "invalid_input")   // charmeta.go:95-98 shape
    return
}
```

**Base64 decode → magic-byte sniff → size cap (NEW — D-04 security core, see "No Analog Found")** — this replaces charmeta.go's `validCharMeta` value-set check with the image validation:
- `base64.StdEncoding.DecodeString(req.ImageBase64)` → on error `400 invalid_input`.
- **Sniff the decoded bytes** (PNG `89 50 4E 47`, JPEG `FF D8 FF`, WebP `RIFF….WEBP`); reject anything else (incl. SVG) → `400` (a distinct code so the web maps the wrong-type copy). Set `content_type` from the sniff, NOT the client.
- Enforce `len(decoded) <= 256*1024` (D-04 default) → `400` too-large.
- The stdlib `http.DetectContentType` (or `image.DecodeConfig`) is available but returns broad types; a fixed 3-way magic-byte switch is the precise, SVG-excluding choice the threat model wants.

**Identity → audit → one tx** (charmeta.go:106-130) — copy verbatim:

```go
writer := caller(ctx)   // officers.go:58 — records identity; the AUTHORIZATION is in the store gate, not here
now := nowUnix()        // officers.go:50
err := withTx(ctx, db, func(tx *sql.Tx) error {   // audit.go:88 — BEGIN IMMEDIATE
    if e := store.SetPortraitTx(ctx, tx, charName, decoded, sniffedType, writer, now); e != nil {
        return e   // the store gate returns ErrNotAuthorized / ErrCharNotFound
    }
    return AppendAuditTx(ctx, tx, "portrait_set", writer, map[string]any{
        "character": charName,
    }, now)   // audit.go:57
})
```

**Error mapping** (charmeta.go:121-130) — `errors.Is(err, store.ErrCharNotFound)` → 400; `errors.Is(err, store.ErrNotAuthorized)` → **403 not_authorized** (the RequireOfficer error string, session.go:231 — so the web AuthGate routes it); else 500 `internal`. slog carries op+err ONLY, never the char name (characters.go:19 / inventory.go V7).

**The DELETE handler** (D-08) is the same shell minus the body decode: resolve `{name}`, `withTx` → `store.DeletePortraitTx` (same assignee-or-officer gate) → audit `"portrait_removed"`. Response: `writeJSON(w, ...)` (officers.go:44).

**The `{name}` path segment** comes from `r.PathValue("name")` (Go 1.22+ ServeMux, inventory.go:57) — bound only as a `?` placeholder downstream (V5), validated implicitly by the name-lookup returning `ErrCharNotFound`.

---

### `internal/backendsrv/readapi/portrait.go` (controller — serve raw blob bytes)

**Analog (structure only):** `readapi/inventory.go` — the `{char}` wildcard read, `RequireSession`-gated, name-by-value, GET-only (405 otherwise), V7 logging. **This handler is the LEAST-precedented file in the phase** — it writes raw image bytes, NOT `json.NewEncoder`. See "No Analog Found."

Copy from inventory.go:50-64:
- GET-only guard (inventory.go:51-54).
- `char := r.PathValue("char")` (inventory.go:57) — here `{name}`.
- Handler holds `*store.Store` (or `*sql.DB`), constructed once (`NewInventory`, inventory.go:41).

Then DIVERGE (the new part):
```go
blob, ct, err := h.store.GetPortrait(ctx, name)   // store/portrait.go
if errors.Is(err, store.ErrPortraitNotFound) {
    http.Error(w, "not found", http.StatusNotFound)   // 404-vs-401 discipline (D-07); RequireSession already handled 401
    return
}
// D-04 serve hardening:
w.Header().Set("Content-Type", ct)                     // image/<sniffed>, from the stored value
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("Cache-Control", "private, max-age=…")  // the ?v= updated_at busts it; safe to cache
w.WriteHeader(http.StatusOK)
_, _ = w.Write(blob)                                   // raw bytes — the ONLY non-JSON write path in the API
```

**Registration analog:** `mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))` (main.go:378).

---

### `internal/backendsrv/readapi/characters.go` (extend — `rosterChar` + additive fields)

**Analog:** the file itself; extend `rosterChar` (characters.go:34-43) with two additive fields (the extend-only schema house rule):

```go
type rosterChar struct {
    // ...existing fields...
    LastSeen          string `json:"last_seen"`
    HasPortrait       bool   `json:"has_portrait"`         // D-07 — the flag, never the bytes
    PortraitUpdatedAt string `json:"portrait_updated_at"`  // D-07 — cache-bust key for ?v=
}
```

Populate in the `for _, c := range rows` copy loop (characters.go:83-94). The roster read `store.RosterFor` gains a `LEFT JOIN character_portrait` (a cheap additive read — the flag rides the existing query; the blob never does). **D-07 discretion:** the planner chooses roster vs `CharacterInventory` (compute/types.go:186-195) vs both. Roster is the natural home (the Characters tab reads it to render the frame); `CharacterInventory` is the alternative if the frame renders off the inventory fetch. Whichever, the field is additive JSON, contract-safe.

⚠️ **P39 LEFT JOIN caution (MEMORY):** the P38/P39 catalog LEFT JOIN fanned out duplicate rows for same-name entries and crashed the web (`each_key_duplicate`), fixed with `GROUP BY norm + MAX(flag)`. Here the join is on `character_id` (a PK on BOTH sides — `character.id` and `character_portrait.character_id`), so it is 1:1 and CANNOT fan out. No aggregation needed — but the planner should note this is why the join is safe (unlike the name-keyed catalog joins).

---

### `cmd/squirebot-server/main.go` (register 3 routes)

**Analog:** the char-meta login-only block (main.go:334-341) for the write routes' gate choice, and the wishlist block (main.go:352-364) for the "login-only but IDOR-checked in-tx" posture — D-05's assignee-or-officer gate lives IN the store tx, so the ROUTE is `RequireSession` (login-only), NOT `RequireOfficer` (an officer isn't the only writer — an assignee is too):

```go
// Character portrait (Phase 41 / CHARUI-02) — LOGIN-ONLY at the route (RequireSession);
// the assignee-OR-officer gate (D-05/D-06) is enforced IN the store tx (IsCharAssignedToTx
// OR isOfficerTx), the wishlist/assignment IDOR precedent. NEVER RequireOfficer at the route
// (a regular assignee is a legitimate writer). The serve GET is a guild-wide member read.
mux.Handle("GET /api/v1/characters/{name}/portrait", webauth.RequireSession(db, readapi.NewPortrait(st)))
mux.Handle("POST /api/v1/characters/{name}/portrait", webauth.RequireSession(db, webadmin.PortraitSetHandler(db)))
mux.Handle("DELETE /api/v1/characters/{name}/portrait", webauth.RequireSession(db, webadmin.PortraitDeleteHandler(db)))
```

The Go 1.22+ ServeMux `{name}` wildcard + method-prefixed patterns are already the house idiom (main.go:378, 386). No CORS change — the outer credential-aware CORS wrap (main.go:319-320) covers them.

---

### `web/src/lib/components/InventoryWindow.svelte` (CHARUI-01 compaction + portrait render)

**Analog:** itself (the `.doll` block at `:159-162` + `:338-372` CSS + `:479-483` breakpoint) and `PaperdollSlot.svelte:91-124` (the img + `onImgError` fallback).

**CHARUI-01 compaction — the exact edits** (values from 41-UI-SPEC § "concrete target values"):
- `.paperdoll { gap: 24px }` → `gap: 16px` (InventoryWindow.svelte:342).
- `.doll { min-height: 260px }` → **REMOVE** the `min-height` line (InventoryWindow.svelte:356); size the frame by `aspect-ratio: 1 / 1` + `width: min(190px, 100%)` instead (190px = 3 tiles + 2 gaps).
- `@media (max-width: 640px) .paperdoll { gap: 16px }` → `gap: 8px` (InventoryWindow.svelte:479-482).
- Keep the 3-column `grid-template-columns: auto 1fr auto` (InventoryWindow.svelte:341) and the 62px tile module — compaction TIGHTENS, does not restructure.
- Fallback (empty) frame keeps `1px dashed var(--border, var(--accent))` at `opacity: 0.8` (InventoryWindow.svelte:357-359 — unchanged); the FILLED state switches to `1px solid` (dashed→solid on the same token, no new color).

**Portrait `<img>` + fallback** — mirror `PaperdollSlot.svelte:119-124` and its `onImgError` (`PaperdollSlot.svelte:91-94`):

```svelte
<!-- in the reclaimed .doll frame -->
{#if inventory.has_portrait}
  <img
    src={`${API_BASE}/api/v1/characters/${encodeURIComponent(inventory.char)}/portrait?v=${inventory.portrait_updated_at}`}
    alt={inventory.char}
    class="portrait-img"
    onerror={onImgError}
  />
{/if}
<!-- the existing silhouette + name stay underneath as the fallback (paints through on img error / no portrait) -->
<span class="silhouette" aria-hidden="true">⚔</span>
<p class="doll-line">{inventory.char}</p>
```

```js
function onImgError(e) {
    // Hide the <img> so the silhouette under-layer shows (PaperdollSlot.svelte:91-94).
    (e.currentTarget).style.display = 'none';
}
```

The char name renders via plain `{}` (Svelte auto-escapes) — NEVER `{@html}` (CharMetaForm.svelte:19, T-16-03). `?v={portrait_updated_at}` is the cache-bust (D-07). The silhouette stays `aria-hidden` (the name is already in the char-head `<h1>`).

---

### Portrait upload/remove control (new component OR inline)

**Analog:** `web/src/lib/components/CharMetaForm.svelte` (the whole file) — the state machine, `.primary` accent button, `.result.success`/`.error` aria-live messages, the `LoaderCircle` "…" spinner, 44px targets, `:focus-visible`, `prefers-reduced-motion`, and the `authGuard` 401/403 hand-off are all here to copy.

Copy directly:
- **Saving state + spinner** (CharMetaForm.svelte:203-211): `{#if saving}<LoaderCircle .../> <span>Uploading…</span>{:else}...` — swap "Saving…" → "Uploading…" (UI-SPEC copy).
- **Result messages** (CharMetaForm.svelte:196-202, 254-265): `.result.success` = `var(--status-ok)`, `.result.error` = `var(--status-missing)`, both `aria-live="polite"`.
- **Primary button** (CharMetaForm.svelte:266-291): accent fill, `--bg` text, uppercase Label type, `min-height: 44px`, `:focus-visible { outline: 2px solid var(--accent) }`.
- **AuthGuard hand-off** (CharMetaForm.svelte:28,46,79-82,114-117): `getContext<AuthGuard>(AUTH_GUARD_KEY)`; on `Unauthenticated` → `authGuard(err)`. A 403 `not_authorized` also routes through it (UI-SPEC state 8).
- **`prefers-reduced-motion`** spinner disable (CharMetaForm.svelte:300-304).

**NEW pieces (no precedent — call out):**
- The hidden `<input type="file" accept="image/png,image/jpeg,image/webp">` + a real `<button>`/`<label>` trigger (UI-SPEC state 2). `accept` excludes SVG/GIF. No file-input exists anywhere in `web/src/lib/components/` — this is new.
- Client `FileReader` → base64 + optional canvas downscale to ~256–512px square (UI-SPEC states 3-4). No canvas/image-processing precedent in the web tree.
- The confirm-before-DELETE step (UI-SPEC "Remove photo:" copy). The app's destructive-confirm idiom is `ConfirmDialog` (referenced in CharMetaForm.svelte:5 as "Non-destructive ⇒ NO ConfirmDialog") — the planner should locate `ConfirmDialog.svelte` for the remove confirmation.

**Visibility gate (client UX; server re-checks)** — show the controls when `RosterCharacter.is_mine === true` OR `session.isOfficer === true`; for a bank/bot (`is_bank_toon`/`is_guild_bot`), only `isOfficer` (D-06). Hidden entirely otherwise (no disabled ghost). This mirrors how the app hides officer-only affordances (main.go officer-gate philosophy; UI-SPEC § "Editor controls").

---

### `web/src/lib/api.ts` (types + wrappers)

**Analog:** `saveCharMeta` (api.ts:616-621) over `postJSON` (api.ts:416) for the upload; `getJSON` (api.ts:272) + `API_BASE` (api.ts:21) for the serve URL construction.

**Type additions** (api.ts:160-171 `RosterCharacter` and/or :209-215 `CharacterInventory` — matching the Go `rosterChar` field placement decided by D-07):

```ts
export interface RosterCharacter {
    // ...existing...
    last_seen: string;
    has_portrait: boolean;          // D-07
    portrait_updated_at: string;    // D-07 cache-bust key
}
```

**Upload wrapper** (copy `saveCharMeta`, api.ts:616-621):

```ts
export function setPortrait(
    name: string,
    body: { image_base64: string },
    fetchFn: typeof fetch = fetch
): Promise<PortraitResult> {
    return postJSON<PortraitResult>(`/api/v1/characters/${encodeURIComponent(name)}/portrait`, body, fetchFn);
}
```

**Remove wrapper** — `postJSON` (api.ts:416) is POST-only; the DELETE route needs either a small `deleteJSON` helper (a 3-line copy of `postJSON` with `method: 'DELETE'` — see api.ts:419) OR the planner registers the remove as a `POST …/portrait/remove` to reuse `postJSON` unchanged (the wishlist `/wishlist/remove` POST precedent, main.go:363). Both are valid; the POST-remove path reuses `postJSON` with zero new plumbing.

**The serve URL is NOT a wrapper** — it's an `<img src>` built inline as `` `${API_BASE}/api/v1/characters/${encodeURIComponent(name)}/portrait?v=${portrait_updated_at}` `` (the browser fetches it credentialed via the cookie; `credentials: 'include'` is automatic for same-registrable-domain img requests to `api.squirebot.quest`). `API_BASE` is exported (api.ts:21).

---

## Shared Patterns

### Authorization (assignee-OR-officer, in-tx — D-05/D-06)
**Source:** `store/assignment.go` — `IsCharAssignedToTx:119`, `charSharedTx:97`, `isOfficerTx` (via `OfficerAssignTx:265`), `store.ErrNotAuthorized` (admins.go).
**Apply to:** every portrait WRITE (POST + DELETE) in `store/portrait.go`, authorized UNDER the tx (WR-04 TOCTOU), NOT at the route. The route is `RequireSession` (login-only); the gate is in the store.
The 4-line ORed gate excerpt is in the `store/portrait.go` assignment above.

### Atomic write + audit
**Source:** `webadmin/audit.go:88` (`withTx`, BEGIN IMMEDIATE) + `:57` (`AppendAuditTx`).
**Apply to:** both portrait mutators — the blob write + its `portrait_set`/`portrait_removed` audit row compose in ONE tx (charmeta.go:113-120 shape).

### Read-API session gate + name-by-value
**Source:** `readapi/inventory.go:50-64` (`RequireSession` reg, `r.PathValue`, GET-only 405, V7 logging) + `webauth.RequireSession` (session.go:194).
**Apply to:** the portrait serve GET (and the roster field read is already under this gate).

### Web write core + AuthGate hand-off
**Source:** `api.ts:416` (`postJSON`, `credentials: 'include'`, typed `Unauthenticated`/`Forbidden`) + `CharMetaForm.svelte:79-82,114-117` (`authGuard(err)` on 401/403).
**Apply to:** the upload/remove control — a mid-session 401→LoginScreen, a 403 `not_authorized`→AuthGate, both routed through the `AUTH_GUARD_KEY` context.

### Theme-token discipline (no literal hex/font)
**Source:** `CharMetaForm.svelte` + `InventoryWindow.svelte` + `PaperdollSlot.svelte` — every color/font is `var(--…)` resolved off `[data-theme]`.
**Apply to:** all new CSS (frame border, focus ring, `--status-ok`/`--status-missing` messages, `--accent` primary). 41-UI-SPEC § Color pins every token; add NO new token, NO literal hex.

---

## No Analog Found

Files/pieces with no close match in the codebase (planner should treat these as genuinely new; lean on `41-UI-SPEC.md` + `41-CONTEXT.md` security posture, not a copy-target):

| File / piece | Role | Data Flow | Reason |
|--------------|------|-----------|--------|
| `readapi/portrait.go` byte-streaming serve (`w.Write(blob)` + `Content-Type: image/*` + `X-Content-Type-Options: nosniff`) | controller | file-I/O / byte streaming | **The API has ZERO raw-byte response handlers** — every existing handler does `json.NewEncoder(w).Encode(...)`. The only `w.Write([]byte(...))` calls in the tree are inside `*_test.go` mock servers (oauth_test.go, wiki_test.go). inventory.go supplies the request-side scaffold (gate + `{char}` + 405 + V7 logging) but NOT the response side. New: the header set + raw write. |
| Base64 decode + 3-way magic-byte sniff + 256 KB cap (in `webadmin/portrait.go`) | controller (validation) | transform | **No image/upload validation exists** — the codebase's only validators are value-set checks (`validCharMeta`, charmeta.go:162) and modernc SQLite result-code sniffs (`RequestTx`, assignment.go:229-231). No `encoding/base64`, no `http.DetectContentType`, no `image.DecodeConfig` in any handler. New: the whole decode→sniff→cap pipeline. Follow the CONTEXT § "Security posture" byte signatures exactly (PNG `89 50 4E 47`, JPEG `FF D8 FF`, WebP `RIFF….WEBP`; SVG rejected). |
| Web file `<input>` + `FileReader` base64 + canvas downscale (upload control) | component | file-I/O / transform | **No `<input type="file">`, no `FileReader`, no `<canvas>` anywhere in `web/src/`** — every existing form is `<select>`/`<input type="text">` (CharMetaForm). CharMetaForm supplies the button/state/message/a11y shell to copy; the file-pick + read + downscale is new browser code (UI-SPEC states 2-4). |

---

## Metadata

**Analog search scope:** `internal/backendsrv/{migrations,store,webadmin,readapi,webauth,compute}`, `cmd/squirebot-server`, `web/src/lib/{components,api.ts}`.
**Files scanned:** ~18 (6 migrations, assignment.go, charmeta.go×2, characters.go, inventory.go, session.go, audit.go, officers.go, admins.go, binding.go, main.go, InventoryWindow.svelte, PaperdollSlot.svelte, CharMetaForm.svelte, api.ts, types.go).
**Pattern extraction date:** 2026-07-15
