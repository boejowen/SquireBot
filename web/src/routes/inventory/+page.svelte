<script lang="ts">
	// /inventory — the item-centric Inventory tab (Phase 32, ITEM-01/02/03).
	// Replaces the Phase-30 placeholder stub. The guild-wide answer to
	// "which characters have item X?": a bespoke, viewer-first selectable item list
	// (left) + a scoped viewer-priority search (top) + the selected item's detail
	// (right) = the REUSED P31 ExaminePanel (charLastSeen="" → footer omitted) plus a
	// holders table whose rows deep-link into the live /characters?c=<name> window
	// (the P31 selection seam — zero P31 change). Selection is URL-reflected via
	// ?i=<name> (history.replaceState — no per-item route file; one reusable detail,
	// the relaxed consolidated-views rule). Single pinned panel, replace-on-click
	// (D-03a). Master-detail mirrors /characters/+page.svelte 1:1 so the two tabs feel
	// identical (32-UI-SPEC §A/§E).
	//
	// The pure sort/filter (items.ts) is node-tested; the list/search/selection +
	// detail DOM here is NOT covered by node vitest (DOM-blind) — its browser
	// verification is the 32-03 deploy-then-browser-smoke.
	//
	// SECURITY (T-32-07/08/10): item + character names render via plain {}
	// interpolation (Svelte auto-escapes) in the list rows, the detail header, and the
	// holders table — this file uses NO raw-HTML directive. The ONE sanctioned escaped
	// raw-HTML sink is inside the reused ExaminePanel (composeItemNote) — no new sink.
	// The holder deep-link encodeURIComponent's the guildie-controlled char name
	// (T-32-08). The item-icon <img> uses a TRUSTED integer icon_id (T-32-09).

	import { onMount, onDestroy, getContext } from 'svelte';
	import Search from '@lucide/svelte/icons/search';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import ExaminePanel from '$lib/components/ExaminePanel.svelte';
	import FacetBar from '$lib/components/FacetBar.svelte';
	import LastSyncedCell from '$lib/components/cells/LastSyncedCell.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
	import {
		Unauthenticated,
		Forbidden,
		fetchItems,
		searchCatalog,
		type ItemRollup,
		type ItemHolder,
		type CatalogItem,
		type InventorySlot
	} from '$lib/api';
	import { filterItems, facetItems, sortHolders } from '$lib/items';

	type Status = 'loading' | 'error' | 'ready';
	type Scope = 'holdings' | 'catalog';

	// The AuthGate guard from context (server-truth re-routing on a 401/403).
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	let status = $state<Status>('loading');
	let items = $state<ItemRollup[]>([]);
	let query = $state('');
	// The NORMALIZED name of the pinned item (replace-on-click, D-03a).
	let selected = $state<string | null>(null);

	// Phase 39 — the Holdings↔Catalog scope segmented control (D-03 default Holdings)
	// + the AND-combined Clicky/Haste facet chips (D-02). The facets work in BOTH
	// scopes; the scope only switches the DATA SOURCE (holdings = the already-loaded
	// rollup filtered client-side; catalog = a debounced server search).
	let scope = $state<Scope>('holdings');
	let clicky = $state(false);
	let haste = $state(false);
	// Catalog-scope server results + their own load state (the debounced search).
	let catalogRows = $state<CatalogItem[]>([]);
	let catalogStatus = $state<'idle' | 'searching' | 'ready'>('idle');

	async function load() {
		status = 'loading';
		try {
			items = await fetchItems();
			// A Retry/refetch can drop the item the detail is pinned to — clear a
			// now-stale selection so the detail column doesn't stick on a gone item.
			const sel = selected;
			if (sel && !items.some((r) => r.name === sel)) {
				selected = null;
			}
			status = 'ready';
		} catch (err) {
			// Server-truth: a 401/403 means the session is gone or refused — hand it to
			// the AuthGate guard so the site re-routes instead of showing a stale view.
			// Any other failure (network/5xx) stays the generic error StateBlock.
			if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
				authGuard(err);
			} else {
				status = 'error';
			}
		}
	}

	// Retry re-fires the fetch (UI-SPEC §H error-state Retry).
	function refetch() {
		void load();
	}

	// One-shot initial fetch + pre-select ?i=<name> if present (deep-link). onMount is
	// the correct primitive for a fire-once load (a bare $effect would re-fire).
	onMount(() => {
		const i = new URLSearchParams(window.location.search).get('i');
		if (i) selected = i;
		void load();
	});

	// The HOLDINGS facet+search list (ITEM-02 / D-02): name-filter (viewer-first) THEN
	// the AND-combined Clicky/Haste facet, both pure node-tested helpers from $lib/items.
	// Catalog scope NEVER uses this — it reads `catalogRows` from the server instead.
	let shown = $derived(facetItems(filterItems(items, query), { clicky, haste }));

	// A normalized-name → rollup map over the already-loaded holdings (NO new endpoint,
	// D-04): a catalog-scope row reuses this to show holders for a held item / detect
	// an unheld one. Rebuilds only when `items` changes.
	let holdingsByName = $derived(
		new Map(items.map((r) => [r.name.toLowerCase().trim(), r]))
	);

	// ── The unified row view-model — both scopes render the SAME list/detail markup ──
	// Holdings rows carry their own rollup; catalog rows carry the CatalogItem + the
	// held rollup looked up by normalized name (held → holders; unheld → "not held in
	// the guild"). `held` is the source of truth for the headline + the holders table.
	type RowVM = {
		name: string;
		icon_id: number;
		price: number | null;
		wiki_url: string;
		held: ItemRollup | null; // the holdings rollup when this item is held (any scope)
	};

	let rows = $derived<RowVM[]>(
		scope === 'holdings'
			? shown.map((r) => ({
					name: r.name,
					icon_id: r.icon_id,
					price: r.price,
					wiki_url: r.wiki_url,
					held: r
				}))
			: catalogRows.map((c) => {
					const held = holdingsByName.get(c.name.toLowerCase().trim()) ?? null;
					return {
						name: c.name,
						// A held catalog row reuses the rollup's icon/price; an unheld one falls
						// back to the catalog's id-less tile (icon_id 0 → colored fallback) +
						// current_avg price (CatalogItem carries no wiki_url).
						icon_id: held?.icon_id ?? 0,
						price: held?.price ?? c.current_avg ?? null,
						wiki_url: held?.wiki_url ?? '',
						held
					};
				})
	);

	// The currently-pinned row VM (or null when nothing is selected / it's gone from the
	// active scope's result set). Selection is keyed by the normalized item NAME so it
	// survives a scope flip when the item exists in both (catalog ⊇ holdings).
	let selectedRow = $derived(rows.find((r) => r.name === selected) ?? null);

	// The examine reuse seam (load-bearing — UI-SPEC §C): a representative
	// InventorySlot-shaped object so <ExaminePanel> renders UNCHANGED. A HELD item
	// surfaces its full rollup (statsblock/wiki/prices); an UNHELD catalog item only
	// carries name + catalog price (no stored stats/wiki yet) — examine omits those
	// lines (the "" / [] sentinels). The list-context-irrelevant fields
	// (count/slots/children/canonical_slot/location) are zero/empty — examine ignores
	// them. category MUST be the literal 'general' (the union member). charLastSeen=""
	// omits the footer (per-holder last-synced lives in the holders table, ITEM-03).
	let asSlot = $derived<InventorySlot | null>(
		selectedRow
			? {
					item: selectedRow.name,
					icon_id: selectedRow.held?.icon_id ?? 0,
					statsblock: selectedRow.held?.statsblock ?? '',
					wiki_summary: selectedRow.held?.wiki_summary ?? '',
					is_quest_item: selectedRow.held?.is_quest_item ?? false,
					price: selectedRow.price,
					prices: selectedRow.held?.prices ?? [],
					wiki_url: selectedRow.wiki_url,
					is_no_drop: selectedRow.held?.is_no_drop ?? false,
					is_lore: selectedRow.held?.is_lore ?? false,
					is_magic: selectedRow.held?.is_magic ?? false,
					quest_links: selectedRow.held?.quest_links ?? [],
					location: '',
					category: 'general',
					canonical_slot: '',
					id: 0,
					count: 0,
					slots: 0,
					last_listed: '',
					children: []
				}
			: null
	);

	// The selected item's holders (held → its rollup's holders; unheld → []). Drives the
	// detail HOLDERS section in BOTH scopes (D-04: a held item shows its holders even in
	// catalog scope; an unheld catalog item reads "not held in the guild").
	let selectedHolders = $derived<ItemHolder[]>(selectedRow?.held?.holders ?? []);

	// ── Catalog scope: the debounced server search (clone of the wishlist add-form
	// idiom — DEBOUNCE_MS + a monotonic seq-guard). Fires on query/clicky/haste change
	// while scope === 'catalog'. The 2-rune guard mirrors the server (Plan 01 Open-Q2):
	// a query under 2 runes returns [] — no client-side "browse all clickies". ───────
	const CATALOG_DEBOUNCE_MS = 250;
	let catalogSeq = 0;
	let catalogTimer: ReturnType<typeof setTimeout> | null = null;

	// Params (NOT the reactive $state) carry the values captured by the debounce effect
	// at fire time — the seq-guard then discards any stale resolution. Named query/clicky/
	// haste so the call below reads with those identifiers (the catalog search-source seam).
	async function runCatalogSearch(query: string, clicky: boolean, haste: boolean) {
		const seq = ++catalogSeq;
		catalogStatus = 'searching';
		try {
			const res = await searchCatalog(query, { clicky, haste });
			if (seq !== catalogSeq) return; // a newer search superseded this one
			catalogRows = res;
			catalogStatus = 'ready';
		} catch (err) {
			if (seq !== catalogSeq) return;
			// A 401/403 hands off to the AuthGate guard (server-truth); other failures
			// surface the empty/no-results state rather than crashing (SC-4 graceful).
			if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
				authGuard(err);
				return;
			}
			catalogRows = [];
			catalogStatus = 'ready';
		}
	}

	// The catalog-search effect: re-runs the debounced fetch whenever the inputs change
	// in catalog scope. Reads `scope`/`query`/`clicky`/`haste` so Svelte tracks them.
	$effect(() => {
		if (scope !== 'catalog') return;
		const q = query.trim();
		const c = clicky;
		const h = haste;
		if (catalogTimer) clearTimeout(catalogTimer);
		if (q.length < 2) {
			// Mirror the server's 2-rune guard — no empty/short-q corpus dump.
			++catalogSeq; // cancel any in-flight search
			catalogRows = [];
			catalogStatus = 'ready';
			return;
		}
		catalogStatus = 'searching';
		catalogTimer = setTimeout(() => void runCatalogSearch(q, c, h), CATALOG_DEBOUNCE_MS);
	});

	onDestroy(() => {
		if (catalogTimer) clearTimeout(catalogTimer);
	});

	// Non-empty list after a ready load with zero matches AND a non-empty query → the
	// no-results state (vs. the "No items yet" empty state when the list is empty). In
	// catalog scope the "no matches" state also keys off the catalog search finishing.
	let noResults = $derived(
		scope === 'holdings'
			? status === 'ready' && rows.length === 0 && query.trim() !== ''
			: catalogStatus === 'ready' && rows.length === 0 && query.trim().length >= 2
	);

	/** Flip the Holdings↔Catalog scope. D-03 "a lens, not a reset": NEVER touch
	 *  query / clicky / haste here — the toggle re-runs the SAME search+facets against
	 *  the other data source. A selection absent in the new scope's result set clears
	 *  via the stale-selection guard below (catalog ⊇ holdings, so Holdings→Catalog
	 *  keeps a held selection; Catalog→Holdings drops a selection on an unheld item). */
	function setScope(next: Scope) {
		if (next === scope) return;
		scope = next;
	}

	// Stale-selection guard on scope flip / list change (reuses the load()-time idiom):
	// clear a selection that is absent in the active scope's current result set so the
	// detail column doesn't stick on an item the new scope doesn't list.
	$effect(() => {
		const sel = selected;
		if (sel === null) return;
		// In catalog scope, hold the selection while the search is still resolving (the
		// rows are transiently empty mid-fetch) — only clear once the results settle.
		if (scope === 'catalog' && catalogStatus !== 'ready') return;
		if (!rows.some((r) => r.name === sel)) {
			selected = null;
		}
	});

	function select(name: string) {
		selected = name;
		// URL-reflect the selection (?i=<name>) WITHOUT a full navigation or a new
		// route file — deep-linkable + shareable; the single reusable detail renders
		// for the selected item. The URL setter encodes the value; history.replaceState
		// keeps focus + scroll (no reload).
		if (typeof history !== 'undefined') {
			const url = new URL(window.location.href);
			url.searchParams.set('i', name);
			history.replaceState(history.state, '', url);
		}
	}

	// A deterministic hue from the item name so the colored-tile fallback is stable
	// per item (the PaperdollSlot mechanic, D-02 — the ONE sanctioned non-token color).
	function hueFor(name: string, iconId: number): number {
		const key = name + ':' + iconId;
		let h = 0;
		for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) % 360;
		return h;
	}

	function onImgError(e: Event) {
		// Hide the <img> so the colored-tile under-layer shows through (D-02).
		(e.currentTarget as HTMLImageElement).style.display = 'none';
	}

	// Round + comma-group the PigParse price for the inline row + detail meta (matches
	// the examine's formatPp posture). Caller renders nothing when price is null (D-09).
	function fmtPp(n: number): string {
		return Math.round(n).toLocaleString('en-US');
	}

	// The list-row + detail-header headline: "{qty} · {N} holders" (D-04; singular
	// "1 holder"). The numbers ride accent in the template; this is just the count word.
	function holderWord(n: number): string {
		return n === 1 ? 'holder' : 'holders';
	}
