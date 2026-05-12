---
phase: 07-admin-allowlist-eviction-enforcement
fixed_at: 2026-05-12T04:57:00Z
review_path: .planning/phases/07-admin-allowlist-eviction-enforcement/07-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 7: Code Review Fix Report

**Fixed at:** 2026-05-12T04:57:00Z
**Source review:** .planning/phases/07-admin-allowlist-eviction-enforcement/07-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (all warnings — no blockers; info findings out of scope)
- Fixed: 5
- Skipped: 0

All five WR-* findings were fixed and committed atomically. The full
apps-script test suite (327 tests across 30 files) passes after each
fix. Verification hooks 1–4 (admin policy module behavior) and hook 5
(`schema_version` unchanged) remain intact; no source code outside
`apps-script/src/lib/admin.ts`,
`apps-script/src/triggers/showAdminMgmtSidebar.ts`, and the two test
files was modified.

## Fixed Issues

### WR-01: `escapeHtml` in admin-mgmt sidebar does not escape quote characters — admin-to-admin HTML-attribute injection

**Files modified:** `apps-script/src/triggers/showAdminMgmtSidebar.ts`, `apps-script/src/lib/admin.ts`
**Commit:** `cbf6f2d`
**Applied fix:** Two-layer defense in depth.
- Sidebar: added new `escapeAttr()` helper that escapes `&`, `<`, `>`,
  `"`, `'`. Switched the two attribute-context interpolations
  (`data-email`, `aria-label`) on what is now lines 224–225 to use it.
  Kept `escapeHtml` for element-text contexts (the `<span>` body on
  line 226).
- Server: tightened `addAdmin`'s validation from `target.indexOf('@') !== -1`
  to a conservative RFC 5321 subset regex
  (`/^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$/i`). Now rejects emails
  containing `"`, `<`, `>`, whitespace, or control chars before they
  ever reach `_meta` or another admin's sidebar.

### WR-02: `appendAdminLogEntry` is exported but documented as caller-must-hold-lock — unsafe public surface

**Files modified:** `apps-script/src/lib/admin.ts`, `apps-script/src/__tests__/admin.test.ts`
**Commit:** `f10669f`
**Applied fix:** Dropped the `export` keyword on `appendAdminLogEntry`
and updated its docstring to record why (the type system can't enforce
the "caller must hold the lock" contract). Verified no other
production module imports the helper. Rewrote T20 to drive the
malformed-existing-log recovery branch through `addAdmin` instead of
calling the now-private helper directly.

### WR-03: `bootstrapGuildAdminsManual` has zero test coverage and depends on `getActiveSpreadsheet().toast()` which the mock does not stub

**Files modified:** `apps-script/src/__tests__/test-helpers.ts`, `apps-script/src/__tests__/admin.test.ts`
**Commit:** `6e32cb3`
**Applied fix:** Stubbed `Spreadsheet.toast(msg)` on the
`SpreadsheetApp.getActiveSpreadsheet()` proxy and capture every call
into a new `state.toastCalls: string[]`. Added `state.alertReturn`
override so tests can simulate a CANCEL response from an OK_CANCEL
dialog (default behavior — return `'OK'` — preserved). Added three
new test cases to `admin.test.ts`:
- T21: empty Session — alerts "Could not determine your email", no
  writes, no toasts, info log emits `skipped: 'session_email_empty'`.
- T22: user clicks Cancel — single OK_CANCEL alert, no writes, no
  toasts, info log emits `cancelled: true`.
- T23: happy path — OK_CANCEL alert shown, `_meta.guild_admins`+
  `workbook_owner_floor` written, `admin_log` entry with
  `initiated_by: 'manual_fallback'`, success toast emitted.

### WR-04: TOCTOU window between `requireAdminOrThrow` and `lock.tryLock` in `addAdmin`/`removeAdmin`

**File modified:** `apps-script/src/lib/admin.ts`
**Commit:** `0711ae3`
**Applied fix:** Moved the `requireAdminOrThrow(callerEmail)` call
INSIDE the `try` block in both `addAdmin` and `removeAdmin`, after
`lock.tryLock(30000)` succeeds. Input validation
(`invalid_email`) stays outside the lock — no point holding it for a
guaranteed-throw. The owner-floor protection inside `removeAdmin`
already re-read `_meta` under the lock for the same reason; the admin
gate now does too. Added inline comments referencing WR-04.

### WR-05: `removeAdmin` accepts non-email targets without validation — silent `notFound` instead of `invalid_email`

**File modified:** `apps-script/src/lib/admin.ts`
**Commit:** `a91b62b`
**Applied fix:** Mirrored `addAdmin`'s `@`-validation in `removeAdmin`:
`if (!target || target.indexOf('@') === -1) throw new Error('invalid_email');`.
Added an inline comment documenting the symmetry rationale. The
sidebar's existing `routeError` already maps `/invalid_email/` to the
right copy, so no client-side change is needed.

---

## Verification

After every fix:
- `npx tsc --noEmit` clean (no new TS errors)
- `npx vitest run` all 327 tests pass across 30 files (including the
  3 new T21–T23 tests added in WR-03)

The five Phase-7 verification hooks remain intact:
- D-01 (onOpen never throws) — `bootstrapGuildAdmins` unchanged on the
  `lock_busy` swallow path
- D-04 (owner-floor lockout) — `removeAdmin`'s floor check still
  inside the lock, throws `owner_floor_protected` for non-floor
  callers (test T12 / TS5 still pass)
- D-05 (LockService envelope on every multi-step _meta write) —
  every mutator still wraps in `tryLock(30000)`/`releaseLock()`
- D-06 (dual-policy caller identity) — auth path
  (`requireAdminOrThrow`) still fail-closes on empty Session;
  audit-log path (`resolveInitiatedBy`) still soft-falls-back to
  `'unknown'` for non-authorizing callers (`bootstrapGuildAdmins`)

## Skipped Issues

None — all 5 in-scope warnings were fixed.

The 6 IN-* (info) findings are out of scope for this fix pass per
`fix_scope: critical_warning`. They remain documented in
`07-REVIEW.md` for a future polish pass.

---

_Fixed: 2026-05-12T04:57:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
