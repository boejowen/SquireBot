# Phase 8: Test Infra + Persistence + Docs Backfill — Context

**Gathered:** 2026-05-12
**Status:** Ready for planning
**Mode:** `/gsd-discuss-phase 8 --auto` (Claude auto-selected the recommended option in every gray area)

<domain>
## Phase Boundary

Close the v1.0 carry-over debt that wasn't installer- or admin-shaped:

1. **TEST-01** — Wire JSDOM into `apps-script/vitest.config.ts` so `npm test` exercises sidebar inline-JS (DOM events, payload assembly, error rendering) without a separate command.
2. **TEST-02** — Every shipping sidebar (Search, Eviction, Bank-Coin, Char-Info, Theme Picker) gets a co-located `__tests__/*-sidebar.test.ts` companion that mounts the sidebar HTML into JSDOM and exercises real handlers.
3. **SEARCH-05** — Migrate the recent-3 search MRU from `CacheService.getDocumentCache()` (25-min TTL, ephemeral) to `PropertiesService.getUserProperties()` (per-user, persistent across sessions and quota tier renewal).
4. **DOC-04** — Backfill `SUMMARY.md` files for the 8 plans in Phases 3 and 4 that shipped without one. Follow the Phase 5 SUMMARY.md template (provides / affects / tech-stack / key-files / decisions / metrics).

**Hard non-goals (push back if surfaced):**

- Extracting `SIDEBAR_BODY` constants to standalone `.html` files (backlog 999.7 — cosmetic refactor, not in v1.0.1 scope).
- Adding new features to the sidebars under the banner of "we're touching them anyway."
- Schema changes. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3 (verification hook 5 grep gate, same as Phase 7).
- Self-service eviction, bank-coin permission lock, wantlist+Discord (999.1 / 999.5 / 999.12 — all out of scope).

</domain>

<decisions>
## Implementation Decisions (auto-locked recommended defaults)

### D-01 — JSDOM environment is globally configured in vitest.config.ts (not per-test annotation)

`apps-script/vitest.config.ts` declares `environment: 'jsdom'` at the top level so every existing test and every new sidebar test gets a DOM by default. Per-test `// @vitest-environment node` overrides remain available for the handful of tests that need to assert "the SUT didn't accidentally touch `document`" (none currently exist; the door stays open).

**Why this default beats per-test:** sidebar tests outnumber non-DOM tests by ~5:1 once TEST-02 ships, so the inversion is correct. Existing tests that use Apps Script mocks (`Session`, `SpreadsheetApp`, `CacheService`) don't care about the DOM — JSDOM presence is a no-op for them.

**Quota check:** `jsdom` ships ~1.5MB and is already an indirect dep via vitest's `happy-dom` alternative resolution. Confirmed by `apps-script/package.json` audit during scout.

### D-02 — Sidebar tests mount the existing HTML string verbatim — no SIDEBAR_BODY extraction (backlog 999.7 deferred)

Each sidebar's `SIDEBAR_BODY` template literal in `apps-script/src/triggers/show*Sidebar.ts` stays where it is. Tests import the template-building function (`buildSidebarHtml(theme)` or equivalent), call it with a fixture theme, and feed the result string into `mountSidebar(htmlString)` (see D-04). Inline `<script>` blocks execute under JSDOM the same way they execute in the live HtmlService iframe.

**Why not extract:** 999.7 is a 5-sidebar refactor with its own threat model (CSP changes, theme-token interpolation safety). It belongs in v1.1 cosmetic-cleanup, not v1.0.1 test-debt close. Adding it here doubles the surface area for review.

**Constraint downstream agents must honor:** if you find yourself wanting to extract a sidebar's HTML to make it testable, STOP — that's a signal you're testing the wrong thing. The string-build function is testable as-is.

### D-03 — Test scope per sidebar = happy path + at least 1 error path

For each of the 5 sidebars (Search, Eviction, Bank-Coin, Char-Info, Theme Picker):

