---
phase: 12-enrichment-job-migration
verified: 2026-05-29T22:25:00Z
status: human_needed
score: 8/8 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification:
  - test: "Run one daily PigParse + one weekly wiki cycle on the live VPS, then spot-check the backend's pigparse_price / item_master / wiki_spells / wiki_gear_tier / quest_items against the live Sheet's _pigparse / _item_master / _wiki_spells / _wiki_gear_tier / _quest_items for the same item_ids (D-7 / SC-4)."
    expected: "Per-item current_avg/t30/a30 equal between backend and Sheet for ~10 well-known item_ids (Cloak of Flames, Fungi Tunic, etc.); backend pigparse_price row count is LOWER than the Sheet's _pigparse (D-9 WTS-only dedup — expected, not a failure); spell/gear/quest counts match within the SHA-1/full-replace semantics."
    why_human: "Requires the jobs to fire on their production timer (24h / Sunday) on the deployed Hetzner VPS AND a side-by-side read of the still-live Google Sheet. Inherently an operational comparison against an external system — not verifiable from the codebase. NOTE: the verifier already exercised both jobs end-to-end against the LIVE PigParse + P1999 wiki APIs via `run-job` (pigparse=4,338 WTS rows, wiki=14 classes + 1,183 gear rows, all 4 tables populated), so the code path is proven; only the Sheet-parity equality assertion remains for the operator."
---

# Phase 12: Enrichment Job Migration — Verification Report

