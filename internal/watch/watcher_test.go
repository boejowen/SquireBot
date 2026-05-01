package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Helper: spin up Run in a goroutine watching tmpDir; return cancel + a channel
// that buffers all onChange invocations and a counter for assertions.
type harness struct {
	cancel    context.CancelFunc
	calls     chan string
	count     *int64
	runErrCh  chan error
	t         *testing.T
}

func startWatcher(t *testing.T, dir string) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	calls := make(chan string, 16)
	var count int64
	onChange := func(p string) {
		atomic.AddInt64(&count, 1)
		select {
		case calls <- p:
		default:
		}
	}
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, dir, onChange) }()
	// Tiny grace period so fsnotify.Add registers before tests start writing.
	time.Sleep(50 * time.Millisecond)
	return &harness{cancel: cancel, calls: calls, count: &count, runErrCh: runErr, t: t}
}

func (h *harness) stop() error {
	h.cancel()
	select {
	case err := <-h.runErrCh:
		return err
	case <-time.After(2 * time.Second):
		h.t.Fatal("Run did not exit within 2s after ctx cancel")
		return nil
	}
}

// Watcher Test 1: writing a *-Inventory.txt file triggers onChange within 800ms.
func TestWatcher_InventoryWriteTriggers(t *testing.T) {
	dir := t.TempDir()
	h := startWatcher(t, dir)
	t.Cleanup(func() { _ = h.stop() })

	path := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-h.calls:
		if filepath.Base(got) != "Foo-Inventory.txt" {
			t.Errorf("expected callback for Foo-Inventory.txt, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onChange did not fire within 2s for Foo-Inventory.txt")
	}
}

// Watcher Test 2: writing a *-Spellbook.txt file does NOT trigger onChange (Phase 1 scope).
func TestWatcher_SpellbookFiltered(t *testing.T) {
	dir := t.TempDir()
	h := startWatcher(t, dir)
	t.Cleanup(func() { _ = h.stop() })

	path := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait long enough that any debounced emission would have fired (500ms + slop).
	select {
	case got := <-h.calls:
		t.Fatalf("onChange fired for spellbook file (Phase 1 scope violation): %q", got)
	case <-time.After(900 * time.Millisecond):
		// expected silence
	}
	if atomic.LoadInt64(h.count) != 0 {
		t.Errorf("expected 0 onChange calls for spellbook write, got %d", atomic.LoadInt64(h.count))
	}
}

// Watcher Test 3: 5 rapid writes to the same Inventory file produce exactly one onChange.
func TestWatcher_BurstCoalescesToOne(t *testing.T) {
	dir := t.TempDir()
	h := startWatcher(t, dir)
	t.Cleanup(func() { _ = h.stop() })

	path := filepath.Join(dir, "Bar-Inventory.txt")
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(path, []byte{byte('a' + i)}, 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce + slop.
	select {
	case got := <-h.calls:
		if filepath.Base(got) != "Bar-Inventory.txt" {
			t.Errorf("expected callback for Bar-Inventory.txt, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onChange did not fire after burst")
	}

	// Confirm no second emission within another debounce window.
	select {
	case extra := <-h.calls:
		t.Fatalf("expected exactly one emission per burst, got an extra: %q (total=%d)", extra, atomic.LoadInt64(h.count))
	case <-time.After(800 * time.Millisecond):
		// expected silence
	}
	if got := atomic.LoadInt64(h.count); got != 1 {
		t.Errorf("expected exactly 1 onChange call, got %d", got)
	}
}

// Watcher Test 4: cancelling ctx → Run returns ctx.Err().
func TestWatcher_CtxCancelExits(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	noop := func(string) {}
	go func() { runErr <- Run(ctx, dir, noop) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit within 2s after ctx cancel")
	}
}

// _ = sync.Once is unused here, but keep sync imported via WaitGroup if we add it.
// (Avoid unused-import lint by using sync explicitly somewhere.)
var _ sync.Once
