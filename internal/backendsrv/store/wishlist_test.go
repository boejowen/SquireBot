package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// wishlist_test.go covers the Phase 34 owner-scoped wishlist store funcs (34-01
// Task 2): AddWishlistTx (returns the new id; returns the TYPED ErrDuplicateWishlist
// on a unique-index conflict, detected via *sqlite.Error.Code()==2067 — NEVER a raw
// driver error and NEVER a string-match), ListOwnWishlist (owner- AND char-scoped,
// active-only, non-nil slice), RemoveOwnWishlistTx / SetPingedTx (owner-scoped;
// cross-owner is a silent (false,nil) IDOR no-op), and AlertedWishlistIDs (the
// owner-scoped EC-hit badge set). Identity keys on discord_user_id. Reuses
// insertWebUser / commitTx (admins_test.go) + insertOwner / insertChar
// (eviction_test.go) + i64ptr (itemids_test.go) + NewTestDB (testhelper.go).

// listWishlist is a small read helper around ListOwnWishlist for the assertions below.
func listWishlist(t *testing.T, ctx context.Context, db *sql.DB, discordID, char string) []WishlistTargetRow {
	t.Helper()
	rows, err := ListOwnWishlist(ctx, db, discordID, char)
	if err != nil {
		t.Fatalf("ListOwnWishlist(%q, %q): %v", discordID, char, err)
	}
	return rows
}

func TestAddWishlistTx_InsertsActiveRowAndReturnsID(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	itemID := int64(1001)
	var newID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		newID, e = AddWishlistTx(ctx, tx, "disc-1", charID, "Primary", &itemID, "Rusty Dagger", 1700)
		return e
	}); err != nil {
		t.Fatalf("AddWishlistTx: %v", err)
	}
	if newID <= 0 {
		t.Fatalf("AddWishlistTx returned non-positive id %d", newID)
	}

	got := listWishlist(t, ctx, db, "disc-1", "Slampeach")
	if len(got) != 1 {
		t.Fatalf("ListOwnWishlist len = %d, want 1", len(got))
	}
	r := got[0]
	if r.ID != newID || r.ItemID == nil || *r.ItemID != itemID || r.ItemName != "Rusty Dagger" ||
		r.Slot != "Primary" || r.CharacterID != charID || r.CreatedAt != 1700 {
		t.Errorf("ListOwnWishlist row = %+v, want the inserted fields", r)
	}
	// pinged defaults ON (the DDL DEFAULT 1, Pitfall 8).
	if !r.Pinged {
		t.Errorf("fresh wishlist target Pinged = false, want true (DEFAULT 1, default-ON)")
	}
}

func TestListOwnWishlist_OwnerAndCharScopedActiveOnlyNonNil(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")
	aliceOwner := insertOwner(t, ctx, db, "Alice-Owner")
	aliceChar := insertChar(t, ctx, db, aliceOwner, "Slampeach", false)
	bobOwner := insertOwner(t, ctx, db, "Bob-Owner")
	bobChar := insertChar(t, ctx, db, bobOwner, "Bobtoon", false)

	// Empty wishlist → non-nil empty slice (JSON []).
	if got := listWishlist(t, ctx, db, "disc-1", "Slampeach"); got == nil {
		t.Fatalf("ListOwnWishlist on empty list returned nil, want non-nil empty slice")
	} else if len(got) != 0 {
		t.Fatalf("ListOwnWishlist on empty list len = %d, want 0", len(got))
	}

	i1, i2 := int64(10), int64(20)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		if _, e := AddWishlistTx(ctx, tx, "disc-1", aliceChar, "Head", &i1, "Item Ten", 1); e != nil {
			return e
		}
		if _, e := AddWishlistTx(ctx, tx, "disc-1", aliceChar, "Chest", &i2, "Item Twenty", 2); e != nil {
			return e
		}
		// Bob's target must NOT appear in Alice's list (owner scoping).
		if _, e := AddWishlistTx(ctx, tx, "disc-2", bobChar, "Head", &i1, "Item Ten", 3); e != nil {
			return e
		}
		return nil
	}); err != nil {
		t.Fatalf("seed targets: %v", err)
	}

	alice := listWishlist(t, ctx, db, "disc-1", "Slampeach")
	if len(alice) != 2 {
		t.Fatalf("Alice ListOwnWishlist len = %d, want 2 (Bob's must not leak)", len(alice))
	}
	// A wrong char name for the same owner returns nothing (char scoping).
	if got := listWishlist(t, ctx, db, "disc-1", "Bobtoon"); len(got) != 0 {
		t.Fatalf("Alice's wishlist filtered by Bob's char name len = %d, want 0 (char scoping)", len(got))
	}
	bob := listWishlist(t, ctx, db, "disc-2", "Bobtoon")
	if len(bob) != 1 {
		t.Fatalf("Bob ListOwnWishlist len = %d, want 1", len(bob))
	}
}

