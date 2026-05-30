<script lang="ts">
	// SearchBox — cross-character item search (WEB-03 / D-03, 14-UI-SPEC Search
	// Contract). Runs the ported in-memory engine (searchRows) over the already
	// fetched `view` rows — data is tiny (~12 users) so it is synchronous and
	// well under the <2s bar, no cache/prewarm. Case-insensitive substring match
	// grouped by item, with a clickable inline "did you mean?" on no exact match.

	import Search from '@lucide/svelte/icons/search';
	import SearchResults from './SearchResults.svelte';
	import { searchRows, type SearchResultRow } from '$lib/search/searchIndex';
	import type { ViewRow } from '$lib/api';

	let { rows }: { rows: ViewRow[] } = $props();

	let query = $state('');

	// Map the API's snake_case ViewRow -> the search engine's SearchResultRow
	// (the tooltip enrichment fields ride along so each result name is a working
	// tooltip trigger + wiki link). Derived so it recomputes if `rows` changes.
	let searchRowsInput = $derived<SearchResultRow[]>(
		rows.map((r) => ({
			itemName: r.item,
			itemId: r.id,
			char: r.char,
			location: r.slot,
			count: r.count,
			wikiUrl: r.wiki_url,
			wikiSummary: r.wiki_summary ?? '',
			pricePp: r.price
		}))
	);

	let result = $derived(searchRows(query, searchRowsInput));

	function onSuggest(term: string) {
		// D-09: clicking the suggestion re-runs the search with that term.
		query = term;
	}
</script>

<div class="search-box">
	<label class="search-field">
		<Search size={18} aria-hidden="true" />
		<input
			type="search"
			placeholder="Search items across the guild…"
			bind:value={query}
			aria-label="Search items across the guild"
		/>
	</label>

	{#if query.trim() !== ''}
		<div class="search-output">
			<SearchResults
				groups={result.groups}
				suggestions={result.suggestions}
				{query}
				{onSuggest}
			/>
		</div>
	{/if}
</div>

<style>
	.search-box {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.search-field {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 0 12px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		background: var(--panel);
		color: var(--accent);
		max-width: 480px;
	}
	.search-field input {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--text);
		font-family: var(--font-body);
		font-size: 16px;
		padding: 8px 0;
		min-height: 44px;
		outline: none;
	}
	/* Accent focus ring on the active search box (reserved accent use). */
	.search-field:focus-within {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.search-output {
		max-width: 640px;
	}
</style>
