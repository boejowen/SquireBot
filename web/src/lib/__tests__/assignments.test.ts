// Vitest for the pure character-assignment panel helpers ($lib/assignments) — the
// node-only project (no jsdom / @testing-library). These prove the behavioral logic
// the MyCharactersPanel + AssignmentAdminPanel SHIP (the panels import the helpers
// rather than inlining the decisions), so `npm test` covers the Claim-vs-Request
// partition + the request-status label even though the panels' RENDERING is DOM-blind
// here (that browser-smoke gap stays flagged for /gsd-ui-review). Mirrors the
// WatcherCodesPanel formatLastSeen pure-helper pattern, but actually wired to a test.

import { describe, it, expect } from 'vitest';
import type { ClaimableCharacter } from '../api';
import { partitionClaimable, requestStatusLabel } from '../assignments';

function claimable(over: Partial<ClaimableCharacter> = {}): ClaimableCharacter {
	return { character_id: 1, name: 'Slampeach', ...over };
}

describe('partitionClaimable — Claim (unassigned) vs Request (assigned to others)', () => {
	it('puts no-assignee rows in unassigned and held rows in assignedToOthers', () => {
		const list: ClaimableCharacter[] = [
			claimable({ character_id: 10, name: 'Aaa' }), // unassigned (no field)
			claimable({ character_id: 11, name: 'Bbb', assignee: null }), // unassigned (null)
			claimable({ character_id: 12, name: 'Ccc', assignee: '' }), // unassigned (empty)
			claimable({ character_id: 13, name: 'Ddd', assignee: 'discord-99' }) // contested
		];
		const { unassigned, assignedToOthers } = partitionClaimable(list);
		expect(unassigned.map((c) => c.character_id)).toEqual([10, 11, 12]);
		expect(assignedToOthers.map((c) => c.character_id)).toEqual([13]);
	});

	it('an all-unassigned list (the backend default today) yields no contested rows', () => {
		const list: ClaimableCharacter[] = [
			claimable({ character_id: 1, name: 'One' }),
			claimable({ character_id: 2, name: 'Two' })
		];
		const { unassigned, assignedToOthers } = partitionClaimable(list);
		expect(unassigned.map((c) => c.character_id)).toEqual([1, 2]);
		expect(assignedToOthers).toEqual([]);
	});

	it('an empty list partitions to two empty arrays', () => {
		expect(partitionClaimable([])).toEqual({ unassigned: [], assignedToOthers: [] });
	});
});

describe('requestStatusLabel — the human label the officer request-queue rows render', () => {
	it('maps the known statuses to their capitalized label', () => {
		expect(requestStatusLabel('pending')).toBe('Pending');
		expect(requestStatusLabel('approved')).toBe('Approved');
		expect(requestStatusLabel('denied')).toBe('Denied');
		expect(requestStatusLabel('cancelled')).toBe('Cancelled');
	});
	it('round-trips an unknown status unchanged (degrade gracefully, never render "")', () => {
		expect(requestStatusLabel('weird')).toBe('weird');
		expect(requestStatusLabel('')).toBe('');
	});
});
