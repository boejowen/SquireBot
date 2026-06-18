// Vitest for the pure examine helper ($lib/examine) — the node-only project (no
// jsdom / @testing-library). These prove the D-08 examine field ORDER + the D-09
// field OMISSION the in-game inventory window SHIPS (ExaminePanel.svelte imports
// examineFields rather than inlining the order/omission). The panel's DOM render
// is DOM-blind here (browser-smoke gap closed in 31-04's deploy). Mirrors the
// roster.test.ts / myview.test.ts factory-fixture + describe/it idiom.

import { describe, it, expect } from 'vitest';
import type { InventorySlot } from '../api';
import { examineFields, type ExamineField } from '../examine';

function slot(over: Partial<InventorySlot> = {}): InventorySlot {
	return {
		location: 'Primary',
		category: 'equipment',
		canonical_slot: 'Primary',
		item: 'Cloak of Flames',
		id: 12345,
		count: 1,
		slots: 0,
		price: 4200,
		last_listed: '2026-05-01T00:00:00Z',
		wiki_url: 'https://wiki.project1999.com/Cloak_of_Flames',
		wiki_summary: 'STR +10 · HP +90 · AC 13',
		is_quest_item: false,
		prices: [],
		children: [],
		icon_id: 658,
		...over
	};
}

/** The ordered `kind` sequence — the load-bearing D-08 assertion target. */
function kinds(fields: ExamineField[]): string[] {
	return fields.map((f) => f.kind);
}

describe('examineFields — name is ALWAYS first and present', () => {
	it('a fully-populated slot leads with the name', () => {
		const fields = examineFields(slot(), '2026-06-18T00:00:00Z');
		expect(fields[0].kind).toBe('name');
		expect(fields[0].text).toBe('Cloak of Flames');
	});

	it('a BARE slot (no flags/stats/price/wiki/last-synced) still renders the name', () => {
		const bare = slot({
			canonical_slot: '',
			wiki_summary: '',
			wiki_url: '',
			price: null,
			is_quest_item: false,
			item: 'Mystery Item'
		});
		const fields = examineFields(bare, '');
		// Only the name survives — and a wiki link derived from the name (always
		// derivable from a non-blank item name); nothing else.
		expect(fields[0].kind).toBe('name');
		expect(fields[0].text).toBe('Mystery Item');
		expect(kinds(fields)).toEqual(['name', 'wiki']);
	});

	it('a truly nameless+sourceless slot still renders exactly one name field', () => {
		const empty = slot({
			item: '',
			canonical_slot: '',
			wiki_summary: '',
			wiki_url: '',
			price: null
		});
		const fields = examineFields(empty, '');
		// Blank item → no derivable wiki link either; only the (empty-text) name row.
		expect(kinds(fields)).toEqual(['name']);
	});
});

describe('examineFields — D-09 omission (no blank/null rows)', () => {
	it('price === null OMITS the price field entirely (no "PigParse: null")', () => {
		const fields = examineFields(slot({ price: null }), '2026-06-18T00:00:00Z');
		expect(kinds(fields)).not.toContain('price');
		// And nothing renders the literal "null".
		expect(fields.some((f) => /null/i.test(f.text))).toBe(false);
	});

	it('a present price renders a formatted pp line', () => {
		const fields = examineFields(slot({ price: 4200 }), '');
		const price = fields.find((f) => f.kind === 'price');
		expect(price?.text).toBe('PigParse: 4,200pp');
	});

	it('charLastSeen === "" OMITS the last-synced field', () => {
		const fields = examineFields(slot(), '');
		expect(kinds(fields)).not.toContain('lastsynced');
	});

	it('a blank wiki_summary OMITS the stats field', () => {
		const fields = examineFields(slot({ wiki_summary: '' }), '');
		expect(kinds(fields)).not.toContain('stats');
	});

	it('a blank canonical_slot OMITS the slot field', () => {
		const fields = examineFields(slot({ canonical_slot: '' }), '');
		expect(kinds(fields)).not.toContain('slot');
	});

	it('is_quest_item=false OMITS the flags field; true includes it', () => {
		expect(kinds(examineFields(slot({ is_quest_item: false }), ''))).not.toContain('flags');
		expect(kinds(examineFields(slot({ is_quest_item: true }), ''))).toContain('flags');
	});
});

describe('examineFields — D-08 relative ORDER of present fields', () => {
	it('a fully-populated slot orders name → flags → slot → stats → price → wiki → lastsynced', () => {
		const fields = examineFields(
			slot({ is_quest_item: true }),
			'2026-06-18T00:00:00Z'
		);
		expect(kinds(fields)).toEqual([
			'name',
			'flags',
			'slot',
			'stats',
			'price',
			'wiki',
			'lastsynced'
		]);
	});

	it('omitted fields collapse without disturbing the relative order of the rest', () => {
		// No flags, no price → name → slot → stats → wiki → lastsynced (still in D-08 order).
		const fields = examineFields(
			slot({ is_quest_item: false, price: null }),
			'2026-06-18T00:00:00Z'
		);
		expect(kinds(fields)).toEqual(['name', 'slot', 'stats', 'wiki', 'lastsynced']);
	});
});

describe('examineFields — last-synced uses charLastSeen, NOT slot.last_listed (Pitfall 2)', () => {
	it('renders the passed charLastSeen and ignores the per-slot last_listed', () => {
		const charLastSeen = '2026-06-18T12:00:00Z';
		const fields = examineFields(
			slot({ last_listed: '2001-01-01T00:00:00Z' }),
			charLastSeen
		);
		const ls = fields.find((f) => f.kind === 'lastsynced');
		expect(ls?.text).toBe(`Last synced: ${charLastSeen}`);
		// The price last-listed date must NOT appear anywhere in the examine.
		expect(fields.some((f) => f.text.includes('2001-01-01'))).toBe(false);
	});
});

describe('examineFields — wiki link', () => {
	it('uses the stored wiki_url when present', () => {
		const fields = examineFields(slot({ wiki_url: 'https://wiki.project1999.com/Cloak_of_Flames' }), '');
		const wiki = fields.find((f) => f.kind === 'wiki');
		expect(wiki?.href).toBe('https://wiki.project1999.com/Cloak_of_Flames');
	});

	it('derives the page URL from the item name when wiki_url is blank', () => {
		const fields = examineFields(slot({ wiki_url: '', item: 'Ring of the Ancients' }), '');
		const wiki = fields.find((f) => f.kind === 'wiki');
		expect(wiki?.href).toBe('https://wiki.project1999.com/Ring_of_the_Ancients');
	});
});
