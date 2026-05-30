<script lang="ts" generics="TData">
	// DataGrid — the ONE reusable filterable/sortable grid (14-UI-SPEC Grid
	// Contract), instantiated 4x (view / gear_check / spell_check / bank). NEVER
	// per-character tabs (CLAUDE.md LOCKED: consolidated views with a leading
	// Char column). Headless engine = @tanstack/table-core via the local
	// createSvelteTable adapter (NOT the Svelte-4-only TanStack wrapper —
	// 14-RESEARCH Pitfall 1); all styling/stickiness is ours (table-core is
	// headless).
	//
	// Behavior (UI-SPEC):
	//  - Leading Char column is sticky (position: sticky; left: 0) and the header
	//    row is sticky (position: sticky; top: 0) — together they reproduce the
	//    Sheet's frozen Char col + header.
	//  - Every sortable column toggles asc -> desc -> none on header click (and
	//    Enter/Space — a11y); an accent caret shows direction.
	//  - A global filter input PLUS per-column filters: a faceted <select> for
	//    columns marked meta.filter==='facet' (Status/Tier/Class), text-contains
	//    otherwise. Filtering is client-side (data is tiny).
	//  - NO pagination — the table scrolls inside a fixed-height region under the
	//    sticky header.
	//  - Zebra striping + row hover at ~4-5% accent alpha.

	import {
		getCoreRowModel,
		getSortedRowModel,
		getFilteredRowModel,
		getFacetedRowModel,
		getFacetedUniqueValues,
		type ColumnDef,
		type SortingState,
		type ColumnFiltersState
	} from '@tanstack/table-core';
	import { untrack } from 'svelte';
	import { createSvelteTable, resolveUpdater, FlexRender } from '$lib/table';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
	import Search from '@lucide/svelte/icons/search';

	let {
		data,
		columns,
		defaultSorting = [{ id: 'char', desc: false }]
	}: {
		data: TData[];
		columns: ColumnDef<TData, unknown>[];
		/** Seeds the multi-key default sort (Char asc + the view's secondary keys). */
		defaultSorting?: SortingState;
	} = $props();

	// Pitfall 2: every onXChange unwraps the updater via resolveUpdater, and the
	// state is $state consumed through `get` getters in the `state:` block.
	// `defaultSorting` only SEEDS the initial multi-sort order (Char asc + the
	// view's secondary keys); a one-time copy avoids the reactive-prop-in-init
	// warning and decouples the local sort state from the prop binding.
	const seedSorting: SortingState = untrack(() => defaultSorting.map((s) => ({ ...s })));
	let sorting = $state<SortingState>(seedSorting);
	let columnFilters = $state<ColumnFiltersState>([]);
	let globalFilter = $state('');

	const table = createSvelteTable<TData>({
		get data() {
			return data;
		},
		get columns() {
			return columns;
		},
		state: {
			get sorting() {
				return sorting;
			},
			get columnFilters() {
				return columnFilters;
			},
			get globalFilter() {
				return globalFilter;
			}
		},
		enableMultiSort: true,
		onSortingChange: (u) => (sorting = resolveUpdater(u, sorting)),
		onColumnFiltersChange: (u) => (columnFilters = resolveUpdater(u, columnFilters)),
		onGlobalFilterChange: (u) => (globalFilter = resolveUpdater(u, globalFilter)),
		globalFilterFn: 'includesString',
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		getFilteredRowModel: getFilteredRowModel(),
		getFacetedRowModel: getFacetedRowModel(),
		getFacetedUniqueValues: getFacetedUniqueValues()
	});

	// Distinct values for a faceted column, sorted for a stable <select>.
	function facetOptions(columnId: string): string[] {
		const col = table.getColumn(columnId);
		if (!col) return [];
		return Array.from(col.getFacetedUniqueValues().keys())
			.filter((v): v is string => v != null && v !== '')
			.map(String)
			.sort((a, b) => a.localeCompare(b));
	}
</script>

