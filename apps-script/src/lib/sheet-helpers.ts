// Thin wrappers over SpreadsheetApp for the operations Phase 3 uses
// repeatedly. None of these acquire LockService — caller is responsible
// for wrapping write-multiple operations in a single tryLock.

export function getActiveSpreadsheet(): GoogleAppsScript.Spreadsheet.Spreadsheet {
  return SpreadsheetApp.getActiveSpreadsheet();
}

export function getOrCreateSheet(name: string): GoogleAppsScript.Spreadsheet.Sheet {
  const ss = getActiveSpreadsheet();
  const existing = ss.getSheetByName(name);
  if (existing) return existing;
  return ss.insertSheet(name);
}

export interface MetaRow { key: string; value: string; rowIndex: number; }

// readMetaRows reads _meta!A:B (skipping the header row at A1) and
// returns one MetaRow per non-empty key cell, with rowIndex 1-based and
// pointing at the row in the sheet (so writes can address it directly).
export function readMetaRows(sheetName = '_meta'): MetaRow[] {
  const sheet = getActiveSpreadsheet().getSheetByName(sheetName);
  if (!sheet) return [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return [];
  const values = sheet.getRange(2, 1, lastRow - 1, 2).getValues();
  const rows: MetaRow[] = [];
  for (let i = 0; i < values.length; i++) {
    const key = String(values[i][0] ?? '').trim();
    if (!key) continue;
    rows.push({ key, value: String(values[i][1] ?? ''), rowIndex: i + 2 });
  }
  return rows;
}

// writeMetaRow upserts a single key on a KV-shaped sheet (cols A=key,
// B=value). On insert: appendRow. On update: setValue at the existing
// cell. Returns true if a write happened, false if the row already had
// the same value.
export function writeMetaRow(sheetName: string, key: string, value: string): boolean {
  const sheet = getOrCreateSheet(sheetName);
  const rows = readMetaRows(sheetName);
  const existing = rows.find((r) => r.key === key);
  if (existing) {
    if (existing.value === value) return false;
    sheet.getRange(existing.rowIndex, 2).setValue(value);
    return true;
  }
  sheet.appendRow([key, value]);
  return true;
}

// readMetaRowInt reads a numeric value out of a KV sheet. Returns null
// if the key is absent or the value isn't a finite number.
export function readMetaRowInt(sheetName: string, key: string): number | null {
  const rows = readMetaRows(sheetName);
  const row = rows.find((r) => r.key === key);
  if (!row) return null;
  const n = parseInt(row.value, 10);
  return Number.isFinite(n) ? n : null;
}

// appendColumns is the migration utility. It reads row 1 (the header
// row) of `sheetName`, finds which of `headers` are absent, and writes
// them at the right edge. Idempotent: re-running with the same headers
// is a no-op. Returns the count of columns added.
export function appendColumns(sheetName: string, headers: string[]): number {
  const sheet = getActiveSpreadsheet().getSheetByName(sheetName);
  if (!sheet) {
    throw new Error(`appendColumns: sheet ${sheetName} missing — run scaffold first`);
  }
  const lastCol = sheet.getLastColumn();
  const existingHeaders = lastCol > 0
    ? sheet.getRange(1, 1, 1, lastCol).getValues()[0].map((c) => String(c ?? '').trim())
    : [];
  const toAdd = headers.filter((h) => !existingHeaders.includes(h));
  if (toAdd.length === 0) return 0;
  sheet.getRange(1, lastCol + 1, 1, toAdd.length).setValues([toAdd]);
  return toAdd.length;
}
