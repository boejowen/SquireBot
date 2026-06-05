---
phase: 20-bot-dm-notification-infrastructure
plan: 01
subsystem: database
tags: [sqlite, goose, migrations, store-layer, idor, notifications, alert_log, discord]

# Dependency graph
requires:
  - phase: 19-wantlist-crud
    provides: "wantlist_item + alert_log (00006, full shape, zero rows) + the owner-scoped wantlist.go store grain (ListOwnWants/RemoveOwnWantTx, ErrDuplicateWant via modernc extended result code)"
provides:
  - "Migration 00007: notify_prefs (per-user opt-in, default-ON) + guild_channel (officer source channels) + monitor_flag (three guild-wide kill-switches, seeded) + alert_log rebuilt with NULLABLE wantlist_item_id + read_at + wantlist_item.muted"
  - "store/notifyprefs.go: GetPrefs (default-ON reader) + UpsertPrefsTx (owner-scoped upsert)"
  - "store/alertlog.go: ListInbox + MarkAlertReadTx + MarkAllAlertsReadTx + UnreadCount + InsertAlertTx (nullable wantID) + RecentAlertExists (sent OR dm_blocked dedup probe)"
  - "store/guildchannel.go: GetMonitorFlags + SetMonitorFlagTx + AddGuildChannelTx (ErrDuplicateChannel) + ListGuildChannels + RemoveGuildChannelTx"
  - "store/wantlist.go extended: WantlistRow.Muted + ListOwnWants returns muted + SetMutedTx (owner-scoped)"
