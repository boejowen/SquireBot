---
phase: 04-differentiator-features
plan: 01
subsystem: apps-script-schema-v3-migration-and-char-info-sidebar
tags: [apps-script, schema-migration, check-01, char-info-sidebar, eq-constants]
requires:
  - 03 (Phase 3 CODE-COMPLETE — apps-script scaffold, migrateToV2, THEMES registry, dist/Code.js bundle pipeline)
  - 03-01 (MigrationOutcome enum + appendColumns helper reused for migrateToV3)
provides:
  - "CHECK-01 foundation: _char_owner.race column added — schema v=3 — so gear_check (04-03) can filter Iksar-tier rows for chars with race='IKS'"
  - "migrateToV3() idempotent: 1 column added to _char_owner; outcome-enum renamed to version-agnostic shape (noop_already_current / migrated / noop_unsupported_version)"
  - "eq-constants module: CLASSES (14 abbrev), RACES (14 abbrev), CLASS_DISPLAY_TO_ABBREV (Bard→BRD etc.), WIKI_SLOT_TO_INV_SLOTS (17 keys — many-to-many pair-slot mapping for gear_check matching), isClassAbbrev/isRaceAbbrev type guards"
  - "showCharInfoSidebar + getCharsForForm + saveCharInfo: 360px HtmlService sidebar with class/level/race dropdowns; lock-guarded upsert (in-place update only — watcher owns row creation)"
  - "build.mjs CI assertion: fails build if Code.ts exports diverge from TRIGGER_GLOBALS array (prevents the migrateToV2-missing-from-globals class of bug observed during Phase 3 deploy)"
  - "WatcherMaxSchemaVersion = 3 (Go side) — coordinated with v0.4.0 watcher ship before any clasp push"
affects:
  - "04-02 (refreshWikiSpells + buildSpellCheck): consumes char_owner.class + char_owner.level to filter spell_check rows"
  - "04-03 (refreshWikiGearTier + buildGearCheck): consumes char_owner.class + char_owner.level + char_owner.race for tier filtering (Iksar-tier rows ONLY for race='IKS')"
  - "04-04 (Phase 4 wrap): inherits the CI assertion that prevents Phase 3's missing-global bug class"
tech-stack:
  added: []
  patterns:
    - "Version-agnostic MigrationOutcome enum: 'noop_already_current' | 'migrated' | 'noop_unsupported_version' replaces version-encoded names. Future migrations (v3→v4 etc.) inherit the same enum without rename churn. Locked by PLAN Task 3."
    - "Sidebar in-place upsert only (no row insertion): watcher owns _char_owner row creation; sidebar updates existing chars. Single-writer-per-row invariant preserved."
    - "HtmlService 360px sidebar with inline JS + google.script.run.withSuccessHandler/withFailureHandler: pattern reused across Phase 4+ sidebars (bank coin in 04-04, search/eviction in Phase 5)."
    - "CI assertion exports↔TRIGGER_GLOBALS in build.mjs: catches stub-not-replaced + missing-globals class of bugs. Locked by PATTERNS §3 Pattern D."
    - "eq-constants single-source-of-truth: CLASSES/RACES/WIKI_SLOT_TO_INV_SLOTS lives in one module; both Phase 4 builders + future phases import. No constant duplication."
key-files:
  created:
    - apps-script/src/lib/eq-constants.ts (~80 lines; CLASSES + CLASS_DISPLAY_TO_ABBREV + RACES + WIKI_SLOT_TO_INV_SLOTS + type guards)
    - apps-script/src/triggers/showCharInfoSidebar.ts (~280 lines; sidebar opener + getCharsForForm + saveCharInfo lock-guarded upsert + inline HTML)
    - apps-script/src/__tests__/charInfoSidebar.test.ts (~180 lines; 7 vitest scenarios)
    - .planning/phases/04-differentiator-features/04-01-SUMMARY.md (this file)
  modified:
    - internal/sheet/client.go (+1/-1 lines; WatcherMaxSchemaVersion 2 → 3)
    - internal/scaffold/scaffold.go (+1 line; _char_owner.Headers append "race"; total 14 cols)
    - internal/scaffold/scaffold_test.go (+2/-1 lines; row-count assertions bump 13 → 14)
    - apps-script/src/lib/migrations.ts (+50 lines; migrateToV3 + outcome-enum cleanup applied to migrateToV2)
    - apps-script/src/__tests__/migrations.test.ts (+80 lines; 5 new migrateToV3 scenarios mirroring migrateToV2)
    - apps-script/src/Code.ts (+5/-1 lines; export migrateToV3, showCharInfoSidebar, getCharsForForm, saveCharInfo)
    - apps-script/build.mjs (+30/-5 lines; add 4 new TRIGGER_GLOBALS entries + assertExportsMatchGlobals CI assertion)
    - apps-script/src/triggers/onOpen.ts (+1 line; addItem 'Set Character Info…' between Refresh Wiki Items Now and Set Theme…)
