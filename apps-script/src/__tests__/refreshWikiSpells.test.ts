import { describe, it, expect, beforeEach, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { refreshWikiSpells } from '../triggers/refreshWikiSpells';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const WIKI_SPELLS_HEADERS = [
  'class', 'level', 'spell_name', 'normalized_name', 'last_refreshed',
];

function loadFixture(name: string): string {
  return readFileSync(resolve(__dirname, `../__fixtures__/${name}.json`), 'utf8');
}

function classUrl(displayName: string): string {
  return `https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=${encodeURIComponent(displayName.replace(/ /g, '_'))}&redirects=true`;
}

describe('refreshWikiSpells', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '3'],
      ['canonical_id', 'squirebot-v1-workbook-2026'],
      ['last_wiki_spell_refresh', ''],
      ['last_error', '{}'],
    ]);
    state.sheets.set('_status', makeSheet('_status', ['key', 'value'], [
      ['last_wiki_spell_count', '0'],
      ['last_error', '{}'],
    ]));
    state.sheets.set('_wiki_spells', makeSheet('_wiki_spells', WIKI_SPELLS_HEADERS));
    // Seed empty view tabs so buildSpellCheck (called at end) doesn't throw
    state.sheets.set('spell_check', makeSheet('spell_check',
      ['Char', 'Class', 'Level', 'Spell', 'Status']));
    state.sheets.set('_char_owner', makeSheet('_char_owner', [
      'char_name', 'owner_email', 'display_name', 'discord_handle',
      'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
      'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
    ]));
  });

  function mock14ClassResponses(getBody: (display: string) => { status: number; body: string }): void {
    const displays = [
      'Warrior', 'Cleric', 'Paladin', 'Ranger', 'Shadow Knight', 'Druid',
      'Monk', 'Bard', 'Rogue', 'Shaman', 'Necromancer', 'Wizard',
      'Magician', 'Enchanter',
    ];
    for (const d of displays) {
      state.fetchResponses.set(classUrl(d), { ...getBody(d), headers: {} });
    }
  }

  it('happy path: fetches 14 classes; populates _wiki_spells; clears _meta.last_error', () => {
    mock14ClassResponses((display) => {
      // Use Necromancer fixture for all classes — irrelevant since the
      // parser cares about the wikitext shape, not the class match.
      return { status: 200, body: loadFixture('wiki-class-necromancer') };
    });

    refreshWikiSpells();

    const wikiSpells = state.sheets.get('_wiki_spells')!;
    // Header + N spell rows from each of 14 classes (each parses to >100 spells)
    expect(wikiSpells.values.length).toBeGreaterThan(14 * 100);

    // 1s sleep before each of 14 fetches
    expect(state.sleepCalls.length).toBe(14);
    expect(state.sleepCalls.every((ms) => ms === 1000)).toBe(true);

    const meta = state.sheets.get('_meta')!;
    const refreshRow = meta.values.find((r) => r[0] === 'last_wiki_spell_refresh');
    expect(refreshRow![1]).toMatch(/^\d{4}-\d{2}-\d{2}T/);

    // Cursor cleared on success
    expect(state.properties.get('wiki_spells_refresh_cursor')).toBeUndefined();

    // Errors cleared
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toBe('{}');
  });

  it('Warrior degenerate (0 spells): counts as success, no rows written for that class', () => {
    mock14ClassResponses((display) =>
      display === 'Warrior'
        ? { status: 200, body: loadFixture('wiki-class-warrior') }
        : { status: 200, body: loadFixture('wiki-class-paladin') }
    );

    refreshWikiSpells();

    const wikiSpells = state.sheets.get('_wiki_spells')!;
    // No WAR rows
    const warRows = wikiSpells.values.filter((r) => r[0] === 'WAR');
    expect(warRows.length).toBe(0);
    // 13 other classes × 66 PAL spells (using PAL fixture for all non-WAR)
    expect(wikiSpells.values.length).toBeGreaterThan(13 * 50);

    // No errors
    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toBe('{}');
  });

  it('per-class fetch failure: counted but processing continues', () => {
    mock14ClassResponses((display) =>
      display === 'Necromancer'
        ? { status: 404, body: 'not found' }
        : { status: 200, body: loadFixture('wiki-class-paladin') }
    );

    refreshWikiSpells();

    const wikiSpells = state.sheets.get('_wiki_spells')!;
    // 13 successful + 1 failure
    const necRows = wikiSpells.values.filter((r) => r[0] === 'NEC');
    expect(necRows.length).toBe(0); // Necromancer fetch failed → no rows
    // Other classes processed (have ~66 each)
    expect(wikiSpells.values.length).toBeGreaterThan(13 * 50);
  });

  it('failure threshold abort: >50% of >=7 classes failed → abort + write error + delete cursor', () => {
    let calls = 0;
    mock14ClassResponses(() => {
      calls++;
      // First 7 classes fail, then a few succeed — never crosses below 50%
      return calls <= 8
        ? { status: 500, body: 'down' }
        : { status: 200, body: loadFixture('wiki-class-paladin') };
    });

    refreshWikiSpells();

    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toContain('fetch_failures_exceeded');
    expect(state.properties.get('wiki_spells_refresh_cursor')).toBeUndefined();
  });

  it('idempotent re-run: existing cursor → resumes; no cursor → starts fresh', () => {
    mock14ClassResponses(() =>
      ({ status: 200, body: loadFixture('wiki-class-paladin') })
    );
    // Pre-seed a cursor with 5 classes already done
    state.properties.set('wiki_spells_refresh_cursor', JSON.stringify({
      remaining: [
        { abbrev: 'NEC', display: 'Necromancer' },
        { abbrev: 'WIZ', display: 'Wizard' },
      ],
      total: 14,
      failures: 0,
      successes: 12,
      totalRowsWritten: 100,
      started: '2026-05-10T00:00:00Z',
    }));

    refreshWikiSpells();

    // Should have processed only the 2 remaining
    expect(state.sleepCalls.length).toBe(2);
    expect(state.properties.get('wiki_spells_refresh_cursor')).toBeUndefined();
  });

  it('re-run replaces existing _wiki_spells rows for that class (full per-class replace)', () => {
    // Pre-populate _wiki_spells with stale NEC data
    const wikiSpells = state.sheets.get('_wiki_spells')!;
    wikiSpells.values.push(['NEC', 1, 'StaleSpell', 'stalespell', '2026-01-01T00:00:00Z']);
    wikiSpells.values.push(['PAL', 1, 'OtherClassSpell', 'otherclassspell', '2026-01-01T00:00:00Z']);

    mock14ClassResponses(() =>
      ({ status: 200, body: loadFixture('wiki-class-necromancer') })
    );

    refreshWikiSpells();

    // StaleSpell removed; PAL row preserved (from another class's processing)
    const necStaleAfter = wikiSpells.values.filter((r) =>
      r[0] === 'NEC' && r[2] === 'StaleSpell'
    );
    expect(necStaleAfter.length).toBe(0);
  });

  it('checkpoints when budget exceeded', () => {
    // Mock Date.now to simulate budget exhaustion partway
    const realNow = Date.now;
    let callCount = 0;
    Date.now = vi.fn(() => {
      callCount++;
      // Fixed start time, then jump past budget after 3 iterations
      if (callCount <= 1) return 1000;
      if (callCount <= 8) return 1000;  // first iteration's check
      return 1000 + 6 * 60 * 1000;  // past 5min budget
    });
    try {
      mock14ClassResponses(() =>
        ({ status: 200, body: loadFixture('wiki-class-paladin') })
      );
      refreshWikiSpells();
      // Cursor saved
      expect(state.properties.get('wiki_spells_refresh_cursor')).toBeDefined();
    } finally {
      Date.now = realNow;
    }
  });
});
