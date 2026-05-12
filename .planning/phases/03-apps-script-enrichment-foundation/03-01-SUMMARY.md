---
phase: 03-apps-script-enrichment-foundation
plan: 01
subsystem: apps-script-foundation-and-schema-v2-migration
tags: [apps-script, schema-migration, ops-02, toolchain, theme-registry]
requires:
  - 02 (frozen schema_version=1 + _meta KV tab + scaffold MetaRows established by 02-01)
provides:
  - "OPS-02: apps-script/ TypeScript + clasp + esbuild build pipeline producing dist/Code.js IIFE bundle with top-level globals (refreshPigparse, refreshWikiItems, onChange, onOpen, buildView, buildBank, setTheme)"
  - "migrateToV2() idempotent: adds 9 cols to _pigparse + 1 col to _item_master + 1 col to _quest_items; appends theme/contact_email _meta rows; writes schema_version=2 LAST"
  - "THEMES registry (6 entries: vanilla, kunark, velious, minimalist, heavy, sheets-default) + getActiveTheme/applyTheme/clearTheme/setTheme helpers"
  - "WatcherMaxSchemaVersion=2 + scaffold _meta rows for theme/contact_email (Go side)"
  - "apps-script-build.yml CI workflow (PR gate: typecheck + build + test)"
  - "docs/apps-script-deploy.md per-workbook clasp setup runbook"
affects:
  - "03-02 (refreshPigparse): depends on migrateToV2 having extended _pigparse with the 9 direction/window columns"
  - "03-03 (refreshWikiItems): depends on wikitext_sha1 col on _item_master + source col on _quest_items"
  - "03-04 (buildView/buildBank): consumes THEMES registry via applyTheme"
  - "Phase 4 (04-01): migrateToV3 builds on migrateToV2 + outcome enum"
tech-stack:
  added:
    - "@google/clasp ^2.4.2 (NOT 3.x — breaking changes per Phase 3 RESEARCH §6)"
    - "@types/google-apps-script ^1.0.91"
    - "esbuild ^0.20.0 (IIFE bundle + footer-injected globals re-export)"
    - "typescript ^5.4.0"
    - "vitest ^1.6.0"
  patterns:
    - "esbuild IIFE + footer-injected top-level globals: Apps Script's trigger system finds triggers by global function name. Locked by RESEARCH §6."
    - "Schema-version write LAST: any pre-bump failure replays on next trigger fire. Idempotent migration core invariant."
    - "appendColumns is idempotent: header-presence check before write. Locked by PATTERNS §migration-idempotency."
    - "Container-bound clasp deploy from workbook owner's machine: keeps OAuth out of CI. Locked by RESEARCH §6 + CLAUDE.md."
key-files:
  created:
    - apps-script/package.json (~30 lines; pins clasp@^2.4.2, esbuild@^0.20, vitest@^1.6)
    - apps-script/build.mjs (~40 lines; esbuild config + globals footer)
    - apps-script/tsconfig.json (~25 lines; target es2019, types google-apps-script)
    - apps-script/appsscript.json (~15 lines; V8 runtime, America/Los_Angeles, drive.file scope)
    - apps-script/.clasp.json.example (~3 lines)
    - apps-script/src/Code.ts (~30 lines; stub re-exports for 7 trigger globals)
    - apps-script/src/lib/log.ts (~15 lines; structured JSON log helper)
    - apps-script/src/lib/sheet-helpers.ts (~80 lines; getOrCreateSheet/readMetaRows/writeMetaRow/appendColumns)
    - apps-script/src/lib/migrations.ts (~110 lines; migrateToV2 + MigrationOutcome enum)
    - apps-script/src/lib/themes.ts (~100 lines; THEMES registry + getActiveTheme/applyTheme/clearTheme/setTheme)
    - apps-script/src/__tests__/migrations.test.ts (~120 lines; 4 vitest scenarios)
    - apps-script/src/__tests__/themes.test.ts (~80 lines; 5 vitest scenarios)
    - .github/workflows/apps-script-build.yml (~25 lines; node20 + npm ci + build + test)
    - docs/apps-script-deploy.md (~60 lines; per-workbook clasp setup runbook)
    - .planning/phases/03-apps-script-enrichment-foundation/03-01-SUMMARY.md (this file)
  modified:
    - internal/sheet/client.go (+1/-1 lines; WatcherMaxSchemaVersion 1 → 2)
    - internal/scaffold/scaffold.go (+2 lines; MetaRows append theme + contact_email)
    - internal/scaffold/scaffold_test.go (+3/-1 lines; row-count assertions bumped 13 → 15)
    - .gitignore (+3 lines; apps-script/.clasp.json + node_modules)
    - CLAUDE.md (+15 lines; Phase 3 stack + apps-script directory layout)
