---
phase: 20-bot-dm-notification-infrastructure
reviewed: 2026-06-05T21:32:41Z
depth: deep
files_reviewed: 20
files_reviewed_list:
  - internal/backendsrv/bot/bot.go
  - internal/backendsrv/notify/dm.go
  - internal/backendsrv/migrations/00007_notify.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/alertlog.go
  - internal/backendsrv/store/notifyprefs.go
  - internal/backendsrv/store/guildchannel.go
  - internal/backendsrv/store/wantlist.go
  - internal/backendsrv/wantmatch/match.go
  - internal/backendsrv/webadmin/notifications.go
  - internal/backendsrv/webadmin/monitors.go
  - internal/backendsrv/webadmin/wantlist.go
  - cmd/squirebot-server/main.go
  - docs/backend-deploy.md
  - web/src/lib/api.ts
  - web/src/lib/columns.ts
  - web/src/lib/stores/unread.ts
  - web/src/lib/components/Toggle.svelte
  - web/src/lib/components/NotificationRow.svelte
  - web/src/lib/components/NotificationPrefsPanel.svelte
  - web/src/lib/components/NotificationInbox.svelte
  - web/src/lib/components/MonitorAdminPanel.svelte
  - web/src/lib/components/cells/WantMuteCell.svelte
  - web/src/lib/components/SiteShell.svelte
  - web/src/lib/components/StateBlock.svelte
  - web/src/routes/notifications/+page.svelte
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 20: Code Review Report

**Reviewed:** 2026-06-05T21:32:41Z
**Depth:** deep
**Files Reviewed:** 25 (source files; tests scanned, not separately graded)
**Status:** issues_found (no BLOCKERs; 2 WARNINGs, both backlog-safe — none block deploy)

## Summary

I reviewed the entire phase-20 bot/DM/notification surface adversarially, with the
security-critical paths (bot-token handling, the recover boundary, IDOR scoping,
officer gating, the two-gate + dedup logic, SQL injection, XSS, and PII-in-logs)
traced through the actual shipped code rather than the plans.

**The security surface holds up.** Concretely verified:

- **Bot token** is read only from `os.Getenv("DISCORD_BOT_TOKEN")` (bot.go:56) and is
  never carried by any `slog` field — the connect log uses `cfg.GuildID` only
  (bot.go:124), the shutdown log uses `ctx.Err()`. The deploy doc uses a `<bot-token>`
  placeholder in a `chmod 600` env file. No token leak.
- **Recover boundary** is real: every goroutine and the Ready handler installs
  `defer recoverBoundary(op)` as its first line (bot.go:123, 137); `Start` is
  non-blocking and non-fatal (a `New`/`Open` error is *returned*, never panics), and
  `runServe` logs-and-continues on a bot error (main.go:244-247). A bot panic cannot
  reach the HTTP listener.
- **IDOR**: every notify/inbox/mark-read/mark-all/mute handler derives the owner from
  `caller(ctx)` (the session), never a body field. The store mutators match
  `... AND discord_user_id = ?` so a cross-owner mark-read/mute is `RowsAffected=0` →
  silent `false` no-op (alertlog.go:96-108, wantlist.go SetMutedTx). No body-supplied
  owner anywhere.
- **Officer gating**: `/admin/monitors/*` + test-alert are `RequireOfficer` at the route
  AND re-check `store.IsOfficerTx` inside the write tx (monitors.go:118, 175, 243, 302,
  341). The test-alert can only DM `caller(ctx)` with `Source:"test"` — no
  arbitrary-recipient vector.
- **Two-gate + dedup**: `notify.Send` enforces GATE 1 (monitor flag) AND GATE 2 (master
  + per-source pref) AND the cooldown probe before sending; the probe filters
  `send_status IN ('sent','dm_blocked')` so a repeat CAN'T-DM is suppressed; the `test`
  path bypasses gates+cooldown and its nil `WantID` is rendered log-safe via
  `wantIDLog` (-1) — no nil deref (dm.go:112-179, 241).
- **SQL**: every query is parameterized `?`; the dup-channel path uses the typed
  modernc extended result code, not a string match. The 00007 migration is forward-only
  (the `DROP+CREATE alert_log` is justified by zero rows; 00001-00006 untouched) and the
  migrate test proves the NULL-FK insert, the default-ON seed, and idempotency.
- **XSS**: zero new `{@html}`. Every user/server-controlled string (alert detail, source,
  officer channel label, item name) renders via Svelte `{}` auto-escape. The lone app
  `{@html}` (ItemTooltip) is untouched and out of this phase's data path.
