package compute_test

// inventory_test.go is the INV-05 / DATA-01 / DATA-02 parity suite for the Phase 29
// compute transforms (StructuredInventory / BankValuation / TotalPlatinum), proven over
// the real-name nested-bag fixture (testdata/Slampeach-Inventory.txt, from Plan 29-01)
// + targeted seeds. It mirrors bank_test.go's external-package scaffold: newTestDB(t) →
// store.NewStore(db) → seed via the shared fixtures_test.go helpers → call the compute
// fn → assert the shaped result.

import (
	"context"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// findSlot returns the first InventorySlot at the given raw Location across the three
// groups (Equipment/General/Bank), or nil — a test helper for assertions.
func findSlot(inv compute.CharacterInventory, location string) *compute.InventorySlot {
	for _, group := range [][]compute.InventorySlot{inv.Equipment, inv.General, inv.Bank} {
		for i := range group {
			if group[i].Location == location {
				return &group[i]
			}
		}
	}
	return nil
}

// TestStructuredInventory_Classify seeds a Head (equipment), a General4 (general), and a
// Bank1 (bank) and asserts each lands in the right group with the right CanonicalSlot.
// The classifier is case-INSENSITIVE (A5 belt-and-suspenders): it is robust whether live
// inventory_item.location is Title- or upper-case, so an on-box `SELECT DISTINCT location
// FROM inventory_item LIMIT 40` mismatch (Title vs upper) does not break it — the canonical
// OUTPUT key is always Title-case (covered explicitly by TestClassifySlot's "HEAD" case).
func TestStructuredInventory_Classify(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	seedInvFull(t, db, char, "Head", "Crown of Narandi", 2050, 1, 0, 1)
	seedInvFull(t, db, char, "General4", "Large Bag", 1038, 1, 10, 2)
	seedInvFull(t, db, char, "Bank1", "Bag of Holding", 1039, 1, 8, 3)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	if len(inv.Equipment) != 1 || inv.Equipment[0].CanonicalSlot != "Head" {
		t.Errorf("Equipment = %+v, want one slot canonical Head", inv.Equipment)
	}
	if len(inv.General) != 1 || inv.General[0].CanonicalSlot != "General4" {
		t.Errorf("General = %+v, want one slot canonical General4", inv.General)
	}
	if len(inv.Bank) != 1 || inv.Bank[0].CanonicalSlot != "Bank1" {
		t.Errorf("Bank = %+v, want one slot canonical Bank1", inv.Bank)
	}
}

// TestStructuredInventory_Nesting loads the real-name fixture and asserts the one-level
// bag nesting: General4 contains Diamond (count 5) + Black Pearl; Bank1 contains its
// children; no top-level slot has a "-Slot" Location (children nested, augment
// flattened); the count=5 child keeps count 5.
func TestStructuredInventory_Nesting(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	loadInventoryFixture(t, db, char, "Slampeach-Inventory.txt")

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	// General4 (Large Bag) nests its 3 children; Diamond keeps count 5.
	gen4 := findSlot(inv, "General4")
	if gen4 == nil {
		t.Fatalf("General4 container not found in General group: %+v", inv.General)
	}
	if len(gen4.Children) != 3 {
		t.Fatalf("General4 children = %d, want 3 (Diamond, Black Pearl, Words of the Spoken): %+v", len(gen4.Children), gen4.Children)
	}
	var diamond *compute.InventorySlot
	for i := range gen4.Children {
		if gen4.Children[i].Item == "Diamond" {
			diamond = &gen4.Children[i]
		}
	}
	if diamond == nil {
		t.Fatalf("Diamond not nested under General4: %+v", gen4.Children)
	}
	if diamond.Count != 5 {
		t.Errorf("nested Diamond count = %d, want 5 (stack count preserved)", diamond.Count)
	}

	// Bank1 (Bag of Holding) nests its 2 children.
	bank1 := findSlot(inv, "Bank1")
	if bank1 == nil {
		t.Fatalf("Bank1 container not found in Bank group: %+v", inv.Bank)
	}
	if len(bank1.Children) != 2 {
		t.Errorf("Bank1 children = %d, want 2 (Words of the Spoken, Rough Diamond): %+v", len(bank1.Children), bank1.Children)
	}

	// NO top-level slot is a "-Slot" child or the Head-Slot1 augment — children are
	// nested under their parent, the augment is flattened (A3).
	for _, slot := range []string{"General4-Slot1", "Bank1-Slot1", "Head-Slot1"} {
		if findSlot(inv, slot) != nil {
			t.Errorf("%q surfaced as a top-level slot, want nested/flattened", slot)
		}
	}
}

// TestNameJoin_HitMiss proves the DATA-01 inventory price join: a priced item resolves
// its price by NORMALIZED NAME (catalog id != EQ id), an unpriced item has Price nil.
func TestNameJoin_HitMiss(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	// HIT: inventory holds EQ id 14536; the catalog row for the same name has a
	// DIFFERENT id 19450 — the name bridge attaches the price.
	seedInvFull(t, db, char, "General1", "10 Dose Ant's Potion", 14536, 1, 0, 1)
	seedPigparse(t, db, 19450, "10 Dose Ant's Potion", "0", 320, 12)
	// MISS: no pigparse row.
	seedInvFull(t, db, char, "General2", "Worthless Trinket", 9997, 1, 0, 2)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	hit := findSlot(inv, "General1")
	if hit == nil || hit.Price == nil || *hit.Price != 320 {
		t.Errorf("Ant's Potion Price = %v, want *320 (name-bridged 14536↔19450)", priceVal(hit))
	}
	miss := findSlot(inv, "General2")
	if miss == nil || miss.Price != nil {
		t.Errorf("Worthless Trinket Price = %v, want nil (no pigparse row)", priceVal(miss))
	}
}

// TestLastListed_NotCharFreshness proves Pitfall 2: a priced slot's LastListed equals the
// pigparse last_seen (last-listed-for-sale) and differs from the char's upload freshness.
// A7 belt-and-suspenders: pigparse_price.last_seen is the daily-getall last-listed-for-sale
// date (post-WTS-filter), surfaced here verbatim as the ISO string LastListed; an on-box
// `SELECT name,last_seen FROM pigparse_price LIMIT 5` confirms the semantics but does not
// gate this test (the value is carried through, not interpreted).
func TestLastListed_NotCharFreshness(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	seedInvFull(t, db, char, "Head", "Crown of Narandi", 2050, 1, 0, 1)
	seedPigparse(t, db, 2050, "Crown of Narandi", "0", 4500, 75) // writes last_seen="2026-05-09"

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}
	slot := findSlot(inv, "Head")
	if slot == nil {
		t.Fatalf("Head slot not found")
	}
	if slot.LastListed != "2026-05-09" {
		t.Errorf("LastListed = %q, want %q (pigparse_price.last_seen — last-listed-for-sale)", slot.LastListed, "2026-05-09")
	}
	// seedChar stamps character.last_seen="2026-05-09T00:00:00Z" — the DISTINCT upload
	// freshness value; LastListed must not equal it (the two last_seen columns crossed).
	if slot.LastListed == "2026-05-09T00:00:00Z" {
		t.Errorf("LastListed (%q) equals the char upload-freshness timestamp — the two last_seen columns were crossed", slot.LastListed)
	}
}

