import { describe, it, expect, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { refreshWikiGearTier } from '../triggers/refreshWikiGearTier';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const HEADERS = ['tier', 'class', 'slot', 'item_id', 'item_name', 'rank', 'last_refreshed'];

function loadFixture(name: string): string {
  return readFileSync(resolve(__dirname, `../__fixtures__/${name}.json`), 'utf8');
}
function pageUrl(page: string): string {
  return `https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=${encodeURIComponent(page.replace(/ /g, '_'))}&redirects=true`;
}

describe('refreshWikiGearTier', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist'], ['last_error', '{}']]);
    state.sheets.set('_status', makeSheet('_status', ['key', 'value'], [
      ['last_wiki_gear_count', '0'], ['last_error', '{}'],
    ]));
    state.sheets.set('_wiki_gear_tier', makeSheet('_wiki_gear_tier', HEADERS));
    state.sheets.set('gear_check', makeSheet('gear_check',
      ['Char', 'Class', 'Tier', 'Slot', 'Have', 'Recommended', 'Status']));
    state.sheets.set('_char_owner', makeSheet('_char_owner', [
      'char_name', 'owner_email', 'display_name', 'discord_handle',
      'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
      'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
    ]));
  });

  it('happy path: 2 sources fetched, _wiki_gear_tier populated, Iksar items tagged', () => {
    state.fetchResponses.set(pageUrl('Players:Velious Pre-Raid Gear'),
      { status: 200, body: loadFixture('wiki-velious-preraid-gear'), headers: {} });
    state.fetchResponses.set(pageUrl('Players:Velious Raiding Gear'),
      { status: 200, body: loadFixture('wiki-velious-raiding-gear'), headers: {} });

    refreshWikiGearTier();

    const sheet = state.sheets.get('_wiki_gear_tier')!;
    expect(sheet.values.length).toBeGreaterThan(800);
    const iksarRows = sheet.values.filter((r) => r[0] === 'Iksar');
    expect(iksarRows.length).toBeGreaterThanOrEqual(4);
    expect(iksarRows.every((r) => String(r[4]).startsWith('Iksar '))).toBe(true);

    const meta = state.sheets.get('_meta')!;
    expect(meta.values.find((r) => r[0] === 'last_wiki_gear_refresh')![1]).toMatch(/^\d{4}/);
    expect(state.sleepCalls).toEqual([1000, 1000]);
  });

  it('partial failure (1 of 2 fetches fails): aborts, no clobber', () => {
    state.fetchResponses.set(pageUrl('Players:Velious Pre-Raid Gear'),
      { status: 200, body: loadFixture('wiki-velious-preraid-gear'), headers: {} });
    state.fetchResponses.set(pageUrl('Players:Velious Raiding Gear'),
      { status: 500, body: 'down', headers: {} });

    // Pre-populate stale data
    const sheet = state.sheets.get('_wiki_gear_tier')!;
    sheet.values.push(['Stale Tier', 'WAR', 'Head', '', 'Stale Item', 1, '2026-01-01']);

    refreshWikiGearTier();

    // Stale data preserved (not clobbered)
    expect(sheet.values.find((r) => r[4] === 'Stale Item')).toBeDefined();
    const meta = state.sheets.get('_meta')!;
    expect(meta.values.find((r) => r[0] === 'last_error')![1]).toContain('partial_failure');
  });

  it('both fetches fail: aborts with fetch_failed error', () => {
    state.fetchResponses.set(pageUrl('Players:Velious Pre-Raid Gear'),
      { status: 500, body: 'x', headers: {} });
    state.fetchResponses.set(pageUrl('Players:Velious Raiding Gear'),
      { status: 500, body: 'x', headers: {} });

    refreshWikiGearTier();

    const meta = state.sheets.get('_meta')!;
    expect(meta.values.find((r) => r[0] === 'last_error')![1]).toContain('fetch_failed');
  });

  it('idempotent re-run: full clear then rewrite (no row duplication)', () => {
    state.fetchResponses.set(pageUrl('Players:Velious Pre-Raid Gear'),
      { status: 200, body: loadFixture('wiki-velious-preraid-gear'), headers: {} });
    state.fetchResponses.set(pageUrl('Players:Velious Raiding Gear'),
      { status: 200, body: loadFixture('wiki-velious-raiding-gear'), headers: {} });

    refreshWikiGearTier();
    const after1 = state.sheets.get('_wiki_gear_tier')!.values.length;
    refreshWikiGearTier();
    const after2 = state.sheets.get('_wiki_gear_tier')!.values.length;
    expect(after2).toBe(after1);
  });
});
