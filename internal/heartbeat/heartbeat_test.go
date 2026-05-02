package heartbeat

// Tests for the heartbeat goroutine (Plan 02-05 Task 2). Six behaviours:
//
//   1. Run launches -> WriteHeartbeat called once within ~10ms
//      (immediate first fire).
//   2. After the immediate fire, the next sleep is requested with
//      d=24h (Interval).
//   3. authSuspended.Load()==true -> WriteHeartbeat NOT called on tick;
//      the next 24h sleep is still scheduled.
//   4. ctx cancellation -> goroutine exits cleanly without further
//      WriteHeartbeat calls.
//   5. WriteHeartbeat returning an error does NOT kill the goroutine;
//      the next tick is still scheduled.
//   6. Empty charNames (config has no LastKnown*Mtime entries) ->
//      WriteHeartbeat is still called (with empty list); next tick
//      scheduled.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/config"
)

// stubWriter captures WriteHeartbeat calls. Goroutine-safe.
type stubWriter struct {
	mu          sync.Mutex
	calls       []stubCall
	returnErr   error
	signal      chan struct{} // closed-each-call (recreated per fire) for synchronisation
	signalReset bool
}

type stubCall struct {
	OwnerEmail     string
	CharNames      []string
	WatcherVersion string
}

func newStubWriter() *stubWriter {
	return &stubWriter{signal: make(chan struct{}, 16)}
}

func (s *stubWriter) WriteHeartbeat(ctx context.Context, ownerEmail string, charNames []string, watcherVersion string) error {
	s.mu.Lock()
	cn := make([]string, len(charNames))
	copy(cn, charNames)
	s.calls = append(s.calls, stubCall{
		OwnerEmail:     ownerEmail,
		CharNames:      cn,
		WatcherVersion: watcherVersion,
	})
	err := s.returnErr
	s.mu.Unlock()
	// Non-blocking send so a second fire before the test reads the channel
	// doesn't deadlock.
	select {
	case s.signal <- struct{}{}:
	default:
	}
	return err
}

func (s *stubWriter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *stubWriter) lastCall() (stubCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return stubCall{}, false
	}
	return s.calls[len(s.calls)-1], true
}

// installFakeSleep captures requested sleep durations and lets the test
// release the sleep on demand via the returned releaseFn. Restores the
// previous sleepFn on t.Cleanup.
type sleepCapture struct {
	mu        sync.Mutex
	durations []time.Duration
	gate      chan error // tests push the err the sleepFn should return
	closed    bool
}

func newSleepCapture() *sleepCapture {
	return &sleepCapture{gate: make(chan error, 16)}
}

func (s *sleepCapture) install(t *testing.T) {
	t.Helper()
	prev := sleepFn
	sleepFn = func(ctx context.Context, d time.Duration) error {
		s.mu.Lock()
		s.durations = append(s.durations, d)
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-s.gate:
			if !ok {
				return ctx.Err()
			}
			return err
		}
	}
	t.Cleanup(func() {
		sleepFn = prev
		s.mu.Lock()
		if !s.closed {
			close(s.gate)
			s.closed = true
		}
		s.mu.Unlock()
	})
}

func (s *sleepCapture) durationsCopy() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.durations))
	copy(out, s.durations)
	return out
}

// release lets the next sleepFn call return with the supplied err (use
// nil to simulate the timer firing normally).
func (s *sleepCapture) release(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.gate <- err
}

// waitForCalls polls until stubWriter has at least n calls or timeout.
func waitForCalls(t *testing.T, s *stubWriter, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.callCount() >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitForCalls: got %d, want %d (timeout %v)", s.callCount(), n, timeout)
}

// waitForDurations polls until sleepCapture has at least n recorded calls.
func waitForDurations(t *testing.T, s *sleepCapture, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(s.durationsCopy()) >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitForDurations: got %d, want %d (timeout %v)", len(s.durationsCopy()), n, timeout)
}

// Test 1: Run fires WriteHeartbeat once immediately on entry.
func TestRun_ImmediateFirstFire(t *testing.T) {
	sleeps := newSleepCapture()
	sleeps.install(t)

	w := newStubWriter()
	cfg := &config.Config{
		LastKnownInventoryMtime: map[string]string{"Foo": "2026-04-30T00:00:00Z"},
	}
	var suspended atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go Run(ctx, w, cfg, "alice@example.com", "0.2.0", &suspended)

	waitForCalls(t, w, 1, 500*time.Millisecond)

	c, ok := w.lastCall()
	if !ok {
		t.Fatal("no calls captured")
	}
	if c.OwnerEmail != "alice@example.com" {
		t.Errorf("OwnerEmail = %q, want alice@example.com", c.OwnerEmail)
	}
	if c.WatcherVersion != "0.2.0" {
		t.Errorf("WatcherVersion = %q, want 0.2.0", c.WatcherVersion)
	}
	if len(c.CharNames) != 1 || c.CharNames[0] != "Foo" {
		t.Errorf("CharNames = %v, want [Foo]", c.CharNames)
	}
}

