<script lang="ts">
	// SessionIndicator — the signed-in identity in the shell header (15-UI-SPEC §
	// SessionIndicator, AUTH-09). Renders the Discord avatar (CDN image, or a
	// `user` glyph fallback when no avatar) + the username; an officer gets a
	// `shield` badge (accessible label "Officer") before the name. A "Sign out"
	// control (log-out glyph) destroys the session then returns to `/`.
	//
	// SECURITY (T-15-22): the username is user-controlled (Discord). It renders
	// ONLY via plain {} interpolation (Svelte auto-escapes) — never the raw-HTML
	// directive — and the avatar `alt` is the same escaped username. A malicious
	// display name is inert text, never a live tag.

	import UserIcon from '@lucide/svelte/icons/user';
	import Shield from '@lucide/svelte/icons/shield';
	import LogOut from '@lucide/svelte/icons/log-out';
	import { logout, type Session } from '$lib/auth';

	let { session }: { session: Session } = $props();

	let signingOut = $state(false);

	// Discord CDN avatar URL (PNG); only when both id + avatar hash are present.
	let avatarUrl = $derived(
		session.avatar && session.discordUserId
			? `https://cdn.discordapp.com/avatars/${session.discordUserId}/${session.avatar}.png`
			: null
	);

	async function signOut() {
		if (signingOut) return;
		signingOut = true;
		await logout();
		window.location.href = '/';
	}
</script>

<div class="session">
	<span class="identity">
		{#if avatarUrl}
			<img class="avatar" src={avatarUrl} alt={session.username} width="28" height="28" />
		{:else}
			<span class="avatar avatar-fallback" aria-hidden="true">
				<UserIcon size={16} />
			</span>
		{/if}
		{#if session.isOfficer}
			<Shield size={14} aria-label="Officer" class="officer-badge" />
		{/if}
		<span class="username">{session.username}</span>
	</span>
	<button type="button" class="signout" onclick={signOut} disabled={signingOut}>
		<LogOut size={16} aria-hidden="true" />
		<span>Sign out</span>
	</button>
</div>

<style>
	.session {
		display: inline-flex;
		align-items: center;
		gap: 12px;
	}
	.identity {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.avatar {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		object-fit: cover;
		border: 1px solid var(--border, var(--accent));
	}
	.avatar-fallback {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--panel);
		color: var(--text);
	}
	:global(.officer-badge) {
		color: var(--accent);
		flex: none;
	}
	.username {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label (UI-SPEC) */
		letter-spacing: 0.04em;
		color: var(--text);
	}
	.signout {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px 12px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
		background: none;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
		opacity: 0.85;
	}
	.signout:hover {
		opacity: 1;
		color: var(--accent);
	}
	.signout:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.signout:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
