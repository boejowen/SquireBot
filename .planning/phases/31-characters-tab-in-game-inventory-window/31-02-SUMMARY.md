---
phase: 31-characters-tab-in-game-inventory-window
plan: 02
subsystem: backend-readapi
tags: [go, net-http, readapi, sqlite, session-gate, roster, structured-inventory, char-path-param]

# Dependency graph
requires:
  - phase: 31-characters-tab-in-game-inventory-window (plan 31-01)
    provides: InventorySlot.icon_id + CharacterInventory.last_seen on compute.StructuredInventory (the icon_id/last_seen JSON contract this plan exposes over HTTP)
  - phase: 29-data-foundation-inventory-parse-price-value-aggregation
    provides: compute.StructuredInventory + InventoryForChar (the ?-bound, name-keyed read seam) + the 23-slot taxonomy
  - phase: 26-character-assignment
    provides: character_assignment (the v2.3 "my characters" key) + is_guild_bot/is_bank_toon designation flags on character
  - phase: 15-discord-session-gate
    provides: webauth.RequireSession + webauth.UserFromContext (the read-API login gate + viewer identity)
provides:
  - "GET /api/v1/inventory/{char} — one character's compute.StructuredInventory, session-gated, empty-not-404"
  - "GET /api/v1/characters — the viewer-first band-tagged roster (is_mine + bank/bot flags), session-gated"
  - "store.RosterFor(ctx, viewerDiscordID) + RosterRow — the full roster shape no prior read returned together"
affects: [31-03, 31-04, 33-banks-tab, 34-wishlist-rework]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Read handler = views.go/meta.go shape: NewX(st) ctor + GET-only 405 guard + nil->[] coercion + V7 op/count-only logging"
    - "{char} path wildcard (Go ServeMux) bound ONLY as a ? placeholder downstream (StructuredInventory -> InventoryForChar); never SQL/log text"
    - "Viewer-first band ordering done in Go as a pure stable sort over an A-Z (COLLATE NOCASE) SQL fetch — node/Go-testable, not buried in SQL"

key-files:
  created:
    - internal/backendsrv/readapi/inventory.go
    - internal/backendsrv/readapi/characters.go
  modified:
    - internal/backendsrv/store/readviews.go
    - internal/backendsrv/store/readviews_test.go
    - internal/backendsrv/readapi/readapi_test.go
    - cmd/squirebot-server/main.go

key-decisions:
  - "RosterFor is a single *Store method (RESEARCH Open Q2 recommendation) doing the viewer-assignment join in-SQL, so both handlers hold only *store.Store like views.go — no *sql.DB threaded into NewCharacters"
  - "Viewer-first banding (yours -> guild -> banks/bots, A-Z within each) is a pure Go stable sort over the A-Z SQL fetch, keeping the D-10 logic observable to TestRosterFor; IsMine wins the bank/bot tie-break"
  - "Unknown char returns an empty CharacterInventory (200) via the nil->[] coercion, NOT a 404 — the V4/D-11 empty-not-404 contract the client renders as 'no inventory synced yet'"
  - "Both routes are webauth.RequireSession-wrapped (NOT public, NOT RequireOfficer): guild-wide member reads, gate = membership not ownership (T-31-05/T-31-07)"

patterns-established:
  - "A {char} ServeMux wildcard read: handler reads r.PathValue('char') and passes it by value to the ?-bound store seam — never concatenated into SQL or a content log (V5/V7)"

requirements-completed: [CHAR-01, CHAR-02, CHAR-03, INV-01, INV-02, INV-03, INV-04]

# Metrics
duration: 11min
completed: 2026-06-18
---

# Phase 31 Plan 02: Session-Gated Inventory + Characters Read Routes Summary

