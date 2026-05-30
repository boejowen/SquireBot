<script lang="ts">
	// ThemePicker — a simple <select> over the 5 EQ themes (14-UI-SPEC; fancy
	// live-preview tiles are DEFERRED per 14-CONTEXT). The shell owns the
	// [data-theme] attribute: this picker just mutates the $bindable `theme`
	// state, and SiteShell's $effect writes the attribute + persists via
	// applyTheme. 44px touch target; the control + label use accent.
	import { THEME_KEYS, type ThemeKey } from '$lib/theme/themes';

	let { theme = $bindable() }: { theme: ThemeKey } = $props();

	// Friendly labels for the dropdown (keys are lowercase identifiers).
	const LABEL: Record<ThemeKey, string> = {
		velious: 'Velious',
		vanilla: 'Vanilla',
		kunark: 'Kunark',
		minimalist: 'Minimalist',
		heavy: 'Heavy'
	};
</script>

<label class="theme-picker">
	<span class="label">Theme</span>
	<select bind:value={theme} aria-label="Choose a theme">
		{#each THEME_KEYS as key (key)}
			<option value={key}>{LABEL[key]}</option>
		{/each}
	</select>
</label>

<style>
	.theme-picker {
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}
	.label {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
	}
	select {
		min-height: 44px;
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--accent);
		border-radius: 4px;
		cursor: pointer;
	}
	select:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
</style>
