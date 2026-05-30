<script lang="ts">
	// ItemCell — the `Item` column cell for view/bank: an accent item link that
	// is also the ItemTooltip trigger (14-UI-SPEC Grid Contract "Item = accent
	// link + tooltip trigger"). The accent underline is in the reserved accent
	// list (UI-SPEC § Color). The rich tooltip body comes from the row's inline
	// enrichment fields (D-03) via ItemTooltip → composeItemNote.
	import ItemTooltip from '../ItemTooltip.svelte';
	import type { ViewRow } from '$lib/api';

	let { row }: { row: ViewRow } = $props();

	// Map the row's snake_case enrichment fields to composeItemNote's inputs.
	let summary = $derived(
		row.wiki_summary || row.is_quest_item
			? { summary: row.wiki_summary ?? '', is_quest_item: row.is_quest_item }
			: null
	);
</script>

<ItemTooltip
	itemName={row.item}
	wikiUrl={row.wiki_url}
	{summary}
	prices={row.prices ?? []}
	questLinks={row.quest_links ?? []}
>
	<span class="item-link">{row.item}</span>
</ItemTooltip>

<style>
	.item-link {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		font-family: var(--font-body);
	}
</style>
