// Phase 7 plan 07-02 task 2 — vitest scenarios for the admin-management
// sidebar trigger. Coverage per plan <behavior>:
//   TS1 sidebar opens at 300px with locked title + body strings (admin caller)
//   TS2 non-admin caller fires getUi().alert with locked copy AND no sidebar opens
//   TS3 getAdminList returns { admins, floor, callerEmail } shape
//   TS4 addAdmin wrapper rejects non-admin caller with /not_authorized/
//       and writes nothing to _meta
//   TS5 removeAdmin wrapper rejects floor target by non-floor caller with
//       /owner_floor_protected/ — defense-in-depth across the wrapper +
//       lib/admin.removeAdmin (admin.test.ts T12 covers the same boundary
//       at the lib layer)
//   TS6 (optional, RECOMMENDED) removeAdmin self-removal of floor by floor
//       caller succeeds and floor row is preserved (orphan pointer per D-04)

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { resetMocks, makeSheet, seedMeta, type MockState } from './test-helpers';
import {
  showAdminMgmtSidebar,
  getAdminList,
  addAdmin,
  removeAdmin,
} from '../triggers/showAdminMgmtSidebar';

// Local Session mock helper — mirrors showEvictionSidebar.test.ts:42-47.
// Not exported from test-helpers because every sidebar test currently
// keeps its own copy (single-test-file ergonomics; admin.test.ts also
// has a local helper). afterEach below cleans up the global so other
// test files don't see a leaked stub.
function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

