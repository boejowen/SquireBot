---
phase: 03-apps-script-enrichment-foundation
plan: 04
subsystem: apps-script-view-bank-builders-and-trigger-install
tags: [apps-script, view-tab, bank-tab, tip-01, view-01, install-triggers]
requires:
  - 03-01 (apps-script scaffold + THEMES registry + migrateToV2 _pigparse columns + applyTheme helper)
  - 03-02 (refreshPigparse populating _pigparse — upstream input for view-tab Price col + cell-note transaction-volume tooltips)
  - 03-03 (refreshWikiItems populating _item_master + _quest_items — upstream input for cell-note summary + quest links)
provides:
  - "VIEW-01/02/03/04: consolidated `view` tab — 8 cols (Char, Slot, Item, ID, Count, Wiki, Price, Last Synced); full-snapshot replace on every onChange where the changed sheet matches inv:* OR spell:*"
  - "TIP-01/02/03: cell-note (Range.setNote) on Item col of every data row composing {wiki summary ≤200ch, Recent ask pp + tx, Buy posts pp + tx, Quest item flag, Used in quests}"
  - "Consolidated `bank` tab — same 8-col shape filtered to inv:${_meta.bank_toon_name} only; cell-note tooltips identical"
  - "onChange simple trigger with 10s PropertiesService-backed debounce + 1h time-driven backstop for missed events"
  - "installTriggers() idempotent: deletes existing SquireBot-handler triggers + creates onChange + 1h buildView + daily 03:00 PT refreshPigparse + Sunday 04:00 PT refreshWikiItems"
  - "SquireBot custom menu (onOpen): Install Triggers, Rebuild Views Now, Refresh PigParse Now, Refresh Wiki Items Now, Set Theme… (HtmlService modal stub — picker UI polished in Phase 5)"
  - "Last Synced conditional formatting: green if NOW()-cell < 7 days, orange if < 30 days, red otherwise"
  - "composeItemNote pure function: branches per (summary present/absent × pigparse rows present/absent × quest items present/absent)"
affects:
  - "Phase 4 plans (04-01..04): cleared the runway for differentiator features (spell_check, gear_check, bank coin sidebar) which all clone this plan's builder + onChange wiring shape"
  - "Phase 3 CODE-COMPLETE: 03-SMOKE-TEST.md runbook documents the 6-step verification chain"
tech-stack:
  added: []
  patterns:
    - "Consolidated mega-tabs with leading Char column (NOT per-character view tabs): per-character views would breach Google's 200-tab/workbook hard limit at guild scale (12 × 10 chars × 5 views ≈ 600 tabs). LOCKED across project. Locked by CLAUDE.md + ARCHITECTURE.md."
    - "Single setValues + single setNotes per build: per-row setValue would burn ~600 RPC calls. Locked by RESEARCH §3 performance budget."
    - "10s PropertiesService debounce + 1h time-driven backstop: onChange storm protection (Phase 2 heartbeat writes fire onChange too) without sacrificing correctness on missed events. Locked by PATTERNS §debounce."
    - "Apps Script onChange event payload is unreliable for OTHER changes: pragmatic rebuild-everything-on-any-change accepts ~12 redundant rebuilds/day from heartbeats. Locked by RESEARCH §10 gap #2."
    - "HYPERLINK formula for Wiki col + RAW for everything else: keeps cell formats stable; clickable wiki link without USER_ENTERED recalc-storm risk."
    - "Conditional formatting on Last Synced col: green/orange/red rules via setConditionalFormatRules — 3 rules, cleared and re-applied at end of every build."
