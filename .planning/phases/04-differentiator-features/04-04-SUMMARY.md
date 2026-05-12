---
phase: 04-differentiator-features
plan: 04
subsystem: apps-script-bank-coin-sidebar-and-install-triggers-expansion
tags: [apps-script, bank-coin, bank-01, ops-07, install-triggers, range-protect, monitor-cell-count]
requires:
  - 04-01 (HtmlService sidebar pattern from showCharInfoSidebar; migrations.ts with migrateToV3 post-success hook)
  - 04-02 (refreshWikiSpells time-driven trigger registration target)
  - 04-03 (refreshWikiGearTier time-driven trigger registration target)
  - 03-04 (installTriggers 4-trigger baseline to extend; buildBank to modify for coin-row prepend)
provides:
  - "BANK-01/02/03/04: bank coin sidebar — 320px HtmlService 4-input form (PP/GP/SP/CP); pre-populated from _meta.bank_coin_* rows; lock-guarded saveBankCoin writes + fires buildBank"
  - "OPS-07: monitorCellCount weekly Sun 03:30 PT — sums getLastRow × getLastColumn across all sheets; writes _status.cell_count; if > 5M writes {kind: 'cell_count_threshold', detail: 'X/10M (top: a, b, c)'} envelope to _meta.last_error"
  - "bank tab coin row at row 2: [bank_toon, 'COIN', 'Platinum: pp | Gold: gp | Silver: sp | Copper: cp', '', '', '', '', last_updated_iso]. Inventory data shifts to row 3."
  - "Range.protect on _meta.bank_coin_pp/gp/sp/cp cells: prevents direct edits; users must go through Set Bank Coin sidebar"
  - "installTriggers expanded 4 → 7: existing 4 + refreshWikiSpells (Sun 04:00 PT) + refreshWikiGearTier (Sun 05:00 PT) + monitorCellCount (Sun 03:00 PT). All time slots offset to avoid lock contention"
  - "SquireBot menu: 'Set Bank Coin…' between 'Set Character Info…' and 'Set Theme…'"
  - "Phase 4 CODE-COMPLETE → SHIPPED as v0.4.0 (2026-05-11)"
affects:
  - "Phase 5 onwards: builds on the 7-trigger baseline; 05-01/05-02/05-03/05-04 add 5 more handlers; 05-05 ships v1.0.0"
tech-stack:
  added: []
  patterns:
    - "Lazy _meta row creation via writeMetaRow (write-if-absent semantics): bank_coin_last_updated row appended on first saveBankCoin call without scaffold/schema bump. Avoids overkill v=4 migration for a single timestamp."
    - "Range.protect with description gate for idempotency: skip protect call if existing protection has the canonical description. Apps Script's protect() is idempotent on same range but description matching makes the no-op cheap + explicit."
    - "monitorCellCount top-5 reporting: sort perSheet by cells desc, slice(0, 5), comma-join — gives officers actionable insight on which sheets are bloating."
    - "Time-driven trigger offsets: refreshPigparse 03:00, monitorCellCount 03:00 Sun (different cadence — daily vs weekly), refreshWikiItems Sun 04:00, refreshWikiSpells Sun 04:00 (in same atHour() window, independent sheets), refreshWikiGearTier Sun 05:00. Apps Script's atHour() schedules within 1h window — relying on per-trigger locks to handle concurrent runs."
    - "buildBank coin row prepend: coin row at row 2 + inventory at row 3+. Single bankSheet.getRange(2,1,1,8).setValues + bankSheet.getRange(3,1,N,8).setValues. Two RPC calls instead of one but explicit row separation."
