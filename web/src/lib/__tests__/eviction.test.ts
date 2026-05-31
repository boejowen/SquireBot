// Vitest for the pure eviction grace-date helper (CR-02). Proves the
// epoch-SECONDS → human-date contract under the repo's node-only test project
// (the form .svelte is a thin renderer over it). This is the regression that the
// prior tests could not catch: nothing exercised the real JSON shape (an epoch
// SECONDS number) against new Date()'s MILLISECONDS expectation, so the "Jan 1970"
// bug shipped green.

import { describe, it, expect } from 'vitest';
import { graceDate } from '../eviction';

describe('graceDate — epoch SECONDS → human date (CR-02)', () => {
	it('converts an epoch-SECONDS value to its real date, NOT 1970', () => {
		// 1782789805 s = 2026-06-30T07:23:25Z. The bug fed this straight to
		// new Date() (which reads MILLISECONDS) and produced "Wed Jan 21 1970".
		const secs = 1782789805;
		const out = graceDate(secs);
		expect(out).not.toContain('1970');
		// Same instant, computed the correct way (seconds→ms) — must match exactly.
		expect(out).toBe(new Date(secs * 1000).toDateString());
		// And it is the year the epoch-seconds value actually denotes.
		expect(out).toContain('2026');
	});

	it('a known epoch-seconds midnight maps to the expected calendar date', () => {
		// 2026-07-30T00:00:00Z in seconds.
		const secs = Math.floor(Date.UTC(2026, 6, 30, 0, 0, 0) / 1000);
		expect(graceDate(secs)).toBe(new Date(secs * 1000).toDateString());
	});

	it('0 seconds maps to the epoch instant (treated as seconds, not a falsy passthrough)', () => {
		// 0s = 0ms = the epoch. Compared against the correctly-computed value so the
		// assertion is timezone-robust (the local date for the epoch is "Dec 31 1969"
		// at negative UTC offsets, "Jan 1 1970" otherwise — both are correct).
		expect(graceDate(0)).toBe(new Date(0).toDateString());
	});

	it('a non-finite value falls back to its string form (never "Invalid Date")', () => {
		expect(graceDate(NaN)).toBe('NaN');
		expect(graceDate(Infinity)).toBe('Infinity');
	});
});
