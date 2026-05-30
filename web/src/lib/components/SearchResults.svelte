<script lang="ts">
	// SearchResults — renders the cross-character search output (14-UI-SPEC
	// Search Contract). Per matched item: the item name (accent link + the same
	// ItemTooltip trigger) + a de-emphasized id, then ONE LINE PER HOLDER
	//   ↳ <Char>: <Location>, count <n>
	// surfacing which character(s)/location(s) hold it — the core-value framing.
	// Groups with >5 holders (group.collapsed) render collapsed behind an expand
	// affordance.
	//
	// No exact match (groups.length === 0): the no-results StateBlock heading
	// `No matches for "<query>"` (query via plain Svelte `{}` interpolation —
	// auto-escaped, T-14.04-02; never the raw-HTML directive); then, if a
	// suggestion exists, the clickable inline `Did you mean <suggestion>?` accent
	// link (D-09) calling onSuggest to RE-RUN the search; else
	// `Try a shorter or different spelling.`

	import StateBlock from './StateBlock.svelte';
	import ItemTooltip from './ItemTooltip.svelte';
	import type { SearchResultGroup } from '$lib/search/searchIndex';
	import type { PigparsePriceRow } from '$lib/tooltip/composeNotes';

	let {
		groups,
		suggestions,
		query,
		onSuggest
	}: {
		groups: SearchResultGroup[];
		suggestions: string[];
		query: string;
		/** Re-runs the search with the suggested term (D-09). */
		onSuggest: (term: string) => void;
	} = $props();

	// Track which collapsed groups the user has expanded.
	let expanded = $state<Set<string>>(new Set());

	function key(g: SearchResultGroup): string {
		return `${g.itemName}|${g.itemId}`;
	}
	function toggle(g: SearchResultGroup) {
		const k = key(g);
		const next = new Set(expanded);
		if (next.has(k)) next.delete(k);
		else next.add(k);
		expanded = next;
	}

	// The search row carries only wikiSummary + pricePp (not the full prices[] /
	// quest_links); synthesize a single WTS price row so the tooltip still shows
	// the ask line. Empty/undefined price -> no price row (tooltip says "No
	// recent transactions").
	function priceRows(g: SearchResultGroup): PigparsePriceRow[] {
		return g.pricePp != null && Number.isFinite(g.pricePp)
			? [{ direction: '0', a30: g.pricePp, t30: 0 }]
			: [];
	}
	function summaryFor(g: SearchResultGroup) {
		return g.wikiSummary ? { summary: g.wikiSummary, is_quest_item: false } : null;
	}
</script>

{#if groups.length > 0}
	<ul class="results">
		{#each groups as g (key(g))}
			{@const isOpen = !g.collapsed || expanded.has(key(g))}
			{@const shown = isOpen ? g.rows : g.rows.slice(0, 5)}
			<li class="result-group">
				<div class="group-head">
					<ItemTooltip
						itemName={g.itemName}
						wikiUrl={g.wikiUrl}
						summary={summaryFor(g)}
						prices={priceRows(g)}
						questLinks={[]}
					>
						<span class="item-name">{g.itemName}</span>
					</ItemTooltip>
					<span class="item-id">#{g.itemId}</span>
				</div>
				<ul class="holders">
					{#each shown as r (r.char + '|' + r.location)}
						<li class="holder">↳ {r.char}: {r.location}, count {r.count}</li>
					{/each}
				</ul>
				{#if g.collapsed}
					<button class="expand" type="button" onclick={() => toggle(g)}>
						{#if isOpen}
							Show fewer
						{:else}
							Show all {g.rows.length} holders
						{/if}
					</button>
				{/if}
			</li>
		{/each}
	</ul>
{:else if query.trim() !== ''}
	<div class="no-results">
		<StateBlock kind="no-results" {query} />
		<p class="suggest-body">
			{#if suggestions.length > 0}
				Did you mean <button class="suggest-link" type="button" onclick={() => onSuggest(suggestions[0])}
					>{suggestions[0]}</button
				>?
			{:else}
				Try a shorter or different spelling.
			{/if}
		</p>
	</div>
{/if}

<style>
	.results {
		display: flex;
		flex-direction: column;
		gap: 16px;
		list-style: none;
		padding: 0;
		margin: 0;
	}
	.result-group {
		border-bottom: 1px solid var(--border, rgba(74, 101, 133, 0.35));
		padding-bottom: 8px;
	}
	.group-head {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}
	.item-name {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		font-family: var(--font-body);
		font-size: 16px;
	}
	.item-id {
		font-size: 13px;
		opacity: 0.55;
		font-variant-numeric: tabular-nums;
	}
	.holders {
		list-style: none;
		margin: 4px 0 0;
		padding: 0;
	}
	.holder {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		padding-left: 8px;
		font-variant-numeric: tabular-nums;
	}
	.expand {
		margin-top: 4px;
		min-height: 44px;
		background: none;
		border: none;
		color: var(--accent);
		font-family: var(--font-body);
		font-size: 13px;
		cursor: pointer;
		padding: 4px 0;
	}
	.expand:focus-visible,
	.suggest-link:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.no-results {
		display: flex;
		flex-direction: column;
		align-items: center;
	}
	.suggest-body {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.85;
		margin-top: -24px;
	}
	/* The "did you mean?" suggestion is a clickable accent link (D-09). */
	.suggest-link {
		display: inline;
		background: none;
		border: none;
		padding: 0;
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		font: inherit;
		cursor: pointer;
	}
</style>
