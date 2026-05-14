# Phase 5: Search + Onboarding + Privacy Polish — Pattern Map

**Mapped:** 2026-05-10
**Files analyzed:** 24 (new + modified)
**Analogs found:** 22 / 24 (Jekyll docs surface has no in-repo analog — Pages site is greenfield)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `apps-script/src/triggers/showSearchSidebar.ts` | sidebar-trigger | request-response (3-call: open + read + run) | `apps-script/src/triggers/showCharInfoSidebar.ts` | exact |
| `apps-script/src/triggers/showEvictionSidebar.ts` | sidebar-trigger | request-response (3-call: open + preview + commit) | `apps-script/src/triggers/showCharInfoSidebar.ts` | exact |
| `apps-script/src/triggers/weeklySchemaHealthcheck.ts` | time-driven trigger | batch + KV write | `apps-script/src/triggers/monitorCellCount.ts` | exact |
| `apps-script/src/triggers/weeklyStaleCharArchive.ts` | time-driven trigger | batch transform | `apps-script/src/triggers/monitorCellCount.ts` | role-match (housekeeping) |
| `apps-script/src/triggers/weeklyEvictionArchive.ts` | time-driven trigger | batch transform | `apps-script/src/triggers/monitorCellCount.ts` | role-match |
| `apps-script/src/triggers/installTriggers.ts` (MOD) | install/setup trigger | idempotent batch | itself (extend in place) | self |
| `apps-script/src/triggers/onOpen.ts` (MOD) | menu registration | declarative | itself (extend in place) | self |
| `apps-script/src/lib/searchIndex.ts` | library (cache + scan + fuzzy) | CRUD + transform | `apps-script/src/lib/sheet-helpers.ts` + `monitorCellCount.ts` getSheets loop | partial (no exact cache-heavy lib yet) |
| `apps-script/src/lib/archive.ts` | library (tab + row move) | transform | `apps-script/src/lib/migrations.ts` | role-match |
| `apps-script/src/lib/migrations.ts` (MOD) | library (extend) | idempotent setup | itself — add `protectBankToonName` + `hideAllSystemTabs` mirroring `protectBankCoinCells` | self |
| `apps-script/src/lib/eq-constants.ts` (MOD) | constants registry | declarative | itself — add `INVENTORY_SLOTS` array | self |
| `apps-script/src/Code.ts` (MOD) | barrel re-export | declarative | itself — add new exports | self |
| `apps-script/build.mjs` (MOD) | build config | declarative | itself — add new names to `TRIGGER_GLOBALS` | self |
| `apps-script/src/sidebars/searchSidebar.html` | HtmlService template | inline HTML + JS | `apps-script/src/triggers/showCharInfoSidebar.ts:135-217` (inline `buildSidebarHtml`) | role-match (NEW: extracted to file + theme-aware) |
| `apps-script/src/sidebars/evictionSidebar.html` | HtmlService template | inline HTML + JS | `apps-script/src/triggers/showCharInfoSidebar.ts:135-217` | role-match |
| `apps-script/src/__tests__/searchIndex.test.ts` | test | unit | `apps-script/src/__tests__/charInfoSidebar.test.ts` + `monitorCellCount.test.ts` | role-match |
| `apps-script/src/__tests__/weeklySchemaHealthcheck.test.ts` | test | unit | `apps-script/src/__tests__/monitorCellCount.test.ts` | exact |
| `apps-script/src/__tests__/archive.test.ts` | test | unit | `apps-script/src/__tests__/migrations.test.ts` | role-match |
| `apps-script/src/__tests__/showEvictionSidebar.test.ts` | test | unit | `apps-script/src/__tests__/charInfoSidebar.test.ts` | exact |
| `apps-script/src/__tests__/test-helpers.ts` (MOD) | mock library | extend | itself — replace `CacheService` stub with real Map-backed mock + add `Range.protect`/`hideSheet`/`getSheetId` returns | self |
| `docs/_config.yml` | Jekyll config | declarative | none (greenfield) | no-analog |
| `docs/index.md`, `install.md`, `troubleshooting.md`, `dev.md` | Markdown content | static | none (greenfield) | no-analog |
| `docs/assets/*.png`, `smartscreen.gif` | static asset | n/a | none | no-analog |
| `README.md` (MOD — shrink to pointer) | docs | static | itself | self |

---

## Pattern Assignments

### `apps-script/src/triggers/showSearchSidebar.ts` (sidebar-trigger, request-response)

