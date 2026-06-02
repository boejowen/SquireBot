<script lang="ts" module>
	// Pure, DOM-free date helpers split into the module block so they're unit-
	// testable under the node vitest project (the repo runs vitest with NO jsdom;
	// the same split ConfirmDialog uses). They never throw on a bad/empty string —
	// they fall back to the raw value so a malformed timestamp degrades gracefully
	// rather than rendering "Invalid Date" or crashing the row.

	/** "Created May 12, 2026" form — a human absolute date (or the raw string on parse failure). */
	export function formatCreated(iso: string): string {
		const ms = Date.parse(iso);
		if (Number.isNaN(ms)) return iso;
		return new Date(ms).toLocaleDateString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	/**
	 * "last used 5 min ago" / "never used yet" — relative-time meta for a code's
	 * last_seen. null ⇒ never (NOT an error — the guildie may not have pasted it).
	 * `now` is injectable for deterministic tests.
	 */
	export function formatLastSeen(lastSeen: string | null, now: number = Date.now()): string {
		if (lastSeen === null || lastSeen === '') return 'never used yet';
		const ms = Date.parse(lastSeen);
		if (Number.isNaN(ms)) return `last used ${lastSeen}`;
		const diff = Math.max(0, now - ms);
		const min = Math.floor(diff / 60_000);
		if (min < 1) return 'last used just now';
		if (min < 60) return `last used ${min} min ago`;
		const hr = Math.floor(min / 60);
		if (hr < 24) return `last used ${hr} hour${hr === 1 ? '' : 's'} ago`;
		const day = Math.floor(hr / 24);
		if (day < 30) return `last used ${day} day${day === 1 ? '' : 's'} ago`;
		if (day < 365) {
			// Consistent 365-day boundary with the year tier closes the [360,364]
			// gap that previously rendered "0 years ago".
			const mon = Math.max(1, Math.floor(day / 30));
			return `last used ${mon} month${mon === 1 ? '' : 's'} ago`;
		}
		const yr = Math.floor(day / 365);
		return `last used ${yr} year${yr === 1 ? '' : 's'} ago`;
	}
</script>

<script lang="ts">
	// WatcherCodesPanel — the /account working component (LINK-01/03/04/05,
	// 17-UI-SPEC). Composes: the Generate action → the SHOW-ONCE plaintext panel
	// (LINK-04, the most security-sensitive surface) → the active-codes list with
	// per-row confirm-before-commit Revoke (LINK-05). Mirrors EvictionForm's
	// Svelte-5-runes load→confirm→commit lifecycle, ConfirmDialog reuse, and
	// optimistic-collapse.
	//
	// SERVER-TRUTH (D-02/D-08, T-17.03-03): the owner is derived from the Discord
	// session on the server; this page never sends an owner. A 401 bubbles to
	// AuthGate → LoginScreen via authGuard (the EvictionForm route() pattern); the
	// hidden-from-anon nav is UX only, never the boundary.
	//
	// SECURITY (LINK-04 / RESEARCH Pitfall 4 / T-17.03-01,02): the freshly-minted
	// plaintext lives ONLY in `mintedPlaintext` component state for the panel's
	// lifetime — NEVER persisted to browser storage, NEVER re-fetched, NEVER
	// logged. Every
	// interpolation (the code, #N, dates, error reasons) renders via plain {}
	// (Svelte auto-escapes) — NEVER the raw-HTML directive.

	import { onMount, getContext } from 'svelte';
	import Copy from '@lucide/svelte/icons/copy';
	import Check from '@lucide/svelte/icons/check';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import StateBlock from './StateBlock.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import {
		fetchOwnCodes,
		mintOwnCode,
		revokeOwnCode,
		classifyAdminError,
		Unauthenticated,
		type OwnCode
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let codes = $state<OwnCode[]>([]);

	// Generate / show-once panel state.
	let minting = $state(false);
	// The plaintext is held ONLY here, for the panel's lifetime (LINK-04). Never
	// persisted, never re-fetched, never logged.
	let mintedPlaintext = $state<string | null>(null);
	let copied = $state(false);
	let copyTimer: ReturnType<typeof setTimeout> | null = null;
	let mintErrorMsg = $state('');

	// Revoke (confirm-before-commit) state.
	let revokeTarget = $state<OwnCode | null>(null);
	let revokeDialogOpen = $state(false);
	let revoking = $state(false);
	let revokeSuccessMsg = $state('');
	let revokeErrorMsg = $state('');

	async function load() {
		phase = 'loading';
		try {
			codes = await fetchOwnCodes();
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	async function generate() {
		if (minting) return;
		minting = true;
		mintErrorMsg = '';
		copied = false;
		try {
			const { code } = await mintOwnCode();
			// The plaintext crosses to the panel EXACTLY once (LINK-04).
			mintedPlaintext = code;
			// Refresh the list so the new code appears below as #N (additive, LINK-03).
			// The plaintext is NOT in this payload — the list is #N/created/last-seen only.
			codes = await fetchOwnCodes();
		} catch (err) {
			if (route(err)) return;
			mintErrorMsg = `Couldn't generate a code. ${reason(err)} No code was created — try again.`;
		} finally {
			minting = false;
		}
	}

	async function copyCode() {
		if (!mintedPlaintext) return;
		try {
			await navigator.clipboard.writeText(mintedPlaintext);
			copied = true;
			if (copyTimer) clearTimeout(copyTimer);
			// Revert the "Copied!" swap after ~2s.
			copyTimer = setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard API unavailable / permission denied — never block. The token
			// stays user-select:all so a manual select-copy still works (the fallback).
			copied = false;
		}
	}

	function dismissPanel() {
		// Cosmetic only — the code is already in the list as #N; dismiss does NOT
		// revoke. The plaintext drops out of state and is gone forever (LINK-04).
		mintedPlaintext = null;
		copied = false;
		if (copyTimer) clearTimeout(copyTimer);
	}

	function openRevokeConfirm(code: OwnCode) {
		if (revoking) return;
		revokeTarget = code;
		revokeDialogOpen = true;
	}

	async function doRevoke() {
		revokeDialogOpen = false;
		const target = revokeTarget;
		if (!target || revoking) return;
		revoking = true;
		revokeSuccessMsg = '';
		revokeErrorMsg = '';
		try {
			const { revoked } = await revokeOwnCode(target.id);
			if (revoked) {
				revokeSuccessMsg = `Code #${target.ordinal} revoked. That watcher will stop uploading on its next attempt.`;
				// Re-load from the server so the remaining rows' #N ordinals stay
				// authoritative (mirror generate()); a local filter would leave the
				// survivors with stale server ordinals until the next full reload.
				codes = await fetchOwnCodes();
			} else {
				// revoked:false = not the caller's / already revoked — a no-op, not a
				// hard error. Keep the row; surface a quiet note via the error line.
				revokeErrorMsg = `Couldn't revoke code #${target.ordinal}. No change was made.`;
			}
			revokeTarget = null;
		} catch (err) {
			if (route(err)) return;
			revokeErrorMsg = `Couldn't revoke code #${target.ordinal}. No change was made.`;
		} finally {
			revoking = false;
		}
	}

	/**
	 * Route a caught error by its server-truth verdict. Returns true when handled
	 * as a re-route (a bubbled 401 → AuthGate → LoginScreen, or a 403 collapse) so
	 * the caller stops; false for a generic error the caller renders inline. The
	 * /account page is RequireSession (not officer-gated), so the expected auth
	 * failure is a 401; a stray 403 still collapses via authGuard (defense-in-depth).
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

	/** The <reason> fragment for the inline mint-error copy (the server code, plainly). */
	function reason(err: unknown): string {
		const code =
			err && typeof err === 'object' && 'code' in err ? (err as { code?: string }).code : undefined;
		if (code === 'ambiguous_owner') {
			return "We couldn't match your guildie data — ask an officer to sort it out.";
		}
		return 'The server rejected the request.';
	}

	onMount(() => {
		void load();
		return () => {
			if (copyTimer) clearTimeout(copyTimer);
		};
	});
</script>

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else}
	<div class="codes-panel">
		<!-- Generate action — the single accent primary at rest (17-UI-SPEC). The
		     owner is session-derived (D-02): no inputs, no device-name field (D-06). -->
		<div class="generate-block">
			<button type="button" class="primary" disabled={minting} onclick={generate}>
				{minting ? 'Generating…' : 'Generate a new code'}
			</button>
			{#if mintErrorMsg}
				<p class="result error" aria-live="polite">{mintErrorMsg}</p>
			{/if}
		</div>

		<!-- SHOW-ONCE plaintext panel (LINK-04) — appears only after a successful
		     mint, never re-rendered after reload (hash-only at rest). The plaintext
		     renders via {} auto-escape — never the Svelte raw-HTML directive. -->
		{#if mintedPlaintext}
			<section class="show-once" aria-live="polite" aria-label="Your new watcher code">
				<h2 class="show-once-heading">Copy your new watcher code now</h2>
				<div class="token-row">
					<code class="token">{mintedPlaintext}</code>
					<button type="button" class="primary copy-btn" onclick={copyCode}>
						{#if copied}
							<Check size={16} aria-hidden="true" />
							Copied!
						{:else}
							<Copy size={16} aria-hidden="true" />
							Copy
						{/if}
					</button>
				</div>
				<p class="paste-instructions">
					Open the SquireBot watcher, paste this where it asks for your guild code, and you're
					linked. The same code works on this PC forever — no need to re-link.
				</p>
				<div class="consequence" role="note">
					<TriangleAlert size={18} aria-hidden="true" class="consequence-icon" />
					<span
						>This is the only time you'll see this code. Copy it now — you can't view it again.
						(Lost it? Just generate a new one.)</span
					>
				</div>
				<div class="show-once-actions">
					<button type="button" class="ghost" onclick={dismissPanel}>Done</button>
				</div>
			</section>
		{/if}

		<!-- Codes list (LINK-05) — the caller's own active codes, server-scoped. -->
		<section class="codes-section" aria-label="Your active codes">
			<div class="divider"></div>
			<h2 class="codes-heading">Your active codes ({codes.length})</h2>
			{#if revokeSuccessMsg}
				<p class="result success" aria-live="polite">{revokeSuccessMsg}</p>
			{/if}
			{#if revokeErrorMsg}
				<p class="result error" aria-live="polite">{revokeErrorMsg}</p>
			{/if}
			{#if codes.length === 0}
				<StateBlock kind="no-codes" />
			{:else}
				<ul class="codes-list">
					{#each codes as c (c.id)}
						<li class="code-row">
							<span class="code-info">
								<span class="code-ordinal">#{c.ordinal}</span>
								<span class="code-meta">Created {formatCreated(c.created_at)}</span>
								<span class="code-meta">{formatLastSeen(c.last_seen)}</span>
							</span>
							<button
								type="button"
								class="revoke-btn"
								disabled={revoking}
								onclick={() => openRevokeConfirm(c)}
							>
								{revoking && revokeTarget?.id === c.id ? 'Revoking…' : 'Revoke'}
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	</div>

	<ConfirmDialog
		open={revokeDialogOpen}
		heading="Revoke this code?"
		body={`Revoking code #${revokeTarget?.ordinal ?? ''} stops that PC from uploading. Your other codes keep working, and you can always generate a fresh one.`}
		confirmLabel={`Revoke code #${revokeTarget?.ordinal ?? ''}`}
		onConfirm={doRevoke}
		onCancel={() => {
			revokeDialogOpen = false;
			revokeTarget = null;
		}}
	/>
{/if}

<style>
	.codes-panel {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg (17-UI-SPEC) */
	}
	.generate-block {
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-items: flex-start;
	}
	/* --- Primary button (lifted from EvictionForm's .primary) --- */
	.primary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px; /* touch target (17-UI-SPEC) */
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
	/* --- Show-once panel (LINK-04) — the one genuinely-new surface --- */
	.show-once {
		display: flex;
		flex-direction: column;
		gap: 16px; /* md */
		padding: 16px;
		border: 1px solid var(--border, var(--accent));
		border-left: 2px solid var(--accent);
		border-radius: 6px;
		/* The low-alpha accent wash that marks it as the prominent credential reveal. */
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.show-once-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (17-UI-SPEC) */
		line-height: 1.2;
		color: var(--text);
	}
	.token-row {
		display: flex;
		align-items: center;
		gap: 16px;
		flex-wrap: wrap;
	}
	/* The plaintext token: --text (data, not a link), tabular-nums + tracking so the
	   base64-ish string is unambiguous; user-select:all is the manual-copy fallback
	   when the Clipboard API is denied; word-break so a long token wraps. */
	.token {
		flex: 1 1 auto;
		min-width: 0;
		font-family: var(--font-body);
		font-size: 16px; /* Body (17-UI-SPEC — no monospace face in the system) */
		font-variant-numeric: tabular-nums;
		letter-spacing: 0.04em;
		color: var(--text);
		user-select: all;
		word-break: break-all;
		overflow-wrap: anywhere;
	}
	.copy-btn {
		flex: none;
	}
	.paste-instructions {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.9;
	}
	/* The irreversibility warning — destructive callout (mirror EvictionForm's). */
	.consequence {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 12px;
		font-family: var(--font-body);
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
	.show-once-actions {
		display: flex;
		justify-content: flex-end;
	}
	/* Quiet ghost "Done" dismiss — neutral, not destructive (cosmetic-only). */
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
	}
	.ghost:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	/* --- Codes list (LINK-05) --- */
	.codes-section {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.divider {
		border-top: 1px solid var(--border, var(--accent));
		margin-top: 8px;
	}
	.codes-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (17-UI-SPEC) */
		line-height: 1.2;
		color: var(--text);
	}
	.codes-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.code-row {
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
	.code-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		font-family: var(--font-body);
	}
	.code-ordinal {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.04em;
		color: var(--text);
	}
	.code-meta {
		font-size: 13px;
		opacity: 0.75;
	}
	/* Resting Revoke = a quiet destructive-text ghost (the full destructive FILL is
	   reserved for the ConfirmDialog confirm — 17-UI-SPEC § Revoke Interaction). */
	.revoke-btn {
		flex: none;
		min-height: 44px; /* touch target (17-UI-SPEC) */
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
	/* --- Shared result lines (mirror EvictionForm's .result) --- */
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
	@media (max-width: 640px) {
		.token-row {
			flex-direction: column;
			align-items: stretch;
		}
		.copy-btn {
			align-self: flex-start;
		}
	}
</style>
