---
phase: 07-admin-allowlist-eviction-enforcement
verified: 2026-05-12T06:00:00Z
status: verified
score: 5/5 must-haves verified
goal_achieved: true
verification_date: 2026-05-12
requirements_status:
  ADMIN-01: complete
  ADMIN-02: complete
  ADMIN-03: complete
verification_hooks:
  hook_1: PASS
  hook_2: PASS
  hook_3: PASS
  hook_4: PASS
  hook_5: PASS
code_review:
  warnings_total: 5
  warnings_fixed: 5
  info_total: 6
  info_deferred: 6
notes: |
  Phase 7 SHIPPED + UAT-VERIFIED. 5/5 verification hooks PASS in dev-workbook smoke (07-03-SMOKE.md, operator boejowen@gmail.com, dev workbook script ID 1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT). All 5 code-review warnings (WR-01..WR-05) auto-fixed via gsd-code-fixer; 6 info items deferred to a future polish pass. apps-script vitest 327/327 GREEN re-run during this verification (30 test files, 9.18s). Go test suite 15/15 packages PASS (cached). Notable mid-smoke deviation: latent v1.0 OAuth-scope manifest gap (missing userinfo.email) surfaced by Phase 7's correct fail-closed admin guards on Hook 3, fixed inline (commit 544bef8) — non-sensitive scope addition, no Production-consent change required; side-effect retires pre-existing v1.0 'initiated_by=unknown' silent audit-log fallback. WR-01 XSS attribute-escape gap caught by code review and fixed pre-phase-close (commit cbf6f2d) — defense in depth via new escapeAttr() helper + tightened RFC 5321 subset email regex in addAdmin. Schema impact zero (_meta.schema_version=3, WatcherMaxSchemaVersion=3 both untouched).
---

# Phase 7: Admin Allowlist + Eviction Enforcement — Verification Report

**Phase Goal (ROADMAP §53-62):** Officer-only eviction is enforced by code (not by social convention), and admins can manage the allowlist without risk of locking themselves out.

**Verified:** 2026-05-12T06:00:00Z
**Status:** verified (passed)
**Score:** 5/5 verification hooks + 3/3 requirements + 5/5 code-review warnings closed
**Re-verification:** No — initial verification pass.

---

## Goal-Backward Analysis

The phase goal decomposes into three observable outcomes:

1. **Code-enforced (not convention-enforced) officer-only eviction** — verified at `apps-script/src/triggers/showEvictionSidebar.ts:49-75` (opener admin guard via `isAdmin(callerEmail)` + `getUi().alert` non-admin path) AND at `:77-101`, `:108-140`, `:147+` (each of the 3 callbacks calls `requireAdminOrThrow(callerEmail)` as its FIRST statement before any read or write). The check is server-side, identity-sourced from `Session.getEffectiveUser().getEmail()` (load-bearing per CLAUDE.md — `getActiveUser` would return script owner). Hook 2 PASS (07-03-SMOKE.md) confirms the fail-closed path live: non-admin saw "Not authorized" modal + zero `_char_owner` flips + zero `eviction_log` envelope appends.

2. **Admin-management UX with owner-floor lockout** — verified at `apps-script/src/triggers/showAdminMgmtSidebar.ts:49-111` (1 opener + 3 google.script.run callbacks: `getAdminList`, `addAdmin`, `removeAdmin`, all admin-gated server-side). Owner-floor enforcement is two-layer: client-side at `:222-228` (Remove button suppressed when `email === floor && callerEmail !== floor`); server-side at `apps-script/src/lib/admin.ts:222-229` (`removeAdmin` throws `Error('owner_floor_protected')` under the lock). Hook 4 PASS confirmed visually: joseph.bowen2 (non-floor admin) saw boejowen@gmail.com row with `(owner)` annotation and NO Remove button.

