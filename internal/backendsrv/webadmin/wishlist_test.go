package webadmin

// wishlist_test.go covers the per-character / per-slot wishlist write handlers
// (Phase 34, WISH-02/03/05): add (owned char) derives the owner from the session,
// authorizes the character tag in-tx (T-34-07), and audits IDs only; an add for a
// NON-owned character → 403 char_not_assigned with no row, no audit; an exact re-add
// → 409 duplicate; an unknown slot → 400 invalid_input; remove/ping of a cross-owner
// id is a silent no-op (removed:false / pinged echoed) that flips no row.
//
// The handlers read the acting discord_user_id via webauth.UserFromContext; these
// unit tests inject it with withCaller (officers_test.go) without standing up the
// session machinery (route-level RequireSession is covered by main_test.go).

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// wlSeedAssignment inserts a character_assignment row (charID → discordID), the
// fixture behind the tagged-target IDOR cases: a row owned by the CALLER is the
// authorized tag; a row owned by ANOTHER member is the forged-tag (403) fixture.
func wlSeedAssignment(t *testing.T, ctx context.Context, db *sql.DB, charID int64, discordID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, ?, ?, 'self')`, charID, discordID, 1700000000); err != nil {
		t.Fatalf("seed character_assignment (char=%d, %s): %v", charID, discordID, err)
	}
}

// activeWishCount reads the active-target count for a discord_user_id.
func activeWishCount(t *testing.T, ctx context.Context, db *sql.DB, discordID string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wishlist_item WHERE discord_user_id = ? AND active = 1`, discordID).Scan(&n); err != nil {
		t.Fatalf("count active wishlist (%s): %v", discordID, err)
	}
	return n
}

// pingedOf reads a target's pinged flag by id (the ping round-trip assertion).
func pingedOf(t *testing.T, ctx context.Context, db *sql.DB, id int64) bool {
	t.Helper()
	var p int
	if err := db.QueryRowContext(ctx,
		`SELECT pinged FROM wishlist_item WHERE id = ?`, id).Scan(&p); err != nil {
		t.Fatalf("read pinged (id=%d): %v", id, err)
	}
	return p != 0
}

// decodeRemovedWL extracts the {"removed":bool} field.
func decodeRemovedWL(t *testing.T, rec interface{ Bytes() []byte }) bool {
	t.Helper()
	var body struct {
		Removed bool `json:"removed"`
	}
	if err := json.Unmarshal(rec.Bytes(), &body); err != nil {
		t.Fatalf("decode removed body: %v", err)
	}
	return body.Removed
}

