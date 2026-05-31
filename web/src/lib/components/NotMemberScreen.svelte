<script lang="ts">
	// NotMemberScreen — the AUTH-08 refusal for a valid Discord user who is NOT a
	// member of the configured guild server (15-UI-SPEC § NotMemberScreen). Same
	// centered card as LoginScreen: a shield-alert glyph (--status-missing) + the
	// exact heading/body + a single "Sign in as someone else" action that
	// restarts the OAuth flow (→ loginUrl()). No views, no nav — this user has no
	// authorized surface.

	import ShieldAlert from '@lucide/svelte/icons/shield-alert';
	import { loginUrl } from '$lib/auth';

	let redirecting = $state(false);

	function signInAsSomeoneElse() {
		redirecting = true;
		window.location.href = loginUrl();
	}
</script>

<div class="nm-screen">
	<div class="nm-card">
		<ShieldAlert size={36} aria-hidden="true" class="nm-icon" />
		<h1 class="nm-heading">You're not in the guild</h1>
		<p class="nm-body">
			This site is for members of the guild's Discord server. Join the guild Discord, then sign in
			again.
		</p>
		<button type="button" class="nm-action" onclick={signInAsSomeoneElse} disabled={redirecting}>
			{redirecting ? 'Redirecting…' : 'Sign in as someone else'}
		</button>
	</div>
</div>

<style>
	.nm-screen {
		flex: 1;
		min-height: 60vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 48px 16px;
	}
	.nm-card {
		width: 100%;
		max-width: 420px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		padding: 24px;
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		box-shadow: 0 6px 24px rgba(0, 0, 0, 0.4);
		text-align: center;
	}
	:global(.nm-icon) {
		color: var(--status-missing);
	}
	.nm-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC) */
		line-height: 1.2;
	}
	.nm-body {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.9;
		max-width: 42ch;
	}
	.nm-action {
		margin-top: 8px;
		width: 100%;
		min-height: 44px;
		padding: 8px 24px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--bg);
		background: var(--accent);
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	.nm-action:disabled {
		opacity: 0.7;
		cursor: default;
	}
	.nm-action:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
