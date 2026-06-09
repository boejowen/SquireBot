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
		newID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Rusty Dagger", "buy", "high", strptr("for alt"), nil, 1700)
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
		if _, e := AddWantTx(ctx, tx, "disc-1", &i1, "Item Ten", "buy", "med", nil, nil, 1); e != nil {
			return e
		}
		if _, e := AddWantTx(ctx, tx, "disc-1", &i2, "Item Twenty", "quest", "low", nil, nil, 2); e != nil {
			return e
		}
		// Bob's want must NOT appear in Alice's list (owner scoping).
		if _, e := AddWantTx(ctx, tx, "disc-2", &i1, "Item Ten", "buy", "med", nil, nil, 3); e != nil {
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
		wantID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Soft Me", "buy", "med", nil, nil, 1)
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
		wantID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Alice Only", "buy", "med", nil, nil, 1)
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
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Dup Item", "buy", "med", nil, nil, 1)
		return e
	}); err != nil {
		t.Fatalf("first AddWantTx: %v", err)
	}

	// Exact (owner, item, reason) re-add → TYPED ErrDuplicateWant (not a raw driver error).
	addErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Dup Item", "buy", "med", nil, nil, 2)
		return e
	})
	if !errors.Is(addErr, ErrDuplicateWant) {
		t.Fatalf("duplicate (user,item,reason) AddWantTx err = %v, want ErrDuplicateWant", addErr)
	}

	// SAME item, DIFFERENT reason → inserts fine (D-05: buy + quest coexist).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Dup Item", "quest", "med", nil, nil, 3)
		return e
	}); err != nil {
		t.Fatalf("same item different reason AddWantTx should succeed, got: %v", err)
	}

	if got := listWants(t, ctx, db, "disc-1"); len(got) != 2 {
		t.Fatalf("after dup + different-reason, ListOwnWants len = %d, want 2", len(got))
	}
}

func TestListOwnWants_ReturnsMutedDefaultFalse(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	itemID := int64(99)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Bell Me", "buy", "med", nil, nil, 1)
		return e
	}); err != nil {
		t.Fatalf("seed want: %v", err)
	}

	// A fresh want defaults muted=false (the migration DEFAULT 0; mute-bell off).
	got := listWants(t, ctx, db, "disc-1")
	if len(got) != 1 {
		t.Fatalf("ListOwnWants len = %d, want 1", len(got))
	}
	if got[0].Muted {
		t.Errorf("fresh want Muted = true, want false (DEFAULT 0)")
	}
}

func TestSetMutedTx_OwnerScopedAndReflectedInList(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")

	itemID := int64(99)
	var wantID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		wantID, e = AddWantTx(ctx, tx, "disc-1", &itemID, "Bell Me", "buy", "med", nil, nil, 1)
		return e
	}); err != nil {
		t.Fatalf("seed want: %v", err)
	}

	// Bob tries to mute Alice's want → IDOR-safe silent no-op (false, nil).
	var ok bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		ok, e = SetMutedTx(ctx, tx, wantID, "disc-2", true)
		return e
	}); err != nil {
		t.Fatalf("cross-owner SetMutedTx errored: %v (want silent no-op)", err)
	}
	if ok {
		t.Fatalf("cross-owner SetMutedTx = true, want false (IDOR no-op)")
	}
	if listWants(t, ctx, db, "disc-1")[0].Muted {
		t.Fatalf("after cross-owner no-op, Alice's want Muted = true, want false (untouched)")
	}

	// Alice mutes her own want → (true, nil), and ListOwnWants reflects it.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		ok, e = SetMutedTx(ctx, tx, wantID, "disc-1", true)
		return e
	}); err != nil {
		t.Fatalf("SetMutedTx own: %v", err)
	}
	if !ok {
		t.Fatalf("SetMutedTx own want = false, want true")
	}
	if !listWants(t, ctx, db, "disc-1")[0].Muted {
		t.Fatalf("after own mute, ListOwnWants Muted = false, want true (BLOCKER-3 read path)")
	}

	// Un-mute round-trips back to false.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := SetMutedTx(ctx, tx, wantID, "disc-1", false)
		return e
	}); err != nil {
		t.Fatalf("SetMutedTx unmute: %v", err)
	}
	if listWants(t, ctx, db, "disc-1")[0].Muted {
		t.Errorf("after unmute, ListOwnWants Muted = true, want false")
	}
}

// TestAddWantTx_PersistsCharacterTagAndListSurfacesName proves AddWantTx persists a
// non-nil characterID and ListOwnWants returns that row with character_id + character_name
// (the LEFT JOIN character) populated (CWANT-01/06).
func TestAddWantTx_PersistsCharacterTagAndListSurfacesName(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	ownerID := insertOwner(t, ctx, db, "Alice-Owner")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)

	itemID := int64(1234)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Tagged Item", "buy", "high", nil, i64ptr(charID), 1700)
		return e
	}); err != nil {
		t.Fatalf("AddWantTx tagged: %v", err)
	}

	got := listWants(t, ctx, db, "disc-1")
	if len(got) != 1 {
		t.Fatalf("ListOwnWants len = %d, want 1", len(got))
	}
	r := got[0]
	if r.CharacterID == nil || *r.CharacterID != charID {
		t.Errorf("CharacterID = %v, want %d", r.CharacterID, charID)
	}
	if r.CharacterName == nil || *r.CharacterName != "Slampeach" {
		t.Errorf("CharacterName = %v, want \"Slampeach\"", r.CharacterName)
	}
}

