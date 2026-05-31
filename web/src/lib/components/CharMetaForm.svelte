<script lang="ts">
	// CharMetaForm — set class/level/race/is_bank_toon on an EXISTING character
	// (CUTOVER-02, P16; the fresh-start replacement for the Sheet backfill, D-01/
	// D-02). ANY authenticated member may use it (D-03) — it is NOT officer-gated
	// (RequireSession on the backend; this form never consults officer status).
	// Non-destructive ⇒ NO ConfirmDialog (D-12 precedent — no confirm even on
	// un-setting is_bank_toon). The form edits characters that already exist (created
	// by their first watcher upload); it never creates a character (D-03).
	//
	// Flow: fetchCharsForMeta() on mount → pick a character (a native <select>) →
	// the fields pre-fill from that char's current metadata (level null/0 → blank,
	// never a fabricated 0) → validated (class+race required, level blank-or-1..60,
	// via the pure charmeta.ts helpers) → Save (disabled until valid AND ≥1 value
	// changed) → success keeps the select so a second edit is easy. Once class (+
	// level) are set, gear_check/spell_check stop showing blank for that char; an
	// is_bank_toon-flagged char appears in the bank view + the BankCoinForm picker.
	//
	// SECURITY: the character name is user-controlled (interpolated into the success
	// copy + the <option> labels) — it renders ONLY via plain {} (Svelte
	// auto-escapes), never the raw-HTML directive (T-16-03). Client validation is UX
	// defense-in-depth only; the SERVER re-validates (400 invalid_input, T-16-02).

	import { onMount, getContext } from 'svelte';
	import Save from '@lucide/svelte/icons/save';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import FormField from './FormField.svelte';
	import StateBlock from './StateBlock.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { fetchCharsForMeta, saveCharMeta, Unauthenticated, type CharMetaItem } from '$lib/api';
	import {
		CLASSES,
		RACES,
		validateClass,
		validateRace,
		validateLevel,
		charMetaIsValid,
		charMetaPayload,
		inputsFromChar,
		charMetaChanged,
		type CharMetaInputs
	} from '$lib/charmeta';

	// A 401 mid-session hands off to the whole-site gate (→ LoginScreen). A char-meta
	// call should never 403 (login-only), but if it does the typed Forbidden also
	// routes through here defensively.
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let chars = $state<CharMetaItem[]>([]);

	// The chosen character_id as a string (a <select> value is a string; '' = none).
	let selectedId = $state('');
	// The form inputs (level is a raw string; isBankToon a checkbox bool).
	let inputs = $state<CharMetaInputs>({ class: '', race: '', level: '', isBankToon: false });

	let saving = $state(false);
	let successMsg = $state('');
	let errorMsg = $state('');

	let selectedChar = $derived(chars.find((c) => String(c.character_id) === selectedId) ?? null);
	let classError = $derived(validateClass(inputs.class));
	let raceError = $derived(validateRace(inputs.race));
	let levelError = $derived(validateLevel(inputs.level));
	// Save is enabled only when a char is chosen, the fields validate (class+race
	// non-blank, level blank-or-1..60), AND at least one differs from the loaded
	// value (UX gate; the server is authoritative).
	let canSave = $derived(
		!!selectedChar && charMetaIsValid(inputs) && charMetaChanged(inputs, selectedChar) && !saving
	);

	async function load() {
		phase = 'loading';
		errorMsg = '';
		try {
			chars = await fetchCharsForMeta();
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
		// Pre-fill the inputs from the chosen char's current metadata (null/0 level →
		// blank); clear any prior result.
		successMsg = '';
		errorMsg = '';
		inputs = selectedChar
			? inputsFromChar(selectedChar)
			: { class: '', race: '', level: '', isBankToon: false };
	}

	async function onSave(e: SubmitEvent) {
		e.preventDefault();
		if (!selectedChar || !canSave) return;
		saving = true;
		successMsg = '';
		errorMsg = '';
		try {
			const payload = charMetaPayload(inputs);
			const res = await saveCharMeta({ character_id: selectedChar.character_id, ...payload });
			// Reflect the persisted metadata back into the loaded char so the Save gate
			// re-disables (no diff now) and the bank/gear/spell views (which re-fetch)
			// are fresh.
			chars = chars.map((c) =>
				c.character_id === selectedChar!.character_id
					? { ...c, class: payload.class, level: payload.level, race: payload.race, is_bank_toon: payload.is_bank_toon }
					: c
			);
			successMsg = `Saved details for ${res.character}.`;
		} catch (err) {
			if (authGuard && err instanceof Unauthenticated) {
				authGuard(err);
				return;
			}
			const reason =
				err && typeof err === 'object' && 'code' in err && (err as { code?: string }).code
					? reasonText((err as { code?: string }).code as string)
					: '';
			errorMsg = `Couldn't save the details. ${reason} No changes were saved.`.replace('  ', ' ');
		} finally {
			saving = false;
		}
	}

	// Map the server's {error} code to the human <reason> fragment in the copy.
	function reasonText(code: string): string {
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
{:else if chars.length === 0}
	<StateBlock kind="empty" />
{:else}
	<form class="meta-form" onsubmit={onSave}>
		<FormField label="Character" id="meta-char">
			<select id="meta-char" class="field" bind:value={selectedId} onchange={onSelect}>
				<option value="">Choose a character…</option>
				{#each chars as c (c.character_id)}
					<option value={String(c.character_id)}>{c.name}</option>
				{/each}
			</select>
		</FormField>

		{#if selectedChar}
			<FormField label="Class" id="meta-class" error={classError}>
				<select id="meta-class" class="field" class:invalid={!!classError} bind:value={inputs.class}>
					<option value="">Choose a class…</option>
					{#each CLASSES as cls (cls)}
						<option value={cls}>{cls}</option>
					{/each}
				</select>
			</FormField>

			<FormField label="Race" id="meta-race" error={raceError}>
				<select id="meta-race" class="field" class:invalid={!!raceError} bind:value={inputs.race}>
					<option value="">Choose a race…</option>
					{#each RACES as r (r)}
						<option value={r}>{r}</option>
					{/each}
				</select>
			</FormField>

			<FormField label="Level (optional)" id="meta-level" error={levelError}>
				<!-- CR-01: a text input + numeric keypad, NEVER a number-typed input.
				     Svelte 5's bind:value on a number-like input coerces the written-back
				     value through to_number() (→ number|null), but inputs.level is
				     typed/used as a string (the charmeta.ts helpers call .trim()).
				     type="text" with inputmode="numeric" keeps the on-screen numeric
				     keypad WITHOUT the coercion, so the binding stays a string and the
				     strict /^\d+$/ validation holds. (The Style-B source test asserts
				     inputmode="numeric" is present AND no number-typed attribute exists.) -->
				<input
					id="meta-level"
					class="field meta-level"
					class:invalid={!!levelError}
					type="text"
					inputmode="numeric"
					pattern="[0-9]*"
					bind:value={inputs.level}
					aria-invalid={levelError ? 'true' : undefined}
				/>
			</FormField>

			<FormField label="Bank character" id="meta-bank">
				<label class="checkbox-row">
					<input id="meta-bank" type="checkbox" bind:checked={inputs.isBankToon} />
					<span>This character is the guild bank (its coin + inventory show in the bank view).</span>
				</label>
			</FormField>

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
						<Save size={16} aria-hidden="true" />
						<span>Save details</span>
					{/if}
				</button>
			</div>
		{/if}
	</form>
{/if}

<style>
	.meta-form {
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
	.field.invalid {
		border-color: var(--status-missing);
	}
	.meta-level {
		width: 120px;
		font-variant-numeric: tabular-nums;
		cursor: text;
	}
	.checkbox-row {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		cursor: pointer;
	}
	.checkbox-row input {
		margin-top: 4px;
		width: 18px;
		height: 18px;
		accent-color: var(--accent);
		cursor: pointer;
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
