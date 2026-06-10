<script lang="ts">
	// /my-characters — the all-members "your characters" page (ASSIGN-01/02/03, Phase
	// 26). The login-only sibling of /account + /char-meta: reachable by ANY
	// authenticated member (D-06/D-08) — the AuthGate (layout) already gates login, and
	// this page has NO officer check. The owner is derived SERVER-side from the Discord
	// session (D-02), so the page never sends an owner. Data-driven, so it inherits the
	// layout's prerender=false and renders client-side via the 200.html SPA fallback.

	import MyCharactersPanel from '$lib/components/MyCharactersPanel.svelte';
</script>

<svelte:head>
	<title>SquireBot — my characters</title>
</svelte:head>

<section class="form-card">
	<header class="intro">
		<h1 class="form-title">My characters</h1>
		<p class="form-purpose">
			Claim the characters you play so your guildies know who holds what. Release one to hand
			it back, or request one that’s already taken — an officer approves it.
		</p>
		<!-- 260610-fm5 WS3: cross-link to the sibling setup page (/char-meta links back). -->
		<p class="cross-link">
			Already claimed yours? <a href="/char-meta">Set class &amp; level</a> so gear and spell
			checks work.
		</p>
	</header>
	<MyCharactersPanel />
</section>

<style>
	.form-card {
		max-width: 720px;
		padding: 24px; /* lg (matches /account + /char-meta) */
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
		font-size: 20px; /* Heading */
		line-height: 1.2;
		color: var(--text);
	}
	.form-purpose {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	.cross-link {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	.cross-link a {
		color: var(--accent);
	}
</style>
