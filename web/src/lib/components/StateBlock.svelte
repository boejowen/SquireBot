<script lang="ts" module>
	// StateBlock — the shared empty / error / loading / no-results / no-coin
	// presentation (14-UI-SPEC Copywriting Contract). All copy is the EXACT
	// UI-SPEC string; tone is plain + guild-casual. Color is never load-bearing
	// here — these are text blocks.
	export type StateKind =
		| 'empty'
		| 'view-empty'
		| 'error'
		| 'loading'
		| 'no-results'
		| 'no-coin'
		// 15-04: the auth lifecycle states (AuthGate resolution + the direct-nav
		// officers-only refusal). Copy is verbatim from the 15-UI-SPEC Copywriting
		// Contract.
		| 'auth-loading'
		| 'officers-only'
		// 15-05: the picker-empty states for the three forms (when there's nothing
		// to act on). Copy is verbatim from the 15-UI-SPEC Copywriting Contract.
		| 'no-bank-toons'
		| 'no-promotable-users'
		// 17-03: the /account page empty state — the caller has zero active watcher
		// codes (brand-new guildie, or after revoking their last). Copy is verbatim
		// from the 17-UI-SPEC Copywriting Contract.
		| 'no-codes'
		// 19-03: the /wantlist page empty state — the caller has zero wants (a fresh
		// account, or after removing the last). Copy is verbatim from the 19-UI-SPEC
		// Copywriting Contract.
		| 'no-wants'
		// 20-04: the /notifications inbox empty state — the caller has zero alert_log
		// rows yet (a fresh account, or no monitor has matched a want). Copy is
		// verbatim from the 20-UI-SPEC Copywriting Contract. The Preferences region
		// stays visible above this block.
		| 'no-notifications';
</script>

<script lang="ts">
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import ShieldAlert from '@lucide/svelte/icons/shield-alert';

	let {
		kind,
		/** view-empty: the friendly view name ("inventory" / "gear check" / ...). */
		viewName = '',
		/** no-results: the search query (rendered via plain {query} — auto-escaped). */
		query = '',
		/** error: re-fires the fetch. */
		onRetry
	}: {
		kind: StateKind;
		viewName?: string;
		query?: string;
		onRetry?: () => void;
	} = $props();
</script>

