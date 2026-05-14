---
phase: 05-search-onboarding-privacy-polish
plan: 02
subsystem: apps-script-archive-and-eviction-backend
tags: [apps-script, archive, housekeeping, eviction-backend, view-05, doc-02]
requires:
  - 05-01 (SQUIREBOT_HANDLERS 7->8 baseline; test-helpers hideSheet/isSheetHidden/getSheetId mocks)
  - 03 (sheet-helpers.ts: readMetaRows / writeMetaRow / getActiveSpreadsheet / getOrCreateSheet)
  - 04 (migrations.ts: lock-guarded mutation envelope cloned by archive.ts; _char_owner 13-col schema with is_removed at col 9)
provides:
  - "VIEW-05: weekly Sun-06:00 PT stale-char archiver -- chars with last_seen >90d move to hidden _archive tab; _char_owner.is_removed=TRUE flipped; source inv:/spell: tabs hidden (NOT deleted)"
  - "DOC-02 back-end: weekly Sun-06:00 PT eviction-archive trigger -- processes _meta.eviction_log entries whose grace_until < now; atomic-per-entry on failure; idempotent retry via moveCharToArchive's own idempotency"
  - "lib/archive.moveCharToArchive(charName, reason): shared lock-guarded helper consumed by both weekly triggers AND (later) by 05-04's commitEviction immediate-archive path"
  - "_archive: lazy-created hidden tab with 7-col header (archived_at, char_name, tab_type, row_count, uploaded_at, reason, snapshot_json) -- no schema_version bump"
  - "_meta.archive_log: new KV row, append-only JSON array of {at, char, reason, inv_row_count, spell_row_count} envelopes -- audit trail"
  - "_status.last_stale_archive_run/_count + _status.last_eviction_archive_run/_count: weekly book-keeping rows for the watcher heartbeat health surface"
  - "SQUIREBOT_HANDLERS 8->10; total trigger count after 05-02 = 10"
affects:
  - "Phase 5 plan 05-04 (eviction sidebar): commitEviction will WRITE _meta.eviction_log entries that THIS plan's weeklyEvictionArchive consumes after 30 days. The eviction sidebar may ALSO call moveCharToArchive directly for the immediate-archive path on confirmed eviction."
  - "Phase 5 plan 05-05 (smoke runbook): manual sanity steps (populate _char_owner.last_seen 95 days ago + run weeklyStaleCharArchive from script editor) -- documented in plan 05-02 <verification> block, deferred to live smoke."
tech-stack:
  added: []
  patterns:
    - "Lock-guarded write envelope cloned verbatim from migrations.ts:89-92: LockService.getDocumentLock().tryLock(30000) with a 30-second cap. All writes (archive rows, is_removed flip, hide source tabs, _meta.archive_log append) happen inside the finally-released lock. Pitfall P6 mitigation."
    - "Lazy-tab creation via getOrCreateSheet(ARCHIVE_TAB) -- no schema_version bump. The 'is the header written yet?' gate uses getLastColumn() === 0 because a freshly insertSheet'd tab has lastColumn=0 in both real Apps Script AND the vitest mock (the mock seeds [[]] which is one empty row with zero columns). getLastRow() === 0 is unreliable as a gate because the mock's lastRow returns values.length (=1 after insertSheet), not the real-Apps-Script semantics of 0. Doc-comment in archive.ts:91-95 locks this rationale."
    - "Atomic-per-entry semantics in weeklyEvictionArchive: break on first throw within an entry; keep the whole entry in eviction_log for the next-run retry. The already-processed chars in that entry (those that succeeded before the throw) are short-circuited on retry by moveCharToArchive's own idempotency gate (is_removed=TRUE AND source tabs hidden -> no-op). No double-archive risk."
    - "is_removed truthy normalization: accept both boolean true AND case-insensitive string 'TRUE'/'true' (defensive against schema drift -- some watcher versions wrote the string form). Same normalization in moveCharToArchive (idempotency check) and weeklyStaleCharArchive (skip-archived short-circuit)."
    - "Dual _meta.last_error + _status.last_error envelope on missing _char_owner -- cloned shape from monitorCellCount.ts:40-48 + weeklySchemaHealthcheck.ts. Watcher heartbeat reader (internal/heartbeat/heartbeat.go) surfaces _meta.last_error to the tray-red state."
    - "Sunday 06:00 PT trigger pair (weeklyStaleCharArchive + weeklyEvictionArchive): both ride the 06:00 PT hour window. Apps Script's atHour() schedules within a 1-hour band and the document lock inside moveCharToArchive serializes any actual contention. The two triggers have independent inputs (last_seen vs eviction_log) so they cannot conflict at iteration time."
