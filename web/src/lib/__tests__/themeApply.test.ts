// Vitest for the Plan 14-04 theme-apply WIRING (the SiteShell/ThemePicker
// contract): applyTheme(key, root) must write the single [data-theme] attribute
// on the shell root AND persist the choice to localStorage, and loadTheme()
// must resolve the velious default with no stored pref (the regression guard
// for the WEB-05 velious-default + persistence behavior — D-06).
//
// Node-environment test (no jsdom in the web/ test infra): applyTheme takes the
// root element as a parameter, so a minimal fake element + a stubbed
// localStorage exercise the wiring without a DOM. This is the new behavior
// Plan 14-04 adds on top of 14-02's loadTheme/saveTheme/resolveTheme.

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { applyTheme, loadTheme, DEFAULT_THEME } from '../theme/themes';

// A minimal stand-in for the shell root element: applyTheme should set
// `dataset.theme` (the [data-theme] attribute) on it.
function fakeRoot(): { dataset: Record<string, string> } {
	return { dataset: {} };
}

// In-memory localStorage stub (the web/ tests run in node — no real Storage).
function stubLocalStorage(initial: Record<string, string> = {}) {
	const store = new Map<string, string>(Object.entries(initial));
	const ls = {
		getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
		setItem: (k: string, v: string) => void store.set(k, v),
		removeItem: (k: string) => void store.delete(k),
		clear: () => store.clear(),
		key: (i: number) => Array.from(store.keys())[i] ?? null,
		get length() {
			return store.size;
		}
	};
	vi.stubGlobal('localStorage', ls);
	return store;
}

describe('applyTheme wiring (Plan 14-04)', () => {
	beforeEach(() => {
		vi.unstubAllGlobals();
	});

	it('writes the [data-theme] attribute on the shell root', () => {
		stubLocalStorage();
		const root = fakeRoot();
		applyTheme('kunark', root as unknown as HTMLElement);
		expect(root.dataset.theme).toBe('kunark');
	});

	it('persists the chosen theme to localStorage', () => {
		const store = stubLocalStorage();
		const root = fakeRoot();
		applyTheme('heavy', root as unknown as HTMLElement);
		expect(store.get('theme')).toBe('heavy');
	});

	it('coerces an invalid key to the velious default on both attribute and storage', () => {
		const store = stubLocalStorage();
		const root = fakeRoot();
		applyTheme('not-a-real-theme' as never, root as unknown as HTMLElement);
		expect(root.dataset.theme).toBe('velious');
		expect(store.get('theme')).toBe('velious');
	});

	it('is a no-op-safe call when root is null (SSR / pre-mount)', () => {
		stubLocalStorage();
		expect(() => applyTheme('velious', null)).not.toThrow();
	});
});

describe('loadTheme default (WEB-05 regression guard)', () => {
	beforeEach(() => {
		vi.unstubAllGlobals();
	});

	it('returns velious with no stored preference', () => {
		stubLocalStorage();
		expect(loadTheme()).toBe(DEFAULT_THEME);
		expect(loadTheme()).toBe('velious');
	});

	it('returns the stored key when valid', () => {
		stubLocalStorage({ theme: 'kunark' });
		expect(loadTheme()).toBe('kunark');
	});
});
