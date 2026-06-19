---
phase: 30-app-shell-5-tab-navigation
plan: 02
subsystem: ui
tags: [sveltekit, svelte5, routing, settings, wishlist, notifications, theme-context, officer-gate, web]

# Dependency graph
requires:
  - phase: 30-app-shell-5-tab-navigation
    plan: 01
    provides: "the routing spine — 5-tab strip, THEME_KEY context, /wishlist + /settings as redirect targets, the relocated Wishlist-tab unread badge"
  - phase: 20-bot-dm-notification-infrastructure
    provides: "NotificationPrefsPanel + NotificationInbox + the unread store the Wishlist tab composes/reads"
  - phase: 19-wantlist-crud
    provides: "WantlistPanel (the rehomed wantlist surface + its own filter = the NAV-02 scoped search)"
provides:
  - "/wishlist route body — WantlistPanel (intro snippet) + a Notifications region (NotificationPrefsPanel + divider + NotificationInbox); the wantlist filter is the NAV-02 scoped search (NAV-04 content)"
  - "/settings route body — 6 existing panels composed as in-page id'd sections (settings-theme/-watcher-codes/-class-level/-my-characters/-admin) behind an officer-gated Admin section, with a live settings search (NAV-03)"
  - "SettingsThemePicker — the context→bindable bridge wrapping ThemePicker so the relocated picker mutates the single theme state via THEME_KEY (no second [data-theme] writer — D-06)"
  - "settings.test.ts — source-grep guards pinning the Settings IA, the section-level officer gate, the SESSION_KEY two-layer pattern, the no-{@html} guard, and the single-writer theme bridge"
affects: [34-wishlist-rework]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Context→bindable bridge: a thin wrapper reads a get/set context accessor (THEME_KEY) into a local $state and write-throughs via $effect to a $bindable child prop — reaches a {@render children()} page that can't be bound directly"
    - "Live in-page section filter: a fixed per-section keyword list + a case-insensitive substring predicate gates each section's {#if}; empty query shows all; a dimmed no-match line replaces blanking"
    - "Section-level officer gate: {#if isOfficer} (from SESSION_KEY context) wraps the whole Admin <section> — Layer-1 UX over the unchanged Go server boundary"

key-files:
  created:
    - web/src/routes/wishlist/+page.svelte
    - web/src/routes/settings/+page.svelte
    - web/src/lib/components/SettingsThemePicker.svelte
    - web/src/lib/__tests__/settings.test.ts
  modified: []

key-decisions:
  - "The wantlist's own filter/add bar IS the Wishlist tab's NAV-02 scoped search for Phase 30 — no separate search box added (full WISH-07 cross-wishlist search is Phase 34)"
  - "The officer-gated Admin section wraps the {#if matches('settings-admin')} search predicate INSIDE {#if isOfficer} so a non-officer's Admin keywords never reveal the section (T-30-08) and the gate stays the outer guard"
  - "The settings.test.ts gate-ordering assertion matches the `<Panel` mount form (not the bare name) because the bare panel names also appear in the top-of-file imports BEFORE the gate"

patterns-established:
  - "Context→bindable bridge (SettingsThemePicker) — the canonical way to relocate a $bindable control into a {@render children()} page without a second writer"
  - "Live in-page section-filter search (keyword list + substring predicate + dimmed no-match copy) — the NAV-02 'scoped search on a tab with content' pattern, reusable by Phases 31/32/33"

requirements-completed: [NAV-02, NAV-03, NAV-04]

# Metrics
duration: ~20min
completed: 2026-06-18
---

# Phase 30 Plan 02: Wishlist + Settings Tab Bodies Summary

**The two functional tab bodies on Plan 01's spine: the Wishlist tab (rehomed wantlist + notifications inbox/prefs, the wantlist filter serving as the NAV-02 scoped search), and the Settings tab (the 6 existing panels composed as in-page id'd sections behind an officer-gated Admin section, with a live settings search and the theme picker relocated via the Plan-01 THEME_KEY context bridge) — code-complete + all node gates green, the load-bearing deployed browser-smoke (Task 4) PENDING a human.**

## Status: CODE-COMPLETE, CHECKPOINT PENDING

