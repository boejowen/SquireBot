---
phase: 09-watcher-robustness-polish
plan: 04
plan_id: 09-04-auth07-boot-invalidgrant
type: execute
wave: 2
depends_on:
  - 09-01-tray-prereadyqueue
files_modified:
  - internal/app/runapp.go
  - internal/app/runapp_test.go
autonomous: true
requirements: [AUTH-07]
tags: [auth, runapp, auth-07, boot-time, invalid-grant, wave2]

must_haves:
  truths:
    - "When `RunApp` fast-fails on a cold-start `buildTokenSourceFromWincred` error AND that error matches `auth.IsRevokedRefreshToken(err)`, the controller receives (in order) `SetIconHealth(HealthRed)`, `SetStatus(\"Reauthorize: refresh token died. Click Reauthorize…\")`, `ShowReauthorize()` — mirroring AUTH-05's canonical `suspendForAuth` triple (runapp.go:628-637) for symmetry."
    - "When `RunApp` fast-fails on a cold-start `buildTokenSourceFromWincred` error that does NOT match `auth.IsRevokedRefreshToken`, the existing ContinueSetup-style recovery is preserved (red icon + status with the raw error + ShowContinueSetup) — non-revoked wincred breakage routes to wizard re-entry, not to Reauthorize."
    - "Because Plan 09-01 (OPS-06 queue) has landed in Wave 1, these three boot-time pre-Ready tray calls are buffered in the controller's pending queue and replayed once `OnReady` fires — the tray menu opens already showing the auth-error state with a clickable Reauthorize, no transient \"Initialising…\" window."
    - "The Reauthorize click handler is UNCHANGED — it remains wired to the existing AUTH-05 `OnReauthorize` callback (`app.RunReauthorize` flow) which on success swaps the wincred token, re-enters RunApp, and clears the red icon."
    - "A unit test asserts the classification + tray-call sequence using a real `*tray.Controller` (test mode, no systray.Run) and `pendingSnapshot()` from Plan 09-01."
  artifacts:
    - path: internal/app/runapp.go
      provides: "Modified ts == nil cold-start fast-fail block: branches on auth.IsRevokedRefreshToken(err) to mirror AUTH-05 canonical tray triple, else preserves existing ContinueSetup path. Plus a small applyBootAuthError helper to make the branch unit-testable."
      contains: "auth.IsRevokedRefreshToken(err)"
    - path: internal/app/runapp_test.go
      provides: "TestApplyBootAuthError_Revoked + TestApplyBootAuthError_NonRevoked — table-driven coverage of the boot classifier"
      contains: "TestApplyBootAuthError"
  key_links:
    - from: "internal/app/runapp.go cold-start fast-fail block (~lines 102-112)"
      to: "internal/auth/refresh.go IsRevokedRefreshToken classifier"
      via: "direct call on rebuild error"
      pattern: "auth\\.IsRevokedRefreshToken\\(err\\)"
    - from: "applyBootAuthError → tray.Controller.{SetIconHealth, SetStatus, ShowReauthorize}"
      to: "Plan 09-01 pending queue → OnReady drain"
      via: "pre-Ready tray calls"
      pattern: "ShowReauthorize\\(\\)"
---

<objective>
Close the Phase 6 UAT Finding C foot-gun: when a guildie's refresh token is revoked between sessions (e.g., they signed the SquireBot app out of their Google Account, the OAuth grant expired in Testing-mode-prior-to-Production, or the token was administratively invalidated), the boot-time `buildTokenSourceFromWincred` returns an `invalid_grant` error. Today this lands the user at "Initialising…" forever — the AUTH-05 Reauthorize UX only surfaces after a running-state Sheets API call fails.

