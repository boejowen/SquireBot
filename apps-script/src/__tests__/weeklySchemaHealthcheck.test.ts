import { describe, it, expect, beforeEach } from 'vitest';
import { weeklySchemaHealthcheck } from '../triggers/weeklySchemaHealthcheck';
import { resetMocks, makeSheet, type MockState } from './test-helpers';

// Phase 5 plan 05-01 (OPS-06). Weekly Sun-03:00 PT trigger that verifies
// all 13 EXPECTED_TABS exist by sheet ID (resilient to user-renames per
// RESEARCH §Pitfall P7). On missing-tab: dual-writes structured JSON to
// _meta.last_error + _status.last_error (envelope identical to
// monitorCellCount). On clean run: writes _status.last_schema_check +
// _status.last_schema_check_status = 'ok'.

const EXPECTED_TABS = [
  '_meta', '_char_owner', '_item_master', '_pigparse',
  '_wiki_spells', '_wiki_gear_tier', '_quest_items', '_audit', '_status',
  'view', 'gear_check', 'spell_check', 'bank',
];

function seedAllExpectedTabs(state: MockState): void {
  // _meta + _status are KV-shaped; rest are stub headers.
  state.sheets.set('_meta', makeSheet('_meta', ['key', 'value'], [
    ['schema_version', '3'],
  ]));
  state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
  for (const name of EXPECTED_TABS) {
    if (state.sheets.has(name)) continue;
    state.sheets.set(name, makeSheet(name, ['col1']));
  }
}

describe('weeklySchemaHealthcheck', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
  });

  it('first-run backfill: writes _meta.expected_sheet_ids with all 13 entries and _status.last_schema_check_status=ok', () => {
    seedAllExpectedTabs(state);

    weeklySchemaHealthcheck();

    const meta = state.sheets.get('_meta')!;
    const idsRow = meta.values.find((r) => r[0] === 'expected_sheet_ids');
    expect(idsRow).toBeDefined();
    const idsMap = JSON.parse(String(idsRow![1])) as Record<string, number>;
    expect(Object.keys(idsMap).sort()).toEqual([...EXPECTED_TABS].sort());

    const status = state.sheets.get('_status')!;
    const okRow = status.values.find((r) => r[0] === 'last_schema_check_status');
    expect(String(okRow![1])).toBe('ok');
  });

  it('all-present steady state: rewrites _status.last_schema_check timestamp + status=ok; no last_error', () => {
    seedAllExpectedTabs(state);

    // First run backfills expected_sheet_ids.
    weeklySchemaHealthcheck();

    // Second run: steady state. last_error must NOT be set.
    weeklySchemaHealthcheck();

    const status = state.sheets.get('_status')!;
    const tsRow = status.values.find((r) => r[0] === 'last_schema_check');
    expect(String(tsRow![1])).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    const okRow = status.values.find((r) => r[0] === 'last_schema_check_status');
    expect(String(okRow![1])).toBe('ok');
    const errRow = status.values.find((r) => r[0] === 'last_error');
    expect(errRow).toBeUndefined();
  });

  it('one tab missing by id: writes tab_missing _meta.last_error + _status.last_error', () => {
    seedAllExpectedTabs(state);
    weeklySchemaHealthcheck();  // backfill

    // Delete _pigparse — its ID is now unresolvable.
    state.sheets.delete('_pigparse');

    weeklySchemaHealthcheck();

    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow).toBeDefined();
    const err = JSON.parse(String(errRow![1]));
    expect(err.where).toBe('weeklySchemaHealthcheck');
    expect(err.kind).toBe('tab_missing');
    expect(err.detail).toBe('_pigparse');
    expect(err.at).toMatch(/^\d{4}-\d{2}-\d{2}T/);

    // Dual-write to _status mirrors the same JSON.
    const status = state.sheets.get('_status')!;
    const statusErrRow = status.values.find((r) => r[0] === 'last_error');
    expect(statusErrRow).toBeDefined();
    expect(JSON.parse(String(statusErrRow![1])).kind).toBe('tab_missing');
  });

  it('multiple tabs missing: detail is comma-separated', () => {
    seedAllExpectedTabs(state);
    weeklySchemaHealthcheck();  // backfill

    state.sheets.delete('_pigparse');
    state.sheets.delete('_quest_items');

    weeklySchemaHealthcheck();

    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    const err = JSON.parse(String(errRow![1]));
    expect(err.detail.split(',').sort()).toEqual(['_pigparse', '_quest_items'].sort());
  });

  it('_archive is NOT in EXPECTED_TABS (lazy creation by archive lib in 05-02)', () => {
    seedAllExpectedTabs(state);
    weeklySchemaHealthcheck();

    const meta = state.sheets.get('_meta')!;
    const idsRow = meta.values.find((r) => r[0] === 'expected_sheet_ids');
    const idsMap = JSON.parse(String(idsRow![1])) as Record<string, number>;
    expect(idsMap['_archive']).toBeUndefined();

    // And no last_error fires because of the absent _archive.
    const status = state.sheets.get('_status')!;
    const okRow = status.values.find((r) => r[0] === 'last_schema_check_status');
    expect(String(okRow![1])).toBe('ok');
  });
});
