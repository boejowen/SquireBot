---
phase: 07-admin-allowlist-eviction-enforcement
plan: 03
artifact: smoke-evidence
date: 2026-05-12
operator: boejowen@gmail.com (workbook owner)
verdict: SHIPPED (5/5 verification hooks PASS)
script_id: 1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT
tags: [apps-script, eviction, admin-allowlist, clasp-push, smoke, ship-evidence]
---

# Phase 7 Plan 03 — Dev-Workbook Smoke Evidence

**Date:** 2026-05-12
**Operator:** workbook owner (boejowen@gmail.com)
**Bundle:** `apps-script/dist/Code.js` + `apps-script/dist/appsscript.json`
**Dev workbook script ID:** `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT`
**Verdict:** **SHIPPED** — all 5 verification hooks from 07-CONTEXT.md §verification_hooks PASS.

---

## Step 1 — Build + clasp push

```text
$ cd apps-script
$ npm test           # 324/324 PASS (baseline + admin.test.ts + adminMgmtSidebar.test.ts + reseeded showEvictionSidebar.test.ts)
$ npm run build      # dist/Code.js fresh; sync assertion (TRIGGER_GLOBALS ↔ Code.ts) PASS
$ npx clasp push --force
└─ pushed 2 files: dist/Code.js, dist/appsscript.json
```

**Push composition:** the bundle deployed during the smoke includes the OAuth-scope fix from commit `544bef8` (see Deviations §1 below). Two `clasp push` invocations occurred during the smoke session — the first deployed the original Phase 7 bundle (commits `1937fd0` + `7f7ffb0` + `c3c0033`), Hook 3 surfaced the latent `userinfo.email` scope gap, fix landed as `544bef8`, second `clasp push --force` redeployed the patched bundle, smoke resumed and PASSED.

---

## Step 2 — 5-Hook Verification Matrix

