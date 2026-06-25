package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

// normEnrich mirrors the SQL dedup key lower(trim(name)) so the test can assert
// "one ref per normalized name" the same way the union groups.
func normEnrich(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// seedRaw inserts one inventory_item row directly (raw INSERT, not via
// ReplaceInventory) so the test can control item_id = 0 / NULL precisely.
// Pass an *int64 for a concrete id (including 0) or nil for a SQL NULL.
func seedRaw(t *testing.T, db *sql.DB, charID int64, location, name string, itemID *int64, ordinal int64) {
	t.Helper()
	var idArg interface{}
	if itemID != nil {
		idArg = *itemID
	} // else idArg stays nil → SQL NULL
	if _, err := db.Exec(
		`INSERT INTO inventory_item (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		charID, location, name, idArg, 1, 0, ordinal,
	); err != nil {
		t.Fatalf("seed inventory_item %q: %v", name, err)
	}
}

func i64ptr(v int64) *int64 { return &v }

// TestDistinctInventoryItemIDs verifies the wiki items pass's read helper:
// the distinct (item_id, name) union across all of inventory_item, EXCLUDING
// item_id = 0 and NULL, deduped, ordered by item_id.
func TestDistinctInventoryItemIDs(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charA := seedOwnerChar(t, db, "owner-a", "Aragorn")
	_, charB := seedOwnerChar(t, db, "owner-b", "Boromir")

	// charA: two real items + a duplicate of one of them + an empty slot (id=0)
	// + a NULL-id row.
	seedRaw(t, db, charA, "General1", "Cloth Cap", i64ptr(1001), 1)
	seedRaw(t, db, charA, "General2", "Fungi Tunic", i64ptr(13128), 2)
	seedRaw(t, db, charA, "Bank1", "Cloth Cap (dup)", i64ptr(1001), 3) // duplicate item_id 1001
	seedRaw(t, db, charA, "General3", "(empty)", i64ptr(0), 4)         // empty slot id=0 → excluded
	seedRaw(t, db, charA, "General4", "Unknown", nil, 5)               // NULL id → excluded

	// charB: a third real item + ANOTHER duplicate of 13128 (cross-character dedup).
	seedRaw(t, db, charB, "General1", "Pearl", i64ptr(11000), 1)
	seedRaw(t, db, charB, "General2", "Fungi Tunic", i64ptr(13128), 2) // duplicate item_id 13128

	refs, err := s.DistinctInventoryItemIDs(ctx)
	if err != nil {
		t.Fatalf("DistinctInventoryItemIDs: %v", err)
	}

	// Expect exactly the deduped real ids, ordered: 1001, 11000, 13128 — ONE row
	// per item_id even though 1001 and 13128 each appear twice (dedup is by id
	// alone, like the Sheet, so the wiki pass fetches each id's page once).
	want := []ItemRef{
		{ItemID: 1001, Name: "Cloth Cap"},   // MIN("Cloth Cap","Cloth Cap (dup)")
		{ItemID: 11000, Name: "Pearl"},
		{ItemID: 13128, Name: "Fungi Tunic"},
	}
	if len(refs) != len(want) {
		t.Fatalf("got %d refs %+v, want %d (%v)", len(refs), refs, len(want), want)
	}
	for i, w := range want {
		if refs[i].ItemID != w.ItemID {
			t.Errorf("refs[%d].ItemID = %d, want %d (full: %+v)", i, refs[i].ItemID, w.ItemID, refs)
		}
		if refs[i].Name != w.Name {
			t.Errorf("refs[%d].Name = %q, want %q (full: %+v)", i, refs[i].Name, w.Name, refs)
		}
	}

	// Explicitly assert 0 and NULL are excluded.
	for _, r := range refs {
		if r.ItemID == 0 {
			t.Errorf("item_id 0 must be excluded, got %+v", r)
		}
	}
}

// TestDistinctEnrichmentRefs verifies the WIDENED enrichment ref set (Phase 38,
// ENRICH-14): the held EQ-id refs (DistinctInventoryItemIDs, unchanged) UNIONed
// with the catalog-only PigParse refs, deduped by lower(trim(name)). It mirrors
// TestDistinctInventoryItemIDs for the held arm and adds the four D-04 guard cases:
//   - A: an unheld catalog item appears in the union keyed by its PigParse id.
//   - B: a catalog item whose name IS held appears EXACTLY ONCE, keyed by the held
//     EQ id (held wins) — never duplicated under the PigParse id.
//   - C: a catalog item_id numerically equal to an existing item_master row id (for
//     a DIFFERENT, unheld name) is EXCLUDED (the NOT IN (SELECT item_id FROM
//     item_master) collision guard), so ON CONFLICT(item_id) can never overwrite it.
//   - D: exactly one ref per normalized name after the full union (Pitfall 1).
func TestDistinctEnrichmentRefs(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charA := seedOwnerChar(t, db, "owner-a", "Aragorn")
	_, charB := seedOwnerChar(t, db, "owner-b", "Boromir")

	// --- Held arm (re-prove DistinctInventoryItemIDs behavior through the union) ---
	// charA: two real items + a duplicate of one + an empty slot (id=0) + a NULL-id row.
	seedRaw(t, db, charA, "General1", "Cloth Cap", i64ptr(1001), 1)
	seedRaw(t, db, charA, "General2", "Fungi Tunic", i64ptr(13128), 2)
	seedRaw(t, db, charA, "Bank1", "Cloth Cap (dup)", i64ptr(1001), 3) // duplicate item_id 1001
	seedRaw(t, db, charA, "General3", "(empty)", i64ptr(0), 4)         // empty slot id=0 → excluded
	seedRaw(t, db, charA, "General4", "Unknown", nil, 5)               // NULL id → excluded
	// charB: a third real item + ANOTHER duplicate of 13128 (cross-character dedup).
	seedRaw(t, db, charB, "General1", "Pearl", i64ptr(11000), 1)
	seedRaw(t, db, charB, "General2", "Fungi Tunic", i64ptr(13128), 2) // duplicate item_id 13128

	// --- Catalog arm seeding (pigparse_price; ids in the DIFFERENT PigParse namespace) ---
	// Case A: unheld catalog item → should appear keyed by its PigParse id 90001.
	seedPigparse(t, db, 90001, "Cloak of Flames", "up", 1000, 5)
	// Case B: a catalog row whose normalized name IS held ("cloth cap"), under a
	// DIFFERENT PigParse id — must be excluded by held-name dedup (held EQ id 1001 wins).
	seedPigparse(t, db, 90002, "Cloth Cap", "down", 50, 9)
	// Case B': casing/whitespace variant of a held name → still excluded by lower(trim()).
	seedPigparse(t, db, 90003, "  fungi tunic  ", "flat", 30, 3)
	// Case C: a catalog item_id (1001) that numerically equals a DIFFERENT item's
	// item_master row id → must be EXCLUDED so it can't overwrite that held row.
	// seedItemMaster a row at id 1001 for an UNHELD name distinct from the catalog name.
	seedItemMaster(t, db, 1001, "Cloth Cap", "summary", "http://example/cc", false)
	seedPigparse(t, db, 1001, "Colliding Catalog Name", "up", 5, 2) // same id 1001 → excluded
	// Case D-support: a SECOND distinct unheld catalog name to confirm the catalog arm
	// emits it once and the union has no duplicate-name rows.
	seedPigparse(t, db, 90004, "Manastone", "up", 9000, 1)
	// A catalog row with a blank name → excluded by trim(name) <> '' guard.
	seedPigparse(t, db, 90005, "   ", "flat", 1, 1)

	refs, err := s.DistinctEnrichmentRefs(ctx)
	if err != nil {
		t.Fatalf("DistinctEnrichmentRefs: %v", err)
	}

	// Build a name→id lookup + a normalized-name multiset for the assertions.
	idByName := make(map[string]int64)
	normCount := make(map[string]int)
	for _, r := range refs {
		idByName[r.Name] = r.ItemID
		normCount[normEnrich(r.Name)] += 1
		if r.ItemID == 0 {
			t.Errorf("item_id 0 must never appear in the union, got %+v", r)
		}
	}

	// Held arm preserved: 1001 (Cloth Cap), 11000 (Pearl), 13128 (Fungi Tunic).
	if got := idByName["Cloth Cap"]; got != 1001 {
		t.Errorf("held Cloth Cap keyed by %d, want EQ id 1001 (held wins)", got)
	}
	if got := idByName["Pearl"]; got != 11000 {
		t.Errorf("held Pearl keyed by %d, want EQ id 11000", got)
	}
	if got := idByName["Fungi Tunic"]; got != 13128 {
		t.Errorf("held Fungi Tunic keyed by %d, want EQ id 13128", got)
	}

	// Case A: unheld "Cloak of Flames" appears keyed by its PigParse id 90001.
	if got, ok := idByName["Cloak of Flames"]; !ok {
		t.Errorf("unheld catalog item Cloak of Flames missing from the union")
	} else if got != 90001 {
		t.Errorf("Cloak of Flames keyed by %d, want PigParse id 90001", got)
	}

	// Case A: a second unheld catalog item Manastone is present keyed by its id.
	if got, ok := idByName["Manastone"]; !ok {
		t.Errorf("unheld catalog item Manastone missing from the union")
	} else if got != 90004 {
		t.Errorf("Manastone keyed by %d, want PigParse id 90004", got)
	}

	// Case B: the held name "cloth cap"/"fungi tunic" appears EXACTLY ONCE and the
	// duplicate catalog ids (90002, 90003) are NOT present.
	if normCount[normEnrich("Cloth Cap")] != 1 {
		t.Errorf("normalized name 'cloth cap' appears %d times, want exactly 1 (held wins, catalog dup excluded)", normCount[normEnrich("Cloth Cap")])
	}
	if normCount[normEnrich("Fungi Tunic")] != 1 {
		t.Errorf("normalized name 'fungi tunic' appears %d times, want exactly 1 (held wins over casing variant)", normCount[normEnrich("Fungi Tunic")])
	}
	for _, r := range refs {
		if r.ItemID == 90002 || r.ItemID == 90003 {
			t.Errorf("catalog dup of a held name leaked into the union: %+v (held name must win)", r)
		}
	}

	// Case C: the id-collision catalog row "Colliding Catalog Name" is EXCLUDED.
	if _, ok := idByName["Colliding Catalog Name"]; ok {
		t.Errorf("catalog row whose id (1001) collides with an item_master row leaked into the union — collision guard broken")
	}

	// The blank-name catalog row is excluded.
	for _, r := range refs {
		if strings.TrimSpace(r.Name) == "" {
			t.Errorf("blank-name catalog row leaked into the union: %+v", r)
		}
	}

	// Case D: exactly one ref per normalized name across the WHOLE union.
	for norm, c := range normCount {
		if c != 1 {
			t.Errorf("normalized name %q appears %d times, want exactly 1 (Pitfall 1 — one wiki page per item)", norm, c)
		}
	}
}