**Analog:** `apps-script/src/triggers/showCharInfoSidebar.ts`

**Three-function shape to clone** (showCharInfoSidebar.ts:41-46, 48-70, 72-127):

```typescript
// showCharInfoSidebar.ts:41-46 — opener
export function showCharInfoSidebar(): void {
  const html = HtmlService.createHtmlOutput(buildSidebarHtml())
    .setTitle('SquireBot — Character Info')
    .setWidth(360);
  SpreadsheetApp.getUi().showSidebar(html);
}
```

Phase 5 changes: title `'SquireBot — Search'`, width `300` (UI-SPEC locks 300px), and `buildSidebarHtml(theme)` injects theme tokens. Replace `saveCharInfo` with `runSearch(query, charFilter, slotFilter): SearchResult[]`, replace `getCharsForForm` with `getSearchInitialData(): { chars: string[]; slots: string[]; recent: string[] }`.

**`google.script.run` wiring pattern** (showCharInfoSidebar.ts:152, 201-213):

```typescript
// Inside the inline <script> block:
google.script.run.withSuccessHandler(render).withFailureHandler(showErr).getCharsForForm();
// …
google.script.run
  .withSuccessHandler(function (r) { /* paint */ })
  .withFailureHandler(function (err) { /* red error */ })
  .saveCharInfo(updated);
```

**LockService pattern — NOT used for search** (read-only) but IS used for the recent-query push (write to `squirebot:search:recent`) and any cache invalidation that writes properties. Copy showCharInfoSidebar.ts:99-101 exactly when a write path is needed:

```typescript
// showCharInfoSidebar.ts:99-101
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) throw new Error('saveCharInfo: lock_busy');
try { /* writes */ } finally { lock.releaseLock(); }
```

**Inline `escapeHtml` helper to clone verbatim** (showCharInfoSidebar.ts:154):
```javascript
function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
```

**Theme-awareness extension** (NEW for Phase 5 — derive from `lib/themes.ts:35-67` registry):
- Read `getActiveTheme()` (themes.ts:75-80), look up `THEMES[key]`. If `null` (sheets-default), emit no `<style>` token block. Else emit `:root { --bg, --bg-row, --fg, --fg-header, --accent-bg, --accent-fg, --font-body }` matching UI-SPEC §Color.

**Tier-mismatch risk:** Search scan + Levenshtein logic does NOT belong in this trigger file. Put it in `lib/searchIndex.ts`. This file's `runSearch` is a thin wrapper that calls into the lib.

---

### `apps-script/src/triggers/showEvictionSidebar.ts` (sidebar-trigger, request-response)

**Analog:** `apps-script/src/triggers/showCharInfoSidebar.ts` (sidebar shell), `apps-script/src/triggers/showBankCoinSidebar.ts:47-75` (mutating save handler with lock + downstream rebuild)

**Mutating-save pattern** (showBankCoinSidebar.ts:47-75 — direct clone for `commitEviction`):

```typescript
// showBankCoinSidebar.ts:47-75
export function saveBankCoin(coin: BankCoinForm): void {
  // 1. Validate
  for (const k of ['pp', 'gp', 'sp', 'cp'] as const) {
    const v = coin[k];
    if (typeof v !== 'number' || !Number.isFinite(v) || v < 0) {
      throw new Error(`saveBankCoin: invalid ${k} value ${String(v)}`);
    }
  }
  // 2. Lock-guarded write
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) throw new Error('saveBankCoin: lock_busy');
  try {
    writeMetaRow('_meta', 'bank_coin_pp', String(coin.pp));
    // … more writes
    log('info', 'saveBankCoin', { /* … */ });
  } finally {
    lock.releaseLock();
  }
  // 3. Downstream rebuild outside the lock
  buildBank();
}
```

For `commitEviction(email)`:
- Validate: `email` is non-empty + present in `_char_owner.owner_email` distinct list.
- Inside lock: scan `_char_owner`, set `is_removed=TRUE` for matching rows; write `_meta.eviction_log` JSON entry `{at, initiated_by, email, char_names, grace_until}` (extend the existing `_meta.last_error` JSON envelope idiom from monitorCellCount.ts:40-45).
- Outside lock: no `buildView()` call needed — eviction does not affect the view tab structurally (only the `is_removed` filter on existing rows; rebuild will pick it up on next onChange).

**Confirmation modal:** Use native browser `confirm()` per UI-SPEC §Interaction Contract — no server-side modal stack. The HTML template handles this in-page; no extra Apps Script API call.

