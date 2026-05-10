// refreshWikiItems — weekly Sunday ~04:00 PT P1999 wiki summary scrape.
//
// Phase 3 plan 03-03. Iterates the union of distinct (item_id, name)
// pairs across all inv:* tabs, fetches each item's wiki page via the
// MediaWiki API with redirects=true, parses {{Itempage}} via
// parseItempage, computes wikitext SHA-1 for change-detection, writes
// to _item_master + _quest_items.
//
// RESUMABLE: at 5 min wall-clock the function checkpoints its remaining
// work to PropertiesService and self-reschedules a one-shot trigger 60s
// out. The next fire reads the cursor and continues. Cursor is deleted
// on completion or abort.
//
// Inter-request 1s sleep between wiki fetches per ROADMAP SC-4.
// Per-page failure tolerance: aborts if >50% of items processed in this
// run fail (after at least 50 items processed — avoids spurious aborts
// on tiny test runs).
//
// The matching weekly time-driven trigger is installed by plan 03-04's
// installTriggers(). The function is also callable manually from the
// SquireBot menu's "Refresh Wiki Items Now".

import { log } from '../lib/log';
import { politeFetch } from '../lib/politeFetch';
import { parseItempage, pageNameToSlug } from '../lib/wiki-parser';
import type { WikiQuestItemLink, ParsedWikiItem } from '../lib/wiki-types';
import {
  getActiveSpreadsheet,
  writeMetaRow,
} from '../lib/sheet-helpers';

const WIKI_API_BASE = 'https://wiki.project1999.com/api.php';
const BUDGET_MS = 5 * 60 * 1000;       // 5 min wall-clock per trigger fire
const INTER_REQUEST_SLEEP_MS = 1000;
const FAILURE_THRESHOLD_PCT = 0.5;
const FAILURE_THRESHOLD_MIN_TOTAL = 50;
const CURSOR_KEY = 'wiki_refresh_cursor';
const RESUME_DELAY_MS = 60_000;
const HANDLER_NAME = 'refreshWikiItems';

interface ItemRef { id: number; name: string; }

interface CursorState {
  remaining: ItemRef[];
  total: number;
  failures: number;
  successes: number;
  unchanged: number;
  started: string;
}

interface ErrorRecord {
  at: string;
  where: 'refreshWikiItems';
  kind: 'fetch_failures_exceeded' | 'collect_failed';
  detail: string;
}

const ITEM_MASTER_HEADERS = [
  'item_id', 'name', 'wiki_summary', 'wiki_url',
  'slot', 'is_quest_item', 'last_refreshed', 'wikitext_sha1',
];
const QUEST_ITEMS_HEADERS = [
  'item_id', 'quest_name', 'source_url', 'last_refreshed', 'source',
];

export function refreshWikiItems(): void {
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
    unchanged: state.unchanged,
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
    if (outcome === 'success') state.successes++;
    else if (outcome === 'unchanged') state.unchanged++;
    else state.failures++;

    const processed = state.successes + state.unchanged + state.failures;
    if (
      processed >= FAILURE_THRESHOLD_MIN_TOTAL
      && state.failures / processed > FAILURE_THRESHOLD_PCT
    ) {
      writeError({
        at: nowIso(),
        where: 'refreshWikiItems',
        kind: 'fetch_failures_exceeded',
        detail: `failures=${state.failures} successes=${state.successes} unchanged=${state.unchanged}`,
      });
      props.deleteProperty(CURSOR_KEY);
      cleanupResumeTriggers();
      log('warn', HANDLER_NAME, { aborted: 'failure_threshold', state });
      return;
    }
  }

  // All items processed.
  const now = nowIso();
  writeMetaRow('_meta', 'last_wiki_summary_refresh', now);
  writeMetaRow('_meta', 'last_quest_items_refresh', now);
  writeMetaRow('_status', 'last_wiki_item_count', String(state.successes + state.unchanged));
  clearError();
  props.deleteProperty(CURSOR_KEY);
  cleanupResumeTriggers();
  log('info', HANDLER_NAME, {
    done: true,
    successes: state.successes,
    unchanged: state.unchanged,
    failures: state.failures,
    total: state.total,
    durationSec: Math.round((Date.now() - startMs) / 1000),
  });
}

