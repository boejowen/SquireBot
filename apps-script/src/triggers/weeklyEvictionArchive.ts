// weeklyEvictionArchive — Phase 5 plan 05-02 task 2 (DOC-02 back-end).
//
// Weekly trigger (Sun 06:00 PT — installed by installTriggers, same
// hour window as weeklyStaleCharArchive) that processes the eviction
// queue produced by 05-04's commitEviction sidebar:
//
//   1. Read _meta.eviction_log JSON array.
//   2. For each entry whose grace_until < now: try to archive every
//      char in entry.chars. ATOMIC PER ENTRY — if any char in the entry
//      throws, the whole entry stays in the log for retry on the next
//      run. The entry's processed chars (those that succeeded before
//      the throw) are already irreversibly archived via moveCharToArchive's
//      idempotency, so the retry will short-circuit them and only retry
//      the failed one — there's no double-archive risk.
//   3. Future entries (grace_until > now) are kept in the log untouched.
//   4. Rewrite _meta.eviction_log only if the count changed (cheap
//      write-avoidance).
//
// 05-02 dependency direction: this plan READS the eviction log. The
// WRITE side (commitEviction populating eviction_log) ships in plan
// 05-04. Until then this trigger sees an empty log and is a no-op.

import { log } from '../lib/log';
import { readMetaRows, writeMetaRow } from '../lib/sheet-helpers';
import { moveCharToArchive } from '../lib/archive';

interface EvictionLogEntry {
  at: string;
  email: string;
  initiated_by: string;
  grace_until: string;
  chars: string[];
  reason: 'evicted';
}

export function weeklyEvictionArchive(): void {
  const meta = readMetaRows('_meta');
  const row = meta.find((r) => r.key === 'eviction_log');
  const list: EvictionLogEntry[] = row && row.value
    ? JSON.parse(row.value)
    : [];

  const now = Date.now();
  const remaining: EvictionLogEntry[] = [];
  let archived = 0;

  for (const entry of list) {
    const graceMs = Date.parse(entry.grace_until);
    if (!Number.isFinite(graceMs) || graceMs > now) {
      remaining.push(entry);
      continue;
    }
    // Grace expired — archive every char in the entry atomically.
    let entryFailed = false;
    for (const char of entry.chars) {
      if (!char) continue;
      try {
        moveCharToArchive(char, 'evicted');
        archived++;
      } catch (e) {
        log('warn', 'weeklyEvictionArchive', {
          failed: char, email: entry.email, error: String(e),
        });
        entryFailed = true;
        break;
      }
    }
    if (entryFailed) {
      // Keep the entry in the log; moveCharToArchive's idempotency
      // ensures the already-processed chars short-circuit on retry.
      remaining.push(entry);
    }
  }

  if (remaining.length !== list.length) {
    writeMetaRow('_meta', 'eviction_log', JSON.stringify(remaining));
  }
  writeMetaRow('_status', 'last_eviction_archive_run', new Date().toISOString());
  writeMetaRow('_status', 'last_eviction_archive_count', String(archived));
  log('info', 'weeklyEvictionArchive', {
    archived,
    processed: list.length - remaining.length,
    remaining: remaining.length,
  });
}
