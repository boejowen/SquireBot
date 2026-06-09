---
phase: 26-character-assignment
fixed_at: 2026-06-08T00:00:00Z
review_path: .planning/phases/26-character-assignment/26-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
deferred: 3
status: all_fixed
worktree: /tmp/sv-26-reviewfix-gZEceJ (branch reviewfix-26, off master @ c08b4e6)
---

# Phase 26: Code Review Fix Report

**Fixed at:** 2026-06-08
**Source review:** .planning/phases/26-character-assignment/26-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (MD-01, MD-02, NIT-01, NIT-02)
- Fixed: 4
- Skipped: 0
- Deferred (out of scope, LOW): 3 (LO-01, LO-02, LO-03)

All commits were made on an isolated worktree branch `reviewfix-26` (forked from
`master` @ `c08b4e6`, the Phase-26 head) because `master` was already checked out
in the main working tree. The orchestrator should integrate `reviewfix-26` into
`master` (fast-forward / merge). The Go watcher was NOT touched.

## Fixed Issues

### MD-01: `RequestTx` files a pending request for a non-contested character

**Files modified:** `internal/backendsrv/store/assignment.go`,
`internal/backendsrv/store/assignment_test.go`,
`internal/backendsrv/webadmin/assignment.go`,
`internal/backendsrv/webadmin/assignment_test.go`
**Commit:** `b1852eb`
**Applied fix:** Added `ErrCharNotContested` and made `RequestTx` read the current
assignee BEFORE inserting the pending row, rejecting per D-07:
- `!current.Valid` (unassigned) → `ErrCharNotContested` (the member should `/claim`)
- `current.String == callerID` (caller already holds it) → `ErrCharNotContested`

The existing shared-char (`ErrCharShared`) and duplicate-pending
(`ErrDuplicateRequest`) guards are unchanged. `mapAssignErr` maps the new sentinel
to **409 `not_contested`**. Store tests assert request-on-unassigned and
request-on-self-held are rejected with no pending row written, and request-on-
other-held still succeeds (1 pending row). A handler test asserts 409 `not_contested`
for both non-contested cases.

### MD-02: `MyCharactersPanel` loses pending-request state on reload

**Files modified (backend, committed with MD-01 due to shared files):**
`internal/backendsrv/store/assignment.go`,
`internal/backendsrv/store/assignment_test.go`,
`internal/backendsrv/webadmin/assignment.go`,
`internal/backendsrv/webadmin/assignment_test.go` — **Commit:** `b1852eb`
**Files modified (route + web):** `cmd/squirebot-server/main.go`,
`web/src/lib/api.ts`, `web/src/lib/components/MyCharactersPanel.svelte` —
**Commit:** `29170a6`
**Applied fix:**
- **store:** `ListMyPendingRequests(ctx, db, requester)` returns the caller's own
  pending `assignment_request` rows (`character_id`, char name, `created_at`),
  requester-scoped, joined to the char name, live chars only, ordered by name.
- **handler:** `ListMyPendingRequestsHandler` — a `RequireSession` GET deriving the
  owner from `caller(ctx)` (never the body), normalizing nil → `[]`.
- **route:** `GET /api/v1/assignments/requests/mine` registered beside the other
  member assignment routes in `main.go`.
- **api.ts:** `MyPendingRequest` interface + `fetchMyPendingRequests()` typed wrapper.
- **panel:** `load()` / `reloadLists()` now also fetch the pending requests and
  rebuild the `requested` set from them, so the Request→Cancel affordance survives a
  reload (and a re-request after reload no longer hits a guaranteed 409
  `duplicate_request`). `doRequest`/`doCancel` still patch the set optimistically.
  Char/user names render via `{}` auto-escape only (no `{@html}`). A `not_contested`
  inline reason string was added to mirror the new MD-01 server code.

Store test: `ListMyPendingRequests` is requester-scoped and excludes a cancelled
request. Handler test: a stranger sees none of the requester's pending requests
(IDOR-safe).

### NIT-01: Stale `isBankToon` comment in `CharMetaForm.svelte`

**Files modified:** `web/src/lib/components/CharMetaForm.svelte`
**Commit:** `55d06d3`
**Applied fix:** Changed `// The form inputs (level is a raw string; isBankToon a
checkbox bool).` to `// The form inputs (level is a raw string).` — the `isBankToon`
field was removed this phase (designation is officer-only now).

### NIT-02: Dead `ErrNotAssignee` export

**Files modified:** `internal/backendsrv/store/assignment.go` (committed with MD-01)
**Commit:** `b1852eb`
**Applied fix:** Grepped the whole tree first — `ErrNotAssignee` appeared ONLY in its
own declaration (no returns, no `errors.Is`). Removed it. The release/cancel paths use
the silent-no-op bool return, so nothing depended on it.

## Deferred Issues (out of scope — 3 LOW findings)

Per the task scope (`critical_warning`), the three LOW findings were intentionally
NOT fixed and are deferred:

### LO-01: `OfficerAssign` assignee existence probe runs outside the write tx
**File:** `internal/backendsrv/webadmin/assignment_admin.go:104-115`
**Reason deferred:** FK-backstopped + `maxconns=1` + ~12-person guild makes the window
effectively unreachable; the review itself rates it optional.

### LO-02: `current_assignee` request snapshot captured but never surfaced
**File:** `internal/backendsrv/store/assignment.go`, `webadmin/assignment_admin.go`,
`web/src/lib/components/AssignmentAdminPanel.svelte`
**Reason deferred:** Unused officer-UI plumbing, not a bug. (The MD-01 fix preserves
the snapshot for a future "currently held by …" render.)

### LO-03: A member can re-`/request` after officer approval
**File:** `internal/backendsrv/store/assignment.go`
**Reason deferred:** Largely subsumed by the MD-01 contested-state guard — a member who
just had a request approved now holds the char, so a fresh `/request` hits
`ErrCharNotContested` (self-held). Any residual is benign queue noise.

## Verify Gate

Both gates ran GREEN after the fixes (in the isolated worktree; `web/node_modules`
was junctioned from the main tree, then removed — it is gitignored, so the worktree
stayed clean):

**Backend:**
- `go build ./...` → clean
- `go vet ./internal/backendsrv/...` → clean (no output)
- `go test ./internal/backendsrv/...` → all packages `ok` (store, webadmin,
  migrations, compute, and all others)

**Web (`cd web`):**
- `npm run check` → 482 files, 0 errors, 0 warnings
- `npm run build` → built OK (adapter-static wrote `build/`)
- `npm test` → 22 files, 287 tests passed

## Commits

| Commit | Finding(s) | Files |
|--------|-----------|-------|
| `b1852eb` | MD-01, MD-02 (backend), NIT-02 | store/assignment.go, store/assignment_test.go, webadmin/assignment.go, webadmin/assignment_test.go |
| `29170a6` | MD-02 (route + web) | cmd/squirebot-server/main.go, web/src/lib/api.ts, web/src/lib/components/MyCharactersPanel.svelte |
| `55d06d3` | NIT-01 | web/src/lib/components/CharMetaForm.svelte |

Note: MD-01 and MD-02's backend pieces both edit `store/assignment.go` and
`webadmin/assignment.go` (+ their tests), so they are physically inseparable at
file granularity and landed in one commit (`b1852eb`); the cleanly-separable MD-02
route/api/panel and the NITs are their own commits.

---

_Fixed: 2026-06-08_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
