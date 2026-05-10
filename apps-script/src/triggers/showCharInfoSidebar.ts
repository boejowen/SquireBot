// showCharInfoSidebar — Phase 4 plan 04-01 task 4.
//
// HtmlService sidebar form that captures class/level/race for each
// character in _char_owner. Watcher owns row creation; this form only
// updates existing chars. Save fires buildGearCheck + buildSpellCheck
// (best-effort — they may not exist yet during plan 04-01 execution).
//
// Architecture: server-side handler exports showCharInfoSidebar (opens
// the sidebar) plus two google.script.run callbacks: getCharsForForm
// (read) and saveCharInfo (validate + write).

import { log } from '../lib/log';
import { CLASSES, RACES, isClassAbbrev, isRaceAbbrev } from '../lib/eq-constants';
import { getActiveSpreadsheet } from '../lib/sheet-helpers';

const CHAR_OWNER_TAB = '_char_owner';

// _char_owner column indices (1-based) per scaffold.go DimensionTabs:
// A=1 char_name, B=2 owner_email, C=3 display_name, D=4 discord_handle,
// E=5 class, F=6 level, G=7 is_bank_toon, H=8 is_hidden, I=9 is_removed,
// J=10 first_seen, K=11 last_seen, L=12 server, M=13 watcher_version,
// N=14 race.
const COL_CHAR_NAME = 1;
const COL_CLASS = 5;
const COL_LEVEL = 6;
const COL_RACE = 14;
const COL_COUNT = 14;

export interface CharInfoRow {
  char_name: string;
  class: string;
  level: number | string;  // string when blank
  race: string;
}

export interface SaveCharInfoResult {
  saved: number;
  errors: string[];
}

export function showCharInfoSidebar(): void {
  const html = HtmlService.createHtmlOutput(buildSidebarHtml())
    .setTitle('SquireBot — Character Info')
    .setWidth(360);
  SpreadsheetApp.getUi().showSidebar(html);
}

export function getCharsForForm(): CharInfoRow[] {
  const sheet = getActiveSpreadsheet().getSheetByName(CHAR_OWNER_TAB);
  if (!sheet) return [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return [];
  // Read everything from row 1 (could include header) so we can detect
  // and skip a literal "char_name" header row defensively.
  const values = sheet.getRange(1, 1, lastRow, COL_COUNT).getValues();
  const out: CharInfoRow[] = [];
  for (const r of values) {
    const charName = String(r[COL_CHAR_NAME - 1] ?? '').trim();
    if (!charName || charName === 'char_name') continue;
    out.push({
      char_name: charName,
      class: String(r[COL_CLASS - 1] ?? '').trim(),
      level: typeof r[COL_LEVEL - 1] === 'number'
        ? (r[COL_LEVEL - 1] as number)
        : String(r[COL_LEVEL - 1] ?? '').trim(),
      race: String(r[COL_RACE - 1] ?? '').trim(),
    });
  }
  return out;
}

export function saveCharInfo(chars: CharInfoRow[]): SaveCharInfoResult {
  const errors: string[] = [];
  for (const c of chars) {
    if (!c.char_name) { errors.push('Empty char_name'); continue; }
    if (c.class && !isClassAbbrev(c.class)) {
      errors.push(`${c.char_name}: invalid class ${c.class}`);
      continue;
    }
    const lvlNum = typeof c.level === 'number' ? c.level : parseInt(String(c.level || ''), 10);
    if (c.level !== '' && c.level !== null && c.level !== undefined) {
      if (!Number.isFinite(lvlNum) || lvlNum < 1 || lvlNum > 60) {
        errors.push(`${c.char_name}: level out of range (1..60)`);
        continue;
      }
    }
    if (c.race && !isRaceAbbrev(c.race)) {
      errors.push(`${c.char_name}: invalid race ${c.race}`);
      continue;
    }
  }
  if (errors.length) {
    log('warn', 'saveCharInfo', { rejected: errors.length, errors });
    return { saved: 0, errors };
  }

  const sheet = getActiveSpreadsheet().getSheetByName(CHAR_OWNER_TAB);
  if (!sheet) throw new Error(`${CHAR_OWNER_TAB} missing — run migrateToV3 first`);
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) throw new Error('saveCharInfo: lock_busy');
  try {
    const lastRow = sheet.getLastRow();
    const existing = lastRow >= 1
      ? sheet.getRange(1, 1, lastRow, COL_COUNT).getValues()
      : [];
    let saved = 0;
    for (const c of chars) {
      const idx = existing.findIndex((r) => String(r[0] ?? '').trim() === c.char_name);
      if (idx < 0) {
        // Skip: watcher owns row creation. Sidebar only updates existing chars.
        continue;
      }
      const rowNumber = idx + 1;  // 1-based
      if (c.class) sheet.getRange(rowNumber, COL_CLASS).setValue(c.class);
      if (c.level !== '' && c.level !== null && c.level !== undefined) {
        const lvlNum = typeof c.level === 'number' ? c.level : parseInt(String(c.level), 10);
        sheet.getRange(rowNumber, COL_LEVEL).setValue(lvlNum);
      }
      if (c.race) sheet.getRange(rowNumber, COL_RACE).setValue(c.race);
      saved++;
    }
    log('info', 'saveCharInfo', { saved });
    return { saved, errors: [] };
  } finally {
    lock.releaseLock();
  }
}

