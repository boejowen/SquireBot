package auth

// OAuth 2.0 loopback PKCE flow — Plan 01-03.
//
// SECURITY / LOGGING POLICY (T-01-01, T-03-03):
//
//   slog calls in this file may carry the following fields:
//     - email (the canonical Google email)
//     - port (the loopback listener port)
//     - state[:8]+"..." (truncated CSRF state, for correlation only)
//     - err (errors from the oauth2 library or store; never contain token bytes)
//
//   slog calls MUST NEVER carry:
//     - tok.RefreshToken / tok.AccessToken / tok (the full Token struct)
//     - the OAuth `code` query parameter
//     - the PKCE `code_verifier` (m.codeVerifier)
//     - the OAuth `client_secret` (we don't have one — desktop client uses PKCE)
//
//   Refresh-token bytes leave this file in exactly two ways:
//     1. They are passed to store.StoreToken which writes them to wincred.
//     2. They are passed to oauth2.ReuseTokenSource which holds them in
//        memory only for the lifetime of the TokenSource.
//   After StoreToken returns, the local *oauth2.Token's RefreshToken
//   and AccessToken fields are deliberately zeroed via a defer'd closure
//   so a subsequent panic / log.Printf("%+v", tok) cannot leak them.
//
// LISTENER LIFECYCLE:
//
//   NewManager allocates its own net.Listener on 127.0.0.1:0 and an
//   *http.Server that wraps it. After RunOAuth returns successfully the
//   listener and server are STILL ALIVE — Plan 06's Drive Picker reuses
//   them by attaching a /picker route to the same mux. The caller
//   (Plan 07's wizard) is responsible for eventually calling
//   result.Server.Shutdown() once the wizard finishes.
//
//   NewManagerWithListener accepts a caller-owned listener + mux. The
//   Manager registers /oauth/callback + /start_paste on the mux and
//   never calls ListenAndServe — the caller drives traffic. This is
//   the shared-listener pattern Plan 07 uses to compose OAuth + Picker
//   + wizard pages on a single port (so the user only sees one browser
//   tab open).

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/jbowen-mn/squirebot/internal/config"
)

// ManualPasteTimeout is the AUTH-01 deadline after which the wizard
// surfaces its manual-paste textarea: if the user closed the browser
// or the redirect was eaten by an extension, they have this long to
// paste the pre-redirect URL into the wizard's fallback form.
const ManualPasteTimeout = 60 * time.Second

// scopeSet is the canonical sensitive-exempt scope trio for Phase 1.
// Per RESEARCH.md §4.2 this combination requires NO Google verification
// audit. Adding `drive` (without .file), `spreadsheets`, or any other
// sensitive scope here is a T-03-07 privilege-escalation regression
// and would force Phase 2 through Google's review queue.
var scopeSet = []string{
	"https://www.googleapis.com/auth/drive.file",
	"openid",
	"https://www.googleapis.com/auth/userinfo.email",
}

// Manager owns one in-flight OAuth flow. Single use; do NOT reuse a
// Manager after RunOAuth returns or after DoneChan fires — the listener
// and TokenSource are handed off to downstream plans by then.
type Manager struct {
	cfg                  *oauth2.Config
	codeVerifier         string
	codeChallenge        string
	expectedState        string
	listener             net.Listener
	server               *http.Server // nil in NewManagerWithListener mode (caller owns the server)
	ownsServer           bool
	config               *config.Config // updated with GoogleEmail on success
	bc                   BuildConstants
	redirectAfterCallback string
	done                 chan OAuthResult
	doneOnce             sync.Once
	pasteTimer           *time.Timer
	startedAt            time.Time
}

// OAuthResult is what flows down DoneChan when the flow finishes
// (success OR terminal error). On success Listener + Server are live
// and ready for Plan 06 to attach /picker.
type OAuthResult struct {
	Email        string
	RefreshToken string             // in-memory only; for handing to TokenSource creators
	TokenSource  oauth2.TokenSource // ready-to-use; uses ReuseTokenSource for refresh hygiene
	Listener     net.Listener       // hand-off to picker.Server (Plan 06)
	Server       *http.Server       // ditto; nil if caller owned the server
	Port         int
	Err          error
}

// Config is the minimal config view OAuthConfigForRefresh needs (so
// callers don't have to import auth.BuildConstants for refresh-only
// paths). Plan 07's runWatcher needs only the OAuth client ID to rebuild
// a TokenSource from a wincred-stored refresh token.
type Config struct {
	OAuthClientID string
}

