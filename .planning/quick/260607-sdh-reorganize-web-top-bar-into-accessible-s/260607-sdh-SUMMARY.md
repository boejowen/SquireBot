---
phase: quick-260607-sdh
plan: 01
subsystem: web (SvelteKit app chrome)
tags: [ui, a11y, ia-cleanup, web, svelte5]
requires:
  - web/src/lib/auth.ts (Session, logout)
  - web/src/lib/components/ThemePicker.svelte
  - web/src/lib/components/AuthGate.svelte (SESSION_KEY context)
provides:
  - web/src/lib/components/SettingsMenu.svelte (accessible gear dropdown)
affects:
  - web/src/lib/components/SiteShell.svelte (decluttered header)
  - web/src/routes/+layout.svelte (comment only)
tech-stack:
  added: []
  patterns:
    - "Pure DOM-free helpers exported from <script module> for node-only vitest (ConfirmDialog idiom)"
    - "svelte:window keydown for Escape-to-close (avoids a static-element a11y warning)"
key-files:
  created:
    - web/src/lib/components/SettingsMenu.svelte
    - web/src/lib/components/SettingsMenu.test.ts
  modified:
    - web/src/lib/components/SiteShell.svelte
    - web/src/lib/components/AuthGate.svelte
    - web/src/routes/+layout.svelte
    - web/src/lib/__tests__/charmeta.test.ts
  deleted:
    - web/src/lib/components/SessionIndicator.svelte
decisions:
  - "SessionIndicator deleted (SiteShell was its only real importer; AuthGate/+layout mentions were comment-only)"
  - "Escape-to-close moved to <svelte:window> (menuKeyAction no-ops while closed) to dodge the static-div keydown a11y warning"
  - "charmeta.test.ts D-03 contract re-pointed from SiteShell to SettingsMenu (the links' new home)"
metrics:
  duration: ~35m
  tasks: 2
  files-created: 2
  files-modified: 4
  files-deleted: 1
  completed: 2026-06-07
---

# Quick 260607-sdh: Reorganize Web Top Bar Into Accessible Settings Dropdown Summary

Folded the web header's identity, theme picker, Account/Character-details/Admin
links, and Sign out into a new accessible `<SettingsMenu>` gear dropdown, leaving
the top bar with only the wordmark + Wantlist + Notifications(+unread badge) +
the gear — a LOCKED IA cleanup, web/-only, not deployed.

## What Shipped

### Task 1 — `SettingsMenu.svelte` + node-only test (commit 88edac7)

A Svelte 5 (`$props`/`$state`/`$derived`/`$effect`/`$bindable`) accessible
dropdown opened from a gear trigger:

- **Trigger:** a `<button>` with `aria-haspopup="menu"`, `aria-expanded={open}`,
  `aria-controls="settings-menu-panel"`, `aria-label="Settings"`, bound via
  `bind:this={triggerEl}`.
- **Panel** (`id="settings-menu-panel"`, `role="menu"`), top→bottom: identity
  header (avatar via `avatarUrlFor` or a `UserIcon` fallback + officer `Shield` +
  username), `<ThemePicker bind:theme />`, "Watcher codes" (`/account`),
  "Character details" (`/char-meta`), officer-only "Admin" (`/admin`) under
  `{#if session?.isOfficer}`, a divider, and "Sign out".
- **A11y/behavior:** Escape closes + returns focus to the trigger (via a
  `<svelte:window onkeydown>` that calls the pure `menuKeyAction`); outside
  `pointerdown` closes; a route change closes (`$page` path-diff); the first menu
  item is focused on open (SSR-guarded `tick()`).
- **Security (T-15-22 / T-sdh-01):** username renders via plain `{}` only; the
  avatar `alt` is the same escaped username; `{@html` never appears.
- **Pure helpers** exported from `<script module>` for the node test:
  `menuKeyAction(key, open)` and `avatarUrlFor(session)`.
- **Test (`SettingsMenu.test.ts`, 19 tests):** asserts the `menuKeyAction` /
  `avatarUrlFor` logic and the source a11y/security contract (haspopup/expanded/
  controls, `role="menu"`, the officer-gated `/admin`, the sign-out flow, no
  `{@html`, the trigger-focus restore, the `bind:theme` passthrough, outside-click
  + route-change close).

### Task 2 — Slim SiteShell; retire SessionIndicator (commit bdde646)

