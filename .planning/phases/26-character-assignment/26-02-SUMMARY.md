---
phase: 26-character-assignment
plan: 02
subsystem: backend (internal/backendsrv/webadmin — member + officer HTTP handlers + routes)
tags: [assignment, handlers, routes, officer-gate, idor, audit, char-meta, is_bank_toon]
requires:
  - 26-01 (store/assignment.go mutators/reads/typed errors + 5-arg SetCharMetaTx, schema v9)
provides:
  - "webadmin/assignment.go: 6 member handlers (RequireSession) + mapAssignErr (the single error mapper)"
  - "webadmin/assignment_admin.go: 6 officer handlers (RequireOfficer + in-tx IsOfficerTx)"
  - "store/assignment.go: ListClaimable (the unassigned/non-shared/live read behind /assignments/claimable)"
  - "12 mux routes (6 member + 6 officer) wired in cmd/squirebot-server/main.go"
  - "char-meta member path de-bank-toon (is_bank_toon dropped from req + payload; officer-only now)"
affects:
  - 26-03 (web UI: MyCharactersPanel + AssignmentAdminPanel call these endpoints)
  - 27 (MYVIEW), 28 (CWANT) — both build on the assignment truth this API exposes
tech-stack:
  added: []
  patterns:
    - "member-CRUD handler spine (wantlist.go): decode body (no actor field) → caller(ctx) → withTx(store mutator + AppendAuditTx) → typed-error map → writeJSON"
    - "owner-scoped silent-no-op + audit-only-on-real-change (release/cancel)"
    - "officer handler spine (officers.go) with errors routed through mapAssignErr (NOT mapOfficerErr — covers ErrCharShared)"
    - "officer assign-target validated by a dedicated SELECT 1 existence probe (NOT usernameOf — avoids the empty-username false-reject)"
    - "designate mode allow-list {bank,bot,none} → store.DesignateMode"
key-files:
  created:
    - internal/backendsrv/webadmin/assignment.go
    - internal/backendsrv/webadmin/assignment_admin.go
    - internal/backendsrv/webadmin/assignment_test.go
  modified:
    - internal/backendsrv/store/assignment.go
    - internal/backendsrv/webadmin/charmeta.go
    - cmd/squirebot-server/main.go
decisions:
  - "mapAssignErr is the SINGLE error mapper for both member and officer paths (extended with ErrNotAuthorized→403 on top of the locked ErrCharShared/ErrCharAlreadyAssigned/ErrDuplicateRequest→409); officer paths do NOT use mapOfficerErr (it lacks the ErrCharShared branch)"
  - "officer assign validates the assignee with SELECT 1 FROM web_user (existence probe), NOT usernameOf — the regression guard test seeds an EMPTY-string username (web_user.username is NOT NULL, so empty is the realizable false-reject, equivalent to NULL for usernameOf)"
  - "added store.ListClaimable + ClaimableChar in store/assignment.go (Rule 3 — the ClaimableHandler needs it and 26-01 did not ship a claimable read)"
  - "member body struct (assignmentReq) carries ONLY character_id — no discord_user_id ACTOR field anywhere (Pitfall 1); a spoofed body id is structurally ignored"
metrics:
  duration: ~50 min
  completed: 2026-06-08
  tasks: 3
  files: 6
---

# Phase 26 Plan 02: Character Assignment API Summary

