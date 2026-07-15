// Vitest for the pure portrait client-validation helpers (Phase 41 / CHARUI-02, D-04).
// This is the node-vitest coverage for the phase's web wave — the PortraitControl
// .svelte is a thin renderer over these rules + the (DOM-bound) canvas downscale, so
// the file-pick/downscale/preview/upload round-trip is a deploy-then-browser-smoke
// checkpoint (the repo runs vitest under node with NO jsdom; it cannot mount the
// component). These pure rules are the testable half: the byte cap, the PNG/JPEG/WebP
// allow-list (SVG/GIF excluded), and the data-URL prefix strip.
//
// The rules are UX defense-in-depth only — the Go server re-sniffs the magic bytes +
// re-enforces the 256 KB cap and is authoritative (41-01). These tests lock the client
// contract that maps a rejection to the UI-SPEC error copy BEFORE any request.

import { describe, it, expect } from 'vitest';
import {
	MAX_PORTRAIT_BYTES,
	ALLOWED_PORTRAIT_TYPES,
	validateImageFile,
	stripDataUrlPrefix
} from '../portrait';

describe('MAX_PORTRAIT_BYTES — the 256 KB decoded-byte cap (mirrors the Go server)', () => {
	it('is exactly 256 * 1024', () => {
		expect(MAX_PORTRAIT_BYTES).toBe(256 * 1024);
	});
});

describe('ALLOWED_PORTRAIT_TYPES — the PNG/JPEG/WebP allow-list (SVG/GIF excluded)', () => {
	it('is exactly png, jpeg, webp — no svg, no gif', () => {
		expect(ALLOWED_PORTRAIT_TYPES).toEqual(['image/png', 'image/jpeg', 'image/webp']);
		expect(ALLOWED_PORTRAIT_TYPES).not.toContain('image/svg+xml');
		expect(ALLOWED_PORTRAIT_TYPES).not.toContain('image/gif');
	});
});

describe('validateImageFile — type + size gate (client defense-in-depth)', () => {
	it('rejects an SVG (image/svg+xml) as reason:type (XSS vector, D-04)', () => {
		expect(validateImageFile({ type: 'image/svg+xml', size: 1000 })).toEqual({
			ok: false,
			reason: 'type'
		});
	});
	it('rejects a GIF (image/gif) as reason:type (animated / out of scope)', () => {
		expect(validateImageFile({ type: 'image/gif', size: 1000 })).toEqual({
			ok: false,
			reason: 'type'
		});
	});
	it('accepts PNG, JPEG, and WebP under the cap', () => {
		expect(validateImageFile({ type: 'image/png', size: 1000 })).toEqual({ ok: true });
		expect(validateImageFile({ type: 'image/jpeg', size: 1000 })).toEqual({ ok: true });
		expect(validateImageFile({ type: 'image/webp', size: 1000 })).toEqual({ ok: true });
	});
	it('accepts a file exactly at the cap (boundary — <= is ok)', () => {
		expect(validateImageFile({ type: 'image/png', size: MAX_PORTRAIT_BYTES })).toEqual({ ok: true });
	});
	it('rejects a file one byte over the cap as reason:size', () => {
		expect(validateImageFile({ type: 'image/png', size: MAX_PORTRAIT_BYTES + 1 })).toEqual({
			ok: false,
			reason: 'size'
		});
	});
	it('checks type BEFORE size (a wrong-type oversize file reports type, not size)', () => {
		// A wrong-type file over the cap surfaces the type problem first — the picker's
		// accept="" already excludes it, but the validator's ordering is asserted here.
		expect(validateImageFile({ type: 'image/gif', size: MAX_PORTRAIT_BYTES + 1 })).toEqual({
			ok: false,
			reason: 'type'
		});
	});
});

describe('stripDataUrlPrefix — the raw base64 for the POST body', () => {
	it('strips a data:*;base64, prefix, yielding only the payload', () => {
		expect(stripDataUrlPrefix('data:image/png;base64,AAECAw==')).toBe('AAECAw==');
		expect(stripDataUrlPrefix('data:image/jpeg;base64,/9j/4AAQ')).toBe('/9j/4AAQ');
	});
	it('passes through an already-raw base64 string (no comma) unchanged', () => {
		expect(stripDataUrlPrefix('AAECAw==')).toBe('AAECAw==');
	});
});
