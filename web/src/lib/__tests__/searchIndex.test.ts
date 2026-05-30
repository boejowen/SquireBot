// Vitest for the ported client search index (Phase 14 Plan 14-02 Task 2).
//
// Ported from apps-script/src/__tests__/searchIndex.test.ts — the pure-logic
// levenshtein + didYouMean describe blocks port directly. The v1 runSearch
// integration block (Apps-Script-mock-coupled) does NOT port; a searchRows
// describe block tests the in-memory engine instead.
//
// Carried-bug fixes proven here:
//   - 999.28: didYouMean('') returns [] (empty-query guard).
//   - 999.30: the formerly-skipped Test 4 is now a real it(...) asserting
//     toEqual([]) — whole-string Levenshtein distance from 'clok' to the
//     multi-word cloak names is >=13, far above the <=2 cutoff, so the
//     correct result is []. The v1 plan-locked assertion was arithmetically
//     wrong. NO skipped case remains on any didYouMean test.

import { describe, it, expect } from 'vitest';
import {
  levenshtein,
  didYouMean,
  searchRows,
  type SearchResultRow,
} from '../search/searchIndex';

// ---------------------------------------------------------------------------
// levenshtein
// ---------------------------------------------------------------------------

describe('levenshtein', () => {
  it('returns 0 for identical strings', () => {
    expect(levenshtein('abc', 'abc')).toBe(0);
  });

  it('returns length when one string is empty', () => {
    expect(levenshtein('abc', '')).toBe(3);
    expect(levenshtein('', 'xyz')).toBe(3);
  });

  it('returns edit distance for close matches', () => {
    expect(levenshtein('clok', 'cloak')).toBe(1); // insert 'a'
    expect(levenshtein('rusett', 'russet')).toBe(2); // transpose = 2 subs
  });
});

// ---------------------------------------------------------------------------
// didYouMean
// ---------------------------------------------------------------------------

describe('didYouMean', () => {
  // 999.28 — empty query returns [] (the empty-query guard is the first line).
  it('returns [] for an empty query', () => {
    expect(didYouMean('', ['Cloak'])).toEqual([]);
    expect(didYouMean('   ', ['Cloak'])).toEqual([]); // whitespace-only too
  });

  // 999.30 (path a) — formerly the skip-marked Test 4, now un-skipped and
  // corrected. Whole-string Levenshtein('clok', 'cloak of confusion') >= 13,
  // well over the <=2 cutoff, so the correct result is []. The v1 assertion
  // (['Cloak of Confusion','Cloak of Flames']) was arithmetically wrong.
  it('returns [] when only multi-word candidates are far in whole-string distance', () => {
    const seed4 = ['Cloak of Confusion', 'Cloak of Flames', 'Sword of X', 'Cloak Pin'];
    expect(didYouMean('clok', seed4)).toEqual([]);
  });

  it('returns close matches when whole-string distance permits', () => {
    // Distances vs 'clok': 'Cloak'→1, 'Floak'→2, 'Sword'→5 (excluded),
    // 'Cloaks'→2. Sort ascending; ties any order; cap at 3.
    const out = didYouMean('clok', ['Cloak', 'Floak', 'Sword', 'Cloaks']);
    expect(out.length).toBeLessThanOrEqual(3);
    expect(out).toContain('Cloak');
    expect(out).not.toContain('Sword');
  });

  it('excludes exact-match (distance 0) from suggestions', () => {
    const out = didYouMean('cloak', ['Cloak', 'Cloaks', 'Clok']);
    expect(out).not.toContain('Cloak'); // distance 0 excluded
    expect(out).toEqual(expect.arrayContaining(['Cloaks', 'Clok'])); // distance 1
  });

  it('returns [] when no candidate is within edit distance <=2', () => {
    expect(didYouMean('zzzzzz', ['Apple', 'Banana', 'Cucumber'])).toEqual([]);
  });

  it('caps suggestions at 3 even when more candidates are within range', () => {
    const seed = ['ab', 'ac', 'ad', 'ae', 'af'];
    expect(didYouMean('a', seed).length).toBe(3);
  });
});

// ---------------------------------------------------------------------------
// searchRows — the in-memory engine (replaces the v1 runSearch integration)
// ---------------------------------------------------------------------------

function row(over: Partial<SearchResultRow>): SearchResultRow {
  return {
    itemName: 'Item',
    itemId: 1,
    char: 'Findom',
    location: 'General-1',
    count: 1,
    wikiUrl: '',
    wikiSummary: '',
    pricePp: null,
    ...over,
  };
}

describe('searchRows', () => {
  const rows: SearchResultRow[] = [
    row({ itemName: 'Bone Helm', itemId: 1234, char: 'Findom', location: 'HEAD-1' }),
    row({ itemName: 'Bone Chips', itemId: 5678, char: 'Findom', location: 'General-1', count: 4 }),
    row({ itemName: 'Bone Chips', itemId: 5678, char: 'Slampeach', location: 'Bank-12', count: 23 }),
    row({ itemName: 'Bone Chips', itemId: 5678, char: 'Abulus', location: 'General-5', count: 7 }),
    row({ itemName: 'Cloak', itemId: 9999, char: 'Slampeach', location: 'HEAD-1' }),
  ];

  it('returns empty groups + suggestions for an empty query', () => {
    expect(searchRows('', rows)).toEqual({ groups: [], suggestions: [] });
    expect(searchRows('   ', rows)).toEqual({ groups: [], suggestions: [] });
  });

  it('groups case-insensitive substring matches by item name+id', () => {
    const { groups } = searchRows('bone', rows);
    // 2 groups: Bone Helm (1 row) + Bone Chips (3 rows).
    expect(groups.length).toBe(2);
    const helm = groups.find((g) => g.itemName === 'Bone Helm');
    const chips = groups.find((g) => g.itemName === 'Bone Chips');
    expect(helm?.rows.length).toBe(1);
    expect(chips?.rows.length).toBe(3);
  });

  it('matches case-insensitively', () => {
    expect(searchRows('BONE', rows).groups.length).toBe(2);
    expect(searchRows('BoNe', rows).groups.length).toBe(2);
  });

  it('sorts holders alphabetically within a group', () => {
    const { groups } = searchRows('chips', rows);
    expect(groups.length).toBe(1);
    expect(groups[0].rows.map((r) => r.char)).toEqual(['Abulus', 'Findom', 'Slampeach']);
  });

  it('sorts groups by item name', () => {
    const { groups } = searchRows('bone', rows);
    expect(groups.map((g) => g.itemName)).toEqual(['Bone Chips', 'Bone Helm']);
  });

  it('flags groups with >5 holders as collapsed', () => {
    const many: SearchResultRow[] = ['A', 'B', 'C', 'D', 'E', 'F'].map((c) =>
      row({ itemName: 'Rusty Dagger', itemId: 42, char: c, location: 'General-1' }),
    );
    const { groups } = searchRows('rusty', many);
    expect(groups.length).toBe(1);
    expect(groups[0].rows.length).toBeGreaterThan(5);
    expect(groups[0].collapsed).toBe(true);
  });

  it('surfaces a didYouMean suggestion when there are no matches', () => {
    // 'clok' has no substring match; 'Cloak' is the distance-1 fuzzy hit.
    const { groups, suggestions } = searchRows('clok', rows);
    expect(groups).toEqual([]);
    expect(suggestions).toContain('Cloak');
  });

  it('returns no suggestion when there is at least one match', () => {
    const { groups, suggestions } = searchRows('cloak', rows);
    expect(groups.length).toBeGreaterThan(0);
    expect(suggestions).toEqual([]);
  });
});
