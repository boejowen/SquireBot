---
phase: 06-installer-overwrite-running-shim
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/system/shutdown_signal_windows.go
  - internal/system/shutdown_signal_other.go
  - internal/system/shutdown_signal_windows_test.go
  - internal/system/doc.go
autonomous: true
requirements: [INST-06]
tags: [windows, named-event, shutdown, ipc]

must_haves:
  truths:
    - "A new `internal/system` package exists with two exported symbols on Windows: `SignalShutdown() error` and `WaitForShutdown(ctx context.Context) <-chan struct{}`."
    - "On non-Windows builds the same symbols compile as stubs so `go vet ./...` and CI cross-compile succeed."
    - "On Windows, calling `SignalShutdown()` after `WaitForShutdown()` is set up causes the returned channel to close within 500ms."
    - "Calling `SignalShutdown()` when no listener is active is a benign no-op that returns nil (idempotent / fire-and-forget per D-01)."
    - "The named event uses the `Local\\SquireBot-Shutdown` namespace (per-session, not Global) per D-01."
  artifacts:
    - path: internal/system/shutdown_signal_windows.go
      provides: "Windows-only event creation, signaling, waiting (CreateEventW / OpenEventW / SetEvent / WaitForSingleObject / CloseHandle)"
      contains: "//go:build windows"
    - path: internal/system/shutdown_signal_other.go
      provides: "Non-Windows stubs so main.go cross-compiles"
      contains: "//go:build !windows"
    - path: internal/system/shutdown_signal_windows_test.go
      provides: "Round-trip test: SignalShutdown unblocks WaitForShutdown"
      contains: "//go:build windows"
    - path: internal/system/doc.go
      provides: "Package-level doc comment (cross-platform, no build tag)"
  key_links:
    - from: internal/system/shutdown_signal_windows.go
      to: golang.org/x/sys/windows
      via: "direct import — first consumer of bare windows package (eqfind uses windows/registry sub-package only)"
      pattern: "golang.org/x/sys/windows"
    - from: internal/system/shutdown_signal_windows_test.go
      to: internal/system/shutdown_signal_windows.go
      via: "round-trip test in same package"
      pattern: "SignalShutdown|WaitForShutdown"
---

<objective>
Create a new `internal/system` package with paired build-tag files providing a one-way Windows named-event shutdown signal. The `SignalShutdown()` function is what `squirebot.exe --quit` will call (Plan 02); the `WaitForShutdown(ctx)` channel is what the normal-launch listener goroutine will block on (Plan 02). The NSIS pre-install shim (Plan 03) drives the whole chain by `ExecWait '"$INSTDIR\squirebot.exe" --quit'`.

