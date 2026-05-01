// SECURITY/CORRECTNESS: This watcher follows the CLAUDE.md / RESEARCH.md §8.3 rule:
// it filters events purely by filename suffix and triggers a debouncer; the
// timer-fire dispatches a path to OnChange. The OnChange callback (Plan 07
// wiring) re-stats and re-reads the file fresh. We NEVER read ev.Op for
// ordering, ev.Name for content, or any other event payload field beyond
// the path. ev.Op is consulted only to drop fsnotify.Chmod (which on Windows
// is rare anyway). Spurious AV events become idempotent re-uploads —
// negligible cost.
package watch

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// OnChange is called by Run when a debounced quiet period elapses for `path`.
// The caller MUST re-stat and re-read `path` fresh — Run never reads file contents.
// Per CLAUDE.md: never trust fsnotify event payload data on Windows.
type OnChange func(path string)

// Run blocks. It watches `eqFolder` for *-Inventory.txt events, debounces 500ms per
// path, and dispatches OnChange after each quiet period. Returns when ctx is
// cancelled or fsnotify channels close.
//
// Phase 1 scope (D-11): single folder; spellbook is Phase 2 (WATCH-02).
func Run(ctx context.Context, eqFolder string, onChange OnChange) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// Per CLAUDE.md / RESEARCH.md Pitfall #4: watch the parent directory, never
	// individual files. fsnotify on Windows uses ReadDirectoryChangesW; only
	// the parent-dir handle survives EQ's overwrite of the inventory file.
	if err := w.Add(eqFolder); err != nil {
		return err
	}

	deb, out := NewDebouncer(500 * time.Millisecond)
	defer deb.Stop()

	slog.Info("watcher started", "folder", eqFolder)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-w.Events:
			if !ok {
				return errors.New("fsnotify Events channel closed")
			}
			base := filepath.Base(ev.Name)
			// Phase 1: ONLY *-Inventory.txt. WATCH-02 (spellbook) is Phase 2.
			if !strings.HasSuffix(base, "-Inventory.txt") {
				continue
			}
			// Drop pure Chmod (Windows: rare but possible).
			if ev.Op == fsnotify.Chmod {
				continue
			}
			// Per CLAUDE.md / RESEARCH.md §8.3: NEVER trust ev.Op or any payload.
			// Trigger the debouncer; on quiet, the receiver re-stats + re-reads.
			deb.Trigger(ev.Name)
		case e, ok := <-w.Errors:
			if !ok {
				return errors.New("fsnotify Errors channel closed")
			}
			slog.Warn("fsnotify error", "err", e)
		case path := <-out:
			// 500ms quiet — caller must re-stat and read fresh.
			slog.Info("watcher debounced", "path", filepath.Base(path))
			onChange(path)
		}
	}
}
