package politefetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// sleepRecorder swaps the package-level sleepFn for the duration of a test so
// retry/backoff waits are INSTANT (no real time passes) while recording every
// requested duration. It mirrors the watcher's checkSleepCapture seam
// (internal/update/check.go). It returns a restore func; callers defer it.
type sleepRecorder struct {
	mu    sync.Mutex
	calls []time.Duration
}

func (s *sleepRecorder) sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	s.calls = append(s.calls, d)
	s.mu.Unlock()
	return nil // instant — no real wait in tests
}

func (s *sleepRecorder) durations() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.calls))
	copy(out, s.calls)
	return out
}

// installSleepRecorder overrides sleepFn with an instant recorder and returns
// (recorder, restore). Always `defer restore()`.
func installSleepRecorder(t *testing.T) (*sleepRecorder, func()) {
	t.Helper()
	rec := &sleepRecorder{}
	prev := sleepFn
	sleepFn = rec.sleep
	return rec, func() { sleepFn = prev }
}

func TestFetch_200_CapturesETag(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "abc")
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2025 07:28:00 GMT")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{})
	if !res.OK {
		t.Fatalf("OK = false, want true; Err=%v Status=%d", res.Err, res.Status)
	}
	if res.Status != 200 {
		t.Errorf("Status = %d, want 200", res.Status)
	}
	if string(res.Body) != `{"ok":true}` {
		t.Errorf("Body = %q, want %q", res.Body, `{"ok":true}`)
	}
	if res.ETag != "abc" {
		t.Errorf("ETag = %q, want %q", res.ETag, "abc")
	}
	if res.LastModified != "Wed, 21 Oct 2025 07:28:00 GMT" {
		t.Errorf("LastModified = %q, want the served Last-Modified", res.LastModified)
	}
	if res.FromCache {
		t.Errorf("FromCache = true, want false on a 200")
	}
	if res.RetriesUsed != 0 {
		t.Errorf("RetriesUsed = %d, want 0", res.RetriesUsed)
	}
	if len(rec.durations()) != 0 {
		t.Errorf("sleepFn called %d times on a clean 200, want 0", len(rec.durations()))
	}
}

func TestFetch_SendsConditionalHeaders(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	var gotINM, gotIMS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotINM = r.Header.Get("If-None-Match")
		gotIMS = r.Header.Get("If-Modified-Since")
		w.WriteHeader(304)
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{ETag: "abc", LastModified: "Wed, 21 Oct 2025 07:28:00 GMT"})

	if gotINM != "abc" {
		t.Errorf("If-None-Match request header = %q, want %q", gotINM, "abc")
	}
	if gotIMS != "Wed, 21 Oct 2025 07:28:00 GMT" {
		t.Errorf("If-Modified-Since request header = %q, want the opts.LastModified", gotIMS)
	}
	if !res.OK {
		t.Fatalf("OK = false on 304, want true (304 is a success/short-circuit)")
	}
	if !res.FromCache {
		t.Errorf("FromCache = false on 304, want true")
	}
	if len(res.Body) != 0 {
		t.Errorf("Body = %q on 304, want empty", res.Body)
	}
	_ = rec
}

func TestFetch_304_ShortCircuits(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(304)
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{ETag: "abc", LastModified: "lm"})
	if !res.OK || !res.FromCache {
		t.Fatalf("304 should short-circuit: OK=%v FromCache=%v", res.OK, res.FromCache)
	}
	if res.Status != 304 {
		t.Errorf("Status = %d, want 304", res.Status)
	}
	if res.Body != nil {
		t.Errorf("Body = %q, want nil on 304 (Pitfall 6: job must not parse it)", res.Body)
	}
	// On 304 the client echoes back the supplied etag/lastmod so the job's
	// SetETag(...) keeps the cache row coherent if it chooses to refresh it.
	if res.ETag != "abc" || res.LastModified != "lm" {
		t.Errorf("304 should echo opts etag/lastmod: ETag=%q LastModified=%q", res.ETag, res.LastModified)
	}
	if len(rec.durations()) != 0 {
		t.Errorf("sleepFn called on a 304, want 0")
	}
}

func TestFetch_RetriesOn503ThenSucceeds(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	var calls int32
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n <= 2 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{})
	if !res.OK {
		t.Fatalf("OK = false after 503,503,200; want true. Err=%v", res.Err)
	}
	if string(res.Body) != "recovered" {
		t.Errorf("Body = %q, want %q", res.Body, "recovered")
	}
	if res.RetriesUsed != 2 {
		t.Errorf("RetriesUsed = %d, want 2 (two 503s before the 200)", res.RetriesUsed)
	}
	// Slept twice: between attempt0->1 and attempt1->2, using the schedule
	// [2s,4s] since no Retry-After was sent.
	got := rec.durations()
	want := []time.Duration{2 * time.Second, 4 * time.Second}
	if len(got) != len(want) {
		t.Fatalf("sleep calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sleep[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestFetch_RetryAfterHonored(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	var calls int32
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{})
	if !res.OK {
		t.Fatalf("OK = false after 429(Retry-After:1),200; want true. Err=%v", res.Err)
	}
	if res.RetriesUsed != 1 {
		t.Errorf("RetriesUsed = %d, want 1", res.RetriesUsed)
	}
	got := rec.durations()
	if len(got) != 1 {
		t.Fatalf("sleep calls = %v, want exactly 1", got)
	}
	if got[0] != 1*time.Second {
		t.Errorf("honored wait = %v, want 1s (the Retry-After value, NOT the 2s schedule default)", got[0])
	}
}