function freshState(): CursorState {
  let items: ItemRef[] = [];
  try {
    items = collectInventoryItemRefs();
  } catch (e) {
    writeError({
      at: nowIso(),
      where: 'refreshWikiItems',
      kind: 'collect_failed',
      detail: (e as Error).message,
    });
    log('warn', HANDLER_NAME, { collect_failed: String(e) });
  }
  return {
    remaining: items,
    total: items.length,
    failures: 0,
    successes: 0,
    unchanged: 0,
    started: nowIso(),
  };
}

function checkpoint(state: CursorState): void {
  const props = PropertiesService.getDocumentProperties();
  props.setProperty(CURSOR_KEY, JSON.stringify(state));
  ScriptApp.newTrigger(HANDLER_NAME)
    .timeBased()
    .after(RESUME_DELAY_MS)
    .create();
}

// processOne fetches + parses + writes one item. Returns the outcome
// for accounting. Per-item exceptions are caught and counted as
// failures — the trigger keeps marching.
type ItemOutcome = 'success' | 'unchanged' | 'failure';

function processOne(ref: ItemRef): ItemOutcome {
  const url = `${WIKI_API_BASE}?action=parse&prop=wikitext&format=json&page=${pageNameToSlug(ref.name)}&redirects=true`;
  const result = politeFetch(url);
  if (!result.ok) {
    log('warn', HANDLER_NAME, { item_id: ref.id, fetch_failed: result.status });
    return 'failure';
  }
  let json: { parse?: { title?: string; wikitext?: { '*'?: string } }; error?: { code: string; info: string } };
  try {
    json = JSON.parse(result.body);
  } catch (e) {
    log('warn', HANDLER_NAME, { item_id: ref.id, parse_failed: String(e) });
    return 'failure';
  }
  if (json.error) {
    log('warn', HANDLER_NAME, { item_id: ref.id, wiki_error: json.error.code, info: json.error.info });
    return 'failure';
  }
  const wikitext = json.parse?.wikitext?.['*'];
  const resolvedTitle = json.parse?.title ?? ref.name;
  if (!wikitext) {
    log('warn', HANDLER_NAME, { item_id: ref.id, empty_wikitext: true });
    return 'failure';
  }
  const parsed = parseItempage(wikitext, resolvedTitle);
  if (!parsed.ok) {
    log('warn', HANDLER_NAME, { item_id: ref.id, parse_reason: parsed.reason });
    return 'failure';
  }

  const existingSha = readItemMasterSha(ref.id);
  if (existingSha === parsed.item.wikitext_sha1) {
    return 'unchanged';
  }

  upsertItemMasterRow(ref.id, parsed.item);
  replaceQuestItemRowsForId(ref.id, parsed.questLinks.map((l) => ({ ...l, item_id: ref.id })));
  return 'success';
}

// collectInventoryItemRefs scans every inv:* tab and returns the
// deduplicated union of (item_id, name) pairs. Inventory schema (per
// internal/parse/inventory.go) is Location | Name | ID | Count | Slots
// (col indices 0..4 in the sheet).
function collectInventoryItemRefs(): ItemRef[] {
  const ss = getActiveSpreadsheet();
  const seen = new Map<number, string>();
  for (const sheet of ss.getSheets()) {
    const name = sheet.getName();
    if (!name.startsWith('inv:')) continue;
    const lastRow = sheet.getLastRow();
    if (lastRow < 2) continue;
    const values = sheet.getRange(2, 1, lastRow - 1, 5).getValues();
    for (const row of values) {
      const itemName = String(row[1] ?? '').trim();
      const idRaw = row[2];
      const id = typeof idRaw === 'number' ? idRaw : parseInt(String(idRaw ?? ''), 10);
      if (!itemName || !Number.isFinite(id) || id <= 0) continue;
      if (!seen.has(id)) seen.set(id, itemName);
    }
  }
  return Array.from(seen.entries()).map(([id, name]) => ({ id, name }));
}

