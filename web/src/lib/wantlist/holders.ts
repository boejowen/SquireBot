// holders.ts — DOM-free in-guild "who holds it" grouping for the wantlist
// In-Guild Display Contract (19-UI-SPEC § In-Guild Display, D-06/D-07). Split
// out as a pure module so it is unit-testable under the node vitest project
// (the repo runs vitest with NO jsdom — the formatLastSeen precedent).
//
// REVIEW MUST-FIX 1 (load-bearing): the `view` payload (fetchView() → ViewRow[])
// is RAW per-row inventory data — one ViewRow per location/stack, with NO server
// GROUP BY (the backend InventoryJoin emits one row per inventory_item row). So a
// SINGLE character commonly holds the same item_id across MULTIPLE rows (worn +
// bank, or several stacks). holdersFor MUST reduce-by-char and SUM the counts so
// each character renders on EXACTLY ONE `↳ Char: count` line with its total — a
// plain map-not-reduce shape would render duplicate `↳ Borticus: 1` lines instead
// of the correct single `↳ Borticus: 2`.

import type { ViewRow } from '$lib/api';

/** One holding character + their SUMMED count for an item across all locations. */
export interface Holder {
	char: string;
	count: number;
}

/**
 * Group every ViewRow whose `id === itemId` by `char` and SUM their `count`,
 * returning one entry per character (the summed total), sorted by char
 * (localeCompare — the SearchResults holder order). A null `itemId` is a custom
 * want (D-07) — it is excluded from the join entirely and returns `[]`.
 */
export function holdersFor(itemId: number | null, viewRows: ViewRow[]): Holder[] {
	if (itemId === null) return []; // custom want → "—" (D-07): never joined.
	const byChar = new Map<string, number>();
	for (const r of viewRows) {
		if (r.id !== itemId) continue;
		// SUM per character — the same char recurs across location/stack rows.
		byChar.set(r.char, (byChar.get(r.char) ?? 0) + r.count);
	}
	return [...byChar.entries()]
		.map(([char, count]) => ({ char, count }))
		.sort((a, b) => a.char.localeCompare(b.char));
}
