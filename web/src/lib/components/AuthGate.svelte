<script lang="ts" module>
	// The context keys descendants use to read the session and report a
	// server-truth auth error. SiteShell reads SESSION_KEY for the
	// SessionIndicator + the officer-only Admin nav; +page.svelte calls
	// AUTH_GUARD_KEY in its catch so a 401/403 re-routes the whole gate.
	export const SESSION_KEY = Symbol('session');
	export const AUTH_GUARD_KEY = Symbol('authGuard');

	import type { Session } from '$lib/auth';
	/** Descendants read the live session via this getter (null while loading). */
	export type SessionGetter = () => Session | null;
	/** Descendants pass a caught error here; the gate re-routes on a typed 401/403. */
	export type AuthGuard = (err: unknown) => void;
</script>

<script lang="ts">
	// AuthGate — the whole-site Discord login gate (D-01, 15-UI-SPEC §
	// Information Architecture). Wraps SiteShell at the layout level. On mount it
	// resolves the session via whoami-web; until then it shows an auth-loading
	// StateBlock. It renders by precedence (the pure resolveGate reducer):
	//   auth-loading → LoginScreen → NotMemberScreen → officers-only → the app.
	//
	// SERVER-TRUTH (B-2 / T-15-25): the gate exposes an `authGuard` to descendants
	// via context. ANY descendant call that throws a typed api.ts error (mount OR
	// mid-session) is fed to authGuard, which sets a server-truth `override` that
	// resolveGate honors ABOVE the cached session — so the user never sits on a
	// stale authorized view with dead buttons after the server says no:
	//   Unauthenticated (401) → drop auth state → LoginScreen.
	//   Forbidden (403)       → the MATCHING refusal (not-member → NotMemberScreen,
	//                           else Officers-only). An officer bit is NEVER cached
	//                           past a 403.
	// The frontend gate is UX; the API (15-02/15-03) is the real gate.

	import { onMount, setContext, type Snippet } from 'svelte';
	import {
		fetchSession,
		classifyAuthError,
		resolveGate,
		ANON,
		type AuthOverride
	} from '$lib/auth';
	import LoginScreen from './LoginScreen.svelte';
	import NotMemberScreen from './NotMemberScreen.svelte';
	import StateBlock from './StateBlock.svelte';

	let { children }: { children: Snippet } = $props();

	// null = still resolving (auth-loading); a Session once whoami-web answers.
	let session = $state<Session | null>(null);
	// A server-truth override from a caught 401/403 (or an early ?not_member=1
	// hint) — wins over `session` in resolveGate.
	let override = $state<AuthOverride | null>(null);

	// The screen to render, derived purely (the same reducer auth.test.ts proves).
	let gate = $derived(resolveGate(session, override));

	// Provide the live session + the guard to descendants (SiteShell, +page).
	setContext(SESSION_KEY, (() => session) satisfies SessionGetter);
	setContext(AUTH_GUARD_KEY, ((err: unknown) => {
		const verdict = classifyAuthError(err);
		if (verdict === 'unauthenticated') {
			// Drop ALL client auth state — the server says we have no session.
			session = ANON;
			override = 'unauthenticated';
		} else if (verdict === 'not_member') {
			override = 'not_member';
		} else if (verdict === 'officer_refused') {
			override = 'officer_refused';
		}
		// verdict === null → not an auth error; the caller renders its own error.
	}) satisfies AuthGuard);

	onMount(async () => {
		// An early ?not_member=1 hint from the OAuth callback bounce — show the
		// NotMember refusal immediately (the whoami resolve confirms it too).
		const params = new URLSearchParams(window.location.search);
		if (params.get('not_member') === '1') {
			override = 'not_member';
		}
		session = await fetchSession();
	});
</script>

{#if gate === 'auth-loading'}
	<StateBlock kind="auth-loading" />
{:else if gate === 'login'}
	<LoginScreen />
{:else if gate === 'not-member'}
	<NotMemberScreen />
{:else if gate === 'officers-only'}
	<StateBlock kind="officers-only" />
{:else}
	{@render children()}
{/if}
