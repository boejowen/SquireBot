<script lang="ts">
	// /bank-coin — the bank-coin entry page (ADMIN-05). Reachable by ANY
	// authenticated member (D-12): the AuthGate (layout) already gates login, and
	// this page has NO officer check — a plain member lands here from the bank
	// view's "Record coin" affordance + the shell nav. The form lives in a
	// --panel card under a Heading-20px title (15-UI-SPEC § Form Contracts).
	// This route is data-driven, so it inherits the layout's prerender=false and
	// renders client-side via the 200.html SPA fallback (no +page.ts override).

	import BankCoinForm from '$lib/components/BankCoinForm.svelte';
</script>

<svelte:head>
	<title>SquireBot — record bank coin</title>
</svelte:head>

<section class="form-card">
	<h1 class="form-title">Record bank coin</h1>
	<BankCoinForm />
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
