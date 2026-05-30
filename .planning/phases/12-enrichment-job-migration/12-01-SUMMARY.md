---
phase: 12-enrichment-job-migration
plan: 01
subsystem: database
tags: [go, sqlite, goose, modernc, migration, upsert, store, slog]

# Dependency graph
requires:
  - phase: 11-backend-foundation
    provides: "00001_init.sql (the 5 empty dimension tables + pigparse_price base shape), 00002_audit.sql, migrations/embed.go (//go:embed *.sql + sqlite3 dialect), store.Open single-writer DSN, store.NewTestDB fixture, store/replace.go (ReplaceInventoryTx/ReplaceSpellbookTx public/Tx split — the analog mirrored here)"
provides:
  - "00003_enrich_columns.sql: 8 ADD COLUMN on pigparse_price (t30,a30,t60,a60,t6m,a6m,ty,ay) + CREATE TABLE job_run + CREATE TABLE etag_cache (forward-only, idempotent)"
  - "store/enrich.go: UpsertPigparsePrices(Tx), UpsertItemMaster(Tx), GetItemMasterSHA1Tx, UpsertWikiSpellsForClass(Tx), ReplaceWikiGearTier(Tx), ReplaceQuestItemsForID(Tx) — the single tested SQL path for all 5 dimension tables"
  - "store/jobstate.go: GetJobRun/SetJobRun (scheduler durable cursor) + GetETag/SetETag (politeFetch ETag/304 state)"
  - "store-local input structs PigparsePrice, ItemMaster, WikiSpell, WikiGearTier, QuestItem (jobs hand these in; store never imports enrich)"
affects: [12-02, 12-03, 12-04, 12-05, enrich-jobs, politefetch, scheduler, 14-web-frontend]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-table conflict strategy (D-12): per-item upsert (pigparse_price, item_master), per-class DELETE+INSERT (wiki_spells), FULL-TABLE replace (wiki_gear_tier — UNIQUE-on-NULL is broken), per-item-id DELETE+INSERT (quest_items)"
    - "Single-tested-SQL-path (11-05 WARNING-3) extended to enrichment: all upsert/replace SQL lives in exported, _test.go-covered store methods; Wave-2 jobs author zero inline SQL and compose *Tx methods over one tx"
    - "Store.X (begins+commits own tx) / XTx (composes in caller's tx) symmetric split, mirroring replace.go"

key-files:
  created:
    - "internal/backendsrv/migrations/00003_enrich_columns.sql"
    - "internal/backendsrv/store/enrich.go"
    - "internal/backendsrv/store/enrich_test.go"
    - "internal/backendsrv/store/jobstate.go"
    - "internal/backendsrv/store/jobstate_test.go"
  modified:
    - "internal/backendsrv/migrations/migrate_test.go"

key-decisions:
  - "00003 omits t90/a90 (the parser emits them but the Sheet's buildRow never wrote them) for byte-parity with the live Sheet (D-7)"
  - "wiki_gear_tier uses full-table replace, not upsert: the declared UNIQUE(tier,class,slot,item_id) never fires because item_id is always NULL and SQLite treats NULLs as distinct — a per-row upsert would duplicate every row each weekly run (Pitfall 1)"
  - "GetJobRun maps absent-row OR NULL last_run_at to zero time + ok=false — the 'never run ⇒ due-on-startup-if-missed' signal the Wave-2 scheduler depends on"
  - "SetJobRun/SetETag are upserts (ON CONFLICT DO UPDATE), one row per job/url — advance-always (even on error) so a failing fetch retries on the next cadence window, not every tick"
  - "PigparsePrice carries current_avg=a30 and blue_volume=t30 as the Sheet's denormalized aliases (set in UpsertPigparsePricesTx); direction is the stringified WTS/WTB flag"

patterns-established:
  - "Full-table DELETE-all+INSERT replace for a table whose only UNIQUE is NULL-poisoned (wiki_gear_tier)"
  - "SHA-1 short-circuit getter (GetItemMasterSHA1Tx) returning \"\" for an absent row, so the wiki job can skip an unchanged item_master upsert"
  - "Store-local input structs keep the dependency one-directional (jobs → store), avoiding a store→enrich import cycle"

requirements-completed: [ENRICH-10, ENRICH-11]

# Metrics
duration: 9min
completed: 2026-05-29
---

# Phase 12 Plan 01: Enrichment Migration + Store Methods Summary

