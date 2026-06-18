<script lang="ts">
	// /wishlist — the Wishlist tab (Phase 30 / D-03/D-07, NAV-04 content + NAV-02
	// wantlist search). It REHOMES two existing surfaces onto one route, top-to-bottom
	// (UI-SPEC §G), every component mounted 1:1 (RESEARCH "treat any panel rewrite as a
	// smell" — nothing is rewritten here):
	//
	//   1. The wantlist surface FIRST — WantlistPanel with the intro passed as a snippet
	//      child (the old /wantlist idiom, 260610-fm5 WS3 item 8). The panel's OWN
	//      filter/add bar IS the Wishlist tab's scoped search for Phase 30 (NAV-02
	//      pattern-established; the full WISH-07 cross-wishlist search is Phase 34). No
	//      separate search box is added. The per-item ping = the existing WantlistPanel
	//      mute bell (unchanged).
	//   2. A "Notifications" region SECOND — the old /notifications .form-card stack:
	//      NotificationPrefsPanel + a divider + NotificationInbox. A clear "Notifications"
	//      heading + the wantlist above it make the rehome read deliberate (UI-SPEC §G).
	//
	// The alert badge lives on the Wishlist TAB (SiteShell, Plan 01) and stays
	// authoritative with NO extra wiring here: NotificationInbox already refreshes the
	// badge count itself after a mark-read (RESEARCH §4 gotcha — this page must NOT
	// import or mutate the badge store). Empty/error states are carried by the panels
	// themselves (StateBlock — UI-SPEC §I); this page adds no new state copy.
	//
	// Login-only (any authenticated member): the AuthGate (layout) already gates login
	// and the owner is derived SERVER-side from the Discord session. Data-driven, so it
	// inherits the layout's prerender=false and renders client-side via the 200.html SPA
	// fallback (no +page.ts override).

	import WantlistPanel from '$lib/components/WantlistPanel.svelte';
	import NotificationPrefsPanel from '$lib/components/NotificationPrefsPanel.svelte';
	import NotificationInbox from '$lib/components/NotificationInbox.svelte';
</script>

<svelte:head>
	<title>SquireBot — your wishlist</title>
</svelte:head>

<div class="wishlist-page">
	<WantlistPanel>
		<header class="wantlist-intro">
			<h1 class="form-title">Your wishlist</h1>
			<p class="form-purpose">
				Track the items you're after. Each one shows whether — and where — the guild already has it.
				Add items from the catalog, or jot a custom want.
			</p>
		</header>
	</WantlistPanel>

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
</div>

<style>
	.wishlist-page {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg — the wantlist surface above, the Notifications region below (UI-SPEC §G) */
	}
	.form-card {
		max-width: 720px;
		padding: 24px; /* lg (matches the rehomed /notifications card) */
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.wantlist-intro,
	.notify-intro {
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
	.divider {
		border-top: 1px solid var(--border, var(--accent));
	}
</style>
