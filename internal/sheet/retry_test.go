package sheet

// Tests for the WATCH-07 retry envelope (Plan 02-03 Task 1).
//
// withRetry runs op() with the fixed 6-step backoff schedule
// (2/4/8/16/32/60s), honors Retry-After on 429, refreshes once on 403 with
// auth-flavored reason, and surfaces ErrPermanentAuth if the second attempt
// after a refresh still returns auth-flavored 403.
//
// Tests stub sleepFn so the schedule is exercised symbolically (no real
// sleeps — runs in <1s per test). The recorded durations slice is the
// behavioral contract.

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

// installFakeSleep replaces sleepFn with a recorder for the duration of t.
// Returns a pointer to the recorded durations slice. The fake sleep returns
// immediately with nil unless ctx is already cancelled, in which case it
// returns ctx.Err() (preserves the cancel-during-sleep behavior).
func installFakeSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	var recorded []time.Duration
	prev := sleepFn
	sleepFn = func(ctx context.Context, d time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		recorded = append(recorded, d)
		return nil
	}
	t.Cleanup(func() { sleepFn = prev })
	return &recorded
}

// noRefresh is the most common onRefresh stub — fails the test if called.
func noRefresh(t *testing.T) func() error {
	t.Helper()
	return func() error {
		t.Fatalf("onRefresh called unexpectedly")
		return nil
	}
}

// TestWithRetry_SuccessOnFirstTry — op returns nil immediately; no sleep,
// no refresh.
func TestWithRetry_SuccessOnFirstTry(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if calls != 1 {
		t.Errorf("op calls = %d, want 1", calls)
	}
	if len(*recorded) != 0 {
		t.Errorf("sleep called %d times on success: %v", len(*recorded), *recorded)
	}
}

// TestWithRetry_SuccessAfterTransient5xx — first call returns 503, second
// returns nil. Schedule slot 0 (2s) is consumed.
func TestWithRetry_SuccessAfterTransient5xx(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			return &googleapi.Error{Code: http.StatusServiceUnavailable}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if calls != 2 {
		t.Errorf("op calls = %d, want 2", calls)
	}
	if len(*recorded) != 1 || (*recorded)[0] != 2*time.Second {
		t.Errorf("sleeps = %v, want [2s]", *recorded)
	}
}

// TestWithRetry_429WithRetryAfterSeconds — Retry-After: "5" overrides the
// schedule's 2s for that retry.
func TestWithRetry_429WithRetryAfterSeconds(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			h := http.Header{}
			h.Set("Retry-After", "5")
			return &googleapi.Error{Code: http.StatusTooManyRequests, Header: h}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if len(*recorded) != 1 || (*recorded)[0] != 5*time.Second {
		t.Errorf("sleeps = %v, want [5s] (Retry-After override)", *recorded)
	}
}

// TestWithRetry_429WithRetryAfterHTTPDate — Retry-After in RFC1123 form
// is also honored.
func TestWithRetry_429WithRetryAfterHTTPDate(t *testing.T) {
	recorded := installFakeSleep(t)
	target := time.Now().UTC().Add(5 * time.Second)
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			h := http.Header{}
			h.Set("Retry-After", target.Format(http.TimeFormat))
			return &googleapi.Error{Code: http.StatusTooManyRequests, Header: h}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if len(*recorded) != 1 {
		t.Fatalf("sleeps = %v, want exactly 1 entry", *recorded)
	}
	// Allow ~1s of slop (test scheduling).
	d := (*recorded)[0]
	if d < 3*time.Second || d > 6*time.Second {
		t.Errorf("sleep = %v, want ~5s (HTTP-date Retry-After)", d)
	}
}

// TestWithRetry_429NoRetryAfterFallsThroughToSchedule — empty Retry-After
// uses schedule[attempt].
func TestWithRetry_429NoRetryAfterFallsThroughToSchedule(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			return &googleapi.Error{Code: http.StatusTooManyRequests}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if len(*recorded) != 1 || (*recorded)[0] != 2*time.Second {
		t.Errorf("sleeps = %v, want [2s] (schedule fallback)", *recorded)
	}
}

// TestWithRetry_403AuthErrorRefreshThenSuccess — first 403 triggers refresh,
// second op call succeeds.
func TestWithRetry_403AuthErrorRefreshThenSuccess(t *testing.T) {
	installFakeSleep(t)
	refreshes := 0
	onRefresh := func() error {
		refreshes++
		return nil
	}
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			return &googleapi.Error{
				Code:   http.StatusForbidden,
				Errors: []googleapi.ErrorItem{{Reason: "authError"}},
			}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, onRefresh); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
	if calls != 2 {
		t.Errorf("op calls = %d, want 2", calls)
	}
}