---

### `apps-script/src/triggers/weeklySchemaHealthcheck.ts` (time-driven trigger, batch + KV write)

**Analog:** `apps-script/src/triggers/monitorCellCount.ts` (line-for-line clone of structure)

**Full skeleton to clone** (monitorCellCount.ts:21-53):

```typescript
// monitorCellCount.ts:21-53
export function monitorCellCount(): void {
  const ss = getActiveSpreadsheet();
  let total = 0;
  const perSheet: Array<{ name: string; cells: number }> = [];
  for (const sheet of ss.getSheets()) {
    const cells = sheet.getLastRow() * sheet.getLastColumn();
    perSheet.push({ name: sheet.getName(), cells });
    total += cells;
  }

  writeMetaRow('_status', 'cell_count', String(total));
  writeMetaRow('_status', 'cell_count_last_check', new Date().toISOString());

  if (total > ALARM_THRESHOLD) {
    // … build top-N detail string
    const err = {
      at: new Date().toISOString(),
      where: 'monitorCellCount',
      kind: 'cell_count_threshold',
      detail: `${total}/${HARD_CAP} (top: ${topN})`,
    };
    const errJson = JSON.stringify(err);
    writeMetaRow('_meta', 'last_error', errJson);
    writeMetaRow('_status', 'last_error', errJson);
    log('warn', 'monitorCellCount', { total, threshold: ALARM_THRESHOLD, topN });
    return;
  }
  log('info', 'monitorCellCount', { total, sheets: perSheet.length });
}
```

For `weeklySchemaHealthcheck`:
- Build `EXPECTED_TABS = ['_meta', '_char_owner', '_item_master', '_pigparse', '_wiki_spells', '_wiki_gear_tier', '_quest_items', '_audit', '_status', 'view', 'gear_check', 'spell_check', 'bank']` (consult `.planning/research/ARCHITECTURE.md` for the canonical list).
- For each expected name, `ss.getSheetByName(name) === null` → missing.
- On any miss: build err with `where: 'weeklySchemaHealthcheck'`, `kind: 'tab_missing'`, `detail: <comma-sep missing names>`. Write the same `_meta.last_error` + `_status.last_error` envelope.
- On all-present: write `_status.schema_healthcheck_last_ok = ISO timestamp`.

**Trigger registration pattern** (installTriggers.ts:81-86):
```typescript
ScriptApp.newTrigger('weeklySchemaHealthcheck')
  .timeBased()
  .onWeekDay(ScriptApp.WeekDay.SUNDAY)
  .atHour(3)
  .inTimezone('America/Los_Angeles')
  .create();
```

---

### `apps-script/src/triggers/weeklyStaleCharArchive.ts` & `weeklyEvictionArchive.ts` (time-driven trigger, batch transform)

**Analog:** `apps-script/src/triggers/monitorCellCount.ts:21-30` (sheet iteration scaffolding) + `apps-script/src/lib/archive.ts` (NEW — owns the move logic)

These triggers are thin wrappers over `archive.ts` helpers. Per the tier-mismatch rule: scanning + move logic lives in the lib; the trigger only iterates candidates and delegates.

**Stale-char skeleton:**

```typescript
// derive from monitorCellCount.ts:21-30 sheet-iteration
export function weeklyStaleCharArchive(): void {
  const now = Date.now();
  const STALE_MS = 90 * 24 * 60 * 60 * 1000;
  const cutoffMs = now - STALE_MS;
  // Read _char_owner.last_seen for each char; iterate.
  // For each stale char: call archive.moveCharToArchive(charName, 'stale_90d').
  // Write _status.last_stale_archive_run = ISO + count.
}
```

**Eviction-archive skeleton:** read `_meta.eviction_log` JSON entries, find ones whose `grace_until` < now, call `archive.moveCharToArchive(charName, 'evicted')`, remove processed entries from the log.

---

### `apps-script/src/triggers/installTriggers.ts` (MODIFIED — extend in place)

**Analog:** itself. Lines 32-40 `SQUIREBOT_HANDLERS` array, lines 53-86 trigger creation, line 91 `protectBankCoinCells()` call.

**Extension pattern** (installTriggers.ts:32-40 — add 3 names):