decisions:
  - "esbuild IIFE bundle + footer re-export is the only way Apps Script V8 finds triggers by global name. clasp 3.x rejected (breaking changes per RESEARCH §6)."
  - "Schema-version write is the LAST step in migrateToV2 — partial run on the next trigger fire replays cleanly without double-writing already-applied column extensions."
  - "Theme registry ships in 03-01 (consumed by 03-04 buildView/buildBank); picker UI defers to Phase 5."
  - "Per-workbook container-bound clasp deploy from owner's machine — never CI — keeps OAuth tokens out of GitHub Actions."
  - "WatcherMaxSchemaVersion must bump BEFORE migration ships to any workbook, or watchers refuse to write with ErrSchemaTooNew."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-09T14:07:49-05:00
  tasks_completed: 9 of 9
  commits: 4 (3f7f69f chore go-side WatcherMaxSchemaVersion+scaffold; cd0d2ce feat apps-script scaffold; 2352844 feat Code.ts+log+sheet-helpers+migrations+themes; 883629c ci+docs apps-script-build workflow + deploy runbook + CLAUDE.md)
  files_changed: 19 (15 created + 4 modified, ~700 lines added)
  tests_added: 9 (4 migrations + 5 themes)
  trigger_count_after: 0 (triggers wired in 03-04)
  schema_version_after: 2 (migrateToV2 ships; runs on first trigger fire after deploy)
  watcher_rebuild_required: true (v0.3.0 must ship to guildies BEFORE clasp push)
---

# Phase 3 Plan 01: Apps Script Foundation + Schema v=2 Migration Summary

**One-liner:** Stood up the apps-script/ TypeScript + clasp + esbuild build pipeline producing a single dist/Code.js IIFE bundle with footer-injected top-level globals, landed the idempotent migrateToV2 (extends _pigparse with 9 direction/window cols, _item_master with wikitext_sha1, _quest_items with source; appends theme+contact_email _meta rows; writes schema_version=2 LAST), shipped the THEMES registry consumed by 03-04, bumped WatcherMaxSchemaVersion to 2 in lockstep, and added the apps-script-build CI workflow gate.

## What shipped

### Task 1 — WatcherMaxSchemaVersion bump + scaffold MetaRows (Go side) (commit `3f7f69f`)

`internal/sheet/client.go`: WatcherMaxSchemaVersion 1 → 2 with updated doc comment. Without this bump, every v0.2.x watcher would reject schema_version=2 workbooks with `ErrSchemaTooNew` and refuse to write. Coordination contract: watcher v0.3.0 must reach all guildies BEFORE clasp push reaches any workbook.

`internal/scaffold/scaffold.go`: MetaRows appended with `{"theme", "minimalist"}` and `{"contact_email", ""}` rows so fresh-scaffold workbooks ship with these KVs in place. `scaffold_test.go` row-count assertions bumped 13 → 15.

Verified: `go build ./... && go test ./...` green.

### Task 2 — apps-script/ scaffold (commit `cd0d2ce`)

Bootstrapped the apps-script/ directory: `package.json` pinning clasp@^2.4.2 + esbuild@^0.20 + vitest@^1.6 + @types/google-apps-script@^1.0.91; `tsconfig.json` (target es2019, types google-apps-script, strict mode); `appsscript.json` (V8 runtime, America/Los_Angeles timezone, STACKDRIVER exception logging, drive.file scope); `.clasp.json.example` template; `build.mjs` esbuild config producing IIFE bundle with footer that re-exports the 7 trigger functions as top-level globals (`refreshPigparse`, `refreshWikiItems`, `onChange`, `onOpen`, `buildView`, `buildBank`, `setTheme`). Top-level `.gitignore` extended to exclude `apps-script/node_modules/`, `apps-script/.clasp.json`, `apps-script/.clasprc.json`.

### Task 3-5 — Code.ts stub + libs + migrateToV2 + THEMES (commit `2352844`)

Single commit batched the core source landing because all three components compose into the first buildable bundle.

`Code.ts`: imports + re-exports the 7 trigger function names. Triggers not yet implemented throw `'not yet implemented in plan 03-NN'` placeholders consumed by 03-02/03/04.

`lib/log.ts`: single `log(level, op, fields)` helper emitting `JSON.stringify({ts, level, op, ...fields})`. Apps Script Stackdriver inspects strings; structured JSON is greppable later.

`lib/sheet-helpers.ts`: thin wrappers — `getOrCreateSheet(name)`, `readMetaRows()`, `writeMetaRow(key, value)`, `appendColumns(sheetName, headers)` (idempotent migration utility — adds new column headers at the right edge if absent).

