// Theme registry for the consolidated view + bank tabs. Phase 3 plan
// 03-01 task 5 ships the registry + applyTheme + clearTheme + setTheme;
// the polished 6-tile picker UI is deferred to Phase 5 (Phase 3 ships
// only the minimal modal-with-links stub in plan 03-04 task 5).
//
// Color palettes derived from docs/design/eq-aesthetic-theme.md.
// 'sheets-default' is the no-styling sentinel — applyTheme is a no-op
// after clearTheme, so the workbook returns to native Sheets defaults.

import { log } from './log';
import { readMetaRows, writeMetaRow } from './sheet-helpers';

export type ThemeKey =
  | 'vanilla'
  | 'kunark'
  | 'velious'
  | 'minimalist'
  | 'heavy'
  | 'sheets-default';

export const THEME_KEYS: ThemeKey[] = [
  'vanilla', 'kunark', 'velious', 'minimalist', 'heavy', 'sheets-default',
];

export interface Theme {
  headerBg: string;
  headerFg: string;
  rowAltBg: string;
  rowFg: string;
  fontFamily: string;
  accentBg: string;
  accentFg: string;
}

export const THEMES: Record<ThemeKey, Theme | null> = {
  vanilla: {
    headerBg: '#3a2616', headerFg: '#d4af37',
    rowAltBg: '#fdf6e3', rowFg: '#2a1810',
    fontFamily: 'Cinzel, serif',
    accentBg: '#8b6f1f', accentFg: '#fdf6e3',
  },
  kunark: {
    headerBg: '#1f3a1f', headerFg: '#b87333',
    rowAltBg: '#f1ede0', rowFg: '#1a2a1a',
    fontFamily: 'Cinzel, serif',
    accentBg: '#6b4423', accentFg: '#f1ede0',
  },
  velious: {
    headerBg: '#1a3a4a', headerFg: '#c0c0c0',
    rowAltBg: '#e8eef5', rowFg: '#0e2230',
    fontFamily: 'Cinzel Decorative, serif',
    accentBg: '#4a7090', accentFg: '#e8eef5',
  },
  minimalist: {
    headerBg: '#f5f5f5', headerFg: '#222222',
    rowAltBg: '#fafafa', rowFg: '#222222',
    fontFamily: 'Inter, Arial, sans-serif',
    accentBg: '#e0e0e0', accentFg: '#222222',
  },
  heavy: {
    headerBg: '#3a2a1a', headerFg: '#e8d4a8',
    rowAltBg: '#f4ead0', rowFg: '#2a1f10',
    fontFamily: 'MedievalSharp, serif',
    accentBg: '#8b6914', accentFg: '#f4ead0',
  },
  'sheets-default': null,
};

export const DEFAULT_THEME: ThemeKey = 'minimalist';

export function isThemeKey(s: unknown): s is ThemeKey {
  return typeof s === 'string' && (THEME_KEYS as string[]).includes(s);
}

export function getActiveTheme(): ThemeKey {
  const rows = readMetaRows('_meta');
  const row = rows.find((r) => r.key === 'theme');
  if (!row) return DEFAULT_THEME;
  return isThemeKey(row.value) ? row.value : DEFAULT_THEME;
}

// clearTheme resets fonts/colors/borders on the given range to Sheets
// defaults. Required when switching themes — without this, residual
// styling from a prior theme would bleed through (e.g. switching
// Velious → Sheets default would leave icy-blue headers).
export function clearTheme(range: GoogleAppsScript.Spreadsheet.Range): void {
  range
    .setFontFamily(null as unknown as string)
    .setBackground(null as unknown as string)
    .setFontColor(null as unknown as string)
    .setFontWeight(null as unknown as GoogleAppsScript.Spreadsheet.FontWeight);
  range.setBorder(false, false, false, false, false, false);
}

// applyTheme styles a sheet's used range with the given theme. For
// 'sheets-default' the function clears existing styling and returns
// without applying anything new — the sheet renders with native Sheets
// defaults.
export function applyTheme(
  sheet: GoogleAppsScript.Spreadsheet.Sheet,
  key: ThemeKey,
): void {
  const lastRow = sheet.getLastRow();
  const lastCol = sheet.getLastColumn();
  if (lastRow < 1 || lastCol < 1) return;
  const fullRange = sheet.getRange(1, 1, lastRow, lastCol);
  clearTheme(fullRange);

  const theme = THEMES[key];
  if (!theme) return; // sheets-default: clear-only

  const headerRange = sheet.getRange(1, 1, 1, lastCol);
  headerRange
    .setBackground(theme.headerBg)
    .setFontColor(theme.headerFg)
    .setFontFamily(theme.fontFamily)
    .setFontWeight('bold');

  if (lastRow >= 2) {
    const dataRange = sheet.getRange(2, 1, lastRow - 1, lastCol);
    dataRange
      .setFontFamily(theme.fontFamily)
      .setFontColor(theme.rowFg);
  }
}

// setTheme writes the chosen theme to _meta.theme. Plan 03-04's
// onChange handler will pick up the change on next rebuild; this
// function does NOT itself rebuild views — keeps it cheap to call
// from the theme picker modal which already triggers an onChange via
// the _meta write.
export function setTheme(key: ThemeKey): void {
  if (!isThemeKey(key)) {
    throw new Error(`setTheme: unknown theme key ${String(key)}`);
  }
  writeMetaRow('_meta', 'theme', key);
  log('info', 'setTheme', { key });
}