```typescript
// installTriggers.ts:32-40 (existing 7) — extend to 10
const SQUIREBOT_HANDLERS = [
  'onChange',
  'buildView',
  'refreshPigparse',
  'refreshWikiItems',
  'refreshWikiSpells',
  'refreshWikiGearTier',
  'monitorCellCount',
  'weeklySchemaHealthcheck',   // NEW Phase 5
  'weeklyStaleCharArchive',    // NEW Phase 5
  'weeklyEvictionArchive',     // NEW Phase 5
];
```

**Add 3 new trigger registrations** mirroring installTriggers.ts:81-86 (Sunday 03:00/06:00/06:30 PT).

**Defensive idempotent calls** — append after `protectBankCoinCells()` at line 91, in the same out-of-lock style (warning-only `Range.protect` doesn't acquire the document lock):

```typescript
// Append after installTriggers.ts:91
protectBankCoinCells();          // existing
protectBankToonName();           // NEW — new helper in migrations.ts
hideAllSystemTabs();             // NEW — new helper in migrations.ts
```

**Test pattern** (installTriggers.test.ts:19-27): assert trigger count bumps from 7 → 10 and the handler-names array includes the three new entries.

---

### `apps-script/src/triggers/onOpen.ts` (MODIFIED — extend in place)

**Analog:** itself (onOpen.ts:7-26).

**Menu-item insertion pattern** (insert between lines 20-21):

```typescript
// onOpen.ts:7-26 — extend
SpreadsheetApp.getUi()
  .createMenu('SquireBot')
  // … existing items …
  .addItem('Set Character Info…', 'showCharInfoSidebar')
  .addItem('Set Bank Coin…', 'showBankCoinSidebar')
  .addItem('Search…', 'showSearchSidebar')             // NEW Phase 5
  .addItem('Evict Guildie…', 'showEvictionSidebar')    // NEW Phase 5
  .addItem('Set Theme…', 'showThemePickerModal')
  // …
  .addToUi();
```

Trailing `…` convention from existing items (lines 19-21) — copy verbatim.

---

### `apps-script/src/lib/searchIndex.ts` (library, CRUD + transform)

**Analog:** `apps-script/src/lib/sheet-helpers.ts` (general lib shape) + `apps-script/src/triggers/monitorCellCount.ts:21-30` (sheet iteration via `ss.getSheets().filter(name → startsWith('inv:'))`) + `apps-script/src/lib/migrations.ts:36-39` (outcome enum pattern).

**Sheet-iteration filter** (clone monitorCellCount.ts:25-29 with prefix filter):

```typescript
// derive from monitorCellCount.ts:25-29
const ss = getActiveSpreadsheet();
const invSheets = ss.getSheets().filter((s) => s.getName().startsWith('inv:'));
for (const sheet of invSheets) {
  const charName = sheet.getName().slice('inv:'.length);
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) continue;
  const values = sheet.getRange(2, 1, lastRow - 1, 4).getValues();  // A=Location B=Name C=ID D=Count
  // project to [string, string, number, number][]
}
```

**CacheService access pattern** (NEW — `CacheService.getDocumentCache().getAll(keys)` for single round-trip read; `.putAll(map, 60)` for bulk write). No existing analog uses this method — see RESEARCH §Pattern 2. The mock at test-helpers.ts:332-337 (current stub) MUST be extended for these methods.

**Hand-rolled Levenshtein** (per RESEARCH §Pattern 3) — no in-repo analog; place in this file as a local helper. ~25-line Wagner-Fischer DP. Tested independently in `searchIndex.test.ts`.

**Tier-mismatch risk:** This file is the natural home for `runSearch`, `buildInvCache(charName)`, `searchInvCache(query, charFilter, slotFilter)`, `didYouMean(query, items)`, `pushRecentSearch(q)`, `getRecentSearches()`. Do NOT inline any of these in `showSearchSidebar.ts`.

---

### `apps-script/src/lib/archive.ts` (library, transform)

**Analog:** `apps-script/src/lib/sheet-helpers.ts:9-14` (`getOrCreateSheet`) for lazy `_archive` tab creation + `apps-script/src/lib/migrations.ts:55-74` for the lock-guarded mutation envelope.

**Lazy tab creation pattern** (sheet-helpers.ts:9-14):

```typescript
// sheet-helpers.ts:9-14 — clone for _archive
export function getOrCreateSheet(name: string): GoogleAppsScript.Spreadsheet.Sheet {
  const ss = getActiveSpreadsheet();
  const existing = ss.getSheetByName(name);
  if (existing) return existing;
  return ss.insertSheet(name);
}
```

`moveCharToArchive(charName, reason)` calls `getOrCreateSheet('_archive')`, copies the char's `inv:<Char>` + `spell:<Char>` rows in, then `hideSheet()` the source tabs (or deletes — TBD by planner; archive table is the safer choice). Also flips `_char_owner.is_hidden=TRUE` for that char.

**Lock-guarded envelope** (migrations.ts:55-74):
```typescript
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) throw new Error('moveCharToArchive: could not acquire document lock within 30s');
try { /* writes */ } finally { lock.releaseLock(); }
```

---

### `apps-script/src/lib/migrations.ts` (MODIFIED — add helpers, no schema bump)

**Analog:** itself (migrations.ts:125-155 `protectBankCoinCells`).

**`protectBankToonName` — direct clone of `protectBankCoinCells` for a single cell**:

```typescript
// migrations.ts:125-155 — pattern to clone, swap single cell for the four
export function protectBankToonName(): void {
  const sheet = getActiveSpreadsheet().getSheetByName('_meta');
  if (!sheet) {
    log('warn', 'protectBankToonName', { skipped: '_meta_missing' });
    return;
  }
  const meta = readMetaRows('_meta');
  const row = meta.find((r) => r.key === 'bank_toon_name');
  if (!row) { log('info', 'protectBankToonName', { skipped: 'row_not_set' }); return; }
  const cell = sheet.getRange(row.rowIndex, 2);
  const cellA1 = cell.getA1Notation();
  const description = 'SquireBot bank toon name — edit via SquireBot menu';
  const existing = sheet.getProtections(SpreadsheetApp.ProtectionType.RANGE)
    .find((p) => p.getRange().getA1Notation() === cellA1
              && p.getDescription() === description);
  if (existing) { log('info', 'protectBankToonName', { skipped: 'already_protected' }); return; }
  cell.protect().setDescription(description).setWarningOnly(true);
  log('info', 'protectBankToonName', { added: 1 });
}
```

Warning-only rationale — copy comment from migrations.ts:143-148 verbatim.

**`hideAllSystemTabs` — NEW helper** (no exact analog; idempotent SpreadsheetApp pattern):

```typescript
export function hideAllSystemTabs(): void {
  const ss = getActiveSpreadsheet();
  let hidden = 0;
  for (const sheet of ss.getSheets()) {
    if (!sheet.getName().startsWith('_')) continue;
    if (sheet.isSheetHidden && sheet.isSheetHidden()) continue;  // idempotent
    sheet.hideSheet();
    hidden++;
  }
  log('info', 'hideAllSystemTabs', { hidden });
}
```

---

### `apps-script/src/lib/eq-constants.ts` (MODIFIED — add `INVENTORY_SLOTS`)

**Analog:** itself (eq-constants.ts:8-12 `CLASSES`, eq-constants.ts:44-48 `RACES`).

**Pattern to clone** (eq-constants.ts:8-12):

```typescript
// eq-constants.ts:8-12 — clone the as-const + type alias pattern
export const INVENTORY_SLOTS = [
  'HEAD', 'CHEST', 'EAR1', 'EAR2', 'ARMS', 'WRIST1', 'WRIST2',
  'LEGS', 'FEET', 'HANDS', 'NECK', 'FINGER1', 'FINGER2',
  'SHOULDERS', 'BACK', 'WAIST', 'RANGE', 'AMMO', 'PRIMARY', 'SECONDARY',
  'FACE',
  // Bag/general slots (TBD: scrape distinct Location values from real inv:* tabs)
] as const;
export type InventorySlot = typeof INVENTORY_SLOTS[number];
```

Planner's discretion (CONTEXT D-01 + Claude's Discretion): either hardcode P99-known slots above OR scrape distinct `Location` values from real `inv:*` tabs at first sidebar open. Hardcoded list is simpler for tests; use as default.

