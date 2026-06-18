<script lang="ts">
	// /settings — the consolidated Settings tab (Phase 30 / D-05/D-06, NAV-03 + the
	// NAV-02 settings search). ONE page composing the EXISTING panels as stacked in-page
	// <section>s (NOT a menu of links — D-05). Every panel is mounted 1:1 (RESEARCH
	// "treat any panel rewrite as a smell"):
	//
	//   1. Theme           → SettingsThemePicker (the context bridge; member)
	//   2. Watcher Codes    → WatcherCodesPanel    (member; was /account)
	//   3. Set Class & Level→ CharMetaForm         (member; was /char-meta)
	//   4. My Characters    → MyCharactersPanel     (member; was /my-characters)
	//   5. Admin            → EvictionForm + AdminMgmtForm + MonitorAdminPanel +
	//                         AssignmentAdminPanel  (OFFICER-only; was /admin)
	//
	// TWO-LAYER AUTHORIZATION (T-30-05): the {#if isOfficer} gate around the Admin
	// section is Layer-1 UX ONLY — a non-officer simply never renders it. The Go API
	// (webadmin) re-checks officer on EVERY admin write and authGuard collapses the gate
	// on a 403; the hidden section is NEVER the boundary. The officer bit is read via the
	// verbatim SESSION_KEY context idiom (same as the old /admin page).
	//
	// MonitorAdminPanel stays HERE in Settings→Admin (officer config), NOT on the Wishlist
	// tab — only the member's own NotificationPrefsPanel/NotificationInbox moved there (A4).
	//
	// SETTINGS SEARCH (UI-SPEC §F2, D-05): a single live in-page section filter at the top.
	// An empty query shows all sections; otherwise a case-insensitive substring match over
	// each section's title + a fixed keyword list decides visibility. When NOTHING matches,
	// a compact dimmed "No settings match" line shows (query auto-escaped via plain {}) —
	// the page is never blanked. Each section's stable `id` also lets old deep links land
	// (D-02) and is the search's anchor.
	//
	// Login-only (any authenticated member): the AuthGate (layout) already gates login.
	// Data-driven, so it inherits the layout's prerender=false and renders client-side via
	// the 200.html SPA fallback (no +page.ts override).

	import { getContext } from 'svelte';
	import { SESSION_KEY, type SessionGetter } from '$lib/components/AuthGate.svelte';
	import SettingsThemePicker from '$lib/components/SettingsThemePicker.svelte';
	import WatcherCodesPanel from '$lib/components/WatcherCodesPanel.svelte';
	import CharMetaForm from '$lib/components/CharMetaForm.svelte';
	import MyCharactersPanel from '$lib/components/MyCharactersPanel.svelte';
	import EvictionForm from '$lib/components/EvictionForm.svelte';
	import AdminMgmtForm from '$lib/components/AdminMgmtForm.svelte';
	import MonitorAdminPanel from '$lib/components/MonitorAdminPanel.svelte';
	import AssignmentAdminPanel from '$lib/components/AssignmentAdminPanel.svelte';

	// Officer gate (verbatim /admin idiom) — Layer-1 UX over the unchanged server boundary.
	const getSession = getContext<SessionGetter>(SESSION_KEY);
	let session = $derived(getSession ? getSession() : null);
	let isOfficer = $derived(!!session?.isOfficer);

	// Section keyword sets for the live search (title + keywords, case-insensitive
	// substring). The Admin section is ALSO gated by isOfficer, so its keywords never
	// reveal it to a non-officer (T-30-08).
	const SECTIONS = [
		{ id: 'settings-theme', title: 'Theme', keywords: ['theme', 'color', 'colour', 'appearance'] },
		{
			id: 'settings-watcher-codes',
			title: 'Watcher Codes',
			keywords: ['watcher', 'code', 'pc', 'link', 'install']
		},
		{
			id: 'settings-class-level',
			title: 'Set Class & Level',
			keywords: ['class', 'level', 'race', 'character details', 'gear', 'spell']
		},
		{
			id: 'settings-my-characters',
			title: 'My Characters',
			keywords: ['characters', 'claim', 'release', 'request', 'my characters']
		},
		{
			id: 'settings-admin',
			title: 'Admin',
			keywords: ['admin', 'officer', 'monitor', 'evict', 'assignment', 'officers']
		}
	] as const;

	let query = $state('');

	// Per-section visibility predicate (empty query → all visible).
	function matches(id: string): boolean {
		const q = query.trim().toLowerCase();
		if (q === '') return true;
		const sec = SECTIONS.find((s) => s.id === id);
		if (!sec) return false;
		return (
			sec.title.toLowerCase().includes(q) || sec.keywords.some((k) => k.toLowerCase().includes(q))
		);
	}

	// "Nothing matches" is computed against the sections the user can actually SEE: the
	// Admin section only counts for an officer (so a non-officer typing "admin" correctly
	// matches nothing — T-30-08).
	let anyVisible = $derived(
		SECTIONS.some((s) => (s.id === 'settings-admin' ? isOfficer : true) && matches(s.id))
	);
	let noMatch = $derived(query.trim() !== '' && !anyVisible);
