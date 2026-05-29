---
phase: 11-backend-foundation-ingest-api
plan: 02
subsystem: database
tags: [sqlite, modernc, goose, migrations, go, schema, wal, pragmas, test-helper]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api (Plan 01)
    provides: "D-01 verdict HAND-ROLLED Go (net/http + modernc.org/sqlite + goose); confirms modernc v1.51.0 (no cgo) is the driver"
provides:
  - "Forward-only goose migration 00001_init.sql creating all D-13 tables (owner/character/inventory_item/spellbook_entry/guild_code + 5 empty dimension tables)"
  - "migrations.RunMigrations(db) — //go:embed goose-on-startup runner, idempotent (BACKEND-02)"
  - "store.Open(dbPath) — modernc DB handle with WAL/busy_timeout/foreign_keys/synchronous/_txlock=immediate pragmas + SetMaxOpenConns(1) (single-writer)"
  - "store.DSN(dbPath) — exported connection-string builder"
  - "store.NewTestDB(t) — shared temp-DB fixture (Open + goose.Up) reused by 11-03/04/05 store/ingest/auth tests"
affects: [11-03, 11-04, 11-05, 12-enrichment-job-migration, 14-web-frontend]

# Tech tracking
tech-stack:
  added:
    - "modernc.org/sqlite@v1.51.0 (pure-Go SQLite driver, no cgo — promoted indirect->direct)"
    - "github.com/pressly/goose/v3@v3.27.1 (forward-only embedded migrations — promoted indirect->direct)"
  patterns:
    - "modernc DSN with per-connection pragmas (foreign_keys is NOT persistent — must be in the DSN so every pooled conn enforces FK actions)"
    - "Single-writer SQLite server via SetMaxOpenConns(1) + _txlock=immediate (mirrors the watcher's batchMu intent; eliminates SQLITE_BUSY)"
    - "goose-on-startup via //go:embed *.sql + SetDialect(sqlite3) (dialect string deliberately distinct from the modernc driver name sqlite)"
    - "Shared cross-package test fixture (NewTestDB) in a non-_test.go file, importable by other packages' tests (httptest pattern)"

key-files:
  created:
    - "internal/backendsrv/migrations/00001_init.sql"
    - "internal/backendsrv/migrations/embed.go"
    - "internal/backendsrv/migrations/migrate_test.go"
    - "internal/backendsrv/store/db.go"
    - "internal/backendsrv/store/testhelper.go"
    - "internal/backendsrv/store/db_test.go"
  modified:
    - "go.mod / go.sum (modernc.org/sqlite + goose/v3 promoted to direct deps)"

key-decisions:
  - "Co-located *.sql next to embed.go and used goose.Up(db, \".\") (one of the plan's two sanctioned embed layouts)"
  - "DSN path forward-slashed via filepath.ToSlash so the file: URI is valid on both the Windows dev/test box and the Linux deploy host (verified all three URI forms work on Windows; chose the robust one)"
  - "migrate_test.go is an EXTERNAL test package (migrations_test) to avoid the store->migrations import cycle while still exercising the shared store.NewTestDB fixture"
  - "Left goose's default stdout migration logger in place for tests (silencing it is a 11-05 server-startup concern; suppressing now could hide migration errors)"

patterns-established:
  - "Pattern: modernc DSN pragma block (RESEARCH Pattern 5) is the canonical backend DB-open — reuse store.Open everywhere, never hand-build the DSN"
  - "Pattern: every backend test spins a migrated DB via store.NewTestDB(t) (RESEARCH Wave 0 Gaps shared fixture)"

requirements-completed: [BACKEND-02]

# Metrics
duration: 11 min
completed: 2026-05-29
---

# Phase 11 Plan 02: SQLite Schema + goose Migrations + modernc DB Handle Summary

**Forward-only goose `00001_init.sql` creating the full D-13 schema (owner/character split + inventory/spellbook/guild_code + five empty dimension tables), a `//go:embed` `goose.Up`-on-startup runner proven idempotent, a `modernc.org/sqlite` DB handle opened with WAL/busy_timeout/foreign_keys/_txlock=immediate pragmas + `SetMaxOpenConns(1)`, and a shared `NewTestDB` temp-DB fixture — all unit-tested, pure-Go (no cgo), building for the `linux/amd64` deploy target.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-05-29T21:21:59Z
- **Completed:** 2026-05-29T21:33:12Z
- **Tasks:** 2 (Task 2 was TDD: RED -> GREEN)
- **Files created:** 6 (+ go.mod/go.sum modified)

## Accomplishments