func TestAddWishlist_OwnedChar_Persists_AuditsIDsOnly(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-wish-ok"
	seedWebUser(t, ctx, db, caller, "Wisher")
	charID := asgInsertChar(t, ctx, db, "Tankbert")
	wlSeedAssignment(t, ctx, db, charID, caller)

	h := withCaller(caller, AddWishlistHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"slot":"Chest","item_id":321,"item_name":"Fungi Tunic"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var row store.WishlistTargetRow
	if err := json.Unmarshal(rec.Body.Bytes(), &row); err != nil {
		t.Fatalf("decode add resp: %v", err)
	}
	if row.ID == 0 || row.Slot != "Chest" || row.CharacterID != charID || !row.Pinged {
		t.Fatalf("echoed row wrong: %+v", row)
	}
	if activeWishCount(t, ctx, db, caller) != 1 {
		t.Fatalf("target not stored")
	}
	if c := auditCount(t, ctx, db, "wishlist_add"); c != 1 {
		t.Fatalf("wishlist_add audit rows = %d, want 1", c)
	}
	// V7: the audit detail carries item_id/character_id/slot, never the item label or char name.
	var detail string
	if err := db.QueryRowContext(ctx,
		`SELECT detail FROM audit_log WHERE event = 'wishlist_add'`).Scan(&detail); err != nil {
		t.Fatalf("read audit detail: %v", err)
	}
	if !strings.Contains(detail, "character_id") || !strings.Contains(detail, "slot") {
		t.Fatalf("audit detail %q missing character_id/slot", detail)
	}
	if strings.Contains(detail, "Fungi Tunic") || strings.Contains(detail, "Tankbert") {
		t.Fatalf("audit detail leaked a label/name: %q", detail)
	}
}

func TestAddWishlist_NonOwnedChar_403(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-wish-bad"
	seedWebUser(t, ctx, db, caller, "Forger")
	// A char assigned to ANOTHER member — the caller forges its id into the body.
	other := "disc-wish-victim"
	seedWebUser(t, ctx, db, other, "Victim")
	charID := asgInsertChar(t, ctx, db, "NotMine")
	wlSeedAssignment(t, ctx, db, charID, other)

	h := withCaller(caller, AddWishlistHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"slot":"Chest","item_id":654,"item_name":"Forged"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (forged character_id tag) (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "char_not_assigned" {
		t.Fatalf("error = %q, want char_not_assigned", got)
	}
	// Nothing persisted for the caller — the in-tx guard rolled the insert back.
	if n := activeWishCount(t, ctx, db, caller); n != 0 {
		t.Fatalf("active targets for caller = %d, want 0 (the forged-tag add must not persist)", n)
	}
	if c := auditCount(t, ctx, db, "wishlist_add"); c != 0 {
		t.Fatalf("wishlist_add audit rows = %d, want 0 (rejected add not audited)", c)
	}
}

func TestAddWishlist_Duplicate_409(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-wish-dup"
	seedWebUser(t, ctx, db, caller, "Duper")
	charID := asgInsertChar(t, ctx, db, "DupToon")
	wlSeedAssignment(t, ctx, db, charID, caller)
	h := withCaller(caller, AddWishlistHandler(db))

	body := `{"character_id":` + itoa(charID) + `,"slot":"Chest","item_id":500,"item_name":"Fungi Tunic"}`
	if rec := postJSON(t, h, body); rec.Code != http.StatusOK {
		t.Fatalf("first add status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// Re-adding the SAME (char, slot, item_id) active target → EXACTLY 409 duplicate.
	rec := postJSON(t, h, body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "duplicate" {
		t.Fatalf("dup error = %q, want duplicate", got)
	}
}

func TestAddWishlist_InvalidSlot_400(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-wish-slot"
	seedWebUser(t, ctx, db, caller, "Sloter")
	charID := asgInsertChar(t, ctx, db, "SlotToon")
	wlSeedAssignment(t, ctx, db, charID, caller)
	h := withCaller(caller, AddWishlistHandler(db))

	cases := []struct {
		name string
		body string
	}{
		{"unknown slot", `{"character_id":` + itoa(charID) + `,"slot":"Pocket","item_id":1,"item_name":"X"}`},
		{"omitted slot Charm (post-Velious)", `{"character_id":` + itoa(charID) + `,"slot":"Charm","item_id":1,"item_name":"X"}`},
		{"empty slot", `{"character_id":` + itoa(charID) + `,"slot":"","item_id":1,"item_name":"X"}`},
		{"custom target blank label", `{"character_id":` + itoa(charID) + `,"slot":"Chest","item_id":null,"item_name":"  "}`},
		{"missing character_id", `{"slot":"Chest","item_id":1,"item_name":"X"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, h, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := decodeErr(t, rec); got != "invalid_input" {
				t.Fatalf("error = %q, want invalid_input", got)
			}
		})
	}
}

func TestRemoveOwnWishlist_CrossOwnerNoOp_OwnRemoved(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-wish-rm"
	seedWebUser(t, ctx, db, caller, "Remover")
	cCaller := asgInsertChar(t, ctx, db, "MyToon")
	wlSeedAssignment(t, ctx, db, cCaller, caller)

	// Another member's target (the IDOR fixture) — added by them directly.
	other := "disc-wish-other"
	seedWebUser(t, ctx, db, other, "Other")
	cOther := asgInsertChar(t, ctx, db, "TheirToon")
	wlSeedAssignment(t, ctx, db, cOther, other)
	addOther := withCaller(other, AddWishlistHandler(db))
	otherRec := postJSON(t, addOther, `{"character_id":`+itoa(cOther)+`,"slot":"Chest","item_id":42,"item_name":"Theirs"}`)
	if otherRec.Code != http.StatusOK {
		t.Fatalf("seed other add status = %d (body=%s)", otherRec.Code, otherRec.Body.String())
	}
	var otherRow store.WishlistTargetRow
	if err := json.Unmarshal(otherRec.Body.Bytes(), &otherRow); err != nil {
		t.Fatalf("decode other: %v", err)
	}

	// Caller tries to remove the OTHER member's target → removed:false, row untouched.
	revH := withCaller(caller, RemoveOwnWishlistHandler(db))
	rec := postJSON(t, revH, `{"id":`+itoa(otherRow.ID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("cross-owner remove status = %d, want 200", rec.Code)
	}
	if decodeRemovedWL(t, rec.Body) {
		t.Fatalf("cross-owner remove returned removed=true — IDOR")
	}
	if activeWishCount(t, ctx, db, other) != 1 {
		t.Fatalf("other member's target was removed — IDOR")
	}
	if c := auditCount(t, ctx, db, "wishlist_remove"); c != 0 {
		t.Fatalf("wishlist_remove audit rows = %d, want 0 (no-op not audited)", c)
	}

	// Caller adds + removes its OWN target → removed:true, audited.
	addH := withCaller(caller, AddWishlistHandler(db))
	addRec := postJSON(t, addH, `{"character_id":`+itoa(cCaller)+`,"slot":"Chest","item_id":11,"item_name":"Mine"}`)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add own status = %d (body=%s)", addRec.Code, addRec.Body.String())
	}
	var added store.WishlistTargetRow
	if err := json.Unmarshal(addRec.Body.Bytes(), &added); err != nil {
		t.Fatalf("decode added: %v", err)
	}
	rec2 := postJSON(t, revH, `{"id":`+itoa(added.ID)+`}`)
	if !decodeRemovedWL(t, rec2.Body) {
		t.Fatalf("own remove returned removed=false, want true")
	}
	if activeWishCount(t, ctx, db, caller) != 0 {
		t.Fatalf("own target not removed")
	}
	if c := auditCount(t, ctx, db, "wishlist_remove"); c != 1 {
		t.Fatalf("wishlist_remove audit rows = %d, want 1", c)
	}
}

func TestSetWishlistPing_CrossOwnerNoOp_OwnFlips(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-wish-ping"
	seedWebUser(t, ctx, db, caller, "Pinger")
	cCaller := asgInsertChar(t, ctx, db, "PingToon")
	wlSeedAssignment(t, ctx, db, cCaller, caller)

	// Another member's target (the IDOR fixture).
	other := "disc-wish-pother"
	seedWebUser(t, ctx, db, other, "POther")
	cOther := asgInsertChar(t, ctx, db, "POtherToon")
	wlSeedAssignment(t, ctx, db, cOther, other)
	addOther := withCaller(other, AddWishlistHandler(db))
	otherRec := postJSON(t, addOther, `{"character_id":`+itoa(cOther)+`,"slot":"Chest","item_id":42,"item_name":"Theirs"}`)
	var otherRow store.WishlistTargetRow
	if err := json.Unmarshal(otherRec.Body.Bytes(), &otherRow); err != nil {
		t.Fatalf("decode other: %v", err)
	}

	pingH := withCaller(caller, SetWishlistPingHandler(db))

	// Cross-owner ping-off → pinged echoed as requested (false), other member's row
	// UNCHANGED (still pinged=1) and nothing audited (silent no-op).
	recX := postJSON(t, pingH, `{"id":`+itoa(otherRow.ID)+`,"pinged":false}`)
	if recX.Code != http.StatusOK {
		t.Fatalf("cross-owner ping status = %d, want 200", recX.Code)
	}
	var bodyX struct {
		Pinged bool `json:"pinged"`
	}
	_ = json.Unmarshal(recX.Body.Bytes(), &bodyX)
	if bodyX.Pinged {
		t.Fatalf("cross-owner ping echo = true, want requested false")
	}
	if !pingedOf(t, ctx, db, otherRow.ID) {
		t.Fatalf("other member's target was un-pinged — IDOR")
	}
	if c := auditCount(t, ctx, db, "wishlist_ping"); c != 0 {
		t.Fatalf("wishlist_ping audit rows = %d, want 0 (no-op not audited)", c)
	}

	// Caller pings-off its OWN target → flipped, audited (want_id only).
	addH := withCaller(caller, AddWishlistHandler(db))
	addRec := postJSON(t, addH, `{"character_id":`+itoa(cCaller)+`,"slot":"Chest","item_id":11,"item_name":"Mine"}`)
	var added store.WishlistTargetRow
	if err := json.Unmarshal(addRec.Body.Bytes(), &added); err != nil {
		t.Fatalf("decode added: %v", err)
	}
	if !pingedOf(t, ctx, db, added.ID) {
		t.Fatalf("new target should default pinged=1")
	}
	recPing := postJSON(t, pingH, `{"id":`+itoa(added.ID)+`,"pinged":false}`)
	if recPing.Code != http.StatusOK {
		t.Fatalf("own ping status = %d, want 200", recPing.Code)
	}
	if pingedOf(t, ctx, db, added.ID) {
		t.Fatalf("own target not un-pinged")
	}
	if c := auditCount(t, ctx, db, "wishlist_ping"); c != 1 {
		t.Fatalf("wishlist_ping audit rows = %d, want 1", c)
	}
	var detail string
	if err := db.QueryRowContext(ctx,
		`SELECT detail FROM audit_log WHERE event = 'wishlist_ping'`).Scan(&detail); err != nil {
		t.Fatalf("read ping audit detail: %v", err)
	}
	if !strings.Contains(detail, "want_id") {
		t.Fatalf("ping audit detail %q missing want_id", detail)
	}

	// Re-ping → pinged:true round-trips.
	recOn := postJSON(t, pingH, `{"id":`+itoa(added.ID)+`,"pinged":true}`)
	if recOn.Code != http.StatusOK {
		t.Fatalf("re-ping status = %d, want 200", recOn.Code)
	}
	if !pingedOf(t, ctx, db, added.ID) {
		t.Fatalf("own target still un-pinged after re-ping")
	}
}

func TestWishlistWrite_BadID_400(t *testing.T) {
	db := store.NewTestDB(t)
	caller := "disc-wish-badid"
	seedWebUser(t, context.Background(), db, caller, "BadID")
	for _, h := range []http.Handler{
		withCaller(caller, RemoveOwnWishlistHandler(db)),
		withCaller(caller, SetWishlistPingHandler(db)),
	} {
		rec := postJSON(t, h, `{"id":0}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if got := decodeErr(t, rec); got != "invalid_input" {
			t.Fatalf("error = %q, want invalid_input", got)
		}
	}
}
