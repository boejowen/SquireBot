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

// --- Write core: postJSON (15-05 Task 1) ---------------------------------
// The mutating sibling of getJSON for the three write forms (eviction / coin /
// officer-mgmt). It carries the SAME credential + typed-error contract so a 403
// owner_floor_protected surfaces as a Forbidden with .code='owner_floor_protected'
// (the eviction/admin forms branch on that code, B-2). Identical status mapping
// to getJSON: 401 → Unauthenticated, 403 → Forbidden(code), other non-2xx →
// ApiError(status, code), transport failure → ApiError(0), malformed 2xx body →
// branded ApiError. The {error} code rides along on EVERY non-2xx (not just
// 401/403) so a 400 invalid_input / not_bank_toon is branchable too.

/**
 * POST `${API_BASE}${path}` with a JSON body and decode the JSON reply. Mirrors
 * getJSON's credentialed-fetch + typed-error contract. `fetchFn` is injectable
 * for tests (the same seam getJSON exposes).
 */
async function postJSON<T>(path: string, body: unknown, fetchFn: typeof fetch = fetch): Promise<T> {
	let res: Response;
	try {
		res = await fetchFn(`${API_BASE}${path}`, {
			method: 'POST',
			// D-05 / 15-02: the Domain=squirebot.quest session cookie rides
			// cross-subdomain to api.squirebot.quest only on a credentialed request.
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
			body: JSON.stringify(body)
		});
	} catch {
		throw new ApiError(`network error posting ${path}`, 0);
	}
	if (!res.ok) {
		// The server's {"error":"<code>"} discriminator (owner_floor_protected /
		// not_authorized / not_bank_toon / invalid_input / lock_busy / grace_expired).
		const code = await readErrorCode(res);
		if (res.status === 401) throw new Unauthenticated(`unauthenticated posting ${path}`, 401, code);
		if (res.status === 403) throw new Forbidden(`forbidden posting ${path}`, 403, code);
		throw new ApiError(`unexpected ${res.status} posting ${path}`, res.status, code);
	}
	// A 2xx whose body isn't valid JSON surfaces as a branded ApiError (same
	// WR-04 guard getJSON uses) rather than a raw SyntaxError escaping the catch.
	try {
		return (await res.json()) as T;
	} catch {
		throw new ApiError(`malformed JSON from ${path}`, res.status);
	}
}

// --- Admin/coin response + row contract (snake_case, per 15-03 SUMMARY) ---

/** A bank-toon row (`GET /api/v1/coin/bank-toons` element). Coin is null where unset. */
export interface BankToon {
	character_id: number;
	name: string;
	plat: number | null;
	gold: number | null;
	silver: number | null;
	copper: number | null;
}

/** A coin quad (the persisted/echoed plat/gold/silver/copper). */
export interface Coin {
	plat: number;
	gold: number;
	silver: number;
	copper: number;
}

/** The `POST /api/v1/coin` reply: the character name + the persisted coin. */
export interface SaveCoinResult {
	character: string;
	coin: Coin;
}

/**
 * A char-meta row (`GET /api/v1/char/meta-list` element) — every existing
 * (non-removed) character with its current metadata (P16 / CUTOVER-02). The
 * char-meta form pre-fills from these. `level` is null where unset; `class`/`race`
 * are '' where unset (the Go zero-values). Mirrors BankToon.
 */
export interface CharMetaItem {
	character_id: number;
	name: string;
	class: string;
	level: number | null;
	race: string;
	is_bank_toon: boolean;
}

/** The `POST /api/v1/char/meta` reply: the character name + the persisted metadata. */
export interface SaveCharMetaResult {
	character: string;
	class: string;
	level: number | null;
	race: string;
	is_bank_toon: boolean;
}

/** An officer entry (`officers[]`). `is_floor` marks the un-removable owner-floor. */
export interface Officer {
	discord_user_id: string;
	username: string;
	avatar: string | null;
	is_floor: boolean;
}

/** A promotable user (a signed-in member who is not yet an officer). */
export interface PromotableUser {
	discord_user_id: string;
	username: string;
	avatar: string | null;
}

/** The `GET /api/v1/admin/officers` reply. */
export interface OfficersResponse {
	officers: Officer[];
	promotable: PromotableUser[];
}

/** The `POST /api/v1/admin/officers/add` reply (idempotent: added=false when already one). */
export interface AddOfficerResult {
	added: boolean;
	username: string;
}

/** The `POST /api/v1/admin/officers/remove` reply (idempotent: removed=false when absent). */
export interface RemoveOfficerResult {
	removed: boolean;
	username: string;
}

/** An evictable owner (`GET /api/v1/admin/evictable` element). */
export interface EvictableOwner {
	owner_id: number;
	label: string;
	char_count: number;
}

/**
 * A restorable owner (`GET /api/v1/admin/restorable` element) — an evicted guildie
 * still within the grace window (the inverse of EvictableOwner). char_count is how
 * many characters the restore would un-remove; grace_until is the SOONEST deadline
 * (Unix epoch SECONDS, like EvictionPreview.grace_until — NOT a string).
 */
