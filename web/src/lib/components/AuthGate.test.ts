// Vitest for the AuthGate server-truth routing (Plan 15-04 Task 3, B-2).
//
// The repo runs vitest under NODE with no jsdom and no @testing-library/svelte,
// and the vitest config EXCLUDES *.svelte.test.ts. AuthGate is a thin renderer
// over two PURE functions in auth.ts — classifyAuthError (maps a caught typed
// error → an override) and resolveGate (override + session → the screen). This
// test drives that exact pipeline end-to-end the way AuthGate does, plus
// asserts (by source inspection) that AuthGate is actually wired to it
// (fetchSession on mount, catches BOTH typed errors, renders Login/NotMember).
//
// The mid-session contract proven here (B-2 / T-15-25): a 401 (Unauthenticated)
// → LoginScreen; a 403 (Forbidden) with a not-member code → NotMemberScreen; a
// 403 with a not_authorized code → the Officers-only refusal — and the override
// ALWAYS beats the cached (member/officer) session, so the gate never leaves a
// stale authorized view rendered after the server says no.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { Unauthenticated, Forbidden } from '$lib/api';
import { classifyAuthError, resolveGate, ANON, type Session } from '$lib/auth';

const SOURCE = readFileSync(fileURLToPath(new URL('./AuthGate.svelte', import.meta.url)), 'utf8');

/** Reproduce AuthGate's guard pipeline: caught error → override → gate state. */
function guardThenResolve(err: unknown, cached: Session | null) {
	const verdict = classifyAuthError(err);
	// AuthGate drops auth state to ANON on a 401 before applying the override.
	const session = verdict === 'unauthenticated' ? ANON : cached;
	return resolveGate(session, verdict);
}

describe('AuthGate mid-session routing (B-2, server-truth)', () => {
	const member: Session = { ...ANON, authenticated: true, isMember: true };
	const officer: Session = { ...member, isOfficer: true };

	it('a 401 (Unauthenticated) from a descendant call → LoginScreen, dropping a cached member', () => {
		expect(guardThenResolve(new Unauthenticated('401', 401), member)).toBe('login');
	});

	it('a 403 (Forbidden) with a not-member code → NotMemberScreen (not the stale view)', () => {
		expect(guardThenResolve(new Forbidden('403', 403, 'not_member'), member)).toBe('not-member');
	});

	it('a 403 (Forbidden) with a not_authorized code → the Officers-only refusal (officer bit NOT cached)', () => {
		// Even though the cached session says isOfficer, the server's 403 wins.
		expect(guardThenResolve(new Forbidden('403', 403, 'not_authorized'), officer)).toBe(
			'officers-only'
		);
	});

	it('a bare 403 (no code) defaults to the Officers-only refusal', () => {
		expect(guardThenResolve(new Forbidden('403', 403), officer)).toBe('officers-only');
	});

	it('a non-auth error (503) does NOT re-route the gate (caller renders its own error)', () => {
		// classifyAuthError returns null → no override → the member session stands;
		// the +page caller falls through to its generic error StateBlock instead.
		expect(classifyAuthError(new Error('503'))).toBeNull();
		expect(guardThenResolve(new Error('503'), member)).toBe('app');
	});
});

describe('AuthGate wiring (source contract)', () => {
	it('resolves the session via fetchSession on mount', () => {
		expect(SOURCE).toContain('fetchSession');
		expect(SOURCE).toContain('onMount');
	});

	it('catches BOTH typed errors via classifyAuthError (Unauthenticated + Forbidden paths)', () => {
		expect(SOURCE).toContain('classifyAuthError');
		// The guard maps the three verdicts the typed errors classify into.
		expect(SOURCE).toContain("'unauthenticated'");
		expect(SOURCE).toContain("'not_member'");
		expect(SOURCE).toContain("'officer_refused'");
	});

	it('renders LoginScreen and NotMemberScreen (and the officers-only refusal)', () => {
		expect(SOURCE).toContain('LoginScreen');
		expect(SOURCE).toContain('NotMemberScreen');
		expect(SOURCE).toMatch(/kind="officers-only"|officers-only/);
	});

	it('honors an early ?not_member=1 callback hint', () => {
		expect(SOURCE).toContain('not_member=1');
	});

	it('provides the session + guard to descendants via context', () => {
		expect(SOURCE).toContain('setContext');
		expect(SOURCE).toContain('SESSION_KEY');
		expect(SOURCE).toContain('AUTH_GUARD_KEY');
	});
});
