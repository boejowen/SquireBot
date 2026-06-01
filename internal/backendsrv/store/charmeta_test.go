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
// (fail-closed); a blank (nil) level stays SQL NULL; and — the MD-01 fix — the
// single-bank-toon invariant the bank compute view (compute/bank.go) relies on is
// enforced HERE by demoting any other live bank toon when one is promoted.

// countBankToons returns how many LIVE (is_removed=0) characters are flagged
// is_bank_toon=1 — the MD-01 invariant oracle (the bankOnly InventoryJoin and the
// bank view assume this is at most 1).
func countBankToons(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM character WHERE is_bank_toon = 1 AND is_removed = 0`,
	).Scan(&n); err != nil {
		t.Fatalf("count bank toons: %v", err)
	}
	return n
}

// isBankToon reports whether a single character row is flagged is_bank_toon=1.
func isBankToon(t *testing.T, ctx context.Context, db *sql.DB, charID int64) bool {
	t.Helper()
	var bt int
	if err := db.QueryRowContext(ctx,
		`SELECT is_bank_toon FROM character WHERE id = ?`, charID,
	).Scan(&bt); err != nil {
		t.Fatalf("read is_bank_toon (id=%d): %v", charID, err)
	}
	return bt == 1
}

// TestSetCharMetaTx_WritesMeta is the happy path: the four columns land on an
// existing live character, and a blank (nil) level stays SQL NULL.
func TestSetCharMetaTx_WritesMeta(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	lvl := int64(50)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, charID, "WAR", &lvl, "IKS", false)
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
		return SetCharMetaTx(ctx, tx, charID, "WAR", nil, "IKS", false)
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
		return SetCharMetaTx(ctx, tx, 999999, "WAR", nil, "IKS", false)
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
		return SetCharMetaTx(ctx, tx, removed, "WAR", nil, "IKS", false)
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

// TestSetCharMetaTx_SingleBankToonInvariant is the MD-01 regression: char-meta is
// the first/only writer of is_bank_toon=true, and the bank compute view assumes at
// most ONE live bank toon. Seed an EXISTING bank toon, then flag a SECOND character
// as bank toon via the write path, and assert exactly one is_bank_toon=1 row
// remains — the newly-flagged one, with the prior one demoted.
func TestSetCharMetaTx_SingleBankToonInvariant(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	// An EXISTING bank toon (the prior curated guild bank).
	oldBank := insertChar(t, ctx, db, ownerID, "Oldbank", true)
	// A plain character we will promote to be the new bank toon.
	newBank := insertChar(t, ctx, db, ownerID, "Newbank", false)

	if got := countBankToons(t, ctx, db); got != 1 {
		t.Fatalf("setup: bank toons = %d, want 1 (the seeded Oldbank)", got)
	}

	// Flag the SECOND character as bank toon via the write path.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, newBank, "WAR", nil, "IKS", true)
	}); err != nil {
		t.Fatalf("SetCharMetaTx (promote newBank): %v", err)
	}

	// Exactly ONE live bank toon remains (the MD-01 invariant).
	if got := countBankToons(t, ctx, db); got != 1 {
		t.Fatalf("after promoting a second char: live bank toons = %d, want exactly 1 (single-bank-toon invariant)", got)
	}
	// …and it is the newly-flagged one; the prior bank toon was demoted.
	if !isBankToon(t, ctx, db, newBank) {
		t.Errorf("newBank is not the bank toon after being promoted")
	}
	if isBankToon(t, ctx, db, oldBank) {
		t.Errorf("oldBank is still flagged is_bank_toon=1; it should have been demoted")
	}
}

// TestSetCharMetaTx_ReSaveSelfIsNoOpDemote: re-saving the CURRENT bank toon (with
// is_bank_toon still true) must not demote itself — it stays the bank toon and the
// count is still exactly 1 (the demote excludes self via id <> ?).
func TestSetCharMetaTx_ReSaveSelfIsNoOpDemote(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Banktoon", true)

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, bank, "ENC", nil, "VAH", true)
	}); err != nil {
		t.Fatalf("SetCharMetaTx (re-save self): %v", err)
	}
	if got := countBankToons(t, ctx, db); got != 1 {
		t.Errorf("re-saving the current bank toon: live bank toons = %d, want 1", got)
	}
	if !isBankToon(t, ctx, db, bank) {
		t.Errorf("the bank toon demoted itself on re-save (id <> ? exclusion failed)")
	}
}

// TestSetCharMetaTx_DemoteToFalseLeavesNoBankToon: clearing the bank-toon flag
// (is_bank_toon=false) on the sole bank toon leaves zero live bank toons and never
// promotes anyone else (the demote branch only runs when promoting).
func TestSetCharMetaTx_DemoteToFalseLeavesNoBankToon(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Banktoon", true)
	insertChar(t, ctx, db, ownerID, "Plaintoon", false) // must NOT be auto-promoted

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCharMetaTx(ctx, tx, bank, "WAR", nil, "IKS", false)
	}); err != nil {
		t.Fatalf("SetCharMetaTx (clear flag): %v", err)
	}
	if got := countBankToons(t, ctx, db); got != 0 {
		t.Errorf("after clearing the only bank toon: live bank toons = %d, want 0", got)
	}
}
