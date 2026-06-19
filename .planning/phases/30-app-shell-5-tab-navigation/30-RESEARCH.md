# Phase 30: App Shell + 5-Tab Navigation - Research

**Researched:** 2026-06-17
**Domain:** SvelteKit 2 SPA routing / app-chrome rework (`web/`)
**Confidence:** HIGH (the repo is the primary source; SvelteKit behaviors verified against the installed 2.61.1 runtime)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01 (NAV-01):** Five **real SvelteKit routes**, one per tab — `/characters`, `/inventory`, `/banks`, `/wishlist`, `/settings`. Each tab deep-linkable + shareable; refresh/back/bookmark keep you on the same tab. NOT client-side-only tab state. Active tab clearly indicated; tab strip reachable from any page.
- **D-02 (no broken bookmarks):** Every existing route **redirects** to its new home: `/wantlist` → `/wishlist`; `/notifications` → `/wishlist`; `/account`, `/char-meta`, `/my-characters`, `/admin`, `/bank-coin` → `/settings` (or the relevant section); `/` → the default tab (D-04).
- **D-03:** Tabs whose features ALREADY EXIST are fully functional immediately; genuinely-new tabs are placeholders. **Wishlist** = today's wantlist rehomed + notifications (D-07). **Settings** = Theme / Watcher codes / Set class & level / My characters / Admin consolidated (D-05). **Characters / Inventory / Banks** = friendly placeholders (content in Phases 31/32/33).
- **D-03b (core value must not regress):** The Characters/Inventory/Banks placeholders **preserve access to the existing consolidated inventory/gear/spell/bank views** (linked from the placeholder, or retained at a clearly-labeled interim path) until 31–33 replace them. No guildie loses guild-wide lookup mid-transition.
- **D-04 (default landing):** Tab ORDER is spec-fixed (Characters first) and never changes. During the stub window the default landing (`/`) **resolves to a functional surface** (the preserved inventory view), NOT a Characters stub. Once Phase 31 ships, `/`/default can move to Characters. **The planner owns the exact mechanism.**
- **D-05 (NAV-03):** Settings = **ONE page** composing existing panels as in-page **sections** (Theme · Watcher codes · Set class & level · My characters · Admin[officer-only]) + a **settings search** that filters/jumps to sections. Reuses ThemePicker, WatcherCodesPanel, CharMetaForm, MyCharactersPanel, AssignmentAdminPanel + MonitorAdminPanel. NOT a "menu of links to sub-pages."
- **D-06 (gear-menu fate):** The header **`SettingsMenu` gear dissolves** — config items move into the Settings tab. A **minimal identity + Sign out** stays **top-right**. The **Theme** control moves INTO Settings (no separate top-right theme picker). The `+layout → SiteShell → …` `[data-theme]` writer chain stays the single source of truth — only the picker's *location* moves.
- **D-07 (NAV-04):** ALL notification surfaces live on the **Wishlist tab**, NOT Settings: the unread **badge** on the Wishlist tab; the **alert inbox** + **notification preferences** (global opt-in/cooldown AND per-item ping toggles) reached there. **Overrides the stale NAV-03 wording.** The badge moves off the header onto the Wishlist tab.
- **D-08 (NAV-02):** Each tab owns a search bar **scoped to its own content**, at the top of the tab body (Variant A). A search bar shows **only when the tab has searchable content**: Wishlist + Settings get functional scoped search in Phase 30; Characters/Inventory/Banks receive theirs **with their content** in Phases 31/32/33 — **no inert search bar on stub tabs**. NAV-02 is *pattern-established* in Phase 30 and *completed per-tab* as content lands.

### Claude's Discretion (planner/researcher owns these — recommendations below in §"Decision Recommendations")
- Exact redirect mechanism (`+page.ts` redirect vs. server redirect vs. reroute hook).
- Where/how the preserved classic inventory view is retained (interim `/legacy`-style path, embedded body, or link from placeholder) — access never lost (D-03b).
- Shared layout (`+layout.svelte` tab strip + nested `+page`s) vs. a single shell component — pick the SvelteKit-idiomatic structure.
- Mobile/responsive collapse of the 5-tab strip (the existing header already `flex-wrap`s) — follow UI-SPEC.
- Exact Settings-search behavior (live section filter vs. jump-to-section).

### Deferred Ideas (OUT OF SCOPE for Phase 30)
- Characters tab list + in-game inventory window — **Phase 31** (CHAR/INV).
- Inventory tab item-centric list + holder drill-down — **Phase 32** (ITEM); the home `SearchBox` becomes this tab's search.
- Banks tab list + valuation totals — **Phase 33** (BANK).
- Per-character/per-slot Wishlist *rework* (open-ended targets + wiki suggestions + EC-hit badge) — **Phase 34** (WISH). Phase 30 only rehomes the EXISTING wantlist surface.
- WISH-07 full wishlist search — **Phase 34**.
- Retiring the preserved classic inventory/gear/spell view — happens when 31–33 replace it; **do NOT delete it in Phase 30** (D-03b).
- Gear Check / Spell Check views' long-term home — decided in Phases 34 / later; Phase 30 only keeps them reachable.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **NAV-01** | Five persistent top tabs (Characters, Inventory, Banks, Wishlist, Settings) with the active tab indicated, reachable from any page. | §"Tab-strip structure" — shared `+layout.svelte` strip + 5 nested `+page` routes; active tab from `$page.url.pathname`; existing `.tab`/`.char-meta-nav` styles in `SiteShell`/`+page` are the proven affordance. |
| **NAV-02** | Each tab has its own in-context search scoped to its content. | §"Per-tab search" — Wishlist+Settings get real scoped search now; stub tabs get NO inert search bar (D-08). `WantlistPanel` already owns its own filter; `SearchBox` is reserved for the Inventory tab (Phase 32). |
| **NAV-03** | Settings tab consolidates Theme, Watcher Codes, Set Class & Level, My Characters, Admin (officer) + a settings search. (Notifications prefs moved to Wishlist by D-07.) | §"Settings consolidation" — compose 6 existing self-contained panels as in-page sections; officer-gate the Admin section with `session.isOfficer` from `SESSION_KEY` context (same Layer-1 pattern as `/admin`). |
| **NAV-04** | Unread badge on the Wishlist tab; alert inbox + per-item ping prefs reached there. | §"Notifications onto Wishlist" — relocate the `unread.ts` badge read out of `SiteShell` onto the Wishlist tab strip entry / page; mount `NotificationInbox` + `NotificationPrefsPanel` on `/wishlist`. |
</phase_requirements>

---

## Summary

This is a **pure web routing/chrome rework** over an existing, mature SvelteKit 2 SPA — no backend, no watcher, no new dependencies. The repo runs **SvelteKit 2.61.1 + Svelte 5.56.0 (runes) + `@sveltejs/adapter-static` 3.0.10 in SPA mode** (`fallback: '200.html'`, `ssr=false`, `prerender=false` at the layout; only `/` is `prerender=true` to emit a real `index.html`). Every existing surface the phase touches — the home consolidated views, `/wantlist`, `/notifications`, and the five Settings sub-pages — is already a thin `+page.svelte` that mounts a single self-contained component (`WantlistPanel`, `NotificationInbox`, `WatcherCodesPanel`, etc.). **That is the dominant enabling fact: the phase is mostly *re-composition and re-routing of components that already exist and work*, not new feature code.**

