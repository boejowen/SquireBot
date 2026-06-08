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

/**
 * POST /api/v1/char/meta → { character, class, level, race, is_bank_toon }. Login-only
 * (D-03). The member path no longer sends is_bank_toon — it is officer-only now (Phase
 * 26 / OPEN-3, via designateChar) — so the body carries class/level/race only. The
 * reply still echoes is_bank_toon (read-only) for the views that display it.
 */
export function saveCharMeta(
	body: { character_id: number; class: string; level: number | null; race: string },
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

// --- Wantlist + catalog search (19-03 / WANT-01/WANT-02) ------------------
// The four login-only wantlist endpoints (GET list / POST add / POST remove /
// GET catalog search), all cookie-credentialed via the shared getJSON/postJSON
// cores (D-02: the owner is session-derived server-side — the add/remove bodies
// carry NO owner). The in-guild "who holds it" display joins the wantlist rows'
// stable item_id against the consolidated all-character inventory (fetchView()),
// client-side, in holdersFor (see $lib/wantlist/holders).

/** One of the caller's own wantlist entries (GET /api/v1/wantlist). */
export interface WantlistRow {
	id: number;
	/** null for a custom text want (excluded from the in-guild join + future alerts, D-04/D-07). */
	item_id: number | null;
	item_name: string;
	reason: 'buy' | 'quest';
	priority: 'low' | 'med' | 'high';
	note: string | null;
	created_at: number;
	/**
	 * D-09 per-want mute state. The backend ListOwnWants (Plan 01 — BLOCKER-3 closed)
	 * returns `muted` on every row, so this is a DETERMINISTIC interface field (not a
	 * conditional "add if missing"); WantMuteCell reads it to choose the bell glyph.
	 */
	muted: boolean;
}

/** A catalog match from the PigParse-getall corpus (GET /api/v1/items/search). */
export interface CatalogItem {
	item_id: number;
	name: string;
	/** The recent average price in pp, when known. */
	current_avg?: number;
}

/** GET /api/v1/wantlist → WantlistRow[] ([] when the list is empty). */
export function fetchOwnWants(f: typeof fetch = fetch): Promise<WantlistRow[]> {
	return getJSON<WantlistRow[]>('/api/v1/wantlist', f);
}

/** GET /api/v1/items/search?q=… → CatalogItem[] (server returns [] for q<2). */
export function searchCatalog(q: string, f: typeof fetch = fetch): Promise<CatalogItem[]> {
	return getJSON<CatalogItem[]>('/api/v1/items/search?q=' + encodeURIComponent(q), f);
}

/** POST /api/v1/wantlist → the created WantlistRow. Body carries NO owner (session-derived, D-02). */
export function addWant(
	body: {
		item_id: number | null;
		item_name: string;
		reason: 'buy' | 'quest';
		priority: 'low' | 'med' | 'high';
		note?: string;
	},
	f: typeof fetch = fetch
): Promise<WantlistRow> {
	return postJSON<WantlistRow>('/api/v1/wantlist', body, f);
}

/** POST /api/v1/wantlist/remove → { removed } (false = not the caller's / already removed — a no-op, not an error). */
export function removeWant(id: number, f: typeof fetch = fetch): Promise<{ removed: boolean }> {
	return postJSON<{ removed: boolean }>('/api/v1/wantlist/remove', { id }, f);
}

// --- Notifications (20-04 / WANT-04) ---------------------------------------
// The six login-only /api/v1/notifications endpoints (prefs get/set, inbox,
// unread-count, mark-read, mark-all-read) backing the /notifications page +
// the unread nav badge — all cookie-credentialed via the shared
// getJSON/postJSON cores (D-02: the owner is session-derived server-side; NO
// body carries an owner). This block is OWNED by Plan 04; Plan 05 APPENDS its
// officer-monitor wrappers in a SEPARATE `// --- Monitors (20-05 / WANT-08) ---`
// block BELOW this one (a different wave — do NOT remove or reorder these). The
// muteWant wrapper lives here too because /wantlist already lives in this file;
// Plan 05's WantMuteCell consumes it.
//
// IMPORTANT (P15 epoch-seconds crasher): `sent_at` and `read_at` on AlertLogRow
// are unix EPOCH SECONDS (NOT milliseconds) — every consumer that builds a Date
// MUST multiply by 1000 first (see NotificationRow's relativeTime).

/** The caller's notify prefs — master + three per-monitor booleans (default-ON, D-01). */
export interface NotifyPrefs {
	master: boolean;
	ec: boolean;
	wts: boolean;
	raid: boolean;
}

/**
 * One alert-attempt row from the caller's inbox (GET /api/v1/notifications/inbox,
 * newest-first). `send_status` is the delivery outcome (the load-bearing CAN'T-DM
 * safety net); `read_at` null ⇒ unread. `sent_at` / `read_at` are unix EPOCH
 * SECONDS (multiply by 1000 for a JS Date — the P15 crasher).
 */
export interface AlertLogRow {
	id: number;
	source: string;
	item_id: number | null;
	detail: string | null;
	sent_at: number;
	send_status: 'sent' | 'dm_blocked' | 'error';
	read_at: number | null;
}

/** GET /api/v1/notifications/prefs → NotifyPrefs (default-ON for a new caller). */
export function fetchPrefs(f: typeof fetch = fetch): Promise<NotifyPrefs> {
	return getJSON<NotifyPrefs>('/api/v1/notifications/prefs', f);
}

/** POST /api/v1/notifications/prefs → the stored NotifyPrefs. Body carries NO owner (session-derived, D-02). */
export function savePrefs(body: NotifyPrefs, f: typeof fetch = fetch): Promise<NotifyPrefs> {
	return postJSON<NotifyPrefs>('/api/v1/notifications/prefs', body, f);
}

/** GET /api/v1/notifications/inbox → AlertLogRow[] (newest-first; [] when empty). */
export function fetchInbox(f: typeof fetch = fetch): Promise<AlertLogRow[]> {
	return getJSON<AlertLogRow[]>('/api/v1/notifications/inbox', f);
}

/** GET /api/v1/notifications/unread-count → { count } (the nav-badge number, D-05). */
export function fetchUnreadCount(f: typeof fetch = fetch): Promise<{ count: number }> {
	return getJSON<{ count: number }>('/api/v1/notifications/unread-count', f);
}

/** POST /api/v1/notifications/read → { read } (false = not the caller's / already read — a no-op). Body is {id} only (D-02). */
export function markRead(id: number, f: typeof fetch = fetch): Promise<{ read: boolean }> {
	return postJSON<{ read: boolean }>('/api/v1/notifications/read', { id }, f);
}

/** POST /api/v1/notifications/read-all → { count } (rows flipped). Body is {} — owner is session-derived (D-02). */
export function markAllRead(f: typeof fetch = fetch): Promise<{ count: number }> {
	return postJSON<{ count: number }>('/api/v1/notifications/read-all', {}, f);
}

/** POST /api/v1/wantlist/mute → { muted }. Body is {id, muted} only — owner session-derived (D-02). Consumed by Plan 05's WantMuteCell. */
export function muteWant(
	id: number,
	muted: boolean,
	f: typeof fetch = fetch
): Promise<{ muted: boolean }> {
	return postJSON<{ muted: boolean }>('/api/v1/wantlist/mute', { id, muted }, f);
}

// --- Monitors (20-05 / WANT-08) --------------------------------------------
// The officer-only monitor-control wrappers backing the /admin Monitors section
// (the three guild-wide kill-switches + the registered source-channel CRUD + the
// D-10 "send me a test alert" bot-pulse). ALL cookie-credentialed via the shared
// getJSON/postJSON cores; errors are routed by the CALLER through classifyAdminError
// (a 403 not_authorized collapses the admin UI to the officers-only refusal — the
// same server-truth contract the eviction/officer forms use). This block is
// APPENDED below Plan 04's Notifications block — it does NOT modify or reorder it.
//
// JSON contracts (Plan 03 / webadmin/monitors.go, confirmed against the Go source):
//   GET  /api/v1/admin/monitors                → { flags: {ec,wts,raid}, channels: GuildChannel[] }
//   POST /api/v1/admin/monitors/flag           ← { monitor, enabled } → { monitor, enabled }
//   POST /api/v1/admin/monitors/channel        ← { label, channel_id, monitor } → { added, channel_id, monitor }
//                                                 (409 {"error":"duplicate"} on a dup channel+monitor)
//   POST /api/v1/admin/monitors/channel/remove ← { channel_id, monitor } → { removed }
//   POST /api/v1/admin/monitors/test           ← {} → { status:"sent" } | { error:"dm_blocked" } | (5xx)
// where monitor ∈ 'ec_auction' | 'wts' | 'raid_target'.

/** A guild-wide monitor name (the kill-switch + guild_channel.monitor enum). */
export type MonitorName = 'ec_auction' | 'wts' | 'raid_target';

/** One officer-registered source channel (the channels[] element of the monitors GET). */
export interface GuildChannel {
	id: number;
	channel_id: string;
	label: string;
	monitor: MonitorName;
	enabled: boolean;
	created_at: number;
}

/** The three guild-wide kill-switch flags (EC ships ON; WTS/raid ship dark). */
export interface MonitorFlags {
	ec: boolean;
	wts: boolean;
	raid: boolean;
}

/** The `GET /api/v1/admin/monitors` reply: the flags + the registered channels. */
export interface MonitorState {
	flags: MonitorFlags;
	channels: GuildChannel[];
}

/** GET /api/v1/admin/monitors → { flags, channels }. Officer-only. */
export function fetchMonitors(f: typeof fetch = fetch): Promise<MonitorState> {
	return getJSON<MonitorState>('/api/v1/admin/monitors', f);
}

/** POST /api/v1/admin/monitors/flag → { monitor, enabled }. Officer-only; toggles one guild-wide kill-switch. */
export function setMonitorFlag(
	monitor: string,
	enabled: boolean,
	f: typeof fetch = fetch
): Promise<{ monitor: string; enabled: boolean }> {
	return postJSON<{ monitor: string; enabled: boolean }>(
		'/api/v1/admin/monitors/flag',
		{ monitor, enabled },
		f
	);
}

/**
 * POST /api/v1/admin/monitors/channel → { added, channel_id, monitor }. Officer-only;
 * registers a guild_channel row. The body carries NO owner (guild-wide). A duplicate
 * (channel_id, monitor) surfaces as a 409 Forbidden/ApiError with .code='duplicate'.
 */
export function addGuildChannel(
	body: { label: string; channel_id: string; monitor: string },
	f: typeof fetch = fetch
): Promise<{ added: boolean; channel_id: string; monitor: string }> {
	return postJSON<{ added: boolean; channel_id: string; monitor: string }>(
		'/api/v1/admin/monitors/channel',
		body,
		f
	);
}

/** POST /api/v1/admin/monitors/channel/remove → { removed }. Officer-only; body {channel_id, monitor}. */
export function removeGuildChannel(
	channel_id: string,
	monitor: string,
	f: typeof fetch = fetch
): Promise<{ removed: boolean }> {
	return postJSON<{ removed: boolean }>(
		'/api/v1/admin/monitors/channel/remove',
		{ channel_id, monitor },
		f
	);
}

/**
 * POST /api/v1/admin/monitors/test → { status:"sent" } | { error:"dm_blocked" } | (5xx). Officer-only.
 * The D-10 bot-pulse: DMs the CALLING officer + logs to their inbox. Body is {} — the
 * target is always the caller, never another user (T-20-22). A bot-down surfaces as a
 * 5xx ApiError the caller maps to the bot_unavailable feedback line.
 */
export function sendTestAlert(
	f: typeof fetch = fetch
): Promise<{ status?: string; error?: string }> {
	return postJSON<{ status?: string; error?: string }>('/api/v1/admin/monitors/test', {}, f);
}

// --- Character assignment (26-03 / ASSIGN-01..05) --------------------------
// The twelve assignment endpoints over the 26-02 backend: six member ones under
// RequireSession (mine/claimable/claim/release/request/request-cancel) and six
// officer ones under RequireOfficer (list-all/assign/remove/approve/deny/designate).
// ALL cookie-credentialed via the shared getJSON/postJSON cores. Per D-02 / Pitfall 1
// every member body carries ONLY character_id — NO actor field; the acting identity
// is the session cookie the server reads. Field names are snake_case to match the Go
// JSON contract (the BankToon/CharMetaItem precedent at api.ts:274-310). Error codes
// the panels route on: already_assigned / char_shared / duplicate_request (409),
// not_authorized (403), invalid_input (400), internal (500).

/**
 * One of the caller's assigned characters (GET /api/v1/assignments/mine — the
 * "My characters" read). Mirrors the Go store.Assignment JSON: the assignee is
 * discord_user_id (always the caller for this endpoint); assigned_by is 'self' /
 * 'migration' / an officer id.
 */
export interface MyCharacter {
	character_id: number;
	name: string;
	discord_user_id: string;
	assigned_at: number;
	assigned_by: string;
}

/**
 * One claimable character (GET /api/v1/assignments/claimable). The backend today
 * returns ONLY unassigned, non-shared, live characters, so `assignee` is absent on
 * every current row — but the field is modeled (optional) so partitionClaimable can
 * split instantly-claimable (no assignee → Claim) from contested (assigned to someone
 * else → Request) without a schema change if the backend ever widens this read.
 */
export interface ClaimableCharacter {
	character_id: number;
	name: string;
	/** The current holder's discord_user_id when the char is assigned to someone else; absent/null = unassigned (instant-claimable). */
	assignee?: string | null;
}

/**
 * One of the caller's OWN pending assignment requests (GET
 * /api/v1/assignments/requests/mine — the member "my outstanding requests" read).
 * Mirrors the Go store.MyPendingRequest. MyCharactersPanel uses it to rehydrate the
 * Request→Cancel affordance across a reload (so a re-request after reload doesn't hit a
 * guaranteed 409 duplicate_request). Requester-scoped server-side (the session is the
 * actor — no actor field on the wire).
 */
export interface MyPendingRequest {
	character_id: number;
	character_name: string;
	created_at: number;
}

/**
 * One assignment row (GET /api/v1/admin/assignments → assignments[]). Mirrors the Go
 * store.Assignment: the character + its current assignee (discord_user_id) + provenance.
 */
export interface Assignment {
	character_id: number;
	name: string;
	discord_user_id: string;
	assigned_at: number;
	assigned_by: string;
}

/**
 * One pending request (GET /api/v1/admin/assignments → requests[]) — the officer
 * approval queue. Mirrors the Go store.PendingRequest: the queue lists pending rows
 * only, so there is no status field on the wire (every row is implicitly 'pending');
 * `id` is the request_id approve/deny take. current_assignee is null for a now-
 * unassigned contested char.
 */
export interface PendingRequest {
	id: number;
	character_id: number;
	character_name: string;
	requester: string;
	current_assignee: string | null;
	created_at: number;
}

/** The `GET /api/v1/admin/assignments` reply: every live-char assignment + the pending queue. */
export interface AllAssignments {
	assignments: Assignment[];
	requests: PendingRequest[];
}

/** An officer designate mode (mutually-exclusive guild bank / guild bot / neither). */
export type DesignateMode = 'bank' | 'bot' | 'none';

/** GET /api/v1/assignments/mine → MyCharacter[] ([] when none). Login-only. */
export function fetchMyCharacters(f: typeof fetch = fetch): Promise<MyCharacter[]> {
	return getJSON<MyCharacter[]>('/api/v1/assignments/mine', f);
}

/** GET /api/v1/assignments/requests/mine → MyPendingRequest[] ([] when none). Login-only, requester-scoped. */
export function fetchMyPendingRequests(f: typeof fetch = fetch): Promise<MyPendingRequest[]> {
	return getJSON<MyPendingRequest[]>('/api/v1/assignments/requests/mine', f);
}

/** GET /api/v1/assignments/claimable → ClaimableCharacter[] ([] when none). Login-only. */
export function fetchClaimable(f: typeof fetch = fetch): Promise<ClaimableCharacter[]> {
	return getJSON<ClaimableCharacter[]>('/api/v1/assignments/claimable', f);
}

/** POST /api/v1/assignments/claim → { claimed }. Body {character_id} — actor is session-derived (D-02). */
export function claimChar(character_id: number, f: typeof fetch = fetch): Promise<{ claimed: boolean }> {
	return postJSON<{ claimed: boolean }>('/api/v1/assignments/claim', { character_id }, f);
}

/** POST /api/v1/assignments/release → { released } (false = not the caller's / unassigned — a silent no-op). */
export function releaseChar(character_id: number, f: typeof fetch = fetch): Promise<{ released: boolean }> {
	return postJSON<{ released: boolean }>('/api/v1/assignments/release', { character_id }, f);
}

/** POST /api/v1/assignments/request → { requested }. Files a pending request for a contested char (D-07). */
export function requestChar(character_id: number, f: typeof fetch = fetch): Promise<{ requested: boolean }> {
	return postJSON<{ requested: boolean }>('/api/v1/assignments/request', { character_id }, f);
}

/** POST /api/v1/assignments/request/cancel → { cancelled } (false = not the caller's / not pending — a no-op). */
export function cancelRequest(character_id: number, f: typeof fetch = fetch): Promise<{ cancelled: boolean }> {
	return postJSON<{ cancelled: boolean }>('/api/v1/assignments/request/cancel', { character_id }, f);
}

/** GET /api/v1/admin/assignments → { assignments, requests }. Officer-only (403 not_authorized for a member). */
export function fetchAllAssignments(f: typeof fetch = fetch): Promise<AllAssignments> {
	return getJSON<AllAssignments>('/api/v1/admin/assignments', f);
}

/** POST /api/v1/admin/assignments/assign → { assigned }. Officer-only; body {character_id, assignee}. */
export function officerAssign(
	character_id: number,
	assignee: string,
	f: typeof fetch = fetch
): Promise<{ assigned: boolean }> {
	return postJSON<{ assigned: boolean }>(
		'/api/v1/admin/assignments/assign',
		{ character_id, assignee },
		f
	);
}

/** POST /api/v1/admin/assignments/remove → { removed }. Officer-only; body {character_id}. */
export function officerRemoveAssign(
	character_id: number,
	f: typeof fetch = fetch
): Promise<{ removed: boolean }> {
	return postJSON<{ removed: boolean }>('/api/v1/admin/assignments/remove', { character_id }, f);
}

/** POST /api/v1/admin/assignments/approve → { approved }. Officer-only; body {request_id}. */
export function approveRequest(
	request_id: number,
	f: typeof fetch = fetch
): Promise<{ approved: boolean }> {
	return postJSON<{ approved: boolean }>('/api/v1/admin/assignments/approve', { request_id }, f);
}

/** POST /api/v1/admin/assignments/deny → { denied }. Officer-only; body {request_id}. */
export function denyRequest(request_id: number, f: typeof fetch = fetch): Promise<{ denied: boolean }> {
	return postJSON<{ denied: boolean }>('/api/v1/admin/assignments/deny', { request_id }, f);
}

/** POST /api/v1/admin/characters/designate → 200. Officer-only; body {character_id, mode∈{bank,bot,none}}. */
export function designateChar(
	character_id: number,
	mode: DesignateMode,
	f: typeof fetch = fetch
): Promise<{ designated: boolean }> {
	return postJSON<{ designated: boolean }>(
		'/api/v1/admin/characters/designate',
		{ character_id, mode },
		f
	);
}
