---
phase: 10-apps-script-test-quality
plan: 03
plan_id: 10-03-ship-gate
type: execute
wave: 3
depends_on: [10-01-test04-fixes, 10-02-admin-mgmt-inline-test]
files_modified: []
autonomous: false
requirements: []
tags: [ship-gate, clasp-push, dev-workbook, smoke, wave3, checkpoint]

must_haves:
  truths:
    - "Typecheck is clean: `cd apps-script && npx tsc --noEmit` exits 0."
    - "Bundle build is clean: `cd apps-script && npm run build` exits 0 and produces an updated `apps-script/dist/Code.js` IIFE."
    - "Full apps-script vitest suite passes green with ≥ 340 tests (336 baseline + 2 new Admin-Mgmt TM1/TM2 + ≥ 2 more from any unrelated growth in the suite since baseline; if exactly 338, accept and document)."
    - "Schema lock unchanged: `_meta.schema_version` = 3 in `apps-script/src/lib/migrations.ts`; `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go`."
    - "All 4 WR-* warning-level findings from 08-REVIEW.md are closed (Plan 10-01 verified)."
    - "5/5 shipping sidebars have inline-JS test coverage (Plan 10-02 verified)."
    - "USER CHECKPOINT: user reviewed the green-CI evidence summary and explicitly responded 'approved' (or equivalent) before any clasp push runs."
    - "`clasp push` from the workbook owner's machine succeeded — exit 0 — and the dev-workbook now serves the new `dist/Code.js` bundle."
    - "Dev-workbook manual smoke check: each of the 5 sidebars (Search, Eviction, Bank-Coin, Char-Info, Admin-Mgmt) opens without browser-console errors and renders its expected UI."
  artifacts: []
  key_links:
    - from: "green CI evidence (typecheck + build + tests + schema gates)"
      to: "user-approval checkpoint"
      via: "Task 2 — present evidence summary; wait for explicit 'approved' before Task 3"
      pattern: "checkpoint:human-verify"
    - from: "user approval"
      to: "clasp push"
      via: "Task 3 — credentialed action from the workbook owner's machine, NOT automatable from CI per .planning/PROJECT.md clasp v2.4+ Desktop-OAuth constraint"
      pattern: "clasp push"
    - from: "clasp push"
      to: "dev-workbook smoke"
      via: "Task 4 — manual UAT-equivalent; catches HtmlService quirks vitest+JSDOM cannot"
      pattern: "smoke"
---

<objective>
Run the ship gate for Phase 10. This plan ships the work landed by Plans 10-01 (TEST-04 fixes) and 10-02 (TEST-03 Admin-Mgmt inline test) to the dev workbook via `clasp push` and verifies the apps-script bundle works end-to-end with a manual smoke check of all 5 sidebars.

Per CONTEXT.md D-04 (locked under the "simple, seamless" criterion D-06), the ship-gate ordering is:

1. **CI green BEFORE clasp push** (defensive — prevents shipping a broken bundle to dev workbook).
2. **User checkpoint BEFORE clasp push** (`autonomous: false` — clasp is a per-developer credentialed action; not automatable from CI per PROJECT.md).
3. **Manual dev-workbook smoke AFTER clasp push** (the only end-user-facing UAT in this phase; catches HtmlService runtime quirks vitest+JSDOM cannot).

This plan is **`autonomous: false`** — the executor MUST pause at Task 2's checkpoint and wait for the user to explicitly approve before running `clasp push`. The user will be at the keyboard for the credentialed action (per memory `feedback_toolchain_installs.md` — clasp is the user's tool to invoke).

**No source files are modified by this plan.** It is a verification + deploy + smoke gate. The only filesystem side-effect is `apps-script/dist/Code.js` produced by `npm run build` (which is a build artifact, not source — `dist/` is typically gitignored or committed-as-build-output per existing convention).

**Scope discipline — non-negotiable (per CONTEXT.md D-01 + D-05):**
- Do NOT edit any source file in this plan. If a test fails during Task 1, do NOT silently fix it — surface to the user as a Phase 10 deviation note + propose either (a) a revision pass on Plan 10-01 or 10-02, or (b) ship-gate-acceptable deferral with a v1.1 backlog candidate.
- Do NOT auto-`clasp push`. The `autonomous: false` flag is load-bearing here. Wait for explicit user approval.
- Do NOT touch `_meta.schema_version` or `WatcherMaxSchemaVersion`. Verification grep gates are READ-ONLY checks.
- Do NOT tag a v1.0.2-phase10 git tag or push a GitHub Release. Per ROADMAP Phase 10 ship gate: this is apps-script-only and ships via `clasp push`, NOT via tag → binary release (that path is reserved for the Go watcher).

