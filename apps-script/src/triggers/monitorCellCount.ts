// monitorCellCount — Phase 4 plan 04-04 task 4 (OPS-07).
//
// Weekly trigger (Sun 03:00 PT — installed by installTriggers) that
// sums addressable cells across all sheets and writes the total to
// _status.cell_count. Google Sheets enforces a 10M cell hard cap per
// workbook; we alarm at 5M (50% headroom) so the guild has time to
// trim before they hit the wall.
//
// On threshold trip: writes a structured JSON blob to _meta.last_error
// AND _status.last_error of kind 'cell_count_threshold' with detail
// '<count>/<cap> (top: <top-5 sheets>)'. The watcher reads _meta.last_error
// for status-tab visibility so the next heartbeat surfaces the warning.

import { log } from '../lib/log';
import { getActiveSpreadsheet, writeMetaRow } from '../lib/sheet-helpers';

const ALARM_THRESHOLD = 5_000_000;  // 50% of the 10M cell cap
const HARD_CAP = 10_000_000;
const TOP_N = 5;

export function monitorCellCount(): void {
  const ss = getActiveSpreadsheet();
  let total = 0;
  const perSheet: Array<{ name: string; cells: number }> = [];
  for (const sheet of ss.getSheets()) {
    const cells = sheet.getLastRow() * sheet.getLastColumn();
    perSheet.push({ name: sheet.getName(), cells });
    total += cells;
  }

  writeMetaRow('_status', 'cell_count', String(total));
  writeMetaRow('_status', 'cell_count_last_check', new Date().toISOString());

  if (total > ALARM_THRESHOLD) {
    const topN = perSheet
      .sort((a, b) => b.cells - a.cells)
      .slice(0, TOP_N)
      .map((s) => `${s.name}=${s.cells}`)
      .join(', ');
    const err = {
      at: new Date().toISOString(),
      where: 'monitorCellCount',
      kind: 'cell_count_threshold',
      detail: `${total}/${HARD_CAP} (top: ${topN})`,
    };
    const errJson = JSON.stringify(err);
    writeMetaRow('_meta', 'last_error', errJson);
    writeMetaRow('_status', 'last_error', errJson);
    log('warn', 'monitorCellCount', { total, threshold: ALARM_THRESHOLD, topN });
    return;
  }
  log('info', 'monitorCellCount', { total, sheets: perSheet.length });
}
