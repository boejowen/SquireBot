---
phase: 15-admin-web-forms-login
plan: 01
subsystem: database
tags: [sqlite, goose, discord-oauth, sessions, sha256, audit-log, eviction, admin-allowlist]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api
    provides: "store package conventions (Open DSN, NewStore, NewTestDB, *Tx mutator pattern), goose //go:embed migrations, owner/character/guild_code schema, auth/guard.go sha256 hashing + auth/mint.go crypto/rand idiom"
  - phase: 14-web-frontend
    provides: "read API + exact-origin CORS the session gate will wrap; bank view that consumes the new coin columns"
provides:
  - "00004_web_auth.sql forward-only migration: web_user, web_session (hashed), guild_admins, app_config tables + 4 nullable coin columns + grace_until/archived_at + generic audit_log actor/detail/at columns"
  - "websession.go: GenerateSessionID, HashSession, UpsertWebUser, GetWebUser, CreateSession, ResolveSession (fail-closed), TouchSession, DeleteSession + SessionTTLSeconds"
  - "admins.go: IsOfficer, ListOfficers, ListPromotableUsers, GetOwnerFloor, SetOwnerFloor, AddOfficerTx, RemoveOfficerTx (ported admin.ts semantics: fail-closed, authorize-under-tx, idempotent, owner-floor protected)"
  - "eviction.go: ListEvictableOwners, PreviewEviction, EvictOwnerTx (cascade + code-revoke in one tx), RestoreOwnerTx, ArchiveExpiredEvictions"
  - "coin.go: ListBankToons, GetCoin, SetCoinTx (bank-toon-gated)"
affects: [15-02, 15-03, 15-04, 15-05, 16-cutover]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Opaque session, hash-only at rest: plaintext only in the cookie, sha256 hex in web_session.session_hash (mirrors guild_code bearer discipline)"
    - "Authorize-under-transaction (*Tx mutators re-check caller IsOfficer as their first SELECT) to close the v1 WR-04 TOCTOU window"
    - "Eviction as one app-controlled action: is_removed cascade + grace_until + guild_code.disabled_at revoke in a single tx (D-10)"
    - "INTEGER epoch-seconds timestamps on the new self-contained tables; existing TEXT datetime('now') columns (guild_code.disabled_at) keep their type"

key-files:
  created:
    - internal/backendsrv/migrations/00004_web_auth.sql
    - internal/backendsrv/store/websession.go
    - internal/backendsrv/store/admins.go
    - internal/backendsrv/store/eviction.go
    - internal/backendsrv/store/coin.go
    - internal/backendsrv/store/websession_test.go
    - internal/backendsrv/store/admins_test.go
    - internal/backendsrv/store/eviction_test.go
    - internal/backendsrv/store/coin_test.go
  modified:
    - internal/backendsrv/migrations/migrate_test.go

key-decisions:
  - "guild_admins is its own table keyed by Discord snowflake (not an is_admin column on web_user) — mirrors v1's distinct allowlist + keeps promotable-user query a simple NOT IN"
  - "owner-floor is an app_config['owner_floor_discord_id'] singleton row (not a column) — generic key/value table reused for future single-value ops config"
  - "SetOwnerFloor seeds a minimal placeholder web_user (username=snowflake) when the floor hasn't logged in, so the guild_admins FK holds and the floor shows in ListOfficers immediately (replaces v1's onOpen/getOwner bootstrap)"
  - "Coin columns are *int64 in Go (nullable) so an entered 0 is distinguishable from never-entered (NULL) — the form pre-fills from these"
  - "Eviction archive is a lazy/scheduled UPDATE (ArchiveExpiredEvictions) gated by archived_at IS NULL for idempotency — wiring to a job vs lazy-on-read is a 15-03 call"

patterns-established:
  - "commitTx test helper (COMMIT-on-success) distinct from enrich_test.go's rollback-only withTx — multi-step assertions read committed state"
  - "Generic audit_log extension (actor/detail/at) reused, not a parallel log (D-06)"

requirements-completed: [AUTH-08, AUTH-09, ADMIN-04, ADMIN-05, ADMIN-06]

# Metrics
duration: 13min
completed: 2026-05-31
---

# Phase 15 Plan 01: Schema + Store Foundation Summary

