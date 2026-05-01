// Package watch provides the fsnotify-based EQ folder watcher and the per-path
// timer-reset Debouncer that coalesces bursts of file events into single
// "quiet-period elapsed" emissions.
//
// SECURITY/CORRECTNESS: per CLAUDE.md / RESEARCH.md §8.3, this package never
// trusts fsnotify event payloads beyond the path. ev.Op is consulted only to
// drop pure Chmod events; Size, Mtime, ModTime fields are NEVER read (and
// fsnotify.Event does not actually carry them — the discipline is to make
// "always re-read fresh" structurally impossible to violate downstream).
package watch

import (
	"sync"
	"time"
)

// Debouncer coalesces bursts of Trigger(path) calls into a single
// emission per path on the channel returned by NewDebouncer. The timer
// for each path resets on every Trigger; after `delay` of quiet for a
// given path, that path is sent on the out channel.
//
// Pattern reference: RESEARCH.md §2.5 Pattern 3 (Per-Path Timer-Reset Debouncer).
type Debouncer struct {
	delay   time.Duration
	timers  sync.Map // path string -> *time.Timer
	out     chan string
	stopped chan struct{}
	once    sync.Once
}

// NewDebouncer returns a Debouncer and the read-end of its out channel.
// Buffer size 16 absorbs bursts without blocking the producer (fsnotify loop).
func NewDebouncer(delay time.Duration) (*Debouncer, <-chan string) {
	out := make(chan string, 16)
	d := &Debouncer{delay: delay, out: out, stopped: make(chan struct{})}
	return d, out
}

// Trigger resets (or creates) the per-path timer. Safe for concurrent calls.
// After Stop has been called, subsequent Triggers are silently dropped.
func (d *Debouncer) Trigger(path string) {
	select {
	case <-d.stopped:
		return
	default:
	}
	if t, ok := d.timers.Load(path); ok {
		t.(*time.Timer).Reset(d.delay)
		return
	}
	t := time.AfterFunc(d.delay, func() {
		d.timers.Delete(path)
		select {
		case d.out <- path:
		case <-d.stopped:
		}
	})
	d.timers.Store(path, t)
}

// Stop cancels all in-flight timers and silences future Trigger calls.
// Idempotent.
func (d *Debouncer) Stop() {
	d.once.Do(func() {
		close(d.stopped)
		d.timers.Range(func(_, v any) bool {
			v.(*time.Timer).Stop()
			return true
		})
	})
}
