---
phase: 15-admin-web-forms-login
plan: 02
subsystem: auth
tags: [discord-oauth2, oauth2, sessions, cookies, cors, csrf, middleware, cli, go]

# Dependency graph
requires:
  - phase: 15-admin-web-forms-login (plan 01)
    provides: "store session/officer/owner-floor methods (GenerateSessionID, HashSession, SessionTTLSeconds, UpsertWebUser, GetWebUser, CreateSession, ResolveSession, TouchSession, DeleteSession, IsOfficer, SetOwnerFloor) + the 00004 web_auth schema"
  - phase: 14-web-frontend
    provides: "the read API (4 views + meta) + exact-origin CORS the session gate now wraps; the bank view that will consume coin in 15-03"
  - phase: 11-backend-foundation-ingest-api
    provides: "net/http ServeMux handler conventions (whoami.go), crypto/rand mint idiom (auth/mint.go), the run() subcommand dispatch + splitFlagsAndPositionals/openMigratedDB CLI helpers, politefetch's bounded-read net/http pattern"
provides:
  - "internal/backendsrv/webauth package: hand-rolled Discord OAuth2 (golang.org/x/oauth2) authorize-URL build + server-side code exchange + /users/@me + /users/@me/guilds + fail-closed IsGuildMember (AUTH-08)"
  - "opaque-session cookie helpers (SetSessionCookie/ClearSessionCookie: HttpOnly+Secure+SameSite=Lax+Domain=squirebot.quest, 30d) + RequireSession (D-01 read gate, fail-closed 401 + rolling TouchSession) + RequireOfficer (403 not_authorized)"
  - "login/callback/whoami-web/logout handlers — state-CSRF guard, regenerate-on-login, W-4 hardcoded-origin redirect, {authenticated,isMember,isOfficer,username,avatar,discord_user_id} AuthGate feed"
  - "CORS credential-aware upgrade (Access-Control-Allow-Credentials:true + POST), still exact-origin (never wildcard)"
  - "squirebot-server set-owner-floor <discord-id> CLI (D-08): seeds the owner-floor + bootstrap officer"
affects: [15-03, 15-04, 15-05, 16-cutover]

# Tech tracking
tech-stack:
  added: ["golang.org/x/oauth2 v0.36.0 (direct, server-side only — the watcher shed its oauth2 tree in P13)"]
  patterns:
    - "Test-overridable external endpoints: package-var seams (discordAuthURL/discordTokenURL/discordAPIBase, webOrigin) repointed at httptest servers via setXForTest helpers, restored in t.Cleanup — the whole OAuth flow is unit-tested with zero live credentials"
    - "Handler factory shape func(db *sql.DB, cfg Config) http.HandlerFunc closing over CookieOptsFromEnv() — mirrors the ingest NewWhoami(guard, db) convention; method-check first; JSON {\"error\":\"code\"} errors"
    - "Two-layer officer authorization: RequireOfficer is the cheap REQUEST-TIME outer gate; the 15-03 *Tx mutators still re-check IsOfficer INSIDE their tx (TOCTOU) — the outer gate is not a substitute for the in-tx re-check"
    - "io.LimitReader-bounded Discord identity/guilds reads (1 MB) mirroring politefetch + the ingest MaxBytesReader discipline"

key-files:
  created:
    - internal/backendsrv/webauth/oauth.go
    - internal/backendsrv/webauth/oauth_test.go
    - internal/backendsrv/webauth/session.go
    - internal/backendsrv/webauth/session_test.go
    - internal/backendsrv/webauth/handlers.go
    - internal/backendsrv/webauth/handlers_test.go
    - cmd/squirebot-server/ownerfloor.go
  modified:
    - internal/backendsrv/readapi/cors.go
    - internal/backendsrv/readapi/readapi_test.go
    - cmd/squirebot-server/main.go
    - cmd/squirebot-server/main_test.go
    - go.mod
    - go.sum

