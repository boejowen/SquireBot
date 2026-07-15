<script lang="ts">
	// PortraitControl — the per-character portrait upload/remove control (Phase 41 /
	// CHARUI-02, D-03/D-05/D-06/D-08; 41-UI-SPEC § Interaction & State Contract states
	// 1-8 + Remove flow). It sits directly beneath the portrait frame in InventoryWindow,
	// rendered ONLY when the parent's canEdit gate is true (assignee OR officer). The
	// state machine + .primary button + .result aria-live + spinner + 44px targets +
	// :focus-visible + prefers-reduced-motion + the authGuard 401/403 hand-off all copy
	// CharMetaForm.svelte; the file-pick → validate → canvas-downscale → preview → upload
	// is genuinely new (no file-input/canvas precedent in the web tree).
	//
	// Flow: Add/Change photo → hidden <input type="file" accept="png,jpeg,webp"> (SVG/GIF
	// excluded) → validateImageFile (client defense-in-depth; the server re-sniffs magic
	// bytes + re-caps at 256KB and is authoritative) → canvas downscale to a ~512px square
	// → preview in the frame + Save/Cancel → Save posts base64-in-JSON (setPortrait) →
	// success bumps the local cache-bust via the `changed` callback. Remove → ConfirmDialog
	// → removePortrait (DELETE).
	//
	// SECURITY: the character name renders ONLY via plain {} (Svelte auto-escapes), NEVER
	// the raw-HTML directive (T-41W-01). Client validation is UX only; the Go store re-checks
	// the assignee-or-officer gate + re-sniffs the bytes on every write (41-01).

	import { getContext } from 'svelte';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import ImageUp from '@lucide/svelte/icons/image-up';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Upload from '@lucide/svelte/icons/upload';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import { AUTH_GUARD_KEY, type AuthGuard } from './AuthGate.svelte';
	import { setPortrait, removePortrait, Unauthenticated, Forbidden } from '$lib/api';
	import { validateImageFile, MAX_PORTRAIT_BYTES, stripDataUrlPrefix } from '$lib/portrait';

	let {
		char,
		hasPortrait,
		onchanged
	}: {
		char: string;
		hasPortrait: boolean;
		/** Emitted after a successful set/remove so the parent bumps its cache-bust key. */
		onchanged: (detail: { has_portrait: boolean; updated_at: string }) => void;
	} = $props();

	// A 401 mid-session hands off to the whole-site gate (→ LoginScreen); a 403
	// not_authorized (the gate collapsed mid-session — an un-assign / de-officer) also
	// routes through it defensively.
	const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);

	// The downscale target: a square edge in px (D-04 recommends ~256–512px). 512 keeps the
	// photo crisp on hi-dpi while staying well under the 256 KB decoded cap after JPEG encode.
	const DOWNSCALE_EDGE = 512;

	type Mode = 'idle' | 'previewing' | 'saving';
	let mode = $state<Mode>('idle');
	let successMsg = $state('');
	let errorMsg = $state('');
	// The downscaled preview data URL (shown in the frame) + its raw base64 payload.
	let previewDataUrl = $state('');
	let confirmingRemove = $state(false);

	let fileInput = $state<HTMLInputElement | undefined>();

	function openPicker() {
		successMsg = '';
		errorMsg = '';
		fileInput?.click();
	}

	async function onPick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		// Reset the input value so re-picking the SAME file fires change again.
		input.value = '';
		if (!file) return;
		successMsg = '';
		errorMsg = '';

		// Client validation (UX defense-in-depth; the server re-checks — states 3).
		const v = validateImageFile({ type: file.type, size: file.size });
		if (!v.ok) {
			errorMsg =
				v.reason === 'type'
					? "That file isn't a PNG, JPEG, or WebP. Choose an image in one of those formats."
					: 'That image is too large (max 256 KB). Try a smaller or more compressed image.';
			return;
		}

		try {
			// Downscale/crop to a square (state 4) — this is the DOM-bound half (canvas/Image),
			// so it lives here, not in the pure portrait.ts. object-fit:cover crop math.
			previewDataUrl = await downscaleToSquare(file, DOWNSCALE_EDGE);
			mode = 'previewing';
		} catch {
			errorMsg = "Couldn't read that image. Try a different file.";
		}
	}

	// Read the file, draw a center-cropped square onto a canvas, return a data URL.
	function downscaleToSquare(file: File, edge: number): Promise<string> {
		return new Promise((resolve, reject) => {
			const url = URL.createObjectURL(file);
			const img = new Image();
			img.onload = () => {
				try {
					const canvas = document.createElement('canvas');
					canvas.width = edge;
					canvas.height = edge;
					const ctx = canvas.getContext('2d');
					if (!ctx) {
						reject(new Error('no 2d context'));
						return;
					}
					// object-fit: cover — scale the shorter side to the square edge, center-crop.
					const side = Math.min(img.width, img.height);
					const sx = (img.width - side) / 2;
					const sy = (img.height - side) / 2;
					ctx.drawImage(img, sx, sy, side, side, 0, 0, edge, edge);
					// JPEG keeps photos small (well under the 256 KB decoded cap); quality 0.85 is
					// a good size/fidelity balance. The server re-sniffs → stores image/jpeg.
					resolve(canvas.toDataURL('image/jpeg', 0.85));
				} catch (err) {
					reject(err as Error);
				} finally {
					URL.revokeObjectURL(url);
				}
			};
			img.onerror = () => {
				URL.revokeObjectURL(url);
				reject(new Error('image decode failed'));
			};
			img.src = url;
		});
	}

	function cancelPreview() {
		// Restore the prior state (portrait or fallback) with NO request (state 5).
		mode = 'idle';
		previewDataUrl = '';
		errorMsg = '';
	}

	async function savePhoto() {
		if (mode !== 'previewing' || !previewDataUrl) return;
		mode = 'saving';
		errorMsg = '';
		try {
			const image_base64 = stripDataUrlPrefix(previewDataUrl);
			const res = await setPortrait(char, { image_base64 });
			onchanged({ has_portrait: true, updated_at: res.updated_at ?? '' });
			successMsg = `Saved ${char}'s photo.`;
			mode = 'idle';
			previewDataUrl = '';
		} catch (err) {
			handleWriteError(err, 'save');
			mode = 'previewing'; // keep the preview so the user can retry Save/Cancel
		}
	}

	function askRemove() {
		successMsg = '';
		errorMsg = '';
		confirmingRemove = true;
	}

	async function confirmRemove() {
		confirmingRemove = false;
		mode = 'saving';
		errorMsg = '';
		try {
			await removePortrait(char);
			onchanged({ has_portrait: false, updated_at: '' });
			successMsg = `Removed ${char}'s photo.`;
			mode = 'idle';
			previewDataUrl = '';
		} catch (err) {
			handleWriteError(err, 'remove');
			mode = 'idle';
		}
	}

	// Map a write failure to the UI-SPEC error copy (state 8). A 401/403 routes to the
	// AuthGate (mid-session gate collapse); a too_large/invalid_image maps to its copy;
	// anything else → the generic "no changes were made".
	function handleWriteError(err: unknown, _op: 'save' | 'remove') {
		if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) {
			authGuard(err);
			return;
		}
		const code =
			err && typeof err === 'object' && 'code' in err ? (err as { code?: string }).code : undefined;
		if (code === 'too_large') {
			errorMsg = 'That image is too large (max 256 KB). Try a smaller or more compressed image.';
		} else if (code === 'invalid_image') {
			errorMsg = "That file isn't a PNG, JPEG, or WebP. Choose an image in one of those formats.";
		} else {
			errorMsg = "Couldn't save the photo. No changes were made.";
		}
	}
