// Phase 5 plan 05-02 task 2: weeklyStaleCharArchive trigger.
//
// moveCharToArchive is mocked here via vi.mock — the actual lock + tab
// snapshot path is covered by archive.test.ts. This file asserts the
// trigger's iteration semantics and _status book-keeping.

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

vi.mock('../lib/archive', () => ({
  moveCharToArchive: vi.fn(),
}));

import { moveCharToArchive } from '../lib/archive';
import { weeklyStaleCharArchive } from '../triggers/weeklyStaleCharArchive';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version',
];

const DAY_MS = 24 * 60 * 60 * 1000;

function row(charName: string, opts: { daysAgo: number; isRemoved?: boolean | string }): unknown[] {
  return [
    charName, `${charName.toLowerCase()}@x.com`, charName, '',
    'Shaman', 60, false, false, opts.isRemoved ?? false,
    '2025-11-01T00:00:00Z',
    new Date(Date.now() - opts.daysAgo * DAY_MS).toISOString(),
    'blue', '0.4.0',
  ];
}

describe('weeklyStaleCharArchive', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3']]);
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
    vi.mocked(moveCharToArchive).mockReset();
  });

  it('happy path: 2 chars >90d → moveCharToArchive called twice, _status updated', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', { daysAgo: 91 }),
      row('Slampeach', { daysAgo: 5 }),
      row('Skinhouse', { daysAgo: 100 }),
    ]));

    weeklyStaleCharArchive();

    expect(vi.mocked(moveCharToArchive)).toHaveBeenCalledTimes(2);
    const callArgs = vi.mocked(moveCharToArchive).mock.calls.map((c) => c[0]);
    expect(callArgs.sort()).toEqual(['Findom', 'Skinhouse']);
    expect(vi.mocked(moveCharToArchive).mock.calls.every((c) => c[1] === 'stale_90d')).toBe(true);

    const status = state.sheets.get('_status')!;
    const runRow = status.values.find((r) => r[0] === 'last_stale_archive_run')!;
    const countRow = status.values.find((r) => r[0] === 'last_stale_archive_count')!;
    expect(String(runRow[1])).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    expect(countRow[1]).toBe('2');
  });

  it('no stale chars: moveCharToArchive not called; _status.count=0', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', { daysAgo: 10 }),
      row('Slampeach', { daysAgo: 5 }),
    ]));

    weeklyStaleCharArchive();

    expect(vi.mocked(moveCharToArchive)).not.toHaveBeenCalled();
    const status = state.sheets.get('_status')!;
    const countRow = status.values.find((r) => r[0] === 'last_stale_archive_count')!;
    expect(countRow[1]).toBe('0');
    const runRow = status.values.find((r) => r[0] === 'last_stale_archive_run')!;
    expect(String(runRow[1])).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  it('skips chars where is_removed=TRUE even if >90d', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', { daysAgo: 100, isRemoved: true }),
      row('Slampeach', { daysAgo: 95 }),
    ]));

    weeklyStaleCharArchive();

    expect(vi.mocked(moveCharToArchive)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(moveCharToArchive).mock.calls[0][0]).toBe('Slampeach');
  });

  it('missing _char_owner: writes _meta.last_error tab_missing envelope and returns', () => {
    // No _char_owner seeded.
    weeklyStaleCharArchive();

    expect(vi.mocked(moveCharToArchive)).not.toHaveBeenCalled();
    const meta = state.sheets.get('_meta')!;
    const errRow = meta.values.find((r) => r[0] === 'last_error')!;
    expect(errRow).toBeDefined();
    const err = JSON.parse(String(errRow[1]));
    expect(err.kind).toBe('tab_missing');
    expect(err.where).toBe('weeklyStaleCharArchive');
    expect(err.detail).toBe('_char_owner');
  });
});
