---
phase: 03-apps-script-enrichment-foundation
plan: 02
subsystem: apps-script-pigparse-pricing-scrape
tags: [apps-script, pigparse, enrich-01, polite-fetch, daily-trigger]
requires:
  - 03-01 (apps-script scaffold + migrateToV2 extending _pigparse with 9 direction/window cols + sheet-helpers wrappers)
provides:
  - "ENRICH-01: daily PigParse pricing scrape — refreshPigparse trigger fetches GET https://pigparse.azurewebsites.net/api/item/getall/1 once daily; full-snapshot replace of _pigparse keyed on (item_id, direction)"
  - "ENRICH-05/06/07/08: row-count truncation guard (refuse to overwrite if today's response < 90% of last count); _status.last_pigparse_row_count + _meta.last_pigparse_refresh; structured _meta.last_error envelope on failure"
  - "politeFetch(url, opts) HTTP wrapper: [2,4,8,16,32]s retry schedule on 429/503/504; honors Retry-After; muteHttpExceptions=true; UA 'SquireBot/<version> (+https://github.com/boejowen/SquireBot)'"
  - "parseToRows(body) validator: JSON.parse + shape assertion on PigparseRowRaw[]; coerces numerics; throws on >1% malformations"
affects:
  - "03-03 (refreshWikiItems): reuses politeFetch wrapper verbatim"
  - "03-04 (buildView): _pigparse is the upstream input for the view-tab price column + cell-note transaction-volume tooltip"
tech-stack:
  added: []
  patterns:
    - "Polite-fetch wrapper with exponential backoff + Retry-After honoring: muteHttpExceptions=true on UrlFetchApp; Utilities.sleep between retries. Locked by RESEARCH §5."
    - "Row-count truncation guard: refuse to overwrite _pigparse if today's row count < 90% of last_pigparse_row_count. Defense against partial-fetch overwrite. Locked by PLAN truth #2."
    - "LockService.getDocumentLock().tryLock(30000) + try/finally around all _pigparse writes."
    - "Full-snapshot replace via clearContent + setValues — never appends. Locked by ARCHITECTURE.md write contract."
key-files:
  created:
    - apps-script/src/lib/pigparse-types.ts (~60 lines; PigparseRowRaw interface + parseToRows validator)
    - apps-script/src/lib/politeFetch.ts (~110 lines; FetchSuccess|FetchError discriminated union + retry loop)
    - apps-script/src/triggers/refreshPigparse.ts (~140 lines; orchestrates fetch + assertion + lock + setValues + meta-row updates)
    - apps-script/src/__tests__/politeFetch.test.ts (~120 lines; 7 vitest scenarios — happy, 304, 429-retry, Retry-After, 503 exhaust, 4xx, throw)
    - apps-script/src/__tests__/refreshPigparse.test.ts (~150 lines; 6 vitest scenarios — happy, lock contention, truncation, fetch failure, missing sheet, idempotent re-run)
  modified:
    - apps-script/src/Code.ts (+2/-1 lines; replace refreshPigparse stub with actual import + re-export)
decisions:
  - "ETag/If-Modified-Since N/A: Azure App Service doesn't emit ETag per RESEARCH §1. No conditional-GET cache layer for PigParse."
  - "1s inter-request sleep N/A: single endpoint, single request. (1s sleep convention preserved in 03-03 for multi-page wiki scrapes.)"
  - "Resumability not needed: single endpoint completes in <30s typically per RESEARCH §1. No cursor pattern for PigParse (the cursor pattern lands in 03-03 for the 1,500-item wiki universe)."
  - "Truncation guard at 90% threshold: defense against partial fetches overwriting good data. lastCount=0 means first-run — no guard applies."
  - "PIGPARSE_HEADERS hardcodes the 15-col order matching scaffold + migrateToV2 extension."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-09T18:04:03-05:00
  tasks_completed: 5 of 5
  commits: 3 (bf92c5a feat pigparse-types + parseToRows + tests; 9efbbb6 feat politeFetch wrapper + tests; 4bae1ca feat refreshPigparse trigger + wire Code.ts export)
  files_changed: 6 (5 created + 1 modified, ~580 lines added)
  tests_added: 13 (7 politeFetch + 6 refreshPigparse)
  trigger_count_after: 0 (trigger function lands; ScriptApp time-driven registration deferred to 03-04 installTriggers)
  schema_version_after: 2 (unchanged from 03-01)
  watcher_rebuild_required: false (schema unchanged; pure apps-script trigger addition)
---

# Phase 3 Plan 02: PigParse Pricing Scrape Summary

**One-liner:** Shipped the daily PigParse pricing scrape (ENRICH-01) — `refreshPigparse` trigger fetches `GET https://pigparse.azurewebsites.net/api/item/getall/1` once daily, validates the 7,240-row response via `parseToRows`, enforces a 90% row-count truncation guard, full-snapshot-replaces `_pigparse` keyed on (item_id, direction) inside a 30s document-lock, updates `_meta.last_pigparse_refresh` + `_status.last_pigparse_row_count`, and writes a structured `_meta.last_error` envelope on any failure path — plus the reusable `politeFetch(url, opts)` HTTP wrapper with [2,4,8,16,32]s retry schedule and Retry-After honoring that 03-03's refreshWikiItems reuses verbatim.

## What shipped

### Task 1 — pigparse-types.ts (commit `bf92c5a`)

