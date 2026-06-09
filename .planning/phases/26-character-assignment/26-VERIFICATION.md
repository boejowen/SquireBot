---
phase: 26-character-assignment
verified: 2026-06-08T14:10:00Z
status: human_needed
score: 6/6 must-haves verified (code) — web interactive flows pending browser-smoke
overrides_applied: 0
human_verification:
  - test: "Member 'My characters' flow at /my-characters (signed-in member): claim an unassigned char, release a held char (ConfirmDialog), request a contested char, cancel a pending request; verify inline 409 copy (char_shared / already_assigned / duplicate_request) and per-row busy disabling."
    expected: "Claimed char moves to 'Your characters'; released char returns to claimable; request files and the Request→Cancel toggle tracks it; error codes render as readable inline messages."
    why_human: "web/ vitest is node-only (no jsdom / @testing-library/svelte per the toolchain-install rule); panel RENDERING + interaction are not covered by the 287 passing node tests. (Memory: web-tests-node-only-blind-to-dom.)"
  - test: "Officer assignment section in /admin (signed-in officer): assign/reassign/remove an assignment, approve/deny a pending request, designate a char bank/bot/none; confirm a non-officer hitting /admin sees the 403 officers-only collapse."
    expected: "Assign reassigns the single-assignee row; approve reassigns to requester and clears siblings; designate-bank/bot removes the assignment and disappears from claimable; non-officer gets nothing actionable (server 403s every fetch)."
    why_human: "Officer-panel rendering + the 403 collapse are browser-only; node tests cover the pure helpers + the server handlers, not the Svelte interaction."
  - test: "/my-characters nav link appears in the gear SettingsMenu for a member (not officer-gated); CharMetaForm renders with NO bank-toon checkbox."
    expected: "Link visible to every signed-in member; char-meta form edits class/level/race only."
    why_human: "Visual/nav presence; node tests assert the markup exists but not its rendered visibility for a member session."
  - test: "Deploy 00009 to the live Hetzner DB (goose.Up on boot) and confirm idempotent apply + correct auto-seed against REAL guild data (linked-owner chars assigned; legacy/NULL-owner + bank/bot chars unassigned)."
    expected: "goose advances to version 9; existing characters/assignments undisturbed; linked guildies see their chars under 'My characters' day-one."
    why_human: "Migration idempotency + seed correctness are unit-tested against synthetic fixtures (TestMigrate_00009), but the live-DB apply + the real-data seed outcome is an ops/deploy step out of scope for the plans (backend redeploy happens at phase close)."
---

# Phase 26: Character Assignment Verification Report

**Phase Goal:** Members self-claim/release characters; contested claim → officer-approvable request; officers assign/reassign/remove + designate guildwide-shared guild bank/bot chars; versioned + audited assignment data layer (00009 → schema v9); auto-seeded from owner.discord_user_id. Backend + web only; watcher untouched.

