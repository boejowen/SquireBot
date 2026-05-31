// Vitest for the admin/coin write wrappers + the shared postJSON core (Plan
// 15-05 Task 1). Proves the credentialed-POST contract + the typed 401/403
// mapping that the three forms (BankCoinForm / EvictionForm / AdminMgmtForm)
// branch on. postJSON is internal, so it's exercised through the typed wrappers
// that inject a stub fetch (the same test seam api.test.ts established).
//
// B-2 cross-ref (15-05): a 403 owner_floor_protected AND a 403 not_authorized
// must EACH surface as a Forbidden carrying the matching .code — the eviction +
// admin-mgmt forms route on exactly that distinction (owner_floor_protected →
// inline floor message; not_authorized → authGuard → Officers-only refusal).

import { describe, it, expect } from 'vitest';
import {
	saveCoin,
	removeOfficer,
	addOfficer,
	fetchBankToons,
	fetchOfficers,
	evict,
	ApiError,
	Unauthenticated,
	Forbidden,
	classifyAdminError
} from '../api';

/** A minimal Response-like stub whose .json() resolves to `body`. */
function jsonResponse(ok: boolean, status: number, body: unknown): Response {
	return {
		ok,
		status,
		json: async () => body
	} as unknown as Response;
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

/**
 * Capture the (url, init) the wrapper passes to fetch so a test can assert the
 * method + credentials + JSON body contract. Resolves the POST to `body`.
 */
function capturingFetch(body: unknown): {
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

describe('postJSON contract (via saveCoin / removeOfficer)', () => {
	it('POSTs credentials:"include" + a JSON content-type + the stringified body', async () => {
		const cap = capturingFetch({ character: 'Banker', coin: { plat: 5, gold: 0, silver: 0, copper: 0 } });
		await saveCoin({ character_id: 7, plat: 5, gold: 0, silver: 0, copper: 0 }, cap.fetchFn);
		const init = cap.init();
		expect(init?.method).toBe('POST');
		expect(init?.credentials).toBe('include');
		expect((init?.headers as Record<string, string>)['Content-Type']).toBe('application/json');
		// The body is the JSON-stringified payload (snake_case, per 15-03).
		expect(JSON.parse(init?.body as string)).toEqual({
			character_id: 7,
			plat: 5,
			gold: 0,
			silver: 0,
			copper: 0
		});
		expect(cap.url()).toContain('/api/v1/coin');
	});

	it('resolves the parsed JSON on a 2xx', async () => {
		const result = { character: 'Banker', coin: { plat: 5, gold: 0, silver: 0, copper: 0 } };
		const fetchFn = async () => jsonResponse(true, 200, result);
		await expect(
			saveCoin({ character_id: 7, plat: 5, gold: 0, silver: 0, copper: 0 }, fetchFn as unknown as typeof fetch)
		).resolves.toEqual(result);
	});

	it('throws a branded ApiError (NOT a raw SyntaxError) on a malformed 2xx body', async () => {
		const fetchFn = async () => malformedJsonResponse(200);
		const err = await saveCoin(
			{ character_id: 7, plat: 5, gold: 0, silver: 0, copper: 0 },
			fetchFn as unknown as typeof fetch
		).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect(err).not.toBeInstanceOf(SyntaxError);
		expect((err as ApiError).status).toBe(200);
	});

	it('throws a branded ApiError with status 0 on a transport failure', async () => {
		const fetchFn = async () => {
			throw new TypeError('Failed to fetch');
		};
		const err = await addOfficer('123', fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as ApiError).status).toBe(0);
	});
});

describe('15-05 B-2: a write 403/401/400 surfaces as the typed error carrying the server code', () => {
	it('saveCoin: a 400 {"error":"not_bank_toon"} → ApiError with code="not_bank_toon"', async () => {
		const fetchFn = async () => jsonResponse(false, 400, { error: 'not_bank_toon' });
		const err = await saveCoin(
			{ character_id: 7, plat: 0, gold: 0, silver: 0, copper: 0 },
			fetchFn as unknown as typeof fetch
		).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as ApiError).status).toBe(400);
		expect((err as ApiError).code).toBe('not_bank_toon');
	});

	it('saveCoin: a 400 {"error":"invalid_input"} → ApiError with code="invalid_input"', async () => {
		const fetchFn = async () => jsonResponse(false, 400, { error: 'invalid_input' });
		const err = await saveCoin(
			{ character_id: 7, plat: -1, gold: 0, silver: 0, copper: 0 },
			fetchFn as unknown as typeof fetch
		).catch((e) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as ApiError).code).toBe('invalid_input');
	});

	it('saveCoin: a 401 → Unauthenticated (instanceof ApiError too)', async () => {
		const fetchFn = async () => jsonResponse(false, 401, { error: 'unauthenticated' });
		const err = await saveCoin(
			{ character_id: 7, plat: 0, gold: 0, silver: 0, copper: 0 },
			fetchFn as unknown as typeof fetch
		).catch((e) => e);
		expect(err).toBeInstanceOf(Unauthenticated);
		expect(err).toBeInstanceOf(ApiError);
	});

	it('removeOfficer: a 403 {"error":"owner_floor_protected"} → Forbidden with code="owner_floor_protected"', async () => {
		const fetchFn = async () => jsonResponse(false, 403, { error: 'owner_floor_protected' });
		const err = await removeOfficer('999', fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(Forbidden);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as Forbidden).status).toBe(403);
		expect((err as Forbidden).code).toBe('owner_floor_protected');
	});

	it('removeOfficer: a 403 {"error":"not_authorized"} → Forbidden with code="not_authorized"', async () => {
		const fetchFn = async () => jsonResponse(false, 403, { error: 'not_authorized' });
		const err = await removeOfficer('123', fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(Forbidden);
		expect((err as Forbidden).code).toBe('not_authorized');
	});

	it('evict: a 403 {"error":"not_authorized"} → Forbidden with code="not_authorized"', async () => {
		const fetchFn = async () => jsonResponse(false, 403, { error: 'not_authorized' });
		const err = await evict(4, fetchFn as unknown as typeof fetch).catch((e) => e);
		expect(err).toBeInstanceOf(Forbidden);
		expect((err as Forbidden).code).toBe('not_authorized');
	});
});

