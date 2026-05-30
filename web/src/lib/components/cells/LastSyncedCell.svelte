<script lang="ts">
	// LastSyncedCell — the `Last Synced` cell for view/bank: the ISO timestamp
	// shown as a friendly date with a small freshness DOT (14-UI-SPEC § Color
	// "Last-Synced freshness"). v1 colored the cell green/amber/red at <7d /
	// <30d / >=30d; ported here as a subtle dot reusing the status tokens:
	//   <7d  -> --status-ok      (fresh)
	//   <30d -> --status-other   (aging / amber-ish)
	//   >=30d-> --status-missing (stale)
	// Data encoding, not chrome — the human-readable date is always present so
	// color is never the only signal (a11y).
	let { lastSynced }: { lastSynced: string } = $props();

	const DAY_MS = 86_400_000;

	let parsed = $derived(lastSynced ? new Date(lastSynced) : null);
	let valid = $derived(parsed != null && !Number.isNaN(parsed.getTime()));

	let ageDays = $derived(valid ? (Date.now() - (parsed as Date).getTime()) / DAY_MS : Infinity);

	let freshness = $derived(
		!valid
			? { token: 'var(--text)', label: 'unknown' }
			: ageDays < 7
				? { token: 'var(--status-ok)', label: 'fresh (under 7 days)' }
				: ageDays < 30
					? { token: 'var(--status-other)', label: 'aging (under 30 days)' }
					: { token: 'var(--status-missing)', label: 'stale (30+ days)' }
	);

	// Friendly local date (YYYY-MM-DD) — stable, locale-light, scannable.
	let dateLabel = $derived(valid ? (parsed as Date).toISOString().slice(0, 10) : '—');
</script>

<span class="last-synced">
	<span class="dot" style:background={freshness.token} title={freshness.label} aria-hidden="true"
	></span>
	<span class="date">{dateLabel}</span>
</span>

<style>
	.last-synced {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-variant-numeric: tabular-nums;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex: 0 0 auto;
	}
</style>
