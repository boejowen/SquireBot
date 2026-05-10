# Phase 4 Patterns — File-by-File Closest Analogs

**Date:** 2026-05-10
**Method:** Direct survey. Most files have direct Phase 3 analogs (clone + adapt); the new patterns are HtmlService sidebar forms, wiki table-list parsing, `Range.protect()`, and cell-count monitoring.

---

## §1. Files Phase 4 will create — closest analogs

### Schema migration
- **`apps-script/src/lib/migrations.ts`** (extend, not new) — adds `migrateToV3()` immediately after `migrateToV2()`. **Direct clone of `migrateToV2`'s shape.** Same idempotent pattern: schema_version write LAST, lock-protected, single appendColumns call. Difference: only one column to add (`_char_owner.race`).

### Constants module
- **`apps-script/src/lib/eq-constants.ts`** (new) — single source of truth for class/race lists + slot map. **No analog in Phase 3** (Phase 3's themes.ts is the closest pattern: a `Record<Key, Value>` const exported as the SoT). Lives at `lib/` because used by triggers + tabs + sidebars.

### Wiki parsers
- **`apps-script/src/lib/wiki-spell-parser.ts`** (new) — pure function. **Closest analog: `apps-script/src/lib/wiki-parser.ts`** (Phase 3's `parseItempage`). Same shape (pure, returns discriminated-union ParseResult), but parses different wikitext structure (==Level N== sections + {{SpellRow}} templates instead of {{Itempage}} key-value pairs). Reuses `computeSha1Hex` and the depth-counting brace scanner. Doesn't need redirect handling (class pages don't redirect).
- **`apps-script/src/lib/wiki-gear-tier-parser.ts`** (new) — pure function. **No close analog**; parses `<ul><li>` HTML lists. Steal: pure-function shape + ParseResult discriminated union from wiki-parser.ts.

### Triggers (refreshXxx)
- **`apps-script/src/triggers/refreshWikiSpells.ts`** (new) — **direct clone of `refreshWikiItems.ts`** with two changes: (a) iterates `CLASSES` from eq-constants instead of `collectInventoryItemRefs()`; (b) different post-processing per fetched page (calls wiki-spell-parser.parseClassPage instead of parseItempage). Cursor pattern identical (5min budget, 60s self-reschedule). Final-step calls `buildSpellCheck()`.
- **`apps-script/src/triggers/refreshWikiGearTier.ts`** (new) — **direct clone of `refreshWikiSpells.ts`**. Iterates 2 page names (Pre-Raid, Raiding). Final-step calls `buildGearCheck()`. Cursor pattern is overkill (2 fetches) but consistent.

### Builders
- **`apps-script/src/tabs/buildSpellCheck.ts`** (new) — **direct clone of `buildView.ts`** structure. Reads spell:* tabs (instead of inv:* tabs), reads `_wiki_spells` (instead of `_pigparse`+`_item_master`), reads `_char_owner` for class+level. Same lock+debounce+applyTheme. PropertiesService key: `spell_check_last_build_ms`.
- **`apps-script/src/tabs/buildGearCheck.ts`** (new) — **like buildSpellCheck.ts** but reads inv:* tabs + `_wiki_gear_tier`. Includes Iksar-tier filtering by char.race. Slot pair-matching via `WIKI_SLOT_TO_INV_SLOTS`. PropertiesService key: `gear_check_last_build_ms`.

### Sidebar forms (NEW PATTERN)
- **`apps-script/src/triggers/showCharInfoSidebar.ts`** (new) — **no analog in Phase 3** (the theme picker is a modal dialog, not a sidebar form). Shape: server-side handler exports `showCharInfoSidebar()` (creates the HtmlOutput + opens it via `SpreadsheetApp.getUi().showSidebar()`); also exports `getCharsForForm()` and `saveCharInfo(charsArray)` as `google.script.run` callbacks invoked from the sidebar's client-side JS. The HTML is inline as a template-literal string (Phase 5 may move to .html files via clasp's html convention, deferred).
- **`apps-script/src/triggers/showBankCoinSidebar.ts`** (new) — same pattern as showCharInfoSidebar but simpler (4 number fields, single save callback).

### Cell count monitor
- **`apps-script/src/triggers/monitorCellCount.ts`** (new) — **no analog in Phase 3**. Single function: iterate sheets, sum lastRow*lastColumn, write to `_status.cell_count`, conditionally write `_meta.last_error` if > 5M. Closest pattern: `refreshPigparse.ts` (single function, lock-protected, error-record writeback).

### Wiring
- **`apps-script/src/triggers/onChange.ts`** (modify) — extend Phase 3's onChange to call `buildGearCheck` + `buildSpellCheck` alongside `buildView` + `buildBank`.
- **`apps-script/src/triggers/installTriggers.ts`** (modify) — extend Phase 3's 4-trigger install to 7 triggers. Add `refreshWikiSpells` (Sun 04:30 PT), `refreshWikiGearTier` (Sun 05:00 PT), `monitorCellCount` (Sun 03:30 PT).
- **`apps-script/src/triggers/onOpen.ts`** (modify) — extend SquireBot menu with two new items: "Set Character Info…" and "Set Bank Coin…".
- **`apps-script/src/Code.ts`** (modify) — re-export 6 new functions: `migrateToV3`, `showCharInfoSidebar`, `showBankCoinSidebar`, `refreshWikiSpells`, `refreshWikiGearTier`, `buildSpellCheck`, `buildGearCheck`, `monitorCellCount`. Plus 2 server-side callbacks invoked by sidebars: `getCharsForForm`, `saveCharInfo`, `getBankCoinForForm`, `saveBankCoin`. **All must be added to `build.mjs` TRIGGER_GLOBALS** (Phase 3 lesson from bug `d0a2645`).

### CI assertion (NEW)
- **`apps-script/build.mjs`** (modify) — add a check that grep's `^export {` from `src/Code.ts` and compares against the `TRIGGER_GLOBALS` array. Throws if any export is missing from globals (or vice versa). Catches the Phase 3 class of bugs at build time. **No analog**; new pattern.

### Tests
- **`apps-script/src/__tests__/migrations.test.ts`** (extend) — add 3-4 tests for `migrateToV3` (clone of migrateToV2 tests).
- **`apps-script/src/__tests__/wiki-spell-parser.test.ts`** (new) — fixture-driven against Necromancer/Paladin/Warrior fixtures.
- **`apps-script/src/__tests__/wiki-gear-tier-parser.test.ts`** (new) — fixture-driven against Pre-Raid + Raiding fixtures, including Iksar detection.
- **`apps-script/src/__tests__/refreshWikiSpells.test.ts`** (new) — clone of refreshWikiItems.test.ts.
- **`apps-script/src/__tests__/refreshWikiGearTier.test.ts`** (new) — clone of refreshWikiSpells.test.ts.
- **`apps-script/src/__tests__/buildSpellCheck.test.ts`** (new) — clone of buildView.test.ts.
- **`apps-script/src/__tests__/buildGearCheck.test.ts`** (new) — clone of buildSpellCheck.test.ts. Includes Iksar-tier-filtering tests.
- **`apps-script/src/__tests__/charInfoSidebar.test.ts`** (new) — tests the server-side callbacks (`getCharsForForm`, `saveCharInfo`) with mocked SpreadsheetApp. The sidebar HTML itself isn't unit-tested (it's a thin client; integration testing is the smoke test).
- **`apps-script/src/__tests__/bankCoinSidebar.test.ts`** (new) — same pattern.
- **`apps-script/src/__tests__/monitorCellCount.test.ts`** (new) — synthetic sheets at different sizes; assert correct sum + threshold-trip.

---

## §2. Patterns to STEAL from Phase 3

| Pattern | Phase 3 example | Phase 4 use |
|---|---|---|
| Migration: schema_version-write-LAST + idempotent | `migrateToV2` | `migrateToV3` |
| Resumable cursor: 5min budget, 60s self-reschedule | `refreshWikiItems` | `refreshWikiSpells`, `refreshWikiGearTier` |
| Pure parser: discriminated-union ParseResult | `parseItempage` in wiki-parser.ts | `parseClassPage`, `parseGearTierPage` |
| Builder: lock + debounce + applyTheme | `buildView` | `buildGearCheck`, `buildSpellCheck` |
| Tab-iteration with name prefix | `inv:` filter in buildView | `spell:` filter in buildSpellCheck |
| In-memory join via Map<key, rows> | _pigparse + _item_master maps in buildView | _wiki_spells + _wiki_gear_tier maps |
| Atomic full-replace via clearContent + setValues | buildView's data-region rewrite | All builders |
| Cell-note composition | composeItemNote.ts | (skipped Phase 4 — no notes on gear_check/spell_check in v1) |
| Test mock SpreadsheetApp/LockService/etc. | test-helpers.ts | Reused; minor extension for sidebar callbacks |
| Doc-comment preamble explaining algorithm | refreshWikiItems.ts top comment | All new triggers/builders |
| Ship watcher v0.X.0 BEFORE running migration | v0.3.0 → migrateToV2 | v0.4.0 → migrateToV3 |

---

## §3. New patterns introduced in Phase 4

### Pattern A: HtmlService sidebar form with google.script.run callbacks

The theme picker (Phase 3) is a modal dialog. Sidebar forms are different:

```typescript
export function showCharInfoSidebar(): void {
  const html = HtmlService.createHtmlOutput(`
    <html><body>
      <div id="form"></div>
      <script>
        google.script.run.withSuccessHandler(renderForm).getCharsForForm();
        function renderForm(chars) { /* render input rows */ }
        function save() {
          const data = collectFormData();
          google.script.run
            .withSuccessHandler(() => google.script.host.close())
            .withFailureHandler((err) => alert(err.message))
            .saveCharInfo(data);
        }
      </script>
    </body></html>
  `).setTitle('Set Character Info').setWidth(360);
  SpreadsheetApp.getUi().showSidebar(html);
}

export function getCharsForForm(): CharRow[] { /* read _char_owner */ }
export function saveCharInfo(chars: CharRow[]): void { /* upsert _char_owner */ }
```

Key constraints:
- `google.script.run.<fn>` only finds top-level globals (same as triggers). Add to `TRIGGER_GLOBALS`.
- Sidebar HTML lives inline as a template literal (not a separate .html file via clasp's HTML convention). Cleaner for Phase 4; revisit in Phase 5 if HTML grows large.
- Server-side callbacks (`getCharsForForm`, `saveCharInfo`) are the testable surfaces. Test them with mocked SpreadsheetApp; don't try to test the inline JS.

### Pattern B: Range.protect()

Used to prevent raw cell edits to bank coin cells:

```typescript
const sheet = ss.getSheetByName('_meta');
const ranges = ['B' + bankCoinPpRow, 'B' + bankCoinGpRow, ...]; // each coin cell
ranges.forEach(a1 => {
  const protection = sheet.getRange(a1).protect()
    .setDescription('SquireBot bank coin — edit via SquireBot menu')
    .setWarningOnly(false);
  // editors automatically include the script owner; no need to call addEditor.
});
```

Apps Script docs: protected ranges allow the script owner (workbook owner) to bypass the protection. The script runs as the workbook owner under container-bound auth, so writes succeed; raw edits by other users (or the owner via raw cell edit) get the protection prompt.

**Verification gap (RESEARCH §5 #6):** I haven't tested in production whether the script-as-owner truly bypasses protection. Plan 04-04 should include a smoke check: after Range.protect runs, manually try editing the cell from the spreadsheet UI as the owner; confirm the warning prompt fires. Then run `saveBankCoin` from the sidebar; confirm it succeeds without prompt.

### Pattern C: Cell-count monitoring

```typescript
function totalAddressableCells(ss: GoogleAppsScript.Spreadsheet.Spreadsheet): number {
  return ss.getSheets().reduce((sum, sheet) => {
    return sum + sheet.getLastRow() * sheet.getLastColumn();
  }, 0);
}
```

Note: this counts the addressable range, not just non-empty cells. Sheets's 10M cap is on addressable cells, so this is correct. (Sheets's cap formula: `rows × cols ≤ 10,000,000` per workbook.)

### Pattern D: CI assertion for export ↔ globals alignment

In `build.mjs`, before the esbuild call:

```javascript
import { readFileSync } from 'node:fs';
const codeTs = readFileSync('src/Code.ts', 'utf8');
const exportedNames = new Set();
for (const m of codeTs.matchAll(/^export\s+\{\s*([^}]+)\s*\}/gm)) {
  m[1].split(',').forEach(n => exportedNames.add(n.trim().split(/\s+as\s+/)[0]));
}
for (const m of codeTs.matchAll(/^export\s+function\s+(\w+)/gm)) {
  exportedNames.add(m[1]);
}
const globals = new Set(TRIGGER_GLOBALS);
const missingFromGlobals = [...exportedNames].filter(n => !globals.has(n));
const missingFromExports = [...globals].filter(n => !exportedNames.has(n));
if (missingFromGlobals.length || missingFromExports.length) {
  console.error('TRIGGER_GLOBALS / Code.ts exports out of sync.');
  if (missingFromGlobals.length) console.error('  In Code.ts but not TRIGGER_GLOBALS:', missingFromGlobals);
  if (missingFromExports.length) console.error('  In TRIGGER_GLOBALS but not Code.ts:', missingFromExports);
  process.exit(1);
}
```

Catches the Phase 3 class of bugs (`d0a2645`: migrateToV2 missing from globals) at build time, before clasp push.

---

## §4. Patterns to REJECT (don't carry over)

- **Cell notes on gear_check/spell_check rows** — not REQ in v1; would bloat the bundle and add latency. v2 idea.
- **Per-character rebuild** (only rebuilding rows for the changed inv:* tab) — full-rebuild stays correct + simple at our scale; same call as Phase 3's buildView.
- **Auto-rebuild on _char_owner edits via sidebar** — the sidebar's save callback explicitly fires `buildGearCheck` + `buildSpellCheck`. Don't rely on `onChange` catching the _char_owner edit (it would, but explicit-trigger is clearer).

---

## §5. Cross-side coordination requirements

Same as Phase 3's PATTERNS §5, with these additions:

### Schema version coordination (CRITICAL)
- `internal/sheet/client.go:44` declares `WatcherMaxSchemaVersion = 2`.
- Phase 4 plan 04-01 task 1 bumps to **3**.
- Migration sequence: ship watcher v0.4.0 → user updates → THEN run `migrateToV3` → both sides agree on schema_version=3.

### `_char_owner.race` column scaffold
- Append to `internal/scaffold/scaffold.go` `DimensionTabs[_char_owner].Headers` so fresh workbooks get `race` at v=2 scaffold time.
- Existing v=2 workbooks get `race` via `migrateToV3`'s `appendColumns('_char_owner', ['race'])` (idempotent — write-if-absent).

### MetaRows
- No new MetaRows needed in Phase 4 (`bank_coin_*` already scaffolded by Phase 2).

### onChange handler
- Phase 3's onChange handler calls buildView + buildBank.
- Phase 4 extends to also call buildGearCheck + buildSpellCheck.
- Same 10s debounce inside each builder protects against fan-out storms.

### Trigger inventory growth (4 → 7)
- Phase 3 installTriggers: onChange, buildView (1h backstop), refreshPigparse (daily 03:00 PT), refreshWikiItems (weekly Sun 04:00 PT).
- Phase 4 adds: refreshWikiSpells (Sun 04:30 PT), refreshWikiGearTier (Sun 05:00 PT), monitorCellCount (Sun 03:30 PT).
- Stagger times prevent lock contention. Cell-count monitor is intentionally first (read-only-mostly, gathers baseline before scrapes mutate things).

---

*Phase: 04-differentiator-features*
*Patterns mapped: 2026-05-10 by orchestrator-direct inspection*
*Most of Phase 4 is "clone Phase 3 file, adapt the data shape" — see §1 table for the analogs.*
