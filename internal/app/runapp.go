// Package app is the load-bearing Phase 1 orchestrator. Plans 01-06
// produce isolated packages; package app composes them into a runnable
// pipeline:
//
//	cold start → config.Load
//	  ├─ if needsWizard → wizard.Server.Run → returns email + spreadsheet + folder + TokenSource
//	  └─ else           → auth.ReadToken → OAuthConfigForRefresh → ReuseTokenSource
//	then sheet.NewClient + ValidateWorkbook
//	then watch.Run with onChange = parse.Parse → sheet.WriteInventory → sheet.UpsertCharOwner
//
// D-04 ChangeWorkbook is a separate entry-point fired from the tray's
// Change Workbook… menu item. It re-runs picker.Server on a fresh
// loopback listener with the existing wincred-backed TokenSource (no
// OAuth re-prompt) and persists the new spreadsheetID on success.
//
// File ownership: Plan 03 ships auth.ReadToken / auth.OAuthConfigForRefresh /
// auth.NewManagerWithListener; this file consumes them by import only and
// does NOT modify internal/auth/oauth.go.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"golang.org/x/oauth2"

	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/parse"
	"github.com/boejowen/SquireBot/internal/picker"
	"github.com/boejowen/SquireBot/internal/sheet"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/watch"
	"github.com/boejowen/SquireBot/internal/wizard"
)

// charNameRE extracts <Char> from "<Char>-Inventory.txt". Phase 1
// ignores anything that doesn't match (spellbook handling = Phase 2).
var charNameRE = regexp.MustCompile(`^(.+)-Inventory\.txt$`)

// changeWorkbookTimeout is how long ChangeWorkbook waits for the user
// to complete a pick before it gives up and tears down the listener.
const changeWorkbookTimeout = 5 * time.Minute

// RunApp is the background goroutine launched from main.go. Blocks
// until ctx is cancelled. Branches on config completeness:
//   - incomplete config → wizard, then watcher
//   - complete config   → re-load wincred token + watcher directly
//
// Tray Continue setup… invokes RunApp again; D-04 Change Workbook…
// invokes ChangeWorkbook (NOT RunApp).
func RunApp(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
	if err := bc.Validate(); err != nil {
		slog.Error("build constants missing", "err", err)
		t.SetStatus("Build error: missing OAuth constants")
		t.SetIconHealth(tray.HealthRed)
		return
	}

	var ts oauth2.TokenSource

	if needsWizard(cfg) {
		t.SetStatus("Setup needed")
		t.SetIconHealth(tray.HealthRed)
		t.ShowContinueSetup()

		ws := wizard.NewServer(cfg, bc)
		res := ws.Run(ctx)
		if res.Err != nil {
			slog.Error("wizard failed", "err", res.Err)
			t.SetStatus(fmt.Sprintf("Setup error: %v", res.Err))
			// Fall through; tray's Continue setup… click triggers a fresh
			// RunApp invocation (main.go wiring).
			return
		}
		t.HideContinueSetup()
		ts = res.TokenSource
		// Wizard already wrote cfg.GoogleEmail (auth.Manager) +
		// cfg.SpreadsheetID (picker.Server) + cfg.EQFolder (handleEQFolderConfirm).
	}

	// Watcher path. If we came through the wizard, ts is live; otherwise
	// (skip-wizard cold start) we rebuild it from wincred.
	if ts == nil {
		built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
		if err != nil {
			slog.Error("token rebuild from wincred failed", "err", err)
			t.SetStatus(fmt.Sprintf("Auth error: %v", err))
			t.SetIconHealth(tray.HealthRed)
			t.ShowContinueSetup()
			return
		}
		ts = built
	}

	if err := runWatcher(ctx, cfg, t, ts); err != nil {
		slog.Error("watcher exited", "err", err)
		t.SetStatus(fmt.Sprintf("Watcher error: %v", err))
		t.SetIconHealth(tray.HealthRed)
	}
}

// needsWizard reports whether any of the three config values RunApp
// needs is missing. Used by both RunApp and the tray's startup flow.
func needsWizard(cfg *config.Config) bool {
	return cfg.GoogleEmail == "" || cfg.SpreadsheetID == "" || cfg.EQFolder == ""
}

