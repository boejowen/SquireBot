---
phase: 15-admin-web-forms-login
verified: 2026-05-31T04:30:00Z
status: human_needed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification:
  - test: "Live Discord OAuth2 login smoke against the real Discord application"
    expected: "A guild Discord-server member signs in and lands on the app home with a session; a non-member is bounced to /?not_member=1 with NO session cookie set"
    why_human: "Requires real DISCORD_CLIENT_ID/SECRET/GUILD_ID set as systemd Environment on the Hetzner box — explicitly deferred to the deploy step by the user-locked build-only directive (the 15-02 checkpoint was pre-resolved 'build-only'). The OAuth/membership/session logic is fully proven locally against httptest fakes (TestCallbackHandler_NonMember_NoSession_RedirectNotMember, TestCallbackHandler_Member_MintsSession_RedirectHome, TestIsGuildMember_Table incl. the refusal + fail-closed cases)."
  - test: "Run goose up (00004_web_auth.sql) against the live VPS SQLite database"
    expected: "goose applies 00001→00004 cleanly on the production DB; web_user/web_session/guild_admins/app_config tables + the coin/grace/archive/audit columns exist; a second startup is a no-op"
    why_human: "Production DB migration is a deploy-time action (deploy = drop binary + restart; goose.Up runs on startup). The migration + idempotent re-run are proven locally in the migrations package tests (ok, uncached)."
  - test: "Seed the owner-floor on the box: squirebot-server set-owner-floor <maintainer-discord-id>"
    expected: "app_config['owner_floor_discord_id'] + a guild_admins row for the maintainer exist; the maintainer is the bootstrap officer and shows in ListOfficers"
    why_human: "One-time deploy seed with the real maintainer Discord snowflake. The CLI + store.SetOwnerFloor are proven locally (TestRun_SetOwnerFloorDispatch, TestSetOwnerFloor_SeedsConfigAndBootstrapOfficer)."
  - test: "Live evict / restore / coin / officer-add-remove smokes against production data"
    expected: "An officer evicts a guildie (is_removed cascade + guild_code revoked + 30-day grace), restore re-mints a code during grace, a non-officer 403s on admin endpoints, any member records bank coin, a peer cannot remove/evict the owner-floor"
    why_human: "Live destructive write verification against real guild data is a deploy-time human step. Every enforcement rule is proven locally by uncached httptest + temp-DB tests (TestEvict_*, TestRestore_*, TestOfficer*, TestCoinSet_NonOfficerCanWrite, TestWriteRoutes_Gates)."
  - test: "Visual / interaction QA of the login flow + the three forms in a browser"
    expected: "LoginScreen, NotMemberScreen, SessionIndicator, BankCoinForm, EvictionForm (with ConfirmDialog), AdminMgmtForm render with the EQ theme, the exact copy, and the accessible confirm/focus behavior"
    why_human: "Visual appearance + real focus-trap/keyboard interaction cannot be verified programmatically (the repo has no jsdom/@testing-library by deliberate decision). The a11y/markup contracts (role=dialog, aria-modal, Cancel-focused, Esc dismiss) and copy are proven by source-inspection + pure-helper unit tests (ConfirmDialog.test.ts, coin.test.ts, adminHelpers.test.ts); the rendered experience still wants a human eye."
---

# Phase 15: Admin Web Forms + Login Verification Report

**Phase Goal:** Add authenticated human access and the officer-only write actions to the website — Discord OAuth2 login gated on guild Discord-server membership (which also captures per-user Discord identity), plus eviction, bank-coin entry, and admin/officer management as authenticated web forms that port the v1 enforcement rules.
**Verified:** 2026-05-31T04:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Scope note

