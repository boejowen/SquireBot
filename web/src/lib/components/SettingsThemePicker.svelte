<script lang="ts">
	// SettingsThemePicker — the context→bindable BRIDGE (Phase 30 / D-06, RESEARCH §5a).
	// The ThemePicker moved out of the dissolved header gear and into the Settings tab.
	// A `{@render children()}` page can't receive `bind:theme` as a prop the way the old
	// SettingsMenu did, so this thin wrapper reads the get/set accessor +layout exposes
	// via THEME_KEY context (mirrors AuthGate's SESSION_KEY idiom) and bridges it to
	// ThemePicker's unchanged $bindable `theme` prop.
	//
	// SINGLE-WRITER INVARIANT (Pitfall 3 / T-30-07): this bridge MUST NOT write the
	// [data-theme] attribute. It only MUTATES +layout's single `theme` $state through the
	// context setter; the lone theme-applying $effect in +layout stays the ONE attribute
	// writer. There is deliberately no theme-apply helper imported here — a second writer
	// could drift the attribute.
	import { getContext } from 'svelte';
	import { THEME_KEY, type ThemeContext } from '$lib/theme/themeContext';
	import ThemePicker from './ThemePicker.svelte';
	import type { ThemeKey } from '$lib/theme/themes';

	const themeCtx = getContext<ThemeContext>(THEME_KEY);

	// Seed the local bindable from the single source, then write THROUGH the context
	// setter on every change (the write-through to +layout's `theme` $state).
	let theme = $state<ThemeKey>(themeCtx.get());
	$effect(() => {
		themeCtx.set(theme);
	});
</script>

<ThemePicker bind:theme />
