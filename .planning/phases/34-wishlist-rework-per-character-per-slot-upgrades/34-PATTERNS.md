# Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades - Pattern Map

**Mapped:** 2026-06-18
**Files analyzed:** 19 (5 CREATE backend, 4 MODIFY backend, 1 REWRITE web, 2 CREATE web, 1 MODIFY web, 2 DELETE web, + 5 new test files)
**Analogs found:** 19 / 19 (every new/modified/deleted file has a real in-repo analog)

> **Read this first — the two highest-risk spots in the phase:**
>
> 1. **The slot-vocabulary bridge (canonical "Finger1" → wiki "Fingers").** THREE vocabularies coexist
>    (canonical Title-case, wiki-prose, UPPERCASE inv token). Get the inverse map wrong and EVERY
>    suggestion list is silently empty. Analog + unit-test target below in §`compute/wishlist.go`. This is
>    Pitfall 2 of the RESEARCH and is flagged on every file that touches a slot.
> 2. **The `alert_log` FK rebuild.** `alert_log.wantlist_item_id REFERENCES wantlist_item(id)` (00007:50).
>    D-01 drops `wantlist_item`, so `alert_log` MUST be DROP+CREATE'd to FK `wishlist_item(id)` (SQLite
>    can't ALTER a FK). The analog is the 00007 `alert_log` rebuild itself (it already did this once). This
>    is the only place the clean break has teeth.

---

## File Classification

