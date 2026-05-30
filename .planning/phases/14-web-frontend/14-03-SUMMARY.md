---
phase: 14-web-frontend
plan: 03
subsystem: api
tags: [go, net-http, servemux, read-api, cors, json-contract, httptest, public-read, backend-05]

# Dependency graph
requires:
  - phase: 14-web-frontend (Plan 14-01)
    provides: "the compute package (View/Bank/GearCheck/SpellCheck) + store.CharFreshness + the FIXED snake_case row contract these handlers JSON-encode"
  - phase: 11-backend-foundation-ingest-api
    provides: "the hand-rolled net/http ServeMux + store.Store/NewStore/NewTestDB + the whoami.go read-only handler precedent these mirror (minus the bearer guard)"
provides:
  - "internal/backendsrv/readapi — the 5 public read handlers (ViewsHandler dispatched by name + MetaHandler) + the stdlib CORS middleware (exact origin, OPTIONS 204)"
  - "the PINNED endpoint JSON contract (the 5 paths + their response shapes) Plan 14-04's Svelte client wires to"
  - "cmd/squirebot-server route registration (the 5 GET routes) + the whole mux wrapped in CORS via a -cors-origin flag (default https://app.squirebot.quest)"
affects: [14-04-svelte-client, 15-admin-web-forms]

# Tech tracking
tech-stack:
  added: []  # no new deps — pure Go stdlib net/http + the existing modernc store; CORS hand-rolled (~10 lines)
  patterns:
    - "Public read handler = whoami.go's method-guard → read → Content-Type json → Encode → slog.Info skeleton MINUS the bearer-guard/401 block (D-04); one ViewsHandler type parameterized by view name dispatches the 4 views"
    - "stdlib CORS middleware (func(string, http.Handler) http.Handler): exact origin echo (never wildcard), Vary: Origin, 204 short-circuit on OPTIONS — wraps the whole mux once in Go (Pitfall 5: Caddy must not duplicate)"
    - "Handler boundary coerces nil compute row-slices to [] so every view endpoint encodes a stable JSON array (never null) for the thin client"

key-files:
  created:
    - "internal/backendsrv/readapi/views.go"
    - "internal/backendsrv/readapi/meta.go"
    - "internal/backendsrv/readapi/cors.go"
    - "internal/backendsrv/readapi/readapi_test.go"
  modified:
    - "cmd/squirebot-server/main.go"

key-decisions:
  - "meta.go defines a LOCAL typed response (MetaResponse{characters:[{name,last_seen}]}) rather than encoding store.CharFreshness directly — store.CharFreshness has Go field names but no JSON tags, so a local struct pins the snake_case contract Plan 04 consumes"
  - "Empty view results encode as [] not null (nil-slice → []) — a stable shape the thin client iterates without a nil-guard. Rule 2 (missing critical: stable contract). The bank's rows[] gets the same coercion; coin stays null (P14)"
  - "ViewsHandler is ONE type parameterized by view name (NewViews(st, \"view\")) dispatching a switch to the matching compute fn — 4 routes, 1 handler type, mirroring the PATTERNS suggestion; meta is its own MetaHandler"
  - "CORS wraps the WHOLE mux (not just the read routes) — ingest/whoami keep their bearer guard and the extra CORS headers are harmless on them; this keeps the wrap a one-liner and CORS travels with every route"
  - "-cors-origin is a serve flag (default https://app.squirebot.quest, the locked Cloudflare Pages root subdomain) so a staging/preview origin is a flag change, not a recompile; the exact-origin echo keeps a P15 credentialed upgrade one-line"

patterns-established:
  - "readapi package = the HTTP surface that composes compute (compute → store → SQLite); handlers author ZERO SQL and ZERO writes (GET-only, 405 otherwise)"
  - "V7 logging on the read path: slog carries op + view name + row count + status + err ONLY — never row content or query params"

requirements-completed: [BACKEND-05]

# Metrics
duration: 11min
completed: 2026-05-30
---

# Phase 14 Plan 03: Read Handlers + CORS Summary

