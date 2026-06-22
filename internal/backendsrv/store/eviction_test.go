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

// TestListRestorableOwners proves the inverse of ListEvictableOwners: only owners
// with >=1 character STILL IN GRACE (evicted, grace not yet expired, not archived)
// are returned — a live owner and a past-grace/archived owner are both EXCLUDED.
func TestListRestorableOwners(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	// (1) LIVE owner — never evicted. Must be EXCLUDED (nothing in grace).
	live := insertOwner(t, ctx, db, "Live-Owner")
	insertChar(t, ctx, db, live, "Livetoon", false)

	// (2) IN-GRACE owner — evicted now, grace runs to now+30d. The two chars must
	//     both be counted; grace_until is the (single shared) deadline.
	inGrace := insertOwner(t, ctx, db, "Grace-Owner")
	insertChar(t, ctx, db, inGrace, "Graceone", false)
	insertChar(t, ctx, db, inGrace, "Gracetwo", false)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, inGrace, now)
		return e
	}); err != nil {
		t.Fatalf("evict in-grace owner: %v", err)
	}

	// (3) PAST-GRACE owner — evicted with grace already expired, then archived. Must
	//     be EXCLUDED (archived data is never restorable).
	expired := insertOwner(t, ctx, db, "Expired-Owner")
	insertChar(t, ctx, db, expired, "Expiredtoon", false)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, expired, now-EvictionGraceSeconds-10)
		return e
	}); err != nil {
		t.Fatalf("evict past-grace owner: %v", err)
	}
	if n, err := ArchiveExpiredEvictions(ctx, db, now); err != nil || n != 1 {
		t.Fatalf("archive past-grace owner: n=%d err=%v, want 1/nil", n, err)
	}

	owners, err := ListRestorableOwners(ctx, db, now)
	if err != nil {
		t.Fatalf("ListRestorableOwners: %v", err)
	}
	if len(owners) != 1 {
		t.Fatalf("ListRestorableOwners = %+v, want exactly 1 (only the in-grace owner)", owners)
	}
	got := owners[0]
	if got.OwnerID != inGrace {
		t.Errorf("owner_id = %d, want %d (the in-grace owner; live + archived must be excluded)", got.OwnerID, inGrace)
	}
	if got.Label != "Grace-Owner" {
		t.Errorf("label = %q, want Grace-Owner", got.Label)
	}
	if got.CharCount != 2 {
		t.Errorf("char_count = %d, want 2", got.CharCount)
	}
	if got.GraceUntil != now+EvictionGraceSeconds {
		t.Errorf("grace_until = %d, want %d (now + 30d)", got.GraceUntil, now+EvictionGraceSeconds)
	}
}

// TestListEvictableOwners_ExcludesGuildSentinel proves the guild sentinel owner
// (GuildSentinelOwnerID, seeded by 00015) is NEVER offered as an evictable guildie,
// even when it holds live banks/bots — OWN-02: an officer can't pick "evict the guild
// bank". A normal live owner IS returned.
func TestListEvictableOwners_ExcludesGuildSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// A live sentinel-owned guild bank (the sentinel owner row exists via 00015).
	insertChar(t, ctx, db, GuildSentinelOwnerID, "Guildbank", true /*isBank*/)

	// A normal live owner + char.
	normal := insertOwner(t, ctx, db, "Guildie-A")
	insertChar(t, ctx, db, normal, "Normaltoon", false)

	owners, err := ListEvictableOwners(ctx, db)
	if err != nil {
		t.Fatalf("ListEvictableOwners: %v", err)
	}
	for _, o := range owners {
		if o.OwnerID == GuildSentinelOwnerID {
			t.Errorf("ListEvictableOwners returned the guild sentinel owner %d (OWN-02 — must be excluded)", GuildSentinelOwnerID)
		}
	}
	if len(owners) != 1 || owners[0].OwnerID != normal {
		t.Errorf("ListEvictableOwners = %+v, want only the normal owner %d", owners, normal)
	}
}

// TestListRestorableOwners_ExcludesGuildSentinel proves the guild sentinel owner is
// NEVER offered as a restorable guildie, even with a sentinel-owned char forced into
// the in-grace state — the list must never surface the sentinel (OWN-02). A normal
// in-grace owner IS returned.
func TestListRestorableOwners_ExcludesGuildSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	// A sentinel-owned char forced into the in-grace state directly via SQL (it should
	// never reach this state through the app, but the list must still exclude it).
	sentChar := insertChar(t, ctx, db, GuildSentinelOwnerID, "Guildbank", true)
	if _, err := db.ExecContext(ctx,
		`UPDATE character SET is_removed = 1, grace_until = ?, archived_at = NULL WHERE id = ?`,
		now+EvictionGraceSeconds, sentChar); err != nil {
		t.Fatalf("force sentinel char in-grace: %v", err)
	}

	// A normal in-grace owner (evicted now).
	normal := insertOwner(t, ctx, db, "Grace-Owner")
	insertChar(t, ctx, db, normal, "Graceone", false)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, normal, now)
		return e
	}); err != nil {
		t.Fatalf("evict normal owner: %v", err)
	}

	owners, err := ListRestorableOwners(ctx, db, now)
	if err != nil {
		t.Fatalf("ListRestorableOwners: %v", err)
	}
	for _, o := range owners {
		if o.OwnerID == GuildSentinelOwnerID {
			t.Errorf("ListRestorableOwners returned the guild sentinel owner %d (OWN-02 — must be excluded)", GuildSentinelOwnerID)
		}
	}
	if len(owners) != 1 || owners[0].OwnerID != normal {
		t.Errorf("ListRestorableOwners = %+v, want only the normal in-grace owner %d", owners, normal)
	}
}

// TestEvictOwnerTx_GuildBankSurvivesEviction is THE OWN-02 PROOF: evicting a real
// guildie flips is_removed=1 only on THEIR own characters; a sentinel-owned guild bank
// (owner_id = GuildSentinelOwnerID) is left is_removed=0 with grace_until NULL — it
// survives the eviction by construction (the cascade is `WHERE owner_id = realOwner`,
// which never matches the sentinel id). removedCount counts only the real owner's char.
func TestEvictOwnerTx_GuildBankSurvivesEviction(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	// A real guildie with one normal char + a guild code.
	realOwner := insertOwner(t, ctx, db, "Guildie-A")
	normalChar := insertChar(t, ctx, db, realOwner, "Normaltoon", false)
	insertGuildCode(t, ctx, db, realOwner, "code-A")

	// A sentinel-owned guild bank (the surviving resource).
	bankChar := insertChar(t, ctx, db, GuildSentinelOwnerID, "Guildbank", true)

	var removedCount int
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, _, e = EvictOwnerTx(ctx, tx, realOwner, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}

	// The cascade flipped ONLY the real owner's char.
	if removedCount != 1 {
		t.Errorf("removedCount = %d, want 1 (only the real owner's char, NOT the bank)", removedCount)
	}
	if isRemoved, _, _ := charState(t, ctx, db, normalChar); isRemoved != 1 {
		t.Errorf("real owner's char is_removed = %d, want 1 (cascade worked)", isRemoved)
	}

	// OWN-02: the sentinel-owned bank SURVIVES — still live, no grace stamp.
	isRemovedBank, graceBank, _ := charState(t, ctx, db, bankChar)
	if isRemovedBank != 0 {
		t.Errorf("guild bank is_removed = %d, want 0 (OWN-02 — bank survives the eviction)", isRemovedBank)
	}
	if graceBank.Valid {
		t.Errorf("guild bank grace_until = %v, want NULL (untouched by the eviction)", graceBank)
	}
}
