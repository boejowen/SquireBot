// Local Svelte-5 adapter over @tanstack/table-core (14-RESEARCH Pattern 3 /
// Pitfall 1). This is the shadcn-svelte data-table idiom vendored locally — we
// deliberately do NOT depend on the published Svelte TanStack wrapper package,
// which is Svelte-4-only (its peer dep is svelte ^4 and it imports the v4
// internals; verified in 14-RESEARCH). @tanstack/table-core is
// framework-agnostic; this shim wires it to Svelte 5.
//
// Why this is a plain .ts (not .svelte.ts): the reactivity is owned by the
// CALLER. The DataGrid holds `$state` for sorting/columnFilters/globalFilter
// and passes them in via reactive `state` getters; it reads
// `table.getRowModel()` / `getHeaderGroups()` inside its reactive template, so
// table-core re-derives on every tracked read. This module only has to (a)
// construct the table and (b) re-pipe the live options each render via
// `setOptions` so the getters are re-read. No module-level rune is needed.
//
// Pitfall 2 (the load-bearing detail): TanStack's onXChange callbacks hand you
// an UPDATER that is either a new value or an `(old) => new` function. With
// Svelte 5 runes you MUST detect-and-call it — `typeof u === 'function'
// ? u(prev) : u` — or clicking a header / typing a filter silently does
// nothing. The `resolveUpdater` helper below is the single place that unwrap
// lives; the DataGrid's onXChange handlers call it.

import {
	createTable,
	type RowData,
	type TableOptions,
	type TableOptionsResolved,
	type Table,
	type Updater
} from '@tanstack/table-core';

/**
 * Resolve a TanStack updater (a value OR an `(old) => new` function) against
 * the previous state — the Pitfall-2 unwrap, in one tested place. The DataGrid
 * passes its `$state` setters through here:
 *   onSortingChange: (u) => (sorting = resolveUpdater(u, sorting))
 */
export function resolveUpdater<T>(u: Updater<T>, prev: T): T {
	return typeof u === 'function' ? (u as (old: T) => T)(prev) : u;
}

/**
 * Merge a base options object with partials while PRESERVING getters — so the
 * caller's reactive `data`/`columns`/`state` getters survive instead of being
 * snapshotted to a plain value at construction time. (Mirrors shadcn-svelte's
 * `mergeObjects`.)
 */
function mergeOptions<T extends object>(base: T, override: Partial<T>): T {
	return new Proxy(base, {
		get(target, prop, receiver) {
			if (prop in override) {
				return Reflect.get(override as object, prop, receiver);
			}
			return Reflect.get(target, prop, receiver);
		},
		has(target, prop) {
			return prop in override || Reflect.has(target, prop);
		}
	});
}

/**
 * Create a reactive TanStack table backed by the caller's Svelte 5 runes. Pass
 * reactive getters for `data` / `columns` / `state` (see DataGrid) so the table
 * re-derives when the caller's `$state` changes. Returns the `Table` instance;
 * consume its `getHeaderGroups()` / `getRowModel()` / `getFlatHeaders()` etc.
 * inside a reactive Svelte template.
 */
export function createSvelteTable<TData extends RowData>(
	options: TableOptions<TData>
): Table<TData> {
	const resolved = mergeOptions(
		{
			state: {},
			onStateChange() {},
			renderFallbackValue: null
		} as unknown as TableOptionsResolved<TData>,
		options as Partial<TableOptionsResolved<TData>>
	);

	const table = createTable<TData>(resolved);

	// Re-pipe the live options on every (re)render so the reactive getters in
	// `options` (data/columns/state) are re-read by table-core. Because the
	// DataGrid reads the row/header models inside its reactive template, this
	// runs whenever a tracked dependency (sorting/filters/data) changes.
	table.setOptions((prev) =>
		mergeOptions(prev, {
			...options,
			state: mergeOptions(prev.state, options.state ?? {})
		} as Partial<typeof prev>)
	);

	return table;
}
