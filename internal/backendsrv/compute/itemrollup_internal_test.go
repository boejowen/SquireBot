package compute

// itemrollup_internal_test.go is a white-box (package compute) unit test for the
// unexported pure transform buildItemRollups + the slotLabel helper — the
// grouping/flag-propagation/slot-label logic the view.go public-fn → pure-helper split
// exists to make directly table-testable (no ctx/store, mirrors pickprice_internal_test.go).

import (
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// TestBuildItemRollups proves the pure grouping over hand-built slices: two ViewRows of
// the same name in different chars/slots collapse to ONE rollup (summed qty, distinct
// holder count); is_mine propagates from any viewer-assigned holder; price/icon/stats are
// copied/keyed from the representative row; an unpriced row stays nil.
func TestBuildItemRollups(t *testing.T) {
	price := 42.0
	viewRows := []ViewRow{
		{Char: "Apple", Slot: "Primary", Item: "Short Sword", ID: 100, Count: 1, Price: &price, LastSynced: "2026-05-09T00:00:00Z"},
		{Char: "Banktoon", Slot: "Bank2", Item: "Short Sword", ID: 100, Count: 3, LastSynced: "2026-05-08T00:00:00Z"},
		{Char: "Apple", Slot: "General3", Item: "Cloth Cap", ID: 200, Count: 1, LastSynced: "2026-05-09T00:00:00Z"},
	}
	roster := []store.RosterRow{
		{Name: "Apple", IsMine: true},
		{Name: "Banktoon", IsBankToon: true},
	}
	iconStats := map[int64]store.IconStats{
		100: {IconID: 7, Statsblock: "DMG: 5"},
	}

	rolls := buildItemRollups(viewRows, roster, iconStats)
	if len(rolls) != 2 {
		t.Fatalf("got %d rollups, want 2 (Short Sword, Cloth Cap): %+v", len(rolls), rolls)
	}
	// First-seen order preserved: Short Sword (row 0) before Cloth Cap.
	if rolls[0].Name != "Short Sword" || rolls[1].Name != "Cloth Cap" {
		t.Errorf("order = [%q, %q], want first-seen Short Sword then Cloth Cap", rolls[0].Name, rolls[1].Name)
	}

	byName := map[string]ItemRollup{}
	for _, r := range rolls {
		byName[r.Name] = r
	}

	ss := byName["Short Sword"]
	if ss.SummedQty != 4 {
		t.Errorf("Short Sword summed_qty = %d, want 4 (1 + 3)", ss.SummedQty)
	}
	if ss.HolderCount != 2 {
		t.Errorf("Short Sword holder_count = %d, want 2", ss.HolderCount)
	}
	if !ss.IsMine {
		t.Errorf("Short Sword is_mine = false, want true (Apple is the viewer's)")
	}
	if ss.Price == nil || *ss.Price != 42 {
		t.Errorf("Short Sword price = %v, want 42 (copied from the representative row)", ss.Price)
	}
	if ss.IconID != 7 || ss.Statsblock != "DMG: 5" {
		t.Errorf("Short Sword icon/stats = {%d, %q}, want 7 / DMG: 5 (keyed by representative id 100)", ss.IconID, ss.Statsblock)
	}
	// Holder bank flag comes from the roster (Banktoon is_bank_toon).
	for _, h := range ss.Holders {
		if h.Char == "Banktoon" && !h.IsBank {
			t.Errorf("Banktoon holder IsBank = false, want true (is_bank_toon)")
		}
	}

	cap := byName["Cloth Cap"]
	if !cap.IsMine {
		t.Errorf("Cloth Cap is_mine = false, want true (held only by Apple)")
	}
	if cap.Price != nil {
		t.Errorf("Cloth Cap price = %v, want nil (no price on the ViewRow)", cap.Price)
	}
	if len(cap.Holders) != 1 || cap.Holders[0].SlotLabel != "General · General3" {
		t.Errorf("Cloth Cap holder = %+v, want one General · General3", cap.Holders)
	}
}

// TestSlotLabel proves the UI-SPEC §F label mapping over the canonical strings
// classifySlot actually returns: equipment → "Worn · {Slot}", general → "General · {Slot}",
// bank → "Bank · {Slot}", and a "*-Slot<N>" bagged copy → "Bag" (A2 — the parent bag name
// is not joined).
func TestSlotLabel(t *testing.T) {
	tests := []struct {
		location string
		want     string
	}{
		{"Primary", "Worn · Primary"},
		{"Head", "Worn · Head"},
		{"General4", "General · General4"},
		{"Bank1", "Bank · Bank1"},
		{"General4-Slot1", "Bag"}, // bagged copy
		{"Bank2-Slot3", "Bag"},    // bagged copy in a bank bag
	}
	for _, tc := range tests {
		if got := slotLabel(tc.location); got != tc.want {
			t.Errorf("slotLabel(%q) = %q, want %q", tc.location, got, tc.want)
		}
	}
}