- `SiteShell.svelte` now imports/renders `<SettingsMenu bind:theme {session} />`
  in `.shell-controls` after the Wantlist + Notifications links, under the single
  `{#if session?.authenticated}` guard. Removed the inline Admin button +
  `goAdmin()`, the standalone `<ThemePicker>`, the `/account` + `/char-meta`
  links, and `<SessionIndicator>`. The `+layout → SiteShell → SettingsMenu →
  ThemePicker` `bind:theme` chain is intact. Dropped the now-dead `.admin-nav`
  CSS; kept `.char-meta-nav` / `.notify-nav` / `.unread-badge` (still used by the
  surviving links).
- **SessionIndicator.svelte deleted** — SiteShell was its only real importer; the
  remaining mentions in `AuthGate.svelte` + `+layout.svelte` were comment-only and
  were scrubbed to reference the SettingsMenu instead.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Duplicate `Session` import broke `npm run check`**
- **Found during:** Task 2 verify gate (`npm run check`).
- **Issue:** `Session` was imported in both the `<script module>` and the instance
  `<script>` of SettingsMenu → "Duplicate identifier 'Session'" (×2).
- **Fix:** Dropped `type Session` from the instance import (the module-script
  import is in scope for the instance script in Svelte 5).
- **Files modified:** `web/src/lib/components/SettingsMenu.svelte`
- **Commit:** bdde646

**2. [Rule 1 - Bug] a11y warning: keydown on a static, role-less `<div>`**
- **Found during:** Task 1 (first vitest run surfaced the svelte plugin warning;
  `npm run check` would have flagged it).
- **Issue:** the root `<div onkeydown>` tripped `a11y_no_static_element_interactions`.
- **Fix:** moved Escape handling to `<svelte:window onkeydown={onKeydown} />`
  (`menuKeyAction` already no-ops while closed), so no role-less interactive
  element remains. Committed within Task 1 (88edac7).

**3. [Rule 1 - Bug] Relocating `/char-meta` broke an existing source-contract test**
- **Found during:** Task 2 verify gate (`npm test`).
- **Issue:** `charmeta.test.ts` asserted the D-03 member-accessible nav contract by
  reading `SiteShell.svelte` for `/char-meta`; the link moved into SettingsMenu, so
  the assertion sliced an empty guard and failed.
- **Fix:** re-pointed the `describe` block at `SettingsMenu.svelte` (the link's new
  home): it now asserts (a) SiteShell renders `<SettingsMenu>` only under
  `{#if session?.authenticated}` and SiteShell no longer gates anything on
  `isOfficer`, and (b) inside SettingsMenu `/char-meta` sits before the officer gate
  that wraps `/admin` (so it is never trapped behind `isOfficer`). The D-03 contract
  is preserved, just asserted at the new location.
- **Files modified:** `web/src/lib/__tests__/charmeta.test.ts`
- **Commit:** bdde646

## Verify Gate (web/)

| Step            | Result |
| --------------- | ------ |
| `npm run check` | PASS — 476 files, 0 errors, 0 warnings |
| `npm run build` | PASS — vite build + adapter-static "✔ done" |
| `npm test`      | PASS — 21 files, 281 tests (incl. SettingsMenu's 19) |

## Known Stubs

None. All links route to existing pages (`/account`, `/char-meta`, `/admin`,
`/wantlist`, `/notifications`); the session is the same object already in the
shell (no new fetch, no placeholder data).

## Browser-Smoke Gap (carried from the plan's verification)

The web/ vitest suite is node-only and DOM-blind (no jsdom / @testing-library —
toolchain-install rule). The menu's live behaviors — open/close on click, Escape
returning focus to the gear, outside-click close, route-change close, first-item
focus on open, and a theme swap re-theming the whole app through the
`bind:theme` chain — are covered only via the extractable helpers + the source
contract, NOT a mounted DOM. **Flag for a later browser smoke** (deploy-then-smoke
or a full local stack, per the project's "web tests are DOM-blind" memory).

## Threat Flags

None. No new network surface, auth path, or schema change. T-sdh-02 (officer
Admin item) remains Layer-1 UX suppression only — the server re-checks officer on
every `/admin` endpoint (15-03); the hidden nav is never the boundary.

## Self-Check: PASSED

- FOUND: web/src/lib/components/SettingsMenu.svelte
- FOUND: web/src/lib/components/SettingsMenu.test.ts
- FOUND (deleted as planned): web/src/lib/components/SessionIndicator.svelte — absent from the tree, `delete mode` recorded in bdde646
- FOUND commit: 88edac7 (feat — SettingsMenu + test)
- FOUND commit: bdde646 (refactor — SiteShell slim + SessionIndicator retire)
- No `.planning/` files committed; working tree clean except this untracked SUMMARY.
