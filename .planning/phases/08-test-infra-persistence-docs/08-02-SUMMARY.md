---
phase: 08-test-infra-persistence-docs
plan: 02
subsystem: testing
tags: [apps-script, sidebar-tests, jsdom, inline-js, google-script-run, vitest, indirect-eval]

# Dependency graph
requires:
  - phase: 08-01
    provides: vitest jsdom environment + mountSidebar helper scaffold + getUserProperties mock
  - phase: 05-03
    provides: showSearchSidebar trigger + SIDEBAR_BODY pattern
  - phase: 05-04
    provides: showEvictionSidebar trigger + window.confirm flow
  - phase: 04-04
    provides: showBankCoinSidebar trigger
  - phase: 04-01
    provides: showCharInfoSidebar trigger
  - phase: 07-02
    provides: adminMgmtSidebar.test.ts trigger-call coverage (5th-sidebar gap fill)
provides:
  - 4 net-new sidebar inline-JS test files exercising the SAME contract live HtmlService iframes run
  - exported `buildSidebarHtml` on 4 sidebars (Search, Eviction, Bank-Coin, Char-Info) for test access
  - hardened mountSidebar helper that handles nested scripts AND cross-realm vm context
  - REQUIREMENTS.md TEST-02 wording correction (Theme Picker → Admin-Mgmt + historical-correction parenthetical)
affects: [08-04 (DOC-04 deferred-items doc backfill), v1.1 (admin-mgmt inline-JS test deferral)]

# Tech tracking
tech-stack:
  added: []  # no new dependencies; uses jsdom + vitest from 08-01
  patterns:
    - "mountSidebar(html) → MountedSidebar — single entry point for inline-JS tests; resolves enqueued google.script.run callbacks FIFO via dispatchRunCall / failRunCall."
    - "Indirect-eval `(0, eval)(src)` evaluates inline scripts in test realm globalThis, sidestepping JSDOM's separate vm context (the Plan 08-02 RED-phase finding)."
    - "querySelectorAll('script') on a detached <template>'s content extracts EVERY script regardless of nesting depth — required for sidebars whose <script> is inside the outer <div> wrapper."

key-files:
  created:
    - apps-script/src/__tests__/searchSidebar.inline.test.ts
    - apps-script/src/__tests__/evictionSidebar.inline.test.ts
    - apps-script/src/__tests__/bankCoinSidebar.inline.test.ts
    - apps-script/src/__tests__/charInfoSidebar.inline.test.ts
  modified:
    - apps-script/src/triggers/showSearchSidebar.ts
    - apps-script/src/triggers/showEvictionSidebar.ts
    - apps-script/src/triggers/showBankCoinSidebar.ts
    - apps-script/src/triggers/showCharInfoSidebar.ts
    - apps-script/src/__tests__/test-helpers.ts
    - .planning/REQUIREMENTS.md

key-decisions:
  - "Theme Picker is NOT a sidebar — it's `showThemePickerModal` in onOpen.ts:52-77, a `SpreadsheetApp.getUi().showModalDialog()` call. REQUIREMENTS.md TEST-02 historical wording was wrong; corrected to list Admin-Mgmt as the 5th surface."
  - "Admin-Mgmt inline-JS tests deferred to v1.1 — adminMgmtSidebar.test.ts already provides trigger-level coverage (Phase 7), and the 4 sidebars under this plan are the higher-value gap."
  - "Export `buildSidebarHtml` as a named function rather than refactoring to `HtmlService.createTemplateFromFile` (Option A locked in 05-03 / 05-04). Pure function, additive change; production trigger flow unchanged."
  - "mountSidebar uses indirect-eval `(0, eval)(src)` instead of appending <script> elements. The Plan 08-01 design appended scripts to document.head, but JSDOM evaluates those via vm.runInContext in a separate VM context — the test-side `window.google` stub was invisible to the script's globalThis. Indirect-eval runs in the test realm where `document`, `window`, and `google` all resolve to the test-side bindings."
  - "mountSidebar uses querySelectorAll('script') (recursive) instead of a top-level childNodes walk. Bank-Coin and Char-Info inline their <script> INSIDE the outer <div> wrapper; a top-level walk would miss them and let JSDOM auto-evaluate them in its separate vm context."

