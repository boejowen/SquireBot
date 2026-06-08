package store

// assignment_test.go covers the Phase 26 character→user assignment data layer
// (ASSIGN-01..06, 26-01) at the store seam: the claim/release/request/cancel member
// mutators, the officer assign/remove/approve/deny/designate mutators (authorize-under-
// tx), the bidirectional shared-char exemption (Pitfall 6), the double-approval defense
// (Pitfall 3), the owner-scoped silent-no-op (IDOR), the partial-unique duplicate-request
// mapping, and the List reads. Reuses the package test helpers insertOwner/insertChar/
// insertWebUser/commitTx (eviction_test.go / admins_test.go).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// makeOfficer seeds a web_user + a guild_admins row so callerID passes the in-tx
// officer re-check. now is the seed timestamp.
func makeOfficer(t *testing.T, ctx context.Context, db *sql.DB, id string) {
	t.Helper()
	insertWebUser(t, ctx, db, id, "Officer-"+id)
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO guild_admins (discord_user_id, added_at, added_by) VALUES (?, 0, 'test')`, id,
	); err != nil {
		t.Fatalf("seed officer %q: %v", id, err)
	}
}

// setGuildBot flags charID is_guild_bot=1 directly (insertChar only sets is_bank_toon).
func setGuildBot(t *testing.T, ctx context.Context, db *sql.DB, charID int64) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `UPDATE character SET is_guild_bot = 1 WHERE id = ?`, charID); err != nil {
		t.Fatalf("set is_guild_bot (id=%d): %v", charID, err)
	}
}

// assigneeOf returns the assignment's discord_user_id (and whether a row exists).
func assigneeOf(t *testing.T, ctx context.Context, db *sql.DB, charID int64) (string, bool) {
	t.Helper()
	var who string
	err := db.QueryRowContext(ctx,
		`SELECT discord_user_id FROM character_assignment WHERE character_id = ?`, charID,
	).Scan(&who)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatalf("read assignee (id=%d): %v", charID, err)
	}
	return who, true
}

// requestStatus returns a request's status by id.
func requestStatus(t *testing.T, ctx context.Context, db *sql.DB, reqID int64) string {
	t.Helper()
	var s string
	if err := db.QueryRowContext(ctx, `SELECT status FROM assignment_request WHERE id = ?`, reqID).Scan(&s); err != nil {
		t.Fatalf("read request status (id=%d): %v", reqID, err)
	}
	return s
}

// pendingRequestID files a pending request for (charID, requester) directly and
// returns its id (a test fixture for the officer-resolution cases).
func pendingRequestID(t *testing.T, ctx context.Context, db *sql.DB, charID int64, requester string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		`INSERT INTO assignment_request (character_id, requester, status, created_at) VALUES (?, ?, 'pending', 0)`,
		charID, requester)
	if err != nil {
		t.Fatalf("seed pending request (char=%d, requester=%q): %v", charID, requester, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("pending request last insert id: %v", err)
	}
	return id
}

// TestClaimCharTx_HappyAndAlreadyAssigned: a first claim assigns the char to the
// caller (assigned_by='self'); a second claim (by anyone) → ErrCharAlreadyAssigned.
func TestClaimCharTx_HappyAndAlreadyAssigned(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "member-1", "Member1")
	insertWebUser(t, ctx, db, "member-2", "Member2")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "member-1", 100)
	}); err != nil {
		t.Fatalf("ClaimCharTx (first): %v", err)
	}
	who, ok := assigneeOf(t, ctx, db, charID)
	if !ok || who != "member-1" {
		t.Errorf("after claim: assignee = (%q, %v), want member-1, true", who, ok)
	}
	var by string
	if err := db.QueryRowContext(ctx, `SELECT assigned_by FROM character_assignment WHERE character_id = ?`, charID).Scan(&by); err != nil {
		t.Fatalf("read assigned_by: %v", err)
	}
	if by != "self" {
		t.Errorf("assigned_by = %q, want self", by)
	}

	// A second claim (even by a different member) → ErrCharAlreadyAssigned.
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "member-2", 200)
	})
	if !errors.Is(err, ErrCharAlreadyAssigned) {
		t.Errorf("second claim: err = %v, want ErrCharAlreadyAssigned", err)
	}
}

// TestClaimCharTx_RejectsSharedChars: a guild bank (is_bank_toon=1) and a guild bot
// (is_guild_bot=1) are not claimable → ErrCharShared.
func TestClaimCharTx_RejectsSharedChars(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	insertWebUser(t, ctx, db, "member-1", "Member1")

	bank := insertChar(t, ctx, db, ownerID, "Guildbank", true)
	bot := insertChar(t, ctx, db, ownerID, "Guildbot", false)
	setGuildBot(t, ctx, db, bot)

	for _, c := range []struct {
		name string
		id   int64
	}{{"bank", bank}, {"bot", bot}} {
		err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
			return ClaimCharTx(ctx, tx, c.id, "member-1", 100)
		})
		if !errors.Is(err, ErrCharShared) {
			t.Errorf("claim %s char: err = %v, want ErrCharShared", c.name, err)
		}
		if _, ok := assigneeOf(t, ctx, db, c.id); ok {
			t.Errorf("a shared %s char got an assignment row", c.name)
		}
	}
}

// TestReleaseCharTx_OwnerScopedSilentNoOp: a member releases their own char (removed=
// true, row gone); a FOREIGN-row release affects 0 rows → (false, nil) and leaves the
// real owner's assignment intact (IDOR defense).
func TestReleaseCharTx_OwnerScopedSilentNoOp(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "member-1", "Member1")
	insertWebUser(t, ctx, db, "member-2", "Member2")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "member-1", 100)
	}); err != nil {
		t.Fatalf("ClaimCharTx: %v", err)
	}

	// A FOREIGN member's release is a silent no-op (false, nil) — the assignment stays.
	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = ReleaseCharTx(ctx, tx, charID, "member-2")
		return e
	}); err != nil {
		t.Fatalf("ReleaseCharTx (foreign): %v", err)
	}
	if removed {
		t.Errorf("foreign-row release returned removed=true; want false (silent no-op)")
	}
	if who, ok := assigneeOf(t, ctx, db, charID); !ok || who != "member-1" {
		t.Errorf("after foreign release: assignee = (%q, %v), want member-1 still assigned", who, ok)
	}

	// The real owner's release removes it (removed=true).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = ReleaseCharTx(ctx, tx, charID, "member-1")
		return e
	}); err != nil {
		t.Fatalf("ReleaseCharTx (owner): %v", err)
	}
	if !removed {
		t.Errorf("owner release returned removed=false; want true")
	}
	if _, ok := assigneeOf(t, ctx, db, charID); ok {
		t.Errorf("char still assigned after the owner released it")
	}
}

// TestRequestTx_DuplicatePending: a first pending request files fine; a SECOND from the
// same requester for the same char → ErrDuplicateRequest (partial-unique pending index).
func TestRequestTx_DuplicatePending(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "holder", "Holder")
	insertWebUser(t, ctx, db, "requester", "Requester")

	// The char is held by someone else (a contested claim).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "holder", 100)
	}); err != nil {
		t.Fatalf("seed holder claim: %v", err)
	}

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return RequestTx(ctx, tx, charID, "requester", 200)
	}); err != nil {
		t.Fatalf("RequestTx (first): %v", err)
	}
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return RequestTx(ctx, tx, charID, "requester", 300)
	})
	if !errors.Is(err, ErrDuplicateRequest) {
		t.Errorf("second pending request: err = %v, want ErrDuplicateRequest", err)
	}
}

// TestCancelRequestTx_RequesterScoped: a requester cancels their own pending request
// (cancelled=true, status='cancelled'); a foreign/non-pending cancel → (false, nil).
func TestCancelRequestTx_RequesterScoped(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "requester", "Requester")
	insertWebUser(t, ctx, db, "stranger", "Stranger")
	reqID := pendingRequestID(t, ctx, db, charID, "requester")

	// A STRANGER cannot cancel another member's request (false, nil) — it stays pending.
	var cancelled bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		cancelled, e = CancelRequestTx(ctx, tx, charID, "stranger", 400)
		return e
	}); err != nil {
		t.Fatalf("CancelRequestTx (foreign): %v", err)
	}
	if cancelled {
		t.Errorf("foreign cancel returned cancelled=true; want false")
	}
	if s := requestStatus(t, ctx, db, reqID); s != "pending" {
		t.Errorf("after foreign cancel: status = %q, want pending", s)
	}

	// The requester cancels their own (cancelled=true).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		cancelled, e = CancelRequestTx(ctx, tx, charID, "requester", 500)
		return e
	}); err != nil {
		t.Fatalf("CancelRequestTx (owner): %v", err)
	}
	if !cancelled {
		t.Errorf("requester cancel returned cancelled=false; want true")
	}
	if s := requestStatus(t, ctx, db, reqID); s != "cancelled" {
		t.Errorf("after requester cancel: status = %q, want cancelled", s)
	}
}

// TestOfficerAssignTx_AuthorizeAndReassign: a non-officer caller → ErrNotAuthorized; an
// officer assigns then reassigns (override, ON CONFLICT) — one row, the new assignee.
func TestOfficerAssignTx_AuthorizeAndReassign(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "alice", "Alice")
	insertWebUser(t, ctx, db, "bob", "Bob")
	makeOfficer(t, ctx, db, "officer-1")

	// Non-officer → ErrNotAuthorized (and no write).
	insertWebUser(t, ctx, db, "rando", "Rando")
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return OfficerAssignTx(ctx, tx, charID, "alice", "rando", 100)
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer assign: err = %v, want ErrNotAuthorized", err)
	}
	if _, ok := assigneeOf(t, ctx, db, charID); ok {
		t.Errorf("a non-officer assign wrote an assignment row")
	}

	// Officer assigns to alice.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return OfficerAssignTx(ctx, tx, charID, "alice", "officer-1", 200)
	}); err != nil {
		t.Fatalf("OfficerAssignTx (alice): %v", err)
	}
	if who, _ := assigneeOf(t, ctx, db, charID); who != "alice" {
		t.Errorf("assignee = %q, want alice", who)
	}

	// Officer reassigns to bob (override, ON CONFLICT — single row).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return OfficerAssignTx(ctx, tx, charID, "bob", "officer-1", 300)
	}); err != nil {
		t.Fatalf("OfficerAssignTx (bob reassign): %v", err)
	}
	if who, _ := assigneeOf(t, ctx, db, charID); who != "bob" {
		t.Errorf("after reassign: assignee = %q, want bob", who)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_assignment WHERE character_id = ?`, charID).Scan(&n); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if n != 1 {
		t.Errorf("reassign produced %d rows, want exactly 1 (PK upsert)", n)
	}
}

