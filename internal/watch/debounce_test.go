package watch

import (
	"sync"
	"testing"
	"time"
)

// Debouncer Test 1: single trigger emits once after ~delay.
func TestDebouncer_SingleTriggerEmits(t *testing.T) {
	d, out := NewDebouncer(50 * time.Millisecond)
	defer d.Stop()
	start := time.Now()
	d.Trigger("a")
	select {
	case got := <-out:
		elapsed := time.Since(start)
		if got != "a" {
			t.Errorf("expected 'a', got %q", got)
		}
		// Window: 40ms .. 200ms (CI / Windows scheduling slop).
		if elapsed < 40*time.Millisecond {
			t.Errorf("emit too fast: %s (expected >= 40ms)", elapsed)
		}
		if elapsed > 200*time.Millisecond {
			t.Errorf("emit too slow: %s (expected <= 200ms)", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Debouncer did not emit within 500ms")
	}
}

// Debouncer Test 2: 5 rapid triggers coalesce into a single emission.
func TestDebouncer_CoalescesBurst(t *testing.T) {
	d, out := NewDebouncer(50 * time.Millisecond)
	defer d.Stop()
	for i := 0; i < 5; i++ {
		d.Trigger("a")
		time.Sleep(10 * time.Millisecond)
	}
	// Wait for the quiet period.
	select {
	case got := <-out:
		if got != "a" {
			t.Errorf("expected 'a', got %q", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Debouncer did not emit after burst")
	}
	// Verify no second emission within a generous window.
	select {
	case extra := <-out:
		t.Fatalf("Debouncer emitted twice (burst should coalesce): got extra %q", extra)
	case <-time.After(150 * time.Millisecond):
		// expected — silence after first emit
	}
}

// Debouncer Test 3: parallel triggers on different paths emit independently.
func TestDebouncer_IndependentPaths(t *testing.T) {
	d, out := NewDebouncer(50 * time.Millisecond)
	defer d.Stop()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); d.Trigger("a") }()
	go func() { defer wg.Done(); d.Trigger("b") }()
	wg.Wait()

	got := map[string]int{}
	timeout := time.After(500 * time.Millisecond)
	for i := 0; i < 2; i++ {
		select {
		case p := <-out:
			got[p]++
		case <-timeout:
			t.Fatalf("only got %d emissions before timeout: %v", i, got)
		}
	}
	if got["a"] != 1 || got["b"] != 1 {
		t.Errorf("expected exactly one emission per path, got %v", got)
	}
}

// Debouncer Test 4: after Stop(), subsequent Trigger calls do not produce emissions.
func TestDebouncer_StopSilencesTrigger(t *testing.T) {
	d, out := NewDebouncer(50 * time.Millisecond)
	d.Stop()
	d.Trigger("a")
	select {
	case got := <-out:
		t.Fatalf("expected no emission after Stop, got %q", got)
	case <-time.After(150 * time.Millisecond):
		// expected
	}
	// Stop is idempotent.
	d.Stop()
}
