// installTriggers — Phase 3 plan 03-04 task 6.
//
// Idempotent setup. Deletes existing SquireBot triggers (matched by
// handler-function name), then re-creates the four required triggers:
//
//   1. onChange (sheet-bound) — fires on any workbook change; debounced
//      and lock-protected inside buildView.
//   2. buildView every 1 hour — backstop for missed onChange events
//      (Apps Script simple/installable triggers are best-effort).
//   3. refreshPigparse daily 03:00 PT — single-endpoint scrape.
//   4. refreshWikiItems weekly Sunday 04:00 PT — resumable cursor.
//
// Callable from the SquireBot menu's "Install Triggers" item OR from
// the script editor's Run dropdown. Re-running is safe (deletes prior
// SquireBot triggers first; idempotent).

import { log } from '../lib/log';

const SQUIREBOT_HANDLERS = [
  'onChange',
  'buildView',
  'refreshPigparse',
  'refreshWikiItems',
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

  // Step 2: re-create all four.
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

  log('info', 'installTriggers', { deleted, created: 4 });

  SpreadsheetApp.getUi().alert(
    [
      'SquireBot triggers installed.',
      '',
      '• onChange: rebuilds view + bank (debounced 10s)',
      '• 1h backstop: catches missed onChange events',
      '• Daily 03:00 PT: refreshPigparse',
      '• Sunday 04:00 PT: refreshWikiItems',
    ].join('\n'),
  );
}