patterns-established:
  - "Sidebar inline-JS test pattern: import {buildSidebarHtml} from triggers; mountSidebar(buildSidebarHtml(theme)); dispatchRunCall(method, payload) for happy paths; failRunCall(method, err) for error paths; assert DOM mutations on the test-realm document."
  - "Tests assert on real DOM IDs read from each sidebar's SIDEBAR_BODY source-of-truth — if a trigger changes its IDs the test fails loudly (T-08-02-01 mitigation)."
  - "window.confirm gotcha mitigation: any sidebar that calls window.confirm before a callback MUST stub it via vi.spyOn(window, 'confirm').mockReturnValue(true) before clicking the relevant button. Documented in evictionSidebar.inline.test.ts header (08-RESEARCH Pitfalls #3)."

requirements-completed: [TEST-02]

# Metrics
duration: 10min
completed: 2026-05-12
---

# Phase 08 Plan 02: Sidebar inline-JS tests Summary

**4 net-new sidebar inline-JS test files (Search, Eviction, Bank-Coin, Char-Info) exercising the live SIDEBAR_BODY against a JSDOM-mounted google.script.run mock; mountSidebar helper hardened to handle nested scripts + cross-realm vm context; REQUIREMENTS.md TEST-02 wording corrected to drop the Theme-Picker misnomer.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-05-12T16:57:31Z
- **Completed:** 2026-05-12T17:07:44Z
- **Tasks:** 3
- **Files modified:** 10 (4 created tests, 4 modified sidebar triggers, test-helpers.ts, REQUIREMENTS.md)

## Accomplishments

- **All 5 shipping sidebars now under vitest coverage** — 4 net-new inline-JS files (Search, Eviction, Bank-Coin, Char-Info) plus the existing adminMgmtSidebar.test.ts trigger-call companion from Phase 7. TEST-02 closeable.
- **8 net-new test cases** (4 files × 2 cases each: happy + error per D-03) covering google.script.run callback wiring, DOM event handlers, error rendering, and `window.confirm`-gated commit flow.
- **mountSidebar helper hardened** with two Rule-1 bug fixes (see Deviations) so it actually works against real sidebar HTML — Plan 08-01 shipped the helper but no test exercised it; the latent bugs surfaced immediately when Plan 08-02 wrote the first real test.
- **REQUIREMENTS.md TEST-02 wording corrected** — Theme Picker (a `showModalDialog` in `onOpen.ts:52-77`, NOT a sidebar) removed; Admin-Mgmt added; admin-mgmt inline-JS deferral to v1.1 documented inline.
- **Full apps-script suite ends green at 336/336** (328 baseline + 8 new), build clean, typecheck clean.

## Task Commits

1. **Task 1: Export `buildSidebarHtml` in 4 sidebars + REQUIREMENTS.md TEST-02 wording fix**
   - `e7320cc` (refactor — 4-sidebar export rename)
   - `c0a6606` (docs — REQUIREMENTS.md TEST-02 wording correction)
2. **Task 2: Search + Eviction inline-JS tests (TDD GREEN)**
   - `1e4eb26` (fix — mountSidebar uses indirect-eval, Rule 1 auto-fix)
   - `be37ae2` (test — searchSidebar.inline.test.ts + evictionSidebar.inline.test.ts)
3. **Task 3: Bank-Coin + Char-Info inline-JS tests (TDD GREEN)**
   - `1a2fc21` (fix — mountSidebar nested-script extraction, Rule 1 auto-fix)
   - `77d4fcb` (test — bankCoinSidebar.inline.test.ts + charInfoSidebar.inline.test.ts)

_Note: Plan 08-02 is TDD-style under each task. Auto-fix commits to test-helpers.ts (Rule 1) precede the GREEN test commits because the latent mountSidebar bugs blocked the new tests from passing._

## Files Created/Modified