The one genuinely version-sensitive decision is **D-02's redirect mechanism**. Verified against the installed runtime: because `ssr=false`, a universal `+page.ts` `load` runs **client-side**, and a thrown `redirect(308, …)` is caught by the SvelteKit client router and **changes the address bar** — this is exactly the "old bookmark lands on the new URL" behavior D-02 wants. The `reroute` hook is the wrong tool for D-02 (it deliberately does *not* change the URL), though it is a viable alternative for `/` default-landing aliasing. Recommendation below: thin `+page.ts` redirect modules at each old path. This is verified to run on the initial hard load too (the SPA's `navigate({type:'enter'})` boot path resolves the route through the load chain).

The chrome rework is low-risk: the `[data-theme]` writer chain (`+layout.svelte` `$effect(applyTheme)` → `bind:theme` → `SiteShell` → `SettingsMenu`/`ThemePicker`) must survive intact — only the *picker's location* moves into Settings (D-06). The single real trap is the **DOM-blind test suite** (node vitest, no jsdom, no @testing-library): green tests will not catch a broken route, a dead redirect, or a mis-wired badge — a **browser-smoke on a deployed build is mandatory** (localhost can't auth against prod).

**Primary recommendation:** Put the 5-tab strip in a new shared layout segment, redirect old paths with one-line `+page.ts` `redirect(308,…)` modules, rehome the existing components onto `/wishlist` and `/settings` unchanged, move the `unread` badge read onto the Wishlist tab entry, and keep `/` redirecting to a clearly-labeled interim path that renders today's `+page.svelte` consolidated views verbatim (D-03b / D-04).

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 5-tab top nav + active indicator | Frontend (SvelteKit client routing) | — | NAV-01 is pure client chrome; routes are static, content is data-driven at runtime. |
| Old-path → new-path redirects (D-02) | Frontend (`+page.ts` load, client-side) | — | `ssr=false` ⇒ no server load; the client router catches the thrown `redirect`. Caddy is a dumb static file server here, NOT a redirect tier. |
| Per-tab scoped search (NAV-02) | Frontend (in-component, in-memory) | — | Existing `WantlistPanel`/`SearchBox` already filter client-side over fetched rows. No API change. |
| Settings section composition (NAV-03) | Frontend (page composes existing components) | API (re-checks officer on every admin write) | UI gating is Layer-1 UX; the Go API (`webadmin`) is the real boundary — unchanged this phase. |
| Notification badge + inbox + prefs (NAV-04) | Frontend (`unread.ts` store + existing panels) | API (`/api/v1/notifications/*`, RequireSession) | Store/panels move location only; server endpoints unchanged. |
| Officer gate on the Admin section | API (server re-checks) | Frontend (`session.isOfficer` suppresses the section) | Matches the established two-layer pattern from `/admin`. |
| Theming (`[data-theme]`) | Frontend (`+layout.svelte` single writer) | — | Single-attribute write chain stays the source of truth; only the picker UI relocates (D-06). |

**No backend, no watcher, no DB, no CDN-config work in this phase.**

---

## Standard Stack

No new dependencies. Everything needed is already installed and in use.

### Core (already present — `web/package.json`)
| Library | Version (verified in package.json) | Purpose | Why Standard |
|---------|-----------|---------|--------------|
| `@sveltejs/kit` | **2.61.1** | App framework + client routing | Already the app's framework; tab routes are stock SvelteKit. |
| `svelte` | **5.56.0** (runes forced on, `svelte.config.js`) | Component runtime | Repo is all runes (`$state`/`$derived`/`$props`/`$effect`/`$bindable`). |
| `@sveltejs/adapter-static` | **3.0.10** | SPA build (`fallback: '200.html'`) | The deployed shape; constrains the redirect mechanism (see below). |
| `@lucide/svelte` | **1.17.0** | Icons (e.g. `settings`, `user`, `shield`, `log-out`) | Already used in `SettingsMenu`; reuse for tab/identity glyphs if needed. |
| `@tanstack/table-core` | **8.21.3** | DataGrid engine behind the consolidated views | Backs the preserved D-03b views; untouched. |

### Supporting (in-repo modules, not packages)
| Module | Path | Purpose | Use In Phase 30 |
|--------|------|---------|-----------------|
| theme writer | `web/src/lib/theme/themes.ts` (`loadTheme`/`applyTheme`/`ThemeKey`) | Single `[data-theme]` writer | Keep chain intact; relocate `ThemePicker` into Settings. |
| unread store | `web/src/lib/stores/unread.ts` (`unreadCount`, `refreshUnread`) | Badge count, server-truth | Move the read out of `SiteShell` onto the Wishlist tab. |
| session context | `web/src/lib/components/AuthGate.svelte` (`SESSION_KEY`, `SessionGetter`, `AUTH_GUARD_KEY`) | Session + officer bit + server-truth 401/403 re-route | Read `session.isOfficer` to gate the Settings→Admin section. |
| auth types | `web/src/lib/auth.ts` (`Session`, `resolveGate`, `ANON`) | Session shape | `session.isOfficer` is the officer gate; `session.username`/`avatar`/`discordUserId` feed the top-right identity. |
| `$app/stores` | SvelteKit | `page` store for `$page.url.pathname` | Compute the active tab. The repo uses `$app/stores` (NOT `$app/state`) — stay consistent. |
| `$app/navigation` | SvelteKit | `goto` (if any imperative nav needed) | Optional; prefer `<a href>` for tabs (prefetch + SSR-safe). |

**Installation:** none. `npm install` already satisfied.

**Version verification:** versions read directly from `web/package.json` (committed) and confirmed against `node_modules/@sveltejs/kit/package.json` → `2.61.1`. `reroute` and `redirect` exports both confirmed present in the installed 2.61.1 runtime (`src/exports/index.js`, `src/runtime/client/client.js`). `$app/stores` AND `$app/state` both exist in 2.61.1 — the repo standardizes on `$app/stores`.

---

## Decision Recommendations (Claude's Discretion items — concrete answers)

### 1. Redirect mechanism for D-02 — **USE `+page.ts` load `redirect(308, …)`** (one tiny module per old path)

**Recommended approach.** At each old route, add a `+page.ts` whose universal `load` throws a redirect:

```ts
// web/src/routes/wantlist/+page.ts   (and analogous for the others)
// Source: @sveltejs/kit 2.61.1 — redirect() (src/exports/index.js)
import { redirect } from '@sveltejs/kit';
export const ssr = false;          // inherit the SPA default; load runs in the browser
export const prerender = false;    // do NOT prerender a redirect page
export function load() {
  redirect(308, '/wishlist');      // 308 = permanent; address bar changes
}
```

Then **delete the old `+page.svelte` body** at that path (move its mount onto the new home). Mapping:

