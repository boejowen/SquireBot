// Pure bank-coin validation + change-detection helpers (Plan 15-05 Task 2,
// D-11). Extracted from BankCoinForm.svelte so the validation contract is
// node-unit-testable WITHOUT a DOM (the repo's established philosophy — see
// 15-04 SUMMARY: vitest runs node-only, no @testing-library/svelte, no jsdom;
// the form is a thin renderer over these). The SERVER (15-03) is the
// authoritative validator (re-checks → 400 invalid_input / not_bank_toon);
// this is UX defense-in-depth only (threat T-15-29 disposition: accept).

import type { BankToon } from './api';

/** The four coin field keys, in display order (Platinum→Copper). */
export const COIN_FIELDS = ['plat', 'gold', 'silver', 'copper'] as const;
export type CoinField = (typeof COIN_FIELDS)[number];

/** The four inputs as raw strings (a number input's value is a string). */
export type CoinInputs = Record<CoinField, string>;

/** The UI-SPEC inline error copy (verbatim). */
export const PLAT_ERROR = 'Enter a whole number (0 or more).';
export const SUBUNIT_ERROR = 'Enter 0–999.';

/**
 * Validate ONE coin field's raw string. Platinum is a free non-negative integer
 * (no upper bound — a guild bank can hold large plat). Gold/silver/copper are
 * bounded 0–999 (EQ carries at 1000 → the next denomination, D-11). An empty
 * string is treated as 0 (a blank field = zero coin, valid). Returns the exact
 * UI-SPEC error string, or undefined when valid.
 */
export function validateCoinField(field: CoinField, raw: string): string | undefined {
	const trimmed = raw.trim();
	// Blank = 0 (valid). Otherwise it must be a non-negative integer (no sign, no
	// decimal, no exponent) — a strict digits-only check (parseInt would accept
	// "5abc"; Number("") is 0; both are wrong here).
	if (trimmed === '') return undefined;
	if (!/^\d+$/.test(trimmed)) {
		return field === 'plat' ? PLAT_ERROR : SUBUNIT_ERROR;
	}
	const n = Number(trimmed);
	if (field === 'plat') {
		// Non-negative integer; the regex already excludes negatives/decimals.
		return Number.isSafeInteger(n) ? undefined : PLAT_ERROR;
	}
	// gold/silver/copper: 0..999 inclusive.
	return n >= 0 && n <= 999 ? undefined : SUBUNIT_ERROR;
}

/** Per-field errors for the whole quad (a field maps to undefined when valid). */
export function validateCoin(inputs: CoinInputs): Record<CoinField, string | undefined> {
	return {
		plat: validateCoinField('plat', inputs.plat),
		gold: validateCoinField('gold', inputs.gold),
		silver: validateCoinField('silver', inputs.silver),
		copper: validateCoinField('copper', inputs.copper)
	};
}

/** True when every field validates (a blank field counts as a valid 0). */
export function coinIsValid(inputs: CoinInputs): boolean {
	return COIN_FIELDS.every((f) => validateCoinField(f, inputs[f]) === undefined);
}

/** Coerce a raw field string to its integer value (blank → 0). Assumes validity. */
export function coinValue(raw: string): number {
	const t = raw.trim();
	return t === '' ? 0 : Number(t);
}

/** The numeric quad (snake_case) the save payload carries. Assumes validity. */
export function coinPayload(inputs: CoinInputs): {
	plat: number;
	gold: number;
	silver: number;
	copper: number;
} {
	return {
		plat: coinValue(inputs.plat),
		gold: coinValue(inputs.gold),
		silver: coinValue(inputs.silver),
		copper: coinValue(inputs.copper)
	};
}

/** Render a stored coin value (null → "" so the input shows empty, not "0"). */
export function coinToInput(v: number | null | undefined): string {
	return v === null || v === undefined ? '' : String(v);
}

/** The four inputs pre-filled from a bank toon's current coin (null → blank). */
export function inputsFromToon(toon: BankToon): CoinInputs {
	return {
		plat: coinToInput(toon.plat),
		gold: coinToInput(toon.gold),
		silver: coinToInput(toon.silver),
		copper: coinToInput(toon.copper)
	};
}

/**
 * True when at least one input differs from the toon's loaded value — the
 * "Save coin" gate (disabled until valid AND changed). A blank input and a
 * null/0 stored value are compared by numeric value (blank→0), so re-typing the
 * same number does NOT enable Save, but clearing a recorded value (→ 0) does.
 */
export function coinChanged(inputs: CoinInputs, toon: BankToon): boolean {
	return COIN_FIELDS.some((f) => coinValue(inputs[f]) !== (toon[f] ?? 0));
}

/** True when a toon has ANY recorded (non-null) coin value — drives the bank-view surfacing. */
export function hasRecordedCoin(toon: BankToon): boolean {
	return toon.plat !== null || toon.gold !== null || toon.silver !== null || toon.copper !== null;
}
