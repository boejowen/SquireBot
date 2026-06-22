<script lang="ts">
	// EvictionForm — officer-only guildie eviction (ADMIN-04, D-09/D-10,
	// 15-UI-SPEC § EvictionForm). Ports v1's showEvictionSidebar UX over HTTP:
	// pick a guildie → preview the character cascade + the 30-day grace + the
	// guild-code-revoke consequence → an explicit ConfirmDialog → commit. The
	// destructive commit is gated by the shared accessible ConfirmDialog (W-5,
	// confirm-before-commit, T-15-27) — a misclick cannot evict.
	//
	// SERVER-TRUTH (B-2, T-15-26): the API is the gate. A 403 from ANY admin call
	// collapses the WHOLE admin UI to the Officers-only refusal via authGuard (the
	// "you're no longer an officer" path — NOT a generic form-error), EXCEPT a
	// 403 owner_floor_protected, which is the inline floor block. A 401 bubbles to
	// AuthGate → LoginScreen. The officer bit is never cached past a 403.
	//
	// SECURITY: guildie labels + char names + error reasons are user/Discord-
	// controlled — every interpolation is plain {} (Svelte auto-escapes), never
	// the raw-HTML directive (T-15-28). Ports v1's escapeHtml discipline.

	import { onMount, getContext } from 'svelte';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import FormField from './FormField.svelte';
	import StateBlock from './StateBlock.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import {
		fetchEvictable,
		fetchRestorable,
		previewEviction,
		evict,
		restoreEviction,
		classifyAdminError,
		type EvictableOwner,
		type EvictionPreview,
		type RestorableOwner
	} from '$lib/api';
	import { canEvictPreview, evictPreviewSummary, graceDate, restoreResultMessage } from '$lib/eviction';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	const CONSEQUENCE =
		"Evicting also revokes this guildie's guild code right away, so their watcher stops uploading. Their data auto-archives after 30 days. Reversible during the grace period — you can restore them and re-issue a code.";

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let owners = $state<EvictableOwner[]>([]);

	let selectedId = $state(''); // owner_id as a string ('' = none)
	let preview = $state<EvictionPreview | null>(null);
	let previewing = $state(false);

	let dialogOpen = $state(false);
	let evicting = $state(false);
	let successMsg = $state('');
	let errorMsg = $state('');
	// Set when the SERVER says owner_floor_protected — the inline floor block.
	let floorBlocked = $state(false);

	// --- Restore (close G-1): the evicted-within-grace guildies + a Restore row.
	// Loaded alongside the evictable picker on mount. An empty list renders nothing
	// (it is NOT an error — most of the time no one is in grace).
	let restorable = $state<RestorableOwner[]>([]);
	let restoreTarget = $state<RestorableOwner | null>(null); // the row pending confirm
	let restoreDialogOpen = $state(false);
	let restoring = $state(false);
	let restoreSuccessMsg = $state('');
	let restoreErrorMsg = $state('');

	let selectedOwner = $derived(owners.find((o) => String(o.owner_id) === selectedId) ?? null);
	// The pure preview-shape classification (D-06) — drives both the gate and the
	// render branch so the two can never drift. 'cascade' / 'code-only' / 'empty'.
	let previewSummary = $derived(preview ? evictPreviewSummary(preview) : null);
	// Evict is enabled when the preview has something to act on (a cascade OR an
	// all-shared code-only revoke — D-06), and not floor-blocked / mid-evict. A
	// genuine zero-live-chars owner (canEvictPreview === false) stays disabled.
	let canEvict = $derived(
		!!selectedOwner && !!preview && canEvictPreview(preview) && !floorBlocked && !evicting
	);

	// graceDate (the human grace-deadline string) is the pure $lib/eviction helper
	// — it converts the backend's epoch-SECONDS grace_until to ms before building
	// the Date (CR-02; feeding raw epoch seconds to new Date() rendered Jan 1970).

	async function load() {
		phase = 'loading';
		errorMsg = '';
		try {
			// Both pickers in one pass — the evictable set drives `phase`, the
			// restorable set (close G-1) is rendered as its own section below. A 401/403
			// on either routes through the SAME server-truth handler.
			const [ev, re] = await Promise.all([fetchEvictable(), fetchRestorable()]);
			owners = ev;
			restorable = re;
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	async function onSelect() {
		successMsg = '';
		errorMsg = '';
		floorBlocked = false;
		preview = null;
		if (!selectedOwner) return;
		previewing = true;
		try {
			preview = await previewEviction(selectedOwner.owner_id);
		} catch (err) {
			if (route(err)) return;
			errorMsg = `Eviction failed: ${reason(err)}. No changes were written.`;
		} finally {
			previewing = false;
		}
	}

	function openConfirm() {
		if (!canEvict) return;
		dialogOpen = true;
	}

	async function doEvict() {
		dialogOpen = false;
		if (!selectedOwner || !canEvict) return;
		evicting = true;
		successMsg = '';
		errorMsg = '';
		try {
			const res = await evict(selectedOwner.owner_id);
			successMsg = `Marked ${res.removed_count} character(s) as removed and revoked the guild code. Grace until ${graceDate(res.grace_until)}.`;
			// Drop the now-evicted owner from the picker + reset.
			owners = owners.filter((o) => o.owner_id !== selectedOwner!.owner_id);
			selectedId = '';
			preview = null;
		} catch (err) {
			if (route(err)) return;
			errorMsg = `Eviction failed: ${reason(err)}. No changes were written.`;
		} finally {
			evicting = false;
		}
	}

	// --- Restore handlers (close G-1) ----------------------------------------

	function openRestoreConfirm(owner: RestorableOwner) {
		if (restoring) return;
		restoreTarget = owner;
		restoreDialogOpen = true;
	}

	async function doRestore() {
		restoreDialogOpen = false;
		const target = restoreTarget;
		if (!target || restoring) return;
		restoring = true;
		restoreSuccessMsg = '';
		restoreErrorMsg = '';
		try {
			const res = await restoreEviction(target.owner_id);
			// WR-02: the re-minted code is server-side only — the copy says so and never
			// implies it is shown here. Both outcomes (issued / mint-failed) are mapped
			// by the pure restoreResultMessage helper. The label renders via {} below.
			restoreSuccessMsg = restoreResultMessage(res, target.label);
			// Drop the now-restored owner from the list.
			restorable = restorable.filter((o) => o.owner_id !== target.owner_id);
			restoreTarget = null;
		} catch (err) {
			// Same server-truth router as the evict path: officers-only collapse / 401
			// bubble / owner-floor inline; a grace_expired 409 falls through to generic.
			if (route(err)) return;
			restoreErrorMsg = `Restore failed: ${reason(err)}. No changes were written.`;
		} finally {
			restoring = false;
		}
	}

	/**
	 * Route a caught error by its server-truth verdict (B-2). Returns true when
	 * the error was handled as a re-route (officers-only collapse via authGuard,
	 * the inline owner-floor block, or a bubbled 401) so the caller stops. Returns
	 * false for a generic error (the caller renders the inline form-error).
	 */
	function route(err: unknown): boolean {
		const verdict = classifyAdminError(err);
		if (verdict === 'officers-only') {
			// Collapse the WHOLE admin UI to the Officers-only refusal (server-truth).
			if (authGuard) authGuard(err);
			return true;
		}
		if (verdict === 'owner-floor') {
			floorBlocked = true;
			return true;
		}
		if (verdict === 'unauthenticated') {
			// Bubble to the AuthGate → LoginScreen.
			if (authGuard) authGuard(err);
			return true;
		}
		// 'lock-busy' is not expected on eviction; treat as generic inline.
		return false;
	}

	/** The <reason> fragment for the inline error copy (the server code, plainly). */
	function reason(err: unknown): string {
		const code =
			err && typeof err === 'object' && 'code' in err ? (err as { code?: string }).code : undefined;
		if (code === 'grace_expired') return 'the grace period has expired';
		return 'the server rejected the request';
	}

	onMount(() => {
		void load();
	});
</script>

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else if owners.length === 0}
	<!-- No evictable guildies — an empty picker, not the form (UI-SPEC). The
	     Restore section still renders below (an evicted guildie may be in grace
	     even when no live guildie is currently evictable). -->
	<StateBlock kind="view-empty" viewName="evictable guildies" />
	{@render restoreSection()}
{:else}
	<div class="evict-form">
		<FormField label="Guildie" id="evict-owner">
			<select id="evict-owner" class="field" bind:value={selectedId} onchange={onSelect}>
				<option value="">Choose a guildie…</option>
				{#each owners as o (o.owner_id)}
					<option value={String(o.owner_id)}>{o.label} ({o.char_count})</option>
				{/each}
			</select>
		</FormField>

		{#if previewing}
			<p class="hint" aria-live="polite">Loading preview…</p>
		{:else if preview && previewSummary}
			<div class="preview">
				{#if previewSummary.kind === 'cascade'}
					<p class="preview-heading">Characters affected ({preview.characters.length}):</p>
					<ul class="char-list">
						{#each preview.characters as c (c)}
							<li class="char">{c}</li>
						{/each}
					</ul>
				{:else}
					<!-- 'code-only' (all-shared owner: nothing removed but the code is still
					     revoked — the button stays ENABLED) and 'empty' (zero live chars —
					     the button stays DISABLED) both render the helper's plain-text
					     message via auto-escaping {} (never the raw-HTML directive, T-15-28).
					     The button state is the only difference and canEvict already reflects it. -->
					<p class="cascade-empty">{previewSummary.message}</p>
				{/if}
				<p class="grace">Grace expires: {graceDate(preview.grace_until)} (30 days from today).</p>
				<div class="consequence" role="note">
					<TriangleAlert size={18} aria-hidden="true" class="consequence-icon" />
					<span>{CONSEQUENCE}</span>
				</div>
			</div>
		{/if}

		{#if floorBlocked}
			<p class="floor-block" aria-live="polite">
				Owner-floor protected — a peer officer can't evict the maintainer's data.
			</p>
		{/if}

		<div class="actions">
			{#if successMsg}
				<p class="result success" aria-live="polite">{successMsg}</p>
			{/if}
			{#if errorMsg}
				<p class="result error" aria-live="polite">{errorMsg}</p>
			{/if}
			<button type="button" class="primary destructive" disabled={!canEvict} onclick={openConfirm}>
				{evicting ? 'Evicting…' : 'Evict guildie'}
			</button>
		</div>
	</div>

	<ConfirmDialog
		open={dialogOpen}
		heading="Evict guildie"
		body={`This marks ${preview?.characters.length ?? 0} character(s) as removed and revokes ${selectedOwner?.label ?? 'this guildie'}'s guild code right away. Their data auto-archives after 30 days. Reversible during the grace period — you can restore them and re-issue a code.`}
		confirmLabel={`Evict ${selectedOwner?.label ?? ''}`}
		onConfirm={doEvict}
		onCancel={() => (dialogOpen = false)}
	/>

	{@render restoreSection()}
{/if}

<!--
	Restore evicted guildies (close G-1). Rendered in BOTH ready branches (whether or
	not there is a live evictable guildie) via this snippet. An empty restorable list
	renders a quiet note — it is NOT an error. Each row offers a Restore button gated
	by a ConfirmDialog (confirm-before-commit). Re-mints a guild code that is NOT
	web-deliverable (WR-02) — the success copy makes that explicit. Every guildie
	label renders via plain {} (Svelte auto-escapes; never the raw-HTML directive,
	T-15-28).
-->
{#snippet restoreSection()}
	<section class="restore-section" aria-label="Restore evicted guildies">
		<h3 class="restore-heading">Restore evicted guildies</h3>
		{#if restoreSuccessMsg}
			<p class="result success" aria-live="polite">{restoreSuccessMsg}</p>
		{/if}
		{#if restoreErrorMsg}
			<p class="result error" aria-live="polite">{restoreErrorMsg}</p>
		{/if}
		{#if restorable.length === 0}
			<p class="restore-empty">No evicted guildies are within the grace period.</p>
		{:else}
			<ul class="restore-list">
				{#each restorable as o (o.owner_id)}
					<li class="restore-row">
						<span class="restore-info">
							<span class="restore-label">{o.label}</span>
							<span class="restore-meta"
								>{o.char_count} character(s) · grace until {graceDate(o.grace_until)}</span
							>
						</span>
						<button
							type="button"
							class="primary restore-btn"
							disabled={restoring}
							onclick={() => openRestoreConfirm(o)}
						>
							{restoring && restoreTarget?.owner_id === o.owner_id ? 'Restoring…' : 'Restore'}
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<ConfirmDialog
		open={restoreDialogOpen}
		heading="Restore guildie"
		body={`This un-removes ${restoreTarget?.char_count ?? 0} character(s) for ${restoreTarget?.label ?? 'this guildie'} and re-issues a guild code. The new code is retrieved on the server (from the logs / \`mint-code\`) and must be handed off out-of-band — it is not shown here.`}
		confirmLabel={`Restore ${restoreTarget?.label ?? ''}`}
		onConfirm={doRestore}
		onCancel={() => {
			restoreDialogOpen = false;
			restoreTarget = null;
		}}
	/>
{/snippet}

<style>
	.evict-form {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg (UI-SPEC) */
		max-width: 640px;
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
	.preview {
		display: flex;
		flex-direction: column;
		gap: 8px;
		font-family: var(--font-body);
		font-size: 16px;
	}
	.preview-heading {
		font-weight: 600;
	}
	.char-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		list-style: none;
		margin: 0;
		padding: 0 0 0 8px;
	}
	/* Destructive framing: the "will be removed" chars in the status-missing tint. */
	.char {
		color: var(--status-missing);
	}
	.cascade-empty {
		opacity: 0.85;
	}
	.grace {
		font-size: 13px;
		opacity: 0.75;
	}
	/* The D-10 consequence callout — destructive tint + the triangle glyph. */
	.consequence {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 12px;
		font-size: 16px;
		line-height: 1.5;
		border: 1px solid var(--destructive);
		border-radius: 4px;
		background: color-mix(in srgb, var(--destructive) 8%, transparent);
	}
	:global(.consequence-icon) {
		color: var(--destructive);
		flex: none;
		margin-top: 2px;
	}
	.floor-block {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--status-missing);
	}
	.hint {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.75;
	}
	.actions {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 8px;
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
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	/* The Evict primary uses the destructive token (color is paired with the
	   explicit "Evict" label + the consequence callout — never color-only). */
	.primary.destructive {
		color: var(--bg);
		background: var(--destructive);
	}
	:global([data-theme='heavy']) .primary.destructive {
		color: var(--accent);
	}
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	/* --- Restore section (close G-1) --- */
	.restore-section {
		display: flex;
		flex-direction: column;
		gap: 12px;
		max-width: 640px;
		margin-top: 32px;
		padding-top: 24px;
		border-top: 1px solid var(--border, var(--accent));
	}
	.restore-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 16px;
		letter-spacing: 0.04em;
		margin: 0;
	}
	.restore-empty {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.75;
		margin: 0;
	}
	.restore-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.restore-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 8px 12px;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		background: var(--panel);
	}
	.restore-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		font-family: var(--font-body);
	}
	.restore-label {
		font-size: 16px;
		font-weight: 600;
		color: var(--text);
	}
	.restore-meta {
		font-size: 13px;
		opacity: 0.75;
	}
	/* The Restore action is non-destructive — the standard accent primary, NOT the
	   destructive token the Evict button uses. */
	.restore-btn {
		color: var(--bg);
		background: var(--accent);
		flex: none;
	}
	:global([data-theme='heavy']) .restore-btn {
		color: var(--bg);
	}
</style>
