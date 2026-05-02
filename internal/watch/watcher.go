// SECURITY/CORRECTNESS: This watcher follows the CLAUDE.md / RESEARCH.md §8.3 rule:
// it filters events purely by filename suffix and triggers a debouncer; the
// timer-fire dispatches a path to one of two OnChange callbacks (inventory or
// spellbook). The OnChange callbacks (Plan 02-02 wiring) re-stat and re-read
// the file fresh. We NEVER read ev.Op for ordering, ev.Name for content, or
// any other event payload field beyond the path. ev.Op is consulted only to
// drop fsnotify.Chmod (which on Windows is rare anyway). Spurious AV events
// become idempotent re-uploads — negligible cost.
package watch

import (
	"context"
	"errors"
	"fmt"
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

// File-suffix constants for the EQ /outputfile output. Exported so callers
// (catch-up scanners in package app, tests in this package) can re-use them.
const (
	InventorySuffix = "-Inventory.txt"
	SpellbookSuffix = "-Spellbook.txt"
)

// Run blocks. It watches every folder in `eqFolders` for *-Inventory.txt and
// *-Spellbook.txt events, debounces 500ms per path, and dispatches to
// onInventory or onSpellbook respectively after each quiet period. Files with
// neither suffix are ignored. Returns when ctx is cancelled or fsnotify
// channels close.
//
// Plan 02-02 (WATCH-02 + WATCH-03): supersedes the Phase 1 single-folder,
// inventory-only Run. fsnotify spans every folder; the debouncer is shared
// across folders and keys on the full path, so different files in different
// folders cannot collide.
func Run(ctx context.Context, eqFolders []string, onInventory, onSpellbook OnChange) error {
	if len(eqFolders) == 0 {
		return errors.New("watch.Run: no folders configured")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// Per CLAUDE.md / RESEARCH.md Pitfall #4: watch the parent directory, never
	// individual files. fsnotify on Windows uses ReadDirectoryChangesW; only
	// the parent-dir handle survives EQ's overwrite of the inventory/spellbook
	// file.
	for _, folder := range eqFolders {
		if err := w.Add(folder); err != nil {
			return fmt.Errorf("watch.Add %s: %w", folder, err)
		}
		slog.Info("watcher added folder", "folder", folder)
	}

	deb, out := NewDebouncer(500 * time.Millisecond)
	defer deb.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-w.Events:
			if !ok {
				return errors.New("fsnotify Events channel closed")
			}
			// Drop pure Chmod (Windows: rare but possible).
			if ev.Op == fsnotify.Chmod {
				continue
			}
			base := filepath.Base(ev.Name)
			if !strings.HasSuffix(base, InventorySuffix) && !strings.HasSuffix(base, SpellbookSuffix) {
				continue
			}
			// Per CLAUDE.md / RESEARCH.md §8.3: NEVER trust ev.Op or any
			// payload. Trigger the debouncer; on quiet, the receiver re-stats
			// + re-reads.
			deb.Trigger(ev.Name)
		case e, ok := <-w.Errors:
			if !ok {
				return errors.New("fsnotify Errors channel closed")
			}
			slog.Warn("fsnotify error", "err", e)
		case path := <-out:
			// 500ms quiet — caller must re-stat and read fresh.
			base := filepath.Base(path)
			slog.Info("watcher debounced", "path", base)
			switch {
			case strings.HasSuffix(base, InventorySuffix):
				onInventory(path)
			case strings.HasSuffix(base, SpellbookSuffix):
				onSpellbook(path)
			}
		}
	}
}
