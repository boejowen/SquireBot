---
phase: 26-character-assignment
plan: 01
subsystem: backend (internal/backendsrv — store + migrations + compute)
tags: [schema, store, assignment, is_bank_toon, sqlite, goose]
requires:
  - 00008 (schema v8) applied; web_user (00004), owner.discord_user_id (00005)
provides:
  - "schema v9: character_assignment (single-assignee PK), assignment_request (pending|approved|denied|cancelled + partial-unique-pending), character.is_guild_bot, idempotent auto-seed"
  - "store/assignment.go: 9 Tx mutators + 3 List reads + 4 typed errors"
  - "relaxed single-bank invariant: SetCharMetaTx 5-arg, multiple guild banks render"
affects:
  - 26-02 (API plan: composes these store mutators + the 5-arg SetCharMetaTx)
  - 27 (MYVIEW), 28 (CWANT) — both build on the assignment truth
tech-stack:
  added: []
  patterns:
    - "goose forward-only migration (partial-unique + CHECK + INSERT OR IGNORE auto-seed)"
    - "authorize-under-transaction (isOfficerTx first stmt, WR-04 TOCTOU)"
    - "owner-scoped silent-no-op (IDOR defense)"
    - "single-assignee = character_id PRIMARY KEY (schema, not store logic)"
    - "partial-unique conflict → typed error via modernc extended result code 2067"
key-files:
  created:
    - internal/backendsrv/migrations/00009_character_assignment.sql
    - internal/backendsrv/store/assignment.go
    - internal/backendsrv/store/assignment_test.go
  modified:
    - internal/backendsrv/migrations/migrate_test.go
    - internal/backendsrv/store/charmeta.go
    - internal/backendsrv/store/charmeta_test.go
    - internal/backendsrv/compute/bank.go
    - internal/backendsrv/compute/bank_test.go
    - internal/backendsrv/webadmin/charmeta.go
    - internal/backendsrv/webadmin/charmeta_test.go
decisions:
  - "guild bank SUBSUMES is_bank_toon (no parallel column); officer-only via DesignateCharTx; single-bank invariant relaxed"
  - "guild bot = new is_guild_bot column (no existing analog)"
  - "DesignateMode 3-state (bank|bot|neither, mutually exclusive) — resolves Open Question 3"
  - "approval = immediate reassignment + deny all sibling pending requests (Open Question 2 / Pitfall 3)"
  - "NO _meta.schema_version write (goose tracks version; backend-only, watcher untouched)"
metrics:
  duration: ~45 min
  completed: 2026-06-08
  tasks: 3
  files: 10
---

# Phase 26 Plan 01: Character Assignment Data Layer Summary

**One-liner:** The `00009` schema-v9 migration (single-assignee `character_assignment` PK table, contested-claim `assignment_request` queue with a partial-unique-pending index, the new `is_guild_bot` column, and an idempotent owner-linked auto-seed) plus the `store/assignment.go` mutators/reads/typed-errors that enforce single-assignee + the bidirectional shared-char exemption + double-approval denial, and the `is_bank_toon` reconciliation that relaxes the single-bank invariant so multiple officer-designated guild banks render in the consolidated bank view.

## What Was Built

### Task 1 — `00009` migration + migrate test (commit `aff1553`)
- `00009_character_assignment.sql`: `ALTER TABLE character ADD COLUMN is_guild_bot`; `character_assignment` (`character_id INTEGER PRIMARY KEY` → single-assignee is the schema, `discord_user_id`/`assigned_at`/`assigned_by`, user index); `assignment_request` (id PK, `character_id`, `requester`, `current_assignee`, `status CHECK IN (pending|approved|denied|cancelled)`, `created_at`/`resolved_at`/`resolved_by`, char/requester indexes, and the partial `CREATE UNIQUE INDEX … WHERE status='pending'`); idempotent `INSERT OR IGNORE … SELECT` auto-seed from `owner.discord_user_id` excluding NULL-owner / `is_removed` / `is_bank_toon` / `is_guild_bot`; explicit no-op `Down`. No `_meta.schema_version` cell (goose tracks version).
- `TestMigrate_00009_CharacterAssignment`: asserts the `is_guild_bot` column, both tables, the `assignment_request_pending_uidx` index, the `status='bogus'` CHECK rejection, the partial-unique pending collision (and that a resolved/denied row frees the index for a new pending), the seed inclusion (linked-owner char → `assigned_by='migration'`) + exclusion (NULL-owner, bank, removed), and an idempotent re-run (goose version count unchanged). Added local `mustInsertOwner`/`mustInsertChar` helpers.

### Task 2 — `store/assignment.go` mutators, reads, typed errors (commit `0071847`)
- Typed sentinels: `ErrCharAlreadyAssigned`, `ErrCharShared`, `ErrNotAssignee`, `ErrDuplicateRequest` (reuses existing `store.ErrNotAuthorized` for officer-gate failures).
- Member mutators: `ClaimCharTx` (pre-checks shared → `ErrCharShared`, existing assignment → `ErrCharAlreadyAssigned`, then plain INSERT `assigned_by='self'`), `ReleaseCharTx` (owner-scoped DELETE → `(removed bool)`, foreign row = silent no-op), `RequestTx` (files pending; partial-unique conflict → `ErrDuplicateRequest` via modernc code 2067), `CancelRequestTx` (requester-scoped → `(cancelled bool)`).
- Officer mutators (each calls `isOfficerTx` FIRST — WR-04): `OfficerAssignTx` (`ON CONFLICT(character_id) DO UPDATE` reassign/override; rejects shared), `RemoveAssignTx`, `ApproveRequestTx` (upserts assignment to requester AND denies ALL OTHER pending requests for the char — Pitfall 3), `DenyRequestTx`, `DesignateCharTx` (3-state `DesignateMode`; sets bank/bot mutually exclusive, does NOT demote other banks, and on bank/bot clears the assignment + denies pending requests — Pitfall 6 bidirectional exemption).
- Reads: `ListMyAssignments`, `ListAllAssignments`, `ListPendingRequests` (joined to character.name, live chars only).
- `assignment_test.go`: 14 test functions covering claim/already-assigned, claim-shared (bank+bot), foreign-row release no-op, duplicate-request, requester-scoped cancel, officer assign/reassign-single-row, officer-assign-shared, remove idempotent, **double-approval sibling-deny (Pitfall 3)**, deny, **designate clears assignment + denies requests + does NOT demote other banks (Pitfall 6)**, bot mutual exclusion, missing-char, and the three List reads. Every officer path also asserts the non-officer → `ErrNotAuthorized` rejection.

