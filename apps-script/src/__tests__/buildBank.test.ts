import { describe, it, expect, beforeEach } from 'vitest';
import { buildBank, BANK_HEADERS } from '../tabs/buildBank';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const PIGPARSE_HEADERS = [
  'item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed',
  'direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay',
];
const ITEM_MASTER_HEADERS = [
  'item_id', 'name', 'wiki_summary', 'wiki_url',
  'slot', 'is_quest_item', 'last_refreshed', 'wikitext_sha1',
];
const QUEST_ITEMS_HEADERS = [
  'item_id', 'quest_name', 'source_url', 'last_refreshed', 'source',
];

describe('buildBank', () => {
  let state: MockState;

  function seedBaseline(): void {
    state.sheets.set('bank', makeSheet('bank', BANK_HEADERS));
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
    state.sheets.set('_pigparse', makeSheet('_pigparse', PIGPARSE_HEADERS, [
      [200, 'Pearl', 25, '2026-05-01', 15, '2026-05-09T03:00:00Z',
        0, 15, 25, 30, 26, 60, 28, 100, 30],
    ]));
    state.sheets.set('_item_master', makeSheet('_item_master', ITEM_MASTER_HEADERS, [
      [200, 'Pearl', 'A reagent.', 'https://wiki.project1999.com/Pearl',
        '', 'FALSE', '2026-05-09T04:00:00Z', 'sha-pearl'],
    ]));
    state.sheets.set('_quest_items', makeSheet('_quest_items', QUEST_ITEMS_HEADERS));
  }

  beforeEach(() => {
    state = resetMocks();
  });

  it('happy path: bank_toon_name set + inv tab present → bank tab populated (coin row + 2 inv)', () => {
    seedMeta(state, [
      ['schema_version', '3'], ['bank_toon_name', 'Bankerton'],
      ['bank_coin_pp', '1234'], ['bank_coin_gp', '56'],
      ['bank_coin_sp', '7'], ['bank_coin_cp', '8'],
      ['bank_coin_last_updated', '2026-05-10T12:00:00Z'],
    ]);
    seedBaseline();
    state.sheets.set('inv:Bankerton', makeSheet('inv:Bankerton',
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'],
      [
        ['Bank1', 'Pearl', 200, 50, 0, '2026-05-09T12:00:00Z'],
        ['Bank2', 'Mystery Item', 999, 5, 0, '2026-05-09T12:00:00Z'],
      ],
    ));

    buildBank();

    const bank = state.sheets.get('bank')!;
    // 1 header + 1 coin + 2 inv
    expect(bank.values.length).toBe(4);
    // Coin row at row 2 (index 1)
    expect(bank.values[1][0]).toBe('Bankerton');
    expect(bank.values[1][1]).toBe('COIN');
    expect(bank.values[1][2]).toBe('Platinum: 1234 | Gold: 56 | Silver: 7 | Copper: 8');
    expect(bank.values[1][3]).toBe('');
    expect(bank.values[1][7]).toBe('2026-05-10T12:00:00Z');
    // Inventory rows shifted to row 3+ (indices 2..3)
    expect(bank.values[2][0]).toBe('Bankerton');
    expect(bank.values[3][0]).toBe('Bankerton');
    // Sort: Mystery Item < Pearl alphabetically
    expect(bank.values[2][2]).toBe('Mystery Item');
    expect(bank.values[3][2]).toBe('Pearl');
    // Pearl row has wiki + price
    expect(bank.values[3][5]).toContain('=HYPERLINK');
    expect(bank.values[3][6]).toBe(25);
    // Mystery Item has no wiki/price
    expect(bank.values[2][5]).toBe('');
    expect(bank.values[2][6]).toBe('');
  });

  it('coin row shows all zeros when bank_coin_* rows missing', () => {
    seedMeta(state, [['schema_version', '3'], ['bank_toon_name', 'Bankerton']]);
    seedBaseline();
    state.sheets.set('inv:Bankerton', makeSheet('inv:Bankerton',
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'], []));

    buildBank();

    const bank = state.sheets.get('bank')!;
    expect(bank.values.length).toBe(2); // header + coin row only
    expect(bank.values[1][2]).toBe('Platinum: 0 | Gold: 0 | Silver: 0 | Copper: 0');
    expect(bank.values[1][7]).toBe('');
  });

  it('coin row written even when inv tab missing (no bank toon sync yet)', () => {
    seedMeta(state, [
      ['schema_version', '3'], ['bank_toon_name', 'Ghost'],
      ['bank_coin_pp', '50'],
    ]);
    seedBaseline();
    // No inv:Ghost tab.

    buildBank();

    const bank = state.sheets.get('bank')!;
    expect(bank.values[1][0]).toBe('Ghost');
    expect(bank.values[1][1]).toBe('COIN');
    expect(bank.values[1][2]).toContain('Platinum: 50');
  });

  it('bank_toon_name unset: clears stale data, no error', () => {
    seedMeta(state, [['schema_version', '2'], ['bank_toon_name', '']]);
    seedBaseline();
    // Pre-populate stale bank data
    const bank = state.sheets.get('bank')!;
    bank.values.push(['StaleBob', 'OldLoc', 'Stale Item', 1, 1, '', '', '']);

    buildBank();

    // Stale data cleared
    expect(state.sheets.get('bank')!.values[1][2]).toBe(''); // cleared
  });

  it('bank inv sheet missing: clears stale inventory but still writes coin row', () => {
    seedMeta(state, [['schema_version', '3'], ['bank_toon_name', 'Ghost']]);
    seedBaseline();
    // Pre-populate stale inventory at row 3+
    const bank = state.sheets.get('bank')!;
    bank.values.push(['Ghost', 'COIN', 'old', '', '', '', '', '']);
    bank.values.push(['Ghost', 'StaleSlot', 'StaleItem', 1, 1, '', '', '']);
    // No inv:Ghost tab seeded.

    buildBank();

    // Coin row at row 2 (index 1); stale inventory at row 3 cleared
    expect(bank.values[1][1]).toBe('COIN');
    expect(bank.values[2][2]).toBe(''); // cleared
  });

  it('lock contention: returns silently', () => {
    seedMeta(state, [['schema_version', '2'], ['bank_toon_name', 'Bankerton']]);
    seedBaseline();
    state.sheets.set('inv:Bankerton', makeSheet('inv:Bankerton',
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'],
      [['x', 'Pearl', 200, 1, 0, '2026-05-09T12:00:00Z']]));
    state.lockTryLockReturn = false;

    buildBank();
    const bank = state.sheets.get('bank')!;
    expect(bank.values.length).toBe(1); // header only — never written to
  });

  it('writes _status.last_bank_build + last_bank_row_count on success', () => {
    seedMeta(state, [['schema_version', '2'], ['bank_toon_name', 'Bankerton']]);
    seedBaseline();
    state.sheets.set('inv:Bankerton', makeSheet('inv:Bankerton',
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'],
      [['x', 'Pearl', 200, 1, 0, '2026-05-09T12:00:00Z']]));

    buildBank();
    const status = state.sheets.get('_status')!;
    const buildRow = status.values.find((r) => r[0] === 'last_bank_build');
    expect(buildRow![1]).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    const countRow = status.values.find((r) => r[0] === 'last_bank_row_count');
    expect(countRow![1]).toBe('1');
  });
});
