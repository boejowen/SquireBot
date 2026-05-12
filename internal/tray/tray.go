// Package tray hosts the SquireBot system-tray UI controller. The
// menu surface follows CONTEXT.md "Claude's Discretion" floor:
//
//	Status            (read-only label, e.g. "Last upload: Foo at 14:32")
//	Open Workbook     (rundll32 url.dll,FileProtocolHandler — Pitfall #6)
//	Open log folder   (explorer.exe %LOCALAPPDATA%\SquireBot)
//	Change Workbook…  (D-04 — re-runs picker via OnChangeWorkbook callback)
//	Continue setup…   (D-07 — hidden until needsWizard; OnContinueSetup callback)
//	Quit              (cancels app ctx + systray.Quit)
//
// The Controller's menu construction is centralised in MenuPlan() so
// unit tests can assert the contract without a live systray (which
// requires a desktop session). Plan 08 smoke checkpoint validates the
// live tray on Win11 VM.
//
// Green/red icon distinction: the Controller swaps tray icons via
// SetIconHealth using the IconGreen / IconRed bytes supplied by main.go
// (sourced from assets/icon-green.ico and assets/icon-red.ico —
// distinct 16x16 BMP-in-ICO solid-color placeholders, 1118 bytes each).
// Day-1 soak finding 2026-05-03 promoted this from the original "same
// bytes for both" stand-in noted as deferred Phase 5 polish.
package tray

import (
	"log/slog"
	"os/exec"
	"path/filepath"
	"sync"

	"fyne.io/systray"
)

// MenuItem describes one entry in the tray menu plan. The plan is
// consumed by OnReady (live) and asserted by tests (offline).
type MenuItem struct {
	Label   string
	Tooltip string
}

// Canonical menu labels — exported so callers / tests can reference
// them by name without relying on string-literal duplication.
const (
	LabelStatus         = "Initialising…" // disabled status row, mutated by SetStatus
	LabelOpenWorkbook   = "Open Workbook"
	LabelOpenLogFolder  = "Open log folder"
	LabelCheckUpdates   = "Check for updates" // Plan 02-06 (OPS-04): manual fire of update.CheckOnce
	LabelChangeWorkbook = "Change Workbook…"
	LabelContinueSetup  = "Continue setup…"
	LabelReauthorize    = "Reauthorize…" // Plan 02-04 (AUTH-05): hidden until ErrPermanentAuth/IsRevokedRefreshToken trips authSuspended
	LabelQuit           = "Quit"
)

// MenuPlan returns the ordered list of action menu items the tray
// builds (excluding the leading Status row and separators). Order is
// the contract: CONTEXT.md mandates Status / Open Workbook / Open log
// folder / Quit be present; D-04 and D-07 add Change Workbook… and
// Continue setup… adjacent to their workflow neighbours.
func MenuPlan() []MenuItem {
	return []MenuItem{
		{Label: LabelOpenWorkbook, Tooltip: "Open the configured Google Sheet in your browser"},
		{Label: LabelOpenLogFolder, Tooltip: `Open %LOCALAPPDATA%\SquireBot in Explorer`},
		{Label: LabelCheckUpdates, Tooltip: "Check GitHub Releases for a newer SquireBot; downloads + verifies in the background."},
		{Label: LabelChangeWorkbook, Tooltip: "Pick a different SquireBot workbook (re-runs Picker)"},
		{Label: LabelContinueSetup, Tooltip: "Resume the SquireBot wizard"},
		{Label: LabelReauthorize, Tooltip: "Refresh token died. Click to re-sign-in to Google."},
		{Label: LabelQuit, Tooltip: "Exit SquireBot"},
	}
}

// Health drives the tray-icon swap. SetIconHealth flips between green
// (normal) and red (Setup needed / watcher error / OAuth gate).
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
	actShowContinueSetup
	actHideContinueSetup
	actShowReauthorize
	actHideReauthorize
	actSetSpreadsheetID
)

// pendingAction is a single deferred mutator call. Only one payload field
// is meaningful per kind (the others stay zero-valued). Plan 09-01.
type pendingAction struct {
	kind          actionKind
	status        string // actStatus
	health        Health // actIconHealth
	spreadsheetID string // actSetSpreadsheetID
}