### Task 3 — `is_bank_toon` reconciliation (commit `98c8ddb`)
- `store/charmeta.go`: `SetCharMetaTx` is now 5-arg (dropped `isBankToon`); deleted the MD-01 demote block; UPDATE writes only `class/level/race`; doc comments rewritten to point at the officer-only `DesignateCharTx`.
- `store/charmeta_test.go`: removed the three demote/re-save/clear regression tests + the now-unused `countBankToons`/`isBankToon` oracles; updated the surviving write + ErrCharNotFound tests to the 5-arg signature.
- `compute/bank.go`: doc comment now states "all guild-bank characters; rows carry their Char (consolidated, multiple banks supported)"; **NO query change** (`WHERE c.is_bank_toon = 1` already returns all).
- `compute/bank_test.go`: added `TestBank_MultipleBankToonsRender` — two `is_bank_toon=1` characters both render, grouped by Char, nothing merged/dropped (Pitfall 5).
- `WatcherMaxSchemaVersion` NOT touched (backend-only; watcher untouched per CLAUDE.md).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated the `webadmin/charmeta.go` call site + its test to the 5-arg `SetCharMetaTx`**
- **Found during:** Task 3 (the verify gate's `go build ./...` and `go test ./internal/backendsrv/...`).
- **Issue:** Dropping the `isBankToon` param from `SetCharMetaTx` broke its only caller (`webadmin/charmeta.go:106`, which passed `req.IsBankToon`) and a webadmin handler test (`TestCharMetaSet_NonOfficerCanWrite` asserted the handler persists `is_bank_toon=1`). The plan assigns the full `webadmin/charmeta.go` req-struct/payload/route reconciliation to plan 26-02, but the build/test gate cannot pass with a stale caller.
- **Fix:** Minimal build-fix — dropped only the `req.IsBankToon` arg from the call (the `charMetaReq` field, echo payload, and route stay for 26-02). Updated the handler test's assertion to expect `is_bank_toon=0` (the member char-meta path no longer writes it; it is officer-only now). Task 3's own plan text already directs "update any call sites of SetCharMetaTx to the new 5-arg signature," so this is within scope.
- **Files modified:** `internal/backendsrv/webadmin/charmeta.go`, `internal/backendsrv/webadmin/charmeta_test.go`
- **Commit:** `98c8ddb`

## Verify Gate Results (run from repo root)

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./...` | PASS (exit 0) |
| Vet | `go vet ./internal/backendsrv/...` | PASS (clean, exit 0) |
| Test | `go test ./internal/backendsrv/...` | PASS — all packages `ok` |

Spot-checks within the gate:
- `go test ./internal/backendsrv/migrations/ -run TestMigrate_00009` → `ok` (00009 applies; auto-seed inclusion/exclusion + 2-bank-safe partial-unique + idempotent re-run all asserted).
- `go test ./internal/backendsrv/store/ -run 'Assign|Claim|Release|Request|Designate'` → `ok` (all 14 assignment cases incl. Pitfall 3 + Pitfall 6).
- `go test ./internal/backendsrv/compute/` → `ok` (incl. `TestBank_MultipleBankToonsRender`).
- `go test ./internal/backendsrv/webadmin/` → `ok` (charmeta handler tests green against the 5-arg signature).

The migration file is named `00009_character_assignment.sql` (goose version 9) and contains no `_meta.schema_version` write.

## Notes for Downstream (26-02)

- The store mutators take `callerID`/`assignee`/`now` as params — the handler supplies `caller(ctx)` (session, never body) + `nowUnix()` + composes `AppendAuditTx` in the same `withTx`. Audit is NOT in the store layer.
- `DesignateCharTx` takes a `store.DesignateMode` (`DesignateNeither|DesignateBank|DesignateBot`); the officer designate handler maps a 3-way radio to it.
- `webadmin/charmeta.go` still carries `IsBankToon` in `charMetaReq` + the echo payload (now inert) — 26-02 removes the field + the member-facing checkbox and adds the officer designate route.
- Officer mutators return `store.ErrNotAuthorized` (→ 403) and `ErrCharShared`/`ErrCharAlreadyAssigned`/`ErrDuplicateRequest` (→ 409) for the `mapAssignErr` switch.

## Known Stubs

None — this plan is a complete data layer; no placeholder values, no unwired data sources. The `webadmin/charmeta.go` `IsBankToon` request field is intentionally left inert (documented above) and is removed in 26-02 per the plan.

## Self-Check: PASSED

- Created files exist:
  - `internal/backendsrv/migrations/00009_character_assignment.sql` — FOUND
  - `internal/backendsrv/store/assignment.go` — FOUND
  - `internal/backendsrv/store/assignment_test.go` — FOUND
- Commits exist on `master`:
  - `aff1553` (Task 1) — FOUND
  - `0071847` (Task 2) — FOUND
  - `98c8ddb` (Task 3) — FOUND
