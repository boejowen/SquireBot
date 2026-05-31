<script lang="ts">
	// LoginScreen — the unauthenticated whole-site gate surface (15-UI-SPEC §
	// Login & Auth Surfaces, AUTH-08). A centered card on the themed --bg:
	// wordmark → one-line purpose → "Sign in with Discord" primary button →
	// membership footnote. The button starts the SERVER-SIDE OAuth flow by
	// navigating the browser to loginUrl() (D-04 — the client secret never
	// touches the frontend; this is a navigation, not a fetch). While the
	// redirect is in flight it shows a disabled "Redirecting…" state.

	import LogIn from '@lucide/svelte/icons/log-in';
	import { loginUrl } from '$lib/auth';

	let redirecting = $state(false);

	function signIn() {
		redirecting = true;
		// Full navigation to the backend OAuth start (302 → Discord).
		window.location.href = loginUrl();
	}
</script>

<div class="login-screen">
	<div class="login-card">
		<span class="wordmark">SquireBot</span>
		<p class="purpose">Sign in with Discord to see the guild's inventory, gear, and bank.</p>
		<button type="button" class="signin" onclick={signIn} disabled={redirecting}>
			<LogIn size={18} aria-hidden="true" />
			<span>{redirecting ? 'Redirecting…' : 'Sign in with Discord'}</span>
		</button>
		<p class="footnote">You must be a member of the guild's Discord server.</p>
	</div>
</div>

<style>
	.login-screen {
		flex: 1;
		min-height: 60vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 48px 16px; /* 2xl vertical centering (UI-SPEC) */
	}
	.login-card {
		width: 100%;
		max-width: 420px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		padding: 24px; /* lg (UI-SPEC) */
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		box-shadow: 0 6px 24px rgba(0, 0, 0, 0.4);
		text-align: center;
	}
	.wordmark {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 28px; /* Display (UI-SPEC) */
		line-height: 1.2;
		color: var(--accent);
		letter-spacing: 0.02em;
	}
	.purpose {
		font-family: var(--font-body);
		font-size: 16px; /* Body (UI-SPEC) */
		line-height: 1.5;
		opacity: 0.9;
	}
	.signin {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		width: 100%;
		min-height: 44px; /* touch target (UI-SPEC) */
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
	.signin:disabled {
		opacity: 0.7;
		cursor: default;
	}
	.signin:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.footnote {
		font-family: var(--font-body);
		font-size: 13px;
		opacity: 0.7;
	}
</style>