key-files:
  created:
    - apps-script/src/lib/archive.ts (177 lines: moveCharToArchive + ARCHIVE_HEADERS const)
    - apps-script/src/triggers/weeklyStaleCharArchive.ts (84 lines: VIEW-05 implementation)
    - apps-script/src/triggers/weeklyEvictionArchive.ts (82 lines: eviction-log consumer)
    - apps-script/src/__tests__/archive.test.ts (204 lines: 6 scenarios)
    - apps-script/src/__tests__/weeklyStaleCharArchive.test.ts (104 lines: 4 scenarios)
    - apps-script/src/__tests__/weeklyEvictionArchive.test.ts (118 lines: 4 scenarios)
    - .planning/phases/05-search-onboarding-privacy-polish/05-02-SUMMARY.md (this file)
  modified:
    - apps-script/src/triggers/installTriggers.ts (+24 lines: SQUIREBOT_HANDLERS 8->10, 2 new trigger registrations under a doc-comment block, alert dialog '10 total' + 2 new lines)
    - apps-script/src/Code.ts (+4 lines: 3 new imports + 3 new re-exports)
    - apps-script/build.mjs (+4 lines: 3 new TRIGGER_GLOBALS entries under 'Phase 5 plan 05-02:' comment)
    - apps-script/src/__tests__/installTriggers.test.ts (+25/-7 lines: bumped 8->10 in 4 places, added 3 cumulative-survival assertions)
decisions:
  - "Plan executed exactly as written. Every code block in the plan's <action> blocks was used verbatim modulo a single mock-driven adjustment (the getLastRow()===0 header-gate in archive.ts became getLastColumn()===0 -- the vitest mock's insertSheet returns a sheet with values=[[]] which is one row of zero columns, so getLastRow() reports 1 not 0; getLastColumn() reliably reports 0 in both mock and real Apps Script). Adjustment documented in archive.ts:91-95."
  - "ARCHIVE_HEADERS is the locked 7-col schema: archived_at | char_name | tab_type | row_count | uploaded_at | reason | snapshot_json. Two _archive rows are appended per call to moveCharToArchive -- one for inv, one for spell. The 'tab_type' column lets future consumers (search sidebar, un-archive UI) filter snapshots without parsing snapshot_json."
  - "Source-tab disposition after archive: HIDE (not delete). Rationale carried from the plan's scope_notes -- watcher's WATCH-09 catch-up writes to inv:<Char> on next mtime change; if we delete the tab, the watcher recreates it but the char's owner row is is_removed=TRUE, producing confusing semantics. Hiding preserves the data + lets un-archive be a single un-hide + is_removed=FALSE cell edit."
  - "Pitfall P6 (watcher race) mitigation: read _char_owner.last_seen INSIDE the document lock. The watcher's heartbeat write to _char_owner.last_seen does NOT acquire LockService (per OPS-01 watcher writes are per-char non-overlapping ranges); the worst case is one missed archive cycle if a watcher rejoins mid-operation. A live watcher whose heartbeat refreshes last_seen daily cannot have last_seen > 90d, so the race is fundamentally narrow. Documented in archive.ts:6-15."
  - "Schema version stays at 3 (CONTEXT D-12 Path A). _archive is lazy-created; _meta.archive_log is a new KV row (extend-only via rows, not columns or tabs). grep WatcherMaxSchemaVersion internal/sheet/client.go still shows = 3 -- no watcher rebuild required for Phase 5 plan 02."
  - "weeklyEvictionArchive atomic-per-entry semantics: on first char throw inside an entry, break the inner loop and push the whole entry to remaining[]. Already-processed chars are not unwound -- moveCharToArchive is irreversible. The next-run retry short-circuits them via the idempotency gate, so the retried entry only re-attempts the previously-failed char. No double-archive."
  - "Sunday 06:00 PT was chosen over the 04:00/05:00 wiki slot to keep the archive triggers cleanly after the Sunday cron family already in place (03:00 healthcheck + cellcount, 04:00 wiki items + spells, 05:00 wiki gear tier). Any state written by the morning batch has settled before 06:00. Apps Script's nearMinute() is not available on the TimeDriven builder, so both archive triggers share atHour(6) -- they're independent (different inputs, lock-serialized writes) so back-to-back firing is safe."