This phase was executed under a user-locked **build + verify LOCALLY only** directive. The live Discord-login smoke, the production DB migration, the owner-floor seed, and the live evict/coin/officer smokes are INTENTIONALLY DEFERRED deploy-time steps and are recorded as human-verification items (not gaps). The verification target is: the OAuth/session/membership logic, the write-form backends + enforcement, and the frontend gate + forms — all proven by the local Go + web test suites. All five requirements' logic is present and locally test-proven, so each is SATISFIED for this build-and-verify scope. Status is `human_needed` solely because of the deferred deploy-time + visual items.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | A visitor signs in with Discord OAuth2 and is admitted only if they are a guild Discord-server member; a non-member is refused, no hand-maintained allowlist (AUTH-08) | ✓ VERIFIED | `webauth/oauth.go` `IsGuildMember` fail-closed (empty config/list → false); `webauth/handlers.go` `CallbackHandler` verifies state→exchanges code server-side→fetches `/users/@me/guilds`→non-member redirects to `/?not_member=1` with NO session, member mints a fresh session. Tests (uncached): `TestIsGuildMember_Table` (incl. absent/empty/nil/empty-config refusal cases), `TestCallbackHandler_NonMember_NoSession_RedirectNotMember`, `TestCallbackHandler_Member_MintsSession_RedirectHome`. Read API walled: `main.go` wraps all 5 read routes in `RequireSession`; `TestReadRoutes_RequireSession_401` proves 401 per-route for all five. |
| 2 | Each signed-in user's Discord identity is captured + stored (AUTH-09 v2 prerequisite) | ✓ VERIFIED | `00004_web_auth.sql` `web_user(discord_user_id PK, username, avatar, first_seen, last_login)`; `webauth/oauth.go` `FetchIdentity` returns id/username/avatar; `handlers.go` calls `store.UpsertWebUser` on every member callback; `store/websession.go` `UpsertWebUser` uses `ON CONFLICT DO UPDATE` (idempotent). Frontend `SessionIndicator.svelte` surfaces avatar+username. Tests: `TestWhoamiWebHandler_AuthedShape`, store upsert covered. |
| 3 | Eviction is an authenticated officer-only form porting v1 enforcement (guild_admins gate + owner-floor lockout) + 30-day grace + archive; a non-officer cannot reach or fire it (ADMIN-04) | ✓ VERIFIED | `store/eviction.go` `EvictOwnerTx` cascades `is_removed=1` + `grace_until` AND revokes `guild_code.disabled_at` in one tx (D-10); `RestoreOwnerTx` reverses non-archived only; `ArchiveExpiredEvictions` idempotent (`archived_at IS NULL` guard). `webadmin/eviction.go` `EvictHandler` re-checks `store.IsOfficerTx` INSIDE the BEGIN IMMEDIATE tx (WR-04 TOCTOU close), owner-floor data protection before any write (WR-05 hardened: TRIM+COLLATE NOCASE + loud warn). Route wrapped in `RequireOfficer`. DAILY archive job uses `duePigparse` (not weekly — W-3). Tests: `TestEvict_OfficerCascadesAndRevokes_Audits`, `TestEvict_NonOfficerRejected_NothingChanged`, `TestEvict_PeerCannotEvictFloorData` (+case-insensitive), `TestRestore_DuringGrace_ReMintsCode_Audits`, `TestRestore_AfterArchive_GraceExpired` (409), `TestArchiveJob_Idempotent`, `TestWriteRoutes_Gates` (admin/evict member→403, anon→401). |
| 4 | Manual bank-coin entry (plat/gold/silver/copper) is an authenticated form persisting the four values (ADMIN-05) | ✓ VERIFIED | `00004` nullable `plat/gold/silver/copper` columns; `store/coin.go` `SetCoinTx` bank-toon-gated (`ErrNotBankToon`); `webadmin/coin.go` login-only with server-side range validation (plat≥0, g/s/c 0–999) + ZERO officer references (comment-stripped grep = 0, D-12). Route wrapped in `RequireSession` (NOT RequireOfficer). Frontend `BankCoinForm.svelte` (299 lines) `type="text" inputmode="numeric"` (CR-01 fix), surfaces in bank view via `fetchBankToons()`. Tests: `TestCoinSet_NonOfficerCanWrite` (D-12 — non-officer writes, columns change), `TestCoinSet_RejectsOutOfRange` (gold/silver/copper 1000, plat -1), `TestCoinSet_RejectsNonBankToon`, `TestCoinSet_SuccessPreFillsOnNextGet`. |
| 5 | Admin/officer management (guild_admins allowlist + owner-floor protection) is an authenticated form; the owner-floor equivalent cannot be removed by a peer admin (ADMIN-06) | ✓ VERIFIED | `store/admins.go` ports admin.ts verbatim: `IsOfficer` fail-closed, `AddOfficerTx`/`RemoveOfficerTx` re-check caller IsOfficer as FIRST in-tx statement (WR-04), owner-floor protection BEFORE any write (`ErrOwnerFloorProtected` when target==floor && caller!=floor), idempotent INSERT OR IGNORE / DELETE, self-removal-of-floor leaves the pointer (documented orphan). `SetOwnerFloor` seeds app_config + placeholder web_user + bootstrap officer. `webadmin/officers.go` audits real mutations only. Frontend `AdminMgmtForm.svelte` (410 lines) promote-by-pick + `showRemoveButton` floor suppression. Tests: `TestRemoveOfficerTx_OwnerFloorProtectedAndIdempotent`, `TestRemoveOfficerTx_SelfRemovalOfFloorLeavesPointer`, `TestOfficerRemove_PeerCannotRemoveFloor`, `TestOfficerAdd_*`, `TestSetOwnerFloor_SeedsConfigAndBootstrapOfficer`. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00004_web_auth.sql` | web_user/web_session/guild_admins/app_config + coin/grace/archive/audit cols | ✓ VERIFIED | Matches plan DDL exactly; migrations tests pass (fresh-DB Up + idempotent re-run) |
| `internal/backendsrv/store/websession.go` | session mint/hash/upsert/resolve | ✓ VERIFIED | sha256-only at rest; CreateSession/ResolveSession/TouchSession/DeleteSession query by `session_hash`; `TestCreateSession_StoresOnlyHashNotPlaintext` passes |
| `internal/backendsrv/store/admins.go` | officer/owner-floor port of admin.ts | ✓ VERIFIED | Full port; authorize-under-tx; owner-floor protection; idempotent |
| `internal/backendsrv/store/eviction.go` | cascade+revoke+grace+restore+archive | ✓ VERIFIED | EvictOwnerTx one-tx cascade+revoke; archive idempotent |
| `internal/backendsrv/store/coin.go` | bank-toon-gated coin write | ✓ VERIFIED | SetCoinTx fail-closed on non-bank-toon |
| `internal/backendsrv/webauth/oauth.go` | Discord OAuth2 + membership | ✓ VERIFIED | identify+guilds scopes, server-side exchange, fail-closed IsGuildMember, secret backend-only/never logged |
| `internal/backendsrv/webauth/session.go` | cookie + RequireSession/RequireOfficer | ✓ VERIFIED | HttpOnly+Secure+SameSite=Lax+Domain; fail-closed 401/403; WR-06 read-only resolve for whoami |
| `internal/backendsrv/webauth/handlers.go` | login/callback/whoami/logout | ✓ VERIFIED | state CSRF, regenerate-on-login, W-4 hardcoded-origin redirect (0 redirect-param reads) |
| `internal/backendsrv/webadmin/{eviction,coin,officers,audit}.go` | 3 write-form backends + audit | ✓ VERIFIED | authorize-under-tx via withTx (BEGIN IMMEDIATE + WR-03 deferred rollback); append-only audit_log (0 UPDATE/DELETE) |
| `internal/backendsrv/scheduler/scheduler.go` | eviction_archive DAILY job | ✓ VERIFIED | Due=duePigparse (daily); 0 dueWiki refs |
| `cmd/squirebot-server/main.go` | route wiring + gates | ✓ VERIFIED | 4 auth routes ungated; 5 read routes RequireSession; 7 admin routes RequireOfficer; 2 coin routes RequireSession |
| `cmd/squirebot-server/ownerfloor.go` | set-owner-floor CLI | ✓ VERIFIED | Full impl; missing-arg→exit 2; dispatch test passes |
| `web/src/lib/api.ts` | credentialed getJSON/postJSON + typed 401/403 | ✓ VERIFIED | credentials:'include' (×2), Unauthenticated/Forbidden with code, 9 admin/coin wrappers |
| `web/src/lib/auth.ts` | Session + fetchSession + resolveGate | ✓ VERIFIED | fail-safe fetchSession, pure classifyAuthError + resolveGate reducer |
| `web/src/lib/components/AuthGate.svelte` | whole-site gate + server-truth re-route | ✓ VERIFIED | fetchSession on mount, authGuard context, override-beats-session |
| `web/src/lib/components/{LoginScreen,NotMemberScreen,SessionIndicator,ConfirmDialog}.svelte` | login surfaces + a11y modal | ✓ VERIFIED | role=dialog/aria-modal/Cancel-focus/Esc; --destructive token ×5; 0 @html on user data |
| `web/src/lib/components/{BankCoinForm,EvictionForm,AdminMgmtForm}.svelte` | the 3 forms | ✓ VERIFIED | 299/358/410 lines; real wiring to the 9 endpoints + ConfirmDialog + authGuard; 0 @html |
| `web/src/routes/{admin,bank-coin}/+page.svelte` | routes | ✓ VERIFIED | /admin gates on isOfficer→officers-only; /bank-coin renders BankCoinForm |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| EvictHandler | store.EvictOwnerTx | in-tx IsOfficerTx re-check (BEGIN IMMEDIATE) | ✓ WIRED | withTx → IsOfficerTx → EvictOwnerTx → AppendAuditTx, all one tx |
| coin route | RequireSession (NOT RequireOfficer) | login-only gate (D-12) | ✓ WIRED | main.go line 329; coin.go 0 officer refs; TestCoinSet_NonOfficerCanWrite |
| main.go | RequireSession wrapping read mux | per-route wrap | ✓ WIRED | all 5 read routes wrapped; TestReadRoutes_RequireSession_401 (all 5) |
| readapi/cors.go | Access-Control-Allow-Credentials: true | CORS header | ✓ WIRED | set on response + preflight; exact-origin, 0 wildcard literals |
| AuthGate.svelte | api.ts typed errors | authGuard catches Unauthenticated/Forbidden | ✓ WIRED | setContext(AUTH_GUARD_KEY); EvictionForm/AdminMgmtForm call it on 403 |
| EvictionForm/AdminMgmtForm | ConfirmDialog | confirm-before-commit | ✓ WIRED | both import + open ConfirmDialog before the destructive call |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| BankCoinForm.svelte | `toons` | `fetchBankToons()` → GET /api/v1/coin/bank-toons → store.ListBankToons (real SELECT) | Yes | ✓ FLOWING |
| EvictionForm.svelte | `evictable` / `preview` | `fetchEvictable()` / `previewEviction()` → store.ListEvictableOwners / PreviewEviction (real SELECT) | Yes | ✓ FLOWING |
| AdminMgmtForm.svelte | `officers` / `promotable` | `fetchOfficers()` → store.ListOfficers + ListPromotableUsers (real JOIN/NOT IN) | Yes | ✓ FLOWING |
| bank view +page.svelte | bank-coin summary | `fetchBankToons()` (same real source) | Yes | ✓ FLOWING |
| SessionIndicator.svelte | avatar/username | whoami-web → store.GetWebUser (real SELECT) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Whole repo builds | `go build ./...` | rc=0 | ✓ PASS |
| Linux server cross-compiles | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` | rc=0 | ✓ PASS |
| Static analysis clean | `go vet ./...` | rc=0 | ✓ PASS |
| Full Go test suite | `go test ./...` | all packages ok | ✓ PASS |
| P15 packages fresh (uncached) | `go test -count=1` webadmin/webauth/store/scheduler/migrations/readapi/cmd | all ok | ✓ PASS |
| Web typecheck | `npm run check` | 432 files, 0 errors, 0 warnings | ✓ PASS |
| Web unit tests | `npm run test:unit -- --run` | 14 files, 172 tests passed | ✓ PASS |
| Web build | `npm run build` | built, wrote site to build/ | ✓ PASS |
| D-12 coin no-officer-gate (comment-stripped) | grep RequireOfficer\|IsOfficer in coin.go | 0 | ✓ PASS |
| W-4 no open-redirect (comment-stripped) | grep redirect/return_to/next reads in handlers.go | 0 | ✓ PASS |
| W-3 archive daily not weekly | grep dueWiki near eviction_archive | 0 | ✓ PASS |
| Append-only audit | grep UPDATE/DELETE audit_log in webadmin | 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| AUTH-08 | 15-02, 15-04 | Discord login gated on guild membership, no allowlist | ✓ SATISFIED (local scope) | Truth 1 — IsGuildMember fail-closed + callback refusal + read-API gate, test-proven. Live smoke = human item. |
| AUTH-09 | 15-01, 15-02, 15-04 | Per-user Discord identity captured + stored | ✓ SATISFIED | Truth 2 — web_user + UpsertWebUser + SessionIndicator |
| ADMIN-04 | 15-01, 15-03, 15-05 | Officer-only eviction form, v1 enforcement + grace/archive | ✓ SATISFIED (local scope) | Truth 3 — full store+handler+form, authorize-under-tx, owner-floor, daily archive, test-proven. Live smoke = human item. |
| ADMIN-05 | 15-01, 15-03, 15-05 | Authenticated bank-coin form persisting 4 values | ✓ SATISFIED | Truth 4 — login-only, range-validated, bank-toon-gated, surfaces in bank view |
| ADMIN-06 | 15-01, 15-03, 15-05 | Officer-management form + owner-floor protection | ✓ SATISFIED (local scope) | Truth 5 — admin.ts port, peer-cannot-remove-floor test-proven. Live smoke = human item. |

