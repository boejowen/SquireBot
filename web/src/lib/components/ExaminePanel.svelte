<script lang="ts">
	// ExaminePanel — the single pinned examine detail panel (Phase 31, INV-02,
	// 31-UI-SPEC §G / D-07 / D-08 / D-09). Prop-driven over the SELECTED slot
	// (single, replace-on-click) + the per-character last_seen. Renders the
	// examineFields() rows in the LOCKED D-08 order with graceful omission (D-09 —
	// the pure logic lives in $lib/examine, node-tested). When slot === null it
	// shows the dimmed "click an item" prompt (§G empty state).
	//
	// SECURITY (T-31-14, the HIGH-severity gate): the wiki link's HTML body is the
	// ONLY {@html} in this component and the only one this phase adds — it routes
	// through composeItemNote (composeNotes.ts), which runs escapeHtml() over every
	// interpolated value AND safeHttpUrl() (http/https scheme allow-list) over the
	// href, so a guildie-named item like `<img onerror=…>` renders inert and a
	// javascript:/data: URL never reaches the href. Every OTHER field (name, flags,
	// slot, stats, price, last-synced) renders via plain {} interpolation (Svelte
	// auto-escapes) — NO other {@html}. The icon never appears here; iconId-based
	// <img> lives in PaperdollSlot.

	import type { InventorySlot } from '$lib/api';
	import { examineFields } from '$lib/examine';
	import { flagColorVar } from '$lib/flags';
	import { composeItemNote, safeHttpUrl, wikiUrlFor } from '$lib/tooltip/composeNotes';

	let {
		slot = null,
		charLastSeen = ''
	}: {
		slot?: InventorySlot | null;
		charLastSeen?: string;
	} = $props();

	// The D-08-ordered, D-09-omitted rows (pure helper — node-tested).
	let fields = $derived(slot ? examineFields(slot, charLastSeen) : []);

	// The priority flag-chip color (No-Drop > Lore > Magic, D-01) — the SAME value the
	// PaperdollSlot tile ring uses, so the examine chip echoes the tile outline. Only a
	// fixed literal var(--flag-*) string (T-40-06); '' when unflagged (the chip field
	// is omitted by examineFields in that case, so this is just the inline color).
	let flagChipColor = $derived(slot ? flagColorVar(slot) : '');

	// The wiki link body — the ONE sanctioned escaped {@html} sink (composeItemNote).
	// The href passes the safeHttpUrl scheme allow-list FIRST (so javascript:/data:
	// can never reach it), then escapeHtml at the sink. A blank/rejected URL renders
	// no link (composeItemNote omits the <a>). We pass only the item name + URL (no
	// summary/prices/quests — those D-08 rows render as escaped {} text below), so
	// the composed body is exactly the title line with the wiki <a>.
	let wikiBodyHtml = $derived.by(() => {
		if (!slot) return '';
		const url = safeHttpUrl(slot.wiki_url || wikiUrlFor(slot.item));
		if (!url) return '';
		return composeItemNote(slot.item, url, null, [], []);
	});
</script>

