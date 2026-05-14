---
phase: 05-search-onboarding-privacy-polish
plan: 01
subsystem: apps-script-housekeeping-and-healthcheck
tags: [apps-script, housekeeping, schema-healthcheck, ops-06, range-protect, system-tab-hide]
requires:
  - 04-04 (protectBankCoinCells, monitorCellCount, installTriggers 7-trigger baseline)
  - 03 (sheet-helpers.ts: readMetaRows / writeMetaRow / getActiveSpreadsheet)
provides:
  - "OPS-06: weekly Sun-03:00 PT schema healthcheck with tab-by-id verification + structured _meta.last_error envelope on missing-tab"
  - "protectBankToonName: warning-only Range.protect on _meta.bank_toon_name (clones protectBankCoinCells shape for a single cell)"
  - "hideAllSystemTabs: idempotent _-prefixed tab hide called from installTriggers"
  - "_meta.expected_sheet_ids: new lazy-backfilled KV row (sheet-id source of truth, resilient to user renames per Pitfall P7)"
  - "8-trigger SQUIREBOT_HANDLERS baseline (was 7); 05-02 + 05-04 extend further"
affects:
  - "Phase 5 plan 05-02 (archive lib extends migrations.ts + SQUIREBOT_HANDLERS array landed here)"
  - "Phase 5 plan 05-04 (eviction sidebar extends SQUIREBOT_HANDLERS + Code.ts re-exports)"
  - "Phase 5 plan 05-05 (/troubleshooting copy references _meta.last_error tab_missing kind written here)"
tech-stack:
  added: []
  patterns:
    - "Tab-by-ID verification (vs. tab-by-name): healthcheck stores {tab_name -> getSheetId()} as the source of truth in _meta.expected_sheet_ids; user renames are NOT false positives because sheet-ID survives renames. Only deletion or wholesale replacement (which mints a new sheet-ID) trips the alarm. Locked by RESEARCH §Pitfall P7."
    - "Lazy backfill of new _meta KV rows (vs. schema_version bump): expected_sheet_ids is added at first healthcheck run on each workbook -- no migration required. CONTEXT D-12 Path A in action: extend-only via new _meta rows keeps WatcherMaxSchemaVersion=3 unchanged, so no watcher rebuild ships in Phase 5."
    - "Dual-write _meta.last_error + _status.last_error envelope (kind/where/at/detail JSON): clone of monitorCellCount.ts:40-48. Watcher heartbeat reads _meta.last_error only, but _status mirror gives a recoverable second copy if _meta itself is deleted (T-05-01-04 in the threat register)."
    - "setWarningOnly(true) is the only Range.protect variant: strict (false) is invisible to the script owner (default editor), so warning-only is the actually-visible deterrent. Phase 4 smoke discovery from migrations.ts:143-148 -- comment copied verbatim into protectBankToonName."
    - "Idempotency-by-description-match on Range.protect: getProtections(SpreadsheetApp.ProtectionType.RANGE) filtered by (A1 + description) -- no setProtected calls, no Sheets-API direct invocation. Re-running installTriggers is always a no-op for already-protected cells."
key-files:
  created:
    - apps-script/src/triggers/weeklySchemaHealthcheck.ts (82 lines)
    - apps-script/src/__tests__/weeklySchemaHealthcheck.test.ts (121 lines)
    - .planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md (this file)
  modified:
    - apps-script/src/lib/migrations.ts (+72 lines; 2 new exported helpers + 1 const)
    - apps-script/src/triggers/installTriggers.ts (+17/-6 lines; SQUIREBOT_HANDLERS 7->8, new trigger registration, 2 new idempotent setup calls, alert text refreshed)
    - apps-script/src/Code.ts (+5/-2 lines; 3 new imports + re-exports)
    - apps-script/build.mjs (+4 lines; 3 new TRIGGER_GLOBALS entries)
    - apps-script/src/__tests__/test-helpers.ts (+10 lines; makeSheetProxy mocks for hideSheet/isSheetHidden/showSheet/getSheetId)
    - apps-script/src/__tests__/migrations.test.ts (+98 lines; 7 new tests across protectBankToonName + hideAllSystemTabs)
    - apps-script/src/__tests__/installTriggers.test.ts (~30 lines net; bumped 7->8 / 4->5 expectations + 3 new assertions)
