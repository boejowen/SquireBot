// Package backend is the watcher-side HTTP client that POSTs a full-snapshot
// inventory/spellbook upload to the SquireBot backend's ingest endpoint
// (POST {base}/api/v1/ingest, BACKEND-03/04, live at https://api.squirebot.quest).
//
// It is the v2.0 SINK that replaces the deleted Sheets `batchUpdate` write path
// (CONTEXT D-1/D-5). The watcher is now THINNER: it sends the RAW (CP1252-decoded-
// to-UTF-8) /outputfile text in Content and the SERVER parses it (D-3) — this
// client does NOT call parse.Parse and does NOT re-decode the content (A1; the
// disk-read side already produced UTF-8 — double-decoding mojibakes curly
// apostrophes).
//
// This package is deliberately decoupled from internal/backendsrv/* (the server):
// it mirrors the Envelope JSON shape and the SquireBot/<version> User-Agent
// convention by hand rather than importing a server package (RESEARCH anti-
// pattern: never import server internals into the watcher binary). The net/http
// shape mirrors internal/update/check.go (ctx + UA + http.Client{Timeout}).
//
// SECURITY (V7): the bearer guild code and the raw Content are NEVER logged. The
// client logs only char/kind/status/err.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Sentinel errors the caller (internal/app's rewritten upload callback, Plan 03)
// classifies the response into. The terminal vs. retryable split drives the
// retry decision (D-5):
//
//	ErrUnauthorized  (401) -> re-prompt onboarding; TERMINAL (never retry).
//	ErrCrossOwner    (409) -> log + surface; TERMINAL.
//	ErrVersionTooOld (426) -> tray "update needed", let the auto-updater handle; TERMINAL.
//	ErrBadPayload    (400/422) -> log (should not happen); TERMINAL.
//
// A 5xx response or a transport error is retryable (bounded). errRetryable is the
// internal marker the retry loop matches with errors.Is; it is NOT exported (the
// caller only cares that a retryable failure eventually surfaces as a non-nil,
// non-sentinel error after the bounded attempts are exhausted).
var (
	// ErrUnauthorized means the backend rejected the bearer guild code (bad,
	// missing, or revoked). The caller suspends uploads and re-prompts onboarding;
	// it MUST NOT retry (Pitfall 5: a 401 retry-loop hammers the backend).
	ErrUnauthorized = errors.New("backend: unauthorized (bad or revoked guild code)")
	// ErrCrossOwner means the character is already bound to a different owner
	// server-side (the v2 first-sighting bind, 11-03). Terminal; surfaced to the
	// tray/logs so the guildie can sort out the ownership conflict.
	ErrCrossOwner = errors.New("backend: character owned by another guildie")
	// ErrVersionTooOld means the watcher_version is below the backend's
	// minWatcherVersion floor (the 426 gate, Plan 13-01). Terminal; the
	// auto-updater is expected to bring the watcher current.
	ErrVersionTooOld = errors.New("backend: watcher too old for this server")
	// ErrBadPayload means the backend rejected the request body as malformed/
	// invalid (400/422). Terminal; this should not happen with a well-formed
	// envelope and is logged for diagnosis.
	ErrBadPayload = errors.New("backend: malformed or invalid payload")

	// errRetryable is the internal marker for a transient failure (5xx or a
	// transport error). The retry loop retries ONLY when errors.Is(err,
	// errRetryable); on the final attempt it surfaces the wrapped failure.
	errRetryable = errors.New("backend: retryable failure")
)

// defaultBackoff is the bounded retry schedule for transient (5xx/network)
// failures: 3 attempts with a sleep of 1s before the 2nd and 2s before the 3rd
// (the slice has one entry per attempt; the slot for attempt 0 is unused as a
// pre-sleep). The idempotent full-snapshot replace makes a re-POST safe, so a
// small bounded retry is correct; an UNBOUNDED loop is forbidden (D-5 locked
// invariant analog: "no silent retry after 401").
var defaultBackoff = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// Client POSTs ingest snapshots to a single backend base URL over the supplied
// *http.Client.
type Client struct {
	baseURL string
	http    *http.Client
	// backoff is the per-attempt retry schedule; len(backoff) is the attempt cap.
	// Production uses defaultBackoff; tests override it to near-zero durations so
	// the retry-path tests don't sleep for real.
	backoff []time.Duration
}

// New returns a Client for baseURL with a 30s-timeout default http.Client
// (mirrors update/check.go). A trailing slash on baseURL is trimmed so
// baseURL+"/api/v1/ingest" is always well-formed.
func New(baseURL string) *Client {
	return NewWithHTTPClient(baseURL, &http.Client{Timeout: 30 * time.Second})
}

// NewWithHTTPClient is the test seam: it injects an arbitrary *http.Client (the
// httptest server's client) and base URL. New delegates here with the default
// client.
func NewWithHTTPClient(baseURL string, hc *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    hc,
		backoff: defaultBackoff,
	}
}

// SetBackoffForTest overrides the per-attempt retry schedule. It is a test seam
// for callers in OTHER packages (e.g. internal/app's callback tests) that need a
// near-zero backoff so the retry path doesn't sleep [1s,2s,4s] for real. The
// length of the supplied slice becomes the attempt cap. Production code never
// calls this (it uses defaultBackoff via New/NewWithHTTPClient).
func (c *Client) SetBackoffForTest(d []time.Duration) {
	c.backoff = d
}

