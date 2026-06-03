---
phase: 19-wantlist-crud
plan: 02
subsystem: backend-http-layer
tags: [wantlist, http-handlers, idor, session-owner, catalog-search, require-session, audit]
requires:
  - "Plan 01 store: WantlistRow + AddWantTx + ListOwnWants + RemoveOwnWantTx + ErrDuplicateWant"
  - "Plan 01 store: CatalogItem + (*Store).SearchCatalog"
  - "webadmin shared helpers: caller / nowUnix / writeJSON / writeJSONError / withTx / AppendAuditTx"
  - "webauth.RequireSession + webauth.UserFromContext"
provides:
  - "webadmin.AddWantHandler / ListOwnWantsHandler / RemoveOwnWantHandler / mapWantErr / validWant"
  - "readapi.NewItemSearch + ItemSearch (GET /api/v1/items/search)"
  - "4 RequireSession routes: GET/POST /api/v1/wantlist, POST /api/v1/wantlist/remove, GET /api/v1/items/search"
affects:
  - "Plan 03 frontend (consumes the wantlist + item-search JSON contracts)"
  - "Phase 20 alert pipeline (reads the wantlist_item rows these handlers write)"
tech-stack:
  added: []  # zero new dependencies
  patterns:
    - "Session-derived owner (caller(r.Context())) in every handler — never a body field (D-02 IDOR boundary)"
    - "Typed-sentinel error mapping: errors.Is(err, store.ErrDuplicateWant) -> 409 {\"error\":\"duplicate\"} (NOT message string-matching)"
    - "Note trimmed BEFORE the utf8.RuneCountInString 280-cap so a whitespace-only note stores NULL, never spaces"
    - "Audit detail carries item_id/want_id ONLY — never the note text (V7)"
    - "Search handler: q<2 short-circuit before any DB hit; logs rows+qlen, never q (V7/DoS)"
key-files:
  created:
    - internal/backendsrv/webadmin/wantlist.go
    - internal/backendsrv/webadmin/wantlist_test.go
    - internal/backendsrv/readapi/itemsearch.go
    - internal/backendsrv/readapi/itemsearch_test.go
  modified:
    - cmd/squirebot-server/main.go
    - cmd/squirebot-server/main_test.go
decisions:
  - "mapWantErr matches errors.Is(store.ErrDuplicateWant), not the driver's textual message (review MUST-FIX 2)"
  - "validWant trims req.Note BEFORE the 280-rune check so 280 spaces is treated as empty/NULL (review WORTH-FIX 6)"
  - "Owner is ALWAYS caller(ctx) from the session; no owner-resolve call, no owner entity (Pitfall 3 / D-02)"
  - "All four routes use RequireSession, NEVER RequireOfficer (login-only, every signed-in member)"
  - "Catalog wants store the client-supplied item_name snapshot (review JUDGMENT-CALL 8 — accepted, not a vuln)"
metrics:
  duration: ~20m
  completed: 2026-06-03
  tasks: 3
  files: 6
---

# Phase 19 Plan 02: Wantlist + Catalog-Search HTTP Layer Summary

The authenticated HTTP surface over Plan 01's store: `webadmin/wantlist.go` (the `account.go` twin — login-only, session-derived owner, IDOR-safe, audited add/list/remove with server-side validation) and `readapi/itemsearch.go` (the `views.go` twin — the D-10 `GET /api/v1/items/search` handler), wired into `main.go` as four `RequireSession` routes. Every handler clones a verified analog and enforces the v2.1 D-02 IDOR boundary (owner from session, never the body). Zero new dependencies; `go build` + the full backend + server test suites are green and gofmt-clean on every touched file.

## What Was Built