// TestBankValuation_SumAndUnpriced seeds a bank toon with 2 priced items (100×2 + 50×1)
// + 1 unpriced; asserts GuildTotal.TotalValue == 250 and UnpricedCount == 1.
func TestBankValuation_SumAndUnpriced(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	bank := seedChar(t, db, "owner-b", "Guildbank", "WAR", 60, "HUM", true)
	seedInvFull(t, db, bank, "Bank1", "Gem A", 5001, 2, 0, 1) // 100 × 2 = 200
	seedInvFull(t, db, bank, "Bank2", "Gem B", 5002, 1, 0, 2) // 50 × 1 = 50
	seedInvFull(t, db, bank, "Bank3", "Junk", 5003, 1, 0, 3)  // unpriced
	seedPigparse(t, db, 5001, "Gem A", "0", 100, 10)
	seedPigparse(t, db, 5002, "Gem B", "0", 50, 5)

	bv, err := compute.BankValuationFor(ctx, s)
	if err != nil {
		t.Fatalf("BankValuation: %v", err)
	}
	if bv.GuildTotal.TotalValue != 250 {
		t.Errorf("GuildTotal.TotalValue = %v, want 250 (100×2 + 50×1)", bv.GuildTotal.TotalValue)
	}
	if bv.GuildTotal.UnpricedCount != 1 {
		t.Errorf("GuildTotal.UnpricedCount = %d, want 1 (the Junk item)", bv.GuildTotal.UnpricedCount)
	}
	if v, ok := bv.PerBank["Guildbank"]; !ok || v.TotalValue != 250 {
		t.Errorf("PerBank[Guildbank] = %+v, want TotalValue 250", v)
	}
}

