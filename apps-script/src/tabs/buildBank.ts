// buildBank — Phase 3 plan 03-04 task 3.
//
// Same shape as buildView but limited to the inv:<bank_toon_name> tab.
// Bank toon name read once from _meta.bank_toon_name; if empty, logs and
// returns. Coin sidebar is Phase 4 — Phase 3 only ships the inventory
// half of bank.

import { log } from '../lib/log';
import {
  getActiveSpreadsheet, readMetaRows, writeMetaRow,
} from '../lib/sheet-helpers';
import { applyTheme, getActiveTheme } from '../lib/themes';
import { composeItemNote } from './composeNotes';
import type { PigparseDirection } from '../lib/pigparse-types';

export const BANK_TAB = 'bank';
export const BANK_HEADERS = [
  'Char', 'Slot', 'Item', 'ID', 'Count', 'Wiki', 'Price', 'Last Synced',
];
const ITEM_COL = 3;
const LAST_SYNCED_COL = 8;
const LOCK_TIMEOUT_MS = 30_000;

export function buildBank(): void {
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(LOCK_TIMEOUT_MS)) {
    log('warn', 'buildBank', { skipped: 'lock_busy' });
    return;
  }
  try {
    runBuild();
  } finally {
    lock.releaseLock();
  }
}

function runBuild(): void {
  const startMs = Date.now();
  const ss = getActiveSpreadsheet();
  const bankSheet = ss.getSheetByName(BANK_TAB);
  if (!bankSheet) {
    log('warn', 'buildBank', { skipped: 'bank_sheet_missing' });
    return;
  }

  const meta = readMetaRows('_meta');
  const bankToon = meta.find((r) => r.key === 'bank_toon_name')?.value.trim() ?? '';
  if (!bankToon) {
    log('info', 'buildBank', { skipped: 'bank_toon_name_unset' });
    // Clear stale data if any, leave header.
    clearDataRows(bankSheet);
    return;
  }

  const invSheet = ss.getSheetByName(`inv:${bankToon}`);
  if (!invSheet) {
    log('warn', 'buildBank', { skipped: 'bank_inv_sheet_missing', bankToon });
    clearDataRows(bankSheet);
    return;
  }

  const inv = readBankInventory(invSheet);
  const pigparse = readPigparseRows(ss);
  const itemMaster = readItemMasterRows(ss);
  const questLinks = readQuestLinksRows(ss);

  inv.sort((a, b) => {
    if (a.itemName !== b.itemName) return a.itemName < b.itemName ? -1 : 1;
    return a.location < b.location ? -1 : 1;
  });

  const dataRows: unknown[][] = [];
  const notes: (string | null)[][] = [];
  for (const row of inv) {
    const masterRow = itemMaster.get(row.itemId);
    const pigRows = pigparse.get(row.itemId) ?? [];
    const links = questLinks.get(row.itemId) ?? [];
    dataRows.push([
      bankToon,
      row.location,
      row.itemName,
      row.itemId,
      row.count,
      masterRow?.wikiUrl ? `=HYPERLINK("${masterRow.wikiUrl}","wiki")` : '',
      pickPrice(pigRows),
      row.uploadedAt,
    ]);
    notes.push([
      composeItemNote(
        masterRow ? { summary: masterRow.summary, is_quest_item: masterRow.isQuestItem } : null,
        pigRows.map((p) => ({ direction: p.direction, a30: p.a30, t30: p.t30 })),
        links.map((l) => ({ quest_name: l.questName, source: l.source })),
      ),
    ]);
  }

  clearDataRows(bankSheet);
  if (dataRows.length > 0) {
    bankSheet.getRange(2, 1, dataRows.length, BANK_HEADERS.length).setValues(dataRows);
    bankSheet.getRange(2, ITEM_COL, dataRows.length, 1).setNotes(notes);
  }

  applyTheme(bankSheet, getActiveTheme());
  writeMetaRow('_status', 'last_bank_build', new Date().toISOString());
  writeMetaRow('_status', 'last_bank_row_count', String(dataRows.length));
  log('info', 'buildBank', { bankToon, rows: dataRows.length, durationMs: Date.now() - startMs });
}

