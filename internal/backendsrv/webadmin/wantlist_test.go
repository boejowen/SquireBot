package webadmin

// wantlist_test.go covers the personal-wantlist handlers (19-02 Task 1): add
// (catalog + custom) derives the owner from the session and audits item_id only;
// server-side validation rejects bad enums, an oversized (TRIMMED) note, and a
// blank custom label; a whitespace-only / 280-space note is stored NULL (never a
// row of spaces — review WORTH-FIX 6); an exact duplicate maps to EXACTLY 409
// {"error":"duplicate"} on BOTH the catalog and custom paths while the same item
// with the OTHER reason is a distinct 200 row; list is owner-scoped; and a
// cross-owner remove is a silent no-op (removed:false) that never leaks another
// member's row (the IDOR guard).
//
// The handlers read the acting discord_user_id via webauth.UserFromContext; these
// unit tests inject it with withCaller (officers_test.go) without standing up the
// session machinery (route-level RequireSession is covered by main_test.go).

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// seedWant inserts a wantlist_item directly (the cross-owner IDOR fixture: a row
// owned by SOMEONE ELSE that the caller must not be able to touch). Returns its id.
func seedWant(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID *int64, itemName, reason, priority string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		discordID, itemID, itemName, reason, priority, 1700000000)
	if err != nil {
		t.Fatalf("seed wantlist_item (%s): %v", discordID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed want last insert id: %v", err)
	}
	return id
}