<div class="datagrid">
	<!-- Toolbar: global filter + per-column filters. Stacks above the grid on
	     small screens (UI-SPEC Responsive). -->
	<div class="toolbar">
		<label class="global-filter">
			<Search size={16} aria-hidden="true" />
			<input
				type="text"
				placeholder="Filter all columns…"
				value={globalFilter}
				oninput={(e) => (globalFilter = e.currentTarget.value)}
				aria-label="Filter all columns"
			/>
		</label>
		<div class="col-filters">
			{#each table.getAllLeafColumns() as col (col.id)}
				{#if col.getCanFilter()}
					{#if col.columnDef.meta?.filter === 'facet'}
						<select
							class="facet"
							aria-label={`Filter by ${col.id}`}
							value={(col.getFilterValue() as string) ?? ''}
							onchange={(e) =>
								col.setFilterValue(e.currentTarget.value === '' ? undefined : e.currentTarget.value)}
						>
							<option value="">{col.id} (all)</option>
							{#each facetOptions(col.id) as opt (opt)}
								<option value={opt}>{opt}</option>
							{/each}
						</select>
					{:else}
						<input
							class="col-text"
							type="text"
							placeholder={String(col.columnDef.header ?? col.id)}
							value={(col.getFilterValue() as string) ?? ''}
							oninput={(e) =>
								col.setFilterValue(e.currentTarget.value === '' ? undefined : e.currentTarget.value)}
							aria-label={`Filter by ${col.id}`}
						/>
					{/if}
				{/if}
			{/each}
		</div>
	</div>

	<!-- Scroll region: vertical scroll under the sticky header; horizontal
	     scroll on narrow viewports with Char kept sticky. -->
	<div class="scroll-region">
		<table>
			<thead>
				{#each table.getHeaderGroups() as headerGroup (headerGroup.id)}
					<tr class="eq-header">
						{#each headerGroup.headers as header, i (header.id)}
							{@const sortDir = header.column.getIsSorted()}
							<th
								class:sticky-char={i === 0}
								class:sortable={header.column.getCanSort()}
								scope="col"
								aria-sort={sortDir === 'asc'
									? 'ascending'
									: sortDir === 'desc'
										? 'descending'
										: 'none'}
								onclick={header.column.getCanSort()
									? header.column.getToggleSortingHandler()
									: undefined}
								onkeydown={(e) => {
									if (header.column.getCanSort() && (e.key === 'Enter' || e.key === ' ')) {
										e.preventDefault();
										header.column.toggleSorting();
									}
								}}
								tabindex={header.column.getCanSort() ? 0 : undefined}
								role={header.column.getCanSort() ? 'button' : undefined}
							>
								<span class="th-inner">
									{#if !header.isPlaceholder}
										<FlexRender
											content={header.column.columnDef.header}
											context={header.getContext()}
										/>
									{/if}
									{#if header.column.getCanSort()}
										<span class="caret">
											{#if sortDir === 'asc'}
												<ChevronUp size={14} aria-hidden="true" />
											{:else if sortDir === 'desc'}
												<ChevronDown size={14} aria-hidden="true" />
											{:else}
												<ChevronsUpDown size={14} aria-hidden="true" class="caret-idle" />
											{/if}
										</span>
									{/if}
								</span>
							</th>
						{/each}
					</tr>
				{/each}
			</thead>
			<tbody>
				{#each table.getRowModel().rows as row (row.id)}
					<tr class="eq-row">
						{#each row.getVisibleCells() as cell, i (cell.id)}
							<td class:sticky-char={i === 0}>
								<FlexRender content={cell.column.columnDef.cell} context={cell.getContext()} />
							</td>
						{/each}
					</tr>
				{:else}
					<tr>
						<td class="no-rows" colspan={columns.length}>No rows match the current filters.</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>

<style>
	.datagrid {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg — toolbar→grid (UI-SPEC Spacing) */
		min-height: 0;
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}
	.global-filter {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 0 8px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		background: var(--panel);
		color: var(--text);
	}
	.global-filter input,
	.col-text,
	.facet {
		background: transparent;
		border: none;
		color: var(--text);
		font-family: var(--font-body);
		font-size: 16px;
		padding: 8px;
		min-height: 44px;
	}
	.col-filters {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.col-text,
	.facet {
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		background: var(--panel);
	}
	.global-filter input:focus-visible,
	.col-text:focus-visible,
	.facet:focus-visible,
	.global-filter:focus-within {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}

	.scroll-region {
		overflow: auto;
		max-height: 70vh; /* fixed-height scroll region; NO pagination (UI-SPEC) */
		border: 1px solid var(--border, rgba(74, 101, 133, 0.6));
		border-radius: 6px;
	}
	table {
		border-collapse: separate;
		border-spacing: 0;
		width: 100%;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		/* counts/prices align (UI-SPEC Typography). */
		font-variant-numeric: tabular-nums;
	}
	thead th {
		position: sticky; /* sticky header row (UI-SPEC) */
		top: 0;
		z-index: 2;
		background: var(--panel);
		color: var(--accent);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-align: left;
		padding: 8px 16px; /* sm × md (UI-SPEC density) */
		border-bottom: 2px solid var(--accent); /* engraved-header accent underline */
		white-space: nowrap;
		user-select: none;
	}
	th.sortable {
		cursor: pointer;
	}
	th.sortable:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.th-inner {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.caret {
		display: inline-flex;
		color: var(--accent);
	}
	:global(.caret-idle) {
		opacity: 0.35;
	}

	tbody td {
		padding: 8px 16px; /* sm × md */
		border-bottom: 1px solid var(--border, rgba(74, 101, 133, 0.35));
		vertical-align: middle;
	}
	/* Sticky leading Char column (header + body). left:0 keeps it frozen during
	   horizontal scroll (UI-SPEC Responsive). */
	th.sticky-char {
		position: sticky;
		left: 0;
		z-index: 3; /* above other header cells when both axes scroll */
	}
	td.sticky-char {
		position: sticky;
		left: 0;
		z-index: 1;
		background: var(--row-bg, var(--panel));
	}
	/* Zebra striping + hover at ~4-5% accent alpha (UI-SPEC). Heavy's parchment
	   rows are themed via the .eq-row hook in app.css, so scope zebra to the
	   non-heavy default by layering a subtle accent tint that reads on both. */
	tbody tr:nth-child(even) td {
		background: color-mix(in srgb, var(--accent) 4%, transparent);
	}
	tbody tr:hover td {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	tbody tr:nth-child(even) td.sticky-char,
	tbody tr:hover td.sticky-char {
		/* keep the sticky cell opaque over the scroll-through, with the same tint */
		background:
			color-mix(in srgb, var(--accent) 6%, transparent), var(--row-bg, var(--panel));
	}
	.no-rows {
		padding: 24px 16px;
		text-align: center;
		opacity: 0.7;
	}
</style>
