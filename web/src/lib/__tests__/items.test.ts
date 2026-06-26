// Vitest for the pure item helpers ($lib/items) — the node-only project (no
// jsdom / @testing-library). These prove the ITEM-02 behavioral logic the
// Inventory tab SHIPS (inventory/+page.svelte imports the helpers rather than
// inlining the sort/filter): viewer-first ordering (is_mine first, then A-Z),
// the viewer-priority case-insensitive search, and the holders-table band sort
// (mine → guild → banks, A-Z within each). The +page.svelte list/search/detail
// RENDERING + selection is DOM-blind here (browser-smoke gap closed in 32-03's
// deploy). Mirrors the roster.test.ts factory-fixture + describe/it idiom.

import { describe, it, expect } from 'vitest';
import type { ItemRollup, ItemHolder } from '../api';
import { viewerFirstItems, filterItems, sortHolders, facetItems } from '../items';

function item(over: Partial<ItemRollup> = {}): ItemRollup {
	return {
		name: 'Cloak of Flames',
		summed_qty: 1,
		holder_count: 1,
		is_mine: false,
		price: null,
		prices: [],
		wiki_url: '',
		wiki_summary: '',
		is_quest_item: false,
		icon_id: 0,
		statsblock: '',
		is_clicky: false,
		has_haste: false,
		is_no_drop: false,
		is_lore: false,
		is_magic: false,
		quest_links: [],
		holders: [],
		...over
	};
}

function holder(over: Partial<ItemHolder> = {}): ItemHolder {
	return {
		char: 'Slampeach',
		slot_label: 'Worn · Back',
		qty: 1,
		last_synced: '2026-06-18T00:00:00Z',
		is_mine: false,
		is_bank: false,
		...over
	};
}

describe('viewerFirstItems — is_mine first, then A-Z (case-insensitive)', () => {
	it('floats the viewer’s items to the top, then alphabetical', () => {
		const rows = [
			item({ name: 'Bone Chips' }), // not mine
			item({ name: 'Velium Battle Axe', is_mine: true }), // mine
			item({ name: 'Acrylia Reinforced Backpack' }), // not mine
			item({ name: 'Cloak of Flames', is_mine: true }) // mine
		];
		expect(viewerFirstItems(rows).map((r) => r.name)).toEqual([
			'Cloak of Flames',
			'Velium Battle Axe', // mine, A-Z
			'Acrylia Reinforced Backpack',
			'Bone Chips' // not mine, A-Z
		]);
	});

	it('sorts A-Z case-insensitively within each band', () => {
		const rows = [
			item({ name: 'zebra band', is_mine: true }),
			item({ name: 'Apple band', is_mine: true }),
			item({ name: 'mango' }),
			item({ name: 'Banana' })
		];
		expect(viewerFirstItems(rows).map((r) => r.name)).toEqual([
			'Apple band',
			'zebra band', // mine, A-Z (case-insensitive)
			'Banana',
			'mango' // not mine, A-Z (case-insensitive)
		]);
	});

	it('returns a NEW array (does not mutate the input)', () => {
		const rows = [item({ name: 'Bone Chips' }), item({ name: 'Acrylia', is_mine: true })];
		const out = viewerFirstItems(rows);
		expect(out).not.toBe(rows);
		expect(rows.map((r) => r.name)).toEqual(['Bone Chips', 'Acrylia']); // input untouched
	});
});

describe('filterItems — viewer-priority case-insensitive search (ITEM-02 / D-02)', () => {
	const rows = [
		item({ name: 'Cloak of Flames', is_mine: true }),
		item({ name: 'Cloak of the Akheva' }), // not mine, also matches "cloak"
		item({ name: 'Bone Chips' }),
		item({ name: 'Flame Lick' }) // not mine, also matches "flame"
	];

	it('an empty query returns the full set viewer-first', () => {
		expect(filterItems(rows, '').map((r) => r.name)).toEqual([
			'Cloak of Flames', // mine
			'Bone Chips',
			'Cloak of the Akheva',
			'Flame Lick' // not mine, A-Z
		]);
	});

	it('a whitespace-only query is treated as empty (full set, viewer-first)', () => {
		expect(filterItems(rows, '   ').map((r) => r.name)).toEqual([
			'Cloak of Flames',
			'Bone Chips',
			'Cloak of the Akheva',
			'Flame Lick'
		]);
	});

	it('a substring query is case-insensitive and KEEPS viewer-first ranking among matches', () => {
		// "cloak" matches Cloak of Flames (mine) + Cloak of the Akheva (not mine).
		expect(filterItems(rows, 'CLOAK').map((r) => r.name)).toEqual([
			'Cloak of Flames',
			'Cloak of the Akheva'
		]);
		// "flame" matches Cloak of Flames (mine) + Flame Lick (not mine) — mine first.
		expect(filterItems(rows, 'flame').map((r) => r.name)).toEqual(['Cloak of Flames', 'Flame Lick']);
	});

	it('a no-match query returns []', () => {
		expect(filterItems(rows, 'gnomish heat source')).toEqual([]);
	});

	it('does not mutate the input array', () => {
		const before = rows.map((r) => r.name);
		filterItems(rows, 'cloak');
		expect(rows.map((r) => r.name)).toEqual(before);
	});
});

