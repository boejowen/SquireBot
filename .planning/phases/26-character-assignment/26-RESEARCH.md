# Phase 26: Character Assignment - Research

**Researched:** 2026-06-08
**Domain:** Go (1.24) + SQLite (modernc, CGO-free) backend + SvelteKit static frontend — a new character→user assignment data layer, member self-service surface, and officer admin surface
**Confidence:** HIGH (every pattern verified in the live codebase; no external/unverifiable claims)

## Summary

Phase 26 adds an **assignment layer** keyed on `character.id`, sitting beside the untouched `owner_id` upload-provenance column. The codebase already contains every spine this phase needs: the member-CRUD endpoint shape (`webadmin/wantlist.go` / `account.go` — session-derived caller, `withTx`+`AppendAuditTx`, owner-scoped silent-no-op), the officer-gated shape (`webadmin/officers.go` + `webauth.RequireOfficer` + `store.IsOfficerTx` authorize-under-tx), the goose forward-only migration convention (`00005`/`00006`/`00007` — partial unique indexes, CHECK constraints, no-op `Down`), and the audit seam (`AppendAuditTx`). There is **zero new infrastructure** — this is a compose of existing patterns plus three new tables/columns and ~6 new endpoints.

The single load-bearing decision is the **`is_bank_toon` reconciliation (OPEN-2/OPEN-3)**. Investigation shows the existing `is_bank_toon` flag has FOUR consumers; only one (`compute.Bank` via `InventoryJoin(bankOnly)`) carries the "exactly one bank toon" assumption — and that assumption is **cosmetic, not structural**: the bank view is a CONSOLIDATED grid with a `Char` column (identical shape to the main `view`), so it already renders N characters cleanly. The single-bank invariant (MD-01) existed only to stop a *member-settable* flag from accidentally merging two unrelated members' inventories. Once the flag becomes **officer-only** and multiple guild banks are *intentional*, the invariant should be **relaxed**, not preserved.

**Primary recommendation:** Make "guild bank" the officer-only successor of `is_bank_toon` (move the flag officer-only, drop the single-bank demote, update the one doc comment + the seed). Add "guild bot" as a **new sibling column** `is_guild_bot` (no existing analog). Add `00009`: a `character_assignment` table (one assignee per char), an `assignment_request` table (pending→approved/denied), the new `is_guild_bot` column, and an idempotent auto-seed `INSERT … SELECT` from `owner.discord_user_id`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Normal characters have **exactly one assignee**. NOT many-to-many.
- **D-02:** Sharing is an **officer-only character designation**, not multi-assignment. An officer can mark a character a **guild bank** (stores guild money/items) or a **guild bot** (shared utility: rez, ports). Guild bank/bot characters are **shared guildwide, have NO assignee, and are not claimable** (exempt from the one-assignee rule). **Multiple** bank/bot chars are allowed.
- **D-03:** Assignment is a **separate layer** — a NEW table keyed on the character. `character.owner_id` is LEFT ALONE (watcher-upload provenance). Ingest first-sighting binding, eviction owner-floor, and `guild_code.owner_id` are **unaffected** by claims. Claiming never re-homes `owner_id`.
- **D-04:** `00009` **auto-seeds** the assignment table: each character whose `owner.discord_user_id` is non-NULL (linked via P17) is auto-assigned to that user. Legacy / NULL-owner characters start **unassigned**. No data loss; idempotent.
- **D-05:** Guild bank/bot designations are NOT auto-derived from data — officers set them. (OPEN-2 resolves whether existing `is_bank_toon` chars pre-seed the guild-bank designation.)
- **D-06:** Self-claim is **open and instant for an UNASSIGNED character** — any signed-in member can claim it.
- **D-07:** Claiming a character **already assigned to another member** files a **request** an **officer approves or denies** (not a silent take, not a hard block). Requester sees pending/approved/denied; can cancel a pending request.
- **D-08:** A member can **release/unclaim** a character they hold (returns it to unassigned).
- **D-09:** **Officers** can directly **assign / reassign / remove** any assignment at any time (no request), **approve/deny** requests, and **designate/clear** guild bank/bot status, from the admin panel.
- **D-10:** Every assignment change is recorded via `AppendAuditTx` with actor + character + action + time. V7 logging discipline (ids/action only, no PII beyond the already-keyed discord_user_id).

### Claude's Discretion
- The exact table/column shapes, request-state-machine encoding, and endpoint routing are the planner's to design (within D-01..D-10 + the OPEN items).
- Discovery UX (pick-from-list vs search) for finding a claimable character is the planner/UI's call — a list of claimable (unassigned) characters is the obvious default.

