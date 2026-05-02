// Command squirebot is the per-guildie Windows watcher described in
// .planning/PROJECT.md. Plan 07 wires the full Phase 1 pipeline:
// logging → config → tray + RunApp goroutine (wizard or watcher) →
// systray.Run as the blocking main-thread loop. systray.Quit (from the
// tray's Quit menu) cancels the root ctx and unblocks main.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"fyne.io/systray"

	"github.com/boejowen/SquireBot/internal/app"
	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/logging"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/update"
)

func main() {
	// Plan 02-06 (OPS-04) startup-swap: BEFORE any other goroutine,
	// before logging.Setup, before config.Load, check for a staged
	// update adjacent to the running binary. If <exepath>.new + the
	// matching .expected-sha256 sidecar are present, minio/selfupdate
	// performs the .new -> live + live -> .old rename dance. On
	// success we MUST os.Exit(0) so the swapped-in binary takes over
	// on the next process launch (Windows: a running .exe holds its
	// file handle; we exit cleanly so the OS releases it).
	//
	// CONTEXT.md (locked): startup-swap NEVER in-process; SHA-256
	// verification is mandatory. All logic lives in internal/update/swap.go.
	//
	// Logging is not yet set up here, so any error goes to stderr.
	if swapped, err := update.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "auto-update apply failed: %v\n", err)
	} else if swapped {
		// Successful swap. Exit cleanly so the next process launch
		// runs the new binary. (The user-observable effect: tray icon
		// flickers; the new version takes over within milliseconds.)
		os.Exit(0)
	}

	log, logDir := logging.Setup()
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err, "path", config.Path())
		os.Exit(1)
	}

	bc := auth.BuildConstants{
		OAuthClientID:     OAuthClientID,
		OAuthClientSecret: OAuthClientSecret,
		PickerAPIKey:      PickerAPIKey,
		GCPProjectNumber:  GCPProjectNumber,
		WatcherVersion:    Version,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var trayCtl *tray.Controller
	trayCtl = tray.NewController(tray.Config{
		IconGreen:     iconBytes,
		IconRed:       iconBytes, // Phase 5 polish — distinct red art deferred
		LogDir:        logDir,
		SpreadsheetID: cfg.SpreadsheetID,
		OnContinueSetup: func() {
			slog.Info("Continue setup clicked — re-running RunApp")
			go app.RunApp(ctx, cfg, bc, trayCtl)
		},
		OnChangeWorkbook: func() {
			// D-04: re-run picker on existing token. RESEARCH.md §5.6.
			slog.Info("Change Workbook clicked — launching picker on existing token")
			go app.ChangeWorkbook(ctx, cfg, bc, trayCtl)
		},
		OnCheckUpdates: func() {
			// Plan 02-06 (OPS-04): manual fire of the daily check. Same
			// flow as the 24h goroutine in runWatcher; checkMu serializes
			// so a click landing concurrently with the tick is safe.
			slog.Info("Check for updates clicked — running update.CheckOnce")
			go func() {
				exe, err := os.Executable()
				if err != nil {
					slog.Warn("Check for updates: os.Executable failed", "err", err)
					trayCtl.SetStatus("Check for updates: cannot resolve exe path")
					return
				}
				if err := update.CheckOnce(ctx, "boejowen", "SquireBot", Version, exe, func(msg string) { trayCtl.SetStatus(msg) }); err != nil {
					slog.Warn("Check for updates failed", "err", err)
					trayCtl.SetStatus(fmt.Sprintf("Update check failed: %v", err))
				}
			}()
		},
		OnReauthorize: func() {
			// Plan 02-04 (AUTH-05): refresh token died. Re-run the OAuth
			// loopback flow against the existing email; on success the
			// wincred entry is replaced and the watcher resumes.
			slog.Info("Reauthorize clicked — running OAuth flow")
			go app.RunReauthorize(ctx, cfg, bc, trayCtl)
		},
		OnQuit: func() {
			slog.Info("Quit clicked — cancelling root context")
			cancel()
		},
	})

	// Background goroutine: wizard (if needed) then watcher loop.
	go app.RunApp(ctx, cfg, bc, trayCtl)

	slog.Info("squirebot starting",
		"version", Version,
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
		"log_dir", logDir,
		"config_path", config.Path(),
		"icon_bytes", len(iconBytes),
		"google_email", cfg.GoogleEmail,
		"spreadsheet_id_set", cfg.SpreadsheetID != "",
		"eq_folder_set", cfg.EQFolder != "",
	)

	// Main goroutine: systray.Run blocks until systray.Quit fires.
	systray.Run(trayCtl.OnReady, trayCtl.OnExit)

	// Tray quit → tear down background work.
	cancel()
	slog.Info("squirebot exit")
}
