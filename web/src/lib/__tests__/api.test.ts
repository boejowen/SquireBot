// Vitest for the typed fetch wrappers (Plan 14-04 Task 1) — proves the ApiError
// contract holds on every failure mode getJSON can hit. getJSON is internal, so
// it's exercised through fetchView(fetchFn) which injects a stub fetch (the
// test seam every wrapper already exposes).
//
// Review WR-04: a 2xx response whose body is NOT valid JSON (an interposing
// proxy/Cloudflare error page served with 200, or an empty body) must surface
// as a *branded* ApiError carrying the real HTTP status — never a raw
// SyntaxError that escapes the contract the UI error state depends on.

import { describe, it, expect } from 'vitest';
import {
	fetchView,
	ApiError,
	Unauthenticated,
	Forbidden,
	fetchOwnCodes,
	mintOwnCode,
	revokeOwnCode,
	type OwnCode
} from '../api';

/** Build a minimal Response-like stub for the injected fetchFn. */
function jsonResponse(ok: boolean, status: number, body: unknown): Response {
	return {
		ok,
		status,
		json: async () => body
	} as unknown as Response;
}

/**
 * Capture the RequestInit the wrapper passes to fetch, so a test can assert the
 * credentialed-fetch contract (15-04 B-2 / D-05 cross-subdomain cookie).
 */
function capturingFetch(body: unknown): { fetchFn: typeof fetch; init: () => RequestInit | undefined } {
	let seen: RequestInit | undefined;
	const fetchFn = (async (_url: string, init?: RequestInit) => {
		seen = init;
		return jsonResponse(true, 200, body);
	}) as unknown as typeof fetch;
	return { fetchFn, init: () => seen };
}

/** A 2xx Response whose .json() rejects, mimicking a malformed/empty body. */
function malformedJsonResponse(status: number): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		json: async () => {
			throw new SyntaxError('Unexpected end of JSON input');
		}
	} as unknown as Response;
}

describe('getJSON ApiError contract (via fetchView)', () => {
	it('resolves the parsed JSON on a well-formed 2xx body', async () => {
		const rows = [{ char: 'Alpha', item: 'Cloak' }];
		const fetchFn = async () => jsonResponse(true, 200, rows);
		await expect(fetchView(fetchFn as unknown as typeof fetch)).resolves.toEqual(rows);
	});

	it('throws a branded ApiError carrying the status on a non-2xx response', async () => {
		const fetchFn = async () => jsonResponse(false, 503, null);
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as ApiError).status).toBe(503);
	});

	it('throws a branded ApiError with status 0 on a transport failure', async () => {
		const fetchFn = async () => {
			throw new TypeError('Failed to fetch');
		};
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as ApiError).status).toBe(0);
	});

	it('throws a branded ApiError (NOT a raw SyntaxError) on a malformed 2xx body (WR-04)', async () => {
		const fetchFn = async () => malformedJsonResponse(200);
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		// The fix turns the SyntaxError into an ApiError so callers that switch on
		// ApiError.status (e.g. a future 401 handler) keep their classification.
		expect(err).toBeInstanceOf(ApiError);
		expect(err).not.toBeInstanceOf(SyntaxError);
		expect((err as ApiError).status).toBe(200);
	});
});

describe('15-04 B-2: typed auth errors + credentialed fetch', () => {
	it('sends credentials:"include" on every call (cross-subdomain cookie, D-05)', async () => {
		const cap = capturingFetch([]);
		await fetchView(cap.fetchFn);
		expect(cap.init()?.credentials).toBe('include');
	});

	it('maps a 401 response to a typed Unauthenticated error (instanceof both Unauthenticated and ApiError)', async () => {
		const fetchFn = async () => jsonResponse(false, 401, { error: 'unauthenticated' });
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(Unauthenticated);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as Unauthenticated).status).toBe(401);
	});

	it('maps a 403 response to a typed Forbidden error carrying the server {error} code', async () => {
		const fetchFn = async () => jsonResponse(false, 403, { error: 'not_authorized' });
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(Forbidden);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as Forbidden).status).toBe(403);
		expect((err as Forbidden).code).toBe('not_authorized');
	});

	it('Forbidden carries a not_member code so the gate can route to NotMemberScreen', async () => {
		const fetchFn = async () => jsonResponse(false, 403, { error: 'not_member' });
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(Forbidden);
		expect((err as Forbidden).code).toBe('not_member');
	});

	it('still throws a plain ApiError (not Unauthenticated/Forbidden) on other non-2xx (e.g. 503)', async () => {
		const fetchFn = async () => jsonResponse(false, 503, null);
		const err = await fetchView(fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect(err).not.toBeInstanceOf(Unauthenticated);
		expect(err).not.toBeInstanceOf(Forbidden);
	});
});

