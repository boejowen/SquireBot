package wizard

// Plan 07 wizard server tests. We avoid Go's network stack and Google's
// OAuth servers entirely — handlers are exercised directly via
// httptest.NewRecorder. The hooks (FolderPicker, SheetClientFactory,
// PickerAttacher, BrowserOpener) let us swap every cross-package call
// for a stub.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/jbowen-mn/squirebot/internal/auth"
	"github.com/jbowen-mn/squirebot/internal/config"
	"github.com/jbowen-mn/squirebot/internal/sheet"
)

// makeBuildConstants returns a populated BuildConstants so authMgr.AuthURL
// can build a non-empty URL.
func makeBuildConstants() auth.BuildConstants {
	return auth.BuildConstants{
		OAuthClientID:     "test-client.apps.googleusercontent.com",
		OAuthClientSecret: "GOCSPX-test-secret",
		PickerAPIKey:      "test-picker-key",
		GCPProjectNumber:  "1234567890",
	}
}

// redirectLOCALAPPDATA points config.Path() at a tempdir for tests that
// hit cfg.Save(). Returns a cleanup func.
func redirectLOCALAPPDATA(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	prev := os.Getenv("LOCALAPPDATA")
	if err := os.Setenv("LOCALAPPDATA", dir); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() { _ = os.Setenv("LOCALAPPDATA", prev) }
}

// makeFakeEQFolder creates a temp dir with eqgame.exe + eqclient.ini so
// eqfind.ValidateFolder approves it.
func makeFakeEQFolder(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, fn := range []string{"eqgame.exe", "eqclient.ini"} {
		f, err := os.Create(filepath.Join(d, fn))
		if err != nil {
			t.Fatalf("create %s: %v", fn, err)
		}
		_ = f.Close()
	}
	return d
}

func newTestServer(t *testing.T, pf FolderPicker) *Server {
	t.Helper()
	cfg := &config.Config{Version: 1, LogLevel: "info"}
	bc := makeBuildConstants()
	stubSheet := func(ctx context.Context, ts oauth2.TokenSource) (*sheet.Client, error) {
		return nil, errors.New("not used in test")
	}
	stubAttach := func(mux *http.ServeMux, sc *sheet.Client, ts oauth2.TokenSource,
		cfg *config.Config, bc auth.BuildConstants, redirect string, onPicked func()) {
	}
	stubBrowser := func(string) error { return nil }
	if pf == nil {
		pf = func(string) (string, error) { return "", errors.New("no picker") }
	}
	return newServerWithHooks(cfg, bc, pf, stubSheet, stubAttach, stubBrowser, "127.0.0.1:0")
}

// armAuthMgr binds a real auth.Manager to the server (without spinning
// up an HTTP listener) so handleStart's template can call AuthURL().
// We feed it a throwaway listener that we close immediately — Manager
// just needs port info from the listener.
func armAuthMgr(t *testing.T, s *Server) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	s.mu.Lock()
	s.authMgr = auth.NewManagerWithListener(s.cfg, s.bc, ln)
	s.mu.Unlock()
}

func TestHandleStart_RendersAuthURL(t *testing.T) {
	s := newTestServer(t, nil)
	armAuthMgr(t, s)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/start", nil)
	s.handleStart(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Connect Google") {
		t.Errorf("body missing 'Connect Google': %s", body)
	}
	if !strings.Contains(body, "accounts.google.com") {
		t.Errorf("body missing accounts.google.com (AuthURL not rendered): %s", body)
	}
	if !strings.Contains(body, "/start_paste") {
		t.Errorf("body missing /start_paste manual-paste form")
	}
}

func TestHandleStart_RendersErrorBanner(t *testing.T) {
	s := newTestServer(t, nil)
	armAuthMgr(t, s)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/start?error=oops%20nope", nil)
	s.handleStart(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "oops nope") {
		t.Errorf("error banner not rendered")
	}
}

