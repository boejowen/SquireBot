---
phase: 08-test-infra-persistence-docs
plan: 01
subsystem: apps-script-test-infra
tags: [apps-script, vitest, jsdom, test-infra, mount-sidebar, properties-service-mock, test-01]
requires:
  - 07 (test-helpers.ts MockState + installAppsScriptMocks baseline at 327 tests)
  - PROJECT.md (apps-script TypeScript layout convention; tests in src/__tests__/)
provides:
  - "TEST-01: vitest.config.ts declaring environment: 'jsdom' at top level so every existing test file (and Plan 08-02's new sidebar tests) gets a DOM by default"
  - "jsdom@^24.1.3 devDependency wired (24.x is the version contemporary with vitest 1.6)"
  - "mountSidebar(html) JSDOM helper exported from test-helpers.ts — parses SIDEBAR_BODY HTML, installs google.script.run Proxy mock BEFORE re-executing inline <script> blocks, returns realm + dispatchRunCall/failRunCall/runCalls for tests"
  - "MountedSidebar TypeScript interface exported for consumer type-safety"
  - "PropertiesService mock extended with getUserProperties() + getScriptProperties() scope aliases — both backed by a SEPARATE state.userProperties Map so user-scope writes don't bleed into document-scope tests (D-04 / D-05)"
  - "MockState.userProperties: Map<string, string> field + newMockState() initializer"
affects:
  - "Plan 08-02 (TEST-02): 4 net-new sidebar test files (searchSidebar/evictionSidebar/bankCoinSidebar/charInfoSidebar) consume mountSidebar verbatim with zero further setup"
  - "Plan 08-03 (SEARCH-05): can swap CacheService.getDocumentCache() → PropertiesService.getUserProperties() in searchIndex.ts and rely on the per-user-scope-isolated mock for tests"
tech-stack:
  added:
    - "jsdom@^24.1.3 (devDependency only — ~50MB transitive incl. cssom, tough-cookie, webidl-conversions, whatwg-url)"
  patterns:
    - "vitest 1.6 global JSDOM env via defineConfig({ test: { environment: 'jsdom' } }) — per-test // @vitest-environment node overrides remain available but no current test needs one (D-01)"
    - "JSDOM script-execution workaround: <script> tags inserted via innerHTML do NOT execute per HTML5 spec; helper extracts script textContent, creates fresh <script> elements via document.createElement, appends to head (Ghinda canonical pattern + JSDOM issue #426)"
    - "google.script.run fluent mock as a Proxy: any-order chaining of .withSuccessHandler/.withFailureHandler + arbitrary terminal METHOD invocation captured into FIFO pending queues; tolerates fire-and-forget calls (Search's pushRecentSearchCall)"
    - "PropertiesService scope isolation in tests: state.properties (document) + state.userProperties (user/script-aliased) — separate Maps prevent cross-scope bleed for SEARCH-05 tests in Plan 08-03"
    - "Type-only tsconfig.json compilerOptions.lib extension ([\"es2019\", \"dom\"]) — DOM globals visible to test code without runtime impact (dist/Code.js verified clean of mountSidebar/userProperties refs)"
key-files:
  created:
    - apps-script/vitest.config.ts (15 lines — defineConfig with environment: 'jsdom', include glob, exclude paths, globals: false)
    - .planning/phases/08-test-infra-persistence-docs/08-01-SUMMARY.md (this file)
  modified:
    - apps-script/package.json (+1 line — jsdom: ^24.1.3 in devDependencies)
    - apps-script/package-lock.json (+676 lines — jsdom transitive deps locked)
    - apps-script/tsconfig.json (+1 line edit — lib gained "dom")
    - apps-script/src/__tests__/test-helpers.ts (+142 lines / -1 line — MockState.userProperties field, newMockState init, PropertiesService getUserProperties + getScriptProperties block, MountedSidebar interface + mountSidebar function at EOF)
