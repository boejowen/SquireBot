# Phase 10: Apps Script Test Quality — Context

**Gathered:** 2026-05-13
**Status:** Ready for planning
**Mode:** `/gsd-discuss-phase 10` (user delegated all 4 gray areas with the locked criterion: "err on the side of making the end-user experience as simple and seamless as possible" — see Meta-Decision)

<domain>
## Phase Boundary

Close v1.0.1-Phase-8-review-surfaced test-quality items so the apps-script vitest suite has clean coverage for all 5 shipping sidebars and the advisory findings from Phase 8 code review are retired:

1. **TEST-03** — Author `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` covering Admin-Mgmt sidebar inline-JS (DOM event handlers + `google.script.run` callback wiring + error-display path), mirroring the 4 inline-JS test files landed in v1.0.1 Plan 08-02.
2. **TEST-04** — Address the 4 warning-level findings in `.planning/phases/08-test-infra-persistence-docs/08-REVIEW.md`:
   - WR-01 — `mountSidebar` JSDOM realm leak (`(0, eval)(src)` pollutes test-realm globalThis with `var` / `function` declarations that persist across tests)
   - WR-02 — `evictionSidebar.inline.test.ts` TE1 overly permissive regex `/Marked 2|removed/i`
   - WR-03 — `searchIndex.test.ts` Test 4 wraps the plan-locked assertion in `try/catch` and asserts nothing if it fails
   - WR-04 — `searchSidebar.inline.test.ts` TS1 leaks an unresolved `pushRecentSearchCall` in the pending mock queue
3. **Ship gate** — `clasp push` to the dev workbook + green CI on a tag-driven workflow run.

**Hard non-goals (push back if surfaced):**

- No new sidebar features; this is a tests-only phase.
- No `SIDEBAR_BODY` → external `.html` refactor (backlog 999.7 still deferred — cosmetic, v1.1 candidate).
- No schema changes. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3 (verification hook 5 grep gate, same as Phases 7–9).
- No production-code changes — IN-01 (`COL_RACE`/`COL_COUNT` collision in `showCharInfoSidebar.ts`) and IN-02 (orphaned CacheService key post-SEARCH-05 migration) are real findings but outside "test quality" scope; deferred to v1.1 backlog per D-01.
- No binary release; this is apps-script-only and ships via `clasp push`, not via tag → GitHub Release.
- No watcher-side work. Phase 9 (Watcher Robustness Polish) is complete and shipped as v1.0.2; Phase 10 is fully decoupled from the Phase 9 OAuth brand-verification blocker (999.19) — apps-script auth is Apps-Script-server-side (`Session.getActiveUser()`), not the watcher's Desktop OAuth client.

</domain>

<decisions>
## Implementation Decisions (auto-locked under "simple, seamless" criterion)

### D-01 — Info-level findings (IN-01..IN-06) are OUT OF SCOPE for Phase 10; deferred to v1.1 backlog

REQUIREMENTS.md TEST-04 explicitly defers the "whether to also fold in the 6 info-level items" decision to this discussion. Locking: NONE of the 6 info-level items land in Phase 10. Each becomes its own v1.1 backlog candidate:

- **999.24** IN-01 — `COL_RACE = 14` / `COL_COUNT = 14` collision in `showCharInfoSidebar.ts:26-27` (production foot-gun for extend-only schema evolution; small fix; 1 LOC + 1 test)
- **999.25** IN-02 — Orphaned `squirebot:search:recent` CacheService key never cleaned up post-SEARCH-05 migration (harmless 25-min TTL but symbolic; one `CacheService.getDocumentCache().remove(KEY_RECENT)` call)
- **999.26** IN-03 — `evictionSidebar.inline.test.ts` bypasses admin gate (defense-in-depth observation; gate is tested elsewhere; possible refactor or docs note)
- **999.27** IN-04 — `showSearchSidebar.test.ts` Test 3 negative assertion is incomplete; should assert "no themed `:root` block emitted" instead of excluding specific hex colors
- **999.28** IN-05 — `didYouMean('')` returns short-name candidates instead of empty list (unreachable today; contract bug)
- **999.29** IN-06 — `test-helpers.ts` CacheService mock TTL boundary is strict-greater-than vs production's undefined behavior (tiny mock-fidelity nit)

