<script lang="ts">
	// WikiCell — the `Wiki` column cell for view/bank: a real external-link <a>
	// to the item's wiki.project1999.com page (14-UI-SPEC "Wiki = external-link
	// icon <a>"). NOT an =HYPERLINK formula (that was a Sheet artifact). The
	// anchor carries rel="noopener" + target="_blank" (T-14.04-03 tab-nabbing
	// mitigation, set explicitly here per the threat register). The lucide
	// external-link icon sits in the reserved accent set.
	import ExternalLink from '@lucide/svelte/icons/external-link';

	let { wikiUrl }: { wikiUrl: string } = $props();
</script>

{#if wikiUrl}
	<a class="wiki-link" href={wikiUrl} target="_blank" rel="noopener" aria-label="Open on the P1999 wiki">
		<ExternalLink size={16} aria-hidden="true" />
	</a>
{/if}

<style>
	.wiki-link {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 44px;
		min-height: 44px;
		color: var(--accent);
	}
	.wiki-link:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
</style>
