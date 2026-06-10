// Vitest for NotificationRow's DOM-free presentation logic (20-04 Task 2). The
// repo runs vitest node-only — so we test the EXTRACTED pure functions, NOT a
// mounted component. Two P15-class crashers are pinned here:
//   1. deliveryBadge — the send_status → WORD mapping (the load-bearing delivery
//      encoding; the word is the non-color signal).
//   2. relativeTime — sent_at is unix EPOCH SECONDS; a seconds-as-ms mistake
//      renders "57 years ago" (the P15 epoch-sec date crasher). This asserts the
//      helper treats the input as seconds (the ×1000 happens exactly once).

import { describe, it, expect } from 'vitest';
import { deliveryBadge, relativeTime, absoluteTime, sourceLabel } from './NotificationRow.svelte';

describe('deliveryBadge — send_status → word + token (color is never the only signal)', () => {
	it("maps 'sent' → DELIVERED / status-ok / not blocked", () => {
		const b = deliveryBadge('sent');
		expect(b.word).toBe('DELIVERED');
		expect(b.token).toBe('var(--status-ok)');
		expect(b.blocked).toBe(false);
	});

	it("maps 'dm_blocked' → CAN'T DM / status-missing / blocked (the safety-net row)", () => {
		const b = deliveryBadge('dm_blocked');
		expect(b.word).toBe("CAN'T DM");
		expect(b.token).toBe('var(--status-missing)');
		expect(b.blocked).toBe(true);
	});

	it("maps 'error' → NOT SENT / status-other / not blocked (no raw enum word)", () => {
		const b = deliveryBadge('error');
		expect(b.word).toBe('NOT SENT');
		expect(b.token).toBe('var(--status-other)');
		expect(b.blocked).toBe(false);
	});

	it('degrades an unknown status to NOT SENT rather than a blank badge', () => {
		const b = deliveryBadge('totally-unexpected');
		expect(b.word).toBe('NOT SENT');
		expect(b.blocked).toBe(false);
	});
});

describe('sourceLabel — friendly fallback for a detail-less row (never the raw source enum)', () => {
	it('maps the three known sources to member-friendly labels', () => {
		expect(sourceLabel('ec_auction')).toBe('EC auction alert');
		expect(sourceLabel('wts')).toBe('WTS alert');
		expect(sourceLabel('raid_target')).toBe('Raid target alert');
	});

	it('degrades an unknown source to the generic SquireBot label', () => {
		expect(sourceLabel('some_future_source')).toBe('SquireBot alert');
		expect(sourceLabel('')).toBe('SquireBot alert');
	});
});

describe('relativeTime — sent_at is EPOCH SECONDS, not ms (the P15 crasher)', () => {
	// A fixed "now": 2026-06-05T16:00:00Z in epoch SECONDS = 1780675200.
	const NOW_SEC = 1_780_675_200;
	const NOW_MS = NOW_SEC * 1000;

	it("reads 'just now' for the current second", () => {
		expect(relativeTime(NOW_SEC, NOW_MS)).toBe('just now');
	});

	it("reads '2 hours ago' for a timestamp 2h earlier (treated as SECONDS)", () => {
		const twoHoursAgo = NOW_SEC - 2 * 60 * 60;
		expect(relativeTime(twoHoursAgo, NOW_MS)).toBe('2 hours ago');
	});

	it("reads 'yesterday' for ~1 day earlier", () => {
		const oneDayAgo = NOW_SEC - 25 * 60 * 60;
		expect(relativeTime(oneDayAgo, NOW_MS)).toBe('yesterday');
	});

	it('does NOT render a "57 years ago" date from a seconds-as-ms mistake', () => {
		// A realistic recent epoch-SECONDS value (~2026). If the helper wrongly fed it
		// to Date() as ms it would land in 1970 → "57 years ago". The seconds-aware
		// formatter must produce a sane recent string instead.
		const out = relativeTime(NOW_SEC - 30 * 60, NOW_MS); // 30 min ago
		expect(out).toBe('30 min ago');
		expect(out).not.toMatch(/year/);
	});

	it("degrades a non-finite timestamp to '' (no 'Invalid Date')", () => {
		expect(relativeTime(NaN, NOW_MS)).toBe('');
		expect(absoluteTime(NaN)).toBe('');
	});

	it('absoluteTime builds the date from seconds×1000 (a 2026 year, not 1970)', () => {
		expect(absoluteTime(NOW_SEC)).toMatch(/2026/);
	});
});