// TestBankValuation_CountsBagContents proves Pitfall 3 / D-02: a bank container (priced)
// AND its nested child (priced) BOTH contribute to the valuation (flat sum over all bank
// rows, never a tree-walk).
func TestBankValuation_CountsBagContents(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	bank := seedChar(t, db, "owner-b", "Guildbank", "WAR", 60, "HUM", true)
	seedInvFull(t, db, bank, "Bank1", "Large Bag", 1038, 1, 10, 1)    // container, priced 30
	seedInvFull(t, db, bank, "Bank1-Slot1", "Diamond", 1071, 5, 0, 2) // nested child, priced 40 × 5
	seedPigparse(t, db, 1038, "Large Bag", "0", 30, 4)
	seedPigparse(t, db, 1071, "Diamond", "0", 40, 9)

	bv, err := compute.BankValuationFor(ctx, s)
	if err != nil {
		t.Fatalf("BankValuation: %v", err)
	}
	// 30 (the bag itself, D-02) + 40×5 (its contents) = 230 — both count.
	if bv.GuildTotal.TotalValue != 230 {
		t.Errorf("GuildTotal.TotalValue = %v, want 230 (bag 30 + contents 40×5 — bag AND contents both count)", bv.GuildTotal.TotalValue)
	}
}

// TestTotalPlatinum_LiteralPlatOnly seeds two bank toons (plat=1000 gold=9999, plat nil)
// + a non-bank char with plat; asserts TotalPlatinum == 1000 (gold excluded, nil skipped,
// non-bank char excluded by ListBankToons scope).
func TestTotalPlatinum_LiteralPlatOnly(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	bankA := seedChar(t, db, "owner-a", "BankA", "WAR", 60, "HUM", true)
	bankB := seedChar(t, db, "owner-b", "BankB", "ENC", 60, "HUM", true)
	nonBank := seedChar(t, db, "owner-c", "Regular", "NEC", 60, "HUM", false)
	// BankA: plat=1000, gold=9999 (gold must NOT roll into plat).
	if _, err := db.Exec(`UPDATE character SET plat=1000, gold=9999 WHERE id=?`, bankA); err != nil {
		t.Fatalf("set BankA coin: %v", err)
	}
	// BankB: plat NULL (never entered — must be skipped, not treated as 0).
	_ = bankB
	// Regular: plat=500 but NOT a bank toon — excluded by ListBankToons scope.
	if _, err := db.Exec(`UPDATE character SET plat=500 WHERE id=?`, nonBank); err != nil {
		t.Fatalf("set Regular coin: %v", err)
	}

	toons, err := store.ListBankToons(ctx, s.DB())
	if err != nil {
		t.Fatalf("ListBankToons: %v", err)
	}
	total := compute.TotalPlatinum(toons)
	if total != 1000 {
		t.Errorf("TotalPlatinum = %d, want 1000 (gold 9999 excluded, nil-plat skipped, non-bank 500 excluded)", total)
	}
}

// priceVal renders a slot's *Price for error messages without panicking on nil.
func priceVal(s *compute.InventorySlot) interface{} {
	if s == nil || s.Price == nil {
		return nil
	}
	return *s.Price
}
