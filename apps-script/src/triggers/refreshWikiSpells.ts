// refreshWikiSpells — weekly Sunday ~04:30 PT P1999 per-class spell scrape.
//
// Phase 4 plan 04-02. Iterates the 14 P1999 classes; fetches each
// class's wiki page (e.g. /Necromancer); parses ==Level N== sections
// containing {{SpellRow|name=...}} templates; writes (class, level,
// spell_name, normalized_name) rows to _wiki_spells via per-class
// full-replace.
//
// RESUMABLE: clones refreshWikiItems's 5-min budget + 60s self-reschedule
// pattern. Overkill at 14 fetches × ~2s each ≈ 30s wall-clock — but
// consistent with the established pattern + safe overkill if a class
// page ever balloons.
//
// Inter-request 1s sleep between class fetches per ROADMAP SC-4. After
// a successful end-to-end run, calls buildSpellCheck() so users see
// fresh data without waiting for the next inv:* / spell:* change.
//
// The matching weekly time-driven trigger is installed by Phase 4 plan
// 04-04's updated installTriggers(). Function is also callable manually
// from the SquireBot menu's "Refresh Wiki Spells Now" item (added in
// plan 04-04 task 6).

import { log } from '../lib/log';
import { politeFetch } from '../lib/politeFetch';
import { parseClassPage } from '../lib/wiki-spell-parser';
import type { WikiSpellRow } from '../lib/wiki-spell-types';
import { CLASSES, CLASS_ABBREV_TO_DISPLAY, type ClassAbbrev } from '../lib/eq-constants';
import { getActiveSpreadsheet, writeMetaRow } from '../lib/sheet-helpers';
import { buildSpellCheck } from '../tabs/buildSpellCheck';

const WIKI_API_BASE = 'https://wiki.project1999.com/api.php';
const BUDGET_MS = 5 * 60 * 1000;       // 5 min wall-clock per trigger fire
const INTER_REQUEST_SLEEP_MS = 1000;
const FAILURE_THRESHOLD_PCT = 0.5;
const FAILURE_THRESHOLD_MIN_TOTAL = 7;  // half of 14 classes
const CURSOR_KEY = 'wiki_spells_refresh_cursor';
const RESUME_DELAY_MS = 60_000;
const HANDLER_NAME = 'refreshWikiSpells';

const WIKI_SPELLS_HEADERS = [
  'class', 'level', 'spell_name', 'normalized_name', 'last_refreshed',
];

interface ClassRef {
  abbrev: ClassAbbrev;
  display: string;
}

interface CursorState {
  remaining: ClassRef[];
  total: number;
  failures: number;
  successes: number;
  totalRowsWritten: number;
  started: string;
}

interface ErrorRecord {
  at: string;
  where: 'refreshWikiSpells';
  kind: 'fetch_failures_exceeded';
  detail: string;
}

export function refreshWikiSpells(): void {
  const startMs = Date.now();
  const props = PropertiesService.getDocumentProperties();
  const cursorRaw = props.getProperty(CURSOR_KEY);
  let state: CursorState;
  try {
    state = cursorRaw ? (JSON.parse(cursorRaw) as CursorState) : freshState();
  } catch (e) {
    log('warn', HANDLER_NAME, { recovered_from: 'cursor_parse_error', error: String(e) });
    state = freshState();
  }

  log('info', HANDLER_NAME, {
    resuming: !!cursorRaw,
    total: state.total,
    remaining: state.remaining.length,
    successes: state.successes,
    failures: state.failures,
  });

  while (state.remaining.length > 0) {
    if (Date.now() - startMs > BUDGET_MS) {
      checkpoint(state);
      log('info', HANDLER_NAME, { checkpoint: true, remaining: state.remaining.length });
      return;
    }

    const ref = state.remaining.shift()!;
    Utilities.sleep(INTER_REQUEST_SLEEP_MS);
    const outcome = processOne(ref);
    if (outcome.kind === 'success') {
      state.successes++;
      state.totalRowsWritten += outcome.rowsWritten;
    } else {
      state.failures++;
    }

    const processed = state.successes + state.failures;
    if (
      processed >= FAILURE_THRESHOLD_MIN_TOTAL
      && state.failures / processed > FAILURE_THRESHOLD_PCT
    ) {
      writeError({
        at: nowIso(),
        where: 'refreshWikiSpells',
        kind: 'fetch_failures_exceeded',
        detail: `failures=${state.failures} successes=${state.successes}`,
      });
      props.deleteProperty(CURSOR_KEY);
      cleanupResumeTriggers();
      log('warn', HANDLER_NAME, { aborted: 'failure_threshold', state });
      return;
    }
  }

  // All classes processed.
  const now = nowIso();
  writeMetaRow('_meta', 'last_wiki_spell_refresh', now);
  writeMetaRow('_status', 'last_wiki_spell_count', String(state.totalRowsWritten));
  clearError();
  props.deleteProperty(CURSOR_KEY);
  cleanupResumeTriggers();
  log('info', HANDLER_NAME, {
    done: true,
    successes: state.successes,
    failures: state.failures,
    totalRowsWritten: state.totalRowsWritten,
    durationSec: Math.round((Date.now() - startMs) / 1000),
  });

  // Trigger spell_check rebuild so users see fresh data immediately.
  // buildSpellCheck has its own lock + debounce — safe to invoke here.
  try {
    buildSpellCheck();
  } catch (e) {
    log('warn', HANDLER_NAME, { post_build_failed: String(e) });
  }
}

