<script lang="ts">
	// /wantlist — the personal wantlist page (WANT-01/WANT-02, 19-UI-SPEC). The
	// login-only sibling of /account: reachable by ANY authenticated member — the
	// AuthGate (layout) already gates login, and this page has NO officer check. The
	// owner is derived SERVER-side from the Discord session (D-02), so the page never
	// sends an owner. Data-driven, so it inherits the layout's prerender=false and
	// renders client-side via the 200.html SPA fallback (no +page.ts override).
	//
	// 260610-fm5 WS3 (item 8): the page no longer wraps the whole panel in a 720px
	// card — the intro is passed as a snippet into WantlistPanel, which keeps it +
	// the add form in its 720px add-card while the filter bar + grids break out to
	// the full content width (mirroring the home layout: forms narrow, grids wide).
	// Snippet markup is declared HERE, so this page's scoped styles apply to it.

	import WantlistPanel from '$lib/components/WantlistPanel.svelte';
</script>

<svelte:head>
	<title>SquireBot — your wantlist</title>
</svelte:head>

<WantlistPanel>
	<header class="wantlist-intro">
		<h1 class="form-title">Your wantlist</h1>
		<p class="form-purpose">
			Track the items you're after. Each one shows whether — and where — the guild already has it.
			Add items from the catalog, or jot a custom want.
		</p>
	</header>
</WantlistPanel>

<style>
	.wantlist-intro {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.form-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (19-UI-SPEC Typography) */
		line-height: 1.2;
		color: var(--text);
	}
	.form-purpose {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
</style>
