---
phase: 21-ec-tunnel-auction-monitor
plan: 01
subsystem: api
tags: [pigparse, go, sqlite, goose, json-parser, cursor, ec-tunnel, discord-monitor]

# Dependency graph
requires:
  - phase: 20-bot-dm-notification-infrastructure
    provides: "the notify/wantmatch/alert_log spine (cooldownEC=22h, both-gates), monitor_flag.ec_auction, the wantlist_item table"
  - phase: 19-wantlist-crud
    provides: "wantlist_item(item_id, item_name, reason, active) — the EC poll-set source"
provides:
  - "21-SPIKE.md: the feasibility verdict (path=getdetails) + the CRITICAL server=0 finding + the NAME-key-form decision + the D-07 coverage caveat"
  - "enrich.ParseItemDetail + ItemAuctionDetail{U,I,P,T} / ItemDetail{Items,ItemName,Players} — the per-auction getdetails parser (t/u collision-aware, import-pure)"
  - "migration 00008_ec_cursor.sql — the ec_auction_cursor diff-cursor table"
  - "store.GetECCursor (first-sight absent) / SetECCursor (advance-only upsert) / ECPollSet (DISTINCT active catalog wants)"
  - "a real anonymized getdetails fixture (__fixtures__/pigparse-getdetails-fungi.json)"
