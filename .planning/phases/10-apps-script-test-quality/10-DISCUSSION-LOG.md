# Phase 10: Apps Script Test Quality — Discussion Log

**Date:** 2026-05-13
**Mode:** `/gsd-discuss-phase 10` (one-pass delegate — user supplied criterion "err on the side of making the end-user experience as simple and seamless as possible" and explicitly waived per-area discussion)
**Outcome:** CONTEXT.md committed; ready for `/gsd-plan-phase 10`

## Phase Boundary Stated

> Close v1.0.1-Phase-8-review-surfaced test-quality items: TEST-03 (Admin-Mgmt inline-JS test) + TEST-04 (4 warnings from `08-REVIEW.md` — mountSidebar realm leak, overly permissive regex, try/catch assertion swallow, leaked pending mock call). Patch-shaped; no schema changes; ship gate is `clasp push` to dev workbook + green CI.

Phase boundary accepted without challenge.

## Gray Areas Presented (multiSelect)

Four gray areas were surfaced for the user to choose:

1. **Info-level findings scope (D-1)** — TEST-04 covers 4 warnings; 6 info-level items (IN-01..IN-06 in `08-REVIEW.md`) are explicitly deferred to this discussion. Decide: fold any/all into Phase 10, or strictly stick to TEST-04 4 warnings?
2. **mountSidebar realm-leak fix approach (D-2)** — WR-01 says `(0, eval)(src)` leaks `var`/`function` declarations to globalThis. Three credible fixes: (a) IIFE wrap, (b) per-test JSDOM realm, (c) explicit globalThis cleanup hook.
3. **Admin-Mgmt test depth (D-3)** — TEST-03 says "at least one inline test." Phase 8 D-03 locked 1-happy + 1-error pattern for the other 4 sidebars. Admin-Mgmt has more inline behavior. Match the 2-test pattern for symmetry, or extend (3–4 tests) for the extra complexity?
4. **Plan structure + ship gate (D-4)** — REQ-per-plan vs area-split vs collapse; ship gate is `clasp push` to dev workbook — green-CI BEFORE push, or push-then-verify?

## User Response

User explicitly responded: **"I have no preference for any of those four gray areas. Please solve them however you see fit, but please err on the side of making the end-user experience as simple and seamless as possible."**

Per the saved memory `feedback_delegate_gray_areas.md` ("when user says 'no preference, apply rule X', stop AskUserQuestion-ing, lock all decisions in one pass, capture the rule in CONTEXT.md"), Claude proceeded to lock all four decisions in a single pass under the supplied criterion. No follow-up questions asked.

## Decisions Made by Claude (under "simple, seamless" criterion)

Phase 10 is INTERNAL test work with zero guildie-visible behavior changes. The criterion was translated to two operational sub-rules — scope simplicity and maintenance simplicity — applied uniformly across all four decisions:

| Gray Area | Locked Decision | Reasoning (per criterion) |
|---|---|---|
| D-1 Info-level scope | **DEFER ALL 6** (IN-01..IN-06) to v1.1 backlog as 999.24..999.29 | Scope simplicity: phase title is "Apps Script Test Quality" — IN-01 and IN-02 touch production code, the other 4 are nits with no current-state impact. Adding 6 advisory items into a tightly-scoped patch phase trades clarity for breadth. |
| D-2 mountSidebar fix | **IIFE wrap (Option a)** — `(0, eval)(\`(function(){ ... })();\`)` | Maintenance simplicity: 3-LOC change in test-helpers.ts. Per-test realm (b) adds ~500ms/run with no benefit; globalThis cleanup (c) is fragile and bug-prone. IIFE is canonical JS. |
| D-3 Admin-Mgmt test depth | **2 tests (1 happy + 1 error)** mirroring the existing 4 sidebars | Maintenance simplicity: symmetry across all 5 inline test files means one pattern to learn. Admin-Mgmt's extra complexity (admin gate, allowlist) is already covered at trigger layer by existing `adminMgmtSidebar.test.ts`. |
| D-4 Plan structure + ship gate | **3 sequential plans:** 10-01 TEST-04 fixes → 10-02 TEST-03 Admin-Mgmt test → 10-03 ship gate (CI green BEFORE clasp push) | Scope simplicity: REQ-per-plan mirrors Phase 9. Sequential because 10-02 consumes the test-helpers.ts patch from 10-01. CI-green-before-push prevents shipping a broken bundle to the dev workbook (direct serve of the "seamless" criterion). |

A fifth implicit decision (D-5 schema lock) was preserved verbatim from prior phases (no schema bump; `_meta.schema_version = 3`, `WatcherMaxSchemaVersion = 3`); not part of the gray-area discussion but recorded in CONTEXT.md for verification-hook completeness.

A meta-decision D-06 was recorded summarizing the tiebreaker for any future decisions arising during planning or execution that weren't covered above: apply the same scope/maintenance simplicity rule.

## Deferred Ideas Captured

| ID | Item | Why deferred from Phase 10 |
|---|---|---|
| 999.24 | IN-01 — `COL_RACE`/`COL_COUNT` collision in `showCharInfoSidebar.ts` | Production code, schema-evolution foot-gun; outside test-quality phase scope |
| 999.25 | IN-02 — Orphaned CacheService key post-SEARCH-05 migration | Production code (lib/searchIndex.ts); 25-min TTL is self-healing |
| 999.26 | IN-03 — `evictionSidebar.inline.test.ts` bypasses admin gate | Defense-in-depth; gate IS tested at trigger layer |
| 999.27 | IN-04 — `showSearchSidebar.test.ts` Test 3 incomplete negative assertion | Test-quality nit; current state has no user impact |
| 999.28 | IN-05 — `didYouMean('')` returns short-name candidates | Unreachable today; contract bug for future caller |
| 999.29 | IN-06 — `test-helpers.ts` CacheService mock TTL boundary nit | Mock-fidelity nit |

No scope creep was attempted by the user during this discussion (single delegate response, no follow-up suggestions outside the four presented gray areas).

## Canonical refs captured

15 refs added to CONTEXT.md `<canonical_refs>` block — see CONTEXT.md for full list. Notable: all 4 existing inline test files (`bankCoinSidebar.inline.test.ts`, `charInfoSidebar.inline.test.ts`, `evictionSidebar.inline.test.ts`, `searchSidebar.inline.test.ts`), the `mountSidebar` helper at `test-helpers.ts:620-625`, the Admin-Mgmt trigger source `showAdminMgmtSidebar.ts`, and `docs/apps-script-deploy.md` for the clasp-push runbook.

## Next Step

`/gsd-plan-phase 10` — planner reads this CONTEXT.md + canonical refs + 08-CONTEXT.md and produces 3 plans matching the D-04 structure (10-01 TEST-04 fixes → 10-02 TEST-03 Admin-Mgmt inline test → 10-03 ship gate). All wave dependencies are encoded in D-04.
