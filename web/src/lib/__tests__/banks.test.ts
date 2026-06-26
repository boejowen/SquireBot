// Vitest for the pure bank helpers ($lib/banks) — the node-only project (no jsdom /
// @testing-library). These prove the BANK-01/03 behavioral logic the Banks tab SHIPS
// (banks/+page.svelte imports the helpers rather than inlining the sort/filter): the
// plain A-Z bank sort (D-01 — banks aren't viewer-scoped), the is_bank-scoped item
// search with the Pitfall-3 bank-slice qty RECOMPUTE (the guild-wide rollup totals
// must NOT leak through), and the bankByName lookup. The +page.svelte list/search/
// selection RENDERING is DOM-blind here (browser-smoke gap closed in 33-03's deploy).
// Mirrors the items.test.ts / roster.test.ts factory-fixture + describe/it idiom.

import { describe, it, expect } from 'vitest';
import type { BankRowSummary, ItemRollup, ItemHolder } from '../api';
import { sortBanksAZ, bankItemSearch, bankByName } from '../banks';

function bank(over: Partial<BankRowSummary> = {}): BankRowSummary {
	return {
		name: 'Guildbank1',
		item_count: 1,
		value: 0,
		unpriced: 0,
		plat: null,
		...over
	};
}

function holder(over: Partial<ItemHolder> = {}): ItemHolder {
	return {
		char: 'Guildbank1',
		slot_label: 'Bank',
		qty: 1,
		last_synced: '2026-06-18T00:00:00Z',
		is_mine: false,
		is_bank: true,
		...over
	};
}

function item(over: Partial<ItemRollup> = {}): ItemRollup {
	return {
		name: 'Blue Diamond',
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

describe('sortBanksAZ — plain A-Z (case-insensitive), NOT viewer-first', () => {
	it('orders banks alphabetically, case-insensitively', () => {
		const banks = [
			bank({ name: 'zebra-bank' }),
			bank({ name: 'Apple-bank' }),
			bank({ name: 'mango-bank' }),
			bank({ name: 'Banana-bank' })
		];
		expect(sortBanksAZ(banks).map((b) => b.name)).toEqual([
			'Apple-bank',
			'Banana-bank',
			'mango-bank',
			'zebra-bank'
		]);
	});

	it('returns a NEW array (does not mutate the input)', () => {
		const banks = [bank({ name: 'Zeta' }), bank({ name: 'Alpha' })];
		const out = sortBanksAZ(banks);
		expect(out).not.toBe(banks);
		expect(banks.map((b) => b.name)).toEqual(['Zeta', 'Alpha']); // input untouched
	});
});

describe('bankItemSearch — is_bank-scoped, bank-slice qty recompute (Pitfall 3)', () => {
	it('drops an item whose only holders are non-bank', () => {
		const rows = [
			item({
				name: 'Bone Chips',
				holders: [holder({ char: 'Slampeach', is_bank: false })]
			})
		];
		expect(bankItemSearch(rows, '')).toEqual([]);
	});

	it('keeps ONLY the is_bank holders within a kept (mixed) item', () => {
		const rows = [
			item({
				name: 'Cloak of Flames',
				holders: [
					holder({ char: 'Guildbank1', is_bank: true }),
					holder({ char: 'Slampeach', is_bank: false }) // a personal holder — must be dropped
				]
			})
		];
		const out = bankItemSearch(rows, '');
		expect(out).toHaveLength(1);
		expect(out[0].holders.map((h) => h.char)).toEqual(['Guildbank1']);
	});

	it('RECOMPUTES summed_qty/holder_count from the bank slice, not the guild-wide rollup (Pitfall 3)', () => {
		// The "Blue Diamond 40× guild / 3× across 2 banks" trap: the ItemRollup carries the
		// GUILD-WIDE summed_qty=40 / holder_count=8, but only 2 bank holders (qty 1 + 2 = 3)
		// are in scope. The result MUST read 3 / 2, never the guild-wide 40 / 8.
		const rows = [
			item({
				name: 'Blue Diamond',
				summed_qty: 40, // guild-wide (incl. personal holdings) — must NOT pass through
				holder_count: 8, // guild-wide
				holders: [
					holder({ char: 'Guildbank1', qty: 1, is_bank: true }),
					holder({ char: 'Guildbank2', qty: 2, is_bank: true }),
					holder({ char: 'Slampeach', qty: 37, is_bank: false }) // personal — dropped
				]
			})
		];
		const out = bankItemSearch(rows, '');
		expect(out).toHaveLength(1);
		expect(out[0].summed_qty).toBe(3); // 1 + 2 across the two banks — NOT 40
		expect(out[0].holder_count).toBe(2); // two distinct banks — NOT 8
		expect(out[0].holders).toHaveLength(2);
	});

	it('counts DISTINCT bank characters for holder_count (two holdings on one bank → 1)', () => {
		const rows = [
			item({
				name: 'Water Flask',
				holders: [
					holder({ char: 'Guildbank1', slot_label: 'Bag', qty: 5, is_bank: true }),
					holder({ char: 'Guildbank1', slot_label: 'Bank', qty: 3, is_bank: true })
				]
			})
		];
		const out = bankItemSearch(rows, '');
		expect(out[0].summed_qty).toBe(8); // 5 + 3
		expect(out[0].holder_count).toBe(1); // ONE distinct bank
	});

	it('name-filters (trim + case-insensitive includes); a non-matching query drops the item', () => {
		const rows = [
			item({ name: 'Blue Diamond', holders: [holder({ is_bank: true })] }),
			item({ name: 'Bone Chips', holders: [holder({ char: 'Guildbank2', is_bank: true })] })
		];
		expect(bankItemSearch(rows, 'DIAMOND').map((r) => r.name)).toEqual(['Blue Diamond']);
		expect(bankItemSearch(rows, '  diamond  ').map((r) => r.name)).toEqual(['Blue Diamond']);
		expect(bankItemSearch(rows, 'gnomish heat source')).toEqual([]);
	});

	it('an empty query keeps ALL bank-holding items, returned A-Z', () => {
		const rows = [
			item({ name: 'Zebra Hide', holders: [holder({ char: 'Guildbank2', is_bank: true })] }),
			item({ name: 'Apple', holders: [holder({ char: 'Guildbank1', is_bank: true })] }),
			item({ name: 'mango', holders: [holder({ char: 'Guildbank3', is_bank: true })] })
		];
		expect(bankItemSearch(rows, '').map((r) => r.name)).toEqual(['Apple', 'mango', 'Zebra Hide']);
	});

	it('does not mutate the input rows', () => {
		const rows = [
			item({
				name: 'Blue Diamond',
				summed_qty: 40,
				holders: [holder({ is_bank: true }), holder({ char: 'X', is_bank: false })]
			})
		];
		bankItemSearch(rows, '');
		expect(rows[0].summed_qty).toBe(40); // untouched
		expect(rows[0].holders).toHaveLength(2); // untouched
	});
});

describe('bankByName — find a row, undefined for a miss', () => {
	it('finds a row by exact name', () => {
		const banks = [bank({ name: 'Guildbank1' }), bank({ name: 'Guildbank2', value: 99 })];
		expect(bankByName(banks, 'Guildbank2')?.value).toBe(99);
	});

	it('returns undefined for a name not in the list', () => {
		const banks = [bank({ name: 'Guildbank1' })];
		expect(bankByName(banks, 'Nope')).toBeUndefined();
	});
});
