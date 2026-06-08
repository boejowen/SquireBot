// Vitest for the pure char-meta validation + change-detection helpers (Phase 16
// Task 2, CUTOVER-02 / D-02/D-03). These prove the CharMetaForm's behavioral
// contract (level range validation, class/race membership, the Save gate) under
// the repo's node-only test project — the form .svelte is a thin renderer over them
// (15-04's pure-logic-in-.ts philosophy, carried forward from coin.ts).
//
// The form .svelte and the SiteShell nav are ALSO source-asserted here (Style B,
// the eviction.test.ts pattern): the node suite is DOM-blind, so the CR-01
// input-type lesson and the D-03 member-accessible nav placement are caught by
// reading the source string — not by mounting the component.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import type { CharMetaItem } from '../api';
import {
	CLASSES,
	RACES,
	LEVEL_ERROR,
	validateLevel,
	validateClass,
	validateRace,
	charMetaIsValid,
	levelPayload,
	charMetaPayload,
	levelToInput,
	inputsFromChar,
	charMetaChanged,
	type CharMetaInputs
} from '../charmeta';

function char(over: Partial<CharMetaItem> = {}): CharMetaItem {
	return {
		character_id: 1,
		name: 'Slampeach',
		class: '',
		level: null,
		race: '',
		is_bank_toon: false,
		...over
	};
}

function inputs(over: Partial<CharMetaInputs> = {}): CharMetaInputs {
	return { class: '', race: '', level: '', isBankToon: false, ...over };
}

describe('CLASSES / RACES — mirror the Go enrich value sets (14 each)', () => {
	it('has the 14 class abbreviations including WAR', () => {
		expect(CLASSES).toHaveLength(14);
		expect(CLASSES).toContain('WAR');
		expect(CLASSES).toContain('ENC');
	});
	it('has the 14 race abbreviations including the load-bearing IKS', () => {
		expect(RACES).toHaveLength(14);
		expect(RACES).toContain('IKS');
		expect(RACES).toContain('VAH');
	});
});

describe('validateLevel — blank or 1..60', () => {
	it('blank is valid (unset — a member may not know the level yet)', () => {
		expect(validateLevel('')).toBeUndefined();
	});
	it('1 and 60 are valid (the boundaries)', () => {
		expect(validateLevel('1')).toBeUndefined();
		expect(validateLevel('60')).toBeUndefined();
		expect(validateLevel('50')).toBeUndefined();
	});
	it('0 and 61 are out of range', () => {
		expect(validateLevel('0')).toBe(LEVEL_ERROR);
		expect(validateLevel('61')).toBe(LEVEL_ERROR);
	});
	it('non-digits are rejected (no sign / decimal / exponent / letters)', () => {
		expect(validateLevel('-5')).toBe(LEVEL_ERROR);
		expect(validateLevel('1.5')).toBe(LEVEL_ERROR);
		expect(validateLevel('5abc')).toBe(LEVEL_ERROR);
	});
});

describe('validateClass / validateRace — membership in the mirrored sets', () => {
	it('an abbreviation is valid; a display name is rejected (Pitfall 5)', () => {
		expect(validateClass('WAR')).toBeUndefined();
		expect(validateClass('Warrior')).toBeTruthy();
		expect(validateRace('IKS')).toBeUndefined();
		expect(validateRace('Iksar')).toBeTruthy();
	});
	it('blank is allowed at the field layer (the Save gate requires non-blank)', () => {
		expect(validateClass('')).toBeUndefined();
		expect(validateRace('')).toBeUndefined();
	});
});

describe('charMetaIsValid — the Save gate validity half', () => {
	it('requires a non-blank class AND race (level may be blank)', () => {
		expect(charMetaIsValid(inputs({ class: 'WAR', race: 'IKS' }))).toBe(true);
		expect(charMetaIsValid(inputs({ class: 'WAR', race: 'IKS', level: '50' }))).toBe(true);
		// Blank class or race → not a meaningful write.
		expect(charMetaIsValid(inputs({ class: '', race: 'IKS' }))).toBe(false);
		expect(charMetaIsValid(inputs({ class: 'WAR', race: '' }))).toBe(false);
		// An out-of-range level invalidates even with class+race set.
		expect(charMetaIsValid(inputs({ class: 'WAR', race: 'IKS', level: '61' }))).toBe(false);
	});
});

