<script lang="ts">
	// SiteShell — the app chrome (Phase 30 / 30-UI-SPEC §A). Carries the SINGLE
	// [data-theme] attribute on +layout's root element (the theme state lives in
	// +layout; this shell no longer threads it — the relocated ThemePicker reaches
	// it via the theme context). Header = wordmark (Display 28px) top-left + the
	// identity/Sign-out affordance top-right. UNDER the header sits the persistent
	// 5-tab strip (Characters · Inventory · Banks · Wishlist · Settings, NAV-01),
	// present on every authenticated route with the current tab marked
	// aria-current="page". The unread badge rides the Wishlist tab (NAV-04 / D-07).
	// <main> renders the page. Footer carries the P1999 wiki CC-BY-SA attribution.
	// prefers-reduced-motion makes transitions instant (handled globally in app.css).

	import { getContext, onMount } from 'svelte';
	import { page } from '$app/stores';
	import SettingsMenu from './SettingsMenu.svelte';
	import { SESSION_KEY, type SessionGetter } from './AuthGate.svelte';
	import { unreadCount, refreshUnread } from '$lib/stores/unread';

	let { children }: { children: import('svelte').Snippet } = $props();

	// The session comes from AuthGate via context. SiteShell only renders when the
	// gate has admitted an authenticated member (AuthGate shows the pre-auth screens
	// otherwise), so `session` is normally an authed member here — but we guard
	// defensively. The identity + Sign out live in the top-right SettingsMenu; the
	// configuration items it used to hold now live in the Settings tab (D-06).
	const getSession = getContext<SessionGetter>(SESSION_KEY);
	let session = $derived(getSession ? getSession() : null);

	// NAV-01: the five persistent top tabs, in the spec-fixed order. Real route
	// links (<a href>, NOT buttons — D-01) so each tab is deep-linkable + gets native
	// middle-click / open-in-new-tab / history; active state derives from the path.
	const TABS = [
		{ href: '/characters', label: 'Characters' },
		{ href: '/inventory', label: 'Inventory' },
		{ href: '/banks', label: 'Banks' },
		{ href: '/wishlist', label: 'Wishlist' },
		{ href: '/settings', label: 'Settings' }
	];
	let path = $derived($page.url?.pathname ?? '');
	// startsWith so deep links (e.g. a future /characters/<name>) keep the tab active.
	function isActive(href: string) {
		return path === href || path.startsWith(href + '/');
	}

	// Unread-alert badge (NAV-04 / D-07). The count is owner-scoped + server-truth
	// (the store's refreshUnread re-fetches; NotificationInbox also refreshes it after
	// a mark-read). We re-fetch on mount and on every route change (no websocket —
	// a load/route refresh is sufficient per the UI-SPEC). The badge moved off the
	// header onto the Wishlist tab; the store is read here, never duplicated.
	let count = $derived($unreadCount);
	// Abbreviate past 9 (guild scale — the UI-SPEC `9+` cap, NOT a "99+").
	let badgeText = $derived(count > 9 ? '9+' : String(count));

	onMount(() => {
		if (session?.authenticated) void refreshUnread();
	});

	// Re-fetch on navigation so the badge reflects reads done elsewhere.
	let lastPath = $state('');
	$effect(() => {
		const p = $page.url?.pathname ?? '';
		if (session?.authenticated && p !== lastPath) {
			lastPath = p;
			void refreshUnread();
		}
	});

	// Mobile (30-UI-SPEC §H): keep the active tab scrolled into view on the
	// horizontal-scroll strip so the user always sees where they are. Reduced-motion
	// is honored implicitly — we never request smooth scroll ('auto' only).
	let stripEl = $state<HTMLElement>();
	$effect(() => {
		// re-run when the path changes
		void path;
		const active = stripEl?.querySelector<HTMLElement>('.tab.active');
		active?.scrollIntoView({ inline: 'nearest', block: 'nearest', behavior: 'auto' });
	});
</script>

