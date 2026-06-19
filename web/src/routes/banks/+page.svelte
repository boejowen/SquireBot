<script lang="ts">
	// /banks — the Banks tab (Phase 33, BANK-01/02/03). Replaces the Phase-30 "coming
	// soon" placeholder. The guild-wide answer to "what's in the guild banks, and
	// what's it worth?": a guild-wide valuation summary header (D-02), an A-Z bank/bot
	// list (D-01) whose left column TOGGLES to an item-search scoped to bank holders
	// (D-03), and a per-bank value/plat header above the REUSED P31 InventoryWindow
	// (D-04). Master-detail on ONE route (the CLAUDE.md consolidated-views lock is
	// RELAXED for exactly this pattern): one reusable detail rendered for the SELECTED
	// bank. Selection is URL-reflected via ?b=<bankName> (history.replaceState — no
	// per-bank route file, one reusable window).
	//
	// Structurally this is the Phase-32 /inventory master-detail reskinned to banks +
	// a valuation header: the list/search/holders come from inventory/+page.svelte and
	// the per-bank window column comes from characters/+page.svelte (the winStatus /
	// stale-drop machine). The reused InventoryWindow + StateBlock + LastSyncedCell are
	// mounted UNCHANGED. The only new web algorithm is $lib/banks's is_bank holder
	// filter with the bank-slice qty recompute (node-tested in banks.test.ts).
	//
	// The pure sort/search ($lib/banks) is node-tested; the list/search/selection +
	// detail DOM here is NOT covered by node vitest (DOM-blind) — its browser
	// verification is the 33-03 deploy-then-browser-smoke.
	//
	// SECURITY (T-31/32 carry-forward): bank + item + char + slot names render via plain
	// {} interpolation (Svelte auto-escapes) in the summary header, the bank-list rows,
	// the per-bank detail header, and the item-search results — this file uses NO
	// raw-HTML directive. The ONE sanctioned escaped raw-HTML sink (composeItemNote)
	// lives INSIDE the reused ExaminePanel (transitively, via InventoryWindow) — no new
	// sink. Any bank name that round-trips through ?b= is encodeURIComponent'd.

	import { onMount, getContext } from 'svelte';
	import Search from '@lucide/svelte/icons/search';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import InventoryWindow from '$lib/components/InventoryWindow.svelte';
	import LastSyncedCell from '$lib/components/cells/LastSyncedCell.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
	import {
		Unauthenticated,
		Forbidden,
		fetchBanks,
		fetchItems,
		fetchInventory,
		type BanksView,
		type ItemRollup,
		type CharacterInventory
	} from '$lib/api';
	import { sortBanksAZ, bankItemSearch, sortBankHolders, bankByName } from '$lib/banks';

	type Status = 'loading' | 'error' | 'ready' | 'empty';
	type WinStatus = 'loading' | 'error' | 'ready';

	// The AuthGate guard from context (server-truth re-routing on a 401/403).
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	let status = $state<Status>('loading');
	let banksView = $state<BanksView | null>(null);
	// The P32 item rollup, fetched once for the BANK-03 search (client-filtered to banks).
	let items = $state<ItemRollup[]>([]);
	let query = $state('');
	// The name of the pinned bank (replace-on-click).
	let selected = $state<string | null>(null);

	// Window-scoped state machine (copied from /characters): selecting a bank fetches
	// its inventory into its OWN loading/error/ready inside the window column (the bank
	// list stays put). `invFor` tracks which bank `inv` belongs to so a stale in-flight
	// response for a previously-selected bank can't overwrite the window.
	let winStatus = $state<WinStatus>('loading');
	let inv = $state<CharacterInventory | null>(null);
	let invFor = $state<string | null>(null);

	async function load() {
		status = 'loading';
		try {
			// The list/summary AND the item rollup for the search — both session-gated.
			const [bv, its] = await Promise.all([fetchBanks(), fetchItems()]);
			banksView = bv;
			items = its;
			if (bv.banks.length === 0) {
				status = 'empty';
				selected = null;
				return;
			}
			// A Retry/refetch (or a stale ?b=) can point at a bank that's gone — clear it
			// so the detail column doesn't stick on a now-absent bank.
			const sel = selected;
			if (sel && !bv.banks.some((b) => b.name === sel)) {
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

	function refetch() {
		void load();
	}

	// One-shot initial fetch + pre-select ?b=<bankName> if present (deep-link). onMount
	// is the correct primitive for a fire-once load (a bare $effect would re-fire). The
	// ?b= value pre-selects a bank so a shared link opens its window directly.
	onMount(() => {
		const b = new URLSearchParams(window.location.search).get('b');
		if (b) selected = b;
		void load();
	});

	// The left-column content toggles off query emptiness (D-03): empty → the A-Z bank
	// list; non-empty → the is_bank-scoped item-search results. Pure helpers from
	// $lib/banks (node-tested).
	let bankRows = $derived(banksView ? sortBanksAZ(banksView.banks) : []);
	let searching = $derived(query.trim() !== '');
	let searchResults = $derived(searching ? bankItemSearch(items, query) : []);

	// Non-empty rollup after a ready load with zero matches AND a non-empty query → the
	// no-results state (shown in the LEFT column in place of the results).
	let noResults = $derived(status === 'ready' && searching && searchResults.length === 0);

	// The selected bank's summary row (value/plat for the D-04 header) — read off the
	// already-loaded list, no second fetch.
	let selectedBank = $derived(
		banksView && selected ? (bankByName(banksView.banks, selected) ?? null) : null
	);

	function select(name: string) {
		selected = name;
		// URL-reflect the selection (?b=<bankName>) WITHOUT a full navigation or a new
		// route file — deep-linkable + shareable; the single reusable detail renders for
		// the selected bank. encodeURIComponent because bank names are guildie-controlled.
		// history.replaceState keeps focus + scroll (no reload).
		if (typeof history !== 'undefined') {
			const url = new URL(window.location.href);
			url.searchParams.set('b', name);
			history.replaceState(history.state, '', url);
		}
	}

	// Fetch the selected bank's inventory into the window column's own state machine. A
	// 401/403 routes to the AuthGate guard (same server-truth re-route as the page load);
	// any other failure stays the in-window error StateBlock + Retry. `bank` is captured
	// so a late response for a since-changed selection is dropped.
	async function loadInventory(bank: string) {
		winStatus = 'loading';
		invFor = bank;
		try {
			const got = await fetchInventory(bank);
			// Drop a stale response (the user picked another bank while this was in flight).
			if (invFor !== bank) return;
			inv = got;
			winStatus = 'ready';
		} catch (err) {
			if (invFor !== bank) return;
			if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
				authGuard(err);
			} else {
				winStatus = 'error';
			}
		}
	}

	function retryInventory() {
		if (selected) void loadInventory(selected);
	}

	// Drive the window fetch off the selection. A bare $effect re-runs when `selected`
	// changes; the guard skips the null (no bank) case so the "pick a bank" prompt shows.
	$effect(() => {
		const sel = selected;
		if (sel === null) {
			inv = null;
			invFor = null;
			return;
		}
		// Only (re)fetch when the selection differs from what the window holds.
		if (invFor !== sel) void loadInventory(sel);
	});

	// True when the selected bank's fetched inventory has zero items anywhere (equipment
	// + general + bank) — the "no inventory synced yet" state (a coin-only bank).
	let noInventory = $derived(
		winStatus === 'ready' &&
			inv !== null &&
			inv.equipment.length === 0 &&
			inv.general.length === 0 &&
			inv.bank.length === 0
	);

	// Round + comma-group a value/platinum number for the summary + detail headers
	// (matches the examine's formatPp posture). A real integer/sum — never null.
	function fmtNum(n: number): string {
		return Math.round(n).toLocaleString('en-US');
	}

	// The item-count word for a bank-list row (singular "1 item").
	function itemWord(n: number): string {
		return n === 1 ? 'item' : 'items';
	}
</script>

<svelte:head>
	<title>SquireBot — Banks</title>
</svelte:head>

{#if status === 'loading'}
	<StateBlock kind="loading" />
{:else if status === 'error'}
	<StateBlock kind="error" onRetry={refetch} />
{:else if status === 'empty'}
	<!-- No IsBankToon || IsGuildBot character designated/synced yet (UI-SPEC §H). -->
	<StateBlock kind="no-bank-toons" />
{:else if banksView !== null}
	<!-- D-02: the guild-wide valuation summary header, spanning the content width above
	     the two-pane grid. Numbers ride accent; the unit words + "·" are dimmed. Real
	     sums → a clean "0 pp" / "0 plat", never "null". -->
	<div class="summary" aria-label="Guild banks valuation">
		<p class="summary-eyebrow">GUILD BANKS</p>
		<p class="summary-line">
			<span class="num">{fmtNum(banksView.guild_value)}</span>
			<span class="unit">pp</span>
			<span class="dot">·</span>
			<span class="num">{fmtNum(banksView.total_platinum)}</span>
			<span class="unit">plat</span>
		</p>
	</div>

	<div class="banks">
		<!-- Left: the scoped item-search bar + the list/search TOGGLE (D-03). Empty query
		     → the A-Z bank list; non-empty → the is_bank-scoped item-search results. -->
		<div class="list-col">
			<div class="search">
				<Search size={16} aria-hidden="true" class="search-icon" />
				<input
					class="search-input"
					type="search"
					placeholder="Search bank items…"
					aria-label="Search bank items"
					bind:value={query}
				/>
			</div>
			<p class="search-hint">Search items held by the guild banks.</p>

			{#if !searching}
				<!-- BANK LIST mode (§C / D-01): single A-Z run, name + item count + a "bank"
				     tag marking why the row is in this banks-only list. NO per-row value/plat. -->
				<div class="rows">
					{#each bankRows as b (b.name)}
						<button
							type="button"
							class="row"
							class:selected={selected === b.name}
							aria-pressed={selected === b.name}
							aria-label={`${b.name}, ${b.item_count} ${itemWord(b.item_count)}`}
							onclick={() => select(b.name)}
						>
							<span class="row-main">
								<!-- Plain {} — Svelte auto-escapes; NO raw-HTML directive. -->
								<span class="row-name">{b.name}</span>
								<span class="tag">bank</span>
							</span>
							<span class="row-headline">
								<span class="num">{b.item_count}</span>
								<span class="unit">{itemWord(b.item_count)}</span>
							</span>
						</button>
					{/each}
				</div>
			{:else if noResults}
				<StateBlock kind="no-results" {query} />
			{:else}
				<!-- ITEM-SEARCH mode (§F.2 / D-03): per matched bank item, the item name +
				     holder rows (each an in-tab deep-link pinning that bank's window). -->
				<div class="results">
					{#each searchResults as it (it.name)}
						<div class="result-group">
							<p class="result-name">{it.name}</p>
							<div class="holders" role="table" aria-label={`Banks holding ${it.name}`}>
								<div class="holders-head" role="row">
									<span role="columnheader">Bank</span>
									<span role="columnheader">Where</span>
									<span class="col-qty" role="columnheader">Qty</span>
									<span role="columnheader">Last synced</span>
								</div>
								<!-- The index disambiguates two genuine holdings on one bank that
								     collapse to the same {char, slot_label} (each_key_duplicate guard,
								     mirrors inventory/+page.svelte). -->
								{#each sortBankHolders(it.holders) as h, i (`${h.char} ${h.slot_label} ${i}`)}
									<!-- THE in-tab deep-link (§F.3 / D-03): a button calling the SAME
									     select(bankName) the list rows call — pins THAT bank's window in
									     the right column, NO route change (unlike P32's cross-tab jump).
									     Selecting a holder does NOT clear the query. -->
									<button
										type="button"
										class="holder-row"
										aria-label={`Open ${h.char}'s window`}
										onclick={() => select(h.char)}
									>
										<span class="holder-char" role="cell">
											<span class="holder-name">{h.char}</span>
										</span>
										<span class="holder-where" role="cell">{h.slot_label}</span>
										<span class="holder-qty col-qty" role="cell">×{h.qty}</span>
										<span class="holder-synced" role="cell">
											<LastSyncedCell lastSynced={h.last_synced} />
										</span>
									</button>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Right: the per-bank detail column. Selection (?b=) drives a window-scoped
		     fetch; the column shows its own loading/error/no-inventory states, then the
		     D-04 per-bank header ABOVE the reused InventoryWindow. -->
		<div class="window-col">
			{#if selected === null}
				<!-- "Pick a bank" prompt — not an error (§I). -->
				<div class="prompt">
					<h2 class="prompt-heading">Pick a bank</h2>
					<p class="prompt-body">
						Choose a guild bank from the list to see its stored items and value.
					</p>
				</div>
			{:else if winStatus === 'loading'}
				<StateBlock kind="loading" />
			{:else if winStatus === 'error'}
				<StateBlock kind="error" onRetry={retryInventory} />
			{:else if noInventory}
				<!-- A coin-only / freshly-flagged / mid-upload bank: it still lists (its
				     coin still counts), but has no synced inventory yet (§G). The D-04
				     header still renders above this prompt so the per-bank value/plat shows. -->
				<div class="detail-header detail-header--standalone">
					<h2 class="detail-name">{selected}</h2>
					{#if selectedBank}
						<p class="detail-meta">
							<span class="num">{fmtNum(selectedBank.value)}</span>
							<span class="unit">pp</span>
							<span class="dot">·</span>
							{#if selectedBank.plat === null}
								<span class="unit">not recorded</span>
							{:else}
								<span class="num">{fmtNum(selectedBank.plat)}</span>
								<span class="unit">plat</span>
							{/if}
						</p>
					{/if}
				</div>
				<div class="prompt">
					<h2 class="prompt-heading">No inventory synced yet</h2>
					<p class="prompt-body">
						{selected} hasn't uploaded inventory yet. Once its watcher syncs, its stored items show
						up here.
					</p>
				</div>
			{:else if inv !== null}
				<!-- D-04: the per-bank value/plat header ABOVE the reused window. Reads its
				     numbers off the already-loaded BanksView.banks (bankByName — no second
				     fetch). Nil plat → "not recorded" (NEVER "0 plat"); a recorded 0 reads
				     "0 plat"; value is a real sum → "0 pp" clean when unpriced. -->
				{#if selectedBank}
					<div class="detail-header">
						<h2 class="detail-name">{selectedBank.name}</h2>
						<p class="detail-meta">
							<span class="num">{fmtNum(selectedBank.value)}</span>
							<span class="unit">pp</span>
							<span class="dot">·</span>
							{#if selectedBank.plat === null}
								<span class="unit">not recorded</span>
							{:else}
								<span class="num">{fmtNum(selectedBank.plat)}</span>
								<span class="unit">plat</span>
							{/if}
						</p>
					</div>
				{/if}
				<InventoryWindow inventory={inv} />
			{/if}
		</div>
	</div>
{/if}

<style>
	/* --- D-02 valuation summary header (§B) — a --panel band spanning the content
	   width above the two-pane grid. The ONE net-new sub-block (no whole-file analog);
	   the number/unit treatment mirrors inventory/+page.svelte's .num/.unit. --- */
	.summary {
		display: flex;
		flex-direction: column;
		gap: 8px; /* sm */
		padding: 16px; /* md */
		margin-bottom: 24px; /* lg into the two-pane grid */
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
	}
	.summary-eyebrow {
		margin: 0;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.summary-line {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 4px; /* xs */
		margin: 0;
		font-family: var(--font-display);
		font-size: 20px; /* Heading number line */
		line-height: 1.2;
	}
	.summary-line .num {
		font-size: 20px;
	}

	/* Two-pane master-detail: list (left) + per-bank window (right). Desktop
	   side-by-side; mobile stacks (§E). Bank names are short like char names, so the
	   280–360px list column matches /characters. Gutters inherited from shell-main. */
	.banks {
		display: grid;
		grid-template-columns: minmax(280px, 360px) 1fr;
		gap: 24px; /* lg */
		align-items: start;
	}

	/* --- Scoped search (§D — reused verbatim from /inventory .search) --- */
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

	/* --- The bespoke A-Z bank list (§C — mirror /inventory .row, no icon tile) --- */
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
	.row-main {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		flex: 1 1 auto;
		min-width: 0;
	}
	.row-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px; /* Body */
		line-height: 1.3;
		overflow-wrap: anywhere;
	}
	.row-headline {
		display: flex;
		align-items: baseline;
		gap: 4px; /* xs */
		flex: 0 0 auto;
		font-size: 13px; /* Label */
		line-height: 1.2;
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

	/* --- The item-search results (§F.2 — mirror /inventory holders table) --- */
	.results {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg between item groups */
	}
	.result-group {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.result-name {
		margin: 0;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading */
		line-height: 1.2;
		color: var(--accent);
		overflow-wrap: anywhere;
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
		width: 100%;
		min-height: 44px; /* touch target — the in-tab deep-link */
		padding: 8px 16px;
		text-align: left;
		background: var(--panel);
		border: none;
		border-bottom: 1px solid var(--border, var(--accent));
		color: var(--text);
		cursor: pointer;
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

	/* --- The per-bank detail header (§F.1 / D-04) — name + value/plat meta --- */
	.window-col {
		min-height: 200px;
	}
	.detail-header {
		margin-bottom: 24px; /* lg into the window */
	}
	.detail-header--standalone {
		margin-bottom: 16px; /* md before the no-inventory prompt */
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
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 4px; /* xs */
		margin: 4px 0 0;
		font-family: var(--font-body);
		font-size: 16px; /* Body */
		line-height: 1.4;
	}
	.detail-meta .unit {
		opacity: 0.85;
	}

	/* --- The "pick a bank" prompt (§I) --- */
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

	/* Mobile: list + window stack (§E) — same breakpoint /inventory + /characters use. */
	@media (max-width: 640px) {
		.banks {
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
