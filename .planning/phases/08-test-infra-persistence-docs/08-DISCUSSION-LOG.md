# Phase 8 Discussion Log — `/gsd-discuss-phase 8 --auto`

**Date:** 2026-05-12
**Mode:** `--auto` (single-pass, Claude picked recommended defaults — no AskUserQuestion calls fired)

This file is for human reference only (audits, retrospectives). Downstream agents read CONTEXT.md.

---

## Domain Analysis

Phase 8 closes v1.0 carry-over debt across three loosely-related streams:

- **TEST-01/02** — Wire JSDOM into vitest + cover 5 sidebars with inline-JS tests
- **SEARCH-05** — Migrate recent-3 MRU from CacheService (ephemeral) to PropertiesService (per-user, persistent)
- **DOC-04** — Backfill 8 retroactive SUMMARY.md files for Phase 3 and Phase 4 plans

No SPEC.md exists. Requirements come from ROADMAP.md and REQUIREMENTS.md.

## Prior Context Loaded

- `.planning/PROJECT.md` — core value, locked stack, schema-evolution rules
- `.planning/REQUIREMENTS.md` — 8 requirements for v1.0.1 (4 in Phase 8 scope)
- `.planning/STATE.md` — Phase 7 just shipped 2026-05-12; v1.0.1 at 2/3 phases
- `.planning/phases/07-admin-allowlist-eviction-enforcement/07-CONTEXT.md` — Phase 7 decisions (D-01..D-06: onOpen-never-throws, lock envelopes, dual-policy caller identity)
- `.planning/phases/06-installer-overwrite-running-shim/` — Phase 6 decisions (D-01..D-07: NSIS shim, named-event IPC, latest.json schema lock)

**Decisions carried forward (not re-asked):**

- Apps-script ships via `clasp push` to dev workbook (no CI deploy) — locked since v1.0.
- Test conventions: vitest, tests in `apps-script/src/__tests__/`, real-name fixtures when sourced from a character (clone of CLAUDE.md convention).
- Schema-evolution rule: extend-only via `_meta` rows; bumps require migration + `WatcherMaxSchemaVersion` change. Phase 8 should make zero schema changes (per ROADMAP success criterion 5).
- Structured logging via `log(level, op, fields)` on both Go and Apps Script sides.
- Owner-floor protection (Phase 7 D-04) doesn't surface here — Phase 8 doesn't touch eviction or admin policy.

**No interrupted checkpoint, no folded todos** (no open todos matched Phase 8's req IDs above the 0.4 relevance threshold).

## Codebase Scout

Sidebars found at `apps-script/src/triggers/`:

| File | Status |
|------|--------|
| `showSearchSidebar.ts` | shipping; test coverage at trigger level (showSearchSidebar.test.ts); NO inline-JS DOM tests |
| `showEvictionSidebar.ts` | shipping; trigger-level tests; NO inline-JS DOM tests (TEST-02 scope) |
| `showBankCoinSidebar.ts` | shipping; trigger-level tests (bankCoinSidebar.test.ts exists); NO inline-JS DOM tests |
| `showCharInfoSidebar.ts` | shipping; trigger-level tests (charInfoSidebar.test.ts exists); NO inline-JS DOM tests |
| `showAdminMgmtSidebar.ts` | shipping (Phase 7); inline-JS coverage via adminMgmtSidebar.test.ts at trigger level only |

**Theme Picker:** scout couldn't find a `showThemePicker*Sidebar.ts` — likely a menu/alert flow, not a sidebar. Plan 08-02 should confirm during research and either include it or document its exclusion. The five `show*Sidebar.ts` files found are the actual coverage scope.

Search MRU current implementation: `apps-script/src/lib/searchIndex.ts:355-371` uses `CacheService.getDocumentCache()` with a `KEY_RECENT` key and `CACHE_TTL_SECONDS` TTL. Two functions: `getRecentSearches()` and `pushRecentSearch(query)`. JSON-encoded string array, capped at `RECENT_LIMIT = 3`.

`apps-script/vitest.config.ts` does NOT exist — vitest is currently running on its implicit default config. Plan 08-01 creates this file from scratch.

Phase 3 plan dir: `.planning/phases/03-apps-script-enrichment-foundation/` has 03-01-PLAN.md through 03-04-PLAN.md + CONTEXT/PATTERNS/RESEARCH + 03-SMOKE-TEST.md. Zero SUMMARY.md files. Phase 4: `.planning/phases/04-differentiator-features/` has 04-01..04 PLANs but zero SUMMARY.md files. Total DOC-04 backfill = 8 files.

Phase 5 SUMMARY.md template confirmed at `.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md` — frontmatter fields: phase, plan, subsystem, tags, requires, provides, affects, tech-stack (added/patterns), key-files (created/modified), decisions, metrics. Bodies are markdown narrative.

## Gray Areas Surfaced + Recommended Defaults Auto-Locked

In `--auto` mode, all 12 sub-questions across 4 areas resolved to the recommended option without user prompting. See CONTEXT.md `<decisions>` D-01 through D-08 for the locked decisions.

| Area | Question | Auto-selected (recommended) | Locked as |
|------|---------|------------------------------|-----------|
| JSDOM config shape | Global `environment: 'jsdom'` vs per-test annotation | Global | D-01 |
| Sidebar testability strategy | Test in-place vs extract SIDEBAR_BODY to .html (999.7) | Test in-place | D-02 |
| Test depth per sidebar | Happy-path only vs happy + error path vs full edge-case matrix | Happy + 1 error path | D-03 |
| Mount boilerplate | Inline per test vs shared `mountSidebar` helper | Shared helper in test-helpers.ts | D-04 |
| PropertiesService scope | `getUserProperties()` vs `getDocumentProperties()` | `getUserProperties()` (matches SEARCH-05 spec) | D-05 |
| Migration semantics | Clear-and-replace vs dual-write transition | Clear-and-replace | D-06 |
| DOC-04 fidelity | Phase 5 template byte-for-byte vs lighter retroactive | Phase 5 template byte-for-byte | D-07 |
| Plan structure | Single mega-plan vs split-by-stream | 4 plans, 2 waves (1 + 3 parallel) | D-08 |

Sub-questions inside each area resolved silently — encoding format (JSON.stringify, same as cache), KEY_RECENT key reuse, fail-closed return on parse error, etc.

## Scope Creep Surfaces (None Acted On)

- Coverage thresholds — could be set in vitest.config.ts during TEST-01 but expands scope. Logged as deferred.
- Snapshot tests for rendered sidebar HTML — JSDOM-friendly but maintenance burden. Logged as deferred.
- PropertiesService quota monitoring trigger — guild scale insufficient to warrant; logged for future.

## Auto-Mode Pass Cap

Single pass. CONTEXT.md written once. No additional passes triggered to "fill gaps." Per `--auto` mode contract, this discuss step is complete.

---

*Auto-advancing to `/gsd-plan-phase 8` per `--auto` chain semantics.*
