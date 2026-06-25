<script lang="ts">
	// /wishlist — the per-character per-slot upgrade Wishlist tab (Phase 34,
	// WISH-01..07). Replaces the Phase-30 WantlistPanel placeholder with the real
	// per-character per-slot master-detail. Structurally it MIRRORS the approved
	// P31/P32 /characters + /inventory master-detail VERBATIM (the two-pane grid,
	// the scoped search visual, StateBlock states, the bespoke selectable-row list,
	// the asSlot ExaminePanel reuse seam) — the only new pieces are the per-slot
	// accordion (the 21 P31 worn slots), the per-slot target rows (name + price +
	// wiki + last-listed + Raid tag + ping Toggle + EC-hit badge + examine), the
	// per-slot suggestion picker (WISH-04), and the cross-wishlist search (WISH-07).
	// The P30 Notifications region (NotificationPrefsPanel + NotificationInbox,
	// NAV-04) stays BELOW the two-pane — the per-character wishlist is the PRIMARY
	// content; this page does NOT import or mutate the badge store (P30 §4 gotcha —
	// NotificationInbox owns its own badge refresh).
	//
	// SECURITY: every character/item/slot name + the search query renders via plain
	// {} interpolation (Svelte auto-escapes, T-34-13) — this file uses NO raw-HTML
	// directive. The ONE sanctioned escaped {@html} sink is INSIDE the reused
	// ExaminePanel (composeItemNote) — no new sink. The ?c= deep-link + the
	// fetchWishlist char are encodeURIComponent'd (guildie-controlled names).
	//
	// SERVER-TRUTH (T-34-15, the v2.2 wantlist discipline — DISTINGUISHES this tab):
	// every add/remove/ping AWAITs the POST then re-fetches the authoritative
	// wishlist (fetchWishlist(selected)) — NEVER optimistic-mutate. Remove is
	// destructive → ConfirmDialog. Writes are owner-scoped (the server re-authorizes
	// the character via IsCharAssignedToTx → 403, T-34-07/14); a non-owned
	// character's wishlist renders READ-ONLY (UI guard, NOT the security boundary).
	//
	// The pure sort/filter/search (wishlist.ts) is node-tested; the list/search/
	// accordion/target-row/ping/suggestion/remove/examine DOM here is NOT covered by
	// node vitest (DOM-blind) — its browser verification is the 34-04 deploy-then-
	// browser-smoke (node vitest can't see the render, hover/pin, or the writes).

	import { onMount, getContext } from 'svelte';
	import Search from '@lucide/svelte/icons/search';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import ExaminePanel from '$lib/components/ExaminePanel.svelte';
	import FacetBar from '$lib/components/FacetBar.svelte';
	import LastSyncedCell from '$lib/components/cells/LastSyncedCell.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import Toggle from '$lib/components/Toggle.svelte';
	import NotificationPrefsPanel from '$lib/components/NotificationPrefsPanel.svelte';
	import NotificationInbox from '$lib/components/NotificationInbox.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
	import {
		Unauthenticated,
		Forbidden,
		fetchCharacters,
		fetchMyCharacters,
		fetchWishlist,
		addWishlist,
		removeWishlist,
		setWishlistPing,
		searchCatalog,
		type RosterCharacter,
		type MyCharacter,
		type CatalogItem,
		type WishlistView,
		type WishlistSlot,
		type WishlistTarget,
		type WishlistSuggestion,
		type InventorySlot
	} from '$lib/api';
	import { wishlistRoster, filterWishlistRoster, searchWishlistItems } from '$lib/wishlist/wishlist';

	type PageStatus = 'loading' | 'error' | 'ready';
	type WinStatus = 'idle' | 'loading' | 'error' | 'ready';

	// The AuthGate guard from context (server-truth re-routing on a 401/403).
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	let pageStatus = $state<PageStatus>('loading');
	let chars = $state<RosterCharacter[]>([]);
	// The viewer's own assigned characters (name → character_id) for the add body's
	// REQUIRED character_id. fetchCharacters' RosterCharacter has no id; this maps it.
	let myChars = $state<MyCharacter[]>([]);
	let query = $state('');
	let selected = $state<string | null>(null);

	// Per-selection window state machine (the /characters winStatus pattern + stale guard).
	let winStatus = $state<WinStatus>('idle');
	let view = $state<WishlistView | null>(null);
	let viewFor = $state<string | null>(null); // which char `view` belongs to (stale guard)

	// The pinned examine target (single ExaminePanel, replace-on-click — WISH-06).
	let examineSlot = $state<InventorySlot | null>(null);

	// The WISH-07 corpus cache (keyed by char name) — every non-bank/bot char's
	// wishlist, lazily fetched on the first non-empty query + populated from the
	// `view` whenever a char is selected (so re-searching doesn't re-fetch a loaded
	// char). There is NO scope-to-loaded-view escape hatch — the corpus is always
	// all non-bank/bot chars' wishlists.
	let wishlistCache = $state<Record<string, WishlistView>>({});
	let corpusLoading = $state(false);
	const corpusErrors = new Set<string>(); // per-char fetch errors (skip, don't abort)

	// Mutation/announce state.
	let writeBusy = $state(false);
	let liveMsg = $state('');

	// Remove (confirm-before-commit) state.
	let removeTarget = $state<{ t: WishlistTarget; slot: string } | null>(null);
	let removeDialogOpen = $state(false);

	// ── Initial load ──────────────────────────────────────────────────────────
	async function load() {
		pageStatus = 'loading';
		try {
			const [roster, mine] = await Promise.all([fetchCharacters(), fetchMyCharacters()]);
			chars = roster;
			myChars = mine;
			// A Retry can drop the pinned char — clear a now-stale selection.
			const sel = selected;
			if (sel && !chars.some((c) => c.name.toLowerCase() === sel.toLowerCase())) {
				selected = null;
			}
			pageStatus = 'ready';
			// If a ?c= pre-selected a char, kick its window fetch now (post-roster).
			if (selected) void loadWishlist(selected);
		} catch (err) {
			route(err, () => (pageStatus = 'error'));
		}
	}

	function refetch() {
		void load();
	}

	// One-shot initial fetch + pre-select ?c=<char> (deep-link). onMount is the
	// correct fire-once primitive (a bare $effect would re-fire).
	onMount(() => {
		const c = new URLSearchParams(window.location.search).get('c');
		if (c) selected = c;
		void load();
	});

	/** Route a caught error: a 401/403 hands to AuthGate (server-truth re-route);
	 *  anything else runs the supplied fallback. Returns true when re-routed. */
	function route(err: unknown, fallback: () => void): boolean {
		if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
			authGuard(err);
			return true;
		}
		fallback();
		return false;
	}

	// ── The character list (WISH-01) ────────────────────────────────────────────
	// The full banks/bots-excluded viewer-first roster (drives the corpus + the list).
	let wlChars = $derived(wishlistRoster(chars));
	// The filtered list shown in CHARACTER-LIST mode + the CHARACTERS search group.
	let shownChars = $derived(filterWishlistRoster(chars, query));

	type Band = 'mine' | 'guild';
	const BAND_LABEL: Record<Band, string> = { mine: 'YOUR CHARACTERS', guild: 'GUILD' };
	const BAND_ROWS: Band[] = ['mine', 'guild'];
	let bands = $derived.by(() => {
		const g: Record<Band, RosterCharacter[]> = { mine: [], guild: [] };
		for (const c of shownChars) g[c.is_mine ? 'mine' : 'guild'].push(c);
		return g;
	});

	let searching = $derived(query.trim() !== '');

	// ── The WISH-07 cross-wishlist search ───────────────────────────────────────
	// On the first non-empty query, lazily fetch every non-bank/bot char's wishlist
	// not already cached (a 401/403 → authGuard; a per-char error → skip + record).
	let corpusEnsured = false;
	async function ensureCorpus() {
		if (corpusEnsured) return;
		corpusEnsured = true;
		corpusLoading = true;
		try {
			const missing = wlChars.filter((c) => !(c.name in wishlistCache));
			for (const c of missing) {
				try {
					const wv = await fetchWishlist(c.name);
					wishlistCache = { ...wishlistCache, [c.name]: wv };
				} catch (err) {
					// A 401/403 aborts the whole corpus to the auth guard; any other
					// per-char error is skipped (recorded) so one bad char doesn't kill
					// the rest of the search corpus.
					if (err instanceof Unauthenticated || err instanceof Forbidden) {
						if (authGuard) authGuard(err);
						return;
					}
					corpusErrors.add(c.name);
				}
			}
		} finally {
			corpusLoading = false;
		}
	}

	// Kick the lazy corpus fetch whenever a query is active (mirrors the search-mode
	// switch; ensureCorpus is idempotent so this fires the fetch exactly once).
	$effect(() => {
		if (searching) void ensureCorpus();
	});

	// The WISHLIST ITEMS results = the cross-wishlist grouping over the FULL cached
	// corpus (every non-bank/bot char's wishlist — NOT scoped to the loaded view).
	let itemResults = $derived(
		searching ? searchWishlistItems(Object.values(wishlistCache), query) : []
	);

	// no-results when a query is active, the corpus is settled, and BOTH groups are empty.
	let noResults = $derived(
		searching && !corpusLoading && shownChars.length === 0 && itemResults.length === 0
	);

	// ── Selection + per-character wishlist fetch ────────────────────────────────
	function select(name: string) {
		selected = name;
		examineSlot = null; // clear the pinned examine on a new selection
		// URL-reflect the selection (?c=<name>) — deep-linkable; encodeURIComponent
		// because names are guildie-controlled; history.replaceState keeps focus/scroll.
		if (typeof history !== 'undefined') {
			const url = new URL(window.location.href);
			url.searchParams.set('c', name);
			history.replaceState(history.state, '', url);
		}
		void loadWishlist(name);
	}

	async function loadWishlist(char: string) {
		winStatus = 'loading';
		viewFor = char;
		try {
			const got = await fetchWishlist(char);
			if (viewFor !== char) return; // a stale response for a since-changed selection
			view = got;
			winStatus = 'ready';
			// Feed the WISH-07 corpus without a re-fetch (re-searching won't re-fetch this char).
			wishlistCache = { ...wishlistCache, [char]: got };
		} catch (err) {
			if (viewFor !== char) return;
			route(err, () => (winStatus = 'error'));
		}
	}

	function retryWishlist() {
		if (selected) void loadWishlist(selected);
	}

	// The selected char's roster row (for the detail-header meta + the is_mine ownership gate).
	let selectedChar = $derived(chars.find((c) => c.name === selected) ?? null);
	// Owner gate: the add/remove/ping controls only render for the viewer's OWN chars.
	let ownsSelected = $derived(selectedChar?.is_mine === true);
	// The character_id for the add body (the viewer's own assigned char; undefined when not owned).
	let selectedCharId = $derived(
		selected ? (myChars.find((m) => m.name === selected)?.character_id ?? null) : null
	);

	// True when the selected char's wishlist has zero (visible) targets anywhere — the
	// D-01 "No targets yet" first-run state (the accordion still renders below).
	let noTargets = $derived(
		winStatus === 'ready' &&
			view !== null &&
			view.slots.every((s) => s.targets.length === 0)
	);

	// The dimmed meta line for a char (D-11): show what's known, never "null".
	function metaLine(c: RosterCharacter): string {
		const parts: string[] = [];
		if (c.level > 0) parts.push(`Level ${c.level}`);
		if (c.race) parts.push(c.race);
		if (c.class) parts.push(c.class);
		return parts.join(' ');
	}

	// ── The examine reuse seam (WISH-06 — the asSlot idiom, identical to /inventory) ─
	// Build a representative InventorySlot per item so <ExaminePanel> renders UNCHANGED.
	// The list-context-irrelevant fields are zero/empty; category MUST be the literal
	// 'general'; charLastSeen="" omits the footer (not a per-wishlist-item fact).
	function asSlot(opts: {
		name: string;
		price: number | null;
		wiki_url: string;
	}): InventorySlot {
		return {
			item: opts.name,
			icon_id: 0,
			statsblock: '',
			wiki_summary: '',
			is_quest_item: false,
			price: opts.price,
			prices: [],
			wiki_url: opts.wiki_url,
			location: '',
			category: 'general',
			canonical_slot: '',
			id: 0,
			count: 0,
			slots: 0,
			last_listed: '',
			children: []
		};
	}
	function examineTarget(t: WishlistTarget) {
		examineSlot = asSlot({ name: t.item_name, price: t.price, wiki_url: t.wiki_url });
	}
	function examineSuggestion(s: WishlistSuggestion) {
		examineSlot = asSlot({ name: s.item_name, price: s.price, wiki_url: s.wiki_url });
	}
	function examineEquipped(name: string) {
		examineSlot = asSlot({ name, price: null, wiki_url: '' });
	}

	// Round + comma-group a price for the inline pp display (matches /inventory fmtPp).
	function fmtPp(n: number): string {
		return Math.round(n).toLocaleString('en-US');
	}

	// Which accordion sections are expanded. Default: expand a slot iff it has ≥1
	// target (UI-SPEC §D); the rest collapse to the equipped line. A user toggle
	// overrides per slot for the current selection.
	let expanded = $state<Record<string, boolean>>({});
	$effect(() => {
		// Reset expansion to the default whenever the loaded view changes (new char).
		const v = view;
		if (v && winStatus === 'ready') {
			const next: Record<string, boolean> = {};
			for (const s of v.slots) next[s.slot] = s.targets.length > 0;
			expanded = next;
		}
	});
	function toggleSlot(slot: string) {
		expanded = { ...expanded, [slot]: !expanded[slot] };
	}
	function targetWord(n: number): string {
		return n === 1 ? '1 target' : `${n} targets`;
	}

	// ── Server-truth writes (await POST → re-fetch the authoritative wishlist) ───
	async function doAdd(slot: string, item_id: number | null, item_name: string) {
		if (!selected || !ownsSelected || selectedCharId == null || writeBusy) return;
		writeBusy = true;
		try {
			await addWishlist({ character_id: selectedCharId, slot, item_id, item_name });
			await loadWishlist(selected);
			liveMsg = `Added ${item_name} to ${selected}'s ${slot} wishlist.`;
		} catch (err) {
			route(err, () => {
				liveMsg = `Couldn't add ${item_name}. Nothing was added — try again.`;
			});
		} finally {
			writeBusy = false;
		}
	}

	function openRemoveConfirm(t: WishlistTarget, slot: string) {
		if (writeBusy) return;
		removeTarget = { t, slot };
		removeDialogOpen = true;
	}

	async function doRemove() {
		removeDialogOpen = false;
		const rt = removeTarget;
		if (!rt || !selected || writeBusy) return;
		writeBusy = true;
		try {
			await removeWishlist(rt.t.id);
			await loadWishlist(selected);
			liveMsg = `Removed ${rt.t.item_name} from your wishlist.`;
		} catch (err) {
			route(err, () => {
				liveMsg = `Couldn't remove ${rt.t.item_name}. No change was made — try again.`;
			});
		} finally {
			removeTarget = null;
			writeBusy = false;
		}
	}

	async function doPing(t: WishlistTarget) {
		if (!selected || !ownsSelected || writeBusy) return;
		writeBusy = true;
		const next = !t.pinged;
		try {
			await setWishlistPing(t.id, next);
			await loadWishlist(selected);
			liveMsg = next ? `Pinging for ${t.item_name}.` : `Stopped pinging for ${t.item_name}.`;
		} catch (err) {
			route(err, () => {
				// Re-read to reconcile the toggle to the persisted state.
				if (selected) void loadWishlist(selected);
			});
		} finally {
			writeBusy = false;
		}
	}

	// ── The per-slot typed-entry add (the WantAddForm debounce idiom, CLONED) ────
	// Per-slot debounced catalog search staging. Keyed by slot name so each open
	// accordion section has its own little add field without N child components.
	const DEBOUNCE_MS = 250;
	let addSlot = $state<string | null>(null); // which slot's add field is active
	let addQuery = $state('');
	let addResults = $state<CatalogItem[]>([]);
	let addSearching = $state(false);
	let addSeq = 0;
	let addTimer: ReturnType<typeof setTimeout> | null = null;
	// Phase 39 (D-01): the Clicky/Haste facet chips narrow the catalog add-search.
	// Catalog-only here — NO Holdings/Catalog scope control on the wishlist add-form.
	let addClicky = $state(false);
	let addHaste = $state(false);

	function openAdd(slot: string) {
		addSlot = slot;
		addQuery = '';
		addResults = [];
		addSearching = false;
		// Reset the facets per open so a new slot's add starts unfiltered.
		addClicky = false;
		addHaste = false;
	}
	function closeAdd() {
		addSlot = null;
		addQuery = '';
		addResults = [];
		addSearching = false;
		addClicky = false;
		addHaste = false;
	}
	function onAddInput() {
		if (addTimer) clearTimeout(addTimer);
		const q = addQuery.trim();
		if (q.length < 2) {
			addResults = [];
			addSearching = false;
			return;
		}
		addSearching = true;
		addTimer = setTimeout(() => void runAddSearch(q), DEBOUNCE_MS);
	}
	async function runAddSearch(q: string) {
		const seq = ++addSeq;
		try {
			const items = await searchCatalog(q, { clicky: addClicky, haste: addHaste });
			if (seq !== addSeq) return;
			addResults = items;
		} catch {
			if (seq !== addSeq) return;
			addResults = [];
		} finally {
			if (seq === addSeq) addSearching = false;
		}
	}

	// Re-run the (already-debounced) catalog add-search when a facet chip toggles, so the
	// suggestions narrow immediately. Respects the existing addSeq seq-guard; honors the
	// same 2-rune guard as onAddInput (a sub-2-rune query just stays empty).
	function onFacetToggle() {
		const q = addQuery.trim();
		if (q.length < 2) {
			addResults = [];
			addSearching = false;
			return;
		}
		addSearching = true;
		void runAddSearch(q);
	}
	async function pickCatalog(slot: string, item: CatalogItem) {
		await doAdd(slot, item.item_id, item.name);
		closeAdd();
	}
	async function pickCustom(slot: string) {
		const label = addQuery.trim();
		if (!label) return;
		await doAdd(slot, null, label);
		closeAdd();
	}
	let addNoExactMatch = $derived(
		addQuery.trim().length >= 2 &&
			!addSearching &&
			!addResults.some((r) => r.name.toLowerCase() === addQuery.trim().toLowerCase())
	);
