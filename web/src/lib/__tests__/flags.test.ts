// Vitest for the pure flag-priority resolver ($lib/flags) — the node-only project
// (no jsdom / @testing-library). These prove the D-01 priority order No-Drop > Lore
// > Magic that the PaperdollSlot tile ring (ITEMUI-01) + the examine flag chip both
// derive from; the .svelte ::before render is DOM-blind here (browser-smoke gap).
// Mirrors the items.test.ts table-test idiom (the S-4 pure-helper precedent).

import { describe, it, expect } from 'vitest';
import { flagColorVar, flagChipLabel, type FlagFlags } from '../flags';

describe('flagColorVar — priority No-Drop > Lore > Magic (D-01)', () => {
	it('No-Drop wins over Lore + Magic', () => {
		const f: FlagFlags = { is_no_drop: true, is_lore: true, is_magic: true };
		expect(flagColorVar(f)).toBe('var(--flag-nodrop)');
	});

	it('Lore wins over Magic when no No-Drop', () => {
		expect(flagColorVar({ is_no_drop: false, is_lore: true, is_magic: true })).toBe('var(--flag-lore)');
	});

	it('Magic when only Magic', () => {
		expect(flagColorVar({ is_magic: true })).toBe('var(--flag-magic)');
	});

	it("'' when no flag is set", () => {
		expect(flagColorVar({ is_no_drop: false, is_lore: false, is_magic: false })).toBe('');
		expect(flagColorVar({})).toBe('');
	});

	it("'' for null / undefined", () => {
		expect(flagColorVar(null)).toBe('');
		expect(flagColorVar(undefined)).toBe('');
	});
});

describe('flagChipLabel — same priority, uppercase label', () => {
	it('No-Drop wins → NO-DROP', () => {
		expect(flagChipLabel({ is_no_drop: true, is_lore: true, is_magic: true })).toBe('NO-DROP');
	});

	it('Lore wins over Magic → LORE', () => {
		expect(flagChipLabel({ is_lore: true, is_magic: true })).toBe('LORE');
	});

	it('Magic only → MAGIC', () => {
		expect(flagChipLabel({ is_magic: true })).toBe('MAGIC');
	});

	it("'' when none / null / undefined", () => {
		expect(flagChipLabel({})).toBe('');
		expect(flagChipLabel(null)).toBe('');
		expect(flagChipLabel(undefined)).toBe('');
	});
});
