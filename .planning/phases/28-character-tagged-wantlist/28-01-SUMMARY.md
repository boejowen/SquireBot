---
phase: 28-character-tagged-wantlist
plan: 01
subsystem: database
tags: [sqlite, goose, migration, wantlist, idor, coalesce-dedup, go]

# Dependency graph
requires:
  - phase: 19-wantlist-crud
    provides: wantlist_item table + AddWantTx/ListOwnWants store + partial-unique dedup indexes (00006)
  - phase: 26-character-assignment
    provides: character_assignment table (00009) + charSharedTx in-tx probe pattern
provides:
  - "Migration 00010: nullable wantlist_item.character_id (NULL backfill) + COALESCE(character_id,-1) dedup-index rewrite"
  - "AddWantTx extended with optional characterID *int64 (persisted)"
  - "WantlistRow + ListOwnWants surface character_id + character_name (LEFT JOIN character)"
  - "NEW ListGuildWants: all-members guildwide read with owner username + optional char, EXCLUDES private note"
  - "NEW IsCharAssignedToTx in-tx IDOR guard + ErrCharNotAssigned sentinel"
affects: [28-02 (wantlist API handler — calls IsCharAssignedToTx + AddWantTx with tag + serves ListGuildWants), 28-03 (web UI — character group/filter + guildwide roll-up)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "COALESCE(col,-1) sentinel in a partial-unique index to make NULL collide-as-one while real ids stay distinct (preserves account-level dedup + allows per-character rows)"
    - "In-tx existence-probe authorization (IsCharAssignedToTx mirrors charSharedTx) so a body-supplied id is authorized under the same tx as the insert (TOCTOU-safe)"
    - "Guildwide read as a distinct func (ListGuildWants) — NOT a forked ListOwnWants — that JOINs web_user for owner and intentionally omits the private note column"

key-files:
  created:
    - internal/backendsrv/migrations/00010_character_tagged_wantlist.sql
  modified:
    - internal/backendsrv/migrations/migrate_test.go
    - internal/backendsrv/store/wantlist.go
    - internal/backendsrv/store/wantlist_test.go
    - internal/backendsrv/store/assignment.go
    - internal/backendsrv/store/assignment_test.go
    - internal/backendsrv/store/alertlog_test.go
    - internal/backendsrv/webadmin/wantlist.go

key-decisions:
  - "Migration is 00010 (new file) — 00001-00009 untouched (extend-only doctrine verified via git diff)"
  - "character_id is NULLABLE with NO ON DELETE CASCADE: chars are soft-removed (is_removed=1), never hard-deleted, so cascade never fires; a dangling tag yields NULL character_name via the read-side LEFT JOIN"
  - "ListGuildWants EXCLUDES the private note column (T-28-02) — note stays owner-scoped to ListOwnWants"
  - "webadmin add-want endpoint passes nil characterID — the tag path is wired in Plan 02 (which authorizes via IsCharAssignedToTx)"

patterns-established:
  - "COALESCE(character_id,-1) partial-unique dedup: NULL→one sentinel (account-level dedup preserved), real ids distinct (same item for two chars = two rows)"
  - "IsCharAssignedToTx: SELECT 1 FROM character_assignment WHERE character_id=? AND discord_user_id=? — sql.ErrNoRows→(false,nil) never leaks existence"

requirements-completed: [CWANT-01, CWANT-02, CWANT-03, CWANT-04, CWANT-06]

# Metrics
duration: 18min
completed: 2026-06-09
---

# Phase 28 Plan 01: Character-Tagged Wantlist Data Layer Summary

**Migration 00010 adds a nullable `wantlist_item.character_id` with a COALESCE(character_id,-1) dedup-index rewrite (account-level dedup preserved, same item for two chars allowed), plus the store threading (AddWantTx tag, ListOwnWants char name, new guildwide ListGuildWants) and the IsCharAssignedToTx IDOR guard.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-06-09T02:55:58Z
- **Completed:** 2026-06-09T03:13:44Z
- **Tasks:** 3
- **Files modified:** 7 (1 created, 6 modified)

## Accomplishments
- **Migration 00010** (schema v9 → v10): `ALTER TABLE wantlist_item ADD COLUMN character_id INTEGER REFERENCES character(id)` (nullable, auto-NULL backfill — no data-migration), and both 00006 dedup indexes (`wantlist_catalog_uidx`/`wantlist_custom_uidx`) rewritten to key on `COALESCE(character_id, -1)`. Forward-only Down (`SELECT 1;`). 00001–00009 untouched (extend-only verified).
- **Store threading:** `AddWantTx` gained an optional `characterID *int64` (just before `now`) and persists it; the `ErrDuplicateWant` extended-result-code (2067) detection is byte-unchanged. `WantlistRow` + `ListOwnWants` now LEFT JOIN `character` and surface `character_id` + `character_name`.
- **NEW `ListGuildWants(ctx, db)`** + `GuildWantRow`: every active want across all members, JOIN `web_user` for the owner username, LEFT JOIN `character` for the optional tag name, **EXCLUDES the private `note`** (T-28-02), non-nil empty slice → JSON `[]`.
- **NEW `IsCharAssignedToTx`** + `ErrCharNotAssigned`: in-tx existence probe (mirrors `charSharedTx`) returning `(true,nil)` only for a real `(characterID, callerID)` assignment, `(false,nil)` otherwise (never leaks existence), `%w`-wrapped on other errors.

## Task Commits

Each task was committed atomically (TDD: RED test was confirmed failing before each GREEN where the test file is separate; the 00010 migration test was proven RED — "table wantlist_item has no column named character_id" — before the migration was written):

1. **Task 1: Migration 00010 — nullable character_id + COALESCE dedup rewrite** - `fb9a1f6` (feat)
2. **Task 2: Store — thread character_id through AddWantTx/ListOwnWants + NEW ListGuildWants** - `c49863a` (feat)
3. **Task 3: Store — IsCharAssignedToTx IDOR guard + ErrCharNotAssigned sentinel** - `00cf8c7` (feat)

## Files Created/Modified
- `internal/backendsrv/migrations/00010_character_tagged_wantlist.sql` (NEW) - nullable `character_id` + COALESCE-keyed dedup-index rewrite.
- `internal/backendsrv/migrations/migrate_test.go` - `TestMigrate_00010_CharacterTaggedWantlist`: asserts the column exists, the NULL backfill, account-level (NULL) dedup still collides, and the same item for two distinct chars produces two rows (catalog + custom paths).
- `internal/backendsrv/store/wantlist.go` - `AddWantTx` +characterID; `WantlistRow`/`ListOwnWants` +char; new `GuildWantRow` + `ListGuildWants`.
- `internal/backendsrv/store/wantlist_test.go` - tag-persist, nil-tag-as-NULL, and guildwide all-members read tests; existing `AddWantTx` callers updated for the new signature.
- `internal/backendsrv/store/assignment.go` - `IsCharAssignedToTx` + `ErrCharNotAssigned`.
- `internal/backendsrv/store/assignment_test.go` - `TestIsCharAssignedToTx` (true / assigned-to-other / unassigned / nonexistent).
- `internal/backendsrv/store/alertlog_test.go` - `seedWant` updated for the new `AddWantTx` signature.
- `internal/backendsrv/webadmin/wantlist.go` - add-want handler passes `nil` characterID (tag path lands in Plan 02).

## Decisions Made
- **character_id has NO `ON DELETE CASCADE`**: characters are soft-removed (`is_removed=1`), never hard-deleted, so a cascade would never fire; a dangling tag is rendered NULL by the read-side LEFT JOIN rather than destroying the want.
- **ListGuildWants is a distinct function**, not a forked/scope-stripped ListOwnWants (the plan's named anti-pattern), and it omits `note` entirely (struct + SELECT) per the security recommendation.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated downstream AddWantTx callers for the new signature**
- **Found during:** Task 2 (AddWantTx signature change)
- **Issue:** Adding `characterID *int64` to `AddWantTx` broke its existing callers (the production `webadmin/wantlist.go` handler, plus the test seed helpers in `wantlist_test.go` and `alertlog_test.go`) — the store package would not compile.
- **Fix:** Passed `nil` for the new param at every existing call site (the account-level behavior is unchanged); the production handler carries a comment that the real tag path is wired in Plan 02 via `IsCharAssignedToTx`.
- **Files modified:** internal/backendsrv/webadmin/wantlist.go, internal/backendsrv/store/wantlist_test.go, internal/backendsrv/store/alertlog_test.go
- **Verification:** `go build ./...` clean; full `store`, `migrations`, and `webadmin` test suites pass.
- **Committed in:** c49863a (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The caller updates were mechanically required for compilation and preserve existing account-level behavior. No scope creep; no behavior change to the existing endpoint.

## Issues Encountered
- A local `i64ptr` helper collided with an existing `i64ptr` in `itemids_test.go` (redeclared-in-block vet error). Removed the duplicate and reused the package-level helper. Resolved during Task 2 before the commit.

## Verification
- `go test ./internal/backendsrv/migrations/ -run TestMigrate_00010 -count=1` — PASS (goose migrated to version 10; column + NULL backfill + dual-path dedup behaviors proven).
- `go test ./internal/backendsrv/store/ -count=1` — PASS (all new + existing wantlist/assignment tests).
- `go test ./internal/backendsrv/webadmin/... -count=1` — PASS (no regression from the handler signature change).
- `go build ./...` clean; `go vet ./internal/backendsrv/store/... ./internal/backendsrv/migrations/...` clean.
- `git diff` confirms NO modification to migrations 00001–00009 (extend-only doctrine intact).

## Threat Model Coverage
- **T-28-01 (IDOR):** `IsCharAssignedToTx` delivered as the server-side authorization probe (Plan 02's handler must call it before insert).
- **T-28-02 (Info disclosure):** `ListGuildWants` excludes the private `note` (no `note` in SELECT or `GuildWantRow`).
- **T-28-03 (SQLi):** all queries parameterized (`?` placeholders); no string interpolation of `character_id`/`callerID`/names.
- **T-28-04 (dedup regression):** `COALESCE(character_id,-1)` sentinel preserves the 00006 account-level dedup; the migrate test asserts NULL-vs-NULL still collides.

## User Setup Required
None - backend-only data-layer change; the watcher is untouched (no WatcherMaxSchemaVersion change). The migration applies automatically via `goose.Up` on backend boot.

## Next Phase Readiness
- The CWANT-01/02 (persistence + backfill), CWANT-03/04 (guildwide read), and CWANT-06 (per-row char name) data layer is ready.
- **Plan 28-02** can now: call `IsCharAssignedToTx` (+ map `ErrCharNotAssigned` → 403) to authorize a body-supplied `character_id`, pass the tag through `AddWantTx`, and serve `ListGuildWants` on a new guildwide endpoint.
- No blockers.

## Self-Check: PASSED

- FOUND: `internal/backendsrv/migrations/00010_character_tagged_wantlist.sql`
- FOUND: `.planning/phases/28-character-tagged-wantlist/28-01-SUMMARY.md`
- FOUND commit `fb9a1f6` (Task 1), `c49863a` (Task 2), `00cf8c7` (Task 3)

---
*Phase: 28-character-tagged-wantlist*
*Completed: 2026-06-09*
