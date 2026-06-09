---
phase: 26-character-assignment
audited: 2026-06-08
auditor: Claude (gsd-secure-phase)
asvs_level: 1
block_on: high
threats_total: 18
threats_closed: 18
threats_open: 0
accepted_risks: 1
status: SECURED
sources:
  - 26-01-PLAN.md <threat_model> (T-26-01..07)
  - 26-02-PLAN.md <threat_model> (T-26-08..14)
  - 26-03-PLAN.md <threat_model> (T-26-15..18)
audited_at_commit: 55d06d3
---

# Phase 26 (Character Assignment) — Security Verification

Every declared mitigation in the three `<threat_model>` blocks (STRIDE register
T-26-01..18) was verified against the IMPLEMENTED code on `master` @ `55d06d3` — not
documentation or intent. Each threat below resolves to CLOSED with `file:line` evidence,
or to a documented accepted risk. The MD-01/MD-02 code-review fixes are confirmed present
on the audited tree (commits `b1852eb`, `29170a6`, `55d06d3` are on `master` HEAD, not a
dangling branch).

Implementation files were treated as READ-ONLY. The Go watcher is out of scope (untouched
this phase). Backend test suites (`store`, `webadmin`, `migrations`) ran GREEN during the
audit, and every test the threat model cites as evidence was confirmed to exist and pass.

## Threat Register

### 26-01 (data layer) — T-26-01..07

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-26-01 | Elevation (officer TOCTOU) | mitigate | CLOSED | `isOfficerTx(ctx, tx, callerID)` is the FIRST in-tx statement in EVERY officer mutator: `store/assignment.go:237` (OfficerAssignTx), `:269` (RemoveAssignTx), `:297` (ApproveRequestTx), `:351` (DenyRequestTx), `:397` (DesignateCharTx); `!ok → ErrNotAuthorized`. `isOfficerTx` is the real helper at `store/admins.go:85`. Test: `webadmin/assignment_test.go:483 TestOfficerEndpoints_NonOfficer_Rejected` (table over every officer endpoint). |
| T-26-02 | Tampering (approval race) | mitigate | CLOSED | `ApproveRequestTx` denies ALL OTHER pending requests for the same `character_id` in the same tx: `store/assignment.go:337-343` (`UPDATE … status='denied' … WHERE character_id=? AND status='pending' AND id<>?`). Stale/non-pending request short-circuits to `(false,nil)` BEFORE any write: `:306-315`. Partial-unique pending index blocks single-member double-file: migration `00009:50-51`. Tests: `store/assignment_test.go:519 TestApproveRequestTx_DeniesSiblings`, `webadmin/assignment_test.go:580 TestApprove_DeniesSiblingPendingRequest`. |
| T-26-03 | Tampering (two-members-one-char) | mitigate | CLOSED | `character_assignment.character_id INTEGER PRIMARY KEY` makes a second row structurally impossible: `00009_character_assignment.sql:23`. Reassign is `INSERT … ON CONFLICT(character_id) DO UPDATE` (never a second row): `store/assignment.go:251-258` (OfficerAssign), `:317-323` (Approve). |
| T-26-04 | Elevation/Tampering (shared-char exemption) | mitigate | CLOSED | Bidirectional in the STORE: `ClaimCharTx` rejects shared (`store/assignment.go:111-117` via `charSharedTx` `:92-104`, checks `is_bank_toon=1 OR is_guild_bot=1`); `RequestTx` `:166-172`; `OfficerAssignTx` `:244-250`. `DesignateCharTx` clears assignment + denies pending in the same tx when bank/bot: `:426-439`. Tests: `webadmin/assignment_test.go:145 TestClaim_BankOrBotChar_CharShared`, `:562 TestOfficerAssign_BankChar_CharShared`, `:628 TestDesignateBank_ClearsExistingAssignment`; `store/assignment_test.go:615 TestDesignateCharTx_ClearsAssignmentAndDeniesRequests`, `:689 TestDesignateCharTx_BotMutualExclusion`. |
| T-26-05 | Spoofing (foreign release/cancel) | mitigate | CLOSED | `ReleaseCharTx` DELETE scoped `AND discord_user_id=?(caller)`: `store/assignment.go:145-147` → 0 rows → `(false,nil)` silent no-op. `CancelRequestTx` scoped `AND requester=?(caller) AND status='pending'`: `:215-219`. No existence leak. Tests: `webadmin/assignment_test.go:186 TestRelease_HeldChar_Succeeds_ForeignIsNoOp`, `:263 TestCancelRequest_OwnPending_ThenForeignNoOp`. |
| T-26-06 | Tampering (SQLi) | mitigate | CLOSED | Parameterized `?` placeholders ONLY across `store/assignment.go` (all ExecContext/QueryRowContext/QueryContext call sites: `:94,:119,:130,:145,:176,:196,:215,:251,:276,:306,:317,:329,:337,:413,:427,:432,:447,:479,:521,:551,:570`). No string concatenation into SQL. The duplicate-request conflict is detected via the modernc extended result code `sqliteConstraintUnique=2067` (`wantlist.go:46`), not a string-match: `assignment.go:201-203`. `owner_id` is never written by any assignment statement (grep-confirmed; D-03 honored). |
| T-26-07 | Tampering (bank-view merge after single-bank relaxation) | **accept** | CLOSED (accepted) | See Accepted Risks AR-26-07. Multiple guild banks are intentional + officer-only via `DesignateCharTx`; the consolidated Char-column bank grid disambiguates; the 2-bank render is proven by `compute/bank_test.go TestBank_MultipleBankToonsRender`. Officer mis-designation is low/recoverable/audited (`char_designate` audit row). |

