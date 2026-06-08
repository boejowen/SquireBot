// Pure, DOM-free decision helpers for the Phase 26 character-assignment panels
// (MyCharactersPanel — member; AssignmentAdminPanel — officer). Extracted to a plain
// .ts (NOT a .svelte module export) so they're unit-testable under the repo's node
// vitest project: vite.config.ts runs the `server` project with environment:node,
// includes only `*.{test,spec}.ts`, and EXCLUDES `*.svelte.{test,spec}.ts` — so a
// node test cannot import a .svelte file. The panels import these and the test in
// __tests__/assignments.test.ts asserts them, so `npm test` covers the panels'
// behavioral logic (the WatcherCodesPanel formatLastSeen pure-helper pattern, but
// actually wired to a test). The panels' RENDERING/interaction remains a browser-smoke
// gap flagged for /gsd-ui-review (node vitest is DOM-blind — no jsdom).
//
// The SERVER is the authoritative gate for every assignment action (26-02:
// RequireSession / RequireOfficer + the in-tx IsOfficerTx re-check); these helpers are
// pure presentation/decision logic, never a security boundary.

import type { ClaimableCharacter } from './api';

/**
 * The two halves of the claimable list the MyCharactersPanel renders differently:
 *   - `unassigned`: no current assignee → an instant Claim (D-06, open self-claim).
 *   - `assignedToOthers`: held by another member → a Request an officer approves
 *     (D-07, the contested-claim queue).
 * The backend's /assignments/claimable returns ONLY unassigned rows today, so
 * `assignedToOthers` is empty in practice — but partitioning here (rather than
 * inlining "always Claim" in the panel) keeps the Claim-vs-Request decision in one
 * tested place and is forward-compatible if the read ever widens to include contested
 * characters. A row whose `assignee` is null/undefined/'' is treated as unassigned.
 */
export function partitionClaimable(claimable: ClaimableCharacter[]): {
	unassigned: ClaimableCharacter[];
	assignedToOthers: ClaimableCharacter[];
} {
	const unassigned: ClaimableCharacter[] = [];
	const assignedToOthers: ClaimableCharacter[] = [];
	for (const c of claimable) {
		// A non-empty assignee string ⇒ contested (Request); otherwise instant-claimable.
		if (c.assignee !== null && c.assignee !== undefined && c.assignee !== '') {
			assignedToOthers.push(c);
		} else {
			unassigned.push(c);
		}
	}
	return { unassigned, assignedToOthers };
}

/**
 * The human label the officer request-queue rows render for a request's status. The
 * backend pending queue (store.PendingRequest) lists pending rows ONLY — there is no
 * status field on the wire — so the panel passes the literal 'pending' for every queue
 * row; the mapping is kept here (rather than inlined) so it is node-tested and so a
 * future approved/denied surfacing renders the right word with zero panel changes. An
 * unknown status round-trips unchanged (degrade gracefully, never render '').
 */
export function requestStatusLabel(status: string): string {
	switch (status) {
		case 'pending':
			return 'Pending';
		case 'approved':
			return 'Approved';
		case 'denied':
			return 'Denied';
		case 'cancelled':
			return 'Cancelled';
		default:
			return status;
	}
}
