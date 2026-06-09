// Vitest for the pure "My characters" filter helpers ($lib/myview) — the node-only
// project (no jsdom / @testing-library). These prove the behavioral logic the
// consolidated views SHIP (+page.svelte imports the helpers rather than inlining the
// filter), so `npm test` covers the passthrough / mine-only / drill-down / empty-mine /
// name-join-exactness predicate even though the +page.svelte <select> RENDERING +
// onchange is DOM-blind here (that browser-smoke gap stays flagged for /gsd-ui-review).
// Mirrors the assignments.test.ts factory-fixture + describe/it idiom.

import { describe, it, expect } from 'vitest';
import type { MyCharacter } from '../api';
import { myCharNameSet, applyMyFilter } from '../myview';

function mine(over: Partial<MyCharacter> = {}): MyCharacter {
	return {
		character_id: 1,
		name: 'Slampeach',
		discord_user_id: 'discord-1',
		assigned_at: 0,
		assigned_by: 'self',
		...over
	};
}

function row(over: Partial<{ char: string }> = {}): { char: string } {
	return { char: 'Slampeach', ...over };
}

describe('myCharNameSet — the caller-assigned-name join key set', () => {
	it('builds a (lower-cased) Set of the MyCharacter.name strings', () => {
		const set = myCharNameSet([mine({ name: 'Slampeach' }), mine({ name: 'Findom' })]);
		expect(set.has('slampeach')).toBe(true);
		expect(set.has('findom')).toBe(true);
		expect(set.has('nobody')).toBe(false);
		expect(set.size).toBe(2);
	});

	it('an empty mine list yields an empty set', () => {
		expect(myCharNameSet([]).size).toBe(0);
	});

	it('collapses names differing only by case to a single entry (IN-03)', () => {
		expect(myCharNameSet([mine({ name: 'Slampeach' }), mine({ name: 'SLAMPEACH' })]).size).toBe(1);
	});
});

describe('applyMyFilter — narrow char-bearing rows to the caller', () => {
	const rows = [row({ char: 'Slampeach' }), row({ char: 'Findom' }), row({ char: 'Otherguy' })];
	const names = myCharNameSet([mine({ name: 'Slampeach' }), mine({ name: 'Findom' })]);

	it('passthrough: mineOnly=false, selectedChar=null → returns rows UNCHANGED (additive default, MYVIEW-01)', () => {
		const out = applyMyFilter(rows, names, false, null);
		expect(out).toBe(rows); // same reference — no copy when passing through
		expect(out.map((r) => r.char)).toEqual(['Slampeach', 'Findom', 'Otherguy']);
	});

	it('mine-only: mineOnly=true, selectedChar=null → only rows whose char is in the my-names set', () => {
		const out = applyMyFilter(rows, names, true, null);
		expect(out.map((r) => r.char)).toEqual(['Slampeach', 'Findom']);
	});

	it('drill-down DOMINATES: selectedChar narrows to just that char regardless of mineOnly (MYVIEW-02)', () => {
		// selectedChar set, mineOnly false → just that char.
		expect(applyMyFilter(rows, names, false, 'Slampeach').map((r) => r.char)).toEqual(['Slampeach']);
		// selectedChar set, mineOnly true → STILL just that char (drill-down wins over mine-only).
		expect(applyMyFilter(rows, names, true, 'Findom').map((r) => r.char)).toEqual(['Findom']);
		// drill into a char NOT in the mine set is honored (it's an all-members grid value).
		expect(applyMyFilter(rows, names, true, 'Otherguy').map((r) => r.char)).toEqual(['Otherguy']);
	});

	it('empty mine: mineOnly=true with an empty names set → returns []', () => {
		expect(applyMyFilter(rows, new Set<string>(), true, null)).toEqual([]);
	});

	it('empty-string selectedChar is falsy → falls through, no accidental drill-down (IN-03)', () => {
		// '' must NOT be treated as a drill-down target — it falls through to the mineOnly branch.
		expect(applyMyFilter(rows, names, true, '').map((r) => r.char)).toEqual(['Slampeach', 'Findom']); // mine-only
		expect(applyMyFilter(rows, names, false, '')).toBe(rows); // passthrough (same ref)
	});

	it('name-join exactness: an in-set char is included, an out-of-set char is excluded', () => {
		const out = applyMyFilter(rows, names, true, null);
		expect(out.some((r) => r.char === 'Slampeach')).toBe(true); // in set → included
		expect(out.some((r) => r.char === 'Otherguy')).toBe(false); // not in set → excluded
	});

	it('case-insensitive defensive join: a casing-drifted row still matches its name (belt-and-suspenders)', () => {
		const out = applyMyFilter([row({ char: 'SLAMPEACH' })], names, true, null);
		expect(out.map((r) => r.char)).toEqual(['SLAMPEACH']);
		// drill-down is likewise case-insensitive.
		expect(applyMyFilter([row({ char: 'slampeach' })], names, false, 'Slampeach').length).toBe(1);
	});

	it('works across the GearCheckRow/SpellCheckRow shapes (any { char } row)', () => {
		const gear = [{ char: 'Slampeach', tier: 'Velious' }, { char: 'Otherguy', tier: 'Kunark' }];
		expect(applyMyFilter(gear, names, true, null).map((r) => r.char)).toEqual(['Slampeach']);
	});
});