affects: [21-02, 21-03, ec_auction_match, wts-monitor, raid-monitor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-item poll-and-diff cursor on an RFC3339 timestamp via lexical string compare (last_seen_t TEXT)"
    - "First-sight-absent => baseline (no replay): GetECCursor returns ok=false on sql.ErrNoRows"
    - "DISTINCT active-catalog poll-set read (owner-agnostic, reason NOT filtered, NULL item_id skipped)"

key-files:
  created:
    - "internal/backendsrv/enrich/pigdetails.go"
    - "internal/backendsrv/enrich/pigdetails_test.go"
    - "internal/backendsrv/enrich/__fixtures__/pigparse-getdetails-fungi.json"
    - "internal/backendsrv/migrations/00008_ec_cursor.sql"
    - "internal/backendsrv/store/eccursor.go"
    - "internal/backendsrv/store/eccursor_test.go"
    - ".planning/phases/21-ec-tunnel-auction-monitor/21-SPIKE.md"
  modified:
    - "internal/backendsrv/migrations/migrate_test.go"

key-decisions:
  - "PATH CHOSEN = per-auction getdetails (D-08 threshold met live); lastWTSSeen fallback NOT adopted"
  - "getdetails KEY FORM = NAME, not id (the bare id form 400s; id-in-name-slot returns empty)"
  - "CRITICAL: the LIVE Blue getdetails feed is server=0, NOT server=1 (research said 1; 1 is stale ~11h) — Plan 03 MUST poll getdetails/0/{name}"
  - "p is *int (nullable, JSON-null-distinct from 0); seller best-effort via players map (aggregate hit-rate only, no PII committed — V7)"
  - "ec_auction_cursor is backend-only: no _meta.schema_version bump, no WatcherMaxSchemaVersion change"

patterns-established:
  - "ParseItemDetail mirrors ParseToRows' malformation discipline (<=1% skip+slog.Warn, >1% error) but guards a top-level OBJECT, not an array; calls out the t/u collision in the doc"
  - "ec_auction_cursor advance-only-on-success upsert (ON CONFLICT(item_id)) — the jobstate.go cursor analog applied per-item"

requirements-completed: [WANT-05]

# Metrics
duration: ~50min
completed: 2026-06-06
---

# Phase 21 Plan 01: EC Monitor Data Foundation + Feasibility Spike Summary

**Live PigParse spike picked the per-auction `getdetails` path (correcting the research's server number AND key form), plus the NEW t/u-collision-aware getdetails parser, the `ec_auction_cursor` diff-cursor migration, and the first-sight/upsert/poll-set store layer the producer job needs.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-06-06T02:11:00Z
- **Completed:** 2026-06-06T02:20:00Z (approx)
- **Tasks:** 3
- **Files modified:** 8 (7 created, 1 modified)

## Accomplishments
- **The mandatory feasibility spike ran live and decided the approach with NO checkpoint (D-08):** per-auction `getdetails` is viable — 25,727 WTS records for Fungi Tunic, a live feed whose newest auction trailed wall-clock by only ~3 minutes.
- **Two research corrections surfaced by the live probe** (both critical for Plan 03): the live Blue feed is `server=0` (server=1 is ~11h stale), and the only working query key is the item NAME (the id form 400s / returns empty).
- **A NEW import-pure `getdetails` parser** that explicitly handles the `t`/`u` collision (here `t`=timestamp, `u`=direction — the opposite of the getall feed), tolerates malformed/null records, and keeps nullable price as `*int` (never 0pp).
- **The `ec_auction_cursor` table (migration 00008, extend-only, idempotent)** plus the first-sight-aware `GetECCursor`, advance-only `SetECCursor`, and the DISTINCT active-catalog `ECPollSet` query — everything the Plan 03 producer reads from the data side.

## Task Commits

Each task was committed atomically:

1. **Task 1: PigParse feasibility spike (GATE)** - `0f29fcc` (feat)
2. **Task 2: NEW getdetails parser** - `ff01c77` (feat; test+impl in one commit — RED verified before GREEN)
3. **Task 3: migration 00008 + cursor/poll-set store** - `486cff2` (feat)

**Plan metadata:** (this SUMMARY + STATE/ROADMAP) committed separately.

_Note: Task 2/3 are TDD tasks — the failing test was written and run (RED: `undefined: ParseItemDetail`) before the implementation made it green; committed together per the single-feature grain._

## Files Created/Modified
- `.planning/phases/21-ec-tunnel-auction-monitor/21-SPIKE.md` - spike verdict, server=0 + NAME-key findings, coverage caveat, Plan-03 hand-off
- `internal/backendsrv/enrich/pigdetails.go` - `ParseItemDetail`, `ItemAuctionDetail{U,I,P,T}`, `ItemDetail{Items,ItemName,Players}`
- `internal/backendsrv/enrich/pigdetails_test.go` - real-fixture happy path + null-items, nullable-price, t-collision regression, malformed-threshold, truncated-body cases
- `internal/backendsrv/enrich/__fixtures__/pigparse-getdetails-fungi.json` - 12 real anonymized server=0 records
- `internal/backendsrv/migrations/00008_ec_cursor.sql` - `ec_auction_cursor(item_id PK, last_seen_t TEXT, updated_at INTEGER)`
- `internal/backendsrv/migrations/migrate_test.go` - extended with `TestMigrate_00008_AddsECCursor` (apply + columns + PK-upsert + idempotent)
- `internal/backendsrv/store/eccursor.go` - `GetECCursor` / `SetECCursor` / `ECPollItem` / `ECPollSet`
- `internal/backendsrv/store/eccursor_test.go` - first-sight absent, round-trip/upsert, full poll-set matrix

## Decisions Made
- **Path = per-auction `getdetails`** (D-08 threshold met: presence + freshness). Documented in 21-SPIKE.md; `lastWTSSeen` fallback not needed.
- **Key form = NAME** (`getdetails/{server}/{itemname}`) — the spike disproved RESEARCH Open Q2's id-form recommendation (id form 400s).
- **Server segment = 0** for the live feed — corrects the research/STACK-v2.2 `server=1=Blue` assumption, which is stale for `getdetails`.
- **Parser placed in `enrich` (import-pure), fixtures in `__fixtures__/`** per the plan's explicit file list and CLAUDE.md's fixture-directory convention (note: existing enrich parsers load from `testdata/`; the plan dictated `__fixtures__/` and the test honors that).

## Deviations from Plan

None — plan executed exactly as written. The two "research corrections" (server=0, NAME key form) are not plan deviations: Task 1 explicitly tasked the spike with pinning the name-vs-id key and recording coverage, and D-07/D-08 anticipated the spike overriding research assumptions. They are recorded as spike findings, which is the task's designed output.

## Issues Encountered
- **JSON `null` price coerced to 0:** `json.Unmarshal` of the literal `null` into a `float64` succeeds with a zero value, so the first GREEN attempt turned a null `p` into `&0`. Fixed by guarding `string(rawP) != "null"` BEFORE the numeric coerce (caught by `TestParseItemDetail_NullablePrice`). This is exactly the threat T-21-04 (null→0pp) the parser must prevent — caught and fixed within Task 2, no separate deviation.
- **`if` used as a variable name** (Go keyword) in the first parser draft — a compile error, renamed to `idf`. Caught immediately on first `go test`.

## User Setup Required
None - no external service configuration required. (The spike made only read-only HTTPS GETs to the public PigParse API.)

## Next Phase Readiness
- **Plan 03 (the producer job) is unblocked and well-specified:** the spike hands it the exact endpoint (`getdetails/0/{itemname}`), the NAME key, the diff key (`t` via lexical compare), the first-sight baseline rule, and the nullable-price/best-effort-seller framing.
- **Data side is complete:** parser, cursor table, and cursor/poll-set store all green (`go test ./internal/backendsrv/enrich/ ./internal/backendsrv/migrations/ ./internal/backendsrv/store/` passes; `go build ./...` + `go vet ./internal/backendsrv/...` clean).
- **Watch-out for Plan 03:** do NOT poll `server=1` (stale); the `i` field inside records is an auction-instance id, NOT the EQ item id and NOT a query key — key the poll on `item_name`, join on `item_id` after the hit.

## Self-Check: PASSED

- `internal/backendsrv/enrich/pigdetails.go` — FOUND
- `internal/backendsrv/enrich/__fixtures__/pigparse-getdetails-fungi.json` — FOUND
- `internal/backendsrv/migrations/00008_ec_cursor.sql` — FOUND
- `internal/backendsrv/store/eccursor.go` — FOUND
- `.planning/phases/21-ec-tunnel-auction-monitor/21-SPIKE.md` — FOUND
- commits `0f29fcc`, `ff01c77`, `486cff2` — FOUND in git log

---
*Phase: 21-ec-tunnel-auction-monitor*
*Completed: 2026-06-06*
