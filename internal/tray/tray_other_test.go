//go:build !windows

package tray

// The headless (!windows) tray controller renders nothing, so this test only
// asserts the platform-agnostic contract: the MenuPlan() ordering, that
// NewController retains LogDir, and that the no-op mutators never panic.
// Phase 25 (LNX-01 / D-01): the Linux watcher is headless — no systray.

import "testing"

// TestMenuPlan_OrderHeadless pins the menu-plan order on the headless build so
// it stays in lockstep with the Windows build's contract.
//
//	0  Open log folder
//	1  Check for updates
//	2  Enter guild code…
//	3  Quit
func TestMenuPlan_OrderHeadless(t *testing.T) {
	plan := MenuPlan()
	want := []string{
		LabelOpenLogFolder,  // 0
		LabelCheckUpdates,   // 1
		LabelEnterGuildCode, // 2
		LabelQuit,           // 3
	}
	if len(plan) != len(want) {
		t.Fatalf("MenuPlan length = %d, want %d (%v)", len(plan), len(want), want)
	}
	for i, w := range want {
		if plan[i].Label != w {
			t.Errorf("MenuPlan[%d].Label = %q, want %q", i, plan[i].Label, w)
		}
		if plan[i].Tooltip == "" {
			t.Errorf("MenuPlan[%d] (%q) has empty tooltip", i, plan[i].Label)
		}
	}
}

// TestNewController_RetainsLogDir verifies the headless Controller threads
// Config.LogDir through to LogDir() (the one piece of state it keeps).
func TestNewController_RetainsLogDir(t *testing.T) {
	c := NewController(Config{LogDir: "/x"})
	if got := c.LogDir(); got != "/x" {
		t.Errorf("LogDir() = %q, want %q", got, "/x")
	}
}

// TestNoOpMutators_DoNotPanic verifies the no-op SetStatus / SetIconHealth never
// panic (they only log) — RunApp calls these unconditionally on every platform.
func TestNoOpMutators_DoNotPanic(t *testing.T) {
	c := NewController(Config{LogDir: "/x"})
	c.SetStatus("ok")
	c.SetIconHealth(HealthRed)
	c.SetIconHealth(HealthGreen)
	c.OnReady()
	c.OnExit()
}

// TestHealthConstants_DistinctHeadless guards that the Health constants stay
// distinct on the headless build (mirrors the Windows guard).
func TestHealthConstants_DistinctHeadless(t *testing.T) {
	if HealthGreen == HealthRed {
		t.Fatal("HealthGreen == HealthRed")
	}
}
