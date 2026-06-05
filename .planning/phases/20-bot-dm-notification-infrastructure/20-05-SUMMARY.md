---
phase: 20-bot-dm-notification-infrastructure
plan: 05
subsystem: ui
tags: [svelte, sveltekit, lucide, tanstack-table, officer-admin, wantlist, mute, monitors, test-alert]

# Dependency graph
requires:
  - phase: 20-bot-dm-notification-infrastructure (Plan 01)
    provides: "the muted read path — store.ListOwnWants + WantlistRow.Muted struct field + JSON tag (BLOCKER-3); no Go change needed in this plan"
  - phase: 20-bot-dm-notification-infrastructure (Plan 03)
    provides: "the officer monitor JSON contracts (GET monitors, flag, channel add/remove, test) + RequireOfficer routing + RequireSession /wantlist/mute"
  - phase: 20-bot-dm-notification-infrastructure (Plan 04)
    provides: "the Toggle primitive + the api.ts notify block (incl. muteWant) — Plan 05 appended its monitor block below it"
provides:
  - "The officer /admin Monitors section: three guild-wide kill-switch Toggles (EC on / WTS+raid dark), the add-channel FormField form, the registered-channel list with ConfirmDialog remove, and the D-10 'Send me a test alert' button with its three feedback states"
  - "The per-want mute bell (WantMuteCell) as a trailing 'Alerts' column on the /wantlist grid — bell/bell-off glyph carrying mute state, server-truth reload, disabled on custom wants"
  - "The five officer monitor api.ts wrappers (fetchMonitors/setMonitorFlag/addGuildChannel/removeGuildChannel/sendTestAlert) + the deterministic WantlistRow.muted TS field"
affects: [phase-20-prod-deploy-live-smoke, future-monitor-ui-phases]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Module-block pure helpers (testAlertFeedback + addChannelValid) split out of the .svelte for node-vitest testability (the Toggle/ConfirmDialog DOM-free split idiom)"
    - "Officer-form lifecycle cloned from BankCoinForm (onMount→fetch→form→save with classifyAdminError authGuard routing) extended to a multi-block panel"
    - "Trailing grid-control column threading new factory args (onMute/muteBusy) exactly like onRemove/removeBusy"

key-files:
  created:
    - web/src/lib/components/MonitorAdminPanel.svelte
    - web/src/lib/components/MonitorAdminPanel.test.ts
    - web/src/lib/components/cells/WantMuteCell.svelte
  modified:
    - web/src/lib/api.ts
    - web/src/routes/admin/+page.svelte
    - web/src/lib/columns.ts
    - web/src/lib/components/WantlistPanel.svelte
    - web/src/lib/__tests__/api.test.ts

key-decisions:
  - "Appended the // --- Monitors (20-05 / WANT-08) --- api.ts block BELOW Plan 04's notify block; fetchPrefs + muteWant untouched (BLOCKER-2 honored)"
  - "WantlistRow.muted added deterministically (Plan 01 returns it on every row) — not a conditional add-if-missing"
  - "addGuildChannel return typed as { added, channel_id, monitor } (the actual Go shape) rather than the plan's GuildChannel-row guess — the UI re-fetches state, so the add response shape is non-load-bearing"
  - "Browser-smoke (Task 3) CONSOLIDATED into the end-of-phase prod-deploy live smoke (Phase-19 pattern) — localhost cannot auth the live backend and the Phase-20 endpoints aren't deployed yet"

patterns-established:
  - "Pure decision-helper module block: the test-alert response→message map + the add-channel validity predicate live in MonitorAdminPanel.svelte's <script module> so they're node-unit-testable without jsdom"
  - "Non-destructive grid control (mute) mirrors the destructive one (remove) but with NO ConfirmDialog — a mute is reversible in one click (the BankCoinForm D-12 rule)"

requirements-completed: [WANT-08, WANT-03]

# Metrics
duration: ~20min
completed: 2026-06-05
---

# Phase 20 Plan 05: /admin Monitors Section + Per-Want Mute Bell Summary

