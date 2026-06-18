// Pure, DOM-free item helpers for the Phase 32 item-centric Inventory tab — the
// D-02 viewer-first ordering (ITEM-02) + the viewer-priority scoped search +
// the holders-table band sort (ITEM-03). Extracted to a plain .ts (NOT a .svelte
// module export) so they're unit-testable under the repo's node vitest project:
// vite.config.ts runs the `server` project with environment:node and EXCLUDES
// `*.svelte.{test,spec}.ts` — a node test cannot import a .svelte file.
// inventory/+page.svelte imports these and __tests__/items.test.ts asserts them,
// so `npm test` covers the sort/filter logic even though the list/search/detail
// DOM render is DOM-blind here (a browser-smoke gap closed in 32-03's deploy).
//
// `is_mine` is SERVER-STAMPED on the rollup (compute.Items joins each item's
// holders against the viewer's character_assignment — Plan 32-01); the client
// NEVER recomputes assignment here. This is presentation ONLY — never access
// control (the read API serves the same guild-wide rollup to every session;
// mirrors the roster.ts / myview.ts T-27-01 negative property).

import type { ItemRollup, ItemHolder } from './api';

/** Stable viewer-first ordering: is_mine rows first, then A-Z (case-insensitive)
 *  by name. Returns a NEW array (never mutates the input). */
export function viewerFirstItems(rows: ItemRollup[]): ItemRollup[] {
	return [...rows].sort(
		(a, b) =>
			(a.is_mine ? 0 : 1) - (b.is_mine ? 0 : 1) ||
			a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
	);
}

/** Case-insensitive name filter that PRESERVES viewer-first order (ITEM-02 / D-02):
 *  matches stay ranked is_mine first, then A-Z. An empty/whitespace query returns
 *  the full set (viewer-first); a no-match query returns []. */
export function filterItems(rows: ItemRollup[], query: string): ItemRollup[] {
	const q = query.trim().toLowerCase();
	const matched = q === '' ? rows : rows.filter((r) => r.name.toLowerCase().includes(q));
	return viewerFirstItems(matched);
}

/** Holders-table viewer-first band order (UI-SPEC §F): the viewer's own characters
 *  first (band 0), then other guild characters (1), then banks/bots (2) — A-Z by
 *  char (case-insensitive) within each band. is_mine WINS the tie-break over
 *  is_bank (a viewer-owned bank toon ranks in "mine"). Returns a NEW array. */
export function sortHolders(holders: ItemHolder[]): ItemHolder[] {
	const band = (h: ItemHolder) => (h.is_mine ? 0 : h.is_bank ? 2 : 1);
	return [...holders].sort(
		(a, b) => band(a) - band(b) || a.char.localeCompare(b.char, undefined, { sensitivity: 'base' })
	);
}