// --- 17-03: self-service watcher-code wrappers ---------------------------
// Prove each wrapper hits the right path + method, that mint POSTs an EMPTY body
// (the owner is session-derived server-side, D-02 — never sent), that revoke
// POSTs {id}, and that all three ride the credentialed-fetch contract. The
// wrappers reuse getJSON/postJSON, so the typed-error mapping is already covered
// by the suites above; these tests pin the request shape.

/** Capture the URL + RequestInit each wrapper passes to fetch, returning `body`. */
function recordingFetch(body: unknown): {
	fetchFn: typeof fetch;
	url: () => string | undefined;
	init: () => RequestInit | undefined;
} {
	let seenUrl: string | undefined;
	let seenInit: RequestInit | undefined;
	const fetchFn = (async (url: string, init?: RequestInit) => {
		seenUrl = url;
		seenInit = init;
		return jsonResponse(true, 200, body);
	}) as unknown as typeof fetch;
	return { fetchFn, url: () => seenUrl, init: () => seenInit };
}

describe('17-03 account-code wrappers', () => {
	it('fetchOwnCodes GETs /api/v1/account/codes with credentials and resolves the array', async () => {
		const rows: OwnCode[] = [
			{ id: 7, ordinal: 1, created_at: '2026-05-12T00:00:00Z', last_seen: null }
		];
		const rec = recordingFetch(rows);
		await expect(fetchOwnCodes(rec.fetchFn)).resolves.toEqual(rows);
		expect(rec.url()).toMatch(/\/api\/v1\/account\/codes$/);
		expect(rec.init()?.method).toBe('GET');
		expect(rec.init()?.credentials).toBe('include');
	});

	it('fetchOwnCodes returns [] when the caller has never minted', async () => {
		const rec = recordingFetch([]);
		await expect(fetchOwnCodes(rec.fetchFn)).resolves.toEqual([]);
	});

	it('mintOwnCode POSTs /api/v1/account/codes with an EMPTY body (owner is session-derived, D-02)', async () => {
		const rec = recordingFetch({ code: 'plaintext-token-xyz' });
		await expect(mintOwnCode(rec.fetchFn)).resolves.toEqual({ code: 'plaintext-token-xyz' });
		expect(rec.url()).toMatch(/\/api\/v1\/account\/codes$/);
		expect(rec.init()?.method).toBe('POST');
		expect(rec.init()?.credentials).toBe('include');
		// The body MUST be `{}` — no owner ever rides the request (D-02).
		expect(JSON.parse(rec.init()?.body as string)).toEqual({});
	});

	it('revokeOwnCode POSTs /api/v1/account/codes/revoke with the {id} body', async () => {
		const rec = recordingFetch({ revoked: true });
		await expect(revokeOwnCode(42, rec.fetchFn)).resolves.toEqual({ revoked: true });
		expect(rec.url()).toMatch(/\/api\/v1\/account\/codes\/revoke$/);
		expect(rec.init()?.method).toBe('POST');
		expect(rec.init()?.credentials).toBe('include');
		expect(JSON.parse(rec.init()?.body as string)).toEqual({ id: 42 });
	});

	it('revokeOwnCode surfaces { revoked: false } as a resolved no-op (not a throw)', async () => {
		const rec = recordingFetch({ revoked: false });
		await expect(revokeOwnCode(99, rec.fetchFn)).resolves.toEqual({ revoked: false });
	});
});