describe('showAdminMgmtSidebar', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    // Default _meta with schema_version = 3 + a pre-seeded admin list.
    // Tests override seedMeta as needed.
    seedMeta(state, [['schema_version', '3']]);
  });
  afterEach(() => {
    delete (globalThis as Record<string, unknown>).Session;
  });

  // --- TS1 ---------------------------------------------------------------

  it('TS1 — admin caller opens 300px-wide sidebar with locked title + body strings', () => {
    installSessionMock('admin@example.com');
    seedMeta(state, [
      ['schema_version', '3'],
      ['guild_admins', JSON.stringify(['admin@example.com'])],
      ['workbook_owner_floor', 'admin@example.com'],
    ]);

    showAdminMgmtSidebar();

    const captured = (state as MockState & {
      lastSidebar?: { _html: string; _title: string; _width: number };
    }).lastSidebar;
    expect(captured).toBeDefined();
    expect(captured!._width).toBe(300);
    expect(captured!._title).toBe('SquireBot — Manage admins');
    expect(captured!._html).toContain('Manage admins');           // h3
    expect(captured!._html).toContain('Add admin');               // form section + button
    expect(captured!._html).toContain('email@example.com');       // input placeholder
    expect(captured!._html).toContain('Manage who can evict guildies'); // .desc
    expect(captured!._html).toContain('aria-live="polite"');      // a11y status region
    // No alert fired on the happy path.
    expect(state.alertCalls.length).toBe(0);
  });

  // --- TS2 ---------------------------------------------------------------

  it('TS2 — non-admin caller fires getUi().alert and does NOT open sidebar (D-03)', () => {
    installSessionMock('intruder@example.com');
    seedMeta(state, [
      ['schema_version', '3'],
      ['guild_admins', JSON.stringify(['admin@example.com'])],
      ['workbook_owner_floor', 'admin@example.com'],
    ]);

    showAdminMgmtSidebar();

    // Exactly one alert with the locked copy from 07-UI-SPEC §Copywriting.
    expect(state.alertCalls.length).toBe(1);
    expect(state.alertCalls[0].title).toBe('Not authorized');
    expect(state.alertCalls[0].body).toBe(
      'Only guild officers can manage admins. Contact a workbook admin if you think this is wrong.',
    );
    expect(state.alertCalls[0].buttonSet).toBe('OK');

    // No sidebar captured — the opener returned BEFORE building HTML.
    const captured = (state as MockState & { lastSidebar?: unknown }).lastSidebar;
    expect(captured).toBeUndefined();
  });

  // --- TS3 ---------------------------------------------------------------

  it('TS3 — getAdminList returns { admins, floor, callerEmail } sorted+lowercased', () => {
    installSessionMock('bob@x.com');
    seedMeta(state, [
      ['schema_version', '3'],
      // Pre-shuffled order; lib/admin sorts on read, so the wrapper
      // surfaces the sorted output. All values are pre-lowercased here
      // (the lib re-normalizes on read regardless).
      ['guild_admins', JSON.stringify(['bob@x.com', 'alice@x.com', 'jbowen@x.com'])],
      ['workbook_owner_floor', 'jbowen@x.com'],
    ]);

    const result = getAdminList();

    expect(result).toEqual({
      admins: ['alice@x.com', 'bob@x.com', 'jbowen@x.com'],
      floor: 'jbowen@x.com',
      callerEmail: 'bob@x.com',
    });
  });

  // --- TS4 ---------------------------------------------------------------

  it('TS4 — addAdmin wrapper rejects non-admin caller with /not_authorized/ + no writes', () => {
    installSessionMock('intruder@x.com');
    seedMeta(state, [
      ['schema_version', '3'],
      ['guild_admins', JSON.stringify(['admin@x.com'])],
      ['workbook_owner_floor', 'admin@x.com'],
    ]);

    expect(() => addAdmin('newadmin@x.com')).toThrowError(/not_authorized/);

    // _meta.guild_admins unchanged.
    const meta = state.sheets.get('_meta')!;
    const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
    expect(adminsRow[1]).toBe(JSON.stringify(['admin@x.com']));

    // No admin_log entry written.
    const logRow = meta.values.find((r) => r[0] === 'admin_log');
    expect(logRow === undefined || logRow[1] === '' || logRow[1] === '[]').toBe(true);

    // No setValues writes to _meta from the wrapper.
    const writes = state.setValuesLog.filter((w) => w.sheet === '_meta');
    expect(writes.length).toBe(0);
  });

  // --- TS5 ---------------------------------------------------------------

  it('TS5 — removeAdmin floor target by non-floor caller throws /owner_floor_protected/ (D-04)', () => {
    installSessionMock('bob@x.com');
    seedMeta(state, [
      ['schema_version', '3'],
      ['guild_admins', JSON.stringify(['bob@x.com', 'jbowen@x.com'])],
      ['workbook_owner_floor', 'jbowen@x.com'],
    ]);

    expect(() => removeAdmin('jbowen@x.com')).toThrowError(/owner_floor_protected/);

    // _meta.guild_admins still has both — no write happened.
    const meta = state.sheets.get('_meta')!;
    const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
    expect(JSON.parse(String(adminsRow[1]))).toEqual(['bob@x.com', 'jbowen@x.com']);

    // No admin_log entry written for the rejected attempt.
    const logRow = meta.values.find((r) => r[0] === 'admin_log');
    expect(logRow === undefined || logRow[1] === '' || logRow[1] === '[]').toBe(true);
  });

  // --- TS6 (optional but RECOMMENDED — D-04 self-removal contract) -------

  it('TS6 — removeAdmin floor self-removal succeeds; floor row preserved (orphan per D-04)', () => {
    installSessionMock('jbowen@x.com');
    seedMeta(state, [
      ['schema_version', '3'],
      ['guild_admins', JSON.stringify(['bob@x.com', 'jbowen@x.com'])],
      ['workbook_owner_floor', 'jbowen@x.com'],
    ]);

    const result = removeAdmin('jbowen@x.com');

    expect(result).toEqual({ removed: true });

    const meta = state.sheets.get('_meta')!;
    const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
    // jbowen removed; bob remains.
    expect(JSON.parse(String(adminsRow[1]))).toEqual(['bob@x.com']);

    // workbook_owner_floor row INTENTIONALLY preserved (D-04 orphan pointer).
    const floorRow = meta.values.find((r) => r[0] === 'workbook_owner_floor')!;
    expect(floorRow[1]).toBe('jbowen@x.com');

    // admin_log has the remove entry.
    const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
    const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
    expect(list.length).toBe(1);
    expect(list[0].action).toBe('remove');
    expect(list[0].email).toBe('jbowen@x.com');
    expect(list[0].initiated_by).toBe('jbowen@x.com');
  });

  // --- Bonus coverage — empty Session fail-closed ------------------------

  it('TS7 — empty Session.getEffectiveUser email fail-closes opener with alert (D-06 auth path)', () => {
    installSessionMock('');
    seedMeta(state, [
      ['schema_version', '3'],
      ['guild_admins', JSON.stringify(['admin@example.com'])],
      ['workbook_owner_floor', 'admin@example.com'],
    ]);

    showAdminMgmtSidebar();

    expect(state.alertCalls.length).toBe(1);
    expect(state.alertCalls[0].title).toBe('Not authorized');
    const captured = (state as MockState & { lastSidebar?: unknown }).lastSidebar;
    expect(captured).toBeUndefined();
  });
});
