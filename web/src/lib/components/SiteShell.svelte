<script lang="ts">
	// SiteShell — the app chrome (14-UI-SPEC Design System component inventory).
	// Carries the SINGLE [data-theme] attribute on its root element; a theme swap
	// is one attribute write + a localStorage persist (applyTheme), no rebuild,
	// no per-component re-theming (WEB-05 / D-06). Header = wordmark (Display
	// 28px) + ThemePicker. <main> renders the page (the view nav + grids live in
	// +page, coupled to the DataGrids). Footer carries the required P1999 wiki
	// CC-BY-SA attribution (UI-SPEC Copywriting). prefers-reduced-motion makes
	// theme transitions instant (handled globally in app.css).

	import { getContext } from 'svelte';
	import ThemePicker from './ThemePicker.svelte';
	import SessionIndicator from './SessionIndicator.svelte';
	import { SESSION_KEY, type SessionGetter } from './AuthGate.svelte';
	import type { ThemeKey } from '$lib/theme/themes';

	// The active theme is owned by +layout.svelte (which seeds it via loadTheme
	// and writes the [data-theme] attribute + persists via applyTheme on every
	// change). The shell binds it so the ThemePicker can mutate the single source
	// of truth.
	let {
		theme = $bindable(),
		children
	}: { theme: ThemeKey; children: import('svelte').Snippet } = $props();

	// The session comes from AuthGate via context. SiteShell only renders when the
	// gate has admitted an authenticated member (AuthGate shows the pre-auth
	// screens otherwise), so `session` is normally an authed member here — but we
	// guard defensively. The shell shows the SessionIndicator (AUTH-09) when
	// authenticated, and an Admin nav entry ONLY for an officer (Layer-1 UX
	// suppression; the server is the real gate — 15-03).
	const getSession = getContext<SessionGetter>(SESSION_KEY);
	let session = $derived(getSession ? getSession() : null);

	function goAdmin() {
		// Layer-1 UX nav only; /admin is built in 15-05. The server re-checks
		// officer status on every admin endpoint (15-03) — the hidden nav is never
		// the gate.
		window.location.href = '/admin';
	}
</script>

<div class="site-shell">
	<header class="shell-header">
		<a href="/" class="wordmark">SquireBot</a>
		<div class="shell-controls">
			{#if session?.isOfficer}
				<!-- Officer-only Admin nav (Layer-1 UX suppression; /admin lands in
				     15-05). A non-officer never sees this affordance — but the server
				     re-checks officer status on every admin endpoint (15-03). -->
				<button type="button" class="admin-nav" onclick={goAdmin}>Admin</button>
			{/if}
			<ThemePicker bind:theme />
			{#if session?.authenticated}
				<SessionIndicator {session} />
			{/if}
		</div>
	</header>

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
		text-decoration: none; /* it's an <a href="/"> home link — no underline (looks identical to the old span) */
	}
	.wordmark:focus-visible {
		/* Keyboard accessibility, consistent with .admin-nav. */
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.shell-controls {
		display: inline-flex;
		align-items: center;
		gap: 16px;
		flex-wrap: wrap;
	}
	/* Admin nav entry — styled like the +page view .tab (UI-SPEC). */
	.admin-nav {
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px 16px;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		cursor: pointer;
		opacity: 0.7;
	}
	.admin-nav:hover {
		opacity: 1;
		color: var(--accent);
	}
	.admin-nav:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
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
		.shell-footer {
			padding-left: 16px;
			padding-right: 16px;
		}
	}
</style>