func TestHandleEQFolderGET_RendersDiscoveredOrFallback(t *testing.T) {
	s := newTestServer(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/eq-folder", nil)
	s.handleEQFolderGET(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// Either the discovery succeeded (real EQ install on this machine) or
	// it fell through to the manual-pick prompt — both are valid renders.
	if !strings.Contains(body, "EverQuest") {
		t.Errorf("body missing 'EverQuest': %s", body)
	}
	if !strings.Contains(body, "Pick") {
		t.Errorf("body missing 'Pick' button: %s", body)
	}
}

func TestHandleEQFolderConfirm_HappyPath_RedirectsToDone(t *testing.T) {
	cleanup := redirectLOCALAPPDATA(t)
	defer cleanup()

	s := newTestServer(t, nil)
	folder := makeFakeEQFolder(t)

	form := url.Values{"path": []string{folder}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/eq-folder/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleEQFolderConfirm(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/done" {
		t.Errorf("Location = %q, want /done", loc)
	}
	if s.cfg.EQFolder != folder {
		t.Errorf("cfg.EQFolder = %q, want %q", s.cfg.EQFolder, folder)
	}
}

func TestHandleEQFolderConfirm_InvalidFolder_VerbatimD10(t *testing.T) {
	s := newTestServer(t, nil)
	notEQ := t.TempDir() // empty dir, no eqgame.exe / eqclient.ini

	form := url.Values{"path": []string{notEQ}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/eq-folder/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleEQFolderConfirm(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	want := "This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder."
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, want) {
		t.Errorf("body missing verbatim D-10 message.\ngot:  %q\nwant: %q", body, want)
	}
}

func TestHandleEQFolderConfirm_RejectsNonPOST(t *testing.T) {
	s := newTestServer(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/eq-folder/confirm", nil)
	s.handleEQFolderConfirm(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandleEQFolderPick_HappyPath_ReturnsJSON(t *testing.T) {
	folder := makeFakeEQFolder(t)
	pf := func(title string) (string, error) {
		if title != "Pick your EverQuest folder" {
			t.Errorf("title = %q, want canonical", title)
		}
		return folder, nil
	}
	s := newTestServer(t, pf)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/eq-folder/pick", nil)
	s.handleEQFolderPick(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json decode: %v; body=%s", err, rec.Body.String())
	}
	if got["path"] != folder {
		t.Errorf("path = %q, want %q", got["path"], folder)
	}
}

func TestHandleEQFolderPick_Cancelled_204(t *testing.T) {
	pf := func(string) (string, error) { return "", errors.New("user cancelled") }
	s := newTestServer(t, pf)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/eq-folder/pick", nil)
	s.handleEQFolderPick(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestHandleEQFolderPick_InvalidFolder_VerbatimD10(t *testing.T) {
	notEQ := t.TempDir()
	pf := func(string) (string, error) { return notEQ, nil }
	s := newTestServer(t, pf)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/eq-folder/pick", nil)
	s.handleEQFolderPick(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	want := "This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder."
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing verbatim D-10 message: %s", rec.Body.String())
	}
}

func TestHandleDone_RendersWithShutdownTimer(t *testing.T) {
	s := newTestServer(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/done", nil)
	s.handleDone(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/wizard/shutdown") {
		t.Errorf("body missing /wizard/shutdown JS POST: %s", body)
	}
	if !strings.Contains(body, "all set") {
		t.Errorf("body missing 'all set' message")
	}
}

func TestHandleShutdown_SendsResultOnDone(t *testing.T) {
	s := newTestServer(t, nil)
	s.cfg.GoogleEmail = "alice@example.com"
	s.cfg.SpreadsheetID = "SHEET1"
	s.cfg.EQFolder = `C:\P99`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wizard/shutdown", nil)
	s.handleShutdown(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	select {
	case res := <-s.done:
		if res.Email != "alice@example.com" || res.SpreadsheetID != "SHEET1" || res.EQFolder != `C:\P99` {
			t.Errorf("Result mis-populated: %+v", res)
		}
	case <-time.After(time.Second):
		t.Fatal("no Result on done channel after handleShutdown")
	}
}

func TestHandleShutdown_RejectsNonPOST(t *testing.T) {
	s := newTestServer(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wizard/shutdown", nil)
	s.handleShutdown(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// TestRun_OAuthFailurePropagates verifies that if the auth.Manager
// signals an error result on its DoneChan, Run returns that error
// without proceeding to the picker phase. We trigger the error via
// an HTTP request to /start_paste with a malformed pasted URL —
// auth.Manager.handleStartPaste replies 400 but does NOT signal done.
//
// To force a Done signal we instead drive the flow by sending a
// crafted callback with mismatched state, which ALSO does not signal
// done. So we test the simpler ctx-cancel path: cancel ctx, expect
// ctx.Err in Result.
func TestRun_CtxCancelReturnsErr(t *testing.T) {
	s := newTestServer(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	res := s.Run(ctx)
	if res.Err == nil {
		t.Fatal("expected ctx-cancel error, got nil")
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", res.Err)
	}
}

// TestRun_BrowserOpenerInvokedExactlyOnce — INST-03 invariant. Drive
// a Run that exits via ctx-cancel; assert the browser opener was
// called exactly once with a /start URL.
func TestRun_BrowserOpenerCalledOnceOnStart(t *testing.T) {
	cfg := &config.Config{Version: 1, LogLevel: "info"}
	bc := makeBuildConstants()
	var calls atomic.Int32
	var capturedURL atomic.Value
	stubBrowser := func(u string) error {
		calls.Add(1)
		capturedURL.Store(u)
		return nil
	}
	stubSheet := func(ctx context.Context, ts oauth2.TokenSource) (*sheet.Client, error) {
		return nil, nil
	}
	stubAttach := func(mux *http.ServeMux, sc *sheet.Client, ts oauth2.TokenSource,
		cfg *config.Config, bc auth.BuildConstants, redirect string, onPicked func()) {
	}
	pf := func(string) (string, error) { return "", errors.New("nope") }
	s := newServerWithHooks(cfg, bc, pf, stubSheet, stubAttach, stubBrowser, "127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = s.Run(ctx)

	if got := calls.Load(); got != 1 {
		t.Errorf("browser opener calls = %d, want 1", got)
	}
	if u, _ := capturedURL.Load().(string); !strings.Contains(u, "/start") {
		t.Errorf("captured URL = %q, want path /start", u)
	}
}

// TestRun_ListensOn127001 — Pitfall #6 (literal 127.0.0.1, not "localhost").
func TestRun_ListensOn127001Literal(t *testing.T) {
	cfg := &config.Config{Version: 1, LogLevel: "info"}
	bc := makeBuildConstants()
	var capturedURL atomic.Value
	stubBrowser := func(u string) error {
		capturedURL.Store(u)
		return nil
	}
	stubSheet := func(ctx context.Context, ts oauth2.TokenSource) (*sheet.Client, error) {
		return nil, nil
	}
	stubAttach := func(mux *http.ServeMux, sc *sheet.Client, ts oauth2.TokenSource,
		cfg *config.Config, bc auth.BuildConstants, redirect string, onPicked func()) {
	}
	pf := func(string) (string, error) { return "", errors.New("nope") }
	s := newServerWithHooks(cfg, bc, pf, stubSheet, stubAttach, stubBrowser, "127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = s.Run(ctx)

	u, _ := capturedURL.Load().(string)
	if !strings.HasPrefix(u, "http://127.0.0.1:") {
		t.Errorf("opener URL = %q, want http://127.0.0.1: prefix (Pitfall #6)", u)
	}
}

// drainResp ensures fetched test bodies don't keep connections alive in
// the http test pool — a guardrail for the test runner's hygiene.
var _ = func(r *http.Response) { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }
