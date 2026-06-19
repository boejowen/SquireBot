package wantmatch

import (
	"context"
	"database/sql"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// match_test.go covers the wantmatch seam (Phase 20 Task 2; REPOINTED to
// wishlist_item in Phase 34, WISH-05): ForItem (stable item_id) + ForName (exact,
// case-insensitive name) both EXCLUDE ping-off (pinged=0) and inactive targets at
// the matcher, so every monitor (P21-23) inherits those gates. The tests seed
// web_users + characters + wishlist_items via raw SQL (wantmatch lives in its own
// package; it only reads, so the fixture writes are direct).

func seedUser(t *testing.T, ctx context.Context, db *sql.DB, discordID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, discordID, "user-"+discordID); err != nil {
		t.Fatalf("seed web_user %q: %v", discordID, err)
	}
}

// seedCharacter inserts a live character under a throwaway upload owner and returns
// its id. ownerID is the character.owner_id UPLOAD provenance — DELIBERATELY a
// DIFFERENT user from the wishlist creator in the regression test, to prove the DM
// target is the wishlist row's discord_user_id, NOT the character's owner.
func seedCharacter(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("seed owner for %q: %v", name, err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, `INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		t.Fatalf("seed character %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedWish inserts a wishlist_item tagged to characterID (NOT NULL) with explicit
// item_id (nullable), name, slot, active, pinged and returns its id. itemID < 0
// means a custom/gear-tier (NULL item_id) target reachable only via ForName.
func seedWish(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64, name, slot string, characterID int64, active, pinged int) int64 {
	t.Helper()
	var idArg interface{}
	if itemID >= 0 {
		idArg = itemID
	} else {
		idArg = nil
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO wishlist_item (discord_user_id, character_id, slot, item_id, item_name, pinged, active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
		discordID, characterID, slot, idArg, name, pinged, active)
	if err != nil {
		t.Fatalf("seed wish (%q, item=%d, name=%q): %v", discordID, itemID, name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed wish last insert id: %v", err)
	}
	return id
}

func wantIDs(hits []Hit) map[int64]Hit {
	m := make(map[int64]Hit, len(hits))
	for _, h := range hits {
		m[h.WantID] = h
	}
	return m
}

func TestForItem_ReturnsActivePingedAcrossUsers(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "bob")
	seedUser(t, ctx, db, "carol")
	seedUser(t, ctx, db, "dave")

	const fungiID int64 = 5000
	// Each owner gets its own character (character_id is NOT NULL; every target is
	// char+slot-scoped, and the catalog partial-unique index keys on
	// (user,char,slot,item_id) so distinct owners/chars are independent rows).
	cA := seedCharacter(t, ctx, db, "AliceToon")
	cB := seedCharacter(t, ctx, db, "BobToon")
	cC := seedCharacter(t, ctx, db, "CarolToon")
	cD := seedCharacter(t, ctx, db, "DaveToon")

	aliceWant := seedWish(t, ctx, db, "alice", fungiID, "Fungi Tunic", "Chest", cA, 1, 1) // hit
	bobWant := seedWish(t, ctx, db, "bob", fungiID, "Fungi Tunic", "Chest", cB, 1, 1)     // hit (across users)
	// ping-off / inactive variants live on distinct owners.
	unpingedWant := seedWish(t, ctx, db, "carol", fungiID, "Fungi Tunic", "Chest", cC, 1, 0) // pinged=0 ⇒ excluded
	inactiveWant := seedWish(t, ctx, db, "dave", fungiID, "Fungi Tunic", "Chest", cD, 0, 1)  // inactive ⇒ excluded
	otherItem := seedWish(t, ctx, db, "alice", 9999, "Cloak of Flames", "Back", cA, 1, 1)    // wrong item ⇒ excluded

	hits, err := ForItem(ctx, db, fungiID)
	if err != nil {
		t.Fatalf("ForItem: %v", err)
	}
	got := wantIDs(hits)
	if _, ok := got[aliceWant]; !ok {
		t.Errorf("alice's active pinged target missing from ForItem hits")
	}
	if _, ok := got[bobWant]; !ok {
		t.Errorf("bob's target missing — ForItem must match across all users")
	}
	if _, ok := got[unpingedWant]; ok {
		t.Errorf("pinged=0 target present in ForItem hits; the ping gate must exclude it (WISH-05)")
	}
	if _, ok := got[inactiveWant]; ok {
		t.Errorf("inactive target present in ForItem hits; active=0 must be excluded")
	}
	if _, ok := got[otherItem]; ok {
		t.Errorf("a target for a different item_id leaked into ForItem hits")
	}
	if len(hits) != 2 {
		t.Fatalf("ForItem returned %d hits; want exactly 2 (alice+bob, active, pinged)", len(hits))
	}
	// Hit carries the fields a notify caller needs.
	if h := got[aliceWant]; h.DiscordUserID != "alice" || h.ItemName != "Fungi Tunic" {
		t.Errorf("Hit fields wrong: %+v", h)
	}
	if h := got[aliceWant]; h.ItemID == nil || *h.ItemID != fungiID {
		t.Errorf("Hit.ItemID = %v; want %d", h.ItemID, fungiID)
	}
	// The character tag is surfaced (DISPLAY-ONLY) via the INNER JOIN.
	if h := got[aliceWant]; h.CharacterName == nil || *h.CharacterName != "AliceToon" {
		t.Errorf("Hit.CharacterName = %v; want %q", h.CharacterName, "AliceToon")
	}
}

func TestForItem_NonNilEmptySlice(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	hits, err := ForItem(ctx, db, 12345)
	if err != nil {
		t.Fatalf("ForItem(no matches): %v", err)
	}
	if hits == nil {
		t.Errorf("ForItem returned a nil slice; want non-nil empty")
	}
	if len(hits) != 0 {
		t.Errorf("ForItem returned %d hits on an empty DB; want 0", len(hits))
	}
}

func TestForName_ExactCaseInsensitive_NoSubstring(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "carol")
	cA := seedCharacter(t, ctx, db, "AliceToon")
	cC := seedCharacter(t, ctx, db, "CarolToon")

	exact := seedWish(t, ctx, db, "alice", -1, "Fungi Tunic", "Chest", cA, 1, 1)           // custom target, exact name
	mixedCase := seedWish(t, ctx, db, "carol", -1, "fungi tunic", "Chest", cC, 1, 1)       // different user, case-insensitive ⇒ also a hit
	substring := seedWish(t, ctx, db, "alice", -1, "Black Fungi Tunic", "Chest", cA, 1, 1) // CONTAINS the query but is NOT an exact match (Pitfall 6)
	unpinged := seedWish(t, ctx, db, "carol", -1, "Fungi Tunic", "Legs", cC, 1, 0)         // pinged=0 ⇒ excluded

	hits, err := ForName(ctx, db, "Fungi Tunic")
	if err != nil {
		t.Fatalf("ForName: %v", err)
	}
	got := wantIDs(hits)
	if _, ok := got[exact]; !ok {
		t.Errorf("exact-name target missing from ForName hits")
	}
	if _, ok := got[mixedCase]; !ok {
		t.Errorf("case-insensitive (fungi tunic) target missing; ForName must be COLLATE NOCASE")
	}
	if _, ok := got[substring]; ok {
		t.Errorf("substring target 'Black Fungi Tunic' matched; ForName must be EXACT, not LIKE (Pitfall 6)")
	}
	if _, ok := got[unpinged]; ok {
		t.Errorf("pinged=0 target present in ForName hits; the ping gate must exclude it (WISH-05)")
	}
	if len(hits) != 2 {
		t.Fatalf("ForName returned %d hits; want exactly 2 (exact + case-variant)", len(hits))
	}
}

// TestForItem_DMTargetIsWishOwner_NotCharacterOwner is the LOAD-BEARING regression
// (T-28-06 / T-34-08): a wishlist target's Hit.DiscordUserID (the DM target consumed
// by notify.Send) MUST be the wishlist row's OWN discord_user_id — NEVER re-derived
// from the tagged character's owner_id or any assignment. The character is OWNED BY A
// DIFFERENT user (and assigned to yet another), so a buggy join that read the
// character's owner would flip the DM target and this assertion would catch it.
func TestForItem_DMTargetIsWishOwner_NotCharacterOwner(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")   // the WISHLIST creator = the correct DM target
	seedUser(t, ctx, db, "mallory") // assigned to the char — must NOT be the DM target

	const itemID int64 = 7777
	// The character is created under its OWN throwaway upload owner (NOT alice), and is
	// assigned to mallory — two distinct identities, neither of which is the wishlist owner.
	charID := seedCharacter(t, ctx, db, "SharedToon")
	if _, err := db.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, 'mallory', 1700000000, 'self')`, charID); err != nil {
		t.Fatalf("seed assignment to mallory: %v", err)
	}

	// alice adds a wishlist target on that character.
	tagged := seedWish(t, ctx, db, "alice", itemID, "Contested Item", "Primary", charID, 1, 1)

	hits, err := ForItem(ctx, db, itemID)
	if err != nil {
		t.Fatalf("ForItem: %v", err)
	}
	got := wantIDs(hits)
	h, ok := got[tagged]
	if !ok {
		t.Fatalf("tagged wishlist target missing from ForItem hits")
	}
	// The DM target is the WISHLIST owner — independent of character_id / its owner / its assignee.
	if h.DiscordUserID != "alice" {
		t.Fatalf("Hit.DiscordUserID = %q; want %q (the wishlist owner) — the DM target must NOT be derived from character_id (T-28-06)", h.DiscordUserID, "alice")
	}
	// And the display name is still surfaced (proving the JOIN ran without hijacking the target).
	if h.CharacterName == nil || *h.CharacterName != "SharedToon" {
		t.Fatalf("Hit.CharacterName = %v; want %q", h.CharacterName, "SharedToon")
	}
}