// activeWantCount reads the active-want count for a discord_user_id (the
// "nothing/something was removed" assertion).
func activeWantCount(t *testing.T, ctx context.Context, db *sql.DB, discordID string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wantlist_item WHERE discord_user_id = ? AND active = 1`, discordID).Scan(&n); err != nil {
		t.Fatalf("count active wants (%s): %v", discordID, err)
	}
	return n
}

// i64p is a small *int64 helper for catalog item ids.
func i64p(v int64) *int64 { return &v }

func TestAddWant_Catalog_AuditsItemIDOnly(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-add"
	seedWebUser(t, ctx, db, caller, "Adder")

	h := withCaller(caller, AddWantHandler(db))
	rec := postJSON(t, h, `{"item_id":123,"item_name":"Rusty Dagger","reason":"buy","priority":"high","note":"for the alt"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var row store.WantlistRow
	if err := json.Unmarshal(rec.Body.Bytes(), &row); err != nil {
		t.Fatalf("decode add resp: %v", err)
	}
	if row.ID == 0 || row.ItemID == nil || *row.ItemID != 123 || row.Reason != "buy" || row.Priority != "high" {
		t.Fatalf("echoed row wrong: %+v", row)
	}
	if c := auditCount(t, ctx, db, "wantlist_add"); c != 1 {
		t.Fatalf("wantlist_add audit rows = %d, want 1", c)
	}
	// V7: the audit detail must carry item_id only, never the note text.
	var detail string
	if err := db.QueryRowContext(ctx,
		`SELECT detail FROM audit_log WHERE event = 'wantlist_add'`).Scan(&detail); err != nil {
		t.Fatalf("read audit detail: %v", err)
	}
	if strings.Contains(detail, "for the alt") || strings.Contains(detail, "note") {
		t.Fatalf("audit detail leaked note text: %q", detail)
	}
	if activeWantCount(t, ctx, db, caller) != 1 {
		t.Fatalf("want not stored")
	}
}

func TestAddWant_Custom_NullItemID(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-custom"
	seedWebUser(t, ctx, db, caller, "Customer")

	h := withCaller(caller, AddWantHandler(db))
	// item_id null + no priority → priority defaults to "med".
	rec := postJSON(t, h, `{"item_id":null,"item_name":"Some custom thing","reason":"quest"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var row store.WantlistRow
	if err := json.Unmarshal(rec.Body.Bytes(), &row); err != nil {
		t.Fatalf("decode add resp: %v", err)
	}
	if row.ItemID != nil {
		t.Fatalf("custom want item_id = %v, want null", *row.ItemID)
	}
	if row.Priority != "med" {
		t.Fatalf("custom want priority = %q, want defaulted med", row.Priority)
	}
}

func TestAddWant_Validation(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-val"
	seedWebUser(t, ctx, db, caller, "Validator")
	h := withCaller(caller, AddWantHandler(db))

	// 280 NON-space runes → valid (boundary passes).
	note280 := strings.Repeat("x", 280)
	// 281 runes → rejected.
	note281 := strings.Repeat("x", 281)
	// 280 SPACES → trimmed to "" → treated as empty, NOT a 280-char note (passes
	// validation and is stored NULL).
	note280spaces := strings.Repeat(" ", 280)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad reason", `{"item_id":1,"item_name":"X","reason":"maybe"}`, http.StatusBadRequest},
		{"bad priority", `{"item_id":1,"item_name":"X","reason":"buy","priority":"urgent"}`, http.StatusBadRequest},
		{"note 281 runes", `{"item_id":1,"item_name":"X","reason":"buy","note":"` + note281 + `"}`, http.StatusBadRequest},
		{"blank custom label", `{"item_id":null,"item_name":"   ","reason":"buy"}`, http.StatusBadRequest},
		{"note 280 runes ok", `{"item_id":1,"item_name":"X","reason":"buy","note":"` + note280 + `"}`, http.StatusOK},
		{"note 280 spaces stored empty", `{"item_id":2,"item_name":"Y","reason":"buy","note":"` + note280spaces + `"}`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, h, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	// The 280-spaces note must have been TRIMMED to NULL, not stored as 280 spaces
	// (review WORTH-FIX 6). item_id=2 is the spaces case.
	var note sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT note FROM wantlist_item WHERE discord_user_id = ? AND item_id = 2`, caller).Scan(&note); err != nil {
		t.Fatalf("read spaces-note row: %v", err)
	}
	if note.Valid {
		t.Fatalf("280-spaces note stored as %q, want NULL (trimmed to empty)", note.String)
	}
}

func TestAddWant_Duplicate_409_OtherReason_200(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-dup"
	seedWebUser(t, ctx, db, caller, "Duper")
	h := withCaller(caller, AddWantHandler(db))

	// First catalog add → 200.
	if rec := postJSON(t, h, `{"item_id":500,"item_name":"Fungi Tunic","reason":"buy"}`); rec.Code != http.StatusOK {
		t.Fatalf("first add status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// Exact duplicate (same item_id + reason) → EXACTLY 409 {"error":"duplicate"}.
	rec := postJSON(t, h, `{"item_id":500,"item_name":"Fungi Tunic","reason":"buy"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "duplicate" {
		t.Fatalf("dup error = %q, want duplicate", got)
	}
	// SAME item, OTHER reason → distinct row, 200.
	if rec := postJSON(t, h, `{"item_id":500,"item_name":"Fungi Tunic","reason":"quest"}`); rec.Code != http.StatusOK {
		t.Fatalf("same-item-other-reason status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// CUSTOM-want duplicate (item_id null, same label+reason) → also 409, proving
	// D-05's no-exact-duplicate rule on the wantlist_custom_uidx path too.
	if rec := postJSON(t, h, `{"item_id":null,"item_name":"My Custom Want","reason":"quest"}`); rec.Code != http.StatusOK {
		t.Fatalf("first custom add status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	recCustom := postJSON(t, h, `{"item_id":null,"item_name":"My Custom Want","reason":"quest"}`)
	if recCustom.Code != http.StatusConflict {
		t.Fatalf("custom dup status = %d, want 409 (body=%s)", recCustom.Code, recCustom.Body.String())
	}
	if got := decodeErr(t, recCustom); got != "duplicate" {
		t.Fatalf("custom dup error = %q, want duplicate", got)
	}
}

func TestListOwnWants_OwnerScoped_NonNilEmpty(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-listw"
	seedWebUser(t, ctx, db, caller, "ListerW")

	// Never added → [].
	listH := withCaller(caller, ListOwnWantsHandler(db))
	rec := httptest.NewRecorder()
	listH.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("empty list status = %d, want 200", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("never-added list body = %q, want []", body)
	}

	// Add one for the caller + one for ANOTHER member that must not show.
	addH := withCaller(caller, AddWantHandler(db))
	if r := postJSON(t, addH, `{"item_id":7,"item_name":"Mine","reason":"buy"}`); r.Code != http.StatusOK {
		t.Fatalf("add own status = %d", r.Code)
	}
	seedWebUser(t, ctx, db, "disc-other", "Other")
	seedWant(t, ctx, db, "disc-other", i64p(99), "Theirs", "buy", "med")

	rec2 := httptest.NewRecorder()
	listH.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	var rows []store.WantlistRow
	if err := json.Unmarshal(rec2.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(rows) != 1 || rows[0].ItemName != "Mine" {
		t.Fatalf("list = %+v, want exactly the caller's one want (other owner excluded)", rows)
	}
}

func TestRemoveOwnWant_CrossOwnerNoOp_OwnRemoved(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-rmw"
	seedWebUser(t, ctx, db, caller, "RemoverW")

	// Another member's want (the IDOR fixture).
	seedWebUser(t, ctx, db, "disc-other", "Other")
	otherID := seedWant(t, ctx, db, "disc-other", i64p(42), "Theirs", "buy", "med")

	// Caller tries to remove the OTHER member's want → removed:false, row untouched.
	revH := withCaller(caller, RemoveOwnWantHandler(db))
	rec := postJSON(t, revH, `{"id":`+itoa(otherID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("cross-owner remove status = %d, want 200", rec.Code)
	}
	if got := decodeRemoved(t, rec); got {
		t.Fatalf("cross-owner remove returned removed=true — IDOR")
	}
	if activeWantCount(t, ctx, db, "disc-other") != 1 {
		t.Fatalf("other member's want was removed — IDOR")
	}
	if c := auditCount(t, ctx, db, "wantlist_remove"); c != 0 {
		t.Fatalf("wantlist_remove audit rows = %d, want 0 (no-op not audited)", c)
	}

	// Caller adds + removes its OWN want → removed:true, audited.
	addH := withCaller(caller, AddWantHandler(db))
	addRec := postJSON(t, addH, `{"item_id":11,"item_name":"Mine","reason":"buy"}`)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add own status = %d", addRec.Code)
	}
	var added store.WantlistRow
	if err := json.Unmarshal(addRec.Body.Bytes(), &added); err != nil {
		t.Fatalf("decode added: %v", err)
	}
	rec2 := postJSON(t, revH, `{"id":`+itoa(added.ID)+`}`)
	if rec2.Code != http.StatusOK {
		t.Fatalf("own remove status = %d, want 200", rec2.Code)
	}
	if !decodeRemoved(t, rec2) {
		t.Fatalf("own remove returned removed=false, want true")
	}
	if activeWantCount(t, ctx, db, caller) != 0 {
		t.Fatalf("own want not removed")
	}
	if c := auditCount(t, ctx, db, "wantlist_remove"); c != 1 {
		t.Fatalf("wantlist_remove audit rows = %d, want 1", c)
	}
}

func TestRemoveOwnWant_BadID_400(t *testing.T) {
	db := store.NewTestDB(t)
	caller := "disc-badid"
	seedWebUser(t, context.Background(), db, caller, "BadID")
	revH := withCaller(caller, RemoveOwnWantHandler(db))
	rec := postJSON(t, revH, `{"id":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Fatalf("error = %q, want invalid_input", got)
	}
}

func TestMuteWant_Toggle_CrossOwnerNoOp_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-mute"
	seedWebUser(t, ctx, db, caller, "Muter")

	// Another member's want (the IDOR fixture).
	seedWebUser(t, ctx, db, "disc-other", "Other")
	otherID := seedWant(t, ctx, db, "disc-other", i64p(42), "Theirs", "buy", "med")

	muteH := withCaller(caller, MuteWantHandler(db))

	// Cross-owner mute → muted echoed as requested, but the other member's row is
	// UNCHANGED (still unmuted) and nothing is audited (silent no-op).
	recX := postJSON(t, muteH, `{"id":`+itoa(otherID)+`,"muted":true}`)
	if recX.Code != http.StatusOK {
		t.Fatalf("cross-owner mute status = %d, want 200", recX.Code)
	}
	if !decodeMuted(t, recX) {
		t.Fatalf("cross-owner mute echo = false, want requested true")
	}
	if mutedOf(t, ctx, db, otherID) {
		t.Fatalf("other member's want was muted — IDOR")
	}
	if c := auditCount(t, ctx, db, "wantlist_mute"); c != 0 {
		t.Fatalf("wantlist_mute audit rows = %d, want 0 (no-op not audited)", c)
	}

	// Caller mutes its OWN want → muted:true, row flipped, audited (want_id only).
	addH := withCaller(caller, AddWantHandler(db))
	addRec := postJSON(t, addH, `{"item_id":11,"item_name":"Mine","reason":"buy"}`)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add own status = %d", addRec.Code)
	}
	var added store.WantlistRow
	if err := json.Unmarshal(addRec.Body.Bytes(), &added); err != nil {
		t.Fatalf("decode added: %v", err)
	}
	recMute := postJSON(t, muteH, `{"id":`+itoa(added.ID)+`,"muted":true}`)
	if recMute.Code != http.StatusOK {
		t.Fatalf("own mute status = %d, want 200", recMute.Code)
	}
	if !decodeMuted(t, recMute) {
		t.Fatalf("own mute echo = false, want true")
	}
	if !mutedOf(t, ctx, db, added.ID) {
		t.Fatalf("own want not muted")
	}
	if c := auditCount(t, ctx, db, "wantlist_mute"); c != 1 {
		t.Fatalf("wantlist_mute audit rows = %d, want 1", c)
	}
	// V7: the audit detail must carry want_id only, never a note/text leak.
	var detail string
	if err := db.QueryRowContext(ctx,
		`SELECT detail FROM audit_log WHERE event = 'wantlist_mute'`).Scan(&detail); err != nil {
		t.Fatalf("read mute audit detail: %v", err)
	}
	if !strings.Contains(detail, "want_id") {
		t.Fatalf("mute audit detail %q missing want_id", detail)
	}

	// Unmute round-trip → muted:false, row cleared.
	recUn := postJSON(t, muteH, `{"id":`+itoa(added.ID)+`,"muted":false}`)
	if recUn.Code != http.StatusOK {
		t.Fatalf("unmute status = %d, want 200", recUn.Code)
	}
	if decodeMuted(t, recUn) {
		t.Fatalf("unmute echo = true, want false")
	}
	if mutedOf(t, ctx, db, added.ID) {
		t.Fatalf("own want still muted after unmute")
	}
}

func TestMuteWant_BadID_400(t *testing.T) {
	db := store.NewTestDB(t)
	caller := "disc-mute-bad"
	seedWebUser(t, context.Background(), db, caller, "BadMute")
	muteH := withCaller(caller, MuteWantHandler(db))
	rec := postJSON(t, muteH, `{"id":0,"muted":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Fatalf("error = %q, want invalid_input", got)
	}
}

// mutedOf reads a want's muted flag by id (the mute round-trip assertion).
func mutedOf(t *testing.T, ctx context.Context, db *sql.DB, wantID int64) bool {
	t.Helper()
	var m int
	if err := db.QueryRowContext(ctx,
		`SELECT muted FROM wantlist_item WHERE id = ?`, wantID).Scan(&m); err != nil {
		t.Fatalf("read muted (id=%d): %v", wantID, err)
	}
	return m != 0
}

// decodeMuted extracts the {"muted":bool} field.
func decodeMuted(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var body struct {
		Muted bool `json:"muted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode muted body %q: %v", rec.Body.String(), err)
	}
	return body.Muted
}

// decodeRemoved extracts the {"removed":bool} field.
func decodeRemoved(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var body struct {
		Removed bool `json:"removed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode removed body %q: %v", rec.Body.String(), err)
	}
	return body.Removed
}
