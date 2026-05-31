<script lang="ts" module>
	// Pure, DOM-free decision helpers for the dialog's dismiss + focus-trap
	// behavior, split out so they're unit-testable in the node test project
	// (the repo runs vitest under node with NO jsdom and NO @testing-library;
	// see api.test.ts/auth.test.ts). The .svelte instance below is a thin
	// renderer that wires these to real DOM events.

	/** A key event reduces to one of: cancel the dialog, trap focus, or ignore. */
	export type DialogKeyAction = 'cancel' | 'trap' | 'ignore';

	/**
	 * Decide what a keydown means while the dialog is open (W-5):
	 *   - Escape            → 'cancel' (dismiss with no side effect)
	 *   - Tab / Shift+Tab   → 'trap'   (keep focus inside the dialog)
	 *   - anything else     → 'ignore'
	 * When closed, every key is ignored.
	 */
	export function dialogKeyAction(key: string, open: boolean): DialogKeyAction {
		if (!open) return 'ignore';
		if (key === 'Escape') return 'cancel';
		if (key === 'Tab') return 'trap';
		return 'ignore';
	}

	/**
	 * Given the focusable controls and the currently-focused element, return the
	 * element Tab/Shift+Tab should wrap to (or null when no wrap is needed). This
	 * is the pure core of the focus trap: Tab off the last → first; Shift+Tab off
	 * the first → last.
	 */
	export function trapTarget(
		items: HTMLElement[],
		active: HTMLElement | null,
		shift: boolean
	): HTMLElement | null {
		if (items.length === 0) return null;
		const first = items[0];
		const last = items[items.length - 1];
		if (shift && active === first) return last;
		if (!shift && active === last) return first;
		return null;
	}
</script>

<script lang="ts">
	// ConfirmDialog — the shared, themed, accessible destructive-confirm modal
	// (15-UI-SPEC § ConfirmDialog + § Accessibility). It replaces v1's
	// window.confirm and is the SINGLE destructive-confirmation pattern for the
	// phase; the three Wave-5 forms (eviction, admin-remove) reuse it.
	//
	// A11y contract (W-5), implemented exactly:
	//   - role="dialog" + aria-modal="true" + aria-labelledby the heading id.
	//   - On open, focus moves to the CANCEL button — the destructive confirm is
	//     NEVER the default-focused element.
	//   - Esc, a backdrop click, and Cancel all dismiss with NO side effect; only
	//     the explicit destructive button commits.
	//   - Focus is trapped within the dialog (Tab/Shift+Tab cycle) and RESTORED to
	//     the trigger element on close.
	//   - Reduced-motion → instant (the global app.css rule + no entrance anim).
	//   - Color is never the only signal: a triangle-alert icon + the explicit
	//     confirmLabel carry the destructive meaning alongside --destructive.
	//
	// All text renders via plain {} (Svelte auto-escapes) — never the raw-HTML
	// directive — so an interpolated guildie/username in heading/body/confirmLabel
	// is inert (T-15-22).

	import { tick } from 'svelte';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';

	let {
		open,
		heading,
		body,
		confirmLabel,
		onConfirm,
		onCancel
	}: {
		open: boolean;
		heading: string;
		body: string;
		confirmLabel: string;
		onConfirm: () => void;
		onCancel: () => void;
	} = $props();

	// A stable id so aria-labelledby points at the heading (W-5).
	const headingId = `confirm-dialog-heading-${Math.random().toString(36).slice(2)}`;

	let dialogEl: HTMLDivElement | undefined = $state();
	let cancelBtn: HTMLButtonElement | undefined = $state();
	// The element focused before the dialog opened — focus returns here on close.
	let lastFocused: HTMLElement | null = null;

	// Open/close lifecycle: capture the trigger, move focus to Cancel on open;
	// restore focus to the trigger on close. The $effect re-runs whenever `open`
	// flips.
	$effect(() => {
		if (open) {
			lastFocused = (document.activeElement as HTMLElement) ?? null;
			// Wait for the dialog DOM to mount, then focus Cancel (never confirm).
			void tick().then(() => cancelBtn?.focus());
		} else if (lastFocused) {
			// Restore focus to whatever opened the dialog.
			lastFocused.focus();
			lastFocused = null;
		}
	});

	/** All tabbable controls inside the dialog, in DOM order (for the focus trap). */
	function tabbables(): HTMLElement[] {
		if (!dialogEl) return [];
		return Array.from(
			dialogEl.querySelectorAll<HTMLElement>(
				'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
			)
		).filter((el) => !el.hasAttribute('disabled'));
	}

	function onKeydown(e: KeyboardEvent) {
		const action = dialogKeyAction(e.key, open);
		if (action === 'cancel') {
			e.preventDefault();
			onCancel();
			return;
		}
		if (action === 'trap') {
			const target = trapTarget(
				tabbables(),
				document.activeElement as HTMLElement | null,
				e.shiftKey
			);
			if (target) {
				e.preventDefault();
				target.focus();
			}
		}
	}

	// A backdrop click (on the overlay itself, not its dialog child) cancels.
	function onBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) onCancel();
	}
