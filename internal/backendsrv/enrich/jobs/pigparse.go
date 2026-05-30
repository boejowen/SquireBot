package jobs

// pigparse.go is the daily PigParse price job (ENRICH-10) — the Go replacement
// for refreshPigparse.ts's runUnderLock flow. It composes the Wave-1 pieces and
// authors ZERO inline SQL (mirrors ingest/handler.go::bindAndReplace):
//
//	store.GetETag → politefetch.Fetch → enrich.ParseToRows
//	  → FILTER to WTS (t=0) [D-9] → truncation guard as a LOG [D-4]
//	  → store.UpsertPigparsePricesTx over ONE tx → store.SetETag + store.SetJobRun
//
// Two genuine Go-vs-Sheet behaviors live HERE (not in the parser/store):
//   - D-9: PigParse returns two rows per item_id (t=0 WTS, t=1 WTB) but
//     pigparse_price.item_id is a PRIMARY KEY. We keep ONLY the WTS (t=0) row as
//     ONE isolated filter (so flipping to last-wins or a (id,direction) key later
//     is a one-line change). The Sheet kept both dups; the backend has fewer rows
//     — expected, NOT a failure (note it in the D-7 parity check).
//   - D-4: the truncation guard (today's kept-row count < 90% of last-known) is a
//     LOG, not an abort. Because UpsertPigparsePricesTx is a per-item upsert, a
//     truncated response updates what it got and leaves the rest — so we PROCEED
//     and never clobber good rows wholesale (the Sheet ABORTED here; we do not).
//
// On a 304 (FromCache) we SKIP the parse + write entirely (Pitfall 6) — the empty
// body is never fed to a parser/replace that would wipe good rows. The job
// advances job_run on EVERY outcome (ok|skipped_unchanged|error) so a failing
// fetch retries on the next daily window, not every scheduler tick.

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// pigparseJobName is the job_run position-marker key + the slog op for this job.
const pigparseJobName = "pigparse_daily"

// rowCountFloorPct mirrors refreshPigparse.ts ROW_COUNT_FLOOR_PCT: if today's
// kept-row count is below 90% of the last-known count, the truncation guard
// fires (a LOG — D-4 — never an abort).
const rowCountFloorPct = 0.90

