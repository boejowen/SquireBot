<script lang="ts" module>
	// ReasonCell — the wantlist Reason chip (19-UI-SPEC Grid Contract). The
	// load-bearing buy/quest differentiator (D-05: the same item may be on the list
	// twice — once Buy, once Quest). Neutral --text chip; the WORD carries the
	// meaning (color is never the only signal).
	const LABEL: Record<string, string> = { buy: 'Buy', quest: 'Quest' };
</script>

<script lang="ts">
	let { reason }: { reason: string } = $props();
	let label = $derived(LABEL[reason] ?? reason);
</script>

<span class="reason-chip">{label}</span>

<style>
	.reason-chip {
		display: inline-block;
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		color: var(--text);
		background: color-mix(in srgb, var(--text) 6%, transparent);
		white-space: nowrap;
	}
</style>
