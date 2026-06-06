//go:build !windows

// Package tray hosts the SquireBot tray UI controller. On non-Windows
// platforms (Phase 25 / D-01: the Linux watcher is HEADLESS) there is no
// system tray — the systray dependency requires CGO/GTK on Linux and is
// therefore excluded from the !windows build. This file provides a no-op/logging
// Controller exposing the IDENTICAL exported surface as the Windows
// implementation in tray_windows.go, so app.RunApp's trayCtl.SetStatus /
// SetIconHealth calls compile and run unchanged. Status/health surface via the
// structured log (slog) instead of an icon.
//
// CRITICAL (D-07): this file must reproduce EVERY exported symbol the Windows
// tray exposes (MenuItem, MenuPlan, Health/HealthGreen/HealthRed, Config,
// Controller, NewController, OnReady, OnExit, SetStatus, SetIconHealth, LogDir,
// and the five Label* consts) so RunApp's signature (`*tray.Controller`,
// concrete) stays byte-for-byte identical across platforms. NO systray import,
// NO CGO.
package tray

import "log/slog"

// MenuItem describes one entry in the tray menu plan. Kept identical to the
// Windows definition so MenuPlan() has the same shape on both platforms.
type MenuItem struct {
	Label   string
	Tooltip string
}

// Canonical menu labels — identical to the Windows build. Retained on the
// headless build so callers/tests can reference them by name without a
// build-tag branch.
const (
	LabelStatus         = "Initialising…" // disabled status row, mutated by SetStatus
	LabelOpenLogFolder  = "Open log folder"
	LabelCheckUpdates   = "Check for updates"
	LabelEnterGuildCode = "Enter guild code…"
	LabelQuit           = "Quit"
)

// MenuPlan returns the same ordered action menu items as the Windows build.
// Kept for test-parity (the menu plan is a platform-agnostic contract even
// though the headless build never renders it).
func MenuPlan() []MenuItem {
	return []MenuItem{
		{Label: LabelOpenLogFolder, Tooltip: `Open the SquireBot log folder`},
		{Label: LabelCheckUpdates, Tooltip: "Check GitHub Releases for a newer SquireBot; downloads + verifies in the background."},
		{Label: LabelEnterGuildCode, Tooltip: "Enter (or re-enter) your guild code to connect to the SquireBot backend."},
		{Label: LabelQuit, Tooltip: "Exit SquireBot"},
	}
}

// Health drives the tray-icon swap on Windows. On the headless build it is only
// logged. Type + constants identical to the Windows build.
type Health int

const (
	HealthGreen Health = iota
	HealthRed
)

// Config bundles the construction-time inputs to NewController. Identical to the
// Windows build so main.go's tray.Config{...} literal compiles unchanged. On the
// headless build the icon bytes and click callbacks are accepted but unused (no
// tray to render or click).
type Config struct {
	IconGreen        []byte
	IconRed          []byte
	LogDir           string
	OnCheckUpdates   func()
	OnEnterGuildCode func()
	OnQuit           func()
}

// Controller is the headless no-op tray controller. It exposes the same method
// set as the Windows Controller but renders nothing; SetStatus/SetIconHealth log
// via slog. It retains only LogDir so the (unused on headless) "Open log folder"
// intent and diagnostics keep working.
type Controller struct {
	logDir string
}

// NewController allocates a headless Controller. The Windows build binds it to
// the live tray via systray.Run; here it is a pure log/no-op object.
func NewController(c Config) *Controller {
	return &Controller{logDir: c.LogDir}
}

// OnReady is a no-op on the headless build (no systray to build a menu in).
func (t *Controller) OnReady() {}

// OnExit is a no-op on the headless build.
func (t *Controller) OnExit() {}

// SetStatus logs the status line instead of mutating a tray label. Goroutine-safe
// (a plain slog call). Never panics. D-01: status surfaces via the log on Linux.
func (t *Controller) SetStatus(s string) { slog.Info("status", "msg", s) }

// SetIconHealth logs the health state instead of swapping a tray icon. Never
// panics. D-01: health surfaces via the log on Linux.
func (t *Controller) SetIconHealth(h Health) { slog.Info("health", "state", int(h)) }

// LogDir returns the directory the (Windows) "Open log folder" item targets; on
// the headless build it is retained for diagnostics / --status.
func (t *Controller) LogDir() string { return t.logDir }
