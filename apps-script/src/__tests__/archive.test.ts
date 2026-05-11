// Phase 5 plan 05-02 task 1: lib/archive.ts.
//
// Clones the lock-mock pattern from migrations.test.ts. resetMocks() +
// makeSheet() come from test-helpers; LockService.tryLock honour
// state.lockTryLockReturn. The hideSheet/isSheetHidden hooks (added in
// 05-01) are exercised here too.

import { describe, it, expect, beforeEach } from 'vitest';
import { moveCharToArchive } from '../lib/archive';
import { resetMocks, seedMeta, makeSheet, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version',
];

function seedCharOwner(state: MockState, charName: string, options: {
  lastSeen?: string;
  isRemoved?: boolean | string;
} = {}): void {
  const lastSeen = options.lastSeen ?? new Date(Date.now() - 91 * 24 * 60 * 60 * 1000).toISOString();
  const isRemoved = options.isRemoved ?? false;
  state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
    [
      charName, `${charName.toLowerCase()}@example.com`, charName, '',
      'Shaman', 60, false, false, isRemoved,
      '2025-11-01T00:00:00Z', lastSeen, 'blue', '0.4.0',
    ],
  ]));
}

function seedInv(state: MockState, charName: string, rowCount: number): void {
  const headers = ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'];
  const data: unknown[][] = [];
  for (let i = 0; i < rowCount; i++) {
    data.push(['INV', `Item ${i}`, 1000 + i, 1, 0, '2025-12-01T00:00:00Z']);
  }
  state.sheets.set(`inv:${charName}`, makeSheet(`inv:${charName}`, headers, data));
}

function seedSpell(state: MockState, charName: string, rowCount: number): void {
  const headers = ['Level', 'Name', '_uploaded_at'];
  const data: unknown[][] = [];
  for (let i = 0; i < rowCount; i++) data.push([1 + i, `Spell ${i}`, '2025-12-01T00:00:00Z']);
  state.sheets.set(`spell:${charName}`, makeSheet(`spell:${charName}`, headers, data));
}

