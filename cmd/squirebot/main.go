// Command squirebot is the per-guildie Windows watcher described in
// .planning/PROJECT.md. Plan 07 wires the full Phase 1 pipeline:
// logging → config → tray + RunApp goroutine (wizard or watcher) →
// systray.Run as the blocking main-thread loop. systray.Quit (from the
// tray's Quit menu) cancels the root ctx and unblocks main.
package main

import (
	"context"
	"log/slog"
	"os"
	"runtime"

	"fyne.io/systray"

	"github.com/jbowen-mn/squirebot/internal/app"
	"github.com/jbowen-mn/squirebot/internal/auth"
	"github.com/jbowen-mn/squirebot/internal/config"
	"github.com/jbowen-mn/squirebot/internal/logging"
	"github.com/jbowen-mn/squirebot/internal/tray"
)

func main() {
	log, logDir := logging.Setup()
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err, "path", config.Path())
		os.Exit(1)
	}

	bc := auth.BuildConstants{
		OAuthClientID:    OAuthClientID,
		PickerAPIKey:     PickerAPIKey,
		GCPProjectNumber: GCPProjectNumber,
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