// Test 2: After the immediate fire, the next sleep is requested with
// d=Interval (24h).
func TestRun_SchedulesTwentyFourHourSleep(t *testing.T) {
	sleeps := newSleepCapture()
	sleeps.install(t)

	w := newStubWriter()
	cfg := &config.Config{}
	var suspended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go Run(ctx, w, cfg, "a@b", "v", &suspended)
	// Wait until the goroutine has issued the first sleep request.
	waitForDurations(t, sleeps, 1, 500*time.Millisecond)

	durs := sleeps.durationsCopy()
	if durs[0] != Interval {
		t.Errorf("first sleep d = %v, want %v", durs[0], Interval)
	}
	if Interval != 24*time.Hour {
		t.Errorf("Interval = %v, want 24h", Interval)
	}
}

// Test 3: authSuspended==true -> WriteHeartbeat NOT called; the next 24h
// sleep is still scheduled (heartbeat resumes after re-auth).
func TestRun_SkipsWhenAuthSuspended(t *testing.T) {
	sleeps := newSleepCapture()
	sleeps.install(t)

	w := newStubWriter()
	cfg := &config.Config{
		LastKnownInventoryMtime: map[string]string{"Foo": "2026-04-30T00:00:00Z"},
	}
	var suspended atomic.Bool
	suspended.Store(true) // BEFORE Run launches

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go Run(ctx, w, cfg, "a@b", "v", &suspended)
	// Wait for the first sleep to be scheduled (proxy for "tick has fired").
	waitForDurations(t, sleeps, 1, 500*time.Millisecond)

	if got := w.callCount(); got != 0 {
		t.Errorf("WriteHeartbeat call count = %d, want 0 (suspended)", got)
	}
	if d := sleeps.durationsCopy()[0]; d != Interval {
		t.Errorf("post-skip sleep = %v, want %v (next tick still scheduled)", d, Interval)
	}
}

// Test 4: ctx cancellation -> goroutine exits cleanly.
func TestRun_CtxCancellationExits(t *testing.T) {
	sleeps := newSleepCapture()
	sleeps.install(t)

	w := newStubWriter()
	cfg := &config.Config{}
	var suspended atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Run(ctx, w, cfg, "a@b", "v", &suspended)
		close(done)
	}()

	// Wait for the immediate fire + first sleep to be requested.
	waitForCalls(t, w, 1, 500*time.Millisecond)
	waitForDurations(t, sleeps, 1, 500*time.Millisecond)

	cancel()

	select {
	case <-done:
		// Goroutine exited cleanly.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return within 500ms of ctx cancellation")
	}

	finalCalls := w.callCount()
	// Wait a beat to confirm no further fires happen post-cancel.
	time.Sleep(20 * time.Millisecond)
	if got := w.callCount(); got != finalCalls {
		t.Errorf("WriteHeartbeat called after ctx cancellation: %d -> %d", finalCalls, got)
	}
}

// Test 5: WriteHeartbeat returning an error does NOT kill the goroutine;
// the next tick still happens.
func TestRun_ContinuesAfterWriteError(t *testing.T) {
	sleeps := newSleepCapture()
	sleeps.install(t)

	w := newStubWriter()
	w.returnErr = errors.New("boom")
	cfg := &config.Config{}
	var suspended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go Run(ctx, w, cfg, "a@b", "v", &suspended)

	// First (failing) fire.
	waitForCalls(t, w, 1, 500*time.Millisecond)
	waitForDurations(t, sleeps, 1, 500*time.Millisecond)

	// Clear the error and let the next sleep complete.
	w.mu.Lock()
	w.returnErr = nil
	w.mu.Unlock()
	sleeps.release(nil)

	// A second tick should follow.
	waitForCalls(t, w, 2, 500*time.Millisecond)
}

// Test 6: empty charNames -> WriteHeartbeat is still called (it's a no-op
// internally per Task 1 Test 6) and the next tick is scheduled.
func TestRun_EmptyCharNamesStillTicks(t *testing.T) {
	sleeps := newSleepCapture()
	sleeps.install(t)

	w := newStubWriter()
	cfg := &config.Config{} // both maps nil/empty
	var suspended atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go Run(ctx, w, cfg, "a@b", "v", &suspended)

	waitForCalls(t, w, 1, 500*time.Millisecond)
	waitForDurations(t, sleeps, 1, 500*time.Millisecond)

	c, _ := w.lastCall()
	if len(c.CharNames) != 0 {
		t.Errorf("CharNames = %v, want empty", c.CharNames)
	}
	if d := sleeps.durationsCopy()[0]; d != Interval {
		t.Errorf("post-empty-fire sleep = %v, want %v", d, Interval)
	}
}
