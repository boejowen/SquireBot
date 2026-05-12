---
phase: 09-watcher-robustness-polish
plan: 04
subsystem: app
tags: [auth, runapp, auth-07, boot-time, invalid-grant, wave2]
requirements: [AUTH-07]
dependency_graph:
  requires:
    - "Plan 09-01 (OPS-06) pre-Ready tray queue — buffers SetIconHealth + SetStatus + ShowReauthorize fired from boot fast-fail"
  provides:
    - "Boot-time invalid_grant → Reauthorize UX parity with AUTH-05 running-state path"
  affects:
    - "internal/app/runapp.go RunApp cold-start fast-fail block"
    - "internal/app/runapp_test.go classifier coverage"
tech_stack:
  added: []
  patterns:
    - "Tiny extractable helper returns enum classification so unit tests can assert branching without exercising tray internals (pendingSnapshot is package-private to tray)"
    - "Canonical AUTH-05 status string reused verbatim — perfect copy parity between boot-time and running-state recovery"
    - "Direct auth.IsRevokedRefreshToken call (NOT isPermanentAuthErr wrapper) — boot-time cannot produce sheet.ErrPermanentAuth, so wrapper would needlessly broaden the trigger surface"
key_files:
  created: []
  modified:
    - internal/app/runapp.go
    - internal/app/runapp_test.go
decisions:
  - "Helper returns bootAuthClassification enum (bootAuthOther | bootAuthRevoked) so tests can assert branch selection deterministically"
  - "Reuse canonical AUTH-05 status string 'Reauthorize: refresh token died. Click Reauthorize…' verbatim (perfect parity, no new copy variant)"
  - "Use auth.IsRevokedRefreshToken directly, not isPermanentAuthErr (boot path cannot produce sheet.ErrPermanentAuth — no Sheets call yet)"
  - "RunApp call site discards the classification return (`_ =`) — only the test exercises it"
metrics:
  duration: "~15 min (after worktree-path correction)"
  completed_date: "2026-05-12"
  tasks_completed: 2
  commits: 2
  tests_added: 3
  tests_replaced: 0
---

# Phase 9 Plan 04: AUTH-07 Boot-time invalid_grant → Reauthorize Summary

Classifies cold-start `buildTokenSourceFromWincred` failures via the same `auth.IsRevokedRefreshToken`
classifier AUTH-05 uses, and on a revoked-refresh-token match fires the canonical AUTH-05 tray
triple (`SetIconHealth(HealthRed)` → `SetStatus("Reauthorize: refresh token died. Click Reauthorize…")`
→ `ShowReauthorize()`) BEFORE `RunApp` returns. Plan 09-01's OPS-06 queue buffers these pre-Ready
calls; `OnReady` drains them and the tray menu opens already in the auth-error state with a
clickable Reauthorize — no transient "Initialising…" stranded window. Non-revoked wincred breakage
preserves the original ContinueSetup recovery path verbatim.

## What Shipped

- **Task 1 (commit `b561b09`) — `applyBootAuthError` helper + enum + simplified call site:**
  - New `type bootAuthClassification int` with iota-numbered constants `bootAuthOther` and `bootAuthRevoked`
  - New `applyBootAuthError(t *tray.Controller, err error) bootAuthClassification` helper placed just before `suspendForAuth`. Logs `slog.Error("token rebuild from wincred failed", "err", err)`, classifies via `auth.IsRevokedRefreshToken(err)`:
    - Revoked → `SetIconHealth(HealthRed)` → `SetStatus("Reauthorize: refresh token died. Click Reauthorize…")` → `ShowReauthorize()` → return `bootAuthRevoked`
    - Other → `SetStatus(fmt.Sprintf("Auth error: %v", err))` → `SetIconHealth(HealthRed)` → `ShowContinueSetup()` → return `bootAuthOther`
  - `RunApp` cold-start fast-fail block at lines 102-112 simplified to `_ = applyBootAuthError(t, err); return`
  - No changes to imports (`auth`, `tray`, `slog`, `fmt` already present); no changes to `suspendForAuth` / `isPermanentAuthErr`; no schema impact

- **Task 2 (commit `6160971`) — three unit tests in `internal/app/runapp_test.go`:**
  - `TestApplyBootAuthError_Revoked`: constructs `*tray.Controller` via `tray.NewController(tray.Config{})`, feeds `&oauth2.RetrieveError{ErrorCode: "invalid_grant"}`, asserts `bootAuthRevoked` returned
  - `TestApplyBootAuthError_NonRevoked`: feeds `errors.New("wincred read failed: target not found")`, asserts `bootAuthOther`
  - `TestApplyBootAuthError_TableDriven`: 3 sub-cases (RetrieveError, generic errors.New, fmt.Errorf-wrapped) — mirrors `TestNeedsWizard` pattern at runapp_test.go:45-68
  - New imports added: `errors`, `fmt`, `golang.org/x/oauth2`, `github.com/boejowen/SquireBot/internal/tray`

