<script lang="ts">
	// PriceCell — the `Price` column cell for view/bank: `<n>pp` rendered with
	// tabular figures so the column aligns (14-UI-SPEC "Price = <n>pp tabular";
	// Typography "tabular-nums"). `price` is nullable (the API sends null when
	// neither WTS nor WTB has a30>0) — render blank, never a fabricated 0.
	let { price }: { price: number | null } = $props();

	// en-US comma grouping, rounded — matches composeNotes' formatPp posture.
	let label = $derived(
		price == null || !Number.isFinite(price) ? '' : `${Math.round(price).toLocaleString('en-US')}pp`
	);
</script>

<span class="price">{label}</span>

<style>
	.price {
		font-variant-numeric: tabular-nums;
	}
</style>