function clearDataRows(sheet: GoogleAppsScript.Spreadsheet.Sheet): void {
  const lastRow = sheet.getLastRow();
  if (lastRow > 1) {
    sheet.getRange(2, 1, lastRow - 1, BANK_HEADERS.length).clearContent();
    sheet.getRange(2, ITEM_COL, lastRow - 1, 1).setNotes(
      Array.from({ length: lastRow - 1 }, () => [null]) as unknown as string[][],
    );
  }
}

interface InvRow {
  location: string;
  itemName: string;
  itemId: number;
  count: number;
  uploadedAt: string;
}

function readBankInventory(sheet: GoogleAppsScript.Spreadsheet.Sheet): InvRow[] {
  const out: InvRow[] = [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  const values = sheet.getRange(2, 1, lastRow - 1, 6).getValues();
  for (const row of values) {
    const itemName = String(row[1] ?? '').trim();
    const idRaw = row[2];
    const id = typeof idRaw === 'number' ? idRaw : parseInt(String(idRaw ?? ''), 10);
    if (!itemName || !Number.isFinite(id) || id <= 0) continue;
    out.push({
      location: String(row[0] ?? '').trim(),
      itemName,
      itemId: id,
      count: typeof row[3] === 'number' ? row[3] : parseInt(String(row[3] ?? '0'), 10) || 0,
      uploadedAt: String(row[5] ?? '').trim(),
    });
  }
  return out;
}

interface PigparseRow {
  direction: PigparseDirection;
  a30: number;
  t30: number;
}
interface ItemMasterRow {
  summary: string;
  wikiUrl: string;
  isQuestItem: boolean;
}
interface QuestLinkRow {
  questName: string;
  source: 'in_game_flag' | 'notes_link';
}

function readPigparseRows(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): Map<number, PigparseRow[]> {
  const out = new Map<number, PigparseRow[]>();
  const sheet = ss.getSheetByName('_pigparse');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  const values = sheet.getRange(2, 1, lastRow - 1, 15).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    const direction = row[6] as PigparseDirection;
    if (!Number.isFinite(id) || id <= 0) continue;
    if (direction !== 0 && direction !== 1 && direction !== 2) continue;
    const t30 = typeof row[7] === 'number' ? row[7] : parseInt(String(row[7] ?? '0'), 10) || 0;
    const a30 = typeof row[8] === 'number' ? row[8] : parseFloat(String(row[8] ?? '0')) || 0;
    if (!out.has(id)) out.set(id, []);
    out.get(id)!.push({ direction, a30, t30 });
  }
  return out;
}

function readItemMasterRows(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): Map<number, ItemMasterRow> {
  const out = new Map<number, ItemMasterRow>();
  const sheet = ss.getSheetByName('_item_master');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  const values = sheet.getRange(2, 1, lastRow - 1, 8).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    if (!Number.isFinite(id) || id <= 0) continue;
    const isQuestRaw = row[5];
    const isQuestItem = isQuestRaw === true
      || (typeof isQuestRaw === 'string' && isQuestRaw.toUpperCase() === 'TRUE');
    out.set(id, {
      summary: String(row[2] ?? '').trim(),
      wikiUrl: String(row[3] ?? '').trim(),
      isQuestItem,
    });
  }
  return out;
}

function readQuestLinksRows(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): Map<number, QuestLinkRow[]> {
  const out = new Map<number, QuestLinkRow[]>();
  const sheet = ss.getSheetByName('_quest_items');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  const values = sheet.getRange(2, 1, lastRow - 1, 5).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    if (!Number.isFinite(id) || id <= 0) continue;
    const sourceRaw = String(row[4] ?? '').trim();
    const source: 'in_game_flag' | 'notes_link' =
      sourceRaw === 'in_game_flag' ? 'in_game_flag' : 'notes_link';
    if (!out.has(id)) out.set(id, []);
    out.get(id)!.push({
      questName: String(row[1] ?? '').trim(),
      source,
    });
  }
  return out;
}

function pickPrice(rows: PigparseRow[]): number | string {
  const wts = rows.find((r) => r.direction === 0);
  if (wts && wts.a30 > 0) return wts.a30;
  const wtb = rows.find((r) => r.direction === 1);
  if (wtb && wtb.a30 > 0) return wtb.a30;
  return '';
}
