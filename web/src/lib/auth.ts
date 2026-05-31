// Session model + the credentialed whoami fetch + the server-truth routing
// reducer (Plan 15-04 Task 1, AUTH-08/09, B-2).
//
// This module is deliberately framework-light — pure functions + types, no
// Svelte runes — so the whole auth contract is unit-testable in the node test
// project (the repo's established philosophy; see api.test.ts). AuthGate.svelte
// is a thin renderer over `resolveGate`; it owns the reactive `$state` and
// merely maps the reducer's verdict to a screen.
//
// Auth state is SERVER-TRUTH (15-UI-SPEC § Accessibility "auth state is
// server-truth"): the frontend gate is UX only — the API (15-02/15-03) is the
// real gate. fetchSession FAIL-SAFES to logged-out (never throws) so a flaky
// whoami can only ever under-privilege, never over-privilege; and the typed
// errors from api.ts (Unauthenticated/Forbidden) drive mid-session re-routing.

import { API_BASE, Unauthenticated, Forbidden } from './api';

/**
 * The authenticated identity the whole app gates on. Mirrors the 15-02
 * whoami-web shape {authenticated,isMember,isOfficer,username,avatar,
 * discord_user_id} (snake→camel for discordUserId). `avatar` is null when the
 * Discord user has no custom avatar (→ a glyph fallback).
 */
export interface Session {
	authenticated: boolean;
	isMember: boolean;
	isOfficer: boolean;
	username: string;
	avatar: string | null;
	discordUserId: string;
}

/** The logged-out session. fetchSession resolves to this on any failure (fail-safe). */
export const ANON: Session = {
	authenticated: false,
	isMember: false,
	isOfficer: false,
	username: '',
	avatar: null,
	discordUserId: ''
};

/** The /api/v1/auth route group (15-02). */
const AUTH_BASE = `${API_BASE}/api/v1/auth`;

/** The Discord OAuth start route — the browser NAVIGATES here (not fetch). */
export function loginUrl(): string {
	return `${AUTH_BASE}/login`;
}

/** The session-destroy route (POST, then navigate to `/`). */
export function logoutUrl(): string {
	return `${AUTH_BASE}/logout`;
}

/** The whoami-web session-resolution endpoint (always 200; authenticated:false when no session). */
function whoamiUrl(): string {
	return `${AUTH_BASE}/whoami-web`;
}

/** The raw whoami-web JSON shape (snake_case for discord_user_id, as the backend emits). */
interface WhoamiBody {
	authenticated?: boolean;
	isMember?: boolean;
	isOfficer?: boolean;
	username?: string;
	avatar?: string | null;
	discord_user_id?: string;
}

/**
 * Resolve the current session via GET whoami-web (credentialed so the cookie
 * rides cross-subdomain). On a 2xx, map the JSON into a Session (missing fields
 * → ANON defaults). On ANY error/non-2xx, FAIL-SAFE to ANON — never throw: the
 * gate treats "can't resolve" as unauthenticated, so a flaky whoami can only
 * under-privilege.
 */
export async function fetchSession(fetchFn: typeof fetch = fetch): Promise<Session> {
	try {
		const res = await fetchFn(whoamiUrl(), {
			method: 'GET',
			headers: { Accept: 'application/json' },
			credentials: 'include'
		});
		if (!res.ok) return ANON;
		const body = (await res.json()) as WhoamiBody;
		if (!body?.authenticated) return ANON;
		return {
			authenticated: true,
			isMember: body.isMember ?? false,
			isOfficer: body.isOfficer ?? false,
			username: body.username ?? '',
			avatar: body.avatar ?? null,
			discordUserId: body.discord_user_id ?? ''
		};
	} catch {
		return ANON;
	}
}

/**
 * POST the logout route (credentialed). The caller navigates to `/` afterward;
 * a failed logout is swallowed (the cookie is httpOnly + bounded TTL anyway).
 */
export async function logout(fetchFn: typeof fetch = fetch): Promise<void> {
	try {
		await fetchFn(logoutUrl(), { method: 'POST', credentials: 'include' });
	} catch {
		// Swallow — the redirect to `/` re-runs the gate, which fail-safes to ANON.
	}
}

/** The override the gate applies after a server says-no, keyed to the matching refusal screen. */
export type AuthOverride = 'unauthenticated' | 'not_member' | 'officer_refused';

/**
 * Classify a caught error into the server-truth override the gate routes on
 * (B-2). Unauthenticated (401) → drop auth → LoginScreen. Forbidden (403) → the
 * MATCHING refusal: a not-member code → NotMemberScreen, else the Officers-only
 * refusal (a bare 403 with no code defaults to the officer gate — the only
 * officer-only surface that 403s without a member-specific code). A non-auth
 * error returns null (it stays a normal error, not an auth re-route).
 */
export function classifyAuthError(err: unknown): AuthOverride | null {
	if (err instanceof Unauthenticated) return 'unauthenticated';
	if (err instanceof Forbidden) {
		const code = (err.code ?? '').toLowerCase();
		// Codes the backend uses for a non-member refusal vs an officer refusal.
		if (code === 'not_member' || code === 'not_a_member' || code === 'forbidden_not_member') {
			return 'not_member';
		}
		return 'officer_refused';
	}
	return null;
}

/** The screen the gate renders. */
export type GateState = 'auth-loading' | 'login' | 'not-member' | 'officers-only' | 'app';

/**
 * The PURE routing reducer AuthGate renders (B-2 / 15-UI-SPEC resolution
 * order). A server-truth `override` (from classifyAuthError on a mid-session
 * 401/403, or an early `?not_member=1` hint) ALWAYS wins over the cached
 * session — so the app never leaves a stale authorized view rendered after the
 * server says no, and never trusts a cached "I'm an officer" bit past a 403.
 *
 * Precedence:
 *   override 'unauthenticated'  → login
 *   override 'not_member'       → not-member
 *   override 'officer_refused'  → officers-only
 *   session === null (loading)  → auth-loading
 *   !session.authenticated      → login
 *   authenticated && !isMember  → not-member
 *   authenticated && isMember   → app
 */
export function resolveGate(session: Session | null, override: AuthOverride | null): GateState {
	if (override === 'unauthenticated') return 'login';
	if (override === 'not_member') return 'not-member';
	if (override === 'officer_refused') return 'officers-only';
	if (session === null) return 'auth-loading';
	if (!session.authenticated) return 'login';
	if (!session.isMember) return 'not-member';
	return 'app';
}
