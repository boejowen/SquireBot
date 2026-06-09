// Pure, DOM-free decision helper for the CWANT-06 "group/filter MY wantlist by
// character" control. Extracted to a plain .ts (NOT a .svelte module export) so it's
// unit-testable under the repo's node vitest project: vite.config.ts runs the `server`
// project with environment:node, includes only `*.{test,spec}.ts`, and EXCLUDES
// `*.svelte.{test,spec}.ts` — so a node test cannot import a .svelte file. WantlistPanel
// imports this and groupByChar.test.ts asserts it, so `npm test` covers the filter's
// behavioral logic. The panel's <select> RENDERING/onchange remains a browser-smoke gap
// flagged for /gsd-ui-review (node vitest is DOM-blind — no jsdom). Mirrors myview.ts.
//
// LOAD-BEARING NEGATIVE SECURITY PROPERTY (T-28-11, ASVS V4): this filter is presentation,
// NOT a security boundary. The wantlist API already returned the identical owner-scoped
// rows to THIS session; the server's RequireSession + owner-scoping is the authoritative
// gate. This helper can only narrow what THIS browser displays — it must NEVER be relied
// on to hide a row from anyone, and there is nothing to leak that the API did not already
// authorize for this session. (Do NOT add a ?char= server param — that would convert a UX
// convenience into access control.)

import type { WantlistRow } from '$lib/api';

/**
 * The sentinel for "account-level only" (the untagged wants — character_id === null).
 * Distinct from `null`, which means "all" (no filter). A number selects that character_id.
 * It's a unique Symbol so it can never collide with a real character_id.
 */
export const ACCOUNT_LEVEL = Symbol('account-level');

/** The selection a group-by-character control can be in:
 *  - `null`          ⇒ ALL wants (no filter / "All characters")
 *  - a `number`      ⇒ only wants tagged to that character_id
 *  - `ACCOUNT_LEVEL` ⇒ only the untagged (account-level, character_id === null) wants */
export type CharSelection = number | null | typeof ACCOUNT_LEVEL;

/**
 * Narrow the caller's OWN wants to a single character selection. `null` passes rows
 * through UNCHANGED (additive default — same reference, no copy, the applyMyFilter
 * precedent). A number keeps only rows whose `character_id === selected`. The
 * `ACCOUNT_LEVEL` sentinel keeps only the untagged (`character_id === null`) wants.
 * Pure + DOM-free → node-testable; NEVER mutates its input array.
 */
export function groupByChar(rows: WantlistRow[], selected: CharSelection): WantlistRow[] {
	if (selected === null) return rows; // "all" — passthrough, same reference
	if (selected === ACCOUNT_LEVEL) return rows.filter((r) => r.character_id === null);
	return rows.filter((r) => r.character_id === selected);
}
