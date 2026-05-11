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