| New/Modified/Deleted File | Status | Role | Data Flow | Closest Analog | Match Quality |
|---------------------------|--------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00014_wishlist.sql` | CREATE | migration | transform/DDL | `00006_wantlist.sql` (table shape) + `00007_notify.sql` (alert_log rebuild) + `00010` (dedup idx) | exact (lineage) |
| `internal/backendsrv/store/wishlist.go` | CREATE | store | CRUD | `store/wantlist.go` | exact |
| `internal/backendsrv/wantmatch/match.go` | MODIFY | matcher seam | request-response (read) | itself (current body) + caller `ec/ec.go:211` | self |
| `internal/backendsrv/store/alertlog.go` | MODIFY (or no-op) | store | CRUD | itself (column FK target only) | self |
| `internal/backendsrv/compute/wishlist.go` | CREATE | compute | transform (compute-on-read) | `compute/inventory.go` (`StructuredInventory`) + `store.GearTierPrices` + `enrich.WIKI_SLOT_TO_INV_SLOTS` | role-match |
| `internal/backendsrv/compute/types.go` | MODIFY (append) | types | — | itself (`InventorySlot`/`CharacterInventory` structs) | self |
| `internal/backendsrv/readapi/wishlist.go` | CREATE | route | request-response (read) | `readapi/inventory.go` | exact |
| `internal/backendsrv/webadmin/wishlist.go` | CREATE | controller/write API | CRUD (owner-scoped) | `webadmin/wantlist.go` | exact (line-for-line) |
| `cmd/squirebot-server/main.go` | MODIFY | route registration | — | itself (lines 339-348, 390) | self |
| `web/src/routes/wishlist/+page.svelte` | REWRITE | component (master-detail) | request-response + CRUD | `web/src/routes/inventory/+page.svelte` + `characters/+page.svelte` | role-match |
| `web/src/lib/wishlist/*.ts` | CREATE | utility (pure helper) | transform | `web/src/lib/roster.ts` + `items.ts` + `wantlist/groupByChar.ts` | exact |
| `web/src/lib/api.ts` | MODIFY (append) | utility (fetch wrappers) | request-response | itself (lines 744-838, the wantlist wrappers) | self |
| `web/src/routes/wantlist/+page.ts` | KEEP (verify) | redirect | — | itself (already a 308 stub) | self (done) |
| `web/src/lib/components/WantlistPanel.svelte` | DELETE (after rewire) | component | — | (mine for reused logic, then delete) | — |
| `web/src/lib/wantlist/*.ts` | DELETE (after mining) | utility | — | (mine pure helpers, then delete) | — |

**Reused-as-is (NO new file, NO edit — mounted/called verbatim):** `web/src/lib/components/ExaminePanel.svelte`
(WISH-06), `StateBlock.svelte`, `LastSyncedCell.svelte`, `ConfirmDialog.svelte`, `Toggle.svelte`, the
`WantAddForm.svelte` add idiom (clone its debounce, NOT the file), `compute.StructuredInventory`,
`store.GearTierPrices`, `store.RosterFor`, the `notify`/`alert_log` spine, the EC job.

---

## Pattern Assignments

### `internal/backendsrv/migrations/00014_wishlist.sql` (CREATE — migration, DDL/transform)

**Analogs:** `00006_wantlist.sql` (table shape + the split partial-unique-index idiom),
`00007_notify.sql` (the `alert_log` DROP+CREATE rebuild — the load-bearing FK pattern), `00010` (the
`COALESCE`-keyed dedup index, if account-level dedup is needed — not here since `character_id` is NOT NULL).

**Table-shape pattern** — copy the security-bearing columns + the two partial unique indexes from
`00006_wantlist.sql:20-34`:
```sql
CREATE TABLE wantlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  item_id         INTEGER,                       -- NULL ⇒ custom want (D-04)
  item_name       TEXT NOT NULL,                 -- snapshot: catalog name OR custom label
  ...
  active          INTEGER NOT NULL DEFAULT 1,    -- soft-delete (Pitfall 4)
  created_at      INTEGER NOT NULL               -- unix epoch secs (nowUnix())
);
CREATE INDEX wantlist_user_idx    ON wantlist_item(discord_user_id);
CREATE INDEX wantlist_item_id_idx ON wantlist_item(item_id);
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(discord_user_id, item_id, reason)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx  ON wantlist_item(discord_user_id, item_name, reason) WHERE item_id IS NULL     AND active = 1;
```
Deltas for `wishlist_item` (per RESEARCH Pattern 1): `character_id INTEGER NOT NULL REFERENCES character(id)`
and `slot TEXT NOT NULL` are added to the dedup-index key; `reason` is dropped (no buy/quest concept);
`muted` becomes `pinged INTEGER NOT NULL DEFAULT 1` (Pitfall 8 — inverse polarity, default-ON).

**`alert_log` FK-rebuild pattern (HIGHEST RISK)** — copy the DROP+CREATE verbatim from `00007_notify.sql:46-59`,
changing only the FK target table:
```sql
-- 00007_notify.sql:46-59 — the exact pattern to mirror (it rebuilt alert_log once already):
DROP INDEX IF EXISTS alert_log_dedup_idx;
DROP TABLE alert_log;
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wantlist_item_id INTEGER REFERENCES wantlist_item(id) ON DELETE CASCADE,  -- ◄── change target to wishlist_item(id)
  discord_user_id  TEXT NOT NULL,
  source           TEXT NOT NULL,
  item_id          INTEGER,
  detail           TEXT,
  sent_at          INTEGER NOT NULL,
  send_status      TEXT NOT NULL,
  read_at          INTEGER                    -- nullable; NULL = unread (D-05)
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wantlist_item_id, source, item_id, sent_at);
```
**Decision (Pitfall 6):** RESEARCH recommends KEEPING the column name `wantlist_item_id` (only the FK target
changes to `wishlist_item(id)`), so `store/alertlog.go` stays byte-identical. Order the migration:
rebuild `alert_log` FIRST, then `DROP TABLE wantlist_item` (correct under either `PRAGMA foreign_keys`
setting — Open Question 3). Then `DROP INDEX … ; DROP TABLE wantlist_item;` per RESEARCH lines 222-226.
**Down migration is an explicit no-op** (`SELECT 1;`) — mirrors `00006:59-61` / `00007:63-65` / `00010:36-38`.

**Test analog:** extend `migrations/migrate_test.go` mirroring `TestMigrate_00007_AddsNotify` (line 414) and
`TestMigrate_00006_AddsWantlist` (line 330): assert `wishlist_item` exists with its columns
(`columnSet`/`indexExists` helpers, line 339-348), assert `wantlist_item` is GONE, assert a NULL-FK
`alert_log` insert still succeeds (the test-alert path, line 444-446), and assert idempotent re-run
(`TestRunMigrations_Idempotent`, line 61).

---

### `internal/backendsrv/store/wishlist.go` (CREATE — store, owner-scoped CRUD)

**Analog:** `store/wantlist.go` (clone its shape function-for-function).

**Imports + the typed-duplicate-error pattern** (`wantlist.go:23-46`) — copy the NAMED `modernc.org/sqlite`
import and the `sqliteConstraintUnique = 2067` extended-result-code idiom verbatim (rename `ErrDuplicateWant`
→ `ErrDuplicateWishlist`):
```go
sqlite "modernc.org/sqlite"
...
var ErrDuplicateWant = errors.New("wantlist: duplicate")
const sqliteConstraintUnique = 2067   // SQLITE_CONSTRAINT_UNIQUE — NOT a string-match of the driver message
```

**Add pattern** (`wantlist.go:81-101` `AddWantTx`) — the `errors.As(&sqliteErr)` → `ErrDuplicateWishlist`
detection is load-bearing; copy it. The new signature drops `reason`/`note`/`priority`, adds `slot string`
(and `character_id` stays, now NOT NULL).

**Ping-toggle pattern** (`wantlist.go:265-285` `SetMutedTx` + `boolToInt`) — the RESEARCH names this the
`SetPingedTx` twin; copy the owner-scoped IDOR guard EXACTLY, inverting only the column:
```go
// store/wantlist.go:265 SetMutedTx — the IDOR-safe owner-scoped UPDATE to mirror as SetPingedTx:
func SetMutedTx(ctx context.Context, tx *sql.Tx, wantID int64, discordID string, muted bool) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE wantlist_item SET muted = ? WHERE id = ? AND discord_user_id = ? AND active = 1`,
		boolToInt(muted), wantID, discordID)
	...
	return n > 0, nil   // cross-owner → RowsAffected=0 → (false,nil): silent no-op, never leaks existence
}
```
→ becomes `UPDATE wishlist_item SET pinged = ? WHERE id = ? AND discord_user_id = ? AND active = 1`.

**Remove pattern** (`wantlist.go:241-253` `RemoveOwnWantTx`) — soft-delete `active = 0`, same owner-scoped
`WHERE id=? AND discord_user_id=? AND active=1` guard → `RemoveOwnWishlistTx`.

**Read pattern** (`wantlist.go:108-157` `ListOwnWants`) — copy the non-nil-slice + `sql.Null*`→pointer
scan; scope to char+slot. RESEARCH names this `ListOwnWishlist(ctx, db, discordID, char)`. Drop `note`,
add `slot`, keep `pinged`. NOTE: the auto-removal (D-02) is NOT done in this store read — it is the
compute layer's job (held-name join). This store read returns the raw active targets.

**Badge-set read (NEW, small)** — RESEARCH Pattern 6 wants `AlertedWishlistIDs(ctx, db, discordID) →
map[int64]bool`. Analog: `store/alertlog.go:130-138` `UnreadCount` (owner-scoped `SELECT … WHERE
discord_user_id = ?`), generalized to `SELECT DISTINCT wishlist_item_id FROM alert_log WHERE
discord_user_id = ? AND wishlist_item_id IS NOT NULL`.

**Test analog:** clone `store/wantlist_test.go` → `store/wishlist_test.go` (owner-scoped add/remove/ping IDOR
silent-no-op + the 2067 dup detection).

---

### `internal/backendsrv/wantmatch/match.go` (MODIFY — matcher seam, the load-bearing repoint)

**Analog:** itself (current body) + its ONE live caller `ec/ec.go:211`.

**The repoint** (the BEFORE is `match.go:64-93`, both `ForItem` and `ForName`):
```go
// match.go:66-69 ForItem — BEFORE:
`SELECT w.id, w.discord_user_id, w.item_id, w.item_name, w.note, c.name AS character_name
   FROM wantlist_item w
   LEFT JOIN character c ON c.id = w.character_id
  WHERE w.item_id = ? AND w.active = 1 AND w.muted = 0`
```
Deltas the planner MUST apply (RESEARCH Pattern 2):
1. `FROM wantlist_item` → `FROM wishlist_item` (both `ForItem` line 67 and `ForName` line 85).
2. `w.muted = 0` → `w.pinged = 1` (inverse polarity — Pitfall 8).
3. `LEFT JOIN character` → `JOIN character` (`character_id` is NOT NULL now) — line 68 + 86.
4. Drop `w.note` from the SELECT (line 66/84) AND drop `Note *string` from `Hit` (line 49-51) + the
   `note` scan in `scanHits` (line 102-118). **Verified safe:** the only `Hit.Note` reader is
   `ec/embed.go:97 whyWanted(hit)`, and `embed.go:98` already guards `if hit.Note != nil` — so a
   removed/nil `Note` degrades to an empty "why" line with NO embed change. (Alternatively keep a nullable
   `note` column for embed parity; RESEARCH recommends drop — simpler.)
5. `CharacterName` stays DISPLAY-ONLY; `DiscordUserID` is STILL scanned from `w.discord_user_id` (the want
   owner) — the T-28-06 invariant at `match.go:39-45` + the `scanHits` comment at line 106-108. **Port the
   `TestForItem_DMTargetIsWantOwner_NotCharacterOwner` regression** from `match_test.go`.
6. `ForName` keeps `= ? COLLATE NOCASE` exact-match (line 87) — NOT a LIKE substring (Pitfall 6).

**Caller (no change needed):** `ec/ec.go:211` calls `wantmatch.ForItem(ctx, db, item.ItemID)` — the
signature is unchanged, so the caller compiles as-is. Only the SQL inside `match.go` changes.

**Test analog:** `wantmatch/match_test.go` currently seeds `wantlist_item` rows — re-point the seed
INSERTs to `wishlist_item` (with `pinged`/`slot`/NOT-NULL `character_id`); keep the DM-target-is-owner
assertion.

---

### `internal/backendsrv/store/alertlog.go` (MODIFY-or-no-op — store, CRUD)

**Analog:** itself. Per RESEARCH Pitfall 6 **option (B) — recommended:** KEEP the column name
`wantlist_item_id`; only the migration's FK *target* table changes. Then `alertlog.go` is BYTE-IDENTICAL —
`InsertAlertTx` (line 147-160), `RecentAlertExists` (line 180-197), the dedup index all stay as-is, and
`notify.Alert.WantID` is untouched. Option (A) renames the column to `wishlist_item_id` everywhere
(alertlog.go + dedup index + migration) for clarity — higher churn. **Pick (B); this file likely needs NO
edit.**

---

### `internal/backendsrv/compute/wishlist.go` (CREATE — compute, compute-on-read transform)

**Analogs:** `compute/inventory.go` (`StructuredInventory`, the public-fn→pure-helper split),
`store.GearTierPrices` (the suggestion source), `enrich.WIKI_SLOT_TO_INV_SLOTS` (the slot bridge),
`compute/slotconst.go` (the canonical worn-slot vocabulary).

**Equipped-item-per-slot pattern** (`compute/inventory.go:118-124`): call
`StructuredInventory(ctx, s, char)` and index `inv.Equipment` by `InventorySlot.CanonicalSlot`
(`types.go:158-175` — `CanonicalSlot` is `"Head"/"Finger1"/…`, `Item` is `""` for an empty slot). The
21-slot iteration list is the InventoryWindow constants (see web §). `Item == ""` ⇒ render "Empty" (D-04).

**Auto-removal pattern (D-02, normalized name)** — RESEARCH Pattern 4. The norm key is
`strings.ToLower(strings.TrimSpace(name))` — the SAME `lower(trim(name))` convention `GearTierPrices`
uses in its `pp_rep` CTE (`readviews.go:466`). Build a held-set from `inv.Equipment + inv.General +
inv.Bank` (+ `.Children`); a target is HIDDEN (not deleted) when its normalized name is in the held set.

**Suggestion-mapping pattern (WISH-04) + THE SLOT-VOCABULARY BRIDGE (HIGHEST RISK)** —
`store.GearTierPrices` (`readviews.go:464-514`) returns every `GearTierPriceRow{Tier, Class, Slot,
ItemName, Rank, …, HasPrice, LastListed}` (`readviews.go:118-129`) with name-keyed price already resolved.
Filter by `CharMeta.Class` (`store/charmeta.go`) AND `row.Slot == wikiSlot(canonicalSlot)`.
- **The "Raid" tag is the TIER, not a column** (Pitfall 3): tag iff `row.Tier == "Velious Raiding"`. The
  exact literals live in `enrich/wikigear.go:25-26` (`TierVeliousPreRaid = "Velious Pre-Raid/Group"`,
  `TierVeliousRaiding = "Velious Raiding"`). There is NO `no_drop` column — do not invent one.
- **The bridge:** the new `wishlist_item.slot` is canonical Title-case (`"Finger1"`, from `slotconst.go:36-42`);
  `GearTierPriceRow.Slot` is the wiki PROSE slot (`"Fingers"`). The map to invert is
  `enrich/eqconst.go:65-83` `WIKI_SLOT_TO_INV_SLOTS` (wiki-prose → UPPERCASE inv tokens):
```go
// enrich/eqconst.go:65-83 — invert THIS (and case-fold) to get canonical→wiki-prose:
var WIKI_SLOT_TO_INV_SLOTS = map[string][]string{
	"Ears":    {"EAR1", "EAR2"},      // ◄── "Ear1" AND "Ear2" both map back to wiki "Ears"
	"Fingers": {"FINGER1", "FINGER2"},// ◄── "Finger1" AND "Finger2" → wiki "Fingers"
	"Wrists":  {"WRIST1", "WRIST2"},
	"Head":    {"HEAD"}, "Primary": {"PRIMARY"}, ...   // singletons 1:1
}
```
  Build the inverse: UPPERCASE inv token → wiki-prose slot; then upper-case the canonical worn-slot
  (`"Finger1"` → `"FINGER1"`) to look it up. **Do NOT re-key `gearcheck.go`** — it walks the OTHER direction
  (wiki→inv); a small new pure helper here is cleaner. **`slotconst.go:5-14`'s own comment warns** these two
  slot maps are deliberately distinct and conflating them "would silently leave every equipment row
  unclassified." `Ammo`/`Charm`/`Power` have NO `WIKI_SLOT_TO_INV_SLOTS` key → empty suggestion list (correct, A5).

**Pure-helper split for testability** — mirror `inventory.go`'s `StructuredInventory` (store access) vs
`buildStructuredInventory` (pure) split (`inventory.go:113-145`): keep the slot-bridge + suggestion-filter +
auto-removal as pure functions so `compute/wishlist_test.go` can table-test them without a DB.

**Test analog:** new `compute/wishlist_test.go` — assert `"Finger1"`&`"Finger2"` BOTH → `"Fingers"`,
`"Ear1"/"Ear2"` → `"Ears"`, `"Head"` → `"Head"`; the Raid-tag-is-tier filter; the auto-removal name join.

---

### `internal/backendsrv/compute/types.go` (MODIFY append-only — types)

**Analog:** itself. Append `WishlistView`/`WishlistSlot`/`WishlistTarget`/`WishlistSuggestion` structs next
to `InventorySlot` (`types.go:158-175`) and `CharacterInventory` (`types.go:180-189`). **Append-only —
NEVER rename an existing snake_case JSON tag** (the schema-evolution rule). Mirror the pointer-for-nullable
+ `json:"snake_case"` convention (e.g. `Price *float64 \`json:"price"\``, `LastListed string
\`json:"last_listed"\``).

---

### `internal/backendsrv/readapi/wishlist.go` (CREATE — route, request-response read)

**Analog:** `readapi/inventory.go` (line-for-line the `{char}` path-wildcard handler shape).

**Handler pattern** (`inventory.go:31-92`): a struct holding `*store.Store`, a `NewWishlist(s)`
constructor, GET-only `ServeHTTP`, `char := r.PathValue("char")` (Go 1.22+ wildcard — the `?`-bind-only
seam, V5/T-31-06 at line 57), nil→[] slice coercion (line 71-79), JSON encode, and V7 slog (op + count +
status only, NEVER the char value — line 19-20, 88-91). The empty-not-404 contract (line 13-15) carries:
an unknown char returns an empty `WishlistView` (200), not a 404.
```go
// readapi/inventory.go:50-59 — the handler skeleton to mirror:
func (h *InventoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	ctx := r.Context()
	char := r.PathValue("char")  // only a `?` bind downstream (V5) — handler builds NO SQL
	inv, err := compute.StructuredInventory(ctx, h.store, char)
	...
}
```
For wishlist this dispatches to `compute.WishlistFor(ctx, st, callerID, char)` — note it ALSO needs the
session `discord_user_id` for the owner-scoped targets + badge set; read it from the `RequireSession`
context the same way `readapi/characters.go` reads the viewer id (see RESEARCH §"Component Responsibilities").

---

### `internal/backendsrv/webadmin/wishlist.go` (CREATE — write API, owner-scoped CRUD)

**Analog:** `webadmin/wantlist.go` (clone LINE-FOR-LINE — every security property carries over).

**Owner-derivation + validation pattern** (`wantlist.go:129-197` `AddWantHandler`):
```go
// webadmin/wantlist.go:154 — owner is ALWAYS the session caller, NEVER the body (D-02):
callerID := caller(r.Context()) // D-02: owner from session, body carries NO owner
...
// wantlist.go:164-172 — the T-28-05 in-tx IDOR guard authorizing the character tag BEFORE the insert:
if req.CharacterID != nil {
	ok, e := store.IsCharAssignedToTx(ctx, tx, *req.CharacterID, callerID)
	if e != nil { return e }
	if !ok { return store.ErrCharNotAssigned }   // → mapWantErr → 403
}
```
For wishlist, `character_id` is REQUIRED (not optional) — every target is char+slot-scoped — so the guard
ALWAYS runs. Add server-side `slot`-enum validation (must be a known canonical worn-slot — the `validWant`
precedent at `wantlist.go:73-95`, V5).

**Error-mapping pattern** (`wantlist.go:102-114` `mapWantErr`): `errors.Is(err, store.ErrDuplicateWishlist)`
→ 409 `{"error":"duplicate"}`; `store.ErrCharNotAssigned` → 403; else 500. Match on the TYPED sentinel
(NOT a string-match of the driver message).

**Audited single-tx pattern** (`wantlist.go:158-181`): add + `AppendAuditTx` in ONE `withTx` (BEGIN
IMMEDIATE); audit detail carries IDs ONLY — never the item label (V7, line 178-180). Use ops
`wishlist_add` / `wishlist_remove` / `wishlist_ping`.

**Remove + ping-toggle handlers** (`wantlist.go:305-346` `RemoveOwnWantHandler` + `wantlist.go:215-251`
`MuteWantHandler`): the silent-no-op contract — a cross-owner id returns `{removed:false}` / the requested
ping state but flips no row (line 247-249). The ping handler echoes the REQUESTED state.

**Test analog:** clone `webadmin/wantlist_test.go` → `webadmin/wishlist_test.go` (handler 400/403/409/200).

---

### `cmd/squirebot-server/main.go` (MODIFY — route registration)

**Analog:** itself, lines 339-348 (the wantlist block) + 390 (the mute route).

**Register the new wishlist routes** under `webauth.RequireSession` (NEVER `RequireOfficer` — owner-scoped;
the comment at `main.go:339-341` is the rationale to copy). Mirror the `inventory/{char}` registration at
`main.go:362` for the read route:
```go
// main.go:342-344 — the wantlist registration pattern (RequireSession, owner from session):
mux.Handle("GET /api/v1/wantlist", webauth.RequireSession(db, webadmin.ListOwnWantsHandler(db)))
mux.Handle("POST /api/v1/wantlist", webauth.RequireSession(db, webadmin.AddWantHandler(db)))
mux.Handle("POST /api/v1/wantlist/remove", webauth.RequireSession(db, webadmin.RemoveOwnWantHandler(db)))
// main.go:362 — the {char} read pattern:
mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
```
**REMOVE the 4 old wantlist routes** (Pitfall 7) — `main.go:342, 343, 344, 348` (`/api/v1/wantlist`,
`/api/v1/wantlist`, `/api/v1/wantlist/remove`, `/api/v1/wantlist/guild`) + the mute at `main.go:390`
(`/api/v1/wantlist/mute`) — else they 500 on the dropped table. New routes (recommended one read route +
3 writes): `GET /api/v1/wishlist/{char}`, `POST /api/v1/wishlist`, `POST /api/v1/wishlist/remove`,
`POST /api/v1/wishlist/ping`. The character LIST reuses `GET /api/v1/characters` (line 363, filter
banks/bots client-side).

---

### `web/src/routes/wishlist/+page.svelte` (REWRITE — master-detail component)

**Analogs:** `web/src/routes/inventory/+page.svelte` + `web/src/routes/characters/+page.svelte` (the
two-pane master-detail + viewer-first list + `?c=`/`?i=` URL seam + the `winStatus`/stale-response state
machine — both files exist and are the approved P31/P32 shapes the UI-SPEC mandates verbatim). The CURRENT
`/wishlist/+page.svelte` (the P30 placeholder, read above) mounts `WantlistPanel` + the Notifications
region — **KEEP the Notifications region** (NAV-04; `NotificationPrefsPanel` + `NotificationInbox`,
placeholder lines 48-62) but REPLACE the `WantlistPanel` mount (lines 38-46) with the per-character
per-slot master-detail.

**Reused components mounted as-is** (the UI-SPEC §reuse list, all confirmed present in
`web/src/lib/components/`): `ExaminePanel.svelte` (WISH-06 — props `slot?: InventorySlot | null` +
`charLastSeen = ''`, confirmed at `ExaminePanel.svelte:23-28`; build an `InventorySlot`-shaped object per
target, `charLastSeen=""`), `StateBlock`, `LastSyncedCell`, `ConfirmDialog`, `Toggle`.

**Worn-slot taxonomy (D-04)** — copy the 21-slot constants VERBATIM from `InventoryWindow.svelte:41-48`:
```js
// InventoryWindow.svelte:41-48 — the SAME paperdoll taxonomy (Charm/Power omitted, post-Velious):
const LEFT_SLOTS  = ['Head', 'Face', 'Ear1', 'Ear2', 'Neck', 'Shoulders', 'Arms', 'Back'];
const RIGHT_SLOTS = ['Wrist1', 'Wrist2', 'Hands', 'Finger1', 'Finger2', 'Chest', 'Legs', 'Feet', 'Waist'];
const WORN_SLOTS  = ['Primary', 'Secondary', 'Range', 'Ammo'];
```
The per-slot accordion iterates `[...LEFT_SLOTS, ...RIGHT_SLOTS, ...WORN_SLOTS]` (21 slots), each section
showing the equipped item (indexed by `canonical_slot`) + targets + suggestions.

**Add-form idiom (WISH-03)** — clone the `WantAddForm.svelte` debounced catalog/custom pattern (its script,
lines 1-90): the ~250ms debounce (`DEBOUNCE_MS = 250`, line 35), the `searchSeq` out-of-order guard (line 41),
the catalog-pin-OR-custom-label staging (line 45-46), the `onAdded` re-fetch callback (line 30-32). Do NOT
import the file as-is (it carries the wantlist priority/note fields); clone the debounce+seq-guard mechanics.

**Server-truth write discipline (load-bearing, DISTINGUISHES this tab)** — mirror `WantlistPanel.svelte`'s
add/remove/ping handlers (`onAdded` line 195-198, `doRemove` line 212-224, `onMute` line 246-262): EVERY
mutation `await`s the POST then RE-FETCHES the authoritative wishlist (`wants = await fetchOwnWants()` —
NEVER optimistic-mutate, T-19-16). Remove opens the `ConfirmDialog` (line 384-389). Port these three idioms
to `addWishlist`/`removeWishlist` (confirm)/`setWishlistPing` → re-fetch `fetchWishlist(char)`.

---

### `web/src/lib/wishlist/*.ts` (CREATE — pure DOM-free helpers)

**Analogs:** `web/src/lib/roster.ts` (the viewer-first sort + filter), `web/src/lib/items.ts` /
`web/src/lib/banks.ts` (the grouping helpers), `web/src/lib/wantlist/groupByChar.ts` (mine + adapt).

**Viewer-first-no-banks/bots filter (WISH-01)** — `roster.ts:26-51` is the exact analog. WISH-01 EXCLUDES
banks/bots, so DROP the `banks` band:
```ts
// roster.ts:26-30 bandOf — is_mine wins the tie; for WISH-01 EXCLUDE is_bank_toon||is_guild_bot OUTRIGHT:
export function bandOf(c: RosterCharacter): Band {
	if (c.is_mine) return 'mine';
	if (c.is_bank_toon || c.is_guild_bot) return 'banks';   // ◄── WISH-01: filter these OUT entirely
	return 'guild';
}
// roster.ts:47-51 filterRoster — the case-insensitive name filter that PRESERVES viewer-first order:
export function filterRoster(rows, query) {
	const q = query.trim().toLowerCase();
	const matched = q === '' ? rows : rows.filter((c) => c.name.toLowerCase().includes(q));
	return viewerFirst(matched);
}
```

**Per-slot grouping + cross-wishlist search (WISH-07)** — the `wantlist/groupByChar.ts` precedent (a pure
DOM-free decision helper, node-testable; its header comment lines 1-9 explains the node-vitest constraint).
Build: per-slot target grouping by `canonical_slot`, the D-02 auto-hide join (if done client-side), and the
WISH-07 cross-wishlist item search (the two-section results — CHARACTERS + WISHLIST ITEMS).

**Test analog:** new `web/src/lib/wishlist/*.test.ts` mirroring `roster.test.ts` / `items.test.ts` /
`wantlist/groupByChar.test.ts` (all node-tested pure helpers).

---

### `web/src/lib/api.ts` (MODIFY append-only — fetch wrappers)

**Analog:** itself, lines 744-838 (the wantlist wrappers + interfaces).

**Append `WishlistView`/`WishlistTarget` interfaces + the fetch/mutation wrappers** mirroring
`api.ts:776-838`:
```ts
// api.ts:776 fetchOwnWants — the credentialed GET wrapper to mirror:
export function fetchOwnWants(f = fetch) { return getJSON<WantlistRow[]>('/api/v1/wantlist', f); }
// api.ts:816-833 addWant — the POST wrapper; body carries NO owner (session-derived, D-02):
export function addWant(body, f = fetch) { return postJSON<WantlistRow>('/api/v1/wantlist', body, f); }
// api.ts:836 removeWant — { removed } (false = no-op, not an error):
export function removeWant(id, f = fetch) { return postJSON<{removed:boolean}>('/api/v1/wantlist/remove', {id}, f); }
```
→ `fetchWishlist(char)` (GET `/api/v1/wishlist/{char}`), `addWishlist(body)`, `removeWishlist(id)`,
`setWishlistPing(id, pinged)`. Reuse `searchCatalog` (`api.ts:811`) for the typed-entry add (UNCHANGED — it
hits the existing `/api/v1/items/search`).

---

### `web/src/routes/wantlist/+page.ts` (KEEP — verify the 308 redirect; D-03 already done)

**Analog:** itself. RESEARCH + the file (read above) confirm it is ALREADY a 308 stub
(`redirect(308, '/wishlist')`, `ssr=false`, `prerender=false`). **No edit — just confirm it still redirects.**
(P30 shipped this.) Leave `redirect()` uncaught (an exception handler would swallow the thrown Redirect).

---

### `web/src/lib/components/WantlistPanel.svelte` + `web/src/lib/wantlist/*.ts` (DELETE — after rewire)

**Action:** AFTER `/wishlist/+page.svelte` stops mounting `WantlistPanel` (and after mining its
server-truth re-fetch idioms — §wishlist/+page.svelte — and the pure helpers — §wishlist/*.ts), DELETE
`WantlistPanel.svelte` + the three `web/src/lib/wantlist/{groupByChar,holders,priority}.{ts,test.ts}`. These
become dead code (Pitfall 7 — a dangling `fetchOwnWants` import would break the build). Removing the old
`/api/v1/wantlist*` routes (main.go) + these files is the same-phase cleanup the clean break (D-01/D-03)
requires.

---

## Shared Patterns

### Owner-scoped IDOR-safe writes (V4)
**Source:** `store/wantlist.go:241-285` (`RemoveOwnWantTx` / `SetMutedTx`) + `webadmin/wantlist.go:129-197`
(`caller(ctx)` + `IsCharAssignedToTx`).
**Apply to:** `store/wishlist.go`, `webadmin/wishlist.go`.
The owner is ALWAYS `caller(r.Context())` (session-derived) — the body carries NO owner. Every mutator's
`WHERE id=? AND discord_user_id=? AND active=1` makes a cross-owner mutation a silent
`RowsAffected=0 → (false,nil)` no-op that never leaks the row's existence. The untrusted `character_id` is
authorized IN-TX via `store.IsCharAssignedToTx` BEFORE the insert (T-28-05) → `ErrCharNotAssigned` → 403.

### Typed duplicate detection (the 2067 idiom)
**Source:** `store/wantlist.go:42-46, 90-93` (`sqliteConstraintUnique = 2067` + `errors.As(&sqliteErr)`).
**Apply to:** `store/wishlist.go` (`ErrDuplicateWishlist`), `webadmin/wishlist.go` (`mapWishlistErr` → 409).
Detect the unique-index violation via the modernc driver's EXTENDED result code, NEVER by string-matching
"UNIQUE constraint failed".

### DM-target-is-the-want-owner invariant (T-28-06)
**Source:** `wantmatch/match.go:39-45` + `scanHits` line 106-108 + `embed.go:97 whyWanted` (nil-safe).
**Apply to:** the `wantmatch` repoint.
`DiscordUserID` is ALWAYS scanned from `wishlist_item.discord_user_id` (the want owner); `CharacterName` is
DISPLAY-ONLY and NEVER affects the DM recipient. Port the regression test.

### Compute-on-read, name-keyed cross-namespace join
**Source:** `store/readviews.go:464-476` (`GearTierPrices` `pp_rep` CTE `lower(trim(name))`) +
`compute/inventory.go` (the pure `build*` split).
**Apply to:** `compute/wishlist.go` (suggestions + auto-removal).
Catalog/gear-tier item_ids ≠ inventory item_ids — join EVERYTHING by normalized name
(`lower(trim(name))`), never by raw item_id. Keep the transform pure for node-testability.

### Server-truth writes, never optimistic (T-19-16)
**Source:** `WantlistPanel.svelte:195-262` (`onAdded`/`doRemove`/`onMute` all re-fetch).
**Apply to:** `web/src/routes/wishlist/+page.svelte`.
Add/remove/ping ALWAYS `await` the POST then re-fetch the authoritative wishlist. Remove gets the
`ConfirmDialog`. A non-owned character's wishlist renders read-only (no write controls).

### Single `{@html}` sink (T-31-14 / T-32-07 / T-33)
**Source:** `web/src/lib/components/ExaminePanel.svelte` (the escaped `composeItemNote` wiki line).
**Apply to:** every web file in this phase.
The ONLY raw-HTML body is `ExaminePanel`'s `composeItemNote` (reused UNCHANGED). Character/item/slot names
render via plain `{}` (Svelte auto-escapes). Any char name in a `?c=` deep-link is `encodeURIComponent`'d.

### V7 structured logging (Info disclosure)
**Source:** `readapi/inventory.go:19-20, 88-91` + `webadmin/wantlist.go:178-180`.
**Apply to:** all new backend files.
slog/audit carries op + counts + IDs + status ONLY — never the item label, char name, or any row content.

---

## No Analog Found

None. Every new/modified/deleted file maps to a concrete in-repo analog (HIGH confidence — this is a
reuse+repoint phase, not a new-invention phase). The genuinely novel logic (the schema, the slot↔wiki-slot
inverse map, the auto-removal name join, the badge read) is small and each piece has a direct structural
precedent listed above.

---

## Metadata

**Analog search scope:** `internal/backendsrv/{migrations,store,compute,enrich,webadmin,readapi,wantmatch,ec}/`,
`cmd/squirebot-server/`, `web/src/{routes,lib,lib/components,lib/wantlist}/`.
**Files scanned (read or grepped):** 24 source files across backend + web.
**Pattern extraction date:** 2026-06-18

---

## PATTERN MAPPING COMPLETE

**Phase:** 34 - Wishlist Rework — Per-Character Per-Slot Upgrades
**Files classified:** 19 (incl. 5 new test files)
**Analogs found:** 19 / 19

### Coverage
- Files with exact analog: 11
- Files with role-match analog: 3
- Files modifying themselves (self): 5
- Files with no analog: 0

### Key Patterns Identified
- The write API is a LINE-FOR-LINE `webadmin/wantlist.go` + `store/wantlist.go` clone (owner session-derived,
  2067 typed-dup detection, in-tx `IsCharAssignedToTx`, silent-no-op IDOR, audited single-tx).
- The `wantmatch` repoint is ONE SELECT in one file (`match.go`) with one live caller (`ec.go:211`,
  unchanged): `FROM wantlist_item`→`wishlist_item`, `muted=0`→`pinged=1`, `LEFT JOIN`→`JOIN`, drop `note`.
- Suggestions are a pure FILTER over `store.GearTierPrices` by class + the inverted
  `WIKI_SLOT_TO_INV_SLOTS` bridge; "Raid" tag = `tier == "Velious Raiding"` (no no-drop column exists).
- The web tab is the P31/P32 master-detail (`/inventory` + `/characters`) re-skinned to a per-slot
  accordion; reuses `ExaminePanel`/`Toggle`/`ConfirmDialog`/`StateBlock`/`LastSyncedCell` + the `roster.ts`
  viewer-first filter + the `WantAddForm` debounce, all unchanged or cloned.

### Two Highest-Risk Spots (flagged per the prompt)
1. **Slot-vocabulary bridge** — canonical `"Finger1"` → wiki `"Fingers"` via the inverse of
   `enrich/eqconst.go:65-83`. Three vocabularies (canonical Title-case / wiki-prose / UPPERCASE inv).
   Get it wrong → every suggestion list silently empty. Unit-test the inverse map.
2. **`alert_log` FK rebuild** — `00007_notify.sql:46-59` is the exact DROP+CREATE to mirror, changing the
   FK target to `wishlist_item(id)`; rebuild BEFORE dropping `wantlist_item`; recommended option (B) keeps
   the column name `wantlist_item_id` so `store/alertlog.go` needs no edit.

### File Created
`.planning/phases/34-wishlist-rework-per-character-per-slot-upgrades/34-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. Each new/modified/deleted file has a concrete analog with line-numbered excerpts.
The planner can reference these directly in PLAN.md action sections.
