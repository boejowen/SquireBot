// Pure, DOM-free examine helpers for the Phase 31 in-game inventory window —
// the D-08 examine field ORDER + the D-09 graceful field OMISSION (INV-02).
// Extracted to a plain .ts (NOT a .svelte module export) so the order/omission
// contract is unit-testable under the repo's node vitest project (vite.config.ts
// runs the `server` project with environment:node and EXCLUDES *.svelte.{test,spec}.ts
// — a node test cannot import a .svelte file). ExaminePanel.svelte imports
// examineFields and __tests__/examine.test.ts asserts the order + omission without
// a DOM (the panel's DOM render stays a browser-smoke gap, closed in 31-04's deploy).
//
// The example/order is the LOCKED D-08 sequence; a field whose source value is empty
// is OMITTED ENTIRELY (D-09) — never rendered as a blank row, "null", or "—". The
// item NAME is always present. "Last synced" is the per-CHARACTER value
// (CharacterInventory.last_seen), explicitly NOT the per-slot last_listed (the
// PRICE last-listed date — 31-RESEARCH Pitfall 2). No DOM, no fetch — pure.

import type { InventorySlot } from './api';

/** One ordered examine line. `kind` keys its visual treatment in ExaminePanel
 *  (name=accent heading, flags=status-other, stats=status-ok, price=tabular-nums,
 *  wiki=accent link with `href`, lastsynced=dimmed footer). `text` is the
 *  already-composed display string; `href` is set ONLY on the wiki line. */
export interface ExamineField {
	kind:
		| 'name'
		| 'flags'
		| 'slot'
		| 'dmgdly'
		| 'ac'
		| 'stats'
		| 'wtsize'
		| 'classrace'
		| 'price'
		| 'wiki'
		| 'lastsynced';
	text: string;
	/** Present only on the `wiki` line — the absolute http(s) page URL. */
	href?: string;
}

/** Round + en-US comma-grouping for a pp price; "0" for a non-finite value.
 *  Mirrors composeNotes.formatPp (kept private there) — the examine price line. */
function formatPp(n: number): string {
	if (!Number.isFinite(n)) return '0';
	return Math.round(n).toLocaleString('en-US');
}

/** Title-case-ish display of a canonical slot key for the "Slot:" line
 *  (e.g. "Primary" → "PRIMARY", "Finger1" → "FINGER1"). The canonical_slot is a
 *  trusted compute constant (slotconst.go), so this is presentation only. */
function slotDisplay(canonicalSlot: string): string {
	return canonicalSlot.trim().toUpperCase();
}

/**
 * Build the examine fields for one filled slot in the LOCKED D-08 order, omitting
 * any field whose source is empty (D-09 — never a blank/"null"/"—" row). The NAME
 * is ALWAYS the first field and always present (even for a bare slot). `charLastSeen`
 * is the per-CHARACTER value (CharacterInventory.last_seen) — NOT slot.last_listed.
 *
 * D-08 order (a field is OMITTED when its source is empty):
 *   1  name        (always)
 *   2  flags       (derived from is_quest_item; omit when none)
 *   3  slot/skill  (canonical_slot; omit when blank)
 *   4  DMG/DLY     (weapons only — not exposed by the current contract → always omitted)
 *   5  AC          (not exposed discretely by the current contract → always omitted)
 *   6  stats       (the stored wiki_summary block; omit when blank)
 *   7  wt/size     (not exposed discretely by the current contract → always omitted)
 *   8  class/race  (not exposed discretely by the current contract → always omitted)
 *   9  PigParse price ("PigParse: {price}pp"; omit when price === null)
 *   10 wiki link   (wiki_url || wikiUrlFor(item); omit only when both are blank)
 *   11 last synced ("Last synced: {charLastSeen}"; omit when "")
 *
 * Discrete DMG/DLY/AC/wt-size/class-race rows are NOT in the read contract
 * (InventorySlot exposes wiki_summary as a single prose block, plus price/prices/
 * wiki_url/is_quest_item). Per D-09 "show what's known, never fabricate": the prose
 * summary renders as the single `stats` block and the structured rows are omitted.
 * If the contract later exposes discrete fields, slot them in at their D-08 index.
 */
export function examineFields(slot: InventorySlot, charLastSeen: string): ExamineField[] {
	const fields: ExamineField[] = [];

	// 1. Name — ALWAYS present, always first.
	fields.push({ kind: 'name', text: slot.item });

	// 2. Flags — the only flag the contract exposes is is_quest_item (D-09: show
	//    what's known). A future flags string would slot in here.
	if (slot.is_quest_item) {
		fields.push({ kind: 'flags', text: 'QUEST ITEM' });
	}

	// 3. Slot / skill — the canonical equipment slot, when known.
	const cs = slot.canonical_slot?.trim();
	if (cs) {
		fields.push({ kind: 'slot', text: `Slot: ${slotDisplay(cs)}` });
	}

	// 4 DMG/DLY · 5 AC · 7 wt/size · 8 class/race — not discretely exposed by the
	//   read contract; omitted entirely (D-09 — never fabricate). The stored prose
	//   carries any such detail and renders as the `stats` block below.

	// 6. Stats — the stored wiki summary prose, when present.
	const summary = slot.wiki_summary?.trim();
	if (summary) {
		fields.push({ kind: 'stats', text: summary });
	}

	// 9. PigParse price — omit ENTIRELY when null (D-09; never "PigParse: null").
	if (slot.price !== null && slot.price !== undefined) {
		fields.push({ kind: 'price', text: `PigParse: ${formatPp(slot.price)}pp` });
	}

	// 10. Wiki link — the stored wiki_url, else a derived page URL from the item
	//     name; omit only when both are blank.
	const href = wikiHref(slot);
	if (href) {
		fields.push({ kind: 'wiki', text: `Wiki: ${slot.item} ↗`, href });
	}

	// 11. Last synced — the per-CHARACTER upload freshness (NOT slot.last_listed);
	//     omit when unknown.
	const ls = charLastSeen?.trim();
	if (ls) {
		fields.push({ kind: 'lastsynced', text: `Last synced: ${ls}` });
	}

	return fields;
}

/** The examine wiki page URL: the stored slot.wiki_url when set, else the derived
 *  P1999 page URL from the item name (spaces → underscores, encodeURIComponent —
 *  mirrors composeNotes.wikiUrlFor). Returns '' when neither is available. The
 *  ExaminePanel additionally runs this through safeHttpUrl before any href (the
 *  scheme allow-list is the load-bearing control — this is the source value). */
function wikiHref(slot: InventorySlot): string {
	const stored = slot.wiki_url?.trim();
	if (stored) return stored;
	const name = slot.item?.trim();
	if (!name) return '';
	return 'https://wiki.project1999.com/' + encodeURIComponent(name.replace(/ /g, '_'));
}
