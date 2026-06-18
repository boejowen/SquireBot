// Vitest for the pure roster helpers ($lib/roster) — the node-only project (no
// jsdom / @testing-library). These prove the D-10 behavioral logic the Characters
// tab SHIPS (+page.svelte imports the helpers rather than inlining the sort/filter):
// viewer-first banding (mine → guild → banks, A-Z within each), the is_mine
// tie-break, and the viewer-priority case-insensitive search. The +page.svelte
// list/search RENDERING + selection is DOM-blind here (browser-smoke gap closed in
// 31-04's deploy). Mirrors the myview.test.ts factory-fixture + describe/it idiom.

import { describe, it, expect } from 'vitest';
import type { RosterCharacter } from '../api';
import { bandOf, viewerFirst, filterRoster } from '../roster';

function char(over: Partial<RosterCharacter> = {}): RosterCharacter {
	return {
		name: 'Slampeach',
		level: 60,
		race: 'Half Elf',
		class: 'Ranger',
		is_mine: false,
		is_bank_toon: false,
		is_guild_bot: false,
		last_seen: '2026-06-18T00:00:00Z',
		...over
	};
}

describe('bandOf — D-10 band classification', () => {
	it('a viewer-owned char is "mine"', () => {
		expect(bandOf(char({ is_mine: true }))).toBe('mine');
	});
	it('a bank toon is "banks"', () => {
		expect(bandOf(char({ is_bank_toon: true }))).toBe('banks');
	});
	it('a guild bot is "banks"', () => {
		expect(bandOf(char({ is_guild_bot: true }))).toBe('banks');
	});
	it('a plain guild char is "guild"', () => {
		expect(bandOf(char())).toBe('guild');
	});
	it('is_mine WINS the tie-break over bank/bot (a viewer-owned bank toon is "mine")', () => {
		expect(bandOf(char({ is_mine: true, is_bank_toon: true }))).toBe('mine');
		expect(bandOf(char({ is_mine: true, is_guild_bot: true }))).toBe('mine');
	});
});

describe('viewerFirst — mine → guild → banks, A-Z within each', () => {
	it('orders the bands mine, then guild, then banks/bots', () => {
		const rows = [
			char({ name: 'Findom', is_bank_toon: true }), // banks
			char({ name: 'Otherguy' }), // guild
			char({ name: 'Slampeach', is_mine: true }) // mine
		];
		expect(viewerFirst(rows).map((c) => c.name)).toEqual(['Slampeach', 'Otherguy', 'Findom']);
	});

	it('sorts A-Z (case-insensitive) WITHIN each band', () => {
		const rows = [
			char({ name: 'zelda', is_mine: true }),
			char({ name: 'Aragorn', is_mine: true }),
			char({ name: 'mab' }),
			char({ name: 'Bob' }),
			char({ name: 'Yak', is_guild_bot: true }),
			char({ name: 'aaaBank', is_bank_toon: true })
		];
		expect(viewerFirst(rows).map((c) => c.name)).toEqual([
			'Aragorn',
			'zelda', // mine, A-Z (case-insensitive)
			'Bob',
			'mab', // guild, A-Z (case-insensitive)
			'aaaBank',
			'Yak' // banks, A-Z (case-insensitive)
		]);
	});

	it('a viewer-owned bank toon ranks in the mine band, not banks (tie-break)', () => {
		const rows = [
			char({ name: 'OtherBank', is_bank_toon: true }),
			char({ name: 'MyBank', is_mine: true, is_bank_toon: true })
		];
		const out = viewerFirst(rows);
		expect(out.map((c) => c.name)).toEqual(['MyBank', 'OtherBank']);
		expect(bandOf(out[0])).toBe('mine');
	});

	it('returns a NEW array (does not mutate the input)', () => {
		const rows = [char({ name: 'Bob' }), char({ name: 'Ann', is_mine: true })];
		const out = viewerFirst(rows);
		expect(out).not.toBe(rows);
		expect(rows.map((c) => c.name)).toEqual(['Bob', 'Ann']); // input untouched
	});
});

describe('filterRoster — viewer-priority case-insensitive search (CHAR-02 / D-10)', () => {
	const rows = [
		char({ name: 'Frodo', is_mine: true }),
		char({ name: 'Frodette' }), // guild, also matches "frod"
		char({ name: 'Samwise' }),
		char({ name: 'Frodobank', is_bank_toon: true }) // banks, also matches "frod"
	];

	it('an empty query returns the full set viewer-first', () => {
		expect(filterRoster(rows, '').map((c) => c.name)).toEqual([
			'Frodo', // mine
			'Frodette',
			'Samwise', // guild
			'Frodobank' // banks
		]);
	});

	it('a whitespace-only query is treated as empty (full set, viewer-first)', () => {
		expect(filterRoster(rows, '   ').map((c) => c.name)).toEqual([
			'Frodo',
			'Frodette',
			'Samwise',
			'Frodobank'
		]);
	});

	it('a substring query is case-insensitive and KEEPS viewer-first ranking among matches', () => {
		// "frodo" matches Frodo (mine) + Frodobank (banks), NOT Frodette/Samwise.
		expect(filterRoster(rows, 'FRODO').map((c) => c.name)).toEqual(['Frodo', 'Frodobank']);
		// "frod" matches all three frod* — viewer's Frodo first, then guild Frodette, then bank.
		expect(filterRoster(rows, 'frod').map((c) => c.name)).toEqual([
			'Frodo',
			'Frodette',
			'Frodobank'
		]);
	});

	it('a no-match query returns []', () => {
		expect(filterRoster(rows, 'gandalf')).toEqual([]);
	});

	it('does not mutate the input array', () => {
		const before = rows.map((c) => c.name);
		filterRoster(rows, 'frod');
		expect(rows.map((c) => c.name)).toEqual(before);
	});
});
