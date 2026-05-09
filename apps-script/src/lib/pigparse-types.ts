// PigParse REST API types and parser. Schema verified against live curl
// captured at apps-script/src/__fixtures__/pigparse-getall-1.json
// (7,240 rows on 2026-05-09). See 03-RESEARCH.md §1 for full field
// decoding.

import { log } from './log';

// 0 = WTS (sell), 1 = WTB (buy), 2 = BOTH (rare; not seen in fixture).
// Confirmed against pigparse-swagger-v1.json's ItemSummary enum.
export type PigparseDirection = 0 | 1 | 2;

export interface PigparseRowRaw {
  i: number;        // EQ item ID
  t: PigparseDirection;
  n: string;        // item name
  l: string;        // ISO 8601 last seen
  tc: number; ta: number;        // today (always 0 in current PigParse)
  t30: number; a30: number;      // 30-day count + avg pp
  t60: number; a60: number;
  t90: number; a90: number;
  t6m: number; a6m: number;
  ty: number; ay: number;        // year
}

const REQUIRED_KEYS: (keyof PigparseRowRaw)[] = ['i', 't', 'n'];
const NUMERIC_KEYS: (keyof PigparseRowRaw)[] = [
  'i', 't', 'tc', 'ta', 't30', 'a30', 't60', 'a60',
  't90', 'a90', 't6m', 'a6m', 'ty', 'ay',
];
const MALFORMATION_TOLERANCE_PCT = 0.01;

// parseToRows takes the raw response body, JSON.parses, validates the
// shape, coerces numerics, and returns the validated array. Throws if:
//   - body is not a JSON array
//   - more than 1% of rows are malformed (missing required keys, t out
//     of range, etc.)
// Tolerates up to 1% malformed rows (silently skipped + logged) since
// PigParse occasionally has weird historical entries.
export function parseToRows(body: string): PigparseRowRaw[] {
  let raw: unknown;
  try {
    raw = JSON.parse(body);
  } catch (e) {
    throw new Error(`PigParse response not valid JSON: ${(e as Error).message}`);
  }
  if (!Array.isArray(raw)) {
    throw new Error(`PigParse response not an array (got ${typeof raw})`);
  }

  const accepted: PigparseRowRaw[] = [];
  let skipped = 0;
  for (const item of raw) {
    if (!isValidRow(item)) {
      skipped++;
      continue;
    }
    accepted.push(coerceRow(item));
  }

  if (raw.length > 0 && skipped / raw.length > MALFORMATION_TOLERANCE_PCT) {
    throw new Error(`PigParse parse: too many malformed rows (${skipped}/${raw.length} skipped, threshold ${MALFORMATION_TOLERANCE_PCT * 100}%)`);
  }
  if (skipped > 0) {
    log('warn', 'parseToRows', { skipped, total: raw.length });
  }
  return accepted;
}

function isValidRow(item: unknown): boolean {
  if (typeof item !== 'object' || item === null) return false;
  const obj = item as Record<string, unknown>;
  for (const k of REQUIRED_KEYS) {
    if (!(k in obj)) return false;
  }
  if (typeof obj.i !== 'number') return false;
  if (obj.t !== 0 && obj.t !== 1 && obj.t !== 2) return false;
  if (typeof obj.n !== 'string' || obj.n.length === 0) return false;
  return true;
}

function coerceRow(item: unknown): PigparseRowRaw {
  const obj = item as Record<string, unknown>;
  const out: Record<string, unknown> = {
    i: obj.i,
    t: obj.t,
    n: String(obj.n).trim(),
    l: typeof obj.l === 'string' ? obj.l : '',
  };
  for (const k of NUMERIC_KEYS) {
    if (k === 'i' || k === 't') continue;
    const v = obj[k];
    out[k] = typeof v === 'number' && Number.isFinite(v) ? v : 0;
  }
  return out as unknown as PigparseRowRaw;
}