**Forward-only `00004_web_auth.sql` (Discord web users, sha256-hashed opaque sessions, Discord-ID-keyed officer allowlist, CLI-seeded owner-floor, nullable bank-coin columns, eviction grace/archive, generic audit_log columns) plus four parameterized store files porting v1 admin.ts + eviction enforcement SEMANTICS into Go/SQL transactions.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-05-31T01:18:02Z
- **Completed:** 2026-05-31T01:31:06Z
- **Tasks:** 3
- **Files modified:** 10 (9 created + 1 modified)

## Accomplishments

- **Migration `00004_web_auth.sql`** applies 00001→00004 cleanly on a fresh DB and re-runs as a no-op (idempotent, proven via the shared `NewTestDB` fixture). Adds 4 new tables, 6 new `character` columns (4 coin + grace_until + archived_at), and 3 generic `audit_log` columns (actor/detail/at) — extend-only, the existing ingest-specific audit_log shape untouched.
- **Sessions store only the SHA-256 hash** of the opaque session id (T-15-01) — `websession_test.go` proves a query for the plaintext returns zero rows while the hash returns exactly one. `ResolveSession` is fail-closed: `ErrSessionExpired` on a stale row, `ErrSessionNotFound` on a miss. `UpsertWebUser` is idempotent with `first_seen` immutable across re-logins.
- **Officer model ports admin.ts verbatim** (semantics, not storage): fail-closed `IsOfficer`, idempotent `AddOfficerTx`/`RemoveOfficerTx` that re-check the caller's officer status as their FIRST in-tx SELECT (authorize-under-transaction, closes the v1 WR-04 TOCTOU window), owner-floor protection (`ErrOwnerFloorProtected` before any write), and the self-removal orphan-pointer rule. Error strings match v1 (`not_authorized` / `owner_floor_protected`).
- **Eviction is one app-controlled transaction** (D-10): `EvictOwnerTx` cascades `is_removed=1` + stamps `grace_until` across all the owner's live characters AND sets `guild_code.disabled_at` to revoke their code — proven by `eviction_test.go` (both effects in one tx; a second owner's data untouched). `RestoreOwnerTx` reverses during grace (skips archived); `ArchiveExpiredEvictions` is idempotent past-grace.
- **Coin writes are bank-toon-gated** at the data layer (T-15-04): `SetCoinTx` returns `ErrNotBankToon` for a non-bank-toon or missing target; coin values round-trip as `*int64` so an entered 0 is distinguishable from unset.

## Task Commits

Each task was committed atomically:

1. **Task 1: 00004_web_auth.sql migration (+ migrate_test extension)** - `abb92a0` (feat)
2. **Task 2: Session + web-user store methods (websession.go)** - `78d7e24` (feat)
3. **Task 3: Officer/owner-floor + eviction + coin store methods** - `4b39e40` (feat)

**Plan metadata:** see final docs commit.

_TDD note: Tasks 1 and 2 followed RED→GREEN (test written first, confirmed failing, then implementation). Task 3 wrote implementation + tests together within the single grouped task, all green._

## Files Created/Modified

- `internal/backendsrv/migrations/00004_web_auth.sql` - The forward-only migration (web_user, web_session, guild_admins, app_config; coin + grace/archive character columns; generic audit_log columns).
- `internal/backendsrv/migrations/migrate_test.go` - Extended with `TestMigrate_00004_AddsWebAuthSchema` (4 tables + 6 character cols + 3 audit_log cols + idempotent re-run) and a `columnSet` PRAGMA helper.
- `internal/backendsrv/store/websession.go` - Session id mint/hash + web-user upsert + session create/resolve/touch/delete; `SessionTTLSeconds=2592000`.
- `internal/backendsrv/store/admins.go` - Officer allowlist + owner-floor, ported admin.ts semantics.
- `internal/backendsrv/store/eviction.go` - Per-owner cascade + code-revoke + grace/archive.
- `internal/backendsrv/store/coin.go` - Bank-toon-gated coin read/write.
- `internal/backendsrv/store/{websession,admins,eviction,coin}_test.go` - Behavior coverage for every plan bullet (hash-only storage, fail-closed resolve, idempotency, owner-floor block, one-tx cascade+revoke, bank-toon gate, idempotent archive).

## Decisions Made

