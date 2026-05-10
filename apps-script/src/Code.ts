// Apps Script entry. Re-exports trigger functions so build.mjs's footer
// can lift them to top-level globals (Apps Script's trigger system finds
// triggers by global function name; ES module exports alone aren't
// enough).
//
// Phase 3 CODE-COMPLETE as of plan 03-04. Phase 4 plan 04-01 adds
// migrateToV3 + showCharInfoSidebar (with its 2 google.script.run
// callbacks getCharsForForm + saveCharInfo). Plans 04-02..04 will add
// more.

import { setTheme } from './lib/themes';
import { migrateToV2, migrateToV3 } from './lib/migrations';
import { refreshPigparse } from './triggers/refreshPigparse';
import { refreshWikiItems } from './triggers/refreshWikiItems';
import { refreshWikiSpells } from './triggers/refreshWikiSpells';
import { refreshWikiGearTier } from './triggers/refreshWikiGearTier';
import { onChange } from './triggers/onChange';
import { onOpen, showThemePickerModal } from './triggers/onOpen';
import { installTriggers } from './triggers/installTriggers';
import {
  showCharInfoSidebar,
  getCharsForForm,
  saveCharInfo,
} from './triggers/showCharInfoSidebar';
import { buildView } from './tabs/buildView';
import { buildBank } from './tabs/buildBank';
import { buildSpellCheck } from './tabs/buildSpellCheck';
import { buildGearCheck } from './tabs/buildGearCheck';

export {
  setTheme,
  migrateToV2, migrateToV3,
  refreshPigparse, refreshWikiItems, refreshWikiSpells, refreshWikiGearTier,
  onChange, onOpen, showThemePickerModal,
  showCharInfoSidebar, getCharsForForm, saveCharInfo,
  installTriggers,
  buildView, buildBank, buildSpellCheck, buildGearCheck,
};