// buildTokenSourceFromWincred reconstructs an oauth2.TokenSource from
// the wincred-stored refresh token. Used on the skip-wizard cold-start
// path and by ChangeWorkbook. Plan 03's OAuthConfigForRefresh keeps the
// scope set in lockstep with consent-time so the refresh succeeds.
func buildTokenSourceFromWincred(ctx context.Context, cfg *config.Config, bc auth.BuildConstants) (oauth2.TokenSource, error) {
	if cfg.GoogleEmail == "" {
		return nil, fmt.Errorf("no GoogleEmail in config — wizard not yet run")
	}
	st, err := auth.ReadToken(cfg.GoogleEmail)
	if err != nil {
		return nil, fmt.Errorf("read wincred token for %s: %w", cfg.GoogleEmail, err)
	}
	// Prefer the build-time client ID (current Cloud project) over the
	// stored ClientID — survives a Cloud project migration without
	// invalidating the wincred entry. Plan 03's StoredToken.ClientID is
	// retained for diagnostics, not for auth.
	clientID := bc.OAuthClientID
	if clientID == "" {
		clientID = st.ClientID
	}
	oauthCfg := auth.OAuthConfigForRefresh(auth.Config{
		OAuthClientID:     clientID,
		OAuthClientSecret: bc.OAuthClientSecret, // Google requires client_secret on refresh exchanges for desktop apps
	})
	tok := &oauth2.Token{RefreshToken: st.RefreshToken}
	ts := oauth2.ReuseTokenSource(tok, oauthCfg.TokenSource(ctx, tok))
	// Defer-zero our local view of the refresh-token bytes so a later
	// panic / log.Printf cannot leak them. ReuseTokenSource already holds
	// its own copy internally.
	st.RefreshToken = ""
	tok.RefreshToken = ""
	_ = st
	return ts, nil
}

// runWatcher starts the watcher loop and dispatches parse → write →
// upsert per inventory event. Returns when ctx is cancelled or
// fsnotify errors fatally.
func runWatcher(ctx context.Context, cfg *config.Config, t *tray.Controller, ts oauth2.TokenSource) error {
	sc, err := sheet.NewClient(ctx, ts, cfg.SpreadsheetID)
	if err != nil {
		return fmt.Errorf("sheet client: %w", err)
	}
	if err := sc.ValidateWorkbook(ctx); err != nil {
		// D-03 / Critical Constraint #5: refuse to start if the workbook
		// is wrong or schema is too new. The wizard's picker step already
		// validated, but the user may have rotated workbooks since.
		return fmt.Errorf("validate workbook on startup: %w", err)
	}

	t.SetSpreadsheetID(cfg.SpreadsheetID)
	t.SetIconHealth(tray.HealthGreen)
	t.SetStatus(fmt.Sprintf("Connected as %s — watching %s", cfg.GoogleEmail, filepath.Base(cfg.EQFolder)))

	onChange := makeOnInventoryChange(ctx, sc, cfg, t)
	return watch.Run(ctx, cfg.EQFolder, onChange)
}

// makeOnInventoryChange wraps the parse → WriteInventory → UpsertCharOwner
// chain into a watch.OnChange callback. Extracted for testability.
func makeOnInventoryChange(ctx context.Context, sc *sheet.Client, cfg *config.Config, t *tray.Controller) watch.OnChange {
	return func(path string) {
		charName := extractCharName(path)
		if charName == "" {
			slog.Warn("inventory file with unexpected name; skipping",
				"path", filepath.Base(path))
			return
		}
		// Per CLAUDE.md / RESEARCH.md §8.3: re-stat + re-read fresh on
		// every event. Never trust fsnotify event payloads on Windows.
		f, err := os.Open(path)
		if err != nil {
			slog.Error("open inventory", "char", charName, "err", err)
			return
		}
		rows, perr := parse.Parse(f)
		_ = f.Close()
		if perr != nil {
			slog.Error("parse inventory", "char", charName, "err", perr)
			return
		}
		if len(rows) == 0 {
			// T-07-05 mitigation: don't WriteInventory with 0 rows (would
			// clear the tab). EQ flush mid-write or empty char → skip.
			slog.Info("inventory empty; skipping write", "char", charName)
			return
		}
		uploadedAt := time.Now().UTC().Format(time.RFC3339)
		if err := sc.WriteInventory(ctx, charName, sheet.InventoryHeader, rows, uploadedAt); err != nil {
			slog.Error("write inventory", "char", charName, "err", err)
			t.SetStatus(fmt.Sprintf("Last upload failed: %s", charName))
			return
		}
		if err := sc.UpsertCharOwner(ctx, charName, cfg.GoogleEmail); err != nil {
			// Non-fatal — inv:Char write succeeded; surface warning, continue.
			slog.Warn("upsert char_owner", "char", charName, "err", err)
		}
		slog.Info("uploaded", "char", charName, "rows", len(rows))
		t.SetStatus(fmt.Sprintf("Last upload: %s at %s", charName, time.Now().Format("15:04")))
	}
}

