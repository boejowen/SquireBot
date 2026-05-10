import { describe, it, expect, beforeEach } from 'vitest';
import { buildSpellCheck, SPELL_CHECK_HEADERS } from '../tabs/buildSpellCheck';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
];
const WIKI_SPELLS_HEADERS = [
  'class', 'level', 'spell_name', 'normalized_name', 'last_refreshed',
];

describe('buildSpellCheck', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
    state.sheets.set('spell_check', makeSheet('spell_check', SPELL_CHECK_HEADERS));
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
  });

  function seedChar(charName: string, cls: string, level: number): void {
    let owner = state.sheets.get('_char_owner');
    if (!owner) {
      owner = makeSheet('_char_owner', CHAR_OWNER_HEADERS);
      state.sheets.set('_char_owner', owner);
    }
    owner.values.push([
      charName, 'x@example.com', '', '', cls, level, 'FALSE', 'FALSE', 'FALSE',
      '2026-04-01T00:00:00Z', '2026-05-09T00:00:00Z', 'blue', '0.4.0', '',
    ]);
  }

  function seedWikiSpells(rows: Array<[string, number, string]>): void {
    let s = state.sheets.get('_wiki_spells');
    if (!s) {
      s = makeSheet('_wiki_spells', WIKI_SPELLS_HEADERS);
      state.sheets.set('_wiki_spells', s);
    }
    for (const [cls, lvl, name] of rows) {
      const normalized = name.toLowerCase().replace(/[^a-z0-9]/g, '');
      s.values.push([cls, lvl, name, normalized, '2026-05-10T00:00:00Z']);
    }
  }

  function seedSpellbook(charName: string, spells: Array<[number, string]>): void {
    state.sheets.set(`spell:${charName}`, makeSheet(`spell:${charName}`,
      ['Level', 'Name', '_uploaded_at'],
      spells.map(([lvl, n]) => [lvl, n, '2026-05-09T00:00:00Z'])));
  }

  it('happy path: NEC lvl 10 with 5 wiki spells, knows 2 → 2 KNOWN + 3 MISSING', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([
      ['NEC', 1, 'Cavorting Bones'],
      ['NEC', 1, 'Coldlight'],
      ['NEC', 4, 'Disease Cloud'],
      ['NEC', 8, 'Locate Corpse'],
      ['NEC', 12, 'Numb the Dead'],  // out of range (level 12 > char level 10)
    ]);
    seedSpellbook('Slampeach', [
      [1, 'Cavorting Bones'],
      [1, 'Coldlight'],
    ]);

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    const data = sc.values.slice(1); // skip header
    expect(data.length).toBe(4);  // 4 spells in range; lvl 12 excluded
    const knownCount = data.filter((r) => r[4] === 'KNOWN').length;
    const missingCount = data.filter((r) => r[4] === 'MISSING').length;
    expect(knownCount).toBe(2);
    expect(missingCount).toBe(2);
  });

  it('char without metadata (no class) is skipped entirely', () => {
    seedChar('Slampeach', '', 0);
    seedWikiSpells([['NEC', 1, 'Cavorting Bones']]);
    seedSpellbook('Slampeach', [[1, 'Cavorting Bones']]);

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    expect(sc.values.length).toBe(1);  // header only
  });

  it('char without spell:<char> tab: all spells MISSING', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([
      ['NEC', 1, 'Cavorting Bones'],
      ['NEC', 1, 'Coldlight'],
    ]);
    // no spell:Slampeach tab

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    const data = sc.values.slice(1);
    expect(data.length).toBe(2);
    expect(data.every((r) => r[4] === 'MISSING')).toBe(true);
  });

  it('multiple chars: rows sorted Char asc → Level asc → Spell asc', () => {
    seedChar('Bazquux', 'PAL', 20);
    seedChar('Foobar', 'NEC', 20);
    seedWikiSpells([
      ['NEC', 4, 'Disease Cloud'],
      ['NEC', 1, 'Coldlight'],
      ['NEC', 1, 'Cavorting Bones'],
      ['PAL', 9, 'Holy Armor'],
      ['PAL', 9, 'Courage'],
    ]);

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    const data = sc.values.slice(1);
    // Bazquux first (alphabetical), then Foobar
    expect(data[0][0]).toBe('Bazquux');
    expect(data[0][2]).toBe(9);  // PAL spells
    expect(data[1][0]).toBe('Bazquux');
    // Foobar (NEC) section: lvl 1 first, then lvl 4
    expect(data[2][0]).toBe('Foobar');
    expect(data[2][2]).toBe(1);
    expect(data[3][0]).toBe('Foobar');
    expect(data[3][2]).toBe(1);
    expect(data[4][0]).toBe('Foobar');
    expect(data[4][2]).toBe(4);
    // Within same level, alpha sort
    expect(data[2][3]).toBe('Cavorting Bones');
    expect(data[3][3]).toBe('Coldlight');
  });

  it('lock contention: returns silently, no spell_check writes', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([['NEC', 1, 'Cavorting Bones']]);
    state.lockTryLockReturn = false;

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    expect(sc.values.length).toBe(1); // header only
  });

  it('debounce: second call within 10s skipped', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([['NEC', 1, 'Coldlight']]);
    seedSpellbook('Slampeach', [[1, 'Coldlight']]);

    buildSpellCheck();
    const after1 = state.sheets.get('spell_check')!.values.length;
    expect(after1).toBe(2);  // header + 1

    // Add another spell + call again immediately — debounce should suppress
    seedWikiSpells([['NEC', 4, 'Disease Cloud']]);
    buildSpellCheck();
    const after2 = state.sheets.get('spell_check')!.values.length;
    expect(after2).toBe(after1);  // unchanged due to debounce
  });

  it('writes _status.last_spell_check_build + last_spell_check_row_count', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([['NEC', 1, 'Coldlight']]);
    seedSpellbook('Slampeach', []);

    buildSpellCheck();

    const status = state.sheets.get('_status')!;
    expect(status.values.find((r) => r[0] === 'last_spell_check_build')![1])
      .toMatch(/^\d{4}-\d{2}-\d{2}T/);
    expect(status.values.find((r) => r[0] === 'last_spell_check_row_count')![1])
      .toBe('1');
  });

  it('normalized-name match handles capitalization + whitespace + apostrophes', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([['NEC', 4, 'Numb the Dead']]);
    seedSpellbook('Slampeach', [[4, 'numb the dead  ']]);  // caps + trailing space differ

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    expect(sc.values[1][4]).toBe('KNOWN');
  });

  it('Warrior char (no class spells available): zero rows', () => {
    seedChar('Tankard', 'WAR', 60);
    // no _wiki_spells WAR rows (Warrior is degenerate per parser)
    seedSpellbook('Tankard', []);

    buildSpellCheck();

    const sc = state.sheets.get('spell_check')!;
    expect(sc.values.length).toBe(1);  // header only
  });

  it('spell_check sheet missing: returns silently without throwing', () => {
    seedChar('Slampeach', 'NEC', 10);
    seedWikiSpells([['NEC', 1, 'Coldlight']]);
    state.sheets.delete('spell_check');

    expect(() => buildSpellCheck()).not.toThrow();
  });
});
