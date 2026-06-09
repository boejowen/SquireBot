package wantmatch

import (
	"context"
	"database/sql"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// match_test.go covers the wantmatch seam (Phase 20 Task 2): ForItem (stable
// item_id) + ForName (exact, case-insensitive name) both EXCLUDE muted and
// inactive wants at the matcher, so every future monitor (P21-23) inherits those
// gates. The tests seed web_users + wantlist_items via raw SQL (wantmatch lives
// in its own package; it only reads, so the fixture writes are direct).

func seedUser(t *testing.T, ctx context.Context, db *sql.DB, discordID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, discordID, "user-"+discordID); err != nil {
		t.Fatalf("seed web_user %q: %v", discordID, err)
	}
}

// seedWant inserts a wantlist_item with explicit item_id (nullable), name,
// active, muted and returns its id. itemID < 0 means a custom (NULL item_id)
// want. The note is NULL (use seedWantNote to set one).
func seedWant(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64, name string, active, muted int) int64 {
	t.Helper()
	return seedWantNote(t, ctx, db, discordID, itemID, name, active, muted, nil)
}

// seedWantNote is seedWant plus an optional note (nil ⇒ NULL note column) so a
// test can assert Hit.Note carries the "why you wanted it" text (D-05).
func seedWantNote(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64, name string, active, muted int, note *string) int64 {
	t.Helper()
	var idArg interface{}
	if itemID >= 0 {
		idArg = itemID
	} else {
		idArg = nil
	}
	var noteArg interface{}
	if note != nil {
		noteArg = *note
	} else {
		noteArg = nil
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, active, muted, created_at)
		 VALUES (?, ?, ?, 'buy', 'med', ?, ?, ?, 1)`,
		discordID, idArg, name, noteArg, active, muted)
	if err != nil {
		t.Fatalf("seed want (%q, item=%d, name=%q): %v", discordID, itemID, name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed want last insert id: %v", err)
	}
	return id
}

// seedCharacter inserts a live character under a throwaway owner and returns its
// id (the tagged-want fixture). ownerID is the character.owner_id UPLOAD provenance
// — DELIBERATELY a DIFFERENT user from the want creator in the regression test, to
// prove the DM target is the want's discord_user_id, NOT the character's owner.
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

// seedWantChar inserts an active, non-muted wantlist_item tagged with characterID
// (a real character.id) and returns its id. The owner is discordID (the DM target).
func seedWantChar(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64, name string, characterID int64) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, character_id, active, muted, created_at)
		 VALUES (?, ?, ?, 'buy', 'med', ?, 1, 0, 1)`,
		discordID, itemID, name, characterID)
	if err != nil {
		t.Fatalf("seed tagged want (%q, char=%d): %v", discordID, characterID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed tagged want last insert id: %v", err)
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

func TestForItem_ReturnsActiveNonMutedAcrossUsers(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "bob")

	seedUser(t, ctx, db, "carol")
	seedUser(t, ctx, db, "dave")

	const fungiID int64 = 5000
	aliceWant := seedWant(t, ctx, db, "alice", fungiID, "Fungi Tunic", 1, 0) // hit
	bobWant := seedWant(t, ctx, db, "bob", fungiID, "Fungi Tunic", 1, 0)     // hit (across users)
	// muted/inactive variants live on distinct owners — the catalog partial
	// unique index forbids two active (user,item,reason) rows for one user.
	mutedWant := seedWant(t, ctx, db, "carol", fungiID, "Fungi Tunic", 1, 1)   // muted ⇒ excluded
	inactiveWant := seedWant(t, ctx, db, "dave", fungiID, "Fungi Tunic", 0, 0) // inactive ⇒ excluded
	otherItem := seedWant(t, ctx, db, "alice", 9999, "Cloak of Flames", 1, 0)  // wrong item ⇒ excluded

	hits, err := ForItem(ctx, db, fungiID)
	if err != nil {
		t.Fatalf("ForItem: %v", err)
	}
	got := wantIDs(hits)
	if _, ok := got[aliceWant]; !ok {
		t.Errorf("alice's active non-muted want missing from ForItem hits")
	}
	if _, ok := got[bobWant]; !ok {
		t.Errorf("bob's want missing — ForItem must match across all users")
	}
	if _, ok := got[mutedWant]; ok {
		t.Errorf("muted want present in ForItem hits; the mute gate must exclude it (D-09)")
	}
	if _, ok := got[inactiveWant]; ok {
		t.Errorf("inactive want present in ForItem hits; active=0 must be excluded")
	}
	if _, ok := got[otherItem]; ok {
		t.Errorf("a want for a different item_id leaked into ForItem hits")
	}
	if len(hits) != 2 {
		t.Fatalf("ForItem returned %d hits; want exactly 2 (alice+bob, active, non-muted)", len(hits))
	}
	// Hit carries the fields a notify caller needs.
	if h := got[aliceWant]; h.DiscordUserID != "alice" || h.ItemName != "Fungi Tunic" || h.Reason != "buy" {
		t.Errorf("Hit fields wrong: %+v", h)
	}
	if h := got[aliceWant]; h.ItemID == nil || *h.ItemID != fungiID {
		t.Errorf("Hit.ItemID = %v; want %d", h.ItemID, fungiID)
	}
	// A want with no note ⇒ Hit.Note is nil (the note column is nullable).
	if h := got[aliceWant]; h.Note != nil {
		t.Errorf("Hit.Note = %q for a noteless want; want nil", *h.Note)
	}
}

func TestForItem_CarriesNote(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "bob")

	const fungiID int64 = 5000
	note := "Need it for my Velious tier-2 checklist"
	withNote := seedWantNote(t, ctx, db, "alice", fungiID, "Fungi Tunic", 1, 0, &note) // note populated
	noNote := seedWant(t, ctx, db, "bob", fungiID, "Fungi Tunic", 1, 0)                 // NULL note

	hits, err := ForItem(ctx, db, fungiID)
	if err != nil {
		t.Fatalf("ForItem: %v", err)
	}
	got := wantIDs(hits)
	if h := got[withNote]; h.Note == nil || *h.Note != note {
		t.Errorf("Hit.Note = %v; want %q (D-05 'why you wanted it')", h.Note, note)
	}
	if h := got[noNote]; h.Note != nil {
		t.Errorf("Hit.Note = %q for a NULL-note want; want nil", *h.Note)
	}
}

