<script lang="ts">
	// WantItemCell — the wantlist `Item` column cell. A catalog want (item_id
	// non-null) renders as an accent link + ItemTooltip trigger + a de-emphasized
	// `#<item_id>` (the ItemCell idiom). A custom want (item_id null, D-04) renders
	// a plain --text label + the neutral "Custom — won't trigger alerts" chip, with
	// NO tooltip and NO link. The item name / custom label render via plain {}
	// (Svelte auto-escapes) — NEVER {@html} (T-19-13 XSS boundary).
	import ItemTooltip from '../ItemTooltip.svelte';
	import type { WantlistRow } from '$lib/api';

	let { row }: { row: WantlistRow } = $props();

	const CUSTOM_TITLE =
		"Custom wants aren't matched against the item catalog, so the EC/WTS/raid monitors can't alert you about them.";
</script>

{#if row.item_id !== null}
	<span class="want-item">
		<ItemTooltip itemName={row.item_name} wikiUrl="">
			<span class="item-link">{row.item_name}</span>
		</ItemTooltip>
		<span class="item-id">#{row.item_id}</span>
	</span>
{:else}
	<span class="want-item">
		<span class="custom-label">{row.item_name}</span>
		<span class="custom-chip" title={CUSTOM_TITLE}>Custom — won't trigger alerts</span>
	</span>
{/if}

<style>
	.want-item {
		display: inline-flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 8px;
	}
	.item-link {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		font-family: var(--font-body);
	}
	.item-id {
		font-size: 13px;
		opacity: 0.55;
		font-variant-numeric: tabular-nums;
	}
	.custom-label {
		color: var(--text);
		font-family: var(--font-body);
	}
	/* Neutral --status-other chip (NOT red — a custom want is a valid choice). */
	.custom-chip {
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		color: var(--status-other);
		background: color-mix(in srgb, var(--status-other) 8%, transparent);
		white-space: nowrap;
	}
</style>
