<script lang="ts">
	// /characters — the Characters tab (Phase 31, CHAR-01/02/03). Replaces the
	// Phase-30 "coming soon" placeholder. Master-detail on ONE route (the CLAUDE.md
	// consolidated-views lock is RELAXED for exactly this pattern): a bespoke,
	// viewer-first 3-band character list (left) + a scoped viewer-priority search
	// (top) + the selected character's inventory window (right). Selection is
	// URL-reflected via ?c=<name> (no per-character route file — one reusable window,
	// not N routes). This plan (31-03) wires the list + search + selection + the
	// inventory fetch; the actual InventoryWindow render lands in 31-04, so the window
	// column shows a "Pick a character" prompt (no selection) or a minimal "Selected"
	// marker (selection wired, window pending) for now.
	//
	// The pure sort/filter logic lives in $lib/roster (node-tested — roster.test.ts).
	// The list/search/selection DOM here is NOT covered by node vitest (DOM-blind) —
	// its browser verification is folded into Plan 31-04's deploy-then-browser-smoke.
	//
	// Data load + state machine mirrors guild-views/+page.svelte: onMount one-shot
	// load; a 401/403 routes to the AuthGate guard (server-truth re-route), else the
	// error StateBlock + Retry. Item/character names render via plain {} interpolation
	// (Svelte auto-escapes) — this file uses NO raw-HTML directive (T-31-10/11); the one
	// sanctioned escaped-HTML sink (composeItemNote) only lands in 31-04's examine.

	import { onMount, getContext } from 'svelte';
	import Search from '@lucide/svelte/icons/search';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
	import { Unauthenticated, Forbidden, fetchCharacters, type RosterCharacter } from '$lib/api';
	import { bandOf, filterRoster, type Band } from '$lib/roster';

	type Status = 'loading' | 'error' | 'ready';

	// The AuthGate guard from context (server-truth re-routing on a 401/403, B-2).
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	let status = $state<Status>('loading');
	let roster = $state<RosterCharacter[]>([]);
	let query = $state('');
	let selected = $state<string | null>(null);

	async function load() {
		status = 'loading';
		try {
			roster = await fetchCharacters();
			// A Retry/refetch can drop the character the window is pinned to — clear a
			// now-stale selection so the window column doesn't stick on a gone char.
			const sel = selected;
			if (sel && !roster.some((c) => c.name.toLowerCase() === sel.toLowerCase())) {
				selected = null;
			}
			status = 'ready';
		} catch (err) {
			// Server-truth (B-2): a 401/403 means the session is gone or refused — hand
			// it to the AuthGate guard so the site re-routes instead of showing a stale
			// view. Any other failure (network/5xx) stays the generic error StateBlock.
			if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
				authGuard(err);
			} else {
				status = 'error';
			}
		}
	}

	// Retry re-fires the fetch (UI-SPEC error-state Retry).
	function refetch() {
		void load();
	}

	// One-shot initial fetch + pre-select ?c=<name> if present (deep-link). onMount is
	// the correct primitive for a fire-once load (a bare $effect would re-fire). The
	// ?c= value pre-selects a character so a shared link opens its window directly.
	onMount(() => {
		const c = new URLSearchParams(window.location.search).get('c');
		if (c) selected = c;
		void load();
	});

	// The filtered, viewer-first roster (CHAR-02 / D-10 — keeps mine → guild → banks
	// ranking among matches). Pure helper from $lib/roster (node-tested).
	let shown = $derived(filterRoster(roster, query));

	// Group the shown rows into the three D-10 bands for the labelled list. An empty
	// band is omitted from rendering (no empty header — §B).
	let bands = $derived.by(() => {
		const groups: Record<Band, RosterCharacter[]> = { mine: [], guild: [], banks: [] };
		for (const c of shown) groups[bandOf(c)].push(c);
		return groups;
	});

	const BAND_LABEL: Record<Band, string> = {
		mine: 'YOUR CHARACTERS',
		guild: 'GUILD',
		banks: 'BANKS & BOTS'
	};
	const BAND_ROWS: Band[] = ['mine', 'guild', 'banks'];

	// Roster is non-empty after a ready load with zero matches AND a non-empty query →
	// the no-results state (vs. the "No characters yet" empty state when the roster
	// itself is empty).
	let noResults = $derived(status === 'ready' && shown.length === 0 && query.trim() !== '');

	function select(name: string) {
		selected = name;
		// URL-reflect the selection (?c=<name>) WITHOUT a full navigation or a new route
		// file — deep-linkable + shareable; the single reusable window renders for the
		// selected char (RESEARCH Open Q3). encodeURIComponent because names are
		// guildie-controlled. history.replaceState keeps focus + scroll (no reload).
		if (typeof history !== 'undefined') {
			const url = new URL(window.location.href);
			url.searchParams.set('c', name);
			history.replaceState(history.state, '', url);
		}
	}

	// The dimmed meta line for a list row (D-11): show what's known, never "null".
	// Drop a zero level / blank race / blank class token rather than render a broken
	// "Level 0" or trailing blank.
	function metaLine(c: RosterCharacter): string {
		const parts: string[] = [];
		if (c.level > 0) parts.push(`Level ${c.level}`);
		if (c.race) parts.push(c.race);
		if (c.class) parts.push(c.class);
		return parts.join(' ');
	}
