<script lang="ts">
	// /guild-views — the PRESERVED consolidated 4-view home (Phase 30 / D-03b/D-04).
	// This is today's home `+page.svelte` moved here VERBATIM (the four consolidated
	// views + cross-character search), kept reachable so guild-wide lookup never
	// regresses during the v2.4 tab transition (the Characters/Inventory/Banks
	// placeholders link here until Phases 31-33 replace it; retired then, NOT now).
	// `/` redirects here (D-04). The ?view= seed still works at this path
	// (/guild-views?view=bank deep-links + the "Record coin" round-trip).
	//
	// Wired to the Go read API (WEB-01/03/04, BACKEND-05 consumption). On mount it
	// fetches all four payloads + meta in parallel and holds loading / error / ready
	// state. A nav switches between the four reusable DataGrid instances (ONE
	// component, 4 instances — never per-character tabs, CLAUDE.md LOCKED), each fed
	// its payload + the matching columns.ts def + the view's multi-key default sort.
	// SearchBox runs over the in-memory `view` rows (D-03). bank shows the recorded
	// coin (or the "not yet recorded" affordance). Fetch failure -> error StateBlock
	// with a working Retry; empty payload -> the per-view-empty StateBlock; loading
	// -> the skeleton.

	import { onMount, getContext } from 'svelte';
	import DataGrid from '$lib/components/DataGrid.svelte';
	import SearchBox from '$lib/components/SearchBox.svelte';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import StatusLegend from '$lib/components/StatusLegend.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
	import { Unauthenticated, Forbidden } from '$lib/api';
	import {
		fetchView,
		fetchGearCheck,
		fetchSpellCheck,
		fetchBank,
		fetchMeta,
		fetchBankToons,
		fetchMyCharacters,
		type ViewRow,
		type GearCheckRow,
		type SpellCheckRow,
		type MetaResponse,
		type BankToon,
		type MyCharacter
	} from '$lib/api';
	import { myCharNameSet, applyMyFilter } from '$lib/myview';
	import { hasRecordedCoin } from '$lib/coin';
	import {
		viewColumns,
		gearCheckColumns,
		spellCheckColumns,
		bankColumns
	} from '$lib/columns';
	import type { SortingState } from '@tanstack/table-core';

	type ViewId = 'view' | 'gear_check' | 'spell_check' | 'bank';
	type Status = 'loading' | 'error' | 'ready';

	const TABS: { id: ViewId; label: string; friendly: string }[] = [
		{ id: 'view', label: 'Inventory', friendly: 'inventory' },
		{ id: 'gear_check', label: 'Gear Check', friendly: 'gear check' },
		{ id: 'spell_check', label: 'Spell Check', friendly: 'spell check' },
		{ id: 'bank', label: 'Bank', friendly: 'bank' }
	];

	// Per-view multi-key default sort (UI-SPEC secondary sorts; column ids from
	// columns.ts). Tier sorts by the custom rank fn registered on that column.
	const SORT: Record<ViewId, SortingState> = {
		view: [
			{ id: 'char', desc: false },
			{ id: 'item', desc: false },
			{ id: 'slot', desc: false }
		],
		bank: [
			{ id: 'char', desc: false },
			{ id: 'item', desc: false },
			{ id: 'slot', desc: false }
		],
		gear_check: [
			{ id: 'char', desc: false },
			{ id: 'tier', desc: false },
			{ id: 'slot', desc: false },
			{ id: 'recommended', desc: false }
		],
		spell_check: [
			{ id: 'char', desc: false },
			{ id: 'level', desc: false },
			{ id: 'spell', desc: false }
		]
	};

	// The AuthGate guard from context (server-truth re-routing on a 401/403, B-2).
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	let active = $state<ViewId>('view');
	let status = $state<Status>('loading');

	let viewRows = $state<ViewRow[]>([]);
	let gearRows = $state<GearCheckRow[]>([]);
	let spellRows = $state<SpellCheckRow[]>([]);
	let bankRows = $state<ViewRow[]>([]);
	let meta = $state<MetaResponse>({ characters: [] });
	// 15-05 (D-11): the read bank endpoint's `coin` is still null (15-03 did not
	// change it), so the recorded coin is surfaced by ALSO loading the bank-toons
	// (the same login-only source BankCoinForm writes). This replaces P14's
	// "Coin: not yet recorded" placeholder once any toon has a recorded value.
	let bankToons = $state<BankToon[]>([]);
	// 27-01 (MYVIEW-01/02): the caller's assigned characters drive the additive
	// "My characters" quick-filter + single-char drill-down. `mineOnly` defaults OFF
	// so all-members visibility is unchanged for everyone (the filter narrows only
	// THIS browser's grid — never a server-side row scope; see myview.ts header /
	// T-27-01). `selectedChar` (the drill-down) dominates when set.
	let myCharacters = $state<MyCharacter[]>([]);
	let mineOnly = $state(false);
	let selectedChar = $state<string | null>(null);

	async function load() {
		status = 'loading';
		try {
			const [v, g, s, b, m, bt, mc] = await Promise.all([
				fetchView(),
				fetchGearCheck(),
				fetchSpellCheck(),
				fetchBank(),
				fetchMeta(),
				fetchBankToons(),
				fetchMyCharacters()
			]);
			viewRows = v;
			gearRows = g;
			spellRows = s;
			bankRows = b.rows;
			meta = m;
			bankToons = bt;
			myCharacters = mc;
			// IN-01 (27-REVIEW LOW): a Retry/refetch can drop the character the drill-down
			// is pinned to — clear a now-stale selection so the grids don't stick on the
			// empty state with an out-of-list <select> value.
			const sel = selectedChar;
			if (sel && !mc.some((c) => c.name.toLowerCase() === sel.toLowerCase())) {
				selectedChar = null;
			}
			status = 'ready';
		} catch (err) {
			// Server-truth (B-2): a 401/403 from ANY of the read endpoints means the
			// session is gone or refused — hand it to the AuthGate guard so the whole
			// site re-routes (401→LoginScreen, 403→matching refusal) instead of
			// showing a stale "Couldn't load" view with dead controls. Any other
			// failure (network/5xx) stays the generic error StateBlock + Retry.
			if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
				authGuard(err);
			} else {
				status = 'error';
			}
		}
	}

	// Retry re-fires the whole parallel fetch (UI-SPEC error-state Retry).
	function refetch() {
		void load();
	}

	// One-shot initial fetch. onMount is the correct primitive for a fire-once
	// load: it runs exactly once after the component mounts and never re-runs.
	// A bare $effect would re-fire the whole five-endpoint parallel fetch if
	// load() ever started reading reactive state synchronously (e.g. scoping the
	// fetch by `active` or a query param) — refactor-fragile (review WR-03).
	// refetch() (the Retry handler) calls load() directly, so the effect is not
	// needed for retry either.
	//
	// 260610-fm5 WS3: a ?view= query param seeds the active tab (the "Record coin"
	// link round-trips via /guild-views?view=bank). VALIDATED against the actual
	// TABS ids — an unknown value is ignored (T-fm5-04; the param is never rendered).
	onMount(() => {
		const v = new URLSearchParams(window.location.search).get('view');
		if (v && TABS.some((t) => t.id === v)) {
			active = v as ViewId;
		}
		void load();
	});

	// "No characters at all" — drives the top-level empty state (UI-SPEC).
	let noCharacters = $derived(
		status === 'ready' && meta.characters.length === 0 && viewRows.length === 0
	);

	// 15-05 (D-11): the bank-coin toons that actually have a recorded value drive
	// the coin summary; when none do, the P14 "not yet recorded" affordance stays.
	// 27-01 (IN-02, deliberate): this is the SHARED guild-bank coin ledger — it stays
	// guild-wide and is intentionally NOT narrowed by the "My characters" filter (the
	// filter narrows the per-character grids, not the shared bank coin summary).
	let coinToons = $derived(bankToons.filter(hasRecordedCoin));

	/** Render a toon's coin as a compact "Np Ng Ns Nc" line (null → 0; tabular). */
	function coinLine(t: BankToon): string {
		return `${t.plat ?? 0}p ${t.gold ?? 0}g ${t.silver ?? 0}s ${t.copper ?? 0}c`;
	}

	// 27-01: the additive "My characters" filter, applied client-side over the rows
	// already in memory (the four grids are fed the FILTERED arrays; the SearchBox is
	// deliberately NOT — it stays guild-wide). Default mineOnly=false / selectedChar=null
	// passes rows through UNCHANGED, so all-members visibility is the default.
	let mineNames = $derived(myCharNameSet(myCharacters));
	let filteredViewRows = $derived(applyMyFilter(viewRows, mineNames, mineOnly, selectedChar));
	let filteredGearRows = $derived(applyMyFilter(gearRows, mineNames, mineOnly, selectedChar));
	let filteredSpellRows = $derived(applyMyFilter(spellRows, mineNames, mineOnly, selectedChar));
	let filteredBankRows = $derived(applyMyFilter(bankRows, mineNames, mineOnly, selectedChar));

	// The filter is "active" (narrowing) whenever mine-only is on OR a single char is
	// selected — drives the DISTINCT "none of YOUR characters" empty copy (vs the
	// generic all-members "no data" StateBlock), so an active filter that empties a grid
	// never reads as missing data (Pitfall 3/5).
	let filterActive = $derived(mineOnly || selectedChar !== null);
	// A member who has claimed nothing: the "My characters" + per-char options are dead —
	// disable them and show a hint linking to the My-characters settings section (Pitfall 5).
	let hasMine = $derived(myCharacters.length > 0);

	// 260610-fm5 WS3: the scope control is a segmented My characters/Guild toggle
	// (the wantlist .seg pattern) + a character <select> shown only in the
	// My-characters scope. PRESENTATION ONLY — the same two filter primitives
	// (mineOnly / selectedChar) drive applyMyFilter unchanged. Default = Guild
	// (today's all-members default).
	type Scope = 'guild' | 'mine';
	let scope = $derived<Scope>(mineOnly || selectedChar !== null ? 'mine' : 'guild');

	/** Flip the segmented scope. Guild → filter OFF; My characters → mine-only
	 *  (the drill-down <select> then narrows further). */
	function setScope(s: Scope) {
		if (s === scope) return;
		if (s === 'guild') {
			mineOnly = false;
			selectedChar = null;
		} else {
			mineOnly = true;
			selectedChar = null;
		}
	}

	// The drill-down <select>'s value: '' = all MY characters; else a character name.
	let charSelectValue = $derived(selectedChar ?? '');

	/** '' → my-characters (no drill-down); a name → single-char drill-down (which
	 *  dominates — same semantics as the old merged control). */
	function onCharSelect(value: string) {
		if (value === '') {
			mineOnly = true;
			selectedChar = null;
		} else {
			selectedChar = value;
			mineOnly = false;
		}
	}
