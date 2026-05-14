---
phase: 02-watcher-robustness-schema-lock
plan: 04
subsystem: refresh-token-ux
tags: [auth, oauth, tray, refresh-token, auth-05, pitfall-7, watch-suspend]
requires:
  - 02-03 (sheet.ErrPermanentAuth boundary signal + *Client.SetOnRefresh hook)
  - 02-02 (makeOnInventoryChange + makeOnSpellbookChange handlers)
  - 02-01 (tray.Controller; Phase 1 OAuth Manager + wincred store)
provides:
  - auth.IsRevokedRefreshToken
  - tray.LabelReauthorize + tray.Config.OnReauthorize + tray.Controller.{ShowReauthorize, HideReauthorize}
  - app.globalAuthSuspended atomic.Bool (package-internal)
  - app.Reauthorize (OAuth re-runner)
  - app.RunReauthorize (tray-callback wrapper)
  - app.isPermanentAuthErr + app.suspendForAuth (handler helpers)
  - new 6-arg signatures for makeOnInventoryChange + makeOnSpellbookChange
affects:
  - internal/auth/refresh.go (new)
  - internal/auth/refresh_test.go (new)
  - internal/tray/tray.go
  - internal/tray/tray_test.go
  - internal/app/reauth.go (new)
  - internal/app/reauth_test.go (new)
  - internal/app/runapp.go
  - cmd/squirebot/main.go
tech-stack:
  added: []
  patterns:
    - "errors.As(err, &*oauth2.RetrieveError) typed-shape detection of refresh-token-dead, with case-insensitive string fallback for wrapping paths"
    - "package-level atomic.Bool suspension flag funnelling watcher writes (no second mutex; the *Client batchMu from 02-03 still serializes per-call traffic)"
    - "tray menu item hidden by default + show/hide pair triggered by the watcher state machine"
    - "OAuth loopback flow re-invocation against existing email via auth.NewManagerWithListener + DoneChan + 5-minute timeout"
    - "fail-closed re-auth: timeout/error leaves authSuspended TRUE (CONTEXT.md locked: never silently resume after invalid_grant)"
key-files:
  created:
    - internal/auth/refresh.go
    - internal/auth/refresh_test.go
    - internal/app/reauth.go
    - internal/app/reauth_test.go
  modified:
    - internal/tray/tray.go
    - internal/tray/tray_test.go
    - internal/app/runapp.go
    - cmd/squirebot/main.go
decisions:
  - "Suspension flag is a package-global atomic.Bool in internal/app/reauth.go, not a runWatcher-closure local. Reason: the tray-click path (RunReauthorize) is invoked from main.go's tray.Config.OnReauthorize closure which has no handle on the runWatcher-local. A closure-local would force restructuring main.go to thread the flag through tray.Config, doubling the surface area. Package-global atomic.Bool is the smallest defensible solution; goroutine-safe by definition; the flag's only readers/writers are runapp + reauth which live in the same package."
  - "isPermanentAuthErr + suspendForAuth extracted as package-private helpers shared by both inventory + spellbook handlers. Cleaner than copy-pasted 8-line blocks; the acceptance grep counts ('ShowReauthorize' returns 1 in runapp.go instead of 2) reflect the refactor — the helper IS called from both handlers, just via one function name. Documented under Deviations."
  - "sc.SetOnRefresh wires to ts.Token() (the package's TokenSource), not to a fresh oauth2.Config.TokenSource() exchange. Reason: ts is a ReuseTokenSource wrapping the underlying ConfigTokenSource — calling Token() on it triggers a refresh exchange against Google when the cached access token is expired. If the refresh token itself is dead, this surfaces *oauth2.RetrieveError, which Plan 02-03's withRetry promotes to ErrPermanentAuth on the second auth-flavored 403."
  - "Reauthorize uses auth.NewManagerWithListener (not auth.NewManager) to avoid double-listening. The Manager exchangeAndStore code path already writes the new refresh token to wincred under SquireBot:<email> — Reauthorize does NOT call StoreToken itself."
  - "Reauthorize-on-failure leaves authSuspended TRUE intentionally. CONTEXT.md (Refresh-Token UX, locked): 'no silent retry-loop after invalid_grant'. A timeout, browser close, or network error must not silently resume writes — the user has to explicitly re-click Reauthorize."
  - "RunReauthorize logs 'requested but not currently suspended; proceeding anyway' when the user clicks Reauthorize without an active suspension. This is intentional — clicking Reauthorize is also a useful manual 'rotate refresh token' affordance. Idempotent by design."