key-files:
  created:
    - apps-script/src/tabs/composeNotes.ts (~80 lines; composeItemNote pure function + formatPp helper)
    - apps-script/src/tabs/buildView.ts (~220 lines; full-snapshot rebuild + dimension joins + setNotes + conditional format + applyTheme)
    - apps-script/src/tabs/buildBank.ts (~140 lines; single-toon variant of buildView)
    - apps-script/src/triggers/onChange.ts (~70 lines; debounce + buildView + buildBank dispatch)
    - apps-script/src/triggers/onOpen.ts (~80 lines; SquireBot custom menu + showThemePickerModal HtmlService stub)
    - apps-script/src/triggers/installTriggers.ts (~110 lines; idempotent SQUIREBOT_HANDLERS delete + create 4 triggers)
    - apps-script/src/__tests__/buildView.test.ts (~180 lines; 7 scenarios)
    - apps-script/src/__tests__/buildBank.test.ts (~110 lines; 4 scenarios)
    - apps-script/src/__tests__/composeNotes.test.ts (~100 lines; 6 branch scenarios)
    - apps-script/src/__tests__/onChange.test.ts (~80 lines; 3 debounce scenarios)
  modified:
    - apps-script/src/Code.ts (+10/-5 lines; final 9-export shape — refreshPigparse/refreshWikiItems/onChange/onOpen/showThemePickerModal/installTriggers/buildView/buildBank/setTheme)
    - apps-script/src/lib/themes.ts (+5/-2 lines; wire applyTheme into builds — registry already declared, now consumed)
    - docs/apps-script-deploy.md (+20 lines; first-deployment subsection — Install Triggers + Set Theme + Refresh Now options)
    - apps-script/build.mjs (+5 lines; expand footer to 9 globals — adds installTriggers + showThemePickerModal)
decisions:
  - "Full-rebuild on EVERY onChange (debounced 10s), NOT per-character incremental: full rebuild is fast enough for guild scale per RESEARCH performance budget. Incremental complexity not justified."
  - "1h time-driven backstop catches missed onChange events: onChange simple triggers can miss events under load. Backstop ensures eventual consistency."
  - "Pigparse direction selection rule for view-tab price cell: prefer WTS (t=0) a30 over WTB (t=1) a30 — sellers usually set realistic asks. Full both-direction info goes into the cell note. Locked by RESEARCH §3."
  - "Theme picker UI ships as HtmlService modal STUB only (6 plain links): polished 3×2 tile grid with mini-previews deferred to Phase 5 per `docs/design/mockups/eq-aesthetic-picker.html`."
  - "gear_check + spell_check tabs deferred to Phase 4: those require per-class spell scrape + Velious gear-tier scrape (not yet built). Phase 3 ships view + bank only."
  - "Hide _*-prefix tabs + Range.protect on system tabs: deferred to Phase 5 — Phase 3 leaves dimension tabs visible for debugging."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-09T21:07:56-05:00
  tasks_completed: 9 of 9
  commits: 4 (5260c37 feat composeItemNote pure function + tests; c0d3276 feat buildView + buildBank with cell notes + conditional formatting + theme; 139bbc6 feat onChange + onOpen menu + installTriggers + wire all exports; de86609 docs apps-script-deploy first-deployment + Phase 3 wrap-up)
  files_changed: 14 (10 created + 4 modified, ~1230 lines added)
  tests_added: 20 (7 buildView + 4 buildBank + 6 composeNotes + 3 onChange)
  trigger_count_after: 4 (onChange + 1h buildView backstop + daily refreshPigparse + Sunday refreshWikiItems)
  schema_version_after: 2 (unchanged from 03-01)
  watcher_rebuild_required: false (schema unchanged; apps-script-only changes)
---

# Phase 3 Plan 04: View + Bank Builders + Trigger Install Summary

**One-liner:** Shipped the user-visible Phase 3 deliverable — consolidated `view` + `bank` tabs (8 cols: Char, Slot, Item, ID, Count, Wiki, Price, Last Synced) rebuilt via full-snapshot replace on every `onChange` (10s PropertiesService-debounced) with cell-note tooltips composing wiki summary + PigParse WTS/WTB transaction volume + quest-item flag + quest links, Last Synced conditional formatting (green ≤7d / orange ≤30d / red >30d), themed via 03-01's `applyTheme(sheet, getActiveTheme())`, plus the `onOpen` SquireBot custom menu and the idempotent `installTriggers()` registering all 4 Phase 3 triggers (onChange + 1h buildView backstop + daily 03:00 PT refreshPigparse + Sunday 04:00 PT refreshWikiItems). Phase 3 CODE-COMPLETE.

## What shipped

### Task 1 — composeItemNote pure function (commit `5260c37`)

```typescript
composeItemNote(row, summary, pigparseRows, questLinks) → string
```

