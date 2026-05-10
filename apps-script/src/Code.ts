// Apps Script entry. Re-exports trigger functions so build.mjs's footer
// can lift them to top-level globals (Apps Script's trigger system finds
// triggers by global function name; ES module exports alone aren't
// enough).
//
// Stub triggers throw a descriptive error until their owning plan lands
// the implementation. This lets us deploy the bundle today (Plan 03-01)
// without exposing the workbook to half-baked code paths.

import { setTheme } from './lib/themes';
import { migrateToV2 } from './lib/migrations';
import { refreshPigparse } from './triggers/refreshPigparse';
import { refreshWikiItems } from './triggers/refreshWikiItems';

// --- Implemented in plan 03-01 ---
export { setTheme, migrateToV2 };

// --- Implemented in plan 03-02 ---
export { refreshPigparse };

// --- Implemented in plan 03-03 ---
export { refreshWikiItems };

// --- Stubs filled by later plans ---
function notImplemented(name: string, plan: string): never {
  throw new Error(`${name} not yet implemented (lands in Phase 3 ${plan})`);
}

export function buildView(): void { notImplemented('buildView', 'plan 03-04'); }
export function buildBank(): void { notImplemented('buildBank', 'plan 03-04'); }
export function onChange(): void { notImplemented('onChange', 'plan 03-04'); }
export function onOpen(): void { notImplemented('onOpen', 'plan 03-04'); }
export function installTriggers(): void { notImplemented('installTriggers', 'plan 03-04'); }
export function showThemePickerModal(): void { notImplemented('showThemePickerModal', 'plan 03-04'); }
