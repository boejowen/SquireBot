<script lang="ts">
	// Root layout = the SiteShell wrapper that carries the SINGLE [data-theme]
	// attribute (WEB-05 / D-06). Owns the active-theme state: seeds it from
	// localStorage via loadTheme (velious default when unstored), and on every
	// change writes the attribute + persists via applyTheme — one attribute
	// write, no rebuild, no per-component re-theming. SiteShell renders the
	// chrome (wordmark + ThemePicker + footer) and binds `theme` so the picker
	// mutates this single source of truth.
	import { setContext } from 'svelte';
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import SiteShell from '$lib/components/SiteShell.svelte';
	import AuthGate from '$lib/components/AuthGate.svelte';
	import { loadTheme, applyTheme, type ThemeKey } from '$lib/theme/themes';
	import { THEME_KEY, type ThemeContext } from '$lib/theme/themeContext';

	let { children } = $props();

	// velious default when no stored pref (D-06); loadTheme is SSR-safe.
	let theme = $state<ThemeKey>(loadTheme());

	// The themed root element; applyTheme writes [data-theme] here + persists.
	let rootEl: HTMLElement | undefined = $state();

	// Apply on mount and on every theme change (the single [data-theme] write).
	$effect(() => {
		applyTheme(theme, rootEl ?? null);
	});

	// Theme-context bridge (Phase 30 / D-06): the ThemePicker now lives in the
	// Settings tab, not the dissolved header gear. A {@render children()} page can't
	// receive bind:theme as a prop, so expose a get/set accessor over THIS single
	// `theme` $state. The $effect above stays the ONLY [data-theme] writer — the
	// relocated picker just calls set(), which mutates `theme` and re-runs the effect.
	setContext(THEME_KEY, { get: () => theme, set: (v) => (theme = v) } satisfies ThemeContext);
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<!-- The [data-theme] root stays OUTERMOST so the themed shell (wordmark +
     ThemePicker + footer) shows on the login / not-member screens too. AuthGate
     wraps the page content inside the shell: it provides the session via
     context (SiteShell reads it for the header SettingsMenu: identity + the
     officer-only Admin item) and renders the pre-auth screens or the app (D-01). -->
<div class="theme-root" data-theme={theme} bind:this={rootEl}>
	<AuthGate>
		<SiteShell bind:theme>
			{@render children()}
		</SiteShell>
	</AuthGate>
</div>

<style>
	.theme-root {
		display: contents;
	}
</style>
