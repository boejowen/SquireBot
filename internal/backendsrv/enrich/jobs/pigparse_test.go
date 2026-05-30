package jobs

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// fakeFetcher returns a canned FetchResult for any URL, so the jobs can be
// tested with no real network. The closure captures whatever the test wants the
// client to "return".
func fakeFetcher(res politefetch.FetchResult) politefetch.Fetcher {
	return func(ctx context.Context, url string, opts politefetch.Options) politefetch.FetchResult {
		return res
	}
}

// readPigparseFixture loads the real 7,240-row PigParse getall capture used by
// the parser tests, so the job test exercises the genuine parse + WTS filter.
func readPigparseFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../testdata/pigparse-getall-1.json")
	if err != nil {
		t.Fatalf("read pigparse fixture: %v", err)
	}
	return b
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// TestRunPigparse_UpsertsWTSOnly: a 200 with the real getall body upserts ONE
// row per distinct t=0 (WTS) item_id — the t=1 (WTB) duplicate rows are dropped
// (D-9) — and item 19450 carries the t=0 price (a30=239, t30=30), not the t=1
// values (a30=0, t30=2). job_run for 'pigparse_daily' is 'ok'.
func TestRunPigparse_UpsertsWTSOnly(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	body := readPigparseFixture(t)
	fetch := fakeFetcher(politefetch.FetchResult{OK: true, Status: 200, Body: body, ETag: `"v1"`})

	if err := RunPigparse(ctx, db, fetch); err != nil {
		t.Fatalf("RunPigparse: %v", err)
	}

	// 4,333 distinct t=0 ids in the fixture (verified out-of-band) — assert the
	// row count equals the WTS subset, not the 7,240 raw rows nor the 4,603
	// all-direction distinct ids.
	got := countRows(t, db, "pigparse_price")
	if got != 4333 {
		t.Errorf("pigparse_price rows = %d, want 4333 (one per distinct WTS item_id)", got)
	}

	// Item 19450: must hold the t=0 (WTS) price, not the t=1 (WTB) price.
	var currentAvg float64
	var blueVolume int
	var direction string
	err := db.QueryRow(
		`SELECT current_avg, blue_volume, direction FROM pigparse_price WHERE item_id = ?`, 19450,
	).Scan(&currentAvg, &blueVolume, &direction)
	if err != nil {
		t.Fatalf("query item 19450: %v", err)
	}
	if currentAvg != 239 { // a30 of the t=0 row
		t.Errorf("item 19450 current_avg = %v, want 239 (the WTS/t=0 a30)", currentAvg)
	}
	if blueVolume != 30 { // t30 of the t=0 row
		t.Errorf("item 19450 blue_volume = %d, want 30 (the WTS/t=0 t30)", blueVolume)
	}
	if direction != "0" { // WTS
		t.Errorf("item 19450 direction = %q, want \"0\" (WTS)", direction)
	}

	assertJobStatus(t, db, "pigparse_daily", "ok")
}

// TestRunPigparse_304_SkipsWrite: a pre-seeded row must survive a 304
// (FromCache) untouched — the empty body is NEVER fed to the parser/writer
// (Pitfall 6) — and job_run records 'skipped_unchanged'.
func TestRunPigparse_304_SkipsWrite(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	// Pre-seed one good row.
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, blue_volume, last_seen, direction, last_refreshed)
		 VALUES (?,?,?,?,?,?,?)`,
		777, "Sentinel Item", 123.0, 5, "2026-05-01T00:00:00Z", "0", "2026-05-01T00:00:00Z",
	); err != nil {
		t.Fatalf("pre-seed: %v", err)
	}

	fetch := fakeFetcher(politefetch.FetchResult{OK: true, Status: 304, FromCache: true})
	if err := RunPigparse(ctx, db, fetch); err != nil {
		t.Fatalf("RunPigparse(304): %v", err)
	}

	if got := countRows(t, db, "pigparse_price"); got != 1 {
		t.Errorf("pigparse_price rows = %d, want 1 (304 must not wipe/append)", got)
	}
	var name string
	var avg float64
	if err := db.QueryRow(`SELECT name, current_avg FROM pigparse_price WHERE item_id = ?`, 777).
		Scan(&name, &avg); err != nil {
		t.Fatalf("query sentinel: %v", err)
	}
	if name != "Sentinel Item" || avg != 123.0 {
		t.Errorf("sentinel row changed after 304: name=%q avg=%v, want unchanged", name, avg)
	}
	assertJobStatus(t, db, "pigparse_daily", "skipped_unchanged")
}

// TestRunPigparse_TruncationGuardLogsButWrites: with a high last-known count in
// job_run.last_detail, a small valid body still upserts (the guard LOGS, never
// aborts — D-4). The row count reflects the small body.
func TestRunPigparse_TruncationGuardLogsButWrites(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	// Pretend yesterday wrote 5,000 rows.
	s := store.NewStore(db)
	if err := s.SetJobRun(ctx, "pigparse_daily", timeNowUTC(), "ok", "rows=5000"); err != nil {
		t.Fatalf("seed prior job_run: %v", err)
	}

	// A tiny but valid getall body (2 rows, both WTS).
	small := []byte(`[
	  {"i":1,"t":0,"n":"Item One","l":"2026-05-09T00:00:00Z","a30":10,"t30":3},
	  {"i":2,"t":0,"n":"Item Two","l":"2026-05-09T00:00:00Z","a30":20,"t30":4}
	]`)
	fetch := fakeFetcher(politefetch.FetchResult{OK: true, Status: 200, Body: small, ETag: `"v2"`})

	if err := RunPigparse(ctx, db, fetch); err != nil {
		t.Fatalf("RunPigparse(small): %v", err)
	}

	// The load-bearing assertion: it WROTE despite the truncation (no abort).
	if got := countRows(t, db, "pigparse_price"); got != 2 {
		t.Errorf("pigparse_price rows = %d, want 2 (truncation guard must LOG, not abort)", got)
	}
	assertJobStatus(t, db, "pigparse_daily", "ok")
}

// TestRunPigparse_FetchError_RecordsError: a non-OK fetch records job_run
// 'error' and returns the error (advance-always; no panic, no write).
func TestRunPigparse_FetchError_RecordsError(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	fetch := fakeFetcher(politefetch.FetchResult{OK: false, Status: 503, Err: errFakeFetch})
	if err := RunPigparse(ctx, db, fetch); err == nil {
		t.Fatalf("RunPigparse: want non-nil error on fetch failure")
	}

	if got := countRows(t, db, "pigparse_price"); got != 0 {
		t.Errorf("pigparse_price rows = %d, want 0 (a failed fetch writes nothing)", got)
	}
	assertJobStatus(t, db, "pigparse_daily", "error")
}
