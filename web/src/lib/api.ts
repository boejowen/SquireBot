// Typed fetch wrappers for the Go read API (BACKEND-05 consumption side).
// Plan 14-04 Task 1. The endpoint JSON contract is PINNED by Plan 14-03
// (14-03-SUMMARY.md "THE PINNED ENDPOINT JSON CONTRACT") — the row interfaces
// below mirror its exact snake_case field names.
//
// Base origin is the API host https://api.squirebot.quest (distinct from the
// static-site origin https://squirebot.quest). It is configurable via the
// PUBLIC_API_BASE env var (SvelteKit $env/dynamic/public) so a staging/preview
// API is an env change, not a recompile; the trailing slash (if any) is
// trimmed so `${API_BASE}/api/v1/...` always joins cleanly.
//
// Every wrapper throws an `ApiError` on a non-2xx response or a network
// failure; the +page.svelte caller catches it and renders the error StateBlock
// (UI-SPEC "Couldn't load the data" + Retry).

import { env } from '$env/dynamic/public';

const DEFAULT_API_BASE = 'https://api.squirebot.quest';

/** The API origin, trailing-slash-trimmed. PUBLIC_API_BASE overrides the default. */
export const API_BASE: string = (env.PUBLIC_API_BASE || DEFAULT_API_BASE).replace(/\/+$/, '');

/** Thrown on any non-2xx response or network/transport failure. */
export class ApiError extends Error {
	readonly status: number;
	/** The server's `{"error":"<code>"}` value when present (auth errors carry it). */
	readonly code?: string;
	constructor(message: string, status: number, code?: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
	}
}

// --- Typed auth errors (15-04 B-2, server-truth) -------------------------
// The read API is session-gated (15-02): a missing/expired session → 401, a
// member hitting an officer-only write → 403. AuthGate (15-04) catches these
// SUBCLASSES on any descendant call and re-routes (401→LoginScreen,
// 403→NotMember/Officers-only) so the user never sits on a stale authorized
// view after the server has said no. Both are instanceof-checkable and also
// instanceof ApiError, so existing `catch (e) { if (e instanceof ApiError) }`
// sites keep working.

/** A 401 from the API — no/expired session. The gate drops auth state → LoginScreen. */
export class Unauthenticated extends ApiError {
	constructor(message: string, status = 401, code?: string) {
		super(message, status, code);
		this.name = 'Unauthenticated';
	}
}

/** A 403 from the API — authenticated but refused (not a member / not an officer). */
export class Forbidden extends ApiError {
	constructor(message: string, status = 403, code?: string) {
		super(message, status, code);
		this.name = 'Forbidden';
	}
}

/**
 * Best-effort parse of a `{"error":"<code>"}` body WITHOUT consuming the stream
 * twice or throwing. Returns the code string or undefined. Used to attach the
 * server's discriminator (`not_member` vs `not_authorized`) to the typed 403 so
 * the gate can pick the matching refusal.
 */
async function readErrorCode(res: Response): Promise<string | undefined> {
	try {
		const body = (await res.json()) as { error?: unknown };
		return typeof body?.error === 'string' ? body.error : undefined;
	} catch {
		return undefined;
	}
}

// --- Row contract (snake_case, per Plan 14-03 SUMMARY) -------------------

/** A single PigParse price detail. `direction` is TEXT "0"=WTS / "1"=WTB / "2"=BOTH. */
export interface PriceDetail {
	direction: '0' | '1' | '2';
	a30: number;
	t30: number;
}

/** A quest-link entry. `source` is "in_game_flag" | "notes_link". */
export interface QuestLink {
	quest_name: string;
	source: 'in_game_flag' | 'notes_link';
}

/** An inventory/bank view row (`/api/v1/views/view` element; bank `rows[]` element). */
export interface ViewRow {
	char: string;
	slot: string;
	item: string;
	id: number;
	count: number;
	wiki_url: string;
	/** null when neither WTS nor WTB has a30>0 — client renders Price blank. */
	price: number | null;
	/** ISO timestamp (character.last_seen); freshness coloring is client-side. */
	last_synced: string;
	// Tooltip enrichment (inline, D-03 — no second fetch).
	wiki_summary: string;
	is_quest_item: boolean;
	prices: PriceDetail[];
	quest_links: QuestLink[];
}

/** A gear_check row. `status` is "OK" | "OTHER" | "MISSING". */
export interface GearCheckRow {
	char: string;
	class: string;
	tier: string;
	slot: string;
	have: string;
	recommended: string;
	status: 'OK' | 'OTHER' | 'MISSING';
}