### Deferred Ideas (OUT OF SCOPE)
- **MYVIEW (Phase 27):** "my characters" inventory filter + single-char drill-down — depends on this phase, but is its own phase.
- **CWANT (Phase 28):** tagging wantlist items to a character — depends on this phase.
- **OPEN-1/2/3** are resolved in this research (see § is_bank_toon Reconciliation and § Recommended 00009 Schema).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ASSIGN-01 | Member sees "My characters" (chars assigned to them) | New `ListMyAssignments` store read keyed on `caller(ctx)` discord_user_id; new `GET /api/v1/assignments/mine` (RequireSession); a "My characters" panel mirroring `WatcherCodesPanel.svelte` load→list rhythm. |
| ASSIGN-02 | Member self-claims an unassigned char (incl. legacy/unlinked-owner char) | `ClaimCharTx`: INSERT into `character_assignment` if the char is currently unassigned (UNIQUE/PK on `character_id` enforces single-assignee); claim ignores `owner_id` entirely (D-03), so a legacy NULL-owner char is claimable. `POST /api/v1/assignments/claim` (RequireSession). |
| ASSIGN-03 | Member releases/unclaims a char they hold | `ReleaseCharTx`: owner-scoped DELETE WHERE character_id=? AND discord_user_id=caller; cross-actor = silent no-op (the wantlist Pitfall-3 pattern). `POST /api/v1/assignments/release`. |
| ASSIGN-04 | Officer assigns/reassigns/overrides any char from admin panel | `OfficerAssignTx` (authorize-under-tx via `IsOfficerTx`): upsert `character_assignment` (ON CONFLICT(character_id) DO UPDATE). `POST /api/v1/admin/assignments/assign` + `/remove` (RequireOfficer). |
| ASSIGN-05 | Model supports the shared-bank case | RESOLVED: shared = officer-only **designation** (`is_bank_toon`→guild bank, new `is_guild_bot`), NOT multi-assignment. Normal chars stay single-assignee (`character_id` PK). No schema rework needed to flip; designation + assignment are orthogonal columns/tables. |
| ASSIGN-06 | Every assignment change audited (actor, char, action, time) | `AppendAuditTx(ctx, tx, event, callerID, {character_id, …}, now)` composed in the SAME `withTx` as every mutation — claim/release/request-create/cancel/officer-assign/reassign/remove/approve/deny/designate. Existing infra; zero new audit code. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Assignment truth (who owns which char) | API / Backend (SQLite) | — | D-03: a server-truth table; never a client concept. The officer-gate and IDOR scoping are server-side (Layer-1 nav is UX only). |
| Self-claim / release | API / Backend | Frontend (member UI) | RequireSession-gated, caller from session; the UI is a thin form over the endpoint. |
| Contested-claim request workflow | API / Backend (state machine) | Frontend (status display) | The pending→approved/denied state machine + race resolution live in the store/tx; the UI only renders status + a cancel button. |
| Officer assign/reassign/designate | API / Backend (RequireOfficer + in-tx re-check) | Frontend (admin panel) | TOCTOU-safe authorize-under-tx (officers.go precedent); the `/admin` panel is officer-suppressed nav (not the gate). |
| Auto-seed at migration | Database (goose migration SQL) | — | Idempotent `INSERT … SELECT` runs once inside `00009`; goose records the version so re-run is a no-op. |
| Bank view (N guild banks) | API / Backend (`compute.Bank`) | Frontend (existing DataGrid) | Consolidated `Char`-column grid already handles N characters; relaxing the single-bank invariant is a backend-only change. |

## Standard Stack