key-files:
  created:
    - apps-script/src/triggers/showBankCoinSidebar.ts (~180 lines; sidebar opener + getBankCoinForForm + saveBankCoin lock-guarded + inline HTML)
    - apps-script/src/triggers/monitorCellCount.ts (~80 lines; per-sheet getLastRow × getLastColumn sum + top-5 envelope + threshold trip)
    - apps-script/src/__tests__/bankCoinSidebar.test.ts (~140 lines; 7 vitest scenarios)
    - apps-script/src/__tests__/monitorCellCount.test.ts (~110 lines; 4 vitest scenarios)
  modified:
    - apps-script/src/tabs/buildBank.ts (+50 lines; composeCoinRow helper + coin row at row 2 + inventory shift to row 3+)
    - apps-script/src/lib/migrations.ts (+40 lines; protectBankCoinCells helper + wired into migrateToV3 post-success)
    - apps-script/src/triggers/installTriggers.ts (+30 lines; SQUIREBOT_HANDLERS list extended 4 → 7; new ScriptApp.newTrigger calls for refreshWikiSpells/refreshWikiGearTier/monitorCellCount; defensive protectBankCoinCells re-apply)
    - apps-script/src/triggers/onOpen.ts (+1 line; addItem 'Set Bank Coin…' between Set Character Info… and Set Theme…)
    - apps-script/src/Code.ts (+5 lines; export showBankCoinSidebar/getBankCoinForForm/saveBankCoin/monitorCellCount/protectBankCoinCells)
    - apps-script/build.mjs (+5 lines; add 5 new TRIGGER_GLOBALS entries)
    - docs/apps-script-deploy.md (+30 lines; Phase 4 deploy steps + Range.protect smoke check + schema-version coordination table v0.1.x–v0.2.x=1, v0.3.x=2, v0.4.0+=3)
decisions:
  - "bank_coin_last_updated row created lazily by writeMetaRow (no scaffold bump, no v=4 migration): same pattern as Phase 3's theme/contact_email rows. Avoids overkill schema bump for a single timestamp."
  - "Range.protect on bank_coin cells called from BOTH migrateToV3 success path AND installTriggers (defensive re-apply): covers the case where a workbook was migrated before this code shipped. Idempotent via existing-protection description check."
  - "monitorCellCount threshold 5M (50% of Google's 10M hard cap): leaves headroom to act before workbook breaks. Top-5 sheet reporting tells officers WHERE the bloat is."
  - "Apps Script atHour() precision is 1-hour window only (no atMinute): two triggers atHour(4) — refreshWikiItems + refreshWikiSpells — may fire concurrently. Rely on per-trigger document lock + independent _item_master vs _wiki_spells writes (no lock contention). refreshWikiGearTier offset to atHour(5) for extra headroom."
  - "Bank coin Range.protect setWarningOnly = false: hard lock, not warning-only. (Live-smoke fix-pack 3c5ea6d later set warningOnly=true for the in-game-coin-tracking UX — users need to be able to update fast without confirmation modal interruptions; documented in fix-pack commits.)"
  - "Phase 4 CODE-COMPLETE here: live smoke against real workbook runs after this plan ships → v0.4.0 release."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-10T18:48:29-05:00
  tasks_completed: 7 of 7
  commits: 8 (3751123 feat showBankCoinSidebar + form callbacks + tests; dfe4102 feat buildBank coin row prepend + tests; 3babef6 feat protectBankCoinCells + wired into migrateToV3; 50714b7 feat monitorCellCount weekly trigger + tests; 14878c3 feat installTriggers 4 -> 7 + bank-coin protect re-apply; 6bca772 chore wire bank-coin menu + Code.ts + TRIGGER_GLOBALS; df636a4 docs apps-script-deploy Phase 4 + Range.protect smoke check; c0cff2d chore STATE.md -> Phase 4 CODE-COMPLETE)
  files_changed: 11 (4 created + 7 modified, ~700 lines added)
  tests_added: 11 (7 bankCoinSidebar + 4 monitorCellCount)
  trigger_count_after: 7 (onChange + 1h buildView backstop + daily refreshPigparse + Sun refreshWikiItems + Sun refreshWikiSpells + Sun refreshWikiGearTier + Sun monitorCellCount)
  schema_version_after: 3 (unchanged from 04-01)
  watcher_rebuild_required: false (apps-script-only; v0.4.0 watcher already shipped with WatcherMaxSchemaVersion=3 from 04-01)
---

# Phase 4 Plan 04: Bank Coin Sidebar + installTriggers Expansion + Phase 4 CODE-COMPLETE Summary

