// Command squirebot is the per-guildie watcher described in
// .planning/PROJECT.md. Plan 07 wires the full Phase 1 pipeline:
// logging → config → tray + RunApp goroutine (wizard or watcher) → a
// build-tag-split blocking main-thread loop (runMainLoop in run_windows.go /
// run_other.go). On Windows that loop runs the system tray; on Linux (Phase 25,
// D-01 headless) it blocks on ctx.Done() with a SIGINT/SIGTERM handler. The
// tray Quit menu (Windows) or a delivered SIGTERM (Linux) cancels the root ctx
// and unblocks main.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/boejowen/SquireBot/internal/app"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/credstore"
	"github.com/boejowen/SquireBot/internal/eqfind"
	"github.com/boejowen/SquireBot/internal/logging"
	"github.com/boejowen/SquireBot/internal/system"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/update"
)

func main() {
	// Plan 02-07 (INST-04 / CONTEXT.md Q3): --uninstall-wipe-credentials.
	// Invoked by the NSIS uninstaller when the user answered "Yes" to the
	// "Also delete saved configuration and credentials?" prompt. Phase 13
	// (WATCH-10): the credential is now the guild code under the fixed target
	// SquireBot:guild-code (no email key), so we delete it via credstore.Delete.
	// The NSIS script runs this BEFORE deleting squirebot.exe so the binary is
	// still on disk to invoke.
	//
	// Runs FIRST (before update.Apply) — auto-update has no business firing
	// during an uninstall. We always exit 0, even on a not-found credential —
	// the uninstaller must not block on a guildie who never onboarded.
	if len(os.Args) >= 2 && os.Args[1] == "--uninstall-wipe-credentials" {
		if err := credstore.Delete(); err != nil {
			fmt.Fprintf(os.Stderr, "wincred delete (guild code) failed or absent: %v\n", err)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "wincred guild-code entry removed")
		os.Exit(0)
	}

	// Plan 06 (INST-06): --quit. Invoked by the NSIS pre-install shim to
	// gracefully stop a running watcher before file overwrite. Opens the
	// Local\SquireBot-Shutdown named event and signals it; the running
	// instance's listener goroutine observes the signal and unwinds
	// through cancel() + the tray Quit path. This invocation exits 0 always
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

	// Phase 13 (WATCH-11): first-launch v1→v2 migration. On a watcher that just
	// auto-updated from v1.x this deletes the stale Google refresh-token wincred
	// entry and drops the dead config.json fields (google_email, spreadsheet_id).
	// Idempotent + non-fatal — a failed migration must not block startup.
	if err := app.MigrateFromV1(cfg); err != nil {
		slog.Warn("v1→v2 migration", "err", err)
	}

	// Backend base URL: a config override wins (advanced/self-host); otherwise
	// the hardcoded build_constants.go default (the canonical host).
	baseURL := BackendBaseURL
	if cfg.BackendBaseURL != "" {
		baseURL = cfg.BackendBaseURL
	}

	// Phase 25 / LNX-04 (D-02): headless Linux CLI subcommands. These are
	// dispatched AFTER logging.Setup + config.Load (they need config) and run
	// synchronously to completion + os.Exit — they NEVER enter the watcher loop.
	// SCOPE: --setup/--status are LINUX-path deliverables this phase; on Windows
	// onboarding stays wizard/tray-driven (a console --setup would pop the Win32
	// dialog — out of scope/untested here). The tray controller used for --setup
	// is the build-tagged no-op on Linux (tray_other.go).
	if len(os.Args) >= 2 && os.Args[1] == "--status" {
		// Phase 25 / CR-01 (LNX-05, D-06): the exit code is the machine-readable
		// half of --status. install.sh keys its first-run onboarding branch on
		// `--status` exiting non-zero ("not configured" → run --setup). So
		// runStatus returns true IFF fully configured (guild code AND an EQ
		// folder) and we exit 0; otherwise exit 1 so a fresh box runs --setup.
		// The human-readable summary still prints either way. The guild code is
		// NEVER printed (V7).
		if runStatus(cfg, logDir) {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if len(os.Args) >= 2 && os.Args[1] == "--setup" {
		// WR-05: install a SIGINT/SIGTERM → cancel handler for the setup path too
		// (mirrors run_other.go's runMainLoop). The stdin prompts themselves are
		// inherently blocking reads, but cancelling setupCtx unwinds any hung
		// backend Validate call (threaded through app.RunSetup → bc.Validate) so
		// Ctrl-C during a slow network validation aborts cleanly instead of
		// hanging until SIGKILL.
		setupCtx, setupCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer setupCancel()
		setupTray := tray.NewController(tray.Config{LogDir: logDir})
		// Best-effort EQ-folder auto-discovery (WINE scan on Linux) to pre-fill
		// config so onboarding can skip the folder prompt when it succeeds.
		if !hasAnyEQFolder(cfg) {
			if found, derr := eqfind.Discover(); derr == nil && found != "" {
				cfg.EQFolder = found
				cfg.EQFolders = []string{found}
				if serr := cfg.Save(); serr != nil {
					slog.Warn("--setup: save auto-discovered EQ folder", "err", serr)
				} else {
					slog.Info("--setup: auto-discovered EQ folder", "path", found)
				}
			}
		}
		if err := app.RunSetup(setupCtx, cfg, baseURL, Version, setupTray); err != nil {
			fmt.Fprintf(os.Stderr, "setup did not complete: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "SquireBot setup complete.")
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var trayCtl *tray.Controller
	trayCtl = tray.NewController(tray.Config{
		IconGreen: iconGreenBytes,
		IconRed:   iconRedBytes,
		LogDir:    logDir,
		OnEnterGuildCode: func() {
			// Phase 13 (D-3): re-run RunApp; its credstore.Read branch re-enters
			// the native onboarding (prompt → validate → store → EQ folder).
			// Folds the old Continue-setup intent into this single item.
			slog.Info("Enter guild code clicked — re-running RunApp")
			go app.RunApp(ctx, cfg, baseURL, Version, trayCtl)
		},
		OnCheckUpdates: func() {
			// Plan 02-06 (OPS-04): manual fire of the daily check. Same flow as
			// the 24h goroutine in runWatcher; checkMu serializes so a click
			// landing concurrently with the tick is safe.
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
		OnQuit: func() {
			slog.Info("Quit clicked — cancelling root context")
			cancel()
		},
	})

	// Background goroutine: onboarding (if needed) then watcher loop.
	go app.RunApp(ctx, cfg, baseURL, Version, trayCtl)

	slog.Info("squirebot starting",
		"version", Version,
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
		"log_dir", logDir,
		"config_path", config.Path(),
		"icon_green_bytes", len(iconGreenBytes),
		"icon_red_bytes", len(iconRedBytes),
		"backend_base_url", baseURL,
		"eq_folder_set", cfg.EQFolder != "" || len(cfg.EQFolders) > 0,
	)

	// Main-goroutine blocking tail. Build-tag-split (run_windows.go /
	// run_other.go): on Windows it runs the live system tray + the named-event
	// shutdown listener (unchanged behavior); on !windows (Linux, D-01 headless) it
	// blocks on ctx.Done() with a mandatory SIGINT/SIGTERM handler driving
	// cancel() so systemd `systemctl --user stop` unwinds gracefully (LNX-05).
	runMainLoop(ctx, cancel, trayCtl)

	slog.Info("squirebot exit")
}

// hasAnyEQFolder reports whether any EQ folder is configured (legacy single
// EQFolder or the EQFolders slice). Mirrors app.hasEQFolder (unexported there).
func hasAnyEQFolder(cfg *config.Config) bool {
	return cfg.EQFolder != "" || len(cfg.EQFolders) > 0
}

// runStatus prints a headless health/config summary to STDOUT for `--status`
// (Phase 25 / LNX-04) and REPORTS CONFIGURED-NESS VIA ITS RETURN VALUE: it
// returns true IFF BOTH a guild code AND an EQ folder are configured. The
// caller maps that bool to the process exit code (CR-01) so install.sh's
// "run --setup when --status fails" contract works on a fresh box. It NEVER
// prints the guild code itself (V7 / T-25-06): credstore.Read is consulted
// ONLY to report "configured" vs "not configured".
func runStatus(cfg *config.Config, logDir string) (configured bool) {
	codeConfigured := false
	if code, err := credstore.Read(); err == nil && code != "" {
		codeConfigured = true
	}
	codeStatus := "not configured"
	if codeConfigured {
		codeStatus = "configured"
	}

	folders := cfg.EQFolders
	if len(folders) == 0 && cfg.EQFolder != "" {
		folders = []string{cfg.EQFolder}
	}
	eqFolders := "(none)"
	if len(folders) > 0 {
		eqFolders = strings.Join(folders, ", ")
	}

	fmt.Printf("SquireBot %s\n", Version)
	fmt.Printf("  config path:  %s\n", config.Path())
	fmt.Printf("  log dir:      %s\n", logDir)
	fmt.Printf("  guild code:   %s\n", codeStatus)
	fmt.Printf("  EQ folder(s): %s\n", eqFolders)
	fmt.Printf("  backend:      %s\n", func() string {
		if cfg.BackendBaseURL != "" {
			return cfg.BackendBaseURL
		}
		return BackendBaseURL
	}())

	// Fully configured == an authenticated bearer code AND at least one EQ
	// folder to watch. Either missing → exit non-zero → install.sh runs --setup.
	return statusConfigured(codeConfigured, len(folders))
}

// statusConfigured is the pure exit-code predicate behind runStatus (CR-01):
// the watcher is "configured" (→ exit 0) only when a guild code is present AND
// at least one EQ folder is set. Extracted as a pure function so the exit-code
// contract is unit-testable without touching the on-disk credstore/config.
func statusConfigured(codeConfigured bool, eqFolderCount int) bool {
	return codeConfigured && eqFolderCount > 0
}