// Validate performs the one-shot onboarding probe: GET {base}/api/v1/whoami with
// Authorization: Bearer <code>. It returns nil on 200 (the code is valid + active),
// ErrUnauthorized on 401 (bad/unknown/revoked code → the onboarding flow re-prompts),
// and a non-nil generic error on any other status or a transport failure. Unlike
// Ingest there is NO retry — validation is a single request whose result the
// onboarding loop acts on immediately (a 500 surfaces as "couldn't reach the
// server", not a silent loop). The bearer code is NEVER logged (V7).
func (c *Client) Validate(ctx context.Context, code string) error {
	url := c.baseURL + "/api/v1/whoami"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("backend: build whoami request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+code)
	req.Header.Set("User-Agent", "SquireBot (+https://github.com/boejowen/SquireBot)")

	resp, err := c.http.Do(req)
	if err != nil {
		// Do NOT echo the code; surface a transport error the caller treats as
		// "network problem", distinct from ErrUnauthorized.
		return fmt.Errorf("backend: whoami request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK: // 200
		return nil
	case http.StatusUnauthorized: // 401
		return ErrUnauthorized
	default:
		return fmt.Errorf("backend: whoami unexpected HTTP %d", resp.StatusCode)
	}
}

// envelope is the client-side mirror of the server's ingest.Envelope (D-04). The
// json tags MUST match the server EXACTLY (character/kind/content/watcher_version)
// — this is intentionally a private copy, NOT an import of the server package
// (client/server decoupling; RESEARCH anti-pattern).
type envelope struct {
	Character      string `json:"character"`
	Kind           string `json:"kind"`
	Content        string `json:"content"`
	WatcherVersion string `json:"watcher_version"`
}

// Ingest POSTs one full-snapshot upload for (character, kind). content is the
// UTF-8 (already CP1252-decoded by the caller) raw /outputfile text — it is sent
// VERBATIM with no further decode (A1). version is the watcher build version,
// sent BOTH in the JSON envelope and in the User-Agent (the 426 gate can read
// either).
//
// It returns nil on 204; one of the terminal sentinels (ErrUnauthorized/
// ErrCrossOwner/ErrVersionTooOld/ErrBadPayload) on the corresponding status with
// NO retry; and a non-nil non-sentinel error after the bounded retry budget is
// exhausted on transient (5xx/network) failures. A surprising/unexpected status
// is treated as terminal (not retried) so it can't spin the loop.
func (c *Client) Ingest(ctx context.Context, code, character, kind, content, version string) error {
	// Build the body bytes ONCE; each attempt wraps a fresh bytes.Reader (a
	// *http.Request body is consumed per Do, so it must be re-created each try).
	body, err := json.Marshal(envelope{
		Character:      character,
		Kind:           kind,
		Content:        content,
		WatcherVersion: version,
	})
	if err != nil {
		return fmt.Errorf("backend: marshal envelope: %w", err)
	}

	url := c.baseURL + "/api/v1/ingest"
	ua := "SquireBot/" + version + " (+https://github.com/boejowen/SquireBot)"

	var lastErr error
	for attempt := 0; attempt < len(c.backoff); attempt++ {
		// Sleep BEFORE the 2nd+ attempt (ctx-aware so cancellation aborts).
		if attempt > 0 {
			if werr := ctxSleep(ctx, c.backoff[attempt]); werr != nil {
				return werr
			}
		}

		err := c.attempt(ctx, url, ua, code, body)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errRetryable) {
			// Terminal (401/409/426/4xx/unexpected): surface IMMEDIATELY, never
			// retry (D-5 locked invariant; Pitfall 5).
			return err
		}
		// Retryable (5xx/network): remember it and loop (bounded).
		lastErr = err
		slog.Warn("backend ingest transient failure; will retry",
			"char", character, "kind", kind, "attempt", attempt+1, "err", err)
	}
	return fmt.Errorf("backend: ingest failed after %d attempts: %w", len(c.backoff), lastErr)
}

// attempt performs ONE POST and classifies the result. NEVER logs code or body
// (V7). On a transport error it returns an errRetryable-wrapped error; otherwise
// it classifies the status code.
func (c *Client) attempt(ctx context.Context, url, ua, code string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		// A malformed request is not a transient condition — terminal.
		return fmt.Errorf("backend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+code)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)

	resp, err := c.http.Do(req)
	if err != nil {
		// Transport/DNS/connection error: retryable (do NOT echo code/body).
		return fmt.Errorf("%w: POST: %v", errRetryable, err)
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused (we don't need the body).
	_, _ = io.Copy(io.Discard, resp.Body)

	return classify(resp.StatusCode)
}

// classify maps a backend ingest response status to nil, a terminal sentinel, or
// errRetryable (D-5):
//
//	204            -> nil (success)
//	401            -> ErrUnauthorized
//	409            -> ErrCrossOwner
//	426            -> ErrVersionTooOld
//	400, 422       -> ErrBadPayload
//	>= 500         -> errRetryable (bounded retry)
//	anything else  -> a non-retryable generic error (a surprise status must not
//	                  spin the retry loop).
func classify(status int) error {
	switch status {
	case http.StatusNoContent: // 204
		return nil
	case http.StatusUnauthorized: // 401
		return ErrUnauthorized
	case http.StatusConflict: // 409
		return ErrCrossOwner
	case http.StatusUpgradeRequired: // 426
		return ErrVersionTooOld
	case http.StatusBadRequest, http.StatusUnprocessableEntity: // 400, 422
		return ErrBadPayload
	}
	if status >= 500 {
		return fmt.Errorf("%w: HTTP %d", errRetryable, status)
	}
	return fmt.Errorf("backend: unexpected HTTP %d", status)
}

// ctxSleep waits for d or returns ctx.Err() if the context is cancelled first,
// so an in-flight retry backoff aborts promptly on shutdown.
func ctxSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		// Still honor an already-cancelled context even with a zero delay.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
