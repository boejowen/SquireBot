// buildView — Phase 3 plan 03-04 task 2.
//
// Full-snapshot rebuild of the consolidated `view` tab. Reads every
// inv:* tab, joins each row against _pigparse (preferring WTS price)
// and _item_master (for the wiki link). Writes to `view` in one atomic
// setValues + one setNotes per build, all inside LockService.
//
// Performance budget: 12 guildies × ~10 toons × ~150 rows × 8 cols
// ≈ 144,000 cells. Empirically ~5s for that volume per RESEARCH §11.
// Target: <10s per build. Debounce of 10s suppresses storms.

import { log } from '../lib/log';
import {
  getActiveSpreadsheet, writeMetaRow,
} from '../lib/sheet-helpers';
import { applyTheme, getActiveTheme } from '../lib/themes';
import { composeItemNote, type PigparsePriceRow } from './composeNotes';
import type { PigparseDirection } from '../lib/pigparse-types';

export const VIEW_TAB = 'view';
export const VIEW_HEADERS = [
  'Char', 'Slot', 'Item', 'ID', 'Count', 'Wiki', 'Price', 'Last Synced',
];
export const ITEM_COL_INDEX_1BASED = 3;       // for setNote target
export const LAST_SYNCED_COL_INDEX_1BASED = 8; // for conditional formatting
export const DEBOUNCE_MS = 10_000;
export const VIEW_LAST_BUILD_PROP = 'view_last_build_ms';
const LOCK_TIMEOUT_MS = 30_000;

interface InvRow {
  char: string;
  location: string;
  itemName: string;
  itemId: number;
  count: number;
  uploadedAt: string;
}

interface PigparseRow {
  itemId: number;
  direction: PigparseDirection;
  a30: number;
  t30: number;
}

interface ItemMasterRow {
  itemId: number;
  summary: string;
  wikiUrl: string;
  isQuestItem: boolean;
}

interface QuestLinkRow {
  itemId: number;
  questName: string;
  source: 'in_game_flag' | 'notes_link';
}

export function buildView(): void {
  const props = PropertiesService.getDocumentProperties();
  const lastBuild = parseInt(props.getProperty(VIEW_LAST_BUILD_PROP) ?? '0', 10);
  const now = Date.now();
  if (lastBuild > 0 && now - lastBuild < DEBOUNCE_MS) {
    log('debug', 'buildView', { skipped: 'debounced', sinceLastMs: now - lastBuild });
    return;
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(LOCK_TIMEOUT_MS)) {
    log('warn', 'buildView', { skipped: 'lock_busy' });
    return;
  }
  try {
    runBuild(now);
    props.setProperty(VIEW_LAST_BUILD_PROP, String(Date.now()));
  } finally {
    lock.releaseLock();
  }
}