function freshState(): CursorState {
  const items: ClassRef[] = CLASSES.map((abbrev) => ({
    abbrev,
    display: CLASS_ABBREV_TO_DISPLAY[abbrev],
  }));
  return {
    remaining: items,
    total: items.length,
    failures: 0,
    successes: 0,
    totalRowsWritten: 0,
    started: nowIso(),
  };
}

function checkpoint(state: CursorState): void {
  const props = PropertiesService.getDocumentProperties();
  props.setProperty(CURSOR_KEY, JSON.stringify(state));
  ScriptApp.newTrigger(HANDLER_NAME).timeBased().after(RESUME_DELAY_MS).create();
}

type ClassOutcome =
  | { kind: 'success'; rowsWritten: number }
  | { kind: 'failure' };

function processOne(ref: ClassRef): ClassOutcome {
  const url = `${WIKI_API_BASE}?action=parse&prop=wikitext&format=json&page=${encodeURIComponent(ref.display.replace(/ /g, '_'))}&redirects=true`;
  const result = politeFetch(url);
  if (!result.ok) {
    log('warn', HANDLER_NAME, { class: ref.abbrev, fetch_failed: result.status });
    return { kind: 'failure' };
  }
  let json: { parse?: { wikitext?: { '*'?: string } }; error?: { code: string; info: string } };
  try {
    json = JSON.parse(result.body);
  } catch (e) {
    log('warn', HANDLER_NAME, { class: ref.abbrev, parse_failed: String(e) });
    return { kind: 'failure' };
  }
  if (json.error) {
    log('warn', HANDLER_NAME, { class: ref.abbrev, wiki_error: json.error.code });
    return { kind: 'failure' };
  }
  const wikitext = json.parse?.wikitext?.['*'];
  if (!wikitext) {
    log('warn', HANDLER_NAME, { class: ref.abbrev, empty_wikitext: true });
    return { kind: 'failure' };
  }
  const parsed = parseClassPage(wikitext, ref.abbrev);
  if (!parsed.ok) {
    log('warn', HANDLER_NAME, { class: ref.abbrev, parse_reason: parsed.reason });
    return { kind: 'failure' };
  }

  // Write all rows for this class — full replace per-class.
  // (Warrior degenerate case: rows.length === 0 — replace deletes any
  // stale entries from a prior run when the class page format changes.)
  replaceWikiSpellsForClass(ref.abbrev, parsed.rows);
  return { kind: 'success', rowsWritten: parsed.rows.length };
}

function replaceWikiSpellsForClass(classAbbrev: string, rows: WikiSpellRow[]): void {
  const sheet = getActiveSpreadsheet().getSheetByName('_wiki_spells');
  if (!sheet) throw new Error('_wiki_spells sheet missing — run scaffold first');
  const lastRow = sheet.getLastRow();
  if (lastRow >= 2) {
    const classCol = sheet.getRange(2, 1, lastRow - 1, 1).getValues();
    // Bottom-up delete so indices don't shift on rows we still need.
    for (let i = classCol.length - 1; i >= 0; i--) {
      if (String(classCol[i][0] ?? '').trim() === classAbbrev) {
        sheet.deleteRow(i + 2);
      }
    }
  }
  if (rows.length === 0) return;
  const dataRows = rows.map((r) => [
    r.class, r.level, r.spell_name, r.normalized_name, r.last_refreshed,
  ]);
  const writeStartRow = sheet.getLastRow() + 1;
  sheet.getRange(writeStartRow, 1, dataRows.length, WIKI_SPELLS_HEADERS.length).setValues(dataRows);
}

function cleanupResumeTriggers(): void {
  // Delete any one-shot resume triggers we created beyond the recurring
  // weekly one. By convention installTriggers creates the recurring
  // trigger first; this loop preserves it and prunes extras.
  const triggers = ScriptApp.getProjectTriggers().filter(
    (t) => t.getHandlerFunction() === HANDLER_NAME,
  );
  if (triggers.length <= 1) return;
  for (let i = 1; i < triggers.length; i++) {
    ScriptApp.deleteTrigger(triggers[i]);
  }
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
