package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

// insertCrossOwnerWrite appends a cross_owner_write audit row — the SHARING
// signal Phase 36 reads (OWN-03 / D-01): another guildie (attemptingOwner)
// uploaded charName whose recorded steward at write time was currentOwner. A char
// is SHARED iff some such row exists with attempting_owner_id <> the evicted owner.
func insertCrossOwnerWrite(t *testing.T, ctx context.Context, db *sql.DB, charName string, attemptingOwner, currentOwner int64) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO audit_log (event, char_name, attempting_owner_id, current_owner_id)
		 VALUES ('cross_owner_write', ?, ?, ?)`,
		charName, attemptingOwner, currentOwner); err != nil {
		t.Fatalf("insert cross_owner_write (char=%q): %v", charName, err)
	}
}

// ownerCodeDisabled reports whether the owner's guild_code has been revoked
// (disabled_at set). Used by the all-shared-owner edge-case test. (Distinct from
// linking_test.go's label-keyed codeDisabled.)
func ownerCodeDisabled(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64) bool {
	t.Helper()
	var disabled sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT disabled_at FROM guild_code WHERE owner_id = ?`, ownerID).Scan(&disabled); err != nil {
		t.Fatalf("read code disabled_at (owner=%d): %v", ownerID, err)
	}
	return disabled.Valid && disabled.String != ""
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

// TestEvictOwnerTx_RefusesSentinel is the CR-01 fix proof: targeting the guild
// sentinel owner DIRECTLY (the directly-POST-able owner_id=1000000 path) is refused
// at the destructive WRITE boundary with ErrCannotEvictSentinel — the picker-list
// exclusion alone was bypassable. The sentinel-owned bank must be left untouched
// (is_removed=0, grace NULL) and NO characters flipped.
func TestEvictOwnerTx_RefusesSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	// A live sentinel-owned guild bank (the sentinel owner row exists via 00015).
	bankChar := insertChar(t, ctx, db, GuildSentinelOwnerID, "Guildbank", true /*isBank*/)

	var removedCount int
	var graceUntil int64
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, graceUntil, e = EvictOwnerTx(ctx, tx, GuildSentinelOwnerID, now)
		return e
	})
	if !errors.Is(err, ErrCannotEvictSentinel) {
		t.Fatalf("EvictOwnerTx(sentinel): err = %v, want ErrCannotEvictSentinel (OWN-02 write-path guard)", err)
	}
	if removedCount != 0 || graceUntil != 0 {
		t.Errorf("EvictOwnerTx(sentinel) returned removedCount=%d graceUntil=%d, want 0/0 (no write)", removedCount, graceUntil)
	}

	// The bank is untouched: still live, no grace stamp.
	isRemoved, grace, _ := charState(t, ctx, db, bankChar)
	if isRemoved != 0 {
		t.Errorf("guild bank is_removed = %d, want 0 (sentinel evict refused — bank survives)", isRemoved)
	}
	if grace.Valid {
		t.Errorf("guild bank grace_until = %v, want NULL (sentinel evict refused — untouched)", grace)
	}
}

// TestRestoreOwnerTx_RefusesSentinel is the symmetric CR-01 fix proof for the restore
// write path: targeting the sentinel owner directly is refused with
// ErrCannotEvictSentinel (return 0 restored, no write). Even a sentinel-owned char
// forced into the in-grace state (which should never happen via the app) is NOT
// touched by a sentinel-targeted restore.
func TestRestoreOwnerTx_RefusesSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	// Force a sentinel-owned char into the in-grace state directly via SQL (it should
	// never reach this state through the app; the guard must still refuse the restore).
	sentChar := insertChar(t, ctx, db, GuildSentinelOwnerID, "Guildbank", true)
	if _, err := db.ExecContext(ctx,
		`UPDATE character SET is_removed = 1, grace_until = ?, archived_at = NULL WHERE id = ?`,
		now+EvictionGraceSeconds, sentChar); err != nil {
		t.Fatalf("force sentinel char in-grace: %v", err)
	}

	var restored int
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		restored, e = RestoreOwnerTx(ctx, tx, GuildSentinelOwnerID, now+100)
		return e
	})
	if !errors.Is(err, ErrCannotEvictSentinel) {
		t.Fatalf("RestoreOwnerTx(sentinel): err = %v, want ErrCannotEvictSentinel (OWN-02 write-path guard)", err)
	}
	if restored != 0 {
		t.Errorf("RestoreOwnerTx(sentinel) restored=%d, want 0 (no write)", restored)
	}

	// The forced-in-grace sentinel char is unchanged (still is_removed=1, grace set):
	// the guard short-circuits before the UPDATE.
	isRemoved, grace, _ := charState(t, ctx, db, sentChar)
	if isRemoved != 1 {
		t.Errorf("sentinel char is_removed = %d, want 1 (restore refused — left as-is)", isRemoved)
	}
	if !grace.Valid {
		t.Errorf("sentinel char grace_until = %v, want non-NULL (restore refused — left as-is)", grace)
	}
}