All five P15 requirement IDs from the plan frontmatter are accounted for in `.planning/REQUIREMENTS.md` (Traceability table: P15 = AUTH-08/09 + ADMIN-04/05/06, 5/5 mapped, no orphans, no duplicates). No requirement ID appears in a plan that is missing from REQUIREMENTS.md, and no P15-mapped requirement in REQUIREMENTS.md is unclaimed by a plan.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| 00004_web_auth.sql | 72-86 | Down migration uses ALTER TABLE DROP COLUMN | ℹ️ Info | Forward-only posture; never run in prod; IN-01 consciously skipped in review-fix. No goal impact. |
| webadmin/eviction.go | callerMayNotEvictFloor | owner-floor protection bridges owner↔discord via owner.label==username (no FK) | ℹ️ Info | WR-05 hardened (TRIM+COLLATE NOCASE + loud slog.Warn when inert); residual schema-level id-link is a documented out-of-phase follow-up. Self-removal/peer-remove via guild_admins (ADMIN-06) is keyed on the snowflake id and is NOT affected — this textual bridge is only the eviction *data* protection (D-09). |

No blocker or warning anti-patterns. No TODO/FIXME/placeholder, no stub returns, no hardcoded-empty data feeding rendering, no unwired artifacts.

### Human Verification Required

