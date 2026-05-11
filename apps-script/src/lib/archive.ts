// Phase 5 plan 05-02 task 1: lazy-creating, lock-guarded character archive.
//
// Path A (per 05-RESEARCH §Pattern 6 + §Pitfall P11): no schema_version
// bump. The _archive tab is created on first call. The watcher does NOT
// write to it, so we don't bump WatcherMaxSchemaVersion either —
// internal/sheet/client.go stays at 3.
//
// RACE NOTE (Pitfall P6): we read _char_owner.last_seen INSIDE the lock.
// The watcher's heartbeat write to _char_owner.last_seen does NOT
// currently acquire LockService (per OPS-01, watchers use per-char
// non-overlapping ranges). Worst case: one missed archive cycle if a
// watcher rejoins mid-operation and writes last_seen while the trigger
// is iterating. Acceptable — the char gets re-evaluated next weekly run.

import { log } from './log';
import { getActiveSpreadsheet, getOrCreateSheet, readMetaRows, writeMetaRow } from './sheet-helpers';

const ARCHIVE_TAB = '_archive';
const ARCHIVE_HEADERS = [
  'archived_at', 'char_name', 'tab_type', 'row_count',
  'uploaded_at', 'reason', 'snapshot_json',
];

// _char_owner column layout (1-indexed) per Phase 2 SCHEMA-05 + Phase 4
// migrateToV3 (race appended at col 14, not used here):
//   1=char_name, 2=owner_email, 3=display_name, 4=discord_handle,
//   5=class, 6=level, 7=is_bank_toon, 8=is_hidden, 9=is_removed,
//   10=first_seen, 11=last_seen, 12=server, 13=watcher_version, 14=race
const COL_CHAR_OWNER_CHAR_NAME = 1;
const COL_CHAR_OWNER_IS_REMOVED = 9;
const CHAR_OWNER_COL_COUNT = 13;

export type ArchiveReason = 'stale_90d' | 'evicted';