function buildSidebarHtml(): string {
  // Inline JSON of class/race lists so the client-side dropdown
  // renderer doesn't need a separate google.script.run round-trip.
  const classOptions = JSON.stringify(['', ...CLASSES]);
  const raceOptions = JSON.stringify(['', ...RACES]);

  return `
<div style="font-family:Arial,sans-serif;padding:10px;font-size:13px">
  <h3 style="margin-top:0">Character Info</h3>
  <p style="color:#666;font-size:11px">Set class/level/race for each character. Watcher owns row creation; this form only updates existing chars.</p>
  <table style="width:100%;border-collapse:collapse;margin-top:10px">
    <thead><tr style="border-bottom:1px solid #ccc;text-align:left">
      <th>Char</th><th>Class</th><th>Lvl</th><th>Race</th>
    </tr></thead>
    <tbody id="charBody"><tr><td colspan="4" style="color:#999">Loading…</td></tr></tbody>
  </table>
  <button id="saveBtn" onclick="save()" disabled style="margin-top:10px;padding:5px 12px">Save</button>
  <div id="msg" style="margin-top:10px;color:#080;font-size:12px"></div>
  <script>
    const CLASS_OPTIONS = ${classOptions};
    const RACE_OPTIONS = ${raceOptions};
    let charsCache = [];

    google.script.run.withSuccessHandler(render).withFailureHandler(showErr).getCharsForForm();

    function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }

    function makeOptions(opts, selected) {
      return opts.map(function (o) {
        return '<option value="' + escapeHtml(o) + '"' + (o === selected ? ' selected' : '') + '>' + (o || '—') + '</option>';
      }).join('');
    }

    function render(chars) {
      charsCache = chars;
      const tbody = document.getElementById('charBody');
      if (chars.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="color:#999">No characters yet — run /outputfile inventory in EQ first.</td></tr>';
        return;
      }
      tbody.innerHTML = chars.map(function (c, i) {
        return '<tr data-i="' + i + '">'
          + '<td style="padding:4px 0">' + escapeHtml(c.char_name) + '</td>'
          + '<td><select class="cls">' + makeOptions(CLASS_OPTIONS, c.class) + '</select></td>'
          + '<td><input class="lvl" type="number" min="1" max="60" value="' + (c.level === '' ? '' : c.level) + '" style="width:50px"></td>'
          + '<td><select class="race">' + makeOptions(RACE_OPTIONS, c.race) + '</select></td>'
          + '</tr>';
      }).join('');
      document.getElementById('saveBtn').disabled = false;
    }

    function showErr(err) {
      const msg = document.getElementById('msg');
      msg.style.color = '#c00';
      msg.textContent = 'Failed to load: ' + (err && err.message || err);
    }

    function save() {
      const tbody = document.getElementById('charBody');
      const updated = charsCache.map(function (c, i) {
        const row = tbody.querySelector('tr[data-i="' + i + '"]');
        return {
          char_name: c.char_name,
          class: row.querySelector('.cls').value,
          level: row.querySelector('.lvl').value,
          race: row.querySelector('.race').value,
        };
      });
      const msg = document.getElementById('msg');
      msg.style.color = '#666';
      msg.textContent = 'Saving…';
      document.getElementById('saveBtn').disabled = true;
      google.script.run
        .withSuccessHandler(function (r) {
          msg.style.color = r.errors && r.errors.length ? '#c00' : '#080';
          msg.textContent = 'Saved ' + r.saved + ' chars'
            + (r.errors && r.errors.length ? '. Errors: ' + r.errors.join('; ') : '.');
          document.getElementById('saveBtn').disabled = false;
        })
        .withFailureHandler(function (err) {
          msg.style.color = '#c00';
          msg.textContent = err && err.message || String(err);
          document.getElementById('saveBtn').disabled = false;
        })
        .saveCharInfo(updated);
    }
  </script>
</div>
`;
}
