package store

import (
	"context"
	"database/sql"
	"testing"
)

// seedOwner inserts one owner and returns its id.
func seedOwner(t *testing.T, db *sql.DB, label string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, label)
	if err != nil {
		t.Fatalf("seed owner %q: %v", label, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// bindInTx runs bindCharacter inside a real transaction and commits on success
// (bindCharacter takes a *sql.Tx so 11-05 can compose bind + replace in ONE tx).
func bindInTx(t *testing.T, db *sql.DB, charName string, tokenOwnerID int64) (int64, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	charID, bindErr := bindCharacter(ctx, tx, charName, tokenOwnerID)
	if bindErr != nil {
		// A non-nil bindErr is now only a real DB failure (cross-owner uploads
		// are allowed, 260621-u6j). Commit anyway so any in-tx audit row is
		// durable for assertions, then surface the error.
		if cerr := tx.Commit(); cerr != nil {
			t.Fatalf("commit after bind error: %v", cerr)
		}
		return charID, bindErr
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return charID, nil
}

// TestBindCharacter_FirstSightingInserts: binding a brand-new name inserts a
// character bound to the uploading token's owner and returns the new charID.
func TestBindCharacter_FirstSightingInserts(t *testing.T) {
	db := NewTestDB(t)
	ownerID := seedOwner(t, db, "owner-1")

	charID, err := bindInTx(t, db, "Newchar", ownerID)
	if err != nil {
		t.Fatalf("bindCharacter: %v", err)
	}
	if charID == 0 {
		t.Fatalf("expected a non-zero charID on first sighting")
	}

	var gotOwner int64
	var gotName string
	if err := db.QueryRow(`SELECT owner_id, name FROM character WHERE id = ?`, charID).
		Scan(&gotOwner, &gotName); err != nil {
		t.Fatalf("query character: %v", err)
	}
	if gotOwner != ownerID {
		t.Errorf("owner_id = %d, want %d (first sighting binds to uploading owner)", gotOwner, ownerID)
	}
	if gotName != "Newchar" {
		t.Errorf("name = %q, want Newchar", gotName)
	}
}

// TestBindCharacter_SameOwnerReturnsExisting: a repeat bind by the SAME owner is
// a no-op insert — same charID, no second row.
func TestBindCharacter_SameOwnerReturnsExisting(t *testing.T) {
	db := NewTestDB(t)
	ownerID := seedOwner(t, db, "owner-1")

	first, err := bindInTx(t, db, "Repeatchar", ownerID)
	if err != nil {
		t.Fatalf("first bind: %v", err)
	}
	second, err := bindInTx(t, db, "Repeatchar", ownerID)
	if err != nil {
		t.Fatalf("second bind: %v", err)
	}
	if first != second {
		t.Errorf("same-owner re-bind returned a different charID: first=%d second=%d", first, second)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character WHERE name = ?`, "Repeatchar").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 character row, got %d (no second insert on same-owner re-bind)", count)
	}
}

// TestBindCharacter_CrossOwnerWriteAllowed: a bind by a DIFFERENT owner now
// SUCCEEDS (260621-u6j — shared chars/banks). It returns the EXISTING charID +
// nil error, leaves owner_id UNCHANGED (the first uploader stays a non-binding
// steward record), and appends one cross_owner_write audit_log row.
func TestBindCharacter_CrossOwnerWriteAllowed(t *testing.T) {
	db := NewTestDB(t)
	owner1 := seedOwner(t, db, "owner-1")
	owner2 := seedOwner(t, db, "owner-2")

	// owner1 first-sights the character.
	origID, err := bindInTx(t, db, "Contested", owner1)
	if err != nil {
		t.Fatalf("owner1 first sighting: %v", err)
	}

	// owner2 binds the same name → ALLOWED, returns the SAME charID + nil error.
	crossID, err := bindInTx(t, db, "Contested", owner2)
	if err != nil {
		t.Fatalf("cross-owner bind must succeed now, got err: %v", err)
	}
	if crossID != origID {
		t.Errorf("cross-owner bind charID = %d, want %d (same character)", crossID, origID)
	}

	// owner_id must be UNCHANGED (still owner1 — the non-binding steward guarantee).
	var gotOwner int64
	if err := db.QueryRow(`SELECT owner_id FROM character WHERE id = ?`, origID).Scan(&gotOwner); err != nil {
		t.Fatalf("query owner_id: %v", err)
	}
	if gotOwner != owner1 {
		t.Errorf("owner_id = %d, want %d (cross-owner write must NOT overwrite owner_id)", gotOwner, owner1)
	}

	// An audit_log row recording the cross-owner WRITE must exist.
	var auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log
		WHERE event = 'cross_owner_write' AND char_name = ?
		  AND attempting_owner_id = ? AND current_owner_id = ?`,
		"Contested", owner2, owner1).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 cross_owner_write audit_log row, got %d", auditCount)
	}
}

// TestBindCharacter_CaseInsensitiveName: "Bob" then "bob" resolve to the SAME
// character (the name column is UNIQUE COLLATE NOCASE) — lookup is by name,
// never by row index.
func TestBindCharacter_CaseInsensitiveName(t *testing.T) {
	db := NewTestDB(t)
	ownerID := seedOwner(t, db, "owner-1")

	upper, err := bindInTx(t, db, "Bob", ownerID)
	if err != nil {
		t.Fatalf("bind Bob: %v", err)
	}
	lower, err := bindInTx(t, db, "bob", ownerID)
	if err != nil {
		t.Fatalf("bind bob: %v", err)
	}
	if upper != lower {
		t.Errorf("case-insensitive name should resolve to same character: Bob=%d bob=%d", upper, lower)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 character row (COLLATE NOCASE), got %d", count)
	}
}