function readItemMasterSha(itemId: number): string | null {
  const sheet = getActiveSpreadsheet().getSheetByName('_item_master');
  if (!sheet) return null;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return null;
  // Read item_id (col 1) + wikitext_sha1 (col 8) for all data rows.
  const values = sheet.getRange(2, 1, lastRow - 1, ITEM_MASTER_HEADERS.length).getValues();
  for (const row of values) {
    const id = typeof row[0] === 'number' ? row[0] : parseInt(String(row[0] ?? ''), 10);
    if (id === itemId) {
      const sha = String(row[7] ?? '').trim();
      return sha || null;
    }
  }
  return null;
}

function upsertItemMasterRow(itemId: number, item: ParsedWikiItem): void {
  const sheet = getActiveSpreadsheet().getSheetByName('_item_master');
  if (!sheet) throw new Error('_item_master sheet missing — run migrateToV2');
  const newRow: unknown[] = [
    itemId,
    item.itemname,
    item.summary,
    item.wiki_url,
    item.slot ?? '',
    item.is_quest_item ? 'TRUE' : 'FALSE',
    nowIso(),
    item.wikitext_sha1,
  ];
  const lastRow = sheet.getLastRow();
  if (lastRow >= 2) {
    const idColValues = sheet.getRange(2, 1, lastRow - 1, 1).getValues();
    for (let i = 0; i < idColValues.length; i++) {
      const id = typeof idColValues[i][0] === 'number'
        ? (idColValues[i][0] as number)
        : parseInt(String(idColValues[i][0] ?? ''), 10);
      if (id === itemId) {
        sheet.getRange(2 + i, 1, 1, ITEM_MASTER_HEADERS.length).setValues([newRow]);
        return;
      }
    }
  }
  sheet.appendRow(newRow);
}

function replaceQuestItemRowsForId(itemId: number, links: WikiQuestItemLink[]): void {
  const sheet = getActiveSpreadsheet().getSheetByName('_quest_items');
  if (!sheet) throw new Error('_quest_items sheet missing — run migrateToV2');
  // Step 1: clear existing rows for this item_id (col 1).
  const lastRow = sheet.getLastRow();
  if (lastRow >= 2) {
    const idCol = sheet.getRange(2, 1, lastRow - 1, 1).getValues();
    // Walk bottom-up so deleteRow doesn't shift indices we still need.
    for (let i = idCol.length - 1; i >= 0; i--) {
      const id = typeof idCol[i][0] === 'number'
        ? (idCol[i][0] as number)
        : parseInt(String(idCol[i][0] ?? ''), 10);
      if (id === itemId) sheet.deleteRow(2 + i);
    }
  }
  // Step 2: append fresh.
  const now = nowIso();
  for (const link of links) {
    sheet.appendRow([
      link.item_id,
      link.quest_name,
      link.source === 'in_game_flag' ? '' : `https://wiki.project1999.com/${pageNameToSlug(link.quest_name)}`,
      now,
      link.source,
    ]);
  }
}

function cleanupResumeTriggers(): void {
  // Delete any one-shot resume triggers we created. There may be more
  // than one if a previous run crashed mid-checkpoint; safe to delete
  // all triggers with our handler name + CLOCK type. The weekly
  // recurring trigger created by installTriggers also has our handler
  // name, but it has a non-CLOCK source — actually no, time-based
  // triggers are CLOCK too. We distinguish recurring vs one-shot by
  // counting: the recurring one is the desired single survivor.
  const triggers = ScriptApp.getProjectTriggers().filter(
    (t) => t.getHandlerFunction() === HANDLER_NAME,
  );
  if (triggers.length <= 1) return;
  // Delete extras (keep the first — by convention the recurring one is
  // installed first by installTriggers; resume triggers are added later).
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
