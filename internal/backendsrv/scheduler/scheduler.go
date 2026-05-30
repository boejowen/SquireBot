// Package scheduler is the SquireBot backend's in-process job registry
// (BACKEND-01: "single Go binary + in-process scheduler"; ENRICH-10/11). Per the
// 11-01 verdict (HAND-ROLLED Go fallback, NOT PocketBase) there is no app.Cron()
// — it is a stdlib time.Ticker goroutine driving a poll-and-check loop over a
// small registry of named, cadenced jobs.
//
// Two real jobs are registered (P12, fleshing out the 11-05 no-op skeleton):
//
//	pigparse_daily — runs jobs.RunPigparse when now-last_run_at >= 24h.
//	wiki_weekly    — runs jobs.RunWiki on Sunday UTC, once per Sunday.
//
// Restart-safety is deterministic via the durable job_run.last_run_at cursor
// (store.GetJobRun / SetJobRun, created by the 00003 migration):
//
//   - On Start the registry loads each job's last_run_at (a NULL/absent row ⇒
//     zero time ⇒ due), then does an IMMEDIATE check pass BEFORE entering the
//     ticker loop. So a job that was due while the process was down fires within
//     seconds of restart (mirroring internal/heartbeat/heartbeat.go's immediate
//     first fire) — never skipped a window.
//   - last_run_at advances AFTER every run (advance-always, even on error — A2),
//     so a job can't double-run (Due returns false until the next window) and a
//     persistently-failing fetch retries on its next cadence window (24h / next
//     Sunday), NOT every 10-minute tick. The politeFetch backoff handles
//     transient failures within a single run.
//
// A per-job sync.Mutex (TryLock → skip-not-queue) replaces the Sheet's
// LockService.getDocumentLock(): it ensures one cycle of a job never overlaps
// another. The DB's SetMaxOpenConns(1) (11-02) already serializes writes, so the
// mutex is about not launching a redundant fetch+parse cycle, not DB safety; the
// two jobs have separate mutexes and may run concurrently on a Sunday.
//
// The ticker-goroutine ergonomics + the ctx.Done() clean-shutdown branch are kept
// VERBATIM from the 11-05 skeleton (already tested by
// scheduler_test.go::TestRun_ReturnsOnContextCancel): a select on ctx.Done() vs
// ticker.C, returning cleanly when the context is cancelled (SIGINT/SIGTERM via
// the server's signal.NotifyContext). Only the heartbeat body + the 1h interval
// are replaced.
package scheduler

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/jobs"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// checkInterval is the poll-and-check cadence: every tick, each registered job's
// Due predicate is evaluated against its persisted last_run_at. 10 minutes is
// fine — a daily job fired up to 10 min late is irrelevant, and the immediate
// check pass on startup means a missed window fires within seconds of restart,
// not after a full interval. (Replaces the 11-05 skeleton's 1-hour heartbeat tick
// — that placeholder constant is intentionally GONE, per RESEARCH §"checkInterval
// choice".)
const checkInterval = 10 * time.Minute

// Job is a named, cadenced unit of work in the registry. Due decides whether the
// job should run given its last persisted run time and the current time; Run does
// the work (composing fetch → parse → upsert in the enrich/jobs package); mu is
// the per-job lock (TryLock-skip) that replaces the Apps Script LockService so two
// cycles of the SAME job never overlap.
type Job struct {
	Name string                            // job_run.job_name key: 'pigparse_daily' | 'wiki_weekly'
	Due  func(last, now time.Time) bool    // cadence predicate (D-10; no cron lib)
	Run  func(ctx context.Context) error   // the work (jobs.RunPigparse / RunWiki, fetch injected)
	mu   sync.Mutex                        // per-job: TryLock to skip an overlapping cycle (LockService replacement)
}

// duePigparse is the daily cadence (D-10): due when at least 24h have elapsed
// since the last run. A zero last (never run / NULL cursor) makes now.Sub(last)
// enormous ⇒ due on the first check pass after startup (the "due-on-startup-if-
// missed" signal). The simpler >=24h predicate is restart-robust and adequate at
// the guild's scale (no wall-clock 03:00 pinning needed — RESEARCH §"Cadence
// predicates").
func duePigparse(last, now time.Time) bool {
	return now.Sub(last) >= 24*time.Hour
}

// dueWiki is the weekly cadence (D-10): due when it is Sunday (UTC) AND the last
// run precedes the start of THIS Sunday (00:00 UTC). This guarantees exactly one
// run per Sunday: once the Sunday run records last_run_at, last is no longer
// before startOfSundayUTC(now), so Due is false for the rest of the day. A zero
// last is before any real Sunday ⇒ due on the first Sunday after startup. A
// missed Sunday (server down all day) simply runs on the next Sunday — never
// double-runs, never silently skips a still-current Sunday.
func dueWiki(last, now time.Time) bool {
	return now.Weekday() == time.Sunday && last.Before(startOfSundayUTC(now))
}

// startOfSundayUTC returns the most recent Sunday 00:00:00 UTC at or before now:
// now converted to UTC, truncated to the day, minus now.Weekday() days (Sunday=0,
// so on a Sunday it truncates to today's midnight; on a Wednesday it backs up to
// the prior Sunday). Used by dueWiki to bound "this Sunday's window".
func startOfSundayUTC(now time.Time) time.Time {
	u := now.UTC()
	midnight := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.AddDate(0, 0, -int(u.Weekday())) // Weekday(): Sunday=0 … Saturday=6
}

