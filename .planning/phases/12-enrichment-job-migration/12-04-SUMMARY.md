---
phase: 12-enrichment-job-migration
plan: 04
subsystem: api
tags: [go, enrichment, pigparse, mediawiki, jobs, etag, sha1, single-sql-path, slog]

# Dependency graph
requires:
  - phase: 12-01
    provides: "store.UpsertPigparsePricesTx / UpsertItemMasterTx / GetItemMasterSHA1Tx / UpsertWikiSpellsForClassTx / ReplaceWikiGearTierTx / ReplaceQuestItemsForIDTx + Get/SetJobRun + Get/SetETag (the *Tx methods these jobs compose over one tx) + the store-local input structs"
  - phase: 12-02
    provides: "enrich.ParseToRows/PigparseRow, ParseItempage/ParsedWikiItem/WikiQuestItemLink, ParseClassPage/WikiSpellRow, ParseGearTierPage/WikiGearTierRow/Tier, CLASSES + CLASS_DISPLAY_TO_ABBREV (the pure parsers the jobs feed)"
  - phase: 12-03
    provides: "politefetch.Fetch/Fetcher/FetchResult/Options (the polite net/http client + ETag/304 seam the jobs inject)"
provides:
  - "internal/backendsrv/enrich/jobs package: the two enrichment jobs that compose Wave-1 (politefetch -> parser -> store *Tx over one tx), authoring ZERO inline SQL"
  - "jobs/pigparse.go RunPigparse(ctx, db, fetch): daily price job — GetETag -> Fetch (304-aware) -> ParseToRows -> D-9 WTS(t=0) filter -> D-4 truncation-guard-as-LOG -> UpsertPigparsePricesTx -> SetETag + SetJobRun"
  - "jobs/wiki.go RunWiki(ctx, db, fetch): weekly items+spells+gear single uninterrupted run (NO cursor) — 1s ctx-aware inter-request sleep, per-item SHA-1 short-circuit, per-resource 304 skip, gear full-replace of the combined set, log-but-continue"
  - "jobs/urls.go: hardcoded PigparseURL + WikiAPIBase constants (SSRF-safe) + wikiParseURL builder"
  - "store.DistinctInventoryItemIDs + store.ItemRef: the dedup-by-id (item_id,name) inventory union the wiki items pass iterates"
  - "store.GetJobRunDetail: read last_detail off the single SQL path (the truncation-guard baseline)"
affects: [12-05 scheduler, 14-web-frontend]

# Tech tracking
tech-stack:
  added: []   # stdlib only (context, database/sql, encoding/json, log/slog, net/url, strconv, strings, time)
  patterns:
    - "Job = compose-over-one-tx: each job calls politefetch.Fetch -> enrich.Parse* -> store.*Tx, mirroring ingest/handler.go::bindAndReplace; ZERO inline SQL (single-tested-SQL-path, 11-05 WARNING-3)"
    - "D-9 WTS(t=0) dedup as ONE isolated in-place filter in the job (not the parser), so flipping to last-wins / (id,direction) later is a one-line change"
    - "D-4 truncation guard = slog.Warn then PROCEED (never abort); the per-item upsert leaves unmentioned rows intact so a partial response never clobbers good data"
    - "Pitfall-6 304 handling: a FromCache result skips parse+write entirely (the empty body never reaches a parser/replace)"
    - "Pitfall-1 gear full-replace fires ONLY with the complete combined set from both pages; a failed/304 page skips the replace rather than wiping with a partial set"
    - "ctx-aware 1s inter-request sleep seam (wikiSleepFn, mirrors update/check.go's checkSleepFn) — unwinds on SIGTERM, no-op in tests; the 1s sleep is the job's, not the polite client's"
    - "fake Fetcher + httptest fixture server (page= -> testdata fixture) over store.NewTestDB drives the jobs deterministically with the real parser path"

