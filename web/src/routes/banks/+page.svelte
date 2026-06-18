<script lang="ts">
	// /banks — "coming soon" placeholder (Phase 30 / D-03). The Banks tab list +
	// valuation totals land in Phase 33 (BANK). Per D-03b the placeholder links to
	// the preserved /guild-views bank surface (seeded via ?view=bank). NO search bar
	// (D-08 — the per-bank search ships with the content).
</script>

<svelte:head>
	<title>SquireBot — Banks</title>
</svelte:head>

<section class="placeholder">
	<h1 class="placeholder-title">Banks</h1>
	<p class="placeholder-body">
		Browse the guild banks and their total value. This view is being built.
	</p>
	<a class="preserved-link" href="/guild-views?view=bank">View the current bank →</a>
</section>

<style>
	/* Dashed-border "coming soon" card (30-UI-SPEC §E). */
	.placeholder {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		max-width: 720px;
		padding: 48px 24px; /* 2xl / lg (UI-SPEC §E) */
		border: 1px dashed var(--border, var(--accent));
		border-radius: 4px;
		text-align: center;
	}
	.placeholder-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC) */
		line-height: 1.2;
		color: var(--text);
	}
	.placeholder-body {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	/* The D-03b preserved-view link — styled like the live .record-coin affordance. */
	.preserved-link {
		display: inline-flex;
		align-items: center;
		min-height: 44px; /* touch target (UI-SPEC) */
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
	.preserved-link:hover {
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}
	.preserved-link:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
