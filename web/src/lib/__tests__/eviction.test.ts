// Vitest for the pure eviction grace-date helper (CR-02). Proves the
// epoch-SECONDS → human-date contract under the repo's node-only test project
// (the form .svelte is a thin renderer over it). This is the regression that the
// prior tests could not catch: nothing exercised the real JSON shape (an epoch
// SECONDS number) against new Date()'s MILLISECONDS expectation, so the "Jan 1970"
// bug shipped green.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { canEvictPreview, evictPreviewSummary, graceDate, restoreResultMessage } from '../eviction';
import type { EvictionPreview, RestoreResult } from '../api';

// Source-inspect the form the way AuthGate.test.ts / ConfirmDialog.test.ts do
// (node-only; the repo runs vitest with NO jsdom / @testing-library — see the file
// header). Proves the Restore section is actually wired to restoreEviction + the
// ConfirmDialog rather than mounting the component.
const EVICTION_FORM_SOURCE = readFileSync(
	fileURLToPath(new URL('../components/EvictionForm.svelte', import.meta.url)),
	'utf8'
);

describe('graceDate — epoch SECONDS → human date (CR-02)', () => {
	it('converts an epoch-SECONDS value to its real date, NOT 1970', () => {
		// 1782789805 s = 2026-06-30T07:23:25Z. The bug fed this straight to
		// new Date() (which reads MILLISECONDS) and produced "Wed Jan 21 1970".
		const secs = 1782789805;
		const out = graceDate(secs);
		expect(out).not.toContain('1970');
		// Same instant, computed the correct way (seconds→ms) — must match exactly.
		expect(out).toBe(new Date(secs * 1000).toDateString());
		// And it is the year the epoch-seconds value actually denotes.
		expect(out).toContain('2026');
	});

	it('a known epoch-seconds midnight maps to the expected calendar date', () => {
		// 2026-07-30T00:00:00Z in seconds.
		const secs = Math.floor(Date.UTC(2026, 6, 30, 0, 0, 0) / 1000);
		expect(graceDate(secs)).toBe(new Date(secs * 1000).toDateString());
	});

	it('0 seconds maps to the epoch instant (treated as seconds, not a falsy passthrough)', () => {
		// 0s = 0ms = the epoch. Compared against the correctly-computed value so the
		// assertion is timezone-robust (the local date for the epoch is "Dec 31 1969"
		// at negative UTC offsets, "Jan 1 1970" otherwise — both are correct).
		expect(graceDate(0)).toBe(new Date(0).toDateString());
	});

	it('a non-finite value falls back to its string form (never "Invalid Date")', () => {
		expect(graceDate(NaN)).toBe('NaN');
		expect(graceDate(Infinity)).toBe('Infinity');
	});
});

describe('restoreResultMessage — WR-01/WR-02 success copy (close G-1)', () => {
	const label = 'Slampeach';

	it('new_code_issued → says the code is SERVER-SIDE, never shown in-browser (WR-02)', () => {
		const res: RestoreResult = { restored_count: 2, new_code_issued: true };
		const msg = restoreResultMessage(res, label);
		expect(msg).toContain(label);
		// WR-02: the copy must locate the code on the SERVER and explicitly deny it is
		// shown here — it must NOT imply the officer already holds a deliverable code.
		expect(msg).toContain('SERVER');
		expect(msg).toContain('mint-code');
		expect(msg.toLowerCase()).toContain('not shown here');
	});

	it('code_mint_failed → tells the officer to re-issue the code on the server (WR-01)', () => {
		// The restore committed but the follow-on re-mint failed: the guildie is
		// restored-but-codeless; the copy must surface the re-issue step.
		const res: RestoreResult = {
			restored_count: 1,
			new_code_issued: false,
			code_mint_failed: true
		};
		const msg = restoreResultMessage(res, label);
		expect(msg).toContain(label);
		expect(msg).toContain('mint-code');
		expect(msg.toLowerCase()).toContain('failed');
	});

	it('neither outcome leaks a literal code value into the copy (WR-02: not web-deliverable)', () => {
		// Whatever the server returns, the message never carries a code plaintext — it
		// only ever references how to retrieve it server-side.
		const issued = restoreResultMessage({ restored_count: 1, new_code_issued: true }, label);
		const failed = restoreResultMessage(
			{ restored_count: 1, new_code_issued: false, code_mint_failed: true },
			label
		);
		for (const msg of [issued, failed]) {
			// The copy points at the server-side retrieval path, not an in-band secret.
			expect(msg).toContain('mint-code');
		}
	});
});