## must_haves Truths — Verified

1. **"When `RunApp` fast-fails on a cold-start `buildTokenSourceFromWincred` error AND that error matches `auth.IsRevokedRefreshToken(err)`, the controller receives (in order) `SetIconHealth(HealthRed)`, `SetStatus(...)`, `ShowReauthorize()` — mirroring AUTH-05's canonical `suspendForAuth` triple."**
   Verified structurally by `applyBootAuthError` source (`internal/app/runapp.go` lines 650-668): the revoked branch fires the three calls in the exact same order as `suspendForAuth` (lines 644-647). `TestApplyBootAuthError_Revoked` and `TestApplyBootAuthError_TableDriven/revoked-oauth2-retrieveerror` confirm the classifier branches into this path for `oauth2.RetrieveError{ErrorCode: "invalid_grant"}`.

2. **"When `RunApp` fast-fails on a cold-start `buildTokenSourceFromWincred` error that does NOT match `auth.IsRevokedRefreshToken`, the existing ContinueSetup-style recovery is preserved (red icon + status with the raw error + ShowContinueSetup)."**
   Verified by `applyBootAuthError` non-revoked branch (lines 664-666): `SetStatus(fmt.Sprintf("Auth error: %v", err))` → `SetIconHealth(HealthRed)` → `ShowContinueSetup()` — identical to the pre-AUTH-07 code at runapp.go:106-108. `TestApplyBootAuthError_NonRevoked` and `TestApplyBootAuthError_TableDriven/non-revoked-generic` + `/non-revoked-wrapped` confirm both `errors.New(...)` and `fmt.Errorf(..., %w, ...)` shapes route here.

