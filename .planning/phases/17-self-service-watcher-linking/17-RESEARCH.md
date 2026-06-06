# Phase 17: Self-Service Watcher Linking - Research

**Researched:** 2026-06-01
**Domain:** Go + SQLite backend (session-scoped mint/list/revoke endpoints, FK-linked owner identity, ingest last-seen stamping) + SvelteKit frontend (`/account` page, show-once credential reveal)
**Confidence:** HIGH (every finding grounded in real repo files read this session; no training-data guesses)

## Summary

Phase 17 is a **composition** phase on a live system. Every primitive it needs already exists: `auth.MintCode` (token-gen + hash-only persist), `webauth.RequireSession`/`UserFromContext` (the login-only gate + caller identity), the `webadmin` handler pattern (`caller(ctx)` → `withTx` → store `*Tx` mutator → `AppendAuditTx` → `writeJSON`/`writeJSONError`), the goose forward-only migration discipline, and the frontend `EvictionForm`/`ConfirmDialog`/`StateBlock`/`AuthGate` primitives. The genuinely new work is: (1) a forward-only `00005` migration adding `owner.discord_user_id` (nullable, partial-unique, FK) and `guild_code.last_seen`; (2) a session-derived **resolve-or-create-owner** algorithm with the D-04 ambiguity guard; (3) three new login-only HTTP handlers (mint / list-own / revoke-own) plus their store funcs; (4) stamping `guild_code.last_seen` on the ingest hot path, which **requires the bearer guard to also return the matched `guild_code.id`** (today it returns only `owner_id`); (5) the D-05 owner-floor rewire to prefer the FK; (6) deleting the `mint-code` CLI arm + its two tests; and (7) the `/account` Svelte page.

The two highest-risk landmines, both confirmed by reading the code: **(a)** SQLite cannot `ALTER TABLE ... ADD COLUMN` a `UNIQUE` column — uniqueness must come from a separate `CREATE UNIQUE INDEX ... WHERE discord_user_id IS NOT NULL` partial index (this also correctly lets many owners stay NULL). **(b)** `auth.guard.resolveToken` returns `(ownerID int64, ok bool)` only — last-seen stamping needs the code-row id, so the guard signature/the ingest path must change to thread it through, and the stamp must be a **separate non-blocking UPDATE outside the atomic replace tx** to avoid write-amplifying the hot path.

**Primary recommendation:** Mirror `webadmin/officers.go` + `store/admins.go` exactly for the three new endpoints; add `owner.discord_user_id` via `ADD COLUMN` + partial unique index in `00005`; resolve-or-create owner in one `BEGIN IMMEDIATE` tx using the existing `TRIM(label) COLLATE NOCASE` bridge with a hard refuse on 2+/already-stamped; thread `guild_code.id` out of the bearer guard and stamp `last_seen` as a fire-and-forget UPDATE after the ingest 204.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Owner identity ↔ Discord linkage | API / Backend (store + migration) | — | The FK + resolve-or-create is a server authorization/data concern; the client never supplies an owner (D-02). |
| Mint a self-service code | API / Backend (`webadmin` handler + `auth.MintCode`) | Frontend (triggers it, shows once) | Owner derived from session server-side; plaintext crosses to the page exactly once. |
| List own active codes | API / Backend (store query) | Frontend (renders `#N`/created/last-seen) | Caller-scoped query; the page only displays. |
| Revoke own code | API / Backend (`disabled_at` UPDATE, caller-scoped) | Frontend (`ConfirmDialog`) | Authorization (own-code-only) is a server concern; confirm is UX. |
| Stamp `last_seen` on upload | API / Backend (ingest path) | — | Only the bearer path knows which code uploaded; pure server-side. |
| Owner-floor rewire to FK | API / Backend (`webadmin/eviction.go`) | — | Authorization-boundary logic. |
| Show-once credential reveal | Frontend (`/account` page) | — | Transient component state; never persisted, never re-fetched (LINK-04 invariant). |
| `/account` nav entry + gate | Frontend (`SiteShell` + layout `AuthGate`) | API (`RequireSession` is the real gate) | The hidden nav is UX; the backend gate is the boundary. |

## Standard Stack

No new dependencies. Everything is already in the repo and pinned by prior phases.

