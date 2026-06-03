package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// wantlist_test.go covers the Phase 19 owner-scoped wantlist store funcs (19-01
// Task 2): AddWantTx (returns the new id; returns the TYPED ErrDuplicateWant on a
// unique-index conflict, detected via *sqlite.Error.Code()==2067 — NEVER a raw
// driver error and NEVER a string-match), ListOwnWants (owner-scoped, active-only,
// non-nil slice), and RemoveOwnWantTx (owner-scoped soft-delete; cross-owner is a
// silent (false,nil) IDOR no-op). Identity keys on discord_user_id (Pitfall 3),
// NOT an owner entity. Reuses insertWebUser / commitTx (admins_test.go) +
// NewTestDB (testhelper.go).

// strptr returns a *string for a note literal (the AddWantTx note param).
func strptr(s string) *string { return &s }

// listWants is a small read helper around ListOwnWants for the assertions below.
func listWants(t *testing.T, ctx context.Context, db *sql.DB, discordID string) []WantlistRow {
	t.Helper()
	rows, err := ListOwnWants(ctx, db, discordID)
	if err != nil {
		t.Fatalf("ListOwnWants(%q): %v", discordID, err)
	}
	return rows
}

func TestAddWantTx_InsertsActiveRowAndReturnsID(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	itemID := int64(1001)
	var newID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		newID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Rusty Dagger", "buy", "high", strptr("for alt"), 1700)
		return e
	}); err != nil {
		t.Fatalf("AddWantTx: %v", err)
	}
	if newID <= 0 {
		t.Fatalf("AddWantTx returned non-positive id %d", newID)
	}

	got := listWants(t, ctx, db, "disc-1")
	if len(got) != 1 {
		t.Fatalf("ListOwnWants len = %d, want 1", len(got))
	}
	r := got[0]
	if r.ID != newID || r.ItemID == nil || *r.ItemID != itemID || r.ItemName != "Rusty Dagger" ||
		r.Reason != "buy" || r.Priority != "high" || r.Note == nil || *r.Note != "for alt" || r.CreatedAt != 1700 {
		t.Errorf("ListOwnWants row = %+v, want the inserted fields", r)
	}
}

func TestListOwnWants_OwnerScopedActiveOnlyNonNil(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")

	// Empty wantlist → non-nil empty slice (JSON []).
	if got := listWants(t, ctx, db, "disc-1"); got == nil {
		t.Fatalf("ListOwnWants on empty list returned nil, want non-nil empty slice")
	} else if len(got) != 0 {
		t.Fatalf("ListOwnWants on empty list len = %d, want 0", len(got))
	}

	i1, i2 := int64(10), int64(20)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		if _, e := AddWantTx(ctx, tx, "disc-1", &i1, "Item Ten", "buy", "med", nil, 1); e != nil {
			return e
		}
		if _, e := AddWantTx(ctx, tx, "disc-1", &i2, "Item Twenty", "quest", "low", nil, 2); e != nil {
			return e
		}
		// Bob's want must NOT appear in Alice's list (owner scoping).
		if _, e := AddWantTx(ctx, tx, "disc-2", &i1, "Item Ten", "buy", "med", nil, 3); e != nil {
			return e
		}
		return nil
	}); err != nil {
		t.Fatalf("seed wants: %v", err)
	}

	alice := listWants(t, ctx, db, "disc-1")
	if len(alice) != 2 {
		t.Fatalf("Alice ListOwnWants len = %d, want 2 (Bob's must not leak)", len(alice))
	}
	bob := listWants(t, ctx, db, "disc-2")
	if len(bob) != 1 {
		t.Fatalf("Bob ListOwnWants len = %d, want 1", len(bob))
	}
}

func TestRemoveOwnWantTx_SoftDeleteExcludesFromList(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	itemID := int64(55)
	var wantID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		wantID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Soft Me", "buy", "med", nil, 1)
		return e
	}); err != nil {
		t.Fatalf("seed want: %v", err)
	}

	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOwnWantTx(ctx, tx, wantID, "disc-1")
		return e
	}); err != nil {
		t.Fatalf("RemoveOwnWantTx: %v", err)
	}
	if !removed {
		t.Fatalf("RemoveOwnWantTx on own active want = false, want true")
	}
	if got := listWants(t, ctx, db, "disc-1"); len(got) != 0 {
		t.Fatalf("after soft-remove, ListOwnWants len = %d, want 0", len(got))
	}
}

func TestRemoveOwnWantTx_CrossOwnerSilentNoOp(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")

	itemID := int64(77)
	var wantID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		wantID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Alice Only", "buy", "med", nil, 1)
		return e
	}); err != nil {
		t.Fatalf("seed Alice want: %v", err)
	}

	// Bob tries to remove Alice's want by id → IDOR-safe silent no-op (false, nil).
	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveOwnWantTx(ctx, tx, wantID, "disc-2")
		return e
	}); err != nil {
		t.Fatalf("cross-owner RemoveOwnWantTx errored: %v (want silent no-op)", err)
	}
	if removed {
		t.Fatalf("cross-owner RemoveOwnWantTx removed = true, want false (IDOR no-op)")
	}
	// Alice's want is untouched.
	if got := listWants(t, ctx, db, "disc-1"); len(got) != 1 {
		t.Fatalf("after cross-owner no-op, Alice ListOwnWants len = %d, want 1 (untouched)", len(got))
	}
}

func TestAddWantTx_DuplicateReturnsTypedSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	itemID := int64(42)
	// First add succeeds.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Dup Item", "buy", "med", nil, 1)
		return e
	}); err != nil {
		t.Fatalf("first AddWantTx: %v", err)
	}

	// Exact (owner, item, reason) re-add → TYPED ErrDuplicateWant (not a raw driver error).
	addErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Dup Item", "buy", "med", nil, 2)
		return e
	})
	if !errors.Is(addErr, ErrDuplicateWant) {
		t.Fatalf("duplicate (user,item,reason) AddWantTx err = %v, want ErrDuplicateWant", addErr)
	}

	// SAME item, DIFFERENT reason → inserts fine (D-05: buy + quest coexist).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Dup Item", "quest", "med", nil, 3)
		return e
	}); err != nil {
		t.Fatalf("same item different reason AddWantTx should succeed, got: %v", err)
	}

	if got := listWants(t, ctx, db, "disc-1"); len(got) != 2 {
		t.Fatalf("after dup + different-reason, ListOwnWants len = %d, want 2", len(got))
	}
}

func TestAddWantTx_CustomDuplicateReturnsTypedSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	// First custom want (item_id NULL, dedupe on item_name).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", nil, "Homemade Label", "buy", "med", nil, 1)
		return e
	}); err != nil {
		t.Fatalf("first custom AddWantTx: %v", err)
	}

	// Exact (owner, label, reason) re-add of a custom want → ErrDuplicateWant
	// (the wantlist_custom_uidx partial index; NULL item_id is dedupe-safe).
	addErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", nil, "Homemade Label", "buy", "med", nil, 2)
		return e
	})
	if !errors.Is(addErr, ErrDuplicateWant) {
		t.Fatalf("duplicate custom (user,label,reason) AddWantTx err = %v, want ErrDuplicateWant", addErr)
	}
}
