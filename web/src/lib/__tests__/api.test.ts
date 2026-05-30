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
import { fetchView, ApiError } from '../api';

/** Build a minimal Response-like stub for the injected fetchFn. */
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