**One-liner:** Shipped the Phase 4 wrap — bank coin sidebar (BANK-01..04: 320px HtmlService 4-input form for PP/GP/SP/CP pre-populated from `_meta.bank_coin_*` rows with lock-guarded `saveBankCoin` + immediate `buildBank` refresh), cell-count monitoring (OPS-07: weekly Sun 03:00 PT sums `getLastRow × getLastColumn` across all sheets, trips warning envelope at 5M of 10M hard cap with top-5 sheet attribution), `Range.protect` on `_meta.bank_coin_*` cells (wired into both migrateToV3 post-success and installTriggers defensive re-apply), `buildBank` coin row prepend at row 2 (inventory shifts to row 3+), and the trigger inventory expansion 4 → 7 (adds refreshWikiSpells Sun 04:00 + refreshWikiGearTier Sun 05:00 + monitorCellCount Sun 03:00) — making Phase 4 CODE-COMPLETE and ready for the live smoke that ships v0.4.0.

## What shipped

### Task 1 — showBankCoinSidebar + form callbacks (commit `3751123`)

`showBankCoinSidebar()`: 320px HtmlService sidebar titled "Set Bank Coin" with 4 number inputs (PP/GP/SP/CP) pre-populated via `google.script.run.withSuccessHandler(populate).getBankCoinForForm()`. Description: "Manual entry — /outputfile inventory does not include coin totals."

`getBankCoinForForm(): BankCoinForm`: reads `_meta` rows; extracts `bank_coin_pp/gp/sp/cp` via key-lookup; returns `{pp, gp, sp, cp}` with 0 fallback for missing rows or non-finite values.

`saveBankCoin(coin)`: validation pass (rejects negative, NaN); acquires 30s document lock; `writeMetaRow('_meta', 'bank_coin_pp', String(coin.pp))` × 4 + `writeMetaRow('_meta', 'bank_coin_last_updated', new Date().toISOString())`; releases lock; calls `buildBank()` outside the lock (buildBank acquires its own).

**Note on bank_coin_last_updated**: NEW _meta row not in Phase 2 scaffold. Created lazily by writeMetaRow's write-if-absent semantics — same pattern as Phase 3's theme/contact_email lazy rows. Avoids overkill v=4 migration for a single timestamp.

7 vitest scenarios: getBankCoinForForm returns 0s when _meta empty; returns parsed values when rows present; saveBankCoin writes 4 + last_updated row; rejects negative; rejects NaN; lock contention throws; calls buildBank after writes.

### Task 2 — buildBank coin row prepend (commit `dfe4102`)

Modified `buildBank.ts` `runBuild`:
- Read `_meta` rows once at start.
- Compose `coinRow = [bankToon, 'COIN', display, '', '', '', '', lastUpdated]` where `display = 'Platinum: <pp> | Gold: <gp> | Silver: <sp> | Copper: <cp>'`.
- After clearing: write `bankSheet.getRange(2, 1, 1, 8).setValues([coinRow])` (row 2 only).
- Write inventory data via `bankSheet.getRange(3, 1, dataRows.length, 8).setValues(dataRows)` (row 3+).
- Notes via `bankSheet.getRange(3, ITEM_COL, dataRows.length, 1).setNotes(notes)` (shifted to row 3).

Two RPC calls instead of one but explicit row separation. Tests extended (4 new scenarios within buildBank.test.ts): coin row at row 2 + inventory at row 3; format string matches spec; bank_coin_* missing → all 0s; bank_toon_name empty → no rows (coin row also blocked by early-return).

### Task 3 — protectBankCoinCells (commit `3babef6`)

`protectBankCoinCells()`: iterate `_meta` rows; for each `bank_coin_pp/gp/sp/cp` key (skip if row absent), get value cell (col B), check existing protections for matching description `'SquireBot bank coin — edit via SquireBot menu'` (idempotency gate), if absent call `cell.protect().setDescription(...).setWarningOnly(false)`. Log `{protected: 4}` on success.

Called from BOTH:
- `migrateToV3()` post-success path — first-time setup on schema migration.
- `installTriggers()` — defensive re-apply (covers workbooks migrated before this code shipped).

Live-smoke note: subsequent fix-pack `3c5ea6d` (out-of-scope for this SUMMARY) changed `setWarningOnly(false)` → `setWarningOnly(true)` because the hard lock interrupted the in-game-coin-tracking UX flow; saveBankCoin auto-applies protection after every save. Documented in v1.0-ROADMAP.md.

