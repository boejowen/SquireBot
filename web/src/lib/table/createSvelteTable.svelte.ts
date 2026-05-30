// Runes implementation of the local table-core Svelte-5 adapter (the
// shadcn-svelte data-table idiom). Lives in a .svelte.ts module because it uses
// `$state` to make the Table instance reactive; the public re-export is in
// createSvelteTable.ts (so the plan's filename + greps resolve, and plain .ts
// callers import cleanly).
//
// 14-RESEARCH Pattern 3 / Pitfall 1: we use @tanstack/table-core directly (NOT
// the Svelte-4-only published wrapper). Pitfall 2: the DataGrid's onXChange
// callbacks unwrap the updater via resolveUpdater (below) — `typeof u ===
// 'function' ? u(prev) : u` — or sorting/filtering silently no-ops.

import {
	createTable,
	type RowData,
	type TableOptions,
	type TableOptionsResolved,
	type Table,
	type Updater
} from '@tanstack/table-core';

// NOTE: resolveUpdater (the Pitfall-2 unwrap) is defined in the sibling
// runes-free entry createSvelteTable.ts so plain .ts callers can import it
// without pulling a .svelte.ts module.

/**
 * Merge objects while preserving getters (so reactive data/columns/state getters
 * survive instead of being snapshotted). Typed loosely (overrides are arbitrary
 * partials) and returns the base type — the strict TanStack option shapes are
 * applied by the callers' casts, not here.
 */
function mergeObjects<T extends object>(base: T, ...overrides: Array<object | undefined>): T {
	const sources = overrides.filter((o): o is object => o != null);
	return new Proxy(base, {
		get(target, prop, receiver) {
			for (let i = sources.length - 1; i >= 0; i--) {
				if (prop in sources[i]) return Reflect.get(sources[i] as object, prop, receiver);
			}
			return Reflect.get(target, prop, receiver);
		},
		has(target, prop) {
			for (const s of sources) if (prop in s) return true;
			return Reflect.has(target, prop);
		},
		ownKeys(target) {
			const keys = new Set<string | symbol>(Reflect.ownKeys(target));
			for (const s of sources) for (const k of Reflect.ownKeys(s)) keys.add(k);
			return Array.from(keys);
		},
		getOwnPropertyDescriptor(target, prop) {
			for (let i = sources.length - 1; i >= 0; i--) {
				if (prop in sources[i]) {
					const d = Reflect.getOwnPropertyDescriptor(sources[i], prop);
					if (d) return { ...d, configurable: true };
				}
			}
			return Reflect.getOwnPropertyDescriptor(target, prop);
		}
	});
}

/**
 * Create a reactive TanStack table backed by Svelte 5 runes. Pass reactive
 * getters for `data`/`columns`/`state`; consume `getHeaderGroups()`/
 * `getRowModel()`/`getColumn()` etc. inside a reactive template.
 */
export function createSvelteTable<TData extends RowData>(
	options: TableOptions<TData>
): Table<TData> {
	const defaults = {
		state: {},
		onStateChange() {},
		renderFallbackValue: null,
		mergeOptions: (defaultOptions: TableOptions<TData>, newOptions: Partial<TableOptions<TData>>) =>
			mergeObjects(defaultOptions, newOptions)
	};
	const resolvedOptions = mergeObjects(defaults, options) as unknown as TableOptionsResolved<TData>;

	const table = createTable<TData>(resolvedOptions);

	// $state makes reads of the table's models reactive in the component.
	const state = $state<Table<TData>>(table);

	state.setOptions((prev) =>
		mergeObjects(prev, options, {
			state: mergeObjects(state.initialState as object, options.state ?? {}),
			onStateChange: (u: Updater<unknown>) => {
				// Re-trigger reactivity, then forward to the caller's setter.
				(options.onStateChange as ((u: Updater<unknown>) => void) | undefined)?.(u);
			}
		} as object)
	);

	return state;
}
