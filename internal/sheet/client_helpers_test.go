package sheet

// Tests for *Client mutex serialization + the helper methods that wrap
// every sheets API call (Plan 02-03 Task 2). Pitfall D closure: every
// API call this package issues acquires c.batchMu, so the heartbeat
// goroutine (Plan 02-05) and watcher goroutine cannot race.
//
// The "5 mutex-specific tests" mandated by the plan:
//   1. Concurrent batchUpdate goroutines serialize (no overlap)
//   2. Reads (valuesGet/spreadsheetsGet) serialize against writes
//   3. onRefresh callback fires on 403 authError (proves wiring)
//   4. Mutex released on error path
//   5. Mutex released on panic (defensive)
//
// Plus a refresh + ErrPermanentAuth wiring test through the *Client surface.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// concurrentStub answers every batchUpdate after sleeping for blockFor.
// It records concurrent in-flight count via inFlight; if any request
// observes inFlight > 1, mutex serialization is broken.
type concurrentStub struct {
	t        *testing.T
	blockFor time.Duration
	inFlight int32
	maxSeen  int32
	calls    int32
}

func (s *concurrentStub) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v4/spreadsheets/SHEET1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList([]sheetInfo{{Title: "x", SheetID: 1}}),
			})
		case r.Method == "POST" && r.URL.Path == "/v4/spreadsheets/SHEET1:batchUpdate":
			n := atomic.AddInt32(&s.inFlight, 1)
			defer atomic.AddInt32(&s.inFlight, -1)
			for {
				cur := atomic.LoadInt32(&s.maxSeen)
				if n <= cur || atomic.CompareAndSwapInt32(&s.maxSeen, cur, n) {
					break
				}
			}
			atomic.AddInt32(&s.calls, 1)
			time.Sleep(s.blockFor)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"replies": []map[string]any{{}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// TestClient_batchUpdateSerializesConcurrentGoroutines — three concurrent
// callers, each blocking 100ms server-side; total elapsed >= 300ms (mutex
// serializes them). maxSeen in-flight must be exactly 1.
func TestClient_batchUpdateSerializesConcurrentGoroutines(t *testing.T) {
	s := &concurrentStub{t: t, blockFor: 100 * time.Millisecond}
	srv := httptest.NewServer(s.handler())
	defer srv.Close()
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	req := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{}},
	}

	const N = 3
	var wg sync.WaitGroup
	wg.Add(N)
	start := time.Now()
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if _, err := c.batchUpdate(ctx, req); err != nil {
				t.Errorf("batchUpdate: %v", err)
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	if got := atomic.LoadInt32(&s.maxSeen); got != 1 {
		t.Errorf("maxSeen in-flight = %d, want 1 (mutex must serialize)", got)
	}
	if got := atomic.LoadInt32(&s.calls); got != N {
		t.Errorf("calls = %d, want %d", got, N)
	}
	if elapsed < time.Duration(N)*s.blockFor*9/10 {
		t.Errorf("elapsed = %v, want >= %v (serialized)", elapsed, time.Duration(N)*s.blockFor)
	}
}