### Task 4 — monitorCellCount (commit `50714b7`)

```typescript
function monitorCellCount(): void {
  const ss = getActiveSpreadsheet();
  let total = 0;
  const perSheet = [];
  for (const sheet of ss.getSheets()) {
    const cells = sheet.getLastRow() * sheet.getLastColumn();
    perSheet.push({ name: sheet.getName(), cells });
    total += cells;
  }
  writeMetaRow('_status', 'cell_count', String(total));
  writeMetaRow('_status', 'cell_count_last_check', new Date().toISOString());
  if (total > 5_000_000) {
    const top5 = perSheet.sort((a, b) => b.cells - a.cells).slice(0, 5)
      .map(s => `${s.name}=${s.cells}`).join(', ');
    const err = { at, where: 'monitorCellCount', kind: 'cell_count_threshold',
                  detail: `${total}/10000000 (top: ${top5})` };
    writeMetaRow('_meta', 'last_error', JSON.stringify(err));
    writeMetaRow('_status', 'last_error', JSON.stringify(err));
  }
}
```

Threshold 5M = 50% of Google's 10M hard cap. Top-5 reporting tells officers which sheets to address.

4 vitest scenarios: 3 small sheets <threshold (cell_count written, no error); 3 sheets summing >5M (cell_count + last_error envelope); top-5 reporting in detail string; empty workbook (cell_count=0).

### Task 5 — installTriggers expansion 4 → 7 (commit `14878c3`)

```typescript
const SQUIREBOT_HANDLERS = [
  'onChange', 'buildView', 'refreshPigparse', 'refreshWikiItems',
  'refreshWikiSpells', 'refreshWikiGearTier', 'monitorCellCount',  // NEW
];
```

Delete-loop extended automatically (any matching handler is deleted before recreate). 3 new triggers added:

```typescript
ScriptApp.newTrigger('refreshWikiSpells').timeBased()
  .onWeekDay(ScriptApp.WeekDay.SUNDAY).atHour(4)
  .inTimezone('America/Los_Angeles').create();
ScriptApp.newTrigger('refreshWikiGearTier').timeBased()
  .onWeekDay(ScriptApp.WeekDay.SUNDAY).atHour(5)
  .inTimezone('America/Los_Angeles').create();
ScriptApp.newTrigger('monitorCellCount').timeBased()
  .onWeekDay(ScriptApp.WeekDay.SUNDAY).atHour(3)
  .inTimezone('America/Los_Angeles').create();
```

Defensive `protectBankCoinCells()` call inside installTriggers — covers workbooks migrated before this code shipped.

Updated alert text listing all 7 triggers. installTriggers.test.ts extended to assert 7 triggers created + idempotent re-run.

### Task 6-7 — Wire onOpen + Code.ts + build.mjs + docs (commits `6bca772`, `df636a4`)

`onOpen.ts`: `addItem('Set Bank Coin…', 'showBankCoinSidebar')` inserted between `'Set Character Info…'` and `'Set Theme…'`.

`Code.ts`: 5 new re-exports — `showBankCoinSidebar`, `getBankCoinForForm`, `saveBankCoin`, `monitorCellCount`, `protectBankCoinCells`. CI assertion catches mis-sync.

`build.mjs` TRIGGER_GLOBALS: 5 new entries. assertExportsMatchGlobals passes.

`docs/apps-script-deploy.md` extended: step 9 says migrateToV3 instead of migrateToV2; step 10 mentions 7 triggers; step 11.5 adds "Set Bank Coin… → enter guild's current coin balance"; new "Range.protect smoke check" subsection (edit B<X> of _meta directly → expect protection warning prompt; Set Bank Coin Save → expect no prompt); schema-version coordination table updated:

| Watcher version | Max schema version it can write |
|-----------------|---------------------------------|
| v0.1.x – v0.2.x | 1                               |
| v0.3.x          | 2                               |
| v0.4.0+         | 3                               |

### Task 8 — STATE.md → Phase 4 CODE-COMPLETE (commit `c0cff2d`)

Status `phase-4-code-complete`. completed_plans 25 → 26. Phase 4 ready for live smoke → v0.4.0 release.

