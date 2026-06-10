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
	<header class="intro">
		<h1 class="form-title">Record bank coin</h1>
		<!-- 260610-fm5 WS3: purpose line (.form-purpose idiom) + the way back —
		     members land here from the bank view's "Record coin" affordance, and
		     /?view=bank reopens the home page on the Bank tab (the page validates
		     the param against its tab ids; T-fm5-04). -->
		<p class="form-purpose">
			Record the coin a bank character is holding — the watcher only sees items, so plat is
			tracked here and surfaces on the bank view.
		</p>
		<a class="back-link" href="/?view=bank">Back to bank</a>
	</header>
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
	.intro {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.form-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC Typography) */
		line-height: 1.2;
		color: var(--text);
	}
	.form-purpose {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	/* "Back to bank" — a navigational link, so it carries the 44px touch target. */
	.back-link {
		display: inline-flex;
		align-items: center;
		align-self: flex-start;
		min-height: 44px; /* touch target (UI-SPEC) */
		color: var(--accent);
		font-family: var(--font-body);
		font-size: 16px;
	}
</style>
