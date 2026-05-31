// Vitest for the pure officer-management helpers (Plan 15-05 Task 3, D-08).
// Proves the owner-floor Remove-suppression + the idempotent result copy that
// AdminMgmtForm renders — node-only, no DOM (15-04's pure-logic-in-.ts
// philosophy). The form .svelte wires these to the live API + ConfirmDialog.

import { describe, it, expect } from 'vitest';
import type { Officer } from '../api';
import {
	showRemoveButton,
	addResultMessage,
	removeResultMessage,
	ADMIN_ERROR_COPY
} from '../admin';

function officer(over: Partial<Officer> = {}): Officer {
	return { discord_user_id: '100', username: 'Alice', avatar: null, is_floor: false, ...over };
}

describe('showRemoveButton — v1 showRemove rule (D-08 owner-floor suppression)', () => {
	it('shows Remove for a non-floor officer regardless of caller', () => {
		expect(showRemoveButton(officer({ is_floor: false }), '999')).toBe(true);
		expect(showRemoveButton(officer({ is_floor: false }), '')).toBe(true);
	});

	it('suppresses Remove on the floor row for a PEER (caller != floor)', () => {
		const floor = officer({ discord_user_id: 'floor1', is_floor: true });
		expect(showRemoveButton(floor, 'peer2')).toBe(false);
	});

	it('shows Remove on the floor row only for the floor THEMSELVES (self-removal allowed)', () => {
		const floor = officer({ discord_user_id: 'floor1', is_floor: true });
		expect(showRemoveButton(floor, 'floor1')).toBe(true);
	});

	it('suppresses Remove on the floor row when the caller id is unknown (safety; server is the gate)', () => {
		const floor = officer({ discord_user_id: 'floor1', is_floor: true });
		expect(showRemoveButton(floor, '')).toBe(false);
	});
});

describe('result copy — idempotent add/remove (15-UI-SPEC verbatim)', () => {
	it('addResultMessage: added vs already-an-officer', () => {
		expect(addResultMessage({ added: true, username: 'Bob' })).toBe('Officer added: Bob.');
		expect(addResultMessage({ added: false, username: 'Bob' })).toBe('Already an officer: Bob.');
	});

	it('removeResultMessage: removed vs not-in-the-list', () => {
		expect(removeResultMessage({ removed: true, username: 'Cara' })).toBe('Officer removed: Cara.');
		expect(removeResultMessage({ removed: false, username: 'Cara' })).toBe('Not in the list: Cara.');
	});
});

describe('ADMIN_ERROR_COPY — the two inline (non-collapsing) error routes', () => {
	it('owner-floor + lock-busy carry the exact UI-SPEC copy', () => {
		expect(ADMIN_ERROR_COPY['owner-floor']).toBe(
			'Owner-floor protected — only the maintainer can remove themselves. No changes were written.'
		);
		expect(ADMIN_ERROR_COPY['lock-busy']).toBe(
			'Another officer action is in flight. Please retry. No changes were written.'
		);
	});
});
