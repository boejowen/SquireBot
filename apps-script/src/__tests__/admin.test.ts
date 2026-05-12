// Phase 7 plan 07-01 task 1 — vitest scenarios for the admin policy
// module (apps-script/src/lib/admin.ts). Coverage map mirrors
// .planning/phases/07-admin-allowlist-eviction-enforcement/07-PATTERNS.md
// §"Test scenario coverage map"; every test name traces 1:1 to T1–T20.
//
// Phase 7 verification hooks 1–4 are exercised at the policy layer here;
// hook 5 (schema_version unchanged) is enforced by acceptance grep against
// admin.ts itself in plan 07-01 §<acceptance_criteria>.
//
// Pattern provenance:
//   - Session mock + afterEach cleanup: cloned verbatim from
//     showEvictionSidebar.test.ts:42-47 + 281-284 (T-05-04 audit-log path
//     is the closest analog; admin.ts extends the same Session pattern
//     for AUTHORIZATION decisions per CONTEXT.md §D-06).
//   - lock-busy switch: state.lockTryLockReturn = false, cloned from
//     showEvictionSidebar.test.ts:212-223.
//   - audit-log JSON-decode assertion shape: cloned from
//     showEvictionSidebar.test.ts:168-188.
//   - getOwner() override: per-test override on the SpreadsheetApp mock;
//     test-helpers.ts:220-243 mock does not expose getOwner so we install
//     a lightweight override per test and rely on resetMocks() in
//     beforeEach to reinstall the base mock for the next test.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { resetMocks, seedMeta, type MockState } from './test-helpers';
import * as admin from '../lib/admin';
import * as logModule from '../lib/log';

function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

// Per-test override for SpreadsheetApp.getActiveSpreadsheet().getOwner().
// The base test-helpers mock does not expose getOwner(); admin.ts's
// bootstrapGuildAdmins reads it during the auto-bootstrap path. We
// install a thin override that preserves every other method by
// delegating to the base mock's getActiveSpreadsheet() proxy.
function installOwnerOverride(ownerEmail: string | null): void {
  const ssGlobal = (globalThis as Record<string, unknown>).SpreadsheetApp as {
    getActiveSpreadsheet: () => Record<string, unknown>;
    getUi: () => unknown;
    newConditionalFormatRule: () => unknown;
    ProtectionType: unknown;
  };
  const baseProxy = ssGlobal.getActiveSpreadsheet();
  const wrappedProxy = {
    ...baseProxy,
    getOwner: () => (ownerEmail === null ? null : { getEmail: () => ownerEmail }),
  };
  (globalThis as Record<string, unknown>).SpreadsheetApp = {
    ...ssGlobal,
    getActiveSpreadsheet: () => wrappedProxy,
  };
}