// NewManager constructs a Manager that owns its own listener — used by
// `squirebot.exe oauth` for standalone testing and by the simple
// fallback path. cfg is the on-disk config; the function will write
// GoogleEmail to it on success.
func NewManager(cfg *config.Config, bc BuildConstants) (*Manager, error) {
	if err := bc.Validate(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("oauth listen: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	m, err := newManagerCore(cfg, bc, listener, port)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	mux := http.NewServeMux()
	m.AttachRoutes(mux)
	// Standalone mode: a tiny /start fallback page for `squirebot.exe oauth`
	// dev testing. NewManagerWithListener mode skips this — the wizard's
	// own start.html supersedes it.
	authURL := m.AuthURL()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body>
<h1>SquireBot OAuth (standalone)</h1>
<p><a href="%s">Connect Google</a></p>
</body></html>`, authURL)
	})
	m.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	m.ownsServer = true
	return m, nil
}

// NewManagerWithListener constructs a Manager that SHARES a caller-owned
// listener. The caller (Plan 07 wizard) is responsible for calling
// http.Serve(listener, mux) on its own. The Manager only attaches
// routes; it does NOT call ListenAndServe.
func NewManagerWithListener(cfg *config.Config, bc BuildConstants, listener net.Listener) *Manager {
	port := 0
	if addr, ok := listener.Addr().(*net.TCPAddr); ok {
		port = addr.Port
	}
	m, _ := newManagerCore(cfg, bc, listener, port)
	return m
}

// newManagerCore builds the per-flow PKCE pair, state value, and
// oauth2.Config. Errors here are crypto-rand failures and are
// effectively unreachable on a healthy Windows box — but if rand.Read
// ever does fail we surface the error rather than silently issuing a
// zero state value.
func newManagerCore(cfg *config.Config, bc BuildConstants, listener net.Listener, port int) (*Manager, error) {
	verifier, challenge, err := NewPKCEPair()
	if err != nil {
		return nil, fmt.Errorf("pkce: %w", err)
	}
	state, err := newState()
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	oc := &oauth2.Config{
		ClientID:    bc.OAuthClientID,
		// ClientSecret deliberately empty — desktop client uses PKCE per RESEARCH.md §4.1
		RedirectURL: fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", port),
		Endpoint:    google.Endpoint,
		Scopes:      append([]string(nil), scopeSet...),
	}
	return &Manager{
		cfg:                   oc,
		codeVerifier:          verifier,
		codeChallenge:         challenge,
		expectedState:         state,
		listener:              listener,
		bc:                    bc,
		config:                cfg,
		redirectAfterCallback: "/picker",
		done:                  make(chan OAuthResult, 1),
		startedAt:             time.Now(),
	}, nil
}

// newState returns a 32-byte crypto-random base64url-NoPadding state
// value. Per T-03-01 the state MUST be at least 32 bytes; 43-char
// output here matches the PKCE verifier and is comfortably above the
// floor.
func newState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthURL returns the consent URL with all PKCE parameters. The wizard's
// start.html template links to this URL.
func (m *Manager) AuthURL() string {
	return m.cfg.AuthCodeURL(
		m.expectedState,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", m.codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// AttachRoutes registers /oauth/callback + /start_paste on a
// caller-owned mux. Called by NewManagerWithListener consumers (Plan 07
// wizard) and internally by NewManager.
func (m *Manager) AttachRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/oauth/callback", m.handleCallback)
	mux.HandleFunc("/start_paste", m.handleStartPaste)
}

// DoneChan returns the receive-only channel that fires exactly once
// when the flow completes (success or terminal error).
func (m *Manager) DoneChan() <-chan OAuthResult {
	return m.done
}

// HandlePastedRedirect parses code+state from a Google redirect URL
// the user manually pasted (AUTH-01 fallback) and runs the same
// callback handler logic. Returns an error if the URL is malformed,
// the state mismatches, or the code is missing. On success the result
// flows down DoneChan exactly as with the regular HTTP callback.
func (m *Manager) HandlePastedRedirect(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parse pasted url: %w", err)
	}
	q := u.Query()
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		return errors.New("pasted url missing code or state")
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(m.expectedState)) != 1 {
		return errors.New("pasted url: state mismatch")
	}
	if errMsg := q.Get("error"); errMsg != "" {
		return fmt.Errorf("oauth error in pasted url: %s", errMsg)
	}
	return m.exchangeAndStore(ctx, code)
}

// OAuthConfigForRefresh returns an *oauth2.Config suitable for
// refresh-only flows: no listener, no PKCE, just refresh_token →
// access_token. Plan 07's runWatcher uses this to rebuild a
// TokenSource from a wincred-stored refresh token without re-running
// the consent flow. Scope set MUST match the consent-time scope set
// so refresh succeeds.
func OAuthConfigForRefresh(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID: cfg.OAuthClientID,
		Endpoint: google.Endpoint,
		Scopes:   append([]string(nil), scopeSet...),
	}
}

// RunOAuth opens the browser, starts the loopback HTTP server, and
// blocks until the user completes (or cancels) consent OR ctx is
// cancelled OR the manual-paste timer fires AND the user later POSTs
// via /start_paste. Used only with NewManager (the Manager owns the
// server). Plan 07's wizard uses NewManagerWithListener +
// AttachRoutes + DoneChan instead.
//
// On success the returned Listener and Server are STILL ALIVE — Plan 06
// attaches /picker to the same mux. Caller MUST eventually call
// result.Server.Shutdown().
func (m *Manager) RunOAuth(ctx context.Context) OAuthResult {
	if !m.ownsServer {
		return OAuthResult{Err: errors.New("RunOAuth requires NewManager (owns server); use DoneChan with NewManagerWithListener")}
	}

	port := m.listener.Addr().(*net.TCPAddr).Port
	slog.Info("oauth started",
		"port", port,
		"state_prefix", safePrefix(m.expectedState),
	)

	// Manual-paste fallback timer (AUTH-01).
	m.pasteTimer = time.AfterFunc(ManualPasteTimeout, func() {
		slog.Warn("oauth manual-paste fallback armed",
			"elapsed", time.Since(m.startedAt).String(),
		)
	})

	// Serve in a goroutine so we can also watch ctx.
	go func() {
		if err := m.server.Serve(m.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.signalDone(OAuthResult{Err: fmt.Errorf("oauth serve: %w", err)})
		}
	}()

	authURL := m.AuthURL()
	if err := OpenBrowser(authURL); err != nil {
		// Do NOT fail the flow — the wizard's manual-paste textarea is the
		// documented backup. Surface the URL via err so Plan 07 can show it.
		slog.Warn("could not auto-open browser; user must navigate manually",
			"err", err,
		)
		// Do NOT log authURL — it contains the state and code_challenge.
		// Plan 07 surfaces it via the AuthURL() method instead.
	}

	select {
	case res := <-m.done:
		return res
	case <-ctx.Done():
		// Caller cancelled (e.g., wizard closed mid-flow). Stop the timer.
		if m.pasteTimer != nil {
			m.pasteTimer.Stop()
		}
		// Close the server so the goroutine can exit; Plan 06 hand-off is
		// cancelled in this branch by definition.
		_ = m.server.Close()
		return OAuthResult{Err: ctx.Err()}
	}
}

// handleCallback is the Google OAuth redirect target. Validates state,
// exchanges code for token, stores refresh token in wincred, looks up
// canonical email, persists to config, and signals DoneChan with
// OAuthResult ready for Plan 06.
func (m *Manager) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// CSRF check (T-03-01). Constant-time compare to avoid timing oracles.
	state := q.Get("state")
	if subtle.ConstantTimeCompare([]byte(state), []byte(m.expectedState)) != 1 {
		http.Error(w, "CSRF: state mismatch", http.StatusBadRequest)
		slog.Error("oauth state mismatch",
			"got_prefix", safePrefix(state),
			"want_prefix", safePrefix(m.expectedState),
		)
		return
	}

	if errParam := q.Get("error"); errParam != "" {
		http.Error(w, fmt.Sprintf("OAuth error: %s", errParam), http.StatusBadRequest)
		slog.Error("oauth callback error", "err", errParam)
		m.signalDone(OAuthResult{Err: fmt.Errorf("oauth: %s", errParam)})
		return
	}

	code := q.Get("code")
	if code == "" {
		http.Error(w, "No code in callback", http.StatusBadRequest)
		slog.Error("oauth callback missing code")
		return
	}

	if err := m.exchangeAndStore(r.Context(), code); err != nil {
		http.Error(w, fmt.Sprintf("OAuth exchange failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect to /picker (Plan 06 owns the route). In Plan 03 standalone
	// the route is unregistered → browser shows 404, which is fine for
	// dev smoke testing. In Plan 07 wizard mode the /picker route is
	// registered before this redirect fires.
	http.Redirect(w, r, m.redirectAfterCallback, http.StatusFound)
}

// handleStartPaste handles the AUTH-01 fallback POST: form field
// `redirect_url` carries the URL the user copied out of the browser
// before/after closing the consent tab. Same body as the normal
// callback handler.
func (m *Manager) handleStartPaste(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	raw := r.FormValue("redirect_url")
	if raw == "" {
		http.Error(w, "redirect_url is required", http.StatusBadRequest)
		return
	}
	if err := m.HandlePastedRedirect(r.Context(), raw); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, m.redirectAfterCallback, http.StatusFound)
}

// exchangeAndStore is the body shared by handleCallback and
// HandlePastedRedirect: exchange the auth code for a token (using the
// PKCE verifier as proof of possession), look up the canonical email,
// store the refresh token in wincred, persist the email to config,
// and signal DoneChan with the live TokenSource.
func (m *Manager) exchangeAndStore(ctx context.Context, code string) (retErr error) {
	tok, err := m.cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", m.codeVerifier),
	)
	if err != nil {
		slog.Error("oauth exchange failed", "err", err)
		m.signalDone(OAuthResult{Err: fmt.Errorf("exchange: %w", err)})
		return fmt.Errorf("exchange: %w", err)
	}
	// Defer-zero the token bytes so a later panic / log.Printf cannot leak
	// them. ReuseTokenSource holds its own copy internally; this only
	// zeroes our local pointer's view.
	defer func() {
		tok.RefreshToken = ""
		tok.AccessToken = ""
	}()

	if tok.RefreshToken == "" {
		err := errors.New("oauth: no refresh_token in response (consent screen may not be in Production mode — see docs/oauth-setup.md)")
		slog.Error("oauth response missing refresh_token")
		m.signalDone(OAuthResult{Err: err})
		return err
	}

	ts := oauth2.ReuseTokenSource(tok, m.cfg.TokenSource(ctx, tok))

	email, err := GetUserEmail(ctx, ts)
	if err != nil {
		slog.Error("userinfo lookup failed", "err", err)
		m.signalDone(OAuthResult{Err: fmt.Errorf("userinfo: %w", err)})
		return fmt.Errorf("userinfo: %w", err)
	}
	slog.Info("oauth callback received", "email", email)

	if err := StoreToken(email, StoredToken{
		RefreshToken: tok.RefreshToken,
		Email:        email,
		ClientID:     m.cfg.ClientID,
	}); err != nil {
		slog.Error("wincred store failed", "email", email, "err", err)
		m.signalDone(OAuthResult{Err: fmt.Errorf("store: %w", err)})
		return fmt.Errorf("store: %w", err)
	}
	slog.Info("token stored in wincred", "email", email, "target", CredPrefix+email)

	if m.config != nil {
		m.config.GoogleEmail = email
		if err := m.config.Save(); err != nil {
			// Non-fatal — token is in wincred, future runs will recover.
			slog.Warn("config save failed; email cache out of date", "err", err)
		}
	}

	if m.pasteTimer != nil {
		m.pasteTimer.Stop()
	}

	port := 0
	if m.listener != nil {
		if addr, ok := m.listener.Addr().(*net.TCPAddr); ok {
			port = addr.Port
		}
	}
	m.signalDone(OAuthResult{
		Email:        email,
		RefreshToken: "", // do not retain after wincred store
		TokenSource:  ts,
		Listener:     m.listener,
		Server:       m.server,
		Port:         port,
	})
	return nil
}

// signalDone fires the OAuthResult exactly once. Subsequent calls are
// no-ops — the channel is buffered size 1 and protected by a sync.Once
// to handle the rare case where both handleCallback and the ctx-cancel
// path race.
func (m *Manager) signalDone(res OAuthResult) {
	m.doneOnce.Do(func() {
		m.done <- res
		close(m.done)
	})
}

// safePrefix returns the first 8 chars of s + "..." for log
// correlation. Used only on state values, never on token bytes.
func safePrefix(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "..."
}

// drainBody is a tiny helper for unit tests that need to exhaust a
// response body so the connection can be reused. Not used in
// production code; kept here for test ergonomics.
var _ = io.Discard
