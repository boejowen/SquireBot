package store

// jobstate.go holds the scheduler's durable cursor (job_run) and the polite
// HTTP client's ETag/304 state (etag_cache), both created by the 00003 migration.
// These are read-before / write-after helpers the scheduler + jobs call:
//
//	GetJobRun  → "is this job due?" (zero time / ok=false ⇒ never run ⇒ due)
//	SetJobRun  → advance the cursor AFTER each run (advance-always, even on error,
//	             so a failing fetch retries on the next cadence window, not every tick)
//	GetETag    → read the cached ETag/Last-Modified before a fetch (304 short-circuit)
//	SetETag    → store them after a successful 200+parse (untouched on 304)
//
// These are plain (*Store) methods doing a single Exec / single-row SELECT — they
// are independent of the dimension-write tx, so unlike enrich.go's *Tx methods they
// need no tx-composing variant. Parameterized ? placeholders only (V5). slog only
// on error; the url key is a hardcoded public API URL (no secret in this path, V7).

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// GetJobRun reads the durable last-run cursor for jobName. It returns:
//   - lastRun: the parsed last_run_at (zero time.Time when the row is absent OR
//     last_run_at is NULL — both mean "never run" ⇒ the job is due);
//   - status: the stored last_status ("" when absent);
//   - ok: true iff a job_run row exists for jobName;
//   - err: a non-nil error only on an unexpected DB / time-parse failure.
//
// A zero time is the load-bearing "due-on-startup-if-missed" signal: the
// scheduler treats now.Sub(zero) as enormous, so a never-run (or never-recorded)
// job fires on the first check pass after startup.
func (s *Store) GetJobRun(ctx context.Context, jobName string) (lastRun time.Time, status string, ok bool, err error) {
	var (
		lastRunAt sql.NullString
		lastStat  sql.NullString
	)
	qerr := s.db.QueryRowContext(ctx,
		`SELECT last_run_at, last_status FROM job_run WHERE job_name = ?`, jobName,
	).Scan(&lastRunAt, &lastStat)
	switch {
	case qerr == sql.ErrNoRows:
		return time.Time{}, "", false, nil // never run ⇒ due
	case qerr != nil:
		return time.Time{}, "", false, fmt.Errorf("read job_run (job=%q): %w", jobName, qerr)
	}

	if lastRunAt.Valid && lastRunAt.String != "" {
		parsed, perr := time.Parse(time.RFC3339, lastRunAt.String)
		if perr != nil {
			return time.Time{}, lastStat.String, true,
				fmt.Errorf("parse job_run.last_run_at %q (job=%q): %w", lastRunAt.String, jobName, perr)
		}
		lastRun = parsed
	} // else: NULL/empty last_run_at ⇒ leave lastRun as zero time (still ok=true)

	return lastRun, lastStat.String, true, nil
}

// GetJobRunDetail reads the last_detail string for jobName ("" when the row is
// absent or last_detail is NULL). The daily PigParse job writes "rows=N" here on
// a successful run; on the NEXT run it reads this back to compute the D-4
// truncation-guard ratio (today's kept count vs. the last-known count) — keeping
// that read on the single tested SQL path instead of an inline job query.
func (s *Store) GetJobRunDetail(ctx context.Context, jobName string) (string, error) {
	var detail sql.NullString
	qerr := s.db.QueryRowContext(ctx,
		`SELECT last_detail FROM job_run WHERE job_name = ?`, jobName,
	).Scan(&detail)
	switch {
	case qerr == sql.ErrNoRows:
		return "", nil
	case qerr != nil:
		return "", fmt.Errorf("read job_run.last_detail (job=%q): %w", jobName, qerr)
	}
	return detail.String, nil // "" when NULL
}

// SetJobRun upserts the cursor for jobName, storing lastRun as RFC3339 UTC. It is
// an upsert (ON CONFLICT(job_name) DO UPDATE) so there is exactly ONE row per job:
// the first run inserts, every subsequent run updates in place. Called AFTER each
// cycle with the run's status ('ok'|'error'|'skipped_unchanged') and a short
// detail (row counts / error summary) for observability.
func (s *Store) SetJobRun(ctx context.Context, jobName string, lastRun time.Time, status, detail string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO job_run (job_name, last_run_at, last_status, last_detail, updated_at)
		 VALUES (?,?,?,?,datetime('now'))
		 ON CONFLICT(job_name) DO UPDATE SET
		   last_run_at=excluded.last_run_at, last_status=excluded.last_status,
		   last_detail=excluded.last_detail, updated_at=datetime('now')`,
		jobName, lastRun.UTC().Format(time.RFC3339), status, detail,
	); err != nil {
		slog.Error("set job_run", "job", jobName, "status", status, "err", err)
		return fmt.Errorf("upsert job_run (job=%q): %w", jobName, err)
	}
	return nil
}

// GetETag reads the cached ETag + Last-Modified for url (both "" when the row is
// absent). The job passes these into the polite client's If-None-Match /
// If-Modified-Since headers to enable the 304 short-circuit.
func (s *Store) GetETag(ctx context.Context, url string) (etag, lastModified string, err error) {
	var (
		e  sql.NullString
		lm sql.NullString
	)
	qerr := s.db.QueryRowContext(ctx,
		`SELECT etag, last_modified FROM etag_cache WHERE url = ?`, url,
	).Scan(&e, &lm)
	switch {
	case qerr == sql.ErrNoRows:
		return "", "", nil
	case qerr != nil:
		slog.Error("get etag_cache", "url", url, "err", qerr)
		return "", "", fmt.Errorf("read etag_cache (url=%q): %w", url, qerr)
	}
	return e.String, lm.String, nil
}

// SetETag upserts the ETag + Last-Modified for url after a successful 200+parse
// (ON CONFLICT(url) DO UPDATE ⇒ one row per url, updated in place). On a 304 the
// job leaves the cached row untouched (does NOT call this), per Pitfall 6.
func (s *Store) SetETag(ctx context.Context, url, etag, lastModified string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO etag_cache (url, etag, last_modified, fetched_at)
		 VALUES (?,?,?,datetime('now'))
		 ON CONFLICT(url) DO UPDATE SET
		   etag=excluded.etag, last_modified=excluded.last_modified, fetched_at=datetime('now')`,
		url, etag, lastModified,
	); err != nil {
		slog.Error("set etag_cache", "url", url, "err", err)
		return fmt.Errorf("upsert etag_cache (url=%q): %w", url, err)
	}
	return nil
}