describe('eviction preview gating + framing (D-06)', () => {
	// These helpers only read `characters` + `preserved_shared_count`; the
	// owner_id/grace_until fields are placeholders (the EvictionPreview literal
	// must still satisfy the interface field-for-field).
	const cascade: EvictionPreview = {
		owner_id: 7,
		characters: ['Soletoon'],
		grace_until: 1782789805,
		preserved_shared_count: 0
	};
	const allShared: EvictionPreview = {
		owner_id: 8,
		characters: [],
		grace_until: 1782789805,
		preserved_shared_count: 2
	};
	const empty: EvictionPreview = {
		owner_id: 9,
		characters: [],
		grace_until: 1782789805,
		preserved_shared_count: 0
	};

	describe('canEvictPreview — the Evict-button preview-shape gate', () => {
		it('cascade (sole-owned chars to remove) → true', () => {
			expect(canEvictPreview(cascade)).toBe(true);
		});

		it('all-shared (characters:[] but preserved_shared_count>0) → true (code-only revoke)', () => {
			// The BLOCKER fix: an all-shared departing member stays evictable so their
			// guild code can still be revoked even though nothing is removed.
			expect(canEvictPreview(allShared)).toBe(true);
		});

		it('zero live chars (characters:[] AND preserved_shared_count==0) → false (unchanged disable)', () => {
			expect(canEvictPreview(empty)).toBe(false);
		});
	});

	describe('evictPreviewSummary — the three render cases', () => {
		it('cascade → {kind:"cascade"} (the form renders its own char list)', () => {
			expect(evictPreviewSummary(cascade)).toEqual({ kind: 'cascade' });
		});

		it('all-shared → kind:"code-only" with the "0 removed; N preserved; code revoked" framing', () => {
			const s = evictPreviewSummary(allShared);
			expect(s.kind).toBe('code-only');
			if (s.kind !== 'code-only') throw new Error('expected code-only');
			expect(s.message).toContain('0 characters removed');
			// The preserved count is surfaced verbatim.
			expect(s.message).toContain('2');
			expect(s.message.toLowerCase()).toContain('guild code will be revoked');
		});

		it('empty → kind:"empty" with the existing "No characters found" copy', () => {
			const s = evictPreviewSummary(empty);
			expect(s.kind).toBe('empty');
			if (s.kind !== 'empty') throw new Error('expected empty');
			expect(s.message).toContain('No characters found');
		});
	});
});

describe('EvictionForm Restore section is wired (source inspection, close G-1)', () => {
	it('imports + calls restoreEviction and restoreResultMessage', () => {
		expect(EVICTION_FORM_SOURCE).toContain('restoreEviction');
		expect(EVICTION_FORM_SOURCE).toContain('fetchRestorable');
		expect(EVICTION_FORM_SOURCE).toContain('restoreResultMessage');
	});

	it('gates the Restore action behind a ConfirmDialog (confirm-before-commit)', () => {
		expect(EVICTION_FORM_SOURCE).toContain('ConfirmDialog');
		// The restore dialog wires doRestore as its confirm handler.
		expect(EVICTION_FORM_SOURCE).toContain('onConfirm={doRestore}');
		expect(EVICTION_FORM_SOURCE).toContain('Restore evicted guildies');
	});

	it('renders owner labels via Svelte auto-escape, never {@html} (T-15-28)', () => {
		// No raw-HTML directive anywhere in the form — every user/Discord string is {}.
		expect(EVICTION_FORM_SOURCE).not.toContain('{@html');
	});
});
