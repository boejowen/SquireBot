// weeklySchemaHealthcheck — Phase 5 plan 05-01 task 2 (OPS-06).
//
// Weekly trigger (Sun 03:00 PT — installed by installTriggers, alongside
// monitorCellCount in the same hour window) that verifies all 13 expected
// tabs exist BY SHEET ID. ID-based lookup is resilient to user renames
// (RESEARCH §Pitfall P7): the user can rename _meta → "_meta (renamed)"
// and the healthcheck stays green; only deletion or replacement (which
// changes sheet ID) trips the alarm.
//
// First-run backfill: when _meta has no `expected_sheet_ids` row, build
// the {tab_name: getSheetId()} map from the current workbook and write
// it as the source of truth.
//
// On missing-tab: writes a structured JSON envelope to _meta.last_error
// AND _status.last_error of kind 'tab_missing' with comma-separated tab
// names in detail. Watcher reads _meta.last_error and surfaces it to the
// tray-red state via the heartbeat reader (same dual-write pattern as
// monitorCellCount per Pitfall P8).
//
// _archive is INTENTIONALLY excluded from EXPECTED_TABS — it is lazy-
// created by the archive lib in plan 05-02. Adding it here would trigger
// a false-positive on every workbook that has not yet seen an eviction.

import { log } from '../lib/log';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from '../lib/sheet-helpers';

const EXPECTED_TABS = [
  '_meta', '_char_owner', '_item_master', '_pigparse',
  '_wiki_spells', '_wiki_gear_tier', '_quest_items', '_audit', '_status',
  'view', 'gear_check', 'spell_check', 'bank',
];
// NOTE: _archive intentionally excluded — lazy-created by archive.ts (05-02).

export function weeklySchemaHealthcheck(): void {
  const ss = getActiveSpreadsheet();
  const allSheets = ss.getSheets();
  const sheetsById = new Map(allSheets.map((s) => [s.getSheetId(), s]));
  const sheetsByName = new Map(allSheets.map((s) => [s.getName(), s]));

  const meta = readMetaRows('_meta');
  const idsJsonRow = meta.find((r) => r.key === 'expected_sheet_ids');
  const expectedIds: Record<string, number> = idsJsonRow
    ? JSON.parse(idsJsonRow.value || '{}')
    : {};

  if (Object.keys(expectedIds).length === 0) {
    for (const name of EXPECTED_TABS) {
      const s = sheetsByName.get(name);
      if (s) expectedIds[name] = s.getSheetId();
    }
    writeMetaRow('_meta', 'expected_sheet_ids', JSON.stringify(expectedIds));
    log('info', 'weeklySchemaHealthcheck', { backfilled: Object.keys(expectedIds).length });
  }

  const missing: string[] = [];
  for (const name of EXPECTED_TABS) {
    const id = expectedIds[name];
    if (id == null) { missing.push(name); continue; }
    if (!sheetsById.has(id)) missing.push(name);
  }

  if (missing.length === 0) {
    writeMetaRow('_status', 'last_schema_check', new Date().toISOString());
    writeMetaRow('_status', 'last_schema_check_status', 'ok');
    log('info', 'weeklySchemaHealthcheck', { ok: true, checked: EXPECTED_TABS.length });
    return;
  }

  const err = {
    at: new Date().toISOString(),
    where: 'weeklySchemaHealthcheck',
    kind: 'tab_missing',
    detail: missing.join(','),
  };
  const errJson = JSON.stringify(err);
  writeMetaRow('_meta', 'last_error', errJson);
  writeMetaRow('_status', 'last_error', errJson);
  log('warn', 'weeklySchemaHealthcheck', { missing });
}
