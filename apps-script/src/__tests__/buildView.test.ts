import { describe, it, expect, beforeEach } from 'vitest';
import { buildView, VIEW_HEADERS, pickPrice } from '../tabs/buildView';
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

describe('pickPrice', () => {
  it('prefers WTS over WTB', () => {
    expect(pickPrice([
      { itemId: 1, direction: 0, a30: 100, t30: 5 },
      { itemId: 1, direction: 1, a30: 50, t30: 3 },
    ])).toBe(100);
  });
  it('falls back to WTB when WTS missing', () => {
    expect(pickPrice([{ itemId: 1, direction: 1, a30: 50, t30: 3 }])).toBe(50);
  });
  it('returns empty when both missing or zero', () => {
    expect(pickPrice([])).toBe('');
    expect(pickPrice([
      { itemId: 1, direction: 0, a30: 0, t30: 0 },
      { itemId: 1, direction: 1, a30: 0, t30: 0 },
    ])).toBe('');
  });
});

describe('buildView', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '2'], ['canonical_id', 'x'], ['theme', 'minimalist'],
    ]);
    state.sheets.set('view', makeSheet('view', VIEW_HEADERS));
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
    state.sheets.set('_pigparse', makeSheet('_pigparse', PIGPARSE_HEADERS, [
      // Cloth Cap WTS row
      [100, 'Cloth Cap', 50, '2026-05-01', 10, '2026-05-09T03:00:00Z',
        0, 10, 50, 20, 55, 30, 60, 40, 65],
      // Pearl WTS row
      [200, 'Pearl', 25, '2026-05-01', 15, '2026-05-09T03:00:00Z',
        0, 15, 25, 30, 26, 60, 28, 100, 30],
      // Pearl WTB row
      [200, 'Pearl', 20, '2026-05-01', 5, '2026-05-09T03:00:00Z',
        1, 5, 20, 10, 21, 20, 22, 40, 23],
    ]));
    state.sheets.set('_item_master', makeSheet('_item_master', ITEM_MASTER_HEADERS, [
      [100, 'Cloth Cap', 'A cloth headwrap.', 'https://wiki.project1999.com/Cloth_Cap',
        'HEAD', 'TRUE', '2026-05-09T04:00:00Z', 'sha-cloth'],
      [200, 'Pearl', 'A reagent.', 'https://wiki.project1999.com/Pearl',
        '', 'FALSE', '2026-05-09T04:00:00Z', 'sha-pearl'],
    ]));
    state.sheets.set('_quest_items', makeSheet('_quest_items', QUEST_ITEMS_HEADERS, [
      [200, 'Call of the Hero', 'https://wiki.project1999.com/Call_of_the_Hero',
        '2026-05-09T04:00:00Z', 'notes_link'],
      [100, '[in-game QUEST flag]', '', '2026-05-09T04:00:00Z', 'in_game_flag'],
    ]));
  });

  function seedInv(charName: string, rows: Array<[string, string, number, number, number]>): void {
    const dataRows = rows.map((r) => [...r, '2026-05-09T12:00:00Z']);
    state.sheets.set(`inv:${charName}`, makeSheet(`inv:${charName}`,
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'], dataRows));
  }

  it('happy path: 2 inv tabs → view sorted with prices, wiki links, notes', () => {
    seedInv('Foobar', [
      ['HEAD', 'Cloth Cap', 100, 1, 0],
      ['GENERAL1', 'Pearl', 200, 5, 0],
    ]);
    seedInv('Bazquux', [
      ['GENERAL1', 'Pearl', 200, 3, 0],
    ]);

    buildView();

    const view = state.sheets.get('view')!;
    // 1 header + 3 data rows
    expect(view.values.length).toBe(4);
    // Sorted: Bazquux first (B<F), then Foobar's Cloth Cap (C<P), then Foobar Pearl
    expect(view.values[1][0]).toBe('Bazquux');
    expect(view.values[1][2]).toBe('Pearl');
    expect(view.values[2][0]).toBe('Foobar');
    expect(view.values[2][2]).toBe('Cloth Cap');
    expect(view.values[3][0]).toBe('Foobar');
    expect(view.values[3][2]).toBe('Pearl');

    // Cloth Cap row: price = 50 (WTS a30), wiki HYPERLINK formula
    expect(view.values[2][6]).toBe(50);
    expect(view.values[2][5]).toContain('=HYPERLINK("https://wiki.project1999.com/Cloth_Cap"');

    // Notes attached on col 3 (Item)
    expect(view.notes[2][2]).toContain('A cloth headwrap.'); // summary
    expect(view.notes[2][2]).toContain('Quest item: yes (in-game flag)'); // is_quest flag
    expect(view.notes[3][2]).toContain('Recent ask: 25pp');
    expect(view.notes[3][2]).toContain('Buy posts: 20pp');
    expect(view.notes[3][2]).toContain('Used in quests: Call of the Hero');
  });

  it('debounce: second call within 10s skipped', () => {
    seedInv('A', [['x', 'Pearl', 200, 1, 0]]);
    buildView();
    const view = state.sheets.get('view')!;
    const firstSnapshot = JSON.stringify(view.values);

    // Add another inv row, then call again immediately — debounce should suppress the rebuild
    seedInv('B', [['y', 'Cloth Cap', 100, 1, 0]]);
    buildView();
    const secondSnapshot = JSON.stringify(view.values);
    expect(secondSnapshot).toBe(firstSnapshot);
  });

  it('lock contention: returns silently, no view writes', () => {
    state.lockTryLockReturn = false;
    seedInv('A', [['x', 'Pearl', 200, 1, 0]]);
    buildView();
    const view = state.sheets.get('view')!;
    expect(view.values.length).toBe(1); // header only
  });

  it('item with no _item_master row: blank wiki, no summary in note', () => {
    seedInv('A', [['x', 'Mystery Item', 999, 1, 0]]);
    buildView();
    const view = state.sheets.get('view')!;
    expect(view.values[1][5]).toBe(''); // wiki blank
    // Note exists but mentions "No recent transactions"
    expect(view.notes[1][2]).toContain('No recent transactions on PigParse.');
  });

  it('item with no _pigparse row: price blank', () => {
    seedInv('A', [['x', 'Mystery Item', 999, 1, 0]]);
    buildView();
    const view = state.sheets.get('view')!;
    expect(view.values[1][6]).toBe('');
  });

  it('skips inv rows with missing/invalid item ID', () => {
    seedInv('A', [
      ['x', 'Real Item', 100, 1, 0],
      ['y', '', 0, 0, 0], // empty name + zero ID — skipped
      ['z', 'Bad ID Item', NaN as unknown as number, 1, 0],
    ]);
    buildView();
    const view = state.sheets.get('view')!;
    expect(view.values.length).toBe(2); // 1 header + 1 valid data row
  });

  it('writes _status.last_view_build + last_view_row_count', () => {
    seedInv('A', [['x', 'Pearl', 200, 1, 0]]);
    buildView();
    const status = state.sheets.get('_status')!;
    const buildRow = status.values.find((r) => r[0] === 'last_view_build');
    const countRow = status.values.find((r) => r[0] === 'last_view_row_count');
    expect(buildRow![1]).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    expect(countRow![1]).toBe('1');
  });

  it('applies conditional formatting (3 rules) when there are data rows', () => {
    seedInv('A', [['x', 'Pearl', 200, 1, 0]]);
    buildView();
    const view = state.sheets.get('view') as unknown as {
      _condFormatRules?: Array<{ background: string; formula: string }>;
    };
    expect(view._condFormatRules).toBeDefined();
    expect(view._condFormatRules!.length).toBe(3);
  });
});
