// admin.ts — Phase 7 plan 07-01.
//
// Central admin-policy module for the SquireBot workbook. Every primitive
// for "who can do destructive things" lives here. Single source of truth
// so the eviction sidebar (Plan 03) and the admin-mgmt sidebar (Plan 02)
// cannot drift into re-implementing the admin check with a subtle bug.
//
// Storage shape (all three _meta rows are extend-only additions; no
// schema_version bump per STATE.md D-02):
//   _meta.guild_admins         — JSON.stringify(string[]) lowercased+sorted
//   _meta.workbook_owner_floor — single email string, lowercased
//   _meta.admin_log            — JSON.stringify(AdminLogEntry[]), append-only
//
// Dual-policy caller identity (D-06):
//   - Authorization (requireAdminOrThrow): empty → fail-closed throw
//   - Audit-log initiated_by: empty → 'unknown' soft fallback
//
// Lock envelope (Pitfall P6 mitigation, cloned from
// showEvictionSidebar.ts:122-186 + migrations.ts:55-74): every multi-step
// _meta write is wrapped in LockService.getDocumentLock().tryLock(30000).
// addAdmin/removeAdmin throw on contention; bootstrapGuildAdmins is the
// documented exception that returns a {reason:'lock_busy'} enum instead
// of throwing (D-01: onOpen mustn't throw).

import { log } from './log';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from './sheet-helpers';

// --- Types --------------------------------------------------------------

export interface AdminLogEntry {
  at: string;          // ISO8601
  action: 'add' | 'remove' | 'bootstrap' | 'bootstrap_failed';
  email: string;
  initiated_by: string;
  reason?: string;     // optional — only set for 'bootstrap_failed' entries
}

export type BootstrapReason =
  | 'already_initialized'
  | 'owner_null'
  | 'lock_busy'
  | 'utf16_failed';

// --- Normalization ------------------------------------------------------

/** Lowercase + trim. SINGLE NORMALIZATION POINT used everywhere policy
 * decisions are made (read, write, compare). Per CONTEXT.md §specifics
 * "apply at THREE points: read / write / compare". */
export function normalizeEmail(s: string | null | undefined): string {
  return String(s ?? '').toLowerCase().trim();
}

// --- Read primitives ----------------------------------------------------

/** Read both _meta rows. Tolerates malformed JSON in guild_admins
 * (returns empty list — fail-closed; nobody is admin if the cell is
 * corrupt). admins[] is sorted+lowercased on the way out. floor is read
 * as a plain lowercased string from a separate _meta row (no JSON). */
export function getAdminList(): { admins: string[]; floor: string } {
  const meta = readMetaRows('_meta');
  let admins: string[] = [];
  const adminsRow = meta.find((r) => r.key === 'guild_admins');
  if (adminsRow && adminsRow.value) {
    try {
      const parsed = JSON.parse(adminsRow.value);
      if (Array.isArray(parsed)) {
        admins = parsed
          .map((s) => normalizeEmail(String(s)))
          .filter(Boolean)
          .sort();
      }
    } catch (_e) {
      log('warn', 'getAdminList', { malformedExistingList: true });
    }
  }
  const floorRow = meta.find((r) => r.key === 'workbook_owner_floor');
  const floor = floorRow ? normalizeEmail(String(floorRow.value)) : '';
  return { admins, floor };
}

/** Convenience: normalizes input, returns whether it's in the list.
 * Empty string → false (fail-closed). */
export function isAdmin(email: string | null | undefined): boolean {
  const normalized = normalizeEmail(email);
  if (!normalized) return false;
  const { admins } = getAdminList();
  return admins.indexOf(normalized) !== -1;
}

/** Throws Error('not_authorized') if !isAdmin(email). Used by every
 * protected callback as the FIRST statement. Empty/null email also
 * throws (fail-closed per D-06). */
export function requireAdminOrThrow(email: string | null | undefined): void {
  const normalized = normalizeEmail(email);
  if (!normalized || !isAdmin(normalized)) {
    log('warn', 'requireAdminOrThrow', { notAuthorized: true, callerEmail: normalized });
    throw new Error('not_authorized');
  }
}

// --- Internal helpers ---------------------------------------------------

/** Audit-log soft fallback (D-06): empty Session → 'unknown'. NEVER used
 * for authorization decisions — those use the fail-closed path in
 * requireAdminOrThrow above. */
function resolveInitiatedBy(): string {
  try {
    const effective = Session.getEffectiveUser().getEmail();
    if (effective) return normalizeEmail(effective);
  } catch (_e) {
    // sandbox quirk — fall through to 'unknown'
  }
  return 'unknown';
}

