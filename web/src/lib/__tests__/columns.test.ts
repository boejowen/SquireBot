// Vitest for the consolidated-view column definitions (Plan 14-04).
//
// Review WR-02: the global "Filter all columns" box runs includesString over
// each column's RAW accessor value. Columns whose accessor diverges from the
// rendered cell (last_synced raw ISO vs "YYYY-MM-DD", wiki full URL vs an icon,
// price raw number vs "1,234pp") must be excluded from the global filter so a
// search only matches what the user can see. These tests pin both the column
// config (enableGlobalFilter flags) and the resulting filter behavior through
// the real table-core adapter.

import { describe, it, expect } from 'vitest';
import {
	getCoreRowModel,
	getFilteredRowModel,
	type ColumnFiltersState,
	type SortingState
} from '@tanstack/table-core';
import { createSvelteTable } from '../table/createSvelteTable';
import { viewColumns } from '../columns';
import type { ViewRow } from '../api';

/** A column id -> whether the global filter should inspect it (WR-02 contract). */
const GLOBAL_FILTERABLE: Record<string, boolean> = {
	char: true,
	slot: true,
	item: true,
	count: true,
	id: false,
	wiki: false,
	price: false,
	last_synced: false
};

function findCol(id: string) {
	return viewColumns.find((c) => c.id === id);
}

function makeViewTable(data: ViewRow[], globalFilter: string) {
	return createSvelteTable<ViewRow>({
		data,
		columns: viewColumns,
		state: { sorting: [] as SortingState, columnFilters: [] as ColumnFiltersState, globalFilter },
		globalFilterFn: 'includesString',
		onSortingChange: () => {},
		onColumnFiltersChange: () => {},
		onGlobalFilterChange: () => {},
		getCoreRowModel: getCoreRowModel(),
		getFilteredRowModel: getFilteredRowModel()
	});
}

function row(over: Partial<ViewRow>): ViewRow {
	return {
		char: 'Alpha',
		slot: 'General1',
		item: 'Cloak of Flames',
		id: 10283,
		count: 1,
		wiki_url: 'https://wiki.project1999.com/Cloak_of_Flames',
		price: 1234,
		last_synced: '2026-05-09T00:00:00Z',
		wiki_summary: '',
		is_quest_item: false,
		prices: [],
		quest_links: [],
		...over
	};
}

describe('viewColumns global-filter scoping (WR-02)', () => {
	it('marks only the user-visible text columns as global-filterable', () => {
		for (const [id, expected] of Object.entries(GLOBAL_FILTERABLE)) {
			const col = findCol(id);
			expect(col, `column ${id} should exist`).toBeDefined();
			// `enableGlobalFilter` is undefined (-> default true) for visible columns
			// and explicitly false for the excluded ones.
			expect(col?.enableGlobalFilter ?? true, `column ${id} global-filterable`).toBe(expected);
		}
	});

	it('the global filter no longer matches the raw last_synced ISO string', () => {
		const data = [row({ char: 'Alpha' }), row({ char: 'Bravo', item: 'Fungi Tunic' })];
		// "09T00" only appears inside the raw ISO "2026-05-09T00:00:00Z" — never in a
		// rendered cell. Before the fix this matched every row; now it matches none.
		const t = makeViewTable(data, '09T00');
		expect(t.getRowModel().rows.length).toBe(0);
	});

	it('the global filter no longer matches the hidden wiki URL', () => {
		const data = [row({ char: 'Alpha' })];
		// "project1999" lives only in wiki_url, which renders as an icon (no text).
		const t = makeViewTable(data, 'project1999');
		expect(t.getRowModel().rows.length).toBe(0);
	});

	it('still matches the visible Item text', () => {
		const data = [row({ char: 'Alpha', item: 'Cloak of Flames' }), row({ char: 'Bravo', item: 'Fungi Tunic' })];
		const t = makeViewTable(data, 'Fungi');
		expect(t.getRowModel().rows.map((r) => r.original.char)).toEqual(['Bravo']);
	});
});
