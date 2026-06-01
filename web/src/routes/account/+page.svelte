<script lang="ts">
	// /account — the all-members "your watcher codes" page (LINK-01/03/04/05,
	// 17-UI-SPEC). The login-only sibling of /char-meta: reachable by ANY
	// authenticated member (D-09) — the AuthGate (layout) already gates login, and
	// this page has NO officer check. The owner is derived SERVER-side from the
	// Discord session (D-02), so the page never sends an owner. Like /char-meta it
	// is data-driven, so it inherits the layout's prerender=false and renders
	// client-side via the 200.html SPA fallback (no +page.ts override).

	import WatcherCodesPanel from '$lib/components/WatcherCodesPanel.svelte';
</script>

<svelte:head>
	<title>SquireBot — your watcher codes</title>
</svelte:head>

<section class="form-card">
	<header class="account-intro">
		<h1 class="form-title">Your watcher codes</h1>
		<p class="form-purpose">
			A watcher code links a PC to your account. Paste one into the SquireBot watcher on each PC
			you run — generate a separate code per PC.
		</p>
	</header>
	<WatcherCodesPanel />
</section>

<style>
	.form-card {
		max-width: 720px;
		padding: 24px; /* lg (17-UI-SPEC § Page Layout — matches /char-meta) */
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.account-intro {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.form-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (17-UI-SPEC Typography) */
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
