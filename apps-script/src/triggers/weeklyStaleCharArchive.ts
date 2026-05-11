// weeklyStaleCharArchive — Phase 5 plan 05-02 task 2 (VIEW-05).
//
// Weekly trigger (Sun 06:00 PT — installed by installTriggers) that
// scans _char_owner.last_seen and archives every char whose last
// heartbeat is older than STALE_MS (90 days). Delegates the actual
// snapshot + tab-hide + is_removed flip to lib/archive.moveCharToArchive.
//
// Skipping chars with is_removed=TRUE is a short-circuit only: the
// underlying moveCharToArchive helper is itself idempotent on already-
// archived chars, but the short-circuit avoids the document-lock
// overhead for already-evicted guildies.
//
// On missing _char_owner: writes the canonical {at, where, kind, detail}
// envelope to _meta.last_error + _status.last_error (kind='tab_missing'),
// same shape as monitorCellCount and weeklySchemaHealthcheck so the
// watcher heartbeat reader surfaces the warning identically.

import { log } from '../lib/log';
import { getActiveSpreadsheet, writeMetaRow } from '../lib/sheet-helpers';
import { moveCharToArchive } from '../lib/archive';

const STALE_MS = 90 * 24 * 60 * 60 * 1000;
const COL_CHAR_NAME = 1;
const COL_IS_REMOVED = 9;
const COL_LAST_SEEN = 11;
const CHAR_OWNER_COL_COUNT = 13;

export function weeklyStaleCharArchive(): void {
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName('_char_owner');
  if (!sheet) {
    const err = {
      at: new Date().toISOString(),
      where: 'weeklyStaleCharArchive',
      kind: 'tab_missing',
      detail: '_char_owner',
    };
    const errJson = JSON.stringify(err);
    writeMetaRow('_meta', 'last_error', errJson);
    writeMetaRow('_status', 'last_error', errJson);
    log('warn', 'weeklyStaleCharArchive', { missing: '_char_owner' });
    return;
  }

  const now = Date.now();
  const cutoff = now - STALE_MS;
  const lastRow = sheet.getLastRow();
  const values = lastRow > 0
    ? sheet.getRange(1, 1, lastRow, CHAR_OWNER_COL_COUNT).getValues()
    : [];
  const candidates: string[] = [];
  for (let i = 1; i < values.length; i++) {
    const row = values[i];
    const isRemovedCell = row[COL_IS_REMOVED - 1];
    const isRemoved = isRemovedCell === true
      || String(isRemovedCell).toLowerCase() === 'true';
    if (isRemoved) continue;
    const lastSeenRaw = row[COL_LAST_SEEN - 1];
    if (!lastSeenRaw) continue;
    const lastSeenMs = typeof lastSeenRaw === 'string'
      ? Date.parse(lastSeenRaw)
      : (lastSeenRaw instanceof Date
        ? lastSeenRaw.getTime()
        : Number(lastSeenRaw));
    if (!Number.isFinite(lastSeenMs)) continue;
    if (lastSeenMs < cutoff) {
      candidates.push(String(row[COL_CHAR_NAME - 1] ?? '').trim());
    }
  }

  let archived = 0;
  for (const char of candidates) {
    if (!char) continue;
    try {
      moveCharToArchive(char, 'stale_90d');
      archived++;
    } catch (e) {
      log('warn', 'weeklyStaleCharArchive', { failed: char, error: String(e) });
    }
  }

  writeMetaRow('_status', 'last_stale_archive_run', new Date().toISOString());
  writeMetaRow('_status', 'last_stale_archive_count', String(archived));
  log('info', 'weeklyStaleCharArchive', { archived, candidates: candidates.length });
}