key-files:
  created:
    - "internal/backendsrv/enrich/jobs/urls.go"
    - "internal/backendsrv/enrich/jobs/pigparse.go"
    - "internal/backendsrv/enrich/jobs/pigparse_test.go"
    - "internal/backendsrv/enrich/jobs/wiki.go"
    - "internal/backendsrv/enrich/jobs/wiki_test.go"
    - "internal/backendsrv/enrich/jobs/helpers_test.go"
    - "internal/backendsrv/store/itemids.go"
    - "internal/backendsrv/store/itemids_test.go"
  modified:
    - "internal/backendsrv/store/jobstate.go (added GetJobRunDetail read method)"

key-decisions:
  - "DistinctInventoryItemIDs dedups by item_id ALONE (GROUP BY item_id, MIN(name)), matching the Sheet's collectInventoryItemRefs (first-name-wins per id) — a per-(id,name) DISTINCT would refetch the same wiki page twice (politeness regression)"
  - "Truncation-guard baseline is read from job_run.last_detail ('rows=N') via a new store.GetJobRunDetail (kept on the single SQL path, not an inline job query)"
  - "Gear-tier: a 304 on either page is treated like an unavailable page — the full-table replace is SKIPPED (we lack that page's current rows, so replacing with a partial set would wipe the other page's rows). Only a fresh 200 on BOTH pages triggers the combined replace"
  - "classAbbrevToDisplay inverts enrich.CLASS_DISPLAY_TO_ABBREV at init for the spell-page titles (eqconst.go carries no abbrev->display map), mirroring refreshWikiSpells.ts's CLASS_ABBREV_TO_DISPLAY"
  - "wikiParseURL uses redirects=1 + url.QueryEscape of the underscored title; the URL doubles as the etag_cache key (one cache row per page)"

patterns-established:
  - "Per-unit transaction granularity: items = one tx per item (SHA-1 read + item_master + quest_items together), spells = one tx per class, gear = one tx for the whole combined replace"
  - "fetchWikiPage helper returns a 3-way outcome (got-page / unchanged-304 / skip) so each sub-pass handles 304 and failures uniformly without aborting the run"
  - "store.GetJobRunDetail: NullString read returning '' for absent/NULL, so a never-run / error / skip baseline simply disables the truncation guard rather than misfiring"

requirements-completed: [ENRICH-10, ENRICH-11]

# Metrics
duration: 17min
completed: 2026-05-30
---

# Phase 12 Plan 04: Enrichment Jobs (Daily PigParse + Weekly Wiki) Summary

**The two enrichment jobs that compose the Wave-1 polite client + pure parsers + store methods over one transaction — the daily PigParse price pull (D-9 WTS-only filter, D-4 truncation-guard-as-LOG, 304-skip) and the weekly P1999 wiki single-uninterrupted-run (no cursor; 1s inter-request sleep; per-item SHA-1 short-circuit; per-resource 304-skip; gear full-replace; log-but-continue) — both authoring ZERO inline SQL.**

## Performance

- **Duration:** ~17 min (Task 1 commit 21:24 -> Task 2 commit 21:38 local)
- **Started:** 2026-05-30T02:21:40Z (UTC)
- **Completed:** 2026-05-30T02:39:05Z (UTC)
- **Tasks:** 2 (both `type=auto`, `tdd=true`)
- **Files modified:** 9 (8 created, 1 extended)