// TestEvictOwnerTx_SharedCharSurvives is THE OWN-03 PROOF: evicting a guildie X who
// stewards both a SOLE-OWNED char and a SHARED char (another guildie Y has a
// cross_owner_write row for it) removes ONLY the sole-owned char — the shared char
// stays is_removed=0 with grace_until NULL (it survives because Y still plays it).
// removedCount counts only the flipped sole-owned char; X's guild_code is still revoked.
func TestEvictOwnerTx_SharedCharSurvives(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	soleChar := insertChar(t, ctx, db, ownerX, "Soletoon", false)
	sharedChar := insertChar(t, ctx, db, ownerX, "Sharedtoon", false)
	insertGuildCode(t, ctx, db, ownerX, "code-X")

	// A second guildie Y who also uploads Sharedtoon → the cross_owner_write row.
	ownerY := insertOwner(t, ctx, db, "Guildie-Y")
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, ownerX)

	var removedCount int
	var graceUntil int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, graceUntil, e = EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}

	// removedCount counts ONLY the sole-owned char.
	if removedCount != 1 {
		t.Errorf("removedCount = %d, want 1 (only the sole-owned char, NOT the shared one)", removedCount)
	}

	// The sole-owned char is removed + grace-stamped exactly as before.
	if isRemoved, grace, _ := charState(t, ctx, db, soleChar); isRemoved != 1 || !grace.Valid || grace.Int64 != graceUntil {
		t.Errorf("Soletoon state = (is_removed=%d, grace=%v), want (1, %d)", isRemoved, grace, graceUntil)
	}

	// OWN-03: the SHARED char SURVIVES — still live, no grace stamp.
	isRemovedShared, graceShared, _ := charState(t, ctx, db, sharedChar)
	if isRemovedShared != 0 {
		t.Errorf("Sharedtoon is_removed = %d, want 0 (OWN-03 — shared char survives the eviction)", isRemovedShared)
	}
	if graceShared.Valid {
		t.Errorf("Sharedtoon grace_until = %v, want NULL (shared char untouched)", graceShared)
	}

	// X's guild_code is still revoked (the watcher stops uploading regardless).
	if !ownerCodeDisabled(t, ctx, db, ownerX) {
		t.Errorf("owner X guild_code not revoked, want disabled_at set")
	}
}

// TestEvictOwnerTx_AllSharedOwnerStillRevokesCode is the D-04 edge case: an owner
// whose ONLY live char is shared flips 0 chars (removedCount=0) but STILL has their
// guild_code revoked — the code revoke is unconditional on removedCount, so a
// departing all-shared member still has their watcher silenced.
func TestEvictOwnerTx_AllSharedOwnerStillRevokesCode(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	sharedChar := insertChar(t, ctx, db, ownerX, "Sharedtoon", false)
	insertGuildCode(t, ctx, db, ownerX, "code-X")

	ownerY := insertOwner(t, ctx, db, "Guildie-Y")
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, ownerX)

	var removedCount int
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, _, e = EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}

	if removedCount != 0 {
		t.Errorf("removedCount = %d, want 0 (every live char is shared)", removedCount)
	}
	if isRemoved, _, _ := charState(t, ctx, db, sharedChar); isRemoved != 0 {
		t.Errorf("Sharedtoon is_removed = %d, want 0 (untouched)", isRemoved)
	}
	if !ownerCodeDisabled(t, ctx, db, ownerX) {
		t.Errorf("owner X guild_code not revoked, want disabled_at set (revoke is unconditional on removedCount)")
	}
}

