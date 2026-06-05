---
phase: 20-bot-dm-notification-infrastructure
plan: 03
subsystem: api
tags: [discordgo, notifications, http, officer-monitors, idor, toctou, dm]

# Dependency graph
requires:
  - phase: 20-bot-dm-notification-infrastructure (Plan 01)
    provides: store layer — GetPrefs/UpsertPrefsTx, ListInbox/MarkAlertReadTx/MarkAllAlertsReadTx/UnreadCount, SetMutedTx, GetMonitorFlags/SetMonitorFlagTx, AddGuildChannelTx (ErrDuplicateChannel)/ListGuildChannels/RemoveGuildChannelTx, migration 00007
  - phase: 20-bot-dm-notification-infrastructure (Plan 02)
    provides: notify.Send + notify.Alert{WantID *int64} + ErrDMBlocked/ErrCooledDown/ErrGatedOff; bot.ConfigFromEnv/bot.Start/Bot.Session (recover-isolated, non-fatal)
provides:
  - "Login-only notifications HTTP layer: prefs (get/set), inbox (list/read/read-all), unread-count — owner-from-session (IDOR-safe)"
  - "Per-want mute handler (POST /api/v1/wantlist/mute), owner-scoped, audited"
  - "Officer Monitors HTTP layer: flags + guild_channel CRUD with in-tx IsOfficerTx re-check (TOCTOU close)"
  - "D-10 test-alert handler: DMs the calling officer via notify.Send with WantID=nil (NULL-FK test row) — the WANT-03 end-to-end proof"
  - "bot.Start wired non-fatally into runServe (after scheduler.Start, before ListenAndServe); shared *discordgo.Session injected into the test-alert handler"
  - "12 new gated routes (6 /notifications/* + 1 /wantlist/mute via RequireSession; 5 /admin/monitors/* via RequireOfficer)"
  - "DISCORD_BOT_TOKEN documented as an EnvironmentFile secret in docs/backend-deploy.md"
affects: [21-frontend-notifications, 22-message-content, frontend-officer-ui, deploy]

# Tech tracking
tech-stack:
  added: [github.com/bwmarrin/discordgo (botSession var type in main.go)]
  patterns:
    - "owner-from-session (caller(ctx)) on every member mutator — body never carries an owner (D-02)"
    - "in-tx IsOfficerTx re-check inside BEGIN IMMEDIATE on every officer mutator (WR-04 TOCTOU close)"
    - "typed-nil guard: SendTestAlertHandler takes a concrete *discordgo.Session (not the notify.Sender interface) so the nil check is a real nil-pointer check"
    - "non-fatal in-process goroutine: bot.Start error logs and continues; the HTTP API + scheduler serve regardless"

key-files:
  created:
    - internal/backendsrv/webadmin/notifications.go
    - internal/backendsrv/webadmin/notifications_test.go
    - internal/backendsrv/webadmin/monitors.go
    - internal/backendsrv/webadmin/monitors_test.go
  modified:
    - internal/backendsrv/webadmin/wantlist.go
    - internal/backendsrv/webadmin/wantlist_test.go
    - cmd/squirebot-server/main.go
    - docs/backend-deploy.md

key-decisions:
  - "test-alert builds notify.Alert{WantID:nil, Source:\"test\"} so the alert_log FK is NULL (BLOCKER-1), never WantID:0; source='test' is the only gate-bypassing source and it self-targets the caller"
  - "SendTestAlertHandler takes a concrete *discordgo.Session (not notify.Sender) to dodge the typed-nil-interface trap; b.Session() returns a real nil when the bot is disabled"
  - "bot.Start is non-fatal in runServe — a start error (or absent token) logs and the website/scheduler serve regardless (T-20-16)"

