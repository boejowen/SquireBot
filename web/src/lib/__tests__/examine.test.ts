// Vitest for the pure examine helper ($lib/examine) — the node-only project (no
// jsdom / @testing-library). These prove the examine field ORDER + the D-09 field
// OMISSION the in-game inventory window SHIPS (ExaminePanel.svelte imports
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
		// statsblock = the in-game stat block (buffs); wiki_summary = the lore/notes.
		statsblock: 'MAGIC ITEM\nSlot: BACK\nAC: 13\nSTR: +10 HP: +90\nWT: 0.5 Size: SMALL',
		wiki_summary: 'A cloak wreathed in everburning flame.',
		is_quest_item: false,
		prices: [],
		children: [],
		icon_id: 658,
		is_no_drop: false,
		is_lore: false,
		is_magic: false,
		quest_links: [],
		...over
	};
}

/** The ordered `kind` sequence — the load-bearing order assertion target. */
function kinds(fields: ExamineField[]): string[] {
	return fields.map((f) => f.kind);
}

describe('examineFields — name is ALWAYS first and present', () => {
	it('a fully-populated slot leads with the name', () => {
		const fields = examineFields(slot(), '2026-06-18T00:00:00Z');
		expect(fields[0].kind).toBe('name');
		expect(fields[0].text).toBe('Cloak of Flames');
	});

	it('a BARE slot (no flags/stats/notes/price/wiki/last-synced) still renders the name', () => {
		const bare = slot({
			statsblock: '',
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
			statsblock: '',
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

	it('a blank statsblock OMITS the stats field', () => {
		const fields = examineFields(slot({ statsblock: '' }), '');
		expect(kinds(fields)).not.toContain('stats');
	});

	it('a present statsblock renders the stat block verbatim', () => {
		const fields = examineFields(slot({ statsblock: 'Slot: BACK\nAC: 13' }), '');
		const stats = fields.find((f) => f.kind === 'stats');
		expect(stats?.text).toBe('Slot: BACK\nAC: 13');
	});

	it('a blank wiki_summary OMITS the notes field', () => {
		const fields = examineFields(slot({ wiki_summary: '' }), '');
		expect(kinds(fields)).not.toContain('notes');
	});

	it('is_quest_item=false OMITS the flags field; true includes it', () => {
		expect(kinds(examineFields(slot({ is_quest_item: false }), ''))).not.toContain('flags');
		expect(kinds(examineFields(slot({ is_quest_item: true }), ''))).toContain('flags');
	});

	it('never emits a standalone slot row (the stat block carries the slot)', () => {
		// The old D-08 "Slot:" row is folded into the stat block — no separate slot field.
		expect(kinds(examineFields(slot(), ''))).not.toContain('slot');
	});
});

describe('examineFields — relative ORDER of present fields', () => {
	it('a fully-populated slot orders name → flags → stats → notes → price → wiki → lastsynced', () => {
		const fields = examineFields(slot({ is_quest_item: true }), '2026-06-18T00:00:00Z');
		expect(kinds(fields)).toEqual([
			'name',
			'flags',
			'stats',
			'notes',
			'price',
			'wiki',
			'lastsynced'
		]);
	});

	it('omitted fields collapse without disturbing the relative order of the rest', () => {
		// No flags, no price → name → stats → notes → wiki → lastsynced.
		const fields = examineFields(slot({ is_quest_item: false, price: null }), '2026-06-18T00:00:00Z');
		expect(kinds(fields)).toEqual(['name', 'stats', 'notes', 'wiki', 'lastsynced']);
	});
});

describe('examineFields — last-synced uses charLastSeen, NOT slot.last_listed (Pitfall 2)', () => {
	it('renders the passed charLastSeen and ignores the per-slot last_listed', () => {
		const charLastSeen = '2026-06-18T12:00:00Z';
		const fields = examineFields(slot({ last_listed: '2001-01-01T00:00:00Z' }), charLastSeen);
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

describe('examineFields — flag chip (ITEMUI-01, priority No-Drop>Lore>Magic)', () => {
	it('a No-Drop item produces a flagchip field with text NO-DROP, right after name', () => {
		const fields = examineFields(slot({ is_no_drop: true }), '');
		const chip = fields.find((f) => f.kind === 'flagchip');
		expect(chip?.text).toBe('NO-DROP');
		// position: immediately after the name field
		expect(fields[1].kind).toBe('flagchip');
	});

	it('a Lore+Magic (no No-Drop) item chips LORE (priority)', () => {
		const fields = examineFields(slot({ is_lore: true, is_magic: true }), '');
		expect(fields.find((f) => f.kind === 'flagchip')?.text).toBe('LORE');
	});

	it('a Magic-only item chips MAGIC', () => {
		const fields = examineFields(slot({ is_magic: true }), '');
		expect(fields.find((f) => f.kind === 'flagchip')?.text).toBe('MAGIC');
	});

	it('an unflagged item omits the flagchip field entirely (D-09)', () => {
		const fields = examineFields(slot({ is_no_drop: false, is_lore: false, is_magic: false }), '');
		expect(kinds(fields)).not.toContain('flagchip');
	});

	it('the flag chip is additive — the QUEST ITEM flags field still appears too', () => {
		const fields = examineFields(slot({ is_lore: true, is_quest_item: true }), '');
		expect(kinds(fields)).toContain('flagchip');
		expect(kinds(fields)).toContain('flags');
		// name → flagchip → flags order
		expect(kinds(fields).slice(0, 3)).toEqual(['name', 'flagchip', 'flags']);
	});
});

describe('examineFields — named quests (ITEMUI-02, notes_link only)', () => {
	it('a notes_link quest produces a quests field after wiki and before lastsynced', () => {
		const fields = examineFields(
			slot({
				quest_links: [
					{ quest_name: 'Coldain Ring 8', source: 'notes_link', source_url: 'https://wiki.project1999.com/Coldain_Ring_8' }
				]
			}),
			'2026-06-18T00:00:00Z'
		);
		const q = fields.find((f) => f.kind === 'quests');
		expect(q?.text).toBe('Used in:');
		expect(q?.quests).toEqual([
			{ quest_name: 'Coldain Ring 8', source_url: 'https://wiki.project1999.com/Coldain_Ring_8' }
		]);
		// position: wiki → quests → lastsynced
		const seq = kinds(fields);
		expect(seq.indexOf('quests')).toBeGreaterThan(seq.indexOf('wiki'));
		expect(seq.indexOf('quests')).toBeLessThan(seq.indexOf('lastsynced'));
	});

	it('multiple notes_link quests all carry through with their source_url', () => {
		const fields = examineFields(
			slot({
				quest_links: [
					{ quest_name: 'Quest A', source: 'notes_link', source_url: 'https://wiki.project1999.com/A' },
					{ quest_name: 'Quest B', source: 'notes_link', source_url: 'https://wiki.project1999.com/B' }
				]
			}),
			''
		);
		const q = fields.find((f) => f.kind === 'quests');
		expect(q?.quests?.map((x) => x.quest_name)).toEqual(['Quest A', 'Quest B']);
	});

	it('a quest_links list that is ALL in_game_flag omits the quests field (D-06/D-09)', () => {
		const fields = examineFields(
			slot({
				is_quest_item: true,
				quest_links: [{ quest_name: '[in-game QUEST flag]', source: 'in_game_flag', source_url: '' }]
			}),
			''
		);
		expect(kinds(fields)).not.toContain('quests');
		// the in_game_flag pseudo-name never reaches the output
		expect(fields.some((f) => /in-game QUEST flag/.test(f.text))).toBe(false);
	});

	it('an empty quest_links omits the quests field', () => {
		const fields = examineFields(slot({ quest_links: [] }), '');
		expect(kinds(fields)).not.toContain('quests');
	});
});