**Why defer:** the phase title is "Apps Script Test Quality" and the goal is closing Phase-8-review test-quality items. Two of the six info findings (IN-01, IN-02) touch production code, not tests — folding them in would expand Phase 10 beyond its name and would require their own decision threads (schema-aware constant naming, post-migration cleanup semantics). The other four are nits with no current-state user impact. Adding 6 advisory items into a tightly-scoped patch phase trades clarity for breadth. Backlog preservation keeps the findings discoverable without expanding scope.

**Constraint downstream agents must honor:** if any plan in this phase ends up touching `apps-script/src/triggers/` or `apps-script/src/lib/` (production code), STOP — that's a signal scope has crept. Plans here touch `apps-script/src/__tests__/` only.

### D-02 — `mountSidebar` realm-leak fix is IIFE wrap (Option a)

`apps-script/src/__tests__/test-helpers.ts:620-625` currently runs `(0, eval)(src)` on each inline `<script>` body. Wrap the eval source in an IIFE before executing so `var` / `function` declarations stay local to the IIFE rather than escaping to the test realm's globalThis:

```typescript
// before:  (0, eval)(src)
// after:   (0, eval)(`(function(){\n${src}\n})()`)
```

Reasoning:

- **Simple:** 3-LOC change in test-helpers.ts. No new dependencies, no new test patterns.
- **Bulletproof:** `var` and function declarations inside an IIFE are scoped to the IIFE, never reaching globalThis. The leak vector is closed at the source rather than reactively cleaned up between tests.
- **Faster than per-test JSDOM realm (Option b):** option b would require constructing a fresh JSDOM Window per test, adding ~10ms × 50 tests = ~500ms to the suite per run. The IIFE wrap is zero-cost.
- **More reliable than globalThis-tracking (Option c):** option c relies on tracking which keys each test added and resetting them — fragile against indirect side-effects (e.g., handlers attaching to `document` itself), and bug-prone to maintain.

