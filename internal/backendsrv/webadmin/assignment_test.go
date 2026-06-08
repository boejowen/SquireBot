package webadmin

// assignment_test.go — Phase 26 Task 1 (member) + Task 2 (officer), TDD. Proves the
// character-assignment HTTP surface over the 26-01 store layer:
//
//   - MEMBER (RequireSession at the route; here the caller is injected via withCaller):
//     claim an unassigned char (200 claimed); claim a held char (409 already_assigned);
//     claim a bank/bot char (409 char_shared); release a held char (200 released:true)
//     vs a foreign char (200 released:false — silent IDOR no-op); request a contested
//     char then double-request (409 duplicate_request); cancel; a body with a spoofed
//     discord_user_id is ignored (the actor is the session caller).
//   - OFFICER (RequireOfficer at the route + in-tx IsOfficerTx): a non-officer caller →
//     403 not_authorized on every officer endpoint; assign to an unknown assignee → 400
//     invalid_input; assign to a real web_user with a NULL/empty username → 200 (the
//     existence-probe false-reject regression guard); assign a bank/bot char → 409
//     char_shared; approve denies a sibling pending request; designate-bank clears an
//     existing assignment.
//
// Shared helpers reused from officers_test.go / coin_test.go (same package): withCaller,
// postJSON, decodeErr, auditCount, seedFloorAndUsers, seedPlainMember, itoa,
// store.NewTestDB.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// asgInsertChar inserts a plain live character (is_bank_toon=0, is_guild_bot=0,
// is_removed=0) under a throwaway owner and returns its id.
func asgInsertChar(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, `INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert char %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// asgInsertSharedChar inserts a live character flagged as a guild bank (bank=true) or
// guild bot (bank=false → is_guild_bot=1) — a SHARED char that is not claimable.
func asgInsertSharedChar(t *testing.T, ctx context.Context, db *sql.DB, name string, bank bool) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	col := "is_guild_bot"
	if bank {
		col = "is_bank_toon"
	}
	res, err = db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name, `+col+`) VALUES (?, ?, 1)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert shared char %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// asgAssigneeOf reads the current assignee of a char (or "" + false if unassigned).
func asgAssigneeOf(t *testing.T, ctx context.Context, db *sql.DB, charID int64) (string, bool) {
	t.Helper()
	var who string
	err := db.QueryRowContext(ctx,
		`SELECT discord_user_id FROM character_assignment WHERE character_id = ?`, charID).Scan(&who)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		t.Fatalf("read assignee (char=%d): %v", charID, err)
	}
	return who, true
}

// asgPendingCount reads how many pending requests exist for a char.
func asgPendingCount(t *testing.T, ctx context.Context, db *sql.DB, charID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM assignment_request WHERE character_id = ? AND status = 'pending'`, charID,
	).Scan(&n); err != nil {
		t.Fatalf("count pending (char=%d): %v", charID, err)
	}
	return n
}

// --- MEMBER (Task 1) ---------------------------------------------------------

func TestClaim_UnassignedChar_Succeeds_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "Member")
	charID := asgInsertChar(t, ctx, db, "Slampeach")

	h := withCaller(member, ClaimCharHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if who, ok := asgAssigneeOf(t, ctx, db, charID); !ok || who != member {
		t.Errorf("assignee = %q (ok=%v), want %q", who, ok, member)
	}
	if c := auditCount(t, ctx, db, "assignment_claim"); c != 1 {
		t.Errorf("assignment_claim audit rows = %d, want 1", c)
	}
}