**The HTTP-surface half of BACKEND-05: five public, read-only `/api/v1` handlers (the 4 consolidated views dispatched by one name-parameterized `ViewsHandler` + a `MetaHandler`) that JSON-encode Plan 14-01's compute structs, plus a ~10-line stdlib CORS middleware (exact locked origin, `OPTIONS` 204) wrapping the whole mux — mirroring `whoami.go` exactly minus the bearer guard (D-04), with the endpoint JSON contract now PINNED for Plan 14-04.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-05-30T16:35:14Z
- **Completed:** 2026-05-30T16:46:47Z
- **Tasks:** 2 (Task 1 TDD: handlers+CORS+httptest landed together; Task 2 auto: main.go wiring)
- **Files created:** 4 (readapi: views/meta/cors + test); **modified:** 1 (main.go)

## Accomplishments

- **`readapi` package — 5 public read handlers** mirroring `whoami.go`'s read-only skeleton with the bearer-guard/401 block DROPPED (D-04 public read): one `ViewsHandler` parameterized by view name dispatches a switch to `compute.View`/`GearCheck`/`SpellCheck`/`Bank`; `MetaHandler` serves the character-freshness shell feed. GET-only (405 otherwise), zero writes, V7 slog (op + view + count + status + err only).
- **`cors.go` — stdlib CORS middleware** (no dependency): echoes the EXACT locked origin (never a wildcard), sets `Vary: Origin` + `Access-Control-Allow-Methods/Headers`, and short-circuits the `OPTIONS` preflight with 204 + no body. Deliberately no `Access-Control-Allow-Credentials` (no cookies in P14; exact-origin echo keeps the P15 credentialed upgrade one-line).
- **`main.go` wiring** — the 5 `GET` routes registered next to `ingest`/`whoami` over `st := store.NewStore(db)`; the whole mux wrapped in `readapi.CORS(*corsOrigin, mux)` via a new `-cors-origin` serve flag (default `https://app.squirebot.quest`). A code comment flags the Caddy-must-not-duplicate-CORS deploy check (Pitfall 5 / T-14.03-06).
- **httptest-proven contract:** each views route → 200 + `application/json` + right-shaped body; bank body is `{rows,coin:null}` (coin asserted JSON-null); `/meta` → `{characters:[{name,last_seen}]}`; non-GET → 405; CORS GET echoes the exact origin (and is asserted never `*`); `OPTIONS` → 204 with the header + empty body (inner handler proven not to run).
- **Whole repo green:** `go test ./internal/backendsrv/...` passes (incl. the new `readapi` httptests + no compute/store/ingest regression); `go build ./...` + static `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` cross-compile + `go vet ./...` all exit 0.

## THE PINNED ENDPOINT JSON CONTRACT (Plan 14-04 consumes this)

All endpoints are `GET`, public (no auth in P14), `Content-Type: application/json`, and carry the CORS headers. Base URL on the box: `https://api.squirebot.quest`. Non-GET → **405**; any handler error → **500** `internal error`.

### `GET /api/v1/views/view` → `200`, a **JSON array** of view rows (`[]` when empty, never `null`)

Each element (snake_case tags from `compute.ViewRow`, `compute/types.go`):

| field | type | notes |
|-------|------|-------|
| `char` | string | leading Char column (consolidated-view contract) |
| `slot` | string | inventory_item.location |
| `item` | string | inventory_item.name (RAW — client escapes) |
| `id` | number (int64) | inventory_item.item_id |
| `count` | number (int64) | |
| `wiki_url` | string | PLAIN url (never an `=HYPERLINK` formula) — client renders a real `<a>` |
| `price` | number \| **null** | `*float64`; **null** when neither WTS nor WTB has a30>0 (client renders Price blank) |
| `last_synced` | string | ISO timestamp (character.last_seen); freshness coloring is client-side |
| `wiki_summary` | string | tooltip text (RAW wiki text — client escapes) |
| `is_quest_item` | bool | |
| `prices` | array of `{direction,a30,t30}` | 0 or 1 element (pigparse_price.item_id is PK). `direction` is a **string** `"0"`=WTS / `"1"`=WTB / `"2"`=BOTH; `a30` number; `t30` number(int64) |
| `quest_links` | array of `{quest_name,source}` | `source` is `"in_game_flag"` \| `"notes_link"` |

### `GET /api/v1/views/gear_check` → `200`, a **JSON array** of `compute.GearCheckRow` (`[]` when empty)