decisions:
  - "Plan executed exactly as written -- no deviations from the action text. Every code block in the plan's <action> was used verbatim, every grep acceptance criterion passed, full suite 229/229 green on first run after Task 3 wired."
  - "EXPECTED_TABS is the 13-tab list from .planning/research/ARCHITECTURE.md (9 system + 4 view), with _archive INTENTIONALLY excluded (lazy-created by 05-02's archive lib). The exclusion is documented in two places in weeklySchemaHealthcheck.ts so future contributors do not 'fix' it."
  - "Trigger schedule for weeklySchemaHealthcheck is Sun-03:00 PT, same window as monitorCellCount. Apps Script atHour() schedules within a 1-hour window so they may fire back-to-back; both touch independent KV rows in _meta/_status so there is no contention. installTriggers.ts comment locks this rationale in place."
  - "Alert dialog after installTriggers now says '8 total' and adds a 'Sunday 03:00 PT: weeklySchemaHealthcheck' line. The dialog also reflects bank-toon-name protection and system-tab hide so the operator knows what just happened."
  - "Schema version stays at 3 (CONTEXT D-12 Path A). grep 'writeMetaRow.*schema_version' apps-script/src/lib/migrations.ts returns 2 hits, unchanged from baseline. internal/sheet/client.go WatcherMaxSchemaVersion = 3 untouched -- no watcher rebuild required for Phase 5 plan 01."
  - "test-helpers.ts extension is forward-compatible with 05-02 (archive tests need hideSheet) and 05-03 (search tests will replace the CacheService stub with a Map-backed mock per PATTERNS §test-helpers MOD note). This plan only adds hide/show/getSheetId because that is what 05-01's tests touch."
metrics:
  duration: ~20min (3 tasks executed sequentially in single agent run)
  completed: 2026-05-11T04:34Z
  tasks_completed: 3 of 3
  commits: 3 (c85586b feat helpers, ae57d61 feat trigger, a9562b6 feat wiring)
  files_changed: 9 (2 created + 7 modified, ~330 lines added)
  tests_added: 15 (7 migrations + 5 weeklySchemaHealthcheck + 3 installTriggers)
  trigger_count_after: 8 (was 7; 05-02 will push to 10, 05-04 may push further)
  schema_version_after: 3 (unchanged; Path A confirmed)
  watcher_rebuild_required: false (WatcherMaxSchemaVersion = 3 still valid)
---

# Phase 5 Plan 01: Schema Healthcheck + Tab Hide + Bank-Toon-Name Protection Summary

**One-liner:** Shipped OPS-06 (weekly Sun-03:00 PT schema healthcheck with tab-by-id verification + structured `_meta.last_error` `kind='tab_missing'` envelope) plus two idempotent housekeeping helpers (`protectBankToonName` warning-only Range.protect, `hideAllSystemTabs` _-prefixed sweep) wired into `installTriggers` -- 8 triggers total now, schema_version stays at 3, no watcher rebuild.

## What shipped

### Task 1 -- `migrations.ts` helpers + tests (commit `c85586b`)

Two new exported functions in `apps-script/src/lib/migrations.ts`, both clones of the Phase 4 `protectBankCoinCells` idiom:

