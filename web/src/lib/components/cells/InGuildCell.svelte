<script lang="ts">
	// InGuildCell — the wantlist's signature "where in the guild is it?" cell
	// (19-UI-SPEC § In-Guild Display Contract, D-06/D-07). Three states:
	//   - catalog want, ≥1 holder → "In guild" (--status-ok) badge + one
	//     `↳ <Char>: <count>` line PER CHARACTER (counts SUMMED via holdersFor —
	//     review MUST-FIX 1; reuses the SearchResults ↳-line visual treatment).
	//     >5 holders collapse behind a "Show all N" / "Show fewer" toggle
	//     (COLLAPSE_THRESHOLD).
	//   - catalog want, 0 holders → "Not in guild" (--status-missing) badge.
	//   - custom want (item_id null) → a muted "—" (--status-other), no join.
	// Char names render via plain {} (auto-escape) — never {@html} (T-19-13).
	//
	// The label is "In guild" / "Not in guild" everywhere (NOT "In bank"): the join
	// is the all-inventory fetchView() (worn + carried + bank, D-06), so "in guild"
	// is the honest superset of "in the bank" (review MUST-FIX 3).
	import { COLLAPSE_THRESHOLD } from '$lib/search/searchIndex';
	import type { WantlistRow } from '$lib/api';
	import type { Holder } from '$lib/wantlist/holders';

	// `holders` is the already-summed holder list (the panel runs holdersFor once
	// per item_id and passes the result — one reduce-by-char-and-SUM per item, not
	// per render). A custom want (item_id null) gets an empty list → the "—" branch.
	let { row, holders }: { row: WantlistRow; holders: Holder[] } = $props();

	let collapsed = $derived(holders.length > COLLAPSE_THRESHOLD);
	let expanded = $state(false);
	let shown = $derived(collapsed && !expanded ? holders.slice(0, COLLAPSE_THRESHOLD) : holders);
</script>

{#if row.item_id === null}
	<!-- Custom want (D-07): excluded from the join — a neutral em-dash. -->
	<span class="dash" aria-label="Not applicable for a custom want">—</span>
{:else if holders.length > 0}
	<div class="in-guild">
		<span class="badge ok">In guild</span>
		<ul class="holders">
			{#each shown as h (h.char)}
				<li class="holder">↳ {h.char}: {h.count}</li>
			{/each}
		</ul>
		{#if collapsed}
			<button class="expand" type="button" onclick={() => (expanded = !expanded)}>
				{#if expanded}Show fewer{:else}Show all {holders.length} holders{/if}
			</button>
		{/if}
	</div>
{:else}
	<span class="badge missing">Not in guild</span>
{/if}

<style>
	.in-guild {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.badge {
		display: inline-block;
		align-self: flex-start;
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		white-space: nowrap;
	}
	.badge.ok {
		color: var(--status-ok);
		background: color-mix(in srgb, var(--status-ok) 8%, transparent);
	}
	.badge.missing {
		color: var(--status-missing);
		background: color-mix(in srgb, var(--status-missing) 8%, transparent);
	}
	.dash {
		color: var(--status-other);
		opacity: 0.7;
	}
	.holders {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.holder {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		padding-left: 8px;
		font-variant-numeric: tabular-nums;
	}
	.expand {
		align-self: flex-start;
		min-height: 44px;
		background: none;
		border: none;
		color: var(--accent);
		font-family: var(--font-body);
		font-size: 13px;
		cursor: pointer;
		padding: 4px 0;
	}
	.expand:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 2px;
	}
</style>
