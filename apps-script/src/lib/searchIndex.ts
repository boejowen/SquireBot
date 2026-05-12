// Phase 5 plan 05-03 task 1 — cross-character search engine.
//
// Pure-logic library backing the showSearchSidebar trigger. Per CONTEXT
// D-01..D-08 and PATTERNS §searchIndex.ts:
//   - per-`inv:Char` CacheService entry with 60s TTL (D-03)
//   - case-insensitive substring scan + group-by-item-name (D-02/D-06)
//   - auto-collapse groups with >5 chars (D-07)
//   - hand-rolled Wagner-Fischer Levenshtein for did-you-mean fallback
//     (D-04) — no external dep per RESEARCH §"no new external runtime
//     deps"
//   - rolling-3 recent-query window (D-08)
//   - prewarmSearchCache rides onChange's debounced tail to keep cold
//     paths off the critical user-search latency budget
//
// Trust boundary: every user-controlled string flowing OUT of this lib
// to the sidebar (item names, char names, locations) is the responsibility
// of the sidebar's inline escapeHtml() to neutralize. This lib does NOT
// pre-escape — it returns raw strings.

import { log } from './log';
import { getActiveSpreadsheet } from './sheet-helpers';
import { INVENTORY_SLOTS } from './eq-constants';

// --- Constants (grep-gate locked) ---------------------------------------

const CACHE_TTL_SECONDS = 60;
const MAX_CACHE_VALUE_BYTES = 95_000;  // 100KB cap minus margin (Pitfall P2)
const RECENT_LIMIT = 3;
const COLLAPSE_THRESHOLD = 5;  // D-07: auto-collapse groups with >5 chars

const KEY_INV = (char: string): string => `squirebot:search:inv:${char}`;
const KEY_RECENT = 'squirebot:search:recent';
const KEY_ITEMS_MASTER = 'squirebot:search:items_master';
const KEY_PIGPARSE = 'squirebot:search:pigparse';

type CachedInvRow = [string, string, number, number];  // [Location, Name, ID, Count]

// --- Public types --------------------------------------------------------

export interface SearchResultRow {
  itemName: string;
  itemId: number;
  char: string;
  location: string;
  count: number;
  wikiUrl: string;
  wikiSummary: string;
  pricePp: number | null;
}

export interface SearchResultGroup {
  itemName: string;
  itemId: number;
  collapsed: boolean;   // true when row count > COLLAPSE_THRESHOLD (D-07)
  rows: SearchResultRow[];
  wikiUrl: string;
  wikiSummary: string;
  pricePp: number | null;
}

export interface SearchResult {
  groups: SearchResultGroup[];
  suggestions: string[];
  coldFill: boolean;
  durationMs: number;
}

// --- Levenshtein + didYouMean -------------------------------------------

/** Wagner-Fischer DP Levenshtein. Hand-rolled per "no new external runtime deps" rule. */
export function levenshtein(a: string, b: string): number {
  if (a === b) return 0;
  if (a.length === 0) return b.length;
  if (b.length === 0) return a.length;
  let prev = new Array<number>(b.length + 1);
  for (let j = 0; j <= b.length; j++) prev[j] = j;
  for (let i = 1; i <= a.length; i++) {
    const curr = new Array<number>(b.length + 1);
    curr[0] = i;
    for (let j = 1; j <= b.length; j++) {
      const cost = a.charCodeAt(i - 1) === b.charCodeAt(j - 1) ? 0 : 1;
      curr[j] = Math.min(curr[j - 1] + 1, prev[j] + 1, prev[j - 1] + cost);
    }
    prev = curr;
  }
  return prev[b.length];
}

export function didYouMean(query: string, itemNames: string[]): string[] {
  const q = query.toLowerCase();
  return itemNames
    .map((n) => ({ n, d: levenshtein(q, n.toLowerCase()) }))
    .filter((x) => x.d <= 2 && x.d > 0)
    .sort((a, b) => a.d - b.d)
    .slice(0, 3)
    .map((x) => x.n);
}

// --- Char + inventory helpers -------------------------------------------

/** Discover candidate chars: intersection of _char_owner (filtering
 * is_removed) and live inv:* sheets. */
function getCandidateChars(charFilter: string | null): string[] {
  const ss = getActiveSpreadsheet();
  const ownerSheet = ss.getSheetByName('_char_owner');
  const invSheetNames = new Set(
    ss.getSheets()
      .map((s) => s.getName())
      .filter((n) => n.startsWith('inv:'))
      .map((n) => n.slice(4)),
  );
  let active: string[];
  if (!ownerSheet) {
    active = Array.from(invSheetNames);
  } else {
    const lastRow = ownerSheet.getLastRow();
    const values = lastRow > 1
      ? ownerSheet.getRange(2, 1, lastRow - 1, 13).getValues()
      : [];
    active = values
      .filter((r) => {
        // col 9 = is_removed; treat boolean true OR string 'true'/'TRUE' as removed
        const v = r[8];
        const isRemoved = v === true || String(v).toLowerCase() === 'true';
        return !isRemoved;
      })
      .map((r) => String(r[0] ?? '').trim())
      .filter((c) => c && invSheetNames.has(c));
  }
  const list = charFilter && charFilter !== 'any'
    ? active.filter((c) => c === charFilter)
    : active;
  return Array.from(new Set(list)).sort();
}