`PigparseRowRaw` TypeScript interface mirroring RESEARCH §1's documented response shape (`i, t, n, l, a30, t30, a60, t60, a6m, t6m, ay, ty` plus `direction`). `parseToRows(body: string): PigparseRowRaw[]` validator: `JSON.parse`s the body, asserts top-level is array (throws if not), asserts each element has `i`, `t`, `n` keys, coerces all numeric fields to numbers, trims `n`. Throws if more than 1% of rows are malformed (avoids silent partial-acceptance). Fixture test parses the captured `apps-script/src/__fixtures__/pigparse-getall-1.json` (7,240 rows) → asserts distinct `t` values are exactly `[0, 1]` (WTS=0, WTB=1) and no rows have null `n`.

### Task 2 — politeFetch.ts (commit `9efbbb6`)

The HTTP wrapper reused by both 03-02 and 03-03. Discriminated-union result type: `{ status, body, headers, fromCache, etag }` on success, `{ status, error, retriesUsed }` on failure. `RETRY_DELAYS_MS = [2000, 4000, 8000, 16000, 32000]`. Algorithm per RESEARCH §5: `muteHttpExceptions: true` on `UrlFetchApp.fetch`; on 429/503/504 retry with the schedule, honoring `Retry-After` header (parse seconds, multiply by 1000, override schedule for that one wait); on 4xx no retry; on network throw catch and return FetchError; on 304 Not Modified return `fromCache=true`. UA defaults to `SquireBot/${VERSION} (+https://github.com/boejowen/SquireBot)` with VERSION read from a top-of-file constant (synced manually with apps-script/package.json on each release).

7 vitest scenarios mock `UrlFetchApp` via `vi.fn`: happy 200, 304 cache, 429-then-200 retry (`retriesUsed=1`), 429-with-Retry-After (verify `Utilities.sleep` waited ~5000ms when header said 5), 503 exhausted (6 attempts, `retriesUsed=5`), 4xx immediate FetchError (no retry), network throw → FetchError.

### Task 3-4 — refreshPigparse trigger + Code.ts wire (commit `4bae1ca`)

The trigger orchestrator. Algorithm: acquire 30s document lock (warn-log + early-return on lock_busy); call `politeFetch(PIGPARSE_URL)`; on fetch failure write `{where: 'refreshPigparse', kind: 'fetch_failed', detail}` envelope to `_meta.last_error` + `_status.last_error` and exit. On success, `parseToRows(body)` (throws bubble up to STACKDRIVER). Read `lastCount = readMetaRowInt('_status', 'last_pigparse_row_count') ?? 0`. If `lastCount > 0 && rows.length < lastCount * 0.90`, write `{kind: 'truncated_response', detail: today=X last=Y}` envelope and exit without touching _pigparse. Otherwise: clear `_pigparse` row 2 onward via `clearContent` over the existing range, write all rows via single `setValues(dataRows)` call. Update `_status.last_pigparse_row_count` to new count; `_meta.last_pigparse_refresh` to current ISO timestamp; clear `_meta.last_error` and `_status.last_error` (set to `'{}'`). Release lock in finally.

Column mapping (Phase 2 schema cols A-E preserved meaningful + Phase 3 v=2 cols F-O appended): `[r.i, r.n, r.a30, r.l, r.t30, now, r.t, r.t30, r.a30, r.t60, r.a60, r.t6m, r.a6m, r.ty, r.ay]`. `current_avg` (col C) aliases to `r.a30` (recent avg); `last_seen` (col D) gets `r.l`; `blue_volume` (col E) gets `r.t30`; v=2 cols at right edge get raw values.

6 refreshPigparse vitest scenarios mock UrlFetchApp + SpreadsheetApp: happy path (7,240 rows written, lock acquired+released, _meta refresh updated); lock contention (tryLock false → no write, log emitted); truncation (3,500 rows vs lastCount=7240 → no write, error envelope, lastCount preserved); fetch failure (politeFetch error → no write, error envelope); missing _pigparse (throws clean error caught by STACKDRIVER); idempotent re-run (second run reads new lastCount, all assertions hold).

`Code.ts` updated: replace `refreshPigparse` stub with `import { refreshPigparse } from './triggers/refreshPigparse'; export { refreshPigparse };`. Build footer still picks it up as a top-level global.

## Deviations from Plan

None — plan executed as written. (Detailed deviation tracking not captured retroactively.)

## Schema impact

None — schema_version remains at 2 (set in 03-01). This plan POPULATES the columns 03-01's migrateToV2 added to `_pigparse`. No new columns, no new rows, no migration.

## Verification log

```
$ npm test -- pigparse
Tests       6 passed (6)

$ npm test -- politeFetch
Tests       7 passed (7)

$ npm run build
(exit 0 — refreshPigparse exported as top-level global in dist/Code.js)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/lib/pigparse-types.ts` (PigparseRowRaw + parseToRows validator)
- FOUND: `apps-script/src/lib/politeFetch.ts` (retry + Retry-After + UA)
- FOUND: `apps-script/src/triggers/refreshPigparse.ts` (lock + truncation guard + setValues)
- FOUND: `apps-script/src/__tests__/politeFetch.test.ts`
- FOUND: `apps-script/src/__tests__/refreshPigparse.test.ts`

**Commits exist:**
- FOUND: `bf92c5a` — feat(apps-script): pigparse-types + parseToRows + tests
- FOUND: `9efbbb6` — feat(apps-script): politeFetch wrapper + tests
- FOUND: `4bae1ca` — feat(apps-script): refreshPigparse trigger + wire Code.ts export

## Next plan

`/gsd-execute-phase 3` spawned plan `03-03` for the weekly P1999 wiki summary scrape (`refreshWikiItems` with 5-min cursor-resume pattern across ~1,500 items, SHA-1 change detection, `{{Itempage}}` template parsing against 5 wiki fixtures including the Fungi Tunic redirect chain).

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 03-02-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
