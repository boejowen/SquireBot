<script lang="ts">
	// /admin — the officer-only Admin area (ADMIN-04 + ADMIN-06, 15-UI-SPEC §
	// Information Architecture). Two sections: "Evict guildie" (EvictionForm) and
	// "Manage officers" (AdminMgmtForm).
	//
	// TWO-LAYER AUTHORIZATION (D-01/D-06, T-15-26):
	//   Layer 1 (UX): if the session is not an officer, this page renders the
	//     Officers-only refusal — a non-officer who navigates DIRECTLY to /admin
	//     (typed URL) sees the refusal, not the forms. The shell Admin nav is also
	//     suppressed for non-officers (15-04), so this is the typed-URL backstop.
	//   Layer 2 (the real gate): the server (15-03) re-checks officer status on
	//     EVERY admin endpoint and authorizes inside the write tx. The forms hand
	//     a 403 to authGuard, collapsing the WHOLE admin UI to the Officers-only
	//     refusal (server-truth) — the hidden nav + this client check are never
	//     trusted as the boundary.
	//
	// This route is data-driven, so it inherits the layout's prerender=false and
	// renders client-side via the 200.html SPA fallback (no +page.ts override).

	import { getContext } from 'svelte';
	import StateBlock from '$lib/components/StateBlock.svelte';
	import EvictionForm from '$lib/components/EvictionForm.svelte';
	import AdminMgmtForm from '$lib/components/AdminMgmtForm.svelte';
	import { SESSION_KEY, type SessionGetter } from '$lib/components/AuthGate.svelte';

	const getSession = getContext<SessionGetter>(SESSION_KEY);
	let session = $derived(getSession ? getSession() : null);
	let isOfficer = $derived(!!session?.isOfficer);
</script>

<svelte:head>
	<title>SquireBot — admin</title>
</svelte:head>

{#if !isOfficer}
	<!-- Layer-1 UX refusal for a non-officer direct-nav (the server is the gate). -->
	<StateBlock kind="officers-only" />
{:else}
	<div class="admin-area">
		<section class="form-card">
			<h2 class="form-title">Evict guildie</h2>
			<EvictionForm />
		</section>
		<section class="form-card">
			<h2 class="form-title">Manage officers</h2>
			<AdminMgmtForm />
		</section>
	</div>
{/if}

<style>
	.admin-area {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg between the two form sections (UI-SPEC) */
	}
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