**Forward-only 00003 goose migration (8 pigparse_price price columns + job_run + etag_cache) plus the single tested, parameterized store layer that writes all 5 dimension tables and the scheduler/ETag cursors — the hard prerequisite for every Phase 12 job.**

## Performance

- **Duration:** ~9 min (first task commit 20:24 → last 20:33 local)
- **Started:** 2026-05-30T01:23Z (UTC)
- **Completed:** 2026-05-30T01:34Z (UTC)
- **Tasks:** 3 (all `type=auto`, `tdd=true`)
- **Files modified:** 6 (5 created, 1 extended)

## Accomplishments
- **00003_enrich_columns.sql** lands forward-only: 8 single-column `ALTER TABLE pigparse_price ADD COLUMN` (SQLite forbids multi-column adds) for the price-history columns the parser emits, plus `CREATE TABLE job_run` (scheduler last-run cursor) and `CREATE TABLE etag_cache` (politeFetch 304 state). `goose.Up` applies it cleanly over 00001+00002 and is a no-op on re-run; 00001/00002 are byte-unchanged.
- **store/enrich.go** — all 5 dimension-table write methods with the correct per-table conflict strategy (D-12): per-item upsert for `pigparse_price`/`item_master`, per-class DELETE+INSERT for `wiki_spells`, full-table replace for `wiki_gear_tier`, per-item-id DELETE+INSERT for `quest_items`. Plus `GetItemMasterSHA1Tx` for the wiki job's SHA-1 short-circuit. Symmetric `Store.X`/`XTx` split so Wave-2 jobs compose `*Tx` methods over one transaction.
- **store/jobstate.go** — `GetJobRun`/`SetJobRun` (durable cursor; never-run maps to a due zero-time) and `GetETag`/`SetETag` (ETag/Last-Modified cache; both upserts, not appends).
- **Single-tested-SQL-path holds:** all new enrichment SQL is confined to `store/enrich.go`, `store/jobstate.go`, and the migration — grep confirms zero enrichment SQL leaked into `scheduler/` or `ingest/`.
- Full `go build ./...` + `go vet ./...` clean; `go test ./internal/backendsrv/...` green; whole-repo `go test ./...` = 24 packages, 0 failures (no v1 watcher regression).

## Task Commits

Each task was committed atomically (TDD: the migration/methods + their assertion tests landed together per task):

1. **Task 1: 00003 migration + migration assertion test** - `5cd347b` (feat)
2. **Task 2: store/enrich.go — 5 dimension write methods** - `f96e96d` (feat)
3. **Task 3: store/jobstate.go — job_run + etag_cache cursors** - `97409ba` (feat)

**Plan metadata:** committed separately with this SUMMARY + STATE.md + ROADMAP.md.

## Files Created/Modified
- `internal/backendsrv/migrations/00003_enrich_columns.sql` - 8 ADD COLUMN on pigparse_price + job_run + etag_cache (forward-only; auto-included by the existing `//go:embed *.sql` — embed.go untouched).
- `internal/backendsrv/migrations/migrate_test.go` - added `TestMigrate_00003_AddsEnrichColumnsAndTables` (PRAGMA table_info asserts the 8 columns; sqlite_master asserts both tables; re-run is a no-op).
- `internal/backendsrv/store/enrich.go` - the 5 dimension write methods + structs + the SHA-1 getter; parameterized `?` only; slog logs counts/ids/err only.
- `internal/backendsrv/store/enrich_test.go` - ON CONFLICT updates-in-place + partial-reupsert-leaves-rest (pigparse); SHA-1 getter present/absent (item_master); per-class drop + other class untouched (wiki_spells); 2nd identical replace == N not 2N (wiki_gear_tier); per-id scoped replace (quest_items).
- `internal/backendsrv/store/jobstate.go` - GetJobRun/SetJobRun + GetETag/SetETag (plain `(*Store)` methods; upserts; never-run ⇒ zero time).
- `internal/backendsrv/store/jobstate_test.go` - empty-table not-due; SetJobRun RFC3339 round-trip via `.Equal` + upsert count==1; GetETag empty blank; SetETag round-trip + upsert count==1.