</script>

<svelte:head>
	<title>SquireBot — Characters</title>
</svelte:head>

{#if status === 'loading'}
	<StateBlock kind="loading" />
{:else if status === 'error'}
	<StateBlock kind="error" onRetry={refetch} />
{:else if roster.length === 0}
	<!-- Roster empty: no character synced anywhere yet. Reuses the "No characters
	     yet" empty copy (§J). -->
	<StateBlock kind="empty" />
{:else}
	<div class="characters">
		<!-- Left: scoped search + the bespoke 3-band viewer-first list. -->
		<div class="list-col">
			<div class="search">
				<Search size={16} aria-hidden="true" class="search-icon" />
				<input
					class="search-input"
					type="search"
					placeholder="Search characters…"
					aria-label="Search characters"
					bind:value={query}
				/>
			</div>
			<p class="search-hint">Your characters match first.</p>

			{#if noResults}
				<StateBlock kind="no-results" {query} />
			{:else}
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
										{#if c.is_mine}<span class="tag">yours</span>{/if}
										{#if c.is_bank_toon}<span class="tag">bank</span>{/if}
										{#if c.is_guild_bot}<span class="tag">bot</span>{/if}
									</span>
									{#if metaLine(c)}
										<span class="row-meta">{metaLine(c)}</span>
									{/if}
								</button>
							{/each}
						{/if}
					{/each}
				</div>
			{/if}
		</div>

		<!-- Right: the inventory window column. 31-03 wires selection; the actual
		     InventoryWindow render lands in 31-04. Until then: the "Pick a character"
		     prompt (no selection) or a minimal Selected marker (selection wired). -->
		<div class="window-col">
			{#if selected === null}
				<div class="prompt">
					<h2 class="prompt-heading">Pick a character</h2>
					<p class="prompt-body">Choose a character from the list to see their gear and bags.</p>
				</div>
			{:else}
				<!-- InventoryWindow mounts here in Plan 31-04. -->
				<p class="selected-marker">Selected: {selected}</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	/* Two-pane master-detail: list (left) + window (right). Desktop side-by-side;
	   mobile stacks (§H). Gutters are inherited from shell-main (32px / 16px). */
	.characters {
		display: grid;
		grid-template-columns: minmax(280px, 360px) 1fr;
		gap: 24px; /* lg — window column gap (§Spacing) */
		align-items: start;
	}

	/* --- Scoped search (§C) --- */
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
		font-size: 13px;
		color: var(--text);
		opacity: 0.7;
	}

	/* --- The bespoke 3-band list (§B) --- */
	.bands {
		display: flex;
		flex-direction: column;
	}
	.band-label {
		margin: 16px 0 4px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
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
		min-height: 44px; /* touch target */
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
		font-size: 16px; /* Body */
		line-height: 1.3;
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
	.row-meta {
		font-size: 16px; /* Body */
		line-height: 1.4;
		opacity: 0.85;
	}

	/* --- Window column (§K prompt until 31-04) --- */
	.window-col {
		min-height: 200px;
	}
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
	.selected-marker {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		opacity: 0.85;
		padding: 16px;
	}

	/* Mobile: list + window stack (§H). */
	@media (max-width: 640px) {
		.characters {
			grid-template-columns: 1fr;
		}
	}
</style>
