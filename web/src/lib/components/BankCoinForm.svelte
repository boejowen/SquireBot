<script lang="ts">
	// BankCoinForm — record plat/gold/silver/copper for an is_bank_toon character
	// (ADMIN-05, 15-UI-SPEC § BankCoinForm). ANY authenticated member may use it
	// (D-12) — it is NOT officer-gated (RequireSession on the backend; this route
	// is reachable without isOfficer). Non-destructive ⇒ NO ConfirmDialog (D-12).
	//
	// Flow: fetchBankToons() on mount → pick a bank character (a native <select>)
	// → the four coin inputs pre-fill from that toon's current coin (null →
	// blank, never a fabricated 0) → range-validated (D-11, via the pure coin.ts
	// helpers) → Save coin (disabled until valid AND ≥1 value changed) → success
	// keeps the select so a second entry is easy. The recorded coin then surfaces
	// in the bank view (+page.svelte), replacing P14's "not yet recorded".
	//
	// SECURITY: the character name is user-controlled (interpolated into the
	// success copy) — it renders ONLY via plain {} (Svelte auto-escapes), never
	// the raw-HTML directive (T-15-28). Client validation is UX defense-in-depth
	// only; the SERVER re-validates (400 invalid_input / not_bank_toon, T-15-29).

	import { onMount, getContext } from 'svelte';
	import Coins from '@lucide/svelte/icons/coins';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import FormField from './FormField.svelte';
	import StateBlock from './StateBlock.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { fetchBankToons, saveCoin, Unauthenticated, type BankToon } from '$lib/api';
	import {
		COIN_FIELDS,
		validateCoin,
		coinIsValid,
		coinPayload,
		inputsFromToon,
		coinChanged,
		type CoinInputs,
		type CoinField
	} from '$lib/coin';

	// A 401 mid-session hands off to the whole-site gate (→ LoginScreen). A
	// bank-coin call should never 403 (login-only), but if it does the typed
	// Forbidden also routes through here defensively.
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let toons = $state<BankToon[]>([]);

	// The chosen character_id as a string (a <select> value is a string; '' = none).
	let selectedId = $state('');
	// The four raw input strings.
	let inputs = $state<CoinInputs>({ plat: '', gold: '', silver: '', copper: '' });

	let saving = $state(false);
	let successMsg = $state('');
	let errorMsg = $state('');

	const FIELD_LABEL: Record<CoinField, string> = {
		plat: 'Platinum',
		gold: 'Gold',
		silver: 'Silver',
		copper: 'Copper'
	};

	let selectedToon = $derived(toons.find((t) => String(t.character_id) === selectedId) ?? null);
	let fieldErrors = $derived(validateCoin(inputs));
	// Save is enabled only when a toon is chosen, all fields validate, AND at
	// least one differs from the loaded value (UX gate; the server is authoritative).
	let canSave = $derived(
		!!selectedToon && coinIsValid(inputs) && coinChanged(inputs, selectedToon) && !saving
	);

	async function load() {
		phase = 'loading';
		errorMsg = '';
		try {
			toons = await fetchBankToons();
			phase = 'ready';
		} catch (err) {
			if (authGuard && err instanceof Unauthenticated) {
				authGuard(err);
				return;
			}
			phase = 'error';
		}
	}

	function onSelect() {
		// Pre-fill the inputs from the chosen toon's current coin (null → blank);
		// clear any prior result.
		successMsg = '';
		errorMsg = '';
		inputs = selectedToon
			? inputsFromToon(selectedToon)
			: { plat: '', gold: '', silver: '', copper: '' };
	}

	async function onSave(e: SubmitEvent) {
		e.preventDefault();
		if (!selectedToon || !canSave) return;
		saving = true;
		successMsg = '';
		errorMsg = '';
		try {
			const res = await saveCoin({ character_id: selectedToon.character_id, ...coinPayload(inputs) });
			// Reflect the persisted coin back into the loaded toon so the Save gate
			// re-disables (no diff now) and the bank view (which re-fetches) is fresh.
			const payload = coinPayload(inputs);
			toons = toons.map((t) =>
				t.character_id === selectedToon!.character_id ? { ...t, ...payload } : t
			);
			successMsg = `Coin saved for ${res.character}.`;
		} catch (err) {
			if (authGuard && err instanceof Unauthenticated) {
				authGuard(err);
				return;
			}
			// 15-UI-SPEC error copy: "<reason>" is the server code surfaced plainly.
			const reason =
				err && typeof err === 'object' && 'code' in err && (err as { code?: string }).code
					? reasonText((err as { code?: string }).code as string)
					: '';
			errorMsg = `Couldn't save the coin. ${reason} No changes were saved.`.replace('  ', ' ');
		} finally {
			saving = false;
		}
	}

	// Map the server's {error} code to the human <reason> fragment in the copy.
	function reasonText(code: string): string {
		if (code === 'not_bank_toon') return 'That character is no longer a bank toon.';
		if (code === 'invalid_input') return 'Those values are out of range.';
		return '';
	}

	onMount(() => {
		void load();
	});
