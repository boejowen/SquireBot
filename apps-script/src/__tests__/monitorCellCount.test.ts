import { describe, it, expect, beforeEach } from 'vitest';
import { monitorCellCount } from '../triggers/monitorCellCount';
import { resetMocks, makeSheet, type MockState } from './test-helpers';

describe('monitorCellCount', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    state.sheets.set('_status', makeSheet('_status', ['key', 'value']));
    state.sheets.set('_meta', makeSheet('_meta', ['key', 'value'], [
      ['schema_version', '3'],
    ]));
  });

  it('empty workbook (only KV sheets): cell_count = small, no alarm', () => {
    monitorCellCount();
    const status = state.sheets.get('_status')!;
    const cellCountRow = status.values.find((r) => r[0] === 'cell_count');
    expect(cellCountRow).toBeDefined();
    // _status: header row + cell_count row → 2 rows × 2 cols = 4
    // _meta: 2 rows × 2 cols = 4
    // Both small; no alarm.
    const total = parseInt(String(cellCountRow![1]), 10);
    expect(total).toBeGreaterThan(0);
    expect(total).toBeLessThan(100);
    const errRow = status.values.find((r) => r[0] === 'last_error');
    expect(errRow).toBeUndefined();
  });

  it('writes cell_count_last_check timestamp on every run', () => {
    monitorCellCount();
    const status = state.sheets.get('_status')!;
    const tsRow = status.values.find((r) => r[0] === 'cell_count_last_check');
    expect(String(tsRow![1])).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  it('threshold trip: total > 5M writes cell_count_threshold last_error', () => {
    // Build a sheet with synthetic dimensions that report large rows*cols.
    // The mock's getLastRow uses values.length and getLastColumn uses
    // values[0].length, so create a sheet with values shaped for big counts.
    const huge: unknown[][] = [];
    for (let i = 0; i < 5001; i++) {
      huge.push(new Array(1001).fill(''));  // 5001 × 1001 = 5,006,001 cells
    }
    const hugeSheet = { name: 'inv:big', values: huge, notes: huge.map(r => r.map(() => null)) };
    state.sheets.set('inv:big', hugeSheet as never);

    monitorCellCount();

    const status = state.sheets.get('_status')!;
    const errRow = status.values.find((r) => r[0] === 'last_error');
    expect(errRow).toBeDefined();
    const err = JSON.parse(String(errRow![1]));
    expect(err.kind).toBe('cell_count_threshold');
    expect(err.where).toBe('monitorCellCount');
    expect(err.detail).toContain('inv:big=5006001');
    expect(err.detail).toContain('/10000000');

    // _meta.last_error mirror written
    const meta = state.sheets.get('_meta')!;
    const metaErrRow = meta.values.find((r) => r[0] === 'last_error');
    expect(metaErrRow).toBeDefined();
  });

  it('top-5 reporting includes the 5 largest sheets in descending order', () => {
    // Add 7 sheets of varying sizes so top-5 sort is meaningful + we trip
    // the alarm threshold (sum needs > 5M to surface the detail string).
    const sizes = [2_500_000, 1_500_000, 1_000_000, 500_000, 250_000, 100_000, 50_000];
    sizes.forEach((cells, i) => {
      // Pick (rows, cols) so rows*cols ≈ cells. Use cols=10 to keep rows
      // count reasonable; rows = cells / 10.
      const rows = cells / 10;
      const values: unknown[][] = [];
      for (let r = 0; r < rows; r++) values.push(new Array(10).fill(''));
      state.sheets.set(`s${i}`, {
        name: `s${i}`, values, notes: values.map(r => r.map(() => null)),
      } as never);
    });

    monitorCellCount();

    const status = state.sheets.get('_status')!;
    const errRow = status.values.find((r) => r[0] === 'last_error')!;
    const err = JSON.parse(String(errRow[1]));
    // Top-5 = s0..s4 (descending). s5, s6 should NOT appear in detail.
    expect(err.detail).toContain('s0=2500000');
    expect(err.detail).toContain('s1=1500000');
    expect(err.detail).toContain('s2=1000000');
    expect(err.detail).toContain('s3=500000');
    expect(err.detail).toContain('s4=250000');
    expect(err.detail).not.toContain('s5=');
    expect(err.detail).not.toContain('s6=');
  });
});
