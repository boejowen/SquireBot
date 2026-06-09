<script lang="ts">
	// WantAddForm — the catalog-first add-item block (19-UI-SPEC § Add-Item
	// Contract, WANT-01 / D-03/D-04). A single debounced search field over the
	// server catalog (GET /api/v1/items/search) → pick a result to pin an item_id,
	// OR take the custom-want escape hatch (item_id null, flagged "won't trigger
	// alerts"). Picking either path reveals the Reason/Priority/Note detail fields;
	// "Add to wantlist" POSTs and dispatches success so the panel re-fetches.
	//
	// XSS boundary (T-19-13): catalog names, the typed custom label, and the note
	// render ONLY via plain {} (Svelte auto-escapes) — never the raw-HTML directive.
	// The catalog search is DEBOUNCED ~250ms (T-19-14 / Pitfall A4) — no fire per
	// keystroke. Detail math (the N/280 counter) delegates to the Task-1 DOM-free
	// noteRuneCount so the counter agrees with the server's 280-rune cap.

	import Search from '@lucide/svelte/icons/search';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import FormField from './FormField.svelte';
	import ItemTooltip from './ItemTooltip.svelte';
	import { onMount } from 'svelte';
	import {
		searchCatalog,
		addWant,
		fetchMyCharacters,
		type CatalogItem,
		type MyCharacter
	} from '$lib/api';
	import { noteRuneCount } from '$lib/wantlist/priority';

	let {
		/** Called after a successful add so the panel reloads from the server. */
		onAdded
	}: { onAdded: (itemName: string) => void } = $props();

	const NOTE_CAP = 280;
	const DEBOUNCE_MS = 250;

	// Search state.
	let query = $state('');
	let results = $state<CatalogItem[]>([]);
	let searching = $state(false);
	let searchSeq = 0; // guards against out-of-order debounced responses.
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	// Staged want: either a pinned catalog item OR a custom text label.
	let pickedItem = $state<CatalogItem | null>(null);
	let customLabel = $state<string | null>(null);

	// Detail fields.
	let reason = $state<'' | 'buy' | 'quest'>('');
	let priority = $state<'low' | 'med' | 'high'>('med');
	let note = $state('');

	// CWANT-01 the OPTIONAL character tag. `myCharacters` sources the <select> options
	// from the caller's OWN assigned characters (fetchMyCharacters, P26 — REUSED, not
	// reinvented). `charSelect` is the bound <select> value as a STRING ('' = the
	// "(no character)" default → account-level); submit coerces it to a number|null.
	// The select is UX convenience, NOT the gate — the server's IsCharAssignedToTx
	// (Plan 02) is the authority; a forged body is rejected 403 (T-28-11).
	let myCharacters = $state<MyCharacter[]>([]);
	let charSelect = $state('');

	// Submit state.
	let adding = $state(false);
	let addErrorMsg = $state('');

	// Load the caller's own characters for the optional tag <select>. A failure is
	// non-fatal — the tag is optional, so on error we just leave the list empty and the
	// form still adds account-level wants (the only option being "(no character)").
	onMount(() => {
		void (async () => {
			try {
				myCharacters = await fetchMyCharacters();
			} catch {
				myCharacters = [];
			}
		})();
	});

	let noteCount = $derived(noteRuneCount(note));
	// Disabled until an item is chosen (catalog OR non-blank custom) AND a reason.
	let staged = $derived(pickedItem !== null || customLabel !== null);
	let canSubmit = $derived(staged && reason !== '' && !adding);

	// Debounced catalog search — does NOT query per keystroke (Pitfall A4). q<2
	// shows nothing (the server returns [] anyway, but we short-circuit too).
	function onQueryInput() {
		addErrorMsg = '';
		if (debounceTimer) clearTimeout(debounceTimer);
		const q = query.trim();
		if (q.length < 2) {
			results = [];
			searching = false;
			return;
		}
		searching = true;
		debounceTimer = setTimeout(() => void runSearch(q), DEBOUNCE_MS);
	}

	async function runSearch(q: string) {
		const seq = ++searchSeq;
		try {
			const items = await searchCatalog(q);
			if (seq !== searchSeq) return; // a newer query superseded this one.
			results = items;
		} catch {
			if (seq !== searchSeq) return;
			results = [];
		} finally {
			if (seq === searchSeq) searching = false;
		}
	}

	function pick(item: CatalogItem) {
		pickedItem = item;
		customLabel = null;
		results = [];
		// Keep the field showing the chosen name as a read-back.
		query = item.name;
	}

	function pickCustom() {
		const label = query.trim();
		if (!label) return;
		customLabel = label;
		pickedItem = null;
		results = [];
	}

	function resetStaging() {
		pickedItem = null;
		customLabel = null;
		reason = '';
		priority = 'med';
		note = '';
		charSelect = '';
		query = '';
		results = [];
		addErrorMsg = '';
	}

	async function submit() {
		if (!canSubmit) return;
		const itemName = pickedItem ? pickedItem.name : (customLabel ?? '');
		const itemId = pickedItem ? pickedItem.item_id : null;
		// '' (the "(no character)" default) → null (account-level); else the picked
		// character_id as a number. The server re-authorizes the tag (T-28-11).
		const charId = charSelect === '' ? null : Number(charSelect);
		adding = true;
		addErrorMsg = '';
		try {
			await addWant({
				item_id: itemId,
				item_name: itemName,
				reason: reason as 'buy' | 'quest',
				priority,
				note: note.trim() || undefined,
				character_id: charId
			});
			onAdded(itemName);
			resetStaging();
		} catch (err) {
			const code =
				err && typeof err === 'object' && 'code' in err
					? (err as { code?: string }).code
					: undefined;
			if (code === 'duplicate') {
				addErrorMsg =
					"That's already on your list with the same reason. (The same item can be on twice — once to buy, once for a quest.)";
			} else {
				addErrorMsg = "Couldn't add that to your wantlist. Nothing was added — try again.";
			}
		} finally {
			adding = false;
		}
	}

	// No exact catalog match for a >=2-char query: offer the custom escape hatch.
	let noExactMatch = $derived(
		query.trim().length >= 2 &&
			!searching &&
			!staged &&
			!results.some((r) => r.name.toLowerCase() === query.trim().toLowerCase())
	);