Purpose: zero-dependency, fire-and-forget IPC primitive for "tell the running watcher to gracefully exit." Note: "fire-and-forget" here means SignalShutdown does not block or error when no listener is active; it does NOT mean a later listener will retroactively observe a lost signal (see TestListenerCreatedAfterSignalDoesNotFire). The NSIS shim guarantees a listener is active by signaling a running watcher (per Plan 03's IfFileExists guard). Per D-01 this beats WM_CLOSE (brittle on `fyne.io/systray`'s internal HWND), named-pipe IPC (overkill, more attack surface), and `taskkill /F` only (violates ROADMAP §44 success criterion 2's "signal gracefully first" mandate).

Output: a compilable package with one Windows-only implementation, one non-Windows stub, a Windows-only round-trip test, and a shared package doc. Zero changes to `go.mod` (`golang.org/x/sys v0.43.0` already present at line 12).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md
@.planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md
@internal/eqfind/registry_windows.go
@internal/eqfind/registry_other.go
@internal/auth/store_test.go
@internal/update/swap.go
@go.mod

<interfaces>
<!-- Exact contract the rest of the phase will consume. The executor MUST keep these signatures EXACT — Plan 02 and the listener goroutine code in main.go are written against them verbatim. -->

```go
// Package system provides Windows-only IPC primitives for cross-process
// coordination of the SquireBot watcher process.
//
// Phase 6 (INST-06): SignalShutdown + WaitForShutdown are the named-event
// based graceful-shutdown channel used by the NSIS pre-install shim to
// stop a running watcher before file overwrite.
package system

// SignalShutdown opens (or creates) the Local\SquireBot-Shutdown named
// event in the current Windows interactive session and sets it.
//
// Called by `squirebot.exe --quit` (cmd/squirebot/main.go) — runs BEFORE
// logging.Setup so any error MUST go to stderr by the caller, not slog.
//
// Returns:
//   - nil               -> event signaled successfully (whether or not
//                          another process is currently waiting on it;
//                          signal-with-no-listener is a benign no-op per D-01).
//   - non-nil error     -> Windows API failed (CreateEventW, OpenEventW,
//                          or SetEvent). Wrapped via fmt.Errorf("API_NAME: %w").
//                          Caller logs to stderr and exits 0 — NSIS falls
//                          back to taskkill /F regardless.
//
// On non-Windows builds this is a no-op that returns nil.
func SignalShutdown() error

// WaitForShutdown returns a channel that closes when the
// Local\SquireBot-Shutdown named event is signaled by another process
// (typically `squirebot.exe --quit` from the NSIS pre-install shim) OR
// when ctx is cancelled (whichever happens first).
//
// Called once from the normal-launch path in cmd/squirebot/main.go after
// logging.Setup. The returned channel is single-use (closed exactly once,
// never re-armed). Listener goroutine must select on both <-WaitForShutdown(ctx)
// AND <-ctx.Done() to avoid leaks if shutdown comes from another source
// (tray Quit menu, OS signal).
//
// Lifecycle: spawns one internal goroutine that holds the event handle
// open for the lifetime of ctx. The handle is closed via defer when
// either the wait returns or ctx is cancelled. No CloseHandle leaks.
//
// On non-Windows builds the returned channel closes only when ctx is done.
func WaitForShutdown(ctx context.Context) <-chan struct{}
```
</interfaces>

<windows_api_refs>
<!-- golang.org/x/sys/windows symbols used. Verified present in v0.43.0 (go.mod line 12). -->

- `windows.CreateEvent(sa *SecurityAttributes, manualReset uint32, initialState uint32, name *uint16) (handle Handle, err error)` — manualReset=1 (manual-reset event so multiple WaitForSingleObject callers all observe the signaled state; we have only one waiter but manual-reset is the conservative choice).
- `windows.OpenEvent(desiredAccess uint32, inheritHandle bool, name *uint16) (handle Handle, err error)` — used by SignalShutdown to attach to an existing event without creating a second handle when the listener already created it.
- `windows.SetEvent(handle Handle) (err error)` — sets the event to signaled.
- `windows.WaitForSingleObject(handle Handle, milliseconds uint32) (event uint32, err error)` — blocks the internal goroutine on the event.
- `windows.CloseHandle(handle Handle) (err error)` — defer-cleanup.
- `windows.UTF16PtrFromString(s string) (*uint16, error)` — name marshaling (`Local\SquireBot-Shutdown`).
- `windows.EVENT_ALL_ACCESS uint32 = 0x1F0003` — desired access for both Create and Open.
- `windows.INFINITE uint32 = 0xFFFFFFFF` — WaitForSingleObject timeout sentinel.
- `windows.WAIT_OBJECT_0 uint32 = 0` — successful wait return code.
- `windows.WAIT_FAILED uint32 = 0xFFFFFFFF` — failure return code.

Pattern for SignalShutdown: try OpenEvent first (listener already created it); if ERROR_FILE_NOT_FOUND, fall back to CreateEvent (no listener active — we'll create the event so SetEvent has something to signal, and the next caller can OpenEvent it; signal-with-no-listener is benign because the next listener's WaitForSingleObject on a pre-signaled manual-reset event returns immediately, which is exactly the semantics we want for a fire-and-forget signal).
</windows_api_refs>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create package doc + non-Windows stubs</name>
  <files>internal/system/doc.go, internal/system/shutdown_signal_other.go</files>
  <read_first>
    - internal/eqfind/registry_other.go (the 7-line stub pattern this MUST mirror exactly, including //go:build !windows + 1-line no-op comment style)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (section "NEW internal/system/shutdown_signal_other.go")
    - .planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md (D-01 rationale for the named-event choice)
  </read_first>
  <behavior>
    - `package system` exists and compiles on linux/darwin/windows.
    - On non-Windows: `SignalShutdown()` returns `nil` immediately (no-op).
    - On non-Windows: `WaitForShutdown(ctx)` returns a channel that closes when `ctx.Done()` fires (NOT a `nil` channel — must be a usable receive operand in `select`).
    - `go vet ./internal/system/...` succeeds on the developer's local Windows machine.
  </behavior>
  <action>
Create two files.

**File 1: `internal/system/doc.go`** — no build tag (cross-platform), 8-12 lines max. Format exactly:

```go
// Package system provides cross-process IPC primitives for the SquireBot
// watcher. Phase 6 (INST-06) introduces a Windows named-event based
// graceful-shutdown channel used by the NSIS pre-install shim to stop a
// running watcher before file overwrite.
//
// Public surface (Windows + stubs on other platforms):
//   - SignalShutdown() error                                — signal side
//   - WaitForShutdown(ctx context.Context) <-chan struct{}  — listener side
//
// The named event uses the Local\ namespace (per-session, not Global) so
// signals from one logon session never bleed into another — matches the
// per-user-installation model locked in Phase 1 (INST-01).
package system
```

**File 2: `internal/system/shutdown_signal_other.go`** — mirror `internal/eqfind/registry_other.go` style:

```go
//go:build !windows

package system

import "context"

// SignalShutdown is a no-op on non-Windows platforms. The Windows
// implementation lives in shutdown_signal_windows.go.
func SignalShutdown() error { return nil }

// WaitForShutdown returns a channel that closes only when ctx is
// cancelled. The Windows implementation in shutdown_signal_windows.go
// also closes the channel when the Local\SquireBot-Shutdown named event
// fires.
func WaitForShutdown(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()
	return done
}
```

Conventions to preserve verbatim from `internal/eqfind/registry_other.go`:
- Build-tag header `//go:build !windows` on line 1, blank line, `package system` on line 3.
- Each exported function has a doc comment that names the Windows counterpart file (cross-reference for grep).
- No external dependencies (only stdlib `context`).
  </action>
  <verify>
    <automated>
      # shell: bash
      cd /c "C:/Users/Virus Canary/Desktop/Claude/SquireBot" && go vet ./internal/system/... 2>&1 | tee /tmp/vet-01.log && grep -c "^$" /tmp/vet-01.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Test-Path internal/system/doc.go` returns True.
    - `Test-Path internal/system/shutdown_signal_other.go` returns True.
    - `Select-String -Path internal/system/shutdown_signal_other.go -Pattern '^//go:build !windows$'` matches exactly 1 line.
    - `Select-String -Path internal/system/shutdown_signal_other.go -Pattern 'func SignalShutdown\(\) error \{ return nil \}'` matches exactly 1 line.
    - `Select-String -Path internal/system/shutdown_signal_other.go -Pattern 'func WaitForShutdown\(ctx context\.Context\) <-chan struct\{\}'` matches exactly 1 line.
    - `Select-String -Path internal/system/doc.go -Pattern '^package system$'` matches exactly 1 line.
    - `Select-String -Path internal/system/doc.go -Pattern '//go:build'` matches zero lines (doc.go must be cross-platform — no build tag).
    - `go vet ./internal/system/...` exits 0.
  </acceptance_criteria>
  <done>
    Package compiles on Windows. The non-Windows stub returns a real channel (not nil) so callers' `select` arms work. The exact function signatures match the `<interfaces>` contract above and will not need to change in Task 2.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Implement Windows-only event signal + listener + round-trip test</name>
  <files>internal/system/shutdown_signal_windows.go, internal/system/shutdown_signal_windows_test.go</files>
  <read_first>
    - internal/eqfind/registry_windows.go (build-tag header + import style + error handling pattern in a Windows-only file)
    - internal/auth/store_test.go (//go:build windows test header + uniqueEmail per-test name pattern; the test here uses a uniqueEventName helper that mirrors it)
    - internal/update/swap.go (error-wrap style: `fmt.Errorf("API_NAME: %w", err)` — see lines 73, 94, 103, 109)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (sections "NEW internal/system/shutdown_signal.go" + "NEW internal/system/shutdown_signal_test.go" + "Shared Patterns -> Idempotent + fire-and-forget signaling")
    - internal/system/shutdown_signal_other.go (just written in Task 1 — same exported signatures must be honored here)
    - go.mod (confirm golang.org/x/sys v0.43.0 line 12 is the dep we'll use; no new go.mod entry)
  </read_first>
  <behavior>
    - Test 1 (`TestSignalUnblocksWait`): listener goroutine calls `WaitForShutdown(ctx)` for a per-test-unique event name; signaler calls `SignalShutdown()` for the same name; the returned channel closes within 500ms.
    - Test 2 (`TestSignalWithNoListenerIsNoOp`): calling `SignalShutdown()` with no listener active returns nil (does not panic, does not error).
    - Test 3 (`TestCtxCancelClosesChannel`): if `WaitForShutdown(ctx)` is called and ctx is cancelled WITHOUT any signal, the returned channel still closes within 500ms (lifecycle hygiene — no leaked goroutines).
    - Test 4 (`TestSignalIsIdempotent`): calling `SignalShutdown()` twice in a row both return nil; the second call does not error.
    - Test 5 (`TestListenerCreatedAfterSignalDoesNotFire`): calling `SignalShutdown()` BEFORE any listener attaches results in the kernel event being destroyed when SignalShutdown's eager `CloseHandle` runs; a later `WaitForShutdown(ctx)` then creates a FRESH unsignaled event and the channel does NOT close until ctx expires. This documents the actual fire-and-forget contract: signaling is only observed by a listener that has already called `WaitForShutdown` (the NSIS shim guarantees this by signaling a running watcher).
    - For the test seam: a package-internal `eventName` variable defaults to `Local\SquireBot-Shutdown` but tests override it via a helper (otherwise parallel `go test` runs collide on the production event name).
  </behavior>
  <action>
Create two files. The implementation file uses the named test-seam variable so tests can supply a per-test-unique event name without collision.

**File 1: `internal/system/shutdown_signal_windows.go`** (~80 lines):

```go
//go:build windows

package system

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// eventName is the named-event identifier opened/created by both signal
// and listener sides. The Local\ namespace is per-logon-session (not
// cross-session) per D-01 — matches the per-user-installation model.
//
// Exposed as a package-level var rather than a const so tests can
// override it with a per-test unique name (TestSignalUnblocksWait runs
// in parallel with other goroutines; collisions on the production event
// name would cause cross-test contamination).
var eventName = `Local\SquireBot-Shutdown`

// SignalShutdown opens (or creates) the named event and sets it.
// See doc.go for the exported contract.
//
// Two-stage handle acquisition:
//   1. OpenEvent — succeeds if a listener already created the event.
//      This is the common case (NSIS shim signaling a running watcher).
//   2. CreateEvent fallback — if OpenEvent returns ERROR_FILE_NOT_FOUND
//      no listener is active. We CreateEvent then SetEvent so that
//      a listener that starts AFTER the signal still observes the
//      signaled state immediately (manual-reset semantics). This is the
//      "fire-and-forget" property D-01 promises.
func SignalShutdown() error {
	namePtr, err := windows.UTF16PtrFromString(eventName)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString: %w", err)
	}

	handle, err := windows.OpenEvent(windows.EVENT_ALL_ACCESS, false, namePtr)
	if err != nil {
		// ERROR_FILE_NOT_FOUND => no listener; create it ourselves so
		// SetEvent has a kernel object to set. Manual-reset (arg 2 = 1)
		// so a later WaitForSingleObject returns immediately.
		if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return fmt.Errorf("OpenEvent: %w", err)
		}
		handle, err = windows.CreateEvent(nil, 1, 0, namePtr)
		if err != nil {
			return fmt.Errorf("CreateEvent: %w", err)
		}
	}
	defer windows.CloseHandle(handle)

	if err := windows.SetEvent(handle); err != nil {
		return fmt.Errorf("SetEvent: %w", err)
	}
	return nil
}