### 26-02 (HTTP API) — T-26-08..14

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-26-08 | Spoofing/Elevation (member identity from body) | mitigate | CLOSED | `assignmentReq` carries ONLY `character_id` — NO actor field: `webadmin/assignment.go:48-50`. Every member mutator derives the actor from `caller(ctx)`: `:179` (claim), `:214` (release), `:255` (request), `:288` (cancel). Test: `webadmin/assignment_test.go:383 TestClaim_SpoofedDiscordIDInBody_Ignored`. |
| T-26-09 | Elevation (bypass hidden nav, hit officer route) | mitigate | CLOSED | `RequireOfficer` at the route for all 6 officer endpoints: `main.go:398-403`; AND in-tx `isOfficerTx` (T-26-01). Test: `webadmin/assignment_test.go:483 TestOfficerEndpoints_NonOfficer_Rejected`. |
| T-26-10 | Elevation (just-demoted officer TOCTOU) | mitigate | CLOSED | Authorize-under-tx: the in-tx `isOfficerTx` re-check (T-26-01 evidence) closes the window between `RequireOfficer` and the write. |
| T-26-11 | Tampering (assign to non-existent/spoofed assignee) | mitigate | CLOSED | `OfficerAssignHandler` validates the body `assignee` with a dedicated existence probe `SELECT 1 FROM web_user WHERE discord_user_id = ?` → 400 invalid_input ONLY on `sql.ErrNoRows`: `webadmin/assignment_admin.go:104-115`. NOT `usernameOf` (avoids the empty-username false-reject). FK `character_assignment.discord_user_id → web_user` is the DB backstop: `00009:24`. Note: probe runs pre-tx (review LO-01, accepted — FK is the integrity guarantee). |
| T-26-12 | Information disclosure (IDOR on release/cancel) | mitigate | CLOSED | Owner-scoped in the store (T-26-05) → 0 rows → 200 `{"released":false}` / `{"cancelled":false}`; the handler never reveals the row: `webadmin/assignment.go:224-234`, `:298-308`. The added member "my pending requests" read is requester-scoped (`store.ListMyPendingRequests` `assignment.go:478-504`, owner from `caller(ctx)` `webadmin/assignment.go:128`). Test: `webadmin/assignment_test.go:350 TestListMyPendingRequests_RequesterScoped` (a stranger sees none). |
| T-26-13 | Repudiation (no audit trail) | mitigate | CLOSED | `AppendAuditTx` composed in the SAME `withTx` as every mutation: claim `assignment.go:186`, release `:225`, request `:262`, cancel `:299`; officer_assign `assignment_admin.go:123`, assignment_remove `:166`, request_approve `:213`, request_deny `:252`, char_designate `:310`. Audit detail carries `character_id` / `request_id` (+ `target` on assign) only — no PII (V7). |
| T-26-14 | Input validation | mitigate | CLOSED | `decodeAssignmentReq` rejects `character_id <= 0` → 400: `webadmin/assignment.go:81-88`. Officer `request_id <= 0` → 400: `assignment_admin.go:198,:237`. Designate `mode` allow-listed `{none,bank,bot}` via `designateModes` map → 400 otherwise: `assignment_admin.go:274-302`. DB CHECK on `status`: `00009:38-39`. Parameterized only. |

