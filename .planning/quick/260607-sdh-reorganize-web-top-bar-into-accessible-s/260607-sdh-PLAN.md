---
phase: quick-260607-sdh
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/lib/components/SettingsMenu.svelte
  - web/src/lib/components/SettingsMenu.test.ts
  - web/src/lib/components/SiteShell.svelte
  - web/src/lib/components/SessionIndicator.svelte
autonomous: true
requirements: [QUICK-260607-sdh]
commit_docs: false

must_haves:
  truths:
    - "The header shows wordmark + Wantlist + Notifications (with unread badge) + a gear settings button — nothing else"
    - "Clicking the gear opens a role=menu dropdown with identity header, theme picker, Watcher-codes link, Character-details link, officer-only Admin link, divider, Sign out"
    - "A non-officer never sees the Admin menu item; an officer does"
    - "Escape closes the menu and returns focus to the gear trigger; outside-click and route change also close it"
    - "Changing the theme in the menu still re-themes the whole app (bind:theme chain intact)"
    - "npm run check, npm run build, and npm test all pass in web/"
  artifacts:
    - path: "web/src/lib/components/SettingsMenu.svelte"
      provides: "Accessible settings dropdown absorbing identity, theme, account/char-meta/admin links, sign out"
      min_lines: 120
    - path: "web/src/lib/components/SettingsMenu.test.ts"
      provides: "Node-only vitest over SettingsMenu's exported logic + source a11y contract"
    - path: "web/src/lib/components/SiteShell.svelte"
      provides: "Decluttered header rendering SettingsMenu instead of inline controls"
  key_links:
    - from: "web/src/routes/+layout.svelte"
      to: "web/src/lib/components/SiteShell.svelte"
      via: "<SiteShell bind:theme>"
      pattern: "SiteShell bind:theme"
    - from: "web/src/lib/components/SiteShell.svelte"
      to: "web/src/lib/components/SettingsMenu.svelte"
      via: "<SettingsMenu bind:theme {session} />"
      pattern: "SettingsMenu bind:theme"
    - from: "web/src/lib/components/SettingsMenu.svelte"
      to: "web/src/lib/components/ThemePicker.svelte"
      via: "<ThemePicker bind:theme />"
      pattern: "ThemePicker bind:theme"
    - from: "web/src/lib/components/SettingsMenu.svelte"
      to: "$lib/auth"
      via: "logout() on Sign out"
      pattern: "logout\\("
---

<objective>
Reorganize the web app top bar into an accessible header "settings" dropdown (gear trigger),
decluttering the primary nav. The header keeps only wordmark + Wantlist + Notifications (with
its unread badge); everything else — identity, theme picker, Account/Character-details/Admin
links, and Sign out — folds into a new <SettingsMenu> opened from a gear icon.

This is a LOCKED IA decision (already settled with the user — do NOT propose a /settings page or
re-litigate which items move). The change is web/ UI-only cleanup. Do NOT deploy.

Purpose: Reduce header clutter; group account/admin/theme/identity affordances under one
discoverable, keyboard-accessible menu that matches the project's high a11y bar (the
SiteShell/SessionIndicator/ConfirmDialog idioms).

Output: A new SettingsMenu.svelte + node-only test, a slimmed SiteShell.svelte, and (if
SiteShell was its only importer) a deleted SessionIndicator.svelte.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md