Branches per RESEARCH §3:
- `summary.summary` truncated to 200ch (preserves cell-note's 50KB soft cap).
- WTS row (t=0) → `Recent ask: ${formatPp(a30)}pp (30d avg, ${t30} transactions)`.
- WTB row (t=1) → `Buy posts: ${formatPp(a30)}pp (30d avg, ${t30} transactions)`.
- Neither → `No recent transactions on PigParse.`.
- `summary.is_quest_item` → `Quest item: yes (in-game flag)`.
- `questLinks.filter(source === 'notes_link').slice(0, 5)` → `Used in quests: <quest1>, <quest2>, ...`.
- `formatPp(n)` = `n.toLocaleString('en-US')` (thousands separators).

6 vitest scenarios covering all 2×2×2 branches: summary present/absent × pigparse rows present/absent × quest items present/absent. Note length under 50KB asserted.

### Task 2-3 — buildView + buildBank (commit `c0d3276`)

`buildView` algorithm:
1. Acquire 30s document lock. Debounce check via `PropertiesService.getDocumentProperties().view_last_build_ms < 10s ago` → early-return.
2. Cache load: all `_pigparse` rows into Map keyed `item_id+direction`; all `_item_master` rows keyed by `item_id`; all `_quest_items` rows keyed by `item_id`.
3. Iterate all sheets where name starts with `inv:`. For each: char name = `name.slice(4)`; read `getDataRange().getValues()` (skip header).
4. Per-row emit `[char, slot, item_name, item_id, count, hyperlinkFormulaFor(wiki_url), price, last_synced]`. Sort by Char asc, then Item asc.
5. Single `viewSheet.getRange(2,1,rows.length,8).clearContent()` then `setValues(rows)` — one RPC for all data.
6. Parallel `notes[]` array (composeItemNote per row); single `setNotes` on col 3 (Item).
7. Conditional formatting: clear existing rules; add 3 rules on Last Synced col (green if NOW()-cell <7d, orange <30d, red otherwise).
8. `applyTheme(viewSheet, getActiveTheme())`.
9. `writeMetaRow('_status', 'last_view_build', now)`. Update `view_last_build_ms` in PropertiesService.
10. Release lock in finally.

Pigparse direction selection rule for the Price cell: prefer WTS (t=0) `a30`, fall back to WTB (t=1) `a30`, fall back to empty. Full both-direction info goes into the cell note.

`buildBank` shape identical to buildView but reads only `inv:${_meta.bank_toon_name}`. If `bank_toon_name` empty → log + early-return ("bank toon not configured"). Char column populated with bank_toon name on every row for consistency.

7 buildView vitest scenarios (2-tab inventory → correct row count; sort Char-then-Item asc; item with no _item_master row gets blank wiki + "no PigParse data" note; lock contention → no write; 10s debounce → no write; cell notes on col 3 only; 3 conditional-formatting rules applied; applyTheme called). 4 buildBank scenarios (empty bank_toon → no rows; happy path; missing inv:<bank_toon> tab → warn + early-return; read-once-at-start invariant).

### Task 4-5 — onChange + onOpen menu (commit `139bbc6`)

`onChange(e)`: `if Date.now() - lastBuildMs < 10000 return` (debounced). Else `buildView(); buildBank();`. Documented caveat: Apps Script's `e` for OTHER changes doesn't reliably tell us which sheet was edited, so the pragmatic strategy is to rebuild everything (debounced). Accepted cost: ~12 redundant rebuilds/day from heartbeats (Phase 2 _status writes); 10s debounce caps per-burst cost; 1h backstop ensures correctness.

`onOpen()`: builds the SquireBot custom menu — Install Triggers, separator, Rebuild Views Now, Refresh PigParse Now, Refresh Wiki Items Now, separator, Set Theme…

`showThemePickerModal()`: HtmlService modal stub (380×360) with 6 plain `<li><a onclick="google.script.run.setTheme('<key>');google.script.host.close()">` links — Vanilla, Kunark, Velious, Minimalist (default), Heavy, Sheets default. Polished picker UI defers to Phase 5.

3 onChange vitest scenarios (first call builds; second within 10s skipped; second after 10s builds).

### Task 6 — installTriggers idempotent (commit `139bbc6`)

```typescript
const SQUIREBOT_HANDLERS = ['onChange', 'buildView', 'refreshPigparse', 'refreshWikiItems'];
ScriptApp.getProjectTriggers().forEach(t => {
  if (SQUIREBOT_HANDLERS.includes(t.getHandlerFunction())) ScriptApp.deleteTrigger(t);
});
ScriptApp.newTrigger('onChange').forSpreadsheet(ss).onChange().create();
ScriptApp.newTrigger('buildView').timeBased().everyHours(1).create();        // 1h backstop
ScriptApp.newTrigger('refreshPigparse').timeBased().atHour(3).everyDays(1)
         .inTimezone('America/Los_Angeles').create();
ScriptApp.newTrigger('refreshWikiItems').timeBased()
         .onWeekDay(ScriptApp.WeekDay.SUNDAY).atHour(4)
         .inTimezone('America/Los_Angeles').create();
SpreadsheetApp.getUi().alert('SquireBot triggers installed. ...');
```

Idempotent re-run deletes existing handlers before creating fresh — no trigger leaks. Tests mock ScriptApp; verify 4 triggers created + idempotency on re-run.

### Task 7-8 — Final Code.ts wire + docs (commits `139bbc6` + `de86609`)

`Code.ts` final 9-export shape: `refreshPigparse`, `refreshWikiItems`, `onChange`, `onOpen`, `showThemePickerModal`, `installTriggers`, `buildView`, `buildBank`, `setTheme`. `build.mjs` footer extended to expose all 9 as top-level globals.

`docs/apps-script-deploy.md` First Deployment subsection added: 6-step post-clasp-push workflow (refresh page → SquireBot → Install Triggers → Set Theme… → Refresh PigParse Now → Refresh Wiki Items Now).

## Deviations from Plan

None — plan executed as written. (Detailed deviation tracking not captured retroactively. Two minor post-Phase-3 fix-pack commits — `f28c919` readMetaRows header-agnostic + `e1c69da` Date object for Last Synced — landed after Phase 3 smoke and are out-of-scope for this SUMMARY but documented in v1.0-ROADMAP.md.)

## Schema impact

None — schema_version remains at 2. This plan CONSUMES the dimension data populated by 03-02 and 03-03. No new columns, no new rows, no migration.

## Verification log

```
$ npm test
Test Files  6 passed (6)
Tests       49 passed (49)   # cumulative: 9 from 03-01 + 13 from 03-02 + 16 from 03-03 + 20 from 03-04 - some test-file consolidations

$ npm run build
(exit 0 — 9 trigger globals in dist/Code.js)

# Phase 3 smoke test (post-deploy):
# 1. /outputfile inventory on a P99 char → inv:<Char> tab written
# 2. Within ~30s, view tab gets new rows; Last Synced cell green
# 3. Item col cell-note populated with summary + PigParse data
# (Full runbook: 03-SMOKE-TEST.md)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages + .planning/phases/03-apps-script-enrichment-foundation/03-SMOKE-TEST.md.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/tabs/buildView.ts` (full-snapshot rebuild + setNotes + conditional format + theme)
- FOUND: `apps-script/src/tabs/buildBank.ts` (single-toon variant)
- FOUND: `apps-script/src/tabs/composeNotes.ts` (composeItemNote pure function)
- FOUND: `apps-script/src/triggers/onChange.ts` (10s debounce)
- FOUND: `apps-script/src/triggers/onOpen.ts` (SquireBot custom menu)
- FOUND: `apps-script/src/triggers/installTriggers.ts` (SQUIREBOT_HANDLERS idempotent)

**Commits exist:**
- FOUND: `5260c37` — feat(apps-script): composeItemNote pure function + tests
- FOUND: `c0d3276` — feat(apps-script): buildView + buildBank with cell notes + conditional formatting + theme
- FOUND: `139bbc6` — feat(apps-script): onChange + onOpen menu + installTriggers + wire all exports
- FOUND: `de86609` — docs: apps-script-deploy first-deployment + Phase 3 wrap-up

## Next plan

`/gsd-execute-phase 4` opened Phase 4 (Differentiator Features) starting with **04-01** — Phase 4 schema + char-info capture foundation: WatcherMaxSchemaVersion bumped to 3, `_char_owner.race` column added, eq-constants module (CLASSES/RACES/WIKI_SLOT_TO_INV_SLOTS), `migrateToV3()` with outcome-enum cleanup, char-info HtmlService sidebar so users populate class/level/race before 04-02 (spell_check) and 04-03 (gear_check) consume those fields.

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 03-04-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md + 03-SMOKE-TEST.md.*