// TestEvictOwnerTx_SelfAttemptingRowIsNotShared pins the predicate's `<> X` guard: a
// cross_owner_write row whose attempting_owner_id == X (a same-owner self-write) must
// NOT mark the char shared — it is still removed on eviction.
func TestEvictOwnerTx_SelfAttemptingRowIsNotShared(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	edgeChar := insertChar(t, ctx, db, ownerX, "Edgetoon", false)
	insertGuildCode(t, ctx, db, ownerX, "code-X")

	// A cross_owner_write row attributed to X itself — NOT a sharing signal.
	insertCrossOwnerWrite(t, ctx, db, "Edgetoon", ownerX, ownerX)

	var removedCount int
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, _, e = EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}
	if removedCount != 1 {
		t.Errorf("removedCount = %d, want 1 (a row attributed only to X does NOT make the char shared)", removedCount)
	}
	if isRemoved, _, _ := charState(t, ctx, db, edgeChar); isRemoved != 1 {
		t.Errorf("Edgetoon is_removed = %d, want 1 (removed — self-write is not sharing)", isRemoved)
	}
}

// removedCharNames returns the set of currently is_removed=1 character names for an
// owner — used to prove PreviewEviction's list == the cascade's actual remove-set.
func removedCharNames(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64) map[string]bool {
	t.Helper()
	rows, err := db.QueryContext(ctx,
		`SELECT name FROM character WHERE owner_id = ? AND is_removed = 1`, ownerID)
	if err != nil {
		t.Fatalf("read removed char names (owner=%d): %v", ownerID, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan removed name: %v", err)
		}
		out[n] = true
	}
	return out
}

// TestPreviewEviction_OmitsSharedChars is THE PARITY PROOF: PreviewEviction(X) lists
// EXACTLY the chars EvictOwnerTx(X) removes (shared chars omitted), and a subsequent
// eviction's actual is_removed=1 set is byte-identical to the preview list — they are
// backed by the SAME sharedCharPredicate so they can never diverge (the CR-01 lesson).
func TestPreviewEviction_OmitsSharedChars(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	insertChar(t, ctx, db, ownerX, "Soletoon", false)
	insertChar(t, ctx, db, ownerX, "Sharedtoon", false)

	ownerY := insertOwner(t, ctx, db, "Guildie-Y")
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, ownerX)

	// The preview lists ONLY the sole-owned char.
	names, err := PreviewEviction(ctx, db, ownerX)
	if err != nil {
		t.Fatalf("PreviewEviction: %v", err)
	}
	if len(names) != 1 || names[0] != "Soletoon" {
		t.Fatalf("PreviewEviction = %v, want exactly [Soletoon] (Sharedtoon omitted)", names)
	}

	// The cascade's actual remove-set is identical to the preview list.
	var removedCount int
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removedCount, _, e = EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}
	if removedCount != 1 {
		t.Errorf("removedCount = %d, want 1", removedCount)
	}
	got := removedCharNames(t, ctx, db, ownerX)
	if len(got) != 1 || !got["Soletoon"] {
		t.Errorf("cascade removed-set = %v, want exactly {Soletoon} (== the preview list)", got)
	}
}

// TestCountPreservedShared_CountsSurvivors covers the survivor count off the SAME
// predicate: the mixed case (1 shared survivor), the all-shared owner (preview [] +
// count>0 — the signal the web reads), and the sole-owned-only owner (count 0, both
// chars previewed). For any owner, len(PreviewEviction) + CountPreservedShared == the
// live-char count.
func TestCountPreservedShared_CountsSurvivors(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerY := insertOwner(t, ctx, db, "Guildie-Y")

	// (1) Mixed: one sole-owned + one shared → count 1.
	mixed := insertOwner(t, ctx, db, "Mixed-Owner")
	insertChar(t, ctx, db, mixed, "Soletoon", false)
	insertChar(t, ctx, db, mixed, "Sharedtoon", false)
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, mixed)
	if n, err := CountPreservedShared(ctx, db, mixed); err != nil || n != 1 {
		t.Errorf("CountPreservedShared(mixed) = %d, err=%v, want 1/nil", n, err)
	}
	if names, _ := PreviewEviction(ctx, db, mixed); len(names) != 1 {
		t.Errorf("PreviewEviction(mixed) = %v, want len 1 (parity: 1 preview + 1 preserved == 2 live)", names)
	}

	// (2) All-shared: only one char, shared → preview [] + count 1 (the web signal).
	allShared := insertOwner(t, ctx, db, "AllShared-Owner")
	insertChar(t, ctx, db, allShared, "Onlyshared", false)
	insertCrossOwnerWrite(t, ctx, db, "Onlyshared", ownerY, allShared)
	if names, err := PreviewEviction(ctx, db, allShared); err != nil || len(names) != 0 {
		t.Errorf("PreviewEviction(allShared) = %v, err=%v, want []/nil (empty)", names, err)
	}
	if n, err := CountPreservedShared(ctx, db, allShared); err != nil || n != 1 {
		t.Errorf("CountPreservedShared(allShared) = %d, err=%v, want 1/nil", n, err)
	}

	// (3) Sole-owned only: no cross_owner_write rows → count 0, both chars previewed.
	sole := insertOwner(t, ctx, db, "Sole-Owner")
	insertChar(t, ctx, db, sole, "Atoon", false)
	insertChar(t, ctx, db, sole, "Btoon", false)
	if n, err := CountPreservedShared(ctx, db, sole); err != nil || n != 0 {
		t.Errorf("CountPreservedShared(sole) = %d, err=%v, want 0/nil", n, err)
	}
	if names, _ := PreviewEviction(ctx, db, sole); len(names) != 2 {
		t.Errorf("PreviewEviction(sole) = %v, want len 2 (no sharing → both previewed)", names)
	}
}

