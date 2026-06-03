<script lang="ts" module>
	// PriorityCell — the wantlist Priority badge (19-UI-SPEC Grid Contract). The
	// literal word (High / Med / Low) at Label type so color is never the only
	// signal: High in --accent, Med in --text, Low muted. Mirrors the StatusCell
	// text-badge idiom.
	const LABEL: Record<string, string> = { high: 'High', med: 'Med', low: 'Low' };
	const COLOR: Record<string, string> = {
		high: 'var(--accent)',
		med: 'var(--text)',
		low: 'color-mix(in srgb, var(--text) 70%, transparent)'
	};
</script>

<script lang="ts">
	let { priority }: { priority: string } = $props();
	let label = $derived(LABEL[priority] ?? priority);
	let color = $derived(COLOR[priority] ?? 'var(--text)');
</script>

<span class="priority-badge" style:color>{label}</span>

<style>
	.priority-badge {
		display: inline-block;
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		white-space: nowrap;
	}
</style>
