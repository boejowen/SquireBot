<script lang="ts">
	// WantRemoveCell — the per-row destructive Remove control (19-UI-SPEC § Remove
	// Interaction; the /account Revoke twin). A quiet --destructive ghost button
	// (trash-2 + "Remove") — the glyph carries meaning alongside color. The actual
	// confirm-before-commit (ConfirmDialog) + server-truth reload live in the
	// panel; this cell just calls back with the row to open the dialog.
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import type { WantlistRow } from '$lib/api';

	let {
		row,
		onRemove,
		busy = false
	}: { row: WantlistRow; onRemove: (row: WantlistRow) => void; busy?: boolean } = $props();
</script>

<button type="button" class="remove-btn" disabled={busy} onclick={() => onRemove(row)}>
	<Trash2 size={16} aria-hidden="true" />
	Remove
</button>

<style>
	.remove-btn {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px;
		padding: 8px 16px;
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--destructive);
		background: none;
		border: 1px solid var(--destructive);
		border-radius: 4px;
		cursor: pointer;
	}
	.remove-btn:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.remove-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
