// Vitest (node project) for the pure CWANT-06 group-by-character filter ($lib/wantlist/
// groupByChar) — the node-only project (no jsdom / @testing-library). This proves the
// behavioral logic WantlistPanel SHIPS (it imports groupByChar rather than inlining the
// filter), so `npm test` covers the all / by-char / no-match / account-level / non-mutation
// cases even though the panel's <select> RENDERING + onchange is DOM-blind here (that
// browser-smoke gap stays flagged for /gsd-ui-review). Mirrors myview.test.ts +
// priority.test.ts.

import { describe, it, expect } from 'vitest';
import type { WantlistRow } from '$lib/api';
import { groupByChar, ACCOUNT_LEVEL } from './groupByChar';

function want(over: Partial<WantlistRow> = {}): WantlistRow {
	return {
		id: 1,
		item_id: 100,
		item_name: 'Cloak of Flames',
		reason: 'buy',
		priority: 'med',
		note: null,
		created_at: 0,
		muted: false,
		character_id: null,
		character_name: null,
		...over
	};
}

describe('groupByChar — filter own wants by the chosen character', () => {
	const rows = [
		want({ id: 1, character_id: 10, character_name: 'Slampeach' }),
		want({ id: 2, character_id: 20, character_name: 'Findom' }),
		want({ id: 3, character_id: null, character_name: null }), // account-level
		want({ id: 4, character_id: 10, character_name: 'Slampeach' })
	];

	it('selected=null → returns ALL rows UNCHANGED (additive default, same reference)', () => {
		const out = groupByChar(rows, null);
		expect(out).toBe(rows); // same reference — no copy when passing through
		expect(out.map((r) => r.id)).toEqual([1, 2, 3, 4]);
	});

	it('selected=charId → only rows whose character_id === that id', () => {
		expect(groupByChar(rows, 10).map((r) => r.id)).toEqual([1, 4]);
		expect(groupByChar(rows, 20).map((r) => r.id)).toEqual([2]);
	});

	it('a charId that matches no rows → []', () => {
		expect(groupByChar(rows, 999)).toEqual([]);
	});

	it('ACCOUNT_LEVEL sentinel → only the untagged (character_id === null) wants', () => {
		const out = groupByChar(rows, ACCOUNT_LEVEL);
		expect(out.map((r) => r.id)).toEqual([3]);
		// it must NOT match a tagged row, and the sentinel is distinct from `null` (=all).
		expect(out.every((r) => r.character_id === null)).toBe(true);
	});

	it('does NOT mutate its input array (purity)', () => {
		const before = rows.map((r) => r.id);
		groupByChar(rows, 10);
		groupByChar(rows, ACCOUNT_LEVEL);
		groupByChar(rows, null);
		expect(rows.map((r) => r.id)).toEqual(before);
		expect(rows.length).toBe(4);
	});
});