key-decisions:
  - "ALL OAuth/session/handler code in ONE webauth package (per the PLAN), not the RESEARCH's oauth/websession/webauth split — fewer packages, the store already owns the SQL; routes are /api/v1/auth/* (the PLAN's shape), not the RESEARCH's bare /auth/*"
  - "webOrigin is a package var defaulting to the hardcoded https://squirebot.quest (env SQUIREBOT_WEB_ORIGIN override) and is the ONLY source of the callback redirect Location — the handler never reads a request redirect/return_to/next param (W-4 / T-15-13)"
  - "ConfigFromEnv tolerates empty DISCORD_* vars so a build/CI/local run starts fine; the live login is the only thing that needs the real values (deferred to deploy)"
  - "SQUIREBOT_COOKIE_INSECURE=1 is the only way to drop Secure (local-http dev); prod never sets it, so Secure stays true"
  - "whoami-web is the ONE read endpoint that returns 200 when unauthenticated (the AuthGate branch feed); every OTHER read route is 401 without a session"

patterns-established:
  - "setXForTest endpoint seams: external base URLs are package vars repointed at httptest, never a live dependency in unit tests"
  - "Comment-hygiene for grep-gates: cors.go says 'never a wildcard' (the word) instead of the literal so the no-wildcard-literal acceptance gate passes while the doc still reads clearly"

requirements-completed: [AUTH-08, AUTH-09]

# Metrics
duration: 14min
completed: 2026-05-30
---

# Phase 15 Plan 02: Discord OAuth2 Login + Opaque Session + CORS-Creds + set-owner-floor Summary

**Hand-rolled Discord OAuth2 authorization-code login (golang.org/x/oauth2, server-side exchange, fail-closed guild-membership gate) backed by an opaque httpOnly+SameSite=Lax cross-subdomain session, the read API walled behind RequireSession (D-01), credential-aware exact-origin CORS, and the set-owner-floor CLI — the project's first authenticated surface, built + unit-tested entirely against httptest fakes.**

## Performance

- **Duration:** ~14 min
- **Started:** 2026-05-30T20:42:49 (Task 1 commit; coding began earlier)
- **Completed:** 2026-05-30T20:56:18 (Task 4 commit)
- **Tasks:** 4 (+ 1 pre-resolved checkpoint, see below)
- **Files modified:** 13 (7 created + 6 modified/extended)

## Checkpoint Resolution (pre-resolved "build-only")

The plan opens with a `checkpoint:human-action` task — the maintainer Discord-app provisioning prerequisite. Per the user's standing Phase 15 directive (STATE.md, 2026-05-30) the answer was pre-resolved to **"build-only"**: the maintainer HAS provisioned the Discord app (CLIENT_ID / CLIENT_SECRET / GUILD_ID in hand, redirect URI `https://api.squirebot.quest/api/v1/auth/callback` registered), but the live values are NOT entered this run — they go on the box via systemd at deploy time only. So this executor did NOT block; it built + unit-tested Tasks 1–4 against httptest fakes (every test points the Discord token/identity/guilds endpoints + the web origin at httptest servers — no live credentials needed). **The live end-to-end login smoke is DEFERRED to deploy-time** (see Next Phase Readiness).

## Accomplishments

