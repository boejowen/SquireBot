<script lang="ts">
	// FormField — the shared label + control + inline-error rhythm for the three
	// 15-05 forms (15-UI-SPEC § Form Contracts). It standardizes the field
	// presentation so BankCoinForm / EvictionForm / AdminMgmtForm look continuous
	// with the shipped SearchBox/ThemePicker controls: a Label (13px/600 display,
	// uppercase) above the control slot, and an inline error below in
	// --status-missing, announced via aria-live="polite" (15-UI-SPEC §
	// Accessibility — matches v1's #msg aria-live).
	//
	// The control is passed as the `children` snippet so the caller owns the
	// native <select>/<input> (and wires `id` to its own `for`). `error` renders
	// ONLY via plain {} (Svelte auto-escapes) — never the raw-HTML directive — so
	// an interpolated <reason> is inert text (T-15-28).

	import type { Snippet } from 'svelte';

	let {
		label,
		error,
		id,
		children
	}: {
		label: string;
		/** Inline error/help text below the control (omitted when falsy). */
		error?: string;
		/** The control's id, mirrored onto the <label for>. */
		id?: string;
		children: Snippet;
	} = $props();
</script>

<div class="form-field">
	<label class="ff-label" for={id}>{label}</label>
	{@render children()}
	{#if error}
		<p class="ff-error" aria-live="polite">{error}</p>
	{/if}
</div>

<style>
	.form-field {
		display: flex;
		flex-direction: column;
		gap: 8px; /* sm — label↔control (UI-SPEC Spacing) */
	}
	.ff-label {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label (UI-SPEC Typography) */
		letter-spacing: 0.08em;
		text-transform: uppercase;
		line-height: 1.3;
		color: var(--text);
	}
	.ff-error {
		font-family: var(--font-body);
		font-size: 16px; /* Body (UI-SPEC) */
		line-height: 1.4;
		color: var(--status-missing);
	}
</style>
