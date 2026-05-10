// refreshWikiGearTier — weekly Sunday ~05:00 PT P1999 Velious gear-tier scrape.
//
// Phase 4 plan 04-03. Fetches the 2 Velious gear-tier wiki pages
// (Pre-Raid + Raiding); parses each via parseGearTierPage; emits rows
// to _wiki_gear_tier with Iksar items tagged tier='Iksar' (parser
// handles this via name pattern detection — see 04-RESEARCH.md §1a).
//
// Replacement strategy: full clear+rewrite at end of run since the 2
// pages collectively define the entire _wiki_gear_tier content.
// Partial-page failures (only 1 of 2 fetches succeeds) write an error
// + abort without clobbering existing data.
//
// Cursor pattern preserved for consistency with refreshWikiItems +
// refreshWikiSpells, but at 2 fetches × ~2s each ≈ 4s wall-clock,
// checkpoint is essentially never triggered.
//
// After successful end-to-end run: writes _meta.last_wiki_gear_refresh +
// _status.last_wiki_gear_count, clears _meta.last_error, and calls
// buildGearCheck() so users see fresh data immediately.

import { log } from '../lib/log';
import { politeFetch } from '../lib/politeFetch';
import { parseGearTierPage } from '../lib/wiki-gear-tier-parser';
import type { WikiGearTierRow } from '../lib/wiki-gear-tier-types';
import { getActiveSpreadsheet, writeMetaRow } from '../lib/sheet-helpers';
import { buildGearCheck } from '../tabs/buildGearCheck';

const WIKI_API_BASE = 'https://wiki.project1999.com/api.php';
const BUDGET_MS = 5 * 60 * 1000;
const INTER_REQUEST_SLEEP_MS = 1000;
const CURSOR_KEY = 'wiki_gear_refresh_cursor';
const RESUME_DELAY_MS = 60_000;
const HANDLER_NAME = 'refreshWikiGearTier';

const WIKI_GEAR_TIER_HEADERS = [
  'tier', 'class', 'slot', 'item_id', 'item_name', 'rank', 'last_refreshed',
];

// The 2 source pages. Tier label is the BASE — parser may override
// individual rows to tier='Iksar' for Iksar-prefixed items on the
// Pre-Raid page.
const SOURCES: Array<{
  pageName: string;
  tier: 'Velious Pre-Raid/Group' | 'Velious Raiding';
}> = [
  { pageName: 'Players:Velious Pre-Raid Gear', tier: 'Velious Pre-Raid/Group' },
  { pageName: 'Players:Velious Raiding Gear', tier: 'Velious Raiding' },
];

interface CursorState {
  remaining: typeof SOURCES;
  total: number;
  failures: number;
  successes: number;
  // Accumulated rows across multi-run cursor cycles. JSON-serialized
  // when checkpointing — at ~1000 rows this is ~100KB which is well
  // under PropertiesService's 9KB-per-property limit. Need to monitor;
  // if rows balloon, switch to per-source cursor + per-source replace
  // (instead of all-at-end replace).
  collectedRows: WikiGearTierRow[];
  unknownSlots: string[];
  started: string;
}

interface ErrorRecord {
  at: string;
  where: 'refreshWikiGearTier';
  kind: 'fetch_failed' | 'parse_failed' | 'partial_failure' | 'unknown_slots';
  detail: string;
}