// extractCharName returns "<Char>" for "<Char>-Inventory.txt" or "" for
// any other basename. The regex is intentionally narrow — Phase 2 will
// add a parallel "<Char>-Spellbook.txt" branch (WATCH-02).
func extractCharName(path string) string {
	base := filepath.Base(path)
	m := charNameRE.FindStringSubmatch(base)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// ChangeWorkbook is the D-04 tray flow. Re-runs picker.Server on a
// fresh loopback listener with a refresh-only TokenSource (no OAuth
// re-prompt). On successful pick + ValidateWorkbook the picker writes
// the new spreadsheetID to config.json; this function then surfaces
// the change in tray status.
//
// Phase 1 limitation (T-07-09 / threat register): the running watcher
// keeps using the OLD spreadsheetID until the user restarts the app.
// Phase 2 polish will hot-swap. Documented as accepted.
func ChangeWorkbook(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
	slog.Info("Change Workbook flow start", "email", cfg.GoogleEmail)
	if cfg.GoogleEmail == "" {
		slog.Warn("Change Workbook with no GoogleEmail — wizard not yet run; ignoring")
		t.SetStatus("Setup needed")
		t.ShowContinueSetup()
		return
	}
	if err := bc.Validate(); err != nil {
		slog.Error("Change Workbook: build constants missing", "err", err)
		return
	}

	ts, err := buildTokenSourceFromWincred(ctx, cfg, bc)
	if err != nil {
		slog.Error("Change Workbook: token rebuild", "err", err)
		t.SetStatus("Change Workbook: re-auth required")
		return
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("Change Workbook: listen", "err", err)
		return
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	sc, err := sheet.NewClient(ctx, ts, "")
	if err != nil {
		slog.Error("Change Workbook: sheet client", "err", err)
		return
	}

	pickerSrv := picker.NewServer(sc, ts, cfg, bc)
	pickerSrv.SetRedirectAfterPick("/changed")
	done := make(chan struct{}, 1)
	pickerSrv.OnPicked(func() {
		select {
		case done <- struct{}{}:
		default:
		}
	})
	pickerSrv.AttachRoutes(mux)

	mux.HandleFunc("/changed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:system-ui;text-align:center;margin-top:4em">
<h1 style="color:#2e7d32">&#10003; Workbook changed</h1>
<p>You can close this tab. SquireBot will pick up the new workbook on the next watcher restart.</p>
</body></html>`))
	})

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("Change Workbook: serve", "err", err)
		}
	}()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	startURL := fmt.Sprintf("http://127.0.0.1:%d/picker", port)
	if err := auth.OpenBrowser(startURL); err != nil {
		slog.Warn("Change Workbook: open browser", "err", err, "url", startURL)
	}

	select {
	case <-ctx.Done():
		return
	case <-done:
		t.SetSpreadsheetID(cfg.SpreadsheetID)
		short := cfg.SpreadsheetID
		if len(short) > 8 {
			short = short[:8] + "…"
		}
		t.SetStatus(fmt.Sprintf("Workbook changed: %s", short))
		slog.Info("Change Workbook complete", "spreadsheet_id_set", cfg.SpreadsheetID != "")
	case <-time.After(changeWorkbookTimeout):
		slog.Warn("Change Workbook: timeout waiting for pick")
		t.SetStatus("Change Workbook: cancelled")
	}
}
