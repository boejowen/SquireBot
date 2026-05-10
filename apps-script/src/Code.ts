// Apps Script entry. Re-exports trigger functions so build.mjs's footer
// can lift them to top-level globals (Apps Script's trigger system finds
// triggers by global function name; ES module exports alone aren't
// enough).
//
// Phase 3 CODE-COMPLETE as of plan 03-04: every trigger function is
// real. No stubs remain.

import { setTheme } from './lib/themes';
import { migrateToV2 } from './lib/migrations';
import { refreshPigparse } from './triggers/refreshPigparse';
import { refreshWikiItems } from './triggers/refreshWikiItems';
import { onChange } from './triggers/onChange';
import { onOpen, showThemePickerModal } from './triggers/onOpen';
import { installTriggers } from './triggers/installTriggers';
import { buildView } from './tabs/buildView';
import { buildBank } from './tabs/buildBank';

export {
  setTheme, migrateToV2,
  refreshPigparse, refreshWikiItems,
  onChange, onOpen, showThemePickerModal,
  installTriggers,
  buildView, buildBank,
};
