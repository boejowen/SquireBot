// buildGearCheck — Phase 4 plan 04-03 task 4.
//
// Full-snapshot rebuild of the consolidated `gear_check` tab. Per
// character (with class+race set in _char_owner), reads the Velious
// gear-tier recommendations for the relevant tiers (Pre-Raid + Raiding
// always; Iksar IFF char.race=='IKS'); checks inv:<Char> for whether
// the recommended item is currently equipped in the matching slot;
// emits Status = OK | MISSING | OTHER per (char, tier, slot, recommendation).
//
// Slot pair-matching: Wiki uses "Ears" / "Fingers" / "Wrists" but inv
// uses EAR1+EAR2 / FINGER1+FINGER2 / WRIST1+WRIST2. WIKI_SLOT_TO_INV_SLOTS
// from eq-constants drives the lookup; a char is OK if the recommended
// item is in EITHER slot of a pair.

import { log } from '../lib/log';
import { getActiveSpreadsheet, writeMetaRow } from '../lib/sheet-helpers';
import { applyTheme, getActiveTheme } from '../lib/themes';
import { WIKI_SLOT_TO_INV_SLOTS } from '../lib/eq-constants';
import type { WikiGearTierRow, Tier } from '../lib/wiki-gear-tier-types';

export const GEAR_CHECK_TAB = 'gear_check';
export const GEAR_CHECK_HEADERS = ['Char', 'Class', 'Tier', 'Slot', 'Have', 'Recommended', 'Status'];
export const DEBOUNCE_MS = 10_000;
export const GEAR_CHECK_LAST_BUILD_PROP = 'gear_check_last_build_ms';
const LOCK_TIMEOUT_MS = 30_000;

// Tier sort order: Pre-Raid → Raiding → Iksar (Iksar last so Iksar
// chars see their racial section visually grouped at the bottom).
const TIER_SORT: Record<Tier, number> = {
  'Velious Pre-Raid/Group': 1,
  'Velious Raiding': 2,
  'Iksar': 3,
};

interface CharMetadata {
  char_name: string;
  class: string;
  race: string;
}

interface InvItem {
  location: string;
  itemName: string;
  itemId: number;
}

export function buildGearCheck(): void {
  const props = PropertiesService.getDocumentProperties();
  const lastBuild = parseInt(props.getProperty(GEAR_CHECK_LAST_BUILD_PROP) ?? '0', 10);
  const now = Date.now();
  if (lastBuild > 0 && now - lastBuild < DEBOUNCE_MS) {
    log('debug', 'buildGearCheck', { skipped: 'debounced', sinceLastMs: now - lastBuild });
    return;
  }

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(LOCK_TIMEOUT_MS)) {
    log('warn', 'buildGearCheck', { skipped: 'lock_busy' });
    return;
  }
  try {
    runBuild(now);
    props.setProperty(GEAR_CHECK_LAST_BUILD_PROP, String(Date.now()));
  } finally {
    lock.releaseLock();
  }
}

function runBuild(startMs: number): void {
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(GEAR_CHECK_TAB);
  if (!sheet) {
    log('warn', 'buildGearCheck', { skipped: 'sheet_missing' });
    return;
  }

  const chars = readCharOwnerWithMetadata(ss);
  const wikiGearByTierClass = readWikiGearByTierClass(ss);
  const inventoriesByChar = readInventoriesByChar(ss);

  const dataRows: unknown[][] = [];
  let charsWithMetadata = 0;
  for (const c of chars) {
    if (!c.class) continue;
    charsWithMetadata++;

    const tiersToShow: Tier[] = ['Velious Pre-Raid/Group', 'Velious Raiding'];
    if (c.race === 'IKS') tiersToShow.push('Iksar');

    for (const tier of tiersToShow) {
      const tierMap = wikiGearByTierClass.get(tier) ?? new Map();
      const classGear: WikiGearTierRow[] = tierMap.get(c.class) ?? [];

      // Group recommendations by slot.
      const bySlot = new Map<string, WikiGearTierRow[]>();
      for (const g of classGear) {
        if (!bySlot.has(g.slot)) bySlot.set(g.slot, []);
        bySlot.get(g.slot)!.push(g);
      }

      for (const [slot, recommendations] of bySlot) {
        const invSlots = WIKI_SLOT_TO_INV_SLOTS[slot] ?? [];
        const charItemsInSlots = (inventoriesByChar.get(c.char_name) ?? [])
          .filter((it) => invSlots.includes(it.location));
        for (const rec of recommendations) {
          const matched = charItemsInSlots.find((it) =>
            it.itemName.toLowerCase() === rec.item_name.toLowerCase()
          );
          let status: 'OK' | 'MISSING' | 'OTHER';
          let have = '';
          if (matched) {
            status = 'OK';
            have = matched.itemName;
          } else if (charItemsInSlots.length > 0) {
            status = 'OTHER';
            // Show the first item in the inv slot (any inv slot; pair-slot
            // chars may have items in both — pick the one most relevant to
            // the rec's slot, but for v1 simplicity: first is fine)
            have = charItemsInSlots[0].itemName;
          } else {
            status = 'MISSING';
            have = '';
          }
          dataRows.push([c.char_name, c.class, tier, slot, have, rec.item_name, status]);
        }
      }
    }
  }

  // Sort: char asc → tier rank asc → slot asc → recommended asc.
  dataRows.sort((a, b) => {
    const ca = String(a[0]); const cb = String(b[0]);
    if (ca !== cb) return ca < cb ? -1 : 1;
    const ta = TIER_SORT[a[2] as Tier] ?? 999;
    const tb = TIER_SORT[b[2] as Tier] ?? 999;
    if (ta !== tb) return ta - tb;
    const sa = String(a[3]); const sb = String(b[3]);
    if (sa !== sb) return sa < sb ? -1 : 1;
    const ra = String(a[5]); const rb = String(b[5]);
    return ra < rb ? -1 : ra > rb ? 1 : 0;
  });

  // Clear prior data range.
  const lastRow = sheet.getLastRow();
  if (lastRow > 1) {
    sheet.getRange(2, 1, lastRow - 1, GEAR_CHECK_HEADERS.length).clearContent();
  }
  if (dataRows.length > 0) {
    sheet.getRange(2, 1, dataRows.length, GEAR_CHECK_HEADERS.length).setValues(dataRows);
  }

  applyTheme(sheet, getActiveTheme());

  writeMetaRow('_status', 'last_gear_check_build', new Date().toISOString());
  writeMetaRow('_status', 'last_gear_check_row_count', String(dataRows.length));

  log('info', 'buildGearCheck', {
    rows: dataRows.length,
    charsWithMetadata,
    charsTotal: chars.length,
    durationMs: Date.now() - startMs,
  });
}