3. **Lazy onOpen bootstrap (zero-click for 95% case)** — verified at `apps-script/src/triggers/onOpen.ts:19-23` (try/catch-wrapped `bootstrapGuildAdmins()` call; never throws out of onOpen per D-01) AND at `apps-script/src/lib/admin.ts:273-330` (idempotent — returns `{bootstrapped:false, reason:'already_initialized'}` when `admins.length > 0`; lock-wrapped with `lock_busy` swallow path). Hook 1 PASS confirmed live: first open wrote `_meta.guild_admins=["boejowen@gmail.com"]` + `_meta.workbook_owner_floor=boejowen@gmail.com` + one `admin_log` bootstrap entry; second open did NOT append a duplicate (three rows byte-for-byte unchanged).

All three goal components verified at the artifact level (file + line numbers above), at the wiring level (Code.ts re-exports + build.mjs TRIGGER_GLOBALS sync — see §Required Artifacts), and at the behavioral level (smoke evidence + 327 unit tests + Hook 5 grep gates).

---

## Verification Hooks (5 from 07-CONTEXT.md §verification_hooks)

| # | Hook | Status | Evidence |
|---|------|--------|----------|
| 1 | `_meta.guild_admins` exists + contains owner; re-bootstrap idempotent | PASS | Live: 07-03-SMOKE.md Hook 1 — first open wrote 3 `_meta` rows; second open no-op (byte-for-byte unchanged). Unit: `admin.test.ts` T15 (empty meta → writes seed/floor/log), T16 (already-seeded → no-op). Code path: `admin.ts:273-330` (idempotent guard at :283-287). |
| 2 | Non-admin sees "not authorized" modal + safe no-op | PASS | Live: 07-03-SMOKE.md Hook 2 — owner temporarily set `guild_admins=["someone-else@example.com"]`, clicked Evict Guildie → modal title "Not authorized", body "Only guild officers can evict members…", clicked OK → no sidebar opened, zero `_char_owner` flips, zero `eviction_log` appends, zero `admin_log` appends. Code path: `showEvictionSidebar.ts:60-68` (opener) + `:85`, `:116`, `:156` (3 callbacks each call `requireAdminOrThrow` FIRST). Unit: `showEvictionSidebar.test.ts` Test 12 reframed to assert empty-Session → `not_authorized` throw + no writes. |
| 3 | Admin can add peer; new admin's eviction sidebar opens | PASS (after OAuth scope fix mid-smoke) | Live: 07-03-SMOKE.md Hook 3 — owner added joseph.bowen2@gmail.com via Manage Admins sidebar; status region confirmed "Admin added". Joseph in second browser session (post Drive Editor share + post OAuth re-consent for the new userinfo.email scope) opened Evict Guildie → sidebar opened normally, email selector populated. Code path: `showAdminMgmtSidebar.ts:91-100` (addAdmin callback) → `admin.ts:145-190` (lock-wrapped, validates RFC 5321 subset regex, idempotent on existing). Unit: `admin.test.ts` T7-T11 add-admin scenarios; `adminMgmtSidebar.test.ts` TS1-TS3. |
| 4 | Admin can remove another; owner-floor cannot be removed by non-floor | PASS | Live: 07-03-SMOKE.md Hook 4 — joseph.bowen2 (non-floor) opened Manage Admins → boejowen@gmail.com row showed `(owner)` annotation + NO Remove button (client-side suppression); joseph's own row had Remove button. Server-side defense-in-depth covered by unit tests (not exercised live — would need devtools console injection). Code path: client `showAdminMgmtSidebar.ts:222-228`; server `admin.ts:217-229` (under-lock floor check throws `owner_floor_protected`). Unit: `admin.test.ts` T12 (`removeAdmin(floor, non_floor)` throws), T13 (self-removal of floor allowed), T14 (admin removes other admin). |
| 5 | `_meta.schema_version=3` + `WatcherMaxSchemaVersion=3` (grep gates) | PASS | `apps-script/src/lib/migrations.ts:67,97` — `writeMetaRow('_meta', 'schema_version', '3')` unchanged. `internal/sheet/client.go:44` — `WatcherMaxSchemaVersion = 3` unchanged. No Phase 7 source file references either constant. |

---

## Required Artifacts (Three-Level Verification)