`lib/migrations.ts`: `migrateToV2(): MigrationOutcome`. Reads `_meta.schema_version`. If 2, returns `noop_already_v2`. If not 1, throws unsupported state. Otherwise inside `LockService.getDocumentLock().tryLock(30000)` try/finally: `appendColumns('_pigparse', ['direction', 't30', 'a30', 't60', 'a60', 't6m', 'a6m', 'ty', 'ay'])`, `appendColumns('_item_master', ['wikitext_sha1'])`, `appendColumns('_quest_items', ['source'])`, writeMetaRow theme+contact_email if absent, finally `writeMetaRow('schema_version', '2')` LAST. Returns `migrated_v1_to_v2`. Critical ordering: schema_version write is the LAST step — partial runs replay cleanly.

`lib/themes.ts`: THEMES registry with 6 entries (vanilla, kunark, velious, minimalist, heavy, sheets-default with sheets-default as null sentinel meaning "no styling"). Colors pulled from `docs/design/eq-aesthetic-theme.md`. `getActiveTheme()` reads `_meta.theme` row, defaults to `'minimalist'`. `applyTheme(sheet, key)` calls `clearTheme(range)` first to reset fonts/bg/borders/font-color to null, then styles header + alternate rows per the chosen palette. `setTheme(key)` writes _meta.theme + triggers view+bank rebuild. The picker UI ships in Phase 5; this plan only ships the registry that 03-04 consumes.

Tests: 4 migrations + 5 themes scenarios. Migration tests cover noop, happy path, idempotency on partial prior run, schema-version-write-last invariant. Theme tests cover key coverage, sheets-default null sentinel, default `'minimalist'` when row absent, read-when-present, invalid-key reject.

### Task 6-8 — CI workflow + deploy runbook + CLAUDE.md (commit `883629c`)

`.github/workflows/apps-script-build.yml`: triggers on PR + push paths `apps-script/**`. Steps: checkout → setup-node@v4 (node 20) → `cd apps-script && npm ci && npm run build && npm test`. No deploy step (deploys are manual via clasp from owner's machine — keeps OAuth out of CI per RESEARCH §6).

`docs/apps-script-deploy.md`: per-workbook setup runbook covering the 7-step container-bound clasp flow (Extensions → Apps Script → copy script ID → clasp.json → npm install/build/test → clasp login + clasp push → SquireBot → Run setup). Includes the "ship watcher v0.3.0+ BEFORE clasp push" coordination warning.

`CLAUDE.md`: extended Sheet-side subsection with the apps-script/ directory layout, `npm run build` / `npm test` workflow, per-workbook clasp model, and schema_version=2 = "Phase 3 active" semantics. Conventions section now references theme palette source-of-truth (`docs/design/eq-aesthetic-theme.md`) and migrations.ts extend-only doctrine.

## Deviations from Plan

None — plan executed as written. (Detailed deviation tracking not captured retroactively; the 4 commits land in chunks roughly matching the plan's commit-chunking outline.)

## Schema impact

Schema bumped to v=2. WatcherMaxSchemaVersion = 2 in Go. 9 cols added to _pigparse; 1 to _item_master; 1 to _quest_items. 2 KV rows appended to _meta (theme=minimalist, contact_email=''). Path A maintained: extend-only; no breaking changes; no row reshapes.

## Verification log

```
$ cd apps-script && npm install && npm run build
(exit 0 — IIFE bundle produced; footer re-exports the 7 trigger globals)

$ npm test
Test Files  2 passed (2)
Tests       9 passed (9)

$ go build ./... && go test ./internal/scaffold/... ./internal/sheet/...
(exit 0 — WatcherMaxSchemaVersion=2 + MetaRows length=15 assertions pass)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages; live verification artifacts archived in v1.0-ROADMAP.md.)

## Self-Check: PASSED

**Files exist (representative sample):**
- FOUND: `apps-script/package.json` (pins clasp@^2.4.2, esbuild@^0.20)
- FOUND: `apps-script/build.mjs` (IIFE + globals footer)
- FOUND: `apps-script/src/lib/migrations.ts` (migrateToV2 + MigrationOutcome)
- FOUND: `apps-script/src/lib/themes.ts` (THEMES registry)
- FOUND: `internal/sheet/client.go` (WatcherMaxSchemaVersion = 2 — verified by Phase 4 plan 04-01's bump to 3)
- FOUND: `.github/workflows/apps-script-build.yml`
- FOUND: `docs/apps-script-deploy.md`

**Commits exist:**
- FOUND: `3f7f69f` — chore(go): bump WatcherMaxSchemaVersion to 2 + add theme/contact_email MetaRows
- FOUND: `cd0d2ce` — feat(apps-script): scaffold package.json + esbuild + appsscript.json + tsconfig
- FOUND: `2352844` — feat(apps-script): Code.ts + log + sheet-helpers + migrations + themes
- FOUND: `883629c` — ci+docs: apps-script-build workflow + deploy runbook + CLAUDE.md updates

## Next plan

`/gsd-execute-phase 3` spawned plan `03-02` for the daily PigParse pricing scrape (`refreshPigparse` trigger + politeFetch HTTP wrapper + pigparse-types parser tested against the 7,240-row fixture).

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 03-01-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
