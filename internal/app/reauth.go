// Plan 02-04 Task 3 — refresh-token death UX (AUTH-05).
//
// State machine:
//
//   1. Watcher steady-state: globalAuthSuspended.Load() == false.
//      makeOnInventoryChange + makeOnSpellbookChange parse + write as normal.
//
//   2. WriteInventory / WriteSpellbook returns sheet.ErrPermanentAuth
//      (Plan 02-03 boundary signal) OR auth.IsRevokedRefreshToken(err)
//      matches against the underlying *oauth2.RetrieveError shape.
//      The handler:
//        - globalAuthSuspended.Store(true)        — suspends all writes
//        - tray.SetIconHealth(HealthRed)          — visible failure
//        - tray.SetStatus("Reauthorize: ...")     — user-readable
//        - tray.ShowReauthorize()                 — tray menu surfaces click target
//        - logs slog.Error("permanent auth failure ...")
//        - returns WITHOUT UpsertCharOwner (don't refresh last_seen — the
//          watcher genuinely is not writing)
//
//   3. While suspended, the watcher loop continues observing fsnotify
//      events but the handlers' first action is `if authSuspended.Load()
//      { skip }`. No API calls are issued. CONTEXT.md (locked):
//      "no silent retry-loop".
//
//   4. User clicks tray "Reauthorize…". main.go's tray.Config.OnReauthorize
//      closure invokes app.RunReauthorize on a goroutine, which calls
//      Reauthorize. Reauthorize runs TWO phases on the same loopback server:
//
//      Phase 1 — OAuth: constructs auth.Manager, opens the browser to the
//        consent URL, waits for the callback. On success exchangeAndStore
//        replaces the wincred entry under SquireBot:<email>.
//
//      Phase 2 — Picker: drive.file scope requires the workbook to be
//        "opened" via the Drive Picker under each new OAuth grant; a plain
//        Spreadsheets.Get (or batchUpdate) against an un-opened file returns
//        401 even with a fresh valid access token. After Phase 1 stores the
//        new refresh token, Reauthorize opens a second browser window to the
//        picker page (same loopback server, new routes attached to the same
//        mux). The user re-selects the workbook; picker.Server.ValidateWorkbook
//        runs under the new token, registering the file with the new grant.
//        Only after OnPicked fires does Reauthorize clear globalAuthSuspended.
//
//   5. On failure (timeout, user closed browser, network error): logs the
//      failure, leaves globalAuthSuspended TRUE, leaves tray red, leaves
//      Reauthorize visible. The user can click again.

package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/picker"
	"github.com/boejowen/SquireBot/internal/sheet"
	"github.com/boejowen/SquireBot/internal/tray"
)

// reauthorizeTimeout is the per-phase budget for each of the two browser
// interactions in the Reauthorize flow (OAuth consent + picker). Generous:
// a guildie could be mid-pull; 5 minutes per phase lets them finish before
// clicking through the screens.
const reauthorizeTimeout = 5 * time.Minute

// globalAuthSuspended is the package-level suspension flag the watcher
// handlers consult on every event. It is set to true the first time
// WriteInventory / WriteSpellbook returns sheet.ErrPermanentAuth (or any
// error matching auth.IsRevokedRefreshToken) and cleared by a successful
// Reauthorize round-trip.
//
// Goroutine-safety: atomic.Bool. Read paths are makeOnInventoryChange /
// makeOnSpellbookChange (watcher goroutine) + RunReauthorize (tray-click
// goroutine). Write paths are the same handlers (Store(true) on permanent
// auth failure) + Reauthorize (Store(false) on success).
var globalAuthSuspended atomic.Bool

