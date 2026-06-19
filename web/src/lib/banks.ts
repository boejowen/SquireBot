// Pure, DOM-free bank helpers for the Phase 33 Banks tab — the D-01 A-Z bank
// ordering (BANK-01) + the D-03 is_bank-scoped item search (BANK-03) + the D-04
// per-bank lookup. Extracted to a plain .ts (NOT a .svelte module export) so they're
// unit-testable under the repo's node vitest project: vite.config.ts runs the `server`
// project with environment:node and EXCLUDES `*.svelte.{test,spec}.ts` — a node test
// cannot import a .svelte file. banks/+page.svelte imports these and
// __tests__/banks.test.ts asserts them, so `npm test` covers the sort/search logic
// even though the list/search/detail DOM render is DOM-blind here (a browser-smoke gap
// closed in 33-03's deploy).
//
// `is_bank` is SERVER-STAMPED on each holder (compute.Items sets it = IsBankToon ||
// IsGuildBot — Plan 32-01); the client NEVER recomputes designation here. This is
// presentation ONLY — never access control (the read API serves the same guild-wide
// rollup to every session; mirrors the items.ts / roster.ts negative property). All
// functions return NEW arrays/objects (immutable) — they never mutate the input.

import type { BankRowSummary, ItemRollup, ItemHolder } from './api';

/** Stable plain A-Z ordering of the bank/bot rows (D-01). NOT viewer-first — banks
 *  aren't anyone's assigned characters, so the Characters-tab is_mine banding doesn't
 *  apply; this is the roadmap's "same ordering style as Characters" reduced to its
 *  banks-only case. Returns a NEW array (never mutates the input). */
export function sortBanksAZ(banks: BankRowSummary[]): BankRowSummary[] {
	return [...banks].sort((a, b) =>
		a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
	);
}

/** The ONE new web algorithm (BANK-03 / D-03): scope the P32 item rollup to the guild
 *  banks. For each item it (1) keeps ONLY the holders where is_bank === true; (2) drops
 *  any item left with zero bank holders; (3) RECOMPUTES the displayed summed_qty (Σ of
 *  the kept-holder qty) and holder_count (distinct kept chars) from the bank slice — it
 *  does NOT pass through ItemRollup.summed_qty / holder_count, which are GUILD-WIDE and
 *  would leak personal holdings (Pitfall 3, the "Blue Diamond 40× guild / 3× bank" trap);
 *  (4) name-filters by trim/case-insensitive includes; (5) returns A-Z. An empty/whitespace
 *  query keeps every bank-holding item (A-Z); a no-match query returns []. Returns NEW
 *  objects (the input rows + their holders arrays are never mutated). */
export function bankItemSearch(rows: ItemRollup[], query: string): ItemRollup[] {
	const q = query.trim().toLowerCase();
	const out: ItemRollup[] = [];
	for (const r of rows) {
		const bankHolders = r.holders.filter((h) => h.is_bank);
		if (bankHolders.length === 0) continue; // drop zero-bank-holder items
		if (q !== '' && !r.name.toLowerCase().includes(q)) continue; // name-filter
		// Bank-slice recompute — NEVER the guild-wide r.summed_qty / r.holder_count (Pitfall 3).
		const summed = bankHolders.reduce((s, h) => s + h.qty, 0);
		const holderCount = new Set(bankHolders.map((h) => h.char)).size;
		out.push({ ...r, holders: bankHolders, summed_qty: summed, holder_count: holderCount });
	}
	return out.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
}

/** Order the per-item bank holders A-Z (case-insensitive) for the search results.
 *  Unlike items.ts's sortHolders, there is no viewer/guild/bank band here — every
 *  holder in a bankItemSearch result is a bank holder, so the band collapses to a
 *  plain A-Z by char. Returns a NEW array (never mutates the input). */
export function sortBankHolders(holders: ItemHolder[]): ItemHolder[] {
	return [...holders].sort((a, b) =>
		a.char.localeCompare(b.char, undefined, { sensitivity: 'base' })
	);
}

/** Look up one bank's summary row by exact name — so the D-04 per-bank detail header
 *  reads its value/plat off the already-loaded BanksView.banks with NO second fetch.
 *  Returns the row or undefined when the name isn't in the list. */
export function bankByName(
	banks: BankRowSummary[],
	name: string
): BankRowSummary | undefined {
	return banks.find((b) => b.name === name);
}
