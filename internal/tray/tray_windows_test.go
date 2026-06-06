//go:build windows

package tray

// Live systray needs a Windows desktop session, so this test file only
// exercises the goroutine-safe mutator surface (SetStatus, SetIconHealth) and
// the offline-assertable MenuPlan() contract. The click-loop and OnReady are
// validated by the human smoke checkpoint on a real Win11 VM.
//
// Phase 13 (WATCH-09/D-3): the Sheets/OAuth menu items (Open Workbook, Change
// Workbook…, Reauthorize…, Continue setup…) were removed; a single always-visible
// "Enter guild code…" item triggers the native onboarding (OnEnterGuildCode).

import (
	"strings"
	"testing"
)

func TestNewController_ConfigPropagation(t *testing.T) {
	called := false
	c := NewController(Config{
		IconGreen:        []byte{0xCA, 0xFE},
		IconRed:          []byte{0xBA, 0xBE},
		LogDir:           `C:\\users\\foo\\AppData\\Local\\SquireBot`,
		OnEnterGuildCode: func() { called = true },
		OnQuit:           func() {},
	})

	if got := c.LogDir(); got != `C:\\users\\foo\\AppData\\Local\\SquireBot` {
		t.Errorf("LogDir = %q", got)
	}
	// Direct invoke the closure to confirm it's wired.
	c.onEnterGuildCode()
	if !called {
		t.Error("OnEnterGuildCode not wired")
	}
}

// TestPreReady_EnqueuesNotDrops verifies that mutator calls made before
// OnReady are queued (not silently dropped). Plan 09-01 / OPS-06.
func TestPreReady_EnqueuesNotDrops(t *testing.T) {
	c := NewController(Config{})
	c.SetStatus("hello")
	c.SetIconHealth(HealthGreen)
	c.SetIconHealth(HealthRed)
	snap := c.pendingSnapshot()
	if len(snap) != 3 {
		t.Fatalf("pending = %d entries, want 3", len(snap))
	}
	// No panic.
}

// TestPreReady_FIFOOrder verifies that queued actions retain insertion order.
// Plan 09-01.
func TestPreReady_FIFOOrder(t *testing.T) {
	c := NewController(Config{})
	c.SetIconHealth(HealthRed)
	c.SetStatus("auth error")
	c.SetStatus("recovered")

	snap := c.pendingSnapshot()
	if len(snap) != 3 {
		t.Fatalf("pendingSnapshot len = %d, want 3", len(snap))
	}
	wantKinds := []actionKind{actIconHealth, actStatus, actStatus}
	for i, w := range wantKinds {
		if snap[i].kind != w {
			t.Errorf("snap[%d].kind = %v, want %v", i, snap[i].kind, w)
		}
	}
	if snap[0].health != HealthRed {
		t.Errorf("snap[0].health = %v, want HealthRed", snap[0].health)
	}
	if snap[1].status != "auth error" {
		t.Errorf("snap[1].status = %q, want %q", snap[1].status, "auth error")
	}
	if snap[2].status != "recovered" {
		t.Errorf("snap[2].status = %q, want %q", snap[2].status, "recovered")
	}
}

// TestSimulateReady_DrainsQueue verifies simulateReady empties the queue
// and flips isReady. Plan 09-01.
func TestSimulateReady_DrainsQueue(t *testing.T) {
	c := NewController(Config{})
	c.SetStatus("queued before ready")
	c.SetIconHealth(HealthRed)
	if len(c.pendingSnapshot()) != 2 {
		t.Fatalf("pre-ready len = %d, want 2", len(c.pendingSnapshot()))
	}
	if c.isReady() {
		t.Fatal("isReady() = true before simulateReady; want false")
	}

	c.simulateReady()

	if !c.isReady() {
		t.Error("isReady() = false after simulateReady; want true")
	}
	if got := c.pendingSnapshot(); len(got) != 0 {
		t.Errorf("post-drain pending len = %d, want 0", len(got))
	}
}

// TestPostReady_ExecutesLive verifies that after OnReady (here simulated),
// subsequent mutator calls do NOT append to the pending queue. Plan 09-01.
func TestPostReady_ExecutesLive(t *testing.T) {
	c := NewController(Config{})
	c.simulateReady() // skip the queued phase entirely

	c.SetStatus("live")
	c.SetIconHealth(HealthGreen)

	if got := c.pendingSnapshot(); len(got) != 0 {
		t.Errorf("post-ready pending len = %d, want 0 (live execution should not enqueue)", len(got))
	}
}

func TestHealthConstants(t *testing.T) {
	if HealthGreen == HealthRed {
		t.Fatal("HealthGreen == HealthRed")
	}
}

// TestMenuPlan_Phase13Items pins the Phase-13 menu set + order: the Sheets/OAuth
// items are GONE and "Enter guild code…" is present.
//
//	0  Open log folder
//	1  Check for updates
//	2  Enter guild code…
//	3  Quit
func TestMenuPlan_Phase13Items(t *testing.T) {
	plan := MenuPlan()

	wantOrder := []string{
		LabelOpenLogFolder,  // 0
		LabelCheckUpdates,   // 1
		LabelEnterGuildCode, // 2
		LabelQuit,           // 3
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

// TestMenuPlan_NoSheetsOrOAuthItems is the deletion guard: the removed Sheets/
// OAuth labels must NOT appear anywhere in the menu plan.
func TestMenuPlan_NoSheetsOrOAuthItems(t *testing.T) {
	plan := MenuPlan()
	banned := []string{"Change Workbook", "Reauthorize", "Open Workbook", "Continue setup"}
	for _, item := range plan {
		for _, b := range banned {
			if strings.Contains(item.Label, b) {
				t.Errorf("MenuPlan contains a removed item label %q (matched %q)", item.Label, b)
			}
		}
	}
}

// TestOnEnterGuildCodeCallback_Wired verifies the OnEnterGuildCode closure is
// propagated from Config into the Controller's internal callback field.
func TestOnEnterGuildCodeCallback_Wired(t *testing.T) {
	calls := 0
	c := NewController(Config{
		OnEnterGuildCode: func() { calls++ },
	})
	if c.onEnterGuildCode == nil {
		t.Fatal("Controller.onEnterGuildCode not wired from Config.OnEnterGuildCode")
	}
	c.onEnterGuildCode()
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

// TestOnCheckUpdatesCallback_Wired (Plan 02-06) verifies the OnCheckUpdates
// closure is propagated from Config into the Controller's internal callback.
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

// TestLabelConstants_Stable guards the canonical menu-item label strings.
func TestLabelConstants_Stable(t *testing.T) {
	cases := map[string]string{
		"LabelOpenLogFolder":  LabelOpenLogFolder,
		"LabelCheckUpdates":   LabelCheckUpdates,
		"LabelEnterGuildCode": LabelEnterGuildCode,
		"LabelQuit":           LabelQuit,
	}
	want := map[string]string{
		"LabelOpenLogFolder":  "Open log folder",
		"LabelCheckUpdates":   "Check for updates",
		"LabelEnterGuildCode": "Enter guild code…",
		"LabelQuit":           "Quit",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s = %q, want %q", name, got, want[name])
		}
	}
}

// TestPendingAction_Zero verifies a freshly-constructed Controller has
// ready=false and an empty pending queue. Plan 09-01 Task 1.
func TestPendingAction_Zero(t *testing.T) {
	c := NewController(Config{})
	if c.isReady() {
		t.Error("freshly constructed Controller should not be ready")
	}
	if snap := c.pendingSnapshot(); len(snap) != 0 {
		t.Errorf("pendingSnapshot() = %d entries, want 0", len(snap))
	}
}
