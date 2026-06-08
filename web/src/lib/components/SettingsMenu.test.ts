// Vitest for the header SettingsMenu gear dropdown (260607-sdh).
//
// The repo runs vitest under NODE with no jsdom and no @testing-library/svelte
// (the established philosophy — see ConfirmDialog.test.ts / auth.test.ts), and
// the vitest config EXCLUDES *.svelte.test.ts. So this proves the a11y +
// security contract two node-runnable ways:
//   1. The dismiss decision + avatar-URL derivation are pure exported helpers
//      (menuKeyAction / avatarUrlFor) exercised directly.
//   2. The rendered-markup contract (gear aria-haspopup/expanded/controls, the
//      role="menu" panel, the officer-gated /admin item, the sign-out flow,
//      username escaped via {} never {@html}, Escape restoring focus to the
//      trigger) is asserted by inspecting the component source.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { menuKeyAction, avatarUrlFor } from './SettingsMenu.svelte';
import type { Session } from '$lib/auth';

const SOURCE = readFileSync(
	fileURLToPath(new URL('./SettingsMenu.svelte', import.meta.url)),
	'utf8'
);

function makeSession(over: Partial<Session> = {}): Session {
	return {
		authenticated: true,
		isMember: true,
		isOfficer: false,
		username: 'Slampeach',
		avatar: 'abc123',
		discordUserId: '42',
		...over
	};
}

describe('SettingsMenu menuKeyAction (Escape-to-close contract)', () => {
	it('Escape while open → close', () => {
		expect(menuKeyAction('Escape', true)).toBe('close');
	});

	it('Escape while closed → ignore (no-op)', () => {
		expect(menuKeyAction('Escape', false)).toBe('ignore');
	});

	it('Enter while open → ignore', () => {
		expect(menuKeyAction('Enter', true)).toBe('ignore');
	});

	it('Tab while open → ignore', () => {
		expect(menuKeyAction('Tab', true)).toBe('ignore');
	});
});

describe('SettingsMenu avatarUrlFor (Discord CDN derivation)', () => {
	it('returns the full CDN URL when both id + avatar hash are present', () => {
		expect(avatarUrlFor(makeSession())).toBe(
			'https://cdn.discordapp.com/avatars/42/abc123.png'
		);
	});

	it('returns null when the avatar hash is null', () => {
		expect(avatarUrlFor(makeSession({ avatar: null }))).toBeNull();
	});

	it('returns null when the discord user id is empty', () => {
		expect(avatarUrlFor(makeSession({ discordUserId: '' }))).toBeNull();
	});

	it('returns null for a null/undefined session', () => {
		expect(avatarUrlFor(null)).toBeNull();
		expect(avatarUrlFor(undefined)).toBeNull();
	});
});

describe('SettingsMenu rendered-markup a11y + security contract', () => {
	it('the gear trigger advertises a menu popup (aria-haspopup="menu")', () => {
		expect(SOURCE).toContain('aria-haspopup="menu"');
	});

	it('the gear trigger has aria-expanded bound to open', () => {
		expect(SOURCE).toContain('aria-expanded={open}');
	});

	it('the gear trigger controls the panel via aria-controls', () => {
		expect(SOURCE).toContain('aria-controls="settings-menu-panel"');
		expect(SOURCE).toContain('id="settings-menu-panel"');
	});

	it('the dropdown panel is role="menu"', () => {
		expect(SOURCE).toContain('role="menu"');
	});

	it('the /admin item is officer-gated (`{#if session?.isOfficer}` guards it)', () => {
		expect(SOURCE).toMatch(/\{#if session\?\.isOfficer\}[\s\S]*href="\/admin"/);
	});

	it('Sign out calls logout() then navigates to `/`', () => {
		expect(SOURCE).toContain('await logout();');
		expect(SOURCE).toContain("window.location.href = '/';");
	});

	it('Escape closes the menu and restores focus to the trigger', () => {
		expect(SOURCE).toContain('menuKeyAction(e.key, open)');
		expect(SOURCE).toContain('triggerEl?.focus()');
	});

	it('the trigger is a bound ref (bind:this={triggerEl}) so focus can return to it', () => {
		expect(SOURCE).toContain('bind:this={triggerEl}');
	});

	it('the username renders via plain {} interpolation — never {@html} (T-15-22)', () => {
		expect(SOURCE).toContain('{session.username}');
		// The Svelte raw-HTML directive must not appear anywhere (match the actual
		// `{@html` opener, not the bare substring used in explanatory comments).
		expect(SOURCE).not.toContain('{@html');
	});

	it('the theme is passed straight through to ThemePicker (bind:theme chain intact)', () => {
		expect(SOURCE).toContain('<ThemePicker bind:theme />');
	});

	it('outside-click and route-change close the menu', () => {
		expect(SOURCE).toContain("document.addEventListener('pointerdown'");
		expect(SOURCE).toContain('$page.url?.pathname');
	});
});