### Core (all EXISTING — reuse, do not add)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `modernc.org/sqlite` | (in go.mod) | Pure-Go SQLite driver (`sqlite`, NOT `sqlite3`) | The backend's single-writer store; driver name differs from goose's `sqlite3` dialect on purpose (`store/db.go`). [VERIFIED: internal/backendsrv/store/db.go] |
| goose | (in go.mod) | Forward-only migrations run on startup (`migrations.RunMigrations`) | All schema changes go through `migrations/0000N_*.sql`. [VERIFIED: migrations/migrate_test.go] |
| `crypto/rand` + `crypto/sha256` + `encoding/base64` | stdlib | Token gen (32B → base64 raw-url → sha256) | The `auth.MintCode` shape; never hand-roll, never `math/rand` (V6). [VERIFIED: auth/mint.go] |
| SvelteKit / Svelte 5.56 | (web/) | The static frontend | Runes (`$state`/`$derived`/`$props`), `getContext`, no SSR for data pages. [VERIFIED: web/src/lib/components/EvictionForm.svelte] |
| `@lucide/svelte` | (in web/package.json) | Icons (`copy`/`check`/`triangle-alert`/`trash-2`) | Already used by `StateBlock`/`EvictionForm`; no new icon dep (17-UI-SPEC). |

**Installation:** None. `go.mod` and `web/package.json` are unchanged for this phase (17-UI-SPEC § Registry Safety confirms "no new UI dependency").

## Architecture Patterns

### System Architecture Diagram

```
                          /account page (SvelteKit, login-gated by layout AuthGate)
                                   │  credentialed fetch (cookie rides apex→api subdomain)
                                   ▼
  ┌──────────────────────── api.squirebot.quest (Go net/http mux) ─────────────────────────┐
  │                                                                                          │
  │  POST /api/v1/account/codes      ─RequireSession→ MintOwnCodeHandler                     │
  │      caller(ctx)=discord_user_id ──► resolveOrCreateOwnerTx (D-03/D-04) ──► auth mint    │
  │                                       │ exactly-1 label match → stamp discord_user_id     │
  │                                       │ zero match            → INSERT owner+stamp        │
  │                                       │ 2+ / stamped-by-other → REFUSE + slog.Warn        │
  │                                       └─► INSERT guild_code(token_hash) ─► return plaintext│
  │                                              (plaintext: HTTP body ONCE, NEVER slog/log)   │
  │                                                                                          │
  │  GET  /api/v1/account/codes      ─RequireSession→ ListOwnCodesHandler                    │
  │      caller→owner(by discord_user_id FK) → SELECT active codes (#N, created, last_seen)  │
  │                                                                                          │
  │  POST /api/v1/account/codes/revoke ─RequireSession→ RevokeOwnCodeHandler                 │
  │      caller→owner → UPDATE guild_code SET disabled_at WHERE id=? AND owner_id=caller's    │
  │                                                                                          │
  │  POST /api/v1/ingest (bearer)    resolveToken → (ownerID, codeID, ok)  ◄── NEW codeID     │
  │      [bind+replace tx commits] ─► UPDATE guild_code SET last_seen=? WHERE id=codeID  ─────┤── separate, non-blocking
  │                                                                                          │   (after 204; outside replace tx)
  └──────────────────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼  SQLite (single writer, WAL, _txlock=immediate, FK ON)
        owner(.discord_user_id NULL UNIQUE-partial FK→web_user)  guild_code(.last_seen)
```

### Recommended Project Structure (where new code lands)
```
internal/backendsrv/
├── migrations/00005_self_service_linking.sql   # NEW: owner.discord_user_id + guild_code.last_seen
├── auth/mint.go                                 # ADD: MintCodeForOwnerTx (session-owner, plaintext returned, no stdout)
├── auth/guard.go                                # CHANGE: resolveToken returns (ownerID, codeID, ok)
├── store/linking.go                             # NEW: ResolveOrCreateOwnerByDiscordTx, ListOwnCodes, RevokeOwnCodeTx, StampCodeLastSeen
├── webadmin/account.go (or webaccount/)         # NEW: Mint/List/Revoke handlers (mirror officers.go)
├── webadmin/eviction.go                         # CHANGE: callerMayNotEvictFloor prefers owner.discord_user_id
└── ingest/handler.go                            # CHANGE: thread codeID, stamp last_seen post-commit
cmd/squirebot-server/
├── main.go                                      # CHANGE: register 3 routes; DELETE runMint + "mint-code" arm
└── main_test.go                                 # CHANGE: DELETE TestRun_MintDispatch{,_MissingOwner}
web/src/
├── routes/account/+page.svelte                  # NEW: the page (mirrors char-meta/+page.svelte shell)
├── lib/components/WatcherCodesPanel.svelte       # NEW: generate → show-once → list → revoke
├── lib/api.ts                                    # ADD: mintOwnCode/fetchOwnCodes/revokeOwnCode wrappers + types
└── lib/components/SiteShell.svelte              # CHANGE: add "Account" nav under session?.authenticated
```