- **Happy path** — primary user flow: render → user interacts → google.script.run callback fires → success handler renders new state. Exactly one test.
- **Error path** — primary failure: callback `withFailureHandler` fires → error message renders in the visible status region. Exactly one test.
- **Optional**: additional cases (empty state, validation rejection, double-click guard) IF the existing trigger-side test for that sidebar (e.g., `showEvictionSidebar.test.ts`) explicitly references the case and the JSDOM test is a clean way to lock the behavior. Otherwise defer to backlog.

**Why this depth:** the existing `*sidebar.test.ts` files already cover server-side logic (`getEvictionEmails`, `previewEviction`, `commitEviction`, etc.) at trigger-call depth. The TEST-02 gap is the inline-JS layer — DOM event handlers, payload assembly, the `withSuccessHandler`/`withFailureHandler` routing. Two tests per sidebar is enough to lock that contract; more risks testing JSDOM rather than SquireBot.

**TEST-02 acceptance proof:** `find apps-script/src/__tests__ -name "*sidebar.test.ts"` returns ≥ 6 files (5 new + 1 existing for admin-mgmt). Each new file has ≥ 2 `it(...)` cases.

### D-04 — Shared `mountSidebar(html: string)` helper lives in `apps-script/src/__tests__/test-helpers.ts`

Add a single helper alongside the existing `resetMocks` / `makeSheet` / `seedMeta` / `installSessionMock` exports:

```typescript
export function mountSidebar(html: string): { window: Window; document: Document; google: typeof globalThis.google } {
  // Sets document.body.innerHTML to html, executes inline <script> blocks
  // under the current JSDOM realm, stubs window.google.script.run with a
  // controllable promise-based mock, returns the realm + the mock so tests
  // can drive interactions and assert post-conditions.
}
```

The mock for `google.script.run` returns a fluent object exposing `.withSuccessHandler(fn).withFailureHandler(fn).METHOD_NAME(...)`; each call is enqueued and resolvable via a `state.dispatchRunCall(methodName, response)` helper, modeled on the existing `state.lockTryLockReturn` pattern.

**Why a shared helper:** five sidebars × two tests each = 10 mount sites. Inline-per-test would be ~30 lines of boilerplate × 10 = 300 lines of churn. The helper makes each test ~20 lines and forces a single mount contract.

### D-05 — SEARCH-05 PropertiesService scope is `getUserProperties()` (per-user, NOT document-shared)

REQUIREMENTS.md SEARCH-05 says this explicitly: "Recent-3 MRU is per-guildie state, not workbook-shared state." Re-locked here so downstream agents don't second-guess.

`apps-script/src/lib/searchIndex.ts:355-371` swaps both `getRecentSearches` and `pushRecentSearch` from `CacheService.getDocumentCache()` to `PropertiesService.getUserProperties()`. The property key stays `KEY_RECENT` (defined at the top of the file). The JSON-encoded-string-array storage shape is identical to the cache shape — no parse changes downstream.

**Why getUserProperties not getDocumentProperties:** per-user means the workbook owner's recent searches don't bleed into other guildies' UI; matches the implicit "my recent searches" expectation a guildie has when reopening the sidebar.

**Quota safety:** PropertiesService caps at 500 KB per scope. Three queries × ~50 chars × overhead ≈ 200 bytes. Zero quota concern. No bounded-size guard needed.

### D-06 — Migration semantics = clear-and-replace (no dual-write, no cache backfill)

The old `CacheService.getDocumentCache()` MRU is ephemeral by definition (25-min TTL, document-scoped). On the day SEARCH-05 ships, every guildie's recent-searches state is at most 25 minutes old. We DROP the old write path entirely — `pushRecentSearch` writes to PropertiesService only; `getRecentSearches` reads from PropertiesService only. No dual-write transition period; no migration that reads cache → writes properties.