IMPORTANT — commit_docs is FALSE for this project:
- Commit CODE changes only (web/src/**). Do NOT git-add or commit anything under .planning/
  (PLAN/SUMMARY/STATE are handled by the orchestrator, not the executor).
- Do NOT deploy. The verify gate is local-only: npm run check / build / test in web/.
</execution_context>

<context>
@.planning/STATE.md
@./CLAUDE.md

<interfaces>
<!-- Contracts the executor needs — extracted from the codebase. No exploration required. -->

From $lib/auth (web/src/lib/auth.ts):
```typescript
export interface Session {
  authenticated: boolean;
  isMember: boolean;
  isOfficer: boolean;
  username: string;       // user-controlled (Discord) — render via {} ONLY, never {@html}
  avatar: string | null;  // null → glyph fallback
  discordUserId: string;
}
export async function logout(fetchFn?: typeof fetch): Promise<void>; // POST logout, swallows errors
```

ThemePicker (web/src/lib/components/ThemePicker.svelte) props:
```typescript
let { theme = $bindable() }: { theme: ThemeKey } = $props();
// import type { ThemeKey } from '$lib/theme/themes';
```

Theme ownership chain (NO context, NO store):
  +layout.svelte:  let theme = $state<ThemeKey>(loadTheme());  →  <SiteShell bind:theme>
  SiteShell.svelte: let { theme = $bindable(), children } = $props();  →  <SettingsMenu bind:theme {session} />
  SettingsMenu.svelte: let { theme = $bindable(), session } = $props();  →  <ThemePicker bind:theme />
This chain MUST stay intact. +layout.svelte's $effect(applyTheme) is the single [data-theme] writer.

Lucide icons available (@lucide/svelte 1.17.0) — import path style already used in SessionIndicator:
  import UserIcon from '@lucide/svelte/icons/user';
  import Shield from '@lucide/svelte/icons/shield';
  import LogOut from '@lucide/svelte/icons/log-out';
  // gear trigger: import Settings from '@lucide/svelte/icons/settings';  (verify the glyph exists;
  //   'cog' is the fallback name if 'settings' isn't present)

vitest (web/vite.config.ts): node-only project; includes src/**/*.test.ts; EXCLUDES *.svelte.test.ts.
  expect.requireAssertions = true → every `it` MUST contain at least one expect().
  Established test idiom: export pure helpers from the component's `<script module>` and/or read the
  .svelte source as a string and assert on it (see ConfirmDialog.test.ts — the closest analog).
  NO @testing-library/svelte, NO jsdom (toolchain-install rule forbids adding them).
</interfaces>

<!-- The two files you will MODIFY (read them fresh before editing): -->
@web/src/lib/components/SiteShell.svelte
@web/src/lib/components/SessionIndicator.svelte
@web/src/lib/components/ConfirmDialog.test.ts
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create SettingsMenu.svelte (accessible gear dropdown) + node-only test</name>
  <files>web/src/lib/components/SettingsMenu.svelte, web/src/lib/components/SettingsMenu.test.ts</files>
  <behavior>
    Pure exported logic in `<script module>` (so the node test can import it; pattern mirrors
    ConfirmDialog/Toggle/MonitorAdminPanel helper-export style):
    - `menuKeyAction(key: string, open: boolean): 'close' | 'ignore'` — returns 'close' for
      'Escape' when open, else 'ignore'. (Pins the Escape-to-close + closed-state no-op contract.)
    - `avatarUrlFor(session)` helper returning the CDN URL string or null
      (`https://cdn.discordapp.com/avatars/${discordUserId}/${avatar}.png` only when BOTH present),
      so the avatar derivation is unit-testable away from the DOM.
    Test (SettingsMenu.test.ts), node-only, every `it` asserts:
    - menuKeyAction: Escape+open → 'close'; Escape+closed → 'ignore'; 'Enter'/'Tab'+open → 'ignore'.
    - avatarUrlFor: full URL when id+avatar present; null when avatar null; null when id empty.
    - Source-string a11y/security contract (readFileSync the .svelte, like ConfirmDialog.test.ts):
        * gear trigger has aria-haspopup="menu", aria-expanded bound to open, aria-controls.
        * the panel is role="menu".
        * officer gate present: `{#if session?.isOfficer}` guards the /admin item.
        * Sign out calls logout() then sets window.location.href = '/'.
        * username renders via plain {} (assert `{session.username}` appears) and {@html} does NOT
          appear anywhere (T-15-22): `expect(SOURCE).not.toContain('{@html')`.
        * focus restored to trigger on close (assert the trigger ref + `.focus()` are wired).
  </behavior>
  <action>
    Create web/src/lib/components/SettingsMenu.svelte using Svelte 5 runes
    ($props/$state/$derived/$effect/$bindable), modeled on SessionIndicator (identity + sign-out
    logic) and ConfirmDialog (Escape + focus-restore + outside-click idioms). Theme tokens ONLY
    (var(--accent)/--panel/--text/--bg/--border, --font-display/--font-body); 44px touch targets;
    focus-visible outline 2px solid var(--accent).

    Props: `let { theme = $bindable(), session }: { theme: ThemeKey; session: Session } = $props();`

    `<script module>` exports the pure helpers above (menuKeyAction, avatarUrlFor). Import Session
    type from '$lib/auth'; ThemeKey from '$lib/theme/themes'.

    Component body:
    - `let open = $state(false);` `let triggerEl = $state<HTMLButtonElement>();`
      `let panelEl = $state<HTMLElement>();`
    - Gear trigger <button>: import Settings from '@lucide/svelte/icons/settings' (verify the glyph;
      use 'cog' if 'settings' is absent). aria-haspopup="menu", aria-expanded={open},
      aria-controls="settings-menu-panel", aria-label="Settings". onclick toggles open.
      bind:this={triggerEl}.
    - Panel <div id="settings-menu-panel" role="menu"> rendered when open, top→bottom:
        1. Identity header: avatar <img> when avatarUrlFor(session) (alt={session.username},
           28x28) else a UserIcon fallback span (aria-hidden); officer Shield (size 14,
           aria-label="Officer") before the name when session?.isOfficer; <span>{session.username}</span>.
           ABSORB this verbatim-equivalent from SessionIndicator, KEEPING the T-15-22 note in a
           comment (username via {} only; alt = escaped username).
        2. <ThemePicker bind:theme /> (import from './ThemePicker.svelte').
        3. <a href="/account" role="menuitem">Watcher codes</a>  (relabeled — was "Account").
        4. <a href="/char-meta" role="menuitem">Character details</a>.
        5. {#if session?.isOfficer}<a href="/admin" role="menuitem">Admin</a>{/if}.
        6. A divider (<hr> or a styled separator, aria-hidden).
        7. <button role="menuitem" onclick={signOut} disabled={signingOut}>Sign out</button> —
           reuse the SessionIndicator signingOut guard: `if (signingOut) return; signingOut = true;
           await logout(); window.location.href = '/';`.
    - Behavior/a11y:
        * Toggle open/close on trigger click.
        * onkeydown on the root (or panel): if menuKeyAction(e.key, open) === 'close', set open=false
          and triggerEl?.focus() (Escape closes + RETURNS FOCUS to trigger).
        * Outside-click: an $effect that, while open, adds a document 'pointerdown' listener closing
          the menu when the event target is outside the root element; clean up on teardown.
        * Close on route change: subscribe to `page` from '$app/stores' in an $effect; when the path
          changes while open, set open=false. (Match SiteShell's lastPath idiom.)
        * On open, focus the first menu item (an $effect keyed on `open` that focuses the first
          [role="menuitem"] or the panel; guard for SSR — only in browser/after mount).
    - Do NOT introduce context or a store for theme — pass bind:theme straight through to ThemePicker.

    Then create SettingsMenu.test.ts per the <behavior> block (node-only, import the module helpers
    + readFileSync the source). Ensure every `it` has an expect (requireAssertions is on).
  </action>
  <verify>
    <automated>cd web && npm test -- --run src/lib/components/SettingsMenu.test.ts</automated>
  </verify>
  <done>SettingsMenu.svelte exists with the gear trigger + role=menu panel + all 7 items + the a11y/sign-out behavior; SettingsMenu.test.ts passes node-only and asserts the menuKeyAction/avatarUrl logic + the source a11y/security contract.</done>
</task>

<task type="auto">
  <name>Task 2: Slim SiteShell to use SettingsMenu; retire SessionIndicator if orphaned</name>
  <files>web/src/lib/components/SiteShell.svelte, web/src/lib/components/SessionIndicator.svelte</files>
  <action>
    Edit web/src/lib/components/SiteShell.svelte:
    - Remove the imports of ThemePicker and SessionIndicator; add
      `import SettingsMenu from './SettingsMenu.svelte';`.
    - Remove the inline Admin <button class="admin-nav"> + its goAdmin() function (the Admin link now
      lives inside SettingsMenu, still officer-gated).
    - Remove the standalone <ThemePicker bind:theme />, the <a href="/account"> "Account" link, the
      <a href="/char-meta"> "Character details" link, and the <SessionIndicator {session} />.
    - KEEP the Wantlist link, the Notifications link WITH its unread-badge + aria-label={notifyLabel}
      + the $page route-change refresh $effect + onMount(refreshUnread) logic + the count/badgeText
      derivations + the unreadCount/refreshUnread imports — these stay verbatim.
    - In .shell-controls, after the authenticated Wantlist + Notifications links, render
      `<SettingsMenu bind:theme {session} />`. The gear should be visible to any authenticated member
      (it always has at least Theme/Watcher-codes/Char-details/Sign-out; Admin is gated inside).
      Keep `theme = $bindable()` in SiteShell's $props (the +layout → SiteShell → SettingsMenu →
      ThemePicker chain). The session $derived from getContext stays as-is.
    - Drop now-unused CSS rules (.admin-nav and, if no remaining consumer, the parts of .char-meta-nav
      no longer used) — but KEEP .char-meta-nav / .notify-nav / .unread-badge styles still used by the
      Wantlist + Notifications links. Do NOT delete styles that the surviving links depend on.

    Then determine SessionIndicator's fate:
    - After saving SiteShell, grep the whole web/ tree for real imports of SessionIndicator
      (an `import ... SessionIndicator` or `from './SessionIndicator'` line — NOT comment mentions).
      Known comment-only mentions in AuthGate.svelte and +layout.svelte do NOT count.
    - If SiteShell was the ONLY importer (expected) → delete web/src/lib/components/SessionIndicator.svelte.
      Optionally scrub the stale "SessionIndicator" comment references in +layout.svelte / AuthGate.svelte
      to avoid dangling mentions (cosmetic; do not change behavior).
    - If ANYTHING else imports it → leave SessionIndicator.svelte in place and note it in the SUMMARY.
  </action>
  <verify>
    <automated>cd web && npm run check && npm run build && npm test -- --run</automated>
  </verify>
  <done>SiteShell header renders wordmark + Wantlist + Notifications(+badge) + SettingsMenu and nothing else; the bind:theme chain is intact; SessionIndicator.svelte is deleted iff SiteShell was its only importer; check + build + full test suite all pass.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Discord username/avatar → DOM | username + avatar hash are user-controlled values rendered in the menu |
| client nav (Admin/account links) → API | the hidden Admin item is UX only; the server re-checks officer on every /admin endpoint |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-sdh-01 | Tampering/XSS | SettingsMenu identity header (username/avatar) | mitigate | Render username ONLY via plain {} auto-escape (never {@html}); avatar alt = the same escaped username. Test asserts `{@html` is absent (carries T-15-22 forward verbatim). |
| T-sdh-02 | Elevation of Privilege | officer-only /admin menu item | accept | Layer-1 UX suppression only (`{#if session?.isOfficer}`); the server is the real gate on every /admin endpoint (15-03). Hidden nav is never the boundary. |
| T-sdh-03 | Information Disclosure | session prop passed to SettingsMenu | accept | Same session object already in the shell; no new data exposed, no new fetch. |
</threat_model>

<verification>
- `cd web && npm run check` — svelte-check + typecheck clean (the new $bindable/$props typings,
  the Session/ThemeKey imports, the page-store subscription all typecheck).
- `cd web && npm run build` — vite build succeeds (no missing imports after SessionIndicator removal).
- `cd web && npm test -- --run` — full node-only suite green, including SettingsMenu.test.ts.
- Manual reasoning (browser-smoke deferred — web tests are DOM-blind; note this gap in the SUMMARY):
  the menu's open/close/Escape/outside-click/route-change/first-item-focus behaviors are exercised
  only via the extractable logic + source contract, NOT a live DOM. Flag for a later browser smoke.
</verification>

<success_criteria>
- Header contains exactly: wordmark, Wantlist, Notifications(+unread badge), gear SettingsMenu.
- SettingsMenu dropdown contains, top→bottom: identity (avatar/fallback + officer shield + username),
  ThemePicker, "Watcher codes" (/account), "Character details" (/char-meta), officer-only "Admin"
  (/admin), divider, "Sign out".
- bind:theme chain +layout → SiteShell → SettingsMenu → ThemePicker intact; theme swap still re-themes.
- Escape closes + returns focus to trigger; outside-click + route-change close; first item focused on open.
- Non-officer sees no Admin item; officer does (`{#if session?.isOfficer}`).
- npm run check + build + test all pass; SessionIndicator.svelte deleted iff orphaned.
- CODE committed; NO .planning/ docs committed; NOT deployed.
</success_criteria>

<output>
After completion, create `.planning/quick/260607-sdh-reorganize-web-top-bar-into-accessible-s/260607-sdh-SUMMARY.md`
(the orchestrator handles committing it — the executor does NOT git-add .planning/). Note in the
SUMMARY: whether SessionIndicator was deleted, and the browser-smoke gap for the menu interactions.
</output>