// WaitForShutdown returns a channel that closes when the named event is
// signaled or ctx is cancelled. See doc.go for the exported contract.
//
// Contract precondition: signaling is only observed by a listener that has
// already called WaitForShutdown. A SignalShutdown call BEFORE any listener
// attaches is lost (the kernel event is destroyed when SignalShutdown's eager
// CloseHandle runs and no other handle exists). The NSIS shim guarantees
// the precondition by signaling a running watcher -- listener active by the
// time main.go reaches go app.RunApp. See TestListenerCreatedAfterSignalDoesNotFire.
//
// Implementation: spawns one goroutine that blocks on
// WaitForSingleObject and races against ctx via a second goroutine that
// SetEvents the handle on ctx.Done (Windows has no "wait on event OR
// context" primitive). The handle is closed exactly once via defer.
func WaitForShutdown(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	namePtr, err := windows.UTF16PtrFromString(eventName)
	if err != nil {
		// Cannot proceed; close immediately so callers don't block forever.
		close(done)
		return done
	}

	handle, err := windows.CreateEvent(nil, 1, 0, namePtr)
	if err != nil {
		close(done)
		return done
	}

	// Ctx-cancel watchdog: SetEvent on cancel so WaitForSingleObject
	// returns and the main goroutine can close(done) + CloseHandle.
	go func() {
		<-ctx.Done()
		_ = windows.SetEvent(handle)
	}()

	go func() {
		defer windows.CloseHandle(handle)
		defer close(done)
		_, _ = windows.WaitForSingleObject(handle, windows.INFINITE)
	}()

	return done
}
```

Conventions enforced:
- Build-tag header `//go:build windows` on line 1.
- All Windows-API call sites wrap with `fmt.Errorf("API_NAME: %w", err)` per swap.go style.
- `package system` (not `system_windows` or any other variant).
- No `slog` calls (signal-side runs before logging.Setup in main.go per Plan 02).
- `var eventName` (not `const`) for the test seam.

