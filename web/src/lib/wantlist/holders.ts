// holders.ts — DOM-free in-guild "who holds it" grouping for the wantlist
// In-Guild Display Contract (19-UI-SPEC § In-Guild Display, D-06/D-07). Split
// out as a pure module so it is unit-testable under the node vitest project
// (the repo runs vitest with NO jsdom — the formatLastSeen precedent).
//
// REVIEW MUST-FIX 1 (load-bearing): the `view` payload (fetchView() → ViewRow[])
// is RAW per-row inventory data — one ViewRow per location/stack, with NO server
// GROUP BY (the backend InventoryJoin emits one row per inventory_item row). So a
// SINGLE character commonly holds the same item across MULTIPLE rows (worn +
// bank, or several stacks). holdersFor MUST reduce-by-char and SUM the counts so
// each character renders on EXACTLY ONE `↳ Char: count` line with its total — a
// plain map-not-reduce shape would render duplicate `↳ Borticus: 1` lines instead
// of the correct single `↳ Borticus: 2`.
//
// BUGFIX (wantlist-in-guild-id-mismatch, 2026-06-06): the join MUST key on the
// NORMALIZED ITEM NAME, not the raw item_id. The wantlist catalog item_id comes
// from PigParse `pigparse_price` (e.g. "10 Dose Ant's Potion" = 19450), but the
// inventory ViewRow.id is the EQ in-game `/outputfile` id (= 14536) — two
// DIFFERENT id namespaces (only 58 of 713 inventory ids exist in the catalog by
// id, vs 559 names matching by name). Raw-id equality therefore false-negatives
// "Not in guild" for held items. The fix mirrors gear_check/spell_check, which
// deliberately bridge by item NAME via a materialized `normalized_name`
// (= lower(trim(name)); readviews.go:323-324, :362). We compute the same
// normalization client-side over WantlistRow.item_name vs ViewRow.item.

import type { ViewRow } from '$lib/api';

/** One holding character + their SUMMED count for an item across all locations. */
export interface Holder {
	char: string;
	count: number;
}

/**
 * Normalize an item name for cross-namespace matching: lower(trim(name)) — the
 * exact convention the server materializes as `normalized_name` for the
 * gear_check / spell_check name-bridge (readviews.go:362). Keeping the rule
 * identical here means the catalog name and the inventory name collapse to the
 * same key whenever they are the same item under different ids.
 */
export function normalizeItemName(name: string): string {
	return name.trim().toLowerCase();
}

/**
 * Group every ViewRow whose normalized `item` name === the want's normalized
 * `itemName` by `char` and SUM their `count`, returning one entry per character
 * (the summed total), sorted by char (localeCompare — the SearchResults holder
 * order).
 *
 * Matching is by NORMALIZED NAME, not item_id: the wantlist catalog id
 * (pigparse_price) and the inventory id (EQ in-game) are different namespaces, so
 * an id join false-negatives held items (see file header). A custom want has
 * itemId === null (D-07) — it is excluded from the join entirely and returns `[]`.
 * The `itemId` is retained in the signature for the null-custom-want gate and for
 * call-site clarity; only `itemName` is used for the actual match.
 */
export function holdersFor(
	itemId: number | null,
	itemName: string,
	viewRows: ViewRow[]
): Holder[] {
	if (itemId === null) return []; // custom want → "—" (D-07): never joined.
	const want = normalizeItemName(itemName);
	if (want === '') return []; // defensive: an empty/blank name matches nothing.
	const byChar = new Map<string, number>();
	for (const r of viewRows) {
		if (normalizeItemName(r.item) !== want) continue;
		// SUM per character — the same char recurs across location/stack rows.
		byChar.set(r.char, (byChar.get(r.char) ?? 0) + r.count);
	}
	return [...byChar.entries()]
		.map(([char, count]) => ({ char, count }))
		.sort((a, b) => a.char.localeCompare(b.char));
}
