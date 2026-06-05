<script lang="ts" module>
	// Toggle — the ONE new visual primitive this phase (20-UI-SPEC § Toggle
	// Vocabulary). A `role="switch"` button (NOT a checkbox) with a VISIBLE ON/OFF
	// word beside the track, so state is never color-only (a11y / color-blindness —
	// the inherited "color is never the only signal" obligation). Reused by the
	// /notifications prefs (×4) and by Plan 05's officer kill-switches (×3).
	//
	// The on/off WORD derivation lives in this module block as a pure function so
	// it's unit-testable under the node vitest project (no jsdom; the same split
	// WatcherCodesPanel/ConfirmDialog use). It STRICTLY coerces to a boolean: a
	// truthy-but-non-bool input (the P15 number-input-coercion class of crasher —
	// 165 green tests, 2 crashing browser blockers) must never leak `true`/
	// `undefined`/`1` into the rendered word; the word derives from `on === true`.

	/** 'ON' iff `on` is strictly the boolean true, else 'OFF' — never a raw value. */
	export function onLabel(on: boolean): 'ON' | 'OFF' {
		return on === true ? 'ON' : 'OFF';
	}
</script>

<script lang="ts">
	let {
		on,
		label,
		onToggle,
		disabled = false
	}: {
		/** The current switch state. Coerced strictly to a boolean for aria-checked + the word. */
		on: boolean;
		/** The control's accessible name (the pref/monitor name) — set as aria-label. */
		label: string;
		/** Fired on click / Space / Enter (the panel does the server write + re-read). */
		onToggle: () => void;
		disabled?: boolean;
	} = $props();

	// STRICT coercion (the P15 crasher): render the switch from `on === true`, never
	// from a truthy-but-non-bool value. aria-checked + the visible word both read
	// this, so the switch can only ever show a clean ON or OFF.
	let checked = $derived(on === true);
	let word = $derived(onLabel(checked));
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	aria-label={label}
	{disabled}
	class="toggle"
	class:on={checked}
	onclick={onToggle}
>
	<span class="track" aria-hidden="true"><span class="thumb"></span></span>
	<span class="word">{word}</span>
</button>

<style>
	.toggle {
		display: inline-flex;
		align-items: center;
		gap: 8px; /* sm — label↔switch gap (UI-SPEC) */
		min-height: 44px; /* touch target (UI-SPEC a11y exception) */
		padding: 8px;
		background: none;
		border: none;
		cursor: pointer;
		font-family: var(--font-display);
	}
	.toggle:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.toggle:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 4px;
	}
	/* OFF — muted --border track + --text@0.6 thumb (UI-SPEC § Toggle color). */
	.track {
		position: relative;
		display: inline-block;
		width: 40px;
		height: 22px;
		border-radius: 11px;
		background: var(--border, rgba(74, 101, 133, 0.5));
		transition: background 120ms ease;
	}
	.thumb {
		position: absolute;
		top: 2px;
		left: 2px;
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: color-mix(in srgb, var(--text) 60%, transparent);
		transition: transform 120ms ease;
	}
	/* ON — --accent track + --bg thumb, thumb slid right (UI-SPEC § Toggle color). */
	.toggle.on .track {
		background: var(--accent);
	}
	.toggle.on .thumb {
		background: var(--bg);
		transform: translateX(18px);
	}
	/* The ON/OFF word carries the state alongside color (never color-only). */
	.word {
		font-size: 13px; /* Label (UI-SPEC Typography) */
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.08em;
		color: var(--text);
		min-width: 2.5ch; /* keeps ON/OFF from shifting layout */
		text-align: left;
	}
	/* Reduced motion — the thumb slide goes instant (the global app.css obligation). */
	@media (prefers-reduced-motion: reduce) {
		.track,
		.thumb {
			transition: none;
		}
	}
</style>
