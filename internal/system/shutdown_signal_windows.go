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
//  1. OpenEvent — succeeds if a listener already created the event.
//     This is the common case (NSIS shim signaling a running watcher).
//  2. CreateEvent fallback — if OpenEvent returns ERROR_FILE_NOT_FOUND
//     no listener is active. We CreateEvent then SetEvent so that
//     a listener that starts AFTER the signal still observes the
//     signaled state immediately (manual-reset semantics). This is the
//     "fire-and-forget" property D-01 promises.
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
