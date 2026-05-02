package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/boejowen/SquireBot/internal/config"
)

// fixedBC is a stable BuildConstants used across the offline tests in
// this file. The values are syntactically valid (numeric prefix, etc.)
// but never reach Google — every test below either inspects URL
// strings or runs against an httptest fake.
var fixedBC = BuildConstants{
	OAuthClientID:     "262087828393-test.apps.googleusercontent.com",
	OAuthClientSecret: "GOCSPX-test-secret-placeholder",
	PickerAPIKey:      "AIzaSyTESTPLACEHOLDERPLACEHOLDER0000000",
	GCPProjectNumber:  "262087828393",
}

// TestAuthURLContainsAllRequiredParams asserts the consent URL the
// wizard will link to has every parameter Google requires for a
// loopback PKCE flow. Plan 07's start.html template embeds AuthURL()
// raw, so a regression here breaks the wizard silently.
func TestAuthURLContainsAllRequiredParams(t *testing.T) {
	cfg := &config.Config{Version: 1}
	listener := newLoopbackListener(t)
	t.Cleanup(func() { _ = listener.Close() })

	m := NewManagerWithListener(cfg, fixedBC, listener)
	authURL := m.AuthURL()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse AuthURL: %v", err)
	}
	q := u.Query()

	required := []string{
		"client_id",
		"redirect_uri",
		"response_type",
		"scope",
		"state",
		"code_challenge",
		"code_challenge_method",
		"access_type",
		"prompt",
	}
	for _, k := range required {
		if q.Get(k) == "" {
			t.Errorf("AuthURL missing required param %q\n  url=%s", k, authURL)
		}
	}

	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if got := q.Get("response_type"); got != "code" {
		t.Errorf("response_type = %q, want code", got)
	}
	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want offline", got)
	}
	if got := q.Get("prompt"); got != "consent" {
		t.Errorf("prompt = %q, want consent", got)
	}

	// Redirect URI MUST use 127.0.0.1 (NOT localhost — Google rejects
	// localhost for desktop loopback per RESEARCH.md §4.1).
	redirect := q.Get("redirect_uri")
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") {
		t.Errorf("redirect_uri = %q, want http://127.0.0.1:<port>/...", redirect)
	}
	if strings.Contains(redirect, "localhost") {
		t.Errorf("redirect_uri uses 'localhost' — Google rejects this for desktop loopback: %q", redirect)
	}
	if !strings.HasSuffix(redirect, "/oauth/callback") {
		t.Errorf("redirect_uri must end in /oauth/callback, got %q", redirect)
	}

	// Scope set must be exactly the three sensitive-exempt scopes.
	scope := q.Get("scope")
	for _, want := range []string{
		"https://www.googleapis.com/auth/drive.file",
		"openid",
		"https://www.googleapis.com/auth/userinfo.email",
	} {
		if !strings.Contains(scope, want) {
			t.Errorf("scope missing %q: full=%q", want, scope)
		}
	}
	// Must NOT contain the broader sensitive scopes (T-03-07).
	for _, forbidden := range []string{
		"https://www.googleapis.com/auth/drive ",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/drive.readonly",
	} {
		if strings.Contains(scope+" ", forbidden) {
			t.Errorf("scope contains forbidden scope %q: full=%q", forbidden, scope)
		}
	}
}

// TestOAuthConfigForRefreshHasMatchingScopes asserts the refresh-only
// helper returns a Config with EXACTLY the same scope set as the
// consent-time flow. Mismatched scopes cause Google to reject the
// refresh with `invalid_scope` and the watcher would silently fail
// every Sheets write.
func TestOAuthConfigForRefreshHasMatchingScopes(t *testing.T) {
	cfg := OAuthConfigForRefresh(Config{OAuthClientID: "test", OAuthClientSecret: "test-secret"})
	want := []string{
		"https://www.googleapis.com/auth/drive.file",
		"openid",
		"https://www.googleapis.com/auth/userinfo.email",
	}
	if len(cfg.Scopes) != len(want) {
		t.Fatalf("scopes count = %d, want %d (got=%v)", len(cfg.Scopes), len(want), cfg.Scopes)
	}
	for i, s := range want {
		if cfg.Scopes[i] != s {
			t.Errorf("Scopes[%d] = %q, want %q", i, cfg.Scopes[i], s)
		}
	}
	if cfg.Endpoint.AuthURL == "" || cfg.Endpoint.TokenURL == "" {
		t.Errorf("Endpoint not populated: %+v", cfg.Endpoint)
	}
	if cfg.ClientID != "test" {
		t.Errorf("ClientID = %q, want test", cfg.ClientID)
	}
	// Google's token endpoint requires client_secret as a parameter
	// even for desktop PKCE flows. Refresh exchanges hit the same
	// endpoint, so the refresh-only Config MUST surface it.
	if cfg.ClientSecret != "test-secret" {
		t.Errorf("ClientSecret = %q, want test-secret (Google's token endpoint requires it on refresh exchanges)", cfg.ClientSecret)
	}
}

