// Typed fetch wrappers for the Go read API (BACKEND-05 consumption side).
// Plan 14-04 Task 1. The endpoint JSON contract is PINNED by Plan 14-03
// (14-03-SUMMARY.md "THE PINNED ENDPOINT JSON CONTRACT") — the row interfaces
// below mirror its exact snake_case field names.
//
// Base origin is the API host https://api.squirebot.quest (distinct from the
// static-site origin app.squirebot.quest). It is configurable via the
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
	constructor(message: string, status: number) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
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
			headers: { Accept: 'application/json' }
		});
	} catch (cause) {
		// Network / CORS / DNS failure — no HTTP status available.
		throw new ApiError(`network error fetching ${path}`, 0);
	}
	if (!res.ok) {
		throw new ApiError(`unexpected ${res.status} fetching ${path}`, res.status);
	}
	return (await res.json()) as T;
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
