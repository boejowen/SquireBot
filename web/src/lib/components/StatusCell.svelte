<script lang="ts" module>
	// StatusCell — the gear/spell status badge (14-UI-SPEC § Color → status
	// tokens). Renders the literal status WORD (OK / MISSING / OTHER / KNOWN) at
	// Label type (13px / 600) in the matching --status-* color over a ~8%-alpha
	// tinted pill. The word is ALWAYS present, so color is never the only signal
	// (a11y / color-blindness — UI-SPEC "Color is never the only signal").

	export type StatusValue = 'OK' | 'MISSING' | 'OTHER' | 'KNOWN';

	// Map each status to its semantic token. OK + KNOWN are both "have it" (the
	// EQ HP/mana color language) → --status-ok; MISSING → --status-missing;
	// OTHER → --status-other (neutral accent). Unknown values fall back to text.
	const TOKEN: Record<StatusValue, string> = {
		OK: 'var(--status-ok)',
		KNOWN: 'var(--status-ok)',
		MISSING: 'var(--status-missing)',
		OTHER: 'var(--status-other)'
	};
</script>

<script lang="ts">
	let { status }: { status: string } = $props();

	let color = $derived(TOKEN[status as StatusValue] ?? 'var(--text)');
</script>

<span class="status-badge" style:color style:--pill={color}>{status}</span>

<style>
	.status-badge {
		display: inline-block;
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		/* ~8% tint of the status color behind the word. color-mix keeps the pill
		   derived from the same --status-* token so every theme stays in sync. */
		background: color-mix(in srgb, var(--pill) 8%, transparent);
		white-space: nowrap;
	}
</style>