func TestRemoveOwnWishlistTx_SoftDeleteExcludesFromList(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	itemID := int64(55)
	var wishID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		wishID, e = AddWishlistTx(ctx, tx, "disc-1", charID, "Waist", &itemID, "Soft Me", 1)
		return e
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOwnWishlistTx(ctx, tx, wishID, "disc-1")
		return e
	}); err != nil {
		t.Fatalf("RemoveOwnWishlistTx: %v", err)
	}
	if !removed {
		t.Fatalf("RemoveOwnWishlistTx on own active target = false, want true")
	}
	if got := listWishlist(t, ctx, db, "disc-1", "Slampeach"); len(got) != 0 {
		t.Fatalf("after soft-remove, ListOwnWishlist len = %d, want 0", len(got))
	}
}

func TestRemoveOwnWishlistTx_CrossOwnerSilentNoOp(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	itemID := int64(77)
	var wishID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		wishID, e = AddWishlistTx(ctx, tx, "disc-1", charID, "Neck", &itemID, "Alice Only", 1)
		return e
	}); err != nil {
		t.Fatalf("seed Alice target: %v", err)
	}

	// Bob tries to remove Alice's target by id → IDOR-safe silent no-op (false, nil).
	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOwnWishlistTx(ctx, tx, wishID, "disc-2")
		return e
	}); err != nil {
		t.Fatalf("cross-owner RemoveOwnWishlistTx errored: %v (want silent no-op)", err)
	}
	if removed {
		t.Fatalf("cross-owner RemoveOwnWishlistTx removed = true, want false (IDOR no-op)")
	}
	// Alice's target is untouched.
	if got := listWishlist(t, ctx, db, "disc-1", "Slampeach"); len(got) != 1 {
		t.Fatalf("after cross-owner no-op, Alice ListOwnWishlist len = %d, want 1 (untouched)", len(got))
	}
}

func TestSetPingedTx_OwnerScopedAndReflectedInList(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	itemID := int64(99)
	var wishID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		wishID, e = AddWishlistTx(ctx, tx, "disc-1", charID, "Finger1", &itemID, "Bell Me", 1)
		return e
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	// Bob tries to silence Alice's target → IDOR-safe silent no-op (false, nil).
	var ok bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		ok, e = SetPingedTx(ctx, tx, wishID, "disc-2", false)
		return e
	}); err != nil {
		t.Fatalf("cross-owner SetPingedTx errored: %v (want silent no-op)", err)
	}
	if ok {
		t.Fatalf("cross-owner SetPingedTx = true, want false (IDOR no-op)")
	}
	if !listWishlist(t, ctx, db, "disc-1", "Slampeach")[0].Pinged {
		t.Fatalf("after cross-owner no-op, Alice's target Pinged = false, want true (untouched, default-ON)")
	}

	// Alice silences her own target → (true, nil), and ListOwnWishlist reflects pinged=false.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		ok, e = SetPingedTx(ctx, tx, wishID, "disc-1", false)
		return e
	}); err != nil {
		t.Fatalf("SetPingedTx own: %v", err)
	}
	if !ok {
		t.Fatalf("SetPingedTx own target = false, want true")
	}
	if listWishlist(t, ctx, db, "disc-1", "Slampeach")[0].Pinged {
		t.Fatalf("after own silence, ListOwnWishlist Pinged = true, want false")
	}

	// Re-enable round-trips back to true.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := SetPingedTx(ctx, tx, wishID, "disc-1", true)
		return e
	}); err != nil {
		t.Fatalf("SetPingedTx re-enable: %v", err)
	}
	if !listWishlist(t, ctx, db, "disc-1", "Slampeach")[0].Pinged {
		t.Errorf("after re-enable, ListOwnWishlist Pinged = false, want true")
	}
}

