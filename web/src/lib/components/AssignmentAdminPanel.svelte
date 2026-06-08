<script lang="ts">
	// AssignmentAdminPanel — the officer /admin "Character assignments" section (ASSIGN-
	// 04/05, Phase 26). Mirrors MonitorAdminPanel's officer-form lifecycle (onMount →
	// fetch → act, with authGuard via classifyAdminError + a 403 → officers-only
	// collapse). Three blocks:
	//   1. All assignments — every live char + its current assignee, with an Assign/
	//      Reassign control (a discord_user_id field → officerAssign) and a Remove
	//      button (officerRemoveAssign). D-09: officers reassign with no request.
	//   2. Pending request queue — the contested-claim requests, each with Approve
	//      (approveRequest) / Deny (denyRequest). The status label renders via the
	//      tested requestStatusLabel (the queue is pending-only on the wire).
	//   3. Designate — a per-character 3-way radio (Guild bank / Guild bot / Neither)
	//      calling designateChar(character_id, mode); designating bank/bot clears any
	//      assignment server-side (Pitfall 6).
	//
	// TWO-LAYER AUTHORIZATION (T-26-15): the /admin page's Layer-1 {#if !isOfficer}
	// refusal + this panel's 403 → officers-only collapse are UX only; the server's
	// RequireOfficer + in-tx IsOfficerTx (26-02) is the actual gate. Every fetch 403s
	// for a non-officer, so the panel renders nothing useful to one.
	//
	// SECURITY (T-26-16 XSS): the character name + assignee/requester (discord-derived)
	// render ONLY via plain {} interpolation (Svelte auto-escapes), NEVER the raw-HTML
	// directive. A bogus name/id is inert text.

	import { onMount, getContext } from 'svelte';
	import StateBlock from './StateBlock.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { requestStatusLabel } from '$lib/assignments';
	import {
		fetchAllAssignments,
		officerAssign,
		officerRemoveAssign,
		approveRequest,
		denyRequest,
		designateChar,
		classifyAdminError,
		Unauthenticated,
		type Assignment,
		type PendingRequest,
		type DesignateMode
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let assignments = $state<Assignment[]>([]);
	let requests = $state<PendingRequest[]>([]);

	// The per-row Assign/Reassign assignee input, keyed by character_id (a discord_user_id
	// string the officer pastes — the raw API contract; there is no username picker in the
	// assignment API).
	let assigneeInput = $state<Record<number, string>>({});

	// Per-row in-flight state + inline result lines.
	let busyKey = $state<string | null>(null);
	let resultMsg = $state('');
	let resultError = $state('');

	const DESIGNATE_MODES: { mode: DesignateMode; label: string }[] = [
		{ mode: 'bank', label: 'Guild bank' },
		{ mode: 'bot', label: 'Guild bot' },
		{ mode: 'none', label: 'Neither' }
	];

	async function load() {
		phase = 'loading';
		try {
			const all = await fetchAllAssignments();
			assignments = all.assignments;
			requests = all.requests;
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	/**
	 * Route a caught error by its server-truth verdict (the MonitorAdminPanel pattern):
	 * a 401 → AuthGate → LoginScreen; a not_authorized 403 → collapse the admin UI to
	 * officers-only. Returns true when handled (the caller stops), false for inline.
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
				return 'That character is a guild bank/bot — it can’t be assigned to a member.';
			case 'invalid_input':
				return 'That user or character isn’t valid.';
			case 'not_authorized':
				return 'You’re not authorized.';
			default:
				return 'The server rejected the request.';
		}
	}

	async function reloadAll() {
		const all = await fetchAllAssignments();
		assignments = all.assignments;
		requests = all.requests;
	}

	/** Run a mutation under a busy guard with uniform result/error handling + reload. */
	async function act(key: string, fn: () => Promise<void>, success: string) {
		if (busyKey !== null) return;
		busyKey = key;
		resultMsg = '';
		resultError = '';
		try {
			await fn();
			await reloadAll();
			resultMsg = success;
		} catch (err) {
			if (route(err)) return;
			resultError = `${reason(err)} Nothing changed.`;
		} finally {
			busyKey = null;
		}
	}

	function doAssign(a: Assignment | { character_id: number; name: string }) {
		const assignee = (assigneeInput[a.character_id] ?? '').trim();
		if (assignee === '') {
			resultError = 'Enter a Discord user id to assign.';
			return;
		}
		void act(
			`assign-${a.character_id}`,
			async () => {
				await officerAssign(a.character_id, assignee);
				assigneeInput = { ...assigneeInput, [a.character_id]: '' };
			},
			`Assigned ${a.name} to ${assignee}.`
		);
	}

	function doRemove(a: Assignment) {
		void act(
			`remove-${a.character_id}`,
			async () => {
				await officerRemoveAssign(a.character_id);
			},
			`Removed the assignment for ${a.name}.`
		);
	}

	function doApprove(req: PendingRequest) {
		void act(
			`approve-${req.id}`,
			async () => {
				await approveRequest(req.id);
			},
			`Approved ${req.requester}’s request for ${req.character_name}.`
		);
	}

	function doDeny(req: PendingRequest) {
		void act(
			`deny-${req.id}`,
			async () => {
				await denyRequest(req.id);
			},
			`Denied the request for ${req.character_name}.`
		);
	}

	function doDesignate(a: Assignment, mode: DesignateMode) {
		void act(
			`designate-${a.character_id}-${mode}`,
			async () => {
				await designateChar(a.character_id, mode);
			},
			`Updated the designation for ${a.name}.`
		);
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
	<div class="assign-admin">
		{#if resultMsg}
			<p class="result success" aria-live="polite">{resultMsg}</p>
		{/if}
		{#if resultError}
			<p class="result error" aria-live="polite">{resultError}</p>
		{/if}

		<!-- 1. All assignments — assign/reassign + remove + designate. -->
		<div class="block">
			<h3 class="sub-heading">All assignments</h3>
			{#if assignments.length === 0}
				<p class="empty-note">No characters are assigned yet.</p>
			{:else}
				<ul class="assign-list">
					{#each assignments as a (a.character_id)}
						<li class="assign-row">
							<div class="assign-meta">
								<span class="char-name">{a.name}</span>
								<span class="assignee">held by {a.discord_user_id}</span>
							</div>
							<div class="assign-actions">
								<input
									class="field assignee-field"
									type="text"
									placeholder="Discord user id"
									bind:value={assigneeInput[a.character_id]}
								/>
								<button
									type="button"
									class="primary"
									disabled={busyKey !== null}
									onclick={() => doAssign(a)}
								>
									{busyKey === `assign-${a.character_id}` ? 'Saving…' : 'Reassign'}
								</button>
								<button
									type="button"
									class="revoke-btn"
									disabled={busyKey !== null}
									onclick={() => doRemove(a)}
								>
									{busyKey === `remove-${a.character_id}` ? 'Removing…' : 'Remove'}
								</button>
							</div>
							<fieldset class="designate">
								<legend class="designate-legend">Designate</legend>
								{#each DESIGNATE_MODES as d (d.mode)}
									<button
										type="button"
										class="ghost designate-btn"
										disabled={busyKey !== null}
										onclick={() => doDesignate(a, d.mode)}
									>
										{d.label}
									</button>
								{/each}
							</fieldset>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<!-- 2. Pending request queue — approve / deny. -->
		<div class="block">
			<div class="divider"></div>
			<h3 class="sub-heading">Pending requests ({requests.length})</h3>
			{#if requests.length === 0}
				<p class="empty-note">No pending requests.</p>
			{:else}
				<ul class="assign-list">
					{#each requests as req (req.id)}
						<li class="assign-row">
							<div class="assign-meta">
								<span class="char-name">{req.character_name}</span>
								<span class="assignee">
									requested by {req.requester}
									<span class="status-chip">{requestStatusLabel('pending')}</span>
								</span>
							</div>
							<div class="assign-actions">
								<button
									type="button"
									class="primary"
									disabled={busyKey !== null}
									onclick={() => doApprove(req)}
								>
									{busyKey === `approve-${req.id}` ? 'Approving…' : 'Approve'}
								</button>
								<button
									type="button"
									class="revoke-btn"
									disabled={busyKey !== null}
									onclick={() => doDeny(req)}
								>
									{busyKey === `deny-${req.id}` ? 'Denying…' : 'Deny'}
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>
{/if}

<style>
	.assign-admin {
		display: flex;
		flex-direction: column;
		gap: 24px;
		max-width: 640px;
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
	.sub-heading {
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
	.assign-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.assign-row {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		background: var(--panel);
	}
	.assign-meta {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.char-name {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
	}
	.assignee {
		font-family: var(--font-body);
		font-size: 13px;
		opacity: 0.75;
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}
	.status-chip {
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		padding: 2px 8px;
		border-radius: 4px;
		opacity: 1;
	}
	.assign-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
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
		cursor: text;
	}
	.field:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.assignee-field {
		flex: 1 1 160px;
		min-width: 0;
	}
	.designate {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		margin: 0;
		padding: 8px 0 0;
		border: none;
	}
	.designate-legend {
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
		opacity: 0.75;
		padding: 0 8px 0 0;
	}
	.primary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
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
	.ghost {
		min-height: 44px;
		padding: 8px 16px;
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
		align-self: flex-start;
	}
	.result.success {
		color: var(--status-ok);
	}
	.result.error {
		color: var(--status-missing);
	}
</style>
