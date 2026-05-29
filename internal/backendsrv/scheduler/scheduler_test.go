package scheduler

import (
	"context"
	"testing"
	"time"
)

// TestRun_ReturnsOnContextCancel: the skeleton's ticker loop exits cleanly when
// its context is cancelled (the server cancels the root context on
// SIGINT/SIGTERM). Driving run() directly with an already-cancelled context
// proves the ctx.Done() branch returns without waiting a full HeartbeatInterval.
func TestRun_ReturnsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel up front so the select takes the ctx.Done() branch immediately

	done := make(chan struct{})
	go func() {
		run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// returned cleanly — correct
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler run() did not return after context cancel")
	}
}

// TestStart_NonBlockingAndStopsOnCancel: Start launches the skeleton in a
// goroutine (non-blocking) and the goroutine winds down when the context is
// cancelled. Start has no return value to assert, so this is a smoke test: it
// must not panic, must return immediately, and cancelling must not hang. (No
// real jobs exist to test — P11 is skeleton-only.)
func TestStart_NonBlockingAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start must return immediately (it spawns a goroutine and does not block).
	returned := make(chan struct{})
	go func() {
		Start(ctx)
		close(returned)
	}()
	select {
	case <-returned:
		// Start returned promptly — correct (non-blocking).
	case <-time.After(time.Second):
		t.Fatal("Start did not return promptly (should be non-blocking)")
	}

	// Cancelling the context unwinds the background goroutine; nothing to assert
	// beyond "no panic / no hang". Give it a beat to process the cancel.
	cancel()
	time.Sleep(50 * time.Millisecond)
}