function readInvSheetCompact(char: string): CachedInvRow[] {
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(`inv:${char}`);
  if (!sheet) return [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return [];
  const values = sheet.getRange(2, 1, lastRow - 1, 5).getValues();
  return values.map((r): CachedInvRow => [
    String(r[0] ?? ''),
    String(r[1] ?? ''),
    Number(r[2]) || 0,
    Number(r[3]) || 0,
  ]);
}

function getOrFillInvCache(
  char: string,
  cache: GoogleAppsScript.Cache.Cache,
  missesOut: { coldFill: boolean },
): CachedInvRow[] {
  const key = KEY_INV(char);
  const cached = cache.get(key);
  if (cached) {
    try { return JSON.parse(cached) as CachedInvRow[]; }
    catch { /* fall through to re-fill */ }
  }
  const rows = readInvSheetCompact(char);
  const json = JSON.stringify(rows);
  if (json.length < MAX_CACHE_VALUE_BYTES) {
    cache.put(key, json, CACHE_TTL_SECONDS);
  } else {
    log('warn', 'searchIndex', { skipCachePut: char, bytes: json.length });
  }
  missesOut.coldFill = true;
  return rows;
}

// --- prewarmSearchCache -------------------------------------------------

export function prewarmSearchCache(): void {
  const cache = CacheService.getDocumentCache();
  if (!cache) return;
  const chars = getCandidateChars(null);
  const keys = chars.map(KEY_INV);
  const existing = ((cache as unknown as {
    getAll?: (k: string[]) => Record<string, string>;
  }).getAll
    ? (cache as unknown as { getAll: (k: string[]) => Record<string, string> })
        .getAll(keys)
    : {});  // defensive — real Apps Script supports getAll, mock does too
  const missesOut = { coldFill: false };
  for (const char of chars) {
    if (existing[KEY_INV(char)]) continue;
    getOrFillInvCache(char, cache, missesOut);
  }
  log('info', 'prewarmSearchCache', { chars: chars.length, filled: missesOut.coldFill });
}

// --- enrichResults: join _item_master + _pigparse ------------------------

/** Read _item_master and _pigparse (with caching), mutate matches in
 * place with wikiUrl/wikiSummary/pricePp. */
export function enrichResults(
  matches: SearchResultRow[],
  cache: GoogleAppsScript.Cache.Cache,
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): void {
  let itemMasterMap: Record<string, { wikiUrl: string; wikiSummary: string }> = {};
  let pigparseMap: Record<string, number> = {};

  const cachedItems = cache.get(KEY_ITEMS_MASTER);
  if (cachedItems) {
    try { itemMasterMap = JSON.parse(cachedItems); } catch { /* refill below */ }
  } else {
    const sheet = ss.getSheetByName('_item_master');
    if (sheet) {
      const lastRow = sheet.getLastRow();
      const lastCol = sheet.getLastColumn();
      if (lastRow > 1 && lastCol > 0) {
        const headerVals = sheet.getRange(1, 1, 1, lastCol).getValues()[0].map(String);
        const idIdx = headerVals.indexOf('item_id');
        const urlIdx = headerVals.indexOf('wiki_url');
        const sumIdx = headerVals.indexOf('wiki_summary');
        if (idIdx >= 0) {
          const rows = sheet.getRange(2, 1, lastRow - 1, lastCol).getValues();
          for (const r of rows) {
            const id = String(r[idIdx] ?? '');
            if (!id) continue;
            itemMasterMap[id] = {
              wikiUrl: urlIdx >= 0 ? String(r[urlIdx] ?? '') : '',
              wikiSummary: sumIdx >= 0 ? String(r[sumIdx] ?? '') : '',
            };
          }
        }
      }
    }
    const itemsJson = JSON.stringify(itemMasterMap);
    if (itemsJson.length < MAX_CACHE_VALUE_BYTES) {
      cache.put(KEY_ITEMS_MASTER, itemsJson, CACHE_TTL_SECONDS);
    }
  }

  const cachedPigparse = cache.get(KEY_PIGPARSE);
  if (cachedPigparse) {
    try { pigparseMap = JSON.parse(cachedPigparse); } catch { /* refill below */ }
  } else {
    const sheet = ss.getSheetByName('_pigparse');
    if (sheet) {
      const lastRow = sheet.getLastRow();
      const lastCol = sheet.getLastColumn();
      if (lastRow > 1 && lastCol > 0) {
        const headerVals = sheet.getRange(1, 1, 1, lastCol).getValues()[0].map(String);
        const idIdx = headerVals.indexOf('item_id');
        const priceIdx = headerVals.indexOf('current_avg');
        if (idIdx >= 0 && priceIdx >= 0) {
          const rows = sheet.getRange(2, 1, lastRow - 1, lastCol).getValues();
          for (const r of rows) {
            const id = String(r[idIdx] ?? '');
            if (!id) continue;
            const p = Number(r[priceIdx]);
            if (Number.isFinite(p)) pigparseMap[id] = p;
          }
        }
      }
    }
    const pigJson = JSON.stringify(pigparseMap);
    if (pigJson.length < MAX_CACHE_VALUE_BYTES) {
      cache.put(KEY_PIGPARSE, pigJson, CACHE_TTL_SECONDS);
    }
  }

  for (const m of matches) {
    const im = itemMasterMap[String(m.itemId)];
    if (im) { m.wikiUrl = im.wikiUrl; m.wikiSummary = im.wikiSummary; }
    const price = pigparseMap[String(m.itemId)];
    if (typeof price === 'number') m.pricePp = price;
  }
}

// --- Grouping + sort ----------------------------------------------------

function groupAndSort(matches: SearchResultRow[]): SearchResultGroup[] {
  const groupsMap = new Map<string, SearchResultRow[]>();
  for (const m of matches) {
    const key = `${m.itemName}|${m.itemId}`;
    const arr = groupsMap.get(key) ?? [];
    arr.push(m);
    groupsMap.set(key, arr);
  }
  const groups: SearchResultGroup[] = [];
  for (const [, rows] of groupsMap) {
    rows.sort((a, b) => a.char.localeCompare(b.char));
    const first = rows[0];
    groups.push({
      itemName: first.itemName,
      itemId: first.itemId,
      collapsed: rows.length > COLLAPSE_THRESHOLD,
      rows,
      wikiUrl: first.wikiUrl,
      wikiSummary: first.wikiSummary,
      pricePp: first.pricePp,
    });
  }
  groups.sort((a, b) => a.itemName.localeCompare(b.itemName));
  return groups;
}

// --- runSearch ----------------------------------------------------------

export function runSearch(query: string, charFilter: string, slotFilter: string): SearchResult {
  const startMs = Date.now();
  const q = (query || '').toLowerCase().trim();
  if (!q) return { groups: [], suggestions: [], coldFill: false, durationMs: 0 };

  const cache = CacheService.getDocumentCache();
  if (!cache) throw new Error('CacheService unavailable');

  const chars = getCandidateChars(charFilter || 'any');
  const slotFilterUpper = slotFilter && slotFilter !== 'any'
    ? slotFilter.toUpperCase()
    : null;

  const missesOut = { coldFill: false };
  const matches: SearchResultRow[] = [];
  const allNames = new Set<string>();  // for didYouMean fallback

  for (const char of chars) {
    const rows = getOrFillInvCache(char, cache, missesOut);
    for (const [loc, name, id, count] of rows) {
      allNames.add(name);
      if (slotFilterUpper && !loc.toUpperCase().startsWith(slotFilterUpper)) continue;
      if (!name.toLowerCase().includes(q)) continue;
      matches.push({
        itemName: name, itemId: id, char, location: loc, count,
        wikiUrl: '', wikiSummary: '', pricePp: null,
      });
    }
  }

  const ss = getActiveSpreadsheet();
  if (matches.length > 0) enrichResults(matches, cache, ss);
  const groups = groupAndSort(matches);

  let suggestions: string[] = [];
  if (groups.length === 0) {
    suggestions = didYouMean(q, Array.from(allNames));
  }

  const durationMs = Date.now() - startMs;
  log('info', 'runSearch', {
    query: q, charFilter, slotFilter,
    matches: matches.length, groups: groups.length,
    suggestions: suggestions.length, durationMs, coldFill: missesOut.coldFill,
  });
  return { groups, suggestions, coldFill: missesOut.coldFill, durationMs };
}

// --- Recent searches ----------------------------------------------------
// SEARCH-05 (Phase 8 plan 08-03): per-user persistent MRU via Apps Script
// PropertiesService (user scope). KEY_RECENT and the JSON-encoded string-
// array storage shape are unchanged; only the storage backend swaps from
// the prior document-scoped CacheService (25-min default eviction) to the
// per-user properties store (durable across sessions). D-06 clear-and-
// replace: no dual-write, no cache backfill. Worst-case UX is one empty
// recent[] on a guildie's first search after v1.0.1 ships.
// CACHE_TTL_SECONDS is NOT deleted -- prewarmSearchCache and runSearch
// still consume it for the per-`inv:Char` enrichment cache.

export function getRecentSearches(): string[] {
  const props = PropertiesService.getUserProperties();
  if (!props) return [];
  const raw = props.getProperty(KEY_RECENT);
  if (!raw) return [];
  try { return JSON.parse(raw) as string[]; } catch { return []; }
}

export function pushRecentSearch(query: string): void {
  const q = (query || '').trim();
  if (!q) return;
  const props = PropertiesService.getUserProperties();
  if (!props) return;
  const current = getRecentSearches().filter((x) => x !== q);
  const next = [q, ...current].slice(0, RECENT_LIMIT);
  props.setProperty(KEY_RECENT, JSON.stringify(next));
}

// --- Exposed for the sidebar dropdown -----------------------------------

export function listInventorySlots(): string[] {
  return [...INVENTORY_SLOTS];
}
