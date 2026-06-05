<script lang="ts">
	// WantMuteCell — the per-row mute bell on the /wantlist grid (D-09, 20-UI-SPEC §
	// Per-Want Mute). A bell / bell-off toggle: the GLYPH carries the mute state
	// (color is never the only signal). Clicking calls back with the row; the panel
	// does the muteWant() server write + server-truth reload (NEVER optimistic — the
	// WantRemoveCell idiom). A custom want (item_id === null) renders a DISABLED
	// bell-off (a custom want never matches an alert), keeping the column aligned.
	//
	// The mute state coerces STRICTLY to a boolean (`row.muted === true`) so a
	// truthy-but-non-bool value never picks the wrong glyph (the P15 coercion class).
	import Bell from '@lucide/svelte/icons/bell';
	import BellOff from '@lucide/svelte/icons/bell-off';
	import type { WantlistRow } from '$lib/api';

	let {
		row,
		onMute,
		busy = false
	}: { row: WantlistRow; onMute: (row: WantlistRow) => void; busy?: boolean } = $props();

	// A custom want (no catalog id) is un-matchable — the bell is disabled.
	let isCustom = $derived(row.item_id === null);
	// STRICT coercion: render the glyph from `row.muted === true`, never a raw value.
	let muted = $derived(row.muted === true);
	let label = $derived(
		muted
			? `Alerts muted for ${row.item_name} — click to unmute`
			: `Alerts on for ${row.item_name} — click to mute`
	);
</script>

{#if isCustom}
	<!-- Custom wants never trigger alerts → a disabled muted bell, column-aligned. -->
	<button type="button" class="mute-btn custom" disabled title="Custom wants never trigger alerts.">
		<BellOff size={16} aria-hidden="true" />
		<span class="dash" aria-hidden="true">—</span>
		<span class="sr-only">Custom wants never trigger alerts.</span>
	</button>
{:else}
	<button
		type="button"
		class="mute-btn"
		class:muted
		disabled={busy}
		aria-label={label}
		onclick={() => onMute(row)}
	>
		{#if muted}
			<BellOff size={16} aria-hidden="true" />
		{:else}
			<Bell size={16} aria-hidden="true" />
		{/if}
	</button>
{/if}

<style>
	.mute-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 4px;
		min-width: 44px;
		min-height: 44px; /* touch target (UI-SPEC a11y exception) */
		padding: 8px;
		color: var(--text);
		background: none;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	/* Muted = the struck bell-off in the neutral status accent (glyph + color). */
	.mute-btn.muted {
		color: var(--status-other);
	}
	.mute-btn:disabled {
		cursor: default;
	}
	.mute-btn.custom {
		color: var(--text);
		opacity: 0.5;
	}
	.mute-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.dash {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
</style>
