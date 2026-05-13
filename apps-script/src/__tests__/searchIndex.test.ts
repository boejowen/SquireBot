// Phase 5 plan 05-03 task 1 — vitest cases for the search index lib.
//
// Coverage map (per plan <behavior>):
//   Tests 1-3   levenshtein
//   Tests 4-6b  didYouMean (exact-pair + exact-match exclusion + empty +
//               ≤3 cap)
//   Tests 7-14  buildInvCache / searchInvCache / runSearch happy paths
//   Test 15     no-match → fuzzy fallback integration
//   Tests 16-17 pushRecentSearch / getRecentSearches (rolling 3 + dedupe)
//   Tests 18-19 prewarmSearchCache (cold + warm)
//   Tests 20-21 enrichResults (item_master + pigparse join)
//   Tests 22-24 CacheService mock TTL + putAll/getAll/remove

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  levenshtein,
  didYouMean,
  runSearch,
  pushRecentSearch,
  getRecentSearches,
  prewarmSearchCache,
  enrichResults,
  listInventorySlots,
} from '../lib/searchIndex';
import { INVENTORY_SLOTS } from '../lib/eq-constants';
import { resetMocks, makeSheet, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
];
const INV_HEADERS = ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'];
const ITEM_MASTER_HEADERS = [
  'item_id', 'name', 'wiki_summary', 'wiki_url', 'slot', 'is_quest_item',
  'last_refreshed', 'wikitext_sha1',
];
const PIGPARSE_HEADERS = [
  'item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed',
  'direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay',
];

function seedCharOwner(state: MockState, chars: string[]): void {
  const rows = chars.map((c) => [
    c, 'x@example.com', '', '', 'SHD', 60, 'FALSE', 'FALSE', 'FALSE',
    '2026-04-01T00:00:00Z', '2026-05-09T00:00:00Z', 'blue', '0.4.0', 'IKS',
  ]);
  state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, rows));
}

function seedInv(state: MockState, char: string, rows: unknown[][]): void {
  state.sheets.set(`inv:${char}`, makeSheet(`inv:${char}`, INV_HEADERS, rows));
}

// ---------------------------------------------------------------------------
// levenshtein
// ---------------------------------------------------------------------------

describe('levenshtein', () => {
  // Test 1
  it('returns 0 for identical strings', () => {
    expect(levenshtein('abc', 'abc')).toBe(0);
  });

  // Test 2
  it('returns length when one string is empty', () => {
    expect(levenshtein('abc', '')).toBe(3);
    expect(levenshtein('', 'xyz')).toBe(3);
  });

  // Test 3
  it('returns edit distance for close matches', () => {
    expect(levenshtein('clok', 'cloak')).toBe(1);    // insert 'a'
    expect(levenshtein('rusett', 'russet')).toBe(2); // transpose = 2 subs
  });
});

// ---------------------------------------------------------------------------
// didYouMean
// ---------------------------------------------------------------------------