// TestAddWantTx_NilCharacterTagListsAsNull proves a nil characterID lists back as
// character_id NULL + character_name NULL (an account-level want).
func TestAddWantTx_NilCharacterTagListsAsNull(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	itemID := int64(1234)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", &itemID, "Account Item", "buy", "med", nil, nil, 1700)
		return e
	}); err != nil {
		t.Fatalf("AddWantTx account-level: %v", err)
	}

	got := listWants(t, ctx, db, "disc-1")
	if len(got) != 1 {
		t.Fatalf("ListOwnWants len = %d, want 1", len(got))
	}
	if got[0].CharacterID != nil {
		t.Errorf("CharacterID = %v, want nil (account-level)", got[0].CharacterID)
	}
	if got[0].CharacterName != nil {
		t.Errorf("CharacterName = %v, want nil (account-level)", got[0].CharacterName)
	}
}

// TestListGuildWants_AllMembersWithOwnerAndOptionalChar proves ListGuildWants returns
// active wants from MULTIPLE members, each with the owner username, a tagged want shows
// the character name and an untagged one shows nil — and that the empty list is a non-nil
// JSON [] (CWANT-03/04).
func TestListGuildWants_AllMembersWithOwnerAndOptionalChar(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// Empty → non-nil empty slice (JSON []).
	if got, err := ListGuildWants(ctx, db); err != nil {
		t.Fatalf("ListGuildWants (empty): %v", err)
	} else if got == nil {
		t.Fatalf("ListGuildWants on empty DB returned nil, want non-nil empty slice")
	} else if len(got) != 0 {
		t.Fatalf("ListGuildWants on empty DB len = %d, want 0", len(got))
	}

	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")
	aliceOwner := insertOwner(t, ctx, db, "Alice-Owner")
	aliceChar := insertChar(t, ctx, db, aliceOwner, "Slampeach", false)

	i1, i2 := int64(10), int64(20)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		// Alice: one tagged want.
		if _, e := AddWantTx(ctx, tx, "disc-1", &i1, "Alice Tagged", "buy", "high", nil, i64ptr(aliceChar), 1); e != nil {
			return e
		}
		// Bob: one untagged (account-level) want.
		if _, e := AddWantTx(ctx, tx, "disc-2", &i2, "Bob Untagged", "quest", "low", nil, nil, 2); e != nil {
			return e
		}
		return nil
	}); err != nil {
		t.Fatalf("seed wants: %v", err)
	}

	got, err := ListGuildWants(ctx, db)
	if err != nil {
		t.Fatalf("ListGuildWants: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListGuildWants len = %d, want 2 (all members)", len(got))
	}

	byItem := map[string]GuildWantRow{}
	for _, r := range got {
		byItem[r.ItemName] = r
	}
	at, ok := byItem["Alice Tagged"]
	if !ok {
		t.Fatalf("Alice's tagged want missing from guild roll-up")
	}
	if at.Owner != "Alice" {
		t.Errorf("Alice Tagged owner = %q, want \"Alice\"", at.Owner)
	}
	if at.DiscordUserID != "disc-1" {
		t.Errorf("Alice Tagged discord_user_id = %q, want \"disc-1\"", at.DiscordUserID)
	}
	if at.CharacterName == nil || *at.CharacterName != "Slampeach" {
		t.Errorf("Alice Tagged character_name = %v, want \"Slampeach\"", at.CharacterName)
	}
	bu, ok := byItem["Bob Untagged"]
	if !ok {
		t.Fatalf("Bob's untagged want missing from guild roll-up")
	}
	if bu.Owner != "Bob" {
		t.Errorf("Bob Untagged owner = %q, want \"Bob\"", bu.Owner)
	}
	if bu.CharacterID != nil || bu.CharacterName != nil {
		t.Errorf("Bob Untagged char = (%v, %v), want (nil, nil)", bu.CharacterID, bu.CharacterName)
	}
}

func TestAddWantTx_CustomDuplicateReturnsTypedSentinel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	// First custom want (item_id NULL, dedupe on item_name).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", nil, "Homemade Label", "buy", "med", nil, nil, 1)
		return e
	}); err != nil {
		t.Fatalf("first custom AddWantTx: %v", err)
	}

	// Exact (owner, label, reason) re-add of a custom want → ErrDuplicateWant
	// (the wantlist_custom_uidx partial index; NULL item_id is dedupe-safe).
	addErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddWantTx(ctx, tx, "disc-1", nil, "Homemade Label", "buy", "med", nil, nil, 2)
		return e
	})
	if !errors.Is(addErr, ErrDuplicateWant) {
		t.Fatalf("duplicate custom (user,label,reason) AddWantTx err = %v, want ErrDuplicateWant", addErr)
	}
}
