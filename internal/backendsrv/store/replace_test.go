package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// seedOwnerChar inserts one owner + one character and returns the character id.
// Shared by replace_test.go and binding_test.go.
func seedOwnerChar(t *testing.T, db *sql.DB, ownerLabel, charName string) (ownerID, charID int64) {
	t.Helper()
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, ownerLabel)
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	ownerID, _ = res.LastInsertId()
	res, err = db.Exec(`INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, charName)
	if err != nil {
		t.Fatalf("seed character: %v", err)
	}
	charID, _ = res.LastInsertId()
	return ownerID, charID
}

// TestReplaceInventory_InsertsAllRows: a replace inserts every row with the
// correct INTEGER item_id/count/slots and row_ordinal = file line order.
func TestReplaceInventory_InsertsAllRows(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")

	rows := [][]string{
		{"General1", "Cloth Cap", "1001", "1", "0"},
		{"General2", "Fungi Tunic", "13128", "2", "0"},
		{"Bank1", "Bag of Holding", "1038", "1", "10"},
	}
	ctx := context.Background()
	if err := s.ReplaceInventory(ctx, charID, rows, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("ReplaceInventory: %v", err)
	}

	got, err := db.Query(`SELECT location, name, item_id, count, slots, row_ordinal
		FROM inventory_item WHERE character_id = ? ORDER BY row_ordinal`, charID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer got.Close()

	type rec struct {
		loc, name        string
		itemID, cnt, slt int64
		ord              int64
	}
	var out []rec
	for got.Next() {
		var r rec
		if err := got.Scan(&r.loc, &r.name, &r.itemID, &r.cnt, &r.slt, &r.ord); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(out))
	}
	// Verify integers stored as INTEGER (not strings) and order preserved.
	if out[0].itemID != 1001 || out[0].cnt != 1 || out[0].slt != 0 || out[0].ord != 0 {
		t.Errorf("row 0 mismatch: %+v", out[0])
	}
	if out[1].itemID != 13128 || out[1].cnt != 2 || out[1].ord != 1 {
		t.Errorf("row 1 mismatch: %+v", out[1])
	}
	if out[2].itemID != 1038 || out[2].slt != 10 || out[2].ord != 2 {
		t.Errorf("row 2 mismatch: %+v", out[2])
	}
	if out[2].name != "Bag of Holding" {
		t.Errorf("row 2 name = %q, want Bag of Holding", out[2].name)
	}
}

// TestReplaceInventory_ShrinkingSnapshotDropsRows (BACKEND-03): replacing a
// 3-row snapshot with a 1-row snapshot leaves exactly the 1 new row — the
// dropped rows are gone (full-snapshot replace, never row-diff).
func TestReplaceInventory_ShrinkingSnapshotDropsRows(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")
	ctx := context.Background()

	three := [][]string{
		{"General1", "Cloth Cap", "1001", "1", "0"},
		{"General2", "Fungi Tunic", "13128", "1", "0"},
		{"Bank1", "Bag of Holding", "1038", "1", "10"},
	}
	if err := s.ReplaceInventory(ctx, charID, three, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	one := [][]string{
		{"General1", "Rusty Dagger", "5001", "1", "0"},
	}
	if err := s.ReplaceInventory(ctx, charID, one, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("second (shrinking) replace: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_item WHERE character_id = ?`, charID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 row after shrinking snapshot, got %d", n)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM inventory_item WHERE character_id = ?`, charID).Scan(&name); err != nil {
		t.Fatalf("name query: %v", err)
	}
	if name != "Rusty Dagger" {
		t.Errorf("surviving row name = %q, want Rusty Dagger (old rows should be dropped)", name)
	}
}

