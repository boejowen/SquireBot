import { describe, it, expect, beforeEach } from 'vitest';
import { migrateToV2 } from '../lib/migrations';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

describe('migrateToV2', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
  });

  function seedV1Workbook(): void {
    seedMeta(state, [
      ['schema_version', '1'],
      ['canonical_id', 'squirebot-v1-workbook-2026'],
      ['theme', 'minimalist'],
      ['contact_email', ''],
    ]);
    state.sheets.set('_pigparse', makeSheet('_pigparse',
      ['item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed']));
    state.sheets.set('_item_master', makeSheet('_item_master',
      ['item_id', 'name', 'wiki_summary', 'wiki_url', 'slot', 'is_quest_item', 'last_refreshed']));
    state.sheets.set('_quest_items', makeSheet('_quest_items',
      ['item_id', 'quest_name', 'source_url', 'last_refreshed']));
  }

  it('returns noop_already_v2 when schema_version is already 2', () => {
    seedMeta(state, [['schema_version', '2'], ['canonical_id', 'x']]);
    expect(migrateToV2()).toBe('noop_already_current');
    // No header changes should have been logged
    const pigparseValues = state.sheets.get('_pigparse');
    expect(pigparseValues).toBeUndefined();
  });

  it('returns noop_unsupported_version when schema_version is not 1 or 2', () => {
    seedMeta(state, [['schema_version', '99']]);
    expect(migrateToV2()).toBe('noop_unsupported_version');
  });

  it('migrates v1 → v2 by appending columns and bumping schema_version', () => {
    seedV1Workbook();
    expect(migrateToV2()).toBe('migrated');

    const pigparse = state.sheets.get('_pigparse')!;
    const pigHeaders = pigparse.values[0] as string[];
    expect(pigHeaders).toEqual([
      'item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed',
      'direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay',
    ]);

    const master = state.sheets.get('_item_master')!;
    expect((master.values[0] as string[])[(master.values[0] as string[]).length - 1]).toBe('wikitext_sha1');

    const quest = state.sheets.get('_quest_items')!;
    expect((quest.values[0] as string[])[(quest.values[0] as string[]).length - 1]).toBe('source');

    const meta = state.sheets.get('_meta')!;
    const schemaRow = meta.values.find((r) => r[0] === 'schema_version');
    expect(schemaRow).toBeDefined();
    expect(schemaRow![1]).toBe('2');
  });

  it('is idempotent on the column-extension step', () => {
    seedV1Workbook();
    // Pretend a prior partial migration already added some columns:
    const pigparse = state.sheets.get('_pigparse')!;
    pigparse.values[0] = [...(pigparse.values[0] as string[]), 'direction', 't30', 'a30'];

    expect(migrateToV2()).toBe('migrated');
    const pigHeaders = pigparse.values[0] as string[];
    // No duplicate columns added
    const counts: Record<string, number> = {};
    pigHeaders.forEach((h) => { counts[h] = (counts[h] ?? 0) + 1; });
    expect(counts['direction']).toBe(1);
    expect(counts['t30']).toBe(1);
    expect(counts['a30']).toBe(1);
    expect(counts['t60']).toBe(1); // appended fresh
  });

  it('schema_version write happens AFTER column extensions', () => {
    seedV1Workbook();
    migrateToV2();

    const setValuesLog = state.setValuesLog;
    const schemaWriteIdx = setValuesLog.findIndex((l) =>
      l.sheet === '_meta' && l.values.flat().some((v) => v === '2')
    );
    const pigColAddIdx = setValuesLog.findIndex((l) =>
      l.sheet === '_pigparse' && l.values.flat().includes('direction')
    );
    expect(schemaWriteIdx).toBeGreaterThan(-1);
    expect(pigColAddIdx).toBeGreaterThan(-1);
    expect(schemaWriteIdx).toBeGreaterThan(pigColAddIdx);
  });

  it('writes theme=minimalist and contact_email when absent (write-if-absent)', () => {
    // seed v1 WITHOUT the new theme/contact_email rows
    seedMeta(state, [
      ['schema_version', '1'],
      ['canonical_id', 'squirebot-v1-workbook-2026'],
    ]);
    state.sheets.set('_pigparse', makeSheet('_pigparse', ['item_id']));
    state.sheets.set('_item_master', makeSheet('_item_master', ['item_id']));
    state.sheets.set('_quest_items', makeSheet('_quest_items', ['item_id']));

    migrateToV2();
    const meta = state.sheets.get('_meta')!;
    const themeRow = meta.values.find((r) => r[0] === 'theme');
    const contactRow = meta.values.find((r) => r[0] === 'contact_email');
    expect(themeRow?.[1]).toBe('minimalist');
    expect(contactRow?.[1]).toBe('');
  });
});