---

### `apps-script/src/Code.ts` (MODIFIED — add re-exports)

**Analog:** itself (Code.ts:1-46).

**Re-export pattern** (Code.ts:21-30):

```typescript
// Code.ts:21-30 — clone the named-import + re-export shape
import {
  showSearchSidebar,
  runSearch,
  getSearchInitialData,
  pushRecentSearch,
} from './triggers/showSearchSidebar';
import {
  showEvictionSidebar,
  getEvictionEmails,
  previewEviction,
  commitEviction,
} from './triggers/showEvictionSidebar';
import { weeklySchemaHealthcheck } from './triggers/weeklySchemaHealthcheck';
import { weeklyStaleCharArchive } from './triggers/weeklyStaleCharArchive';
import { weeklyEvictionArchive } from './triggers/weeklyEvictionArchive';
import { protectBankToonName, hideAllSystemTabs } from './lib/migrations';

export {
  // … existing …
  showSearchSidebar, runSearch, getSearchInitialData, pushRecentSearch,
  showEvictionSidebar, getEvictionEmails, previewEviction, commitEviction,
  weeklySchemaHealthcheck, weeklyStaleCharArchive, weeklyEvictionArchive,
  protectBankToonName, hideAllSystemTabs,
};
```

**Coupled change in `build.mjs:21-52`** — every name above MUST also be added to `TRIGGER_GLOBALS`. The CI assertion at build.mjs:54-122 catches divergence — see build.mjs:18-20 lesson comment.