**Two new `webauth.RequireSession`-gated read endpoints — `GET /api/v1/inventory/{char}` (one character's `compute.StructuredInventory`, including the Plan 31-01 `icon_id`/`last_seen`) and `GET /api/v1/characters` (the viewer-first band-tagged roster) — plus the `RosterFor` store read that returns the full roster shape (`is_mine` + bank/bot flags + meta + `last_seen`) no existing read returned together.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-06-18T07:46:55Z
- **Completed:** 2026-06-18T07:57:15Z
- **Tasks:** 2
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments
- `store.RosterFor(ctx, viewerDiscordID)` + `RosterRow`: a single tested SQL path returning `id/name/class/level/race/is_bank_toon/is_guild_bot/last_seen` plus a computed `is_mine` (the v2.3 `character_assignment` LEFT JOIN bound on the viewer id). The D-10 viewer-first banding (yours → guild → banks/bots, A-Z within each via the SQL `COLLATE NOCASE` fetch) is a pure Go stable sort, so the banding logic stays observable to a unit test; `IsMine` wins the bank/bot tie-break.
- `readapi.NewInventory` (`GET /api/v1/inventory/{char}`): the `views.go` handler shape reading the `{char}` ServeMux wildcard and dispatching to `compute.StructuredInventory`. The three slot arrays are coerced `nil → []` so an unknown char returns an empty `CharacterInventory` (200, **not** 404 — the D-11 "no inventory synced yet" contract). The per-slot `icon_id` and per-character `last_seen` from Plan 31-01 ride through unchanged.
- `readapi.NewCharacters` (`GET /api/v1/characters`): reads the viewer id from the `RequireSession`-populated context (`webauth.UserFromContext`) → `store.RosterFor` → a pre-sized `[]rosterChar` (snake_case contract; empty marshals as `[]`).
- Both routes registered under `webauth.RequireSession` in `cmd/squirebot-server/main.go` (login-gated since P15 — **NOT** public, **NOT** `RequireOfficer`).
- Go coverage: `TestRosterFor` (viewer-first order / `IsMine`-only-for-viewer / nullable-meta zero-values / banks-bots band / no-assignment-viewer collapse); `TestInventory_UnknownChar_EmptyNot404`, `TestInventory_KnownChar_RendersSlots` (incl. the `icon_id` contract key), `TestInventory_NonGET_405`; `TestCharacters_ViewerFirstRoster`, `TestCharacters_EmptyRoster_ArrayNotNull`, `TestCharacters_NonGET_405`; and `TestNewRoutes_RequireSession_401WithoutCookie` (the **BLOCKING T-31-05 gate** exercised over the real `RequireSession` wrap → 401 fail-closed on both routes).

## Task Commits

Each task was committed atomically (TDD: failing test authored first — RED — then the implementation landed GREEN in the same task commit, matching the 31-01 convention):

1. **Task 1: RosterFor store read (viewer-aware, band-tagged, ?-only)** — `4c43a0c` (feat)
2. **Task 2: readapi inventory + characters handlers + RequireSession route registration** — `506e623` (feat)

## Files Created/Modified
- `internal/backendsrv/readapi/inventory.go` — NEW: `InventoryHandler`/`NewInventory`; GET-only 405 guard; `r.PathValue("char")` → `compute.StructuredInventory`; `nil → []` on Equipment/General/Bank; V7 logging (rows + status, never `char`/content).
- `internal/backendsrv/readapi/characters.go` — NEW: `CharactersHandler`/`NewCharacters`; `webauth.UserFromContext` viewer id → `h.store.RosterFor`; `make([]rosterChar, 0, len)` (empty → `[]`); snake_case JSON tags pinning the contract.
- `internal/backendsrv/store/readviews.go` — `RosterRow` struct + `RosterFor` (the `?`-bound viewer-assignment join, A-Z `COLLATE NOCASE` SQL) + `sortRosterViewerFirst`/`rosterBand` (the pure D-10 band sort); added the `sort` import.
- `internal/backendsrv/store/readviews_test.go` — `seedAssignment` helper + `TestRosterFor`.
- `internal/backendsrv/readapi/readapi_test.go` — `seedWebUser`/`seedAssignment` helpers + the inventory/characters handler tests + the route-level `RequireSession` 401 proof; added `context` + `webauth` imports.
- `cmd/squirebot-server/main.go` — registered both new routes under `webauth.RequireSession` (after the `items/search` route).

## Decisions Made
- **`RosterFor` is a single `*Store` method** (RESEARCH Open Q2) that joins `character_assignment` filtered to the viewer in-SQL — so `NewCharacters(st)` holds only `*store.Store` exactly like `views.go`, and no `*sql.DB` is threaded into the handler (the plan flagged `NewCharacters(st, db)` only as a fallback; it was not needed).
- **Banding done in Go, not SQL** — the SQL fetches A-Z (`COLLATE NOCASE`); a pure stable sort keyed on the band index reorders the bands while preserving the alphabetical order inside each. This keeps the D-10 logic table-testable (`TestRosterFor`) rather than buried in a multi-key `ORDER BY`.
- **Empty-not-404 via the existing `nil → []` coercion** — an unknown char flows through `StructuredInventory` with all three slices nil; the coercion turns them into `[]`, yielding a 200 with empty arrays (D-11), no special-case 404 branch.
- **Both routes `RequireSession`, never `RequireOfficer`** — the roster + inventory are guild-wide member reads (consolidated-views model): every member may view every character; the gate is membership, the `{char}` is not an access-control boundary (no IDOR surface — read-only, no per-owner scoping).

## Deviations from Plan

The plan was executed as written. One small additive item beyond the literal task file (completeness, not scope creep):

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added the route-level `RequireSession` 401 proof (`TestNewRoutes_RequireSession_401WithoutCookie`)**
- **Found during:** Task 2 (readapi tests)
- **Issue:** The plan's `<action>` step 4 noted the end-to-end 401-without-session "is enforced by the `RequireSession` wrap at registration" and left asserting it conditional on "if the existing harness mounts the mux." T-31-05 is the phase's **BLOCKING** gate, so a direct, deterministic proof is warranted rather than documenting the gate as un-asserted.
- **Fix:** Added a test that wraps both `readapi.NewInventory`/`NewCharacters` in `webauth.RequireSession(db, …)` (the **exact** wrap `main.go` applies) and asserts a cookieless GET returns **401** on both — proving fail-closed at the API, not just the UI.
- **Files modified:** `internal/backendsrv/readapi/readapi_test.go`
- **Verification:** `go test ./internal/backendsrv/readapi/...` green.
- **Committed in:** `506e623` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 2 — security-test completeness for the BLOCKING gate).
**Impact on plan:** Stays within the plan's named file (`readapi_test.go`). No production-code scope creep; no new public surface.