Fields: `char` (string), `class` (string), `tier` (string), `slot` (string), `have` (string), `recommended` (string), `status` (string = `"OK"` \| `"OTHER"` \| `"MISSING"`).

### `GET /api/v1/views/spell_check` → `200`, a **JSON array** of `compute.SpellCheckRow` (`[]` when empty)

Fields: `char` (string), `class` (string), `level` (number int64), `spell` (string), `status` (string = `"KNOWN"` \| `"MISSING"`).

### `GET /api/v1/views/bank` → `200`, a **JSON object** (NOT a bare array)

```json
{ "rows": [ <ViewRow>, ... ], "coin": null }
```
- `rows` — same element shape as `/views/view` (`[]` when empty, never null).
- `coin` — **always `null` in P14** (inherited stub from Plan 14-01; `/outputfile inventory` has no coin data; ADMIN-05 fills it in P15 as `{pp,gp,sp,cp}`). The client must render "Coin: not yet recorded" on null — **never fabricate `0pp`**.

### `GET /api/v1/meta` → `200`, a **JSON object**

```json
{ "characters": [ { "name": "Alpha", "last_seen": "2026-05-09T00:00:00Z" }, ... ] }
```
- `characters` — non-removed characters + per-character freshness (`[]` when none). `last_seen` is `""` when never seen. (Available-themes list is a compile-time client constant, NOT in this payload — RESEARCH.)

### CORS (every response)

- `Access-Control-Allow-Origin: https://app.squirebot.quest` (the **exact** `-cors-origin` flag value — never `*`)
- `Vary: Origin`
- `Access-Control-Allow-Methods: GET, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`
- An `OPTIONS` preflight to any route → **204**, those headers, empty body.
- No `Access-Control-Allow-Credentials` in P14 (no cookies; P15 tightens when sessions arrive).

**CORS origin flag default:** `https://app.squirebot.quest` (the locked Cloudflare Pages root subdomain — Plan 02 Task 1). Override with `squirebot-server serve -cors-origin <origin>` for a staging/preview deploy.

**Caddy verification note (deploy-time, T-14.03-06 / Pitfall 5):** CORS is set ONCE, in Go. The on-box Caddyfile fronting 443 → loopback `127.0.0.1:8090` MUST NOT also emit `Access-Control-Allow-Origin` — a duplicated header ("origin, origin") makes the browser reject the response. Verify on the VPS that Caddy's `reverse_proxy` block adds no CORS headers (operational step, mirroring P11's manual-deploy posture; flagged in a `main.go` code comment).

## Task Commits

1. **Task 1: 5 read handlers (views + meta) + stdlib CORS + httptest** — `5c4a8de` (feat)
   _TDD task: the httptest + the handlers/CORS landed in one commit (Go can't compile a test referencing handler types that don't exist yet — same precedent as Plan 14-01)._
2. **Task 2: register the 5 routes in main.go + wrap mux in CORS (locked origin flag)** — `377bd31` (feat)

**Plan metadata:** _(this SUMMARY + STATE + ROADMAP)_ committed separately.

## Files Created/Modified

- `internal/backendsrv/readapi/views.go` — `ViewsHandler` (struct{store, view} + `NewViews`); `ServeHTTP` = 405-guard → switch on view name to the matching `compute.*` fn → nil-slice→[] coercion → `Content-Type: application/json` → `json.Encode` → `slog.Info`. Bank case encodes the `BankView` struct (yielding `{rows,coin:null}`).
- `internal/backendsrv/readapi/meta.go` — `MetaHandler` + `NewMeta`; local `MetaResponse{Characters []metaChar}` with `metaChar{Name "name", LastSeen "last_seen"}` snake_case tags; maps `store.CharFreshness` rows into it (pre-sized so empty → `[]`).
- `internal/backendsrv/readapi/cors.go` — `CORS(allowOrigin string, next http.Handler) http.Handler`: exact-origin echo, `Vary: Origin`, methods/headers, `OPTIONS`→204. No dependency.
- `internal/backendsrv/readapi/readapi_test.go` — `package readapi_test`; self-contained raw-INSERT seed helpers over `store.NewTestDB` (the store/compute seed helpers are package-private); httptest assertions for all 5 endpoints + 405 + the CORS GET/OPTIONS contract.
- `cmd/squirebot-server/main.go` — added the `readapi` import, the `defaultCORSOrigin` const, the `-cors-origin` serve flag, `st := store.NewStore(db)`, the 5 `mux.Handle` routes, and the `Handler: readapi.CORS(*corsOrigin, mux)` wrap (+ the Pitfall 5 Caddy comment).