**Phase Goal:** Move the two enrichment feeds off Apps Script triggers and into the backend as in-process scheduled jobs so the new SQLite DB self-populates its dimension data — the daily PigParse price pull (ENRICH-10) and the weekly P1999 wiki scrape (ENRICH-11) — with the existing parsers and `politeFetch` politeness controls carried over verbatim. Backend scheduled jobs only; no UI.
**Verified:** 2026-05-29T22:25:00Z
**Status:** human_needed (all 8 code-level must-haves VERIFIED; only the SC-4 live Sheet-parity spot-check remains, which is operational by nature)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (the 4 ROADMAP Success Criteria + the 8 must-haves)

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| SC-1 / MH-1 | Daily PigParse job: fetches `getall/1` via polite client, filters to WTS t=0 (D-9), upserts `pigparse_price`, degrades gracefully (truncation-guard-as-LOG, D-4), registered daily (D-10) | ✓ VERIFIED | `enrich/jobs/pigparse.go` — WTS filter lines 102-107, truncation-guard-as-LOG (slog.Warn, no return) lines 113-117, 304-skip lines 86-89, zero inline SQL (composes `store.UpsertPigparsePricesTx`). **LIVE run:** `run-job pigparse` against real PigParse API upserted 4,338 WTS rows, `distinct directions=1` (only t=0 survived), job_run=ok. |
| SC-2 / MH-2 | Weekly wiki job: ONE uninterrupted pass (NO resumable cursor, D-5), populates item_master + wiki_spells + wiki_gear_tier + quest_items, 1s inter-request sleep + SHA-1 short-circuit, registered Sunday (D-10) | ✓ VERIFIED | `enrich/jobs/wiki.go` — single pass items→spells→gear (lines 138-154), `interRequestSleep=1*time.Second` before every fetch (lines 169,274,349), SHA-1 short-circuit (lines 224-231), no cursor/watchdog. **LIVE run:** `run-job wiki` against real wiki API → `items_ok=1 spells_classes=14 gear_replaced=true gear_rows=1183`, all 4 tables populated. |
| SC-3 / MH-3 | `politeFetch` controls observably in force on both jobs: identifying User-Agent, ETag/If-Modified-Since→304 short-circuit, exponential backoff honoring Retry-After, 1s wiki sleep | ✓ VERIFIED | `enrich/politefetch/politefetch.go` — UA via `buildinfo.UserAgent()` line 150, If-None-Match+If-Modified-Since lines 151-156, 304→FromCache lines 205-218, backoff [2s,4s,8s,16s,32s] lines 84-90, Retry-After clamp 0-600 lines 270-282, TLS on. Tests assert UA prefix+github URL (292-296), 304 (109,128), Retry-After honored (224). **LIVE:** etag_cache grew to 18 rows (304 state persisting). |
| MH-4 | `00003` migration adds 8 pigparse price cols + `job_run` + `etag_cache`, never edits 00001/00002, exercised by a test asserting columns/tables exist | ✓ VERIFIED | `migrations/00003_enrich_columns.sql` — exactly t30/a30/t60/a60/t6m/a6m/ty/ay + job_run + etag_cache; 00001/00002 untouched. `migrate_test.go::TestMigrate_00003_AddsEnrichColumnsAndTables` asserts all 8 cols via PRAGMA table_info + both tables. **LIVE:** goose migrated fresh DB to v3, all 8 cols populated (4,338 rows w/ a30 set). |
| MH-5 | The two verified hazards avoided: `wiki_gear_tier` FULL-TABLE replace (not upsert on broken UNIQUE-on-NULL); PigParse dedups t=0/t=1 (no PK violation) | ✓ VERIFIED | `store/enrich.go::ReplaceWikiGearTierTx` = `DELETE FROM wiki_gear_tier` (no WHERE) + INSERT (lines 278-300). `enrich_test.go::TestReplaceWikiGearTier_FullReplaceDoesNotDuplicate` — 2nd identical replace = N not 2N. `pigparse_test.go::TestRunPigparse_UpsertsWTSOnly` — real 7,240-row fixture (incl. dup item 19450) → 4,333 rows, item 19450 holds t=0 price. **LIVE:** gear 1,183 rows non-duplicating; pigparse distinct directions=1. |
| MH-6 | Single-tested-SQL-path: zero inline DELETE/INSERT/UPDATE/ALTER SQL in `enrich/jobs/` or `scheduler/`; all SQL in `store/` + `migrations/` | ✓ VERIFIED | Grep of `jobs/*.go` + `scheduler/*.go` non-test: zero DML statements (2 wiki.go matches are doc COMMENTS; all real SQL strings are in `_test.go` seeding). Jobs compose `store.*Tx`; scheduler uses `store.GetJobRun/SetJobRun`. |
| MH-7 | No scope creep (D-8): no VIEW-tab/read-API building, no eviction/backfill; no Google/OAuth/Sheets/PocketBase dependency | ✓ VERIFIED | `go list -deps ./cmd/squirebot-server` + `./internal/backendsrv/...` → only match is stdlib `database/sql/driver` (coincidental "drive" substring). No google/oauth2/sheets/pocketbase/gapi. No evict/archive/backfill/view code in enrich tree. Only HTTP route is the P11 `POST /api/v1/ingest`. |
| MH-8 | `go build ./...`, `go vet ./...`, `go test ./...` all green | ✓ VERIFIED | build exit 0; vet exit 0; `go test -count=1` all green — enrich (28), jobs (9), politefetch (11), store (21), scheduler (8), migrations (4). Linux/amd64 static ELF cross-compiles. |

