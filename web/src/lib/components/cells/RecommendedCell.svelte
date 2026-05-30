<script lang="ts">
	// RecommendedCell — the `Recommended` column cell for gear_check: the
	// recommended item name as an ItemTooltip trigger (14-UI-SPEC Grid Contract
	// "Recommended = tooltip trigger"). gear_check rows carry no inline
	// enrichment (the read API's GearCheckRow is name-only), so the tooltip
	// shows the name + the no-transactions line + a wiki link only if a URL is
	// known. The wiki URL is derived from the item name via the standard P1999
	// wiki page convention (spaces -> underscores) so the link still works.
	import ItemTooltip from '../ItemTooltip.svelte';

	let { recommended }: { recommended: string } = $props();

	const WIKI_BASE = 'https://wiki.project1999.com/';

	// P1999 wiki page titles use underscores for spaces; encodeURI keeps it safe.
	let wikiUrl = $derived(recommended ? WIKI_BASE + encodeURI(recommended.replace(/ /g, '_')) : '');
</script>

{#if recommended}
	<ItemTooltip itemName={recommended} {wikiUrl} summary={null} prices={[]} questLinks={[]}>
		<span class="rec-link">{recommended}</span>
	</ItemTooltip>
{/if}

<style>
	.rec-link {
		color: var(--accent);
		border-bottom: 1px dotted var(--accent);
		font-family: var(--font-body);
	}
</style>