- **`protectBankToonName()`**: applies `Range.protect().setWarningOnly(true)` to the single `_meta.bank_toon_name` cell. Description string is the Claude's-Discretion-defaulted copy from the plan: `'Edit only via SquireBot -> Set Bank Coin... (sets the bank-toon name used by the bank view and search).'`. Idempotent via description-string match. Skip-if-row-missing (bank_toon_name is lazy-created by the existing `saveBankCoin` sidebar -- installTriggers re-run after first save will pick it up).
- **`hideAllSystemTabs()`**: iterates `getActiveSpreadsheet().getSheets()`, skips non-`_`-prefixed sheets, skips already-hidden sheets via `isSheetHidden()`, calls `hideSheet()` on the rest. Logs hidden-count. No lock acquired -- `hideSheet()` does not contend with the document write lock.

Both are no-locks helpers per the lib's existing convention (migrations.ts:102-104 comment on `protectBankCoinCells`).

The warning-only rationale comment from `migrations.ts:143-148` was copied verbatim above `cell.protect()...` in `protectBankToonName`.

`test-helpers.ts` was extended (10 lines) with `isSheetHidden`/`hideSheet`/`showSheet`/`getSheetId` on the `makeSheetProxy` factory. `getSheetId` returns a deterministic 32-bit hash of the sheet name (`s.name.split('').reduce((h,c) => h*31 + c.charCodeAt(0), 0) | 0`) so tests can predict the integer keys that `weeklySchemaHealthcheck` writes into `_meta.expected_sheet_ids`.

Seven new test cases in `migrations.test.ts`:
1. `protectBankToonName` happy path (1 protection added, warningOnly=true, description matches the locked string)
2. `protectBankToonName` idempotent (3x run -> still 1 protection)
3. `protectBankToonName` skip-if-row-missing (row absent -> no protections)
4. `protectBankToonName` skip-if-`_meta`-missing (sheet absent -> no throw)
5. `hideAllSystemTabs` happy path (`_meta` + `_char_owner` hidden, `view` + `bank` visible)
6. `hideAllSystemTabs` idempotent (`_meta` already hidden -> second run is a no-op)
7. `hideAllSystemTabs` no-op when no system tabs (only `view`/`bank`/`gear_check` seeded -> no hides)

`npm test -- migrations` -> 24/24 green (17 existing + 7 new).

### Task 2 -- `weeklySchemaHealthcheck` trigger + tests (commit `ae57d61`)

New file `apps-script/src/triggers/weeklySchemaHealthcheck.ts` (82 lines). Two-phase logic:

1. **First-run backfill**: when `_meta.expected_sheet_ids` row is absent, build the `{tab_name -> getSheetId()}` map for every name in the 13-entry EXPECTED_TABS constant that currently exists in the workbook, then `writeMetaRow('_meta', 'expected_sheet_ids', JSON.stringify(map))`. Logs `backfilled: N`.
2. **Verification**: iterate EXPECTED_TABS; for each name, look up its expected sheet-ID and check `sheetsById.has(id)`. Tabs whose ID is missing OR not yet recorded in expected_sheet_ids land in the `missing[]` array.

On `missing.length === 0`: writes `_status.last_schema_check` (ISO timestamp) + `_status.last_schema_check_status = 'ok'`. Logs `{ok: true, checked: 13}`.

On any missing tab: builds the canonical `{at, where:'weeklySchemaHealthcheck', kind:'tab_missing', detail:<comma-sep names>}` envelope, dual-writes to `_meta.last_error` AND `_status.last_error` (same JSON). Logs `{missing: [...]}` at warn level. The watcher heartbeat reader (`internal/heartbeat/heartbeat.go`) reads `_meta.last_error` and surfaces it to the tray-red state on the next 60s heartbeat write.

`_archive` is INTENTIONALLY excluded from EXPECTED_TABS (documented in two adjacent comments) -- it is lazy-created by `archive.ts` in plan 05-02, so workbooks that have not yet seen an eviction must not throw a false-positive on it.