No new dependencies. Everything is already in `go.mod` / `package.json` and proven live.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pressly/goose/v3` | (in go.mod) | Forward-only embedded migrations (`00009`) | The established migration runner (`migrations/embed.go`); dialect string is **`sqlite3`** (NOT the `sqlite` driver name — documented foot-gun). |
| `modernc.org/sqlite` | (in go.mod) | CGO-free SQLite driver | The whole-backend store; DSN already sets `_txlock=immediate` + `foreign_keys(ON)` + `busy_timeout` + `SetMaxOpenConns(1)`. |
| `net/http` (stdlib `ServeMux`) | Go 1.24 | Route registration in `cmd/squirebot-server/main.go` | Method+path patterns (`"POST /api/v1/…"`) wrapped in `webauth.RequireSession` / `RequireOfficer`. |
| SvelteKit (static adapter) + Svelte 5 runes | (in web/package.json) | `/account`-style member page + `/admin` officer panel | The existing component idiom (`$state`, `$derived`, `getContext` AuthGuard). |

### Supporting (existing helpers to reuse verbatim)
| Helper | File | Purpose |
|--------|------|---------|
| `caller(ctx)` | `webadmin/officers.go:58` | Session-derived discord_user_id (D-02 — never from body). |
| `withTx` | `webadmin/audit.go:88` | BEGIN IMMEDIATE → fn → COMMIT/ROLLBACK, panic-safe deferred rollback. |
| `AppendAuditTx` | `webadmin/audit.go:57` | One append-only `audit_log` row in the caller's tx (ASSIGN-06). |
| `writeJSON` / `writeJSONError` | `webadmin/officers.go:37,44` | `{"error":"code"}` body shape the frontend routes. |
| `nowUnix()` | `webadmin/officers.go:52` | Unix-epoch web-write timestamp. |
| `store.IsOfficerTx` | `store/admins.go:110` | Authorize-under-transaction officer re-check (WR-04 TOCTOU). |
| `webauth.RequireSession` / `RequireOfficer` | `webauth/session.go:194,217` | The two route gates. |
| `getJSON`/`postJSON` + `Unauthenticated`/`Forbidden` | `web/src/lib/api.ts` | Typed `credentials:'include'` fetch wrappers + the error classes AuthGuard routes on. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff / Why rejected |
|------------|-----------|------------------------|
| Single-assignee `character_assignment` table | Many-to-many join table | D-01 LOCKS single-assignee for normal chars; sharing is handled by designation (D-02), not multi-assignment. A join table would over-model and breach D-01. |
| New `assignment_request` table | Reuse `audit_log` as the request queue | The audit log is append-only and not a mutable state machine; a request has status transitions (pending→approved/denied) and needs a UNIQUE-pending guard. Separate table is correct. |
| New `is_guild_bot` column on `character` | A `character_role` lookup table | A boolean flag mirrors the existing `is_bank_toon` / `is_hidden` / `is_removed` pattern exactly; a role table is unjustified ceremony for two booleans. (If a 3rd+ role ever appears, refactor then.) |

## The `is_bank_toon` Reconciliation (OPEN-2 / OPEN-3) — THE load-bearing decision

### What exists today

`is_bank_toon` is an `INTEGER NOT NULL DEFAULT 0` column on `character` (`00001_init.sql:15`). Its FOUR consumers:

| # | Consumer | File | Assumption |
|---|----------|------|------------|
| 1 | **Bank view** — `compute.Bank` → `store.InventoryJoin(bankOnly=true)` | `compute/bank.go`, `store/readviews.go:165` | `WHERE c.is_bank_toon = 1`. Comment claims "Char is constant within it" (one bank toon). **This is the only single-bank consumer.** |
| 2 | **Coin entry** — `store.ListBankToons` / `SetCoinTx` / `GetCoin` | `store/coin.go:46,89` | `ListBankToons` already returns a **LIST** (`WHERE is_bank_toon=1 … ORDER BY name`) — **already N-safe**. Coin is written per bank-toon character. |
| 3 | **The single-bank demote invariant (MD-01)** | `store/charmeta.go:64` | `SetCharMetaTx` demotes every OTHER live bank toon when promoting one — enforced in-tx. The reason it exists: the flag is **member-settable via `/char-meta`**, so MD-01 stops a member accidentally flagging a 2nd char and silently merging two members' inventories in the bank view. |
| 4 | **Char-meta picker** — `CharsWithMeta` returns `IsBankToon` per char | `store/readviews.go:311`, `web CharMetaForm.svelte` | A checkbox in the member-settable char-meta form. |

### The key finding: multiple bank toons do NOT break the bank view

`compute.Bank` builds rows via `buildViewRows(joinRows, …)` — the **same** `Char`-column row shape as the main `view` tab. The bank query already `ORDER BY c.name, …`. So if two characters are flagged `is_bank_toon=1`, the bank grid simply shows both, grouped by `Char` — exactly like every other consolidated view. **The "Char is constant" comment is an artifact of the historical single-bank assumption, not a structural requirement.** The CONSOLIDATED-views architecture (CLAUDE.md LOCKED) is what makes N bank toons safe: the `Char` column disambiguates.

MD-01's demote existed purely because the flag was *member-settable* and an *accidental* second flag would mix unrelated inventories. Once the flag is **officer-only and multiple guild banks are intentional**, that risk disappears.

### RECOMMENDATION (resolves OPEN-2 + OPEN-3)

1. **"Guild bank" SUBSUMES `is_bank_toon`** (do NOT add a parallel `is_guild_bank` column — that would fork the bank view, the coin form, and 4 consumers). Keep the column name `is_bank_toon`; it simply *becomes* the "guild bank" designation.

2. **Move it officer-only (OPEN-3 = officer-only).** Change the route for char-meta's bank-toon setting:
   - **Minimal-churn option (RECOMMENDED):** split the bank-toon toggle OUT of the member `/char-meta` form into the officer `/admin` panel (a new "designate guild bank/bot" officer endpoint, `RequireOfficer`). Leave `class`/`level`/`race` member-settable on `/char-meta` (those are harmless metadata). This is a small `CharMetaForm.svelte` edit (remove the `isBankToon` checkbox) + `SetCharMetaTx` signature change (drop the `isBankToon` param) + a new officer `DesignateCharTx`.
   - The officer designation endpoint sets `is_bank_toon` AND the new `is_guild_bot` together (one "designate this char as guild bank / guild bot / neither" surface).

3. **Relax the single-bank invariant (MD-01).** Delete the demote block in `SetCharMetaTx` (`charmeta.go:64-71`) — or, more precisely, move the bank-toon write entirely to the new officer `DesignateCharTx`, which does NOT demote (multiple guild banks are allowed). Update the `compute.Bank` doc comment (`bank.go:25-31`) from "the single is_bank_toon character; Char is constant" to "all guild-bank characters; rows carry their Char (consolidated, multiple banks supported)." **`compute.Bank` query needs NO change** — `WHERE c.is_bank_toon = 1` already returns all of them; only the comment is stale.

4. **"Guild bot" is a BRAND-NEW flag** (no existing analog). Add `is_guild_bot INTEGER NOT NULL DEFAULT 0` to `character` in `00009`. It is NOT consumed by the bank view or coin (a guild bot stores utility, not coin) — in Phase 26 it's purely a designation flag + "exempt from assignment." Phase 27/28 may filter on it later.

5. **Designation clears/blocks assignment (D-02 "have NO assignee, not claimable").** When an officer designates a char as guild bank OR guild bot: in the same tx, DELETE any existing `character_assignment` row for it AND reject any pending `assignment_request` for it (mark denied or delete). Conversely, `ClaimCharTx` / `OfficerAssignTx` must refuse a char that is currently `is_bank_toon=1 OR is_guild_bot=1` (return a typed `ErrCharShared` → 409). This is the exemption rule, enforced server-side.

6. **OPEN-2 pre-seed:** YES — an existing `is_bank_toon=1` char is ALREADY a guild bank under the new model (same column), so no migration data work is needed for bank designation. The auto-seed (D-04) should **exclude** `is_bank_toon=1` chars from the assignment backfill (a guild bank has no assignee) — see the seed SQL below. There is currently at most one such char live (MD-01 held until now), but write the seed to exclude ALL `is_bank_toon=1` rows for forward-safety.

### Files this reconciliation touches
- `internal/backendsrv/store/charmeta.go` — drop the demote + the `isBankToon` param from `SetCharMetaTx` (or split bank-toon write to a new officer `DesignateCharTx`).
- `internal/backendsrv/compute/bank.go` — update the stale single-bank doc comment (no query change).
- `internal/backendsrv/webadmin/charmeta.go` + `web/src/lib/components/CharMetaForm.svelte` + `web/src/lib/charmeta.ts` — remove the member-facing bank-toon checkbox + its validation/payload field.
- `internal/backendsrv/webadmin/` — NEW officer `DesignateCharHandler` (RequireOfficer).
- `cmd/squirebot-server/main.go` — re-route: remove bank-toon from the member char-meta path; add the officer designate route.
- Existing tests: `store/charmeta_test.go` (the demote test), `compute/bank_test.go` (currently seeds exactly one bank toon — add a 2-bank case), `web charmeta.test.ts`.

## Recommended `00009` Schema (resolves OPEN-1)

Mirror the `00005`/`00006`/`00007` conventions exactly: forward-only, partial unique indexes (SQLite can't ALTER-ADD-UNIQUE), CHECK constraints as DB-level defense-in-depth, explicit no-op `Down`. **No `_meta.schema_version` write** — that's Apps-Script-era language; in the Go backend "schema v9" == "goose migration `00009` applied" (goose's own `goose_db_version` table is the version record; see § Schema Versioning below).

```sql
-- +goose Up
-- Phase 26 plan 26-01 (ASSIGN-01..06). The character→user assignment layer over
-- the untouched character.owner_id upload provenance (D-03): a single-assignee
-- table, a contested-claim request queue, the new is_guild_bot designation, and
-- the idempotent auto-seed from owner.discord_user_id (D-04). Forward-only;
-- 00001-00008 are SHIPPED and NOT edited.

