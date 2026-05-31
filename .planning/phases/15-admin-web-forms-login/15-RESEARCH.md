# Phase 15: Admin Web Forms + Login - Research

**Researched:** 2026-05-30
**Domain:** Discord OAuth2 (hand-rolled `golang.org/x/oauth2`) + server-side opaque sessions + cross-subdomain cookies + credentialed CORS + ported v1 admin/eviction enforcement (Go/SQL) + auth-gated SvelteKit routing
**Confidence:** HIGH on mechanics (Discord endpoints, cookie/CORS semantics, schema, ported enforcement rules — all verified against official docs + the actual codebase); MEDIUM on a few in-session-unverifiable internals (flagged in the Assumptions Log)

> **Tooling note:** A sustained tool-execution outage mid-session prevented re-reading a handful of backend internals (`auth/guard.go` body, `scheduler/scheduler.go` body, `cmd/squirebot-server/main.go` body, the SvelteKit `+layout.ts`/`+page.ts` load functions). Their public shapes are known from files I *did* read this session (`ingest/whoami.go` shows the exact `auth.ResolveToken` signature and handler pattern; `readapi/cors.go` documents the exact CORS upgrade path; `00001`/`00002` give the full schema; `admin.ts`/`showEvictionSidebar.ts` give the enforcement oracle verbatim) plus ROADMAP/STATE which document the scheduler + CLI subcommand patterns. Items I could not directly re-verify are tagged `[ASSUMED]` and collected in the Assumptions Log — the planner should have the executor confirm them by reading the cited file before relying on the exact symbol name.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions (D-01..D-12 — DO NOT re-open)