// ownerOfChar reads the current owner_id of the character named name.
func ownerOfChar(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(ctx,
		`SELECT owner_id FROM character WHERE name = ?`, name).Scan(&id); err != nil {
		t.Fatalf("read owner_id (char=%q): %v", name, err)
	}
	return id
}

// TestEvictOwnerTx_RepointsSurvivingSharedChar proves the D-03 repoint: a surviving
// shared char still stewarded by the evicted owner X is repointed to the remaining
// sharer (the cross_owner_write attempting_owner_id <> X). It still survives
// (is_removed=0) AND its owner_id moves off X to Y.
func TestEvictOwnerTx_RepointsSurvivingSharedChar(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	sharedChar := insertChar(t, ctx, db, ownerX, "Sharedtoon", false)

	ownerY := insertOwner(t, ctx, db, "Guildie-Y")
	// Y is a LIVE steward (owns a live char of their own) so the WR-02 live-steward
	// filter keeps Y as a valid repoint target.
	insertChar(t, ctx, db, ownerY, "Ymain", false)
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, ownerX)

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}

	// Survives.
	if isRemoved, _, _ := charState(t, ctx, db, sharedChar); isRemoved != 0 {
		t.Errorf("Sharedtoon is_removed = %d, want 0 (survives)", isRemoved)
	}
	// Repointed off X to the remaining sharer Y.
	if got := ownerOfChar(t, ctx, db, "Sharedtoon"); got != ownerY {
		t.Errorf("Sharedtoon owner_id = %d, want %d (repointed to the remaining sharer)", got, ownerY)
	}
}

// TestEvictOwnerTx_RepointPicksMostRecentSharer proves the repoint picks the
// MOST-RECENT (highest audit_log.id) other sharer: with two cross_owner_write rows
// for the char (earlier Y, later Z), the surviving char repoints to Z.
func TestEvictOwnerTx_RepointPicksMostRecentSharer(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	insertChar(t, ctx, db, ownerX, "Sharedtoon", false)
	ownerY := insertOwner(t, ctx, db, "Guildie-Y")
	ownerZ := insertOwner(t, ctx, db, "Guildie-Z")
	// Both candidate sharers are LIVE stewards (own a live char) so the WR-02 filter
	// keeps them eligible — this test isolates the most-recent-wins ordering.
	insertChar(t, ctx, db, ownerY, "Ymain", false)
	insertChar(t, ctx, db, ownerZ, "Zmain", false)

	// Earlier row → Y; later row (higher id) → Z. Both <> X.
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, ownerX)
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerZ, ownerX)

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}
	if got := ownerOfChar(t, ctx, db, "Sharedtoon"); got != ownerZ {
		t.Errorf("Sharedtoon owner_id = %d, want %d (the most-recent sharer wins)", got, ownerZ)
	}
}