<div class="site-shell">
	<header class="shell-header">
		<span class="wordmark">SquireBot</span>
		<div class="shell-controls">
			{#if session?.authenticated}
				<!-- Top-right identity + Sign out (Phase 30 / D-06). The hidden-from-anon
				     affordance is UX; the server RequireSession gate is the real boundary. -->
				<SettingsMenu {session} />
			{/if}
		</div>
	</header>

	{#if session?.authenticated}
		<!-- The 5-tab strip: a sibling band UNDER the header, present on EVERY
		     authenticated route (NAV-01). The Wishlist tab hosts the relocated unread
		     badge (NAV-04). Real <a href> route links; aria-current marks the active
		     tab (the non-visual signal). -->
		<nav class="tab-strip" aria-label="Primary" bind:this={stripEl}>
			{#each TABS as t (t.href)}
				{#if t.href === '/wishlist'}
					<a
						href="/wishlist"
						class="tab notify-tab"
						class:active={isActive('/wishlist')}
						aria-current={isActive('/wishlist') ? 'page' : undefined}
						aria-label={count > 0 ? `Wishlist, ${count} unread` : 'Wishlist'}
					>
						Wishlist
						{#if count > 0}
							<span class="unread-badge" aria-hidden="true">{badgeText}</span>
						{/if}
					</a>
				{:else}
					<a
						href={t.href}
						class="tab"
						class:active={isActive(t.href)}
						aria-current={isActive(t.href) ? 'page' : undefined}>{t.label}</a
					>
				{/if}
			{/each}
		</nav>
	{/if}

	<main class="shell-main">
		{@render children()}
	</main>

	<footer class="shell-footer">
		<p>Item &amp; class data from the Project 1999 Wiki (CC-BY-SA) and PigParse.</p>
	</footer>
</div>

<style>
	.site-shell {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		background: var(--bg);
		color: var(--text);
		font-family: var(--font-body);
	}
	.shell-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		flex-wrap: wrap;
		padding: 16px 32px; /* xl gutters (UI-SPEC) */
		background: linear-gradient(var(--panel), var(--bg));
		border-bottom: 1px solid var(--border, var(--accent));
	}
	.wordmark {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 28px; /* Display (UI-SPEC Typography — wordmark only) */
		line-height: 1.2;
		color: var(--accent);
		letter-spacing: 0.02em;
	}
	.shell-controls {
		display: inline-flex;
		align-items: center;
		gap: 16px;
		flex-wrap: wrap;
	}
	/* The 5-tab strip (NAV-01 / 30-UI-SPEC §B). A panel-backdrop band with a 1px
	   bottom rule; the active tab's 2px accent border sits over that rule. Mobile
	   (§H): a single-row horizontally-scrollable strip, never a wrap or hamburger. */
	.tab-strip {
		display: flex;
		flex-wrap: nowrap;
		overflow-x: auto;
		gap: 4px;
		padding: 0 32px; /* xl gutters, matches the header */
		background: var(--panel);
		border-bottom: 1px solid var(--border, var(--accent));
		scroll-snap-type: x proximity;
		-webkit-overflow-scrolling: touch;
		scrollbar-width: thin;
	}
	/* A tab — the proven +page.svelte .tab idiom, as a route <a> (D-01). 13px display
	   uppercase, inactive at 0.7 opacity, active = accent label + 2px accent border. */
	.tab {
		display: inline-flex;
		align-items: center;
		flex: none;
		scroll-snap-align: start;
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px 16px;
		border-bottom: 2px solid transparent;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-decoration: none;
		cursor: pointer;
		white-space: nowrap;
		opacity: 0.7;
	}
	.tab:hover {
		opacity: 1;
		color: var(--accent);
	}
	/* Active (current route) tab in accent — reserved accent use (UI-SPEC §B). */
	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
		opacity: 1;
	}
	/* Inset focus ring so it doesn't clip under the strip's bottom rule (UI-SPEC §B). */
	.tab:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	/* The Wishlist tab hosts the unread badge on its baseline (UI-SPEC §B2). */
	.notify-tab {
		gap: 6px; /* before the inline badge */
	}
	/* Unread badge — a small accent-fill pill (accent bg, --bg text), tabular-nums,
	   ~9px radius. Rendered INLINE on the Wishlist tab (UI-SPEC §B2), not absolute.
	   The tab link (not the badge) carries the 44px hit target; the count is ALSO in
	   the link's aria-label (the non-color signal), so the pill is aria-hidden. */
	.unread-badge {
		min-width: 18px;
		margin-left: 6px;
		padding: 0 4px; /* xs inset */
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 18px;
		font-variant-numeric: tabular-nums;
		text-align: center;
		color: var(--bg);
		background: var(--accent);
		border-radius: 9px;
	}
	.shell-main {
		flex: 1;
		min-height: 0;
		padding: 32px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.shell-footer {
		padding: 24px 32px;
		border-top: 1px solid var(--border, rgba(74, 101, 133, 0.4));
		font-size: 13px;
		opacity: 0.75;
		text-align: center;
	}
	@media (max-width: 640px) {
		.shell-header,
		.shell-main,
		.shell-footer,
		.tab-strip {
			padding-left: 16px;
			padding-right: 16px;
		}
	}
</style>
