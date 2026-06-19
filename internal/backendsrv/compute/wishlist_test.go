package compute

import (
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// wishlist_test.go covers the Phase 34 pure compute helpers (34-01 Task 3) — all
// DB-free over buildWishlistView / wikiSlotFor (the StructuredInventory testability
// split): Behavior 1 the slot-vocabulary bridge (HIGHEST RISK — Pitfall 2; Finger1 AND
// Finger2 BOTH → "Fingers"); Behavior 2 the class+slot suggestion filter incl. the
// IsRaid-is-tier tag; Behavior 3 the D-02 auto-removal name join (a held target is
// HIDDEN, not deleted); Behavior 4 the name-keyed target price over the FULL catalog
// (WARNING-3 — a catalog-only priced item resolves a price, not just a gear-tier match).

// ── Behavior 1: the slot-vocabulary bridge (HIGHEST RISK, Pitfall 2) ───────────────
func TestWikiSlotFor_CanonicalToWikiBridge(t *testing.T) {
	cases := []struct {
		canonical string
		want      string
	}{
		// The load-bearing pair-slot collapse: BOTH numbered slots → the one wiki slot.
		{"Finger1", "Fingers"},
		{"Finger2", "Fingers"},
		{"Ear1", "Ears"},
		{"Ear2", "Ears"},
		{"Wrist1", "Wrists"},
		{"Wrist2", "Wrists"},
		// Singletons map 1:1.
		{"Head", "Head"},
		{"Primary", "Primary"},
		{"Chest", "Chest"},
		// No gear-tier vocabulary → "" (empty suggestion list, A5).
		{"Ammo", ""},
		{"Charm", ""},
		{"Power", ""},
	}
	for _, c := range cases {
		if got := wikiSlotFor(c.canonical); got != c.want {
			t.Errorf("wikiSlotFor(%q) = %q, want %q", c.canonical, got, c.want)
		}
	}
}

// buildSlotIndex finds the WishlistSlot for a canonical slot in a built view.
func slotOf(t *testing.T, v WishlistView, canonical string) WishlistSlot {
	t.Helper()
	for _, s := range v.Slots {
		if s.Slot == canonical {
			return s
		}
	}
	t.Fatalf("slot %q not found in view (have %d slots)", canonical, len(v.Slots))
	return WishlistSlot{}
}

// ── Behavior 2: suggestions filtered by class+slot, IsRaid is the tier ─────────────
func TestBuildWishlistView_SuggestionsByClassAndSlotWithRaidTag(t *testing.T) {
	inv := CharacterInventory{Char: "Slampeach"}
	tiers := []store.GearTierPriceRow{
		// WAR / Fingers: one pre-raid (buyable, priced) + one raiding (Raid tag).
		{Tier: "Velious Pre-Raid/Group", Class: "WAR", Slot: "Fingers", ItemName: "Ring of Dain", Direction: "0", A30: 1200, T30: 5, HasPrice: true, LastListed: "2026-05-09"},
		{Tier: "Velious Raiding", Class: "WAR", Slot: "Fingers", ItemName: "Ring of Narandi", HasPrice: false},
		// A DIFFERENT class for the same slot — must be filtered out.
		{Tier: "Velious Raiding", Class: "CLR", Slot: "Fingers", ItemName: "Cleric Ring", HasPrice: false},
		// A DIFFERENT slot for the same class — must be filtered out of Finger1.
		{Tier: "Velious Raiding", Class: "WAR", Slot: "Head", ItemName: "Crown of Narandi", HasPrice: false},
	}

	v := buildWishlistView("Slampeach", inv, nil, tiers, nil, nil, "WAR")

	// Finger1 AND Finger2 BOTH get the two WAR/Fingers suggestions (the bridge collapse).
	for _, slot := range []string{"Finger1", "Finger2"} {
		s := slotOf(t, v, slot)
		if len(s.Suggestions) != 2 {
			t.Fatalf("%s suggestions len = %d, want 2 (the two WAR/Fingers rows; have: %+v)", slot, len(s.Suggestions), s.Suggestions)
		}
		byName := map[string]WishlistSuggestion{}
		for _, sug := range s.Suggestions {
			byName[sug.ItemName] = sug
		}
		preRaid := byName["Ring of Dain"]
		if preRaid.IsRaid {
			t.Errorf("%s Ring of Dain IsRaid = true, want false (Pre-Raid/Group tier)", slot)
		}
		if preRaid.Price == nil || *preRaid.Price != 1200 {
			t.Errorf("%s Ring of Dain Price = %v, want 1200 (gear-tier name-keyed price)", slot, preRaid.Price)
		}
		if preRaid.LastListed != "2026-05-09" {
			t.Errorf("%s Ring of Dain LastListed = %q, want 2026-05-09", slot, preRaid.LastListed)
		}
		raid := byName["Ring of Narandi"]
		if !raid.IsRaid {
			t.Errorf("%s Ring of Narandi IsRaid = false, want true (Velious Raiding tier ⇒ Raid tag)", slot)
		}
		if raid.Price != nil {
			t.Errorf("%s Ring of Narandi Price = %v, want nil (HasPrice false ⇒ no price / not for sale)", slot, raid.Price)
		}
	}

	// Head gets ONLY the WAR/Head row; the CLR row never appears anywhere.
	head := slotOf(t, v, "Head")
	if len(head.Suggestions) != 1 || head.Suggestions[0].ItemName != "Crown of Narandi" {
		t.Fatalf("Head suggestions = %+v, want exactly [Crown of Narandi]", head.Suggestions)
	}

	// Ammo has no wiki gear-tier slot → empty suggestions (A5).
	if ammo := slotOf(t, v, "Ammo"); len(ammo.Suggestions) != 0 {
		t.Errorf("Ammo suggestions len = %d, want 0 (no gear-tier vocabulary)", len(ammo.Suggestions))
	}
}

// ── Behavior 3: auto-removal (D-02) — a held target is HIDDEN, not deleted ──────────
func TestBuildWishlistView_AutoRemovalHidesHeldTargets(t *testing.T) {
	// The char HOLDS "Cloak of Flames" (in a bag on Bank, to prove hold-ANYWHERE) and
	// has "Stein of Moggok" equipped in Primary. It does NOT hold "Singing Steel Breastplate".
	inv := CharacterInventory{
		Char: "Slampeach",
		Equipment: []InventorySlot{
			{CanonicalSlot: "Primary", Item: "Stein of Moggok"},
			{CanonicalSlot: "Back", Item: ""}, // empty back slot
		},
		Bank: []InventorySlot{
			{CanonicalSlot: "Bank1", Item: "Large Bag", Children: []InventorySlot{
				{Item: "Cloak of Flames"},
			}},
		},
	}
	targets := []store.WishlistTargetRow{
		// Back slot wishlist: one held (Cloak of Flames, in a bag) + one not-held.
		{ID: 1, ItemName: "Cloak of Flames", Slot: "Back"},
		{ID: 2, ItemName: "Singing Steel Breastplate", Slot: "Back"},
		// Primary slot: the equipped item itself is wishlisted (held → hidden).
		{ID: 3, ItemName: "Stein of Moggok", Slot: "Primary"},
	}

	v := buildWishlistView("Slampeach", inv, targets, nil, nil, nil, "WAR")

	back := slotOf(t, v, "Back")
	if len(back.Targets) != 1 {
		t.Fatalf("Back targets len = %d, want 1 (Cloak of Flames held-in-bag is HIDDEN; have: %+v)", len(back.Targets), back.Targets)
	}
	if back.Targets[0].ItemName != "Singing Steel Breastplate" {
		t.Errorf("Back surviving target = %q, want Singing Steel Breastplate", back.Targets[0].ItemName)
	}

	primary := slotOf(t, v, "Primary")
	if len(primary.Targets) != 0 {
		t.Errorf("Primary targets len = %d, want 0 (the equipped Stein of Moggok auto-hides)", len(primary.Targets))
	}
}

// ── Behavior 4: target price by NAME over the FULL catalog (WARNING-3) ──────────────
func TestBuildWishlistView_TargetPriceByCatalogName(t *testing.T) {
	inv := CharacterInventory{Char: "Slampeach"} // holds nothing → no auto-removal
	targets := []store.WishlistTargetRow{
		{ID: 10, ItemName: "Fungi Tunic", Slot: "Chest", Pinged: true},   // priced in the catalog
		{ID: 11, ItemName: "Bespoke Custom Thing", Slot: "Chest"},        // no catalog match
	}
	// The catalog price map keyed by NORMALIZED name — note "Fungi Tunic" is NOT a
	// gear-tier row here (tiers is nil), proving the price comes from the full catalog,
	// not a gear-tier match (the WARNING-3 regression).
	prices := map[string]store.PriceByNameRow{
		"fungi tunic": {Direction: "0", A30: 50000, T30: 20, LastListed: "2026-05-09", HasPrice: true},
	}
	alerted := map[int64]bool{10: true} // EC-hit badge on the Fungi Tunic target

	v := buildWishlistView("Slampeach", inv, targets, nil, prices, alerted, "WAR")

	chest := slotOf(t, v, "Chest")
	if len(chest.Targets) != 2 {
		t.Fatalf("Chest targets len = %d, want 2; have: %+v", len(chest.Targets), chest.Targets)
	}
	byName := map[string]WishlistTarget{}
	for _, tg := range chest.Targets {
		byName[tg.ItemName] = tg
	}

	fungi := byName["Fungi Tunic"]
	if fungi.Price == nil || *fungi.Price != 50000 {
		t.Errorf("Fungi Tunic Price = %v, want 50000 (resolved from the FULL catalog by name, not a gear-tier row)", fungi.Price)
	}
	if fungi.LastListed != "2026-05-09" {
		t.Errorf("Fungi Tunic LastListed = %q, want 2026-05-09 (catalog last_seen)", fungi.LastListed)
	}
	if !fungi.PingedHit {
		t.Errorf("Fungi Tunic PingedHit = false, want true (alert_log row exists, id 10)")
	}
	if !fungi.Pinged {
		t.Errorf("Fungi Tunic Pinged = false, want true (ping toggle ON)")
	}

	custom := byName["Bespoke Custom Thing"]
	if custom.Price != nil {
		t.Errorf("Bespoke Custom Thing Price = %v, want nil (no catalog match ⇒ genuinely unpriced)", custom.Price)
	}
	if custom.LastListed != "" {
		t.Errorf("Bespoke Custom Thing LastListed = %q, want \"\" (no catalog match)", custom.LastListed)
	}
	if custom.PingedHit {
		t.Errorf("Bespoke Custom Thing PingedHit = true, want false (no alert_log row)")
	}
}

// TestBuildWishlistView_AllWornSlotsIncludingEmpty proves D-04: all 21 worn slots
// appear (Charm/Power omitted), even an empty one, with the equipped item per slot.
func TestBuildWishlistView_AllWornSlotsIncludingEmpty(t *testing.T) {
	inv := CharacterInventory{
		Char:      "Slampeach",
		Equipment: []InventorySlot{{CanonicalSlot: "Head", Item: "Helm"}},
	}
	v := buildWishlistView("Slampeach", inv, nil, nil, nil, nil, "WAR")

	if len(v.Slots) != 21 {
		t.Fatalf("view slot count = %d, want 21 (the worn-slot taxonomy, Charm/Power omitted)", len(v.Slots))
	}
	if got := slotOf(t, v, "Head").Equipped; got != "Helm" {
		t.Errorf("Head equipped = %q, want Helm", got)
	}
	// An unfilled slot is present with Equipped "" (D-04 — you can wishlist an empty slot).
	if got := slotOf(t, v, "Waist").Equipped; got != "" {
		t.Errorf("Waist equipped = %q, want \"\" (empty slot still rendered)", got)
	}
	// Charm/Power are NOT in the worn-slot set.
	for _, s := range v.Slots {
		if s.Slot == "Charm" || s.Slot == "Power" {
			t.Errorf("unexpected slot %q in the view (Charm/Power omitted, D-04)", s.Slot)
		}
	}
}
