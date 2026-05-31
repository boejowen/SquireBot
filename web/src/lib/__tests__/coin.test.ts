// Vitest for the pure bank-coin validation + change/surfacing helpers (Plan
// 15-05 Task 2, D-11). These prove the BankCoinForm's behavioral contract
// (range validation, the Save gate, the bank-view surfacing predicate) under
// the repo's node-only test project — the form .svelte is a thin renderer over
// them (15-04's pure-logic-in-.ts philosophy, carried forward).

import { describe, it, expect } from 'vitest';
import type { BankToon } from '../api';
import {
	validateCoinField,
	validateCoin,
	coinIsValid,
	coinValue,
	coinPayload,
	coinToInput,
	inputsFromToon,
	coinChanged,
	hasRecordedCoin,
	PLAT_ERROR,
	SUBUNIT_ERROR,
	type CoinInputs
} from '../coin';

function toon(over: Partial<BankToon> = {}): BankToon {
	return { character_id: 1, name: 'Banker', plat: null, gold: null, silver: null, copper: null, ...over };
}

function inputs(over: Partial<CoinInputs> = {}): CoinInputs {
	return { plat: '', gold: '', silver: '', copper: '', ...over };
}

describe('validateCoinField — D-11 ranges + the exact UI-SPEC copy', () => {
	it('platinum: a non-negative integer is valid; large values are allowed (no upper bound)', () => {
		expect(validateCoinField('plat', '0')).toBeUndefined();
		expect(validateCoinField('plat', '5')).toBeUndefined();
		expect(validateCoinField('plat', '1000000')).toBeUndefined();
	});

	it('platinum: a blank field is valid (treated as 0)', () => {
		expect(validateCoinField('plat', '')).toBeUndefined();
		expect(validateCoinField('plat', '   ')).toBeUndefined();
	});

	it('platinum: negative / decimal / non-numeric → the exact platinum error', () => {
		expect(validateCoinField('plat', '-1')).toBe(PLAT_ERROR);
		expect(validateCoinField('plat', '1.5')).toBe(PLAT_ERROR);
		expect(validateCoinField('plat', '5abc')).toBe(PLAT_ERROR);
		expect(validateCoinField('plat', 'abc')).toBe(PLAT_ERROR);
	});

	it('gold/silver/copper: 0..999 inclusive is valid', () => {
		for (const f of ['gold', 'silver', 'copper'] as const) {
			expect(validateCoinField(f, '0')).toBeUndefined();
			expect(validateCoinField(f, '999')).toBeUndefined();
			expect(validateCoinField(f, '500')).toBeUndefined();
		}
	});

	it('gold/silver/copper: 1000+ or negative or non-integer → the exact 0–999 error', () => {
		expect(validateCoinField('gold', '1000')).toBe(SUBUNIT_ERROR);
		expect(validateCoinField('silver', '-1')).toBe(SUBUNIT_ERROR);
		expect(validateCoinField('copper', '12.3')).toBe(SUBUNIT_ERROR);
		expect(validateCoinField('gold', 'x')).toBe(SUBUNIT_ERROR);
	});
});

describe('coinIsValid + validateCoin', () => {
	it('a fully-blank quad is valid (all zeros)', () => {
		expect(coinIsValid(inputs())).toBe(true);
	});

	it('one out-of-range subunit invalidates the quad and reports only that field', () => {
		const errs = validateCoin(inputs({ gold: '1000' }));
		expect(errs.gold).toBe(SUBUNIT_ERROR);
		expect(errs.plat).toBeUndefined();
		expect(errs.silver).toBeUndefined();
		expect(coinIsValid(inputs({ gold: '1000' }))).toBe(false);
	});

	it('a bad platinum invalidates the quad', () => {
		expect(coinIsValid(inputs({ plat: '-5' }))).toBe(false);
	});
});

describe('coinValue / coinPayload', () => {
	it('blank → 0; otherwise the integer', () => {
		expect(coinValue('')).toBe(0);
		expect(coinValue('  ')).toBe(0);
		expect(coinValue('42')).toBe(42);
	});

	it('coinPayload builds the snake_case numeric quad (blank fields → 0)', () => {
		expect(coinPayload(inputs({ plat: '10', gold: '5' }))).toEqual({
			plat: 10,
			gold: 5,
			silver: 0,
			copper: 0
		});
	});
});

describe('coinToInput / inputsFromToon — null → empty (never a fabricated 0)', () => {
	it('null/undefined → "" so the input renders empty', () => {
		expect(coinToInput(null)).toBe('');
		expect(coinToInput(undefined)).toBe('');
		expect(coinToInput(0)).toBe('0');
		expect(coinToInput(7)).toBe('7');
	});

	it('inputsFromToon pre-fills recorded values and blanks the nulls', () => {
		expect(inputsFromToon(toon({ plat: 100, gold: null, silver: 50, copper: null }))).toEqual({
			plat: '100',
			gold: '',
			silver: '50',
			copper: ''
		});
	});
});

describe('coinChanged — the Save-coin gate (disabled until ≥1 differs)', () => {
	it('false when every input matches the loaded value (null treated as 0)', () => {
		const t = toon({ plat: 10, gold: null, silver: null, copper: null });
		expect(coinChanged(inputsFromToon(t), t)).toBe(false);
	});

	it('false when re-typing the same number on an all-null toon (blank≡0≡null)', () => {
		const t = toon(); // all null
		expect(coinChanged(inputs(), t)).toBe(false);
		// Typing 0 into a null field is still "no change" (0 === null→0).
		expect(coinChanged(inputs({ plat: '0' }), t)).toBe(false);
	});

	it('true when a field is increased', () => {
		const t = toon({ plat: 10 });
		expect(coinChanged(inputs({ plat: '11' }), t)).toBe(true);
	});

	it('true when clearing a recorded value to 0 (blank on a non-null toon)', () => {
		const t = toon({ gold: 5 });
		// gold loaded 5, input blank (→0) ⇒ changed.
		expect(coinChanged(inputsFromToon(toon({ gold: 5 })) , t)).toBe(false); // same value pre-filled
		expect(coinChanged({ plat: '', gold: '', silver: '', copper: '' }, t)).toBe(true);
	});
});

describe('hasRecordedCoin — the bank-view surfacing predicate', () => {
	it('false when ALL coin columns are null (P14 placeholder stays)', () => {
		expect(hasRecordedCoin(toon())).toBe(false);
	});

	it('true when ANY column is recorded (even 0)', () => {
		expect(hasRecordedCoin(toon({ plat: 0 }))).toBe(true);
		expect(hasRecordedCoin(toon({ copper: 12 }))).toBe(true);
	});
});