// TestEvictOwnerTx_RepointSkipsEvictedSharer is the WR-02 proof: the repoint must
// skip a more-recent OTHER sharer who is THEMSELVES already evicted (owns no live
// char) and fall back to an earlier sharer who is still a live steward. Without the
// live-steward EXISTS filter the survivor would be repointed onto a dead steward.
//
// Setup: X owns Sharedtoon. Y (earlier cross_owner_write) is LIVE (owns Ymain). Z
// (later cross_owner_write — the most-recent, which the bare ORDER BY id DESC would
// pick) is EVICTED (their only char is is_removed=1). Evicting X must repoint
// Sharedtoon to Y, skipping the more-recent-but-dead Z.
func TestEvictOwnerTx_RepointSkipsEvictedSharer(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	ownerX := insertOwner(t, ctx, db, "Guildie-X")
	insertChar(t, ctx, db, ownerX, "Sharedtoon", false)

	ownerY := insertOwner(t, ctx, db, "Guildie-Y")
	insertChar(t, ctx, db, ownerY, "Ymain", false) // Y is a live steward.

	ownerZ := insertOwner(t, ctx, db, "Guildie-Z")
	zChar := insertChar(t, ctx, db, ownerZ, "Zmain", false)
	// Z is evicted: their only char is removed, so Z owns no live char.
	if _, err := db.ExecContext(ctx,
		`UPDATE character SET is_removed = 1 WHERE id = ?`, zChar); err != nil {
		t.Fatalf("evict Z's char: %v", err)
	}

	// Earlier row → Y (live); later row (higher id) → Z (evicted). The bare subquery
	// would pick Z; the live-steward filter must skip Z and pick Y.
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerY, ownerX)
	insertCrossOwnerWrite(t, ctx, db, "Sharedtoon", ownerZ, ownerX)

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := EvictOwnerTx(ctx, tx, ownerX, now)
		return e
	}); err != nil {
		t.Fatalf("EvictOwnerTx: %v", err)
	}
	if got := ownerOfChar(t, ctx, db, "Sharedtoon"); got != ownerY {
		t.Errorf("Sharedtoon owner_id = %d, want %d (skip the evicted most-recent sharer Z, pick live Y)", got, ownerY)
	}
}

// TestRepointSubquery_LocksPredicateToSharedPredicate is the WARNING lock: it pins
// the COMMON load-bearing tokens (event string, COLLATE NOCASE name match, the <> ?
// exclusion) to identical text across recentOtherSharerSubquery AND
// sharedCharPredicate, so a future edit cannot silently diverge the SHARED part of the
// repoint subquery from the shared-detection predicate.
//
// WR-02 re-scope: recentOtherSharerSubquery now intentionally carries ONE extra clause
// the predicate must NOT have — the live-steward EXISTS filter (don't repoint onto an
// evicted steward). sharedCharPredicate decides SURVIVAL (a char is shared regardless of
// whether the other sharer is still live), so it must stay filter-free. The test therefore
// (a) keeps the shared tokens locked in both and (b) separately locks the live-steward
// clause into the repoint subquery only — so neither the shared part nor the WR-02 fix can
// silently regress.
func TestRepointSubquery_LocksPredicateToSharedPredicate(t *testing.T) {
	tokens := []string{
		"a.event = 'cross_owner_write'",
		"a.char_name = character.name COLLATE NOCASE",
		"a.attempting_owner_id <> ?",
	}
	for _, tok := range tokens {
		if !strings.Contains(recentOtherSharerSubquery, tok) {
			t.Errorf("recentOtherSharerSubquery missing load-bearing token %q (drift from sharedCharPredicate)", tok)
		}
		if !strings.Contains(sharedCharPredicate, tok) {
			t.Errorf("sharedCharPredicate missing load-bearing token %q", tok)
		}
	}

	// WR-02: the live-steward filter is locked into the repoint subquery (so it cannot be
	// dropped) but must NOT leak into the survival predicate (which must count even an
	// evicted other sharer as making the char shared).
	const liveStewardClause = "AND EXISTS (SELECT 1 FROM character c2 WHERE c2.owner_id = a.attempting_owner_id AND c2.is_removed = 0)"
	if !strings.Contains(recentOtherSharerSubquery, liveStewardClause) {
		t.Errorf("recentOtherSharerSubquery missing the WR-02 live-steward clause %q", liveStewardClause)
	}
	if strings.Contains(sharedCharPredicate, liveStewardClause) {
		t.Errorf("sharedCharPredicate must NOT carry the live-steward filter (it decides survival, not stewardship)")
	}
}
