<script lang="ts" module>
	// NotificationRow — one inbox alert row (20-UI-SPEC § Inbox Region + § CAN'T-DM
	// Row). The two pieces of presentation logic that the P15 lesson says MUST be
	// unit-tested (165 green tests, 2 crashing browser blockers) live in this module
	// block as pure, DOM-free functions:
	//   1. deliveryBadge — the send_status → {word, token} mapping (the load-bearing
	//      delivery encoding; color is NEVER the only signal, the WORD always rides).
	//   2. relativeTime — sent_at is unix EPOCH SECONDS (NOT ms); a seconds-as-ms
	//      mistake renders "57 years ago" (the P15 epoch-sec date crasher). This
	//      helper multiplies by 1000 exactly once.
	//
	// XSS (T-20-17): the alert text / source / detail are user/wiki-controlled — the
	// TEMPLATE renders them via plain {} (Svelte auto-escape) ONLY; never {@html}.

	import type { AlertLogRow } from '$lib/api';

	export type SendStatus = AlertLogRow['send_status'];

	/** The delivery badge for a send_status: the WORD + the --status-* token + the unread-row default. */
	export interface DeliveryBadge {
		word: string;
		/** A CSS var() string for the badge color + its ~8% pill (the StatusCell idiom). */
		token: string;
		/** dm_blocked rows carry the actionable hint + the distinct red treatment. */
		blocked: boolean;
	}

	/**
	 * Map a send_status to its delivery badge. DELIVERED = have-it green; CAN'T DM =
	 * rose-red (the one outcome the user must ACT on — earns the hint); ERROR =
	 * neutral (a transient backend hiccup). An unknown status degrades to ERROR
	 * rather than rendering a blank badge.
	 */
	export function deliveryBadge(status: SendStatus | string): DeliveryBadge {
		switch (status) {
			case 'sent':
				return { word: 'DELIVERED', token: 'var(--status-ok)', blocked: false };
			case 'dm_blocked':
				return { word: "CAN'T DM", token: 'var(--status-missing)', blocked: true };
			case 'error':
			default:
				return { word: 'ERROR', token: 'var(--status-other)', blocked: false };
		}
	}

	/**
	 * "2 hours ago" / "yesterday"-style relative time for a unix EPOCH-SECONDS
	 * timestamp. The ×1000 (seconds → ms) happens HERE, exactly once — passing the
	 * raw seconds straight to Date() is the P15 crasher ("57 years ago"). `now` is
	 * injectable for deterministic tests. A non-finite input degrades to '' (the
	 * caller hides the timestamp) rather than rendering "Invalid Date".
	 */
	export function relativeTime(sentAtSeconds: number, now: number = Date.now()): string {
		if (!Number.isFinite(sentAtSeconds)) return '';
		const ms = sentAtSeconds * 1000; // epoch SECONDS → ms (the load-bearing ×1000)
		const diff = Math.max(0, now - ms);
		const min = Math.floor(diff / 60_000);
		if (min < 1) return 'just now';
		if (min < 60) return `${min} min ago`;
		const hr = Math.floor(min / 60);
		if (hr < 24) return `${hr} hour${hr === 1 ? '' : 's'} ago`;
		const day = Math.floor(hr / 24);
		if (day === 1) return 'yesterday';
		if (day < 30) return `${day} days ago`;
		if (day < 365) {
			const mon = Math.max(1, Math.floor(day / 30));
			return `${mon} month${mon === 1 ? '' : 's'} ago`;
		}
		const yr = Math.floor(day / 365);
		return `${yr} year${yr === 1 ? '' : 's'} ago`;
	}

	/** The absolute date string for the row's `title` (hover/SR full timestamp). */
	export function absoluteTime(sentAtSeconds: number): string {
		if (!Number.isFinite(sentAtSeconds)) return '';
		return new Date(sentAtSeconds * 1000).toLocaleString();
	}
</script>

