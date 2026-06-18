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
	return { class: '', race: '', level: '', ...over };
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
	it('charMetaPayload mirrors the Go charMetaReq JSON tags (class/level/race only)', () => {
		expect(charMetaPayload(inputs({ class: 'WAR', race: 'IKS', level: '50' }))).toEqual({
			class: 'WAR',
			level: 50,
			race: 'IKS'
		});
		expect(charMetaPayload(inputs({ class: 'CLR', race: 'HUM', level: '' }))).toEqual({
			class: 'CLR',
			level: null,
			race: 'HUM'
		});
	});
	it('charMetaPayload OMITS is_bank_toon (officer-only now — Phase 26 / OPEN-3)', () => {
		// The member path no longer sends the bank-toon flag; the key must be absent
		// (not present-and-false) so the server-side member handler never receives it.
		expect(charMetaPayload(inputs({ class: 'WAR', race: 'IKS', level: '50' }))).not.toHaveProperty(
			'is_bank_toon'
		);
	});
});

describe('inputsFromChar / charMetaChanged — pre-fill + the Save gate', () => {
	it('inputsFromChar pre-fills from a loaded char (level 0/null → blank)', () => {
		expect(inputsFromChar(char({ class: 'WAR', level: 50, race: 'IKS', is_bank_toon: true }))).toEqual({
			class: 'WAR',
			race: 'IKS',
			level: '50'
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
		expect(charMetaPayload(coerced)).toEqual({ class: 'WAR', level: 50, race: 'IKS' });
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
// Phase 30 / D-06: the header gear DISSOLVED. The /char-meta + /account + /admin
// links no longer live in SettingsMenu — they moved into the Settings TAB as
// in-page sections (composed in Plan 02). SettingsMenu is now identity + Sign out
// ONLY. The Set-class-&-level surface (CharMetaForm) is reached via the persistent
// 5-tab strip → Settings tab (rendered by SiteShell whenever session?.authenticated).
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

describe('SettingsMenu.svelte source — the gear is DISSOLVED to identity + Sign out (Phase 30 / D-06)', () => {
	it('is pruned to identity + Sign out: keeps the escaped username + signOut', () => {
		// The dissolved menu still carries the signed-in identity + the Sign out flow.
		expect(SETTINGS_MENU_SOURCE).toContain('signOut');
		expect(SETTINGS_MENU_SOURCE).toContain('{session.username}');
	});
	it('no longer holds the relocated config nav links (they moved to the Settings tab)', () => {
		// D-06: Watcher codes (/account), Set class & level (/char-meta), My characters
		// (/my-characters) and the officer Admin (/admin) are GONE from this menu — each
		// is now an in-page Settings section. The Theme picker likewise left the menu.
		expect(SETTINGS_MENU_SOURCE).not.toContain('href="/char-meta"');
		expect(SETTINGS_MENU_SOURCE).not.toContain('href="/account"');
		expect(SETTINGS_MENU_SOURCE).not.toContain('href="/my-characters"');
		expect(SETTINGS_MENU_SOURCE).not.toContain('href="/admin"');
		expect(SETTINGS_MENU_SOURCE).not.toContain('ThemePicker');
	});
	it('renders the SettingsMenu only for an authenticated member (it sits under session?.authenticated in SiteShell)', () => {
		// The identity + Sign out affordance is member-only chrome: SiteShell renders
		// <SettingsMenu> inside {#if session?.authenticated}. (The server RequireSession
		// gate is the real boundary; this is UX.)
		const authGuardIdx = SITE_SHELL_SOURCE.indexOf('{#if session?.authenticated}');
		const menuIdx = SITE_SHELL_SOURCE.indexOf('<SettingsMenu');
		expect(authGuardIdx).toBeGreaterThan(-1);
		expect(menuIdx).toBeGreaterThan(authGuardIdx);
		// The SettingsMenu line itself carries no officer gate.
		const menuLine = SITE_SHELL_SOURCE.slice(menuIdx, SITE_SHELL_SOURCE.indexOf('\n', menuIdx));
		expect(menuLine).not.toContain('isOfficer');
	});
});

describe('SettingsMenu.svelte source — the T-15-22 / T-30-01 XSS invariant survives the dissolve', () => {
	it('renders the user-controlled username via plain interpolation, never {@html}', () => {
		// The username + avatar alt are Discord-controlled; they MUST stay plain {}
		// (Svelte auto-escape), never the raw-HTML directive (T-15-22 / T-30-01).
		expect(SETTINGS_MENU_SOURCE).not.toContain('{@html');
		expect(SETTINGS_MENU_SOURCE).toContain('alt={session.username}');
	});
});

describe('The /char-meta surface (CharMetaForm) is now reached via the 5-tab Settings route', () => {
	it('CharMetaForm is no longer mounted by SettingsMenu (it moves to the Settings tab in Plan 02)', () => {
		// Negative assertion: SettingsMenu no longer links /char-meta; the form is
		// composed onto /settings (asserted in Plan 02's test scope once that page lands).
		// Its member-accessible contract (no officer gate on the form) is still proven
		// by the CharMetaForm.svelte source assertions above.
		expect(SETTINGS_MENU_SOURCE).not.toContain('/char-meta');
	});
});