- **D-01: Whole-site gate.** Discord login walls the ENTIRE site. The membership gate is enforced at the **read API** (every `/api/v1/...` read endpoint requires a valid session), not just the SvelteKit frontend. Non-members of the guild Discord server are refused.
- **D-02: Scopes = `identify` + `guilds`** ONLY. Membership check = the configured guild ID is present in the user's `guilds` list — **no Discord bot**. No `email` scope.
- **D-03: Identity stored (AUTH-09) = `discord_user_id` (snowflake) + `username` + `avatar`.** Upserted on each login. The snowflake is the stable key + the v2-pinger DM handle.
- **D-04: Login = hand-rolled `golang.org/x/oauth2`** authorization-code flow, server-side code exchange. Client secret **backend-only** (env / systemd). Locked by the 11-01 spike verdict (no PocketBase).
- **D-05: Server-side opaque session.** Random opaque id, stored **hashed** in a new `web_session` table (→ `discord_user_id`), set as an **httpOnly + Secure + SameSite=Lax cookie scoped to `Domain=squirebot.quest`** so it rides cross-subdomain to `api.squirebot.quest`. CORS gains `Access-Control-Allow-Credentials: true`. **TTL ≈ 30 days, rolling.** Membership re-checked at each login.
- **D-06: Officers = ported `guild_admins` allowlist keyed by Discord user ID** (snowflake), in the **DB**. Port v1 `admin.ts` semantics verbatim: fail-closed `requireAdmin`; **authorize INSIDE the write transaction** (TOCTOU/WR-04 fix); idempotent add/remove; **append-only admin audit log** (reuse/extend `audit_log`).
- **D-07: Add-officer UX = pick from users who've already logged in.** No snowflakes typed. Trade-off: a guildie must sign in once before promotion.
- **D-08: Owner-floor = CLI-seeded maintainer Discord ID** via `squirebot-server set-owner-floor <discord-id>`, run once at deploy. Un-removable by peers (self-removal follows v1's orphan-pointer rule). The seeded ID is also the first/bootstrap admin.
- **D-09: Eviction targets a whole guildie (owner).** Cascade `is_removed = 1` across ALL their `character` rows. Officer-only; port owner-floor protection so a peer can't evict the floor's data.
- **D-10: Eviction also revokes the owner's guild code immediately** (set `guild_code.disabled_at`) AND keeps v1's 30-day grace + auto-archive. **Reversible during grace** (un-set `is_removed` + re-mint code). Requires a grace/archive mechanism + scheduled archive step (port `weeklyEvictionArchive`).
- **D-11: Coin entry limited to `is_bank_toon` characters.** Stored as **nullable `plat`/`gold`/`silver`/`copper` integer columns on `character`**. Form writes them; bank view surfaces them.
- **D-12: Bank-coin entry is gated by login only, NOT officer-only** (ADMIN-05 says "authenticated"). Any signed-in guild member may update coin.

### Claude's Discretion (research recommends below; planner decides)
- Session/cookie exact attributes + OAuth redirect-callback route shape + cross-subdomain cookie + CORS-credentials mechanics → **§2, §3, §4 below**.
- Exact `00004_*` goose migration DDL → **§5 below**.
- `guild_admins` as its own table vs `is_admin`/`role` column on web-user; owner-floor as a singleton row vs config → **§5 below (recommended: separate table; singleton row)**.
- Coin input validation/units → **§5/§6: plat free `≥0`, gold/silver/copper `0–999`** (matches UI-SPEC D-11 lock).
- Eviction archive as a scheduled job vs lazy-on-read → **§7 (recommended: in-process scheduled job via the existing Job registry)**.
- All login-screen + form visual/interaction layout → already locked by `15-UI-SPEC.md` (approved 2026-05-30).

### Deferred Ideas (OUT OF SCOPE — do not plan)
- v2 Wantlist + Discord pinger (WANT-01..08) — this phase only captures AUTH-09 identity.
- Per-member visibility tiers (universal visibility remains).
- Tightening bank-coin to officer-only (currently authenticated-member per ADMIN-05/D-12).
- Shadow-soak / human-data backfill / coordinated watcher flip / Sheet + Apps Script + Google-OAuth decommission → P16.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **AUTH-08** | Discord OAuth2 login gated on guild Discord-server membership; no hand-maintained allowlist | §1 (hand-rolled OAuth flow + `/users/@me/guilds` membership gate), §2 (session), §8 (SvelteKit gate), §10 (Discord app prereq) |
| **AUTH-09** | Capture + store each signed-in user's Discord identity (v2 pinger prerequisite) | §1 (`/users/@me` → snowflake + username + avatar), §5 (`web_user` upsert DDL) |
| **ADMIN-04** | Eviction (30-day grace + archive) as authenticated, **officer-only** web form; ports v1 `guild_admins` gate + owner-floor protection | §6 (ported eviction semantics + error strings), §5 (grace/archive DDL), §7 (archive job), §8 (officer routing) |
| **ADMIN-05** | Manual bank-coin entry (plat/gold/silver/copper) as authenticated web form | §5 (coin columns DDL), §6 (bank-toon-only write rule), UI-SPEC §Form Contracts (validation) |
| **ADMIN-06** | Admin/officer management (`guild_admins` allowlist + owner-floor lockout) as **officer-only** web form | §6 (ported `addAdmin`/`removeAdmin` semantics + error strings), §5 (`guild_admins` DDL), §8 |
</phase_requirements>

## Summary

This phase is an **additive auth + write layer** on top of the live, hand-rolled Go backend (`net/http` ServeMux + `modernc.org/sqlite` + `goose`, live at `api.squirebot.quest`) and the static SvelteKit site (`squirebot.quest`, served by Caddy on the same VPS at the apex). Nothing in the existing stack changes shape; we add: (1) a hand-rolled Discord OAuth2 authorization-code flow, (2) a server-side opaque session backed by a new `web_session` table, (3) a session-gate middleware that **wraps the existing read API** plus credentialed CORS, (4) a `whoami` endpoint the SvelteKit `AuthGate` polls, (5) three write endpoints (`bank-coin`, `eviction`, `admin-mgmt`) whose enforcement is a **verbatim port** of v1's battle-tested `admin.ts` / `showEvictionSidebar.ts` semantics into Go/SQL transactions, and (6) the `00004_*` goose migration carrying `web_user`, `web_session`, `guild_admins`, owner-floor, coin columns, and eviction grace/archive.

The cross-subdomain story is the subtle part and is fully solvable with **SameSite=Lax** because `squirebot.quest` (the SvelteKit origin) and `api.squirebot.quest` (the API) share the same **registrable domain** (`squirebot.quest`) — they are *same-site* even though they are *cross-origin*. SameSite=Lax therefore sends the session cookie on the credentialed cross-origin API calls, and it also permits the top-level GET navigation that returns from Discord's OAuth redirect. The cookie must be set with `Domain=squirebot.quest` so the browser scopes it to the registrable domain (and thus includes the `api.` subdomain). Credentialed CORS requires the **exact origin echo** (`Access-Control-Allow-Origin: https://squirebot.quest`, never `*`) plus `Access-Control-Allow-Credentials: true`, and the SvelteKit fetches must pass `credentials: 'include'`.

**Primary recommendation:** Build the OAuth + session layer as a new `internal/backendsrv/websession` (session mint/lookup/delete + cookie helpers) + `internal/backendsrv/oauth` (Discord flow) package pair, mirror the `ingest/whoami.go` handler conventions exactly, port `admin.ts`/`showEvictionSidebar.ts` enforcement into a new `internal/backendsrv/adminapi` package authorizing **inside** each write `*sql.Tx`, land all schema as one forward-only `00004_websession_admin.sql`, register a daily `eviction_archive` job in the existing scheduler, and add a `set-owner-floor` subcommand to `cmd/squirebot-server`. Surface the **Discord app provisioning** as a loud non-code maintainer prerequisite (§10).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Discord OAuth redirect + code exchange | API / Backend | — | Client secret is backend-only (D-04); the exchange MUST be server-side. The frontend only does a full-page navigation to the backend's `/auth/login`. |
| CSRF `state` generation + verification | API / Backend | — | State is minted + stored server-side and verified on callback; never trusted from the client. |
| Guild-membership gate | API / Backend | — | `/users/@me/guilds` is called with the user's access token server-side; membership is the AUTH-08 boundary and must not be client-evaluable. |
| Identity capture (snowflake/username/avatar) | API / Backend | DB / Storage | `/users/@me` server-side → upsert into `web_user` (AUTH-09). |
| Session mint / lookup / expiry / destroy | API / Backend | DB / Storage | Opaque id hashed into `web_session`; cookie is the only client-side artifact. |
| Session cookie (httpOnly/Secure/SameSite/Domain) | Browser / Client | API / Backend | Set by the backend; stored + auto-sent by the browser. httpOnly keeps JS out of it. |
| Read-API session gate | API / Backend | — | D-01: every read endpoint requires a valid session. Frontend gating is UX only. |
| Officer authorization (`guild_admins`) | API / Backend | DB / Storage | Authorize **inside** the write tx (D-06 TOCTOU). The server is the boundary. |
| Eviction cascade + code-revoke + grace | API / Backend | DB / Storage | One app-controlled tx over `character` + `guild_code` (D-09/D-10). |
| Eviction auto-archive after grace | API / Backend (scheduler) | DB / Storage | In-process daily job (D-10 / §7), mirrors `pigparse_daily`/`wiki_weekly`. |
| Bank-coin write | API / Backend | DB / Storage | Authenticated-member (D-12); writes `character` coin columns for `is_bank_toon` rows. |
| AuthGate / LoginScreen / NotMemberScreen routing | Frontend Server (SvelteKit, static SPA) | — | UX-only gate; resolves session via `whoami` with `credentials:'include'`; routes by `{authenticated,isMember,isOfficer}`. |
| The 3 forms' inputs + confirm dialogs | Browser / Client (Svelte) | — | Native controls; `{}` auto-escape on all interpolated Discord/user strings (never `{@html}`). |
| Owner-floor seed | CLI / ops | DB / Storage | `set-owner-floor` subcommand, run once at deploy (D-08). |
| Discord app (client id/secret, redirect URI) | External (Discord Dev Portal) + ops | — | Non-code maintainer prerequisite (§10). |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/oauth2` | **add to go.mod** (latest tagged release at install time, e.g. `v0.30.x` line) | Discord authorization-code flow: `oauth2.Config`, `AuthCodeURL(state)`, `Exchange(ctx, code)`, `cfg.Client(ctx, tok)` | Locked by D-04 / 11-01 spike. Already in the watcher's *old* dependency tree (deleted in P13); re-adding for the server side is idiomatic. No PocketBase, no third-party OAuth framework. `[VERIFIED: go.mod — x/oauth2 currently ABSENT from server deps; must be re-added]` |
| `net/http` (stdlib) | Go 1.25.7 | ServeMux routing, handlers, `http.SetCookie`, `http.Cookie`, the `cfg.Client` transport | Project rule: hand-rolled `net/http`, NO web framework `[CITED: CLAUDE.md "Never use… Electron"; STATE "no web framework on the backend"]` |
| `crypto/rand` (stdlib) | Go 1.25.7 | Opaque session id + OAuth `state` generation (high-entropy random bytes) | Same generator family the existing guild-code mint uses. Never `math/rand`. |
| `crypto/sha256` (stdlib) | Go 1.25.7 | Hash the session id + `state` before DB storage (store the hash, not the secret) | Mirrors `guild_code.token_hash` = `sha256(plaintext)` exactly `[VERIFIED: 00001_init.sql line 52]` |
| `crypto/subtle` (stdlib) | Go 1.25.7 | Constant-time compare on session/state hash lookup | Mirrors the 11-04 bearer guard (`crypto/subtle` constant-time compare) `[VERIFIED: ROADMAP 11-04 line]` |
| `modernc.org/sqlite` | v1.51.0 | The store (pure-Go, no cgo) | Existing backend DB driver `[VERIFIED: go.mod line 15]` |
| `github.com/pressly/goose/v3` | v3.27.1 | Forward-only migrations; `goose.Up` on startup | Existing migration tool `[VERIFIED: go.mod line 10]` |
| `encoding/json` (stdlib) | Go 1.25.7 | Decode Discord `/users/@me` + `/users/@me/guilds`; encode `whoami` + form responses | Existing pattern `[VERIFIED: ingest/whoami.go]` |

### Supporting (frontend — NO new deps)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| SvelteKit (`@sveltejs/adapter-static`) | as shipped in P14 | Static SPA; the AuthGate/forms slot into `web/src/` | Confirm the P14 adapter is `adapter-static` (SPA, `200.html` fallback) — see §8 / Assumptions Log |
| Svelte 5 + Tailwind v4 + `@lucide/svelte` 1.17 | as shipped in P14 | The login screen + 3 forms reuse the P14 token system + glyphs | UI-SPEC §Design System: **no new UI dependency** for P15 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `golang.org/x/oauth2` | Raw `net/http` token POST | x/oauth2 is locked (D-04) and handles the `client_secret`-in-body + token-refresh plumbing; raw http is more code for no gain. Use x/oauth2. |
| Opaque server-side session | Signed JWT cookie | D-05 locks opaque server-side (revocable, no client-trusted claims). JWT would re-introduce the "can't revoke until expiry" problem. Do not use JWT. |
| Separate `guild_admins` table | `is_admin` column on `web_user` | **Recommend separate table** (§5) — matches v1's discrete allowlist concept, keeps an explicit `added_by`/`added_at` audit trail, and lets a future role expansion not churn `web_user`. (Either is defensible per D-discretion.) |
| In-process archive job | Lazy-on-read archive | **Recommend in-process job** (§7) — the scheduler + `job_run` cursor already exist (`pigparse_daily`/`wiki_weekly`); lazy-on-read scatters the grace logic across every read path. |

**Installation:**
```bash
# server side only — the watcher already shed its oauth2 tree in P13
go get golang.org/x/oauth2@latest
go mod tidy
```

**Version verification:** Confirm the resolved `x/oauth2` version after `go get` (training data versions are stale; the package is stable and rarely breaks). `golang.org/x/crypto v0.52.0` is already an indirect dep `[VERIFIED: go.mod line 31]` — the x/oauth2 add will pull a compatible x/net/x/crypto. No other new deps.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────────┐
                         │  Browser (guildie)                          │
                         │  cookie: sb_session=<opaque>  (httpOnly)    │
                         └───────────────┬─────────────────────────────┘
                                         │
        (1) full-page nav                │  (4) fetch(..., credentials:'include')
        GET squirebot.quest              │      to api.squirebot.quest/api/v1/*
        (static SvelteKit SPA)           │      cookie rides (same registrable domain)
                                         │
   ┌─────────────────────────────────────┼──────────────────────────────────────┐
   │  Caddy (one VPS, auto-HTTPS)         │                                        │
   │  apex squirebot.quest → static       │   api.squirebot.quest → 127.0.0.1:8090 │
   └─────────────────────────────────────┼──────────────────────────────────────┘
                                         │
                         ┌───────────────▼─────────────────────────────┐
                         │  squirebot-server (Go, net/http ServeMux)    │
                         │                                              │
   (2) login button ────►│  GET  /auth/login                            │
   (full nav, not fetch) │     mint state→store hash→302 to Discord     │──► discord.com/oauth2/authorize
                         │                                              │     (?client_id&scope=identify guilds
   (3) Discord 302 back ►│  GET  /auth/callback?code&state              │      &state&redirect_uri&response_type=code)
                         │     verify state ─► exchange code ───────────┼──► discord.com/api/oauth2/token  (client_secret in body)
                         │     GET /users/@me ──────────────────────────┼──► identity (snowflake/username/avatar)
                         │     GET /users/@me/guilds ───────────────────┼──► membership check (GUILD_ID present?)
                         │     ├─ member  → upsert web_user             │
                         │     │            mint session→hash→web_session│
                         │     │            Set-Cookie sb_session        │
                         │     │            302 → https://squirebot.quest│
                         │     └─ not member → 302 → squirebot.quest?notmember=1
                         │                                              │
                         │  [session-gate middleware] wraps:            │
   (4) whoami/reads ────►│    GET /api/v1/whoami   → {authenticated,    │
                         │                            isMember,isOfficer,│
                         │                            username,avatar,   │
                         │                            discord_user_id}   │
                         │    GET /api/v1/views/*  (now session-gated)   │──► store/compute (P14)
                         │    GET /api/v1/meta     (now session-gated)   │
                         │                                              │
   (5) writes ──────────►│    POST /api/v1/bank-coin   (member)          │──┐
                         │    POST /api/v1/eviction    (officer, in-tx)  │  │  one *sql.Tx each;
                         │    GET/POST /api/v1/admins   (officer, in-tx) │  │  authorize INSIDE the tx
                         │    POST /auth/logout (delete row + clear cookie)│ │  (D-06 TOCTOU)
                         │                                              │  ▼
                         │  [scheduler] daily eviction_archive job ─────┼──► SQLite (00004 tables)
                         └──────────────────────────────────────────────┘
```

A reader can trace the primary use case (sign in → see data → officer evicts) by following (1)→(2)→(3)→(4)→(5).

### Recommended Backend Package Layout
```
internal/backendsrv/
├── oauth/              # NEW — Discord authorization-code flow
│   ├── discord.go      #   oauth2.Config builder, /users/@me + /users/@me/guilds calls, membership test
│   └── handlers.go     #   GET /auth/login, GET /auth/callback, POST /auth/logout
├── websession/         # NEW — opaque session lifecycle + cookie helpers
│   ├── session.go      #   Mint(discordUserID) (id, error); Lookup(rawCookie) (Session, ok); Delete(rawCookie); slide-expiry
│   └── cookie.go       #   setSessionCookie(w, rawID); clearSessionCookie(w); the exact attribute set (§2)
├── webauth/            # NEW — the session-gate middleware + officer resolution
│   └── gate.go         #   RequireSession(next) middleware; ResolveOfficer(ctx, discordUserID) bool
├── adminapi/           # NEW — ported v1 enforcement (admin.ts + eviction)
│   ├── admins.go       #   addAdmin / removeAdmin / listAdmins / listPromotable (authorize-under-tx)
│   ├── eviction.go     #   listEvictable / previewEviction / commitEviction (cascade + revoke + grace)
│   ├── coin.go         #   updateCoin (bank-toon-only, member-gated)
│   └── archive.go      #   the daily eviction-archive job body (registered in scheduler)
├── store/              # EXISTS — add session/web_user/admin/coin/archive read+write methods here
│   └── websession.go   #   NEW file: parameterized SQL for all 00004 tables (zero inline SQL elsewhere — project rule)
└── migrations/
    └── 00004_websession_admin.sql   # NEW — forward-only, mirrors 00001 shape
```

### Pattern 1: Mirror `whoami.go` for every new handler
**What:** New handlers are plain `http.Handler` structs constructed once at startup, registered on the ServeMux with method-prefixed patterns (`mux.Handle("GET /api/v1/whoami", ...)`), defensively re-check method, and never log secrets.
**When to use:** All P15 endpoints.
**Example (the established convention, verbatim from the shipped code):**
```go
// Source: internal/backendsrv/ingest/whoami.go (read this session)
type WhoamiHandler struct { guard *auth.Auth; db *sql.DB }
func NewWhoami(guard *auth.Auth, db *sql.DB) *WhoamiHandler { return &WhoamiHandler{guard: guard, db: db} }
func (h *WhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    ownerID, ok := h.guard.ResolveToken(r.Context(), r.Header.Get("Authorization"))
    if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
    // ... parameterized read, JSON encode ...
}
```
Note the **bearer** `auth.ResolveToken` is the WATCHER path (guild codes). The P15 **human session** path is a *sibling* — `webauth.RequireSession` resolves the `sb_session` cookie, not the `Authorization` header. They coexist; the read API gate uses the cookie path, the ingest path keeps the bearer path.

### Pattern 2: Authorize INSIDE the write transaction (the load-bearing v1 port)
**What:** Every officer-only mutator opens the `*sql.Tx` FIRST, then re-checks `guild_admins` membership *within that tx*, then mutates, then commits. A just-removed officer must not slip one final write through (v1's WR-04 / TOCTOU fix).
**When to use:** `commitEviction`, `addAdmin`, `removeAdmin`. (Bank-coin is member-gated, not officer — it still authorizes the session inside its handler, but there's no allowlist re-check.)
**Example (the Go shape; semantics ported verbatim from `admin.ts`):**
```go
// Source: ported from apps-script/src/lib/admin.ts addAdmin (read this session, lines 145-190)
func (s *Store) AddAdmin(ctx context.Context, targetDiscordID, callerDiscordID string) (added bool, err error) {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil { return false, err }
    defer tx.Rollback() // no-op after Commit
    // Authorize UNDER the tx (WR-04 — close the TOCTOU window).
    if !isAdminTx(ctx, tx, callerDiscordID) { return false, ErrNotAuthorized }
    // Idempotent: already an admin → no write.
    if isAdminTx(ctx, tx, targetDiscordID) { return false, nil } // alreadyExists
    if _, err = tx.ExecContext(ctx,
        `INSERT INTO guild_admins (discord_user_id, added_by, added_at)
         VALUES (?, ?, datetime('now'))`, targetDiscordID, callerDiscordID); err != nil { return false, err }
    if _, err = tx.ExecContext(ctx,
        `INSERT INTO audit_log (event, char_name, attempting_owner_id, current_owner_id)
         VALUES ('admin_add', ?, NULL, NULL)`, targetDiscordID); err != nil { return false, err } // see §5 audit note
    return true, tx.Commit()
}
```
The single `*sql.Tx` is SQLite's natural lock envelope — it **replaces** v1's `LockService.getDocumentLock().tryLock(30000)`. There is no separate lock object; the transaction *is* the serialization. (If a `lock_busy` user-facing string is still wanted for parity, map a `SQLITE_BUSY` / context-timeout error to it — see §6.)

### Anti-Patterns to Avoid
- **`Access-Control-Allow-Origin: *` with credentials.** Browsers reject it outright; it is a spec violation. Always echo the exact origin (cors.go already does this and explicitly documents the P15 upgrade as "a one-line change"). `[VERIFIED: readapi/cors.go comment]`
- **Putting the Discord client secret anywhere the static bundle can see it.** It is `PUBLIC_API_BASE`-adjacent only as a *backend* env var; never a `PUBLIC_*` SvelteKit var (those are inlined into the client bundle).
- **`{@html}` on a Discord username.** Svelte `{}` auto-escapes; the only sanctioned `{@html}` in the app is `ItemTooltip`'s pre-escaped `composeItemNote` (P14, unchanged). `[CITED: UI-SPEC §Copywriting Trust-boundary]`
- **Authorizing before opening the tx.** Re-introduces the TOCTOU window v1 closed. Authorize *inside*.
- **Trusting the `state` round-trip without server-side storage.** A `state` value the server didn't mint+store is forgeable. Store the state hash with a short TTL; verify+consume on callback.
- **Per-character view tabs / any per-character API explosion.** Not relevant here but the standing CLAUDE.md guard: consolidated views only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OAuth2 code exchange + token transport | A bespoke token POST + refresh handler | `golang.org/x/oauth2` `oauth2.Config.Exchange` + `cfg.Client(ctx, tok)` | Handles `client_secret`-in-body (Discord requires it even with confidential clients), token JSON parsing, and an `http.Client` that injects the bearer for the `/users/@me*` calls. D-04 locks it. |
| Session id / CSRF state entropy | `math/rand`, timestamps, UUIDv4-as-secret | `crypto/rand` → base64url (mirror the guild-code mint) | Predictable session ids = account takeover. |
| Constant-time secret compare | `==` on hashes | `crypto/subtle.ConstantTimeCompare` | Mirrors the 11-04 bearer guard; avoids timing leak on session lookup. |
| Cookie attribute assembly | Manual `Set-Cookie` string concatenation | `http.Cookie{...}` + `http.SetCookie(w, &c)` | The stdlib serializes `Domain`/`SameSite`/`Secure`/`HttpOnly`/`Path`/`MaxAge` correctly; hand-built strings get the `SameSite`/`Domain` form wrong. |
| Lock envelope for multi-step admin writes | A Go mutex + read-modify-write | One `*sql.Tx` (BEGIN…COMMIT) | The transaction is the serialization boundary; it replaces v1's `LockService` AND closes the TOCTOU window in the same construct. |
| 30-day grace date math | Ad-hoc time arithmetic in Go strings | SQLite `datetime('now','+30 days')` + `strftime`/`datetime('now')` comparisons | Keeps the grace boundary in the DB (consistent with the TEXT-datetime columns already used everywhere — `created_at TEXT DEFAULT (datetime('now'))`). |

**Key insight:** Every primitive this phase needs already has a battle-tested in-repo or stdlib counterpart. The *novel* work is wiring, not invention — and the enforcement *rules* are not novel at all: they exist, tested, in `admin.ts` + `showEvictionSidebar.ts` + `weeklyEvictionArchive.ts`. Port the rules; don't re-derive them.

## Code Examples

### §1 — Discord OAuth2 hand-rolled flow (D-02/D-04)

**Verified endpoints** `[VERIFIED: Discord OAuth2 docs via web search + 03-watcher-auth.md §4.2]`:
- Authorize: `https://discord.com/oauth2/authorize`
- Token: `https://discord.com/api/oauth2/token` (requires `client_id`+`client_secret` in the form body — Discord enforces the secret even for confidential clients; this is fine, the secret is backend-only)
- Identity: `GET https://discord.com/api/users/@me` (scope `identify`) → `{ id (snowflake string), username, avatar (hash or null), global_name, ... }`
- Membership: `GET https://discord.com/api/users/@me/guilds` (scope `guilds`) → array of partial guild objects `[{ id (snowflake string), name, ... }]`; membership = configured `GUILD_ID` present in that array. **No bot, no `guilds.members.read`.**
- Avatar CDN URL (for the frontend): `https://cdn.discordapp.com/avatars/<user_id>/<avatar_hash>.png` (null hash → default avatar; the frontend handles the `user`-glyph fallback per UI-SPEC).

```go
// Source: golang.org/x/oauth2 + Discord OAuth2 docs (endpoints VERIFIED)
package oauth

import (
    "context"
    "encoding/json"
    "net/http"
    "golang.org/x/oauth2"
)

// Endpoints are hardcoded (Discord-specific). The token URL uses /api/oauth2/token.
var discordEndpoint = oauth2.Endpoint{
    AuthURL:  "https://discord.com/oauth2/authorize",
    TokenURL: "https://discord.com/api/oauth2/token",
}

func newConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
    return &oauth2.Config{
        ClientID:     clientID,        // from env (Discord app)
        ClientSecret: clientSecret,    // BACKEND-ONLY env/systemd secret
        RedirectURL:  redirectURL,     // e.g. https://api.squirebot.quest/auth/callback
        Scopes:       []string{"identify", "guilds"}, // D-02 — exactly these two
        Endpoint:     discordEndpoint,
    }
}

type DiscordUser struct {
    ID       string `json:"id"`       // snowflake (string!) — D-03 stable key
    Username string `json:"username"`
    Avatar   string `json:"avatar"`   // hash or "" ; frontend builds the CDN URL
}
type partialGuild struct {
    ID string `json:"id"`
}

// fetchIdentity + membership use the token-injecting client x/oauth2 builds.
func (s *Service) identityAndMembership(ctx context.Context, tok *oauth2.Token) (DiscordUser, bool, error) {
    client := s.cfg.Client(ctx, tok) // injects Authorization: Bearer <access_token>
    var u DiscordUser
    if err := getJSON(ctx, client, "https://discord.com/api/users/@me", &u); err != nil {
        return DiscordUser{}, false, err
    }
    var guilds []partialGuild
    if err := getJSON(ctx, client, "https://discord.com/api/users/@me/guilds", &guilds); err != nil {
        return u, false, err
    }
    isMember := false
    for _, g := range guilds {
        if g.ID == s.guildID { isMember = true; break } // s.guildID from env
    }
    return u, isMember, nil
}

func getJSON(ctx context.Context, c *http.Client, url string, dst any) error {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := c.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { return errUnexpectedStatus(resp.StatusCode) } // handle 429 w/ Retry-After
    return json.NewDecoder(resp.Body).Decode(dst)
}
```

```go
// Source: idiomatic Go + the whoami.go handler conventions (VERIFIED pattern)
// GET /auth/login — mint state, store its hash, 302 to Discord.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    state, err := randToken() // crypto/rand → base64url
    if err != nil { http.Error(w, "internal", http.StatusInternalServerError); return }
    if err := h.store.PutOAuthState(r.Context(), sha256hex(state)); err != nil { // short TTL row
        http.Error(w, "internal", http.StatusInternalServerError); return
    }
    // (Optional hardening: also set a short-lived httpOnly state cookie and require both to match —
    //  double-submit. The server-side stored-state check is the baseline and is sufficient.)
    http.Redirect(w, r, h.cfg.AuthCodeURL(state), http.StatusFound)
}

// GET /auth/callback?code=...&state=... — verify state, exchange, fetch, gate, mint session.
func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    state := r.URL.Query().Get("state")
    if state == "" || !h.store.ConsumeOAuthState(r.Context(), sha256hex(state)) { // verify + single-use
        http.Error(w, "bad state", http.StatusBadRequest); return // CSRF guard (ASVS V5)
    }
    tok, err := h.cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
    if err != nil { http.Error(w, "exchange failed", http.StatusBadGateway); return }
    user, isMember, err := h.svc.identityAndMembership(r.Context(), tok)
    if err != nil { http.Error(w, "discord error", http.StatusBadGateway); return }
    if !isMember {
        // AUTH-08 refusal. Do NOT mint a session. Bounce to the SvelteKit NotMemberScreen.
        http.Redirect(w, r, h.frontendBase+"/?notmember=1", http.StatusFound); return
    }
    if err := h.store.UpsertWebUser(r.Context(), user.ID, user.Username, user.Avatar); err != nil { // AUTH-09
        http.Error(w, "internal", http.StatusInternalServerError); return
    }
    rawID, err := h.sessions.Mint(r.Context(), user.ID) // stores sha256(rawID) in web_session
    if err != nil { http.Error(w, "internal", http.StatusInternalServerError); return }
    setSessionCookie(w, rawID) // §2 attribute set
    http.Redirect(w, r, h.frontendBase+"/", http.StatusFound) // back to https://squirebot.quest
}
```

**Rate-limit / error handling** `[CITED: Discord docs — global + per-route buckets]`: Discord returns `429` with a JSON body `{retry_after, global}` and a `Retry-After` header. For a 12-person login flow you will essentially never hit it, but the `getJSON` helper should treat non-200 as an error and a `429` as a transient "try again" surfaced to the user (do not hammer). The existing `internal/backendsrv/enrich/politefetch` already implements Retry-After backoff for the enrichment jobs and is a reference for the pattern — but the OAuth path is interactive, so a single clean error → "Discord is rate-limiting, try again in a moment" is the right UX, not a retry loop.

### §2 — Server-side opaque session + cross-subdomain cookie (D-05)

**The exact cookie attribute set** (use `http.Cookie`, never hand-built strings):
```go
// Source: net/http stdlib + the cross-subdomain analysis below
const sessionCookieName = "sb_session"
const sessionTTL = 30 * 24 * time.Hour // D-05 rolling 30 days

func setSessionCookie(w http.ResponseWriter, rawID string) {
    http.SetCookie(w, &http.Cookie{
        Name:     sessionCookieName,
        Value:    rawID,                  // the OPAQUE id; only its sha256 is in the DB
        Path:     "/",
        Domain:   "squirebot.quest",      // registrable domain → rides to api.squirebot.quest
        HttpOnly: true,                   // JS cannot read it (XSS-resistant; ASVS V3)
        Secure:   true,                   // HTTPS only (Caddy serves TLS — always true in prod)
        SameSite: http.SameSiteLaxMode,   // see WHY below
        MaxAge:   int(sessionTTL.Seconds()),
    })
}
func clearSessionCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name: sessionCookieName, Path: "/", Domain: "squirebot.quest",
        HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
        MaxAge: -1, // delete now
    })
}
```

**WHY `SameSite=Lax` works here (the load-bearing reasoning, D-05):**
- `squirebot.quest` (SvelteKit) and `api.squirebot.quest` (API) share the **registrable domain** `squirebot.quest`. Per the SameSite spec, "same-site" is defined by the registrable domain (eTLD+1), NOT the full origin. So a request from `squirebot.quest` to `api.squirebot.quest` is **same-site** (it is cross-*origin* but same-*site*). SameSite=Lax sends the cookie on same-site requests including these credentialed `fetch`es. `[CITED: MDN SameSite cookies; RFC 6265bis]`
- `Domain=squirebot.quest` scopes the cookie to the registrable domain, so the browser includes it on requests to the apex AND all subdomains (`api.`). Setting `Domain` to the registrable domain is what makes the cookie "ride" to the subdomain. (Note the modern form: the leading-dot is implied; setting `Domain=squirebot.quest` already covers subdomains.)
- SameSite=Lax also **permits the top-level GET navigation** that returns from Discord's OAuth `302` (the callback is a top-level GET, which Lax allows — unlike `Strict`, which would drop the cookie on the cross-site return and is therefore the wrong choice here for any first-set-then-immediately-used scenario).
- **Pitfall — `Secure` requires HTTPS:** fine in prod (Caddy auto-HTTPS). For any local dev over plain `http://localhost`, `Secure` cookies are dropped by the browser — document a dev override (e.g. a `-cookie-insecure` dev flag) if local end-to-end is ever needed. Prod is always HTTPS.
- **Pitfall — `Domain` exact form:** do not set `Domain=api.squirebot.quest` (that would scope it to the API subdomain only and the SvelteKit origin couldn't… actually the SvelteKit origin never needs to *read* it — it's httpOnly — but the browser decides inclusion by the cookie's Domain vs the request host; `Domain=squirebot.quest` is the correct registrable-domain value so the cookie attaches to api.* requests). Set it to the **registrable domain**.
- **Confirm same-site:** `api.squirebot.quest` is unambiguously same-site to `squirebot.quest` (same eTLD+1). If the frontend were ever moved to a *different* registrable domain (e.g. a `*.pages.dev` Cloudflare Pages origin — the original P14 plan before the apex-Caddy switch), SameSite=Lax would NOT send the cookie and you'd need `SameSite=None; Secure` + the exact-origin credentialed CORS. **The apex-Caddy deploy (STATE 2026-05-30) is what makes Lax viable — keep the frontend on `squirebot.quest`.** `[VERIFIED: STATE.md line 26 — apex Caddy deploy, "keeps the P15 same-origin option open"]`

**Session lifecycle (mint / lookup / slide / delete):**
```go
// Source: mirrors guild_code.token_hash (sha256) pattern from 00001_init.sql (VERIFIED)
func (s *Sessions) Mint(ctx context.Context, discordUserID string) (rawID string, err error) {
    rawID, err = randToken() // crypto/rand → base64url, ~32 bytes
    if err != nil { return "", err }
    _, err = s.db.ExecContext(ctx,
        `INSERT INTO web_session (id_hash, discord_user_id, created_at, expires_at)
         VALUES (?, ?, datetime('now'), datetime('now','+30 days'))`,
        sha256bytes(rawID), discordUserID)
    return rawID, err
}

type Session struct{ DiscordUserID string }

func (s *Sessions) Lookup(ctx context.Context, rawID string) (Session, bool) {
    if rawID == "" { return Session{}, false } // fail-closed
    var sess Session
    err := s.db.QueryRowContext(ctx,
        `SELECT discord_user_id FROM web_session
          WHERE id_hash = ? AND expires_at > datetime('now')`,
        sha256bytes(rawID)).Scan(&sess.DiscordUserID)
    if err != nil { return Session{}, false } // sql.ErrNoRows or expired → not authed
    // Rolling TTL (D-05): slide expiry forward on use. Cheap; one UPDATE.
    _, _ = s.db.ExecContext(ctx,
        `UPDATE web_session SET expires_at = datetime('now','+30 days') WHERE id_hash = ?`,
        sha256bytes(rawID))
    return sess, true
}

func (s *Sessions) Delete(ctx context.Context, rawID string) {
    if rawID == "" { return }
    _, _ = s.db.ExecContext(ctx, `DELETE FROM web_session WHERE id_hash = ?`, sha256bytes(rawID))
}
```
Sign-out = `Delete(rawID)` + `clearSessionCookie(w)`. (Expired rows can be reaped lazily on lookup-miss or by the same daily job as the archive — optional housekeeping, not load-bearing.)

### §3 — CORS with credentials (D-05): the exact header set

The 14-03 `CORS` middleware currently sets the exact-origin echo + `Vary: Origin` + `Allow-Methods: GET, OPTIONS` + `Allow-Headers: Content-Type`. The P15 upgrade is **additive** and is exactly the "one-line change" cors.go anticipated, plus widening methods/headers for the write endpoints:

```go
// Source: extends internal/backendsrv/readapi/cors.go (VERIFIED current behavior)
func CORS(allowOrigin string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", allowOrigin)          // EXACT origin, never "*"
        w.Header().Set("Access-Control-Allow-Credentials", "true")          // NEW (D-05) — carries the cookie
        w.Header().Set("Vary", "Origin")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS") // POST added for the write forms
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")       // JSON content-type; no custom auth header (cookie carries auth)
        if r.Method == http.MethodOptions {                                  // preflight: headers above already set
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```
**Critical rules** `[CITED: MDN CORS — credentialed requests]`:
- `Access-Control-Allow-Credentials: true` is REQUIRED on **both** the preflight (OPTIONS) response AND the actual response. The middleware sets it before the OPTIONS short-circuit, so both are covered. ✅
- `Access-Control-Allow-Origin` MUST be the exact origin (`https://squirebot.quest`), NEVER `*`, when credentials are allowed — the browser rejects `*`+credentials. The middleware already echoes the exact origin from the `-cors-origin` flag (default `https://squirebot.quest`). ✅
- The SvelteKit fetches MUST pass `credentials: 'include'` (otherwise the browser won't attach or accept the cookie). See §8.
- **Caddy must not also emit CORS headers** — a duplicated `Access-Control-Allow-Origin` ("origin, origin") makes the browser reject the response. cors.go already flags this deploy-time check. ✅ `[VERIFIED: cors.go comment lines 21-25]`

### §4 — The whoami/session endpoint (UI-SPEC AuthGate)

The UI-SPEC's `AuthGate` polls a session endpoint that returns exactly:
`{ authenticated, isMember, isOfficer, username, avatar, discord_user_id }`. `[VERIFIED: 15-UI-SPEC.md §IA line 121]`

> **Naming caution:** an authed *bearer* `GET /api/v1/whoami` already exists for the watcher (returns `{owner_id, owner_label}` — `[VERIFIED: ingest/whoami.go]`). Do NOT overload it. Use a distinct path for the human session — recommend **`GET /api/v1/session`** (or `/api/v1/me`). The planner should pick the name; this doc uses `/api/v1/session`.

```go
// Source: mirrors whoami.go structure (VERIFIED) but reads the cookie, not the bearer header
// GET /api/v1/session — fail-closed defaults; this is the ONE endpoint that returns 200
// even when unauthenticated (so the AuthGate can branch). All OTHER read endpoints 401.
func (h *SessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    w.Header().Set("Content-Type", "application/json")
    // Fail-closed defaults: everything false/empty unless a valid session proves otherwise.
    out := map[string]any{
        "authenticated": false, "isMember": false, "isOfficer": false,
        "username": "", "avatar": "", "discord_user_id": "",
    }
    c, err := r.Cookie(sessionCookieName)
    if err != nil { _ = json.NewEncoder(w).Encode(out); return } // no cookie → all false
    sess, ok := h.sessions.Lookup(r.Context(), c.Value)
    if !ok { _ = json.NewEncoder(w).Encode(out); return }        // invalid/expired → all false
    // Valid session ⇒ authenticated + member (a session is only ever minted for a member, D-05).
    u, _ := h.store.GetWebUser(r.Context(), sess.DiscordUserID) // username + avatar (AUTH-09)
    out["authenticated"] = true
    out["isMember"] = true
    out["isOfficer"] = h.store.IsAdmin(r.Context(), sess.DiscordUserID) // guild_admins lookup
    out["username"] = u.Username
    out["avatar"] = u.Avatar // hash; frontend builds the CDN URL (or falls back to user glyph)
    out["discord_user_id"] = sess.DiscordUserID
    _ = json.NewEncoder(w).Encode(out)
}
```
**Design note:** because a session is only minted *after* the membership gate passes (§1 callback), "has a valid session" ≡ "isMember == true". `isMember:false` only ever appears for the unauthenticated default. The `NotMemberScreen` (a valid Discord user who is NOT in the guild) is reached via the callback's `?notmember=1` redirect, NOT via a `isMember:false` session — there is no session for a non-member. The UI-SPEC's shape still includes `isMember` for clarity; populate it as above.

### §5 — goose migration `00004_websession_admin.sql` (extend-only, forward-only)

Mirrors the `00001` shape (TEXT snowflakes, INTEGER coins, `TEXT DEFAULT (datetime('now'))`, explicit indexes). `[VERIFIED: 00001_init.sql + 00002_audit.sql conventions]`

```sql
-- Source: proposed; mirrors 00001_init.sql + 00002_audit.sql conventions (VERIFIED)
-- +goose Up
-- 00004 adds the human-login + officer/admin + bank-coin + eviction-grace surface (P15).
-- Forward-only: 00001/00002/00003 are NOT edited; the //go:embed *.sql glob auto-includes
-- this file; goose.Up stays idempotent. (CLAUDE.md: extend-only, forward-only.)

-- AUTH-09: per-user Discord identity, upserted on each login. Snowflake is the stable PK.
CREATE TABLE web_user (
  discord_user_id TEXT PRIMARY KEY,              -- Discord snowflake (string, NOT integer)
  username        TEXT NOT NULL,                 -- current Discord username (re-captured each login)
  avatar          TEXT,                          -- avatar hash, nullable; frontend builds CDN URL
  first_login     TEXT NOT NULL DEFAULT (datetime('now')),
  last_login      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- D-05: opaque server-side session. Store ONLY sha256(rawID); the raw id lives only in the cookie.
CREATE TABLE web_session (
  id_hash         BLOB PRIMARY KEY,              -- sha256(raw session id); 32 bytes (mirrors guild_code.token_hash)
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  expires_at      TEXT NOT NULL                  -- set to datetime('now','+30 days'); slid on use (rolling TTL)
);
CREATE INDEX web_session_user_idx ON web_session(discord_user_id);
CREATE INDEX web_session_expires_idx ON web_session(expires_at);

-- CSRF state for the OAuth round-trip (server-side stored, single-use, short TTL).
CREATE TABLE oauth_state (
  state_hash  BLOB PRIMARY KEY,                  -- sha256(state)
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  expires_at  TEXT NOT NULL                      -- e.g. datetime('now','+10 minutes')
);

-- D-06/D-08: officer allowlist keyed by Discord snowflake (separate table — recommended over an
-- is_admin column, keeps added_by/added_at + a clean role-expansion seam). Append/remove are rows.
CREATE TABLE guild_admins (
  discord_user_id TEXT PRIMARY KEY,              -- an officer; need NOT have a web_user row yet… but
                                                 -- D-07 promotes only logged-in users, so in practice it does
  added_by        TEXT,                          -- the acting officer's snowflake (NULL for the CLI seed)
  added_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

-- D-08: owner-floor = the un-removable maintainer, CLI-seeded once. Singleton row (recommended over
-- config, so it lives in the same backed-up DB and is queryable in-tx alongside guild_admins).
-- Enforce singleton via a fixed id=1 CHECK.
CREATE TABLE owner_floor (
  id              INTEGER PRIMARY KEY CHECK (id = 1),
  discord_user_id TEXT NOT NULL,
  set_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

-- D-11: nullable coin columns on character (the /outputfile format carries no coin; manual entry only).
-- INTEGER, nullable = "not yet recorded" (distinct from 0 = recorded-as-zero).
ALTER TABLE character ADD COLUMN plat   INTEGER;
ALTER TABLE character ADD COLUMN gold   INTEGER;
ALTER TABLE character ADD COLUMN silver INTEGER;
ALTER TABLE character ADD COLUMN copper INTEGER;

-- D-10: eviction grace + archive. grace_until = when the 30-day window ends; archived_at = when the
-- daily job archived the owner's data after grace. Both nullable (NULL = not in eviction lifecycle).
-- RECOMMENDED placement: on OWNER (eviction targets a whole guildie/owner, D-09 — one row per eviction
-- event, not one per character). is_removed stays per-character (00001) for the cascade + the view filter.
ALTER TABLE owner ADD COLUMN evicted_at   TEXT;  -- when the eviction was committed (NULL = active)
ALTER TABLE owner ADD COLUMN grace_until  TEXT;  -- datetime('now','+30 days') at eviction; reversible until then
ALTER TABLE owner ADD COLUMN archived_at  TEXT;  -- set by the daily job once now > grace_until

-- +goose Down
DROP TABLE oauth_state;
DROP TABLE web_session;
DROP TABLE guild_admins;
DROP TABLE owner_floor;
DROP TABLE web_user;
-- NOTE: SQLite (modernc) supports ALTER TABLE DROP COLUMN (3.35+); the Down is best-effort and
-- forward-only migrations rarely run Down in prod. If a Down is authored, DROP the added columns:
ALTER TABLE character DROP COLUMN plat;
ALTER TABLE character DROP COLUMN gold;
ALTER TABLE character DROP COLUMN silver;
ALTER TABLE character DROP COLUMN copper;
ALTER TABLE owner DROP COLUMN evicted_at;
ALTER TABLE owner DROP COLUMN grace_until;
ALTER TABLE owner DROP COLUMN archived_at;
```

**SQLite specifics that bite (verify these in plan):**
- **Snowflakes are TEXT, not INTEGER.** Discord snowflakes are 64-bit but are delivered as JSON *strings* and can exceed JS safe-integer range on the frontend; store/compare as TEXT everywhere. (The frontend never does math on them.)
- **Coins are INTEGER, nullable.** NULL = "not yet recorded" (the bank view's "not yet recorded" affordance keys on NULL); 0 = explicitly recorded zero. Don't `DEFAULT 0` — that would erase the not-recorded distinction the P14 bank view relies on (D-11 replaces the "null/0 placeholder").
- **`COLLATE NOCASE`** is already on `character.name` (00001). Snowflakes are exact-match (no collation needed). Usernames are display-only (no uniqueness constraint — Discord usernames aren't unique; the snowflake is the key).
- **30-day grace = `datetime('now','+30 days')`** at commit; the archive job's due test is `WHERE archived_at IS NULL AND grace_until IS NOT NULL AND datetime('now') > grace_until`. Use `datetime()` comparisons on the TEXT columns (lexicographic compare works because `datetime('now')` is ISO-8601 `YYYY-MM-DD HH:MM:SS`).
- **No `CITEXT`** (Postgres-ism) — `TEXT COLLATE NOCASE` is the SQLite equivalent, already used.

**Reuse `audit_log` (00002), do NOT add a parallel log (D-06).** The existing `audit_log` has `event, char_name, attempting_owner_id, current_owner_id, created_at`. It was designed for the cross-owner-reject path; its columns are a loose fit for admin/eviction events. **Recommended approach:** reuse the table by writing `event` values like `'admin_add'`, `'admin_remove'`, `'eviction'`, `'eviction_archive'`, `'coin_update'`, and put the Discord snowflake into a column. The cleanest extend-only move is to **add two nullable columns** to `audit_log` in 00004 so admin/eviction events have a home without overloading the owner-id columns:
```sql
-- (include in 00004 Up if reusing audit_log for admin/eviction events — RECOMMENDED)
ALTER TABLE audit_log ADD COLUMN actor_discord_id  TEXT;  -- who initiated (officer snowflake)
ALTER TABLE audit_log ADD COLUMN target            TEXT;  -- target snowflake / owner label / char list (JSON)
```
This keeps it a single append-only log (D-06) and stays extend-only. (Alternatively, stuff the snowflake into the existing nullable `char_name`/owner-id columns — uglier; the two added columns are cleaner and still one table.)

### §6 — Ported v1 enforcement SEMANTICS → Go/SQL (the behavioral oracle)

These are extracted **verbatim** from `apps-script/src/lib/admin.ts` + `showEvictionSidebar.ts` (both read this session). Port the *rules*; the *storage* changes from `_meta` JSON to SQL.

**From `admin.ts` (ADMIN-06):**
| v1 rule | Go/SQL port |
|---------|-------------|
| `requireAdminOrThrow(email)` — empty/not-in-list ⇒ throw `'not_authorized'`, fail-closed `[VERIFIED admin.ts:93-99]` | `isAdminTx(ctx, tx, discordID)`; empty/absent ⇒ return `ErrNotAuthorized`. Called as the FIRST statement inside every officer mutator's tx. |
| **Authorize UNDER the lock** to close the TOCTOU/WR-04 window `[VERIFIED admin.ts:165-171,216-217]` | Authorize **inside the `*sql.Tx`** (Pattern 2). The tx is the lock. A just-removed officer's in-flight write fails the in-tx re-check. |
| `addAdmin` idempotent: existing ⇒ `{added:false, alreadyExists:true}`, NO write `[VERIFIED admin.ts:173-176]` | `INSERT … ` guarded by an in-tx existence check (or `INSERT OR IGNORE` + rows-affected); return `alreadyExists` when no row added. |
| `removeAdmin` idempotent: not-in-list ⇒ `{removed:false, notFound:true}`, NO write `[VERIFIED admin.ts:231-237]` | `DELETE …`; rows-affected==0 ⇒ `notFound`. |
| **Owner-floor protection:** `if target===floor && caller!==floor ⇒ throw 'owner_floor_protected'` (checked BEFORE any write) `[VERIFIED admin.ts:221-229]` | In-tx: read `owner_floor.discord_user_id`; if `target == floor && caller != floor` ⇒ return `ErrOwnerFloorProtected`. |
| **Self-removal of floor allowed**, floor row NOT updated (documented orphan pointer) `[VERIFIED admin.ts:241-245]` | If `target == floor && caller == floor`: allow the `guild_admins` DELETE; do NOT touch `owner_floor` (the floor pointer is "who's protected," not "who's currently admin"). |
| Append-only `admin_log` entry per mutation `[VERIFIED admin.ts:179-184,246-251]` | Append to `audit_log` (reuse, §5) inside the same tx. |
| `lock_busy` on contention (tryLock 30000 fails) `[VERIFIED admin.ts:162-163]` | Map a `SQLITE_BUSY`/tx-begin timeout to `ErrLockBusy` → the `lock_busy` UI string. (Rare with one writer; modernc serializes.) |

**From `showEvictionSidebar.ts` + the D-09/D-10 lock (ADMIN-04):**
| v1 rule | Go/SQL port |
|---------|-------------|
| Admin-gate the opener AND every callback, server-side identity, fail-closed `[VERIFIED showEvictionSidebar.ts:60-68,85,116,156]` | Officer-gate the eviction endpoints (list/preview/commit) via `isAdminTx`; the session's `discord_user_id` is the server-side identity (never client-supplied). |
| `getEvictionEmails` = distinct owners with ≥1 non-removed character `[VERIFIED showEvictionSidebar.ts:77-101]` | `SELECT DISTINCT o.id, o.label FROM owner o JOIN character c ON c.owner_id=o.id WHERE c.is_removed=0 AND o.archived_at IS NULL` (label is owner.label or the Discord-linked label — planner picks). |
| `previewEviction(email)` = the char list that will flip + `graceUntil = now+30d` `[VERIFIED showEvictionSidebar.ts:108-140]` | `SELECT name FROM character WHERE owner_id=? AND is_removed=0`; compute `grace_until = datetime('now','+30 days')`. |
| `commitEviction`: cascade `is_removed=TRUE` across the owner's chars, only flip FALSE→TRUE, append `eviction_log` envelope `[VERIFIED showEvictionSidebar.ts:147-230]` | In ONE tx: `UPDATE character SET is_removed=1 WHERE owner_id=? AND is_removed=0`; set `owner.evicted_at=datetime('now')`, `owner.grace_until=datetime('now','+30 days')`; **(D-10 NEW)** `UPDATE guild_code SET disabled_at=datetime('now') WHERE owner_id=? AND disabled_at IS NULL` (revoke the code); append `audit_log` `event='eviction'` with the char list. |
| 30-day grace + auto-archive `[VERIFIED showEvictionSidebar.ts:47 GRACE_MS; weeklyEvictionArchive port]` | `grace_until` column + the daily archive job (§7). |
| Reversible during grace (un-set is_removed + re-mint code) — **D-10 NEW** | A "restore" path (officer-only, in-tx): `UPDATE character SET is_removed=0 WHERE owner_id=?`; clear `owner.evicted_at/grace_until`; re-mint a guild code (reuse the `mint-code` logic). Only valid while `archived_at IS NULL`. (The UI-SPEC doesn't spec a restore form for P15; the *capability* is the reversibility — planner decides whether to surface a button or document it as a CLI/grace-window action. The schema must support it regardless.) |

**Exact error codes/strings the UI-SPEC routes** (return these as a stable machine code in the JSON error body so the frontend maps them) `[VERIFIED: 15-UI-SPEC.md §Copywriting "Admin error routing"]`:
| Machine code | When | UI-SPEC string |
|--------------|------|----------------|
| `owner_floor_protected` | peer tries to remove/evict the floor | "Owner-floor protected — only the maintainer can remove themselves. No changes were written." (admin) / "Owner-floor protected — a peer officer can't evict the maintainer's data." (eviction) |
| `not_authorized` | session is not an officer (or no longer is) | "You're no longer an officer. Please reload." |
| `lock_busy` | tx contention | "Another officer action is in flight. Please retry. No changes were written." |
| `invalid_*` | bad input (e.g. coin out of range, unknown target) | field-level inline errors per the form copy |
| (success shapes) | — | "Officer added: <username>." / "Already an officer: <username>." / "Officer removed: <username>." / "Not in the list: <username>." / eviction "Marked <n> character(s) as removed and revoked the guild code. Grace until <date>." |

**Bank-coin (ADMIN-05/D-11/D-12) — member-gated, bank-toon-only:**
```sql
-- write is gated by: valid session (any member, D-12) + the target is is_bank_toon (D-11).
UPDATE character SET plat=?, gold=?, silver=?, copper=? WHERE id=? AND is_bank_toon=1;
-- rows-affected==0 ⇒ either not found or not a bank toon ⇒ reject (do not silently no-op).
```
Validation (locks the D-11 discretion item, matches UI-SPEC): `plat` = integer `≥0` (no upper bound); `gold`/`silver`/`copper` = integer `0–999` (EQ carries at 1000). Reject negatives/non-integers/out-of-range server-side (never trust the client's `min`/`max`).

### §7 — Eviction archive mechanism (D-10) — recommend the in-process scheduled job

**Recommendation: register a new daily job in the existing in-process Job registry**, exactly as `pigparse_daily` and `wiki_weekly` are registered. The scheduler, the `job_run` cursor (last-run timestamp), the per-job mutex, the immediate-check-on-startup, and the `run-job <name>` CLI entrypoint all already exist `[VERIFIED: ROADMAP 12-05 line + STATE — "db-backed Job registry: pigparse_daily (>=24h) + wiki_weekly (Sunday UTC) with immediate-check-on-startup + advance-always job_run cursor + per-job sync.Mutex; run-job pigparse|wiki entrypoint"]`. Lazy-on-read is rejected: it scatters the grace/archive logic across every read path and makes "archived" non-deterministic.

**Registration pattern (mirrors the two existing jobs):**
```go
// Source: pattern documented for 12-05 scheduler (registry shape ASSUMED from ROADMAP/STATE —
// executor: read internal/backendsrv/scheduler/scheduler.go for the exact Register signature)
// Register a daily job; due predicate ~ now - last_run >= 24h (like pigparse_daily).
sched.Register("eviction_archive", everyInterval(24*time.Hour), func(ctx context.Context) error {
    return adminapi.RunEvictionArchive(ctx, store) // the job body below
})

// adminapi/archive.go — the 30-day-grace archive (port weeklyEvictionArchive's logic).
func RunEvictionArchive(ctx context.Context, s *store.Store) error {
    // Find owners whose grace has expired and aren't yet archived.
    // For each: mark archived_at; (archive policy = what v1's weeklyEvictionArchive did —
    //   v1 moved/flagged the data; in the DB world "archive" can be as simple as setting
    //   archived_at so the views/eviction-list exclude them, OR a hard delete of the owner's
    //   inventory/spellbook rows — port whatever weeklyEvictionArchive.ts actually did).
    rows, err := s.OwnersPastGrace(ctx) // WHERE archived_at IS NULL AND grace_until IS NOT NULL AND datetime('now') > grace_until
    if err != nil { return err }
    for _, ownerID := range rows {
        if err := s.ArchiveOwner(ctx, ownerID); err != nil { // in-tx: set archived_at + (delete/flag data) + audit_log
            // log-but-continue (mirror the enrichment jobs' "one bad item doesn't abort the run")
        }
    }
    return nil
}
```
> **Executor must read `weeklyEvictionArchive.ts`** (I could not re-read it this session due to the tool outage — flagged in the Assumptions Log) to copy the EXACT archive semantics: whether "archive" means (a) set a flag and exclude from views, or (b) physically remove the inventory/spellbook rows while retaining the `eviction_log`/`audit_log` envelope. The schema above supports either via `archived_at`; the job body must match v1's choice. The D-10 lock says "auto-archive" + "reversible during grace" — so archival is the *post-grace* finalization (after `grace_until`, reversibility ends).

### §8 — SvelteKit auth-gated routing (UI-SPEC AuthGate)

**Adapter:** P14 shipped `adapter-static` as an SPA (`200.html` fallback, `npm run build` emits `index.html`+`200.html`) `[VERIFIED: REQUIREMENTS WEB-01 + STATE — "adapter-static SPA"; 14-02-SUMMARY "npm run build emits index.html+200.html"]`. **No SSR** — so session resolution happens client-side on load by calling the backend `session` endpoint with `credentials:'include'`. Confirm `svelte.config.js` uses `adapter-static` with a SPA fallback (I could not re-read it this session — Assumptions Log).

**The gate flow (UI-SPEC §IA resolution order):**
```ts
// Source: SvelteKit SPA load + UI-SPEC AuthGate (api.ts client pattern VERIFIED from P14 api.ts)
// web/src/lib/api.ts — add the session probe + write calls; ALL use credentials:'include'.
const API = import.meta.env.PUBLIC_API_BASE; // e.g. https://api.squirebot.quest  (VERIFIED P14 pattern)

export async function getSession(): Promise<Session> {
  const r = await fetch(`${API}/api/v1/session`, { credentials: 'include' }); // cookie rides (same-site)
  if (!r.ok) return { authenticated: false, isMember: false, isOfficer: false, username: '', avatar: '', discord_user_id: '' };
  return r.json();
}
export async function saveCoin(body: CoinBody) {
  return fetch(`${API}/api/v1/bank-coin`, {
    method: 'POST', credentials: 'include',
    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
  });
}
// listAdmins/addAdmin/removeAdmin/listEvictable/previewEviction/commitEviction follow the same shape.
```

```svelte
<!-- web/src/routes/+layout.svelte (AuthGate wraps SiteShell) — resolution order per UI-SPEC -->
<script lang="ts">
  import { getSession } from '$lib/api';
  let state: 'loading'|'login'|'notmember'|'app' = 'loading';
  let session;
  // On mount: if the callback bounced us with ?notmember=1, show NotMemberScreen directly.
  // Otherwise probe the session.
  onMount(async () => {
    if (new URLSearchParams(location.search).get('notmember') === '1') { state = 'notmember'; return; }
    session = await getSession();
    state = !session.authenticated ? 'login' : 'app';
  });
</script>
{#if state === 'loading'}   <StateBlock kind="auth-loading" />
{:else if state === 'login'}<LoginScreen />            <!-- "Sign in with Discord" -->
{:else if state === 'notmember'}<NotMemberScreen />    <!-- AUTH-08 refusal -->
{:else}                      <SiteShell {session}><slot /></SiteShell>
{/if}
```

**Critical interaction details (UI-SPEC):**
- **The login button is a FULL NAVIGATION, not a fetch.** `<a href="https://api.squirebot.quest/auth/login">` (or `window.location.href = ...`). The OAuth flow is a series of cross-origin top-level redirects (SvelteKit → backend → Discord → backend → SvelteKit) that a `fetch` cannot follow. `[VERIFIED: UI-SPEC §LoginScreen Interaction "navigates to the backend's /auth/login style route"]`
- **The callback finally `302`s the browser back to `https://squirebot.quest/`** (or `/?notmember=1`). The SvelteKit app re-mounts, the cookie is now set, `getSession()` returns authenticated → the app renders.
- **`401` from any read endpoint ⇒ route to LoginScreen; `403`/`not_authorized` from an officer endpoint ⇒ collapse the Admin UI + show the "you're no longer an officer" message.** Frontend gating is UX only; the server is the boundary (D-01). `[VERIFIED: UI-SPEC §A11y "Auth state is server-truth"]`
- **Officer nav entry renders only when `session.isOfficer`** (Layer 1 UX suppression); the server re-checks on every officer endpoint (Layer 2 boundary). A non-officer typing `/admin` sees an "Officers only" `StateBlock`, and the endpoint 403s. `[VERIFIED: UI-SPEC §IA two-layer contract]`
- **Sign-out** = `POST /auth/logout` with `credentials:'include'` → backend deletes the row + clears the cookie → frontend routes to LoginScreen.

### §9 — Security (ASVS L1, block-on-high)

This phase introduces the project's **first authenticated session, first destructive web actions, and first secret-on-the-server**. The threats + concrete mitigations the planner MUST bake in:

| Threat | STRIDE | Concrete mitigation (where) |
|--------|--------|----------------------------|
| **OAuth CSRF** (forged callback) | Spoofing/Tampering | `state` param: `crypto/rand` → store `sha256(state)` server-side with short TTL → verify+consume on callback (§1). Reject missing/unknown state with 400. |
| **Session theft via XSS** | Info disclosure | `HttpOnly` cookie (JS can't read it) + Svelte `{}` auto-escape on all Discord usernames, never `{@html}` (§2, UI-SPEC). |
| **Session theft on the wire** | Info disclosure | `Secure` cookie (HTTPS only; Caddy) + opaque id (no PII/claims in the token) (§2). |
| **Session fixation** | Spoofing | Mint a FRESH random session id at login (§1 callback); never accept a client-supplied session id. |
| **Predictable session id** | Spoofing | `crypto/rand` ~32 bytes (§2). Store only the hash; constant-time compare on lookup. |
| **CSRF on the write forms** | Tampering | **Recommendation below** — SameSite=Lax + JSON-content-type + credentialed-CORS-exact-origin is the baseline; an explicit CSRF token is NOT required at this scale. See the analysis. |
| **Authorization bypass / TOCTOU** | Elevation of privilege | Server is the boundary; authorize **inside** the write tx (D-06, §6, Pattern 2). Frontend gating is UX only. |
| **Destructive eviction/admin-removal** | Tampering/Repudiation | Confirm-before-commit (ConfirmDialog, UI-SPEC); owner-floor protection (§6); append-only `audit_log` for non-repudiation (§5). |
| **Secret leakage** (client secret in bundle) | Info disclosure | Client secret is a backend-only env/systemd var; NEVER a `PUBLIC_*` SvelteKit var (those inline into the client bundle). The code-exchange is server-side only (§1). |
| **Membership bypass** | Elevation | Membership is evaluated server-side from `/users/@me/guilds` against the env `GUILD_ID`; a session is only minted on pass (§1). Re-checked at each login (D-05) — a departed guildie loses access when their bounded session expires + can't re-login. |
| **Open redirect on callback** | Tampering | The callback redirects to a SERVER-CONFIGURED frontend base URL, never a URL from the request/query. |

**CSRF on write forms — clear recommendation:** **Do NOT add a separate CSRF token for P15.** The combination is sufficient at this scale and threat model:
1. **SameSite=Lax** stops the classic CSRF vector — a cross-*site* attacker page on `evil.com` cannot get the browser to attach the `squirebot.quest` cookie to a `POST` (Lax only sends on same-site requests + top-level GET navigations; a cross-site `POST` from a form/fetch on evil.com does NOT carry the cookie).
2. **Credentialed CORS with exact-origin echo** means even a cross-site `fetch` with `credentials:'include'` from `evil.com` is blocked by the browser (the preflight/response only allows `https://squirebot.quest`).
3. **JSON content-type requirement** (`Content-Type: application/json`) means the writes are non-"simple" requests that always trigger a CORS preflight — a `<form>`-based CSRF (which can only send simple content-types) can't invoke them at all; reject any non-JSON content-type server-side as defense-in-depth.
This is the standard modern "SameSite + CORS + JSON" CSRF defense for a same-site SPA+API. (If the guild later wanted belt-and-suspenders, a double-submit CSRF token is the cheap add — but it is **not** required to clear ASVS L1 here given SameSite=Lax + exact-origin credentialed CORS.) **Recommendation: ship SameSite+CORS+JSON; document the double-submit token as a deferred hardening option.**

**ASVS categories applicable to this stack:**
| ASVS Category | Applies | Standard control (in this phase) |
|---------------|---------|----------------------------------|
| V2 Authentication | yes | Discord OAuth2 (delegated); no local passwords. State param for the flow. |
| V3 Session Management | yes | Opaque server-side session, httpOnly+Secure cookie, 30-day rolling TTL, server-side revoke on logout (§2). |
| V4 Access Control | yes | Officer allowlist re-checked in-tx; owner-floor lockout; member-vs-officer endpoint split (§6). Fail-closed. |
| V5 Input Validation | yes | Coin bounds; reject non-JSON content-type; validate target snowflakes exist; never trust client min/max. |
| V6 Cryptography | yes | `crypto/rand` (session/state), `crypto/sha256` (hash-at-rest), `crypto/subtle` (constant-time). Never hand-roll. |
| V7 Error/Logging | yes | NEVER log the session id, the OAuth code/token, or the client secret (mirror whoami.go's "never log the token"); DO append `audit_log` for admin/eviction (non-repudiation). |
| V13 API/Web Service | yes | Exact-origin credentialed CORS; preflight handling; method-checked handlers. |

### §10 — Maintainer Prerequisites (Discord app provisioning) — FLAG LOUDLY

> **This is a non-code, human-only prerequisite that BLOCKS end-to-end functioning, exactly like the Hetzner/domain prereqs in prior phases. The planner MUST surface it as an explicit setup step (not a build task) and the executor cannot complete it autonomously.** `[VERIFIED: CONTEXT Deferred Ideas line 139 — "flag in research so the planner surfaces it before build"]`

Before Discord login works end-to-end, the maintainer (boejowen) must, at the **Discord Developer Portal** (https://discord.com/developers/applications):

1. **Create a Discord application** (free, instant — no review, no brand verification; this is the whole point of choosing Discord over Google, per finding 03 §4.2).
2. **Add an OAuth2 redirect URI** = the backend callback, e.g. **`https://api.squirebot.quest/auth/callback`** (must match `RedirectURL` in the `oauth2.Config` byte-for-byte; Discord rejects mismatches). Register it under the app's OAuth2 → Redirects.
3. **Obtain `CLIENT_ID` + `CLIENT_SECRET`** (OAuth2 → General). The **secret goes ONLY into the backend** as an env var / systemd `Environment=` / EnvironmentFile (the same secret-handling posture as the existing deploy; never in the repo, never in the static bundle, never in CI). `CLIENT_ID` is not secret (it's in the authorize URL) but configure both as env for symmetry.
4. **Scopes are set at request time** (the `scope=identify guilds` in the authorize URL — §1), NOT in the portal; no portal scope config needed. **No bot, no bot token, no privileged intents.** (Do NOT add a bot to the app — D-02 needs none.)
5. **`GUILD_ID`** (the guild's Discord server snowflake) — the maintainer obtains it from Discord directly (enable Developer Mode in Discord → right-click the guild → "Copy Server ID"). It is configured as a backend env var; the membership gate tests for its presence in `/users/@me/guilds`.

**Backend env vars this phase introduces (all backend-only):**
`DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET`, `DISCORD_REDIRECT_URL` (=`https://api.squirebot.quest/auth/callback`), `DISCORD_GUILD_ID`, `FRONTEND_BASE_URL` (=`https://squirebot.quest`, for the post-callback redirect). Wire them through the server flags/env the same way `-cors-origin` is wired today.

**Plus the one-time CLI ops step (D-08):** after deploy, the maintainer runs `squirebot-server set-owner-floor <their-discord-snowflake>` once — this seeds the owner-floor row AND the first/bootstrap admin (replaces v1's gone `onOpen`/`getOwner()` bootstrap). The maintainer must sign in once first (or the subcommand can insert the `guild_admins` + `owner_floor` rows directly by snowflake without requiring a prior `web_user` row — recommend the latter so the floor is set before anyone logs in; the `web_user` row fills on the maintainer's first login).

## Common Pitfalls

### Pitfall 1: SameSite=Lax silently drops the cookie if the frontend leaves the registrable domain
**What goes wrong:** Move the SvelteKit app to `*.pages.dev` (the original P14 plan) and the session cookie stops riding to `api.squirebot.quest` — login appears to work but every subsequent API call is unauthenticated.
**Why it happens:** SameSite is keyed on the registrable domain (eTLD+1). `pages.dev` ≠ `squirebot.quest`.
**How to avoid:** Keep the frontend on the apex `squirebot.quest` (the STATE 2026-05-30 Caddy-apex deploy already does this and explicitly notes it "keeps the P15 same-origin option open"). If the host ever changes, switch to `SameSite=None; Secure`.
**Warning sign:** `getSession()` returns authenticated:false immediately after a successful Discord login.

### Pitfall 2: `Access-Control-Allow-Credentials` missing on the preflight
**What goes wrong:** The actual `POST` is blocked even though the simple GET worked, because the browser's preflight (OPTIONS) didn't see `Allow-Credentials: true`.
**Why it happens:** Setting the credentials header only on the non-OPTIONS path.
**How to avoid:** Set it BEFORE the OPTIONS short-circuit (the §3 middleware does). Both preflight and actual response carry it.
**Warning sign:** Reads work, writes fail with a CORS error in the browser console mentioning credentials.

### Pitfall 3: Caddy double-emits CORS headers
**What goes wrong:** `Access-Control-Allow-Origin: https://squirebot.quest, https://squirebot.quest` → browser rejects (must be a single value).
**Why it happens:** Both the Go middleware AND a Caddy `header` directive set CORS.
**How to avoid:** CORS is set ONCE, in Go. Verify the Caddyfile's reverse_proxy block adds no CORS headers (cors.go already documents this deploy check). `[VERIFIED: cors.go lines 21-25]`
**Warning sign:** Duplicated header value in the network tab.

### Pitfall 4: Authorizing before the transaction (TOCTOU)
**What goes wrong:** A just-removed officer's in-flight request passes an outside-the-tx admin check, then writes.
**Why it happens:** Checking `isAdmin` before `BeginTx`.
**How to avoid:** Authorize INSIDE the tx (Pattern 2 / §6). This is the entire point of v1's WR-04 fix — port it.
**Warning sign:** Two near-simultaneous "remove officer X" + "X does an admin action" both succeed.

### Pitfall 5: Storing the raw session id (or raw state) in the DB
**What goes wrong:** A DB read (backup leak, SQL injection elsewhere) yields live session tokens.
**Why it happens:** Convenience — storing the value you compare against.
**How to avoid:** Store `sha256(rawID)` only; the raw id lives solely in the cookie. Mirror `guild_code.token_hash`. `[VERIFIED: 00001 line 52]`
**Warning sign:** `web_session.id_hash` column holds a base64 string instead of a 32-byte BLOB.

### Pitfall 6: Coin `DEFAULT 0` erases the "not recorded" distinction
**What goes wrong:** The bank view can't tell "0 plat recorded" from "no one has entered coin yet," breaking the D-11 replacement of the "not yet recorded" affordance.
**Why it happens:** Reflexively adding `NOT NULL DEFAULT 0`.
**How to avoid:** Coin columns are NULLABLE, no default. NULL = not recorded; 0 = recorded zero.
**Warning sign:** Every bank toon shows "0/0/0/0" immediately after the migration instead of "not yet recorded."

### Pitfall 7: Snowflake as INTEGER
**What goes wrong:** Large snowflakes lose precision (esp. if ever round-tripped through JS `number`); comparisons fail.
**Why it happens:** "It's a 64-bit id, INTEGER seems right."
**How to avoid:** Snowflakes are TEXT everywhere (Discord delivers them as JSON strings). `[CITED: Discord docs — IDs are strings]`
**Warning sign:** Officer lookups intermittently fail for users with high snowflakes.

## Runtime State Inventory

> This is an additive/greenfield phase (new tables, new endpoints) — not a rename/refactor. Included for completeness because it touches deploy-time config + a live service.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | New `web_user`/`web_session`/`oauth_state`/`guild_admins`/`owner_floor` tables + new `character` coin cols + `owner` grace cols, all via `00004` goose. No existing data is renamed. `is_removed`/`disabled_at`/`is_bank_toon` columns ALREADY EXIST (00001) and are reused. | `goose.Up` on deploy (runs on startup). No data migration — additive only. |
| Live service config | The live `squirebot-server` on the Hetzner VPS gains new env vars (`DISCORD_*`, `FRONTEND_BASE_URL`); the `-cors-origin` flag stays `https://squirebot.quest`. The systemd unit / EnvironmentFile must add the Discord secret. | Ops: add env vars to the systemd unit (maintainer step, §10). Redeploy binary (deploy = drop binary + restart). |
| OS-registered state | None — no Task Scheduler / systemd-timer changes (the archive job is in-process via the existing scheduler, not a new OS timer). | None — verified by §7 (in-process Job registry). |
| Secrets/env vars | `DISCORD_CLIENT_SECRET` is NEW (backend-only). The Discord app + GUILD_ID are provisioned at the Discord portal (§10). | Maintainer creates the Discord app + sets env (§10). Never in repo/CI/bundle. |
| Build artifacts | `go.mod`/`go.sum` change (re-add `golang.org/x/oauth2` + its x/net/x/crypto deps). The static `web/` build gains new components but no new npm dep. | `go mod tidy` after `go get x/oauth2`; `npm run build` for the frontend. |

**The canonical question — after every file is updated, what runtime systems still need attention?** The Discord application (must exist + redirect URI registered + secret in the server env) and the one-time `set-owner-floor` CLI run. Both are §10 maintainer steps; neither is code the executor can do.

## State of the Art

| Old Approach (v1, Sheet/Apps Script) | Current Approach (P15, Go/DB) | When Changed | Impact |
|--------------------------------------|-------------------------------|--------------|--------|
| `_meta.guild_admins` JSON row | `guild_admins` SQL table | P15 | Discrete rows, real `added_by`/`added_at`, queryable in-tx |
| `_meta.workbook_owner_floor` string | `owner_floor` singleton row, CLI-seeded | P15 (D-08) | No `onOpen`/`getOwner()` bootstrap (that API is gone); explicit deploy step |
| `LockService.getDocumentLock().tryLock(30000)` | one `*sql.Tx` (BEGIN…COMMIT) | P15 | The tx IS the lock AND the TOCTOU envelope; simpler + stronger |
| `_meta.eviction_log` JSON envelope | `audit_log` rows (event='eviction') + `owner.grace_until`/`archived_at` | P15 (D-10) | Append-only relational log; grace is a column, not a JSON field |
| Eviction = mark removed + separate Google-Drive un-share | Eviction = one tx: cascade is_removed + revoke guild_code + set grace | P15 (D-10) | "One clean app-controlled action" — the DB-world win the roadmap Note calls out |
| Identity = Google `userinfo.email` → `_char_owner` | Discord OAuth `identify`/`guilds` + opaque session | P15 (AUTH-08/09) | No Google, no brand-verification gate; snowflake pre-pays the v2 pinger |
| `weeklyEvictionArchive` time-driven trigger (6-min cap, resumable cursor) | `eviction_archive` in-process daily job (no cap, no cursor workaround) | P15 (D-10/§7) | Single uninterrupted run via the existing Job registry |

**Deprecated/outdated for this phase:** the entire Apps Script admin/eviction sidebar stack is the *behavioral oracle only* — it is NOT a build target and is decommissioned in P16. Do not `clasp push` anything.

## Assumptions Log

> Claims I could not directly verify THIS session due to a sustained tool-execution outage (I read the file's siblings / it's documented in ROADMAP/STATE, but did not re-open the exact file). The executor should confirm each by reading the cited file before relying on the exact symbol name. None of these change the design — they're symbol-name / signature confirmations.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `auth.ResolveToken(ctx, authHeader) (ownerID int64, ok bool)` is the bearer-guard signature, and `auth.New(db) *auth.Auth` constructs it. | §0/Pattern1 | LOW — verified directly in `ingest/whoami.go` THIS session (lines 49, 66). High confidence; listed only because the `auth/guard.go` body itself wasn't re-read. |
| A2 | The scheduler exposes a `Register(name, duePredicate, jobFunc)`-style API with a `job_run` cursor + per-job mutex + `run-job <name>` CLI, registering `pigparse_daily`/`wiki_weekly`. | §7 | MEDIUM — documented verbatim in ROADMAP 12-05 + STATE, but the exact `Register` signature wasn't re-read from `scheduler/scheduler.go`. Executor: read it before writing the registration call. |
| A3 | `cmd/squirebot-server/main.go` has `serve`/`mint-code`/`revoke-code` subcommands and runs `goose.Up` on startup; `set-owner-floor` lands beside them. | §10 | MEDIUM — documented in ROADMAP 11-05/CONTEXT + STATE; the main.go body wasn't re-read. Executor: confirm the subcommand-dispatch shape. |
| A4 | `web/svelte.config.js` uses `@sveltejs/adapter-static` as an SPA (200.html fallback), so client-side session resolution on load is the right pattern (no SSR). | §8 | MEDIUM — REQUIREMENTS WEB-01 + 14-02-SUMMARY say "adapter-static SPA" + "emits index.html+200.html"; svelte.config.js wasn't re-read. If it were SSR, the gate could move to a server `load` — but the static-SPA assumption matches all P14 evidence. |
| A5 | `web/src/lib/api.ts` fetches over `import.meta.env.PUBLIC_API_BASE`; adding `credentials:'include'` + new calls follows the existing shape. | §8 | LOW — REQUIREMENTS/ROADMAP/STATE all cite `api.ts over PUBLIC_API_BASE` repeatedly; the file wasn't re-read this session. |
| A6 | `weeklyEvictionArchive.ts`'s archive = set-a-flag-and-exclude vs hard-delete the owner's rows. | §7 | MEDIUM — I read `showEvictionSidebar.ts` (the eviction half) but the tool outage blocked `weeklyEvictionArchive.ts`. Executor MUST read it to copy the exact archive policy; the schema (`archived_at`) supports either. |
| A7 | `golang.org/x/oauth2` is currently ABSENT from the server's go.mod and must be `go get`-added. | Stack | LOW — verified directly: go.mod (read this session) lists no `golang.org/x/oauth2` (P13 deleted the watcher's copy). High confidence. |
| A8 | `showAdminMgmtSidebar.ts` error-routing strings match the UI-SPEC's "Admin error routing" table (owner_floor_protected/not_authorized/lock_busy). | §6 | LOW — the strings are quoted verbatim in 15-UI-SPEC.md (read this session); the sidebar file itself wasn't re-read, but the UI-SPEC is the authoritative contract the frontend routes against. |

## Open Questions

1. **Owner ↔ Discord linkage for eviction labeling.**
   - What we know: eviction targets an `owner` (D-09); `owner` has `id`+`label` (00001); the human session is keyed by `discord_user_id` (web_user). The watcher's guild code maps to an `owner` (guild_code.owner_id).
   - What's unclear: how the eviction form LABELS a guildie — by `owner.label`, or by a Discord-linked name. There is no FK linking `owner` ↔ `web_user` today (an owner is a watcher identity; a web_user is a login identity; they may belong to the same human but aren't joined).
   - Recommendation: for P15, label evictable guildies by `owner.label` (what v1 effectively did via owner email). The Discord↔owner join is a P16 backfill concern (CUTOVER-02 imports owner/character metadata). Don't invent the join in P15; the eviction list is `owner`-based. Flag for the planner.

2. **`set-owner-floor` before any login.**
   - What we know: D-08 seeds the floor by snowflake at deploy; D-07 promotes only logged-in users.
   - What's unclear: whether the floor row + bootstrap `guild_admins` row can be inserted by snowflake before that user has a `web_user` row.
   - Recommendation: yes — insert directly into `owner_floor` + `guild_admins` by snowflake (no `web_user` FK requirement on `guild_admins`; the `web_user` row fills on first login). The `guild_admins` table above intentionally does NOT FK to `web_user` for this reason. Confirm in the DDL.

3. **Restore-during-grace UX surface.**
   - What we know: D-10 mandates the *capability* (reversible during grace); the UI-SPEC specs eviction + the forms but no explicit "restore" form.
   - Recommendation: ship the schema + a server endpoint that supports restore (un-set is_removed + clear grace + re-mint code); surface it minimally (e.g. the eviction list could show grace-window owners with a "Restore" action) OR document it as a CLI/grace-window action for P15 and a fuller UI later. Planner decides the surface; the backend capability is required.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | building the server | ✓ | 1.25.7 `[VERIFIED: go.mod]` | — |
| `modernc.org/sqlite` | the store | ✓ | v1.51.0 `[VERIFIED: go.mod]` | — |
| `goose/v3` | migrations | ✓ | v3.27.1 `[VERIFIED: go.mod]` | — |
| `golang.org/x/oauth2` | Discord flow | ✗ (must add) | — | none needed — `go get` adds it (network at build time) |
| Hetzner VPS + Caddy auto-HTTPS | live deploy, Secure cookies, TLS | ✓ | live at api.squirebot.quest / squirebot.quest `[VERIFIED: STATE]` | — |
| Discord application (client id/secret/redirect/guild id) | OAuth login end-to-end | ✗ (maintainer must create) | — | **NO fallback — blocks AUTH-08 end-to-end** (§10) |
| `npm` + SvelteKit toolchain | frontend build | ✓ | as P14 | — |

**Missing dependencies with no fallback:**
- **Discord application provisioning** (§10) — blocks login end-to-end. Maintainer-only, non-code. Planner MUST surface as a setup prerequisite.

**Missing dependencies with fallback:**
- `golang.org/x/oauth2` — `go get golang.org/x/oauth2@latest` + `go mod tidy` (standard, no risk).

## Security Domain

(Full threat table + ASVS mapping + the CSRF recommendation are in **§9** above — not duplicated here. Summary: ASVS V2/V3/V4/V5/V6/V7/V13 all apply; the standout decisions are (a) opaque hashed server-side sessions with httpOnly+Secure+SameSite=Lax cross-subdomain cookies, (b) authorize-inside-the-tx for officer writes, (c) state-param OAuth CSRF, and (d) SameSite+exact-origin-credentialed-CORS+JSON-content-type as the write-form CSRF defense with NO separate token required at this scale.)

## Sources

### Primary (HIGH confidence)
- **Codebase (read this session):** `internal/backendsrv/migrations/00001_init.sql`, `00002_audit.sql`; `internal/backendsrv/ingest/whoami.go`; `internal/backendsrv/readapi/cors.go`; `apps-script/src/lib/admin.ts`; `apps-script/src/triggers/showEvictionSidebar.ts`; `go.mod` — the authoritative shapes for schema, handler conventions, CORS upgrade path, and the v1 enforcement oracle.
- **Planning docs (read this session):** `15-CONTEXT.md` (D-01..D-12), `15-UI-SPEC.md` (session shape, AuthGate flow, error strings), `15-DISCUSSION-LOG.md`, `REQUIREMENTS.md`, `ROADMAP.md` (Phase 15 detail + scheduler 12-05 + CLI 11-05), `STATE.md` (apex-Caddy deploy + scheduler registry), `CLAUDE.md` (conventions/never-list).
- `.planning/explorations/website-milestone/03-watcher-auth.md` §3–§4 — Discord OAuth2 scopes (`identify`+`guilds`), membership-via-guilds-list, the definitive "no brand-verification / app-review gate" confirmation, Google-rejected rationale.

### Secondary (MEDIUM confidence — verified against official source)
- Discord OAuth2 docs (https://docs.discord.com/developers/topics/oauth2) via web search — authorize URL `https://discord.com/oauth2/authorize`, token URL `https://discord.com/api/oauth2/token`, scopes, the `state` CSRF parameter ("highly recommend"), client_id+client_secret in the token form body. Cross-verified with 03-watcher-auth.md.
- MDN — SameSite cookies (registrable-domain definition of same-site; Lax permits top-level GET + same-site subresource), CORS credentialed-request rules (exact-origin required, `Allow-Credentials` on preflight+response). `[CITED]`

### Tertiary (LOW confidence — pattern documented, file not re-read this session)
- Scheduler `Register`/`run-job` exact signature, `cmd/squirebot-server` subcommand dispatch, `svelte.config.js` adapter, `api.ts` shape, `weeklyEvictionArchive.ts` archive policy — documented in ROADMAP/STATE/REQUIREMENTS but not directly re-opened (tool outage). See Assumptions Log A2/A3/A4/A5/A6.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — go.mod verified; x/oauth2 locked by D-04; stdlib crypto mirrors the existing guild-code/bearer patterns.
- Architecture / OAuth mechanics: HIGH — Discord endpoints verified; cookie/CORS cross-subdomain reasoning verified against MDN + the existing cors.go upgrade note.
- Schema (00004 DDL): HIGH — mirrors the verified 00001/00002 conventions; SQLite specifics (TEXT snowflakes, nullable coins, datetime grace) confirmed.
- Ported enforcement (admin/eviction): HIGH — extracted verbatim from `admin.ts` + `showEvictionSidebar.ts` (both read this session); error strings cross-checked against the UI-SPEC.
- A few backend internals (scheduler/main/svelte.config/api.ts/weeklyEvictionArchive): MEDIUM — documented but not re-read this session; flagged in the Assumptions Log for executor confirmation (none change the design).

**Research date:** 2026-05-30
**Valid until:** ~2026-06-29 (30 days — stable stack; Discord OAuth2 endpoints are long-stable; the only fast-moving risk is the resolved x/oauth2 patch version, which is non-breaking).