**Score:** 8/8 must-haves verified (all four ROADMAP Success Criteria SC-1/2/3 proven; SC-4 parity is the operational/human check below).

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00003_enrich_columns.sql` | 8 price cols + job_run + etag_cache, forward-only | ✓ VERIFIED | 47 lines; 8 single-col ALTERs + 2 CREATE TABLE; 00001/00002 not edited; goose Up/Down |
| `internal/backendsrv/store/enrich.go` | 5 dimension write methods, single tested SQL path | ✓ VERIFIED | 343 lines; pigparse upsert / item_master upsert+SHA1 getter / wiki_spells per-class replace / gear FULL replace / quest per-id replace; all `?`-bound |
| `internal/backendsrv/store/jobstate.go` | job_run + etag_cache cursors | ✓ VERIFIED | GetJobRun/SetJobRun/GetETag/SetETag/GetJobRunDetail (tested in jobstate_test.go: 4 tests) |
| `internal/backendsrv/enrich/pigparse.go` (+ wikiitem/wikispell/wikigear/eqconst) | 4 pure parsers, byte-parity | ✓ VERIFIED | `ParseToRows` 7240-row parity, `ParseItempage`+SHA-1 (TestSHA1Hex_MatchesTS), `ParseClassPage` (NEC=171), `ParseGearTierPage`; pure (no net/SQL); fixtures reused from apps-script |
| `internal/backendsrv/enrich/politefetch/politefetch.go` | net/http polite client, 12 controls | ✓ VERIFIED | 292 lines; all SC-3 controls; io.LimitReader 16MB cap; ctx-aware sleep seam |
| `internal/backendsrv/buildinfo/buildinfo.go` | settable Version + UserAgent (D-11) | ✓ VERIFIED | `var Version="dev"` (-ldflags settable); UA `SquireBot/<ver> (+github)` |
| `internal/backendsrv/enrich/jobs/pigparse.go` | daily job, composes Wave-1 | ✓ VERIFIED | 199 lines; WTS filter + truncation-LOG + 304-skip + advance-always; zero inline SQL |
| `internal/backendsrv/enrich/jobs/wiki.go` | weekly single-run job, composes Wave-1 | ✓ VERIFIED | 477 lines; one pass, 1s sleeps, SHA-1 short-circuit, gear-full-replace-only-if-complete, log-but-continue; zero inline SQL |
| `internal/backendsrv/scheduler/scheduler.go` | 2 jobs registered, cursor+due+mutex+immediate-check | ✓ VERIFIED | 244 lines; pigparse_daily (>=24h) + wiki_weekly (Sunday UTC) wired to `politefetch.Fetch`; immediate-check-on-startup; advance-always; per-job TryLock; ctx.Done() shutdown verbatim |
| `cmd/squirebot-server/main.go` | scheduler.Start wired into serve + run-job CLI | ✓ VERIFIED | `runServe` runs migrations→`scheduler.Start(ctx,db)` (line 251); `run-job pigparse\|wiki` (D-7 parity entrypoint) with exit-2 arg validation |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `scheduler.Start` | server `runServe` | `scheduler.Start(ctx, db)` after migrations | ✓ WIRED | main.go:251 — real production wiring, ctx-driven shutdown; not orphaned |
| `pigparse.go` job | PigParse API | `politefetch.Fetch(ctx, PigparseURL, …)` | ✓ WIRED | `PigparseURL` hardcoded const; injected `politefetch.Fetcher`; LIVE run hit real API |
| `wiki.go` job | P1999 wiki API | `politefetch.Fetch(ctx, wikiParseURL(title), …)` | ✓ WIRED | `WikiAPIBase` const + `wikiParseURL` builder; LIVE run fetched 14 class + 2 gear + 1 item pages |
| jobs | DB | `store.*Tx` over one `*sql.Tx` | ✓ WIRED | Zero inline SQL; composes UpsertPigparsePricesTx / UpsertItemMasterTx / UpsertWikiSpellsForClassTx / ReplaceWikiGearTierTx / ReplaceQuestItemsForIDTx |
| politefetch | UA | `buildinfo.UserAgent()` | ✓ WIRED | Imported + called per request (politefetch.go:150) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `pigparse_price` table | a30/t30/...8 cols | live PigParse `getall/1` → ParseToRows → WTS filter → UpsertPigparsePricesTx | ✓ Yes — 4,338 rows, all a30 set | ✓ FLOWING |
| `wiki_spells` table | class/level/spell_name | live wiki class pages → ParseClassPage → UpsertWikiSpellsForClassTx | ✓ Yes — 1,562 rows, 11 classes | ✓ FLOWING |
| `wiki_gear_tier` table | tier/class/slot/item_name | live 2 Velious pages → ParseGearTierPage → ReplaceWikiGearTierTx | ✓ Yes — 1,183 rows, 3 tiers, 4 Iksar | ✓ FLOWING |
| `item_master` table | name/url/slot/sha1 | live item wiki page → ParseItempage → UpsertItemMasterTx | ✓ Yes — name/url/slot/sha1 populated (summary empty for that item, parser-faithful) | ✓ FLOWING |
| `etag_cache` table | etag/last_modified | politefetch response headers → SetETag | ✓ Yes — 18 rows after wiki run | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Server cross-compiles for deploy target | `GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` | static ELF x86-64, statically linked | ✓ PASS |
| run-job arg validation (missing/dup/unknown) | `run-job` / `run-job pigparse wiki` / `run-job bogus` | all exit 2 with usage message | ✓ PASS |
| ENRICH-10 end-to-end (LIVE PigParse) | `run-job pigparse --db <fresh>` | migrated to v3; 4,338 WTS rows; direction=0 only; job_run=ok; exit 0 | ✓ PASS |
| ENRICH-11 end-to-end (LIVE wiki) | `run-job wiki --db <seeded>` | 14 classes, 1,183 gear rows, all 4 tables populated; 1s sleeps observed; exit 0 | ✓ PASS |
| Phase-12 test suite (no cache) | `go test -count=1 ./internal/backendsrv/...` | enrich 28 / jobs 9 / politefetch 11 / store 21 / scheduler 8 / migrations 4 — all PASS | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
| ----------- | -------------- | ----------- | ------ | -------- |
| ENRICH-10 | 12-01,02,03,04,05 | Daily PigParse pull as in-process job, reusing parser + politeFetch | ✓ SATISFIED | pigparse job + scheduler `pigparse_daily` (>=24h) + LIVE 4,338-row run; all 5 plans declare it |
| ENRICH-11 | 12-01,02,03,04,05 | Weekly wiki scrape (items/spells/gear/quests) as in-process job, same politeness | ✓ SATISFIED | wiki job + scheduler `wiki_weekly` (Sunday UTC) + LIVE 14-class/1,183-gear run; all 5 plans declare it |

**No orphaned requirements.** REQUIREMENTS.md maps ONLY ENRICH-10/11 to P12; both are claimed in every plan's `requirements:` frontmatter and both are marked complete with matching evidence.

### Locked Decisions (D-1..D-12) — Honored

| Decision | Status | Evidence |
| -------- | ------ | -------- |
| D-1 port 4 pure parsers verbatim, reuse fixtures | ✓ | enrich/*.go pure; testdata/ has the 12 apps-script fixtures; byte-parity tests pass |
| D-2 register 2 jobs, persist last-run in DB | ✓ | scheduler registry + job_run cursor |
| D-3 port politeFetch w/ all controls, ETag→DB | ✓ | politefetch.go + etag_cache table |
| D-4 graceful degradation via upsert, truncation-guard-as-LOG | ✓ | ON CONFLICT upsert + slog.Warn no-abort (pigparse.go:113-117) |
| D-5 delete AS workarounds (cursor/watchdogs/LockService) | ✓ | grep: only COMMENTS reference them; no impl; per-job mutex replaces LockService |
| D-6 populate the 5 dimension tables; add 00003 for missing cols | ✓ | 00003 adds 8 pigparse cols; all 5 tables populated in LIVE run |
| D-7 parity check is acceptance proof | ✓ (operational) | run-job entrypoint exists; flagged for human Sheet-parity below |
| D-8 strict scope: 2 feeds only, no eviction/backfill/views | ✓ | no evict/archive/backfill/view code; no forbidden deps |
| D-9 PigParse keep WTS (t=0) before upsert | ✓ | isolated filter pigparse.go:102-107; LIVE direction=0 only |
| D-10 cadence = simple interval (24h / Sunday UTC) | ✓ | duePigparse/dueWiki; 6-case TestDueWiki |
| D-11 backend Version var for UA | ✓ | buildinfo.Version + UserAgent |
| D-12 stale-row cleanup = Sheet-faithful per-key replace | ✓ | per-class / per-id / full-gear-replace / per-item-upsert all in store/enrich.go |

### Verified Hazards (12-RESEARCH) — Both Avoided in Code

| Hazard | Mitigation in Code | Test Proof | LIVE Proof |
| ------ | ------------------ | ---------- | ---------- |
| `wiki_gear_tier` UNIQUE(…,item_id) broken (item_id always NULL → upsert never fires → dup explosion) | `ReplaceWikiGearTierTx` = DELETE-all + INSERT (full-table replace), NOT upsert | TestReplaceWikiGearTier_FullReplaceDoesNotDuplicate (N not 2N) + TestRunWiki_GearFullReplaceNoDuplicates | gear 1,183 rows, non-duplicating on the full-replace |
| PigParse t=0/t=1 dup item_id vs PK on item_id (naive insert → PK violation) | D-9 filter to WTS t=0 before upsert (one isolated filter) | TestRunPigparse_UpsertsWTSOnly (real fixture 7,240→4,333, item 19450 keeps t=0) | LIVE 4,338 rows, distinct directions=1, no PK error |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | No TODO/FIXME/placeholder/panic in enrich tree; no inline DML in jobs/scheduler; no `time.Tick` leak (scheduler correctly uses `time.NewTicker`) | — | No blockers, no warnings |

### Human Verification Required

**1. Live daily + weekly cycle → Sheet-parity spot-check (D-7 / SC-4)**

- **Test:** Let `pigparse_daily` (24h cadence) and `wiki_weekly` (Sunday UTC) fire on the deployed Hetzner VPS — OR run `squirebot-server run-job pigparse` and `run-job wiki` against the production DB on-box. Then compare the backend's `pigparse_price` / `item_master` / `wiki_spells` / `wiki_gear_tier` / `quest_items` against the still-live Google Sheet's `_pigparse` / `_item_master` / `_wiki_spells` / `_wiki_gear_tier` / `_quest_items` for ~10 well-known item_ids (Cloak of Flames, Fungi Tunic, etc.).
- **Expected:** Per-item `current_avg`/`t30`/`a30` equal between backend and Sheet (same source API, same parser). Backend `pigparse_price` row count is LOWER than the Sheet's `_pigparse` (D-9 WTS-only dedup — expected and documented, NOT a failure). Spell/gear/quest set membership matches the SHA-1/full-replace semantics.
- **Why human:** Requires the production timer to fire on the deployed VPS and a side-by-side read of an external Google Sheet — an operational comparison against a live external system, not verifiable from the codebase. **The verifier already proved the full code path end-to-end against the LIVE PigParse + P1999 wiki APIs** (pigparse 4,338 WTS rows; wiki 14 classes + 1,183 gear rows; all 4 tables populated with real data); only the Sheet-equality assertion remains for the operator.

### Gaps Summary

**No code gaps.** All 8 must-haves and all 4 ROADMAP Success Criteria (SC-1/2/3 fully; SC-4's code path) are VERIFIED against the actual Go source, the test suite (81 Phase-12 tests green with `-count=1`), AND live end-to-end runs against the real PigParse and P1999 wiki services. Every locked decision (D-1..D-12) is honored; both verified hazards are avoided in code with explicit regression tests; the single-tested-SQL-path rule holds (zero inline DML in jobs/scheduler); no scope creep and no forbidden Google/OAuth/Sheets/PocketBase dependency.

The 12-03-SUMMARY (orchestrator-backfilled after an executor socket-close) was scrutinized with extra care: its claims are accurate — the 4 commits (`124c55b`, `0aefd01`, `33aa224`, `1a32144`) exist, politefetch.go (292 lines) + buildinfo.go (42 lines) are complete and substantive, and all 11 politefetch + 2 buildinfo tests pass.

Status is `human_needed` solely because ROADMAP Success Criterion 4 (live Sheet-parity spot-check) is inherently an operational check on the deployed VPS against an external Google Sheet — per the verification brief, this is classified as human verification rather than a code gap. Everything verifiable from the codebase passes.

---

_Verified: 2026-05-29T22:25:00Z_
_Verifier: Claude (gsd-verifier)_