<script lang="ts">
	import CircleCheck from '@lucide/svelte/icons/circle-check';
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	// AlertLogRow is already imported in the module block above (in scope here).

	let {
		row,
		onMarkRead,
		busy = false
	}: {
		row: AlertLogRow;
		/** Marks THIS row read (the panel does the server write + re-read). */
		onMarkRead: (id: number) => void;
		busy?: boolean;
	} = $props();

	let badge = $derived(deliveryBadge(row.send_status));
	let unread = $derived(row.read_at === null);
	// The alert text: the server-composed detail, or a quiet fallback. Rendered via
	// plain {} ONLY (XSS boundary — never {@html}).
	let alertText = $derived(row.detail ?? `Alert from ${row.source}`);
	let rel = $derived(relativeTime(row.sent_at));
	let abs = $derived(absoluteTime(row.sent_at));
</script>

<div class="row" class:unread>
	<div class="main">
		<!-- Alert text — user/wiki-controlled → plain {} auto-escape ONLY (T-20-17). -->
		<p class="alert-text">{alertText}</p>
		{#if badge.blocked}
			<!-- The load-bearing CAN'T-DM hint (D-05 / Pitfall 3) — actionable copy. -->
			<!-- prettier-ignore -->
			<p class="hint">We couldn't DM you — Discord is blocking messages from the server. Turn on "Allow direct messages from server members" in your Discord privacy settings to receive these.</p>
		{/if}
	</div>

	<div class="meta">
		<!-- Delivery badge: WORD + icon + ~8% tinted pill — color is never the only
		     signal (the StatusCell idiom). -->
		<span class="delivery" class:blocked={badge.blocked} style:color={badge.token} style:--pill={badge.token}>
			{#if row.send_status === 'sent'}
				<CircleCheck size={14} aria-hidden="true" />
			{:else if row.send_status === 'dm_blocked'}
				<TriangleAlert size={14} aria-hidden="true" />
			{:else}
				<CircleAlert size={14} aria-hidden="true" />
			{/if}
			{badge.word}
		</span>

		{#if rel}
			<time class="rel" datetime={abs} title={abs}>{rel}</time>
		{/if}

		{#if unread}
			<button
				type="button"
				class="mark-read"
				disabled={busy}
				aria-label="Mark read"
				onclick={() => onMarkRead(row.id)}
			>
				<CircleCheck size={16} aria-hidden="true" />
			</button>
		{/if}
	</div>
</div>

<style>
	.row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px; /* md horizontal (UI-SPEC) */
		padding: 8px 16px; /* sm vertical / md horizontal (UI-SPEC) */
		border-bottom: 1px solid var(--border, rgba(74, 101, 133, 0.4));
		color: color-mix(in srgb, var(--text) 80%, transparent);
	}
	/* Unread = a 2px accent left-border + full-opacity text (NOT a weight change —
	   only 2 weights). Color-not-only: unread is ALSO conveyed by the mark-read
	   button's presence + its inclusion in the nav badge count. */
	.row.unread {
		border-left: 2px solid var(--accent);
		padding-left: 14px; /* 16 − 2px border keeps the text aligned */
		color: var(--text);
	}
	.main {
		display: flex;
		flex-direction: column;
		gap: 8px; /* sm — hint offset (UI-SPEC) */
		min-width: 0;
	}
	.alert-text {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
	}
	.hint {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		color: var(--status-missing);
	}
	.meta {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
	}
	.delivery {
		display: inline-flex;
		align-items: center;
		gap: 4px; /* xs — icon↔word (UI-SPEC) */
		font-family: var(--font-display);
		font-size: 13px; /* Label (UI-SPEC) */
		font-weight: 600;
		line-height: 1.3;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 4px;
		background: color-mix(in srgb, var(--pill) 8%, transparent);
		white-space: nowrap;
	}
	.rel {
		font-family: var(--font-body);
		font-size: 16px;
		opacity: 0.7;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.mark-read {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 44px;
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px;
		background: none;
		border: none;
		color: var(--accent);
		cursor: pointer;
	}
	.mark-read:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.mark-read:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 4px;
	}
</style>
