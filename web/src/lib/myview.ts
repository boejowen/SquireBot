// Pure, DOM-free decision helpers for the Phase 27 "My characters" quick-filter +
// single-character drill-down over the consolidated views (inventory / bank / gear /
// spell). Extracted to a plain .ts (NOT a .svelte module export) so they're unit-
// testable under the repo's node vitest project: vite.config.ts runs the `server`
// project with environment:node, includes only `*.{test,spec}.ts`, and EXCLUDES
// `*.svelte.{test,spec}.ts` — so a node test cannot import a .svelte file. +page.svelte
// imports these and the test in __tests__/myview.test.ts asserts them, so `npm test`
// covers the filter's behavioral logic. The +page.svelte <select> RENDERING/onchange
// remains a browser-smoke gap flagged for /gsd-ui-review (node vitest is DOM-blind — no
// jsdom).
//
// LOAD-BEARING NEGATIVE SECURITY PROPERTY (T-27-01, ASVS V4): this filter is presentation,
// NOT a security boundary. The view API already returns the identical all-members rows to
// every authenticated session; the server's RequireSession is the authoritative gate.
// This helper can only narrow what THIS browser displays — it must NEVER be relied on to
// hide a row from any other member, and there is nothing to leak that the API did not
// already authorize for this session. (Do NOT add a ?mine=1 param or a per-caller server
// filter — that would convert a UX convenience into access control.)

import type { MyCharacter } from './api';

/** The set of the caller's assigned character NAMES — the join key against the
 *  `char` string every view row carries (ViewRow/GearCheckRow/SpellCheckRow source
 *  the SAME character.name column the assignment store does — readviews.go:150 ≡
 *  assignment.go:448 — so exact-match is correct; we also lower-case for a defensive
 *  case-insensitive join). */
export function myCharNameSet(mine: MyCharacter[]): Set<string> {
	return new Set(mine.map((c) => c.name.toLowerCase()));
}

/** Narrow any char-bearing rows to the caller's characters. selectedChar (the
 *  drill-down, MYVIEW-02) DOMINATES: only that char's rows survive. Otherwise
 *  mineOnly=false passes rows through UNCHANGED (additive default, MYVIEW-01) and
 *  mineOnly=true keeps only rows whose char is in mineNames. Pure + DOM-free →
 *  node-testable. Case-insensitive to match myCharNameSet. */
export function applyMyFilter<T extends { char: string }>(
	rows: T[],
	mineNames: Set<string>,
	mineOnly: boolean,
	selectedChar: string | null
): T[] {
	if (selectedChar) {
		const sel = selectedChar.toLowerCase();
		return rows.filter((r) => r.char.toLowerCase() === sel);
	}
	if (!mineOnly) return rows;
	return rows.filter((r) => mineNames.has(r.char.toLowerCase()));
}
