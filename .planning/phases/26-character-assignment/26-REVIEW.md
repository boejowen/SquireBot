---
phase: 26-character-assignment
reviewed: 2026-06-08T00:00:00Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/compute/bank.go
  - internal/backendsrv/compute/bank_test.go
  - internal/backendsrv/migrations/00009_character_assignment.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/assignment.go
  - internal/backendsrv/store/assignment_test.go
  - internal/backendsrv/store/charmeta.go
  - internal/backendsrv/store/charmeta_test.go
  - internal/backendsrv/webadmin/assignment.go
  - internal/backendsrv/webadmin/assignment_admin.go
  - internal/backendsrv/webadmin/assignment_test.go
  - internal/backendsrv/webadmin/charmeta.go
  - internal/backendsrv/webadmin/charmeta_test.go
  - web/src/lib/__tests__/assignments.test.ts
  - web/src/lib/__tests__/charmeta.test.ts
  - web/src/lib/api.ts
  - web/src/lib/assignments.ts
  - web/src/lib/charmeta.ts
  - web/src/lib/components/AssignmentAdminPanel.svelte
  - web/src/lib/components/CharMetaForm.svelte
  - web/src/lib/components/MyCharactersPanel.svelte
  - web/src/lib/components/SettingsMenu.svelte
  - web/src/routes/admin/+page.svelte
  - web/src/routes/my-characters/+page.svelte
findings:
  blocker: 0
  high: 0
  medium: 2
  low: 3
  nit: 2
  total: 7
status: issues_found
---

# Phase 26: Code Review Report

**Reviewed:** 2026-06-08
**Depth:** standard
**Files Reviewed:** 25
**Status:** issues_found

## Summary

Phase 26 (Character Assignment) is a high-quality, security-disciplined implementation. The
adversarial review focused on the auth/assignment surface (IDOR, the officer gate, the
two-members-one-character race, the bank/bot exemption, SQL correctness, audit, and web XSS),
and found **no BLOCKER and no HIGH findings**. The structural guards the research mandated are
all present and correct:

- **IDOR:** every member mutator derives identity from `caller(ctx)`; no member handler reads
  an actor from the body (`assignmentReq` carries only `character_id`). Release/cancel are
  owner-scoped silent no-ops. A dedicated `TestClaim_SpoofedDiscordIDInBody_Ignored` proves the
  spoof is ignored.
- **Officer gate (TOCTOU):** every officer mutator calls `isOfficerTx` as its FIRST in-tx
  statement, behind `RequireOfficer` at the route; `TestOfficerEndpoints_NonOfficer_Rejected`
  covers every officer endpoint.
- **Approval race:** the `character_assignment` PK + the `assignment_request_pending_uidx`
  partial-unique index are present; `ApproveRequestTx` denies all sibling pending requests in
  the same tx and short-circuits on a stale/non-pending request. Tested at both store and
  handler layers.
- **Exemption (bidirectional):** enforced in the STORE — `ClaimCharTx`/`RequestTx`/
  `OfficerAssignTx` reject `is_bank_toon=1 OR is_guild_bot=1`; `DesignateCharTx` clears the
  assignment + denies pending requests in the same tx.
- **SQL:** parameterized `?` everywhere; the auto-seed `INSERT OR IGNORE … SELECT`, the
  existence probe, and the migration are correct and idempotent (proven by
  `TestMigrate_00009_CharacterAssignment`).
- **`owner_id`** is never touched by any assignment code (D-03 honored).
- **Audit:** `AppendAuditTx` is composed in the same `withTx` as every mutation; details carry
  only `character_id`/`request_id`/`target` (V7).
- **Web:** all char-names/usernames render via `{}` auto-escape; no `{@html}`; the `/admin`
  section is officer-gated (Layer-1 UX) over the server gate; `api.ts` error mapping is correct.

`go build` and the backend test suites (`store`, `webadmin`, `migrations`, `compute`) all pass.

The findings below are correctness/spec-fidelity and quality issues, not security holes.

## Medium

### MD-01: `RequestTx` files a pending request for a non-contested character (unassigned or self-held)

**File:** `internal/backendsrv/store/assignment.go:164-197`
**Issue:** D-07 scopes a request to "a character **already assigned to another member**." But
`RequestTx` snapshots the current assignee as *nullable* and inserts a pending request
unconditionally — it never verifies the char is (a) assigned at all, nor (b) assigned to
*someone other than the caller*. Consequences:
- A member can `POST /assignments/request` for an **unassigned** char (instead of `/claim`),
  creating an officer-queue entry for a char they could have claimed instantly.
- A member can file a request for a char **they already hold** (`current_assignee` == caller),
  or for a char assigned to themselves — pure queue noise that an officer must triage.
- Approving any of these is benign (`ApproveRequestTx` always assigns to the *requester* read
  from the row, so there is no privilege or identity confusion — hence Medium, not a BLOCKER),
  but it muddies the contested-claim workflow the queue is supposed to model.