patterns-established:
  - "owner-from-session member mutators: caller(ctx) owner, withTx + AppendAuditTx, silent no-op on a cross-owner target (mirrors removed:false)"
  - "officer mutators: RequireOfficer at the route + in-tx store.IsOfficerTx re-check rolls back a just-removed officer's write"
  - "non-fatal in-process bot goroutine wired after scheduler.Start, before ListenAndServe, with the shared session injected into the handler that needs it"

requirements-completed: [WANT-03, WANT-04, WANT-08]

# Metrics
duration: 10min
completed: 2026-06-05
---

# Phase 20 Plan 03: Bot DM Notification HTTP Layer + runServe Wiring Summary

**The notifications/monitors HTTP spine — 12 gated routes (prefs/inbox/mute + officer monitors + the WantID=nil test-alert) plus a non-fatal in-process Discord bot wired into runServe; code-complete and verified bot-disconnected, live DM proof deferred to the end-of-phase deploy.**

## Performance

- **Duration:** ~10 min (code tasks; checkpoint setup ongoing by user)
- **Started:** 2026-06-05T15:04:08-05:00 (first task commit)
- **Completed:** 2026-06-05T15:14:15-05:00 (last task commit)
- **Tasks:** 3 code tasks complete + 1 human-action checkpoint (deploy-time UAT)
- **Files modified:** 8 (4 created, 4 modified)

## Accomplishments

- **notifications.go** — 6 login-only handlers (GetPrefs/SetPrefs/ListInbox/UnreadCount/MarkRead/MarkAllRead), owner-from-session, audited mutations, IDOR-safe (cross-owner is a silent no-op). Default-ON prefs for new users; inbox returns newest-first as a non-nil JSON array.
- **wantlist.go extended** — `MuteWantHandler` (POST /api/v1/wantlist/mute), owner-scoped `SetMutedTx`, audits `wantlist_mute` with want_id only; cross-owner mute returns the requested flag but changes nothing (mirrors removed:false).
- **monitors.go** — 6 officer handlers (MonitorFlags/SetMonitorFlag/AddGuildChannel/ListGuildChannels/RemoveGuildChannel/SendTestAlert). Every mutator re-checks `store.IsOfficerTx` in-tx (WR-04 TOCTOU close). Channel-id validated `^[0-9]+$`, label non-blank, monitor enum allow-list; `ErrDuplicateChannel` maps to 409.
- **D-10 test-alert** — `SendTestAlertHandler(db, botSession)` builds `notify.Alert{WantID:nil, Source:"test", DiscordUserID:caller}` and DMs the calling officer via `notify.Send`. Two gates honored: nil session → 503 `bot_unavailable`; `ErrDMBlocked` (50007) → 200 `dm_blocked` (already inbox-logged); other err → 502 `bot_unavailable`. WantID=nil so the alert_log row is a NULL FK (BLOCKER-1, never WantID:0). This is the WANT-03 end-to-end proof.
- **runServe wiring** — `bot.Start(ctx, bot.ConfigFromEnv())` inserted AFTER `scheduler.Start` and BEFORE the mux/`ListenAndServe`, non-fatal (a start error logs `continuing without it` and proceeds). The shared `*discordgo.Session` (`b.Session()`, a real nil when disabled) injected into the test-alert handler. 12 routes registered: 6 `/notifications/*` + 1 `/wantlist/mute` (RequireSession), 5 `/admin/monitors/*` (RequireOfficer).
- **Deploy doc** — `DISCORD_BOT_TOKEN` documented in docs/backend-deploy.md as an EnvironmentFile secret (chmod 600, root-only, never logged/bundled); note that the EnvironmentFile= line already loads it (no unit change) and an absent token leaves the bot disconnected while the server still boots. MESSAGE_CONTENT explicitly NOT needed in Phase 20 (DM-send is pure REST) — it's a Phase-22 toggle.

## Task Commits

Each task was committed atomically:

1. **Task 1: notifications.go (prefs + inbox + unread-count) + wantlist MuteWantHandler** — `9fcf790` (feat)
2. **Task 2: monitors.go (officer flags + guild_channel CRUD + D-10 test-alert)** — `221f038` (feat)
3. **Task 3: wire bot.Start + 12 routes into runServe + deploy-doc token** — `0f9303a` (feat)

