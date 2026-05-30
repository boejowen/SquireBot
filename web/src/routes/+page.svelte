<script lang="ts">
	// The product page: the four consolidated views + cross-character search,
	// wired to the Go read API (WEB-01/03/04, BACKEND-05 consumption). On mount it
	// fetches all four payloads + meta in parallel and holds loading / error /
	// ready state. A nav switches between the four reusable DataGrid instances
	// (ONE component, 4 instances — never per-character tabs, CLAUDE.md LOCKED),
	// each fed its payload + the matching columns.ts def + the view's multi-key
	// default sort. SearchBox runs over the in-memory `view` rows (D-03). bank
	// shows the "Coin: not yet recorded" affordance (coin is null in P14).
	// Fetch failure -> error StateBlock with a working Retry; empty payload -> the
	// per-view-empty StateBlock; loading -> the skeleton. (The bank coin affordance
	// renders the "not yet recorded" copy, never a fabricated zero-platinum value.)

	import { onMount } from 'svelte';
	import DataGrid from '$lib/components/DataGrid.svelte';
	import SearchBox from '$lib/components/SearchBox.svelte';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import StatusLegend from '$lib/components/StatusLegend.svelte';
	import {
		fetchView,
		fetchGearCheck,
		fetchSpellCheck,
		fetchBank,
		fetchMeta,
		type ViewRow,
		type GearCheckRow,
		type SpellCheckRow,
		type MetaResponse
	} from '$lib/api';
	import {
		viewColumns,
		gearCheckColumns,
		spellCheckColumns,
		bankColumns
	} from '$lib/columns';
	import type { SortingState } from '@tanstack/table-core';

	type ViewId = 'view' | 'gear_check' | 'spell_check' | 'bank';
	type Status = 'loading' | 'error' | 'ready';

	const TABS: { id: ViewId; label: string; friendly: string }[] = [
		{ id: 'view', label: 'Inventory', friendly: 'inventory' },
		{ id: 'gear_check', label: 'Gear Check', friendly: 'gear check' },
		{ id: 'spell_check', label: 'Spell Check', friendly: 'spell check' },
		{ id: 'bank', label: 'Bank', friendly: 'bank' }
	];

	// Per-view multi-key default sort (UI-SPEC secondary sorts; column ids from
	// columns.ts). Tier sorts by the custom rank fn registered on that column.
	const SORT: Record<ViewId, SortingState> = {
		view: [
			{ id: 'char', desc: false },
			{ id: 'item', desc: false },
			{ id: 'slot', desc: false }
		],
		bank: [
			{ id: 'char', desc: false },
			{ id: 'item', desc: false },
			{ id: 'slot', desc: false }
		],
		gear_check: [
			{ id: 'char', desc: false },
			{ id: 'tier', desc: false },
			{ id: 'slot', desc: false },
			{ id: 'recommended', desc: false }
		],
		spell_check: [
			{ id: 'char', desc: false },
			{ id: 'level', desc: false },
			{ id: 'spell', desc: false }
		]
	};

	let active = $state<ViewId>('view');
	let status = $state<Status>('loading');

	let viewRows = $state<ViewRow[]>([]);
	let gearRows = $state<GearCheckRow[]>([]);
	let spellRows = $state<SpellCheckRow[]>([]);
	let bankRows = $state<ViewRow[]>([]);
	let meta = $state<MetaResponse>({ characters: [] });

	async function load() {
		status = 'loading';
		try {
			const [v, g, s, b, m] = await Promise.all([
				fetchView(),
				fetchGearCheck(),
				fetchSpellCheck(),
				fetchBank(),
				fetchMeta()
			]);
			viewRows = v;
			gearRows = g;
			spellRows = s;
			bankRows = b.rows;
			meta = m;
			status = 'ready';
		} catch {
			// api.ts already logged nothing sensitive; surface the error state.
			status = 'error';
		}
	}

	// Retry re-fires the whole parallel fetch (UI-SPEC error-state Retry).
	function refetch() {
		void load();
	}

	// One-shot initial fetch. onMount is the correct primitive for a fire-once
	// load: it runs exactly once after the component mounts and never re-runs.
	// A bare $effect would re-fire the whole five-endpoint parallel fetch if
	// load() ever started reading reactive state synchronously (e.g. scoping the
	// fetch by `active` or a query param) — refactor-fragile (review WR-03).
	// refetch() (the Retry handler) calls load() directly, so the effect is not
	// needed for retry either.
	onMount(() => {
		void load();
	});

	// "No characters at all" — drives the top-level empty state (UI-SPEC).
	let noCharacters = $derived(
		status === 'ready' && meta.characters.length === 0 && viewRows.length === 0
	);