function runBuild(startMs: number): void {
  const ss = getActiveSpreadsheet();
  const viewSheet = ss.getSheetByName(VIEW_TAB);
  if (!viewSheet) {
    log('warn', 'buildView', { skipped: 'view_sheet_missing — run scaffold/migrateToV2 first' });
    return;
  }

  const inv = readAllInventoryRows(ss);
  const pigparse = readPigparseRows(ss);
  const itemMaster = readItemMasterRows(ss);
  const questLinks = readQuestLinksRows(ss);

  // Sort: char asc then item asc.
  inv.sort((a, b) => {
    if (a.char !== b.char) return a.char < b.char ? -1 : 1;
    if (a.itemName !== b.itemName) return a.itemName < b.itemName ? -1 : 1;
    return a.location < b.location ? -1 : 1;
  });

  const dataRows: unknown[][] = [];
  const notes: (string | null)[][] = [];
  for (const row of inv) {
    const masterRow = itemMaster.get(row.itemId);
    const pigRows = pigparse.get(row.itemId) ?? [];
    const links = questLinks.get(row.itemId) ?? [];
    const wikiCell = masterRow?.wikiUrl
      ? `=HYPERLINK("${masterRow.wikiUrl}","wiki")`
      : '';
    const price = pickPrice(pigRows);
    // Convert ISO 8601 string to Date so Sheets can compute NOW() - cell
    // for conditional formatting. Falls back to original string if parse
    // fails (still readable, just won't trigger color rules).
    const lastSynced = parseToDate(row.uploadedAt);
    dataRows.push([
      row.char,
      row.location,
      row.itemName,
      row.itemId,
      row.count,
      wikiCell,
      price,
      lastSynced,
    ]);
    notes.push([
      composeItemNote(
        masterRow ? { summary: masterRow.summary, is_quest_item: masterRow.isQuestItem } : null,
        pigRows.map(toPriceRow),
        links.map(toLinkRow),
      ),
    ]);
  }

  // Clear prior data range below header (col-aware to VIEW_HEADERS width).
  const lastRow = viewSheet.getLastRow();
  if (lastRow > 1) {
    viewSheet.getRange(2, 1, lastRow - 1, VIEW_HEADERS.length).clearContent();
    viewSheet.getRange(2, ITEM_COL_INDEX_1BASED, lastRow - 1, 1).setNotes(
      Array.from({ length: lastRow - 1 }, () => [null]) as unknown as string[][],
    );
  }

  if (dataRows.length > 0) {
    viewSheet.getRange(2, 1, dataRows.length, VIEW_HEADERS.length).setValues(dataRows);
    viewSheet.getRange(2, ITEM_COL_INDEX_1BASED, dataRows.length, 1).setNotes(notes);
  }

  applyLastSyncedConditionalFormatting(viewSheet, dataRows.length);
  applyTheme(viewSheet, getActiveTheme());

  writeMetaRow('_status', 'last_view_build', new Date().toISOString());
  writeMetaRow('_status', 'last_view_row_count', String(dataRows.length));

  log('info', 'buildView', {
    rows: dataRows.length,
    chars: new Set(inv.map((r) => r.char)).size,
    items: itemMaster.size,
    durationMs: Date.now() - startMs,
  });
}

function readAllInventoryRows(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): InvRow[] {
  const out: InvRow[] = [];
  for (const sheet of ss.getSheets()) {
    const name = sheet.getName();
    if (!name.startsWith('inv:')) continue;
    const char = name.slice(4);
    const lastRow = sheet.getLastRow();
    if (lastRow < 2) continue;
    const values = sheet.getRange(2, 1, lastRow - 1, 6).getValues();
    for (const row of values) {
      const itemName = String(row[1] ?? '').trim();
      const idRaw = row[2];
      const id = typeof idRaw === 'number' ? idRaw : parseInt(String(idRaw ?? ''), 10);
      if (!itemName || !Number.isFinite(id) || id <= 0) continue;
      out.push({
        char,
        location: String(row[0] ?? '').trim(),
        itemName,
        itemId: id,
        count: typeof row[3] === 'number' ? row[3] : parseInt(String(row[3] ?? '0'), 10) || 0,
        uploadedAt: String(row[5] ?? '').trim(),
      });
    }
  }
  return out;
}

function readPigparseRows(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): Map<number, PigparseRow[]> {
  const out = new Map<number, PigparseRow[]>();
  const sheet = ss.getSheetByName('_pigparse');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  // 15 cols per migration; we only need item_id (col 1), direction (col 7),
  // a30 (col 9), t30 (col 8).
  const values = sheet.getRange(2, 1, lastRow - 1, 15).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    const direction = row[6] as PigparseDirection;
    if (!Number.isFinite(id) || id <= 0) continue;
    if (direction !== 0 && direction !== 1 && direction !== 2) continue;
    const t30 = typeof row[7] === 'number' ? row[7] : parseInt(String(row[7] ?? '0'), 10) || 0;
    const a30 = typeof row[8] === 'number' ? row[8] : parseFloat(String(row[8] ?? '0')) || 0;
    if (!out.has(id)) out.set(id, []);
    out.get(id)!.push({ itemId: id, direction, a30, t30 });
  }
  return out;
}

