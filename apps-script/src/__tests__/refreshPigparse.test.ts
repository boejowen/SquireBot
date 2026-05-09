import { describe, it, expect, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { refreshPigparse } from '../triggers/refreshPigparse';
import {
  resetMocks, seedMeta, makeSheet, type MockState,
} from './test-helpers';

const FIXTURE_PATH = resolve(__dirname, '../__fixtures__/pigparse-getall-1.json');
const FIXTURE_BODY = readFileSync(FIXTURE_PATH, 'utf8');
const PIGPARSE_URL = 'https://pigparse.azurewebsites.net/api/item/getall/1';

const PIGPARSE_HEADERS = [
  'item_id', 'name', 'current_avg', 'last_seen', 'blue_volume', 'last_refreshed',
  'direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay',
];

describe('refreshPigparse', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '2'],
      ['canonical_id', 'squirebot-v1-workbook-2026'],
      ['theme', 'minimalist'],
      ['contact_email', ''],
      ['last_pigparse_refresh', ''],
      ['last_error', '{}'],
    ]);
    state.sheets.set('_pigparse', makeSheet('_pigparse', PIGPARSE_HEADERS));
    state.sheets.set('_status', makeSheet('_status', ['key', 'value'], [
      ['last_pigparse_row_count', '0'],
      ['last_error', '{}'],
    ]));
  });

  it('happy path: writes 7,240 rows from fixture, updates meta + status', () => {
    state.fetchResponses.set(PIGPARSE_URL, { status: 200, body: FIXTURE_BODY, headers: {} });
    refreshPigparse();

    const pig = state.sheets.get('_pigparse')!;
    // 7,240 data rows + 1 header
    expect(pig.values.length).toBe(7241);
    // First data row matches fixture: i=19178, name="10 Dose Adrenaline Tap"
    expect(pig.values[1][0]).toBe(19178);
    expect(pig.values[1][1]).toBe('10 Dose Adrenaline Tap');
    // direction (col 7, 0-indexed 6) = 0 (WTS) for the first row
    expect(pig.values[1][6]).toBe(0);

    const meta = state.sheets.get('_meta')!;
    const refreshRow = meta.values.find((r) => r[0] === 'last_pigparse_refresh');
    expect(refreshRow).toBeDefined();
    expect(refreshRow![1]).toMatch(/^\d{4}-\d{2}-\d{2}T/); // ISO timestamp written

    const status = state.sheets.get('_status')!;
    const countRow = status.values.find((r) => r[0] === 'last_pigparse_row_count');
    expect(countRow![1]).toBe('7240');

    // Errors cleared on success
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toBe('{}');
  });

  it('lock contention: returns silently, writes lock_busy error', () => {
    state.lockTryLockReturn = false;
    refreshPigparse();
    expect(state.fetchCalls.length).toBe(0); // no fetch attempted
    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toContain('lock_busy');
  });

  it('truncation: refuses to overwrite, writes truncated_response error, preserves lastCount', () => {
    // Seed lastCount as 1000; respond with only 500 rows (50% < 90% floor)
    const status = state.sheets.get('_status')!;
    const countRow = status.values.find((r) => r[0] === 'last_pigparse_row_count')!;
    countRow[1] = '1000';

    const truncatedBody = JSON.stringify(
      Array.from({ length: 500 }, (_, k) => ({
        i: k, t: 0, n: `Item ${k}`, l: '2026-05-09T00:00:00Z',
        tc: 0, ta: 0, t30: 0, a30: 0, t60: 0, a60: 0,
        t90: 0, a90: 0, t6m: 0, a6m: 0, ty: 0, ay: 0,
      }))
    );
    state.fetchResponses.set(PIGPARSE_URL, { status: 200, body: truncatedBody, headers: {} });

    refreshPigparse();

    // _pigparse data rows untouched (just header)
    const pig = state.sheets.get('_pigparse')!;
    expect(pig.values.length).toBe(1);

    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toContain('truncated_response');
    expect(errRow![1]).toContain('today=500');
    expect(errRow![1]).toContain('last=1000');

    // lastCount preserved (NOT overwritten with the bogus 500)
    expect(countRow[1]).toBe('1000');
  });

  it('fetch failure: writes fetch_failed error, no _pigparse change', () => {
    state.fetchResponses.set(PIGPARSE_URL, { status: 500, body: 'oops', headers: {} });
    refreshPigparse();
    const pig = state.sheets.get('_pigparse')!;
    expect(pig.values.length).toBe(1); // header only

    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toContain('fetch_failed');
    expect(errRow![1]).toContain('500');
  });

  it('parse failure: writes parse_failed error', () => {
    state.fetchResponses.set(PIGPARSE_URL, { status: 200, body: '{"not": "an array"}', headers: {} });
    refreshPigparse();
    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error');
    expect(errRow![1]).toContain('parse_failed');
  });

  it('first-ever run: lastCount=0 means the floor check is skipped', () => {
    // Seed lastCount=0; respond with 50 rows. Should NOT trigger truncation.
    const tinyBody = JSON.stringify(
      Array.from({ length: 50 }, (_, k) => ({
        i: k, t: 0, n: `Item ${k}`, l: '2026-05-09T00:00:00Z',
        tc: 0, ta: 0, t30: 0, a30: 0, t60: 0, a60: 0,
        t90: 0, a90: 0, t6m: 0, a6m: 0, ty: 0, ay: 0,
      }))
    );
    state.fetchResponses.set(PIGPARSE_URL, { status: 200, body: tinyBody, headers: {} });
    refreshPigparse();
    const pig = state.sheets.get('_pigparse')!;
    expect(pig.values.length).toBe(51); // 50 data + 1 header
    const status = state.sheets.get('_status')!;
    const countRow = status.values.find((r) => r[0] === 'last_pigparse_row_count');
    expect(countRow![1]).toBe('50');
  });

  it('idempotent re-run: second run reads new lastCount and accepts same row count', () => {
    state.fetchResponses.set(PIGPARSE_URL, { status: 200, body: FIXTURE_BODY, headers: {} });
    refreshPigparse();
    const pig1Length = state.sheets.get('_pigparse')!.values.length;

    refreshPigparse(); // run 2
    const pig2 = state.sheets.get('_pigparse')!;
    expect(pig2.values.length).toBe(pig1Length); // same — full replace, no duplication
  });
});
