package store

// owner_test.go covers the Phase 35 guild-sentinel repoint (OWN-01/OWN-02): when an
// officer designates a char a guild bank/bot, DesignateCharTx repoints its owner_id to
// GuildSentinelOwnerID (so the char is GUILD-HELD and survives the first uploader's
// eviction), the officer re-check still gates the write, and clearing a designation
// ('neither') does NOT re-home the char. Reuses the package helpers makeOfficer/
// insertOwner/insertChar/insertWebUser/commitTx (assignment_test.go / eviction_test.go).
//
// NewTestDB runs migration 00015, which seeds the sentinel owner row, so the repoint's
// `owner_id = GuildSentinelOwnerID` satisfies the character.owner_id FK.

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// charOwnerFlags reads owner_id, is_bank_toon, is_guild_bot for a character id.
func charOwnerFlags(t *testing.T, ctx context.Context, db *sql.DB, charID int64) (ownerID int64, isBank, isBot int) {
	t.Helper()
	if err := db.QueryRowContext(ctx,
		`SELECT owner_id, is_bank_toon, is_guild_bot FROM character WHERE id = ?`, charID,
	).Scan(&ownerID, &isBank, &isBot); err != nil {
		t.Fatalf("read char owner/flags (id=%d): %v", charID, err)
	}
	return
}

// TestDesignateCharTx_BankRepointsOwnerToSentinel: designating a char a guild bank
// repoints owner_id to GuildSentinelOwnerID (OWN-01/OWN-02) and removes any prior
// assignment (the existing D-02 behavior still holds).
func TestDesignateCharTx_BankRepointsOwnerToSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	realOwner := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, realOwner, "Slampeach", false)
	insertWebUser(t, ctx, db, "alice", "Alice")
	makeOfficer(t, ctx, db, "officer-1")

	// alice claims it first (a real owner-bound, assigned char).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "alice", 100)
	}); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	// Pre-designation it sits under the real owner.
	if owner, _, _ := charOwnerFlags(t, ctx, db, charID); owner != realOwner {
		t.Fatalf("pre-designate owner_id = %d, want realOwner %d", owner, realOwner)
	}

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateBank, "officer-1", 300)
	}); err != nil {
		t.Fatalf("DesignateCharTx (bank): %v", err)
	}

	owner, isBank, isBot := charOwnerFlags(t, ctx, db, charID)
	if owner != GuildSentinelOwnerID {
		t.Errorf("owner_id after bank designate = %d, want sentinel %d (OWN-01/02)", owner, GuildSentinelOwnerID)
	}
	if isBank != 1 || isBot != 0 {
		t.Errorf("flags after bank designate = is_bank_toon=%d is_guild_bot=%d, want 1,0", isBank, isBot)
	}
	// The prior assignment is gone (D-02 still holds).
	if _, ok := assigneeOf(t, ctx, db, charID); ok {
		t.Errorf("assignment survived a guild-bank designation (Pitfall 6)")
	}
}

// TestDesignateCharTx_BotRepointsOwnerToSentinel: designating a char a guild bot
// repoints owner_id to GuildSentinelOwnerID and sets is_guild_bot=1.
func TestDesignateCharTx_BotRepointsOwnerToSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	realOwner := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, realOwner, "Buffbot", false)
	makeOfficer(t, ctx, db, "officer-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateBot, "officer-1", 100)
	}); err != nil {
		t.Fatalf("DesignateCharTx (bot): %v", err)
	}

	owner, isBank, isBot := charOwnerFlags(t, ctx, db, charID)
	if owner != GuildSentinelOwnerID {
		t.Errorf("owner_id after bot designate = %d, want sentinel %d (OWN-01/02)", owner, GuildSentinelOwnerID)
	}
	if isBank != 0 || isBot != 1 {
		t.Errorf("flags after bot designate = is_bank_toon=%d is_guild_bot=%d, want 0,1", isBank, isBot)
	}
}

// TestDesignateCharTx_NeitherDoesNotRepoint: a char already sentinel-owned (designated
// a bank) then designated 'neither' clears both flags but LEAVES owner_id at the
// sentinel (clearing a designation does not re-home the char to any individual owner).
func TestDesignateCharTx_NeitherDoesNotRepoint(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	realOwner := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, realOwner, "Wasbank", false)
	makeOfficer(t, ctx, db, "officer-1")

	// First designate bank → owner_id becomes the sentinel.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateBank, "officer-1", 100)
	}); err != nil {
		t.Fatalf("DesignateCharTx (bank): %v", err)
	}
	if owner, _, _ := charOwnerFlags(t, ctx, db, charID); owner != GuildSentinelOwnerID {
		t.Fatalf("after bank designate owner_id = %d, want sentinel %d", owner, GuildSentinelOwnerID)
	}

	// Now clear the designation.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateNeither, "officer-1", 200)
	}); err != nil {
		t.Fatalf("DesignateCharTx (neither): %v", err)
	}

	owner, isBank, isBot := charOwnerFlags(t, ctx, db, charID)
	if isBank != 0 || isBot != 0 {
		t.Errorf("flags after 'neither' = is_bank_toon=%d is_guild_bot=%d, want 0,0", isBank, isBot)
	}
	if owner != GuildSentinelOwnerID {
		t.Errorf("owner_id after 'neither' = %d, want sentinel %d (clearing does not re-home)", owner, GuildSentinelOwnerID)
	}
}

// TestDesignateCharTx_NonOfficerNoRepoint: a non-officer caller is rejected with
// ErrNotAuthorized BEFORE any write — owner_id is unchanged (still the real owner).
func TestDesignateCharTx_NonOfficerNoRepoint(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	realOwner := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, realOwner, "Slampeach", false)
	insertWebUser(t, ctx, db, "rando", "Rando") // a non-officer

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateBank, "rando", 100)
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer designate: err = %v, want ErrNotAuthorized", err)
	}

	owner, isBank, _ := charOwnerFlags(t, ctx, db, charID)
	if owner != realOwner {
		t.Errorf("owner_id after rejected designate = %d, want realOwner %d (no write)", owner, realOwner)
	}
	if isBank != 0 {
		t.Errorf("is_bank_toon after rejected designate = %d, want 0 (no write)", isBank)
	}
}