The frontend only ever offers Request for `assignedToOthers` (empty today), so the noise is not
reachable through the UI — but the endpoint is directly POST-able by any session.

**Fix:** In `RequestTx`, after the snapshot, reject the non-contested cases with a typed error
(e.g. reuse `ErrCharAlreadyAssigned` semantics or add `ErrNotContested`):
```go
if !current.Valid {
    return ErrCharNotContested // unassigned — caller should /claim, not /request
}
if current.String == callerID {
    return ErrCharNotContested // caller already holds it
}
```
Map the new sentinel to 409 in `mapAssignErr`. Add a store + handler test.

### MD-02: `MyCharactersPanel` loses pending-request state across reloads → guaranteed 409 on re-request

**File:** `web/src/lib/components/MyCharactersPanel.svelte:59,113-117,136-151`
**Issue:** The set of characters the caller has an outstanding request for (`requested`) is
purely client-side optimistic state. `load()` / `reloadLists()` do NOT repopulate it (the code
comment acknowledges "the backend has no 'my requests' read"). After any page reload, a char the
member already has a pending request for renders the **Request** button again; clicking it hits
the partial-unique index → 409 `duplicate_request`, surfaced as an inline error. The member has
no way to see or cancel their own pending request after a reload — the Cancel affordance vanishes.
Because `assignedToOthers` is empty today (the claimable read returns only unassigned rows) this
is latent, but it becomes a live UX defect the moment the contested path is exercised (Phase
27/28 or a widened claimable read).

**Fix:** Add a member "my pending requests" read (`GET /api/v1/assignments/requests/mine`,
RequireSession, requester-scoped) and hydrate `requested` from it in `load()`. Until that exists,
this remains a known gap — flag it explicitly for the contested-path work rather than relying on
the empty-list accident.

## Low

### LO-01: `OfficerAssign` assignee existence probe runs outside the write tx (benign TOCTOU)

**File:** `internal/backendsrv/webadmin/assignment_admin.go:104-115`
**Issue:** The `SELECT 1 FROM web_user WHERE discord_user_id = ?` existence probe is executed
on `db` before `withTx`. A web_user deleted between the probe and the `OfficerAssignTx` insert
would pass the 400 pre-check, then fail the FK inside the tx and surface as a generic 500 rather
than a clean 400. With `maxconns=1` + the ~12-person guild this is effectively unreachable, and
the FK is the real backstop — hence Low.
**Fix:** Optional. Either accept (the FK guarantees integrity; the comment already notes the FK
backstop) or move the existence probe inside the tx and map its `sql.ErrNoRows` to a typed
`invalid_input`.

### LO-02: `current_assignee` request snapshot is captured but never surfaced or used

**File:** `internal/backendsrv/store/assignment.go:172-184`, `webadmin/assignment_admin.go`,
`web/src/lib/components/AssignmentAdminPanel.svelte:275-301`
**Issue:** `RequestTx` snapshots `current_assignee` and `ListPendingRequests`/`PendingRequest`
carry it across the wire, but the officer queue UI renders only `requester` + `character_name`
— the snapshot of who currently holds the contested char is never shown. An officer approving a
contested request can't see who they're displacing without cross-referencing the All-assignments
list. Dead-ish data path; not a bug, but unused plumbing.
**Fix:** Render `current_assignee` ("currently held by …") in the pending-request row, or drop
the field if it is genuinely unused.

### LO-03: A member can re-`/request` after officer approval, since approval doesn't block a fresh pending row

**File:** `internal/backendsrv/store/assignment.go:164-197, 285-334`
**Issue:** The partial-unique pending index only prevents a *second simultaneous pending* row.
Once a request is approved/denied (status no longer 'pending'), the same member can file a brand
new pending request for the same char. Combined with MD-01 (no contested-state check), a member
who already holds a char (e.g. just had a request approved) can immediately file another pending
request for it. Benign (re-approval is a self-reassign), but it lets the queue accrue redundant
entries. Largely subsumed by the MD-01 fix.
**Fix:** The MD-01 contested-state guard (reject self-held / unassigned) closes most of this.

## Nit

### NIT-01: Stale comment references a removed `isBankToon` checkbox

**File:** `web/src/lib/components/CharMetaForm.svelte:54`
**Issue:** `// The form inputs (level is a raw string; isBankToon a checkbox bool).` — the
`isBankToon` field was removed from `CharMetaInputs` in this phase (it's officer-only now). The
comment now describes a field that no longer exists.
**Fix:** Update to `// The form inputs (level is a raw string).`

### NIT-02: `ErrNotAssignee` is exported but never returned or matched

**File:** `internal/backendsrv/store/assignment.go:57-60`
**Issue:** The sentinel is declared "for completeness" but the release/cancel paths use the
silent-no-op bool return instead, so nothing ever returns or `errors.Is`-checks it. Dead export.
**Fix:** Remove `ErrNotAssignee`, or wire it where a typed not-assignee error would actually be
returned. (Low value either way — it's harmless documentation.)

---

_Reviewed: 2026-06-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