describe('migrateToV3', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
  });

  function seedV2Workbook(): void {
    seedMeta(state, [
      ['schema_version', '2'],
      ['canonical_id', 'squirebot-v1-workbook-2026'],
      ['theme', 'minimalist'],
      ['contact_email', ''],
    ]);
    state.sheets.set('_char_owner', makeSheet('_char_owner', [
      'char_name', 'owner_email', 'display_name', 'discord_handle',
      'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
      'first_seen', 'last_seen', 'server', 'watcher_version',
    ]));
  }

  it('returns noop_already_current when schema_version is already 3', async () => {
    const { migrateToV3 } = await import('../lib/migrations');
    seedMeta(state, [['schema_version', '3'], ['canonical_id', 'x']]);
    expect(migrateToV3()).toBe('noop_already_current');
    expect(state.sheets.get('_char_owner')).toBeUndefined();
  });

  it('returns noop_unsupported_version on schema_version=1 (must run migrateToV2 first)', async () => {
    const { migrateToV3 } = await import('../lib/migrations');
    seedMeta(state, [['schema_version', '1']]);
    expect(migrateToV3()).toBe('noop_unsupported_version');
  });

  it('returns noop_unsupported_version on unknown schema_version', async () => {
    const { migrateToV3 } = await import('../lib/migrations');
    seedMeta(state, [['schema_version', '99']]);
    expect(migrateToV3()).toBe('noop_unsupported_version');
  });

  it('migrates v2 → v3 by appending race column and bumping schema_version', async () => {
    const { migrateToV3 } = await import('../lib/migrations');
    seedV2Workbook();
    expect(migrateToV3()).toBe('migrated');

    const charOwner = state.sheets.get('_char_owner')!;
    const headers = charOwner.values[0] as string[];
    expect(headers[headers.length - 1]).toBe('race');
    expect(headers.length).toBe(14);

    const meta = state.sheets.get('_meta')!;
    const schemaRow = meta.values.find((r) => r[0] === 'schema_version');
    expect(schemaRow![1]).toBe('3');
  });

  it('is idempotent on column-extension step (race already present)', async () => {
    const { migrateToV3 } = await import('../lib/migrations');
    seedV2Workbook();
    const charOwner = state.sheets.get('_char_owner')!;
    charOwner.values[0] = [...(charOwner.values[0] as string[]), 'race'];

    expect(migrateToV3()).toBe('migrated');
    const headers = charOwner.values[0] as string[];
    const counts: Record<string, number> = {};
    headers.forEach((h) => { counts[h] = (counts[h] ?? 0) + 1; });
    expect(counts['race']).toBe(1);
  });

  it('schema_version write happens AFTER column extension', async () => {
    const { migrateToV3 } = await import('../lib/migrations');
    seedV2Workbook();
    migrateToV3();

    const setValuesLog = state.setValuesLog;
    const schemaWriteIdx = setValuesLog.findIndex((l) =>
      l.sheet === '_meta' && l.values.flat().some((v) => v === '3')
    );
    const charColAddIdx = setValuesLog.findIndex((l) =>
      l.sheet === '_char_owner' && l.values.flat().includes('race')
    );
    expect(schemaWriteIdx).toBeGreaterThan(-1);
    expect(charColAddIdx).toBeGreaterThan(-1);
    expect(schemaWriteIdx).toBeGreaterThan(charColAddIdx);
  });
});