## Accomplishments
- **`jobs/pigparse.go` RunPigparse** — the daily price job: reads the cached ETag, fetches getall/1, **skips parse+write on a 304** (Pitfall 6, never wipes good rows with an empty body), parses, applies the **D-9 WTS (t=0) filter** as one isolated step (4,333 of the fixture's 7,240 rows — one per distinct sell-side item_id), runs the **D-4 truncation guard as a LOG then PROCEEDS** (the per-item upsert leaves unmentioned rows intact), upserts over one tx, and advances the ETag + job_run cursors. Proven: item 19450 keeps its t=0 price (current_avg=239, blue_volume=30, direction="0"), not the t=1 (0/2) values.
- **`jobs/wiki.go` RunWiki** — the weekly job as **ONE uninterrupted pass (no 6-minute resumable machinery — D-5)**: items+quest_items, then per-class spells, then the 2-page gear-tier full replace. A **1-second ctx-aware sleep runs before every wiki fetch** (SC-4), a **per-item SHA-1 short-circuit** (GetItemMasterSHA1Tx) skips unchanged item_master upserts, a **per-resource 304 skips** that page, the **gear full-replace fires only with the complete combined set** (a failed/304 page skips the replace — Pitfall 1), and a **single bad page is logged-and-skipped** (the run never aborts).
- **`store.DistinctInventoryItemIDs`** — the dedup-by-id (item_id, name) inventory union the wiki items pass iterates, excluding empty-slot (0/NULL) rows, mirroring the Sheet's collectInventoryItemRefs.
- **Single-tested-SQL-path holds:** grep confirms **0** INSERT/DELETE/UPDATE/ALTER in the production job files; every write goes through the 12-01 `store.*Tx` methods composed over one tx. **0** ported Sheets machinery (no cursor / watchdogs / lock / properties / view rebuilds) in the production job files.
- `go build ./...` + `go vet ./...` clean; `go test ./internal/backendsrv/...` 0 failures; whole-repo `go test ./...` = 28 packages, 0 failures (no v1 watcher regression).

## Task Commits

Each task was committed atomically (TDD `auto`: the failing test + impl co-committed per task):

1. **Task 1: store/itemids.go + urls.go + the daily PigParse job** — `9c8120c` (feat)
2. **Task 2: the weekly wiki job (items + spells + gear, single uninterrupted run)** — `a79b37b` (feat)

**Plan metadata:** committed separately with this SUMMARY + STATE.md + ROADMAP.md.

## Files Created/Modified
- `internal/backendsrv/enrich/jobs/urls.go` — hardcoded `PigparseURL` (getall/1) + `WikiAPIBase` constants (SSRF mitigation: never user-supplied) + `wikiParseURL` builder (redirects=1, query-escaped underscored title; doubles as the etag_cache key).
- `internal/backendsrv/enrich/jobs/pigparse.go` — `RunPigparse` + the D-9 WTS filter + the D-4 truncation-guard-as-LOG + `lastKnownRowCount`/`parseRowsDetail` (read the prior 'rows=N' baseline).
- `internal/backendsrv/enrich/jobs/pigparse_test.go` — WTS-only (4,333 rows, item 19450 t=0 price), 304-skips-write (sentinel row untouched), truncation-guard-logs-but-writes, fetch-error-records-error.
- `internal/backendsrv/enrich/jobs/wiki.go` — `RunWiki` + the 3 sub-passes (items/spells/gear) + `fetchWikiPage` (3-way outcome) + `upsertItemAndQuests` (SHA-1 short-circuit + item_master + quest_items in one tx) + the ctx-aware `wikiSleepFn` seam + `setWikiSleepNoop`.
- `internal/backendsrv/enrich/jobs/wiki_test.go` — httptest fixture server (page= -> testdata fixture); all-4-tables-populated (+Iksar +summary/sha1), SHA-1-short-circuit, 304-skips-resource, gear-full-replace-no-duplicates, one-bad-page-does-not-abort.
- `internal/backendsrv/enrich/jobs/helpers_test.go` — shared test helpers (fakeFetcher, countRows, assertJobStatus, local seedOwnerChar).
- `internal/backendsrv/store/itemids.go` — `DistinctInventoryItemIDs` + `ItemRef` (dedup-by-id union, read side, single SQL path).
- `internal/backendsrv/store/itemids_test.go` — dedup-by-id + 0/NULL exclusion + ordering, over NewTestDB.
- `internal/backendsrv/store/jobstate.go` — added `GetJobRunDetail` (reads last_detail for the truncation-guard baseline).

## Decisions Made
- **Dedup the inventory union by item_id alone** (GROUP BY item_id, MIN(name)) — the Sheet's collectInventoryItemRefs keeps one name per id (first-write-wins); a naive `SELECT DISTINCT item_id, name` would keep both "Cloth Cap" and "Cloth Cap (dup)" for id 1001 and make the wiki pass fetch the same page twice (a politeness regression). Caught by `TestDistinctInventoryItemIDs` (see Deviations Rule 1).
- **Truncation-guard baseline via a new store read method.** The guard compares today's kept count against the prior run's count, which the job stores in `job_run.last_detail` as `rows=N`. Reading it back needed a `store.GetJobRunDetail` (12-01's `GetJobRun` returns status but not detail) — added on the single SQL path rather than an inline job query (Rule 3).
- **A 304 on a gear page skips the full replace.** Gear-tier is a full-table replace of BOTH pages combined; a 304 means we don't have that page's current rows, so replacing with only the other page's rows would wipe it. Only a fresh 200 on both pages triggers the combined replace (faithful to the TS partial-failure abort + Pitfall 1).
- **`classAbbrevToDisplay` inverts `CLASS_DISPLAY_TO_ABBREV` at init** for the spell-page titles — eqconst.go carries the display->abbrev map but not the reverse, and the wiki spell URL needs the display name (e.g. "Necromancer"), mirroring refreshWikiSpells.ts.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] DistinctInventoryItemIDs must dedup by item_id, not (item_id, name)**
- **Found during:** Task 1 (store/itemids.go)
- **Issue:** The plan's literal SQL sketch was `SELECT DISTINCT item_id, name`. That keeps a SEPARATE row for each distinct (id, name) pair, so an item that appears under two names/casings (e.g. id 1001 as "Cloth Cap" and "Cloth Cap (dup)") yields TWO refs — and the wiki items pass would fetch the same wiki page twice. The Sheet's union (collectInventoryItemRefs) dedups by id alone (`if (!seen.has(id)) seen.set(id, name)`).
- **Fix:** Changed the query to `SELECT item_id, MIN(name) AS name ... GROUP BY item_id ORDER BY item_id` so each id yields exactly one ref with a deterministic representative name. Still excludes item_id 0/NULL. Tightened the test to assert one row per id (1001/11000/13128) with the MIN name.
- **Files modified:** internal/backendsrv/store/itemids.go, internal/backendsrv/store/itemids_test.go
- **Verification:** `TestDistinctInventoryItemIDs` (which RED-failed against the per-pair query, showing 4 refs incl. the dup) passes with the GROUP BY (3 refs).
- **Committed in:** `9c8120c` (Task 1 commit)

**2. [Rule 3 - Blocking] store.GetJobRunDetail added so the truncation guard can read its baseline**
- **Found during:** Task 1 (pigparse.go truncation guard)
- **Issue:** The D-4 truncation guard compares today's row count against the last-known count, which the plan stores in `job_run.last_detail` (`rows=N`). But 12-01's `store.GetJobRun` returns `(lastRun, status, ok, err)` — NOT the detail. Reading the detail inline in the job would author SQL outside the single tested SQL path.
- **Fix:** Added `GetJobRunDetail(ctx, jobName) (string, error)` to `store/jobstate.go` (NullString read, "" for absent/NULL) and called it from the job's `lastKnownRowCount`. Keeps all SQL in store/.
- **Files modified:** internal/backendsrv/store/jobstate.go, internal/backendsrv/enrich/jobs/pigparse.go
- **Verification:** `TestRunPigparse_TruncationGuardLogsButWrites` seeds a prior `rows=5000` job_run, runs a 2-row body, and asserts it STILL writes 2 rows (guard logged, did not abort) — exercising the read end-to-end.
- **Committed in:** `9c8120c` (Task 1 commit)

### Cosmetic doc-comment rewordings (no behavior change)

Reworded the package/method doc comments in `wiki.go`, `urls.go`, and `pigparse.go` so the plan's `<verification>` machinery grep (`CURSOR|monitorCellCount|weeklySchemaHealthcheck|buildSpellCheck|buildGearCheck|LockService|PropertiesService`) returns **0** across the production job files. The comments originally NAMED the deleted Apps Script constructs to explain they are *not ported*; the rewording describes them descriptively ("the 6-minute resumable-position machinery", "the Apps Script document lock", "the post-run gear/spell view rebuilds", "job_run position marker"). This mirrors the same doc-vs-grep adjustment 12-01/12-02 made. No code behavior changed; the D-5/D-8 intent (none of that machinery is actually ported) is unchanged and was already true.

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking) + cosmetic doc rewordings for grep-cleanliness.
**Impact on plan:** Both auto-fixes were necessary for correctness (the dedup-by-id avoids duplicate wiki fetches; the GetJobRunDetail read is required for the truncation guard to function while honoring the single-SQL-path rule). No scope creep — every change stayed inside the jobs + the item-id/job-detail store reads the plan scoped.

