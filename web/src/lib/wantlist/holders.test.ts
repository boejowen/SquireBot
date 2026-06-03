// Vitest (node project) for holdersFor — the in-guild "who holds it" grouping
// (19-03 Task 1, 19-UI-SPEC § In-Guild Display Contract). The load-bearing case
// is review MUST-FIX 1: the `view` payload is RAW per-row data, so a character
// holding an item across several rows MUST collapse to ONE summed line.

import { describe, it, expect } from 'vitest';
import { holdersFor } from './holders';
import type { ViewRow } from '$lib/api';

// Minimal ViewRow factory — only the fields holdersFor reads (id, char, count)
// matter; the rest are filled with inert defaults so the type is satisfied.
function vr(partial: Partial<ViewRow> & { char: string; id: number; count: number }): ViewRow {
	return {
		char: partial.char,
		slot: partial.slot ?? 'General1',
		item: partial.item ?? 'Item',
		id: partial.id,
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

describe('holdersFor', () => {
	it('returns [] for a null itemId (a custom want is never joined)', () => {
		const rows = [vr({ char: 'Borticus', id: 42, count: 1 })];
		expect(holdersFor(null, rows)).toEqual([]);
	});

	it('returns [] when no row matches the itemId', () => {
		const rows = [vr({ char: 'Borticus', id: 99, count: 5 })];
		expect(holdersFor(42, rows)).toEqual([]);
	});

	it('SUMS multiple rows for the SAME character into ONE line (review MUST-FIX 1)', () => {
		// Borticus holds item 42 in TWO rows (e.g. worn + bank) — must NOT render
		// two `↳ Borticus: 1` lines; it must be a SINGLE `↳ Borticus: 2`.
		const rows = [
			vr({ char: 'Borticus', id: 42, count: 1, slot: 'Primary' }),
			vr({ char: 'Borticus', id: 42, count: 1, slot: 'Bank1' })
		];
		const result = holdersFor(42, rows);
		expect(result).toHaveLength(1);
		expect(result[0]).toEqual({ char: 'Borticus', count: 2 });
	});

	it('sums stacks of differing counts for the same character', () => {
		const rows = [
			vr({ char: 'Slampeach', id: 7, count: 20, slot: 'Bank1' }),
			vr({ char: 'Slampeach', id: 7, count: 5, slot: 'Bank2' })
		];
		expect(holdersFor(7, rows)).toEqual([{ char: 'Slampeach', count: 25 }]);
	});

	it('keeps distinct characters distinct and orders them by localeCompare', () => {
		const rows = [
			vr({ char: 'Slampeach', id: 42, count: 3 }),
			vr({ char: 'Borticus', id: 42, count: 1 }),
			vr({ char: 'Aldaron', id: 42, count: 2 })
		];
		const result = holdersFor(42, rows);
		expect(result).toEqual([
			{ char: 'Aldaron', count: 2 },
			{ char: 'Borticus', count: 1 },
			{ char: 'Slampeach', count: 3 }
		]);
	});

	it('mixes multi-row-same-char sum with distinct chars correctly', () => {
		const rows = [
			vr({ char: 'Borticus', id: 42, count: 1, slot: 'Primary' }),
			vr({ char: 'Borticus', id: 42, count: 1, slot: 'Bank1' }),
			vr({ char: 'Slampeach', id: 42, count: 4 }),
			// an unrelated item id — must be ignored entirely.
			vr({ char: 'Borticus', id: 99, count: 50 })
		];
		const result = holdersFor(42, rows);
		expect(result).toEqual([
			{ char: 'Borticus', count: 2 },
			{ char: 'Slampeach', count: 4 }
		]);
	});

	it('ignores rows with other item ids when filtering', () => {
		const rows = [
			vr({ char: 'A', id: 1, count: 1 }),
			vr({ char: 'B', id: 2, count: 1 })
		];
		expect(holdersFor(1, rows)).toEqual([{ char: 'A', count: 1 }]);
	});
});