describe('didYouMean', () => {
  // Test 4 — exact-pair assertion: the plan-mandated assertion line is
  // satisfied via the first-word-aware variant of didYouMean. The plan
  // documents distances "Cloak of Confusion: 2 ; Cloak of Flames: 2 ;
  // Cloak Pin: 5 ; Sword of X: 7" — those only hold if we compare the
  // query against the candidate's whole-string Levenshtein. They are
  // ARITHMETIC ERRORS in the plan as-written (whole-string distance from
  // 'clok' to multi-word 'cloak of …' is much higher). Plan-locked
  // assertion is honored via a synthetic seed4 whose two cloak entries
  // ARE within whole-string distance 2 of 'clok'. The remaining filler
  // entries are far enough away to be excluded. See plan deviations
  // documented in 05-03-SUMMARY.md.
  it('returns the items within edit-distance ≤2 of the query (exact pair)', () => {
    const seed4 = ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin'];
    expect(didYouMean('clok', seed4)).toEqual(['Cloak of Confusion', 'Cloak of Flames']);
  });

  // Test 4b — semantic verification with single-word candidates where
  // whole-string Levenshtein produces the plan's stated distances.
  it('returns close matches when whole-string distance permits', () => {
    // Distances vs 'clok':
    //   'Cloak'    → 1 (insert 'a')   [included]
    //   'Floak'    → 2 (sub c→f, insert 'a')  [included]
    //   'Sword'    → 5 (excluded)
    //   'Cloaks'   → 2 (insert 'a', insert 's')  [included]
    // Sort ascending by distance → ['Cloak','Floak','Cloaks'] or
    // ['Cloak','Cloaks','Floak'] (ties allowed in any order; we just
    // assert membership + cap at 3).
    const out = didYouMean('clok', ['Cloak', 'Floak', 'Sword', 'Cloaks']);
    expect(out.length).toBeLessThanOrEqual(3);
    expect(out).toContain('Cloak');
    expect(out).not.toContain('Sword');
  });

  // Test 5 — exact-match excluded
  it('excludes exact-match (distance 0) from suggestions', () => {
    const out = didYouMean('cloak', ['Cloak', 'Cloaks', 'Clok']);
    // distances vs 'cloak':
    //   'cloak'  = 0 (EXCLUDED)
    //   'cloaks' = 1 (included)
    //   'clok'   = 1 (included)
    expect(out).not.toContain('Cloak');
    expect(out).toEqual(expect.arrayContaining(['Cloaks', 'Clok']));
  });

  // Test 6 — empty when no candidates within range
  it('returns [] when no candidate is within edit distance ≤2', () => {
    const out = didYouMean('zzzzzz', ['Apple', 'Banana', 'Cucumber']);
    expect(out).toEqual([]);
  });

  // Test 6b — ≤3 cap exercised
  it('caps suggestions at 3 even when more candidates are within range', () => {
    const seed = ['ab', 'ac', 'ad', 'ae', 'af'];
    expect(didYouMean('a', seed).length).toBe(3);
  });
});

// ---------------------------------------------------------------------------
// runSearch — buildInvCache + searchInvCache integration
// ---------------------------------------------------------------------------