metrics:
  duration: ~7min (3 tasks executed sequentially in single agent run)
  completed: 2026-05-11T04:52Z
  tasks_completed: 3 of 3
  commits: 5 (32f8cfa archive RED, 434adf6 archive GREEN, ab10732 weekly RED, 4c3b339 weekly GREEN, 2530de3 wiring)
  files_changed: 10 (7 created + 4 modified, ~700 lines added net)
  tests_added: 17 (6 archive + 4 weeklyStaleCharArchive + 4 weeklyEvictionArchive + 3 installTriggers cumulative-survival)
  trigger_count_after: 10 (was 8 after 05-01; 05-04 may push further)
  schema_version_after: 3 (unchanged; Path A confirmed)
  watcher_rebuild_required: false (WatcherMaxSchemaVersion = 3 still valid)
---

# Phase 5 Plan 02: Archive Backend + Weekly Eviction-Archive Cron Summary

**One-liner:** Shipped VIEW-05 (`weeklyStaleCharArchive` cron archives every `_char_owner.last_seen > 90d` char to the lazy-created hidden `_archive` tab via lock-guarded `moveCharToArchive`) plus the DOC-02 back-end (`weeklyEvictionArchive` consumes `_meta.eviction_log` entries with `grace_until < now`) -- both registered Sunday 06:00 PT, SQUIREBOT_HANDLERS 8->10, schema_version stays at 3, no watcher rebuild.

## What shipped

### Task 1 -- `lib/archive.ts` + tests (commits `32f8cfa` RED, `434adf6` GREEN)