<aside class="examine" aria-label="Item details">
	{#if !slot}
		<p class="prompt">
			Click an item to see its details — name, stats, PigParse price, wiki link, last-synced.
		</p>
	{:else}
		{#each fields as f (f.kind)}
			{#if f.kind === 'name'}
				<h3 class="ex-name">{f.text}</h3>
			{:else if f.kind === 'flagchip'}
				<!-- The priority flag chip (NO-DROP/LORE/MAGIC), colored by the same
				     --flag-color the tile ring uses (ITEMUI-01). Plain {} text. -->
				<p class="ex-flagchip" style={`--flag-color: ${flagChipColor}`}>{f.text}</p>
			{:else if f.kind === 'flags'}
				<p class="ex-flags">{f.text}</p>
			{:else if f.kind === 'wiki'}
				<!-- The ONE sanctioned escaped {@html} sink (composeItemNote): the title
				     line + the wiki <a>, fully escaped (T-31-14). -->
				<p class="ex-wiki">{@html wikiBodyHtml}</p>
			{:else if f.kind === 'quests'}
				<!-- Named "Used in:" quest links (ITEMUI-02). NATIVE Svelte path (no
				     {@html}): {q.quest_name} is auto-escaped (T-40-04); every href passes
				     safeHttpUrl FIRST (D-05/T-40-05) so a blocked scheme → plain text. -->
				<p class="ex-quests">
					{f.text}
					{#each f.quests ?? [] as q, i (q.quest_name + i)}{#if i > 0},
						{/if}{#if safeHttpUrl(q.source_url)}<a
								href={safeHttpUrl(q.source_url)}
								target="_blank"
								rel="noopener">{q.quest_name}</a
							>{:else}{q.quest_name}{/if}{/each}
				</p>
			{:else if f.kind === 'price'}
				<p class="ex-price">{f.text}</p>
			{:else if f.kind === 'stats'}
				<!-- The in-game stat block — multi-line (newline-separated); pre-line keeps
				     each stat on its own row. Plain {} interpolation (Svelte auto-escapes). -->
				<p class="ex-stats">{f.text}</p>
			{:else if f.kind === 'notes'}
				<p class="ex-notes">{f.text}</p>
			{:else if f.kind === 'lastsynced'}
				<p class="ex-footer">{f.text}</p>
			{:else}
				<p class="ex-line">{f.text}</p>
			{/if}
		{/each}
	{/if}
</aside>

<style>
	.examine {
		border: 1px solid var(--border, var(--accent));
		border-radius: 6px;
		background: var(--panel);
		color: var(--text);
		padding: 16px;
		position: sticky;
		top: 60px;
		min-height: 200px;
		/* Long wiki summaries must stay INSIDE the box: cap the height and scroll the
		   overflow rather than spilling past the border (2026-06-18). min-width:0 lets the
		   panel shrink in its grid column so word-wrap can take effect. */
		max-height: calc(100vh - 80px);
		overflow-y: auto;
		min-width: 0;
	}
	.prompt {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		font-style: italic;
		opacity: 0.75;
	}
	.ex-name {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading */
		line-height: 1.2;
		color: var(--accent);
		margin: 0 0 8px;
	}
	.ex-flags {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--status-other);
		margin: 0 0 8px;
	}
	/* The priority flag chip (ITEMUI-01) — a bordered, transparent-fill text chip in
	   the flag color, beside the name. Mirrors .ex-flags type but bounds the flag
	   color as a small swatch (easier to clear 3:1; doesn't recolor the heading). */
	.ex-flagchip {
		display: inline-block;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--flag-color);
		border: 1px solid var(--flag-color);
		border-radius: 3px;
		padding: 1px 6px;
		margin: 0 0 8px;
	}
	.ex-name,
	.ex-line,
	.ex-stats,
	.ex-notes,
	.ex-price,
	.ex-wiki,
	.ex-quests,
	.ex-footer {
		/* Break long unbreakable tokens (URLs, run-on wiki text) so nothing overflows the
		   290px panel width — pairs with .examine's overflow-y for the vertical case. */
		overflow-wrap: anywhere;
		word-break: break-word;
	}
	.ex-line,
	.ex-stats,
	.ex-notes,
	.ex-price,
	.ex-wiki,
	.ex-quests,
	.ex-footer {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		margin: 4px 0;
	}
	.ex-stats {
		/* The in-game stat block is newline-separated — keep each stat on its own line. */
		white-space: pre-line;
		color: var(--status-ok);
	}
	.ex-notes {
		/* The wiki description/lore — dimmed + italic to set it apart from the live stats. */
		opacity: 0.85;
		font-style: italic;
	}
	.ex-price {
		color: var(--status-other);
		font-variant-numeric: tabular-nums;
	}
	.ex-footer {
		opacity: 0.8;
		font-size: 13px;
		margin-top: 12px;
	}
	/* composeItemNote markup hooks (mirrors ItemTooltip's :global rules). */
	.ex-wiki :global(.tooltip-title) {
		font-family: var(--font-body);
	}
	.ex-wiki :global(.tooltip-wiki-link) {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		text-decoration: none;
		margin-left: 6px;
	}
	/* Named quest links (ITEMUI-02) — the SAME accent-link affordance as the wiki
	   link (a quest link and a wiki link are the same kind of thing: a jump to the
	   P1999 wiki). These are NATIVE Svelte <a>s (not :global), so a direct rule. */
	.ex-quests a {
		color: var(--accent);
		border-bottom: 1px solid var(--accent);
		text-decoration: none;
	}
</style>