| Artifact | Exists | Substantive | Wired | Status |
|----------|--------|-------------|-------|--------|
| `apps-script/src/lib/admin.ts` | ✓ | ✓ (379 LOC, 8 exports + 1 module-private helper post WR-02 fix) | ✓ (imported by `showAdminMgmtSidebar.ts`, `showEvictionSidebar.ts`, `onOpen.ts`, `Code.ts`) | VERIFIED |
| `apps-script/src/triggers/showAdminMgmtSidebar.ts` | ✓ | ✓ (14.8 KB, 1 opener + 3 callbacks + inline SIDEBAR_BODY String.raw template + escapeAttr() helper from WR-01 fix) | ✓ (re-exported in Code.ts:51-55, listed in TRIGGER_GLOBALS at build.mjs:72-75) | VERIFIED |
| `apps-script/src/triggers/showEvictionSidebar.ts` (Phase 5 file, Phase 7 modified) | ✓ | ✓ (admin guard inserted at opener `:60-68` + each of 3 callbacks `:85`, `:116`, `:156`) | ✓ (existing wiring preserved; new import `:40` for `isAdmin/requireAdminOrThrow/normalizeEmail`) | VERIFIED |
| `apps-script/src/triggers/onOpen.ts` (Phase 3 file, Phase 7 modified) | ✓ | ✓ (lazy bootstrap try/catch at `:19-23` + 2 new menu items at `:40` and `:46`) | ✓ (`bootstrapGuildAdmins` import at `:9`; menu items target `showAdminMgmtSidebar` and `bootstrapGuildAdminsManual` globals — both lifted in build.mjs) | VERIFIED |
| `apps-script/appsscript.json` | ✓ | ✓ (5 oauthScopes including new `userinfo.email` per WR/Rule-3 fix commit 544bef8) | ✓ (clasp-pushed to dev workbook 2026-05-12) | VERIFIED |
| `apps-script/src/__tests__/admin.test.ts` | ✓ | ✓ (T1-T20 base suite + T21-T23 from WR-03 fix; 23.5 KB) | ✓ (vitest discovery; 23 tests passed in 327/327 run) | VERIFIED |
| `apps-script/src/__tests__/adminMgmtSidebar.test.ts` | ✓ | ✓ (TS1-TS7; 9.1 KB) | ✓ (vitest discovery; passed in 327/327 run) | VERIFIED |
| `apps-script/src/Code.ts` re-exports | ✓ | ✓ (5 new globals at `:51-55, :75-76`) | ✓ (build.mjs sync assertion at build time enforces parity) | VERIFIED |
| `apps-script/build.mjs` TRIGGER_GLOBALS | ✓ | ✓ (5 Phase 7 entries at `:72-76`) | ✓ (footer rebuilds top-level globals from this list; sync assertion gates the build) | VERIFIED |

---

## Key Link Verification (Wiring)

| From | To | Via | Status |
|------|-----|------|--------|
| onOpen menu | `showAdminMgmtSidebar` global | `addItem('Manage Admins…', 'showAdminMgmtSidebar')` at onOpen.ts:40 | WIRED |
| onOpen menu | `bootstrapGuildAdminsManual` global | `addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')` at onOpen.ts:46 | WIRED |
| onOpen lazy boot | `bootstrapGuildAdmins()` | direct call at onOpen.ts:20 (inside try/catch) | WIRED |
| `showEvictionSidebar` opener | `isAdmin()` | import + call at showEvictionSidebar.ts:40, :60 | WIRED |
| `showEvictionSidebar` callbacks ×3 | `requireAdminOrThrow()` | first statement of each callback (`:85, :116, :156`) | WIRED |
| `showAdminMgmtSidebar` opener + callbacks ×3 | `requireAdminOrThrow()` / `isAdmin()` | first statement of each (`:55, :83, :96, :107`) | WIRED |
| `addAdmin`/`removeAdmin` (lib) | `LockService.getDocumentLock().tryLock(30000)` envelope | admin.ts:161-189, :211-258 | WIRED (post WR-04 fix: auth re-checked under the lock) |
| `addAdmin`/`removeAdmin`/`bootstrapGuildAdmins` | `appendAdminLogEntry` (module-private) | admin.ts:179-184, :246-251, :305-322 | WIRED (post WR-02 fix: `export` removed; only same-file callers) |
| Bundle deploy | dev workbook | `clasp push --force` of `dist/Code.js` + `dist/appsscript.json` (post-OAuth-scope-fix re-push) | WIRED (script ID 1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT, 2026-05-12) |

