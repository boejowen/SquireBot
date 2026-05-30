// Package tray hosts the SquireBot system-tray UI controller.
//
// Phase 13 (WATCH-09 / D-3) menu surface — the Sheets/OAuth items are gone:
//
//	Status            (read-only label, e.g. "Last upload: Foo at 14:32")
//	Open log folder   (explorer.exe %LOCALAPPDATA%\SquireBot)
//	Check for updates (manual fire of update.CheckOnce)
//	Enter guild code… (D-3: triggers the native onboarding via OnEnterGuildCode)
//	Quit              (cancels app ctx + systray.Quit)
//
// Removed in Phase 13: "Open Workbook" (no Google Sheet), "Change Workbook…"
// (no workbook), "Reauthorize…" (no OAuth refresh-token death path), and
// "Continue setup…" (folded into the always-visible "Enter guild code…" item —
// re-running onboarding is always allowed).
//
// The Controller's menu construction is centralised in MenuPlan() so unit tests
// can assert the contract without a live systray (which requires a desktop
// session). The human smoke checkpoint validates the live tray on a Win11 VM.
//
// Green/red icon distinction: the Controller swaps tray icons via SetIconHealth
// using the IconGreen / IconRed bytes supplied by main.go.
package tray

import (
	"log/slog"
	"os/exec"
	"path/filepath"
	"sync"

	"fyne.io/systray"
)

// MenuItem describes one entry in the tray menu plan. The plan is consumed by
// OnReady (live) and asserted by tests (offline).
type MenuItem struct {
	Label   string
	Tooltip string
}

// Canonical menu labels — exported so callers / tests can reference them by name
// without relying on string-literal duplication.
const (
	LabelStatus         = "Initialising…" // disabled status row, mutated by SetStatus
	LabelOpenLogFolder  = "Open log folder"
	LabelCheckUpdates   = "Check for updates" // Plan 02-06 (OPS-04): manual fire of update.CheckOnce
	LabelEnterGuildCode = "Enter guild code…" // Phase 13 (D-3): triggers native onboarding
	LabelQuit           = "Quit"
)

// MenuPlan returns the ordered list of action menu items the tray builds
// (excluding the leading Status row and separators). Order is the contract.
func MenuPlan() []MenuItem {
	return []MenuItem{
		{Label: LabelOpenLogFolder, Tooltip: `Open %LOCALAPPDATA%\SquireBot in Explorer`},
		{Label: LabelCheckUpdates, Tooltip: "Check GitHub Releases for a newer SquireBot; downloads + verifies in the background."},
		{Label: LabelEnterGuildCode, Tooltip: "Enter (or re-enter) your guild code to connect to the SquireBot backend."},
		{Label: LabelQuit, Tooltip: "Exit SquireBot"},
	}
}

// Health drives the tray-icon swap. SetIconHealth flips between green (normal)
// and red (Setup needed / watcher error / invalid guild code).
type Health int

const (
	HealthGreen Health = iota
	HealthRed
)

// actionKind tags a deferred mutator call queued before OnReady. Plan 09-01 (OPS-06).
type actionKind int

const (
	actStatus actionKind = iota
	actIconHealth
)

// pendingAction is a single deferred mutator call. Only one payload field is
// meaningful per kind (the others stay zero-valued). Plan 09-01.
type pendingAction struct {
	kind   actionKind
	status string // actStatus
	health Health // actIconHealth
}

// Config bundles the construction-time inputs to NewController.
type Config struct {
	IconGreen        []byte
	IconRed          []byte
	LogDir           string
	OnCheckUpdates   func() // Plan 02-06 (OPS-04): manual fire of update.CheckOnce
	OnEnterGuildCode func() // Phase 13 (D-3): trigger native onboarding (re-runs RunApp)
	OnQuit           func() // app shutdown trigger
}

// Controller is the tray UI. NewController + OnReady/OnExit are the
// systray-facing surface; SetStatus / SetIconHealth are the goroutine-safe
// mutators called from RunApp.
type Controller struct {
	mu        sync.Mutex
	iconGreen []byte
	iconRed   []byte
	logDir    string

	// OPS-06 / Plan 09-01: queue-and-replay so pre-OnReady mutator calls are not
	// silently dropped. Both fields are guarded by t.mu (above).
	ready   bool
	pending []pendingAction

	mStatus         *systray.MenuItem
	mLogs           *systray.MenuItem
	mCheckUpdates   *systray.MenuItem // Plan 02-06 (OPS-04)
	mEnterGuildCode *systray.MenuItem // Phase 13 (D-3)
	mQuit           *systray.MenuItem

	onCheckUpdates   func()
	onEnterGuildCode func()
	onQuit           func()
}

// NewController allocates a Controller. systray.Run(t.OnReady, t.OnExit) from
// cmd/squirebot/main.go binds it to the live tray.
func NewController(c Config) *Controller {
	return &Controller{
		iconGreen:        c.IconGreen,
		iconRed:          c.IconRed,
		logDir:           c.LogDir,
		onCheckUpdates:   c.OnCheckUpdates,
		onEnterGuildCode: c.OnEnterGuildCode,
		onQuit:           c.OnQuit,
	}
}