---

### `apps-script/src/sidebars/searchSidebar.html` & `evictionSidebar.html` (NEW directory)

**Analog:** `apps-script/src/triggers/showCharInfoSidebar.ts:135-217` (inline HTML+CSS+JS template).

**NOTE — architectural shift:** UI-SPEC promotes the inline template (currently embedded in the `.ts` file as a template literal) to a sibling `.html` file under a new `sidebars/` directory. The TS sidebar trigger now reads the HTML via `HtmlService.createHtmlOutputFromFile('sidebars/searchSidebar')` (Apps Script reads `.html` files from the clasp-pushed project). Verify clasp push surfaces `sidebars/*.html`; if it flattens directories (it does — clasp pushes all `.html` siblings of `.gs`), the include path becomes `searchSidebar` (no directory prefix). Planner: confirm clasp behavior during scaffold.

**Existing template body to clone** (showCharInfoSidebar.ts:135-217 — copy structure, replace content):

```html
<!-- showCharInfoSidebar.ts:136-146 — outer shell, swap to use theme vars -->
<div style="font-family:Arial,sans-serif;padding:10px;font-size:13px">
  <h3 style="margin-top:0">Character Info</h3>
  <p style="color:#666;font-size:11px">Set class/level/race…</p>
  <table style="width:100%;border-collapse:collapse;margin-top:10px">
    <thead><tr><th>Char</th>…</tr></thead>
    <tbody id="charBody"><tr><td colspan="4">Loading…</td></tr></tbody>
  </table>
  <button id="saveBtn" onclick="save()" disabled>Save</button>
  <div id="msg"></div>
  <script>
    // google.script.run wiring — see lines 152, 201-213
  </script>
</div>
```

Phase 5 swaps inline `font-family:Arial,sans-serif` → `font-family: var(--font-body, Arial, sans-serif)` and inline colors → CSS custom properties per UI-SPEC §Color. The state machine (loading/results/no-results/error) follows UI-SPEC §Search sidebar — states.

---

### `apps-script/src/__tests__/test-helpers.ts` (MODIFIED — extend mocks)

**Analog:** itself (test-helpers.ts:332-344 CacheService + HtmlService stubs).

**CacheService mock is currently insufficient** (test-helpers.ts:332-337):

```typescript
// test-helpers.ts:332-337 — current stub: get/put are no-ops
(globalThis as Record<string, unknown>).CacheService = {
  getDocumentCache: () => ({
    get: () => null,
    put: () => {},
  }),
};
```

For Phase 5 we need a real Map-backed mock with `getAll(keys)`, `putAll(map, ttl)`, and `remove(key)` so `searchIndex.test.ts` can assert cache writes, TTL respect (manual time-advance), and invalidation. Add to `MockState`:

```typescript
// extension to test-helpers.ts MockState — derive from existing properties Map pattern
cache: Map<string, { value: string; expiresAt: number }>;
```

And replace the CacheService stub with one that reads from `state.cache` honoring `expiresAt` against `Date.now()` (tests can `vi.setSystemTime` to advance).

**`Sheet.hideSheet` + `Sheet.isSheetHidden` mock additions** — currently absent. Add to the `makeSheetProxy` in test-helpers.ts:49-172 (lines 50-172 is the proxy factory):

```typescript
// extension to makeSheetProxy (test-helpers.ts:50-172)
isSheetHidden: () => Boolean((s as FakeSheet & { _hidden?: boolean })._hidden),
hideSheet: () => { (s as FakeSheet & { _hidden?: boolean })._hidden = true; },
showSheet: () => { (s as FakeSheet & { _hidden?: boolean })._hidden = false; },
getSheetId: () => s.name.split('').reduce((h, c) => h * 31 + c.charCodeAt(0), 0) | 0,
```