decisions:
  - "Rule 3 fix applied: tsconfig.json compilerOptions.lib gained 'dom' (was [\"es2019\"]). The mountSidebar helper's signature references Document, Window, HTMLScriptElement — all DOM globals invisible under the prior lib config. Adding 'dom' is a type-only declaration (no runtime impact) and the dist/Code.js build was verified to contain zero references to mountSidebar or userProperties after the change (T-08-01-02 grep gate)."
  - "PropertiesService mock uses SEPARATE Maps for user-scope vs document-scope per D-04 + D-05 + REQUIREMENTS.md SEARCH-05's 'per-guildie state, not workbook-shared state' rule. getScriptProperties() aliases the per-user Map for completeness — no current test exercises script-scope, the production code never reads script-scope, so the alias is safe."
  - "jsdom pinned to ^24.1.3 (24.x is contemporary with vitest 1.6, April 2024). Newer jsdom (25+, 29+) would likely work via vitest's optionalDependencies relationship but is untested here."
  - "vitest.config.ts include glob is explicit ('src/__tests__/**/*.test.ts') — hardens the discovery contract over vitest's default crawl (which would also walk node_modules/ for any nested *.test.ts)."
  - "mountSidebar installs the google.script.run Proxy mock BEFORE re-executing inline scripts because some sidebars (Search, at minimum) reach window.google synchronously at the top of init(). Step ordering is load-bearing — documented in the helper's comment block."
  - "Plan executed exactly as written by the planner — no scope changes beyond the Rule 3 tsconfig lib extension."
metrics:
  duration: ~13min
  completed: 2026-05-12T15:54Z
  tasks_completed: 3 of 3
  commits: 3 (53433d1 jsdom install + vitest.config.ts, 48c47a8 PropertiesService getUserProperties, 98b0fb1 mountSidebar helper)
  files_changed: 5 (2 created + 3 modified, ~833 lines added — most of which are package-lock.json transitive-dep locks)
  tests_added: 0 (infra-only plan; Plan 08-02 ships the consumers)
  tests_passing_before: 327
  tests_passing_after: 327
  trigger_count_after: 8 (unchanged)
  schema_version_after: 3 (unchanged)
  watcher_rebuild_required: false
---

# Phase 8 Plan 01: Test Infra (JSDOM + mountSidebar + PropertiesService user-scope) Summary

**One-liner:** Wired JSDOM as vitest's default environment, installed `jsdom@^24.1.3`, and extended `test-helpers.ts` with two test-infrastructure surfaces (`mountSidebar(html)` JSDOM helper + `PropertiesService.getUserProperties()` scope alias) that Wave 2 (Plan 08-02) and Plan 08-03 consume — all 327 existing tests stay green under the new JSDOM env.

## What shipped

### Task 1 — Install jsdom + create vitest.config.ts (commit `53433d1`)

Ran `npm install -D jsdom@^24.0.0` from `apps-script/`; npm resolved to `jsdom@24.1.3` and locked transitive deps (cssom, tough-cookie, webidl-conversions, whatwg-url, et al.) into `package-lock.json`. Created `apps-script/vitest.config.ts` (15 lines, NEW FILE) declaring `environment: 'jsdom'` at the top level of `defineConfig({ test: { ... } })` plus an explicit `include: ['src/__tests__/**/*.test.ts']` glob and `exclude: ['node_modules', 'dist', 'src/__fixtures__']`. `globals: false` was kept explicit so the existing tests' explicit `import { describe, it, expect, beforeEach } from 'vitest'` style remains the contract.

Full existing suite (327 tests across 30 files) stayed green under the new env — confirming the planner's call that JSDOM is a no-op for tests that mock Apps Script globals and never touch `document` / `window`.

### Task 2 — Extend PropertiesService mock with getUserProperties scope alias (commit `48c47a8`)

Three additive edits to `apps-script/src/__tests__/test-helpers.ts`:

1. New `userProperties: Map<string, string>` field added to the `MockState` interface immediately after the existing `properties` field, with a Phase 8 / SEARCH-05 comment marker.
2. `newMockState()` initializer gained `userProperties: new Map()` immediately after the existing `properties: new Map()`.
3. The PropertiesService block at the existing `installAppsScriptMocks()` site was extended (not replaced) with `getUserProperties()` + `getScriptProperties()` methods, both backed by `state.userProperties` (a SEPARATE Map). `getDocumentProperties()` was byte-identical preserved — zero behavior change for any test that currently uses it.

