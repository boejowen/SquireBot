<script lang="ts">
	// ItemTooltip — the rich-HTML item popover (WEB-04 / D-08, 14-UI-SPEC "Item
	// Tooltip Contract"). Anchored to its trigger (an Item cell, a Recommended
	// cell, or a search-result name). Opens on HOVER (pointer devices) and on
	// TAP/CLICK (touch); dismisses on OUTSIDE-TAP and Esc. Keyboard: the trigger
	// is focusable and opens on focus; Esc closes.
	//
	// SECURITY (T-14.04-01, the one HIGH-severity gate): the popover body is
	// `{@html composeItemNote(...)}`. This is the ONLY {@html} in the whole app,
	// and it is safe ONLY because composeItemNote (ported in 14-02) runs
	// escapeHtml() over every interpolated value (item/quest names, summary, and
	// the wiki URL inside the href) — a malicious item name renders as inert
	// text, never a live tag (vitest-proven in 14-02). Do NOT {@html} anything
	// else; do NOT pass un-escaped content here.

	import { composeItemNote, wikiUrlFor } from '$lib/tooltip/composeNotes';
	import type {
		PigparsePriceRow,
		WikiSummaryForNote,
		QuestLinkForNote
	} from '$lib/tooltip/composeNotes';

	let {
		itemName,
		wikiUrl,
		summary = null,
		prices = [],
		questLinks = [],
		children
	}: {
		itemName: string;
		wikiUrl: string;
		summary?: WikiSummaryForNote | null;
		prices?: PigparsePriceRow[];
		questLinks?: QuestLinkForNote[];
		/** The trigger content (the accent item link / recommended name). */
		children: import('svelte').Snippet;
	} = $props();

	let open = $state(false);
	let triggerEl: HTMLSpanElement | undefined = $state();

	// Body HTML is fully escaped by composeItemNote (14-02). Derived so it only
	// recomputes when inputs change.
	let bodyHtml = $derived(
		composeItemNote(itemName, wikiUrl || wikiUrlFor(itemName), summary, prices, questLinks)
	);

	function show() {
		open = true;
	}
	function hide() {
		open = false;
	}
	function toggle() {
		open = !open;
	}

	// Dismiss on outside-tap / Esc while open (window listeners attach only when
	// open — $effect re-runs on `open`).
	$effect(() => {
		if (!open) return;
		function onPointerDown(e: PointerEvent) {
			if (triggerEl && !triggerEl.contains(e.target as Node)) hide();
		}
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape') hide();
		}
		window.addEventListener('pointerdown', onPointerDown, true);
		window.addEventListener('keydown', onKey);
		return () => {
			window.removeEventListener('pointerdown', onPointerDown, true);
			window.removeEventListener('keydown', onKey);
		};
	});
</script>

<span
	class="tooltip-trigger"
	bind:this={triggerEl}
	role="button"
	tabindex="0"
	aria-haspopup="dialog"
	aria-expanded={open}
	onmouseenter={show}
	onmouseleave={hide}
	onfocus={show}
	onblur={hide}
	onclick={(e) => {
		// Tap/click (touch) toggles; stopPropagation so the window outside-tap
		// listener (capture phase) doesn't immediately re-close it.
		e.stopPropagation();
		toggle();
	}}
	onkeydown={(e) => {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			toggle();
		}
	}}
>
	{@render children()}

	{#if open}
		<!-- The body is escaped HTML from composeItemNote (14-02) — the sole, safe
		     {@html} sink in the app. -->
		<div class="tooltip-popover" role="dialog" aria-label="Item details">
			{@html bodyHtml}
		</div>
	{/if}
</span>

<style>
	.tooltip-trigger {
		position: relative;
		display: inline-flex;
		align-items: center;
		/* 44px min touch target via padding even where the glyph is smaller
		   (UI-SPEC Spacing exception). */
		min-height: 44px;
		cursor: pointer;
		outline: none;
	}
	.tooltip-trigger:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
	.tooltip-popover {
		position: absolute;
		top: 100%;
		left: 0;
		z-index: 50;
		margin-top: 4px;
		max-width: 360px;
		min-width: 220px;
		padding: 16px;
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		box-shadow: 0 6px 24px rgba(0, 0, 0, 0.45);
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		text-align: left;
		white-space: normal;
		animation: tooltip-fade 0.12s ease-out;
	}
	@keyframes tooltip-fade {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	/* composeItemNote markup hooks. */
	.tooltip-popover :global(.tooltip-title) {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		margin-bottom: 8px;
	}
	.tooltip-popover :global(.tooltip-wiki-link) {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		text-decoration: none;
		margin-left: 6px;
	}
	/* Named quest links in the "Used in quests:" line (ITEMUI-02) — the same accent-
	   link affordance as the wiki link, but inline in prose (no left margin). */
	.tooltip-popover :global(.tooltip-quest-link) {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		text-decoration: none;
	}
	.tooltip-popover :global(p) {
		margin: 4px 0;
	}
	.tooltip-popover :global(.tooltip-no-price) {
		opacity: 0.7;
	}
	@media (prefers-reduced-motion: reduce) {
		.tooltip-popover {
			animation: none;
		}
	}
</style>