decisions:
  - "MigrationOutcome enum renamed to version-agnostic shape ('noop_already_current' / 'migrated' / 'noop_unsupported_version'): clean cleanup applied retroactively to migrateToV2. Future v3→v4 etc. migrations inherit without rename churn."
  - "Sidebar updates in-place ONLY — never inserts new _char_owner rows: watcher owns row creation. Sidebar can't conjure chars that haven't been uploaded yet. Documented limitation; deferred auto-prompt to Phase 5."
  - "Race auto-detection from inventory data REJECTED: P99 inventory file format doesn't expose race. User-entry is the only reliable signal. Locked by PLAN deferred-items + CONTEXT."
  - "build.mjs assertExportsMatchGlobals CI assertion: triggered by Phase 3 deploy where migrateToV2 was missing from TRIGGER_GLOBALS (hotfixed in d0a2645). This plan adds the assertion so the bug class can't recur."
  - "Schema bumped to v=3 — coordinated with v0.4.0 watcher ship. WatcherMaxSchemaVersion=3 must reach guildies BEFORE clasp push reaches workbook."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-10T17:55:29-05:00
  tasks_completed: 6 of 6
  commits: 6 (650f598 chore go-side WatcherMaxSchemaVersion+scaffold race; d55fa9f feat eq-constants; e81731a feat migrateToV3 + outcome-enum cleanup; 8936f2f feat showCharInfoSidebar + getCharsForForm + saveCharInfo + tests; f3712c0 chore wire char-info menu + Code.ts + build.mjs CI assertion; 627206c chore STATE.md → plan 04-01 complete)
  files_changed: 12 (4 created + 8 modified, ~830 lines added)
  tests_added: 12 (5 migrateToV3 + 7 charInfoSidebar)
  trigger_count_after: 4 (unchanged from 03-04; new functions are sidebar callbacks not time-driven triggers)
  schema_version_after: 3 (migrateToV3 ships; runs on first trigger fire after deploy)
  watcher_rebuild_required: true (v0.4.0 must ship to guildies BEFORE clasp push)
---

# Phase 4 Plan 01: Schema v=3 Migration + Char Info Sidebar Summary

**One-liner:** Laid the Phase 4 schema + char-info capture foundation — bumped WatcherMaxSchemaVersion to 3 (ship as v0.4.0), added `_char_owner.race` column for Iksar-tier filtering in 04-03, shipped the `eq-constants` module (CLASSES + RACES + WIKI_SLOT_TO_INV_SLOTS) used by all Phase 4 builders, landed idempotent `migrateToV3()` (1 column add to `_char_owner`) with a version-agnostic MigrationOutcome enum cleanup applied to migrateToV2 retroactively, built the 360px HtmlService char-info sidebar with class/level/race dropdowns (in-place upsert only — watcher owns row creation), and added the `build.mjs` CI assertion that catches Code.ts↔TRIGGER_GLOBALS divergence (the bug class that hotfixed during Phase 3 deploy via `d0a2645`).

## What shipped

### Task 1 — Go-side WatcherMaxSchemaVersion + scaffold race column (commit `650f598`)

`internal/sheet/client.go`: WatcherMaxSchemaVersion 2 → 3 with updated doc comment. Coordination contract: watcher v0.4.0 ships to all guildies BEFORE any clasp push reaches the workbook, or v0.3.x watchers will reject schema_version=3 workbooks with `ErrSchemaTooNew`.