// Start launches the in-process scheduler in a new goroutine and returns
// immediately (non-blocking — the server's main goroutine owns the HTTP
// listener). It builds the registry with the two real jobs wired to the
// production politefetch.Fetch, then hands the goroutine's lifetime to ctx: the
// server cancels ctx on SIGINT/SIGTERM and the loop unwinds (matching the
// watcher's heartbeat shape + the server's signal.NotifyContext-driven shutdown).
//
// db is the migrated SQLite handle (the scheduler needs it for the job_run cursor
// and to pass to the jobs' Tx composition). No Google/OAuth/Sheets dependency is
// introduced — the jobs talk only to the community PigParse/wiki HTTP APIs and
// the local DB.
func Start(ctx context.Context, db *sql.DB) {
	registry := []*Job{
		{
			Name: "pigparse_daily",
			Due:  duePigparse,
			Run: func(ctx context.Context) error {
				return jobs.RunPigparse(ctx, db, politefetch.Fetch)
			},
		},
		{
			Name: "wiki_weekly",
			Due:  dueWiki,
			Run: func(ctx context.Context) error {
				return jobs.RunWiki(ctx, db, politefetch.Fetch)
			},
		},
	}
	go run(ctx, db, registry)
}

// run is the poll-and-check loop. Split out from Start so a test can drive it
// directly (with a short-deadline context + a custom registry) and assert both
// the immediate check pass and the ctx-cancel shutdown.
//
//  1. Load each job's last_run_at (zero time when the cursor is absent ⇒ due).
//  2. IMMEDIATE check pass BEFORE the ticker loop (heartbeat precedent) so a
//     missed job fires within seconds of startup, not after one checkInterval.
//  3. The ticker loop: on each tick, run every due job. The ctx.Done() branch is
//     KEPT VERBATIM from the 11-05 skeleton (the tested clean-shutdown contract).
func run(ctx context.Context, db *sql.DB, registry []*Job) {
	s := store.NewStore(db)

	// Durable cursor: last known run time per job. A missing/NULL row ⇒ zero time
	// ⇒ Due (the "never run ⇒ due-on-startup" signal). A read error is logged and
	// the job treated as never-run (zero time) — failing safe toward running.
	last := make(map[string]time.Time, len(registry))
	for _, job := range registry {
		lastRun, _, ok, err := s.GetJobRun(ctx, job.Name)
		if err != nil {
			slog.Warn("scheduler: read job cursor failed; treating as never-run", "job", job.Name, "err", err)
			lastRun = time.Time{}
		}
		_ = ok // ok=false already yields the zero time GetJobRun returns
		last[job.Name] = lastRun
	}

	slog.Info("scheduler started", "interval", checkInterval.String(), "jobs", len(registry))

	// Immediate check pass: run anything already due (a window missed while the
	// process was down) right now, before waiting a full checkInterval.
	checkAndRun(ctx, s, registry, last)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Clean shutdown (SIGINT/SIGTERM cancelled the root context). KEPT
			// VERBATIM from the skeleton — the tested shutdown contract.
			slog.Info("scheduler stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			checkAndRun(ctx, s, registry, last)
		}
	}
}

// checkAndRun evaluates every job's Due predicate against the current time and
// runs the due ones, advancing the in-memory `last` map for each job it runs.
// Shared by the immediate startup pass and every ticker tick.
func checkAndRun(ctx context.Context, s *store.Store, registry []*Job, last map[string]time.Time) {
	now := time.Now().UTC()
	for _, job := range registry {
		if job.Due(last[job.Name], now) {
			runJob(ctx, s, job)
			last[job.Name] = now
		}
	}
}

// runJob runs one due job under its per-job mutex and persists the cursor AFTER
// the run.
//
//   - TryLock (skip, not queue): if a previous cycle of THIS job is still running
//     (a long wiki crawl spanning a tick), the overlap is logged and skipped — the
//     LockService replacement. Separate jobs have separate mutexes and don't block
//     each other.
//   - SetJobRun AFTER the run, advance-always (even on error — A2): the scheduler
//     is the AUTHORITATIVE cursor writer. The load-bearing invariant is that
//     last_run_at advances after every attempt so a failing fetch retries on its
//     next cadence window rather than hot-looping every tick. (RunPigparse/RunWiki
//     ALSO write SetJobRun internally with richer detail; that earlier write is a
//     harmless idempotent overwrite — this final write is the one that guarantees
//     the cursor advanced even if a job returned before reaching its own SetJobRun.
//     Ownership choice per the plan's RECOMMEND: scheduler owns the authoritative
//     advance; the jobs' internal write provides observability detail.)
func runJob(ctx context.Context, s *store.Store, job *Job) {
	if !job.mu.TryLock() {
		slog.Warn("scheduler: job overlap skipped", "job", job.Name)
		return
	}
	defer job.mu.Unlock()

	now := time.Now().UTC()
	err := job.Run(ctx)
	status := "ok"
	detail := "scheduled"
	if err != nil {
		status = "error"
		detail = errDetail(err)
		slog.Warn("scheduler: job run failed", "job", job.Name, "err", err)
	}
	// Advance the durable cursor even on error (A2). A SetJobRun failure here is
	// logged inside the store method; we don't abort the loop over it.
	if serr := s.SetJobRun(ctx, job.Name, now, status, detail); serr != nil {
		slog.Error("scheduler: persist job cursor failed", "job", job.Name, "err", serr)
	}
}

// errDetail renders a short, log-safe detail string for the job_run cursor (the
// error message — counts/status only, never raw response bodies or secrets, V7).
// Empty err yields "" (callers pass a non-nil err here).
func errDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