</script>

<svelte:head>
	<title>SquireBot — settings</title>
</svelte:head>

<div class="settings-area">
	<h1 class="page-title">Settings</h1>

	<div class="settings-search">
		<label class="sr-only" for="settings-search-input">Search settings</label>
		<input
			id="settings-search-input"
			type="search"
			class="search-input"
			placeholder="Search settings…"
			autocomplete="off"
			bind:value={query}
		/>
	</div>

	{#if noMatch}
		<p class="no-match">No settings match "{query}".</p>
	{/if}

	{#if matches('settings-theme')}
		<section id="settings-theme" class="form-card">
			<h2 class="form-title">Theme</h2>
			<SettingsThemePicker />
		</section>
	{/if}

	{#if matches('settings-watcher-codes')}
		<section id="settings-watcher-codes" class="form-card">
			<h2 class="form-title">Watcher Codes</h2>
			<WatcherCodesPanel />
		</section>
	{/if}

	{#if matches('settings-class-level')}
		<section id="settings-class-level" class="form-card">
			<h2 class="form-title">Set Class &amp; Level</h2>
			<CharMetaForm />
		</section>
	{/if}

	{#if matches('settings-my-characters')}
		<section id="settings-my-characters" class="form-card">
			<h2 class="form-title">My Characters</h2>
			<MyCharactersPanel />
		</section>
	{/if}

	{#if isOfficer}
		{#if matches('settings-admin')}
			<section id="settings-admin" class="form-card admin-section">
				<h2 class="form-title">Admin</h2>
				<EvictionForm />
				<AdminMgmtForm />
				<MonitorAdminPanel />
				<AssignmentAdminPanel />
			</section>
		{/if}
	{/if}
</div>

<style>
	.settings-area {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg section rhythm (UI-SPEC §F — matches /admin .admin-area) */
	}
	.page-title {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC Typography) */
		line-height: 1.2;
		color: var(--text);
	}
	.settings-search {
		max-width: 720px;
	}
	/* Search-input visual (UI-SPEC §D): full-width, panel fill, border, 44px, accent focus. */
	.search-input {
		width: 100%;
		min-height: 44px;
		padding: 8px 12px; /* sm/md */
		font-family: var(--font-body);
		font-size: 16px; /* Body */
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
	}
	.search-input::placeholder {
		color: var(--text);
		opacity: 0.6;
	}
	.search-input:focus-visible {
		border-color: var(--accent);
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.no-match {
		font-family: var(--font-body);
		font-size: 16px; /* Body, dimmed (UI-SPEC §F2 copy) */
		line-height: 1.5;
		opacity: 0.7;
	}
	.form-card {
		max-width: 720px;
		padding: 24px; /* lg (UI-SPEC §F — matches the /admin + /account form cards) */
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
	/* Visually-hidden label for the search input (accessible name without visible chrome). */
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
</style>
