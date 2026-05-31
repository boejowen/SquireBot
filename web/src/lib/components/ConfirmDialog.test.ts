// Vitest for the shared accessible ConfirmDialog (Plan 15-04 Task 2, W-5).
//
// The repo runs vitest under NODE with no jsdom and no @testing-library/svelte
// (the established philosophy — see api.test.ts/auth.test.ts), and the vitest
// config EXCLUDES *.svelte.test.ts. So this proves the W-5 a11y contract two
// ways, both node-runnable:
//   1. The dismiss + focus-trap DECISION logic is split into pure exported
//      helpers (dialogKeyAction / trapTarget) and exercised directly — Escape
//      fires cancel, Tab traps, focus wraps both directions.
//   2. The rendered-markup contract (role="dialog" + aria-modal="true" +
//      aria-labelledby the heading; Cancel — NOT the destructive confirm — is
//      bound as the on-open focus target; the triangle-alert icon carries the
//      destructive signal alongside color; text is escaped via {} never {@html})
//      is asserted by inspecting the component source. A full mounted
//      focus-trap test is a bonus the harness can't host without a DOM.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dialogKeyAction, trapTarget } from './ConfirmDialog.svelte';

const SOURCE = readFileSync(fileURLToPath(new URL('./ConfirmDialog.svelte', import.meta.url)), 'utf8');

describe('ConfirmDialog dismiss logic (W-5)', () => {
	it('Escape while open → cancel (dismiss, no side effect)', () => {
		expect(dialogKeyAction('Escape', true)).toBe('cancel');
	});

	it('Tab while open → trap (focus stays inside the dialog)', () => {
		expect(dialogKeyAction('Tab', true)).toBe('trap');
	});

	it('a non-dismiss key (Enter) → ignore', () => {
		expect(dialogKeyAction('Enter', true)).toBe('ignore');
	});

	it('every key is ignored when the dialog is closed', () => {
		expect(dialogKeyAction('Escape', false)).toBe('ignore');
		expect(dialogKeyAction('Tab', false)).toBe('ignore');
	});
});

describe('ConfirmDialog focus trap (W-5)', () => {
	// Stand-in focusables; identity comparison is all trapTarget needs.
	const a = { id: 'cancel' } as unknown as HTMLElement;
	const b = { id: 'confirm' } as unknown as HTMLElement;
	const items = [a, b];

	it('Tab off the last element wraps to the first', () => {
		expect(trapTarget(items, b, false)).toBe(a);
	});

	it('Shift+Tab off the first element wraps to the last', () => {
		expect(trapTarget(items, a, true)).toBe(b);
	});

	it('no wrap when focus is mid-list', () => {
		// Tab from the first (not the last) → browser default, no forced wrap.
		expect(trapTarget(items, a, false)).toBeNull();
		expect(trapTarget(items, b, true)).toBeNull();
	});

	it('no target when there are no focusables', () => {
		expect(trapTarget([], null, false)).toBeNull();
	});
});

describe('ConfirmDialog rendered-markup a11y contract (W-5)', () => {
	it('the modal is role="dialog" + aria-modal="true"', () => {
		expect(SOURCE).toContain('role="dialog"');
		expect(SOURCE).toContain('aria-modal="true"');
	});

	it('the dialog is labelled by its heading (aria-labelledby → headingId)', () => {
		expect(SOURCE).toContain('aria-labelledby={headingId}');
		expect(SOURCE).toContain('id={headingId}');
	});

	it('the CANCEL button is the on-open focus target (NOT the destructive confirm)', () => {
		// The Cancel <button> binds cancelBtn, and the open $effect focuses cancelBtn.
		expect(SOURCE).toMatch(/class="cd-cancel"[^>]*bind:this=\{cancelBtn\}/);
		expect(SOURCE).toContain('cancelBtn?.focus()');
		// The destructive confirm must NOT be a bound focus target.
		expect(SOURCE).not.toMatch(/class="cd-confirm"[^>]*bind:this/);
	});

	it('focus is restored to the trigger on close', () => {
		expect(SOURCE).toContain('lastFocused');
		expect(SOURCE).toContain('lastFocused.focus()');
	});

	it('Escape dismissal is wired (onkeydown → dialogKeyAction)', () => {
		expect(SOURCE).toContain('onkeydown={onKeydown}');
		expect(SOURCE).toContain('dialogKeyAction(e.key, open)');
	});

	it('a backdrop click cancels (target === currentTarget → onCancel)', () => {
		expect(SOURCE).toContain('onBackdropClick');
		expect(SOURCE).toContain('e.target === e.currentTarget');
	});

	it('color is not the only signal: a triangle-alert icon accompanies the destructive action', () => {
		expect(SOURCE).toContain('TriangleAlert');
		expect(SOURCE).toContain('{confirmLabel}');
	});

	it('all text is escaped via plain {} interpolation — never {@html} on dialog content', () => {
		expect(SOURCE).toContain('{heading}');
		expect(SOURCE).toContain('{body}');
		// The Svelte raw-HTML directive must not appear in the MARKUP. (The
		// explanatory comment legitimately mentions the directive name, so match
		// the actual `{@html` opener, not the bare substring.)
		expect(SOURCE).not.toContain('{@html');
	});
});
