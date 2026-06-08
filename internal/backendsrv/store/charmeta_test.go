package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// charmeta_test.go covers the char-metadata write persistence layer (CUTOVER-02 /
// P16, D-02/D-03) at the store seam: SetCharMetaTx writes class/level/race onto an
// EXISTING, non-removed character; a missing/removed target → ErrCharNotFound
// (fail-closed); a blank (nil) level stays SQL NULL.
//
// Phase 26 reconciliation: is_bank_toon is no longer written here (it became the
// officer-only "guild bank" designation, store.DesignateCharTx) and the MD-01
// single-bank-toon demote is GONE (multiple guild banks are now allowed). The
// former demote/re-save/clear regression tests were removed; the multi-bank
// behavior is proven in store/assignment_test.go (DesignateCharTx) +
// compute/bank_test.go (the 2-bank render case).

// TestSetCharMetaTx_WritesMeta is the happy path: the three columns land on an
// existing live character, and a blank (nil) level stays SQL NULL.
func TestSetCharMetaTx_WritesMeta(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	lvl := int64(50)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, charID, "WAR", &lvl, "IKS")
	}); err != nil {
		t.Fatalf("SetCharMetaTx: %v", err)
	}

	var class, race sql.NullString
	var level sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT class, level, race FROM character WHERE id = ?`, charID,
	).Scan(&class, &level, &race); err != nil {
		t.Fatalf("read meta: %v", err)
	}
	if !class.Valid || class.String != "WAR" {
		t.Errorf("class = %v, want WAR", class)
	}
	if !level.Valid || level.Int64 != 50 {
		t.Errorf("level = %v, want 50", level)
	}
	if !race.Valid || race.String != "IKS" {
		t.Errorf("race = %v, want IKS", race)
	}

	// A nil level stays NULL (blank = unset; the form must not fabricate a 0).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, charID, "WAR", nil, "IKS")
	}); err != nil {
		t.Fatalf("SetCharMetaTx (nil level): %v", err)
	}
	level = sql.NullInt64{}
	if err := db.QueryRowContext(ctx, `SELECT level FROM character WHERE id = ?`, charID).Scan(&level); err != nil {
		t.Fatalf("read level: %v", err)
	}
	if level.Valid {
		t.Errorf("level = %v, want NULL (blank stays unset)", level)
	}
}

// TestSetCharMetaTx_RejectsRemovedOrMissing: a missing or soft-removed character is
// not editable (ErrCharNotFound, fail-closed) and nothing is written.
func TestSetCharMetaTx_RejectsRemovedOrMissing(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")

	// Missing id → ErrCharNotFound.
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, 999999, "WAR", nil, "IKS")
	})
	if !errors.Is(err, ErrCharNotFound) {
		t.Errorf("SetCharMetaTx on missing char: err = %v, want ErrCharNotFound", err)
	}

	// Soft-removed char → ErrCharNotFound, nothing written.
	removed := insertChar(t, ctx, db, ownerID, "Oldtoon", false)
	if _, err := db.ExecContext(ctx, `UPDATE character SET is_removed = 1 WHERE id = ?`, removed); err != nil {
		t.Fatalf("mark removed: %v", err)
	}
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, removed, "WAR", nil, "IKS")
	})
	if !errors.Is(err, ErrCharNotFound) {
		t.Errorf("SetCharMetaTx on removed char: err = %v, want ErrCharNotFound", err)
	}
	var class sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT class FROM character WHERE id = ?`, removed).Scan(&class); err != nil {
		t.Fatalf("read class: %v", err)
	}
	if class.Valid {
		t.Errorf("class written on a removed char: %v", class)
	}
}