- **Hand-rolled Discord OAuth2 (D-02/D-04, AUTH-08).** `oauth.go` builds the authorize URL with scopes `identify`+`guilds` exactly, does the server-side code exchange (client secret backend-only, never logged), fetches `/users/@me` (snowflake/username/avatar — AUTH-09) and `/users/@me/guilds`, and `IsGuildMember` is **fail-closed**: an empty configured id or empty/nil guild list → false; a non-member gets no session, with no allowlist. The whole flow is exercised against httptest fakes via package-var endpoint seams.
- **Opaque cross-subdomain session + read-API gate (D-01/D-05, T-15-11).** `session.go` sets the `sb_session` cookie HttpOnly+Secure+SameSite=Lax+Domain=squirebot.quest (rides the apex → `api.` subdomain, same registrable domain) with a 30-day MaxAge; `RequireSession` is the D-01 read gate — fail-closed 401 without a valid session, rolling `TouchSession` on success, discord_user_id into context; `RequireOfficer` 403s a non-officer. **All five read routes** (meta + 4 views) are now wrapped per-route in `main.go` (W-1 proven by a five-route 401 table test).
- **CORS is credential-aware yet still exact-origin (D-05/T-15-10).** `cors.go` now emits `Access-Control-Allow-Credentials: true` on both the actual response and the preflight, adds POST, and keeps the EXACT origin (never a wildcard — the wildcard+credentials combination is forbidden).
- **Login/callback/whoami-web/logout (D-01/D-03, T-15-06/07/13).** `handlers.go`: login mints state into a short-lived `sb_oauth_state` cookie and 302s to Discord; callback rejects a missing/mismatched state with 400 before any exchange (CSRF), refuses a non-member to `?not_member=1` with NO session, and for a member mints a **brand-new** session id (regenerate-on-login) after `UpsertWebUser`; the post-callback redirect Location is built **only** from the hardcoded `webOrigin` constant — an attacker-supplied `redirect`/`return_to`/`next` is ignored (W-4, proven by an open-redirect regression test); whoami-web returns the UI-SPEC AuthGate shape (always-200).
- **set-owner-floor CLI (D-08).** `ownerfloor.go` adds `squirebot-server set-owner-floor <discord-id>` (mirrors revoke-code's arg handling) → `store.SetOwnerFloor`, seeding both the `app_config` owner-floor pointer and the bootstrap-officer `guild_admins` row (proven against a temp DB).

## Task Commits

Each task was committed atomically (TDD RED→GREEN folded into one feat commit per task; tests written first, confirmed failing, then implementation):

1. **Task 1: Discord OAuth helpers + membership check (oauth.go)** — `b75cf71` (feat)
2. **Task 2: Session cookie + RequireSession/RequireOfficer middleware + CORS credentials** — `8fa642c` (feat)
3. **Task 3: OAuth login/callback/whoami-web/logout handlers (handlers.go)** — `394522d` (feat)
4. **Task 4: Wire the session gate into the read API + set-owner-floor CLI** — `0dc73ab` (feat)

**Plan metadata:** see the final docs commit (this SUMMARY + STATE + ROADMAP).

## Files Created/Modified

- `internal/backendsrv/webauth/oauth.go` — Config + ConfigFromEnv (DISCORD_* env), oauth2.Config builder (scopes identify+guilds), GenerateState, AuthCodeURL, server-side Exchange, FetchIdentity, FetchGuilds, fail-closed IsGuildMember; io.LimitReader-bounded reads; test-only endpoint seams.
- `internal/backendsrv/webauth/session.go` — SessionCookieName, CookieOpts/CookieOptsFromEnv, SetSessionCookie/ClearSessionCookie, UserFromContext, RequireSession (fail-closed 401 + rolling touch), RequireOfficer (403 not_authorized).
- `internal/backendsrv/webauth/handlers.go` — LoginHandler/CallbackHandler/WhoamiWebHandler/LogoutHandler; state CSRF; regenerate-on-login; W-4 hardcoded-origin redirect.
- `internal/backendsrv/webauth/{oauth,session,handlers}_test.go` — full httptest coverage (AuthCodeURL scopes, Exchange/identity/guilds fakes, IsGuildMember table, cookie attributes, 401 no-cookie, 403 non-officer, CORS creds, mismatched/missing state, non-member no-session, member session+web_user row, open-redirect regression, authed/anon whoami, logout).
- `internal/backendsrv/readapi/cors.go` — added Access-Control-Allow-Credentials:true + POST; updated the doc comment for the P15 reality (kept exact-origin).
- `internal/backendsrv/readapi/readapi_test.go` — added TestCORS_Credentials_OnGETandPreflight (creds header on response + preflight, never wildcard, POST allowed).
- `cmd/squirebot-server/main.go` — register the 4 auth routes ungated; wrap all 5 read routes in RequireSession; documented the live deploy env; added set-owner-floor to run().
- `cmd/squirebot-server/ownerfloor.go` — runSetOwnerFloor subcommand.
- `cmd/squirebot-server/main_test.go` — set-owner-floor dispatch test (app_config + guild_admins rows) + the W-1 five-route 401 read-gate table test.
- `go.mod` / `go.sum` — golang.org/x/oauth2 v0.36.0 direct dep.

## Decisions Made

- **One `webauth` package, `/api/v1/auth/*` routes (PLAN over RESEARCH).** The RESEARCH §"package layout" proposed splitting into `oauth/` + `websession/` + `webauth/` and used bare `/auth/*` routes; the PLAN consolidated everything into one `webauth` package and versioned `/api/v1/auth/*` routes. Followed the PLAN (it's authoritative + the store already owns all SQL, so the extra packages bought nothing).
- **`webOrigin` is the single redirect source (W-4).** A package var defaulting to the hardcoded `https://squirebot.quest`, env-overridable via `SQUIREBOT_WEB_ORIGIN` — the callback Location is `webOrigin + "/"` or `webOrigin + "/?not_member=1"`, never anything derived from request input.
- **`ConfigFromEnv` tolerates empty values.** The server starts without the DISCORD_* secrets (build/CI/local) — only the live login needs them. Keeps the build-only run honest.
- **Officer gate is two-layered.** `RequireOfficer` is the request-time outer rejection; the 15-03 mutators still re-authorize inside their tx (TOCTOU). Documented in the code so 15-03 doesn't mistake the middleware for the in-tx check.

## Deviations from Plan

None — plan executed exactly as written. Every task's `<action>` and `<acceptance_criteria>` were implemented literally; all grep-gates pass against the real code; no auto-fixes (Rules 1–3) were needed and no architectural questions (Rule 4) arose.

One trivial in-task hygiene adjustment worth noting (not a behavior deviation): the new `cors.go` comment was reworded to say "never a wildcard" instead of the literal `"*"`, so the plan's `no "*" literal in cors.go` acceptance gate passes while the comment still documents the rule. This matches the original file's own convention (it already said "wildcard", not the literal).

## Issues Encountered

- **`go mod tidy` prunes an unimported dep.** After `go get golang.org/x/oauth2@latest`, the first `go mod tidy` REMOVED it (nothing imported it yet). Resolved as designed: writing `oauth.go` (which imports it) then re-running `go mod tidy` re-pinned `golang.org/x/oauth2 v0.36.0` as a direct require. Network access for the module download worked.
- **A grep-gate `&&`-chain shell artifact (not a code issue).** `grep -c` exits 1 on zero matches, which short-circuited a naive `CNT=$(...) && echo PASS` chain and momentarily looked like the W-4 param-read gate "FAILED". Re-running the gate standalone confirmed the count is **0** (PASS) — the handler reads no caller-supplied redirect param. No code change needed.

## Secret Handling (V7 / scope directive)

No Discord secret value appears in any committed file, fixture, log line, or this SUMMARY. The real secret comes ONLY from `os.Getenv("DISCORD_CLIENT_SECRET")` (set on the box via systemd at deploy, never this run). The only literal "secret" strings in the tree are clearly-labeled placeholders in tests (`"csecret-not-real"`, `"test-client-secret-not-real"`). Every `slog` call in the package logs only an op label, `"err"`, or the public `discord_user_id` snowflake — never the token, secret, session id, or cookie value.

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are enforced + covered by a test:
- **T-15-06** (OAuth CSRF) — `state` (crypto/rand) stored in a short-lived httpOnly cookie + verified equal on callback; missing/mismatched → 400 before any exchange (`TestCallbackHandler_MismatchedState_400` / `_MissingState_400`).
- **T-15-07** (session theft/fixation) — opaque id, sha256-only at rest (15-01), httpOnly+Secure+SameSite=Lax, rolling 30d, brand-new id minted every login (`TestCallbackHandler_Member_MintsSession_RedirectHome` + cookie-attribute test).
- **T-15-08** (non-member admittance) — `IsGuildMember` fail-closed; non-member gets no session (`TestIsGuildMember_Table` + `TestCallbackHandler_NonMember_NoSession_RedirectNotMember`).
- **T-15-09** (client secret disclosure) — backend-only env, server-side exchange, never logged (see Secret Handling).
- **T-15-10** (cross-subdomain cookie + CORS-creds) — Domain=squirebot.quest, exact-origin + Allow-Credentials:true, never wildcard (`TestCORS_Credentials_OnGETandPreflight`).
- **T-15-11** (read API without a session) — RequireSession wraps all five read routes; per-route 401 table (`TestReadRoutes_RequireSession_401`).
- **T-15-12** (cookie tampering) — opaque id resolved server-side against the hashed store (15-01 `ResolveSession`); a forged value just fails resolution (`TestRequireSession_InvalidCookie_401`).
- **T-15-13** (open redirect) — post-callback Location is the `webOrigin` constant only; no caller redirect param read (`TestCallbackHandler_IgnoresCallerRedirect_W4` + the comment-stripped grep-gate, count 0).

## Known Stubs

None. Every handler composes real 15-01 store methods over the migrated schema; no hardcoded empty values flow to a UI, no placeholder text, no unwired data sources. (The bank-view coin placeholder remains `null` from P14 — that is 15-03's job per D-11, not a stub introduced here.)

## User Setup Required

None for this plan (local build-and-verify only). Deferred to the deploy step (per the STATE.md Phase 15 directives — NOT done this run):
- The 4 `DISCORD_*` vars (CLIENT_ID / CLIENT_SECRET / GUILD_ID / REDIRECT_URI) go in the `squirebot-server` systemd unit (root-only `EnvironmentFile=`, chmod 600) — secret backend-only.
- Set `SQUIREBOT_WEB_ORIGIN=https://squirebot.quest` + `SQUIREBOT_COOKIE_DOMAIN=squirebot.quest` on the unit (cors-origin already defaults to the apex).
- Run `squirebot-server set-owner-floor <maintainer-discord-USER-id>` once on the box (this plan added that subcommand).
- `goose` already applied `00004` (15-01); the rebuilt binary restart picks up the new routes.
- **Then run the deferred live login smoke**: sign in via Discord end-to-end, confirm a member gets in + a non-member is bounced to `?not_member=1`.

## Next Phase Readiness

- **15-03 (write forms)** can now compose the session/officer gates: `RequireSession` for the bank-coin form (ADMIN-05, authenticated-only — D-12) and `RequireOfficer` for the eviction + admin-mgmt forms (ADMIN-04/06, officer-only), with the 15-01 `*Tx` mutators re-authorizing inside their tx. `UserFromContext` gives each handler the caller's discord_user_id for the audit-log actor.
- **15-04/15-05 (frontend)** have the `/api/v1/auth/{login,callback,whoami-web,logout}` contract pinned + the `{authenticated,isMember,isOfficer,username,avatar,discord_user_id}` AuthGate shape; the SvelteKit fetches must pass `credentials:'include'` (CORS is now credential-aware).
- No blockers. `go build ./...`, `GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server`, `go vet`, and `go test ./...` all pass. The only open item is the deploy-time live login smoke (intentionally deferred — build-only directive).

## Self-Check: PASSED

All 7 created code files + this SUMMARY verified present on disk; all 4 task commit hashes (`b75cf71`, `8fa642c`, `394522d`, `0dc73ab`) verified in git history.

---
*Phase: 15-admin-web-forms-login*
*Completed: 2026-05-30*
