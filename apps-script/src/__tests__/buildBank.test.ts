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

  it('happy path: bank_toon_name set + inv tab present → bank tab populated', () => {
    seedMeta(state, [
      ['schema_version', '2'], ['bank_toon_name', 'Bankerton'],
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
    // 1 header + 2 data rows
    expect(bank.values.length).toBe(3);
    // Both rows leading-Char column = bank toon
    expect(bank.values[1][0]).toBe('Bankerton');
    expect(bank.values[2][0]).toBe('Bankerton');
    // Sort: Mystery Item < Pearl alphabetically
    expect(bank.values[1][2]).toBe('Mystery Item');
    expect(bank.values[2][2]).toBe('Pearl');
    // Pearl row has wiki + price
    expect(bank.values[2][5]).toContain('=HYPERLINK');
    expect(bank.values[2][6]).toBe(25);
    // Mystery Item has no wiki/price
    expect(bank.values[1][5]).toBe('');
    expect(bank.values[1][6]).toBe('');
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

  it('bank inv sheet missing: clears bank, logs warning', () => {
    seedMeta(state, [['schema_version', '2'], ['bank_toon_name', 'Ghost']]);
    seedBaseline();
    // No inv:Ghost tab seeded.

    buildBank();

    const bank = state.sheets.get('bank')!;
    // Only header should remain (data rows cleared); no exception thrown
    expect(bank.values.length).toBeLessThanOrEqual(1);
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
