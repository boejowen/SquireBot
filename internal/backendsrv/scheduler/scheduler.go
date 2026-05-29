// Package scheduler is the SquireBot backend's in-process scheduler SKELETON
// (BACKEND-01: "single Go binary + in-process scheduler"). Per the 11-01 verdict
// (HAND-ROLLED Go fallback, NOT PocketBase) it is a stdlib time.Ticker goroutine
// — there is no app.Cron(). It registers NO real jobs: the PigParse/wiki
// enrichment jobs land in P12. P11 only proves the scheduler loop exists, fires,
// and shuts down cleanly on context cancel.
//
// The ticker-goroutine ergonomics mirror the watcher's existing long-running
// scheduler precedent (internal/heartbeat/heartbeat.go): a select on
// ctx.Done() vs ticker.C, returning cleanly when the context is cancelled
// (SIGINT/SIGTERM via the server's signal.NotifyContext). RESEARCH "Don't
// Hand-Roll" is explicit: do NOT build a general scheduler here — this is a
// placeholder P12 fills in with real cron expressions / jobs.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// HeartbeatInterval is the skeleton's tick cadence. It exists only to prove the
// scheduler loop fires while the server runs; P12 replaces this with the real
// enrichment-job schedule (daily PigParse, weekly wiki). One hour is a harmless,
// low-noise placeholder for a long-running process.
const HeartbeatInterval = time.Hour

// Start launches the in-process scheduler skeleton in a new goroutine and
// returns immediately (non-blocking — the server's main goroutine owns the HTTP
// listener). The goroutine ticks every HeartbeatInterval, logging a heartbeat,
// and exits cleanly when ctx is cancelled (the server cancels ctx on
// SIGINT/SIGTERM). It registers NO real jobs (P12).
//
// Returning the goroutine's lifetime to ctx (rather than a stop channel) matches
// the watcher's heartbeat shape and the server's signal.NotifyContext-driven
// shutdown: cancel the root context and every background loop unwinds.
func Start(ctx context.Context) {
	go run(ctx)
}

// run is the ticker loop. Split out from Start so a test can drive it directly
// with a short-deadline context and assert it returns on cancel.
func run(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	slog.Info("scheduler started", "interval", HeartbeatInterval.String(), "jobs", 0)
	for {
		select {
		case <-ctx.Done():
			// Clean shutdown (SIGINT/SIGTERM cancelled the root context).
			slog.Info("scheduler stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			// SKELETON: no real jobs until P12. Just prove the loop fires.
			slog.Info("scheduler heartbeat")
		}
	}
}
