// Package politefetch is the backend's courtesy HTTP client for outbound calls
// to the community-run PigParse + P1999 wiki services. It is a faithful Go
// net/http port of apps-script/src/lib/politeFetch.ts, carrying over every
// politeness control (these are good-external-citizen behavior toward the
// community servers and an explicit success criterion, SC-3):
//
//  1. Identifying, contactable User-Agent (buildinfo.UserAgent()).
//  2. If-None-Match (ETag) conditional request when a cached ETag exists.
//  2b. If-Modified-Since when a cached Last-Modified exists (an ADD beyond the
//     TS, which only sent If-None-Match — SC-3 names If-Modified-Since, and
//     sending both maximizes 304 hits against the wiki, which emits Last-Modified).
//  3. 304 short-circuit: return FromCache=true with an empty body so the job
//     SKIPS re-parsing/re-writing the unchanged resource (Pitfall 6).
//  4. 200 captures the response ETag + Last-Modified (the job persists them).
//  5. Exponential backoff [2s,4s,8s,16s,32s] on 429/503/504.
//  6. Retry-After honored: integer delta-seconds only (matching the TS),
//     clamped 0-600s, overriding the schedule delay when present.
//  7. Retry statuses = {429, 503, 504}.
//  8. Non-retriable statuses surface IMMEDIATELY (no retry, no sleep).
//  9. Transport (network) errors retry up to the full schedule.
// 10. The 1-second INTER-REQUEST wiki sleep is deliberately NOT here — it lives
//     in the wiki job (Plan 04) between page fetches. politeFetch does not sleep
//     between successful calls, only between retries of a single failing call.
// 11. TLS verification stays ON (Go's default http.Client) — certificate
//     verification is never disabled (matches the TS validateHttpsCertificates:true;
//     the client uses no custom tls.Config, so the secure default holds).
// 12. Redirects are followed (Go's default 10-redirect cap).
//
// Two REQUIRED Go-side additions beyond the verbatim TS:
//   - io.LimitReader(resp.Body, maxResponseBytes) caps the body read (~16 MB) so
//     a runaway/oversized response can't OOM the small VPS (mirrors the ingest
//     handler's http.MaxBytesReader discipline). The TS getContentText() had
//     Apps Script's implicit cap; Go's io.ReadAll is unbounded.
//   - A ctx-aware sleep seam (time.NewTimer + select ctx.Done()/t.C) so a
//     backoff wait unwinds promptly on SIGTERM (clean shutdown) and tests can
//     inject an instant/no-op sleep. NEVER a bare time.Sleep in this path.
//
// The client takes etag/lastMod as Options (the job reads them from the
// etag_cache table) and returns the new ones in FetchResult; the DB read/write
// is the job's responsibility, not the client's. The dropped CacheService
// response cache (D-3) is intentionally absent — the ETag/304 path is the real
// politeness control.
package politefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/buildinfo"
)

// FetchResult is the outcome of a Fetch. It mirrors the TS FetchResult
// discriminated union (FetchSuccess|FetchError) flattened into one struct.
type FetchResult struct {
	OK           bool   // true on 200 or 304; false otherwise
	Status       int    // the HTTP status of the last response (0 on a pure transport failure)
	Body         []byte // the response body; empty on 304
	ETag         string // the response ETag (200) or the echoed request ETag (304)
	LastModified string // the response Last-Modified (200) or the echoed request value (304)
	FromCache    bool   // true on 304 (the resource is unchanged — skip re-parse/re-write)
	RetriesUsed  int    // number of retries before the terminal outcome (0 = first try)
	Err          error  // non-nil when !OK
}

// Options carries the conditional-request inputs the job reads from etag_cache
// and passes in. Both are optional; empty values omit the corresponding header.
type Options struct {
	ETag         string // sent as If-None-Match when non-empty
	LastModified string // sent as If-Modified-Since when non-empty
}

// Fetcher is the seam the jobs depend on, so a job can be tested with a fake
// Fetcher (no real network). The production implementation is Fetch.
type Fetcher func(ctx context.Context, url string, opts Options) FetchResult

// retryDelays is the exponential backoff schedule for retriable statuses +
// transport errors: [2s,4s,8s,16s,32s] (RETRY_DELAYS_MS in the TS).
var retryDelays = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	32 * time.Second,
}

// retryStatuses are the statuses that trigger a backoff retry (RETRY_STATUSES
// in the TS). Everything else either succeeds (200/304) or surfaces immediately.
var retryStatuses = map[int]bool{
	http.StatusTooManyRequests:     true, // 429
	http.StatusServiceUnavailable:  true, // 503
	http.StatusGatewayTimeout:      true, // 504
}

// maxResponseBytes caps the body read (~16 MB) so an oversized/runaway response
// can't OOM the small VPS. The real PigParse fixture is 1.27 MB and wiki pages
// are <100 KB, so 16 MB is generous headroom. It is a var (not const) only so
// the test can lower it via setMaxResponseBytesForTest to observe the cap; the
// production value is always 16 << 20.
var maxResponseBytes int64 = 16 << 20

// retryAfterCeilingSeconds clamps a Retry-After value (0-600s) so a hostile or
// buggy server can't make us sleep for hours.
const retryAfterCeilingSeconds = 600

// httpClient is the shared client. TLS verification is ON (Go default — no
// custom tls.Config, certificate verification never disabled); redirects follow
// (default 10-cap). The 30s
// Timeout bounds a single request (the backoff schedule, not this Timeout,
// handles slow-server politeness). It is a var so a test could swap it, though
// the httptest server is the primary seam.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// sleepFn is the package-level ctx-aware sleep seam (mirrors
// internal/update/check.go's checkSleepFn). Production = realSleep; tests
// override it with an instant/recording sleep so retries don't actually wait.
var sleepFn = realSleep