**Officer /admin Monitors section (three guild-wide kill-switch Toggles, an add-channel form, a ConfirmDialog-gated channel list, and the D-10 "Send me a test alert" bot-pulse with three feedback states) plus a per-want bell/bell-off mute column on the /wantlist grid — all node-verified green, browser-smoke deferred to the end-of-phase prod-deploy live smoke.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-06-05T16:20:00Z (approx)
- **Completed:** 2026-06-05T16:30:00Z (approx)
- **Tasks:** 2 code tasks complete (Task 3 browser-smoke deferred — see Deviations)
- **Files modified:** 8 (3 created, 5 modified)

## Accomplishments
- The officer `/admin` Monitors section: three guild-wide kill-switch `Toggle`s (EC ON; WTS + raid dark with the "Ships dark — enable once the bot is invited to the source channels." note), the add-channel `FormField` form (server label / numeric channel id / monitor `<select>`), the registered-channel list with a `ConfirmDialog`-gated Remove (the one destructive action; Cancel default-focused), and the **"Send me a test alert"** button mapping the response to the three feedback states (sent / dm_blocked / bot_unavailable — word + icon, not color-only).
- The per-want **mute bell** (`WantMuteCell`) as a trailing "Alerts" column on the `/wantlist` grid: `bell`/`bell-off` glyph carries state, click → `muteWant(id, !muted)` → server-truth reload (never optimistic, no row-dim), and a disabled bell-off `—` on custom wants (`item_id` null) with the "Custom wants never trigger alerts." title.
- The five officer monitor `api.ts` wrappers appended BELOW Plan 04's notify block (which survived intact — `fetchPrefs` + `muteWant` untouched), and `WantlistRow.muted` added deterministically.

## Task Commits

1. **Task 1: officer monitor api.ts wrappers + MonitorAdminPanel + /admin section** — `d3b943e` (feat)
2. **Task 2: WantMuteCell + columns.ts "Alerts" column + WantlistPanel onMute wiring + WantlistRow.muted** — `fce18bf` (feat)

**Plan metadata:** (this SUMMARY commit)

## Files Created/Modified
- `web/src/lib/components/MonitorAdminPanel.svelte` - The officer Monitors UI (kill-switches + add-channel form + channel list + ConfirmDialog remove + test-alert) with two pure module-block helpers
- `web/src/lib/components/MonitorAdminPanel.test.ts` - Node vitest: the test-alert response→feedback map (three states) + the add-channel validity predicate
- `web/src/lib/components/cells/WantMuteCell.svelte` - The per-row bell/bell-off mute toggle; disabled bell-off on custom wants
- `web/src/lib/api.ts` - Appended the Monitors wrapper block (GuildChannel/MonitorFlags/MonitorState interfaces + 5 wrappers); added WantlistRow.muted
- `web/src/routes/admin/+page.svelte` - Added the third `.form-card` Monitors section to the officer-gated `.admin-area`
- `web/src/lib/columns.ts` - Added the trailing "Alerts" column to wantlistColumns; threaded onMute + muteBusy through the factory
- `web/src/lib/components/WantlistPanel.svelte` - Added onMute + muteBusy + the polite mute announce; passed them into the wantlistColumns factory call
- `web/src/lib/__tests__/api.test.ts` - Appended the five monitor-wrapper request-shape assertions (URL/method/body/no-owner); Plan 04's notify assertions untouched

## Decisions Made
- **api.ts append order:** the Monitors block sits below Plan 04's Notifications block; `fetchPrefs` and `muteWant` are preserved verbatim (BLOCKER-2).
- **`addGuildChannel` return type:** typed to the ACTUAL Go shape `{ added, channel_id, monitor }` (confirmed in `webadmin/monitors.go:193`) rather than the plan's `GuildChannel`-row guess — the UI re-fetches `fetchMonitors()` after a successful add, so the add response body is non-load-bearing.
- **`WantlistRow.muted` is deterministic:** Plan 01's `store.WantlistRow` carries `Muted bool \`json:"muted"\`` (`store/wantlist.go:60`), so the TS field mirrors it unconditionally — verified against the Go source, NOT added "if missing".
- **No Go files touched:** the backend mute read path + monitor handlers were Plans 01 and 03; this plan is frontend-only (`git diff -- '*.go'` is empty).

