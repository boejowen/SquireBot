// Pure officer-management decision helpers (Plan 15-05 Task 3, D-08). Extracted
// from AdminMgmtForm.svelte so the owner-floor suppression + the idempotent
// result-message logic are node-unit-testable WITHOUT a DOM (the repo's
// established philosophy — 15-04 SUMMARY). These port v1's showAdminMgmtSidebar
// rules verbatim (showRemove + the add/remove result copy). The SERVER (15-03)
// is the authoritative gate (owner_floor_protected / not_authorized re-checked
// in-tx); these are Layer-1 UX only.

import type { Officer, AddOfficerResult, RemoveOfficerResult } from './api';

/**
 * Whether the Remove button shows for an officer row (v1's `showRemove`):
 * always for a non-floor officer; for the floor row ONLY when the CALLER is the
 * floor (self-removal allowed; a peer cannot). When the caller's id is unknown
 * (empty), suppress on the floor row for safety and rely on the server's
 * owner_floor_protected 403 (defense-in-depth, D-08).
 */
export function showRemoveButton(officer: Officer, callerDiscordUserId: string): boolean {
	if (!officer.is_floor) return true;
	return callerDiscordUserId !== '' && callerDiscordUserId === officer.discord_user_id;
}

/** The add-officer result copy (15-UI-SPEC verbatim): idempotent already-officer vs added. */
export function addResultMessage(result: AddOfficerResult): string {
	return result.added
		? `Officer added: ${result.username}.`
		: `Already an officer: ${result.username}.`;
}

/** The remove-officer result copy (15-UI-SPEC verbatim): idempotent not-found vs removed. */
export function removeResultMessage(result: RemoveOfficerResult): string {
	return result.removed
		? `Officer removed: ${result.username}.`
		: `Not in the list: ${result.username}.`;
}

/**
 * The inline error copy for the two NON-collapsing admin error routes
 * (15-UI-SPEC verbatim). owner_floor_protected + lock_busy stay inline; the
 * not_authorized / bare-403 case is handled by the caller via authGuard (it
 * collapses the whole admin UI — see classifyAdminError in api.ts), so it is
 * NOT a string here.
 *
 * WR-07: the 'lock-busy' copy is defense-in-depth only — the backend never emits
 * lock_busy today (busy_timeout + maxconns=1 serialize writes; see classifyAdminError
 * in api.ts and webadmin/audit.go). It is retained so the handling is ready if a
 * future busy_timeout change ever makes SQLITE_BUSY reachable.
 */
export const ADMIN_ERROR_COPY = {
	'owner-floor': 'Owner-floor protected — only the maintainer can remove themselves. No changes were written.',
	'lock-busy': 'Another officer action is in flight. Please retry. No changes were written.'
} as const;
