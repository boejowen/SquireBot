<script lang="ts">
	// /char-meta — the char-metadata entry page (CUTOVER-02, P16). Reachable by ANY
	// authenticated member (D-03): the AuthGate (layout) already gates login, and
	// this page has NO officer check — a plain member lands here from the shell nav.
	// The form lets any member set class/level/race/is_bank_toon on an existing
	// character (the fresh-start replacement for the Sheet backfill). The form lives
	// in a --panel card under a Heading-20px title (15-UI-SPEC § Form Contracts).
	// This route is data-driven, so it inherits the layout's prerender=false and
	// renders client-side via the 200.html SPA fallback (no +page.ts override).

	import CharMetaForm from '$lib/components/CharMetaForm.svelte';
</script>

<svelte:head>
	<title>SquireBot — set character details</title>
</svelte:head>

<section class="form-card">
	<h1 class="form-title">Set character details</h1>
	<CharMetaForm />
</section>

<style>
	.form-card {
		max-width: 720px;
		padding: 24px; /* lg (UI-SPEC § Form Contracts) */
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.form-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC Typography) */
		line-height: 1.2;
		color: var(--text);
	}
</style>
