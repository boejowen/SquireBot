package store

import (
	"context"
	"database/sql"
	"testing"
)

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
