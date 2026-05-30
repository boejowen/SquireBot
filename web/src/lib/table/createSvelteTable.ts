// Public entry for the local Svelte-5 adapter over @tanstack/table-core
// (14-RESEARCH Pattern 3 / Pitfall 1 — we use table-core directly, NOT the
// Svelte-4-only published wrapper). The reactive `createSvelteTable` itself
// uses `$state`, so it lives in the sibling `createSvelteTable.svelte.ts`
// module and is re-exported here; this file owns the Pitfall-2 updater unwrap
// (`resolveUpdater`) and the public types so plain `.ts` callers import from a
// single, runes-free entry point.

import type { Updater } from '@tanstack/table-core';

// Re-export the reactive table factory (runes-backed) from the .svelte.ts impl.
export { createSvelteTable } from './createSvelteTable.svelte';

/**
 * Resolve a TanStack updater against the previous state — the single Pitfall-2
 * unwrap. TanStack's onXChange callbacks hand you an updater that is either a
 * new value OR an `(old) => new` function; with Svelte 5 runes you MUST
 * detect-and-call it — `typeof u === 'function' ? u(prev) : u` — or clicking a
 * header / typing a filter silently does nothing. The DataGrid pipes its
 * `$state` setters through here:
 *   onSortingChange: (u) => (sorting = resolveUpdater(u, sorting))
 */
export function resolveUpdater<T>(u: Updater<T>, prev: T): T {
	return typeof u === 'function' ? (u as (old: T) => T)(prev) : u;
}