`internal/scaffold/scaffold.go`: `DimensionTabs[_char_owner].Headers` appended with `"race"` (14 cols total). Doc comments updated. `scaffold_test.go` row-count assertions bumped 13 → 14. `internal/sheet/meta_test.go` Test 5 (`"too new"` schema_version) bumped from 3 to 4.

Verified: `go build ./... && go test ./...` green.

### Task 2 — eq-constants.ts (commit `d55fa9f`)

Single source of truth for class + race + slot vocabulary, imported by both Phase 4 builders + future phases:

- `CLASSES = ['WAR', 'CLR', 'PAL', 'RNG', 'SHD', 'DRU', 'MNK', 'BRD', 'ROG', 'SHM', 'NEC', 'WIZ', 'MAG', 'ENC']` as const → `ClassAbbrev` type.
- `CLASS_DISPLAY_TO_ABBREV` maps wiki display name (`'Bard'`, `'Cleric'`, `'Shadow Knight'`, …) → 3-letter abbrev. Inverted as needed for 04-02's class-page fetcher.
- `RACES = ['HUM', 'BAR', 'ERU', 'ELF', 'HIE', 'DEF', 'HEF', 'DWF', 'TRL', 'OGR', 'HFL', 'GNM', 'IKS', 'VAH']`.
- `WIKI_SLOT_TO_INV_SLOTS`: 17-entry map of wiki vocab → inv slot arrays. Many-to-many for pair slots (`'Ears' → ['EAR1', 'EAR2']`, `'Fingers' → ['FINGER1', 'FINGER2']`, `'Wrists' → ['WRIST1', 'WRIST2']`). gear_check considers a char OK if EITHER slot has the matching item.
- `isClassAbbrev(s)` and `isRaceAbbrev(s)` type guards — used by saveCharInfo for runtime validation.

### Task 3 — migrateToV3 + outcome-enum cleanup (commit `e81731a`)

Appended `migrateToV3()` to `apps-script/src/lib/migrations.ts`. Reads `_meta.schema_version`; if 3 returns `noop_already_current`; if not 2 returns `noop_unsupported_version`. Otherwise acquires 30s document lock, `appendColumns('_char_owner', ['race'])`, writes `_meta.schema_version='3'` LAST, returns `migrated`. Releases lock in finally.

**Outcome-enum cleanup** applied retroactively to migrateToV2: replaced `'noop_already_v2' | 'migrated_v1_to_v2' | 'noop_unsupported_version'` with version-agnostic `'noop_already_current' | 'migrated' | 'noop_unsupported_version'`. Future v3→v4 etc. migrations inherit without rename churn. Tests + callers updated.

5 vitest scenarios mirror migrateToV2: schema_version=3 noop, schema_version=2 happy path, schema_version=1 unsupported (cleaner error — was previously `migrated_v1_to_v2`), idempotent on partial prior run, schema_version write LAST invariant.

### Task 4 — showCharInfoSidebar + form callbacks (commit `8936f2f`)

`showCharInfoSidebar()`: opens a 360px-wide `HtmlService` sidebar titled "Set Character Info" with inline HTML — class/level/race dropdowns per char row, Save button calling `google.script.run.saveCharInfo(chars)`. Description: "Set class/level/race for each character. Watcher owns row creation; this form only updates existing chars."

`getCharsForForm(): CharInfoRow[]`: reads `_char_owner` rows 2..lastRow; emits `{char_name, class, level, race}` for each non-empty char_name. Column indices hardcoded: char_name=1, class=5, level=6, race=14.

`saveCharInfo(chars): { saved, errors }`: validation pass first (collect all errors without writing); `isClassAbbrev` / `1<=level<=60` / `isRaceAbbrev` checks. If any errors, return `{saved: 0, errors}` (no partial writes). Otherwise acquire 30s document lock, read existing _char_owner rows once, for each input char find matching row by char_name (string equality, trimmed), update class/level/race in place via per-cell `setValue` only if the value is non-empty. Never inserts new rows. Release lock in finally.

