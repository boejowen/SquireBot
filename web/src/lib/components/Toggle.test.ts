// Vitest for the Toggle primitive's DOM-free word derivation (20-04 Task 1). The
// repo runs vitest node-only (no jsdom, no @testing-library/svelte) — so we test
// the EXTRACTED pure function `onLabel`, NOT a mounted component. This pins the
// P15 number-input-coercion class of crasher: the switch must derive a clean
// 'ON'/'OFF' from a STRICT boolean and never leak a truthy-but-non-bool value
// (`1`, a string, undefined) into the rendered word.

import { describe, it, expect } from 'vitest';
import { onLabel } from './Toggle.svelte';

describe('Toggle onLabel — strict ON/OFF derivation (P15 coercion guard)', () => {
	it("maps true → 'ON'", () => {
		expect(onLabel(true)).toBe('ON');
	});

	it("maps false → 'OFF'", () => {
		expect(onLabel(false)).toBe('OFF');
	});

	it('never returns anything but ON or OFF for a strict boolean', () => {
		expect(['ON', 'OFF']).toContain(onLabel(true));
		expect(['ON', 'OFF']).toContain(onLabel(false));
	});

	it("coerces a truthy-but-non-bool to 'OFF' rather than leaking the raw value (P15 crasher)", () => {
		// These would be type errors in real call sites, but a runtime payload glitch
		// (e.g. a JSON 0/1, or undefined from a partial parse) must STILL render a
		// clean word — never `1`/`undefined`/a string. onLabel uses `on === true`.
		expect(onLabel(1 as unknown as boolean)).toBe('OFF');
		expect(onLabel('yes' as unknown as boolean)).toBe('OFF');
		expect(onLabel(undefined as unknown as boolean)).toBe('OFF');
		expect(onLabel(null as unknown as boolean)).toBe('OFF');
	});
});
