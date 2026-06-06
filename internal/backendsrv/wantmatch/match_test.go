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
