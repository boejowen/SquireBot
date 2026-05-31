// Vitest for the session model + the credentialed whoami fetch + the
// server-truth error-classification reducer (Plan 15-04 Task 1, B-2).
//
// auth.ts is intentionally framework-light (pure functions + types) so the
// whole auth contract is exercised here in the node test project WITHOUT
// mounting a Svelte component or needing a DOM (the repo's established test
// philosophy — see api.test.ts). fetchSession takes an injectable fetchFn (the
// same seam api.ts uses); the gate-routing logic lives in pure functions
// (classifyAuthError + resolveGate) that AuthGate.svelte merely renders.

import { describe, it, expect } from 'vitest';
import {
	ANON,
	loginUrl,
	logoutUrl,
	fetchSession,
	logout,
	classifyAuthError,
	resolveGate,
	type Session
} from '../auth';
import { API_BASE, Unauthenticated, Forbidden, ApiError } from '../api';

/** A whoami-web 2xx body for an authenticated member (snake_case, as the backend emits). */
function memberBody(overrides: Record<string, unknown> = {}) {
	return {
		authenticated: true,
		isMember: true,
		isOfficer: false,
		username: 'Slampeach',
		avatar: 'abc123',
		discord_user_id: '424242424242',
		...overrides
	};
}

function okJson(body: unknown): Response {
	return { ok: true, status: 200, json: async () => body } as unknown as Response;
}

describe('auth URLs', () => {
	it('loginUrl / logoutUrl point at the API_BASE auth routes', () => {
		expect(loginUrl()).toBe(`${API_BASE}/api/v1/auth/login`);
		expect(logoutUrl()).toBe(`${API_BASE}/api/v1/auth/logout`);
	});
});

describe('fetchSession (whoami-web → Session)', () => {
	it('maps an authenticated member body into a Session (snake→camel, isMember/isOfficer)', async () => {
		const fetchFn = async () => okJson(memberBody({ isOfficer: true }));
		const s = await fetchSession(fetchFn as unknown as typeof fetch);
		expect(s.authenticated).toBe(true);
		expect(s.isMember).toBe(true);
		expect(s.isOfficer).toBe(true);
		expect(s.username).toBe('Slampeach');
		expect(s.avatar).toBe('abc123');
		expect(s.discordUserId).toBe('424242424242');
	});

	it('returns an unauthenticated Session on {authenticated:false} (no session)', async () => {
		const fetchFn = async () => okJson({ authenticated: false });
		const s = await fetchSession(fetchFn as unknown as typeof fetch);
		expect(s.authenticated).toBe(false);
		expect(s.isMember).toBe(false);
		expect(s.isOfficer).toBe(false);
	});

	it('fail-safes to ANON on a non-2xx whoami (never throws)', async () => {
		const fetchFn = async () => ({ ok: false, status: 500, json: async () => ({}) }) as unknown as Response;
		const s = await fetchSession(fetchFn as unknown as typeof fetch);
		expect(s).toEqual(ANON);
	});

	it('fail-safes to ANON on a thrown/transport fetch error (never throws)', async () => {
		const fetchFn = async () => {
			throw new TypeError('Failed to fetch');
		};
		const s = await fetchSession(fetchFn as unknown as typeof fetch);
		expect(s).toEqual(ANON);
	});

	it('sends credentials:"include" to whoami-web (cross-subdomain cookie)', async () => {
		let seen: RequestInit | undefined;
		const fetchFn = (async (_url: string, init?: RequestInit) => {
			seen = init;
			return okJson(memberBody());
		}) as unknown as typeof fetch;
		await fetchSession(fetchFn);
		expect(seen?.credentials).toBe('include');
	});

	it('hits the whoami-web endpoint', async () => {
		let url = '';
		const fetchFn = (async (u: string) => {
			url = u;
			return okJson(memberBody());
		}) as unknown as typeof fetch;
		await fetchSession(fetchFn);
		expect(url).toBe(`${API_BASE}/api/v1/auth/whoami-web`);
	});
});

describe('logout', () => {
	it('POSTs the logout URL with credentials:"include"', async () => {
		let seen: { url: string; init?: RequestInit } | undefined;
		const fetchFn = (async (url: string, init?: RequestInit) => {
			seen = { url, init };
			return { ok: true, status: 204, json: async () => ({}) } as unknown as Response;
		}) as unknown as typeof fetch;
		await logout(fetchFn);
		expect(seen?.url).toBe(logoutUrl());
		expect(seen?.init?.method).toBe('POST');
		expect(seen?.init?.credentials).toBe('include');
	});
});

describe('classifyAuthError (server-truth, B-2)', () => {
	it('maps Unauthenticated → "unauthenticated"', () => {
		expect(classifyAuthError(new Unauthenticated('x', 401))).toBe('unauthenticated');
	});

	it('maps Forbidden with a not-member code → "not_member"', () => {
		expect(classifyAuthError(new Forbidden('x', 403, 'not_member'))).toBe('not_member');
	});

	it('maps Forbidden with a not_authorized code → "officer_refused"', () => {
		expect(classifyAuthError(new Forbidden('x', 403, 'not_authorized'))).toBe('officer_refused');
	});

	it('maps a bare Forbidden (no code) → "officer_refused" (default officer gate)', () => {
		expect(classifyAuthError(new Forbidden('x', 403))).toBe('officer_refused');
	});

	it('returns null for a non-auth error (a generic ApiError / 503 stays a normal error)', () => {
		expect(classifyAuthError(new ApiError('boom', 503))).toBeNull();
		expect(classifyAuthError(new Error('nope'))).toBeNull();
	});
});

describe('resolveGate (the pure routing reducer AuthGate renders)', () => {
	const member: Session = { ...ANON, authenticated: true, isMember: true };
	const officer: Session = { ...member, isOfficer: true };
	const nonMember: Session = { ...ANON, authenticated: true, isMember: false };

	it('null session (still loading) → auth-loading', () => {
		expect(resolveGate(null, null)).toBe('auth-loading');
	});

	it('unauthenticated session → login', () => {
		expect(resolveGate(ANON, null)).toBe('login');
	});

	it('authenticated non-member → not-member', () => {
		expect(resolveGate(nonMember, null)).toBe('not-member');
	});

	it('authenticated member → app', () => {
		expect(resolveGate(member, null)).toBe('app');
		expect(resolveGate(officer, null)).toBe('app');
	});

	// Server-truth overrides win over any cached session state (B-2 / T-15-25):
	it('an "unauthenticated" override forces login even with a member session cached', () => {
		expect(resolveGate(member, 'unauthenticated')).toBe('login');
	});

	it('a "not_member" override forces not-member even with a member session cached', () => {
		expect(resolveGate(member, 'not_member')).toBe('not-member');
	});

	it('an "officer_refused" override forces officers-only (never the stale authorized view)', () => {
		expect(resolveGate(officer, 'officer_refused')).toBe('officers-only');
	});
});