## Issues Encountered
- **Test-file seed SQL note:** the `_test.go` helpers author raw `INSERT INTO inventory_item` / `INSERT INTO pigparse_price` (and the SetJobRun seed) for precise control of item_id 0/NULL cases and the 304-sentinel/truncation-baseline fixtures. This mirrors the store package's own `seedOwnerChar`/`seedRaw` test precedent. The load-bearing acceptance greps target the PRODUCTION job files (`pigparse.go`/`wiki.go`), which are 0 — the single-tested-SQL-path constraint is about job code, not test setup. A blanket recursive grep over `jobs/` will still match the test seeds; this is expected and not a single-SQL-path violation.

## Threat Surface Scan
No new security-relevant surface beyond the plan's `<threat_model>`. The jobs add no inbound endpoint (timer-triggered), no auth, no secrets; the fetch destinations are hardcoded constants in urls.go (T-12.04-04 accept — no SSRF vector); all writes are parameterized store methods (T-12.04-03 mitigate); 304 + truncation handling preserve good rows (T-12.04-01/02 mitigate); the 1s ctx-aware sleep + log-but-continue keep the crawl a good citizen (T-12.04-05 mitigate); slog logs counts/ids/status/err only, never raw bodies (T-12.04-06 mitigate). No `threat_flag` raised.