Tasks 1–3 (all `type=auto`) are executed, committed atomically, and pass every node gate. **Task 4 (`checkpoint:human-verify`, gate="blocking") is NOT done** — it is a browser-smoke of the full 5-tab rework on a DEPLOYED build, which cannot be performed by the executor (nothing is deployed; node vitest is DOM-blind; localhost can't auth against prod). It awaits a human. See "Pending Verification" below.

## Performance

- **Duration:** ~20 min (Tasks 1–3)
- **Tasks:** 3 of 4 executed (`type=auto`); Task 4 is the blocking human-verify checkpoint, deferred to a human
- **Files created:** 4 (all under `web/`)

## Accomplishments

- **NAV-04 (content):** `/wishlist` composes the rehomed wantlist surface FIRST (`WantlistPanel` with the intro passed as a snippet child — the old `/wantlist` idiom so the page's scoped styles apply and grids break full-width) THEN a "Notifications" region (`NotificationPrefsPanel` + a `divider` + `NotificationInbox`) in a 720px `.form-card`. The Wishlist-tab unread badge (Plan 01) stays authoritative with NO extra wiring — `NotificationInbox` already refreshes the count after a mark-read. No panel rewritten; no unread-store import/mutation on the page.
- **NAV-02 (pattern-established):** the `WantlistPanel`'s own filter/add bar is the Wishlist tab's scoped search this phase (full WISH-07 cross-wishlist search is Phase 34). The Settings tab has a real live section-filter search. The two tabs that have content this phase get functional scoped search; the stub tabs get theirs with their content in 31/32/33.
- **NAV-03 (D-05/D-06):** `/settings` is ONE page composing the 4 member panels (`SettingsThemePicker`, `WatcherCodesPanel`, `CharMetaForm`, `MyCharactersPanel`) + the officer-gated 4 admin panels (`EvictionForm`, `AdminMgmtForm`, `MonitorAdminPanel`, `AssignmentAdminPanel`) as in-page `<section>`s with stable ids (`settings-theme`, `settings-watcher-codes`, `settings-class-level`, `settings-my-characters`, `settings-admin`). A live settings search filters which sections are visible (per-section keyword list + case-insensitive substring); empty query shows all; nothing-matches shows a dimmed `No settings match "{query}".` line (query auto-escaped via `{}`), never a blank page.
- **D-06 (theme bridge):** `SettingsThemePicker` reads `getContext<ThemeContext>(THEME_KEY)`, seeds a local `$state` from `themeCtx.get()`, and write-throughs via `$effect` to `themeCtx.set(theme)`, binding that local state to the unchanged `<ThemePicker bind:theme>`. The bridge contains NO theme-apply call — the single `$effect(applyTheme)` in `+layout` stays the ONE `[data-theme]` writer (Pitfall 3 / T-30-07).
- **Officer gate (T-30-05):** `{#if isOfficer}` (derived from the verbatim `SESSION_KEY` context idiom) wraps the entire Admin `<section>`. A non-officer never renders it; the Go API re-checks officer on every admin write — the hidden section is Layer-1 UX, never the boundary. `MonitorAdminPanel` stays in Settings→Admin (officer config), NOT on Wishlist (A4).

## Task Commits

Each task was committed atomically on `master`:

1. **Task 1: /wishlist — rehome wantlist + notifications inbox/prefs (D-03/D-07, NAV-04)** — `0e4aaac` (feat)
2. **Task 2: /settings — 6 panels as sections + officer-gated Admin + theme bridge + search (D-05/D-06, NAV-03)** — `e9f71a3` (feat)
3. **Task 3: settings.test.ts — pin the Settings IA + section-level officer gate (NAV-03)** — `8a5ba58` (test)

## Files Created

- `web/src/routes/wishlist/+page.svelte` — `WantlistPanel` (intro snippet child) + a "Notifications" `.form-card` region (`NotificationPrefsPanel` + divider + `NotificationInbox`); reuses the `.form-card`/`.form-title`/`.form-purpose`/`.divider` rhythm; no new search box, no `{@html}`, no unread-store reference.
- `web/src/routes/settings/+page.svelte` — the 6-section Settings page; the verbatim `SESSION_KEY` officer-gate idiom; the live settings search (keyword list + substring predicate + dimmed no-match copy); five stable section ids; UI-SPEC §D search-input visual (full-width, panel fill, 44px, accent focus-visible).
- `web/src/lib/components/SettingsThemePicker.svelte` — the context→bindable bridge wrapping `ThemePicker`; `getContext<ThemeContext>(THEME_KEY)` + `$effect` write-through; no second `[data-theme]` writer.
- `web/src/lib/__tests__/settings.test.ts` — 8 source-grep tests pinning the new IA: member panels mounted, five ids, search + empty-match copy, the `<Panel`-after-`{#if isOfficer}` ordering, the `SESSION_KEY`/`getContext` two-layer pattern, the no-`{@html}` guard, and the `getContext<ThemeContext>(THEME_KEY)`-but-no-`applyTheme` bridge invariant.

## Decisions Made

- **The wantlist filter is the NAV-02 scoped search; no separate box.** Phase 30 only rehomes the existing wantlist surface (the full WISH-07 cross-wishlist search is Phase 34). Adding a redundant box would be the worse experience (D-08) and double-search the same data.
- **Admin search predicate nested inside the officer gate.** `{#if isOfficer}` is the OUTER guard and `{#if matches('settings-admin')}` is inner — so a non-officer's "admin" query matches nothing (T-30-08, no officer-surface disclosure) and the section is never reachable for them regardless of the search.
- **Gate-ordering assertion matches `<Panel`, not the bare name.** The four admin panel names appear FIRST in the top-of-file `import` statements (before the gate), so an `indexOf('AssignmentAdminPanel')`-after-gate check would fail on the import. Asserting against the `<AssignmentAdminPanel` mount form (markup-only) is the correct, robust guard (see Deviations).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] settings.test.ts gate-ordering assertion matched the import, not the mount**
- **Found during:** Task 3 (running `npm test`).
- **Issue:** The plan's acceptance text suggested `source.indexOf('{#if isOfficer}') < source.indexOf('AssignmentAdminPanel')`. But `AssignmentAdminPanel` (and the other three admin panel names) first appear in the top-of-file `import` statements — BEFORE the `{#if isOfficer}` gate. So the literal-name `indexOf` resolved to the import (646), failing the `> gateIdx` (821) assertion even though the *mounts* are correctly inside the gate. 1 test failing.
- **Fix:** Changed the assertion to match the `<Panel` mount form (`<EvictionForm`, `<AdminMgmtForm`, `<MonitorAdminPanel`, `<AssignmentAdminPanel`) — the `<` only appears in the markup mount, not the import, so it correctly pins "the panel MOUNTS sit after the gate." Same intent the plan specified; the `<`-prefix makes it robust.
- **Files modified:** `web/src/lib/__tests__/settings.test.ts`
- **Verification:** `npm test` 317/317 green (was 1 failing).
- **Committed in:** `8a5ba58` (Task 3 commit).

### Comment-wording adjustments (grep-faithful acceptance, no behavior change)

Two acceptance greps are literal (`grep unread = 0` on `/wishlist`; bridge `does NOT contain applyTheme`). My initial explanatory comments contained the bare tokens `unread` and `applyTheme` (in prose). I reworded those comments to avoid the literal tokens while preserving the guidance — same approach Plan 01 documented under "Issues Encountered." No behavioral change; the code never imports the unread store on the page nor calls a theme-apply helper in the bridge.

---

**Total deviations:** 1 auto-fixed (1 bug — test-precision fix) + 2 comment rewordings.
**Impact on plan:** None on scope or behavior. The bug fix tightened a source-grep guard to its intended meaning; the rewordings keep the literal acceptance greps satisfiable. No security assertion weakened.

## Known Stubs

None introduced by this plan. The `/wishlist` and `/settings` bodies are fully functional (they mount the existing, working panels 1:1). The three coming-soon placeholder tabs (Characters/Inventory/Banks) are Plan 01's intentional, planned stubs (D-03) — unchanged here.

## Threat Flags

None. No new security surface:
- No new server endpoint, auth path, file access, or schema change — this is web-only composition of existing panels.
- The officer gate (T-30-05) is preserved as Layer-1 UX over the unchanged Go boundary; the hidden section is never trusted (asserted in `settings.test.ts`).
- The settings-search echo (`No settings match "{query}"`) renders `{query}` via plain `{}` (auto-escaped); `grep '{@html'` = 0 on both new route bodies and the bridge (T-30-06).
- The single `[data-theme]` writer stays in `+layout`; `SettingsThemePicker` contains no theme-apply call (T-30-07, asserted).

## Pending Verification (Task 4 — checkpoint:human-verify, gate="blocking")

**Task 4 is NOT done — it is a blocking human-verify checkpoint and awaits a human in a real browser on a DEPLOYED build.** The executor cannot perform it: nothing is deployed, node vitest is DOM-blind (`web-tests-node-only-blind-to-dom` — green tests do NOT prove the tabs render, the redirects fire, the badge tracks `$unreadCount`, the officer gate hides, or the relocated theme picker writes `[data-theme]`), and `localhost can't auth against prod` (cookie Domain=squirebot.quest + apex-only CORS).

**To verify:** deploy the web build (per `docs/backend-deploy.md` web-only atomic-swap) OR run a full local stack (local backend + `SQUIREBOT_COOKIE_INSECURE` + `PUBLIC_API_BASE` + a seeded `sb_session`), then signed in as a member (and re-check the gated items as an officer):

1. All 5 tabs render in order (Characters · Inventory · Banks · Wishlist · Settings), reachable from every route; the current tab shows the accent underline + exactly one tab has `aria-current="page"`.
2. Hard-refresh `https://squirebot.quest/` — lands on the preserved guild-views surface (NOT a 404, NOT a Characters stub). If the apex 404s, the `/` prerender drop needs a minimal prerendered redirect (RESEARCH A2) — report it.
3. Each old bookmark moves the address bar with no 404: `/wantlist`→`/wishlist`, `/notifications`→`/wishlist`, `/account`/`/char-meta`/`/my-characters`/`/admin`/`/bank-coin`→`/settings`.
4. Characters/Inventory/Banks each show the "coming soon" placeholder with a working link to `/guild-views`; none has a search bar.
5. Wishlist tab: the wantlist add/filter works, the Notifications inbox + prefs render below it, and the unread badge on the Wishlist tab reflects `$unreadCount` (read an alert → badge updates).
6. Settings tab: all member sections render; type "watcher" in the settings search → only the Watcher Codes section shows; clear it → all return; the Admin section is HIDDEN as a non-officer and VISIBLE as an officer; the bank-coin/`?view=bank` round-trip from `/guild-views` still works.
7. Theme: switch the theme from the Settings Theme section → the `<html>`/.theme-root `[data-theme]` attribute changes and the whole shell re-themes; switch among all 5 themes and confirm the tab strip + active underline + badge contrast hold (especially Heavy + Minimalist).
8. Identity menu (top-right): opens, Escape closes + restores focus, Sign out works; the username renders as inert text (no HTML injection).

**Resume signal:** the human types "approved" or lists the issues found. Do NOT mark Phase 30 verified until all 8 items pass on the deployed build.

## Verification (node gates — all green)

- `cd web && npm run check` → **0 errors / 0 warnings** (498 files).
- `cd web && npm test` → **317/317 passing** (was 309 in Plan 01; +8 in `settings.test.ts`).
- `cd web && npm run build` → **succeeds** (SPA bundle emitted; adapter-static wrote `build/`).
- **Deployed browser-smoke (Task 4) → PENDING (awaiting human).** This is the load-bearing verification; the node gates do not substitute for it.

## Next Phase Readiness

- **The NAV-02 scoped-search pattern is established** (live section filter on Settings; the WantlistPanel filter on Wishlist) — Phases 31/32/33 add their per-tab search with their content.
- **Phase 34 (Wishlist rework)** owns the per-character/per-slot upgrade-target rework + WISH-07 cross-wishlist search; Phase 30 only rehomed today's wantlist surface.
- **Phase 30 is code-complete pending the Task-4 deployed browser-smoke** — once a human confirms all 8 items, Phase 30 closes.

## Self-Check: PASSED

All 4 created files verified present on disk (`web/src/routes/wishlist/+page.svelte`, `web/src/routes/settings/+page.svelte`, `web/src/lib/components/SettingsThemePicker.svelte`, `web/src/lib/__tests__/settings.test.ts`); all 3 task commits (`0e4aaac`, `e9f71a3`, `8a5ba58`) verified in git history. Node gates: `npm run check` 0 errors / 0 warnings (498 files); `npm test` 317/317 passing; `npm run build` succeeds. All changes confined to `web/` — no `internal/`, Go, or `.sql` files touched. **Caveat:** the load-bearing Task-4 deployed browser-smoke is PENDING (awaiting a human) — the node gates do not prove the routes render/redirect/badge/gate/theme in a browser.

---
*Phase: 30-app-shell-5-tab-navigation*
*Plan 02 code-complete: 2026-06-18 — Task 4 browser-smoke awaiting human verification.*