**Plan metadata:** this SUMMARY (docs commit)

## Files Created/Modified

- `internal/backendsrv/webadmin/notifications.go` (created) — 6 login-only prefs/inbox/unread handlers, owner-from-session, audited
- `internal/backendsrv/webadmin/notifications_test.go` (created) — table-driven coverage: default-ON prefs, set round-trip + audit, inbox newest-first, unread-count, mark-read (incl. cross-owner no-op), read-all
- `internal/backendsrv/webadmin/monitors.go` (created) — officer flags + guild_channel CRUD + the D-10 test-alert + mapMonitorErr
- `internal/backendsrv/webadmin/monitors_test.go` (created) — flags GET, flag set + audit, add-channel valid/duplicate-409/invalid-400, remove, non-officer in-tx ErrNotAuthorized, nil-session bot_unavailable
- `internal/backendsrv/webadmin/wantlist.go` (modified) — added `MuteWantHandler` + `wantlist_mute` audit
- `internal/backendsrv/webadmin/wantlist_test.go` (modified) — mute toggle + cross-owner no-op + audit row
- `cmd/squirebot-server/main.go` (modified) — non-fatal bot.Start wiring + 12-route block + botSession injection
- `docs/backend-deploy.md` (modified) — DISCORD_BOT_TOKEN secret + the no-unit-change/disconnected-boot/MESSAGE_CONTENT notes

## Verification Evidence

- **`go build ./...`** — green (the new discordgo import in main.go + all handler packages compile).
- **Full test suite green** — `go test ./internal/backendsrv/... -count=1` passes across webadmin (notifications/monitors/wantlist), cmd/squirebot-server, bot, notify, wantmatch, and store.
- **12 routes confirmed wired** — 6 session `/notifications/*` + 1 session `/wantlist/mute` + 5 officer `/admin/monitors/*`; `SendTestAlertHandler(db, botSession)` registered once.
- **Bot wiring ordering** — `bot.Start(ctx, botCfg)` appears AFTER `scheduler.Start(ctx, db)` and BEFORE `ListenAndServe`, non-fatal (`continuing without it` on error).
- **Defer-live smoke** — a server booted with NO DISCORD_BOT_TOKEN: migrated to schema v7, logs the bot as disabled ("bot disabled"/"continuing without it"), listens on the HTTP port, and ingest returns 401 (auth gate intact). Every endpoint serves; the test-alert returns `bot_unavailable` (the nil-session branch), exactly as designed.
- **acceptance greps** — caller(r.Context()) ≥6 in notifications.go; 6 prefs/inbox handler funcs; MuteWantHandler + wantlist_mute present; IsOfficerTx in-tx re-check ≥4 in monitors.go; `Source: "test"` ×1; `WantID: nil` ×1; `bot_unavailable` ≥2; ErrDuplicateChannel ×1; numeric channel-id validation present; DISCORD_BOT_TOKEN ≥1 in the deploy doc.

## Bot Lifecycle & Security Constraints Honored

- **Recover-isolated, non-fatal:** bot.Start (Plan 02) is recover-bounded; runServe treats a start error as non-fatal (logs + continues). A bot panic cannot take down the website/scheduler (T-20-16).
- **Enabled flag / no token:** an absent/empty token leaves the bot disabled; `b.Session()` returns a real nil; the server still boots and serves.
- **DISCORD_BOT_TOKEN never logged:** documented only as an EnvironmentFile secret (chmod 600, root-only), never echoed in a response or log line (the existing DISCORD_CLIENT_SECRET posture).
- **Two-gate test-alert (D-10):** `WantID:nil` (NULL FK, BLOCKER-1) + `Source:"test"` (the only gate-bypassing source, and it self-targets caller(ctx)) — cannot be aimed at another user (T-20-13).
- **IDOR owner-from-session:** every member handler derives the owner from caller(ctx); the request body never carries an owner; a cross-owner target is a silent no-op (T-20-11, D-02).
- **TOCTOU close:** every officer mutator does RequireOfficer at the route + an in-tx `store.IsOfficerTx` re-check inside BEGIN IMMEDIATE — a just-removed officer's write rolls back (T-20-12, WR-04).
- **Injection:** AddGuildChannelHandler server-validates channel_id `^[0-9]+$`, non-blank label, monitor enum allow-list; parameterized store insert (T-20-14).

