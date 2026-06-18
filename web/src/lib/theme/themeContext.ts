// Theme-context bridge (Phase 30 / D-06). The ThemePicker moved out of the header
// SettingsMenu (which dissolved to identity + Sign out) and into the Settings tab.
// A `{@render children()}` page can't receive `bind:theme` as a prop the way
// SettingsMenu did, so +layout.svelte provides a small get/set accessor over the
// SAME `theme` $state via setContext (mirrors AuthGate's SESSION_KEY idiom). The
// single `$effect(applyTheme)` writer in +layout stays the only [data-theme]
// writer — this context only lets a relocated picker MUTATE that one state; it
// never writes the attribute itself.
import type { ThemeKey } from './themes';

/** Context key for the theme accessor (mirrors AuthGate's SESSION_KEY symbol). */
export const THEME_KEY = Symbol('theme');

/** A get/set accessor over +layout's single `theme` $state. */
export type ThemeContext = { get: () => ThemeKey; set: (v: ThemeKey) => void };