// TestOfficerAssignTx_RejectsShared: an officer assigning a guild bank/bot char →
// ErrCharShared.
func TestOfficerAssignTx_RejectsShared(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Guildbank", true)
	insertWebUser(t, ctx, db, "alice", "Alice")
	makeOfficer(t, ctx, db, "officer-1")

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return OfficerAssignTx(ctx, tx, bank, "alice", "officer-1", 100)
	})
	if !errors.Is(err, ErrCharShared) {
		t.Errorf("officer assign of a bank char: err = %v, want ErrCharShared", err)
	}
}

// TestRemoveAssignTx_AuthorizeAndIdempotent: a non-officer → ErrNotAuthorized; an
// officer removes the assignment (removed=true), a second remove is a no-op (false).
func TestRemoveAssignTx_AuthorizeAndIdempotent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "alice", "Alice")
	insertWebUser(t, ctx, db, "rando", "Rando")
	makeOfficer(t, ctx, db, "officer-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return OfficerAssignTx(ctx, tx, charID, "alice", "officer-1", 100)
	}); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	// Non-officer remove → ErrNotAuthorized.
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := RemoveAssignTx(ctx, tx, charID, "rando")
		return e
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer remove: err = %v, want ErrNotAuthorized", err)
	}

	// Officer remove (removed=true).
	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveAssignTx(ctx, tx, charID, "officer-1")
		return e
	}); err != nil {
		t.Fatalf("RemoveAssignTx: %v", err)
	}
	if !removed {
		t.Errorf("officer remove returned removed=false; want true")
	}
	if _, ok := assigneeOf(t, ctx, db, charID); ok {
		t.Errorf("assignment still present after officer remove")
	}

	// A second remove is an idempotent no-op (false).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveAssignTx(ctx, tx, charID, "officer-1")
		return e
	}); err != nil {
		t.Fatalf("RemoveAssignTx (second): %v", err)
	}
	if removed {
		t.Errorf("second remove returned removed=true; want false (idempotent no-op)")
	}
}