## Decisions Made
- Followed the plan's verbatim 00003 SQL exactly, including the `DROP COLUMN`-based `-- +goose Down` block (Down is courtesy-only; this project deploys forward via `goose.Up`).
- Kept the per-table strategy precisely as locked in D-12; the wiki_gear_tier full-replace is proven non-duplicating by the N-not-2N row-count test.
- Stored `last_refreshed` as a job-supplied string (the job owns the clock), so `enrich.go` needs no `time` import — mirrors how `replace.go` takes `uploadedAt` from the caller.

## Deviations from Plan

### Minor deviations (no rule-triggered fixes; behavior matches the plan's intent)

**1. [Doc-vs-grep] `ON CONFLICT(...)` literal appears twice per clause (doc comment + SQL), not once**
- **Found during:** Task 3 (jobstate.go) acceptance-criteria check.
- **Issue:** The plan's literal acceptance criterion `grep -c "ON CONFLICT(job_name) DO UPDATE"` (and the `url` variant) expects exactly `1`. My implementation documents the upsert clause in a doc comment AND uses it in the SQL, so the phrase occurs twice per clause.
- **Resolution:** Kept the accurate doc comments — they match how the rest of this package documents its SQL strategy (binding.go/replace.go cite their constraints in comments). The *substantive* intent (exactly one `ON CONFLICT … DO UPDATE` SQL statement per table ⇒ one row per job/url, not append) is satisfied and is directly asserted by the `count==1` checks in `TestJobRun_SetThenGetRoundTripsAndUpserts` and `TestETag_SetThenGetRoundTripsAndUpserts`.
- **Files:** internal/backendsrv/store/jobstate.go (no functional change vs plan).

**2. [Down block uses `DROP COLUMN`, not `ADD COLUMN`] — grep count 8, not 16**
- **Found during:** Task 1 acceptance-criteria check.
- **Issue:** One acceptance line said `grep -c "ALTER TABLE pigparse_price ADD COLUMN"` should return 16 (8 Up + 8 "DROP-paired" in Down). The verbatim SQL block the plan body specified (which is authoritative) uses `DROP COLUMN` in the Down section, so `ADD COLUMN` appears exactly 8 times.
- **Resolution:** Followed the plan's verbatim SQL exactly (8 `ADD COLUMN` under `-- +goose Up`, 8 `DROP COLUMN` under `-- +goose Down`). The load-bearing check ("confirm 8 ADD COLUMN lines exist under `-- +goose Up`") is satisfied. No file change needed.

---

**Total deviations:** 0 rule-triggered auto-fixes; 2 minor doc-vs-literal-grep notes where the plan's prose grep counts didn't match the plan's own verbatim SQL/comment style. No code behavior diverges from the plan's intent.
**Impact on plan:** None. All success criteria and the substantive acceptance checks pass.

## Issues Encountered
- gofmt reformatted `enrich.go` and `enrich_test.go` (struct-field + var-block alignment) after the initial write; re-ran the targeted tests post-format to confirm no behavior change (all green). I also dropped an initially-unused `time` import from `enrich.go` (cleaner than a blank `var _ = time.RFC3339`).

## User Setup Required
None - no external service configuration required. (The PigParse + wiki APIs are public and unauthenticated; no API key, no secret in this path. Live API calls happen only when the Wave-2 scheduled jobs run on the VPS.)

## Next Phase Readiness
- **Wave 2 (jobs + politefetch + scheduler flesh-out) is unblocked.** The store layer they compose is in place and tested:
  - The 4 pure parser ports (`enrich/`) write through `Upsert*`/`Replace*` methods; the PigParse job applies the D-9 t=0 WTS filter then calls `UpsertPigparsePricesTx`; the wiki job uses `GetItemMasterSHA1Tx` for its per-item short-circuit.
  - `politefetch` reads `GetETag` before a fetch and writes `SetETag` after a 200+parse (untouched on 304).
  - The scheduler reads each job's `GetJobRun` for the due-check and writes `SetJobRun` after each run (advance-always).
- No blockers. 00003 is embedded in the binary via `//go:embed *.sql`, so the Wave-2 deploy is "drop the new binary + restart" (goose.Up applies 00003 on startup, D-10).

## Self-Check: PASSED

- All 5 created files + the SUMMARY exist on disk (verified).
- All 3 task commit hashes (`5cd347b`, `f96e96d`, `97409ba`) exist in git history (verified).
- `go build ./...`, `go vet ./...` clean; `go test ./...` = 24 packages, 0 failures.

---
*Phase: 12-enrichment-job-migration*
*Completed: 2026-05-29*
