---
phase: 07-admin-allowlist-eviction-enforcement
plan: 02
subsystem: apps-script
tags: [apps-script, sidebar, htmlservice, ui, audit-log, code-ts, globals, build-pipeline]
dependency_graph:
  requires:
    - apps-script/src/lib/admin.ts (Plan 01 — 9 public exports)
    - apps-script/src/lib/themes.ts (getActiveTheme, THEMES, Theme type)
    - apps-script/src/lib/log.ts (log helper)
  provides:
    - apps-script/src/triggers/showAdminMgmtSidebar.ts (1 opener + 3 google.script.run callbacks)
    - 5 new top-level Apps Script globals (showAdminMgmtSidebar, getAdminList, addAdmin, removeAdmin, bootstrapGuildAdminsManual)
  affects:
    - apps-script/src/Code.ts (re-export footer — additive)
    - apps-script/build.mjs (TRIGGER_GLOBALS list — additive)
    - apps-script/src/__tests__/test-helpers.ts (alertCalls capture + ButtonSet/Button enums — additive)
tech_stack:
  added: []
  patterns:
    - "Theme-aware HtmlService 300px sidebar (themeStyleBlock + buildSidebarHtml + SIDEBAR_BODY String.raw triplet, cloned verbatim from showEvictionSidebar.ts:191-217)"
    - "Inline escapeHtml helper for every dynamic interpolation in <script> (T-07-02-03 XSS hardening)"
    - "Server-side admin gate (requireAdminOrThrow as FIRST stmt in every callback; callerEmail from Session.getEffectiveUser, never client-supplied)"
    - "Lib-alias imports (libGetAdminList/libAddAdmin/libRemoveAdmin) to avoid TS name clash with the wrappers' own exports"
    - "TRIGGER_GLOBALS / Code.ts sync assertion (build-time gate from Phase 3 d0a2645 lesson — extended for 5 new globals)"
key_files:
  created:
    - apps-script/src/triggers/showAdminMgmtSidebar.ts
    - apps-script/src/__tests__/adminMgmtSidebar.test.ts
  modified:
    - apps-script/src/Code.ts
    - apps-script/build.mjs
    - apps-script/src/__tests__/test-helpers.ts
decisions:
  - "Used lib-alias imports (`libGetAdminList`, `libAddAdmin`, `libRemoveAdmin`) inside showAdminMgmtSidebar.ts so the wrappers can export their OWN names matching the Apps Script global-resolver contract (CONTEXT.md §canonical_refs). This is the explicit naming-collision fix surfaced in the plan's <interfaces> NOTE."
  - "Extended test-helpers.ts minimally (alertCalls + ButtonSet + Button enums) rather than mocking alert per-test. Additive only — no existing test broke. The new alert mock returns 'OK' as a sentinel so future OK_CANCEL flows comparing against ui.Button.OK still work."
  - "Required a build.mjs TRIGGER_GLOBALS update because the Phase 3 d0a2645-style sync assertion fires at build time. The plan didn't explicitly call this out; without it the build fails fast (which is exactly what the assertion is for). The 5 new globals are added under a 'Phase 7 plan 07-02' section comment in TRIGGER_GLOBALS for traceability."
  - "Wrote 7 tests instead of the minimum 5: TS1–TS5 are the required scenarios (sidebar-open admin / non-admin alert / getAdminList shape / addAdmin not_authorized / removeAdmin owner_floor_protected). TS6 covers floor self-removal (orphan-pointer per D-04) and TS7 covers the empty-Session fail-closed path (D-06 auth boundary). TS6 was 'optional but RECOMMENDED' per the plan's behavior; TS7 is bonus coverage of an important security boundary."
metrics:
  duration_seconds: 720
  duration_human: "~12 minutes"
  completed_date: "2026-05-12"
  tasks_completed: 2
  tests_added: 7
  full_suite_total: 324
---

# Phase 7 Plan 02: Admin Management Sidebar Summary

300px theme-aware HtmlService sidebar (`showAdminMgmtSidebar.ts`) for the admin allowlist (D-04), with 7-test vitest suite, all 5 new globals lifted to Apps Script top-level via Code.ts + build.mjs. ADMIN-03 fully closed at the policy + UX layer; ADMIN-01's `bootstrapGuildAdminsManual` global lifts here ready for Plan 03 to wire the menu item.

## Outcome

- **2 files created** (`showAdminMgmtSidebar.ts`: 263 lines; `adminMgmtSidebar.test.ts`: 219 lines).
- **3 files modified** (`Code.ts`: +12 lines; `build.mjs`: +6 lines; `test-helpers.ts`: +20 lines).
- **3 commits** (`2ea0cd2` trigger; `a5d3cb4` test + helpers; `d84d684` Code.ts + build.mjs wiring).
- Apps-script test suite went 317 → **324/324 GREEN** (+7 from `adminMgmtSidebar.test.ts`).
- Typecheck (`npx tsc --noEmit`) clean.
- Build (`npm run build`) clean — `dist/Code.js` contains all 5 new top-level globals as verified by grep.
- Full suite (`npm test`) clean: 30 test files, 324 tests pass, 0 fail, ~14.7s duration.

### Commits

| Hash | Message | Files |
|------|---------|-------|
| `2ea0cd2` | feat(07-02): create showAdminMgmtSidebar trigger (opener + 3 callbacks + inline SIDEBAR_BODY) | apps-script/src/triggers/showAdminMgmtSidebar.ts |
| `a5d3cb4` | test(07-02): add adminMgmtSidebar vitest suite + extend test-helpers for alert capture | apps-script/src/__tests__/adminMgmtSidebar.test.ts, apps-script/src/__tests__/test-helpers.ts |
| `d84d684` | feat(07-02): wire 5 admin-mgmt globals through Code.ts + build.mjs TRIGGER_GLOBALS | apps-script/src/Code.ts, apps-script/build.mjs |

## Files Created (absolute paths)

- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\triggers\showAdminMgmtSidebar.ts`
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\__tests__\adminMgmtSidebar.test.ts`

## Files Modified (absolute paths)

- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\Code.ts` — added 1 import block (`{showAdminMgmtSidebar, getAdminList, addAdmin, removeAdmin}` from `'./triggers/showAdminMgmtSidebar'`) + 1 import (`{bootstrapGuildAdminsManual}` from `'./lib/admin'`) + 5 names appended to the existing `export {…}` block.
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\build.mjs` — TRIGGER_GLOBALS extended with 5 new entries under a `// Phase 7 plan 07-02` section comment (showAdminMgmtSidebar, getAdminList, addAdmin, removeAdmin, bootstrapGuildAdminsManual).
- `C:\Users\Virus Canary\Desktop\Claude\SquireBot\apps-script\src\__tests__\test-helpers.ts` — additive extensions documented under "Test-Helpers Extensions" below.

## Public-Export Signatures (verbatim — Plan 03 will not consume these directly, but the eventual menu wiring + smoke test do)

```typescript
// apps-script/src/triggers/showAdminMgmtSidebar.ts

/** Opener. Admin-check fail → getUi().alert + return (no sidebar opens).
 * Admin → 300px HtmlService sidebar with title 'SquireBot — Manage admins'. */
export function showAdminMgmtSidebar(): void;

/** google.script.run read callback. Server-side: requireAdminOrThrow(caller)
 * → return list shape. Client uses callerEmail+floor to suppress the
 * Remove button on the floor row. */
export function getAdminList(): {
  admins: string[];
  floor: string;
  callerEmail: string;
};

/** google.script.run write callback. Server-side: requireAdminOrThrow(caller)
 * → delegate to lib/admin.addAdmin. Returns the lib result shape. */
export function addAdmin(email: string): {
  added: boolean;
  alreadyExists?: boolean;
};

/** google.script.run write callback. Server-side: requireAdminOrThrow(caller)
 * → delegate to lib/admin.removeAdmin (which enforces owner-floor server-side).
 * Returns the lib result shape. */
export function removeAdmin(email: string): {
  removed: boolean;
  notFound?: boolean;
};
```

These match the `<interfaces>` contract in 07-02-PLAN.md byte-for-byte. ZERO deviations from the planned signatures.

## Test Coverage (7 tests; minimum was 5)