describe('levelPayload / charMetaPayload — snake_case wire shape', () => {
	it('a blank level → null (server stores NULL); a value → its number', () => {
		expect(levelPayload('')).toBeNull();
		expect(levelPayload('50')).toBe(50);
	});
	it('charMetaPayload mirrors the Go charMetaReq JSON tags', () => {
		expect(charMetaPayload(inputs({ class: 'WAR', race: 'IKS', level: '50', isBankToon: true }))).toEqual({
			class: 'WAR',
			level: 50,
			race: 'IKS',
			is_bank_toon: true
		});
		expect(charMetaPayload(inputs({ class: 'CLR', race: 'HUM', level: '', isBankToon: false }))).toEqual({
			class: 'CLR',
			level: null,
			race: 'HUM',
			is_bank_toon: false
		});
	});
});

describe('inputsFromChar / charMetaChanged — pre-fill + the Save gate', () => {
	it('inputsFromChar pre-fills from a loaded char (level 0/null → blank)', () => {
		expect(inputsFromChar(char({ class: 'WAR', level: 50, race: 'IKS', is_bank_toon: true }))).toEqual({
			class: 'WAR',
			race: 'IKS',
			level: '50',
			isBankToon: true
		});
		// An unset (null) or 0 level shows blank, never a fabricated "0".
		expect(levelToInput(null)).toBe('');
		expect(levelToInput(0)).toBe('');
		expect(inputsFromChar(char()).level).toBe('');
	});
	it('charMetaChanged is true on a class change, false when unchanged', () => {
		const c = char({ class: 'WAR', level: 50, race: 'IKS', is_bank_toon: false });
		// Identical → no diff (Save stays disabled).
		expect(charMetaChanged(inputsFromChar(c), c)).toBe(false);
		// Class change → changed.
		expect(charMetaChanged(inputs({ class: 'CLR', race: 'IKS', level: '50' }), c)).toBe(true);
		// Level change → changed.
		expect(charMetaChanged(inputs({ class: 'WAR', race: 'IKS', level: '51' }), c)).toBe(true);
		// is_bank_toon flip → changed.
		expect(charMetaChanged(inputs({ class: 'WAR', race: 'IKS', level: '50', isBankToon: true }), c)).toBe(true);
	});
});

describe('CR-01 input-contract: helpers tolerate the value types the DOM binding produces', () => {
	// Regression for CR-01 (the P15 bank-coin crash). Svelte's bind:value on a
	// number-like input coerces the written-back value to `number` (or `null` when
	// emptied). If the level input is ever switched to type="number", these shapes
	// reach the helpers — they MUST NOT throw `raw.trim is not a function`. Drive the
	// helpers with number/null/undefined directly to lock the crash-proof contract.
	it('validateLevel does NOT throw on a number / null / undefined', () => {
		expect(validateLevel(5 as unknown as string)).toBeUndefined();
		expect(validateLevel(60 as unknown as string)).toBeUndefined();
		expect(validateLevel(0 as unknown as string)).toBe(LEVEL_ERROR);
		expect(validateLevel(61 as unknown as string)).toBe(LEVEL_ERROR);
		expect(validateLevel(null as unknown as string)).toBeUndefined();
		expect(validateLevel(undefined as unknown as string)).toBeUndefined();
		expect(validateLevel(1.5 as unknown as string)).toBe(LEVEL_ERROR);
	});
	it('levelPayload does NOT throw on a number / null and returns number|null', () => {
		expect(levelPayload(50 as unknown as string)).toBe(50);
		expect(levelPayload(null as unknown as string)).toBeNull();
		expect(levelPayload(undefined as unknown as string)).toBeNull();
		expect(levelPayload('' as unknown as string)).toBeNull();
	});
	it('charMetaIsValid / charMetaPayload survive a number-coerced level', () => {
		const coerced = inputs({ class: 'WAR', race: 'IKS', level: 50 as unknown as string });
		expect(() => charMetaIsValid(coerced)).not.toThrow();
		expect(charMetaIsValid(coerced)).toBe(true);
		expect(charMetaPayload(coerced)).toEqual({ class: 'WAR', level: 50, race: 'IKS', is_bank_toon: false });
	});
});