// Reauthorize re-runs the OAuth loopback flow against cfg.GoogleEmail,
// then (Phase 2) re-registers the workbook via the Drive Picker so that
// the new drive.file grant covers the workbook before writes resume.
//
// On success: replaces the wincred entry, re-registers the workbook,
// clears authSuspended, returns the tray to green.
// On failure: returns the error and leaves the suspension state alone
// (CONTEXT.md locked invariant — never silently resume after a failed
// re-auth).
//
// The caller (RunReauthorize, wired from main.go's tray.Config.OnReauthorize)
// owns the goroutine; this function blocks until both browser interactions
// complete OR a per-phase timeout fires OR ctx is cancelled.
//
// Phase 1 lesson #10 (locked): Google's /token endpoint requires
// client_secret as a parameter even on Desktop PKCE clients. The Manager
// built by auth.NewManagerWithListener uses the same OAuth config as the
// Phase 1 wizard, so client_secret is already plumbed.
func Reauthorize(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller, authSuspended *atomic.Bool) error {
	if cfg == nil || cfg.GoogleEmail == "" {
		return fmt.Errorf("Reauthorize: no GoogleEmail in config; wizard not yet run")
	}
	if err := bc.Validate(); err != nil {
		return fmt.Errorf("Reauthorize: build constants invalid: %w", err)
	}

	slog.Info("Reauthorize start", "email", cfg.GoogleEmail)
	t.SetStatus("Reauthorize: opening browser…")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("Reauthorize: listen: %w", err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	m := auth.NewManagerWithListener(cfg, bc, ln)
	m.AttachRoutes(mux)

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("Reauthorize: serve", "err", err)
		}
	}()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	authURL := m.AuthURL()
	if err := auth.OpenBrowser(authURL); err != nil {
		// Non-fatal — the user can navigate to the URL manually if needed.
		// Don't log the URL itself (contains state + code_challenge).
		slog.Warn("Reauthorize: open browser failed", "err", err)
	}

	oauthCtx, oauthCancel := context.WithTimeout(ctx, reauthorizeTimeout)
	defer oauthCancel()

	// ── Phase 1: OAuth ────────────────────────────────────────────────────
	select {
	case res := <-m.DoneChan():
		if res.Err != nil {
			slog.Error("Reauthorize: OAuth failed", "err", res.Err)
			t.SetStatus(fmt.Sprintf("Reauthorize failed: %v", res.Err))
			return res.Err
		}
		// Sanity check: the user should have re-consented as the same
		// email. If they swapped accounts mid-flow, persist the new email
		// and continue (better than dropping a successful auth).
		if res.Email != cfg.GoogleEmail {
			slog.Warn("Reauthorize: email changed",
				"expected", cfg.GoogleEmail, "got", res.Email)
			cfg.GoogleEmail = res.Email
			if err := cfg.Save(); err != nil {
				slog.Warn("Reauthorize: save cfg after email change", "err", err)
			}
		}
		// auth.Manager.exchangeAndStore already wrote the new refresh
		// token to wincred under SquireBot:<email>.

	case <-oauthCtx.Done():
		err := oauthCtx.Err()
		slog.Error("Reauthorize: OAuth timeout / cancelled", "err", err)
		t.SetStatus("Reauthorize cancelled (timeout)")
		return fmt.Errorf("Reauthorize OAuth: %w", err)
	}

	// ── Phase 2: Picker (drive.file file registration) ────────────────────
	// drive.file scope only covers files "opened" via the Drive Picker under
	// the current grant. Revoking a grant and re-authorizing creates a new
	// grant with zero registered files; any Sheets API call (even GET)
	// returns 401 until the file is re-opened via the Picker. We attach
	// picker routes to the same mux and open a new browser tab so the user
	// can re-select the workbook, registering it under the new grant.
	t.SetStatus("Reauthorize: pick your workbook in the browser to restore access…")
	slog.Info("Reauthorize: picker phase start", "email", cfg.GoogleEmail)

	freshTS, err := buildTokenSourceFromWincred(ctx, cfg, bc)
	if err != nil {
		return fmt.Errorf("Reauthorize: rebuild token for picker: %w", err)
	}
	sc, err := sheet.NewClient(ctx, freshTS, cfg.SpreadsheetID)
	if err != nil {
		return fmt.Errorf("Reauthorize: sheet client for picker: %w", err)
	}

	mux.HandleFunc("/reauth-done", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:system-ui;text-align:center;margin-top:4em">` +
			`<h1 style="color:#2e7d32">&#10003; Reauthorization complete</h1>` +
			`<p>You can close this tab. SquireBot has been reauthorized and will resume uploads.</p>` +
			`</body></html>`))
	})

	pickedCh := make(chan struct{}, 1)
	pickerSrv := picker.NewServer(sc, freshTS, cfg, bc)
	pickerSrv.SetRedirectAfterPick("/reauth-done")
	pickerSrv.OnPicked(func() {
		select {
		case pickedCh <- struct{}{}:
		default:
		}
	})
	pickerSrv.AttachRoutes(mux)

	port := ln.Addr().(*net.TCPAddr).Port
	pickerURL := fmt.Sprintf("http://127.0.0.1:%d/picker", port)
	if err := auth.OpenBrowser(pickerURL); err != nil {
		slog.Warn("Reauthorize: open picker browser", "err", err)
	}

	pickerCtx, pickerCancel := context.WithTimeout(ctx, reauthorizeTimeout)
	defer pickerCancel()

	select {
	case <-pickedCh:
		slog.Info("Reauthorize: picker step complete", "email", cfg.GoogleEmail)
	case <-pickerCtx.Done():
		err := pickerCtx.Err()
		slog.Error("Reauthorize: picker timeout / cancelled", "err", err)
		t.SetStatus("Reauthorize: picker cancelled (timeout)")
		return fmt.Errorf("Reauthorize picker: %w", err)
	}

	// Both phases complete. The new grant now covers the workbook.
	authSuspended.Store(false)
	t.SetIconHealth(tray.HealthGreen)
	t.HideReauthorize()
	t.SetStatus(fmt.Sprintf("Reauthorized as %s — resumed", cfg.GoogleEmail))
	slog.Info("Reauthorize complete", "email", cfg.GoogleEmail)
	return nil
}

// RunReauthorize is the goroutine entry point wired from
// cmd/squirebot/main.go's tray.Config.OnReauthorize closure. It runs
// Reauthorize against the package-global globalAuthSuspended flag and
// logs (but does not propagate) any error — the tray status / icon
// already surface failure to the user.
//
// Idempotent against rapid double-clicks at the level of the user's
// own behaviour: a second click while a Reauthorize is in flight will
// open a second browser window and a second listener; both will race
// to populate wincred. The tray closure in main.go could serialize via
// a sync.Mutex if this becomes a real problem; for now the cost is one
// extra redundant browser tab.
func RunReauthorize(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
	if !globalAuthSuspended.Load() {
		// User clicked Reauthorize while not currently suspended. Run
		// anyway — a successful re-auth simply replaces the wincred
		// token. Useful as a manual "rotate my refresh token" affordance.
		slog.Info("Reauthorize requested but not currently suspended; proceeding anyway")
	}
	if err := Reauthorize(ctx, cfg, bc, t, &globalAuthSuspended); err != nil {
		slog.Error("Reauthorize failed", "err", err)
	}
}

