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
		previewEviction,
		evict,
		classifyAdminError,
		type EvictableOwner,
		type EvictionPreview
	} from '$lib/api';
	import { graceDate } from '$lib/eviction';

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

	let selectedOwner = $derived(owners.find((o) => String(o.owner_id) === selectedId) ?? null);
	let cascadeEmpty = $derived(!!preview && preview.characters.length === 0);
	// Evict is enabled only with a non-empty preview, not floor-blocked, idle.
	let canEvict = $derived(
		!!selectedOwner && !!preview && !cascadeEmpty && !floorBlocked && !evicting
	);

	// graceDate (the human grace-deadline string) is the pure $lib/eviction helper
	// — it converts the backend's epoch-SECONDS grace_until to ms before building
	// the Date (CR-02; feeding raw epoch seconds to new Date() rendered Jan 1970).

	async function load() {
		phase = 'loading';
		errorMsg = '';
		try {
			owners = await fetchEvictable();
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
	<!-- No evictable guildies — an empty picker, not the form (UI-SPEC). -->
	<StateBlock kind="view-empty" viewName="evictable guildies" />
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
		{:else if preview}
			<div class="preview">
				{#if cascadeEmpty}
					<p class="cascade-empty">No characters found for this guildie.</p>
				{:else}
					<p class="preview-heading">Characters affected ({preview.characters.length}):</p>
					<ul class="char-list">
						{#each preview.characters as c (c)}
							<li class="char">{c}</li>
						{/each}
					</ul>
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
{/if}

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
</style>
