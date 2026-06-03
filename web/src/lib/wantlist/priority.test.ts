// Vitest (node project) for priorityRank + noteRuneCount (19-03 Task 1). Covers
// the rank mapping, the high→low ordering it produces, the else→0 fallthrough,
// and the UTF-aware rune count that mirrors the server's RuneCountInString cap.

import { describe, it, expect } from 'vitest';
import { priorityRank, noteRuneCount } from './priority';

describe('priorityRank', () => {
	it('maps high=3, med=2, low=1', () => {
		expect(priorityRank('high')).toBe(3);
		expect(priorityRank('med')).toBe(2);
		expect(priorityRank('low')).toBe(1);
	});

	it('falls through to 0 for any unrecognized value', () => {
		expect(priorityRank('')).toBe(0);
		expect(priorityRank('urgent')).toBe(0);
		expect(priorityRank('HIGH')).toBe(0); // case-sensitive — server stores lowercase
	});

	it('orders high → med → low when used as a sort key', () => {
		const sorted = ['low', 'high', 'med'].sort((a, b) => priorityRank(b) - priorityRank(a));
		expect(sorted).toEqual(['high', 'med', 'low']);
	});
});

describe('noteRuneCount', () => {
	it('counts plain ASCII by character', () => {
		expect(noteRuneCount('')).toBe(0);
		expect(noteRuneCount('hello')).toBe(5);
	});

	it('counts a non-BMP code point as ONE rune (not two UTF-16 units)', () => {
		// A single astral-plane emoji is `.length === 2` but ONE rune — the server
		// counts runes, so the counter must too (otherwise N/280 disagrees).
		const emoji = '😀';
		expect(emoji.length).toBe(2);
		expect(noteRuneCount(emoji)).toBe(1);
	});

	it('counts a mixed ASCII + emoji string by rune', () => {
		expect(noteRuneCount('ab😀')).toBe(3);
	});
});
