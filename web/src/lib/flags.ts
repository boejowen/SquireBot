// Pure, DOM-free flag-priority resolver for Phase 40 (ITEMUI-01) — the locked
// D-01 priority order No-Drop > Lore > Magic, shared by PaperdollSlot.svelte
// (the ::before tile ring) and examine.ts (the examine flag chip). Extracted to a
// plain .ts (NOT a .svelte module export) so the priority LOGIC is unit-testable
// under the repo's node vitest project (vite.config.ts runs the `server` project
// with environment:node and EXCLUDES *.svelte.{test,spec}.ts — a node test cannot
// import a .svelte file). The .svelte components import these and __tests__/flags.test.ts
// asserts them, so `npm test` covers the priority resolution even though the tile
// ::before render is DOM-blind here (a browser-smoke gap — the web-tests-node-only
// discipline). Mirrors the items.ts facetItems pure-helper precedent (S-4).
//
// The component sets `--flag-color` to ONLY the literal var string this returns —
// never an item-derived or user string (T-40-06: no untrusted value reaches a
// style= sink). The three flags driving the choice are server-derived booleans.

/** The minimal flag shape both InventorySlot and ItemRollup satisfy. Accepts a
 *  partial so any payload (or a synthetic slot) can be resolved. */
export interface FlagFlags {
	is_no_drop?: boolean;
	is_lore?: boolean;
	is_magic?: boolean;
}

/** The priority CSS var string for the flag ring/chip color, '' when no flag (D-01,
 *  No-Drop > Lore > Magic). Returns one of three FIXED literal var names — never a
 *  user/item string. */
export function flagColorVar(f: FlagFlags | null | undefined): string {
	if (!f) return '';
	if (f.is_no_drop) return 'var(--flag-nodrop)';
	if (f.is_lore) return 'var(--flag-lore)';
	if (f.is_magic) return 'var(--flag-magic)';
	return '';
}

/** The priority chip label NO-DROP/LORE/MAGIC, '' when no flag (D-01, same order). */
export function flagChipLabel(f: FlagFlags | null | undefined): string {
	if (!f) return '';
	if (f.is_no_drop) return 'NO-DROP';
	if (f.is_lore) return 'LORE';
	if (f.is_magic) return 'MAGIC';
	return '';
}