7 vitest scenarios: `getCharsForForm` empty when _char_owner missing; non-empty rows skip empty char_name; saveCharInfo with invalid class returns `{saved: 0, errors}` and writes nothing; valid data updates _char_owner in place; chars not in _char_owner are skipped (no insert); lock contention throws; race validation case-sensitive (IKS yes, "iksar" no).

### Task 5 — Wire onOpen + Code.ts + build.mjs CI assertion (commit `f3712c0`)

`onOpen.ts`: SquireBot menu chain extended with `addItem('Set Character Info…', 'showCharInfoSidebar')` between `'Refresh Wiki Items Now'` and the separator before `'Set Theme…'`.

`Code.ts`: 4 new re-exports — `migrateToV3`, `showCharInfoSidebar`, `getCharsForForm`, `saveCharInfo`. The MigrationOutcome rename triggers a cascade fix on existing migration callers.

`build.mjs`: 4 new entries added to TRIGGER_GLOBALS array. **NEW**: `assertExportsMatchGlobals()` CI assertion per PATTERNS §3 Pattern D — extracts named exports from `apps-script/src/Code.ts` via regex, compares against TRIGGER_GLOBALS array; build exits non-zero if there's any divergence (extra in Code.ts OR missing from globals). This catches the bug class that hotfixed during Phase 3 deploy (`d0a2645` exposing migrateToV2 as top-level global) and prevents Phase 4+ recurrence.

### Task 6 — STATE.md (commit `627206c`)

Status `phase-4-plan-01-complete`. completed_plans 22 → 23.

## Deviations from Plan

None — plan executed as written. (Detailed deviation tracking not captured retroactively.)

## Schema impact

Schema bumped to v=3. WatcherMaxSchemaVersion = 3 in Go. 1 col added to `_char_owner` (race, col 14). No new tabs, no new _meta rows. Extend-only path A maintained.

## Verification log

```
$ npm test -- migrations
Tests       8 passed (8)   # 4 from migrateToV2 (cumulative) + 5 from migrateToV3, minus 1 consolidated

$ npm test -- charInfoSidebar
Tests       7 passed (7)

$ npm run build
(exit 0 — assertExportsMatchGlobals passes; 14 trigger globals
 in dist/Code.js: 10 from Phase 3 + migrateToV3 + showCharInfoSidebar +
 getCharsForForm + saveCharInfo)

$ go build ./... && go test ./...
(exit 0 — WatcherMaxSchemaVersion=3; _char_owner.Headers length=14)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/lib/eq-constants.ts` (CLASSES + RACES + WIKI_SLOT_TO_INV_SLOTS)
- FOUND: `apps-script/src/triggers/showCharInfoSidebar.ts`
- FOUND: `apps-script/src/__tests__/charInfoSidebar.test.ts`
- FOUND: `apps-script/src/lib/migrations.ts` (now contains migrateToV3)
- FOUND: `internal/sheet/client.go` (WatcherMaxSchemaVersion=3 verified)

**Commits exist:**
- FOUND: `650f598` — chore(go): bump WatcherMaxSchemaVersion to 3 + scaffold _char_owner.race
- FOUND: `d55fa9f` — feat(apps-script): eq-constants — CLASSES + RACES + WIKI_SLOT_TO_INV_SLOTS
- FOUND: `e81731a` — feat(apps-script): migrateToV3 + outcome-enum cleanup
- FOUND: `8936f2f` — feat(apps-script): showCharInfoSidebar + getCharsForForm + saveCharInfo + tests
- FOUND: `f3712c0` — chore(apps-script): wire char-info menu + Code.ts exports + build.mjs CI assertion
- FOUND: `627206c` — chore: STATE.md → plan 04-01 complete

## Next plan

`/gsd-execute-phase 4` spawned plan `04-02` for the per-class spell scrape + spell_check builder — `refreshWikiSpells` fetches one wiki page per class (14 total) via politeFetch with 1s sleep, parses `==Level N==` sections + `{{SpellRow|name=...}}` templates, full-replaces `_wiki_spells` per class block; `buildSpellCheck` joins `_char_owner.class+level ↔ spell:<char> ↔ _wiki_spells` to emit (Char, Class, Level, Spell, Status) rows with KNOWN/MISSING status.

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 04-01-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