## Decisions Made

None beyond the plan — followed the plan's locked decisions (WantID=nil NULL-FK test row, concrete-*discordgo.Session typed-nil guard, non-fatal bot.Start) as specified.

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria and the verification block were satisfied by the three task commits.

## Issues Encountered

None during the code tasks. The only outstanding item is the live DM proof, which is a deploy-time UAT (see below), not a code problem.

## User Setup Required

**External Discord setup is required for the LIVE test-alert DM (the WANT-03 end-to-end proof).** The user chose `bot-live` and is performing these steps now:

1. Discord Developer Portal → the existing SquireBot application → **Bot** tab → Reset Token.
2. Invite the bot to the **guild's own Discord server** (OAuth2 → URL Generator → scope `bot`) so it shares a server with every guildie (the 50007 mutual-server precondition for DM-send).
3. On the Hetzner box, add `DISCORD_BOT_TOKEN=<token>` to root-only `/etc/squirebot/squirebot.env` (chmod 600), then restart the service.

**Sequencing fact:** the box currently runs the Phase-19 binary (no bot.Start), so the bot cannot connect until the Phase-20 backend is DEPLOYED. The live "bot connected guild=<id>" log + the live test-alert DM (DELIVERED inbox row) will therefore be verified at the **end-of-phase deploy** (after Plans 04/05), when the new binary + migration 00007 ship and the restart picks up the token.

## Outstanding (deploy-time UAT — NOT a code gap)

- **Live "bot connected" log** — confirmed via journald only after the Phase-20 binary is deployed and the token is present.
- **Live test-alert DM** (the WANT-03 end-to-end proof) — the officer "Send me a test alert" button DMs the clicking officer and logs a DELIVERED row in their Notifications inbox; verifiable only once Plan 05's officer UI ships AND the bot is connected on the box.

Both are HUMAN-UAT / deploy items. The code path is complete and unit-tested for the nil/blocked/sent branches; the live proof is gated solely on the external Discord setup + the end-of-phase deploy.

## Requirements Satisfied

- **WANT-03** — bot DM capability: the test-alert handler DMs the calling officer via notify.Send with WantID=nil (NULL-FK test row). Code complete + unit-tested (nil/blocked/sent branches); live proof at the end-of-phase deploy.
- **WANT-04** — prefs + inbox + unread-count + mark-read/read-all + mute handlers wired, owner-from-session, deduped via the Plan-02 notify layer.
- **WANT-08** — officer monitor flags + guild_channel CRUD wired, officer-gated + in-tx re-checked + audited.

## Next Phase Readiness

- The full backend spine is complete and compiles; this was the last backend plan. Plans 04/05 (frontend: member notifications UI + officer monitors UI) are unblocked — they consume the 12 routes documented here.
- The bot goroutine is non-fatal: the website serves with or without a token, so the frontend can be built and merged before the Discord setup lands.
- **Deploy blocker reminder:** the live bot connection + test-alert DM depend on the end-of-phase deploy of the Phase-20 binary (migration 00007 + DISCORD_BOT_TOKEN on the box). Confirm `bot connected guild=<id>` in journald post-restart before signing off WANT-03 live.

## Self-Check: PASSED

All 7 listed files exist on disk; all 3 task commits (9fcf790, 221f038, 0f9303a) found in git history.

---
*Phase: 20-bot-dm-notification-infrastructure*
*Completed: 2026-06-05*
