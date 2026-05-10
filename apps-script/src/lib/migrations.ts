// Schema migrations for the SquireBot workbook.
//
// Each migrateToVN function is idempotent: detecting that the workbook
// is already at version N OR at an unsupported version returns a noop
// outcome without writes. Detecting a write-eligible state acquires the
// document lock, performs all column/row extensions, and writes
// _meta.schema_version LAST so a partial run replays cleanly.
//
// Pre-condition for each migration: the watcher's WatcherMaxSchemaVersion
// constant in internal/sheet/client.go MUST be ≥ N before clasp push,
// otherwise older watchers refuse to write to the migrated workbook with
// ErrSchemaTooNew.
//
//   migrateToV2 — Phase 3 plan 03-01:
//     - _pigparse: +9 cols (direction, t30, a30, t60, a60, t6m, a6m, ty, ay)
//     - _item_master: +1 col (wikitext_sha1)
//     - _quest_items: +1 col (source: 'in_game_flag' | 'notes_link')
//     - _meta: +2 KV rows (theme=minimalist, contact_email='')
//     - _meta.schema_version: 1 → 2 (LAST)
//   migrateToV3 — Phase 4 plan 04-01:
//     - _char_owner: +1 col (race) — populated lazily via showCharInfoSidebar
//     - _meta.schema_version: 2 → 3 (LAST)

import { log } from './log';
import { appendColumns, readMetaRowInt, writeMetaRow } from './sheet-helpers';

// Outcome enum redesigned in Phase 4 plan 04-01: was version-specific
// ('noop_already_v2', 'migrated_v1_to_v2'); now version-agnostic so
// future migrations don't proliferate enum members.
export type MigrationOutcome =
  | 'noop_already_current'    // workbook is already at the target version
  | 'migrated'                // a successful migration ran
  | 'noop_unsupported_version'; // workbook is at a version this migration can't bridge

const PIGPARSE_V2_COLUMNS = ['direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay'];
const ITEM_MASTER_V2_COLUMNS = ['wikitext_sha1'];
const QUEST_ITEMS_V2_COLUMNS = ['source'];

export function migrateToV2(): MigrationOutcome {
  const current = readMetaRowInt('_meta', 'schema_version');
  if (current === 2) {
    return 'noop_already_current';
  }
  if (current !== 1) {
    log('warn', 'migrateToV2', { skipped: 'unsupported_version', current });
    return 'noop_unsupported_version';
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    log('warn', 'migrateToV2', { skipped: 'lock_busy' });
    throw new Error('migrateToV2: could not acquire document lock within 30s');
  }
  try {
    const pigAdded = appendColumns('_pigparse', PIGPARSE_V2_COLUMNS);
    const masterAdded = appendColumns('_item_master', ITEM_MASTER_V2_COLUMNS);
    const questAdded = appendColumns('_quest_items', QUEST_ITEMS_V2_COLUMNS);
    writeMetaRow('_meta', 'theme', 'minimalist');
    writeMetaRow('_meta', 'contact_email', '');
    // schema_version write is LAST — committing the migration.
    writeMetaRow('_meta', 'schema_version', '2');
    log('info', 'migrateToV2', {
      done: true, pigAdded, masterAdded, questAdded,
    });
    return 'migrated';
  } finally {
    lock.releaseLock();
  }
}

const CHAR_OWNER_V3_COLUMNS = ['race'];

export function migrateToV3(): MigrationOutcome {
  const current = readMetaRowInt('_meta', 'schema_version');
  if (current === 3) {
    return 'noop_already_current';
  }
  if (current !== 2) {
    log('warn', 'migrateToV3', { skipped: 'unsupported_version', current });
    return 'noop_unsupported_version';
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) {
    log('warn', 'migrateToV3', { skipped: 'lock_busy' });
    throw new Error('migrateToV3: could not acquire document lock within 30s');
  }
  try {
    const charAdded = appendColumns('_char_owner', CHAR_OWNER_V3_COLUMNS);
    // schema_version write is LAST — committing the migration.
    writeMetaRow('_meta', 'schema_version', '3');
    log('info', 'migrateToV3', { done: true, charAdded });
    return 'migrated';
  } finally {
    lock.releaseLock();
  }
}
