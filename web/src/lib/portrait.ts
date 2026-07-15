// Pure, DOM-light client-side portrait helpers (Phase 41 / CHARUI-02, D-04). These
// are the node-testable rules for the portrait upload control — the byte cap + the
// PNG/JPEG/WebP allow-list + the data-URL prefix strip. They are UX defense-in-depth
// ONLY: the Go server re-sniffs the magic bytes and re-enforces the 256 KB cap and is
// authoritative (41-01, T-41-01/T-41-03). The canvas downscale itself stays in the
// .svelte component (it needs a real DOM <canvas>/Image); this module holds only the
// pure rules so the repo's node-only vitest (no jsdom) can cover them.

/** The decoded-byte cap for a stored portrait (matches the Go server's 256 KB cap). */
export const MAX_PORTRAIT_BYTES = 256 * 1024;

/** The allowed portrait MIME types. SVG is EXCLUDED on purpose (it can carry script —
 *  an XSS vector, D-04); GIF is excluded (animated/out of scope). PNG/JPEG/WebP only. */
export const ALLOWED_PORTRAIT_TYPES = ['image/png', 'image/jpeg', 'image/webp'] as const;

export type ImageValidation = { ok: true } | { ok: false; reason: 'type' | 'size' };

/** Client-side defense-in-depth (the server re-sniffs + re-caps and is authoritative).
 *  Rejects SVG/GIF/anything-not-allowed by MIME type, and files over the byte cap. */
export function validateImageFile(file: { type: string; size: number }): ImageValidation {
	if (!ALLOWED_PORTRAIT_TYPES.includes(file.type as (typeof ALLOWED_PORTRAIT_TYPES)[number])) {
		return { ok: false, reason: 'type' };
	}
	if (file.size > MAX_PORTRAIT_BYTES) {
		return { ok: false, reason: 'size' };
	}
	return { ok: true };
}

/** Strip a `data:*;base64,` prefix so only the raw base64 payload goes in the POST body.
 *  A data URL with no comma (already raw base64) passes through unchanged. */
export function stripDataUrlPrefix(dataUrl: string): string {
	const i = dataUrl.indexOf(',');
	return i >= 0 ? dataUrl.slice(i + 1) : dataUrl;
}
