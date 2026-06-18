package compute_test

// inventory_test.go is the INV-05 / DATA-01 / DATA-02 parity suite for the Phase 29
// compute transforms (StructuredInventory / BankValuation / TotalPlatinum), proven over
// the real-name nested-bag fixture (testdata/Slampeach-Inventory.txt, from Plan 29-01)
// + targeted seeds. It mirrors bank_test.go's external-package scaffold: newTestDB(t) →
// store.NewStore(db) → seed via the shared fixtures_test.go helpers → call the compute
// fn → assert the shaped result.

import (
	"context"
	"database/sql"
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

// TestStructuredInventory_OrphanBeforeContainer is the CR-01 regression: an orphan
// (or grandchild) row that precedes a REAL container's child in row_ordinal order must
// NOT cause the container to lose its children. The orphan branch used to append to
// inv.General / inv.Bank — the same backing arrays parentRef points into — so a realloc
// dangled every parent pointer and a later parent.Children write landed in the stale
// (orphaned) array, vanishing from the returned inventory. We seed many top-level General
// slots so the orphan append is guaranteed to grow inv.General past its Pass-A capacity,
// then assert the real container keeps its nested child.
func TestStructuredInventory_OrphanBeforeContainer(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)

	// ordinal 1: an ORPHAN general child — its parent container (General9) is never a
	// top-level slot, so it takes the orphan branch and appends to inv.General. That
	// append must reallocate inv.General's backing array to dangle parentRef, which
	// requires Pass A to leave inv.General at len==cap. Go's append growth lands cap at
	// {1,4,8,16,...}; seeding exactly 4 real top-level General containers below makes
	// Pass A finish at len 4 == cap 4, so this orphan append (the 5th element) reallocs.
	seedInvFull(t, db, char, "General9-Slot1", "Orphaned Gem", 9990, 1, 0, 1)
	// ordinals 2..5: exactly 4 real top-level General containers (len==cap after Pass A).
	seedInvFull(t, db, char, "General1", "Large Bag", 1038, 1, 10, 2)
	seedInvFull(t, db, char, "General2", "Small Bag", 1037, 1, 6, 3)
	seedInvFull(t, db, char, "General3", "Pouch", 1036, 1, 4, 4)
	seedInvFull(t, db, char, "General5", "Sack", 1034, 1, 4, 5)
	// ordinal 6: General1's REAL nested child — appears AFTER the orphan in row order.
	seedInvFull(t, db, char, "General1-Slot1", "Diamond", 1071, 1, 0, 6)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	gen1 := findSlot(inv, "General1")
	if gen1 == nil {
		t.Fatalf("General1 container not found in General group: %+v", inv.General)
	}
	if len(gen1.Children) != 1 {
		t.Fatalf("General1 children = %d, want 1 (Diamond) — orphan-before-container dropped the child (CR-01 dangling pointer): %+v", len(gen1.Children), gen1.Children)
	}
	if gen1.Children[0].Item != "Diamond" {
		t.Errorf("General1 child = %q, want Diamond", gen1.Children[0].Item)
	}
	// The orphan itself is flattened to a top-level General slot (not dropped).
	if findSlot(inv, "General9-Slot1") == nil {
		t.Errorf("orphan General9-Slot1 missing — it must flatten to top-level, not vanish")
	}
}

// TestStructuredInventory_OrphanBeforeContainer_Bank is the CR-01 regression on the Bank
// group: same dangling-pointer hazard, but the orphan/realloc happens on inv.Bank.
func TestStructuredInventory_OrphanBeforeContainer_Bank(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)

	// ordinal 1: an orphan bank child (Bank9 is never a top-level slot). Same len==cap
	// realloc requirement as the General case — seed exactly 4 real Bank containers below.
	seedInvFull(t, db, char, "Bank9-Slot1", "Orphaned Gem", 9991, 1, 0, 1)
	// ordinals 2..5: exactly 4 real top-level Bank containers (len==cap after Pass A).
	seedInvFull(t, db, char, "Bank1", "Bag of Holding", 1039, 1, 8, 2)
	seedInvFull(t, db, char, "Bank2", "Small Bag", 1037, 1, 6, 3)
	seedInvFull(t, db, char, "Bank3", "Pouch", 1036, 1, 4, 4)
	seedInvFull(t, db, char, "Bank5", "Sack", 1034, 1, 4, 5)
	// ordinal 6: Bank1's REAL nested child, AFTER the orphan in row order.
	seedInvFull(t, db, char, "Bank1-Slot1", "Rough Diamond", 7002, 3, 0, 6)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	bank1 := findSlot(inv, "Bank1")
	if bank1 == nil {
		t.Fatalf("Bank1 container not found in Bank group: %+v", inv.Bank)
	}
	if len(bank1.Children) != 1 {
		t.Fatalf("Bank1 children = %d, want 1 (Rough Diamond) — orphan-before-container dropped the child (CR-01): %+v", len(bank1.Children), bank1.Children)
	}
}

