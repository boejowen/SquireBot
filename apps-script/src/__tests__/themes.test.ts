import { describe, it, expect, beforeEach } from 'vitest';
import {
  THEME_KEYS, THEMES, DEFAULT_THEME, isThemeKey,
  getActiveTheme, setTheme,
} from '../lib/themes';
import { resetMocks, seedMeta, type MockState } from './test-helpers';

describe('themes registry', () => {
  it('every ThemeKey is a key in THEMES', () => {
    for (const k of THEME_KEYS) {
      expect(k in THEMES).toBe(true);
    }
  });

  it('sheets-default is the null sentinel', () => {
    expect(THEMES['sheets-default']).toBeNull();
  });

  it('all other themes have non-null Theme objects with required fields', () => {
    for (const k of THEME_KEYS) {
      if (k === 'sheets-default') continue;
      const t = THEMES[k]!;
      expect(t.headerBg).toMatch(/^#[0-9a-f]{6}$/i);
      expect(t.headerFg).toMatch(/^#[0-9a-f]{6}$/i);
      expect(t.fontFamily).toBeTruthy();
    }
  });

  it('isThemeKey accepts known keys and rejects unknowns', () => {
    expect(isThemeKey('vanilla')).toBe(true);
    expect(isThemeKey('sheets-default')).toBe(true);
    expect(isThemeKey('nonsense')).toBe(false);
    expect(isThemeKey('')).toBe(false);
    expect(isThemeKey(null)).toBe(false);
    expect(isThemeKey(123)).toBe(false);
  });
});

describe('getActiveTheme', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
  });

  it('returns DEFAULT_THEME when _meta.theme row is absent', () => {
    seedMeta(state, [['schema_version', '2'], ['canonical_id', 'x']]);
    expect(getActiveTheme()).toBe(DEFAULT_THEME);
    expect(DEFAULT_THEME).toBe('minimalist');
  });

  it('reads _meta.theme value when present', () => {
    seedMeta(state, [['schema_version', '2'], ['theme', 'velious']]);
    expect(getActiveTheme()).toBe('velious');
  });

  it('falls back to DEFAULT_THEME when _meta.theme value is invalid', () => {
    seedMeta(state, [['schema_version', '2'], ['theme', 'gibberish']]);
    expect(getActiveTheme()).toBe(DEFAULT_THEME);
  });
});

describe('setTheme', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '2']]);
  });

  it('writes _meta.theme on a known key', () => {
    setTheme('kunark');
    const meta = state.sheets.get('_meta')!;
    const row = meta.values.find((r) => r[0] === 'theme');
    expect(row?.[1]).toBe('kunark');
  });

  it('throws on unknown key', () => {
    // Cast through unknown to bypass the type guard (simulates a bad
    // call from JS-side onClick handlers in the picker modal).
    expect(() => setTheme('not-a-real-theme' as unknown as 'vanilla')).toThrow(/unknown theme/);
  });

  it('updates an existing theme row in place', () => {
    seedMeta(state, [['schema_version', '2'], ['theme', 'minimalist']]);
    setTheme('heavy');
    const meta = state.sheets.get('_meta')!;
    const themeRows = meta.values.filter((r) => r[0] === 'theme');
    expect(themeRows.length).toBe(1);
    expect(themeRows[0][1]).toBe('heavy');
  });
});
