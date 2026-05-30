// Cross-character fuzzy search — pure client TS port of the v1 Apps Script
// search engine (apps-script/src/lib/searchIndex.ts), Phase 14 Plan 14-02 Task 2.
//
// PORTED VERBATIM from v1 (they are the WEB-03 oracle):
//   - levenshtein  (Wagner-Fischer whole-string edit distance)
//   - groupAndSort + COLLAPSE_THRESHOLD=5  (group matches by item name+id,
//     auto-collapse groups with >5 holders, sort groups by name / rows by char)
//   - the SearchResultRow / SearchResultGroup types
//
// CHANGED during the port:
//   - didYouMean gains the 999.28 empty-query guard as its FIRST line
//     (`if (!query || !query.trim()) return [];`) — a bare empty query has
//     nothing to "mean", so it must return [] instead of every 1-2-char name.
//   - 999.30 is fixed in the test (path (a)): whole-string Levenshtein is the
//     correct algorithm; the v1 it.skip Test 4 assertion was arithmetically
//     wrong. See web/src/lib/__tests__/searchIndex.test.ts.
//
// DROPPED (all the v1 Apps-Script-runtime I/O — none of it exists in the
// browser; D-03 runs over the already-fetched `view` rows in memory): the
// document-cache + user-properties services, the active-spreadsheet handle,
// the inventory-slot list + its accessor, the cache prewarm pass, the per-char
// cache fill + compact sheet read, the Sheet-tab enrichment join, the
// candidate-char discovery, the recent-search MRU get/push, and the
// Sheet-coupled runSearch body (replaced by the pure searchRows engine below).
//
// Trust boundary (carried forward VERBATIM from the v1 header): every
// user/wiki-controlled string flowing OUT of this lib (item names, char names,
// locations) is the presentation layer's responsibility to HTML-escape. This
// lib does NOT pre-escape — it returns RAW strings. The Svelte consumer's `{}`
// interpolation and composeNotes' escapeHtml() are the escaping layer.

// --- Constants (locked) -------------------------------------------------

export const COLLAPSE_THRESHOLD = 5; // auto-collapse groups with >5 holders

// --- Public types --------------------------------------------------------

export interface SearchResultRow {
  itemName: string;
  itemId: number;
  char: string;
  location: string;
  count: number;
  // Enrichment fields the tooltip consumes (populated by the caller from the
  // `view` payload; D-03 — the read API ships these inline so no second fetch).
  wikiUrl: string;
  wikiSummary: string;
  pricePp: number | null;
}

export interface SearchResultGroup {
  itemName: string;
  itemId: number;
  collapsed: boolean; // true when row count > COLLAPSE_THRESHOLD
  rows: SearchResultRow[];
  wikiUrl: string;
  wikiSummary: string;
  pricePp: number | null;
}

export interface SearchResult {
  groups: SearchResultGroup[];
  suggestions: string[];
}

// --- Levenshtein + didYouMean -------------------------------------------

/** Wagner-Fischer DP Levenshtein. Ported VERBATIM from v1 — it is correct and
 * is the WEB-03 oracle. */
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
  if (!query || !query.trim()) return []; // 999.28 FIX — first line
  const q = query.toLowerCase();
  return itemNames
    .map((n) => ({ n, d: levenshtein(q, n.toLowerCase()) }))
    .filter((x) => x.d <= 2 && x.d > 0)
    .sort((a, b) => a.d - b.d)
    .slice(0, 3)
    .map((x) => x.n);
}

// --- Grouping + sort (ported verbatim) ----------------------------------

export function groupAndSort(matches: SearchResultRow[]): SearchResultGroup[] {
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

// --- searchRows: the WEB-03 in-memory engine ----------------------------

/**
 * Pure in-memory search over already-fetched `view` rows (replaces the v1
 * runSearch's Sheet-coupled body). Case-insensitive substring match on item
 * name; groups + sorts the matches; when there are no matches, returns a
 * "did you mean?" suggestion list over the distinct item names in `rows`.
 *
 * Data is tiny at guild scale (~12 users × ~10 chars × ~150 rows = a few
 * thousand rows), so this substring scan + Levenshtein is sub-millisecond —
 * no cache/prewarm needed (WEB-03 <2s is trivial).
 */
export function searchRows(query: string, rows: SearchResultRow[]): SearchResult {
  const q = (query || '').toLowerCase().trim();
  if (!q) return { groups: [], suggestions: [] }; // mirrors v1 runSearch guard

  const matches: SearchResultRow[] = [];
  const allNames = new Set<string>(); // distinct item names for didYouMean
  for (const r of rows) {
    allNames.add(r.itemName);
    if (!r.itemName.toLowerCase().includes(q)) continue;
    matches.push(r);
  }

  const groups = groupAndSort(matches);
  const suggestions = groups.length === 0 ? didYouMean(q, Array.from(allNames)) : [];
  return { groups, suggestions };
}
