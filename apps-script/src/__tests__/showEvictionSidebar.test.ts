// Phase 5 plan 05-04 task 1 — vitest scenarios for the eviction sidebar
// trigger. Coverage per plan <behavior>:
//   Test 1   sidebar opens at 300px with locked title + body strings
//   Test 2   getEvictionEmails happy path — distinct sorted active emails
//   Test 3   getEvictionEmails — partial removal NOT excluded
//   Test 4   previewEviction happy path — chars + graceUntil ISO+30d
//   Test 5   previewEviction — no chars (does NOT throw)
//   Test 6   commitEviction happy path — flips is_removed=TRUE +
//            appends eviction_log entry + returns {affected, graceUntil}
//   Test 7   commitEviction idempotent — does not toggle already-removed
//   Test 8   commitEviction lock failure — throws, no writes
//   Test 9   commitEviction missing _char_owner — throws
//   Test 10  commitEviction invalid email — throws
//   Test 11  commitEviction appends to existing log
//   Test 12  initiated_by fallback — empty Session.getEffectiveUser email
//            → 'unknown'

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  showEvictionSidebar,
  getEvictionEmails,
  previewEviction,
  commitEviction,
} from '../triggers/showEvictionSidebar';
import { resetMocks, makeSheet, seedMeta, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version',
];

// Build a 13-col _char_owner row. is_removed at col index 8 (1-based 9).
function row(charName: string, email: string, isRemoved: boolean | string = false): unknown[] {
  return [
    charName, email, charName, '',
    'SHD', 60, false, false, isRemoved,
    '2026-04-01T00:00:00Z', '2026-05-01T00:00:00Z', 'blue', '0.4.0',
  ];
}

function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

// Phase 7 plan 07-03: seed _meta.guild_admins + _meta.workbook_owner_floor
// alongside schema_version so the post-guard eviction sidebar (opener +
// getEvictionEmails/previewEviction/commitEviction callbacks) admits the
// test's installSessionMock-mocked caller. Mirrors what bootstrapGuildAdmins
// would write on first open. seedMeta REPLACES the entire _meta sheet, so
// this helper is the single seeder for these tests; callers that need extra
// rows (Test 11 — pre-existing eviction_log) should use seedMetaWithAdmins
// with the extra rows in one call.
function seedMetaWithAdmins(
  state: MockState,
  adminEmails: string[],
  extraRows: Array<[string, string]> = [],
  floor?: string,
): void {
  const normalized = adminEmails.map((e) => e.toLowerCase().trim()).sort();
  const rows: Array<[string, string]> = [
    ['schema_version', '3'],
    ['guild_admins', JSON.stringify(normalized)],
    ['workbook_owner_floor', (floor ?? normalized[0] ?? '').toLowerCase().trim()],
    ...extraRows,
  ];
  seedMeta(state, rows);
}

describe('showEvictionSidebar — open + width + title + body', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMetaWithAdmins(state, ['officer@example.com']);
    installSessionMock('officer@example.com');
  });

  it('Test 1 — opens 300px-wide sidebar with locked title + body strings', () => {
    showEvictionSidebar();
    const captured = (state as MockState & { lastSidebar?: { _html: string; _title: string; _width: number } }).lastSidebar;
    expect(captured).toBeDefined();
    expect(captured!._width).toBe(300);
    expect(captured!._title).toBe('SquireBot — Evict guildie');
    expect(captured!._html).toContain('Evict guildie');                           // h3 title
    expect(captured!._html).toContain('Choose…');                                  // placeholder
    expect(captured!._html).toContain("Mark a departed guildie's characters as removed"); // desc
    expect(captured!._html).toContain('Grace expires:');                           // grace read-back
    expect(captured!._html).toContain('Evict');                                    // primary CTA
  });
});

