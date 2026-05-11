// Phase 5 plan 05-02 task 2: weeklyEvictionArchive trigger.
//
// Reads _meta.eviction_log entries written by 05-04's commitEviction.
// moveCharToArchive is mocked here — the underlying archive path is
// covered by archive.test.ts. This file asserts iteration semantics,
// atomic-per-entry retry behaviour, and log-rewrite semantics.

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

vi.mock('../lib/archive', () => ({
  moveCharToArchive: vi.fn(),
}));

import { moveCharToArchive } from '../lib/archive';
import { weeklyEvictionArchive } from '../triggers/weeklyEvictionArchive';

const DAY_MS = 24 * 60 * 60 * 1000;

function entry(chars: string[], graceDaysFromNow: number, email = 'departed@x.com'): Record<string, unknown> {
  return {
    at: new Date(Date.now() - 31 * DAY_MS).toISOString(),
    email,
    initiated_by: 'officer@x.com',
    grace_until: new Date(Date.now() + graceDaysFromNow * DAY_MS).toISOString(),
    chars,
    reason: 'evicted',
  };
}

describe('weeklyEvictionArchive', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
    vi.mocked(moveCharToArchive).mockReset();
  });

  it('happy path: grace-expired entry processed, future entry kept', () => {
    const entryA = entry(['Findom', 'Slampeach'], -1);  // grace_until in past
    const entryB = entry(['Other'], 5);  // future
    seedMeta(state, [
      ['schema_version', '3'],
      ['eviction_log', JSON.stringify([entryA, entryB])],
    ]);

    weeklyEvictionArchive();

    expect(vi.mocked(moveCharToArchive)).toHaveBeenCalledTimes(2);
    const calls = vi.mocked(moveCharToArchive).mock.calls.map((c) => [c[0], c[1]]);
    expect(calls).toEqual([
      ['Findom', 'evicted'],
      ['Slampeach', 'evicted'],
    ]);

    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'eviction_log')!;
    const remaining = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
    expect(remaining.length).toBe(1);
    expect(remaining[0].chars).toEqual(['Other']);
  });

  it('no grace-expired entries: moveCharToArchive not called; log unchanged', () => {
    const entryB = entry(['Other'], 10);
    seedMeta(state, [
      ['schema_version', '3'],
      ['eviction_log', JSON.stringify([entryB])],
    ]);

    weeklyEvictionArchive();

    expect(vi.mocked(moveCharToArchive)).not.toHaveBeenCalled();
    const status = state.sheets.get('_status')!;
    const runRow = status.values.find((r) => r[0] === 'last_eviction_archive_run')!;
    expect(String(runRow[1])).toMatch(/^\d{4}-\d{2}-\d{2}T/);

    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'eviction_log')!;
    const remaining = JSON.parse(String(logRow[1])) as Array<unknown>;
    expect(remaining.length).toBe(1);
  });

  it('empty / missing eviction_log: no-op with _status update', () => {
    seedMeta(state, [['schema_version', '3']]);

    weeklyEvictionArchive();

    expect(vi.mocked(moveCharToArchive)).not.toHaveBeenCalled();
    const status = state.sheets.get('_status')!;
    expect(status.values.find((r) => r[0] === 'last_eviction_archive_run')).toBeDefined();
    expect(status.values.find((r) => r[0] === 'last_eviction_archive_count')![1]).toBe('0');
  });

  it('partial failure: entry stays in log when any char in it throws', () => {
    const entryA = entry(['BadChar', 'GoodChar'], -1);
    const entryB = entry(['Other'], 5);
    seedMeta(state, [
      ['schema_version', '3'],
      ['eviction_log', JSON.stringify([entryA, entryB])],
    ]);

    vi.mocked(moveCharToArchive).mockImplementation((charName: string) => {
      if (charName === 'BadChar') throw new Error('boom');
    });

    weeklyEvictionArchive();

    // Only BadChar was called (we break on failure within the entry).
    // GoodChar is NOT called — atomicity per entry.
    expect(vi.mocked(moveCharToArchive)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(moveCharToArchive).mock.calls[0][0]).toBe('BadChar');

    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'eviction_log')!;
    const remaining = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
    // Both entries stay: entry A because it failed and gets retried; entry B because grace not yet up.
    expect(remaining.length).toBe(2);
    expect((remaining[0].chars as string[])).toEqual(['BadChar', 'GoodChar']);
  });
});