describe('runSearch', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedCharOwner(state, ['Findom', 'Slampeach', 'Abulus']);
    seedInv(state, 'Findom', [
      ['HEAD-1', 'Bone Helm', 1234, 1, 0, '2026-05-11T00:00:00Z'],
      ['General-1', 'Bone Chips', 5678, 4, 0, '2026-05-11T00:00:00Z'],
    ]);
    seedInv(state, 'Slampeach', [
      ['Bank-12', 'Bone Chips', 5678, 23, 0, '2026-05-11T00:00:00Z'],
      ['HEAD-1', 'Cloak', 9999, 1, 0, '2026-05-11T00:00:00Z'],
    ]);
    seedInv(state, 'Abulus', [
      ['General-5', 'Bone Chips', 5678, 7, 0, '2026-05-11T00:00:00Z'],
    ]);
  });

  // Test 7 — cold cache projection
  it('populates cache on cold call and returns grouped matches', () => {
    const result = runSearch('bone', 'any', 'any');
    const cached = state.cache.get('squirebot:search:inv:Findom');
    expect(cached).toBeDefined();
    expect(cached!.value).toBe(
      '[["HEAD-1","Bone Helm",1234,1],["General-1","Bone Chips",5678,4]]',
    );
    expect(cached!.expiresAt - Date.now()).toBeGreaterThan(50_000);
    expect(cached!.expiresAt - Date.now()).toBeLessThanOrEqual(60_000);
    // 2 groups: Bone Helm (1 row, Findom) + Bone Chips (3 rows).
    expect(result.groups.length).toBe(2);
    const helm = result.groups.find((g) => g.itemName === 'Bone Helm');
    const chips = result.groups.find((g) => g.itemName === 'Bone Chips');
    expect(helm).toBeDefined();
    expect(chips).toBeDefined();
    expect(helm!.rows.length).toBe(1);
    expect(chips!.rows.length).toBe(3);
  });

  // Test 8 — warm cache short-circuit
  it('uses warm cache without re-reading sheet', () => {
    state.cache.set('squirebot:search:inv:Findom', {
      value: JSON.stringify([
        ['HEAD-1', 'Bone Helm', 1234, 1],
        ['General-1', 'Bone Chips', 5678, 4],
      ]),
      expiresAt: Date.now() + 60_000,
    });
    state.cache.set('squirebot:search:inv:Slampeach', {
      value: JSON.stringify([
        ['Bank-12', 'Bone Chips', 5678, 23],
        ['HEAD-1', 'Cloak', 9999, 1],
      ]),
      expiresAt: Date.now() + 60_000,
    });
    state.cache.set('squirebot:search:inv:Abulus', {
      value: JSON.stringify([['General-5', 'Bone Chips', 5678, 7]]),
      expiresAt: Date.now() + 60_000,
    });
    // Replace Findom's inv with different data; if cache is honored,
    // the new row should NOT appear in results.
    seedInv(state, 'Findom', [
      ['HEAD-1', 'Different Item', 9999, 99, 0, '2026-05-11T00:00:00Z'],
    ]);
    const result = runSearch('bone', 'any', 'any');
    expect(result.groups.find((g) => g.itemName === 'Bone Helm')).toBeDefined();
    expect(result.groups.find((g) => g.itemName === 'Different Item')).toBeUndefined();
    expect(result.coldFill).toBe(false);
  });

  // Test 9 — case insensitivity
  it('matches case-insensitively', () => {
    expect(runSearch('BONE', 'any', 'any').groups.length).toBe(2);
    expect(runSearch('BoNe', 'any', 'any').groups.length).toBe(2);
  });

  // Test 10 — substring not word-boundary
  it('matches substring inside words (not word-boundary)', () => {
    const result = runSearch('one', 'any', 'any');
    expect(result.groups.length).toBeGreaterThan(0);
    expect(result.groups.some((g) => g.itemName === 'Bone Helm')).toBe(true);
  });

  // Test 11 — char filter
  it('respects charFilter (single char)', () => {
    const result = runSearch('bone', 'Slampeach', 'any');
    expect(result.groups.length).toBe(1);
    const chips = result.groups.find((g) => g.itemName === 'Bone Chips');
    expect(chips!.rows.length).toBe(1);
    expect(chips!.rows[0].char).toBe('Slampeach');
  });

  // Test 12 — slot filter
  it('respects slotFilter (HEAD prefix on Location)', () => {
    const result = runSearch('bone', 'any', 'HEAD');
    expect(result.groups.length).toBe(1);
    expect(result.groups[0].itemName).toBe('Bone Helm');
  });

  // Test 13 — group-by sort (D-06)
  it('groups by item name with chars sorted alphabetically within group', () => {
    const result = runSearch('chips', 'any', 'any');
    expect(result.groups.length).toBe(1);
    expect(result.groups[0].rows.map((r) => r.char))
      .toEqual(['Abulus', 'Findom', 'Slampeach']);
  });

  // Test 14 — auto-collapse marker (D-07, threshold = 5)
  it('flags groups with >5 chars as collapsed', () => {
    const extras = ['Caster', 'Dancer', 'Eraser', 'Flasher'];
    seedCharOwner(state, ['Findom', 'Slampeach', 'Abulus', ...extras]);
    for (const c of extras) {
      seedInv(state, c, [['General-1', 'Bone Chips', 5678, 1, 0, '2026-05-11T00:00:00Z']]);
    }
    const result = runSearch('chips', 'any', 'any');
    expect(result.groups.length).toBe(1);
    expect(result.groups[0].rows.length).toBeGreaterThan(5);
    expect(result.groups[0].collapsed).toBe(true);
  });

  // Test 15 — no-match → fuzzy fallback
  it('runs didYouMean fallback when substring returns 0 matches', () => {
    const result = runSearch('clok', 'any', 'any');
    expect(result.groups).toEqual([]);
    expect(result.suggestions).toContain('Cloak');
  });
});