</script>

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else if toons.length === 0}
	<StateBlock kind="no-bank-toons" />
{:else}
	<form class="coin-form" onsubmit={onSave}>
		<FormField label="Bank character" id="coin-char">
			<select id="coin-char" class="field" bind:value={selectedId} onchange={onSelect}>
				<option value="">Choose a bank character…</option>
				{#each toons as t (t.character_id)}
					<option value={String(t.character_id)}>{t.name}</option>
				{/each}
			</select>
		</FormField>

		{#if selectedToon}
			<div class="coin-row">
				{#each COIN_FIELDS as f (f)}
					<FormField label={FIELD_LABEL[f]} id={`coin-${f}`} error={fieldErrors[f]}>
						<!-- CR-01: a text input + numeric keypad, NOT type="number". Svelte 5's
						     bind:value on a number-like input coerces the written-back value
						     through to_number() (→ number|null), but inputs[f] is typed/used as
						     a string (the coin.ts helpers call .trim()). type="text" with
						     inputmode="numeric" keeps the on-screen numeric keypad WITHOUT the
						     coercion, so the binding stays a string and the strict /^\d+$/
						     validation holds. -->
						<input
							id={`coin-${f}`}
							class="field coin-input"
							class:invalid={!!fieldErrors[f]}
							type="text"
							inputmode="numeric"
							pattern="[0-9]*"
							bind:value={inputs[f]}
							aria-invalid={fieldErrors[f] ? 'true' : undefined}
						/>
					</FormField>
				{/each}
			</div>

			<div class="actions">
				{#if successMsg}
					<p class="result success" aria-live="polite">{successMsg}</p>
				{/if}
				{#if errorMsg}
					<p class="result error" aria-live="polite">{errorMsg}</p>
				{/if}
				<button type="submit" class="primary" disabled={!canSave}>
					{#if saving}
						<LoaderCircle size={16} aria-hidden="true" class="spin" />
						<span>Saving…</span>
					{:else}
						<Coins size={16} aria-hidden="true" />
						<span>Save coin</span>
					{/if}
				</button>
			</div>
		{/if}
	</form>
{/if}

<style>
	.coin-form {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg between sections (UI-SPEC) */
		max-width: 640px;
	}
	/* Native control styled like ThemePicker's select (UI-SPEC § Form Contracts). */
	.field {
		min-height: 44px; /* touch target */
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
	}
	.field:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.coin-row {
		display: flex;
		flex-wrap: wrap;
		gap: 16px; /* md between fields (UI-SPEC) */
	}
	.coin-row :global(.form-field) {
		flex: 1 1 120px;
	}
	.coin-input {
		width: 100%;
		font-variant-numeric: tabular-nums; /* plat/gold/silver/copper align (UI-SPEC) */
		cursor: text;
	}
	.coin-input.invalid {
		border-color: var(--status-missing);
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
	/* Primary button = accent fill / --bg text (mirrors StateBlock .retry). */
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
	}
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	:global(.spin) {
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		:global(.spin) {
			animation: none;
		}
	}
</style>
