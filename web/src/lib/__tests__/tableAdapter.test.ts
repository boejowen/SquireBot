// Vitest for the local table-core adapter wiring (Plan 14-04 Task 1) — proves
// the adapter actually sorts/filters via @tanstack/table-core and that the
// Pitfall-2 updater unwrap (resolveUpdater) is correct. This is the logic gate
// for the grid (component DOM testing is optional per the plan); it runs in
// node because createSvelteTable uses no runes (reactivity is owned by the
// caller's component, not the adapter).

import { describe, it, expect } from 'vitest';
import {
	getCoreRowModel,
	getSortedRowModel,
	getFilteredRowModel,
	type ColumnDef,
	type SortingState,
	type ColumnFiltersState
} from '@tanstack/table-core';
import { createSvelteTable, resolveUpdater } from '../table/createSvelteTable';

interface Row {
	char: string;
	count: number;
}

const DATA: Row[] = [
	{ char: 'Charlie', count: 3 },
	{ char: 'Alpha', count: 1 },
	{ char: 'Bravo', count: 2 }
];

const COLUMNS: ColumnDef<Row, unknown>[] = [
	{ id: 'char', accessorKey: 'char', header: 'Char' },
	{ id: 'count', accessorKey: 'count', header: 'Count' }
];

function makeTable(sorting: SortingState, columnFilters: ColumnFiltersState, globalFilter: string) {
	return createSvelteTable<Row>({
		data: DATA,
		columns: COLUMNS,
		state: { sorting, columnFilters, globalFilter },
		enableMultiSort: true,
		globalFilterFn: 'includesString',
		onSortingChange: () => {},
		onColumnFiltersChange: () => {},
		onGlobalFilterChange: () => {},
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		getFilteredRowModel: getFilteredRowModel()
	});
}

describe('resolveUpdater (Pitfall 2)', () => {
	it('returns a plain value as-is', () => {
		expect(resolveUpdater(5, 0)).toBe(5);
	});
	it('calls an updater function against the previous state', () => {
		expect(resolveUpdater((prev: number) => prev + 1, 41)).toBe(42);
	});
});

describe('createSvelteTable (table-core adapter)', () => {
	it('returns rows unsorted by default (core row model order)', () => {
		const t = makeTable([], [], '');
		expect(t.getRowModel().rows.map((r) => r.original.char)).toEqual(['Charlie', 'Alpha', 'Bravo']);
	});

	it('sorts ascending by the Char column when sorting state is set', () => {
		const t = makeTable([{ id: 'char', desc: false }], [], '');
		expect(t.getRowModel().rows.map((r) => r.original.char)).toEqual(['Alpha', 'Bravo', 'Charlie']);
	});

	it('sorts descending by Char', () => {
		const t = makeTable([{ id: 'char', desc: true }], [], '');
		expect(t.getRowModel().rows.map((r) => r.original.char)).toEqual(['Charlie', 'Bravo', 'Alpha']);
	});

	it('applies a per-column filter (text contains)', () => {
		const t = makeTable([], [{ id: 'char', value: 'lph' }], '');
		expect(t.getRowModel().rows.map((r) => r.original.char)).toEqual(['Alpha']);
	});

	it('applies the global filter across columns', () => {
		const t = makeTable([], [], 'Bravo');
		expect(t.getRowModel().rows.map((r) => r.original.char)).toEqual(['Bravo']);
	});

	it('exposes faceted-capable columns by id', () => {
		const t = makeTable([], [], '');
		expect(t.getColumn('char')).toBeDefined();
		expect(t.getColumn('count')).toBeDefined();
	});
});