</script>

<svelte:head>
	<title>SquireBot — Wishlist</title>
</svelte:head>

{#if pageStatus === 'loading'}
	<StateBlock kind="loading" />
{:else if pageStatus === 'error'}
	<StateBlock kind="error" onRetry={refetch} />
{:else if wlChars.length === 0}
	<!-- No non-bank/bot characters synced yet. Reuses the shared empty copy (§I). -->
	<StateBlock kind="empty" />
{:else}
	<p class="live" aria-live="polite">{liveMsg}</p>

	<div class="wishlist">
		<!-- LEFT: scoped search + the bespoke viewer-first char list (banks/bots excluded). -->
		<div class="list-col">
			<div class="search">
				<Search size={16} aria-hidden="true" class="search-icon" />
				<input
					class="search-input"
					type="search"
					placeholder="Search characters and wishlists…"
					aria-label="Search characters and wishlists"
					bind:value={query}
				/>
			</div>
			<p class="search-hint">Search characters and wishlists.</p>

			{#if !searching}
				<!-- CHARACTER-LIST mode: the two-band viewer-first list (§B). -->
				<div class="bands">
					{#each BAND_ROWS as band (band)}
						{#if bands[band].length > 0}
							<p class="band-label">{BAND_LABEL[band]}</p>
							{#each bands[band] as c (c.name)}
								<button
									type="button"
									class="row"
									class:selected={selected === c.name}
									aria-pressed={selected === c.name}
									onclick={() => select(c.name)}
								>
									<span class="row-main">
										<span class="row-name">{c.name}</span>
										{#if c.is_mine}<span class="tag">you</span>{/if}
									</span>
									{#if metaLine(c)}
										<span class="row-meta">{metaLine(c)}</span>
									{/if}
								</button>
							{/each}
						{/if}
					{/each}
				</div>
			{:else if noResults}
				<StateBlock kind="no-results" {query} />
			{:else}
				<!-- SEARCH-RESULTS mode: two labelled groups (§C). -->
				<div class="results">
					{#if shownChars.length > 0}
						<p class="band-label">CHARACTERS</p>
						{#each shownChars as c (c.name)}
							<button
								type="button"
								class="row"
								class:selected={selected === c.name}
								aria-pressed={selected === c.name}
								onclick={() => select(c.name)}
							>
								<span class="row-main">
									<span class="row-name">{c.name}</span>
									{#if c.is_mine}<span class="tag">you</span>{/if}
								</span>
								{#if metaLine(c)}
									<span class="row-meta">{metaLine(c)}</span>
								{/if}
							</button>
						{/each}
					{/if}

					<p class="band-label">WISHLIST ITEMS</p>
					{#if corpusLoading}
						<p class="corpus-loading">Searching all wishlists…</p>
					{:else if itemResults.length === 0}
						<p class="corpus-empty">No wishlisted items match.</p>
					{:else}
						{#each itemResults as r (r.item_name)}
							<button type="button" class="item-result" onclick={() => select(r.where[0].char)}>
								<span class="item-result-name">{r.item_name}</span>
								<span class="item-result-where">
									{#each r.where as w, i (`${w.char}:${w.slot}:${i}`)}
										<span class="where-chip">{w.char} · {w.slot}</span>
									{/each}
								</span>
							</button>
						{/each}
					{/if}
				</div>
			{/if}
		</div>

		<!-- RIGHT: the per-character detail (the per-slot accordion). -->
		<div class="detail-col">
			{#if selected === null}
				<!-- §J "pick a character" prompt — not an error. -->
				<div class="prompt">
					<h2 class="prompt-heading">Pick a character</h2>
					<p class="prompt-body">
						Choose one of your characters to see its equipment slots and what you can wishlist to
						upgrade them.
					</p>
				</div>
			{:else if winStatus === 'loading'}
				<StateBlock kind="loading" />
			{:else if winStatus === 'error'}
				<StateBlock kind="error" onRetry={retryWishlist} />
			{:else if view !== null}
				<div class="detail">
					<!-- Per-character detail header: NAME (Heading accent) + meta (§D.1). -->
					<div class="detail-header">
						<h2 class="detail-name">{view.char}</h2>
						{#if selectedChar && metaLine(selectedChar)}
							<p class="detail-meta">{metaLine(selectedChar)}</p>
						{/if}
					</div>

					<!-- The single pinned examine (WISH-06) — REUSED ExaminePanel, UNCHANGED.
					     charLastSeen="" omits the footer. The .examine-wrap drops the sticky
					     positioning IN THIS TAB (the examine stacks above the accordion in one
					     scroll column, the /inventory precedent). -->
					<div class="examine-wrap">
						<ExaminePanel slot={examineSlot} charLastSeen="" />
					</div>

					{#if noTargets}
						<!-- D-01 first-run: friendly empty block; the accordion still renders below. -->
						<div class="no-targets">
							<h3 class="no-targets-heading">No targets yet</h3>
							<p class="no-targets-body">
								This wishlist is empty. Open a slot below and add an upgrade target — pick from the
								suggestions or type any item.
							</p>
						</div>
					{/if}

					<!-- The per-slot accordion (§D — server-ordered, render as-given). -->
					<div class="accordion">
						{#each view.slots as slot (slot.slot)}
							<section class="slot">
								<button
									type="button"
									class="slot-header"
									aria-expanded={expanded[slot.slot] === true}
									onclick={() => toggleSlot(slot.slot)}
								>
									<span class="slot-eyebrow">{slot.slot}</span>
									<span class="slot-summary">
										{#if slot.targets.length > 0}
											<span class="slot-count">{targetWord(slot.targets.length)}</span>
										{:else}
											<span class="slot-count dim">no targets</span>
										{/if}
									</span>
								</button>

								<!-- Equipped line (always visible — the slot's at-a-glance state). -->
								<div class="equipped">
									<span class="eq-eyebrow">EQUIPPED</span>
									{#if slot.equipped}
										<button
											type="button"
											class="eq-name"
											onclick={() => examineEquipped(slot.equipped)}
											onmouseenter={() => examineEquipped(slot.equipped)}
										>{slot.equipped}</button>
									{:else}
										<span class="eq-empty">Empty</span>
									{/if}
								</div>

								{#if expanded[slot.slot]}
									<!-- Target rows (§E) — already auto-removal-filtered server-side (D-02). -->
									{#if slot.targets.length === 0}
										<p class="slot-empty">No targets yet — add one below.</p>
									{:else}
										<ul class="targets">
											{#each slot.targets as t (t.id)}
												<li class="target">
													<button
														type="button"
														class="t-name"
														onclick={() => examineTarget(t)}
														onmouseenter={() => examineTarget(t)}
													>{t.item_name}</button>
													{#if t.price != null}
														<span class="t-price">{fmtPp(t.price)}pp</span>
													{/if}
													{#if t.wiki_url}
														<a
															class="t-wiki"
															href={t.wiki_url}
															target="_blank"
															rel="noopener"
															onclick={(e) => e.stopPropagation()}>Wiki ↗</a
														>
													{/if}
													{#if t.last_listed}
														<span class="t-listed"><LastSyncedCell lastSynced={t.last_listed} /></span>
													{/if}
													{#if t.pinged_hit}
														<span class="ec-badge" title="SquireBot pinged you — this appeared in EC"
															>Seen in EC</span
														>
													{/if}
													{#if ownsSelected}
														<span class="t-controls">
															<Toggle
																on={t.pinged}
																label={`Ping me when ${t.item_name} appears in EC`}
																onToggle={() => doPing(t)}
																disabled={writeBusy}
															/>
															<button
																type="button"
																class="t-remove"
																aria-label={`Remove ${t.item_name}`}
																disabled={writeBusy}
																onclick={() => openRemoveConfirm(t, slot.slot)}>Remove</button
															>
														</span>
													{/if}
												</li>
											{/each}
										</ul>
									{/if}

									<!-- The add control + suggestion picker (OWNED chars only, §D/§F/§G). -->
									{#if ownsSelected}
										<div class="add-control">
											{#if addSlot === slot.slot}
												<label class="add-field">
													<Search size={16} aria-hidden="true" />
													<input
														type="search"
														placeholder="Search items to add…"
														aria-label={`Search items to add to ${slot.slot}`}
														bind:value={addQuery}
														oninput={onAddInput}
													/>
												</label>
												<!-- Phase 39 (D-01): the Clicky/Haste facet chips narrow the
												     catalog suggestions — catalog-only, NO scope toggle here. -->
												<div class="add-facets">
													<FacetBar
														clicky={addClicky}
														haste={addHaste}
														onToggleClicky={() => {
															addClicky = !addClicky;
															onFacetToggle();
														}}
														onToggleHaste={() => {
															addHaste = !addHaste;
															onFacetToggle();
														}}
													/>
												</div>
												{#if addResults.length > 0}
													<ul class="add-results">
														{#each addResults as item (item.item_id)}
															<li>
																<button
																	type="button"
																	class="add-pick"
																	disabled={writeBusy}
																	onclick={() => pickCatalog(slot.slot, item)}>{item.name}</button
																>
															</li>
														{/each}
													</ul>
												{/if}
												{#if addNoExactMatch}
													<button
														type="button"
														class="add-custom"
														disabled={writeBusy}
														onclick={() => pickCustom(slot.slot)}
													>
														Add "{addQuery.trim()}" as a custom target
													</button>
												{/if}
												<button type="button" class="add-cancel" onclick={closeAdd}>Cancel</button>
											{:else}
												<button type="button" class="add-open" onclick={() => openAdd(slot.slot)}
													>Add a target</button
												>
											{/if}

											{#if slot.suggestions.length > 0}
												<div class="suggestions">
													<p class="sug-eyebrow">SUGGESTIONS</p>
													<ul class="sug-list">
														{#each slot.suggestions as s (s.item_name)}
															<li class="sug">
																<button
																	type="button"
																	class="sug-name"
																	onclick={() => examineSuggestion(s)}
																	onmouseenter={() => examineSuggestion(s)}>{s.item_name}</button
																>
																{#if s.is_raid}
																	<span class="raid-tag">Raid</span>
																	<span class="not-for-sale">Not for sale</span>
																{:else if s.price != null}
																	<span class="t-price">{fmtPp(s.price)}pp</span>
																{/if}
																{#if s.wiki_url}
																	<a
																		class="t-wiki"
																		href={s.wiki_url}
																		target="_blank"
																		rel="noopener"
																		onclick={(e) => e.stopPropagation()}>Wiki ↗</a
																	>
																{/if}
																{#if s.last_listed}
																	<span class="t-listed"
																		><LastSyncedCell lastSynced={s.last_listed} /></span
																	>
																{/if}
																<button
																	type="button"
																	class="sug-add"
																	aria-label={`Add ${s.item_name}`}
																	disabled={writeBusy}
																	onclick={() => doAdd(slot.slot, null, s.item_name)}>Add</button
																>
															</li>
														{/each}
													</ul>
												</div>
											{/if}
										</div>
									{/if}
								{/if}
							</section>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</div>

	<!-- The P30 Notifications region (NAV-04) — stays reachable on this tab. This
	     page does NOT import/mutate the badge store (NotificationInbox owns it). -->
	<section class="form-card notify-region">
		<header class="notify-intro">
			<h2 class="form-title">Notifications</h2>
			<p class="form-purpose">
				Your SquireBot alerts live here. Choose what you get pinged about, and read everything the
				bot has tried to send you.
			</p>
		</header>

		<NotificationPrefsPanel />

		<div class="divider"></div>

		<NotificationInbox />
	</section>

	<ConfirmDialog
		open={removeDialogOpen}
		heading="Remove this target?"
		body={`Remove ${removeTarget?.t.item_name ?? ''} from ${selected ?? ''}'s ${removeTarget?.slot ?? ''} wishlist? You can always add it back later.`}
		confirmLabel={`Remove ${removeTarget?.t.item_name ?? ''}`}
		onConfirm={doRemove}
		onCancel={() => {
			removeDialogOpen = false;
			removeTarget = null;
		}}
	/>
{/if}

<style>
	/* A visually-hidden polite live region for the add/remove/ping announces. */
	.live {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	/* Two-pane master-detail (identical to /characters — short char names, 280–360px). */
	.wishlist {
		display: grid;
		grid-template-columns: minmax(280px, 360px) 1fr;
		gap: 24px; /* lg */
		align-items: start;
	}

	/* --- Scoped search (reused verbatim from /characters .search) --- */
	.search {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 44px;
		padding: 8px 12px;
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
	}
	.search:focus-within {
		border-color: var(--accent);
	}
	:global(.search-icon) {
		flex: 0 0 auto;
		color: var(--text);
		opacity: 0.7;
	}
	.search-input {
		flex: 1 1 auto;
		min-width: 0;
		background: transparent;
		border: none;
		color: var(--text);
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
	}
	.search-input::placeholder {
		color: var(--text);
		opacity: 0.6;
	}
	.search-input:focus {
		outline: none;
	}
	.search-input:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.search-hint {
		margin: 8px 0 16px;
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text);
		opacity: 0.7;
	}

	/* --- The bespoke viewer-first char list (§B) + search-result groups (§C) --- */
	.bands,
	.results {
		display: flex;
		flex-direction: column;
	}
	.band-label {
		margin: 16px 0 4px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.band-label:first-child {
		margin-top: 0;
	}
	.row {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		width: 100%;
		min-height: 44px;
		padding: 8px 16px;
		text-align: left;
		background: var(--panel);
		border: none;
		border-bottom: 1px solid var(--border, var(--accent));
		border-left: 3px solid transparent;
		color: var(--text);
		cursor: pointer;
		font-family: var(--font-body);
	}
	.row:hover {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.row:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.row.selected {
		border-left-color: var(--accent);
	}
	.row.selected .row-name {
		color: var(--accent);
	}
	.row-main {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.row-name {
		font-size: 16px;
		line-height: 1.3;
	}
	.tag {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--accent);
		opacity: 0.85;
	}
	.row-meta {
		font-size: 16px;
		line-height: 1.4;
		opacity: 0.85;
	}

	/* --- The WISHLIST ITEMS search-result rows (§C) --- */
	.corpus-loading,
	.corpus-empty {
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text);
		opacity: 0.7;
		padding: 8px 16px;
	}
	.item-result {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 4px;
		width: 100%;
		min-height: 44px;
		padding: 8px 16px;
		text-align: left;
		background: var(--panel);
		border: none;
		border-bottom: 1px solid var(--border, var(--accent));
		border-left: 3px solid transparent;
		color: var(--text);
		cursor: pointer;
		font-family: var(--font-body);
	}
	.item-result:hover {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.item-result:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.item-result-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px;
	}
	.item-result-where {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
	.where-chip {
		font-size: 13px;
		opacity: 0.7;
	}

	/* --- The detail column (§D) --- */
	.detail-col {
		min-height: 200px;
	}
	.detail {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.detail-header {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.detail-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px;
		line-height: 1.2;
		color: var(--accent);
		margin: 0;
		overflow-wrap: anywhere;
	}
	.detail-meta {
		margin: 0;
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		color: var(--text);
		opacity: 0.85;
	}

	/* Drop ExaminePanel's sticky positioning IN THIS TAB (the /inventory precedent —
	   the examine stacks above the accordion in one scroll column). */
	.examine-wrap :global(.examine) {
		position: static;
		max-height: none;
		overflow: visible;
	}

	/* --- D-01 empty (no targets) block --- */
	.no-targets {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 24px 16px;
		text-align: center;
		opacity: 0.9;
	}
	.no-targets-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px;
		line-height: 1.2;
		color: var(--text);
		margin: 0;
	}
	.no-targets-body {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		max-width: 44ch;
		margin: 0 auto;
		color: var(--text);
	}

	/* --- The per-slot accordion (§D) --- */
	.accordion {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.slot {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 16px 0;
		border-top: 1px solid var(--border, var(--accent));
	}
	.slot:first-child {
		border-top: none;
		padding-top: 0;
	}
	.slot-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		min-height: 44px;
		padding: 0;
		background: none;
		border: none;
		cursor: pointer;
		text-align: left;
	}
	.slot-header:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.slot-eyebrow {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.slot-count {
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text);
		opacity: 0.85;
	}
	.slot-count.dim {
		opacity: 0.55;
	}
	.equipped {
		display: flex;
		align-items: baseline;
		gap: 8px;
		flex-wrap: wrap;
	}
	.eq-eyebrow {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--text);
		opacity: 0.6;
	}
	.eq-name {
		background: none;
		border: none;
		padding: 0;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		cursor: pointer;
		text-align: left;
	}
	.eq-name:hover {
		color: var(--accent);
	}
	.eq-name:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.eq-empty {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		opacity: 0.7;
	}
	.slot-empty {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		opacity: 0.7;
		margin: 0;
	}

	/* --- Target rows (§E) --- */
	.targets,
	.sug-list,
	.add-results {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.target,
	.sug {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px;
		min-height: 44px;
		padding: 8px 0;
	}
	.t-name,
	.sug-name {
		background: none;
		border: none;
		padding: 0;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px;
		color: var(--text);
		cursor: pointer;
		text-align: left;
	}
	.t-name:hover,
	.sug-name:hover {
		color: var(--accent);
	}
	.t-name:focus-visible,
	.sug-name:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.t-price {
		color: var(--status-other);
		font-variant-numeric: tabular-nums;
		font-size: 16px;
	}
	.t-wiki {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		text-decoration: none;
		font-size: 13px;
		white-space: nowrap;
	}
	.t-wiki:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.t-listed {
		font-size: 13px;
	}
	.raid-tag,
	.not-for-sale {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--status-other);
	}
	.not-for-sale {
		text-transform: none;
		letter-spacing: 0;
		opacity: 0.85;
	}
	.ec-badge {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		color: var(--accent);
		background: color-mix(in srgb, var(--accent) 8%, transparent);
		white-space: nowrap;
	}
	.t-controls {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-left: auto;
	}
	.t-remove {
		min-height: 44px;
		padding: 4px 12px;
		background: none;
		border: 1px solid transparent;
		color: var(--destructive);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		cursor: pointer;
	}
	.t-remove:hover:not(:disabled) {
		border-color: var(--destructive);
		border-radius: 4px;
	}
	.t-remove:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.t-remove:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 4px;
	}

	/* --- The add control + suggestion picker (§F/§G) --- */
	.add-control {
		display: flex;
		flex-direction: column;
		gap: 16px;
		padding-top: 8px;
	}
	.add-open {
		align-self: flex-start;
		min-height: 44px;
		padding: 8px 16px;
		background: var(--accent);
		border: none;
		border-radius: 4px;
		color: var(--bg);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		cursor: pointer;
	}
	.add-open:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.add-field {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 0 12px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		background: var(--panel);
		color: var(--accent);
		max-width: 480px;
	}
	/* Phase 39: the facet chips below the add-search input (spacing handled by the
	   .add-control flex gap; this wrapper keeps them on their own row, left-aligned). */
	.add-facets {
		display: flex;
	}
	.add-field input {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--text);
		font-family: var(--font-body);
		font-size: 16px;
		padding: 8px 0;
		min-height: 44px;
		outline: none;
	}
	.add-field:focus-within {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.add-pick {
		display: block;
		width: 100%;
		min-height: 44px;
		padding: 8px 12px;
		background: none;
		border: 1px solid transparent;
		border-radius: 4px;
		color: var(--accent);
		font-family: var(--font-body);
		font-size: 16px;
		text-align: left;
		cursor: pointer;
	}
	.add-pick:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 6%, transparent);
		border-color: var(--border, var(--accent));
	}
	.add-pick:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.add-pick:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.add-custom {
		align-self: flex-start;
		min-height: 44px;
		padding: 8px 0;
		background: none;
		border: none;
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		font-family: var(--font-body);
		font-size: 16px;
		cursor: pointer;
	}
	.add-custom:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.add-custom:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.add-cancel {
		align-self: flex-start;
		min-height: 44px;
		padding: 4px 8px;
		background: none;
		border: none;
		color: var(--text);
		opacity: 0.7;
		font-family: var(--font-body);
		font-size: 13px;
		cursor: pointer;
	}
	.add-cancel:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.suggestions {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.sug-eyebrow {
		margin: 0;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.sug-add {
		min-height: 44px;
		padding: 4px 12px;
		margin-left: auto;
		background: none;
		border: 1px solid var(--accent);
		border-radius: 4px;
		color: var(--accent);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		cursor: pointer;
	}
	.sug-add:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.sug-add:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.sug-add:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}

	/* --- The "pick a character" prompt (§J) --- */
	.prompt {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 48px 16px;
		text-align: center;
		opacity: 0.85;
	}
	.prompt-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px;
		line-height: 1.2;
		color: var(--text);
	}
	.prompt-body {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		max-width: 44ch;
		color: var(--text);
	}

	/* --- The Notifications region (NAV-04) — the rehomed /notifications card --- */
	.notify-region {
		margin-top: 24px;
	}
	.form-card {
		max-width: 720px;
		padding: 24px;
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.notify-intro {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.form-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px;
		line-height: 1.2;
		color: var(--text);
	}
	.form-purpose {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	.divider {
		border-top: 1px solid var(--border, var(--accent));
	}

	/* Mobile: list + detail stack (§H) — same breakpoint /characters uses. */
	@media (max-width: 640px) {
		.wishlist {
			grid-template-columns: 1fr;
		}
	}
</style>
