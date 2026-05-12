package tray

// Live systray needs a Windows desktop session, so this test file only
// exercises the goroutine-safe mutator surface that runApp uses
// (SetSpreadsheetID, SetStatus, SetIconHealth, ShowContinueSetup,
// HideContinueSetup, LogDir). The click-loop and OnReady are validated
// by Plan 08's smoke checkpoint on a real Win11 VM.

import (
	"testing"
)

func TestNewController_ConfigPropagation(t *testing.T) {
	called := false
	c := NewController(Config{
		IconGreen:        []byte{0xCA, 0xFE},
		IconRed:          []byte{0xBA, 0xBE},
		LogDir:           `C:\\users\\foo\\AppData\\Local\\SquireBot`,
		SpreadsheetID:    "SHEET1",
		OnContinueSetup:  func() { called = true },
		OnChangeWorkbook: func() {},
		OnQuit:           func() {},
	})

	if got := c.SpreadsheetID(); got != "SHEET1" {
		t.Errorf("SpreadsheetID = %q, want SHEET1", got)
	}
	if got := c.LogDir(); got != `C:\\users\\foo\\AppData\\Local\\SquireBot` {
		t.Errorf("LogDir = %q", got)
	}
	// Direct invoke the closure to confirm it's wired.
	c.onContinueSetup()
	if !called {
		t.Error("OnContinueSetup not wired")
	}
}

func TestSetSpreadsheetID_Mutates(t *testing.T) {
	c := NewController(Config{SpreadsheetID: ""})
	if c.SpreadsheetID() != "" {
		t.Fatalf("initial = %q, want ''", c.SpreadsheetID())
	}
	c.SetSpreadsheetID("NEW_ID")
	if c.SpreadsheetID() != "NEW_ID" {
		t.Errorf("after Set = %q, want NEW_ID", c.SpreadsheetID())
	}
}

// TestPreReady_EnqueuesNotDrops verifies that mutator calls made before
// OnReady are queued (not silently dropped). Plan 09-01 / OPS-06. Replaces
// the original no-panic-only smoke assertion with a positive enqueue check.
func TestPreReady_EnqueuesNotDrops(t *testing.T) {
	c := NewController(Config{})
	c.SetStatus("hello")
	c.SetIconHealth(HealthGreen)
	c.SetIconHealth(HealthRed)
	c.ShowContinueSetup()
	c.HideContinueSetup()
	c.ShowReauthorize()
	c.HideReauthorize()
	c.SetSpreadsheetID("abc")
	snap := c.pendingSnapshot()
	if len(snap) != 8 {
		t.Fatalf("pending = %d entries, want 8", len(snap))
	}
	// No panic.
}

func TestHealthConstants(t *testing.T) {
	if HealthGreen == HealthRed {
		t.Fatal("HealthGreen == HealthRed")
	}
}

// TestMenuPlan_ContextMandatoryItems guards CONTEXT.md's four mandatory
// menu items (Status / Open Workbook / Open log folder / Quit) plus
// the D-04 / D-07 / AUTH-05 additions, in the exact order OnReady builds them.
//
// Hotfix #4 motivation: the live binary was observed missing
// "Open log folder" — this test now fails-loud if anyone removes it.
//
// Plan 02-04 (AUTH-05): Reauthorize sits between Continue setup… and Quit.
// Plan 02-06 (OPS-04): "Check for updates" inserted at index 2 (between
// Open log folder and Change Workbook…). Final 7-item order:
//
//	0  Open Workbook
//	1  Open log folder         — CONTEXT.md mandatory (hotfix #4)
//	2  Check for updates       — Plan 02-06 (OPS-04)
//	3  Change Workbook…        — D-04
//	4  Continue setup…         — D-07 (hidden until needsWizard)
//	5  Reauthorize…            — Plan 02-04 (AUTH-05) (hidden until authSuspended)
//	6  Quit
//
// Hidden-by-default items (Continue setup, Reauthorize) still occupy a
// MenuPlan slot — the slot exists; visibility is the runtime concern.
func TestMenuPlan_ContextMandatoryItems(t *testing.T) {
	plan := MenuPlan()

	wantOrder := []string{
		LabelOpenWorkbook,   // 0
		LabelOpenLogFolder,  // 1 — CONTEXT.md mandatory, hotfix #4
		LabelCheckUpdates,   // 2 — Plan 02-06 (OPS-04)
		LabelChangeWorkbook, // 3 — D-04
		LabelContinueSetup,  // 4 — D-07
		LabelReauthorize,    // 5 — Plan 02-04 (AUTH-05)
		LabelQuit,           // 6
	}

	if len(plan) != len(wantOrder) {
		t.Fatalf("MenuPlan length = %d, want %d (%v)", len(plan), len(wantOrder), wantOrder)
	}
	for i, want := range wantOrder {
		if plan[i].Label != want {
			t.Errorf("MenuPlan[%d].Label = %q, want %q", i, plan[i].Label, want)
		}
		if plan[i].Tooltip == "" {
			t.Errorf("MenuPlan[%d] (%q) has empty tooltip", i, plan[i].Label)
		}
	}
}

// TestMenuPlan_ReauthorizePosition (Plan 02-04) pins Reauthorize between
// Continue setup… and Quit so the AUTH-05 click handler is adjacent to
// the other setup/recovery items.
func TestMenuPlan_ReauthorizePosition(t *testing.T) {
	plan := MenuPlan()
	idxContinue, idxReauth, idxQuit := -1, -1, -1
	for i, item := range plan {
		switch item.Label {
		case LabelContinueSetup:
			idxContinue = i
		case LabelReauthorize:
			idxReauth = i
		case LabelQuit:
			idxQuit = i
		}
	}
	if idxReauth == -1 {
		t.Fatal(`"Reauthorize…" missing from MenuPlan (AUTH-05)`)
	}
	if !(idxContinue < idxReauth && idxReauth < idxQuit) {
		t.Errorf("expected Continue setup… (%d) < Reauthorize… (%d) < Quit (%d)",
			idxContinue, idxReauth, idxQuit)
	}
}