// Config bundles the construction-time inputs to NewController.
type Config struct {
	IconGreen        []byte
	IconRed          []byte
	LogDir           string
	SpreadsheetID    string // initial; can be empty (wizard not yet run)
	OnContinueSetup  func() // wizard re-entry trigger (D-07)
	OnChangeWorkbook func() // D-04: re-run picker on existing token
	OnCheckUpdates   func() // Plan 02-06 (OPS-04): manual fire of update.CheckOnce
	OnReauthorize    func() // Plan 02-04 (AUTH-05): re-run OAuth loopback flow on refresh-token death
	OnQuit           func() // app shutdown trigger
}

// Controller is the tray UI. NewController + OnReady/OnExit are the
// systray-facing surface; SetStatus / SetIconHealth /
// ShowContinueSetup / HideContinueSetup / SetSpreadsheetID are the
// goroutine-safe mutators called from runApp.
type Controller struct {
	mu            sync.Mutex
	iconGreen     []byte
	iconRed       []byte
	logDir        string
	spreadsheetID string

	// OPS-06 / Plan 09-01: queue-and-replay so pre-OnReady mutator calls
	// are not silently dropped. Both fields are guarded by t.mu (above).
	ready   bool
	pending []pendingAction

	mStatus         *systray.MenuItem
	mWorkbook       *systray.MenuItem
	mLogs           *systray.MenuItem
	mCheckUpdates   *systray.MenuItem // Plan 02-06 (OPS-04)
	mChangeWorkbook *systray.MenuItem // D-04
	mContinueSetup  *systray.MenuItem // D-07 (hidden by default)
	mReauthorize    *systray.MenuItem // Plan 02-04 (AUTH-05) — hidden by default
	mQuit           *systray.MenuItem

	onContinueSetup  func()
	onChangeWorkbook func()
	onCheckUpdates   func()
	onReauthorize    func()
	onQuit           func()
}

// NewController allocates a Controller. systray.Run(t.OnReady, t.OnExit)
// from cmd/squirebot/main.go binds it to the live tray.
func NewController(c Config) *Controller {
	return &Controller{
		iconGreen:        c.IconGreen,
		iconRed:          c.IconRed,
		logDir:           c.LogDir,
		spreadsheetID:    c.SpreadsheetID,
		onContinueSetup:  c.OnContinueSetup,
		onChangeWorkbook: c.OnChangeWorkbook,
		onCheckUpdates:   c.OnCheckUpdates,
		onReauthorize:    c.OnReauthorize,
		onQuit:           c.OnQuit,
	}
}

// drainPending replays every queued mutator call against the now-live
// menu items, in FIFO insertion order. The caller MUST hold t.mu.
// Plan 09-01 / OPS-06.
func (t *Controller) drainPending() {
	for _, a := range t.pending {
		switch a.kind {
		case actStatus:
			if t.mStatus != nil {
				t.mStatus.SetTitle(a.status)
			}
		case actIconHealth:
			t.applyIconHealthLocked(a.health)
		case actShowContinueSetup:
			if t.mContinueSetup != nil {
				t.mContinueSetup.Show()
			}
		case actHideContinueSetup:
			if t.mContinueSetup != nil {
				t.mContinueSetup.Hide()
			}
		case actShowReauthorize:
			if t.mReauthorize != nil {
				t.mReauthorize.Show()
			}
		case actHideReauthorize:
			if t.mReauthorize != nil {
				t.mReauthorize.Hide()
			}
		case actSetSpreadsheetID:
			t.spreadsheetID = a.spreadsheetID
		}
	}
	t.pending = nil
}

// applyIconHealthLocked performs the systray icon swap. Caller MUST hold
// t.mu. Plan 09-01 / OPS-06.
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

// OnReady is the systray.Run callback that builds the menu. systray
// itself is not test-friendly (needs a desktop session), so unit tests
// assert MenuPlan() instead of running this function. Order MUST match
// MenuPlan() — both reflect CONTEXT.md's "Claude's Discretion" floor.
func (t *Controller) OnReady() {
	if len(t.iconGreen) > 0 {
		systray.SetIcon(t.iconGreen)
	}
	systray.SetTooltip("SquireBot")

	t.mStatus = systray.AddMenuItem(LabelStatus, "")
	t.mStatus.Disable()

	systray.AddSeparator()
	plan := MenuPlan()
	t.mWorkbook = systray.AddMenuItem(plan[0].Label, plan[0].Tooltip)       // Open Workbook
	t.mLogs = systray.AddMenuItem(plan[1].Label, plan[1].Tooltip)           // Open log folder
	t.mCheckUpdates = systray.AddMenuItem(plan[2].Label, plan[2].Tooltip)   // Check for updates (Plan 02-06 / OPS-04)
	t.mChangeWorkbook = systray.AddMenuItem(plan[3].Label, plan[3].Tooltip) // Change Workbook… (D-04)
	t.mContinueSetup = systray.AddMenuItem(plan[4].Label, plan[4].Tooltip)  // Continue setup… (D-07)
	t.mContinueSetup.Hide()                                                 // D-07: shown only when wizard is incomplete
	t.mReauthorize = systray.AddMenuItem(plan[5].Label, plan[5].Tooltip)    // Reauthorize… (Plan 02-04 / AUTH-05)
	t.mReauthorize.Hide()                                                   // surfaced only when authSuspended is true
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem(plan[6].Label, plan[6].Tooltip) // Quit

	go t.loop()
}