### Pattern 1: The canonical webadmin handler shape (MIRROR THIS)
**What:** Every new mutating endpoint copies `OfficerAddHandler`'s structure.
**When to use:** Mint and Revoke handlers.
**Example:**
```go
// Source: internal/backendsrv/webadmin/officers.go (OfficerAddHandler)
func MintOwnCodeHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        ctx := r.Context()
        callerID := caller(ctx)          // webauth.UserFromContext — the session discord_user_id (D-02)
        now := nowUnix()
        var plaintext string
        err := withTx(ctx, db, func(tx *sql.Tx) error {
            ownerID, derr := store.ResolveOrCreateOwnerByDiscordTx(ctx, tx, callerID /*, username */)
            if derr != nil { return derr }      // ErrAmbiguousOwner → refuse (D-04)
            var merr error
            plaintext, merr = auth.MintCodeForOwnerTx(ctx, tx, ownerID) // returns plaintext, INSERTs hash
            if merr != nil { return merr }
            return AppendAuditTx(ctx, tx, "code_mint", callerID, map[string]any{"owner_id": ownerID}, now)
        })
        if err != nil { mapAccountErr(w, err); return }
        // V7: plaintext goes in the HTTP body EXACTLY ONCE, never slog'd.
        writeJSON(w, map[string]any{"code": plaintext})
    }
}
```
**Key shape facts (verified):** `caller(ctx)` = `webauth.UserFromContext` (officers.go:58); `withTx` is `BEGIN IMMEDIATE` + deferred-rollback (audit.go:88); `AppendAuditTx` writes `(at, event, detail, actor)` (audit.go:57); `writeJSON`/`writeJSONError` are package helpers (officers.go:37-48). The session derives identity — the request body carries **no owner** (D-02).

### Pattern 2: Resolve-or-create owner from session (D-03/D-04) — the new core algorithm
**What:** Map the caller's `discord_user_id` to an `owner.id`, stamping the FK on first link.
**Source bridge:** the existing `TRIM(label) = TRIM(?) COLLATE NOCASE` match (eviction.go:372) + the username from `web_user` (officers.go:218 `usernameOf`).
**Algorithm (run inside the mint tx, on the `BEGIN IMMEDIATE` snapshot):**
```
1. SELECT id FROM owner WHERE discord_user_id = ?(caller)          -- already linked?
     → exactly one row  → return it (subsequent codes attach here, D-03).
2. (not linked yet) resolve caller's username via web_user.
3. SELECT id FROM owner WHERE TRIM(label) = TRIM(?username) COLLATE NOCASE
                              AND discord_user_id IS NULL          -- unlinked label matches
     → exactly ONE match  → UPDATE owner SET discord_user_id=?(caller) WHERE id=? ; return it (D-04 adopt).
     → ZERO matches       → INSERT owner(label=username, discord_user_id=caller) ; return new id (D-04 new).
     → 2+ matches         → return ErrAmbiguousOwner (REFUSE + slog.Warn, D-04 guard).
4. ALSO refuse if any label-matching owner is already stamped with a DIFFERENT discord_user_id
   (mis-adoption guard — never silently attach to data owned by someone else).
```
**Critical race note:** Run steps 1-3 entirely inside the one `withTx` `BEGIN IMMEDIATE` write tx so the UPDATE/INSERT cannot interleave with a concurrent ingest's `BindCharacter` (which also INSERTs owners on first sighting — binding.go:61). Because the store is `SetMaxOpenConns(1)` + `_txlock=immediate` (db.go:61), the write is already serialized — but composing resolve+mint in ONE tx is what makes it atomic and replayable. The partial unique index (below) is the backstop: a racing double-stamp surfaces as a constraint error, not silent corruption.

### Pattern 3: last-seen stamping OUTSIDE the hot-path tx (D-07)
**What:** After a successful ingest, record which code uploaded.
**Why not inside the replace tx:** the ingest replace tx (handler.go:188 `bindAndReplace`) is the atomic full-snapshot replace — the load-bearing write. Folding a `guild_code` UPDATE into it adds a second write to every upload (write-amplification at ~50-150 writes/day is trivial, but the bigger issue is coupling an auth-bookkeeping write to the data-integrity tx). **Recommendation:** stamp it as a *separate* `db.Exec` AFTER the 204, best-effort (a failed stamp logs but does not fail the ingest — last_seen is advisory UI metadata, not data integrity).
**Coalescing:** Not warranted at this guild's volume (a watcher uploads on file-change, debounced 500ms; ~12 guildies). A plain `UPDATE guild_code SET last_seen=? WHERE id=?` per upload is fine. If ever needed, throttle to "only update if last_seen older than N seconds" — but that is premature now.
**Blocker this exposes:** `auth.resolveToken` (guard.go:69) returns `(ownerID int64, ok bool)` — it does **not** return the matched `guild_code.id`. To stamp, the guard must `SELECT id, owner_id, token_hash ...` and return the id. `ResolveToken`/`Auth.ResolveToken` (guard.go:41) and both call sites (ingest `handler.go:94`, whoami `ingest/whoami.go`) change signature. This is the single most invasive change in the phase — flag it for the planner as its own task with its own tests.