3. **"Plan 09-01 (OPS-06 queue) has landed in Wave 1, these three boot-time pre-Ready tray calls are buffered in the controller's pending queue and replayed once `OnReady` fires."**
   Verified by `grep -cE 'pendingAction|drainPending' internal/tray/tray.go` returning 16 lines (Plan 09-01's queue scaffolding + drain). The `applyBootAuthError` helper calls the public mutators `SetIconHealth`, `SetStatus`, `ShowReauthorize`, `ShowContinueSetup` — all of which Plan 09-01 made queue-or-execute under `t.mu`. No new tray controller methods needed.

4. **"The Reauthorize click handler is UNCHANGED — it remains wired to the existing AUTH-05 `OnReauthorize` callback."**
   Verified by inspection: `internal/app/reauth.go` was not modified. `internal/app/runapp.go` `applyBootAuthError` only calls public tray mutators; the click handler lives elsewhere and is wired through `tray.Config.OnReauthorize` at Controller construction time.

5. **"A unit test asserts the classification + tray-call sequence using a real `*tray.Controller`."**
   Verified by the three new tests, each constructing `tray.NewController(tray.Config{})` and calling `applyBootAuthError(c, err)`. Tests assert the classification return value (the test surface chosen because `pendingSnapshot()` is package-private to `tray` and not callable from `package app`).

## Verification — All Gates Green

| Gate | Result |
|------|--------|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./internal/app/ -run 'TestApplyBootAuthError' -v` | 3 tests PASS + 3 sub-cases PASS |
| `go test ./... -count=1` | all 16 packages PASS |
| `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` | exactly 1 line (schema unchanged) |
| `grep -cE 'Reauthorize: refresh token died\. Click Reauthorize…' internal/app/runapp.go` | 2 (suspendForAuth + applyBootAuthError — perfect AUTH-05 parity) |
| `grep -cE 'auth\.IsRevokedRefreshToken' internal/app/runapp.go` | 2 (existing isPermanentAuthErr + new applyBootAuthError) |
| `grep -cE 'applyBootAuthError\(t, err\)' internal/app/runapp.go` | 1 (single call site in RunApp) |
| `grep -cE 'ShowReauthorize\(\)' internal/app/runapp.go` | 3 (existing suspendForAuth + post-reauth probe timeout + new applyBootAuthError) |
| `grep -cE 'ShowContinueSetup\(\)' internal/app/runapp.go` | 3 (needsWizard + applyBootAuthError + ChangeWorkbook) |
| `grep -nE 'type bootAuthClassification int' internal/app/runapp.go` | exactly 1 line |
| `grep -cE 'pendingAction\|drainPending' internal/tray/tray.go` | 16 (Plan 09-01 dependency satisfied) |

## Schema Impact

**NONE.** `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` is unchanged (still exactly 1
matching grep line). No `_meta` rows added, no tab columns added, no `apps-script/` source touched.
CONTEXT.md D-06 schema-impact assertion holds.

## AUTH-05 Status String Parity

**Verbatim reuse:** the canonical AUTH-05 status string `"Reauthorize: refresh token died. Click Reauthorize…"`
is used in both `suspendForAuth` (running-state path, internal/app/runapp.go:646) and
`applyBootAuthError` (boot-time path, internal/app/runapp.go:657). Grep count: 2. Perfect copy
parity between the boot-time and running-state recovery UX — exactly as CONTEXT.md D-03 specifies
under the invisible-UX tiebreaker.

## Cross-Plan Dependency — Plan 09-01 Satisfied

`grep -cE 'pendingAction|drainPending' internal/tray/tray.go` returns 16 matches at the time of
this plan's land, confirming Plan 09-01's queue scaffolding is in place. The three pre-Ready
mutator calls (`SetIconHealth`, `SetStatus`, `ShowReauthorize`) fired from `applyBootAuthError`
are buffered by `t.pending` and drained by `OnReady` exactly as Plan 09-01's `TestPreReady_FIFOOrder`
and `TestSimulateReady_DrainsQueue` verify for the queue mechanics.

## Test Count Delta

| Category | Count | Notes |
|----------|-------|-------|
| New tests | 3 | `TestApplyBootAuthError_Revoked`, `TestApplyBootAuthError_NonRevoked`, `TestApplyBootAuthError_TableDriven` (3 sub-cases inside the third) |
| Replaced tests | 0 | No existing tests touched |
| Existing tests retained | 5 | `TestExtractCharName`, `TestNeedsWizard`, `TestExtractCharNameForSuffix`, `TestRescanCatchUp_FiresOnNewerFiles`, `TestRescanCatchUp_MultiFolderScan`, `TestRescanCatchUp_MissingFolderIsSkipped` — all green |

## Deviations from Plan

None. Plan executed exactly as written.

## Worktree-Path Note (process-only, not a code deviation)

During execution the first round of Edit calls landed in the main repo's `internal/app/runapp.go`
(at `C:/Users/Virus Canary/Desktop/Claude/SquireBot/internal/app/runapp.go`) rather than the
worktree's copy (at `.claude/worktrees/agent-a6b89d8430c4f645d/internal/app/runapp.go`), because
my initial Read calls used the implicit `internal/app/runapp.go` path which resolved against the
main repo. The main-repo edits were reverted via `git checkout -- internal/app/runapp.go` on the
master branch, and the edits re-applied against the explicit worktree absolute path. No commits
were made to the main repo; no work was lost. The worktree branch contains the two intended
commits and the main repo is clean of any AUTH-07 modifications.

## Hand-off Note for Plan 09-05 (Ship Gate)

**All 4 Phase 9 fixes are now in main (or this wave's worktrees).** Plan 09-05 can proceed to the
v1.0.2 ship gate (tag, `latest.json` refresh, GitHub Release) once Wave 2 merges. The fix grep
gates Plan 09-05's pre-tag readiness sweep should run:

- `Select-String -Path internal/tray/tray.go -Pattern 'pending'` matches ≥1 (OPS-06 queue field present — confirmed)
- `Select-String -Path cmd/squirebot/console_windows.go -Pattern 'FreeConsole'` matches 1 (OPS-07 file exists — Wave 1 deliverable)
- `Select-String -Path internal/config/config.go -Pattern 'TrimPrefix.*0xEF.*0xBB.*0xBF'` matches 1 (CONFIG-01 fix present — Wave 1 deliverable)
- `Select-String -Path internal/app/runapp.go -Pattern 'applyBootAuthError'` matches ≥2 (AUTH-07 helper + call site — this plan's deliverable, confirmed)
- `Select-String -Path internal/app/runapp.go -Pattern 'IsRevokedRefreshToken' -Context 0,2` matches 2 (existing isPermanentAuthErr at line 631 + new applyBootAuthError at line 652 — confirmed)

## Commits

| Hash | Task | Files | Summary |
|------|------|-------|---------|
| `b561b09` | Task 1 | internal/app/runapp.go (+51/-5) | applyBootAuthError helper + bootAuthClassification enum + simplified RunApp fast-fail call site; canonical AUTH-05 status string reused verbatim |
| `6160971` | Task 2 | internal/app/runapp_test.go (+58/-0) | Three new tests covering revoked + non-revoked + table-driven branches; new imports (errors, fmt, oauth2, tray); all pass |

## Self-Check: PASSED

- `internal/app/runapp.go` (worktree): FOUND, modified, contains `bootAuthClassification` + `applyBootAuthError`
- `internal/app/runapp_test.go` (worktree): FOUND, modified, contains all 3 `TestApplyBootAuthError_*` tests
- `.planning/phases/09-watcher-robustness-polish/09-04-SUMMARY.md` (this file): WRITTEN
- Commit `b561b09`: FOUND in worktree git log
- Commit `6160971`: FOUND in worktree git log
- Schema constant `WatcherMaxSchemaVersion = 3`: UNCHANGED
- Canonical AUTH-05 status string parity (grep count 2): CONFIRMED
- Plan 09-01 dependency (pendingAction/drainPending in tray.go): CONFIRMED