func TestForName_CarriesNote(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "bob")

	note := "saving for the epic"
	withNote := seedWantNote(t, ctx, db, "alice", -1, "Fungi Tunic", 1, 0, &note) // custom want, note set
	noNote := seedWant(t, ctx, db, "bob", -1, "Fungi Tunic", 1, 0)                // NULL note

	hits, err := ForName(ctx, db, "Fungi Tunic")
	if err != nil {
		t.Fatalf("ForName: %v", err)
	}
	got := wantIDs(hits)
	if h := got[withNote]; h.Note == nil || *h.Note != note {
		t.Errorf("Hit.Note = %v; want %q (the shared scanHits path serves ForName too)", h.Note, note)
	}
	if h := got[noNote]; h.Note != nil {
		t.Errorf("Hit.Note = %q for a NULL-note want; want nil", *h.Note)
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

	exact := seedWant(t, ctx, db, "alice", -1, "Fungi Tunic", 1, 0)           // custom want, exact name
	mixedCase := seedWant(t, ctx, db, "carol", -1, "fungi tunic", 1, 0)       // different user, case-insensitive ⇒ also a hit
	substring := seedWant(t, ctx, db, "alice", -1, "Black Fungi Tunic", 1, 0) // CONTAINS the query but is NOT an exact match (Pitfall 6)
	// muted want on carol (alice already has an active "Fungi Tunic" custom
	// want — the partial unique index forbids a second active one for the same
	// user/name/reason, so the muted variant must belong to a different owner).
	muted := seedWant(t, ctx, db, "carol", -1, "Fungi Tunic", 1, 1) // muted ⇒ excluded

	hits, err := ForName(ctx, db, "Fungi Tunic")
	if err != nil {
		t.Fatalf("ForName: %v", err)
	}
	got := wantIDs(hits)
	if _, ok := got[exact]; !ok {
		t.Errorf("exact-name want missing from ForName hits")
	}
	if _, ok := got[mixedCase]; !ok {
		t.Errorf("case-insensitive (fungi tunic) want missing; ForName must be COLLATE NOCASE")
	}
	if _, ok := got[substring]; ok {
		t.Errorf("substring want 'Black Fungi Tunic' matched; ForName must be EXACT, not LIKE (Pitfall 6)")
	}
	if _, ok := got[muted]; ok {
		t.Errorf("muted want present in ForName hits; the mute gate must exclude it (D-09)")
	}
	if len(hits) != 2 {
		t.Fatalf("ForName returned %d hits; want exactly 2 (exact + case-variant)", len(hits))
	}
}

