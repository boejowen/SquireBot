// Package heartbeat owns the WATCH-08 + OPS-05 daily heartbeat goroutine.
// Plan 02-05 Task 2.
//
// Cadence: 24-hour rolling INTERVAL (not wall-clock fixed time) per
// 02-CONTEXT.md "Heartbeat Cadence". Fires immediately on entry, then
// reschedules every Interval via a sleep-then-fire loop (the AfterFunc-
// style self-reschedule pattern from 02-RESEARCH.md Example 4 -- one
// job, no cron expression complexity, no DST concerns at UTC, and we
// avoid tick-pile-up on a hung write because each iteration starts a
// fresh sleep AFTER the previous tick returns).
//
// Suspension: when authSuspended.Load() is true the tick skips the API
// call but STILL schedules the next 24h fire -- the heartbeat resumes
// automatically once Plan 02-04's Reauthorize clears the flag, without
// requiring a watcher restart.
//
// Concurrency: this goroutine and the watcher goroutine share a single
// *sheet.Client. WriteHeartbeat goes through the mutex-funneled
// c.batchUpdate helper from Plan 02-03, so heartbeat fires cannot
// interleave with WriteInventory / WriteSpellbook calls. Pitfall D
// (RESEARCH.md) is closed.
package heartbeat

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/boejowen/SquireBot/internal/config"
)

// Interval is the cadence between heartbeat fires. 24h interval (NOT
// wall-clock daily) per 02-CONTEXT.md decision.
const Interval = 24 * time.Hour

// writer is the minimal interface heartbeat.Run needs. *sheet.Client
// satisfies it via its WriteHeartbeat method (Plan 02-05 Task 1). Defining
// the interface inside this package keeps tests independent of the live
// Sheets stack.
type writer interface {
	WriteHeartbeat(ctx context.Context, ownerEmail string, charNames []string, watcherVersion string) error
}

// sleepFn is the package-level sleep used during reschedule waits.
// Production uses realSleep (timer + select on ctx.Done()). Tests
// override via t.Cleanup-restored installFakeSleep -- see heartbeat_test.go.
var sleepFn = realSleep

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Run blocks until ctx is cancelled. Fires WriteHeartbeat once on entry,
// then every Interval. authSuspended is consulted on every tick: when
// true, the API call is skipped but the next tick is still scheduled
// (heartbeat resumes after Reauthorize clears the flag).
//
// The active-char roster is the union of cfg.LastKnownInventoryMtime
// and cfg.LastKnownSpellbookMtime keys -- a char appears in either map
// as soon as it has had a successful upload. A spellbook-only char
// (e.g., a level-1 alt scribed but never inventoried) is still picked
// up via LastKnownSpellbookMtime.
func Run(ctx context.Context, w writer, cfg *config.Config, ownerEmail, watcherVersion string, authSuspended *atomic.Bool) {
	tick := func() {
		if authSuspended != nil && authSuspended.Load() {
			slog.Info("heartbeat skipped: auth suspended")
			return
		}
		charNames := activeChars(cfg)
		if err := w.WriteHeartbeat(ctx, ownerEmail, charNames, watcherVersion); err != nil {
			// Non-fatal. Plan 02-03's withRetry already exhausted the
			// retry slice inside WriteHeartbeat; if the next tick lands
			// while auth is dead, the watcher's makeOnInventoryChange
			// path will trip globalAuthSuspended, and the subsequent tick
			// will skip cleanly via the authSuspended guard above.
			slog.Warn("heartbeat write failed", "err", err, "chars", len(charNames))
		} else {
			slog.Info("heartbeat written", "chars", len(charNames))
		}
	}

	// Immediate first fire.
	tick()

	// Self-reschedule loop.
	for {
		if err := sleepFn(ctx, Interval); err != nil {
			slog.Info("heartbeat goroutine exiting", "err", err)
			return
		}
		tick()
	}
}

// activeChars returns the union of char_names known from inventory and
// spellbook mtime maps. Either map being nil is fine.
func activeChars(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	seen := map[string]bool{}
	for charName := range cfg.LastKnownInventoryMtime {
		seen[charName] = true
	}
	for charName := range cfg.LastKnownSpellbookMtime {
		seen[charName] = true
	}
	out := make([]string, 0, len(seen))
	for charName := range seen {
		out = append(out, charName)
	}
	return out
}
