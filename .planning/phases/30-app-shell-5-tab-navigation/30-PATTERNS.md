# Phase 30: App Shell + 5-Tab Navigation - Pattern Map

**Mapped:** 2026-06-17
**Files analyzed:** 23 (6 new routes + 7 redirect stubs + 1 interim route + ~3 chrome edits + link repoints)
**Analogs found:** 23 / 23 (every file mirrors an in-repo analog; only `redirect(308,…)` is net-new one-liner code)

> This map is the grep-anchored companion to `30-RESEARCH.md` §"Decision Recommendations" and `30-UI-SPEC.md` §"Layout & Interaction Contract". RESEARCH already named the analogs and recommended the mechanisms; this file pins **exact file paths + line ranges + the 5-20 lines worth copying** so the planner's `<read_first>`/`<action>` fields cite concrete excerpts. **Phase 30 is ~90% re-composition of existing components** — treat any task that *rewrites* a settings/notification/wantlist panel as a smell (RESEARCH "Key insight").

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/src/routes/characters/+page.svelte` | new-route (placeholder) | request-response | `web/src/routes/account/+page.svelte` (`.form-card` thin wrapper) | role-match |
| `web/src/routes/inventory/+page.svelte` | new-route (placeholder) | request-response | `web/src/routes/account/+page.svelte` | role-match |
| `web/src/routes/banks/+page.svelte` | new-route (placeholder) | request-response | `web/src/routes/account/+page.svelte` | role-match |
| `web/src/routes/wishlist/+page.svelte` | new-route (rehome) | request-response | `web/src/routes/notifications/+page.svelte` + `web/src/routes/wantlist/+page.svelte` | exact (compose both) |
| `web/src/routes/settings/+page.svelte` | new-route (compose) | request-response | `web/src/routes/admin/+page.svelte` (officer gate + section stacking) | exact |
| `web/src/routes/guild-views/+page.svelte` (interim, D-03b) | new-route (verbatim move) | CRUD/transform | `web/src/routes/+page.svelte` (moved verbatim) | exact (it IS the body) |
| `web/src/routes/+page.ts` (`/` → interim) | redirect-stub | request-response | `web/src/routes/+layout.ts` (`ssr`/`prerender` export shape) | role-match (mechanism net-new) |
| `web/src/routes/wantlist/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/routes/notifications/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/routes/account/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/routes/char-meta/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/routes/my-characters/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/routes/admin/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/routes/bank-coin/+page.ts` | redirect-stub | request-response | `web/src/routes/+layout.ts` | role-match |
| `web/src/lib/components/SiteShell.svelte` (rework header → 5-tab strip + badge relocation) | layout-chrome | event-driven | `web/src/routes/+page.svelte` `.view-nav`/`.tab` (active-state idiom) + self (existing `.char-meta-nav`/badge) | exact (in-repo idiom) |
| `web/src/lib/components/SettingsMenu.svelte` (dissolve → identity+SignOut only) | layout-chrome | event-driven | self (prune to identity header + Sign out) | exact (subset of current) |
| Theme-context bridge (`+layout.svelte`/`SiteShell.svelte` setContext + Settings ThemePicker getContext) | store-rewire | event-driven | `web/src/lib/components/AuthGate.svelte` `SESSION_KEY` context idiom | role-match |
| Unread badge relocation (`SiteShell` render site → Wishlist tab `<a>`) | store-rewire | event-driven | `web/src/lib/components/SiteShell.svelte` lines 42-60, 80-85, 177-192 | exact (move render only) |
| Link repoints (`+page.svelte`, `bank-coin`, `char-meta`, `my-characters`, `NotificationPrefsPanel`) | store-rewire | — | grep table below | exact |

---

## Pattern Assignments

### `web/src/routes/{characters,inventory,banks}/+page.svelte` (new-route, placeholder)

**Analog:** `web/src/routes/account/+page.svelte` — the canonical thin `+page.svelte` that mounts ONE thing inside a `.form-card` with `<svelte:head><title>`. The 3 placeholders are even thinner (no component, a dashed-border card per UI-SPEC §E). Mirror the wrapper shape + the scoped-style `.form-title`/`.form-purpose` rhythm.

**Thin-wrapper shape** (`account/+page.svelte:13-26`):
```svelte
<svelte:head>
	<title>SquireBot — your watcher codes</title>