// TestApproveRequestTx_DeniesSiblings (Pitfall 3 — double-approval defense): two members
// file pending requests for the same char; approving member A's request reassigns the
// char to A AND denies member B's still-pending request, in one tx. A non-officer caller
// → ErrNotAuthorized.
func TestApproveRequestTx_DeniesSiblings(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "holder", "Holder")
	insertWebUser(t, ctx, db, "alice", "Alice")
	insertWebUser(t, ctx, db, "bob", "Bob")
	insertWebUser(t, ctx, db, "rando", "Rando")
	makeOfficer(t, ctx, db, "officer-1")

	// The char is held by someone else; alice + bob each file a pending request.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "holder", 100)
	}); err != nil {
		t.Fatalf("seed holder: %v", err)
	}
	aliceReq := pendingRequestID(t, ctx, db, charID, "alice")
	bobReq := pendingRequestID(t, ctx, db, charID, "bob")

	// A non-officer cannot approve.
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := ApproveRequestTx(ctx, tx, aliceReq, "rando", 200)
		return e
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer approve: err = %v, want ErrNotAuthorized", err)
	}

	// The officer approves alice's request.
	var approved bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		approved, e = ApproveRequestTx(ctx, tx, aliceReq, "officer-1", 300)
		return e
	}); err != nil {
		t.Fatalf("ApproveRequestTx: %v", err)
	}
	if !approved {
		t.Errorf("ApproveRequestTx returned approved=false; want true")
	}
	// The char is now assigned to alice (override of the holder).
	if who, _ := assigneeOf(t, ctx, db, charID); who != "alice" {
		t.Errorf("after approve: assignee = %q, want alice", who)
	}
	// Alice's request is approved; Bob's sibling pending request is DENIED.
	if s := requestStatus(t, ctx, db, aliceReq); s != "approved" {
		t.Errorf("alice request status = %q, want approved", s)
	}
	if s := requestStatus(t, ctx, db, bobReq); s != "denied" {
		t.Errorf("bob sibling request status = %q, want denied (Pitfall 3)", s)
	}
}