**File 2: `internal/system/shutdown_signal_windows_test.go`** (~90 lines):

```go
//go:build windows

package system

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// uniqueEventName returns a per-test event name that cannot collide
// with the production name or with parallel test goroutines.
// Mirrors internal/auth/store_test.go's uniqueEmail helper.
func uniqueEventName(name string) string {
	return fmt.Sprintf(`Local\SquireBot-Shutdown-test-%s-%d`, name, time.Now().UnixNano())
}

// withEventName temporarily overrides the package-level eventName var
// for the duration of a test. Restored via t.Cleanup.
func withEventName(t *testing.T, name string) {
	t.Helper()
	prev := eventName
	eventName = name
	t.Cleanup(func() { eventName = prev })
}

func TestSignalUnblocksWait(t *testing.T) {
	withEventName(t, uniqueEventName("unblocks"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := WaitForShutdown(ctx)

	// Give the listener goroutine a tick to call WaitForSingleObject
	// before we signal — exercises the "listener-already-active" path
	// of SignalShutdown (OpenEvent succeeds, no CreateEvent fallback).
	time.Sleep(50 * time.Millisecond)

	if err := SignalShutdown(); err != nil {
		t.Fatalf("SignalShutdown: %v", err)
	}

	select {
	case <-ch:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("WaitForShutdown channel did not close within 500ms after SignalShutdown")
	}
}

func TestSignalWithNoListenerIsNoOp(t *testing.T) {
	withEventName(t, uniqueEventName("no-listener"))
	// No WaitForShutdown call. SignalShutdown must succeed via the
	// CreateEvent fallback path and return nil.
	if err := SignalShutdown(); err != nil {
		t.Fatalf("SignalShutdown with no listener: %v", err)
	}
}

func TestCtxCancelClosesChannel(t *testing.T) {
	withEventName(t, uniqueEventName("ctx-cancel"))
	ctx, cancel := context.WithCancel(context.Background())
	ch := WaitForShutdown(ctx)

	// Cancel without ever signaling. The ctx-cancel watchdog
	// goroutine inside WaitForShutdown should SetEvent the handle,
	// the listener goroutine should return, channel should close.
	cancel()

	select {
	case <-ch:
		// success — no goroutine leak
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("WaitForShutdown channel did not close within 500ms after ctx cancel")
	}
}

func TestSignalIsIdempotent(t *testing.T) {
	withEventName(t, uniqueEventName("idempotent"))
	if err := SignalShutdown(); err != nil {
		t.Fatalf("first SignalShutdown: %v", err)
	}
	if err := SignalShutdown(); err != nil {
		t.Fatalf("second SignalShutdown: %v", err)
	}
}

// TestListenerCreatedAfterSignalDoesNotFire documents the actual contract:
// signaling is only observed by a listener that has ALREADY called
// WaitForShutdown. The "no-op when no listener" framing in D-01 means
// SignalShutdown does not error, NOT that a later listener will retroactively
// observe the lost signal. Manual-reset semantics do not survive an eager
// CloseHandle on the signaler side when no other handle exists.
//
// The NSIS shim guarantees the listener-active precondition by signaling a
// running watcher (listener is active by the time main.go reaches go app.RunApp).
func TestListenerCreatedAfterSignalDoesNotFire(t *testing.T) {
	withEventName(t, uniqueEventName("late-listener"))

	// Signal first, with no listener attached. SignalShutdown's defer
	// CloseHandle destroys the kernel event since no other handle exists.
	if err := SignalShutdown(); err != nil {
		t.Fatalf("SignalShutdown: %v", err)
	}

	// Give the kernel a beat to fully tear down the event.
	time.Sleep(50 * time.Millisecond)

	// Now attach a listener. It will CreateEvent a FRESH unsignaled
	// event under the same name. The channel must NOT close before
	// ctx expires because the original signal was lost.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch := WaitForShutdown(ctx)

	select {
	case <-ch:
		// Channel closed BEFORE ctx expired -> the lost signal was
		// retroactively observed, which would contradict the contract.
		if ctx.Err() == nil {
			t.Fatalf("WaitForShutdown channel closed before ctx expiry; signal-before-listener should be lost, not retroactively observed")
		}
		// If ctx already expired, the channel closed via the ctx-cancel
		// watchdog path -- that is the expected outcome.
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("ctx timeout (200ms) did not propagate to channel close within 500ms")
	}
}
```

