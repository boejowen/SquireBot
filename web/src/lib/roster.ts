// Pure, DOM-free roster helpers for the Phase 31 Characters tab — the D-10
// viewer-first ordering (CHAR-01) + viewer-priority scoped search (CHAR-02).
// Extracted to a plain .ts (NOT a .svelte module export) so they're unit-testable
// under the repo's node vitest project: vite.config.ts runs the `server` project
// with environment:node and EXCLUDES `*.svelte.{test,spec}.ts` — a node test
// cannot import a .svelte file. +page.svelte imports these and __tests__/roster.test.ts
// asserts them, so `npm test` covers the sort/filter logic even though the list/
// search DOM render is DOM-blind here (a browser-smoke gap closed in 31-04's deploy).
//
// The SERVER already returns viewer-first order (Plan 31-02 RosterFor sorts the
// bands in Go), but these helpers (a) power the client SEARCH ranking and (b) apply
// a defensive re-sort so the D-10 banding contract is observable to a node test and
// holds regardless of server order. This is presentation only — never access control
// (the read API serves the same all-members roster to every session; mirrors the
// myview.ts T-27-01 negative property).

import type { RosterCharacter } from './api';

/** The three D-10 display bands: the viewer's own characters, then other guild
 *  characters, then guild banks/bots. */
export type Band = 'mine' | 'guild' | 'banks';

/** Classify a roster row into its D-10 band. `is_mine` WINS the tie-break even when
 *  the character is also a bank toon/bot — matching the Go RosterFor banding (a
 *  viewer's own bank toon ranks in "mine"). */
export function bandOf(c: RosterCharacter): Band {
	if (c.is_mine) return 'mine';
	if (c.is_bank_toon || c.is_guild_bot) return 'banks';
	return 'guild';
}

const BAND_ORDER: Record<Band, number> = { mine: 0, guild: 1, banks: 2 };

/** Stable viewer-first ordering: mine → guild → banks, A-Z (case-insensitive)
 *  within each band. Returns a NEW array (never mutates the input). */
export function viewerFirst(rows: RosterCharacter[]): RosterCharacter[] {
	return [...rows].sort(
		(a, b) =>
			BAND_ORDER[bandOf(a)] - BAND_ORDER[bandOf(b)] ||
			a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
	);
}

/** Case-insensitive name filter that PRESERVES viewer-first order (CHAR-02 / D-10):
 *  matches stay ranked mine → guild → banks, A-Z within each. An empty/whitespace
 *  query returns the full set (viewer-first); a no-match query returns []. */
export function filterRoster(rows: RosterCharacter[], query: string): RosterCharacter[] {
	const q = query.trim().toLowerCase();
	const matched = q === '' ? rows : rows.filter((c) => c.name.toLowerCase().includes(q));
	return viewerFirst(matched);
}