**Created (4):**
- `apps-script/src/__tests__/searchSidebar.inline.test.ts` — TS1 (initial-data → search → render) + TS2 (runSearch failure → #results error region).
- `apps-script/src/__tests__/evictionSidebar.inline.test.ts` — TE1 (emails load → preview → confirm + commit, with `window.confirm` stubbed true) + TE2 (getEvictionEmails failure → #msg.error).
- `apps-script/src/__tests__/bankCoinSidebar.inline.test.ts` — TB1 (initial-data populates pp/gp/sp/cp + enables #saveBtn) + TB2 (getBankCoinForForm failure → #msg with red color).
- `apps-script/src/__tests__/charInfoSidebar.inline.test.ts` — TC1 (chars list populates #charBody tbody + enables #saveBtn) + TC2 (getCharsForForm failure → #msg).

**Modified (6):**
- `apps-script/src/triggers/showSearchSidebar.ts` — `function buildSidebarHtml` → `export function buildSidebarHtml` (additive, 1-line change).
- `apps-script/src/triggers/showEvictionSidebar.ts` — same export rename.
- `apps-script/src/triggers/showBankCoinSidebar.ts` — same export rename.
- `apps-script/src/triggers/showCharInfoSidebar.ts` — same export rename.
- `apps-script/src/__tests__/test-helpers.ts` — mountSidebar rewritten to (a) extract nested `<script>` via `querySelectorAll`, (b) execute extracted scripts via indirect-eval `(0, eval)(src)` instead of appending to document.head.
- `.planning/REQUIREMENTS.md` — TEST-02 acceptance line updated (Theme Picker removed; Admin-Mgmt added; historical-correction parenthetical).

## Decisions Made

- **Theme Picker is a modal, not a sidebar.** `showThemePickerModal` lives in `apps-script/src/triggers/onOpen.ts:52-77` and calls `SpreadsheetApp.getUi().showModalDialog(...)`. The original TEST-02 wording listed it as a sidebar — historical inaccuracy now corrected. Modal HTML can be tested in a future plan if needed, but Plan 08-02 scope is sidebars only.
- **Admin-Mgmt inline-JS tests deferred to v1.1.** `apps-script/src/__tests__/adminMgmtSidebar.test.ts` from Phase 7 (Plan 07-02) provides trigger-level coverage for the 5th sidebar. The 4 sidebars under this plan are the higher-value gap.
- **`buildSidebarHtml` exported as a named function rather than refactored to `HtmlService.createTemplateFromFile`.** Option A (inline SIDEBAR_BODY String.raw constant) was locked in 05-03 / 05-04; the export rename is purely additive, the production trigger flow is unchanged, and a future v1.0.x polish could still extract to companion .html files without touching test files.
- **mountSidebar uses indirect-eval `(0, eval)(src)`.** Plan 08-01's design appended `<script>` elements to `document.head`, but JSDOM evaluates those via `vm.runInContext` against a SEPARATE VM context from the test realm. Symptom: `ReferenceError: google is not defined` even after `window.google = ...` was set. Indirect-eval runs in the test realm's global scope where `document`, `window`, and `google` all resolve to the test-side bindings.
- **mountSidebar uses `querySelectorAll('script')` (recursive) instead of a top-level `childNodes` walk.** Bank-Coin and Char-Info inline their `<script>` inside the outer `<div>` wrapper. A top-level walk would miss them and let JSDOM auto-evaluate when the fragment is appended to body — re-triggering the cross-realm failure.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] mountSidebar helper appended scripts to document.head; JSDOM evaluated them in a separate vm context invisible to the test realm.**

- **Found during:** Task 2 (Search + Eviction tests). First real consumer of the mountSidebar helper (Plan 08-01 shipped the helper but no test exercised it).
- **Issue:** `(window as ...).google = ...` set on the test-side `window` was invisible to inline scripts evaluated by JSDOM. Symptomatic error: `ReferenceError: google is not defined` thrown from inside the sidebar's init() the moment the helper appended the `<script>` to document.head. Probe tests confirmed JSDOM evaluates inline scripts via `vm.runInContext` against a separate context whose `window` is NOT the test-side `window`.
- **Fix:** Replaced `document.head.appendChild(s)` with `(0, eval)(src)`. Indirect-eval evaluates the script source as a Program in the test realm's global scope; top-level `var` / `function` declarations attach to the test-realm `globalThis` (aliased to `window` by vitest's jsdom env), and references to `google` / `document` / `window` resolve to the test-side bindings.
- **Files modified:** `apps-script/src/__tests__/test-helpers.ts` (mountSidebar helper)
- **Verification:** searchSidebar.inline.test.ts + evictionSidebar.inline.test.ts go from 4/4 failed → 4/4 passed. Eval risk bounded: test-helpers.ts is never imported from src/Code.ts (esbuild entry), so the helper never ships to dist/Code.js.
- **Committed in:** `1e4eb26` (separate fix commit before GREEN test commit, per audit trail clarity)

**2. [Rule 1 - Bug] mountSidebar helper's top-level childNodes walk missed nested `<script>` tags.**

- **Found during:** Task 3 (Bank-Coin + Char-Info tests). Both sidebars inline their `<script>` INSIDE the outer `<div>` wrapper rather than at top level.
- **Issue:** Walking only `tpl.content.childNodes` (top-level) left nested scripts in the document fragment. When the fragment was appended to `document.body`, JSDOM auto-evaluated those scripts in its separate vm context — re-triggering the exact `google is not defined` failure the indirect-eval fix was supposed to prevent (since the helper never extracted the script to evaluate it itself).
- **Fix:** Switched to `tpl.content.querySelectorAll('script')` (recursive across the whole fragment) to capture every script regardless of nesting depth. Each captured script is removed from its parent BEFORE the fragment is appended, so JSDOM never sees them as inline DOM scripts.
- **Files modified:** `apps-script/src/__tests__/test-helpers.ts` (mountSidebar helper)
- **Verification:** bankCoinSidebar.inline.test.ts + charInfoSidebar.inline.test.ts go from 4/4 failed → 4/4 passed. Search + Eviction (top-level scripts) remain green — the recursive selector is a strict superset of the prior behavior.
- **Committed in:** `1a2fc21` (separate fix commit before second GREEN test commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 - bug fixes in the Wave-1-shipped mountSidebar helper)

**Impact on plan:** Both auto-fixes were essential to make ANY mountSidebar-based test pass. Plan 08-01 shipped the helper without exercising it; the latent bugs surfaced immediately when Plan 08-02 wrote the first real test. Both fixes are test-only and never bundled into dist/Code.js (esbuild entry is src/Code.ts, which does NOT import test-helpers). No scope creep — both fixes were prerequisites to delivering the plan's stated outputs.

## Issues Encountered

**Vitest's `jsdom` environment defaults `runScripts: "dangerously"` (so inline `<script>` elements DO execute), but the execution context is isolated from the test realm.** The contradiction surfaced when probe tests showed JSDOM throwing errors from inside script bodies (proving execution) while writes from those scripts to `window.*` were invisible on the test side (proving cross-realm isolation). The indirect-eval workaround is the standard fix for this vitest/jsdom limitation; documented in the mountSidebar helper's header so future maintainers understand why we don't simply `document.head.appendChild(scriptElement)`.

## User Setup Required

None — Plan 08-02 is test-only and ships no new dependencies or external services.

## Next Phase Readiness

- TEST-02 closeable: 5/5 shipping sidebars under at least one form of vitest coverage. ROADMAP / REQUIREMENTS update deferred to the orchestrator (per execution context: "Do NOT update STATE.md or ROADMAP.md").
- mountSidebar helper now battle-tested against 4 real sidebars with varying script-nesting patterns; the helper is ready for any future sidebar that needs inline-JS coverage (e.g., admin-mgmt inline-JS in v1.1).
- The two helper fixes also benefit any FUTURE consumer of mountSidebar — they're not search-or-bank specific.
- No blockers for Wave 3 (08-03 / 08-04) — Plan 08-02 touches only test files, 4 sidebar trigger exports, and REQUIREMENTS.md.

## Self-Check: PASSED

Verified the following claims before marking complete:

**Files created (4):**
- `apps-script/src/__tests__/searchSidebar.inline.test.ts` — FOUND
- `apps-script/src/__tests__/evictionSidebar.inline.test.ts` — FOUND
- `apps-script/src/__tests__/bankCoinSidebar.inline.test.ts` — FOUND
- `apps-script/src/__tests__/charInfoSidebar.inline.test.ts` — FOUND

**Commits in git log (6 plan commits):**
- `e7320cc` refactor(08-02): export buildSidebarHtml in 4 sidebars — FOUND
- `c0a6606` docs(08-02): correct TEST-02 wording — FOUND
- `1e4eb26` fix(08-02): mountSidebar uses indirect-eval — FOUND
- `be37ae2` test(08-02): add search + eviction sidebar inline-JS tests — FOUND
- `1a2fc21` fix(08-02): mountSidebar extracts nested <script> tags — FOUND
- `77d4fcb` test(08-02): add bank-coin + char-info sidebar inline-JS tests — FOUND

**Verification gates:**
- `npm test` ends with 336/336 passed (328 baseline + 8 new) — PASS
- `npm run build` exits 0 — PASS
- `npx tsc --noEmit` exits 0 — PASS
- `grep -c "^export function buildSidebarHtml"` returns 1 for each of the 4 sidebars under test — PASS
- `grep -c "Theme Picker" .planning/REQUIREMENTS.md` returns 0 — PASS
- `grep -c "Admin-Mgmt\|adminMgmtSidebar" .planning/REQUIREMENTS.md` returns ≥1 — PASS
- Schema gates unchanged: `writeMetaRow.*schema_version.*'3'` count = 1; `WatcherMaxSchemaVersion = 3` — PASS

---
*Phase: 08-test-infra-persistence-docs*
*Completed: 2026-05-12*
