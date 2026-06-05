---
phase: 20-bot-dm-notification-infrastructure
verified: 2026-06-05T16:40:00Z
status: human_needed
score: 5/5 must-haves verified (code); 2 deploy-time UATs pending
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification:
  - test: "Live bot connect + 'Send me a test alert' DMs the clicking officer and logs a DELIVERED row in their /notifications inbox"
    expected: "After the Phase-20 backend deploys with DISCORD_BOT_TOKEN set and the bot invited to the guild's own server, journald shows 'bot connected guild=<id>'; clicking 'Send me a test alert' on /admin DMs the officer in Discord AND logs a DELIVERED inbox row — the WANT-03 / ROADMAP SC#5 end-to-end keystone proof. A DMs-off officer instead gets the CAN'T-DM toast + a dm_blocked inbox row (SC#2 live 50007 path)."
    why_human: "The box currently runs the Phase-19 binary; the in-process bot connects only after the Phase-20 backend deploys. Requires the live Discord gateway + a real DM round-trip — cannot be exercised from a unit test or localhost. User is setting up the Discord bot (token + guild invite) now."
  - test: "Live frontend browser-smoke of /notifications + /admin Monitors + the /wantlist mute bell against the deployed backend"
    expected: "Signed in as a guildie on squirebot.quest: /notifications shows prefs (default-ON master + 3 toggles) + the inbox with delivery badges (word+icon), the toggle shows a clean ON/OFF word (P15 coercion guard), and timestamps read as sane relative time (P15 epoch-seconds guard); /admin shows the officer Monitors section (EC on, WTS/raid dark); the /wantlist mute bell toggles + persists across reload."
    why_human: "web/ vitest is DOM-blind (no @testing-library/svelte) and localhost cannot authenticate the live backend; the live DOM smoke is consolidated into the end-of-phase prod-deploy smoke (the same deploy-then-smoke pattern Phase 19 used). Node-level checks (vitest 257 green, svelte-check 0/0, build OK) are done and pass, including the two P15-class crasher guards."
---

# Phase 20: Bot + DM + Notification Infrastructure Verification Report

**Phase Goal:** The keystone alerting spine exists — an in-process Discord bot can DM any guildie from the guild's own server; alerts are opt-in, deduplicated/cooled-down, and recorded in an in-site inbox that serves as the can't-DM fallback; officers can enable/disable each monitor and register source channels per server. (No real EC/WTS/raid monitors yet — those are Phases 21-23.)
**Verified:** 2026-06-05T16:40:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + PLAN must_haves)

| #   | Truth (ROADMAP SC) | Status | Evidence |
| --- | ------------------ | ------ | -------- |
| 1   | `squirebot-server` starts an in-process discordgo gateway goroutine behind an `Enabled` flag — `recover()`-isolated, non-fatal, ctx-tied; a bot panic can never take down the website/ingest | ✓ VERIFIED | `bot/bot.go`: `Start` non-blocking, returns `(&Bot{}, nil)` when disabled, `dg.Open()` error RETURNED not fatal; `recoverBoundary` deferred in every goroutine; ctx-tied `<-ctx.Done()` → `dg.Close()`. Wired in `main.go:243-251` AFTER `scheduler.Start` (:237), BEFORE the mux/ListenAndServe; start error logged "continuing without it" (non-fatal). `IntentsGuilds | IntentsDirectMessages` only (no MESSAGE_CONTENT). `go build` clean; `bot` tests pass. |
| 2   | The bot can DM a guildie; an undeliverable DM (50007) is marked `dm_blocked` and surfaced in the in-site inbox, never silently dropped | ✓ VERIFIED (code) / live DM pending | `notify/dm.go`: `isDMBlocked` checks typed `*discordgo.RESTError` code `ErrCodeCannotSendMessagesToThisUser`; 50007 → `recordAttempt("dm_blocked")` + `ErrDMBlocked`; every attempt logged via local `db.Begin` tx (no webadmin import). Inbox surfaces it: `NotificationRow.svelte` maps `dm_blocked` → "CAN'T DM" word+icon + the actionable hint line. `notify` tests pass. **Live 50007 round-trip = deploy-time UAT.** |
| 3   | A guildie can opt in/out + set prefs; repeat matches for the same `(wantlist_item, source, item)` suppressed within a per-source cooldown; every attempt recorded in `alert_log` | ✓ VERIFIED | `store/notifyprefs.go` GetPrefs default-ON (D-01) + UpsertPrefsTx; `notify/dm.go` two-gate (officer flag + user pref) + `RecentAlertExists` cooldown (`IN ('sent','dm_blocked')` suppresses repeat dm_blocked); per-source windows EC=22h/WTS=90m/raid=90m/test=0. Handlers `notifications.go` (6, RequireSession, owner-from-`caller(ctx)`, audited). `store`+`webadmin` tests pass. |
| 4   | An officer can enable/disable each monitor + register source channels per server (feature flags + `guild_channel` rows), ship-dark, flip on with no rebuild | ✓ VERIFIED | Migration 00007 seeds `monitor_flag` ec_auction=1/wts=0/raid_target=0 (ships dark); `store/guildchannel.go` GetMonitorFlags/SetMonitorFlagTx/Add/List/RemoveGuildChannelTx (+typed `ErrDuplicateChannel`); `monitors.go` 5 officer handlers w/ in-tx `IsOfficerTx` re-check (WR-04) + server-side enum/numeric validation + audit. 5 `/admin/monitors/*` routes RequireOfficer-gated in `main.go:364-368`. |
| 5   | `wantmatch` (single shared `ForItem` + `ForName`) exercised end-to-end via a test trigger that DMs a real guildie, proving the spine | ✓ VERIFIED (code) / live DM pending | `wantmatch/match.go` ForItem (stable id) + ForName (exact-name COLLATE NOCASE, NOT substring) both apply `active=1 AND muted=0`. `SendTestAlertHandler` builds `Alert{WantID:nil, Source:"test"}`, DMs `caller(ctx)`, typed-nil guard on `*discordgo.Session`, maps sent/dm_blocked/bot_unavailable. **The live DM that completes the end-to-end proof = deploy-time UAT.** |

