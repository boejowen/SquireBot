<script lang="ts">
	// WantlistPanel — the /wantlist working component (WANT-01/WANT-02, 19-UI-SPEC).
	// Clones WatcherCodesPanel's load→confirm→server-truth-reload lifecycle: onMount
	// fetches BOTH the owner's wants AND the consolidated all-character inventory
	// (fetchView, for the in-guild join, D-06); add/remove ALWAYS re-fetch from the
	// server (authoritative grid — NEVER optimistic-mutate, T-19-16). Composes the
	// WantAddForm (add) + the 5th DataGrid (wantlistColumns) + ConfirmDialog (remove)
	// + the no-wants StateBlock (empty).
	//
	// SERVER-TRUTH (D-02): the owner is session-derived server-side; this page never
	// sends an owner. A 401 bubbles to AuthGate → LoginScreen via authGuard.
	//
	// XSS (T-19-13): every item name / custom label / note renders via plain {}
	// (Svelte auto-escapes) — the only raw-HTML sink in the app stays ItemTooltip.
	// The in-guild indicator reads "In guild" / "Not in guild" (the all-inventory
	// join is the honest superset of the guild bank — review MUST-FIX 3).

	import { onMount, getContext } from 'svelte';
	import DataGrid from './DataGrid.svelte';
	import StateBlock from './StateBlock.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import WantAddForm from './WantAddForm.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { wantlistColumns } from '$lib/columns';
	import { holdersFor, type Holder } from '$lib/wantlist/holders';
	import {
		fetchOwnWants,
		fetchView,
		removeWant,
		muteWant,
		classifyAdminError,
		Unauthenticated,
		type WantlistRow,
		type ViewRow
	} from '$lib/api';

	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	type Phase = 'loading' | 'error' | 'ready';
	let phase = $state<Phase>('loading');
	let wants = $state<WantlistRow[]>([]);
	let viewRows = $state<ViewRow[]>([]);

	// Add announce (polite aria-live).
	let addAnnounce = $state('');

	// Remove (confirm-before-commit) state.
	let removeTarget = $state<WantlistRow | null>(null);
	let removeDialogOpen = $state(false);
	let removing = $state(false);
	let removeSuccessMsg = $state('');
	let removeErrorMsg = $state('');

	// Mute (per-want bell) state — mirrors onRemove/removeBusy but NON-destructive
	// (D-09 → no ConfirmDialog; a mute is reversible in one click).
	let muteBusy = $state(false);
	let muteAnnounce = $state('');

	// Run the reduce-by-char-and-SUM join ONCE per distinct item_id (review MUST-FIX
	// 1) into a memo keyed on item_id, rebuilt when the inventory payload changes.
	// The grid's in-guild cell reads its summed holders from here (one holdersFor
	// pass per item, not per render). A custom want (item_id null) → [].
	//
	// BUGFIX (wantlist-in-guild-id-mismatch): holdersFor now bridges by NORMALIZED
	// NAME (catalog ids ≠ inventory ids — different namespaces), so we pass the
	// want's item_name through. The memo key stays item_id (a stable per-row key;
	// each row carries its own item_name, so two rows that share a name still pass
	// the same name to holdersFor and get the same holder set).
	let holdersByItem = $derived.by(() => {
		const memo = new Map<number, Holder[]>();
		return (row: WantlistRow): Holder[] => {
			if (row.item_id === null) return [];
			let h = memo.get(row.item_id);
			if (h === undefined) {
				h = holdersFor(row.item_id, row.item_name, viewRows);
				memo.set(row.item_id, h);
			}
			return h;
		};
	});

	// The grid columns close over the in-guild holder lookup + the remove opener;
	// rebuilt reactively when viewRows / removing changes (server-truth).
	let columns = $derived(
		wantlistColumns(holdersByItem, openRemoveConfirm, removing, onMute, muteBusy)
	);
	const defaultSorting = [
		{ id: 'priority', desc: true },
		{ id: 'in_guild', desc: false }
	];

	async function load() {
		phase = 'loading';
		try {
			// Both authoritative payloads — the owner's wants + the all-char inventory.
			const [w, v] = await Promise.all([fetchOwnWants(), fetchView()]);
			wants = w;
			viewRows = v;
			phase = 'ready';
		} catch (err) {
			if (route(err)) return;
			phase = 'error';
		}
	}

	// After a successful add: re-fetch the owner's wants (server-truth, never
	// optimistic-mutate). The view rows don't change on an add, so only wants reload.
	async function onAdded(itemName: string) {
		addAnnounce = `Added ${itemName} to your wantlist.`;
		try {
			wants = await fetchOwnWants();
		} catch (err) {
			if (route(err)) return;
			// A reload failure is non-fatal — the add succeeded server-side; surface
			// nothing destructive (the next load() reconciles).
		}
	}

	function openRemoveConfirm(want: WantlistRow) {
		if (removing) return;
		removeTarget = want;
		removeDialogOpen = true;
	}

	async function doRemove() {
		removeDialogOpen = false;
		const target = removeTarget;
		if (!target || removing) return;
		removing = true;
		removeSuccessMsg = '';
		removeErrorMsg = '';
		try {
			const { removed } = await removeWant(target.id);
			if (removed) {
				removeSuccessMsg = `Removed ${target.item_name} from your wantlist.`;
				// Re-load from the server so the grid stays authoritative (mirror add).
				wants = await fetchOwnWants();
			} else {
				// removed:false = not the caller's / already removed — a no-op, not a
				// hard error. Keep the row; surface a quiet note.
				removeErrorMsg = `Couldn't remove that. No change was made — try again.`;
			}
			removeTarget = null;
		} catch (err) {
			if (route(err)) return;
			removeErrorMsg = `Couldn't remove that. No change was made — try again.`;
		} finally {
			removing = false;
		}
	}

	/**
	 * Toggle a single want's mute flag (D-09). Mirrors doRemove's load→mutate→
	 * server-truth-reload (NEVER optimistic — the grid stays authoritative): flip
	 * via muteWant(id, !muted), then re-fetch the wants so the bell reads the server.
	 * A custom want never reaches here (its cell is disabled). Non-destructive, so no
	 * ConfirmDialog.
	 */
	async function onMute(want: WantlistRow) {
		if (muteBusy) return;
		muteBusy = true;
		muteAnnounce = '';
		const next = !want.muted;
		try {
			await muteWant(want.id, next);
			// Server-truth reload so the bell reflects the persisted state, not the flip.
			wants = await fetchOwnWants();
			muteAnnounce = next
				? `Muted alerts for ${want.item_name}.`
				: `Unmuted alerts for ${want.item_name}.`;
		} catch (err) {
			if (route(err)) return;
			// A reload/write failure is non-fatal — re-read to reconcile the bell.
			try {
				wants = await fetchOwnWants();
			} catch {
				/* leave the prior rows; the next load() reconciles */
			}
		} finally {
			muteBusy = false;
		}
	}

	/**
	 * Route a caught error by its server-truth verdict (the WatcherCodesPanel
	 * pattern). Returns true when handled as a re-route (a bubbled 401 → AuthGate →
	 * LoginScreen) so the caller stops; false for a generic error rendered inline.
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

	onMount(() => {
		void load();
	});
</script>

{#if phase === 'loading'}
	<StateBlock kind="loading" />
{:else if phase === 'error'}
	<StateBlock kind="error" onRetry={load} />
{:else}
	<div class="wantlist-panel">
		<!-- Add-item block (above the grid). On success it re-fetches the wants. -->
		<WantAddForm {onAdded} />
		{#if addAnnounce}
			<p class="result success" aria-live="polite">{addAnnounce}</p>
		{/if}

		<div class="divider"></div>

		{#if removeSuccessMsg}
			<p class="result success" aria-live="polite">{removeSuccessMsg}</p>
		{/if}
		{#if removeErrorMsg}
			<p class="result error" aria-live="polite">{removeErrorMsg}</p>
		{/if}
		{#if muteAnnounce}
			<p class="result success" aria-live="polite">{muteAnnounce}</p>
		{/if}

		{#if wants.length === 0}
			<StateBlock kind="no-wants" />
		{:else}
			<DataGrid data={wants} {columns} {defaultSorting} />
		{/if}
	</div>

	<ConfirmDialog
		open={removeDialogOpen}
		heading="Remove this want?"
		body={`Remove ${removeTarget?.item_name ?? ''} from your wantlist? You can always add it back later.`}
		confirmLabel={`Remove ${removeTarget?.item_name ?? ''}`}
		onConfirm={doRemove}
		onCancel={() => {
			removeDialogOpen = false;
			removeTarget = null;
		}}
	/>
{/if}

<style>
	.wantlist-panel {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.divider {
		border-top: 1px solid var(--border, var(--accent));
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