describe('getEvictionEmails', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMetaWithAdmins(state, ['officer@example.com']);
    installSessionMock('officer@example.com');
  });

  it('Test 2 — returns distinct sorted active emails; excludes fully-removed', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
      row('Slampeach', 'b@x'),
      row('Abulus', 'a@x'),         // a@x repeated — distinct
      row('Departed', 'c@x', true), // c@x fully removed → excluded
    ]));
    const out = getEvictionEmails();
    expect(out).toEqual(['a@x', 'b@x']);
  });

  it('Test 3 — partial removal does NOT exclude the email', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x', true),   // a@x: one char removed
      row('Abulus', 'a@x', false),  // a@x: one char active
    ]));
    const out = getEvictionEmails();
    expect(out).toEqual(['a@x']);
  });
});

describe('previewEviction', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMetaWithAdmins(state, ['officer@example.com']);
    installSessionMock('officer@example.com');
  });

  it('Test 4 — happy path: returns chars (sorted) + graceUntil ISO+30d', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
      row('Slampeach', 'a@x'),
      row('Abulus', 'a@x'),
      row('Other', 'b@x'),
    ]));
    const before = Date.now();
    const out = previewEviction('a@x');
    const after = Date.now();
    expect(out.chars).toEqual(['Abulus', 'Findom', 'Slampeach']);
    const graceMs = new Date(out.graceUntil).getTime();
    const expectedMin = before + 30 * 24 * 60 * 60 * 1000;
    const expectedMax = after + 30 * 24 * 60 * 60 * 1000;
    expect(graceMs).toBeGreaterThanOrEqual(expectedMin);
    expect(graceMs).toBeLessThanOrEqual(expectedMax);
  });

  it('Test 5 — no chars: returns empty list with graceUntil (does NOT throw)', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Other', 'b@x'),
    ]));
    const out = previewEviction('a@x');
    expect(out.chars).toEqual([]);
    expect(typeof out.graceUntil).toBe('string');
    expect(out.graceUntil).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });
});