| Old route | Redirect target | Notes |
|-----------|-----------------|-------|
| `/wantlist` | `/wishlist` | Body (`WantlistPanel`) moves to `/wishlist`. |
| `/notifications` | `/wishlist` | Body (`NotificationPrefsPanel` + `NotificationInbox`) moves to `/wishlist` (D-07). |
| `/account` | `/settings` (Watcher-codes section; optionally `/settings#watcher-codes`) | `WatcherCodesPanel` → Settings section. |
| `/char-meta` | `/settings` (Set-class-&-level section) | `CharMetaForm` → Settings section. |
| `/my-characters` | `/settings` (My-characters section) | `MyCharactersPanel` → Settings section. |
| `/admin` | `/settings` (Admin section, officer-only) | `EvictionForm`+`AdminMgmtForm`+`MonitorAdminPanel`+`AssignmentAdminPanel` → Settings Admin section. |
| `/bank-coin` | `/settings` **or** the preserved-inventory path | `BankCoinForm`. Note: today the bank view links `/?view=bank` and `/bank-coin` links back to `/?view=bank` — keep that pairing coherent with the D-03b interim path (see §3). |
| `/` | the **interim preserved-inventory path** (D-04) | See §3 — redirect `/` to the path that renders today's `+page.svelte`. |

**Why this and not the alternatives — verified facts:**

- **`redirect()` in a universal load DOES run client-side here.** `redirect()` (verified, `src/exports/index.js`) `throw`s a `Redirect`; the status-range guard only fires under `!BROWSER || DEV`, i.e. it is browser-aware by design. Because `ssr=false` there is no server load — the universal `load` executes in the browser and the **client router catches the thrown redirect and updates the address bar**. `[CITED: svelte.dev/docs/kit/load — redirect() throws; client handles it]` `[VERIFIED: node_modules @sveltejs/kit 2.61.1 redirect impl]`
- **It fires on a cold bookmark hit, not just in-app nav.** Verified in the installed client runtime: SPA boot calls `navigate({ type: 'enter', … })` (`client.js:391`) → `get_navigation_intent(url)` (`:1682`) → resolves the matched route's load chain. So a guildie pasting an old `https://squirebot.quest/wantlist` bookmark loads `200.html`, the router matches `/wantlist`, runs its `+page.ts` load, and redirects to `/wishlist`. `[VERIFIED: node_modules @sveltejs/kit 2.61.1 client.js call graph]`
- **`reroute` is the WRONG tool for D-02.** The `reroute` hook deliberately *"will not change the contents of the browser's address bar."* `[CITED: svelte.dev/docs/kit/hooks]` D-02's whole point is *"no guildie's bookmark 404s"* — a redirect that leaves the URL on `/wantlist` would leave a stale, non-canonical address bar and defeat "the URL always reflects where you are" (CONTEXT resolution rule). Use `redirect()`, which moves the bar.
- **Server/Caddy redirects are out of reach.** Caddy serves the static `200.html` fallback; there is no per-route server load in this SPA. Don't push redirect logic into infra — it belongs in `+page.ts`. (A Caddy `redir` would also be a deploy-config change outside `web/`, which the phase scope forbids.)

**Gotcha to flag for the planner/verifier:** each redirect `+page.ts` MUST set `prerender = false`. The crawler is started from `/` (which is `prerender = true`); these redirect routes are not reachable from the prerendered `/` and won't be crawled, but if anything ever links to them at build time, a prerendered redirect page is meaningless. Keeping `prerender = false` + `ssr = false` makes them pure client-side redirect stubs. Also: do NOT wrap the `redirect()` call in a `try/catch` — catching the thrown `Redirect` silently breaks it (`[CITED: svelte.dev/docs/kit/load — "Make sure you're not catching the thrown redirect"]`).

**Optional refinement (only if the verifier wants the address bar to land on a section anchor):** redirect to `/settings#watcher-codes` etc. SvelteKit honors the hash on navigation; pair it with an `id` on each Settings section. Low value for Phase 30 — a plain `/settings` redirect is sufficient and simplest (D-rule).

### 2. Tab-strip structure — **shared layout segment + 5 nested `+page` routes** (the SvelteKit-idiomatic choice)

**Recommended.** Put the 5-tab strip in a layout so it renders once and persists across tab navigations, with one real route per tab:

```
web/src/routes/
  +layout.svelte            # UNCHANGED root: [data-theme] + AuthGate + SiteShell (theme chain)
  +layout.ts                # UNCHANGED: ssr=false, prerender=false
  +page.ts                  # CHANGE: redirect('/') → interim inventory path (D-04)
  characters/+page.svelte   # NEW placeholder (D-03) + link to preserved views (D-03b)
  inventory/+page.svelte    # NEW placeholder (D-03)
  banks/+page.svelte        # NEW placeholder (D-03)
  wishlist/+page.svelte     # NEW: WantlistPanel + NotificationInbox + NotificationPrefsPanel + badge home (D-03/D-07)
  settings/+page.svelte     # NEW: 6 panels as sections + settings search + officer-gated Admin (D-05)
  <interim>/+page.svelte    # the preserved consolidated views = TODAY's +page.svelte body (D-03b)
  wantlist/+page.ts         # redirect 308 → /wishlist          (body deleted)
  notifications/+page.ts    # redirect 308 → /wishlist          (body deleted)
  account/+page.ts          # redirect 308 → /settings          (body deleted)
  char-meta/+page.ts        # redirect 308 → /settings          (body deleted)
  my-characters/+page.ts    # redirect 308 → /settings          (body deleted)
  admin/+page.ts            # redirect 308 → /settings          (body deleted)
  bank-coin/+page.ts        # redirect 308 → /settings (or interim path)
```

**Where the tab strip lives:** the cleanest placement is **inside `SiteShell`'s `<header>` / between header and `<main>`**, replacing today's `Inventory | Wantlist | Notifications` nav links (lines 67–91 of `SiteShell.svelte`). `SiteShell` already renders `{@render children()}` in `<main>` and already reads `$page.url.pathname` (for the badge refresh effect), so it is the natural owner of the active-tab computation. The 5 tabs only show when `session?.authenticated` (same guard the current nav uses). This keeps the existing single shell component and avoids introducing a second nested-layout file — the simplest structure that satisfies D-01.

