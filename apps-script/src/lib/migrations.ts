// migrateToV2 — Phase 3 plan 03-01 task 4. Idempotent migration that
// extends Phase 2's frozen schema_version=1 scaffold to v2.
//
// What changes:
//   - _pigparse: +9 cols (direction, t30, a30, t60, a60, t6m, a6m, ty, ay)
//   - _item_master: +1 col (wikitext_sha1)
//   - _quest_items: +1 col (source: 'in_game_flag' | 'notes_link')
//   - _meta: +2 KV rows (theme=minimalist, contact_email='')
//   - _meta.schema_version: 1 → 2 (LAST write — commits the migration)
//
// Pre-existing v1 workbooks already have _meta.theme + _meta.contact_email
// scaffolded by the Go side from plan 03-01 task 1, but the writeMetaRow
// helper is write-if-absent, so this stays safe.
//
// Pre-condition: WatcherMaxSchemaVersion >= 2 (bumped in plan 03-01
// task 1; ship watcher v0.3.0+ before running this).
//
// Algorithm correctness: schema_version write is the LAST step. If
// anything before it fails, the migration replays cleanly on the next
// trigger fire (appendColumns + writeMetaRow are both idempotent).

import { log } from './log';
import { appendColumns, readMetaRowInt, writeMetaRow } from './sheet-helpers';

export type MigrationOutcome = 'noop_already_v2' | 'migrated_v1_to_v2' | 'noop_unsupported_version';

const PIGPARSE_V2_COLUMNS = ['direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay'];
const ITEM_MASTER_V2_COLUMNS = ['wikitext_sha1'];
const QUEST_ITEMS_V2_COLUMNS = ['source'];

export function migrateToV2(): MigrationOutcome {
  const current = readMetaRowInt('_meta', 'schema_version');
  if (current === 2) {
    return 'noop_already_v2';
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
    return 'migrated_v1_to_v2';
  } finally {
    lock.releaseLock();
  }
}