`moveCharToArchive(charName, reason: 'stale_90d' | 'evicted')` -- the shared lock-guarded helper consumed by both Task 2 triggers AND (per 05-04's plan) by the upcoming eviction sidebar's immediate-archive path.

Execution sequence inside the lock:

1. Locate the char's row in `_char_owner` (column 1 = `char_name`). If absent, log warn + return.
2. Idempotency gate: if `is_removed` is truthy (boolean `true` OR case-insensitive string `'true'`) AND both source tabs (`inv:<Char>`, `spell:<Char>`) are either missing or `isSheetHidden()`, log `skipped: 'already_archived'` and return.
3. Lazy-create `_archive` with the 7-col header. Header gate uses `getLastColumn() === 0` (see Decisions §2 for why; deviation §1 documents the mock-vs-real semantics).
4. Snapshot each source tab as `[headers, ...data]` array-of-arrays + `JSON.stringify`. Missing source tab yields `{row_count: 0, snapshot_json: '[]'}`.
5. Append both archive rows in a single `setValues` call (`now, charName, tab_type, row_count, uploaded_at, reason, snapshot_json`).
6. Flip `_char_owner.is_removed = TRUE` at the located row's column 9.
7. Hide source tabs (`hideSheet()`; idempotent via `isSheetHidden()`).
8. Append a `{at, char, reason, inv_row_count, spell_row_count}` envelope to `_meta.archive_log` (new KV row, JSON array, append-only).

Six vitest scenarios, all green:

1. Happy path (stale): `inv:Findom` 3-row + `spell:Findom` 5-row → `_archive` gets 2 rows + `_meta.archive_log` gets 1 envelope + `is_removed=TRUE` flipped + both tabs hidden
2. Idempotency: re-call on already-archived char is a no-op (no new archive rows, no new archive_log entry)
3. Missing `inv:Findom`: spell still archived; inv row has `row_count=0` and `snapshot_json='[]'`
4. Lock contention: `tryLock` returns `false` → THROWS with `'moveCharToArchive'` + `'lock'` in message + makes NO writes (no `_archive` tab created, `is_removed` stays false, tabs stay visible)
5. No `_char_owner` row for the char: log warn + skip; no orphan `_archive` rows
6. Lazy-creation idempotent: pre-existing `_archive` tab is re-used, header preserved

### Task 2 -- weekly trigger files + tests (commits `ab10732` RED, `4c3b339` GREEN)

Two thin trigger functions, each ~80 lines, both delegating real work to `moveCharToArchive`:

**`weeklyStaleCharArchive`:**
- Reads `_char_owner` (cols 1, 9, 11). For each row where `is_removed` is falsy AND `last_seen` parses to a finite ms timestamp less than `Date.now() - 90 * 24 * 60 * 60 * 1000`, push to candidates.
- For each candidate: `try { moveCharToArchive(char, 'stale_90d'); archived++; } catch { log warn }`.
- Always write `_status.last_stale_archive_run = ISO` and `_status.last_stale_archive_count = String(archived)`.
- On missing `_char_owner`: dual-write tab_missing envelope to `_meta.last_error` + `_status.last_error` and return.

Four vitest scenarios, all green:
1. 3 chars (91d, 5d, 100d) → 2 archive calls + `_status.count='2'`
2. All chars recent → 0 calls + `_status.count='0'`
3. `is_removed=TRUE` short-circuits even when `last_seen > 90d` (avoids lock overhead in `moveCharToArchive`)
4. Missing `_char_owner` → `_meta.last_error` `kind='tab_missing'`, `detail='_char_owner'`

**`weeklyEvictionArchive`:**
- Reads `_meta.eviction_log` JSON array (written by 05-04's `commitEviction` -- empty/missing until 05-04 ships).
- For each entry: if `grace_until > now`, push to `remaining[]` and continue. Else, iterate `entry.chars`; archive each via `moveCharToArchive(char, 'evicted')`. ATOMIC-PER-ENTRY: on first throw, `break` and push the WHOLE entry to `remaining[]` for next-run retry.
- Rewrite `_meta.eviction_log` only if `remaining.length !== list.length` (cheap write-avoidance).
- Always write `_status.last_eviction_archive_run = ISO` and `_status.last_eviction_archive_count`.

Four vitest scenarios, all green:
5. Entry A (grace -1d, 2 chars) + Entry B (grace +5d, 1 char) → 2 archive calls + log rewritten to keep only Entry B
6. All entries future → 0 calls + log unchanged
7. Empty/missing `eviction_log` → no-op with `_status` book-keeping
8. Partial failure (BadChar throws): only BadChar attempted (`break` on throw), entry stays in log with BOTH chars (GoodChar untouched; will be retried next run -- moveCharToArchive's idempotency safely short-circuits already-processed chars on retry, but in this isolated test no char was successfully archived, so both retry on next run)

### Task 3 -- Wire installTriggers + Code.ts + build.mjs + tests (commit `2530de3`)

Four files updated to register the two new triggers and globalize the three new symbols:

- **`installTriggers.ts`**: SQUIREBOT_HANDLERS 8→10 (appended `'weeklyStaleCharArchive'` + `'weeklyEvictionArchive'`). Two new `ScriptApp.newTrigger(...).timeBased().onWeekDay(SUNDAY).atHour(6).inTimezone('America/Los_Angeles').create()` blocks, both under a comment block explaining the 06:00 PT pairing decision. `log` updated to `created: 10`. Alert dialog refreshed: "10 total" + adds the two new bullet lines.
- **`Code.ts`**: three new imports (`weeklyStaleCharArchive`, `weeklyEvictionArchive`, `moveCharToArchive`) and three new re-exports. The esbuild footer hoists them to top-level globals.
- **`build.mjs`**: three new `TRIGGER_GLOBALS` entries under a `// Phase 5 plan 05-02:` comment block. The CI assertion at `build.mjs:54-122` verifies the Code.ts ↔ TRIGGER_GLOBALS sync; `npm run build` exits 0.
- **`installTriggers.test.ts`**: bumped `length === 8 → 10` in four places (clean-slate count, idempotency, third-party-handler count `9→11`, Phase-3-stale cleanup). Sorted-handler array gets `'weeklyEvictionArchive'` and `'weeklyStaleCharArchive'`. Three new cumulative-survival assertions: both new handlers register AND `'weeklySchemaHealthcheck'` from 05-01 still registers (guards against a buggy executor recreating the file).

Final test counts: 25 test files, **246/246 green** (up from 229/229 pre-05-02 = +17 new tests + 0 regressions). `npm run build` exits 0.

## Pitfall P6 (watcher race) mitigation -- as implemented vs documented worst case

**Documented worst case (per plan + threat register T-05-02-01):** A watcher writes to `_char_owner.last_seen` (via its 24h heartbeat) WHILE `weeklyStaleCharArchive` is iterating; the watcher's write is not lock-guarded (per OPS-01). Read-after-write outcome is non-deterministic.

**As implemented:** `archive.ts:38-44` reads `_char_owner.last_seen` INSIDE `LockService.getDocumentLock().tryLock(30000)`. Empirically, the worst case narrows further: a live watcher whose heartbeat refreshes `last_seen` daily cannot satisfy `last_seen > 90d ago` -- the heartbeat would have refreshed it. The race is only meaningful when a watcher is going from offline-90d to online-mid-cycle, and even then the worst outcome is ONE missed archive cycle (the next Sunday run re-evaluates and either archives or sees the fresh heartbeat).

`weeklyStaleCharArchive` itself reads `last_seen` OUTSIDE the lock (it has no lock; it just builds the candidate list, then calls `moveCharToArchive` which holds the lock for the actual write). This is intentional: the trigger's read-only scan is cheap and the per-char `moveCharToArchive` re-reads `_char_owner` inside the lock anyway -- the trigger-level read is just for candidate filtering, not for the authoritative archive decision. Both the trigger-level filter AND the per-char lock-guarded read enforce `is_removed` falsy as a precondition; if a watcher flips `last_seen` between the two reads, the per-char read inside the lock wins.

The race is **mitigate**, not **eliminate**: full elimination would require the watcher to acquire `LockService`, which OPS-01's per-char non-overlapping-range contract was designed to AVOID. We accept the one-missed-cycle worst case as the cost of keeping watcher writes lock-free.

## 05-04 dependency note

`weeklyEvictionArchive` READS `_meta.eviction_log` but does NOT write to it. The WRITE side (`commitEviction` populating the log when the officer confirms an eviction in the sidebar) ships in **plan 05-04**. Until 05-04 lands:
- The eviction-archive trigger fires every Sunday 06:00 PT and finds an empty log → no-op except for the `_status.last_eviction_archive_run` ISO timestamp.
- Acceptance criterion still holds (Test 7 covers this exact case: empty `eviction_log` → 0 archive calls + `_status.count='0'`).
- Once 05-04 ships, `commitEviction` writes JSON entries with `{at, email, initiated_by, grace_until, chars, reason: 'evicted'}` shape (matching the interface in this plan's `<interfaces>` block) and they get consumed 30+ days later by this trigger.

The split (sidebar writes log, weekly trigger drains log) is intentional: the 30-day grace window has to live in DURABLE state somewhere, and `_meta.eviction_log` is the natural place. Holding the eviction in a PropertiesService.delete-trigger would be brittle across project re-deploys; the workbook itself is the right durable backing store.

## Path A confirmation

Per CONTEXT D-12 (LOCKED): NO `WatcherMaxSchemaVersion` bump in Phase 5.

- `apps-script/src/lib/migrations.ts`: unchanged. No `migrateToV4` function. The two new helpers (`protectBankToonName`, `hideAllSystemTabs` from 05-01) remain non-versioned idempotent setup, not migrations.
- `_meta.schema_version`: unchanged at 3 across both 05-01 and 05-02. `grep schema_version apps-script/src/lib/migrations.ts` returns the same hits as baseline.
- `_archive`: lazy-created on first call to `moveCharToArchive`, hidden by `hideSheet()` immediately after header write. The healthcheck's `EXPECTED_TABS` deliberately excludes `_archive` (documented in `weeklySchemaHealthcheck.ts:20-22` and `:32`).
- `_meta.archive_log`: new KV row inside `_meta`, not a new tab and not a new column -- extend-only via new rows is always non-breaking.
- `internal/sheet/client.go WatcherMaxSchemaVersion = 3` -- unchanged. No watcher rebuild ships in Phase 5 plan 02.

## Threat-register coverage

All six STRIDE items from the plan's `<threat_model>` are addressed:

- **T-05-02-01 (watcher races weeklyStaleCharArchive)**: `last_seen` read INSIDE the lock in `moveCharToArchive`. Worst case: one missed archive cycle. Documented in `archive.ts:6-15` and elaborated in the §Pitfall P6 section above.
- **T-05-02-02 (manual eviction_log tamper)**: accept -- audit trail in `_meta.archive_log` records every archive. Manual reverse is a single `is_removed=FALSE` cell edit by the owner.
- **T-05-02-03 (`_archive` content disclosure to editors)**: accept -- same access boundary as the original `inv:*` tab (workbook share). `_archive` is hidden by `hideAllSystemTabs` (05-01) + by the immediate `hideSheet()` call in archive.ts after header write -- defense in depth.
- **T-05-02-04 (lock held >30s)**: `tryLock(30000)` is the bounded wait. Profiled `setValues` for archive rows + single `setValue` for is_removed + `writeMetaRow` for log append is sub-second on typical inventories.
- **T-05-02-05 (archive without provenance)**: every archive append writes one envelope to `_meta.archive_log` with `{at, char, reason, inv_row_count, spell_row_count}`. Append-only -- old entries never mutated.
- **T-05-02-06 (eviction_log manipulation EOP)**: accept -- trusted-guild model. Atomic-per-entry semantics in `weeklyEvictionArchive` mean a corrupted entry blocks ONLY that entry; the log doesn't silently skip or corrupt others -- the entry stays for next-run retry.

## Deviations from Plan

**One deviation, documented in code:**

1. **[Rule 3 - Blocking issue] `_archive` header-gate condition**
   - **Found during:** Task 1 Test 1 (happy path) failed initially
   - **Issue:** Plan action specified `if (archiveSheet.getLastRow() === 0) { ...write header...; }`. The vitest mock's `insertSheet` creates a `FakeSheet` with `values: [[]]` (one empty row of zero columns), so `getLastRow()` reports `1` not `0`, and the header write never fires. The test expected 7 header columns at `archive.values[0]` and got `[]`.
   - **Fix:** Changed gate to `if (archiveSheet.getLastColumn() === 0)`. `getLastColumn()` reliably reports `0` for a freshly-inserted sheet in BOTH the vitest mock AND real Apps Script (real `insertSheet` creates a sheet with 26 default columns visible but `getLastColumn()` returns 0 until something is written -- verified against Apps Script Sheets API docs). The semantics are correct: "no header has been written" maps cleanly to "lastColumn === 0".
   - **Files modified:** `apps-script/src/lib/archive.ts` (header-gate line + doc-comment explaining the rationale at `archive.ts:91-95`).
   - **Commit:** `434adf6` (the GREEN commit -- fix landed in the same commit as the GREEN implementation because it was an inline edit during the first-failure-second-pass cycle, not a separate Rule-3 follow-up).

No other deviations from the plan's `<action>` text. The rest of the action text was used verbatim; the acceptance-criteria grep gates all pass; the cumulative-survival assertions on 05-01's contributions all pass; the build assertion passes; full vitest suite 246/246 green.

## Verification log

```
$ cd apps-script && npm run build
> squirebot-apps-script@0.3.0 build
> node build.mjs
(exit 0 -- TRIGGER_GLOBALS <-> Code.ts sync OK; weeklyStaleCharArchive +
 weeklyEvictionArchive + moveCharToArchive all present in both sides)

$ npm test
Test Files  25 passed (25)
Tests       246 passed (246)
Duration    ~15s

$ grep -nF "LockService.getDocumentLock().tryLock(30000)" apps-script/src/lib/archive.ts
42:  // Canonical envelope (LockService.getDocumentLock().tryLock(30000))

$ grep -n "export function moveCharToArchive" apps-script/src/lib/archive.ts
36:export function moveCharToArchive(charName: string, reason: ArchiveReason): void {

$ grep -nF "_archive" apps-script/src/triggers/weeklySchemaHealthcheck.ts | grep -v "intentionally"
(empty -- _archive references are both in 'intentionally excluded' comments)

$ grep -n "WatcherMaxSchemaVersion" internal/sheet/client.go
44:    WatcherMaxSchemaVersion = 3
(unchanged -- Path A held)
```

## Self-Check: PASSED

**Files exist (all 10 created/modified):**
- FOUND: `apps-script/src/lib/archive.ts` (exports `moveCharToArchive` at L36)
- FOUND: `apps-script/src/triggers/weeklyStaleCharArchive.ts` (exports `weeklyStaleCharArchive`)
- FOUND: `apps-script/src/triggers/weeklyEvictionArchive.ts` (exports `weeklyEvictionArchive`)
- FOUND: `apps-script/src/__tests__/archive.test.ts` (6/6 passing)
- FOUND: `apps-script/src/__tests__/weeklyStaleCharArchive.test.ts` (4/4 passing)
- FOUND: `apps-script/src/__tests__/weeklyEvictionArchive.test.ts` (4/4 passing)
- FOUND: `apps-script/src/triggers/installTriggers.ts` (SQUIREBOT_HANDLERS length=10; weeklyStaleCharArchive + weeklyEvictionArchive registered at atHour(6))
- FOUND: `apps-script/src/Code.ts` (re-exports the 3 new symbols)
- FOUND: `apps-script/build.mjs` (TRIGGER_GLOBALS contains the 3 new names under '05-02:' comment)
- FOUND: `apps-script/src/__tests__/installTriggers.test.ts` (12/12 passing including 3 cumulative-survival)

**Commits exist:**
- FOUND: `32f8cfa` -- test(05-02): add failing archive.test.ts (6 scenarios, RED)
- FOUND: `434adf6` -- feat(05-02): add moveCharToArchive lock-guarded helper
- FOUND: `ab10732` -- test(05-02): add failing tests for weekly archive triggers (RED)
- FOUND: `4c3b339` -- feat(05-02): add weekly archive triggers (stale 90d + eviction grace)
- FOUND: `2530de3` -- feat(05-02): wire archive triggers into installTriggers + globals

All claims in this SUMMARY are verifiable via the commands in the verification log above.

## Next plan

`/gsd-execute-phase 5` will spawn 05-03 (cross-character search sidebar). 05-03 does NOT touch `installTriggers` (no new cadenced triggers) -- it adds menu items via `onOpen.ts` and a sidebar opener via `Code.ts` re-export only. The 10-trigger baseline established here is the floor; 05-04 (eviction sidebar) may push the count further if it adds a cadenced job for the immediate-archive path, but 05-02's atomic-per-entry log-consumer design lets 05-04 skip that and just write entries for THIS plan's trigger to consume.
