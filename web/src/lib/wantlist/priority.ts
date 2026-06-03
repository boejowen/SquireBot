// priority.ts — DOM-free priority ranking + note rune-count helpers for the
// wantlist (19-03 Task 1). Pure module so the node vitest project can cover the
// rank mapping and the N/280 counter math without a DOM (the formatLastSeen
// precedent). priorityRank backs the columns.ts custom sortingFn (high→low);
// noteRuneCount mirrors the server's utf8.RuneCountInString so the live "N/280"
// counter agrees with the server's 280-RUNE cap (NOT byte length — Pitfall 2).

/** Map a priority enum to a sort rank: high=3, med=2, low=1, anything-else→0. */
export function priorityRank(p: string): number {
	switch (p) {
		case 'high':
			return 3;
		case 'med':
			return 2;
		case 'low':
			return 1;
		default:
			return 0;
	}
}

/**
 * The UTF-aware rune count of a note (`[...s].length`) for the "N/280" counter.
 * Uses code-point iteration (not `.length`, which counts UTF-16 units) so it
 * matches the server's `utf8.RuneCountInString` 280-rune cap exactly.
 */
export function noteRuneCount(s: string): number {
	return [...s].length;
}