Test conventions enforced:
- Build-tag header `//go:build windows` matches `internal/auth/store_test.go` line 1.
- Per-test unique name via `uniqueEventName(name)` helper (UnixNano collision-free, mirrors `uniqueEmail`).
- `t.Cleanup` restores the package-level var (no test pollution).
- 500ms timeout on assertions (matches the "must close within 500ms" behavior contract above).
  </action>
  <verify>
    <automated>
      # shell: bash
      cd /c "C:/Users/Virus Canary/Desktop/Claude/SquireBot" && go test ./internal/system/... -count=1 -v 2>&1 | tee /tmp/test-system.log && grep -c "^--- PASS: Test" /tmp/test-system.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Test-Path internal/system/shutdown_signal_windows.go` returns True.
    - `Test-Path internal/system/shutdown_signal_windows_test.go` returns True.
    - `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern '^//go:build windows$'` matches exactly 1 line.
    - `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern 'var eventName = `Local\\SquireBot-Shutdown`'` matches exactly 1 line (uses backticks; verify literal `Local\SquireBot-Shutdown` per D-01).
    - `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern 'golang\.org/x/sys/windows'` matches exactly 1 line in the import block.
    - `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern 'windows\.(CreateEvent|OpenEvent|SetEvent|WaitForSingleObject|CloseHandle)'` matches at least 5 distinct call sites.
    - `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern 'fmt\.Errorf\("(UTF16PtrFromString|OpenEvent|CreateEvent|SetEvent): %w"' -AllMatches` matches at least 3 lines (error-wrap style per swap.go).
    - `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern 'slog\.' -AllMatches` matches zero lines (signal side runs before logging.Setup; no slog).
    - `go build ./internal/system/...` exits 0.
    - `go test ./internal/system/... -count=1 -v` exits 0 with at least 5 `--- PASS: Test` lines (TestSignalUnblocksWait, TestSignalWithNoListenerIsNoOp, TestCtxCancelClosesChannel, TestSignalIsIdempotent, TestListenerCreatedAfterSignalDoesNotFire).
    - `go vet ./internal/system/...` exits 0.
  </acceptance_criteria>
  <done>
    The Windows implementation exists, all 5 tests pass on the developer's Windows box, no slog imports in the signal path, error wraps match the swap.go house style, and the eventName test seam allows parallel `go test` runs without cross-contamination.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Cross-process named-event signal | Any other process in the same Windows interactive logon session can OpenEvent `Local\SquireBot-Shutdown` and SetEvent it, triggering watcher shutdown. |
| Build-time platform gate | `//go:build` tags partition Windows vs. non-Windows code; mis-tagged code would silently no-op on the wrong platform. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-01 | Spoofing | `Local\SquireBot-Shutdown` named event — any user-session process can signal it | accept | Per-session `Local\` namespace (NOT `Global\`) limits the spoofing surface to the user's own logon session. Worst-case impact: the watcher exits gracefully; no data loss (WATCH-09 catch-up re-uploads on next launch). Attacker capability is bounded by user-session integrity; if an attacker already runs code in the user's session, killing SquireBot is the least of the user's problems. Disposition follows CONTEXT.md security framing. |
| T-06-02 | Tampering | `eventName` package var (test seam) | mitigate | The var is unexported (lowercase `eventName`). Reflection-based override requires Go runtime access (already a sandbox escape). No external input feeds into `eventName` — only test code overrides via `withEventName` helper inside the same package. |
| T-06-03 | Denial of Service | Repeated signal floods from a hostile user-session process | accept | `SetEvent` on a manual-reset event is O(1) and idempotent. The listener processes one signal and exits the goroutine; subsequent SetEvents to the now-closed handle are no-ops. Cost of a flood is bounded by the speed of the first signal taking the listener down — same as T-06-01. |
| T-06-04 | Information Disclosure | Event handle leak via missed CloseHandle | mitigate | Both goroutines use `defer windows.CloseHandle(handle)`; the ctx-cancel watchdog SetEvents the handle to unblock the listener so the defer fires. Test `TestCtxCancelClosesChannel` verifies the channel closes within 500ms of ctx cancel — a leaked handle would manifest as a hung listener goroutine and the test would time out. |
| T-06-05 | Elevation of Privilege | Signal could escalate to admin if invoked from a privileged process | accept | `EVENT_ALL_ACCESS` is requested but the event itself carries no privileges. Setting an event triggers shutdown only; there is no code-execution path opened. Watcher runs at user integrity (INST-01 `RequestExecutionLevel user`) — no elevation surface to exploit. |