Per D-05 + REQUIREMENTS.md SEARCH-05, per-user scope is the correct semantics for the recent-MRU search migration. The script-scope alias is included for completeness; no current consumer distinguishes the two, but the alias guards against future "where did it go" reaches for the wrong scope.

327/327 tests stayed green.

### Task 3 — Add mountSidebar(html) JSDOM helper to test-helpers.ts (commit `98b0fb1`)

Two changes:

1. `apps-script/tsconfig.json` `compilerOptions.lib` extended from `["es2019"]` to `["es2019", "dom"]`. This is a Rule 3 (blocking) deviation — the mountSidebar helper's signature uses `Document`, `Window`, `HTMLScriptElement` globals that are NOT in scope under `lib: ["es2019"]`. Type-only declaration; verified by post-build grep that `dist/Code.js` contains zero references to `mountSidebar` or `userProperties` (test-helpers.ts is never imported by `src/Code.ts`, which is esbuild's entry point — the structural separation is preserved).

2. ~120 lines appended at the end of `apps-script/src/__tests__/test-helpers.ts`:

   - `export interface MountedSidebar` — `{ document, window, runCalls, dispatchRunCall, failRunCall, getPendingCalls }`.
   - `export function mountSidebar(html: string): MountedSidebar` — five-step procedure: (1) reset body, (2) parse HTML into a detached `<template>` so the parser splits script nodes from non-script nodes without executing them, (3) walk the parsed fragment cloning non-script nodes into the body, (4) install the `window.google.script.run` Proxy mock BEFORE re-executing scripts (step ordering is load-bearing — Search at minimum reads `window.google` at the top of `init()`), (5) recreate each script element with `document.createElement('script')` + set `.textContent` + append to head (canonical Ghinda workaround for HTML5's no-exec-via-innerHTML rule).
   - The Proxy intercepts `withSuccessHandler` / `withFailureHandler` chain calls (any order) and any terminal method invocation — captures the call into `runCalls` and enqueues handlers per-method into `pendingByMethod`. `dispatchRunCall(method, payload)` resolves the FIRST pending success handler FIFO; `failRunCall(method, error)` resolves the FIRST pending failure handler FIFO. Empty-queue throws are explicit and labeled.
   - Fire-and-forget terminal calls (no `withSuccessHandler` chain — e.g., Search's `pushRecentSearchCall`) are tolerated; the call is recorded in `runCalls` but no handlers are invoked on dispatch attempt (the queue entry has neither `success` nor `failure`).

Verification: `npx tsc --noEmit` clean; `npm test` 327/327 green; `npm run build` clean; `grep -c "mountSidebar\|userProperties" dist/Code.js` returns 0 (T-08-01-02 confirmed).

## Threat-register coverage

| Threat ID | Disposition | Mitigation evidence |
|-----------|-------------|---------------------|
| T-08-01-01 (Tampering: jsdom transitive deps) | accept | package-lock.json captures resolved hashes; same risk profile as existing vitest/esbuild/typescript deps |
| T-08-01-02 (InfoDisc: test-helpers.ts leaks into prod bundle) | mitigate | Verified post-build: `grep -c "mountSidebar\|userProperties" dist/Code.js` = 0. esbuild entry is `src/Code.ts`, structurally separate from `__tests__/` |
| T-08-01-03 (Repudiation: mock supports superset of live surface) | accept | Documented in plan threat model; existing tradeoff for all Apps Script tests |
| T-08-01-04 (DoS: jsdom install bloats CI) | accept | ~50MB transitive; CI install adds ~5–10s; well within budget |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] tsconfig.json `compilerOptions.lib` extended to include `"dom"`**

- **Found during:** Task 3 (mountSidebar helper write)
- **Issue:** The plan's mountSidebar block (and its exported `MountedSidebar` interface) references `Document`, `Window`, `HTMLScriptElement`, `HTMLElement`-shaped types. Under the existing `tsconfig.json` `"lib": ["es2019"]`, these globals do not exist — `npx tsc --noEmit` would have failed with `TS2304: Cannot find name 'Document'` etc.
- **Fix:** One-line edit to `apps-script/tsconfig.json`: `"lib": ["es2019"]` → `"lib": ["es2019", "dom"]`. Type-only declaration; DOM globals are visible to TypeScript but no runtime polyfill is added (production code never references them; tests get DOM globals from JSDOM at runtime).
- **Files modified:** `apps-script/tsconfig.json` (+1 char edit, captured in commit 98b0fb1)
- **Commit:** `98b0fb1`
- **Safety verification:** Post-build grep confirmed `dist/Code.js` contains 0 references to `mountSidebar` or `userProperties` — the type-level DOM availability did not cause production code to pick up DOM types. The structural separation (esbuild entry = `src/Code.ts`, never imports `__tests__/`) was preserved.

No other deviations. Plan executed as written for Tasks 1 and 2.

## Schema impact

**Zero schema impact.** Phase 8 Plan 01 touched only test infrastructure files (`vitest.config.ts`, `package.json`, `package-lock.json`, `tsconfig.json`, `test-helpers.ts`). Schema gate verification passes:

- `apps-script/src/lib/migrations.ts` `writeMetaRow('_meta', 'schema_version', '3')` count: 1 (unchanged)
- `internal/sheet/client.go` `WatcherMaxSchemaVersion = 3` (unchanged)

No watcher rebuild required; no `_meta.schema_version` bump; no `SCRIPT_MIN_SCHEMA_VERSION` change.

## Verification log

```
# Plan-level <verification> block (all PASS)
cd apps-script
test -f vitest.config.ts                                              # OK
grep -q "environment: 'jsdom'" vitest.config.ts                       # OK
grep -q '"jsdom"' package.json                                        # OK ("jsdom": "^24.1.3")
test -d node_modules/jsdom                                            # OK (24.1.3)
grep -q "^export function mountSidebar" src/__tests__/test-helpers.ts # OK (1 match)
grep -q "getUserProperties:" src/__tests__/test-helpers.ts            # OK (1 match)
grep -q "userProperties: Map<string, string>" src/__tests__/test-helpers.ts  # OK (1 match)
npx tsc --noEmit                                                      # OK (clean exit)
npm test                                                              # OK — 327/327 across 30 files

# Schema gate (untouched)
grep -c "writeMetaRow.*schema_version.*'3'" apps-script/src/lib/migrations.ts  # 1
grep "WatcherMaxSchemaVersion" internal/sheet/client.go               # = 3, unchanged

# Production-bundle safety (T-08-01-02)
npm run build && grep -c "mountSidebar\|userProperties" dist/Code.js  # 0
```

## Self-Check: PASSED

**Files exist (all 5 changed):**
- FOUND: `apps-script/vitest.config.ts` (NEW — 15 lines, `environment: 'jsdom'` declared)
- FOUND: `apps-script/package.json` (MOD — jsdom devDep entry present)
- FOUND: `apps-script/package-lock.json` (MOD — jsdom 24.1.3 + transitive deps locked)
- FOUND: `apps-script/tsconfig.json` (MOD — lib gained "dom")
- FOUND: `apps-script/src/__tests__/test-helpers.ts` (MOD — userProperties + getUserProperties + mountSidebar all present)

**Commits exist:**
- FOUND: `53433d1` — test(08-01): add jsdom + vitest.config.ts for TEST-01
- FOUND: `48c47a8` — test(08-01): extend PropertiesService mock with getUserProperties scope alias
- FOUND: `98b0fb1` — test(08-01): add mountSidebar JSDOM helper for TEST-02

## Next plan

Wave 2 (`08-02-tests-sidebar-jsdom-PLAN.md`) consumes `mountSidebar` from this plan's `test-helpers.ts` surface to ship the 4 net-new sidebar test files (Search, Eviction, Bank-Coin, Char-Info). Plan 08-03 (Wave 1, parallel) consumes the `getUserProperties()` scope alias when swapping `searchIndex.ts`'s recent-MRU storage from CacheService to PropertiesService. Plan 08-04 (Wave 1, parallel) is documentation-only and has no dependency on this plan.
