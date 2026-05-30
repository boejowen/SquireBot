<script lang="ts">
	// SiteShell — the app chrome (14-UI-SPEC Design System component inventory).
	// Carries the SINGLE [data-theme] attribute on its root element; a theme swap
	// is one attribute write + a localStorage persist (applyTheme), no rebuild,
	// no per-component re-theming (WEB-05 / D-06). Header = wordmark (Display
	// 28px) + ThemePicker. <main> renders the page (the view nav + grids live in
	// +page, coupled to the DataGrids). Footer carries the required P1999 wiki
	// CC-BY-SA attribution (UI-SPEC Copywriting). prefers-reduced-motion makes
	// theme transitions instant (handled globally in app.css).

	import ThemePicker from './ThemePicker.svelte';
	import type { ThemeKey } from '$lib/theme/themes';

	// The active theme is owned by +layout.svelte (which seeds it via loadTheme
	// and writes the [data-theme] attribute + persists via applyTheme on every
	// change). The shell binds it so the ThemePicker can mutate the single source
	// of truth.
	let {
		theme = $bindable(),
		children
	}: { theme: ThemeKey; children: import('svelte').Snippet } = $props();
</script>

<div class="site-shell">
	<header class="shell-header">
		<span class="wordmark">SquireBot</span>
		<ThemePicker bind:theme />
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