**Handling Plan 10-01 Task 3 deviation (WR-03):**
If Plan 10-01 Task 3 caused `searchIndex.test.ts` Test 4 to fail (the unguarded plan-locked toEqual now fails under whole-string Levenshtein math), this plan MUST NOT silently re-introduce the try/catch. Instead:
- Surface the failure to the user at Task 2's checkpoint with the full failure diff.
- Propose either: (a) accept ship gate WITHOUT a fix (the test fails loudly — that IS the WR-03 intent) and open v1.1 backlog 999.30 to reconcile `didYouMean` semantics with the plan-locked assertion; (b) execute a separate Plan 10-04 to revise the assertion or the `didYouMean` implementation; (c) defer Phase 10 ship until a separate fix lands.
- Let the USER decide which path. Do NOT auto-decide.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/10-apps-script-test-quality/10-CONTEXT.md
@.planning/phases/10-apps-script-test-quality/10-01-test04-fixes-PLAN.md
@.planning/phases/10-apps-script-test-quality/10-02-admin-mgmt-inline-test-PLAN.md
@CLAUDE.md
@docs/apps-script-deploy.md
@apps-script/package.json
@apps-script/vitest.config.ts

<interfaces>
<!-- Key commands and gates. Extracted from CONTEXT.md §Specifics §6. -->

**Ship-gate criteria (verbatim from CONTEXT.md §Specifics §6):**

| Check | Command | Pass criterion |
|---|---|---|
| Typecheck | `cd apps-script && npx tsc --noEmit` | Exit 0 |
| Bundle build | `cd apps-script && npm run build` | Produces `dist/Code.js`; exit 0 |
| Test suite | `cd apps-script && npm test -- --run` | All tests pass; count is ≥ 340 (336 baseline from STATE.md + 2 new Admin-Mgmt tests + the same 4 WR-fixed tests still passing — no test deletions). If exactly 338, accept and document the +2 delta (Plan 10-02 added exactly 2; Plans 10-01 fixed 4 in-place without adding count). |
| Schema gate (apps-script) | `grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts` | ≥ 1 |
| Schema gate (watcher) | `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` | = 1 |
| User checkpoint | (interactive) | User says "approved" (or equivalent) after reviewing the green CI summary; supplies any clasp environment confirmation needed |
| Clasp push | `cd apps-script && clasp push` (from owner's machine) | Exit 0; success message shows the new `dist/Code.js` timestamp |
| Smoke (manual, dev workbook) | Open each of 5 sidebars in the dev workbook; check browser console | No console errors; expected UI renders for all 5 |

**Note on test count delta:** STATE.md baseline says 336 vitest. Plan 10-02 adds exactly 2 (TM1 + TM2). Plan 10-01 fixes 4 in-place without adding count. Expected post-Phase-10 count is exactly 338. CONTEXT.md §Specifics §6 lists "≥ 340" which assumes some unrelated growth; if the count is 338 with no other commits between baseline and Phase 10 landing, that is the correct number — accept it and document in the SUMMARY.

**Clasp credentials environment (per docs/apps-script-deploy.md):**
- Run from the workbook owner's machine (the user's local box). The user owns the dev workbook's container-bound script project.
- `clasp` v2.4+ (NOT v3.x — Phase 3 RESEARCH §6 documents v3.x has breaking changes; this is locked in `apps-script/package.json`).
- Owner must be logged in: `clasp login` (one-time; persists in `~/.clasprc.json`).
- The `apps-script/.clasp.json` file maps to the dev-workbook scriptId (committed to repo; not a secret — the scriptId alone is useless without the OAuth credential).
- If `clasp` is not installed or login is stale, surface to the user — do NOT attempt to install or re-authenticate from this plan.

**Dev-workbook smoke checklist (manual UAT):**

The user (NOT Claude) opens the dev workbook in a browser and:
1. Opens each of the 5 sidebars from the SquireBot menu:
   - Search
   - Eviction (admin only — must be logged in as an admin)
   - Bank-Coin
   - Char-Info
   - Manage admins (Admin-Mgmt — admin only)
2. For each sidebar, verifies:
   - The sidebar opens without an `HtmlService` runtime error.
   - The expected UI elements render (input fields, buttons, list regions).
   - No errors appear in the browser console (F12 → Console tab).
   - At least one happy-path interaction works (e.g., type a search query and click Search; click Add admin with a test email — the user should NOT actually add a test admin to the real allowlist; cancel before submitting).

