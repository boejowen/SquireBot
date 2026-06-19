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

	import { onMount, getContext } from 'svelte';
	import Search from '@lucide/svelte/icons/search';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import ExaminePanel from '$lib/components/ExaminePanel.svelte';
	import LastSyncedCell from '$lib/components/cells/LastSyncedCell.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
	import {
		Unauthenticated,
		Forbidden,
		fetchItems,
		type ItemRollup,
		type InventorySlot
	} from '$lib/api';
	import { filterItems, sortHolders } from '$lib/items';

	type Status = 'loading' | 'error' | 'ready';

	// The AuthGate guard from context (server-truth re-routing on a 401/403).
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	let status = $state<Status>('loading');
	let items = $state<ItemRollup[]>([]);
	let query = $state('');
	// The NORMALIZED name of the pinned item (replace-on-click, D-03a).
	let selected = $state<string | null>(null);

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

	// The filtered, viewer-first item list (ITEM-02 / D-02 — keeps is_mine-first then
	// A-Z among matches). Pure helper from $lib/items (node-tested).
	let shown = $derived(filterItems(items, query));

	// Non-empty list after a ready load with zero matches AND a non-empty query → the
	// no-results state (vs. the "No items yet" empty state when the list is empty).
	let noResults = $derived(status === 'ready' && shown.length === 0 && query.trim() !== '');

	// The currently-pinned rollup (or null when nothing is selected / it's gone).
	let selectedRollup = $derived(items.find((r) => r.name === selected) ?? null);

	// The examine reuse seam (load-bearing — UI-SPEC §C): a representative
	// InventorySlot-shaped object so <ExaminePanel> renders UNCHANGED. The
	// list-context-irrelevant fields (count/slots/children/canonical_slot/location)
	// are zero/empty — examine ignores them. category MUST be the literal 'general'
	// (the union member, not a bare string). charLastSeen="" omits the footer (the
	// per-holder last-synced lives in the holders table, ITEM-03).
	let asSlot = $derived<InventorySlot | null>(
		selectedRollup
			? {
					item: selectedRollup.name,
					icon_id: selectedRollup.icon_id,
					statsblock: selectedRollup.statsblock,
					wiki_summary: selectedRollup.wiki_summary,
					is_quest_item: selectedRollup.is_quest_item,
					price: selectedRollup.price,
					prices: selectedRollup.prices,
					wiki_url: selectedRollup.wiki_url,
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
			<p class="search-hint">Items on your characters match first.</p>

			{#if noResults}
				<StateBlock kind="no-results" {query} />
			{:else}
				<!-- Single viewer-first-then-A-Z run — no band group-labels (§B). -->
				<div class="rows">
					{#each shown as it (it.name)}
						<button
							type="button"
							class="row"
							class:selected={selected === it.name}
							aria-pressed={selected === it.name}
							aria-label={`${it.name}, ${it.summed_qty} guild-wide, ${it.holder_count} ${holderWord(it.holder_count)}`}
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
									{#if it.is_mine}<span class="tag">you</span>{/if}
								</span>
								<span class="row-headline">
									<span class="num">{it.summed_qty}</span>
									<span class="dot">·</span>
									<span class="num">{it.holder_count}</span>
									<span class="unit">{holderWord(it.holder_count)}</span>
								</span>
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
			{#if selectedRollup === null}
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
					     qty/holder summary meta (price/wiki ship in the ExaminePanel below). -->
					<div class="detail-header">
						<span
							class="ico ico-lg"
							style={`--tile-hue: ${hueFor(selectedRollup.name, selectedRollup.icon_id)};`}
						>
							<!-- {#key} the detail-header icon on the selected item so the shared <img>
							     node is RECREATED on each selection: onImgError sets display:none
							     imperatively and Svelte would otherwise keep that stale hide when the
							     src updates to a NEXT item whose icon loads fine (CR-WR-01). The list
							     rows are immune (keyed per-name → one <img> each). -->
							{#key selectedRollup.name}
								{#if selectedRollup.icon_id > 0}
									<img
										src={`https://wiki.project1999.com/images/Item_${selectedRollup.icon_id}.png`}
										alt=""
										class="icon-img"
										onerror={onImgError}
									/>
								{/if}
							{/key}
						</span>
						<div class="detail-head-text">
							<h2 class="detail-name">{selectedRollup.name}</h2>
							<p class="detail-meta">
								<span class="num">{selectedRollup.summed_qty}</span> guild-wide across
								<span class="num">{selectedRollup.holder_count}</span>
								{holderWord(selectedRollup.holder_count)}
							</p>
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

					<!-- HOLDERS (ITEM-03): one row per holding, deep-linking into /characters?c=. -->
					<p class="holders-eyebrow">HOLDERS</p>
					{#if selectedRollup.holders.length === 0}
						<p class="holders-empty">No holders</p>
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
							{#each sortHolders(selectedRollup.holders) as h, i (`${h.char} ${h.slot_label} ${i}`)}
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
