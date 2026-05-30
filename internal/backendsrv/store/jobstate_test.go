package store

import (
	"context"
	"testing"
	"time"
)

// jobstate_test.go covers the scheduler cursor (job_run) + ETag cache (etag_cache)
// get/set methods over the shared NewTestDB fixture (00003 creates both tables):
//   - GetJobRun on an empty table → ok=false, zero time, nil err (the "never run
//     ⇒ due" signal); SetJobRun then GetJobRun round-trips the time via RFC3339;
//     a second SetJobRun updates in place (count stays 1 — upsert, not append).
//   - GetETag on empty → ("","",nil); SetETag then GetETag round-trips; re-SetETag
//     updates in place (count stays 1).

func TestJobRun_EmptyTableIsNotDueRecorded(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	lastRun, status, ok, err := s.GetJobRun(ctx, "pigparse_daily")
	if err != nil {
		t.Fatalf("GetJobRun on empty: %v", err)
	}
	if ok {
		t.Errorf("GetJobRun on empty table: ok = true, want false (no row recorded)")
	}
	if !lastRun.IsZero() {
		t.Errorf("GetJobRun on empty table: lastRun = %v, want zero time (never run ⇒ due)", lastRun)
	}
	if status != "" {
		t.Errorf("GetJobRun on empty table: status = %q, want empty string", status)
	}
}

func TestJobRun_SetThenGetRoundTripsAndUpserts(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// Use a timestamp that round-trips exactly through RFC3339 (second precision).
	t1 := time.Date(2026, 5, 29, 3, 0, 0, 0, time.UTC)
	if err := s.SetJobRun(ctx, "pigparse_daily", t1, "ok", "rows=7240"); err != nil {
		t.Fatalf("SetJobRun (first): %v", err)
	}

	lastRun, status, ok, err := s.GetJobRun(ctx, "pigparse_daily")
	if err != nil {
		t.Fatalf("GetJobRun after set: %v", err)
	}
	if !ok {
		t.Errorf("GetJobRun after set: ok = false, want true")
	}
	if !lastRun.Equal(t1) {
		t.Errorf("GetJobRun lastRun = %v, want %v (RFC3339 round-trip)", lastRun, t1)
	}
	if status != "ok" {
		t.Errorf("GetJobRun status = %q, want %q", status, "ok")
	}

	// Second SetJobRun with a later time + 'error' → updates in place (still one row).
	t2 := t1.Add(24 * time.Hour)
	if err := s.SetJobRun(ctx, "pigparse_daily", t2, "error", "fetch failed"); err != nil {
		t.Fatalf("SetJobRun (second): %v", err)
	}
	lastRun2, status2, _, err := s.GetJobRun(ctx, "pigparse_daily")
	if err != nil {
		t.Fatalf("GetJobRun after second set: %v", err)
	}
	if !lastRun2.Equal(t2) {
		t.Errorf("GetJobRun lastRun = %v after second set, want %v", lastRun2, t2)
	}
	if status2 != "error" {
		t.Errorf("GetJobRun status = %q after second set, want %q", status2, "error")
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM job_run`).Scan(&count); err != nil {
		t.Fatalf("count job_run: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 job_run row after two SetJobRun calls (upsert), got %d", count)
	}
}

func TestETag_EmptyReturnsBlank(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	etag, lastMod, err := s.GetETag(ctx, "https://wiki.project1999.com/api.php")
	if err != nil {
		t.Fatalf("GetETag on empty: %v", err)
	}
	if etag != "" || lastMod != "" {
		t.Errorf("GetETag on empty: (%q, %q), want (\"\", \"\")", etag, lastMod)
	}
}

func TestETag_SetThenGetRoundTripsAndUpserts(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	const url = "https://pigparse.azurewebsites.net/api/item/getall/1"
	const lm = "Wed, 21 Oct 2026 07:28:00 GMT"
	if err := s.SetETag(ctx, url, "etag-abc", lm); err != nil {
		t.Fatalf("SetETag (first): %v", err)
	}

	etag, gotLm, err := s.GetETag(ctx, url)
	if err != nil {
		t.Fatalf("GetETag after set: %v", err)
	}
	if etag != "etag-abc" {
		t.Errorf("GetETag etag = %q, want %q", etag, "etag-abc")
	}
	if gotLm != lm {
		t.Errorf("GetETag last_modified = %q, want %q", gotLm, lm)
	}

	// Re-SetETag updates in place (count stays 1).
	if err := s.SetETag(ctx, url, "etag-xyz", "Thu, 22 Oct 2026 07:28:00 GMT"); err != nil {
		t.Fatalf("SetETag (second): %v", err)
	}
	etag2, _, err := s.GetETag(ctx, url)
	if err != nil {
		t.Fatalf("GetETag after second set: %v", err)
	}
	if etag2 != "etag-xyz" {
		t.Errorf("GetETag etag = %q after second set, want %q", etag2, "etag-xyz")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM etag_cache`).Scan(&count); err != nil {
		t.Fatalf("count etag_cache: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 etag_cache row after two SetETag calls (upsert), got %d", count)
	}
}