export interface RestorableOwner {
	owner_id: number;
	label: string;
	char_count: number;
	/** Unix epoch SECONDS (mirrors the Go side: MIN(grace_until)). NOT a string. */
	grace_until: number;
}

/** The `GET /api/v1/admin/eviction/preview` reply: the affected character names + grace deadline. */
export interface EvictionPreview {
	owner_id: number;
	characters: string[];
	/** Unix epoch SECONDS (mirrors the Go side: nowUnix()+EvictionGraceSeconds). NOT a string. */
	grace_until: number;
}

/** The `POST /api/v1/admin/evict` reply. */
export interface EvictResult {
	removed_count: number;
	/** Unix epoch SECONDS (CR-02/IN-02: the backend sends a JSON number, not a string). */
	grace_until: number;
}

/**
 * The `POST /api/v1/admin/eviction/restore` reply (re-mints a fresh guild code
 * per D-10). WR-01: when the post-commit re-mint fails, the restore still
 * committed — the server returns new_code_issued:false + code_mint_failed:true so
 * the form can tell the officer "restored, but re-issue a code via the CLI"
 * instead of implying nothing happened. WR-02: new_code_issued:true means a fresh
 * code now EXISTS (printed to the server's journald only — never the response),
 * NOT that the officer holds a deliverable code; it must be handed off out-of-band.
 */
export interface RestoreResult {
	restored_count: number;
	new_code_issued: boolean;
	/** Present + true ONLY when the restore committed but the follow-on code mint failed (WR-01). */
	code_mint_failed?: boolean;
}

// --- Typed admin/coin wrappers (one per 15-03 route) ---------------------

/** GET /api/v1/coin/bank-toons → BankToon[] ([] when none). Login-only. */
export function fetchBankToons(fetchFn: typeof fetch = fetch): Promise<BankToon[]> {
	return getJSON<BankToon[]>('/api/v1/coin/bank-toons', fetchFn);
}

/** POST /api/v1/coin → { character, coin }. Login-only (D-12). */
export function saveCoin(
	body: { character_id: number; plat: number; gold: number; silver: number; copper: number },
	fetchFn: typeof fetch = fetch
): Promise<SaveCoinResult> {
	return postJSON<SaveCoinResult>('/api/v1/coin', body, fetchFn);
}

/** GET /api/v1/char/meta-list → CharMetaItem[] ([] when none). Login-only (D-03). */
export function fetchCharsForMeta(fetchFn: typeof fetch = fetch): Promise<CharMetaItem[]> {
	return getJSON<CharMetaItem[]>('/api/v1/char/meta-list', fetchFn);
}

/** POST /api/v1/char/meta → { character, class, level, race, is_bank_toon }. Login-only (D-03). */
export function saveCharMeta(
	body: { character_id: number; class: string; level: number | null; race: string; is_bank_toon: boolean },
	fetchFn: typeof fetch = fetch
): Promise<SaveCharMetaResult> {
	return postJSON<SaveCharMetaResult>('/api/v1/char/meta', body, fetchFn);
}

/** GET /api/v1/admin/officers → { officers, promotable }. Officer-only (403 not_authorized for a member). */
export function fetchOfficers(fetchFn: typeof fetch = fetch): Promise<OfficersResponse> {
	return getJSON<OfficersResponse>('/api/v1/admin/officers', fetchFn);
}

/** POST /api/v1/admin/officers/add → { added, username }. Officer-only. */
export function addOfficer(
	discord_user_id: string,
	fetchFn: typeof fetch = fetch
): Promise<AddOfficerResult> {
	return postJSON<AddOfficerResult>('/api/v1/admin/officers/add', { discord_user_id }, fetchFn);
}

/** POST /api/v1/admin/officers/remove → { removed, username }. Officer-only; owner-floor protected. */
export function removeOfficer(
	discord_user_id: string,
	fetchFn: typeof fetch = fetch
): Promise<RemoveOfficerResult> {
	return postJSON<RemoveOfficerResult>('/api/v1/admin/officers/remove', { discord_user_id }, fetchFn);
}

/** GET /api/v1/admin/evictable → EvictableOwner[]. Officer-only. */
export function fetchEvictable(fetchFn: typeof fetch = fetch): Promise<EvictableOwner[]> {
	return getJSON<EvictableOwner[]>('/api/v1/admin/evictable', fetchFn);
}

/** GET /api/v1/admin/restorable → RestorableOwner[] ([] when none). Officer-only. */
export function fetchRestorable(fetchFn: typeof fetch = fetch): Promise<RestorableOwner[]> {
	return getJSON<RestorableOwner[]>('/api/v1/admin/restorable', fetchFn);
}