// TestReplaceInventory_AtomicOnError: an INSERT failure inside the replace
// transaction rolls back EVERYTHING — no partial state is visible, and rows of
// a neighbouring character are untouched.
//
// Mechanism: with foreign_keys(ON), inserting an inventory_item whose
// character_id has no matching character row fails the FK constraint. We seed a
// real neighbour character (3 committed rows) and a target character, then
// delete the target's character row out-of-band so the in-tx INSERT fails FK
// after the in-tx DELETE has already run. A correct atomic replace rolls the
// DELETE back; a broken (two-transaction / non-atomic) implementation would
// leave the target empty AND/OR commit a partial set.
func TestReplaceInventory_AtomicOnError(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// Neighbour character with 3 committed rows — must survive the failed replace.
	_, neighbourID := seedOwnerChar(t, db, "owner-a", "Neighbour")
	neighbourRows := [][]string{
		{"General1", "Cloth Cap", "1001", "1", "0"},
		{"General2", "Fungi Tunic", "13128", "1", "0"},
		{"Bank1", "Bag of Holding", "1038", "1", "10"},
	}
	if err := s.ReplaceInventory(ctx, neighbourID, neighbourRows, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("seed neighbour rows: %v", err)
	}

	// Target character: seed rows, then delete its character row so a re-INSERT
	// fails the FK constraint (FK action requires foreign_keys(ON), set in DSN).
	_, targetID := seedOwnerChar(t, db, "owner-b", "Target")
	if err := s.ReplaceInventory(ctx, targetID, neighbourRows, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("seed target rows: %v", err)
	}
	// Deleting the character cascades and removes its inventory rows; the point
	// is that the NEXT ReplaceInventory(targetID, …) must fail its INSERTs (FK).
	if _, err := db.Exec(`DELETE FROM character WHERE id = ?`, targetID); err != nil {
		t.Fatalf("delete target character: %v", err)
	}

	// This replace must FAIL (FK violation on INSERT) and roll back atomically.
	err := s.ReplaceInventory(ctx, targetID, neighbourRows, time.Now().UTC(), "0.3.0")
	if err == nil {
		t.Fatalf("expected ReplaceInventory to fail (FK violation), got nil")
	}

	// No partial state for the target.
	var targetCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_item WHERE character_id = ?`, targetID).Scan(&targetCount); err != nil {
		t.Fatalf("target count: %v", err)
	}
	if targetCount != 0 {
		t.Errorf("expected 0 rows for target after rolled-back replace, got %d (partial state leaked)", targetCount)
	}
	// Neighbour rows untouched.
	var neighbourCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_item WHERE character_id = ?`, neighbourID).Scan(&neighbourCount); err != nil {
		t.Fatalf("neighbour count: %v", err)
	}
	if neighbourCount != 3 {
		t.Errorf("neighbour rows must be untouched by the failed replace; want 3, got %d", neighbourCount)
	}
}

// TestReplaceInventory_UpdatesCharacterLastSeen: last_seen and watcher_version
// are written in the same transaction as the row replace.
func TestReplaceInventory_UpdatesCharacterLastSeen(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")
	ctx := context.Background()

	uploadedAt := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	rows := [][]string{{"General1", "Cloth Cap", "1001", "1", "0"}}
	if err := s.ReplaceInventory(ctx, charID, rows, uploadedAt, "1.2.3"); err != nil {
		t.Fatalf("ReplaceInventory: %v", err)
	}

	var lastSeen, watcherVer sql.NullString
	if err := db.QueryRow(`SELECT last_seen, watcher_version FROM character WHERE id = ?`, charID).
		Scan(&lastSeen, &watcherVer); err != nil {
		t.Fatalf("query character: %v", err)
	}
	if !lastSeen.Valid || lastSeen.String == "" {
		t.Errorf("last_seen not set after replace")
	}
	if watcherVer.String != "1.2.3" {
		t.Errorf("watcher_version = %q, want 1.2.3", watcherVer.String)
	}
}

// TestReplaceSpellbook_NormalizedName: a spellbook replace stores
// normalized_name = lower(trim(name)) (the P12/P14 wiki join key) and level as
// a real INTEGER.
func TestReplaceSpellbook_NormalizedName(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")
	ctx := context.Background()

	rows := [][]string{
		{"39", "  Complete Heal  "},
		{"29", "Sense Summoned"},
	}
	if err := s.ReplaceSpellbook(ctx, charID, rows, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("ReplaceSpellbook: %v", err)
	}

	var level int64
	var name, normalized string
	if err := db.QueryRow(`SELECT level, name, normalized_name FROM spellbook_entry
		WHERE character_id = ? AND level = 39`, charID).Scan(&level, &name, &normalized); err != nil {
		t.Fatalf("query: %v", err)
	}
	if level != 39 {
		t.Errorf("level = %d, want 39 (stored as INTEGER)", level)
	}
	if normalized != "complete heal" {
		t.Errorf("normalized_name = %q, want %q (lower(trim(name)))", normalized, "complete heal")
	}
}