Five new vitest cases in `weeklySchemaHealthcheck.test.ts`:
1. First-run backfill writes `_meta.expected_sheet_ids` with all 13 entries + `_status.last_schema_check_status = 'ok'`
2. Steady-state second run keeps `last_schema_check_status = 'ok'`, refreshes timestamp, does not write `last_error`
3. Single missing tab: `_meta.last_error` JSON has `kind=tab_missing`, `where=weeklySchemaHealthcheck`, `detail='_pigparse'`, `at` matches ISO format; `_status.last_error` mirrors
4. Multiple missing tabs: `detail` is comma-separated (`_pigparse,_quest_items`)
5. `_archive` is NOT in the backfilled map; clean run still reports `status = 'ok'`

`npm test -- weeklySchemaHealthcheck` -> 5/5 green.

### Task 3 -- Wire installTriggers + Code.ts + build.mjs + tests (commit `a9562b6`)

Four files updated to register the new trigger and globalize the new symbols:

- **`installTriggers.ts`**: SQUIREBOT_HANDLERS extended 7 -> 8 (appended `'weeklySchemaHealthcheck'`). New `ScriptApp.newTrigger('weeklySchemaHealthcheck')...atHour(3).inTimezone('America/Los_Angeles')...create()` block mirrors the existing `monitorCellCount` registration. After `protectBankCoinCells()` at the existing call site: `protectBankToonName(); hideAllSystemTabs();`. `log('info', 'installTriggers', { deleted, created: 8 })`. UI alert dialog refreshed to say '8 total' + adds the new healthcheck line + mentions bank-toon-name protect + system-tab hide.
- **`Code.ts`**: new imports `protectBankToonName, hideAllSystemTabs` from `./lib/migrations` and `weeklySchemaHealthcheck` from `./triggers/weeklySchemaHealthcheck`. Re-exports extended to include all three. The esbuild footer lifts them to top-level globals.
- **`build.mjs`**: three new TRIGGER_GLOBALS entries (`weeklySchemaHealthcheck`, `protectBankToonName`, `hideAllSystemTabs`) under a `// Phase 5 plan 05-01:` comment. The CI assertion at `build.mjs:54-122` verifies Code.ts <-> TRIGGER_GLOBALS sync; `npm run build` exits 0.
- **`installTriggers.test.ts`**: existing assertions bumped `length === 7 -> 8`, sorted-handler array gets `'weeklySchemaHealthcheck'`, third-party-handler test bumped `8 -> 9`, protection count `4 -> 5` (added `bank_toon_name` row to the seeded `_meta`). Three new tests: weeklySchemaHealthcheck is registered, bank-toon-name protection has the locked description string + warningOnly=true, all `_`-prefixed tabs are hidden after install (with `view` staying visible).

`npm run build` exits 0. `npm test` (full suite) -> 22 test files, 229/229 tests green.

`onOpen.ts` was NOT modified -- per the plan, the 'Search...' and 'Evict Guildie...' menu items are 05-03 and 05-04 work. `git diff apps-script/src/triggers/onOpen.ts` is empty.

## Threat-register coverage

All six STRIDE items from the plan's `<threat_model>` are addressed:

- **T-05-01-01 (unhide `_meta`)**: `hideAllSystemTabs()` is idempotent and runs on every install. A guildie can unhide via right-click; the next install re-hides. Documented in the 05-05 troubleshooting writeup as a known UX gap.
- **T-05-01-02 (edit `bank_toon_name`)**: `Range.protect().setWarningOnly(true)` with the locked description string. Prompt fires; cannot enforce strict (script owner is default editor of strict protections per Phase 4 smoke).
- **T-05-01-03 (rename tab)**: healthcheck uses `getSheetId()` not name -- renames are NOT false-positives.
- **T-05-01-04 (delete `_meta`)**: dual-write to `_status.last_error` mitigates partial loss. Both deleted = unrecoverable, restore via Sheets version history (covered in 05-05).
- **T-05-01-05 (sheet IDs in `_meta.expected_sheet_ids`)**: accepted; sheet IDs are not secrets.
- **T-05-01-06 (any editor can re-run installTriggers)**: accepted; trusted-guild model.