**Verified:** 2026-06-08T14:10:00Z
**Status:** human_needed (all 6 success criteria code-verified; web interactive flows + live-DB deploy are browser/ops UAT — explicitly flagged by the executor, not failures)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|-----------------------------------|--------|----------|
| 1 | Member sees "My characters" + can self-claim (incl. legacy/unlinked-owner or unassigned) (ASSIGN-01/02) | ✓ VERIFIED (code) | `store.ListMyAssignments` + `ListClaimable` (assignment.go:435,465); `ClaimCharTx` plain INSERT `assigned_by='self'` (assignment.go:109); claimable read is owner-agnostic (`NOT EXISTS character_assignment`) so a legacy/NULL-owner char IS claimable. Handlers `ListMyAssignmentsHandler`/`ClaimableHandler`/`ClaimCharHandler` (assignment.go) under RequireSession (main.go:368-370). UI: MyCharactersPanel + /my-characters route + nav link. Tests: `TestClaimCharTx_HappyAndAlreadyAssigned`, `TestClaim_UnassignedChar_Succeeds_Audits`, `TestListMyAssignments_And_Claimable` PASS. |
| 2 | Member can release/unclaim a held char → returns to unassigned (ASSIGN-03) | ✓ VERIFIED (code) | `ReleaseCharTx` owner-scoped DELETE `AND discord_user_id=caller` → `(removed bool)`; foreign row = silent no-op (assignment.go:143). `ReleaseCharHandler` audits only on real release. Test `TestReleaseCharTx_OwnerScopedSilentNoOp` + `TestRelease_HeldChar_Succeeds_ForeignIsNoOp` PASS. |
| 3 | Contested claim → officer-approvable request (4-state machine, partial-unique pending); approve denies siblings (ASSIGN-03/04) | ✓ VERIFIED (code) | `assignment_request` CHECK IN (pending/approved/denied/cancelled) + `assignment_request_pending_uidx` partial-unique (00009 SQL:38,50). `RequestTx` maps the unique conflict → `ErrDuplicateRequest` via modernc code 2067 (assignment.go:191). `ApproveRequestTx` upserts to requester AND denies ALL OTHER pending for that char in one tx (assignment.go:326, Pitfall 3). Tests `TestRequestTx_DuplicatePending`, `TestApproveRequestTx_DeniesSiblings`, `TestApprove_DeniesSiblingPendingRequest` PASS. |
| 4 | Officer assign/reassign/remove + approve/deny, IDOR-safe, server-truth gated (ASSIGN-04) | ✓ VERIFIED (code) | Officer mutators call `isOfficerTx` as FIRST in-tx stmt → `ErrNotAuthorized` (assignment.go:226,258,286,340,386, WR-04 TOCTOU). Reassign = `ON CONFLICT(character_id) DO UPDATE` (single row, never a 2nd). Routes RequireOfficer (main.go:397-402). Assignee validated by `SELECT 1 FROM web_user` existence probe (not usernameOf — avoids NULL-username false-reject). Tests `TestOfficerEndpoints_NonOfficer_Rejected` (table over all 5), `TestOfficerAssign_NullUsernameAssignee_Succeeds`, `TestOfficerAssignTx_AuthorizeAndReassign` PASS. |
| 5 | Shared guild bank/bot designation: single-assignee otherwise; is_guild_bot column; guild-bank subsumes is_bank_toon (officer-only); bank view works with MULTIPLE banks (ASSIGN-05) | ✓ VERIFIED (code) | Single-assignee = `character_assignment.character_id PRIMARY KEY` (00009 SQL:23, schema not store logic). `is_guild_bot` column added (SQL:16). `DesignateCharTx` 3-state mutually-exclusive bank/bot/neither, officer-only, clears assignment + denies pending on bank/bot (Pitfall 6), does NOT demote other banks (assignment.go:385). Single-bank demote REMOVED from `SetCharMetaTx` (now 5-arg, charmeta.go:52); member char-meta no longer writes is_bank_toon. `compute.Bank` doc updated to "multiple banks supported", no query change. 2-bank test `TestBank_MultipleBankToonsRender` asserts both banks render distinct (bank_test.go:61). Tests `TestDesignateCharTx_ClearsAssignmentAndDeniesRequests`, `_BotMutualExclusion`, `TestDesignateBank_ClearsExistingAssignment` PASS. |
| 6 | Every assignment change audited (actor, character, action, time) (ASSIGN-06) | ✓ VERIFIED (code) | Every mutating handler composes `AppendAuditTx` in the SAME withTx: `assignment_claim`/`assignment_release`/`assignment_request`/`request_cancel` (member, assignment.go) + `officer_assign`(+target)/`assignment_remove`/`request_approve`/`request_deny`/`char_designate` (officer, assignment_admin.go). Actor = `caller(ctx)` (session, never body); detail = character_id/request_id only (V7). Audit-only-on-real-change for release/cancel/remove/approve/deny. |
| 7 | 00009 applies idempotently → schema v9, no _meta.schema_version cell, watcher untouched (D-04, CLAUDE.md) | ✓ VERIFIED (code) | `00009_character_assignment.sql` is goose migration 9 (9 .sql files); ONLY `schema_version` mention is a comment saying there is NO write. Auto-seed `INSERT OR IGNORE … JOIN owner … WHERE discord_user_id IS NOT NULL AND is_removed=0 AND is_bank_toon=0 AND is_guild_bot=0` (idempotent, excludes legacy/bank/bot). `owner_id` never touched (D-03). `TestMigrate_00009_CharacterAssignment` asserts column/tables/index/CHECK/seed-inclusion+exclusion/idempotent-rerun PASS. No assignment refs in internal/app, internal/eqfind, internal/sheet — watcher clean. |