### 26-03 (web UI) — T-26-15..18

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-26-15 | Elevation (non-officer reaches admin panel via typed URL) | accept (UX) / mitigate (server) | CLOSED | UX layer: `/admin` Layer-1 `{#if !isOfficer}` refusal + `AssignmentAdminPanel` 403 officers-only collapse. Server truth: `RequireOfficer` (`main.go:398-403`) + in-tx `isOfficerTx` (T-26-09/T-26-01) is the real gate; every officer fetch 403s for a non-officer, so the panel renders nothing useful. The server-side mitigation is the load-bearing one and is CLOSED. |
| T-26-16 | Tampering (XSS) | mitigate | CLOSED | No `{@html}` in either new panel (grep: 0 matches in `MyCharactersPanel.svelte` and `AssignmentAdminPanel.svelte`). All char/user/error values render via plain `{}` auto-escape: e.g. `AssignmentAdminPanel.svelte:221 {a.name}`, `:280 {req.requester}`, `:169 {req.character_name}`. A bogus character/owner name cannot inject markup. |
| T-26-17 | Spoofing (client supplies actor identity) | mitigate | CLOSED | `api.ts` member bodies carry ONLY `character_id`: `claimChar`/`releaseChar`/`requestChar`/`cancelRequest` `api.ts:921-937`. Officer `officerAssign` body carries a TARGET (`assignee`) `:946-956` — a target, not an actor; the server actor is always `caller(ctx)`. No frontend path sends a `discord_user_id` actor (grep: 0 in `MyCharactersPanel.svelte`). |
| T-26-18 | Information disclosure (401/403 leaks) | mitigate | CLOSED | `getJSON`/`postJSON` throw typed `Unauthenticated`/`Forbidden`; panels route 401→AuthGuard/LoginScreen and 403→officers-only collapse — no stack/detail surfaces. (`MyCharactersPanel` `route(err)` 401→guard; `AssignmentAdminPanel` 403 collapse — per 26-03-SUMMARY, confirmed no `{@html}` / no raw error rendering.) |

## Accepted Risks

### AR-26-07 — Multiple guild-bank characters after single-bank relaxation (T-26-07)

**Disposition:** accept (declared in 26-01-PLAN `<threat_model>`).
**Risk:** The single-`is_bank_toon` invariant was intentionally relaxed so multiple
officer-designated guild banks render in the consolidated bank view. An officer could
mis-designate a character as a guild bank.
**Why accepted:** Multiple guild banks are now an intended, officer-only feature
(`DesignateCharTx` is officer-gated via `RequireOfficer` + in-tx `isOfficerTx`). The
consolidated Char-column bank grid disambiguates rows (no merge/drop — proven by
`compute/bank_test.go TestBank_MultipleBankToonsRender`, query unchanged). A
mis-designation is low-impact, fully recoverable (re-designate to `none`/`bank`/`bot`),
and audited (`char_designate` audit row, T-26-13). Trust-rich ~12-person guild context.
**Residual:** None requiring action this phase.

## Unregistered Flags

None. Both 26-02-SUMMARY and 26-03-SUMMARY `## Threat Flags` sections explicitly report
"None — no security surface beyond the plan's `<threat_model>` was introduced." No new
attack surface appeared during implementation without a mapped threat ID. (26-01-SUMMARY
has no Threat Flags section; its surface is fully covered by T-26-01..07.)

## Notes on Review-Fix Provenance

The 26-REVIEW found 0 BLOCKER / 0 HIGH and 2 MEDIUM (MD-01, MD-02). 26-REVIEW-FIX landed
the fixes on a worktree branch `reviewfix-26`. This audit independently confirmed those
commits are integrated on `master` HEAD (`b1852eb`, `29170a6`, `55d06d3`) and that:
- MD-01: `RequestTx` now rejects non-contested chars with `ErrCharNotContested` →
  `mapAssignErr` maps it to 409 `not_contested` (`assignment.go:186-191`,
  `webadmin/assignment.go:66-67`). Tightens T-26-14 (input validity of the request queue).
- MD-02: the requester-scoped `GET /api/v1/assignments/requests/mine` read exists, is
  registered under `RequireSession` (`main.go:369`), and is IDOR-safe — reinforcing T-26-12.

The three LOW findings (LO-01..03) were deferred by the review-fix; none is a declared
threat mitigation, and each is FK-backstopped or subsumed by the MD-01 guard. No declared
mitigation is left open by the deferral.

## Audit Trail

| Date | Auditor | Action | Result |
|------|---------|--------|--------|
| 2026-06-08 | Claude (gsd-secure-phase) | Verified T-26-01..18 against implemented code @ `55d06d3`; ran `store`/`webadmin`/`migrations` suites (GREEN); confirmed all cited evidence tests exist + pass; confirmed review-fix commits integrated on master | SECURED — 18/18 closed (17 mitigate + 1 accept), 0 open, 0 unregistered flags |
