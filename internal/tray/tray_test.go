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

// SetStatus / SetIconHealth / ShowContinueSetup / HideContinueSetup are
// no-ops when the underlying systray menu items are nil (i.e., before
// OnReady has been called). Verify they don't panic.
func TestMutators_SafeBeforeOnReady(t *testing.T) {
	c := NewController(Config{})
	c.SetStatus("hello")
	c.SetIconHealth(HealthGreen)
	c.SetIconHealth(HealthRed)
	c.ShowContinueSetup()
	c.HideContinueSetup()
}

func TestHealthConstants(t *testing.T) {
	if HealthGreen == HealthRed {
		t.Fatal("HealthGreen == HealthRed")
	}
}

// TestMenuPlan_ContextMandatoryItems guards CONTEXT.md's four mandatory
// menu items (Status / Open Workbook / Open log folder / Quit) plus
// the D-04 / D-07 additions, in the exact order OnReady builds them.
//
// Hotfix #4 motivation: the live binary was observed missing
// "Open log folder" — this test now fails-loud if anyone removes it.
func TestMenuPlan_ContextMandatoryItems(t *testing.T) {
	plan := MenuPlan()

	wantOrder := []string{
		LabelOpenWorkbook,   // 0
		LabelOpenLogFolder,  // 1 — CONTEXT.md mandatory, hotfix #4
		LabelChangeWorkbook, // 2 — D-04
		LabelContinueSetup,  // 3 — D-07
		LabelQuit,           // 4
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
		"LabelChangeWorkbook": LabelChangeWorkbook,
		"LabelContinueSetup":  LabelContinueSetup,
		"LabelQuit":           LabelQuit,
	}
	want := map[string]string{
		"LabelOpenWorkbook":   "Open Workbook",
		"LabelOpenLogFolder":  "Open log folder",
		"LabelChangeWorkbook": "Change Workbook…",
		"LabelContinueSetup":  "Continue setup…",
		"LabelQuit":           "Quit",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s = %q, want %q", name, got, want[name])
		}
	}
}