describe('admin.ts', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, []);  // ensure _meta sheet exists
    installSessionMock('officer@example.com');
  });

  afterEach(() => {
    delete (globalThis as Record<string, unknown>).Session;
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------
  describe('requireAdminOrThrow', () => {
    it('T1 requireAdminOrThrow_emptyEmail_throwsNotAuthorized', () => {
      seedMeta(state, [['guild_admins', JSON.stringify(['alice@x.com'])]]);
      expect(() => admin.requireAdminOrThrow('')).toThrowError(/not_authorized/);
      expect(() => admin.requireAdminOrThrow(null as unknown as string)).toThrowError(/not_authorized/);
      expect(() => admin.requireAdminOrThrow(undefined as unknown as string)).toThrowError(/not_authorized/);
    });

    it('T2 requireAdminOrThrow_notInList_throwsNotAuthorized', () => {
      seedMeta(state, [['guild_admins', JSON.stringify(['alice@x.com'])]]);
      expect(() => admin.requireAdminOrThrow('intruder@x.com')).toThrowError(/not_authorized/);
    });

    it('T3 requireAdminOrThrow_caseMismatched_succeeds', () => {
      seedMeta(state, [['guild_admins', JSON.stringify(['alice@x.com'])]]);
      // ALICE@X.COM (uppercase input) vs. alice@x.com (lowercase stored)
      // → must not throw because compare is normalized.
      expect(() => admin.requireAdminOrThrow('ALICE@X.COM')).not.toThrow();
    });
  });

  // -------------------------------------------------------------------
  describe('isAdmin', () => {
    it('T4 isAdmin_emptyList_returnsFalse', () => {
      // No guild_admins row at all.
      seedMeta(state, []);
      expect(admin.isAdmin('anyone@x.com')).toBe(false);
    });

    it('T5 isAdmin_caseInsensitiveMatch', () => {
      seedMeta(state, [['guild_admins', JSON.stringify(['alice@x.com'])]]);
      expect(admin.isAdmin('Alice@X.com')).toBe(true);
    });
  });

  // -------------------------------------------------------------------
  describe('getAdminList', () => {
    it('T6 getAdminList_malformedJson_failsClosedToEmpty', () => {
      const logSpy = vi.spyOn(logModule, 'log').mockImplementation(() => {});
      seedMeta(state, [['guild_admins', 'not valid json {{']]);
      const out = admin.getAdminList();
      expect(out).toEqual({ admins: [], floor: '' });
      // warn log was emitted with the malformedExistingList sentinel.
      expect(logSpy).toHaveBeenCalledWith(
        'warn',
        'getAdminList',
        expect.objectContaining({ malformedExistingList: true }),
      );
      logSpy.mockRestore();
    });
  });

  // -------------------------------------------------------------------
  describe('addAdmin', () => {
    it('T7 addAdmin_happyPath_appendsSortedLowercasedAndLogs', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['bob@x.com'])],
        ['workbook_owner_floor', 'bob@x.com'],
      ]);
      installSessionMock('bob@x.com');

      const result = admin.addAdmin('Alice@X.com', 'bob@x.com');
      expect(result).toEqual({ added: true });

      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      // Sorted + lowercased.
      expect(adminsRow[1]).toBe('["alice@x.com","bob@x.com"]');

      const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
      expect(logRow).toBeDefined();
      const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
      expect(list).toHaveLength(1);
      expect(list[0]).toMatchObject({
        action: 'add',
        email: 'alice@x.com',
        initiated_by: 'bob@x.com',
      });
      expect(String(list[0].at)).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    });

    it('T8 addAdmin_alreadyExists_idempotentNoWriteNoLog', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['alice@x.com', 'bob@x.com'])],
        ['workbook_owner_floor', 'alice@x.com'],
      ]);
      installSessionMock('bob@x.com');

      // Snapshot current writes count to filter out the seeded state.
      const writesBefore = state.setValuesLog.filter((w) => w.sheet === '_meta').length;
      const appendsBefore = state.appendedRowsLog.filter((a) => a.sheet === '_meta').length;

      const result = admin.addAdmin('alice@x.com', 'bob@x.com');
      expect(result).toEqual({ added: false, alreadyExists: true });

      // No NEW guild_admins write happened: appendedRows unchanged for
      // _meta, AND no setValue updated the existing guild_admins cell.
      const writesAfter = state.setValuesLog.filter((w) => w.sheet === '_meta').length;
      const appendsAfter = state.appendedRowsLog.filter((a) => a.sheet === '_meta').length;
      expect(writesAfter).toBe(writesBefore);
      expect(appendsAfter).toBe(appendsBefore);

      // No admin_log row appended either.
      const meta = state.sheets.get('_meta')!;
      const logRow = meta.values.find((r) => r[0] === 'admin_log');
      expect(logRow).toBeUndefined();
    });

    it('T9 addAdmin_rejectsEmptyOrMissingAt', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['bob@x.com'])],
        ['workbook_owner_floor', 'bob@x.com'],
      ]);
      installSessionMock('bob@x.com');

      expect(() => admin.addAdmin('', 'bob@x.com')).toThrowError(/invalid_email/);
      expect(() => admin.addAdmin('   ', 'bob@x.com')).toThrowError(/invalid_email/);
      expect(() => admin.addAdmin('notanemail', 'bob@x.com')).toThrowError(/invalid_email/);

      // No writes: guild_admins cell still has just bob.
      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['bob@x.com']));
      // No admin_log row appended.
      expect(meta.values.find((r) => r[0] === 'admin_log')).toBeUndefined();
    });

    it('T10 addAdmin_lockBusy_throwsAndDoesNotWrite', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['bob@x.com'])],
        ['workbook_owner_floor', 'bob@x.com'],
      ]);
      installSessionMock('bob@x.com');

      state.lockTryLockReturn = false;
      expect(() => admin.addAdmin('new@x.com', 'bob@x.com')).toThrowError(/addAdmin: lock_busy/);

      // No _meta write happened beyond the seed.
      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['bob@x.com']));
      expect(meta.values.find((r) => r[0] === 'admin_log')).toBeUndefined();
    });
  });

  // -------------------------------------------------------------------
  describe('removeAdmin', () => {
    it('T11 removeAdmin_nonFloorByNonFloorCaller_succeeds', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['alice@x.com', 'bob@x.com', 'jbowen@x.com'])],
        ['workbook_owner_floor', 'jbowen@x.com'],
      ]);
      installSessionMock('bob@x.com');

      const result = admin.removeAdmin('alice@x.com', 'bob@x.com');
      expect(result).toEqual({ removed: true });

      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['bob@x.com', 'jbowen@x.com']));

      const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
      const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
      expect(list).toHaveLength(1);
      expect(list[0]).toMatchObject({
        action: 'remove',
        email: 'alice@x.com',
        initiated_by: 'bob@x.com',
      });
    });

    it('T12 removeAdmin_floorByNonFloor_throwsOwnerFloorProtected', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['alice@x.com', 'bob@x.com', 'jbowen@x.com'])],
        ['workbook_owner_floor', 'jbowen@x.com'],
      ]);
      installSessionMock('bob@x.com');

      expect(() => admin.removeAdmin('jbowen@x.com', 'bob@x.com'))
        .toThrowError(/owner_floor_protected/);

      // No writes: guild_admins unchanged; no admin_log entry.
      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['alice@x.com', 'bob@x.com', 'jbowen@x.com']));
      expect(meta.values.find((r) => r[0] === 'admin_log')).toBeUndefined();
    });

    it('T13 removeAdmin_floorByFloor_selfRemovalSucceeds', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['alice@x.com', 'bob@x.com', 'jbowen@x.com'])],
        ['workbook_owner_floor', 'jbowen@x.com'],
      ]);
      installSessionMock('jbowen@x.com');

      const result = admin.removeAdmin('jbowen@x.com', 'jbowen@x.com');
      expect(result).toEqual({ removed: true });

      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['alice@x.com', 'bob@x.com']));

      // workbook_owner_floor is INTENTIONALLY NOT updated (orphan per D-04).
      const floorRow = meta.values.find((r) => r[0] === 'workbook_owner_floor')!;
      expect(floorRow[1]).toBe('jbowen@x.com');
    });

    it('T14 removeAdmin_notInList_idempotentNoWriteNoLog', () => {
      seedMeta(state, [
        ['guild_admins', JSON.stringify(['alice@x.com'])],
        ['workbook_owner_floor', 'alice@x.com'],
      ]);
      installSessionMock('alice@x.com');

      const result = admin.removeAdmin('ghost@x.com', 'alice@x.com');
      expect(result).toEqual({ removed: false, notFound: true });

      // No writes beyond seed.
      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['alice@x.com']));
      expect(meta.values.find((r) => r[0] === 'admin_log')).toBeUndefined();
    });
  });

  // -------------------------------------------------------------------
  describe('bootstrapGuildAdmins', () => {
    it('T15 bootstrapGuildAdmins_emptyMeta_writesSeedAndFloorAndLog', () => {
      // _meta exists but has no guild_admins row.
      seedMeta(state, []);
      installOwnerOverride('OWNER@X.COM');
      installSessionMock('owner@x.com');

      const result = admin.bootstrapGuildAdmins();
      expect(result).toEqual({ bootstrapped: true, seedEmail: 'owner@x.com' });

      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['owner@x.com']));
      const floorRow = meta.values.find((r) => r[0] === 'workbook_owner_floor')!;
      expect(floorRow[1]).toBe('owner@x.com');

      const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
      const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
      expect(list).toHaveLength(1);
      expect(list[0]).toMatchObject({
        action: 'bootstrap',
        email: 'owner@x.com',
        initiated_by: 'onOpen',
      });
    });

    it('T16 bootstrapGuildAdmins_alreadySeeded_noOp', () => {
      seedMeta(state, [['guild_admins', JSON.stringify(['existing@x.com'])]]);
      installOwnerOverride('owner@x.com');

      const writesBefore = state.setValuesLog.filter((w) => w.sheet === '_meta').length;
      const appendsBefore = state.appendedRowsLog.filter((a) => a.sheet === '_meta').length;

      const result = admin.bootstrapGuildAdmins();
      expect(result).toEqual({ bootstrapped: false, reason: 'already_initialized' });

      // No writes happened on the no-op path.
      const writesAfter = state.setValuesLog.filter((w) => w.sheet === '_meta').length;
      const appendsAfter = state.appendedRowsLog.filter((a) => a.sheet === '_meta').length;
      expect(writesAfter).toBe(writesBefore);
      expect(appendsAfter).toBe(appendsBefore);

      const meta = state.sheets.get('_meta')!;
      expect(meta.values.find((r) => r[0] === 'admin_log')).toBeUndefined();
    });

    it('T17 bootstrapGuildAdmins_ownerNull_writesFailedLogAndReturns', () => {
      const logSpy = vi.spyOn(logModule, 'log').mockImplementation(() => {});
      seedMeta(state, []);
      installOwnerOverride(null);  // getOwner() returns null

      const result = admin.bootstrapGuildAdmins();
      expect(result).toEqual({ bootstrapped: false, reason: 'owner_null' });

      // No guild_admins write; admin_log has 1 bootstrap_failed entry.
      const meta = state.sheets.get('_meta')!;
      expect(meta.values.find((r) => r[0] === 'guild_admins')).toBeUndefined();

      const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
      const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
      expect(list).toHaveLength(1);
      expect(list[0]).toMatchObject({
        action: 'bootstrap_failed',
        email: '',
        initiated_by: 'onOpen',
        reason: 'owner_null',
      });

      // warn log emitted.
      expect(logSpy).toHaveBeenCalledWith(
        'warn',
        'bootstrapGuildAdmins',
        expect.objectContaining({ reason: 'owner_null' }),
      );
      logSpy.mockRestore();
    });

    it('T18 bootstrapGuildAdmins_manualOpts_usesSeedEmail', () => {
      seedMeta(state, []);
      installOwnerOverride(null);  // getOwner() would fail, but opts override it

      const result = admin.bootstrapGuildAdmins({
        seedEmail: 'manual@x.com',
        initiatedBy: 'manual_fallback',
      });
      expect(result).toEqual({ bootstrapped: true, seedEmail: 'manual@x.com' });

      const meta = state.sheets.get('_meta')!;
      const adminsRow = meta.values.find((r) => r[0] === 'guild_admins')!;
      expect(adminsRow[1]).toBe(JSON.stringify(['manual@x.com']));
      const floorRow = meta.values.find((r) => r[0] === 'workbook_owner_floor')!;
      expect(floorRow[1]).toBe('manual@x.com');

      const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
      const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
      expect(list).toHaveLength(1);
      expect(list[0]).toMatchObject({
        action: 'bootstrap',
        email: 'manual@x.com',
        initiated_by: 'manual_fallback',
      });
    });

    it('T19 bootstrapGuildAdmins_lockBusy_silentNoOpDoesNotThrow', () => {
      const logSpy = vi.spyOn(logModule, 'log').mockImplementation(() => {});
      seedMeta(state, []);
      installOwnerOverride('owner@x.com');
      state.lockTryLockReturn = false;

      // Must NOT throw (D-01: onOpen mustn't throw).
      const result = admin.bootstrapGuildAdmins();
      expect(result).toEqual({ bootstrapped: false, reason: 'lock_busy' });

      // No writes.
      const meta = state.sheets.get('_meta')!;
      expect(meta.values.find((r) => r[0] === 'guild_admins')).toBeUndefined();
      expect(meta.values.find((r) => r[0] === 'workbook_owner_floor')).toBeUndefined();
      expect(meta.values.find((r) => r[0] === 'admin_log')).toBeUndefined();

      // warn log emitted with skipped='lock_busy'.
      expect(logSpy).toHaveBeenCalledWith(
        'warn',
        'bootstrapGuildAdmins',
        expect.objectContaining({ skipped: 'lock_busy' }),
      );
      logSpy.mockRestore();
    });
  });

  // -------------------------------------------------------------------
  describe('appendAdminLogEntry', () => {
    it('T20 appendAdminLogEntry_malformedExisting_startsFreshAndWarns', () => {
      const logSpy = vi.spyOn(logModule, 'log').mockImplementation(() => {});
      seedMeta(state, [['admin_log', '{ broken json']]);

      admin.appendAdminLogEntry({
        at: '2026-05-11T00:00:00.000Z',
        action: 'add',
        email: 'x@x.com',
        initiated_by: 'y@y.com',
      });

      const meta = state.sheets.get('_meta')!;
      const logRow = meta.values.find((r) => r[0] === 'admin_log')!;
      const list = JSON.parse(String(logRow[1])) as Array<Record<string, unknown>>;
      expect(list).toHaveLength(1);
      expect(list[0]).toMatchObject({
        action: 'add',
        email: 'x@x.com',
        initiated_by: 'y@y.com',
        at: '2026-05-11T00:00:00.000Z',
      });

      // warn log emitted with malformedExistingLog: true.
      expect(logSpy).toHaveBeenCalledWith(
        'warn',
        'appendAdminLogEntry',
        expect.objectContaining({ malformedExistingLog: true }),
      );
      logSpy.mockRestore();
    });
  });
});