describe('facetItems — AND-combined Clicky/Haste facet (D-02)', () => {
	// A mixed set: one clicky-only, one haste-only, one both, one neither.
	const rows = [
		item({ name: 'Reaper of the Dead', is_clicky: true, has_haste: false }),
		item({ name: 'Robe of the Lost Circle', is_clicky: false, has_haste: true }),
		item({ name: 'Jade Reaver', is_clicky: true, has_haste: true }),
		item({ name: 'Bone Chips', is_clicky: false, has_haste: false })
	];

	it('neither flag set → pass-through (the full set; no facet filter)', () => {
		expect(facetItems(rows, { clicky: false, haste: false }).map((r) => r.name)).toEqual([
			'Reaper of the Dead',
			'Robe of the Lost Circle',
			'Jade Reaver',
			'Bone Chips'
		]);
	});

	it('clicky-only → only is_clicky rows', () => {
		expect(facetItems(rows, { clicky: true, haste: false }).map((r) => r.name)).toEqual([
			'Reaper of the Dead',
			'Jade Reaver'
		]);
	});

	it('haste-only → only has_haste rows', () => {
		expect(facetItems(rows, { clicky: false, haste: true }).map((r) => r.name)).toEqual([
			'Robe of the Lost Circle',
			'Jade Reaver'
		]);
	});

	it('both flags set → the intersection (rows that are BOTH clicky AND haste)', () => {
		expect(facetItems(rows, { clicky: true, haste: true }).map((r) => r.name)).toEqual([
			'Jade Reaver'
		]);
	});

	it('an empty AND-combination (a set with no row that is both) returns []', () => {
		const noBoth = [
			item({ name: 'Clicky Only', is_clicky: true, has_haste: false }),
			item({ name: 'Haste Only', is_clicky: false, has_haste: true })
		];
		expect(facetItems(noBoth, { clicky: true, haste: true })).toEqual([]);
	});

	it('returns a NEW array (does not mutate the input)', () => {
		const before = rows.map((r) => r.name);
		const out = facetItems(rows, { clicky: true, haste: false });
		expect(out).not.toBe(rows);
		expect(rows.map((r) => r.name)).toEqual(before); // input untouched
	});
});

describe('sortHolders — mine → guild → banks, A-Z within each band', () => {
	it('orders the bands mine, then guild, then banks/bots', () => {
		const holders = [
			holder({ char: 'Findom', is_bank: true }), // banks
			holder({ char: 'Otherguy' }), // guild
			holder({ char: 'Slampeach', is_mine: true }) // mine
		];
		expect(sortHolders(holders).map((h) => h.char)).toEqual(['Slampeach', 'Otherguy', 'Findom']);
	});

	it('sorts A-Z (case-insensitive) WITHIN each band', () => {
		const holders = [
			holder({ char: 'zelda', is_mine: true }),
			holder({ char: 'Aragorn', is_mine: true }),
			holder({ char: 'mab' }),
			holder({ char: 'Bob' }),
			holder({ char: 'zzBank', is_bank: true }),
			holder({ char: 'aaaBank', is_bank: true })
		];
		expect(sortHolders(holders).map((h) => h.char)).toEqual([
			'Aragorn',
			'zelda', // mine, A-Z (case-insensitive)
			'Bob',
			'mab', // guild, A-Z (case-insensitive)
			'aaaBank',
			'zzBank' // banks, A-Z (case-insensitive)
		]);
	});

	it('a viewer-owned bank holder ranks in the mine band (is_mine wins the tie-break)', () => {
		const holders = [
			holder({ char: 'OtherBank', is_bank: true }),
			holder({ char: 'MyBank', is_mine: true, is_bank: true })
		];
		expect(sortHolders(holders).map((h) => h.char)).toEqual(['MyBank', 'OtherBank']);
	});

	it('returns a NEW array (does not mutate the input)', () => {
		const holders = [holder({ char: 'Bob' }), holder({ char: 'Ann', is_mine: true })];
		const out = sortHolders(holders);
		expect(out).not.toBe(holders);
		expect(holders.map((h) => h.char)).toEqual(['Bob', 'Ann']); // input untouched
	});
});
