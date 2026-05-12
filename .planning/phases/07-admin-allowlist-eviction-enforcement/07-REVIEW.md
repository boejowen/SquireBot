---
phase: 07-admin-allowlist-eviction-enforcement
reviewed: 2026-05-11T23:30:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - apps-script/src/lib/admin.ts
  - apps-script/src/triggers/showAdminMgmtSidebar.ts
  - apps-script/src/triggers/showEvictionSidebar.ts
  - apps-script/src/triggers/onOpen.ts
  - apps-script/src/Code.ts
  - apps-script/build.mjs
  - apps-script/appsscript.json
  - apps-script/src/__tests__/admin.test.ts
  - apps-script/src/__tests__/adminMgmtSidebar.test.ts
  - apps-script/src/__tests__/showEvictionSidebar.test.ts
  - apps-script/src/__tests__/test-helpers.ts
findings:
  blocker: 0
  warning: 5
  info: 6
  total: 11
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-05-11T23:30:00Z
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

Phase 7 ships an admin-policy module (`lib/admin.ts`), an admin-management
sidebar, admin-gating on the existing eviction sidebar, lazy onOpen
bootstrap, two new menu items, and the `userinfo.email` OAuth scope
addition. The locked decisions (D-01 onOpen-no-throw, D-04 owner-floor
self-removal, D-05 lock envelope, D-06 dual-policy caller identity) are
correctly implemented at the lib layer and well-tested (T1–T20 + TS1–TS7).

No blockers found — the security-critical paths (auth-before-write,
fail-closed empty Session, lock envelopes, server-side identity sourcing)
are sound. However, **the admin-mgmt sidebar's HTML interpolation has a
quote-escape gap that allows an authenticated admin to inject HTML
attributes into another admin's view via a crafted email address**
(WR-01) — that is the most material finding. Several smaller defects
around dead code, test gaps in `bootstrapGuildAdminsManual`, and an
exported-but-fragile `appendAdminLogEntry` are documented below.

## Warnings

### WR-01: `escapeHtml` in admin-mgmt sidebar does not escape quote characters — admin-to-admin HTML-attribute injection

**File:** `apps-script/src/triggers/showAdminMgmtSidebar.ts:192,217,219`
**Issue:** The inline `escapeHtml` helper (line 192) routes input through
`document.createElement('div').textContent = s; return d.innerHTML`. Per
HTML serialization spec, this escapes `<`, `>`, `&`, and U+00A0 — but
NOT `"` or `'`. Lines 217 and 219 then interpolate the (escaped) email
into HTML attribute contexts:

```javascript
'<button class="remove-btn" aria-label="Remove admin ' + escapeHtml(email) +
  '" data-email="' + escapeHtml(email) + '">Remove</button>'
```

Server-side `addAdmin` only validates that the email is non-empty and
contains `'@'` (admin.ts:147) — it accepts `"`, `>`, and event-handler
syntax. An admin can therefore call `addAdmin('a"@x onmouseover=alert(1) "@x.com')`,
and when a different admin opens the sidebar, the unescaped `"` breaks
out of the `data-email`/`aria-label` attribute and injects an
`onmouseover` handler that runs in the sidebar's caja-sandboxed context.

Threat model is bounded (insider-only — attacker must already be admin)
but the new code claims `T-07-02-03 XSS hardening` (line 157) and the
phase context explicitly requested verification of `escapeHtml usage on
every interpolation`. The hardening is incomplete.

**Fix:** Replace the textContent-based helper with one that also escapes
quotes, or emit attribute values via DOM API instead of string
concatenation. Minimum fix:

```javascript
function escapeAttr(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
// then at lines 217/219:
'<button class="remove-btn" aria-label="Remove admin ' + escapeAttr(email) +
  '" data-email="' + escapeAttr(email) + '">Remove</button>'
```

Alternative: build the `<li>` and `<button>` via `document.createElement`
+ `setAttribute`, which never has this hazard. Server-side belt-and-suspenders:
add a stricter regex check in `admin.ts addAdmin` (e.g. reject any email
containing `"`, `<`, `>`, whitespace, or control chars).