export function moveCharToArchive(charName: string, reason: ArchiveReason): void {
  if (!charName || typeof charName !== 'string') {
    log('warn', 'moveCharToArchive', { skipped: 'invalid_char_name' });
    return;
  }

  // Pitfall P6 mitigation: 30-second cap on lock acquisition.
  // Canonical envelope (LockService.getDocumentLock().tryLock(30000))
  // cloned verbatim from migrations.ts:89-92 — same shape across all
  // lock-guarded helpers in apps-script/src/lib/.
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    throw new Error('moveCharToArchive: could not acquire document lock within 30s');
  }
  try {
    const ss = getActiveSpreadsheet();
    const ownerSheet = ss.getSheetByName('_char_owner');
    if (!ownerSheet) {
      log('warn', 'moveCharToArchive', { skipped: '_char_owner_missing', charName });
      return;
    }

    // 1. Locate the char's row in _char_owner.
    const lastRow = ownerSheet.getLastRow();
    const values = lastRow > 0
      ? ownerSheet.getRange(1, 1, lastRow, CHAR_OWNER_COL_COUNT).getValues()
      : [];
    let ownerRowIdx = -1;
    for (let i = 1; i < values.length; i++) {
      if (String(values[i][COL_CHAR_OWNER_CHAR_NAME - 1] ?? '').trim() === charName) {
        ownerRowIdx = i;
        break;
      }
    }
    if (ownerRowIdx === -1) {
      log('warn', 'moveCharToArchive', { skipped: 'char_not_in_owner_table', charName });
      return;
    }

    // 2. Idempotency: if already archived (is_removed truthy AND source
    //    tabs hidden), no-op.
    const isRemovedCell = values[ownerRowIdx][COL_CHAR_OWNER_IS_REMOVED - 1];
    const alreadyRemoved = isRemovedCell === true
      || String(isRemovedCell).toLowerCase() === 'true';
    const invSheet = ss.getSheetByName(`inv:${charName}`);
    const spellSheet = ss.getSheetByName(`spell:${charName}`);
    const sourcesHidden = (!invSheet || invSheet.isSheetHidden())
                       && (!spellSheet || spellSheet.isSheetHidden());
    if (alreadyRemoved && sourcesHidden) {
      log('info', 'moveCharToArchive', { skipped: 'already_archived', charName, reason });
      return;
    }

    // 3. Lazy-create _archive with the 7-col header. A freshly-inserted
    //    sheet has lastColumn=0 (and lastRow=0 in real Apps Script; in
    //    the vitest mock lastRow=1 because the FakeSheet starts with an
    //    empty `[[]]` row). Either way, lastColumn===0 reliably indicates
    //    "no header written yet".
    const archiveSheet = getOrCreateSheet(ARCHIVE_TAB);
    if (archiveSheet.getLastColumn() === 0) {
      archiveSheet.getRange(1, 1, 1, ARCHIVE_HEADERS.length).setValues([ARCHIVE_HEADERS]);
      archiveSheet.hideSheet();
    }

    // 4. Snapshot each source tab (inv first, then spell).
    const now = new Date().toISOString();
    const sourceTabs: Array<['inv' | 'spell', GoogleAppsScript.Spreadsheet.Sheet | null]> = [
      ['inv', invSheet],
      ['spell', spellSheet],
    ];
    const snapshots: Array<{
      tabType: 'inv' | 'spell';
      rowCount: number;
      uploadedAt: string;
      json: string;
    }> = [];
    for (const [tabType, sheet] of sourceTabs) {
      if (!sheet) {
        snapshots.push({ tabType, rowCount: 0, uploadedAt: '', json: '[]' });
        continue;
      }
      const lastSrc = sheet.getLastRow();
      const lastCol = sheet.getLastColumn();
      const rows = lastSrc > 0 && lastCol > 0
        ? sheet.getRange(1, 1, lastSrc, lastCol).getValues()
        : [];
      // _uploaded_at lookup: SCHEMA-01/02 convention puts it as one of
      // the columns; we don't assume position.
      let uploadedAt = '';
      if (rows.length > 1) {
        const headers = rows[0].map((h) => String(h));
        const idx = headers.indexOf('_uploaded_at');
        if (idx >= 0) uploadedAt = String(rows[1][idx] ?? '');
      }
      snapshots.push({
        tabType,
        rowCount: Math.max(0, lastSrc - 1),
        uploadedAt,
        json: JSON.stringify(rows),
      });
    }

    // 5. Append archive rows.
    const archiveRows = snapshots.map((s) => [
      now, charName, s.tabType, s.rowCount, s.uploadedAt, reason, s.json,
    ]);
    const startRow = archiveSheet.getLastRow() + 1;
    archiveSheet.getRange(startRow, 1, archiveRows.length, ARCHIVE_HEADERS.length)
      .setValues(archiveRows);

    // 6. Flip is_removed=TRUE in _char_owner (column 9, 1-based row index
    //    in sheet coords = ownerRowIdx + 1).
    ownerSheet.getRange(ownerRowIdx + 1, COL_CHAR_OWNER_IS_REMOVED).setValue(true);

    // 7. Hide source tabs (NOT delete — preserves watcher recovery via
    //    WATCH-09 catch-up on next mtime change).
    if (invSheet && !invSheet.isSheetHidden()) invSheet.hideSheet();
    if (spellSheet && !spellSheet.isSheetHidden()) spellSheet.hideSheet();

    // 8. Append to _meta.archive_log JSON array.
    const meta = readMetaRows('_meta');
    const logRow = meta.find((r) => r.key === 'archive_log');
    const list: Array<Record<string, unknown>> = logRow && logRow.value
      ? JSON.parse(logRow.value)
      : [];
    list.push({
      at: now,
      char: charName,
      reason,
      inv_row_count: snapshots[0].rowCount,
      spell_row_count: snapshots[1].rowCount,
    });
    writeMetaRow('_meta', 'archive_log', JSON.stringify(list));

    log('info', 'moveCharToArchive', {
      charName, reason,
      invRows: snapshots[0].rowCount,
      spellRows: snapshots[1].rowCount,
    });
  } finally {
    lock.releaseLock();
  }
}