// loop fires the click-handlers. Each menu item ships its own
// ClickedCh so we just multiplex. systray.Quit is the canonical way
// to break out of systray.Run; we call OnQuit first so runApp can
// cancel the root ctx, then systray.Quit unblocks main().
func (t *Controller) loop() {
	for {
		select {
		case _, ok := <-t.mWorkbook.ClickedCh:
			if !ok {
				return
			}
			t.mu.Lock()
			id := t.spreadsheetID
			t.mu.Unlock()
			if id == "" {
				slog.Info("Open Workbook clicked but no spreadsheet configured yet")
				continue
			}
			url := "https://docs.google.com/spreadsheets/d/" + id
			// Pitfall #6: rundll32 sidesteps the cmd shell's `&` ambiguity.
			if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
				slog.Warn("Open Workbook failed", "err", err)
			}
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
		case _, ok := <-t.mChangeWorkbook.ClickedCh:
			if !ok {
				return
			}
			slog.Info("Change Workbook clicked")
			if t.onChangeWorkbook != nil {
				t.onChangeWorkbook()
			}
		case _, ok := <-t.mContinueSetup.ClickedCh:
			if !ok {
				return
			}
			slog.Info("Continue setup clicked")
			if t.onContinueSetup != nil {
				t.onContinueSetup()
			}
		case _, ok := <-t.mReauthorize.ClickedCh:
			if !ok {
				return
			}
			slog.Info("Reauthorize clicked")
			if t.onReauthorize != nil {
				t.onReauthorize()
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

// OnExit is the systray exit callback. No-op; runApp's ctx cancellation
// (triggered by mQuit's onQuit) is what tears down background work.
func (t *Controller) OnExit() {}

// SetStatus updates the disabled top menu label. Goroutine-safe. Pre-Ready
// calls are queued and replayed by OnReady. Plan 09-01 / OPS-06.
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

// ShowContinueSetup makes the Continue setup… item visible. D-07. Pre-Ready
// calls are queued. Plan 09-01 / OPS-06.
func (t *Controller) ShowContinueSetup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actShowContinueSetup})
		return
	}
	if t.mContinueSetup != nil {
		t.mContinueSetup.Show()
	}
}

// HideContinueSetup hides the Continue setup… item. Pre-Ready calls are queued. Plan 09-01.
func (t *Controller) HideContinueSetup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actHideContinueSetup})
		return
	}
	if t.mContinueSetup != nil {
		t.mContinueSetup.Hide()
	}
}

// ShowReauthorize makes the Reauthorize… item visible. Plan 02-04
// (AUTH-05) + AUTH-07 boot path. Pre-Ready calls are queued. Plan 09-01 / OPS-06.
func (t *Controller) ShowReauthorize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actShowReauthorize})
		return
	}
	if t.mReauthorize != nil {
		t.mReauthorize.Show()
	}
}

// HideReauthorize hides the Reauthorize… item. Pre-Ready calls are queued. Plan 09-01.
func (t *Controller) HideReauthorize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actHideReauthorize})
		return
	}
	if t.mReauthorize != nil {
		t.mReauthorize.Hide()
	}
}

// SetSpreadsheetID updates the cached spreadsheet ID. Plan 09-01 / OPS-06:
// the in-memory field is always kept current; if pre-Ready, the assignment is
// also queued so the drain path is symmetric with the other mutators.
func (t *Controller) SetSpreadsheetID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spreadsheetID = id
	if !t.ready {
		t.pending = append(t.pending, pendingAction{kind: actSetSpreadsheetID, spreadsheetID: id})
	}
}

// SpreadsheetID returns the currently-tracked spreadsheet ID
// (read-only, for diagnostics).
func (t *Controller) SpreadsheetID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.spreadsheetID
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