describe('GET wrappers (fetchBankToons / fetchOfficers) send credentials + hit the right path', () => {
	it('fetchBankToons GETs /api/v1/coin/bank-toons credentialed', async () => {
		const cap = capturingFetch([]);
		await fetchBankToons(cap.fetchFn);
		expect(cap.url()).toContain('/api/v1/coin/bank-toons');
		expect(cap.init()?.credentials).toBe('include');
		// A GET wrapper must NOT carry a POST body.
		expect(cap.init()?.method ?? 'GET').toBe('GET');
	});

	it('fetchOfficers GETs /api/v1/admin/officers credentialed', async () => {
		const cap = capturingFetch({ officers: [], promotable: [] });
		await fetchOfficers(cap.fetchFn);
		expect(cap.url()).toContain('/api/v1/admin/officers');
		expect(cap.init()?.credentials).toBe('include');
	});
});

describe('classifyAdminError — the forms’ server-truth routing helper (B-2 cross-ref)', () => {
	// This is the PURE decision helper the EvictionForm + AdminMgmtForm catch
	// paths use: it maps a caught write error to one of
	//   'officers-only'  → collapse the whole admin UI via authGuard (the
	//                       "you're no longer an officer" path; NOT a form-error)
	//   'owner-floor'     → the inline owner-floor protection copy
	//   'lock-busy'       → the inline lock-busy retry copy
	//   'unauthenticated' → let it bubble to the AuthGate (→ LoginScreen)
	//   'generic'         → a normal inline form-error
	// Proving it here keeps the 403→officers-only contract node-testable without
	// mounting a component (the repo's node-only philosophy, 15-04).

	it('Forbidden(not_authorized) → "officers-only" (collapse to the refusal, not a generic error)', () => {
		expect(classifyAdminError(new Forbidden('x', 403, 'not_authorized'))).toBe('officers-only');
	});

	it('a bare Forbidden (no code) → "officers-only" (defaults to the officer gate)', () => {
		expect(classifyAdminError(new Forbidden('x', 403))).toBe('officers-only');
	});

	it('Forbidden(owner_floor_protected) → "owner-floor" (inline floor message)', () => {
		expect(classifyAdminError(new Forbidden('x', 403, 'owner_floor_protected'))).toBe('owner-floor');
	});

	it('Forbidden(lock_busy) → "lock-busy" (inline retry message)', () => {
		expect(classifyAdminError(new Forbidden('x', 403, 'lock_busy'))).toBe('lock-busy');
	});

	it('Unauthenticated → "unauthenticated" (bubbles to AuthGate → LoginScreen)', () => {
		expect(classifyAdminError(new Unauthenticated('x', 401))).toBe('unauthenticated');
	});

	it('a plain ApiError (e.g. 500) → "generic" (a normal inline form-error)', () => {
		expect(classifyAdminError(new ApiError('x', 500))).toBe('generic');
	});

	it('a non-ApiError (e.g. a TypeError) → "generic"', () => {
		expect(classifyAdminError(new TypeError('boom'))).toBe('generic');
	});
});
