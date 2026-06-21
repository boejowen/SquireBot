// Column definitions for the four consolidated views (14-UI-SPEC Grid
// Contract). Column ORDER, names, and status semantics are LOAD-BEARING — they
// mirror the v1 builders exactly so WEB-02 parity is provable. One reusable
// DataGrid (CLAUDE.md LOCKED — never per-character tabs) is instantiated 4x,
// each fed the matching ColumnDef[] from here.
//
// The first column in every array is the sticky leading `Char` column.
//
// Special cells are mounted via `renderComponent` (the local FlexRender helper)
// so the columnDef stays plain data and the actual <Component> mount happens in
// FlexRender, inside a reactive Svelte context:
//   view/bank: Item -> ItemCell (accent link + tooltip), Wiki -> WikiCell
//              (external <a>), Price -> PriceCell (tabular pp), Last Synced ->
//              LastSyncedCell (freshness dot).
//   gear_check: Status -> StatusCell (OK/MISSING/OTHER), Recommended ->
//               RecommendedCell (tooltip trigger).
//   spell_check: Status -> StatusCell (KNOWN/MISSING).
//
// Faceted-select filter columns (Status / Tier / Class) set
// `meta.filter = 'facet'`; all other columns are text-contains (the DataGrid
// reads meta.filter to pick the control). The Tier column uses a custom
// sortingFn mapping the three tier strings to rank 1/2/3 (Pre-Raid -> Raiding
// -> Iksar), matching buildGearCheck's tier rank.

import type { ColumnDef, Row } from '@tanstack/table-core';
import { renderComponent } from '$lib/table';
import { wikiUrlFor } from '$lib/tooltip/composeNotes';
import type { ViewRow, GearCheckRow, SpellCheckRow } from '$lib/api';
import StatusCell from '$lib/components/StatusCell.svelte';
import ItemCell from '$lib/components/cells/ItemCell.svelte';
import WikiCell from '$lib/components/cells/WikiCell.svelte';
import PriceCell from '$lib/components/cells/PriceCell.svelte';
import LastSyncedCell from '$lib/components/cells/LastSyncedCell.svelte';
import RecommendedCell from '$lib/components/cells/RecommendedCell.svelte';

/** Per-column metadata the DataGrid reads to choose a filter control. */
export interface ColumnFilterMeta {
	/** 'facet' = faceted <select> of distinct values; default = text-contains. */
	filter?: 'facet' | 'text';
}

// Tier rank for the gear_check secondary sort (buildGearCheck.ts:29-33):
// Velious Pre-Raid/Group = 1, Velious Raiding = 2, Iksar = 3.
const TIER_RANK: Record<string, number> = {
	'Velious Pre-Raid/Group': 1,
	'Velious Raiding': 2,
	Iksar: 3
};

function tierRank(tier: string): number {
	return TIER_RANK[tier] ?? 99;
}

/** Custom TanStack sortingFn: order the Tier column by tier rank, not alpha. */
function tierSort(a: Row<GearCheckRow>, b: Row<GearCheckRow>): number {
	return tierRank(a.original.tier) - tierRank(b.original.tier);
}

// --- view / bank (identical column set; UI-SPEC) -------------------------
// Char · Slot · Item · Count · Wiki · Price · Last Synced. (No raw item-ID
// column — IDs are watcher plumbing, not member-facing data.)
// Default sort Char asc, then item asc, then location asc (DataGrid seeds the
// multi-sort state; these accessors back it).
// The global filter box runs `includesString` over each column's
// raw accessor value. For columns whose accessor diverges from what the user
// actually sees rendered, that produces confusing "phantom" matches (review
// WR-02): `last_synced` carries the raw ISO string ("2026-05-09T00:00:00Z")
// while the cell renders only "2026-05-09", `wiki` carries the full wiki URL
// while the cell renders just an icon (no visible text), and `price` carries a
// raw number that the cell formats as "1,234pp". Each of these sets
// `enableGlobalFilter: false` so the global box only matches the user-visible
// text columns (Char / Slot / Item); per-column filtering is unaffected.
export const viewColumns: ColumnDef<ViewRow, unknown>[] = [
	{ id: 'char', accessorKey: 'char', header: 'Char' },
	{ id: 'slot', accessorKey: 'slot', header: 'Slot' },
	{
		id: 'item',
		accessorKey: 'item',
		header: 'Item',
		cell: (ctx) => renderComponent(ItemCell, { row: ctx.row.original })
	},
	{ id: 'count', accessorKey: 'count', header: 'Count' },
	{
		id: 'wiki',
		accessorKey: 'wiki_url',
		header: 'Wiki',
		enableSorting: false,
		enableColumnFilter: false,
		enableGlobalFilter: false,
		cell: (ctx) =>
			renderComponent(WikiCell, {
				wikiUrl: ctx.row.original.wiki_url || wikiUrlFor(ctx.row.original.item)
			})
	},
	{
		id: 'price',
		accessorKey: 'price',
		header: 'Price',
		enableGlobalFilter: false,
		cell: (ctx) => renderComponent(PriceCell, { price: ctx.row.original.price })
	},
	{
		id: 'last_synced',
		accessorKey: 'last_synced',
		header: 'Last Synced',
		enableColumnFilter: false,
		enableGlobalFilter: false,
		cell: (ctx) => renderComponent(LastSyncedCell, { lastSynced: ctx.row.original.last_synced })
	}
];

// bank uses the SAME columns as view (the coin affordance is rendered by
// +page.svelte alongside the grid, not as a column).
export const bankColumns: ColumnDef<ViewRow, unknown>[] = viewColumns;

// --- gear_check (UI-SPEC) ------------------------------------------------
// Char · Class · Tier · Slot · Have · Recommended · Status.
// Secondary sort: tier rank, then slot, then recommended (DataGrid seeds it).
export const gearCheckColumns: ColumnDef<GearCheckRow, unknown>[] = [
	{ id: 'char', accessorKey: 'char', header: 'Char' },
	{ id: 'class', accessorKey: 'class', header: 'Class', meta: { filter: 'facet' } },
	{
		id: 'tier',
		accessorKey: 'tier',
		header: 'Tier',
		sortingFn: tierSort,
		meta: { filter: 'facet' }
	},
	{ id: 'slot', accessorKey: 'slot', header: 'Slot' },
	{ id: 'have', accessorKey: 'have', header: 'Have' },
	{
		id: 'recommended',
		accessorKey: 'recommended',
		header: 'Recommended',
		cell: (ctx) => renderComponent(RecommendedCell, { recommended: ctx.row.original.recommended })
	},
	{
		id: 'status',
		accessorKey: 'status',
		header: 'Status',
		meta: { filter: 'facet' },
		cell: (ctx) => renderComponent(StatusCell, { status: ctx.row.original.status })
	}
];

// --- spell_check (UI-SPEC) -----------------------------------------------
// Char · Class · Level · Spell · Status.
// Secondary sort: level asc, then spell asc (DataGrid seeds it).
export const spellCheckColumns: ColumnDef<SpellCheckRow, unknown>[] = [
	{ id: 'char', accessorKey: 'char', header: 'Char' },
	{ id: 'class', accessorKey: 'class', header: 'Class', meta: { filter: 'facet' } },
	{ id: 'level', accessorKey: 'level', header: 'Level' },
	{ id: 'spell', accessorKey: 'spell', header: 'Spell' },
	{
		id: 'status',
		accessorKey: 'status',
		header: 'Status',
		meta: { filter: 'facet' },
		cell: (ctx) => renderComponent(StatusCell, { status: ctx.row.original.status })
	}
];
