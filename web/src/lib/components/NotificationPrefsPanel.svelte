<script lang="ts">
	// NotificationPrefsPanel — the top region of /notifications (WANT-04 / D-01,
	// D-02; 20-UI-SPEC § Preferences Region). Clones the WatcherCodesPanel/WantlistPanel
	// load→mutate→server-truth-reload lifecycle: onMount fetches the caller's prefs
	// (default-ON for a new user, D-01); each toggle writes server-side then RE-READS
	// (never optimistic — the switch must never sit in a lying position). When master
	// is OFF the three per-monitor rows visibly dim (still readable; aria-disabled is
	// NOT set — the copy makes clear they're inert while master is off).
	//
	// SERVER-TRUTH (D-02): the owner is session-derived server-side; the page never
	// sends an owner. A 401 bubbles to AuthGate → LoginScreen via authGuard.

	import { onMount, getContext } from 'svelte';
	import StateBlock from './StateBlock.svelte';
	import Toggle from './Toggle.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import {
		fetchPrefs,
		savePrefs,
		classifyAdminError,
		Unauthenticated,
		type NotifyPrefs
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let prefs = $state<NotifyPrefs>({ master: true, ec: true, wts: true, raid: true });
	let busy = $state(false);
	let announce = $state('');
	let errorMsg = $state('');

	type PrefKey = keyof NotifyPrefs;

	// The per-monitor rows dim while master is OFF (still readable — D-01 global mute).
	let masterOff = $derived(prefs.master !== true);

	const NAMES: Record<PrefKey, string> = {
		master: 'Alerts',
		ec: 'EC-tunnel auctions',
		wts: 'WTS cross-server',
		raid: 'Raid targets'
	};

	async function load() {
		phase = 'loading';
		try {
			prefs = await fetchPrefs();
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	// Toggle a pref → write the full prefs body server-side → RE-READ the authoritative
	// state (server-truth, never optimistic — the WantlistPanel rule). On failure,
	// surface the inline error AND re-read so the switch reflects the real state.
	async function toggle(key: PrefKey) {
		if (busy) return;
		busy = true;
		errorMsg = '';
		const next: NotifyPrefs = { ...prefs, [key]: !(prefs[key] === true) };
		const turningOn = next[key] === true;
		try {
			prefs = await savePrefs(next);
			const name = key === 'master' ? 'Alerts' : NAMES[key];
			announce = `${name} turned ${turningOn ? 'on' : 'off'}.`;
		} catch (err) {
			if (route(err)) return;
			errorMsg = 'Couldn’t save that preference. Nothing changed — try again.';
			// Re-read so we never leave the switch in a lying position.
			try {
				prefs = await fetchPrefs();
			} catch (e) {
				if (route(e)) return;
			}
		} finally {
			busy = false;
		}
	}

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

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else}
	<section class="prefs" aria-labelledby="prefs-heading">
		<h2 id="prefs-heading" class="sub-heading">Alert preferences</h2>

		<!-- Master — All alerts. Toggling OFF is the global mute (D-01). -->
		<div class="pref-row">
			<div class="pref-text">
				<span class="pref-name">All alerts</span>
				<span class="pref-desc">
					Turn every SquireBot alert on or off. The per-monitor switches below only apply while
					this is on.
				</span>
			</div>
			<Toggle
				on={prefs.master}
				label={NAMES.master}
				disabled={busy}
				onToggle={() => toggle('master')}
			/>
		</div>

		<!-- Per-monitor rows — dim (still readable) while master is OFF. -->
		<div class="monitors" class:dim={masterOff}>
			<div class="pref-row">
				<div class="pref-text">
					<span class="pref-name">EC-tunnel auctions</span>
					<span class="pref-desc">DM me when a wanted item is auctioned in the EC tunnel.</span>
				</div>
				<Toggle on={prefs.ec} label={NAMES.ec} disabled={busy} onToggle={() => toggle('ec')} />
			</div>

			<div class="pref-row">
				<div class="pref-text">
					<span class="pref-name">WTS cross-server</span>
					<span class="pref-desc">
						DM me when a wanted item is posted for sale in a watched WTS channel.
					</span>
				</div>
				<Toggle on={prefs.wts} label={NAMES.wts} disabled={busy} onToggle={() => toggle('wts')} />
			</div>

			<div class="pref-row">
				<div class="pref-text">
					<span class="pref-name">Raid targets</span>
					<span class="pref-desc">
						DM me when a raid target tied to one of my quest wants is called.
					</span>
				</div>
				<Toggle on={prefs.raid} label={NAMES.raid} disabled={busy} onToggle={() => toggle('raid')} />
			</div>

			<!-- Honest "ships dark" note under the per-monitor rows (UI-SPEC default: include). -->
			<p class="dark-note">These monitors switch on as the guild's officers enable them.</p>
		</div>

		<!-- 260610-fm5 WS3: point at the per-item mute bell (it lives on the wantlist,
		     not here). OUTSIDE .monitors so it never dims — it's navigation, not a
		     monitor setting. -->
		<p class="mute-hint">
			To mute alerts for a single item, use the bell next to it on
			<a href="/wantlist">your wantlist</a>.
		</p>

		{#if announce}
			<p class="result success" aria-live="polite">{announce}</p>
		{/if}
		{#if errorMsg}
			<p class="result error" aria-live="polite">{errorMsg}</p>
		{/if}
	</section>
{/if}

<style>
	.prefs {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.sub-heading {
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 20px; /* Heading (UI-SPEC) */
		line-height: 1.2;
		color: var(--text);
	}
	.monitors {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	/* Master OFF → dim the per-monitor rows (still readable; NOT aria-disabled). */
	.monitors.dim {
		opacity: 0.5;
	}
	.pref-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px; /* md — toggle row ↔ description (UI-SPEC) */
	}
	.pref-text {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}
	.pref-name {
		font-family: var(--font-display);
		font-size: 13px; /* Label (UI-SPEC) */
		font-weight: 600;
		letter-spacing: 0.04em;
		color: var(--text);
	}
	.pref-desc {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	.dark-note {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.7;
	}
	.mute-hint {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.5;
		opacity: 0.85;
	}
	.mute-hint a {
		color: var(--accent);
	}
	.result {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
	}
	.result.success {
		color: var(--status-ok);
	}
	.result.error {
		color: var(--status-missing);
	}
</style>
