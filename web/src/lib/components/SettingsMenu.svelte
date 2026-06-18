<script lang="ts" module>
	// Pure, DOM-free decision helpers for the account menu, split out so they're
	// unit-testable in the node test project (the repo runs vitest under node with
	// NO jsdom and NO @testing-library; see ConfirmDialog.test.ts / auth.test.ts).
	// The .svelte instance below is a thin renderer that wires these to real DOM
	// events. (260607-sdh: gear settings dropdown; Phase 30 / D-06: DISSOLVED — the
	// gear's config items moved into the Settings tab; this menu is now identity +
	// Sign out only, triggered by an avatar + username + chevron affordance.)

	import type { Session } from '$lib/auth';

	/** A key event reduces to either closing the menu or ignoring the key. */
	export type MenuKeyAction = 'close' | 'ignore';

	/**
	 * Decide what a keydown means for the menu:
	 *   - Escape while open → 'close' (dismiss + restore focus to the trigger)
	 *   - anything else, or any key while closed → 'ignore'
	 */
	export function menuKeyAction(key: string, open: boolean): MenuKeyAction {
		if (!open) return 'ignore';
		if (key === 'Escape') return 'close';
		return 'ignore';
	}

	/**
	 * The Discord CDN avatar URL (PNG) for a session — only when BOTH the user id
	 * and the avatar hash are present, else null (→ a glyph fallback). Pulled out
	 * so the avatar derivation is unit-testable away from the DOM.
	 */
	export function avatarUrlFor(session: Session | null | undefined): string | null {
		if (!session) return null;
		if (!session.avatar || !session.discordUserId) return null;
		return `https://cdn.discordapp.com/avatars/${session.discordUserId}/${session.avatar}.png`;
	}
</script>

<script lang="ts">
	// SettingsMenu — the top-right "Joe ▾"-style account affordance (Phase 30 / D-06,
	// 30-UI-SPEC §C). The header gear DISSOLVED: Theme, Watcher codes, Set class &
	// level, My characters, and the officer-only Admin all moved into the Settings
	// tab (the canonical route in is the tab strip — there is intentionally no
	// "Settings" item here, "one place per concept"). What stays top-right is ONLY
	// the signed-in identity (avatar / officer shield / username) + Sign out, the web
	// convention users expect.
	//
	// A11y contract (modeled on ConfirmDialog § Accessibility — unchanged from the gear):
	//   - The trigger is a <button> with aria-haspopup="menu", aria-expanded bound
	//     to `open`, and aria-controls the panel id; aria-label includes the username.
	//   - The panel is role="menu"; its items are role="menuitem".
	//   - Escape closes the menu and RESTORES focus to the trigger.
	//   - An outside (pointerdown) click closes; a route change closes.
	//   - On open, focus moves to the first menu item.
	//   - Theme tokens only; 44px touch targets; focus-visible 2px accent outline.
	//
	// SECURITY (T-15-22 / T-30-01, carried verbatim): the username is user-controlled
	// (Discord). It renders ONLY via plain {} interpolation (Svelte auto-escapes) —
	// never the raw-HTML directive — and the avatar `alt` is the same escaped
	// username. A malicious display name is inert text, never a live tag.

	import { tick } from 'svelte';
	import { page } from '$app/stores';
	import UserIcon from '@lucide/svelte/icons/user';
	import Shield from '@lucide/svelte/icons/shield';
	import LogOut from '@lucide/svelte/icons/log-out';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { logout } from '$lib/auth';
	// `Session` is already imported in the <script module> block above (in scope here).

	// Phase 30 / D-06: the theme picker moved to Settings (via the theme context in
	// +layout), so this menu no longer carries a $bindable theme — props are the
	// session only.
	let { session }: { session: Session } = $props();

	let open = $state(false);
	let signingOut = $state(false);
	let triggerEl = $state<HTMLButtonElement>();
	let panelEl = $state<HTMLElement>();
	let rootEl = $state<HTMLElement>();

	// The avatar URL (null → glyph fallback). Derived via the testable helper.
	let avatarUrl = $derived(avatarUrlFor(session));

	function toggle() {
		open = !open;
	}

	function close() {
		open = false;
	}

	function onKeydown(e: KeyboardEvent) {
		if (menuKeyAction(e.key, open) === 'close') {
			e.preventDefault();
			open = false;
			// Escape returns focus to the trigger.
			triggerEl?.focus();
		}
	}

	async function signOut() {
		if (signingOut) return;
		signingOut = true;
		await logout();
		window.location.href = '/';
	}

	// Outside-click: while open, a document pointerdown outside the root closes the
	// menu. Cleaned up on teardown / when `open` flips.
	$effect(() => {
		if (!open) return;
		function onPointerDown(e: PointerEvent) {
			const target = e.target as Node | null;
			if (rootEl && target && !rootEl.contains(target)) {
				open = false;
			}
		}
		document.addEventListener('pointerdown', onPointerDown, true);
		return () => document.removeEventListener('pointerdown', onPointerDown, true);
	});

	// Close on route change (match SiteShell's lastPath idiom — no websocket).
	let lastPath = $state('');
	$effect(() => {
		const path = $page.url?.pathname ?? '';
		if (path !== lastPath) {
			if (lastPath !== '' && open) open = false;
			lastPath = path;
		}
	});

	// On open, move focus to the first menu item (or the panel). Guarded for SSR —
	// tick() resolves after the panel mounts in the browser.
	$effect(() => {
		if (!open) return;
		void tick().then(() => {
			const first = panelEl?.querySelector<HTMLElement>('[role="menuitem"]');
			(first ?? panelEl)?.focus();
		});
	});