See the `human_verification` frontmatter block. Five items, all deploy-time or visual, all explicitly designated as deferred human-verification by the user-locked build-only directive:

1. **Live Discord OAuth2 login smoke** — needs real DISCORD_* systemd secrets on the box.
2. **Run 00004_web_auth.sql against the live VPS DB** — deploy-time goose.Up.
3. **Seed the owner-floor** — `set-owner-floor <maintainer-discord-id>` on the box.
4. **Live evict/restore/coin/officer smokes** — destructive writes against real guild data.
5. **Visual/interaction QA** of the login flow + 3 forms in a browser (no jsdom in the repo by design).

### Gaps Summary

No gaps. All five observable truths are VERIFIED against the actual codebase — not the SUMMARYs. The verification was adversarial: every store method, handler, route wiring, and frontend component was read in full source; the load-bearing security claims (D-12 login-only coin, W-4 open-redirect, W-3 daily archive, append-only audit, fail-closed membership, hash-only sessions, owner-floor protection) were re-checked with comment-stripped grep-gates; and the full Go + web test suites were re-run uncached and confirmed green (go build/vet/test + linux cross-compile + npm check 0/0 + 172/172 unit + build). The 2 review BLOCKERs (CR-01 coin-input crash, CR-02 epoch-seconds date bug) are confirmed FIXED in the live source with regression coverage. Status is `human_needed` only because the deploy-time live smokes + browser visual QA cannot be performed in the build-and-verify scope and are recorded as the deferred human-verification items the directive prescribed.

---

_Verified: 2026-05-31T04:30:00Z_
_Verifier: Claude (gsd-verifier)_
