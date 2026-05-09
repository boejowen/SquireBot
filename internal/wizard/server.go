// Package wizard hosts the four-step setup flow described in CONTEXT.md
// D-06 / D-07 on the same loopback HTTP listener Plan 03's auth.Manager
// owns. The user sees a single browser tab carry them through:
//
//	/start          (Plan 07)  — Connect Google call-to-action
//	/oauth/callback (Plan 03)  — auth.Manager.handleCallback
//	/picker         (Plan 06)  — picker.Server.handlePicker
//	/picker/result  (Plan 06)  — picker.Server.handleResult
//	/eq-folder       (Plan 07)  — auto-discovered folder + native picker
//	/eq-folder/pick  (Plan 07)  — sqweek/dialog folder dialog
//	/eq-folder/confirm (Plan 07) — D-10 validation + config.Save
//	/done            (Plan 07)  — "you're all set" + 3s graceful shutdown
//	/wizard/shutdown (Plan 07)  — POSTed by /done's setTimeout to tear down
//
// INST-03 invariant: Plan 07 calls auth.OpenBrowser EXACTLY ONCE with the
// /start URL. Every subsequent navigation is server-side
// http.Redirect or in-page <a href> on the same tab.
//
// Plan 03 ownership invariant: this file CONSUMES auth.NewManagerWithListener,
// auth.Manager.AttachRoutes, auth.Manager.AuthURL, auth.Manager.DoneChan, and
// auth.OAuthConfigForRefresh. It does NOT modify internal/auth/oauth.go.
package wizard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/eqfind"
	"github.com/boejowen/SquireBot/internal/picker"
	"github.com/boejowen/SquireBot/internal/sheet"
)

// FolderPicker abstracts the sqweek/dialog native folder dialog so tests
// can stub it. The production binding lives in folderpicker_dialog.go and
// calls dialog.Directory().Title(...).Browse().
type FolderPicker func(title string) (string, error)

// SheetClientFactory builds a sheet.Client. Hookable for tests so we don't
// have to touch the real Google Sheets endpoint.
type SheetClientFactory func(ctx context.Context, ts oauth2.TokenSource) (*sheet.Client, error)

// PickerAttacher mounts picker routes on a mux. Hookable for tests so the
// integration suite can run without picker.html's JS wiring.
type PickerAttacher func(mux *http.ServeMux, sc *sheet.Client, ts oauth2.TokenSource, cfg *config.Config, bc auth.BuildConstants, redirect string, onPicked func())

// BrowserOpener launches the user's browser at url. The production binding
// is auth.OpenBrowser. The wizard tests stub this to a no-op so the test
// process doesn't pop browser tabs.
type BrowserOpener func(url string) error

// Result is what Run returns on completion. The HTTP server has already
// been gracefully shut down by the time Run returns, so callers do NOT
// need to manage the listener.
type Result struct {
	Email         string
	SpreadsheetID string
	EQFolder      string
	TokenSource   oauth2.TokenSource // ready-to-use for sheet.Client
	Err           error
}

// Server is one wizard run. NOT reusable — once Run returns, the listener
// and HTTP server are gone. Construct a fresh Server for "Continue setup…"
// re-entry from the tray.
type Server struct {
	cfg *config.Config
	bc  auth.BuildConstants

	// hooks
	pickFolder       FolderPicker
	newSheetClient   SheetClientFactory
	attachPicker     PickerAttacher
	openBrowser      BrowserOpener
	listenAddr       string // "127.0.0.1:0" in production; tests can override

	// runtime
	startTmpl *template.Template
	eqTmpl    *template.Template
	doneTmpl  *template.Template

	mu          sync.Mutex
	authMgr     *auth.Manager
	tokenSource oauth2.TokenSource
	listener    net.Listener
	httpSrv     *http.Server
	mux         *http.ServeMux
	port        int
	done        chan Result
	doneOnce    sync.Once
}

// NewServer constructs a Server. The production binding wires the real
// sqweek/dialog, sheet.NewClient, picker.NewServer, and auth.OpenBrowser.
// Tests can use NewTestServer to swap each for a stub.
func NewServer(cfg *config.Config, bc auth.BuildConstants) *Server {
	return newServerWithHooks(cfg, bc, defaultPickFolder, defaultSheetClient, defaultAttachPicker, auth.OpenBrowser, "127.0.0.1:0")
}

