<script lang="ts">
	// FacetBar — the Phase 39 Clicky/Haste filter chips (SEARCH-04/05, D-02), a small
	// prop-driven presentational component shared by the Inventory tab + the wishlist
	// add-form (UI-SPEC Reuse Note). NOT the Toggle.svelte ON/OFF switch — that primitive
	// is reserved for async server-write switches; a facet is a client-side filter
	// predicate, so the lighter `aria-pressed` filter-button idiom (the .tab/.seg-btn
	// vocabulary) is the established, correct pattern.
	//
	// AND-combined at the call site (the two chips are INDEPENDENT — toggling one never
	// clears the other; the parent owns the boolean state). Each chip carries three
	// active signals, never color alone (UI-SPEC Color §3): (a) aria-pressed, (b) the
	// accent fill + inverted label, (c) a leading filled-state mark in the current text
	// color. Token-only styling → all 5 EQ themes work for free (no literal hex).

	let {
		clicky,
		haste,
		onToggleClicky,
		onToggleHaste
	}: {
		clicky: boolean;
		haste: boolean;
		onToggleClicky: () => void;
		onToggleHaste: () => void;
	} = $props();
</script>

<div class="facets" role="group" aria-label="Item facets">
	<button
		type="button"
		class="facet"
		class:active={clicky}
		aria-pressed={clicky}
		onclick={onToggleClicky}
	>
		<!-- The leading filled-state mark rides the current text color (never color-alone,
		     UI-SPEC §3); aria-hidden so the screen-reader state is aria-pressed, not the glyph. -->
		<span class="mark" aria-hidden="true">{clicky ? '✓' : '+'}</span>
		<span class="label">Clicky</span>
	</button>
	<button
		type="button"
		class="facet"
		class:active={haste}
		aria-pressed={haste}
		onclick={onToggleHaste}
	>
		<span class="mark" aria-hidden="true">{haste ? '✓' : '+'}</span>
		<span class="label">Haste</span>
	</button>
</div>

<style>
	/* Two filter chips, left-aligned, wrapping on narrow viewports (UI-SPEC §1 layout). */
	.facets {
		display: inline-flex;
		align-items: center;
		gap: 8px; /* sm — chip ↔ chip */
		flex-wrap: wrap;
	}
	/* .facet mirrors .seg-btn so chips + scope segments read as one control family
	   (UI-SPEC Typography). Token-only — no literal hex, 5-theme parity for free. */
	.facet {
		display: inline-flex;
		align-items: center;
		gap: 4px; /* xs — mark ↔ label */
		min-height: 44px; /* touch target */
		padding: 8px 16px; /* sm × md */
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
	}
	.facet:hover {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.facet:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.facet.active {
		background: var(--accent);
		color: var(--bg);
	}
	/* The leading filled-state mark — inherits the chip's current text color (var(--text)
	   inactive, var(--bg) active via the fill inversion), so it is never the only signal. */
	.mark {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		line-height: 1;
		opacity: 0.85;
	}
	/* Label style (UI-SPEC Typography): 13px, display font, uppercase, 0.08em tracking. */
	.label {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
</style>