---

### Test files (`__tests__/*.test.ts`)

**Analogs:**

- `searchIndex.test.ts` ← clones `monitorCellCount.test.ts:1-78` (sheet-iteration assertions) + `charInfoSidebar.test.ts:1-49` (returns shape assertions)
- `weeklySchemaHealthcheck.test.ts` ← exact clone of `monitorCellCount.test.ts` line-for-line, swapping cell-count for tab-presence checks
- `archive.test.ts` ← clones `migrations.test.ts` (lock-guarded helper tests; assert `_archive` is created, char rows move, source tabs marked hidden)
- `showEvictionSidebar.test.ts` ← clones `charInfoSidebar.test.ts:1-90` (sidebar reader + writer tests)

**Test scaffolding pattern** (charInfoSidebar.test.ts:1-15, 51-58 — copy verbatim):

```typescript
import { describe, it, expect, beforeEach } from 'vitest';
import { /* function under test */ } from '../triggers/foo';
import { resetMocks, makeSheet, type MockState } from './test-helpers';

describe('foo', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    state.sheets.set('_meta', makeSheet('_meta', ['key', 'value'], [['schema_version', '3']]));
    // … seed other tabs as needed
  });

  it('handles missing dependency tab gracefully', () => { /* … */ });
  it('writes expected output rows', () => { /* … */ });
  it('is idempotent on re-run', () => { /* … */ });
});
```

---

### Jekyll docs surface (NO ANALOG)

The `/docs/` Jekyll site is greenfield — no existing in-repo file analogs. RESEARCH §Pattern 7 + UI-SPEC §Onboarding Site Visual Contract are the sources of truth.

| File | Source of truth |
|------|-----------------|
| `docs/_config.yml` | RESEARCH §Standard Stack — Static Site Stack: `remote_theme: pages-themes/cayman@v0.2.0`, `plugins: [jekyll-remote-theme]`, `title: SquireBot` |
| `docs/index.md` | UI-SPEC §Copywriting Contract — Onboarding site row `index.md` (1 paragraph + 2 links) |
| `docs/install.md` | UI-SPEC §Copywriting Contract + RESEARCH §Pitfall P9 (GIF placement) — chronological 5-step with PNG/GIF |
| `docs/troubleshooting.md` | UI-SPEC §Copywriting Contract — symptom-keyed `<h2>` sections |
| `docs/dev.md` | UI-SPEC §Onboarding Site Visual Contract — flat link list into existing `docs/oauth-setup.md`, `docs/apps-script-deploy.md`, `docs/build-and-install.md` |
| `docs/assets/*.png`, `smartscreen.gif` | UI-SPEC §Onboarding — alt text on every image; GIF ≤5 MB |

`README.md` modification (D-12): shrink existing content; planner must move long-form tech overview to `docs/dev.md` BEFORE stripping it from README. Use git history reference to existing README at HEAD `9319c6b` to verify which sections move where.

---

## Shared Patterns

### Authentication / Authorization
**Not applicable in Phase 5** — Apps Script sidebars run as the active user; OAuth scope is workbook-bound. No new auth surface.

### Error handling
**Source:** `apps-script/src/triggers/monitorCellCount.ts:40-48`
**Apply to:** All new time-driven triggers (`weeklySchemaHealthcheck`, `weeklyStaleCharArchive`, `weeklyEvictionArchive`) AND any sidebar write that needs to surface to the watcher (eviction commits).

```typescript
// monitorCellCount.ts:40-48 — canonical _meta.last_error envelope
const err = {
  at: new Date().toISOString(),
  where: '<function-name>',
  kind: '<short_snake_case_kind>',
  detail: `<human-readable specifics>`,
};
const errJson = JSON.stringify(err);
writeMetaRow('_meta', 'last_error', errJson);
writeMetaRow('_status', 'last_error', errJson);
```

Watcher heartbeat reader (`internal/heartbeat/heartbeat.go` — already deployed) surfaces this to the tray red state without further code on the Go side.

### LockService guard envelope
**Source:** `apps-script/src/triggers/showCharInfoSidebar.ts:99-126` (also `showBankCoinSidebar.ts:54-65`, `migrations.ts:55-74`)
**Apply to:** Every write path in Phase 5 — `commitEviction`, `pushRecentSearch` (if it writes properties), `moveCharToArchive`, `protectBankToonName` (NO — protect() does not need the lock; see migrations.ts:102-104 comment), `hideAllSystemTabs` (NO — same reason).

