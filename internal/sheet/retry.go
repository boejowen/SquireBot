package sheet

// Plan 02-03 Task 1 — WATCH-07 retry/backoff envelope.
//
// withRetry runs op with the WATCH-07 retry policy:
//
//   401 (Unauthorized):
//     Refresh ONCE via onRefresh; retry immediately. If refresh itself
//     fails (e.g. invalid_grant the refresh token is dead) the wrapped
//     error is surfaced — isPermanentAuthErr catches it via
//     IsRevokedRefreshToken. If the post-refresh retry is also 401,
//     return ErrPermanentAuth. This covers the case where the access
//     token is still cached but the underlying OAuth grant was revoked
//     (Google's resource server returns 401 before the access token
//     expires and a refresh is attempted by the transport layer).
//
//   429 (Too Many Requests):
//     Honor Retry-After header (overrides schedule for that retry); else
//     schedule[attempt]. Both RFC 7231 forms are accepted (integer seconds
//     or HTTP-date).
//
//   403 (Forbidden):
//     Distinguish by ge.Errors[0].Reason (Pitfall B):
//       - authError, insufficientPermissions, forbidden:
//           refresh ONCE via onRefresh; retry; if same auth-flavored 403
//           reappears, return ErrPermanentAuth (consumed by Plan 02-04 to
//           turn the tray red and prompt re-OAuth).
//       - userRateLimitExceeded, rateLimitExceeded:
//           transient quota throttle — fall through to schedule.
//
//   5xx (500, 502, 503, 504):
//     Schedule.
//
//   Non-googleapi error (network/DNS):
//     Treat as transient — schedule.
//
//   400, 404, anything else:
//     Surface immediately (non-retryable).
//
// Schedule exhaustion returns the last error wrapped with attempt count.
//
// Context cancellation during sleep returns ctx.Err() promptly.
//
// CONTEXT.md (locked): do NOT add `cenkalti/backoff` or any external backoff
// library. The 6-step schedule is hard-coded per WATCH-07. The
// google.golang.org/api/sheets/v4 library does internal gax retries; we
// only handle errors at the boundary AFTER gax has given up.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

// retrySchedule is the exact 6-step backoff WATCH-07 mandates: 2s, 4s, 8s,
// 16s, 32s, 60s. Total elapsed at exhaustion = 122s. Locked literal — see
// CONTEXT.md §Sheets Retry/Backoff.
var retrySchedule = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	32 * time.Second,
	60 * time.Second,
}

// ErrPermanentAuth signals that the second 403 with auth-flavored reason
// (authError, insufficientPermissions, or the defensive-default `forbidden`)
// after a refresh-token attempt also failed. The caller should NOT keep
// retrying — Plan 02-04 wires this to the tray red icon + suspend writes
// + prompt re-OAuth flow. Refresh-token death (invalid_grant) and revoked
// scope both surface here.
var ErrPermanentAuth = errors.New("permanent auth failure -- re-OAuth required")

// sleepFn is the package-level sleep function. Tests override it via
// installFakeSleep to skip real waits while still exercising the schedule
// symbolically. Default is realSleep, which preserves ctx cancellation.
var sleepFn = realSleep

// realSleep blocks for d unless ctx is cancelled first. Returns ctx.Err()
// on cancellation, nil otherwise.
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

// withRetry runs op with the WATCH-07 retry envelope. onRefresh is invoked
// at most ONCE — on a 401, or on a 403 with reason in {authError,
// insufficientPermissions, forbidden}. If onRefresh returns an error,
// withRetry surfaces that error wrapped (no further retries). If the
// post-refresh retry returns the same auth error, withRetry returns
// ErrPermanentAuth.
func withRetry(ctx context.Context, op func() error, onRefresh func() error) error {
	refreshed := false
	for attempt := 0; ; attempt++ {
		err := op()
		if err == nil {
			return nil
		}
		var ge *googleapi.Error
		if !errors.As(err, &ge) {
			// Non-googleapi error (network, DNS, transport): transient.
			if attempt >= len(retrySchedule) {
				return fmt.Errorf("non-googleapi error after %d attempts: %w", attempt+1, err)
			}
			if waitErr := sleepFn(ctx, retrySchedule[attempt]); waitErr != nil {
				return waitErr
			}
			continue
		}
		switch ge.Code {
		case http.StatusUnauthorized: // 401
			// A 401 is unambiguously an auth error. Unlike a 403 there is no
			// reason field to inspect — go straight to one refresh attempt.
			// If refresh fails (e.g. invalid_grant) the wrapped error surfaces
			// to isPermanentAuthErr via IsRevokedRefreshToken. If the second
			// attempt is also 401, return ErrPermanentAuth directly.
			if refreshed {
				return ErrPermanentAuth
			}
			refreshed = true
			if rerr := onRefresh(); rerr != nil {
				return fmt.Errorf("token refresh after 401: %w", rerr)
			}
			continue
		case http.StatusTooManyRequests: // 429
			d := retrySchedule[min(attempt, len(retrySchedule)-1)]
			if ra := parseRetryAfter(ge.Header.Get("Retry-After")); ra > 0 {
				d = ra
			}
			if attempt >= len(retrySchedule) {
				return fmt.Errorf("429 after %d attempts: %w", attempt+1, err)
			}
			if waitErr := sleepFn(ctx, d); waitErr != nil {
				return waitErr
			}
		case http.StatusForbidden: // 403
			reason := ""
			if len(ge.Errors) > 0 {
				reason = ge.Errors[0].Reason
			}
			switch reason {
			case "authError", "insufficientPermissions", "forbidden":
				// Pitfall B: distinguish auth-flavored 403 from quota 403.
				// Refresh once; if the second attempt is also auth-flavored,
				// surface as permanent (consumed by Plan 02-04 wiring).
				if refreshed {
					return ErrPermanentAuth
				}
				refreshed = true
				if rerr := onRefresh(); rerr != nil {
					return fmt.Errorf("token refresh after 403 %q: %w", reason, rerr)
				}
				// Retry immediately — do NOT consume a backoff slot.
				continue
			default:
				// userRateLimitExceeded, rateLimitExceeded, empty: transient.
				if attempt >= len(retrySchedule) {
					return fmt.Errorf("403 (%s) after %d attempts: %w", reason, attempt+1, err)
				}
				if waitErr := sleepFn(ctx, retrySchedule[attempt]); waitErr != nil {
					return waitErr
				}
			}
		case http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 5xx
			if attempt >= len(retrySchedule) {
				return fmt.Errorf("5xx after %d attempts: %w", attempt+1, err)
			}
			if waitErr := sleepFn(ctx, retrySchedule[attempt]); waitErr != nil {
				return waitErr
			}
		default:
			// 400, 404, 405, 410, etc.: not transient, surface immediately.
			return err
		}
	}
}

// parseRetryAfter accepts both RFC 7231 forms of the Retry-After header:
// integer seconds (e.g. "30") or HTTP-date (e.g. "Wed, 21 Oct 2026 07:28:00 GMT").
// Returns 0 on parse failure or for past dates (caller falls back to the
// schedule).
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil {
		if n <= 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