/** A spell_check row. `status` is "KNOWN" | "MISSING". */
export interface SpellCheckRow {
	char: string;
	class: string;
	level: number;
	spell: string;
	status: 'KNOWN' | 'MISSING';
}

/** The bank endpoint object: `{ rows, coin }`. `coin` is always null in P14. */
export interface BankResponse {
	rows: ViewRow[];
	coin: null;
}

/** A meta character entry. `last_seen` is "" when never seen. */
export interface MetaCharacter {
	name: string;
	last_seen: string;
}

/** The meta endpoint object: `{ characters }`. */
export interface MetaResponse {
	characters: MetaCharacter[];
}

// --- Fetch core ----------------------------------------------------------

/**
 * GET `${API_BASE}${path}` and decode JSON. Throws `ApiError` on a non-2xx
 * status or any transport failure (the caller renders the error StateBlock).
 * `fetchFn` defaults to the global `fetch` but is injectable for tests.
 */
async function getJSON<T>(path: string, fetchFn: typeof fetch = fetch): Promise<T> {
	let res: Response;
	try {
		res = await fetchFn(`${API_BASE}${path}`, {
			method: 'GET',
			headers: { Accept: 'application/json' },
			// D-05 / 15-02: the session cookie is Domain=squirebot.quest and rides
			// cross-subdomain to api.squirebot.quest ONLY when the request is
			// credentialed; the API's CORS sends Access-Control-Allow-Credentials.
			credentials: 'include'
		});
	} catch (cause) {
		// Network / CORS / DNS failure — no HTTP status available.
		throw new ApiError(`network error fetching ${path}`, 0);
	}
	if (!res.ok) {
		// 15-04 B-2 server-truth: classify the two auth statuses into typed
		// subclasses the AuthGate catches and re-routes on (401→Login,
		// 403→matching refusal). The {error} code rides along so the gate can
		// distinguish not-member from not-officer. Any other non-2xx stays a
		// plain ApiError (the generic error StateBlock).
		if (res.status === 401) {
			const code = await readErrorCode(res);
			throw new Unauthenticated(`unauthenticated fetching ${path}`, 401, code);
		}
		if (res.status === 403) {
			const code = await readErrorCode(res);
			throw new Forbidden(`forbidden fetching ${path}`, 403, code);
		}
		throw new ApiError(`unexpected ${res.status} fetching ${path}`, res.status);
	}
	// A 2xx body that isn't valid JSON (e.g. an interposing proxy/Cloudflare
	// error page served with 200, or an empty body) makes res.json() reject with
	// a raw SyntaxError that is NOT an ApiError — it would escape the fetch
	// try/catch above and break the ApiError contract every caller relies on
	// (status classification, the +page.svelte error state). Wrap the parse so a
	// malformed body surfaces as a branded ApiError carrying the real status.
	try {
		return (await res.json()) as T;
	} catch {
		throw new ApiError(`malformed JSON from ${path}`, res.status);
	}
}

// --- Public wrappers (one per pinned endpoint) ---------------------------

/** GET /api/v1/views/view → ViewRow[] ([] when empty). */
export function fetchView(fetchFn: typeof fetch = fetch): Promise<ViewRow[]> {
	return getJSON<ViewRow[]>('/api/v1/views/view', fetchFn);
}

/** GET /api/v1/views/gear_check → GearCheckRow[] ([] when empty). */
export function fetchGearCheck(fetchFn: typeof fetch = fetch): Promise<GearCheckRow[]> {
	return getJSON<GearCheckRow[]>('/api/v1/views/gear_check', fetchFn);
}

/** GET /api/v1/views/spell_check → SpellCheckRow[] ([] when empty). */
export function fetchSpellCheck(fetchFn: typeof fetch = fetch): Promise<SpellCheckRow[]> {
	return getJSON<SpellCheckRow[]>('/api/v1/views/spell_check', fetchFn);
}

/** GET /api/v1/views/bank → { rows, coin: null }. */
export function fetchBank(fetchFn: typeof fetch = fetch): Promise<BankResponse> {
	return getJSON<BankResponse>('/api/v1/views/bank', fetchFn);
}

/** GET /api/v1/meta → { characters: [{ name, last_seen }] }. */
export function fetchMeta(fetchFn: typeof fetch = fetch): Promise<MetaResponse> {
	return getJSON<MetaResponse>('/api/v1/meta', fetchFn);
}
