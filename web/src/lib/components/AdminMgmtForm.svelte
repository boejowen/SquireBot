<script lang="ts">
	// AdminMgmtForm — officer-only officer management (ADMIN-06, D-06/D-07/D-08,
	// 15-UI-SPEC § AdminMgmtForm). Ports v1's showAdminMgmtSidebar UX over HTTP:
	// list current officers (the owner-floor flagged "(owner)" + Remove suppressed
	// on the floor row for a peer, D-08), promote a signed-in member by PICK (D-07
	// — no snowflakes typed), and Remove per row through the shared ConfirmDialog
	// (confirm-before-commit, T-15-27).
	//
	// SERVER-TRUTH (B-2, T-15-26): a 403 not_authorized (or a bare 403) from ANY
	// admin call collapses the WHOLE admin UI to the Officers-only refusal via
	// authGuard (NOT a generic inline error); owner_floor_protected → the inline
	// floor message; lock_busy → the inline retry message; a 401 bubbles to the
	// AuthGate → LoginScreen. The officer bit is never cached past a 403.
	//
	// SECURITY: usernames are Discord-controlled — every interpolation is plain {}
	// (Svelte auto-escapes), never the raw-HTML directive (T-15-28). The avatar
	// `alt` is the same escaped username. Ports v1's escapeHtml/escapeAttr discipline.

	import { onMount, getContext } from 'svelte';
	import UserIcon from '@lucide/svelte/icons/user';
	import FormField from './FormField.svelte';
	import StateBlock from './StateBlock.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard, SESSION_KEY, type SessionGetter } from './AuthGate.svelte';
	import {
		fetchOfficers,
		addOfficer,
		removeOfficer,
		classifyAdminError,
		type Officer,
		type PromotableUser
	} from '$lib/api';
	import {
		showRemoveButton,
		addResultMessage,
		removeResultMessage,
		ADMIN_ERROR_COPY
	} from '$lib/admin';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);
	const getSession = getContext<SessionGetter>(SESSION_KEY);
	// The caller's Discord id drives the Layer-1 Remove suppression on the floor
	// row (D-08). Empty when unknown → suppress on the floor row for safety.
	let callerId = $derived(getSession?.()?.discordUserId ?? '');

	const FLOOR_TOOLTIP =
		'This is the maintainer. The owner-floor lockout prevents anyone else from removing them.';

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let officers = $state<Officer[]>([]);
	let promotable = $state<PromotableUser[]>([]);

	let promoteId = $state(''); // chosen promotable discord_user_id ('' = none)
	let adding = $state(false);

	// The officer currently pending a Remove confirm (null = dialog closed).
	let pendingRemove = $state<Officer | null>(null);
	let removing = $state(false);

	let successMsg = $state('');
	let errorMsg = $state('');

	let canAdd = $derived(promoteId !== '' && !adding);

	async function load() {
		phase = 'loading';
		errorMsg = '';
		try {
			const res = await fetchOfficers();
			officers = res.officers;
			promotable = res.promotable;
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	async function onAdd(e: SubmitEvent) {
		e.preventDefault();
		if (!canAdd) return;
		adding = true;
		successMsg = '';
		errorMsg = '';
		try {
			const res = await addOfficer(promoteId);
			successMsg = addResultMessage(res);
			promoteId = '';
			await reload();
		} catch (err) {
			if (route(err)) return;
			errorMsg = `Couldn't add the officer. ${reasonFrag(err)}`.trim();
		} finally {
			adding = false;
		}
	}

	function openRemove(o: Officer) {
		successMsg = '';
		errorMsg = '';
		pendingRemove = o;
	}

	async function doRemove() {
		const target = pendingRemove;
		pendingRemove = null;
		if (!target) return;
		removing = true;
		successMsg = '';
		errorMsg = '';
		try {
			const res = await removeOfficer(target.discord_user_id);
			successMsg = removeResultMessage(res);
			await reload();
		} catch (err) {
			if (route(err)) return;
			errorMsg = `Couldn't remove the officer. ${reasonFrag(err)}`.trim();
		} finally {
			removing = false;
		}
	}

	/** Re-fetch the list after a successful mutation (without flipping to loading). */
	async function reload() {
		try {
			const res = await fetchOfficers();
			officers = res.officers;
			promotable = res.promotable;
		} catch (err) {
			route(err);
		}
	}

	/**
	 * Route a caught error by its server-truth verdict (B-2). Returns true when
	 * handled (officers-only collapse via authGuard / inline owner-floor / inline
	 * lock-busy / bubbled 401) so the caller stops; false for a generic error.
	 */
	function route(err: unknown): boolean {
		const verdict = classifyAdminError(err);
		if (verdict === 'officers-only') {
			// Collapse the WHOLE admin UI to the Officers-only refusal (server-truth):
			// "You're no longer an officer." — NOT a generic inline form-error.
			if (authGuard) authGuard(err);
			return true;
		}
		if (verdict === 'owner-floor') {
			errorMsg = ADMIN_ERROR_COPY['owner-floor'];
			return true;
		}
		if (verdict === 'lock-busy') {
			errorMsg = ADMIN_ERROR_COPY['lock-busy'];
			return true;
		}
		if (verdict === 'unauthenticated') {
			if (authGuard) authGuard(err);
			return true;
		}
		return false;
	}

	/** A generic <reason> fragment for the rare non-routed error. */
	function reasonFrag(_err: unknown): string {
		return 'No changes were written.';
	}

	function avatarUrl(u: { discord_user_id: string; avatar: string | null }): string | null {
		return u.avatar && u.discord_user_id
			? `https://cdn.discordapp.com/avatars/${u.discord_user_id}/${u.avatar}.png`
			: null;
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
	<div class="admin-mgmt">
		<!-- Current officers list. -->
		<div class="officers">
			<h3 class="officers-heading">Current officers ({officers.length}):</h3>
			<ul class="officer-list">
				{#each officers as o (o.discord_user_id)}
					<li class="officer-row" title={o.is_floor ? FLOOR_TOOLTIP : undefined}>
						<span class="identity">
							{#if avatarUrl(o)}
								<img class="avatar" src={avatarUrl(o)} alt={o.username} width="24" height="24" />
							{:else}
								<span class="avatar avatar-fallback" aria-hidden="true">
									<UserIcon size={14} />
								</span>
							{/if}
							<span class="username">{o.username}</span>
							{#if o.is_floor}
								<span class="owner-tag">(owner)</span>
							{/if}
						</span>
						{#if showRemoveButton(o, callerId)}
							<button
								type="button"
								class="remove-btn"
								disabled={removing}
								onclick={() => openRemove(o)}
							>
								Remove
							</button>
						{/if}
					</li>
				{/each}
			</ul>
		</div>

		<!-- Promote a signed-in member by pick (D-07 — no snowflakes typed). -->
		{#if promotable.length === 0}
			<StateBlock kind="no-promotable-users" />
		{:else}
			<form class="promote" onsubmit={onAdd}>
				<FormField label="Promote a member" id="promote-user">
					<select id="promote-user" class="field" bind:value={promoteId}>
						<option value="">Choose a member who's signed in…</option>
						{#each promotable as p (p.discord_user_id)}
							<option value={p.discord_user_id}>{p.username}</option>
						{/each}
					</select>
				</FormField>
				<div class="actions">
					<button type="submit" class="primary" disabled={!canAdd}>
						{adding ? 'Adding…' : 'Add officer'}
					</button>
				</div>
			</form>
		{/if}

		{#if successMsg}
			<p class="result success" aria-live="polite">{successMsg}</p>
		{/if}
		{#if errorMsg}
			<p class="result error" aria-live="polite">{errorMsg}</p>
		{/if}
	</div>

	<ConfirmDialog
		open={pendingRemove !== null}
		heading="Remove officer"
		body={`Remove ${pendingRemove?.username ?? ''} as an officer? They'll no longer be able to evict guildies or manage officers. Reversible — you can re-add them here.`}
		confirmLabel={`Remove ${pendingRemove?.username ?? ''}`}
		onConfirm={doRemove}
		onCancel={() => (pendingRemove = null)}
	/>
{/if}

<style>
	.admin-mgmt {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg (UI-SPEC) */
		max-width: 640px;
	}
	.officers {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.officers-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label (UI-SPEC) */
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.officer-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.officer-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 8px 0;
		border-bottom: 1px solid var(--border, rgba(74, 101, 133, 0.3));
	}
	.identity {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-family: var(--font-body);
		font-size: 16px;
	}
	.avatar {
		width: 24px;
		height: 24px;
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
	.username {
		color: var(--text);
	}
	/* The owner-floor annotation — not accent (UI-SPEC: the (owner) tag is not accent). */
	.owner-tag {
		font-size: 13px;
		opacity: 0.7;
	}
	/* Remove = a subordinate ghost button (v1 .remove-btn; subordinate to Add). */
	.remove-btn {
		min-height: 44px; /* touch target */
		padding: 6px 12px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--text);
		background: transparent;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
		opacity: 0.8;
	}
	.remove-btn:hover {
		opacity: 1;
		border-color: var(--destructive);
		color: var(--destructive);
	}
	.remove-btn:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.remove-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.promote {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.field {
		min-height: 44px;
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
		width: 100%;
	}
	.field:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
	}
	.primary {
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
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:focus-visible {
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
