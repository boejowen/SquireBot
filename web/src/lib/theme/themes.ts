// EQ theme registry — CSS-custom-property port of the v1 Apps Script THEMES
// (apps-script/src/lib/themes.ts), Phase 14 Plan 14-02 Task 4 / WEB-05 / D-06.
//
// PORTED from v1: the 5 themed KEYS (vanilla / kunark / velious / minimalist /
// heavy). DROPPED: the v1 no-styling opt-out sentinel (meaningless off-Sheets
// where there is no native-spreadsheet default to fall back to — D-06).
// CHANGED: DEFAULT_THEME from 'minimalist' to 'velious' (the guild's site
// leans into Velious identity — D-06).
//
// The v1 Theme interface was Sheet-API-shaped (headerBg/rowFg + applyTheme
// calling setBackground/setFontColor). The web emits CSS custom properties on
// a [data-theme="<key>"] root instead — a theme swap is a single attribute
// write. The TOKEN VALUES come from the 14-UI-SPEC Theme Catalog (the richer
// mockup values, which supersede the dimmer v1 hexes); CLAUDE.md's "derive
// from docs/design/eq-aesthetic-theme.md" rule is satisfied transitively
// because the UI-SPEC catalog itself derives from that doc.
//
// The matching [data-theme] CSS blocks live in src/app.css (emitted from
// these same values). The actual attribute write + localStorage wiring into
// the shell is Plan 04's ThemePicker/SiteShell; this module provides the
// registry + the resolve/load/save helpers.

export type ThemeKey = 'velious' | 'vanilla' | 'kunark' | 'minimalist' | 'heavy';

export const THEME_KEYS: ThemeKey[] = ['velious', 'vanilla', 'kunark', 'minimalist', 'heavy'];

export const DEFAULT_THEME: ThemeKey = 'velious';

const STORAGE_KEY = 'theme';

export interface ThemeTokens {
  bg: string;
  panel: string;
  text: string;
  accent: string;
  statusOk: string;
  statusMissing: string;
  statusOther: string;
  /** Phase 40 flag-outline tokens — No-Drop>Lore>Magic; WCAG ≥3:1 vs --panel. */
  flagNodrop: string;
  flagLore: string;
  flagMagic: string;
  fontDisplay: string;
  fontBody: string;
  /** 400 for minimalist, 600 for the other four (UI-SPEC Typography note). */
  weightDisplay: number;
  /**
   * Heavy is the one inverting theme: light parchment data rows on a dark
   * frame. These optional row-surface tokens carry the parchment fill + ink
   * for Heavy's data rows; the other themes leave them undefined (their rows
   * use panel/text). app.css emits Heavy's parchment row CSS from these.
   */
  rowBg?: string;
  rowText?: string;
}

export const THEMES: Record<ThemeKey, ThemeTokens> = {
  velious: {
    bg: '#060b18',
    panel: '#0f1729',
    text: '#d4dee8',
    accent: '#a8c5e0',
    statusOk: '#6fc8b0',
    statusMissing: '#e88a8a',
    statusOther: '#a8c5e0',
    flagNodrop: '#e88a8a',
    flagLore: '#e3c46b',
    flagMagic: '#6db3e0',
    fontDisplay: 'Cinzel Decorative',
    fontBody: 'IM Fell English',
    weightDisplay: 600,
  },
  vanilla: {
    bg: '#1a120b',
    panel: '#2a1f15',
    text: '#c9b896',
    accent: '#d4af37',
    statusOk: '#6fa86f',
    statusMissing: '#c66666',
    statusOther: '#d4af37',
    flagNodrop: '#d96b6b',
    flagLore: '#d4af37',
    flagMagic: '#6fa0d4',
    fontDisplay: 'Cinzel',
    fontBody: 'Crimson Text',
    weightDisplay: 600,
  },
  kunark: {
    bg: '#0a1108',
    panel: '#0f1612',
    text: '#d4cab0',
    accent: '#c89060',
    statusOk: '#6fa86f',
    statusMissing: '#c66666',
    statusOther: '#d4a020',
    flagNodrop: '#d96b6b',
    flagLore: '#d4a020',
    flagMagic: '#5f9fd4',
    fontDisplay: 'Cinzel',
    fontBody: 'IM Fell English',
    weightDisplay: 600,
  },
  minimalist: {
    bg: '#141414',
    panel: '#1f1f1d',
    text: '#e8e0d4',
    accent: '#b8915c',
    statusOk: '#7fa87f',
    statusMissing: '#c87878',
    statusOther: '#b8915c',
    flagNodrop: '#d98a8a',
    flagLore: '#d4b15c',
    flagMagic: '#7fa8d4',
    fontDisplay: 'Cinzel',
    fontBody: 'Inter',
    weightDisplay: 400, // the one sanctioned weight-variance (UI-SPEC)
  },
  heavy: {
    bg: '#1a0e05', // dark frame
    panel: '#1a0e05',
    text: '#e8d8a8', // chrome text
    accent: '#f0d088', // gold (ink-red #6b1a1a is the status-missing token)
    statusOk: '#2d5a2d',
    statusMissing: '#6b1a1a',
    statusOther: '#5a3a1f',
    flagNodrop: '#c0392b',
    flagLore: '#d4a017',
    flagMagic: '#3f78c0',
    fontDisplay: 'MedievalSharp',
    fontBody: 'IM Fell English',
    weightDisplay: 600,
    rowBg: '#c9b072', // parchment data rows
    rowText: '#3a2817', // dark ink on parchment
  },
};

/** Returns `stored` if it is a valid theme key, else the velious default. */
export function resolveTheme(stored: string | null): ThemeKey {
  return stored !== null && (THEME_KEYS as string[]).includes(stored)
    ? (stored as ThemeKey)
    : DEFAULT_THEME;
}

/** SSR-safe read of the per-user theme from localStorage (-> velious default). */
export function loadTheme(): ThemeKey {
  if (typeof localStorage === 'undefined') return DEFAULT_THEME;
  return resolveTheme(localStorage.getItem(STORAGE_KEY));
}

/** SSR-safe persist of the per-user theme choice. */
export function saveTheme(key: ThemeKey): void {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(STORAGE_KEY, key);
}

/**
 * Apply a theme site-wide (Plan 14-04 SiteShell/ThemePicker wiring): write the
 * SINGLE `[data-theme]` attribute on `root` AND persist the choice to
 * localStorage. The key is resolved through `resolveTheme` (the 5-key
 * whitelist), so an arbitrary value falls back to the velious default — no
 * attribute/CSS injection (T-14.04-04). A null `root` (SSR / pre-mount) is a
 * safe no-op for the DOM write; persistence still runs.
 *
 * Returns the resolved key actually applied (so the caller's `theme` state can
 * track the canonical value rather than the raw input).
 */
export function applyTheme(key: string, root: HTMLElement | null): ThemeKey {
  const resolved = resolveTheme(key);
  if (root) root.dataset.theme = resolved;
  saveTheme(resolved);
  return resolved;
}
