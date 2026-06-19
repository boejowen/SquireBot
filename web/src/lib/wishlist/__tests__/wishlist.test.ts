// Vitest for the pure wishlist helpers ($lib/wishlist/wishlist) — the node-only
// project (no jsdom / @testing-library). These prove the WISH-01 + WISH-07
// behavioral logic the Wishlist tab SHIPS (wishlist/+page.svelte imports the
// helpers rather than inlining the filter/grouping): the banks/bots-EXCLUDED
// viewer-first character list (WISH-01) and the cross-wishlist item search
// grouping over EVERY non-bank/bot character's wishlist (WISH-07 — the corpus
// is the whole passed-in set, never scoped to one selected character). The
// +page.svelte list/search/accordion RENDERING + selection is DOM-blind here
// (browser-smoke gap closed in 34-04's deploy). Mirrors the roster.test.ts /
// items.test.ts factory-fixture + describe/it idiom.

import { describe, it, expect } from 'vitest';
import type { RosterCharacter } from '../../api';
import {
	wishlistBandOf,
	wishlistRoster,
	filterWishlistRoster,
	searchWishlistItems
} from '../wishlist';

function char(over: Partial<RosterCharacter> = {}): RosterCharacter {
	return {
		name: 'Slampeach',
		level: 60,
		race: 'Human',
		class: 'Shadow Knight',
		is_mine: false,
		is_bank_toon: false,
		is_guild_bot: false,
		last_seen: '2026-06-18T00:00:00Z',
		...over
	};
}

// A minimal wishlist-shaped fixture for the WISH-07 cross-wishlist search. The
// helper is structurally-typed over { char, slots: [{ slot, targets: [{ item_name }] }] }
// — it does NOT need a full WishlistView (it ignores equipped/suggestions/price/…).
function wl(name: string, slots: { slot: string; items: string[] }[]) {
	return {
		char: name,
		slots: slots.map((s) => ({ slot: s.slot, targets: s.items.map((n) => ({ item_name: n })) }))
	};
}

describe('wishlistBandOf — two non-bank bands only (callers pre-filter banks/bots)', () => {
	it('returns mine for a viewer-owned character', () => {
		expect(wishlistBandOf(char({ is_mine: true }))).toBe('mine');
	});
	it('returns guild for a non-owned, non-bank character', () => {
		expect(wishlistBandOf(char({ is_mine: false }))).toBe('guild');
	});
});

describe('wishlistRoster — WISH-01: banks/bots EXCLUDED, viewer-first then A-Z', () => {
	it('drops every bank toon / guild bot (even a viewer-owned bank toon)', () => {
		const rows = [
			char({ name: 'Slampeach', is_mine: true }),
			char({ name: 'GuildBank', is_bank_toon: true }), // excluded
			char({ name: 'EcBot', is_guild_bot: true }), // excluded
			char({ name: 'MyBank', is_mine: true, is_bank_toon: true }), // excluded even though mine
			char({ name: 'Grimjaw' }) // guild
		];
		expect(wishlistRoster(rows).map((c) => c.name)).toEqual([
			'Slampeach', // mine
			'Grimjaw' // guild
		]);
	});

	it('orders mine → guild, A-Z within each band', () => {
		const rows = [
			char({ name: 'Zelda' }), // guild
			char({ name: 'Abe', is_mine: true }), // mine
			char({ name: 'Bo' }), // guild
			char({ name: 'Yara', is_mine: true }) // mine
		];
		expect(wishlistRoster(rows).map((c) => c.name)).toEqual(['Abe', 'Yara', 'Bo', 'Zelda']);
	});

	it('returns a NEW array (does not mutate the input order)', () => {
		const rows = [char({ name: 'Zed' }), char({ name: 'Aly', is_mine: true })];
		const out = wishlistRoster(rows);
		expect(out).not.toBe(rows);
		expect(rows.map((c) => c.name)).toEqual(['Zed', 'Aly']); // input untouched
	});
});

describe('filterWishlistRoster — WISH-01 name filter preserving viewer-first order', () => {
	const rows = [
		char({ name: 'Slampeach', is_mine: true }),
		char({ name: 'Grimjaw' }),
		char({ name: 'GuildBank', is_bank_toon: true }) // always excluded
	];

	it('empty query → the full banks/bots-excluded viewer-first set', () => {
		expect(filterWishlistRoster(rows, '').map((c) => c.name)).toEqual(['Slampeach', 'Grimjaw']);
		expect(filterWishlistRoster(rows, '   ').map((c) => c.name)).toEqual(['Slampeach', 'Grimjaw']);
	});

	it('case-insensitive name match, preserving viewer-first order', () => {
		expect(filterWishlistRoster(rows, 'gr').map((c) => c.name)).toEqual(['Grimjaw']);
		expect(filterWishlistRoster(rows, 'PEACH').map((c) => c.name)).toEqual(['Slampeach']);
	});

	it('never surfaces a bank/bot even if its name matches the query', () => {
		expect(filterWishlistRoster(rows, 'bank')).toEqual([]);
	});

	it('no match → []', () => {
		expect(filterWishlistRoster(rows, 'zzz')).toEqual([]);
	});
});

describe('searchWishlistItems — WISH-07: cross-wishlist item grouping over the WHOLE corpus', () => {
	const corpus = [
		wl('Slampeach', [
			{ slot: 'Back', items: ['Cloak of Flames'] },
			{ slot: 'Primary', items: ['Jagged Blade of War'] }
		]),
		wl('Grimjaw', [
			{ slot: 'Back', items: ['Cloak of Flames'] }, // same item, different char + same slot name
			{ slot: 'Head', items: ['Crown of King Tranix'] }
		]),
		wl('Yara', [{ slot: 'Chest', items: ['Cloak of Flames'] }]) // same item, third char
	];

	it('groups a matched item across ALL characters in the corpus, naming each (char, slot)', () => {
		const out = searchWishlistItems(corpus, 'cloak');
		expect(out.length).toBe(1);
		expect(out[0].item_name).toBe('Cloak of Flames');
		expect(out[0].where).toEqual([
			{ char: 'Slampeach', slot: 'Back' },
			{ char: 'Grimjaw', slot: 'Back' },
			{ char: 'Yara', slot: 'Chest' }
		]);
	});

	it('finds an item that lives ONLY on a character other than any "selected" one (no scope-down)', () => {
		// 'tranix' is only on Grimjaw — the corpus is the whole set, not one char.
		const out = searchWishlistItems(corpus, 'tranix');
		expect(out.map((r) => r.item_name)).toEqual(['Crown of King Tranix']);
		expect(out[0].where).toEqual([{ char: 'Grimjaw', slot: 'Head' }]);
	});

	it('case-insensitive substring match', () => {
		expect(searchWishlistItems(corpus, 'BLADE').map((r) => r.item_name)).toEqual([
			'Jagged Blade of War'
		]);
	});

	it('empty / whitespace query → []', () => {
		expect(searchWishlistItems(corpus, '')).toEqual([]);
		expect(searchWishlistItems(corpus, '   ')).toEqual([]);
	});

	it('no match → []', () => {
		expect(searchWishlistItems(corpus, 'zzz')).toEqual([]);
	});

	it('empty corpus → []', () => {
		expect(searchWishlistItems([], 'cloak')).toEqual([]);
	});
});