**Why clear-and-replace is safe:** worst-case UX impact is "the recent-3 list is empty for your first search after the v1.0.1 push" — a negligible single-session regression. Implementing dual-write would add a code path that has to be removed in v1.1 (decision rot risk) and would never observably help any user.

### D-07 — DOC-04 backfill matches Phase 5 SUMMARY.md template byte-for-byte (provides / affects / tech-stack / key-files / decisions / metrics)

Eight retroactive `SUMMARY.md` files (4 in `.planning/phases/03-apps-script-enrichment-foundation/`, 4 in `.planning/phases/04-differentiator-features/`) get authored using the Phase 5 template as the canonical reference. See `.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md` for the frontmatter shape.

**Source of truth for content:** git log (commits matching `03-0N` / `04-0N` patterns), the existing `*-PLAN.md` files, and STATE.md "Last Session Summary" entries for the relevant 2026-04-30 → 2026-05-04 window. The v1.0 milestone archive at `.planning/milestones/v1.0-ROADMAP.md` is the chronological backbone.

**Verification depth:** existence grep gate on all 8 SUMMARY.md paths + spot-check of `key-files.created` and `decisions` arrays. The 8 docs do NOT need to be deeply auditable — they need to exist so the v1.0 milestone audit's "Phase 3/4 documentation debt" line item can be retired.

### D-08 — Plan structure = 4 plans across 2 waves

- **Plan 08-01 (wave 1, autonomous):** TEST-01 — JSDOM environment in `apps-script/vitest.config.ts` + `mountSidebar` helper in `test-helpers.ts`. Dependency root for Plan 08-02.
- **Plan 08-02 (wave 2, autonomous, depends_on: [08-01]):** TEST-02 — 5 new co-located sidebar test files (`searchSidebar.test.ts`, `evictionSidebar.test.ts` if absent, `bankCoinSidebar.test.ts`, `charInfoSidebar.test.ts`, `themePickerSidebar.test.ts`) each with happy-path + error-path tests using the helper from Plan 08-01.
- **Plan 08-03 (wave 1, autonomous, parallel with 08-01):** SEARCH-05 — swap `CacheService.getDocumentCache()` to `PropertiesService.getUserProperties()` in `searchIndex.ts`; update existing `searchIndex.test.ts` mocks; ensure the existing `showSearchSidebar.test.ts` still passes.
- **Plan 08-04 (wave 1, autonomous, parallel with 08-01 and 08-03):** DOC-04 — author 8 retroactive `SUMMARY.md` files using the Phase 5 template. Pure docs; no source files touched.