</script>

<svelte:head>
	<title>SquireBot — guild inventory</title>
</svelte:head>

{#if status === 'loading'}
	<StateBlock kind="loading" />
{:else if status === 'error'}
	<StateBlock kind="error" onRetry={refetch} />
{:else if noCharacters}
	<StateBlock kind="empty" />
{:else}
	<!-- Cross-character search over the in-memory view rows (D-03). -->
	<section class="search-section" aria-label="Search">
		<SearchBox rows={viewRows} />
	</section>

	<!-- View nav: 4 tabs over the ONE reusable DataGrid. -->
	<nav class="view-nav" aria-label="Views">
		{#each TABS as tab (tab.id)}
			<button
				class="tab"
				class:active={active === tab.id}
				type="button"
				aria-current={active === tab.id ? 'page' : undefined}
				onclick={() => (active = tab.id)}
			>
				{tab.label}
			</button>
		{/each}
	</nav>

	<section class="view-panel">
		{#if active === 'view'}
			{#if viewRows.length === 0}
				<StateBlock kind="view-empty" viewName="inventory" />
			{:else}
				<DataGrid data={viewRows} columns={viewColumns} defaultSorting={SORT.view} />
			{/if}
		{:else if active === 'gear_check'}
			<StatusLegend variant="gear" />
			{#if gearRows.length === 0}
				<StateBlock kind="view-empty" viewName="gear check" />
			{:else}
				<DataGrid data={gearRows} columns={gearCheckColumns} defaultSorting={SORT.gear_check} />
			{/if}
		{:else if active === 'spell_check'}
			<StatusLegend variant="spell" />
			{#if spellRows.length === 0}
				<StateBlock kind="view-empty" viewName="spell check" />
			{:else}
				<DataGrid data={spellRows} columns={spellCheckColumns} defaultSorting={SORT.spell_check} />
			{/if}
		{:else if active === 'bank'}
			<!-- coin is null in P14 (ADMIN-05 fills it in P15) — render the
			     not-yet-recorded affordance, never a fabricated zero-platinum value. -->
			<StateBlock kind="no-coin" />
			{#if bankRows.length === 0}
				<!-- bank may be legitimately empty until P16 sets is_bank_toon — empty
				     state, NOT an error (RESEARCH Open-Q4 / A7). -->
				<StateBlock kind="view-empty" viewName="bank" />
			{:else}
				<DataGrid data={bankRows} columns={bankColumns} defaultSorting={SORT.bank} />
			{/if}
		{/if}
	</section>
{/if}

<style>
	.search-section {
		max-width: 720px;
	}
	.view-nav {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		border-bottom: 1px solid var(--border, rgba(74, 101, 133, 0.4));
	}
	.tab {
		min-height: 44px;
		padding: 8px 16px;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		cursor: pointer;
		opacity: 0.7;
	}
	.tab:hover {
		opacity: 1;
	}
	/* Active view tab in accent (reserved accent use, UI-SPEC). */
	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
		opacity: 1;
	}
	.tab:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.view-panel {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-height: 0;
	}
</style>