func TestForItem_CarriesCharacterName_TaggedAndUntagged(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "bob")

	const fungiID int64 = 5000
	charID := seedCharacter(t, ctx, db, "Tankbert")
	tagged := seedWantChar(t, ctx, db, "alice", fungiID, "Fungi Tunic", charID) // CharacterName populated
	untagged := seedWant(t, ctx, db, "bob", fungiID, "Fungi Tunic", 1, 0)        // NULL character_id ⇒ nil

	hits, err := ForItem(ctx, db, fungiID)
	if err != nil {
		t.Fatalf("ForItem: %v", err)
	}
	got := wantIDs(hits)
	if h := got[tagged]; h.CharacterName == nil || *h.CharacterName != "Tankbert" {
		t.Errorf("Hit.CharacterName = %v; want %q (CWANT-05 tagged)", h.CharacterName, "Tankbert")
	}
	if h := got[untagged]; h.CharacterName != nil {
		t.Errorf("Hit.CharacterName = %q for an untagged want; want nil", *h.CharacterName)
	}
}

func TestForName_CarriesCharacterName_TaggedAndUntagged(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedUser(t, ctx, db, "bob")

	charID := seedCharacter(t, ctx, db, "Healz")
	tagged := seedWantChar(t, ctx, db, "alice", 6001, "Cloak of Flames", charID) // CharacterName populated
	untagged := seedWant(t, ctx, db, "bob", 6001, "Cloak of Flames", 1, 0)        // NULL ⇒ nil

	hits, err := ForName(ctx, db, "Cloak of Flames")
	if err != nil {
		t.Fatalf("ForName: %v", err)
	}
	got := wantIDs(hits)
	if h := got[tagged]; h.CharacterName == nil || *h.CharacterName != "Healz" {
		t.Errorf("Hit.CharacterName = %v; want %q (ForName mirrors ForItem)", h.CharacterName, "Healz")
	}
	if h := got[untagged]; h.CharacterName != nil {
		t.Errorf("Hit.CharacterName = %q for an untagged want; want nil", *h.CharacterName)
	}
}

// TestForItem_DMTargetIsWantOwner_NotCharacterOwner is the LOAD-BEARING regression
// (T-28-06): a tagged want's Hit.DiscordUserID (the DM target consumed by notify.Send)
// MUST be the want's OWN discord_user_id — NEVER re-derived from the tagged
// character's owner_id or any assignment. The character is OWNED BY A DIFFERENT user
// (and assigned to yet another), so a buggy join that read the character's owner would
// flip the DM target and this assertion would catch it.
func TestForItem_DMTargetIsWantOwner_NotCharacterOwner(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")    // the WANT creator = the correct DM target
	seedUser(t, ctx, db, "mallory")  // assigned to the char — must NOT be the DM target

	const itemID int64 = 7777
	// The character is created under its OWN throwaway upload owner (NOT alice), and is
	// assigned to mallory — two distinct identities, neither of which is the want owner.
	charID := seedCharacter(t, ctx, db, "SharedToon")
	if _, err := db.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, 'mallory', 1700000000, 'self')`, charID); err != nil {
		t.Fatalf("seed assignment to mallory: %v", err)
	}

	// alice tags a want to that character.
	tagged := seedWantChar(t, ctx, db, "alice", itemID, "Contested Item", charID)

	hits, err := ForItem(ctx, db, itemID)
	if err != nil {
		t.Fatalf("ForItem: %v", err)
	}
	got := wantIDs(hits)
	h, ok := got[tagged]
	if !ok {
		t.Fatalf("tagged want missing from ForItem hits")
	}
	// The DM target is the WANT owner — independent of character_id / its owner / its assignee.
	if h.DiscordUserID != "alice" {
		t.Fatalf("Hit.DiscordUserID = %q; want %q (the want owner) — the DM target must NOT be derived from character_id (T-28-06)", h.DiscordUserID, "alice")
	}
	// And the display name is still surfaced (proving the JOIN ran without hijacking the target).
	if h.CharacterName == nil || *h.CharacterName != "SharedToon" {
		t.Fatalf("Hit.CharacterName = %v; want %q", h.CharacterName, "SharedToon")
	}
}
