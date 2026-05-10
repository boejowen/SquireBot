import { describe, it, expect, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { refreshWikiItems } from '../triggers/refreshWikiItems';
import {
  resetMocks, seedMeta, makeSheet, type MockState,
} from './test-helpers';

function loadFixture(name: string): string {
  return readFileSync(resolve(__dirname, `../__fixtures__/${name}.json`), 'utf8');
}

const ITEM_MASTER_HEADERS = [
  'item_id', 'name', 'wiki_summary', 'wiki_url',
  'slot', 'is_quest_item', 'last_refreshed', 'wikitext_sha1',
];
const QUEST_ITEMS_HEADERS = [
  'item_id', 'quest_name', 'source_url', 'last_refreshed', 'source',
];

function urlFor(name: string): string {
  return `https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=${encodeURIComponent(name.replace(/ /g, '_'))}&redirects=true`;
}

describe('refreshWikiItems', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '2'],
      ['canonical_id', 'squirebot-v1-workbook-2026'],
      ['theme', 'minimalist'],
      ['contact_email', ''],
      ['last_wiki_summary_refresh', ''],
      ['last_quest_items_refresh', ''],
      ['last_error', '{}'],
    ]);
    state.sheets.set('_status', makeSheet('_status', ['key', 'value'], [
      ['last_wiki_item_count', '0'],
      ['last_error', '{}'],
    ]));
    state.sheets.set('_item_master', makeSheet('_item_master', ITEM_MASTER_HEADERS));
    state.sheets.set('_quest_items', makeSheet('_quest_items', QUEST_ITEMS_HEADERS));
  });

  function seedInventoryWith(items: Array<{ id: number; name: string }>): void {
    // Inventory schema: Location, Name, ID, Count, Slots
    const dataRows = items.map((it) => ['Inventory', it.name, it.id, 1, 0]);
    state.sheets.set('inv:Slampeach', makeSheet('inv:Slampeach',
      ['Location', 'Name', 'ID', 'Count', 'Slots'], dataRows));
  }

  it('happy path: 3 items processed, all parse cleanly, all written', () => {
    seedInventoryWith([
      { id: 100, name: 'Cloth Cap' },
      { id: 101, name: 'Pearl' },
      { id: 102, name: 'Cloak of Flames' },
    ]);
    state.fetchResponses.set(urlFor('Cloth Cap'),
      { status: 200, body: loadFixture('wiki-parse-cloth-cap'), headers: {} });
    state.fetchResponses.set(urlFor('Pearl'),
      { status: 200, body: loadFixture('wiki-parse-pearl'), headers: {} });
    state.fetchResponses.set(urlFor('Cloak of Flames'),
      { status: 200, body: loadFixture('wiki-parse-cloak-of-flames'), headers: {} });

    refreshWikiItems();

    const master = state.sheets.get('_item_master')!;
    expect(master.values.length).toBe(4); // 1 header + 3 data
    const ids = master.values.slice(1).map((r) => r[0]);
    expect(ids.sort()).toEqual([100, 101, 102]);

    const meta = state.sheets.get('_meta')!;
    const refreshRow = meta.values.find((r) => r[0] === 'last_wiki_summary_refresh');
    expect(refreshRow![1]).toMatch(/^\d{4}-\d{2}-\d{2}T/);

    // 1s sleep before each fetch
    expect(state.sleepCalls).toEqual([1000, 1000, 1000]);

    // No cursor remains
    expect(state.properties.get('wiki_refresh_cursor')).toBeUndefined();
  });

  it('quest links written for items with notes-link references (Pearl)', () => {
    seedInventoryWith([{ id: 200, name: 'Pearl' }]);
    state.fetchResponses.set(urlFor('Pearl'),
      { status: 200, body: loadFixture('wiki-parse-pearl'), headers: {} });

    refreshWikiItems();

    const quest = state.sheets.get('_quest_items')!;
    const dataRows = quest.values.slice(1);
    const targets = dataRows.map((r) => r[1]);
    expect(targets).toContain('Call of the Hero');
    expect(targets).toContain('Death Pact');
    expect(targets).toContain('Thicken Mana');
    // All rows reference item_id 200
    for (const r of dataRows) expect(r[0]).toBe(200);
  });

  it('in-game QUEST flag link written for Cloth Cap', () => {
    seedInventoryWith([{ id: 300, name: 'Cloth Cap' }]);
    state.fetchResponses.set(urlFor('Cloth Cap'),
      { status: 200, body: loadFixture('wiki-parse-cloth-cap'), headers: {} });

    refreshWikiItems();

    const quest = state.sheets.get('_quest_items')!;
    const flagRow = quest.values.find((r) => r[4] === 'in_game_flag');
    expect(flagRow).toBeDefined();
    expect(flagRow![0]).toBe(300);
    expect(flagRow![1]).toBe('[in-game QUEST flag]');
  });

  it('SHA short-circuit: existing matching SHA → no _item_master rewrite', () => {
    seedInventoryWith([{ id: 400, name: 'Cloth Cap' }]);
    state.fetchResponses.set(urlFor('Cloth Cap'),
      { status: 200, body: loadFixture('wiki-parse-cloth-cap'), headers: {} });

    // First run → writes a row.
    refreshWikiItems();
    const master = state.sheets.get('_item_master')!;
    expect(master.values.length).toBe(2);
    const firstSha = master.values[1][7];
    const firstRefresh = master.values[1][6];
    expect(typeof firstSha).toBe('string');

    // Second run with the same fixture → SHA matches, no rewrite.
    refreshWikiItems();
    expect(master.values.length).toBe(2);
    // last_refreshed should NOT be touched (proof of skip)
    expect(master.values[1][6]).toBe(firstRefresh);
  });

  it('per-item failure: non-200 fetch counted as failure, processing continues', () => {
    seedInventoryWith([
      { id: 500, name: 'GoodItem' },
      { id: 501, name: 'BadItem' },
    ]);
    state.fetchResponses.set(urlFor('GoodItem'),
      { status: 200, body: loadFixture('wiki-parse-pearl'), headers: {} });
    state.fetchResponses.set(urlFor('BadItem'),
      { status: 404, body: 'not found', headers: {} });

    refreshWikiItems();

    const master = state.sheets.get('_item_master')!;
    expect(master.values.length).toBe(2); // GoodItem only
    expect(master.values[1][0]).toBe(500);
  });

  it('failure threshold: aborts when >50% fail after 50+ items', () => {
    const items: Array<{ id: number; name: string }> = [];
    for (let i = 0; i < 60; i++) items.push({ id: 1000 + i, name: `Item${i}` });
    seedInventoryWith(items);
    // First 30 succeed, remaining 30 fail (>50% threshold once we cross 50 processed)
    for (let i = 0; i < 60; i++) {
      const url = urlFor(`Item${i}`);
      state.fetchResponses.set(url,
        i < 25
          ? { status: 200, body: loadFixture('wiki-parse-pearl'), headers: {} }
          : { status: 500, body: 'down', headers: {} });
    }

    refreshWikiItems();

    // We aborted somewhere after 50 processed. Cursor was deleted.
    expect(state.properties.get('wiki_refresh_cursor')).toBeUndefined();
    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toContain('fetch_failures_exceeded');
  });

  it('idempotent re-run of a completed cycle: fresh full run', () => {
    seedInventoryWith([{ id: 600, name: 'Pearl' }]);
    state.fetchResponses.set(urlFor('Pearl'),
      { status: 200, body: loadFixture('wiki-parse-pearl'), headers: {} });

    refreshWikiItems();
    refreshWikiItems(); // re-run (no cursor)

    // Master still 1 data row (full replace via SHA-skip on second pass)
    const master = state.sheets.get('_item_master')!;
    expect(master.values.length).toBe(2);
  });

  it('wiki API error response: counted as failure', () => {
    seedInventoryWith([{ id: 700, name: 'WeirdItem' }]);
    state.fetchResponses.set(urlFor('WeirdItem'),
      { status: 200, body: '{"error":{"code":"missingtitle","info":"page does not exist"}}', headers: {} });

    refreshWikiItems();

    const master = state.sheets.get('_item_master')!;
    expect(master.values.length).toBe(1); // only header
  });
});