/** GET /api/v1/admin/eviction/preview?owner_id=N → EvictionPreview. Officer-only. */
export function previewEviction(
	owner_id: number,
	fetchFn: typeof fetch = fetch
): Promise<EvictionPreview> {
	return getJSON<EvictionPreview>(
		`/api/v1/admin/eviction/preview?owner_id=${encodeURIComponent(owner_id)}`,
		fetchFn
	);
}

/** POST /api/v1/admin/evict → { removed_count, grace_until }. Officer-only; owner-floor protected. */
export function evict(owner_id: number, fetchFn: typeof fetch = fetch): Promise<EvictResult> {
	return postJSON<EvictResult>('/api/v1/admin/evict', { owner_id }, fetchFn);
}

/** POST /api/v1/admin/eviction/restore → { restored_count, new_code_issued }. Re-mints the guild code. */
export function restoreEviction(
	owner_id: number,
	fetchFn: typeof fetch = fetch
): Promise<RestoreResult> {
	return postJSON<RestoreResult>('/api/v1/admin/eviction/restore', { owner_id }, fetchFn);
}

// --- The forms' server-truth routing helper (B-2 cross-ref) --------------
// A PURE classifier the EvictionForm + AdminMgmtForm catch paths call to decide
// what a caught write error MEANS — kept here (not inline in a .svelte) so the
// 403→officers-only contract is node-unit-testable without mounting a component
// (15-04's established philosophy). The forms map the verdict to behavior:
//   'officers-only'   → call authGuard(err) to collapse the whole admin UI to
//                       the Officers-only refusal (the "you're no longer an
//                       officer" path — NOT a generic inline form-error).
//   'owner-floor'     → render the inline owner-floor protection copy.
//   'lock-busy'       → render the inline lock-busy retry copy.
//   'unauthenticated' → re-throw so the AuthGate guard routes to LoginScreen.
//   'generic'         → render a normal inline form-error with <reason>.

/** The verdict an admin form acts on for a caught write error. */
export type AdminErrorRoute =
	| 'officers-only'
	| 'owner-floor'
	| 'lock-busy'
	| 'unauthenticated'
	| 'generic';

/**
 * Classify a caught admin/officer write error into its server-truth route
 * (B-2). A 401 → 'unauthenticated' (bubbles to AuthGate). A 403 → the matching
 * inline branch by its server code (owner_floor_protected / lock_busy) ELSE
 * 'officers-only' (not_authorized, or a bare 403, collapses the admin UI — never
 * a generic error). Anything else → 'generic'.
 */
export function classifyAdminError(err: unknown): AdminErrorRoute {
	if (err instanceof Unauthenticated) return 'unauthenticated';
	if (err instanceof Forbidden) {
		const code = (err.code ?? '').toLowerCase();
		if (code === 'owner_floor_protected') return 'owner-floor';
		// WR-07: 'lock_busy' is UNREACHABLE from the current backend — the store's
		// busy_timeout(5000) + maxconns=1 serialize writes and wait rather than
		// erroring, so SQLITE_BUSY never surfaces. This branch is kept purely as
		// defense-in-depth for a future where busy_timeout is lowered; the matching
		// backend emitter does not exist today (see webadmin/audit.go WR-07 note).
		if (code === 'lock_busy') return 'lock-busy';
		// not_authorized, or any other/absent 403 code → collapse to the refusal.
		return 'officers-only';
	}
	return 'generic';
}

// --- Self-service watcher codes (17-03 / LINK-01/03/04/05) ----------------
// The three login-only /account/codes endpoints (POST mint / GET list / POST
// revoke), all cookie-credentialed via the shared getJSON/postJSON cores (D-02:
// the owner is session-derived server-side — the mint body is `{}`, never an
// owner). The plaintext `code` crosses to the show-once panel EXACTLY once and
// is never re-fetched (the list returns only #N/created/last-seen — LINK-04).

/** One of the caller's own active watcher codes (GET /api/v1/account/codes). */
export interface OwnCode {
	id: number;
	/** 1-based per-owner ordinal over the active set (the auto-label #N, D-06). */
	ordinal: number;
	created_at: string;
	/** Last upload time; null until the code's watcher first uploads. */
	last_seen: string | null;
}

/** GET /api/v1/account/codes → OwnCode[] ([] when never minted). */
export function fetchOwnCodes(f: typeof fetch = fetch): Promise<OwnCode[]> {
	return getJSON<OwnCode[]>('/api/v1/account/codes', f);
}

/** POST /api/v1/account/codes → { code } (the plaintext, shown ONCE). Body is {} — owner is session-derived (D-02). */
export function mintOwnCode(f: typeof fetch = fetch): Promise<{ code: string }> {
	return postJSON<{ code: string }>('/api/v1/account/codes', {}, f);
}

/** POST /api/v1/account/codes/revoke → { revoked } (false = not the caller's / already revoked — a no-op, not an error). */
export function revokeOwnCode(id: number, f: typeof fetch = fetch): Promise<{ revoked: boolean }> {
	return postJSON<{ revoked: boolean }>('/api/v1/account/codes/revoke', { id }, f);
}