## User Setup Required
None — the PigParse + P1999 wiki APIs are public and unauthenticated (no API key, no secret in this path). Live API calls happen only when the Wave-3 (12-05) scheduler runs these jobs on the VPS.

## Next Phase Readiness
- **12-05 (scheduler) is unblocked.** It registers `RunPigparse` + `RunWiki` into a real db-backed scheduler: the due-check reads `store.GetJobRun` (never-run -> zero time -> due-on-startup), the per-job `sync.Mutex` replaces LockService, and `squirebot-server run-job pigparse|wiki` (D-7 parity check) calls these two functions directly with `politefetch.Fetch` as the production Fetcher. The jobs already advance `job_run` on every outcome (ok|skipped_unchanged|error), so the scheduler only needs the cadence predicates (D-10) + immediate-check-on-startup.
- **Scope guard (D-8) held:** the jobs only WRITE the dimension tables; no VIEW build, no Sheet->DB backfill, no eviction/archive job. The `-ldflags -X .../buildinfo.Version` stamping for the politeFetch User-Agent is wired at the 12-05/deploy step (buildinfo.Version defaults to "dev").
- No blockers.

## Self-Check: PASSED

- All 8 created files + the SUMMARY exist on disk (verified below).
- Both task commit hashes (`9c8120c`, `a79b37b`) exist in git history (verified below).
- `go build ./...` + `go vet ./...` clean; `go test ./...` = 28 packages, 0 failures; production-job-file inline-SQL grep = 0; production-job-file deleted-machinery grep = 0.

---
*Phase: 12-enrichment-job-migration*
*Completed: 2026-05-30*