| Hook | What | Status | Evidence |
|------|------|--------|----------|
| 1 | `_meta.guild_admins` exists after first open + idempotent re-open | **PASS** | After first open, `_meta.guild_admins` cell B = `["boejowen@gmail.com"]`; `_meta.workbook_owner_floor` cell B = `boejowen@gmail.com`; `_meta.admin_log` cell B contained one entry: `[{"action":"bootstrap","email":"boejowen@gmail.com","initiated_by":"onOpen","at":"<ISO>"}]`. Second open of the same workbook tab did NOT append a new bootstrap entry to `admin_log` — the three rows remained byte-for-byte unchanged. (D-01 idempotent already-initialized check verified live.) |
| 2 | Non-admin sees alert + zero eviction writes | **PASS** | Owner temporarily edited `_meta.guild_admins` cell to `["someone-else@example.com"]` (a value excluding the active caller). After page refresh, clicked `SquireBot → Evict Guildie…`. The Apps Script modal alert appeared with title "Not authorized" and body containing "Only guild officers can evict members. Contact a workbook admin if you think this is wrong." Clicked OK; the modal dismissed and NO sidebar opened. Inspected `_char_owner` — no `is_removed` flips. Inspected `_meta.eviction_log` — no new entry. Inspected `_meta.admin_log` — no new entry. Restored `_meta.guild_admins` to `["boejowen@gmail.com"]`. (D-03 modal UX + D-05 server-side fail-closed both verified.) |
| 3 | Admin adds peer + peer's eviction sidebar opens | **PASS (after OAuth scope fix — see Deviations §1)** | Owner opened `SquireBot → Manage Admins…` and added `joseph.bowen2@gmail.com` via the sidebar's add input + button. Status region confirmed `Admin added: joseph.bowen2@gmail.com.` Owner separately shared the workbook with that address as **Editor** in Google Drive (sharing is a Google permission distinct from SquireBot's policy list — the sidebar grants admin status, not workbook access). joseph.bowen2 signed in via a separate browser session, refreshed the workbook → SquireBot menu appeared. joseph.bowen2 clicked `SquireBot → Evict Guildie…` → Google OAuth consent dialog appeared (first-time auth for that account, including the new `userinfo.email` scope), they clicked Allow → eviction sidebar opened normally on the right-hand side and the email selector populated with `_char_owner` rows. (No `Not authorized` alert path triggered for the now-admin caller.) |
| 4 | Owner-floor cannot be removed by non-floor admin | **PASS** | With joseph.bowen2 (a non-floor admin) signed in, they clicked `SquireBot → Manage Admins…`. The sidebar rendered both rows: the boejowen@gmail.com row had the `(owner)` annotation and **no Remove button** (client-side suppression because `callerEmail !== floor`); the joseph.bowen2 row had a Remove button (their own row). Client-side enforcement of D-04 owner-floor lockout verified visually. Defense-in-depth server check from admin.test.ts T18 covers the `removeAdmin(floor, non_floor)` → `'owner_floor_protected'` throw path; that path is exercised in unit tests and was not invoked live during this smoke. |
| 5 | `_meta.schema_version=3` + `WatcherMaxSchemaVersion=3` (grep gates) | **PASS** | `Select-String apps-script/src/lib/migrations.ts -Pattern "writeMetaRow\('_meta', 'schema_version', '3'\)"` matches exactly 1 line (unchanged from baseline). `Select-String internal/sheet/client.go -Pattern "WatcherMaxSchemaVersion\s*=\s*3"` matches exactly 1 line (unchanged from baseline). Neither Phase 7 source file (showEvictionSidebar.ts, onOpen.ts, showAdminMgmtSidebar.ts, lib/admin.ts) references `schema_version` or `WatcherMaxSchemaVersion`. Zero schema impact, zero watcher rebuild required. |

---

## Step 3 — Deviations

### 1. (Significant) Latent v1.0 OAuth-scope bug surfaced and fixed mid-smoke

**Discovered:** Hook 3, on joseph.bowen2's first invocation of `Evict Guildie…` after being added as admin.

**Symptom:** Despite being correctly added to `_meta.guild_admins`, joseph.bowen2 saw the "Not authorized — Only guild officers can evict members" modal alert when clicking the eviction menu item. Same symptom applied to `Manage Admins…`. The admin guards (`isAdmin(callerEmail)` → false → alert) fired even though the caller was a legitimate admin.

**Root cause:** The original Apps Script manifest (`apps-script/appsscript.json`) declared 4 oauthScopes — `spreadsheets.currentonly`, `script.external_request`, `script.scriptapp`, `script.container.ui`. None of these grant access to user email. For consumer @gmail.com accounts, `Session.getEffectiveUser().getEmail()` silently returns `""` (empty string) without `https://www.googleapis.com/auth/userinfo.email` declared in the manifest. The Phase 7 admin guards correctly fail-closed on empty caller email per D-06 (`requireAdminOrThrow('')` → `not_authorized`), surfacing the latent v1.0 manifest gap immediately.

**Pre-existing latent bug retired:** the eviction sidebar's `initiated_by` audit-log field has been silently writing `'unknown'` (the D-06 soft fallback) for every eviction action across the entire v1.0 release because of this same scope omission. This Phase 7 fix retires that bug as a side effect.

**Fix shipped:** commit `544bef8` (`fix(07-03): add userinfo.email OAuth scope to Apps Script manifest`) added the `https://www.googleapis.com/auth/userinfo.email` scope to `apps-script/appsscript.json`. Bundle rebuilt; force-pushed via `npx clasp push --force`. User re-authorized at next sidebar invocation (Google's OAuth re-consent dialog fired naturally because the scope set changed). Hook 3 then proceeded normally.

**Authorization classification:** the `userinfo.email` scope is **non-sensitive** per Google's OAuth scope classification — no Production consent screen change required, no Google verification audit triggered, no impact on the existing `drive.file` scope's non-sensitive status.

**Threat-model alignment:** consistent with T-07-03-01 (server-side requireAdminOrThrow on every callback) — the threat register correctly documented that `Session.getEffectiveUser().getEmail()` is the load-bearing identity source, but did not enumerate the manifest dependency. The fix closes the gap; no follow-up threat surface introduced.

**Rule classification:** this is a **Rule 3 deviation** (auto-fix blocking issue — without the scope, every Phase 7 admin-protected action fails closed for every guildie). Single-commit inline fix; no architectural change; threat surface unchanged (admin guard still fail-closes; identity source still server-side `Session.getEffectiveUser`).

### 2. (Minor, no action) Hook 4 server-side path covered by unit tests, not invoked live

The defense-in-depth server-side `removeAdmin(floor, non_floor)` → `'owner_floor_protected'` throw is not invoked in the live smoke (would require devtools console-level google.script.run injection from joseph.bowen2's session). The path is covered by admin.test.ts T18 + adminMgmtSidebar.test.ts TS6, both PASS in the 324/324 suite. Live smoke confirms client-side suppression (no Remove button visible to non-floor admin); unit tests cover server-side rejection. Combined evidence covers ADMIN-03 owner-floor lockout fully.

---

## Step 4 — Plan 03 Commit History

| Hash | Message | Files |
|------|---------|-------|
| `1937fd0` | test(07-03): seed _meta.guild_admins in showEvictionSidebar tests | `apps-script/src/__tests__/showEvictionSidebar.test.ts` |
| `7f7ffb0` | feat(07-03): admin-guard showEvictionSidebar opener + 3 callbacks | `apps-script/src/triggers/showEvictionSidebar.ts` (+ test reseed for Test 12 reframe) |
| `c3c0033` | feat(07-03): wire onOpen lazy bootstrap + Manage Admins menu items | `apps-script/src/triggers/onOpen.ts` |
| `2d03581` | docs(07-03): interim SUMMARY + STATE + ROADMAP — code-complete pending smoke | `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md` + `.planning/STATE.md` + `.planning/ROADMAP.md` |
| `544bef8` | fix(07-03): add userinfo.email OAuth scope to Apps Script manifest | `apps-script/appsscript.json` (+ rebuilt `dist/appsscript.json`) |

---

## Step 5 — Phase 7 Ship Verdict

**Phase 7 = SHIPPED + UAT-VERIFIED 2026-05-12.**

- ADMIN-01 closed (Plans 01 + 03): bootstrap primitive shipped in Plan 01; lazy `onOpen` call-site shipped in Plan 03; Hook 1 PASS confirms live bootstrap + idempotent re-open.
- ADMIN-02 closed (Plan 03): eviction sidebar admin-gated at opener + 3 callbacks; Hook 2 PASS confirms non-admin path is a safe no-op.
- ADMIN-03 closed (Plans 01 + 02 + 03): admin-mgmt sidebar UX shipped in Plan 02; owner-floor enforcement shipped in Plan 01 (`removeAdmin`) + reinforced client-side in Plan 02 sidebar; Hook 3 + Hook 4 PASS confirm peer-add round-trip + visual owner-floor lockout.
- Schema impact: zero (Hook 5 PASS — `schema_version=3`, `WatcherMaxSchemaVersion=3` both untouched).
- Bundle deployed: `dist/Code.js` + `dist/appsscript.json` live on dev workbook script ID `1Y9Uiw-QWgLQRIKGnQxmXKoi2oUwekk1CbUQfjO6jjNloXXz0QPn3YwCT` as of 2026-05-12.

**Cleanup performed:** test admin `joseph.bowen2@gmail.com` retained in `_meta.guild_admins` per operator preference (real second-officer account, not a synthetic test fixture). `_meta.eviction_log` and `_char_owner` unchanged from pre-smoke state (no eviction was committed during the smoke — Hook 3 stopped at "sidebar opens normally" without exercising the destructive workflow).