The same `escapeHtml` is cloned from `showEvictionSidebar.ts:300`, but
the eviction sidebar only uses it in element-text contexts (`<li>` body,
`<div>` body) — quote-escape is moot there. The admin sidebar's
attribute-context use is the new exposure.

---

### WR-02: `appendAdminLogEntry` is exported but documented as caller-must-hold-lock — unsafe public surface

**File:** `apps-script/src/lib/admin.ts:120-134`
**Issue:** `appendAdminLogEntry` is a top-level `export function` and the
docstring (line 117-119) explicitly states `NOT lock-wrapped on its own
— caller is responsible for holding the lock`. There is no compile-time
or runtime mechanism to enforce that contract. Any future caller (or
future test, or future debug-from-script-editor invocation) who calls
`appendAdminLogEntry({...})` outside a lock envelope will silently
race-corrupt `_meta.admin_log` (lost log entries) under concurrent
admin operations.

The function is only meant to be called from the three lock-wrapped
mutators in the same file (`addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`).
None of those callers need it to be exported.

**Fix:** Drop the `export` keyword — make it module-private. Or, if
external testability is the reason for the export (which T20 in
admin.test.ts confirms), keep it exported but rename to
`_unsafeAppendAdminLogEntry` and add a runtime assertion that the
document lock is currently held. Cheapest fix is just removing
`export`; T20 would need to be rewritten to drive the entry through
`addAdmin` instead.

---

### WR-03: `bootstrapGuildAdminsManual` has zero test coverage and depends on `getActiveSpreadsheet().toast()` which the mock does not stub

**File:** `apps-script/src/lib/admin.ts:317-357` (subject); `apps-script/src/__tests__/test-helpers.ts:227-236` (missing mock)
**Issue:** `bootstrapGuildAdminsManual` is a brand-new exported entry
point (registered as the menu item "Initialize Admin Allowlist (manual)")
with three execution paths: empty-Session early-return, OK_CANCEL
cancellation, and successful bootstrap. It calls
`getActiveSpreadsheet().toast(...)` on three branches (lines 349, 351,
353-355). The test-helpers `getActiveSpreadsheet` mock (test-helpers.ts:227)
exposes `getSheetByName`, `insertSheet`, and `getSheets` only — no
`toast` method. Any test that exercised the success path would crash
with `toast is not a function`.

`admin.test.ts` covers `bootstrapGuildAdmins` (T15–T19) but not the
`Manual` wrapper at all. This means: (a) the user-facing menu item is
shipped untested, (b) the toast copy strings are unverified, (c) the
OK_CANCEL branch (which re-uses the `state.alertCalls` capture for two
distinct alert types in one flow) is unverified.

**Fix:** Add three tests in `admin.test.ts`:
1. Empty Session → alerts "Could not determine your email" + no writes.
2. OK_CANCEL CANCEL → no writes, info-level "cancelled" log emitted.
3. Happy path → calls `bootstrapGuildAdmins` with `seedEmail`,
   `initiatedBy='manual_fallback'` + writes.

To make the happy-path test runnable, add a `toast` stub to the
SpreadsheetApp proxy in `test-helpers.ts:227-236`:
```typescript
getActiveSpreadsheet: () => ({
  // ...existing methods...
  toast: (msg: string) => {
    (state as MockState & { toastCalls?: string[] }).toastCalls ??= [];
    (state as MockState & { toastCalls?: string[] }).toastCalls!.push(msg);
  },
}),
```

---

### WR-04: TOCTOU window between `requireAdminOrThrow` and `lock.tryLock` in `addAdmin`/`removeAdmin`

**File:** `apps-script/src/lib/admin.ts:145+151,185+191`
**Issue:** Both mutators do `requireAdminOrThrow(callerEmail)` (which
performs a full `_meta` read) BEFORE calling `lock.tryLock(30000)`.
Between those two lines, another concurrent admin operation could remove
the caller's admin status. The mutator then proceeds to write under the
lock with stale authorization.

Practical impact is small — the `requireAdminOrThrow` check is
defense-in-depth (the sidebar wrapper already gated the same check),
and the worst case is "an admin who was JUST evicted gets one final
write through". But for an authorization module whose entire point is
fail-closed correctness, this window is worth closing.