export function refreshWikiGearTier(): void {
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

    const src = state.remaining.shift()!;
    Utilities.sleep(INTER_REQUEST_SLEEP_MS);
    const outcome = processOne(src);
    if (outcome.kind === 'success') {
      state.successes++;
      state.collectedRows.push(...outcome.rows);
      for (const us of outcome.unknownSlots) {
        if (!state.unknownSlots.includes(us)) state.unknownSlots.push(us);
      }
    } else {
      state.failures++;
    }
  }

  // All sources processed. Decide whether to commit:
  // - 2/2 success → commit
  // - 1/2 success → partial; abort with error, do NOT clobber existing data
  // - 0/2 success → abort with error
  if (state.failures > 0) {
    writeError({
      at: nowIso(),
      where: 'refreshWikiGearTier',
      kind: state.successes > 0 ? 'partial_failure' : 'fetch_failed',
      detail: `successes=${state.successes} failures=${state.failures}`,
    });
    props.deleteProperty(CURSOR_KEY);
    cleanupResumeTriggers();
    log('warn', HANDLER_NAME, { aborted: true, state });
    return;
  }

  // All-or-nothing replace.
  replaceAllWikiGearTier(state.collectedRows);

  const now = nowIso();
  writeMetaRow('_meta', 'last_wiki_gear_refresh', now);
  writeMetaRow('_status', 'last_wiki_gear_count', String(state.collectedRows.length));

  if (state.unknownSlots.length > 0) {
    // Surface unknown slots as a non-fatal warning. Doesn't abort the
    // commit (data is still useful); just a heads-up that slot vocab
    // expanded on the wiki side.
    writeError({
      at: now,
      where: 'refreshWikiGearTier',
      kind: 'unknown_slots',
      detail: `Unmapped wiki slot labels (will appear in gear_check but won't match inv): ${state.unknownSlots.join(', ')}`,
    });
  } else {
    clearError();
  }

  props.deleteProperty(CURSOR_KEY);
  cleanupResumeTriggers();
  log('info', HANDLER_NAME, {
    done: true,
    successes: state.successes,
    rowCount: state.collectedRows.length,
    iksarCount: state.collectedRows.filter((r) => r.tier === 'Iksar').length,
    unknownSlots: state.unknownSlots,
    durationSec: Math.round((Date.now() - startMs) / 1000),
  });

  // Trigger gear_check rebuild so users see fresh data immediately.
  try {
    buildGearCheck();
  } catch (e) {
    log('warn', HANDLER_NAME, { post_build_failed: String(e) });
  }
}

function freshState(): CursorState {
  return {
    remaining: SOURCES.slice(),
    total: SOURCES.length,
    failures: 0,
    successes: 0,
    collectedRows: [],
    unknownSlots: [],
    started: nowIso(),
  };
}

function checkpoint(state: CursorState): void {
  const props = PropertiesService.getDocumentProperties();
  props.setProperty(CURSOR_KEY, JSON.stringify(state));
  ScriptApp.newTrigger(HANDLER_NAME).timeBased().after(RESUME_DELAY_MS).create();
}

type SourceOutcome =
  | { kind: 'success'; rows: WikiGearTierRow[]; unknownSlots: string[] }
  | { kind: 'failure' };

function processOne(src: typeof SOURCES[number]): SourceOutcome {
  const url = `${WIKI_API_BASE}?action=parse&prop=wikitext&format=json&page=${encodeURIComponent(src.pageName.replace(/ /g, '_'))}&redirects=true`;
  const result = politeFetch(url);
  if (!result.ok) {
    log('warn', HANDLER_NAME, { source: src.pageName, fetch_failed: result.status });
    return { kind: 'failure' };
  }
  let json: { parse?: { wikitext?: { '*'?: string } }; error?: { code: string; info: string } };
  try {
    json = JSON.parse(result.body);
  } catch (e) {
    log('warn', HANDLER_NAME, { source: src.pageName, parse_failed: String(e) });
    return { kind: 'failure' };
  }
  if (json.error) {
    log('warn', HANDLER_NAME, { source: src.pageName, wiki_error: json.error.code });
    return { kind: 'failure' };
  }
  const wikitext = json.parse?.wikitext?.['*'];
  if (!wikitext) {
    log('warn', HANDLER_NAME, { source: src.pageName, empty_wikitext: true });
    return { kind: 'failure' };
  }
  const parsed = parseGearTierPage(wikitext, src.tier);
  if (!parsed.ok) {
    log('warn', HANDLER_NAME, { source: src.pageName, parse_reason: parsed.reason });
    return { kind: 'failure' };
  }
  return { kind: 'success', rows: parsed.rows, unknownSlots: parsed.unknownSlots };
}

function replaceAllWikiGearTier(rows: WikiGearTierRow[]): void {
  const sheet = getActiveSpreadsheet().getSheetByName('_wiki_gear_tier');
  if (!sheet) throw new Error('_wiki_gear_tier sheet missing — run scaffold first');
  const lastRow = sheet.getLastRow();
  if (lastRow > 1) {
    sheet.getRange(2, 1, lastRow - 1, WIKI_GEAR_TIER_HEADERS.length).clearContent();
  }
  if (rows.length === 0) return;
  const dataRows = rows.map((r) => [
    r.tier, r.class, r.slot, r.item_id ?? '', r.item_name, r.rank, r.last_refreshed,
  ]);
  sheet.getRange(2, 1, dataRows.length, WIKI_GEAR_TIER_HEADERS.length).setValues(dataRows);
}

function cleanupResumeTriggers(): void {
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
