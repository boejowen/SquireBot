<script lang="ts">
	// MyCharactersPanel — the /my-characters member self-service component (ASSIGN-
	// 01/02/03, Phase 26). Mirrors WatcherCodesPanel's runes load→phase→AuthGuard
	// lifecycle + ConfirmDialog confirm-before-commit. Three sections:
	//   1. "My characters" — the caller's assigned chars, each with a Release button
	//      that opens a ConfirmDialog (the destructive action, D-08).
	//   2. "Characters you can claim" — the unassigned, non-shared chars (D-06): an
	//      instant Claim. A char already held by someone else (partitionClaimable's
	//      assignedToOthers) renders a Request button instead (D-07) — these are empty
	//      in practice today since the backend's claimable read returns only unassigned
	//      rows, but the Claim-vs-Request split lives in the tested partitionClaimable.
	//
	// SERVER-TRUTH (D-02 / Pitfall 1): the owner is the Discord session on the server;
	// this panel never sends an owner — every body carries character_id only. A 401
	// bubbles to AuthGate → LoginScreen via authGuard (the WatcherCodesPanel route()
	// pattern); the hidden-from-anon nav is UX only, never the boundary.
	//
	// SECURITY (T-26-16 XSS): the character name is watcher/Discord-controlled — it
	// renders ONLY via plain {} interpolation (Svelte auto-escapes), NEVER the raw-HTML
	// directive. A bogus character name is inert text, never a live tag.

	import { onMount, getContext } from 'svelte';
	import StateBlock from './StateBlock.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { partitionClaimable } from '$lib/assignments';
	import {
		fetchMyCharacters,
		fetchClaimable,
		claimChar,
		releaseChar,
		requestChar,
		cancelRequest,
		classifyAdminError,
		Unauthenticated,
		type MyCharacter,
		type ClaimableCharacter
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let mine = $state<MyCharacter[]>([]);
	let claimable = $state<ClaimableCharacter[]>([]);

	// The Claim-vs-Request split (tested pure helper). unassigned → instant Claim;
	// assignedToOthers → Request an officer approves.
	let split = $derived(partitionClaimable(claimable));

	// Per-row action state (the in-flight character_id) so a single click disables only
	// that row's button, and the inline error attaches to the row it came from.
	let busyId = $state<number | null>(null);
	let actionError = $state('');
	let actionSuccess = $state('');
	// The set of character_ids the caller has an outstanding Request for this session
	// (the backend has no "my requests" read — track the optimistic local state so the
	// row can offer Cancel after a successful Request).
	let requested = $state<Set<number>>(new Set());

	// Release (confirm-before-commit) state — the one destructive action.
	let releaseTarget = $state<MyCharacter | null>(null);
	let releaseDialogOpen = $state(false);

	async function load() {
		phase = 'loading';
		try {
			const [m, c] = await Promise.all([fetchMyCharacters(), fetchClaimable()]);
			mine = m;
			claimable = c;
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	/**
	 * Route a caught error by its server-truth verdict (the WatcherCodesPanel pattern):
	 * a 401 → AuthGate → LoginScreen; a stray 403 collapses via authGuard
	 * (defense-in-depth — /my-characters is RequireSession, not officer-gated). Returns
	 * true when handled (the caller stops), false for an inline error.
	 */
	function route(err: unknown): boolean {
		if (err instanceof Unauthenticated) {
			if (authGuard) authGuard(err);
			return true;
		}
		const verdict = classifyAdminError(err);
		if (verdict === 'officers-only' || verdict === 'unauthenticated') {
			if (authGuard) authGuard(err);
			return true;
		}
		return false;
	}

	/** The inline <reason> for a caught mutation error — the server {error} code, plainly. */
	function reason(err: unknown): string {
		const code =
			err && typeof err === 'object' && 'code' in err ? (err as { code?: string }).code : undefined;
		switch (code) {
			case 'char_shared':
				return 'That character is a guild bank/bot and is shared — it can’t be claimed.';
			case 'already_assigned':
				return 'That character is already assigned to someone.';
			case 'duplicate_request':
				return 'You already have a pending request for that character.';
			default:
				return 'The server rejected the request.';
		}
	}

	async function reloadLists() {
		const [m, c] = await Promise.all([fetchMyCharacters(), fetchClaimable()]);
		mine = m;
		claimable = c;
	}

	async function doClaim(c: ClaimableCharacter) {
		if (busyId !== null) return;
		busyId = c.character_id;
		actionError = '';
		actionSuccess = '';
		try {
			await claimChar(c.character_id);
			actionSuccess = `Claimed ${c.name}.`;
			await reloadLists();
		} catch (err) {
			if (route(err)) return;
			actionError = `Couldn’t claim ${c.name}. ${reason(err)} Nothing changed.`;
		} finally {
			busyId = null;
		}
	}

	async function doRequest(c: ClaimableCharacter) {
		if (busyId !== null) return;
		busyId = c.character_id;
		actionError = '';
		actionSuccess = '';
		try {
			await requestChar(c.character_id);
			requested = new Set(requested).add(c.character_id);
			actionSuccess = `Requested ${c.name}. An officer will approve or deny it.`;
		} catch (err) {
			if (route(err)) return;
			actionError = `Couldn’t request ${c.name}. ${reason(err)} Nothing changed.`;
		} finally {
			busyId = null;
		}
	}

	async function doCancel(c: ClaimableCharacter) {
		if (busyId !== null) return;
		busyId = c.character_id;
		actionError = '';
		actionSuccess = '';
		try {
			await cancelRequest(c.character_id);
			const next = new Set(requested);
			next.delete(c.character_id);
			requested = next;
			actionSuccess = `Cancelled your request for ${c.name}.`;
		} catch (err) {
			if (route(err)) return;
			actionError = `Couldn’t cancel that request. ${reason(err)} Nothing changed.`;
		} finally {
			busyId = null;
		}
	}

	function openReleaseConfirm(c: MyCharacter) {
		if (busyId !== null) return;
		releaseTarget = c;
		releaseDialogOpen = true;
		actionError = '';
		actionSuccess = '';
	}

	async function doRelease() {
		releaseDialogOpen = false;
		const target = releaseTarget;
		if (!target || busyId !== null) return;
		busyId = target.character_id;
		actionError = '';
		actionSuccess = '';
		try {
			const { released } = await releaseChar(target.character_id);
			if (released) {
				actionSuccess = `Released ${target.name}. It’s now claimable by anyone.`;
				await reloadLists();
			} else {
				actionError = `Couldn’t release ${target.name}. No change was made.`;
			}
			releaseTarget = null;
		} catch (err) {
			if (route(err)) return;
			actionError = `Couldn’t release ${target.name}. No change was made.`;
		} finally {
			busyId = null;
		}
	}

	onMount(() => {
		void load();
	});
</script>

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else}
	<div class="my-chars">
		{#if actionSuccess}
			<p class="result success" aria-live="polite">{actionSuccess}</p>
		{/if}
		{#if actionError}
			<p class="result error" aria-live="polite">{actionError}</p>
		{/if}

		<!-- 1. My characters — Release (ConfirmDialog) per row. -->
		<section class="block" aria-label="Your characters">
			<h2 class="block-heading">Your characters ({mine.length})</h2>
			{#if mine.length === 0}
				<p class="empty-note">
					You don’t hold any characters yet. Claim one below, and it shows up here.
				</p>
			{:else}
				<ul class="char-list">
					{#each mine as c (c.character_id)}
						<li class="char-row">
							<span class="char-name">{c.name}</span>
							<button
								type="button"
								class="revoke-btn"
								disabled={busyId !== null}
								onclick={() => openReleaseConfirm(c)}
							>
								{busyId === c.character_id ? 'Releasing…' : 'Release'}
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<!-- 2. Claimable — instant Claim (unassigned) / Request (contested). -->
		<section class="block" aria-label="Characters you can claim">
			<div class="divider"></div>
			<h2 class="block-heading">Characters you can claim ({claimable.length})</h2>
			{#if claimable.length === 0}
				<p class="empty-note">No unclaimed characters right now.</p>
			{:else}
				<ul class="char-list">
					{#each split.unassigned as c (c.character_id)}
						<li class="char-row">
							<span class="char-name">{c.name}</span>
							<button
								type="button"
								class="primary"
								disabled={busyId !== null}
								onclick={() => doClaim(c)}
							>
								{busyId === c.character_id ? 'Claiming…' : 'Claim'}
							</button>
						</li>
					{/each}
					{#each split.assignedToOthers as c (c.character_id)}
						<li class="char-row">
							<span class="char-name">{c.name}</span>
							{#if requested.has(c.character_id)}
								<button
									type="button"
									class="revoke-btn"
									disabled={busyId !== null}
									onclick={() => doCancel(c)}
								>
									{busyId === c.character_id ? 'Cancelling…' : 'Cancel request'}
								</button>
							{:else}
								<button
									type="button"
									class="ghost"
									disabled={busyId !== null}
									onclick={() => doRequest(c)}
								>
									{busyId === c.character_id ? 'Requesting…' : 'Request'}
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	</div>

	<ConfirmDialog
		open={releaseDialogOpen}
		heading="Release this character?"
		body={`Releasing ${releaseTarget?.name ?? ''} returns it to unassigned — anyone in the guild can then claim it. You can re-claim it later if it’s still free.`}
		confirmLabel={`Release ${releaseTarget?.name ?? ''}`}
		onConfirm={doRelease}
		onCancel={() => {
			releaseDialogOpen = false;
			releaseTarget = null;
		}}
	/>
{/if}

<style>
	.my-chars {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg between sections */
	}
	.block {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.divider {
		border-top: 1px solid var(--border, var(--accent));
		margin-top: 8px;
	}
	.block-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading */
		line-height: 1.2;
		color: var(--text);
	}
	.empty-note {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		color: var(--text);
		opacity: 0.7;
	}
	.char-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.char-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 8px 12px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		background: var(--panel);
		flex-wrap: wrap;
	}
	.char-name {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
	}
	/* Primary (Claim) = accent fill (mirror WatcherCodesPanel .primary). */
	.primary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px; /* touch target */
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
		flex: none;
	}
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	/* Ghost (Request) = neutral outline. */
	.ghost {
		min-height: 44px;
		padding: 8px 20px;
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
		flex: none;
	}
	.ghost:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.ghost:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	/* Destructive (Release / Cancel request) = destructive-text ghost; the full fill is
	   reserved for the ConfirmDialog confirm. */
	.revoke-btn {
		flex: none;
		min-height: 44px;
		padding: 8px 16px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--destructive);
		background: none;
		border: 1px solid var(--destructive);
		border-radius: 4px;
		cursor: pointer;
	}
	.revoke-btn:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.revoke-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.result {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
	}
	.result.success {
		color: var(--status-ok);
	}
	.result.error {
		color: var(--status-missing);
	}
</style>
