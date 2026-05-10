// buildSpellCheck — Phase 4 plan 04-02 task 4.
//
// Full-snapshot rebuild of the consolidated `spell_check` tab. Per
// character (with class+level set in _char_owner), reads spell:<Char>
// landing tab, joins against _wiki_spells filtered by class AND
// level<=char.level, emits one row per (char, level, spell) with
// Status = KNOWN | MISSING.
//
// Join key: normalized_name (lowercase + alphanumeric-only). The
// spellbook landing tab has no spell IDs (per Phase 1 file format), so
// name-based joining is the only option.
//
// Performance: 12 chars × ~200 spells per class = ~2,400 rows × 5 cols
// = 12,000 cells. Well within the 6-min trigger budget; <2s observed.

import { log } from '../lib/log';
import { getActiveSpreadsheet, writeMetaRow } from '../lib/sheet-helpers';
import { applyTheme, getActiveTheme } from '../lib/themes';
import { normalizeSpellName } from '../lib/wiki-spell-types';

export const SPELL_CHECK_TAB = 'spell_check';
export const SPELL_CHECK_HEADERS = ['Char', 'Class', 'Level', 'Spell', 'Status'];
export const DEBOUNCE_MS = 10_000;
export const SPELL_CHECK_LAST_BUILD_PROP = 'spell_check_last_build_ms';
const LOCK_TIMEOUT_MS = 30_000;

interface CharMetadata {
  char_name: string;
  class: string;
  level: number;
}

interface WikiSpell {
  class: string;
  level: number;
  spell_name: string;
  normalized_name: string;
}

export function buildSpellCheck(): void {
  const props = PropertiesService.getDocumentProperties();
  const lastBuild = parseInt(props.getProperty(SPELL_CHECK_LAST_BUILD_PROP) ?? '0', 10);
  const now = Date.now();
  if (lastBuild > 0 && now - lastBuild < DEBOUNCE_MS) {
    log('debug', 'buildSpellCheck', { skipped: 'debounced', sinceLastMs: now - lastBuild });
    return;
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(LOCK_TIMEOUT_MS)) {
    log('warn', 'buildSpellCheck', { skipped: 'lock_busy' });
    return;
  }
  try {
    runBuild(now);
    props.setProperty(SPELL_CHECK_LAST_BUILD_PROP, String(Date.now()));
  } finally {
    lock.releaseLock();
  }
}

function runBuild(startMs: number): void {
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(SPELL_CHECK_TAB);
  if (!sheet) {
    log('warn', 'buildSpellCheck', { skipped: 'sheet_missing — run scaffold first' });
    return;
  }

  const chars = readCharOwnerWithMetadata(ss);
  const wikiByClass = readWikiSpellsByClass(ss);
  const knownByChar = readSpellbooksByChar(ss);

  const dataRows: unknown[][] = [];
  let charsWithMetadata = 0;
  for (const c of chars) {
    if (!c.class || !Number.isFinite(c.level) || c.level < 1) continue;
    charsWithMetadata++;
    const wikiRows = wikiByClass.get(c.class) ?? [];
    const known = knownByChar.get(c.char_name) ?? new Set<string>();
    for (const w of wikiRows) {
      if (w.level > c.level) continue;
      const status = known.has(w.normalized_name) ? 'KNOWN' : 'MISSING';
      dataRows.push([c.char_name, c.class, w.level, w.spell_name, status]);
    }
  }

  // Sort: char asc → level asc → spell asc.
  dataRows.sort((a, b) => {
    const ca = String(a[0]); const cb = String(b[0]);
    if (ca !== cb) return ca < cb ? -1 : 1;
    const la = Number(a[2]); const lb = Number(b[2]);
    if (la !== lb) return la - lb;
    const sa = String(a[3]); const sb = String(b[3]);
    return sa < sb ? -1 : sa > sb ? 1 : 0;
  });

  // Clear prior data range.
  const lastRow = sheet.getLastRow();
  if (lastRow > 1) {
    sheet.getRange(2, 1, lastRow - 1, SPELL_CHECK_HEADERS.length).clearContent();
  }
  if (dataRows.length > 0) {
    sheet.getRange(2, 1, dataRows.length, SPELL_CHECK_HEADERS.length).setValues(dataRows);
  }

  applyTheme(sheet, getActiveTheme());

  writeMetaRow('_status', 'last_spell_check_build', new Date().toISOString());
  writeMetaRow('_status', 'last_spell_check_row_count', String(dataRows.length));

  log('info', 'buildSpellCheck', {
    rows: dataRows.length,
    charsWithMetadata,
    charsTotal: chars.length,
    durationMs: Date.now() - startMs,
  });
}

// readCharOwnerWithMetadata reads _char_owner and returns one entry per
// non-empty char_name row. Doesn't filter for class/level/race set —
// builder filters those itself so missing-metadata cases are observable.
function readCharOwnerWithMetadata(
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): CharMetadata[] {
  const sheet = ss.getSheetByName('_char_owner');
  if (!sheet) return [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 1) return [];
  // _char_owner: char_name=A, class=E, level=F, race=N. Read 14 cols.
  const values = sheet.getRange(1, 1, lastRow, 14).getValues();
  const out: CharMetadata[] = [];
  for (const r of values) {
    const charName = String(r[0] ?? '').trim();
    if (!charName || charName === 'char_name') continue;
    const cls = String(r[4] ?? '').trim();
    const lvlRaw = r[5];
    const lvl = typeof lvlRaw === 'number' ? lvlRaw : parseInt(String(lvlRaw ?? ''), 10);
    out.push({ char_name: charName, class: cls, level: Number.isFinite(lvl) ? lvl : 0 });
  }
  return out;
}

// readWikiSpellsByClass groups _wiki_spells rows by class. Schema (per
// scaffold.go): class | level | spell_name | normalized_name | last_refreshed.
function readWikiSpellsByClass(
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): Map<string, WikiSpell[]> {
  const out = new Map<string, WikiSpell[]>();
  const sheet = ss.getSheetByName('_wiki_spells');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  const values = sheet.getRange(2, 1, lastRow - 1, 5).getValues();
  for (const r of values) {
    const cls = String(r[0] ?? '').trim();
    const lvlRaw = r[1];
    const lvl = typeof lvlRaw === 'number' ? lvlRaw : parseInt(String(lvlRaw ?? ''), 10);
    const name = String(r[2] ?? '').trim();
    const normalized = String(r[3] ?? '').trim();
    if (!cls || !name || !Number.isFinite(lvl)) continue;
    if (!out.has(cls)) out.set(cls, []);
    out.get(cls)!.push({ class: cls, level: lvl, spell_name: name, normalized_name: normalized });
  }
  return out;
}

// readSpellbooksByChar iterates spell:* tabs and returns a Map of
// char_name → Set of normalized spell names known by that char.
//
// Spellbook landing tab schema (per Phase 2 scaffold + CLAUDE.md):
//   col A = Level (integer 1..60)
//   col B = Name (string)
//   col C = _uploaded_at (ISO 8601)
function readSpellbooksByChar(
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>();
  for (const sheet of ss.getSheets()) {
    const name = sheet.getName();
    if (!name.startsWith('spell:')) continue;
    const charName = name.slice(6);
    const lastRow = sheet.getLastRow();
    if (lastRow < 2) continue;
    const values = sheet.getRange(2, 1, lastRow - 1, 2).getValues();
    const known = new Set<string>();
    for (const r of values) {
      const spellName = String(r[1] ?? '').trim();
      if (!spellName) continue;
      known.add(normalizeSpellName(spellName));
    }
    out.set(charName, known);
  }
  return out;
}