// TestDenyRequestTx_AuthorizeAndResolve: a non-officer → ErrNotAuthorized; an officer
// denies the one pending request (status='denied').
func TestDenyRequestTx_AuthorizeAndResolve(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	insertWebUser(t, ctx, db, "alice", "Alice")
	insertWebUser(t, ctx, db, "rando", "Rando")
	makeOfficer(t, ctx, db, "officer-1")
	reqID := pendingRequestID(t, ctx, db, charID, "alice")

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := DenyRequestTx(ctx, tx, reqID, "rando", 100)
		return e
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer deny: err = %v, want ErrNotAuthorized", err)
	}

	var denied bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		denied, e = DenyRequestTx(ctx, tx, reqID, "officer-1", 200)
		return e
	}); err != nil {
		t.Fatalf("DenyRequestTx: %v", err)
	}
	if !denied {
		t.Errorf("DenyRequestTx returned denied=false; want true")
	}
	if s := requestStatus(t, ctx, db, reqID); s != "denied" {
		t.Errorf("request status = %q, want denied", s)
	}
}

// TestDesignateCharTx_ClearsAssignmentAndDeniesRequests (Pitfall 6 — bidirectional
// exemption): designating a char a guild bank, in the same tx, DELETEs its existing
// assignment AND denies its pending requests. A non-officer → ErrNotAuthorized. It does
// NOT demote OTHER bank toons (multiple guild banks allowed). 'neither' clears the flags.
func TestDesignateCharTx_ClearsAssignmentAndDeniesRequests(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	charID := insertChar(t, ctx, db, ownerID, "Slampeach", false)
	otherBank := insertChar(t, ctx, db, ownerID, "Oldbank", true) // a pre-existing guild bank
	insertWebUser(t, ctx, db, "alice", "Alice")
	insertWebUser(t, ctx, db, "bob", "Bob")
	insertWebUser(t, ctx, db, "rando", "Rando")
	makeOfficer(t, ctx, db, "officer-1")

	// charID is assigned to alice + has a pending request from bob.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return ClaimCharTx(ctx, tx, charID, "alice", 100)
	}); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	bobReq := pendingRequestID(t, ctx, db, charID, "bob")

	// Non-officer designate → ErrNotAuthorized.
	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateBank, "rando", 200)
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("non-officer designate: err = %v, want ErrNotAuthorized", err)
	}

	// Officer designates charID a guild bank.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateBank, "officer-1", 300)
	}); err != nil {
		t.Fatalf("DesignateCharTx (bank): %v", err)
	}
	// The char is now a bank, its assignment is gone, and bob's request is denied.
	var isBank, isBot int
	if err := db.QueryRowContext(ctx, `SELECT is_bank_toon, is_guild_bot FROM character WHERE id = ?`, charID).Scan(&isBank, &isBot); err != nil {
		t.Fatalf("read flags: %v", err)
	}
	if isBank != 1 || isBot != 0 {
		t.Errorf("flags after bank designate = is_bank_toon=%d is_guild_bot=%d, want 1,0", isBank, isBot)
	}
	if _, ok := assigneeOf(t, ctx, db, charID); ok {
		t.Errorf("assignment survived a guild-bank designation (Pitfall 6)")
	}
	if s := requestStatus(t, ctx, db, bobReq); s != "denied" {
		t.Errorf("pending request status after bank designate = %q, want denied (Pitfall 6)", s)
	}

	// The OTHER pre-existing bank toon is NOT demoted (multiple guild banks allowed).
	var otherIsBank int
	if err := db.QueryRowContext(ctx, `SELECT is_bank_toon FROM character WHERE id = ?`, otherBank).Scan(&otherIsBank); err != nil {
		t.Fatalf("read other bank flag: %v", err)
	}
	if otherIsBank != 1 {
		t.Errorf("the pre-existing bank toon was demoted (otherIsBank=%d); multiple guild banks must be allowed", otherIsBank)
	}

	// Designating 'neither' clears the flags (the char becomes claimable again).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, charID, DesignateNeither, "officer-1", 400)
	}); err != nil {
		t.Fatalf("DesignateCharTx (neither): %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT is_bank_toon, is_guild_bot FROM character WHERE id = ?`, charID).Scan(&isBank, &isBot); err != nil {
		t.Fatalf("read flags after neither: %v", err)
	}
	if isBank != 0 || isBot != 0 {
		t.Errorf("flags after 'neither' = is_bank_toon=%d is_guild_bot=%d, want 0,0", isBank, isBot)
	}
}