describe('commitEviction', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMetaWithAdmins(state, ['officer@example.com']);
    installSessionMock('officer@example.com');
  });

  it('Test 6 — happy path: flips is_removed=TRUE + appends entry + returns shape', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
      row('Slampeach', 'a@x'),
      row('Abulus', 'a@x'),
      row('Other', 'b@x'),
    ]));
    const before = Date.now();
    const out = commitEviction('a@x');
    const after = Date.now();

    // (a) Returned shape
    expect(out.affected).toBe(3);
    expect(typeof out.graceUntil).toBe('string');
    expect(out.graceUntil).toMatch(/^\d{4}-\d{2}-\d{2}T/);

    // (b) All three a@x rows are is_removed=TRUE
    const owner = state.sheets.get('_char_owner')!;
    expect(owner.values[1][8]).toBe(true);   // Findom
    expect(owner.values[2][8]).toBe(true);   // Slampeach
    expect(owner.values[3][8]).toBe(true);   // Abulus
    expect(owner.values[4][8]).toBe(false);  // Other (b@x) untouched

    // (c) _meta.eviction_log has one entry
    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'eviction_log');
    expect(logRow).toBeDefined();
    const list = JSON.parse(String(logRow![1])) as Array<Record<string, unknown>>;
    expect(list.length).toBe(1);
    const entry = list[0];
    expect(entry.email).toBe('a@x');
    expect(entry.reason).toBe('evicted');
    // Chars preserve insertion order from the row scan (not sorted).
    expect(entry.chars).toEqual(['Findom', 'Slampeach', 'Abulus']);
    expect(typeof entry.at).toBe('string');
    expect(String(entry.at)).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    // at and grace_until are exactly 30 days apart (allowing 1ms wiggle).
    const atMs = new Date(String(entry.at)).getTime();
    const graceMs = new Date(String(entry.grace_until)).getTime();
    const diff = graceMs - atMs;
    const thirtyDays = 30 * 24 * 60 * 60 * 1000;
    expect(Math.abs(diff - thirtyDays)).toBeLessThan(1000);
    // initiated_by is the mocked Session email.
    expect(entry.initiated_by).toBe('officer@example.com');
    // sanity: clock advanced during the call
    expect(after).toBeGreaterThanOrEqual(before);
  });

  it('Test 7 — idempotent for already-removed rows: does not toggle them', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x', false),
      row('Slampeach', 'a@x', true),  // already removed
      row('Abulus', 'a@x', false),
    ]));
    const out = commitEviction('a@x');
    // 3 chars are still listed (insertion order), but only the two
    // false→true flips were written. The audit-log entry still records all 3.
    expect(out.affected).toBe(3);
    const owner = state.sheets.get('_char_owner')!;
    expect(owner.values[1][8]).toBe(true);
    expect(owner.values[2][8]).toBe(true);  // was already true, stays true
    expect(owner.values[3][8]).toBe(true);
    // Verify the setValue log shows only 2 writes (not 3):
    const flipWrites = state.setValuesLog.filter((w) => w.sheet === '_char_owner' && w.range.endsWith('c9'));
    expect(flipWrites.length).toBe(2);
  });

  it('Test 8 — lock failure: throws and writes nothing', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
    ]));
    state.lockTryLockReturn = false;
    expect(() => commitEviction('a@x')).toThrowError(/commitEviction: lock_busy/);
    // is_removed unchanged; no _meta.eviction_log row.
    const owner = state.sheets.get('_char_owner')!;
    expect(owner.values[1][8]).toBe(false);
    const meta = state.sheets.get('_meta')!;
    expect(meta.values.find((r) => r[0] === 'eviction_log')).toBeUndefined();
  });

  it('Test 9 — missing _char_owner: throws with locked prefix', () => {
    // No _char_owner seeded.
    expect(() => commitEviction('a@x')).toThrowError(/_char_owner missing/);
  });

  it('Test 10 — invalid email: throws on empty or non-string', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
    ]));
    expect(() => commitEviction('')).toThrowError(/commitEviction: invalid email/);
    expect(() => commitEviction(null as unknown as string)).toThrowError(/commitEviction: invalid email/);
    expect(() => commitEviction(undefined as unknown as string)).toThrowError(/commitEviction: invalid email/);
  });

  it('Test 11 — appends to existing eviction_log (preserves old entries verbatim)', () => {
    const existing = [
      { at: '2026-04-01T00:00:00Z', email: 'old1@x', initiated_by: 'oldofficer@example.com',
        grace_until: '2026-05-01T00:00:00Z', chars: ['OldChar1'], reason: 'evicted' },
      { at: '2026-04-15T00:00:00Z', email: 'old2@x', initiated_by: 'oldofficer@example.com',
        grace_until: '2026-05-15T00:00:00Z', chars: ['OldChar2'], reason: 'evicted' },
    ];
    seedMetaWithAdmins(state, ['officer@example.com'], [
      ['eviction_log', JSON.stringify(existing)],
    ]);
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
    ]));

    commitEviction('a@x');

    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'eviction_log')!;
    const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
    expect(list.length).toBe(3);
    // Old entries preserved verbatim
    expect(list[0]).toEqual(existing[0]);
    expect(list[1]).toEqual(existing[1]);
    // New entry at the end
    expect(list[2].email).toBe('a@x');
    expect(list[2].chars).toEqual(['Findom']);
  });

  it('Test 12 — initiated_by fallback to "unknown" when Session email empty', () => {
    installSessionMock('');  // empty effective email
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      row('Findom', 'a@x'),
    ]));
    commitEviction('a@x');
    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'eviction_log')!;
    const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
    expect(list[0].initiated_by).toBe('unknown');
  });
});

afterEach(() => {
  // Cleanup Session global so other test files don't see a leaked stub.
  delete (globalThis as Record<string, unknown>).Session;
});