{#if kind === 'loading'}
	<div class="state state-loading" role="status" aria-live="polite">
		<div class="skeleton" aria-hidden="true">
			<span class="shimmer"></span>
			<span class="shimmer"></span>
			<span class="shimmer"></span>
		</div>
		<p class="state-body">Loading…</p>
	</div>
{:else if kind === 'auth-loading'}
	<!-- Brief while AuthGate resolves the session against whoami-web (15-UI-SPEC). -->
	<div class="state state-loading" role="status" aria-live="polite">
		<div class="skeleton" aria-hidden="true">
			<span class="shimmer"></span>
			<span class="shimmer"></span>
		</div>
		<p class="state-body">Checking your access…</p>
	</div>
{:else if kind === 'officers-only'}
	<!-- Direct-nav-to-/admin refusal for a non-officer (15-UI-SPEC Copywriting). -->
	<div class="state state-empty" role="alert">
		<ShieldAlert size={28} aria-hidden="true" class="state-icon" />
		<h2 class="state-heading">Officers only</h2>
		<p class="state-body">
			This area is for guild officers. If you think you should have access, ask an officer to add
			you.
		</p>
	</div>
{:else if kind === 'no-bank-toons'}
	<!-- BankCoinForm: no is_bank_toon character exists yet (15-UI-SPEC Copywriting). -->
	<div class="state state-empty">
		<h2 class="state-heading">No bank characters yet</h2>
		<p class="state-body">
			No character is marked as a bank toon yet. Once one is, you can record its coin here.
		</p>
	</div>
{:else if kind === 'no-promotable-users'}
	<!-- AdminMgmtForm: no signed-in non-officer to promote (15-UI-SPEC Copywriting). -->
	<div class="state state-empty">
		<h2 class="state-heading">No one to promote yet</h2>
		<p class="state-body">
			Only members who've signed in at least once can be promoted. Once another member signs in,
			they'll show up here.
		</p>
	</div>
{:else if kind === 'no-codes'}
	<!-- WatcherCodesPanel: the caller has no active watcher codes yet (17-UI-SPEC
	     Copywriting). Reuses the circle-alert glyph + the shared empty layout. The
	     Generate CTA stays visible above this block (one click to the next step). -->
	<div class="state state-empty">
		<CircleAlert size={28} aria-hidden="true" class="state-icon" />
		<h2 class="state-heading">No watcher codes yet</h2>
		<p class="state-body">
			You haven't linked any PCs yet. Generate a code above, then paste it into the SquireBot
			watcher on the PC you want to link.
		</p>
	</div>
{:else if kind === 'no-wants'}
	<!-- WantlistPanel: the caller has no wantlist entries yet (19-UI-SPEC
	     Copywriting). Reuses the circle-alert glyph + the shared empty layout. The
	     add-item block stays visible above this block (one search to the next step). -->
	<div class="state state-empty">
		<CircleAlert size={28} aria-hidden="true" class="state-icon" />
		<h2 class="state-heading">Your wantlist is empty</h2>
		<p class="state-body">
			Search the catalog above to add what you're after — you'll see right away whether the guild
			already has it.
		</p>
	</div>
{:else if kind === 'no-notifications'}
	<!-- NotificationInbox: the caller has zero alert rows yet (20-UI-SPEC
	     Copywriting). Reuses the circle-alert glyph + the shared empty layout. The
	     Preferences region stays visible above this block. -->
	<div class="state state-empty">
		<CircleAlert size={28} aria-hidden="true" class="state-icon" />
		<h2 class="state-heading">No alerts yet</h2>
		<p class="state-body">
			You'll see your SquireBot alerts here once a monitor matches one of your wants. Make sure your
			wantlist has the items you're after.
		</p>
	</div>
{:else if kind === 'error'}
	<div class="state state-error" role="alert">
		<CircleAlert size={28} aria-hidden="true" class="state-icon" />
		<h2 class="state-heading">Couldn't load the data</h2>
		<p class="state-body">
			The SquireBot server didn't respond. Check your connection and try again.
		</p>
		{#if onRetry}
			<button class="retry" type="button" onclick={onRetry}>Retry</button>
		{/if}
	</div>
{:else if kind === 'empty'}
	<div class="state state-empty">
		<h2 class="state-heading">No characters yet</h2>
		<p class="state-body">
			No one's uploaded inventory yet. Once a guildie's watcher syncs, their characters show up
			here automatically.
		</p>
	</div>
{:else if kind === 'view-empty'}
	<div class="state state-empty">
		<h2 class="state-heading">Nothing to show here</h2>
		<p class="state-body">No {viewName} data for any character yet — it'll appear after the next sync.</p>
	</div>
{:else if kind === 'no-coin'}
	<!-- bank coin is null in P14 (ADMIN-05 fills it in P15) — never a fabricated 0 pp. -->
	<p class="coin-note">Coin: not yet recorded</p>
{:else if kind === 'no-results'}
	<div class="state state-empty">
		<!-- The query is interpolated via plain Svelte `{}` interpolation; Svelte
		     auto-escapes it (T-14.04-02 reflected-XSS mitigation) — never the
		     raw-HTML directive. -->
		<h2 class="state-heading">No matches for "{query}"</h2>
		<!-- The "Did you mean?" suggestion body lives in SearchResults (it owns the
		     clickable accent link). This block renders only the heading + the
		     no-suggestion fallback; SearchResults passes its own body when a
		     suggestion exists. -->
	</div>
{/if}

<style>
	.state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 48px 16px; /* 2xl vertical centering block (UI-SPEC) */
		text-align: center;
		color: var(--text);
	}
	.state-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px;
		line-height: 1.2;
	}
	.state-body {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		max-width: 42ch;
		opacity: 0.85;
	}
	:global(.state-icon) {
		color: var(--status-missing);
	}
	.retry {
		margin-top: 8px;
		min-height: 44px;
		padding: 8px 24px;
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: var(--weight-display);
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--bg);
		background: var(--accent);
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	.retry:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.coin-note {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.85;
		padding: 8px 0;
	}
	/* Loading skeleton — brief shimmer rows (payloads are small). */
	.skeleton {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: min(420px, 80%);
	}
	.shimmer {
		height: 16px;
		border-radius: 4px;
		background: linear-gradient(
			90deg,
			color-mix(in srgb, var(--accent) 6%, transparent),
			color-mix(in srgb, var(--accent) 14%, transparent),
			color-mix(in srgb, var(--accent) 6%, transparent)
		);
		background-size: 200% 100%;
		animation: shimmer 1.2s ease-in-out infinite;
	}
	@media (prefers-reduced-motion: reduce) {
		.shimmer {
			animation: none;
		}
	}
	@keyframes shimmer {
		from {
			background-position: 200% 0;
		}
		to {
			background-position: -200% 0;
		}
	}
</style>