</script>

{#if open}
	<!-- The overlay captures the backdrop click + the trapped keydown. The inner
	     dialog is the labelled modal surface. -->
	<div
		class="cd-backdrop"
		role="presentation"
		onclick={onBackdropClick}
		onkeydown={onKeydown}
	>
		<div
			class="cd-dialog"
			role="dialog"
			aria-modal="true"
			aria-labelledby={headingId}
			bind:this={dialogEl}
		>
			<h2 class="cd-heading" id={headingId}>
				<TriangleAlert size={22} aria-hidden="true" class="cd-icon" />
				<span>{heading}</span>
			</h2>
			<p class="cd-body">{body}</p>
			<div class="cd-actions">
				<button type="button" class="cd-cancel" bind:this={cancelBtn} onclick={onCancel}>
					Cancel
				</button>
				<button type="button" class="cd-confirm" onclick={onConfirm}>
					{confirmLabel}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.cd-backdrop {
		position: fixed;
		inset: 0;
		z-index: 100;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
		/* Dimmed scrim — a color-mix of the theme --bg so it reads on every theme. */
		background: color-mix(in srgb, var(--bg) 70%, transparent);
	}
	.cd-dialog {
		width: 100%;
		max-width: 440px;
		padding: 24px; /* lg (UI-SPEC) */
		background: var(--panel);
		color: var(--text);
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
		font-family: var(--font-body);
		animation: cd-in 0.12s ease-out;
	}
	.cd-heading {
		display: flex;
		align-items: center;
		gap: 8px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC) */
		line-height: 1.2;
		margin-bottom: 16px;
	}
	/* The destructive triangle glyph (icon carries meaning alongside color). */
	:global(.cd-icon) {
		color: var(--destructive);
		flex: none;
	}
	.cd-body {
		font-size: 16px; /* Body (UI-SPEC) */
		line-height: 1.5;
		opacity: 0.9;
	}
	.cd-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 24px;
	}
	.cd-cancel,
	.cd-confirm {
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px 20px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		border-radius: 4px;
		cursor: pointer;
	}
	/* Cancel = neutral ghost button (--text). */
	.cd-cancel {
		color: var(--text);
		background: none;
		border: 1px solid var(--border, var(--accent));
	}
	/* Confirm = the destructive action: --destructive fill, --bg text. */
	.cd-confirm {
		color: var(--bg);
		background: var(--destructive);
		border: 1px solid var(--destructive);
	}
	/* Heavy contrast caveat (UI-SPEC § Color): Heavy's --status-missing/destructive
	   is dark (#6b1a1a), so --bg (also dark) text would be dark-on-dark. On Heavy
	   the destructive button uses accent-colored text on the dark destructive tint. */
	:global([data-theme='heavy']) .cd-confirm {
		color: var(--accent);
	}
	.cd-cancel:focus-visible,
	.cd-confirm:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	@keyframes cd-in {
		from {
			opacity: 0;
			transform: translateY(4px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.cd-dialog {
			animation: none;
		}
	}
</style>