// drainPending replays every queued mutator call against the now-live menu
// items, in FIFO insertion order. The caller MUST hold t.mu. Plan 09-01 / OPS-06.
func (t *Controller) drainPending() {
	for _, a := range t.pending {
		switch a.kind {
		case actStatus:
			if t.mStatus != nil {
				t.mStatus.SetTitle(a.status)
			}
		case actIconHealth:
			t.applyIconHealthLocked(a.health)
		}
	}
	t.pending = nil
}

// applyIconHealthLocked performs the systray icon swap. Caller MUST hold t.mu.
// Plan 09-01 / OPS-06.
func (t *Controller) applyIconHealthLocked(h Health) {
	switch h {
	case HealthGreen:
		if len(t.iconGreen) > 0 {
			systray.SetIcon(t.iconGreen)
		}
	case HealthRed:
		if len(t.iconRed) > 0 {
			systray.SetIcon(t.iconRed)
		}
	}
}

// OnReady is the systray.Run callback that builds the menu. systray itself is
// not test-friendly (needs a desktop session), so unit tests assert MenuPlan()
// instead of running this function. Order MUST match MenuPlan().
func (t *Controller) OnReady() {
	if len(t.iconGreen) > 0 {
		systray.SetIcon(t.iconGreen)
	}
	systray.SetTooltip("SquireBot")

	t.mStatus = systray.AddMenuItem(LabelStatus, "")
	t.mStatus.Disable()

	systray.AddSeparator()
	plan := MenuPlan()
	t.mLogs = systray.AddMenuItem(plan[0].Label, plan[0].Tooltip)           // Open log folder
	t.mCheckUpdates = systray.AddMenuItem(plan[1].Label, plan[1].Tooltip)   // Check for updates
	t.mEnterGuildCode = systray.AddMenuItem(plan[2].Label, plan[2].Tooltip) // Enter guild code…
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem(plan[3].Label, plan[3].Tooltip) // Quit

	// Plan 09-01 / OPS-06: drain any mutator calls queued before OnReady.
	t.mu.Lock()
	t.ready = true
	t.drainPending()
	t.mu.Unlock()

	go t.loop()
}

// loop fires the click-handlers. Each menu item ships its own ClickedCh so we
// just multiplex. systray.Quit is the canonical way to break out of systray.Run;
// we call OnQuit first so RunApp can cancel the root ctx, then systray.Quit
// unblocks main().
func (t *Controller) loop() {
	for {
		select {
		case _, ok := <-t.mLogs.ClickedCh:
			if !ok {
				return
			}
			if err := exec.Command("explorer.exe", filepath.Clean(t.logDir)).Start(); err != nil {
				slog.Warn("Open log folder failed", "err", err)
			}
		case _, ok := <-t.mCheckUpdates.ClickedCh:
			if !ok {
				return
			}
			slog.Info("Check for updates clicked")
			if t.onCheckUpdates != nil {
				t.onCheckUpdates()
			}
		case _, ok := <-t.mEnterGuildCode.ClickedCh:
			if !ok {
				return
			}
			slog.Info("Enter guild code clicked")
			if t.onEnterGuildCode != nil {
				t.onEnterGuildCode()
			}
		case _, ok := <-t.mQuit.ClickedCh:
			if !ok {
				return
			}
			slog.Info("Quit clicked")
			if t.onQuit != nil {
				t.onQuit()
			}
			systray.Quit()
			return
		}
	}
}

// OnExit is the systray exit callback. No-op; RunApp's ctx cancellation
// (triggered by mQuit's onQuit) is what tears down background work.
func (t *Controller) OnExit() {}

// SetStatus updates the disabled top menu label. Goroutine-safe. Pre-Ready calls
// are queued and replayed by OnReady. Plan 09-01 / OPS-06.
func (t *Controller) SetStatus(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actStatus, status: s})
		return
	}
	if t.mStatus != nil {
		t.mStatus.SetTitle(s)
	}
}

// SetIconHealth swaps the tray icon between green (normal) and red. Pre-Ready
// calls are queued. Plan 09-01 / OPS-06.
func (t *Controller) SetIconHealth(h Health) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actIconHealth, health: h})
		return
	}
	t.applyIconHealthLocked(h)
}

// LogDir returns the directory the "Open log folder" item targets.
func (t *Controller) LogDir() string { return t.logDir }

// pendingSnapshot returns a copy of the pending-action queue for tests.
// Plan 09-01 / OPS-06 — test surface only; not called from production.
func (t *Controller) pendingSnapshot() []pendingAction {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]pendingAction, len(t.pending))
	copy(out, t.pending)
	return out
}

// isReady reports whether OnReady has run and drained the queue.
// Plan 09-01 / OPS-06 — test surface only.
func (t *Controller) isReady() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ready
}

// simulateReady is a TEST-ONLY helper that flips the ready flag and drains the
// pending queue. Mirrors OnReady's drain block exactly. Plan 09-01.
func (t *Controller) simulateReady() {
	t.mu.Lock()
	t.ready = true
	t.drainPending()
	t.mu.Unlock()
}