**Score:** 6/6 ROADMAP success criteria code-verified (criterion 6 "ASSIGN-05 model without schema rework" + criterion 6 "00009 idempotent" both folded in above as truths 5 + 7).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/backendsrv/migrations/00009_character_assignment.sql` | 2 tables + is_guild_bot + idempotent auto-seed | ✓ VERIFIED | character_assignment (PK single-assignee), assignment_request (4-state CHECK + partial-unique pending), is_guild_bot ALTER, INSERT OR IGNORE seed, Down=no-op. No schema_version write. |
| `internal/backendsrv/store/assignment.go` | 9 Tx mutators + 3 reads + ClaimableChar/ListClaimable + 4 typed errors | ✓ VERIFIED | All 12+ exports present, substantive, parameterized; officer mutators authorize-under-tx; reads joined to character.name, live-only. |
| `internal/backendsrv/webadmin/assignment.go` | 6 member handlers + mapAssignErr | ✓ VERIFIED | RequireSession; body has NO actor field; mapAssignErr covers ErrCharShared→409 + ErrNotAuthorized→403; release/cancel audit-only-on-change. |
| `internal/backendsrv/webadmin/assignment_admin.go` | 6 officer handlers | ✓ VERIFIED | RequireOfficer + in-tx IsOfficerTx; existence-probe (not usernameOf); designate allow-list {bank,bot,none}; all errors via mapAssignErr; all audited. |
| `cmd/squirebot-server/main.go` | 12 routes (6 RequireSession + 6 RequireOfficer) | ✓ VERIFIED | Lines 368-373 (member) + 397-402 (officer); single-space method pattern; char-meta routes unchanged. |
| `web/src/lib/api.ts` | 4 interfaces + 12 typed fns | ✓ VERIFIED | 12 `export function` for the endpoints; snake_case bodies; member bodies carry character_id only. |
| `web/src/lib/components/MyCharactersPanel.svelte` | member panel, imports partitionClaimable + api fns | ✓ VERIFIED | 16 refs to partitionClaimable/api fns; ConfirmDialog release; plain `{}` escape. |
| `web/src/lib/components/AssignmentAdminPanel.svelte` | officer panel, imports requestStatusLabel + officer api fns | ✓ VERIFIED | 20 refs to requestStatusLabel/officer fns; 403 collapse; designate radio. |
| `web/src/routes/my-characters/+page.svelte` | member route | ✓ VERIFIED | Exists, renders MyCharactersPanel. |
| `web/src/lib/components/SettingsMenu.svelte` | member-visible /my-characters link | ✓ VERIFIED | Line 191 `<a href="/my-characters">My characters</a>`, OUTSIDE the officer gate (comment line 187). |
| `web/src/routes/admin/+page.svelte` | "Character assignments" section after Monitors | ✓ VERIFIED | Imports + renders AssignmentAdminPanel (lines 25, 56). |
| `web/src/lib/assignments.ts` + `__tests__/assignments.test.ts` | pure helpers + node test | ✓ VERIFIED | partitionClaimable + requestStatusLabel; test asserts both inside `expect(...)`. |
| `web/src/lib/charmeta.ts` + CharMetaForm.svelte | de-bank-toon | ✓ VERIFIED | Only `is_bank_toon` mention in charmeta.ts is a doc comment; payload omits it; form checkbox removed. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| store/charmeta.go | compute/bank.go | is_bank_toon relaxation | ✓ WIRED | Demote block removed; SetCharMetaTx 5-arg; bank.go doc multi-bank, query unchanged; 2-bank test green. |
| store/assignment.go | character_assignment table | ON CONFLICT(character_id) DO UPDATE | ✓ WIRED | OfficerAssignTx + ApproveRequestTx both upsert on the PK (assignment.go:243,309). |
| main.go | webadmin assignment handlers | mux.Handle RequireSession/RequireOfficer | ✓ WIRED | 12 routes registered with correct gates. |
| webadmin/charmeta.go | store.SetCharMetaTx (5-arg) | IsBankToon removed from req + payload | ✓ WIRED | Build passes against 5-arg signature; field + payload dropped. |
| MyCharactersPanel.svelte | /api/v1/assignments/* | api.ts typed fns | ✓ WIRED | Panel imports fetchMyCharacters/fetchClaimable/claimChar/releaseChar/requestChar/cancelRequest. |
| AssignmentAdminPanel.svelte | /api/v1/admin/assignments/* | api.ts officer fns | ✓ WIRED | Panel imports fetchAllAssignments/officerAssign/remove/approve/deny/designate. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Backend build | `go build ./...` | exit 0 | ✓ PASS |
| Backend vet | `go vet ./internal/backendsrv/... ./cmd/squirebot-server/...` | clean | ✓ PASS |
| Phase 26 store + handler + migration tests (uncached) | `go test … -run 'Assign\|Claim\|Release\|Request\|Designate\|Approve\|Deny\|Officer\|Migrate_00009' -count=1` | all PASS (store 17.0s, webadmin 1.6s, migrations 2.7s) | ✓ PASS |
| 2-bank bank view | `go test ./internal/backendsrv/compute -run MultipleBank -count=1` | ok | ✓ PASS |
| Web typecheck | `cd web && npm run check` | 482 files, 0 errors, 0 warnings | ✓ PASS |
| Web build | `cd web && npm run build` | wrote site, exit 0 | ✓ PASS |
| Web unit tests | `cd web && npm test` | 22 files / 287 tests passed | ✓ PASS |
| Phase 26 commits on branch | `git log` for 10 claimed hashes | all 10 FOUND | ✓ PASS |
| Web panels' RENDERING / interaction | (browser) | not testable in node | ? SKIP → human (node-only vitest, no jsdom) |
| 00009 live-DB apply + real-data seed | (ops deploy) | not run | ? SKIP → human (deploy out of plan scope) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ASSIGN-01 | 26-01/02/03 | Member sees "My characters" | ✓ SATISFIED (code) | Truth 1 |
| ASSIGN-02 | 26-01/02/03 | Self-claim incl. legacy/unlinked-owner | ✓ SATISFIED (code) | Truth 1 — claimable is owner-agnostic |
| ASSIGN-03 | 26-01/02/03 | Release/unclaim → unassigned | ✓ SATISFIED (code) | Truth 2 |
| ASSIGN-04 | 26-01/02/03 | Officer assign/reassign/remove | ✓ SATISFIED (code) | Truths 3, 4 |
| ASSIGN-05 | 26-01/02/03 | Shared bank/bot designation, single-assignee otherwise | ✓ SATISFIED (code) | Truth 5 — single-assignee PK; multi-bank; is_guild_bot; officer-only designate |
| ASSIGN-06 | 26-01/02 | Every change audited (actor/char/action/time) | ✓ SATISFIED (code) | Truth 6 |

All 6 ASSIGN requirements (the full Phase 26 mapping in REQUIREMENTS.md) are code-satisfied. No orphaned requirements for this phase.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None | — | No TODO/FIXME/placeholder/stub in the Phase 26 surface. `return []`/`nil→[]` are intentional empty-state normalization backed by real queries. `partitionClaimable.assignedToOthers` empty-in-practice is an API-shape reality (claimable = unassigned-only), not a stub — and is forward-compatible by design. |

### Human Verification Required

1. **Member "My characters" flow** at `/my-characters` — claim / release(confirm) / request / cancel, inline 409 error copy, busy disabling. *Why human:* web vitest is node-only (no jsdom); panel rendering uncovered (Memory: web-tests-node-only-blind-to-dom — green node tests can hide DOM bugs).
2. **Officer assignment section** in `/admin` — assign/reassign/remove, approve/deny, designate bank/bot/none, 403 officers-only collapse for a non-officer. *Why human:* officer-panel rendering + collapse are browser-only.
3. **Nav link + de-banked CharMetaForm** — `/my-characters` link visible to a member in SettingsMenu; CharMetaForm has no bank-toon checkbox. *Why human:* visual/nav presence for a member session.
4. **Live-DB deploy of 00009** — goose.Up applies on boot, idempotent, auto-seed correct against real guild data. *Why human:* unit-tested on synthetic fixtures only; live apply + real seed is an ops step (backend redeploy happens at phase close, out of plan scope).

Note: the executor explicitly flagged items 1-3 as a browser-smoke gap for /gsd-ui-review and item 4 as deploy-out-of-scope in 26-03-SUMMARY.md — these are expected, not regressions.

### Gaps Summary

No blocking gaps. All 6 ROADMAP success criteria and all 6 ASSIGN requirements are verified in the ACTUAL codebase, not merely from SUMMARY claims:

- The 00009 migration is goose v9 with the single-assignee PK, the 4-state request machine with a partial-unique-pending index, the is_guild_bot column, an idempotent owner-linked auto-seed, and NO `_meta.schema_version` write (verified by reading the SQL + the passing `TestMigrate_00009`).
- The store layer mechanically enforces single-assignee (PK), the bidirectional shared-char exemption (Pitfall 6), and double-approval denial (Pitfall 3) — all backed by passing, uncached tests.
- The single-bank invariant is genuinely relaxed: the demote block is gone from `SetCharMetaTx` (5-arg), and a real 2-bank `compute.Bank` test proves multiple banks render distinct rows.
- The HTTP layer is fully gated (RequireSession / RequireOfficer + in-tx IsOfficerTx), IDOR-safe (owner-scoped silent no-op), and audits every mutation; the existence-probe avoids the NULL-username false-reject.
- The web UI exists, imports the pure helpers + api fns, passes check (0 errors) + build + 287 node tests; the watcher is untouched (no assignment refs in watcher packages; WatcherMaxSchemaVersion not modified — it no longer lives in Go watcher code post-v2.0 "Off Google").

The ONLY open items are (a) the browser-smoke of the interactive web flows — unavoidable given the node-only test harness, explicitly pre-flagged — and (b) the live-DB deploy of 00009. Both are human/ops UAT, not code failures. Status is therefore **human_needed**, not gaps_found.

---

_Verified: 2026-06-08T14:10:00Z_
_Verifier: Claude (gsd-verifier)_