**Score:** 5/5 success criteria verified at the code level. SC#2 (live undeliverable) and SC#5 (live DM) have a deploy-time human-UAT component that is intentionally deferred (the box runs the Phase-19 binary until the Phase-20 backend deploys).

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00007_notify.sql` | notify_prefs + guild_channel + monitor_flag (seeded) + alert_log rebuild (nullable wantlist_item_id + read_at) + wantlist_item.muted | ✓ VERIFIED | Latest migration; matches plan byte-for-byte intent; DROP+CREATE alert_log nullable FK; muted via plain DEFAULT ALTER; migrate test green |
| `internal/backendsrv/store/{notifyprefs,alertlog,guildchannel}.go` + wantlist.muted | default-ON prefs, inbox+dedup probe, officer CRUD, SetMutedTx + ListOwnWants muted | ✓ VERIFIED | All funcs present, owner/officer-scoped, parameterized, non-nil slices; tests green |
| `internal/backendsrv/bot/bot.go` | recover-isolated non-fatal Start, ConfigFromEnv, Session/Connected | ✓ VERIFIED | All present; token never logged; no MESSAGE_CONTENT |
| `internal/backendsrv/notify/dm.go` | DM send + 50007→dm_blocked + dedup + two-gate + local tx | ✓ VERIFIED | All present; no webadmin import (cycle avoided); Body never logged |
| `internal/backendsrv/wantmatch/match.go` | ForItem/ForName w/ active+muted gates | ✓ VERIFIED | Exact-name (no substring); mute+active at the seam |
| `internal/backendsrv/webadmin/{notifications,monitors}.go` + wantlist mute | 6 prefs/inbox + 6 monitor/test-alert handlers + MuteWantHandler | ✓ VERIFIED | All present, gated, audited, IDOR-safe |
| `cmd/squirebot-server/main.go` | bot.Start wiring + 12 routes | ✓ VERIFIED | bot.Start after scheduler, non-fatal; 6+1+5 routes gated; botSession injected |
| `web/.../{Toggle,NotificationPrefsPanel,NotificationInbox,NotificationRow,MonitorAdminPanel,cells/WantMuteCell}.svelte` + `/notifications/+page.svelte` + api.ts | page + components + 12 wrappers + WantlistRow.muted | ✓ VERIFIED | All exist (102-660 lines each); P15 guards present; no `{@html}` directive on user data |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| main.go runServe | bot package | `bot.Start(ctx, botCfg)` after scheduler, non-fatal | ✓ WIRED | main.go:243-244; awk-order scheduler(237)<bot(244) |
| monitors.go SendTestAlertHandler | notify.Send | source="test", WantID=nil, injected botSession | ✓ WIRED | sendTestAlert var → notify.Send; nil-guard returns bot_unavailable |
| notifications.go | store (GetPrefs/ListInbox/MarkRead/...) | owner from caller(ctx), withTx+audit | ✓ WIRED | all 6 handlers use caller(r.Context()) |
| notify/dm.go | store (RecentAlertExists/GetPrefs/GetMonitorFlags/InsertAlertTx) | dedup + two-gate + record | ✓ WIRED | all present in Send |
| WantMuteCell → columns.ts → WantlistPanel → muteWant → /wantlist/mute → MuteWantHandler → SetMutedTx | full mute chain | onMute callback + server-truth reload | ✓ WIRED | columns.ts:260 "Alerts" col; WantlistPanel onMute re-fetches |
| SiteShell | /notifications | nav link + fetchUnreadCount badge, count in aria-label | ✓ WIRED | notifyLabel "Notifications, N unread" |
| admin/+page.svelte | MonitorAdminPanel | third .form-card section | ✓ WIRED | imported + mounted line 24/51 |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Static Go build (CGO_ENABLED=0) | `go build ./...` | exit 0 | ✓ PASS |
| Phase-20 Go tests (6 packages) | `go test ./.../{migrations,store,bot,notify,wantmatch,webadmin}` | all ok | ✓ PASS |
| Web vitest | `npx vitest run` | 257 passed (20 files) | ✓ PASS |
| Web typecheck | `npm run check` (svelte-check) | 0 errors, 0 warnings, 474 files | ✓ PASS |
| Web production build | `npm run build` | built OK, site written | ✓ PASS |
| discordgo dependency | `grep go.mod` | `github.com/bwmarrin/discordgo v0.29.0` | ✓ PASS |
| No `{@html}` directive on user data | grep across notify/monitor/mute components | only 2 matches, both inside `// never {@html}` comments | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| WANT-03 | 20-01/02/03/05 | In-process recover-isolated non-fatal bot can DM a guildie from the guild's own server | ✓ SATISFIED (code) / live DM = deploy UAT | bot.go + notify.Send + test-alert handler + wiring; live DM is SC#5 deploy-time |
| WANT-04 | 20-01/02/03/04 | Opt in/out + prefs; dedup w/ cooldown; in-site inbox as 50007 fallback | ✓ SATISFIED | prefs handlers + RecentAlertExists + alert_log inbox + NotificationRow CAN'T-DM |
| WANT-08 | 20-01/03/05 | Officer enable/disable each monitor + register source channels (ship dark, flip on, no rebuild) | ✓ SATISFIED | monitor_flag seed + guild_channel CRUD + officer handlers + MonitorAdminPanel |