// newServerWithHooks is the testable constructor.
func newServerWithHooks(cfg *config.Config, bc auth.BuildConstants,
	pf FolderPicker, sf SheetClientFactory, pa PickerAttacher, ob BrowserOpener, addr string) *Server {
	startTmpl := template.Must(template.ParseFS(pagesFS, "pages/start.html"))
	eqTmpl := template.Must(template.ParseFS(pagesFS, "pages/eq-folder.html"))
	doneTmpl := template.Must(template.ParseFS(pagesFS, "pages/done.html"))
	return &Server{
		cfg:            cfg,
		bc:             bc,
		pickFolder:     pf,
		newSheetClient: sf,
		attachPicker:   pa,
		openBrowser:    ob,
		listenAddr:     addr,
		startTmpl:      startTmpl,
		eqTmpl:         eqTmpl,
		doneTmpl:       doneTmpl,
		done:           make(chan Result, 1),
	}
}

// Port returns the bound port; useful for tests.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Run boots the wizard. Blocks until /done -> /wizard/shutdown is reached,
// ctx is cancelled, or OAuth fails. Returns a Result with the final config
// values + a live TokenSource on success.
func (s *Server) Run(ctx context.Context) Result {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return Result{Err: fmt.Errorf("wizard listen: %w", err)}
	}
	port := 0
	if a, ok := ln.Addr().(*net.TCPAddr); ok {
		port = a.Port
	}

	mux := http.NewServeMux()
	httpSrv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	authMgr := auth.NewManagerWithListener(s.cfg, s.bc, ln)
	authMgr.AttachRoutes(mux) // /oauth/callback + /start_paste

	s.mu.Lock()
	s.listener = ln
	s.httpSrv = httpSrv
	s.mux = mux
	s.authMgr = authMgr
	s.port = port
	s.mu.Unlock()

	// Wizard routes.
	mux.HandleFunc("/start", s.handleStart)
	mux.HandleFunc("/eq-folder", s.handleEQFolderGET)
	mux.HandleFunc("/eq-folder/pick", s.handleEQFolderPick)
	mux.HandleFunc("/eq-folder/confirm", s.handleEQFolderConfirm)
	mux.HandleFunc("/done", s.handleDone)
	mux.HandleFunc("/wizard/shutdown", s.handleShutdown)

	startURL := fmt.Sprintf("http://127.0.0.1:%d/start", port)
	slog.Info("wizard started", "port", port, "start_url", startURL)

	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.signalDone(Result{Err: fmt.Errorf("wizard serve: %w", err)})
		}
	}()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	// INST-03: open the browser ONCE.
	if err := s.openBrowser(startURL); err != nil {
		// Non-fatal — the manual-paste textarea on /start is the documented backup.
		slog.Warn("wizard could not auto-open browser; user must navigate manually",
			"err", err, "url", startURL)
	}

	// Phase A: wait for OAuth done.
	oauthDone := authMgr.DoneChan()
	var oauthRes auth.OAuthResult
	select {
	case <-ctx.Done():
		return Result{Err: ctx.Err()}
	case oauthRes = <-oauthDone:
		if oauthRes.Err != nil {
			return Result{Err: fmt.Errorf("oauth: %w", oauthRes.Err)}
		}
	}
	slog.Info("wizard oauth complete", "email", oauthRes.Email)

	// Build sheet client + attach picker routes. Picker's redirectAfterPick
	// is /eq-folder so the same browser tab continues the flow.
	sc, err := s.newSheetClient(ctx, oauthRes.TokenSource)
	if err != nil {
		return Result{Err: fmt.Errorf("sheet client: %w", err)}
	}
	s.mu.Lock()
	s.tokenSource = oauthRes.TokenSource
	s.mu.Unlock()
	s.attachPicker(mux, sc, oauthRes.TokenSource, s.cfg, s.bc, "/eq-folder", func() {
		slog.Info("wizard picker step complete", "spreadsheet_id_set", s.cfg.SpreadsheetID != "")
	})

	// Phase B: wait for /wizard/shutdown POST or ctx cancellation.
	select {
	case <-ctx.Done():
		return Result{Err: ctx.Err()}
	case res := <-s.done:
		// Fill in the live TokenSource so callers don't need to re-load wincred.
		if res.TokenSource == nil {
			res.TokenSource = oauthRes.TokenSource
		}
		if res.Email == "" {
			res.Email = oauthRes.Email
		}
		return res
	}
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	mgr := s.authMgr
	s.mu.Unlock()
	data := struct {
		AuthURL string
		Error   string
	}{
		Error: r.URL.Query().Get("error"),
	}
	if mgr != nil {
		data.AuthURL = mgr.AuthURL()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.startTmpl.Execute(w, data); err != nil {
		slog.Error("start template render failed", "err", err)
	}
}