### Task 1 — `webadmin/wantlist.go` add/list/remove + validation + mapWantErr (commit `010ff53`)
- `addWantReq` body struct (`item_id` pointer, `item_name`, `reason`, `priority`, `note`) — NO owner field (D-02).
- `validWant`: `reason ∈ {buy,quest}`; non-empty `priority ∈ {low,med,high}` (empty allowed, defaults to `med`); the note is `strings.TrimSpace`'d BEFORE `utf8.RuneCountInString(...) > 280` (RUNES not bytes — Pitfall 2; trim-first means 280 spaces is measured as empty — review WORTH-FIX 6); a custom want (`item_id` nil) requires a non-blank trimmed label.
- `mapWantErr`: `errors.Is(err, store.ErrDuplicateWant)` → 409 `{"error":"duplicate"}` (typed sentinel, NOT message string-matching — review MUST-FIX 2); else `slog.Error` + 500.
- `AddWantHandler` (POST): method-check → decode → `validWant` (400 `invalid_input`) → default priority → compute `trimmedNote`/`notePtr` (whitespace-only → NULL) → `caller(r.Context())` → `withTx{ AddWantTx + AppendAuditTx("wantlist_add", detail={item_id}) }` → `mapWantErr` on error → echo the created `WantlistRow`. No owner-resolve call (Pitfall 3).
- `ListOwnWantsHandler` (GET): owner-scoped `store.ListOwnWants`, non-nil slice → JSON `[]`.
- `RemoveOwnWantHandler` (POST): decode `{id}`, reject `id<=0` → 400, `withTx{ RemoveOwnWantTx + (on real remove) AppendAuditTx("wantlist_remove", detail={want_id}) }`, `{removed}`.
- Tests (`wantlist_test.go`): catalog add audits item_id-only (asserts the note text never appears in the audit detail), custom add stores NULL item_id + defaults priority, validation table (bad reason/priority, 281-rune reject, blank-custom-label reject, 280-rune-ok, 280-spaces → stored NULL), duplicate → EXACTLY 409 `{"error":"duplicate"}` on BOTH the catalog and custom paths with same-item-other-reason → 200, owner-scoped list (other member's want excluded), cross-owner remove → `removed:false` no-op (IDOR, no audit), and bad-id → 400 — all against a real `store.NewTestDB`.

### Task 2 — `readapi/itemsearch.go` D-10 catalog search (commit `efc547d`)
- `ItemSearch{ st *store.Store }` + `NewItemSearch(st)` — the `ViewsHandler`/`NewViews` twin.
- `ServeHTTP`: GET-only (405); `q := strings.TrimSpace(query.Get("q"))`; `utf8.RuneCountInString(q) < 2` short-circuits to `[]` BEFORE any DB hit (empty-query guard + DoS mitigation — Pitfall A4); else `SearchCatalog(q, 25)`; nil→`[]` coercion (views.go:87); empty corpus degrades to `[]` not 500 (Pitfall A2). Logs `rows` + `qlen` + `status` only — NEVER `q` (V7).
- Tests (`itemsearch_test.go`, `package readapi_test`): `?q=rusty` returns the 2 seeded matches; `?q=r` and `?q=` short-circuit to `[]`; an empty `pigparse_price` corpus → 200 `[]`; POST → 405.

### Task 3 — Register 4 RequireSession routes + main_test gates (commit `d29706f`)
- `main.go`: four `mux.Handle(...)` registrations after the `/account/codes` block, ALL behind `webauth.RequireSession` (NEVER `RequireOfficer`), reusing the existing `st := store.NewStore(db)` — `GET`/`POST /api/v1/wantlist`, `POST /api/v1/wantlist/remove`, `GET /api/v1/items/search`.
- `main_test.go`: added `st := store.NewStore(db)` to `TestWriteRoutes_Gates`, mirrored the four route wraps, and added gate subtests — wantlist anon→401 / member→admitted, items/search anon→401 / member→admitted — proving login-only access (a `RequireOfficer` swap would 403 the member and fail the test).

## Deviations from Plan

None — the plan executed exactly as written. Two acceptance-grep wordings initially matched explanatory comments that *named the forbidden pattern in order to negate it* (the phrase "UNIQUE constraint failed" appeared in two doc comments describing what mapWantErr does NOT do). The code never string-matched the driver message; the comments were reworded to "the driver's textual message" so the `grep -c "UNIQUE constraint failed"` gate returns 0, with no behavior change. (Same class of wording fix Plan 01 noted.) Additionally, the cross-owner IDOR test fixtures required seeding a `web_user` row for the "disc-other" owner first (the `wantlist_item.discord_user_id` FK → `web_user`), added during the RED→GREEN loop.

## Verification

- `go build ./...` → exit 0.
- `go test ./internal/backendsrv/... ./cmd/squirebot-server/...` → every package `ok` (webadmin, readapi, squirebot-server, and all others).
- `gofmt -l internal/backendsrv/webadmin internal/backendsrv/readapi cmd/squirebot-server` → clean.
- Acceptance grep gates: `caller(r.Context())` ×3 in wantlist.go; 0 owner-forbidden tokens; `utf8.RuneCountInString` + `TrimSpace(req.Note)` present; `errors.Is(err, store.ErrDuplicateWant)` ×1; 0 `UNIQUE constraint failed`; `func NewItemSearch` ×1; q never logged; 3 `/api/v1/wantlist` + 1 `/api/v1/items/search` routes, 4 `RequireSession` wraps.

## Threat Model Coverage

| Threat | Disposition | How met |
|--------|-------------|---------|
| T-19-06 Spoofing (forged owner) | mitigate | Owner is `caller(r.Context())`; no owner/discord body field; grep-gated to 0 owner-resolve calls. |
| T-19-07 EoP (IDOR remove) | mitigate | Delegates to the store's owner-scoped UPDATE; cross-owner remove → `{removed:false}` no-op proven by `TestRemoveOwnWant_CrossOwnerNoOp_OwnRemoved`. |
| T-19-08 Tampering (bad enums / oversized / blank label) | mitigate | `validWant` re-validates enums + TRIMMED 280-rune note + non-blank custom label server-side; DB CHECK constraints are the second line. |
| T-19-09 Auth bypass (anon access) | mitigate | All 4 routes `RequireSession`; anon→401 proven by `main_test`; none use `RequireOfficer`. |
| T-19-10 DoS (unbounded search) | mitigate | `len(q)>=2` guard short-circuits before any DB hit; `LIMIT 25`; behind `RequireSession`. |
| T-19-11 Info disclosure (query/PII in logs) | mitigate | Search logs `rows`+`qlen` only (never q); audit detail carries item_id/want_id only (never note text); both grep-gated + test-asserted. |
| T-19-12 Tampering (client name snapshot) | accept | Catalog wants store the client `item_name`; the in-bank join keys on `item_id` and the name renders via `{}` auto-escape (Plan 03) — integrity smell, not a vuln. |

## Self-Check: PASSED

- Created files exist: `webadmin/wantlist.go`, `webadmin/wantlist_test.go`, `readapi/itemsearch.go`, `readapi/itemsearch_test.go` — all FOUND.
- Commits exist: `010ff53`, `efc547d`, `d29706f` — all FOUND.