affects: [20-02-notify-bot-seam, 20-03-notifications-monitors-handlers, 20-04-frontend-notifications-page, 21-ec-auction-monitor, 22-wts-monitor, 23-raid-target-monitor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Forward-only DROP+CREATE table rebuild inside a goose Up to change a column's NOT NULL on a zero-row table (BLOCKER-1 / alert_log nullability)"
    - "Default-ON reader: an absent prefs row reads as all-ON (sql.ErrNoRows -> NotifyPrefs{true,true,true,true}) rather than an error (D-01)"
    - "Dedup-probe send_status filter IN ('sent','dm_blocked') so a recent CAN'T-DM also suppresses a repeat (warning 5); 'error' rows stay retryable"

key-files:
  created:
    - internal/backendsrv/migrations/00007_notify.sql
    - internal/backendsrv/store/notifyprefs.go
    - internal/backendsrv/store/alertlog.go
    - internal/backendsrv/store/guildchannel.go
    - internal/backendsrv/store/notifyprefs_test.go
    - internal/backendsrv/store/alertlog_test.go
    - internal/backendsrv/store/guildchannel_test.go
  modified:
    - internal/backendsrv/migrations/migrate_test.go
    - internal/backendsrv/store/wantlist.go
    - internal/backendsrv/store/wantlist_test.go

key-decisions:
  - "alert_log rebuilt (DROP+CREATE) rather than ALTER'd — SQLite cannot drop a NOT NULL, but the table had zero rows (00006 wrote none; confirmed via the 00006 COUNT==0 assertion), so the rebuild copies nothing and also adds read_at from the start"
  - "RecentAlertExists filters send_status IN ('sent','dm_blocked') only — an 'error' send remains retryable, but a dm_blocked is suppressed once per window so a DMs-off user doesn't accrue identical inbox rows every cycle"
  - "guild_channel/monitor_flag funcs are NOT owner-scoped by design — officer authorization is enforced at the route + in-tx (Plan 03); these store funcs are unreachable except behind RequireOfficer"

patterns-established:
  - "Owner-scoped IDOR-safe mutator clones RemoveOwnWantTx (WHERE id=? AND discord_user_id=?) -> cross-owner RowsAffected=0 -> (false,nil) silent no-op: applied to MarkAlertReadTx and SetMutedTx"
  - "Typed conflict sentinel via the modernc *sqlite.Error.Code()==2067 (NOT a string-match): ErrDuplicateChannel mirrors ErrDuplicateWant"

requirements-completed: [WANT-03, WANT-04, WANT-08]

# Metrics
duration: ~22min
completed: 2026-06-05
---

# Phase 20 Plan 01: Notification Spine Data Layer Summary

**Forward-only goose migration 00007 (notify_prefs/guild_channel/monitor_flag + an alert_log rebuild making wantlist_item_id NULLABLE plus read_at + wantlist_item.muted) and the owner/officer-scoped store layer the bot seam, inbox, prefs, monitor controls, and per-want mute all read and write.**

## Performance

- **Duration:** ~22 min
- **Started:** 2026-06-05T19:20Z (approx)
- **Completed:** 2026-06-05T19:42:25Z
- **Tasks:** 3
- **Files modified:** 10 (7 created, 3 modified)

## Accomplishments
- **BLOCKER-1 resolved at the schema:** `alert_log` was rebuilt (DROP+CREATE inside 00007's forward Up, zero rows to copy) so `wantlist_item_id` is now NULLABLE — the D-10 officer test-alert (which has no `wantlist_item`) can log a `wantlist_item_id=NULL` row, proven by a passing NULL-FK insert in the migrate test. The rebuild also adds `read_at` (NULL = unread, D-05).
- **Notification opt-in spine landed:** `notify_prefs` (master + 3 per-monitor toggles, all DEFAULT 1, D-01), `guild_channel` (officer source channels with a `monitor` enum CHECK, D-07/D-08), `monitor_flag` (the 3 guild-wide kill-switches seeded `ec_auction=1, wts=0, raid_target=0` — EC ships ON, WTS/raid ship dark), and `wantlist_item.muted` (D-09).
- **Owner/officer-scoped store layer built against the final schema:** default-ON prefs reader, newest-first inbox with IDOR-safe mark-read/mark-all-read + unread count, nullable-wantID alert insert, the dedup/cooldown probe (suppresses recent `sent` OR `dm_blocked`, warning 5), owner-scoped per-want mute (BLOCKER-3 read path closed via `ListOwnWants.Muted`), and the officer guild-channel/monitor-flag CRUD with a typed `ErrDuplicateChannel`.
- **Forward-only invariant held:** 00001–00006 are byte-identical (`git diff` on 00006 is empty); the alert_log nullability change lives entirely in 00007.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 00007 + migrate_test assertion** - `5559452` (feat)
2. **Task 2: notify_prefs + alert_log + wantlist.muted store layer** - `bb1b666` (feat)
3. **Task 3: guild_channel + monitor_flag officer store layer** - `9663852` (feat)

_TDD: each task's behavior assertions and implementation landed in a single feat commit (the migration SQL and store funcs are the implementation a same-task test asserts against)._

## Files Created/Modified
- `internal/backendsrv/migrations/00007_notify.sql` - Forward-only DDL: notify_prefs, guild_channel, monitor_flag (seeded), alert_log rebuild (nullable wantlist_item_id + read_at), wantlist_item.muted
- `internal/backendsrv/migrations/migrate_test.go` - `TestMigrate_00007_AddsNotify`: tables/columns exist, NULL-FK insert succeeds, prefs default all-ON, 3 seeded flags, monitor CHECK bites, idempotent re-run
- `internal/backendsrv/store/notifyprefs.go` - `NotifyPrefs`, `GetPrefs` (default-ON on ErrNoRows), `UpsertPrefsTx` (owner-scoped ON CONFLICT)
- `internal/backendsrv/store/alertlog.go` - `AlertLogRow`, `ListInbox`, `MarkAlertReadTx`, `MarkAllAlertsReadTx`, `UnreadCount`, `InsertAlertTx` (nullable wantID), `RecentAlertExists` (sent/dm_blocked dedup)
- `internal/backendsrv/store/guildchannel.go` - `GuildChannel`, `MonitorFlags`, `GetMonitorFlags`, `SetMonitorFlagTx`, `AddGuildChannelTx` (+`ErrDuplicateChannel`), `ListGuildChannels`, `RemoveGuildChannelTx`
- `internal/backendsrv/store/wantlist.go` - `WantlistRow.Muted` + `ListOwnWants` returns muted + `SetMutedTx` (owner-scoped)
- `internal/backendsrv/store/{notifyprefs,alertlog,guildchannel}_test.go` + `wantlist_test.go` - behavior coverage (default-ON, owner-scope no-ops, nil-wantID test-alert, dm_blocked suppression, mute round-trip, dup channel)

## Decisions Made
- **alert_log rebuild vs ALTER:** SQLite can't drop a NOT NULL constraint, but the table has zero rows (verified safe via the existing 00006 `COUNT(*)==0` assertion), so `DROP TABLE alert_log; CREATE TABLE alert_log (...)` with the column nullable is the clean idempotent path — and it lets `read_at` land in the CREATE rather than a follow-up ALTER.
- **Dedup probe excludes `error`:** `RecentAlertExists` filters `send_status IN ('sent','dm_blocked')` so a transient send error stays retryable, while a recent CAN'T-DM (50007) is suppressed once per window (warning 5).
- **Officer funcs not owner-scoped:** `monitor_flag`/`guild_channel` state is guild-wide (D-08); authorization is the route's RequireOfficer + the Plan-03 in-tx `IsOfficerTx` re-check, not a per-row owner clause.
- **`b2i`/`boolToInt` helpers:** introduced two tiny bool→int converters (one per file) rather than sharing a single exported one — keeps each store file self-contained and matches the existing per-file helper style (`strptr` lives in the test file, etc.).

## Deviations from Plan

None — plan executed exactly as written. The only mid-task fix was adding the named `sqlite "modernc.org/sqlite"` import to `guildchannel.go` (Go imports are per-file; the package-level named import in `wantlist.go` does not carry over). This was an in-task compile fix required to use the established `*sqlite.Error.Code()` typed-conflict idiom the plan specified — not a design deviation. Committed in `9663852` (Task 3 commit).

## Issues Encountered
- **Per-file import scoping:** `guildchannel.go` failed to compile (`undefined: sqlite`) because Go imports are per-file, not per-package — the named `sqlite` import in `wantlist.go` is not visible in a sibling file. Resolved by adding the same named import to `guildchannel.go`. No new dependency (modernc.org/sqlite already required).

## Acceptance / Verification
- `go test ./internal/backendsrv/migrations/ ./internal/backendsrv/store/ -count=1` — all green.
- `go build ./...` — compiles.
- `gofmt -l` on all 10 created/modified files — clean. (Two pre-existing unrelated files, `itemids_test.go` and `readviews_test.go`, are gofmt-dirty but were NOT touched by this plan — logged to `deferred-items.md`, out of scope per the scope boundary.)
- Forward-only: `git diff internal/backendsrv/migrations/00006_wantlist.sql` is empty (00001–00006 untouched).

## Next Phase Readiness
- **Plan 02 (notify/wantmatch/bot seam)** can build against real signatures: `InsertAlertTx(wantID *int64, ...)`, `RecentAlertExists(...)`, `GetPrefs`, `GetMonitorFlags`, `ListGuildChannels`, and the `wantlist_item.muted` gate all exist.
- **Plan 03 (handlers)** has the owner-scoped prefs/inbox/mute funcs + the officer monitor-flag/channel CRUD ready; it owns the RequireOfficer route + in-tx `IsOfficerTx` re-check the officer store funcs assume.
- No blockers. discordgo (the v2.2 new dependency) is NOT introduced here — that lands with Plan 02's bot package.

## Self-Check: PASSED

- All 7 created files exist on disk.
- All 3 task commits (`5559452`, `bb1b666`, `9663852`) exist in git history.

---
*Phase: 20-bot-dm-notification-infrastructure*
*Completed: 2026-06-05*