No orphaned requirements: REQUIREMENTS.md maps exactly WANT-03/04/08 to Phase 20, matching all PLAN frontmatter `requirements` fields.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none material) | — | — | — | Scanned source for stubs/empty-returns/{@html}/console-only handlers; none found. The 2 `@html` grep hits are comments asserting "never {@html}", not directives. `return null`/`[]` patterns in store readers are correct non-nil slice + nullable-pointer handling, not stubs. |

Pre-existing gofmt drift in two unrelated test files (`itemids_test.go`, `readviews_test.go`) is documented in `deferred-items.md` as out-of-scope (not introduced by Phase 20); informational only.

### Human Verification Required

1. **Live bot connect + test-alert DM** — Deploy the Phase-20 backend with `DISCORD_BOT_TOKEN` set and the bot invited to the guild's own server; confirm `bot connected` in journald; click "Send me a test alert" → receive a Discord DM + a DELIVERED inbox row (WANT-03 / SC#5). A DMs-off officer → CAN'T-DM toast + dm_blocked inbox row (SC#2 live 50007).
2. **Live frontend browser-smoke** — Against the deployed site: /notifications prefs + inbox (clean ON/OFF word, sane relative timestamps — the two P15 guards), /admin Monitors section (EC on, WTS/raid dark), /wantlist mute bell persists.

Both are the standard deploy-then-smoke pattern (same as Phase 19) and are the documented blockers for phase close. They are deploy-time UATs, NOT code gaps.

### Gaps Summary

No code gaps. All 5 ROADMAP success criteria, all declared artifacts, and all key links are verified in the actual codebase. The full backend spine (migration 00007, recover-isolated non-fatal bot wired in runServe, notify with 50007→dm_blocked + dedup/cooldown + two-gate + local tx, wantmatch, 12 gated routes, the test-alert handler with WantID=nil) and the full frontend (/notifications page, /admin Monitors, /wantlist mute bell, no `{@html}` on user data, both P15 crasher guards) are present and substantive. Go build + 6-package test suite + web vitest (257) + svelte-check (0/0) + production build all pass.

The only outstanding items are the two deploy-time human UATs (the live "bot connected" + live test-alert DM, and the live browser smokes) which are intentionally deferred to the end-of-phase prod deploy — the box currently runs the Phase-19 binary and localhost cannot authenticate the live backend. Per the Step 9 decision tree, a non-empty human-verification section forces **status: human_needed** even though the code score is 5/5.

---

_Verified: 2026-06-05T16:40:00Z_
_Verifier: Claude (gsd-verifier)_