-- D-02: "guild bot" is a NEW designation (no existing analog). "guild bank" reuses
-- the EXISTING is_bank_toon (now officer-only). A guild bank/bot char is shared,
-- has NO assignee, and is not claimable (exemption enforced in the store layer).
ALTER TABLE character ADD COLUMN is_guild_bot INTEGER NOT NULL DEFAULT 0;

-- D-01/D-03: exactly one assignee per NORMAL character, keyed on character_id (PK
-- ⇒ the single-assignee invariant is the schema, not store logic). discord_user_id
-- is the assignee (the PERSON, the wantlist/notify identity — NOT owner_id). The
-- claim/release/officer-assign mutators write here; owner_id is never touched.
CREATE TABLE character_assignment (
  character_id    INTEGER PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE,  -- one row per char ⇒ single assignee (D-01)
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  assigned_at     INTEGER NOT NULL,                 -- unix epoch secs (nowUnix())
  assigned_by     TEXT NOT NULL                     -- 'self' for self-claim, else the officer's discord_user_id (D-06/D-09); auto-seed uses 'migration'
);
CREATE INDEX character_assignment_user_idx ON character_assignment(discord_user_id);

-- D-07: a contested claim (char already assigned to someone else) files a request
-- an officer approves/denies. State machine: pending → approved | denied | cancelled.
-- The requester can cancel a pending request; the officer approve/deny resolves it.
CREATE TABLE assignment_request (
  id               INTEGER PRIMARY KEY,
  character_id     INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
  requester        TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  current_assignee TEXT,                            -- snapshot of who held it at request time (nullable: may be unassigned by approval time)
  status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','approved','denied','cancelled')),  -- DB-level defense-in-depth (the wantlist CHECK precedent)
  created_at       INTEGER NOT NULL,
  resolved_at      INTEGER,                         -- NULL until approve/deny/cancel
  resolved_by      TEXT                             -- the officer (approve/deny) or the requester (cancel); NULL while pending
);
CREATE INDEX assignment_request_char_idx ON assignment_request(character_id);
CREATE INDEX assignment_request_requester_idx ON assignment_request(requester);
-- At most ONE pending request per (character, requester) — SQLite treats NULL as
-- DISTINCT in a UNIQUE index, so scope the partial index to status='pending'
-- (the 00006 wantlist partial-unique precedent). A second pending request from the
-- same member for the same char collides; resolved rows never collide.
CREATE UNIQUE INDEX assignment_request_pending_uidx
  ON assignment_request(character_id, requester) WHERE status = 'pending';

-- D-04: idempotent auto-seed. Each character whose owner is linked (owner.discord_
-- user_id non-NULL via P17) AND is not already assigned AND is not a guild bank/bot
-- is auto-assigned to that user. INSERT OR IGNORE on the character_id PK makes a
-- re-run a no-op (goose already guards re-run, but OR IGNORE is belt-and-suspenders
-- and lets the same SELECT be safe if ever re-applied). Legacy/NULL-owner chars and
-- guild banks/bots are excluded ⇒ they start unassigned. assigned_by='migration'.
INSERT OR IGNORE INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
SELECT c.id, o.discord_user_id, strftime('%s','now'), 'migration'
  FROM character c
  JOIN owner o ON o.id = c.owner_id
 WHERE o.discord_user_id IS NOT NULL
   AND c.is_removed = 0
   AND c.is_bank_toon = 0
   AND c.is_guild_bot = 0;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00008): explicit no-op.
