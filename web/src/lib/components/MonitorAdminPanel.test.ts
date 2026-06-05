// Vitest for the MonitorAdminPanel pure decision helpers (20-05 Task 1) — the
// node test project (no jsdom; the Toggle/ConfirmDialog split). Two pure cores:
// the test-alert response→feedback map (the three states) and the add-channel
// validity predicate. Exercising them here proves the WANT-08/D-10 logic without
// mounting the component (web/ vitest is DOM-blind — the browser-smoke covers the
// live render).

import { describe, it, expect } from 'vitest';
import {
	testAlertFeedback,
	addChannelValid,
	TEST_ALERT_SENT,
	TEST_ALERT_DM_BLOCKED,
	TEST_ALERT_BOT_DOWN
} from './MonitorAdminPanel.svelte';

describe('testAlertFeedback (the three D-10 feedback states)', () => {
	it('maps status:"sent" to the ok line', () => {
		expect(testAlertFeedback({ status: 'sent' })).toEqual({
			kind: 'ok',
			message: TEST_ALERT_SENT
		});
	});

	it('maps error:"dm_blocked" to the blocked (50007) line', () => {
		expect(testAlertFeedback({ error: 'dm_blocked' })).toEqual({
			kind: 'blocked',
			message: TEST_ALERT_DM_BLOCKED
		});
	});

	it('maps error:"bot_unavailable" to the bot-down line', () => {
		expect(testAlertFeedback({ error: 'bot_unavailable' })).toEqual({
			kind: 'down',
			message: TEST_ALERT_BOT_DOWN
		});
	});

	it('maps an unknown/empty response shape to the bot-down line (defensive default)', () => {
		expect(testAlertFeedback({}).kind).toBe('down');
		expect(testAlertFeedback({ error: 'something_else' }).kind).toBe('down');
	});

	it('the three messages are the exact UI-SPEC copy and distinct', () => {
		const msgs = [TEST_ALERT_SENT, TEST_ALERT_DM_BLOCKED, TEST_ALERT_BOT_DOWN];
		expect(new Set(msgs).size).toBe(3);
		expect(TEST_ALERT_DM_BLOCKED).toContain('blocking messages');
		expect(TEST_ALERT_BOT_DOWN).toContain('may be offline');
	});
});

describe('addChannelValid (the add-channel submit predicate)', () => {
	it('is true for a non-blank label, a numeric channel id, and a set monitor', () => {
		expect(addChannelValid('Raid Alliance — Red', '123456789012345678', 'wts')).toBe(true);
	});

	it('is false when the label is blank or whitespace-only', () => {
		expect(addChannelValid('', '123', 'wts')).toBe(false);
		expect(addChannelValid('   ', '123', 'wts')).toBe(false);
	});

	it('is false when the channel id is empty or non-numeric', () => {
		expect(addChannelValid('Label', '', 'wts')).toBe(false);
		expect(addChannelValid('Label', '12ab34', 'wts')).toBe(false);
		expect(addChannelValid('Label', 'abc', 'wts')).toBe(false);
	});

	it('is false when no monitor is selected', () => {
		expect(addChannelValid('Label', '123', '')).toBe(false);
	});

	it('trims surrounding whitespace on the label and channel id before validating', () => {
		expect(addChannelValid('  Label  ', '  123  ', 'ec_auction')).toBe(true);
	});
});