## Deviations from Plan

### 1. [Orchestrator decision — not a code deviation] Task 3 browser-smoke DEFERRED, not skipped

- **Found during:** Task 3 (the blocking `checkpoint:human-verify`)
- **Issue:** The plan's final task is a mandatory blocking browser-smoke against the **live** Discord bot + the deployed Phase-20 endpoints (the WANT-03 "bot pulse" keystone proof). Per the documented project constraint, localhost cannot authenticate against the live backend, AND the Phase-20 endpoints are not deployed yet, so the live DOM smoke cannot run now.
- **Resolution:** The orchestrator decided to CONSOLIDATE this smoke into the **end-of-phase prod-deploy live smoke** (deploy backend + frontend → smoke `/notifications` + `/admin` Monitors + the mute bell + the live test-alert together on squirebot.quest), exactly like Phase 19. The code tasks were executed and node-verified GREEN; this SUMMARY records the browser-smoke as **DEFERRED to a pending deploy-time HUMAN-UAT**, not skipped.
- **What still needs the live smoke (the deferred UAT checklist):**
  1. Officer sees the Monitors section; EC ON, WTS+raid dark; a toggle persists across reload; a non-officer cannot reach it.
  2. Add a channel → appears in the list; re-add → duplicate error.
  3. Remove a channel → ConfirmDialog (Cancel default-focused) → confirmed removal.
  4. **"Send me a test alert"** → a Discord DM lands AND a DELIVERED row appears in the officer's `/notifications` inbox (the WANT-03 end-to-end proof).
  5. DMs-off path → the CAN'T-DM (50007) feedback line + inbox log.
  6. `/wantlist` each row has an "Alerts" bell → toggles bell↔bell-off + announce + persists; a custom want shows a disabled bell; a muted row is NOT dimmed.

---

**Total deviations:** 0 auto-fixed code deviations; 1 orchestrator-directed checkpoint consolidation (browser-smoke → end-of-phase live smoke).
**Impact on plan:** No code scope change. The plan's code, tests, typecheck, and build are all green; only the live human-verification is deferred to the prod-deploy smoke (the same Phase-19 pattern).

## Issues Encountered
- The plan's `files_modified` listed `web/src/lib/api.test.ts`, but the repo's test lives at `web/src/lib/__tests__/api.test.ts`; assertions were appended there. The plan's `<interfaces>` block also guessed the `addGuildChannel` return as the full row — the actual Go handler returns `{ added, channel_id, monitor }`; the wrapper type matches the source. Neither affected behavior (the UI re-fetches state).

## Verification

- `cd web && npx vitest run` → **257 passed (20 files)** GREEN
- `cd web && npm run check` (svelte-check) → **0 errors, 0 warnings**
- `cd web && npm run build` → **built + wrote static site** OK
- `git diff -- '*.go'` → **empty** (no Go files touched)
- api.ts acceptance greps: `fetchMonitors|setMonitorFlag|addGuildChannel|removeGuildChannel|sendTestAlert` = 5 exports; `fetchPrefs` = 1 (Plan 04 block survived); `@html` in MonitorAdminPanel = 0 (officer label via {} only); `dm_blocked|bot_unavailable` present (the three test toasts); `header: 'Alerts'` = 1; `Custom wants never trigger alerts` present in WantMuteCell.

## User Setup Required
None - no external service configuration required by this plan. (The live test-alert UAT requires the Discord bot connected — that is the deferred end-of-phase prod-deploy smoke precondition, tracked there.)

## Next Phase Readiness
- All three Phase-20 frontend surfaces (`/notifications` from Plan 04, the `/admin` Monitors section + the `/wantlist` mute bell from this plan) are code-complete and node-verified.
- **Blocker for phase close:** the end-of-phase prod-deploy live smoke (the consolidated browser-smoke) must run against the deployed backend + connected bot to prove the WANT-03 bot pulse end-to-end (DM + inbox log) and confirm the toggles/mute persist server-side.

## Self-Check: PASSED

---
*Phase: 20-bot-dm-notification-infrastructure*
*Completed: 2026-06-05*
