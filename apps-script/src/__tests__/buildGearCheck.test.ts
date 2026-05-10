import { describe, it, expect, beforeEach } from 'vitest';
import { buildGearCheck, GEAR_CHECK_HEADERS } from '../tabs/buildGearCheck';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
];
const GEAR_TIER_HEADERS = ['tier', 'class', 'slot', 'item_id', 'item_name', 'rank', 'last_refreshed'];
const INV_HEADERS = ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'];

describe('buildGearCheck', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
    state.sheets.set('gear_check', makeSheet('gear_check', GEAR_CHECK_HEADERS));
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
  });

  function seedChar(name: string, cls: string, race: string): void {
    let owner = state.sheets.get('_char_owner');
    if (!owner) {
      owner = makeSheet('_char_owner', CHAR_OWNER_HEADERS);
      state.sheets.set('_char_owner', owner);
    }
    owner.values.push([name, '', '', '', cls, 60, '', '', '', '', '', 'blue', '0.4.0', race]);
  }
  function seedGear(rows: Array<[string, string, string, string, number]>): void {
    let s = state.sheets.get('_wiki_gear_tier');
    if (!s) {
      s = makeSheet('_wiki_gear_tier', GEAR_TIER_HEADERS);
      state.sheets.set('_wiki_gear_tier', s);
    }
    for (const [tier, cls, slot, name, rank] of rows) {
      s.values.push([tier, cls, slot, '', name, rank, '2026-05-10']);
    }
  }
  function seedInv(charName: string, items: Array<[string, string, number]>): void {
    state.sheets.set(`inv:${charName}`, makeSheet(`inv:${charName}`, INV_HEADERS,
      items.map(([loc, name, id]) => [loc, name, id, 1, 0, '2026-05-09T00:00:00Z'])));
  }

  it('happy path: NEC HUM with 2 wiki items, has 1 → 1 OK + 1 MISSING', () => {
    seedChar('Slampeach', 'NEC', 'HUM');
    seedGear([
      ['Velious Pre-Raid/Group', 'NEC', 'Head', 'Circlet of Vallon', 1],
      ['Velious Pre-Raid/Group', 'NEC', 'Chest', 'Robe of the Lost Circle', 1],
    ]);
    seedInv('Slampeach', [['HEAD', 'Circlet of Vallon', 1234]]);

    buildGearCheck();
    const data = state.sheets.get('gear_check')!.values.slice(1);
    expect(data.length).toBe(2);
    const head = data.find((r) => r[3] === 'Head')!;
    expect(head[6]).toBe('OK');
    expect(head[4]).toBe('Circlet of Vallon');
    const chest = data.find((r) => r[3] === 'Chest')!;
    expect(chest[6]).toBe('MISSING');
  });

  it('OTHER status: char has wrong item in slot', () => {
    seedChar('Slampeach', 'NEC', 'HUM');
    seedGear([['Velious Pre-Raid/Group', 'NEC', 'Head', 'Circlet of Vallon', 1]]);
    seedInv('Slampeach', [['HEAD', 'Some Other Helm', 999]]);

    buildGearCheck();
    const data = state.sheets.get('gear_check')!.values.slice(1);
    expect(data[0][6]).toBe('OTHER');
    expect(data[0][4]).toBe('Some Other Helm');
  });

  it('Iksar tier shown ONLY for race=IKS', () => {
    seedChar('IksarSk', 'SHD', 'IKS');
    seedChar('HumanSk', 'SHD', 'HUM');
    seedGear([
      ['Velious Pre-Raid/Group', 'SHD', 'Head', 'Pre-Raid Helm', 1],
      ['Iksar', 'SHD', 'Head', 'Iksar Hide Cap', 1],
    ]);

    buildGearCheck();
    const data = state.sheets.get('gear_check')!.values.slice(1);
    const iksarRows = data.filter((r) => r[2] === 'Iksar');
    expect(iksarRows.every((r) => r[0] === 'IksarSk')).toBe(true);
    expect(iksarRows.length).toBeGreaterThan(0);
    // HumanSk must NOT have Iksar tier rows
    const humanIksar = data.filter((r) => r[0] === 'HumanSk' && r[2] === 'Iksar');
    expect(humanIksar.length).toBe(0);
  });

  it('pair-slot match: Iksar Hide Cap recommended for Ears, char has it in EAR2 → OK', () => {
    seedChar('Slampeach', 'MNK', 'HUM');
    seedGear([['Velious Pre-Raid/Group', 'MNK', 'Ears', 'Fingerbone Hoop', 1]]);
    seedInv('Slampeach', [
      ['EAR1', 'Other Earring', 1],
      ['EAR2', 'Fingerbone Hoop', 2],
    ]);

    buildGearCheck();
    const data = state.sheets.get('gear_check')!.values.slice(1);
    expect(data[0][6]).toBe('OK');
  });

  it('char without metadata (no class) is skipped', () => {
    seedChar('NoClass', '', 'HUM');
    seedGear([['Velious Pre-Raid/Group', 'NEC', 'Head', 'Helm', 1]]);
    buildGearCheck();
    expect(state.sheets.get('gear_check')!.values.length).toBe(1);  // header only
  });

  it('lock contention returns silently', () => {
    seedChar('S', 'NEC', 'HUM');
    seedGear([['Velious Pre-Raid/Group', 'NEC', 'Head', 'X', 1]]);
    state.lockTryLockReturn = false;
    buildGearCheck();
    expect(state.sheets.get('gear_check')!.values.length).toBe(1);
  });

  it('debounce skips second call within 10s', () => {
    seedChar('S', 'NEC', 'HUM');
    seedGear([['Velious Pre-Raid/Group', 'NEC', 'Head', 'X', 1]]);
    buildGearCheck();
    const after1 = state.sheets.get('gear_check')!.values.length;
    seedGear([['Velious Pre-Raid/Group', 'NEC', 'Chest', 'Y', 1]]);
    buildGearCheck();
    expect(state.sheets.get('gear_check')!.values.length).toBe(after1);
  });

  it('sort order: char asc → tier rank asc → slot asc', () => {
    seedChar('Bee', 'NEC', 'IKS');
    seedChar('Apple', 'NEC', 'HUM');
    seedGear([
      ['Iksar', 'NEC', 'Head', 'Iksar Helm', 1],
      ['Velious Raiding', 'NEC', 'Chest', 'Raid Robe', 1],
      ['Velious Pre-Raid/Group', 'NEC', 'Head', 'Pre Helm', 1],
      ['Velious Pre-Raid/Group', 'NEC', 'Chest', 'Pre Robe', 1],
    ]);
    buildGearCheck();
    const data = state.sheets.get('gear_check')!.values.slice(1);
    // Apple first
    expect(data[0][0]).toBe('Apple');
    // Apple's section ordered: Pre-Raid (Chest, Head), Raiding (Chest)
    // Pre-Raid Chest, Pre-Raid Head, Raiding Chest
    const appleRows = data.filter((r) => r[0] === 'Apple');
    expect(appleRows[0][2]).toBe('Velious Pre-Raid/Group');
    expect(appleRows[0][3]).toBe('Chest');
    expect(appleRows[1][3]).toBe('Head');
    expect(appleRows[appleRows.length - 1][2]).toBe('Velious Raiding');
    // Bee's section also in order; Iksar last
    const beeRows = data.filter((r) => r[0] === 'Bee');
    expect(beeRows[beeRows.length - 1][2]).toBe('Iksar');
  });

  it('writes _status build + row count', () => {
    seedChar('S', 'NEC', 'HUM');
    seedGear([['Velious Pre-Raid/Group', 'NEC', 'Head', 'X', 1]]);
    buildGearCheck();
    const status = state.sheets.get('_status')!;
    expect(status.values.find((r) => r[0] === 'last_gear_check_build')![1]).toMatch(/^\d{4}/);
    expect(status.values.find((r) => r[0] === 'last_gear_check_row_count')![1]).toBe('1');
  });
});
