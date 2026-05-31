package store

import (
	"context"
	"database/sql"
	"testing"
)

// eviction_test.go covers the v1 eviction port (15-01 Task 3): per-owner
// is_removed cascade + grace_until stamp + the D-10 guild_code revoke in ONE tx,
// reversibility during grace, and idempotent archive-past-grace. Behavioral
// oracle: showEvictionSidebar.ts + weeklyEvictionArchive.ts.

// insertOwner inserts an owner row and returns its id.
func insertOwner(t *testing.T, ctx context.Context, db *sql.DB, label string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, label)
	if err != nil {
		t.Fatalf("insert owner %q: %v", label, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("owner last insert id: %v", err)
	}
	return id
}

// insertChar inserts a character for ownerID and returns its id.
func insertChar(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64, name string, isBank bool) int64 {
	t.Helper()
	bank := 0
	if isBank {
		bank = 1
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name, is_bank_toon) VALUES (?, ?, ?)`, ownerID, name, bank)
	if err != nil {
		t.Fatalf("insert character %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("character last insert id: %v", err)
	}
	return id
}

// insertGuildCode inserts an active guild_code for ownerID.
func insertGuildCode(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64, label string) {
	t.Helper()
	// token_hash is a UNIQUE BLOB; use the label bytes as a unique stand-in.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)`,
		ownerID, []byte("hash-"+label), label); err != nil {
		t.Fatalf("insert guild_code for owner %d: %v", ownerID, err)
	}
}

// charState reads is_removed, grace_until, archived_at for a character id.
func charState(t *testing.T, ctx context.Context, db *sql.DB, charID int64) (isRemoved int, grace, archived sql.NullInt64) {
	t.Helper()
	if err := db.QueryRowContext(ctx,
		`SELECT is_removed, grace_until, archived_at FROM character WHERE id = ?`, charID,
	).Scan(&isRemoved, &grace, &archived); err != nil {
		t.Fatalf("read char state (id=%d): %v", charID, err)
	}
	return
}

func TestEvictOwnerTx_CascadesAndRevokesCodeInOneTx(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	c1 := insertChar(t, ctx, db, ownerID, "Charone", false)
	c2 := insertChar(t, ctx, db, ownerID, "Chartwo", true)
	insertGuildCode(t, ctx, db, ownerID, "code-A")

	// A second owner whose data must be untouched by the eviction.
	otherID := insertOwner(t, ctx, db, "Guildie-B")
	cOther := insertChar(t, ctx, db, otherID, "Otherchar", false)
	insertGuildCode(t, ctx, db, otherID, "code-B")

	var removedCount int
	var graceUntil int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, graceUntil, e = EvictOwnerTx(ctx, tx, ownerID, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}

	if removedCount != 2 {
		t.Errorf("removedCount = %d, want 2 (both of owner A's chars)", removedCount)
	}
	if graceUntil != now+EvictionGraceSeconds {
		t.Errorf("graceUntil = %d, want %d (now + 30d)", graceUntil, now+EvictionGraceSeconds)
	}

	// BOTH of owner A's characters are is_removed=1 with grace_until set.
	for _, id := range []int64{c1, c2} {
		isRemoved, grace, _ := charState(t, ctx, db, id)
		if isRemoved != 1 {
			t.Errorf("char %d is_removed = %d, want 1", id, isRemoved)
		}
		if !grace.Valid || grace.Int64 != graceUntil {
			t.Errorf("char %d grace_until = %v, want %d", id, grace, graceUntil)
		}
	}

	// Owner A's guild_code is revoked (disabled_at set) — D-10, SAME tx.
	var disabledA sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT disabled_at FROM guild_code WHERE owner_id = ?`, ownerID).Scan(&disabledA); err != nil {
		t.Fatalf("read owner A code: %v", err)
	}
	if !disabledA.Valid || disabledA.String == "" {
		t.Errorf("owner A guild_code.disabled_at = %v, want a non-empty timestamp (D-10 revoke)", disabledA)
	}

	// Owner B is completely untouched: char live, code active.
	isRemovedOther, _, _ := charState(t, ctx, db, cOther)
	if isRemovedOther != 0 {
		t.Errorf("owner B char is_removed = %d, want 0 (must be untouched)", isRemovedOther)
	}
	var disabledB sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT disabled_at FROM guild_code WHERE owner_id = ?`, otherID).Scan(&disabledB); err != nil {
		t.Fatalf("read owner B code: %v", err)
	}
	if disabledB.Valid {
		t.Errorf("owner B guild_code.disabled_at = %v, want NULL (must be untouched)", disabledB)
	}
}