- **PII/secrets in logs**: notify's slog lines carry `source` + `status` + the want id
  only — never the DM Body, item name, or token (dm.go:154-177); the audit detail
  carries flags/ids/counts only.

The findings below are quality/robustness items, not security or correctness blockers.

## Warnings

### WR-01: Test-alert success path discards the audit-write error and can return "sent" with no audit row

**File:** `internal/backendsrv/webadmin/monitors.go:340-349`
**Issue:** On the bot-enabled test-alert path, after `sendTestAlert` returns, the audit
is written in a second `withTx` whose error is discarded (`_ = withTx(...)`). If that tx
fails — most plausibly because the officer was removed between the send and the audit, so
the in-tx `IsOfficerTx` re-check returns `ErrNotAuthorized` and rolls back — the DM has
already gone out but **no `monitor_test_alert` audit row lands**, and the handler still
returns `200 {"status":"sent"}`. The audit trail silently loses a real send. (The DM only
ever targets the caller, so this is an audit-completeness gap, not a spam/authorization
hole.) Note also the dual-`withTx` shape means the officer re-check runs twice with a
send in between — a removed officer can still fire one DM-to-self even though the audit is
then refused, which is a slightly odd ordering.
**Fix:** Capture and at least log the audit-tx error so a dropped audit row is visible,
e.g.:
```go
if auditErr := withTx(ctx, db, func(tx *sql.Tx) error {
    okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
    if e != nil { return e }
    if !okOfficer { return store.ErrNotAuthorized }
    return AppendAuditTx(ctx, tx, "monitor_test_alert", callerID, map[string]any{"status": status}, now)
}); auditErr != nil {
    slog.Warn("test alert audit write failed", "status", status, "err", auditErr)
}
```
Backlog (not deploy-blocking): the send itself is correctly gated and self-targeted; only
the audit row is at risk.

### WR-02: Dedup probe and the alert-log write are not in one transaction — a concurrent duplicate alert can double-DM

**File:** `internal/backendsrv/notify/dm.go:137-179`
**Issue:** `RecentAlertExists` (the cooldown/dedup probe) runs on the plain `*sql.DB`
before the send, and the `sent` row is written afterward in a *separate*
`recordAttempt` transaction. There is no lock spanning the probe and the insert, so two
concurrent `Send` calls for the same `(want, source, item)` inside the window can both
pass the probe and both DM the user, defeating the dedup. The package comment leans on
"monitor mutex serialization" upstream, but `notify.Send` itself offers no guarantee and
is the documented single entry point for all three future monitors (P21-23) plus the
test-alert. As written the dedup is best-effort, not the once-per-window contract the
doc claims.
**Fix:** Either document the serialization precondition as a hard invariant `Send`
*requires* (and assert callers hold it), or move the dedup probe inside the same
`recordAttempt`-style `BEGIN IMMEDIATE` tx so the check-then-insert is atomic. Given the
~12-person guild scale a duplicate DM is low-harm, so backlog is acceptable — but the
"surfaced ONCE per window" guarantee in the dm.go header comment is currently overstated
and should be softened if the code isn't changed.

## Info

### IN-01: `addError` whitespace cleanup only collapses the first double-space

**File:** `web/src/lib/components/MonitorAdminPanel.svelte:230`
**Issue:** `\`Couldn't add that channel. ${reason} Nothing was added — try again.\`.replace('  ', ' ')`
uses a string-literal `.replace`, which replaces only the *first* occurrence. When
`reason` is empty there is exactly one double-space so it works today, but it is fragile
if the copy ever changes. Cosmetic only.
**Fix:** Build the message conditionally (omit the `reason` segment entirely when empty)
or use `.replace(/\s{2,}/g, ' ')`.

### IN-02: `markRead`/`markAllRead`/`mute`/`flag` failures after a successful server write swallow the error silently

**File:** `web/src/lib/components/NotificationInbox.svelte:60-66`, `NotificationPrefsPanel.svelte`, `MonitorAdminPanel.svelte`
**Issue:** In several handlers the `catch` after a successful mutation does `if (route(err)) return;` and then falls through with no user-facing message when `route` returns false (a non-auth error during the *re-fetch* after the write already committed). The code comments call this out as intentional ("the read succeeded server-side; the next load reconciles"), and it is a deliberate, defensible UX trade-off — flagged only so it is a conscious choice, not an oversight. No action required.
**Fix:** None required; optionally surface a transient "Refresh failed — your change was saved" hint for clarity.

---

_Reviewed: 2026-06-05T21:32:41Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