/** Module-private helper. Reads _meta.admin_log, defensively parses JSON
 * (malformed → fresh array + warn log), pushes the entry, writes back.
 * NOT lock-wrapped on its own — caller is responsible for holding the
 * lock when called inside add/remove/bootstrap. Intentionally NOT
 * exported (WR-02): every external caller would have to remember to
 * acquire the document lock first, and the type system can't enforce
 * that contract. The three lock-wrapped mutators in this file are the
 * only legitimate callers. */
function appendAdminLogEntry(entry: AdminLogEntry): void {
  const meta = readMetaRows('_meta');
  const row = meta.find((r) => r.key === 'admin_log');
  let list: AdminLogEntry[] = [];
  if (row && row.value) {
    try {
      const parsed = JSON.parse(row.value);
      if (Array.isArray(parsed)) list = parsed as AdminLogEntry[];
    } catch (_e) {
      log('warn', 'appendAdminLogEntry', { malformedExistingLog: true });
    }
  }
  list.push(entry);
  writeMetaRow('_meta', 'admin_log', JSON.stringify(list));
}

// --- Mutating operations (lock-wrapped) --------------------------------

/** Lock-wrapped. Caller MUST be admin (requireAdminOrThrow). Validates
 * email (non-empty after normalize, contains '@'). Idempotent: existing
 * email returns {added: false, alreadyExists: true} with NO writes. */
export function addAdmin(
  email: string,
  callerEmail: string,
): { added: boolean; alreadyExists?: boolean } {
  requireAdminOrThrow(callerEmail);
  const target = normalizeEmail(email);
  // Stricter validation than just '@' — reject characters that would let
  // a crafted email break out of HTML attribute / event-handler contexts
  // in the admin-mgmt sidebar (WR-01 belt-and-suspenders alongside the
  // sidebar's escapeAttr helper). Allows the conservative subset of
  // RFC 5321 chars actually used in practice: alphanumerics, dot, dash,
  // underscore, plus, percent, at-sign.
  if (!target || !/^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$/i.test(target)) {
    throw new Error('invalid_email');
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    throw new Error('addAdmin: lock_busy');
  }
  try {
    const { admins } = getAdminList();
    if (admins.indexOf(target) !== -1) {
      log('info', 'addAdmin', { email: target, callerEmail, alreadyExists: true });
      return { added: false, alreadyExists: true };
    }
    const next = admins.concat([target]).sort();
    writeMetaRow('_meta', 'guild_admins', JSON.stringify(next));
    appendAdminLogEntry({
      at: new Date().toISOString(),
      action: 'add',
      email: target,
      initiated_by: normalizeEmail(callerEmail) || resolveInitiatedBy(),
    });
    log('info', 'addAdmin', { email: target, callerEmail, added: true });
    return { added: true };
  } finally {
    lock.releaseLock();
  }
}

/** Lock-wrapped. Caller MUST be admin (requireAdminOrThrow). Enforces
 * owner-floor: throws Error('owner_floor_protected') if target===floor
 * && caller!==floor. Self-removal of floor allowed (floor row is NOT
 * updated — documented orphan per D-04). Idempotent: not-in-list returns
 * {removed: false, notFound: true} with NO writes. */
export function removeAdmin(
  email: string,
  callerEmail: string,
): { removed: boolean; notFound?: boolean } {
  requireAdminOrThrow(callerEmail);
  const target = normalizeEmail(email);
  if (!target) {
    throw new Error('invalid_email');
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    throw new Error('removeAdmin: lock_busy');
  }
  try {
    const { admins, floor } = getAdminList();
    const normalizedCaller = normalizeEmail(callerEmail);

    // Owner-floor protection (D-04). Check happens BEFORE any write.
    if (target === floor && normalizedCaller !== floor) {
      log('warn', 'removeAdmin', {
        email: target,
        callerEmail: normalizedCaller,
        blockedBy: 'owner_floor_protected',
      });
      throw new Error('owner_floor_protected');
    }

    const idx = admins.indexOf(target);
    if (idx === -1) {
      log('info', 'removeAdmin', {
        email: target, callerEmail: normalizedCaller, notFound: true,
      });
      return { removed: false, notFound: true };
    }

    const next = admins.slice(0, idx).concat(admins.slice(idx + 1));
    writeMetaRow('_meta', 'guild_admins', JSON.stringify(next));
    // NOTE: workbook_owner_floor row is INTENTIONALLY not updated on
    // self-removal of floor (D-04). The floor row is the "who is
    // protected from non-self removal" pointer, not the "who is
    // currently an admin" pointer. Orphan-pointer state is documented
    // as intentional.
    appendAdminLogEntry({
      at: new Date().toISOString(),
      action: 'remove',
      email: target,
      initiated_by: normalizedCaller || resolveInitiatedBy(),
    });
    log('info', 'removeAdmin', {
      email: target, callerEmail: normalizedCaller, removed: true,
    });
    return { removed: true };
  } finally {
    lock.releaseLock();
  }
}

// --- Bootstrap ----------------------------------------------------------

