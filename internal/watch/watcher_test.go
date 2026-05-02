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

// Helper: spin up Run in a goroutine watching one or more folders; return
// cancel + per-callback channels and counters for assertions.
type harness struct {
	cancel    context.CancelFunc
	invCalls  chan string
	spbCalls  chan string
	invCount  *int64
	spbCount  *int64
	runErrCh  chan error
	t         *testing.T
}

func startWatcher(t *testing.T, folders ...string) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	invCalls := make(chan string, 16)
	spbCalls := make(chan string, 16)
	var invCount, spbCount int64
	onInventory := func(p string) {
		atomic.AddInt64(&invCount, 1)
		select {
		case invCalls <- p:
		default:
		}
	}
	onSpellbook := func(p string) {
		atomic.AddInt64(&spbCount, 1)
		select {
		case spbCalls <- p:
		default:
		}
	}
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, folders, onInventory, onSpellbook) }()
	// Tiny grace period so fsnotify.Add registers before tests start writing.
	time.Sleep(50 * time.Millisecond)
	return &harness{
		cancel:   cancel,
		invCalls: invCalls,
		spbCalls: spbCalls,
		invCount: &invCount,
		spbCount: &spbCount,
		runErrCh: runErr,
		t:        t,
	}
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

// Test 1 (existing-coverage continuity): writing a *-Inventory.txt file fires
// onInventory within 800ms; onSpellbook does NOT fire.
func TestWatcher_InventoryWriteTriggersInventoryOnly(t *testing.T) {
	dir := t.TempDir()
	h := startWatcher(t, dir)
	t.Cleanup(func() { _ = h.stop() })

	path := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-h.invCalls:
		if filepath.Base(got) != "Foo-Inventory.txt" {
			t.Errorf("expected callback for Foo-Inventory.txt, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onInventory did not fire within 2s for Foo-Inventory.txt")
	}
	// onSpellbook must NOT have fired.
	if atomic.LoadInt64(h.spbCount) != 0 {
		t.Errorf("expected 0 onSpellbook calls, got %d", atomic.LoadInt64(h.spbCount))
	}
}

// Test 2 (Plan 02-02 dual-suffix): writing a *-Spellbook.txt fires onSpellbook;
// onInventory does NOT fire.
func TestWatcher_SpellbookWriteTriggersSpellbookOnly(t *testing.T) {
	dir := t.TempDir()
	h := startWatcher(t, dir)
	t.Cleanup(func() { _ = h.stop() })

	path := filepath.Join(dir, "Bar-Spellbook.txt")
	if err := os.WriteFile(path, []byte("9\tLifetap\n"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-h.spbCalls:
		if filepath.Base(got) != "Bar-Spellbook.txt" {
			t.Errorf("expected callback for Bar-Spellbook.txt, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onSpellbook did not fire within 2s for Bar-Spellbook.txt")
	}
	if atomic.LoadInt64(h.invCount) != 0 {
		t.Errorf("expected 0 onInventory calls, got %d", atomic.LoadInt64(h.invCount))
	}
}

// Test 3: a non-matching suffix → neither callback fires.
func TestWatcher_NonMatchingSuffixIgnored(t *testing.T) {
	dir := t.TempDir()
	h := startWatcher(t, dir)
	t.Cleanup(func() { _ = h.stop() })

	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// Wait long enough that any debounced emission would have fired (500ms + slop).
	time.Sleep(900 * time.Millisecond)
	if atomic.LoadInt64(h.invCount) != 0 || atomic.LoadInt64(h.spbCount) != 0 {
		t.Errorf("expected 0/0 calls for unrelated file, got inv=%d spb=%d",
			atomic.LoadInt64(h.invCount), atomic.LoadInt64(h.spbCount))
	}
}

// Test 4: 5 rapid writes to the same Inventory file produce exactly one onInventory.
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

	select {
	case got := <-h.invCalls:
		if filepath.Base(got) != "Bar-Inventory.txt" {
			t.Errorf("expected callback for Bar-Inventory.txt, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onInventory did not fire after burst")
	}
	// Confirm no second emission within another debounce window.
	select {
	case extra := <-h.invCalls:
		t.Fatalf("expected exactly one emission per burst, got an extra: %q (total=%d)",
			extra, atomic.LoadInt64(h.invCount))
	case <-time.After(800 * time.Millisecond):
		// expected silence
	}
	if got := atomic.LoadInt64(h.invCount); got != 1 {
		t.Errorf("expected exactly 1 onInventory call, got %d", got)
	}
}

// Test 5: cancelling ctx → Run returns ctx.Err().
func TestWatcher_CtxCancelExits(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	noop := func(string) {}
	go func() { runErr <- Run(ctx, []string{dir}, noop, noop) }()
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

// Test 6 (Plan 02-02 multi-folder, WATCH-03): Run with two folders configured;
// drop *-Inventory.txt into folder A AND *-Spellbook.txt into folder B; both
// callbacks fire with the correct full path.
func TestWatcher_MultiFolderDualSuffixDispatch(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	h := startWatcher(t, dirA, dirB)
	t.Cleanup(func() { _ = h.stop() })

	invPath := filepath.Join(dirA, "Slampeach-Inventory.txt")
	spbPath := filepath.Join(dirB, "Slampeach-Spellbook.txt")
	if err := os.WriteFile(invPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spbPath, []byte("9\tLifetap\n"), 0644); err != nil {
		t.Fatal(err)
	}

	gotInv := false
	gotSpb := false
	deadline := time.After(2500 * time.Millisecond)
	for !(gotInv && gotSpb) {
		select {
		case got := <-h.invCalls:
			gotInv = true
			if got != invPath {
				t.Errorf("inventory callback path = %q, want %q", got, invPath)
			}
		case got := <-h.spbCalls:
			gotSpb = true
			if got != spbPath {
				t.Errorf("spellbook callback path = %q, want %q", got, spbPath)
			}
		case <-deadline:
			t.Fatalf("timeout waiting for both callbacks (gotInv=%v gotSpb=%v)", gotInv, gotSpb)
		}
	}
}

// Test 7 (Plan 02-02): empty eqFolders slice → Run returns an error before
// touching fsnotify.
func TestWatcher_NoFoldersConfigured(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	noop := func(string) {}
	err := Run(ctx, nil, noop, noop)
	if err == nil {
		t.Fatal("expected error for nil folders, got nil")
	}
}

// Keep sync imported (avoids unused-import lint if a future refactor drops it).
var _ sync.Once