**One-liner:** The Phase 26 backend HTTP surface over the 26-01 store layer — six member handlers (`webadmin/assignment.go`: list-mine/claimable/claim/release/request/cancel under `RequireSession`, identity from `caller(ctx)` with the request body carrying only `character_id`, release/cancel as owner-scoped silent IDOR no-ops, every mutation audited), six officer handlers (`webadmin/assignment_admin.go`: list-all/assign/remove/approve/deny/designate under `RequireOfficer` plus the store's in-tx `IsOfficerTx` re-check, errors routed through the shared `mapAssignErr` so a bank/bot assign surfaces a clean 409 `char_shared`, the assignee validated by a dedicated `SELECT 1 FROM web_user` existence probe that does not false-reject an empty-username user), the `store.ListClaimable` read behind `/assignments/claimable`, the twelve additive `mux.Handle` routes, and the char-meta member-path de-bank-toon reconciliation (OPEN-3 — `is_bank_toon` is now officer-only via `/admin/characters/designate`).

## What Was Built

### Task 1 — Member handlers + char-meta de-bank-toon (commit `a7cf5ee`)
- `webadmin/assignment.go` (new): `assignmentReq{CharacterID int64}` (NO actor field — Pitfall 1); `mapAssignErr` (the LOCKED switch — `ErrCharAlreadyAssigned`/`ErrCharShared`/`ErrDuplicateRequest` → 409, `ErrNotAuthorized` → 403, default 500 with `slog.Error` first; defined once here, reused by the officer handlers); `decodeAssignmentReq` (rejects `character_id <= 0` → 400 invalid_input). Handlers: `ListMyAssignmentsHandler` (GET, nil→[]), `ClaimableHandler` (GET, nil→[]), `ClaimCharHandler` (claim + `assignment_claim` audit), `ReleaseCharHandler` (owner-scoped, audit only on a real release), `RequestCharHandler` (`assignment_request` audit), `CancelRequestHandler` (requester-scoped, audit only on a real cancel). Each mutator: `caller(ctx)` + `nowUnix()` + `withTx(store *Tx + AppendAuditTx)`; audit detail `{"character_id": …}` only (V7).
- `store/assignment.go` (modified): added `ClaimableChar` + `ListClaimable` — live, `is_bank_toon=0 AND is_guild_bot=0`, and `NOT EXISTS` a `character_assignment` row, ordered by name (the unassigned/non-shared read behind `/assignments/claimable`). `ClaimCharTx`'s in-tx re-check closes the list→claim TOCTOU.
- `webadmin/charmeta.go` (modified): dropped `IsBankToon bool` from `charMetaReq`; removed `"is_bank_toon": req.IsBankToon` from the echo payload; the `SetCharMetaTx` call was already 5-arg (26-01). Doc comments rewritten to state `is_bank_toon` is now officer-only (`store.DesignateCharTx` via `/admin/characters/designate`); the member path writes only class/level/race and ignores any `is_bank_toon` body field.
- `webadmin/assignment_test.go` (member half): claim-unassigned (+audit, +read-back), claim-already-assigned → 409 already_assigned, claim-bank/bot → 409 char_shared (table), claim `character_id=0` → 400, release held vs foreign (200 released:false silent no-op, row untouched, only the real release audited), request-then-duplicate → 409 duplicate_request, cancel own vs foreign (silent no-op), **spoofed `discord_user_id` in the body ignored** (assignee is the session caller), list-mine + claimable correctness, empty-mine → `[]`.

### Task 2 — Officer handlers (commit `3b75daa`)
- `webadmin/assignment_admin.go` (new): `ListAllAssignmentsHandler` (GET → `{assignments, requests}`, each nil→[]), `OfficerAssignHandler`, `OfficerRemoveAssignHandler`, `ApproveRequestHandler`, `DenyRequestHandler`, `DesignateCharHandler`. Each: decode → `caller(ctx)` (actor, never the body) → `withTx(store *Tx + AppendAuditTx)` → `mapAssignErr`. Audit events: `officer_assign` (detail `{character_id, target}`), `assignment_remove`, `request_approve`, `request_deny`, `char_designate` (audit only on a real change for remove/approve/deny).
- `OfficerAssignHandler`: validates the body `assignee` via the LOCKED dedicated existence probe `SELECT 1 FROM web_user WHERE discord_user_id = ?` → 400 invalid_input only on `sql.ErrNoRows`, 500 on a real probe error (slog first). NOT `usernameOf` (it false-rejects an empty/NULL username).
- `DesignateCharHandler`: `designateModes` allow-list maps `{none,bank,bot}` → `store.DesignateMode`; any other mode → 400 invalid_input; `store.ErrCharNotFound` → 400 invalid_input (missing/removed char).
- `webadmin/assignment_test.go` (officer half): non-officer → 403 not_authorized on **every** officer endpoint (table-driven over assign/remove/approve/deny/designate), unknown-assignee → 400 invalid_input (no assign), **empty-username real user → 200** (the existence-probe regression guard), bank-char assign → 409 char_shared (proves `mapAssignErr` not `mapOfficerErr`), **approve denies the sibling pending request** (Pitfall 3 — pending count 2→0, char reassigned to the approved requester), designate-bank clears an existing assignment (Pitfall 6) + sets `is_bank_toon=1` + audits, invalid mode → 400, list-all returns the assignment + the pending request.

### Task 3 — Route registration (commit `464599f`)
- `cmd/squirebot-server/main.go` (modified): 6 member routes (`GET /api/v1/assignments/mine|claimable`, `POST /api/v1/assignments/claim|release|request|request/cancel`) under `RequireSession`, placed after the wantlist block; 6 officer routes (`GET /api/v1/admin/assignments`, `POST /api/v1/admin/assignments/assign|remove|approve|deny`, `POST /api/v1/admin/characters/designate`) under `RequireOfficer`, placed after the monitors block. Single-space method pattern matching the existing lines. The char-meta comment was updated for the OPEN-3 reconciliation. The char-meta routes themselves are UNCHANGED (`POST /api/v1/char/meta` stays `RequireSession`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `store.ListClaimable` + `ClaimableChar` to `store/assignment.go`**
- **Found during:** Task 1 — the plan's `ClaimableHandler` (GET /assignments/claimable, "the unassigned, non-shared characters JSON") requires a store read that 26-01 did not ship (26-01 provided `ListMyAssignments`/`ListAllAssignments`/`ListPendingRequests` but no claimable read).
- **Fix:** Added `ClaimableChar{character_id, name}` + `ListClaimable(ctx, db)` — live (`is_removed=0`), non-shared (`is_bank_toon=0 AND is_guild_bot=0`), and `NOT EXISTS` a `character_assignment` row, ordered by name. Parameterized; nil→[] normalized in the handler. The list→claim TOCTOU is closed by `ClaimCharTx`'s in-tx shared/already-assigned re-check.
- **Files modified:** `internal/backendsrv/store/assignment.go`
- **Commit:** `a7cf5ee`

**2. [Rule 1 - Test correctness] Empty-string (not literal NULL) username for the existence-probe regression guard**
- **Found during:** Task 2 — the first cut of `TestOfficerAssign_NullUsernameAssignee_Succeeds` inserted a `web_user` with a literal `NULL` username and failed: `web_user.username` carries a `NOT NULL` constraint (00004), so a NULL is unrepresentable.
- **Fix:** Seed an **empty-string** username instead. `usernameOf` returns `""` for an empty username exactly as it would for a (hypothetical) NULL, so the empty-string case is the realizable false-reject and is a faithful guard for the probe-vs-usernameOf distinction the plan targets (T-26-11). Test comment documents the schema reason.
- **Files modified:** `internal/backendsrv/webadmin/assignment_test.go`
- **Commit:** `3b75daa`

## Verify Gate Results (run from repo root)

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./...` | PASS (exit 0) |
| Vet (backend) | `go vet ./internal/backendsrv/...` | PASS (clean, exit 0) |
| Vet (server) | `go vet ./cmd/squirebot-server/...` | PASS (clean, exit 0) |
| Test | `go test ./internal/backendsrv/...` | PASS — every package `ok` |

Spot-checks within the gate:
- `go test ./internal/backendsrv/webadmin/ -run 'Claim|Release|Request|Cancel|CharMeta|ListMy'` → `ok` (member half + the charmeta de-bank-toon test, incl. silent no-op, duplicate-request, char_shared, spoofed-id-ignored).
- `go test ./internal/backendsrv/webadmin/ -run 'Officer|Approve|Deny|Designate|AllAssign'` → `ok` (officer half, incl. non-officer 403 on every endpoint, the empty-username regression guard, bank assign 409 char_shared, Pitfall 3 sibling-deny, Pitfall 6 designate-clears).
- `go test ./internal/backendsrv/...` → all packages `ok` (`store` 54.4s, `webadmin` 40.7s — both run the migration suite per test DB).

Verification greps (from the plan):
- assignment.go has NO `discord_user_id` ACTOR struct field (the only matches are doc comments + the `caller`-scoped store WHERE clauses described in prose) — CONFIRMED.
- The officer handlers map errors via `mapAssignErr` (5 call sites) and NEVER `mapOfficerErr`; `OfficerAssign` uses `SELECT 1 FROM web_user WHERE discord_user_id = ?` (not `usernameOf`) — CONFIRMED.
- All 6 member routes use `webauth.RequireSession`, all 6 officer routes use `webauth.RequireOfficer` in main.go (lines 368–373, 397–402) — CONFIRMED.

## Notes for Downstream (26-03 web UI)

- Member endpoints return `{claimed|released|requested|cancelled: bool}`; `release`/`cancel` return `false` for a foreign/non-pending target (silent no-op — the UI should treat `false` as "nothing to do", not an error). `GET /assignments/mine` → `[]store.Assignment`, `GET /assignments/claimable` → `[]store.ClaimableChar` (both `[]` never null).
- Officer `GET /admin/assignments` → `{assignments: []Assignment, requests: []PendingRequest}`. `assign` body is `{character_id, assignee}`; `designate` body is `{character_id, mode}` with `mode ∈ {bank,bot,none}`; approve/deny take `{request_id}`; remove takes `{character_id}`.
- Error codes the frontend routes: `already_assigned` / `char_shared` / `duplicate_request` (409), `not_authorized` (403), `invalid_input` (400), `internal` (500).
- The member char-meta form (26-03) must DROP the is_bank_toon checkbox — the member path no longer writes it; designation is the officer designate radio (bank/bot/none).

## Known Stubs

None — every handler is fully wired to the 26-01 store mutators/reads; no placeholder values, no mock data sources. `store.ListClaimable` is a real query, not a stub.

## Threat Flags

None — no security surface beyond the plan's `<threat_model>` (T-26-08..14) was introduced. The 12 new routes are all gated (`RequireSession` / `RequireOfficer`), the officer mutators re-check `IsOfficerTx` in-tx, identity is server-derived, and every mutation audits.

## Self-Check: PASSED

- Created files exist:
  - `internal/backendsrv/webadmin/assignment.go` — FOUND
  - `internal/backendsrv/webadmin/assignment_admin.go` — FOUND
  - `internal/backendsrv/webadmin/assignment_test.go` — FOUND
- Commits exist on `master`:
  - `a7cf5ee` (Task 1) — FOUND
  - `3b75daa` (Task 2) — FOUND
  - `464599f` (Task 3) — FOUND