Smoke FAIL signal: any console error, any sidebar that fails to render, any HtmlService error toast.
Smoke PASS signal: all 5 sidebars render clean; no console noise; happy-path interactions work as in v1.0.1.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Run all CI gates and gather green-CI evidence</name>
  <files></files>
  <read_first>
    - .planning/phases/10-apps-script-test-quality/10-CONTEXT.md §Specifics §6 (ship-gate criteria table)
    - .planning/STATE.md (for the 336 baseline test count — verify the delta math)
    - apps-script/package.json (confirm `npm run build` and `npm test` scripts are defined as expected; clasp v2.4+ pin)
    - apps-script/vitest.config.ts (confirm JSDOM env from Phase 8 D-01 is still in place — no change in this phase)
  </read_first>
  <behavior>
    - Run all 5 CI gates sequentially. Capture exit codes + key output lines.
    - If ANY gate fails, STOP and surface to the user with the failure output. Do NOT proceed to Task 2's checkpoint with a failing gate. Specifically:
      - If typecheck fails: report the TS error + the file/line; propose either a revision pass on Plan 10-01 or 10-02 (depending on which file the error is in).
      - If build fails: report the esbuild error.
      - If tests fail: report each failing test's name + diff. Distinguish "WR-03 deviation acceptable per CONTEXT.md §3" (searchIndex Test 4) from genuine regressions in other tests.
      - If schema gates fail: that is a CRITICAL FAILURE — schema constants should be untouched by Phase 10. Surface as a scope-creep finding and propose immediate revert.
    - If all gates pass green, proceed to Task 2.
  </behavior>
  <action>
    Run the following commands in sequence from the repo root `/c/Users/Virus Canary/Desktop/Claude/SquireBot/`. Capture exit code + last 20 lines of output for each:

    1. **Typecheck:**
       ```bash
       cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot/apps-script" && npx tsc --noEmit
       ```
       Expected: exit 0; no output.

    2. **Bundle build:**
       ```bash
       cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot/apps-script" && npm run build
       ```
       Expected: exit 0; output includes `dist/Code.js` written; bundle size reported.

    3. **Test suite (full run):**
       ```bash
       cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot/apps-script" && npm test -- --run
       ```
       Expected: exit 0; vitest summary line shows `Test Files  XX passed (XX)` and `Tests  ≥338 passed (≥338)`. If vitest reports 338 exactly (336 baseline + 2 from 10-02), accept — that is the correct count per `<interfaces>` note above. If vitest reports fewer than 338, investigate (did any test get deleted? — DO NOT proceed).

       **Special handling for searchIndex.test.ts Test 4 (WR-03 deviation):**
       - If Test 4 passes: ship-gate proceeds cleanly.
       - If Test 4 fails AND that is the ONLY failure: report the diff; CONTEXT.md §3 + Plan 10-01 task 3 specifically permits this outcome ("if the plan-locked behavior is actually correct, the assertion passes and the test is clean; if it's wrong, the test fails — which is the entire point of a test"). Do NOT auto-fix. Surface to user at Task 2's checkpoint with the failure diff and let the user decide between paths (a)/(b)/(c) in this plan's `<objective>`.
       - If Test 4 fails AND other tests also fail: that is a genuine regression; STOP and propose revision passes on the offending plans.

    4. **Schema gate — apps-script:**
       ```bash
       cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot" && grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts
       ```
       Expected: ≥ 1.

    5. **Schema gate — watcher:**
       ```bash
       cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot" && grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go
       ```
       Expected: exactly 1.

    Build a structured evidence summary for Task 2's checkpoint:

    ```
    PHASE 10 SHIP-GATE — GREEN CI EVIDENCE

    | Gate | Command | Result | Output snippet |
    |---|---|---|---|
    | Typecheck | npx tsc --noEmit | PASS / FAIL | <last line or error> |
    | Build | npm run build | PASS / FAIL | <dist/Code.js timestamp + size> |
    | Test suite | npm test -- --run | PASS (NNN/NNN) / FAIL | <vitest summary> |
    | Schema (apps-script) | grep migrations.ts | <count> | <line> |
    | Schema (watcher) | grep client.go | <count> | <line> |

    WR-03 Test 4 outcome: PASS / FAIL <diff if failure>
    Total test count: NNN (baseline 336 + Plan 10-02 +2 = expected 338)
    Production-code edits: 1 (the `export` keyword on showAdminMgmtSidebar.ts:135 per Plan 10-02)
    ```
  </action>
  <verify>
    <automated>cd apps-script && npx tsc --noEmit && npm run build && npm test -- --run && cd .. && [ "$(grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go)" = "1" ] && [ "$(grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts)" -ge "1" ]</automated>
  </verify>
  <acceptance_criteria>
    - All 5 gates report PASS in the evidence table OR the executor has stopped and surfaced the failure to the user.
    - If proceeding: evidence summary is composed in the exact format above and ready to present at Task 2.
    - Test count is ≥ 338 (or 338 exactly with no other commits between baseline and Phase 10 landing).
    - Schema gates BOTH non-zero / exactly 1 as required.
  </acceptance_criteria>
  <done>All 5 CI gates green (or executor halted on first failure with structured failure report ready for user); evidence summary composed and ready for Task 2 checkpoint.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: USER CHECKPOINT — present green-CI evidence and request approval for clasp push</name>
  <what-built>
    Plan 10-01 (TEST-04 fixes): 4 surgical edits across 4 test files closing WR-01..WR-04.
    Plan 10-02 (TEST-03 Admin-Mgmt inline test): new `showAdminMgmtSidebar.inline.test.ts` with 2 tests (TM1 + TM2) + single `export` keyword affordance on `showAdminMgmtSidebar.ts:135`.
    All CI gates from Task 1 are GREEN (or Task 1 halted and this checkpoint is now a deviation discussion).
  </what-built>
  <how-to-verify>
    The executor presents the Task 1 evidence summary to the user and waits for explicit approval. The user reviews:

    1. **Evidence table:** confirm typecheck/build/tests/schema all PASS. Pay special attention to:
       - Test count delta (336 → expected 338).
       - WR-03 Test 4 outcome (PASS expected; FAIL acceptable per CONTEXT.md §3 deviation handling).
       - Production-code edits limited to the single `export` keyword.

    2. **Decision points for the user:**
       - **If all green and no deviations:** approve clasp push → executor proceeds to Task 3.
       - **If WR-03 Test 4 failed (and was the ONLY failure):** decide between:
         - (a) Accept ship gate as-is; document the failure as v1.1 backlog 999.30; clasp push proceeds with the failing test.
         - (b) Spawn a new Plan 10-04 to revise either the `didYouMean` impl or the plan-locked assertion; clasp push deferred.
         - (c) Defer Phase 10 ship entirely until a separate fix lands.
       - **If any other test failed:** defer Task 3; spawn a revision pass on the offending plan.
       - **If clasp credentials are stale or clasp not installed:** the user installs/re-authenticates clasp before approving Task 3 (per memory `feedback_toolchain_installs.md` — user installs missing toolchains themselves).

    3. **Required user signal to proceed:** the user must say "approved" (or equivalent — "go", "ship it", "proceed with clasp push") in plain language. The executor MUST NOT proceed to Task 3 without this explicit signal. Silence is NOT approval.

    The exact clasp push command the user is approving:

    ```bash
    cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot/apps-script" && clasp push
    ```

    This will overwrite the current dev-workbook bundle with the new `dist/Code.js` produced by Task 1's `npm run build`. The dev workbook is the workbook owner's container-bound script project (per `apps-script/.clasp.json` scriptId).
  </how-to-verify>
  <resume-signal>User types "approved" (or equivalent: "go", "ship it", "proceed with clasp push") in plain language. Silence or ambiguous response → DO NOT proceed; ask the user to clarify.</resume-signal>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Run clasp push from the workbook owner's machine</name>
  <files></files>
  <read_first>
    - docs/apps-script-deploy.md (the clasp push runbook — confirms working directory, credential location, and rollback procedure if push fails)
    - apps-script/.clasp.json (read-only — confirms scriptId)
    - .planning/PROJECT.md §Technology Stack (clasp v2.4+ NOT v3.x constraint)
  </read_first>
  <behavior>
    - Task 2's user-approval signal has been received. Now run the clasp push.
    - `clasp push` reads the current `dist/Code.js` and uploads it to the Apps Script project bound to the scriptId in `.clasp.json`.
    - On success, clasp prints something like `Pushed N files` and lists the files that were updated.
    - On failure (network error, expired credentials, scriptId mismatch), STOP and surface the failure to the user. Common failure modes:
      - `Error 401 Unauthorized`: clasp credentials expired — user runs `clasp login` and retries.
      - `Could not read .clasp.json`: working directory is wrong — must be run from `apps-script/`, not the repo root.
      - `Cannot find module`: clasp not installed — user runs `npm install -g @google/clasp@2.4` (or per the version pinned in `apps-script/package.json`).
    - DO NOT retry the push automatically. Each retry is a user-credentialed action.
  </behavior>
  <action>
    From the working directory `/c/Users/Virus Canary/Desktop/Claude/SquireBot/apps-script/`, run:

    ```bash
    cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot/apps-script" && clasp push
    ```

    Capture the full stdout + stderr. Expected success output (paraphrased):

    ```
    └─ src/Code.js
    Pushed 1 files.
    ```

    If success: proceed to Task 4 (the manual smoke). Capture the timestamp and file count for the SUMMARY.

    If failure: surface the error to the user. Do NOT auto-retry; do NOT proceed to Task 4 with a failed push.

    DO NOT modify any source file in this task. DO NOT commit anything. DO NOT push a git tag. The only side-effect of this task is the dev-workbook bundle update via clasp; the local working tree is unchanged.
  </action>
  <verify>
    <automated>cd apps-script && clasp push 2>&1 | tee /tmp/clasp-push.log && grep -E 'Pushed [0-9]+ file' /tmp/clasp-push.log</automated>
  </verify>
  <acceptance_criteria>
    - `clasp push` exit code is 0.
    - Output contains `Pushed N files` (where N ≥ 1).
    - No `Error` / `401` / `403` / `Could not` strings in the output.
    - The dev workbook's Apps Script editor (Extensions → Apps Script) shows an updated `Code.gs` timestamp matching this push (manual verification possible at Task 4).
  </acceptance_criteria>
  <done>`clasp push` succeeded; dev-workbook bundle is updated; the user is now ready to run the manual smoke in Task 4.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 4: Manual dev-workbook smoke check — all 5 sidebars render clean</name>
  <what-built>The new `dist/Code.js` is now live in the dev workbook (Task 3 confirmed `clasp push` success). All 5 shipping sidebars (Search, Eviction, Bank-Coin, Char-Info, Admin-Mgmt) should open without errors. This task is the UAT-equivalent that vitest+JSDOM cannot replicate — it catches HtmlService runtime quirks, Apps Script V8 differences, and CSS-rendering issues that only manifest in the real product.</what-built>
  <how-to-verify>
    The user (NOT Claude) performs the following in a browser on the dev workbook:

    1. **Refresh the dev workbook** (Ctrl+R / Cmd+R) — forces the new bundle to load.
    2. **Open browser dev tools** (F12 → Console tab) to watch for runtime errors during sidebar opens.
    3. **Open each of the 5 sidebars** from the SquireBot menu in this order:

       | Sidebar | How to open | Expected UI | Pass criterion |
       |---|---|---|---|
       | Search | SquireBot → Search items… | 300px sidebar; query input + Search button; char/slot dropdowns | Renders clean; no console error; type 'sword' + click Search → results region populates |
       | Eviction | SquireBot → Admin → Manage evictions… (admin only) | 320px sidebar; email list with checkboxes; Mark for eviction button | Renders clean; no console error; checkboxes toggle |
       | Bank-Coin | SquireBot → Set bank coin… | 280px sidebar; pp/gp/sp/cp inputs; Save button | Renders clean; no console error; type a value + tab → input retains value |
       | Char-Info | SquireBot → Char info… | 300px sidebar; char dropdown + info pane | Renders clean; no console error; select a char → info pane updates |
       | Admin-Mgmt | SquireBot → Admin → Manage admins… (admin only) | 300px sidebar; current-admins list; email input + Add admin button | Renders clean; no console error; type a fake test email like `smoke@example.com` + click Add admin → success message appears in #msg (NOTE: this WILL add the email to the real admin list — either use a clearly-marked test email and remove it after, OR cancel before submitting) |

    4. **Console-error tolerance:** zero browser-console errors during sidebar open. Warnings (yellow) are acceptable IF they pre-existed before Phase 10 (e.g., a deprecated CSS property). Errors (red) are NOT acceptable — any error means the smoke fails.

    5. **Smoke verdict:**
       - **All 5 sidebars render clean + no console errors:** smoke PASSES → user signals "smoke passed" → executor proceeds to wrap-up.
       - **ANY sidebar fails to render OR ANY console error appears:** smoke FAILS → user provides the failure details (which sidebar; error message; reproduction steps); executor opens a Phase 10 deviation note + proposes either (a) revert clasp push to the prior bundle, (b) hotfix plan to address the regression, (c) tolerate-and-defer.

    **CRITICAL note on Admin-Mgmt smoke (per CONTEXT.md D-01 + admin-allowlist sensitivity):** if the user actually clicks "Add admin" with a real or test email, that email IS added to the dev-workbook's `_meta` admin allowlist. To avoid polluting the dev workbook's admin list, prefer ONE of:
    - Type a test email like `smoke-test-DELETE-ME@example.com`, click Add, then immediately use the Remove button to clean up.
    - Cancel the action before clicking Add admin (the UI render is enough to validate this sidebar's basic functionality).
    The user chooses the smoke depth.
  </how-to-verify>
  <resume-signal>User types "smoke passed" (or equivalent: "all sidebars clean", "passed") to confirm successful smoke; OR types failure details to trigger deviation handling.</resume-signal>
</task>

<task type="auto" tdd="false">
  <name>Task 5: Wrap-up — finalize the Phase 10 SUMMARY and mark phase complete in ROADMAP</name>
  <files>
    - .planning/phases/10-apps-script-test-quality/10-03-SUMMARY.md (CREATE)
    - .planning/ROADMAP.md (UPDATE — flip Phase 10 checkbox from [ ] to [x])
    - .planning/STATE.md (UPDATE — record Phase 10 completion + new test count)
  </files>
  <read_first>
    - .planning/ROADMAP.md (the Phase 10 entry at line ~44; the milestone-progress table at line ~75-85)
    - .planning/STATE.md (current state shape — recent-history section to append Phase 10 entry)
    - .planning/REQUIREMENTS.md (the TEST-03 + TEST-04 traceability rows at lines ~59-60)
    - .planning/phases/10-apps-script-test-quality/10-01-SUMMARY.md (Plan 10-01 outcome from prior wave)
    - .planning/phases/10-apps-script-test-quality/10-02-SUMMARY.md (Plan 10-02 outcome from prior wave)
  </read_first>
  <behavior>
    - Create `10-03-SUMMARY.md` summarizing the ship-gate run end-to-end: green CI evidence (or accepted deviations), user-approval timestamp, clasp push outcome, manual smoke verdict, total test count post-phase, schema lock verification.
    - Update `ROADMAP.md` Phase 10 entry from `- [ ] **Phase 10: Apps Script Test Quality**` to `- [x] **Phase 10: Apps Script Test Quality** — closed v1.0.1-Phase-8-review-surfaced test-quality items (TEST-03 Admin-Mgmt inline-JS coverage + TEST-04 4 warning-level fixes). Shipped via clasp push to dev workbook 2026-MM-DD. 5/5 inline-JS sidebars green; 338 vitest tests passing.`
    - Update `ROADMAP.md` Progress table — Phase 10 row Status: `✅ Shipped`, Completed: `2026-MM-DD` (today's date).
    - Update `ROADMAP.md` milestone-progress table — v1.0.2 Status: confirm if both Phase 9 + Phase 10 are now shipped (transitions to `✅ Shipped`); if Phase 9 has remaining UAT blocker (999.19 Google brand verification) keep milestone as `🚧 In progress`.
    - Update `STATE.md` with: phase completion entry; new test count baseline (338); reference to 10-03-SUMMARY.md.
    - DO NOT update REQUIREMENTS.md traceability rows here — that is the milestone-close concern, not a per-phase concern (the rows already reference "Phase 10" by phase number).
    - DO NOT tag a git release. DO NOT push a GitHub Release. Phase 10 ships via clasp push only.
  </behavior>
  <action>
    1. Compose `10-03-SUMMARY.md` with the following sections:
       - **Overview:** date, ship-gate outcome (passed / passed-with-deviation), CI evidence table.
       - **Plans landed:** 10-01 (TEST-04 fixes) + 10-02 (TEST-03 Admin-Mgmt inline test) + 10-03 (this ship gate).
       - **Verification evidence:**
         - All 5 must_haves truths satisfied (typecheck, build, tests, schema, smoke).
         - Test count delta: 336 → 338 (or actual if different).
         - Production-code edits: exactly 1 (`export` keyword on `showAdminMgmtSidebar.ts:135`).
         - `git diff --stat apps-script/src/triggers/`: 1 file, 1 insertion, 1 deletion.
         - `git diff --stat apps-script/src/lib/`: no files.
         - Schema gates: PASS PASS.
       - **Deviations (if any):** WR-03 outcome (Test 4 PASS/FAIL); admin-mgmt smoke approach (test email vs. cancel-before-submit); any other surprises.
       - **clasp push result:** timestamp + Pushed N files line from Task 3 output.
       - **Manual smoke verdict:** verbatim user signal from Task 4 + which sidebars were exercised + any console-warning notes.
       - **REQ closure:** TEST-03 closed, TEST-04 closed. (Note in SUMMARY — but REQUIREMENTS.md row update is a milestone-close concern, not this plan's.)
       - **Backlog updates:**
         - 999.17 retired (TEST-03 closed).
         - 999.18 retired (TEST-04 closed).
         - 999.24..999.29 (IN-01..IN-06) confirmed deferred to v1.1 per CONTEXT.md D-01.
         - If WR-03 Test 4 failed → propose new 999.30 (didYouMean vs whole-string Levenshtein contract).
       - **Hand-off:** "v1.0.2 Phase 10 shipped via clasp push to dev workbook. If Phase 9 v1.0.2 watcher binary is also shipped, v1.0.2 milestone is now complete (subject to 999.19 Google brand verification re-approval for end-to-end UAT)."

    2. Update `ROADMAP.md` Phase 10 entry (line ~44):
       - Change `- [ ] **Phase 10: Apps Script Test Quality** — close v1.0.1-Phase-8-review-surfaced test-quality items...` to `- [x] **Phase 10: Apps Script Test Quality** — closed v1.0.1-Phase-8-review-surfaced test-quality items (TEST-03 Admin-Mgmt inline-JS coverage + TEST-04 4 warning-level fixes addressed). Shipped via clasp push to dev workbook YYYY-MM-DD; 5/5 inline-JS sidebars green; 338 vitest tests passing.`

    3. Update `ROADMAP.md` Progress table (line ~82-84):
       - Phase 10 row Status: `✅ Shipped`, Completed: today's date.
       - If Phase 9 is also `✅ Shipped`, update v1.0.2 milestone Status to `✅ Shipped` and add completion date. (Per current ROADMAP, Phase 9 is shipped with HUMAN-UAT blocker on 999.19 — keep milestone as `🚧 In progress` and add a note that ship is code-complete but HUMAN-UAT blocked on 999.19.)

    4. Update `STATE.md`:
       - Add a recent-history entry: `Phase 10 (Apps Script Test Quality) shipped via clasp push YYYY-MM-DD. TEST-03 + TEST-04 closed. Vitest count: 338 (+2 from 336 baseline). See .planning/phases/10-apps-script-test-quality/10-03-SUMMARY.md.`
       - Update the "current phase" position field to reflect Phase 10 complete.

    5. Commit the SUMMARY + ROADMAP + STATE updates in a single commit:
       ```bash
       cd "/c/Users/Virus Canary/Desktop/Claude/SquireBot" && git add -f .planning/phases/10-apps-script-test-quality/10-03-SUMMARY.md .planning/ROADMAP.md .planning/STATE.md
       git commit -m "docs(10): close Phase 10 — TEST-03 + TEST-04 shipped via clasp push

       - Plan 10-01 (TEST-04): WR-01 mountSidebar IIFE wrap + WR-02/03/04 assertion fixes
       - Plan 10-02 (TEST-03): new showAdminMgmtSidebar.inline.test.ts (TM1 + TM2)
       - Plan 10-03 (ship gate): green CI + clasp push + dev-workbook smoke PASS
       - Test count: 336 → 338 (+2)
       - 5/5 inline-JS sidebars now have coverage
       - Schema lock unchanged (schema_version=3 + WatcherMaxSchemaVersion=3)"
       ```
  </action>
  <verify>
    <automated>ls .planning/phases/10-apps-script-test-quality/10-03-SUMMARY.md && grep -E '^- \[x\] \*\*Phase 10' .planning/ROADMAP.md && git log -1 --pretty=%s | grep -E 'docs\(10\): close Phase 10'</automated>
  </verify>
  <acceptance_criteria>
    - `.planning/phases/10-apps-script-test-quality/10-03-SUMMARY.md` exists with all sections per the action.
    - ROADMAP.md Phase 10 entry has `[x]` checkbox + closed-date.
    - ROADMAP.md Progress table Phase 10 row shows `✅ Shipped`.
    - STATE.md has a recent-history entry for Phase 10 completion.
    - Git commit with subject `docs(10): close Phase 10 — TEST-03 + TEST-04 shipped via clasp push` exists.
    - Schema gates STILL hold post-commit (Phase 10 changes nothing schema-related).
  </acceptance_criteria>
  <done>Phase 10 SUMMARY exists; ROADMAP + STATE updated to reflect completion; single commit lands the doc updates; v1.0.2 milestone status reflects Phase 10 closure.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| local build → clasp push | The `apps-script/dist/Code.js` produced by `npm run build` is uploaded by clasp to the dev-workbook's container-bound Apps Script project. The credential boundary is the user's `~/.clasprc.json` OAuth token — user-controlled and not transmitted by this plan. |
| dev workbook → guildies | The dev workbook is a developer-owned workbook, NOT the production guildie workbook (per `docs/apps-script-deploy.md` — production workbook deploys are a separate, manual `clasp push` step against a different scriptId). This ship gate touches the dev workbook only; production-workbook deploy is a downstream concern not covered by Phase 10. |
| user-approval signal → clasp push | Task 2's `checkpoint:human-verify` is the trust boundary — no clasp push runs without an explicit user signal. Silence is NOT approval. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-10-03-01 | Tampering | dist/Code.js bundle | accept | The bundle is produced by `esbuild` from `src/Code.ts` + transitively imported source. Phase 10's source changes are limited to a single `export` keyword (production) + test files (NOT in the bundle — esbuild's entry is `src/Code.ts`, which does not import test files). Risk: low. |
| T-10-03-02 | Elevation of Privilege | clasp push to wrong workbook | mitigate | `apps-script/.clasp.json` pins the dev-workbook scriptId. The user is responsible for running `clasp push` against the correct project (confirmed at Task 2 checkpoint). Production-workbook push is a separate, deliberate action covered by `docs/apps-script-deploy.md`. |
| T-10-03-03 | Repudiation | unaudited deploy | mitigate | Task 5 commits the SUMMARY + ROADMAP + STATE updates with a clear commit subject. clasp's own push log (`Pushed N files` output) is captured in the 10-03-SUMMARY.md. Both the bundle change and the deploy are auditable post-hoc. |
| T-10-03-04 | Denial of Service | broken bundle deployed to dev workbook | mitigate | CI-green-BEFORE-clasp-push ordering (CONTEXT.md D-04 + Task 1 → Task 2 → Task 3 sequence) prevents a broken bundle from reaching the dev workbook. Manual smoke (Task 4) catches HtmlService runtime issues that vitest+JSDOM cannot. Worst case: a 5-minute window where the dev workbook serves a broken sidebar; recovery is `clasp push` of the prior bundle (rollback runbook in `docs/apps-script-deploy.md`). |
| T-10-03-05 | Information Disclosure | admin-mgmt smoke pollutes real allowlist | accept | Task 4 explicitly flags this risk; the user chooses smoke depth (test email + cleanup vs. cancel-before-submit). No PII; the dev workbook is developer-owned, not a guildie-facing workbook. |

**Schema impact:** NONE. This plan is verification + deploy + docs. No source files are modified by Tasks 1-4. Task 5 modifies only `.planning/` documentation. The verifier grep gates BOTH still pass post-plan: `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1; `grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts` returns ≥ 1.

**Scope-creep tripwire (per CONTEXT.md D-01 + D-05):** if at any point during this plan the executor finds itself editing a source file (anything outside `.planning/`), STOP and surface as a scope creep. This plan is verification-only on source.
</threat_model>

<verification>
1. Task 1 evidence summary shows all 5 CI gates PASS (typecheck, build, tests, schema gate apps-script, schema gate watcher).
2. Test count is ≥ 338 (336 baseline + 2 from Plan 10-02; Plan 10-01 added 0).
3. Task 2 user-approval signal received and captured in the SUMMARY.
4. Task 3 `clasp push` exit code 0; output contains `Pushed N files`.
5. Task 4 manual smoke verdict captured: all 5 sidebars render clean OR documented failure handling.
6. Task 5 SUMMARY exists; ROADMAP Phase 10 checkbox flipped to `[x]`; STATE.md updated; single commit landed with subject `docs(10): close Phase 10 — TEST-03 + TEST-04 shipped via clasp push`.
7. `grep -cE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns 1 (schema lock preserved).
8. `grep -cE "schema_version.*=\s*['\"]?3['\"]?" apps-script/src/lib/migrations.ts` returns ≥ 1 (schema lock preserved).
9. `git diff --stat HEAD~1..HEAD apps-script/` shows ZERO files (Phase 10 closes with no apps-script source changes since Plan 10-02; the Task 5 commit touches only `.planning/`).
</verification>

<success_criteria>
- All 4 WR-* warnings from 08-REVIEW.md are closed in code AND shipped to dev workbook via clasp push.
- 5/5 shipping sidebars have inline-JS test coverage (Admin-Mgmt added; the v1.0.1 TEST-02 deferral note is retired).
- The dev workbook serves the updated `dist/Code.js` bundle without console errors across all 5 sidebars (smoke PASS).
- ROADMAP.md Phase 10 entry is marked `[x]` with completion date.
- A single commit lands the SUMMARY + ROADMAP + STATE updates with a clear subject.
- v1.0.2 milestone status reflects Phase 10 closure (transition to `✅ Shipped` if Phase 9 is also shipped; otherwise remain `🚧 In progress` with a note).
- Schema constants (`_meta.schema_version` + `WatcherMaxSchemaVersion`) unchanged throughout — verifier grep gates pass.
- Zero production-code edits in this plan (the single `export` keyword landed in Plan 10-02; this plan touches only `.planning/` in Task 5).
- User remains the sole authority on clasp push timing — autonomous: false enforced at Task 2's checkpoint.
</success_criteria>

<output>
After Task 5 completes, `10-03-SUMMARY.md` is the Phase 10 ship-gate canonical record. It feeds:
- v1.0.2 milestone-close summary (when Phase 9 + Phase 10 are both shipped).
- REQUIREMENTS.md TEST-03 + TEST-04 traceability close-out at milestone close (not this plan).
- Any future investigation into "what shipped in Phase 10" — the SUMMARY is the single answer.

Hand-off after this plan:
- If Phase 9 is also shipped: v1.0.2 milestone is code-complete; the only outstanding gate is 999.19 (Google OAuth brand verification re-approval, ETA 2026-05-16/05-18). Open a milestone-close ceremony when 999.19 resolves.
- If WR-03 Test 4 deviation was accepted at Task 2: track 999.30 as v1.1 backlog item and address at v1.1 planning.
- No further apps-script work is required in v1.0.2.
</output>
</content>
</invoke>