// installTriggers — extended in Phase 4 plan 04-04 from 4 → 7 triggers.
//
// Idempotent setup. Deletes existing SquireBot triggers (matched by
// handler-function name in SQUIREBOT_HANDLERS), then re-creates the
// seven required triggers:
//
//   1. onChange (sheet-bound) — fires on any workbook change; debounced
//      and lock-protected inside each builder.
//   2. buildView every 1 hour — backstop for missed onChange events
//      (Apps Script simple/installable triggers are best-effort).
//   3. refreshPigparse daily 03:00 PT — single-endpoint scrape.
//   4. refreshWikiItems weekly Sunday 04:00 PT — resumable cursor.
//   5. refreshWikiSpells weekly Sunday 04:00 PT — class-page scrape.
//   6. refreshWikiGearTier weekly Sunday 05:00 PT — 2-page scrape.
//   7. monitorCellCount weekly Sunday 03:00 PT — 10M cell-cap watchdog.
//
// Time precision: Apps Script's atHour() schedules within a 1-hour
// window — the 04:00-PT items may both fire in the same hour. Each
// trigger's lock + debounce protects against contention; the wiki
// triggers touch independent dimension tabs so no real contention.
//
// Defensive: also re-applies bank-coin cell protection (idempotent —
// covers workbooks migrated before plan 04-04 shipped, and re-applies
// to bank_coin_* rows created lazily after migrateToV3 ran).
//
// Callable from the SquireBot menu's "Install Triggers" item OR from
// the script editor's Run dropdown. Re-running is safe.

import { log } from '../lib/log';
import { protectBankCoinCells, protectBankToonName, hideAllSystemTabs } from '../lib/migrations';

const SQUIREBOT_HANDLERS = [
  'onChange',
  'buildView',
  'refreshPigparse',
  'refreshWikiItems',
  'refreshWikiSpells',
  'refreshWikiGearTier',
  'monitorCellCount',
  'weeklySchemaHealthcheck',  // NEW 05-01
];

export function installTriggers(): void {
  // Step 1: delete existing SquireBot-handler triggers (idempotency).
  const existing = ScriptApp.getProjectTriggers();
  let deleted = 0;
  for (const t of existing) {
    if (SQUIREBOT_HANDLERS.includes(t.getHandlerFunction())) {
      ScriptApp.deleteTrigger(t);
      deleted++;
    }
  }

  // Step 2: re-create all seven.
  const ss = SpreadsheetApp.getActiveSpreadsheet();
  ScriptApp.newTrigger('onChange').forSpreadsheet(ss).onChange().create();
  ScriptApp.newTrigger('buildView').timeBased().everyHours(1).create();
  ScriptApp.newTrigger('refreshPigparse')
    .timeBased()
    .atHour(3)
    .everyDays(1)
    .inTimezone('America/Los_Angeles')
    .create();
  ScriptApp.newTrigger('refreshWikiItems')
    .timeBased()
    .onWeekDay(ScriptApp.WeekDay.SUNDAY)
    .atHour(4)
    .inTimezone('America/Los_Angeles')
    .create();
  ScriptApp.newTrigger('refreshWikiSpells')
    .timeBased()
    .onWeekDay(ScriptApp.WeekDay.SUNDAY)
    .atHour(4)
    .inTimezone('America/Los_Angeles')
    .create();
  ScriptApp.newTrigger('refreshWikiGearTier')
    .timeBased()
    .onWeekDay(ScriptApp.WeekDay.SUNDAY)
    .atHour(5)
    .inTimezone('America/Los_Angeles')
    .create();
  ScriptApp.newTrigger('monitorCellCount')
    .timeBased()
    .onWeekDay(ScriptApp.WeekDay.SUNDAY)
    .atHour(3)
    .inTimezone('America/Los_Angeles')
    .create();
  // Phase 5 plan 05-01: schema healthcheck rides the same Sun-03:00 PT
  // window as monitorCellCount. Both touch independent KV rows in
  // _meta/_status so no contention; Apps Script may fire them back-to-back
  // inside the same hour window.
  ScriptApp.newTrigger('weeklySchemaHealthcheck')
    .timeBased()
    .onWeekDay(ScriptApp.WeekDay.SUNDAY)
    .atHour(3)
    .inTimezone('America/Los_Angeles')
    .create();

  // Step 3: defensive re-apply of bank-coin cell protection. Idempotent
  // — protectBankCoinCells skips already-protected cells by description
  // match. Also handles workbooks migrated before plan 04-04 shipped.
  protectBankCoinCells();
  // Phase 5 plan 05-01: same idempotent shape, single-cell protection on
  // _meta.bank_toon_name. Skip-if-row-missing — lazy-created by saveBankCoin.
  protectBankToonName();
  // Phase 5 plan 05-01: every install hides any _-prefixed tab that is
  // currently visible. Idempotent via isSheetHidden() short-circuit.
  hideAllSystemTabs();

  log('info', 'installTriggers', { deleted, created: 8 });

  SpreadsheetApp.getUi().alert(
    [
      'SquireBot triggers installed (8 total).',
      '',
      '• onChange: rebuilds view + bank + spell_check + gear_check (debounced 10s)',
      '• 1h backstop: catches missed onChange events',
      '• Daily 03:00 PT: refreshPigparse',
      '• Sunday 03:00 PT: monitorCellCount (10M cell-cap watchdog)',
      '• Sunday 03:00 PT: weeklySchemaHealthcheck (expected-tab watchdog)',
      '• Sunday 04:00 PT: refreshWikiItems',
      '• Sunday 04:00 PT: refreshWikiSpells',
      '• Sunday 05:00 PT: refreshWikiGearTier',
      '',
      'Bank coin + bank-toon-name cells in _meta are now protected, and',
      'all system (_-prefixed) tabs are hidden. Use SquireBot → Set Bank',
      'Coin… to update them.',
    ].join('\n'),
  );
}