// RunPigparse runs the daily PigParse price pull: it reads the cached ETag,
// fetches getall/1 (304-aware), parses, keeps only the WTS (t=0) rows, logs a
// truncation guard if the response shrank, upserts the prices over one tx, and
// advances the job_run + etag position markers. fetch is injected
// (politefetch.Fetch in production; a fake in tests). It returns a non-nil error
// only on a fetch/parse/DB failure (the caller logs + continues; the marker
// already advanced).
func RunPigparse(ctx context.Context, db *sql.DB, fetch politefetch.Fetcher) error {
	s := store.NewStore(db)
	now := time.Now().UTC()

	// 1) Conditional-request state for the 304 short-circuit.
	etag, lastMod, err := s.GetETag(ctx, PigparseURL)
	if err != nil {
		// A cache-read failure shouldn't block the pull — log and fetch
		// unconditionally (etag/lastMod stay "").
		slog.Warn(pigparseJobName+": etag read failed; fetching unconditionally", "err", err)
		etag, lastMod = "", ""
	}

	// 2) Fetch.
	res := fetch(ctx, PigparseURL, politefetch.Options{ETag: etag, LastModified: lastMod})

	// 3) Fetch failure → record error + return (advance-always; the daily cadence
	// retries next window rather than hot-looping).
	if !res.OK {
		slog.Warn(pigparseJobName+": fetch failed", "status", res.Status, "err", res.Err)
		detail := fmt.Sprintf("fetch_failed status=%d", res.Status)
		_ = s.SetJobRun(ctx, pigparseJobName, now, "error", detail)
		if res.Err != nil {
			return fmt.Errorf("%s: fetch: %w", pigparseJobName, res.Err)
		}
		return fmt.Errorf("%s: fetch failed (status=%d)", pigparseJobName, res.Status)
	}

	// 4) 304 unchanged → SKIP parse + write entirely (Pitfall 6). The empty body
	// is never parsed; the good rows are left exactly as they are.
	if res.FromCache {
		slog.Info(pigparseJobName + ": unchanged (304); skipping parse+write")
		return s.SetJobRun(ctx, pigparseJobName, now, "skipped_unchanged", "304")
	}

	// 5) Parse the getall body (returns BOTH t=0 and t=1 rows; no dedup yet).
	rawRows, err := enrich.ParseToRows(res.Body)
	if err != nil {
		slog.Error(pigparseJobName+": parse failed", "err", err)
		_ = s.SetJobRun(ctx, pigparseJobName, now, "error", "parse_failed")
		return fmt.Errorf("%s: parse: %w", pigparseJobName, err)
	}

	// 6) D-9 — keep ONLY the WTS (t=0) row per item_id. ONE isolated filter:
	// flipping to last-wins or a (item_id, direction) composite later is a
	// one-line change here. (in-place filter; no extra allocation.)
	kept := rawRows[:0]
	for _, r := range rawRows {
		if r.T == 0 { // 0 = WTS / sell-side — the standard "what's it worth to sell"
			kept = append(kept, r)
		}
	}

	// 7) D-4 — truncation guard: a LOG, never an abort. Read the last-known kept
	// count from the prior job_run.last_detail ("rows=N"); if today's kept count
	// dropped below 90% of it, WARN and PROCEED (the per-item upsert leaves
	// unmentioned rows intact, so a partial response never clobbers good data).
	if lastKnown := lastKnownRowCount(ctx, s); lastKnown > 0 &&
		float64(len(kept)) < rowCountFloorPct*float64(lastKnown) {
		slog.Warn(pigparseJobName+": truncation guard", "today", len(kept), "last", lastKnown, "floor_pct", rowCountFloorPct)
		// NO return here — D-4: log then proceed.
	}

	// 8) Map kept enrich.PigparseRow → store.PigparsePrice. current_avg/blue_volume
	// are the Sheet's a30/t30 aliases (UpsertPigparsePricesTx sets them); direction
	// is strconv.Itoa(r.T) which is always "0" after the WTS filter.
	nowStr := now.Format(time.RFC3339)
	prices := make([]store.PigparsePrice, 0, len(kept))
	for _, r := range kept {
		prices = append(prices, store.PigparsePrice{
			ItemID:        r.I,
			Name:          r.N,
			Direction:     strconv.Itoa(r.T),
			T30:           r.T30,
			A30:           r.A30,
			T60:           r.T60,
			A60:           r.A60,
			T6m:           r.T6m,
			A6m:           r.A6m,
			Ty:            r.Ty,
			Ay:            r.Ay,
			LastSeen:      r.L,
			LastRefreshed: nowStr,
		})
	}

	// 9) Upsert all prices over ONE tx (compose the store method; no inline SQL).
	tx, err := db.BeginTx(ctx, nil) // BEGIN IMMEDIATE (single-writer DSN)
	if err != nil {
		_ = s.SetJobRun(ctx, pigparseJobName, now, "error", "begin_tx_failed")
		return fmt.Errorf("%s: begin tx: %w", pigparseJobName, err)
	}
	defer tx.Rollback() // no-op after Commit
	if err := store.UpsertPigparsePricesTx(ctx, tx, prices); err != nil {
		_ = s.SetJobRun(ctx, pigparseJobName, now, "error", "upsert_failed")
		return fmt.Errorf("%s: upsert: %w", pigparseJobName, err)
	}
	if err := tx.Commit(); err != nil {
		_ = s.SetJobRun(ctx, pigparseJobName, now, "error", "commit_failed")
		return fmt.Errorf("%s: commit: %w", pigparseJobName, err)
	}

	// 10) Persist the ETag (304 next time if unchanged) + advance the position
	// marker, stashing the kept-row count in last_detail so the NEXT run can
	// compute the truncation ratio.
	if err := s.SetETag(ctx, PigparseURL, res.ETag, res.LastModified); err != nil {
		// Non-fatal: the data is written; a stale/absent ETag just means we
		// re-fetch unconditionally next time.
		slog.Warn(pigparseJobName+": set etag failed", "err", err)
	}
	detail := fmt.Sprintf("rows=%d", len(prices))
	slog.Info(pigparseJobName+": ok", "rows", len(prices), "retries", res.RetriesUsed)
	return s.SetJobRun(ctx, pigparseJobName, now, "ok", detail)
}

// lastKnownRowCount reads the prior run's kept-row count from job_run.last_detail
// (which this job writes as "rows=N" on a successful run). It returns 0 when the
// job has never run, the last run was an error/skip (no "rows=" detail), or the
// detail is unparseable — in all those cases the truncation guard is simply not
// applied (there is no baseline to compare against). The read goes through the
// store (single tested SQL path), not an inline job query.
func lastKnownRowCount(ctx context.Context, s *store.Store) int {
	detail, err := s.GetJobRunDetail(ctx, pigparseJobName)
	if err != nil {
		return 0
	}
	return parseRowsDetail(detail)
}

// parseRowsDetail extracts N from a "rows=N" detail string (the only detail this
// job writes on success). Returns 0 for any other shape (error/skip details, or
// a malformed value), so the guard is skipped rather than misfiring.
func parseRowsDetail(detail string) int {
	const prefix = "rows="
	if !strings.HasPrefix(detail, prefix) {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(detail[len(prefix):]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