</script>

<!-- Escape-to-close is window-scoped (menuKeyAction no-ops while closed), which
     avoids putting a keydown handler on a static, role-less wrapper. -->
<svelte:window onkeydown={onKeydown} />

<!-- The root scopes the outside-click test (rootEl.contains). -->
<div class="settings-menu" bind:this={rootEl}>
	<!-- The "Joe ▾" trigger (30-UI-SPEC §C): avatar (or User glyph fallback) +
	     escaped username + officer shield + chevron. The accessible name includes the
	     username. T-30-01: the username + alt render via plain {} ONLY. -->
	<button
		type="button"
		class="trigger"
		bind:this={triggerEl}
		aria-haspopup="menu"
		aria-expanded={open}
		aria-controls="settings-menu-panel"
		aria-label={`Account menu, ${session.username}`}
		onclick={toggle}
	>
		{#if avatarUrl}
			<img class="avatar" src={avatarUrl} alt={session.username} width="28" height="28" />
		{:else}
			<span class="avatar avatar-fallback" aria-hidden="true">
				<UserIcon size={16} />
			</span>
		{/if}
		{#if session?.isOfficer}
			<Shield size={14} aria-label="Officer" class="officer-badge" />
		{/if}
		<span class="username">{session.username}</span>
		<ChevronDown size={16} aria-hidden="true" class="caret" />
	</button>

	{#if open}
		<div id="settings-menu-panel" class="panel" role="menu" bind:this={panelEl} tabindex="-1">
			<!-- 1. Identity header. T-15-22 / T-30-01: the username renders via plain {}
			     ONLY (auto-escaped); the avatar alt is the same escaped username.
			     NEVER the raw-HTML directive. -->
			<div class="identity">
				{#if avatarUrl}
					<img class="avatar" src={avatarUrl} alt={session.username} width="28" height="28" />
				{:else}
					<span class="avatar avatar-fallback" aria-hidden="true">
						<UserIcon size={16} />
					</span>
				{/if}
				{#if session?.isOfficer}
					<Shield size={14} aria-label="Officer" class="officer-badge" />
				{/if}
				<span class="username">{session.username}</span>
			</div>

			<!-- 2. Divider. -->
			<hr class="divider" aria-hidden="true" />

			<!-- 3. Sign out (the only menu item now — D-06). Reuses the signingOut guard. -->
			<button
				type="button"
				role="menuitem"
				class="menu-link signout"
				onclick={signOut}
				disabled={signingOut}
			>
				<LogOut size={16} aria-hidden="true" />
				<span>Sign out</span>
			</button>
		</div>
	{/if}
</div>

<style>
	.settings-menu {
		position: relative;
		display: inline-flex;
		align-items: center;
	}
	/* The "Joe ▾" trigger — avatar + username + chevron (30-UI-SPEC §C). */
	.trigger {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 4px 12px;
		color: var(--text);
		background: none;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
		opacity: 0.85;
	}
	.trigger:hover {
		opacity: 1;
		color: var(--accent);
	}
	.trigger:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	:global(.caret) {
		flex: none;
		opacity: 0.7;
	}
	.panel {
		position: absolute;
		top: calc(100% + 8px);
		right: 0;
		z-index: 50;
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 220px;
		padding: 12px;
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
		font-family: var(--font-body);
	}
	.panel:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.identity {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 4px 8px 8px;
	}
	.avatar {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		object-fit: cover;
		border: 1px solid var(--border, var(--accent));
	}
	.avatar-fallback {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--bg);
		color: var(--text);
	}
	:global(.officer-badge) {
		color: var(--accent);
		flex: none;
	}
	.username {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label (UI-SPEC) */
		letter-spacing: 0.04em;
		color: var(--text);
	}
	.menu-link {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px 8px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-decoration: none;
		color: var(--text);
		background: none;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		opacity: 0.85;
		text-align: left;
	}
	.menu-link:hover {
		opacity: 1;
		color: var(--accent);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}
	.menu-link:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.signout:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.divider {
		height: 1px;
		margin: 4px 0;
		border: none;
		background: var(--border, var(--accent));
		opacity: 0.5;
	}
</style>
