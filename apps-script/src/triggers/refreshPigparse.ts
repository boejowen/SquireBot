// refreshPigparse — daily 03:00 PT pricing scrape.
//
// One GET against PigParse, validate row count >= 90% of last known
// (refuse to clobber on truncation), atomic full replace of _pigparse,
// update _meta.last_pigparse_refresh + _status.last_pigparse_row_count,
// clear _meta.last_error on success.
//
// Phase 3 plan 03-02. The matching daily time-driven trigger is
// installed by plan 03-04's installTriggers() — the function exported
// here can also be invoked manually from the SquireBot menu's
// "Refresh PigParse Now" item.

import { log } from '../lib/log';
import { politeFetch } from '../lib/politeFetch';
import { parseToRows, type PigparseRowRaw } from '../lib/pigparse-types';
import {
  getActiveSpreadsheet,
  readMetaRowInt,
  writeMetaRow,
} from '../lib/sheet-helpers';

const PIGPARSE_URL = 'https://pigparse.azurewebsites.net/api/item/getall/1';
const ROW_COUNT_FLOOR_PCT = 0.90;
const PIGPARSE_TAB = '_pigparse';

// Header order MUST match scaffold.go's DimensionTabs[_pigparse] +
// migrateToV2's appendColumns. Keep this constant in sync if either
// changes (CI will catch via the "headers match" assertion at startup).
const PIGPARSE_HEADERS = [
  // v1 cols (from internal/scaffold/scaffold.go)
  'item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed',
  // v2 cols (from migrations.ts PIGPARSE_V2_COLUMNS)
  'direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay',
];

interface ErrorRecord {
  at: string;
  where: 'refreshPigparse';
  kind: 'fetch_failed' | 'parse_failed' | 'truncated_response' | 'lock_busy' | 'sheet_missing';
  detail: string;
}

export function refreshPigparse(): void {
  const startMs = Date.now();
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    log('warn', 'refreshPigparse', { skipped: 'lock_busy' });
    writeError({ at: nowIso(), where: 'refreshPigparse', kind: 'lock_busy',
                 detail: 'could not acquire document lock within 30s' });
    return;
  }
  try {
    runUnderLock(startMs);
  } finally {
    lock.releaseLock();
  }
}

function runUnderLock(startMs: number): void {
  const result = politeFetch(PIGPARSE_URL);
  if (!result.ok) {
    writeError({ at: nowIso(), where: 'refreshPigparse', kind: 'fetch_failed',
                 detail: `status=${result.status} ${result.error}` });
    return;
  }

  let rows: PigparseRowRaw[];
  try {
    rows = parseToRows(result.body);
  } catch (e) {
    writeError({ at: nowIso(), where: 'refreshPigparse', kind: 'parse_failed',
                 detail: (e as Error).message });
    return;
  }

  const lastCount = readMetaRowInt('_status', 'last_pigparse_row_count') ?? 0;
  if (lastCount > 0 && rows.length < lastCount * ROW_COUNT_FLOOR_PCT) {
    writeError({ at: nowIso(), where: 'refreshPigparse', kind: 'truncated_response',
                 detail: `today=${rows.length} last=${lastCount} threshold=${ROW_COUNT_FLOOR_PCT}` });
    log('warn', 'refreshPigparse', { abandoned: 'truncated', today: rows.length, last: lastCount });
    return;
  }

  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(PIGPARSE_TAB);
  if (!sheet) {
    writeError({ at: nowIso(), where: 'refreshPigparse', kind: 'sheet_missing',
                 detail: `${PIGPARSE_TAB} tab missing — run migrateToV2 first` });
    throw new Error(`${PIGPARSE_TAB} missing — run migrateToV2 first`);
  }

  const now = nowIso();
  const dataRows = rows.map((r) => buildRow(r, now));

  // Clear all data rows below header (col-aware: clear the full PIGPARSE_HEADERS
  // width so any pre-v2 stale columns get blanked too).
  const lastRow = sheet.getLastRow();
  if (lastRow > 1) {
    sheet.getRange(2, 1, lastRow - 1, PIGPARSE_HEADERS.length).clearContent();
  }
  if (dataRows.length > 0) {
    sheet.getRange(2, 1, dataRows.length, PIGPARSE_HEADERS.length).setValues(dataRows);
  }

  writeMetaRow('_status', 'last_pigparse_row_count', String(rows.length));
  writeMetaRow('_meta', 'last_pigparse_refresh', now);
  clearError();

  log('info', 'refreshPigparse', {
    rows: rows.length,
    durationMs: Date.now() - startMs,
    retriesUsed: result.retriesUsed,
  });
}

function buildRow(r: PigparseRowRaw, now: string): unknown[] {
  // v1 cols: item_id, name, current_avg (alias for a30), last_seen,
  // blue_volume (alias for t30), last_refreshed.
  // v2 cols: direction, t30, a30, t60, a60, t6m, a6m, ty, ay.
  return [
    r.i, r.n, r.a30, r.l, r.t30, now,
    r.t, r.t30, r.a30, r.t60, r.a60, r.t6m, r.a6m, r.ty, r.ay,
  ];
}

function nowIso(): string {
  return new Date().toISOString();
}

function writeError(err: ErrorRecord): void {
  const json = JSON.stringify(err);
  writeMetaRow('_meta', 'last_error', json);
  writeMetaRow('_status', 'last_error', json);
}

function clearError(): void {
  writeMetaRow('_meta', 'last_error', '{}');
  writeMetaRow('_status', 'last_error', '{}');
}