</script>

<div class="add-form">
	<!-- Search field (debounced). The Lucide search icon + accent focus ring. -->
	<label class="search-field">
		<Search size={18} aria-hidden="true" />
		<input
			type="search"
			placeholder="Search items to add…"
			bind:value={query}
			oninput={onQueryInput}
			aria-label="Search items to add"
		/>
	</label>

	{#if !staged}
		{#if results.length > 0}
			<ul class="results" aria-label="Catalog matches">
				{#each results as item (item.item_id)}
					<li class="result">
						<button type="button" class="result-pick" onclick={() => pick(item)}>
							<ItemTooltip itemName={item.name} wikiUrl="">
								<span class="result-name">{item.name}</span>
							</ItemTooltip>
							<span class="result-id">#{item.item_id}</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
		{#if noExactMatch}
			<!-- Custom escape hatch (D-04). The label renders via plain {} (auto-escaped). -->
			<button type="button" class="custom-affordance" onclick={pickCustom}>
				Add "{query.trim()}" as a custom want
			</button>
		{/if}
	{/if}

	{#if staged}
		<div class="staged">
			<!-- Item read-back (locked). Catalog pick shows the name + #id; a custom
			     want shows the label + the mandatory "won't trigger alerts" chip. -->
			<div class="staged-item">
				{#if pickedItem}
					<span class="staged-name">{pickedItem.name}</span>
					<span class="staged-id">#{pickedItem.item_id}</span>
				{:else}
					<span class="staged-name">{customLabel}</span>
					<span
						class="custom-chip"
						title="Custom wants aren't matched against the item catalog, so the EC/WTS/raid monitors can't alert you about them."
						>Custom — won't trigger alerts</span
					>
				{/if}
				<button type="button" class="change-btn" onclick={resetStaging}>Change</button>
			</div>

			<div class="detail-fields">
				<FormField label="Reason" id="want-reason">
					<select id="want-reason" class="field" bind:value={reason}>
						<option value="" disabled>Choose…</option>
						<option value="buy">Buy</option>
						<option value="quest">Quest</option>
					</select>
				</FormField>

				<FormField label="Priority" id="want-priority">
					<select id="want-priority" class="field" bind:value={priority}>
						<option value="low">Low</option>
						<option value="med">Med</option>
						<option value="high">High</option>
					</select>
				</FormField>

				<!-- CWANT-01 the OPTIONAL character tag. Options come ONLY from the caller's
				     own assigned characters (fetchMyCharacters) — the "(no character)" default
				     is account-level. Character names render via plain auto-escaped braces, never
				     raw-HTML (T-28-10). The select is UX, not the gate; the server re-authorizes. -->
				<FormField label="Character (optional)" id="want-character">
					<select id="want-character" class="field" bind:value={charSelect}>
						<option value="">(no character)</option>
						{#each myCharacters as c (c.character_id)}
							<option value={String(c.character_id)}>{c.name}</option>
						{/each}
					</select>
				</FormField>

				<FormField label="Note (optional)" id="want-note">
					<textarea
						id="want-note"
						class="field note-area"
						maxlength={NOTE_CAP}
						bind:value={note}
						rows="2"
					></textarea>
					<span class="note-counter" class:at-cap={noteCount >= NOTE_CAP}>{noteCount}/{NOTE_CAP}</span>
				</FormField>
			</div>

			{#if addErrorMsg}
				<p class="result-error" aria-live="polite">{addErrorMsg}</p>
			{/if}

			<button type="button" class="primary" disabled={!canSubmit} onclick={submit}>
				{#if adding}
					<LoaderCircle size={16} aria-hidden="true" class="spin" />
					Adding…
				{:else}
					Add to wantlist
				{/if}
			</button>
		</div>
	{/if}
</div>

<style>
	.add-form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.search-field {
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
	.search-field input {
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
	.search-field:focus-within {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.results {
		display: flex;
		flex-direction: column;
		gap: 4px;
		list-style: none;
		margin: 0;
		padding: 0;
		max-width: 480px;
	}
	.result-pick {
		display: inline-flex;
		align-items: baseline;
		gap: 8px;
		width: 100%;
		min-height: 44px;
		padding: 8px 12px;
		background: none;
		border: 1px solid transparent;
		border-radius: 4px;
		cursor: pointer;
		text-align: left;
	}
	.result-pick:hover {
		background: color-mix(in srgb, var(--accent) 6%, transparent);
		border-color: var(--border, var(--accent));
	}
	.result-pick:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.result-name {
		color: var(--accent);
		font-family: var(--font-body);
		font-size: 16px;
	}
	.result-id {
		font-size: 13px;
		opacity: 0.55;
		font-variant-numeric: tabular-nums;
	}
	/* The custom-want affordance is an accent text button (D-04). */
	.custom-affordance {
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
	.custom-affordance:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.staged {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.staged-item {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 8px;
	}
	.staged-name {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
	}
	.staged-id {
		font-size: 13px;
		opacity: 0.55;
		font-variant-numeric: tabular-nums;
	}
	.custom-chip {
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		color: var(--status-other);
		background: color-mix(in srgb, var(--status-other) 8%, transparent);
		white-space: nowrap;
	}
	.change-btn {
		min-height: 44px;
		padding: 4px 8px;
		background: none;
		border: none;
		color: var(--accent);
		font-family: var(--font-body);
		font-size: 13px;
		cursor: pointer;
	}
	.change-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.detail-fields {
		display: flex;
		flex-direction: column;
		gap: 16px;
		max-width: 480px;
	}
	.field {
		min-height: 44px;
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
	}
	.field:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.note-area {
		resize: vertical;
		min-height: 64px;
		line-height: 1.5;
	}
	.note-counter {
		font-family: var(--font-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		opacity: 0.7;
		align-self: flex-end;
	}
	.note-counter.at-cap {
		color: var(--status-missing);
		opacity: 1;
	}
	.primary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		align-self: flex-start;
		min-height: 44px;
		padding: 8px 24px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--bg);
		background: var(--accent);
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	:global(.spin) {
		animation: spin 0.8s linear infinite;
	}
	@media (prefers-reduced-motion: reduce) {
		:global(.spin) {
			animation: none;
		}
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.result-error {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		color: var(--status-missing);
	}
</style>