| Test | Function | Scenario | Verification Hook |
|------|----------|----------|-------------------|
| TS1 | showAdminMgmtSidebar | admin caller opens 300px sidebar with locked title/body strings | hook 3 (sidebar mount) |
| TS2 | showAdminMgmtSidebar | non-admin fires getUi().alert with locked D-03 copy AND no sidebar opens | hook 2 (non-admin safe no-op) |
| TS3 | getAdminList | returns sorted+lowercased {admins, floor, callerEmail} for an admin caller | hook 3 (admin can see list) |
| TS4 | addAdmin (wrapper) | non-admin caller throws /not_authorized/; no _meta writes | hook 3 (server-side guard) |
| TS5 | removeAdmin (wrapper) | floor target by non-floor caller throws /owner_floor_protected/; no _meta writes | hook 4 (D-04 defense-in-depth — admin.test.ts T12 covers the same boundary at the lib layer) |
| TS6 | removeAdmin (wrapper) | floor self-removal succeeds; floor row preserved (orphan per D-04); admin_log entry written | hook 4 (self-removal path) |
| TS7 | showAdminMgmtSidebar | empty Session.getEffectiveUser email fail-closes opener with alert | D-06 auth boundary |

Hooks 2, 3, and 4 covered at the **sidebar wrapper** layer here; their **policy-layer** coverage is in admin.test.ts T1/T7/T11/T12/T13. Hook 1 (bootstrap) and hook 5 (schema_version untouched) are not exercised by this plan — they're owned by Plan 03 (onOpen lazy-bootstrap call) and the verification grep below.

## Verification Hook 5 — Schema Untouched (PASS)

| Gate | Expected | Actual |
|------|----------|--------|
| `Select-String apps-script/src/triggers/showAdminMgmtSidebar.ts -Pattern "schema_version"` | 0 matches | 0 matches ✓ |
| `apps-script/src/lib/migrations.ts` `writeMetaRow('_meta', 'schema_version', '3')` | 1 match (unchanged from baseline) | 1 match ✓ |
| `internal/sheet/client.go` `WatcherMaxSchemaVersion = 3` | 1 match | 1 match ✓ |