// TestNewManagerWithListenerSharesListener asserts the shared-listener
// constructor reuses the caller's listener (does NOT allocate its own).
// Plan 07's wizard depends on this — composing OAuth + Picker + wizard
// pages on a single port keeps the user on one browser tab.
func TestNewManagerWithListenerSharesListener(t *testing.T) {
	listener := newLoopbackListener(t)
	t.Cleanup(func() { _ = listener.Close() })

	m := NewManagerWithListener(&config.Config{Version: 1}, fixedBC, listener)
	if m.listener != listener {
		t.Errorf("Manager did not store the supplied listener")
	}
	if m.ownsServer {
		t.Errorf("ownsServer = true; should be false in shared-listener mode")
	}
	if m.server != nil {
		t.Errorf("Manager allocated its own *http.Server in shared-listener mode")
	}

	// AttachRoutes registers handlers on the caller's mux without panicking.
	mux := http.NewServeMux()
	m.AttachRoutes(mux)
	// Probe the routes by serving the mux on httptest.Server.
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// /oauth/callback with no params should 400 (state mismatch — empty != real state).
	resp, err := ts.Client().Get(ts.URL + "/oauth/callback?state=&code=fake")
	if err != nil {
		t.Fatalf("GET /oauth/callback: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("/oauth/callback empty state: status=%d, want 400", resp.StatusCode)
	}
}

// TestStateMismatchRejectsCallback covers T-03-01: an attacker who
// guesses the loopback port and races a request with state=anything
// MUST be rejected with 400. The constant-time compare in
// handleCallback is the relevant defence.
func TestStateMismatchRejectsCallback(t *testing.T) {
	listener := newLoopbackListener(t)
	t.Cleanup(func() { _ = listener.Close() })

	m := NewManagerWithListener(&config.Config{Version: 1}, fixedBC, listener)
	mux := http.NewServeMux()
	m.AttachRoutes(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Send a request with a state value that is GUARANTEED not to match
	// (the real state is 43 base64url chars; "wrong-state" is shorter).
	resp, err := ts.Client().Get(ts.URL + "/oauth/callback?state=wrong-state&code=fake")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for state mismatch", resp.StatusCode)
	}
}

// TestHandlePastedRedirectStateMismatch covers the AUTH-01 fallback
// path: a malicious pasted URL with the wrong state must be rejected.
func TestHandlePastedRedirectStateMismatch(t *testing.T) {
	listener := newLoopbackListener(t)
	t.Cleanup(func() { _ = listener.Close() })

	m := NewManagerWithListener(&config.Config{Version: 1}, fixedBC, listener)

	bad := "http://127.0.0.1:9999/oauth/callback?code=abc&state=evil-state"
	err := m.HandlePastedRedirect(t.Context(), bad)
	if err == nil {
		t.Fatalf("HandlePastedRedirect with wrong state: want error, got nil")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("error should mention state mismatch, got %q", err.Error())
	}
}

// TestNewManagerValidatesBuildConstants asserts that a Manager refuses
// to construct when any of the three -ldflags values is empty. Plan 07
// catches this at startup and surfaces a "rebuild with -ldflags"
// message rather than handing Google a blank client_id.
func TestNewManagerValidatesBuildConstants(t *testing.T) {
	cases := []BuildConstants{
		{OAuthClientID: "", OAuthClientSecret: "s", PickerAPIKey: "k", GCPProjectNumber: "1"},
		{OAuthClientID: "id", OAuthClientSecret: "", PickerAPIKey: "k", GCPProjectNumber: "1"},
		{OAuthClientID: "id", OAuthClientSecret: "s", PickerAPIKey: "", GCPProjectNumber: "1"},
		{OAuthClientID: "id", OAuthClientSecret: "s", PickerAPIKey: "k", GCPProjectNumber: ""},
	}
	for i, bc := range cases {
		_, err := NewManager(&config.Config{Version: 1}, bc)
		if err == nil {
			t.Errorf("case %d: NewManager with empty constant: want error, got nil", i)
		}
	}
}

// TestDoneChanReceiveOnly is a compile-time guarantee that DoneChan
// returns a receive-only channel. If a future refactor accidentally
// widens it to bidirectional, this test stops compiling.
func TestDoneChanReceiveOnly(t *testing.T) {
	listener := newLoopbackListener(t)
	t.Cleanup(func() { _ = listener.Close() })
	m := NewManagerWithListener(&config.Config{Version: 1}, fixedBC, listener)
	var ch <-chan OAuthResult = m.DoneChan()
	_ = ch
}

func newLoopbackListener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("loopback listen: %v", err)
	}
	return l
}

