// Vitest (node project) for holdersFor — the in-guild "who holds it" grouping
// (19-03 Task 1, 19-UI-SPEC § In-Guild Display Contract). The load-bearing case
// is review MUST-FIX 1: the `view` payload is RAW per-row data, so a character
// holding an item across several rows MUST collapse to ONE summed line.
//
// BUGFIX coverage (wantlist-in-guild-id-mismatch): holdersFor bridges by
// NORMALIZED NAME, not raw item_id — the catalog (pigparse_price) id and the
// inventory (EQ in-game) id are different namespaces. The regression test proves
// a same-name / different-id held item now reads "In guild".

import { describe, it, expect } from 'vitest';
import { holdersFor, normalizeItemName } from './holders';
import type { ViewRow } from '$lib/api';

// Minimal ViewRow factory — only the fields holdersFor reads (item, char, count)
// matter; the rest are filled with inert defaults so the type is satisfied. The
// `id` is still settable (and deliberately mismatched in the regression case) to
// prove the join NO LONGER depends on it.
function vr(
	partial: Partial<ViewRow> & { char: string; item: string; count: number }
): ViewRow {
	return {
		char: partial.char,
		slot: partial.slot ?? 'General1',
		item: partial.item,
		id: partial.id ?? 0,
		count: partial.count,
		wiki_url: '',
		price: null,
		last_synced: '',
		wiki_summary: '',
		is_quest_item: false,
		prices: [],
		quest_links: []
	};
}

describe('normalizeItemName', () => {
	it('lower-cases and trims (the gear_check normalized_name rule)', () => {
		expect(normalizeItemName('  Fine Steel Long Sword  ')).toBe('fine steel long sword');
		expect(normalizeItemName("10 Dose Ant's Potion")).toBe("10 dose ant's potion");
	});
});

describe('holdersFor', () => {
	it('returns [] for a null itemId (a custom want is never joined)', () => {
		const rows = [vr({ char: 'Borticus', item: 'Widget', count: 1 })];
		expect(holdersFor(null, 'Widget', rows)).toEqual([]);
	});

	it('returns [] when no row matches the want name', () => {
		const rows = [vr({ char: 'Borticus', item: 'Other Thing', count: 5 })];
		expect(holdersFor(42, 'Widget', rows)).toEqual([]);
	});

	it('returns [] for a blank want name (defensive)', () => {
		const rows = [vr({ char: 'Borticus', item: 'Widget', count: 1 })];
		expect(holdersFor(42, '   ', rows)).toEqual([]);
	});

	// THE REGRESSION CASE (wantlist-in-guild-id-mismatch): the want carries the
	// catalog id 19450 for "10 Dose Ant's Potion"; the holders' inventory rows carry
	// the EQ in-game id 14536 — DIFFERENT ids, SAME name. The old raw-id join read
	// "Not in guild"; the name-bridge correctly reports both holders.
	it('matches by NAME across mismatched id namespaces (catalog id ≠ inventory id)', () => {
		const wantCatalogId = 19450;
		const rows = [
			vr({ char: 'Findom', item: "10 Dose Ant's Potion", id: 14536, count: 1 }),
			vr({ char: 'Slampeach', item: "10 Dose Ant's Potion", id: 14536, count: 2 })
		];
		const result = holdersFor(wantCatalogId, "10 Dose Ant's Potion", rows);
		expect(result).toEqual([
			{ char: 'Findom', count: 1 },
			{ char: 'Slampeach', count: 2 }
		]);
	});

	it('matches case-insensitively and ignoring surrounding whitespace', () => {
		const rows = [vr({ char: 'Findom', item: '  fine STEEL Dagger ', count: 3 })];
		expect(holdersFor(7, 'Fine Steel Dagger', rows)).toEqual([{ char: 'Findom', count: 3 }]);
	});

	it('SUMS multiple rows for the SAME character into ONE line (review MUST-FIX 1)', () => {
		// Borticus holds the item in TWO rows (e.g. worn + bank) — must NOT render
		// two `↳ Borticus: 1` lines; it must be a SINGLE `↳ Borticus: 2`.
		const rows = [
			vr({ char: 'Borticus', item: 'Widget', count: 1, slot: 'Primary' }),
			vr({ char: 'Borticus', item: 'Widget', count: 1, slot: 'Bank1' })
		];
		const result = holdersFor(42, 'Widget', rows);
		expect(result).toHaveLength(1);
		expect(result[0]).toEqual({ char: 'Borticus', count: 2 });
	});

	it('sums stacks of differing counts for the same character', () => {
		const rows = [
			vr({ char: 'Slampeach', item: 'Bone Chips', count: 20, slot: 'Bank1' }),
			vr({ char: 'Slampeach', item: 'Bone Chips', count: 5, slot: 'Bank2' })
		];
		expect(holdersFor(7, 'Bone Chips', rows)).toEqual([{ char: 'Slampeach', count: 25 }]);
	});

	it('keeps distinct characters distinct and orders them by localeCompare', () => {
		const rows = [
			vr({ char: 'Slampeach', item: 'Widget', count: 3 }),
			vr({ char: 'Borticus', item: 'Widget', count: 1 }),
			vr({ char: 'Aldaron', item: 'Widget', count: 2 })
		];
		const result = holdersFor(42, 'Widget', rows);
		expect(result).toEqual([
			{ char: 'Aldaron', count: 2 },
			{ char: 'Borticus', count: 1 },
			{ char: 'Slampeach', count: 3 }
		]);
	});

	it('mixes multi-row-same-char sum with distinct chars correctly', () => {
		const rows = [
			vr({ char: 'Borticus', item: 'Widget', count: 1, slot: 'Primary' }),
			vr({ char: 'Borticus', item: 'Widget', count: 1, slot: 'Bank1' }),
			vr({ char: 'Slampeach', item: 'Widget', count: 4 }),
			// an unrelated item — must be ignored entirely.
			vr({ char: 'Borticus', item: 'Something Else', count: 50 })
		];
		const result = holdersFor(42, 'Widget', rows);
		expect(result).toEqual([
			{ char: 'Borticus', count: 2 },
			{ char: 'Slampeach', count: 4 }
		]);
	});

	it('ignores rows with other item names when filtering', () => {
		const rows = [
			vr({ char: 'A', item: 'Widget', count: 1 }),
			vr({ char: 'B', item: 'Gadget', count: 1 })
		];
		expect(holdersFor(1, 'Widget', rows)).toEqual([{ char: 'A', count: 1 }]);
	});
});