func TestEvictOwnerTx_AlreadyRemovedIsNoOpCount(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	insertChar(t, ctx, db, ownerID, "Charone", false)

	// First eviction flips 1.
	var n int
	_ = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		n, _, e = EvictOwnerTx(ctx, tx, ownerID, now)
		return e
	})
	if n != 1 {
		t.Fatalf("first evict count = %d, want 1", n)
	}
	// Second eviction flips 0 (already removed).
	_ = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		n, _, e = EvictOwnerTx(ctx, tx, ownerID, now+1)
		return e
	})
	if n != 0 {
		t.Errorf("second evict count = %d, want 0 (idempotent on already-removed)", n)
	}
}

func TestRestoreOwnerTx_ReversesDuringGrace(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	c1 := insertChar(t, ctx, db, ownerID, "Charone", false)

	_ = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, ownerID, now)
		return e
	})

	var restored int
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		restored, e = RestoreOwnerTx(ctx, tx, ownerID, now+100)
		return e
	}); err != nil {
		t.Fatalf("RestoreOwnerTx: %v", err)
	}
	if restored != 1 {
		t.Errorf("restoredCount = %d, want 1", restored)
	}
	isRemoved, grace, _ := charState(t, ctx, db, c1)
	if isRemoved != 0 {
		t.Errorf("after restore is_removed = %d, want 0", isRemoved)
	}
	if grace.Valid {
		t.Errorf("after restore grace_until = %v, want NULL (cleared)", grace)
	}
}

func TestRestoreOwnerTx_DoesNotRestoreArchived(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	c1 := insertChar(t, ctx, db, ownerID, "Charone", false)

	// Evict with a grace already in the past, then archive.
	_ = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, ownerID, now-EvictionGraceSeconds-10)
		return e
	})
	archived, err := ArchiveExpiredEvictions(ctx, db, now)
	if err != nil {
		t.Fatalf("ArchiveExpiredEvictions: %v", err)
	}
	if archived != 1 {
		t.Fatalf("archived = %d, want 1", archived)
	}

	// Restore must NOT bring back the archived character.
	restored, err := func() (int, error) {
		var r int
		e := commitTx(t, ctx, db, func(tx *sql.Tx) error {
			var ee error
			r, ee = RestoreOwnerTx(ctx, tx, ownerID, now+1)
			return ee
		})
		return r, e
	}()
	if err != nil {
		t.Fatalf("RestoreOwnerTx (post-archive): %v", err)
	}
	if restored != 0 {
		t.Errorf("restoredCount = %d, want 0 (archived chars are not restored)", restored)
	}
	isRemoved, _, arch := charState(t, ctx, db, c1)
	if isRemoved != 1 || !arch.Valid {
		t.Errorf("archived char state: is_removed=%d archived_at=%v, want 1 / valid", isRemoved, arch)
	}
}

func TestArchiveExpiredEvictions_Idempotent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	insertChar(t, ctx, db, ownerID, "Charone", false)

	// Evict with grace in the past.
	_ = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, ownerID, now-EvictionGraceSeconds-10)
		return e
	})

	first, err := ArchiveExpiredEvictions(ctx, db, now)
	if err != nil {
		t.Fatalf("first archive: %v", err)
	}
	if first != 1 {
		t.Errorf("first archive = %d, want 1", first)
	}
	// Second run archives 0 (already archived — idempotent).
	second, err := ArchiveExpiredEvictions(ctx, db, now+1)
	if err != nil {
		t.Fatalf("second archive: %v", err)
	}
	if second != 0 {
		t.Errorf("second archive = %d, want 0 (idempotent)", second)
	}
}

func TestListEvictableOwners_AndPreview(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerA := insertOwner(t, ctx, db, "Guildie-A")
	insertChar(t, ctx, db, ownerA, "Aone", false)
	insertChar(t, ctx, db, ownerA, "Atwo", false)

	// Owner B has only removed chars → excluded from the evictable list.
	ownerB := insertOwner(t, ctx, db, "Guildie-B")
	cb := insertChar(t, ctx, db, ownerB, "Bone", false)
	if _, err := db.ExecContext(ctx, `UPDATE character SET is_removed = 1 WHERE id = ?`, cb); err != nil {
		t.Fatalf("mark B removed: %v", err)
	}

	owners, err := ListEvictableOwners(ctx, db)
	if err != nil {
		t.Fatalf("ListEvictableOwners: %v", err)
	}
	if len(owners) != 1 || owners[0].OwnerID != ownerA || owners[0].CharCount != 2 {
		t.Errorf("ListEvictableOwners = %+v, want only owner A with char_count 2", owners)
	}

	names, err := PreviewEviction(ctx, db, ownerA)
	if err != nil {
		t.Fatalf("PreviewEviction: %v", err)
	}
	if len(names) != 2 || names[0] != "Aone" || names[1] != "Atwo" {
		t.Errorf("PreviewEviction = %v, want [Aone Atwo] (sorted)", names)
	}
}