## Decisions Made

- **Local typed meta response, not raw `store.CharFreshness` encode.** `store.CharFreshness{Name, LastSeen}` has no JSON tags, so encoding it directly would emit `{"Name":...,"LastSeen":...}` (PascalCase), breaking the locked contract. `meta.go` defines `MetaResponse`/`metaChar` with snake_case tags and maps the rows — the single right place to pin the `{characters:[{name,last_seen}]}` shape. (This is the only place these handlers add a struct rather than encoding a compute struct verbatim.)
- **nil row-slice → `[]` at the handler boundary (Rule 2 — missing critical: stable contract).** `compute.View`/`GearCheck`/`SpellCheck` return a `nil` slice when no rows match, which `json.Encode` emits as `null`. A thin client doing `rows.map(...)` would break on `null`. Coercing nil → `[]` (for all three array views + the bank's `rows`) guarantees every view endpoint always returns a JSON array. Documented inline; the gear_check test (no Velious tiers seeded) is the regression catcher.
- **One `ViewsHandler` parameterized by view name** (vs four handler types) — matches the PATTERNS suggestion `readapi.NewViews(st, "view")`, keeps the 4 routes a 4-line block, and the `switch` is the single dispatch point. `meta` is a distinct `MetaHandler` (different response shape, no view param).
- **CORS wraps the whole mux, set once in Go.** Wrapping everything (not just the read routes) keeps the wrap a one-liner and means CORS travels with every route; ingest/whoami are functionally unaffected (they keep their bearer guard; the extra response headers are harmless). Set once in Go per Pitfall 5 — Caddy must not duplicate.

## Deviations from Plan

None affecting behavior or scope. The plan's `<interfaces>` matched the codebase exactly (`compute.View/Bank/GearCheck/SpellCheck` signatures, `store.NewStore`/`NewTestDB`, `store.CharFreshness`, the `main.go:258-265` mux block, the whoami.go skeleton).

**Two in-task adjustments (not behavioral deviations):**

**1. [Rule 2 — Missing critical: stable JSON contract] nil-slice → `[]` coercion.**
- **Found during:** Task 1 (the `gear_check` httptest decoded the body to `nil` because an empty `[]GearCheckRow` JSON-encodes as `null`).
- **Fix:** Coerce a nil row-slice to a non-nil empty slice in each of the four view cases before encoding, so every view endpoint returns a JSON array (never `null`) — a stable shape the thin Plan-04 client can iterate without a nil-guard. The bank's `rows` gets the same treatment; `coin` stays `null`.
- **Files modified:** `internal/backendsrv/readapi/views.go`.
- **Verification:** `TestViewsGearCheck_OK` (empty result) now decodes as a non-nil array; the other view tests still pass; `go test ./readapi/` green.
- **Committed in:** `5c4a8de` (Task 1 commit).

**2. [Cosmetic — literal-grep satisfaction, no behavior change] Comment rewording.** Two acceptance-criteria greps are literal-token counts: `grep -c "ResolveToken" readapi/` must be 0, and `grep -c '"\*"' cors.go` must be 0. My initial explanatory comments contained the literal tokens `ResolveToken` ("...DROP the bearer guard (auth.ResolveToken)...") and `"*"` ("...never the \"*\" wildcard..."). Following the Plan 14-01 precedent (which reworded its `HYPERLINK` comments for the same reason), I reworded the comments ("the token-resolve / 401 block", "never a wildcard") to satisfy the literal greps while preserving intent. No code behavior changed — the handlers never call `ResolveToken` and the CORS code uses the `allowOrigin` parameter, never a literal `"*"`.

---

**Total deviations:** 1 auto-fix (Rule 2 — stable contract) + 1 cosmetic comment reword.
**Impact on plan:** The Rule-2 coercion hardens the contract Plan 04 depends on (no scope creep). The reword is documentation-only.

## Issues Encountered

- **Test seed helpers are package-private.** `store`'s and `compute_test`'s seed helpers (`seedChar`/`seedInv`/etc.) are not exported, so `package readapi_test` can't import them. Resolved by re-defining thin self-contained raw-INSERT helpers in `readapi_test.go` over `store.NewTestDB`'s migrated `*sql.DB`, mirroring the verified column layouts in `migrations/00001_init.sql` + `00003` (the same approach `compute/fixtures_test.go` took). Keeps the production API clean (no exported test-only seam).

## Threat Surface

This plan ADDS 5 public HTTP endpoints — but every one is covered by the plan's `<threat_model>` (T-14.03-01..06). No new surface beyond those dispositions:
- **T-14.03-01 (CORS spoofing/info-disclosure) — mitigated:** exact-origin echo, no wildcard (grep-verified `grep -c '"\*"' cors.go` = 0), no `Allow-Credentials`.
- **T-14.03-02 (public data) — accepted, time-boxed:** D-04 ships unauthenticated to unblock the guild; P15's AUTH-08 walls it; D-05 noindex keeps it off search engines.
- **T-14.03-03 (elevation) — mitigated:** every handler is GET-only (405 otherwise) and calls only SELECT-backed compute/store reads — `grep -c "INSERT\|UPDATE\|DELETE" views.go meta.go` = 0.
- **T-14.03-04 (SQL injection) — mitigated:** reads take no user input server-side in P14 (no query-param filters); all SQL is parameterized in Plan 01's store.
- **T-14.03-05 (log info-disclosure) — mitigated:** slog logs op + view + count + status + err only, never row content (V7).
- **T-14.03-06 (CORS header duplication) — mitigated (deploy-time):** CORS set once in Go; `main.go` code comment flags the Caddy-must-not-duplicate check for the on-box deploy.

No threat flags (no surface outside the threat model).

## Known Stubs

- **`bank.coin` is always `null` in P14** — INHERITED from Plan 14-01 (not introduced here). These handlers pass through the `compute.BankView.Coin` (nil) value; the JSON shape (`{rows,coin:null}`) is stable for P15's ADMIN-05. The client renders "Coin: not yet recorded" on null and must never fabricate `0pp`. Does NOT block P14's goal (the bank inventory grid is fully functional). No new stubs introduced by this plan.

## Next Phase / Next Plan Readiness

- **Plan 14-04 (Svelte client)** can now wire `lib/api.ts` fetch wrappers to the 5 endpoints above using the PINNED field names — notably: every view endpoint returns a JSON array (`[]` when empty, never null); `/views/bank` returns `{rows,coin:null}`; `/meta` returns `{characters:[{name,last_seen}]}`; `price` is nullable; `prices[].direction` is a string `"0"`/`"1"`/`"2"`; the CORS origin is `https://app.squirebot.quest`.
- **Deploy (operational, outside this build plan, mirroring P11's manual posture):** drop the new linux/amd64 binary on the Hetzner box + restart; verify Caddy's `reverse_proxy` block does NOT also set `Access-Control-Allow-Origin` (Pitfall 5); confirm the public Cloudflare Pages origin matches the `-cors-origin` default. The static-site Cloudflare deploy is Plan 14-04 / the milestone's cutover step.
- **No blockers.** `go test ./internal/backendsrv/...` green; `go build ./...` + static linux/amd64 cross-compile + `go vet ./...` exit 0.

## Self-Check: PASSED

- All 4 created Go files verified present on disk (`readapi/{views,meta,cors}.go` + `readapi_test.go`) + this SUMMARY.
- `cmd/squirebot-server/main.go` verified modified (contains `readapi.CORS(`).
- Both task commits verified in git log: `5c4a8de` (Task 1), `377bd31` (Task 2).
- `go test ./internal/backendsrv/...` green (uncached run of readapi/compute/store); `go build ./...` + static `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` cross-compile + `go vet ./...` exit 0.
- All Task 1 + Task 2 acceptance greps confirmed (ViewsHandler ServeHTTP present; `ResolveToken` count 0; all 4 compute calls; CORS exact-origin with wildcard-literal count 0; `StatusNoContent` 204; read-only INSERT/UPDATE/DELETE count 0; 5 routes + `readapi.CORS` wrap + `-cors-origin` default in main.go; Pitfall 5 Caddy note).

---
*Phase: 14-web-frontend*
*Completed: 2026-05-30*
