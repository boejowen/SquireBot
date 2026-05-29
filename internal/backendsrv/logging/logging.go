// Package logging configures the SquireBot backend's process-wide structured
// logger. Unlike the watcher's internal/logging (which rotates a JSON file under
// %LOCALAPPDATA%\SquireBot via lumberjack — Windows-only, OPS-03), the backend
// runs on a Linux VPS under systemd, so it logs JSON to STDOUT and lets
// journald capture/rotate it (D-10). The JSON-handler shape is reused verbatim
// from the watcher (internal/logging/logger.go:47-51); only the sink changes
// (os.Stdout, not lumberjack-to-LOCALAPPDATA).
//
// SECURITY (V7 / CLAUDE.md): the backend NEVER logs the raw bearer token or the
// raw `content` payload — handlers/store emit operation + status + char name
// only. That discipline lives at the call sites (ingest/handler.go,
// store/*.go); this package only wires the handler.
package logging

import (
	"log/slog"
	"os"
)

// Setup builds the backend's JSON slog logger writing to os.Stdout (journald
// captures stdout for a systemd service — D-10) and installs it as the process
// default via slog.SetDefault, so package-level slog.Info/Warn/Error calls
// across the backend route through it. It returns the logger for callers that
// want an explicit handle.
//
// The handler options mirror the watcher's (Level=Info, AddSource=true) so log
// lines are greppable and carry source positions, matching CLAUDE.md's
// structured-logging convention. Server-side there is no log directory to return
// (systemd/journald owns retention), so unlike the watcher's Setup this returns
// only the *slog.Logger.
func Setup() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