// TestDesignateCharTx_BotMutualExclusion: designating a char a guild bot sets
// is_guild_bot=1 AND is_bank_toon=0 (mutually exclusive), even if it was a bank before.
func TestDesignateCharTx_BotMutualExclusion(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Wasbank", true)
	makeOfficer(t, ctx, db, "officer-1")

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, bank, DesignateBot, "officer-1", 100)
	}); err != nil {
		t.Fatalf("DesignateCharTx (bot): %v", err)
	}
	var isBank, isBot int
	if err := db.QueryRowContext(ctx, `SELECT is_bank_toon, is_guild_bot FROM character WHERE id = ?`, bank).Scan(&isBank, &isBot); err != nil {
		t.Fatalf("read flags: %v", err)
	}
	if isBank != 0 || isBot != 1 {
		t.Errorf("flags after bot designate = is_bank_toon=%d is_guild_bot=%d, want 0,1 (mutual exclusion)", isBank, isBot)
	}
}

// TestDesignateCharTx_MissingChar: designating a missing/removed char → ErrCharNotFound.
func TestDesignateCharTx_MissingChar(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	makeOfficer(t, ctx, db, "officer-1")

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return DesignateCharTx(ctx, tx, 999999, DesignateBank, "officer-1", 100)
	})
	if !errors.Is(err, ErrCharNotFound) {
		t.Errorf("designate missing char: err = %v, want ErrCharNotFound", err)
	}
}

// TestListAssignmentsAndPendingRequests: ListMyAssignments scopes to the caller;
// ListAllAssignments returns every assignment; ListPendingRequests returns the pending
// queue with the contested character's name.
func TestListAssignmentsAndPendingRequests(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	c1 := insertChar(t, ctx, db, ownerID, "Aaa", false)
	c2 := insertChar(t, ctx, db, ownerID, "Bbb", false)
	c3 := insertChar(t, ctx, db, ownerID, "Ccc", false)
	insertWebUser(t, ctx, db, "alice", "Alice")
	insertWebUser(t, ctx, db, "bob", "Bob")

	// alice holds c1 + c2; bob holds c3.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		if e := ClaimCharTx(ctx, tx, c1, "alice", 100); e != nil {
			return e
		}
		if e := ClaimCharTx(ctx, tx, c2, "alice", 100); e != nil {
			return e
		}
		return ClaimCharTx(ctx, tx, c3, "bob", 100)
	}); err != nil {
		t.Fatalf("seed claims: %v", err)
	}
	// bob files a pending request for c1 (alice's char).
	pendingRequestID(t, ctx, db, c1, "bob")

	mine, err := ListMyAssignments(ctx, db, "alice")
	if err != nil {
		t.Fatalf("ListMyAssignments: %v", err)
	}
	if len(mine) != 2 || mine[0].Name != "Aaa" || mine[1].Name != "Bbb" {
		t.Errorf("ListMyAssignments(alice) = %+v, want [Aaa, Bbb]", mine)
	}

	all, err := ListAllAssignments(ctx, db)
	if err != nil {
		t.Fatalf("ListAllAssignments: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListAllAssignments = %d rows, want 3", len(all))
	}

	pending, err := ListPendingRequests(ctx, db)
	if err != nil {
		t.Fatalf("ListPendingRequests: %v", err)
	}
	if len(pending) != 1 || pending[0].CharacterName != "Aaa" || pending[0].Requester != "bob" {
		t.Errorf("ListPendingRequests = %+v, want one (Aaa, bob)", pending)
	}
}