function readItemMasterRows(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): Map<number, ItemMasterRow> {
  const out = new Map<number, ItemMasterRow>();
  const sheet = ss.getSheetByName('_item_master');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  // 8 cols: item_id, name, wiki_summary, wiki_url, slot, is_quest_item, last_refreshed, wikitext_sha1
  const values = sheet.getRange(2, 1, lastRow - 1, 8).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    if (!Number.isFinite(id) || id <= 0) continue;
    out.set(id, {
      itemId: id,
      summary: String(row[2] ?? '').trim(),
      wikiUrl: String(row[3] ?? '').trim(),
      isQuestItem: parseTruthy(row[5]),
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
  // 5 cols: item_id, quest_name, source_url, last_refreshed, source
  const values = sheet.getRange(2, 1, lastRow - 1, 5).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    if (!Number.isFinite(id) || id <= 0) continue;
    const sourceRaw = String(row[4] ?? '').trim();
    const source: 'in_game_flag' | 'notes_link' =
      sourceRaw === 'in_game_flag' ? 'in_game_flag' : 'notes_link';
    if (!out.has(id)) out.set(id, []);
    out.get(id)!.push({
      itemId: id,
      questName: String(row[1] ?? '').trim(),
      source,
    });
  }
  return out;
}

// pickPrice prefers WTS (sell-side ask) → falls back to WTB (buy bid)
// → returns empty string if neither has a 30-day average. Matches
// RESEARCH §3's "Default" rule.
export function pickPrice(rows: PigparseRow[]): number | string {
  const wts = rows.find((r) => r.direction === 0);
  if (wts && wts.a30 > 0) return wts.a30;
  const wtb = rows.find((r) => r.direction === 1);
  if (wtb && wtb.a30 > 0) return wtb.a30;
  return '';
}

function applyLastSyncedConditionalFormatting(
  sheet: GoogleAppsScript.Spreadsheet.Sheet,
  dataRowCount: number,
): void {
  if (dataRowCount === 0) {
    sheet.setConditionalFormatRules([]);
    return;
  }
  const range = sheet.getRange(2, LAST_SYNCED_COL_INDEX_1BASED, dataRowCount, 1);
  // Use ISBLANK guard so empty cells don't trigger any rule. Date cells
  // subtract cleanly from NOW(). The rules evaluate top-down and stop
  // at the first match, so order is: green (most recent) → orange → red.
  const greenRule = SpreadsheetApp.newConditionalFormatRule()
    .whenFormulaSatisfied(`=AND(NOT(ISBLANK(H2)), NOW()-H2<7)`)
    .setBackground('#b7e1cd')
    .setRanges([range])
    .build();
  const orangeRule = SpreadsheetApp.newConditionalFormatRule()
    .whenFormulaSatisfied(`=AND(NOT(ISBLANK(H2)), NOW()-H2<30)`)
    .setBackground('#fce8b2')
    .setRanges([range])
    .build();
  const redRule = SpreadsheetApp.newConditionalFormatRule()
    .whenFormulaSatisfied(`=AND(NOT(ISBLANK(H2)), NOW()-H2>=30)`)
    .setBackground('#f4c7c3')
    .setRanges([range])
    .build();
  sheet.setConditionalFormatRules([greenRule, orangeRule, redRule]);
}

// parseToDate converts an ISO 8601 string to a Date. Returns the
// original string when parse fails so users still see something
// readable (and the conditional-format rules just don't fire on it).
function parseToDate(s: string): Date | string {
  if (!s) return '';
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? s : d;
}

function parseTruthy(v: unknown): boolean {
  if (v === true) return true;
  if (typeof v === 'string') return v.toUpperCase() === 'TRUE' || v === '1';
  if (typeof v === 'number') return v !== 0;
  return false;
}

function toPriceRow(p: PigparseRow): PigparsePriceRow {
  return { direction: p.direction, a30: p.a30, t30: p.t30 };
}
function toLinkRow(l: QuestLinkRow): { quest_name: string; source: 'in_game_flag' | 'notes_link' } {
  return { quest_name: l.questName, source: l.source };
}