func TestAddWishlistTx_DuplicateReturnsTypedSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	itemID := int64(42)
	// First add succeeds.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWishlistTx(ctx, tx, "disc-1", charID, "Primary", &itemID, "Dup Item", 1)
		return e
	}); err != nil {
		t.Fatalf("first AddWishlistTx: %v", err)
	}

	// Exact (owner, char, slot, item) re-add → TYPED ErrDuplicateWishlist (the
	// wishlist_catalog_uidx partial index).
	addErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWishlistTx(ctx, tx, "disc-1", charID, "Primary", &itemID, "Dup Item", 2)
		return e
	})
	if !errors.Is(addErr, ErrDuplicateWishlist) {
		t.Fatalf("duplicate (user,char,slot,item) AddWishlistTx err = %v, want ErrDuplicateWishlist", addErr)
	}

	// The SAME item in a DIFFERENT slot is distinct (slot is part of the key).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWishlistTx(ctx, tx, "disc-1", charID, "Secondary", &itemID, "Dup Item", 3)
		return e
	}); err != nil {
		t.Errorf("same item in a different slot should NOT collide, got: %v", err)
	}

	if got := listWishlist(t, ctx, db, "disc-1", "Slampeach"); len(got) != 2 {
		t.Fatalf("after dup re-add + different-slot add, ListOwnWishlist len = %d, want 2", len(got))
	}
}

func TestAddWishlistTx_CustomDuplicateReturnsTypedSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	// First custom target (item_id NULL, dedupe on item_name in the slot).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWishlistTx(ctx, tx, "disc-1", charID, "Range", nil, "Homemade Label", 1)
		return e
	}); err != nil {
		t.Fatalf("first custom AddWishlistTx: %v", err)
	}

	// Exact (owner, char, slot, label) re-add of a custom target → ErrDuplicateWishlist
	// (the wishlist_custom_uidx partial index; NULL item_id is dedupe-safe).
	addErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWishlistTx(ctx, tx, "disc-1", charID, "Range", nil, "Homemade Label", 2)
		return e
	})
	if !errors.Is(addErr, ErrDuplicateWishlist) {
		t.Fatalf("duplicate custom (user,char,slot,label) AddWishlistTx err = %v, want ErrDuplicateWishlist", addErr)
	}
}

// TestAlertedWishlistIDs_OwnerScopedSet proves AlertedWishlistIDs returns the set of
// wishlist ids the caller has an alert_log row for (WISH-05 EC-hit badge), keyed on
// the alert_log.wantlist_item_id column (00014 kept the name, repointed the FK to
// wishlist_item) — owner-scoped, non-nil map, NULL-FK rows excluded.
func TestAlertedWishlistIDs_OwnerScopedSet(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	// Empty → non-nil empty map.
	if got, err := AlertedWishlistIDs(ctx, db, "disc-1"); err != nil {
		t.Fatalf("AlertedWishlistIDs (empty): %v", err)
	} else if got == nil {
		t.Fatalf("AlertedWishlistIDs on no alerts returned nil, want non-nil empty map")
	} else if len(got) != 0 {
		t.Fatalf("AlertedWishlistIDs on no alerts len = %d, want 0", len(got))
	}

	item := int64(123)
	var hitID, noHitID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		hitID, e = AddWishlistTx(ctx, tx, "disc-1", charID, "Primary", &item, "Hit Item", 1)
		if e != nil {
			return e
		}
		noHitID, e = AddWishlistTx(ctx, tx, "disc-1", charID, "Secondary", &item, "No Hit Item", 2)
		return e
	}); err != nil {
		t.Fatalf("seed targets: %v", err)
	}

	// Seed an alert_log row FK'ing hitID for Alice, a NULL-FK test-alert for Alice
	// (must be excluded), and a row for Bob (must not leak into Alice's set).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, sent_at, send_status)
		 VALUES (?, 'disc-1', 'ec_auction', 0, 'sent'),
		        (NULL, 'disc-1', 'test', 0, 'sent'),
		        (?, 'disc-2', 'ec_auction', 0, 'sent')`, hitID, hitID); err != nil {
		t.Fatalf("seed alert_log: %v", err)
	}

	got, err := AlertedWishlistIDs(ctx, db, "disc-1")
	if err != nil {
		t.Fatalf("AlertedWishlistIDs: %v", err)
	}
	if !got[hitID] {
		t.Errorf("AlertedWishlistIDs missing the alerted id %d (have: %v)", hitID, got)
	}
	if got[noHitID] {
		t.Errorf("AlertedWishlistIDs includes a non-alerted id %d (have: %v)", noHitID, got)
	}
	if len(got) != 1 {
		t.Errorf("AlertedWishlistIDs len = %d, want 1 (NULL-FK + Bob's row excluded; have: %v)", len(got), got)
	}
}
