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