// --- Style B: .svelte source-assertion (node-only, DOM-blind) --------------
// The repo runs vitest with NO jsdom / @testing-library, so it cannot mount the
// component. Read the source string and assert on the markup — this is how the repo
// catches the CR-01 input-type lesson + the D-03 member-accessible nav placement.
const CHARMETA_FORM_SOURCE = readFileSync(
	fileURLToPath(new URL('../components/CharMetaForm.svelte', import.meta.url)),
	'utf8'
);
// The /char-meta + /account links moved from SiteShell into the header
// SettingsMenu gear dropdown (260607-sdh IA cleanup). The D-03 member-accessible
// contract now lives there: the menu itself is rendered by SiteShell only under
// {#if session?.authenticated}, and inside the menu /char-meta sits OUTSIDE the
// {#if session?.isOfficer} gate (which wraps only /admin).
const SETTINGS_MENU_SOURCE = readFileSync(
	fileURLToPath(new URL('../components/SettingsMenu.svelte', import.meta.url)),
	'utf8'
);
const SITE_SHELL_SOURCE = readFileSync(
	fileURLToPath(new URL('../components/SiteShell.svelte', import.meta.url)),
	'utf8'
);

describe('CharMetaForm.svelte source — the CR-01 input-type guard + the wiring', () => {
	it('the level input is type="text" inputmode="numeric", never type="number" (CR-01)', () => {
		expect(CHARMETA_FORM_SOURCE).toContain('inputmode="numeric"');
		expect(CHARMETA_FORM_SOURCE).not.toContain('type="number"');
	});
	it('is wired to the login-only char-meta flow (fetchCharsForMeta + saveCharMeta)', () => {
		expect(CHARMETA_FORM_SOURCE).toContain('fetchCharsForMeta');
		expect(CHARMETA_FORM_SOURCE).toContain('saveCharMeta');
	});
	it('renders the character name via plain interpolation, never {@html} (T-16-03 XSS)', () => {
		expect(CHARMETA_FORM_SOURCE).not.toContain('{@html');
	});
	it('is NOT officer-gated (no isOfficer / RequireOfficer in the form)', () => {
		expect(CHARMETA_FORM_SOURCE).not.toContain('isOfficer');
	});
});

describe('SettingsMenu.svelte source — the /char-meta nav is member-accessible (D-03)', () => {
	it('surfaces a /char-meta link (relocated into the gear menu, 260607-sdh)', () => {
		expect(SETTINGS_MENU_SOURCE).toContain('/char-meta');
	});
	it('renders the SettingsMenu only for an authenticated member (the menu sits under session?.authenticated in SiteShell)', () => {
		// SiteShell gates the WHOLE gear menu behind the single authenticated guard,
		// so every item inside — /char-meta included — is member-accessible (D-03).
		// Assert <SettingsMenu> falls between {#if session?.authenticated} and its
		// matching {/if}, and is NOT itself wrapped in an officer gate.
		const authGuardIdx = SITE_SHELL_SOURCE.indexOf('{#if session?.authenticated}');
		const menuIdx = SITE_SHELL_SOURCE.indexOf('<SettingsMenu');
		expect(authGuardIdx).toBeGreaterThan(-1);
		expect(menuIdx).toBeGreaterThan(authGuardIdx);
		// The SettingsMenu line itself carries no officer gate (Admin is gated INSIDE
		// the menu component, not at the SiteShell render site).
		const menuLine = SITE_SHELL_SOURCE.slice(menuIdx, SITE_SHELL_SOURCE.indexOf('\n', menuIdx));
		expect(menuLine).not.toContain('isOfficer');
		// There is exactly one officer gate in SiteShell now: none (Admin moved into
		// the menu). Confirm SiteShell no longer gates anything on isOfficer.
		expect(SITE_SHELL_SOURCE).not.toContain('session?.isOfficer');
	});
	it('does NOT officer-gate /char-meta (it sits outside the {#if session?.isOfficer} block that wraps only /admin)', () => {
		// The Admin <a href="/admin"> is the ONLY officer-gated nav item. Verify the
		// /char-meta LINK precedes the officer gate that immediately wraps /admin — so
		// /char-meta is never trapped behind isOfficer (D-03). Match the hrefs (not
		// bare paths) to skip comment mentions, and find the officer gate that
		// directly precedes the /admin link (NOT the earlier identity-Shield gate).
		const charMetaIdx = SETTINGS_MENU_SOURCE.indexOf('href="/char-meta"');
		const adminIdx = SETTINGS_MENU_SOURCE.indexOf('href="/admin"');
		expect(charMetaIdx).toBeGreaterThan(-1);
		expect(adminIdx).toBeGreaterThan(charMetaIdx);
		// The officer gate that wraps /admin is the last {#if session?.isOfficer}
		// before the /admin link; it must open AFTER /char-meta (so /char-meta is
		// outside it).
		const adminGateIdx = SETTINGS_MENU_SOURCE.lastIndexOf('{#if session?.isOfficer}', adminIdx);
		expect(adminGateIdx).toBeGreaterThan(charMetaIdx);
	});
});
