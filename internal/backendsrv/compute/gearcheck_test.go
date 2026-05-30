package compute_test

// gearcheck_test.go proves compute.GearCheck parity against the v1 vitest oracle
// (apps-script/src/__tests__/buildGearCheck.test.ts). Each subtest translates the
// corresponding v1 seed-array + expected (Char,Tier,Slot,Have,Recommended,Status)
// tuples into the equivalent DB rows over store.NewTestDB and asserts the same
// output. The v1 expected tuples are the parity bar.

import (
	"context"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// findGear returns the first row matching (char, tier, slot, recommended), or nil.
func findGear(rows []compute.GearCheckRow, char, tier, slot, rec string) *compute.GearCheckRow {
	for i := range rows {
		r := rows[i]
		if r.Char == char && r.Tier == tier && r.Slot == slot && r.Recommended == rec {
			return &rows[i]
		}
	}
	return nil
}

// v1: 'happy path: NEC HUM with 2 wiki items, has 1 → 1 OK + 1 MISSING'
func TestGearCheck_HappyPath_OKAndMissing(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	c := seedChar(t, db, "o", "Slampeach", "NEC", 60, "HUM", false)
	seedGear(t, db, "Velious Pre-Raid/Group", "NEC", "Head", "Circlet of Vallon", 1)
	seedGear(t, db, "Velious Pre-Raid/Group", "NEC", "Chest", "Robe of the Lost Circle", 1)
	seedInv(t, db, c, "HEAD", "Circlet of Vallon", 1234, 1)

	rows, err := compute.GearCheck(ctx, s)
	if err != nil {
		t.Fatalf("GearCheck: %v", err)
	}

	head := findGear(rows, "Slampeach", "Velious Pre-Raid/Group", "Head", "Circlet of Vallon")
	if head == nil || head.Status != "OK" || head.Have != "Circlet of Vallon" {
		t.Errorf("Head row = %+v, want OK / Have=Circlet of Vallon", head)
	}
	chest := findGear(rows, "Slampeach", "Velious Pre-Raid/Group", "Chest", "Robe of the Lost Circle")
	if chest == nil || chest.Status != "MISSING" || chest.Have != "" {
		t.Errorf("Chest row = %+v, want MISSING / Have=''", chest)
	}
}

// v1: 'OTHER status: char has wrong item in slot'
func TestGearCheck_Other(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	c := seedChar(t, db, "o", "Slampeach", "NEC", 60, "HUM", false)
	seedGear(t, db, "Velious Pre-Raid/Group", "NEC", "Head", "Circlet of Vallon", 1)
	seedInv(t, db, c, "HEAD", "Some Other Helm", 999, 1)

	rows, err := compute.GearCheck(ctx, s)
	if err != nil {
		t.Fatalf("GearCheck: %v", err)
	}
	head := findGear(rows, "Slampeach", "Velious Pre-Raid/Group", "Head", "Circlet of Vallon")
	if head == nil || head.Status != "OTHER" || head.Have != "Some Other Helm" {
		t.Errorf("Head row = %+v, want OTHER / Have=Some Other Helm", head)
	}
}

// v1: 'Iksar tier shown ONLY for race=IKS'
func TestGearCheck_IksarTierOnlyForIKS(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	seedChar(t, db, "o1", "IksarSk", "SHD", 60, "IKS", false)
	seedChar(t, db, "o2", "HumanSk", "SHD", 60, "HUM", false)
	seedGear(t, db, "Velious Pre-Raid/Group", "SHD", "Head", "Pre-Raid Helm", 1)
	seedGear(t, db, "Iksar", "SHD", "Head", "Iksar Hide Cap", 1)

	rows, err := compute.GearCheck(ctx, s)
	if err != nil {
		t.Fatalf("GearCheck: %v", err)
	}

	// Every Iksar-tier row must belong to IksarSk; HumanSk must have none.
	iksarCount := 0
	for _, r := range rows {
		if r.Tier == "Iksar" {
			iksarCount++
			if r.Char != "IksarSk" {
				t.Errorf("Iksar-tier row for non-IKS char: %+v", r)
			}
		}
		if r.Char == "HumanSk" && r.Tier == "Iksar" {
			t.Errorf("HumanSk must not have Iksar-tier rows: %+v", r)
		}
	}
	if iksarCount == 0 {
		t.Errorf("expected at least one Iksar-tier row for IksarSk")
	}
}

// v1: 'pair-slot match: ... recommended for Ears, char has it in EAR2 → OK'
func TestGearCheck_PairSlotMatchEAR2(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	c := seedChar(t, db, "o", "Slampeach", "MNK", 60, "HUM", false)
	seedGear(t, db, "Velious Pre-Raid/Group", "MNK", "Ears", "Fingerbone Hoop", 1)
	seedInv(t, db, c, "EAR1", "Other Earring", 1, 1)
	seedInv(t, db, c, "EAR2", "Fingerbone Hoop", 2, 2)

	rows, err := compute.GearCheck(ctx, s)
	if err != nil {
		t.Fatalf("GearCheck: %v", err)
	}
	ears := findGear(rows, "Slampeach", "Velious Pre-Raid/Group", "Ears", "Fingerbone Hoop")
	if ears == nil || ears.Status != "OK" || ears.Have != "Fingerbone Hoop" {
		t.Errorf("Ears row = %+v, want OK / Have=Fingerbone Hoop (matched in EAR2)", ears)
	}
}

// v1: 'char without metadata (no class) is skipped'
func TestGearCheck_NoClassSkipped(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	seedChar(t, db, "o", "NoClass", "", 60, "HUM", false) // empty class → NULL
	seedGear(t, db, "Velious Pre-Raid/Group", "NEC", "Head", "Helm", 1)

	rows, err := compute.GearCheck(ctx, s)
	if err != nil {
		t.Fatalf("GearCheck: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0 (classless char skipped): %+v", len(rows), rows)
	}
}

// v1: 'sort order: char asc → tier rank asc → slot asc' (+ Iksar last for Bee).
func TestGearCheck_SortOrder(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	seedChar(t, db, "o1", "Bee", "NEC", 60, "IKS", false)
	seedChar(t, db, "o2", "Apple", "NEC", 60, "HUM", false)
	seedGear(t, db, "Iksar", "NEC", "Head", "Iksar Helm", 1)
	seedGear(t, db, "Velious Raiding", "NEC", "Chest", "Raid Robe", 1)
	seedGear(t, db, "Velious Pre-Raid/Group", "NEC", "Head", "Pre Helm", 1)
	seedGear(t, db, "Velious Pre-Raid/Group", "NEC", "Chest", "Pre Robe", 1)

	rows, err := compute.GearCheck(ctx, s)
	if err != nil {
		t.Fatalf("GearCheck: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no rows")
	}

	// Apple sorts before Bee (char asc).
	if rows[0].Char != "Apple" {
		t.Errorf("rows[0].Char = %q, want Apple first", rows[0].Char)
	}

	// Apple's section: Pre-Raid (Chest, Head) then Raiding (Chest). Apple is HUM
	// so it has NO Iksar tier.
	var apple []compute.GearCheckRow
	for _, r := range rows {
		if r.Char == "Apple" {
			apple = append(apple, r)
		}
	}
	if len(apple) != 3 {
		t.Fatalf("Apple rows = %d, want 3 (Pre-Raid Chest+Head, Raiding Chest): %+v", len(apple), apple)
	}
	if apple[0].Tier != "Velious Pre-Raid/Group" || apple[0].Slot != "Chest" {
		t.Errorf("apple[0] = %+v, want Pre-Raid/Chest", apple[0])
	}
	if apple[1].Tier != "Velious Pre-Raid/Group" || apple[1].Slot != "Head" {
		t.Errorf("apple[1] = %+v, want Pre-Raid/Head", apple[1])
	}
	if apple[2].Tier != "Velious Raiding" {
		t.Errorf("apple[2].Tier = %q, want Velious Raiding last", apple[2].Tier)
	}

	// Bee (IKS) section: Iksar tier ranks last.
	var bee []compute.GearCheckRow
	for _, r := range rows {
		if r.Char == "Bee" {
			bee = append(bee, r)
		}
	}
	if len(bee) == 0 {
		t.Fatal("no Bee rows")
	}
	if bee[len(bee)-1].Tier != "Iksar" {
		t.Errorf("Bee last row tier = %q, want Iksar last", bee[len(bee)-1].Tier)
	}
}