SELECT 1;
```

### Schema design notes for the planner
- **`character_assignment.character_id` is the PK** — this is what mechanically guarantees D-01 (single assignee). A reassign is `INSERT … ON CONFLICT(character_id) DO UPDATE SET discord_user_id=…, assigned_at=…, assigned_by=…`. A claim of an unassigned char is a plain `INSERT` that fails (or is gated by a pre-check) if a row already exists.
- **Identity is `discord_user_id`** (the web_user / person), NOT `owner_id` — consistent with `wantlist_item` (00006) and what Phase 27/28 will join on. D-03: never touch `owner_id`.
- **The exemption** (D-02 "guild banks/bots not claimable") is enforced in the store mutators (`ClaimCharTx` / `OfficerAssignTx` reject `is_bank_toon=1 OR is_guild_bot=1`), NOT in the schema — a CHECK across tables isn't expressible in SQLite. Document it as a store invariant with a test.
- **`assigned_by`** distinguishes self-claim (`'self'`), officer-assign (the officer's id), and the seed (`'migration'`) for audit/UI clarity.
- **Verify in a `migrate_test.go` case** (mirror `TestMigrate_00008_AddsECCursor`): the new column + two tables exist; the partial unique index `assignment_request_pending_uidx` exists; the CHECK rejects `status='bogus'`; the auto-seed assigned a linked-owner char and skipped a NULL-owner char + a bank toon; a second `RunMigrations` is a clean no-op.

## Schema Versioning — there is NO `_meta.schema_version` cell in the Go backend

CLAUDE.md's "schema_version write is the LAST migration step" and `WATCHER_MAX_SCHEMA_VERSION` language is **Apps-Script / Sheet-era** (the `_meta` tab). The Go + SQLite backend tracks version via **goose's own `goose_db_version` table** — one row per applied migration number. "Schema v9" simply means **migration `00009` has been applied** (goose version 9). There is:
- **No** `_meta` table to stamp, **no** ordering concern about "stamp version last" (goose records the version atomically when the migration's statements succeed).
- **No** `WatcherMaxSchemaVersion` concern — the watcher is untouched (it targets the ingest API, not these backend tables); `00006`/`00007`/`00008` all explicitly note this.
- The idempotency contract is goose's: `RunMigrations` re-run on an at-`00009` DB is a no-op (`TestRunMigrations_Idempotent` asserts the `goose_db_version` row count is unchanged).

**Implication for the planner:** do NOT add a `_meta.schema_version` write to `00009`. The phase deliverable "stamps schema v9" is satisfied by the file being named `00009_*.sql` and goose recording version 9 on apply. (The ROADMAP/STATE "schema v9" wording maps to goose version 9, not a cell write.)

## Architecture Patterns

### System Architecture Diagram (request flow)

```
                    Discord-OAuth session cookie (sb_session, Domain=squirebot.quest)
                                          │
            ┌─────────────────────────────┼──────────────────────────────┐
            │ MEMBER surface              │             OFFICER surface   │
            ▼ (RequireSession)            │             ▼ (RequireOfficer)│
  /account "My characters" panel          │   /admin assignment section   │
  ─ list my assignments                   │   ─ assign / reassign / remove │
  ─ claim unassigned char                 │   ─ approve / deny requests    │
  ─ release a char I hold                 │   ─ designate guild bank/bot   │
  ─ request a contested char / cancel     │                               │
            │                             │             │                 │
            ▼ api.ts postJSON/getJSON (credentials:include)                │
            ▼                             │             ▼                 │
   POST /api/v1/assignments/*   ──┐       │   POST /api/v1/admin/assignments/*
   GET  /api/v1/assignments/mine  │       │   GET  /api/v1/admin/assignments  (+ requests)
                                  ▼       │             ▼
                        webauth.RequireSession      webauth.RequireOfficer
                        caller(ctx)=session id      caller(ctx) + IsOfficer
                                  │                          │
                                  ▼ withTx (BEGIN IMMEDIATE) ▼
                        ┌──────────────────────────────────────────────┐
                        │ store mutator (single-SQL-path *Tx)           │
                        │  ClaimCharTx / ReleaseCharTx / RequestTx /    │
                        │  CancelRequestTx / OfficerAssignTx /          │
                        │  RemoveAssignTx / ApproveTx / DenyTx /        │
                        │  DesignateCharTx  (+ IsOfficerTx re-check on   │
                        │  every officer path — WR-04 TOCTOU)           │
                        │  + AppendAuditTx(...) in the SAME tx (ASSIGN-06)│
                        └──────────────────────────────────────────────┘
                                  │
                                  ▼  SQLite (modernc, maxconns=1, _txlock=immediate)
                        character_assignment · assignment_request · character.is_guild_bot
```

### Recommended file structure (additive)
```
internal/backendsrv/
├── migrations/00009_character_assignment.sql   # the migration (above)
├── store/assignment.go                         # ClaimCharTx, ReleaseCharTx, RequestTx,
│                                               #   CancelRequestTx, OfficerAssignTx,
│                                               #   RemoveAssignTx, ApproveRequestTx,
│                                               #   DenyRequestTx, DesignateCharTx,
│                                               #   ListMyAssignments, ListAllAssignments,
│                                               #   ListPendingRequests + typed errors
├── store/assignment_test.go
├── webadmin/assignment.go                      # member handlers (RequireSession)
├── webadmin/assignment_admin.go                # officer handlers (RequireOfficer)
└── webadmin/assignment_test.go
web/src/
├── lib/components/MyCharactersPanel.svelte      # mirrors WatcherCodesPanel rhythm
├── lib/components/AssignmentAdminPanel.svelte   # mirrors MonitorAdminPanel rhythm
├── lib/api.ts                                   # +typed fns/interfaces (additive)
└── routes/account/ (or a new /my-characters)    # member route; +/admin section
```

### Pattern 1: Member-CRUD handler (copy `wantlist.go` / `account.go` verbatim)
**What:** Session-derived owner, `withTx`+audit, owner-scoped silent-no-op, typed-error→HTTP map.
**When:** every member claim/release/request endpoint.
```go
// Source: internal/backendsrv/webadmin/wantlist.go:115-168 (AddWantHandler shape)
func ClaimCharHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
        ctx := r.Context()
        var req struct{ CharacterID int64 `json:"character_id"` }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
            writeJSONError(w, http.StatusBadRequest, "invalid_input"); return
        }
        callerID := caller(ctx) // D-02: identity from SESSION, never the body
        now := nowUnix()
        err := withTx(ctx, db, func(tx *sql.Tx) error {
            if e := store.ClaimCharTx(ctx, tx, req.CharacterID, callerID, now); e != nil {
                return e // ErrCharAlreadyAssigned→409, ErrCharShared→409, etc.
            }
            // ASSIGN-06: detail carries character_id ONLY (V7 — no PII)
            return AppendAuditTx(ctx, tx, "assignment_claim", callerID,
                map[string]any{"character_id": req.CharacterID}, now)
        })
        if err != nil { mapAssignErr(w, err); return }
        writeJSON(w, map[string]any{"claimed": true})
    }
}
```

### Pattern 2: Officer handler with authorize-under-transaction (copy `officers.go`)
**What:** `RequireOfficer` at the route + `IsOfficerTx` as the mutator's FIRST statement (TOCTOU-safe).
```go
// Source: internal/backendsrv/store/admins.go:269-289 (AddOfficerTx pattern)
func OfficerAssignTx(ctx context.Context, tx *sql.Tx, characterID int64, assignee, callerID string, now int64) error {
    ok, err := store.IsOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
    if err != nil { return err }
    if !ok { return store.ErrNotAuthorized } // → mapped 403 not_authorized
    // reject sharing-designated chars (D-02 exemption)
    // upsert: INSERT ... ON CONFLICT(character_id) DO UPDATE (reassign/override, D-09)
    ...
}
```

### Pattern 3: Owner-scoped silent-no-op (IDOR defense — copy the wantlist remove)
**What:** A release/cancel scoped to `WHERE character_id=? AND discord_user_id=caller` returns `affected==0` as a benign `{"released":false}`, never leaking another member's row.
```go
// Source: wantlist.go RemoveOwnWantHandler — cross-actor = silent no-op (removed:false)
```

### Pattern 4: Member self-service UI (copy `WatcherCodesPanel.svelte`)
**What:** `onMount→load()`, `$state` phases (loading/error/ready), `getContext<AuthGuard>` 401→LoginScreen, `ConfirmDialog` for destructive release, pure DOM-free helpers in a `<script module>` block for node-vitest testability.

### Anti-Patterns to Avoid
- **Trusting a `discord_user_id` / `owner` from the request body.** Identity is ALWAYS `caller(ctx)` (D-02). The body carries only the `character_id` (and, for officer-assign, the *target* assignee — which IS legitimately a body field on the officer path, validated against `web_user`).
- **A per-character view tab** for "my characters" — CLAUDE.md LOCKED (200-tab era reasoning carries: keep everything consolidated). Phase 27 does the filter client-side; Phase 26 creates no views at all.
- **Hand-rolling the single-assignee guard in store code** when the `character_id` PK already enforces it. Let the schema do the work; the store just maps the conflict to a typed error.
- **A parallel `is_guild_bank` column.** It would fork 4 consumers. Reuse `is_bank_toon`.
- **Adding a `_meta.schema_version` write to `00009`.** Wrong era; goose tracks the version.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Single-assignee enforcement | Application-level "check then insert" race guard | `character_id` PRIMARY KEY on `character_assignment` | The DB enforces it atomically; the store maps the conflict to a typed error. |
| One-pending-request-per-(char,member) | A SELECT-COUNT pre-check | Partial `CREATE UNIQUE INDEX … WHERE status='pending'` | The 00006 wantlist precedent; SQLite NULL-distinct rule handled. |
| Officer authorization | A re-implemented role check | `RequireOfficer` (route) + `store.IsOfficerTx` (in-tx) | WR-04 TOCTOU-safe; the officers.go precedent. |
| Audit trail | A new log table | `AppendAuditTx` into the existing `audit_log` | D-06: extend, don't fork. ASSIGN-06 is a compose. |
| Atomic write+audit | Manual BEGIN/COMMIT | `withTx` | Panic-safe deferred rollback; BEGIN IMMEDIATE. |
| Migration version tracking | A `_meta.schema_version` cell | goose `goose_db_version` | Already the system of record; idempotent re-run. |

**Key insight:** Every "hard" part of this phase (single-assignee, race-free requests, TOCTOU-safe officer gate, atomic audit, idempotent migration) is already solved by an existing pattern. The new code is glue + two tables + a column.

## Common Pitfalls

### Pitfall 1: IDOR on claim/release/request — trusting the body for identity
**What goes wrong:** A member crafts a request body with someone else's `discord_user_id` to claim/release on their behalf.
**Why:** Copy-pasting a handler but reading the actor from the body instead of the session.
**How to avoid:** ALWAYS `callerID := caller(ctx)`; the body carries only `character_id` (member paths) or the *target assignee* (officer paths, which are already RequireOfficer-gated and validate the target against `web_user`). Release/cancel are scoped `AND discord_user_id = caller` → silent no-op on a foreign row (`{"released":false}`).
**Warning signs:** any `req.DiscordUserID` field on a member endpoint; a release that affects rows it didn't own.

### Pitfall 2: Officer-gate treated as a frontend concern
**What goes wrong:** The `/admin` panel is hidden from non-officers in the nav, and someone assumes that IS the gate.
**Why:** The SettingsMenu suppresses the Admin link for non-officers (UX), which looks like enforcement.
**How to avoid:** Every officer endpoint is `RequireOfficer` AND its mutator re-checks `IsOfficerTx` in-tx. The nav suppression is "Layer-1 UX, never the boundary" (verbatim from SettingsMenu.svelte:189-194). Server is truth.

### Pitfall 3: The contested-claim race (two members request the same char; approving one)
**What goes wrong:** Two members file pending requests for char X; an officer approves member A; member B's stale pending request still shows "pending" or gets approved later and silently overwrites A.
**Why:** No reconciliation of sibling pending requests on approval.
**How to avoid:** On `ApproveRequestTx`: (1) authorize-under-tx; (2) re-read current state — if the char's assignment changed since the request, the `current_assignee` snapshot is stale; (3) set the approved request's assignment (ON CONFLICT upsert) AND, in the same tx, mark all OTHER pending requests for that `character_id` as `denied` (status transition) so the queue can't double-approve. The partial unique index stops a single member double-filing; this tx step stops cross-member double-approval. Add a test.

### Pitfall 4: Auto-seed not idempotent on re-run / re-apply
**What goes wrong:** A botched re-run inserts duplicate or wrong-owner assignments.
**Why:** Plain `INSERT` without conflict handling.
**How to avoid:** `INSERT OR IGNORE … SELECT` on the `character_id` PK (above). goose already guards re-run, but OR IGNORE makes the SELECT itself replay-safe. Exclude `is_bank_toon=1 OR is_guild_bot=1` and `is_removed=1`. Verify with a migrate test that seeds a linked-owner char, a NULL-owner char, and a bank toon, then asserts exactly the right rows.

### Pitfall 5: Breaking the bank view by relaxing the single-bank invariant carelessly
**What goes wrong:** Removing the `SetCharMetaTx` demote without checking the bank-view consumer, fearing a merge bug.
**Why:** The MD-01 comment is alarming ("silently mix two characters' inventories").
**How to avoid:** The merge "bug" was only a bug because the flag was *member-settable and accidental*. The consolidated bank view has a `Char` column and already groups N chars (it's the same `buildViewRows` as the main view). Multiple INTENTIONAL guild banks render correctly. The fix is: move the flag officer-only + drop the demote + update the stale comment + add a 2-bank `compute.Bank` test. Confirmed safe by reading `compute/bank.go` + `readviews.go:InventoryJoin`.

### Pitfall 6: Forgetting the designation↔assignment exemption is bidirectional
**What goes wrong:** A char gets designated a guild bank while it still has an assignee, or a guild-bank char gets claimed.
**Why:** The exemption (D-02) isn't expressible as a schema constraint across tables.
**How to avoid:** Enforce both directions in the store: `DesignateCharTx` clears any existing assignment + denies pending requests for that char in the same tx; `ClaimCharTx`/`OfficerAssignTx` reject `is_bank_toon=1 OR is_guild_bot=1` with a typed `ErrCharShared`→409. Two tests.

### Pitfall 7: Web tests are node-only — green ≠ works in the browser
**What goes wrong:** vitest passes but the panel crashes in a real browser (no jsdom, no @testing-library/svelte per the toolchain-install rule).
**How to avoid:** Put pure logic in `<script module>` helpers (the WatcherCodesPanel pattern) for node tests, then **browser-smoke on prod** (or a full local stack with a seeded `sb_session`) before calling it verified. (Memory: `web-tests-node-only-blind-to-dom`, `web-local-dev-cant-auth-against-prod`.)

## Code Examples

### Idempotent goose migration with partial-unique + CHECK + auto-seed
See the full `00009` block in § Recommended Schema. Sourced from `00006_wantlist.sql` (partial unique + CHECK), `00005_self_service_linking.sql` (ADD COLUMN + FK + partial index), `00007_notify.sql` (seeded rows + CHECK), all verified in-repo.

### Officer route registration (additive in main.go)
```go
// Source: cmd/squirebot-server/main.go:308-372 (RequireOfficer/RequireSession patterns)
// Member (RequireSession)
mux.Handle("GET  /api/v1/assignments/mine",    webauth.RequireSession(db, webadmin.ListMyAssignmentsHandler(db)))
mux.Handle("GET  /api/v1/assignments/claimable", webauth.RequireSession(db, webadmin.ClaimableHandler(db)))
mux.Handle("POST /api/v1/assignments/claim",   webauth.RequireSession(db, webadmin.ClaimCharHandler(db)))
mux.Handle("POST /api/v1/assignments/release", webauth.RequireSession(db, webadmin.ReleaseCharHandler(db)))
mux.Handle("POST /api/v1/assignments/request", webauth.RequireSession(db, webadmin.RequestCharHandler(db)))
mux.Handle("POST /api/v1/assignments/request/cancel", webauth.RequireSession(db, webadmin.CancelRequestHandler(db)))
// Officer (RequireOfficer)
mux.Handle("GET  /api/v1/admin/assignments",         webauth.RequireOfficer(db, webadmin.ListAllAssignmentsHandler(db)))
mux.Handle("POST /api/v1/admin/assignments/assign",  webauth.RequireOfficer(db, webadmin.OfficerAssignHandler(db)))
mux.Handle("POST /api/v1/admin/assignments/remove",  webauth.RequireOfficer(db, webadmin.OfficerRemoveAssignHandler(db)))
mux.Handle("POST /api/v1/admin/assignments/approve", webauth.RequireOfficer(db, webadmin.ApproveRequestHandler(db)))
mux.Handle("POST /api/v1/admin/assignments/deny",    webauth.RequireOfficer(db, webadmin.DenyRequestHandler(db)))
mux.Handle("POST /api/v1/admin/characters/designate", webauth.RequireOfficer(db, webadmin.DesignateCharHandler(db)))
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `_meta.schema_version` cell (Apps Script) | goose `goose_db_version` table | v2.0 (Phase 11, Go backend) | No version-cell write in `00009`; "v9" == migration 00009 applied. |
| Single member-settable bank toon (MD-01 demote) | Multiple officer-only guild banks | THIS phase (recommended) | Drop the demote; consolidated `Char`-column bank view already supports N. |
| `WatcherMaxSchemaVersion` gate on schema bumps | N/A — backend-only schema, watcher untouched | v2.0+ | No watcher concern; `00006/07/08` precedent. |

**Deprecated/outdated for this phase:**
- CLAUDE.md "`_meta.schema_version` write is the LAST migration step" + `WATCHER_MAX_SCHEMA_VERSION` — Sheet-era; does not apply to a backend-only goose migration. (The extend-only / idempotent spirit still applies; the *mechanism* is goose.)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Relaxing the single-bank invariant won't break any consumer | is_bank_toon Reconciliation, Pitfall 5 | LOW — verified by reading all 4 consumers; the bank view is a `Char`-column consolidated grid (`compute/bank.go` + `readviews.go`). The only "fix" needed beyond the demote removal is a doc-comment + a 2-bank test. If a hidden consumer assumes one bank toon, a 2-bank test catches it. |
| A2 | "Guild bot" has no existing analog and needs a new `is_guild_bot` column | Recommended Schema | LOW — grep for `guild_bot`/`is_guild_bot` returns zero code hits; confirmed new. |
| A3 | The existing `<=1` live bank toon means the seed's bank-toon exclusion affects ~0-1 rows today | Recommended Schema, Pitfall 4 | LOW — MD-01 held since P16, so at most one live `is_bank_toon=1` row exists. Excluding it from the assignment seed is correct (a guild bank has no assignee) and forward-safe. |
| A4 | Whether "My characters" lives at `/account`, `/char-meta`, or a NEW `/my-characters` route is a UI/discuss call | File structure, ASSIGN-01 | LOW — purely a nav/IA decision (SettingsMenu already links /account + /char-meta). The planner/UI-spec picks; no backend impact. |

**Note:** A1-A4 are LOW-risk design judgments, not unverified facts. No external/compliance/security assumptions exist in this research.

## Open Questions

1. **Where does "My characters" land in the nav?**
   - What we know: SettingsMenu already routes to /account ("Watcher codes") + /char-meta ("Character details") + /admin (officer). A "My characters" member surface fits beside them.
   - What's unclear: new `/my-characters` route vs a section on `/account` vs a new gear-menu link.
   - Recommendation: a NEW `/my-characters` route + a SettingsMenu link (cleaner separation from watcher-codes); confirm in the UI-spec. Backend is route-agnostic.

2. **Does the contested-claim approval auto-reassign, or just record the right to claim?**
   - What we know: D-07 says the officer "approves or denies"; D-09 says officers assign directly.
   - What's unclear: does an *approved request* immediately move the assignment (reassign to the requester), or does it just unblock the requester to claim?
   - Recommendation: approval = immediate reassignment (officer's approve IS the assign), denying sibling pendings in the same tx (Pitfall 3). Simpler + matches "officer approves the claim." Confirm at plan time.

3. **Should `DesignateCharTx` toggle bank/bot as a 3-state (bank | bot | neither) or two independent booleans?**
   - What we know: a char is plausibly a guild bank OR a guild bot, not both.
   - Recommendation: enforce mutual exclusion in the store (`is_bank_toon` and `is_guild_bot` not both 1) for clarity, exposed as a 3-way radio in the officer UI. Low stakes; planner decides.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | backend build/test | ✓ (project builds live) | 1.24 | — |
| goose v3 | `00009` migration | ✓ (in go.mod, runs on boot) | v3 | — |
| modernc SQLite | store | ✓ | (go.mod) | — |
| Node + vitest | web build/test | ✓ | (package.json) | — |
| Live prod (api.squirebot.quest) for browser-smoke | UI verification | ✓ (Hetzner VPS, ssh-agent) | — | full local stack w/ SQUIREBOT_COOKIE_INSECURE + seeded sb_session |

No missing dependencies. Deploy path (per memory + 26-CONTEXT): web build→tarball→atomic-swap + backend `scp` binary + `systemctl restart`; `goose.Up` applies `00009` on boot. NOT a watcher release.

## Security Domain

`security_enforcement` assumed enabled (no config override seen).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Discord-OAuth session cookie (`sb_session`, httpOnly+Secure+SameSite=Lax); `RequireSession`. No new auth. |
| V3 Session Management | yes | Existing opaque hashed session + rolling TTL (`webauth/session.go`). Unchanged. |
| V4 Access Control | **yes (core)** | Member endpoints owner-scoped (caller from session, silent-no-op on foreign rows — IDOR). Officer endpoints `RequireOfficer` + in-tx `IsOfficerTx` (TOCTOU). The designation/exemption is server-enforced. |
| V5 Input Validation | yes | `character_id` must be a positive int; officer-assign target validated against `web_user`; status enum guarded by DB CHECK + handler allow-list. Parameterized `?` only. |
| V6 Cryptography | no | No new crypto; no secrets minted/stored this phase. |
| V7 Logging | yes | `AppendAuditTx` detail carries `character_id`/action/actor ONLY — no PII beyond the already-keyed discord_user_id (D-10). Never log bodies. |

### Known Threat Patterns
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Claim/release on another member's behalf (body-supplied identity) | Spoofing / Elevation | Identity from session (`caller(ctx)`), never body; owner-scoped silent-no-op. |
| Non-officer hitting an officer endpoint directly (bypassing hidden nav) | Elevation | `RequireOfficer` + in-tx `IsOfficerTx`; nav suppression is UX only. |
| Just-demoted officer landing one final assign (TOCTOU) | Elevation | Authorize-under-transaction (officers.go / admins.go precedent). |
| Double-approval of competing requests | Tampering | Approve-tx denies sibling pendings; partial unique pending index. |
| SQL injection via character_id / assignee | Tampering | Parameterized `?` placeholders only (codebase-wide rule). |
| Audit-log forgery / repudiation | Repudiation | append-only `audit_log`, INSERT-only `AppendAuditTx`, atomic with the write. |

## Sources

### Primary (HIGH confidence — read in-session this session)
- `internal/backendsrv/migrations/{00001,00005,00006,00007,00008}_*.sql` + `embed.go` + `migrate_test.go` — migration conventions, partial-unique/CHECK patterns, goose versioning, idempotency tests.
- `internal/backendsrv/store/{charmeta.go,coin.go,admins.go,readviews.go}` — `is_bank_toon` consumers (all 4), single-bank invariant (MD-01), `IsOfficerTx` authorize-under-tx, `ListBankToons` (already N-safe).
- `internal/backendsrv/compute/bank.go` (+ `bank_test.go`, `gearcheck.go`, `spellcheck.go`) — the consolidated `Char`-column bank grid; confirms N bank toons render cleanly.
- `internal/backendsrv/webadmin/{wantlist.go,account.go,officers.go,audit.go}` — member-CRUD + officer + audit handler shapes.
- `internal/backendsrv/webauth/session.go` — `RequireSession`/`RequireOfficer`/`caller`/`UserFromContext` gates.
- `cmd/squirebot-server/main.go` — route registration patterns.
- `web/src/lib/components/{WatcherCodesPanel,SettingsMenu,CharMetaForm}.svelte` + `web/src/lib/api.ts` — frontend member/officer panel rhythm, AuthGuard routing, typed fetch.
- `.planning/phases/26-character-assignment/26-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`, `./CLAUDE.md` — locked decisions, requirements, scope.

### Secondary / Tertiary
- None. This phase required no external research — every claim is verified against the live codebase.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; all helpers verified in-repo.
- Architecture / schema: HIGH — mirrors 3 shipped migrations; PK/partial-index mechanics verified against 00005/00006.
- is_bank_toon reconciliation: HIGH — all 4 consumers read directly; the "N bank toons break the view" fear disproved by reading `compute.Bank` + `InventoryJoin`.
- Pitfalls: HIGH — every pitfall traces to a verified codebase precedent or memory note.

**Research date:** 2026-06-08
**Valid until:** 2026-07-08 (stable backend; the only churn risk is an interleaved phase touching `character_assignment` or `is_bank_toon` — none scheduled before P27/P28, which depend on this).
