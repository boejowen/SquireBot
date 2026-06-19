// Pure, DOM-free wishlist helpers for the Phase 34 per-character per-slot
// Wishlist tab — the WISH-01 banks/bots-EXCLUDED viewer-first character list +
// the WISH-07 cross-wishlist item search. Extracted to a plain .ts (NOT a
// .svelte module export) so they're unit-testable under the repo's node vitest
// project: vite.config.ts runs the `server` project with environment:node and
// EXCLUDES `*.svelte.{test,spec}.ts` — a node test cannot import a .svelte file.
// wishlist/+page.svelte imports these and __tests__/wishlist.test.ts asserts
// them, so `npm test` covers the filter/grouping logic even though the list/
// search/accordion DOM render is DOM-blind here (a browser-smoke gap closed in
// 34-04's deploy).
//
// `is_mine` is SERVER-STAMPED on the roster row (the viewer's character_assignment
// — RosterFor, P31); the client NEVER recomputes assignment here. This is
// presentation ONLY — never access control (the read API serves the same roster +
// the same per-character wishlists to every session; the WISH-07 browse leg is
// readable by design — mirrors the roster.ts / items.ts T-27-01 negative property).

import type { RosterCharacter } from '../api';

/** The two WISH-01 display bands: the viewer's own characters, then other guild
 *  characters. WISH-01 EXCLUDES banks/bots from the list entirely (callers
 *  pre-filter via wishlistRoster), so there is no "banks" band here — a bank toon
 *  has no worn paperdoll to upgrade. */
export type WishlistBand = 'mine' | 'guild';

const BAND_ORDER: Record<WishlistBand, number> = { mine: 0, guild: 1 };

/** Classify a (PRE-filtered, non-bank/bot) roster row into its WISH-01 band.
 *  Callers must drop banks/bots first (wishlistRoster does); a bank toon never
 *  reaches here. `is_mine` selects the viewer's own band; everyone else is guild. */
export function wishlistBandOf(c: RosterCharacter): WishlistBand {
	return c.is_mine ? 'mine' : 'guild';
}

/** Predicate: a row that belongs in the wishlist character list (WISH-01). EXCLUDE
 *  every bank toon / guild bot OUTRIGHT — even a viewer-owned bank toon — because a
 *  bank toon has no worn paperdoll to wishlist-upgrade (UI-SPEC §B). */
function isWishlistChar(c: RosterCharacter): boolean {
	return !c.is_bank_toon && !c.is_guild_bot;
}

/** WISH-01: the banks/bots-EXCLUDED viewer-first roster — drop every
 *  is_bank_toon || is_guild_bot row (even if is_mine), then sort mine → guild,
 *  A-Z (case-insensitive) within each band. Returns a NEW array (never mutates). */
export function wishlistRoster(rows: RosterCharacter[]): RosterCharacter[] {
	return rows.filter(isWishlistChar).sort(
		(a, b) =>
			BAND_ORDER[wishlistBandOf(a)] - BAND_ORDER[wishlistBandOf(b)] ||
			a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
	);
}

/** WISH-01 name filter that PRESERVES the banks/bots-excluded viewer-first order:
 *  matches stay ranked mine → guild, A-Z within each. An empty/whitespace query
 *  returns the full (banks/bots-excluded) set; a no-match query returns []. A
 *  bank/bot never surfaces even if its name matches the query. */
export function filterWishlistRoster(
	rows: RosterCharacter[],
	query: string
): RosterCharacter[] {
	const q = query.trim().toLowerCase();
	const base = wishlistRoster(rows); // banks/bots already excluded + viewer-first
	return q === '' ? base : base.filter((c) => c.name.toLowerCase().includes(q));
}

// ── WISH-07 cross-wishlist item search ──────────────────────────────────────
// The minimal structural shape the search reads off a WishlistView. It ignores
// equipped/suggestions/price/… — only (char, slot, target item_name) drive the
// grouping. The caller passes the FULL corpus (EVERY non-bank/bot character's
// wishlist — lazily fetched + cached by +page.svelte); this helper does NOT scope
// or fetch. There is NO "scope to the loaded/selected view" escape hatch.

/** One slot's targets, item-name only (the search ignores price/ping/etc.). */
export interface WishlistSearchSlot {
	slot: string;
	targets: { item_name: string }[];
}

/** One character's wishlist, item-name only (a structural subset of WishlistView). */
export interface WishlistSearchSource {
	char: string;
	slots: WishlistSearchSlot[];
}

/** Where a matched item is wishlisted: a (char, slot) holding. */
export interface WishlistItemHolding {
	char: string;
	slot: string;
}

/** One WISH-07 search result: an item name + every (char, slot) that wishlists it. */
export interface WishlistItemResult {
	item_name: string;
	where: WishlistItemHolding[];
}

/** WISH-07: group every target matching `query` across the WHOLE passed-in corpus
 *  (every non-bank/bot character's wishlist) by item name, listing each (char, slot)
 *  that wishlists it. The match is a case-insensitive substring on the item name;
 *  grouping is case-insensitive (the first-seen casing is the display name). The
 *  corpus order is preserved (the caller passes viewer-first wishlists), so a result's
 *  `where` reads in corpus order. Empty/whitespace query → []; no match → []; empty
 *  corpus → []. Pure + immutable (never mutates the input). */
export function searchWishlistItems(
	wishlists: WishlistSearchSource[],
	query: string
): WishlistItemResult[] {
	const q = query.trim().toLowerCase();
	if (q === '') return [];
	// Group by the normalized (lowercased) item name; keep insertion order so the
	// results read in corpus order (viewer-first). The first-seen casing displays.
	const groups = new Map<string, WishlistItemResult>();
	for (const w of wishlists) {
		for (const s of w.slots) {
			for (const t of s.targets) {
				if (!t.item_name.toLowerCase().includes(q)) continue;
				const key = t.item_name.toLowerCase();
				let g = groups.get(key);
				if (g === undefined) {
					g = { item_name: t.item_name, where: [] };
					groups.set(key, g);
				}
				g.where.push({ char: w.char, slot: s.slot });
			}
		}
	}
	return [...groups.values()];
}