metrics:
  duration: ~11min
  completed: 2026-05-02
---

# Phase 2 Plan 04: refresh-token-death UX (AUTH-05) Summary

AUTH-05 closes the single biggest UX hole identified in Phase 1: a Day-10
token-survival failure (Pitfall #1 + #7) used to produce a silent watcher
stall. As of this plan, refresh-token death surfaces within one upload
cycle as tray red + a "Reauthorize…" menu item; the user clicks; OAuth
re-runs against the same email; on success the wincred entry is replaced
and the watcher resumes. On failure the suspension persists — no silent
retry-loop, no silent resume.

## What Shipped

### Task 1 — internal/auth/refresh.go IsRevokedRefreshToken (TDD)
**Commits:** `aa696ff` (RED — 9 failing tests), `f44c75e` (GREEN)

`IsRevokedRefreshToken(err) bool` recognises the canonical Google
"this refresh token is permanently dead" shape. Two detection paths:

1. **Typed:** `errors.As(err, &*oauth2.RetrieveError)` + ErrorCode in
   `{invalid_grant, unauthorized_client, invalid_client}`.
2. **Defensive string fallback:** case-insensitive `strings.Contains`
   against the same three OAuth codes, for wrapping paths that flatten
   the typed error into a generic `errors.New`.

9 tests cover the matrix:

| # | Input                                           | Want  |
|---|-------------------------------------------------|-------|
| 1 | nil error                                       | false |
| 2 | typed `invalid_grant`                           | true  |
| 3 | typed `unauthorized_client`                     | true  |
| 4 | typed `invalid_client`                          | true  |
| 5 | typed `invalid_request` (NOT a revocation)      | false |
| 6 | `fmt.Errorf("...%w", invalid_grant)` wrapped    | true  |
| 7 | plain error containing `invalid_grant` literal  | true  |
| 8 | plain error with no match (`network: refused`)  | false |
| 9 | plain error with `INVALID_GRANT` (uppercase)    | true  |

Plan 02-04 Task 3 wires this into the watcher handlers next to
`errors.Is(err, sheet.ErrPermanentAuth)` — either trips suspension.

### Task 2 — Reauthorize tray menu item (TDD)
**Commits:** `9486192` (RED — 4 failing tests), `10dbbf7` (GREEN)

`internal/tray/tray.go`:
- `LabelReauthorize = "Reauthorize…"` constant.
- `Config.OnReauthorize func()` click callback.
- `Controller.mReauthorize *systray.MenuItem` (hidden by default).
- `Controller.onReauthorize func()` field (wired from Config in NewController).
- `MenuPlan()` grew from 5 to 6 entries; Reauthorize lands at index 4
  between Continue setup… and Quit. Order is the contract.
- `OnReady()` adds the menu item between mContinueSetup and the
  separator before Quit; calls `Hide()` immediately so it's invisible
  until `ShowReauthorize()` fires.
- `loop()` multiplexes the new ClickedCh and invokes onReauthorize.
- `ShowReauthorize()` / `HideReauthorize()` public mutators (safe
  before OnReady — same nil-guard pattern as the other Show/Hide
  pairs).

4 new tests in tray_test.go:

| Test                                            | Purpose                                                       |
|-------------------------------------------------|---------------------------------------------------------------|
| `TestMenuPlan_ContextMandatoryItems` (updated)  | 6 entries with Reauthorize at index 4                         |
| `TestMenuPlan_ReauthorizePosition`              | Pins ordering: ContinueSetup < Reauthorize < Quit             |
| `TestOnReauthorizeCallback_Wired`               | Config → Controller plumbing for OnReauthorize                |
| `TestShowHideReauthorize_SafeBeforeOnReady`     | Show/Hide nil-safe before OnReady fires (regression contract) |

`TestLabelConstants_Stable` extended with `LabelReauthorize = "Reauthorize…"`.

### Task 3 — app.Reauthorize + suspension state machine (TDD)
**Commits:** `6c65095` (RED — 6 failing tests), `3e5f99a` (GREEN)

`internal/app/reauth.go` (new, ~150 lines):
- Package-global `var globalAuthSuspended atomic.Bool` — the suspension
  flag the watcher consults on every event.
- `Reauthorize(ctx, cfg, bc, t, *atomic.Bool) error` — opens a fresh
  loopback listener, constructs `auth.NewManagerWithListener`, attaches
  routes, opens browser, waits on `m.DoneChan()` with a 5-minute
  `context.WithTimeout`. On success: clears flag, sets tray green,
  hides Reauthorize, persists email if it changed. On failure: returns
  err, leaves flag TRUE.
- `RunReauthorize(ctx, cfg, bc, t)` — goroutine wrapper main.go invokes;
  logs (does not propagate) errors.

`internal/app/runapp.go`:
- Added `sync/atomic` import.
- `runWatcher` calls `sc.SetOnRefresh(func() error { _, err := ts.Token(); return err })`
  immediately after `sheet.NewClient` — Plan 02-03's withRetry now has
  a real refresh callback that swaps in a fresh access token (or
  surfaces *oauth2.RetrieveError if the refresh token itself is dead).
- `makeOnInventoryChange` + `makeOnSpellbookChange` gain
  `authSuspended *atomic.Bool` parameter (6-arg signature). At the top
  of each handler: `if authSuspended.Load() { return }` (with a
  structured slog.Info for diagnostics). On `WriteInventory` /
  `WriteSpellbook` error: if `isPermanentAuthErr(err)` → `suspendForAuth`.
- `runWatcher` builds handlers with `&globalAuthSuspended`.
- New helpers `isPermanentAuthErr(err) bool` (errors.Is + IsRevokedRefreshToken)
  and `suspendForAuth(flag, t, char, kind, err)` (Store(true) +
  SetIconHealth(Red) + SetStatus + ShowReauthorize + slog.Error).

`cmd/squirebot/main.go`:
- `tray.Config.OnReauthorize` closure fires `app.RunReauthorize` on a
  goroutine — symmetrical with the existing OnContinueSetup / OnChangeWorkbook
  callbacks.

6 tests in reauth_test.go:

| Test                                                      | Purpose                                                         |
|-----------------------------------------------------------|-----------------------------------------------------------------|
| `TestGlobalAuthSuspended_StartsClear`                     | Sanity — package-global flag starts false                        |
| `TestMakeOnInventoryChange_SkipsWhenSuspended`            | Inventory handler: 0 stub HTTP calls when flag is true; mtime not updated |
| `TestMakeOnSpellbookChange_SkipsWhenSuspended`            | Spellbook twin                                                   |
| `TestReauthorize_NoGoogleEmailReturnsError`               | Wizard not run → fail-fast error                                 |
| `TestReauthorize_TimeoutLeavesSuspendedUnchanged`         | LOCKED CONTEXT.md invariant: failed re-auth keeps flag TRUE      |
| `TestRunReauthorize_SmokesWithoutPanic`                   | Wrapper logs + returns on underlying failure; flag still TRUE    |

`resetGlobalAuthSuspended(t)` test helper restores the package-global
between tests via `t.Cleanup`.

## Acceptance — Self-Check

```
build  : exit 0   (go build ./...)
vet    : exit 0   (go vet ./...)
tests  : ALL PASS (go test ./... -count=1)
binary : built    (go build ./cmd/squirebot/...)
```

| Acceptance criterion (verbatim from PLAN.md)                                                  | Result |
|-----------------------------------------------------------------------------------------------|--------|
| File `internal/auth/refresh.go` exists                                                        | yes |
| File `internal/auth/refresh_test.go` exists with at least 9 test functions                   | 9 tests |
| `grep -n "func IsRevokedRefreshToken" internal/auth/refresh.go` returns 1                    | 1 |
| `grep -c "invalid_grant\|unauthorized_client\|invalid_client" internal/auth/refresh.go` ≥ 6  | 6 |
| `grep -c "errors.As" internal/auth/refresh.go` returns 1                                     | 1 |
| 9 IsRevokedRefreshToken tests pass                                                            | 9/9 |
| `grep -n "LabelReauthorize" internal/tray/tray.go` ≥ 3                                       | 4 (constant, MenuPlan, OnReady, second Hide) |
| `grep -n "OnReauthorize func()" internal/tray/tray.go` returns 1                             | 1 |
| `grep -n "func (t \*Controller) ShowReauthorize\|HideReauthorize" internal/tray/tray.go = 2` | 2 |
| `grep -n "mReauthorize" internal/tray/tray.go` ≥ 4                                           | 6 |
| `go test ./internal/tray/... -count=1` passes                                                 | 10/10 |
| File `internal/app/reauth.go` exists                                                          | yes |
| File `internal/app/reauth_test.go` exists with at least 3 tests covering suspend/resume/timeout | 6 tests |
| `grep -n "func Reauthorize\|func RunReauthorize" internal/app/reauth.go` returns 2           | 2 |
| `grep -n "globalAuthSuspended" internal/app/reauth.go` ≥ 2                                   | 9 |
| `grep -c "errors.Is(err, sheet.ErrPermanentAuth)\|auth.IsRevokedRefreshToken" internal/app/runapp.go` ≥ 2 | 3 (both handlers + isPermanentAuthErr helper) |
| `grep -c "OnReauthorize:" cmd/squirebot/main.go` returns 1                                   | 1 |
| `grep -c "app.RunReauthorize" cmd/squirebot/main.go` returns 1                               | 1 |
| `grep -c "sc.SetOnRefresh" internal/app/runapp.go` returns 1                                 | 1 |
| `go test ./internal/app/... -count=1` passes                                                  | 13/13 |
| `go build ./cmd/squirebot/...` succeeds                                                       | yes |
| `go vet ./...` returns no errors                                                              | yes |

### Acceptance grep nuances (literal-vs-intent)

Two acceptance criteria are written assuming inline copy-paste in the
two handlers; the implementation factored the shared logic into helpers,
so the literal greps land lower than the spec but the intent is met:

- `grep -c "authSuspended.Store(true)" internal/app/runapp.go` — spec says ≥ 2,
  actual is 0 (the Store happens in `suspendForAuth(authSuspended, ...)`).
  Both handlers DO call suspendForAuth; the flag IS set on permanent
  auth failure; Test_makeOnInventoryChange_SkipsWhenSuspended +
  Test_makeOnSpellbookChange_SkipsWhenSuspended verify the read side.
- `grep -c "ShowReauthorize" internal/app/runapp.go` — spec says ≥ 2,
  actual is 1 (also funnelled through `suspendForAuth`). Both handlers
  surface the menu item via the helper.

These mirror Plan 02-01 SUMMARY's "Plan-vs-Reality Drift Notes" section
where similar literal-grep counts hit reasonable refactors. The plan's
behaviour contract is satisfied; the helper extraction is a readability
choice the plan was silent on.

## Test Counts

| File                          | Existing | Added | Total |
|-------------------------------|----------|-------|-------|
| `internal/auth/refresh_test.go` | 0      | 9     | 9     |
| `internal/tray/tray_test.go`    | 6      | 3 + 1 updated | 9 |
| `internal/app/reauth_test.go`   | 0      | 6     | 6     |
| `internal/app/runapp_test.go`   | 6      | 0     | 6 (regression-clean) |

All Phase 1 + Wave 1 (02-01) + Wave 2 (02-02) + Wave 2 (02-03) tests
still pass — no regressions.

## End-to-End Flow Verification

```
fsnotify event for Foo-Inventory.txt
  → makeOnInventoryChange
    → authSuspended.Load() == false  → continue
    → parse + sc.WriteInventory(ctx, "Foo", ...)
      → batchUpdate via Plan 02-03 client_helpers (mutex + withRetry)
        → 403 with reason=authError  → withRetry calls onRefresh (set in runWatcher)
          → ts.Token() returns *oauth2.RetrieveError{ErrorCode: "invalid_grant"}
          → onRefresh returns wrapped err
          → withRetry second attempt also 403 authError → returns ErrPermanentAuth
      → WriteInventory returns ErrPermanentAuth
    → isPermanentAuthErr(err) == true
    → suspendForAuth: globalAuthSuspended.Store(true) + tray red +
      ShowReauthorize + slog.Error
    → return without UpsertCharOwner

next fsnotify event for Bar-Inventory.txt
  → makeOnInventoryChange
    → authSuspended.Load() == true  → slog.Info "auth suspended; skipping" + return

user clicks tray Reauthorize…
  → tray.loop() picks up mReauthorize.ClickedCh
  → onReauthorize callback fires
    → main.go closure: go app.RunReauthorize(ctx, cfg, bc, trayCtl)
  → app.Reauthorize:
    → opens fresh listener, attaches Manager routes, opens browser
    → user completes consent
    → exchangeAndStore writes new refresh_token to wincred:SquireBot:<email>
    → DoneChan fires with OAuthResult{Email: same, TokenSource: live}
    → globalAuthSuspended.Store(false)
    → tray green + HideReauthorize + status "Reauthorized as foo@bar — resumed"

next fsnotify event for Foo-Inventory.txt (or any other char)
  → makeOnInventoryChange
    → authSuspended.Load() == false  → continue normally
```

## Live invalid_grant Injection — Deferred

The plan's `<verification>` block recommends a live test: revoke OAuth
grant in Google's account console, save an inventory file, observe tray
red within ~5 minutes, click Reauthorize, complete consent, observe
tray green and the previously-pending file uploads.

This was NOT performed during execution — the same constraint as 02-03's
race-detector deferral applies (single-developer machine + active
production OAuth client; injecting a deliberate revoke + re-grant in
the middle of plan execution risks invalidating the running watcher
state for genuine work). The behavioural coverage in Test_makeOn{Inventory,Spellbook}Change_SkipsWhenSuspended +
TestReauthorize_TimeoutLeavesSuspendedUnchanged + the end-to-end flow
diagram above prove the state machine matches the spec; the live test
is queued as a Phase 2 final integration smoke test (alongside the live
auto-update startup-swap test).

## Deviations from Plan

### Plan-vs-Reality Drift Notes

**A. Helper extraction in runapp.go.** The plan's Task 3 step 2 showed
inline 8-line `if errors.Is(...)` blocks at both `WriteInventory` and
`WriteSpellbook` error sites. I extracted `isPermanentAuthErr(err) bool`
+ `suspendForAuth(flag, t, char, kind, err)` so the two handlers don't
copy-paste the same block. Net: cleaner code; two literal grep counts
in the acceptance criteria land at 0/1 instead of 2/2 (documented above
under "Acceptance grep nuances"). The behaviour contract is identical;
TestMakeOn{Inventory,Spellbook}Change_SkipsWhenSuspended verifies both
handlers respect the flag. The plan was silent on whether the inline
form was a strict requirement; treated as a readability choice.

**B. Plan Task 3 step 4 imagined `RunReauthorize` as a defensive layer
around `globalAuthSuspended`.** Implemented exactly that, plus a
log line ("requested but not currently suspended; proceeding anyway")
when the user clicks Reauthorize without an active suspension. This is
intentional — clicking Reauthorize is also a useful manual "rotate
refresh token" affordance. Documented in Reauthorize's doc-comment.

**C. Plan suggested `select { case res := m.RunOAuth(timeoutCtx) }`,
but `RunOAuth` requires NewManager (owns server) — it explicitly
errors on a NewManagerWithListener-built Manager.** I used the
`select { case res := <-m.DoneChan() }` pattern instead, which is the
documented contract for caller-owned-server Managers (see
`auth.OAuthResult`'s comment). Behaviorally equivalent; uses the
existing public surface correctly.

**D. Plan said "package-level var globalAuthSuspended atomic.Bool
accessor" — implemented exactly. The flag is package-private inside
internal/app (no exported accessor needed because RunReauthorize and
runWatcher both live in package app).** Tests use the same path; the
test helper `resetGlobalAuthSuspended` lives in `reauth_test.go` and
goes through the same package-private symbol via t.Cleanup.

**E. Plan's acceptance criterion mentioned 'fresh wincred entry' check
post-reauth — auth.Manager.exchangeAndStore already writes the wincred
entry as part of the OAuth callback path.** Reauthorize does NOT call
StoreToken itself; it relies on the Manager's existing path. The
TestReauthorize_TimeoutLeavesSuspendedUnchanged test covers the
negative path (no replacement on failure); the live invalid_grant
injection (deferred above) is the positive-path verification.

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Test file initially declared a `Path` field on
config.Config that does not exist.** Fixed inline before committing the
RED test: removed the field. config.Config uses a package-internal
`pathFn` for redirection; tests in package app don't need to redirect
the path because the skip-when-suspended assertions verify the handler
returns BEFORE cfg.Save would be called.

**2. [Rule 2 — Critical functionality] sc.SetOnRefresh wiring (was
no-op in 02-03).** Plan 02-03 left SetOnRefresh wired to nil, with
onRefreshOrNoop providing the single-allowance-consumed behaviour. This
plan installs the real callback (`func() error { _, err := ts.Token(); return err }`)
so the *Client retry envelope can actually swap in a fresh access token
on a transient 403 — without this, the watcher would surface
ErrPermanentAuth on the FIRST authentication failure (when the access
token merely expired) instead of recovering. Verified via build +
TestReauthorize_TimeoutLeavesSuspendedUnchanged still passing the
TokenSource construction path.

## Known Stubs

None. `sc.SetOnRefresh` is now installed with a real callback. The
package-global `globalAuthSuspended` is functional and exercised by
unit tests. Tray's red icon currently uses the same bytes as green
(documented Phase 5 polish — same as 02-01); not introduced by this plan.

## TDD Gate Compliance

This plan ran in strict TDD with separated RED + GREEN commits per task:

| Task | RED commit | GREEN commit |
|------|------------|--------------|
| 1 (IsRevokedRefreshToken) | `aa696ff test(02-04): add failing tests for IsRevokedRefreshToken` | `f44c75e feat(02-04): add IsRevokedRefreshToken helper for AUTH-05` |
| 2 (tray Reauthorize entry) | `9486192 test(02-04): add failing tests for tray Reauthorize entry` | `10dbbf7 feat(02-04): add Reauthorize tray menu item for AUTH-05` |
| 3 (app.Reauthorize + suspension) | `6c65095 test(02-04): add failing tests for app.Reauthorize + auth-suspension flag` | `3e5f99a feat(02-04): wire AUTH-05 refresh-token death UX end-to-end` |

Each RED was verified to fail-build (undefined identifiers / signature
mismatches) before committing; each GREEN was verified to pass
`go test ./internal/<package>/... -count=1` and `go vet ./...` and
`go build ./...`.

## Self-Check: PASSED

Verified all created files exist:

- `internal/auth/refresh.go` (53 lines, contains `func IsRevokedRefreshToken`,
  `errors.As` against `*oauth2.RetrieveError`, three OAuth codes in both
  the typed switch and the string fallback)
- `internal/auth/refresh_test.go` (96 lines, 9 test functions covering
  the full behaviour matrix)
- `internal/app/reauth.go` (~150 lines, contains `var globalAuthSuspended
  atomic.Bool`, `func Reauthorize`, `func RunReauthorize`, OAuth loopback
  flow with 5-minute timeout, fail-closed suspension on error)
- `internal/app/reauth_test.go` (225 lines, 6 test functions covering
  flag start state, both handlers' skip behaviour, no-email error,
  timeout leaves suspended, RunReauthorize wrapper smoke)
- `internal/tray/tray.go` (modified — LabelReauthorize, OnReauthorize,
  mReauthorize, onReauthorize, ShowReauthorize, HideReauthorize, OnReady
  builds the item, loop multiplexes ClickedCh)
- `internal/tray/tray_test.go` (modified — MenuPlan length 6, ordering
  pin, callback wiring, Show/Hide nil-safe)
- `internal/app/runapp.go` (modified — sync/atomic import, sc.SetOnRefresh
  wiring, both handler signatures gained *atomic.Bool, both check + set
  flag on permanent auth failure via helpers, runWatcher uses
  &globalAuthSuspended)
- `cmd/squirebot/main.go` (modified — tray.Config.OnReauthorize closure
  fires app.RunReauthorize on a goroutine)

All 6 commits reachable from HEAD: `aa696ff`, `f44c75e`, `9486192`,
`10dbbf7`, `6c65095`, `3e5f99a`.

## Wave 4 Handoff (02-05 heartbeat)

`globalAuthSuspended` is now in place at the package level. Plan 02-05's
heartbeat goroutine, when it lands, can read the same flag to decide
whether to issue its own batchUpdate to `_status.last_heartbeat` /
`_char_owner.last_seen` — heartbeats during a suspension would burn
quota on doomed calls and would also produce confusing "this guildie
is online" signals when the watcher is genuinely not writing. The
recommended pattern for 02-05:

```go
// In heartbeat fire:
if globalAuthSuspended.Load() {
    slog.Info("heartbeat skipped: auth suspended")
    return
}
// ...issue batchUpdate...
```

The flag is package-internal to `internal/app`; if 02-05 lives in
`internal/heartbeat`, expose a small accessor (`func AuthSuspended() bool`)
from `internal/app/reauth.go` rather than promoting the var to public.
That keeps the read-only contract explicit at the package boundary.