**Constraint downstream agents must honor:** the IIFE wrap MUST preserve `this === window` semantics for inline scripts that reference `this` at top level. An arrow IIFE `(() => { ... })()` would break this (arrow `this` ≡ caller's `this`); use the function-expression IIFE form `(function(){ ... })()`.

### D-03 — Admin-Mgmt inline-JS test depth = 2 tests (1 happy + 1 error path), mirroring the 4 existing sidebars

Match the Phase 8 D-03 pattern locked for the other 4 inline-JS tests:

- **Happy path** — primary admin action (e.g., add allowlist email) → user fills the input → submit handler fires → `google.script.run.withSuccessHandler` callback → success message renders in the status region. Exactly one test.
- **Error path** — primary failure (e.g., backend rejects with "Email already in allowlist") → `withFailureHandler` callback → error renders in the visible status region. Exactly one test.

Reasoning:

- **Simple:** symmetry with the 4 existing inline-JS test files (`bankCoinSidebar.inline.test.ts`, `charInfoSidebar.inline.test.ts`, `evictionSidebar.inline.test.ts`, `searchSidebar.inline.test.ts`) makes the suite predictable to read and maintain. A future maintainer learns one pattern, applies it five places.
- **Seamless:** the admin gate (`requireAdminOrThrow` + `SpreadsheetApp.getUi().alert(...)`) is already covered at the trigger layer by the existing `adminMgmtSidebar.test.ts` + `admin.test.ts`. The inline-JS layer's job is DOM event handlers + `google.script.run` wiring — same surface as the other 4. Adding 3rd/4th cases would test admin-mgmt logic that's already tested server-side, doubling the surface area for review with no new coverage.

**Constraint downstream agents must honor:** if during execution an obvious gap presents itself (e.g., the Admin-Mgmt sidebar has a confirm dialog the other 4 don't), flag it and surface as a Phase 10 deviation note rather than silently expanding the test count. The 2-test contract is locked.

### D-04 — Plan structure = 3 plans across 3 waves (sequential); ship gate = green CI **before** `clasp push`

**Plan structure (sequential due to test-helpers.ts dependency):**

- **Plan 10-01 (wave 1, autonomous): TEST-04 fixes.** Touches `apps-script/src/__tests__/test-helpers.ts` (IIFE wrap for WR-01), `evictionSidebar.inline.test.ts` (tighten WR-02 regex), `searchIndex.test.ts` (remove WR-03 try/catch, fail loudly on assertion miss), `searchSidebar.inline.test.ts` (WR-04 resolve the leaked `pushRecentSearchCall` mock). 4 file edits, ~20-30 LOC net change. No new files.
- **Plan 10-02 (wave 2, autonomous, depends_on 10-01): TEST-03 Admin-Mgmt inline test.** Creates `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` (new file, ~80-100 LOC mirroring the existing 4 inline test files). Uses the IIFE-fixed `mountSidebar` from 10-01. Reads `showAdminMgmtSidebar.ts` trigger and existing `adminMgmtSidebar.test.ts` for happy-path fixture shape.
- **Plan 10-03 (wave 3, autonomous=false — ship-gate checkpoint, depends_on 10-01, 10-02): Ship gate.** Run `npm run build` (esbuild → dist/Code.js); `npm test -- --run` (≥ 340 tests green); on both green, `clasp push` to the dev workbook from the workbook owner's machine; perform a smoke check that the dev workbook still loads each sidebar without console errors. User checkpoint required for clasp push (it's a per-developer credentialed action; not automatable from CI).

**Why sequential, not parallel:** Plan 10-02's new Admin-Mgmt test depends on the IIFE-fixed `mountSidebar` from 10-01. If 10-02 ran in parallel with 10-01, the new test would either be written against the leaky mountSidebar (and then need rewriting after the merge) or land first and silently fail intermittently due to leaked state from other tests. Sequential is cheaper than coordination.

**Ship gate ordering — CI green BEFORE `clasp push` (defensive):**

- `npm run build` failing → typecheck or esbuild error → would push a broken bundle to dev workbook → guildies who hit the workbook during the ~minute the bundle is broken see errors. Cheap to prevent.
- `npm test` failing → at least one regression → pushing would mean the dev workbook is running un-tested code. Higher-confidence push to gate on green test suite.
- The dev-workbook smoke is the human equivalent of a UAT — it catches things vitest+JSDOM cannot (e.g., HtmlService quirks, Apps Script runtime differences).

**Constraint downstream agents must honor:** Plan 10-03 is `autonomous: false`. Do NOT auto-`clasp push`. Surface the green-CI evidence to the user, present the exact `clasp push` command, wait for "approved" before any deploy.

### D-05 — Schema lock (verification hook 5)

`_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. `SCRIPT_MIN_SCHEMA_VERSION` (or its equivalent in `apps-script/src/lib/migrations.ts`) stays at 3. Phase 10 is tests-only and explicitly NOT a schema bump.

Verification: `grep -c "schema_version = 3\|WatcherMaxSchemaVersion = 3\|writeMetaRow.*'_meta', 'schema_version', '3'" internal/sheet/client.go apps-script/src/lib/migrations.ts` must return non-zero on each path. Mirrors the Phase 9 schema gate exactly.

### D-06 — Mode applied: invisible-UX tiebreaker for this internal phase

The user delegated all four gray areas with: "err on the side of making the end-user experience as simple and seamless as possible." This is a TEST-only phase with zero guildie-visible behavior, so the criterion translates to two concrete sub-rules applied throughout the decisions above:

1. **Scope simplicity** — defer info-level findings (D-01); keep the Admin-Mgmt test count at the minimum that mirrors the existing pattern (D-03); 3 plans is the minimum that respects the test-helpers.ts dependency (D-04). Don't expand surface area.
2. **Maintenance simplicity** — IIFE wrap (D-02) is canonical JS, three lines, no new abstractions. Symmetry across sidebars (D-03) means future maintainers learn one pattern, not five.

The dev-workbook smoke step in Plan 10-03 is the only end-user-facing UAT in this phase, and the green-CI-before-push ordering directly serves the "seamless" criterion (no broken bundle ever reaches the workbook).

</decisions>

<specifics>
## Particular References & Constraints

### Canonical references (MUST read before planning)

- **`.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md`** — Phase 8 D-01 (JSDOM in vitest.config.ts), D-02 (no SIDEBAR_BODY extraction), D-03 (test scope per sidebar = happy + error), D-04 (`mountSidebar` helper contract)
- **`.planning/phases/08-test-infra-persistence-docs/08-REVIEW.md`** — Source of TEST-04 4 warnings (WR-01..WR-04) and the 6 info-level items deferred to v1.1 (IN-01..IN-06)
- **`apps-script/src/__tests__/test-helpers.ts`** — `mountSidebar` helper at lines 620-625 (eval point), CacheService mock at 419/436 (IN-06 reference), `installSessionMock` and `resetMocks` exports used by all inline tests
- **`apps-script/src/__tests__/bankCoinSidebar.inline.test.ts`** — Reference template for new Admin-Mgmt inline test structure (2-test pattern; one mount per `it`; `mock.dispatchRunCall` for `google.script.run` resolution)
- **`apps-script/src/__tests__/charInfoSidebar.inline.test.ts`** — Same; second reference
- **`apps-script/src/__tests__/evictionSidebar.inline.test.ts`** — Same; third reference + contains WR-02 (line 87 regex) and the IN-03 admin-gate bypass note for v1.1
- **`apps-script/src/__tests__/searchSidebar.inline.test.ts`** — Same; fourth reference + contains WR-04 (TS1 leaked pending mock call, lines 53-58)
- **`apps-script/src/__tests__/searchIndex.test.ts`** — Contains WR-03 (Test 4 try/catch wrapping, lines 94-114)
- **`apps-script/src/triggers/showAdminMgmtSidebar.ts`** — The trigger that Admin-Mgmt inline test will exercise. Read its SIDEBAR_BODY template literal and `google.script.run` callback wiring to derive fixture shape.
- **`apps-script/src/__tests__/adminMgmtSidebar.test.ts`** — Existing trigger-level test for Admin-Mgmt; pattern reference for fixture data + admin-gate coverage (so the inline test doesn't duplicate)
- **`.planning/PROJECT.md`** — Key Decisions table (esp. "clasp v2.4+ NOT 3.x" from Phase 3 RESEARCH §6); the apps-script-deploy convention (`docs/apps-script-deploy.md`) for clasp push from owner's machine
- **`.planning/REQUIREMENTS.md`** — TEST-03 + TEST-04 acceptance criteria (and the explicit "discuss-phase decides whether to fold info-level items" clause that D-01 locks)
- **`.planning/ROADMAP.md` § Phase 10** — Goal + 5 success criteria + ship gate
- **`apps-script/vitest.config.ts`** — JSDOM environment declaration (Phase 8 D-01); test should still pass after 10-01's mountSidebar change
- **`apps-script/package.json`** — `npm run build` and `npm test` script definitions; clasp 2.4 dep pin

### Concrete file paths the planner needs

| File | Operation | Phase 10 reason |
|---|---|---|
| `apps-script/src/__tests__/test-helpers.ts` | edit (lines 620-625, eval site) | WR-01 IIFE wrap |
| `apps-script/src/__tests__/evictionSidebar.inline.test.ts` | edit (line 87, regex) | WR-02 tighten assertion |
| `apps-script/src/__tests__/searchIndex.test.ts` | edit (lines 94-114, try/catch block) | WR-03 remove swallowing |
| `apps-script/src/__tests__/searchSidebar.inline.test.ts` | edit (lines 53-58, dangling mock) | WR-04 resolve pending mock |
| `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` | create (new file) | TEST-03 new inline test |

That is the complete list. No other source files are touched.

### Specifics §1: WR-01 IIFE wrap — exact form

```typescript
// In apps-script/src/__tests__/test-helpers.ts, replace:
// (0, eval)(scriptSrc);
// with:
(0, eval)(`(function(){\n${scriptSrc}\n})();`);
```

Use a function-expression IIFE (NOT arrow), so `this === window` semantics are preserved for any inline script that references top-level `this`. Verify by re-running the existing 4 inline-JS tests post-change; all 8 happy+error cases must remain green.

### Specifics §2: WR-02 regex tightening — exact form

```typescript
// In evictionSidebar.inline.test.ts line 87, replace:
// expect(statusText).toMatch(/Marked 2|removed/i);
// with the exact-substring form:
expect(statusText).toContain('Marked 2 character(s) as removed');
```

This locks the test to the actual production copy. If a future commit changes the success-message wording, the test fails loudly rather than silently passing on a stale regex match.

### Specifics §3: WR-03 try/catch removal — exact form

The current Test 4 body at `searchIndex.test.ts:94-114` wraps the plan-locked assertion in try/catch. Remove the catch entirely so a failure throws and vitest reports it. If the plan-locked behavior is actually correct, the assertion passes and the test is clean; if it's wrong, the test fails — which is the entire point of a test.

### Specifics §4: WR-04 pending mock cleanup — exact form

After `m.dispatchRunCall('runSearch', {...})` resolves at `searchSidebar.inline.test.ts:53-58`, the inline success handler enqueues a `pushRecentSearchCall`. Test TS1 currently exits without resolving that second call, leaving it in the pending queue. Either:

- (preferred) Resolve it: add `m.dispatchRunCall('pushRecentSearch', null);` immediately after the existing dispatch.
- (acceptable) Drain the queue in an `afterEach`: `mock.drainPending();` (would need to be added to the `mountSidebar` mock contract).

Locked choice: explicit resolve in the test, NOT an `afterEach` mock-contract change. Reasoning: it's one extra line in one test vs an API expansion that all 5 inline test files would have to remember to invoke.

### Specifics §5: Admin-Mgmt happy + error fixture shapes

- **Happy path (TM1):** mount → fill `<input id="email">` with `"newadmin@example.com"` → click `<button id="add">` → assert `google.script.run.addAdmin` was called with `"newadmin@example.com"` → resolve via `m.dispatchRunCall('addAdmin', { ok: true })` → assert success message appears in `<div id="status">` and the input is cleared.
- **Error path (TM2):** mount → fill `<input id="email">` with `"existing@example.com"` → click `<button id="add">` → resolve via `m.dispatchRunCall.failure('addAdmin', new Error('Email already in allowlist'))` → assert the error message renders in `<div id="status">` and the input is NOT cleared (preserves user's typed value so they can correct it).

If the actual DOM element IDs in `showAdminMgmtSidebar.ts` differ from `#email` / `#add` / `#status`, use the actual ones — these are placeholders. The planner reads the trigger source to derive real IDs.

### Specifics §6: Ship gate criteria for Plan 10-03

The ship gate evidence required before `clasp push` runs:

| Check | Command | Pass criterion |
|---|---|---|
| Typecheck | `cd apps-script && npx tsc --noEmit` | Exit 0 |
| Bundle build | `cd apps-script && npm run build` | Produces `dist/Code.js`; exit 0 |
| Test suite | `cd apps-script && npm test -- --run` | All tests pass; count is ≥ 340 (336 baseline from STATE.md + 2 new Admin-Mgmt tests + the same 4 WR-fixed tests still passing — no test deletions) |
| Schema gate | `grep -c "writeMetaRow.*'_meta', 'schema_version', '3'" apps-script/src/lib/migrations.ts` | ≥ 1 |
| WatcherMaxSchemaVersion gate | `grep -c "WatcherMaxSchemaVersion = 3" internal/sheet/client.go` | = 1 |
| User checkpoint | (interactive) | User says "approved" after reviewing the green CI summary; supplies the dev-workbook scriptId or confirms env is set |
| Clasp push | `cd apps-script && clasp push` (from owner's machine) | Exit 0; success message shows the new dist/Code.js timestamp |
| Smoke | (manual, in dev workbook) | Open each of 5 sidebars; no console errors; expected UI renders |

</specifics>

<canonical_refs>
## Canonical refs (mandatory — full relative paths)

- `.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md` — Phase 8 decisions D-01..D-08 that govern apps-script test conventions
- `.planning/phases/08-test-infra-persistence-docs/08-REVIEW.md` — TEST-04 source: 4 warnings + 6 info-level items
- `.planning/phases/08-test-infra-persistence-docs/08-01-SUMMARY.md` — JSDOM env wiring (Phase 8 Plan 1)
- `.planning/phases/08-test-infra-persistence-docs/08-02-SUMMARY.md` — 4 sidebar inline-JS test files (template for the new Admin-Mgmt inline test)
- `.planning/PROJECT.md` — clasp v2.4+ NOT 3.x constraint; apps-script deploy convention
- `.planning/REQUIREMENTS.md` — TEST-03 + TEST-04 acceptance criteria
- `.planning/ROADMAP.md` § Phase 10 — Goal + success criteria + ship gate
- `apps-script/src/__tests__/test-helpers.ts` — `mountSidebar` helper (WR-01 fix site)
- `apps-script/src/__tests__/bankCoinSidebar.inline.test.ts` — Inline-test reference template (1 of 4)
- `apps-script/src/__tests__/charInfoSidebar.inline.test.ts` — Inline-test reference template (2 of 4)
- `apps-script/src/__tests__/evictionSidebar.inline.test.ts` — Inline-test reference template (3 of 4); also WR-02 fix site
- `apps-script/src/__tests__/searchSidebar.inline.test.ts` — Inline-test reference template (4 of 4); also WR-04 fix site
- `apps-script/src/__tests__/searchIndex.test.ts` — WR-03 fix site
- `apps-script/src/__tests__/adminMgmtSidebar.test.ts` — Existing trigger-level Admin-Mgmt test (for fixture-data reference; not modified)
- `apps-script/src/triggers/showAdminMgmtSidebar.ts` — The Admin-Mgmt sidebar trigger whose inline-JS the new test exercises
- `apps-script/vitest.config.ts` — JSDOM environment config (already in place from Phase 8)
- `apps-script/package.json` — `npm run build` + `npm test` scripts; clasp dependency pin
- `docs/apps-script-deploy.md` — clasp push runbook from the workbook owner's machine

</canonical_refs>

<code_context>
## Reusable Assets & Patterns

### Reusable from Phase 8

- `mountSidebar(html: string)` helper in `test-helpers.ts` — every new inline test calls this. Phase 10's 10-01 patches it once; 10-02 consumes the patched version.
- `m.dispatchRunCall(methodName, response)` fluent mock pattern — drives `google.script.run.withSuccessHandler(...).withFailureHandler(...).METHOD(...)` flows. The 4 existing inline test files use this; Admin-Mgmt test will reuse identically.
- `installSessionMock(email)` / `resetMocks()` — Apps Script global mocks; called from each test's setup. No changes needed for Phase 10.
- `buildSidebarHtml(theme)` exports for the 5 sidebars — Admin-Mgmt's exists at `apps-script/src/triggers/showAdminMgmtSidebar.ts` (Phase 8 D-02 locks "HTML stays inline, no extraction"); the inline test imports this builder, calls it with a fixture theme, feeds result into `mountSidebar`.

### Cross-cutting patterns to follow

- **2-test inline pattern (Phase 8 D-03)** — one happy path + one error path per sidebar. Admin-Mgmt follows this verbatim (D-03 here).
- **Per-test mount (NOT module-level)** — every `it(...)` calls `mountSidebar` in its own `beforeEach` or inline at the top of the test body. The IIFE wrap (D-02) makes this safe; without it, var-leakage across tests is the WR-01 bug.
- **Schema lock as a verification gate** — same grep pattern as Phases 7, 8, 9 (`WatcherMaxSchemaVersion = 3` + `writeMetaRow` schema_version pin). The verifier hook 5 in Phase 8 is the template.

### Anti-patterns to avoid (from prior phases)

- Do NOT introduce a per-test JSDOM realm (Option b for WR-01). Adds ~500ms/run, no benefit over IIFE wrap.
- Do NOT add `afterEach` global-state cleanup hooks (Option c for WR-01). Fragile, requires every test author to maintain the cleanup map.
- Do NOT extract sidebar HTML to standalone `.html` files (999.7 still deferred). The inline-JS test pattern works against inline HTML.
- Do NOT touch production code under any TEST-04 plan. If a test reveals a real production bug, surface it as a deviation note and route to a new backlog item, not a Phase 10 plan-edit.

</code_context>

<deferred>
## Deferred Ideas

Folded out of Phase 10 scope (each becomes a v1.1 backlog item; backlog numbers assigned in CONTEXT.md per ROADMAP convention):

| ID | Item | Reason |
|---|---|---|
| 999.24 | IN-01 — `COL_RACE = 14` / `COL_COUNT = 14` collision in `showCharInfoSidebar.ts` | Production code, schema-evolution foot-gun; outside "test quality" phase scope |
| 999.25 | IN-02 — Orphaned `squirebot:search:recent` CacheService key post-SEARCH-05 migration | Production code (lib/searchIndex.ts); 25-min TTL is self-healing |
| 999.26 | IN-03 — `evictionSidebar.inline.test.ts` bypasses admin gate | Defense-in-depth note; gate IS tested at trigger layer; informational |
| 999.27 | IN-04 — `showSearchSidebar.test.ts` Test 3 negative assertion is incomplete | Test-quality nit; current state has no user impact |
| 999.28 | IN-05 — `didYouMean('')` returns short-name candidates | Unreachable today (callers short-circuit); contract bug for future caller |
| 999.29 | IN-06 — `test-helpers.ts` CacheService mock TTL boundary nit | Mock-fidelity nit; tests don't exercise this boundary |

Also still deferred from earlier milestones (not surfaced in this discussion but worth re-flagging so the roadmapper doesn't lose them):
- 999.1 (bank-coin permission lock) — v1.1
- 999.2 (theme picker tile UI) — v1.1
- 999.7 (SIDEBAR_BODY → external `.html`) — v1.1 cosmetic refactor
- 999.11 (verification doctrine decision) — process change at v1.1 planning
- 999.12 (v2 wantlist + Discord pinger)
- 999.20, 999.21, 999.22, 999.23 (Phase 9 follow-ups from 09-REVIEW.md and OAuth-incident retros)

</deferred>

<meta_decision>
## Meta-Decision: applied tiebreaker for this phase

User explicitly delegated all four gray areas (D-1 info scope, D-2 mountSidebar fix, D-3 Admin-Mgmt depth, D-4 plan structure + ship gate) with the criterion: **"err on the side of making the end-user experience as simple and seamless as possible."**

Phase 10 is internal test work — no guildie-visible behavior changes. The criterion translates here to two operational sub-rules applied uniformly throughout the decisions above:

1. **Scope simplicity** — defer everything not strictly required by the phase title and goal. Six info-level items go to backlog (D-01); test depth matches the existing pattern, no expansion (D-03); 3 plans is the minimum that respects the test-helpers.ts dependency without conflating concerns (D-04).
2. **Maintenance simplicity** — choose the canonical / least-novel implementation. IIFE wrap (D-02) is one of the most well-understood JS patterns; symmetric 2-test pattern (D-03) means future maintainers learn one pattern across 5 sidebars; CI-green-before-clasp-push (D-04) is straightforward order-of-operations.

If during planning or execution a decision arises that wasn't covered above, downstream agents should apply the same tiebreaker: prefer the option that minimizes scope expansion and maintenance burden. This is consistent with the same user-criterion that drove Phase 9's invisible-UX tiebreaker.

</meta_decision>