// TestStructuredInventory_UppercaseNesting is the WR-01 regression: uppercase live data
// (GENERAL4, GENERAL4-SLOT1, BANK1-SLOT1) must still nest children under their container
// and emit Title-case canonical keys. The sub-slot regex was case-SENSITIVE (^Slot\d+$)
// while the container regexes carried (?i), so "GENERAL4-SLOT1" failed splitChild and
// surfaced as a phantom second top-level "General4" instead of nesting (defeating A5).
func TestStructuredInventory_UppercaseNesting(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	seedInvFull(t, db, char, "GENERAL4", "Large Bag", 1038, 1, 10, 1)
	seedInvFull(t, db, char, "GENERAL4-SLOT1", "Diamond", 1071, 5, 0, 2)
	seedInvFull(t, db, char, "BANK1", "Bag of Holding", 1039, 1, 8, 3)
	seedInvFull(t, db, char, "BANK1-SLOT1", "Rough Diamond", 7002, 3, 0, 4)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	// Exactly ONE top-level General slot (the container) — no phantom GENERAL4-SLOT1.
	if len(inv.General) != 1 {
		t.Fatalf("General group = %d top-level slots, want 1 (uppercase child must nest, not surface): %+v", len(inv.General), inv.General)
	}
	gen4 := findSlot(inv, "GENERAL4")
	if gen4 == nil {
		t.Fatalf("GENERAL4 container not found: %+v", inv.General)
	}
	if gen4.CanonicalSlot != "General4" {
		t.Errorf("GENERAL4 CanonicalSlot = %q, want Title-case General4 (A5)", gen4.CanonicalSlot)
	}
	if len(gen4.Children) != 1 || gen4.Children[0].Item != "Diamond" {
		t.Fatalf("GENERAL4 children = %+v, want 1 (Diamond) nested (WR-01 case-sensitive sub-slot regex)", gen4.Children)
	}

	// Same for the bank: one top-level container, the uppercase child nested.
	if len(inv.Bank) != 1 {
		t.Fatalf("Bank group = %d top-level slots, want 1: %+v", len(inv.Bank), inv.Bank)
	}
	bank1 := findSlot(inv, "BANK1")
	if bank1 == nil {
		t.Fatalf("BANK1 container not found: %+v", inv.Bank)
	}
	if bank1.CanonicalSlot != "Bank1" {
		t.Errorf("BANK1 CanonicalSlot = %q, want Title-case Bank1 (A5)", bank1.CanonicalSlot)
	}
	if len(bank1.Children) != 1 || bank1.Children[0].Item != "Rough Diamond" {
		t.Fatalf("BANK1 children = %+v, want 1 (Rough Diamond) nested (WR-01)", bank1.Children)
	}
}