function readCharOwnerWithMetadata(
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): CharMetadata[] {
  const sheet = ss.getSheetByName('_char_owner');
  if (!sheet) return [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 1) return [];
  // _char_owner cols: char_name=A(1), class=E(5), race=N(14)
  const values = sheet.getRange(1, 1, lastRow, 14).getValues();
  const out: CharMetadata[] = [];
  for (const r of values) {
    const charName = String(r[0] ?? '').trim();
    if (!charName || charName === 'char_name') continue;
    out.push({
      char_name: charName,
      class: String(r[4] ?? '').trim(),
      race: String(r[13] ?? '').trim(),
    });
  }
  return out;
}

function readWikiGearByTierClass(
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): Map<Tier, Map<string, WikiGearTierRow[]>> {
  const out = new Map<Tier, Map<string, WikiGearTierRow[]>>();
  const sheet = ss.getSheetByName('_wiki_gear_tier');
  if (!sheet) return out;
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return out;
  // 7 cols: tier, class, slot, item_id, item_name, rank, last_refreshed
  const values = sheet.getRange(2, 1, lastRow - 1, 7).getValues();
  for (const r of values) {
    const tier = String(r[0] ?? '').trim() as Tier;
    const cls = String(r[1] ?? '').trim();
    const slot = String(r[2] ?? '').trim();
    const itemName = String(r[4] ?? '').trim();
    const rank = typeof r[5] === 'number' ? r[5] : parseInt(String(r[5] ?? '0'), 10) || 0;
    if (!tier || !cls || !slot || !itemName) continue;
    if (!out.has(tier)) out.set(tier, new Map());
    const tierMap = out.get(tier)!;
    if (!tierMap.has(cls)) tierMap.set(cls, []);
    tierMap.get(cls)!.push({
      tier, class: cls, slot, item_id: null, item_name: itemName, rank,
      last_refreshed: String(r[6] ?? ''),
    });
  }
  return out;
}

function readInventoriesByChar(
  ss: GoogleAppsScript.Spreadsheet.Spreadsheet,
): Map<string, InvItem[]> {
  const out = new Map<string, InvItem[]>();
  for (const sheet of ss.getSheets()) {
    const name = sheet.getName();
    if (!name.startsWith('inv:')) continue;
    const charName = name.slice(4);
    const lastRow = sheet.getLastRow();
    if (lastRow < 2) continue;
    // inv schema: Location | Name | ID | Count | Slots | _uploaded_at
    const values = sheet.getRange(2, 1, lastRow - 1, 6).getValues();
    const items: InvItem[] = [];
    for (const r of values) {
      const itemName = String(r[1] ?? '').trim();
      const idRaw = r[2];
      const id = typeof idRaw === 'number' ? idRaw : parseInt(String(idRaw ?? ''), 10);
      if (!itemName) continue;
      items.push({
        location: String(r[0] ?? '').trim(),
        itemName,
        itemId: Number.isFinite(id) ? id : 0,
      });
    }
    out.set(charName, items);
  }
  return out;
}