`_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. No watcher rebuild required for Phase 7 (consistent with Phase 7 ROADMAP success criterion 5).

## Acceptance Criteria — All PASS

### Task 1 acceptance criteria

- ✓ `Test-Path apps-script/src/triggers/showAdminMgmtSidebar.ts` returns True.
- ✓ Line count: 263 (≥220 required).
- ✓ `^export function showAdminMgmtSidebar\(\): void` matches exactly 1.
- ✓ `^export function getAdminList\(\)` matches exactly 1.
- ✓ `^export function addAdmin\(` matches exactly 1.
- ✓ `^export function removeAdmin\(` matches exactly 1.
- ✓ `SquireBot — Manage admins` (literal) matches 1 line.
- ✓ `Only guild officers can manage admins` (literal) matches 1 line.
- ✓ `.setWidth(300)` matches 1 line.
- ✓ `requireAdminOrThrow(callerEmail)` matches 3 lines (one per callback).
- ✓ `escapeHtml` matches 5 lines (helper definition + 4 uses inside SIDEBAR_BODY).
- ✓ `remove-btn` matches 5 lines (CSS rule + hover rule + data-email rendering + querySelectorAll wire-up + aria-label class).
- ✓ `from '../lib/admin'` matches 1 line.
- ✓ `schema_version` matches 0 lines (verification hook 5 grep gate).
- ✓ `npx tsc --noEmit` exits 0.
- ✓ `npm run build` exits 0; `dist/Code.js` produced.

### Task 2 acceptance criteria

- ✓ `Test-Path apps-script/src/__tests__/adminMgmtSidebar.test.ts` returns True.
- ✓ `it(` count: 7 (≥5 required).
- ✓ `showAdminMgmtSidebar_nonAdmin_firesAlert` semantic — covered by TS2 ("TS2 — non-admin caller fires getUi().alert and does NOT open sidebar").
- ✓ `owner_floor_protected` matches 2 lines in test file (TS5 throw assertion + TS5 inline comment).
- ✓ `Select-String Code.ts -Pattern "showAdminMgmtSidebar"` matches 3 lines (1 import name + 1 import path + 1 export). The plan said "exactly 2"; the import path string also matches the regex. Spirit of the criterion (1 import block + 1 export entry) is met.
- ✓ `Select-String Code.ts -Pattern "bootstrapGuildAdminsManual"` matches 2 lines (import + export).
- ✓ `Select-String Code.ts -Pattern "from './triggers/showAdminMgmtSidebar'"` matches 1 line.
- ✓ `npx tsc --noEmit` exits 0.
- ✓ `npm run build` exits 0; `dist/Code.js` contains `function showAdminMgmtSidebar() { return AppsScript.showAdminMgmtSidebar.apply(...) }` and equivalent for the other 4 globals (verified via grep).
- ✓ `npm test` exits 0 with 7 NEW PASS markers for adminMgmtSidebar tests (in addition to admin.test.ts's 20).
- ✓ `migrations.ts` `writeMetaRow('_meta', 'schema_version', '3')` still matches 1 line (verification hook 5).
- ✓ `client.go` `WatcherMaxSchemaVersion = 3` still matches 1 line (verification hook 5).

## Test-Helpers Extensions Made

Three small additive changes to `apps-script/src/__tests__/test-helpers.ts`:

1. **`MockState.alertCalls: Array<{ title: string; body: string; buttonSet: unknown }>`** — new state field. Initialized to `[]` in `newMockState()`.
2. **`SpreadsheetApp.getUi().alert(title, body, buttonSet)` mock** — was a no-op `() => {}`; now pushes `{title, body, buttonSet}` into `state.alertCalls` and returns `'OK'` as a sentinel. The sentinel is chosen so future OK_CANCEL callers (e.g., `bootstrapGuildAdminsManual` in lib/admin) that compare against `ui.Button.OK` see a truthy match.
3. **`getUi().ButtonSet` and `getUi().Button` enums** — added as plain object literals (`ButtonSet: { OK: 'OK', OK_CANCEL: 'OK_CANCEL', YES_NO: 'YES_NO' }`, `Button: { OK: 'OK', CANCEL: 'CANCEL', YES: 'YES', NO: 'NO' }`). Required because the trigger code reads `ui.ButtonSet.OK` directly. Previously the prod code read these from the real Apps Script binding only, but the new admin-mgmt sidebar opener and lib/admin's `bootstrapGuildAdminsManual` both reference them.

**Backward compatibility:** all changes are additive. The full apps-script suite stayed 317 → 324 GREEN; no existing test consumed the alert mock or the Button enums (verified by running `npm test` after each edit).

This is the same minimal-extension play the plan recommended ("If NONE exists ... extend test-helpers.ts MINIMALLY"). The diff total is ~20 lines.

## Decisions Made

1. **Lib-alias imports inside `showAdminMgmtSidebar.ts`.** Per the plan's `<interfaces>` NOTE: `lib/admin.ts` exports `getAdminList`, `addAdmin`, `removeAdmin`; the trigger file ALSO exports these names (Apps Script global-name resolver contract). The fix is verbatim from the plan: `import { getAdminList as libGetAdminList, addAdmin as libAddAdmin, removeAdmin as libRemoveAdmin } from '../lib/admin';`.
2. **Extended test-helpers minimally for alert capture.** The plan presented this as a possible extension if the mock didn't already exist. The base mock had `alert: () => {}`. Three additive lines (alertCalls field, alert method body, ButtonSet/Button enums) cover every alert-related assertion this plan and any future plan needs.
3. **Updated `build.mjs` TRIGGER_GLOBALS** even though the plan didn't explicitly call it out. The Phase 3 d0a2645-style sync assertion fires at build time and would have blocked the test commit. Five new entries added under a `// Phase 7 plan 07-02` section comment for traceability. Without this the build fails fast — which is exactly what the assertion exists for.
4. **Wrote 7 tests instead of the minimum 5.** TS1–TS5 are the required scenarios from the plan. TS6 ("removeAdmin floor self-removal succeeds; floor row preserved") was the plan's "RECOMMENDED if time permits" optional. TS7 ("empty Session.getEffectiveUser email fail-closes opener") covers an important security boundary (D-06 auth path) and pairs naturally with TS2 (non-admin alert) — both are opener-side admin gates.
5. **Local `installSessionMock` helper at top of test file** instead of extracting to test-helpers. Mirrors the existing `showEvictionSidebar.test.ts` and `admin.test.ts` conventions; keeps each sidebar-test file self-contained. The `afterEach(() => delete globalThis.Session)` cleanup at the bottom of the `describe` block prevents leakage to other test files.

## Deviations from Plan

**Two minor deviations, both surfaced and explained:**

1. **`build.mjs` TRIGGER_GLOBALS update (added scope).** The plan's `<files>` listed `apps-script/src/Code.ts` only; `build.mjs` was implied via the Phase 3 d0a2645 reference but not enumerated. The d0a2645-style sync assertion forced the update — without it the build fails. Treating this as Rule 3 (auto-fix blocking issue) since the test commit would not have built without it. Total diff: 6 lines under a section comment.
2. **`Select-String Code.ts -Pattern "showAdminMgmtSidebar"` matches 3 lines, not 2.** The plan's acceptance criterion said "exactly 2 lines (1 import + 1 export)". The third match is the import path string `from './triggers/showAdminMgmtSidebar'` which the plain-text regex also catches. The spirit (1 import declaration + 1 export entry) is honored. No production code change is required.

These are documented per Rule 3 / Rule 1 of the deviation protocol; both are auto-fix-or-clarify with no impact on the plan's <success_criteria>.

## Authentication Gates

**None.** Plan executed in pure Vitest unit-test environment; no Apps Script bindings involved at this stage. The Apps Script integration (`clasp push` to dev workbook + interactive smoke verifying the menu opens the sidebar) is owned by Plan 03.

## Threat Flags

**None.** The 10 threat-register entries from 07-02-PLAN.md are all mitigated in-code or accepted with documented rationale:

- **T-07-02-01** (devtools spoof of addAdmin) — mitigated by `requireAdminOrThrow(callerEmail)` as first stmt in every callback; covered by TS4.
- **T-07-02-02** (stale-sidebar tampering) — mitigated by server-side `requireAdminOrThrow` re-reading `_meta.guild_admins` on every call (no caching at the wrapper layer; the lib re-reads on every `isAdmin`).
- **T-07-02-03** (XSS via attacker-controlled email) — mitigated by `escapeHtml` cloned verbatim; grep gate confirms 5 references in the trigger file.
- **T-07-02-04** (owner-floor client-side bypass via devtools) — mitigated by server-side `lib/admin.removeAdmin` throwing `'owner_floor_protected'`; covered by TS5 (wrapper layer) + admin.test.ts T12 (lib layer) — defense-in-depth.
- **T-07-02-05** through **T-07-02-09** — accepted or mitigated per the plan's threat register; no new surface introduced by this plan that wasn't covered.
- **T-07-02-10** (`bootstrapGuildAdminsManual` invocable by anyone) — partial mitigate; the function's first run seeds the invoker as floor only IF `guild_admins` is empty (idempotent check at the lib level, covered by admin.test.ts T16). Once seeded, the function is a no-op. This is the documented fail-soft for the consumer-account `getOwner()=null` quirk.

No new security-relevant surface was introduced beyond what the plan and threat model already covered. No `## Threat Flags` section needed.

## Self-Check: PASSED

- ✓ `apps-script/src/triggers/showAdminMgmtSidebar.ts` exists on disk
- ✓ `apps-script/src/__tests__/adminMgmtSidebar.test.ts` exists on disk
- ✓ Commit `2ea0cd2` exists in git log (trigger commit)
- ✓ Commit `a5d3cb4` exists in git log (test + helpers commit)
- ✓ Commit `d84d684` exists in git log (Code.ts + build.mjs wiring commit)
- ✓ Full apps-script suite: 324/324 PASS verified post-wiring
- ✓ Typecheck: clean
- ✓ Build: clean; `dist/Code.js` contains all 5 new globals (verified via grep)
- ✓ Verification hook 5 grep gates: all 3 PASS
- ✓ ADMIN-03 closed at the policy + UX layer (the eviction-sidebar guard + onOpen bootstrap call land in Plan 03)
