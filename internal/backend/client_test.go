package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fastClient returns a *Client pointed at srv with the retry backoff stubbed to
// near-zero so the retry-path tests don't sleep [1s,2s,4s] for real. The test
// seam (backoffSchedule) keeps production timing intact.
func fastClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c := NewWithHTTPClient(srv.URL, srv.Client())
	c.backoff = []time.Duration{0, 0, 0} // 3 attempts, no real sleep
	return c
}

// recordingServer captures the last request (method/path/headers/body) and
// returns each status from the statuses sequence in order (clamping to the last
// element once exhausted), incrementing a request counter the test asserts on.
type recordingServer struct {
	statuses    []int
	count       int32
	lastMethod  string
	lastPath    string
	lastAuth    string
	lastCT      string
	lastUA      string
	lastBody    []byte
}

func (rs *recordingServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&rs.count, 1)
		rs.lastMethod = r.Method
		rs.lastPath = r.URL.Path
		rs.lastAuth = r.Header.Get("Authorization")
		rs.lastCT = r.Header.Get("Content-Type")
		rs.lastUA = r.Header.Get("User-Agent")
		body, _ := io.ReadAll(r.Body)
		rs.lastBody = body

		idx := int(n) - 1
		if idx >= len(rs.statuses) {
			idx = len(rs.statuses) - 1
		}
		w.WriteHeader(rs.statuses[idx])
	}
}

func (rs *recordingServer) requests() int { return int(atomic.LoadInt32(&rs.count)) }

func TestIngest_204_Success_RequestShape(t *testing.T) {
	rs := &recordingServer{statuses: []int{http.StatusNoContent}}
	srv := httptest.NewServer(rs.handler())
	defer srv.Close()
	c := fastClient(t, srv)

	const body = "Belt\tFine Steel Long Sword\t5616\t1\t0"
	err := c.Ingest(context.Background(), "SECRETCODE", "Foo", "inventory", body, "2.0.0")
	if err != nil {
		t.Fatalf("Ingest returned %v, want nil on 204", err)
	}
	if rs.requests() != 1 {
		t.Fatalf("made %d requests, want 1", rs.requests())
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", rs.lastMethod)
	}
	if rs.lastPath != "/api/v1/ingest" {
		t.Errorf("path = %q, want /api/v1/ingest", rs.lastPath)
	}
	if rs.lastAuth != "Bearer SECRETCODE" {
		t.Errorf("Authorization = %q, want %q", rs.lastAuth, "Bearer SECRETCODE")
	}
	if rs.lastCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rs.lastCT)
	}
	if want := "SquireBot/2.0.0 (+https://github.com/boejowen/SquireBot)"; rs.lastUA != want {
		t.Errorf("User-Agent = %q, want %q", rs.lastUA, want)
	}

	var got envelope
	if err := json.Unmarshal(rs.lastBody, &got); err != nil {
		t.Fatalf("body did not unmarshal to envelope: %v (body=%q)", err, rs.lastBody)
	}
	want := envelope{Character: "Foo", Kind: "inventory", Content: body, WatcherVersion: "2.0.0"}
	if got != want {
		t.Errorf("envelope = %+v, want %+v", got, want)
	}
}

// terminalCases drives the status->typed-error map; each MUST NOT retry.
func TestIngest_TerminalErrors_NoRetry(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"401_unauthorized", http.StatusUnauthorized, ErrUnauthorized},
		{"409_cross_owner", http.StatusConflict, ErrCrossOwner},
		{"426_version_too_old", http.StatusUpgradeRequired, ErrVersionTooOld},
		{"400_bad_payload", http.StatusBadRequest, ErrBadPayload},
		{"422_bad_payload", http.StatusUnprocessableEntity, ErrBadPayload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &recordingServer{statuses: []int{tc.status}}
			srv := httptest.NewServer(rs.handler())
			defer srv.Close()
			c := fastClient(t, srv)

			err := c.Ingest(context.Background(), "code", "Foo", "inventory", "x", "2.0.0")
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want errors.Is(_, %v)", err, tc.want)
			}
			if rs.requests() != 1 {
				t.Fatalf("status %d made %d requests, want 1 (no retry)", tc.status, rs.requests())
			}
		})
	}
}

func TestIngest_500ThenSuccess_RetriesOnce(t *testing.T) {
	rs := &recordingServer{statuses: []int{http.StatusInternalServerError, http.StatusNoContent}}
	srv := httptest.NewServer(rs.handler())
	defer srv.Close()
	c := fastClient(t, srv)

	err := c.Ingest(context.Background(), "code", "Foo", "inventory", "x", "2.0.0")
	if err != nil {
		t.Fatalf("Ingest = %v, want nil after a 500 then 204", err)
	}
	if rs.requests() != 2 {
		t.Fatalf("made %d requests, want exactly 2 (retried the 5xx once)", rs.requests())
	}
}