> Alternative considered: a dedicated `(app)/+layout.svelte` route group holding the tab strip, with redirects + pre-auth screens outside it. Rejected for Phase 30 — `SiteShell` already *is* the post-auth shell (it renders only inside `AuthGate`'s admitted branch), so a route group adds a file and a layout boundary for no behavioral gain. Revisit only if Phases 31–34 need per-tab layout data loaders.

**Active-tab indicator (NAV-01):** compute from the path, reusing the proven `.tab.active` styling already in `+page.svelte` (lines 431–436) and `.char-meta-nav` (lines 145–168):

```svelte
<script lang="ts">
  import { page } from '$app/stores';  // repo convention (NOT $app/state)
  const TABS = [
    { href: '/characters', label: 'Characters' },
    { href: '/inventory',  label: 'Inventory' },
    { href: '/banks',      label: 'Banks' },
    { href: '/wishlist',   label: 'Wishlist' },
    { href: '/settings',   label: 'Settings' }
  ];
  // startsWith so deep links (/settings#watcher-codes, future /characters/<name>) stay active.
  let path = $derived($page.url.pathname);
  function isActive(href: string) {
    return href === '/' ? path === '/' : path === href || path.startsWith(href + '/');
  }
</script>
<nav class="tab-strip" aria-label="Primary">
  {#each TABS as t (t.href)}
    <a href={t.href} class="tab" class:active={isActive(t.href)}
       aria-current={isActive(t.href) ? 'page' : undefined}>{t.label}</a>
  {/each}
</nav>
```

Use `<a href>` (not `goto`) so SvelteKit's client router + prefetch handle it, the URL updates, and it degrades gracefully. The `aria-current="page"` + accent underline are the existing accessible active-indicator idiom (`+page.svelte` line 321).

### 3. Preserving the classic inventory view (D-03b) — **retain today's `+page.svelte` body at a clearly-labeled interim path; redirect `/` there; link to it from the 3 stub placeholders**

**Recommended.** Move the current `web/src/routes/+page.svelte` (the 4-view consolidated home — Inventory/Gear/Spell/Bank + `SearchBox` + My/Guild scope) verbatim to an interim route, e.g. `web/src/routes/guild-views/+page.svelte` (label it "Guild views (classic)" or similar in the UI). Then:

- **`/` redirects to `/guild-views`** (D-04: default landing resolves to a *functional* surface, not a Characters stub). Implement as the `+page.ts` redirect in §1. Once Phase 31 ships the Characters tab, flip `/`'s target to `/characters` (one-line change — the planner notes this as the Phase-31 follow-up).
- **The Characters / Inventory / Banks placeholders each link to `/guild-views`** so the core "what's in the guild?" lookup is one click away from every stub (D-03b: the site never gets *less* useful). Copy suggestion: "Guild inventory lookup still lives here → [Open guild views]."
- **`/bank-coin`'s "Back to bank" + the bank view's `Record coin`** currently round-trip via `/?view=bank`. Repoint these to `/guild-views?view=bank` so the existing `?view=` query-seed (validated in `+page.svelte` `onMount`, lines 164–170) keeps working at the new path. (Redirect `/bank-coin` → `/settings` per D-02, but keep `BankCoinForm` reachable and its "back" link pointing at `/guild-views?view=bank`.)

**Why an interim route, not an embedded body:** the consolidated views are a ~600-line component with its own fetch/loading/scope state; embedding it inside three placeholder pages would triple-mount it and break the single-`SearchBox`/single-fetch model. One labeled route, linked-to, is the simplest non-regressing option. It's explicitly temporary — the Deferred list says it's retired when 31–33 land, NOT in Phase 30.

### 4. Notifications onto Wishlist (D-07 / NAV-04) — relocate the badge read + mount the inbox/prefs on `/wishlist`

**Current wiring (verified in `SiteShell.svelte`):**
- `SiteShell` imports `{ unreadCount, refreshUnread }` from `$lib/stores/unread` (line 15).
- It derives `count = $derived($unreadCount)`, `badgeText` (`9+` cap), `notifyLabel` (lines 42–46).
- `onMount` → `refreshUnread()` if authenticated (line 48–50); an `$effect` re-fetches on every route change keyed off `$page.url.pathname` (lines 53–60).
- The badge renders inside the `Notifications` nav link (lines 80–85) with `.unread-badge` styling (lines 177–192).

**Changes to relocate (NAV-04):**
1. **Remove** the `Notifications` nav link + badge from `SiteShell`'s header nav (delete lines 80–85 and the now-unused `Inventory`/`Wantlist` links at 74–75 — they become tabs).
2. **Put the unread badge on the Wishlist tab entry** in the new tab strip: the `Wishlist` tab `<a>` hosts the badge pill (reuse `.unread-badge` / `.notify-nav` styles, `position: relative`). Keep `count = $derived($unreadCount)` + `badgeText`/`notifyLabel` in whichever component owns the strip (SiteShell, since it stays the shell). The `refreshUnread()` on mount + route-change `$effect` STAY in `SiteShell` (they're global and already correct) — only the *render location* of the badge moves to the Wishlist tab.
3. **Mount the inbox + prefs on `/wishlist`:** `web/src/routes/wishlist/+page.svelte` renders `WantlistPanel` (rehomed body from `/wantlist`) PLUS `NotificationPrefsPanel` + `NotificationInbox` (rehomed from `/notifications`). `NotificationInbox` already calls `refreshUnread()` after a mark-read (per `unread.ts` header comment), so the badge stays authoritative when the user reads alerts on the Wishlist tab — no extra wiring.
4. **Per-item ping toggles** (D-07 "per-item ping prefs"): in Phase 30 these are whatever the *current* wantlist offers (the `muted`/bell affordance already in `WantlistPanel` per the 20-05 plan). The full per-slot ping rework is Phase 34 — Phase 30 only rehomes today's surface.

**Gotcha:** `unread.ts` is a module-level `writable` store shared across components — moving the badge's render location does NOT require touching the store. Don't duplicate the store or prop-drill the count; read `$unreadCount` wherever the Wishlist tab `<a>` is rendered.

### 5. Settings consolidation (D-05/D-06) — compose 6 self-contained panels as sections; officer-gate Admin; dissolve the gear; keep identity+Sign out+theme-writer

**The panels are already drop-in.** Each current settings sub-page is a thin wrapper that mounts ONE component and adds a `.form-card` + title (verified by reading all six). So `/settings` is built by importing the components directly and stacking them as `<section>`s — no logic extraction needed:

| Settings section | Component to mount | Source page (now redirected) | Gate |
|------------------|--------------------|------------------------------|------|
| Theme | `ThemePicker` (with `bind:theme`) | (was in `SettingsMenu` gear) | member |
| Watcher codes | `WatcherCodesPanel` | `/account` | member |
| Set class & level | `CharMetaForm` | `/char-meta` | member |
| My characters | `MyCharactersPanel` | `/my-characters` | member |
| Admin → Evict guildie | `EvictionForm` | `/admin` | **officer** |
| Admin → Manage officers | `AdminMgmtForm` | `/admin` | **officer** |
| Admin → Monitors | `MonitorAdminPanel` | `/admin` | **officer** |
| Admin → Character assignments | `AssignmentAdminPanel` | `/admin` | **officer** |

> Note: `MonitorAdminPanel` is **notification-monitor admin (kill-switches / source channels)** — that is officer config and stays in Settings→Admin (NOT on the Wishlist tab; D-07's "notification preferences" means the *member's own* prefs/inbox, which are `NotificationPrefsPanel`/`NotificationInbox`). Keep the distinction crisp for the planner.

**Officer-gating the Admin section (NAV-03):** read the session from context exactly as `/admin/+page.svelte` does (lines 26–31):

```svelte
import { getContext } from 'svelte';
import { SESSION_KEY, type SessionGetter } from '$lib/components/AuthGate.svelte';
const getSession = getContext<SessionGetter>(SESSION_KEY);
let session = $derived(getSession ? getSession() : null);
let isOfficer = $derived(!!session?.isOfficer);
// ...
{#if isOfficer}
  <section class="admin-sections"> … EvictionForm / AdminMgmtForm / MonitorAdminPanel / AssignmentAdminPanel … </section>
{/if}
```

This is **Layer-1 UX suppression only** — the Go API re-checks officer status on every admin write and the forms hand a 403 to `authGuard` (which collapses the whole gate to the Officers-only screen). The hidden section is never the boundary (`/admin/+page.svelte` comment, lines 6–15). The `/admin` redirect (§1) sends a non-officer who bookmarked `/admin` to `/settings`, where the Admin section simply isn't rendered — correct and non-alarming.

**Gear dissolves; identity + Sign out + theme-writer survive (D-06):**
- **Dissolve `SettingsMenu`** from the header. Its link list (`/account`, `/char-meta`, `/my-characters`, `/admin`) becomes the Settings sections above; its `ThemePicker` moves into Settings.
- **Keep a minimal identity + Sign out top-right** in `SiteShell`'s header `.shell-controls`. Reuse the *display* logic from `SettingsMenu`'s `<script module>`: `avatarUrlFor(session)` (Discord CDN avatar or glyph fallback), the escaped `session.username` (T-15-22: plain `{}` interpolation only — never raw HTML), the officer `Shield`, and the `signOut()` flow (`logout()` then `window.location.href = '/'`). This can be a small new `IdentityMenu`/inline block or a trimmed `SettingsMenu` that keeps ONLY identity + Sign out. **Security note:** preserve the username-escaping invariant when porting — it is a real XSS control, not cosmetic.
- **Theme writer chain stays intact (D-06):** `+layout.svelte`'s `$effect(() => applyTheme(theme, rootEl))` remains the single `[data-theme]` writer. `SiteShell` keeps `theme = $bindable()` and passes `bind:theme` down. The ONLY change is that `ThemePicker` now lives inside `/settings` instead of inside the header `SettingsMenu`. **The bind chain must reach the Settings page.** `/settings` is rendered as `{@render children()}` inside `SiteShell`, so it can't receive `bind:theme` as a prop the way `SettingsMenu` does. Two clean options:
  - **(a) Recommended — a tiny theme context:** `+layout.svelte` (or `SiteShell`) `setContext`s a `{ get theme, set theme }` accessor; the Settings `ThemePicker` reads/writes it. This keeps the single-writer `$effect` in `+layout` and lets a page-level picker mutate `theme`. Mirrors the existing `SESSION_KEY` context pattern.
  - **(b) Alternative — move theme state into a store:** lift `theme` into a `$lib/theme` writable; `+layout`'s `$effect` subscribes and writes `[data-theme]`; the Settings picker sets the store. Slightly larger change.
  - Option (a) is the smaller diff and matches the repo's context idiom. **Flag this as the one non-trivial wiring task** — the rest of Settings is mechanical composition.

**Settings search (D-05, exact behavior is Claude's discretion):** recommend the **simplest end-user experience** per the resolution rule — a single text input at the top of `/settings` that **live-filters which sections are visible** (case-insensitive substring over a fixed list of section titles/keywords). Each section gets an `id` + a keyword list; typing "watcher" shows only the Watcher-codes section, "" shows all. This is purely client-side, no new state machinery, and avoids a "jump-to-anchor" that can feel like a dead control when the section is already on screen. (A jump-to-section variant is acceptable but adds scroll-management complexity for no clarity gain at 5–8 sections.)

---

## Architecture Patterns

### System Architecture Diagram (data/render flow — Phase 30 scope)

```
  Browser hits squirebot.quest/<path>
        │
        ▼
  Caddy serves 200.html (SPA fallback)              [adapter-static, unchanged]
        │
        ▼
  SvelteKit client router boot: navigate({type:'enter'})   [client.js:391]
        │  resolves matched route → runs its load chain
        ▼
  ┌───────────────────────────── route match ─────────────────────────────┐
  │  OLD path (/wantlist, /account, …)   │   NEW path (/characters, …) or /│
  │        │                              │            │                    │
  │  +page.ts load: redirect(308, …)      │     +page.svelte renders        │
  │   → client router updates URL ────────┼──────────► (re-enters here)     │
  └───────────────────────────────────────┴────────────────────────────────┘
        │
        ▼
  +layout.svelte:  <div data-theme> AuthGate > SiteShell > {children}
        │              (single [data-theme] writer via $effect(applyTheme))
        ▼
  AuthGate (session via whoami-web; setContext SESSION_KEY/AUTH_GUARD_KEY)
        │  gate: auth-loading → login → not-member → officers-only → app
        ▼
  SiteShell (post-auth):  wordmark │ [5-TAB STRIP w/ active indicator + Wishlist badge] │ identity+SignOut
        │                                    ▲ unreadCount store; refreshUnread on mount + route change
        ▼  <main>{children}</main>
  ┌─────────────┬───────────┬────────┬──────────────────────────┬───────────────────────────┐
  │ /characters │ /inventory│ /banks │ /wishlist                │ /settings                 │
  │ placeholder │ placeholder│ placeh.│ WantlistPanel +          │ ThemePicker · WatcherCodes·│
  │  → link to  │  → link to │  → link│ NotificationPrefsPanel + │ CharMeta · MyCharacters ·  │
  │ /guild-views│/guild-views│  ...   │ NotificationInbox        │ Admin[officer] + search    │
  └─────────────┴───────────┴────────┴──────────────────────────┴───────────────────────────┘
                        │
                        ▼  (D-03b preserved, interim)
                  /guild-views  = today's +page.svelte (4 views + SearchBox + scope)
```

External dependency boundary: the Go read/notification API (`api.squirebot.quest`, `credentials:'include'`) — **unchanged this phase**; all calls go through the already-built `$lib/api` + the existing panels.

### Pattern 1: Thin route → single component (the repo's established page idiom)
**What:** every leaf route is a `+page.svelte` that mounts one self-contained component inside a `.form-card`, sets `<svelte:head><title>`, and adds intro copy. **When to use:** the placeholder tabs and the rehomed Wishlist/Settings follow this exact shape. **Example (verified, `/account/+page.svelte`):** imports `WatcherCodesPanel`, wraps it in `.form-card` + heading. Phase 30 reuses these components 1:1.

### Pattern 2: Context-provided session for UX gating (verified, `/admin/+page.svelte`)
**What:** read `SESSION_KEY` via `getContext`, derive `isOfficer`, suppress officer-only UI. The server is the real gate. **When to use:** the Settings→Admin section.

### Pattern 3: `[data-theme]` single-writer chain (verified, `+layout.svelte`)
**What:** one `$effect(applyTheme)` in `+layout`, `bind:theme` threaded down; a theme change is one attribute write + localStorage persist. **Phase 30 constraint:** preserve it; relocate only the picker (use a theme context to reach the Settings page — §5a).

### Anti-Patterns to Avoid
- **`reroute` for bookmark redirects.** It doesn't change the URL → defeats D-02. Use `redirect()`.
- **Catching the thrown `redirect`** in the load — silently breaks it. Leave it uncaught.
- **An inert search bar on the Characters/Inventory/Banks stubs.** D-08 forbids it — those get search *with* their content in 31/32/33.
- **Per-character persistent routes/tabs at guild scale.** The consolidated-views lock is relaxed to allow master-detail drill-down, NOT N materialized per-character routes (CLAUDE.md). Not a Phase-30 risk (no tab content here), but the placeholders must not seed that pattern.
- **Duplicating the `unread` store / prop-drilling the count.** Read `$unreadCount` where the Wishlist tab renders.
- **Re-skinning.** Out of scope — EQ theme tokens only, existing 5 themes unchanged.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Old-path → new-path redirect | A custom in-`onMount` `window.location` hack, or a client `goto` in every old page | `+page.ts` `redirect(308, …)` | SvelteKit's redirect runs in the load chain on cold loads + in-app nav, updates the URL, and is one line. Verified to fire on `type:'enter'` boot. |
| Active-tab detection | Manual click-state booleans | `$page.url.pathname` + `class:active` + `aria-current` | Survives refresh/back/deep-link; already the repo idiom (`+page.svelte` nav). |
| Settings panels | New forms for theme/watcher/char-meta/etc. | Mount the existing `ThemePicker`/`WatcherCodesPanel`/`CharMetaForm`/`MyCharactersPanel`/admin panels | They're already built, tested, IDOR-safe, and self-contained. Phase 30 = composition, not new forms. |
| Unread badge | A new counter/fetch | `$lib/stores/unread` (`unreadCount`/`refreshUnread`) | Server-truth store already wired; just move the render location. |
| Identity + avatar + sign-out | New session UI + logout | Reuse `SettingsMenu`'s `avatarUrlFor` + `logout()` + escaped-username render | Carries the T-15-22 XSS-safe username invariant; don't re-derive it. |
| Officer gating | A new permission check | `session.isOfficer` from `SESSION_KEY` context (server re-checks) | Established two-layer pattern; never trust the client gate. |

**Key insight:** Phase 30 is ~90% **re-composition of existing, working components into a new route/chrome layout** + ~10% genuinely-new code (the tab strip, the redirect stubs, the theme-context bridge, the Settings-search filter, the three placeholders). Treat any task that proposes *rewriting* a settings/notification/wantlist panel as a smell.

---

## Common Pitfalls

### Pitfall 1: DOM-blind tests pass while the app is broken
**What goes wrong:** `npm test` (node vitest, no jsdom, no @testing-library/svelte) stays green even if a route 404s, a redirect is mis-targeted, the badge vanishes, or the theme context doesn't reach Settings. (Real precedent: P15 shipped 165 green tests with 2 crashing browser BLOCKERs.)
**Why it happens:** the suite tests pure functions (`resolveGate`, `avatarUrlFor`, `menuKeyAction`, search/sort logic) — it never renders Svelte components or exercises routing.
**How to avoid:** **mandatory browser-smoke on a deployed build** of: every old bookmark redirecting correctly; all 5 tabs reachable + active-indicator; Wishlist badge present + inbox/prefs work; Settings sections all render; Admin section hidden for a non-officer; theme switch from Settings still writes `[data-theme]`; `/` lands on the preserved views. Localhost can't auth against prod (cookie Domain=squirebot.quest + apex-only CORS) — smoke on prod after deploy OR run a full local stack (local backend + `SQUIREBOT_COOKIE_INSECURE` + `PUBLIC_API_BASE` + seeded `sb_session`).
**Warning signs:** "all tests pass" used as the completion criterion for a routing/nav change.

### Pitfall 2: `redirect()` mis-handled in SPA mode
**What goes wrong:** redirect doesn't fire, or fires but throws "Invalid status code," or is swallowed by a `try/catch`.
**Why it happens:** forgetting `ssr=false` is inherited (load runs client-side — that's fine); catching the thrown `Redirect`; or using a status outside 300–308.
**How to avoid:** plain uncaught `redirect(308, '/target')` in a `load` that returns nothing else; keep `prerender=false`; don't wrap in try/catch.
**Warning signs:** a redirect page that briefly renders content before navigating (means you put nav in `onMount` instead of `load`).

### Pitfall 3: Theme picker can't mutate `theme` from `/settings`
**What goes wrong:** moving `ThemePicker` into the Settings page breaks the `bind:theme` chain (the page receives `children`, not a `theme` prop), so changing the theme does nothing.
**Why it happens:** `bind:theme` flowed `+layout → SiteShell → SettingsMenu`; a `{@render children()}` page can't be bound that way.
**How to avoid:** bridge via a theme **context** (`setContext` in `+layout`/`SiteShell`, `getContext` in the Settings `ThemePicker`) — §5a. Keep the single `$effect(applyTheme)` writer in `+layout`.
**Warning signs:** theme switch works in the header before the change but not from Settings after.

### Pitfall 4: Losing the D-03b lookup mid-transition
**What goes wrong:** the home `+page.svelte` is deleted/replaced and the consolidated guild views vanish before Phases 31–33 land, so guildies can't look up "what's in the guild."
**Why it happens:** treating Characters/Inventory/Banks as "done" placeholders without preserving the old surface.
**How to avoid:** move today's `+page.svelte` to `/guild-views`, redirect `/` there, and link to it from the three stubs (§3). Do NOT delete it (Deferred list).
**Warning signs:** the three placeholders have no path back to the existing views.

### Pitfall 5: `?view=bank` deep-link / "Record coin" round-trip breaks
**What goes wrong:** `/bank-coin`'s "Back to bank" → `/?view=bank` and the bank view's "Record coin" → `/bank-coin` stop working after `/` is repointed.
**Why it happens:** the `?view=` seed lives in the old home `onMount`; redirecting `/` moves it.
**How to avoid:** repoint those links to `/guild-views?view=bank`; keep the param validation (it ignores unknown values). Verify the round-trip in the browser-smoke.

---

## Code Examples

### Old-path redirect stub (one per old route)
```ts
// web/src/routes/account/+page.ts
// Source: @sveltejs/kit 2.61.1 — redirect() (verified src/exports/index.js)
import { redirect } from '@sveltejs/kit';
export const ssr = false;
export const prerender = false;
export function load() {
  redirect(308, '/settings');
}
```

### Default-landing redirect (D-04, flips to /characters when Phase 31 ships)
```ts
// web/src/routes/+page.ts  (REPLACES the current `export const prerender = true;`)
import { redirect } from '@sveltejs/kit';
export const ssr = false;
export const prerender = false;
export function load() {
  redirect(307, '/guild-views'); // 307 (temporary) — target moves to /characters post-Phase-31
}
```
> Note: this drops `/`'s `prerender = true`. That is fine in SPA mode — `200.html` is the entry document and the client router takes `/` from there. If a prerendered `index.html` is still desired for the apex, keep a minimal prerendered `/` whose body is just the redirect; the simplest path is the redirect above. Planner: confirm Caddy serves `200.html` for `/` (it already does — the SPA fallback).

### Officer-gated Settings→Admin section (verified pattern from /admin)
```svelte
<script lang="ts">
  import { getContext } from 'svelte';
  import { SESSION_KEY, type SessionGetter } from '$lib/components/AuthGate.svelte';
  import EvictionForm from '$lib/components/EvictionForm.svelte';
  import AdminMgmtForm from '$lib/components/AdminMgmtForm.svelte';
  import MonitorAdminPanel from '$lib/components/MonitorAdminPanel.svelte';
  import AssignmentAdminPanel from '$lib/components/AssignmentAdminPanel.svelte';
  const getSession = getContext<SessionGetter>(SESSION_KEY);
  let session = $derived(getSession ? getSession() : null);
  let isOfficer = $derived(!!session?.isOfficer);
</script>
{#if isOfficer}
  <section id="admin" class="settings-section">
    <h2>Admin</h2>
    <EvictionForm /><AdminMgmtForm /><MonitorAdminPanel /><AssignmentAdminPanel />
  </section>
{/if}
```

### Wishlist tab entry with relocated unread badge (NAV-04)
```svelte
<!-- inside the tab strip in SiteShell; count/badgeText already derived there -->
<a href="/wishlist" class="tab notify-tab" class:active={isActive('/wishlist')}
   aria-current={isActive('/wishlist') ? 'page' : undefined}
   aria-label={count > 0 ? `Wishlist, ${count} unread` : 'Wishlist'}>
  Wishlist
  {#if count > 0}<span class="unread-badge" aria-hidden="true">{badgeText}</span>{/if}
</a>
```

---

## State of the Art

| Old (this repo, pre-Phase-30) | Current (post-Phase-30) | Why |
|-------------------------------|--------------------------|-----|
| Header nav: `Inventory / Wantlist / Notifications` links + gear menu | 5-tab strip + minimal identity/Sign-out top-right | NAV-01/D-06 — tabs are primary nav; gear dissolves. |
| Scattered settings sub-pages (`/account`, `/char-meta`, `/my-characters`, `/admin`) | One `/settings` page with sections + search | NAV-03/D-05 — one place per concept. |
| Notifications at `/notifications` + badge in header | Notifications on `/wishlist`, badge on the Wishlist tab | NAV-04/D-07 — every alert is a wishlist ping. |
| `/` prerendered to the 4-view home | `/` redirects to interim `/guild-views`; `/characters` is the eventual default | D-03b/D-04 — preserve lookup, land on something functional. |

**Not deprecated, just relocated:** all settings/notification/wantlist components survive unchanged — only their routes/composition change.

**SvelteKit-version note:** `$app/stores` (used here) is the older API; `$app/state` is the newer fine-grained-reactivity equivalent and is present in 2.61.1. **Stay on `$app/stores`** for consistency with the existing `SiteShell`/`SettingsMenu`/`+page` code — mixing the two adds churn for no benefit this phase.

---

## Project Constraints (from CLAUDE.md)

- **GSD workflow enforcement:** all file changes go through a GSD command (this phase = `/gsd-execute-phase`); no direct repo edits outside the workflow.
- **EQ theme tokens only:** `--accent` / `--panel` / `--font-display` / `--bg` / `--text` / `--border`; no new palette (milestone Out-of-Scope: no re-skin).
- **44px touch targets; focus-visible 2px accent outline** on every new nav affordance (matches existing `.tab` / `.char-meta-nav` / `.menu-link`).
- **Consolidated-views lock RELAXED:** per-character master-detail drill-down allowed; do NOT materialize N persistent per-character routes/tabs (guild-scale explosion). Not triggered in Phase 30 (no tab content), but the placeholders must not seed it.
- **Single `[data-theme]` writer** on the shell root is the source of truth — preserve the chain (D-06).
- **Structured logging** convention is Go/Apps-Script-side; not applicable to this web-only phase.
- **Watcher untouched; no backend change; extend-only schema** — none of these are touched in Phase 30.
- **Username is user-controlled (Discord) → render via plain `{}` interpolation only, never raw HTML** (T-15-22) — preserve when porting the identity affordance out of `SettingsMenu`.

---

## Runtime State Inventory

> This is a rename/relocation-flavored phase (routes move, the gear dissolves, the badge relocates), so the inventory applies — but everything is **code-only**; nothing is stored, OS-registered, or env-bound.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **None** — no DB/datastore keys reference route paths. Routes are client-side only; the Go API endpoints (`/api/v1/...`) are unchanged and unaffected by frontend path changes. | None |
| Live service config | **None** — Caddy serves the static `200.html` SPA fallback for all paths; there is no per-route server config to change (verified `svelte.config.js` SPA comment + `+layout.ts`). No CDN/edge config. | None |
| OS-registered state | **None** — no OS-level registration references these web routes. | None |
| Secrets/env vars | **None** — no env var names reference route paths. `PUBLIC_API_BASE` / cookie domain unchanged. | None |
| Build artifacts | **Stale link targets in `web/src`:** any in-repo `href`/`goto`/`<a href>` pointing at an old path (`/wantlist`, `/notifications`, `/account`, `/char-meta`, `/my-characters`, `/admin`, `/bank-coin`, `/?view=bank`) must be repointed to the new tab/section so internal navigation doesn't bounce through a redirect. **Known references to fix:** `SiteShell.svelte` header nav (`/`, `/wantlist`, `/notifications`); `+page.svelte` "Claim characters" → `/my-characters`, "Record coin" → `/bank-coin`; `/bank-coin` "Back to bank" → `/?view=bank`; `/char-meta` ↔ `/my-characters` cross-links; `wantlist`/`account` intro cross-links. Grep `href="/` and `goto(` across `web/src` during planning to enumerate all. | code edit (link repoint) |

**The canonical check:** after the routes move, every internal link should target a *new canonical* path (tab or `/settings`), not rely on the redirect stubs. Redirect stubs are for external bookmarks; internal links should be clean.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The interim preserved-views path name (`/guild-views`) is a free choice; any clear label works. | §3 / D-03b | None — naming is cosmetic; the requirement is "reachable + labeled." Planner/UI-SPEC may pick a different name. |
| A2 | Caddy serves `200.html` for `/` and unknown paths (standard SPA fallback). | Code Examples / §1 | LOW — if Caddy only serves `index.html` for `/`, dropping `/`'s `prerender=true` could 404 the apex on a hard refresh. Verify in browser-smoke; if so, keep a minimal prerendered `/` that redirects. `[ASSUMED]` from the SPA config comment, not from the live Caddyfile (which lives outside `web/` and outside phase scope). |
| A3 | The current wantlist's per-item mute/bell is the extent of "per-item ping prefs" for Phase 30 (full per-slot ping = Phase 34). | §4 D-07 | LOW — if the user expects more per-item ping UI now, scope creeps into Phase 34 territory; CONTEXT D-07 + Deferred list say Phase 30 only rehomes the existing surface. |
| A4 | `MonitorAdminPanel` (monitor kill-switches/channels) belongs in Settings→Admin, not on the Wishlist tab; only `NotificationPrefsPanel`/`NotificationInbox` move to Wishlist. | §5 | LOW — D-07 says "notification preferences (global opt-in/cooldown AND per-item ping toggles)" go to Wishlist; that maps to `NotificationPrefsPanel`, not the officer `MonitorAdminPanel`. If the user wants monitor admin on Wishlist too, adjust — but that mixes member-prefs and officer-config, which D-07's "one mental model" argues against. |

---

## Open Questions

1. **Apex prerender vs. SPA fallback for `/` (A2).**
   - What we know: `svelte.config.js` ships `fallback: '200.html'`; the current `/+page.ts` is `prerender=true` to emit `index.html`.
   - What's unclear: whether Caddy maps the bare apex to `index.html` (prerendered) or `200.html` (fallback). Dropping `prerender=true` on `/` to make it a redirect is clean IF the fallback serves the apex.
   - Recommendation: implement the `/` redirect via `+page.ts` (drop `prerender`), then **browser-smoke a hard refresh of `https://squirebot.quest/`** on the deployed build; if it 404s, restore a minimal prerendered `/` whose only job is the redirect. Cheap to verify, cheap to fix.

2. **Section-anchor redirects vs. plain `/settings`.**
   - What we know: redirects can target `/settings#watcher-codes`.
   - What's unclear: whether the UI-SPEC wants old bookmarks to land scrolled to the specific section.
   - Recommendation: ship plain `/settings` redirects (simplest, D-rule); add anchors only if `/gsd-ui-phase 30` calls for them. Low effort to add later.

3. **Mobile collapse of the 5-tab strip.**
   - What we know: the existing header `flex-wrap`s; 5 short labels fit most widths.
   - What's unclear: exact small-screen treatment (wrap vs. horizontal scroll vs. menu).
   - Recommendation: defer to the UI-SPEC (`/gsd-ui-phase 30`); the existing `flex-wrap` + `@media (max-width:640px)` gutter reduction in `SiteShell` is a safe default for Phase 30.

---

## Environment Availability

Step 2.6: SKIPPED (no external dependencies). Phase 30 is web-only, no new tooling — `npm` + the already-installed `web/` toolchain (SvelteKit 2.61.1, Svelte 5, vite 8, vitest 4) is the entire surface, and all are present per `web/package.json`. No DB, no service, no CLI, no backend, no watcher.

---

## Security Domain

> `security_enforcement: true` in config. Phase 30 introduces no new server surface; the security posture is *preservation*, not new controls.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control (existing, must be preserved) |
|---------------|---------|-----------------|
| V2 Authentication | yes (preserve) | `AuthGate` + whoami-web session; fail-safe to `ANON`; server is the real gate. Unchanged. |
| V3 Session Management | yes (preserve) | httpOnly `sb_session` cookie, apex-only CORS, `credentials:'include'`. Unchanged. |
| V4 Access Control | yes (preserve) | Two-layer officer gate: client `session.isOfficer` suppresses the Settings→Admin section (Layer 1 UX); the Go API re-checks officer on every admin write (Layer 2, real boundary). The `/admin` redirect to `/settings` does NOT weaken this — a non-officer sees no Admin section, and the server still 403s + collapses the gate. |
| V5 Input Validation | yes (preserve) | The Discord `username` is user-controlled → rendered via plain `{}` interpolation only (Svelte auto-escape), NEVER raw HTML (T-15-22). **Preserve this invariant when porting the identity affordance out of `SettingsMenu`.** Settings-search input is filtered client-side over a static section list — no injection surface. |
| V6 Cryptography | no | No crypto in this phase. |

### Known Threat Patterns for {SvelteKit SPA, this phase}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stored/reflected XSS via Discord display name in the new top-right identity block | Tampering / Elevation | Plain `{}` interpolation only (auto-escaped); avatar `alt` = same escaped name. Port `avatarUrlFor` + the escaped-username render verbatim from `SettingsMenu`. |
| Client-side officer gate trusted as the boundary | Elevation of Privilege | It is NOT the boundary — the Go API re-checks officer on every admin endpoint and `authGuard` collapses the gate on a 403. Keep the section suppression as UX only. |
| Bookmark redirect leaking to an unintended route | Information Disclosure | Redirect targets are static literals (`/wishlist`, `/settings`) — no user input flows into `redirect(location)`. Do not build the target from query/path input. |

**Net:** Phase 30 adds no new server endpoints and no new trust boundary. The single live security task is **preserving the username-escaping invariant** when the identity affordance moves out of `SettingsMenu`.

---

## Sources

### Primary (HIGH confidence)
- **The SquireBot repo itself** (read in this session): `web/package.json`, `web/svelte.config.js`, `web/src/routes/{+layout,+page,wantlist,notifications,account,char-meta,admin,my-characters,bank-coin}/*`, `web/src/lib/components/{SiteShell,SettingsMenu,AuthGate}.svelte`, `web/src/lib/stores/unread.ts`, `web/src/lib/auth.ts`. `[VERIFIED: codebase grep/read]`
- **Installed SvelteKit 2.61.1 runtime** — `node_modules/@sveltejs/kit/src/exports/index.js` (`redirect` impl, browser-aware), `src/runtime/client/client.js` (`reroute`/`get_rerouted_url`/`navigate({type:'enter'})` call graph confirming redirects + reroute fire on cold SPA boot). `[VERIFIED: node_modules]`
- `.planning/phases/30-app-shell-5-tab-navigation/30-CONTEXT.md` (D-01..D-08), `.planning/REQUIREMENTS.md` (NAV-01..04), `.planning/ROADMAP.md` (Phase 30 goal + 4 success criteria), `.planning/sketches/MANIFEST.md` + `001-app-shell-5tab-nav/README.md` (Variant A lock; notifications→Wishlist), `./CLAUDE.md`. `[VERIFIED: codebase read]`

### Secondary (MEDIUM-HIGH confidence)
- SvelteKit docs — `redirect()` in load (status 300–308; don't catch the throw; client handles it). `[CITED: svelte.dev/docs/kit/load]`
- SvelteKit docs — `reroute` hook (lives in `src/hooks.js`; runs on initial load + client nav; **does not change the address bar**; pure/idempotent; async since 2.18). `[CITED: svelte.dev/docs/kit/hooks]`

### Tertiary (LOW confidence)
- A2 (Caddy apex → `200.html` fallback) is `[ASSUMED]` from the SPA config comment, not from the live Caddyfile (outside `web/`/phase scope). Resolve via browser-smoke.

---

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** — versions read from committed `package.json` + verified in `node_modules`; no new deps.
- Redirect mechanism (D-02): **HIGH** — `redirect()`/`reroute` behavior verified against the installed 2.61.1 runtime AND official docs; the SPA cold-boot reroute path traced in `client.js`.
- Tab structure / chrome rework: **HIGH** — every component the phase recomposes was read directly; the active-tab + theme-chain + officer-gate patterns are lifted from existing code.
- Pitfalls: **HIGH** — drawn from the repo's own documented incidents (DOM-blind tests, localhost-can't-auth, `?view=bank` round-trip) + verified SvelteKit semantics.
- A2 (apex prerender vs fallback): **LOW** — flagged as the one thing to confirm in browser-smoke.

**Research date:** 2026-06-17
**Valid until:** ~2026-07-17 (stable — pinned SvelteKit/Svelte versions; behavior won't drift unless the team bumps `@sveltejs/kit` or changes the adapter). Re-verify the redirect mechanism only if SvelteKit is upgraded past 2.x.