ASVS L1: no `high` severity threats. All mitigations are in-code (per-session namespace, defers, unexported test seam); no runtime config required.
</threat_model>

<verification>
- `go build ./internal/system/...` exits 0 on Windows.
- `go test ./internal/system/... -count=1` reports at least 4 passing tests.
- `go vet ./internal/system/...` exits 0.
- `Select-String -Path internal/system/shutdown_signal_windows.go -Pattern 'Local\\SquireBot-Shutdown'` confirms D-01 event name.
- Cross-compile sanity: `GOOS=linux go build ./internal/system/...` exits 0 (proves the stub compiles).
</verification>

<success_criteria>
- New `internal/system` package exists with 4 files (doc.go, shutdown_signal_other.go, shutdown_signal_windows.go, shutdown_signal_windows_test.go).
- `SignalShutdown()` and `WaitForShutdown(ctx)` exported on all platforms with the exact signatures in `<interfaces>`.
- 4 Windows-only tests pass: unblock, no-listener no-op, ctx-cancel close, idempotent signal.
- Zero changes to `go.mod` (no new dep).
- ROADMAP §45 success criterion 2 partial coverage: "NSIS pre-install step signals `squirebot.exe` to exit gracefully, waits for it" — the Go-side signaling primitive is now in place. The graceful-exit funnel lands in Plan 02; the NSIS poll+timeout lands in Plan 03.
- D-01 honored: named Windows event, `Local\` namespace, zero new dependencies, fire-and-forget semantics tested.
</success_criteria>

<output>
After completion, create `.planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md` capturing:
- Files created (4 absolute paths).
- Test results (4/4 pass).
- The exact `SignalShutdown` + `WaitForShutdown` signatures (so Plan 02 doesn't have to re-derive them).
- Note: the package is now the canonical location for future Windows IPC primitives — if a Plan 03 picks `nsProcess` plugin vs. `System::Call`, document the choice here for the v1.0.1 SUMMARY.
</output>
</content>
</invoke>