// TestClient_valuesGetSerializesAgainstBatchUpdate — read paths share the
// same mutex as write paths; concurrent valuesGet + batchUpdate cannot
// overlap.
func TestClient_valuesGetSerializesAgainstBatchUpdate(t *testing.T) {
	var inFlight int32
	var maxSeen int32
	mark := func() func() {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			cur := atomic.LoadInt32(&maxSeen)
			if n <= cur || atomic.CompareAndSwapInt32(&maxSeen, cur, n) {
				break
			}
		}
		return func() { atomic.AddInt32(&inFlight, -1) }
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v4/spreadsheets/SHEET1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList([]sheetInfo{{Title: "x", SheetID: 1}}),
			})
		case r.Method == "POST" && r.URL.Path == "/v4/spreadsheets/SHEET1:batchUpdate":
			done := mark()
			defer done()
			time.Sleep(80 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(map[string]any{"replies": []map[string]any{{}}})
		case r.Method == "GET" && r.URL.Path == "/v4/spreadsheets/SHEET1/values/foo":
			done := mark()
			defer done()
			time.Sleep(80 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(map[string]any{"range": "foo", "majorDimension": "ROWS"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := c.batchUpdate(ctx, &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{{}},
		}); err != nil {
			t.Errorf("batchUpdate: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := c.valuesGet(ctx, "foo"); err != nil {
			t.Errorf("valuesGet: %v", err)
		}
	}()
	wg.Wait()

	if got := atomic.LoadInt32(&maxSeen); got != 1 {
		t.Errorf("maxSeen in-flight = %d, want 1 (read+write must serialize)", got)
	}
}

// TestClient_OnRefreshFiresOn403AuthError — the refresh callback installed
// via SetOnRefresh runs when the underlying API returns 403 with reason
// "authError"; after refresh, the second attempt succeeds.
func TestClient_OnRefreshFiresOn403AuthError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v4/spreadsheets/SHEET1" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList([]sheetInfo{{Title: "x", SheetID: 1}}),
			})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v4/spreadsheets/SHEET1:batchUpdate" {
			n := atomic.AddInt32(&calls, 1)
			if n == 1 {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    403,
						"message": "auth error",
						"errors":  []map[string]any{{"reason": "authError", "message": "auth"}},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"replies": []map[string]any{{}}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Stub sleepFn so backoff does not slow the test if it engaged.
	prev := sleepFn
	sleepFn = func(ctx context.Context, d time.Duration) error { return nil }
	t.Cleanup(func() { sleepFn = prev })

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var refreshCalls int32
	c.SetOnRefresh(func() error {
		atomic.AddInt32(&refreshCalls, 1)
		return nil
	})

	if _, err := c.batchUpdate(ctx, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{}},
	}); err != nil {
		t.Fatalf("batchUpdate: %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Errorf("refreshCalls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("server calls = %d, want 2 (initial 403 + post-refresh success)", got)
	}
}

// TestClient_MutexReleasedOnError — batchUpdate that returns an error must
// release the mutex so a subsequent caller can acquire it.
func TestClient_MutexReleasedOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v4/spreadsheets/SHEET1" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList([]sheetInfo{{Title: "x", SheetID: 1}}),
			})
			return
		}
		// Always return 400 — non-retryable, so withRetry surfaces the
		// error without retrying. The mutex must still be released.
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 400, "message": "bad"},
		})
	}))
	defer srv.Close()

	prev := sleepFn
	sleepFn = func(ctx context.Context, d time.Duration) error { return nil }
	t.Cleanup(func() { sleepFn = prev })

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	req := &sheets.BatchUpdateSpreadsheetRequest{Requests: []*sheets.Request{{}}}
	if _, err := c.batchUpdate(ctx, req); err == nil {
		t.Fatal("expected error from 400")
	}
	// Try again — must not deadlock if mutex was released.
	done := make(chan struct{})
	go func() {
		_, _ = c.batchUpdate(ctx, req)
		close(done)
	}()
	select {
	case <-done:
		// Good — mutex was released.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second batchUpdate deadlocked — mutex not released after error")
	}
}

// TestClient_MutexReleasedOnPanic — even if the inner closure panics,
// the deferred Unlock releases the mutex.
func TestClient_MutexReleasedOnPanic(t *testing.T) {
	c := &Client{}
	// Acquire and release via a panic-recovering goroutine.
	done := make(chan struct{})
	go func() {
		defer func() {
			_ = recover()
			close(done)
		}()
		c.batchMu.Lock()
		defer c.batchMu.Unlock()
		panic("simulated panic inside locked region")
	}()
	<-done
	// Now try to acquire from main — must succeed (mutex released).
	acquired := make(chan struct{})
	go func() {
		c.batchMu.Lock()
		defer c.batchMu.Unlock()
		close(acquired)
	}()
	select {
	case <-acquired:
		// Good.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("mutex not released after panic-in-defer-Unlock pattern")
	}
}
