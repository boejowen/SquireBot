<script lang="ts">
	// PaperdollSlot — one 62×62 in-game inventory slot tile (Phase 31, INV-01/04,
	// 31-UI-SPEC §E). Prop-driven over a single InventorySlot: the wiki item icon
	// (Item_<iconId>.png) over a deterministic colored-tile fallback (D-02), a
	// stack-count badge (count > 1), and a bag-open marker (slots > 0). A FILLED
	// slot is a <button> with an item-naming aria-label; an EMPTY equipment slot is
	// a non-interactive labelled tile (D-11) — NOT focusable.
	//
	// SECURITY (T-31-15): icon_id is an INTEGER from the trusted weekly wiki job
	// (Plan 31-01 parseIconID); the URL is `Item_${int}.png` — no guildie string in
	// the path. A bad/absent id (icon_id === 0) skips the <img> and shows the colored
	// tile; an <img> load error (onerror) hides it so the tile shows through. The
	// item NAME reaches the DOM only via the button aria-label + the count text —
	// both plain {} interpolation (Svelte auto-escapes); NO raw-HTML directive here
	// (the one sanctioned escaped-HTML sink lives in ExaminePanel).

	import type { InventorySlot } from '$lib/api';

	let {
		slot = null,
		/** The canonical slot label for an EMPTY equipment position (e.g. "NECK"). */
		label = '',
		/** Pin this item's examine (a filled, non-container click). */
		onpin,
		/** Toggle this container's inline expand (a filled bag click — slots > 0). */
		onopen,
		/** Show the transient examine preview (a filled, non-container hover/focus). */
		onhover,
		/** Dismiss the transient preview (mouse leave / blur). */
		onleave,
		/** True when this is a container (slots > 0) and its inline expand is open. */
		expanded = false
	}: {
		slot?: InventorySlot | null;
		label?: string;
		onpin?: (slot: InventorySlot) => void;
		onopen?: (slot: InventorySlot) => void;
		onhover?: (slot: InventorySlot, e: MouseEvent) => void;
		onleave?: () => void;
		expanded?: boolean;
	} = $props();

	// A filled slot has a non-empty item name (the data foundation keeps empty
	// equipment slots with item_id 0 / blank name — D-11).
	let filled = $derived(slot !== null && (slot.item?.trim() ?? '') !== '');
	// A tile is an openable BAG only when it has capacity AND lives in general/bank. WORN
	// EQUIPMENT is never a bag: real /outputfile inventory reports a non-zero Slots (aug-slot)
	// count on equipped items, which must NOT make the paperdoll tile openable — it stays a
	// plain examine tile (click pins, hover previews). 2026-06-18 worn-items-not-clickable fix.
	let isBag = $derived(filled && (slot?.slots ?? 0) > 0 && slot?.category !== 'equipment');
	let count = $derived(slot?.count ?? 0);
	let iconId = $derived(slot?.icon_id ?? 0);

	// A deterministic hue from the item name/id so the colored-tile fallback is
	// stable per item (D-02). This is the ONE sanctioned non-token color — a
	// per-item gradient, intentionally not a theme token.
	let hue = $derived(hueFor(slot));

	function hueFor(s: InventorySlot | null): number {
		if (!s) return 0;
		const key = (s.item || '') + ':' + s.id;
		let h = 0;
		for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) % 360;
		return h;
	}

	// The accessible name for a filled tile: item + (count when stacked) + (Open … for a bag).
	let ariaLabel = $derived.by(() => {
		if (!slot || !filled) return '';
		const base = count > 1 ? `${slot.item}, count ${count}` : slot.item;
		return isBag ? `Open ${slot.item}` : base;
	});

	function activate() {
		if (!slot || !filled) return;
		// A container click OPENS the bag (does NOT pin — §G); a non-container filled
		// click PINS the examine.
		if (isBag) onopen?.(slot);
		else onpin?.(slot);
	}

	function onImgError(e: Event) {
		// Hide the <img> so the colored-tile under-layer shows through (D-02).
		(e.currentTarget as HTMLImageElement).style.display = 'none';
	}
</script>

{#if filled && slot}
	<button
		type="button"
		class="slot filled"
		class:bag={isBag}
		aria-label={ariaLabel}
		aria-expanded={isBag ? expanded : undefined}
		onclick={activate}
		onmouseenter={(e) => {
			// A transient preview fires only for a filled NON-container tile (a bag
			// opens, it doesn't preview); the parent decides what to render.
			if (slot && !isBag) onhover?.(slot, e);
		}}
		onmouseleave={() => onleave?.()}
		style={`--tile-hue: ${hue};`}
	>
		<span class="ico">
			{#if iconId > 0}
				<!-- icon_id is a trusted integer (T-31-15) — the only dynamic part of the
				     src is `${iconId}` (a number), never a guildie string. -->
				<img
					src={`https://wiki.project1999.com/images/Item_${iconId}.png`}
					alt=""
					class="icon-img"
					onerror={onImgError}
				/>
			{/if}
		</span>
		{#if count > 1}
			<span class="count">{count}</span>
		{/if}
		{#if isBag}
			<span class="bag-marker" aria-hidden="true">⊞</span>
		{/if}
	</button>
{:else}
	<!-- Empty equipment slot (D-11): non-interactive, NOT focusable; labelled only. -->
	<div class="slot empty" aria-hidden="true">
		{#if label}<span class="empty-label">{label}</span>{/if}
	</div>
{/if}

<style>
	.slot {
		position: relative;
		width: 62px;
		height: 62px;
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 3px;
		overflow: hidden;
		padding: 0;
		flex: 0 0 auto;
	}
	.slot.filled {
		cursor: pointer;
		transition: transform 0.08s ease-out, border-color 0.08s ease-out, box-shadow 0.08s ease-out;
	}
	.slot.filled:hover,
	.slot.filled:focus-visible {
		border-color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
		transform: scale(1.04);
	}
	.slot.filled:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.slot.empty {
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: default;
	}
	.empty-label {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		line-height: 1.1;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--text);
		opacity: 0.45;
		text-align: center;
		padding: 0 2px;
	}
	/* The icon frame: the wiki <img> over the deterministic colored-tile fallback
	   (D-02). The gradient is the ONE sanctioned non-token color — a per-item hue. */
	.ico {
		position: absolute;
		inset: 6px;
		display: block;
		border-radius: 2px;
		background-image: linear-gradient(
			135deg,
			hsl(var(--tile-hue) 45% 30%),
			hsl(calc(var(--tile-hue) + 40) 40% 18%)
		);
	}
	.icon-img {
		width: 100%;
		height: 100%;
		image-rendering: pixelated;
		object-fit: contain;
	}
	.count {
		position: absolute;
		top: 1px;
		right: 3px;
		font-family: var(--font-body);
		font-size: 13px; /* Label */
		line-height: 1;
		font-variant-numeric: tabular-nums;
		/* UI-SPEC §Typography: count badge = var(--text) with a dark text-shadow for
		   contrast over any icon (no literal hex — token + shadow only). */
		color: var(--text);
		text-shadow: 0 0 2px rgba(0, 0, 0, 0.9), 0 0 3px rgba(0, 0, 0, 0.9);
	}
	.bag-marker {
		position: absolute;
		top: 1px;
		left: 3px;
		font-size: 13px;
		line-height: 1;
		color: var(--accent);
		text-shadow: 0 0 2px rgba(0, 0, 0, 0.9);
	}
	@media (prefers-reduced-motion: reduce) {
		.slot.filled {
			transition: none;
		}
		.slot.filled:hover,
		.slot.filled:focus-visible {
			transform: none;
		}
	}
</style>