Additionally, when admin status DID change in that window, the code
silently proceeds rather than re-checking. The owner-floor protection
inside the locked block (line 200) does re-read the meta, but the admin
gate does not.

**Fix:** Move the `requireAdminOrThrow(callerEmail)` call INSIDE the
`try` block, AFTER the lock has been acquired:

```typescript
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) throw new Error('addAdmin: lock_busy');
try {
  requireAdminOrThrow(callerEmail);  // re-check under the lock
  const target = normalizeEmail(email);
  if (!target || target.indexOf('@') === -1) throw new Error('invalid_email');
  // ... rest unchanged
}
```

Same shape for `removeAdmin`. Note: input validation (`invalid_email`)
can stay outside the lock; only the auth check needs to be inside.

---

### WR-05: `removeAdmin` accepts non-email targets without validation — silent `notFound` instead of `invalid_email`

**File:** `apps-script/src/lib/admin.ts:181-189`
**Issue:** `addAdmin` requires `target` to contain `'@'` (line 147,
throws `invalid_email`). `removeAdmin` only requires `target` to be
non-empty (line 187) and silently returns `{removed: false, notFound: true}`
for any garbage input (e.g. `removeAdmin('not-an-email', 'admin@x.com')`
returns `notFound`).

This asymmetry hides input bugs. A buggy client passing a malformed
email gets a `success` toast ("Not found in list") rather than the
correct `error` toast. The audit trail also shows nothing — no log
entry for the rejected garbage. If the client is ever refactored to
trust `notFound` as "operation completed cleanly", a typo could be
mistaken for a successful removal.

**Fix:** Mirror the `addAdmin` validation at line 188:
```typescript
if (!target || target.indexOf('@') === -1) {
  throw new Error('invalid_email');
}
```

The sidebar's existing `routeError` already maps `/invalid_email/` to
the right copy (showAdminMgmtSidebar.ts:199), so no client-side change
is needed.

## Info

### IN-01: Dead code — `resolveInitiatedBy()` fallback in `addAdmin`/`removeAdmin` is unreachable

**File:** `apps-script/src/lib/admin.ts:167,228`
**Issue:** `initiated_by: normalizeEmail(callerEmail) || resolveInitiatedBy()`
appears in both mutators. By the time those lines execute,
`requireAdminOrThrow(callerEmail)` has already returned (lines 145, 185),
which guarantees `normalizeEmail(callerEmail)` is non-empty (otherwise
`requireAdminOrThrow` would have thrown). The `|| resolveInitiatedBy()`
branch is therefore unreachable.

Not incorrect — just confusing. A reader might think `resolveInitiatedBy`
is needed here for the audit-log soft-fallback (D-06), but D-06 only
applies to call sites that DO NOT first call `requireAdminOrThrow`.
`bootstrapGuildAdmins` is the legitimate D-06 audit-log path.

**Fix:** Drop the `|| resolveInitiatedBy()` from both mutators:
```typescript
initiated_by: normalizeEmail(callerEmail),
```
And add a comment that the fallback is intentionally absent because
`requireAdminOrThrow` already guarantees a non-empty caller.

### IN-02: `getActiveTheme` + `THEMES` interpolated directly into `<style>` block — relies on registry being trusted

**File:** `apps-script/src/triggers/showAdminMgmtSidebar.ts:117-133`; `apps-script/src/triggers/showEvictionSidebar.ts:234-250` (pre-existing)
**Issue:** `themeStyleBlock` interpolates `theme.headerBg`, `theme.rowAltBg`,
etc. directly into a `<style>` tag without escaping. If a theme value
ever contained `</style><script>...</script>`, it would break out and
execute. Today the THEMES registry (`apps-script/src/lib/themes.ts`) is
hardcoded module-private, so this is safe. But the pattern is fragile —
any future "user-customizable themes" feature would need to remember to
sanitize first.

**Fix:** Either (a) document at the top of `themeStyleBlock` that theme
values MUST be source-controlled and never user-supplied, or (b)
preemptively pass each value through a CSS-token whitelist
(`/^[#a-zA-Z0-9 ,()'.-]+$/`).

This is a clone of an existing Phase 5 pattern — flagging here for
visibility, not as a Phase-7-specific defect.

