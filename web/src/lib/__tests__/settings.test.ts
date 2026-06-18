// Vitest source-grep guards for the new Settings IA (Phase 30 Plan 02, NAV-03 /
// D-05/D-06). The repo runs vitest with NO jsdom / @testing-library — it cannot mount
// a .svelte component (web-tests-node-only-blind-to-dom). So, mirroring charmeta.test.ts
// (Style B, the eviction.test.ts pattern), we read the route + bridge source as strings
// and assert the contract the DOM-blind suite CAN enforce:
//
//   - /settings mounts the 4 member panels as sections (WatcherCodesPanel, CharMetaForm,
//     MyCharactersPanel, SettingsThemePicker);
//   - the 4 admin panels sit AFTER an `{#if isOfficer}` gate (the section-level officer
//     gate is preserved — T-30-05, Layer-1 UX over the server boundary);
//   - the page reads the session via getContext + SESSION_KEY (the two-layer pattern,
//     never a hand-rolled permission check);
//   - the five stable settings-* section ids exist (D-02 deep-link landing + the search);
//   - NO {@html} (the XSS guard — T-30-06);
//   - SettingsThemePicker bridges via getContext<ThemeContext>(THEME_KEY) and contains NO
//     theme-apply call (single-writer invariant — T-30-07; the lone writer stays +layout).
//
// These are SOURCE guards, not behavior — the deployed browser-smoke (PLAN Task 4) is the
// load-bearing verification (it proves the gate actually hides, the search filters, the
// theme actually re-skins). A regression in the IA still fails CI here even DOM-blind.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const SETTINGS_PAGE_SOURCE = readFileSync(
	fileURLToPath(new URL('../../routes/settings/+page.svelte', import.meta.url)),
	'utf8'
);
const THEME_PICKER_BRIDGE_SOURCE = readFileSync(
	fileURLToPath(new URL('../components/SettingsThemePicker.svelte', import.meta.url)),
	'utf8'
);

describe('/settings — composes the member panels as in-page sections (NAV-03 / D-05)', () => {
	it('mounts the 4 member panels (Theme bridge + Watcher codes + Class&Level + My characters)', () => {
		expect(SETTINGS_PAGE_SOURCE).toContain('SettingsThemePicker');
		expect(SETTINGS_PAGE_SOURCE).toContain('WatcherCodesPanel');
		expect(SETTINGS_PAGE_SOURCE).toContain('CharMetaForm');
		expect(SETTINGS_PAGE_SOURCE).toContain('MyCharactersPanel');
	});

	it('carries the five stable section ids (D-02 deep-link landing + the settings search)', () => {
		for (const id of [
			'settings-theme',
			'settings-watcher-codes',
			'settings-class-level',
			'settings-my-characters',
			'settings-admin'
		]) {
			expect(SETTINGS_PAGE_SOURCE).toContain(id);
		}
	});

	it('has the live settings search (placeholder) + the dimmed empty-match copy', () => {
		expect(SETTINGS_PAGE_SOURCE).toContain('Search settings');
		expect(SETTINGS_PAGE_SOURCE).toContain('No settings match');
	});
});

describe('/settings — the section-level officer gate (T-30-05, Layer-1 UX)', () => {
	it('mounts the 4 admin panels AFTER an {#if isOfficer} gate (the gate wraps the section)', () => {
		const gateIdx = SETTINGS_PAGE_SOURCE.indexOf('{#if isOfficer}');
		expect(gateIdx).toBeGreaterThan(-1);
		// Every admin panel's MOUNT (`<Panel`) appears strictly after the gate token in
		// source order — so a non-officer never renders them. (Match the `<` mount form,
		// not the bare name, which also appears in the top-of-file imports BEFORE the gate.)
		for (const panel of [
			'<EvictionForm',
			'<AdminMgmtForm',
			'<MonitorAdminPanel',
			'<AssignmentAdminPanel'
		]) {
			expect(SETTINGS_PAGE_SOURCE.indexOf(panel)).toBeGreaterThan(gateIdx);
		}
	});

	it('reads the session via getContext + SESSION_KEY (the server-truth two-layer pattern)', () => {
		// Never a hand-rolled permission check — the officer bit comes from the shared
		// session context; the Go API re-checks on every admin write (the real boundary).
		expect(SETTINGS_PAGE_SOURCE).toContain('getContext');
		expect(SETTINGS_PAGE_SOURCE).toContain('SESSION_KEY');
		expect(SETTINGS_PAGE_SOURCE).toContain('isOfficer');
	});

	it('renders no raw HTML (the XSS guard — T-30-06)', () => {
		expect(SETTINGS_PAGE_SOURCE).not.toContain('{@html');
	});
});

describe('SettingsThemePicker — the context→bindable bridge (single-writer invariant, T-30-07)', () => {
	it('reads the theme via getContext<ThemeContext>(THEME_KEY) and binds it to ThemePicker', () => {
		expect(THEME_PICKER_BRIDGE_SOURCE).toContain('getContext<ThemeContext>(THEME_KEY)');
		expect(THEME_PICKER_BRIDGE_SOURCE).toContain('<ThemePicker bind:theme');
	});

	it('does NOT apply the theme itself — the single [data-theme] writer stays in +layout', () => {
		// A second writer could drift the attribute; the bridge only mutates +layout's
		// `theme` $state through the context setter.
		expect(THEME_PICKER_BRIDGE_SOURCE).not.toContain('applyTheme');
	});
});
