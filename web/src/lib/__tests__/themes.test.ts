// Vitest for the ported theme registry (Phase 14 Plan 14-02 Task 4).
//
// The v1 themes.test.ts is Sheet-applyTheme-coupled and does NOT port; this
// suite asserts the CSS-custom-property registry shape instead (WEB-05 / D-06).

import { describe, it, expect } from 'vitest';
import {
  THEME_KEYS,
  DEFAULT_THEME,
  THEMES,
  resolveTheme,
  type ThemeKey,
  type ThemeTokens,
} from '../theme/themes';

const REQUIRED_TOKENS: (keyof ThemeTokens)[] = [
  'bg',
  'panel',
  'text',
  'accent',
  'statusOk',
  'statusMissing',
  'statusOther',
  'flagNodrop',
  'flagLore',
  'flagMagic',
  'fontDisplay',
  'fontBody',
  'weightDisplay',
];

describe('THEME_KEYS', () => {
  it('has exactly 5 themed keys', () => {
    expect(THEME_KEYS.length).toBe(5);
  });

  it('excludes the dropped sheets-default sentinel (D-06)', () => {
    expect(THEME_KEYS as string[]).not.toContain('sheets-default');
  });

  it('contains the expected 5 keys', () => {
    expect([...THEME_KEYS].sort()).toEqual(
      ['heavy', 'kunark', 'minimalist', 'vanilla', 'velious'].sort(),
    );
  });
});

describe('DEFAULT_THEME', () => {
  it('is velious (D-06)', () => {
    expect(DEFAULT_THEME).toBe('velious');
  });
});

describe('THEMES registry', () => {
  it('defines all required tokens for every theme', () => {
    for (const key of THEME_KEYS) {
      const tokens = THEMES[key];
      for (const field of REQUIRED_TOKENS) {
        expect(tokens[field], `${key}.${field}`).toBeDefined();
      }
    }
  });

  it('uses --weight-display 400 for minimalist and 600 for the others', () => {
    expect(THEMES.minimalist.weightDisplay).toBe(400);
    expect(THEMES.velious.weightDisplay).toBe(600);
    expect(THEMES.vanilla.weightDisplay).toBe(600);
    expect(THEMES.kunark.weightDisplay).toBe(600);
    expect(THEMES.heavy.weightDisplay).toBe(600);
  });

  it('matches the UI-SPEC catalog spot-checks (velious tokens)', () => {
    expect(THEMES.velious.bg).toBe('#060b18');
    expect(THEMES.velious.panel).toBe('#0f1729');
    expect(THEMES.velious.accent).toBe('#a8c5e0');
    expect(THEMES.velious.statusOk).toBe('#6fc8b0');
    expect(THEMES.velious.fontDisplay).toBe('Cinzel Decorative');
  });

  it('matches the Phase 40 flag-token UI-SPEC values (velious)', () => {
    expect(THEMES.velious.flagNodrop).toBe('#e88a8a');
    expect(THEMES.velious.flagLore).toBe('#e3c46b');
    expect(THEMES.velious.flagMagic).toBe('#6db3e0');
  });

  it('carries the inverting parchment row tokens for heavy only', () => {
    expect(THEMES.heavy.rowBg).toBe('#c9b072');
    expect(THEMES.heavy.rowText).toBe('#3a2817');
    expect(THEMES.velious.rowBg).toBeUndefined();
  });
});

describe('resolveTheme', () => {
  it('returns the stored key when valid', () => {
    expect(resolveTheme('kunark')).toBe('kunark');
    expect(resolveTheme('heavy')).toBe('heavy');
  });

  it('falls back to velious for an unknown or null value (no injection)', () => {
    expect(resolveTheme('bogus')).toBe('velious');
    expect(resolveTheme('sheets-default')).toBe('velious'); // dropped key -> default
    expect(resolveTheme(null)).toBe('velious');
    expect(resolveTheme('')).toBe('velious');
  });

  it('never returns a key outside the 5-theme enum', () => {
    const out: ThemeKey = resolveTheme('<script>');
    expect(THEME_KEYS).toContain(out);
  });
});
