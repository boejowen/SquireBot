import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  getBankCoinForForm, saveBankCoin,
} from '../triggers/showBankCoinSidebar';
import { resetMocks, makeSheet, seedMeta, type MockState } from './test-helpers';

// buildBank touches several sheets; stub it for sidebar tests so we can
// assert the saveBankCoin contract without dragging the bank tab pipeline.
vi.mock('../tabs/buildBank', () => ({
  buildBank: vi.fn(),
}));

import { buildBank as mockedBuildBank } from '../tabs/buildBank';

describe('getBankCoinForForm', () => {
  let state: MockState;
  beforeEach(() => { state = resetMocks(); });

  it('returns all zeros when _meta missing', () => {
    expect(getBankCoinForForm()).toEqual({ pp: 0, gp: 0, sp: 0, cp: 0 });
  });

  it('returns all zeros when _meta has no bank_coin_* rows', () => {
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
    expect(getBankCoinForForm()).toEqual({ pp: 0, gp: 0, sp: 0, cp: 0 });
  });

  it('returns parsed values when rows present', () => {
    seedMeta(state, [
      ['schema_version', '3'],
      ['bank_coin_pp', '12345'],
      ['bank_coin_gp', '67'],
      ['bank_coin_sp', '8'],
      ['bank_coin_cp', '0'],
    ]);
    expect(getBankCoinForForm()).toEqual({ pp: 12345, gp: 67, sp: 8, cp: 0 });
  });

  it('treats unparseable / negative values as 0', () => {
    seedMeta(state, [
      ['bank_coin_pp', 'abc'],
      ['bank_coin_gp', '-5'],
      ['bank_coin_sp', ''],
      ['bank_coin_cp', '7'],
    ]);
    expect(getBankCoinForForm()).toEqual({ pp: 0, gp: 0, sp: 0, cp: 7 });
  });
});

describe('saveBankCoin', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    vi.mocked(mockedBuildBank).mockClear();
    seedMeta(state, [['schema_version', '3']]);
  });

  it('writes 4 coin rows + bank_coin_last_updated and fires buildBank', () => {
    saveBankCoin({ pp: 1000, gp: 2, sp: 3, cp: 4 });
    const meta = state.sheets.get('_meta')!.values;
    const find = (k: string): string => {
      const row = meta.find((r) => r[0] === k);
      return row ? String(row[1]) : '<missing>';
    };
    expect(find('bank_coin_pp')).toBe('1000');
    expect(find('bank_coin_gp')).toBe('2');
    expect(find('bank_coin_sp')).toBe('3');
    expect(find('bank_coin_cp')).toBe('4');
    expect(find('bank_coin_last_updated')).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    expect(mockedBuildBank).toHaveBeenCalledTimes(1);
  });

  it('rejects negative values', () => {
    expect(() => saveBankCoin({ pp: -1, gp: 0, sp: 0, cp: 0 }))
      .toThrow(/invalid pp/);
    expect(mockedBuildBank).not.toHaveBeenCalled();
  });

  it('rejects NaN', () => {
    expect(() => saveBankCoin({ pp: 0, gp: NaN, sp: 0, cp: 0 }))
      .toThrow(/invalid gp/);
  });

  it('rejects Infinity', () => {
    expect(() => saveBankCoin({ pp: 0, gp: 0, sp: Infinity, cp: 0 }))
      .toThrow(/invalid sp/);
  });

  it('lock contention throws', () => {
    state.lockTryLockReturn = false;
    expect(() => saveBankCoin({ pp: 0, gp: 0, sp: 0, cp: 0 }))
      .toThrow(/lock_busy/);
    expect(mockedBuildBank).not.toHaveBeenCalled();
  });

  it('zero values are accepted (clear-the-bank case)', () => {
    saveBankCoin({ pp: 0, gp: 0, sp: 0, cp: 0 });
    const meta = state.sheets.get('_meta')!.values;
    const find = (k: string): string => {
      const row = meta.find((r) => r[0] === k);
      return row ? String(row[1]) : '<missing>';
    };
    expect(find('bank_coin_pp')).toBe('0');
    expect(mockedBuildBank).toHaveBeenCalledTimes(1);
  });

  it('updates existing rows in place (idempotent re-save)', () => {
    saveBankCoin({ pp: 100, gp: 0, sp: 0, cp: 0 });
    saveBankCoin({ pp: 200, gp: 0, sp: 0, cp: 0 });
    const meta = state.sheets.get('_meta')!.values;
    const ppRows = meta.filter((r) => r[0] === 'bank_coin_pp');
    expect(ppRows.length).toBe(1);
    expect(ppRows[0][1]).toBe('200');
  });

  it('first save creates 4 protections (closes lazy-creation gap)', () => {
    // _meta has only schema_version — no bank_coin_* rows yet.
    // protectBankCoinCells called from migrateToV3 / installTriggers
    // would be a no-op at this point. saveBankCoin must apply protection
    // itself after creating the rows; otherwise the user gets no
    // protection warning until they manually re-run Install Triggers.
    expect(state.sheets.get('_meta')!.protections).toBeUndefined();
    saveBankCoin({ pp: 1000, gp: 0, sp: 0, cp: 0 });
    const meta = state.sheets.get('_meta')!;
    expect(meta.protections?.length).toBe(4);
    expect(meta.protections?.every((p) => p.warningOnly)).toBe(true);
  });

  it('second save does not duplicate protections (idempotent)', () => {
    saveBankCoin({ pp: 100, gp: 0, sp: 0, cp: 0 });
    saveBankCoin({ pp: 200, gp: 0, sp: 0, cp: 0 });
    expect(state.sheets.get('_meta')!.protections?.length).toBe(4);
  });
});
