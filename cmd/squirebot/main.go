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
	"github.com/boejowen/SquireBot/internal/system"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/update"
)

func main() {
	// Plan 02-07 (INST-04 / CONTEXT.md Q3): --uninstall-wipe-credentials.
	// Invoked by the NSIS uninstaller when the user answered "Yes" to the
	// "Also delete saved configuration and Google account credentials?"
	// prompt. We read config.GoogleEmail, delete the wincred entry under
	// SquireBot:<email>, and exit. The NSIS script runs this BEFORE
	// deleting squirebot.exe so the binary is still on disk to invoke.
	//
	// Runs FIRST (before update.Apply) — auto-update has no business
	// firing during an uninstall. We always exit 0, even on partial
	// state (no email in config, config load failure, wincred delete
	// failure) — the uninstaller must not block on a guildie who never
	// completed the wizard but ran the installer.
	if len(os.Args) >= 2 && os.Args[1] == "--uninstall-wipe-credentials" {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
			os.Exit(0)
		}
		if cfg.GoogleEmail == "" {
			fmt.Fprintln(os.Stderr, "no email in config; nothing to wipe")
			os.Exit(0)
		}
		if err := auth.DeleteToken(cfg.GoogleEmail); err != nil {
			fmt.Fprintf(os.Stderr, "wincred delete failed for %s: %v\n", cfg.GoogleEmail, err)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "wincred entry removed for %s\n", cfg.GoogleEmail)
		os.Exit(0)
	}

	// Plan 06 (INST-06): --quit. Invoked by the NSIS pre-install shim to
	// gracefully stop a running watcher before file overwrite. Opens the
	// Local\SquireBot-Shutdown named event and signals it; the running
	// instance's listener goroutine observes the signal and unwinds
	// through cancel() + systray.Quit(). This invocation exits 0 always
	// — a signal with no listener is a benign no-op per D-01, and NSIS
	// falls back to taskkill /F on timeout regardless of any error here.
	//
	// Runs FIRST (before update.Apply) — auto-update has no business
	// firing during a --quit signal invocation. Logging is not yet set
	// up; use stderr for all output (matches --uninstall-wipe-credentials
	// and update.Apply's stderr-only contract).
	if len(os.Args) >= 2 && os.Args[1] == "--quit" {
		if err := system.SignalShutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown signal failed: %v\n", err)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "shutdown signal sent")
		os.Exit(0)
	}

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

	// Plan 09-02 (OPS-07): detach from any inherited console. Must run AFTER
	// the --uninstall-wipe-credentials, --quit, and update.Apply short-circuit
	// blocks above (those paths write to stderr that NSIS / parent process
	// captures), but BEFORE logging.Setup so subsequent slog writes target
	// only the lumberjack-backed log file. Closing the launching shell no
	// longer kills the watcher. Safe (no-op) when the process has no
	// console (e.g., launched via Explorer double-click). See
	// console_windows.go / console_other.go for the build-tagged
	// implementations.
	_ = freeConsole()

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
		IconGreen:     iconGreenBytes,
		IconRed:       iconRedBytes,
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

	// Plan 06 (INST-06): named-event shutdown listener. Blocks on
	// Local\SquireBot-Shutdown; on signal, funnels through the SAME path
	// as the tray's Quit menu (cancel() + systray.Quit()). Idempotent —
	// double-fire (tray Quit + installer --quit racing) is harmless
	// because systray.Quit is internally idempotent and cancel() on an
	// already-cancelled ctx is a no-op. Goroutine exits on either signal
	// OR ctx.Done so it cannot leak when shutdown comes from another path.
	//
	// Per D-03: no drain coordination. In-flight batchUpdate calls
	// observe ctx cancellation through the existing mutex-funneled
	// sheet.Client retry envelope and abandon. WATCH-09 catch-up
	// re-uploads any missed file changes on next launch.
	go func() {
		select {
		case <-system.WaitForShutdown(ctx):
			slog.Info("shutdown signal received — cancelling root context")
			cancel()
			systray.Quit()
		case <-ctx.Done():
			// Normal shutdown from another path (tray Quit, OS signal).
			// WaitForShutdown's internal goroutine also observes ctx.Done
			// and cleans up its event handle via defer.
			return
		}
	}()

	slog.Info("squirebot starting",
		"version", Version,
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
		"log_dir", logDir,
		"config_path", config.Path(),
		"icon_green_bytes", len(iconGreenBytes),
		"icon_red_bytes", len(iconRedBytes),
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