- **D-13 schema, forward-only:** `00001_init.sql` creates all ten tables — `owner`, `character` (separate, with `owner_id` FK + `name UNIQUE COLLATE NOCASE`), `inventory_item` and `spellbook_entry` (both `ON DELETE CASCADE`), `guild_code`, and the five EMPTY dimension tables (`item_master`, `pigparse_price`, `wiki_spells`, `wiki_gear_tier`, `quest_items`) — copied verbatim from RESEARCH §"Migration SQL Sketch".
- **Idempotent goose-on-startup (BACKEND-02):** `migrations.RunMigrations` embeds `*.sql` via `//go:embed`, sets the `sqlite3` dialect (deliberately NOT the `sqlite` driver name — RESEARCH Pitfall 3), and runs `goose.Up`. A second run is a no-op (`goose_db_version` unchanged), proven by `TestRunMigrations_Idempotent`.
- **modernc single-writer DB handle:** `store.Open` opens the pure-Go `"sqlite"` driver with the Pattern 5 DSN (WAL + busy_timeout(5000) + foreign_keys(ON) + synchronous(NORMAL) + _txlock=immediate) and `SetMaxOpenConns(1)`. `TestOpen_ForeignKeysEnabled` proves FK enforcement is ON on a fresh connection (T-11.02-01 mitigation — the pragma is per-connection, not persistent).
- **Shared test fixture:** `store.NewTestDB(t)` opens a temp-file DB via `Open`, runs `goose.Up`, and registers `t.Cleanup` — the reusable fixture 11-03/04/05 import. Lives in a non-`_test.go` file so it is importable across packages (httptest pattern).
- **Pure-Go, no cgo:** `CGO_ENABLED=0 go build` succeeds on both the Windows host and the `linux/amd64` deploy target — the binary stays single-static + cross-compilable from Windows (CLAUDE.md no-cgo ethos).

## Task Commits

Each task was committed atomically:

1. **Task 1: goose migration `00001_init.sql` + embed runner** — `c262816` (feat)
2. **Task 2 (RED): failing DB-open + migration tests** — `3117a1a` (test)
3. **Task 2 (GREEN): modernc DB open + `NewTestDB` helper** — `165582a` (feat)

_Task 2 followed RED -> GREEN; no REFACTOR commit was needed (implementation was minimal and clean). Plan metadata committed separately._

## Files Created/Modified

- `internal/backendsrv/migrations/00001_init.sql` — forward-only goose migration; full D-13 DDL with `-- +goose Up`/`-- +goose Down`.
- `internal/backendsrv/migrations/embed.go` — `//go:embed *.sql` + `goose.SetDialect("sqlite3")` + `goose.Up(db, ".")`; idempotent `RunMigrations`.
- `internal/backendsrv/migrations/migrate_test.go` — external `migrations_test` package; CreatesAllTables / Idempotent / DimensionTables_Empty.
- `internal/backendsrv/store/db.go` — `Open` (modernc `"sqlite"` driver, Pattern 5 DSN, `SetMaxOpenConns(1)`) + exported `DSN`.
- `internal/backendsrv/store/testhelper.go` — `NewTestDB(t)` shared temp-DB fixture.
- `internal/backendsrv/store/db_test.go` — `TestOpen_ForeignKeysEnabled`, `TestDSN_ContainsPragmas`.
- `go.mod` / `go.sum` — `modernc.org/sqlite` + `pressly/goose/v3` promoted indirect -> direct (now directly imported).

## Decisions Made

- **Embed layout:** co-located `*.sql` next to `embed.go` and used `goose.Up(db, ".")` (the plan explicitly sanctioned this over the `migrations/*.sql` + `goose.Up(db, "migrations")` form; either is valid as long as the embed path and Up-dir argument agree).
- **DSN path encoding:** forward-slashed the path with `filepath.ToSlash` for a valid `file:` URI on both Windows (test box) and Linux (deploy). Empirically probed three URI forms on Windows — raw-backslash, forward-slashed, and forward-slashed-with-leading-slash — all opened and reported `foreign_keys=1`; chose the forward-slashed form as the most robust against drive-letter/space edge cases while keeping the pragma list verbatim.
- **Import-cycle avoidance:** `store` (production) imports `migrations` for `RunMigrations`, so the migration test uses an EXTERNAL test package (`migrations_test`) — which may depend on `store` (which depends on `migrations`) without forming a cycle. This let the migration tests exercise the real shared `store.NewTestDB` fixture rather than a duplicated open path.
- **goose logger:** left goose's default stdout migration logging in place. It is harmless in tests and surfaces migration steps; silencing it cleanly belongs to 11-05 server startup, and suppressing it now risks masking a real migration error.

## Deviations from Plan

None - plan executed exactly as written.

