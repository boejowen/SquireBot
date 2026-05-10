import { describe, it, expect, beforeEach } from 'vitest';
import {
  getCharsForForm, saveCharInfo, type CharInfoRow,
} from '../triggers/showCharInfoSidebar';
import { resetMocks, makeSheet, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
];

describe('getCharsForForm', () => {
  let state: MockState;
  beforeEach(() => { state = resetMocks(); });

  it('returns empty when _char_owner missing', () => {
    expect(getCharsForForm()).toEqual([]);
  });

  it('returns empty when _char_owner has only header', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS));
    expect(getCharsForForm()).toEqual([]);
  });

  it('returns rows; skips literal "char_name" header row defensively', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      ['Slampeach', 'boejowen@gmail.com', '', '', 'SHD', 60, 'FALSE', 'FALSE', 'FALSE',
        '2026-04-01T00:00:00Z', '2026-05-09T00:00:00Z', 'blue', '0.4.0', 'IKS'],
      ['Findom', 'boejowen@gmail.com', '', '', 'BRD', 35, 'FALSE', 'FALSE', 'FALSE',
        '2026-04-01T00:00:00Z', '2026-05-09T00:00:00Z', 'blue', '0.4.0', ''],
    ]));
    const out = getCharsForForm();
    expect(out.length).toBe(2);
    expect(out[0]).toEqual({ char_name: 'Slampeach', class: 'SHD', level: 60, race: 'IKS' });
    expect(out[1]).toEqual({ char_name: 'Findom', class: 'BRD', level: 35, race: '' });
  });

  it('skips empty char_name rows', () => {
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      ['Slampeach', 'x@example.com', '', '', 'SHD', 60, '', '', '', '', '', 'blue', '0.4.0', 'IKS'],
      ['', '', '', '', '', '', '', '', '', '', '', '', '', ''],
      ['Findom', 'x@example.com', '', '', 'BRD', 35, '', '', '', '', '', 'blue', '0.4.0', ''],
    ]));
    const out = getCharsForForm();
    expect(out.length).toBe(2);
    expect(out.map(c => c.char_name)).toEqual(['Slampeach', 'Findom']);
  });
});

describe('saveCharInfo — validation', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      ['Slampeach', 'x@example.com', '', '', '', '', '', '', '', '', '', 'blue', '0.4.0', ''],
    ]));
  });

  it('rejects invalid class', () => {
    const r = saveCharInfo([{ char_name: 'Slampeach', class: 'NOTAREALCLASS', level: 60, race: '' }]);
    expect(r.saved).toBe(0);
    expect(r.errors).toHaveLength(1);
    expect(r.errors[0]).toContain('invalid class');
  });

  it('rejects out-of-range level', () => {
    const r = saveCharInfo([{ char_name: 'Slampeach', class: '', level: 99, race: '' }]);
    expect(r.saved).toBe(0);
    expect(r.errors[0]).toContain('level out of range');
  });

  it('rejects level=0', () => {
    const r = saveCharInfo([{ char_name: 'Slampeach', class: '', level: 0, race: '' }]);
    expect(r.saved).toBe(0);
    expect(r.errors[0]).toContain('level out of range');
  });

  it('accepts blank level (empty string)', () => {
    const r = saveCharInfo([{ char_name: 'Slampeach', class: 'SHD', level: '', race: 'IKS' }]);
    expect(r.saved).toBe(1);
    expect(r.errors).toEqual([]);
  });

  it('rejects invalid race (case-sensitive — IKS valid, iksar not)', () => {
    const r = saveCharInfo([{ char_name: 'Slampeach', class: '', level: '', race: 'iksar' }]);
    expect(r.saved).toBe(0);
    expect(r.errors[0]).toContain('invalid race');
  });

  it('rejects empty char_name', () => {
    const r = saveCharInfo([{ char_name: '', class: 'SHD', level: 60, race: '' }]);
    expect(r.saved).toBe(0);
    expect(r.errors[0]).toContain('Empty char_name');
  });

  it('any error in batch fails the whole batch (saved=0)', () => {
    const r = saveCharInfo([
      { char_name: 'Slampeach', class: 'SHD', level: 60, race: 'IKS' }, // good
      { char_name: 'Findom', class: 'BAD', level: 35, race: '' },        // bad
    ]);
    expect(r.saved).toBe(0);
    expect(r.errors).toHaveLength(1);
  });
});

describe('saveCharInfo — happy path', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, [
      ['Slampeach', 'x@example.com', '', '', '', '', '', '', '', '', '', 'blue', '0.4.0', ''],
      ['Findom', 'x@example.com', '', '', '', '', '', '', '', '', '', 'blue', '0.4.0', ''],
    ]));
  });

  it('updates existing row in place (class, level, race)', () => {
    const r = saveCharInfo([
      { char_name: 'Slampeach', class: 'SHD', level: 60, race: 'IKS' },
    ]);
    expect(r.saved).toBe(1);
    const sheet = state.sheets.get('_char_owner')!;
    // Slampeach is at row index 1 (0-indexed) in values, which is row 2 (1-indexed)
    expect(sheet.values[1][4]).toBe('SHD');     // col E (class)
    expect(sheet.values[1][5]).toBe(60);         // col F (level)
    expect(sheet.values[1][13]).toBe('IKS');     // col N (race)
  });

  it('does NOT insert new rows for chars not in _char_owner', () => {
    const r = saveCharInfo([
      { char_name: 'NeverHeardOfThisChar', class: 'WAR', level: 1, race: 'HUM' },
    ]);
    expect(r.saved).toBe(0);
    expect(r.errors).toEqual([]);
    const sheet = state.sheets.get('_char_owner')!;
    expect(sheet.values.length).toBe(3); // header + 2 originals; no new row
  });

  it('partial update: only class set, level + race left alone', () => {
    const sheet = state.sheets.get('_char_owner')!;
    sheet.values[1][5] = 50;        // existing level
    sheet.values[1][13] = 'BAR';    // existing race
    const r = saveCharInfo([
      { char_name: 'Slampeach', class: 'SHD', level: '', race: '' },
    ]);
    expect(r.saved).toBe(1);
    expect(sheet.values[1][4]).toBe('SHD');     // class updated
    expect(sheet.values[1][5]).toBe(50);         // level UNTOUCHED
    expect(sheet.values[1][13]).toBe('BAR');    // race UNTOUCHED
  });

  it('throws when _char_owner missing entirely', () => {
    state.sheets.delete('_char_owner');
    expect(() => saveCharInfo([{ char_name: 'Slampeach', class: 'SHD', level: 60, race: 'IKS' }]))
      .toThrow(/_char_owner missing/);
  });

  it('lock contention throws', () => {
    state.lockTryLockReturn = false;
    expect(() => saveCharInfo([{ char_name: 'Slampeach', class: 'SHD', level: 60, race: 'IKS' }]))
      .toThrow(/lock_busy/);
  });
});
