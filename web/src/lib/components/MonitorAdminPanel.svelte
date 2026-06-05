<script lang="ts" module>
	// MonitorAdminPanel — the officer /admin "Monitors" section (WANT-08 + the D-10
	// test-alert / WANT-03 frontend proof, 20-UI-SPEC § /admin Monitors Section).
	// Clones the BankCoinForm/EvictionForm officer-form lifecycle (onMount → fetch →
	// FormField form → save, with authGuard via classifyAdminError). Three lg-gapped
	// sub-blocks: the three guild-wide kill-switch Toggles (EC ON / WTS+raid dark) +
	// the add-channel FormField form + the registered-channel list (remove via the
	// shared ConfirmDialog — the ONE destructive action) + the "Send me a test alert"
	// bot-pulse button with its three feedback states.
	//
	// The two pure decision helpers (the test-alert response→message map + the
	// add-channel validity predicate) live in this module block so they're unit-
	// testable under the node vitest project (no jsdom — the WatcherCodesPanel/
	// ConfirmDialog split). The .svelte instance below is the thin renderer.
	//
	// XSS (T-20-23): the officer-entered server label renders ONLY via plain {}
	// (Svelte auto-escapes) — never the raw-HTML directive.

	/** The three test-alert feedback lines (20-UI-SPEC § Copywriting — copy verbatim). */
	export const TEST_ALERT_SENT =
		'Test alert sent — check your Discord DMs (and your Notifications inbox).';
	export const TEST_ALERT_DM_BLOCKED =
		"We couldn't DM you — Discord is blocking messages from the server. It's logged in your Notifications inbox; turn on server DMs in Discord to receive alerts.";
	export const TEST_ALERT_BOT_DOWN =
		"Couldn't send the test alert — the bot may be offline. Nothing was sent; try again shortly.";

	/** A test-alert feedback line + its semantic kind (the word/icon carries state, not color). */
	export interface TestAlertFeedback {
		kind: 'ok' | 'blocked' | 'down';
		message: string;
	}

	/**
	 * Map the /admin/monitors/test response to its feedback line + kind (the three
	 * states). status==="sent" → ok; error==="dm_blocked" → blocked (the 50007 path);
	 * anything else (error==="bot_unavailable", an absent/unknown shape) → down. A
	 * thrown ApiError (a 5xx bot-down) is mapped by the caller to `down` directly.
	 */
	export function testAlertFeedback(res: { status?: string; error?: string }): TestAlertFeedback {
		if (res.status === 'sent') return { kind: 'ok', message: TEST_ALERT_SENT };
		if (res.error === 'dm_blocked') return { kind: 'blocked', message: TEST_ALERT_DM_BLOCKED };
		return { kind: 'down', message: TEST_ALERT_BOT_DOWN };
	}

	/**
	 * The add-channel submit-enable predicate: a non-blank (trimmed) label AND a
	 * numeric, non-empty channel id AND a set monitor. UX defense-in-depth only —
	 * the server re-validates (`^[0-9]+$` + non-blank + the monitor enum, T-20-25).
	 */
	export function addChannelValid(label: string, channelId: string, monitor: string): boolean {
		return label.trim() !== '' && /^[0-9]+$/.test(channelId.trim()) && monitor !== '';
	}
</script>

