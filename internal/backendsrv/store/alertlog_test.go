package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

// alertlog_test.go covers the Phase 20 (WANT-03/04) alert_log store funcs (20-01
// Task 2): InsertAlertTx (nullable wantID — the D-10 test-alert path, BLOCKER-1),
// ListInbox (newest-first, owner-scoped, non-nil), MarkAlertReadTx /
// MarkAllAlertsReadTx (owner-scoped, cross-owner silent no-op), UnreadCount, and
// RecentAlertExists (the dedup probe — suppresses a recent sent OR dm_blocked,
// warning 5). Reuses insertWebUser / commitTx (admins_test.go) + NewTestDB.

// alSeedSeq keeps each seedWant's throwaway character.name unique (UNIQUE COLLATE
// NOCASE) across the multiple seedWant calls a single test makes.
var alSeedSeq int

// seedWant inserts an active wishlist_item and returns its id (the FK target for
// non-test alerts). Phase 34 repoint: wantlist_item → wishlist_item, which requires
// a NOT-NULL character_id, so a throwaway owner+character are created per call.
func seedWant(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+discordID)
	if err != nil {
		t.Fatalf("seedWant owner(%q): %v", discordID, err)
	}
	ownerID, _ := res.LastInsertId()
	alSeedSeq++
	res, err = db.ExecContext(ctx, `INSERT INTO character (owner_id, name) VALUES (?, ?)`,
		ownerID, fmt.Sprintf("Toon-%s-%d", discordID, alSeedSeq))
	if err != nil {
		t.Fatalf("seedWant character(%q): %v", discordID, err)
	}
	charID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO wishlist_item (discord_user_id, character_id, slot, item_id, item_name, pinged, active, created_at)
		 VALUES (?, ?, 'Chest', ?, 'Seed Want', 1, 1, 1)`, discordID, charID, itemID)
	if err != nil {
		t.Fatalf("seedWant(%q, %d): %v", discordID, itemID, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestInsertAlertTx_NullableWantID_TestAlertPath(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	// BLOCKER-1: the D-10 test-alert has NO wantlist_item → wantID=nil → a row
	// with wantlist_item_id NULL. Under 00006's NOT NULL this would FK-fail.
	var alertID int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		alertID, e = InsertAlertTx(ctx, tx, nil, "disc-1", "test", nil, nil, 100, "sent")
		return e
	}); err != nil {
		t.Fatalf("InsertAlertTx (nil wantID, test-alert): %v", err)
	}
	if alertID <= 0 {
		t.Fatalf("InsertAlertTx returned non-positive id %d", alertID)
	}

	// The row reads back with wantlist_item_id NULL.
	var wantNull sql.NullInt64
	if err := db.QueryRow(`SELECT wantlist_item_id FROM alert_log WHERE id = ?`, alertID).Scan(&wantNull); err != nil {
		t.Fatalf("read back test-alert row: %v", err)
	}
	if wantNull.Valid {
		t.Errorf("test-alert wantlist_item_id = %d, want NULL", wantNull.Int64)
	}

	// It shows in the inbox as unread (read_at NULL).
	inbox := mustInbox(t, ctx, db, "disc-1")
	if len(inbox) != 1 {
		t.Fatalf("inbox len = %d, want 1", len(inbox))
	}
	if inbox[0].ReadAt != nil {
		t.Errorf("test-alert ReadAt = %v, want nil (unread)", *inbox[0].ReadAt)
	}
	if inbox[0].Source != "test" || inbox[0].SendStatus != "sent" {
		t.Errorf("test-alert row = %+v, want source=test status=sent", inbox[0])
	}
}

func TestListInbox_NewestFirstOwnerScopedNonNil(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")

	// Empty inbox → non-nil empty slice (JSON []).
	if got := mustInbox(t, ctx, db, "disc-1"); got == nil {
		t.Fatalf("ListInbox on empty returned nil, want non-nil empty slice")
	} else if len(got) != 0 {
		t.Fatalf("ListInbox on empty len = %d, want 0", len(got))
	}

	wantID := seedWant(t, ctx, db, "disc-1", 10)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		if _, e := InsertAlertTx(ctx, tx, &wantID, "disc-1", "ec_auction", nil, nil, 100, "sent"); e != nil {
			return e
		}
		if _, e := InsertAlertTx(ctx, tx, &wantID, "disc-1", "ec_auction", nil, nil, 300, "dm_blocked"); e != nil {
			return e
		}
		// Bob's alert must NOT leak into Alice's inbox.
		if _, e := InsertAlertTx(ctx, tx, nil, "disc-2", "test", nil, nil, 200, "sent"); e != nil {
			return e
		}
		return nil
	}); err != nil {
		t.Fatalf("seed alerts: %v", err)
	}

	alice := mustInbox(t, ctx, db, "disc-1")
	if len(alice) != 2 {
		t.Fatalf("Alice inbox len = %d, want 2 (Bob's must not leak)", len(alice))
	}
	// Newest-first: sent_at 300 then 100.
	if alice[0].SentAt != 300 || alice[1].SentAt != 100 {
		t.Errorf("inbox order = [%d, %d], want [300, 100] (newest-first)", alice[0].SentAt, alice[1].SentAt)
	}
	if got := mustInbox(t, ctx, db, "disc-2"); len(got) != 1 {
		t.Fatalf("Bob inbox len = %d, want 1", len(got))
	}
}

func TestMarkAlertReadTx_OwnerScopedAndUnreadCount(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")

	wantID := seedWant(t, ctx, db, "disc-1", 10)
	var a1 int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		a1, e = InsertAlertTx(ctx, tx, &wantID, "disc-1", "ec_auction", nil, nil, 100, "sent")
		if e != nil {
			return e
		}
		_, e = InsertAlertTx(ctx, tx, &wantID, "disc-1", "ec_auction", nil, nil, 200, "sent")
		return e
	}); err != nil {
		t.Fatalf("seed alerts: %v", err)
	}

	if n := mustUnread(t, ctx, db, "disc-1"); n != 2 {
		t.Fatalf("initial UnreadCount = %d, want 2", n)
	}

	// Bob tries to mark Alice's alert read → IDOR-safe silent no-op (false, nil).
	var ok bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		ok, e = MarkAlertReadTx(ctx, tx, a1, "disc-2", 999)
		return e
	}); err != nil {
		t.Fatalf("cross-owner MarkAlertReadTx errored: %v (want silent no-op)", err)
	}
	if ok {
		t.Fatalf("cross-owner MarkAlertReadTx = true, want false (IDOR no-op)")
	}
	if n := mustUnread(t, ctx, db, "disc-1"); n != 2 {
		t.Fatalf("after cross-owner no-op, Alice UnreadCount = %d, want 2 (untouched)", n)
	}

	// Alice marks her own alert read → (true, nil), unread drops to 1.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		ok, e = MarkAlertReadTx(ctx, tx, a1, "disc-1", 999)
		return e
	}); err != nil {
		t.Fatalf("MarkAlertReadTx own: %v", err)
	}
	if !ok {
		t.Fatalf("MarkAlertReadTx own unread alert = false, want true")
	}
	if n := mustUnread(t, ctx, db, "disc-1"); n != 1 {
		t.Fatalf("after own mark-read, UnreadCount = %d, want 1", n)
	}

	// Mark-all-read clears the rest.
	var flipped int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		flipped, e = MarkAllAlertsReadTx(ctx, tx, "disc-1", 1000)
		return e
	}); err != nil {
		t.Fatalf("MarkAllAlertsReadTx: %v", err)
	}
	if flipped != 1 {
		t.Errorf("MarkAllAlertsReadTx flipped = %d, want 1 (only the remaining unread)", flipped)
	}
	if n := mustUnread(t, ctx, db, "disc-1"); n != 0 {
		t.Fatalf("after mark-all-read, UnreadCount = %d, want 0", n)
	}
}

func TestRecentAlertExists_SuppressesSentAndDmBlocked(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	wantID := seedWant(t, ctx, db, "disc-1", 10)
	itemID := int64(10)

	// No alerts yet → no recent.
	if got := mustRecent(t, ctx, db, wantID, "ec_auction", &itemID, 50); got {
		t.Fatalf("RecentAlertExists with no rows = true, want false")
	}

	// A recent dm_blocked (warning 5): it MUST suppress a repeat — a DMs-off user
	// shouldn't accrue an identical dm_blocked inbox row every cycle.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := InsertAlertTx(ctx, tx, &wantID, "disc-1", "ec_auction", &itemID, nil, 100, "dm_blocked")
		return e
	}); err != nil {
		t.Fatalf("seed dm_blocked alert: %v", err)
	}
	if got := mustRecent(t, ctx, db, wantID, "ec_auction", &itemID, 50); !got {
		t.Errorf("RecentAlertExists with recent dm_blocked = false, want true (warning 5 suppression)")
	}

	// Outside the window (since > sent_at) → not recent.
	if got := mustRecent(t, ctx, db, wantID, "ec_auction", &itemID, 200); got {
		t.Errorf("RecentAlertExists outside window = true, want false")
	}

	// A 'sent' row also suppresses (the normal dedup case).
	want2 := seedWant(t, ctx, db, "disc-1", 20)
	item2 := int64(20)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := InsertAlertTx(ctx, tx, &want2, "disc-1", "ec_auction", &item2, nil, 100, "sent")
		return e
	}); err != nil {
		t.Fatalf("seed sent alert: %v", err)
	}
	if got := mustRecent(t, ctx, db, want2, "ec_auction", &item2, 50); !got {
		t.Errorf("RecentAlertExists with recent sent = false, want true")
	}

	// An 'error' row does NOT suppress (transient send error should be retried).
	want3 := seedWant(t, ctx, db, "disc-1", 30)
	item3 := int64(30)
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := InsertAlertTx(ctx, tx, &want3, "disc-1", "ec_auction", &item3, nil, 100, "error")
		return e
	}); err != nil {
		t.Fatalf("seed error alert: %v", err)
	}
	if got := mustRecent(t, ctx, db, want3, "ec_auction", &item3, 50); got {
		t.Errorf("RecentAlertExists with only an 'error' row = true, want false (errors are retryable)")
	}
}

func mustInbox(t *testing.T, ctx context.Context, db *sql.DB, discordID string) []AlertLogRow {
	t.Helper()
	rows, err := ListInbox(ctx, db, discordID)
	if err != nil {
		t.Fatalf("ListInbox(%q): %v", discordID, err)
	}
	return rows
}

func mustUnread(t *testing.T, ctx context.Context, db *sql.DB, discordID string) int {
	t.Helper()
	n, err := UnreadCount(ctx, db, discordID)
	if err != nil {
		t.Fatalf("UnreadCount(%q): %v", discordID, err)
	}
	return n
}

func mustRecent(t *testing.T, ctx context.Context, db *sql.DB, wantID int64, source string, itemID *int64, since int64) bool {
	t.Helper()
	got, err := RecentAlertExists(ctx, db, wantID, source, itemID, since)
	if err != nil {
		t.Fatalf("RecentAlertExists(%d, %q): %v", wantID, source, err)
	}
	return got
}
