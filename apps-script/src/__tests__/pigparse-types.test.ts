import { describe, it, expect, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseToRows, type PigparseRowRaw } from '../lib/pigparse-types';
import { resetMocks } from './test-helpers';

const FIXTURE_PATH = resolve(__dirname, '../__fixtures__/pigparse-getall-1.json');
const FIXTURE_BODY = readFileSync(FIXTURE_PATH, 'utf8');

describe('parseToRows (live PigParse fixture)', () => {
  beforeEach(() => { resetMocks(); });

  it('parses 7,240 rows from the live fixture', () => {
    const rows = parseToRows(FIXTURE_BODY);
    expect(rows.length).toBe(7240);
  });

  it('distinct t values are exactly [0, 1] in the fixture', () => {
    const rows = parseToRows(FIXTURE_BODY);
    const ts = new Set(rows.map((r) => r.t));
    expect(Array.from(ts).sort()).toEqual([0, 1]);
  });

  it('first row matches the known fixture shape (decoded numerics)', () => {
    const rows = parseToRows(FIXTURE_BODY);
    const first = rows[0];
    expect(first.i).toBe(19178);
    expect(first.t).toBe(0);
    expect(first.n).toBe('10 Dose Adrenaline Tap');
    expect(first.l).toBe('2026-01-02T22:56:07.581+00:00');
  });

  it('every row has all 16 expected fields', () => {
    const rows = parseToRows(FIXTURE_BODY);
    const expectedKeys: (keyof PigparseRowRaw)[] = [
      'i', 't', 'n', 'l', 'tc', 'ta', 't30', 'a30',
      't60', 'a60', 't90', 'a90', 't6m', 'a6m', 'ty', 'ay',
    ];
    for (const r of rows.slice(0, 100)) {
      for (const k of expectedKeys) {
        expect(r).toHaveProperty(k);
      }
    }
  });
});

describe('parseToRows (synthetic edge cases)', () => {
  beforeEach(() => { resetMocks(); });

  it('throws on non-array body', () => {
    expect(() => parseToRows('{"items": []}')).toThrow(/not an array/);
  });

  it('throws on invalid JSON', () => {
    expect(() => parseToRows('not json at all')).toThrow(/not valid JSON/);
  });

  it('skips malformed rows below 1% tolerance', () => {
    const good = { i: 1, t: 0, n: 'X', l: '', tc: 0, ta: 0,
      t30: 0, a30: 0, t60: 0, a60: 0, t90: 0, a90: 0,
      t6m: 0, a6m: 0, ty: 0, ay: 0 };
    const arr = [];
    for (let k = 0; k < 200; k++) arr.push({ ...good, i: k });
    arr.push({ i: 'not-a-number', t: 0, n: 'X' });  // 1 malformed in 201
    const rows = parseToRows(JSON.stringify(arr));
    expect(rows.length).toBe(200);
  });

  it('throws when malformed row count exceeds 1% tolerance', () => {
    const arr: unknown[] = [];
    for (let k = 0; k < 100; k++) arr.push({ i: 1, t: 0, n: 'X' });
    for (let k = 0; k < 5; k++) arr.push({ i: 'bad', t: 0, n: 'X' });
    expect(() => parseToRows(JSON.stringify(arr))).toThrow(/too many malformed/);
  });

  it('rejects rows with t outside the {0,1,2} enum', () => {
    const arr = [{ i: 1, t: 7, n: 'X' }];
    expect(() => parseToRows(JSON.stringify(arr))).toThrow(/too many malformed/);
  });

  it('coerces missing numeric fields to 0', () => {
    const arr = [{ i: 1, t: 0, n: 'X' }]; // no t30/a30/etc.
    const rows = parseToRows(JSON.stringify(arr));
    expect(rows[0].t30).toBe(0);
    expect(rows[0].ay).toBe(0);
  });

  it('returns empty array for empty input array', () => {
    expect(parseToRows('[]')).toEqual([]);
  });
});