// TestOnReauthorizeCallback_Wired (Plan 02-04) verifies the
// OnReauthorize closure is propagated from Config into the Controller's
// internal callback field. Live menu-click semantics are validated by
// Plan 08's smoke checkpoint.
func TestOnReauthorizeCallback_Wired(t *testing.T) {
	calls := 0
	c := NewController(Config{
		OnReauthorize: func() { calls++ },
	})
	if c.onReauthorize == nil {
		t.Fatal("Controller.onReauthorize not wired from Config.OnReauthorize")
	}
	c.onReauthorize()
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

// TestShowHideReauthorize_SafeBeforeOnReady — same contract as the other
// mutators: must not panic when the underlying systray menu item is nil.
func TestShowHideReauthorize_SafeBeforeOnReady(t *testing.T) {
	c := NewController(Config{})
	c.ShowReauthorize()
	c.HideReauthorize()
}

// TestMenuPlan_OpenLogFolder_Position pins the position of the
// "Open log folder" item between "Open Workbook" and "Change Workbook…"
// (CONTEXT.md ordering — keeps the two workbook-related items adjacent).
func TestMenuPlan_OpenLogFolder_Position(t *testing.T) {
	plan := MenuPlan()
	idxOpen, idxLogs, idxChange := -1, -1, -1
	for i, item := range plan {
		switch item.Label {
		case LabelOpenWorkbook:
			idxOpen = i
		case LabelOpenLogFolder:
			idxLogs = i
		case LabelChangeWorkbook:
			idxChange = i
		}
	}
	if idxLogs == -1 {
		t.Fatal(`"Open log folder" missing from MenuPlan (CONTEXT.md mandatory)`)
	}
	if !(idxOpen < idxLogs && idxLogs < idxChange) {
		t.Errorf("expected Open Workbook (%d) < Open log folder (%d) < Change Workbook… (%d)",
			idxOpen, idxLogs, idxChange)
	}
}

// TestLabelConstants_Stable guards the canonical menu-item label
// strings against accidental rename. Watcher logs and any future
// integration tests pin against these constants.
func TestLabelConstants_Stable(t *testing.T) {
	cases := map[string]string{
		"LabelOpenWorkbook":   LabelOpenWorkbook,
		"LabelOpenLogFolder":  LabelOpenLogFolder,
		"LabelCheckUpdates":   LabelCheckUpdates,
		"LabelChangeWorkbook": LabelChangeWorkbook,
		"LabelContinueSetup":  LabelContinueSetup,
		"LabelReauthorize":    LabelReauthorize,
		"LabelQuit":           LabelQuit,
	}
	want := map[string]string{
		"LabelOpenWorkbook":   "Open Workbook",
		"LabelOpenLogFolder":  "Open log folder",
		"LabelCheckUpdates":   "Check for updates",
		"LabelChangeWorkbook": "Change Workbook…",
		"LabelContinueSetup":  "Continue setup…",
		"LabelReauthorize":    "Reauthorize…",
		"LabelQuit":           "Quit",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s = %q, want %q", name, got, want[name])
		}
	}
}

// TestMenuPlan_CheckUpdatesPosition (Plan 02-06) pins LabelCheckUpdates
// between LabelOpenLogFolder and LabelChangeWorkbook so update concerns
// sit alongside the other operational menu items.
func TestMenuPlan_CheckUpdatesPosition(t *testing.T) {
	plan := MenuPlan()
	idxLogs, idxCheck, idxChange := -1, -1, -1
	for i, item := range plan {
		switch item.Label {
		case LabelOpenLogFolder:
			idxLogs = i
		case LabelCheckUpdates:
			idxCheck = i
		case LabelChangeWorkbook:
			idxChange = i
		}
	}
	if idxCheck == -1 {
		t.Fatal(`"Check for updates" missing from MenuPlan (OPS-04)`)
	}
	if !(idxLogs < idxCheck && idxCheck < idxChange) {
		t.Errorf("expected Open log folder (%d) < Check for updates (%d) < Change Workbook… (%d)",
			idxLogs, idxCheck, idxChange)
	}
}

// TestOnCheckUpdatesCallback_Wired (Plan 02-06) verifies the
// OnCheckUpdates closure is propagated from Config into the Controller's
// internal callback field. Live menu-click semantics validated by Plan
// 08's smoke checkpoint.
func TestOnCheckUpdatesCallback_Wired(t *testing.T) {
	calls := 0
	c := NewController(Config{
		OnCheckUpdates: func() { calls++ },
	})
	if c.onCheckUpdates == nil {
		t.Fatal("Controller.onCheckUpdates not wired from Config.OnCheckUpdates")
	}
	c.onCheckUpdates()
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

// TestPendingAction_Zero verifies the type scaffolding from Plan 09-01 Task 1:
// a freshly-constructed Controller has ready=false and an empty pending queue.
func TestPendingAction_Zero(t *testing.T) {
	c := NewController(Config{})
	if c.isReady() {
		t.Error("freshly constructed Controller should not be ready")
	}
	if snap := c.pendingSnapshot(); len(snap) != 0 {
		t.Errorf("pendingSnapshot() = %d entries, want 0", len(snap))
	}
}