## Issues Encountered
None blocking. Two reality-vs-plan checks resolved cleanly:
- `character_assignment.discord_user_id` has an FK to `web_user`, and `NewTestDB` runs with `foreign_keys` ON — so the `RosterFor`/characters-handler tests seed a `web_user` row before the assignment row (the established `insertWebUser` / new `seedWebUser` pattern). No production-code impact.
- An early draft used a non-existent `withPathValue` context helper for the `{char}` handler test; switched to the Go 1.22+ `Request.SetPathValue("char", …)` (the API the production ServeMux populates), which is the correct test seam.

## Known Stubs
None. Both endpoints are real data-flow wiring (path param/viewer id → store read → compute → JSON), end-to-end test-covered. The web Characters tab that consumes these is Plan 31-04.

## Threat Flags
None. Both endpoints inherit the shipped `RequireSession` + credential-aware CORS + V7-logging + `?`-bind posture; they add no new trust-boundary surface beyond the two routes the plan's `<threat_model>` already registers (T-31-05..09). The `{char}` is bound only as a `?` placeholder (T-31-06), the gate is `RequireSession` on both (T-31-05/07), and no slog line carries the `char`/row content (T-31-08).

## User Setup Required
None — no external service configuration. The routes go live on the next backend deploy (Plan 31-04's deploy step, alongside the binary). Manual sanity (documented in the plan's `<verification>`, fired in 31-04): `curl https://api.squirebot.quest/api/v1/characters` → 401 when unauthenticated (proves the gate).

## Next Phase Readiness
- The two backend read endpoints the web Characters tab consumes (CHAR-01/02/03, INV-01..04) are wired and session-gated. `GET /api/v1/inventory/{char}` carries the full `CharacterInventory` (per-slot `icon_id`, per-character `last_seen`); `GET /api/v1/characters` carries the viewer-first roster.
- **Ready for Plan 31-03/31-04** (the SvelteKit `api.ts` `fetchInventory`/`fetchCharacters` wrappers + the `InventoryWindow`/roster components rendering these payloads) and **31-04's live deploy** (migration 00012 from 31-01 + these routes).
- The inventory endpoint is also the backend the Banks tab (P33) reuses per bank toon, and the roster `is_mine`/equipped-slot data feeds the Wishlist rework (P34).

## Self-Check: PASSED

- Created/modified files verified present on disk: `readapi/inventory.go`, `readapi/characters.go`, `store/readviews.go`, `readapi/readapi_test.go`, `cmd/squirebot-server/main.go`, this SUMMARY.
- Both task commits verified in git log: `4c43a0c` (Task 1), `506e623` (Task 2).
- Gates green: `go test ./internal/backendsrv/...` (all packages ok), `go vet ./internal/backendsrv/{store,readapi}/...` clean, `go build ./...` rc=0.
- Both routes verifiably `RequireSession`-wrapped in `main.go` (grep `RequireSession\(db, readapi\.New(Inventory|Characters)` → 2 matches); `RequireOfficer.*(inventory|characters)` → 0 matches. `{char}` bound via `r.PathValue("char")` → `compute.StructuredInventory` `?` seam; no slog line logs `char`.

---
*Phase: 31-characters-tab-in-game-inventory-window*
*Completed: 2026-06-18*