func TestClaim_AlreadyAssigned_Conflict(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	holder := "111111111111111111"
	other := "222222222222222222"
	seedPlainMember(t, ctx, db, holder, "Holder")
	seedPlainMember(t, ctx, db, other, "Other")
	charID := asgInsertChar(t, ctx, db, "Held")

	// Holder claims first.
	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("holder claim status = %d, want 200", rec.Code)
	}
	// Other tries to claim the same char → 409 already_assigned.
	rec := postJSON(t, withCaller(other, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "already_assigned" {
		t.Errorf("error = %q, want already_assigned", got)
	}
}

func TestClaim_BankOrBotChar_CharShared(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "Member")

	for _, tc := range []struct {
		name string
		bank bool
	}{
		{"bank", true},
		{"bot", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			charID := asgInsertSharedChar(t, ctx, db, "Shared-"+tc.name, tc.bank)
			rec := postJSON(t, withCaller(member, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`)
			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := decodeErr(t, rec); got != "char_shared" {
				t.Errorf("error = %q, want char_shared", got)
			}
		})
	}
}

func TestClaim_InvalidCharID_BadRequest(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "Member")

	rec := postJSON(t, withCaller(member, ClaimCharHandler(db)), `{"character_id":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
}

func TestRelease_HeldChar_Succeeds_ForeignIsNoOp(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	holder := "111111111111111111"
	stranger := "222222222222222222"
	seedPlainMember(t, ctx, db, holder, "Holder")
	seedPlainMember(t, ctx, db, stranger, "Stranger")
	charID := asgInsertChar(t, ctx, db, "Held")

	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("claim status = %d, want 200", rec.Code)
	}

	// A stranger's release of a char they do NOT hold is a silent no-op: 200 released:false,
	// the row is UNTOUCHED, and nothing is audited (T-26-12).
	rec := postJSON(t, withCaller(stranger, ReleaseCharHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("foreign release status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if released := decodeBoolField(t, rec, "released"); released {
		t.Errorf("released = true on a foreign release, want false (silent no-op)")
	}
	if who, ok := asgAssigneeOf(t, ctx, db, charID); !ok || who != holder {
		t.Errorf("assignee = %q (ok=%v) after foreign release, want still %q", who, ok, holder)
	}

	// The holder's own release succeeds and audits.
	rec2 := postJSON(t, withCaller(holder, ReleaseCharHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec2.Code != http.StatusOK {
		t.Fatalf("holder release status = %d, want 200", rec2.Code)
	}
	if released := decodeBoolField(t, rec2, "released"); !released {
		t.Errorf("released = false on the holder's own release, want true")
	}
	if _, ok := asgAssigneeOf(t, ctx, db, charID); ok {
		t.Errorf("char still assigned after the holder released it")
	}
	if c := auditCount(t, ctx, db, "assignment_release"); c != 1 {
		t.Errorf("assignment_release audit rows = %d, want 1 (foreign no-op audits nothing)", c)
	}
}

func TestRequest_Contested_ThenDuplicate_Conflict(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	holder := "111111111111111111"
	requester := "222222222222222222"
	seedPlainMember(t, ctx, db, holder, "Holder")
	seedPlainMember(t, ctx, db, requester, "Requester")
	charID := asgInsertChar(t, ctx, db, "Contested")

	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("holder claim status = %d, want 200", rec.Code)
	}

	// First request → 200 requested, one pending row, audited.
	rec := postJSON(t, withCaller(requester, RequestCharHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("request status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if n := asgPendingCount(t, ctx, db, charID); n != 1 {
		t.Fatalf("pending count = %d, want 1", n)
	}
	if c := auditCount(t, ctx, db, "assignment_request"); c != 1 {
		t.Errorf("assignment_request audit rows = %d, want 1", c)
	}

	// Second request from the same member for the same char → 409 duplicate_request.
	rec2 := postJSON(t, withCaller(requester, RequestCharHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("duplicate request status = %d, want 409 (body=%s)", rec2.Code, rec2.Body.String())
	}
	if got := decodeErr(t, rec2); got != "duplicate_request" {
		t.Errorf("error = %q, want duplicate_request", got)
	}
}

func TestCancelRequest_OwnPending_ThenForeignNoOp(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	holder := "111111111111111111"
	requester := "222222222222222222"
	stranger := "333333333333333333"
	seedPlainMember(t, ctx, db, holder, "Holder")
	seedPlainMember(t, ctx, db, requester, "Requester")
	seedPlainMember(t, ctx, db, stranger, "Stranger")
	charID := asgInsertChar(t, ctx, db, "Contested")

	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("holder claim status = %d, want 200", rec.Code)
	}
	if rec := postJSON(t, withCaller(requester, RequestCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("request status = %d, want 200", rec.Code)
	}

	// A stranger cancelling the requester's pending request is a silent no-op (false),
	// the pending row survives.
	rec := postJSON(t, withCaller(stranger, CancelRequestHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("foreign cancel status = %d, want 200", rec.Code)
	}
	if cancelled := decodeBoolField(t, rec, "cancelled"); cancelled {
		t.Errorf("cancelled = true on a foreign cancel, want false")
	}
	if n := asgPendingCount(t, ctx, db, charID); n != 1 {
		t.Errorf("pending count = %d after foreign cancel, want 1 (untouched)", n)
	}

	// The requester's own cancel succeeds, audited.
	rec2 := postJSON(t, withCaller(requester, CancelRequestHandler(db)), `{"character_id":`+itoa(charID)+`}`)
	if rec2.Code != http.StatusOK {
		t.Fatalf("own cancel status = %d, want 200", rec2.Code)
	}
	if cancelled := decodeBoolField(t, rec2, "cancelled"); !cancelled {
		t.Errorf("cancelled = false on the requester's own cancel, want true")
	}
	if n := asgPendingCount(t, ctx, db, charID); n != 0 {
		t.Errorf("pending count = %d after own cancel, want 0", n)
	}
	if c := auditCount(t, ctx, db, "request_cancel"); c != 1 {
		t.Errorf("request_cancel audit rows = %d, want 1", c)
	}
}

func TestClaim_SpoofedDiscordIDInBody_Ignored(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	victim := "999999999999999999"
	seedPlainMember(t, ctx, db, member, "Member")
	seedPlainMember(t, ctx, db, victim, "Victim")
	charID := asgInsertChar(t, ctx, db, "Slampeach")

	// The body carries a spoofed discord_user_id (the victim). The actor MUST still be
	// the session caller (member) — the body field is structurally ignored (Pitfall 1).
	h := withCaller(member, ClaimCharHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"discord_user_id":"`+victim+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if who, _ := asgAssigneeOf(t, ctx, db, charID); who != member {
		t.Errorf("assignee = %q, want the SESSION caller %q (the spoofed body id was honored)", who, member)
	}
}

func TestListMyAssignments_And_Claimable(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "Member")

	mine := asgInsertChar(t, ctx, db, "Mine")
	claimable := asgInsertChar(t, ctx, db, "Free")
	_ = asgInsertSharedChar(t, ctx, db, "GuildBank", true) // shared: never claimable

	if rec := postJSON(t, withCaller(member, ClaimCharHandler(db)), `{"character_id":`+itoa(mine)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("claim status = %d, want 200", rec.Code)
	}

	// GET /assignments/mine → the claimed char only.
	var assigned []store.Assignment
	getJSONInto(t, withCaller(member, ListMyAssignmentsHandler(db)), &assigned)
	if len(assigned) != 1 || assigned[0].CharacterID != mine {
		t.Errorf("mine = %+v, want exactly the claimed char %d", assigned, mine)
	}

	// GET /assignments/claimable → the free char, NOT the claimed one, NOT the bank.
	var free []store.ClaimableChar
	getJSONInto(t, withCaller(member, ClaimableHandler(db)), &free)
	sawClaimable, sawMine := false, false
	for _, c := range free {
		if c.CharacterID == claimable {
			sawClaimable = true
		}
		if c.CharacterID == mine {
			sawMine = true
		}
	}
	if !sawClaimable {
		t.Errorf("claimable list missing the free char %d: %+v", claimable, free)
	}
	if sawMine {
		t.Errorf("claimable list includes the already-claimed char %d", mine)
	}
}

func TestListMyAssignments_EmptyIsArrayNotNull(t *testing.T) {
	db := store.NewTestDB(t)
	member := "555555555555555555"
	seedPlainMember(t, context.Background(), db, member, "Member")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	withCaller(member, ListMyAssignmentsHandler(db)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "[]\n" && body != "[]" {
		t.Errorf("empty mine body = %q, want []", body)
	}
}

// --- OFFICER (Task 2) --------------------------------------------------------

// officerEndpoints enumerates every officer handler + a minimal valid body, so a
// single table-driven test can assert a non-officer caller is rejected on ALL of them.
func officerEndpoints(db *sql.DB, charID, requestID int64, assignee string) []struct {
	name string
	h    http.Handler
	body string
} {
	return []struct {
		name string
		h    http.Handler
		body string
	}{
		{"assign", OfficerAssignHandler(db), `{"character_id":` + itoa(charID) + `,"assignee":"` + assignee + `"}`},
		{"remove", OfficerRemoveAssignHandler(db), `{"character_id":` + itoa(charID) + `}`},
		{"approve", ApproveRequestHandler(db), `{"request_id":` + itoa(requestID) + `}`},
		{"deny", DenyRequestHandler(db), `{"request_id":` + itoa(requestID) + `}`},
		{"designate", DesignateCharHandler(db), `{"character_id":` + itoa(charID) + `,"mode":"bank"}`},
	}
}

func TestOfficerEndpoints_NonOfficer_Rejected(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	stranger := "222222222222222222"
	assignee := "333333333333333333"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{stranger: "Stranger", assignee: "Assignee"})
	charID := asgInsertChar(t, ctx, db, "AnyChar")

	for _, ep := range officerEndpoints(db, charID, 1, assignee) {
		t.Run(ep.name, func(t *testing.T) {
			rec := postJSON(t, withCaller(stranger, ep.h), ep.body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := decodeErr(t, rec); got != "not_authorized" {
				t.Errorf("error = %q, want not_authorized", got)
			}
		})
	}
}

func TestOfficerAssign_UnknownAssignee_BadRequest(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)
	charID := asgInsertChar(t, ctx, db, "AnyChar")

	// "777..." is not a web_user → the existence probe yields 400 invalid_input (NOT a
	// 500 FK violation, NOT a silent assign).
	rec := postJSON(t, withCaller(floor, OfficerAssignHandler(db)),
		`{"character_id":`+itoa(charID)+`,"assignee":"777777777777777777"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
	if _, ok := asgAssigneeOf(t, ctx, db, charID); ok {
		t.Errorf("char assigned despite an unknown assignee")
	}
}

// TestOfficerAssign_NullUsernameAssignee_Succeeds is the false-reject regression guard
// for the existence-probe fix: a REAL web_user whose username column is empty is still a
// valid assignment target. usernameOf would return "" and wrongly reject it; the
// SELECT 1 existence probe accepts it. (web_user.username is NOT NULL, so the realizable
// false-reject is an EMPTY-string username, not a literal NULL — usernameOf returns ""
// for both, so the probe-vs-usernameOf distinction is exactly what this guards.)
func TestOfficerAssign_NullUsernameAssignee_Succeeds(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)
	charID := asgInsertChar(t, ctx, db, "AnyChar")

	// A real web_user with an EMPTY username (the row exists; usernameOf would return ""
	// and false-reject it — the existence probe must still accept the row).
	nullUser := "444444444444444444"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, '', NULL, 0, 0)`, nullUser); err != nil {
		t.Fatalf("insert empty-username web_user: %v", err)
	}

	rec := postJSON(t, withCaller(floor, OfficerAssignHandler(db)),
		`{"character_id":`+itoa(charID)+`,"assignee":"`+nullUser+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (a NULL-username real user is a valid target) (body=%s)", rec.Code, rec.Body.String())
	}
	if who, ok := asgAssigneeOf(t, ctx, db, charID); !ok || who != nullUser {
		t.Errorf("assignee = %q (ok=%v), want %q", who, ok, nullUser)
	}
	if c := auditCount(t, ctx, db, "officer_assign"); c != 1 {
		t.Errorf("officer_assign audit rows = %d, want 1", c)
	}
}

func TestOfficerAssign_BankChar_CharShared(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	assignee := "333333333333333333"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{assignee: "Assignee"})
	bankID := asgInsertSharedChar(t, ctx, db, "GuildBank", true)

	rec := postJSON(t, withCaller(floor, OfficerAssignHandler(db)),
		`{"character_id":`+itoa(bankID)+`,"assignee":"`+assignee+`"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "char_shared" {
		t.Errorf("error = %q, want char_shared (mapAssignErr, NOT mapOfficerErr)", got)
	}
}

func TestApprove_DeniesSiblingPendingRequest(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	holder := "222222222222222222"
	reqA := "333333333333333333"
	reqB := "444444444444444444"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{holder: "Holder", reqA: "ReqA", reqB: "ReqB"})
	charID := asgInsertChar(t, ctx, db, "Contested")

	// Holder claims; reqA and reqB both file requests.
	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("holder claim status = %d", rec.Code)
	}
	if rec := postJSON(t, withCaller(reqA, RequestCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("reqA request status = %d", rec.Code)
	}
	if rec := postJSON(t, withCaller(reqB, RequestCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("reqB request status = %d", rec.Code)
	}
	if n := asgPendingCount(t, ctx, db, charID); n != 2 {
		t.Fatalf("pending count = %d, want 2 before approval", n)
	}

	// Find reqA's request id.
	var reqAID int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM assignment_request WHERE character_id = ? AND requester = ? AND status = 'pending'`,
		charID, reqA).Scan(&reqAID); err != nil {
		t.Fatalf("find reqA request id: %v", err)
	}

	// Officer approves reqA → char reassigned to reqA, reqB's pending request denied.
	rec := postJSON(t, withCaller(floor, ApproveRequestHandler(db)), `{"request_id":`+itoa(reqAID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if who, ok := asgAssigneeOf(t, ctx, db, charID); !ok || who != reqA {
		t.Errorf("assignee = %q (ok=%v) after approval, want %q", who, ok, reqA)
	}
	if n := asgPendingCount(t, ctx, db, charID); n != 0 {
		t.Errorf("pending count = %d after approval, want 0 (sibling denied — Pitfall 3)", n)
	}
	if c := auditCount(t, ctx, db, "request_approve"); c != 1 {
		t.Errorf("request_approve audit rows = %d, want 1", c)
	}
}

func TestDesignateBank_ClearsExistingAssignment(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	holder := "222222222222222222"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{holder: "Holder"})
	charID := asgInsertChar(t, ctx, db, "SoonBank")

	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("holder claim status = %d", rec.Code)
	}
	if _, ok := asgAssigneeOf(t, ctx, db, charID); !ok {
		t.Fatalf("char not assigned after claim")
	}

	// Officer designates it a guild bank → the assignment is cleared (Pitfall 6).
	rec := postJSON(t, withCaller(floor, DesignateCharHandler(db)), `{"character_id":`+itoa(charID)+`,"mode":"bank"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("designate status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if _, ok := asgAssigneeOf(t, ctx, db, charID); ok {
		t.Errorf("assignment NOT cleared after bank designation (Pitfall 6)")
	}
	var isBank int
	if err := db.QueryRowContext(ctx, `SELECT is_bank_toon FROM character WHERE id = ?`, charID).Scan(&isBank); err != nil {
		t.Fatalf("read is_bank_toon: %v", err)
	}
	if isBank != 1 {
		t.Errorf("is_bank_toon = %d, want 1", isBank)
	}
	if c := auditCount(t, ctx, db, "char_designate"); c != 1 {
		t.Errorf("char_designate audit rows = %d, want 1", c)
	}
}

func TestDesignate_InvalidMode_BadRequest(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)
	charID := asgInsertChar(t, ctx, db, "AnyChar")

	rec := postJSON(t, withCaller(floor, DesignateCharHandler(db)), `{"character_id":`+itoa(charID)+`,"mode":"banana"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
}

func TestListAllAssignments_ReturnsAssignmentsAndRequests(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	holder := "222222222222222222"
	requester := "333333333333333333"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{holder: "Holder", requester: "Requester"})
	charID := asgInsertChar(t, ctx, db, "Contested")

	if rec := postJSON(t, withCaller(holder, ClaimCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("holder claim status = %d", rec.Code)
	}
	if rec := postJSON(t, withCaller(requester, RequestCharHandler(db)), `{"character_id":`+itoa(charID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("request status = %d", rec.Code)
	}

	var resp struct {
		Assignments []store.Assignment     `json:"assignments"`
		Requests    []store.PendingRequest `json:"requests"`
	}
	getJSONInto(t, withCaller(floor, ListAllAssignmentsHandler(db)), &resp)
	if len(resp.Assignments) != 1 || resp.Assignments[0].DiscordUserID != holder {
		t.Errorf("assignments = %+v, want one held by %q", resp.Assignments, holder)
	}
	if len(resp.Requests) != 1 || resp.Requests[0].Requester != requester {
		t.Errorf("requests = %+v, want one from %q", resp.Requests, requester)
	}
}

// --- small test decode helpers ----------------------------------------------

// decodeBoolField extracts a single named bool from a JSON response body.
func decodeBoolField(t *testing.T, rec *httptest.ResponseRecorder, field string) bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	v, ok := m[field].(bool)
	if !ok {
		t.Fatalf("field %q missing or not a bool in %q", field, rec.Body.String())
	}
	return v
}

// getJSONInto issues a GET against a handler and unmarshals the 200 body into out.
func getJSONInto(t *testing.T, h http.Handler, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode GET body %q: %v", rec.Body.String(), err)
	}
}