</svelte:head>

<section class="form-card">
	<header class="account-intro">
		<h1 class="form-title">Your watcher codes</h1>
		<p class="form-purpose">A watcher code links a PC to your account…</p>
	</header>
	<WatcherCodesPanel />
</section>
```

**Title/purpose CSS to reuse verbatim** (`account/+page.svelte:44-56`): `.form-title` = display 20px Heading; `.form-purpose` = body 16px @ 0.85 opacity. The placeholder card itself is the UI-SPEC §E dashed idiom (`border: 1px dashed var(--border)`, `padding: 48px 24px`, `text-align: center`).

**D-03b load-bearing link** (UI-SPEC §E.3 + Copywriting table): each of the 3 placeholders MUST carry a real link to the preserved `/guild-views`. Style it like the existing **`.record-coin`** affordance in `+page.svelte:386` (`<a class="record-coin" href="/bank-coin">Record coin</a>`) — accent-bordered, 44px, uppercase 13px. Exact copy strings from UI-SPEC Copywriting table ("View the current inventory →" / "View the current bank →").

**Do NOT add a search bar** (D-08 / UI-SPEC §D): stub tabs get NO inert search.

---

### `web/src/routes/wishlist/+page.svelte` (new-route, rehome — D-03/D-07)

**Analog:** `web/src/routes/wantlist/+page.svelte` (the WantlistPanel snippet idiom) + `web/src/routes/notifications/+page.svelte` (the prefs+divider+inbox stack). Compose BOTH bodies onto one route.

**Wantlist body to rehome** (`wantlist/+page.svelte:22-30`) — note the intro is passed as a **snippet child** into `WantlistPanel` (so the page's scoped styles apply), grids break out to full width:
```svelte
<WantlistPanel>
	<header class="wantlist-intro">
		<h1 class="form-title">Your wantlist</h1>
		<p class="form-purpose">Track the items you're after…</p>
	</header>
</WantlistPanel>
```

**Notifications body to rehome** (`notifications/+page.svelte:20-34`) — prefs + divider + inbox in one `.form-card`:
```svelte
<section class="form-card">
	<header class="notify-intro"><h1 class="form-title">Notifications</h1>…</header>
	<NotificationPrefsPanel />
	<div class="divider"></div>
	<NotificationInbox />
