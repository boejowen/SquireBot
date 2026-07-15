<script lang="ts">
	// InventoryWindow — the in-game-style inventory window (Phase 31, INV-01..04,
	// 31-UI-SPEC §D/§E/§F/§G/§H). A GENERIC, prop-driven component over ONE
	// CharacterInventory (NO Characters-tab-only assumptions — Phase 33 reuses it
	// per bank toon, Phase 34 reads its equipped slots). Top-to-bottom, ONE
	// continuous scroll (NO Inventory/Bank toggle — D-05): char-head → 23-slot
	// paperdoll (§E) → general grid → bank grid (same renderer, D-05) → hint, with a
	// sticky ExaminePanel as the right column (master-detail; §G).
	//
	// Interactions: a FILLED non-container tile HOVER shows a transient examine
	// preview (the same examineFields body as the pin; pointer-events:none; dismissed
	// on leave/Esc); a CLICK PINS it into the single ExaminePanel (replace-on-click,
	// D-07). A CONTAINER tile (slots>0) click OPENS its INLINE expand in-flow (D-04 —
	// the contents appear beneath the grid row, never as a pop-out overlay) and does
	// NOT pin. "Last synced" is inventory.last_seen (the per-character value —
	// Pitfall 2), passed to the examine for every item.
	//
	// SECURITY: this file adds NO raw-HTML directive — the only escaped-HTML sink is
	// inside ExaminePanel (composeItemNote). Item names reach the DOM only via
	// PaperdollSlot's aria-label and the examine preview's plain {} interpolation
	// (Svelte auto-escapes). The portrait <img alt> + the char name likewise render via
	// plain {} (auto-escaped), NEVER the raw-HTML directive (Phase 41 / T-41W-01).

	import type { CharacterInventory, InventorySlot } from '$lib/api';
	import { portraitUrl } from '$lib/api';
	import PaperdollSlot from '$lib/components/PaperdollSlot.svelte';
	import ExaminePanel from '$lib/components/ExaminePanel.svelte';
	import PortraitControl from '$lib/components/PortraitControl.svelte';
	import { examineFields } from '$lib/examine';

	// `canEdit` (Phase 41 / D-05/D-06) is the assignee-OR-officer visibility gate result,
	// derived in characters/+page.svelte and passed in; it decides whether the portrait
	// upload/remove control renders. The server re-checks on every write (41-01), so the
	// gate is Layer-1 UX only — a forged request still 403s.
	let { inventory, canEdit = false }: { inventory: CharacterInventory; canEdit?: boolean } = $props();

	// Optimistic local override for the portrait flag/cache-bust key so a set/remove
	// re-renders the frame WITHOUT mutating the `inventory` prop directly. `null` = follow
	// the prop; onPortraitChanged sets it (keyed by char so it self-clears when the user
	// selects a different character and a fresh `inventory` flows in). The $derived reads the
	// reactive prop, so switching characters shows the new char's server flag immediately.
	let portraitOverride = $state<{ char: string; has: boolean; version: string } | null>(null);
	let hasPortrait = $derived(
		portraitOverride && portraitOverride.char === inventory.char
			? portraitOverride.has
			: inventory.has_portrait
	);
	let portraitVersion = $derived(
		portraitOverride && portraitOverride.char === inventory.char
			? portraitOverride.version
			: inventory.portrait_updated_at
	);

	function onPortraitChanged(detail: { has_portrait: boolean; updated_at: string }) {
		portraitOverride = { char: inventory.char, has: detail.has_portrait, version: detail.updated_at };
		// Re-show the <img> if a prior error had hidden it (a fresh upload replaces a broken one).
		imgHidden = false;
	}

	// Mirror the PaperdollSlot onImgError hide-and-fall-through: hide the <img> so the
	// silhouette under-layer paints through (a broken/expired blob degrades gracefully).
	// Reset the hide when the selected character changes (a new inventory flows in).
	let imgHidden = $state(false);
	let imgHiddenFor = $derived(inventory.char);
	$effect(() => {
		// Referencing imgHiddenFor makes this re-run on a char change; reset the hide.
		void imgHiddenFor;
		imgHidden = false;
	});
	function onImgError() {
		imgHidden = true;
	}

	// The single pinned examine slot (D-07 — replace-on-click).
	let pinned = $state<InventorySlot | null>(null);
	// The transient hover-preview slot + its anchor rect (desktop hover, §G).
	let preview = $state<InventorySlot | null>(null);
	let previewPos = $state<{ x: number; y: number }>({ x: 0, y: 0 });
	// The set of open container locations (inline bag expand — D-04; keyed by location).
	let openBags = $state<Set<string>>(new Set());

	// --- the 23 canonical equipment slots (slotconst.go — authoritative, NOT 21) ---
	// Placement per 31-UI-SPEC §E: left column, right column, then the bottom WORN row.
	// Left column leads with Head + Face, then the two ear slots, then the rest (user layout).
	const LEFT_SLOTS = ['Head', 'Face', 'Ear1', 'Ear2', 'Neck', 'Shoulders', 'Arms', 'Back'];
	// Waist lives up here with the worn armor (body column), not down in the weapons row.
	const RIGHT_SLOTS = ['Wrist1', 'Wrist2', 'Hands', 'Finger1', 'Finger2', 'Chest', 'Legs', 'Feet', 'Waist'];
	// No Charm or Power Source position: both equipment slots were added in post-Velious
	// expansions Project 1999 will never implement, so they never hold an item — omit them
	// from the paperdoll rather than render a permanently-empty tile (2026-06-18). The bottom
	// row is now weapons + ammo only (Waist moved up).
	const WORN_SLOTS = ['Primary', 'Secondary', 'Range', 'Ammo'];

	// Index the equipment array by canonical_slot so each paperdoll position maps to
	// its slot (or null → an empty labelled tile, D-11).
	let equipBySlot = $derived.by(() => {
		const m = new Map<string, InventorySlot>();
		for (const s of inventory.equipment) {
			if (s.canonical_slot) m.set(s.canonical_slot, s);
		}
		return m;
	});

	function equipAt(canonical: string): InventorySlot | null {
		return equipBySlot.get(canonical) ?? null;
	}

	// A filled slot has a non-empty item name (empty equipment slots are kept, D-11).
	function isFilled(s: InventorySlot | null): boolean {
		return s !== null && (s.item?.trim() ?? '') !== '';
	}

	// A slot is an openable container (bag) only when it actually CONTAINS slots — the data
	// nested child rows under it. Real /outputfile bags enumerate every slot as a child (even
	// empty ones, so kids == capacity); a non-bag item with a stray Slots value (worn gear,
	// boots, a circlet — all Slots=5) has NO children. Children-presence is the reliable signal,
	// NOT slots > 0 (2026-06-18). Keep in lockstep with PaperdollSlot.isBag.
	function isContainer(s: InventorySlot | null): boolean {
		return s !== null && (s.children?.length ?? 0) > 0;
	}

	// --- pin / open / hover handlers (§G) ---
	function pin(s: InventorySlot) {
		pinned = s;
		preview = null; // a pin supersedes the transient preview
	}

	function toggleBag(s: InventorySlot) {
		preview = null; // opening/closing a bag dismisses any transient hover preview
		const next = new Set(openBags);
		if (next.has(s.location)) next.delete(s.location);
		else next.add(s.location);
		openBags = next;
	}

	function isOpen(s: InventorySlot): boolean {
		return openBags.has(s.location);
	}

	// Open-state for an equipment position by canonical slot (null/empty → false).
	function isOpenAt(canonical: string): boolean {
		const s = equipAt(canonical);
		return s ? isOpen(s) : false;
	}

	function hoverEnter(s: InventorySlot, e: MouseEvent) {
		// Transient preview for ANY filled tile — including a bag, so a user can tell which
		// bag is which (e.g. "Bag of Sewn Evil-Eye") on hover without opening it. A bag still
		// OPENS on click (it doesn't pin); the preview is purely the identify-on-hover affordance.
		// pointer-events:none means the floating preview never steals the next hover/click.
		preview = s;
		previewPos = { x: e.clientX + 14, y: e.clientY + 14 };
	}
	function hoverLeave() {
		preview = null;
	}

	// Esc dismisses the transient preview (the pinned panel stays — D-07).
	$effect(() => {
		if (!preview) return;
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape') preview = null;
		}
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	// The preview body (same D-08 fields as the pin; the hover shares the content).
	let previewFields = $derived(preview ? examineFields(preview, inventory.last_seen) : []);

	// A dimmed meta line for the char-head (D-11 — show what's known, never "null").
	// The roster carries level/race/class; the inventory payload carries only the
	// name + last_seen, so the head shows the name + a sync line here.
	let synced = $derived(inventory.last_seen ? `Last synced ${inventory.last_seen}` : '');
</script>

<!-- Two-pane (§H): the window column (paperdoll + grids) + a sticky ExaminePanel.
     ≤900px the panel drops below (single column). -->
<div class="layout">
	<div class="window">
		<!-- Char-head (§D.1) -->
		<header class="charhead">
			<h1 class="charname">{inventory.char}</h1>
			{#if synced}<p class="charmeta">{synced}</p>{/if}
		</header>

		<!-- Paperdoll (§E): left col / center figure / right col -->
		<section class="paperdoll" aria-label="Equipment">
			<div class="equip-col">
				{#each LEFT_SLOTS as name (name)}
					<PaperdollSlot
						slot={equipAt(name)}
						label={name.toUpperCase()}
						onpin={pin}
						onopen={toggleBag}
						onhover={hoverEnter}
						onleave={hoverLeave}
						expanded={isOpenAt(name)}
					/>
				{/each}
			</div>

			<div class="figure">
				<div class="doll" class:filled={hasPortrait && !imgHidden}>
					{#if hasPortrait && !imgHidden}
						<!-- The portrait overlays the silhouette; on img error it hides (imgHidden)
						     and the silhouette under-layer paints through. alt={char} via plain {}
						     (Svelte auto-escapes) — NEVER the raw-HTML directive (T-41W-01). ?v= cache-busts. -->
						<img
							class="portrait-img"
							src={portraitUrl(inventory.char, portraitVersion)}
							alt={inventory.char}
							onerror={onImgError}
						/>
					{/if}
					<!-- Silhouette + name under-layer: the fallback that paints through on img-error
					     or when no portrait is set. aria-hidden — the name is already in the char-head. -->
					<span class="silhouette" aria-hidden="true">⚔</span>
					<p class="doll-line" aria-hidden="true">{inventory.char}</p>
				</div>

				{#if canEdit}
					<PortraitControl
						char={inventory.char}
						hasPortrait={hasPortrait && !imgHidden}
						onchanged={onPortraitChanged}
					/>
				{/if}
			</div>

			<div class="equip-col">
				{#each RIGHT_SLOTS as name (name)}
					<PaperdollSlot
						slot={equipAt(name)}
						label={name.toUpperCase()}
						onpin={pin}
						onopen={toggleBag}
						onhover={hoverEnter}
						onleave={hoverLeave}
						expanded={isOpenAt(name)}
					/>
				{/each}
			</div>
		</section>

		<!-- WORN — WEAPONS & MISC (§D.3 / §E bottom row) -->
		<section class="worn" aria-label="Worn weapons and miscellaneous">
			<p class="eyebrow">WORN — WEAPONS &amp; MISC</p>
			<div class="grid">
				{#each WORN_SLOTS as name (name)}
					{@const s = equipAt(name)}
					<div class="cell">
						<PaperdollSlot
							slot={s}
							label={name.toUpperCase()}
							onpin={pin}
							onopen={toggleBag}
							onhover={hoverEnter}
							onleave={hoverLeave}
							expanded={s ? isOpen(s) : false}
						/>
						{#if s && isFilled(s) && isContainer(s) && isOpen(s)}
							{@render bagExpand(s)}
						{/if}
					</div>
				{/each}
			</div>
		</section>

		<!-- GENERAL INVENTORY (§F) -->
		<section class="bag-section" aria-label="General inventory">
			<p class="eyebrow">GENERAL INVENTORY</p>
			{@render itemGrid(inventory.general)}
		</section>

		<!-- BANK — STORED ITEMS (§F / D-05, same renderer) -->
		<section class="bag-section" aria-label="Bank stored items">
			<p class="eyebrow">BANK — STORED ITEMS</p>
			{@render itemGrid(inventory.bank)}
		</section>

		<p class="hint">
			Click any item to pin its details on the right. Click a ⊞ bag to open it. Item icons load
			from the P1999 wiki.
		</p>
	</div>

	<!-- Right column: the single pinned examine (sticky desktop; below on mobile). -->
	<div class="pin-col">
		<ExaminePanel slot={pinned} charLastSeen={inventory.last_seen} />
	</div>
</div>

<!-- The transient hover preview (desktop; pointer-events:none; §G). Same D-08 body
     as the pin. Rendered at the document level so it floats over the grid. -->
{#if preview}
	<div class="preview" style={`left: ${previewPos.x}px; top: ${previewPos.y}px;`} aria-hidden="true">
		{#each previewFields as f (f.kind)}
			{#if f.kind === 'name'}
				<p class="pv-name">{f.text}</p>
			{:else if f.kind === 'wiki'}
				<!-- The preview shows the wiki label as inert text (no link; the pinned
				     panel carries the real escaped <a> sink). -->
				<p class="pv-line pv-dim">{f.text}</p>
			{:else}
				<p class="pv-line" class:pv-dim={f.kind === 'lastsynced'}>{f.text}</p>
			{/if}
		{/each}
	</div>
{/if}

<!-- One reusable grid renderer for general + bank (D-05). Each container tile, when
     open, renders an INLINE expand of its children directly below its cell (D-04). -->
{#snippet itemGrid(items: InventorySlot[])}
	{#if items.length === 0}
		<p class="empty-grid">No items here.</p>
	{:else}
		<div class="grid">
			<!-- Key by location+index, not location alone: real /outputfile inventory can write
			     the SAME base token for two rows (the doubled Ear/Wrist/Finger slots are
			     numbered upstream now, but the +index guarantees a unique key so no
			     duplicate-Location data can ever throw each_key_duplicate and freeze the
			     window — the 2026-06-18 stuck-loading bug). -->
			{#each items as s, i (s.location + '#' + i)}
				<div class="cell">
					<PaperdollSlot
						slot={s}
						onpin={pin}
						onopen={toggleBag}
						onhover={hoverEnter}
						onleave={hoverLeave}
						expanded={isOpen(s)}
					/>
					{#if isFilled(s) && isContainer(s) && isOpen(s)}
						{@render bagExpand(s)}
					{/if}
				</div>
			{/each}
		</div>
	{/if}
{/snippet}

<!-- The INLINE bag expand (D-04 — in-flow beneath the grid row, never a pop-out
     overlay). An indented --panel sub-region: a Label sub-header + a sub-grid of
     the SAME tiles over children. -->
{#snippet bagExpand(bag: InventorySlot)}
	{@const kids = bag.children ?? []}
	{@const used = kids.filter((k) => isFilled(k)).length}
	<div class="bag-expand">
		<p class="bag-subhead">{bag.item} — {used} of {bag.slots} slots</p>
		{#if kids.length === 0}
			<p class="bag-empty">Empty</p>
		{:else}
			<div class="grid">
				{#each kids as k, i (k.location + '#' + i)}
					<PaperdollSlot
						slot={k}
						onpin={pin}
						onopen={toggleBag}
						onhover={hoverEnter}
						onleave={hoverLeave}
						expanded={isOpen(k)}
					/>
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

<style>
	.layout {
		display: grid;
		grid-template-columns: 1fr 290px;
		gap: 24px; /* lg */
		align-items: start;
	}
	.window {
		display: flex;
		flex-direction: column;
		gap: 16px; /* md — section → section */
		min-width: 0;
	}

	/* --- char-head (§D.1) --- */
	.charhead {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.charname {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 24px; /* Display (char-head) */
		line-height: 1.2;
		color: var(--accent);
		margin: 0;
	}
	.charmeta {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.85;
		margin: 0;
	}

	/* --- paperdoll (§E) — CHARUI-01 compaction (Phase 41 / D-01) --- */
	.paperdoll {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 16px; /* md — tightened from 24px to reclaim the loose center gap (CHARUI-01) */
		align-items: start;
	}
	.equip-col {
		display: grid;
		grid-template-columns: repeat(2, 62px);
		gap: 8px; /* sm */
	}
	/* The center figure column: the square portrait frame + (for an editor) the control
	   beneath it, at 8px (sm) below the frame. */
	.figure {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px; /* sm — frame → control */
		min-width: 0;
	}
	/* The portrait frame: a square min(190px,100%) (190px = 3 tiles + 2 gaps) — replaces the
	   260px dead-space floor (CHARUI-01). EMPTY = dashed; FILLED = solid, both on the SAME
	   --border token (dashed→solid switch, no new color). position:relative so the <img>
	   overlays the silhouette under-layer. */
	.doll {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		width: min(190px, 100%);
		aspect-ratio: 1 / 1;
		padding: 8px; /* sm — centers the fallback glyph without dead height */
		border: 1px dashed var(--border, var(--accent));
		border-radius: 4px;
		opacity: 0.8;
	}
	.doll.filled {
		border-style: solid; /* a real, framed photo — solid on the SAME token as the dashed empty */
		opacity: 1;
	}
	/* The portrait <img> overlays the silhouette; on error it is removed from the DOM and the
	   silhouette paints through (the onImgError hide-and-fall-through). */
	.portrait-img {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 4px;
	}
	.silhouette {
		font-size: 64px;
		line-height: 1;
		color: var(--text);
		opacity: 0.5;
	}
	.doll-line {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.7;
		margin: 0;
		/* Keep a long name inside the square frame rather than overflowing it. */
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* --- section eyebrows + grids (§F) --- */
	.eyebrow {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--accent);
		margin: 0 0 8px;
	}
	.bag-section {
		display: flex;
		flex-direction: column;
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, 62px);
		gap: 8px; /* sm */
	}
	.cell {
		display: contents;
	}
	.empty-grid {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.7;
		margin: 0;
	}

	/* --- inline bag expand (§F / D-04) — in-flow, never a pop-out overlay --- */
	.bag-expand {
		grid-column: 1 / -1;
		margin: 4px 0 8px;
		padding: 8px 12px;
		background: var(--panel);
		border-left: 2px solid var(--border, var(--accent));
		border-radius: 4px;
	}
	.bag-subhead {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.04em;
		text-transform: uppercase;
		opacity: 0.85;
		margin: 0 0 8px;
	}
	.bag-empty {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.7;
		margin: 0;
	}

	.hint {
		font-family: var(--font-body);
		font-size: 13px;
		opacity: 0.7;
		margin: 8px 0 0;
	}

	/* --- the transient hover preview (§G) --- */
	.preview {
		position: fixed;
		z-index: 60;
		max-width: 320px;
		min-width: 200px;
		padding: 12px;
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		box-shadow: 0 6px 24px rgba(0, 0, 0, 0.45);
		pointer-events: none;
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		/* Keep a long wiki summary inside the floating preview: wrap long tokens and clip the
		   height (the transient hover is a glance — the pinned panel scrolls the full text). */
		max-height: 60vh;
		overflow: hidden;
		overflow-wrap: anywhere;
		word-break: break-word;
	}
	.pv-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px;
		color: var(--accent);
		margin: 0 0 4px;
	}
	.pv-line {
		margin: 2px 0;
	}
	.pv-dim {
		opacity: 0.8;
		font-size: 13px;
	}

	/* --- responsive (§H): ≤900px the pin panel drops below; tiles stay 62px --- */
	@media (max-width: 900px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 640px) {
		.paperdoll {
			gap: 8px; /* sm — tighter on mobile where horizontal room is scarce (CHARUI-01) */
		}
	}
</style>