Per CONTEXT.md D-03 (locked under invisible-UX tiebreaker), classify the boot-time rebuild error using the SAME `auth.IsRevokedRefreshToken` classifier that AUTH-05 uses, and on match fire the SAME tray triple (`SetIconHealth(HealthRed)` + `SetStatus("Reauthorize: refresh token died. Click Reauthorize…")` + `ShowReauthorize()`) BEFORE `RunApp` returns. Plan 09-01 (Wave 1) buffers these pre-Ready calls in the controller's pending queue; `OnReady` drains them and the tray menu opens already in the auth-error state.

Per planner guidance in the planning context: use the **canonical AUTH-05 status string** (`"Reauthorize: refresh token died. Click Reauthorize…"`) verbatim — perfect parity with the running-state path keeps the user experience symmetric and avoids introducing a copy variant.

**Cross-plan dependency (load-bearing, per CONTEXT.md specifics §2):** This plan MUST land AFTER Plan 09-01. Without the OPS-06 queue, the three boot-time `t.SetIconHealth` / `t.SetStatus` / `t.ShowReauthorize` calls land in the pre-Ready silent-no-op window (the bug AUTH-07 is meant to fix) and the requirement fails acceptance. Wave 1 (09-01, 09-02, 09-03) blocks Wave 2 (09-04).

