<script lang="ts">
	// Root layout = the SiteShell wrapper that carries the SINGLE [data-theme]
	// attribute (WEB-05 / D-06). Owns the active-theme state: seeds it from
	// localStorage via loadTheme (velious default when unstored), and on every
	// change writes the attribute + persists via applyTheme — one attribute
	// write, no rebuild, no per-component re-theming. SiteShell renders the
	// chrome (wordmark + ThemePicker + footer) and binds `theme` so the picker
	// mutates this single source of truth.
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import SiteShell from '$lib/components/SiteShell.svelte';
	import { loadTheme, applyTheme, type ThemeKey } from '$lib/theme/themes';

	let { children } = $props();

	// velious default when no stored pref (D-06); loadTheme is SSR-safe.
	let theme = $state<ThemeKey>(loadTheme());

	// The themed root element; applyTheme writes [data-theme] here + persists.
	let rootEl: HTMLElement | undefined = $state();

	// Apply on mount and on every theme change (the single [data-theme] write).
	$effect(() => {
		applyTheme(theme, rootEl ?? null);
	});
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<div class="theme-root" data-theme={theme} bind:this={rootEl}>
	<SiteShell bind:theme>
		{@render children()}
	</SiteShell>
</div>

<style>
	.theme-root {
		display: contents;
	}
</style>
