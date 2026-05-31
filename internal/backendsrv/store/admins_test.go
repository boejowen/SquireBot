package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// admins_test.go covers the v1 admin.ts port (15-01 Task 3): fail-closed
// IsOfficer, idempotent AddOfficerTx/RemoveOfficerTx, authorize-under-transaction
// (WR-04), owner-floor protection (a peer cannot remove the floor), and the
// self-removal orphan rule. Behavioral oracle: apps-script/src/lib/admin.ts.

// insertWebUser is a test helper inserting a minimal web_user row (the FK target
// for guild_admins).
func insertWebUser(t *testing.T, ctx context.Context, db *sql.DB, id, username string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, id, username); err != nil {
		t.Fatalf("insert web_user %q: %v", id, err)
	}
}

// commitTx runs fn inside a transaction, COMMITTING on success (so post-tx
// assertions can read the committed state). Distinct from enrich_test.go's
// rollback-only withTx. Mirrors how the 15-02/15-03 handlers compose the *Tx
// mutators: open tx → call the *Tx method → commit.
func commitTx(t *testing.T, ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if ferr := fn(tx); ferr != nil {
		_ = tx.Rollback()
		return ferr
	}
	if cerr := tx.Commit(); cerr != nil {
		t.Fatalf("commit tx: %v", cerr)
	}
	return nil
}

func TestIsOfficer_FailClosed(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// Empty id → false, no query.
	if ok, err := IsOfficer(ctx, db, ""); err != nil || ok {
		t.Errorf("IsOfficer(\"\") = (%v, %v), want (false, nil)", ok, err)
	}
	// Unknown id → false.
	if ok, err := IsOfficer(ctx, db, "999"); err != nil || ok {
		t.Errorf("IsOfficer(unknown) = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestSetOwnerFloor_SeedsConfigAndBootstrapOfficer(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"

	// Floor has NOT logged in yet — SetOwnerFloor must still succeed (placeholder
	// web_user) and make the floor the bootstrap officer.
	if err := SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}

	got, err := GetOwnerFloor(ctx, db)
	if err != nil {
		t.Fatalf("GetOwnerFloor: %v", err)
	}
	if got != floor {
		t.Errorf("GetOwnerFloor = %q, want %q", got, floor)
	}
	// Floor is the bootstrap officer.
	if ok, err := IsOfficer(ctx, db, floor); err != nil || !ok {
		t.Errorf("IsOfficer(floor) = (%v, %v), want (true, nil)", ok, err)
	}
	// Floor shows in ListOfficers with IsFloor=true.
	officers, err := ListOfficers(ctx, db)
	if err != nil {
		t.Fatalf("ListOfficers: %v", err)
	}
	if len(officers) != 1 || officers[0].DiscordUserID != floor || !officers[0].IsFloor {
		t.Errorf("ListOfficers = %+v, want one floor officer with IsFloor=true", officers)
	}

	// Idempotent: a second SetOwnerFloor (e.g. re-run) does not duplicate.
	if err := SetOwnerFloor(ctx, db, floor, 1700000001); err != nil {
		t.Fatalf("SetOwnerFloor (second): %v", err)
	}
	officers, _ = ListOfficers(ctx, db)
	if len(officers) != 1 {
		t.Errorf("after re-seed, ListOfficers len = %d, want 1", len(officers))
	}
}

func TestAddOfficerTx_AuthorizeAndIdempotent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	target := "222222222222222222"
	stranger := "333333333333333333"

	if err := SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}
	insertWebUser(t, ctx, db, target, "TargetToon")
	insertWebUser(t, ctx, db, stranger, "Stranger")

	// A non-officer caller is rejected (fail-closed, authorize-under-tx).
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddOfficerTx(ctx, tx, target, stranger, 1700000001)
		return e
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("AddOfficerTx by non-officer: err = %v, want ErrNotAuthorized", err)
	}
	// Target was NOT added.
	if ok, _ := IsOfficer(ctx, db, target); ok {
		t.Errorf("target became officer despite unauthorized caller")
	}

	// The floor (an officer) promotes the target → added=true.
	var added bool
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		added, e = AddOfficerTx(ctx, tx, target, floor, 1700000002)
		return e
	})
	if err != nil {
		t.Fatalf("AddOfficerTx by floor: %v", err)
	}
	if !added {
		t.Errorf("AddOfficerTx added = false, want true (fresh promotion)")
	}
	if ok, _ := IsOfficer(ctx, db, target); !ok {
		t.Errorf("target is not an officer after promotion")
	}

	// Idempotent: promoting again → added=false, no duplicate.
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		added, e = AddOfficerTx(ctx, tx, target, floor, 1700000003)
		return e
	})
	if err != nil {
		t.Fatalf("AddOfficerTx (idempotent): %v", err)
	}
	if added {
		t.Errorf("AddOfficerTx added = true on re-add, want false (idempotent)")
	}
}

