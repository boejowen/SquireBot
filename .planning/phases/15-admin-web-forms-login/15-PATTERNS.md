# Phase 15: Admin Web Forms + Login - Pattern Map

**Mapped:** 2026-05-30
**Files analyzed:** 31 (17 backend Go/SQL/CLI + 14 frontend Svelte/TS/route)
**Analogs found:** 31 / 31 (every new/modified file has a same-repo analog)

> **Source note:** `15-RESEARCH.md` does NOT exist in the phase directory (only `15-CONTEXT.md`, `15-DISCUSSION-LOG.md`, `15-UI-SPEC.md`). The new-file list below is derived from `15-CONTEXT.md` (D-01..D-12 + canonical_refs), `15-UI-SPEC.md` (component inventory + IA + form contracts), and the orchestrator's two-halves breakdown. If a `RESEARCH.md` with the `00004` DDL + Go skeletons appears later, reconcile its exact file names against the table below.
>
> **Repo reality (verified by reading, load-bearing for the planner):**
> - The P11/P12/P14 backend in `internal/backendsrv/**` is **fully implemented**, not skeletal. `runServe` (`cmd/squirebot-server/main.go:226-319`) is a complete server: opens the DB, runs `migrations.RunMigrations`, calls `scheduler.Start(ctx, db)`, builds a `net/http.ServeMux`, mounts ingest+whoami+the 4 read views+meta, wraps the whole mux in `readapi.CORS`, and runs `srv.ListenAndServe` in a goroutine with a `ctx.Done()` graceful-shutdown branch. P15 EDITS this function to mount the new routes + the archive job.
> - **There is NO `internal/backendsrv/server.go`** (the orchestrator brief named it speculatively). Server/scheduler/CORS wiring all live in `cmd/squirebot-server/main.go:runServe`.
> - The CLI switch arms are named **`runMint` / `runRevoke` / `runJobCmd`** (main.go:75-84), NOT `runMintCode`/`runRevokeCode`. The new subcommand follows their shape; call it `runSetOwnerFloor`.
> - The bearer-guard package is `internal/backendsrv/auth`, type `*auth.Auth` with `New(db)` + `ResolveToken(ctx, authHeader) (ownerID int64, ok bool)`. Mint/revoke are package funcs `auth.MintCode(db, label) (string, error)` / `auth.RevokeCode(db, idOrLabel) error` — NOT a `MintStore`/`TokenStore` struct. The hash idiom is `sha256.Sum256([]byte(code))` + `subtle.ConstantTimeCompare` (constant-time, iterate active rows), NOT `hex.EncodeToString`.
> - The read API does **NOT** use a shared JSON-envelope helper. ingest/whoami/readapi all call `http.Error(w, msg, status)` for errors and inline `json.NewEncoder(w).Encode(...)` for success. There is **no `WriteJSONError`/`ErrorEnvelope`** in the repo today. P15 should ADD a small JSON-error helper for the new web/form endpoints (the frontend wants `{error, message}` JSON, not `http.Error`'s text/plain) — flagged below as a NEW shared pattern, not a copy.
> - There is **no `methodGuard` middleware**; each handler does an inline `if r.Method != http.MethodX { http.Error(..., 405); return }` defensive check, with method routing done by the Go 1.22+ `"POST /api/v1/ingest"` ServeMux pattern string. P15 mirrors the inline-method-check idiom (or adds a small guard helper — planner's call).
> - The store mutator helper is the **inline `tx, _ := s.db.BeginTx(ctx, nil); defer tx.Rollback(); ...; tx.Commit()`** pattern (`store/replace.go:63-75`) — there is **no `withTx` wrapper method**. `BEGIN IMMEDIATE` is achieved via the `_txlock=immediate` DSN (`store/db.go:45`), so every `BeginTx` is already an immediate (write-lock-up-front) transaction — the TOCTOU-close mechanism D-06 needs is already the default; the executor just does the in-transaction re-authorization on that same `*sql.Tx`.
>
> **v1 apps-script files are the SEMANTIC ORACLE ONLY.** `apps-script/src/lib/admin.ts`, `triggers/showEvictionSidebar.ts`, `weeklyEvictionArchive.ts`, `showAdminMgmtSidebar.ts` define the *rules* to re-implement in Go/SQL. They are **NOT modified** — they retire in P16. Where they store in `_meta` JSON or use `LockService.getDocumentLock().tryLock(30000)`, the Go port stores in DB tables and uses the `_txlock=immediate` transaction.

---

## File Classification

### Backend (Go — hand-rolled `net/http` + `modernc.org/sqlite` + `goose`)

| New/Modified File | New/Mod | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|---------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00004_web_auth.sql` | NEW | migration | DDL | `migrations/00001_init.sql` + `00002_audit.sql` + `00003_enrich_columns.sql` | exact |
| `internal/backendsrv/webauth/oauth.go` (Discord authorize+callback, server-side code exchange) | NEW | handler/service | request-response (OAuth2 code flow) | `ingest/version.go` (unauth handler) + `auth/mint.go` (`crypto/rand` state) + `enrich/jobs/pigparse.go` (outbound fetch) | partial |
| `internal/backendsrv/webauth/session.go` (mint/lookup/destroy opaque session, sha256-hashed, TTL) | NEW | service | CRUD (session store) | `auth/mint.go` (`MintCode`) + `auth/guard.go` (sha256 resolve) | exact |
| `internal/backendsrv/webauth/guard.go` (cookie→session→ctx middleware; `requireOfficer`) | NEW | middleware | request-response | `auth/guard.go` (whole file — resolve + ctx pattern) | exact |
| `internal/backendsrv/webauth/membership.go` (Discord `guilds`-list check, D-02) | NEW | service | request-response (external API) | `enrich/pigparse.go` (outbound JSON GET) + `auth/guard.go` (fail-closed) | partial |
| `internal/backendsrv/webauth/whoami.go` (`{authenticated,isMember,isOfficer,username,avatar,discord_user_id}`) | NEW | handler | request-response | `ingest/whoami.go` (ctx/guard → JSON, whole file) | exact |
| `internal/backendsrv/webauth/jsonerr.go` (NEW shared `{error,message}` JSON envelope for web/form endpoints) | NEW | utility | request-response | none (repo uses `http.Error`); model on `ErrorEnvelope` convention | none (new) |
| `internal/backendsrv/store/webuser.go` (upsert web_user, list promotable users) | NEW | service (store method) | CRUD | `auth/mint.go:upsertOwner` (SELECT-then-INSERT) + `store/readviews.go` (read methods) | exact |
| `internal/backendsrv/store/admins.go` (guild_admins add/remove/list, owner-floor, authz-under-tx) | NEW | service (store method) | CRUD (transactional authz) | `store/replace.go` (BeginTx mutator) + **oracle** `apps-script/src/lib/admin.ts` | role-match (port semantics) |
| `internal/backendsrv/store/eviction.go` (cascade `is_removed`, grace_until, revoke code) | NEW | service (store method) | batch / transform (cascade write) | `store/replace.go` (BeginTx) + `auth/mint.go:RevokeCode` (disabled_at) + **oracle** `showEvictionSidebar.ts` | role-match (port semantics) |
| `internal/backendsrv/store/coin.go` (write plat/gold/silver/copper on `is_bank_toon` char) | NEW | service (store method) | CRUD | `store/replace.go` (BeginTx + UPDATE character tail) | exact |
| `internal/backendsrv/store/config.go` (owner-floor read/write — `_meta` row or app_config k/v) | NEW | service (store method) | CRUD (key/value) | `store/jobstate.go` (GetJobRun/SetJobRun k/v shape) + `_meta` in `00001` | role-match |
| `internal/backendsrv/store/audit.go` (append admin/eviction/coin rows to `audit_log`, in-tx) | NEW | service (store method) | event-driven (append-only log) | `00002_audit.sql` (table) + `store/binding.go` (in-tx audit insert) | role-match |
| `internal/backendsrv/forms/handlers.go` (POST eviction / admin add+remove / coin) | NEW | controller | request-response (write) | `ingest/handler.go` (whole file — guard→decode→tx→status) | exact |
| `internal/backendsrv/readapi/cors.go` | **MODIFY** | middleware | request-response | itself (add `Allow-Credentials: true` + `POST` to methods) | self |
| `internal/backendsrv/scheduler/jobs` → `enrich/jobs/evictionarchive.go` (new job `Run` fn) | NEW | job | batch (scheduled) | `enrich/jobs/pigparse.go` (`RunPigparse` shape) + **oracle** `weeklyEvictionArchive.ts` | role-match (port semantics) |
| `internal/backendsrv/scheduler/scheduler.go` (register the archive job in the registry) | **MODIFY** | job registry | batch | itself (the `registry := []*Job{...}` in `Start`, lines 116-131) | self |
| `cmd/squirebot-server/main.go` (`set-owner-floor` subcommand + mount webauth/forms/archive in `runServe`) | **MODIFY** | config/route | request-response | itself (`runMint`/`runRevoke`/`runJobCmd` arms + the `runServe` mux wiring) | self |

### Frontend (SvelteKit static, Svelte 5 runes, Tailwind v4 + P14 token system)

| New/Modified File | New/Mod | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|---------|------|-----------|----------------|---------------|
| `web/src/lib/components/AuthGate.svelte` | NEW | provider (layout gate) | request-response | `routes/+page.svelte` (onMount fetch → status ladder) + `StateBlock` | role-match |
| `web/src/lib/components/LoginScreen.svelte` | NEW | component | request-response (nav to OAuth) | `StateBlock.svelte` (centered panel) + `.retry` button (StateBlock:104-118) | role-match |
| `web/src/lib/components/NotMemberScreen.svelte` | NEW | component | request-response | `StateBlock.svelte` + `LoginScreen` | role-match |
| `web/src/lib/components/SessionIndicator.svelte` | NEW | component | request-response | `ThemePicker.svelte` (header-right control + label) | role-match |
| `web/src/lib/components/BankCoinForm.svelte` | NEW | component (form) | CRUD (write) | `ThemePicker.svelte` (`<select>`) + `SearchBox.svelte` (input) + `StateBlock` | role-match |
| `web/src/lib/components/EvictionForm.svelte` | NEW | component (form) | request-response (destructive write) | `+page.svelte` (load+state) + `ThemePicker` select + **oracle** `showEvictionSidebar.ts` | role-match |
| `web/src/lib/components/AdminMgmtForm.svelte` | NEW | component (form) | CRUD (write) | `ThemePicker` select + `StateBlock` + **oracle** `showAdminMgmtSidebar.ts` | role-match |
| `web/src/lib/components/ConfirmDialog.svelte` | NEW | component (modal) | event-driven | `StateBlock.svelte` (panel + `@lucide` icon) — no existing modal | partial |
| `web/src/lib/components/FormField.svelte` | NEW (optional) | component | n/a | `ThemePicker.svelte` (label+control rhythm) | role-match |
| `web/src/lib/components/SiteShell.svelte` | **MODIFY** | component (shell) | n/a | itself (add SessionIndicator + officer Admin tab) | self |
| `web/src/lib/components/StateBlock.svelte` | **MODIFY** | component | n/a | itself (extend `StateKind` union + copy) | self |
| `web/src/lib/api.ts` | **MODIFY** | service (client) | request-response | itself (`getJSON` → add `credentials:'include'` + write fns + 401/403) | self |
| `web/src/routes/+layout.svelte` | **MODIFY** | route (layout) | n/a | itself (wrap children in `<AuthGate>`) | self |
| `web/src/lib/index.ts` | **MODIFY** | config (barrel) | n/a | itself (it's an empty placeholder today) | self |
| `web/src/routes/bank-coin/+page.svelte` (+ `+page.ts`) | NEW | route | n/a | `routes/+page.svelte` + `routes/+page.ts` | exact |
| `web/src/routes/admin/+page.svelte` (+ `+page.ts`) | NEW | route | n/a | `routes/+page.svelte` + `routes/+page.ts` | exact |

---

## Pattern Assignments

### BACKEND

---

### `internal/backendsrv/migrations/00004_web_auth.sql` (migration, DDL)

**Analog:** `migrations/00001_init.sql`, `00002_audit.sql`, `00003_enrich_columns.sql`

**Goose header (copy verbatim).** Note: the existing migrations do **NOT** wrap statements in `-- +goose StatementBegin/End` — they use a bare `-- +goose Up` / `-- +goose Down`. SQLite DDL is single-statement, so bare directives are correct; only add `StatementBegin/End` if you write a multi-statement trigger/function (you won't). `embed.go` runs `goose.SetDialect("sqlite3")` (NOT `"sqlite"`) then `goose.Up(db, ".")` over `//go:embed *.sql` on every startup:
```sql
-- +goose Up
CREATE TABLE web_session ( ... );
ALTER TABLE character ADD COLUMN plat INTEGER;
-- ... (one statement per line; SQLite allows only ONE column per ALTER ADD COLUMN — see 00003 header)

-- +goose Down
DROP TABLE web_session;
-- ... reverse order
```

**Table-shape conventions to mirror** (from `00001_init.sql`): `id INTEGER PRIMARY KEY` (no `AUTOINCREMENT` — the repo omits it); timestamps as `TEXT NOT NULL DEFAULT (datetime('now'))`; lifecycle/soft-delete as a **nullable** timestamp where `NULL = active` (the `guild_code.disabled_at` idiom, line 54 — reuse exactly for session TTL + eviction grace); booleans as `INTEGER NOT NULL DEFAULT 0`; FKs as `INTEGER NOT NULL REFERENCES other(id)`; `UNIQUE` on natural keys; the hash column convention is `token_hash BLOB NOT NULL UNIQUE -- sha256(plaintext); 32 bytes` (line 52).

**Extend-only / coin + grace columns** (mirror `00003`'s nullable `ALTER TABLE ... ADD COLUMN`, one per line — D-11 coin is nullable on `character`):
```sql
ALTER TABLE character ADD COLUMN plat   INTEGER;
ALTER TABLE character ADD COLUMN gold   INTEGER;
ALTER TABLE character ADD COLUMN silver INTEGER;
ALTER TABLE character ADD COLUMN copper INTEGER;
ALTER TABLE character ADD COLUMN grace_until TEXT;   -- D-10: NULL = not in grace
ALTER TABLE character ADD COLUMN archived_at TEXT;   -- D-10: NULL = not archived
```

**New tables (shapes the planner finalizes; D-03/D-05/D-06 + Claude's-Discretion lines 50-51):**
- `web_user` — `discord_user_id TEXT PRIMARY KEY` (snowflake, the stable key, D-03), `username TEXT NOT NULL`, `avatar TEXT`, `created_at TEXT NOT NULL DEFAULT (datetime('now'))`, `last_login_at TEXT`. Upserted each login.
- `web_session` — opaque session, **store the HASH** (mirror `guild_code.token_hash`): `id INTEGER PRIMARY KEY`, `session_hash BLOB NOT NULL UNIQUE`, `discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id)`, `created_at TEXT NOT NULL DEFAULT (datetime('now'))`, `expires_at TEXT NOT NULL` (30-day rolling, D-05). Look up by `session_hash = ? AND expires_at > datetime('now')` (the `disabled_at IS NULL` pattern adapted to a TTL).
- `guild_admins` — officer allowlist keyed by snowflake (D-06): `discord_user_id TEXT PRIMARY KEY REFERENCES web_user(discord_user_id)`, `added_at TEXT NOT NULL DEFAULT (datetime('now'))`, `added_by TEXT`. (Discretion line 51 permits instead an `is_admin` column on `web_user`; the allowlist table mirrors v1's list-semantics more directly — recommend the table.)
- owner-floor: a row in a small `app_config` k/v table **or** reuse the existing `_meta`-style pattern. The repo's nearest k/v analog is `job_run` (`00003`, single-row-per-key). Recommend a 2-col `app_config(key TEXT PRIMARY KEY, value TEXT)` seeded by the CLI (D-08); mirrors v1 `workbook_owner_floor`.

**`audit_log` is REUSED, not re-created** (`00002_audit.sql`). Its current columns (`event, char_name, attempting_owner_id, current_owner_id, created_at`) are owner-id-shaped; P15 actors are Discord snowflakes, so **extend `audit_log` with nullable columns** (e.g. `ALTER TABLE audit_log ADD COLUMN actor TEXT; ADD COLUMN target TEXT; ADD COLUMN detail TEXT;`) rather than inventing a parallel log. Write new `event` values (`admin_add`, `admin_remove`, `eviction_commit`, `coin_set`). (CONTEXT line 86/112 explicitly says reuse/extend `audit_log`.)

**Schema-version:** P15 is extend-only (new tables + nullable columns), so a `_meta.schema_version` bump + watcher `WatcherMaxSchemaVersion` change is likely **NOT** required (the CLAUDE.md rule applies only to breaking changes). The planner confirms; if a bump is needed, make the `UPDATE _meta ... schema_version` the LAST statement (replay-safe).

---

### `internal/backendsrv/webauth/session.go` (service, CRUD — opaque session mint+lookup)

**Analog:** `internal/backendsrv/auth/mint.go` (`MintCode`) + `internal/backendsrv/auth/guard.go` (the sha256 resolve in `resolveToken`)

**The mint idiom — `crypto/rand` → encode → store hash, return plaintext once** (from `auth/mint.go:30-54`; for sessions the plaintext goes into the cookie, never to disk; use `crypto/rand`, NEVER `math/rand`):
```go
func MintCode(db *sql.DB, ownerLabel string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { // crypto/rand (V6)
		return "", fmt.Errorf("generate token entropy: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(raw) // plaintext, shown ONCE
	sum := sha256.Sum256([]byte(code))
	// ... INSERT ... (token_hash, ...) VALUES (sum[:], ...)  -- store ONLY the hash
	return code, nil
}
```
Session port: `MintSession(ctx, db, discordUserID) (rawCookieValue string, err error)` — 32 `crypto/rand` bytes → `base64.RawURLEncoding` → `sha256.Sum256` → `INSERT INTO web_session (session_hash, discord_user_id, expires_at) VALUES (?, ?, datetime('now','+30 day'))`. Return the plaintext to the `oauth.go` callback, which sets it as the cookie (D-05). Never log it (V7).

**The lookup idiom — hash → row, `sql.ErrNoRows` → not-found** (from `auth/guard.go:69-102`; the bearer guard iterates active rows with `subtle.ConstantTimeCompare` because there's no token id; sessions have a unique `session_hash`, so a direct `WHERE session_hash = ? AND expires_at > datetime('now')` lookup is correct and simpler):
```go
sum := sha256.Sum256([]byte(rawCookie))
var discordUserID string
err := db.QueryRowContext(ctx,
	`SELECT discord_user_id FROM web_session WHERE session_hash = ? AND expires_at > datetime('now')`,
	sum[:]).Scan(&discordUserID)
if errors.Is(err, sql.ErrNoRows) { return "", false }  // expired/unknown → not authed
```
Hash-only at rest (mirror `auth/mint.go` package doc lines 5-12). `DestroySession` = `DELETE FROM web_session WHERE session_hash = ?` (logout, D-05). Rolling TTL = on each successful lookup optionally `UPDATE web_session SET expires_at = datetime('now','+30 day')`.

---

### `internal/backendsrv/webauth/guard.go` (middleware — cookie→session→context + requireOfficer)

**Analog:** `internal/backendsrv/auth/guard.go` (whole file)

**Mirror the guard struct + resolve shape.** The bearer guard is `type Auth struct{ db *sql.DB }` with `New(db)` and `ResolveToken(ctx, authHeader) (ownerID int64, ok bool)` (lines 23-43). The web guard is a sibling: `type WebAuth struct{ db *sql.DB }` + `New(db)` + a `ResolveSession(ctx, rawCookie) (*Session, bool)` that returns the resolved identity + officer flag. **KEY DIFFERENCE:** the bearer guard reads `r.Header.Get("Authorization")` and checks the `"Bearer "` prefix (lines 45-74); the web guard reads the **cookie** `c, err := r.Cookie("squirebot_session")` (D-05's httpOnly+Secure+SameSite=Lax, `Domain=squirebot.quest`). Everything else (hash-then-resolve, fail-closed on miss, never log the token — V7) is identical.

**Context handoff (NEW — the repo's guards currently return values directly, not via ctx; ingest reads them inline).** Because P15 has many endpoints behind one middleware, stash the resolved session in `r.Context()` under a typed key (standard Go idiom; the repo doesn't yet do this but it's the clean pattern for middleware):
```go
type sessionCtxKey struct{}
var SessionContextKey = sessionCtxKey{}

func (a *WebAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("squirebot_session")
		if err != nil { writeJSONError(w, 401, "unauthorized", "no session"); return }
		sess, ok := a.ResolveSession(r.Context(), c.Value)
		if !ok { writeJSONError(w, 401, "unauthorized", "invalid session"); return }
		ctx := context.WithValue(r.Context(), SessionContextKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```
`requireOfficer` is **fail-closed** (mirror `auth/guard.go`'s "every failure returns not-authenticated" + v1 `requireAdminOrThrow`): empty/unknown session → reject; non-officer on an `/admin/*` route → 403 `not_authorized`. The DEFINITIVE officer re-check happens **inside the write transaction** in the store layer (D-06) — the middleware check is the cheap first gate, not the only gate.

---

### `internal/backendsrv/webauth/whoami.go` (handler — session introspection)

**Analog:** `internal/backendsrv/ingest/whoami.go` (whole file — `WhoamiHandler` struct + `ServeHTTP`)

Copy the struct + `ServeHTTP` shape (ingest/whoami.go:40-94), but the response keys are fixed by UI-SPEC line 121 and **unauthenticated is a 200, not a 401** (the bearer whoami 401s because the watcher must hold a token; the web whoami returns `{authenticated:false}` because "logged out" is a first-class UI state — UI-SPEC § IA steps 1-2):
```go
func (h *WhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sess, ok := r.Context().Value(webauth.SessionContextKey).(*webauth.Session)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true, "isMember": sess.IsMember, "isOfficer": sess.IsOfficer,
		"username": sess.Username, "avatar": sess.Avatar, "discord_user_id": sess.DiscordUserID,
	})
}
```
Reuse ingest/whoami.go's exact JSON-encode tail: `w.Header().Set("Content-Type","application/json"); w.WriteHeader(...); json.NewEncoder(w).Encode(...)` (lines 85-92). Mount this **outside** the session-required middleware (or have the middleware allow whoami through with `authenticated:false`), so a logged-out browser can poll it.

---

### `internal/backendsrv/webauth/oauth.go` (handler — Discord authorize + callback + logout)

**Analog:** handler shape from `ingest/version.go` (unauth, no-guard handler); `crypto/rand` state from `auth/mint.go:31-33`; outbound JSON GET from `enrich/jobs/pigparse.go:56-70` / `enrich/pigparse.go`.

- **`GET /auth/login`** → build the Discord authorize URL (scopes `identify guilds`, D-02), set a random `state` cookie (the `rand.Read`→`base64.RawURLEncoding` idiom), `http.Redirect`. Server-side only; the Discord **client secret comes from env/systemd** (D-04), never the bundle.
- **`GET /auth/callback`** → verify `state`, exchange the code **server-side** (`golang.org/x/oauth2` Discord endpoint), GET `https://discord.com/api/users/@me` + `/users/@me/guilds`, run `membership.go` (D-02), `UpsertWebUser` (D-03), `MintSession` (session.go), `Set-Cookie` (D-05 attributes), redirect to `/`. On membership fail → redirect to `/` with the session unset (AuthGate then shows NotMemberScreen via whoami `isMember:false`) OR mint a session flagged `isMember:false` — planner picks; UI-SPEC § IA needs whoami to distinguish authenticated-not-member.
- **`POST /auth/logout`** (or GET) → `DestroySession` + clear cookie, redirect to `/`.

**Hand-rolled per D-04 / the 11-01 spike verdict** — NO Discord SDK (UI-SPEC line 148). The outbound `http.Client` + `json.NewDecoder(resp.Body).Decode(...)` shape matches the existing enrich fetchers.

---

### `internal/backendsrv/webauth/jsonerr.go` (NEW shared helper) + `forms/handlers.go` (controller — write endpoints)

**Analog (controller):** `internal/backendsrv/ingest/handler.go` (the canonical write-endpoint flow, whole file).
**Note:** the repo today returns errors via `http.Error(w, msg, status)` (text/plain) and successes via inline `json.NewEncoder(w).Encode(...)`. The frontend's 401/403 routing + inline form errors want **JSON** error bodies, so P15 should ADD a tiny helper (this is the one genuinely-new shared backend pattern):
```go
// webauth/jsonerr.go (NEW)
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
```

**The request flow to mirror (from `ingest/handler.go:81-158`):** (1) inline method check (`if r.Method != http.MethodPost { http.Error(...,405) }`, line 82-85 — or rely on the `"POST /api/v1/..."` ServeMux pattern), (2) `r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)` (line 88, `maxBodyBytes = 1<<20`), (3) read the session from ctx (the P15 equivalent of the bearer guard at lines 94-99), (4) decode the JSON body, (5) ONE transaction that re-authorizes + writes + audits, (6) success status.

**Decode convention** (the repo uses a plain `json.NewDecoder(r.Body).Decode(&env)` in `ingest/envelope.go:79-86`; it deliberately does NOT set `DisallowUnknownFields` for forward-compat — for the form bodies, the planner may set it since these aren't watcher-versioned):
```go
var req EvictRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	writeJSONError(w, http.StatusBadRequest, "bad_request", "malformed JSON")
	return
}
```

**Auth-from-context + route registration** (P15 mux mirrors the `runServe` `mux.Handle("POST /api/v1/...", ...)` style):
```go
sess, ok := r.Context().Value(webauth.SessionContextKey).(*webauth.Session)
if !ok { writeJSONError(w, 401, "unauthorized", "no session"); return }
// coin handler: any valid session passes (D-12 — login-gated, NOT officer-gated)
// admin/evict handlers: pass sess.DiscordUserID as the CALLER to the store method,
//   which re-authorizes UNDER the transaction (D-06).
```
Error mapping per UI-SPEC line 234: `not_authorized` (403), `owner_floor_protected`, `lock_busy`, `bad_request` (400). Routes: `POST /api/v1/admin/evict`, `/api/v1/admin/officers/add`, `/api/v1/admin/officers/remove`, `/api/v1/coin`, plus the picker reads `GET /api/v1/admin/evictable`, `/api/v1/admin/promotable`, `/api/v1/bank-toons`.

---

### `internal/backendsrv/store/admins.go` + `store/eviction.go` + `store/coin.go` (store methods — transactional writes)

**Analog (structural):** `internal/backendsrv/store/replace.go` (`ReplaceInventory` — the canonical BeginTx mutator, lines 63-75) + `store/binding.go` (in-transaction audit insert).
**Analog (semantic oracle):** `apps-script/src/lib/admin.ts`, `triggers/showEvictionSidebar.ts` — PORT the rules, not the storage.

**The transaction pattern (copy from `store/replace.go:63-75`; there is NO `withTx` wrapper — this inline shape IS the pattern; `BeginTx` is already `BEGIN IMMEDIATE` via the `_txlock=immediate` DSN, store/db.go:45):**
```go
func (s *Store) CommitEviction(ctx context.Context, callerDiscordID, targetOwnerID string) (Result, error) {
	tx, err := s.db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE (write lock up front)
	if err != nil { return Result{}, fmt.Errorf("begin eviction tx: %w", err) }
	defer tx.Rollback() // no-op after Commit
	// 1) re-authorize the officer ON tx (D-06 TOCTOU close — see below)
	// 2) owner-floor protection
	// 3) the cascade writes
	// 4) append audit_log row ON tx
	return res, tx.Commit()
}
```
The `_txlock=immediate` DSN means the in-transaction re-authorization (`SELECT 1 FROM guild_admins WHERE discord_user_id=? ON tx`) and the mutation cannot interleave with a concurrent remove — this is the Go/SQL equivalent of v1's `LockService.getDocumentLock().tryLock(30000)` + re-check-inside-lock. No extra mechanism needed; just do the SELECT and the write on the same `*sql.Tx`.

**PORT: `requireAdmin` fail-closed + authorize-inside-the-transaction (from `admin.ts:93-99` + `:166-171`):**
```typescript
export function requireAdminOrThrow(email): void {           // fail-closed on empty
  if (!normalizeEmail(email) || !isAdmin(email)) throw new Error('not_authorized');
}
export function addAdmin(email, callerEmail) {
  // ... cheap validation before the lock ...
  lock.tryLock(30000);
  requireAdminOrThrow(callerEmail);   // <-- re-check UNDER the lock (WR-04 / TOCTOU)
  if (admins.indexOf(target) !== -1) return { added:false, alreadyExists:true };  // idempotent
  // ... write guild_admins + appendAdminLogEntry ...
}
```
Go port (`store/admins.go`): `requireOfficerTx(ctx, tx, callerDiscordID) error` = `SELECT 1 FROM guild_admins WHERE discord_user_id=?` ON the tx; the owner-floor is always implicitly an officer (mirror `getAdminList` always-includes-floor); empty/missing → `ErrNotOfficer`. `AddOfficer`/`RemoveOfficer` are **idempotent** (no-op if already in/absent) and **owner-floor-protected** (a peer cannot remove the floor — `admin.ts:222-229`; self-removal of floor allowed, floor config row NOT updated — the documented orphan-pointer rule, `admin.ts:241-245`). Append an `audit_log` row inside the same tx (D-06).

**PORT: eviction cascade + grace + code-revoke (from `showEvictionSidebar.ts:147-230`):**
```typescript
export function commitEviction(email): EvictionResult {
  requireAdminOrThrow(callerEmail);                          // re-check inside lock
  lock.tryLock(30000);
  // scan _char_owner; for each row matching this owner & not already removed:
  //   sheet.getRange(i+1, COL_IS_REMOVED).setValue(true);   // flip is_removed
  const graceUntil = new Date(Date.now() + GRACE_MS).toISOString();  // GRACE_MS = 30 days
  // append _meta.eviction_log entry {email, grace_until, chars[], reason:'evicted'}
  return { affected, graceUntil };
}
```
Go port (`store/eviction.go`): one `BeginTx` that (1) `requireOfficerTx`, (2) owner-floor-protects (a peer can't evict the floor's own guildie data — D-09), (3) `UPDATE character SET is_removed=1, grace_until=date('now','+30 day') WHERE owner_id=? AND is_removed=0`, (4) `UPDATE guild_code SET disabled_at=datetime('now') WHERE owner_id=? AND disabled_at IS NULL` (the D-10 "one clean app-controlled action" — reuse the EXACT `disabled_at` idiom from `auth/mint.go:RevokeCode`, lines 64-72), (5) append `audit_log`. Return `{affected, graceUntil}` — UI-SPEC success copy (line 227) interpolates both (v1 returns the same shape, `showEvictionSidebar.ts:142-145/226`). Reversible-during-grace (D-10) = a sibling method that un-sets `is_removed`+`grace_until` and re-mints a code.

**Coin write (`store/coin.go`):** plain `BeginTx`; `UPDATE character SET plat=?, gold=?, silver=?, copper=? WHERE id=? AND is_bank_toon=1` (D-11 — guard `is_bank_toon` server-side, not just in the form). Validation bounds (gold/silver/copper 0–999, plat ≥0) locked in UI-SPEC line 172 — enforce server-side too. Mirror the `UPDATE character SET ... WHERE id=?` tail already in `store/replace.go:116-120`.

---

### `internal/backendsrv/store/webuser.go` (store method — upsert + list) + `store/config.go` (owner-floor k/v)

**Analog:** `auth/mint.go:upsertOwner` (the SELECT-then-INSERT upsert, lines 36-55) + `store/readviews.go` (the read-method shape) + `store/jobstate.go` (k/v get/set for config).

`UpsertWebUser(ctx, discordUserID, username, avatar)` — SQLite supports `INSERT ... ON CONFLICT(discord_user_id) DO UPDATE SET username=excluded.username, avatar=excluded.avatar, last_login_at=datetime('now')` (cleaner than the v1 SELECT-then-INSERT; either is fine). `ListPromotableUsers(ctx)` (AdminMgmtForm picker, D-07) = `SELECT discord_user_id, username, avatar FROM web_user WHERE discord_user_id NOT IN (SELECT discord_user_id FROM guild_admins) ORDER BY username` — copy the `QueryContext → rows.Next()/Scan → rows.Err()` loop from `store/readviews.go` (e.g. `CharFreshness`, lines 388-415). `ListEvictableOwners` similarly. `config.go` `GetOwnerFloor`/`SetOwnerFloor` mirror `store/jobstate.go`'s single-key get/set.

---

### `internal/backendsrv/enrich/jobs/evictionarchive.go` (job `Run` fn) + `scheduler/scheduler.go` (register it)

**Analog (structural):** `internal/backendsrv/enrich/jobs/pigparse.go` (`RunPigparse(ctx, db, fetch)`, whole file) — but the archive job needs no `fetch`; it's DB-only.
**Analog (semantic oracle):** `apps-script/src/triggers/weeklyEvictionArchive.ts`.

**The job function shape** (mirror `RunPigparse`'s `func Run...(ctx, db) error` signature minus the fetcher; advance `job_run` on every outcome via `store.SetJobRun`, pigparse.go:88/168):
```go
const evictionArchiveJobName = "eviction_archive_weekly"
func RunEvictionArchive(ctx context.Context, db *sql.DB) error {
	s := store.NewStore(db)
	now := time.Now().UTC()
	n, err := s.ArchiveExpiredEvictions(ctx)   // the ported 30-day scan (store/eviction.go)
	if err != nil { _ = s.SetJobRun(ctx, evictionArchiveJobName, now, "error", err.Error()); return err }
	return s.SetJobRun(ctx, evictionArchiveJobName, now, "ok", fmt.Sprintf("archived=%d", n))
}
```

**Register it in the scheduler registry (EDIT `scheduler/scheduler.go:116-131` — add a third `*Job` next to pigparse/wiki):**
```go
registry := []*Job{
	{Name: "pigparse_daily", Due: duePigparse, Run: func(ctx) error { return jobs.RunPigparse(ctx, db, politefetch.Fetch) }},
	{Name: "wiki_weekly",    Due: dueWiki,     Run: func(ctx) error { return jobs.RunWiki(ctx, db, politefetch.Fetch) }},
	{Name: "eviction_archive_weekly", Due: dueWiki, Run: func(ctx) error { return jobs.RunEvictionArchive(ctx, db) }}, // NEW
}
```
The `Job` struct = `{Name string; Due func(last, now time.Time) bool; Run func(ctx) error; mu sync.Mutex}` (scheduler.go:66-71). Reuse `dueWiki` (Sunday-once) or add a `dueDaily`-style predicate (`duePigparse` is `>=24h`). Jobs run an immediate check pass on startup then every `checkInterval` (10 min); `last_run_at` advances after every run (advance-always). **PORT the 30-day scan (from `weeklyEvictionArchive.ts:36-85`):** `SELECT` `character` rows where `is_removed=1 AND grace_until <= date('now') AND archived_at IS NULL`, set `archived_at=datetime('now')` (idempotent — the `archived_at IS NULL` guard mirrors v1's `moveCharToArchive` idempotency). Discretion line 53 also permits lazy-on-read; the scheduled job mirrors v1 most directly.

---

### `internal/backendsrv/readapi/cors.go` (MODIFY — add credentials)

**Analog:** the file itself (`CORS`, lines 36-50). The file already pins the exact origin (line 38) and deliberately notes (lines 15-18) that omitting `Allow-Credentials` was a P14 choice to keep "the P15 credentialed upgrade a one-line change". Make exactly that change:
```go
w.Header().Set("Access-Control-Allow-Origin", allowOrigin) // exact origin (already there)
w.Header().Set("Access-Control-Allow-Credentials", "true") // <-- NEW (D-05): cookie rides cross-subdomain
w.Header().Set("Vary", "Origin")
w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS") // <-- add POST (was "GET, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
```
`Allow-Credentials: true` is incompatible with `Allow-Origin: *` — the existing exact-origin reflection is exactly why this is safe (the file's own comment, lines 12-18). Keep the OPTIONS-preflight 204 short-circuit (lines 44-47) BEFORE any handler. **D-01 read-gate wiring lives in `runServe`, not here** (see below) — `cors.go` stays a thin header wrapper.

---

### `cmd/squirebot-server/main.go` (MODIFY — `set-owner-floor` subcommand + mount new routes/job)

**Analog:** the file itself (`runMint` lines 95-121, `runRevoke` lines 131-158, the `run` switch lines 74-90, `runServe` lines 226-319).

**Add the subcommand arm (mirror the switch, lines 75-84):**
```go
switch args[0] {
case "mint-code":       return runMint(args[1:])
case "revoke-code":     return runRevoke(args[1:])
case "run-job":         return runJobCmd(args[1:])
case "set-owner-floor": return runSetOwnerFloor(args[1:])   // <-- NEW (D-08)
case "serve":           return runServe(args[1:])
}
```

**`runSetOwnerFloor` — copy `runMint`'s flag/db/exec skeleton (lines 95-121):**
```go
func runSetOwnerFloor(args []string) int {
	fs := flag.NewFlagSet("set-owner-floor", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
	discordID := fs.String("discord-id", "", "Discord user id (snowflake) of the maintainer (required)")
	if err := fs.Parse(args); err != nil { return 2 }
	if *discordID == "" { fmt.Fprintln(os.Stderr, "set-owner-floor: --discord-id is required"); return 2 }
	db, err := openMigratedDB(*dbPath)   // reuse the existing helper (lines 359-371)
	if err != nil { fmt.Fprintf(os.Stderr, "set-owner-floor: %v\n", err); return 1 }
	defer db.Close()
	// store.SetOwnerFloor: write the floor (app_config) AND seed it as the bootstrap
	// first officer (D-08 — replaces v1's onOpen/getOwner bootstrap, admin.ts:273-330).
	if err := store.NewStore(db).SetOwnerFloor(context.Background(), *discordID); err != nil {
		fmt.Fprintf(os.Stderr, "set-owner-floor: %v\n", err); return 1
	}
	return 0
}
```
Run once at deploy (D-08). The flag style (`-db` default `defaultDB`, required-flag → exit 2) mirrors the three existing subcommands exactly. The positional-vs-flag `splitFlagsAndPositionals` shim (lines 331-347) isn't needed here (only flags, no positional).

**Mount the new HTTP surface + job into `runServe` (EDIT lines 261-293).** `runServe` is fully implemented — extend its mux + the CORS wrap:
```go
scheduler.Start(ctx, db)   // already present — now also runs eviction_archive_weekly (registered in scheduler.go)

webGuard := webauth.New(db)
mux := http.NewServeMux()
mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))   // existing
mux.Handle("GET /api/v1/whoami", ingest.NewWhoami(auth.New(db), db)) // existing watcher whoami (bearer)
// --- P15 web-auth surface (NEW) ---
mux.Handle("GET /auth/login",     webauth.LoginHandler(webGuard, cfg))     // cfg = Discord client id/secret/redirect/guild from env
mux.Handle("GET /auth/callback",  webauth.CallbackHandler(webGuard, cfg))
mux.Handle("POST /auth/logout",   webauth.LogoutHandler(webGuard))
mux.Handle("GET /api/v1/me",      webGuard.Allow(webauth.WhoAmI()))        // 200 {authenticated:false} when no cookie
// --- P14 read views, NOW session-gated (D-01) ---
st := store.NewStore(db)
mux.Handle("GET /api/v1/meta",            webGuard.Middleware(readapi.NewMeta(st)))
mux.Handle("GET /api/v1/views/view",      webGuard.Middleware(readapi.NewViews(st, "view")))
// ... gear_check / spell_check / bank, each wrapped in webGuard.Middleware ...
// --- P15 write forms (NEW, session + per-endpoint officer re-check) ---
mux.Handle("POST /api/v1/coin",            webGuard.Middleware(forms.Coin(st)))           // D-12 login-only
mux.Handle("POST /api/v1/admin/evict",     webGuard.Middleware(forms.Evict(st)))          // officer (re-checked in tx)
mux.Handle("POST /api/v1/admin/officers/add",    webGuard.Middleware(forms.AddOfficer(st)))
mux.Handle("POST /api/v1/admin/officers/remove", webGuard.Middleware(forms.RemoveOfficer(st)))

srv := &http.Server{ Addr: *addr, Handler: readapi.CORS(*corsOrigin, mux) }  // CORS still outermost
```
`migrations.RunMigrations(db)` already runs first (line 247) — `00004` applies automatically on restart (deploy = drop binary + restart, CONTEXT line 111). Add `serve` flags (or env reads) for the Discord client id/secret + redirect URI + guild id (D-04 — secret server-side only, mirror the `defaultCORSOrigin` flag style at lines 230-231, but source the SECRET from env not a CLI flag so it isn't in `ps`). Ordering invariant (from cors.go comments + the OPTIONS short-circuit): **CORS outermost → session middleware → handler**; OPTIONS preflight 204s before the session check.

---

### FRONTEND

---

### `web/src/lib/api.ts` (MODIFY — auth-aware fetch + write calls + 401/403)

**Analog:** the file itself (the `getJSON` core, lines 111-136; the `ApiError` class, lines 24-31; the per-endpoint wrappers, lines 141-163).

**Add `credentials: 'include'` to the shared fetch (D-05 — the session cookie must ride to `api.squirebot.quest`) and surface 401/403:**
```ts
async function getJSON<T>(path: string, fetchFn = fetch): Promise<T> {
	const res = await fetchFn(`${API_BASE}${path}`, {
		method: 'GET',
		headers: { Accept: 'application/json' },
		credentials: 'include'          // <-- NEW (D-05)
	});
	if (res.status === 401) throw new ApiError('unauthorized', 401);  // AuthGate → LoginScreen
	if (res.status === 403) throw new ApiError('forbidden', 403);     // → NotMember / Officers-only
	if (!res.ok) throw new ApiError(`unexpected ${res.status} fetching ${path}`, res.status);
	// ... existing JSON-parse-with-ApiError-wrap (lines 131-135) ...
}
```
The existing `ApiError` already carries `.status` (line 25) — the 401/403 routing keys off that (UI-SPEC line 249), no new error classes strictly needed (planner may still add `UnauthorizedError`/`ForbiddenError` subclasses for clarity). **New functions** (mirror the one-liner wrapper style, lines 141-163): `fetchWhoAmI()` (GET `/api/v1/me` → the `{authenticated,isMember,isOfficer,username,avatar,discord_user_id}` shape — AuthGate's source of truth; this one tolerates 200-`authenticated:false` so it does NOT throw on logged-out); `saveCoin(charId, coin)`, `evictGuildie(ownerId)`, `addOfficer(discordId)`, `removeOfficer(discordId)` (each a `POST` with `{ method:'POST', credentials:'include', headers:{'Content-Type':'application/json'}, body: JSON.stringify(...) }`); picker reads `fetchPromotableUsers()`, `fetchEvictableOwners()`, `fetchBankToons()`. Keep the `API_BASE`/`env.PUBLIC_API_BASE` import (lines 16-21) and the snake_case typed-interface convention (lines 36-101).

---

### `web/src/lib/components/AuthGate.svelte` (NEW — layout-level gate)

**Analog:** `web/src/routes/+page.svelte` (the `onMount` async load → `status` state → `{#if}` ladder, lines 14-137) + `StateBlock.svelte`.

Mirror the load-and-branch shape, adapted to the 4 auth states (UI-SPEC § IA lines 120-124). Svelte 5 runes (`$props`/`$state`) + `onMount(async () => {...})` + the `{#if status}...{:else}` ladder are the exact P14 idioms (`+page.svelte`):
```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import LoginScreen from '$lib/components/LoginScreen.svelte';
	import NotMemberScreen from '$lib/components/NotMemberScreen.svelte';
	import { fetchWhoAmI } from '$lib/api';

	let { session = $bindable(), children } = $props();
	let state = $state<'loading' | 'login' | 'notmember' | 'ok'>('loading');

	onMount(async () => {
		const me = await fetchWhoAmI();
		if (!me.authenticated) state = 'login';
		else if (!me.isMember)  state = 'notmember';
		else { session = me; state = 'ok'; }
	});
</script>

{#if state === 'loading'}<StateBlock kind="auth-loading" />
{:else if state === 'login'}<LoginScreen />
{:else if state === 'notmember'}<NotMemberScreen />
{:else}{@render children?.()}{/if}
```
Expose `session` (a `$bindable` prop or a context store) so `SiteShell` can render `SessionIndicator` + the officer Admin tab. Wired in `+layout.svelte` (below).

---

### `web/src/lib/components/StateBlock.svelte` (MODIFY — extend StateKind union)

**Analog:** the file itself (the `module` block exports `StateKind`, lines 1-7; the `{#if kind === ...}` ladder; the `.state`/`.retry` styles).

**Extend the exported union (line 6)** — add the auth + form-lifecycle + empty-picker kinds (UI-SPEC line 47):
```ts
export type StateKind =
	| 'empty' | 'view-empty' | 'error' | 'loading' | 'no-results' | 'no-coin'   // existing (P14)
	| 'auth-loading' | 'form-saving' | 'form-success' | 'form-error'            // NEW
	| 'no-bank-toons' | 'no-evictable-guildies' | 'no-promotable-users'         // NEW empty states
	| 'officers-only';                                                          // direct-nav refusal (UI-SPEC line 214)
```
Add each new kind's branch + copy to the `{#if}` ladder — the EXACT strings are in UI-SPEC § Copywriting Contract (lines 213-231). Reuse the existing centered `.state` block + `CircleAlert` import (lines 10, 39) — or swap to the UI-SPEC glyphs (`shield-alert`, `triangle-alert`, `loader-circle`). The `role="status"`/`aria-live="polite"` (line 29) carries to the form-lifecycle states. The success/error use `--status-missing`/`--status-ok` per UI-SPEC § Color.

---

### `web/src/lib/components/SiteShell.svelte` (MODIFY — SessionIndicator + officer Admin tab)

**Analog:** the file itself (the `.shell-header` holds `<ThemePicker bind:theme />` at line 27; the wordmark at line 26; `$bindable` props at lines 18-21).

Add `<SessionIndicator {session} />` after `<ThemePicker bind:theme />` in `.shell-header` (line 27). The 4-tab view-nav currently lives in `+page.svelte` (lines 144-156), NOT in SiteShell — so the **officer "Admin" entry** is most naturally an `<a href="/admin">` added to that nav (or promoted into the shell). Render it **only when `session?.isOfficer`** (UI-SPEC lines 132/135 — Layer-1 UX hiding; the server is the real gate). Add a `/bank-coin` affordance (UI-SPEC line 131). Add `session` to the props alongside `theme`/`children` (line 18-21). The `.wordmark` style (lines 58-65) is reused by `LoginScreen`; the `.shell-header` flex layout is unchanged.

---

### `web/src/lib/components/LoginScreen.svelte` + `NotMemberScreen.svelte` (NEW — centered cards)

**Analog:** `StateBlock.svelte` (the centered flex `.state` block, `padding: 48px 16px`, lines 78-87) for layout; the `.retry` primary-button pattern (StateBlock lines 104-118 — accent fill + `--bg` text + uppercase Label) for the CTA; `SiteShell .wordmark` (lines 58-65) for the wordmark.

`LoginScreen`: centered `--panel` card (max-width ~420px, UI-SPEC line 146), wordmark (Display 28px — reuse the `.wordmark` rule), purpose line (copy UI-SPEC line 208), "Sign in with Discord" `<a href="{API_BASE}/auth/login">` styled as the accent primary button + `@lucide/svelte/icons/log-in` glyph (UI-SPEC lines 147-148 — it NAVIGATES to the server OAuth start, not a fetch), footnote (line 209). In-flight → `loader-circle` + "Redirecting…". `NotMemberScreen`: same card; heading + body (copy lines 211-212), `shield-alert` in `--status-missing`, "Sign in as someone else" action. Both honor `prefers-reduced-motion` (global rule in `app.css`; StateBlock's `@media (prefers-reduced-motion)` at line 148 is the precedent).

---

### `web/src/lib/components/SessionIndicator.svelte` (NEW — header avatar + sign-out menu)

**Analog:** `ThemePicker.svelte` (header-right control, the `.label` 13px Label treatment lines 36-43, the `outline: 2px solid var(--accent)` focus ring lines 55-58).

Discord-CDN avatar `<img alt={username}>` (UI-SPEC line 246) with a `@lucide/svelte/icons/user` fallback + username (Label 13px, copy ThemePicker `.label`). Officer `shield` badge in `--accent` if `session.isOfficer` (UI-SPEC line 157). Click → small menu with "Sign out" (`log-out` glyph) → POST `{API_BASE}/auth/logout` then redirect to `/` (or an `<a>`). 44px min target; menu dismisses on Esc / outside-click / selection (UI-SPEC line 159).

---

### `web/src/lib/components/BankCoinForm.svelte` (NEW — coin entry, any member)

**Analog:** `ThemePicker.svelte` (the `<label><span class="label">…</span><select>` structure + control CSS, whole file) + `SearchBox.svelte` (the `<input>` field styling lines 81-96) + `+page.svelte` (onMount load + StateBlock states).

Native `<select>` of `is_bank_toon` chars (copy ThemePicker's `<select>` markup + CSS, lines 21-58); if the char has coin, pre-fill the 4 inputs (UI-SPEC line 170). Four `<input type="number" inputmode="numeric">` (Platinum/Gold/Silver/Copper, `font-variant-numeric: tabular-nums` per UI-SPEC line 85); validation plat≥0, g/s/c 0–999 (UI-SPEC line 172) with inline `--status-missing` errors. "Save coin" accent primary (the `.retry` pattern, StateBlock lines 104-118); states via `StateBlock kind="form-saving|form-success|form-error|no-bank-toons"`. **No ConfirmDialog** (non-destructive, D-12). Reuse the 44px-min-height + accent-focus-ring field conventions (ThemePicker lines 44-58 / SearchBox lines 81-96). Calls `saveCoin` in `api.ts`.

---

### `web/src/lib/components/EvictionForm.svelte` (NEW — officer, destructive) — ports `showEvictionSidebar.ts`

**Analog (structure):** `+page.svelte` (load+state ladder) + `ThemePicker` select + `ConfirmDialog` (below).
**Analog (semantic oracle):** `apps-script/src/triggers/showEvictionSidebar.ts` (the preview→confirm→commit JS, lines 299-366 in `SIDEBAR_BODY`).

Guildie `<select>` (copy ThemePicker select; v1's `<select id="emailSel"><option value="">Choose…</option>`, sidebar line 294). On select → **preview block** (mirror v1 `onEmailChange`/`previewEviction`, sidebar lines 313-339): "Characters affected (N):" + the cascade list tinted `--status-missing`, grace line "Grace expires: <date> (30 days from today).", and the D-10 consequence callout (`triangle-alert` + the exact copy UI-SPEC line 224). Owner-floor block (UI-SPEC line 181). "Evict guildie" opens `ConfirmDialog` (destructive); only dialog-confirm calls `evictGuildie(ownerId)` (v1 used `window.confirm`, sidebar line 344 — P15 uses the themed dialog). Success/error copy interpolates `{affected}`/`{graceUntil}` (UI-SPEC line 227 — the store method returns both, mirroring v1's `EvictionResult` return). All interpolated names via plain `{}` (auto-escape — UI-SPEC line 238, ports v1's `escapeHtml`, sidebar line 300).

---

### `web/src/lib/components/AdminMgmtForm.svelte` (NEW — officer) — ports `showAdminMgmtSidebar.ts`

**Analog (structure):** `ThemePicker` select + `StateBlock` + `ConfirmDialog`.
**Analog (semantic oracle):** `apps-script/src/triggers/showAdminMgmtSidebar.ts` (the `renderList`/`onAdd`/`onRemove` JS, lines 213-265, and the `routeError` map, lines 205-211).

Officers list (mirror v1 `renderList`, lines 213-237): each row = avatar+username + `(owner)` annotation on the floor; **Remove suppressed on the floor row when caller is a peer** (`showRemove = !isFloor || (callerEmail === floor)`, line 223 — Layer-1 UX; server re-checks). Heading "Current officers (N):" (line 216). Promote = `<select>` of `fetchPromotableUsers()` (D-07 — pick, don't type; copy ThemePicker select — REPLACES v1's free-text `<input id="addInput">`, line 187) + "Add officer"; empty → `StateBlock kind="no-promotable-users"`. Remove → `ConfirmDialog` (destructive; v1 used `window.confirm`, line 256). Map `owner_floor_protected`/`not_authorized`/`lock_busy` to the exact inline strings (UI-SPEC line 234, ported verbatim from v1 `routeError`, lines 207-210). Add/remove call `addOfficer`/`removeOfficer` in `api.ts` (v1 delegates to `libAddAdmin`/`libRemoveAdmin`, lines 99/110). All interpolation via plain `{}` (auto-escape; v1's `escapeHtml`/`escapeAttr`, lines 192-202 → UI-SPEC line 238).

---

### `web/src/lib/components/ConfirmDialog.svelte` (NEW — shared destructive modal)

**Analog:** `StateBlock.svelte` (the `--panel` block + `@lucide` icon + scoped-style structure) — **the repo has NO modal/dialog component**, so this is the partial-match new pattern; build it from StateBlock's surface conventions + the focus-trap/`role="dialog"`/backdrop spec in UI-SPEC lines 194-197.

Centered modal over a dimmed backdrop (`color-mix(in srgb, var(--bg) 70%, transparent)` — the `color-mix` idiom is already used in StateBlock's shimmer, lines 139-143); modal surface `--panel`, `--border`, `lg` padding, max-width ~440px. Heading (20px) + `triangle-alert` in `--status-missing` + consequence body + action row: **destructive confirm** (`--status-missing` fill / `--bg` text, with the Heavy-theme caveat UI-SPEC line 112) labeled with the specific action; neutral **Cancel** (text/ghost, `--text`). `role="dialog" aria-modal="true"` labelled by the heading; **focus Cancel by default** (never the destructive action — UI-SPEC line 197); Esc / backdrop-click / Cancel all dismiss with no side effect; restore focus to trigger on close; reduced-motion → instant. Reuse `outline: 2px solid var(--accent)` focus ring (ThemePicker line 56). This is the project's FIRST `--destructive` (= `--status-missing`) usage.

---

### `web/src/routes/bank-coin/+page.svelte` + `admin/+page.svelte` (+ their `+page.ts`) (NEW — routes)

**Analog:** `web/src/routes/+page.svelte` + `web/src/routes/+page.ts` (whole files).

`+page.ts` mirrors the existing `routes/+page.ts`: it's a one-liner. **CAUTION:** the root `routes/+page.ts` is `export const prerender = true;` but the **layout** `routes/+layout.ts` sets `ssr=false; prerender=false;` and a comment warns NOT to prerender data-driven routes (a cross-origin fetch during prerender fails — 14-RESEARCH anti-pattern). The new form routes ARE data-driven (they fetch on mount), so their `+page.ts` should **inherit the layout default** (omit the file, or set `export const prerender = false;`) — do NOT copy the root's `prerender = true`. `bank-coin/+page.svelte` renders `<BankCoinForm/>`. `admin/+page.svelte` renders the two officer sub-sections (`<EvictionForm/>` + `<AdminMgmtForm/>` as the two `.tab`-styled sub-tabs, reusing the `.tab`/`.tab.active` CSS from `+page.svelte` lines 204-231); a non-officer reaching `/admin` directly shows `StateBlock kind="officers-only"` (UI-SPEC line 138/214) — read `session.isOfficer` and branch (the server still rejects the data calls regardless — D-01). Both inherit `AuthGate` from the root layout, so they only mount post-auth.

---

### `web/src/routes/+layout.svelte` (MODIFY — wrap in AuthGate) + `web/src/lib/index.ts` (MODIFY — barrel)

**Analog:** the files themselves. `+layout.svelte` currently wraps `{@render children()}` in `<SiteShell bind:theme>` (lines 31-33). Wrap the children render in `<AuthGate>` and thread `session` so the shell shows the indicator + officer tab:
```svelte
<div class="theme-root" data-theme={theme} bind:this={rootEl}>
	<SiteShell bind:theme {session}>
		<AuthGate bind:session>
			{@render children()}
		</AuthGate>
	</SiteShell>
</div>
```
Add `let session = $state<Session | null>(null);` alongside the existing `theme` state (line 17). Keep the `applyTheme` `$effect` (lines 23-25) + `loadTheme()` seed (line 17) unchanged. `web/src/lib/index.ts` is currently an empty placeholder (just a comment) — optionally add `export { default as AuthGate } from './components/AuthGate.svelte';` etc., but the codebase imports components by direct `$lib/components/X.svelte` path (see `+page.svelte` lines 15-18), so the barrel is **optional** and may stay empty (match the prevailing direct-import convention).

---

## Shared Patterns

### sha256 hash-only-at-rest (sessions, mirroring bearer codes)
**Source:** `internal/backendsrv/auth/mint.go:30-54` (mint) + `auth/guard.go:69-102` (resolve) + the package doc (mint.go:5-21)
**Apply to:** `webauth/session.go`. 32 `crypto/rand` bytes → `base64.RawURLEncoding` plaintext (cookie) → `sha256.Sum256` → store the hash (`BLOB`) only. NEVER store/log the plaintext (V6/V7). `crypto/rand`, NEVER `math/rand`.

### Guard → context handoff (middleware → handler)
**Source:** `auth/guard.go` (resolve pattern) + `ingest/whoami.go:66`/`ingest/handler.go:94` (the call sites). NOTE the repo passes resolved identity by RETURN VALUE today; P15 introduces the ctx-key variant (`webauth.SessionContextKey`) because one middleware fronts many endpoints.
**Apply to:** `webauth/guard.go` stashes `*Session`; every form/whoami handler reads it with the comma-ok type assertion and 401s on `!ok`.

### BeginTx immediate-lock mutator (ALL store writes)
**Source:** `internal/backendsrv/store/replace.go:63-75` (the canonical `tx,_ := s.db.BeginTx(ctx,nil); defer tx.Rollback(); ...; tx.Commit()` shape) + `store/db.go:39-46` (the `_txlock=immediate` DSN that makes every `BeginTx` a `BEGIN IMMEDIATE`). There is NO `withTx` wrapper.
**Apply to:** `store/admins.go`, `store/eviction.go`, `store/coin.go`, `store/webuser.go`, `store/audit.go`. The officer-write methods do their re-authorization `SELECT` on the SAME `*sql.Tx` as the mutation — the `_txlock=immediate` write lock IS the TOCTOU close (D-06), no extra mechanism.

### nullable-timestamp lifecycle flag
**Source:** `guild_code.disabled_at` (`00001_init.sql:54`; set via `auth/mint.go:RevokeCode:64-72`)
**Apply to:** `web_session.expires_at` (TTL predicate), `character.grace_until` + `character.archived_at` (eviction grace/archive), and the eviction code-revoke (reuse `disabled_at` EXACTLY — D-10). `NULL = active/not-yet`; non-NULL = the lifecycle event happened. Look up "active" rows with `... WHERE <col> IS NULL` (or `expires_at > datetime('now')` for the TTL variant).

### append-only audit, in-transaction (ALL admin/eviction/coin writes)
**Source:** `00002_audit.sql` (the `audit_log` table) + `store/binding.go` (the cross-owner-reject in-tx audit insert, the existing writer). CONTEXT line 86/112: REUSE/EXTEND `audit_log`, don't invent a parallel log.
**Apply to:** `store/audit.go` inserts a row INSIDE the same transaction as each admin/eviction/coin mutation (D-06). Extend `audit_log` with nullable `actor`/`target`/`detail` columns in `00004` for the snowflake-shaped P15 actors.

### goose migration header + driver/dialect foot-gun
**Source:** `00001`/`00002`/`00003` (bare `-- +goose Up`/`Down`, NO StatementBegin/End); `embed.go:33-39` (runs `goose.SetDialect("sqlite3")` then `goose.Up(db,".")` over `//go:embed *.sql`). **Foot-gun (embed.go:8-12):** driver name is `"sqlite"` (modernc), goose dialect is `"sqlite3"` — they differ on purpose.
**Apply to:** `00004_web_auth.sql`. Forward-only, extend-only; auto-applies on deploy-restart (CONTEXT line 111). SQLite allows only ONE column per `ALTER TABLE ADD COLUMN` (00003 header).

### JSON error/success envelope (NEW — web/form endpoints only)
**Source:** none in-repo (ingest/readapi use `http.Error` text + inline `json.Encode`). P15 ADDS `webauth/jsonerr.go`'s `writeJSONError(w, status, code, msg)` (`{error,message}` JSON) + `writeJSON(w, status, body)` because the SvelteKit client routes on JSON error codes (`not_authorized`/`owner_floor_protected`/`lock_busy`, UI-SPEC lines 234/249).
**Apply to:** every handler in `webauth/` + `forms/`. (The existing read/ingest endpoints keep their `http.Error` style; only the new surface uses JSON errors.)

### Inline method-check + Go 1.22 method routing
**Source:** `ingest/handler.go:82-85` (inline `if r.Method != http.MethodPost`) + `runServe`'s `mux.Handle("POST /api/v1/ingest", ...)` (main.go:269). There is NO `methodGuard` middleware.
**Apply to:** every new route — register with the `"POST /api/v1/..."` / `"GET /api/v1/..."` pattern string; keep the inline defensive method check in each `ServeHTTP` (or add a small shared guard — planner's call).

### Svelte 5 component conventions (ALL new frontend components)
**Source:** `ThemePicker.svelte`, `+page.svelte`, `StateBlock.svelte`, `SearchBox.svelte`
**Apply to:** all 8 new components.
- `let { ... } = $props();` with an inline type or `interface Props` (ThemePicker:9, +page implicit; SiteShell:18-21 shows the typed `$bindable` form).
- `$state(...)` for reactive local state; `$bindable()` for two-way props (ThemePicker:9, SearchBox:13); `$derived(...)` for computed (SearchBox:20-33).
- `onMount(async () => {...})` for read-API loads (+page.svelte:117-119).
- `{#if status}...{:else}` state ladder with `StateBlock` (+page.svelte:131-192).
- Scoped `<style>` consuming ONLY the P14 CSS custom properties (`--bg --panel --text --accent --border --status-missing --status-ok --font-display --font-body --weight-display`); control fields = `--panel` bg, `1px var(--border)` (or `var(--accent)`), 4–6px radius, 16px `--font-body`, `min-height: 44px`, `outline: 2px solid var(--accent)` on `:focus-visible` (ThemePicker:44-58 / SearchBox:81-96). NO new design tokens, NO new UI deps (UI-SPEC § Registry Safety).
- `@lucide/svelte/icons/*` for glyphs (already a dep; StateBlock:10 imports `circle-alert`).
- Trust boundary: interpolate user/Discord strings via plain `{}` (auto-escaped, StateBlock:69 precedent); **never `{@html}`** on them (UI-SPEC line 238).

### Auth-aware fetch + 401/403 routing (frontend)
**Source:** `web/src/lib/api.ts` `getJSON` (lines 111-136) + `ApiError` (lines 24-31)
**Apply to:** every read AND write call in `api.ts` — add `credentials: 'include'`; classify 401 (→ AuthGate → LoginScreen) and 403 (→ NotMember / Officers-only) off `ApiError.status` (UI-SPEC line 249). Auth is server-truth — never render stale authorized UI after a 401/403.

---

## No Analog Found

No file is fully analog-less. The genuinely-new patterns (flagged so the planner sets expectations):

| File / Pattern | Role | Reason / Guidance |
|----------------|------|-------------------|
| `web/src/lib/components/ConfirmDialog.svelte` | component (modal) | The repo has **no modal/dialog**. Build from `StateBlock`'s `--panel`+icon+scoped-style + `color-mix` backdrop conventions + the focus-trap/`role="dialog"` spec (UI-SPEC lines 194-197). First `--destructive` (= `--status-missing`) usage. |
| `internal/backendsrv/webauth/membership.go` | service | No existing **outbound OAuth/identity** call. Closest outbound-JSON pattern is `enrich/jobs/pigparse.go` / `enrich/pigparse.go` (`http.Client` + `json.Decode`); the Discord `guilds`-list membership check (D-02) reuses that fetch shape but is a new integration. Hand-rolled `golang.org/x/oauth2` per D-04 — no Discord SDK. |
| `internal/backendsrv/webauth/jsonerr.go` | utility | No JSON-error envelope exists (repo uses `http.Error` text). NEW, but trivial; the `{error,message}` shape is the de-facto contract the SvelteKit client + UI-SPEC error copy assume. |
| ctx-key auth handoff | middleware idiom | The repo's guards pass identity by return value, not `context.WithValue`. P15 introduces the standard ctx-key middleware idiom because one middleware fronts many endpoints — new to this repo but textbook Go. |

---

## Metadata

**Analog search scope:** `internal/backendsrv/{auth,ingest,readapi,store,scheduler,migrations,enrich/jobs}`, `cmd/squirebot-server`, `web/src/{lib,lib/components,routes}`, `apps-script/src/{lib,triggers}` (oracle only).
**Files read in full:** 31 analog files + 3 phase docs (`15-CONTEXT.md`, `15-UI-SPEC.md`; `15-DISCUSSION-LOG.md` listed only).
**Key absences confirmed by reading:** no `15-RESEARCH.md`; no `internal/backendsrv/server.go`; no `withTx` wrapper; no `methodGuard`; no `WriteJSONError`/`ErrorEnvelope`. The CLI arms are `runMint`/`runRevoke`/`runJobCmd`; `runServe` is fully implemented (main.go:226-319) and is EDITED, not authored.
**Pattern extraction date:** 2026-05-30