describe('moveCharToArchive', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3']]);
  });

  it('happy path: archives stale char with inv (3 rows) + spell (5 rows)', () => {
    seedCharOwner(state, 'Findom');
    seedInv(state, 'Findom', 3);
    seedSpell(state, 'Findom', 5);

    moveCharToArchive('Findom', 'stale_90d');

    // (a) _archive tab exists with the 7-column header
    const archive = state.sheets.get('_archive')!;
    expect(archive).toBeDefined();
    const headers = archive.values[0] as string[];
    expect(headers).toEqual([
      'archived_at', 'char_name', 'tab_type', 'row_count',
      'uploaded_at', 'reason', 'snapshot_json',
    ]);

    // (b) 2 archive rows (inv + spell) appended after header
    expect(archive.values.length).toBe(3);
    const invRow = archive.values.find((r) => r[2] === 'inv')!;
    const spellRow = archive.values.find((r) => r[2] === 'spell')!;
    expect(invRow).toBeDefined();
    expect(spellRow).toBeDefined();
    expect(invRow[1]).toBe('Findom');
    expect(spellRow[1]).toBe('Findom');
    expect(invRow[3]).toBe(3);  // row_count (data rows, header excluded)
    expect(spellRow[3]).toBe(5);
    expect(invRow[5]).toBe('stale_90d');
    expect(spellRow[5]).toBe('stale_90d');
    const invSnapshot = JSON.parse(String(invRow[6])) as unknown[][];
    const spellSnapshot = JSON.parse(String(spellRow[6])) as unknown[][];
    expect(invSnapshot.length).toBe(4);  // header + 3 data rows
    expect(spellSnapshot.length).toBe(6);  // header + 5 data rows

    // (c) _char_owner.is_removed=TRUE flipped (column 9)
    const owner = state.sheets.get('_char_owner')!;
    expect(owner.values[1][8]).toBe(true);

    // (d) inv:Findom + spell:Findom hidden
    const inv = state.sheets.get('inv:Findom') as { _hidden?: boolean };
    const spell = state.sheets.get('spell:Findom') as { _hidden?: boolean };
    expect(inv._hidden).toBe(true);
    expect(spell._hidden).toBe(true);

    // (e) _meta.archive_log has 1 entry with the expected envelope
    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'archive_log');
    expect(logRow).toBeDefined();
    const list = JSON.parse(String(logRow![1])) as Array<Record<string, unknown>>;
    expect(list.length).toBe(1);
    expect(list[0].char).toBe('Findom');
    expect(list[0].reason).toBe('stale_90d');
    expect(list[0].inv_row_count).toBe(3);
    expect(list[0].spell_row_count).toBe(5);
    expect(String(list[0].at)).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  it('idempotent: re-calling on an already-archived char is a no-op', () => {
    seedCharOwner(state, 'Findom', { isRemoved: true });
    seedInv(state, 'Findom', 3);
    seedSpell(state, 'Findom', 5);
    // Simulate the source tabs already being hidden.
    (state.sheets.get('inv:Findom') as { _hidden?: boolean })._hidden = true;
    (state.sheets.get('spell:Findom') as { _hidden?: boolean })._hidden = true;

    moveCharToArchive('Findom', 'stale_90d');

    // No _archive tab created (or if created, no data rows)
    const archive = state.sheets.get('_archive');
    if (archive) {
      // header-only or absent
      expect(archive.values.length).toBeLessThanOrEqual(1);
    }
    // No archive_log row added
    const meta = state.sheets.get('_meta')!;
    const logRow = meta.values.find((r) => r[0] === 'archive_log');
    expect(logRow).toBeUndefined();
  });

  it('missing inv tab: still archives spell, inv archive row has row_count=0', () => {
    seedCharOwner(state, 'Findom');
    // No inv:Findom seeded
    seedSpell(state, 'Findom', 5);

    moveCharToArchive('Findom', 'stale_90d');

    const archive = state.sheets.get('_archive')!;
    const invRow = archive.values.find((r) => r[2] === 'inv')!;
    expect(invRow).toBeDefined();
    expect(invRow[3]).toBe(0);
    expect(String(invRow[6])).toBe('[]');

    const spellRow = archive.values.find((r) => r[2] === 'spell')!;
    expect(spellRow[3]).toBe(5);
  });

  it('lock contention: throws and writes nothing when tryLock returns false', () => {
    state.lockTryLockReturn = false;
    seedCharOwner(state, 'Findom');
    seedInv(state, 'Findom', 3);
    seedSpell(state, 'Findom', 5);

    expect(() => moveCharToArchive('Findom', 'stale_90d')).toThrowError(
      /moveCharToArchive.*lock/,
    );

    // No _archive tab; no is_removed flip; tabs not hidden.
    expect(state.sheets.get('_archive')).toBeUndefined();
    const owner = state.sheets.get('_char_owner')!;
    expect(owner.values[1][8]).toBe(false);
    expect((state.sheets.get('inv:Findom') as { _hidden?: boolean })._hidden).toBeFalsy();
    expect((state.sheets.get('spell:Findom') as { _hidden?: boolean })._hidden).toBeFalsy();
  });

  it('no _char_owner row for the char: logs warn and skips (no orphan _archive rows)', () => {
    // _char_owner seeded with a DIFFERENT char.
    seedCharOwner(state, 'Slampeach');
    seedInv(state, 'Findom', 3);
    seedSpell(state, 'Findom', 5);

    moveCharToArchive('Findom', 'evicted');

    // No archive entries written
    const archive = state.sheets.get('_archive');
    if (archive) {
      expect(archive.values.length).toBeLessThanOrEqual(1);
    }
    // Slampeach's row untouched
    const owner = state.sheets.get('_char_owner')!;
    expect(owner.values[1][8]).toBe(false);
  });

  it('lazy _archive creation is idempotent: re-using existing tab does not re-insert', () => {
    // Pre-seed _archive with a header so we can verify it stays.
    state.sheets.set('_archive', makeSheet('_archive', [
      'archived_at', 'char_name', 'tab_type', 'row_count',
      'uploaded_at', 'reason', 'snapshot_json',
    ]));
    seedCharOwner(state, 'Findom');
    seedInv(state, 'Findom', 2);
    seedSpell(state, 'Findom', 2);

    moveCharToArchive('Findom', 'evicted');

    const archive = state.sheets.get('_archive')!;
    // header preserved as row 0; appended 2 rows
    expect((archive.values[0] as unknown[])[0]).toBe('archived_at');
    expect(archive.values.length).toBe(3);
  });
});