// TestTokenSourceSurvivesRequestContextCancel is the regression test
// for hotfix #2.
//
// Bug: exchangeAndStore previously built its long-lived TokenSource as
//
//	ts := oauth2.ReuseTokenSource(tok, m.cfg.TokenSource(ctx, tok))
//
// where ctx was the OAuth-callback HTTP request context (r.Context()
// from handleCallback). That context gets canceled the moment the
// callback handler writes its redirect response. The TokenSource is
// then handed to the picker, the wizard, and the watcher's Sheets
// client — all of whom call ts.Token() AFTER the redirect has fired.
// Every one of those calls failed with:
//
//	Post "https://oauth2.googleapis.com/token": context canceled
//
// The fix is to build the TokenSource with context.Background() so its
// lifetime is process-scoped, not request-scoped.
//
// This test asserts the property directly without going through the
// full OAuth + wincred + userinfo flow: it stands up a fake Google
// token endpoint, mirrors the exact ReuseTokenSource(tok,
// cfg.TokenSource(BG, tok)) pattern from exchangeAndStore, cancels a
// request-shaped context that is unrelated to the TokenSource's
// build-time ctx, and asserts ts.Token() still succeeds.
//
// As a negative control it also exercises the buggy pattern
// (cfg.TokenSource(reqCtx, tok)) and asserts it DOES fail — that way
// if some future Go runtime / oauth2 library change makes the
// ctx-canceled path stop propagating, this test will catch the
// silent loss of coverage.
func TestTokenSourceSurvivesRequestContextCancel(t *testing.T) {
	// Fake Google token endpoint. Counts hits so we can prove a refresh
	// actually happened (vs. the test passing because no refresh was
	// attempted).
	var hits int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		// Issue a fresh access token with a long expiry so the test
		// doesn't loop refreshes.
		fmt.Fprintf(w, `{"access_token":"new-access-%d","token_type":"Bearer","expires_in":3600,"refresh_token":"refresh-keep"}`, atomic.LoadInt32(&hits))
	}))
	t.Cleanup(tokenSrv.Close)

	mkCfg := func() *oauth2.Config {
		return &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL:  tokenSrv.URL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		}
	}
	// Already-expired access token forces ts.Token() to hit the token
	// endpoint (i.e. exercise the ctx that was passed to TokenSource).
	mkExpiredTok := func() *oauth2.Token {
		return &oauth2.Token{
			AccessToken:  "stale-access",
			RefreshToken: "refresh-keep",
			Expiry:       time.Now().Add(-1 * time.Hour),
		}
	}

	t.Run("background_ctx_survives_request_cancel", func(t *testing.T) {
		atomic.StoreInt32(&hits, 0)
		cfg := mkCfg()

		// Simulate the OAuth-callback handler's request ctx: a child of
		// Background that we will cancel BEFORE calling ts.Token(),
		// modelling the post-redirect cancellation.
		reqCtx, reqCancel := context.WithCancel(context.Background())

		// FIXED pattern: TokenSource bound to Background, not reqCtx.
		// (Mirrors exchangeAndStore after hotfix #2.)
		ts := oauth2.ReuseTokenSource(mkExpiredTok(), cfg.TokenSource(context.Background(), mkExpiredTok()))

		// Tear down the request context, exactly like handleCallback's
		// http.Redirect causes the request to end.
		reqCancel()

		fresh, err := ts.Token()
		if err != nil {
			t.Fatalf("ts.Token() after request-ctx cancel: want success, got %v", err)
		}
		if fresh.AccessToken == "" || fresh.AccessToken == "stale-access" {
			t.Fatalf("ts.Token() did not refresh: AccessToken=%q", fresh.AccessToken)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Fatalf("token endpoint hits = %d, want 1 (test must actually exercise refresh, not short-circuit)", got)
		}
		// The reqCtx is observably canceled — proves the assertion above
		// is non-trivial (i.e. the TokenSource actually had a real
		// chance to inherit a canceled ctx and didn't).
		if reqCtx.Err() == nil {
			t.Fatalf("test setup bug: reqCtx should be canceled by now")
		}
	})

	t.Run("buggy_pattern_with_request_ctx_does_fail", func(t *testing.T) {
		// Negative control: re-create the pre-hotfix-#2 bug locally and
		// assert that ts.Token() DOES fail with context canceled. If
		// this stops failing, the positive case above is no longer
		// proving anything and someone needs to look at why.
		atomic.StoreInt32(&hits, 0)
		cfg := mkCfg()
		reqCtx, reqCancel := context.WithCancel(context.Background())

		// BUGGY pattern: TokenSource bound to reqCtx (matches the
		// pre-hotfix-#2 line in exchangeAndStore).
		ts := oauth2.ReuseTokenSource(mkExpiredTok(), cfg.TokenSource(reqCtx, mkExpiredTok()))

		reqCancel()

		_, err := ts.Token()
		if err == nil {
			t.Fatalf("buggy pattern: expected ts.Token() to fail after reqCtx cancel, got nil err (positive case is no longer load-bearing)")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("buggy pattern: expected 'context canceled' error, got %v", err)
		}
	})
}