</section>
```

**Composition note** (UI-SPEC §G): wantlist surface on top, then a `"Notifications"` section heading + the prefs/inbox stack. `NotificationInbox` already calls `refreshUnread()` after a mark-read (per `unread.ts:3-5` header) — the relocated Wishlist-tab badge stays authoritative with **no extra wiring**. Per-item ping = the existing wantlist bell only (Phase 34 owns the rework — A3).

---

### `web/src/routes/settings/+page.svelte` (new-route, compose — D-05/D-06, NAV-03)

**Analog:** `web/src/routes/admin/+page.svelte` — it ALREADY reads the session from context, derives `isOfficer`, and stacks the 4 admin panels as `.form-card` sections. The Settings page is the same pattern, extended to also mount the 4 member panels (Theme via context-bridge, WatcherCodes, CharMeta, MyCharacters) as sections, with the 4 admin panels behind the officer gate.

**Officer-gate idiom — copy verbatim** (`admin/+page.svelte:20-30`):
```svelte
import { getContext } from 'svelte';
import { SESSION_KEY, type SessionGetter } from '$lib/components/AuthGate.svelte';
const getSession = getContext<SessionGetter>(SESSION_KEY);
let session = $derived(getSession ? getSession() : null);
let isOfficer = $derived(!!session?.isOfficer);
```

**Section stack + gate** (`admin/+page.svelte:40-61`) — the 4 officer panels are already imported and stacked here; lift the whole `{:else}` block into a `{#if isOfficer}` Admin section on Settings:
```svelte
<section class="form-card"><h2 class="form-title">Monitors</h2><MonitorAdminPanel /></section>
<section class="form-card"><h2 class="form-title">Character assignments</h2><AssignmentAdminPanel /></section>
```
Imports already present at `admin/+page.svelte:22-25`: `EvictionForm`, `AdminMgmtForm`, `MonitorAdminPanel`, `AssignmentAdminPanel`.

**The member-section inventory comes straight from the dissolving gear** (`SettingsMenu.svelte:190-198`) — these 4 links become 4 in-page sections (NOT links):
```svelte
<a href="/account" …>Watcher codes</a>        → <WatcherCodesPanel /> section
<a href="/char-meta" …>Set class &amp; level</a> → <CharMetaForm /> section
<a href="/my-characters" …>My characters</a>   → <MyCharactersPanel /> section
{#if session?.isOfficer}<a href="/admin" …>Admin</a>{/if} → the officer-gated section above
```

**Section card CSS** to mirror: `admin/+page.svelte:64-93` (`.admin-area` gap 24px lg; `.form-card` max-width 720px, panel bg, 6px radius; `.form-title` display 20px). Each section gets a stable `id` (e.g. `id="settings-theme"`) for D-02 deep-link landing + the settings-search jump (UI-SPEC §F/F2).

**Settings search** (UI-SPEC §F2, D-05 — Claude's discretion → live in-page section filter): a single text input over a fixed keyword list per section title; empty query = all sections. Reuse the search-input visual from UI-SPEC §D. Purely client-side, no new state machinery.

> **MonitorAdminPanel stays in Settings→Admin** (officer config), NOT on Wishlist — only `NotificationPrefsPanel`/`NotificationInbox` (member's own prefs) move to Wishlist (RESEARCH §5 note + A4).

---

### `web/src/routes/guild-views/+page.svelte` (interim, D-03b/D-04)

**Analog:** `web/src/routes/+page.svelte` — **moved verbatim**. It is the ~600-line consolidated 4-view home (Inventory/Gear/Spell/Bank + `SearchBox` + My/Guild scope). RESEARCH §3: move, don't embed (embedding triples-mounts the single-SearchBox/single-fetch model). Keep its `onMount` `?view=` seed intact (`+page.svelte:164-170`) so `/guild-views?view=bank` deep-links keep working:
```svelte
onMount(() => {
	const v = new URLSearchParams(window.location.search).get('view');
	if (v && TABS.some((t) => t.id === v)) { active = v as ViewId; }
	void load();
});
```
Do NOT delete the original (Deferred list — retired only when Phases 31-33 land). Label it "Guild views (classic)" or similar in the UI (A1 — naming is cosmetic, planner/UI-SPEC may pick another path name).

---

### Redirect stubs (D-02) — `web/src/routes/{wantlist,notifications,account,char-meta,my-characters,admin,bank-coin}/+page.ts` + `web/src/routes/+page.ts`

**Analog:** `web/src/routes/+layout.ts` — the only in-repo source of the `export const ssr/prerender` shape these stubs need. The `redirect(308,…)` call itself is net-new (no existing example), but it is a one-liner verified against the installed `@sveltejs/kit` 2.61.1 (RESEARCH §1).

**Existing export shape to mirror** (`+layout.ts:6-7`):
```ts
export const ssr = false;
export const prerender = false;
```

**The redirect stub (one per old path)** — from RESEARCH "Code Examples":
```ts
// web/src/routes/account/+page.ts
import { redirect } from '@sveltejs/kit';
export const ssr = false;
export const prerender = false;
export function load() {
	redirect(308, '/settings');
}
```
**Redirect mapping** (RESEARCH §1 table): `/wantlist`→`/wishlist`, `/notifications`→`/wishlist`, `/account`/`/char-meta`/`/my-characters`/`/admin`/`/bank-coin`→`/settings`. After adding each stub, **delete the old `+page.svelte` body** (its mount moves to the new home).

**`/` default-landing redirect** — REPLACES the current `+page.ts:10` `export const prerender = true;` (RESEARCH "Code Examples", D-04):
```ts
import { redirect } from '@sveltejs/kit';
export const ssr = false;
export const prerender = false;
export function load() {
	redirect(307, '/guild-views'); // 307 temporary — flips to /characters post-Phase-31
}
```
**Anti-patterns (RESEARCH §1 / Pitfall 2):** keep `prerender = false`; do NOT wrap `redirect()` in try/catch (it throws and the catch swallows it); status must be 300-308. A2 caveat: browser-smoke a hard refresh of `https://squirebot.quest/` — if the apex 404s after dropping `/`'s `prerender=true`, restore a minimal prerendered `/` whose only job is the redirect.

---

### `web/src/lib/components/SiteShell.svelte` (layout-chrome — the 5-tab strip + badge)

**Analog (active-state idiom):** `web/src/routes/+page.svelte` lines 315-327 (the in-page `.view-nav`/`.tab` markup) + 407-440 (its CSS). The new top-level strip uses **`<a href>`** (D-01/UI-SPEC §B — real routes) instead of `<button>`, with `$page.url.pathname` driving active state instead of a client `active` var.

**The proven `.tab` active markup to adapt** (`+page.svelte:315-327`):
```svelte
<nav class="view-nav" aria-label="Views">
	{#each TABS as tab (tab.id)}
		<button class="tab" class:active={active === tab.id} type="button"
			aria-current={active === tab.id ? 'page' : undefined}
			onclick={() => (active = tab.id)}>{tab.label}</button>
	{/each}
</nav>
```
→ becomes (RESEARCH §2, `<a>` + path-derived):
```svelte
import { page } from '$app/stores';   // repo convention — NOT $app/state
let path = $derived($page.url.pathname);
function isActive(href: string) { return path === href || path.startsWith(href + '/'); }
…
<a href={t.href} class="tab" class:active={isActive(t.href)}
   aria-current={isActive(t.href) ? 'page' : undefined}>{t.label}</a>
```

**The `.tab` CSS to copy verbatim** (`+page.svelte:407-440`): `.view-nav` flex + 1px `--border` bottom rule; `.tab` 44px min-height, 8px/16px padding, display 13px uppercase 0.08em, `border-bottom: 2px solid transparent`, `opacity: 0.7`; `.tab.active` accent color + `border-bottom-color: var(--accent)` + opacity 1; `.tab:focus-visible` 2px accent outline **`outline-offset: -2px`** (inset, so it doesn't clip under the strip rule — UI-SPEC §B).

**`SiteShell` already owns the active-tab inputs:** it imports `page` (`SiteShell.svelte:12`) and reads `$page.url?.pathname` for the badge `$effect` (`:55`) — so it's the natural strip owner. The strip only shows when `session?.authenticated` (the existing guard at `:67`).

**REMOVE the old header nav** (`SiteShell.svelte:74-90`): the `Inventory`/`Wantlist`/`Notifications` `.char-meta-nav` links + the `<SettingsMenu>` mount. Replace with the 5-tab strip (+ Wishlist badge, below) and the pruned identity affordance (next section). The existing `.char-meta-nav` CSS (`:145-168`) is the same `.tab` idiom and can seed the strip styles.

---

### Unread badge relocation (store-rewire — NAV-04/D-07)

**Analog:** `web/src/lib/components/SiteShell.svelte` — the badge logic STAYS in SiteShell (it's the shell); only the **render location** moves from the Notifications nav link onto the Wishlist tab `<a>`. The store (`unread.ts`) is untouched (module-level `writable`; do NOT duplicate or prop-drill — RESEARCH §4 gotcha).

**Keep these derivations in SiteShell verbatim** (`SiteShell.svelte:42-60`):
```svelte
let count = $derived($unreadCount);
let badgeText = $derived(count > 9 ? '9+' : String(count));
let notifyLabel = $derived(count > 0 ? `Notifications, ${count} unread` : 'Notifications');
onMount(() => { if (session?.authenticated) void refreshUnread(); });
// $effect re-fetches on every route change keyed off $page.url.pathname
```
> Re-label `notifyLabel` to the UI-SPEC §B2 Wishlist phrasing: `count > 0 ? `Wishlist, ${count} unread` : 'Wishlist'`.

**Current badge render site to MOVE** (`SiteShell.svelte:80-85`) — delete from the Notifications nav link:
```svelte
<a href="/notifications" class="char-meta-nav notify-nav" aria-label={notifyLabel}>
	Notifications
	{#if count > 0}<span class="unread-badge" aria-hidden="true">{badgeText}</span>{/if}
</a>
```
**New render site** — on the Wishlist tab entry (RESEARCH "Code Examples" / UI-SPEC §B2):
```svelte
<a href="/wishlist" class="tab notify-tab" class:active={isActive('/wishlist')}
   aria-current={isActive('/wishlist') ? 'page' : undefined}
   aria-label={count > 0 ? `Wishlist, ${count} unread` : 'Wishlist'}>
	Wishlist
	{#if count > 0}<span class="unread-badge" aria-hidden="true">{badgeText}</span>{/if}
</a>
```
**The `.unread-badge` + `.notify-nav` CSS to reuse verbatim** (`SiteShell.svelte:169-192`): `.notify-nav { position: relative; }`; `.unread-badge` = min-width 18px, padding 0 4px, display 13px/600, line-height 18px, `font-variant-numeric: tabular-nums`, `color: var(--bg)`, `background: var(--accent)`, `border-radius: 9px`. UI-SPEC §B2 renders it **inline** (`margin-left: 6px`) rather than the absolute top-right; both are token-faithful.

---

### Theme-context bridge (store-rewire — D-06, the ONE non-trivial wiring task)

**Problem (Pitfall 3):** `ThemePicker` currently rides the `bind:theme` prop chain `+layout → SiteShell → SettingsMenu → ThemePicker`. Moving it into `/settings` breaks that — a `{@render children()}` page can't receive `bind:theme` as a prop.

**Current bind chain to understand:**
- `+layout.svelte:18-26` — owns `theme = $state(loadTheme())` + the **single** `$effect(() => applyTheme(theme, rootEl))` writer; `:36-42` writes `data-theme={theme}` on `.theme-root` and passes `<SiteShell bind:theme>`.
- `SiteShell.svelte:22-25` — `let { theme = $bindable(), children } = $props();` then `<SettingsMenu bind:theme {session} />` (`:90`).
- `SettingsMenu.svelte:73` — `let { theme = $bindable(), session } = $props();` → `<ThemePicker bind:theme />` (`:182-184`).
- `ThemePicker.svelte:9` — `let { theme = $bindable() } = $props();` → `<select bind:value={theme}>`.

**Fix — a tiny theme context (RESEARCH §5a, recommended):** mirror the existing `SESSION_KEY` idiom in `AuthGate.svelte:1-13` / `:58`:

*AuthGate's pattern to copy* (`AuthGate.svelte:6` + `:58`):
```ts
export const SESSION_KEY = Symbol('session');
export type SessionGetter = () => Session | null;
// …
setContext(SESSION_KEY, (() => session) satisfies SessionGetter);
```
*Apply it for theme:* in `+layout.svelte` (or `SiteShell`), `setContext(THEME_KEY, { get: () => theme, set: (v) => (theme = v) })`; the Settings `ThemePicker` does `getContext(THEME_KEY)` and reads/writes through it. The single `$effect(applyTheme)` in `+layout` stays the only `[data-theme]` writer. Settings consumer reads session-style via `getContext` exactly as `admin/+page.svelte:28-30` reads `getSession`.

> Keep the picker itself unchanged where possible — `ThemePicker.svelte` exposes `theme = $bindable()`; a thin Settings-side wrapper can bridge the context accessor to that bindable so `ThemePicker` need not change.

---

### `web/src/lib/components/SettingsMenu.svelte` (layout-chrome — dissolve to identity+SignOut)

**Analog:** itself — prune to a subset. D-06: the gear's config items become Settings sections; ONLY the identity header + Sign out stay top-right (UI-SPEC §C). The display logic is already pure + testable.

**Keep the identity render (T-15-22 XSS invariant — load-bearing, copy verbatim)** (`SettingsMenu.svelte:167-179`):
```svelte
<div class="identity">
	{#if avatarUrl}
		<img class="avatar" src={avatarUrl} alt={session.username} width="28" height="28" />
	{:else}
		<span class="avatar avatar-fallback" aria-hidden="true"><UserIcon size={16} /></span>
	{/if}
	{#if session?.isOfficer}<Shield size={14} aria-label="Officer" class="officer-badge" />{/if}
	<span class="username">{session.username}</span>   <!-- plain {} ONLY — never raw HTML -->
</div>
```
**Keep `avatarUrlFor` (the testable helper)** (`SettingsMenu.svelte:29-33`) and the **Sign out flow** (`:101-106` + `:204-213`):
```ts
async function signOut() { if (signingOut) return; signingOut = true; await logout(); window.location.href = '/'; }
```
**Keep the dropdown a11y mechanics** (`SettingsMenu.svelte:92-145`): `menuKeyAction`/Escape→close+restore-focus, outside-pointerdown close (`:110-120`), route-change close (`:124-130`), focus-first-item on open (`:134-140`), `aria-haspopup`/`aria-expanded`/`aria-controls` (`:153-156`), `role="menu"`/`role="menuitem"`.

**DELETE from the menu** (`SettingsMenu.svelte:181-198`): the `ThemePicker` block (`:182-184` — moves to Settings via context) and the 4 nav links (`:190-197` — become Settings sections). UI-SPEC §C: do NOT add a redundant "Settings" item (one place per concept — the tab strip is the canonical route in). Swap the trigger glyph from gear → avatar+username+`ChevronDown` per UI-SPEC §C.

---

## Shared Patterns

### Session-context officer gate (Layer-1 UX suppression)
**Source:** `web/src/lib/components/AuthGate.svelte:6` (`SESSION_KEY`) + `web/src/routes/admin/+page.svelte:20-30`
**Apply to:** the Settings→Admin section (`{#if isOfficer}`), and read for the identity affordance.
```svelte
import { getContext } from 'svelte';
import { SESSION_KEY, type SessionGetter } from '$lib/components/AuthGate.svelte';
const getSession = getContext<SessionGetter>(SESSION_KEY);
let session = $derived(getSession ? getSession() : null);
let isOfficer = $derived(!!session?.isOfficer);
```
The server (`webadmin`) re-checks officer on every admin write — the hidden section is **never** the boundary (`admin/+page.svelte:6-15` comment). A non-officer who bookmarked `/admin` redirects to `/settings`, where the Admin section simply isn't rendered.

### Thin route → single component (the repo's page idiom)
**Source:** `web/src/routes/account/+page.svelte` (all leaf routes follow it)
**Apply to:** all 6 new routes + the interim route.
Every leaf is a `+page.svelte` that sets `<svelte:head><title>`, wraps content in a `.form-card` (`max-width: 720px; padding: 24px; background: var(--panel); border: 1px solid var(--border); border-radius: 6px`), with `.form-title` (display 20px) + `.form-purpose` (body 16px @ 0.85). Forms narrow (720px), grids break to full width.

### `[data-theme]` single-writer chain
**Source:** `web/src/routes/+layout.svelte:18-26` (the `$effect(applyTheme)` writer) + `:36`
**Apply to:** preserve intact; relocate ONLY the picker via the theme context above. NEVER add a second `[data-theme]` writer.
```svelte
let theme = $state<ThemeKey>(loadTheme());
$effect(() => { applyTheme(theme, rootEl ?? null); });   // the ONE writer
```

### EQ theme tokens + 44px + focus-visible (every nav affordance)
**Source:** `web/src/lib/components/SiteShell.svelte:145-168` (`.char-meta-nav`) + `web/src/routes/+page.svelte:413-440` (`.tab`)
**Apply to:** every new tab/link/search-input/identity-trigger. Tokens ONLY (`--accent`/`--panel`/`--text`/`--border`/`--font-display`); `min-height: 44px`; `:focus-visible { outline: 2px solid var(--accent) }` (tabs use `outline-offset: -2px`). No literal hex, no new palette (re-skin is out of scope).

### Internal-link repoints (clean canonical paths, not redirect bounces)
**Source (grep `href="/` across `web/src`):** RESEARCH "Runtime State Inventory". After the routes move, repoint these so internal nav targets the **new canonical** path (redirect stubs are for external bookmarks only):

| File:line | Current `href` | Repoint to |
|-----------|----------------|------------|
| `web/src/routes/+page.svelte:309` (moves to `/guild-views`) | `/my-characters` | `/settings` (My-characters section) |
| `web/src/routes/+page.svelte:386` (moves to `/guild-views`) | `/bank-coin` | `/settings` **or** keep `BankCoinForm` reachable; "Record coin" round-trip → `/guild-views?view=bank` |
| `web/src/routes/bank-coin/+page.svelte:28` (`BankCoinForm` rehomed) | `/?view=bank` | `/guild-views?view=bank` (Pitfall 5) |
| `web/src/routes/char-meta/+page.svelte:28` (→ Settings section) | `/my-characters` | `/settings` section / in-page |
| `web/src/routes/my-characters/+page.svelte:25` (→ Settings section) | `/char-meta` | `/settings` section / in-page |
| `web/src/lib/components/NotificationPrefsPanel.svelte:166` | `/wantlist` | `/wishlist` |
| `web/src/lib/components/SiteShell.svelte:65` (wordmark) | `/` | keep `/` (still home → redirects to interim; fine) |
| `web/src/lib/components/SiteShell.svelte:74,75,80` (nav links) | `/`, `/wantlist`, `/notifications` | become the 5-tab strip (removed) |
| `web/src/lib/components/SettingsMenu.svelte:190-197` | `/account`,`/char-meta`,`/my-characters`,`/admin` | become Settings sections (removed) |

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `redirect(308,…)` body of each `+page.ts` stub | redirect-stub | request-response | No existing `redirect()` call in the repo (all current `+page.ts`/`+layout.ts` only set `ssr`/`prerender`). The call is a verified one-liner (`@sveltejs/kit` 2.61.1, RESEARCH §1) — net-new but trivial; the export-shape analog (`+layout.ts`) covers the rest. |
| Settings live-section-filter search | new logic | transform | No in-page section-filter exists yet (the home `SearchBox` filters *rows*, not sections). Net-new but purely client-side string match over a fixed section list (UI-SPEC §F2) — no analog needed beyond the search-input visual (UI-SPEC §D). |
| Mobile horizontal-scroll tab strip + `scrollIntoView` active-tab (UI-SPEC §H) | layout-chrome | event-driven | No horizontal-scroll nav precedent (the header `flex-wrap`s). Net-new CSS (`overflow-x: auto; scroll-snap-type`) + a reduced-motion-aware `scrollIntoView`; follow UI-SPEC §H. |

---

## Metadata

**Analog search scope:** `web/src/routes/**` (all `+page.svelte`/`+page.ts`/`+layout.*`), `web/src/lib/components/*.svelte`, `web/src/lib/stores/unread.ts`
**Files scanned:** 13 read in full + 1 grep over `web/src/**/*.svelte` for internal links + 2 targeted reads of `web/src/routes/+page.svelte` (the ~600-line home)
**Key sources:** `30-RESEARCH.md` §"Decision Recommendations" (mechanisms verified against installed SvelteKit 2.61.1), `30-UI-SPEC.md` §"Layout & Interaction Contract" (the structural contract)
**Pattern extraction date:** 2026-06-17

**Browser-smoke gate (carried from RESEARCH Pitfall 1 / UI-SPEC Build Notes — load-bearing):** node vitest is DOM-blind; green tests ≠ working routes. A deployed browser-smoke is MANDATORY: all 5 tabs reachable + `aria-current` tracks the route, every old bookmark redirects with no 404, Wishlist badge reflects `$unreadCount`, Settings sections render + search works, Admin section hidden for a non-officer, theme switch from Settings still writes `[data-theme]`, `/` lands on the preserved views, and the shell renders under all 5 themes (Heavy + Minimalist contrast). Localhost can't auth against prod — smoke on prod after deploy OR a full local stack.
