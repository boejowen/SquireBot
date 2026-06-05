---
phase: 20-bot-dm-notification-infrastructure
plan: 04
subsystem: ui
tags: [sveltekit, svelte5-runes, notifications, accessibility, xss, vitest, lucide]

# Dependency graph
requires:
  - phase: 20-bot-dm-notification-infrastructure (Plan 03)
    provides: the six /api/v1/notifications endpoints (prefs get/set, inbox, unread-count, read, read-all), all RequireSession + owner-scoped (D-02)
  - phase: 19-wantlist-crud
    provides: the api.ts getJSON/postJSON cores + the /wantlist load→server-truth-reload panel idiom + StateBlock/SiteShell/StatusCell patterns to clone
provides:
  - the guildie-facing /notifications page (alert preferences + the full alert inbox)
  - the one new accessible Toggle primitive (role=switch + ON/OFF word), reused by Plan 05's officer kill-switches
  - the unread-count Notifications nav badge (the load-bearing CAN'T-DM "you missed something" signal)
  - the notifications api.ts wrapper block (fetchPrefs/savePrefs/fetchInbox/fetchUnreadCount/markRead/markAllRead) + muteWant (consumed by Plan 05)
  - the unread-count store (cross-component badge refresh after mark-read)
  - StateBlock no-notifications empty kind
affects: [20-05 (officer monitors — reuses Toggle + the api.ts core + muteWant), the end-of-phase prod deploy + live browser-smoke]

# Tech tracking
tech-stack:
  added: []  # no new frontend dependency — composes existing components + Lucide icons
  patterns:
    - "one shared Toggle primitive (role=switch button, NOT a checkbox) with a module-block pure onLabel() for node-testability + strict on===true coercion"
    - "epoch-SECONDS relative-time formatter as a module-block pure function (×1000 once) — the P15 epoch-sec crasher guarded by node tests"
    - "a writable unread-count store + refreshUnread() to keep the nav badge authoritative after a mark-read without prop-drilling through AuthGate"

key-files:
  created:
    - web/src/lib/components/Toggle.svelte
    - web/src/lib/components/Toggle.test.ts
    - web/src/lib/components/NotificationPrefsPanel.svelte
    - web/src/lib/components/NotificationInbox.svelte
    - web/src/lib/components/NotificationRow.svelte
    - web/src/lib/components/NotificationRow.test.ts
    - web/src/lib/stores/unread.ts
    - web/src/routes/notifications/+page.svelte
  modified:
    - web/src/lib/api.ts
    - web/src/lib/components/StateBlock.svelte
    - web/src/lib/components/SiteShell.svelte
    - web/src/lib/__tests__/api.test.ts

key-decisions:
  - "Co-located the component tests next to their components (Toggle.test.ts / NotificationRow.test.ts) matching the shipped ConfirmDialog.test.ts convention; the api-wrapper assertions went into the existing src/lib/__tests__/api.test.ts (the established api test file) rather than a new web/src/lib/api.test.ts — same logical file the plan named, no duplicate."
  - "Backed the nav badge with a small writable unread store + refreshUnread() (no existing store pattern in the repo) so the inbox can refresh the badge after a mark-read without prop-drilling — server-truth, never optimistically decremented."
  - "Extracted onLabel (Toggle), deliveryBadge + relativeTime + absoluteTime (NotificationRow) into module blocks as pure functions so the two P15-class crashers (toggle coercion + epoch-seconds time) are covered by node tests despite vitest being DOM-blind."
  - "Browser-smoke DEFERRED (not skipped) to the end-of-phase prod deploy — localhost dev cannot auth against the live backend (cookie Domain=squirebot.quest + apex-only CORS) and the Phase-20 endpoints are not deployed to prod yet; the live DOM smoke runs at deploy, exactly how Phase 19 shipped."

patterns-established:
  - "Toggle: the one new visual primitive — reused verbatim by Plan 05's officer kill-switches"
  - "every prefs change / mark-read / mark-all-read writes server-side then RE-READS (never optimistic); a failed pref save re-reads so the switch never sits in a lying position"
  - "all server-provided alert text/source/detail render via plain {} (Svelte auto-escape) ONLY — zero {@html} on user/wiki data (T-20-17 XSS boundary)"

requirements-completed: [WANT-04]

# Metrics
duration: ~35min
completed: 2026-06-05
---

# Phase 20 Plan 04: /notifications Page (Prefs + Inbox + Unread Badge) Summary

**The guildie-facing /notifications page — server-truth alert preferences (master + 3 per-monitor Toggles, default-ON) stacked over the full alert inbox with word+icon delivery badges, the CAN'T-DM safety-net hint, and the unread-count nav badge — built on one new accessible Toggle primitive + the notifications api.ts block, all node-green (svelte-check 0 errors, 241 vitest, build emits /notifications).**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-06-05T16:08:00Z (approx)
- **Completed:** 2026-06-05T16:15:00Z
- **Tasks:** 2 of 2 code tasks complete; Task 3 (browser-smoke) DEFERRED to the end-of-phase prod-deploy live smoke
- **Files modified:** 12 (8 created, 4 modified)

## Accomplishments
- `/notifications` page: a single `.form-card` stacking `NotificationPrefsPanel` (master + EC/WTS/raid Toggles, default-ON, server-truth save+re-read, master-OFF dims the per-monitor rows) over a divider over `NotificationInbox`.
- The inbox is the 50007 safety net: each `NotificationRow` shows a delivery badge (DELIVERED / CAN'T DM / ERROR — a WORD + icon + tinted pill, color is never the only signal); `dm_blocked` rows carry the actionable "enable server DMs" hint verbatim; unread rows get a 2px accent left-border + a per-row mark-read button.
- The `Toggle` primitive: a `role="switch"` button with a visible ON/OFF word, 44px target, accent-when-on track, `prefers-reduced-motion`, and strict `on === true` coercion (the P15 number-input crasher) — reused by Plan 05's officer kill-switches.
- The Notifications nav link + unread-count badge in `SiteShell` (accent pill, `9+` cap, accessible name `Notifications, N unread`), backed by an `unread` store refreshed on mount/route-change and after every mark-read.
- The notifications api.ts block (6 wrappers) + `muteWant`, all cookie-credentialed via the shared cores with NO owner in any body (D-02); Plan 05 appends its monitor block below.
- Node tests cover the two P15-class crashers: `onLabel` coercion and the epoch-SECONDS `relativeTime` (explicitly asserts no "57 years ago"), plus the `deliveryBadge` mapping and the 7 api-wrapper request shapes (URL/method/credentials/no-owner).

## Task Commits

Each task was committed atomically:

1. **Task 1: api.ts notifications block + Toggle primitive + StateBlock/SiteShell extensions** — `5ba8139` (feat)
2. **Task 2: NotificationPrefsPanel + NotificationInbox + NotificationRow + /notifications page** — `f503f3b` (feat)
3. **Task 3: blocking browser-smoke** — DEFERRED to the end-of-phase prod-deploy live smoke (see Deviations)

**Plan metadata:** _(orchestrator owns the metadata commit + STATE/ROADMAP)_

## Files Created/Modified
- `web/src/lib/components/Toggle.svelte` - the one new accessible on/off primitive (role=switch + ON/OFF word + 44px + reduced-motion + strict bool coercion); module-block `onLabel()` for node tests
- `web/src/lib/components/Toggle.test.ts` - node tests for `onLabel` coercion (P15 guard)
- `web/src/lib/components/NotificationPrefsPanel.svelte` - the Preferences region: master + 3 Toggles, default-ON, save→re-read (never optimistic), master-OFF dim, ships-dark note
- `web/src/lib/components/NotificationInbox.svelte` - the Inbox region: load→list→StateBlock(no-notifications), mark-read/mark-all-read → re-read + refresh nav badge
- `web/src/lib/components/NotificationRow.svelte` - one inbox row: deliveryBadge (word+icon) + epoch-seconds relative time + CAN'T-DM hint + per-row mark-read; alert text via plain {} only
- `web/src/lib/components/NotificationRow.test.ts` - node tests for `deliveryBadge` + the epoch-SECONDS `relativeTime`/`absoluteTime` (P15 guards)
- `web/src/lib/stores/unread.ts` - the unread-count store + `refreshUnread()` (cross-component badge authority)
- `web/src/routes/notifications/+page.svelte` - the page shell stacking PrefsPanel + Inbox in a `.form-card`
- `web/src/lib/api.ts` - the `// --- Notifications (20-04 / WANT-04) ---` wrapper block + `muteWant` (no owner in any body, D-02)
- `web/src/lib/components/StateBlock.svelte` - added the `no-notifications` empty kind (UI-SPEC copy)
- `web/src/lib/components/SiteShell.svelte` - Notifications nav link + unread badge (count in the accessible name)
- `web/src/lib/__tests__/api.test.ts` - the 9 new notification-wrapper request-shape assertions

## Decisions Made
See `key-decisions` in the frontmatter. In short: tests co-located per the shipped ConfirmDialog convention (api-wrapper tests in the existing `__tests__/api.test.ts`); a writable unread store backs the badge (no prior store pattern existed); the P15-class logic was extracted into module-block pure functions for node coverage.

## Deviations from Plan

### Task 3 (browser-smoke): DEFERRED, not skipped

**1. [Documented-constraint deferral] The blocking `checkpoint:human-verify` browser-smoke is consolidated into the end-of-phase prod-deploy live smoke**
- **Found during:** Task 3 (the plan's final blocking checkpoint)
- **Issue:** The localhost dev server cannot authenticate against the live backend (the session cookie is `Domain=squirebot.quest` and CORS allow-origin is the apex only), AND the Phase-20 notification endpoints are not yet deployed to prod — so the live DOM smoke genuinely cannot run now.
- **Resolution:** Per the orchestrator's decision, the live DOM browser-smoke is consolidated into the end-of-phase prod deploy (deploy the Phase-20 backend + frontend, then smoke `/notifications` live on squirebot.quest) — exactly how Phase 19 shipped ("deploy then smoke live"). It is a pending deploy-time HUMAN-UAT, NOT skipped.
- **Node-blind lesson honored:** vitest is DOM-blind (no `@testing-library/svelte`), so node-green is NOT proof the browser works. The two P15-class crashers are pinned by node tests now (Toggle `onLabel` coercion; the epoch-SECONDS `relativeTime`), and the live DOM smoke (toggle word changes, dim-on-master-off, sane relative time, CAN'T-DM hint, badge accessible name, Heavy-theme legibility) WILL run at deploy before the phase is called done.

### Minor

**2. [Test-location alignment] api-wrapper tests added to the existing `src/lib/__tests__/api.test.ts`**
- The plan named `web/src/lib/api.test.ts`; the repo's established api test file is `web/src/lib/__tests__/api.test.ts`. Added the 9 notification-wrapper assertions there (same logical file, no duplicate) and co-located the component tests (Toggle/NotificationRow) per the shipped `ConfirmDialog.test.ts` convention. No behavior change.

---

**Total deviations:** 1 deferral (the blocking browser-smoke → deploy-time live smoke) + 1 minor test-location alignment.
**Impact on plan:** No scope change. All code + node verification complete; the only outstanding item is the deploy-time live DOM smoke, which is gated on the Phase-20 prod deploy.

## Issues Encountered
- **`Duplicate identifier 'AlertLogRow'` (svelte-check):** `NotificationRow.svelte` imported the type in both the module block and the instance block. Removed the instance-block import (the module-block import is in scope for the whole Svelte 5 component). Resolved; svelte-check 0 errors.
- **CAN'T-DM hint grep returned 0:** the verbatim hint copy line-wrapped in the source, splitting the substring across two lines. Put the hint on a single source line (with `<!-- prettier-ignore -->`) so the acceptance grep matches AND the copy stays verbatim. Resolved.

## Verification (node-level — GREEN)
- `cd web && npx vitest run` → **241 passed (19 files)**, incl. the new Toggle coercion + epoch-seconds relative-time + deliveryBadge + 9 notify-wrapper tests.
- `cd web && npm run check` (svelte-check) → **0 errors, 0 warnings**.
- `cd web && npm run build` → **success**; emits `entries/pages/notifications/_page.svelte.js` (the `/notifications` route).
- Acceptance greps verified: 7 notify wrappers; `role="switch"`; `prefers-reduced-motion`; `no-notifications` in StateBlock (≥2) + Inbox (1); `Notifications, ` accessible name in SiteShell; the CAN'T-DM hint line present; **zero actual `{@html}` directives** on user data in Row/Inbox (the only `@html` occurrences are comments forbidding it).

## Pending (deploy-time HUMAN-UAT)
The live DOM browser-smoke of `/notifications` on squirebot.quest, to run at the end-of-phase prod deploy. The deploy-time checklist (from the plan's Task 3):
1. Notifications nav link → page loads with "Alert preferences" (master + 3 Toggles, all ON) + "Your alerts".
2. Master OFF → per-monitor rows dim (readable) + polite announce; persists across reload; toggle back ON.
3. A per-monitor toggle off/on → the ON/OFF **word** changes (not just color), never a blank/garbled `true`/`undefined` state; persists.
4. Keyboard: Tab to a toggle, Space/Enter flips it; visible focus ring.
5. Inbox: empty → "No alerts yet"; with rows → DELIVERED/CAN'T DM/ERROR as word+icon; the timestamp reads as a sane relative time (NOT "57 years ago"); a CAN'T-DM row shows the enable-server-DMs hint; the unread badge shows + "Mark all read" clears it; the badge's accessible name reads "Notifications, N unread".
6. Heavy parchment theme: the CAN'T-DM red word stays legible.

## Next Phase Readiness
- **Plan 05 (officer monitors)** is ready: the `Toggle` primitive, the api.ts notifications core, `muteWant`, and the `unread` store are all in place. Plan 05 appends its `// --- Monitors (20-05 / WANT-08) ---` api block BELOW this plan's block (different wave — no overwrite) and adds the `WantlistRow.muted` field + the WantMuteCell.
- **Blocker for "phase done":** the deploy-time live browser-smoke above (gated on the Phase-20 prod deploy of the backend endpoints + this frontend).

## Self-Check: PASSED

- All 8 created code files + the SUMMARY verified present on disk.
- Both task commits (`5ba8139`, `f503f3b`) verified in git history.
- Node verification GREEN: 241 vitest passed, svelte-check 0 errors, build emits `/notifications`.

---
*Phase: 20-bot-dm-notification-infrastructure*
*Completed: 2026-06-05*