/** Lazy onOpen bootstrap (D-01). Idempotent. Lock-wrapped but SWALLOWS
 * lock_busy (returns reason, no throw — onOpen must not throw). Uses
 * opts.seedEmail if provided (manual-fallback path), else
 * SpreadsheetApp.getActiveSpreadsheet().getOwner().getEmail(). Writes
 * guild_admins + workbook_owner_floor + admin_log bootstrap entry.
 *
 * On getOwner() returning null: writes a 'bootstrap_failed' entry to
 * admin_log + warn log, returns {bootstrapped: false, reason: 'owner_null'}.
 * The manual-fallback menu item (bootstrapGuildAdminsManual) is the
 * recovery path for that case. */
export function bootstrapGuildAdmins(
  opts?: { seedEmail?: string; initiatedBy?: string },
): { bootstrapped: boolean; seedEmail?: string; reason?: BootstrapReason } {
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    // D-01 exception: onOpen must NOT throw. Silent retry on next open.
    log('warn', 'bootstrapGuildAdmins', { skipped: 'lock_busy' });
    return { bootstrapped: false, reason: 'lock_busy' };
  }
  try {
    const { admins } = getAdminList();
    if (admins.length > 0) {
      // Idempotent: already seeded. No writes, no log entry.
      return { bootstrapped: false, reason: 'already_initialized' };
    }

    let seed = normalizeEmail(opts?.seedEmail ?? '');
    const initiatedBy = opts?.initiatedBy ?? 'onOpen';

    if (!seed) {
      try {
        const owner = getActiveSpreadsheet().getOwner();
        const ownerEmail = owner ? owner.getEmail() : '';
        seed = normalizeEmail(ownerEmail);
      } catch (_e) {
        // getOwner() threw (rare consumer-account quirk under drive.file).
        // Fall through to the owner_null branch below.
      }
    }

    if (!seed) {
      log('warn', 'bootstrapGuildAdmins', { reason: 'owner_null' });
      appendAdminLogEntry({
        at: new Date().toISOString(),
        action: 'bootstrap_failed',
        email: '',
        initiated_by: initiatedBy,
        reason: 'owner_null',
      });
      return { bootstrapped: false, reason: 'owner_null' };
    }

    writeMetaRow('_meta', 'guild_admins', JSON.stringify([seed]));
    writeMetaRow('_meta', 'workbook_owner_floor', seed);
    appendAdminLogEntry({
      at: new Date().toISOString(),
      action: 'bootstrap',
      email: seed,
      initiated_by: initiatedBy,
    });
    log('info', 'bootstrapGuildAdmins', {
      seedEmail: seed, initiatedBy, bootstrapped: true,
    });
    return { bootstrapped: true, seedEmail: seed };
  } finally {
    lock.releaseLock();
  }
}

/** Manual-fallback wrapper for the "Initialize Admin Allowlist (manual)"
 * menu item (D-01). Reads Session.getEffectiveUser().getEmail() as seed;
 * shows getUi().alert OK_CANCEL confirmation BEFORE writing; on success
 * toasts via SpreadsheetApp.getActiveSpreadsheet().toast. Calls
 * bootstrapGuildAdmins({seedEmail, initiatedBy: 'manual_fallback'}) under
 * the hood. Designed for the consumer-account quirk where getOwner()
 * returns null under drive.file. */
export function bootstrapGuildAdminsManual(): void {
  const ui = SpreadsheetApp.getUi();
  let seed = '';
  try {
    seed = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* sandbox quirk — seed stays empty */ }

  if (!seed) {
    ui.alert(
      'Initialize Admin Allowlist',
      'Could not determine your email. Please ensure you are signed in and try again.',
      ui.ButtonSet.OK,
    );
    log('warn', 'bootstrapGuildAdminsManual', { skipped: 'session_email_empty' });
    return;
  }

  const response = ui.alert(
    'Initialize Admin Allowlist',
    'About to add ' + seed + ' as the first admin and owner-floor. ' +
      'This is the seed identity that bootstraps the allowlist; the ' +
      'owner-floor protection means no one else will be able to remove ' +
      'this email. Continue?',
    ui.ButtonSet.OK_CANCEL,
  );
  if (response !== ui.Button.OK) {
    log('info', 'bootstrapGuildAdminsManual', { cancelled: true });
    return;
  }

  const result = bootstrapGuildAdmins({ seedEmail: seed, initiatedBy: 'manual_fallback' });
  if (result.bootstrapped) {
    getActiveSpreadsheet().toast('Admin allowlist initialized with ' + seed + '.');
  } else if (result.reason === 'already_initialized') {
    getActiveSpreadsheet().toast('Admin allowlist already initialized.');
  } else {
    getActiveSpreadsheet().toast(
      'Admin allowlist bootstrap failed: ' + (result.reason ?? 'unknown') + '.',
    );
  }
}