```typescript
// showCharInfoSidebar.ts:99-126 — canonical lock envelope
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) throw new Error('<op>: lock_busy');
try {
  // all writes here
  log('info', '<op>', { /* metrics */ });
  return { /* result */ };
} finally {
  lock.releaseLock();
}
```

### `google.script.run` server callback
**Source:** `apps-script/src/triggers/showCharInfoSidebar.ts:152, 201-213`
**Apply to:** Both new sidebars (search + eviction).

Always pair `.withSuccessHandler` AND `.withFailureHandler`. Never call a server function bare. Surface errors via the `#msg` element with red color (showCharInfoSidebar.ts:180-184).

### `Range.protect().setWarningOnly(true)` idiom
**Source:** `apps-script/src/lib/migrations.ts:125-155` (`protectBankCoinCells`)
**Apply to:** `protectBankToonName` only.

Key rules from existing implementation:
- Idempotency via description-string match (migrations.ts:139-142).
- Always `setWarningOnly(true)` — strict protection is invisible to the script owner (migrations.ts:143-148 comment).
- Skip-if-row-missing (migrations.ts:135-136) — bank_coin/bank_toon_name rows may not exist on first install.

### Theme-aware HTML rendering (NEW Phase 5 baseline)
**Source:** `apps-script/src/lib/themes.ts:35-67` (THEMES registry) + `apps-script/src/lib/themes.ts:75-80` (`getActiveTheme`)
**Apply to:** `showSearchSidebar`, `showEvictionSidebar` (per UI-SPEC §Color).

Inject theme tokens as CSS custom properties on `:root` at server-render time; emit no style block when `THEMES[key] === null` (sheets-default). The existing `showCharInfoSidebar.ts` + `showBankCoinSidebar.ts` use hardcoded `Arial 13px` and are explicitly out-of-scope for retrofit (CONTEXT — deferred theme picker tile UI does not apply here).

### Logging
**Source:** `apps-script/src/lib/log.ts` (helper used throughout — e.g. `monitorCellCount.ts:49, 52`).
**Apply to:** All new triggers and lib functions.

```typescript
import { log } from '../lib/log';
log('info', 'runSearch', { query, charFilter, slotFilter, matched: results.length, ms });
log('warn', 'runSearch', { skipped: 'cache_busy' });
```

### Build-time `TRIGGER_GLOBALS` sync
**Source:** `apps-script/build.mjs:21-52` + `apps-script/src/Code.ts:36-46`
**Apply to:** Every new top-level callable added in Phase 5 (sidebar openers, google.script.run callbacks, time-driven triggers, menu-callable helpers).

The CI assertion at build.mjs:54-122 catches drift — see build.mjs:18-20 comment about the Phase 3 `migrateToV2` lesson. Adding to one file without the other = build failure.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `docs/_config.yml` | Jekyll config | declarative | First Jekyll surface in repo — RESEARCH §Standard Stack is the spec |
| `docs/index.md`, `install.md`, `troubleshooting.md`, `dev.md` | Markdown content | static | First Jekyll surface — UI-SPEC §Copywriting Contract is the spec |
| `docs/assets/smartscreen.gif`, `docs/assets/*.png` | static asset | n/a | Pre-recorded; no code analog |
| `apps-script/src/sidebars/searchSidebar.html`, `evictionSidebar.html` | template file | inline | First `.html` extracted-from-TS surface (existing sidebars embed HTML in `.ts`). UI-SPEC §Architecture note locks this as the new baseline. Pattern body still clones `showCharInfoSidebar.ts:135-217`. |

---

## Metadata

**Analog search scope:** `apps-script/src/triggers/`, `apps-script/src/lib/`, `apps-script/src/tabs/`, `apps-script/src/__tests__/`, `apps-script/build.mjs`, `apps-script/src/Code.ts`
**Files scanned:** 12 (`showCharInfoSidebar.ts`, `showBankCoinSidebar.ts`, `monitorCellCount.ts`, `onOpen.ts`, `installTriggers.ts`, `migrations.ts`, `sheet-helpers.ts`, `eq-constants.ts`, `themes.ts`, `Code.ts`, `build.mjs`, `test-helpers.ts` + 3 test files for shape)
**Pattern extraction date:** 2026-05-10