## Deviations from Plan

Plan executed essentially as written. Three post-plan fix-packs landed during Phase 4 live-smoke (out-of-scope for this SUMMARY but documented in v1.0-ROADMAP.md):

1. **`3c5ea6d` bank coin protection — warningOnly + saveBankCoin auto-apply** (2026-05-10 20:16): hard lock interrupted in-game-coin-tracking UX. Switched `setWarningOnly(false)` → `setWarningOnly(true)`; saveBankCoin now auto-applies protection after every save.
2. **`b9482a6` add on-demand menu items for new Phase-4 triggers** (2026-05-10 20:22): missing "Refresh Wiki Spells Now" + "Refresh Wiki Gear Tier Now" + "Run Cell Count Check" entries in SquireBot menu (only the time-driven triggers were wired; on-demand menu items were forgotten in the menu chain).
3. **`9319c6b` wiki-spell-parser handles 3 template variants** (2026-05-10 21:09): see 04-02 SUMMARY deviation note — handles SpellRow / RadSpellRow / RadSpellRow2 / SongRow.

## Schema impact

None — schema_version remains at 3. `_meta.bank_coin_pp/gp/sp/cp/last_updated` rows are KV entries inside the existing `_meta` tab (extend-only via rows — always non-breaking). No new tabs, no new columns, no migration.

## Verification log

```
$ npm test -- bankCoinSidebar
Tests       7 passed (7)

$ npm test -- monitorCellCount
Tests       4 passed (4)

$ npm test -- installTriggers
Tests       N passed (assertions verify 7 triggers + idempotent re-run)

$ npm run build
(exit 0 — 23 trigger globals total — Phase 3's 10 + Phase 4's 13 cumulative
 across 04-01 + 04-02 + 04-03 + 04-04)

# Live smoke (post-deploy):
# 1. Installer + tray green; version=0.4.0-rc1 in heartbeat log
# 2. ErrSchemaTooNew startup gate fires on un-migrated workbook
# 3. migrateToV3 + Install Triggers (7 triggers + bank-coin protect)
# 4. Range.protect warning prompt fires on direct _meta cell edit
# 5. gear_check populates with OK/MISSING/OTHER status
# 6. spell_check populates (post 9319c6b fix: 1,562 spells across 11 classes)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages + v1.0-ROADMAP.md Phase 4 details.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/triggers/showBankCoinSidebar.ts`
- FOUND: `apps-script/src/triggers/monitorCellCount.ts`
- FOUND: `apps-script/src/__tests__/bankCoinSidebar.test.ts`
- FOUND: `apps-script/src/__tests__/monitorCellCount.test.ts`
- FOUND: `apps-script/src/lib/migrations.ts` (contains protectBankCoinCells)
- FOUND: `apps-script/src/triggers/installTriggers.ts` (7-trigger expansion)

**Commits exist:**
- FOUND: `3751123` — feat(apps-script): showBankCoinSidebar + form callbacks + tests
- FOUND: `dfe4102` — feat(apps-script): buildBank coin row prepend + tests
- FOUND: `3babef6` — feat(apps-script): protectBankCoinCells + wired into migrateToV3
- FOUND: `50714b7` — feat(apps-script): monitorCellCount weekly trigger + tests
- FOUND: `14878c3` — feat(apps-script): installTriggers 4 -> 7 + bank-coin protect re-apply
- FOUND: `6bca772` — chore(apps-script): wire bank-coin menu + Code.ts + TRIGGER_GLOBALS
- FOUND: `df636a4` — docs: apps-script-deploy Phase 4 + Range.protect smoke check
- FOUND: `c0cff2d` — chore: STATE.md -> Phase 4 CODE-COMPLETE

## Next plan

`/gsd-execute-phase 5` opened Phase 5 (Search + Onboarding + Privacy Polish) starting with **05-01** — system-tab hide + bank_toon_name protect + weekly schema healthcheck. Phase 5 D-12 Path A: no further `WatcherMaxSchemaVersion` bump (schema held at 3 end-to-end across all 5 Phase-5 plans). Apps-script-only changes. After Phase 5 ships: v1.0.0 milestone CODE-COMPLETE.

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 04-04-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