func TestRemoveOfficerTx_OwnerFloorProtectedAndIdempotent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	peer := "222222222222222222"

	if err := SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}
	insertWebUser(t, ctx, db, peer, "PeerOfficer")
	// Promote peer to officer.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddOfficerTx(ctx, tx, peer, floor, 1700000001)
		return e
	}); err != nil {
		t.Fatalf("promote peer: %v", err)
	}

	// A peer cannot remove the floor (owner-floor protection, BEFORE any write).
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := RemoveOfficerTx(ctx, tx, floor, peer, 1700000002)
		return e
	})
	if !errors.Is(err, ErrOwnerFloorProtected) {
		t.Errorf("peer removing floor: err = %v, want ErrOwnerFloorProtected", err)
	}
	// Floor is still an officer.
	if ok, _ := IsOfficer(ctx, db, floor); !ok {
		t.Errorf("floor was removed despite owner-floor protection")
	}

	// A non-officer caller is rejected before the floor check even matters.
	insertWebUser(t, ctx, db, "444444444444444444", "Nobody")
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := RemoveOfficerTx(ctx, tx, peer, "444444444444444444", 1700000003)
		return e
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer removing peer: err = %v, want ErrNotAuthorized", err)
	}

	// The floor CAN remove the peer → removed=true.
	var removed bool
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOfficerTx(ctx, tx, peer, floor, 1700000004)
		return e
	})
	if err != nil {
		t.Fatalf("floor removing peer: %v", err)
	}
	if !removed {
		t.Errorf("RemoveOfficerTx removed = false, want true")
	}
	if ok, _ := IsOfficer(ctx, db, peer); ok {
		t.Errorf("peer still an officer after removal")
	}

	// Idempotent: removing the (now absent) peer again → removed=false, no error.
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOfficerTx(ctx, tx, peer, floor, 1700000005)
		return e
	})
	if err != nil {
		t.Fatalf("RemoveOfficerTx (idempotent): %v", err)
	}
	if removed {
		t.Errorf("RemoveOfficerTx removed = true on absent target, want false")
	}
}

func TestRemoveOfficerTx_SelfRemovalOfFloorLeavesPointer(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"

	if err := SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}

	// Self-removal of the floor is allowed (v1's documented orphan rule).
	var removed bool
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOfficerTx(ctx, tx, floor, floor, 1700000001)
		return e
	})
	if err != nil {
		t.Fatalf("floor self-removal: %v", err)
	}
	if !removed {
		t.Errorf("self-removal removed = false, want true")
	}
	// The floor pointer (app_config) is INTENTIONALLY left intact (orphan).
	got, _ := GetOwnerFloor(ctx, db)
	if got != floor {
		t.Errorf("owner-floor pointer = %q after self-removal, want %q (documented orphan)", got, floor)
	}
	// But the floor is no longer in guild_admins.
	if ok, _ := IsOfficer(ctx, db, floor); ok {
		t.Errorf("floor still in guild_admins after self-removal")
	}
}

func TestListPromotableUsers_ExcludesOfficers(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	candidate := "222222222222222222"

	if err := SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}
	insertWebUser(t, ctx, db, candidate, "Candidate")

	promotable, err := ListPromotableUsers(ctx, db)
	if err != nil {
		t.Fatalf("ListPromotableUsers: %v", err)
	}
	// The candidate (not an officer) is listed; the floor (an officer) is not.
	var sawCandidate, sawFloor bool
	for _, p := range promotable {
		if p.DiscordUserID == candidate {
			sawCandidate = true
		}
		if p.DiscordUserID == floor {
			sawFloor = true
		}
	}
	if !sawCandidate {
		t.Errorf("ListPromotableUsers omitted the non-officer candidate: %+v", promotable)
	}
	if sawFloor {
		t.Errorf("ListPromotableUsers included the floor (already an officer): %+v", promotable)
	}
}