### Anti-Patterns to Avoid
- **`ALTER TABLE owner ADD COLUMN discord_user_id TEXT UNIQUE`** — SQLite rejects adding a UNIQUE (or PK, or non-constant-default) column via ALTER. Use `ADD COLUMN` (no UNIQUE) + a separate `CREATE UNIQUE INDEX`. [VERIFIED: SQLite ALTER TABLE limitation — see Pitfall 1]
- **Logging the plaintext code** anywhere (slog, fmt to a captured writer, the audit `detail` blob). The audit detail must carry `owner_id`/`code_id` only — never the token (V6/V7, mirrors `AppendAuditTx` "never raw bodies or secrets").
- **Letting the client send an owner/label.** Owner is session-derived only (D-02). The v1 `mint-code --owner <label>` free-text path is being deleted, not ported.
- **Per-character view tabs / any owner-supplied SQL fragment** — parameterized `?` only (V5), consistent with the whole store.
- **Trusting green vitest for the show-once DOM/clipboard behavior** — `web/` vitest is node-only with no `@testing-library/svelte` (MEMORY: web-tests-node-only). Code-review + browser-smoke the `/account` page; do not call it verified on unit tests alone.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token generation | A custom random/encoding scheme | `auth.MintCode`'s exact shape (32B `crypto/rand` → `base64.RawURLEncoding` → `sha256`) | Already audited (V6); hash-only at rest is the locked contract. |
| Tx + audit composition | Inline `BeginTx`/`Commit`/rollback | `webadmin.withTx` + `AppendAuditTx` | Deferred-rollback panic-safety (WR-03) + atomic write+audit already solved (audit.go). |
| Caller identity | Reading cookies/headers in the handler | `caller(ctx)` / `webauth.UserFromContext` | The gate is the only legitimate identity setter (session.go:124). |
| Session gate | A new auth check | `webauth.RequireSession(db, handler)` at the route | The login-only gate already exists and is the real boundary (D-09). |
| Error→HTTP mapping | Ad-hoc status codes | A `mapAccountErr` mirroring `mapOfficerErr` (officers.go:203) | Frontend routes off the `{"error":"code"}` shape via `classifyAdminError` (api.ts:517). |
| Confirm-before-commit revoke UI | A bespoke modal | The shared `ConfirmDialog` (reused by `EvictionForm`) | Focus-trap, Esc/backdrop dismiss, destructive styling already done. |
| Credentialed fetch + typed errors | A raw `fetch` | `getJSON`/`postJSON` in `api.ts` | `credentials:'include'` + `Unauthenticated`/`Forbidden` subclasses already wired to `AuthGate`. |
| Empty/loading/error states | New markup | `StateBlock` (add a `no-codes` kind) | The lifecycle primitive `EvictionForm` already uses. |

**Key insight:** This phase has near-zero novel infrastructure. The only genuinely new *logic* is the resolve-or-create-owner decision tree (D-04) and the show-once panel; everything else is copy-the-pattern.

## Runtime State Inventory

> This is a feature-addition phase, but it touches a LIVE system with ~11 already-minted codes and ~11 logged-in guildies. The migration + FK rewire interact with that live state.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **Live `owner` rows** keyed by free-text `label` (from the v2.0 CLI mint — owner label was the guildie name). **Live `guild_code` rows** (~11 active, `last_seen` will be NULL until each watcher next uploads). **Live `web_user` rows** for the ~11 guildies who logged in during the P15/P16 cutover. | The `00005` migration adds the columns NULL — existing owners stay unlinked until each guildie's first self-mint triggers resolve-or-create (D-03 adopts the existing owner by label). No backfill needed; the FK stamps lazily. `last_seen` NULL renders "never used yet" (correct — not an error, 17-UI-SPEC). |
| Live service config | None — no external service stores the renamed/new columns. | None. |
| OS-registered state | None. | None. |
| Secrets/env vars | None new. The deploy env (`SQUIREBOT_COOKIE_DOMAIN`, `SQUIREBOT_WEB_ORIGIN`, `DISCORD_*`) is unchanged; the new routes inherit the existing CORS + cookie wrap. | None — verified by reading `main.go` route wiring (the new routes register on the same mux under the same CORS wrap). |
| Build artifacts | The single `squirebot-server` binary is redeployed (drop-binary + restart runs `goose.Up` on startup, applying `00005`). The watcher is **NOT rebuilt** (HARD CONSTRAINT — no watcher change). | Standard backend redeploy (`docs/backend-deploy.md`); `goose.Up` is idempotent on startup (main.go:252). |