**Scope discipline (per CONTEXT.md domain section + D-07):**
- Only modify the cold-start `ts == nil` block in `RunApp` (~runapp.go:102-112). Do NOT refactor `RunApp` more broadly.
- Reuse the existing `auth.IsRevokedRefreshToken` classifier — do NOT create a new classifier or expand `isPermanentAuthErr` (which would broaden the trigger surface beyond what's needed for boot).
- Do NOT touch the Reauthorize CLICK handler — it lives in `internal/app/reauth.go` and is shared with AUTH-05 unchanged.
- Do NOT add new tray controller methods — Plan 09-01 makes the existing methods sufficient on the pre-Ready path.
- No schema changes (D-06): `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` MUST remain unchanged.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/09-watcher-robustness-polish/09-CONTEXT.md
@.planning/phases/09-watcher-robustness-polish/09-PATTERNS.md
@.planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md
@CLAUDE.md
@internal/app/runapp.go
@internal/app/runapp_test.go
@internal/tray/tray.go

<interfaces>
<!-- Exact insertion site, classifier signature, canonical AUTH-05 triple. Extracted from runapp.go. -->

internal/app/runapp.go cold-start fast-fail block (lines 102-112) — CURRENT shape:

```go
// Watcher path. If we came through the wizard, ts is live; otherwise
// (skip-wizard cold start) we rebuild it from wincred.
if ts == nil {
    built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
    if err != nil {
        slog.Error("token rebuild from wincred failed", "err", err)
        t.SetStatus(fmt.Sprintf("Auth error: %v", err))
        t.SetIconHealth(tray.HealthRed)
        t.ShowContinueSetup()
        return
    }
    ts = built
}
```

internal/app/runapp.go AUTH-05 canonical triple (suspendForAuth, lines 628-637):

```go
func suspendForAuth(authSuspended *atomic.Bool, t *tray.Controller, charName, kind string, err error) {
    if authSuspended != nil {
        authSuspended.Store(true)
    }
    slog.Error("permanent auth failure — suspending writes",
        "char", charName, "kind", kind, "err", err)
    t.SetIconHealth(tray.HealthRed)
    t.SetStatus("Reauthorize: refresh token died. Click Reauthorize…")
    t.ShowReauthorize()
}
```

The canonical status string is: `"Reauthorize: refresh token died. Click Reauthorize…"`.

internal/app/runapp.go isPermanentAuthErr (lines ~614-622, for reference only — DO NOT use for boot path):

```go
func isPermanentAuthErr(err error) bool {
    if err == nil { return false }
    if errors.Is(err, sheet.ErrPermanentAuth) { return true }
    return auth.IsRevokedRefreshToken(err)
}
```

The boot path uses `auth.IsRevokedRefreshToken(err)` DIRECTLY (per CONTEXT.md D-03 paragraph "do NOT reuse isPermanentAuthErr") — boot cannot produce `sheet.ErrPermanentAuth` (no Sheets call has happened yet).

internal/auth classifier signature (canonical — per pattern map and existing runapp.go:621):

```go
func IsRevokedRefreshToken(err error) bool
```

Located in package `auth`; already imported by runapp.go.

internal/tray Controller method signatures (Plan 09-01 has now made these all queue-safe pre-Ready):
```go
func (t *Controller) SetIconHealth(h Health)
func (t *Controller) SetStatus(s string)
func (t *Controller) ShowReauthorize()
func (t *Controller) ShowContinueSetup()
```

Plan 09-01 test surface (test-only, package tray):
```go
func (t *Controller) pendingSnapshot() []pendingAction
func (t *Controller) isReady() bool
```
Used by tests in `package tray`. NOT accessible from `package app` (different package). For `runapp_test.go` (package app), test the helper directly with a real `*tray.Controller` constructed via `tray.NewController(tray.Config{})` and assert by:
- Calling `tray.SimulateReady()` if exported, OR
- Constructing a custom `*tray.Controller` via `tray.NewController(tray.Config{})` and observing the controller's externally-visible state via SAFE side effects (status string via a test-only exported helper).

**Cross-package observability problem:** `pendingSnapshot()` is lowercase (package-private to `tray`). Plan 09-01 keeps it that way. To unit-test the boot classifier from `package app`, the runapp_test.go test must use a different approach: extract the classification logic into a tiny pure helper that returns a discriminated result (e.g., an enum or a struct), and test THAT helper without exercising the tray controller.

Recommended pattern (see Task 2): the helper returns a `bootAuthClassification` enum (`bootAuthRevoked`, `bootAuthOther`) plus the canonical status string; the call site in `RunApp` switches on the enum and fires the appropriate tray triple. The test exercises the helper only.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add applyBootAuthError() helper to runapp.go (refactor + classify + dispatch)</name>
  <files>internal/app/runapp.go</files>
  <read_first>
    - internal/app/runapp.go (READ ENTIRELY — 735 LOC; confirm the cold-start block at lines 102-112, the suspendForAuth helper at lines 628-637, the isPermanentAuthErr helper at lines 614-622, and verify `auth` is already imported)
    - internal/auth/refresh.go (find IsRevokedRefreshToken; confirm signature; do NOT modify)
    - internal/tray/tray.go (verify the Controller method signatures referenced — should be unchanged from Plan 09-01 land)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Plan 09-04" (analog: cold-start fast-fail at 102-112; AUTH-05 triple at 628-637; classifier reuse at 614-622)
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-03 (rationale + AUTH-05 status string parity)
    - .planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md (confirm Plan 09-01 has landed and pre-Ready calls are queued)
  </read_first>
  <behavior>
    - `applyBootAuthError(t *tray.Controller, err error)` is a private helper inside `package app` that:
      - logs `slog.Error("token rebuild from wincred failed", "err", err)` (preserves the existing log line semantics)
      - if `auth.IsRevokedRefreshToken(err)` returns true: fires `t.SetIconHealth(tray.HealthRed)` then `t.SetStatus("Reauthorize: refresh token died. Click Reauthorize…")` then `t.ShowReauthorize()` (mirrors AUTH-05 suspendForAuth exactly, in the same order)
      - otherwise: fires `t.SetStatus(fmt.Sprintf("Auth error: %v", err))` then `t.SetIconHealth(tray.HealthRed)` then `t.ShowContinueSetup()` (preserves the EXISTING non-revoked path verbatim)
    - The cold-start fast-fail block in `RunApp` (lines 102-112) is simplified to: on error, call `applyBootAuthError(t, err)` then `return`.
    - The helper is small enough that the classification logic can be unit-tested by passing an `*tray.Controller` and inspecting downstream side effects. To make the test deterministic, the helper should ALSO return a small enum `bootAuthClassification` indicating which branch ran — this lets the test assert classification without depending on the (now-queued) tray side effects.
  </behavior>
  <action>
    Open `internal/app/runapp.go`. Make exactly these changes:

    1. Add a new type + enum near the top of the file, alongside any other package-level type declarations (search for `type ` declarations; place near the existing utility types around lines 50-100):

       ```go
       // bootAuthClassification distinguishes a revoked-refresh-token boot error
       // (Reauthorize path) from any other rebuild failure (ContinueSetup path).
       // Plan 09-04 / AUTH-07.
       type bootAuthClassification int

       const (
       	bootAuthOther bootAuthClassification = iota
       	bootAuthRevoked
       )
       ```

    2. Add a new helper function placed JUST BEFORE the existing `suspendForAuth` function (search for `func suspendForAuth(`; insert above it):

       ```go
       // applyBootAuthError handles a cold-start buildTokenSourceFromWincred
       // failure. If the error matches the revoked-refresh-token classifier
       // (same one AUTH-05 uses), it mirrors AUTH-05's canonical
       // SetIconHealth(Red) + SetStatus + ShowReauthorize tray triple
       // (suspendForAuth, runapp.go ~lines 628-637) so the user sees a clickable
       // Reauthorize from boot. Otherwise it preserves the original
       // ContinueSetup-style recovery (red icon + raw error in status + wizard
       // re-entry).
       //
       // Plan 09-01 (OPS-06) buffers these pre-Ready tray calls so they replay
       // in OnReady — the tray menu opens already in the auth-error state.
       //
       // Returns the classification so unit tests can assert which branch ran
       // without depending on tray internals.
       //
       // Plan 09-04 / AUTH-07. Phase 6 UAT Finding C.
       func applyBootAuthError(t *tray.Controller, err error) bootAuthClassification {
       	slog.Error("token rebuild from wincred failed", "err", err)
       	if auth.IsRevokedRefreshToken(err) {
       		// AUTH-07: boot-time invalid_grant. Mirror AUTH-05's
       		// suspendForAuth triple exactly so the running-state and
       		// boot-time recovery UX are identical.
       		t.SetIconHealth(tray.HealthRed)
       		t.SetStatus("Reauthorize: refresh token died. Click Reauthorize…")
       		t.ShowReauthorize()
       		return bootAuthRevoked
       	}
       	// Non-revoked wincred breakage (corrupted credential, machine
       	// migration, etc.) — route to wizard re-entry per the original
       	// pre-AUTH-07 behavior.
       	t.SetStatus(fmt.Sprintf("Auth error: %v", err))
       	t.SetIconHealth(tray.HealthRed)
       	t.ShowContinueSetup()
       	return bootAuthOther
       }
       ```

    3. Replace the cold-start fast-fail block currently at lines 102-112:

       ```go
       // Watcher path. If we came through the wizard, ts is live; otherwise
       // (skip-wizard cold start) we rebuild it from wincred.
       if ts == nil {
           built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
           if err != nil {
               slog.Error("token rebuild from wincred failed", "err", err)
               t.SetStatus(fmt.Sprintf("Auth error: %v", err))
               t.SetIconHealth(tray.HealthRed)
               t.ShowContinueSetup()
               return
           }
           ts = built
       }
       ```

       with:

       ```go
       // Watcher path. If we came through the wizard, ts is live; otherwise
       // (skip-wizard cold start) we rebuild it from wincred. AUTH-07 (Plan
       // 09-04): classify the rebuild error so revoked refresh tokens surface
       // the Reauthorize path instead of ContinueSetup. Pre-Ready tray calls
       // are buffered by Plan 09-01 (OPS-06).
       if ts == nil {
           built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
           if err != nil {
               _ = applyBootAuthError(t, err)
               return
           }
           ts = built
       }
       ```

       The `_ =` discard is intentional — `RunApp` itself doesn't act on the classification; only the unit test does.

    4. Do NOT modify `suspendForAuth`, `isPermanentAuthErr`, or any other function. Do NOT change `package app`'s import list (you should already have `errors`, `fmt`, `log/slog`, the `auth` package, the `tray` package — confirm by reading the imports; if `auth` or `tray` is not imported in runapp.go, this is unexpected and you must stop and surface it; per pattern map line 374 `auth.IsRevokedRefreshToken` is already used at runapp.go:621 so the import IS present).

    Schema-impact assertion (per CONTEXT.md D-06): `internal/sheet/client.go` not touched by this task.
  </action>
  <verify>
    <automated>go build ./internal/app/... && go vet ./internal/app/...</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE 'type bootAuthClassification int' internal/app/runapp.go` matches exactly 1 line.
    - `grep -nE 'bootAuthRevoked bootAuthClassification' internal/app/runapp.go` returns 0 (the constant is in an iota block — verify the iota block instead).
    - `grep -nE 'bootAuthOther bootAuthClassification = iota' internal/app/runapp.go` matches exactly 1 line.
    - `grep -nE 'bootAuthRevoked' internal/app/runapp.go` returns at least 2 lines (constant declaration + helper return statement).
    - `grep -nE 'func applyBootAuthError\(t \*tray\.Controller, err error\) bootAuthClassification' internal/app/runapp.go` matches exactly 1 line.
    - `grep -nE 'auth\.IsRevokedRefreshToken\(err\)' internal/app/runapp.go` matches AT LEAST 2 lines (the existing isPermanentAuthErr call at line 621 + the new applyBootAuthError call).
    - `grep -cE 'Reauthorize: refresh token died\. Click Reauthorize…' internal/app/runapp.go` returns AT LEAST 2 (existing suspendForAuth + new applyBootAuthError — verify the canonical AUTH-05 string is reused verbatim).
    - `grep -cE 'applyBootAuthError\(t, err\)' internal/app/runapp.go` returns 1 (the call site in RunApp).
    - `grep -cE 'ShowReauthorize\(\)' internal/app/runapp.go` returns AT LEAST 2 (existing suspendForAuth + new applyBootAuthError).
    - `grep -cE 'ShowContinueSetup\(\)' internal/app/runapp.go` returns AT LEAST 1 (the non-revoked branch preserved).
    - The old code line `t.SetStatus(fmt.Sprintf("Auth error: %v", err))` MUST still appear (now inside applyBootAuthError's non-revoked branch): `grep -cE 'Auth error: %v' internal/app/runapp.go` returns AT LEAST 1.
    - `go build ./internal/app/...` exits 0.
    - `go vet ./internal/app/...` exits 0.
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (unchanged).
  </acceptance_criteria>
  <done>applyBootAuthError helper + bootAuthClassification enum + simplified RunApp fast-fail call site land in runapp.go; canonical AUTH-05 status string reused verbatim; classifier directly uses auth.IsRevokedRefreshToken (not isPermanentAuthErr); build + vet clean; schema constant unchanged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add unit tests covering the boot auth classifier (revoked + non-revoked branches)</name>
  <files>internal/app/runapp_test.go</files>
  <read_first>
    - internal/app/runapp_test.go (full file — read all existing tests + helpers + imports; understand the `NeedsWizard` test pattern at lines 45-68 as a table-driven analog)
    - internal/app/runapp.go (the freshly modified file from Task 1; confirm applyBootAuthError exists and its signature)
    - internal/auth/refresh.go (find sample error shapes that match IsRevokedRefreshToken; the file should expose either the classifier itself or a doc/comment about what shapes match)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Plan 09-04" subsection on test approach (the helper is the unit-test surface; sample matchable errors at PATTERNS.md lines ~470-475)
    - .planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md (confirm tray controller can be constructed offline via `tray.NewController(tray.Config{})`)
  </read_first>
  <behavior>
    - TestApplyBootAuthError_Revoked: constructs a `*tray.Controller` via `tray.NewController(tray.Config{})`, calls `applyBootAuthError(c, revokedErr)`, asserts the return value is `bootAuthRevoked`. The revokedErr is a synthetic error that matches `auth.IsRevokedRefreshToken` (use either `&oauth2.RetrieveError{ErrorCode: "invalid_grant"}` if oauth2 is already imported, OR an error wrapping a message the classifier matches — verify the exact shape by reading internal/auth/refresh.go).
    - TestApplyBootAuthError_NonRevoked: constructs a `*tray.Controller`, calls `applyBootAuthError(c, errors.New("some unrelated wincred error"))`, asserts the return is `bootAuthOther`.
    - Table-driven structure mirrors `TestNeedsWizard` at runapp_test.go:45-68.
    - Tests are PURE classifier tests — they do not assert tray side effects (those depend on Plan 09-01's queue and are exercised in Plan 09-01's own tests). The classifier return value IS the test surface.
    - One additional defensive test: TestApplyBootAuthError_NilTray would be useful but the Controller is not nil-safe by contract; skip it to avoid introducing a nil-safety requirement out of scope.
  </behavior>
  <action>
    Open `internal/app/runapp_test.go` and add a new test block. First, verify what `auth.IsRevokedRefreshToken` matches against by reading `internal/auth/refresh.go` (or its test file `internal/auth/refresh_test.go` if it exists). Per PATTERNS.md the classifier matches both `*oauth2.RetrieveError` with `ErrorCode == "invalid_grant"` AND string-form errors containing "invalid_grant".

    Use this test approach — it does NOT depend on the exact internal shape of IsRevokedRefreshToken; instead it uses the SAFEST guaranteed-matching shape (a `*oauth2.RetrieveError`):

    ```go
    // TestApplyBootAuthError_Revoked verifies AUTH-07 boot classification: when
    // buildTokenSourceFromWincred returns an error matching
    // auth.IsRevokedRefreshToken, applyBootAuthError returns bootAuthRevoked.
    // Plan 09-04. Phase 6 UAT Finding C.
    func TestApplyBootAuthError_Revoked(t *testing.T) {
    	// Synthetic revoked-token error. Matches auth.IsRevokedRefreshToken
    	// via the oauth2 RetrieveError + ErrorCode "invalid_grant" path
    	// (see internal/auth/refresh.go IsRevokedRefreshToken).
    	revokedErr := &oauth2.RetrieveError{ErrorCode: "invalid_grant"}

    	c := tray.NewController(tray.Config{})
    	got := applyBootAuthError(c, revokedErr)
    	if got != bootAuthRevoked {
    		t.Errorf("applyBootAuthError(revoked) = %v, want bootAuthRevoked", got)
    	}
    }

    // TestApplyBootAuthError_NonRevoked verifies that a wincred-rebuild error
    // that does NOT match the revoked classifier routes to the ContinueSetup
    // path (bootAuthOther). Plan 09-04.
    func TestApplyBootAuthError_NonRevoked(t *testing.T) {
    	otherErr := errors.New("wincred read failed: target not found")
    	c := tray.NewController(tray.Config{})
    	got := applyBootAuthError(c, otherErr)
    	if got != bootAuthOther {
    		t.Errorf("applyBootAuthError(other) = %v, want bootAuthOther", got)
    	}
    }

    // TestApplyBootAuthError_TableDriven exercises both branches in one place
    // for traceability — mirrors the TestNeedsWizard pattern at runapp_test.go:45-68.
    // Plan 09-04.
    func TestApplyBootAuthError_TableDriven(t *testing.T) {
    	cases := []struct {
    		name string
    		err  error
    		want bootAuthClassification
    	}{
    		{"revoked-oauth2-retrieveerror", &oauth2.RetrieveError{ErrorCode: "invalid_grant"}, bootAuthRevoked},
    		{"non-revoked-generic", errors.New("wincred load failed"), bootAuthOther},
    		{"non-revoked-wrapped", fmt.Errorf("rebuild: %w", errors.New("io error")), bootAuthOther},
    	}
    	for _, tc := range cases {
    		t.Run(tc.name, func(t *testing.T) {
    			c := tray.NewController(tray.Config{})
    			got := applyBootAuthError(c, tc.err)
    			if got != tc.want {
    				t.Errorf("applyBootAuthError(%v) = %v, want %v", tc.err, got, tc.want)
    			}
    		})
    	}
    }
    ```

    Ensure the test file imports include (add any that are missing — check the existing import block first):
    - `"errors"` (likely already present)
    - `"fmt"` (for `fmt.Errorf` in the wrapped-error case)
    - `"testing"` (already present)
    - `"golang.org/x/oauth2"` (for `oauth2.RetrieveError`)
    - `"github.com/boejowen/SquireBot/internal/tray"` (for `tray.NewController` + `tray.Config`)

    If `golang.org/x/oauth2` is NOT already imported by `runapp_test.go`, add it. The dependency is already in go.mod (used elsewhere in the watcher).

    If `oauth2.RetrieveError` does not match `IsRevokedRefreshToken` after running the test (i.e., the test fails because the classifier rejects this shape), then read `internal/auth/refresh.go` more carefully and ADJUST the revoked-error construction to a shape the classifier definitively matches. The valid alternative shapes per PATTERNS.md:
    1. `&oauth2.RetrieveError{ErrorCode: "invalid_grant"}`
    2. `errors.New("oauth2: \"invalid_grant\" refresh token revoked")` (or similar string-form the classifier substring-matches against)

    Pick whichever matches. Do NOT modify `auth.IsRevokedRefreshToken` itself — its behavior is contract from AUTH-05 land.

    Schema-impact assertion (per CONTEXT.md D-06): `internal/sheet/client.go` not touched.
  </action>
  <verify>
    <automated>go build ./internal/app/... && go vet ./internal/app/... && go test ./internal/app/ -run 'TestApplyBootAuthError' -v</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE 'func TestApplyBootAuthError_Revoked' internal/app/runapp_test.go` matches exactly 1 line.
    - `grep -nE 'func TestApplyBootAuthError_NonRevoked' internal/app/runapp_test.go` matches exactly 1 line.
    - `grep -nE 'func TestApplyBootAuthError_TableDriven' internal/app/runapp_test.go` matches exactly 1 line.
    - `grep -nE 'bootAuthRevoked' internal/app/runapp_test.go` matches at least 2 lines (Revoked test + TableDriven case).
    - `grep -nE 'bootAuthOther' internal/app/runapp_test.go` matches at least 2 lines (NonRevoked test + TableDriven case).
    - `grep -nE 'tray\.NewController\(tray\.Config\{\}\)' internal/app/runapp_test.go` matches at least 3 lines (one per test that constructs a Controller).
    - `go test ./internal/app/ -run 'TestApplyBootAuthError_Revoked' -v` reports PASS.
    - `go test ./internal/app/ -run 'TestApplyBootAuthError_NonRevoked' -v` reports PASS.
    - `go test ./internal/app/ -run 'TestApplyBootAuthError_TableDriven' -v` reports 3/3 sub-cases PASS.
    - Full `go test ./... -count=1` exits 0 (no broader regression across the watcher).
    - `go vet ./internal/app/...` exits 0.
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (unchanged).
    - Cross-plan dependency check: `grep -nE 'pendingAction|drainPending' internal/tray/tray.go` returns matches (confirms Plan 09-01 has landed before this plan executes; if it returns 0, this plan cannot succeed and must wait).
  </acceptance_criteria>
  <done>Three new tests in runapp_test.go covering both classifier branches; tests pass; full `go test ./... -count=1` exits 0; schema constant unchanged.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| wincred → token rebuild | Untrusted-input-style boundary: a revoked or corrupted refresh token surfaces as error; this plan classifies it |
| classifier → tray UX | Classification result drives whether the user sees Reauthorize (revoked) or ContinueSetup (other) |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-09-04-01 | Spoofing | Misclassification — showing Reauthorize for a transient error | mitigate | Reuse the SAME `auth.IsRevokedRefreshToken` classifier AUTH-05 uses in production. Do NOT expand the trigger surface via `isPermanentAuthErr` (which adds `sheet.ErrPermanentAuth`, irrelevant at boot). Tests cover the boundary (revoked → bootAuthRevoked; everything else → bootAuthOther). |
| T-09-04-02 | Information Disclosure | Status string leaks refresh-token internals | mitigate | The non-revoked branch interpolates the raw error: `fmt.Sprintf("Auth error: %v", err)`. This is EXISTING behavior preserved verbatim; CLAUDE.md prohibits logging refresh tokens specifically — generic wincred errors don't contain token material. The revoked branch uses a static string ("Reauthorize: refresh token died. Click Reauthorize…") with no error interpolation. |
| T-09-04-03 | Repudiation | Lost log trace of boot failure | mitigate | `slog.Error("token rebuild from wincred failed", "err", err)` runs first in applyBootAuthError BEFORE classification — every boot failure leaves a structured log line regardless of branch. |
| T-09-04-04 | Tampering | Cross-plan dependency on Plan 09-01 | mitigate | This plan's wave (Wave 2) depends on Plan 09-01 (Wave 1). Task 2 acceptance includes a grep verification that `pendingAction|drainPending` exist in `internal/tray/tray.go` before this plan can complete — proves Plan 09-01 has landed. If absent, executor must stop and surface the wave-ordering violation. |

**Schema impact:** NONE. `internal/sheet/client.go` is not in `files_modified`. Verifier grep gate: `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line.
</threat_model>

<verification>
1. `go build ./...` exits 0.
2. `go vet ./...` exits 0.
3. `go test ./internal/app/ -count=1 -v` reports `TestApplyBootAuthError_Revoked`, `TestApplyBootAuthError_NonRevoked`, `TestApplyBootAuthError_TableDriven` all PASS; broader runapp tests (TestNeedsWizard etc.) remain green.
4. `go test ./... -count=1` exits 0 (no regressions in tray or anywhere else; Plan 09-01's tray tests remain green).
5. `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (schema unchanged).
6. `grep -cE 'Reauthorize: refresh token died\. Click Reauthorize…' internal/app/runapp.go` returns ≥ 2 (AUTH-05 suspendForAuth + AUTH-07 applyBootAuthError — perfect copy parity).
7. `grep -nE 'pendingAction' internal/tray/tray.go` matches at least 1 line (proves Plan 09-01 dependency satisfied).
</verification>

<success_criteria>
- A guildie whose refresh token was revoked between sessions sees, from boot: red tray icon + visible Reauthorize menu item with the canonical "Reauthorize: refresh token died. Click Reauthorize…" status string.
- Clicking Reauthorize reuses the existing AUTH-05 flow (no new wiring on the click side) and on success clears the auth-error state.
- A non-revoked wincred breakage still routes to ContinueSetup (no behavior regression for the existing common failure mode).
- The cold-start tray triple lands in Plan 09-01's pending queue and replays correctly via OnReady drain.
- No schema change; no broader RunApp refactor; no expansion of `isPermanentAuthErr`'s trigger surface.
</success_criteria>

<output>
After completion, create `.planning/phases/09-watcher-robustness-polish/09-04-SUMMARY.md` summarizing:
- The applyBootAuthError helper shape + classification enum.
- The exact AUTH-05 status string parity (verbatim reuse — record the grep count).
- Test additions (3 new tests) and pass counts.
- Confirmation that Plan 09-01's pendingAction / drainPending exist in tray.go at the time of this plan's land (cross-plan dependency satisfied).
- Schema constant `WatcherMaxSchemaVersion = 3` confirmed unchanged.
- Hand-off to Plan 09-05: "All 4 fixes are now in main; ship gate (tag, latest.json, GitHub Release) can proceed."
</output>