func (s *Server) handleEQFolderGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := struct {
		Discovered string
		Error      string
	}{
		Error: r.URL.Query().Get("error"),
	}
	if p, err := eqfind.Discover(); err == nil {
		data.Discovered = p
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.eqTmpl.Execute(w, data); err != nil {
		slog.Error("eq-folder template render failed", "err", err)
	}
}

// handleEQFolderPick triggers the native folder picker and returns the
// selected path as JSON. On user-cancel returns 204; on validation
// failure returns the verbatim D-10 message as a 400.
func (s *Server) handleEQFolderPick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path, err := s.pickFolder("Pick your EverQuest folder")
	if err != nil {
		// User cancelled or sqweek error; surface as 204 so the page can re-prompt.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if path == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := eqfind.ValidateFolder(path); err != nil {
		// Verbatim D-10 message.
		http.Error(w,
			"This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder.",
			http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"path": path})
}

func (s *Server) handleEQFolderConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	path := r.PostForm.Get("path")
	if err := eqfind.ValidateFolder(path); err != nil {
		http.Error(w,
			"This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder.",
			http.StatusBadRequest)
		return
	}
	// Plan 02-02 (WATCH-03): write BOTH the legacy single-folder field and
	// the Phase 2 multi-folder slice so runWatcher reads either correctly
	// without depending on a fresh config.Load round-trip. Phase 5 will
	// own the multi-folder wizard UX; this one is single-folder only.
	s.cfg.EQFolder = path
	s.cfg.EQFolders = []string{path}
	if err := s.cfg.Save(); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("wizard eq-folder confirmed", "folder", path)

	// BUG-001 fix (v0.2.1): signal Run() that the wizard is complete NOW,
	// not later via /done's setTimeout-driven /wizard/shutdown POST. The
	// /done page's JS-fired shutdown is brittle: if the user closes the
	// browser tab (or navigates away, or minimises) before its 3-second
	// timer fires, the shutdown POST never arrives, Run() blocks forever
	// on <-s.done, and the watcher never starts scaffolding. Signalling
	// here makes wizard completion independent of browser behaviour: as
	// soon as config.Save() has persisted the final value, Run() can
	// return and runWatcher (in internal/app) can begin. /done remains
	// purely cosmetic; its shutdown POST still fires but is now a no-op
	// safety net (signalDone uses sync.Once, and httpSrv.Shutdown is
	// triggered by Run's defer once the channel send completes).
	s.mu.Lock()
	res := Result{
		Email:         s.cfg.GoogleEmail,
		SpreadsheetID: s.cfg.SpreadsheetID,
		EQFolder:      s.cfg.EQFolder,
		TokenSource:   s.tokenSource,
	}
	s.mu.Unlock()
	s.signalDone(res)

	http.Redirect(w, r, "/done", http.StatusFound)
}

func (s *Server) handleDone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.doneTmpl.Execute(w, nil); err != nil {
		slog.Error("done template render failed", "err", err)
	}
}

// handleShutdown is POSTed by done.html's setTimeout. Sends the Result on
// the done channel and replies 204 — Run's defer http.Server.Shutdown
// then tears down the listener.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.mu.Lock()
	res := Result{
		Email:         s.cfg.GoogleEmail,
		SpreadsheetID: s.cfg.SpreadsheetID,
		EQFolder:      s.cfg.EQFolder,
		TokenSource:   s.tokenSource,
	}
	s.mu.Unlock()
	s.signalDone(res)
}

// signalDone fires the Result exactly once.
func (s *Server) signalDone(res Result) {
	s.doneOnce.Do(func() {
		select {
		case s.done <- res:
		default:
		}
	})
}

// defaultSheetClient builds a real sheet.Client from a TokenSource.
func defaultSheetClient(ctx context.Context, ts oauth2.TokenSource) (*sheet.Client, error) {
	return sheet.NewClient(ctx, ts, "")
}

// defaultAttachPicker mounts the real picker.Server on the wizard's mux.
func defaultAttachPicker(mux *http.ServeMux, sc *sheet.Client, ts oauth2.TokenSource,
	cfg *config.Config, bc auth.BuildConstants, redirect string, onPicked func()) {
	psrv := picker.NewServer(sc, ts, cfg, bc)
	psrv.SetRedirectAfterPick(redirect)
	psrv.OnPicked(onPicked)
	psrv.AttachRoutes(mux)
}