- **`guild_admins` is its own table** (not an `is_admin` column on `web_user`) — keeps the v1 distinct-allowlist shape and makes `ListPromotableUsers` a clean `NOT IN` subquery.
- **Owner-floor lives in `app_config['owner_floor_discord_id']`** (singleton key/value row, not a dedicated column) — generic table reusable for future single-value ops config (Claude's-discretion item resolved this way per the plan's recommendation).
- **`SetOwnerFloor` seeds a placeholder `web_user`** (username = the snowflake) when the floor has not logged in yet, so the `guild_admins` FK holds and the floor is the bootstrap officer immediately. On the floor's first real login, `UpsertWebUser` refreshes the placeholder username/avatar in place. This replaces v1's `onOpen`/`getOwner()` bootstrap (gone in the DB world).
- **Coin columns surface as `*int64`** so an entered `0` (non-nil pointer) is distinguishable from never-entered (`NULL` → nil) — required because the form pre-fills from these values.
- **Eviction archive is a lazy/scheduled `UPDATE`** gated by `archived_at IS NULL` for idempotency; whether 15-03 wires it into the in-process scheduler or runs it lazy-on-read is left open per the plan.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed test helper to avoid a name collision**
- **Found during:** Task 3 (officer/eviction/coin tests)
- **Issue:** The new test files declared a `withTx(t, ctx, db, fn)` commit-on-success helper, but `enrich_test.go` already declares a `withTx(t, db, fn)` rollback-only helper in the same `store` package — a redeclaration compile error.
- **Fix:** Renamed the new helper to `commitTx` (commit-on-success, distinct from the existing rollback-only `withTx`) and updated all 21 call sites across `admins_test.go`/`eviction_test.go`/`coin_test.go`.
- **Files modified:** internal/backendsrv/store/admins_test.go, eviction_test.go, coin_test.go
- **Verification:** `go test ./internal/backendsrv/store/...` exits 0.
- **Committed in:** `4b39e40` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking).
**Impact on plan:** Test-only rename to satisfy the compiler; zero impact on production code or plan scope. No scope creep.

## Issues Encountered

- The `GOOS=linux GOARCH=amd64` cross-compile check emits a `squirebot-server` binary into the repo root; it is already covered by `.gitignore` (`/squirebot-server`) and was removed after the check. No commit contamination.

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are enforced at the store layer:
- **T-15-01** (session at rest) — `CreateSession` stores only `HashSession(id)`; test proves the plaintext is never persisted.
- **T-15-02** (privilege escalation on RemoveOfficerTx) — owner-floor protection before any write + authorize-under-transaction first-SELECT re-check.
- **T-15-03** (SQL injection) — every query uses `?` placeholders; no string interpolation.
- **T-15-04** (coin onto non-bank-toon) — `SetCoinTx` gate → `ErrNotBankToon`.
- **T-15-05** (repudiation) — accepted for this plan; the generic audit_log columns are added here, but the audited writes compose at the handler layer in 15-03. No new threat surface beyond the register.

## User Setup Required

None for this plan (local build-and-verify only). Deferred to the deploy step (per STATE.md Phase 15 directives, NOT done this run):
- The 4 `DISCORD_*` vars go in the `squirebot-server` systemd unit (root-only `EnvironmentFile=`, chmod 600).
- Run `squirebot-server set-owner-floor <maintainer-discord-USER-id>` once on the box (the `SetOwnerFloor` store method this plan added is the logic behind that subcommand, wired in 15-02/15-03).
- `goose` applies `00004` on the next binary restart on the VPS.

## Next Phase Readiness

- **15-02 (backend auth)** can now build the Discord OAuth2 callback + session-gate middleware on top of `UpsertWebUser`/`CreateSession`/`ResolveSession`/`TouchSession`/`DeleteSession`, and the `set-owner-floor` CLI on `SetOwnerFloor`.
- **15-03 (write surface)** has the `*Tx` mutators (`AddOfficerTx`/`RemoveOfficerTx`/`EvictOwnerTx`/`RestoreOwnerTx`/`SetCoinTx`) ready to compose under an audited transaction, plus the generic `audit_log` columns to write into.
- **15-04/15-05 (frontend)** have the `Officer`/`EvictableOwner`/`BankToon` JSON-tagged read structs (`ListOfficers`/`ListPromotableUsers`/`ListEvictableOwners`/`ListBankToons`/`GetCoin`) for the forms.
- No blockers. `go build ./...`, `GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server`, `go vet ./internal/backendsrv/...`, and `go test ./...` all pass.

## Self-Check: PASSED

All 9 created files + 1 modified file verified present on disk; all 3 task commit hashes (`abb92a0`, `78d7e24`, `4b39e40`) verified in git history.

---
*Phase: 15-admin-web-forms-login*
*Completed: 2026-05-31*