<script lang="ts">
	import { onMount, getContext } from 'svelte';
	import Plus from '@lucide/svelte/icons/plus';
	import Send from '@lucide/svelte/icons/send';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import Toggle from './Toggle.svelte';
	import FormField from './FormField.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import StateBlock from './StateBlock.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import {
		fetchMonitors,
		setMonitorFlag,
		addGuildChannel,
		removeGuildChannel,
		sendTestAlert,
		classifyAdminError,
		Unauthenticated,
		type MonitorState,
		type MonitorFlags,
		type GuildChannel
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let flags = $state<MonitorFlags>({ ec: false, wts: false, raid: false });
	let channels = $state<GuildChannel[]>([]);

	// The three kill-switches map a UI flag key (ec/wts/raid) to its monitor enum.
	const KILL_SWITCHES: { key: keyof MonitorFlags; monitor: string; label: string; desc: string; dark: boolean }[] = [
		{
			key: 'ec',
			monitor: 'ec_auction',
			label: 'EC-tunnel auctions',
			desc: 'DM guildies when a wanted item is auctioned in the EC tunnel.',
			dark: false
		},
		{
			key: 'wts',
			monitor: 'wts',
			label: 'WTS cross-server',
			desc: 'DM guildies when a wanted item is posted for sale in a watched WTS channel.',
			dark: true
		},
		{
			key: 'raid',
			monitor: 'raid_target',
			label: 'Raid targets',
			desc: 'DM guildies when a raid target tied to one of their quest wants is called.',
			dark: true
		}
	];

	let flagAnnounce = $state('');
	let flagBusy = $state(false);

	// Add-channel form state (raw strings — the inputs are type=text per the coin idiom).
	let label = $state('');
	let channelId = $state('');
	let monitorType = $state('');
	let adding = $state(false);
	let addSuccess = $state('');
	let addError = $state('');

	let canAdd = $derived(addChannelValid(label, channelId, monitorType) && !adding);

	// Remove (confirm-before-commit) state — the ONE destructive action.
	let removeTarget = $state<GuildChannel | null>(null);
	let removeDialogOpen = $state(false);
	let removing = $state(false);
	let removeSuccess = $state('');
	let removeError = $state('');

	// Test-alert state.
	let testing = $state(false);
	let testKind = $state<'ok' | 'blocked' | 'down' | ''>('');
	let testMessage = $state('');

	const MONITOR_LABEL: Record<string, string> = {
		ec_auction: 'EC-tunnel auctions',
		wts: 'WTS cross-server',
		raid_target: 'Raid targets'
	};

	async function load() {
		phase = 'loading';
		try {
			const state: MonitorState = await fetchMonitors();
			flags = state.flags;
			channels = state.channels;
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	/**
	 * Route a caught error by its server-truth verdict (the EvictionForm pattern). A
	 * 401 → AuthGate → LoginScreen; a not_authorized 403 → collapse to officers-only.
	 * Returns true when handled (the caller stops), false for an inline error.
	 */
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

	async function onFlagToggle(sw: { key: keyof MonitorFlags; monitor: string; label: string }) {
		if (flagBusy) return;
		flagBusy = true;
		flagAnnounce = '';
		const next = !flags[sw.key];
		try {
			await setMonitorFlag(sw.monitor, next);
			// Server-truth re-read so the switch never lies (mirror the panel lifecycle).
			const state = await fetchMonitors();
			flags = state.flags;
			channels = state.channels;
			flagAnnounce = `${sw.label} turned ${next ? 'on' : 'off'}.`;
		} catch (err) {
			if (route(err)) return;
			// Re-read the authoritative state so the toggle reflects the server, not the
			// optimistic flip that failed.
			try {
				const state = await fetchMonitors();
				flags = state.flags;
			} catch {
				/* leave the prior flags; the next load() reconciles */
			}
			flagAnnounce = `Couldn't save that change. Nothing changed — try again.`;
		} finally {
			flagBusy = false;
		}
	}

	async function onAddChannel(e: SubmitEvent) {
		e.preventDefault();
		if (!canAdd) return;
		adding = true;
		addSuccess = '';
		addError = '';
		const submittedLabel = label.trim();
		const submittedMonitor = monitorType;
		try {
			await addGuildChannel({
				label: submittedLabel,
				channel_id: channelId.trim(),
				monitor: submittedMonitor
			});
			const state = await fetchMonitors();
			flags = state.flags;
			channels = state.channels;
			addSuccess = `Added ${submittedLabel} to ${MONITOR_LABEL[submittedMonitor] ?? submittedMonitor} monitoring.`;
			// Clear the form for the next entry.
			label = '';
			channelId = '';
			monitorType = '';
		} catch (err) {
			if (route(err)) return;
			const code =
				err && typeof err === 'object' && 'code' in err
					? ((err as { code?: string }).code ?? '')
					: '';
			const reason = code === 'duplicate' ? 'That channel is already registered for this monitor.' : '';
			addError = `Couldn't add that channel. ${reason} Nothing was added — try again.`.replace('  ', ' ');
		} finally {
			adding = false;
		}
	}

	function openRemoveConfirm(channel: GuildChannel) {
		if (removing) return;
		removeTarget = channel;
		removeDialogOpen = true;
		removeSuccess = '';
		removeError = '';
	}

	async function doRemove() {
		removeDialogOpen = false;
		const target = removeTarget;
		if (!target || removing) return;
		removing = true;
		removeSuccess = '';
		removeError = '';
		try {
			const { removed } = await removeGuildChannel(target.channel_id, target.monitor);
			if (removed) {
				const state = await fetchMonitors();
				flags = state.flags;
				channels = state.channels;
				removeSuccess = `Removed ${target.label} from monitoring.`;
			} else {
				removeError = `Couldn't remove that channel. No change was made — try again.`;
			}
			removeTarget = null;
		} catch (err) {
			if (route(err)) return;
			removeError = `Couldn't remove that channel. No change was made — try again.`;
		} finally {
			removing = false;
		}
	}

	async function onTestAlert() {
		if (testing) return;
		testing = true;
		testKind = '';
		testMessage = '';
		try {
			const res = await sendTestAlert();
			const fb = testAlertFeedback(res);
			testKind = fb.kind;
			testMessage = fb.message;
		} catch (err) {
			if (route(err)) return;
			// A 5xx (bot offline / bad gateway) → the bot-down feedback line.
			testKind = 'down';
			testMessage = TEST_ALERT_BOT_DOWN;
		} finally {
			testing = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else}
	<div class="monitor-admin">
		<!-- 1. Guild-wide kill-switches (D-07/D-08). EC ON; WTS+raid dark. -->
		<div class="block">
			<h3 class="sub-heading">Guild monitors</h3>
			{#each KILL_SWITCHES as sw (sw.key)}
				<div class="switch-row">
					<Toggle
						on={flags[sw.key]}
						label={sw.label}
						disabled={flagBusy}
						onToggle={() => onFlagToggle(sw)}
					/>
					<div class="switch-text">
						<span class="switch-name">{sw.label}</span>
						<span class="switch-desc">{sw.desc}</span>
						{#if sw.dark}
							<span class="dark-note">Ships dark — enable once the bot is invited to the source channels.</span>
						{/if}
					</div>
				</div>
			{/each}
			{#if flagAnnounce}
				<p class="result" aria-live="polite">{flagAnnounce}</p>
			{/if}
		</div>

		<!-- 2. Add a source channel (writes a guild_channel row). -->
		<form class="block add-form" onsubmit={onAddChannel}>
			<h3 class="sub-heading">Add a source channel</h3>
			<FormField label="Server label" id="mon-label">
				<input id="mon-label" class="field" type="text" bind:value={label} />
			</FormField>
			<FormField label="Channel ID" id="mon-channel">
				<input
					id="mon-channel"
					class="field"
					type="text"
					inputmode="numeric"
					pattern="[0-9]*"
					bind:value={channelId}
				/>
			</FormField>
			<FormField label="Monitor type" id="mon-type">
				<select id="mon-type" class="field" bind:value={monitorType}>
					<option value="">Choose a monitor…</option>
					<option value="ec_auction">EC-tunnel auctions</option>
					<option value="wts">WTS cross-server</option>
					<option value="raid_target">Raid targets</option>
				</select>
			</FormField>
			<div class="actions">
				{#if addSuccess}
					<p class="result success" aria-live="polite">{addSuccess}</p>
				{/if}
				{#if addError}
					<p class="result error" aria-live="polite">{addError}</p>
				{/if}
				<button type="submit" class="primary" disabled={!canAdd}>
					{#if adding}
						<LoaderCircle size={16} aria-hidden="true" class="spin" />
						<span>Adding…</span>
					{:else}
						<Plus size={16} aria-hidden="true" />
						<span>Add channel</span>
					{/if}
				</button>
			</div>
		</form>

		<!-- 3. Registered channels + remove (ConfirmDialog — the one destructive action). -->
		<div class="block">
			<h3 class="sub-heading">Registered channels</h3>
			{#if channels.length === 0}
				<p class="empty-note">
					No channels registered yet. EC-tunnel monitoring uses PigParse and needs no
					channel; WTS and raid monitors need a channel here.
				</p>
			{:else}
				<ul class="channel-list">
					{#each channels as ch (ch.id)}
						<li class="channel-row">
							<div class="channel-meta">
								<span class="channel-label">{ch.label}</span>
								<span class="channel-id">#{ch.channel_id}</span>
								<span class="channel-chip">{MONITOR_LABEL[ch.monitor] ?? ch.monitor}</span>
							</div>
							<button
								type="button"
								class="revoke-btn"
								disabled={removing}
								onclick={() => openRemoveConfirm(ch)}
							>
								<Trash2 size={16} aria-hidden="true" />
								Remove
							</button>
						</li>
					{/each}
				</ul>
			{/if}
			{#if removeSuccess}
				<p class="result success" aria-live="polite">{removeSuccess}</p>
			{/if}
			{#if removeError}
				<p class="result error" aria-live="polite">{removeError}</p>
			{/if}
		</div>

		<!-- 4. The D-10 bot-pulse — "Send me a test alert" (three feedback states). -->
		<div class="block">
			<h3 class="sub-heading">Test the bot</h3>
			<div class="actions">
				{#if testMessage}
					<p
						class="result"
						class:success={testKind === 'ok'}
						class:error={testKind === 'blocked'}
						class:neutral={testKind === 'down'}
						aria-live="polite"
					>
						{testMessage}
					</p>
				{/if}
				<button type="button" class="primary" disabled={testing} onclick={onTestAlert}>
					{#if testing}
						<LoaderCircle size={16} aria-hidden="true" class="spin" />
						<span>Sending…</span>
					{:else}
						<Send size={16} aria-hidden="true" />
						<span>Send me a test alert</span>
					{/if}
				</button>
			</div>
		</div>
	</div>

	<ConfirmDialog
		open={removeDialogOpen}
		heading="Remove this channel?"
		body={`Stop monitoring ${removeTarget?.label ?? ''} for ${MONITOR_LABEL[removeTarget?.monitor ?? ''] ?? removeTarget?.monitor ?? ''}? You can add it back later.`}
		confirmLabel={`Remove ${removeTarget?.label ?? ''}`}
		onConfirm={doRemove}
		onCancel={() => {
			removeDialogOpen = false;
			removeTarget = null;
		}}
	/>
{/if}

<style>
	.monitor-admin {
		display: flex;
		flex-direction: column;
		gap: 24px; /* lg between the sub-blocks (UI-SPEC) */
		max-width: 640px;
	}
	.block {
		display: flex;
		flex-direction: column;
		gap: 16px; /* md within a sub-block */
	}
	.add-form {
		gap: 16px;
	}
	.sub-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC Typography) */
		line-height: 1.2;
		color: var(--text);
	}
	.switch-row {
		display: flex;
		align-items: flex-start;
		gap: 16px;
	}
	.switch-text {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.switch-name {
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
	}
	.switch-desc {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		color: var(--text);
		opacity: 0.85;
	}
	.dark-note {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		color: var(--text);
		opacity: 0.6;
	}
	/* Native control styled like BankCoinForm's field (UI-SPEC § Form Contracts). */
	.field {
		min-height: 44px;
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: text;
	}
	select.field {
		cursor: pointer;
	}
	.field:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.actions {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 8px;
	}
	.empty-note {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		color: var(--text);
		opacity: 0.7;
	}
	.channel-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.channel-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 8px 0;
		border-bottom: 1px solid var(--border, var(--accent));
	}
	.channel-meta {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px;
	}
	.channel-label {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
	}
	.channel-id {
		font-family: var(--font-body);
		font-size: 16px;
		color: var(--text);
		opacity: 0.6;
		font-variant-numeric: tabular-nums;
	}
	.channel-chip {
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		padding: 2px 8px;
		border-radius: 4px;
	}
	/* The destructive remove button — the shipped .revoke-btn treatment. */
	.revoke-btn {
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
		flex: none;
	}
	.revoke-btn:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.revoke-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.result {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		align-self: flex-start;
	}
	.result.success {
		color: var(--status-ok);
	}
	.result.error {
		color: var(--status-missing);
	}
	.result.neutral {
		color: var(--status-other);
	}
	/* Primary button = accent fill / --bg text (mirrors BankCoinForm .primary). */
	.primary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px;
		padding: 8px 24px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--bg);
		background: var(--accent);
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	:global(.spin) {
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		:global(.spin) {
			animation: none;
		}
	}
</style>