**Owner-floor live interaction:** The owner-floor (`app_config['owner_floor_discord_id']`) currently resolves the protected owner via the label bridge (eviction.go:351-385) and is documented as fail-OPEN (WR-05) when the bridge can't match. After D-05, once the maintainer self-mints (stamping their `owner.discord_user_id`), the floor resolves via the FK directly — closing WR-05 for that owner. **Verify on the box:** the maintainer should self-mint once post-deploy so their floor protection becomes FK-backed (otherwise it stays on the label bridge until they do).

## Common Pitfalls

### Pitfall 1: SQLite can't ADD a UNIQUE column
**What goes wrong:** `ALTER TABLE owner ADD COLUMN discord_user_id TEXT UNIQUE REFERENCES web_user(discord_user_id)` fails — SQLite forbids adding a column with a UNIQUE constraint (and forbids a non-constant default, which a fresh FK-checked column would need for existing rows).
**Why it happens:** ALTER ADD COLUMN in SQLite supports only column constraints that don't require rewriting/scanning existing rows; UNIQUE requires building an index over existing data.
**How to avoid:** Two statements in `00005`:
```sql
-- +goose Up
ALTER TABLE owner ADD COLUMN discord_user_id TEXT REFERENCES web_user(discord_user_id);
CREATE UNIQUE INDEX owner_discord_user_id_uidx
  ON owner(discord_user_id) WHERE discord_user_id IS NOT NULL;  -- partial: many NULLs allowed, one owner per Discord id
ALTER TABLE guild_code ADD COLUMN last_seen TEXT;               -- TEXT to match disabled_at/created_at (datetime('now') convention)

-- +goose Down
DROP INDEX owner_discord_user_id_uidx;
-- (column drops are best-effort / forward-only in practice — mirror 00004's Down comment)
```
**Verified conventions to mirror:** `owner.created_at` and `guild_code.created_at`/`disabled_at` are **TEXT** with `datetime('now')` defaults (00001_init.sql:5,54-55) — so `last_seen` should be **TEXT** stamped with `datetime('now')`, NOT epoch INTEGER. (Contrast: the 00004 web-side columns use epoch INTEGER. The split is real — `guild_code` is a 00001 table, so match 00001's TEXT/datetime convention for column-type consistency within that table.) The FK target `web_user(discord_user_id)` is a TEXT PRIMARY KEY (00004:12), so `owner.discord_user_id` is TEXT — types match.
**FK enforcement caveat:** the DSN sets `_pragma=foreign_keys(ON)` on every pooled connection (db.go:43) — so the FK IS enforced at runtime. But SQLite does **not** retroactively check existing rows when a column is added; existing `owner` rows get NULL `discord_user_id` (NULL is exempt from FK checks). New stamps must reference a real `web_user` row — the caller's `web_user` always exists (a session is only minted for a logged-in member), so the FK holds.
**Warning sign:** a goose migration error on startup ("Cannot add a UNIQUE column") = you wrote UNIQUE inline.

### Pitfall 2: The bearer guard doesn't expose the code id
**What goes wrong:** You try to stamp `last_seen` in the ingest handler but `ownerID, ok := h.guard.ResolveToken(...)` (handler.go:94) gives you no `guild_code.id`.
**Why it happens:** `resolveToken` (guard.go:77) selects `owner_id, token_hash` only.
**How to avoid:** Change `resolveToken` to `SELECT id, owner_id, token_hash`, return `(ownerID, codeID int64, ok bool)`; update `ResolveToken`/`Auth.ResolveToken` and the two call sites (ingest handler, whoami). Keep the constant-time compare. Treat this as a discrete task with its own guard_test.go updates.
**Warning sign:** trying to look up the code id by re-hashing the token in the handler — don't; the guard already did the hash-compare, thread the id out.

### Pitfall 3: Revoke that isn't caller-scoped (cross-owner revoke)
**What goes wrong:** `RevokeOwnCodeHandler` accepts a `code_id` and revokes it without checking it belongs to the caller's owner — a guildie could revoke another guildie's code.
**Why it happens:** The existing `auth.RevokeCode` (store.go:64) matches by `id OR label` with no owner scope (it's the ops CLI, intentionally broad).
**How to avoid:** The self-service revoke MUST be `UPDATE guild_code SET disabled_at=datetime('now') WHERE id=? AND owner_id=? AND disabled_at IS NULL`, where `owner_id` is resolved from the caller's session (never the request body). RowsAffected=0 → the code wasn't theirs (or already revoked) → 404/idempotent no-op, never an error that leaks existence. Do NOT reuse `auth.RevokeCode` for the web path.
**Warning sign:** the revoke store func takes only a code id, no owner id.

### Pitfall 4: Show-once plaintext leaking past its one viewing
**What goes wrong:** The plaintext gets persisted to localStorage, re-fetched, logged, or put in the audit detail — breaking the LINK-04 "hash-only at rest, shown once" invariant.
**How to avoid:** Plaintext lives ONLY in the HTTP response body (once) and in Svelte component state for the panel's lifetime. Never `localStorage`, never `{@html}` (use `{}` auto-escape), never re-request. The list endpoint returns `#N`/created/last-seen — never a code. Audit `detail` carries `owner_id`/`code_id` only.
**Warning sign:** any reference to the code variable after the panel dismisses; any `localStorage.setItem` near it.

### Pitfall 5: Frontend green tests that don't see the DOM
**What goes wrong:** 100% green vitest, but the clipboard copy or the show-once reveal is broken in a real browser (the P15 precedent: 165 green tests, 2 crashing BLOCKERs).
**Why it happens:** `web/` vitest is node-only; no `@testing-library/svelte` (toolchain-install rule).
**How to avoid:** Unit-test the *pure* helpers (relative-time formatter, the `#N` ordinal logic, error classification) and browser-smoke the page itself. The clipboard `navigator.clipboard.writeText` + `user-select:all` fallback must be manually verified.
**Warning sign:** a plan that says "verified" with only vitest evidence for the panel.

## Code Examples

### Stamp last-seen after the ingest commits (D-07)
```go
// Source: pattern derived from internal/backendsrv/ingest/handler.go:139-157
// AFTER bindAndReplace returns the 204 status, best-effort stamp (does NOT fail the ingest):
status, err := h.bindAndReplace(r, ownerID, env, rows)
if err != nil { /* existing 409/500 mapping */ return }
// codeID came from the (changed) guard. Separate write, outside the replace tx.
if _, serr := h.db.ExecContext(r.Context(),
    `UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?`, codeID); serr != nil {
    slog.Warn("stamp last_seen failed (non-fatal)", "code_id", codeID, "err", serr) // never the token (V7)
}
w.WriteHeader(status)
```

### List own codes, ordered for a stable #N (LINK-05)
```go
// Source: query pattern mirrors store/admins.go:188 (ListPromotableUsers) + eviction.go:56
// Caller-scoped; #N is the row's position ORDER BY created_at (stable per owner).
const q = `SELECT id, created_at, last_seen
            FROM guild_code
           WHERE owner_id = ? AND disabled_at IS NULL
           ORDER BY created_at ASC, id ASC`
// The handler assigns #N = index+1 over the active set (the UI shows #N, created, last_seen).
```

### Frontend api.ts wrappers (mirror the existing ones)
```ts
// Source: web/src/lib/api.ts (postJSON/getJSON already carry credentials + typed errors)
export interface OwnCode { id: number; ordinal: number; created_at: string; last_seen: string | null; }
export function fetchOwnCodes(f: typeof fetch = fetch): Promise<OwnCode[]> {
    return getJSON<OwnCode[]>('/api/v1/account/codes', f);            // login-only; 401 → AuthGate
}
export function mintOwnCode(f: typeof fetch = fetch): Promise<{ code: string }> {
    return postJSON<{ code: string }>('/api/v1/account/codes', {}, f); // owner derived server-side (D-02)
}
export function revokeOwnCode(id: number, f: typeof fetch = fetch): Promise<{ revoked: boolean }> {
    return postJSON<{ revoked: boolean }>('/api/v1/account/codes/revoke', { id }, f);
}
```

### Nav entry in SiteShell (member-visible, NOT officer-gated)
```svelte
<!-- Source: web/src/lib/components/SiteShell.svelte:54-62 (the char-meta-nav precedent) -->
{#if session?.authenticated}
    <a href="/account" class="char-meta-nav">Account</a>   <!-- reuse .char-meta-nav styling verbatim -->
    <a href="/char-meta" class="char-meta-nav">Character details</a>
    <SessionIndicator {session} />
{/if}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mint-code --owner <label>` CLI, plaintext to stdout, maintainer DMs it | Session-scoped `/account` mint, owner from Discord session, plaintext in HTTP body once | This phase (LINK-01/06) | The CLI arm + `runMint` + its two tests are DELETED; `revoke-code` CLI stays. |
| `owner.label == web_user.username` loose string bridge (WR-05 fail-open) | Real `owner.discord_user_id` FK, preferred in owner-floor resolution | This phase (LINK-02 / D-05) | Closes the WR-05 fail-open for linked owners; label bridge remains the fallback for not-yet-linked owners. |
| `guild_code` with no last-use signal | `guild_code.last_seen` stamped on ingest | This phase (D-07) | "which PC is this / is it dead?" answered without a device-name field (D-06). |

**Deprecated/outdated:**
- `auth.MintCode(db, ownerLabel)` (the free-text-owner, prints-to-stdout signature) is **not deleted** but is no longer the web path — `RestoreHandler` (eviction.go:292) still calls it on `*sql.DB` for the post-restore re-mint. **Do not break that call.** Add a NEW tx-based, session-owner function (`MintCodeForOwnerTx`) for the web handler rather than changing `MintCode`'s signature. (If you refactor, keep a `*sql.DB` + stdout-printing wrapper for the restore path, or update `RestoreHandler` too — flag the choice for the planner.)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A best-effort (non-fatal) `last_seen` stamp is acceptable — a failed stamp should not fail the upload | Pattern 3 / Pitfall 2 | If the project wants last_seen guaranteed, fold it into the replace tx instead (higher coupling). Low risk — last_seen is advisory UI per 17-UI-SPEC. |
| A2 | `last_seen` should be TEXT `datetime('now')` (matching 00001's `guild_code` columns), not epoch INTEGER | Pitfall 1 | If a planner prefers epoch INTEGER for frontend relative-time math, the column type differs from sibling columns. Either works; TEXT keeps `guild_code` internally consistent. The frontend formats whichever it gets. |
| A3 | `#N` is the 1-based index over the owner's active codes ordered by `created_at` (not a stored column) | Code Examples / 17-UI-SPEC D-06 | If a planner wants a stable-across-revokes ordinal, a persisted per-owner counter is needed. 17-UI-SPEC says "1-based, stable" — index-over-active is stable enough for the show-once-then-list flow; confirm during planning. |
| A4 | whoami-web does NOT need a new "link status" field — the `/account` page fetches its own codes on mount, so it knows link/code state directly | (resolves the CONTEXT § Discretion open item) | Low — if the nav needs a badge ("link your watcher!") for never-linked members, whoami-web could expose a bool later; not required for the page to function. |
| A5 | The three new routes are best placed in a new `webaccount` package OR added to `webadmin` (which already holds login-only handlers like coin/char-meta) | Project Structure | Naming/organization only; `webadmin` already mixes officer-only and login-only handlers, so adding there is consistent. No functional risk. |

## Open Questions

1. **Refactor `auth.MintCode` vs. add a sibling?**
   - What we know: `RestoreHandler` (eviction.go:292) calls `auth.MintCode(db, ownerLabel)` on a bare `*sql.DB`, AFTER its tx commits, and relies on the stdout print as the one-time disclosure (WR-01/WR-02).
   - What's unclear: whether to add `MintCodeForOwnerTx(ctx, tx, ownerID) (string, error)` (no stdout, returns plaintext) and leave `MintCode` for restore, or unify them.
   - Recommendation: **Add the sibling.** Keep `MintCode` untouched so the restore path's documented WR-01/WR-02 behavior is preserved; the web handler gets a clean tx-composable function that returns plaintext and never prints/logs. (D-02 explicitly drops the free-text owner — the new function takes an `ownerID`, not a label.)

2. **`guild_code.label` for self-minted codes (CONTEXT § Discretion).**
   - What we know: `MintCode` stores `label = ownerLabel` (mint.go:46). Self-minted codes have no free-text label.
   - Recommendation: store the resolved owner's label (or NULL). Since `#N`/created/last-seen identify the code (D-06), `label` is non-load-bearing here — NULL is cleanest. Confirm in planning; either is safe (the column is nullable, 00001:53).

3. **Does the maintainer need to self-mint once post-deploy to FK-back their floor protection?**
   - What we know: D-05 prefers the FK only when `owner.discord_user_id` is set; the maintainer's owner is currently unlinked.
   - Recommendation: yes — note it as a one-line deploy step (the maintainer visits `/account` and mints once, stamping their FK; their existing codes keep working — additive). Until then, floor protection stays on the (hardened) label bridge, which is the current live behavior anyway (no regression).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Live backend (`api.squirebot.quest`, Hetzner VPS) | Deploy of `00005` + new routes | ✓ (MEMORY: v2-backend-live) | — | — |
| `goose` startup migration | Applying `00005` on restart | ✓ (in binary) | — | — |
| SSH to the box (ssh-agent + passphrase key) | Deploy + maintainer-self-mint verification | ✓ (MEMORY: ops access) | — | — |
| Node toolchain for `web/` build | Building the `/account` page | assumed ✓ (P14/P15 built here) | — | If missing, STOP and wait for the user (MEMORY: feedback_toolchain_installs) |

**Missing dependencies with no fallback:** None identified — this is a redeploy of an existing live stack.

## Project Constraints (from CLAUDE.md / CONTEXT.md)

- **Forward-only goose migrations; never edit `00001`-`00004`.** New work is `00005_*.sql`. [CLAUDE.md / CONTEXT D-01/D-07]
- **Hash-only at rest; plaintext shown exactly once; never slog/log the plaintext (V6/V7).** [CLAUDE.md / CONTEXT LINK-04]
- **HARD CONSTRAINT: NO watcher change, NO Discord OAuth in the watcher.** The code stays a static reusable bearer token; onboarding is unchanged. [REQUIREMENTS / CONTEXT]
- **Server is the authorization boundary; `/account` gate is `webauth.RequireSession` (login-only, NOT officer-gated).** The hidden nav is UX only. [CONTEXT D-09]
- **Parameterized `?` placeholders only (V5); never interpolate user input into SQL.** [CLAUDE.md store convention]
- **Owner derived server-side from the session (D-02); client never supplies an owner/label.** [CONTEXT]
- **Consolidated view tabs only (project-wide) — irrelevant here but noted: no per-character anything.** [CLAUDE.md]
- **`web/` vitest is node-only — green ≠ works in browser.** Code-review/browser-smoke the page. [MEMORY: web-tests-node-only]

## Sources

### Primary (HIGH confidence — read this session)
- `internal/backendsrv/auth/mint.go`, `auth/store.go`, `auth/guard.go` — token-gen, upsertOwner, bearer resolve (no code-id returned).
- `internal/backendsrv/migrations/00001_init.sql`, `00004_web_auth.sql`, `migrate_test.go` — column types (`guild_code` TEXT/datetime vs web-side epoch INTEGER), goose Up/Down + table_info test convention.
- `internal/backendsrv/webadmin/officers.go`, `eviction.go`, `audit.go` — the canonical handler shape, `caller`/`withTx`/`AppendAuditTx`/`writeJSON`, `callerMayNotEvictFloor`/`ownerLabelOf` (the D-05 rewire target + label bridge).
- `internal/backendsrv/store/admins.go`, `binding.go`, `eviction.go` — store query/`*Tx` patterns, `TRIM/COLLATE NOCASE` bridge, owner upsert/bind, `SetOwnerFloor` placeholder-web_user precedent.
- `internal/backendsrv/webauth/session.go`, `handlers.go` (WhoamiWebHandler) — `RequireSession`/`UserFromContext`, whoami-web shape (no link field today).
- `internal/backendsrv/ingest/handler.go` — the ingest flow + where bearer resolves (the last-seen stamp insertion point).
- `internal/backendsrv/store/db.go` — DSN pragmas (`foreign_keys(ON)`, `_txlock=immediate`, maxconns=1).
- `cmd/squirebot-server/main.go`, `ownerfloor.go`, `main_test.go` — route wiring, `runMint`/`case "mint-code"` + the two mint tests to delete, `runRevoke`/`revoke-code` to keep, `splitFlagsAndPositionals` (shared helper — keep).
- `web/src/routes/char-meta/+page.svelte`, `routes/+layout.svelte`, `lib/components/{EvictionForm,AuthGate,SiteShell}.svelte`, `lib/api.ts`, `lib/auth.ts` — the page/nav/gate/clipboard/api-client patterns to mirror.
- `.planning/phases/17-self-service-watcher-linking/{17-CONTEXT.md,17-UI-SPEC.md}`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` — locked decisions D-01..D-10, the approved UI contract, LINK-01..06.

### Secondary (MEMORY — project facts)
- web-tests-node-only-blind-to-dom; v2-backend-live-and-ops-access; feedback_toolchain_installs.

### Tertiary (none)
- No web search needed — every claim is grounded in repo code.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all deps in-repo and version-pinned by prior phases; no additions.
- Architecture / handler patterns: HIGH — read every rewire target and the patterns to mirror line-by-line.
- Migration mechanics: HIGH — column-type conventions verified against 00001/00004; the SQLite UNIQUE-via-ALTER limitation is a well-established SQLite behavior, mitigated by the partial-index pattern.
- Ingest last-seen blocker (guard returns no code-id): HIGH — confirmed by reading guard.go + handler.go.
- last_seen column type (A2) and `#N` stability (A3): MEDIUM — both are safe either way; flagged as assumptions for planner confirmation.

**Research date:** 2026-06-01
**Valid until:** ~2026-07-01 (stable — live codebase, no fast-moving external deps; re-verify only if `auth`/`webadmin`/`ingest`/`migrations` change before planning).
