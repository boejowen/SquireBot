<script lang="ts">
	// NotificationInbox — the bottom region of /notifications (WANT-04 / D-04, D-05;
	// 20-UI-SPEC § Inbox Region). Clones the WantlistPanel load→list→StateBlock
	// lifecycle: onMount fetches the owner's alert_log rows (newest-first); mark-read
	// + mark-all-read ALWAYS re-fetch from the server (server-truth — NEVER
	// optimistic-mutate, the T-19-16 rule) and refresh the nav badge via the unread
	// store. It is BOTH "what was I pinged about?" AND the 50007 CAN'T-DM safety net.
	//
	// SERVER-TRUTH (D-02): the owner is session-derived server-side; the page never
	// sends an owner — mark-read sends the alert id only. A 401 bubbles to AuthGate →
	// LoginScreen via authGuard.
	//
	// XSS (T-20-17): NotificationRow renders all alert text via plain {} (auto-escape)
	// — the only raw-HTML sink in the app stays ItemTooltip (unused here).

	import { onMount, getContext } from 'svelte';
	import StateBlock from './StateBlock.svelte';
	import NotificationRow from './NotificationRow.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { refreshUnread } from '$lib/stores/unread';
	import {
		fetchInbox,
		markRead,
		markAllRead,
		classifyAdminError,
		Unauthenticated,
		type AlertLogRow
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let rows = $state<AlertLogRow[]>([]);
	let busy = $state(false);
	let announce = $state('');

	let unreadN = $derived(rows.filter((r) => r.read_at === null).length);

	async function load() {
		phase = 'loading';
		try {
			rows = await fetchInbox();
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	// Mark a single row read → re-fetch (server-truth, never optimistic) + refresh
	// the nav badge so the unread count stays authoritative.
	async function onMarkRead(id: number) {
		if (busy) return;
		busy = true;
		try {
			await markRead(id);
			rows = await fetchInbox();
			void refreshUnread();
		} catch (err) {
			if (route(err)) return;
			// A reload failure is non-fatal — the read succeeded server-side; the next
			// load() reconciles.
		} finally {
			busy = false;
		}
	}

	async function onMarkAll() {
		if (busy || unreadN === 0) return;
		busy = true;
		announce = '';
		try {
			await markAllRead();
			rows = await fetchInbox();
			void refreshUnread();
			announce = 'Marked all alerts read.';
		} catch (err) {
			if (route(err)) return;
		} finally {
			busy = false;
		}
	}

	/** Route a caught error by its server-truth verdict (the WantlistPanel pattern). */
	function route(err: unknown): boolean {
		if (err instanceof Unauthenticated) {
			if (authGuard) authGuard(err);
			return true;
		}
		const verdict = classifyAdminError(err);
		if (verdict === 'officers-only' || verdict === 'unauthenticated') {
			if (authGuard) authGuard(err);
			return true;
		}
		return false;
	}

	onMount(() => {
		void load();
	});
</script>

<section class="inbox" aria-labelledby="inbox-heading">
	<div class="inbox-head">
		<h2 id="inbox-heading" class="sub-heading">Your alerts</h2>
		<button
			type="button"
			class="mark-all"
			disabled={busy || unreadN === 0}
			onclick={onMarkAll}
		>
			Mark all read
		</button>
	</div>

	{#if announce}
		<p class="result success" aria-live="polite">{announce}</p>
	{/if}

	{#if phase === 'loading'}
		<StateBlock kind="loading" />
	{:else if phase === 'error'}
		<StateBlock kind="error" onRetry={load} />
	{:else if rows.length === 0}
		<StateBlock kind="no-notifications" />
	{:else}
		<ul class="rows">
			{#each rows as row (row.id)}
				<li>
					<NotificationRow {row} {onMarkRead} {busy} />
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.inbox {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.inbox-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.sub-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC) */
		line-height: 1.2;
		color: var(--text);
	}
	/* "Mark all read" — accent-text action (UI-SPEC § Color reserved-for #6). */
	.mark-all {
		min-height: 44px; /* touch target (UI-SPEC) */
		padding: 8px 12px;
		background: none;
		border: none;
		color: var(--accent);
		font-family: var(--font-display);
		font-size: 13px;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		cursor: pointer;
	}
	.mark-all:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.mark-all:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 4px;
	}
	.rows {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
	}
	.result {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
	}
	.result.success {
		color: var(--status-ok);
	}
</style>