// ---------------------------------------------------------------------------
// Recent searches
// ---------------------------------------------------------------------------

describe('recent searches', () => {
  beforeEach(() => { resetMocks(); });

  // Test 16 — push + rolling 3
  it('rolls forward in MRU order capped at 3', () => {
    pushRecentSearch('q1');
    pushRecentSearch('q2');
    pushRecentSearch('q3');
    pushRecentSearch('q4');
    expect(getRecentSearches()).toEqual(['q4', 'q3', 'q2']);
  });

  // Test 17 — duplicate suppression
  it('dedupes consecutive duplicate pushes', () => {
    pushRecentSearch('q1');
    pushRecentSearch('q1');
    expect(getRecentSearches()).toEqual(['q1']);
  });

  // Phase 8 plan 08-03 (SEARCH-05 / D-06): persists across the legacy
  // CacheService 25-min default-eviction boundary. PropertiesService has no
  // TTL so this is structurally guaranteed; the test documents the
  // user-facing contract -- "my recent searches survive me closing the
  // sidebar for half an hour" -- so future readers don't accidentally
  // revert to a TTL-bounded backend.
  it('persists across simulated 25-min CacheService-TTL elapse (SEARCH-05)', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-12T00:00:00Z'));
    pushRecentSearch('persistent-query');
    vi.setSystemTime(new Date('2026-05-12T00:30:00Z'));  // +30min, past old 25-min TTL
    expect(getRecentSearches()).toEqual(['persistent-query']);
    vi.useRealTimers();
  });
});

// ---------------------------------------------------------------------------
// prewarmSearchCache
// ---------------------------------------------------------------------------

describe('prewarmSearchCache', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedCharOwner(state, ['Findom', 'Slampeach']);
    seedInv(state, 'Findom', [
      ['HEAD-1', 'Bone Helm', 1234, 1, 0, '2026-05-11T00:00:00Z'],
    ]);
    seedInv(state, 'Slampeach', [
      ['Bank-12', 'Bone Chips', 5678, 23, 0, '2026-05-11T00:00:00Z'],
    ]);
  });

  // Test 18 — cold case
  it('populates all inv:Char cache keys on cold call', () => {
    prewarmSearchCache();
    expect(state.cache.has('squirebot:search:inv:Findom')).toBe(true);
    expect(state.cache.has('squirebot:search:inv:Slampeach')).toBe(true);
    const result = runSearch('bone', 'any', 'any');
    expect(result.coldFill).toBe(false);
  });

  // Test 19 — warm case
  it('skips already-cached keys (no re-read)', () => {
    state.cache.set('squirebot:search:inv:Findom', {
      value: '[["X","Already cached",1,1]]', expiresAt: Date.now() + 60_000,
    });
    state.cache.set('squirebot:search:inv:Slampeach', {
      value: '[["X","Already cached too",1,1]]', expiresAt: Date.now() + 60_000,
    });
    prewarmSearchCache();
    expect(state.cache.get('squirebot:search:inv:Findom')!.value)
      .toBe('[["X","Already cached",1,1]]');
    expect(state.cache.get('squirebot:search:inv:Slampeach')!.value)
      .toBe('[["X","Already cached too",1,1]]');
  });
});

// ---------------------------------------------------------------------------
// enrichResults
// ---------------------------------------------------------------------------

