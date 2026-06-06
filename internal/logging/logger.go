// Package logging configures the process-wide structured logger.
//
// OPS-03 mandates JSON-formatted logs at %LOCALAPPDATA%\SquireBot\squirebot.log,
// rotated at 5 MB with 3 backups (cap ~20 MB) and a 28-day age cap.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup configures the global slog logger with a JSON handler writing to
// %LOCALAPPDATA%\SquireBot\squirebot.log via lumberjack rotation
// (MaxSize=5 MB, MaxBackups=3, MaxAge=28 days, LocalTime=true, Compress=false).
//
// Returns the *slog.Logger AND the directory used (so callers can locate logs
// for the tray "Open log folder" action — Plan 07).
//
// Side effect: also calls slog.SetDefault so package-level slog.Info calls
// elsewhere route through the same handler.
func Setup() (*slog.Logger, string) {
	logDir := defaultLogDir()
	logger, _, _ := setupAt(logDir)
	return logger, logDir
}

// defaultLogDir resolves the per-platform log directory.
//
//   - Windows: %LOCALAPPDATA%\SquireBot (UNCHANGED — D-05/D-07).
//   - Other (Linux, Phase 25 / D-05): $XDG_STATE_HOME/squirebot, defaulting to
//     ~/.local/state/squirebot. Logs are STATE (persistent, not cache), so
//     $XDG_STATE_HOME is the correct XDG base dir. The Go stdlib has no
//     UserStateDir helper, so the fallback is hand-rolled (no new dependency).
func defaultLogDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "SquireBot")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(stateHome, "squirebot")
}

// setupAt is the testable form of Setup: it lets the caller pin the log
// directory and reclaim the underlying io.Closer so tests can release the
// Windows file handle before t.TempDir() cleanup runs.
func setupAt(logDir string) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, err
	}
	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "squirebot.log"),
		MaxSize:    5,     // megabytes (OPS-03)
		MaxBackups: 3,     // (OPS-03)
		MaxAge:     28,    // days
		Compress:   false, // simpler debugging; lumberjack default
		LocalTime:  true,
	}
	handler := slog.NewJSONHandler(rotator, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, rotator, nil
}
