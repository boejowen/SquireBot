# Phase 28: Character-Tagged Wantlist - Research

**Researched:** 2026-06-08
**Domain:** Backend (Go + SQLite/goose) schema extension + read API + Discord embed + SvelteKit web UI — extending the shipped Phase-19 wantlist with an optional `character_id` dimension.
**Confidence:** HIGH (every surface located and read in this repo; no external/unverifiable dependencies)

## Summary

Phase 28 adds an OPTIONAL `character_id` dimension to the shipped per-user wantlist. The work is almost entirely *threading one nullable column* through five already-built surfaces: the `wantlist_item` schema (migration `00010` → schema **v10**), the `store` CRUD (`AddWantTx` / `ListOwnWants` / `wantmatch`), the wantlist add/list HTTP handlers, the web add-form + list UI, and the EC-tunnel embed builder. Three of the six requirements (CWANT-01/02/06) are pure additive plumbing of an existing pattern. The remaining three (CWANT-03/04/05) carry the only genuinely *new* construction.

**The single most load-bearing finding: there is NO existing "guildwide wantlist" surface to extend.** The shipped wantlist is exclusively per-user — `ListOwnWants` (owner-scoped `WHERE discord_user_id = ?`), `GET /api/v1/wantlist` (session-scoped), `/wantlist` page (the caller's own list only). `wantmatch.ForItem`/`ForName` query across all users, but only *internally* for alert fan-out — they are never surfaced as a readable list. So CWANT-03/04 ("character-tagged wants roll up into the guildwide wantlist") is **net-new**: a new all-members read store func + a new read API route + a new web display. The planner must build this, not extend a fork.

**Primary recommendation:** Migration `00010` adds a single `character_id INTEGER REFERENCES character(id)` (nullable, NO `ON DELETE CASCADE`), extends BOTH partial-unique dedup indexes to include `character_id` so the same item can be wanted for two different characters, threads the column through the existing store/handler/UI verbatim using the established patterns, and (for CWANT-03/04/05) adds a new guildwide read + adds a `CharacterName *string` field to `wantmatch.Hit` (a LEFT JOIN to `character.name`) so the EC embed can name the character WITHOUT touching the owner-targeting (`Hit.DiscordUserID` is unchanged). Server-side IDOR guard: a new `IsCharAssignedToTx(characterID, callerID)` check rejects tagging a want to a character the caller is not assigned (no such guard exists yet — must be written).

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CWANT-01 | When adding a want, a member can OPTIONALLY tag it to one of their assigned characters | `wantlist_item.character_id` nullable column (00010) + `AddWantTx` gains `characterID *int64` param + add-form `<select>` sourced from `fetchMyCharacters()`. NEW server-side IDOR guard `IsCharAssignedToTx` (none exists). See Standard Stack, Architecture Patterns §1, Security Domain. |
| CWANT-02 | Untagged / pre-existing wants remain valid; 00010 backfills existing wants to NULL with no data loss | `ALTER TABLE wantlist_item ADD COLUMN character_id INTEGER` (no DEFAULT, no NOT NULL) ⇒ every existing row keeps `character_id = NULL` automatically. SQLite `ALTER … ADD COLUMN` is in-place, non-rewriting. See Architecture Patterns §1, Common Pitfalls #1. |
| CWANT-03 | Character-tagged wants aggregate ("filter up") into the guildwide wantlist alongside untagged wants | NET-NEW: no guildwide surface exists. NEW `ListGuildWants` store func (all-members, `LEFT JOIN character`), NEW `GET /api/v1/wantlist/guild` route (RequireSession), NEW web display. See Don't Hand-Roll, Architecture Patterns §2. |
| CWANT-04 *(open→LOCKED: SHOW attribution)* | Guildwide list surfaces per-want character/owner attribution | The new `ListGuildWants` SELECT adds `owner` (discord identity / display name) + `character_name` (LEFT JOIN). Resolve owner→display-name via `web_user.username` (already joined elsewhere). See Architecture Patterns §2, Open Questions Q1. |
| CWANT-05 *(open→LOCKED: NAME the character)* | EC-tunnel DM still targets the OWNER; embed names the tagged character (omitted when untagged) | Add `CharacterName *string` to `wantmatch.Hit` (LEFT JOIN `character` on `character_id`). `Hit.DiscordUserID` (the DM target) is structurally UNCHANGED — owner targeting is untouched. `embed.go` `buildEmbed` appends a "For" line when `hit.CharacterName != nil`. See Architecture Patterns §3, Common Pitfalls #3. |
| CWANT-06 | A member can filter/group their own wantlist by character | `ListOwnWants` returns `character_id` + `character_name` (LEFT JOIN); pure client-side group/filter in `WantlistPanel` (the P27 `myview.ts` precedent — DOM-free helper + node test). See Architecture Patterns §4. |
</phase_requirements>

## User Constraints (from ROADMAP / STATE / additional_context — no CONTEXT.md yet)

> No `28-CONTEXT.md` exists at research time (discuss-phase has not run). The locked decisions below come from the milestone scope (STATE.md / ROADMAP.md) and the additional_context handed to this researcher (resolved 2026-06-08). Treat them as locked; do NOT reopen.

### Locked Decisions
- **NEW migration `00010` → schema v10.** NOT 00009 — `00009_character_assignment.sql` already shipped (Phase 26) and is live on prod at schema v9. `[VERIFIED: migrations dir — 00001..00009 all present; 00009 == character_assignment]`.
- **`character_id` is NULLABLE** on `wantlist_item`, FK → `character.id`, extend-only/idempotent; existing wants keep `character_id = NULL` (account-level). No data loss.
- **Backend-only schema change — watcher UNTOUCHED.** No `WatcherMaxSchemaVersion` concern (the watcher targets the ingest API, not these backend tables — same as 00009). `[VERIFIED: 00009 header comment states exactly this; watcher reads no backend tables]`.
- **CWANT-04 = SHOW attribution** (per-want character/owner on the guildwide list, not an aggregate count).
- **CWANT-05 = EC embed NAMES the character** (e.g. "For Slampeach"); the DM TARGET stays the owner (`discord_user_id`), unchanged. An untagged want's embed omits the character name.
- **The tag source = the caller's assigned characters** — reuse Phase 26 `fetchMyCharacters()` / `GET /api/v1/assignments/mine`; do NOT invent a new "my characters" source.
- **Consolidated views rule (CLAUDE.md LOCKED)** — no per-character view tabs. (Tangential here: the wantlist is a 5th DataGrid, not a view tab; group-by-character is a client-side filter, not a new tab.)
- **Schema doctrine (CLAUDE.md):** extend-only (add column at right edge), version-stamped, idempotent; forward-only Down no-op (the 00004–00009 precedent).

### Claude's Discretion
- The exact guildwide display shape (a 6th DataGrid vs. a section on `/wantlist` vs. a new route) — recommend in plan; see Open Questions Q2.
- Embed "For <char>" wording/placement (title suffix vs. a field) — see Architecture Patterns §3.
- Whether group-by-character on the personal list is a `<select>` (P27 precedent) or grouped sections.

### Deferred Ideas (OUT OF SCOPE)
- Wantlist sharing beyond the guildwide read (e.g. cross-member edit) — REQUIREMENTS.md "Future Requirements".
- WTB monitoring, price-threshold alerts, auto-derived quest→NPC — REQUIREMENTS.md "Future Requirements".
- Backlog **999.33** (officer panel designated-char one-way door) — a Phase-26 UI gap; *a candidate to fold in* per the backlog note, but NOT a CWANT requirement. Flag to the planner as optional.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Persist the optional character tag | Database / Storage (SQLite, migration 00010) | — | The tag is durable per-want state; the FK + dedup-index change is schema, not app logic |
| Validate the tag is the caller's own char (IDOR) | API / Backend (`store` + `webadmin`) | — | Authorization MUST be server-side; never trust the client `<select>` (V4/V5) |
| Add-with-tag, list-with-char-name | API / Backend (`store.AddWantTx` / `ListOwnWants`) | — | Owner-scoped mutation + read; the existing wantlist.go pattern |
| Guildwide roll-up read (CWANT-03/04) | API / Backend (NEW `ListGuildWants` + route) | — | All-members aggregate; a guild-wide read like `wantmatch` but surfaced |
| EC embed names the character (CWANT-05) | API / Backend (`wantmatch.Hit` + `ec/embed.go`) | — | The embed is built server-side in the bot goroutine; owner-targeting stays in `notify.Send` |
| Tag `<select>` + group-by-char filter UI | Frontend Server (SvelteKit static) → Browser | — | Pure presentation over already-served data; the P27 client-filter precedent |

## Standard Stack

This phase introduces NO new libraries. It reuses the shipped v2.x stack verbatim.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.25.7 | Backend language | `[VERIFIED: go.mod]` — the live backend |
| modernc.org/sqlite | (in go.mod) | CGO-free SQLite driver | `[VERIFIED: store/wantlist.go imports it]` — extended result codes already used for dedup |
| pressly/goose | (in go.mod) | Forward-only SQL migrations | `[VERIFIED: migrations/ dir, 00001..00009]` — the migration mechanism; "schema vN" == `00N` applied |
| bwmarrin/discordgo | v0.29.0 | Discord embed send (CWANT-05) | `[VERIFIED: ec/embed.go imports it]` — the only Discord seam; CWANT-05 only SHAPES the embed |
| SvelteKit (Svelte 5 runes) | (web/) | Static frontend | `[VERIFIED: WantAddForm.svelte uses $state/$derived/$props]` |
| vitest | (web/) | Node-only web tests | `[VERIFIED: P27 myview.test.ts; config skips browser]` |

### Supporting (existing patterns to reuse, not libraries)
| Pattern | Location | Purpose |
|---------|----------|---------|
| `webauth.RequireSession` | `cmd/squirebot-server/main.go` | Login-only route wrapper (every wantlist route uses it) |
| `withTx` + `AppendAuditTx` | `webadmin/audit.go` | Atomic mutation + audit (used by `AddWantHandler`) |
| `caller(ctx)` | `webadmin/officers.go` | Session-derived discord_user_id (D-02 — never the body) |
| `fetchMyCharacters()` / `MyCharacter` | `web/src/lib/api.ts:906` / `:832` | The tag `<select>` option source (CWANT-01) |
| `ListMyAssignments` | `store/assignment.go:446` | The Go-side "my characters" read (LEFT JOIN `character c ON c.id = a.character_id`, `SELECT c.name`) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `ALTER TABLE ADD COLUMN character_id` | A separate `wantlist_character` junction table | Rejected — a want has at most ONE character (the locked model is single-tag, mirroring `character_assignment`'s single-assignee PK). A nullable column is the extend-only-doctrine fit; a junction adds a join for zero gain. |
| Extending the dedup indexes to include `character_id` | Leaving the indexes as-is | MUST extend — see Common Pitfalls #2. Leaving them collides "want X for char A" with "want X for char B". |
| New `ListGuildWants` store func | Reusing `wantmatch.ForItem` per item | Rejected — `wantmatch` is per-item fan-out, not a listable roll-up; a guildwide list is one query across all active wants. |

**Installation:** None — no new dependencies.

**Version verification:** No package install this phase. Existing deps confirmed present in `go.mod` (go 1.25.7) and `web/` lockfile. `[VERIFIED: go.mod, repo grep]`.

## Architecture Patterns

### System Architecture Diagram

```
ADD-WITH-TAG (CWANT-01/02)                          GUILDWIDE ROLL-UP (CWANT-03/04, NET-NEW)
  Browser: WantAddForm                                Browser: NEW guildwide display
   <select> sourced from fetchMyCharacters()           │ GET /api/v1/wantlist/guild
   │ POST /api/v1/wantlist {…, character_id?}           ▼
   ▼                                                  RequireSession → NEW ListGuildWantsHandler
  RequireSession → AddWantHandler                      │
   │ validWant() + NEW: if character_id set,            ▼
   │   IsCharAssignedToTx(char, caller)  ──reject──▶ 403  store.ListGuildWants(db)
   │   (IDOR guard — NONE exists today)                  SELECT … w.*, wu.username AS owner,
   ▼                                                       c.name AS character_name
  withTx: AddWantTx(…, characterID) + AppendAuditTx        FROM wantlist_item w
   │ INSERT … character_id                                 JOIN web_user wu ON wu.discord_user_id = w.discord_user_id
   ▼                                                       LEFT JOIN character c ON c.id = w.character_id
  wantlist_item (00010: + character_id NULLABLE,           WHERE w.active = 1
   FK→character.id, dedup indexes incl. character_id)

PERSONAL LIST + GROUP-BY-CHAR (CWANT-06)            EC EMBED NAMES CHARACTER (CWANT-05)
  GET /api/v1/wantlist → ListOwnWants                 scheduler.ec_auction_match → ec.RunMatch
   (LEFT JOIN character → character_id+name on row)    │ wantmatch.ForItem(item_id)
   ▼                                                    │   SELECT … LEFT JOIN character  ──▶ Hit.CharacterName *string
  WantlistPanel: client-side group/filter             │   (Hit.DiscordUserID UNCHANGED = owner = DM target)
   (DOM-free helper + node test, P27 myview precedent) ▼
                                                      buildEmbed: if hit.CharacterName != nil → "For <name>"
                                                       ▼
                                                      notify.Send(Alert{DiscordUserID: hit.DiscordUserID, Embed})
                                                       └── owner-targeting in notify.Send is UNTOUCHED
```

### Recommended Project Structure (files this phase touches/adds)
```
internal/backendsrv/
├── migrations/00010_character_tagged_wantlist.sql   # NEW — ADD COLUMN + dedup-index swap
├── migrations/migrate_test.go                       # EDIT — add TestMigrate_00010_*
├── store/wantlist.go                                # EDIT — AddWantTx +characterID; ListOwnWants +char; NEW ListGuildWants
├── store/wantlist_test.go                           # EDIT — tag/dedup/guild tests
├── store/assignment.go                              # EDIT — NEW IsCharAssignedToTx (the IDOR guard)
├── wantmatch/match.go                               # EDIT — Hit.CharacterName via LEFT JOIN
├── ec/embed.go                                      # EDIT — buildEmbed "For <char>" line
└── webadmin/wantlist.go                             # EDIT — addWantReq +character_id; NEW ListGuildWantsHandler
cmd/squirebot-server/main.go                         # EDIT — register GET /api/v1/wantlist/guild
web/src/lib/
├── api.ts                                           # EDIT — WantlistRow +character_id/name; addWant body; NEW fetchGuildWants
├── wantlist/groupByChar.ts                          # NEW — DOM-free group/filter helper (P27 myview precedent)
└── components/
    ├── WantAddForm.svelte                           # EDIT — character <select> from fetchMyCharacters
    └── WantlistPanel.svelte                         # EDIT — group-by-char filter; (guildwide display TBD shape)
```

### Pattern 1: Extend-only nullable column + dedup-index swap (the `00010` migration)
**What:** Add `character_id` nullable, then DROP + recreate the two partial-unique dedup indexes to include `character_id` in their key.
**When to use:** This phase's schema change. Mirrors `00006`'s partial-unique-index idiom exactly.
**Example:**
```sql
-- +goose Up
-- Phase 28 (CWANT-01/02). Optional character tag on the per-user wantlist.
-- Backend-only: the watcher never reads wantlist_item, so NO WatcherMaxSchemaVersion
-- change (same as 00009). "Schema v10" == goose 00010 applied.
ALTER TABLE wantlist_item ADD COLUMN character_id INTEGER REFERENCES character(id);
-- NB: SQLite ADD COLUMN with a REFERENCES clause does NOT enforce the FK retroactively;
-- existing rows keep character_id NULL (CWANT-02 backfill = automatic, no rewrite).

-- The dedup key MUST now include character_id so the SAME item can be wanted for two
-- different characters (CWANT-01). SQLite treats NULL as DISTINCT in a UNIQUE index, so
-- two account-level (character_id NULL) wants for the same (user,item,reason) would NO
-- LONGER collide if we naively add character_id — that REGRESSES the 00006 dedup. Use
-- COALESCE(character_id, -1) in the index expression so NULL collapses to a single
-- sentinel and account-level dedup is preserved, while distinct char ids stay distinct.
DROP INDEX wantlist_catalog_uidx;
DROP INDEX wantlist_custom_uidx;
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(
  discord_user_id, item_id, reason, COALESCE(character_id, -1)
) WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx ON wantlist_item(
  discord_user_id, item_name, reason, COALESCE(character_id, -1)
) WHERE item_id IS NULL AND active = 1;

-- +goose Down
SELECT 1; -- forward-only (mirrors 00004-00009)
```
`[VERIFIED: 00006_wantlist.sql index definitions; SQLite expression-index support]`
**The dedup decision (answers a key planner question):** YES, the dedup key must change. The same item *for two different characters* must be two rows. The COALESCE-sentinel preserves the existing account-level (NULL) dedup while making distinct character ids distinct. `[ASSUMED: that "same item, two chars = two rows" is the intended product behavior — see Assumptions Log A1. This is the natural reading of CWANT-01 but should be confirmed in discuss/plan.]`

### Pattern 2: The guildwide roll-up (NET-NEW — CWANT-03/04)
**What:** A new all-members read. There is no existing surface; this is built fresh.
**When to use:** CWANT-03 ("filter up into the guildwide wantlist") + CWANT-04 (attribution).
**Example (store):**
```go
// ListGuildWants returns every ACTIVE want across ALL members with per-want owner +
// optional character attribution (CWANT-03/04). Read-only (plain *sql.DB), non-nil
// slice (JSON []), parameterized. The owner display name joins web_user.username; the
// character name LEFT-JOINs character (NULL ⇒ account-level want, no name).
func ListGuildWants(ctx context.Context, db *sql.DB) ([]GuildWantRow, error) {
    rows, err := db.QueryContext(ctx,
        `SELECT w.id, w.item_id, w.item_name, w.reason, w.priority,
                w.discord_user_id, wu.username AS owner, w.character_id, c.name AS character_name
           FROM wantlist_item w
           JOIN web_user wu ON wu.discord_user_id = w.discord_user_id
           LEFT JOIN character c ON c.id = w.character_id
          WHERE w.active = 1
          ORDER BY w.created_at DESC`)
    // … scan nullable character_id/character_name via sql.Null* → pointers
}
```
**Route:** `mux.Handle("GET /api/v1/wantlist/guild", webauth.RequireSession(db, webadmin.ListGuildWantsHandler(db)))` — login-gated like every other read (the v2.2 read API is login-only, NOT public — `[VERIFIED: MEMORY — "read API login-gated since P15 (NOT public)"]`).
`[CITED: store/wantlist.go ListOwnWants shape; assignment.go:480 LEFT JOIN character precedent]`

### Pattern 3: EC embed names the character WITHOUT touching owner-targeting (CWANT-05)
**What:** Add one nullable field to `wantmatch.Hit`; the DM target field is untouched.
**Why it's safe:** `notify.Send` reads `Alert.DiscordUserID` for the DM target (`UserChannelCreate(a.DiscordUserID)`). That value comes from `Hit.DiscordUserID`, which is `wantlist_item.discord_user_id` — the want's OWNER — and is NOT derived from `character_id`. Naming the character is a *display-only* addition. `[VERIFIED: notify/dm.go:161 UserChannelCreate(a.DiscordUserID); ec/ec.go:258 DiscordUserID: hit.DiscordUserID]`.
**Example:**
```go
// wantmatch.Hit gains:
type Hit struct {
    WantID        int64
    DiscordUserID string  // UNCHANGED — the DM target = the want owner
    ItemID        *int64
    ItemName      string
    Reason        string
    Note          *string
    CharacterName *string // NEW (CWANT-05): the tagged char's name, NULL ⇒ untagged
}
// ForItem/ForName SELECT gains a LEFT JOIN:
//   SELECT w.id, w.discord_user_id, w.item_id, w.item_name, w.reason, w.note, c.name
//     FROM wantlist_item w LEFT JOIN character c ON c.id = w.character_id
//    WHERE w.item_id = ? AND w.active = 1 AND w.muted = 0
// (scanHits gains a nullable character_name scan → *string)

// ec/embed.go buildEmbed, after the "Why you wanted it" field:
if hit.CharacterName != nil && strings.TrimSpace(*hit.CharacterName) != "" {
    fields = append(fields, &discordgo.MessageEmbedField{
        Name: "For", Value: *hit.CharacterName, Inline: true,
    })
}
```
`[CITED: wantmatch/match.go scanHits; ec/embed.go buildEmbed]`

### Pattern 4: Group-by-character on the personal list (CWANT-06) — the P27 client-filter precedent
**What:** A DOM-free helper (`groupByChar.ts`) + node test, then a `<select>` in `WantlistPanel` feeding the grid pre-filtered/grouped rows. ZERO new backend beyond `ListOwnWants` returning the char name.
**When to use:** CWANT-06. This is the *exact* shape Phase 27 shipped for the inventory filter (`web/src/lib/myview.ts` + `myview.test.ts`, 9 cases, wired into a `<select>`). Reuse it.
`[VERIFIED: STATE.md P27 — "pure web/src/lib/myview.ts filter helper + node test, then the single <select>"; the file exists at web/src/lib/myview.ts]`

### Anti-Patterns to Avoid
- **Forking the wantlist into a "guildwide" copy.** There is no guildwide surface to fork — build the new read cleanly. Do NOT duplicate `ListOwnWants` and strip the owner scope inline in a handler; add a distinct `ListGuildWants`.
- **Trusting the client `<select>` for the character tag.** The add-form sends `character_id`; the server MUST re-validate it is one of the caller's assigned characters (IDOR — see Security Domain). No such guard exists today.
- **Deriving the DM target from `character_id`.** The owner is `wantlist_item.discord_user_id`, NOT the character's owner_id and NOT the character's current assignee. Keep `Hit.DiscordUserID` exactly as-is.
- **Adding `ON DELETE CASCADE` to the FK.** A deleted character should NOT delete the want — it should fall back to account-level. Plain `REFERENCES character(id)` (no cascade); the LEFT JOIN already renders a dangling/deleted char as NULL name. (SQLite also does not enforce FKs unless `PRAGMA foreign_keys=ON`; confirm the app's pragma — see Open Questions Q3.)
- **A new view tab.** Group-by-character is a client-side filter over one DataGrid, never a per-character tab (CLAUDE.md LOCKED).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Which characters are mine?" (tag options) | A new my-characters query/endpoint | `fetchMyCharacters()` / `GET /api/v1/assignments/mine` (P26) | Locked decision; the authoritative source. Inventing a second one risks divergence. |
| Owner-scoped IDOR-safe mutation | A bespoke ownership check | The `RemoveOwnWantTx` / `caller(ctx)` session-scoped pattern | Already the audited, tested shape; the character IDOR guard is an *added* read inside the same tx. |
| Dedup of duplicate wants | App-level "does this exist?" SELECT-then-INSERT | The partial-unique index + `ErrDuplicateWant` extended-result-code detection | Race-free at the DB; already shipped. Just extend the index key. |
| Char name for display | Storing a name snapshot on the want | LEFT JOIN `character.name` at read time | `character.name` is authoritative + unique; P27 verified the name-join consistency (raw `SELECT c.name`, COLLATE NOCASE is ORDER-BY-only). |
| Client-side group/filter | A stateful store + manual reactivity | The P27 `myview.ts` DOM-free-helper + `$derived` precedent | Node-testable, DOM-blind-safe, already proven this milestone. |

**Key insight:** Nearly every "new" capability in this phase already exists one surface over — the *only* genuinely new constructions are the guildwide read (CWANT-03/04) and the `IsCharAssignedToTx` IDOR guard. Everything else is threading `character_id` through established, tested code with its established pattern.

## Runtime State Inventory

> This is a schema-extension phase, not a rename/refactor. There is no string-rename runtime state to chase. The relevant "what persists" analysis is the migration's effect on live data — covered here for completeness.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | Live `wantlist_item` rows on prod (schema v9). `00010` adds `character_id` — every existing row auto-backfills to NULL (CWANT-02). | Migration only — no data migration; `ADD COLUMN` is in-place. |
| Live service config | None — no UI/DB-resident config references the wantlist schema. | None — verified: `wantlist_item` is read only by the backend store/handlers/wantmatch, all in git. |
| OS-registered state | None — the `ec_auction_match` scheduler job is registered by name in `main.go`, not OS-level; no new job this phase. | None. |
| Secrets/env vars | None — no new secret. `DISCORD_BOT_TOKEN` (existing) unchanged. | None. |
| Build artifacts | The web bundle (`web/dist`) rebuilds on `npm run build`; the Go binary rebuilds. Standard deploy. | Rebuild + redeploy per `docs/backend-deploy.md` (binary swap for the schema migration via `goose.Up`; web tarball atomic-swap). |

**Deploy note:** This phase ships a LIVE schema migration (`00010`, v9→v10). The deploy path is the proven P26 one: R2 backup → binary swap (`.bak` kept) → `goose.Up` applies `00010` → web tarball atomic-swap. `[VERIFIED: STATE.md P26 deploy log; docs/backend-deploy.md referenced throughout]`.

## Common Pitfalls

### Pitfall 1: Assuming `ADD COLUMN` needs a backfill step
**What goes wrong:** Planning a separate "backfill existing rows to NULL" task.
**Why it happens:** Treating SQLite like a system where new columns need population.
**How to avoid:** `ALTER TABLE … ADD COLUMN character_id INTEGER` (no DEFAULT, no NOT NULL) makes every existing row NULL automatically and in-place — that IS the CWANT-02 backfill. The migrate test asserts existing rows read NULL; no data task.
**Warning signs:** A plan task titled "backfill character_id" — delete it.

### Pitfall 2: Naive dedup-index extension regresses account-level dedup
**What goes wrong:** Adding `character_id` to the unique index key directly. SQLite treats NULL as DISTINCT in a UNIQUE index, so two account-level wants (`character_id NULL`) for the same (user,item,reason) STOP colliding — re-introducing the exact duplicate-want bug 00006 fixed.
**Why it happens:** The 00006 header even documents the "NULL is DISTINCT" trap for `item_id`; it bites again for `character_id`.
**How to avoid:** Index on `COALESCE(character_id, -1)` so NULL collapses to one sentinel (account-level dedup preserved) while real char ids stay distinct (same item for two chars allowed). `[VERIFIED: 00006_wantlist.sql comment block explains the identical NULL-DISTINCT trap]`.
**Warning signs:** A migrate test that can insert two identical account-level wants without a constraint error.

### Pitfall 3: Conflating the DM target with the character
**What goes wrong:** "Tagged to a character → DM the character's owner/assignee" — re-deriving the target from `character_id`.
**Why it happens:** It *sounds* right ("the want is for Slampeach, so tell Slampeach's owner").
**How to avoid:** The DM target is ALWAYS the want's own `discord_user_id` (the member who created the want) — locked (CWANT-05) and structurally so (`Hit.DiscordUserID` is `wantlist_item.discord_user_id`, never touched). The character name is display-only. A member tags their OWN want to their OWN character, so in practice owner == assignee anyway, but the *target field* must not change.
**Warning signs:** Any code reading `character.owner_id` or `character_assignment.discord_user_id` in the notify/embed path.

### Pitfall 4: web vitest is DOM-blind (the P15/P26/P27 trap)
**What goes wrong:** Green node tests, broken browser UI (the `<select>` doesn't render, group-by doesn't apply).
**Why it happens:** No `@testing-library/svelte` (toolchain-install rule); vitest runs node-only.
**How to avoid:** Node-test the *pure* parts (the `groupByChar.ts` helper, the store, the handlers, `buildEmbed`, `IsCharAssignedToTx`). Browser-smoke the *interactive* parts (the character `<select>` in the add-form, the group-by-char filter, the guildwide display) on a DEPLOYED build or full local stack — `npm run dev` bounces login against prod. `[VERIFIED: MEMORY web-tests-node-only + web-local-dev-cant-auth-against-prod; STATE.md P27 smoke gap]`.
**Warning signs:** "all tests green" claimed as "verified" without a deploy-then-smoke.

### Pitfall 5: Forgetting `IsCharAssignedToTx` — the IDOR gap
**What goes wrong:** A member POSTs `character_id` of a char they don't hold; the server stores it; the guildwide list now mis-attributes the want.
**Why it happens:** No such guard exists — the wantlist handlers today have NO character concept, and `store/assignment.go` has no "is char C assigned to user U?" reader (closest is the OfficerAssignTx upsert).
**How to avoid:** Add `IsCharAssignedToTx(ctx, tx, characterID, callerID) (bool, error)` (`SELECT 1 FROM character_assignment WHERE character_id=? AND discord_user_id=?`); `AddWantHandler` rejects with 403/400 when `character_id` is set but the guard returns false. `[VERIFIED: grep of assignment.go — no such function exists]`.
**Warning signs:** `AddWantTx` accepting `character_id` with no preceding assignment check.

## Code Examples

### Verified existing pattern: owner-scoped add with audit (the shape CWANT-01 extends)
```go
// Source: internal/backendsrv/webadmin/wantlist.go:115 AddWantHandler (existing)
err := withTx(ctx, db, func(tx *sql.Tx) error {
    // NEW for CWANT-01: validate the optional character tag is the caller's own.
    if req.CharacterID != nil {
        ok, e := store.IsCharAssignedToTx(ctx, tx, *req.CharacterID, callerID)
        if e != nil { return e }
        if !ok { return store.ErrCharNotAssigned } // → mapWantErr → 403/400
    }
    id, e := store.AddWantTx(ctx, tx, callerID, req.ItemID, itemName, req.Reason, priority, notePtr, req.CharacterID, now)
    if e != nil { return e }
    newID = id
    return AppendAuditTx(ctx, tx, "wantlist_add", callerID,
        map[string]any{"item_id": req.ItemID, "character_id": req.CharacterID}, now) // V7: ids only
})
```

### Verified existing pattern: the P27 client-filter helper to mirror (CWANT-06)
```typescript
// Source: web/src/lib/myview.ts (P27) — the DOM-free pattern to clone as groupByChar.ts
// applyMyFilter(rows, myCharNameSet) → filtered rows; node-tested in myview.test.ts (9 cases)
// CWANT-06's groupByChar(wants, charId|null) is the identical shape over WantlistRow[].
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Wantlist is per-user only | Wantlist gains optional `character_id` + a guildwide read | This phase (00010 / v10) | The first all-members wantlist surface; the first character dimension on a want |
| `wantmatch.Hit` carries no char | `Hit.CharacterName` (LEFT JOIN) | This phase | EC embed can name the char; owner-targeting unchanged |
| Schema v9 (00009 character_assignment, live) | Schema v10 (00010) | This phase | The roadmap text saying "00009" for v2.3's char_id is STALE — 00009 shipped as character_assignment; this phase is 00010 |

**Deprecated/outdated:**
- ROADMAP.md / REQUIREMENTS.md text "`00009` migration → schema v9 … `character_id` NULLABLE on `wantlist_item`" — that conflated P26 and P28. **Correct:** P26 shipped `00009` (character_assignment, v9, LIVE). P28 ships `00010` (wantlist `character_id`, v10). `[VERIFIED: migrations dir + STATE.md P26 deploy log "goose.Up applied 00009 (schema v8→v9)"]`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | "Same item wanted for two different characters" = two distinct rows (drives the COALESCE dedup-index design) | Pattern 1, Common Pitfalls #2 | If product intends "one row, multi-char" the junction-table model would be needed instead — a larger schema change. Natural reading of CWANT-01; confirm in discuss/plan. |
| A2 | The guildwide list owner-attribution should display `web_user.username` (Discord display name) rather than the raw `discord_user_id` | Pattern 2, Open Questions Q1 | Showing a raw snowflake id is poor UX; username is the human-readable field. Low risk — easy to adjust the SELECT. |
| A3 | The guildwide read should be login-gated (RequireSession), not public | Pattern 2 | The whole read API has been login-gated since P15; a public guildwide want list would contradict that. Low risk. |

**If this table is empty:** It is not — three assumptions are flagged for confirmation in discuss/plan. None block planning; all have low-cost adjustments.

## Open Questions

1. **CWANT-04 attribution: show owner display-name + character, or character only?**
   - What we know: the locked decision is "SHOW attribution" (character/owner per want). `web_user.username` is the display name; `character.name` is the char.
   - What's unclear: whether owner display-name is shown alongside the character, or just the character.
   - Recommendation: show both (owner + character) — a guildwide list's value is knowing WHO wants it. Default to `web_user.username`; resolve in discuss/plan.

2. **Where does the guildwide list live? (Claude's discretion)**
   - Options: (a) a 6th DataGrid section on `/wantlist`; (b) a new `/wantlist/guild` route; (c) a toggle on `/wantlist` (My / Guild) mirroring P27's My/All toggle.
   - Recommendation: (c) a My/Guild toggle on `/wantlist` — consistent with the P27 mental model and reuses the existing page shell. Confirm in plan.

3. **Is `PRAGMA foreign_keys = ON` set on the backend connection?**
   - What we know: the FK clause is documentation-correct regardless; SQLite only *enforces* FKs when the pragma is on (off by default per-connection).
   - What's unclear: whether the modernc driver/db.go sets it. If OFF, a deleted character won't trip the FK but the LEFT JOIN still renders NULL (the desired fallback), so the no-cascade design is safe either way.
   - Recommendation: planner reads `store/db.go` to confirm the pragma; either way the LEFT-JOIN-renders-NULL fallback holds. Low risk.

## Environment Availability

> Skip — no NEW external dependency. The existing prod stack (Hetzner VPS, SQLite, discordgo bot, scheduler) is live and unchanged. The migration deploys via the proven `goose.Up` path. No tool to probe.

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high`. `[VERIFIED: .planning/config.json]`

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | Owner identity is session-derived (`caller(ctx)`), never the body (D-02) — the established wantlist posture |
| V2 Authentication | no (reuses) | Discord OAuth session (P15) — unchanged this phase |
| V3 Session Management | no (reuses) | `RequireSession` wrapper — unchanged |
| V4 Access Control | **yes (NEW)** | **The character-tag IDOR guard: `IsCharAssignedToTx` rejects tagging a want to a character the caller is not assigned.** This is the phase's primary new control — no such guard exists today. Owner-scope on add/list/guild reads is preserved. |
| V5 Input Validation | yes | `validWant` re-validates server-side; `character_id` is a nullable int — validate it's a positive int when present, then authorize via V4 |
| V6 Cryptography | no | None introduced |
| V7 Error/Logging | yes | Audit detail + slog carry `character_id`/ids ONLY — never note text, never the embed body (the existing V7 discipline; extend to the new audit field) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tag a want to another member's character (IDOR) | Elevation / Tampering | NEW `IsCharAssignedToTx` server-side check inside the add tx (Pitfall 5) — the locked decision #4 enforced in code |
| Guildwide read leaks more than intended | Information Disclosure | Login-gate (`RequireSession`); the read API has been login-only since P15. Surface only item/reason/owner-name/character-name — no notes? (notes are the wantlister's private context — recommend EXCLUDING note from the guildwide read; confirm in plan) |
| SQL injection via item/char fields | Tampering | Parameterized `?` placeholders ONLY (the established store discipline) |
| XSS via item name / character name in the new UI | Tampering | Svelte `{}` auto-escape only — never `{@html}` (the WantAddForm T-19-13 boundary; character.name is wiki/in-game text) |
| Embed injection via character name (CWANT-05) | Tampering | discordgo embed fields are plain-text values, not markdown-evaluated for injection of mentions/links by default; character.name is constrained in-game text. Low risk; do not interpolate into a URL without `EncodeURIComponent` (the existing `wikiURLFor` discipline) |
| DM mis-targeting (wrong recipient) | Spoofing | `Alert.DiscordUserID` = want owner, structurally unchanged (Pitfall 3); a regression test asserting the target == owner regardless of `character_id` is the backstop |

**Security recommendation for the planner:** EXCLUDE the private `note` field from the guildwide roll-up read (CWANT-04). A note is the wantlister's private context ("for my epic"); the guildwide list needs item + reason + owner + character, not the note. Confirm in discuss/plan.

## Sources

### Primary (HIGH confidence) — read in this repo this session
- `internal/backendsrv/migrations/00006_wantlist.sql` — wantlist_item DDL + the two partial-unique dedup indexes + the NULL-DISTINCT trap comment
- `internal/backendsrv/migrations/00009_character_assignment.sql` — confirms 00009 is taken (character_assignment), the watcher-untouched rationale, the `SELECT c.name`/LEFT JOIN idioms
- `internal/backendsrv/migrations/migrate_test.go:569` — `TestMigrate_00009_*` (the migrate-test pattern to mirror for 00010)
- `internal/backendsrv/store/wantlist.go` — `AddWantTx` / `ListOwnWants` / `RemoveOwnWantTx` / `ErrDuplicateWant` (extended-result-code dedup)
- `internal/backendsrv/store/assignment.go` — `ListMyAssignments` (LEFT JOIN character, SELECT c.name); confirms NO `IsCharAssignedTo` exists
- `internal/backendsrv/wantmatch/match.go` — `Hit` struct + `ForItem`/`ForName`/`scanHits` (where CharacterName joins in)
- `internal/backendsrv/ec/ec.go` + `ec/embed.go` — the embed build path + `notify.Send(Alert{DiscordUserID: hit.DiscordUserID})` (owner-targeting proof)
- `internal/backendsrv/notify/dm.go` — `UserChannelCreate(a.DiscordUserID)` (the DM target is the want owner)
- `internal/backendsrv/webadmin/wantlist.go` — `AddWantHandler` / `validWant` / `withTx` + audit (the handler to extend)
- `cmd/squirebot-server/main.go:342` — the wantlist + assignment route registrations (RequireSession)
- `web/src/lib/api.ts` (580, 832, 906) — `WantlistRow`, `addWant`, `MyCharacter`, `fetchMyCharacters`
- `web/src/lib/components/WantAddForm.svelte` / `WantlistPanel.svelte` — the add-form + list UI to extend
- `web/src/routes/wantlist/+page.svelte` — the page shell (per-user only — no guildwide surface)
- `.planning/ROADMAP.md` / `.planning/REQUIREMENTS.md` / `.planning/STATE.md` — phase scope, CWANT-01..06, locked decisions, P26/P27 status
- `.planning/config.json` — nyquist_validation:false, security_enforcement:true/L1
- `./CLAUDE.md` — schema doctrine, consolidated-views lock, EC-targets-owner

### Secondary (MEDIUM) — auto-memory (prior-session verified facts)
- web-tests-node-only-blind-to-dom; web-local-dev-cant-auth-against-prod; read-API-login-gated-since-P15; P26/P27 ship + deploy state

### Tertiary (LOW)
- None. No web search needed — this is an internal-codebase extension with no external library or API change.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; all reused libs verified in go.mod / imports
- Architecture (the five threading points + the two net-new constructions): HIGH — every surface read directly this session
- Migration shape (00010 + COALESCE dedup): HIGH — mirrors the verified 00006 pattern; the dedup-key decision is reasoned from the documented NULL-DISTINCT behavior (one product assumption A1 flagged)
- Security (the IDOR guard): HIGH — verified no guard exists; the control is well-specified
- Pitfalls: HIGH — drawn from the codebase's own documented traps + prior-phase memory

**Research date:** 2026-06-08
**Valid until:** 2026-07-08 (stable internal codebase; valid until the wantlist/assignment schema or the EC embed path changes)