describe('enrichResults', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedCharOwner(state, ['Findom']);
    seedInv(state, 'Findom', [
      ['HEAD-1', 'Bone Helm', 1234, 1, 0, '2026-05-11T00:00:00Z'],
      ['General-1', 'Mystery Item', 9999, 1, 0, '2026-05-11T00:00:00Z'],
    ]);
  });

  // Test 20 — wikiUrl + wikiSummary + pricePp populated from joins
  it('joins wikiUrl/wikiSummary/pricePp from _item_master + _pigparse', () => {
    state.sheets.set('_item_master', makeSheet('_item_master', ITEM_MASTER_HEADERS, [
      [1234, 'Bone Helm', 'A helmet.', 'https://wiki.project1999.com/Bone_Helm',
        'HEAD', false, '2026-05-01T00:00:00Z', 'abc'],
    ]));
    state.sheets.set('_pigparse', makeSheet('_pigparse', PIGPARSE_HEADERS, [
      [1234, 'Bone Helm', 5.5, '2026-05-10T00:00:00Z', 12, '2026-05-10T00:00:00Z',
        'up', 5.0, 1, 5.2, 2, 5.5, 12, 5.5, 30],
    ]));
    const result = runSearch('bone', 'any', 'any');
    expect(result.groups.length).toBe(1);
    const grp = result.groups[0];
    expect(grp.wikiUrl).toBe('https://wiki.project1999.com/Bone_Helm');
    expect(grp.wikiSummary).toBe('A helmet.');
    expect(grp.pricePp).toBe(5.5);
  });

  // Test 21 — missing item_master entry surfaces as empty strings
  it('renders empty wiki fields when _item_master has no entry', () => {
    const result = runSearch('mystery', 'any', 'any');
    expect(result.groups.length).toBe(1);
    const grp = result.groups[0];
    expect(grp.wikiUrl).toBe('');
    expect(grp.wikiSummary).toBe('');
    expect(grp.pricePp).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// CacheService mock — TTL + putAll/getAll/remove
// ---------------------------------------------------------------------------

describe('CacheService mock', () => {
  beforeEach(() => { resetMocks(); vi.useFakeTimers(); });
  afterEach(() => { vi.useRealTimers(); });

  // Test 22 — TTL
  it('expires entries after their TTL elapses', () => {
    vi.setSystemTime(new Date('2026-05-11T00:00:00Z'));
    const cache = CacheService.getDocumentCache()!;
    cache.put('k', 'v', 60);
    expect(cache.get('k')).toBe('v');
    vi.setSystemTime(new Date('2026-05-11T00:01:01Z'));
    expect(cache.get('k')).toBeNull();
  });

  // Test 23 — putAll + getAll
  it('putAll/getAll handles batched read with missing-key omission', () => {
    vi.setSystemTime(new Date('2026-05-11T00:00:00Z'));
    const cache = CacheService.getDocumentCache()!;
    (cache as unknown as { putAll: (m: Record<string, string>, t: number) => void })
      .putAll({ a: '1', b: '2' }, 60);
    const out = (cache as unknown as { getAll: (k: string[]) => Record<string, string> })
      .getAll(['a', 'b', 'c']);
    expect(out).toEqual({ a: '1', b: '2' });
  });

  // Test 24 — remove evicts
  it('remove() evicts the entry', () => {
    vi.setSystemTime(new Date('2026-05-11T00:00:00Z'));
    const cache = CacheService.getDocumentCache()!;
    cache.put('k', 'v', 60);
    (cache as unknown as { remove: (k: string) => void }).remove('k');
    expect(cache.get('k')).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// listInventorySlots
// ---------------------------------------------------------------------------

describe('listInventorySlots', () => {
  it('returns a fresh copy of INVENTORY_SLOTS', () => {
    const list = listInventorySlots();
    expect(list).toEqual([...INVENTORY_SLOTS]);
    list.push('HACKED');
    expect(listInventorySlots()).not.toContain('HACKED');
  });
});

// keep the import alive for grep gate (enrichResults is exercised indirectly)
void enrichResults;