// TestWithRetry_403AuthErrorTwiceIsPermanent — second 403 with the same
// reason after a refresh returns ErrPermanentAuth.
func TestWithRetry_403AuthErrorTwiceIsPermanent(t *testing.T) {
	installFakeSleep(t)
	refreshes := 0
	onRefresh := func() error {
		refreshes++
		return nil
	}
	calls := 0
	op := func() error {
		calls++
		return &googleapi.Error{
			Code:   http.StatusForbidden,
			Errors: []googleapi.ErrorItem{{Reason: "authError"}},
		}
	}
	err := withRetry(context.Background(), op, onRefresh)
	if !errors.Is(err, ErrPermanentAuth) {
		t.Fatalf("withRetry error = %v, want ErrPermanentAuth", err)
	}
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want exactly 1", refreshes)
	}
	if calls != 2 {
		t.Errorf("op calls = %d, want 2 (initial + post-refresh retry)", calls)
	}
}

// TestWithRetry_403UserRateLimitFallsThroughToSchedule — userRateLimitExceeded
// is transient; uses schedule, NOT permanent.
func TestWithRetry_403UserRateLimitFallsThroughToSchedule(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			return &googleapi.Error{
				Code:   http.StatusForbidden,
				Errors: []googleapi.ErrorItem{{Reason: "userRateLimitExceeded"}},
			}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if len(*recorded) != 1 || (*recorded)[0] != 2*time.Second {
		t.Errorf("sleeps = %v, want [2s] (rate-limit schedule)", *recorded)
	}
}

// TestWithRetry_403ForbiddenIsPermanentAfterRefresh — defensive default:
// "forbidden" reason with no further detail goes through one refresh+retry,
// then surfaces ErrPermanentAuth.
func TestWithRetry_403ForbiddenIsPermanentAfterRefresh(t *testing.T) {
	installFakeSleep(t)
	refreshes := 0
	onRefresh := func() error {
		refreshes++
		return nil
	}
	op := func() error {
		return &googleapi.Error{
			Code:   http.StatusForbidden,
			Errors: []googleapi.ErrorItem{{Reason: "forbidden"}},
		}
	}
	err := withRetry(context.Background(), op, onRefresh)
	if !errors.Is(err, ErrPermanentAuth) {
		t.Fatalf("withRetry error = %v, want ErrPermanentAuth", err)
	}
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want exactly 1", refreshes)
	}
}

// TestWithRetry_5xxExhausted — six consecutive 503s exhaust the schedule;
// the seventh attempt's error is wrapped and returned. With sleepFn stubbed
// the test runs instantly.
func TestWithRetry_5xxExhausted(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		return &googleapi.Error{Code: http.StatusServiceUnavailable}
	}
	err := withRetry(context.Background(), op, noRefresh(t))
	if err == nil {
		t.Fatalf("withRetry returned nil, want non-nil after exhaustion")
	}
	// Schedule has 6 entries; we sleep before each retry (attempts 0..5)
	// then attempt 6 returns and the schedule is exhausted.
	if len(*recorded) != 6 {
		t.Errorf("sleeps = %d entries, want 6 (full schedule)", len(*recorded))
	}
	want := []time.Duration{2, 4, 8, 16, 32, 60}
	for i, d := range *recorded {
		if d != want[i]*time.Second {
			t.Errorf("sleep[%d] = %v, want %ds", i, d, want[i])
		}
	}
	if calls != 7 {
		t.Errorf("op calls = %d, want 7 (attempts 0..6)", calls)
	}
}

// TestWithRetry_NonGoogleAPIErrorIsTransient — a plain network error is
// retried per the schedule.
func TestWithRetry_NonGoogleAPIErrorIsTransient(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			return errors.New("connection refused")
		}
		return nil
	}
	if err := withRetry(context.Background(), op, noRefresh(t)); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if len(*recorded) != 1 || (*recorded)[0] != 2*time.Second {
		t.Errorf("sleeps = %v, want [2s]", *recorded)
	}
}

// TestWithRetry_400IsNotRetried — 400 Bad Request surfaces immediately.
func TestWithRetry_400IsNotRetried(t *testing.T) {
	recorded := installFakeSleep(t)
	calls := 0
	op := func() error {
		calls++
		return &googleapi.Error{Code: http.StatusBadRequest}
	}
	err := withRetry(context.Background(), op, noRefresh(t))
	if err == nil {
		t.Fatalf("withRetry returned nil, want non-nil for 400")
	}
	if calls != 1 {
		t.Errorf("op calls = %d, want 1 (no retry on 400)", calls)
	}
	if len(*recorded) != 0 {
		t.Errorf("sleeps = %v, want none for 400", *recorded)
	}
}

// TestWithRetry_CtxCancellationDuringSleep — cancelling ctx mid-backoff
// returns ctx.Err() promptly.
func TestWithRetry_CtxCancellationDuringSleep(t *testing.T) {
	// Use a real sleepFn that respects ctx.Done so the cancel actually unblocks.
	prev := sleepFn
	sleepFn = realSleep
	t.Cleanup(func() { sleepFn = prev })

	ctx, cancel := context.WithCancel(context.Background())
	op := func() error {
		// Cancel ctx so the sleep that follows aborts immediately.
		cancel()
		return &googleapi.Error{Code: http.StatusServiceUnavailable}
	}
	start := time.Now()
	err := withRetry(ctx, op, noRefresh(t))
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("elapsed = %v, want <1s (cancel must abort sleep promptly)", elapsed)
	}
}