</script>

<svelte:head>
	<title>SquireBot — guild views (classic)</title>
</svelte:head>

{#if status === 'loading'}
	<StateBlock kind="loading" />
{:else if status === 'error'}
	<StateBlock kind="error" onRetry={refetch} />
{:else if noCharacters}
	<StateBlock kind="empty" />
{:else}
	<!-- Cross-character search over the in-memory view rows (D-03). -->
	<section class="search-section" aria-label="Search">
		<SearchBox rows={viewRows} />
	</section>

	<!-- 27-01 / 260610-fm5 WS3: the "My characters" scope control (MYVIEW-01 +
	     MYVIEW-02), restyled to the wantlist's segmented-toggle pattern: a
	     My characters | Guild two-button toggle + a character <select> shown only
	     in the My-characters scope. It lives in the view-orchestration layer
	     (NEVER inside DataGrid — the grid stays view-agnostic, fed pre-filtered
	     data; CLAUDE.md consolidated-views LOCK). The filter is presentation only,
	     never access control (T-27-01). -->
	<div class="filter-bar">
		<div class="seg" role="group" aria-label="Whose characters to show">
			<button
				type="button"
				class="seg-btn"
				class:active={scope === 'mine'}
				aria-pressed={scope === 'mine'}
				disabled={!hasMine}
				onclick={() => setScope('mine')}>My characters</button
			>
			<button
				type="button"
				class="seg-btn"
				class:active={scope === 'guild'}
				aria-pressed={scope === 'guild'}
				onclick={() => setScope('guild')}>Guild</button
			>
		</div>
		{#if scope === 'mine'}
			<!-- The drill-down options are sourced ONLY from fetchMyCharacters()
			     (session-scoped, IDOR-safe — T-27-03), never meta.characters. Names
			     render via plain {} (Svelte auto-escapes — never the raw-HTML
			     directive; T-27-02). -->
			<label class="filter-label" for="char-filter">Character</label>
			<select
				id="char-filter"
				class="char-filter"
				aria-label="Filter views by character"
				value={charSelectValue}
				onchange={(e) => onCharSelect(e.currentTarget.value)}
			>
				<option value="">All my characters</option>
				{#each myCharacters as c (c.character_id)}
					<option value={c.name}>{c.name}</option>
				{/each}
			</select>
		{/if}
		{#if !hasMine}
			<!-- Zero claimed characters: the control is never a dead empty toggle — point
			     the member at where they can claim (Pitfall 5). -->
			<span class="filter-hint">
				<a href="/settings">Claim characters</a> to filter to your own.
			</span>
		{/if}
	</div>

	<!-- View nav: 4 tabs over the ONE reusable DataGrid. -->
	<nav class="view-nav" aria-label="Views">
		{#each TABS as tab (tab.id)}
			<button
				class="tab"
				class:active={active === tab.id}
				type="button"
				aria-current={active === tab.id ? 'page' : undefined}
				onclick={() => (active = tab.id)}
			>
				{tab.label}
			</button>
		{/each}
	</nav>

	<section class="view-panel">
		{#if active === 'view'}
			{#if filteredViewRows.length === 0}
				{#if filterActive}
					<p class="filter-empty">None of your characters have rows in this view.</p>
				{:else}
					<StateBlock kind="view-empty" viewName="inventory" />
				{/if}
			{:else}
				<DataGrid data={filteredViewRows} columns={viewColumns} defaultSorting={SORT.view} />
			{/if}
		{:else if active === 'gear_check'}
			<StatusLegend variant="gear" />
			{#if filteredGearRows.length === 0}
				{#if filterActive}
					<p class="filter-empty">None of your characters have rows in this view.</p>
				{:else}
					<StateBlock kind="view-empty" viewName="gear check" />
				{/if}
			{:else}
				<DataGrid data={filteredGearRows} columns={gearCheckColumns} defaultSorting={SORT.gear_check} />
			{/if}
		{:else if active === 'spell_check'}
			<StatusLegend variant="spell" />
			{#if filteredSpellRows.length === 0}
				{#if filterActive}
					<p class="filter-empty">None of your characters have rows in this view.</p>
				{:else}
					<StateBlock kind="view-empty" viewName="spell check" />
				{/if}
			{:else}
				<DataGrid data={filteredSpellRows} columns={spellCheckColumns} defaultSorting={SORT.spell_check} />
			{/if}
		{:else if active === 'bank'}
			<!-- coin is null in P14 (ADMIN-05 fills it in P15) — render the
			     not-yet-recorded affordance, never a fabricated zero-platinum value. -->
			<div class="bank-toolbar">
				{#if coinToons.length > 0}
					<!-- Recorded coin surfaces here (15-05 D-11), replacing P14's null
					     placeholder. Character names render via plain {} (auto-escaped,
					     T-15-28). -->
					<div class="coin-summary">
						<h3 class="coin-summary-title">Bank coin</h3>
						<ul class="coin-list">
							{#each coinToons as t (t.character_id)}
								<li class="coin-item">
									<span class="coin-char">{t.name}</span>
									<span class="coin-amount">{coinLine(t)}</span>
								</li>
							{/each}
						</ul>
					</div>
				{:else}
					<StateBlock kind="no-coin" />
				{/if}
				<!-- "Record coin" affordance → /settings (the BankCoinForm rehomes there);
				     the bank round-trip seeds /guild-views?view=bank (Pitfall 5). -->
				<a class="record-coin" href="/settings">Record coin</a>
			</div>
			{#if filteredBankRows.length === 0}
				{#if filterActive}
					<p class="filter-empty">None of your characters have rows in this view.</p>
				{:else}
					<!-- bank may be legitimately empty until P16 sets is_bank_toon — empty
					     state, NOT an error (RESEARCH Open-Q4 / A7). -->
					<StateBlock kind="view-empty" viewName="bank" />
				{/if}
			{:else}
				<DataGrid data={filteredBankRows} columns={bankColumns} defaultSorting={SORT.bank} />
			{/if}
		{/if}
	</section>
{/if}

<style>
	.search-section {
		max-width: 720px;
	}
	.view-nav {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		border-bottom: 1px solid var(--border, rgba(74, 101, 133, 0.4));
	}
	.tab {
		min-height: 44px;
		padding: 8px 16px;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		cursor: pointer;
		opacity: 0.7;
	}
	.tab:hover {
		opacity: 1;
	}
	/* Active view tab in accent (reserved accent use, UI-SPEC). */
	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
		opacity: 1;
	}
	.tab:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	/* 27-01: the "My characters" filter bar (view-orchestration layer, above the grid).
	   Styled with the same EQ-theme token set as DataGrid's .facet <select>. */
	.filter-bar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		margin: 8px 0;
	}
	/* 260610-fm5 WS3: the segmented scope toggle — DUPLICATED from WantlistPanel's
	   .seg/.seg-btn block (CONTEXT: duplicating the small style block is acceptable;
	   Svelte styles are component-scoped, so the classes must live here too). */
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
	/* "My characters" is disabled until the caller has claimed at least one
	   character (Pitfall 5 — the hint below points at /settings). */
	.seg-btn:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.filter-label {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
		opacity: 0.85;
	}
	.char-filter {
		min-height: 44px; /* touch target */
		padding: 8px 12px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		background: var(--panel);
		color: var(--text);
		font-family: var(--font-body);
		font-size: 16px;
		cursor: pointer;
	}
	.char-filter:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.filter-hint {
		font-family: var(--font-body);
		font-size: 14px;
		opacity: 0.8;
	}
	.filter-hint a {
		color: var(--accent);
	}
	/* Distinct copy when the filter is active but empties the grid — NOT the generic
	   all-members "no data" StateBlock (Pitfall 3/5). */
	.filter-empty {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.85;
		padding: 24px 0;
		text-align: center;
	}
	.view-panel {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-height: 0;
	}
	/* Bank toolbar: the coin summary + the "Record coin" affordance (15-05 D-11). */
	.bank-toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}
	.coin-summary {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.coin-summary-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label (UI-SPEC) */
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.coin-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.coin-item {
		display: flex;
		gap: 12px;
		font-family: var(--font-body);
		font-size: 16px;
	}
	.coin-char {
		font-weight: 600;
		min-width: 12ch;
	}
	.coin-amount {
		font-variant-numeric: tabular-nums; /* plat/gold/silver/copper align (UI-SPEC) */
		opacity: 0.9;
	}
	/* "Record coin" link — styled like the .tab nav affordance (UI-SPEC). */
	.record-coin {
		min-height: 44px; /* touch target */
		display: inline-flex;
		align-items: center;
		padding: 8px 16px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
		text-decoration: none;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
	}
	.record-coin:hover {
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}
	.record-coin:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