### IN-03: `getEvictionEmails` does not lowercase emails before deduplication / comparison

**File:** `apps-script/src/triggers/showEvictionSidebar.ts:97-100,130,174`
**Issue:** Pre-existing bug, not Phase 7's, but newly visible because the
email-comparison path is now wrapped by Phase-7-added admin guards. The
function `String(r[COL_OWNER_EMAIL - 1] ?? '').trim()` does not lowercase.
If `_char_owner` has both `Officer@Guild.com` and `officer@guild.com`,
the deduped set keeps both; the dropdown shows two entries; choosing one
will only evict one casing's chars. `previewEviction` and `commitEviction`
likewise compare with `!== email` — case-sensitive.

The new admin module (`admin.ts`) consistently lowercases at three
points (read, write, compare). The eviction sidebar should do the same
for consistency.

**Fix:** Apply `normalizeEmail` (now exported from `lib/admin`) at lines
97, 130, 174. Phase 7 is the right time because admin.ts already exports
the helper.

### IN-04: `bootstrapGuildAdmins` writes `bootstrap_failed` audit entry for `owner_null` but NOT for `lock_busy`

**File:** `apps-script/src/lib/admin.ts:255-258 vs. 281-291`
**Issue:** The `owner_null` failure mode writes a `bootstrap_failed`
entry to `_meta.admin_log` (lines 282-289). The `lock_busy` failure mode
only emits a server-side `log('warn', ...)` (line 257) and returns —
no `admin_log` entry. This is consistent (lock_busy means the lock is
held, so `appendAdminLogEntry` couldn't safely write anyway) but it
means operators cannot see lock-busy bootstraps in the workbook's own
audit trail.

Consequence: if onOpen-bootstrap silently fails on lock_busy for many
sessions in a row, there is no in-workbook signal — only Stackdriver
logs. Acceptable but worth documenting.

**Fix:** Add a comment at line 257 explicitly noting the asymmetry, e.g.
`// NOTE: cannot write a bootstrap_failed audit entry here — we don't hold the lock. Stackdriver log only.`

### IN-05: `bootstrapGuildAdmins` doc comment promises both server-side log AND audit-log entry on `owner_null`, but only writes one

**File:** `apps-script/src/lib/admin.ts:248-249,281-291`
**Issue:** Docstring says `On getOwner() returning null: writes a 'bootstrap_failed' entry to admin_log + warn log`. Line 282-289 does
write the audit entry. But `appendAdminLogEntry` itself reads `_meta` and
calls `writeMetaRow`. The `bootstrap_failed` audit entry write happens
INSIDE the lock (line 254-307) — which is fine — but if `appendAdminLogEntry`
itself throws (e.g. `_meta` sheet was deleted between the bootstrap
check and the failed-entry write), the throw will propagate out of the
`try` block and through `lock.releaseLock()` in `finally`. That throw
would then bubble to `onOpen`, which catches it (good) — but the
`{bootstrapped: false, reason: 'owner_null'}` return is lost.

Low impact. Mostly worth noting because the docstring promises a clean
return and the actual code can throw on that path.

**Fix:** Wrap the `bootstrap_failed` audit-log write in its own
try/catch:
```typescript
if (!seed) {
  log('warn', 'bootstrapGuildAdmins', { reason: 'owner_null' });
  try {
    appendAdminLogEntry({ ... });
  } catch (e) {
    log('warn', 'bootstrapGuildAdmins', { auditWriteFailed: String(e) });
  }
  return { bootstrapped: false, reason: 'owner_null' };
}
```

### IN-06: Stale comment in `showAdminMgmtSidebar.ts` references wrong line number in eviction sidebar

**File:** `apps-script/src/triggers/showAdminMgmtSidebar.ts:18,157`
**Issue:** Comments say "cloned verbatim from showEvictionSidebar.ts:257"
(line 18) and "verbatim from showEvictionSidebar.ts:257" (line 157).
The actual escapeHtml in showEvictionSidebar.ts is on **line 300**, not
257. Off by 43 lines. Likely a stale reference from an earlier version
of the eviction sidebar.

**Fix:** Update both comments to `showEvictionSidebar.ts:300`.

---

_Reviewed: 2026-05-11T23:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