## Deviations from Plan

None. Plan executed exactly as written.

The three commit messages, the acceptance-criteria greps, the test counts (7 / 5 / 3 new across the three suites), and the file-change inventory all match the plan's `<output>` block.

## Schema impact

**Path A confirmed.** Per CONTEXT D-12:

- `apps-script/src/lib/migrations.ts` has no new `migrateToVN` function -- the two new helpers are non-versioned idempotent setup, not migrations.
- `_meta.expected_sheet_ids` is a new KV row inside `_meta`, not a new tab and not a new column -- extend-only via new rows is always non-breaking.
- `grep -c "schema_version" apps-script/src/lib/migrations.ts` == 6, unchanged from baseline.
- `internal/sheet/client.go WatcherMaxSchemaVersion = 3` -- unchanged. No watcher rebuild ships in Phase 5 plan 01.

## Verification log

```
$ cd apps-script && npm run build
> squirebot-apps-script@0.3.0 build
> node build.mjs
(exit 0 -- TRIGGER_GLOBALS <-> Code.ts sync OK)

$ npm test
Test Files  22 passed (22)
Tests       229 passed (229)
Duration    ~11s

$ grep -n "export function protectBankToonName" apps-script/src/lib/migrations.ts
172:export function protectBankToonName(): void {

$ grep -n "export function hideAllSystemTabs" apps-script/src/lib/migrations.ts
213:export function hideAllSystemTabs(): void {

$ grep -c "setProtected" apps-script/src/lib/migrations.ts
0  (forbidden API not used)

$ git diff apps-script/src/triggers/onOpen.ts
(empty -- onOpen unchanged in this plan)
```

## Self-Check: PASSED

**Files exist (all 9 changed):**
- FOUND: `apps-script/src/lib/migrations.ts` (with `protectBankToonName` at L172 and `hideAllSystemTabs` at L213)
- FOUND: `apps-script/src/triggers/installTriggers.ts` (with `weeklySchemaHealthcheck` registration + protectBankToonName + hideAllSystemTabs calls)
- FOUND: `apps-script/src/triggers/weeklySchemaHealthcheck.ts` (new file, exports `weeklySchemaHealthcheck`)
- FOUND: `apps-script/src/Code.ts` (re-exports the 3 new symbols)
- FOUND: `apps-script/build.mjs` (TRIGGER_GLOBALS contains the 3 new names)
- FOUND: `apps-script/src/__tests__/test-helpers.ts` (makeSheetProxy has `isSheetHidden`/`hideSheet`/`showSheet`/`getSheetId`)
- FOUND: `apps-script/src/__tests__/migrations.test.ts` (24/24 passing including 7 new)
- FOUND: `apps-script/src/__tests__/weeklySchemaHealthcheck.test.ts` (5/5 passing)
- FOUND: `apps-script/src/__tests__/installTriggers.test.ts` (9/9 passing including 3 new)

**Commits exist:**
- FOUND: `c85586b` -- feat(05-01): add protectBankToonName + hideAllSystemTabs helpers
- FOUND: `ae57d61` -- feat(05-01): add weeklySchemaHealthcheck trigger (OPS-06)
- FOUND: `a9562b6` -- feat(05-01): wire weeklySchemaHealthcheck + new helpers into installTriggers

All claims in this SUMMARY are verifiable via the commands in the verification log above.

## Next plan

`/gsd-execute-phase 5` will spawn 05-02 (archive lib + weeklyStaleCharArchive + weeklyEvictionArchive). 05-02 extends the same `SQUIREBOT_HANDLERS` array landed here (8 -> 10) and adds `_archive` lazily (no schema bump). The pattern map at `05-PATTERNS.md` §archive.ts has the analogs.