The plan offered two sanctioned embed layouts and two phrasings of the migration-test source (`NewTestDB` *or* open+RunMigrations); the choices made (co-located `*.sql`, `NewTestDB` via an external test package) are within the plan's stated latitude, not deviations. No bugs, missing-critical, or blocking issues were encountered (Rules 1-3 not triggered); no architectural questions arose (Rule 4 not triggered).

**Note on `go.mod` (not a deviation):** `go mod tidy` reclassified `github.com/pocketbase/pocketbase` + `pocketbase/dbx` from indirect to direct. That is correct tidy behavior driven by the PRE-EXISTING `spike/pocketbase/` tree (from 11-01), not by any code this plan added. The PocketBase dependency and the spike tree are slated for removal in 11-05 per the D-01 verdict; this plan leaves that disposition untouched.

**Total deviations:** 0.
**Impact on plan:** None — clean execution; BACKEND-02 satisfied at the build/test tier.

## Issues Encountered

None. The one point of uncertainty — whether a Windows temp-dir path (with a space, e.g. `Virus Canary`) would parse correctly in a modernc `file:` URI — was resolved up front with a throwaway probe (all three URI forms worked, `foreign_keys=1`), then locked to the forward-slashed form. The throwaway probe was removed before committing.

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are covered and test-proven:

- **T-11.02-01 (FK actions silently ignored):** `foreign_keys(ON)` is in the DSN (per-connection, not persistent); `TestOpen_ForeignKeysEnabled` proves FK=1 on a fresh connection.
- **T-11.02-02 (SQLITE_BUSY under concurrency):** `busy_timeout(5000)` + `_txlock=immediate` + `SetMaxOpenConns(1)` all present in `Open`.
- **T-11.02-03 (half-applied migration):** goose tracks applied versions in `goose_db_version`; `TestRunMigrations_Idempotent` proves replay safety.
- **T-11.02-04 (DSN logging — accept):** no DSN/connection-string logging added; documented habit honored.

No new security-relevant surface introduced (no network endpoints, no auth paths, no untrusted input crosses any boundary in this plan — ingest is 11-05).

## Known Stubs

None. The five dimension tables are intentionally created EMPTY per D-13 (P11 stands up the schema; P12 populates the data) — this is the locked design, not an unfinished stub. `TestDimensionTables_Empty` asserts the empty state on purpose.

## User Setup Required

None - no external service configuration required. All tests run in CI on the Windows dev box (pure-Go modernc, no cgo, no live box).

## Next Phase Readiness

- **BACKEND-02 done at the build/test tier.** The migrated DB handle (`store.Open`) and the shared fixture (`store.NewTestDB`) are exported and ready for the Wave-3 parallel plans.
- **11-03 (store tx)** can build the atomic full-snapshot replace `*sql.Tx` on top of `store.Open` + `NewTestDB`.
- **11-04 (auth store)** can build the `guild_code` hash storage/lookup against the migrated schema using `NewTestDB`.
- **11-05 (ingest + HTTP/cron wiring)** consumes the DB handle, wires `net/http` ServeMux + a ticker scheduler (per the 11-01 FALLBACK verdict), and carries the two cleanup chores (remove the `pocketbase` dep + delete `spike/pocketbase/`, then `go mod tidy`).
- **No blockers.**

## Self-Check: PASSED

- Files on disk: `00001_init.sql` FOUND, `migrations/embed.go` FOUND, `migrations/migrate_test.go` FOUND, `store/db.go` FOUND, `store/testhelper.go` FOUND, `store/db_test.go` FOUND, `11-02-SUMMARY.md` FOUND.
- Commits exist: `c262816` (Task 1 feat) FOUND, `3117a1a` (Task 2 RED test) FOUND, `165582a` (Task 2 GREEN feat) FOUND.
- Plan `<verification>`: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./internal/backendsrv/store/... ./internal/backendsrv/migrations/...` exit 0 (6/6 tests pass). `goose.SetDialect("sqlite3")` (dialect) and `sql.Open("sqlite", …)` (driver) both present and distinct (Pitfall 3). `goose.Up` idempotent (second run no-ops; `goose_db_version` unchanged). `PRAGMA foreign_keys` = 1 on a fresh connection. `NewTestDB` exported. `CGO_ENABLED=0` builds on host + `linux/amd64`.

## TDD Gate Compliance

Task 2 followed the RED -> GREEN gate sequence in git history: `test(11-02)` RED commit `3117a1a` (failing, fails to compile — `store.Open`/`DSN`/`NewTestDB` undefined) precedes the `feat(11-02)` GREEN commit `165582a` (all 6 tests pass). No REFACTOR commit (implementation was minimal/clean). Task 1's deliverable (SQL + embed) is static-verified (`go vet` + grep acceptance checks) per the plan's design — its runnable behavior is proven by Task 2's migration tests.

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