func TestFetch_RetryAfterClamped(t *testing.T) {
	// A Retry-After above the 600s ceiling must clamp to 600s (defensive — a
	// hostile/buggy server can't make us sleep for hours).
	if d := parseRetryAfter("700"); d != 600*time.Second {
		t.Errorf("parseRetryAfter(700) = %v, want 600s (clamped)", d)
	}
	if d := parseRetryAfter("0"); d != 0 {
		t.Errorf("parseRetryAfter(0) = %v, want 0 (caller falls back to schedule)", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Errorf("parseRetryAfter(\"\") = %v, want 0 (absent)", d)
	}
	if d := parseRetryAfter("not-a-number"); d != 0 {
		t.Errorf("parseRetryAfter(garbage) = %v, want 0 (unparseable)", d)
	}
	if d := parseRetryAfter("Wed, 21 Oct 2025 07:28:00 GMT"); d != 0 {
		t.Errorf("parseRetryAfter(http-date) = %v, want 0 (TS handles delta-seconds only)", d)
	}
	if d := parseRetryAfter("5"); d != 5*time.Second {
		t.Errorf("parseRetryAfter(5) = %v, want 5s", d)
	}
}

func TestFetch_NonRetriable404_Immediate(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{})
	if res.OK {
		t.Fatalf("OK = true on 404, want false")
	}
	if res.Status != 404 {
		t.Errorf("Status = %d, want 404", res.Status)
	}
	if res.RetriesUsed != 0 {
		t.Errorf("RetriesUsed = %d, want 0 (non-retriable surfaces immediately)", res.RetriesUsed)
	}
	if res.Err == nil {
		t.Errorf("Err = nil on 404, want non-nil")
	}
	if n := len(rec.durations()); n != 0 {
		t.Errorf("sleepFn invoked %d times on a 404, want 0 (no sleep on non-retriable)", n)
	}
}

func TestFetch_SendsUserAgent(t *testing.T) {
	_, restore := installSleepRecorder(t)
	defer restore()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	_ = Fetch(context.Background(), srv.URL, Options{})
	if !strings.HasPrefix(gotUA, "SquireBot/") {
		t.Errorf("User-Agent = %q, want it to start with %q", gotUA, "SquireBot/")
	}
	if !strings.Contains(gotUA, "github.com/boejowen/SquireBot") {
		t.Errorf("User-Agent = %q, want it to contain the contactable github URL", gotUA)
	}
}

func TestFetch_NetworkError_Retries(t *testing.T) {
	rec, restore := installSleepRecorder(t)
	defer restore()

	// Point at a server we immediately close so every Do() is a transport
	// error → the client retries all 5 schedule slots, then surfaces an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close() // nothing is listening now

	res := Fetch(context.Background(), deadURL, Options{})
	if res.OK {
		t.Fatalf("OK = true against a dead server, want false")
	}
	if res.Err == nil {
		t.Errorf("Err = nil, want a transport error")
	}
	if res.RetriesUsed != 5 {
		t.Errorf("RetriesUsed = %d, want 5 (all schedule slots retried on network error)", res.RetriesUsed)
	}
	if n := len(rec.durations()); n != 5 {
		t.Errorf("sleepFn invoked %d times, want 5 (one per retry slot)", n)
	}
}

func TestFetch_BoundedRead(t *testing.T) {
	// Lower the cap to a tiny value for this test (the hook exists so the
	// LimitReader truncation is observable without serving 16MB+). The body the
	// server sends is larger than the cap → the client reads at most `cap` bytes.
	const cap = 8
	restore := setMaxResponseBytesForTest(cap)
	defer restore()
	_, sleepRestore := installSleepRecorder(t)
	defer sleepRestore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("0123456789ABCDEF")) // 16 bytes, > cap
	}))
	defer srv.Close()

	res := Fetch(context.Background(), srv.URL, Options{})
	if !res.OK {
		t.Fatalf("OK = false, want true. Err=%v", res.Err)
	}
	if len(res.Body) != cap {
		t.Errorf("len(Body) = %d, want %d (io.LimitReader must cap the read)", len(res.Body), cap)
	}
	if string(res.Body) != "01234567" {
		t.Errorf("Body = %q, want the first %d bytes %q", res.Body, cap, "01234567")
	}
}

func TestFetch_CtxCancelDuringBackoff(t *testing.T) {
	// With the REAL ctx-aware sleep (not the no-op recorder), a cancelled ctx
	// during a backoff wait must unwind promptly (clean shutdown on SIGTERM) and
	// surface as a failed fetch rather than blocking for the full 2s schedule.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503) // always retriable → forces a backoff sleep
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel almost immediately so the first backoff sleep observes ctx.Done().
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	res := Fetch(ctx, srv.URL, Options{})
	elapsed := time.Since(start)

	if res.OK {
		t.Fatalf("OK = true, want false (ctx was cancelled mid-backoff)")
	}
	if res.Err == nil {
		t.Errorf("Err = nil, want a context-cancellation error")
	}
	// The first schedule slot is 2s; a clean ctx-aware unwind returns well
	// under that. Allow generous headroom for a slow CI box but prove we did
	// NOT wait the full 2s.
	if elapsed >= 2*time.Second {
		t.Errorf("Fetch blocked %v on a cancelled ctx; want a prompt (<2s) unwind via the ctx-aware timer", elapsed)
	}
}
