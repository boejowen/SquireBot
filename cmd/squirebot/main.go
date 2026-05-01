// Command squirebot is the per-guildie Windows watcher described in
// .planning/PROJECT.md. Phase 1 ships a smoke-only entry point that proves
// out logging + config + icon embedding; Plans 03-07 wire the real behaviour
// (OAuth, Drive Picker, EQ-folder watcher, Sheets writes, tray UI).
package main

import (
	"log/slog"
	"os"
	"runtime"

	"github.com/jbowen-mn/squirebot/internal/config"
	"github.com/jbowen-mn/squirebot/internal/logging"
)

// Version moved to build_constants.go in Plan 01-03 — that file is the
// canonical home for every -ldflags-injected package-main variable.

func main() {
	log, logDir := logging.Setup()
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err, "path", config.Path())
		os.Exit(1)
	}

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

	// Phase 1 contract: print one structured INFO line and exit cleanly.
	// Plan 03/04/05/06/07 will replace this body with OAuth → Picker →
	// folder discovery → fsnotify → Sheets batchUpdate → systray wiring.
}
