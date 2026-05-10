import { describe, it, expect, beforeEach } from 'vitest';
import { onChange } from '../triggers/onChange';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const VIEW_HEADERS = ['Char', 'Slot', 'Item', 'ID', 'Count', 'Wiki', 'Price', 'Last Synced'];
const PIGPARSE_HEADERS = [
  'item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed',
  'direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay',
];
const ITEM_MASTER_HEADERS = [
  'item_id', 'name', 'wiki_summary', 'wiki_url', 'slot', 'is_quest_item',
  'last_refreshed', 'wikitext_sha1',
];
const QUEST_ITEMS_HEADERS = ['item_id', 'quest_name', 'source_url', 'last_refreshed', 'source'];

describe('onChange', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '2'], ['theme', 'minimalist']]);
    state.sheets.set('view', makeSheet('view', VIEW_HEADERS));
    state.sheets.set('bank', makeSheet('bank', VIEW_HEADERS));
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
    state.sheets.set('_pigparse', makeSheet('_pigparse', PIGPARSE_HEADERS));
    state.sheets.set('_item_master', makeSheet('_item_master', ITEM_MASTER_HEADERS));
    state.sheets.set('_quest_items', makeSheet('_quest_items', QUEST_ITEMS_HEADERS));
    state.sheets.set('inv:Foo', makeSheet('inv:Foo',
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'],
      [['x', 'Pearl', 200, 1, 0, '2026-05-09T12:00:00Z']]));
  });

  it('triggers buildView + buildBank on call', () => {
    onChange();
    const view = state.sheets.get('view')!;
    expect(view.values.length).toBe(2); // header + 1 data row
  });

  it('handles undefined event arg (manual invocation from menu)', () => {
    expect(() => onChange()).not.toThrow();
  });

  it('debounce inside buildView suppresses rapid second call', () => {
    onChange();
    const view = state.sheets.get('view')!;
    const before = JSON.stringify(view.values);
    // Add data to inv but call onChange immediately — debounce should suppress.
    state.sheets.set('inv:Bar', makeSheet('inv:Bar',
      ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'],
      [['y', 'Cloth Cap', 100, 1, 0, '2026-05-09T12:00:00Z']]));
    onChange();
    const after = JSON.stringify(view.values);
    expect(after).toBe(before); // debounced
  });
});