**Wave 1 parallelism check:** Plans 01, 03, 04 touch zero overlapping source files (vitest.config.ts + test-helpers.ts vs. searchIndex.ts + searchIndex.test.ts vs. .planning/phases/03-*/04-*/*-SUMMARY.md). Safe for parallel execution.

**Ship gate:** `clasp push` of the apps-script bundle to the dev workbook (SEARCH-05 is a code change that ships, even though it's small) + green CI + v1.0.1 milestone retrospective + tag `v1.0.2` if a watcher rebuild ships, OR no tag if pure apps-script (decision deferred to ship time — only needed if SEARCH-05's PropertiesService access requires a manifest scope change; PropertiesService is covered by `script.scriptapp` already).

**Watcher rebuild check:** zero Go code touched in Phase 8. Manifest unchanged (PropertiesService is already in-scope via `script.scriptapp`). No `_meta.schema_version` bump. → No watcher rebuild ships; v1.0.1 stays as the binary release.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner, executor) MUST read these before acting:**

### Phase scope + requirements
- `.planning/ROADMAP.md` — Phase 8 success criteria (lines 70–82 in v1.0.1 section)
- `.planning/REQUIREMENTS.md` — TEST-01, TEST-02, SEARCH-05, DOC-04 acceptance criteria

### Test infrastructure analogs (TEST-01/02)
- `apps-script/vitest.config.ts` — does not exist yet; this phase creates it. Look at how `apps-script/package.json` declares vitest (`"test": "vitest run"`) — no config file currently means vitest is using its default discovery.
- `apps-script/src/__tests__/test-helpers.ts` — existing helper exports (resetMocks, makeSheet, seedMeta, installSessionMock); the `mountSidebar` helper from D-04 lands here.
- `apps-script/src/__tests__/showEvictionSidebar.test.ts` — template for server-side (trigger callback) sidebar tests. The TEST-02 inline-JS tests are a NEW companion shape, not a replacement.
- `apps-script/src/__tests__/showSearchSidebar.test.ts` — same template, already covers the search trigger callbacks server-side.
- `apps-script/src/__tests__/adminMgmtSidebar.test.ts` — only existing sidebar test that already exercises the opener+callbacks shape Plan 08-02 will replicate for the other four.

### Sidebar source-of-truth (TEST-02 surfaces under test)
- `apps-script/src/triggers/showSearchSidebar.ts` — SIDEBAR_BODY with theme tokens + recent-list rendering + Did-you-mean suggestions
- `apps-script/src/triggers/showEvictionSidebar.ts` — eviction sidebar (already has guards from Phase 7; tests must seed `_meta.guild_admins`)
- `apps-script/src/triggers/showBankCoinSidebar.ts` — bank-coin sidebar
- `apps-script/src/triggers/showCharInfoSidebar.ts` — char-info sidebar
- `apps-script/src/triggers/themePicker.ts` or wherever — find Theme Picker entry point; if it's the existing menu item not a sidebar, exclude from TEST-02 scope and update REQUIREMENTS.md note

### Persistence migration (SEARCH-05)
- `apps-script/src/lib/searchIndex.ts:355-371` — `getRecentSearches` + `pushRecentSearch` current cache-based implementation (this is the surgical-change site)
- `apps-script/src/lib/searchIndex.ts` top of file — `KEY_RECENT`, `RECENT_LIMIT`, `CACHE_TTL_SECONDS` constants; `CACHE_TTL_SECONDS` becomes unused after Plan 08-03 lands (acceptable to leave or delete; planner decides)
- `apps-script/src/__tests__/searchIndex.test.ts` — current mock for CacheService; rewire to PropertiesService mock

### Doc backfill (DOC-04)
- `.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md` — canonical Phase 5 template to clone the frontmatter from
- `.planning/phases/03-apps-script-enrichment-foundation/03-0{1,2,3,4}-PLAN.md` — Phase 3 plan files (4 SUMMARY.md targets)
- `.planning/phases/04-differentiator-features/04-0{1,2,3,4}-PLAN.md` — Phase 4 plan files (4 SUMMARY.md targets)
- `.planning/milestones/v1.0-ROADMAP.md` — chronological execution log of v1.0; contains the Phase 3/4 "Last Session Summary" entries that feed the SUMMARY.md decisions sections
- `.planning/STATE.md` — current frontmatter + decisions log (post-Phase-7); doesn't drive backfill content but the planner should reference it to avoid double-counting

### Project rules
- `./CLAUDE.md` — apps-script TypeScript conventions, structured-logging contract, schema-evolution rules, test fixture naming
- `.planning/PROJECT.md` — core value, locked decisions, technology stack constraints
- `.planning/research/PITFALLS.md` — 27 pitfalls catalogue; Pitfall P6 (LockService) applies to nothing in Phase 8; check PropertiesService quota pitfalls if any
- `.planning/research/STACK.md` — locked stack (vitest 1.6, esbuild 0.20+, clasp 2.4)

### v1.0.1 ship gate context
- `docs/apps-script-deploy.md` — clasp push runbook; Phase 8 ships via this path (apps-script-only)
- `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SMOKE.md` — example smoke evidence shape (Phase 8 doesn't need a smoke this deep, but a "verify recent-searches persists across browser-tab close" smoke for SEARCH-05 is the user-acceptance bar)

</canonical_refs>

<specifics>
## Specific Ideas

- Sidebar identification: of the 5 named in TEST-02 (Search, Eviction, Bank-Coin, Char-Info, Theme Picker), confirm during research whether "Theme Picker" is a sidebar or just a menu/alert flow. Scout found `showAdminMgmtSidebar.ts`, `showBankCoinSidebar.ts`, `showCharInfoSidebar.ts`, `showEvictionSidebar.ts`, `showSearchSidebar.ts` — five sidebars total, none named "themePicker". The Phase 7 admin-mgmt sidebar shipped AFTER REQUIREMENTS.md was written; it's the 5th sidebar that should appear in TEST-02's test coverage even though SEARCH-05's authors didn't know about it. **Recommendation:** Plan 08-02 lands tests for all 5 currently-shipping sidebars; if Theme Picker is truly only a menu, document that in the SUMMARY and skip; if Theme Picker is a sidebar that lives elsewhere, find it and include it.

- `apps-script/vitest.config.ts` doesn't exist yet — the default vitest config is implicit. Plan 08-01 creates the file from scratch with `environment: 'jsdom'`, `include: ['src/__tests__/**/*.test.ts']` (or whatever the existing default discovery is), and any path-alias config needed.

- `mountSidebar` helper must handle inline `<script>` execution carefully — JSDOM doesn't auto-execute script tags injected via `innerHTML`. Pattern: parse the HTML, extract `<script>` text content, create a fresh script element via `document.createElement('script')`, set `.textContent`, append to `document.head`. Standard JSDOM pattern; not novel.

- For SEARCH-05, the existing `searchIndex.test.ts` uses a Map-backed CacheService mock per Phase 5 patterns. The new PropertiesService mock should be the same shape (`getProperty(key)` / `setProperty(key, value)`) — add to `test-helpers.ts` as `makePropertiesServiceMock()`.

</specifics>

<deferred>
## Deferred Ideas

- **999.7 (recurring) — Extract `SIDEBAR_BODY` to `apps-script/src/sidebars/*.html`** — re-confirmed not in Phase 8 scope. Re-evaluate in v1.1 cosmetic cleanup; would also enable HTML-linting CI.
- **Coverage thresholds in vitest.config.ts** — could set `coverage.thresholds.lines: 70` etc. as part of TEST-01 but expands scope unnecessarily for v1.0.1. Defer to v1.1.
- **Snapshot tests for sidebar rendered HTML** — vitest supports them; not worth the maintenance burden when the test suite already exercises the build functions imperatively. Defer.
- **PropertiesService quota monitoring trigger** — currently no quota guard; if guild ever balloons to 100 users this could matter. Defer to 999.X.
- **Backfilling SUMMARY.md for milestone v1.0 (the milestone-level retrospective)** — out of scope; the milestone audit at `.planning/milestones/v1.0-MILESTONE-AUDIT.md` already exists.

## Claude's Discretion

(Implementation details the planner picks during research/planning, not pre-locked here:)

- Exact ordering of fields inside each new sidebar test file (`describe` block hierarchy, beforeEach setup, etc.). Match `adminMgmtSidebar.test.ts` shape.
- Whether `mountSidebar` returns the `Document` or a wrapping object — pick whichever produces the cleanest test bodies.
- Whether to add a typecheck step to CI specifically for the new `.test.ts` files. The existing `tsc --noEmit` already covers them.
- For DOC-04: prose voice in the 8 retroactive SUMMARY.md files. Past tense, declarative; mirror Phase 5's voice.
- For SEARCH-05: whether to delete the now-unused `CACHE_TTL_SECONDS` constant or leave it. Tiny clean-up; planner's call.

</deferred>

---

*Phase: 08-test-infra-persistence-docs*
*Context gathered: 2026-05-12 via `/gsd-discuss-phase 8 --auto` (single pass, recommended defaults locked)*