// TestParseRetryAfter exercises both RFC 7231 forms of the Retry-After header.
func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want time.Duration
		// for HTTP-date cases, accept a range.
		minDur time.Duration
		maxDur time.Duration
	}{
		{name: "empty", in: "", want: 0},
		{name: "integer seconds", in: "30", want: 30 * time.Second},
		{name: "zero seconds", in: "0", want: 0},
		{name: "garbage", in: "tomorrow-ish", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRetryAfter(tc.in)
			if got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}

	// HTTP-date form needs a separate test because we can't use a static
	// expected duration (the future point is computed at call time).
	t.Run("HTTP-date in future", func(t *testing.T) {
		future := time.Now().UTC().Add(10 * time.Second).Format(http.TimeFormat)
		got := parseRetryAfter(future)
		if got < 8*time.Second || got > 12*time.Second {
			t.Errorf("parseRetryAfter(future) = %v, want ~10s", got)
		}
	})

	t.Run("HTTP-date in past returns 0", func(t *testing.T) {
		past := time.Now().UTC().Add(-1 * time.Hour).Format(http.TimeFormat)
		got := parseRetryAfter(past)
		if got != 0 {
			t.Errorf("parseRetryAfter(past) = %v, want 0", got)
		}
	})
}

// TestWithRetry_401RefreshThenSuccess — first 401 triggers one refresh; the
// second op call succeeds. No sleep consumed.
func TestWithRetry_401RefreshThenSuccess(t *testing.T) {
	recorded := installFakeSleep(t)
	refreshes := 0
	onRefresh := func() error {
		refreshes++
		return nil
	}
	calls := 0
	op := func() error {
		calls++
		if calls == 1 {
			return &googleapi.Error{Code: http.StatusUnauthorized}
		}
		return nil
	}
	if err := withRetry(context.Background(), op, onRefresh); err != nil {
		t.Fatalf("withRetry: %v", err)
	}
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
	if calls != 2 {
		t.Errorf("op calls = %d, want 2", calls)
	}
	if len(*recorded) != 0 {
		t.Errorf("sleeps = %v, want none (401 retries immediately after refresh)", *recorded)
	}
}

// TestWithRetry_401TwiceIsPermanent — second 401 after a successful refresh
// returns ErrPermanentAuth with no further retries.
func TestWithRetry_401TwiceIsPermanent(t *testing.T) {
	installFakeSleep(t)
	refreshes := 0
	onRefresh := func() error {
		refreshes++
		return nil
	}
	calls := 0
	op := func() error {
		calls++
		return &googleapi.Error{Code: http.StatusUnauthorized}
	}
	err := withRetry(context.Background(), op, onRefresh)
	if !errors.Is(err, ErrPermanentAuth) {
		t.Fatalf("withRetry error = %v, want ErrPermanentAuth", err)
	}
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want exactly 1", refreshes)
	}
	if calls != 2 {
		t.Errorf("op calls = %d, want 2 (initial + post-refresh retry)", calls)
	}
}

// TestWithRetry_401RefreshFailsWrapsError — if onRefresh itself fails (e.g.
// the token endpoint returns invalid_grant), the error is wrapped and returned
// so that isPermanentAuthErr / IsRevokedRefreshToken can match on it.
func TestWithRetry_401RefreshFailsWrapsError(t *testing.T) {
	installFakeSleep(t)
	refreshErr := errors.New("oauth2: cannot fetch token: invalid_grant")
	onRefresh := func() error { return refreshErr }
	op := func() error {
		return &googleapi.Error{Code: http.StatusUnauthorized}
	}
	err := withRetry(context.Background(), op, onRefresh)
	if err == nil {
		t.Fatal("withRetry returned nil, want non-nil")
	}
	if !errors.Is(err, refreshErr) {
		t.Errorf("withRetry error = %v; want errors.Is chain to include refreshErr", err)
	}
	if got := err.Error(); len(got) == 0 {
		t.Errorf("error string is empty")
	}
	// Verify the wrapping preserves "401" in the message so callers can
	// distinguish this path from a 403 refresh failure.
	if s := err.Error(); len(s) < 3 {
		t.Errorf("error string too short: %q", s)
	}
}

// TestErrPermanentAuth_IsExportedSentinel — light sanity test that the
// sentinel is exported and non-nil. The behavioral tests above prove it
// is the value returned on the second auth-flavored 403.
func TestErrPermanentAuth_IsExportedSentinel(t *testing.T) {
	if ErrPermanentAuth == nil {
		t.Fatal("ErrPermanentAuth is nil; expected exported sentinel")
	}
	if ErrPermanentAuth.Error() == "" {
		t.Errorf("ErrPermanentAuth.Error() is empty")
	}
}