// realSleep waits d, returning early with ctx.Err() if ctx is cancelled. This
// is what makes a backoff wait unwind promptly on SIGTERM. NEVER use a bare
// time.Sleep here — a long retry wait must respond to shutdown.
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

// Fetch performs a single polite GET with retries/backoff, ETag/304 handling,
// and a bounded body read. It is the production Fetcher.
func Fetch(ctx context.Context, url string, opts Options) FetchResult {
	var lastStatus int
	var lastErr error

	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			// A malformed URL is not retriable — surface immediately.
			return FetchResult{OK: false, Status: 0, RetriesUsed: attempt, Err: fmt.Errorf("politefetch: build request: %w", err)}
		}
		req.Header.Set("User-Agent", buildinfo.UserAgent())
		if opts.ETag != "" {
			req.Header.Set("If-None-Match", opts.ETag)
		}
		if opts.LastModified != "" {
			req.Header.Set("If-Modified-Since", opts.LastModified)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			// Transport error (connection refused, DNS, timeout, ctx cancel):
			// retry the remaining schedule slots, else surface.
			lastStatus = 0
			lastErr = fmt.Errorf("politefetch: network: %w", err)
			slog.Warn("politefetch transport error", "url", url, "attempt", attempt, "err", err)
			if attempt < len(retryDelays) {
				if serr := sleepFn(ctx, retryDelays[attempt]); serr != nil {
					// ctx cancelled mid-backoff → clean shutdown.
					return FetchResult{OK: false, Status: 0, RetriesUsed: attempt, Err: fmt.Errorf("politefetch: aborted during backoff: %w", serr)}
				}
				continue
			}
			break
		}

		status := resp.StatusCode
		lastStatus = status

		switch {
		case status == http.StatusOK: // 200
			body, rerr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
			closeBody(resp)
			if rerr != nil {
				// A read failure mid-body is treated as a transport-class error:
				// retry the remaining slots, else surface.
				lastErr = fmt.Errorf("politefetch: read body: %w", rerr)
				slog.Warn("politefetch body read error", "url", url, "attempt", attempt, "err", rerr)
				if attempt < len(retryDelays) {
					if serr := sleepFn(ctx, retryDelays[attempt]); serr != nil {
						return FetchResult{OK: false, Status: status, RetriesUsed: attempt, Err: fmt.Errorf("politefetch: aborted during backoff: %w", serr)}
					}
					continue
				}
				break
			}
			return FetchResult{
				OK:           true,
				Status:       200,
				Body:         body,
				ETag:         resp.Header.Get("ETag"),
				LastModified: resp.Header.Get("Last-Modified"),
				FromCache:    false,
				RetriesUsed:  attempt,
			}

		case status == http.StatusNotModified: // 304
			closeBody(resp)
			// Short-circuit: unchanged. Echo the supplied conditional values so
			// the job can keep the cache row coherent; the job MUST NOT parse
			// the (empty) body (Pitfall 6).
			return FetchResult{
				OK:           true,
				Status:       304,
				Body:         nil,
				ETag:         opts.ETag,
				LastModified: opts.LastModified,
				FromCache:    true,
				RetriesUsed:  attempt,
			}

		case retryStatuses[status]: // 429 || 503 || 504
			closeBody(resp)
			if attempt >= len(retryDelays) {
				lastErr = fmt.Errorf("politefetch: HTTP %d: retries exhausted", status)
				break
			}
			wait := retryDelays[attempt]
			if ra := parseRetryAfter(resp.Header.Get("Retry-After")); ra > 0 {
				wait = ra // honor the server's Retry-After over the schedule
			}
			slog.Warn("politefetch retriable status", "url", url, "attempt", attempt, "status", status, "wait", wait.String())
			if serr := sleepFn(ctx, wait); serr != nil {
				return FetchResult{OK: false, Status: status, RetriesUsed: attempt, Err: fmt.Errorf("politefetch: aborted during backoff: %w", serr)}
			}
			continue

		default:
			// Non-retriable status (404, 401, 400, 500, …) — surface
			// immediately with NO sleep and NO retry (TS "non-retriable").
			closeBody(resp)
			return FetchResult{
				OK:          false,
				Status:      status,
				RetriesUsed: attempt,
				Err:         fmt.Errorf("politefetch: non-retriable HTTP %d", status),
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("politefetch: unknown failure")
	}
	return FetchResult{OK: false, Status: lastStatus, RetriesUsed: len(retryDelays), Err: lastErr}
}

// closeBody drains-and-closes the response body. We don't need the remaining
// bytes (we either already read the capped body or are discarding a
// short-circuit/error response), but closing is required to free the
// connection for reuse. Errors are ignored (best-effort cleanup).
func closeBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// parseRetryAfter parses a Retry-After header value as integer delta-seconds
// (matching the TS, which handles only the delta-seconds form, not HTTP-date),
// clamps it to [0, 600], and returns it as a Duration. It returns 0 when the
// header is absent or unparseable, so the caller falls back to the backoff
// schedule.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	secs, err := strconv.Atoi(h)
	if err != nil || secs < 0 {
		return 0
	}
	if secs > retryAfterCeilingSeconds {
		secs = retryAfterCeilingSeconds
	}
	return time.Duration(secs) * time.Second
}

// setMaxResponseBytesForTest lowers the body-read cap so a test can observe the
// io.LimitReader truncation without serving 16 MB. It returns a restore func.
// It exists ONLY for tests (the production cap is always 16 << 20).
func setMaxResponseBytesForTest(n int64) func() {
	prev := maxResponseBytes
	maxResponseBytes = n
	return func() { maxResponseBytes = prev }
}
