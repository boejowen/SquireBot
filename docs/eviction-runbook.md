# Eviction runbook (DOC-02)

Eviction is the documented workflow for cleanly removing a departed guildie
from the workbook. SquireBot ships the sidebar code (Phase 5 plan 05-04) and
the grace-period archiver (plan 05-02); this runbook describes the end-to-end
officer experience.

## Lifecycle

1. **Owner action (manual).** In the Google Sheets `Share` dialog, remove the
   departing guildie's email from the workbook share. Their watcher will start
   failing on next batchUpdate (tray turns red on their end). This is the
   access-revocation step.

2. **Mark the chars in the workbook.** From `SquireBot → Evict Guildie…`,
   select the guildie's email from the dropdown. Verify the affected-chars
   preview shows the correct list. Click `Evict` and confirm in the native
   browser modal.

   Under the hood, this writes:
   - `_char_owner.is_removed = TRUE` for every row matching `owner_email`.
   - One JSON entry to `_meta.eviction_log` with
     `{at, email, initiated_by, grace_until, chars, reason:'evicted'}`
     (grace_until = now + 30 days).

3. **Wait 30 days.** The `weeklyEvictionArchive` cron fires every Sunday
   06:00 PT. As soon as `grace_until < now` for any entry, it:
   - Snapshots each char's `inv:<Char>` and `spell:<Char>` rows into the
     hidden `_archive` tab (lazy-created on first archive).
   - Hides the source `inv:<Char>` + `spell:<Char>` tabs (NOT deletes —
     supports recovery if the guildie returns).
   - Appends a `_meta.archive_log` entry.
   - Removes the processed entry from `_meta.eviction_log` so subsequent runs
     don't double-archive.

## Un-evict (within the 30-day grace period)

If you mark someone in error, or they return before grace expires:

1. Open `_meta` (unhide via `Right-click tab strip → Unhide` if necessary).
2. Locate each affected char in `_char_owner`.
3. Manually edit each `is_removed` cell from `TRUE` back to `FALSE`. The
   protection prompt warns you but does NOT block the edit.
4. The eviction-log entry for those chars is left in place for audit purposes.
   The cron's next run is a no-op for chars that have been restored (their
   `is_removed` is now FALSE).
5. Re-add the guildie's email to the workbook share if you previously removed
   it.

## Post-archive recovery (after grace elapsed)

If a char was archived (grace ran out):

- **Option A (data preserved):** Locate the `_archive` row(s) for the char
  (filter by `char_name`). The `snapshot_json` column contains the full
  source-tab content as JSON. Manually paste it back into a re-created
  `inv:<Char>` or `spell:<Char>` tab.
- **Option B (clean restart):** If the guildie reinstalls SquireBot and lets
  the watcher run, WATCH-09's startup catch-up writes fresh content into
  `inv:<Char>` and `spell:<Char>`. Note: the watcher recreates the tab by
  name; the previously-hidden archive snapshot stays in `_archive` for
  posterity.
- **Option C (version history):** Sheets retains 30 days of version history.
  If you can pinpoint a pre-archive timestamp, restore via
  `File → Version history → See version history`.

## Edge cases

- **A guildie has multiple emails.** Eviction is per-email. Repeat the sidebar
  flow for each known email.
- **Two officers run eviction concurrently.**
  `LockService.getDocumentLock().tryLock(30000)` serializes the cascade; the
  second run sees the updated `_char_owner` state.
- **Partial eviction (one char of two).** The eviction sidebar always flips
  ALL chars matching the email. To evict only one char, manually edit that
  single `is_removed` cell to TRUE and leave the eviction-log alone
  (auto-archive will fire 90 days later via the stale-char route).
- **`_meta.eviction_log` corruption.** If the existing log JSON is malformed,
  `commitEviction` logs a warning and treats the prior list as empty before
  appending — the new entry still lands, but old entries may be lost. Recover
  prior entries via Sheets version history if needed.

## Permissions note (v1 limitation)

In v1, ANY workbook editor can run the Evict Guildie sidebar. The
trusted-guild model + the 30-day grace + the audit log
(`_meta.eviction_log`) mitigate the worst case. A `_meta.guild_admins`
allowlist is a v1.0.x polish candidate; tracked in the deferred backlog.

## Related artifacts

- `apps-script/src/triggers/showEvictionSidebar.ts` — sidebar opener +
  3 server callbacks (this plan: 05-04).
- `apps-script/src/triggers/weeklyEvictionArchive.ts` — the grace-expired
  archiver cron (plan: 05-02).
- `apps-script/src/lib/archive.ts` — the shared `moveCharToArchive` helper
  used by both the eviction archiver and the stale-char auto-archiver
  (plan: 05-02).
