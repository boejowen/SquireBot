// Pure char-meta validation + change-detection helpers (Phase 16 Task 2,
// CUTOVER-02 / D-02/D-03). Extracted from CharMetaForm.svelte so the validation
// contract is node-unit-testable WITHOUT a DOM (the repo's established philosophy —
// vitest runs node-only, no @testing-library/svelte, no jsdom; the form is a thin
// renderer over these). The SERVER (webadmin/charmeta.go) is the authoritative
// validator (re-checks class ∈ CLASSES / race ∈ RACES / level 1..60 → 400
// invalid_input); this is UX defense-in-depth only (T-16-02 disposition: accept the
// client layer, mitigate at the server).

import type { CharMetaItem } from './api';

/**
 * The class abbreviations the class <select> offers — mirrors the Go enrich.CLASSES
 * set (UX only; the server is authoritative). Store the abbreviation, never a
 * display name (a typo silently breaks the gear/spell joins — Pitfall 5).
 */
export const CLASSES = [
	'WAR', 'CLR', 'PAL', 'RNG', 'SHD', 'DRU', 'MNK', 'BRD',
	'ROG', 'SHM', 'NEC', 'WIZ', 'MAG', 'ENC'
] as const;

/**
 * The race abbreviations the race <select> offers — mirrors the Go enrich.RACES
 * set. "IKS" is load-bearing: compute/gearcheck.go keys the Iksar gear tier on this
 * exact literal.
 */
export const RACES = [
	'HUM', 'BAR', 'ERU', 'ELF', 'HIE', 'DEF', 'HEF', 'DWF',
	'TRL', 'OGR', 'HFL', 'GNM', 'IKS', 'VAH'
] as const;

/** The inline error copy for an out-of-range / malformed level. */
export const LEVEL_ERROR = 'Enter a level from 1 to 60 (or leave blank).';
/** The inline error copy for a class/race not in the value set (a <select> should make this unreachable). */
export const CLASS_ERROR = 'Choose a class.';
export const RACE_ERROR = 'Choose a race.';

/**
 * The value type a single char-meta helper accepts (CR-01 input contract). The form
 * binds the level to a TEXT input so the value is a string, but Svelte's
 * number-input coercion (and any future re-introduction of type="number") can write
 * a `number` or `null` back into the bound store. Accepting the union here makes the
 * helpers crash-proof regardless of the input element — the earlier
 * `.trim()`-on-a-number TypeError (CR-01, the P15 bank-coin crash) is structurally
 * impossible.
 */
export type CharMetaRaw = string | number | null | undefined;

/**
 * Normalize whatever the binding produced (string | number | null | undefined) to
 * the trimmed string the validation logic expects. null/undefined → '' (blank =
 * unset, valid); a number → its string form (a non-finite number becomes '' so it
 * validates as blank, and a fractional number keeps its decimal so the /^\d+$/ check
 * still rejects it). This is the single choke point that lets every helper below
 * treat its input as a string. Copied verbatim from coin.ts's rawToTrimmed.
 */
function rawToTrimmed(raw: CharMetaRaw): string {
	if (raw === null || raw === undefined) return '';
	if (typeof raw === 'number') return Number.isFinite(raw) ? String(raw) : '';
	return raw.trim();
}

/** The four char-meta form fields. level is a raw string; isBankToon a checkbox bool. */
export interface CharMetaInputs {
	class: string;
	race: string;
	level: string;
	isBankToon: boolean;
}

/**
 * Validate the level field. Blank is valid (unset — a member may know a char's
 * class+race before its level; the server stores NULL and spellcheck treats it as
 * unleveled). A non-blank level must be a whole number 1..60 — a strict digits-only
 * check (no sign / decimal / exponent / letters; parseInt would accept "5abc",
 * Number("") is 0 — both wrong here). Returns the error copy, or undefined when valid.
 */
export function validateLevel(raw: CharMetaRaw): string | undefined {
	const trimmed = rawToTrimmed(raw);
	if (trimmed === '') return undefined; // blank = unset (valid)
	if (!/^\d+$/.test(trimmed)) return LEVEL_ERROR;
	const n = Number(trimmed);
	if (Number.isSafeInteger(n) && n >= 1 && n <= 60) return undefined;
	return LEVEL_ERROR;
}

/** Validate the class value — blank (allowed at the field layer) or a CLASSES member. */
export function validateClass(v: string): string | undefined {
	return v === '' || (CLASSES as readonly string[]).includes(v) ? undefined : CLASS_ERROR;
}

/** Validate the race value — blank (allowed at the field layer) or a RACES member. */
export function validateRace(v: string): string | undefined {
	return v === '' || (RACES as readonly string[]).includes(v) ? undefined : RACE_ERROR;
}

/**
 * True when the inputs are a meaningful, valid write: class AND race non-blank (a
 * char with no class is skipped by gear/spell check, so a blank class is not worth
 * saving) AND the level valid (blank allowed). The "changed" half of the Save gate
 * is charMetaChanged.
 */
export function charMetaIsValid(inputs: CharMetaInputs): boolean {
	return (
		inputs.class !== '' &&
		inputs.race !== '' &&
		validateClass(inputs.class) === undefined &&
		validateRace(inputs.race) === undefined &&
		validateLevel(inputs.level) === undefined
	);
}

/**
 * The level wire value: a blank level → null (the server stores SQL NULL — unset,
 * not 0), otherwise its number. Assumes the level already validated.
 */
export function levelPayload(raw: CharMetaRaw): number | null {
	const t = rawToTrimmed(raw);
	return t === '' ? null : Number(t);
}

/**
 * The snake_case payload the save POST carries — matches the Go charMetaReq JSON
 * tags (class / level / race / is_bank_toon). level is number|null (blank → null).
 */
export function charMetaPayload(inputs: CharMetaInputs): {
	class: string;
	level: number | null;
	race: string;
	is_bank_toon: boolean;
} {
	return {
		class: inputs.class,
		level: levelPayload(inputs.level),
		race: inputs.race,
		is_bank_toon: inputs.isBankToon
	};
}

/** Render a stored level (null/0 → '' so the input shows empty, never a fabricated "0"). */
export function levelToInput(v: number | null | undefined): string {
	return v === null || v === undefined || v === 0 ? '' : String(v);
}

/** The four inputs pre-filled from an existing character (null/0 level → blank). */
export function inputsFromChar(c: CharMetaItem): CharMetaInputs {
	return {
		class: c.class ?? '',
		race: c.race ?? '',
		level: levelToInput(c.level),
		isBankToon: c.is_bank_toon
	};
}

/**
 * True when at least one input differs from the loaded character — the "Save" gate
 * (disabled until valid AND changed). A blank level and a null/0 stored level are
 * compared by their payload (blank → null), so re-typing the same value does NOT
 * enable Save, but clearing a recorded level (→ null) does.
 */
export function charMetaChanged(inputs: CharMetaInputs, c: CharMetaItem): boolean {
	const storedLevel = c.level === 0 ? null : c.level;
	return (
		inputs.class !== (c.class ?? '') ||
		inputs.race !== (c.race ?? '') ||
		levelPayload(inputs.level) !== storedLevel ||
		inputs.isBankToon !== c.is_bank_toon
	);
}
