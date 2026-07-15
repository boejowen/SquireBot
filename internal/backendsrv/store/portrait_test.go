package store

// portrait_test.go covers the Phase 41 plan 41-01 portrait blob data layer
// (CHARUI-02) at the store seam: the SetPortraitTx/DeletePortraitTx/GetPortrait blob
// round-trip, the D-05/D-06 assignee-OR-officer in-tx gate (assignee ✓, officer ✓,
// stranger ✗, bank/bot officer-only), the upsert (second write overwrites, one row),
// the ErrPortraitNotFound / ErrCharNotFound sentinels, delete idempotency, and the
// migration's ON DELETE CASCADE. Reuses the package test helpers insertOwner/insertChar/
// insertWebUser/makeOfficer/setGuildBot/commitTx (assignment_test.go / admins_test.go /
// eviction_test.go); NewTestDB opens the DSN with foreign_keys(ON) so the CASCADE test
// exercises the real FK.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
)

// assignPortraitChar files a character_assignment row directly (the "this char is
// assigned to discordUserID" fixture) — the assignee is the legitimate portrait writer
// (D-05). Mirrors the seed shape ClaimCharTx/OfficerAssignTx write.
func assignPortraitChar(t *testing.T, ctx context.Context, db *sql.DB, charID int64, discordUserID string) {
	t.Helper()
	insertWebUser(t, ctx, db, discordUserID, "Assignee-"+discordUserID)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, ?, 0, 'test')`, charID, discordUserID); err != nil {
		t.Fatalf("seed character_assignment (char=%d, user=%q): %v", charID, discordUserID, err)
	}
}

// portraitRowCount returns how many character_portrait rows exist for charID (0 or 1).
func portraitRowCount(t *testing.T, ctx context.Context, db *sql.DB, charID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM character_portrait WHERE character_id = ?`, charID).Scan(&n); err != nil {
		t.Fatalf("count portrait rows (char=%d): %v", charID, err)
	}
	return n
}

// TestSetPortrait_AssigneeRoundTrip: the char's ASSIGNEE stores the blob and GetPortrait
// returns the identical bytes + content_type.
func TestSetPortrait_AssigneeRoundTrip(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")

	blob := []byte{0x89, 0x50, 0x4E, 0x47, 0xDE, 0xAD, 0xBE, 0xEF}
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", blob, "image/png", "member-1", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx (assignee): %v", err)
	}

	got, ct, err := st.GetPortrait(ctx, "Slampeach")
	if err != nil {
		t.Fatalf("GetPortrait: %v", err)
	}
	if !bytes.Equal(got, blob) {
		t.Errorf("round-trip blob = %v, want %v", got, blob)
	}
	if ct != "image/png" {
		t.Errorf("content_type = %q, want image/png", ct)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 1 {
		t.Errorf("portrait rows = %d, want 1", n)
	}
}

// TestSetPortrait_OfficerNonAssignee: an OFFICER (not the assignee) may set an assigned
// char's portrait.
func TestSetPortrait_OfficerNonAssignee(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")
	makeOfficer(t, ctx, db, "officer-9")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", []byte{0xFF, 0xD8, 0xFF, 0x01}, "image/jpeg", "officer-9", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx (officer, non-assignee): %v", err)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 1 {
		t.Errorf("portrait rows = %d, want 1", n)
	}
}

// TestSetPortrait_StrangerRejected: a caller who is neither the assignee nor an officer →
// ErrNotAuthorized, nothing written.
func TestSetPortrait_StrangerRejected(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")
	insertWebUser(t, ctx, db, "stranger-7", "Stranger")

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "stranger-7", "2026-07-15T00:00:00Z")
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("stranger SetPortraitTx: err = %v, want ErrNotAuthorized", err)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 0 {
		t.Errorf("portrait rows after rejected write = %d, want 0", n)
	}
}

// TestSetPortrait_BankBotOfficerOnly: a bank/bot char (charSharedTx=true, no assignee)
// rejects a NON-officer and accepts an OFFICER (D-06).
func TestSetPortrait_BankBotOfficerOnly(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bankID := insertChar(t, ctx, db, ownerID, "Guildbank", true) // is_bank_toon=1 (shared)
	insertWebUser(t, ctx, db, "member-1", "Member1")
	makeOfficer(t, ctx, db, "officer-9")

	// A non-officer (no assignment — bank has none) → ErrNotAuthorized.
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Guildbank", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "member-1", "2026-07-15T00:00:00Z")
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("bank non-officer SetPortraitTx: err = %v, want ErrNotAuthorized", err)
	}
	if n := portraitRowCount(t, ctx, db, bankID); n != 0 {
		t.Errorf("portrait rows after rejected bank write = %d, want 0", n)
	}

	// An officer → succeeds.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Guildbank", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "officer-9", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("bank officer SetPortraitTx: %v", err)
	}
	if n := portraitRowCount(t, ctx, db, bankID); n != 1 {
		t.Errorf("portrait rows after officer bank write = %d, want 1", n)
	}
}