---

## Data-Flow Trace (Level 4)

| Artifact | Data Source | Produces Real Data | Status |
|----------|-------------|--------------------|--------|
| `showAdminMgmtSidebar` admin list rendering | `getAdminList()` callback → `lib/admin.getAdminList()` → `readMetaRows('_meta')` | Yes — reads JSON-array-encoded `_meta.guild_admins` cell, normalizes lowercase+sort, returns `{admins, floor, callerEmail}` | FLOWING (Hook 1 confirmed `_meta.guild_admins=["boejowen@gmail.com"]` written; Hook 3 confirmed sidebar populated with both admins after add) |
| `showEvictionSidebar` email selector | `getEvictionEmails()` → `_char_owner` row scan | Yes — pre-existing Phase 5 path; Phase 7 only added the admin-guard prologue | FLOWING (Hook 3: joseph.bowen2's eviction sidebar populated email selector from `_char_owner`) |
| `onOpen` bootstrap | `getActiveSpreadsheet().getOwner().getEmail()` | Yes — Hook 1 confirmed live `boejowen@gmail.com` written to all three `_meta` rows on first open | FLOWING |

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| apps-script test suite | `cd apps-script && npm test` | 30 test files, 327 tests, PASS, 9.18s | PASS |
| Go test suite | `go test ./...` | 15 packages PASS (cached) | PASS |
| `_meta.schema_version` unchanged | `grep "writeMetaRow.*'schema_version', '3'" apps-script/src/lib/migrations.ts` | matches lines 67, 97 (both v=3 writes); no v=4 string anywhere | PASS |
| `WatcherMaxSchemaVersion` unchanged | `grep "WatcherMaxSchemaVersion" internal/sheet/client.go` | matches line 44 (`= 3`); no other assignment | PASS |
| build.mjs TRIGGER_GLOBALS sync | (assertion runs in `npm run build`) | All 5 Phase 7 globals present at build.mjs:72-76; CI sync gate passes per 07-03-SUMMARY | PASS |
| Live dev-workbook smoke (5 hooks) | manual interactive — workbook owner | 5/5 PASS per 07-03-SMOKE.md | PASS |

---

## Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| ADMIN-01 | 07-01 + 07-03 | `_meta.guild_admins` row maintains authorized-officer list; bootstrap on first run; idempotent on subsequent runs | SATISFIED | Bootstrap primitive `bootstrapGuildAdmins` at `admin.ts:273-330` (idempotent guard at `:283-287`); lazy call-site at `onOpen.ts:19-23`; Hook 1 PASS confirmed live bootstrap + idempotent re-open. Unit: T15-T19. Commits: dfc3533, 8780222, c3c0033. |
| ADMIN-02 | 07-03 | Eviction sidebar reads `_meta.guild_admins`; refuses non-admin invokers; non-admins see clear "not authorized"; safe no-op | SATISFIED | Opener guard at `showEvictionSidebar.ts:60-68`; per-callback `requireAdminOrThrow` at `:85, :116, :156`; Hook 2 PASS confirmed modal + zero writes. Unit: showEvictionSidebar.test.ts (Test 12 reframed for new auth boundary). Commits: 7f7ffb0, 1937fd0, 544bef8. |
| ADMIN-03 | 07-01 + 07-02 + 07-03 | Admin management UX (add/remove other admins); workbook-owner-floor enforced (only owner can remove themselves) | SATISFIED | UX shipped at `showAdminMgmtSidebar.ts` (entire file); owner-floor server-side at `admin.ts:222-229` (`owner_floor_protected` throw); client-side suppression at `showAdminMgmtSidebar.ts:222-228`. Hooks 3+4 PASS confirmed peer-add round-trip + visual owner-floor lockout. Unit: T12-T14, TS1-TS7. Commits: 2ea0cd2, a5d3cb4, d84d684. |

No orphaned requirements: REQUIREMENTS.md ADMIN section maps exactly the three IDs, all three covered.

---

## Code Review Status

**Source:** `07-REVIEW.md` (depth=standard, 2026-05-11T23:30:00Z) — 0 blockers, 5 warnings, 6 info.

### Warnings — 5/5 Fixed (07-REVIEW-FIX.md, 2026-05-12T04:57:00Z)

| ID | Title | Status | Fix Commit |
|----|-------|--------|------------|
| WR-01 | `escapeHtml` doesn't escape quotes — admin-to-admin HTML-attribute injection | FIXED | `cbf6f2d` — added `escapeAttr()` helper at `showAdminMgmtSidebar.ts:198-202`; switched attribute interpolations at `:227`; tightened `addAdmin` validation to RFC 5321 subset regex at `admin.ts:157` |
| WR-02 | `appendAdminLogEntry` exported but caller-must-hold-lock — unsafe public surface | FIXED | `f10669f` — dropped `export` keyword (now module-private at `admin.ts:124`); rewrote T20 to drive malformed-log path through `addAdmin` |
| WR-03 | `bootstrapGuildAdminsManual` zero test coverage; missing `toast()` mock | FIXED | `6e32cb3` — added `toast()` stub + `state.alertReturn` override in test-helpers.ts; added T21 (empty Session), T22 (Cancel), T23 (happy path) — all PASS in 327/327 |
| WR-04 | TOCTOU window between `requireAdminOrThrow` and `lock.tryLock` | FIXED | `0711ae3` — moved `requireAdminOrThrow(callerEmail)` INSIDE the try-block in both `addAdmin` (`admin.ts:171`) and `removeAdmin` (`admin.ts:217`); input validation stays outside lock |
| WR-05 | `removeAdmin` accepts non-email targets — silent `notFound` instead of `invalid_email` | FIXED | `a91b62b` — mirrored `addAdmin`'s `@`-validation in `removeAdmin` (`admin.ts:206-209`) |

### Info — 6/6 Deferred to Future Polish Pass

Per `07-REVIEW-FIX.md` `fix_scope: critical_warning`, IN-* items are out of scope for the post-ship fix wave. Documented in `07-REVIEW.md` for future polish:

| ID | Title | Disposition |
|----|-------|-------------|
| IN-01 | Dead code — `\|\| resolveInitiatedBy()` fallback unreachable in `addAdmin`/`removeAdmin` | Deferred (cosmetic; reachability-analysis only) |
| IN-02 | `themeStyleBlock` interpolates theme tokens into `<style>` without escaping | Deferred (clone of pre-existing Phase 5 pattern; THEMES registry is module-private/trusted today) |
| IN-03 | `getEvictionEmails` doesn't lowercase emails before dedup/compare | Deferred (pre-existing Phase 5 bug newly visible behind Phase 7 guards) |
| IN-04 | `bootstrapGuildAdmins` writes `bootstrap_failed` for `owner_null` but NOT `lock_busy` | Deferred (intentional asymmetry — `lock_busy` means we don't hold the lock; documented behavior) |
| IN-05 | `bootstrapGuildAdmins` doc comment vs. throw-from-`appendAdminLogEntry` edge | Deferred (low-impact; docstring vs. defensive code) |
| IN-06 | Stale comment in `showAdminMgmtSidebar.ts` referencing eviction-sidebar `:257` (actual: `:300`) | Deferred (comment-only) |

---

## Notable Deviations

### 1. (Significant) Latent v1.0 OAuth-scope manifest gap surfaced and retired mid-smoke

Phase 7's correct fail-closed admin guards (D-06: empty `Session.getEffectiveUser` → `not_authorized`) immediately exposed a pre-existing v1.0 manifest defect during Hook 3: the original `apps-script/appsscript.json` did not declare `https://www.googleapis.com/auth/userinfo.email`, so under consumer @gmail.com accounts `Session.getEffectiveUser().getEmail()` silently returned `""`. Joseph.bowen2 (a correctly-added admin) saw "Not authorized" on his first eviction click. Fix shipped inline as commit `544bef8` (single-commit Rule 3 deviation; non-sensitive scope addition; no Production-consent change required; no Google verification audit triggered). **Side effect:** retires a pre-existing latent v1.0 bug — every eviction's `initiated_by` audit-log field has been silently writing `'unknown'` (the D-06 soft fallback) across the entire v1.0 release because of the same scope omission. Verified at `apps-script/appsscript.json` (5 oauthScopes, includes `userinfo.email`).

### 2. (Material catch) WR-01 XSS attribute-escape gap caught by code review and fixed pre-phase-close

The inline `escapeHtml` cloned from `showEvictionSidebar.ts` uses `textContent` → `innerHTML` round-trip, which per the HTML serialization spec escapes `<`, `>`, `&`, U+00A0 — but NOT `"` or `'`. Phase 5's eviction sidebar gets away with this because it only interpolates into element-text contexts; Phase 7's admin-mgmt sidebar interpolates into `data-email` and `aria-label` attribute contexts (`showAdminMgmtSidebar.ts:227`), creating an admin-to-admin attribute-injection vector. Fix shipped as commit `cbf6f2d` two layers deep: (a) new `escapeAttr()` helper that explicitly escapes `&<>"'`, switched attribute interpolations to it; (b) server-side belt-and-suspenders — tightened `addAdmin`'s email validation from `target.indexOf('@') !== -1` to a conservative RFC 5321 subset regex (`/^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$/i`) so quote/angle/whitespace characters are rejected before they reach `_meta`. Verified at `admin.ts:157` (regex) and `showAdminMgmtSidebar.ts:198-202, :227` (escapeAttr). Threat model boundary unchanged (insider-only — attacker must already be admin to call `addAdmin`); the hardening closes the in-context exploit path the threat register explicitly flagged.

### 3. (Minor) Rule 1 — Test 12 reframe in `showEvictionSidebar.test.ts`

Pre-Phase-7 Test 12 asserted `initiated_by='unknown'` audit-log soft-fallback when `Session.getEffectiveUser` returns empty. Post-Phase-7 the same Session call drives the new admin guard FIRST and fail-closes empty per `requireAdminOrThrow`. Test 12 was reframed to assert the new auth boundary (empty Session → `not_authorized` throw + zero writes) — preserves the test's intent while honoring the new sequence. The audit-log `'unknown'` soft-fallback code path in `commitEviction` remains as residual defensive code (would only be reached if Session returned non-empty at the guard and empty at the later log call — non-physical).

---

## Gaps Summary

None. All 5 verification hooks PASS. All 3 requirements SATISFIED. All 5 code-review warnings FIXED. 6 info items explicitly deferred to a future polish pass per the fix_scope contract.

---

## Recommendation

**Phase 7 verdict: SHIPPED + VERIFIED.** Goal achieved.

Phase 7 successfully retires v1.0's "social convention only" eviction model. Officer-only enforcement is now code-mediated at the opener AND each callback layer (defense in depth against stale-sidebar replay), with server-side identity sourced from `Session.getEffectiveUser` per CLAUDE.md's load-bearing distinction. The admin-management UX is wired end-to-end with two-layer owner-floor lockout (client suppression + server throw). Lazy `onOpen` bootstrap is idempotent, lock-wrapped, and never throws out of the menu chain. Schema impact zero (no `_meta.schema_version` bump, no watcher rebuild).

The phase additionally retired a latent v1.0 OAuth-scope manifest bug (audit-log `initiated_by='unknown'` silent fallback across the entire v1.0 release) as a side effect of correct fail-closed authorization design — a high-leverage outcome for the Hook 3 mid-smoke deviation. The WR-01 XSS gap was caught by code review pre-ship and hardened in two layers.

No follow-up required for v1.0.1. The 6 IN-* polish items are documented in `07-REVIEW.md` and can be batched into a future cleanup phase or absorbed into Phase 8 if the planner sees fit (none are blocking; IN-03 in particular is a pre-existing Phase 5 bug that pre-dates this phase). Phase 8 (Test Infra + Persistence + Docs Backfill) is unblocked.

---

_Verified: 2026-05-12T06:00:00Z_
_Verifier: Claude (gsd-verifier)_
_Mode: goal-backward, initial verification_