</script>

<!-- The preview overlays into the frame ABOVE this control via the parent, but the
     downscaled preview is also shown inline here so the user sees exactly what saves. -->
<div class="portrait-control">
	{#if mode === 'previewing'}
		<!-- State 5: preview + confirm. The preview image is the downscaled square. -->
		<div class="preview-frame">
			<img class="preview-img" src={previewDataUrl} alt={`Preview of ${char}'s photo`} />
		</div>
		<div class="row">
			<button type="button" class="primary" onclick={savePhoto}>
				<Upload size={16} aria-hidden="true" />
				<span>Save photo</span>
			</button>
			<button type="button" class="ghost" onclick={cancelPreview}>
				<span>Cancel</span>
			</button>
		</div>
	{:else if mode === 'saving'}
		<!-- State 6: saving. Controls locked; spinner. -->
		<button type="button" class="primary" disabled>
			<LoaderCircle size={16} aria-hidden="true" class="spin" />
			<span>Uploading…</span>
		</button>
	{:else if hasPortrait}
		<!-- State 1': idle, portrait set → Change + Remove side by side. The Change
		     button IS the file-picker trigger (UI-SPEC state 2 label "Choose an image…"). -->
		<div class="row">
			<button type="button" class="primary" aria-label="Choose an image…" onclick={openPicker}>
				<ImageUp size={16} aria-hidden="true" />
				<span>Change photo</span>
			</button>
			<button type="button" class="destructive" onclick={askRemove}>
				<Trash2 size={16} aria-hidden="true" />
				<span>Remove photo</span>
			</button>
		</div>
	{:else}
		<!-- State 1: idle, no portrait → Add photo + the formats/size hint. The Add
		     button IS the file-picker trigger (UI-SPEC state 2 label "Choose an image…"). -->
		<button type="button" class="primary" aria-label="Choose an image…" onclick={openPicker}>
			<ImagePlus size={16} aria-hidden="true" />
			<span>Add photo</span>
		</button>
		<p class="hint">PNG, JPEG, or WebP, up to 256 KB. Square images look best.</p>
	{/if}

	<!-- The hidden native file input (state 2). accept excludes SVG/GIF (D-04). -->
	<input
		bind:this={fileInput}
		class="file-input"
		type="file"
		accept="image/png,image/jpeg,image/webp"
		aria-label="Choose a portrait image"
		onchange={onPick}
	/>

	{#if successMsg}
		<p class="result success" aria-live="polite">{successMsg}</p>
	{/if}
	{#if errorMsg}
		<p class="result error" aria-live="polite">{errorMsg}</p>
	{/if}
</div>

<!-- Remove confirmation (state: destructive-confirm before the DELETE, D-08). The
     "Remove photo:" heading + body copy are verbatim from the UI-SPEC. -->
<ConfirmDialog
	open={confirmingRemove}
	heading="Remove photo"
	body={`Remove ${char}'s photo? This can't be undone, but you can add a new one anytime.`}
	confirmLabel="Remove photo"
	onConfirm={confirmRemove}
	onCancel={() => (confirmingRemove = false)}
/>

<style>
	.portrait-control {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px; /* sm */
		width: min(190px, 100%);
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: center;
	}
	/* The visually-hidden native file input (triggered by the button, state 2). */
	.file-input {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
	/* Inline preview of the downscaled square (state 5). */
	.preview-frame {
		width: min(190px, 100%);
		aspect-ratio: 1 / 1;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		overflow: hidden;
	}
	.preview-img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	/* Primary button = accent fill / --bg text (mirrors CharMetaForm .primary). */
	.primary {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px; /* touch target */
		padding: 8px 16px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px; /* Label */
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
	/* Ghost (Cancel) = neutral bordered button (mirrors ConfirmDialog .cd-cancel). */
	.ghost {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px;
		padding: 8px 16px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text);
		background: none;
		border: 1px solid var(--border, var(--accent));
		border-radius: 4px;
		cursor: pointer;
	}
	.ghost:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	/* Destructive (Remove) = --destructive fill / --bg text (mirrors ConfirmDialog .cd-confirm). */
	.destructive {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 44px;
		padding: 8px 16px;
		font-family: var(--font-display);
		font-weight: var(--weight-display);
		font-size: 13px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--bg);
		background: var(--destructive);
		border: 1px solid var(--destructive);
		border-radius: 4px;
		cursor: pointer;
	}
	.destructive:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	/* Heavy contrast caveat (UI-SPEC § Color, mirrors ConfirmDialog): Heavy's --destructive
	   is dark (#6b1a1a), so --bg (also dark) text would be dark-on-dark — use accent text. */
	:global([data-theme='heavy']) .destructive {
		color: var(--accent);
	}
	.hint {
		font-family: var(--font-body);
		font-size: 13px;
		line-height: 1.4;
		opacity: 0.75;
		text-align: center;
		margin: 0;
		max-width: 190px;
	}
	.result {
		font-family: var(--font-body);
		font-size: 16px;
		line-height: 1.4;
		text-align: center;
		margin: 0;
	}
	.result.success {
		color: var(--status-ok);
	}
	.result.error {
		color: var(--status-missing);
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
