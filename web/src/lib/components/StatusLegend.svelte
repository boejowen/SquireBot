<script lang="ts">
	// StatusLegend — the compact OK / MISSING / OTHER (or KNOWN / MISSING) swatch
	// + word legend shown near the gear_check and spell_check grids (14-UI-SPEC
	// Grid Contract "Status legend") so a first-time viewer reads the color
	// encoding without guessing. Words back every swatch (color is never the only
	// signal — a11y).
	let { variant }: { variant: 'gear' | 'spell' } = $props();

	const ITEMS =
		// gear_check: OK / MISSING / OTHER; spell_check: KNOWN / MISSING.
		// Tokens mirror StatusCell.
		{
			gear: [
				{ label: 'OK', token: 'var(--status-ok)' },
				{ label: 'MISSING', token: 'var(--status-missing)' },
				{ label: 'OTHER', token: 'var(--status-other)' }
			],
			spell: [
				{ label: 'KNOWN', token: 'var(--status-ok)' },
				{ label: 'MISSING', token: 'var(--status-missing)' }
			]
		} as const;
</script>

<ul class="legend" aria-label="Status legend">
	{#each ITEMS[variant] as item (item.label)}
		<li class="legend-item">
			<span class="swatch" style:background={item.token} aria-hidden="true"></span>
			<span class="word">{item.label}</span>
		</li>
	{/each}
</ul>

<style>
	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		list-style: none;
		padding: 0;
		margin: 0;
	}
	.legend-item {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.swatch {
		width: 12px;
		height: 12px;
		border-radius: 3px;
		flex: 0 0 auto;
	}
	.word {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		color: var(--text);
	}
</style>