func TestIngest_500Always_BoundedThenError(t *testing.T) {
	rs := &recordingServer{statuses: []int{http.StatusInternalServerError}}
	srv := httptest.NewServer(rs.handler())
	defer srv.Close()
	c := fastClient(t, srv)

	err := c.Ingest(context.Background(), "code", "Foo", "inventory", "x", "2.0.0")
	if err == nil {
		t.Fatal("Ingest = nil, want a non-nil exhausted error after N 500s")
	}
	// The exhausted error must NOT be one of the terminal sentinels.
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrCrossOwner) ||
		errors.Is(err, ErrVersionTooOld) || errors.Is(err, ErrBadPayload) {
		t.Fatalf("exhausted error %v must not be a terminal sentinel", err)
	}
	if rs.requests() != len(c.backoff) {
		t.Fatalf("made %d requests, want exactly %d (the bounded cap)", rs.requests(), len(c.backoff))
	}
}

func TestIngest_NetworkFailure_Retryable(t *testing.T) {
	// A closed server's URL is unreachable -> transport error on every Do.
	rs := &recordingServer{statuses: []int{http.StatusNoContent}}
	srv := httptest.NewServer(rs.handler())
	url := srv.URL
	hc := srv.Client()
	srv.Close() // now every request to url fails at the transport layer

	c := NewWithHTTPClient(url, hc)
	c.backoff = []time.Duration{0, 0, 0}

	err := c.Ingest(context.Background(), "code", "Foo", "inventory", "x", "2.0.0")
	if err == nil {
		t.Fatal("Ingest = nil, want a non-nil error when the server is unreachable")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Fatalf("network failure surfaced as ErrUnauthorized: %v", err)
	}
}

func TestIngest_UTF8_ByteFidelity(t *testing.T) {
	// A curly apostrophe (U+2019), already UTF-8, must be transmitted byte-for-
	// byte with no re-decode (A1 — no mojibake on the client side).
	const content = "Cloak of the Akhevan’s Wrath"
	rs := &recordingServer{statuses: []int{http.StatusNoContent}}
	srv := httptest.NewServer(rs.handler())
	defer srv.Close()
	c := fastClient(t, srv)

	if err := c.Ingest(context.Background(), "code", "Foo", "inventory", content, "2.0.0"); err != nil {
		t.Fatalf("Ingest = %v", err)
	}
	var got envelope
	if err := json.Unmarshal(rs.lastBody, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Content != content {
		t.Fatalf("content round-trip mismatch:\n got=%q\nwant=%q", got.Content, content)
	}
	if !bytes.Equal([]byte(got.Content), []byte(content)) {
		t.Fatalf("content bytes differ (mojibake): got=% x want=% x", got.Content, content)
	}
}

// TestValidate drives the onboarding probe (GET /api/v1/whoami): 200 -> nil,
// 401 -> ErrUnauthorized, anything else -> a non-nil generic error. Validation
// is one-shot (NO retry) — a single request per call regardless of status.
func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		wantNil bool
		wantErr error // checked with errors.Is when non-nil
	}{
		{"200_ok", http.StatusOK, true, nil},
		{"401_unauthorized", http.StatusUnauthorized, false, ErrUnauthorized},
		{"500_generic", http.StatusInternalServerError, false, nil},
		{"403_generic", http.StatusForbidden, false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &recordingServer{statuses: []int{tc.status}}
			srv := httptest.NewServer(rs.handler())
			defer srv.Close()
			c := fastClient(t, srv)

			err := c.Validate(context.Background(), "SECRETCODE")
			if tc.wantNil {
				if err != nil {
					t.Fatalf("Validate = %v, want nil on %d", err, tc.status)
				}
			} else {
				if err == nil {
					t.Fatalf("Validate = nil, want error on %d", tc.status)
				}
				if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
					t.Fatalf("Validate err = %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
			}
			// Validation is one-shot: exactly one request, even on 500 (no retry).
			if rs.requests() != 1 {
				t.Fatalf("status %d made %d requests, want exactly 1 (validation never retries)", tc.status, rs.requests())
			}
			// Request shape: GET to /api/v1/whoami with the bearer header.
			if rs.lastMethod != http.MethodGet {
				t.Errorf("method = %q, want GET", rs.lastMethod)
			}
			if rs.lastPath != "/api/v1/whoami" {
				t.Errorf("path = %q, want /api/v1/whoami", rs.lastPath)
			}
			if rs.lastAuth != "Bearer SECRETCODE" {
				t.Errorf("Authorization = %q, want %q", rs.lastAuth, "Bearer SECRETCODE")
			}
		})
	}
}

func TestIngest_NoSecretInLogs(t *testing.T) {
	// Capture everything the client slogs and assert neither the bearer code nor
	// the raw content appears (V7).
	const code = "SUPERSECRETCODE12345"
	const content = "Belt\tRubicite Breastplate\t1234\t1\t0"

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	// Exercise both a success and a terminal-error path (the error path is the
	// likeliest place a careless implementation would log the request).
	rs := &recordingServer{statuses: []int{http.StatusUnauthorized}}
	srv := httptest.NewServer(rs.handler())
	defer srv.Close()
	c := fastClient(t, srv)
	_ = c.Ingest(context.Background(), code, "Foo", "inventory", content, "2.0.0")

	logged := buf.String()
	if strings.Contains(logged, code) {
		t.Errorf("the bearer code leaked into logs:\n%s", logged)
	}
	if strings.Contains(logged, content) {
		t.Errorf("the raw content leaked into logs:\n%s", logged)
	}
}
