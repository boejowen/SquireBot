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
import { flagChipLabel } from './flags';

/** One ordered examine line. `kind` keys its visual treatment in ExaminePanel
 *  (name=accent heading, flagchip=priority flag-color chip, flags=status-other,
 *  stats=status-ok, price=tabular-nums, wiki=accent link with `href`,
 *  quests=accent named-quest links, lastsynced=dimmed footer). `text` is the
 *  already-composed display string; `href` is set ONLY on the wiki line; `quests`
 *  is set ONLY on the quests line. */
export interface ExamineField {
	kind:
		| 'name'
		| 'flagchip' // the priority flag chip (NO-DROP/LORE/MAGIC), beside the name (ITEMUI-01)
		| 'flags'
		| 'stats' // the in-game stat block (Slot/AC/STR.../WT/class/race), from statsblock
		| 'notes' // the wiki description/lore (the former wiki_summary)
		| 'price'
		| 'wiki'
		| 'quests' // the named "Used in:" quest links (notes_link only, ITEMUI-02)
		| 'lastsynced';
	text: string;
	/** Present only on the `wiki` line — the absolute http(s) page URL. */
	href?: string;
	/** Present only on the `quests` line — the notes_link named quests (each with its
	 *  raw source_url; the render sink runs every href through safeHttpUrl, D-05). */
	quests?: { quest_name: string; source_url: string }[];
}

/** Round + en-US comma-grouping for a pp price; "0" for a non-finite value.
 *  Mirrors composeNotes.formatPp (kept private there) — the examine price line. */
function formatPp(n: number): string {
	if (!Number.isFinite(n)) return '0';
	return Math.round(n).toLocaleString('en-US');
}

/**
 * Build the examine fields for one filled slot in the LOCKED D-08 order, omitting
 * any field whose source is empty (D-09 — never a blank/"null"/"—" row). The NAME
 * is ALWAYS the first field and always present (even for a bare slot). `charLastSeen`
 * is the per-CHARACTER value (CharacterInventory.last_seen) — NOT slot.last_listed.
 *
 * Order (a field is OMITTED when its source is empty):
 *   1  name        (always)
 *   1b flagchip    (the priority flag chip NO-DROP/LORE/MAGIC; omit when no flag — ITEMUI-01)
 *   2  flags       (the is_quest_item badge; omit when not a quest item)
 *   3  stats       (the stored in-game stat block — slot, AC, stat buffs, WT/size,
 *                   class/race, MAGIC/LORE/NO-DROP flags — from statsblock; omit when blank)
 *   4  notes       (the wiki description/lore — the former wiki_summary; omit when blank)
 *   5  PigParse price ("PigParse: {price}pp"; omit when price === null)
 *   6  wiki link   (wiki_url || wikiUrlFor(item); omit only when both are blank)
 *   6b quests      (the "Used in:" notes_link named quests; omit when none — ITEMUI-02)
 *   7  last synced ("Last synced: {charLastSeen}"; omit when "")
 *
 * The D-08 discrete rows (slot/DMG/DLY/AC/wt-size/class-race) are NOT split out: the
 * wiki statsblock already presents them in the in-game order as a single block, so a
 * separate "Slot:" line is dropped (it would just duplicate the stat block's first
 * line, and for a bag it was the noisy inventory position). Per D-09, the stat block
 * is shown verbatim when present and omitted when blank.
 */
export function examineFields(slot: InventorySlot, charLastSeen: string): ExamineField[] {
	const fields: ExamineField[] = [];

	// 1. Name — ALWAYS present, always first.
	fields.push({ kind: 'name', text: slot.item });

	// 1b. Flag chip — the priority flag (No-Drop > Lore > Magic, D-01) as a small
	//     uppercase chip beside the name (ITEMUI-01). Additive: the full stat block
	//     below still lists every flag. Omit when no flag (D-09).
	const chip = flagChipLabel(slot);
	if (chip) {
		fields.push({ kind: 'flagchip', text: chip });
	}

	// 2. Flags — the is_quest_item badge (a quick top indicator). The MAGIC/LORE/NO-DROP
	//    flags live inside the stat block below.
	if (slot.is_quest_item) {
		fields.push({ kind: 'flags', text: 'QUEST ITEM' });
	}

	// 3. Stats — the in-game stat block (Slot/AC/STR.../WT/class/race), straight from the
	//    stored wiki statsblock: the item's actual buffs + requirements. Omit when blank (D-09).
	const stats = slot.statsblock?.trim();
	if (stats) {
		fields.push({ kind: 'stats', text: stats });
	}

	// 4. Notes — the item's wiki description/lore (the former summary). Omit when blank (D-09).
	const notes = slot.wiki_summary?.trim();
	if (notes) {
		fields.push({ kind: 'notes', text: notes });
	}

	// 5. PigParse price — omit ENTIRELY when null (D-09; never "PigParse: null").
	if (slot.price !== null && slot.price !== undefined) {
		fields.push({ kind: 'price', text: `PigParse: ${formatPp(slot.price)}pp` });
	}

	// 6. Wiki link — the stored wiki_url, else a derived page URL from the item name;
	//    omit only when both are blank.
	const href = wikiHref(slot);
	if (href) {
		fields.push({ kind: 'wiki', text: `Wiki: ${slot.item} ↗`, href });
	}

	// 6b. Named quests — the "Used in:" list (ITEMUI-02). notes_link ONLY (D-06; NEVER
	//     the in_game_flag '[in-game QUEST flag]' pseudo-name). The render sink runs each
	//     source_url through safeHttpUrl (D-05). Omit entirely when zero named quests (D-09).
	const named = (slot.quest_links ?? []).filter((q) => q.source === 'notes_link');
	if (named.length > 0) {
		fields.push({
			kind: 'quests',
			text: 'Used in:',
			quests: named.map((q) => ({ quest_name: q.quest_name, source_url: q.source_url }))
		});
	}

	// 7. Last synced — the per-CHARACTER upload freshness (NOT slot.last_listed); omit
	//    when unknown.
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