// TestBankValuation_CoinOnlyBankToon is the MR-02 regression: a bank toon with platinum
// entered but NO inventory_item rows must still appear in PerBank (zero Valuation), so the
// per-bank breakdown is complete and matches the TotalPlatinum it contributes to.
func TestBankValuation_CoinOnlyBankToon(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	// A bank toon with items + price.
	withItems := seedChar(t, db, "owner-a", "Guildbank", "WAR", 60, "HUM", true)
	seedInvFull(t, db, withItems, "Bank1", "Gem A", 5001, 1, 0, 1) // priced 100
	seedPigparse(t, db, 5001, "Gem A", "0", 100, 10)
	// A coin-only bank toon: platinum entered, NO inventory_item rows at all.
	coinOnly := seedChar(t, db, "owner-b", "Vault", "ENC", 60, "HUM", true)
	if _, err := db.Exec(`UPDATE character SET plat=777 WHERE id=?`, coinOnly); err != nil {
		t.Fatalf("set Vault plat: %v", err)
	}

	bv, err := compute.BankValuationFor(ctx, s)
	if err != nil {
		t.Fatalf("BankValuation: %v", err)
	}

	// The coin-only toon must have a PerBank entry (zero value), not be omitted.
	v, ok := bv.PerBank["Vault"]
	if !ok {
		t.Fatalf("PerBank missing the coin-only bank toon Vault — its platinum is in TotalPlatinum but its per-bank row vanished (MR-02): %+v", bv.PerBank)
	}
	if v.TotalValue != 0 || v.UnpricedCount != 0 {
		t.Errorf("PerBank[Vault] = %+v, want zero Valuation (no items)", v)
	}
	// Its platinum is still counted in the total.
	if bv.TotalPlatinum != 777 {
		t.Errorf("TotalPlatinum = %d, want 777 (coin-only toon's plat)", bv.TotalPlatinum)
	}
	// The toon with items is unaffected.
	if v := bv.PerBank["Guildbank"]; v.TotalValue != 100 {
		t.Errorf("PerBank[Guildbank].TotalValue = %v, want 100", v.TotalValue)
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

// TestStructuredInventory_IconID proves INV-04 end-to-end through the store→compute
// seam: an item whose item_master row carries icon_id N surfaces slot.IconID == N on
// the matching slot (the EQ-namespace im.item_id = ii.item_id join, Pitfall 3 — NOT a
// name join), while an item whose item_master has a NULL icon_id (or no item_master row
// at all) surfaces IconID == 0 (the no-icon sentinel → colored-tile fallback, D-02).
func TestStructuredInventory_IconID(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	// HIT: item_master row with icon_id 658 (Cloak of Flames' real lucy_img_ID).
	seedInvFull(t, db, char, "Back", "Cloak of Flames", 2010, 1, 0, 1)
	seedItemMasterIcon(t, db, 2010, "Cloak of Flames", 658)
	// NULL icon_id: an item_master row exists but icon_id was never enriched.
	seedInvFull(t, db, char, "Head", "Helm of Rile", 2011, 1, 0, 2)
	seedItemMaster(t, db, 2011, "Helm of Rile", "A helm.", "http://wiki/Helm", false) // no icon_id → NULL
	// NO item_master row at all (LEFT JOIN miss).
	seedInvFull(t, db, char, "Hands", "Mystery Gauntlets", 2012, 1, 0, 3)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	hit := findSlot(inv, "Back")
	if hit == nil {
		t.Fatalf("Back slot not found: %+v", inv.Equipment)
	}
	if hit.IconID != 658 {
		t.Errorf("Cloak of Flames IconID = %d, want 658 (item_master.icon_id, id-joined)", hit.IconID)
	}
	nullIcon := findSlot(inv, "Head")
	if nullIcon == nil {
		t.Fatalf("Head slot not found")
	}
	if nullIcon.IconID != 0 {
		t.Errorf("NULL-icon item IconID = %d, want 0 (no-icon sentinel)", nullIcon.IconID)
	}
	noMaster := findSlot(inv, "Hands")
	if noMaster == nil {
		t.Fatalf("Hands slot not found")
	}
	if noMaster.IconID != 0 {
		t.Errorf("no-item_master item IconID = %d, want 0 (LEFT JOIN miss → sentinel)", noMaster.IconID)
	}
}

// TestStructuredInventory_LastSeen proves the examine "Last synced" carrier (D-08 #12 /
// Pitfall 2): CharacterInventory.LastSeen equals the per-CHARACTER character.last_seen
// (the same value on every row), and is DISTINCT from a slot's per-item LastListed (the
// price last-listed-for-sale date). seedChar stamps character.last_seen=2026-05-09T00:00:00Z;
// seedPigparse writes pigparse_price.last_seen=2026-05-09 (the LastListed value).
func TestStructuredInventory_LastSeen(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	seedInvFull(t, db, char, "Head", "Crown of Narandi", 2050, 1, 0, 1)
	seedPigparse(t, db, 2050, "Crown of Narandi", "0", 4500, 75) // last_seen=2026-05-09 (LastListed)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	// LastSeen is the per-character upload freshness (character.last_seen).
	if inv.LastSeen != "2026-05-09T00:00:00Z" {
		t.Errorf("CharacterInventory.LastSeen = %q, want %q (character.last_seen — upload freshness)", inv.LastSeen, "2026-05-09T00:00:00Z")
	}
	// It must NOT be aliased to the per-slot LastListed (the price last-listed date).
	slot := findSlot(inv, "Head")
	if slot == nil {
		t.Fatalf("Head slot not found")
	}
	if inv.LastSeen == slot.LastListed {
		t.Errorf("CharacterInventory.LastSeen (%q) equals the per-slot LastListed (%q) — the two last_seen sources were crossed (Pitfall 2)", inv.LastSeen, slot.LastListed)
	}
}

// TestStructuredInventory_PairedSlots is the Phase 31 window-crash regression (2026-06-18):
// real /outputfile inventory writes the BASE token for the DOUBLED equipment slots — the
// SAME "Ear"/"Fingers"/"Wrist" for BOTH of each pair, NOT "Ear1"/"Ear2". They must classify
// as EQUIPMENT and be numbered by occurrence into Ear1/Ear2, Finger1/Finger2, Wrist1/Wrist2
// (each its own paperdoll position), instead of falling to the general default where the two
// identical Locations collided in the web's keyed grid and froze the inventory window on
// "Loading…". (The synthetic 29-fixture used the numbered tokens, so this real-data shape
// was never exercised until the live browser-smoke.)
func TestStructuredInventory_PairedSlots(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	// Both ears / both fingers / both wrists share the SAME base Location — exactly what the
	// live Slampeach dump carries (the rows that triggered each_key_duplicate).
	seedInvFull(t, db, char, "Ear", "Black Sapphire Electrum Earring", 14701, 1, 0, 1)
	seedInvFull(t, db, char, "Ear", "Black Sapphire Electrum Earring", 14701, 1, 0, 2)
	seedInvFull(t, db, char, "Fingers", "Velium Fire Wedding Ring", 30339, 1, 0, 3)
	seedInvFull(t, db, char, "Fingers", "Velium Fire Wedding Ring", 30339, 1, 0, 4)
	seedInvFull(t, db, char, "Wrist", "Bracer of Benevolence", 5301, 1, 0, 5)
	seedInvFull(t, db, char, "Wrist", "Bracer of Benevolence", 5301, 1, 0, 6)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}

	// None fall to General (that was the crash path) — all six are equipment.
	if len(inv.General) != 0 {
		t.Errorf("General = %+v, want empty (paired equipment must NOT fall to general)", inv.General)
	}
	if len(inv.Equipment) != 6 {
		t.Fatalf("Equipment = %d slots, want 6: %+v", len(inv.Equipment), inv.Equipment)
	}
	// Each pair is numbered into its two distinct canonical positions (the keys the
	// paperdoll's LEFT/RIGHT/WORN slot lists look up — Ear1/Ear2/Finger1/Finger2/Wrist1/Wrist2).
	seen := map[string]int{}
	for i := range inv.Equipment {
		seen[inv.Equipment[i].CanonicalSlot]++
	}
	for _, want := range []string{"Ear1", "Ear2", "Finger1", "Finger2", "Wrist1", "Wrist2"} {
		if seen[want] != 1 {
			t.Errorf("canonical %q appeared %d times, want exactly 1: %+v", want, seen[want], inv.Equipment)
		}
	}
}

// TestStructuredInventory_SharedBank: real /outputfile inventory writes the account-wide
// shared-bank slots as "SharedBank<N>" alongside the personal "Bank<N>". They must classify
// as bank (render in the bank section), not fall to the general default — and a nested
// shared-bank-bag child must still nest under its container.
func TestStructuredInventory_SharedBank(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	char := seedChar(t, db, "owner-a", "Slampeach", "SHM", 60, "TRL", false)
	seedInvFull(t, db, char, "SharedBank1", "Rough Diamond", 7002, 3, 0, 1)
	seedInvFull(t, db, char, "SharedBank2", "Backpack", 17005, 1, 8, 2)
	seedInvFull(t, db, char, "SharedBank2-Slot1", "Diamond", 1071, 1, 0, 3)

	inv, err := compute.StructuredInventory(ctx, s, "Slampeach")
	if err != nil {
		t.Fatalf("StructuredInventory: %v", err)
	}
	if len(inv.General) != 0 {
		t.Errorf("General = %+v, want empty (shared bank must classify as bank)", inv.General)
	}
	sb1 := findSlot(inv, "SharedBank1")
	if sb1 == nil || sb1.Category != compute.SlotBank {
		t.Errorf("SharedBank1 = %+v, want category bank", sb1)
	}
	// The nested child stays nested under SharedBank2 (not surfaced as a phantom top-level slot).
	sb2 := findSlot(inv, "SharedBank2")
	if sb2 == nil || len(sb2.Children) != 1 || sb2.Children[0].Item != "Diamond" {
		t.Errorf("SharedBank2 children = %+v, want 1 (Diamond) nested", sb2)
	}
	if findSlot(inv, "SharedBank2-Slot1") != nil {
		t.Errorf("SharedBank2-Slot1 surfaced as a top-level slot, want nested")
	}
}

// seedItemMasterIcon inserts one item_master row with an explicit icon_id (INV-04),
// so the store→compute icon flow is seedable. The other item_master columns are
// minimal — only the id-join (item_id) + icon_id matter here.
func seedItemMasterIcon(t *testing.T, db *sql.DB, itemID int64, name string, iconID int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed, icon_id)
		 VALUES (?,?,?,?,?,?,?,datetime('now'),?)`,
		itemID, name, "", "", "", 0, "sha", iconID,
	); err != nil {
		t.Fatalf("seed item_master icon (item_id=%d): %v", itemID, err)
	}
}

// priceVal renders a slot's *Price for error messages without panicking on nil.
func priceVal(s *compute.InventorySlot) interface{} {
	if s == nil || s.Price == nil {
		return nil
	}
	return *s.Price
}