// TestSetPortrait_UpsertOverwrites: a second write by the assignee overwrites (GetPortrait
// returns the second blob; still one row).
func TestSetPortrait_UpsertOverwrites(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")

	first := []byte{0x89, 0x50, 0x4E, 0x47, 0x01}
	second := []byte{0xFF, 0xD8, 0xFF, 0x02, 0x03}
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", first, "image/png", "member-1", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx (first): %v", err)
	}
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", second, "image/jpeg", "member-1", "2026-07-15T01:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx (second): %v", err)
	}

	got, ct, err := st.GetPortrait(ctx, "Slampeach")
	if err != nil {
		t.Fatalf("GetPortrait: %v", err)
	}
	if !bytes.Equal(got, second) {
		t.Errorf("after upsert blob = %v, want the second write %v", got, second)
	}
	if ct != "image/jpeg" {
		t.Errorf("after upsert content_type = %q, want image/jpeg", ct)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 1 {
		t.Errorf("portrait rows after upsert = %d, want 1 (upsert, not append)", n)
	}
}

// TestGetPortrait_NotFound: a char with no portrait → ErrPortraitNotFound.
func TestGetPortrait_NotFound(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	insertChar(t, ctx, db, ownerID, "Slampeach", false)

	_, _, err := st.GetPortrait(ctx, "Slampeach")
	if !errors.Is(err, ErrPortraitNotFound) {
		t.Errorf("GetPortrait on portrait-less char: err = %v, want ErrPortraitNotFound", err)
	}
}

// TestSetPortrait_UnknownChar: SetPortraitTx on an unknown char name → ErrCharNotFound.
func TestSetPortrait_UnknownChar(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	makeOfficer(t, ctx, db, "officer-9")

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "NoSuchChar", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "officer-9", "2026-07-15T00:00:00Z")
	})
	if !errors.Is(err, ErrCharNotFound) {
		t.Errorf("SetPortraitTx on unknown char: err = %v, want ErrCharNotFound", err)
	}
}

// TestDeletePortrait_AssigneeRemoves: the assignee removes the row (GetPortrait →
// ErrPortraitNotFound after).
func TestDeletePortrait_AssigneeRemoves(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "member-1", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx: %v", err)
	}
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DeletePortraitTx(ctx, tx, "Slampeach", "member-1")
	}); err != nil {
		t.Fatalf("DeletePortraitTx (assignee): %v", err)
	}
	if _, _, err := st.GetPortrait(ctx, "Slampeach"); !errors.Is(err, ErrPortraitNotFound) {
		t.Errorf("GetPortrait after delete: err = %v, want ErrPortraitNotFound", err)
	}
}

// TestDeletePortrait_StrangerRejected: a stranger cannot delete (gate runs BEFORE the
// DELETE → ErrNotAuthorized, the row survives).
func TestDeletePortrait_StrangerRejected(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")
	insertWebUser(t, ctx, db, "stranger-7", "Stranger")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "member-1", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx: %v", err)
	}
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DeletePortraitTx(ctx, tx, "Slampeach", "stranger-7")
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("stranger DeletePortraitTx: err = %v, want ErrNotAuthorized", err)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 1 {
		t.Errorf("portrait rows after rejected delete = %d, want 1 (row must survive)", n)
	}
}

// TestDeletePortrait_IdempotentWhenAbsent: delete-when-absent is not an error (the assignee
// deleting a char that never had a portrait).
func TestDeletePortrait_IdempotentWhenAbsent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DeletePortraitTx(ctx, tx, "Slampeach", "member-1")
	}); err != nil {
		t.Errorf("DeletePortraitTx on absent portrait: err = %v, want nil (idempotent)", err)
	}
}

// TestPortrait_ONDeleteCascade: deleting the character row drops its portrait row (the
// 00019 FK ON DELETE CASCADE). foreign_keys(ON) is in the NewTestDB DSN.
func TestPortrait_ONDeleteCascade(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	assignPortraitChar(t, ctx, db, charID, "member-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "Slampeach", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "member-1", "2026-07-15T00:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx: %v", err)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 1 {
		t.Fatalf("portrait rows before char delete = %d, want 1", n)
	}
	// Hard-delete the character row → the FK CASCADE must drop the portrait row.
	if _, err := db.ExecContext(ctx, `DELETE FROM character WHERE id = ?`, charID); err != nil {
		t.Fatalf("delete character row: %v", err)
	}
	if n := portraitRowCount(t, ctx, db, charID); n != 0 {
		t.Errorf("portrait rows after char delete = %d, want 0 (ON DELETE CASCADE)", n)
	}
}

// TestPortraitMeta_FlagAndAbsent: PortraitMeta reports (true, updatedAt) for a char with a
// portrait and (false, "", nil) for one without.
func TestPortraitMeta_FlagAndAbsent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	withID := insertChar(t, ctx, db, ownerID, "HasPortrait", false)
	insertChar(t, ctx, db, ownerID, "NoPortrait", false)
	assignPortraitChar(t, ctx, db, withID, "member-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetPortraitTx(ctx, tx, "HasPortrait", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", "member-1", "2026-07-15T09:00:00Z")
	}); err != nil {
		t.Fatalf("SetPortraitTx: %v", err)
	}

	has, updatedAt, err := st.PortraitMeta(ctx, "HasPortrait")
	if err != nil {
		t.Fatalf("PortraitMeta (has): %v", err)
	}
	if !has || updatedAt != "2026-07-15T09:00:00Z" {
		t.Errorf("PortraitMeta(HasPortrait) = (%v, %q), want (true, 2026-07-15T09:00:00Z)", has, updatedAt)
	}

	has, updatedAt, err = st.PortraitMeta(ctx, "NoPortrait")
	if err != nil {
		t.Fatalf("PortraitMeta (absent): %v", err)
	}
	if has || updatedAt != "" {
		t.Errorf("PortraitMeta(NoPortrait) = (%v, %q), want (false, \"\")", has, updatedAt)
	}
}