</script>

<svelte:head>
	<title>Inventory — SquireBot</title>
</svelte:head>

{#if status === 'loading'}
	<StateBlock kind="loading" />
{:else if status === 'error'}
	<StateBlock kind="error" onRetry={refetch} />
{:else if items.length === 0}
	<!-- Nothing held by anyone yet: no inventory synced. Reuses the shared empty copy. -->
	<StateBlock kind="empty" />
{:else}
	<div class="inventory">
		<!-- Left: scoped viewer-priority search + the bespoke viewer-first item list. -->
		<div class="list-col">
			<div class="search">
				<Search size={16} aria-hidden="true" class="search-icon" />
				<input
					class="search-input"
					type="search"
					placeholder="Search items…"
					aria-label="Search items"
					bind:value={query}
				/>
			</div>

			<!-- Phase 39 control bar (UI-SPEC §1 layout `[ Holdings | Catalog ]  [Clicky]
			     [Haste]`): the scope segmented control (Inventory-ONLY) on the LEFT, the
			     facet chips on the right, gap:16px between the two clusters. -->
			<div class="control-bar">
				<div class="seg" role="group" aria-label="Search scope">
					<button
						type="button"
						class="seg-btn"
						class:active={scope === 'holdings'}
						aria-pressed={scope === 'holdings'}
						onclick={() => setScope('holdings')}>Holdings</button
					>
					<button
						type="button"
						class="seg-btn"
						class:active={scope === 'catalog'}
						aria-pressed={scope === 'catalog'}
						onclick={() => setScope('catalog')}>Catalog</button
					>
				</div>
				<FacetBar
					{clicky}
					{haste}
					onToggleClicky={() => (clicky = !clicky)}
					onToggleHaste={() => (haste = !haste)}
				/>
			</div>

			<!-- Scope helper hint (UI-SPEC Copywriting): Holdings keeps the existing line;
			     Catalog swaps to the "everything in the catalog" framing. -->
			<p class="search-hint">
				{#if scope === 'holdings'}
					Items on your characters match first.
				{:else}
					Showing everything in the P99 catalog — held or not.
				{/if}
			</p>

			{#if noResults}
				<StateBlock kind="no-results" {query} />
				{#if scope === 'catalog'}
					<!-- A sparse catalog reads as "still loading", not "broken" (SC-4 graceful
					     degradation): unheld-item facet coverage fills on the weekly sync. -->
					<p class="catalog-hint">
						The full catalog fills in after the weekly sync — held items are always searchable.
					</p>
				{/if}
			{:else}
				<!-- Single run — Holdings is viewer-first-then-A-Z (§B); Catalog is the
				     server's prefix-first order (Plan 01 Open-Q1). Both render via `rows`. -->
				<div class="rows">
					{#each rows as it (it.name)}
						<button
							type="button"
							class="row"
							class:selected={selected === it.name}
							aria-pressed={selected === it.name}
							aria-label={it.held
								? `${it.name}, ${it.held.summed_qty} guild-wide, ${it.held.holder_count} ${holderWord(it.held.holder_count)}`
								: `${it.name}, not held in the guild`}
							onclick={() => select(it.name)}
						>
							<span class="ico ico-sm" style={`--tile-hue: ${hueFor(it.name, it.icon_id)};`}>
								{#if it.icon_id > 0}
									<!-- icon_id is a TRUSTED integer (T-32-09) — the only dynamic part of
									     the src is the number, never a guildie string. -->
									<img
										src={`https://wiki.project1999.com/images/Item_${it.icon_id}.png`}
										alt=""
										class="icon-img"
										onerror={onImgError}
									/>
								{/if}
							</span>
							<span class="row-body">
								<span class="row-top">
									<span class="row-name">{it.name}</span>
									{#if it.held?.is_mine}<span class="tag">you</span>{/if}
								</span>
								{#if it.held}
									<span class="row-headline">
										<span class="num">{it.held.summed_qty}</span>
										<span class="dot">·</span>
										<span class="num">{it.held.holder_count}</span>
										<span class="unit">{holderWord(it.held.holder_count)}</span>
									</span>
								{:else}
									<!-- Catalog-only (unheld) row: the holder count slot reads the
									     UI-SPEC "not held in the guild" line (Body prose, dimmed). -->
									<span class="row-unheld">not held in the guild</span>
								{/if}
							</span>
							<span class="row-trail">
								{#if it.price != null}
									<span class="row-price">{fmtPp(it.price)}pp</span>
								{/if}
								{#if it.wiki_url}
									<!-- The wiki link must NOT activate the row select (stopPropagation). -->
									<a
										class="row-wiki"
										href={it.wiki_url}
										target="_blank"
										rel="noopener"
										onclick={(e) => e.stopPropagation()}>Wiki ↗</a
									>
								{/if}
							</span>
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Right: the single pinned detail panel. Until an item is selected, the §I
		     "Pick an item" prompt; then the detail header + reused ExaminePanel +
		     holders table. Replace-on-click (D-03a). -->
		<div class="detail-col">
			{#if selectedRow === null}
				<div class="prompt">
					<h2 class="prompt-heading">Pick an item</h2>
					<p class="prompt-body">
						Choose an item from the list to see who in the guild has it — character, slot, quantity,
						and last-synced.
					</p>
				</div>
			{:else}
				<div class="detail">
					<!-- Detail header: 40px icon + item NAME (Heading 20px accent) + the
					     qty/holder summary meta (price/wiki ship in the ExaminePanel below).
					     A held item shows the guild-wide summary; an unheld catalog item shows
					     the "not held in the guild" line (D-04). -->
					<div class="detail-header">
						<span
							class="ico ico-lg"
							style={`--tile-hue: ${hueFor(selectedRow.name, selectedRow.icon_id)};`}
						>
							<!-- {#key} the detail-header icon on the selected item so the shared <img>
							     node is RECREATED on each selection: onImgError sets display:none
							     imperatively and Svelte would otherwise keep that stale hide when the
							     src updates to a NEXT item whose icon loads fine (CR-WR-01). The list
							     rows are immune (keyed per-name → one <img> each). -->
							{#key selectedRow.name}
								{#if selectedRow.icon_id > 0}
									<img
										src={`https://wiki.project1999.com/images/Item_${selectedRow.icon_id}.png`}
										alt=""
										class="icon-img"
										onerror={onImgError}
									/>
								{/if}
							{/key}
						</span>
						<div class="detail-head-text">
							<h2 class="detail-name">{selectedRow.name}</h2>
							{#if selectedRow.held}
								<p class="detail-meta">
									<span class="num">{selectedRow.held.summed_qty}</span> guild-wide across
									<span class="num">{selectedRow.held.holder_count}</span>
									{holderWord(selectedRow.held.holder_count)}
								</p>
							{:else}
								<p class="detail-meta detail-unheld">not held in the guild</p>
							{/if}
						</div>
					</div>

					<!-- The examine block (D-03a): REUSED ExaminePanel, UNCHANGED. charLastSeen=""
					     omits the footer (last-synced is per-holder, in the table below). The single
					     sanctioned escaped raw-HTML composeItemNote sink lives INSIDE this component.
					     The .examine-wrap override drops ExaminePanel's sticky positioning IN THIS
					     TAB ONLY (scoped :global) — here the examine is followed by the holders table
					     in the SAME scroll column, so a sticky panel slides over the holder rows on
					     scroll and hides who-holds-it (browser-smoke 2026-06-18). P31's character-
					     window usage of ExaminePanel keeps its sticky positioning. -->
					<div class="examine-wrap">
						<ExaminePanel slot={asSlot} charLastSeen="" />
					</div>

					<!-- HOLDERS (ITEM-03): one row per holding, deep-linking into /characters?c=.
					     A held item shows its holders in BOTH scopes (D-04); an unheld catalog
					     item reads "not held in the guild" in place of the table. -->
					<p class="holders-eyebrow">HOLDERS</p>
					{#if selectedHolders.length === 0}
						<p class="holders-empty">
							{selectedRow.held ? 'No holders' : 'not held in the guild'}
						</p>
					{:else}
						<div class="holders" role="table" aria-label="Holders">
							<div class="holders-head" role="row">
								<span role="columnheader">Character</span>
								<span role="columnheader">Where</span>
								<span class="col-qty" role="columnheader">Qty</span>
								<span role="columnheader">Last synced</span>
							</div>
							<!-- Key includes the index: slotLabel collapses every bagged copy to the
							     literal "Bag", so one char holding the same stackable in two bags yields
							     two holders with an IDENTICAL {char, slot_label} — keying on that alone
							     duplicate-keys and crashes the panel (each_key_duplicate, CR-01). The
							     index disambiguates the two genuine holdings. -->
							{#each sortHolders(selectedHolders) as h, i (`${h.char} ${h.slot_label} ${i}`)}
								<a
									class="holder-row"
									role="row"
									href={`/characters?c=${encodeURIComponent(h.char)}`}
									aria-label={`View ${h.char}`}
								>
									<span class="holder-char" role="cell">
										<!-- Plain {} — Svelte auto-escapes; NO raw-HTML directive (T-32-07). -->
										<span class="holder-name">{h.char}</span>
										{#if h.is_mine}<span class="tag">you</span>{/if}
										{#if h.is_bank}<span class="tag">bank</span>{/if}
									</span>
									<span class="holder-where" role="cell">{h.slot_label}</span>
									<span class="holder-qty col-qty" role="cell">×{h.qty}</span>
									<span class="holder-synced" role="cell">
										<!-- Friendly date + freshness dot (matches view/bank) instead of the
										     raw ISO string / misleading 00:00:00Z (WR-02). -->
										<LastSyncedCell lastSynced={h.last_synced} />
									</span>
								</a>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	/* Two-pane master-detail: list (left) + detail (right). Desktop side-by-side;
	   mobile stacks (§E). Gutters inherited from shell-main (32px / 16px). The list
	   column is a touch wider than /characters (item names run longer, §A). */
	.inventory {
		display: grid;
		grid-template-columns: minmax(300px, 380px) 1fr;
		gap: 24px; /* lg — detail column gap (§Spacing) */
		align-items: start;
	}

	/* --- Scoped search (§D — reused verbatim from /characters .search) --- */
	.search {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 44px; /* touch target */
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
		font-size: 13px; /* Label */
		color: var(--text);
		opacity: 0.7;
	}
	/* The sparse-catalog "still filling in" reassurance under the no-results state
	   (SC-4 graceful degradation — Body prose, dimmed). */
	.catalog-hint {
		margin: 8px 0 0;
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text);
		opacity: 0.7;
		text-align: center;
	}

	/* --- Phase 39 control bar: scope segmented control + facet chips (UI-SPEC §1) --- */
	.control-bar {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 16px; /* md — separates the scope cluster from the facet cluster */
		margin: 8px 0 0;
	}
	/* The .seg/.seg-btn segmented control — DUPLICATED verbatim from
	   guild-views/+page.svelte:459-494 (Svelte styles are component-scoped, so the
	   classes must live here too; the sanctioned precedent). Token-only → 5-theme
	   parity for free. The :disabled rule is DROPPED — scope/facets are never disabled
	   here (UI-SPEC §1 states). */
	.seg {
		display: inline-flex;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		overflow: hidden;
	}
	.seg-btn {
		min-height: 44px; /* touch target */
		padding: 8px 16px;
		background: var(--panel);
		border: none;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		cursor: pointer;
	}
	.seg-btn + .seg-btn {
		border-left: 1px solid var(--border, var(--accent));
	}
	.seg-btn.active {
		background: var(--accent);
		color: var(--bg);
	}
	.seg-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}

	/* --- The bespoke viewer-first item list (§B) --- */
	.rows {
		display: flex;
		flex-direction: column;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px; /* sm */
		width: 100%;
		min-height: 44px; /* touch target */
		padding: 8px 16px; /* sm × md */
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
	.row-body {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1 1 auto;
		min-width: 0;
	}
	.row-top {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.row-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px; /* Body */
		line-height: 1.3;
	}
	.row-headline {
		display: flex;
		align-items: baseline;
		gap: 4px; /* xs */
		font-size: 13px; /* Label */
		line-height: 1.2;
	}
	/* Catalog-only (unheld) row headline — the UI-SPEC "not held in the guild" line
	   (Body prose, dimmed) in place of the holder count. */
	.row-unheld {
		font-family: var(--font-body);
		font-size: 13px;
		line-height: 1.2;
		color: var(--text);
		opacity: 0.7;
	}
	.num {
		color: var(--accent);
		font-variant-numeric: tabular-nums;
		font-size: 16px;
	}
	.dot {
		color: var(--text);
		opacity: 0.5;
	}
	.unit {
		color: var(--text);
		opacity: 0.7;
	}
	.row-trail {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 0 0 auto;
		flex-wrap: wrap;
		justify-content: flex-end;
	}
	.row-price {
		color: var(--status-other);
		font-variant-numeric: tabular-nums;
		font-size: 16px;
	}
	.row-wiki {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		text-decoration: none;
		font-size: 13px;
		white-space: nowrap;
	}
	.row-wiki:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.tag {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--accent);
		opacity: 0.85;
	}

	/* --- The small item-icon tile (PaperdollSlot .ico mechanic, §Color) --- */
	.ico {
		position: relative;
		flex: 0 0 auto;
		border-radius: 3px;
		overflow: hidden;
		/* The deterministic colored-tile fallback (D-02 — the ONE sanctioned non-token
		   color, a per-item hue). A loaded <img> covers it; an onerror/absent icon
		   leaves the gradient showing. */
		background-image: linear-gradient(
			135deg,
			hsl(var(--tile-hue) 45% 30%),
			hsl(calc(var(--tile-hue) + 40) 40% 18%)
		);
	}
	.ico-sm {
		width: 32px;
		height: 32px;
	}
	.ico-lg {
		width: 40px;
		height: 40px;
	}
	.icon-img {
		width: 100%;
		height: 100%;
		image-rendering: pixelated;
		object-fit: contain;
	}

	/* --- The detail column (§F) --- */
	.detail-col {
		min-height: 200px;
	}
	.detail {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg — header → examine → holders */
	}
	.detail-header {
		display: flex;
		align-items: center;
		gap: 8px; /* sm */
	}
	.detail-head-text {
		min-width: 0;
	}
	.detail-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading */
		line-height: 1.2;
		color: var(--accent);
		margin: 0;
		overflow-wrap: anywhere;
	}
	.detail-meta {
		margin: 4px 0 0;
		font-family: var(--font-body);
		font-size: 16px; /* Body */
		line-height: 1.4;
		color: var(--text);
		opacity: 0.85;
	}
	/* The detail-header unheld line (catalog scope, D-04) — dimmer than the held meta. */
	.detail-unheld {
		opacity: 0.7;
	}

	/* Drop ExaminePanel's sticky positioning IN THIS TAB ONLY (scoped :global targets the
	   reused component's internal .examine). Here the examine is stacked ABOVE the holders
	   table in one scroll column, so `position: sticky` pinned it and it slid over the holder
	   rows on scroll, hiding who-holds-the-item (browser-smoke 2026-06-18). Static = the
	   examine scrolls with the page and the holders below it stay reachable. max-height/overflow
	   reset because the viewport cap was only there to bound the sticky panel. */
	.examine-wrap :global(.examine) {
		position: static;
		max-height: none;
		overflow: visible;
	}

	/* --- HOLDERS section (§F.3 / the holders table) --- */
	.holders-eyebrow {
		margin: 0 0 8px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.holders-empty {
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text);
		opacity: 0.7;
	}
	.holders {
		display: flex;
		flex-direction: column;
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		overflow: hidden;
	}
	.holders-head {
		display: grid;
		grid-template-columns: 1.4fr 1.2fr auto 1fr;
		gap: 8px;
		padding: 8px 16px;
		background: var(--panel);
		border-bottom: 1px solid var(--border, var(--accent));
	}
	.holders-head span {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.holder-row {
		display: grid;
		grid-template-columns: 1.4fr 1.2fr auto 1fr;
		gap: 8px;
		align-items: center;
		min-height: 44px; /* touch target — the deep-link */
		padding: 8px 16px;
		background: var(--panel);
		border-bottom: 1px solid var(--border, var(--accent));
		color: var(--text);
		text-decoration: none;
		font-family: var(--font-body);
	}
	.holder-row:last-child {
		border-bottom: none;
	}
	.holder-row:hover {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.holder-row:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.holder-char {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		min-width: 0;
	}
	.holder-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px; /* Body */
		overflow-wrap: anywhere;
	}
	.holder-where {
		font-size: 16px; /* Body */
		opacity: 0.7;
	}
	.col-qty {
		justify-self: end;
		text-align: right;
	}
	.holder-qty {
		color: var(--accent);
		font-variant-numeric: tabular-nums;
		font-size: 16px;
	}
	.holder-synced {
		font-size: 16px; /* Body */
		opacity: 0.7;
	}

	/* --- The "pick an item" prompt (§I) --- */
	.prompt {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 48px 16px; /* 2xl / md */
		text-align: center;
		opacity: 0.85;
	}
	.prompt-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading */
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

	/* Mobile: list + detail stack (§E) — same breakpoint /characters uses. */
	@media (max-width: 640px) {
		.inventory {
			grid-template-columns: 1fr;
		}
		.holders-head {
			display: none;
		}
		.holder-row {
			grid-template-columns: 1fr auto;
			grid-auto-rows: min-content;
		}
		.holder-where {
			grid-column: 1 / -1;
		}
		.holder-synced {
			grid-column: 1 / -1;
		}
	}
</style>
