<script lang="ts">
	// /notifications — the all-members notification page (WANT-04, 20-UI-SPEC). The
	// login-only sibling of /account + /wantlist: reachable by ANY authenticated
	// member — the AuthGate (layout) already gates login, and this page has NO officer
	// check. The owner is derived SERVER-side from the Discord session (D-02), so the
	// page never sends an owner. Data-driven, so it inherits the layout's
	// prerender=false and renders client-side via the 200.html SPA fallback.
	//
	// Layout: a single .form-card stacking the Preferences region (top) + a divider +
	// the Inbox region (bottom) — the established personal-CRUD page rhythm.

	import NotificationPrefsPanel from '$lib/components/NotificationPrefsPanel.svelte';
	import NotificationInbox from '$lib/components/NotificationInbox.svelte';
</script>

<svelte:head>
	<title>SquireBot — Notifications</title>
</svelte:head>

<section class="form-card">
	<header class="notify-intro">
		<h1 class="form-title">Notifications</h1>
		<p class="form-purpose">
			Your SquireBot alerts live here. Choose what you get pinged about, and read everything the bot
			has tried to send you.
		</p>
	</header>

	<NotificationPrefsPanel />

	<div class="divider"></div>

	<NotificationInbox />
</section>

<style>
	.form-card {
		max-width: 720px;
		padding: 24px; /* lg (20-UI-SPEC § Page structure — matches /account + /wantlist) */
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.notify-intro {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.form-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (20-UI-SPEC Typography) */
		line-height: 1.2;
		color: var(--text);
	}
	.form-purpose {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	.divider {
		border-top: 1px solid var(--border, var(--accent));
	}
</style>
